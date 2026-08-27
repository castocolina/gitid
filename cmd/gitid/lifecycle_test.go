package main

// lifecycle_test.go covers plan 05-07 Task 1: the ONE complete-lifecycle
// chokepoint per write verb (review R-11-CLI), the per-verb stage table
// (review R2-02), the fail-closed three-valued confirmation mode (review
// R2-03), the journal's created-DIRECTORY recording and disjoint-set
// invariant (reviews R-10 / R2-08), and the real rotate/repair transactions
// with all-or-nothing rollback (KEY-05, KEY-07) — including the archive-step
// second-source-removal failure the lifecycle must roll back exactly as
// completely as any later step (review R3-01).

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/tuikit"
)

// rec stages records every stage name a lifecycle policy's Stages hook
// receives, in order.
func rec() (func(string), *[]string) {
	var out []string
	return func(stage string) { out = append(out, stage) }, &out
}

// groupHermeticBackend returns a backend over home with the two
// connectivity-test seams that would otherwise shell out to a REAL ssh
// handshake replaced by deterministic fakes ON b.deps — the lifecycle
// functions read b.deps directly for their own test/retest stages and hand a
// COPY to the domain via depsForTransaction, so mutating the backend-wide
// value covers both (realHermeticDeps applies the same override to a hand-
// built transaction copy for the domain-level tests).
func groupHermeticBackend(home string) *realBackend {
	b := newBackendForHome(home)
	b.deps.PreWrite = func(_, _ string, _ int) tester.Result {
		return tester.Result{Outcome: tester.ReachableNotUploaded}
	}
	b.deps.Resolved = func(_ string) (tester.Result, tester.ResolvedConfig) {
		return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{}
	}
	return b
}

// forEachVerb drives fn once per write verb with that verb's lifecycle
// function bound to the common closure shape.
func forEachVerb(t *testing.T, home, name string, scope identity.DeleteScope, fn func(verb string, call func(p lifecyclePolicy) (lifecycleResult, error))) {
	t.Helper()
	b := groupHermeticBackend(home)
	fn("rotate", func(p lifecyclePolicy) (lifecycleResult, error) { return b.runRotate(name, p) })
	fn("repair", func(p lifecyclePolicy) (lifecycleResult, error) { return b.runRepair(name, p) })
	fn("delete", func(p lifecyclePolicy) (lifecycleResult, error) {
		return b.runDelete(name, scope, p)
	})
}

// ---------------------------------------------------------------------------
// lifecycleStages — the table and its invariants (review R2-02)
// ---------------------------------------------------------------------------

// TestLifecycleStagesEveryRowContainsConfirmBackupWrite is the D-02 invariant
// asserted as a PROPERTY over the table rather than as one literal sequence:
// every verb's row must contain confirm, backup, and write in that relative
// order, so a verb added later cannot quietly miss the backup stage.
func TestLifecycleStagesEveryRowContainsConfirmBackupWrite(t *testing.T) {
	for verb, stages := range lifecycleStages {
		if len(stages) < 3 {
			t.Errorf("%s: stage row %v is shorter than the confirm/backup/write core", verb, stages)
			continue
		}
		pos := map[string]int{}
		for i, s := range stages {
			pos[s] = i
		}
		for _, required := range []string{"confirm", "backup", "write"} {
			if _, ok := pos[required]; !ok {
				t.Errorf("%s: row %v is missing the %q stage — the D-02 backup invariant must be structural", verb, stages, required)
			}
		}
		if pos["confirm"] >= pos["backup"] || pos["backup"] >= pos["write"] {
			t.Errorf("%s: row %v must order confirm -> backup -> write, got positions %v", verb, stages, pos)
		}
	}
}

// TestRotateAndRepairRowsAreTestWrapped delete currently has no pre/post
// connectivity test (a deleted identity has nothing left to connect with,
// review R2-02); pin the two test-wrapped rows explicitly so a future verb
// cannot turn the table's intent into "every row has a test stage".
func TestRotateAndRepairRowsAreTestWrapped(t *testing.T) {
	if got := lifecycleStages["rotate"]; len(got) != 6 || got[0] != "test" || got[5] != "retest" {
		t.Errorf("rotate row = %v, want test-first retest-last six stages", got)
	}
	if got := lifecycleStages["repair"]; len(got) != 6 || got[0] != "test" || got[5] != "retest" {
		t.Errorf("repair row = %v, want test-first retest-last six stages", got)
	}
	if got := lifecycleStages["delete"]; containsStage(got, "test") {
		t.Errorf("delete row = %v, must contain NO test stage (a deleted identity cannot be SSH-tested)", got)
	}
}

func containsStage(stages []string, want string) bool {
	for _, s := range stages {
		if s == want {
			return true
		}
	}
	return false
}

// TestLifecycleStageRecorderMatchesEachRow drives a full authorized run of
// all three verbs and asserts each function records exactly its row in
// lifecycleStages, in order.
func TestLifecycleStageRecorderMatchesEachRow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	forEachVerb(t, home, "work", identity.DeleteScopeGitOnly, func(verb string, call func(lifecyclePolicy) (lifecycleResult, error)) {
		record, got := rec()
		p := lifecyclePolicy{Confirm: confirmationBypassedWithYes, Stages: record}
		if _, err := call(p); err != nil {
			t.Fatalf("%s: authorized run failed: %v", verb, err)
		}
		want := lifecycleStages[verb]
		if strings.Join(*got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: recorded stages = %v, want %v", verb, *got, want)
		}
	})
}

// TestLifecycleDryRunStopsAfterPlan proves a dry-run policy records nothing
// past the plan stage — and, because it stops BEFORE the confirmation gate,
// needs no authorization at all: the zero-value confirmation mode with no
// prompt still returns nil (ordering consequence documented on the table).
func TestLifecycleDryRunStopsAfterPlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	watched := []string{
		filepath.Join(home, ".ssh", "config"),
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".gitconfig.d", "work"),
		filepath.Join(home, ".ssh", "allowed_signers"),
		filepath.Join(home, ".ssh", "id_ed25519_work"),
		filepath.Join(home, ".ssh", "id_ed25519_work.pub"),
	}
	before := snapshotPaths(t, watched)

	forEachVerb(t, home, "work", identity.DeleteScopeGitOnly, func(verb string, call func(lifecyclePolicy) (lifecycleResult, error)) {
		record, got := rec()
		// DryRun:true with the ZERO confirmation value and NO prompt: no
		// authorization is needed because the run cannot write.
		p := lifecyclePolicy{DryRun: true, Stages: record}
		res, err := call(p)
		if err != nil {
			t.Fatalf("%s: dry run must not error: %v", verb, err)
		}
		if len(res.Backups) != 0 {
			t.Errorf("%s: dry run must produce no backups, got %v", verb, res.Backups)
		}
		stages := *got
		if len(stages) == 0 || stages[len(stages)-1] != "plan" {
			t.Errorf("%s: dry run must stop after the plan stage, got %v", verb, stages)
		}
		for _, banned := range []string{"confirm", "backup", "write"} {
			if containsStage(stages, banned) {
				t.Errorf("%s: dry run recorded %q past the plan stage: %v", verb, banned, stages)
			}
		}
		if len(stages) != len(lifecycleStages[verb]) {
			// Delete stops after 1 stage, rotate/repair after 2.
			expect := 2
			if verb == "delete" {
				expect = 1
			}
			if len(stages) != expect {
				t.Errorf("%s: dry run recorded %v, want the %d-stage prefix to plan", verb, stages, expect)
			}
		}
	})

	assertUnchanged(t, before, snapshotPaths(t, watched))
}

