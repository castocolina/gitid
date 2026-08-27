package globalgit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
)

func TestVerifyAuthorResolution_EmptyMatchedDirIsNotVerifiable(t *testing.T) {
	deps := Deps{
		RunGitConfig: func(_ context.Context, _ ...string) (string, error) {
			return "file:/tmp/.gitconfig\tfallback@example.com", nil
		},
		NonRepoCwd: t.TempDir(),
	}
	got, err := VerifyAuthorResolution(deps, "", deps.NonRepoCwd)
	if err != nil {
		t.Fatalf("VerifyAuthorResolution: %v", err)
	}
	if got.MatchedOutcome != MatchedNotVerifiable {
		t.Errorf("empty matchedDir: MatchedOutcome = %v, want MatchedNotVerifiable", got.MatchedOutcome)
	}
	if got.MatchedOutcome == MatchedVerified {
		t.Error("not-verifiable must be distinguishable from a passing matched read")
	}
}

func TestVerifyAuthorResolution_NilSeamNamed(t *testing.T) {
	_, err := VerifyAuthorResolution(Deps{}, "", t.TempDir())
	if err == nil {
		t.Fatal("nil RunGitConfig must be rejected by name")
	}
	if !strings.Contains(err.Error(), "RunGitConfig") {
		t.Errorf("error should name the nil seam, got: %v", err)
	}
}

func TestAuthorResolution_ParseShowOriginGet_StripsFilePrefix(t *testing.T) {
	got := parseShowOriginGet("file:/home/alice/.gitconfig\tpat@example.com\n")
	if got.Origin != "/home/alice/.gitconfig" {
		t.Errorf("Origin = %q, want stripped path", got.Origin)
	}
	if got.Value != "pat@example.com" {
		t.Errorf("Value = %q, want pat@example.com", got.Value)
	}
}

func TestVerifyAuthorResolution_RealGitMatchedAndUnmatched(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	repoDir := filepath.Join(home, "git", "work", "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil { //nolint:gosec // test fixture dir
		t.Fatalf("creating repo dir: %v", err)
	}
	initCmd := exec.Command("git", "init", repoDir) //nolint:gosec // t.TempDir fixture
	initCmd.Env = []string{"HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")}
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	fragDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(fragDir, 0o700); err != nil {
		t.Fatalf("creating fragment dir: %v", err)
	}
	fragPath := filepath.Join(fragDir, "work")
	if err := gitconfig.WriteFragment(fragPath, "Work User", "work@example.com", "", false); err != nil {
		t.Fatalf("WriteFragment: %v", err)
	}

	unmatchedDir := filepath.Join(home, "unmatched")
	if err := os.MkdirAll(unmatchedDir, 0o755); err != nil { //nolint:gosec // test fixture dir
		t.Fatalf("creating unmatched dir: %v", err)
	}

	gitconfigPath := filepath.Join(home, ".gitconfig")
	includeBody := "[include]\n\tpath = " + filepath.Join(fragDir, "00-baseline")
	content := []byte(
		filewriter.BeginPrefix + gitconfig.BaselineIncludeBlockName + "\n" + includeBody + "\n" +
			filewriter.EndPrefix + gitconfig.BaselineIncludeBlockName + "\n",
	)
	composed, err := gitconfig.EnsureGitFallbackAuthor(content, "Fallback Name", "fallback@example.com")
	if err != nil {
		t.Fatalf("EnsureGitFallbackAuthor: %v", err)
	}
	matches := []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/work"}}
	includeIf := gitconfig.RenderIncludeIf("work", fragPath, matches)
	composed = append(composed, []byte(includeIf+"\n")...)
	if err := os.WriteFile(gitconfigPath, composed, 0o644); err != nil { //nolint:gosec // gitconfig is not secret
		t.Fatalf("writing ~/.gitconfig: %v", err)
	}

	deps := BuildProbeDeps(unmatchedDir)
	got, err := VerifyAuthorResolution(deps, repoDir, unmatchedDir)
	if err != nil {
		t.Fatalf("VerifyAuthorResolution: %v", err)
	}
	if got.MatchedOutcome != MatchedVerified {
		t.Fatalf("MatchedOutcome = %v, want MatchedVerified", got.MatchedOutcome)
	}

	t.Logf("matched email origin=%q value=%q", got.Matched.Email.Origin, got.Matched.Email.Value)
	t.Logf("unmatched email origin=%q value=%q", got.Unmatched.Email.Origin, got.Unmatched.Email.Value)

	if got.Matched.Email.Value != "work@example.com" {
		t.Errorf("matched email = %q, want work@example.com", got.Matched.Email.Value)
	}
	if !strings.Contains(got.Matched.Email.Origin, fragPath) && !strings.HasSuffix(got.Matched.Email.Origin, "work") {
		t.Errorf("matched origin %q should name the identity fragment %q", got.Matched.Email.Origin, fragPath)
	}
	if got.Unmatched.Email.Value != "fallback@example.com" {
		t.Errorf("unmatched email = %q, want fallback@example.com", got.Unmatched.Email.Value)
	}
	if !strings.Contains(got.Unmatched.Email.Origin, gitconfigPath) && !strings.Contains(got.Unmatched.Email.Origin, ".gitconfig") {
		t.Errorf("unmatched origin %q should name the main config %q", got.Unmatched.Email.Origin, gitconfigPath)
	}
}

func TestVerifyAuthorResolution_MissingIncludeTargetIsSilent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	unmatchedDir := filepath.Join(home, "unmatched")
	if err := os.MkdirAll(unmatchedDir, 0o755); err != nil { //nolint:gosec // test fixture dir
		t.Fatalf("creating unmatched dir: %v", err)
	}

	gitconfigPath := filepath.Join(home, ".gitconfig")
	missingBaseline := filepath.Join(home, ".gitconfig.d", "00-baseline")
	includeBody := "[include]\n\tpath = " + missingBaseline
	content := []byte(
		filewriter.BeginPrefix + gitconfig.BaselineIncludeBlockName + "\n" + includeBody + "\n" +
			filewriter.EndPrefix + gitconfig.BaselineIncludeBlockName + "\n",
	)
	composed, err := gitconfig.EnsureGitFallbackAuthor(content, "Fallback Name", "fallback@example.com")
	if err != nil {
		t.Fatalf("EnsureGitFallbackAuthor: %v", err)
	}
	if err := os.WriteFile(gitconfigPath, composed, 0o644); err != nil { //nolint:gosec // gitconfig is not secret
		t.Fatalf("writing ~/.gitconfig: %v", err)
	}

	cmd := exec.Command("git", "config", "--show-origin", "--get", "user.email") //nolint:gosec // hermetic
	cmd.Dir = unmatchedDir
	cmd.Env = []string{"HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	t.Logf("git config --show-origin --get user.email (missing include target):\nstdout+stderr: %q\nerr: %v", out, err)
	if err != nil {
		t.Fatalf("git must silently ignore a missing include target; got err %v output %q", err, out)
	}
	got := strings.TrimSpace(string(out))
	if !strings.Contains(got, "fallback@example.com") {
		t.Errorf("read should name the fallback email, got %q", got)
	}
	if !strings.Contains(got, gitconfigPath) && !strings.Contains(got, ".gitconfig") {
		t.Errorf("origin should name the main config, got %q", got)
	}
}
