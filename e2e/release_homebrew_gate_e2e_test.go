//go:build e2e

package e2e

// release_homebrew_gate_e2e_test.go — D-18 (10-CONTEXT.md addendum,
// 2026-09-05): a REAL functional proof that the Makefile's --skip=homebrew
// gate genuinely changes goreleaser's behavior when HOMEBREW_TAP_GITHUB_TOKEN
// is absent, run against THIS repo's own .goreleaser.yaml (redirected to a
// throwaway dist directory via a rewritten `dist:` line, so it never
// collides with stampedArtifacts's shared, memoized dist/ output other e2e
// tests in this package depend on) and a real, throwaway, semver-shaped git
// tag created and removed within the test.
//
// REVIEW cycle-1 finding #4 (10-REVIEWS.md) explicitly asked for ONE
// unambiguous, observed (not guessed) claim per run. This file's two
// sub-tests record exactly what was observed on this pinned goreleaser
// binary (Makefile GORELEASER_VERSION):
//   - Run A (--skip=homebrew, alongside announce/validate/publish, which a
//     scratch tag/no real GitHub remote cannot perform anyway): succeeds,
//     and dist/homebrew is never created — the homebrew pipe is not entered
//     at all.
//   - Run B (same flags MINUS homebrew — i.e. homebrew is NOT skipped):
//     ALSO succeeds, and DOES write dist/homebrew/Formula/gitid.rb locally
//     with an empty token. This is the real, observed pinned-goreleaser
//     behavior: the brews: pipe's LOCAL formula-render step does not itself
//     require HOMEBREW_TAP_GITHUB_TOKEN — the token is only consumed by the
//     PUBLISH step that pushes the rendered formula to the tap repo, which
//     is the network leg --skip=publish already removes from both runs
//     here (this sandbox has no real castocolina/homebrew-tap repo or
//     credentials to push to). This means the Makefile's --skip=homebrew
//     gate is a defense-in-depth safeguard — it keeps a token-absent build
//     from even ATTEMPTING the homebrew leg — rather than the only thing
//     standing between a tag push and a hard failure; the true "would this
//     fail for real" question can only be answered by an actual network
//     push to a real tap repo, which is out of reach for a hermetic test
//     and is exactly why D-18 already gates the entire leg off by default.
//     Both facts are asserted below exactly as observed, never guessed.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// realGoreleaserConfigWithScratchDist copies THIS repo's real .goreleaser.yaml
// verbatim except for the `dist: dist` line, redirected to a fresh temp
// directory — so a real release-mode run exercises the actual production
// build/archive/release/brews stanzas without ever touching the shared
// dist/ directory stampedArtifacts (release_e2e_test.go) memoizes for the
// rest of this package's e2e suite.
func realGoreleaserConfigWithScratchDist(t *testing.T) (configPath, distDir string) {
	t.Helper()
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml")) //nolint:gosec
	if err != nil {
		t.Fatalf("reading .goreleaser.yaml: %v", err)
	}
	distDir = t.TempDir()
	rewritten := strings.Replace(string(raw), "dist: dist\n", "dist: "+distDir+"\n", 1)
	if rewritten == string(raw) {
		t.Fatal("realGoreleaserConfigWithScratchDist: literal `dist: dist` line not found to rewrite — .goreleaser.yaml shape changed?")
	}
	tmpConfig := filepath.Join(t.TempDir(), "goreleaser-scratch-dist.yaml")
	if err := os.WriteFile(tmpConfig, []byte(rewritten), 0o644); err != nil { //nolint:gosec
		t.Fatalf("writing scratch goreleaser config: %v", err)
	}
	return tmpConfig, distDir
}

// withScratchReleaseTag creates a real, throwaway, semver-prerelease-shaped
// LIGHTWEIGHT git tag (never `-a`/`-m` — an annotated tag creates a tag
// OBJECT, which requires a configured committer identity; a fresh CI runner
// has none, and this exact class of failure was a real code-review finding
// against this file's first draft) at HEAD on the ACTUAL repo (never pushed
// anywhere) so goreleaser's own `git describe`-based .Version resolution has
// something real to parse. Defensively deletes any leftover tag of the same
// exact name FIRST (a prior killed test run could have left one behind —
// another real code-review finding), then guarantees removal via
// t.Cleanup regardless of this run's own outcome.
func withScratchReleaseTag(t *testing.T, name string) {
	t.Helper()
	root := repoRoot(t)
	preclean := exec.Command("git", "tag", "-d", name)
	preclean.Dir = root
	_ = preclean.Run() // best-effort; fails harmlessly if no leftover tag exists

	cmd := exec.Command("git", "tag", name)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git tag %s: %v\n%s", name, err, out)
	}
	t.Cleanup(func() {
		cleanupCmd := exec.Command("git", "tag", "-d", name)
		cleanupCmd.Dir = root
		_ = cleanupCmd.Run()
	})
}

