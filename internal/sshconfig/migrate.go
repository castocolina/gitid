package sshconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/tester"
)

// absentDigestMarker is the content digest value used when a file does not
// exist at plan time. It is distinct from the SHA256 of empty bytes, so a
// file that goes from absent to zero-length is caught as a modification.
const absentDigestMarker = "absent"

// contentDigest returns the SHA256 hex digest of content, or absentDigestMarker
// when content is nil (the file did not exist).
func contentDigest(content []byte) string {
	if content == nil {
		return absentDigestMarker
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// ErrConfigChangedSincePreview is the errors.Is-matchable sentinel returned
// by MigrateWithPlan when one of the two config files' on-disk content differs
// from the digest the plan was computed against. The caller can test for it
// with errors.Is to distinguish "you need to re-open the preview" from any
// other migration failure, so the screen can render the correct message.
var ErrConfigChangedSincePreview = errors.New("sshconfig: migrate: config changed since preview")

// configChangedError wraps ErrConfigChangedSincePreview with the path of the
// changed file, for errors.Is-compatible detection.
type configChangedError struct {
	path string
}

func (e *configChangedError) Error() string {
	return fmt.Sprintf("%s: %s (its current content was left unchanged)", ErrConfigChangedSincePreview.Error(), e.path)
}

func (e *configChangedError) Is(target error) bool {
	return target == ErrConfigChangedSincePreview
}

// MigrationPlan is the pure, non-mutating result of PlanMigration. It carries
// the exact bytes the migration would write to each file, a rendered diff, and
// a per-file content digest of the bytes it was computed against. MigrateWithPlan
// re-reads both files, compares their content against these digests, and
// aborts with ErrConfigChangedSincePreview if anything moved under the preview
// — before taking a single backup — and otherwise writes plan.SourceAfter /
// plan.DestAfter verbatim without recomposing.
//
// Carrying the plan OBJECT (not just the function) is what closes the
// cycle-2 HIGH: a pure PlanMigration that both the preview and Migrate call
// independently against disk at two different times allows a change landing
// between preview and confirm to commit different bytes the user never
// approved. With one plan object, MigrateWithPlan writes exactly what
// PlanMigration returned, or nothing.
type MigrationPlan struct {
	Direction       MigrateDirection
	SourcePath      string
	DestPath        string
	SourceBefore    []byte
	DestBefore      []byte
	SourceAfter     []byte
	DestAfter       []byte
	MovedIdentities []string
	MovedGlobals    []string
	Diff            string
	// Digests is the content digest of the EXACT bytes this plan was computed
	// against, keyed by path, with an explicit absent marker for a file that
	// did not exist. It is what makes the plan verifiable at commit time.
	Digests map[string]string
}

// MigrateDirection selects which layout gains the managed blocks.
type MigrateDirection int

const (
	// MigrateToInclude moves managed identity blocks OUT of ~/.ssh/config
	// and INTO the gitid-owned Include'd file, leaving the Include line
	// floored in ~/.ssh/config.
	MigrateToInclude MigrateDirection = iota
	// MigrateToInFile moves managed identity blocks OUT of the Include'd
	// file and back IN-LINE into ~/.ssh/config directly.
	MigrateToInFile
)

// MigrateStep identifies a transaction step for the afterStep test hook —
// the five-step order Migrate follows (Codex HIGH): preflight, backup,
// destination written, source trimmed, commit.
type MigrateStep int

const (
	// StepPreflight fires after the pre-migration snapshot + both-file parse
	// check (step 1) completes.
	StepPreflight MigrateStep = iota
	// StepBackup fires after both files are backed up (step 2).
	StepBackup
	// StepDestinationWritten fires after the destination file is written and
	// validated (step 3) — the add-before-remove commit point.
	StepDestinationWritten
	// StepSourceTrimmed fires after the source file is trimmed/rewritten and
	// the final combined state is validated (step 4).
	StepSourceTrimmed
	// StepCommit fires after the result is assembled (step 5), immediately
	// before Migrate returns success.
	StepCommit
)

// backupSnapshot pairs a file's path and step-2 on-disk backup path with its
// IN-MEMORY pristine pre-migration bytes, captured once at preflight. Codex
// HIGH #1: rollback restores from these in-memory bytes via the no-backup
// RestoreFile seam, so it NEVER re-enters WriteFile's own backup-creation
// step and can never clobber the on-disk step-2 backup it names for the
// user's manual recovery.
type backupSnapshot struct {
	path       string
	backupPath string
	content    []byte
}

// MigrateResult carries the outcome of a Migrate call, whether it committed
// or rolled back.
type MigrateResult struct {
	// SourceBackup / TargetBackup are the timestamped backup paths for the
	// file LOSING blocks (source) and the file GAINING blocks (target),
	// captured before any content-changing write (step 2). Populated only on
	// a successful commit.
	SourceBackup string
	// TargetBackup is the backup path for the file gaining blocks. Populated
	// only on a successful commit.
	TargetBackup string
	// Recovery is a human-readable restore-from-backup description.
	Recovery string
	// Restored lists the paths this transaction rolled back to their
	// pre-migration state (CR-04). Populated ONLY on a rollback — a file
	// gitid wrote earlier in THIS transaction and then restored (or
	// removed, when it did not pre-exist) after an abort. A file gitid never
	// wrote in this transaction is never listed, matching rollbackTracked's
	// writtenByUs guard. Empty on a successful commit.
	Restored []string
}

// MigrateDeps holds all external effects Migrate needs, injectable for
// tests. Named MigrateDeps (NOT a bare Deps) — adopt.go's AdoptDeps lives in
// the same package, and two `type Deps` in one Go package is a `Deps
// redeclared` compile error that per-task isolated test runs would not catch.
type MigrateDeps struct {
	// ConfigPath is ~/.ssh/config — always the real entry point `ssh`
	// itself reads.
	ConfigPath string
	// IncludePath is the gitid-owned Include'd file
	// (~/.ssh/config.d/gitid.config).
	IncludePath string
	// Aliases lists every managed alias to snapshot/validate resolution for.
	Aliases []string

	// ReadFile reads a file's bytes. Wired to os.ReadFile in production.
	ReadFile func(path string) ([]byte, error)
	// WriteFile writes content to path at mode through the filewriter
	// chokepoint (backup + atomic temp->rename->chmod). Wired to
	// filewriter.Write in production — every write in this file routes
	// through this seam (STORE-04); no direct stdlib whole-file write is
	// ever used here.
	WriteFile func(path string, content []byte, mode os.FileMode) (backupPath string, err error)
	// ResolveAlias runs a real `ssh -G -F <configPath> <alias>` resolution
	// and returns the resolved IdentityFile list — the real-binary proof of
	// behavior preservation (never faked, per the CONTEXT.md-locked
	// constraint).
	ResolveAlias func(configPath, alias string) ([]string, error)
	// RemoveFile deletes path, tolerating an already-missing file
	// (idempotent). Used only during rollback to restore a file that did NOT
	// pre-exist before migration (empty backupPath) back to its true
	// pre-migration "absent" state — otherwise content written by a later
	// step would survive an aborted rollback. Wired to os.Remove in
	// production.
	RemoveFile func(path string) error
	// RestoreFile atomically replaces path with content at mode WITHOUT
	// creating a backup (wired to filewriter.WriteNoBackup in production).
	// This is the dedicated rollback/restore seam (Codex HIGH #1): rollback
	// restores a file that DID pre-exist from the IN-MEMORY pristine bytes
	// captured at preflight through THIS seam, never through WriteFile —
	// re-entering WriteFile's own backup-creation step during a restore
	// would create a NEW backup of the failed live file, and that new
	// backup could clobber a still-live pristine recovery snapshot on a
	// same-instant collision (the exact STORE-03 crash-safety gap this seam
	// closes).
	RestoreFile func(path string, content []byte, mode os.FileMode) error

	// BackupFile copies path to a timestamped sibling WITHOUT replacing the
	// target, returning the backup path (or "" when path does not exist). It
	// is the 06-05 backup-only seam that replaces the previous step-2
	// mechanism of calling WriteFile with the same bytes: the old approach
	// was itself a content-changing write, so an external edit landing
	// between the preflight snapshot and step 2 was silently overwritten
	// before any concurrency check could see it. BackupFile never replaces
	// the target, so it cannot destroy an external edit. Wired to
	// filewriter.Backup in production.
	BackupFile func(path string) (backupPath string, err error)

	// afterStep is an optional test hook invoked after each step; a non-nil
	// return aborts the transaction as if that step had failed. nil in
	// production.
	afterStep func(step MigrateStep) error
}

// migrateFileMode is the restrictive mode for both files Migrate touches —
// ~/.ssh/config and the Include'd gitid.config file are both private,
// potentially key-path-referencing material (0600, never relying on umask).
const migrateFileMode os.FileMode = 0o600

// migrateResolveTimeout bounds every real `ssh -G` resolution Migrate runs
// (preflight snapshot + per-step validation) so a pathological config (e.g.
// a hanging `Match exec`) can never block a migration indefinitely (T-01-03,
// Codex HIGH #2). It is a var, not a const, so tests can shrink it to
// exercise real timeout behavior without waiting out the production
// default — mirrors internal/platform's probeTimeout.
var migrateResolveTimeout = 3 * time.Second

// RealMigrateDeps returns production MigrateDeps wired to the real
// filesystem and a real `ssh -G` resolver — the live constructor for
// cmd-layer callers.
func RealMigrateDeps(configPath, includePath string, aliases []string) MigrateDeps {
	return MigrateDeps{
		ConfigPath:  configPath,
		IncludePath: includePath,
		Aliases:     aliases,
		ReadFile:    os.ReadFile,
		WriteFile:   filewriter.Write,
		BackupFile:  filewriter.Backup,
		ResolveAlias: func(cfgPath, alias string) ([]string, error) {
			// Bounded by migrateResolveTimeout (T-01-03, Codex HIGH #2): a
			// pathological config (e.g. a hanging `Match exec`) must never
			// block a migration indefinitely — mirrors internal/platform's
			// probeTimeout + exec.CommandContext pattern.
			ctx, cancel := context.WithTimeout(context.Background(), migrateResolveTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, "ssh", "-G", "-F", cfgPath, alias) //nolint:gosec // arg-slice form, no shell; cfgPath/alias are gitid-managed inputs (G204)
			// Run ssh in its own process group and SIGKILL the whole group on
			// timeout. A pathological `Match exec` (or a shell that forks its
			// child) can leave a grandchild holding ssh's stdout pipe after the
			// direct child is killed; on Linux /bin/sh forks rather than exec's
			// the child, so .Output() would block on that pipe until the
			// grandchild exits — defeating the context deadline (observed only
			// on Linux CI, 30s hang). Group-killing reaps the grandchild;
			// WaitDelay is a belt-and-suspenders bound so Output() can never wait
			// on held pipes past the deadline even if the kill races (T-01-03).
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error {
				if cmd.Process != nil {
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // best-effort group kill
				}
				return nil
			}
			cmd.WaitDelay = 500 * time.Millisecond
			out, err := cmd.Output()
			if err != nil {
				if ctx.Err() != nil {
					return nil, fmt.Errorf("sshconfig: migrate: ssh -G timed out after %s resolving %s: %w", migrateResolveTimeout, alias, ctx.Err())
				}
				return nil, err
			}
			return tester.ParseResolved(string(out)).IdentityFiles, nil
		},
		RemoveFile: func(path string) error {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		},
		RestoreFile: filewriter.WriteNoBackup,
	}
}

// PlanMigration performs the read-only planning half of the migration:
// classifying the managed blocks, composing the destination and source bytes,
// and recording a per-file content digest of the EXACT bytes it read. It never
// writes, backs up, or shells out beyond the resolution snapshot.
//
// The returned MigrationPlan carries SourceAfter/DestAfter (the bytes
// MigrateWithPlan will write) and Digests (keyed by path, with absentDigestMarker
// for a file that does not exist). MigrateWithPlan re-reads both files and
// compares them against these digests before taking any backup, aborting with
// ErrConfigChangedSincePreview if anything moved.
func PlanMigration(direction MigrateDirection, deps MigrateDeps) (MigrationPlan, error) {
	sourcePath, destPath := migratePaths(direction, deps)

	// NO EnsureIncludeDir here: planning is read-only. MigrateWithPlan creates
	// the directory after the confirm gate (see below). A missing directory is
	// not an error at plan time — readOrEmpty already tolerates a missing
	// destination file (and a missing parent directory surfaces the same
	// os.IsNotExist path).

	sourceContent, err := readOrEmpty(deps, sourcePath)
	if err != nil {
		return MigrationPlan{}, fmt.Errorf("sshconfig: plan migration: reading %s: %w", sourcePath, err)
	}
	destContent, err := readOrEmpty(deps, destPath)
	if err != nil {
		return MigrationPlan{}, fmt.Errorf("sshconfig: plan migration: reading %s: %w", destPath, err)
	}

	_, sourceGlobals := migrationClasses(sourceContent)
	_, destGlobals := migrationClasses(destContent)
	if len(sourceGlobals) > 0 && len(destGlobals) > 0 {
		return MigrationPlan{}, fmt.Errorf(
			"sshconfig: plan migration: refusing to guess which wildcard stanza wins: both %s and %s carry a gitid globals block; merge or remove one before migrating — resolve it on the Options screen",
			sourcePath, destPath)
	}

	movableIdentities, movableGlobals := migrationClasses(sourceContent)
	destAfter := reorderGlobalLast(composeDestination(destContent, sourceContent, movableIdentities, movableGlobals))
	sourceAfter := reorderGlobalLast(composeSource(direction, sourceContent, movableIdentities, movableGlobals))

	diff := buildMigrationDiff(direction, movableIdentities, movableGlobals)

	return MigrationPlan{
		Direction:       direction,
		SourcePath:      sourcePath,
		DestPath:        destPath,
		SourceBefore:    sourceContent,
		DestBefore:      destContent,
		SourceAfter:     sourceAfter,
		DestAfter:       destAfter,
		MovedIdentities: movableIdentities,
		MovedGlobals:    movableGlobals,
		Diff:            diff,
		Digests: map[string]string{
			sourcePath: contentDigest(sourceContent),
			destPath:   contentDigest(destContent),
		},
	}, nil
}

// buildMigrationDiff returns a human-readable summary of what will move.
func buildMigrationDiff(direction MigrateDirection, identities, globals []string) string {
	var lines []string
	for _, name := range identities {
		lines = append(lines, "+ identity block: "+name)
	}
	for _, name := range globals {
		lines = append(lines, "+ globals block: "+name)
	}
	if direction == MigrateToInclude {
		lines = append(lines, "+ Include ~/.ssh/config.d/gitid.config (floored near top of ~/.ssh/config)")
	}
	if len(lines) == 0 {
		return "(nothing to move)"
	}
	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	return result
}

// checkDigestMatch reads path via deps.ReadFile and compares the content
// digest against expected. Returns a *configChangedError (wrapping
// ErrConfigChangedSincePreview) when the digest differs.
func checkDigestMatch(deps MigrateDeps, path, expected string) error {
	content, err := readOrEmpty(deps, path)
	if err != nil {
		return fmt.Errorf("sshconfig: migrate: re-reading %s for digest check: %w", path, err)
	}
	if contentDigest(content) != expected {
		return &configChangedError{path: path}
	}
	return nil
}

// MigrateWithPlan is the ONLY function in this package that writes to disk.
// Its FIRST action — before any backup and before anything content-changing —
// is to re-read both files and compare them against plan.Digests. A mismatch
// returns a distinct, errors.Is-matchable ErrConfigChangedSincePreview naming
// the changed file, having written nothing and backed up nothing. Only after
// that check passes does it take the backups (via deps.BackupFile, which
// copies WITHOUT replacing the target) and write precisely plan.DestAfter and
// plan.SourceAfter — it never recomposes from the current disk contents.
//
// Why the pair rather than just PlanMigration: a pure PlanMigration that both
// the preview and Migrate call independently is TWO reads of disk at TWO
// different times. The preview renders plan A; the user reads it, thinks, and
// confirms; a second PlanMigration computes plan B against whatever disk says
// now and commits plan B. Both plans are internally consistent, the concurrency
// detector inside the transaction sees nothing wrong because plan B matches
// the disk it was just computed from, and the user has approved bytes that were
// never written. Carrying the plan OBJECT plus its digests closes that window.
// (06-REVIEWS.md cycle-2 HIGH finding.)
func MigrateWithPlan(plan MigrationPlan, deps MigrateDeps) (MigrateResult, error) {
	sourcePath := plan.SourcePath
	destPath := plan.DestPath

	if plan.Direction == MigrateToInclude {
		if derr := EnsureIncludeDir(filepath.Dir(destPath)); derr != nil {
			return MigrateResult{}, fmt.Errorf("sshconfig: migrate: %w", derr)
		}
	}

	// --- Step 0 (plan-digest check): re-read both files and compare against
	// the digests the plan was computed against. This check guards the
	// preview-to-confirm window — the much wider window the per-step checks
	// below cannot reach. A mismatch aborts BEFORE taking any backup or
	// writing anything.
	for path, expected := range plan.Digests {
		if err := checkDigestMatch(deps, path, expected); err != nil {
			return MigrateResult{}, err
		}
	}

	// --- Step 1: preflight snapshot for the transaction's own pre-backup /
	// pre-write concurrency checks. The plan's digests are used as the
	// preflight baseline — they were just re-verified against disk, so they
	// are current.
	preflightDigests := map[string]string{
		sourcePath: plan.Digests[sourcePath],
		destPath:   plan.Digests[destPath],
	}

	// Resolution snapshot for post-write validation.
	preSnapshot, err := snapshotResolution(deps)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: preflight: %w", err)
	}

	if _, perr := Parse(plan.SourceBefore); plan.SourceBefore != nil && perr != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: preflight: %s does not parse: %w", sourcePath, perr)
	}
	if _, perr := Parse(plan.DestBefore); plan.DestBefore != nil && perr != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: preflight: %s does not parse: %w", destPath, perr)
	}
	if serr := callStep(deps, StepPreflight); serr != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: aborted at preflight: %w", serr)
	}

	// --- Step 2: pre-backup check (the review's HIGH placement finding) and
	// BACKUP BOTH files via deps.BackupFile — which copies WITHOUT replacing
	// the target. The previous mechanism called deps.WriteFile with the
	// snapshotted bytes, which BACKED UP AND REPLACED; that made the "backup"
	// step itself a content-changing write, so an external edit between
	// preflight and step 2 was silently destroyed before any check could see
	// it. BackupFile is a pure copy: it cannot destroy an external edit.
	for _, path := range []string{sourcePath, destPath} {
		if err := checkDigestMatch(deps, path, preflightDigests[path]); err != nil {
			// Abort BEFORE taking any backup — no file has been written yet.
			return MigrateResult{}, fmt.Errorf("sshconfig: migrate: pre-backup check: %w", err)
		}
	}

	sourceBackup, err := deps.BackupFile(sourcePath)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: backing up %s: %w", sourcePath, err)
	}
	destBackup, err := deps.BackupFile(destPath)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: backing up %s: %w", destPath, err)
	}

	// sourceSnap/destSnap for rollback: IN-MEMORY pristine bytes (from the
	// plan) plus the backup path. written tracks whether THIS transaction has
	// written each file — rollback only restores files gitid itself wrote.
	sourceSnap := backupSnapshot{path: sourcePath, backupPath: sourceBackup, content: plan.SourceBefore}
	destSnap := backupSnapshot{path: destPath, backupPath: destBackup, content: plan.DestBefore}
	writtenByUs := map[string]bool{}

	if serr := callStep(deps, StepBackup); serr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: aborted at backup: %w", serr))
	}

	// --- Step 3: pre-write check for destination, then write DESTINATION
	// (add-before-remove ordering). After the write, update the recorded
	// digest to the bytes we just wrote so the source step's check compares
	// against the destination step's result rather than against preflight —
	// getting this wrong makes every two-step migration abort on the source.
	if err := checkDigestMatch(deps, destPath, preflightDigests[destPath]); err != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: pre-write check for %s: %w", destPath, err))
	}
	if _, perr := Parse(plan.DestAfter); perr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: composed %s is not parseable, refusing to write: %w", destPath, perr))
	}
	if _, werr := deps.WriteFile(destPath, plan.DestAfter, migrateFileMode); werr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: writing %s: %w", destPath, werr))
	}
	writtenByUs[destPath] = true
	// Update the preflight digest to the bytes we just wrote so the source
	// step's pre-write check compares against our write, not the original.
	preflightDigests[destPath] = contentDigest(plan.DestAfter)

	if verr := validateResolution(deps, preSnapshot); verr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs, verr)
	}
	if serr := callStep(deps, StepDestinationWritten); serr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: aborted after destination write: %w", serr))
	}

	// --- Step 4: pre-write check for source, then write/trim SOURCE.
	if err := checkDigestMatch(deps, sourcePath, preflightDigests[sourcePath]); err != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: pre-write check for %s: %w", sourcePath, err))
	}
	if _, perr := Parse(plan.SourceAfter); perr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: composed %s is not parseable, refusing to write: %w", sourcePath, perr))
	}
	if _, werr := deps.WriteFile(sourcePath, plan.SourceAfter, migrateFileMode); werr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: writing %s: %w", sourcePath, werr))
	}
	writtenByUs[sourcePath] = true

	if verr := validateResolution(deps, preSnapshot); verr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs, verr)
	}
	if serr := callStep(deps, StepSourceTrimmed); serr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: aborted after source trim: %w", serr))
	}

	// --- Step 5: commit.
	result := MigrateResult{
		SourceBackup: sourceBackup,
		TargetBackup: destBackup,
		Recovery: fmt.Sprintf(
			"restore both files from their pre-migration backups if needed: %s -> %s, and %s -> %s",
			sourceBackup, sourcePath, destBackup, destPath),
	}
	if serr := callStep(deps, StepCommit); serr != nil {
		return rollbackTracked(deps, sourceSnap, destSnap, writtenByUs,
			fmt.Errorf("sshconfig: migrate: aborted at commit: %w", serr))
	}
	return result, nil
}

