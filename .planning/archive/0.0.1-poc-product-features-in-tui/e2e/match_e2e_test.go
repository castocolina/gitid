//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMatchStrategy_Gitdir verifies that when the match strategy picker receives
// "1" (gitdir), the ~/.gitconfig contains an [includeIf "gitdir:..."] condition
// and does NOT contain a hasconfig condition.
//
// This test is RED until MATCH-URL-01 is implemented (the current binary has no
// match strategy picker — it always uses gitdir without a choice prompt).
func TestMatchStrategy_Gitdir(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSHDir := FakeSSHDir(t, "pass")

	// After MATCH-URL-01: prompt includes a strategy picker (1=gitdir, 2=hasconfig,
	// 3=both) between port and passphrase. Current binary has no picker prompt.
	stdin := strings.NewReader(strings.Join([]string{
		"1",             // mode: create-new
		"testid",        // identity name
		"Test User",     // git user.name
		"t@example.com", // git user.email
		"github",        // provider
		"",              // alias (default)
		"",              // hostname (default)
		"",              // port (default)
		"1",             // match strategy: 1=gitdir (MATCH-URL-01 new picker)
		"",              // gitdir value (default ~/git/testid/)
		"",              // passphrase (empty)
		"y",             // confirm (current binary; absent after FIX-CREATE-01 PASS auto-persist)
	}, "\n") + "\n")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "identity", "add")
	cmd.Stdin = stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"GITID_FAKE_SSH_MODE=pass",
		"PATH="+fakeSSHDir+":"+os.Getenv("PATH"),
	)

	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid identity add failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	// Assert: ~/.gitconfig contains gitdir includeIf.
	gitconfigPath := filepath.Join(home, ".gitconfig")
	assertFileExists(t, gitconfigPath, "~/.gitconfig")
	content, err := os.ReadFile(gitconfigPath)
	if err != nil {
		t.Fatalf("reading ~/.gitconfig: %v", err)
	}
	if !strings.Contains(string(content), `[includeIf "gitdir:`) {
		t.Errorf("~/.gitconfig missing [includeIf \"gitdir:...\"]; content:\n%s", content)
	}
	// Assert: no hasconfig condition when strategy=1 (gitdir only).
	if strings.Contains(string(content), `hasconfig:`) {
		t.Errorf("~/.gitconfig must not contain hasconfig: when strategy=gitdir; content:\n%s", content)
	}
}

// TestMatchStrategy_HasConfig verifies that when the match strategy picker
// receives "2" (hasconfig URL), the ~/.gitconfig contains an
// [includeIf "hasconfig:remote.*.url:..."] condition (D-07, D-08).
//
// This test is RED until MATCH-URL-01 is implemented.
func TestMatchStrategy_HasConfig(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSHDir := FakeSSHDir(t, "pass")

	// After MATCH-URL-01: strategy=2 → URL pattern prompt.
	stdin := strings.NewReader(strings.Join([]string{
		"1",             // mode: create-new
		"testid",        // identity name
		"Test User",     // git user.name
		"t@example.com", // git user.email
		"github",        // provider
		"",              // alias (default)
		"",              // hostname (default)
		"",              // port (default)
		"2",             // match strategy: 2=hasconfig URL (MATCH-URL-01 new picker)
		"",              // URL pattern (default: git@ssh.github.com:testid/**)
		"",              // passphrase (empty)
		"y",             // confirm (current binary)
	}, "\n") + "\n")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "identity", "add")
	cmd.Stdin = stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"GITID_FAKE_SSH_MODE=pass",
		"PATH="+fakeSSHDir+":"+os.Getenv("PATH"),
	)

	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid identity add failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	// Assert: ~/.gitconfig contains hasconfig includeIf.
	gitconfigPath := filepath.Join(home, ".gitconfig")
	assertFileExists(t, gitconfigPath, "~/.gitconfig")
	content, err := os.ReadFile(gitconfigPath)
	if err != nil {
		t.Fatalf("reading ~/.gitconfig: %v", err)
	}
	if !strings.Contains(string(content), `hasconfig:remote.*.url:`) {
		t.Errorf("~/.gitconfig missing hasconfig:remote.*.url: condition; content:\n%s", content)
	}
}