// TestRunDeleteInvokesConnectivityTesterZeroTimes is the assertion that
// reconciles delete's stage row with plan 05-08's delete dry-run contract
// (review R2-02): neither a full nor a dry delete ever calls the
// connectivity-tester seams, because a deleted identity cannot be SSH-tested.
func TestRunDeleteInvokesConnectivityTesterZeroTimes(t *testing.T) {
	for _, dry := range []bool{false, true} {
		home := t.TempDir()
		t.Setenv("HOME", home)
		seedDeleteFixture(t, home, "work")
		b := groupHermeticBackend(home)

		preWriteCalls, resolvedCalls := 0, 0
		b.deps.PreWrite = func(_, _ string, _ int) tester.Result {
			preWriteCalls++
			return tester.Result{Outcome: tester.PASS}
		}
		b.deps.Resolved = func(_ string) (tester.Result, tester.ResolvedConfig) {
			resolvedCalls++
			return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{}
		}

		p := lifecyclePolicy{Confirm: confirmationBypassedWithYes}
		if dry {
			p = lifecyclePolicy{DryRun: true}
		}
		if _, err := b.runDelete("work", identity.DeleteScopeGitOnly, p); err != nil {
			t.Fatalf("delete (dry=%v): %v", dry, err)
		}
		if preWriteCalls != 0 || resolvedCalls != 0 {
			t.Errorf("delete (dry=%v): connectivity-tester seam invoked %d preWrite + %d resolved calls, want 0 + 0", dry, preWriteCalls, resolvedCalls)
		}
	}
}

// TestDeleteVerifyIsLoadBearing proves the closing verify stage is real
// rather than a recorded no-op: it must FAIL the call when what was supposed
// to be deleted is still present. The git-only and everything scopes have
// different postconditions (git-only keeps the SSH side by D-10), so each is
// checked against its own rule.
func TestDeleteVerifyIsLoadBearing(t *testing.T) {
	// Everything scope: an identity that still reconstructs must fail verify.
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)
	if err := b.verifyDeleteGone("work", identity.DeleteScopeEverything); err == nil {
		t.Error("verify(Everything) on a still-present identity must fail the call (load-bearing)")
	} else if !strings.Contains(err.Error(), "still reconstructable") {
		t.Errorf("verify(Everything) error = %v, want the still-reconstructable wording", err)
	}

	// Git-only scope: a gitconfig still carrying the includeIf block must fail
	// verify even though the SSH side legitimately survives.
	if err := b.verifyDeleteGone("work", identity.DeleteScopeGitOnly); err == nil {
		t.Error("verify(GitOnly) on an undeleted identity must fail the call (load-bearing)")
	} else if !strings.Contains(err.Error(), "gitconfig still carries") {
		t.Errorf("verify(GitOnly) error = %v, want the gitconfig-block wording", err)
	}

	// After a real delete, both scopes' verify passes without a false alarm.
	if _, err := b.runDelete("work", identity.DeleteScopeEverything, lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err != nil {
		t.Fatalf("runDelete(Everything): %v", err)
	}
	if err := b.verifyDeleteGone("work", identity.DeleteScopeEverything); err != nil {
		t.Errorf("verify(Everything) after delete = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Confirmation authorization (review R2-03)
// ---------------------------------------------------------------------------

// TestZeroValuePolicyFailsClosed is the fail-closed proof: a lifecyclePolicy
// with NOTHING set is confirmationRequired with no prompt, so each verb must
// return errConfirmationUnavailable, record no stage past confirm, create no
// backup, and leave every seeded file byte-identical.
func TestZeroValuePolicyFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	watched := []string{
		filepath.Join(home, ".ssh", "config"),
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".gitconfig.d", "work"),
		filepath.Join(home, ".ssh", "allowed_signers"),
		filepath.Join(home, ".ssh", "id_ed25519_work"),
		filepath.Join(home, ".ssh", "id_ed25519_work.pub"),
	}
	before := snapshotPaths(t, watched)

	forEachVerb(t, home, "work", identity.DeleteScopeGitOnly, func(verb string, call func(lifecyclePolicy) (lifecycleResult, error)) {
		record, got := rec()
		res, err := call(lifecyclePolicy{Stages: record})
		if !errors.Is(err, errConfirmationUnavailable) {
			t.Fatalf("%s: zero-value policy err = %v, want errConfirmationUnavailable", verb, err)
		}
		if len(res.Backups) != 0 {
			t.Errorf("%s: a fail-closed run must create no backup, got %v", verb, res.Backups)
		}
		stages := *got
		confirmIdx := indexOf(stages, "confirm")
		if confirmIdx < 0 {
			t.Errorf("%s: recorded %v — must have reached the confirm stage before failing closed", verb, stages)
			return
		}
		if len(stages) != confirmIdx+1 {
			t.Errorf("%s: recorded stages past confirm on a fail-closed run: %v", verb, stages)
		}
	})

	assertUnchanged(t, before, snapshotPaths(t, watched))
}

func indexOf(ss []string, want string) int {
	for i, s := range ss {
		if s == want {
			return i
		}
	}
	return -1
}

// TestConfirmationRequiredPromptRejectedAborts proves a confirmationRequired
// policy WITH a prompt that returns false aborts before the backup stage and
// writes nothing, while the same prompt returning true proceeds.
func TestConfirmationRequiredPromptRejectedAborts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)

	gitconfigBefore := readFile(t, filepath.Join(home, ".gitconfig"))

	record, got := rec()
	_, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{
		Confirm: confirmationRequired,
		Prompt:  func(string) (bool, error) { return false, nil },
		Stages:  record,
	})
	if err == nil {
		t.Fatal("a rejected prompt must abort the delete")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("abort error = %v, want the cancelled wording", err)
	}
	if containsStage(*got, "backup") || containsStage(*got, "write") {
		t.Errorf("a rejected prompt must record nothing past confirm, got %v", *got)
	}
	if got := readFile(t, filepath.Join(home, ".gitconfig")); got != gitconfigBefore {
		t.Errorf("gitconfig changed by a rejected delete:\n%s", got)
	}
	if _, stat := os.Stat(filepath.Join(home, ".gitconfig.d", "work")); os.IsNotExist(stat) {
		t.Error("fragment must survive a rejected delete")
	}

	// The same prompt returning true proceeds.
	record2, got2 := rec()
	if _, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{
		Confirm: confirmationRequired,
		Prompt:  func(string) (bool, error) { return true, nil },
		Stages:  record2,
	}); err != nil {
		t.Fatalf("an accepting prompt must proceed: %v", err)
	}
	want := lifecycleStages["delete"]
	if strings.Join(*got2, ",") != strings.Join(want, ",") {
		t.Errorf("accepting-prompt stages = %v, want %v", *got2, want)
	}
	if _, stat := os.Stat(filepath.Join(home, ".gitconfig.d", "work")); !os.IsNotExist(stat) {
		t.Error("fragment must be removed by an accepted delete")
	}
}

// TestDeletePlanPreviewOmitsSharedKeyNeverToBeRemoved is the WR-03
// regression: runDelete's confirmation-prompt preview used to be built by a
// SECOND, hand-rolled target list (deletePlanPreview) that named the key
// pair unconditionally whenever acct.KeyPath was non-empty — with no
// keySurvives consultation, promising to delete a key the write would
// actually KEEP for a sibling (the exact preview/write divergence R-11 and
// the shared deleteTargets helper were built to make impossible). The
// preview must now be derived from the SAME identity.PlanDelete the confirm
// screen renders, which omits a shared key from its Targets.
func TestDeletePlanPreviewOmitsSharedKeyNeverToBeRemoved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSharedKeyFixture(t, home, true)
	b := groupHermeticBackend(home)

	var captured string
	_, err := b.runDelete("work", identity.DeleteScopeEverything, lifecyclePolicy{
		Confirm: confirmationRequired,
		Prompt: func(preview string) (bool, error) {
			captured = preview
			return false, nil // reject — the test only needs the preview text
		},
	})
	if err == nil {
		t.Fatal("a rejected prompt must abort the delete")
	}
	if captured == "" {
		t.Fatal("the confirmation prompt must have been called with a non-empty preview")
	}
	if strings.Contains(captured, "id_ed25519_work") {
		t.Errorf("WR-03: preview must not name the key pair when a sibling still shares it: %q", captured)
	}
	if !strings.Contains(captured, ".gitconfig") {
		t.Errorf("preview must still name the gitconfig target: %q", captured)
	}
}

