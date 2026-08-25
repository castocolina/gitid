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

// TestRegionPredicateSatisfiedRejectsSymmetricCases is CR-10 (iteration 4):
// the ORIGINAL version of this test (then named
// TestRegionPredicateSatisfiedMatchesEitherSide) proved the OLD grammar —
// "holds if X appears on EITHER side" for contains:, "holds if X is missing
// from EITHER side" for absent: — which a probe against the real
// BuildRegionDiffs proved vacuous: every shipped predicate is anchored on
// text that one side structurally never contains, so the OR made the
// predicate permanently true no matter what the OTHER side rendered.
//
// The fixed grammar requires the predicate to express the SHAPE of the
// authorized divergence:
//   - contains:X must hold on BOTH sides (the marker survives; the
//     difference is authorized to be elsewhere in the region).
//   - absent:X must hold on EXACTLY ONE side (the presence/absence
//     asymmetry IS the authorized divergence — both-present AND
//     both-absent are unreviewed changes and must be rejected).
func TestRegionPredicateSatisfiedRejectsSymmetricCases(t *testing.T) {
	cases := []struct {
		name      string
		predicate string
		live      string
		approved  string
		want      bool
	}{
		{"empty predicate always matches", "", "anything", "anything else", true},
		{"contains holds when present on both sides", `contains:"ids"`, "3 ids", "8 ids", true},
		{"contains rejects when present on live only", `contains:"acme"`, "identity acme", "identity demo", false},
		{"contains rejects when present on approved only", `contains:"demo"`, "identity acme", "identity demo", false},
		{"contains rejects when present on neither side", `contains:"missing"`, "identity acme", "identity demo", false},
		{"absent holds when present on live only", `absent:"stale"`, "stale text", "current", true},
		{"absent holds when present on approved only", `absent:"stale"`, "current", "stale text", true},
		{"absent rejects when present on both sides — CR-10's vacuous-accept case", `absent:"stale"`, "stale text", "stale text too", false},
		{"absent rejects when present on neither side — symmetric absence is also unreviewed", `absent:"stale"`, "current live", "current approved", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := regionPredicateSatisfied(tc.predicate, tc.live, tc.approved); got != tc.want {
				t.Errorf("regionPredicateSatisfied(%q, %q, %q) = %v, want %v", tc.predicate, tc.live, tc.approved, got, tc.want)
			}
		})
	}
}

// TestBuildRegionDiffsRejectsUnrelatedLiveRegressionUnderProductionPredicate
// is CR-10's required reproduction: the review's exact probe methodology,
// reused as a permanent regression test. It extracts the REAL, shipped
// gitPreviewDisposition from gitScreenSpecs()'s "git-form-filled" entry —
// the actual production `absent:"gitdir:~/git/"` RegionDisposition value,
// not a hand-typed copy of the predicate string — and drives BuildRegionDiffs
// with an unrelated live-side regression that has nothing to do with the
// disposition's authorized divergence. Before the CR-10 fix this predicate
// vacuously accepted ANY live-side text because the frozen dummy fixture
// never contains "gitdir:~/git/" either way (the exact probe result quoted
// in 04-REVIEW.md's CR-10 finding) — this test proves it is now rejected,
// and stays wired to the real production disposition so it tracks any
// future edit to the shipped predicate.
func TestBuildRegionDiffsRejectsUnrelatedLiveRegressionUnderProductionPredicate(t *testing.T) {
	var prodDisposition RegionDisposition
	found := false
	for _, s := range gitScreenSpecs() {
		if s.ScreenID != "git-form-filled" {
			continue
		}
		for _, d := range s.RegionDispositions {
			if d.Region == RegionGitPreview {
				prodDisposition, found = d, true
			}
		}
	}
	if !found {
		t.Fatal("gitScreenSpecs()'s git-form-filled no longer carries a RegionGitPreview disposition — update this regression fixture")
	}
	if prodDisposition.Predicate == "" {
		t.Fatalf("production RegionGitPreview disposition on git-form-filled lost its Predicate — CR-10 requires it stay scoped, got %+v", prodDisposition)
	}

	spec := ScreenSpec{
		ScreenID:              "cr-10-regression-real-disposition",
		StateMarker:           "shared header",
		ApplicableLive:        true,
		ApplicableApprovedTUI: true,
		RequiredRegions:       []RegionName{RegionGitPreview},
		RegionDispositions:    []RegionDisposition{prodDisposition},
		NonApplicability: []SurfaceNonApplicability{{
			Surface: "approved-html", Decision: "CTX-D-02", Reason: "HTML is not a parity target.", Classification: "ux-improvement",
		}},
	}

	// Verbatim reproduction of the reviewer's probe fixture: an unrelated
	// live-side regression (garbage output) paired with the dummy's frozen,
	// structurally-unrelated approved sample.
	live := "shared header\n│ includeIf block\n│ TOTALLY BROKEN GARBAGE OUTPUT\n│ Write it\nfooter1\nfooter2\nfooter3\n"
	approved := "shared header\n│ includeIf block\n│ [includeIf \"gitdir:~/acme/\"]\n│ Write it\nfooter1\nfooter2\nfooter3\n"

	_, err := BuildRegionDiffs("test-commit", map[string]string{spec.ScreenID: live}, map[string]string{spec.ScreenID: approved}, []ScreenSpec{spec})
	if err == nil {
		t.Fatal("CR-10 regression: BuildRegionDiffs accepted an unrelated live-side regression under the production gitPreviewDisposition predicate — the predicate is vacuous again")
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
