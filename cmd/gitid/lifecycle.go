package main

// lifecycle.go is the ONE complete-lifecycle chokepoint per write verb (review
// R-11-CLI). D-02 requires that the CLI and the TUI run the same
// test → preview → confirmation → backup → write → re-test ceremony; sharing
// only the final write transaction does not prove that, because two callers
// could reach the same writer through different ceremonies. Each verb has
// exactly ONE named lifecycle function — runRotate / runRepair / runDelete —
// and every caller (the TUI commit seams in wiring.go today, plan 05-08's CLI
// handlers tomorrow) is a thin adapter over it. The per-verb stage sequence is
// defined once, as data, in lifecycleStages; this file is the single source of
// truth that both skins' tests assert against. When a stage list drifts, the
// CLI test and the TUI test break together.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/globalssh"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/tuikit"
)

// ---------------------------------------------------------------------------
// Confirmation authorization (review R2-03)
// ---------------------------------------------------------------------------

// confirmationMode is the three-valued authorization state a lifecycle
// function reaches at its confirm stage. It is an EXPLICIT enum, never a
// nilable func, because a nil func conflates two opposite intents —
// "authorization already happened" (the TUI's confirm screen ran) versus
// "authorization cannot happen" (no prompt could be installed for a
// non-interactive invocation). The ZERO value is confirmationRequired, so a
// caller that forgets to set the field fails closed rather than silently
// proceeding as authorized.
type confirmationMode int

const (
	// confirmationRequired is the ZERO VALUE. Reaching the confirm stage in
	// this mode with p.Prompt == nil returns errConfirmationUnavailable
	// immediately — before the backup stage, before any write. A caller that
	// sets nothing therefore fails closed: a scripted destructive invocation
	// with no TTY and no --yes can never be mistaken for pre-confirmed.
	confirmationRequired confirmationMode = iota
	// confirmationAlreadyObtained asserts a human already saw and accepted
	// the exact preview on a confirm screen. ONLY the TUI's commit seams may
	// set it — setting it from a code path with no such screen is a security
	// defect, not a shortcut.
	confirmationAlreadyObtained
	// confirmationBypassedWithYes is the CLI's consent: the user passed
	// --yes. It is distinct from confirmationAlreadyObtained so a receipt or
	// a log can tell "a human clicked confirm" from "a script asserted
	// consent up front" — and so the CLI can never reach the TUI's value.
	confirmationBypassedWithYes
)

// lifecyclePolicy carries everything that may legitimately vary across a
// lifecycle invocation. The backup stage is deliberately NOT policy-
// controlled: no field here can suppress it (D-02's invariant, asserted as a
// property over lifecycleStages in lifecycle_test.go).
type lifecyclePolicy struct {
	// DryRun stops the function after the plan stage, before the
	// confirmation gate and therefore before any backup or write. A dry run
	// needs no authorization because it cannot write, which keeps --dry-run
	// usable in non-interactive CI.
	DryRun bool
	// Confirm is one of the three confirmationMode values above.
	Confirm confirmationMode
	// Prompt is required iff Confirm == confirmationRequired. It receives
	// the plan preview and returns whether the user accepted it.
	Prompt func(preview string) (bool, error)
	// Stages is a test hook called with every stage name as the function
	// enters that stage's boundary. nil in production.
	Stages func(stage string)
}

// lifecycleResult is what a lifecycle function reports. Backups are the
// timestamped safety copies the write produced and, once authorized, are
// never suppressed. Restored carries journal.restore()'s outcome lines on a
// rollback. ArchivedKeyPaths names every archive copy THIS transaction
// created (populated for a failure from the archive step onward, review
// R3-01). ReTest carries the closing post-write connectivity test result.
// Advisories carries post-write shadow advisories from the D-04 verify stage
// (plan 06-04). SimulationInconclusive is true when the pre-write simulation
// could not faithfully reproduce the config graph.
type lifecycleResult struct {
	Backups                []string
	Restored               []string
	Removed                []string
	ArchivedKeyPaths       []string
	ReTest                 tester.Result
	Advisories             []string
	SimulationInconclusive bool
}

// ---------------------------------------------------------------------------
// The per-verb stage table (review R2-02)
// ---------------------------------------------------------------------------

// lifecycleStages is the SINGLE source of truth for D-02's
// no-behavioral-fork claim: one complete-Lifecycle function per verb, both
// skins calling it, one table both skins' tests assert against. A stage list
// that drifts breaks the CLI test and the TUI test together.
//
// Rotate and repair run a connectivity test before AND after the write;
// delete CANNOT — a deleted identity has no Host block, no key, and no
// fragment left to connect with, so there is no connectivity test to run
// before or after it (review R2-02, reconciling this plan with plan 05-08's
// delete dry-run contract). Delete's closing `verify` stage is a re-read of
// the reconstructed inventory proving the identity is gone and the survivors
// still parse — the coherence check that plays delete's structural
// equivalent of a re-test without pretending a connection is possible.
//
// The verb-INDEPENDENT part of D-02 is asserted as a PROPERTY over this
// table, not as a literal sequence: every row must contain confirm, backup,
// and write in that relative order — a future verb cannot be added with the
// backup stage quietly missing. Plan 05-08's confirmation matrix and dry-run
// contract MUST be written against this same table; adding a verb means
// adding a row HERE first.
var lifecycleStages = map[string][]string{
	"rotate":          {"test", "plan", "confirm", "backup", "write", "retest"},
	"repair":          {"test", "plan", "confirm", "backup", "write", "retest"},
	"delete":          {"plan", "confirm", "backup", "write", "verify"},
	"global-ssh":      {"plan", "simulate", "confirm", "backup", "write", "verify"},
	"storage-migrate": {"plan", "confirm", "backup", "write"},
}

