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

func TestIncludeIfStrategies(t *testing.T) {
	tests := []struct {
		name       string
		identity   string
		fragment   string
		matches    []Match
		conditions []string
	}{
		{
			name:     "gitdir default retains terminal slash",
			identity: "work",
			fragment: "~/.gitconfig.d/work",
			matches:  []Match{{Kind: MatchGitdir, Value: "~/git/work/"}},
			conditions: []string{
				`[includeIf "gitdir:~/git/work/"]`,
			},
		},
		{
			name:     "gitdir nested editable path gains terminal slash",
			identity: "work",
			fragment: "~/.gitconfig.d/work",
			matches:  []Match{{Kind: MatchGitdir, Value: "~/src/company/platform/services"}},
			conditions: []string{
				`[includeIf "gitdir:~/src/company/platform/services/"]`,
			},
		},
		{
			name:     "hasconfig uses the custom SSH alias only",
			identity: "work",
			fragment: "~/.gitconfig.d/work",
			matches: []Match{{
				Kind:  MatchHasconfig,
				Value: "remote.*.url:git@engineering.github.example:*/**",
			}},
			conditions: []string{
				`[includeIf "hasconfig:remote.*.url:git@engineering.github.example:*/**"]`,
			},
		},
		{
			name:     "both emits two OR alternatives",
			identity: "work",
			fragment: "~/.gitconfig.d/work",
			matches: []Match{
				{Kind: MatchGitdir, Value: "~/src/company/platform/"},
				{Kind: MatchHasconfig, Value: "remote.*.url:git@engineering.github.example:*/**"},
			},
			conditions: []string{
				`[includeIf "gitdir:~/src/company/platform/"]`,
				`[includeIf "hasconfig:remote.*.url:git@engineering.github.example:*/**"]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderIncludeIf(tt.identity, tt.fragment, tt.matches)
			for _, condition := range tt.conditions {
				if !strings.Contains(got, condition) {
					t.Errorf("RenderIncludeIf() missing %q:\n%s", condition, got)
				}
			}
			if strings.Contains(got, "https://") {
				t.Errorf("RenderIncludeIf() rendered deferred HTTPS hasconfig variant:\n%s", got)
			}
		})
	}
}

func TestRenderParseRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		identity string
		fragment string
		matches  []Match
	}{
		{
			name:     "default gitdir",
			identity: "personal",
			fragment: "~/.gitconfig.d/personal",
			matches:  []Match{{Kind: MatchGitdir, Value: "~/git/personal/"}},
		},
		{
			name:     "custom SSH alias",
			identity: "work",
			fragment: "~/.gitconfig.d/work",
			matches: []Match{{
				Kind:  MatchHasconfig,
				Value: "remote.*.url:git@team.gitlab.example:*/**",
			}},
		},
		{
			name:     "both with nested gitdir",
			identity: "work",
			fragment: "~/.gitconfig.d/work",
			matches: []Match{
				{Kind: MatchGitdir, Value: "~/git/clients/acme/work/"},
				{Kind: MatchHasconfig, Value: "remote.*.url:git@team.github.example:*/**"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := []byte(RenderIncludeIf(tt.identity, tt.fragment, tt.matches))
			got, ok := ParseManagedIncludeIf(content)[tt.identity]
			if !ok {
				t.Fatalf("ParseManagedIncludeIf() missing %q", tt.identity)
			}
			if got.FragmentPath != tt.fragment {
				t.Errorf("FragmentPath = %q, want %q", got.FragmentPath, tt.fragment)
			}
			if len(got.Matches) != len(tt.matches) {
				t.Fatalf("matches length = %d, want %d: %#v", len(got.Matches), len(tt.matches), got.Matches)
			}
			for i, want := range tt.matches {
				if got.Matches[i] != want {
					t.Errorf("matches[%d] = %#v, want %#v", i, got.Matches[i], want)
				}
			}
		})
	}
}

func TestIncludeIfRejectsUnsafeInput(t *testing.T) {
	tests := []struct {
		name     string
		identity string
		fragment string
		matches  []Match
	}{
		{
			name:     "newline in gitdir",
			identity: "work",
			fragment: "~/.gitconfig.d/work",
			matches:  []Match{{Kind: MatchGitdir, Value: "~/git/work/\n[remote \"origin\"]"}},
		},
		{
			name:     "HTTPS hasconfig is deferred",
			identity: "work",
			fragment: "~/.gitconfig.d/work",
			matches:  []Match{{Kind: MatchHasconfig, Value: "remote.*.url:https://github.example/*/**"}},
		},
		{
			name:     "malformed SSH hasconfig",
			identity: "work",
			fragment: "~/.gitconfig.d/work",
			matches:  []Match{{Kind: MatchHasconfig, Value: "remote.*.url:git@github.example:owner/repo"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderIncludeIf(tt.identity, tt.fragment, tt.matches); got != "" {
				t.Errorf("RenderIncludeIf() = %q, want empty rejected preview", got)
			}

			path := filepath.Join(t.TempDir(), ".gitconfig")
			if _, err := WriteIncludeIf(path, tt.identity, tt.fragment, tt.matches); err == nil {
				t.Error("WriteIncludeIf() error = nil, want unsafe input rejection")
			}
		})
	}
}
