package tuikit

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func actionMenuRowCount() int { return len(actionMenuLabels()) }

func TestActionMenuHasFiveRowsAndTheCountIsDerived(t *testing.T) {
	labels := actionMenuLabels()
	if got := len(labels); got != 5 {
		t.Fatalf("len(actionMenuLabels()) = %d, want 5", got)
	}
	if labels[4] != IdentityManagerActionRegisterKey {
		t.Errorf("fifth label = %q, want %q", labels[4], IdentityManagerActionRegisterKey)
	}
	if actionMenuRowCount() != len(labels) {
		t.Fatal("row count must be derived from actionMenuLabels (09-RESEARCH.md Pitfall 3)")
	}

	s := Seed()
	m := newIdentitiesModel(stubBackend{}, s)
	m.pane = paneActions
	m.actionsFocus = 0
	for i := 0; i < 4; i++ {
		m = m.handleActionsKey(pressKey("down"), s).model.(identitiesModel)
	}
	if m.actionsFocus != 4 {
		t.Fatalf("after 4 downs, actionsFocus = %d, want 4", m.actionsFocus)
	}
	m = m.handleActionsKey(pressKey("down"), s).model.(identitiesModel)
	if m.actionsFocus != 0 {
		t.Fatalf("wrap from index 4 = %d, want 0", m.actionsFocus)
	}
}

func TestActionMenuFifthRowOpensRegisterKeyPane(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "a", "down", "down", "down", "down", "enter")
	m := identModel(t, a)
	if m.pane != paneRegisterKey {
		t.Fatalf("pane = %v, want paneRegisterKey", m.pane)
	}
	if m.registerKeyName != "personal" {
		t.Errorf("registerKeyName = %q, want personal", m.registerKeyName)
	}
}

func TestDetailUKeyOpensRegisterKeyPane(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "u")
	m := identModel(t, a)
	if m.pane != paneRegisterKey {
		t.Fatalf("pane = %v, want paneRegisterKey", m.pane)
	}
	if m.registerKeyName != "personal" {
		t.Errorf("registerKeyName = %q, want personal", m.registerKeyName)
	}
}

func TestRegisterKeyPaneResolvesItsPlanAsynchronously(t *testing.T) {
	s := Seed()
	b := stubBackend{}
	m := newIdentitiesModel(b, s)
	sel, ok := m.selectedIdentity(s)
	if !ok {
		t.Fatal("seed has no selected identity")
	}
	next, cmd := m.openRegisterKey(sel)
	if cmd == nil {
		t.Fatal("openRegisterKey must return a non-nil command (R3: plan is async)")
	}
	if next.pane != paneRegisterKey {
		t.Fatalf("pane = %v, want paneRegisterKey", next.pane)
	}
	sv := next.view(s, minFrameWidth, minFrameHeight)
	heading := fmt.Sprintf(RegisterKeyModalHeadingFmt, sel.Name, "GitHub")
	if !strings.Contains(stripANSI(sv.body), heading) {
		t.Errorf("heading before plan arrives missing %q in:\n%s", heading, stripANSI(sv.body))
	}
	if strings.Contains(sv.body, "Running:") {
		t.Error("opening the pane must not run a provider command on the update path")
	}
}