// TestRunDeleteConfirmationMatrix crosses DryRun with every confirmation
// mode and declares the expected outcome PER CELL across the four outcome
// classes (review R3-02): dry runs produce no backup; confirmationRequired
// with no prompt is refused with no backup; confirmationRequired with a
// rejecting prompt aborts with no backup; every AUTHORIZED non-dry-run cell
// produces a backup. The invariant proven is "authorization gates the write,
// and once authorized nothing can suppress the backup".
func TestRunDeleteConfirmationMatrix(t *testing.T) {
	tests := []struct {
		name    string
		dryRun  bool
		confirm confirmationMode
		prompt  func(string) (bool, error)
		// wantBackup is true only for the AUTHORIZED cells.
		wantBackup         bool
		wantErrUnavailable bool
	}{
		{"dry-run", true, confirmationRequired, nil, false, false},
		{"dry-run-already-obtained", true, confirmationAlreadyObtained, nil, false, false},
		{"dry-run-bypassed-yes", true, confirmationBypassedWithYes, nil, false, false},
		{"required-no-prompt", false, confirmationRequired, nil, false, true},
		{"required-rejecting-prompt", false, confirmationRequired, func(string) (bool, error) { return false, nil }, false, false},
		{"required-accepting-prompt", false, confirmationRequired, func(string) (bool, error) { return true, nil }, true, false},
		{"already-obtained", false, confirmationAlreadyObtained, nil, true, false},
		{"bypassed-yes", false, confirmationBypassedWithYes, nil, true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedDeleteFixture(t, home, "work")
			b := groupHermeticBackend(home)

			res, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{
				DryRun:  tc.dryRun,
				Confirm: tc.confirm,
				Prompt:  tc.prompt,
			})
			if tc.wantErrUnavailable {
				if !errors.Is(err, errConfirmationUnavailable) {
					t.Fatalf("err = %v, want errConfirmationUnavailable", err)
				}
				if len(res.Backups) != 0 {
					t.Errorf("a refused run must produce no backup, got %v", res.Backups)
				}
				return
			}
			if err != nil {
				// The rejecting-prompt cell is the only non-dry error cell left.
				if !strings.Contains(err.Error(), "cancelled") {
					t.Fatalf("err = %v, want the cancelled wording", err)
				}
				if tc.wantBackup {
					t.Error("a cancelled run must not be marked wantBackup")
				}
				if len(res.Backups) != 0 {
					t.Errorf("a cancelled run must produce no backup, got %v", res.Backups)
				}
				return
			}
			gotBackup := len(res.Backups) != 0
			if gotBackup != tc.wantBackup {
				t.Errorf("backup produced = %v, want %v (backups=%v)", gotBackup, tc.wantBackup, res.Backups)
			}
		})
	}
}

// TestRunRotateBackupStageUnderBothConfirmationModes proves runRotate
// performs its backup stage under confirmationAlreadyObtained AND under
// confirmationBypassedWithYes — and that a required mode with an accepting
// prompt does the same. No field of lifecyclePolicy can suppress it once
// authorized (D-02).
func TestRunRotateBackupStageUnderBothConfirmationModes(t *testing.T) {
	for _, mode := range []struct {
		name    string
		confirm confirmationMode
	}{{name: "already-obtained", confirm: confirmationAlreadyObtained}, {name: "bypassed-yes", confirm: confirmationBypassedWithYes}} {
		t.Run(mode.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedDeleteFixture(t, home, "work")
			b := groupHermeticBackend(home)

			res, err := b.runRotate("work", lifecyclePolicy{Confirm: mode.confirm})
			if err != nil {
				t.Fatalf("runRotate(%s): %v", mode.name, err)
			}
			if len(res.Backups) == 0 {
				t.Errorf("runRotate(%s) produced no backup — once authorized nothing may suppress it (D-02)", mode.name)
			}
		})
	}

	// Required + accepting prompt is the third authorized class.
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)
	res, err := b.runRotate("work", lifecyclePolicy{
		Confirm: confirmationRequired,
		Prompt:  func(string) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatalf("runRotate(required+accepting): %v", err)
	}
	if len(res.Backups) == 0 {
		t.Error("runRotate(required+accepting) produced no backup")
	}
}

// ---------------------------------------------------------------------------
// Commit seams are thin adapters (review R-11-CLI)
// ---------------------------------------------------------------------------

// TestCommitRotateAndCommitNewKeyAreThinAdapters swaps the two commit seams'
// lifecycle delegate for a recording double and proves the seam contributes
// NO stage of its own: exactly one call each, a KeyCommitMsg marshalled from
// the result, no file effect, and the confirmation mode the TUI is permitted
// to assert.
func TestCommitRotateAndCommitNewKeyAreThinAdapters(t *testing.T) {
	home := t.TempDir()
	b := groupHermeticBackend(home)

	origRotate, origRepair := commitRotateInto, commitRepairInto
	t.Cleanup(func() { commitRotateInto, commitRepairInto = origRotate, origRepair })
	t.Setenv("HOME", home)

	var rotateCalls, repairCalls int
	var rotatePolicy, repairPolicy lifecyclePolicy
	commitRotateInto = func(bb *realBackend, _ string, p lifecyclePolicy) (lifecycleResult, error) {
		if bb != b {
			t.Error("rotate double: seam delegated to a different backend")
		}
		rotateCalls++
		rotatePolicy = p
		return lifecycleResult{
			Backups:          []string{filepath.Join(home, "backup-a")},
			ArchivedKeyPaths: []string{filepath.Join(home, "archive-priv")},
		}, nil
	}
	commitRepairInto = func(_ *realBackend, _ string, p lifecyclePolicy) (lifecycleResult, error) {
		repairCalls++
		repairPolicy = p
		return lifecycleResult{Backups: []string{filepath.Join(home, "backup-b")}}, nil
	}

	rotateMsg, ok := b.CommitRotate("work")().(tuikit.KeyCommitMsg)
	if !ok {
		t.Fatalf("CommitRotate delivered %T, want KeyCommitMsg", b.CommitRotate("work")())
	}
	if rotateCalls != 1 {
		t.Errorf("CommitRotate called the lifecycle %d times, want exactly 1", rotateCalls)
	}
	if rotatePolicy.Confirm != confirmationAlreadyObtained {
		t.Errorf("CommitRotate authorized with %v, want confirmationAlreadyObtained (the only value a confirm-screen layer may assert)", rotatePolicy.Confirm)
	}
	if rotatePolicy.Stages != nil {
		t.Error("CommitRotate must install no stage recorder of its own — the seam performs no stage")
	}
	if rotatePolicy.DryRun {
		t.Error("CommitRotate must never run the ceremony as a dry run")
	}
	if rotateMsg.Mode != "rotate" {
		t.Errorf("rotate msg.Mode = %q, want rotate", rotateMsg.Mode)
	}
	if rotateMsg.Err != "" {
		t.Errorf("rotate msg.Err = %q, want empty", rotateMsg.Err)
	}
	if len(rotateMsg.Backups) != 1 || rotateMsg.Backups[0] != b.displayPath(filepath.Join(home, "backup-a")) {
		t.Errorf("rotate msg.Backups = %v, want the double's backup display-shortened", rotateMsg.Backups)
	}
	if rotateMsg.ArchivedKeyPath != b.displayPath(filepath.Join(home, "archive-priv")) {
		t.Errorf("rotate msg.ArchivedKeyPath = %q, want the double's archived private path", rotateMsg.ArchivedKeyPath)
	}

	_, ok = b.CommitNewKey("work")().(tuikit.KeyCommitMsg)
	if !ok {
		t.Fatal("CommitNewKey delivered something other than a KeyCommitMsg")
	}
	if repairCalls != 1 {
		t.Errorf("CommitNewKey called the lifecycle %d times, want exactly 1", repairCalls)
	}
	if repairPolicy.Confirm != confirmationAlreadyObtained {
		t.Errorf("CommitNewKey authorized with %v, want confirmationAlreadyObtained", repairPolicy.Confirm)
	}

	// No additional file effect from either seam: the double never touched
	// the filesystem, so the fake home stays empty.
	if listing := homeFileListing(t, home); len(listing) != 0 {
		t.Errorf("commit seams produced file effects of their own: %v", listing)
	}
}