// errConfirmationUnavailable is returned when a confirmationRequired
// lifecycle call reaches its confirm stage with no prompt installed — the
// fail-closed proof that a non-interactive destructive run without --yes can
// never be mistaken for pre-confirmed (review R2-03).
var errConfirmationUnavailable = errors.New("gitid: confirmation required, but no confirmation prompt is available")

// confirmGate evaluates lifecyclePolicy.Confirm at the confirm stage
// boundary (review R2-03). confirmationAlreadyObtained and
// confirmationBypassedWithYes never consult a prompt — the authorization
// already happened, or a script asserted it up front. confirmationRequired
// consults p.Prompt, failing closed with errConfirmationUnavailable when none
// is installed.
func (b *realBackend) confirmGate(p lifecyclePolicy, preview string) (bool, error) {
	switch p.Confirm {
	case confirmationAlreadyObtained, confirmationBypassedWithYes:
		return true, nil
	case confirmationRequired:
		if p.Prompt == nil {
			return false, errConfirmationUnavailable
		}
		return p.Prompt(preview)
	default:
		return false, fmt.Errorf("gitid: internal: unknown confirmation mode %d: refusing to proceed", p.Confirm)
	}
}

// ---------------------------------------------------------------------------
// runRotate — the ONE complete rotate ceremony
// ---------------------------------------------------------------------------

// runRotate owns rotate's whole declared ceremony — test → plan → confirm →
// backup → write → re-test — driving it from lifecycleStages["rotate"]. No
// other function reimplements a stage. It takes txMu (WR-04), resolves the
// account from the SAME reconstruction the identity list renders, and calls
// the domain through b.depsForTransaction(journal) — never b.deps, whose
// ArchiveKeyPair binding refuses by design (review R3-01). A dry run stops
// after the plan stage; an un-authorized run fails closed before any backup
// or write; a mid-transaction failure restores every watched file to its
// pre-transaction bytes AND mode and removes every archive copy this
// transaction created.
func (b *realBackend) runRotate(name string, p lifecyclePolicy) (lifecycleResult, error) {
	b.txMu.Lock()
	defer b.txMu.Unlock()

	res := lifecycleResult{}
	record := func(stage string) {
		if p.Stages != nil {
			p.Stages(stage)
		}
	}
	stages := lifecycleStages["rotate"]

	if b.initErr != nil {
		return res, b.initErr
	}
	acct, found := b.findAccount(name)
	if !found {
		return res, fmt.Errorf("gitid: no such identity: %q", name)
	}
	acct = b.normalizeAccountForWrite(acct)

	// CR-01: refuse to archive (move) a key another identity still depends
	// on. identity.Rotate calls deps.ArchiveKeyPair — the MOVE primitive —
	// so without this gate a sibling sharing the current key is left with a
	// dangling IdentityFile the moment rotation completes. This is the same
	// class of loss ErrRepairTargetShared exists to prevent on the repair
	// side (below), applied at the ONE chokepoint both the CLI and TUI call
	// through. b.normalizedAccounts() (not b.accounts()) per CR-02, so a
	// mixed tilde/absolute IdentityFile spelling cannot hide the sharing.
	if owners := identity.SharedKeyOwners(b.normalizedAccounts(), acct.KeyPath, name); len(owners) > 0 {
		return res, fmt.Errorf(
			"gitid: refusing to rotate %q: its key pair is also used by %s — use `gitid identity new-key %s` instead: %w",
			name, strings.Join(owners, ", "), name, identity.ErrRepairTargetShared)
	}
	// Secondary consequence of the same missing gate (CR-01): rotating an
	// identity with no current key pair must refuse and point at `new-key`,
	// never call identity.Rotate with an empty/missing key path (which
	// silently archives "" instead of generating the missing key).
	if acct.KeyPath == "" || !fileExists(acct.KeyPath) {
		return res, fmt.Errorf("gitid: refusing to rotate %q: no current key pair to retire — use `gitid identity new-key %s`", name, name)
	}

	// test — the connectivity probe against the identity as it currently
	// resolves through its alias (the only probe a rotation can run before
	// the write). The resident is advisory at commit time: the TUI's two
	// test stages already gated the ceremony, and a transient probe failure
	// must not brick a rotation the user has confirmed.
	record(stages[0])
	_, _ = b.deps.Resolved(acct.Alias)

	// plan — render the confirmed-write preview the confirmation prompt
	// receives.
	record(stages[1])
	preview := "rotate " + name + ": archive the current key pair (move it out of " +
		b.displayPath(acct.KeyPath) + " into the D-06 archive directory), generate a new pair at the same canonical paths, " +
		"append the new signing line, and re-point the SSH block, gitconfig includeIf, fragment, and allowed_signers"
	if p.DryRun {
		// ordering consequence stated here: stopping AFTER the plan stage
		// means a dry run never reaches the confirmation gate and therefore
		// needs no authorization — a non-interactive dry run must not demand
		// consent for a run that cannot write.
		return res, nil
	}

	// confirm — the authorization boundary (review R2-03).
	record(stages[2])
	authorized, cerr := b.confirmGate(p, preview)
	if cerr != nil {
		return res, cerr
	}
	if !authorized {
		return res, fmt.Errorf("gitid: cancelled rotation of %q", name)
	}

	// backup — record the stage boundary. The timestamped backups themselves
	// are taken by the filewriter-backed writers during the write stage
	// below. No field of lifecyclePolicy can remove this stage (D-02).
	record(stages[3])

	// write — the backed-up, all-or-nothing transaction.
	record(stages[4])
	journal := newMutationJournal(b)
	for _, watch := range b.rotateWatchPaths(acct) {
		if werr := journal.watchFile(watch); werr != nil {
			return res, werr
		}
	}
	// A rotation may create the D-06 archive directory mid-transaction. If it
	// did not pre-exist, record it as created so rollback removes it too
	// (review R-10 / R2-08); if it pre-existed, only the copies inside are
	// created-files and only they are removed.
	archiveDir := sshconfig.ArchiveDir(b.sshDir)
	if _, serr := os.Stat(archiveDir); os.IsNotExist(serr) {
		if rerr := journal.recordCreatedDir(archiveDir); rerr != nil {
			return res, rerr
		}
	}
	// WR-04: deps.WriteSSH (writeSSHBlock) may ALSO create b.includeDir
	// (the SSH config.d directory) mid-transaction when the include-dir
	// layout needs its first Include line — same shape as the archive
	// directory above: record it as created only if it does not pre-exist,
	// so a mid-transaction rollback removes the directory it introduced
	// rather than leaving an empty shell behind.
	if b.storage().needsIncludeLine {
		if _, serr := os.Stat(b.includeDir); os.IsNotExist(serr) {
			if rerr := journal.recordCreatedDir(b.includeDir); rerr != nil {
				return res, rerr
			}
		}
	}

	// The domain is called through depsForTransaction(journal) — NEVER the
	// backend-wide b.deps, whose ArchiveKeyPair binding refuses (review
	// R3-01). identity.Deps is a value type, so re-binding one seam on a
	// copy is not a second wiring.
	deps := b.depsForTransaction(journal)
	if b.failCommitAt != nil {
		deps = injectRotateFailures(b, deps)
	}
	rot, terr := identity.Rotate(acct, deps)
	res.Backups = collectCreateBackups(rot.CreateResult)
	res.ArchivedKeyPaths = rotateArchivePaths(rot)
	if terr != nil {
		// Belt and braces (review R-10): the domain result names every
		// archive path it created (populated from the archive step onward,
		// including a failure INSIDE the archive step itself), so record them
		// here too — recordCreatedFile is an idempotent no-op on a path the
		// transaction seam already recorded, and both normally name the same
		// paths.
		for _, a := range res.ArchivedKeyPaths {
			if rerr := journal.recordCreatedFile(a); rerr != nil {
				terr = fmt.Errorf("%w; recording archive path %s for rollback: %v", terr, a, rerr)
			}
		}
		outcomes, restoreErr := journal.restore()
		res.Restored = outcomes
		wrapped := fmt.Errorf("gitid: rotating identity %q: %w", name, terr)
		if restoreErr != nil {
			wrapped = fmt.Errorf("%w; restoration results: %s", wrapped, strings.Join(outcomes, "; "))
		}
		return res, wrapped
	}

	// re-test — the closing post-write probe through the live config, which
	// now points at the new key. Its result is carried back in the lifecycle
	// result so the commit receipt can render it.
	record(stages[5])
	res.ReTest, _ = b.deps.Resolved(acct.Alias)
	return res, nil
}

