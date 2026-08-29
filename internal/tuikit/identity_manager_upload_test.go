package tuikit

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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

// ---------------------------------------------------------------------------
// Task 2: the rotate/repair key-ceremony's own upload beat (09-06-PLAN.md).
// ---------------------------------------------------------------------------

// openKeyCeremonyAtReviewForIdentity selects the identity `downs` rows below
// the default selection, then drives the SAME action-menu -> "Generate new
// key" -> stage1 -> stage2 -> review sequence openKeyCeremonyAtReview uses,
// so a repair-mode identity (clientB, key-missing) can be reached too.
func openKeyCeremonyAtReviewForIdentity(t *testing.T, b Backend, downs int) App {
	t.Helper()
	a := NewApp(b)
	for i := 0; i < downs; i++ {
		a, _ = press(t, a, "down")
	}
	a = pressSeq(t, a, "a", "down", "down", "enter", "enter", "enter")
	m := identModel(t, a)
	if m.pane != paneKeyCeremony || m.keyCeremonyPhase != "review" {
		t.Fatalf("key ceremony = pane %v phase %q, want review", m.pane, m.keyCeremonyPhase)
	}
	return a
}

// confirmKeyCeremony presses Enter to confirm the review screen and runs the
// resulting commit command through Update, mirroring pressAndRun for the
// two-cmd case (commit dispatch, then the upload dispatch this task adds).
func confirmKeyCeremony(t *testing.T, a App) (App, tea.Cmd) {
	t.Helper()
	next, cmd := press(t, a, "enter")
	if cmd == nil {
		t.Fatal("confirm must dispatch the commit")
	}
	msg := cmd()
	model, followUp := next.Update(msg)
	out, ok := model.(App)
	if !ok {
		t.Fatalf("Update(commit msg) returned %T, want App", model)
	}
	return out, followUp
}

func TestRotateCeremonyRunsTheUploadBeatAfterCommitSucceeds(t *testing.T) {
	a := openKeyCeremonyAtReview(t, stubBackend{})
	a, uploadCmd := confirmKeyCeremony(t, a)
	m := identModel(t, a)
	if m.keyCeremonyPhase != "upload" {
		t.Fatalf("phase after a successful commit = %q, want %q", m.keyCeremonyPhase, "upload")
	}
	if !m.keyCeremonyUploadPending {
		t.Fatal("keyCeremonyUploadPending must be set once the commit succeeds")
	}
	if !strings.Contains(stripANSI(m.keyCeremony.view(deleteChoiceNoteWidth)), "Key ceremony completed.") {
		t.Fatal("the ceremony's own result screen must already be visible -- upload never gates it")
	}
	if uploadCmd == nil {
		t.Fatal("a successful commit must dispatch the upload beat (RunUploadForIdentity)")
	}
	msg := uploadCmd()
	model, _ := a.Update(msg)
	next := model.(App)
	m = identModel(t, next)
	if m.keyCeremonyUploadPending {
		t.Fatal("keyCeremonyUploadPending must clear once UploadRunMsg arrives")
	}
	if !uploadRunHasContent(m.keyCeremonyUploadRun) {
		t.Fatal("keyCeremonyUploadRun must hold the delivered view")
	}
	rendered := stripANSI(m.keyCeremony.view(deleteChoiceNoteWidth))
	if !strings.Contains(rendered, "Key ceremony completed.") {
		t.Fatal("the result screen must remain visible after the upload beat completes")
	}
}

func TestRepairCeremonyRunsTheUploadBeat(t *testing.T) {
	// clientB (index 6 in stubIdentityRows) is the key-missing fixture row,
	// which KeyActionFor routes to KeyCeremonyModeRepair.
	a := openKeyCeremonyAtReviewForIdentity(t, stubBackend{}, 6)
	if m := identModel(t, a); m.keyCeremonyMode != KeyCeremonyModeRepair {
		t.Fatalf("keyCeremonyMode = %q, want repair", m.keyCeremonyMode)
	}
	a, uploadCmd := confirmKeyCeremony(t, a)
	if uploadCmd == nil {
		t.Fatal("a successful repair commit must dispatch the upload beat too")
	}
	if m := identModel(t, a); !m.keyCeremonyUploadPending {
		t.Fatal("keyCeremonyUploadPending must be set after a successful repair commit")
	}
}

