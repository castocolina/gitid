package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/identity"
)

// fakeDeleteDeps returns a fully-faked identity.DeleteDeps for cmd-layer tests.
func fakeDeleteDeps(_ io.Writer) identity.DeleteDeps {
	return identity.DeleteDeps{
		ReadSSH:              func() ([]byte, error) { return []byte{}, nil },
		ReadGitconfig:        func() ([]byte, error) { return []byte{}, nil },
		WriteSSH:             func(_ []byte) (string, error) { return "ssh.bak", nil },
		WriteGitconfig:       func(_ []byte) (string, error) { return "gc.bak", nil },
		RemoveFragment:       func(_ string) (string, error) { return "frag.bak", nil },
		RemoveAllowedSigners: func(_, _ string) (string, error) { return "sign.bak", nil },
		RemoveKeyFiles:       func(_, _ string) (string, string, error) { return "", "", nil },
	}
}

// writeHermeticDeleteHome sets up a hermetic HOME with:
//   - ~/.ssh/config containing a managed block for "work"
//   - ~/.gitconfig containing a managed includeIf block for "work"
//   - ~/.gitconfig.d/work minimal fragment
func writeHermeticDeleteHome(t *testing.T, home string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	gitconfigDDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir ssh: %v", err)
	}
	if err := os.MkdirAll(gitconfigDDir, 0o700); err != nil {
		t.Fatalf("mkdir gitconfigD: %v", err)
	}

	keyPath := filepath.Join(sshDir, "id_ed25519_work")
	fragPath := filepath.Join(gitconfigDDir, "work")

	// Write minimal SSH config with a managed block for "work".
	sshConfig := "# BEGIN gitid managed: work\nHost work.github.com\n  Hostname ssh.github.com\n  Port 443\n  IdentityFile " + keyPath + "\n  IdentitiesOnly yes\n# END gitid managed: work\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	// Write minimal .gitconfig with a managed includeIf block for "work".
	gitconfigContent := "# BEGIN gitid managed: work\n[includeIf \"gitdir:~/git/work/\"]\n\tpath = " + fragPath + "\n# END gitid managed: work\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfigContent), 0o600); err != nil { //nolint:gosec // G306: test fixture
		t.Fatalf("write .gitconfig: %v", err)
	}

	// Write minimal fragment file.
	fragContent := "[user]\n\tname = Work User\n\temail = work@example.com\n"
	if err := os.WriteFile(fragPath, []byte(fragContent), 0o600); err != nil {
		t.Fatalf("write fragment: %v", err)
	}
}

// TestRunIdentityDelete_NotFound asserts that requesting a non-existent
// identity returns an error mentioning "no gitid-managed identity named".
func TestRunIdentityDelete_NotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var out bytes.Buffer
	err := runIdentityDelete(strings.NewReader(""), &out, "nonexistent", false, fakeDeleteDeps)
	if err == nil {
		t.Fatal("runIdentityDelete must error when identity is not found")
	}
	if !strings.Contains(err.Error(), "no gitid-managed identity named") {
		t.Errorf("error should mention 'no gitid-managed identity named', got: %v", err)
	}
}

// TestRunIdentityDelete_InvalidName asserts that an invalid identity name is
// rejected before reconstruction.
func TestRunIdentityDelete_InvalidName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var out bytes.Buffer
	err := runIdentityDelete(strings.NewReader(""), &out, "bad name!", false, fakeDeleteDeps)
	if err == nil {
		t.Fatal("runIdentityDelete must error on invalid identity name")
	}
	if !strings.Contains(err.Error(), "invalid identity name") {
		t.Errorf("error should mention 'invalid identity name', got: %v", err)
	}
}