// rollbackTracked is rollback narrowed to the per-file written-by-us tracking
// (06-REVIEWS.md HIGH #3: abort caused by detected external edit must never
// restore a file gitid has not written in this transaction). The externally
// edited file is left exactly as it is on disk; only gitid's own writes roll
// back. The invariant is enforced here and documented to prevent future widening.
//
// INVARIANT: restoreSnapshot is called ONLY for paths where writtenByUs[path]
// is true. A file gitid has not written is never restored, because restoring
// gitid's stale preflight snapshot over an edit another process just made
// would destroy exactly the data the detector was built to protect.
func rollbackTracked(deps MigrateDeps, sourceSnap, destSnap backupSnapshot, writtenByUs map[string]bool, cause error) (MigrateResult, error) {
	var restoreErrs []error
	var restored []string
	if writtenByUs[sourceSnap.path] {
		if errs := restoreSnapshot(deps, sourceSnap); len(errs) > 0 {
			restoreErrs = append(restoreErrs, errs...)
		} else {
			restored = append(restored, sourceSnap.path)
		}
	}
	if writtenByUs[destSnap.path] {
		if errs := restoreSnapshot(deps, destSnap); len(errs) > 0 {
			restoreErrs = append(restoreErrs, errs...)
		} else {
			restored = append(restored, destSnap.path)
		}
	}

	err := fmt.Errorf("sshconfig: migrate: aborted and restored gitid's own writes (source backup: %s, target backup: %s): %w",
		sourceSnap.backupPath, destSnap.backupPath, cause)
	if len(restoreErrs) > 0 {
		err = fmt.Errorf("%w; additionally, restore encountered errors: %v", err, restoreErrs)
	}
	// Restored is returned alongside the error (CR-04) — callers must read
	// res.Restored even on a non-nil error to learn what was rolled back.
	return MigrateResult{Restored: restored}, err
}

