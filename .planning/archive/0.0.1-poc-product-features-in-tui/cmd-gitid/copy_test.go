package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/uploader"
)

// TestRunCopyInvalidName verifies that runCopy rejects an invalid identity name
// (contains a space) with a non-nil error before touching the filesystem (T-05-05).
func TestRunCopyInvalidName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	err := runCopy(&out, "Bad Name", false, false, buildUploaderDeps)
	if err == nil {
		t.Fatal("runCopy with invalid name must return error")
	}
}

// TestRunCopyNotFound verifies that runCopy with a valid but nonexistent identity
// name returns a non-nil error (identity not found in reconstructed list).
func TestRunCopyNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	err := runCopy(&out, "nonexistent", false, false, buildUploaderDeps)
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
	_ = runCopy(&out, "testid", false, false, buildUploaderDeps)
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

// seedMinimalCopyIdentity writes the SSH config, gitconfig, and public key files
// needed to reconstruct a minimal "testid" identity in the given home directory.
// It returns the public key path for assertion use.
func seedMinimalCopyIdentity(t *testing.T, home string) string {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil { //nolint:gosec // test-controlled temp dir
		t.Fatalf("seedMinimalCopyIdentity: mkdir ~/.ssh: %v", err)
	}
	keyPath := filepath.Join(sshDir, "id_ed25519_testid")
	pubPath := keyPath + ".pub"
	if err := os.WriteFile(pubPath, []byte("ssh-ed25519 AAAACFAKE testid-key\n"), 0o600); err != nil { //nolint:gosec // test public key
		t.Fatalf("seedMinimalCopyIdentity: write .pub: %v", err)
	}
	sshBlock := "# BEGIN gitid managed: testid\n" +
		"Host testid.github\n" +
		"  Hostname github.com\n" +
		"  User git\n" +
		"  IdentityFile " + keyPath + "\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: testid\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshBlock), 0o600); err != nil { //nolint:gosec // test ssh config
		t.Fatalf("seedMinimalCopyIdentity: write ssh config: %v", err)
	}
	gcBlock := "# BEGIN gitid managed: testid\n" +
		"[includeIf \"gitdir:~/git/testid/\"]\n" +
		"  path = " + filepath.Join(home, ".gitconfig.d", "testid") + "\n" +
		"# END gitid managed: testid\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gcBlock), 0o644); err != nil { //nolint:gosec // test gitconfig
		t.Fatalf("seedMinimalCopyIdentity: write gitconfig: %v", err)
	}
	return pubPath
}

// fakeUploaderDeps returns a func()->uploader.Deps factory that simulates the
// given authentication state without executing any real binaries.
func fakeUploaderDeps(ghPresent, authOK bool) func() uploader.Deps {
	return func() uploader.Deps {
		return uploader.Deps{
			LookPath: func(name string) (string, error) {
				if name == "gh" && ghPresent {
					return "/fake/gh", nil
				}
				return "", os.ErrNotExist
			},
			RunCmd: func(_ string, args ...string) (string, int, error) {
				// Probe: "auth status" → simulate auth check.
				if len(args) >= 2 && args[0] == "auth" && args[1] == "status" {
					if authOK {
						return "Logged in to github.com", 0, nil
					}
					return "not logged in", 1, nil
				}
				// Upload call: "ssh-key add ..." → always succeeds when reached.
				if len(args) >= 2 && args[0] == "ssh-key" && args[1] == "add" {
					return "Added SSH key.", 0, nil
				}
				return "", 0, nil
			},
		}
	}
}

// TestRunCopy_UploadKeys_GHAuthenticated verifies that with --upload-keys and an
// authenticated gh tool, the command prints the gh ssh-key add command preview
// and the upload result (UI-SPEC §4a — show command before running it).
func TestRunCopy_UploadKeys_GHAuthenticated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	pubPath := seedMinimalCopyIdentity(t, home)

	var out bytes.Buffer
	// runCopy must succeed — upload failures are non-blocking by design (D-11).
	err := runCopy(&out, "testid", true, true, fakeUploaderDeps(true, true))
	if err != nil {
		t.Fatalf("runCopy returned error: %v", err)
	}

	got := out.String()
	// Assert: the command preview contains the --type authentication flag.
	if !strings.Contains(got, "--type authentication") {
		t.Errorf("expected '--type authentication' in output; got:\n%s", got)
	}
	// Assert: the upload result is present.
	if !strings.Contains(got, "Added SSH key") {
		t.Errorf("expected 'Added SSH key' upload result; got:\n%s", got)
	}
	// Security assertion (T-05.7-06-03): private key path must NOT appear —
	// only the .pub path is passed to UploadKey.
	privPath := strings.TrimSuffix(pubPath, ".pub")
	if strings.Contains(got, privPath) && !strings.Contains(got, pubPath) {
		t.Errorf("private key path %q appeared in output — only .pub must be passed", privPath)
	}
}

// TestRunCopy_UploadKeys_GHNotAuthenticated verifies the not-logged-in fallback:
// the command prints the not-authenticated notice and manual instructions.
// The command must still succeed (non-blocking, D-11).
func TestRunCopy_UploadKeys_GHNotAuthenticated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = seedMinimalCopyIdentity(t, home)

	var out bytes.Buffer
	_ = runCopy(&out, "testid", true, true, fakeUploaderDeps(true, false))

	got := out.String()
	// Assert: not-authenticated notice appears in output.
	if !strings.Contains(got, "not authenticated") {
		t.Errorf("expected 'not authenticated' notice; got:\n%s", got)
	}
}

// TestRunCopy_UploadKeys_GHNotPresent verifies that when gh is absent, no
// assisted-upload section appears — only manual instructions (non-blocking).
func TestRunCopy_UploadKeys_GHNotPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = seedMinimalCopyIdentity(t, home)

	var out bytes.Buffer
	_ = runCopy(&out, "testid", true, true, fakeUploaderDeps(false, false))

	got := out.String()
	// Assert: no assisted-upload markers when gh is absent.
	if strings.Contains(got, "ssh-key add") {
		t.Errorf("'ssh-key add' must NOT appear when gh is absent;\ngot:\n%s", got)
	}
}

// TestBuildUploaderDepsNonNil verifies that buildUploaderDeps returns a Deps
// value with both function fields populated (D-16 nil-guard for cmd surface).
func TestBuildUploaderDepsNonNil(t *testing.T) {
	deps := buildUploaderDeps()
	if deps.LookPath == nil {
		t.Error("buildUploaderDeps: LookPath nil")
	}
	if deps.RunCmd == nil {
		t.Error("buildUploaderDeps: RunCmd nil")
	}
}
