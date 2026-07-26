package tui

// health_test.go — Tests for the Health view sub-model (Plan 03 GREEN).
//
// Tests are ported in shape from tui/dashboard.go (Phase 5.5) and adapted to
// the Phase 5.6 Health view (healthViewModel). The test NAMES are LOCKED by
// VALIDATION.md.
//
// Key differences from Phase 5 dashboard_test.go:
//   - The Health view is a sub-model field on rootModel, not a pushed screenModel.
//   - familyResultMsg and the stale-guard runID pattern are preserved verbatim.
//   - The "8 families" count matches doctor.Families() (includes FamilyOverlap).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
)

// fakeTUIDocDepsForHealth returns a minimal doctor.Deps with all 9 CheckFn
// fields set to non-nil stubs (return empty findings). Used by health tests so
// nil-guard errors don't bleed into stale-guard / badge tests.
// Includes CheckRedundancy (UAT G-4 / SSH-03) as the ninth check.
func fakeTUIDocDepsForHealth() doctor.Deps {
	noop := func(_ doctor.Deps) []doctor.Finding { return nil }
	return doctor.Deps{
		CheckDeps:       noop,
		CheckPerms:      noop,
		CheckCoherence:  noop,
		CheckOrphans:    noop,
		CheckSigning:    noop,
		CheckAgent:      noop,
		CheckBaseline:   noop,
		CheckOverlap:    noop,
		CheckRedundancy: noop,
	}
}

// fakeTUIDepsForHealth returns a tuiDeps wrapping fakeTUIDocDepsForHealth.
func fakeTUIDepsForHealth() tuiDeps {
	return tuiDeps{doctor: fakeTUIDocDepsForHealth()}
}

// TestHealthFamilies verifies that the Health view's init() returns a tea.Batch
// containing one familyResultMsg-producing cmd for each of the 9 doctor families
// (TUI-06/D-11: async per-family streaming with runID stale-guard, port of
// TestDashboardInit from Phase 5). The 9th family is FamilyRedundancy (UAT G-4).
// Requirement: TUI-06/D-11 (async health streaming).
// Closes: Plan 03, Plan 11.
func TestHealthFamilies(t *testing.T) {
	m := newHealthModel(fakeTUIDepsForHealth())
	_, cmd := m.init()
	if cmd == nil {
		t.Fatal("init() must return a non-nil Batch cmd")
	}

	rawMsg := cmd()
	batchMsg, ok := rawMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("init() cmd() must return tea.BatchMsg; got %T", rawMsg)
	}

	// Each family produces one family cmd + one spinner Tick = 2 entries per family.
	expectedCount := len(doctor.Families()) * 2
	if len(batchMsg) != expectedCount {
		t.Errorf("expected %d cmds (9 family + 9 spinner ticks), got %d", expectedCount, len(batchMsg))
	}

	// Collect all families seen from the family cmds.
	familiesSeen := make(map[doctor.Family]bool)
	for _, c := range batchMsg {
		if c == nil {
			continue
		}
		msg := c()
		if res, ok := msg.(familyResultMsg); ok {
			familiesSeen[res.family] = true
		}
	}

	// Every family must be covered.
	for _, fam := range doctor.Families() {
		if !familiesSeen[fam] {
			t.Errorf("family %q missing from init() batch", fam)
		}
	}
}

// TestHealthStaleResultGuard verifies that a familyResultMsg with an outdated
// runID is silently dropped (port of TestDashboardStaleResult from Phase 5).
// Requirement: TUI-06/D-11 (runID stale-guard, RESEARCH Pitfall 4).
// Closes: Plan 03.
func TestHealthStaleResultGuard(t *testing.T) {
	m := newHealthModel(fakeTUIDepsForHealth())
	m.runID = 5

	stale := familyResultMsg{runID: 3, family: doctor.FamilyDeps, findings: nil}
	m2, _ := m.update(stale)
	hm := m2

	// Family state must remain loading (unchanged) — stale result was dropped.
	idx := familyIndex(doctor.FamilyDeps)
	if hm.families[idx] != familyLoading {
		t.Errorf("stale result must not change family state; got %v", hm.families[idx])
	}
	if _, exists := hm.findings[doctor.FamilyDeps]; exists {
		t.Error("stale result must not populate findings")
	}
}

// TestHealthWarningGlyphDistinct verifies that renderFinding for a SeverityWarning
// finding contains "!" and not "✗"; a SeverityError finding contains "✗" (D-10 / Eval #2).
// Closes: Plan 03.
func TestHealthWarningGlyphDistinct(t *testing.T) {
	// asciiMode() degrades to ASCII glyphs when $TERM is unset or "dumb"
	// (correct product behavior). CI runners leave $TERM unset, so pin a
	// UTF-8-capable terminal to exercise the Unicode glyph contract this test asserts.
	t.Setenv("TERM", "xterm-256color")
	warning := doctor.Finding{
		Family:   doctor.FamilyDeps,
		Severity: doctor.SeverityWarning,
		Title:    "some warning",
	}
	warnStr := renderFinding(warning)
	if !strings.Contains(warnStr, "!") {
		t.Errorf("warning finding must contain '!'; got: %q", warnStr)
	}
	if strings.Contains(warnStr, "✗") {
		t.Errorf("warning finding must NOT contain '✗' (must be distinct from error); got: %q", warnStr)
	}

	errFind := doctor.Finding{
		Family:   doctor.FamilyDeps,
		Severity: doctor.SeverityError,
		Title:    "some error",
	}
	errStr := renderFinding(errFind)
	if !strings.Contains(errStr, "✗") {
		t.Errorf("error finding must contain '✗'; got: %q", errStr)
	}
}

