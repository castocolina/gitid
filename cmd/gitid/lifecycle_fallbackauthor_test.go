package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/globalgit"
)

func TestGitFallbackAuthorLifecycleStagesRow(t *testing.T) {
	stages, ok := lifecycleStages["global-git-author"]
	if !ok {
		t.Fatal("lifecycleStages must contain a 'global-git-author' row")
	}
	pos := map[string]int{}
	for i, s := range stages {
		pos[s] = i
	}
	for _, req := range []string{"plan", "confirm", "backup", "write"} {
		if _, ok := pos[req]; !ok {
			t.Errorf("global-git-author stages missing %q: %v", req, stages)
		}
	}
	if pos["confirm"] >= pos["backup"] || pos["backup"] >= pos["write"] {
		t.Errorf("global-git-author stages must order confirm -> backup -> write, got: %v", stages)
	}
}

func TestRunGitFallbackAuthorApply_DistinctFromGlobalGitApply(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(testRepoRoot(t), "cmd", "gitid", "lifecycle.go")) //nolint:gosec // repository source
	if err != nil {
		t.Fatalf("reading lifecycle.go: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "func (b *realBackend) runGitFallbackAuthorApply") {
		t.Fatal("runGitFallbackAuthorApply must be a distinct function")
	}
	if !strings.Contains(body, "func (b *realBackend) runGlobalGitApply") {
		t.Fatal("runGlobalGitApply must remain a distinct function")
	}
	authorFn := extractFuncBody(t, body, "func (b *realBackend) runGitFallbackAuthorApply")
	gitFn := extractFuncBody(t, body, "func (b *realBackend) runGlobalGitApply")
	if strings.Contains(authorFn, "runGlobalGitApply") {
		t.Error("runGitFallbackAuthorApply must not call runGlobalGitApply")
	}
	if strings.Contains(gitFn, "runGitFallbackAuthorApply") {
		t.Error("runGlobalGitApply must not call runGitFallbackAuthorApply")
	}
}

func extractFuncBody(t *testing.T, src, signature string) string {
	t.Helper()
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("signature %q not found", signature)
	}
	rest := src[start:]
	next := strings.Index(rest[len(signature):], "\nfunc (")
	if next < 0 {
		return rest
	}
	return rest[:len(signature)+next]
}

func TestRunGitFallbackAuthorApply_BothHalves(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	seedIncludeIf(t, home)
	b := newBackendForHome(home)

	res, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGitFallbackAuthorApply: %v", err)
	}
	if res.Kind != "write" {
		t.Errorf("Kind = %q, want write", res.Kind)
	}
	if len(res.Backups) != 1 {
		t.Errorf("want exactly one backup path, got %v", res.Backups)
	}

	gc := mustRead(t, filepath.Join(home, ".gitconfig"))
	includeEnd := filewriter.EndPrefix + gitconfig.BaselineIncludeBlockName
	includeEndOff := strings.Index(gc, includeEnd)
	fallbackOff := strings.Index(gc, filewriter.BeginPrefix+gitconfig.GitFallbackAuthorBlockName)
	includeIfOff := strings.Index(gc, `[includeIf`)
	if includeEndOff < 0 || fallbackOff < 0 || includeIfOff < 0 {
		t.Fatalf("placement markers missing:\n%s", gc)
	}
	if fallbackOff <= includeEndOff || fallbackOff >= includeIfOff {
		t.Errorf("fallback block not between include and includeIf: includeEnd=%d fallback=%d includeIf=%d",
			includeEndOff, fallbackOff, includeIfOff)
	}
	body := fallbackBody(gc)
	if !strings.Contains(body, "name = Pat Example") || !strings.Contains(body, "email = pat@example.com") {
		t.Errorf("both keys missing from block body:\n%s", body)
	}
}

func TestRunGitFallbackAuthorApply_FirstRunCreatesAnchor(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	b := newBackendForHome(home)

	res, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("first-run apply: %v", err)
	}
	if res.Kind != "write" {
		t.Errorf("Kind = %q, want write", res.Kind)
	}

	gc := mustRead(t, filepath.Join(home, ".gitconfig"))
	t.Logf("first-run ~/.gitconfig:\n%s", gc)
	blocks := filewriter.ListBlocks([]byte(gc))
	if len(blocks) < 2 {
		t.Fatalf("want at least two managed blocks, got %d:\n%s", len(blocks), gc)
	}
	if blocks[0].Name != gitconfig.BaselineIncludeBlockName {
		t.Errorf("first managed block = %q, want %q", blocks[0].Name, gitconfig.BaselineIncludeBlockName)
	}
	if blocks[1].Name != gitconfig.GitFallbackAuthorBlockName {
		t.Errorf("second managed block = %q, want %q", blocks[1].Name, gitconfig.GitFallbackAuthorBlockName)
	}
}

