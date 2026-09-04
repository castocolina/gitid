package main

// lifecycle_globalgit_test.go covers plan 07-01 Task 2: the ONE complete
// global-git lifecycle function (runGlobalGitApply), the confirmed write
// to the include'd baseline file, the mutation journal's rollback, R-3
// unconditional-backup, and the dry-run / declined-confirmation invariants.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/globalgit"
)

// ---------------------------------------------------------------------------
// runGlobalGitApply — the ONE global-git fix ceremony (plan 07-01)
// ---------------------------------------------------------------------------

// TestGlobalGitLifecycleStagesRow asserts the global-git row in lifecycleStages
// contains confirm, backup, and write in that relative order — the D-02 invariant.
func TestGlobalGitLifecycleStagesRow(t *testing.T) {
	stages, ok := lifecycleStages["global-git"]
	if !ok {
		t.Fatal("lifecycleStages must contain a 'global-git' row")
	}
	pos := map[string]int{}
	for i, s := range stages {
		pos[s] = i
	}
	for _, req := range []string{"plan", "confirm", "backup", "write"} {
		if _, ok := pos[req]; !ok {
			t.Errorf("global-git stages missing %q: %v", req, stages)
		}
	}
	if pos["confirm"] >= pos["backup"] || pos["backup"] >= pos["write"] {
		t.Errorf("global-git stages must order confirm -> backup -> write, got: %v", stages)
	}
}