// Migrate performs a cross-file transactional migration of every managed
// identity block AND the gitid globals block between the in-file (~/.ssh/config)
// and Include'd (~/.ssh/config.d/gitid.config) layouts.
//
// Migrate is now a thin PlanMigration + MigrateWithPlan convenience wrapper
// (06-05 Task 1). Existing tests and callers are unaffected; the plan object
// is computed and consumed in the same call, so the digest check inside
// MigrateWithPlan runs against bytes read moments earlier — same as before,
// plus the safety of the pre-backup and pre-write concurrency checks.
//
// WHAT MOVES (06-REVIEWS.md HIGH resolution): every managed identity block
// plus the globals block — the wildcard stanza is layout-following content per
// D-07, and stranding it would leave two `Host *` stanzas across two files,
// the exact D-06 violation this phase closes. The Include-line block is
// explicitly NOT moved in either direction: it stays in the main config file,
// because it is what makes the destination reachable.
//
// Add-to-destination-before-remove-from-source ordering guarantees NO block
// loss at any crash point: the worst intermediate state is a transient
// duplicate (safe under first-match-wins), never a missing block or a
// dangling Include (T-01-22).
//
// Concurrent modification is detected at THREE checkpoints:
//  0. Plan-digest check (plan vs disk at MigrateWithPlan entry): guards the
//     preview-to-confirm window. Not exercised by this wrapper since the plan
//     was computed moments earlier.
//  1. Pre-backup check: before taking any backup, both files are re-read and
//     compared against the preflight digests; abort before touching anything.
//  2. Per-write check: before each of the two writes, the target file is
//     re-read and compared.
//
// An abort caused by a detected external edit restores only files gitid has
// written in this transaction; the externally edited file is left exactly as
// the other process left it (06-REVIEWS.md HIGH #3 / T-06-40).
func Migrate(direction MigrateDirection, deps MigrateDeps) (MigrateResult, error) {
	plan, err := PlanMigration(direction, deps)
	if err != nil {
		return MigrateResult{}, err
	}
	return MigrateWithPlan(plan, deps)
}

