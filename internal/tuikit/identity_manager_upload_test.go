package tuikit

import (
	"errors"
	"fmt"
	"os"
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

// TestRegisterKeyPaneEscWhilePendingSurfacesAbandonNote is the WR-05
// sub-defect regression (review iteration 5): the D-08 register-key pane's
// esc handler used to return no note at all, so a user who pressed Esc
// while the in-flight registration beat was still running got no
// indication that a `gh`/`glab` call was going to complete anyway after
// the pane closed — the wizard's byte-for-byte identical situation already
// has a mitigation (wizardAbandonUploadNote, WR-13/iteration 3); this pane
// had none.
func TestRegisterKeyPaneEscWhilePendingSurfacesAbandonNote(t *testing.T) {
	s := Seed()
	m := newIdentitiesModel(stubBackend{}, s)
	sel, ok := m.selectedIdentity(s)
	if !ok {
		t.Fatal("seed has no selected identity")
	}
	m, _ = m.openRegisterKey(sel)
	res := m.handleMsg(RegisterKeyPlanMsg{
		Name: sel.Name,
		View: UploadEligibilityView{State: UploadEligibilityReady, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com"},
	}, s)
	m, ok = res.model.(identitiesModel)
	if !ok {
		t.Fatalf("handleMsg returned %T, want identitiesModel", res.model)
	}
	if !m.registerKeyPending {
		t.Fatal("setup: a Ready plan must set registerKeyPending (R13: registration runs on open)")
	}

	escRes := m.handleRegisterKeyKey(pressKey("esc"), s)
	if escRes.note == "" {
		t.Fatal("Esc while a registration beat is in flight must surface an abandon note, got none")
	}
	if !strings.Contains(escRes.note, sel.Name) {
		t.Errorf("note = %q, want it to name the identity %q", escRes.note, sel.Name)
	}
	after, ok := escRes.model.(identitiesModel)
	if !ok {
		t.Fatalf("handleRegisterKeyKey returned %T, want identitiesModel", escRes.model)
	}
	if after.pane != paneDetail {
		t.Errorf("pane = %v, want paneDetail (Esc must still leave the pane, note or not)", after.pane)
	}
}

// TestRegisterKeyPaneEscWithoutPendingBeatSurfacesNoNote is the negative
// control: the common case (no registration beat in flight — the plan
// never arrived yet, or resolved to a non-Ready state) must not grow a
// spurious note every time the user leaves the pane.
func TestRegisterKeyPaneEscWithoutPendingBeatSurfacesNoNote(t *testing.T) {
	s := Seed()
	m := newIdentitiesModel(stubBackend{}, s)
	sel, ok := m.selectedIdentity(s)
	if !ok {
		t.Fatal("seed has no selected identity")
	}
	m, _ = m.openRegisterKey(sel)

	escRes := m.handleRegisterKeyKey(pressKey("esc"), s)
	if escRes.note != "" {
		t.Errorf("note = %q, want empty when no registration beat was in flight", escRes.note)
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

// TestStaleUploadRunMsgFromDifferentIdentityIsDiscardedInKeyCeremony is the
// WR-01 regression (review iteration 3): a stale UploadRunMsg from a
// PREVIOUS identity's still-in-flight upload beat must never be consumed as
// the CURRENT identity's ceremony result. openKeyCeremony resets
// keyCeremonyUploadPending when a new ceremony opens, but it cannot cancel
// the in-flight command from the OLD ceremony -- its reply can still arrive
// later, while keyCeremonyUploadPending is true again for a DIFFERENT
// identity. Before the fix, the pane+pending guard alone could not tell the
// two apart and would dispatch RotateDeleteOffer on the wrong identity's
// evidence (the same defect class CR-02 closed, reached by a different
// route).
func TestStaleUploadRunMsgFromDifferentIdentityIsDiscardedInKeyCeremony(t *testing.T) {
	s := Seed()
	m := newIdentitiesModel(stubBackend{}, s)
	m.pane = paneKeyCeremony
	m.keyCeremonyMode = KeyCeremonyModeRotate
	m.keyCeremonyUploadPending = true
	m.selected = "work" // identity B: the ceremony currently in flight

	// Identity A's ("personal") late reply arrives while B is pending.
	res := m.handleMsg(UploadRunMsg{Name: "personal", View: UploadRunView{
		Rows: []UploadResultRow{{Outcome: UploadRowUploaded}},
	}}, s)
	next := res.model.(identitiesModel)

	if res.cmd != nil {
		t.Fatal("a stale reply from a different identity must not dispatch RotateDeleteOffer")
	}
	if !next.keyCeremonyUploadPending {
		t.Fatal("a stale reply must not clear keyCeremonyUploadPending -- B's real reply is still expected")
	}
	if uploadRunHasContent(next.keyCeremonyUploadRun) {
		t.Fatal("a stale reply from a different identity must not populate keyCeremonyUploadRun")
	}
}

// TestStaleUploadRunMsgFromDifferentIdentityIsDiscardedInRegisterKeyPane is
// WR-01's second consequence: pressing "u" on identity A then B must never
// let A's late upload result be displayed as B's.
func TestStaleUploadRunMsgFromDifferentIdentityIsDiscardedInRegisterKeyPane(t *testing.T) {
	s := Seed()
	m := newIdentitiesModel(stubBackend{}, s)
	m.pane = paneRegisterKey
	m.registerKeyPending = true
	m.registerKeyName = "work"

	res := m.handleMsg(UploadRunMsg{Name: "personal", View: UploadRunView{
		Rows: []UploadResultRow{{Outcome: UploadRowUploaded}},
	}}, s)
	next := res.model.(identitiesModel)

	if !next.registerKeyPending {
		t.Fatal("a stale reply from a different identity must not clear registerKeyPending -- work's real reply is still expected")
	}
	if uploadRunHasContent(next.registerKeyRun) {
		t.Fatal("a stale reply from a different identity must not populate registerKeyRun")
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
// D1 (260831-3a9): the register-key manual-fallback modal must not be a
// dead end -- "c" copies the public key, gated by the SAME predicate that
// drives the footer hint (registerKeyCopyable).
// ---------------------------------------------------------------------------

// registerKeyAtManualFallback opens the register-key pane and resolves the
// plan to the pre-run manual-fallback state (D1 Tests 1/2/3/6).
func registerKeyAtManualFallback(t *testing.T, b Backend) (identitiesModel, DemoState, DemoIdentity) {
	t.Helper()
	s := Seed()
	m := newIdentitiesModel(b, s)
	sel, ok := m.selectedIdentity(s)
	if !ok {
		t.Fatal("seed has no selected identity")
	}
	m, _ = m.openRegisterKey(sel)
	res := m.handleMsg(RegisterKeyPlanMsg{
		Name: sel.Name,
		View: UploadEligibilityView{State: UploadEligibilityUnauth, ProviderName: "GitHub", Hostname: "github.com"},
	}, s)
	next, ok := res.model.(identitiesModel)
	if !ok {
		t.Fatalf("handleMsg returned %T, want identitiesModel", res.model)
	}
	return next, s, sel
}

// registerKeyAtPostRunManualFallback resolves a Ready plan through to a
// completed run whose OWN result carries a manual-fallback block (D1 Test 4).
func registerKeyAtPostRunManualFallback(t *testing.T, b Backend) (identitiesModel, DemoState, DemoIdentity) {
	t.Helper()
	s := Seed()
	m := newIdentitiesModel(b, s)
	sel, ok := m.selectedIdentity(s)
	if !ok {
		t.Fatal("seed has no selected identity")
	}
	m, _ = m.openRegisterKey(sel)
	res := m.handleMsg(RegisterKeyPlanMsg{
		Name: sel.Name,
		View: UploadEligibilityView{State: UploadEligibilityReady, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com"},
	}, s)
	m, ok = res.model.(identitiesModel)
	if !ok {
		t.Fatalf("handleMsg returned %T, want identitiesModel", res.model)
	}
	res = m.handleMsg(UploadRunMsg{Name: sel.Name, View: UploadRunView{
		ManualFallback: "gh ssh-key add ~/.ssh/id_ed25519_personal.pub --title 'gitid: personal'",
	}}, s)
	next, ok := res.model.(identitiesModel)
	if !ok {
		t.Fatalf("handleMsg returned %T, want identitiesModel", res.model)
	}
	return next, s, sel
}

// registerKeyAtSuccessfulRun resolves a Ready plan through to a completed,
// successful run carrying no manual fallback -- D1 Test 5's third negative
// state (there is nothing to paste, so "c" must be inert).
func registerKeyAtSuccessfulRun(t *testing.T, b Backend) (identitiesModel, DemoState, DemoIdentity) {
	t.Helper()
	s := Seed()
	m := newIdentitiesModel(b, s)
	sel, ok := m.selectedIdentity(s)
	if !ok {
		t.Fatal("seed has no selected identity")
	}
	m, _ = m.openRegisterKey(sel)
	res := m.handleMsg(RegisterKeyPlanMsg{
		Name: sel.Name,
		View: UploadEligibilityView{State: UploadEligibilityReady, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com"},
	}, s)
	m, ok = res.model.(identitiesModel)
	if !ok {
		t.Fatalf("handleMsg returned %T, want identitiesModel", res.model)
	}
	res = m.handleMsg(UploadRunMsg{Name: sel.Name, View: UploadRunView{Rows: []UploadResultRow{
		{Label: "Authentication", Command: "gh ssh-key add", Outcome: UploadRowUploaded},
	}}}, s)
	next, ok := res.model.(identitiesModel)
	if !ok {
		t.Fatalf("handleMsg returned %T, want identitiesModel", res.model)
	}
	return next, s, sel
}

// registerKeyAtProbeError resolves the register-key pane to the fail-closed
// probe-error state (D1 Test 5's second negative state).
func registerKeyAtProbeError(t *testing.T, b Backend) (identitiesModel, DemoState, DemoIdentity) {
	t.Helper()
	s := Seed()
	m := newIdentitiesModel(b, s)
	sel, ok := m.selectedIdentity(s)
	if !ok {
		t.Fatal("seed has no selected identity")
	}
	m, _ = m.openRegisterKey(sel)
	res := m.handleMsg(RegisterKeyPlanMsg{Name: sel.Name, Err: errors.New("probe failed")}, s)
	next, ok := res.model.(identitiesModel)
	if !ok {
		t.Fatalf("handleMsg returned %T, want identitiesModel", res.model)
	}
	return next, s, sel
}

// registerKeyNotYetLoaded is the freshly-opened pane, before the async plan
// has resolved at all (D1 Test 5's first negative state).
func registerKeyNotYetLoaded(t *testing.T, b Backend) (identitiesModel, DemoState, DemoIdentity) {
	t.Helper()
	s := Seed()
	m := newIdentitiesModel(b, s)
	sel, ok := m.selectedIdentity(s)
	if !ok {
		t.Fatal("seed has no selected identity")
	}
	m, _ = m.openRegisterKey(sel)
	return m, s, sel
}

// TestRegisterKeyCopyActionOfferedInManualFallback is D1 Test 1: the
// pre-run manual-fallback state (the dead end this fix closes) must offer
// a "c copy public key" footer action.
func TestRegisterKeyCopyActionOfferedInManualFallback(t *testing.T) {
	m, s, sel := registerKeyAtManualFallback(t, stubBackend{})
	rv := m.view(s, minFrameWidth, minFrameHeight)
	found := false
	for _, a := range rv.actions {
		if a.Key == "c" && a.Label == "copy public key" {
			found = true
		}
	}
	if !found {
		t.Fatalf("actions = %v, want a {c, copy public key} action in the manual-fallback state (identity %s)", rv.actions, sel.Name)
	}
}

// TestRegisterKeyCCopiesPublicKeyFromManualFallback is D1 Test 2: pressing
// "c" from the manual-fallback state calls Backend.CopyPublicKey exactly
// once and surfaces the backend's returned note.
func TestRegisterKeyCCopiesPublicKeyFromManualFallback(t *testing.T) {
	rec := &recordingCopyBackend{}
	m, s, _ := registerKeyAtManualFallback(t, rec)
	res := m.handleRegisterKeyKey(pressKey("c"), s)
	if rec.copiedPath == "" {
		t.Fatal("CopyPublicKey was not called")
	}
	if res.note == "" {
		t.Error("keyResult.note must surface the backend's returned note")
	}
}

// TestRegisterKeyCCopySeamReceivesOnlyThePubPath is D1 Test 3 (security):
// the seam must receive the .pub path, never the private key path, mirroring
// TestCopyPubSeamReceivesOnlyThePubPath's proof for the create wizard.
func TestRegisterKeyCCopySeamReceivesOnlyThePubPath(t *testing.T) {
	rec := &recordingCopyBackend{}
	m, s, _ := registerKeyAtManualFallback(t, rec)
	_ = m.handleRegisterKeyKey(pressKey("c"), s)
	want := "~/.ssh/id_ed25519_personal.pub"
	if rec.copiedPath != want {
		t.Errorf("CopyPublicKey called with %q, want %q (the .pub path, never the private key)", rec.copiedPath, want)
	}
}

// TestRegisterKeyCopyActionOfferedAfterPostRunManualFallback is D1 Test 4:
// a completed run whose own result carries a manual-fallback block must
// also offer and honor "c".
func TestRegisterKeyCopyActionOfferedAfterPostRunManualFallback(t *testing.T) {
	rec := &recordingCopyBackend{}
	m, s, _ := registerKeyAtPostRunManualFallback(t, rec)
	rv := m.view(s, minFrameWidth, minFrameHeight)
	found := false
	for _, a := range rv.actions {
		if a.Key == "c" {
			found = true
		}
	}
	if !found {
		t.Fatalf("actions = %v, want a c action after a run whose result carries ManualFallback", rv.actions)
	}
	res := m.handleRegisterKeyKey(pressKey("c"), s)
	if rec.copiedPath == "" {
		t.Fatal("c must copy the public key from the post-run manual-fallback state too")
	}
	if res.note == "" {
		t.Error("keyResult.note must surface the backend's returned note")
	}
}

// TestRegisterKeyCopyActionAbsentInNegativeStates is D1 Test 5: in the
// not-yet-loaded, probe-error, and successful-run states there is nothing
// to paste, so "c" must offer no footer action and be inert.
func TestRegisterKeyCopyActionAbsentInNegativeStates(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, b Backend) (identitiesModel, DemoState, DemoIdentity)
	}{
		{"not-yet-loaded", registerKeyNotYetLoaded},
		{"probe-error", registerKeyAtProbeError},
		{"successful-run", registerKeyAtSuccessfulRun},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recordingCopyBackend{}
			m, s, _ := tt.setup(t, rec)
			rv := m.view(s, minFrameWidth, minFrameHeight)
			for _, a := range rv.actions {
				if a.Key == "c" {
					t.Errorf("actions = %v, want no c action in state %s", rv.actions, tt.name)
				}
			}
			_ = m.handleRegisterKeyKey(pressKey("c"), s)
			if rec.copiedPath != "" {
				t.Errorf("c must be inert in state %s, but CopyPublicKey was called with %q", tt.name, rec.copiedPath)
			}
		})
	}
}

// TestRegisterKeyCopyHintAndBindingShareOnePredicate is D1 Test 6, the
// anti-drift proof: across every register-key state, "the c action is
// offered in the footer" and "the c keystroke is handled non-inertly" must
// always agree -- a hint without a binding (or the reverse) is the exact
// defect class this fix closes.
func TestRegisterKeyCopyHintAndBindingShareOnePredicate(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, b Backend) (identitiesModel, DemoState, DemoIdentity)
	}{
		{"not-yet-loaded", registerKeyNotYetLoaded},
		{"probe-error", registerKeyAtProbeError},
		{"manual-fallback", registerKeyAtManualFallback},
		{"post-run-manual-fallback", registerKeyAtPostRunManualFallback},
		{"successful-run", registerKeyAtSuccessfulRun},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recordingCopyBackend{}
			m, s, _ := tt.setup(t, rec)
			rv := m.view(s, minFrameWidth, minFrameHeight)
			hinted := false
			for _, a := range rv.actions {
				if a.Key == "c" {
					hinted = true
				}
			}
			_ = m.handleRegisterKeyKey(pressKey("c"), s)
			bound := rec.copiedPath != ""
			if hinted != bound {
				t.Errorf("state %s: hint offered=%v, binding fired=%v -- they must agree", tt.name, hinted, bound)
			}
		})
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
			model, _ := a.Update(UploadRunMsg{Name: "personal", View: view})
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

// TestRotateDeleteOfferNotDispatchedOnFailedUpload is the CR-02 (iteration
// 2) regression: the D-04 delete offer must never be dispatched when the
// new key's registration did not fully succeed. Accepting the offer in
// that state would remove the OLD key — the account's only remaining
// working credential, since a failed/partial registration leaves the
// provider inventory holding only the old key under the shared D-07 title,
// which OldKeyCandidates then resolves as a clean, unambiguous — and
// wrong — deletion target.
func TestRotateDeleteOfferNotDispatchedOnFailedUpload(t *testing.T) {
	unprovenViews := []UploadRunView{
		{Rows: []UploadResultRow{{Label: "Authentication", Outcome: UploadRowFailed, Reason: "boom"}}},
		{Rows: []UploadResultRow{
			{Label: "Authentication", Outcome: UploadRowUploaded},
			{Label: "Signing", Outcome: UploadRowFailed, Reason: "scope"},
		}},
		{InventoryDegraded: true, ProviderName: "GitHub"},
		{Skipped: true},
		{}, // no rows, no AlreadyComplete: zero evidence of registration
	}
	for i, view := range unprovenViews {
		t.Run(fmt.Sprintf("view-%d", i), func(t *testing.T) {
			a := openKeyCeremonyAtReview(t, stubBackend{})
			a, uploadCmd := confirmKeyCeremony(t, a)
			if uploadCmd == nil {
				t.Fatal("setup: a successful rotate commit must dispatch the upload beat")
			}
			model, offerCmd := a.Update(UploadRunMsg{Name: "personal", View: view})
			if offerCmd != nil {
				t.Fatalf("view %d: a failed/unproven upload must never dispatch the delete-offer probe", i)
			}
			m := identModel(t, model.(App))
			if m.rotateDeleteOfferPending {
				t.Fatalf("view %d: rotateDeleteOfferPending must stay false when the upload did not succeed", i)
			}
		})
	}
}

// TestRotateDeleteOfferDispatchedOnSuccessfulUpload proves the CR-02 gate is
// not overzealous: a genuinely successful upload (every row Uploaded/
// AlreadyPresent, or AlreadyComplete) must still dispatch the offer exactly
// as before.
func TestRotateDeleteOfferDispatchedOnSuccessfulUpload(t *testing.T) {
	provenViews := []UploadRunView{
		{Rows: []UploadResultRow{{Label: "Authentication", Outcome: UploadRowUploaded}}},
		{Rows: []UploadResultRow{
			{Label: "Authentication", Outcome: UploadRowUploaded},
			{Label: "Signing", Outcome: UploadRowAlreadyPresent},
		}},
		{AlreadyComplete: true, ProviderName: "GitHub"},
	}
	for i, view := range provenViews {
		t.Run(fmt.Sprintf("view-%d", i), func(t *testing.T) {
			a := openKeyCeremonyAtReview(t, stubBackend{})
			a, uploadCmd := confirmKeyCeremony(t, a)
			if uploadCmd == nil {
				t.Fatal("setup: a successful rotate commit must dispatch the upload beat")
			}
			model, offerCmd := a.Update(UploadRunMsg{Name: "personal", View: view})
			if offerCmd == nil {
				t.Fatalf("view %d: a successful upload must still dispatch the delete-offer probe", i)
			}
			m := identModel(t, model.(App))
			if !m.rotateDeleteOfferPending {
				t.Fatalf("view %d: rotateDeleteOfferPending must be set for a successful upload", i)
			}
		})
	}
}

func TestKeyCeremonyUploadUsesTheSharedRenderer(t *testing.T) {
	view := UploadRunView{Rows: []UploadResultRow{{Label: "Authentication", Command: "gh ssh-key add x.pub", Outcome: UploadRowUploaded}}}
	a := openKeyCeremonyAtReview(t, stubBackend{})
	a, _ = confirmKeyCeremony(t, a)
	model, _ := a.Update(UploadRunMsg{Name: "personal", View: view})
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

// ---------------------------------------------------------------------------
// Task 3: D-04, the interactive old-key delete offer (09-06-PLAN.md).
// ---------------------------------------------------------------------------

// atRotateDeleteOffer drives a full rotate through commit success, the
// upload beat, and the delete-offer probe, landing with the offer resolved
// Available and the choice row at its default focus.
func atRotateDeleteOffer(t *testing.T, b Backend) App {
	t.Helper()
	a := openKeyCeremonyAtReview(t, b)
	a, uploadCmd := confirmKeyCeremony(t, a)
	if uploadCmd == nil {
		t.Fatal("setup: a successful rotate commit must dispatch the upload beat")
	}
	model, offerCmd := a.Update(uploadCmd())
	a = model.(App)
	if offerCmd == nil {
		t.Fatal("setup: the upload beat's completion must dispatch the delete-offer probe (rotate mode)")
	}
	model, _ = a.Update(offerCmd())
	a = model.(App)
	if m := identModel(t, a); !m.rotateDeleteOffer.Available {
		t.Fatalf("setup: offer must be available, got %+v", m.rotateDeleteOffer)
	}
	return a
}

// TestRotateDeleteCommitMsgStaleReplyDiscarded is the WR-01 regression
// (review iteration 5): RotateDeleteCommitMsg carried no Name field at all
// before this fix, so the consumer guarded on pane + pending only — a reply
// belonging to a DIFFERENT identity's in-flight delete would be consumed
// here, rewriting rotateDeleteConfirmedID (the ID set the NEXT destructive
// retry sends) with a value that names the WRONG provider account's key. A
// stale-named reply must now be discarded entirely: rotateDeleteCommitPending
// stays true, rotateDeleteResult stays unset, and rotateDeleteConfirmedID is
// left exactly as the genuine dispatch set it.
func TestRotateDeleteCommitMsgStaleReplyDiscarded(t *testing.T) {
	a := atRotateDeleteOffer(t, stubBackend{})
	a = pressSeq(t, a, "down") // move to the delete option
	a, cmd := press(t, a, "enter")
	if cmd == nil {
		t.Fatal("setup: choosing delete must dispatch CommitRotateDeleteOldKey")
	}
	before := identModel(t, a)
	if !before.rotateDeleteCommitPending {
		t.Fatal("setup: choosing delete must set rotateDeleteCommitPending")
	}
	if before.rotateDeleteConfirmedID == "" {
		t.Fatal("setup: choosing delete must retain the confirmed ID before dispatch (R12)")
	}

	// A reply naming a DIFFERENT identity than the one currently selected —
	// exactly what a stale async reply from a PREVIOUS ceremony (the user
	// having since navigated to a different identity) looks like.
	stale := RotateDeleteCommitMsg{Name: "work", Err: "network error", RemainingKeyID: "999-from-a-different-identity"}
	model, _ := a.Update(stale)
	after, ok := model.(App)
	if !ok {
		t.Fatalf("Update(stale RotateDeleteCommitMsg) returned %T, want App", model)
	}
	m := identModel(t, after)
	if !m.rotateDeleteCommitPending {
		t.Fatal("a stale-named reply must not clear rotateDeleteCommitPending")
	}
	if m.rotateDeleteResult != "" {
		t.Fatalf("a stale-named reply must not set rotateDeleteResult, got %q", m.rotateDeleteResult)
	}
	if m.rotateDeleteConfirmedID != before.rotateDeleteConfirmedID {
		t.Fatalf("a stale-named reply must not rewrite rotateDeleteConfirmedID: got %q, want unchanged %q", m.rotateDeleteConfirmedID, before.rotateDeleteConfirmedID)
	}
}

func TestRotateDeleteOfferDefaultsToLeave(t *testing.T) {
	a := atRotateDeleteOffer(t, stubBackend{})
	if m := identModel(t, a); m.rotateDeleteChoiceFocus != 0 {
		t.Fatalf("rotateDeleteChoiceFocus = %d, want 0 (leave)", m.rotateDeleteChoiceFocus)
	}
}

func TestRotateDeleteOfferEnterFromDefaultDoesNotDelete(t *testing.T) {
	var calls []string
	sb := stubBackend{rotateDeleteCalls: &calls}
	a := atRotateDeleteOffer(t, sb)
	a = pressAndRun(t, a, "enter")
	if len(calls) != 0 {
		t.Fatalf("Enter from the default (leave) focus must not delete, got calls=%v", calls)
	}
	m := identModel(t, a)
	if !m.rotateDeleteResolved {
		t.Fatal("choosing leave must resolve the offer")
	}
	if !strings.Contains(m.rotateDeleteResult, "Left in place") {
		t.Fatalf("rotateDeleteResult = %q, want the left-in-place message", m.rotateDeleteResult)
	}
}

// TestRotateDeleteOfferEscLeavesWithoutDeleting is the WR-07 regression:
// before this fix, every key but arrows/tab/enter (including esc) hit the
// pane's trailing `handled: true` and was silently swallowed — the rotate
// is already committed by the time this offer renders, so a user who did
// not want to answer the destructive question had no way out at all. Esc
// must resolve to the SAME non-destructive "leave it" answer the default
// choice row does, from EITHER focus position, and must never dispatch a
// delete.
func TestRotateDeleteOfferEscLeavesWithoutDeleting(t *testing.T) {
	var calls []string
	sb := stubBackend{rotateDeleteCalls: &calls}
	a := atRotateDeleteOffer(t, sb)
	a = pressSeq(t, a, "down") // move to the delete option first
	if m := identModel(t, a); m.rotateDeleteChoiceFocus != 1 {
		t.Fatalf("setup: focus = %d, want 1 (delete)", m.rotateDeleteChoiceFocus)
	}
	a = pressAndRun(t, a, "esc")
	if len(calls) != 0 {
		t.Fatalf("esc must never dispatch a delete, got calls=%v", calls)
	}
	m := identModel(t, a)
	if !m.rotateDeleteResolved {
		t.Fatal("esc must resolve the offer (a way out of the pane)")
	}
	if !strings.Contains(m.rotateDeleteResult, "Left in place") {
		t.Fatalf("rotateDeleteResult = %q, want the left-in-place message", m.rotateDeleteResult)
	}
}

func TestRotateDeleteOfferDeleteRequiresAnExplicitMove(t *testing.T) {
	var calls []string
	sb := stubBackend{rotateDeleteCalls: &calls}
	a := atRotateDeleteOffer(t, sb)
	a = pressSeq(t, a, "down")
	if m := identModel(t, a); m.rotateDeleteChoiceFocus != 1 {
		t.Fatalf("after one down, focus = %d, want 1 (delete)", m.rotateDeleteChoiceFocus)
	}
	displayedID := identModel(t, a).rotateDeleteOffer.KeyID
	a = pressAndRun(t, a, "enter")
	if len(calls) != 1 {
		t.Fatalf("exactly one delete dispatch expected, got %v", calls)
	}
	if calls[0] != displayedID {
		t.Fatalf("delete dispatched with ID %q, want the displayed ID %q", calls[0], displayedID)
	}
	if m := identModel(t, a); !m.rotateDeleteResolved || !strings.Contains(m.rotateDeleteResult, "removed") {
		t.Fatalf("a successful delete must resolve the offer with the removed message, got resolved=%v result=%q", m.rotateDeleteResolved, m.rotateDeleteResult)
	}
}

func TestRotateDeleteOfferAbsentAfterRepair(t *testing.T) {
	var calls []string
	sb := stubBackend{rotateDeleteCalls: &calls}
	// clientB (index 6) is the key-missing fixture row -> repair mode.
	a := openKeyCeremonyAtReviewForIdentity(t, sb, 6)
	a, uploadCmd := confirmKeyCeremony(t, a)
	if uploadCmd == nil {
		t.Fatal("setup: a successful repair commit must still dispatch the upload beat")
	}
	model, offerCmd := a.Update(uploadCmd())
	a = model.(App)
	if offerCmd != nil {
		t.Fatal("repair must never dispatch the delete-offer probe -- there is no old remote key to remove")
	}
	m := identModel(t, a)
	if m.rotateDeleteOffer.Available {
		t.Fatal("repair must never show the delete offer")
	}
	rendered := stripANSI(m.renderKeyCeremony(mustSelected(t, a)))
	if strings.Contains(rendered, "Remove the old key from") {
		t.Fatalf("repair result screen must not render the offer heading:\n%s", rendered)
	}
}

func TestRotateDeleteOfferAbsentWhenInventoryFails(t *testing.T) {
	sb := stubBackend{rotateDeleteOfferFn: func(name string) tea.Cmd {
		return func() tea.Msg {
			return RotateDeleteOfferMsg{Name: name, View: RotateDeleteOfferView{Unavailable: "could not read the existing key inventory"}}
		}
	}}
	a := openKeyCeremonyAtReview(t, sb)
	a, uploadCmd := confirmKeyCeremony(t, a)
	model, offerCmd := a.Update(uploadCmd())
	a = model.(App)
	if offerCmd != nil {
		model, _ = a.Update(offerCmd())
		a = model.(App)
	}
	m := identModel(t, a)
	rendered := stripANSI(m.renderKeyCeremony(mustSelected(t, a)))
	if !strings.Contains(rendered, "The old key stays valid at") {
		t.Fatalf("an unavailable offer must fall back to the existing frozen grace hint:\n%s", rendered)
	}
	if strings.Contains(rendered, "Remove the old key from") {
		t.Fatalf("an unavailable offer must render no offer rows:\n%s", rendered)
	}
}

// TestRotateDeleteOfferIsNotTheTypedConfirmClass asserts, at the source
// level, that the D-04 offer path never constructs a ceremonyConfig with
// Destructive set — the plan's own acceptance criterion, since a rendered
// substring check is too easy to false-positive on legitimate copy (the
// upload beat's own "--type authentication" command text, for instance).
func TestRotateDeleteOfferIsNotTheTypedConfirmClass(t *testing.T) {
	a := atRotateDeleteOffer(t, stubBackend{})
	rendered := stripANSI(identModel(t, a).renderKeyCeremony(mustSelected(t, a)))
	if !strings.Contains(rendered, "[ Leave it") || !strings.Contains(rendered, "[ Delete old key from") {
		t.Fatalf("the offer's two-option choice row must render, got:\n%s", rendered)
	}
	src, err := os.ReadFile("identities.go")
	if err != nil {
		t.Fatalf("reading identities.go: %v", err)
	}
	if strings.Contains(string(src), "Destructive:") &&
		strings.Contains(string(src), "renderRotateDeleteOffer") {
		start := strings.Index(string(src), "func (m identitiesModel) renderRotateDeleteOffer")
		end := strings.Index(string(src)[start:], "\n}\n")
		if start >= 0 && strings.Contains(string(src)[start:start+end], "Destructive:") {
			t.Fatal("renderRotateDeleteOffer must not construct a ceremonyConfig with Destructive set")
		}
	}
}

func TestRotateDeleteOfferResolvesAsynchronously(t *testing.T) {
	a := openKeyCeremonyAtReview(t, stubBackend{})
	a, uploadCmd := confirmKeyCeremony(t, a)
	model, offerCmd := a.Update(uploadCmd())
	a = model.(App)
	before := stripANSI(identModel(t, a).renderKeyCeremony(mustSelected(t, a)))
	if offerCmd == nil {
		t.Fatal("reaching the result screen (post-upload) must dispatch RotateDeleteOffer as a command")
	}
	if strings.Contains(before, "Remove the old key from") {
		t.Fatalf("the pre-message screen must not render any offer rows before RotateDeleteOfferMsg arrives:\n%s", before)
	}
	model, _ = a.Update(offerCmd())
	after := stripANSI(identModel(t, model.(App)).renderKeyCeremony(mustSelected(t, model.(App))))
	if !strings.Contains(after, "Remove the old key from") {
		t.Fatalf("once RotateDeleteOfferMsg arrives, the offer must render:\n%s", after)
	}
}

func TestRotateDeleteFailureRetriesTheSameConfirmedTarget(t *testing.T) {
	var calls []string
	sb := stubBackend{rotateDeleteCalls: &calls, rotateDeleteCommitErr: "network error"}
	a := atRotateDeleteOffer(t, sb)
	a = pressSeq(t, a, "down")
	a = pressAndRun(t, a, "enter")
	if len(calls) != 1 {
		t.Fatalf("first delete attempt: want 1 call, got %v", calls)
	}
	m := identModel(t, a)
	if m.rotateDeleteResolved {
		t.Fatal("a FAILED delete must not resolve the offer -- the choice row stays actionable for a retry")
	}
	if m.rotateDeleteChoiceFocus != 1 {
		t.Fatalf("focus after a failed delete = %d, want 1 (delete) so Enter retries directly", m.rotateDeleteChoiceFocus)
	}
	_ = pressAndRun(t, a, "enter")
	if len(calls) != 2 {
		t.Fatalf("retry: want 2 total calls, got %v", calls)
	}
	if calls[0] != calls[1] {
		t.Fatalf("retry must delete the SAME confirmed ID: first=%q second=%q", calls[0], calls[1])
	}
}

func TestRotateDeleteConfirmedTargetIsDiscardedOnLeavingTheScreen(t *testing.T) {
	var calls []string
	sb := stubBackend{rotateDeleteCalls: &calls, rotateDeleteCommitErr: "network error"}
	a := atRotateDeleteOffer(t, sb)
	a = pressSeq(t, a, "down")
	a = pressAndRun(t, a, "enter")
	if len(calls) != 1 {
		t.Fatalf("setup: want 1 failed call, got %v", calls)
	}
	if m := identModel(t, a); m.rotateDeleteConfirmedID == "" {
		t.Fatal("setup: a failed delete must retain the confirmed ID")
	}
	// Move focus back to "leave" and resolve the offer that way (a user
	// giving up on the retry after a failure), then press Enter once more
	// on the now-free ceremony "Done" control to leave the result screen.
	a = pressSeq(t, a, "up")
	a = pressAndRun(t, a, "enter")
	if m := identModel(t, a); !m.rotateDeleteResolved {
		t.Fatal("setup: choosing leave must resolve the offer even after a prior failed delete")
	}
	a = pressAndRun(t, a, "enter")
	final := identModel(t, a)
	if final.rotateDeleteConfirmedID != "" || final.rotateDeleteConfirmedTitle != "" {
		t.Fatalf("leaving the result screen must discard the retained confirmed pair, got id=%q title=%q", final.rotateDeleteConfirmedID, final.rotateDeleteConfirmedTitle)
	}
}

// TestRotateDeleteOfferFitsTheFrameBudget asserts the rotate result screen,
// WITH the three extra offer rows (heading, body, choice row) present,
// still fits the fixed 100x30 frame's body-row budget.
func TestRotateDeleteOfferFitsTheFrameBudget(t *testing.T) {
	a := atRotateDeleteOffer(t, stubBackend{})
	sv := identModel(t, a).view(Seed(), minFrameWidth, minFrameHeight)
	lines := strings.Count(stripANSI(sv.body), "\n") + 1
	if budget := frameBodyRows(minFrameHeight); lines > budget {
		t.Fatalf("rendered pane is %d lines, want <= frameBodyRows(minFrameHeight) = %d:\n%s", lines, budget, stripANSI(sv.body))
	}
}

// TestKeyCeremonyOverflowBackstopStaysWithinFrameBudget is the WR-01
// regression (review iteration 4): renderKeyCeremony's overflow backstop
// computes `budget := frameBodyRows(minFrameHeight) - rendered - 1` and
// clamps the tail's viewport to exactly `budget` VisibleLines, then appends
// an EXTRA "… N more line(s) hidden" cue row on top -- one row past the
// budget the backstop exists to enforce. The tracked rotate fixtures never
// reach this branch (their tail always fits), so a byte-identical A/B check
// against them cannot see the defect; this test forces genuinely long tail
// content (real "gh ssh-key add ..." command lines routinely run 100+
// columns, wrapping into several physical rows apiece) so tailLines exceeds
// a still-POSITIVE budget.
func TestKeyCeremonyOverflowBackstopStaysWithinFrameBudget(t *testing.T) {
	a := openKeyCeremonyAtReview(t, stubBackend{})
	a, _ = confirmKeyCeremony(t, a)

	longCmd := "/usr/local/bin/gh ssh-key add ~/.ssh/id_ed25519_personal.pub --title 'gitid: personal @ a-genuinely-long-demo-machine-hostname-used-to-force-overflow' --type authentication"
	view := UploadRunView{Rows: []UploadResultRow{
		{Label: "Authentication", Command: longCmd, Outcome: UploadRowUploaded},
		{Label: "Signing", Command: longCmd, Outcome: UploadRowUploaded},
	}}
	model, offerCmd := a.Update(UploadRunMsg{Name: "personal", View: view})
	a = model.(App)
	if offerCmd == nil {
		t.Fatal("setup: a successful rotate upload must dispatch the delete-offer probe")
	}
	manualCmd := strings.Repeat(longCmd+"\n", 5) + longCmd
	model, _ = a.Update(RotateDeleteOfferMsg{Name: "personal", View: RotateDeleteOfferView{
		Available: true, ProviderName: "GitHub", IdentityName: "personal", MachineName: "demo-machine",
		KeyTitle: "gitid: personal @ demo-machine", KeyID: "1", KeyDetail: "ID 1",
		ManualCommand: manualCmd,
	}})
	a = model.(App)

	m := identModel(t, a)
	rendered := stripANSI(m.renderKeyCeremony(mustSelected(t, a)))
	lines := strings.Count(rendered, "\n") + 1
	if budget := frameBodyRows(minFrameHeight); lines > budget {
		t.Fatalf("rendered pane is %d lines, want <= frameBodyRows(minFrameHeight) = %d:\n%s", lines, budget, rendered)
	}
}

// TestKeyCeremonyOverflowBackstopClampsWholePaneWhenReceiptAloneOverflows is
// the WR-03 regression (review iteration 5): renderKeyCeremony's own
// budget := frameBodyRows(minFrameHeight) - rendered - 1 arithmetic is exact
// only while budget >= 3 — below that the maxInt(1, budget-2) floor takes
// over and the combined output can overrun the frame by up to 3 rows, and
// the base case (if tail.Len() == 0 { return body }) returns the receipt
// completely UNBOUNDED, so a long receipt overflows the frame even with no
// upload/offer tail at all. Nothing downstream rescued this: unlike Health,
// Fixer, Global SSH and Global Git, the Identities screen never called
// fitPane on its master-detail body. Force the receipt ALONE (before any
// tail) well past the frame's row budget by injecting long Targets/Backups
// directly into the ceremony's receipt state, then assert the TOP-LEVEL
// view()'s rendered body — the same call path every other screen's overflow
// safety net protects — stays within frameBodyRows(minFrameHeight).
func TestKeyCeremonyOverflowBackstopClampsWholePaneWhenReceiptAloneOverflows(t *testing.T) {
	a := openKeyCeremonyAtReview(t, stubBackend{})
	a, _ = confirmKeyCeremony(t, a)

	m := identModel(t, a)
	if !m.keyCeremony.done {
		t.Fatal("setup: a successful commit must leave the ceremony in its receipt (done) state")
	}
	// receiptListMaxLines caps EACH list at 6 entries, but does not wrap
	// them — a long single-logical-line entry becomes several PHYSICAL rows
	// once the outer pane word-wraps it at the frame's width. 6 long entries
	// per list (12 total) comfortably exceeds frameBodyRows(minFrameHeight).
	var long []string
	for i := 0; i < 6; i++ {
		long = append(long, strings.Repeat("a-very-long-receipt-path-segment/", 6)+fmt.Sprintf("%d", i))
	}
	m.keyCeremony.cfg.Targets = long
	m.keyCeremony.cfg.Backups = long

	sv := m.view(Seed(), minFrameWidth, minFrameHeight)
	lines := strings.Count(stripANSI(sv.body), "\n") + 1
	if budget := frameBodyRows(minFrameHeight); lines > budget {
		t.Fatalf("rendered pane is %d lines, want <= frameBodyRows(minFrameHeight) = %d:\n%s", lines, budget, stripANSI(sv.body))
	}
}

// shellQuoteForTest mirrors internal/uploader's unexported shellQuote
// (internal/tuikit cannot import internal/uploader -- the no-backend-import
// rule) closely enough to build realistic previewLine-shaped command
// strings for extractUploadedTitle's test table below: quote only when the
// argument contains a shell-special character, and escape an embedded
// single quote as quote-backslash-quote-quote.
func shellQuoteForTest(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"'\\$`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// TestExtractUploadedTitle is the WR-03 regression (review iteration 4):
// extractUploadedTitle used to take the text between the FIRST pair of
// single quotes in the rendered command line, which is wrong whenever an
// EARLIER argument is quoted too -- buildArgs places pubPath (and the tool
// path) BEFORE --title/-t, and shellQuote quotes ANY argument containing a
// shell-special character, not just the title. This asserts the fix parses
// by FLAG POSITION (the operand immediately following --title or -t)
// instead, across every shape the review named plus the trailing
// --type/--usage-type flag that follows the title in the real argv (a
// naive whitespace-split of "everything after --title" would swallow that
// trailing flag into the returned title).
func TestExtractUploadedTitle(t *testing.T) {
	title := "gitid: personal @ demo-machine"
	quotedTitle := shellQuoteForTest(title)

	cases := []struct {
		name    string
		command string
		want    string
	}{
		{
			name: "plain gh command, no earlier quoting",
			command: "/usr/local/bin/gh ssh-key add " + shellQuoteForTest("~/.ssh/id_ed25519_personal.pub") +
				" --title " + quotedTitle + " --type authentication",
			want: title,
		},
		{
			name: "plain glab command, -t flag",
			command: "/usr/local/bin/glab ssh-key add " + shellQuoteForTest("~/.ssh/id_ed25519_personal.pub") +
				" -t " + quotedTitle + " --usage-type auth_and_signing",
			want: title,
		},
		{
			// A $HOME containing a space -- a routine macOS/Windows account
			// name -- means shellQuote quotes the pub path TOO, and it
			// appears before --title. A quote-position parser returns the
			// pub path instead of the title; this must still return the
			// title.
			name: "spaced $HOME quotes the pub path before --title",
			command: "/usr/local/bin/gh ssh-key add " + shellQuoteForTest("/Users/John Smith/.ssh/id_ed25519_work.pub") +
				" --title " + quotedTitle + " --type authentication",
			want: title,
		},
		{
			// An identity name containing a single quote makes shellQuote
			// escape it as '\''. A quote-position parser (SplitN on "'")
			// returns the truncated prefix before the FIRST escaped quote;
			// this must return the full, correctly unescaped title.
			name: "identity name with an apostrophe",
			command: "/usr/local/bin/gh ssh-key add " + shellQuoteForTest("~/.ssh/id_ed25519_o'brien.pub") +
				" --title " + shellQuoteForTest("gitid: o'brien @ demo-machine") + " --type authentication",
			want: "gitid: o'brien @ demo-machine",
		},
		{
			// A toolPath containing a space (a Homebrew prefix under a
			// spaced HOME) is ALSO quoted, and previewLine emits it FIRST
			// (before every argv element, including the pub path).
			name: "spaced toolPath quoted before every other argument",
			command: shellQuoteForTest("/Users/John Smith/.local/bin/gh") + " ssh-key add " +
				shellQuoteForTest("~/.ssh/id_ed25519_personal.pub") + " --title " + quotedTitle + " --type authentication",
			want: title,
		},
		{
			name:    "no --title/-t present at all",
			command: "/usr/local/bin/gh ssh-key add ~/.ssh/id_ed25519_personal.pub --type authentication",
			want:    "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractUploadedTitle(c.command); got != c.want {
				t.Errorf("extractUploadedTitle(%q) = %q, want %q", c.command, got, c.want)
			}
		})
	}
}

func mustSelected(t *testing.T, a App) DemoIdentity {
	t.Helper()
	m := identModel(t, a)
	sel, ok := m.selectedIdentity(Seed())
	if !ok {
		t.Fatal("no selected identity")
	}
	return sel
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
