package gitconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderIncludeIf_GitdirTrailingSlash(t *testing.T) {
	matches := []Match{{Kind: MatchGitdir, Value: "~/git/work/"}}
	block := RenderIncludeIf("work", "~/.gitconfig.d/work", matches)

	if !strings.Contains(block, "# BEGIN gitid managed: work") {
		t.Errorf("missing BEGIN sentinel:\n%s", block)
	}
	if !strings.Contains(block, "# END gitid managed: work") {
		t.Errorf("missing END sentinel:\n%s", block)
	}
	if !strings.Contains(block, `[includeIf "gitdir:~/git/work/"]`) {
		t.Errorf("missing gitdir includeIf header with trailing slash:\n%s", block)
	}
	if !strings.Contains(block, "path = ~/.gitconfig.d/work") {
		t.Errorf("missing fragment path line:\n%s", block)
	}
}

func TestRenderIncludeIf_GitdirAddsTrailingSlash(t *testing.T) {
	// Pitfall 7: trailing slash is mandatory; a value without one must be normalized.
	matches := []Match{{Kind: MatchGitdir, Value: "~/git/work"}}
	block := RenderIncludeIf("work", "~/.gitconfig.d/work", matches)

	if !strings.Contains(block, `[includeIf "gitdir:~/git/work/"]`) {
		t.Errorf("gitdir value should be normalized to a trailing slash:\n%s", block)
	}
}

func TestRenderIncludeIf_HasconfigForm(t *testing.T) {
	matches := []Match{{Kind: MatchHasconfig, Value: "remote.*.url:git@github.companyname.com:*/**"}}
	block := RenderIncludeIf("companyname", "~/.gitconfig.d/companyname", matches)

	if !strings.Contains(block, `[includeIf "hasconfig:remote.*.url:git@github.companyname.com:*/**"]`) {
		t.Errorf("missing hasconfig includeIf header:\n%s", block)
	}
}

func TestRenderIncludeIf_CombinedMatches(t *testing.T) {
	// GIT-02: both kinds combinable in one managed block.
	matches := []Match{
		{Kind: MatchGitdir, Value: "~/git/work/"},
		{Kind: MatchHasconfig, Value: "remote.*.url:git@gitlab.companyname.com:*/**"},
	}
	block := RenderIncludeIf("work", "~/.gitconfig.d/work", matches)

	if !strings.Contains(block, `[includeIf "gitdir:~/git/work/"]`) {
		t.Errorf("missing gitdir header in combined block:\n%s", block)
	}
	if !strings.Contains(block, `[includeIf "hasconfig:remote.*.url:git@gitlab.companyname.com:*/**"]`) {
		t.Errorf("missing hasconfig header in combined block:\n%s", block)
	}
}

func TestWriteIncludeIf_IdempotentAndPreservesForeign(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")
	foreign := "[core]\n\texcludesfile = ~/.gitignore_global\n"
	if err := os.WriteFile(gitconfigPath, []byte(foreign), 0o644); err != nil { //nolint:gosec // 0644 matches the gitconfig contract; gitconfigPath is a test fixture
		t.Fatalf("seeding gitconfig: %v", err)
	}

	matches := []Match{{Kind: MatchGitdir, Value: "~/git/work/"}}

	if _, err := WriteIncludeIf(gitconfigPath, "work", "~/.gitconfig.d/work", matches); err != nil {
		t.Fatalf("first WriteIncludeIf: %v", err)
	}
	after1, err := os.ReadFile(gitconfigPath) //nolint:gosec // test reads back the fixture it just wrote
	if err != nil {
		t.Fatalf("reading after first write: %v", err)
	}
	if !strings.Contains(string(after1), foreign) {
		t.Errorf("foreign content not preserved:\n%s", after1)
	}
	if !strings.Contains(string(after1), `[includeIf "gitdir:~/git/work/"]`) {
		t.Errorf("managed block not written:\n%s", after1)
	}

	if _, err := WriteIncludeIf(gitconfigPath, "work", "~/.gitconfig.d/work", matches); err != nil {
		t.Fatalf("second WriteIncludeIf: %v", err)
	}
	after2, err := os.ReadFile(gitconfigPath) //nolint:gosec // test reads back the fixture it just wrote
	if err != nil {
		t.Fatalf("reading after second write: %v", err)
	}
	if string(after1) != string(after2) {
		t.Errorf("WriteIncludeIf not idempotent:\nfirst:\n%s\nsecond:\n%s", after1, after2)
	}
}

func TestRenderIncludeIf_RejectsInjection(t *testing.T) {
	// A match value containing a newline could break out of the managed block.
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected RenderIncludeIf to reject a newline-bearing match value")
		}
	}()
	matches := []Match{{Kind: MatchGitdir, Value: "~/git/work/\n[remote \"origin\"]"}}
	_ = RenderIncludeIf("work", "~/.gitconfig.d/work", matches)
}