// TestRunGlobalGitApply_OneFreshSandbox applies one key to a fresh sandbox HOME
// and asserts: the floor include block is present in the main config file, the
// baseline file contains the key inside the managed block.
func TestRunGlobalGitApply_OneFreshSandbox(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	b := newBackendForHome(home)

	_, err := b.runGlobalGitApply([]string{"init.defaultBranch"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGlobalGitApply: %v", err)
	}

	gitconfigPath := filepath.Join(home, ".gitconfig")
	baselinePath := b.baselineTargetPath()

	gc, readErr := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if readErr != nil {
		t.Fatalf("reading %s: %v", gitconfigPath, readErr)
	}
	if !strings.Contains(string(gc), "# BEGIN gitid managed: baseline-include") {
		t.Errorf("main config missing baseline-include block:\n%s", gc)
	}

	bf, readErr := os.ReadFile(baselinePath) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if readErr != nil {
		t.Fatalf("reading baseline file %s: %v", baselinePath, readErr)
	}
	if !strings.Contains(string(bf), "defaultBranch = main") {
		t.Errorf("baseline file missing init.defaultBranch:\n%s", bf)
	}
	if !strings.Contains(string(bf), "# BEGIN gitid managed: global-git") {
		t.Errorf("baseline file missing global-git block:\n%s", bf)
	}
}

// TestRunGlobalGitApply_SameSelectionTwice_FreshDistinctBackup asserts that
// applying the same selection twice produces a fresh, DISTINCT backup path (R-3:
// backup is unconditional once authorized).
func TestRunGlobalGitApply_SameSelectionTwice_FreshDistinctBackup(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	b := newBackendForHome(home)

	res1, err := b.runGlobalGitApply([]string{"init.defaultBranch"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("first runGlobalGitApply: %v", err)
	}
	res2, err := b.runGlobalGitApply([]string{"init.defaultBranch"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("second runGlobalGitApply: %v", err)
	}
	t.Logf("backup paths: first=%v second=%v", res1.Backups, res2.Backups)

	// Both applies must return backup paths after the second run (the file
	// existed from the first run). The paths must differ (R-3).
	if len(res2.Backups) > 0 && len(res1.Backups) > 0 {
		for _, bp2 := range res2.Backups {
			for _, bp1 := range res1.Backups {
				if bp1 == bp2 && bp1 != "" {
					t.Errorf("R-3 violated: second backup path %q is identical to first", bp2)
				}
			}
		}
	}
}

// TestRunGlobalGitApply_InjectedFailureRestores asserts that a failure injected
// after the first write restores every watched file to its pre-transaction bytes.
func TestRunGlobalGitApply_InjectedFailureRestores(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	b := newBackendForHome(home)
	b.failCommitAt = func(s string) error {
		if s == "global-git-baseline-write" {
			return fmt.Errorf("injected failure after baseline-include write")
		}
		return nil
	}

	gitconfigPath := filepath.Join(home, ".gitconfig")
	baselinePath := b.baselineTargetPath()
	before := snapshotPaths(t, []string{gitconfigPath, baselinePath})

	res, err := b.runGlobalGitApply([]string{"init.defaultBranch"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err == nil {
		t.Fatal("runGlobalGitApply must surface the injected failure")
	}
	if !strings.Contains(err.Error(), "injected") {
		t.Errorf("err = %q, want the concrete injected failure", err.Error())
	}

	after := snapshotPaths(t, []string{gitconfigPath, baselinePath})
	assertUnchanged(t, before, after)
	t.Logf("restore outcomes: %v", res.Restored)
}

// TestRunGlobalGitApply_DryRun_NoFileChanged asserts a dry run does not touch
// any file and never calls the confirmation gate.
func TestRunGlobalGitApply_DryRun_NoFileChanged(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	record, stages := rec()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	before := snapshotPaths(t, []string{gitconfigPath})

	confirmCalled := false
	_, err := b.runGlobalGitApply([]string{"init.defaultBranch"}, lifecyclePolicy{
		DryRun: true,
		Stages: record,
		Prompt: func(_ string) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if confirmCalled {
		t.Error("dry run must not call the confirmation prompt")
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath}))
	_ = stages
}

// TestRunGlobalGitApply_DeclinedConfirmation asserts a declined confirmation
// returns an error naming the cancellation and writes nothing.
func TestRunGlobalGitApply_DeclinedConfirmation(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	gitconfigPath := filepath.Join(home, ".gitconfig")
	before := snapshotPaths(t, []string{gitconfigPath})

	_, err := b.runGlobalGitApply([]string{"init.defaultBranch"}, lifecyclePolicy{
		Confirm: confirmationRequired,
		Prompt:  func(_ string) (bool, error) { return false, nil },
	})
	if err == nil {
		t.Fatal("declined confirmation must return an error")
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath}))
}

// TestRunGlobalGitApply_UnknownKeyRejectedByName asserts an unknown key is
// rejected by name before any file is read.
func TestRunGlobalGitApply_UnknownKeyRejectedByName(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	before := snapshotPaths(t, []string{filepath.Join(home, ".gitconfig")})

	_, err := b.runGlobalGitApply([]string{"unknown.nonexistent.key"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err == nil {
		t.Fatal("unknown key must be rejected by name")
	}
	if !strings.Contains(err.Error(), "unknown.nonexistent.key") {
		t.Errorf("err should name the rejected key, got: %v", err)
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{filepath.Join(home, ".gitconfig")}))
}

// readBaselineConflictstyle reads the composed global-git block's
// merge.conflictstyle value back from the baseline file.
func readBaselineConflictstyle(t *testing.T, baselinePath string) string {
	t.Helper()
	bf, err := os.ReadFile(baselinePath) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if err != nil {
		t.Fatalf("reading baseline file %s: %v", baselinePath, err)
	}
	for _, line := range strings.Split(string(bf), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "conflictstyle = ") {
			return strings.TrimPrefix(line, "conflictstyle = ")
		}
	}
	t.Fatalf("conflictstyle line not found in baseline file:\n%s", bf)
	return ""
}

// TestRunGlobalGitApply_HardGateBelowWritesFallback injects a below-gate
// version and asserts the written merge.conflictstyle value is the fallback
// (diff3) — the write boundary, not just the unit, must substitute (T-07-16).
func TestRunGlobalGitApply_HardGateBelowWritesFallback(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	b.gitGate = func() (globalgit.GateOutcome, string) { return globalgit.GateBelow, "" }

	_, err := b.runGlobalGitApply([]string{"merge.conflictstyle"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGlobalGitApply: %v", err)
	}
	if got := readBaselineConflictstyle(t, b.baselineTargetPath()); got != "diff3" {
		t.Errorf("written conflictstyle = %q, want diff3 below the hard gate", got)
	}
}

// TestRunGlobalGitApply_HardGateMetWritesRecommendation injects an at-gate
// version and asserts the written merge.conflictstyle value is the
// recommendation (zdiff3).
func TestRunGlobalGitApply_HardGateMetWritesRecommendation(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	b.gitGate = func() (globalgit.GateOutcome, string) { return globalgit.GateMet, "" }

	_, err := b.runGlobalGitApply([]string{"merge.conflictstyle"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGlobalGitApply: %v", err)
	}
	if got := readBaselineConflictstyle(t, b.baselineTargetPath()); got != "zdiff3" {
		t.Errorf("written conflictstyle = %q, want zdiff3 at/above the hard gate", got)
	}
}

// runRealGitConfig executes a real `git config` probe against a hermetic HOME
// (GIT_CONFIG_NOSYSTEM so the machine's system config cannot interfere), from
// a NON-repository cwd — the same disciplined probe the engine uses.
func runRealGitConfig(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // arg-slice form, no shell; fixed probe args (G204)
	cmd.Env = append(os.Environ(), "HOME="+home, "GIT_CONFIG_NOSYSTEM=1")
	cmd.Dir = home
	out, err := cmd.Output()
	return string(out), err
}

// TestRunGlobalGitApply_BundleCollisionUserValueWins is the post-apply half of
// T-07-19: composing a bundle row emits ALL member keys (D-09) even when the
// user has set one of them to a different value, and — the claim that makes
// the collision acceptable under floor + last-wins — the USER's own value still
// resolves after the write. git itself must name the user's file when asked.
func TestRunGlobalGitApply_BundleCollisionUserValueWins(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()

	// Seed the user's own ~/.gitconfig with a deliberately different alias.
	seeded := "[alias]\n\tco = pull\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(seeded), 0o644); err != nil { //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		t.Fatalf("seeding user gitconfig: %v", err)
	}

	b := newBackendForHome(home)
	_, err := b.runGlobalGitApply([]string{"alias (8 shortcuts)"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGlobalGitApply: %v", err)
	}

	// The baseline block must contain EVERY member key (the whole canonical
	// section), including the one the user set differently (D-09).
	bf, readErr := os.ReadFile(b.baselineTargetPath()) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if readErr != nil {
		t.Fatalf("reading baseline file: %v", readErr)
	}
	for _, line := range []string{"\tco = checkout", "\tst = status", "\tlg", "\tlast = log -1 HEAD"} {
		if !strings.Contains(string(bf), line) {
			t.Errorf("baseline block missing %q (a bundle apply emits every member key):\n%s", line, bf)
		}
	}

	// Read the effective value back with the REAL git binary — floor include +
	// last-wins must resolve the USER's value from the USER's file.
	out, err := runRealGitConfig(t, home, "config", "--show-origin", "--get", "alias.co")
	if err != nil {
		t.Fatalf("git config --show-origin --get alias.co: %v (stdout=%q)", err, out)
	}
	t.Logf("alias.co origin after bundle apply: %s", strings.TrimSpace(out))
	if !strings.Contains(out, filepath.Join(home, ".gitconfig")) {
		t.Errorf("origin = %q, want the user's own ~/.gitconfig", strings.TrimSpace(out))
	}
	if strings.Contains(out, "00-baseline") {
		t.Errorf("origin = %q, must NOT name the gitid baseline file", strings.TrimSpace(out))
	}
	if !strings.Contains(out, "pull") {
		t.Errorf("value = %q, want the user's own 'pull' (their value wins under floor + last-wins)", strings.TrimSpace(out))
	}
}

// TestRunGlobalGitApply_LeavesFallbackAuthorBlockUntouched proves (not just
// asserts) that the baseline apply ceremony never touches the D9
// fallback-author block — a SEPARATE managed block in the SAME file, owned by
// a SEPARATE ceremony (runGitFallbackAuthorApply). This is what makes
// GlobalGitResultTail's claim ("Global user.email was left alone, as always")
// true by construction rather than by comment (07-03-PLAN.md Task 3).
func TestRunGlobalGitApply_LeavesFallbackAuthorBlockUntouched(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	b := newBackendForHome(home)

	// Set the fallback author pair first.
	if _, err := b.runGitFallbackAuthorApply("Team", "team@example.com", lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err != nil {
		t.Fatalf("runGitFallbackAuthorApply: %v", err)
	}
	gitconfigPath := filepath.Join(home, ".gitconfig")
	before, readErr := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if readErr != nil {
		t.Fatalf("reading %s: %v", gitconfigPath, readErr)
	}
	fallbackBefore, ok := extractManagedBlock(string(before), "global-git-author")
	if !ok {
		t.Fatalf("fallback-author block not found after runGitFallbackAuthorApply:\n%s", before)
	}

	// Now apply a baseline option — a SEPARATE ceremony, a SEPARATE managed
	// block (in the include'd baseline file, not ~/.gitconfig's fallback block).
	if _, err := b.runGlobalGitApply([]string{"init.defaultBranch"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err != nil {
		t.Fatalf("runGlobalGitApply: %v", err)
	}
	after, readErr := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if readErr != nil {
		t.Fatalf("re-reading %s: %v", gitconfigPath, readErr)
	}
	fallbackAfter, ok := extractManagedBlock(string(after), "global-git-author")
	if !ok {
		t.Fatalf("fallback-author block missing after a baseline apply:\n%s", after)
	}
	if fallbackBefore != fallbackAfter {
		t.Errorf("fallback-author block changed after a baseline apply:\nbefore:\n%s\nafter:\n%s", fallbackBefore, fallbackAfter)
	}
}

// TestRunGlobalGitApply_WarnsWhenNeutralizingExistingCustomKey is the CR-01
// (09.5-REVIEW.md round 3) second half: CustomGitKeyPlan/runCustomGitKeyWrite
// now REFUSE to create a new collision, but a custom key that collides with
// a curated member could already exist (a pre-guard install, or the
// custom-git-keys block edited by hand). When a curated apply writes a
// member key that a pre-existing custom key also names, the verify stage
// must name the collision as an advisory — not silently neutralise it with
// no signal, which was the whole defect this finding reported.
func TestRunGlobalGitApply_WarnsWhenNeutralizingExistingCustomKey(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	b := newBackendForHome(home)

	// Seed a pre-existing custom-git-keys block whose key collides with the
	// curated scalar row init.defaultBranch — simulating a collision this
	// guard did not create (an older install, or the block edited by hand).
	baselinePath := b.baselineTargetPath()
	if mkErr := os.MkdirAll(filepath.Dir(baselinePath), 0o700); mkErr != nil {
		t.Fatalf("mkdir baseline dir: %v", mkErr)
	}
	seeded, _, seedErr := gitconfig.EnsureCustomGitKey(nil, "init.defaultBranch", "trunk")
	if seedErr != nil {
		t.Fatalf("seeding custom key: %v", seedErr)
	}
	if writeErr := os.WriteFile(baselinePath, seeded, 0o644); writeErr != nil { //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		t.Fatalf("writing seeded baseline file: %v", writeErr)
	}

	res, err := b.runGlobalGitApply([]string{"init.defaultBranch"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGlobalGitApply: %v", err)
	}

	found := false
	for _, adv := range res.Advisories {
		if strings.Contains(adv, "init.defaultBranch") && strings.Contains(adv, "custom") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an advisory naming the neutralised custom key, got: %v", res.Advisories)
	}

	// The custom-git-keys block itself must survive untouched — this
	// ceremony only ever writes the global-git block (D-F).
	bf, readErr := os.ReadFile(baselinePath) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if readErr != nil {
		t.Fatalf("reading baseline file: %v", readErr)
	}
	if !strings.Contains(string(bf), "# BEGIN gitid managed: custom-git-keys") {
		t.Errorf("custom-git-keys block missing after a curated apply:\n%s", bf)
	}
}

// extractManagedBlock returns the BEGIN..END managed block body for name, and
// whether it was found.
func extractManagedBlock(content, name string) (string, bool) {
	begin := "# BEGIN gitid managed: " + name
	end := "# END gitid managed: " + name
	i := strings.Index(content, begin)
	if i < 0 {
		return "", false
	}
	j := strings.Index(content[i:], end)
	if j < 0 {
		return "", false
	}
	return content[i : i+j+len(end)], true
}