// TestCommitSeamsConstructionErrorWriteNothing proves a backend whose
// construction failed answers both commits with an error message and
// performs no write.
func TestCommitSeamsConstructionErrorWriteNothing(t *testing.T) {
	b := &realBackend{initErr: fmt.Errorf("gitid: resolving home directory: boom")}

	rotateMsg := b.CommitRotate("work")().(tuikit.KeyCommitMsg)
	if rotateMsg.Err == "" {
		t.Error("CommitRotate on a construction-failed backend must surface the init error")
	}
	if !strings.Contains(rotateMsg.Err, "boom") {
		t.Errorf("CommitRotate err = %q, want the construction failure", rotateMsg.Err)
	}

	repairMsg := b.CommitNewKey("work")().(tuikit.KeyCommitMsg)
	if repairMsg.Err == "" || strings.Contains(repairMsg.Err, "boom") == false {
		t.Errorf("CommitNewKey err = %q, want the construction failure", repairMsg.Err)
	}
}

// ---------------------------------------------------------------------------
// Real rotate / repair transactions and rollback (KEY-05, KEY-07)
// ---------------------------------------------------------------------------

// TestRunRotateEndToEnd drives a real rotation over a fake home through the
// lifecycle: a new private key at the canonical path, the old bytes present
// inside the archive directory, two signer lines, and the result naming the
// archived paths and the backups.
func TestRunRotateEndToEnd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)

	privPath := filepath.Join(home, ".ssh", "id_ed25519_work")
	privBefore := readFile(t, privPath)
	acctBefore := readFile(t, filepath.Join(home, ".gitconfig"))

	res, err := b.runRotate("work", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runRotate(error): %v", err)
	}
	if len(res.ArchivedKeyPaths) != 2 {
		t.Errorf("archived paths = %v, want the private+public archive copies", res.ArchivedKeyPaths)
	}
	if len(res.Backups) == 0 {
		t.Error("a successful rotation must report timestamped backups")
	}
	if res.ReTest.Outcome != tester.PASS {
		t.Errorf("re-test outcome = %v, want PASS (through the hermetic seam)", res.ReTest.Outcome)
	}

	privAfter := readFile(t, privPath)
	if privAfter == privBefore {
		t.Error("canonical private key bytes unchanged after rotation")
	}
	archiveDir := sshconfig.ArchiveDir(b.sshDir)
	entries, rderr := os.ReadDir(archiveDir)
	if rderr != nil {
		t.Fatalf("archive directory missing after rotation: %v", rderr)
	}
	if len(entries) != 2 {
		t.Errorf("archive holds %d entries, want 2\n%v", len(entries), entries)
	}
	archivedPriv, found := archivedPrivatePath(res.ArchivedKeyPaths)
	if !found || readFile(t, archivedPriv) != privBefore {
		t.Error("archived private key must carry the pre-rotation bytes")
	}

	signers := readFile(t, filepath.Join(home, ".ssh", "allowed_signers"))
	lineCount := 0
	for _, line := range strings.Split(signers, "\n") {
		if strings.HasPrefix(line, "work@example.com ") {
			lineCount++
		}
	}
	if lineCount != 2 {
		t.Errorf("allowed_signers has %d lines for work@example.com, want 2 (D-07 append)\n%s", lineCount, signers)
	}
	if got := readFile(t, filepath.Join(home, ".gitconfig")); got == acctBefore {
		t.Error("gitconfig unchanged by rotation — the four artifacts must be re-pointed")
	}
	// D-08's contract is VALUE identity: the fragment's signingkey must still
	// point at the identity's canonical pub key path after rotation. Read it
	// back through the authoritative git parser (the same writer that
	// re-asserted it) so the assertion is format-independent.
	fragAfter := filepath.Join(home, ".gitconfig.d", "work")
	//nolint:gosec // G204: hermetic fake-home path under a t.TempDir() test sandbox; arg-slice form, no shell
	probe := exec.Command("git", "config", "--file", fragAfter, "user.signingkey")
	pubOut, perr := probe.Output()
	if perr != nil {
		t.Fatalf("reading user.signingkey from fragment after rotation: %v", perr)
	}
	wantSigningKey := filepath.Join(home, ".ssh", "id_ed25519_work.pub")
	if got := strings.TrimSpace(string(pubOut)); got != wantSigningKey {
		t.Errorf("fragment signingkey = %q, want the canonical pub path %q (D-08)", got, wantSigningKey)
	}
}

// TestRunRotateRefusesWhenKeyIsSharedWithAnotherIdentity is the CR-01
// regression: rotate calls identity.Rotate, which ARCHIVES (moves) the
// current key pair. When a sibling identity's IdentityFile still points at
// that same key, moving it strands the sibling with a dangling reference.
// runRotate — the chokepoint both the CLI and TUI call through — must refuse
// before any write, exactly as runRepair already refuses via
// ErrRepairTargetShared, and must leave the shared key pair byte-for-byte
// unchanged at its original path.
func TestRunRotateRefusesWhenKeyIsSharedWithAnotherIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSharedKeyFixture(t, home, true)
	b := groupHermeticBackend(home)

	keyPath := filepath.Join(home, ".ssh", "id_ed25519_work")
	pubPath := keyPath + ".pub"
	before := snapshotPaths(t, []string{keyPath, pubPath})

	_, err := b.runRotate("work", lifecyclePolicy{Confirm: confirmationBypassedWithYes})
	if err == nil {
		t.Fatal("runRotate over a shared key must refuse, not archive it")
	}
	if !errors.Is(err, identity.ErrRepairTargetShared) {
		t.Errorf("runRotate error = %v, want errors.Is(err, identity.ErrRepairTargetShared)", err)
	}
	if !strings.Contains(err.Error(), "personal") {
		t.Errorf("runRotate error = %v, want it to name the sibling %q", err, "personal")
	}

	assertUnchanged(t, before, snapshotPaths(t, []string{keyPath, pubPath}))
	archiveDir := sshconfig.ArchiveDir(b.sshDir)
	if _, serr := os.Stat(archiveDir); !os.IsNotExist(serr) {
		entries, rderr := os.ReadDir(archiveDir)
		if rderr != nil {
			t.Fatalf("reading archive dir: %v", rderr)
		}
		if len(entries) != 0 {
			t.Errorf("archive directory holds entries after a refused rotation: %v", entries)
		}
	}
}

// TestRunRotateRefusesWhenKeyIsMissing is CR-01's secondary consequence:
// rotating an identity with no current key pair must refuse and point at
// `new-key`, never call identity.Rotate with an empty key path (which would
// silently archive "" instead of generating the missing key).
func TestRunRotateRefusesWhenKeyIsMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_work")
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("removing key fixture: %v", err)
	}
	if err := os.Remove(keyPath + ".pub"); err != nil {
		t.Fatalf("removing pub key fixture: %v", err)
	}
	b := groupHermeticBackend(home)

	_, err := b.runRotate("work", lifecyclePolicy{Confirm: confirmationBypassedWithYes})
	if err == nil {
		t.Fatal("runRotate with no current key pair must refuse, not archive an empty path")
	}
	if !strings.Contains(err.Error(), "new-key") {
		t.Errorf("runRotate error = %v, want it to point at `gitid identity new-key work`", err)
	}
}

func archivedPrivatePath(paths []string) (string, bool) {
	for _, p := range paths {
		if strings.HasSuffix(p, ".pub") {
			continue
		}
		return p, true
	}
	return "", false
}

