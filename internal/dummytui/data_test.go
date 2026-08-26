package dummytui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/tuikit"
)

// TestFixtureConsistency asserts the cross-fixture invariants the removed
// static-surface tests used to pin, so the shared fixture data stays
// internally coherent for the upcoming live Go TUI demo (and byte-mirrored
// with mockup-src/src/data/recipeFixtures.ts).
func TestFixtureConsistency(t *testing.T) {
	t.Run("allowed_signers email byte-matches user.email (GITUI-04)", func(t *testing.T) {
		want := GitScreenUserEmail + " " + GitScreenAllowedSignersKeyMaterial
		if GitScreenAllowedSignersLine != want {
			t.Errorf("GitScreenAllowedSignersLine = %q, want %q", GitScreenAllowedSignersLine, want)
		}
	})

	t.Run("header identity count matches the row fixture (MGR-02)", func(t *testing.T) {
		if got := len(IdentityManagerRows); got != ShellHeaderIdentityCount {
			t.Errorf("len(IdentityManagerRows) = %d, want ShellHeaderIdentityCount = %d", got, ShellHeaderIdentityCount)
		}
	})

	// The state→glyph taxonomy moved to internal/tuikit/design.go in the
	// D-17 extraction (both binaries render through it); the fixture rows
	// stayed here, so this cross-package invariant is exactly what keeps the
	// two halves coherent.
	t.Run("every identity row has a glyph for its state", func(t *testing.T) {
		for _, r := range IdentityManagerRows {
			if _, ok := tuikit.IdentityManagerGlyphByState[r.State]; !ok {
				t.Errorf("tuikit.IdentityManagerGlyphByState is missing state %q (row %q)", r.State, r.Name)
			}
		}
	})

	t.Run("fixer findings are the actionable subset of health findings (HLTH-04 hand-off)", func(t *testing.T) {
		actionable := 0
		for _, f := range HealthFindings {
			if f.SuggestedFix != "" {
				actionable++
			}
		}
		if len(FixerFindings) != actionable {
			t.Errorf("len(FixerFindings) = %d, want %d (health findings carrying a SuggestedFix)", len(FixerFindings), actionable)
		}
		for _, f := range FixerFindings {
			if f.SuggestedFix == "" {
				t.Errorf("FixerFindings entry %q has no SuggestedFix — the fixer lists only actionable problems", f.ID)
			}
			if got := HealthFindingByID(f.ID); got != f {
				t.Errorf("FixerFindings entry %q diverges from HealthFindings — must be the SAME data, not a re-derived copy", f.ID)
			}
		}
	})

	t.Run("fixer target is the health detail target (traceable hand-off)", func(t *testing.T) {
		if FixerTarget != HealthFindingDetailTarget {
			t.Errorf("FixerTarget (%q) != HealthFindingDetailTarget (%q)", FixerTarget.ID, HealthFindingDetailTarget.ID)
		}
	})

	t.Run("create-flow managed block carries the recipe-critical directives", func(t *testing.T) {
		for _, want := range []string{"Port 443", "IdentitiesOnly yes", CreateFlowSentinelBegin, CreateFlowSentinelEnd} {
			if !strings.Contains(CreateFlowManagedBlockText, want) {
				t.Errorf("CreateFlowManagedBlockText is missing %q", want)
			}
		}
	})
}

func TestFixtureBackendSatisfiesIdentityPlannerThroughNoop(t *testing.T) {
	var _ tuikit.IdentityPlanner = FixtureBackend{}
	rt := reflect.TypeOf(FixtureBackend{})
	found := false
	for i := range rt.NumField() {
		if rt.Field(i).Type == reflect.TypeOf(tuikit.NoopIdentityPlanner{}) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("FixtureBackend must satisfy IdentityPlanner by embedding NoopIdentityPlanner")
	}
}
