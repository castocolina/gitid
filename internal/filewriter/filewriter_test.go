package filewriter

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestWriteCreatesNewTargetWithExactMode verifies that writing to a
// non-existent target creates it with the exact requested mode and that no
// backup file is produced because there was nothing to back up.
func TestWriteCreatesNewTargetWithExactMode(t *testing.T) {
	cases := []struct {
		name string
		mode os.FileMode
	}{
		{name: "config 0600", mode: 0o600},
		{name: "pub 0644", mode: 0o644},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			content := []byte("hello\n")

			backup, err := Write(target, content, tc.mode)
			if err != nil {
				t.Fatalf("Write returned error: %v", err)
			}
			if backup != "" {
				t.Fatalf("expected empty backup path for new target, got %q", backup)
			}

			got, err := os.ReadFile(target) //nolint:gosec // test reads back the file it just wrote
			if err != nil {
				t.Fatalf("reading target: %v", err)
			}
			if string(got) != string(content) {
				t.Fatalf("content mismatch: got %q want %q", got, content)
			}

			info, err := os.Stat(target)
			if err != nil {
				t.Fatalf("stat target: %v", err)
			}
			if info.Mode().Perm() != tc.mode {
				t.Fatalf("mode mismatch: got %o want %o", info.Mode().Perm(), tc.mode)
			}

			// No stray backup or temp files should be present in the dir.
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read dir: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected exactly 1 file in dir, got %d: %v", len(entries), entries)
			}
		})
	}
}

// TestWriteBacksUpExistingTarget verifies that writing over an existing
// target first copies the original to <target>.bak.<timestamp> at mode 0600,
// then atomically replaces the target with the new content at the requested
// mode. The returned backup path must be non-empty.
func TestWriteBacksUpExistingTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config")
	original := []byte("original-content\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatalf("seeding target: %v", err)
	}

	updated := []byte("updated-content\n")
	backup, err := Write(target, updated, 0o600)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if backup == "" {
		t.Fatal("expected non-empty backup path for pre-existing target")
	}

	// Backup must contain the original content at mode 0600.
	backupBytes, err := os.ReadFile(backup) //nolint:gosec // backup path returned by Write under test
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(backupBytes) != string(original) {
		t.Fatalf("backup content mismatch: got %q want %q", backupBytes, original)
	}
	backupInfo, err := os.Stat(backup)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if backupInfo.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode mismatch: got %o want 0600", backupInfo.Mode().Perm())
	}

	// Backup filename must follow the <target>.bak.<unix-nanoseconds> format
	// (Codex HIGH #1: nanosecond resolution replaces the collision-prone
	// second-resolution timestamp).
	prefix := target + ".bak."
	if !strings.HasPrefix(backup, prefix) {
		t.Fatalf("backup name %q does not have prefix %q", backup, prefix)
	}
	stamp := strings.TrimPrefix(backup, prefix)
	if _, convErr := strconv.ParseInt(stamp, 10, 64); convErr != nil {
		t.Fatalf("backup timestamp %q is not a decimal unix-nanoseconds value: %v", stamp, convErr)
	}

	// Target must now hold the updated content.
	got, err := os.ReadFile(target) //nolint:gosec // test reads back the file it just wrote
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}
	if string(got) != string(updated) {
		t.Fatalf("target content mismatch: got %q want %q", got, updated)
	}
}

// TestWriteRestoreOnError verifies that when the atomic write fails the
// original target is left intact and no partial temp file is leaked into the
// target directory. Failure is forced by making the target's parent directory
// read-only so os.CreateTemp cannot create the temp file.
func TestWriteRestoreOnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission enforcement")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "config")
	original := []byte("keep-me\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatalf("seeding target: %v", err)
	}

	// Make the directory read-only so temp creation inside it fails.
	if err := os.Chmod(dir, 0o500); err != nil { //nolint:gosec // intentionally restrictive read-only dir to force a write failure
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // restoring the 0700 dir contract for cleanup

	_, err := Write(target, []byte("new\n"), 0o600)
	if err == nil {
		t.Fatal("expected Write to fail when temp creation is blocked")
	}

	// Restore permissions to inspect the directory contents.
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // 0700 matches the ~/.ssh directory contract under test
		t.Fatalf("restoring dir mode: %v", err)
	}

	// Original content must be untouched.
	got, err := os.ReadFile(target) //nolint:gosec // test reads back a controlled fixture path
	if err != nil {
		t.Fatalf("reading target after failed write: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("target content changed after failed write: got %q want %q", got, original)
	}

	// No temp file should be left behind in the target dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "gitid-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leaked temp file present: %s", e.Name())
		}
	}
}

