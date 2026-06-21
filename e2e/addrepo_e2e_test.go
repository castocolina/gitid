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

// TestAddRepo_LocalClone verifies that `gitid add repo <file-url> --client personal --yes`
// clones the repository into ~/git/personal/<reponame>.
//
// This test is RED — the `gitid add repo` subcommand does not yet exist.
// Wave 1 Plan 03 (05.7-03) turns this GREEN.
func TestAddRepo_LocalClone(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	cloneURL, repoName := setupLocalBareRepo(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "add", "repo", cloneURL,
		"--client", "personal", "--yes")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)

	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid add repo failed: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	// Assert: repo cloned into ~/git/personal/<reponame>.
	cloneDir := filepath.Join(home, "git", "personal", repoName)
	if _, err := os.Stat(cloneDir); err != nil {
		t.Errorf("expected clone dir %s to exist: %v", cloneDir, err)
	}
}

// TestAddRepo_AliasRewrite verifies that when a github.com HTTPS URL is given
// and a matching identity with SSH alias exists, `gitid add repo` rewrites the
// URL to use the SSH alias (git@personal.github.com:org/repo) in the output.
//
// This test is RED — the `gitid add repo` subcommand does not yet exist.
// Wave 1 Plan 03 (05.7-03) turns this GREEN.
func TestAddRepo_AliasRewrite(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSHDir := FakeSSHDir(t, "pass")

	// Seed an identity with SSH alias personal.github.com by running gitid identity add.
	stdin := strings.NewReader(strings.Join([]string{
		"1",             // mode: create-new
		"personal",      // identity name
		"Test User",     // git user.name
		"t@example.com", // git user.email
		"github",        // provider
		"",              // alias (default: personal.github.com)
		"",              // hostname (default)
		"",              // port (default)
		"",              // match strategy (default: gitdir)
		"",              // gitdir value (default)
		"",              // passphrase (empty)
	}, "\n") + "\n")

	setupCtx, setupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer setupCancel()

	var setupOut, setupErr bytes.Buffer
	setupCmd := exec.CommandContext(setupCtx, bin, "identity", "add")
	setupCmd.Stdin = stdin
	setupCmd.Stdout = &setupOut
	setupCmd.Stderr = &setupErr
	setupCmd.Env = append(os.Environ(),
		"HOME="+home,
		"GITID_FAKE_SSH_MODE=pass",
		"PATH="+fakeSSHDir+":"+os.Getenv("PATH"),
	)
	if err := setupCmd.Run(); err != nil {
		t.Fatalf("setup: gitid identity add failed: %v\nstdout: %s\nstderr: %s",
			err, setupOut.String(), setupErr.String())
	}

	// Now run gitid add repo with an HTTPS github.com URL.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "add", "repo",
		"https://github.com/org/repo", "--yes")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)

	// The command may exit non-zero when the rewritten URL cannot be cloned
	// (no network in e2e sandbox); what we test is the rewrite in output.
	_ = cmd.Run()

	combined := stdout.String() + stderr.String()
	// Assert: the rewritten SSH alias URL appears in output.
	// The alias form is personal.<provider> per DefaultAlias (e.g. personal.github
	// when provider is "github"). The full hostname alias personal.github.com is
	// only produced when the provider input is "github.com".
	if !strings.Contains(combined, "personal.github") {
		t.Errorf("expected SSH alias rewrite (personal.github) in output;\ngot:\n%s", combined)
	}
}