// migratePaths resolves the (source, destination) file pair for direction:
// source is the file CURRENTLY holding the managed blocks (loses them),
// destination is the file GAINING them.
func migratePaths(direction MigrateDirection, deps MigrateDeps) (sourcePath, destPath string) {
	if direction == MigrateToInclude {
		return deps.ConfigPath, deps.IncludePath
	}
	return deps.IncludePath, deps.ConfigPath
}

// readOrEmpty reads path via deps.ReadFile, tolerating a missing file as an
// empty (nil) slice rather than an error.
func readOrEmpty(deps MigrateDeps, path string) ([]byte, error) {
	content, err := deps.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return content, nil
}

// callStep invokes deps.afterStep for step when the hook is wired (non-nil
// in tests only — nil in production, so callStep is a no-op there).
func callStep(deps MigrateDeps, step MigrateStep) error {
	if deps.afterStep == nil {
		return nil
	}
	return deps.afterStep(step)
}

// snapshotResolution runs deps.ResolveAlias against the REAL ~/.ssh/config
// entry point for every managed alias, capturing the pre-migration
// resolution used as the behavior-preservation baseline.
func snapshotResolution(deps MigrateDeps) (map[string][]string, error) {
	snap := make(map[string][]string, len(deps.Aliases))
	for _, alias := range deps.Aliases {
		files, err := deps.ResolveAlias(deps.ConfigPath, alias)
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", alias, err)
		}
		snap[alias] = files
	}
	return snap, nil
}

