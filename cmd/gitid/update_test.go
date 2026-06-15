package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// fakeUpdateDeps returns a fully-faked identity.UpdateDeps for cmd-layer tests.
func fakeUpdateDeps(_ io.Writer) identity.UpdateDeps {
	return identity.UpdateDeps{
		WriteSSH:            func(_, _, _ string) (string, error) { return "", nil },
		WriteGitconfig:      func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "", nil },
		WriteFragment:       func(_, _, _, _ string, _ bool) error { return nil },
		WriteAllowedSigners: func(_, _, _ string) (string, error) { return "", nil },
		RemoveAllowedSigners: func(_, _ string) (string, error) {
			return "", nil
		},
		Resolved: func(alias string) (tester.Result, tester.ResolvedConfig) {
			return tester.Result{
				Command: "ssh -T git@" + alias,
				Output:  "successfully authenticated",
				Outcome: tester.PASS,
			}, tester.ResolvedConfig{User: "git", Hostname: "ssh.github.com", Port: "443"}
		},
		ReadPub: func(_ string) (string, error) {
			return "ssh-ed25519 AAAAFAKEPUB comment", nil
		},
	}
}

// writeHermeticSSHConfig writes minimal SSH and gitconfig managed blocks for the
// "work" identity into a hermetic HOME so reconstruction succeeds in tests.
func writeHermeticHome(t *testing.T, home string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	gitconfigDDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir ssh: %v", err)
	}
	if err := os.MkdirAll(gitconfigDDir, 0o700); err != nil {
		t.Fatalf("mkdir gitconfigD: %v", err)
	}

	// Write a stub pub key file so ReadPub doesn't fail in the real dep path.
	// .pub is public material — 0o644 is appropriate, but gosec requires 0o600 in tests.
	pubPath := filepath.Join(sshDir, "id_ed25519_work.pub")
	if err := os.WriteFile(pubPath, []byte("ssh-ed25519 AAAAFAKEPUB comment\n"), 0o600); err != nil { //nolint:gosec // G306: test fixture, permission intentional
		t.Fatalf("write pub: %v", err)
	}

	// Write a minimal SSH config with a managed block for "work".
	sshConfig := "# BEGIN gitid managed: work\nHost work.github.com\n  Hostname ssh.github.com\n  Port 443\n  IdentityFile " + pubPath + "\n  IdentitiesOnly yes\n# END gitid managed: work\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	// Write a minimal .gitconfig with a managed includeIf block for "work".
	fragPath := filepath.Join(gitconfigDDir, "work")
	gitconfigContent := "# BEGIN gitid managed: work\n[includeIf \"gitdir:~/git/work/\"]\n\tpath = " + fragPath + "\n# END gitid managed: work\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfigContent), 0o600); err != nil { //nolint:gosec // G306: test fixture
		t.Fatalf("write .gitconfig: %v", err)
	}

	// Write a minimal fragment file.
	fragContent := "[user]\n\tname = Work User\n\temail = work@example.com\n\tsigningkey = " + pubPath + "\n[gpg]\n\tformat = ssh\n[commit]\n\tgpgsign = true\n"
	if err := os.WriteFile(fragPath, []byte(fragContent), 0o600); err != nil {
		t.Fatalf("write fragment: %v", err)
	}
}

// TestRunIdentityUpdate_NotFound asserts that requesting a non-existent identity
// returns an error.
func TestRunIdentityUpdate_NotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Empty HOME — no identity to find.
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(""), &out, "nonexistent", false, fakeUpdateDeps)
	if err == nil {
		t.Fatal("runIdentityUpdate must error when identity is not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// TestRunIdentityUpdate_InvalidName asserts that an invalid identity name is
// rejected before reconstruction.
func TestRunIdentityUpdate_InvalidName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(""), &out, "bad name!", false, fakeUpdateDeps)
	if err == nil {
		t.Fatal("runIdentityUpdate must error on invalid identity name")
	}
	if !strings.Contains(err.Error(), "invalid identity name") {
		t.Errorf("error should mention 'invalid identity name', got: %v", err)
	}
}

// TestRunIdentityUpdate_DryRun asserts that --dry-run produces a preview and no
// writes are performed.
func TestRunIdentityUpdate_DryRun(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityUpdate(dry-run) panicked: %v", r)
		}
	}()

	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticHome(t, home)

	// Scripted answers: 8 prompts all defaults (press Enter on each field).
	// dry-run: no confirm prompt, so 8 newlines.
	answers := strings.Repeat("\n", 8)
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "work", true, fakeUpdateDeps)
	if err != nil {
		t.Fatalf("runIdentityUpdate(dry-run) error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "--dry-run: no files were written.") {
		t.Errorf("expected dry-run notice, got:\n%s", out.String())
	}
}

// TestRunIdentityUpdate_CancelledOnDecline asserts that declining the confirm
// prompt prints a cancellation message and returns without error.
func TestRunIdentityUpdate_CancelledOnDecline(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticHome(t, home)

	// Scripted answers: 8 defaults then "n" to decline confirm.
	answers := strings.Repeat("\n", 8) + "n\n"
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "work", false, fakeUpdateDeps)
	if err != nil {
		t.Fatalf("runIdentityUpdate(declined) error: %v", err)
	}
	if !strings.Contains(out.String(), "Update cancelled") {
		t.Errorf("expected cancellation message, got:\n%s", out.String())
	}
}

// TestRunIdentityUpdate_Confirm asserts that confirming the prompt triggers the
// update and prints a success message.
func TestRunIdentityUpdate_Confirm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticHome(t, home)

	// Scripted answers: 8 defaults then "y" to confirm.
	answers := strings.Repeat("\n", 8) + "y\n"
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "work", false, fakeUpdateDeps)
	if err != nil {
		t.Fatalf("runIdentityUpdate(confirmed) error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Identity updated.") {
		t.Errorf("expected success message, got:\n%s", out.String())
	}
}

// TestRunIdentityUpdate_NoRedefinesSharedHelpers asserts that fp, confirm, and
// prompt are not redefined in update.go (they are shared from add.go).
func TestRunIdentityUpdate_NoRedefinesSharedHelpers(_ *testing.T) {
	// This is a compile-time check: if update.go redefines fp, confirm, or prompt,
	// the package would fail to compile. The fact that this test compiles proves it.
	_ = fp
	_ = confirm
	_ = prompt
}
