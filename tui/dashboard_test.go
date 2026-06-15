package tui

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
)

// TestDashboardInit verifies that dashboardModel.init() returns a tea.Batch
// containing one familyResultMsg-producing cmd per doctor family (seven total),
// not a single doctor.Run call. The batch ALSO contains one spinner.Tick per
// family so the loading spinners animate (WR-01); those tick cmds are present
// in addition to the seven family cmds.
func TestDashboardInit(t *testing.T) {
	m := newDashboardModel(fakeTUIDocDeps())
	_, cmd := m.init()
	if cmd == nil {
		t.Fatal("dashboardModel.init() must return a non-nil tea.Cmd (Batch of family + spinner-tick cmds)")
	}

	// Execute the batch and collect messages. Each family must produce a familyResultMsg.
	// tea.Batch returns a cmd that, when called, produces a tea.BatchMsg.
	msg := cmd()
	batchMsg, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("init() cmd must return a tea.BatchMsg; got %T", msg)
	}

	// The batch must include the family cmds plus one spinner tick per family
	// (WR-01): 7 family + 7 tick = 14.
	wantLen := len(doctor.Families()) * 2
	if len(batchMsg) != wantLen {
		t.Errorf("expected %d cmds in Batch (family + spinner ticks), got %d", wantLen, len(batchMsg))
	}

	// Ensure every family is covered by a familyResultMsg-producing cmd, and that
	// at least one spinner.TickMsg-producing cmd is present (the WR-01 animation
	// seed). Non-family cmds must be spinner ticks, never something unexpected.
	familiesSeen := make(map[doctor.Family]bool)
	tickSeen := false
	for i, c := range batchMsg {
		if c == nil {
			t.Errorf("batchMsg[%d] is nil; want a family or spinner-tick cmd", i)
			continue
		}
		switch result := c().(type) {
		case familyResultMsg:
			familiesSeen[result.family] = true
		case spinner.TickMsg:
			tickSeen = true
		default:
			t.Errorf("batchMsg[%d] produced %T; want familyResultMsg or spinner.TickMsg", i, result)
		}
	}

	for _, fam := range doctor.Families() {
		if !familiesSeen[fam] {
			t.Errorf("no cmd produced familyResultMsg for family %q", fam)
		}
	}
	if !tickSeen {
		t.Error("init() Batch must include at least one spinner.Tick cmd so spinners animate (WR-01)")
	}
}

// TestDashboardFamilyResult verifies that receiving a familyResultMsg with the
// current runID transitions that family's state from familyLoading to familyLoaded
// and stores its findings.
func TestDashboardFamilyResult(t *testing.T) {
	m := newDashboardModel(fakeTUIDocDeps())

	// Simulate receiving a result for FamilyDeps with the current runID.
	finding := doctor.Finding{
		Family:   doctor.FamilyDeps,
		Severity: doctor.SeverityInfo,
		Title:    "ssh present",
	}
	msg := familyResultMsg{
		runID:    m.runID,
		family:   doctor.FamilyDeps,
		findings: []doctor.Finding{finding},
	}
	updated, _ := m.update(msg)
	dm, ok := updated.(dashboardModel)
	if !ok {
		t.Fatalf("update returned %T; want dashboardModel", updated)
	}

	// The family index for FamilyDeps is 0 (first in Families()).
	familyIdx := familyIndex(doctor.FamilyDeps)
	if dm.families[familyIdx] != familyLoaded {
		t.Errorf("after familyResultMsg: families[%d] = %v; want familyLoaded", familyIdx, dm.families[familyIdx])
	}
	if len(dm.findings[doctor.FamilyDeps]) != 1 {
		t.Errorf("after familyResultMsg: findings[FamilyDeps] len = %d; want 1", len(dm.findings[doctor.FamilyDeps]))
	}
}

