package checks

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// makeGitdirAccount builds a minimal Account with a single MatchGitdir match.
func makeGitdirAccount(name, gitdir string) identity.Account {
	return identity.Account{
		Name:    name,
		Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: gitdir}},
	}
}

// makeHasconfigAccount builds a minimal Account with a single MatchHasconfig match.
func makeHasconfigAccount(name, urlPattern string) identity.Account {
	return identity.Account{
		Name:    name,
		Matches: []gitconfig.Match{{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:" + urlPattern}},
	}
}

// TestDetectOverlaps_IdenticalGitdir asserts that two accounts sharing the same
// gitdir (after trailing-slash normalisation) produce one OverlapPair with
// Kind "identical-gitdir".
func TestDetectOverlaps_IdenticalGitdir(t *testing.T) {
	a := makeGitdirAccount("work", "~/git/work/")
	b := makeGitdirAccount("work2", "~/git/work") // no trailing slash — normalised
	pairs := DetectOverlaps([]identity.Account{a, b})
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d: %+v", len(pairs), pairs)
	}
	if pairs[0].Kind != "identical-gitdir" {
		t.Errorf("expected Kind=identical-gitdir, got %q", pairs[0].Kind)
	}
}

// TestDetectOverlaps_NestedGitdir asserts that a parent and child gitdir produce
// one OverlapPair with Kind "nested-gitdir".
func TestDetectOverlaps_NestedGitdir(t *testing.T) {
	parent := makeGitdirAccount("root", "~/git/")
	child := makeGitdirAccount("personal", "~/git/personal/")
	pairs := DetectOverlaps([]identity.Account{parent, child})
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d: %+v", len(pairs), pairs)
	}
	if pairs[0].Kind != "nested-gitdir" {
		t.Errorf("expected Kind=nested-gitdir, got %q", pairs[0].Kind)
	}
}

// TestDetectOverlaps_NonOverlappingGitdirs asserts sibling directories produce
// no pairs (~/git/a/ and ~/git/b/ do not nest).
func TestDetectOverlaps_NonOverlappingGitdirs(t *testing.T) {
	a := makeGitdirAccount("awork", "~/git/a/")
	b := makeGitdirAccount("bwork", "~/git/b/")
	pairs := DetectOverlaps([]identity.Account{a, b})
	if len(pairs) != 0 {
		t.Errorf("expected no pairs for sibling dirs, got %d: %+v", len(pairs), pairs)
	}
}

// TestDetectOverlaps_HasconfigURL asserts that two accounts whose hasconfig URL
// patterns share the same git@<host>: prefix produce one OverlapPair with Kind
// "hasconfig-url".
func TestDetectOverlaps_HasconfigURL(t *testing.T) {
	a := makeHasconfigAccount("gh-all", "git@ssh.github.com:**")
	b := makeHasconfigAccount("gh-personal", "git@ssh.github.com:personal/**")
	pairs := DetectOverlaps([]identity.Account{a, b})
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair for overlapping hasconfig, got %d: %+v", len(pairs), pairs)
	}
	if pairs[0].Kind != "hasconfig-url" {
		t.Errorf("expected Kind=hasconfig-url, got %q", pairs[0].Kind)
	}
}

// TestDetectOverlaps_ReservedBlockExcluded asserts that accounts named
// "baseline-include" or "_global" are NOT compared for overlaps (Pitfall 4).
func TestDetectOverlaps_ReservedBlockExcluded(t *testing.T) {
	reserved := makeGitdirAccount("baseline-include", "~/git/")
	global := makeGitdirAccount("_global", "~/git/")
	work := makeGitdirAccount("work", "~/git/")
	// All three share "~/git/" — without the filter all three would pair with "work".
	pairs := DetectOverlaps([]identity.Account{reserved, global, work})
	// Only "real" identities count — "baseline-include" and "_global" must be skipped.
	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs (reserved/global filtered), got %d: %+v", len(pairs), pairs)
	}
}

// TestDetectOverlaps_SingleAccount asserts that a single account cannot form a pair.
func TestDetectOverlaps_SingleAccount(t *testing.T) {
	a := makeGitdirAccount("solo", "~/git/solo/")
	pairs := DetectOverlaps([]identity.Account{a})
	if len(pairs) != 0 {
		t.Errorf("expected no pairs for single account, got %d", len(pairs))
	}
}

// TestCheckOverlap_FindingShape asserts that CheckOverlap wraps overlap pairs as
// doctor.Finding entries with the correct Family, Severity, and a nil Fix.
func TestCheckOverlap_FindingShape(t *testing.T) {
	a := makeGitdirAccount("alpha", "~/git/")
	b := makeGitdirAccount("beta", "~/git/personal/")
	deps := doctor.Deps{
		Identities: []identity.Account{a, b},
	}
	findings := CheckOverlap(deps)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Family != doctor.FamilyOverlap {
		t.Errorf("expected Family=FamilyOverlap, got %q", f.Family)
	}
	if f.Severity != doctor.SeverityWarning {
		t.Errorf("expected SeverityWarning, got %v", f.Severity)
	}
	if f.Fix != nil {
		t.Error("Fix must be nil for overlap findings (advisory only)")
	}
	// Explanation must name both identities.
	if !strings.Contains(f.Explanation, "alpha") || !strings.Contains(f.Explanation, "beta") {
		t.Errorf("Explanation must name both identities; got %q", f.Explanation)
	}
}

// TestCheckOverlap_ExitCodeWarning asserts that an overlap-only finding set
// produces exit code 1 (warning tier, D-15).
func TestCheckOverlap_ExitCodeWarning(t *testing.T) {
	a := makeGitdirAccount("x", "~/git/")
	b := makeGitdirAccount("y", "~/git/")
	deps := doctor.Deps{Identities: []identity.Account{a, b}}
	findings := CheckOverlap(deps)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	code := doctor.ExitCode(findings)
	if code != 1 {
		t.Errorf("expected exit code 1 (warning), got %d", code)
	}
}