// validateResolution re-resolves every managed alias against the REAL
// ~/.ssh/config entry point and compares it to snapshot, returning an error
// naming the first alias whose resolution changed.
func validateResolution(deps MigrateDeps, snapshot map[string][]string) error {
	for _, alias := range deps.Aliases {
		files, err := deps.ResolveAlias(deps.ConfigPath, alias)
		if err != nil {
			return fmt.Errorf("sshconfig: migrate: resolving %s: %w", alias, err)
		}
		if !equalStringSlices(files, snapshot[alias]) {
			return fmt.Errorf(
				"sshconfig: migrate: %s resolved %v, want %v (pre-migration snapshot) — behavior not preserved",
				alias, files, snapshot[alias])
		}
	}
	return nil
}

// equalStringSlices reports whether a and b hold the same elements in the
// same order.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// migrationClasses walks content's gitid-managed blocks and sorts each into one
// of three classes (06-REVIEWS.md HIGH resolution — the predecessor
// movableBlockNames filter answered "which blocks are per-identity content" and
// reused it for "which blocks move"; those diverged the moment D-07 made the
// globals block layout-following):
//
//   - identities — per-identity content that MOVES with a storage migration;
//   - globals — the gitid `Host *` wildcard stanza under EITHER registered
//     sentinel name (IsGlobalBlockName). The globals block is layout-following
//     content per D-07, so it moves exactly like the identities: stranding it
//     leaves two wildcard stanzas across two files, the exact D-06 violation
//     this phase exists to close;
//   - stationary — the reserved Include-line block, returned in NEITHER slice:
//     moving it would relocate the very wiring that makes the destination
//     reachable, breaking the migration's own Include plumbing.
//
// The classification uses the narrow IsGlobalBlockName predicate deliberately,
// NOT the broad IsReservedBlockName — conflating them is what made the Include
// wiring movable under the predecessor filter.
func migrationClasses(content []byte) (identities []string, globals []string) {
	for _, b := range filewriter.ListBlocks(content) {
		switch {
		case IsGlobalBlockName(b.Name):
			globals = append(globals, b.Name)
		case IsReservedBlockName(b.Name):
			// stationary — the Include-line block stays in the main config file
		default:
			identities = append(identities, b.Name)
		}
	}
	return identities, globals
}

