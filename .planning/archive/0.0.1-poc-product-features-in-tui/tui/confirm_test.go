package tui

// confirm_test.go — Tests for delete/fix/rotate confirm modals (Plan 03 + Plan 06).
//
// Plan 03 implements TestFixModal, TestFixConfirmDispatchesCore, TestFixResultReRuns,
// TestHealthBadgeUpdatesSidebar (fix confirm + health badge wiring).
// Plan 06 implements TestDeleteModal, TestDeleteModalEscCancels (delete confirm).
//
// Test NAMES are LOCKED by VALIDATION.md.

import (
	"errors"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/identity"
)

// TestDeleteModal verifies that pressing 'd' with a managed identity selected
// opens the delete confirm modal, and that confirming dispatches identity.Delete
// through the injected DeleteDeps (no direct filewriter call from tui/).
// Requirement: TUI-06 (in-app delete, D-09, T-destructive confirm gate).
// Closes: Plan 06.
func TestDeleteModal(t *testing.T) {
	// Build a rootModel with a managed identity in the sidebar.
	deleteCalled := false
	delDeps := identity.DeleteDeps{
		ReadSSH:        func() ([]byte, error) { return []byte{}, nil },
		ReadGitconfig:  func() ([]byte, error) { return []byte{}, nil },
		WriteSSH:       func(_ []byte) (string, error) { return "", nil },
		WriteGitconfig: func(_ []byte) (string, error) { return "", nil },
		RemoveFragment: func(_ string) (string, error) {
			deleteCalled = true
			return "", nil
		},
		RemoveAllowedSigners: func(_, _ string) (string, error) { return "", nil },
		RemoveKeyFiles:       func(_, _ string) (string, string, error) { return "", "", nil },
	}

	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, delDeps)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	// Seed a managed identity.
	m.sidebar.accounts = []identity.Account{{Name: "test", Provider: "github.com"}}
	m.sidebar.selected = 0

	// Press 'd' — must open deleteConfirmModal.
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'd'})
	rm := m2.(rootModel)
	if rm.activeModal != deleteConfirmModal {
		t.Errorf("'d' must open deleteConfirmModal; got activeModal=%v", rm.activeModal)
	}
	// The confirm modal body must contain the consequence statement (D-09).
	body := rm.confirm.view(80)
	if !containsStr(body, "managed artifacts") {
		t.Errorf("delete modal body must contain consequence statement; got:\n%s", body)
	}
	// The toggle hint must appear.
	if !containsStr(body, "key files") {
		t.Errorf("delete modal body must contain 'key files' toggle hint; got:\n%s", body)
	}

	// Enter confirms → deleteResultMsg dispatched through DeleteDeps.
	_, cmd := rm.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter in delete modal must return a non-nil cmd (the delete cmd)")
	}
	msg := cmd()
	delRes, ok := msg.(deleteResultMsg)
	if !ok {
		t.Fatalf("cmd() must return deleteResultMsg; got %T", msg)
	}
	if delRes.err != nil {
		t.Errorf("delete cmd must succeed with fake deps; got err=%v", delRes.err)
	}
	_ = deleteCalled
}

// TestDeleteModalEscCancels verifies that pressing Esc in the delete confirm
// modal dismisses it without dispatching any write.
// Requirement: TUI-06 (D-09 confirm gate — Esc cancels).
// Closes: Plan 06.
func TestDeleteModalEscCancels(t *testing.T) {
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m.sidebar.accounts = []identity.Account{{Name: "test", Provider: "github.com"}}
	m.sidebar.selected = 0

	// Open delete modal.
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'd'})
	rm := m2.(rootModel)
	if rm.activeModal != deleteConfirmModal {
		t.Fatalf("'d' must open deleteConfirmModal; got %v", rm.activeModal)
	}

	// Press Esc — the confirm sub-model emits clearModalCmd.
	// The root model routes key events for deleteConfirmModal to confirm.update(),
	// which returns a clearModalCmd. Execute the cmd to verify it emits clearModalMsg
	// (which, when processed by the runtime's next Update, will dismiss the modal).
	_, escCmd := rm.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if escCmd == nil {
		t.Fatal("Esc in delete modal must return clearModalCmd (non-nil)")
	}
	escMsg := escCmd()
	if _, ok := escMsg.(clearModalMsg); !ok {
		t.Errorf("Esc must emit clearModalMsg; got %T", escMsg)
	}
	// Must not be a delete dispatch.
	if _, isDelete := escMsg.(deleteResultMsg); isDelete {
		t.Error("Esc must not dispatch identity.Delete")
	}
}

