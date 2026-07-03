package filewriter

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestBackupAndRemove_ExistingFile verifies that BackupAndRemove creates a
// timestamped backup copy and removes the original, returning a non-empty
// backupPath.
func TestBackupAndRemove_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "fragment")
	content := []byte("fragment-content\n")
	if err := os.WriteFile(target, content, 0o600); err != nil {
		t.Fatalf("seeding target: %v", err)
	}

	backupPath, err := BackupAndRemove(target)
	if err != nil {
		t.Fatalf("BackupAndRemove returned error: %v", err)
	}
	if backupPath == "" {
		t.Fatal("expected non-empty backupPath for existing file")
	}

	// Original must be gone.
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("original file still exists after BackupAndRemove")
	}

	// Backup must exist and contain the original content.
	backupBytes, err := os.ReadFile(backupPath) //nolint:gosec // test reads backup path returned under test
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(backupBytes) != string(content) {
		t.Fatalf("backup content mismatch: got %q want %q", backupBytes, content)
	}

	// Backup filename format: <path>.bak.<timestamp>
	prefix := target + ".bak."
	if !strings.HasPrefix(backupPath, prefix) {
		t.Fatalf("backup name %q does not have prefix %q", backupPath, prefix)
	}
	// The backup uses the same collision-proof UnixNano suffix as Write
	// (a decimal nanosecond count), not the old fixed-width date format.
	stamp := strings.TrimPrefix(backupPath, prefix)
	if _, convErr := strconv.ParseInt(stamp, 10, 64); convErr != nil {
		t.Fatalf("backup timestamp %q is not a UnixNano decimal value: %v", stamp, convErr)
	}
}

// TestBackupAndRemove_MissingFile verifies that BackupAndRemove returns ("", nil)
// when the target file does not exist (idempotent).
func TestBackupAndRemove_MissingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nonexistent")

	backupPath, err := BackupAndRemove(target)
	if err != nil {
		t.Fatalf("BackupAndRemove returned error for missing file: %v", err)
	}
	if backupPath != "" {
		t.Fatalf("expected empty backupPath for missing file, got %q", backupPath)
	}
}
