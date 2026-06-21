package filewriter

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCopyFile verifies that CopyFile copies src content to dst via the
// safe-write chokepoint (backup + atomic + 0o644).
func TestCopyFile(t *testing.T) {
	t.Run("copies content to new dst", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "source.txt")
		dst := filepath.Join(dir, "dest.txt")

		content := []byte("[user]\n\tname = Alice\n\temail = alice@example.com\n")
		if err := os.WriteFile(src, content, 0o600); err != nil { //nolint:gosec // seeding test fixture (G306)
			t.Fatalf("seeding src: %v", err)
		}

		backupPath, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile returned error: %v", err)
		}
		if backupPath != "" {
			t.Errorf("expected empty backupPath for new dst, got %q", backupPath)
		}

		got, err := os.ReadFile(dst) //nolint:gosec // dst is a controlled temp path (G304)
		if err != nil {
			t.Fatalf("reading dst: %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("content mismatch: got %q want %q", got, content)
		}

		// Mode must be exactly 0o644 (gitconfig fragment mode).
		info, err := os.Stat(dst)
		if err != nil {
			t.Fatalf("stat dst: %v", err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Errorf("mode mismatch: got %o want 0644", info.Mode().Perm())
		}
	})

	t.Run("backs up existing dst before overwriting", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "source.txt")
		dst := filepath.Join(dir, "dest.txt")

		original := []byte("original-content\n")
		updated := []byte("updated-content\n")

		if err := os.WriteFile(src, updated, 0o600); err != nil { //nolint:gosec // seeding test fixture (G306)
			t.Fatalf("seeding src: %v", err)
		}
		if err := os.WriteFile(dst, original, 0o600); err != nil { //nolint:gosec // seeding test fixture (G306)
			t.Fatalf("seeding dst: %v", err)
		}

		backupPath, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile returned error: %v", err)
		}
		if backupPath == "" {
			t.Error("expected non-empty backupPath for pre-existing dst")
		}

		// Backup must contain the original content.
		backupBytes, err := os.ReadFile(backupPath) //nolint:gosec // backupPath is a controlled temp path returned by CopyFile (G304)
		if err != nil {
			t.Fatalf("reading backup: %v", err)
		}
		if string(backupBytes) != string(original) {
			t.Errorf("backup content: got %q want %q", backupBytes, original)
		}

		// dst must hold the updated (src) content.
		got, err := os.ReadFile(dst) //nolint:gosec // dst is a controlled temp path (G304)
		if err != nil {
			t.Fatalf("reading dst: %v", err)
		}
		if string(got) != string(updated) {
			t.Errorf("dst content: got %q want %q", got, updated)
		}
	})

	t.Run("returns error for missing src", func(t *testing.T) {
		dir := t.TempDir()
		_, err := CopyFile(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "dst"))
		if err == nil {
			t.Error("expected error for missing src, got nil")
		}
	})
}
