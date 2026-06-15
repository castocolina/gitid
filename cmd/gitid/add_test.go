package main

import (
	"bufio"
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
		Generate: func(in identity.CreateInput) (identity.StagedKey, error) {
			return identity.StagedKey{
				TempPrivatePath:  "/tmp/stage/key",
				FinalPrivatePath: "/tmp/.ssh/id_ed25519_" + in.Name,
				FinalPubPath:     "/tmp/.ssh/id_ed25519_" + in.Name + ".pub",
				PubLine:          "ssh-ed25519 AAAAFAKE comment\n",
				PrivPEM:          []byte("FAKEPEM"),
			}, nil
		},
		PersistKey: func(s identity.StagedKey) (identity.KeyResult, error) {
			return identity.KeyResult{
				PrivatePath: s.FinalPrivatePath,
				PubPath:     s.FinalPubPath,
				PubLine:     s.PubLine,
			}, nil
		},
		Cleanup: func(_ identity.StagedKey) {},
		CopyPub: func(_ string) error { return nil },
		PreWrite: func(_, hostname string, _ int) tester.Result {
			return tester.Result{Command: "ssh -T git@" + hostname, Output: "Permission denied (publickey)", Outcome: tester.ReachableNotUploaded}
		},
		WriteSSH:            func(_, _, _ string) (string, error) { return "", nil },
		WriteGitconfig:      func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "", nil },
		WriteFragment:       func(_, _, _, _ string) error { return nil },
		WriteAllowedSigners: func(_, _, _ string) (string, error) { return "", nil },
		Resolved: func(alias string) (tester.Result, tester.ResolvedConfig) {
			return tester.Result{Command: "ssh -T git@" + alias, Output: "ok"}, tester.ResolvedConfig{User: "git"}
		},
		PubExists: func(_ string) bool { return true },
		DerivePub: func(_ string) (string, error) { return "ssh-ed25519 AAAADERIVED comment\n", nil },
		WritePub:  func(_, _ string) error { return nil },
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

	// Scripted answers: mode(1=create-new), name, git name, git email, provider,
	// alias(default), hostname(default), port(default), match(default),
	// passphrase(empty).
	in := strings.NewReader(strings.Join([]string{
		"1", // create-new mode (D-10)
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

// TestRunIdentityAddReuseDryRun drives the reuse-existing-key mode (D-10 mode 2)
// in --dry-run with scripted input, asserting it dispatches to the reuse path
// (prompting for the existing key) and completes without panic or write.
func TestRunIdentityAddReuseDryRun(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityAdd(reuse) panicked: %v", r)
		}
	}()

	t.Setenv("HOME", t.TempDir())

	// mode(2=reuse), name, git name, git email, provider, alias, hostname, port,
	// match, passphrase, existing-key-path.
	in := strings.NewReader(strings.Join([]string{
		"2", // reuse-existing-key mode
		"reused",
		"Reuse User",
		"reuse@example.com",
		"github",
		"", "", "", "", "", // alias/hostname/port/match/passphrase defaults
		"/tmp/.ssh/id_ed25519_reused", // existing key path
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := runIdentityAdd(in, &out, true, fakeDeps); err != nil {
		t.Fatalf("runIdentityAdd(reuse dry-run) error: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "--dry-run: no files were written.") {
		t.Errorf("expected dry-run notice, got:\n%s", out.String())
	}
}

// TestGatherCreateInputRejectsUnsafeName is a table-driven RED test asserting that
// gatherCreateInput rejects unsafe identity names before the name flows into
// filepath.Join or key paths (T-02-23 mitigation).
func TestGatherCreateInputRejectsUnsafeName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{name: "../evil", wantErr: true},
		{name: "a/b", wantErr: true},
		{name: "foo bar", wantErr: true},
		{name: "name;rm", wantErr: true},
		{name: "work", wantErr: false},
		{name: "personal.gh", wantErr: false},
		{name: "my-id_2", wantErr: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			// Scripted answers: identity name, then valid defaults for the
			// remaining prompts (gitName, gitEmail, provider, alias, hostname,
			// port, match, passphrase). dryRun=true so no confirm prompt.
			answers := strings.Join([]string{
				tc.name,         // Identity name
				"Test User",     // Git user.name
				"t@example.com", // Git user.email
				"github",        // Provider
				"",              // alias (default)
				"",              // hostname (default)
				"",              // port (default)
				"",              // match (default)
				"",              // passphrase (empty)
			}, "\n") + "\n"
			r := strings.NewReader(answers)
			var out bytes.Buffer
			_, err := gatherCreateInput(bufio.NewReader(r), &out, "ed25519", true)
			if tc.wantErr && err == nil {
				t.Errorf("gatherCreateInput(%q): expected error, got nil", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("gatherCreateInput(%q): unexpected error: %v", tc.name, err)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "invalid identity name") {
				t.Errorf("gatherCreateInput(%q): error %q does not mention \"invalid identity name\"", tc.name, err.Error())
			}
		})
	}
}

// TestGatherAddAccountRejectsUnsafeName is a table-driven RED test asserting that
// gatherAddAccount rejects unsafe existing identity names before the name flows
// into filepath.Join (T-02-23 mitigation).
func TestGatherAddAccountRejectsUnsafeName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{name: "../evil", wantErr: true},
		{name: "a/b", wantErr: true},
		{name: "foo bar", wantErr: true},
		{name: "name;rm", wantErr: true},
		{name: "work", wantErr: false},
		{name: "personal.gh", wantErr: false},
		{name: "my-id_2", wantErr: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			// For accept cases: supply all required answers so the function
			// runs to completion. For reject cases only the first answer
			// (name) matters; the function returns early on name rejection.
			answers := strings.Join([]string{
				tc.name,                     // Existing identity name
				"Work User",                 // Git user.name
				"work@example.com",          // Git user.email
				"/tmp/.ssh/id_ed25519_work", // Existing private key path
				"gitlab",                    // New provider
				"",                          // New host alias (default)
				"",                          // Hostname (default)
				"",                          // Port (default)
				"",                          // Match gitdir (default)
			}, "\n") + "\n"
			r := strings.NewReader(answers)
			var out bytes.Buffer
			_, _, _, err := gatherAddAccount(bufio.NewReader(r), &out)
			if tc.wantErr && err == nil {
				t.Errorf("gatherAddAccount(%q): expected error, got nil", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("gatherAddAccount(%q): unexpected error: %v", tc.name, err)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "invalid identity name") {
				t.Errorf("gatherAddAccount(%q): error %q does not mention \"invalid identity name\"", tc.name, err.Error())
			}
		})
	}
}

// TestRunIdentityAddAddAccountDryRun drives the add-account mode (D-10 mode 3) in
// --dry-run, asserting the rendered alias preview reuses the existing key path
// (IDENT-06) and no write occurs.
func TestRunIdentityAddAddAccountDryRun(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityAdd(add-account) panicked: %v", r)
		}
	}()

	t.Setenv("HOME", t.TempDir())

	keyPath := "/tmp/.ssh/id_ed25519_work"
	// mode(3=add-account), existing name, git name, git email, existing key path,
	// new provider, new alias, hostname, port, match.
	in := strings.NewReader(strings.Join([]string{
		"3", // add-account mode
		"work",
		"Work User",
		"work@example.com",
		keyPath,
		"gitlab",
		"work.gitlab.com",
		"", "", "", // hostname/port/match defaults
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := runIdentityAdd(in, &out, true, fakeDeps); err != nil {
		t.Fatalf("runIdentityAdd(add-account dry-run) error: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), keyPath) {
		t.Errorf("add-account preview must reuse the existing key path %q, got:\n%s", keyPath, out.String())
	}
	if !strings.Contains(out.String(), "Host work.gitlab.com") {
		t.Errorf("add-account preview must declare the new alias, got:\n%s", out.String())
	}
}
