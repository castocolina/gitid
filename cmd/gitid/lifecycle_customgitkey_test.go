package main

// lifecycle_customgitkey_test.go covers plan 09.5-03 Task 2: the seam
// (CustomGitKeyPlan/CommitCustomGitKey in wiring.go) and the ONE production
// writer (runCustomGitKeyWrite in lifecycle.go) — plan preview, backed-up
// transaction, rollback, and SC-1 idempotency.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCustomGitKeyLifecycleStagesRow asserts the custom-git-key row in
// lifecycleStages contains confirm, backup, and write in that relative order
// — the D-02 invariant every verb's row must satisfy.
func TestCustomGitKeyLifecycleStagesRow(t *testing.T) {
	stages, ok := lifecycleStages["custom-git-key"]
	if !ok {
		t.Fatal("lifecycleStages must contain a 'custom-git-key' row")
	}
	pos := map[string]int{}
	for i, s := range stages {
		pos[s] = i
	}
	for _, req := range []string{"plan", "confirm", "backup", "write"} {
		if _, ok := pos[req]; !ok {
			t.Errorf("custom-git-key stages missing %q: %v", req, stages)
		}
	}
	if pos["confirm"] >= pos["backup"] || pos["backup"] >= pos["write"] {
		t.Errorf("custom-git-key stages must order confirm -> backup -> write, got: %v", stages)
	}
}

// TestCustomGitKeyPlanShowsRealDiff asserts CustomGitKeyPlan returns targets
// naming BOTH the main config and the baseline file, backups only for files
// that already exist, and a diff whose added lines contain the new section
// header and the key = value line. A plan for a key/value already present
// returns an EMPTY diff.
func TestCustomGitKeyPlanShowsRealDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	b := newBackendForHome(home)

	view, err := b.CustomGitKeyPlan("core.pager", "less -FRX")
	if err != nil {
		t.Fatalf("CustomGitKeyPlan: %v", err)
	}
	if len(view.Targets) != 2 {
		t.Fatalf("Targets: got %d want 2 (gitconfig + baseline), got: %v", len(view.Targets), view.Targets)
	}
	if len(view.Backups) != 0 {
		t.Errorf("Backups: fresh sandbox has no pre-existing files, want 0, got: %v", view.Backups)
	}
	if !strings.Contains(view.Diff, "[core]") {
		t.Errorf("Diff missing new section header, got:\n%s", view.Diff)
	}
	if !strings.Contains(view.Diff, "pager = less -FRX") {
		t.Errorf("Diff missing new key = value line, got:\n%s", view.Diff)
	}

	// Write the key for real, then re-plan the SAME key/value: the diff must
	// now be empty (nothing left to add).
	if _, err := b.runCustomGitKeyWrite("core.pager", "less -FRX", lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err != nil {
		t.Fatalf("runCustomGitKeyWrite: %v", err)
	}
	secondView, err := b.CustomGitKeyPlan("core.pager", "less -FRX")
	if err != nil {
		t.Fatalf("second CustomGitKeyPlan: %v", err)
	}
	if secondView.Diff != "" {
		t.Errorf("Diff for an already-present key/value must be empty, got:\n%s", secondView.Diff)
	}
}

// TestCustomGitKeyPlanRejectsMalformedKeyBeforeAnyWrite asserts a malformed
// key or an injection-bearing value returns an error from the PLAN stage;
// nothing is written and no backup is taken.
func TestCustomGitKeyPlanRejectsMalformedKeyBeforeAnyWrite(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	gitconfigPath := filepath.Join(home, ".gitconfig")
	baselinePath := b.baselineTargetPath()
	before := snapshotPaths(t, []string{gitconfigPath, baselinePath})

	if _, err := b.CustomGitKeyPlan("nodothere", "value"); err == nil {
		t.Error("CustomGitKeyPlan with a malformed key (no dot) must return an error")
	}
	if _, err := b.CustomGitKeyPlan("core.pager", "bad\nvalue"); err == nil {
		t.Error("CustomGitKeyPlan with an injection-bearing value (newline) must return an error")
	}

	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath, baselinePath}))
}

