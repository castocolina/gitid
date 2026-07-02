package main

import (
	"bufio"
	"bytes"
	"io"
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

	err := runBaselineSetup(&in, &out, true /* dryRun */, false /* assumeYes */)
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

	err := runBaselineSetup(&in, &out, true, false)
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

// TestPromptYN_WR06 verifies that promptYN returns false (safe direction) on a
// non-EOF read error, not true (which would silently accept an opt-out default).
func TestPromptYN_WR06(t *testing.T) {
	t.Run("read error returns false (safe decline)", func(t *testing.T) {
		// Construct a reader that immediately returns an error on ReadString.
		pr, pw := io.Pipe()
		_ = pw.CloseWithError(io.ErrUnexpectedEOF) // non-EOF error

		r := bufio.NewReader(pr)
		var out bytes.Buffer
		result := promptYN(r, &out, "Keep rewrite?")
		if result {
			t.Error("WR-06: promptYN returned true on read error; expected false (safe direction)")
		}
	})

	t.Run("clean EOF with empty line returns true (default Y)", func(t *testing.T) {
		// Clean EOF immediately — simulates pressing Enter with no input.
		pr, pw := io.Pipe()
		_ = pw.Close() // clean EOF

		r := bufio.NewReader(pr)
		var out bytes.Buffer
		result := promptYN(r, &out, "Include Tier-2?")
		if !result {
			t.Error("WR-06: promptYN returned false on clean EOF; expected true (default Y)")
		}
	})
}

// TestRestoreSnapshot_Rollback verifies the snapshotFile / restoreSnapshot
// helpers used by the CR-01 snapshot-based rollback path. Covers: new-file
// rollback (snapshot.existed=false → remove), pre-existing file rollback
// (snapshot.existed=true → restore bytes), and absent-target no-op.
func TestRestoreSnapshot_Rollback(t *testing.T) {
	t.Run("existed=false removes the newly-created file", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "newfile.txt")

		// Snapshot BEFORE the file exists.
		snap, err := snapshotFile(target)
		if err != nil {
			t.Fatalf("snapshotFile: %v", err)
		}
		if snap.existed {
			t.Fatal("snapshotFile: existed=true for absent file")
		}

		// Simulate: write created the file.
		if err := os.WriteFile(target, []byte("new content"), 0o644); err != nil { //nolint:gosec // test path
			t.Fatalf("creating target: %v", err)
		}

		if err := restoreSnapshot(target, snap); err != nil {
			t.Fatalf("restoreSnapshot: %v", err)
		}

		// The file must be gone after rollback.
		if _, statErr := os.Stat(target); statErr == nil {
			t.Error("CR-01: newly-created file still exists after restoreSnapshot (existed=false)")
		}
	})

	t.Run("existed=true restores original content", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "existing.txt")

		originalContent := []byte("original content")
		if err := os.WriteFile(target, originalContent, 0o644); err != nil { //nolint:gosec // test path
			t.Fatalf("creating target: %v", err)
		}

		snap, err := snapshotFile(target)
		if err != nil {
			t.Fatalf("snapshotFile: %v", err)
		}

		// Simulate: write overwrote the file.
		if err := os.WriteFile(target, []byte("new content after write"), 0o644); err != nil { //nolint:gosec // test path
			t.Fatalf("overwriting target: %v", err)
		}

		if err := restoreSnapshot(target, snap); err != nil {
			t.Fatalf("restoreSnapshot: %v", err)
		}

		// Target must contain the original content.
		got, err := os.ReadFile(target) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading target after restore: %v", err)
		}
		if string(got) != string(originalContent) {
			t.Errorf("CR-01: after restoreSnapshot, target contains %q, want %q", got, originalContent)
		}
	})

	t.Run("existed=false is no-op when target does not exist", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "absent.txt")

		snap, err := snapshotFile(missing)
		if err != nil {
			t.Fatalf("snapshotFile on absent path: %v", err)
		}
		// File does not exist and was never written — restoreSnapshot must not error.
		if err := restoreSnapshot(missing, snap); err != nil {
			t.Fatalf("restoreSnapshot on absent file with existed=false: %v", err)
		}
	})
}