// TestHealthNilCheckFnErrors verifies that makeFamilyCmd with a nil CheckFn
// returns a familyResultMsg carrying an error (not a silent pass), and that a
// panicking CheckFn is recovered into an error msg (D-16 mitigation, Pitfall 7).
// Closes: Plan 03.
func TestHealthNilCheckFnErrors(t *testing.T) {
	// Nil CheckFn must produce an error result.
	nilDeps := doctor.Deps{} // all CheckFn fields nil
	cmd := makeFamilyCmd(1, doctor.FamilyDeps, nilDeps)
	msg := cmd()
	res, ok := msg.(familyResultMsg)
	if !ok {
		t.Fatalf("expected familyResultMsg; got %T", msg)
	}
	if res.err == nil {
		t.Error("nil CheckFn must return err != nil (silent pass forbidden)")
	}
	if len(res.findings) != 0 {
		t.Error("nil CheckFn must return empty findings")
	}

	// Panicking CheckFn must be recovered into an error result.
	panicDeps := doctor.Deps{
		CheckDeps: func(_ doctor.Deps) []doctor.Finding {
			panic("deliberate test panic")
		},
	}
	panicCmd := makeFamilyCmd(1, doctor.FamilyDeps, panicDeps)
	panicMsg := panicCmd()
	panicRes, ok := panicMsg.(familyResultMsg)
	if !ok {
		t.Fatalf("expected familyResultMsg from panicking cmd; got %T", panicMsg)
	}
	if panicRes.err == nil {
		t.Error("panicking CheckFn must be recovered into err != nil")
	}
}

// TestHealthBadgesDerived verifies that badgesFromFindings maps each identity
// name to its worst finding severity (drives the sidebar badge map, D-08).
// Closes: Plan 03.
func TestHealthBadgesDerived(t *testing.T) {
	findings := map[doctor.Family][]doctor.Finding{
		doctor.FamilyDeps: {
			{
				Family:   doctor.FamilyDeps,
				Severity: doctor.SeverityWarning,
				Title:    "git missing",
				// IdentityName: empty — global finding, not identity-scoped
			},
		},
		doctor.FamilyPerms: {
			{
				Family:       doctor.FamilyPerms,
				Severity:     doctor.SeverityError,
				Title:        "bad perms on alice key",
				IdentityName: "alice",
			},
			{
				Family:       doctor.FamilyPerms,
				Severity:     doctor.SeverityWarning,
				Title:        "warning for alice",
				IdentityName: "alice",
			},
		},
		doctor.FamilyCoherence: {
			{
				Family:       doctor.FamilyCoherence,
				Severity:     doctor.SeverityInfo,
				Title:        "info for bob",
				IdentityName: "bob",
			},
		},
	}

	badges := badgesFromFindings(findings)

	// alice: worst is Error (not Warning).
	if sev, ok := badges["alice"]; !ok {
		t.Error("alice must have a badge entry")
	} else if sev != doctor.SeverityError {
		t.Errorf("alice badge: expected SeverityError, got %v", sev)
	}

	// bob: only Info.
	if sev, ok := badges["bob"]; !ok {
		t.Error("bob must have a badge entry")
	} else if sev != doctor.SeverityInfo {
		t.Errorf("bob badge: expected SeverityInfo, got %v", sev)
	}

	// Global (empty IdentityName) findings should not create a badge entry for "".
	// They are not identity-scoped.
	if _, ok := badges[""]; ok {
		t.Error("global findings (empty IdentityName) must not create a badge entry for empty string")
	}
}

// TestHealthFamilyRedundancyDispatch verifies that makeFamilyCmd for
// FamilyRedundancy dispatches d.CheckRedundancy through the real family-run
// path (anti-blindspot: MEMORY doctor wiring blindspot / UAT G-4).
// A non-nil CheckRedundancy wired in deps must produce a FamilyRedundancy
// finding; a nil CheckRedundancy must produce an error (not a silent pass).
func TestHealthFamilyRedundancyDispatch(t *testing.T) {
	wantTitle := "fake redundancy warning"

	// Non-nil CheckRedundancy: must be dispatched and produce its finding.
	docDeps := fakeTUIDocDepsForHealth()
	docDeps.CheckRedundancy = func(_ doctor.Deps) []doctor.Finding {
		return []doctor.Finding{{
			Family:   doctor.FamilyRedundancy,
			Severity: doctor.SeverityWarning,
			Title:    wantTitle,
			Fix:      nil,
		}}
	}
	cmd := makeFamilyCmd(1, doctor.FamilyRedundancy, docDeps)
	msg := cmd()
	res, ok := msg.(familyResultMsg)
	if !ok {
		t.Fatalf("expected familyResultMsg from FamilyRedundancy cmd; got %T", msg)
	}
	if res.err != nil {
		t.Errorf("non-nil CheckRedundancy must not produce an error; got %v", res.err)
	}
	if len(res.findings) != 1 || res.findings[0].Title != wantTitle {
		t.Errorf("expected finding %q; got %v", wantTitle, res.findings)
	}

	// Nil CheckRedundancy must produce an error (not a silent pass — anti-blindspot D-16).
	nilDeps := doctor.Deps{CheckRedundancy: nil}
	nilCmd := makeFamilyCmd(1, doctor.FamilyRedundancy, nilDeps)
	nilMsg := nilCmd()
	nilRes, ok := nilMsg.(familyResultMsg)
	if !ok {
		t.Fatalf("expected familyResultMsg from nil-CheckRedundancy cmd; got %T", nilMsg)
	}
	if nilRes.err == nil {
		t.Error("nil CheckRedundancy must surface an error (not a silent pass, D-16)")
	}
}