func TestKeyCeremonyUploadFailureStillAdvances(t *testing.T) {
	views := []UploadRunView{
		{Rows: []UploadResultRow{{Label: "Authentication", Outcome: UploadRowUploaded}}},
		{Rows: []UploadResultRow{{Label: "Authentication", Outcome: UploadRowFailed, Reason: "boom"}}},
		{Rows: []UploadResultRow{
			{Label: "Authentication", Outcome: UploadRowUploaded},
			{Label: "Signing", Outcome: UploadRowFailed, Reason: "scope"},
		}},
		{InventoryDegraded: true, ProviderName: "GitHub"},
		{AlreadyComplete: true, ProviderName: "GitHub"},
		{Skipped: true},
	}
	for i, view := range views {
		t.Run(fmt.Sprintf("outcome-%d", i), func(t *testing.T) {
			a := openKeyCeremonyAtReview(t, stubBackend{})
			a, _ = confirmKeyCeremony(t, a)
			model, _ := a.Update(UploadRunMsg{View: view})
			next := model.(App)
			m := identModel(t, next)
			if m.keyCeremonyUploadPending {
				t.Fatal("upload pending must clear regardless of outcome shape")
			}
			rendered := stripANSI(m.keyCeremony.view(deleteChoiceNoteWidth))
			if !strings.Contains(rendered, "Key ceremony completed.") {
				t.Fatalf("outcome %d must not gate the ceremony's own result screen:\n%s", i, rendered)
			}
		})
	}
}

func TestKeyCeremonyUploadUsesTheSharedRenderer(t *testing.T) {
	view := UploadRunView{Rows: []UploadResultRow{{Label: "Authentication", Command: "gh ssh-key add x.pub", Outcome: UploadRowUploaded}}}
	a := openKeyCeremonyAtReview(t, stubBackend{})
	a, _ = confirmKeyCeremony(t, a)
	model, _ := a.Update(UploadRunMsg{View: view})
	next := identModel(t, model.(App))
	s := Seed()
	sel, _ := next.selectedIdentity(s)
	rendered := stripANSI(next.renderKeyCeremony(sel))
	if !strings.Contains(rendered, "gh ssh-key add x.pub") {
		t.Fatalf("key-ceremony render must include the shared upload section, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Authentication key registered") {
		t.Fatalf("key-ceremony render must include the same frozen result-row string renderUploadSection produces, got:\n%s", rendered)
	}
}

func TestSameKeyCloneOmitsTheUploadSection(t *testing.T) {
	pre := ClonePrefillView{SourceName: "personal", CloneName: "clone2", AliasPrefix: "clone2", Hostname: "ssh.github.com", Port: "443", ReuseKeyPath: "~/.ssh/id_ed25519_personal"}
	w := newWizardPrefilled(stubBackend{}, pre)
	w, cmd := w.checkUploadEligibility()
	if cmd != nil {
		t.Fatal("a same-key clone must not dispatch an eligibility probe -- there is nothing new to upload")
	}
	if w.uploadRowVisible() {
		t.Fatal("a same-key clone must not show the upload checkbox row")
	}
}

func TestNewKeyCloneRunsTheUploadSection(t *testing.T) {
	pre := ClonePrefillView{SourceName: "personal", CloneName: "clone3", AliasPrefix: "clone3", Hostname: "ssh.github.com", Port: "443"}
	w := newWizardPrefilled(stubBackend{}, pre)
	if w.reuseKeyPath() != "" {
		t.Fatal("setup: prefill must not carry a reuse key path")
	}
	_, cmd := w.checkUploadEligibility()
	if cmd == nil {
		t.Fatal("a new-key clone must dispatch the SAME eligibility probe the create wizard runs")
	}
}