// blockBodyMap indexes content's managed blocks by name for body lookup.
func blockBodyMap(content []byte) map[string]string {
	m := make(map[string]string)
	for _, b := range filewriter.ListBlocks(content) {
		m[b.Name] = b.Body
	}
	return m
}

// composeDestination returns destContent with every movable block set to its
// body from sourceContent — identities first, then the globals block, in that
// order (filewriter.ReplaceBlock — idempotent: replacing an already-present
// identical block is a no-op). The Include-line block is never in either list,
// so it is never written into the destination.
func composeDestination(destContent, sourceContent []byte, movableIdentities, movableGlobals []string) []byte {
	bodies := blockBodyMap(sourceContent)
	composed := destContent
	for _, name := range movableIdentities {
		composed = filewriter.ReplaceBlock(composed, name, bodies[name])
	}
	for _, name := range movableGlobals {
		composed = filewriter.ReplaceBlock(composed, name, bodies[name])
	}
	return composed
}

// composeSource returns sourceContent with every movable block removed
// (filewriter.RemoveBlock — idempotent). For MigrateToInclude, the Include
// line is ALSO floored in the same composed result (filewriter.
// PrependBlockIfNotFound) — the removal and the wiring that makes the
// destination reachable commit atomically together in one write, never as
// two separate writes (T-01-22: a crash between them must never leave a
// dangling Include-less state with the blocks already gone).
func composeSource(direction MigrateDirection, sourceContent []byte, movableIdentities, movableGlobals []string) []byte {
	composed := sourceContent
	for _, name := range movableIdentities {
		composed = filewriter.RemoveBlock(composed, name)
	}
	for _, name := range movableGlobals {
		composed = filewriter.RemoveBlock(composed, name)
	}
	if direction == MigrateToInclude {
		composed = filewriter.PrependBlockIfNotFound(composed, sshIncludeBlockName, sshIncludeLineBody)
	}
	return composed
}

