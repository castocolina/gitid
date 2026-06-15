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
	err := runIdentityUpdate(strings.NewReader(""), &out, "nonexistent", false, updateFlags{}, fakeUpdateDeps)
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
	err := runIdentityUpdate(strings.NewReader(""), &out, "bad name!", false, updateFlags{}, fakeUpdateDeps)
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
	// Prompts: git.name, git.email, alias, hostname, port, strategy, gitdir, signing.
	// dry-run: no confirm prompt, so 8 newlines.
	answers := strings.Repeat("\n", 8)
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "work", true, updateFlags{}, fakeUpdateDeps)
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
	// Prompts: git.name, git.email, alias, hostname, port, strategy, gitdir, signing.
	answers := strings.Repeat("\n", 8) + "n\n"
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "work", false, updateFlags{}, fakeUpdateDeps)
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
	// Prompts: git.name, git.email, alias, hostname, port, strategy, gitdir, signing.
	answers := strings.Repeat("\n", 8) + "y\n"
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "work", false, updateFlags{}, fakeUpdateDeps)
	if err != nil {
		t.Fatalf("runIdentityUpdate(confirmed) error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Identity updated.") {
		t.Errorf("expected success message, got:\n%s", out.String())
	}
}

// TestRunIdentityUpdate_NoProviderPrompt asserts finding #4: the standalone
// "Provider" prompt and its preview line are removed. Provider is purely
// alias-derived (loader.Reconstruct derives Account.Provider from the alias;
// it is never persisted as an independent artifact and Update never writes it),
// so a standalone Provider edit could not round-trip. The UI must not imply an
// edit that cannot persist — the alias prompt is the real lever.
func TestRunIdentityUpdate_NoProviderPrompt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticHome(t, home)

	// Without the Provider prompt there are now 8 editable prompts (the match
	// strategy picker adds 1 extra: strategy choice + value). Press Enter on each,
	// then dry-run (no confirm).
	// Prompts: git.name, git.email, alias, hostname, port, strategy, gitdir, signing.
	answers := strings.Repeat("\n", 8)
	var out bytes.Buffer
	if err := runIdentityUpdate(strings.NewReader(answers), &out, "work", true, updateFlags{}, fakeUpdateDeps); err != nil {
		t.Fatalf("runIdentityUpdate error: %v\noutput: %s", err, out.String())
	}

	s := out.String()
	if strings.Contains(s, "Provider") {
		t.Errorf("Provider prompt/preview must be removed (finding #4 — provider cannot round-trip standalone):\n%s", s)
	}
	// The alias prompt — the real lever for provider — must still be present.
	if !strings.Contains(s, "Host alias") {
		t.Errorf("Host alias prompt missing:\n%s", s)
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

// TestRunIdentityUpdate_HasConfigPitfall6 is the Pitfall 6 regression guard:
// an identity whose existing Matches contain a MatchHasconfig must NOT be silently
// collapsed to a gitdir-only Matches on update. The picker pre-fills from
// existing.Matches so the hasconfig condition is preserved unless the user
// actively changes it.
func TestRunIdentityUpdate_HasConfigPitfall6(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Write hermetic home with a hasconfig-only identity.
	sshDir := filepath.Join(home, ".ssh")
	gitconfigDDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir ssh: %v", err)
	}
	if err := os.MkdirAll(gitconfigDDir, 0o700); err != nil {
		t.Fatalf("mkdir gitconfigD: %v", err)
	}
	pubPath := filepath.Join(sshDir, "id_ed25519_urlid.pub")
	if err := os.WriteFile(pubPath, []byte("ssh-ed25519 AAAAFAKEPUB comment\n"), 0o600); err != nil { //nolint:gosec // G306: test fixture
		t.Fatalf("write pub: %v", err)
	}

	// SSH config with a managed block for "urlid".
	sshConfig := "# BEGIN gitid managed: urlid\nHost urlid.github\n  Hostname ssh.github.com\n  Port 443\n  IdentityFile " + pubPath + "\n  IdentitiesOnly yes\n# END gitid managed: urlid\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	// .gitconfig with a hasconfig-only includeIf block.
	fragPath := filepath.Join(gitconfigDDir, "urlid")
	gitconfigContent := "# BEGIN gitid managed: urlid\n[includeIf \"hasconfig:remote.*.url:git@ssh.github.com:urlowner/**\"]\n\tpath = " + fragPath + "\n# END gitid managed: urlid\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfigContent), 0o600); err != nil { //nolint:gosec // G306: test fixture
		t.Fatalf("write .gitconfig: %v", err)
	}
	fragContent := "[user]\n\tname = URL User\n\temail = url@example.com\n\tsigningkey = " + pubPath + "\n[gpg]\n\tformat = ssh\n"
	if err := os.WriteFile(fragPath, []byte(fragContent), 0o600); err != nil {
		t.Fatalf("write fragment: %v", err)
	}

	// Capture what WriteGitconfig receives for matches.
	var capturedMatches []gitconfig.Match
	trackingDeps := func(_ io.Writer) identity.UpdateDeps {
		d := fakeUpdateDeps(nil)
		d.WriteGitconfig = func(_, _, _ string, matches []gitconfig.Match) (string, error) {
			capturedMatches = matches
			return "", nil
		}
		return d
	}

	// Scripted: accept all defaults (strategy "2" pre-filled from hasconfig match,
	// accept the URL value default) + confirm.
	// Prompts: git.name, git.email, alias, hostname, port, strategy(→2), url-value, signing, confirm.
	answers := strings.Repeat("\n", 8) + "y\n"
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "urlid", false, updateFlags{}, trackingDeps)
	if err != nil {
		t.Fatalf("runIdentityUpdate(hasconfig identity) error: %v\noutput: %s", err, out.String())
	}

	// The captured matches must contain a MatchHasconfig — NOT a gitdir-only Matches.
	hasHashconfigMatch := false
	for _, m := range capturedMatches {
		if m.Kind == gitconfig.MatchHasconfig {
			hasHashconfigMatch = true
		}
		if m.Kind == gitconfig.MatchGitdir {
			t.Errorf("Pitfall 6 regression: hasconfig identity must not be collapsed to MatchGitdir on update; got MatchGitdir in matches: %+v", capturedMatches)
		}
	}
	if !hasHashconfigMatch {
		t.Errorf("Pitfall 6 regression: update must preserve MatchHasconfig; captured matches: %+v", capturedMatches)
	}
}

