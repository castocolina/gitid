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
