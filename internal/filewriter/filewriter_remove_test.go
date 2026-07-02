package filewriter

import (
	"os"
	"path/filepath"
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
	stamp := strings.TrimPrefix(backupPath, prefix)
	if len(stamp) != len("20060102-150405") {
		t.Fatalf("backup timestamp %q is not in 20060102-150405 format", stamp)
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