// TestRunIdentityUpdate_ProviderFlag asserts that --provider sets edited.Provider
// and the new value flows into WriteSSH (D-11/Q3). Without the flag, the existing
// provider is preserved.
func TestRunIdentityUpdate_ProviderFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticHome(t, home)

	var capturedHostBlock string
	trackingDeps := func(_ io.Writer) identity.UpdateDeps {
		d := fakeUpdateDeps(nil)
		d.WriteSSH = func(_, hostBlock, _ string) (string, error) {
			capturedHostBlock = hostBlock
			return "", nil
		}
		return d
	}

	// Scripted: all defaults + confirm.
	answers := strings.Repeat("\n", 8) + "y\n"
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "work", false, updateFlags{provider: "gitlab"}, trackingDeps)
	if err != nil {
		t.Fatalf("runIdentityUpdate(--provider gitlab) error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(capturedHostBlock, "gitlab") {
		t.Errorf("--provider gitlab must flow into the host block; got:\n%s", capturedHostBlock)
	}
}

// TestRunIdentityUpdate_GitdirFlag asserts that --gitdir skips the picker and
// builds a MatchGitdir with the flag value (D-09).
func TestRunIdentityUpdate_GitdirFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticHome(t, home)

	var capturedMatches []gitconfig.Match
	trackingDeps := func(_ io.Writer) identity.UpdateDeps {
		d := fakeUpdateDeps(nil)
		d.WriteGitconfig = func(_, _, _ string, matches []gitconfig.Match) (string, error) {
			capturedMatches = matches
			return "", nil
		}
		return d
	}

	// With --gitdir flag, the picker is skipped; we only need: git.name, git.email,
	// alias, hostname, port, signing, confirm — that's 6 prompts + 1 confirm.
	answers := strings.Repeat("\n", 6) + "y\n"
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "work", false, updateFlags{gitdir: "~/work/projects/"}, trackingDeps)
	if err != nil {
		t.Fatalf("runIdentityUpdate(--gitdir) error: %v\noutput: %s", err, out.String())
	}
	if len(capturedMatches) != 1 || capturedMatches[0].Kind != gitconfig.MatchGitdir {
		t.Errorf("--gitdir: want single MatchGitdir, got %+v", capturedMatches)
	}
	if capturedMatches[0].Value != "~/work/projects/" {
		t.Errorf("--gitdir: want Value %q, got %q", "~/work/projects/", capturedMatches[0].Value)
	}
}

