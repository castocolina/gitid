package tui

// prove_test.go — Tests for the shared wizardProveModel (Plan 04 GREEN).
//
// These tests verify the prove-before-write state machine for structural edits.
// Test names are locked by VALIDATION.md.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// fakeProvePassDeps returns a tuiDeps with identity deps where PreWrite PASS
// and Resolved PASS.
func fakeProvePassDeps() tuiDeps {
	return tuiDeps{
		update: identity.UpdateDeps{
			WriteSSH:            func(_, _, _ string) (string, error) { return "bak", nil },
			WriteGitconfig:      func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "", nil },
			WriteFragment:       func(_, _, _, _ string, _ bool) error { return nil },
			WriteAllowedSigners: func(_, _, _ string) (string, error) { return "", nil },
			RemoveAllowedSigners: func(_, _ string) (string, error) {
				return "", nil
			},
			Resolved: func(_ string) (tester.Result, tester.ResolvedConfig) {
				return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{}
			},
			ReadPub: func(_ string) (string, error) { return "ssh-ed25519 AAA...", nil },
		},
		identity: identity.Deps{
			PreWrite: func(_, _ string, _ int) tester.Result {
				return tester.Result{Outcome: tester.PASS}
			},
		},
	}
}

// fakeProveFailDeps returns a tuiDeps where PreWrite FAILs.
func fakeProveFailDeps() tuiDeps {
	d := fakeProvePassDeps()
	d.identity.PreWrite = func(_, _ string, _ int) tester.Result {
		return tester.Result{Outcome: tester.Failure}
	}
	return d
}

// makeTestProveModel returns a wizardProveModel seeded with test accounts.
func makeTestProveModel(deps tuiDeps) wizardProveModel {
	existing := identity.Account{
		Name:     "personal",
		Alias:    "personal.github.com",
		Hostname: "github.com",
		Port:     22,
		KeyPath:  "~/.ssh/gitid_personal",
	}
	edited := existing
	edited.Alias = "personal2.github.com"
	return newWizardProveModel(existing, edited, false, deps)
}

// TestProveModalPhase1Pass verifies that when PreWrite returns PASS, the prove
// model advances from phase1Running → phase1Done and then dispatches phase 2
// (runResolvedCmd). When Resolved also PASS, confirmActive becomes true.
// Requirement: TUI-05 (prove-before-write, D-07).
// Closes: Plan 04.
func TestProveModalPhase1Pass(t *testing.T) {
	deps := fakeProvePassDeps()
	m := makeTestProveModel(deps)
	m, cmd := m.init()

	if cmd == nil {
		t.Fatal("init() must return a non-nil cmd (preWriteCmd + spinner Tick)")
	}

	// Simulate phase-1 PASS result.
	passResult := preWriteResultMsg{result: tester.Result{Outcome: tester.PASS}}
	m2, cmd2 := m.update(passResult)
	if m2.phase != wizardProvePhase1Done {
		t.Errorf("after phase1 PASS: expected wizardProvePhase1Done, got %v", m2.phase)
	}
	if cmd2 == nil {
		t.Error("after phase1 PASS: must dispatch phase2 cmd (runResolvedCmd)")
	}

	// Simulate phase-2 PASS result.
	passResult2 := resolvedResultMsg{result: tester.Result{Outcome: tester.PASS}, resolved: tester.ResolvedConfig{}}
	m3, _ := m2.update(passResult2)
	if m3.phase != wizardProvePhase2Done {
		t.Errorf("after phase2 PASS: expected wizardProvePhase2Done, got %v", m3.phase)
	}
	if !m3.confirmActive {
		t.Error("after both phases PASS: confirmActive must be true (write gate open)")
	}
}