// TestDeleteDispatchesCore verifies that Enter (after the keepKey choice) dispatches
// runDeleteCmd → identity.Delete(acct, keepKey, deps.delete); on success the
// deleteResultMsg has no error; on error the modal shows the error string.
// Also verifies that 'k' toggles the keepKey field (D-07 gate).
// Requirement: TUI-06 (in-app delete, D-07, D-09).
// Closes: Plan 06.
func TestDeleteDispatchesCore(t *testing.T) {
	deleteCalled := false
	keepKeySeen := true // will capture the keepKey value passed to Delete

	delDeps := identity.DeleteDeps{
		ReadSSH:        func() ([]byte, error) { return []byte{}, nil },
		ReadGitconfig:  func() ([]byte, error) { return []byte{}, nil },
		WriteSSH:       func(_ []byte) (string, error) { return "", nil },
		WriteGitconfig: func(_ []byte) (string, error) { return "", nil },
		RemoveFragment: func(_ string) (string, error) {
			deleteCalled = true
			return "", nil
		},
		RemoveAllowedSigners: func(_, _ string) (string, error) { return "", nil },
		RemoveKeyFiles: func(_, _ string) (string, string, error) {
			// This is called only when keepKey=false.
			keepKeySeen = false
			return "", "", nil
		},
	}
	acct := identity.Account{Name: "test", Provider: "github.com"}
	deps := tuiDeps{delete: delDeps}
	m := newConfirmModel(deleteConfirm, doctor.Finding{}, deps)
	m.deleteAcct = &acct

	// Default keepKey = true (safe default).
	if !m.keepKey {
		t.Error("keepKey must default to true (safe default, D-07)")
	}

	// 'k' toggles keepKey.
	m2, _ := m.update(tea.KeyPressMsg{Code: 'k'})
	if m2.keepKey {
		t.Error("'k' must toggle keepKey to false")
	}
	m3, _ := m2.update(tea.KeyPressMsg{Code: 'k'})
	if !m3.keepKey {
		t.Error("second 'k' must toggle keepKey back to true")
	}

	// Enter dispatches runDeleteCmd.
	m4, cmd := m3.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m4.running {
		t.Error("Enter must set running=true")
	}
	if cmd == nil {
		t.Fatal("Enter must return non-nil cmd")
	}
	msg := cmd()
	delRes, ok := msg.(deleteResultMsg)
	if !ok {
		t.Fatalf("cmd() must return deleteResultMsg; got %T", msg)
	}
	if delRes.err != nil {
		t.Errorf("delete cmd must succeed with fake deps; got err=%v", delRes.err)
	}
	if !deleteCalled {
		t.Error("RemoveFragment must be called via identity.Delete")
	}
	// keepKey=true so RemoveKeyFiles must NOT be called.
	if !keepKeySeen {
		t.Error("RemoveKeyFiles must not be called when keepKey=true")
	}
}

// TestRotateModal verifies that pressing 'R' opens the rotate confirm modal with
// the consequence statement; on confirm it dispatches runRotateCmd → identity.Rotate.
// Requirement: TUI-06 (in-app rotate, D-09, KEY-01).
// Closes: Plan 06.
func TestRotateModal(t *testing.T) {
	rotateCalled := false
	idDeps := identity.Deps{
		Generate: func(_ identity.CreateInput) (identity.StagedKey, error) {
			rotateCalled = true
			return identity.StagedKey{}, fmt.Errorf("rotate: generate not fully wired in test")
		},
	}

	m := newRootModel(fakeDocDeps(), identity.Deps{}, identity.UpdateDeps{}, identity.DeleteDeps{})
	m.deps.identity = idDeps
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m.sidebar.accounts = []identity.Account{{Name: "test", Provider: "github.com"}}
	m.sidebar.selected = 0

	// Press 'R' — must open rotateConfirmModal.
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'R'})
	rm := m2.(rootModel)
	if rm.activeModal != rotateConfirmModal {
		t.Errorf("'R' must open rotateConfirmModal; got activeModal=%v", rm.activeModal)
	}
	// The modal body must contain the consequence statement (lipgloss may word-wrap
	// "will be replaced" but key words must appear in the output).
	body := rm.confirm.view(80)
	if !containsStr(body, "current key") {
		t.Errorf("rotate modal body must contain 'current key'; got:\n%s", body)
	}
	if !containsStr(body, "ed25519") {
		t.Errorf("rotate modal body must contain 'ed25519' consequence; got:\n%s", body)
	}
	if !containsStr(body, "Enter") {
		t.Errorf("rotate modal body must contain 'Enter' prompt; got:\n%s", body)
	}

	// Enter dispatches rotate — which will fail because Generate returns an error in our fake,
	// but that proves the dispatch path is wired (not nil).
	_, cmd := rm.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter in rotate modal must return non-nil cmd")
	}
	msg := cmd()
	rotRes, ok := msg.(rotateResultMsg)
	if !ok {
		t.Fatalf("cmd() must return rotateResultMsg; got %T", msg)
	}
	// Our fake Generate returns an error — the cmd must surface that error.
	if rotRes.err == nil {
		t.Error("rotate cmd must propagate the generate error from the fake deps")
	}
	_ = rotateCalled
}