// ---------------------------------------------------------------------------
// runRepair — the ONE complete new-key (repair) ceremony
// ---------------------------------------------------------------------------

// runRepair owns repair's whole declared ceremony — test → plan → confirm →
// backup → write → re-test — driving it from lifecycleStages["repair"]. It
// mirrors runRotate except for the two D-05 differences: repair generates a
// key at THIS identity's OWN canonical path (RepairKeyPath, never
// Account.KeyPath) and NEVER archives — pre-existing key material is left
// untouched, including a sibling's key the account may be sharing. Its
// repair commit message therefore carries an empty archived path.
func (b *realBackend) runRepair(name string, p lifecyclePolicy) (lifecycleResult, error) {
	b.txMu.Lock()
	defer b.txMu.Unlock()

	res := lifecycleResult{}
	record := func(stage string) {
		if p.Stages != nil {
			p.Stages(stage)
		}
	}
	stages := lifecycleStages["repair"]

	if b.initErr != nil {
		return res, b.initErr
	}
	acct, found := b.findAccount(name)
	if !found {
		return res, fmt.Errorf("gitid: no such identity: %q", name)
	}
	acct = b.normalizeAccountForWrite(acct)

	// test — advisory probe exactly as runRotate's (informational at commit
	// time; the resident stage voices the D-02 ceremony, not a hard gate).
	record(stages[0])
	_, _ = b.deps.Resolved(acct.Alias)

	// plan.
	record(stages[1])
	privTarget, _ := identity.RepairKeyPath(acct)
	preview := "repair " + name + ": generate a fresh key pair at the identity's own canonical path " +
		b.displayPath(privTarget) + " (no archiving, pre-existing key material untouched) and re-point the SSH block, " +
		"gitconfig includeIf, fragment, and allowed_signers to it"
	if p.DryRun {
		return res, nil
	}

	// confirm.
	record(stages[2])
	authorized, cerr := b.confirmGate(p, preview)
	if cerr != nil {
		return res, cerr
	}
	if !authorized {
		return res, fmt.Errorf("gitid: cancelled repair of %q", name)
	}

	// backup — unconditional once authorized (D-02). Nothing to record as
	// created below: repair never archives (D-05), so the archive directory
	// is never touched.
	record(stages[3])

	// write.
	record(stages[4])
	journal := newMutationJournal(b)
	for _, watch := range b.repairWatchPaths(acct) {
		if werr := journal.watchFile(watch); werr != nil {
			return res, werr
		}
	}
	// WR-04: same shape as runRotate's includeDir recording above — repair's
	// deps.WriteSSH may also create b.includeDir mid-transaction.
	if b.storage().needsIncludeLine {
		if _, serr := os.Stat(b.includeDir); os.IsNotExist(serr) {
			if rerr := journal.recordCreatedDir(b.includeDir); rerr != nil {
				return res, rerr
			}
		}
	}
	deps := b.depsForTransaction(journal) // never b.deps (review R3-01)
	if b.failCommitAt != nil {
		deps = injectRepairFailures(b, deps)
	}
	// otherOwnersOfTarget is computed against the repair TARGET path
	// (RepairKeyPath), NEVER against Account.KeyPath — the pathological case
	// a naive reading gets wrong (review R-02). RepairKey fails closed with
	// ErrRepairTargetShared before any seam runs when a sibling depends on
	// the target. CR-02: privTarget is ABSOLUTE (derived from the already-
	// normalized acct), so the comparison list must be b.normalizedAccounts()
	// — comparing against the raw, tilde-spelled b.accounts() made this
	// unreachable for any recipe-shaped identity.
	otherOwners := identity.SharedKeyOwners(b.normalizedAccounts(), privTarget, name)
	rr, rerr := identity.RepairKey(acct, otherOwners, deps)
	res.Backups = collectCreateBackups(rr)
	if rerr != nil {
		outcomes, restoreErr := journal.restore()
		res.Restored = outcomes
		wrapped := fmt.Errorf("gitid: repairing identity %q: %w", name, rerr)
		if restoreErr != nil {
			wrapped = fmt.Errorf("%w; restoration results: %s", wrapped, strings.Join(outcomes, "; "))
		}
		return res, wrapped
	}

	// re-test.
	record(stages[5])
	res.ReTest, _ = b.deps.Resolved(acct.Alias)
	return res, nil
}