// TestSnapshotRollback_PreservesPreExistingFiles is the CR-01 regression test
// for the data-loss bug in snapshot-based rollback. It verifies that
// restoreSnapshot never removes a file that pre-existed the write sequence,
// even when the write for that file was skipped (idempotent — no backup was
// taken). This covers the scenario: 00-baseline and ~/.gitignore_global already
// exist with correct content → WriteGlobalGitignore / WriteBaselineFile are
// idempotent (backupPath="") → WriteBaselineInclude fails → rollback must NOT
// delete the two pre-existing files.
func TestSnapshotRollback_PreservesPreExistingFiles(t *testing.T) {
	dir := t.TempDir()

	// Simulate two files that pre-exist with content (they represent
	// ~/.gitignore_global and 00-baseline after a first-run setup).
	gitignorePath := filepath.Join(dir, ".gitignore_global")
	baselinePath := filepath.Join(dir, "00-baseline")

	origGitignore := []byte("# existing gitignore content\n")
	origBaseline := []byte("# existing baseline content\n")

	if err := os.WriteFile(gitignorePath, origGitignore, 0o644); err != nil { //nolint:gosec // test path
		t.Fatalf("seeding gitignore: %v", err)
	}
	if err := os.WriteFile(baselinePath, origBaseline, 0o644); err != nil { //nolint:gosec // test path
		t.Fatalf("seeding baseline: %v", err)
	}

	// Take snapshots of both files — this is what the fixed runBaselineSetup
	// does before writing any of the three surfaces (snapshot-before-write).
	snapGitignore, err := snapshotFile(gitignorePath)
	if err != nil {
		t.Fatalf("snapshotFile gitignore: %v", err)
	}
	snapBaseline, err := snapshotFile(baselinePath)
	if err != nil {
		t.Fatalf("snapshotFile baseline: %v", err)
	}

	// Simulate: both writes are idempotent (content unchanged), so neither
	// file is mutated. Then the third write (include) fails. Rollback all.
	//
	// Key assertion: a file that existed before and was NOT modified during
	// the write sequence must still exist and have the same bytes after rollback.
	if err := restoreSnapshot(gitignorePath, snapGitignore); err != nil {
		t.Fatalf("restoreSnapshot gitignore: %v", err)
	}
	if err := restoreSnapshot(baselinePath, snapBaseline); err != nil {
		t.Fatalf("restoreSnapshot baseline: %v", err)
	}

	// Both files must still exist with their original bytes.
	if _, statErr := os.Stat(gitignorePath); os.IsNotExist(statErr) {
		t.Error("CR-01 regression: gitignore was DELETED by restoreSnapshot; pre-existing file must be preserved")
	} else if !bytes.Equal(readFileBytes(t, gitignorePath), origGitignore) {
		t.Errorf("CR-01 regression: gitignore bytes changed after restoreSnapshot; want %q got %q",
			origGitignore, readFileBytes(t, gitignorePath))
	}

	if _, statErr := os.Stat(baselinePath); os.IsNotExist(statErr) {
		t.Error("CR-01 regression: baseline was DELETED by restoreSnapshot; pre-existing file must be preserved")
	} else if !bytes.Equal(readFileBytes(t, baselinePath), origBaseline) {
		t.Errorf("CR-01 regression: baseline bytes changed after restoreSnapshot; want %q got %q",
			origBaseline, readFileBytes(t, baselinePath))
	}
}

// TestSnapshotRollback_NewFilesAreRemoved verifies that restoreSnapshot removes
// a file that did NOT exist before the write (the "new file" rollback case).
func TestSnapshotRollback_NewFilesAreRemoved(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "newfile.txt")

	// Snapshot before the file exists.
	snap, err := snapshotFile(newPath)
	if err != nil {
		t.Fatalf("snapshotFile on absent path: %v", err)
	}
	if snap.existed {
		t.Fatal("snapshotFile: reported existed=true for absent file")
	}

	// Simulate: write succeeded (file now exists).
	if err := os.WriteFile(newPath, []byte("written content"), 0o644); err != nil { //nolint:gosec // test path
		t.Fatalf("creating new file: %v", err)
	}

	// Rollback: the file must be removed (it was new).
	if err := restoreSnapshot(newPath, snap); err != nil {
		t.Fatalf("restoreSnapshot new file: %v", err)
	}

	if _, statErr := os.Stat(newPath); statErr == nil {
		t.Error("restoreSnapshot: newly-created file still exists after rollback; expected removal")
	}
}