// TestEnsureDir verifies that EnsureDir creates a missing directory with mode
// 0700 and chmods an already-existing directory to 0700.
func TestEnsureDir(t *testing.T) {
	t.Run("creates missing dir at 0700", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "ssh")
		if err := EnsureDir(sub, 0o700); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		info, err := os.Stat(sub)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("expected a directory")
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("mode mismatch: got %o want 0700", info.Mode().Perm())
		}
	})

	t.Run("chmods existing dir to 0700", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "ssh")
		if err := os.Mkdir(sub, 0o755); err != nil { //nolint:gosec // deliberately loose mode so EnsureDir must tighten it to 0700
			t.Fatalf("mkdir: %v", err)
		}
		if err := EnsureDir(sub, 0o700); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		info, err := os.Stat(sub)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("mode mismatch: got %o want 0700", info.Mode().Perm())
		}
	})
}

// TestWriteBackupNamesAreCollisionProof verifies that two Write calls over
// the same target in immediate succession produce two DISTINCT backup
// files, each with its own correct content intact — proving the
// nanosecond-resolution + exclusive-create naming scheme never lets a
// same-instant backup clobber a still-live recovery snapshot (Codex HIGH #1).
// Under the pre-fix second-resolution naming, two Write calls this close
// together reliably collided on the same backup path.
func TestWriteBackupNamesAreCollisionProof(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config")

	first := []byte("first-content\n")
	if err := os.WriteFile(target, first, 0o600); err != nil {
		t.Fatalf("seeding target: %v", err)
	}

	second := []byte("second-content\n")
	backup1, err := Write(target, second, 0o600)
	if err != nil {
		t.Fatalf("first Write returned error: %v", err)
	}

	third := []byte("third-content\n")
	backup2, err := Write(target, third, 0o600)
	if err != nil {
		t.Fatalf("second Write returned error: %v", err)
	}

	if backup1 == backup2 {
		t.Fatalf("two immediately successive backups collided on the same path: %q", backup1)
	}

	b1, err := os.ReadFile(backup1) //nolint:gosec // backup1 is a path returned by Write under test
	if err != nil {
		t.Fatalf("reading first backup: %v", err)
	}
	if string(b1) != string(first) {
		t.Errorf("first backup content mismatch: got %q want %q", b1, first)
	}

	b2, err := os.ReadFile(backup2) //nolint:gosec // backup2 is a path returned by Write under test
	if err != nil {
		t.Fatalf("reading second backup: %v", err)
	}
	if string(b2) != string(second) {
		t.Errorf("second backup content mismatch: got %q want %q", b2, second)
	}
}

// TestWriteNoBackupDoesNotCreateBackup verifies that WriteNoBackup
// atomically replaces an existing target's content WITHOUT creating any
// backup file — the dedicated rollback/restore seam (Codex HIGH #1) that
// sshconfig.Migrate's rollback uses so recovery never re-enters Write's own
// backup-creation step.
func TestWriteNoBackupDoesNotCreateBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config")
	original := []byte("original-content\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatalf("seeding target: %v", err)
	}

	restored := []byte("restored-content\n")
	if err := WriteNoBackup(target, restored, 0o600); err != nil {
		t.Fatalf("WriteNoBackup returned error: %v", err)
	}

	got, err := os.ReadFile(target) //nolint:gosec // test reads back the file it just wrote
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}
	if string(got) != string(restored) {
		t.Fatalf("content mismatch: got %q want %q", got, restored)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in dir (no backup created), got %d: %v", len(entries), entries)
	}
}
