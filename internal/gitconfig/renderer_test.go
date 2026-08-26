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

func TestProviderRewrite(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")
	foreign := "[core]\n\texcludesfile = ~/.gitignore_global\n[url \"ssh://git@legacy.example/\"]\n\tinsteadOf = https://legacy.example/\n"
	if err := os.WriteFile(gitconfigPath, []byte(foreign), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture; writer sets production gitconfig mode
		t.Fatalf("seeding gitconfig: %v", err)
	}

	firstBackup, err := WriteProviderRewrite(gitconfigPath, "github.com", true)
	if err != nil {
		t.Fatalf("first WriteProviderRewrite: %v", err)
	}
	if firstBackup == "" {
		t.Error("first enabled write backup = empty, want existing-file backup")
	}
	afterFirst, err := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture
	if err != nil {
		t.Fatalf("reading first write: %v", err)
	}

	const wantBlock = "# BEGIN gitid managed: provider-rewrite:github.com\n[url \"git@github.com:\"]\n\tinsteadOf = https://github.com/\n# END gitid managed: provider-rewrite:github.com\n"
	if !strings.Contains(string(afterFirst), wantBlock) {
		t.Errorf("provider rewrite block missing or malformed:\n%s", afterFirst)
	}
	if !strings.Contains(string(afterFirst), foreign) {
		t.Errorf("foreign Git content changed:\n%s", afterFirst)
	}

	if _, err := WriteProviderRewrite(gitconfigPath, "github.com", true); err != nil {
		t.Fatalf("second WriteProviderRewrite: %v", err)
	}
	afterSecond, err := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture
	if err != nil {
		t.Fatalf("reading second write: %v", err)
	}
	if string(afterSecond) != string(afterFirst) {
		t.Errorf("enabled provider rewrite write is not byte-identical:\nfirst:\n%s\nsecond:\n%s", afterFirst, afterSecond)
	}

	if _, err := WriteProviderRewrite(gitconfigPath, "github.com", false); err != nil {
		t.Fatalf("disabled WriteProviderRewrite: %v", err)
	}
	afterDisabled, err := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture
	if err != nil {
		t.Fatalf("reading disabled write: %v", err)
	}
	if string(afterDisabled) != string(afterFirst) {
		t.Errorf("disabled provider rewrite request changed existing block:\nbefore:\n%s\nafter:\n%s", afterFirst, afterDisabled)
	}
}

func TestProviderRewriteSharesProviderBlock(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")

	for _, identity := range []string{"personal", "work"} {
		if _, err := WriteProviderRewrite(gitconfigPath, "github.com", true); err != nil {
			t.Fatalf("WriteProviderRewrite for %s: %v", identity, err)
		}
	}
	got, err := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture
	if err != nil {
		t.Fatalf("reading provider rewrite: %v", err)
	}
	if count := strings.Count(string(got), "# BEGIN gitid managed: provider-rewrite:github.com"); count != 1 {
		t.Errorf("provider rewrite block count = %d, want 1:\n%s", count, got)
	}
	if strings.Contains(string(got), "provider-rewrite:personal") || strings.Contains(string(got), "provider-rewrite:work") {
		t.Errorf("provider rewrite must not be identity-keyed:\n%s", got)
	}
}

func TestProviderRewriteRejectsUnsafeInput(t *testing.T) {
	for _, provider := range []string{
		"github.com\n[core]",
		"github.com/evil",
		"git@github.com",
		"https://github.com",
		"github..com",
	} {
		t.Run(provider, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".gitconfig")
			if _, err := WriteProviderRewrite(path, provider, true); err == nil {
				t.Errorf("WriteProviderRewrite(%q) error = nil, want rejection", provider)
			}
		})
	}
}

