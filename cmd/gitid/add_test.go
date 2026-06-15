package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// fakeDeps returns a fully-faked identity.Deps so the add handler test never
// touches the network, the real keygen, or the filesystem.
func fakeDeps(_ io.Writer) identity.Deps {
	return identity.Deps{
		Generate: func(in identity.CreateInput) (identity.KeyResult, error) {
			return identity.KeyResult{
				PrivatePath: "/tmp/.ssh/id_ed25519_" + in.Name,
				PubPath:     "/tmp/.ssh/id_ed25519_" + in.Name + ".pub",
				PubLine:     "ssh-ed25519 AAAAFAKE comment\n",
			}, nil
		},
		CopyPub: func(_ string) error { return nil },
		PreWrite: func(_, host string) tester.Result {
			return tester.Result{Command: "ssh -T git@" + host, Output: "Permission denied (publickey)", Outcome: tester.ReachableNotUploaded}
		},
		WriteSSH:            func(_, _, _ string) (string, error) { return "", nil },
		WriteGitconfig:      func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "", nil },
		WriteFragment:       func(_, _, _, _ string) error { return nil },
		WriteAllowedSigners: func(_, _, _ string) (string, error) { return "", nil },
		Resolved: func(alias string) (tester.Result, tester.ResolvedConfig) {
			return tester.Result{Command: "ssh -T git@" + alias, Output: "ok"}, tester.ResolvedConfig{User: "git"}
		},
	}
}

// TestUploadInstructionsGitHubBothKeys asserts the GitHub copy covers BOTH the
// authentication key and the signing key registrations and the settings URL
// (UP-01/UP-02).
func TestUploadInstructionsGitHubBothKeys(t *testing.T) {
	got := uploadInstructions("github")
	for _, want := range []string{
		"https://github.com/settings/ssh/new",
		"Authentication key",
		"Signing key",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("uploadInstructions(github) missing %q\n--- got ---\n%s", want, got)
		}
	}
}

// TestUploadInstructionsGitLabOneKey asserts the GitLab copy points at the
// GitLab key settings page and notes a single key serves both roles.
func TestUploadInstructionsGitLabOneKey(t *testing.T) {
	got := uploadInstructions("gitlab")
	if !strings.Contains(got, "https://gitlab.com/-/user_settings/ssh_keys") {
		t.Errorf("uploadInstructions(gitlab) missing settings URL\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "authentication & signing") {
		t.Errorf("uploadInstructions(gitlab) should note the key is for both auth and signing\n%s", got)
	}
}

// TestUploadInstructionsUnknownNonEmpty ensures an unknown provider still gets
// non-empty guidance.
func TestUploadInstructionsUnknownNonEmpty(t *testing.T) {
	if strings.TrimSpace(uploadInstructions("bitbucket")) == "" {
		t.Fatal("uploadInstructions(unknown) must not be empty")
	}
}

// TestRunIdentityAddDryRunDoesNotPanic drives the add handler in --dry-run mode
// with scripted prompt input and an isolated temp HOME, asserting it completes
// without panicking and writes nothing (the dry-run path performs no file
// mutations). It uses the recover panic-guard convention.
func TestRunIdentityAddDryRunDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityAdd() panicked: %v", r)
		}
	}()

	// Isolate HOME so no real ~/.ssh or ~/.gitconfig is touched.
	t.Setenv("HOME", t.TempDir())

	// Scripted answers: name, git name, git email, provider, alias(default),
	// hostname(default), port(default), match(default), passphrase(empty).
	in := strings.NewReader(strings.Join([]string{
		"gitidtest",
		"Test User",
		"test@example.com",
		"github",
		"", // alias default
		"", // hostname default
		"", // port default
		"", // match default
		"", // passphrase empty
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := runIdentityAdd(in, &out, true, fakeDeps); err != nil {
		t.Fatalf("runIdentityAdd(dry-run) returned error: %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "--dry-run: no files were written.") {
		t.Errorf("expected dry-run notice in output, got:\n%s", out.String())
	}
	// The unified preview must show the allowed_signers artifact.
	if !strings.Contains(out.String(), "allowed_signers") {
		t.Errorf("dry-run preview must include the allowed_signers artifact, got:\n%s", out.String())
	}
}