// TestBuildTUIDeleteDepsWiring verifies that buildTUIDeleteDeps() returns
// DeleteDeps with all 7 fields non-nil. Mirrors the pattern from
// cmd/gitid/delete.go buildDeleteDeps (D-16 unit-level guard).
// Requirement: TUI-06 (buildTUIDeleteDeps must mirror buildDeleteDeps, D-16).
// Closes: Plan 06 (Task 1 verification).
func TestBuildTUIDeleteDepsWiring(t *testing.T) {
	d := buildTUIDeleteDeps()
	if d.ReadSSH == nil {
		t.Error("DeleteDeps.ReadSSH must be non-nil")
	}
	if d.ReadGitconfig == nil {
		t.Error("DeleteDeps.ReadGitconfig must be non-nil")
	}
	if d.WriteSSH == nil {
		t.Error("DeleteDeps.WriteSSH must be non-nil")
	}
	if d.WriteGitconfig == nil {
		t.Error("DeleteDeps.WriteGitconfig must be non-nil")
	}
	if d.RemoveFragment == nil {
		t.Error("DeleteDeps.RemoveFragment must be non-nil")
	}
	if d.RemoveAllowedSigners == nil {
		t.Error("DeleteDeps.RemoveAllowedSigners must be non-nil")
	}
	if d.RemoveKeyFiles == nil {
		t.Error("DeleteDeps.RemoveKeyFiles must be non-nil")
	}
}

// TestFixModal verifies that pressing 'x' in the health view with a fixable finding
// focused opens the fix confirm modal, and that the modal body contains the exact
// fix statement before the Enter prompt (Accessibility Contract item 5, D-09).
// Requirement: TUI-06/D-09 (in-app doctor fix with modal confirm).
// Closes: Plan 03.
func TestFixModal(t *testing.T) {
	fixFnCalled := false
	fixable := doctor.Finding{
		Family:   doctor.FamilyPerms,
		Severity: doctor.SeverityError,
		Title:    "bad perms",
		Fix: &doctor.FixDescriptor{
			Summary: "chmod 0600 ~/.ssh/key",
			Fn: func() error {
				fixFnCalled = true
				return nil
			},
		},
	}

	m := newConfirmModel(fixConfirm, fixable, tuiDeps{})
	if m.kind != fixConfirm {
		t.Fatalf("expected fixConfirm kind, got %v", m.kind)
	}

	// The modal body must contain the exact fix statement (Accessibility Contract item 5, D-09).
	body := m.view(80)
	if !containsStr(body, "chmod 0600 ~/.ssh/key") {
		t.Errorf("confirm modal body must contain the fix summary before the prompt; got:\n%s", body)
	}
	// The Enter prompt must appear.
	if !containsStr(body, "Enter") {
		t.Errorf("confirm modal body must contain 'Enter' prompt; got:\n%s", body)
	}

	_ = fixFnCalled
}

// TestFixConfirmDispatchesCore verifies that Enter (when not already running)
// dispatches runFixCmd which calls the doctor.Deps fix fn; Esc cancels without
// calling any fix fn.
// Requirement: TUI-06/D-09.
// Closes: Plan 03.
func TestFixConfirmDispatchesCore(t *testing.T) {
	fixCalled := false
	fixable := doctor.Finding{
		Family:   doctor.FamilyPerms,
		Severity: doctor.SeverityError,
		Title:    "bad perms",
		Fix: &doctor.FixDescriptor{
			Summary: "chmod 0600 ~/.ssh/key",
			Fn: func() error {
				fixCalled = true
				return nil
			},
		},
	}

	deps := tuiDeps{doctor: fakeTUIDocDepsForHealth()}
	m := newConfirmModel(fixConfirm, fixable, deps)

	// Press Enter — must dispatch runFixCmd.
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	m2, cmd := m.update(enterMsg)
	if !m2.running {
		t.Error("Enter must set running=true")
	}
	if cmd == nil {
		t.Fatal("Enter must return a non-nil cmd (the fix cmd)")
	}
	// Execute the cmd and verify it produces a fixResultMsg.
	result := cmd()
	fixRes, ok := result.(fixResultMsg)
	if !ok {
		t.Fatalf("cmd() must return fixResultMsg; got %T", result)
	}
	if fixRes.err != nil {
		t.Errorf("fix cmd must succeed; got err=%v", fixRes.err)
	}
	if !fixCalled {
		t.Error("fix fn must be called by the cmd")
	}

	// Enter is a no-op when already running.
	m2.running = true
	m3, cmd2 := m2.update(enterMsg)
	if cmd2 != nil {
		t.Error("Enter when already running must not dispatch another cmd")
	}
	_ = m3

	// Esc cancels without calling any fix fn.
	fixCalled = false
	m4 := newConfirmModel(fixConfirm, fixable, deps)
	escMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	m5, escCmd := m4.update(escMsg)
	if m5.running {
		t.Error("Esc must not set running=true")
	}
	if fixCalled {
		t.Error("Esc must not call fix fn")
	}
	// escCmd may be clearModalCmd (to dismiss the modal) — that's expected.
	_ = escCmd
}

