package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
)

// TestBaselineSetup_DryRun verifies that --dry-run prints the preview header and
// blast-radius warning but writes NO files (SC-1 / SAFE-03).
func TestBaselineSetup_DryRun(t *testing.T) {
	// Set up a temp home directory so no real ~/.gitconfig etc. is touched.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	absGitconfig := filepath.Join(tmpHome, ".gitconfig")
	absBaseline := filepath.Join(tmpHome, ".gitconfig.d", "00-baseline")
	absGitignore := filepath.Join(tmpHome, ".gitignore_global")

	// stdin is empty (no prompts read under --dry-run).
	var in strings.Reader
	var out bytes.Buffer

	err := runBaselineSetup(&in, &out, true /* dryRun */)
	if err != nil {
		t.Fatalf("runBaselineSetup --dry-run returned error: %v", err)
	}

	// Must print the preview header.
	if !strings.Contains(out.String(), "=== Preview: baseline setup ===") {
		t.Errorf("expected preview header in output; got:\n%s", out.String())
	}

	// Must print the blast-radius warning.
	if !strings.Contains(out.String(), "insteadOf rewrites affect ALL HTTPS operations") {
		t.Errorf("expected blast-radius warning in output; got:\n%s", out.String())
	}

	// Must print the dry-run completion line.
	if !strings.Contains(out.String(), "--dry-run: no files were written.") {
		t.Errorf("expected dry-run completion message in output; got:\n%s", out.String())
	}

	// None of the three files should have been created.
	for _, path := range []string{absGitconfig, absBaseline, absGitignore} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("--dry-run should not create %s, but file exists", path)
		}
	}
}

// TestBaselineSetup_DryRun_NoConflictsNote verifies the no-conflicts note is
// printed when the gitconfig file does not exist (first-run case).
func TestBaselineSetup_DryRun_NoConflictsNote(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var in strings.Reader
	var out bytes.Buffer

	err := runBaselineSetup(&in, &out, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "No conflicts found in ~/.gitconfig.") {
		t.Errorf("expected no-conflicts note; got:\n%s", out.String())
	}
}

// TestBaselineShow_NotInstalled verifies the empty-state copy when no baseline
// artifacts exist in the temp home directory.
func TestBaselineShow_NotInstalled(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var out bytes.Buffer
	err := runBaselineShow(strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("runBaselineShow returned error: %v", err)
	}

	want := "no gitid-managed baseline found"
	if !strings.Contains(out.String(), want) {
		t.Errorf("expected empty-state copy %q; got:\n%s", want, out.String())
	}
	if !strings.Contains(out.String(), "Run 'gitid baseline setup' to initialize.") {
		t.Errorf("expected initialize hint; got:\n%s", out.String())
	}
}

// TestBaselineShow_Installed verifies the installed read-back output after a
// confirmed setup. It drives the three writers directly via the internal package
// (hermetic — no real home touched).
func TestBaselineShow_Installed(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	absGitconfig := filepath.Join(tmpHome, ".gitconfig")
	absBaseline := filepath.Join(tmpHome, ".gitconfig.d", "00-baseline")
	absGitignore := filepath.Join(tmpHome, ".gitignore_global")

	// Write the three surfaces directly using the internal writers (avoids
	// needing an interactive stdin for the confirm prompt).
	cfg := gitconfig.DefaultBaselineConfig()
	rewrites := gitconfig.DefaultURLRewrites()
	patterns := gitconfig.DefaultGitignorePatterns()

	if _, err := gitconfig.WriteGlobalGitignore(absGitignore, patterns); err != nil {
		t.Fatalf("WriteGlobalGitignore: %v", err)
	}
	if _, err := gitconfig.WriteBaselineFile(absBaseline, cfg, rewrites); err != nil {
		t.Fatalf("WriteBaselineFile: %v", err)
	}
	if _, err := gitconfig.WriteBaselineInclude(absGitconfig, absBaseline); err != nil {
		t.Fatalf("WriteBaselineInclude: %v", err)
	}

	var out bytes.Buffer
	err := runBaselineShow(strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("runBaselineShow returned error: %v", err)
	}

	got := out.String()

	// Must contain the installed header.
	if !strings.Contains(got, "baseline: installed") {
		t.Errorf("expected 'baseline: installed'; got:\n%s", got)
	}
	// Must list the baseline file path.
	if !strings.Contains(got, "00-baseline") {
		t.Errorf("expected baseline file reference; got:\n%s", got)
	}
	// Must show url rewrites count.
	if !strings.Contains(got, "url rewrites: 3 active") {
		t.Errorf("expected 'url rewrites: 3 active'; got:\n%s", got)
	}
	// Must show gitignore patterns.
	if !strings.Contains(got, "managed patterns: 13") {
		t.Errorf("expected 'managed patterns: 13'; got:\n%s", got)
	}
	// Must not contain === wrapper (UI-SPEC format note).
	if strings.Contains(got, "===") {
		t.Errorf("show output must NOT contain === wrapper; got:\n%s", got)
	}
	// Must contain baseline keys section.
	if !strings.Contains(got, "baseline keys:") {
		t.Errorf("expected 'baseline keys:'; got:\n%s", got)
	}
}