// goreleaserBinPath resolves the pinned goreleaser binary the SAME way the
// Makefile does ($(go env GOPATH)/bin/goreleaser, see Makefile's GOPATH_BIN/
// GORELEASER vars) rather than trusting it to be on the test process's PATH.
func goreleaserBinPath(t *testing.T) string {
	t.Helper()
	if p, err := exec.LookPath("goreleaser"); err == nil {
		return p
	}
	out, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		t.Fatalf("goreleaserBinPath: go env GOPATH: %v", err)
	}
	return filepath.Join(strings.TrimSpace(string(out)), "bin", "goreleaser")
}

func runScratchGoreleaserRelease(t *testing.T, configPath string, skip string) (stdout, stderr string, err error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second*ciTimeoutMultiplier())
	defer cancel()
	cmd := exec.CommandContext(ctx, goreleaserBinPath(t), "--config", configPath, "release", "--clean", "--skip="+skip)
	cmd.Dir = repoRoot(t)
	// Deliberately UNSET HOMEBREW_TAP_GITHUB_TOKEN (D-18's exact scenario:
	// the tap repo/PAT do not exist yet) — filter it out of the inherited
	// environment rather than trusting the ambient shell not to have it set.
	// VERSION/COMMIT/DATE are exported the SAME way the Makefile's release
	// target does (D-05: the ldflags template reads {{.Env.VERSION}} etc,
	// never goreleaser's own internal git-describe value).
	env := make([]string, 0, len(os.Environ())+3)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "HOMEBREW_TAP_GITHUB_TOKEN=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "VERSION="+e2eStampVersion, "COMMIT="+e2eStampCommit, "DATE="+e2eStampDate)
	cmd.Env = env
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	return out.String(), errOut.String(), err
}

func TestReleaseHomebrewGate_SkippedNeverEntersHomebrewPipe(t *testing.T) {
	configPath, distDir := realGoreleaserConfigWithScratchDist(t)
	withScratchReleaseTag(t, "v0.0.0-e2e-homebrew-gate-skip")

	stdout, stderr, err := runScratchGoreleaserRelease(t, configPath, "announce,validate,publish,homebrew")
	if err != nil {
		t.Fatalf("goreleaser release --skip=announce,validate,publish,homebrew (no HOMEBREW_TAP_GITHUB_TOKEN) failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if _, statErr := os.Stat(filepath.Join(distDir, "homebrew")); statErr == nil {
		t.Fatal("dist/homebrew was created even though homebrew was skipped — the gate did not actually skip the pipe")
	}
}

// TestReleaseHomebrewGate_NotSkippedStillSucceedsLocally records the
// SECOND observed fact from the same real binary: with homebrew NOT
// skipped (but publish still skipped, since this sandbox cannot push to a
// real tap repo), the pinned goreleaser binary (Makefile GORELEASER_VERSION)
// renders the formula file locally with no error even though
// HOMEBREW_TAP_GITHUB_TOKEN is absent — the token is
// only consumed by the publish-time push, not the local render. See this
// file's header comment for why this does not weaken D-18's gate: it is
// what makes --skip=homebrew a defense-in-depth choice rather than the only
// thing preventing a hard failure, and D-18 keeps it in place regardless.
func TestReleaseHomebrewGate_NotSkippedStillSucceedsLocally(t *testing.T) {
	configPath, distDir := realGoreleaserConfigWithScratchDist(t)
	withScratchReleaseTag(t, "v0.0.0-e2e-homebrew-gate-noskip")

	stdout, stderr, err := runScratchGoreleaserRelease(t, configPath, "announce,validate,publish")
	if err != nil {
		t.Fatalf("goreleaser release --skip=announce,validate,publish (homebrew NOT skipped, no HOMEBREW_TAP_GITHUB_TOKEN) failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	formula := filepath.Join(distDir, "homebrew", "Formula", "gitid.rb")
	if _, statErr := os.Stat(formula); statErr != nil {
		t.Fatalf("expected %s to exist (the pinned goreleaser binary's [Makefile GORELEASER_VERSION] local formula-render step runs even with an empty token) — if this now fails, the empirical finding this test records has changed and 10-CONTEXT.md D-18 should be re-checked: %v", formula, statErr)
	}
}