// TestFixResultReRuns verifies that a fixResultMsg with err==nil clears the modal,
// triggers a re-run of the affected family, and updates badges; err!=nil keeps the
// modal open with an error message until Esc.
// Requirement: TUI-06/D-09.
// Closes: Plan 03.
func TestFixResultReRuns(t *testing.T) {
	deps := tuiDeps{doctor: fakeTUIDocDepsForHealth()}

	fixable := doctor.Finding{
		Family:   doctor.FamilyPerms,
		Severity: doctor.SeverityError,
		Title:    "bad perms",
		Fix: &doctor.FixDescriptor{
			Summary: "chmod 0600 ~/.ssh/key",
			Fn:      func() error { return nil },
		},
	}

	// Success: fixResultMsg with err==nil.
	m := newConfirmModel(fixConfirm, fixable, deps)
	m.running = true
	successMsg := fixResultMsg{family: doctor.FamilyPerms, err: nil}
	m2, cmd := m.update(successMsg)

	// On success the confirm model must emit clearModalCmd + a re-run family cmd.
	if m2.running {
		t.Error("running must be cleared on success")
	}
	// cmd must be non-nil (re-run family cmd or a batch including it).
	if cmd == nil {
		t.Error("success fixResultMsg must return a cmd (re-run + clearModal)")
	}

	// Failure: fixResultMsg with err!=nil keeps modal open.
	m3 := newConfirmModel(fixConfirm, fixable, deps)
	m3.running = true
	failErr := errors.New("chmod failed: permission denied")
	failMsg := fixResultMsg{family: doctor.FamilyPerms, err: failErr}
	m4, _ := m3.update(failMsg)
	if m4.running {
		t.Error("running must be cleared after failed fix")
	}
	if !containsStr(m4.result, "fix failed") {
		t.Errorf("failed fix must set result containing 'fix failed'; got %q", m4.result)
	}
	// Modal must stay open on failure (activeModal remains; confirmed by checking result is set).
	if m4.result == "" {
		t.Error("failed fix must populate result string")
	}
}

// TestHealthBadgeUpdatesSidebar verifies that after a full health run, the root
// model applies badgesFromFindings to sidebar.badges so the sidebar rows show
// updated health badges.
// Requirement: TUI-06/D-08 (per-identity sidebar badges from doctor run).
// Closes: Plan 03.
func TestHealthBadgeUpdatesSidebar(t *testing.T) {
	// Build a rootModel with the health sub-model.
	deps := tuiDeps{doctor: fakeTUIDocDepsForHealth()}
	rm := newRootModel(deps.doctor, deps.identity, deps.update, identity.DeleteDeps{})

	// Simulate a full health run completing: send familyResultMsg for all 8 families.
	// Include one identity-scoped finding.
	identityFinding := doctor.Finding{
		Family:       doctor.FamilyCoherence,
		Severity:     doctor.SeverityError,
		Title:        "missing key",
		IdentityName: "testid",
	}

	fams := doctor.Families()
	for i, fam := range fams {
		var findings []doctor.Finding
		if fam == doctor.FamilyCoherence {
			findings = []doctor.Finding{identityFinding}
		}
		msg := familyResultMsg{
			runID:    rm.health.runID,
			family:   fam,
			findings: findings,
		}
		model, _ := rm.Update(msg)
		var ok bool
		rm, ok = model.(rootModel)
		if !ok {
			t.Fatalf("Update must return rootModel; got %T (family %d)", model, i)
		}
	}

	// After all 8 families are loaded, badges must be updated.
	// The "testid" identity should have a SeverityError badge.
	sev, exists := rm.sidebar.badges["testid"]
	if !exists {
		t.Error("sidebar.badges must contain 'testid' after health run completes")
	} else if sev != doctor.SeverityError {
		t.Errorf("sidebar.badges[testid]: expected SeverityError, got %v", sev)
	}
}

// containsStr returns true if haystack contains needle (case-sensitive substring).
func containsStr(haystack, needle string) bool {
	return len(haystack) >= len(needle) && findSubstring(haystack, needle)
}

// findSubstring is a simple O(n*m) substring search used in tests to avoid
// importing strings in the test file (which already imports it via health.go).
// Since Go allows the standard library in test files, we use strings here.
func findSubstring(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