// TestRunIdentityDelete_DryRun asserts that --dry-run shows the manifest and
// no writes are performed.
func TestRunIdentityDelete_DryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticDeleteHome(t, home)

	var out bytes.Buffer
	// dry-run: no prompts needed (returns before confirm).
	err := runIdentityDelete(strings.NewReader(""), &out, "work", true, fakeDeleteDeps)
	if err != nil {
		t.Fatalf("runIdentityDelete(dry-run) error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "--dry-run: no files were written.") {
		t.Errorf("expected dry-run notice, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Will remove:") {
		t.Errorf("expected 'Will remove:' manifest in dry-run output, got:\n%s", out.String())
	}
}

// TestRunIdentityDelete_CancelledOnDecline asserts that declining the first
// confirm prompt prints a cancellation message and returns without error.
func TestRunIdentityDelete_CancelledOnDecline(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticDeleteHome(t, home)

	// Decline the first confirm prompt ("n").
	var out bytes.Buffer
	err := runIdentityDelete(strings.NewReader("n\n"), &out, "work", false, fakeDeleteDeps)
	if err != nil {
		t.Fatalf("runIdentityDelete(declined) error: %v", err)
	}
	if !strings.Contains(out.String(), "Delete cancelled") {
		t.Errorf("expected cancellation message, got:\n%s", out.String())
	}
}

// TestRunIdentityDelete_ConfirmKeepKey asserts that confirming deletion with
// the default key-keep (pressing Enter on second prompt) completes successfully.
func TestRunIdentityDelete_ConfirmKeepKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticDeleteHome(t, home)

	// First prompt: "y" to confirm deletion.
	// Second prompt: Enter (default = no = keep key).
	answers := "y\n\n"
	var out bytes.Buffer
	err := runIdentityDelete(strings.NewReader(answers), &out, "work", false, fakeDeleteDeps)
	if err != nil {
		t.Fatalf("runIdentityDelete(keep key) error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Identity deleted.") {
		t.Errorf("expected 'Identity deleted.', got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "kept (not deleted)") {
		t.Errorf("expected 'kept (not deleted)' in output, got:\n%s", out.String())
	}
}

// TestRunIdentityDelete_ConfirmDeleteKey asserts that confirming both prompts
// ("y" then "y") completes successfully and reports the key removal as backed
// up (CR-02: key files are recoverable, not irreversibly removed).
func TestRunIdentityDelete_ConfirmDeleteKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticDeleteHome(t, home)

	// First prompt: "y" to confirm deletion.
	// Second prompt: "y" to also delete the key.
	answers := "y\ny\n"
	var out bytes.Buffer
	err := runIdentityDelete(strings.NewReader(answers), &out, "work", false, fakeDeleteDeps)
	if err != nil {
		t.Fatalf("runIdentityDelete(delete key) error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Identity deleted.") {
		t.Errorf("expected 'Identity deleted.', got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "removed (backed up)") {
		t.Errorf("expected 'removed (backed up)' in output, got:\n%s", out.String())
	}
}

// TestRunIdentityDelete_ManifestContent verifies that the manifest contains
// all four removal items (D-08).
func TestRunIdentityDelete_ManifestContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticDeleteHome(t, home)

	var out bytes.Buffer
	// dry-run so we only see the manifest.
	_ = runIdentityDelete(strings.NewReader(""), &out, "work", true, fakeDeleteDeps)
	output := out.String()

	checks := []string{
		"[1]", // SSH Host block
		"[2]", // gitconfig block
		"[3]", // fragment file
		"[4]", // allowed_signers line
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("manifest missing %q, full output:\n%s", check, output)
		}
	}
}

// TestRunIdentityDelete_TwoConfirmCalls asserts that the two-step confirm
// flow (D-07) is actually exercised: two confirm prompts appear (first for
// block removal, second for key deletion). We verify by checking that accepting
// the first and declining the second results in "kept (not deleted)".
func TestRunIdentityDelete_TwoConfirmCalls(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticDeleteHome(t, home)

	// Accept first confirm, decline second.
	answers := "y\nn\n"
	var out bytes.Buffer
	err := runIdentityDelete(strings.NewReader(answers), &out, "work", false, fakeDeleteDeps)
	if err != nil {
		t.Fatalf("runIdentityDelete error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "kept (not deleted)") {
		t.Errorf("expected 'kept (not deleted)' after declining key-delete prompt, got:\n%s", out.String())
	}
}

// TestBuildDeleteDeps_KeyFilesBackedUpNotRawRemoved asserts that the real
// RemoveKeyFiles wiring routes BOTH the private key and the .pub through
// filewriter.BackupAndRemove: the originals are gone, timestamped .bak.<ts>
// copies remain on disk, and the backup paths are returned (CR-02).
func TestBuildDeleteDeps_KeyFilesBackedUpNotRawRemoved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir ssh: %v", err)
	}
	keyPath := filepath.Join(sshDir, "id_ed25519_work")
	pubPath := keyPath + ".pub"
	if err := os.WriteFile(keyPath, []byte("PRIVATE-KEY-BYTES"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.WriteFile(pubPath, []byte("ssh-ed25519 AAAA work\n"), 0o600); err != nil {
		t.Fatalf("write pub: %v", err)
	}

	deps := buildDeleteDeps(io.Discard)
	keyBak, pubBak, err := deps.RemoveKeyFiles(keyPath, pubPath)
	if err != nil {
		t.Fatalf("RemoveKeyFiles error: %v", err)
	}

	// Originals must be gone.
	if _, statErr := os.Stat(keyPath); !os.IsNotExist(statErr) {
		t.Errorf("private key still present at %s (expected removed)", keyPath)
	}
	if _, statErr := os.Stat(pubPath); !os.IsNotExist(statErr) {
		t.Errorf("public key still present at %s (expected removed)", pubPath)
	}

	// Backups must exist and be reported (recoverable, CR-02).
	if keyBak == "" {
		t.Fatal("expected a non-empty private key backup path (CR-02)")
	}
	if _, statErr := os.Stat(keyBak); statErr != nil {
		t.Errorf("private key backup missing at %s: %v", keyBak, statErr)
	}
	if pubBak == "" {
		t.Fatal("expected a non-empty public key backup path (CR-02)")
	}
	if _, statErr := os.Stat(pubBak); statErr != nil {
		t.Errorf("public key backup missing at %s: %v", pubBak, statErr)
	}
	if got, _ := os.ReadFile(keyBak); string(got) != "PRIVATE-KEY-BYTES" { //nolint:gosec // test reads its own backup
		t.Errorf("private key backup content = %q, want preserved original", string(got))
	}
}

// TestBuildDeleteDeps_RemoveKeyFilesMissingIsNoOp asserts that removing a key
// pair that does not exist is a no-op (no error, empty backup paths) — CR-02
// missing-file handling matches BackupAndRemove idempotency.
func TestBuildDeleteDeps_RemoveKeyFilesMissingIsNoOp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	deps := buildDeleteDeps(io.Discard)
	keyBak, pubBak, err := deps.RemoveKeyFiles(
		filepath.Join(home, ".ssh", "absent"),
		filepath.Join(home, ".ssh", "absent.pub"),
	)
	if err != nil {
		t.Fatalf("RemoveKeyFiles on missing files returned error: %v", err)
	}
	if keyBak != "" || pubBak != "" {
		t.Errorf("expected empty backup paths for missing files, got %q, %q", keyBak, pubBak)
	}
}

// TestRunIdentityDelete_NoRedefinesSharedHelpers asserts that fp, confirm,
// sanitizeName, and identityNameRe are not redefined in delete.go (they are
// shared from add.go/rotate.go). This is a compile-time check.
func TestRunIdentityDelete_NoRedefinesSharedHelpers(_ *testing.T) {
	_ = fp
	_ = confirm
	_ = sanitizeName
	_ = identityNameRe
}