// TestProveModalRetryPath verifies that when phase1 FAILs, pressing 'r' re-runs
// phase1 with an incremented runID; pressing 's' sets skipConfirmPending; pressing
// 'q' emits clearModalCmd.
// Requirement: TUI-05 (retry/skip/quit loop, D-07, T-05.6-12).
// Closes: Plan 04.
func TestProveModalRetryPath(t *testing.T) {
	deps := fakeProveFailDeps()
	m := makeTestProveModel(deps)
	m, _ = m.init()
	origRunID := m.runID

	// Simulate phase-1 FAIL.
	failResult := preWriteResultMsg{result: tester.Result{Outcome: tester.Failure}}
	m2, _ := m.update(failResult)
	if m2.phase != wizardProvePhase1Failed {
		t.Errorf("phase1 FAIL: expected wizardProvePhase1Failed, got %v", m2.phase)
	}
	if m2.confirmActive {
		t.Error("phase1 FAIL: confirmActive must be false")
	}

	// Press 'r' → re-run phase1 with incremented runID.
	m3, retryCmd := m2.update(tea.KeyPressMsg{Code: 'r'})
	if m3.runID <= origRunID {
		t.Errorf("retry: runID must increase; was %d, now %d", origRunID, m3.runID)
	}
	if m3.phase != wizardProvePhase1Running {
		t.Errorf("retry: expected wizardProvePhase1Running, got %v", m3.phase)
	}
	if retryCmd == nil {
		t.Error("retry: must dispatch a new preWriteCmd")
	}

	// Press 's' → sets skipConfirmPending.
	m4, _ := m2.update(tea.KeyPressMsg{Code: 's'})
	if !m4.skipConfirmPending {
		t.Error("'s' on failed phase must set skipConfirmPending=true")
	}

	// Press 'q' → emits clearModalCmd (dismiss without writing).
	m5, qCmd := m2.update(tea.KeyPressMsg{Code: 'q'})
	_ = m5
	if qCmd == nil {
		t.Error("'q' must emit clearModalCmd")
	}
	// Verify it emits clearModalMsg.
	if qCmd != nil {
		msg := qCmd()
		if _, ok := msg.(clearModalMsg); !ok {
			t.Errorf("'q' cmd must emit clearModalMsg; got %T", msg)
		}
	}
}

// TestProveModalWriteGate verifies the security invariant: identity.Update fires
// ONLY after both phases PASS and the user presses Enter. It must NOT fire when
// Enter is pressed before confirmActive is set.
// Requirement: TUI-05 (write gate, D-07, T-05.6-10, T-write-gate).
// Closes: Plan 04.
func TestProveModalWriteGate(t *testing.T) {
	var writeDispatched bool
	deps := fakeProvePassDeps()
	// Override WriteSSH to track if write was called.
	deps.update.WriteSSH = func(_, _, _ string) (string, error) {
		writeDispatched = true
		return "bak", nil
	}

	m := makeTestProveModel(deps)
	m, _ = m.init()

	// Press Enter before confirmActive is set — write must NOT be dispatched.
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	m2, earlyCmd := m.update(enterMsg)
	_ = m2
	if earlyCmd != nil {
		msg := earlyCmd()
		if _, isWrite := msg.(writeResultMsg); isWrite {
			t.Error("write gate violation: write dispatched before confirmActive (both phases must PASS first)")
		}
	}

	// Simulate both phases passing → confirmActive = true.
	m3, _ := m.update(preWriteResultMsg{result: tester.Result{Outcome: tester.PASS}})
	m4, _ := m3.update(resolvedResultMsg{result: tester.Result{Outcome: tester.PASS}})
	if !m4.confirmActive {
		t.Fatal("confirmActive must be true after both phases pass")
	}

	// Now pressing Enter dispatches the write.
	_, writeCmd := m4.update(enterMsg)
	if writeCmd == nil {
		t.Error("Enter after confirmActive must dispatch a write cmd")
	}
	if writeCmd != nil {
		// Execute the write cmd; it calls identity.Update synchronously in the cmd closure.
		writeCmd() //nolint:errcheck // result checked via writeDispatched flag
		if !writeDispatched {
			t.Error("write gate: Enter after confirmActive must call identity.Update (WriteSSH)")
		}
	}
}
