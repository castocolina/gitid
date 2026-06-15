package gitconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIncludeIfGitdir_ResolvesViaRealGit proves end-to-end that git actually
// resolves gitid's rendered [includeIf "gitdir:<dir>/"] block (GIT-02 / Pitfall 7).
//
// The test:
//  1. Builds a hermetic HOME in t.TempDir() so the developer's real ~/.gitconfig
//     and system gitconfig never participate (GIT_CONFIG_NOSYSTEM=1).
//  2. Calls WriteFragment to write a known user.email into a per-identity fragment
//     at the production ~/.gitconfig.d/<name> location.
//  3. Calls RenderIncludeIf (gitid's own renderer) to produce the managed block,
//     then writes that block into $HOME/.gitconfig as the includeIf stanza.
//  4. Runs `git init` to create a real repository under the gitdir condition path.
//  5. Runs `git config user.email` with cmd.Dir = repoDir and HOME pinned to the
//     temp home, asserting the returned email equals the fragment's value.
//
// This confirms the trailing-slash normalisation in condition() (Pitfall 7 / D-13)
// is sufficient for git to fire the includeIf rule.
func TestIncludeIfGitdir_ResolvesViaRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found in PATH — skipping includeIf resolution test")
	}

	// --- Hermetic HOME ----------------------------------------------------------
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1") // never read /etc/gitconfig

	// --- Repo dir ---------------------------------------------------------------
	// gitdir: conditions match against the resolved .git directory path.
	// We must run `git init` to create a real repository — a bare directory
	// named .git is not recognised as a git repo and causes git to exit 1.
	repoDir := filepath.Join(home, "git", "work", "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil { //nolint:gosec // G301: test fixture dir; 0755 required so git can traverse and write the .git internals (G301)
		t.Fatalf("creating repo dir: %v", err)
	}
	initCmd := exec.Command("git", "init", repoDir) //nolint:gosec // repoDir is a t.TempDir()-derived test fixture path (G204)
	initCmd.Env = []string{
		"HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1",
		"PATH=" + os.Getenv("PATH"),
	}
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", repoDir, err, out)
	}

	// --- Fragment ---------------------------------------------------------------
	// Use the production fragment layout ~/.gitconfig.d/<name> (mirrors add.go and
	// rotate.go) so the test exercises the exact on-disk shape gitid writes.
	fragDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(fragDir, 0o755); err != nil { //nolint:gosec // G301: test fixture dir only; no sensitive data resides at creation time (G301)
		t.Fatalf("creating fragment dir: %v", err)
	}
	fragPath := filepath.Join(fragDir, "work")

	wantEmail := "work@example.com"
	if err := WriteFragment(fragPath, "Work User", wantEmail, "~/.ssh/id_ed25519_work.pub"); err != nil {
		t.Fatalf("WriteFragment: %v", err)
	}

	// --- Parent ~/.gitconfig with gitid's OWN rendered block --------------------
	// The gitdir value uses ~/git/work/ — the tilde-abbreviated parent dir that
	// covers any repo nested beneath it (git expands ~ in gitdir conditions).
	// We intentionally pass "~/git/work" WITHOUT a trailing slash to prove that
	// condition() normalises it to "~/git/work/" as required by Pitfall 7.
	matches := []Match{{Kind: MatchGitdir, Value: "~/git/work"}}
	rendered := RenderIncludeIf("work", fragPath, matches)

	gitconfigPath := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfigPath, []byte(rendered+"\n"), 0o644); err != nil { //nolint:gosec // G306: gitconfig is not a secret file (0644 is the standard mode, matches gitconfigMode in renderer.go) (G306)
		t.Fatalf("writing ~/.gitconfig: %v", err)
	}

	// Sanity-check: the rendered block must contain the trailing slash so git
	// actually fires the rule (Pitfall 7). This assertion is structural but guards
	// against a regression in condition() before the git call below.
	if !strings.Contains(rendered, `"gitdir:~/git/work/"`) {
		t.Fatalf("RenderIncludeIf output missing trailing slash — Pitfall 7 regression:\n%s", rendered)
	}

	// --- Run `git config user.email` inside the repo ----------------------------
	cmd := exec.Command("git", "config", "user.email") //nolint:gosec // repoDir is a t.TempDir()-derived test fixture path (G204)
	cmd.Dir = repoDir
	cmd.Env = []string{
		"HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1",
		"PATH=" + os.Getenv("PATH"), // keep PATH so git can find helpers
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git config user.email (inside repo): %v\nrendered gitconfig:\n%s", err, rendered)
	}

	got := strings.TrimSpace(string(out))
	if got != wantEmail {
		t.Errorf("includeIf resolution: git config user.email = %q, want %q\nrendered gitconfig:\n%s",
			got, wantEmail, rendered)
	}
}