// TestRunIdentityUpdate_URLFlag asserts that --url skips the picker and builds
// a MatchHasconfig with the "remote.*.url:" prefix (D-09).
func TestRunIdentityUpdate_URLFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermeticHome(t, home)

	var capturedMatches []gitconfig.Match
	trackingDeps := func(_ io.Writer) identity.UpdateDeps {
		d := fakeUpdateDeps(nil)
		d.WriteGitconfig = func(_, _, _ string, matches []gitconfig.Match) (string, error) {
			capturedMatches = matches
			return "", nil
		}
		return d
	}

	// With --url flag, the picker is skipped; we only need: git.name, git.email,
	// alias, hostname, port, signing, confirm — that's 6 prompts + 1 confirm.
	answers := strings.Repeat("\n", 6) + "y\n"
	var out bytes.Buffer
	err := runIdentityUpdate(strings.NewReader(answers), &out, "work", false, updateFlags{url: "git@ssh.github.com:myorg/**"}, trackingDeps)
	if err != nil {
		t.Fatalf("runIdentityUpdate(--url) error: %v\noutput: %s", err, out.String())
	}
	if len(capturedMatches) != 1 || capturedMatches[0].Kind != gitconfig.MatchHasconfig {
		t.Errorf("--url: want single MatchHasconfig, got %+v", capturedMatches)
	}
	want := "remote.*.url:git@ssh.github.com:myorg/**"
	if capturedMatches[0].Value != want {
		t.Errorf("--url: want Value %q, got %q", want, capturedMatches[0].Value)
	}
}

// TestNewUpdateCmd_NoNameFlag asserts that --name is NOT registered on the update
// command — name stays positional per Q2 (name is immutable; positional arg suffices).
func TestNewUpdateCmd_NoNameFlag(t *testing.T) {
	cmd := newUpdateCmd()
	if cmd.Flags().Lookup("name") != nil {
		t.Error("newUpdateCmd must NOT have a --name flag (name is positional per Q2)")
	}
}

// TestNewAddCmd_HasRequiredFlags asserts that newAddCmd registers --name,
// --gitdir, --url, and --provider flags (D-09).
func TestNewAddCmd_HasRequiredFlags(t *testing.T) {
	cmd := newAddCmd()
	for _, flagName := range []string{"name", "gitdir", "url", "provider"} {
		if cmd.Flags().Lookup(flagName) == nil {
			t.Errorf("newAddCmd must have --%s flag (D-09)", flagName)
		}
	}
}

// TestNewUpdateCmd_HasMatchFlags asserts that newUpdateCmd registers --gitdir,
// --url, and --provider flags (D-09).
func TestNewUpdateCmd_HasMatchFlags(t *testing.T) {
	cmd := newUpdateCmd()
	for _, flagName := range []string{"gitdir", "url", "provider"} {
		if cmd.Flags().Lookup(flagName) == nil {
			t.Errorf("newUpdateCmd must have --%s flag (D-09)", flagName)
		}
	}
}