// reorderGlobalLast re-positions the gitid `Host *` globals block to the end of
// content when present, under EITHER registered sentinel name (IsGlobalBlockName),
// preserving the "always last" first-match-wins invariant (D-09; mirrors
// sshconfig.Write's placement guarantee) after the compose step may have
// appended new identity blocks after it. By running AFTER the globals block has
// been added to the destination, the file ends with identities first and the
// wildcard stanza last. A no-op when no globals block exists.
func reorderGlobalLast(content []byte) []byte {
	for _, b := range filewriter.ListBlocks(content) {
		if !IsGlobalBlockName(b.Name) {
			continue
		}
		trimmed := filewriter.RemoveBlock(content, b.Name)
		return filewriter.ReplaceBlock(trimmed, b.Name, b.Body)
	}
	return content
}

// restoreSnapshot restores snap.path to its pre-migration state (a no-op
// finding of "did not pre-exist" when snap.backupPath is empty — the file
// did not exist before migration), returning any errors encountered (empty
// slice on success).
func restoreSnapshot(deps MigrateDeps, snap backupSnapshot) []error {
	if snap.backupPath == "" {
		// filewriter.Write's backupPath is non-empty ONLY when the target
		// pre-existed — an empty backupPath from step 2 means path did NOT
		// exist before migration. A later step may since have created it
		// (step 2's own "backup-only" write, or step 3/4's real content
		// write), so the true pre-migration state must be restored by
		// REMOVING it, not by a no-op that would leave that content behind.
		if err := deps.RemoveFile(snap.path); err != nil {
			return []error{fmt.Errorf("removing %s to restore pre-migration (absent) state: %w", snap.path, err)}
		}
		return nil
	}
	// Restore from the IN-MEMORY pristine bytes captured at preflight, via
	// the no-backup RestoreFile seam — deliberately NEVER deps.WriteFile
	// (Codex HIGH #1): re-entering WriteFile's own backup-creation step here
	// would create a NEW backup of the (failed) live file and could clobber
	// the pristine step-2 backup (snap.backupPath) this restore is standing
	// in for.
	if werr := deps.RestoreFile(snap.path, snap.content, migrateFileMode); werr != nil {
		return []error{fmt.Errorf("restoring %s from in-memory pre-migration snapshot: %w", snap.path, werr)}
	}
	return nil
}
