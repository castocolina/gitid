package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/identity"
)

// fakeTUIDeps returns a tuiDeps with no-op stubs for all write fields.
// Used by form and detail tests to avoid real filesystem operations.
func fakeTUIDeps() tuiDeps {
	return tuiDeps{
		identity: identity.Deps{
			CopyPub: func(_ string) error { return nil },
		},
	}
}

// --- TestFormTabNavigation ---

// TestFormTabNavigation verifies that Tab advances createFormModel.focusIdx
// forward (wrapping), and Shift+Tab retreats. Exactly one input is focused at
// a time.
func TestFormTabNavigation(t *testing.T) {
	m := newCreateFormModel(fakeTUIDeps())

	// Initially focusIdx should be 0.
	if m.focusIdx != 0 {
		t.Fatalf("initial focusIdx: want 0, got %d", m.focusIdx)
	}

	// Tab advances focusIdx.
	tabMsg := tea.KeyPressMsg{Code: tea.KeyTab}
	updated, _ := m.update(tabMsg)
	fm, ok := updated.(createFormModel)
	if !ok {
		t.Fatalf("update(Tab) returned %T; want createFormModel", updated)
	}
	if fm.focusIdx != 1 {
		t.Errorf("after Tab: want focusIdx=1, got %d", fm.focusIdx)
	}

	// Shift+Tab retreats focusIdx back to 0.
	shiftTabMsg := tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	updated2, _ := fm.update(shiftTabMsg)
	fm2, ok := updated2.(createFormModel)
	if !ok {
		t.Fatalf("update(Shift+Tab) returned %T; want createFormModel", updated2)
	}
	if fm2.focusIdx != 0 {
		t.Errorf("after Shift+Tab: want focusIdx=0, got %d", fm2.focusIdx)
	}

	// Tab wraps from last to first.
	last := len(fm2.inputs) - 1
	fm2.focusIdx = last
	// blur all, focus last
	for i := range fm2.inputs {
		fm2.inputs[i].Blur()
	}
	_ = fm2.inputs[last].Focus()

	tabWrapMsg := tea.KeyPressMsg{Code: tea.KeyTab}
	updated3, _ := fm2.update(tabWrapMsg)
	fm3, ok := updated3.(createFormModel)
	if !ok {
		t.Fatalf("update(Tab at last) returned %T; want createFormModel", updated3)
	}
	if fm3.focusIdx != 0 {
		t.Errorf("Tab at last field: want focusIdx=0 (wrap), got %d", fm3.focusIdx)
	}
}

// --- TestCreateFormNameValidation ---

// TestCreateFormNameValidation verifies that a create form with an invalid
// Identity Name ("Bad Name") surfaces a validation error (via identity.ValidateName)
// and does NOT emit a submit/prove transition.
func TestCreateFormNameValidation(t *testing.T) {
	m := newCreateFormModel(fakeTUIDeps())

	// Set invalid name in the first field (focusIdx=0 = name field).
	m.inputs[0].SetValue("Bad Name")
	m.focusIdx = len(m.inputs) - 1 // position at last field to trigger submit

	// Press Enter at last field — should detect name validation error.
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, cmd := m.update(enterMsg)
	fm, ok := updated.(createFormModel)
	if !ok {
		t.Fatalf("update(Enter at last) returned %T; want createFormModel", updated)
	}

	// Should not emit a pushScreenMsg (no prove transition).
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(pushScreenMsg); ok {
			t.Error("createFormModel should NOT push prove screen when name is invalid")
		}
	}

	// Should have a validation error.
	if fm.err == "" {
		t.Error("createFormModel should have a non-empty validation error for invalid name 'Bad Name'")
	}
	if !strings.Contains(fm.err, "name") && !strings.Contains(fm.err, "Name") {
		t.Errorf("validation error should mention name; got: %q", fm.err)
	}
}

// --- TestIdentityDetailEditKey ---

// TestIdentityDetailEditKey verifies that 'e' on identity detail returns a
// pushScreenMsg toward the update form, and 'h' returns a pushScreenMsg toward
// the add-account form.
func TestIdentityDetailEditKey(t *testing.T) {
	acct := identity.Account{
		Name:     "personal",
		Provider: "github.com",
		Hostname: "github.com",
		Port:     22,
		KeyPath:  "~/.ssh/gitid_personal",
	}
	deps := fakeTUIDeps()

	m := newIdentityDetailModel(acct, deps)

	// 'e' should push the update form.
	eMsg := tea.KeyPressMsg{Text: "e"}
	_, cmd := m.update(eMsg)
	if cmd == nil {
		t.Fatal("'e' from identity detail must return a non-nil tea.Cmd")
	}
	msg := cmd()
	push, ok := msg.(pushScreenMsg)
	if !ok {
		t.Fatalf("'e' from identity detail cmd produced %T; want pushScreenMsg", msg)
	}
	if push.next == nil {
		t.Error("pushScreenMsg.next must not be nil for update form")
	}
	if _, ok := push.next.(updateFormModel); !ok {
		t.Errorf("'e' pushScreenMsg.next is %T; want updateFormModel", push.next)
	}

	// 'h' should push the add-account form.
	hMsg := tea.KeyPressMsg{Text: "H"}
	_, cmd2 := m.update(hMsg)
	if cmd2 == nil {
		t.Fatal("'h' from identity detail must return a non-nil tea.Cmd")
	}
	msg2 := cmd2()
	push2, ok2 := msg2.(pushScreenMsg)
	if !ok2 {
		t.Fatalf("'h' from identity detail cmd produced %T; want pushScreenMsg", msg2)
	}
	if push2.next == nil {
		t.Error("pushScreenMsg.next must not be nil for add-account form")
	}
	if _, ok := push2.next.(addAccountFormModel); !ok {
		t.Errorf("'h' pushScreenMsg.next is %T; want addAccountFormModel", push2.next)
	}
}