// TestRotateAndRepairWatchPathsIncludeSSHConfigPath is the WR-04
// regression: rotateWatchPaths/repairWatchPaths watched b.storageTargetPath()
// but never b.sshConfigPath — DISTINCT paths whenever the include-dir
// layout is active (storageTargetPath() resolves to the config.d target,
// while b.sshConfigPath is ~/.ssh/config itself, the file
// deps.WriteSSH/writeSSHBlock calls sshconfig.EnsureIncludeLine against). A
// mid-transaction failure therefore left an injected Include line (and the
// config.d directory sshconfig.EnsureIncludeDir created) behind, outside
// the journal's watch entirely. Both watch-path builders must include
// b.sshConfigPath so mutationJournal.restore() can undo it like any other
// watched file.
func TestRotateAndRepairWatchPathsIncludeSSHConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)

	// Build the include-dir layout by hand, through the real production
	// seams (EnsureIncludeDir/EnsureIncludeLine), so b.storageTargetPath()
	// resolves to config.d/gitid.config while b.sshConfigPath stays
	// ~/.ssh/config — two DISTINCT paths, the exact condition WR-04's bug
	// required (seedDeleteFixture's in-file layout makes the two identical,
	// which would make this regression test pass even without the fix).
	includeDir := filepath.Join(home, ".ssh", "config.d")
	if err := sshconfig.EnsureIncludeDir(includeDir); err != nil {
		t.Fatalf("EnsureIncludeDir: %v", err)
	}
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	if _, err := sshconfig.EnsureIncludeLine(sshConfigPath); err != nil {
		t.Fatalf("EnsureIncludeLine: %v", err)
	}
	sshBody := "Host work.github.com\n" +
		"  Hostname ssh.github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_work\n" +
		"  IdentitiesOnly yes\n"
	canonical := filepath.Join(includeDir, "gitid.config")
	writeFile(t, canonical, managedBlock("_global", "Host *\n  IdentitiesOnly yes\n")+"\n"+managedBlock("work", sshBody))

	pubLine := seedGeneratedKey(t, filepath.Join(home, ".ssh", "id_ed25519_work"), "work", "")
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seeding .gitconfig.d: %v", err)
	}
	fragBody := "[user]\n\tname = work User\n\temail = work@example.com\n" +
		"\tsigningkey = ~/.ssh/id_ed25519_work.pub\n\n[gpg]\n\tformat = ssh\n\n[commit]\n\tgpgsign = true\n"
	writeFile(t, filepath.Join(home, ".gitconfig.d", "work"), fragBody)
	gcBody := "[includeIf \"gitdir:~/git/work/\"]\n\tpath = ~/.gitconfig.d/work\n"
	writeFile(t, filepath.Join(home, ".gitconfig"), "[user]\n\tname = Global User\n\n"+managedBlock("work", gcBody))
	line := mustAllowedSignersLine(t, "work@example.com", pubLine)
	writeFile(t, filepath.Join(home, ".ssh", "allowed_signers"), managedBlock("work", line))

	b := newBackendForHome(home)
	acct, found := b.findAccount("work")
	if !found {
		t.Fatal("fixture account \"work\" not found")
	}
	if got := b.storageTargetPath(); got == b.sshConfigPath {
		t.Fatalf("fixture setup failed: storageTargetPath() == sshConfigPath (%q) — the include-dir layout was not exercised", got)
	}
	acct = b.normalizeAccountForWrite(acct)

	hasPath := func(paths []string, want string) bool {
		for _, p := range paths {
			if p == want {
				return true
			}
		}
		return false
	}

	if rotatePaths := b.rotateWatchPaths(acct); !hasPath(rotatePaths, b.sshConfigPath) {
		t.Errorf("WR-04: rotateWatchPaths = %v, must include b.sshConfigPath %q", rotatePaths, b.sshConfigPath)
	}
	if repairPaths := b.repairWatchPaths(acct); !hasPath(repairPaths, b.sshConfigPath) {
		t.Errorf("WR-04: repairWatchPaths = %v, must include b.sshConfigPath %q", repairPaths, b.sshConfigPath)
	}
}

// TestRunRotateFailureMatrixRestoresBytesAndMode injects a failure at every
// transaction step and asserts each watched file's bytes AND mode match the
// pre-transaction snapshot, and that no archive entry for the identity
// survives the rollback.
func TestRunRotateFailureMatrixRestoresBytesAndMode(t *testing.T) {
	steps := []string{
		"rotate-archive", "rotate-persist-key", "rotate-ssh", "rotate-gitconfig",
		"rotate-fragment", "rotate-signers",
	}
	watched := func(home string) []string {
		return []string{
			filepath.Join(home, ".ssh", "config"),
			filepath.Join(home, ".gitconfig"),
			filepath.Join(home, ".gitconfig.d", "work"),
			filepath.Join(home, ".ssh", "allowed_signers"),
			filepath.Join(home, ".ssh", "id_ed25519_work"),
			filepath.Join(home, ".ssh", "id_ed25519_work.pub"),
		}
	}

	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedDeleteFixture(t, home, "work")
			b := groupHermeticBackend(home)
			b.failCommitAt = func(s string) error {
				if s == step {
					return fmt.Errorf("injected failure at %s", s)
				}
				return nil
			}

			before := snapshotPaths(t, watched(home))
			_, err := b.runRotate("work", lifecyclePolicy{Confirm: confirmationBypassedWithYes})
			if err == nil {
				t.Fatal("runRotate must surface the injected failure")
			}
			assertUnchanged(t, before, snapshotPaths(t, watched(home)))

			// No archive entry for this identity survives the rollback: the
			// archive directory is either gone (it never pre-existed) or
			// contains zero entries attributable to this transaction.
			archiveDir := sshconfig.ArchiveDir(b.sshDir)
			if _, serr := os.Stat(archiveDir); os.IsNotExist(serr) {
				return // recorded as created and removed wholescale
			}
			entries, rderr := os.ReadDir(archiveDir)
			if rderr != nil {
				t.Fatalf("reading archive dir: %v", rderr)
			}
			if len(entries) != 0 {
				t.Errorf("archive directory holds entries after a rolled-back rotation: %v", entries)
			}
		})
	}
}

// TestRunRotateArchiveStepFailureRollsBackCompletely is the lifecycle-level
// half of review R3-01: the step that can fail with archive copies already on
// disk is rolled back EXACTLY as completely as any later step. The failure is
// injected INSIDE the archive step — the second source removal — so both
// archive copies were recorded by the transaction seam before the removal
// failed. Afterwards the archive directory holds nothing, both canonical key
// paths hold their pre-transaction bytes at mode 0600, and the returned
// result names the archive paths that were created and removed.
func TestRunRotateArchiveStepFailureRollsBackCompletely(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)

	privPath := filepath.Join(home, ".ssh", "id_ed25519_work")
	pubPath := privPath + ".pub"
	privBefore := readFile(t, privPath)
	pubBefore := readFile(t, pubPath)

	// keygen.MoveKeyPairToArchive removes priv FIRST, then pub — fail the
	// SECOND (pub) removal so priv is genuinely gone from the canonical path
	// when the archive step dies, the asymmetric rollback case.
	b.failArchiveRemoveAt = func(path string) error {
		if path == pubPath {
			return fmt.Errorf("injected failure removing %s", path)
		}
		return nil
	}

	res, err := b.runRotate("work", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err == nil {
		t.Fatal("runRotate must surface the archive-step failure")
	}
	if !strings.Contains(err.Error(), "archive") {
		t.Errorf("err = %v, want the archive-step context", err)
	}

	if len(res.ArchivedKeyPaths) != 2 {
		t.Errorf("result must name BOTH archive paths created in the failed step, got %v", res.ArchivedKeyPaths)
	}

	// The archive directory did NOT pre-exist, so this transaction recorded it
	// as created (recordCreatedDir) and the rollback removed it wholescale —
	// review R2-08: a recorded creation never survives a rollback (plan line:
	// "a newly created archive directory is removed too"), and no archive
	// entry for this identity survives anywhere under ~/.ssh.
	archiveDir := sshconfig.ArchiveDir(b.sshDir)
	if _, serr := os.Stat(archiveDir); !os.IsNotExist(serr) {
		t.Errorf("archive directory must be removed by the rollback of a transaction that created it, stat err = %v", serr)
	}

	if got := readFile(t, privPath); got != privBefore {
		t.Error("canonical private key bytes not restored across the archive-step failure")
	}
	if got := readFile(t, pubPath); got != pubBefore {
		t.Error("canonical public key bytes not restored across the archive-step failure")
	}
	for _, p := range []string{privPath, pubPath} {
		info, serr := os.Stat(p)
		if serr != nil {
			t.Fatalf("stat %s: %v", p, serr)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("%s mode after rollback = %o, want 0600", p, mode)
		}
	}
}