// TestMatchStrategy_Both verifies that when the match strategy picker receives
// "3" (both), the ~/.gitconfig contains TWO [includeIf] conditions: one gitdir
// and one hasconfig (D-07).
//
// This test is RED until MATCH-URL-01 is implemented.
func TestMatchStrategy_Both(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSHDir := FakeSSHDir(t, "pass")

	// After MATCH-URL-01: strategy=3 → gitdir prompt + URL pattern prompt.
	stdin := strings.NewReader(strings.Join([]string{
		"1",             // mode: create-new
		"testid",        // identity name
		"Test User",     // git user.name
		"t@example.com", // git user.email
		"github",        // provider
		"",              // alias (default)
		"",              // hostname (default)
		"",              // port (default)
		"3",             // match strategy: 3=both (MATCH-URL-01)
		"",              // gitdir value (default)
		"",              // URL pattern (default)
		"",              // passphrase (empty)
		"y",             // confirm (current binary)
	}, "\n") + "\n")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "identity", "add")
	cmd.Stdin = stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"GITID_FAKE_SSH_MODE=pass",
		"PATH="+fakeSSHDir+":"+os.Getenv("PATH"),
	)

	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid identity add failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	// Assert: ~/.gitconfig contains BOTH includeIf conditions.
	gitconfigPath := filepath.Join(home, ".gitconfig")
	assertFileExists(t, gitconfigPath, "~/.gitconfig")
	content, err := os.ReadFile(gitconfigPath)
	if err != nil {
		t.Fatalf("reading ~/.gitconfig: %v", err)
	}
	if !strings.Contains(string(content), `[includeIf "gitdir:`) {
		t.Errorf("~/.gitconfig missing [includeIf \"gitdir:...\"]; content:\n%s", content)
	}
	if !strings.Contains(string(content), `hasconfig:remote.*.url:`) {
		t.Errorf("~/.gitconfig missing hasconfig:remote.*.url: condition; content:\n%s", content)
	}
}

// TestListProvider_MarkerlessFallback verifies that `gitid identity list` shows
// "provider: github" for a legacy-style SSH Host block that has no
// "# gitid: provider=" marker comment (FIX-RECON-01 end-to-end, D-12).
// The hostname-to-provider fallback in identity.Reconstruct must map
// "ssh.github.com" → "github" transparently.
func TestListProvider_MarkerlessFallback(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	// Seed a markerless SSH managed block: no "# gitid: provider=" comment.
	sshConfigContent := "" +
		"# BEGIN gitid managed: legacyid\n" +
		"Host legacyid.github\n" +
		"  User git\n" +
		"  Hostname ssh.github.com\n" +
		"  Port 443\n" +
		"  IdentityFile " + filepath.Join(home, ".ssh", "id_ed25519_legacyid") + "\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: legacyid\n"

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("creating .ssh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(sshConfigContent), 0o600); err != nil { //nolint:gosec // test-only sandbox file (G306)
		t.Fatalf("writing ~/.ssh/config: %v", err)
	}

	// Seed a matching gitconfig managed block so the identity is complete.
	gitconfigContent := "" +
		"# BEGIN gitid managed: legacyid\n" +
		"[includeIf \"gitdir:~/git/legacyid/\"]\n" +
		"\tpath = " + filepath.Join(home, ".gitconfig.d", "legacyid") + "\n" +
		"# END gitid managed: legacyid\n"

	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfigContent), 0o600); err != nil { //nolint:gosec // test-only sandbox file (G306)
		t.Fatalf("writing ~/.gitconfig: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "identity", "list")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)

	// list exits 0 when identities exist; we ignore the exit code because a
	// missing fragment file causes "! incomplete" but still shows provider.
	_ = cmd.Run()

	combined := stdout.String() + stderr.String()

	// Assert: provider derived from hostname fallback (FIX-RECON-01, D-12).
	if !strings.Contains(combined, "provider: github") {
		t.Errorf("identity list must show 'provider: github' for markerless block with ssh.github.com;\nstdout:\n%s\nstderr:\n%s",
			stdout.String(), stderr.String())
	}
}