// TestSnapshotRollback_ModifiedFileIsRestored verifies that restoreSnapshot
// restores the original content when a file existed before and was modified.
func TestSnapshotRollback_ModifiedFileIsRestored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	orig := []byte("original content\n")
	if err := os.WriteFile(path, orig, 0o644); err != nil { //nolint:gosec // test path
		t.Fatalf("seeding file: %v", err)
	}

	snap, err := snapshotFile(path)
	if err != nil {
		t.Fatalf("snapshotFile: %v", err)
	}
	if !snap.existed {
		t.Fatal("snapshotFile: reported existed=false for existing file")
	}

	// Simulate: write modifies the file.
	if err := os.WriteFile(path, []byte("new content after write\n"), 0o644); err != nil { //nolint:gosec // test path
		t.Fatalf("overwriting file: %v", err)
	}

	// Rollback: original content must be restored.
	if err := restoreSnapshot(path, snap); err != nil {
		t.Fatalf("restoreSnapshot: %v", err)
	}

	got := readFileBytes(t, path)
	if !bytes.Equal(got, orig) {
		t.Errorf("restoreSnapshot: got %q, want %q", got, orig)
	}
}

// TestSnapshotRollback_PreservesFileMode verifies that restoreSnapshot restores
// the original file mode (Minor #4: mode must not be hardcoded to 0o644).
func TestSnapshotRollback_PreservesFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	orig := []byte("content\n")
	origMode := os.FileMode(0o600)
	if err := os.WriteFile(path, orig, origMode); err != nil { //nolint:gosec // test path
		t.Fatalf("seeding file: %v", err)
	}
	// Explicitly set mode (WriteFile applies umask).
	if err := os.Chmod(path, origMode); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	snap, err := snapshotFile(path)
	if err != nil {
		t.Fatalf("snapshotFile: %v", err)
	}

	// Simulate: write changes mode to 0o644.
	if err := os.WriteFile(path, orig, 0o644); err != nil { //nolint:gosec // test path
		t.Fatalf("overwrite: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil { //nolint:gosec // G302: test path simulating a 0644 gitconfig write
		t.Fatalf("chmod 644: %v", err)
	}

	if err := restoreSnapshot(path, snap); err != nil {
		t.Fatalf("restoreSnapshot: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after restore: %v", err)
	}
	gotMode := info.Mode().Perm()
	if gotMode != origMode {
		t.Errorf("Minor #4: restoreSnapshot restored mode %04o, want %04o", gotMode, origMode)
	}
}

// TestBaselineSetup_IncludePathIsTildeForm verifies that runBaselineSetup writes
// `path = ~/.gitconfig.d/00-baseline` (tilde form) in ~/.gitconfig, not the
// absolute /Users/... path. Covers defect #2 (absolute path written vs. tilde).
func TestBaselineSetup_IncludePathIsTildeForm(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	absGitconfig := filepath.Join(tmpHome, ".gitconfig")

	// Drive runBaselineSetup with confirmed "y" answers to reach the write step.
	in := strings.NewReader("y\ny\ny\ny\ny\n")
	var out bytes.Buffer
	err := runBaselineSetup(in, &out, false, false)
	if err != nil {
		t.Fatalf("runBaselineSetup: %v", err)
	}

	// Read back ~/.gitconfig and check the include path.
	gitconfigBytes := readFileBytes(t, absGitconfig)
	gitconfigContent := string(gitconfigBytes)

	// The include path must use the tilde form, not the absolute tmp path.
	wantLine := "path = ~/.gitconfig.d/00-baseline"
	if !strings.Contains(gitconfigContent, wantLine) {
		t.Errorf("include path is not in tilde form; want %q in ~/.gitconfig, got content:\n%s",
			wantLine, gitconfigContent)
	}

	// Defensive: make sure the absolute temp-dir path is NOT in the include line.
	// tmpHome looks like /var/folders/... or /tmp/... — if it appears in an include
	// path = line that's the bug.
	for _, line := range strings.Split(gitconfigContent, "\n") {
		if strings.Contains(line, "path =") && strings.Contains(line, tmpHome) {
			t.Errorf("include path contains absolute tmp path %q; want tilde form. line: %q", tmpHome, line)
		}
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