func TestRunGitFallbackAuthorApply_ExistingIncludeNotDuplicated(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	gitconfigPath := filepath.Join(home, ".gitconfig")
	include := filewriter.BeginPrefix + gitconfig.BaselineIncludeBlockName + "\n" +
		"[include]\n\tpath = ~/.gitconfig.d/00-baseline\n" +
		filewriter.EndPrefix + gitconfig.BaselineIncludeBlockName + "\n"
	if err := os.WriteFile(gitconfigPath, []byte(include), 0o644); err != nil { //nolint:gosec // test fixture
		t.Fatalf("seeding include: %v", err)
	}
	origOff := strings.Index(include, filewriter.BeginPrefix+gitconfig.BaselineIncludeBlockName)
	b := newBackendForHome(home)

	if _, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	gc := mustRead(t, gitconfigPath)
	begin := filewriter.BeginPrefix + gitconfig.BaselineIncludeBlockName
	if n := strings.Count(gc, begin); n != 1 {
		t.Errorf("want exactly one floor include block, got %d:\n%s", n, gc)
	}
	if got := strings.Index(gc, begin); got != origOff {
		t.Errorf("floor include moved: offset %d -> %d", origOff, got)
	}
}

func TestRunGitFallbackAuthorApply_ExactlyOneWrite(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("# preamble\n"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("seeding: %v", err)
	}
	b := newBackendForHome(home)
	res, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Backups) != 1 {
		t.Errorf("exactly one backup path expected, got %v", res.Backups)
	}
}

func TestRunGitFallbackAuthorApply_NameOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	b := newBackendForHome(home)
	if _, err := b.runGitFallbackAuthorApply("Pat Example", "", lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body := fallbackBody(mustRead(t, filepath.Join(home, ".gitconfig")))
	if strings.Contains(body, "email") {
		t.Errorf("name-only body must omit email:\n%s", body)
	}
	if !strings.Contains(body, "name = Pat Example") {
		t.Errorf("name-only body missing name:\n%s", body)
	}
}

func TestRunGitFallbackAuthorApply_SamePairTwice(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	b := newBackendForHome(home)
	p := lifecyclePolicy{Confirm: confirmationAlreadyObtained}
	if _, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", p); err != nil {
		t.Fatalf("first: %v", err)
	}
	before := mustRead(t, filepath.Join(home, ".gitconfig"))
	res2, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", p)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	after := mustRead(t, filepath.Join(home, ".gitconfig"))
	if before != after {
		t.Errorf("second apply changed bytes:\n--- first ---\n%s\n--- second ---\n%s", before, after)
	}
	if len(res2.Backups) == 0 {
		t.Error("authorized second apply must still return a backup")
	}
}

func TestRunGitFallbackAuthorApply_EmptyPairRemoves(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	b := newBackendForHome(home)
	p := lifecyclePolicy{Confirm: confirmationAlreadyObtained}
	if _, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", p); err != nil {
		t.Fatalf("seed: %v", err)
	}
	res, err := b.runGitFallbackAuthorApply("", "", p)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if res.Kind != "remove" {
		t.Errorf("Kind = %q, want remove", res.Kind)
	}
	gc := mustRead(t, filepath.Join(home, ".gitconfig"))
	if strings.Contains(gc, gitconfig.GitFallbackAuthorBlockName) {
		t.Errorf("block still present after empty-pair apply:\n%s", gc)
	}
}

func TestRunGitFallbackAuthorApply_EmptyPairNoop(t *testing.T) {
	home := t.TempDir()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	before := snapshotPaths(t, []string{gitconfigPath})
	b := newBackendForHome(home)
	res, err := b.runGitFallbackAuthorApply("", "", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("noop: %v", err)
	}
	if res.Kind != "noop" {
		t.Errorf("Kind = %q, want noop", res.Kind)
	}
	if len(res.Backups) != 0 {
		t.Errorf("noop must take no backup, got %v", res.Backups)
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath}))
}

func TestRunGitFallbackAuthorApply_MalformedEmailRejected(t *testing.T) {
	home := t.TempDir()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	before := snapshotPaths(t, []string{gitconfigPath})
	b := newBackendForHome(home)
	_, err := b.runGitFallbackAuthorApply("Pat", "not-an-email", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err == nil {
		t.Fatal("malformed email must be rejected at plan stage")
	}
	if !strings.Contains(err.Error(), "not-an-email") {
		t.Errorf("error must name the email, got: %v", err)
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath}))
}