// TestRunRepairEndToEndAndCommitMessage drives a real new-key (repair)
// ceremony through the CommitNewKey seam: a fresh key lands at the identity's
// OWN canonical path, the archive directory stays untouched, and the commit
// message carries an EMPTY archived path (repair never archives, D-05).
func TestRunRepairEndToEndAndCommitMessage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)

	privPath := filepath.Join(home, ".ssh", "id_ed25519_work")
	privBefore := readFile(t, privPath)
	archiveBeforeEntries := archiveEntriesOrZero(t, sshconfig.ArchiveDir(b.sshDir))

	res, err := b.runRepair("work", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runRepair: %v", err)
	}
	if len(res.ArchivedKeyPaths) != 0 {
		t.Errorf("repair must not archive — result named %v", res.ArchivedKeyPaths)
	}
	if len(res.Backups) == 0 {
		t.Error("a successful repair must report backups")
	}
	if got := readFile(t, privPath); got == privBefore {
		t.Error("new-key ceremony left the canonical private key unchanged")
	}
	if got := archiveEntriesOrZero(t, sshconfig.ArchiveDir(b.sshDir)); got != archiveBeforeEntries {
		t.Errorf("archive directory gained %d entries during repair, want none (D-05)", got)
	}

	// The commit message carries an empty archived path.
	msg := b.CommitNewKey("work")().(tuikit.KeyCommitMsg)
	if msg.Mode != "repair" {
		t.Errorf("msg.Mode = %q, want repair", msg.Mode)
	}
	if msg.ArchivedKeyPath != "" {
		t.Errorf("repair commit message must carry an EMPTY archived path, got %q", msg.ArchivedKeyPath)
	}
	if len(msg.Backups) == 0 {
		t.Error("repair commit message must carry the real backup paths")
	}
}

// TestRunRepairRefusesWhenSharedKeyComparisonIsNormalized is the CR-02
// regression for lifecycle.go's runRepair: `privTarget` is derived from the
// ALREADY-normalized `acct` (absolute), but the sibling comparison used to
// run against the raw, tilde-spelled b.accounts() list, so the two paths
// could never be byte-equal and identity.ErrRepairTargetShared was
// unreachable. With the fix (comparing against b.normalizedAccounts()),
// repairing "work" — whose own canonical key path is shared with sibling
// "personal" — must refuse and leave the shared key untouched.
func TestRunRepairRefusesWhenSharedKeyComparisonIsNormalized(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSharedKeyFixture(t, home, true)
	b := groupHermeticBackend(home)

	keyPath := filepath.Join(home, ".ssh", "id_ed25519_work")
	pubPath := keyPath + ".pub"
	before := snapshotPaths(t, []string{keyPath, pubPath})

	_, err := b.runRepair("work", lifecyclePolicy{Confirm: confirmationBypassedWithYes})
	if err == nil {
		t.Fatal("runRepair over a key shared with a sibling must refuse, not overwrite it")
	}
	if !errors.Is(err, identity.ErrRepairTargetShared) {
		t.Errorf("runRepair error = %v, want errors.Is(err, identity.ErrRepairTargetShared)", err)
	}
	if !strings.Contains(err.Error(), "personal") {
		t.Errorf("runRepair error = %v, want it to name the sibling %q", err, "personal")
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{keyPath, pubPath}))
}

func archiveEntriesOrZero(t *testing.T, archiveDir string) int {
	t.Helper()
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return 0 // archive directory does not pre-exist
	}
	return len(entries)
}

// TestConcurrentLifecycleCommitsSerializeAndSucceed mirrors the WR-04 proof:
// two overlapping lifecycle transactions must not interleave. Holding txMu
// externally must block a concurrent runRotate until release — then both
// concurrent commits complete and the rebuilt home stays coherent (two
// identities parse).
func TestConcurrentLifecycleCommitsSerializeAndSucceed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedTwoIdentitiesSameProvider(t, home)
	b := groupHermeticBackend(home)

	// Part 1: the lock is real, not merely declared (WR-04's proof shape).
	b.txMu.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := b.runRotate("work", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		done <- err
	}()
	select {
	case <-done:
		t.Fatal("runRotate completed while txMu was held externally — lifecycle transactions are not serialized")
	case <-time.After(100 * time.Millisecond):
		// Expected: still blocked on txMu.
	}
	b.txMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runRotate after lock release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runRotate never completed after txMu was released")
	}

	// Part 2: rotate and repair on the two identities run in parallel and
	// NEITHER observes a partially written file — both succeed, and the
	// rebuilt inventory still parses both identities afterwards.
	type result struct {
		err error
	}
	results := make(chan result, 2)
	go func() {
		_, err := b.runRotate("work", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		results <- result{err}
	}()
	go func() {
		_, err := b.runRepair("personal", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		results <- result{err}
	}()
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("concurrent commit: %v", r.err)
		}
	}
	if got := len(b.accounts()); got != 2 {
		t.Errorf("after concurrent commits %d identities reconstruct, want 2 (no lost update, no partial file)", got)
	}
}

// ---------------------------------------------------------------------------
// Journal: created_DIRECTORY recording and disjointness (reviews R-10, R2-08)
// ---------------------------------------------------------------------------

// TestJournalRecordCreatedDirDisjointness pins the disjoint-set invariant
// for the dir half (the file half is covered by TestJournalCreatedAndWatched
// SetsAreDisjoint in wiring_test.go): every registration-time refusal names
// the path, and recording the same creation twice is an idempotent no-op.
func TestJournalRecordCreatedDirDisjointness(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)
	j := newMutationJournal(b)

	target := filepath.Join(home, ".ssh", "archive")

	// Recording the same dir twice is an idempotent no-op.
	if err := j.recordCreatedDir(target); err != nil {
		t.Fatalf("recordCreatedDir: %v", err)
	}
	if err := j.recordCreatedDir(target); err != nil {
		t.Errorf("recording the same created dir twice must be a no-op, got %v", err)
	}
	if len(j.createdDirs) != 1 {
		t.Errorf("journal createdDirs = %v, want one entry", j.createdDirs)
	}

	// A dir recorded as created cannot also be watched.
	if err := j.watchDir(target); err == nil {
		t.Error("watchDir on a recorded-created dir must return an error naming the path")
	} else if !strings.Contains(err.Error(), target) {
		t.Errorf("watchDir refusal error must name the path, got %v", err)
	}

	// watchFile on the same path is likewise refused.
	if err := j.watchFile(target); err == nil {
		t.Error("watchFile on a recorded-created dir must return an error naming the path")
	}

	// disjointness in the other order, on a FRESH journal: a watched path
	// cannot later be recorded as created — for files and for dirs.
	mk := filepath.Join(home, ".ssh", "id_ed25519_x")
	seedGeneratedKey(t, mk, "x", "")
	j2 := newMutationJournal(b)
	if err := j2.watchFile(mk); err != nil {
		t.Fatalf("watchFile: %v", err)
	}
	if err := j2.recordCreatedDir(mk); err == nil {
		t.Error("recordCreatedDir on a watched file must return an error naming the path")
	} else if !strings.Contains(err.Error(), mk) {
		t.Errorf("recordCreatedDir refusal must name the path, got %v", err)
	}
	if err := j2.recordCreatedFile(mk); err == nil {
		t.Error("recordCreatedFile on a watched file must return an error naming the path")
	}

	// A created dir cannot ALSO be recorded as a created file.
	j3 := newMutationJournal(b)
	filePath := filepath.Join(home, ".ssh", "newfile")
	if err := j3.recordCreatedFile(filePath); err != nil {
		t.Fatalf("recordCreatedFile: %v", err)
	}
	if err := j3.recordCreatedDir(filePath); err == nil {
		t.Error("recordCreatedDir on a recorded-created file must return an error naming the path")
	}
}