// --- TestIdentityDetailCopyAction ---

// TestIdentityDetailCopyAction verifies that 'c' on identity detail issues a
// clipboard copy cmd, and the rendered overlay (renderCopyOverlay) contains
// the upload instructions for the provider.
func TestIdentityDetailCopyAction(t *testing.T) {
	acct := identity.Account{
		Name:     "personal",
		Provider: "github.com",
		PubPath:  "/home/user/.ssh/gitid_personal.pub",
	}
	deps := fakeTUIDeps()

	m := newIdentityDetailModel(acct, deps)
	// Set a pub line so renderCopyOverlay has content.
	m.pubLine = "ssh-ed25519 AAAA...test"

	// 'c' should issue a copy cmd.
	cMsg := tea.KeyPressMsg{Text: "c"}
	updated, cmd := m.update(cMsg)
	_ = updated

	if cmd == nil {
		t.Fatal("'c' from identity detail must return a non-nil tea.Cmd (clipboard copy)")
	}

	// The cmd should produce a clipboardResultMsg when called.
	result := cmd()
	if _, ok := result.(clipboardResultMsg); !ok {
		t.Fatalf("'c' cmd produced %T; want clipboardResultMsg", result)
	}

	// After receiving clipboardResultMsg, the overlay should be set.
	idm, ok := updated.(identityDetailModel)
	if !ok {
		t.Fatalf("update('c') returned %T; want identityDetailModel", updated)
	}

	// Simulate the clipboard result arriving.
	cbMsg := clipboardResultMsg{err: nil}
	updated2, _ := idm.update(cbMsg)
	idm2, ok := updated2.(identityDetailModel)
	if !ok {
		t.Fatalf("update(clipboardResultMsg) returned %T; want identityDetailModel", updated2)
	}

	// The overlay should contain upload instructions for github.com.
	overlay := idm2.overlay
	if overlay == "" {
		t.Error("identityDetailModel overlay must be non-empty after successful copy")
	}
	if !strings.Contains(overlay, "github") && !strings.Contains(overlay, "GitHub") {
		t.Errorf("overlay should contain GitHub instructions; got: %q", overlay)
	}
}

// --- TestDetailDeleteHandoff ---

// TestDetailDeleteHandoff verifies that 'd' / 'r' on identity detail show the
// CLI handoff copy and emit no write/push (D-03).
func TestDetailDeleteHandoff(t *testing.T) {
	acct := identity.Account{
		Name:     "personal",
		Provider: "github.com",
	}
	deps := fakeTUIDeps()
	m := newIdentityDetailModel(acct, deps)

	// 'd' should set handoff overlay but NOT push a screen.
	dMsg := tea.KeyPressMsg{Text: "d"}
	updated, cmd := m.update(dMsg)

	// cmd may be nil (no navigation) or non-nil (but must not be pushScreenMsg).
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(pushScreenMsg); ok {
			t.Error("'d' from identity detail must NOT push a screen (D-03 CLI handoff only)")
		}
		if _, ok := msg.(writeResultMsg); ok {
			t.Error("'d' from identity detail must NOT emit a writeResultMsg (D-03)")
		}
	}

	idm, ok := updated.(identityDetailModel)
	if !ok {
		t.Fatalf("update('d') returned %T; want identityDetailModel", updated)
	}
	// handoff message should mention the CLI command.
	if !strings.Contains(idm.overlay, "delete") && !strings.Contains(idm.overlay, "Delete") {
		t.Errorf("'d' handoff overlay should mention delete; got: %q", idm.overlay)
	}
	if !strings.Contains(idm.overlay, "personal") {
		t.Errorf("'d' handoff overlay should contain identity name 'personal'; got: %q", idm.overlay)
	}

	// 'R' should set rotate handoff overlay but NOT push a screen.
	rMsg := tea.KeyPressMsg{Text: "R"}
	updated2, cmd2 := m.update(rMsg)
	if cmd2 != nil {
		msg2 := cmd2()
		if _, ok := msg2.(pushScreenMsg); ok {
			t.Error("'r' from identity detail must NOT push a screen (D-03 CLI handoff only)")
		}
	}

	idm2, ok := updated2.(identityDetailModel)
	if !ok {
		t.Fatalf("update('r') returned %T; want identityDetailModel", updated2)
	}
	if !strings.Contains(idm2.overlay, "rotate") && !strings.Contains(idm2.overlay, "Rotate") {
		t.Errorf("'r' handoff overlay should mention rotate; got: %q", idm2.overlay)
	}
}
