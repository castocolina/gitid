package gitconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitGet reads a single value back out of a fragment file via `git config`.
func gitGet(t *testing.T, fragPath, key string) string {
	t.Helper()
	out, err := exec.Command("git", "config", "--file", fragPath, "--get", key).Output() //nolint:gosec // fragPath is a t.TempDir()-derived test fixture path
	if err != nil {
		t.Fatalf("git config --get %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}

func TestWriteFragment_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	fragPath := filepath.Join(dir, "work")
	pubKeyPath := "~/.ssh/id_ed25519_work.pub"

	if err := WriteFragment(fragPath, "Work User", "work@example.com", pubKeyPath, true); err != nil {
		t.Fatalf("WriteFragment: %v", err)
	}

	if got := gitGet(t, fragPath, "user.name"); got != "Work User" {
		t.Errorf("user.name = %q, want %q", got, "Work User")
	}
	if got := gitGet(t, fragPath, "user.email"); got != "work@example.com" {
		t.Errorf("user.email = %q, want %q", got, "work@example.com")
	}
	if got := gitGet(t, fragPath, "gpg.format"); got != "ssh" {
		t.Errorf("gpg.format = %q, want %q", got, "ssh")
	}
	if got := gitGet(t, fragPath, "user.signingkey"); got != pubKeyPath {
		t.Errorf("user.signingkey = %q, want %q", got, pubKeyPath)
	}
	if got := gitGet(t, fragPath, "commit.gpgsign"); got != "true" {
		t.Errorf("commit.gpgsign = %q, want %q", got, "true")
	}
}

func TestWriteFragment_SigningKeyIsPathNotInline(t *testing.T) {
	dir := t.TempDir()
	fragPath := filepath.Join(dir, "work")
	pubKeyPath := "~/.ssh/id_ed25519_work.pub"

	if err := WriteFragment(fragPath, "Work User", "work@example.com", pubKeyPath, true); err != nil {
		t.Fatalf("WriteFragment: %v", err)
	}

	got := gitGet(t, fragPath, "user.signingkey")
	if got != pubKeyPath {
		t.Errorf("user.signingkey = %q, want the .pub path %q", got, pubKeyPath)
	}
	if strings.Contains(got, "ssh-ed25519 ") {
		t.Errorf("SIGN-02 violated: user.signingkey contains an inline key literal: %q", got)
	}
}

func TestWriteFragment_RejectsRemoteSection(t *testing.T) {
	// Pitfall 9: a [remote] section in a hasconfig fragment is a hard git circular
	// error. Reject any attempt to smuggle a remote in through the identity fields.
	dir := t.TempDir()
	fragPath := filepath.Join(dir, "work")

	err := WriteFragment(fragPath, "Work User", "work@example.com", "[remote \"origin\"]\n\turl = x", true)
	if err == nil {
		t.Errorf("expected WriteFragment to reject a [remote] section, got nil error")
	}
}

func TestWriteFragment_RejectsInvalidEmail(t *testing.T) {
	dir := t.TempDir()
	fragPath := filepath.Join(dir, "work")

	if err := WriteFragment(fragPath, "Work User", "not\nan@email", "~/.ssh/k.pub", true); err == nil {
		t.Errorf("expected WriteFragment to reject a newline-bearing email")
	}
}

func TestWriteFragment_CreatesParentDir(t *testing.T) {
	// fragPath is one level deeper than a directory that does NOT exist yet.
	// WriteFragment must ensure the parent dir before calling git config.
	fragPath := filepath.Join(t.TempDir(), "gitconfig.d", "work")

	if err := WriteFragment(fragPath, "Work User", "work@example.com", "~/.ssh/id_ed25519_work.pub", true); err != nil {
		t.Fatalf("WriteFragment: %v", err)
	}

	if _, err := os.Stat(fragPath); err != nil {
		t.Errorf("fragment file not created: %v", err)
	}
	if got := gitGet(t, fragPath, "user.email"); got != "work@example.com" {
		t.Errorf("user.email = %q, want %q", got, "work@example.com")
	}
}

// TestWriteFragment_LeadingDashValueIsLiteralNotGitOption proves the WR-15
// fix: gitConfigSet's `--` terminator means a value beginning with `-`
// (e.g. `--global`) is written as the literal value, never handed to
// git's own option parser as argument injection.
func TestWriteFragment_LeadingDashValueIsLiteralNotGitOption(t *testing.T) {
	dir := t.TempDir()
	fragPath := filepath.Join(dir, "work")
	dashLikeName := "--global"

	if err := WriteFragment(fragPath, dashLikeName, "work@example.com", "~/.ssh/id_ed25519_work.pub", true); err != nil {
		t.Fatalf("WriteFragment: %v", err)
	}
	if got := gitGet(t, fragPath, "user.name"); got != dashLikeName {
		t.Errorf("user.name = %q, want the literal dash-prefixed value %q", got, dashLikeName)
	}
}

// TestSetAllowedSignersFile_LeadingDashValueIsLiteral is the same WR-15
// proof for the second gitConfigSet call site (allowedSignersFile is a
// filesystem path, so a leading "-" is unusual but must still never be
// misparsed as an option).
func TestSetAllowedSignersFile_LeadingDashValueIsLiteral(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")
	dashLikePath := "--type=bool"

	if err := SetAllowedSignersFile(gitconfigPath, dashLikePath); err != nil {
		t.Fatalf("SetAllowedSignersFile: %v", err)
	}
	if got := gitGet(t, gitconfigPath, "gpg.ssh.allowedSignersFile"); got != dashLikePath {
		t.Errorf("gpg.ssh.allowedSignersFile = %q, want the literal dash-prefixed value %q", got, dashLikePath)
	}
}

func TestSetAllowedSignersFile(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")
	signers := "~/.ssh/allowed_signers"

	if err := SetAllowedSignersFile(gitconfigPath, signers); err != nil {
		t.Fatalf("SetAllowedSignersFile: %v", err)
	}
	if got := gitGet(t, gitconfigPath, "gpg.ssh.allowedSignersFile"); got != signers {
		t.Errorf("gpg.ssh.allowedSignersFile = %q, want %q", got, signers)
	}
}
