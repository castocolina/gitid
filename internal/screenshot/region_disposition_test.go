//go:build screenshot

package screenshot

import "testing"

// TestValidRegionPredicateAcceptsEmptyAndScopedGrammar proves WR-19's
// contract: Predicate is optional (empty is always valid — blanket
// acceptance is preserved for every existing disposition), and when set it
// must use the same "contains:"/"absent:" grammar the e2e git-screen
// allowlist enforces (CR-04: "differs" is never a valid predicate).
func TestValidRegionPredicateAcceptsEmptyAndScopedGrammar(t *testing.T) {
	cases := []struct {
		predicate string
		want      bool
	}{
		{"", true},
		{`contains:"gitdir default"`, true},
		{`absent:"stale text"`, true},
		{"differs", false},
		{"contains", false},
		{"something else", false},
	}
	for _, tc := range cases {
		if got := validRegionPredicate(tc.predicate); got != tc.want {
			t.Errorf("validRegionPredicate(%q) = %v, want %v", tc.predicate, got, tc.want)
		}
	}
}

// TestRegionPredicateSatisfiedMatchesEitherSide proves regionPredicateSatisfied
// mirrors e2e/git_configuration_pty_e2e_test.go's gitScreenPredicateSatisfied:
// contains:X holds if X appears on EITHER side of the live/approved pair;
// absent:X holds if X is missing from EITHER side. An empty predicate always
// matches (WR-19: optional, blanket-acceptance default preserved).
func TestRegionPredicateSatisfiedMatchesEitherSide(t *testing.T) {
	cases := []struct {
		name      string
		predicate string
		live      string
		approved  string
		want      bool
	}{
		{"empty predicate always matches", "", "anything", "anything else", true},
		{"contains matches live only", `contains:"acme"`, "identity acme", "identity demo", true},
		{"contains matches approved only", `contains:"demo"`, "identity acme", "identity demo", true},
		{"contains matches neither side", `contains:"missing"`, "identity acme", "identity demo", false},
		{"absent holds when missing from live", `absent:"stale"`, "current", "stale text", true},
		{"absent holds when missing from approved", `absent:"stale"`, "stale text", "current", true},
		{"absent fails when present on both sides", `absent:"stale"`, "stale text", "stale text too", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := regionPredicateSatisfied(tc.predicate, tc.live, tc.approved); got != tc.want {
				t.Errorf("regionPredicateSatisfied(%q, %q, %q) = %v, want %v", tc.predicate, tc.live, tc.approved, got, tc.want)
			}
		})
	}
}

// TestUxRegionDifferenceScopedCarriesPredicate proves the new constructor
// actually threads Predicate through, unlike uxRegionDifference (which
// intentionally leaves it empty — today's unscoped acceptance, unchanged).
func TestUxRegionDifferenceScopedCarriesPredicate(t *testing.T) {
	scoped := uxRegionDifferenceScoped(RegionGitPreview, "divergence", "D-01", "reason", `contains:"needle"`)
	if scoped.Predicate != `contains:"needle"` {
		t.Errorf("uxRegionDifferenceScoped.Predicate = %q, want the supplied predicate", scoped.Predicate)
	}
	unscoped := uxRegionDifference(RegionGitPreview, "divergence", "D-01", "reason")
	if unscoped.Predicate != "" {
		t.Errorf("uxRegionDifference.Predicate = %q, want empty (unscoped, backward-compatible default)", unscoped.Predicate)
	}
}