func TestRegisterKeyPaneRunsImmediatelyWhenPlanIsReady(t *testing.T) {
	s := Seed()
	m := newIdentitiesModel(stubBackend{}, s)
	sel, _ := m.selectedIdentity(s)
	m, _ = m.openRegisterKey(sel)
	res := m.handleMsg(RegisterKeyPlanMsg{
		Name: sel.Name,
		View: UploadEligibilityView{State: UploadEligibilityReady, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com"},
	}, s)
	if res.cmd == nil {
		t.Fatal("ready plan must dispatch a non-nil follow-up command (RunUploadForIdentity)")
	}
	next := res.model.(identitiesModel)
	if !next.registerKeyPending {
		t.Error("ready plan must set registerKeyPending")
	}
}

func TestRegisterKeyPaneShowsManualFallbackWhenNotReady(t *testing.T) {
	states := []struct {
		name  string
		state UploadEligibilityState
	}{
		{"omitted", UploadEligibilityOmitted},
		{"disabled", UploadEligibilityDisabled},
		{"unauth", UploadEligibilityUnauth},
	}
	s := Seed()
	sel, _ := newIdentitiesModel(stubBackend{}, s).selectedIdentity(s)
	for _, tt := range states {
		t.Run(tt.name, func(t *testing.T) {
			m := newIdentitiesModel(stubBackend{}, s)
			m, _ = m.openRegisterKey(sel)
			res := m.handleMsg(RegisterKeyPlanMsg{
				Name: sel.Name,
				View: UploadEligibilityView{State: tt.state, ProviderName: "GitHub", Hostname: "github.com"},
			}, s)
			if res.cmd != nil {
				t.Fatal("non-ready plan must not dispatch an upload command")
			}
			next := res.model.(identitiesModel)
			if next.registerKeyPending {
				t.Error("non-ready plan must not set registerKeyPending")
			}
			body := stripANSI(next.view(s, minFrameWidth, minFrameHeight).body)
			if !strings.Contains(body, UploadManualHeading) && !strings.Contains(body, "manually") {
				t.Errorf("manual-fallback text missing for state %s:\n%s", tt.name, body)
			}
		})
	}
}

func TestRegisterKeyPlanErrorFailsClosed(t *testing.T) {
	s := Seed()
	m := newIdentitiesModel(stubBackend{}, s)
	sel, _ := m.selectedIdentity(s)
	m, _ = m.openRegisterKey(sel)
	res := m.handleMsg(RegisterKeyPlanMsg{Name: sel.Name, Err: errors.New("probe failed")}, s)
	if res.cmd != nil {
		t.Fatal("plan error must not dispatch an upload command")
	}
	next := res.model.(identitiesModel)
	body := stripANSI(next.view(s, minFrameWidth, minFrameHeight).body)
	if !strings.Contains(body, "probe failed") {
		t.Errorf("error must render, got:\n%s", body)
	}
}

func TestStaleRegisterKeyPlanMsgIsDiscarded(t *testing.T) {
	s := Seed()
	m := newIdentitiesModel(stubBackend{}, s)
	sel, _ := m.selectedIdentity(s)
	m, _ = m.openRegisterKey(sel)
	before := m
	res := m.handleMsg(RegisterKeyPlanMsg{
		Name: "someone-else",
		View: UploadEligibilityView{State: UploadEligibilityReady, ProviderName: "GitHub"},
	}, s)
	next := res.model.(identitiesModel)
	if next.registerKeyPending != before.registerKeyPending || res.cmd != nil {
		t.Fatal("stale plan message must leave the model unchanged")
	}
	if next.registerKeyPlan.State != before.registerKeyPlan.State {
		t.Fatal("stale plan message must not apply its view")
	}
}

func TestRegisterKeyPaneEscReturnsToDetail(t *testing.T) {
	s := Seed()
	m := newIdentitiesModel(stubBackend{}, s)
	sel, _ := m.selectedIdentity(s)
	m, _ = m.openRegisterKey(sel)
	res := m.handleRegisterKeyKey(pressKey("esc"), s)
	next := res.model.(identitiesModel)
	if next.pane != paneDetail {
		t.Fatalf("pane after esc = %v, want paneDetail", next.pane)
	}
}

func TestStaleUploadRunMsgIsIgnoredOutsideThePane(t *testing.T) {
	a := identitiesApp()
	m := identModel(t, a)
	if m.pane != paneDetail {
		t.Fatal("setup: want paneDetail")
	}
	model, _ := a.Update(UploadRunMsg{View: UploadRunView{AlreadyComplete: true, ProviderName: "GitHub"}})
	next := model.(App)
	got := identModel(t, next)
	if got.pane != paneDetail || got.registerKeyPending || uploadRunHasContent(got.registerKeyRun) {
		t.Fatal("UploadRunMsg must be inert while paneDetail is active")
	}
}

func TestRegisterKeyPaneMatchesSiblingPaneGeometry(t *testing.T) {
	s := Seed()
	b := stubBackend{}
	clone := newIdentitiesModel(b, s)
	sel, _ := clone.selectedIdentity(s)
	clone = clone.openClonePrompt(sel)
	rk, _ := newIdentitiesModel(b, s).openRegisterKey(sel)
	cv := clone.view(s, minFrameWidth, minFrameHeight)
	rv := rk.view(s, minFrameWidth, minFrameHeight)
	if len(rv.crumbs) != len(cv.crumbs) {
		t.Errorf("crumbs length = %d, want clone's %d", len(rv.crumbs), len(cv.crumbs))
	}
	if len(rv.crumbs) != 2 || rv.crumbs[1] != "Register key" {
		t.Errorf("crumbs = %v, want [name, Register key]", rv.crumbs)
	}
	if len(rv.actions) != 0 {
		t.Errorf("footer actions = %v, want the Esc-only (empty extra-actions) shape", rv.actions)
	}
	if !strings.Contains(rv.status, "Esc returns to the identity detail without writing anything") {
		t.Errorf("status = %q, want the sibling Esc sentence", rv.status)
	}
	if !strings.Contains(strings.ToLower(rv.status), "runs on open") && !strings.Contains(strings.ToLower(rv.status), "registration runs") {
		t.Errorf("status = %q, want it to state that registration runs on open (R13)", rv.status)
	}
}
