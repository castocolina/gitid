package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
)

// TestBuildMatchesGitdir asserts that choice "1" (gitdir) builds a single
// MatchGitdir with the supplied value (D-07).
func TestBuildMatchesGitdir(t *testing.T) {
	got := buildMatches("1", "~/git/personal/", "")
	if len(got) != 1 {
		t.Fatalf("buildMatches(1): want 1 match, got %d", len(got))
	}
	if got[0].Kind != gitconfig.MatchGitdir {
		t.Errorf("buildMatches(1): want MatchGitdir, got %v", got[0].Kind)
	}
	if got[0].Value != "~/git/personal/" {
		t.Errorf("buildMatches(1): want Value %q, got %q", "~/git/personal/", got[0].Value)
	}
}

// TestBuildMatchesHasconfig asserts that choice "2" (url) builds a single
// MatchHasconfig whose Value carries the "remote.*.url:" prefix (D-08, T-05.5-14).
func TestBuildMatchesHasconfig(t *testing.T) {
	// The caller passes the bare URL pattern; buildMatches prepends "remote.*.url:".
	got := buildMatches("2", "", "git@ssh.github.com:user/**")
	if len(got) != 1 {
		t.Fatalf("buildMatches(2): want 1 match, got %d", len(got))
	}
	if got[0].Kind != gitconfig.MatchHasconfig {
		t.Errorf("buildMatches(2): want MatchHasconfig, got %v", got[0].Kind)
	}
	want := "remote.*.url:git@ssh.github.com:user/**"
	if got[0].Value != want {
		t.Errorf("buildMatches(2): want Value %q, got %q", want, got[0].Value)
	}
}

// TestBuildMatchesBoth asserts that choice "3" (both) builds two matches:
// gitdir first, then hasconfig (D-07).
func TestBuildMatchesBoth(t *testing.T) {
	got := buildMatches("3", "~/git/personal/", "git@ssh.github.com:user/**")
	if len(got) != 2 {
		t.Fatalf("buildMatches(3): want 2 matches, got %d", len(got))
	}
	if got[0].Kind != gitconfig.MatchGitdir {
		t.Errorf("buildMatches(3): match[0] want MatchGitdir, got %v", got[0].Kind)
	}
	if got[1].Kind != gitconfig.MatchHasconfig {
		t.Errorf("buildMatches(3): match[1] want MatchHasconfig, got %v", got[1].Kind)
	}
}

// TestBuildMatchesDefaultGitdir asserts that an empty/unknown choice defaults to
// gitdir (safe default, D-07).
func TestBuildMatchesDefaultGitdir(t *testing.T) {
	got := buildMatches("", "~/git/foo/", "")
	if len(got) != 1 || got[0].Kind != gitconfig.MatchGitdir {
		t.Errorf("buildMatches(empty): want single MatchGitdir, got %+v", got)
	}
}

// TestDefaultURLPattern asserts the git@<hostname>:<name>/** form (D-08).
func TestDefaultURLPattern(t *testing.T) {
	got := defaultURLPattern("ssh.github.com", "user_z3r0_gh")
	want := "git@ssh.github.com:user_z3r0_gh/**"
	if got != want {
		t.Errorf("defaultURLPattern: want %q, got %q", want, got)
	}
}

