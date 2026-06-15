package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunCopyInvalidName verifies that runCopy rejects an invalid identity name
// (contains a space) with a non-nil error before touching the filesystem (T-05-05).
func TestRunCopyInvalidName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	err := runCopy(&out, "Bad Name")
	if err == nil {
		t.Fatal("runCopy with invalid name must return error")
	}
}

// TestRunCopyNotFound verifies that runCopy with a valid but nonexistent identity
// name returns a non-nil error (identity not found in reconstructed list).
func TestRunCopyNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	err := runCopy(&out, "nonexistent")
	if err == nil {
		t.Fatal("runCopy with nonexistent identity must return error")
	}
}

// TestCopyCmdUseString verifies that newCopyCmd returns a command with the
// correct Use string (CLI-01 / D-06).
func TestCopyCmdUseString(t *testing.T) {
	cmd := newCopyCmd()
	if cmd.Use != "copy <name>" {
		t.Errorf("newCopyCmd().Use = %q, want %q", cmd.Use, "copy <name>")
	}
}

// TestIdentityCopyCmdUseString verifies that newIdentityCopyCmd returns a
// command with the correct Use string.
func TestIdentityCopyCmdUseString(t *testing.T) {
	cmd := newIdentityCopyCmd()
	if cmd.Use != "copy <name>" {
		t.Errorf("newIdentityCopyCmd().Use = %q, want %q", cmd.Use, "copy <name>")
	}
}

// TestHostAddCmdUseString verifies that newHostAddCmd returns a command with
// the correct Use string (CLI-01 / D-07).
func TestHostAddCmdUseString(t *testing.T) {
	cmd := newHostAddCmd()
	if cmd.Use != "add" {
		t.Errorf("newHostAddCmd().Use = %q, want %q", cmd.Use, "add")
	}
}

// TestRunCopyOutputContainsKeyAndInstructions verifies that runCopy with a
// valid, reconstructable identity prints the pub key and upload instructions.
// Clipboard may be unavailable in CI; that is acceptable — the output must
// still contain the key and provider instructions (CLIP-02 graceful degradation).
func TestRunCopyOutputContainsKeyAndInstructions(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil { //nolint:gosec // test-controlled temp dir
		t.Fatalf("mkdir ~/.ssh: %v", err)
	}

	pubLine := "ssh-ed25519 AAAACFAKE testid-key\n"
	keyPath := filepath.Join(sshDir, "id_ed25519_testid")
	pubPath := keyPath + ".pub"
	if err := os.WriteFile(pubPath, []byte(pubLine), 0o600); err != nil { //nolint:gosec // test public key
		t.Fatalf("write .pub: %v", err)
	}

	// Write a minimal gitid-managed SSH config block for "testid"
	sshConfigPath := filepath.Join(sshDir, "config")
	sshBlock := "# BEGIN gitid managed: testid\n" +
		"Host testid.github\n" +
		"  Hostname github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile " + keyPath + "\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: testid\n"
	if err := os.WriteFile(sshConfigPath, []byte(sshBlock), 0o600); err != nil { //nolint:gosec // test ssh config
		t.Fatalf("write ssh config: %v", err)
	}

	// Write a minimal gitid-managed includeIf block in ~/.gitconfig
	gitconfigPath := filepath.Join(tmpHome, ".gitconfig")
	gcBlock := "# BEGIN gitid managed: testid\n" +
		"[includeIf \"gitdir:~/git/testid/\"]\n" +
		"  path = " + filepath.Join(tmpHome, ".gitconfig.d", "testid") + "\n" +
		"# END gitid managed: testid\n"
	if err := os.WriteFile(gitconfigPath, []byte(gcBlock), 0o644); err != nil { //nolint:gosec // test gitconfig
		t.Fatalf("write gitconfig: %v", err)
	}

	var out bytes.Buffer
	// runCopy may fail on clipboard (no tool in CI) — graceful degradation
	// means the key is still printed; we assert output, not absence of error.
	_ = runCopy(&out, "testid")
	got := out.String()

	// The key line must appear in output regardless of clipboard availability.
	if !strings.Contains(got, strings.TrimSpace(pubLine)) {
		t.Errorf("runCopy output does not contain public key line; got:\n%s", got)
	}
	// Output must contain either "Copied public key" (success) or a clipboard
	// fallback message.
	if !strings.Contains(got, "Copied public key") && !strings.Contains(got, "clipboard") {
		t.Errorf("runCopy output missing expected copy confirmation or fallback; got:\n%s", got)
	}
}