// TestDashboardRefresh verifies that a Refresh key press (keys.Refresh / "r")
// increments runID and resets all families back to familyLoading.
func TestDashboardRefresh(t *testing.T) {
	m := newDashboardModel(fakeTUIDocDeps())

	// First, mark all families as loaded.
	for i := range m.families {
		m.families[i] = familyLoaded
	}
	oldRunID := m.runID

	// Send the Refresh key press.
	refreshMsg := tea.KeyPressMsg{Text: "r"}
	updated, cmd := m.update(refreshMsg)
	dm, ok := updated.(dashboardModel)
	if !ok {
		t.Fatalf("update returned %T; want dashboardModel", updated)
	}

	if dm.runID != oldRunID+1 {
		t.Errorf("after Refresh: runID = %d; want %d", dm.runID, oldRunID+1)
	}
	for i, state := range dm.families {
		if state != familyLoading {
			t.Errorf("after Refresh: families[%d] = %v; want familyLoading", i, state)
		}
	}
	if cmd == nil {
		t.Error("Refresh must return a non-nil tea.Cmd (new Batch of family cmds)")
	}
}

// TestDashboardStaleResult verifies that a familyResultMsg with a mismatched
// runID (old runID after a refresh) is silently ignored — the family state
// must not change.
func TestDashboardStaleResult(t *testing.T) {
	m := newDashboardModel(fakeTUIDocDeps())

	// Advance runID to simulate a refresh having occurred.
	m.runID = 5
	// All families remain loading (initial state).

	// Deliver a stale result with old runID = 3.
	staleMsg := familyResultMsg{
		runID:    3,
		family:   doctor.FamilyDeps,
		findings: []doctor.Finding{{Title: "should be ignored"}},
	}
	updated, _ := m.update(staleMsg)
	dm, ok := updated.(dashboardModel)
	if !ok {
		t.Fatalf("update returned %T; want dashboardModel", updated)
	}

	familyIdx := familyIndex(doctor.FamilyDeps)
	if dm.families[familyIdx] != familyLoading {
		t.Errorf("stale result: families[%d] = %v; want familyLoading (unchanged)", familyIdx, dm.families[familyIdx])
	}
	if len(dm.findings[doctor.FamilyDeps]) != 0 {
		t.Errorf("stale result: findings[FamilyDeps] should be empty; got %d", len(dm.findings[doctor.FamilyDeps]))
	}
}

// TestMakeFamilyCmdDispatchesEveryFamily is the CR-02 regression guard: every
// family advertised by doctor.Families() must have a real dispatch case in
// makeFamilyCmd AND a wired Deps field. A missing case (the bug that hid the
// Overlap warning behind a false "all checks passed") now surfaces as a
// familyResultMsg carrying an error, never as a silent empty-findings pass.
func TestMakeFamilyCmdDispatchesEveryFamily(t *testing.T) {
	deps := fakeDocDeps() // fully wired, returns no findings for every family
	for _, fam := range doctor.Families() {
		cmd := makeFamilyCmd(0, fam, deps)
		if cmd == nil {
			t.Errorf("makeFamilyCmd(%q) returned nil cmd", fam)
			continue
		}
		res, ok := cmd().(familyResultMsg)
		if !ok {
			t.Errorf("makeFamilyCmd(%q) produced %T; want familyResultMsg", fam, cmd())
			continue
		}
		if res.err != nil {
			t.Errorf("family %q is not dispatched/wired in the TUI: %v", fam, res.err)
		}
	}
}

// TestMakeFamilyCmdUnwiredFamilyErrors proves the nil-guard: a family whose
// Deps field is nil must produce an error, not a false "passed".
func TestMakeFamilyCmdUnwiredFamilyErrors(t *testing.T) {
	deps := fakeDocDeps()
	deps.CheckOverlap = nil // simulate a forgotten wiring
	res, ok := makeFamilyCmd(0, doctor.FamilyOverlap, deps)().(familyResultMsg)
	if !ok {
		t.Fatalf("want familyResultMsg")
	}
	if res.err == nil {
		t.Error("unwired family must yield an error, not a silent empty-findings pass")
	}
}