// normalizeAccountForWrite expands a reconstructed account's tilde-prefixed
// artifact paths against b.home (never os.UserHomeDir()/$HOME — WR-35's
// lesson: a caller that already owns an explicit home must never re-derive
// it) and fills the gitid-managed TARGET paths (GitconfigPath /
// SSHConfigPath / AllowedSignersPath), which Reconstruct deliberately does
// not populate. Every lifecycle function runs an account through this before
// any write, so the delete-only tilt-rblock runDelete used to inline is now
// shared by all three verbs.
func (b *realBackend) normalizeAccountForWrite(acct identity.Account) identity.Account {
	acct.FragmentPath = expandTildeForHome(acct.FragmentPath, b.home)
	acct.KeyPath = expandTildeForHome(acct.KeyPath, b.home)
	acct.PubPath = expandTildeForHome(acct.PubPath, b.home)
	acct.GitconfigPath = b.gitconfigPath
	acct.SSHConfigPath = b.sshConfigPath
	acct.AllowedSignersPath = b.allowedSigners
	return acct
}

// rotateWatchPaths returns every file a rotation transaction can mutate: the
// SSH storage target (where the Host block lives), the gitconfig, the
// identity's fragment, the allowed_signers file, and both canonical key
// paths. All are watched (snapshotted bytes + mode) before the write so a
// rollback restores them exactly. (watchFile deduplicates a repeated path and
// tolerates a missing one, so an account with an empty FragmentPath is fine.)
func (b *realBackend) rotateWatchPaths(acct identity.Account) []string {
	paths := []string{
		b.storageTargetPath(),
		// WR-04: b.sshConfigPath (~/.ssh/config itself) is DISTINCT from
		// storageTargetPath() whenever the include-dir layout is active —
		// deps.WriteSSH (writeSSHBlock) calls sshconfig.EnsureIncludeLine
		// against b.sshConfigPath outside the journal entirely if it is not
		// also watched here, so a mid-transaction failure left the injected
		// Include line behind.
		b.sshConfigPath,
		b.gitconfigPath,
		acct.AllowedSignersPath,
		acct.KeyPath,
		acct.PubPath,
	}
	if acct.FragmentPath != "" {
		paths = append(paths, acct.FragmentPath)
	}
	return paths
}

// repairWatchPaths returns every file a repair transaction can mutate: the
// SSH storage target, ~/.ssh/config itself (WR-04 — see rotateWatchPaths),
// the gitconfig, the identity's fragment, the allowed_signers file, and
// THIS identity's OWN canonical key paths (RepairKeyPath — the account's
// Account.KeyPath is deliberately NOT watched, because repair must never
// touch the key material it is not permitted to overwrite).
func (b *realBackend) repairWatchPaths(acct identity.Account) []string {
	privTarget, pubTarget := identity.RepairKeyPath(acct)
	paths := []string{b.storageTargetPath(), b.sshConfigPath, b.gitconfigPath, acct.AllowedSignersPath, privTarget, pubTarget}
	if acct.FragmentPath != "" {
		paths = append(paths, acct.FragmentPath)
	}
	return paths
}

