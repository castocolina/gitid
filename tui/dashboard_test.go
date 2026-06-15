package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
)

// TestDashboardInit verifies that dashboardModel.init() returns a tea.Batch
// producing one cmd per doctor family (seven total), not a single doctor.Run call.
// This is the RED test — it fails until dashboard.go is implemented.
func TestDashboardInit(t *testing.T) {
	m := newDashboardModel(fakeDocDeps())
	_, cmd := m.init()
	if cmd == nil {
		t.Fatal("dashboardModel.init() must return a non-nil tea.Cmd (Batch of 7 family cmds)")
	}

	// Execute the batch and collect messages. Each family must produce a familyResultMsg.
	// tea.Batch returns a cmd that, when called, produces a tea.BatchMsg.
	msg := cmd()
	batchMsg, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("init() cmd must return a tea.BatchMsg; got %T", msg)
	}

	if len(batchMsg) != len(doctor.Families()) {
		t.Errorf("expected %d cmds in Batch (one per family), got %d", len(doctor.Families()), len(batchMsg))
	}

	// Ensure each cmd in the batch produces a familyResultMsg.
	familiesSeen := make(map[doctor.Family]bool)
	for i, c := range batchMsg {
		if c == nil {
			t.Errorf("batchMsg[%d] is nil; want a familyResultMsg-producing cmd", i)
			continue
		}
		result := c()
		frm, ok := result.(familyResultMsg)
		if !ok {
			t.Errorf("batchMsg[%d] produced %T; want familyResultMsg", i, result)
			continue
		}
		familiesSeen[frm.family] = true
	}

	for _, fam := range doctor.Families() {
		if !familiesSeen[fam] {
			t.Errorf("no cmd produced familyResultMsg for family %q", fam)
		}
	}
}

// TestDashboardFamilyResult verifies that receiving a familyResultMsg with the
// current runID transitions that family's state from familyLoading to familyLoaded
// and stores its findings.
func TestDashboardFamilyResult(t *testing.T) {
	m := newDashboardModel(fakeDocDeps())

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
	m := newDashboardModel(fakeDocDeps())

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
	m := newDashboardModel(fakeDocDeps())

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