// TestRemoveProviderRewrite proves RemoveProviderRewrite removes exactly the
// named provider's rewrite block, leaving a sibling provider's rewrite AND
// the global baseline url-rewrites block untouched, and that a second
// removal is idempotent (byte-identical to the first result).
func TestRemoveProviderRewrite(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")

	if _, err := WriteProviderRewrite(gitconfigPath, "github.com", true); err != nil {
		t.Fatalf("seeding github.com rewrite: %v", err)
	}
	if _, err := WriteProviderRewrite(gitconfigPath, "gitlab.com", true); err != nil {
		t.Fatalf("seeding gitlab.com rewrite: %v", err)
	}
	// A global baseline url-rewrites block, distinct from the per-provider
	// blocks this function must never touch (05-RESEARCH.md Pitfall 4).
	existing, err := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture
	if err != nil {
		t.Fatalf("reading gitconfig: %v", err)
	}
	baselineBlock := "# BEGIN gitid managed: url-rewrites\n[url \"ssh://git@baseline.example/\"]\n\tinsteadOf = https://baseline.example/\n# END gitid managed: url-rewrites\n"
	if err := os.WriteFile(gitconfigPath, append(existing, []byte(baselineBlock)...), 0o644); err != nil { //nolint:gosec // hermetic t.TempDir() fixture; writer sets production gitconfig mode
		t.Fatalf("seeding baseline url-rewrites block: %v", err)
	}

	backup, err := RemoveProviderRewrite(gitconfigPath, "github.com")
	if err != nil {
		t.Fatalf("RemoveProviderRewrite: %v", err)
	}
	if backup == "" {
		t.Error("expected a non-empty backup path for an existing file")
	}

	gc, err := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture
	if err != nil {
		t.Fatalf("reading gitconfig after removal: %v", err)
	}

	githubHas, err := HasProviderRewrite(gc, "github.com")
	if err != nil {
		t.Fatalf("HasProviderRewrite github.com: %v", err)
	}
	if githubHas {
		t.Error("github.com rewrite still present after removal")
	}
	gitlabHas, err := HasProviderRewrite(gc, "gitlab.com")
	if err != nil {
		t.Fatalf("HasProviderRewrite gitlab.com: %v", err)
	}
	if !gitlabHas {
		t.Error("gitlab.com rewrite must survive github.com's removal")
	}
	if !strings.Contains(string(gc), "# BEGIN gitid managed: url-rewrites") {
		t.Errorf("global baseline url-rewrites block must survive a per-provider removal:\n%s", gc)
	}

	// Idempotent: a second removal leaves the file byte-identical AND mints
	// NO spurious backup (WR-12) — the block is already absent, so
	// filewriter.Write must not run at all.
	backupsBefore := countBackupFiles(t, dir)
	backup2, err := RemoveProviderRewrite(gitconfigPath, "github.com")
	if err != nil {
		t.Fatalf("second RemoveProviderRewrite: %v", err)
	}
	if backup2 != "" {
		t.Errorf("WR-12: second (no-op) RemoveProviderRewrite returned a backup path %q, want empty — nothing changed, so nothing should be written", backup2)
	}
	gc2, err := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture
	if err != nil {
		t.Fatalf("reading gitconfig after second removal: %v", err)
	}
	if string(gc) != string(gc2) {
		t.Errorf("RemoveProviderRewrite not idempotent:\nfirst:\n%s\nsecond:\n%s", gc, gc2)
	}
	if got := countBackupFiles(t, dir); got != backupsBefore {
		t.Errorf("WR-12: second (no-op) RemoveProviderRewrite minted %d new backup file(s) on disk, want 0", got-backupsBefore)
	}
}

// countBackupFiles counts timestamped .bak.<nanos>-style backup files under
// dir — WR-12's proof that a no-op removal writes NOTHING to disk, not just
// that its returned backup path is empty.
func countBackupFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir %s: %v", dir, err)
	}
	count := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak.") {
			count++
		}
	}
	return count
}

// TestRemoveProviderRewrite_InvalidHostname proves an invalid provider
// hostname is rejected before any read or write.
func TestRemoveProviderRewrite_InvalidHostname(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")
	seed := []byte("[core]\n\texcludesfile = ~/.gitignore_global\n")
	if err := os.WriteFile(gitconfigPath, seed, 0o644); err != nil { //nolint:gosec // hermetic t.TempDir() fixture; writer sets production gitconfig mode
		t.Fatalf("seeding gitconfig: %v", err)
	}

	if _, err := RemoveProviderRewrite(gitconfigPath, "not a host!"); err == nil {
		t.Fatal("RemoveProviderRewrite with an invalid provider hostname = nil error, want rejection")
	}
	got, err := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture
	if err != nil {
		t.Fatalf("reading gitconfig: %v", err)
	}
	if string(got) != string(seed) {
		t.Errorf("file modified despite invalid hostname rejection:\nbefore:\n%s\nafter:\n%s", seed, got)
	}
}

// TestRemoveProviderRewrite_NoSuchBlockIsNilError proves removing a provider
// rewrite from a file with no such block returns a nil error, and — WR-12 —
// writes NOTHING: no backup path, no rewritten file (byte-identical, same
// mtime-independent content), and no spurious .bak.<nanos> file minted on
// disk. The prior unconditional filewriter.Write call rewrote the file
// byte-for-byte and reported a real-looking backup of a delete transaction
// that touched nothing.
func TestRemoveProviderRewrite_NoSuchBlockIsNilError(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")
	seed := []byte("[core]\n\texcludesfile = ~/.gitignore_global\n")
	if err := os.WriteFile(gitconfigPath, seed, 0o644); err != nil { //nolint:gosec // hermetic t.TempDir() fixture; writer sets production gitconfig mode
		t.Fatalf("seeding gitconfig: %v", err)
	}

	backup, err := RemoveProviderRewrite(gitconfigPath, "github.com")
	if err != nil {
		t.Errorf("RemoveProviderRewrite for an absent block returned an error: %v", err)
	}
	if backup != "" {
		t.Errorf("WR-12: RemoveProviderRewrite for an absent block returned backup path %q, want empty — nothing to remove means no write", backup)
	}
	got, rerr := os.ReadFile(gitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture
	if rerr != nil {
		t.Fatalf("reading gitconfig: %v", rerr)
	}
	if string(got) != string(seed) {
		t.Errorf("WR-12: file rewritten despite having no matching block:\nbefore:\n%s\nafter:\n%s", seed, got)
	}
	if n := countBackupFiles(t, dir); n != 0 {
		t.Errorf("WR-12: a no-op removal minted %d backup file(s) on disk, want 0", n)
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
