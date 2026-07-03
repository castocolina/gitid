package sshconfig

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/tester"
)

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

// MigrateResult carries the outcome of a successful Migrate call.
type MigrateResult struct {
	// SourceBackup / TargetBackup are the timestamped backup paths for the
	// file LOSING blocks (source) and the file GAINING blocks (target),
	// captured before any content-changing write (step 2).
	SourceBackup string
	// TargetBackup is the backup path for the file gaining blocks.
	TargetBackup string
	// Recovery is a human-readable restore-from-backup description.
	Recovery string
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

// Migrate performs a cross-file transactional migration of every managed
// identity block between the in-file (~/.ssh/config) and Include'd
// (~/.ssh/config.d/gitid.config) layouts, in this order (Codex HIGH):
//
//  1. preflight — snapshot the pre-migration `ssh -G` resolution for every
//     managed alias and confirm both files parse.
//  2. backup BOTH files (timestamped, via filewriter) — pristine,
//     pre-any-change snapshots, recorded regardless of what happens later.
//  3. write the DESTINATION file (the one GAINING blocks) FIRST and
//     validate it (parse + `ssh -G`).
//  4. write/trim the SOURCE file (the one LOSING blocks / keeping the
//     Include line for MigrateToInclude) and validate the FINAL combined
//     state (`ssh -G` for every alias equals the preflight snapshot).
//  5. commit — return success + both backup paths + a recovery description.
//
// Add-to-destination-before-remove-from-source ordering guarantees NO block
// loss at any crash point: the worst intermediate state is a transient
// duplicate (safe under first-match-wins), never a missing block or a
// dangling Include (T-01-22). On any post-write validation failure, or any
// afterStep-injected failure, Migrate restores BOTH files from the step-2
// backups and returns a wrapped error naming both backup paths.
//
// Migrate is idempotent: re-running after a crash-induced duplicate (or
// after a fully-completed migration) converges to the intended final state.
func Migrate(direction MigrateDirection, deps MigrateDeps) (MigrateResult, error) {
	sourcePath, destPath := migratePaths(direction, deps)

	// MigrateToInclude's destination (the gitid-owned config.d/gitid.config
	// file) may not have a parent directory yet on a first-ever migration —
	// composes with Task 1's EnsureIncludeDir (0700) so the destination write
	// in step 3 always has somewhere to land.
	if direction == MigrateToInclude {
		if derr := EnsureIncludeDir(filepath.Dir(destPath)); derr != nil {
			return MigrateResult{}, fmt.Errorf("sshconfig: migrate: %w", derr)
		}
	}

	// --- Step 1: preflight.
	preSnapshot, err := snapshotResolution(deps)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: preflight: %w", err)
	}

	sourceContent, err := readOrEmpty(deps, sourcePath)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: preflight: reading %s: %w", sourcePath, err)
	}
	destContent, err := readOrEmpty(deps, destPath)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: preflight: reading %s: %w", destPath, err)
	}
	if _, perr := Parse(sourceContent); perr != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: preflight: %s does not parse: %w", sourcePath, perr)
	}
	if _, perr := Parse(destContent); perr != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: preflight: %s does not parse: %w", destPath, perr)
	}
	if serr := callStep(deps, StepPreflight); serr != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: aborted at preflight: %w", serr)
	}

	// --- Step 2: backup BOTH files (pristine, pre-any-write snapshots).
	sourceBackup, err := deps.WriteFile(sourcePath, sourceContent, migrateFileMode)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: backing up %s: %w", sourcePath, err)
	}
	destBackup, err := deps.WriteFile(destPath, destContent, migrateFileMode)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("sshconfig: migrate: backing up %s: %w", destPath, err)
	}
	// sourceSnap/destSnap bundle each file's path + on-disk backup path with
	// its IN-MEMORY pristine bytes (Codex HIGH #1) — the single source of
	// truth every rollback call below restores from, never re-reading the
	// on-disk backup file.
	sourceSnap := backupSnapshot{path: sourcePath, backupPath: sourceBackup, content: sourceContent}
	destSnap := backupSnapshot{path: destPath, backupPath: destBackup, content: destContent}
	if serr := callStep(deps, StepBackup); serr != nil {
		return rollback(deps, sourceSnap, destSnap,
			fmt.Errorf("sshconfig: migrate: aborted at backup: %w", serr))
	}

	// --- Step 3: write DESTINATION first (add-before-remove ordering).
	movable := movableBlockNames(sourceContent)
	destComposed := reorderGlobalLast(composeDestination(destContent, sourceContent, movable))
	if _, perr := Parse(destComposed); perr != nil {
		return rollback(deps, sourceSnap, destSnap,
			fmt.Errorf("sshconfig: migrate: composed %s is not parseable, refusing to write: %w", destPath, perr))
	}
	if _, werr := deps.WriteFile(destPath, destComposed, migrateFileMode); werr != nil {
		return rollback(deps, sourceSnap, destSnap,
			fmt.Errorf("sshconfig: migrate: writing %s: %w", destPath, werr))
	}
	if verr := validateResolution(deps, preSnapshot); verr != nil {
		return rollback(deps, sourceSnap, destSnap, verr)
	}
	if serr := callStep(deps, StepDestinationWritten); serr != nil {
		return rollback(deps, sourceSnap, destSnap,
			fmt.Errorf("sshconfig: migrate: aborted after destination write: %w", serr))
	}

	// --- Step 4: write/trim SOURCE (remove migrated blocks; MigrateToInclude
	// also floors the Include line in the same composed write, so the
	// removal and the wiring that makes the destination reachable commit
	// atomically together — never as two separate writes).
	sourceComposed := reorderGlobalLast(composeSource(direction, sourceContent, movable))
	if _, perr := Parse(sourceComposed); perr != nil {
		return rollback(deps, sourceSnap, destSnap,
			fmt.Errorf("sshconfig: migrate: composed %s is not parseable, refusing to write: %w", sourcePath, perr))
	}
	if _, werr := deps.WriteFile(sourcePath, sourceComposed, migrateFileMode); werr != nil {
		return rollback(deps, sourceSnap, destSnap,
			fmt.Errorf("sshconfig: migrate: writing %s: %w", sourcePath, werr))
	}
	if verr := validateResolution(deps, preSnapshot); verr != nil {
		return rollback(deps, sourceSnap, destSnap, verr)
	}
	if serr := callStep(deps, StepSourceTrimmed); serr != nil {
		return rollback(deps, sourceSnap, destSnap,
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
		return rollback(deps, sourceSnap, destSnap,
			fmt.Errorf("sshconfig: migrate: aborted at commit: %w", serr))
	}
	return result, nil
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

// movableBlockNames returns the managed block names in content that
// participate in migration — every gitid-managed identity block EXCLUDING
// the reserved Include block and the macOS `_global` wildcard block (both
// are non-identity wiring, not per-identity content).
func movableBlockNames(content []byte) []string {
	var names []string
	for _, b := range filewriter.ListBlocks(content) {
		if b.Name == globalBlockName || IsReservedBlockName(b.Name) {
			continue
		}
		names = append(names, b.Name)
	}
	return names
}

// blockBodyMap indexes content's managed blocks by name for body lookup.
func blockBodyMap(content []byte) map[string]string {
	m := make(map[string]string)
	for _, b := range filewriter.ListBlocks(content) {
		m[b.Name] = b.Body
	}
	return m
}

// composeDestination returns destContent with every name in movable set to
// its body from sourceContent (filewriter.ReplaceBlock — idempotent:
// replacing an already-present identical block is a no-op).
func composeDestination(destContent, sourceContent []byte, movable []string) []byte {
	bodies := blockBodyMap(sourceContent)
	composed := destContent
	for _, name := range movable {
		composed = filewriter.ReplaceBlock(composed, name, bodies[name])
	}
	return composed
}

// composeSource returns sourceContent with every name in movable removed
// (filewriter.RemoveBlock — idempotent). For MigrateToInclude, the Include
// line is ALSO floored in the same composed result (filewriter.
// PrependBlockIfNotFound) — the removal and the wiring that makes the
// destination reachable commit atomically together in one write, never as
// two separate writes (T-01-22: a crash between them must never leave a
// dangling Include-less state with the blocks already gone).
func composeSource(direction MigrateDirection, sourceContent []byte, movable []string) []byte {
	composed := sourceContent
	for _, name := range movable {
		composed = filewriter.RemoveBlock(composed, name)
	}
	if direction == MigrateToInclude {
		composed = filewriter.PrependBlockIfNotFound(composed, sshIncludeBlockName, sshIncludeLineBody)
	}
	return composed
}

// reorderGlobalLast re-positions the macOS `_global` Host * block to the end
// of content when present, preserving the "always last" first-match-wins
// invariant (mirrors sshconfig.Write's placement guarantee) after
// ReplaceBlock may have appended new identity blocks after it. A no-op when
// no `_global` block exists.
func reorderGlobalLast(content []byte) []byte {
	for _, b := range filewriter.ListBlocks(content) {
		if b.Name == globalBlockName {
			trimmed := filewriter.RemoveBlock(content, globalBlockName)
			return filewriter.ReplaceBlock(trimmed, globalBlockName, b.Body)
		}
	}
	return content
}

// rollback restores BOTH files to their pre-migration state and returns a
// wrapped error naming both step-2 backup paths as a human recovery
// reference (T-01-22 recovery path). It is invoked on any post-write
// validation failure or afterStep-injected error.
//
// Restoration itself goes through restoreSnapshot, which uses the
// IN-MEMORY pristine bytes captured at preflight via the no-backup
// RestoreFile seam (Codex HIGH #1) — rollback never calls WriteFile to
// restore a pre-existing file, so it can never create a new backup that
// clobbers the on-disk step-2 backups named in the error message below.
func rollback(deps MigrateDeps, sourceSnap, destSnap backupSnapshot, cause error) (MigrateResult, error) {
	var restoreErrs []error
	restoreErrs = append(restoreErrs, restoreSnapshot(deps, sourceSnap)...)
	restoreErrs = append(restoreErrs, restoreSnapshot(deps, destSnap)...)

	err := fmt.Errorf("sshconfig: migrate: aborted and restored from backups (source: %s, target: %s): %w",
		sourceSnap.backupPath, destSnap.backupPath, cause)
	if len(restoreErrs) > 0 {
		err = fmt.Errorf("%w; additionally, restore encountered errors: %v", err, restoreErrs)
	}
	return MigrateResult{}, err
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