// TestRunCustomGitKeyWriteLandsBothWrites asserts a confirmed run writes the
// [include] floor into ~/.gitconfig and the custom-git-keys block into the
// baseline file, and returns one backup path per file that pre-existed.
func TestRunCustomGitKeyWriteLandsBothWrites(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	b := newBackendForHome(home)
	gitconfigPath := filepath.Join(home, ".gitconfig")
	baselinePath := b.baselineTargetPath()

	// Pre-seed BOTH files so this run's writes have something to back up.
	if err := os.WriteFile(gitconfigPath, []byte("[user]\n\tname = Pre Existing\n"), 0o600); err != nil {
		t.Fatalf("seeding gitconfig: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(baselinePath), 0o700); err != nil {
		t.Fatalf("seeding baseline dir: %v", err)
	}
	if err := os.WriteFile(baselinePath, []byte("# BEGIN gitid managed: global-git\n[core]\n\tignorecase = false\n# END gitid managed: global-git\n"), 0o600); err != nil {
		t.Fatalf("seeding baseline file: %v", err)
	}

	res, err := b.runCustomGitKeyWrite("mytool.sub.key", "value1", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runCustomGitKeyWrite: %v", err)
	}
	if len(res.Backups) != 2 {
		t.Errorf("Backups: both files pre-existed, want 2, got: %v", res.Backups)
	}

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
	if !strings.Contains(string(bf), `[mytool "sub"]`) {
		t.Errorf("baseline file missing custom key section header:\n%s", bf)
	}
	if !strings.Contains(string(bf), "key = value1") {
		t.Errorf("baseline file missing custom key value:\n%s", bf)
	}
	if !strings.Contains(string(bf), "# BEGIN gitid managed: global-git") {
		t.Errorf("baseline file must still contain the curated global-git block:\n%s", bf)
	}
}

// TestRunCustomGitKeyWriteRollsBackOnFailure asserts that with failCommitAt
// injecting a failure at the baseline write, BOTH files are restored to
// their pre-run bytes, the returned error names the cause, and Restored
// lists the outcomes.
func TestRunCustomGitKeyWriteRollsBackOnFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	b := newBackendForHome(home)
	b.failCommitAt = func(s string) error {
		if s == "custom-git-key-baseline-write" {
			return fmt.Errorf("injected failure before the baseline write")
		}
		return nil
	}

	gitconfigPath := filepath.Join(home, ".gitconfig")
	baselinePath := b.baselineTargetPath()
	before := snapshotPaths(t, []string{gitconfigPath, baselinePath})

	res, err := b.runCustomGitKeyWrite("core.pager", "less -FRX", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err == nil {
		t.Fatal("runCustomGitKeyWrite must surface the injected failure")
	}
	if !strings.Contains(err.Error(), "injected") {
		t.Errorf("err = %q, want the concrete injected failure", err.Error())
	}
	if len(res.Restored) == 0 {
		t.Error("Restored must list the rollback outcomes")
	}

	after := snapshotPaths(t, []string{gitconfigPath, baselinePath})
	assertUnchanged(t, before, after)
	t.Logf("restore outcomes: %v", res.Restored)
}

// TestRunCustomGitKeyWriteRefusesWithoutAuthorization asserts an unauthorized
// policy returns a cancellation error and writes nothing.
func TestRunCustomGitKeyWriteRefusesWithoutAuthorization(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	gitconfigPath := filepath.Join(home, ".gitconfig")
	baselinePath := b.baselineTargetPath()
	before := snapshotPaths(t, []string{gitconfigPath, baselinePath})

	_, err := b.runCustomGitKeyWrite("core.pager", "less -FRX", lifecyclePolicy{
		Confirm: confirmationRequired,
		Prompt:  func(_ string) (bool, error) { return false, nil },
	})
	if err == nil {
		t.Fatal("declined confirmation must return an error")
	}

	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath, baselinePath}))
}

// TestRunCustomGitKeyWriteDryRunNeverConfirms asserts a dry-run policy stops
// after the plan stage, never calls the confirmation prompt, and writes
// nothing — the same dry-run-before-confirm ordering runGlobalGitApply's own
// dry-run test pins.
func TestRunCustomGitKeyWriteDryRunNeverConfirms(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	gitconfigPath := filepath.Join(home, ".gitconfig")
	baselinePath := b.baselineTargetPath()
	before := snapshotPaths(t, []string{gitconfigPath, baselinePath})

	confirmCalled := false
	_, err := b.runCustomGitKeyWrite("core.pager", "less -FRX", lifecyclePolicy{
		DryRun: true,
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
	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath, baselinePath}))
}

// TestRunCustomGitKeyWriteIsIdempotent asserts a second run with the same key
// and value writes nothing and returns no backup path (SC-1).
func TestRunCustomGitKeyWriteIsIdempotent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	b := newBackendForHome(home)

	if _, err := b.runCustomGitKeyWrite("core.pager", "less -FRX", lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err != nil {
		t.Fatalf("first runCustomGitKeyWrite: %v", err)
	}

	res2, err := b.runCustomGitKeyWrite("core.pager", "less -FRX", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("second runCustomGitKeyWrite: %v", err)
	}
	if len(res2.Backups) != 0 {
		t.Errorf("SC-1 idempotency: re-writing the same key/value must take no backup, got: %v", res2.Backups)
	}
}