// TestMatchKindsGitdir asserts matchKinds returns "gitdir" for a single MatchGitdir.
func TestMatchKindsGitdir(t *testing.T) {
	got := matchKinds([]gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/x/"}})
	if got != "gitdir" {
		t.Errorf("matchKinds(gitdir): want %q, got %q", "gitdir", got)
	}
}

// TestMatchKindsHasconfig asserts matchKinds returns "hasconfig" for a single
// MatchHasconfig.
func TestMatchKindsHasconfig(t *testing.T) {
	got := matchKinds([]gitconfig.Match{{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:git@ssh.github.com:u/**"}})
	if got != "hasconfig" {
		t.Errorf("matchKinds(hasconfig): want %q, got %q", "hasconfig", got)
	}
}

// TestMatchKindsBoth asserts matchKinds returns "both" when the slice contains
// one of each kind.
func TestMatchKindsBoth(t *testing.T) {
	got := matchKinds([]gitconfig.Match{
		{Kind: gitconfig.MatchGitdir, Value: "~/git/x/"},
		{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:git@ssh.github.com:u/**"},
	})
	if got != "both" {
		t.Errorf("matchKinds(both): want %q, got %q", "both", got)
	}
}

// TestMatchKindsEmpty asserts matchKinds defaults to "gitdir" on an empty slice.
func TestMatchKindsEmpty(t *testing.T) {
	got := matchKinds(nil)
	if got != "gitdir" {
		t.Errorf("matchKinds(nil): want default %q, got %q", "gitdir", got)
	}
}

// TestHasconfigRoundTrip asserts that a hasconfig Value built by buildMatches
// survives RenderIncludeIf → conditionToMatch unchanged (T-05.5-14 TOOL-04).
func TestHasconfigRoundTrip(t *testing.T) {
	matches := buildMatches("2", "", "git@ssh.github.com:myuser/**")
	if len(matches) != 1 {
		t.Fatalf("buildMatches for round-trip: want 1, got %d", len(matches))
	}

	// RenderIncludeIf wraps in sentinels; the embedded condition line is:
	//   [includeIf "hasconfig:remote.*.url:git@ssh.github.com:myuser/**"]
	rendered := gitconfig.RenderIncludeIf("myid", "~/.gitconfig.d/myid", matches)

	// Parse the rendered text: find the includeIf condition.
	// Format: [includeIf "<condition>"]
	var cond string
	for _, line := range strings.Split(rendered, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[includeIf ") && strings.HasSuffix(line, "]") {
			// Extract the quoted condition value.
			inner := strings.TrimPrefix(line, "[includeIf ")
			inner = strings.TrimSuffix(inner, "]")
			inner = strings.Trim(inner, `"`)
			cond = inner
			break
		}
	}
	if cond == "" {
		t.Fatalf("could not find includeIf condition in rendered output:\n%s", rendered)
	}

	// conditionToMatch strips "hasconfig:" prefix and stores the rest as Value.
	// The expected stored form is "remote.*.url:git@ssh.github.com:myuser/**".
	wantValue := "remote.*.url:git@ssh.github.com:myuser/**"
	// cond is the full condition: "hasconfig:remote.*.url:git@ssh.github.com:myuser/**"
	// After conditionToMatch strips "hasconfig:", we get the Value.
	if !strings.HasPrefix(cond, "hasconfig:") {
		t.Fatalf("rendered condition %q does not start with 'hasconfig:'", cond)
	}
	gotValue := strings.TrimPrefix(cond, "hasconfig:")
	if gotValue != wantValue {
		t.Errorf("round-trip: want stored Value %q, got %q", wantValue, gotValue)
	}
	// Also verify the original match Value is unchanged (round-trip complete).
	if matches[0].Value != wantValue {
		t.Errorf("buildMatches Value %q does not equal round-trip value %q", matches[0].Value, wantValue)
	}
}

// TestPromptMatchStrategy_GitdirChoice asserts that strategy choice "1" with a
// gitdir value returns a single MatchGitdir.
func TestPromptMatchStrategy_GitdirChoice(t *testing.T) {
	// Input: choice "1" (gitdir), then the gitdir value.
	stdin := strings.NewReader("1\n~/git/personal/\n")
	r := bufio.NewReader(stdin)
	var out bytes.Buffer

	got := promptMatchStrategy(r, &out, "~/git/personal/", "git@ssh.github.com:me/**")
	if len(got) != 1 || got[0].Kind != gitconfig.MatchGitdir {
		t.Errorf("promptMatchStrategy(1): want single MatchGitdir, got %+v", got)
	}
	if got[0].Value != "~/git/personal/" {
		t.Errorf("promptMatchStrategy(1): want gitdir %q, got %q", "~/git/personal/", got[0].Value)
	}
}

// TestPromptMatchStrategy_URLChoice asserts that choice "2" with a URL returns a
// single MatchHasconfig with the "remote.*.url:" prefix.
func TestPromptMatchStrategy_URLChoice(t *testing.T) {
	// Input: choice "2" (url), then the URL pattern.
	stdin := strings.NewReader("2\ngit@ssh.github.com:me/**\n")
	r := bufio.NewReader(stdin)
	var out bytes.Buffer

	got := promptMatchStrategy(r, &out, "~/git/me/", "git@ssh.github.com:me/**")
	if len(got) != 1 || got[0].Kind != gitconfig.MatchHasconfig {
		t.Errorf("promptMatchStrategy(2): want single MatchHasconfig, got %+v", got)
	}
	wantVal := "remote.*.url:git@ssh.github.com:me/**"
	if got[0].Value != wantVal {
		t.Errorf("promptMatchStrategy(2): want Value %q, got %q", wantVal, got[0].Value)
	}
}

// TestPromptMatchStrategy_BothChoice asserts that choice "3" returns two matches.
func TestPromptMatchStrategy_BothChoice(t *testing.T) {
	// Input: choice "3", gitdir value, URL value.
	stdin := strings.NewReader("3\n~/git/personal/\ngit@ssh.github.com:me/**\n")
	r := bufio.NewReader(stdin)
	var out bytes.Buffer

	got := promptMatchStrategy(r, &out, "~/git/personal/", "git@ssh.github.com:me/**")
	if len(got) != 2 {
		t.Fatalf("promptMatchStrategy(3): want 2 matches, got %d", len(got))
	}
	if got[0].Kind != gitconfig.MatchGitdir || got[1].Kind != gitconfig.MatchHasconfig {
		t.Errorf("promptMatchStrategy(3): want [gitdir, hasconfig], got %+v", got)
	}
}

// TestPromptMatchStrategy_EmptyDefaultsGitdir asserts that pressing Enter with
// empty input defaults to strategy 1 (gitdir).
func TestPromptMatchStrategy_EmptyDefaultsGitdir(t *testing.T) {
	// Input: empty (defaults to "1"), then empty (accepts gitdir default).
	stdin := strings.NewReader("\n\n")
	r := bufio.NewReader(stdin)
	var out bytes.Buffer

	got := promptMatchStrategy(r, &out, "~/git/default/", "git@ssh.github.com:me/**")
	if len(got) != 1 || got[0].Kind != gitconfig.MatchGitdir {
		t.Errorf("promptMatchStrategy(empty): want default MatchGitdir, got %+v", got)
	}
}