// collectCreateBackups gathers every non-empty backup path a CreateResult
// carries, in the same field order the result declares them.
func collectCreateBackups(res identity.CreateResult) []string {
	var out []string
	for _, p := range []string{res.SSHBackup, res.GitconfigBackup, res.AllowedSignersBackup} {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// rotateArchivePaths gathers the two archived key-pair paths a RotateResult
// reports when non-empty (the private and public archive copies).
func rotateArchivePaths(rot identity.RotateResult) []string {
	var out []string
	if rot.ArchivedPrivatePath != "" {
		out = append(out, rot.ArchivedPrivatePath)
	}
	if rot.ArchivedPublicPath != "" {
		out = append(out, rot.ArchivedPublicPath)
	}
	return out
}

// displayPaths maps a raw-path slice through b.displayPath, the WR-01
// discipline CommitGit applies — every path that becomes user-facing must
// render the `~/`-shortened form, never the absolute sandbox path.
func displayPaths(b *realBackend, in []string) []string {
	out := make([]string, len(in))
	for i, p := range in {
		out[i] = b.displayPath(p)
	}
	return out
}

// displayMessages maps a raw-message slice through b.displayMessage, the
// WR-01/WR-23 discipline CommitGit applies to restoration outcome lines.
func displayMessages(b *realBackend, in []string) []string {
	out := make([]string, len(in))
	for i, m := range in {
		out[i] = b.displayMessage(m)
	}
	return out
}

// ---------------------------------------------------------------------------
// runDelete — the ONE complete delete ceremony
// ---------------------------------------------------------------------------

// runDelete owns delete's whole declared ceremony — plan → confirm → backup →
// write → verify — driving it from lifecycleStages["delete"] (review R2-02).
// plan 05-01 shipped this function under THIS exact name so that this plan
// extends one function rather than introducing a differently-named
// replacement (review R3-05): Task 1 changes its signature to take a
// lifecyclePolicy and return a lifecycleResult, Task 2 extends its body for
// the everything scope. It is the ONE production delete writer: the TUI's
// CommitDelete, the everything scope, and plan 05-08's CLI handler all reach
// it and nothing else.
//
// Delete invokes the connectivity-tester seam ZERO times — dry or full —
// because a deleted identity has no Host block, no key, and no fragment left
// to connect with (lifecycleStages["delete"] contains no test stage). Its
// closing verify stage re-reads the reconstructed inventory and fails the
// call if the identity is still reconstructable, so the stage is load-bearing
// rather than a recorded no-op.
func (b *realBackend) runDelete(name string, scope identity.DeleteScope, p lifecyclePolicy) (lifecycleResult, error) {
	b.txMu.Lock()
	defer b.txMu.Unlock()

	res := lifecycleResult{}
	record := func(stage string) {
		if p.Stages != nil {
			p.Stages(stage)
		}
	}
	stages := lifecycleStages["delete"]

	if b.initErr != nil {
		return res, b.initErr
	}
	acct, found := b.findAccount(name)
	if !found {
		return res, fmt.Errorf("gitid: no such identity: %q", name)
	}
	acct = b.normalizeAccountForWrite(acct)

	// plan — the target list the confirmation prompt previews, derived from
	// the SAME identity.PlanDelete both the CLI's --dry-run listing and the
	// TUI's confirm screen render (WR-03) — never a second, hand-rolled
	// preview that can diverge from what the write actually does (the prior
	// deletePlanPreview unconditionally named the key pair with no
	// keySurvives consultation, promising to delete a key the write would
	// keep for a sibling).
	record(stages[0])
	plan, plerr := b.DeletePlan(name, string(scope))
	if plerr != nil {
		return res, fmt.Errorf("gitid: refusing to delete %q: the delete plan could not be built: %w", name, plerr)
	}
	preview := b.deletePlanPreviewText(plan)
	if p.DryRun {
		return res, nil
	}

	// confirm — the authorization boundary.
	record(stages[1])
	authorized, cerr := b.confirmGate(p, preview)
	if cerr != nil {
		return res, cerr
	}
	if !authorized {
		return res, fmt.Errorf("gitid: cancelled delete of %q", name)
	}

	// backup — unconditional once authorized (D-02).
	record(stages[2])

	// write — the backed-up, all-or-nothing transaction.
	record(stages[3])
	journal := newMutationJournal(b)
	if werr := journal.watchFile(b.gitconfigPath); werr != nil {
		return res, werr
	}
	if acct.FragmentPath != "" {
		if werr := journal.watchFile(acct.FragmentPath); werr != nil {
			return res, werr
		}
	}
	if scope == identity.DeleteScopeEverything {
		if werr := journal.watchFile(b.storageTargetPath()); werr != nil {
			return res, werr
		}
		if werr := journal.watchFile(b.allowedSigners); werr != nil {
			return res, werr
		}
		if acct.KeyPath != "" {
			if werr := journal.watchFile(acct.KeyPath); werr != nil {
				return res, werr
			}
		}
		if acct.PubPath != "" {
			if werr := journal.watchFile(acct.PubPath); werr != nil {
				return res, werr
			}
		}
		// An everything delete may create the D-06 archive directory when it
		// copies the key pair out (D-11). Record it as created when it did
		// not pre-exist so a rollback leaves no empty shell behind.
		archiveDir := sshconfig.ArchiveDir(b.sshDir)
		if _, serr := os.Stat(archiveDir); os.IsNotExist(serr) {
			if rerr := journal.recordCreatedDir(archiveDir); rerr != nil {
				return res, rerr
			}
		}
	}

	// The domain is called through deleteDepsForTransaction(journal) — never
	// buildDeleteDeps(b) directly, whose CopyKeyPairToArchive binding refuses
	// (review R3-01).
	deps := b.deleteDepsForTransaction(journal)
	if b.failCommitAt != nil {
		deps = injectDeleteFailures(b, deps)
	}
	del, derr := identity.Delete(acct, scope, deps)
	res.Backups = collectDeleteBackups(del)
	res.ArchivedKeyPaths = del.ArchivedKeyPaths
	if derr != nil {
		for _, a := range res.ArchivedKeyPaths {
			if rerr := journal.recordCreatedFile(a); rerr != nil {
				derr = fmt.Errorf("%w; recording archive path %s for rollback: %v", derr, a, rerr)
			}
		}
		outcomes, restoreErr := journal.restore()
		res.Restored = outcomes
		wrapped := fmt.Errorf("gitid: deleting identity %q: %w", name, derr)
		if restoreErr != nil {
			wrapped = fmt.Errorf("%w; restoration results: %s", wrapped, strings.Join(outcomes, "; "))
		}
		return res, wrapped
	}

	if scope == identity.DeleteScopeEverything {
		// Task 2: surface the two facts the everything-scope receipt must
		// name (D-09/D-11) — that the ref-counted provider rewrite was
		// removed, and WHERE the key pair was copied before removal. The
		// provider label is a constant string (not a path); the archive
		// paths are raw filesystem paths the seam scrubs via displayPath.
		if del.ProviderRewriteRemoved {
			res.Removed = append(res.Removed, "provider rewrite (shared)")
		}
		res.Removed = append(res.Removed, del.ArchivedKeyPaths...)
	}

	// verify — delete's closing coherence check (lifecycleStages["delete"]'s
	// last entry).
	record(stages[4])
	if verr := b.verifyDeleteGone(name, scope); verr != nil {
		return res, verr
	}
	return res, nil
}

// deletePlanPreviewText renders the ONE-line confirmation-prompt preview
// from the real DeletePlanView (WR-03) — a target list built by walking
// plan.Targets (display-formatted the same way renderDeletePlan formats
// them), never a second, hand-rolled list that can name a file the write
// will not actually touch (e.g. a key pair keySurvives is keeping for a
// sibling).
func (b *realBackend) deletePlanPreviewText(plan tuikit.DeletePlanView) string {
	files := make([]string, 0, len(plan.Targets))
	for _, t := range plan.Targets {
		files = append(files, b.displayPath(t.File))
	}
	return "delete " + plan.Name + " (" + plan.Scope + "): " + strings.Join(files, ", ")
}

// verifyDeleteGone is delete's closing verify stage: a re-read of the
// reconstructed inventory — never a no-op, and never the vacuous nil-on-error
// b.accounts() path — failing the call when the delete did not actually take
// effect. The check is deliberately scope-aware:
//
//   - DeleteScopeEverything: the identity must not reconstruct AT ALL — its
//     SSH Host block, gitconfig includeIf block, fragment, allowed_signers
//     block, and key pair are all removed, so any trace of the name is a
//     failed delete.
//
//   - DeleteScopeGitOnly: the SSH Host block stays by design (D-10), so the
//     name WILL still reconstruct from the SSH side — that is the intended
//     outcome, not a failure. The git side must be gone: the gitconfig must
//     no longer carry the identity's managed includeIf block. (The fragment
//     file's absence is asserted separately by the git-only tests.)
func (b *realBackend) verifyDeleteGone(name string, scope identity.DeleteScope) error {
	deps := identity.InventoryDepsForHome(b.home)
	sshBytes, err := deps.ReadSSHConfig()
	if err != nil {
		return fmt.Errorf("gitid: delete verify: rereading ssh config: %w", err)
	}
	gcBytes, err := deps.ReadGitconfig()
	if err != nil {
		return fmt.Errorf("gitid: delete verify: rereading gitconfig: %w", err)
	}
	if scope == identity.DeleteScopeGitOnly {
		blocks := gitconfig.ParseManagedIncludeIf(gcBytes)
		if _, ok := blocks[name]; ok {
			return fmt.Errorf("gitid: delete verify: gitconfig still carries the managed includeIf block for %q", name)
		}
		return nil
	}
	accounts, err := identity.Reconstruct(sshBytes, gcBytes, deps.ReadFragment)
	if err != nil {
		return fmt.Errorf("gitid: delete verify: re-reconstructing the inventory: %w", err)
	}
	for _, a := range accounts {
		if a.Name == name {
			return fmt.Errorf("gitid: delete verify: %q is still reconstructable from the on-disk configuration", name)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// runGlobalSSHApply — the ONE global-SSH fix ceremony
// ---------------------------------------------------------------------------

// runGlobalSSHApply owns the whole declared global-SSH fix ceremony — plan →
// confirm → backup → write — driving it from lifecycleStages["global-ssh"]. It
// is the ONE production writer a global option fix may use: the TUI's
// CommitGlobalSSH reaches it and plan 06-06's CLI verb will too; nothing else
// writes a global option. It takes txMu, resolves the storage target through
// b.storage() (D-07: layout-follows-identities), floors the Include line and
// creates the include directory when the resolved layout reports that it is
// needed, builds the explicit overlay from the requested keys and their D-10
// recommended values, merges the target file's bytes through the single owner
// sshconfig.EnsureGlobals, and writes every path through filewriter (the
// timestamped-backup + atomic temp→rename chokepoint).
//
// A key that is not writable to the wildcard block is REJECTED BY NAME before
// any candidate is built — returning an error rather than silently dropping
// it, because a silent drop would make a later receipt count wrong.
//
// ROLLBACK — the phase's SINGLE restore authority for the apply path: the
// write opens a mutationJournal and watchFile's every path it is about to
// touch (the resolved target, plus ~/.ssh/config itself when the Include line
// is floored on a fresh machine). Each individual file write still goes
// through filewriter.Write, which takes the timestamped backup and performs
// the atomic temp→rename at 0600; the journal does not replace that, it
// records what to put back. Any error after the first write calls the
// journal's restore, which returns every watched file to its pre-transaction
// bytes. A file that did not exist before the transaction is REMOVED rather
// than "restored to empty". The restored paths ride out on the lifecycleResult
// so the receipt can name them.
//
// plan 06-04 EXTENDS this function with pre-write simulation and post-write
// verification and REUSES this journal — it must not add a second restore path
// or layer another journal over it. plan 06-06 calls this function from the
// CLI. The one deliberate exception is plan 06-05's migration ceremony:
// sshconfig.Migrate owns its own rollback because it alone knows which of its
// two files it wrote, so runSSHStorageMigrate does NOT open a journal — one
// restoration authority per transaction.
func (b *realBackend) runGlobalSSHApply(keys []string, p lifecyclePolicy) (lifecycleResult, error) {
	b.txMu.Lock()
	defer b.txMu.Unlock()

	res := lifecycleResult{}
	record := func(stage string) {
		if p.Stages != nil {
			p.Stages(stage)
		}
	}
	stages := lifecycleStages["global-ssh"]

	if b.initErr != nil {
		return res, b.initErr
	}

	// plan — reject unwritable keys by name BEFORE building any candidate, and
	// build the explicit overlay from the D-10 recommended values.
	record(stages[0])
	explicit := make(map[string]string, len(keys))
	var previewKeys []string
	for _, k := range keys {
		policy, ok := globalssh.PolicyFor(k)
		if !ok {
			return res, fmt.Errorf("gitid: unknown global SSH option %q", k)
		}
		if policy.Scope == "per-alias" {
			return res, fmt.Errorf("gitid: option %q cannot be applied to the global Host * block (the recipe scopes it per-alias)", k)
		}
		if policy.MinOpenSSH != "" {
			outcome, _ := globalssh.VersionGate(b.readSSHVersion(), policy)
			if outcome != globalssh.VersionAvailable {
				return res, fmt.Errorf("gitid: option %q cannot be applied until OpenSSH compatibility is verified (run ssh -V)", k)
			}
		}
		explicit[k] = policy.Recommended
		previewKeys = append(previewKeys, k)
	}

	// Build the EnsureGlobals candidate bytes (same bytes the write will use).
	st := b.storage()
	existingForPlan, _ := os.ReadFile(st.targetPath) //nolint:gosec // trusted gitid-managed path
	candidateForPlan, err := sshconfig.EnsureGlobals(existingForPlan, explicit, platform.CurrentOS())
	if err != nil {
		return res, fmt.Errorf("gitid: building candidate for simulation: %w", err)
	}

	// simulate — D-04 pre-write whole-graph simulation. A BuildGraph error
	// becomes INCONCLUSIVE (gitid not being able to model the graph is not a
	// reason to refuse a backed-up, reversible write). Reported on the plan so
	// a dry run shows shadowing without writing.
	record(stages[1])
	simGraph, buildErr := globalssh.BuildGraph(b.sshConfigPath, st.targetPath, candidateForPlan)
	if buildErr != nil {
		res.SimulationInconclusive = true
	} else {
		simResult := globalssh.Simulate(globalssh.BuildProbeDeps(b.sshConfigPath), simGraph, keys)
		res.SimulationInconclusive = simResult.Inconclusive
		for _, f := range simResult.Findings {
			if f.ShadowedByFile != "" {
				res.Advisories = append(res.Advisories, fmt.Sprintf("shadow warning: %s will be shadowed by %s (line %d)", f.Key, b.displayPath(f.ShadowedByFile), f.ShadowedByLine))
			} else {
				res.Advisories = append(res.Advisories, fmt.Sprintf("shadow warning: %s will be shadowed (source unnameable)", f.Key))
			}
		}
	}

	target := b.globalsTargetPath()
	if p.DryRun {
		// Ordering consequence stated here, matching runRotate: stopping AFTER
		// the plan stage means a dry run never reaches the confirmation gate
		// and therefore needs no authorization.
		return res, nil
	}

	// confirm — the authorization boundary.
	record(stages[2])
	preview := fmt.Sprintf("apply global SSH option(s) %s to the gitid Host * block in %s",
		strings.Join(previewKeys, ", "), b.displayPath(target))
	authorized, cerr := b.confirmGate(p, preview)
	if cerr != nil {
		return res, cerr
	}
	if !authorized {
		return res, fmt.Errorf("gitid: cancelled global SSH apply of %q", strings.Join(keys, ", "))
	}

	// backup — unconditional once authorized (D-02). The timestamped backups
	// are taken by the filewriter-backed writes during the write stage below.
	record(stages[3])

	// write — the backed-up, all-or-nothing transaction.
	record(stages[4])
	journal := newMutationJournal(b)
	fail := func(cause error) (lifecycleResult, error) {
		outcomes, restoreErr := journal.restore()
		res.Restored = outcomes
		wrapped := fmt.Errorf("gitid: applying global SSH options: %w", cause)
		if restoreErr != nil {
			wrapped = fmt.Errorf("%w; restoration results: %s", wrapped, strings.Join(outcomes, "; "))
		}
		return res, wrapped
	}
	inject := func(step string) error {
		if b.failCommitAt == nil {
			return nil
		}
		return b.failCommitAt(step)
	}
	if werr := journal.watchFile(st.targetPath); werr != nil {
		return res, werr
	}
	if st.needsIncludeLine {
		// WR-04: ~/.ssh/config itself is DISTINCT from the storage target when
		// the include-dir layout is active — the floored Include line must be
		// watched or a mid-transaction rollback leaves it behind. Record the
		// include directory as created when it does not pre-exist, so a
		// rollback removes it rather than leaving an empty shell.
		if werr := journal.watchFile(b.sshConfigPath); werr != nil {
			return res, werr
		}
		if _, serr := os.Stat(b.includeDir); os.IsNotExist(serr) {
			if rerr := journal.recordCreatedDir(b.includeDir); rerr != nil {
				return res, rerr
			}
		}
		if err := inject("global-include-line"); err != nil {
			return fail(err)
		}
		if err := sshconfig.EnsureIncludeDir(b.includeDir); err != nil {
			return fail(err)
		}
		backup, err := sshconfig.EnsureIncludeLine(b.sshConfigPath)
		if err != nil {
			return fail(err)
		}
		journal.addBackup(backup)
	}
	if err := inject("global-ssh-write"); err != nil {
		return fail(err)
	}
	existing, err := os.ReadFile(st.targetPath) //nolint:gosec // st.targetPath is a trusted gitid-managed path supplied in-process
	if err != nil && !os.IsNotExist(err) {
		return fail(err)
	}
	merged, err := sshconfig.EnsureGlobals(existing, explicit, platform.CurrentOS())
	if err != nil {
		return fail(err)
	}
	backup, err := filewriter.Write(st.targetPath, merged, deleteSSHConfigMode)
	if err != nil {
		return fail(err)
	}
	journal.addBackup(backup)

	res.Backups = append(res.Backups, journal.backups...)

	// verify — D-04 post-write re-verification against the live machine.
	// Advisories from verify stage are appended; the write has already
	// succeeded so verification is informational only.
	record(stages[5])
	verResult := globalssh.Verify(globalssh.BuildProbeDeps(b.sshConfigPath), keys)
	for _, f := range verResult.Findings {
		if f.ShadowedByFile != "" {
			res.Advisories = append(res.Advisories, fmt.Sprintf("advisory: %s was applied but is still shadowed at %s (line %d) — the fix may not take effect", f.Key, b.displayPath(f.ShadowedByFile), f.ShadowedByLine))
		} else {
			res.Advisories = append(res.Advisories, fmt.Sprintf("advisory: %s was applied but is still shadowed by an external directive — the fix may not take effect", f.Key))
		}
	}

	return res, nil
}

// globalsTargetPath is the resolved file the gitid `Host *` globals block
// lands in — the SAME resolution the identity block uses (D-07:
// layout-follows-identities, never a second independent decision).
func (b *realBackend) globalsTargetPath() string {
	return b.storage().targetPath
}

// runSSHStorageMigrate — the ONE complete storage-migration ceremony
//
// runSSHStorageMigrate owns the whole declared storage-migration ceremony:
// plan → confirm → backup → write — driving it from
// lifecycleStages["storage-migrate"]. No other path in this binary performs
// a layout migration; plan 06-06's CLI verb calls this same function.
//
// The engine (sshconfig.MigrateWithPlan / sshconfig.Migrate) owns rollback:
// it alone knows which of its two files it wrote, so this function does NOT
// open a mutationJournal — one restoration authority per transaction. Stated
// explicitly to prevent a future refactor from layering a second journal over
// it by analogy with runGlobalSSHApply.
//
// planToken selects between the two legitimate callers:
//   - Non-empty (TUI, which already showed the user a preview): call
//     takePendingMigration — the atomic lookup-and-consume helper defined in
//     <lock_contract> — while holding txMu. A non-match (selection changed,
//     screen re-entered, plan already committed, or wrong process) is a
//     REFUSAL with errReopenPreview, never a silent re-plan.
//   - Empty (CLI, plan 06-06, no preview screen): call sshconfig.PlanMigration
//     and MigrateWithPlan back-to-back within the same txMu hold. The window
//     is microseconds, not minutes, and the digest check still runs — but
//     there is nothing to carry from a preview that never happened.
//     IMPORTANT: this branch calls sshconfig.PlanMigration DIRECTLY, never
//     b.SSHStorageMigrationPlan. That backend method takes txMu itself
//     (<lock_contract> rule 5) and this function already holds txMu — routing
//     the CLI branch through it would self-deadlock on Go's non-reentrant
//     sync.Mutex. This is the cycle-3 hazard reappearing at a new call site.
func (b *realBackend) runSSHStorageMigrate(target tuikit.SSHStorageLayout, planToken string, p lifecyclePolicy) (lifecycleResult, error) {
	b.txMu.Lock()
	defer b.txMu.Unlock()

	res := lifecycleResult{}
	record := func(stage string) {
		if p.Stages != nil {
			p.Stages(stage)
		}
	}
	stages := lifecycleStages["storage-migrate"]

	if b.initErr != nil {
		return res, b.initErr
	}

	// plan — resolve the current layout and refuse a no-op.
	record(stages[0])
	st := b.storage()
	currentLayout := tuikit.StorageSentinel
	if st.includeLayout {
		currentLayout = tuikit.StorageInclude
	}
	if currentLayout == target {
		return res, fmt.Errorf("gitid: storage layout is already %s — nothing to migrate", target)
	}

	direction := sshconfig.MigrateToInclude
	if target == tuikit.StorageSentinel {
		direction = sshconfig.MigrateToInFile
	}

	aliases := b.managedAliases()
	deps := newMigrateDeps(b.sshConfigPath, filepath.Join(b.includeDir, gitidConfigFileName), aliases)

	var plan sshconfig.MigrationPlan
	if planToken != "" {
		// TUI path: retrieve the pre-computed plan. takePendingMigration
		// takes pendingMigrationMu only, never txMu (<lock_contract> rule 3).
		held, ok := b.takePendingMigration(planToken)
		if !ok {
			return res, errReopenPreview
		}
		plan = held
	} else {
		// CLI path (plan 06-06): plan and commit in one txMu hold.
		// MUST call sshconfig.PlanMigration directly — NOT
		// b.SSHStorageMigrationPlan, which would deadlock on txMu.
		var planErr error
		plan, planErr = sshconfig.PlanMigration(direction, deps)
		if planErr != nil {
			return res, fmt.Errorf("gitid: planning migration: %w", planErr)
		}
	}

	if p.DryRun {
		return res, nil
	}

	// confirm — the authorization boundary.
	record(stages[1])
	authorized, cerr := b.confirmGate(p, fmt.Sprintf("migrate SSH storage layout to %s", target))
	if cerr != nil {
		return res, cerr
	}
	if !authorized {
		return res, fmt.Errorf("gitid: cancelled SSH storage migration to %s", target)
	}

	// backup + write — MigrateWithPlan performs both in one call: it verifies
	// the plan's digests against disk (aborting with ErrConfigChangedSincePreview
	// if they changed), takes the backups (step 2), then writes the two files.
	// The engine's own rollback is the single restoration authority; this
	// function does NOT open a mutationJournal.
	record(stages[2]) // backup
	record(stages[3]) // write
	result, merr := sshconfig.MigrateWithPlan(plan, deps)
	// res.Restored must be read regardless of merr — rollbackTracked returns
	// the restored paths ALONGSIDE the error (CR-04), and sshWriteExitCode
	// only reports exit code 2 (rolled back) when res.Restored is non-empty.
	res.Restored = result.Restored
	if merr != nil {
		return res, fmt.Errorf("gitid: storage migration: %w", merr)
	}

	if result.SourceBackup != "" {
		res.Backups = append(res.Backups, result.SourceBackup)
	}
	if result.TargetBackup != "" {
		res.Backups = append(res.Backups, result.TargetBackup)
	}

	return res, nil
}