// TestBaselineShow_Incomplete verifies the incomplete state when only the include
// block exists but the baseline file is absent.
func TestBaselineShow_Incomplete(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	absGitconfig := filepath.Join(tmpHome, ".gitconfig")
	absBaseline := filepath.Join(tmpHome, ".gitconfig.d", "00-baseline")

	// Write ONLY the include block (no baseline file) to trigger incomplete state.
	if _, err := gitconfig.WriteBaselineInclude(absGitconfig, absBaseline); err != nil {
		t.Fatalf("WriteBaselineInclude: %v", err)
	}

	var out bytes.Buffer
	err := runBaselineShow(strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("runBaselineShow returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "baseline: incomplete") {
		t.Errorf("expected 'baseline: incomplete'; got:\n%s", got)
	}
	if !strings.Contains(got, "! missing:") {
		t.Errorf("expected missing artifact lines; got:\n%s", got)
	}
	if !strings.Contains(got, "Run 'gitid baseline setup' to repair.") {
		t.Errorf("expected repair hint; got:\n%s", got)
	}
}

// TestBaselineSetup_Idempotency is the end-to-end SC-1/SC-2 idempotency check:
// run setup twice (with "y" confirm each time) and assert the three output files
// are byte-identical after the second run and no second backup was created.
func TestBaselineSetup_Idempotency(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	absGitconfig := filepath.Join(tmpHome, ".gitconfig")
	absBaseline := filepath.Join(tmpHome, ".gitconfig.d", "00-baseline")
	absGitignore := filepath.Join(tmpHome, ".gitignore_global")

	// First run: write the three surfaces via internal writers for determinism
	// (avoids interactive stdin complexity for the confirm gate).
	cfg := gitconfig.DefaultBaselineConfig()
	rewrites := gitconfig.DefaultURLRewrites()
	patterns := gitconfig.DefaultGitignorePatterns()

	if _, err := gitconfig.WriteGlobalGitignore(absGitignore, patterns); err != nil {
		t.Fatalf("first WriteGlobalGitignore: %v", err)
	}
	if _, err := gitconfig.WriteBaselineFile(absBaseline, cfg, rewrites); err != nil {
		t.Fatalf("first WriteBaselineFile: %v", err)
	}
	if _, err := gitconfig.WriteBaselineInclude(absGitconfig, absBaseline); err != nil {
		t.Fatalf("first WriteBaselineInclude: %v", err)
	}

	// Capture the file contents after the first run.
	gitconfigAfterFirst := readFileBytes(t, absGitconfig)
	baselineAfterFirst := readFileBytes(t, absBaseline)
	gitignoreAfterFirst := readFileBytes(t, absGitignore)

	// Collect backup files created by the first run.
	backupsAfterFirst := countBackups(t, tmpHome)

	// Second run: call the same writers again with identical inputs.
	if _, err := gitconfig.WriteGlobalGitignore(absGitignore, patterns); err != nil {
		t.Fatalf("second WriteGlobalGitignore: %v", err)
	}
	if _, err := gitconfig.WriteBaselineFile(absBaseline, cfg, rewrites); err != nil {
		t.Fatalf("second WriteBaselineFile: %v", err)
	}
	if _, err := gitconfig.WriteBaselineInclude(absGitconfig, absBaseline); err != nil {
		t.Fatalf("second WriteBaselineInclude: %v", err)
	}

	// Assert byte-identical files after the second run.
	if !bytes.Equal(gitconfigAfterFirst, readFileBytes(t, absGitconfig)) {
		t.Error("SC-1: ~/.gitconfig differs between first and second run")
	}
	if !bytes.Equal(baselineAfterFirst, readFileBytes(t, absBaseline)) {
		t.Error("SC-1: ~/.gitconfig.d/00-baseline differs between first and second run")
	}
	if !bytes.Equal(gitignoreAfterFirst, readFileBytes(t, absGitignore)) {
		t.Error("SC-2: ~/.gitignore_global differs between first and second run")
	}

	// Assert no NEW backup files were created by the second run (idempotent no-op).
	backupsAfterSecond := countBackups(t, tmpHome)
	if backupsAfterSecond > backupsAfterFirst {
		t.Errorf("SC-1/SC-2: second run created %d new backup(s) — idempotent run should create none",
			backupsAfterSecond-backupsAfterFirst)
	}
}

// readFileBytes is a test helper that reads a file and fails the test on error.
func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test helper; path is a test temp dir path
	if err != nil {
		t.Fatalf("readFileBytes(%s): %v", path, err)
	}
	return data
}

// countBackups counts .bak.* files anywhere under dir (recursive) as a proxy
// for "number of backup files created by filewriter.Write".
func countBackups(t *testing.T, dir string) int {
	t.Helper()
	count := 0
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		if strings.Contains(name, ".bak.") {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("countBackups: %v", err)
	}
	return count
}