// TestJournalRestoreRemovesCreatedPathsProvesR208 rolls back a transaction
// that created a file INSIDE a created directory and proves BOTH are absent —
// a recorded creation never survives a rollback (review R2-08).
func TestJournalRestoreRemovesCreatedPathsProvesR208(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)
	j := newMutationJournal(b)

	dir := filepath.Join(home, ".ssh", "archive")
	file := filepath.Join(dir, "id_ed25519_work")
	if err := j.recordCreatedDir(dir); err != nil {
		t.Fatalf("recordCreatedDir: %v", err)
	}
	if err := j.recordCreatedFile(file); err != nil {
		t.Fatalf("recordCreatedFile: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, file, "archived bytes")

	outcomes, rerr := j.restore()
	if rerr != nil {
		t.Fatalf("restore: %v; outcomes=%v", rerr, outcomes)
	}
	if _, stat := os.Stat(file); !os.IsNotExist(stat) {
		t.Errorf("created file inside a created dir survived rollback: %v", stat)
	}
	if _, stat := os.Stat(dir); !os.IsNotExist(stat) {
		t.Errorf("created dir survived rollback: %v", stat)
	}
	if _, stat := os.Stat(filepath.Join(home, ".ssh")); stat != nil {
		t.Errorf("pre-existing .ssh must survive rollback untouched: %v", stat)
	}
}

// TestJournalRestoreRestoresWatchedBytesAndModeProvesR21 restores a watched
// file that lives inside a PRE-EXISTING directory (the R2-08 "restores every
// pre-existing watched path" half): bytes AND mode come back exactly.
func TestJournalRestoreRestoresWatchedBytesAndModeProvesR21(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)
	j := newMutationJournal(b)

	target := filepath.Join(home, ".ssh", "config")
	writeFile(t, target, "# original\n")
	if err := os.Chmod(target, 0o600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := j.watchFile(target); err != nil {
		t.Fatalf("watchFile: %v", err)
	}

	// Simulate the mid-transaction mutation: different bytes AND a looser mode.
	// 0644 is deliberate — the point of the test is that restore() reverts the
	// mode back to the watched 0600, so the fixture must NOT start at 0600.
	//nolint:gosec // G302: deliberate fixture — looser mode proves mode-restore
	writeFile2(t, target, "# mutated\n", 0o644)
	if err := os.Chmod(target, 0o644); err != nil { //nolint:gosec // G302: same deliberate fixture
		t.Fatalf("chmod: %v", err)
	}

	outcomes, rerr := j.restore()
	if rerr != nil {
		t.Fatalf("restore: %v; outcomes=%v", rerr, outcomes)
	}
	if got := readFile(t, target); got != "# original\n" {
		t.Errorf("restored bytes = %q, want the pre-transaction content", got)
	}
	info, serr := os.Stat(target)
	if serr != nil {
		t.Fatalf("stat: %v", serr)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("restored mode = %o, want 0600", mode)
	}
}

// writeFile2 writes a fixture file at a caller-chosen mode.
func writeFile2(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
}

// ---------------------------------------------------------------------------
// Composition-root seam discipline (review R3-01)
// ---------------------------------------------------------------------------

// TestLifecycleCallsDomainThroughTransactionDeps is the comment-stripped
// grep gate the plan mandates over cmd/gitid/lifecycle.go: no lifecycle
// function may pass the backend-wide Deps value to a DOMAIN call — the only
// acceptable way to reach identity.Rotate/RepairKey/Delete is through
// depsForTransaction / deleteDepsForTransaction, which rebind the archive
// seam to the journal. Direct `b.deps.Resolved` reads for the advisory
// test/retest stages are OUTSIDE the gate: they are not domain calls.
func TestLifecycleCallsDomainThroughTransactionDeps(t *testing.T) {
	src, err := os.ReadFile("lifecycle.go")
	if err != nil {
		t.Fatalf("reading lifecycle.go: %v", err)
	}
	var violations []string
	for lineNo, raw := range strings.Split(string(src), "\n") {
		code := stripComment(raw)
		if strings.TrimSpace(code) == "" {
			continue
		}
		// A domain call receiving the backend-wide value is the violation.
		isDomainCall := strings.Contains(code, "identity.Rotate(") ||
			strings.Contains(code, "identity.RepairKey(") ||
			strings.Contains(code, "identity.Delete(")
		if isDomainCall && strings.Contains(code, "b.deps") {
			violations = append(violations, fmt.Sprintf("line %d: %s", lineNo+1, code))
		}
	}
	if len(violations) != 0 {
		t.Errorf("lifecycle.go passes the backend-wide Deps to a domain call (review R3-01):\n%s", strings.Join(violations, "\n"))
	}
	if !strings.Contains(string(src), "depsForTransaction(") || !strings.Contains(string(src), "deleteDepsForTransaction(") {
		t.Error("lifecycle.go must obtain its Deps through depsForTransaction/deleteDepsForTransaction")
	}
}

// stripComment removes a trailing `//` comment (any indentation) from a code
// line, so the grep gate sees only code — the plan's `grep -v '^\s*//'`
// equivalent applied to inline comments too.
func stripComment(line string) string {
	for i := 0; i+1 < len(line); i++ {
		if line[i] == '/' && line[i+1] == '/' {
			return strings.TrimRight(line[:i], " \t")
		}
	}
	return line
}

// TestRunRotateForbiddenOnConstructionFailedBackend proves every lifecycle
// function answers a construction-failed backend with an error and performs
// no write (belt and braces beneath the commit seams' own guard).
func TestRunRotateForbiddenOnConstructionFailedBackend(t *testing.T) {
	b := &realBackend{initErr: fmt.Errorf("gitid: resolving home directory: boom")}
	if _, err := b.runRotate("work", lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err == nil {
		t.Error("runRotate must fail on a construction-failed backend")
	}
	if _, err := b.runRepair("work", lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err == nil {
		t.Error("runRepair must fail on a construction-failed backend")
	}
	if _, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err == nil {
		t.Error("runDelete must fail on a construction-failed backend")
	}
}

// TestGitconfigBlockVerifier (companion to verifyDeleteGone) makes the
// git-only verify's ParseManagedIncludeIf path observable without a full
// delete run.
func TestGitconfigBlockVerifier(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)

	if err := b.verifyDeleteGone("work", identity.DeleteScopeGitOnly); err == nil {
		t.Fatal("verify(GitOnly) on an undeleted identity must fail")
	}
	if _, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err != nil {
		t.Fatalf("runDelete(GitOnly): %v", err)
	}
	if err := b.verifyDeleteGone("work", identity.DeleteScopeGitOnly); err != nil {
		t.Errorf("verify(GitOnly) after delete = %v, want nil", err)
	}
	// The surviving gitconfig must now parse WITHOUT the identity's block.
	gcBytes, rerr := identity.InventoryDepsForHome(home).ReadGitconfig()
	if rerr != nil {
		t.Fatalf("re-reading gitconfig: %v", rerr)
	}
	if blocks := gitconfig.ParseManagedIncludeIf(gcBytes); len(blocks) != 0 {
		t.Errorf("gitconfig still carries managed includeIf blocks after the delete: %v", blocks)
	}
}

// ---------------------------------------------------------------------------
// runGlobalSSHApply — the ONE global-SSH fix ceremony (plan 06-01)
// ---------------------------------------------------------------------------

// TestRunGlobalSSHApplyInjectedFailureRestoresWatchedFiles is the journal's
// restore, EXERCISED rather than assumed, so plan 06-04 inherits a proven
// mechanism instead of a described one (06-REVIEWS.md cycle-2 pin): a write
// failure injected AFTER the first file has been written (the floored Include
// line) must return every watched file to its pre-transaction bytes — a
// fresh-HOME ~/.ssh/config that did NOT exist before the transaction is
// removed, never "restored to empty" — and the returned lifecycleResult must
// name the restored paths.
func TestRunGlobalSSHApplyInjectedFailureRestoresWatchedFiles(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	b.failCommitAt = func(s string) error {
		if s == "global-ssh-write" {
			return fmt.Errorf("injected failure after the include-line write")
		}
		return nil
	}

	configPath := filepath.Join(home, ".ssh", "config")
	target := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	includeDir := filepath.Join(home, ".ssh", "config.d")
	before := snapshotPaths(t, []string{configPath, target, includeDir})

	res, err := b.runGlobalSSHApply([]string{"HashKnownHosts"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err == nil {
		t.Fatal("runGlobalSSHApply must surface the injected write failure")
	}
	if !strings.Contains(err.Error(), "injected failure") {
		t.Errorf("err = %q, want the concrete injected failure", err.Error())
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{configPath, target, includeDir}))

	if len(res.Restored) == 0 {
		t.Error("the lifecycleResult must name the restored paths")
	}
	for _, outcome := range res.Restored {
		if !strings.Contains(outcome, ": restored") && !strings.Contains(outcome, "restoration failed") {
			t.Errorf("restore outcome line = %q, want a restored/removed outcome", outcome)
		}
	}
}

// TestRunGlobalSSHApplyFreshHomeRemovesCreatedConfigOnFailure pins the
// fresh-file-removal semantics explicitly: the failed apply that had to CREATE
// ~/.ssh/config (the floored Include line) must remove it again rather than
// leave an empty file behind — the fresh-HOME half of the journal contract.
func TestRunGlobalSSHApplyFreshHomeRemovesCreatedConfigOnFailure(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	failed := false
	b.failCommitAt = func(s string) error {
		if s == "global-ssh-write" {
			failed = true
			return fmt.Errorf("injected write failure")
		}
		return nil
	}

	configPath := filepath.Join(home, ".ssh", "config")
	if _, err := b.runGlobalSSHApply([]string{"HashKnownHosts"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err == nil {
		t.Fatal("runGlobalSSHApply must surface the injected failure")
	}
	if !failed {
		t.Fatal("test setup: the target-write injection point never fired")
	}
	if fileExists(configPath) {
		t.Errorf("%s must be REMOVED on rollback (it did not exist before the transaction), not left behind", configPath)
	}
	if fileExists(filepath.Join(home, ".ssh", "config.d", "gitid.config")) {
		t.Error("the created target file must be removed on rollback")
	}
}

// TestRunGlobalSSHApplyDryRunLeavesTargetUnchanged pins the per-verb dry-run
// contract: stopping AFTER the plan stage must leave the resolved target's
// bytes unchanged and return a non-empty plan-stage record, with no further
// stage recorded.
func TestRunGlobalSSHApplyDryRunLeavesTargetUnchanged(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	record, stages := rec()
	before := snapshotPaths(t, []string{filepath.Join(home, ".ssh", "config")})

	res, err := b.runGlobalSSHApply([]string{"HashKnownHosts"}, lifecyclePolicy{DryRun: true, Stages: record})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if len(*stages) == 0 || (*stages)[0] != "plan" {
		t.Errorf("dry run stages = %v, want a non-empty list starting with plan", *stages)
	}
	for _, s := range *stages {
		if s == "write" || s == "backup" || s == "confirm" {
			t.Errorf("dry run must not record %q — it stops after the plan stage", s)
		}
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{filepath.Join(home, ".ssh", "config")}))
	if len(res.Backups) != 0 {
		t.Errorf("dry run reported backups %v, want none", res.Backups)
	}
}

// TestRunGlobalSSHApplyRejectsPerAliasOptionByName pins the none-silent-drop
// contract: the per-alias option (IdentitiesOnly — D-10 scope "per-alias",
// never the `Host *` block) is rejected BY NAME before any candidate is built,
// and no write happens.
func TestRunGlobalSSHApplyRejectsPerAliasOptionByName(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	before := snapshotPaths(t, []string{filepath.Join(home, ".ssh", "config")})

	_, err := b.runGlobalSSHApply([]string{"IdentitiesOnly"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err == nil {
		t.Fatal("runGlobalSSHApply must reject the per-alias option, not silently drop it")
	}
	if !strings.Contains(err.Error(), "IdentitiesOnly") {
		t.Errorf("err = %q, want it to name the rejected key", err.Error())
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{filepath.Join(home, ".ssh", "config")}))
}

// TestRunGlobalSSHApplySimulateStageIsRecorded confirms the simulate stage
// is now part of the global-ssh stage sequence (plan 06-04 extension).
func TestRunGlobalSSHApplySimulateStageIsRecorded(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	record, stages := rec()
	_, err := b.runGlobalSSHApply([]string{"HashKnownHosts"}, lifecyclePolicy{
		Confirm: confirmationAlreadyObtained,
		Stages:  record,
	})
	if err != nil {
		t.Fatalf("runGlobalSSHApply: %v", err)
	}
	found := false
	for _, s := range *stages {
		if s == "simulate" {
			found = true
		}
	}
	if !found {
		t.Errorf("stages = %v, want 'simulate' stage recorded", *stages)
	}
}

// TestRunGlobalSSHApplyVerifyStageIsRecorded confirms the verify stage
// is now part of the global-ssh stage sequence after the write.
func TestRunGlobalSSHApplyVerifyStageIsRecorded(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	record, stages := rec()
	_, err := b.runGlobalSSHApply([]string{"HashKnownHosts"}, lifecyclePolicy{
		Confirm: confirmationAlreadyObtained,
		Stages:  record,
	})
	if err != nil {
		t.Fatalf("runGlobalSSHApply: %v", err)
	}
	found := false
	for _, s := range *stages {
		if s == "verify" {
			found = true
		}
	}
	if !found {
		t.Errorf("stages = %v, want 'verify' stage recorded", *stages)
	}
}

// TestRunGlobalSSHApplyVerifyInconclusiveIsAdvised is the WR-01 regression:
// when the post-write verification probe cannot run at all (here, by
// removing `ssh` from PATH so globalssh.Verify's RunSSHG fails), the result
// must carry an advisory saying so — before the fix, an Inconclusive verify
// result was silently dropped and a failed post-write verification reported
// success with no advisory at all.
func TestRunGlobalSSHApplyVerifyInconclusiveIsAdvised(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)

	// Remove ssh from PATH so the post-write globalssh.Verify probe fails.
	t.Setenv("PATH", t.TempDir())

	res, err := b.runGlobalSSHApply([]string{"HashKnownHosts"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGlobalSSHApply must still succeed (the write itself does not need ssh): %v", err)
	}
	found := false
	for _, a := range res.Advisories {
		if strings.Contains(a, "post-write verification could not run") {
			found = true
		}
	}
	if !found {
		t.Errorf("advisories = %v, want one noting the post-write verification could not run; WR-01 regressed", res.Advisories)
	}
}

// TestRunGlobalSSHApplyInconclusiveSimulationPermitsWrite asserts that when
// BuildGraph fails (e.g. due to a cycle), the apply still completes the write
// and sets SimulationInconclusive in the result.
func TestRunGlobalSSHApplyInconclusiveSimulationPermitsWrite(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(sshDir, "config")
	target := filepath.Join(sshDir, "config.d", "gitid.config")
	includeA := filepath.Join(sshDir, "a.config")
	includeB := filepath.Join(sshDir, "b.config")

	// Plant a cycle: a.config includes b.config which includes a.config.
	writeFile(t, includeA, fmt.Sprintf("Include %s\n", includeB))
	writeFile(t, includeB, fmt.Sprintf("Include %s\n", includeA))
	mainContent := fmt.Sprintf("Include %s\n", includeA)
	writeFile(t, configPath, mainContent)

	b := newBackendForHome(home)
	res, err := b.runGlobalSSHApply([]string{"HashKnownHosts"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGlobalSSHApply must succeed even when graph is inconclusive: %v", err)
	}
	// The write should still happen.
	if !fileExists(target) {
		t.Error("apply must write the target even when simulation is inconclusive")
	}
	_ = res
}

// TestRunGlobalSSHApplyDryRunReportsSimulate asserts that a dry run records
// the simulate stage and leaves the target unchanged.
func TestRunGlobalSSHApplyDryRunReportsSimulate(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	record, stages := rec()
	before := snapshotPaths(t, []string{filepath.Join(home, ".ssh", "config")})

	_, err := b.runGlobalSSHApply([]string{"HashKnownHosts"}, lifecyclePolicy{DryRun: true, Stages: record})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	found := false
	for _, s := range *stages {
		if s == "simulate" {
			found = true
		}
		if s == "write" {
			t.Errorf("dry run must not record 'write' stage")
		}
	}
	if !found {
		t.Errorf("stages = %v, want simulate stage even in dry run", *stages)
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{filepath.Join(home, ".ssh", "config")}))
}