func TestRunGitFallbackAuthorApply_DryRun(t *testing.T) {
	home := t.TempDir()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	before := snapshotPaths(t, []string{gitconfigPath})
	confirmCalled := false
	b := newBackendForHome(home)
	_, err := b.runGitFallbackAuthorApply("Pat", "pat@example.com", lifecyclePolicy{
		DryRun: true,
		Prompt: func(_ string) (bool, error) {
			confirmCalled = true
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if confirmCalled {
		t.Error("dry run must not call the confirmation prompt")
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath}))
}

func TestRunGitFallbackAuthorApply_DeclinedConfirmation(t *testing.T) {
	home := t.TempDir()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	before := snapshotPaths(t, []string{gitconfigPath})
	b := newBackendForHome(home)
	_, err := b.runGitFallbackAuthorApply("Pat", "pat@example.com", lifecyclePolicy{
		Confirm: confirmationRequired,
		Prompt:  func(_ string) (bool, error) { return false, nil },
	})
	if err == nil {
		t.Fatal("declined confirmation must return an error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cancel") {
		t.Errorf("error must name cancellation, got: %v", err)
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath}))
}

func TestRunGitFallbackAuthorApply_InjectedFailureRestores(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	gitconfigPath := filepath.Join(home, ".gitconfig")
	preamble := "# user-written\n[core]\n\teditor = vim\n"
	if err := os.WriteFile(gitconfigPath, []byte(preamble), 0o644); err != nil { //nolint:gosec
		t.Fatalf("seeding: %v", err)
	}
	before := snapshotPaths(t, []string{gitconfigPath})
	b := newBackendForHome(home)
	b.failCommitAt = func(s string) error {
		if s == "global-git-author-after-write" {
			return fmt.Errorf("injected failure after fallback write")
		}
		return nil
	}
	res, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err == nil {
		t.Fatal("injected failure must surface")
	}
	if !strings.Contains(err.Error(), "injected") {
		t.Errorf("err = %q, want the injected failure", err.Error())
	}
	t.Logf("restore outcomes: %v", res.Restored)
	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath}))
}

func TestRunGitFallbackAuthorApply_VerifyNoPrecedenceAdvisory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	seedIncludeIf(t, home)
	b := newBackendForHome(home)
	res, err := b.runGitFallbackAuthorApply("Fallback Name", "fallback@example.com", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	for _, a := range res.Advisories {
		if strings.Contains(a, "precedence") {
			t.Errorf("unexpected precedence advisory: %s", a)
		}
	}
}

func TestRunGitFallbackAuthorApply_UnmatchedAdvisoryStillSucceeded(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	b := newBackendForHome(home)
	b.verifyAuthorResolution = func(_ globalgit.Deps, _, _ string) (globalgit.AuthorResolution, error) {
		return globalgit.AuthorResolution{
			MatchedOutcome: globalgit.MatchedNotVerifiable,
			Unmatched: globalgit.DirectoryResolution{
				Email: globalgit.AuthorKeyResolution{Value: "other@example.com", Origin: "/elsewhere"},
			},
		}, nil
	}
	res, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("apply must still succeed: %v", err)
	}
	found := false
	for _, a := range res.Advisories {
		if strings.Contains(a, "/elsewhere") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected advisory naming the unexpected origin, got %v", res.Advisories)
	}
	gc := mustRead(t, filepath.Join(home, ".gitconfig"))
	if !strings.Contains(gc, gitconfig.GitFallbackAuthorBlockName) {
		t.Error("write must have succeeded despite the advisory")
	}
}

func seedIncludeIf(t *testing.T, home string) {
	t.Helper()
	repoDir := filepath.Join(home, "git", "work")
	if err := os.MkdirAll(repoDir, 0o755); err != nil { //nolint:gosec // test fixture
		t.Fatalf("repo dir: %v", err)
	}
	initCmd := exec.Command("git", "init", filepath.Join(repoDir, "repo")) //nolint:gosec
	initCmd.Env = []string{"HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")}
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	fragDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(fragDir, 0o700); err != nil {
		t.Fatalf("frag dir: %v", err)
	}
	fragPath := filepath.Join(fragDir, "work")
	if err := gitconfig.WriteFragment(fragPath, "Work User", "work@example.com", "", false); err != nil {
		t.Fatalf("WriteFragment: %v", err)
	}
	matches := []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/work"}}
	rendered := gitconfig.RenderIncludeIf("work", fragPath, matches)
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(rendered+"\n"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("writing gitconfig: %v", err)
	}
}

func fallbackBody(gc string) string {
	for _, b := range filewriter.ListBlocks([]byte(gc)) {
		if b.Name == gitconfig.GitFallbackAuthorBlockName {
			return b.Body
		}
	}
	return ""
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // hermetic fixture
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
