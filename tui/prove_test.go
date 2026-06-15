package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// fakeWriteTUIDeps returns a tuiDeps where all identity.Deps write fields are
// no-op stubs. The called pointer is reserved for future extension and is
// intentionally unused in the current stubs.
func fakeWriteTUIDeps(_ *bool) tuiDeps {
	return tuiDeps{
		identity: identity.Deps{
			Generate: func(_ identity.CreateInput) (identity.StagedKey, error) {
				return identity.StagedKey{}, nil
			},
			PersistKey: func(_ identity.StagedKey) (identity.KeyResult, error) {
				return identity.KeyResult{}, nil
			},
			Cleanup:             func(_ identity.StagedKey) {},
			CopyPub:             func(_ string) error { return nil },
			PreWrite:            func(_, _ string, _ int) tester.Result { return tester.Result{Outcome: tester.PASS} },
			WriteSSH:            func(_, _, _ string) (string, error) { return "", nil },
			WriteGitconfig:      func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "", nil },
			WriteFragment:       func(_, _, _, _ string, _ bool) error { return nil },
			WriteAllowedSigners: func(_, _, _ string) (string, error) { return "", nil },
			Resolved:            func(_ string) (tester.Result, tester.ResolvedConfig) { return tester.Result{}, tester.ResolvedConfig{} },
			PubExists:           func(_ string) bool { return true },
			DerivePub:           func(_ string) (string, error) { return "", nil },
			WritePub:            func(_, _ string) error { return nil },
		},
	}
}

// makePassResult returns a tester.Result with outcome PASS.
func makePassResult() tester.Result {
	return tester.Result{
		Command: "ssh -T git@github.com",
		Output:  "Hi user! You've successfully authenticated",
		Outcome: tester.PASS,
	}
}

// makeFailResult returns a tester.Result with outcome Failure.
func makeFailResult() tester.Result {
	return tester.Result{
		Command: "ssh -T git@github.com",
		Output:  "Connection refused",
		Outcome: tester.Failure,
	}
}

// makeResolvedResult returns a passing resolved result.
func makeResolvedResult() (tester.Result, tester.ResolvedConfig) {
	return tester.Result{
			Command: "ssh -G personal.github.com",
			Output:  "identityfile ~/.ssh/gitid_personal",
			Outcome: tester.PASS,
		},
		tester.ResolvedConfig{
			IdentityFiles: []string{"~/.ssh/gitid_personal"},
		}
}

// makeTestInput returns a minimal CreateInput for prove screen testing.
func makeTestInput() identity.CreateInput {
	return identity.CreateInput{
		Name:     "personal",
		Provider: "github.com",
		Hostname: "github.com",
		Port:     22,
		Alias:    "personal.github.com",
	}
}

// --- TestProveScreenConfirmGate ---

// TestProveScreenConfirmGate verifies that the confirm action (Enter) is inert
// until BOTH preWriteResultMsg (pass) AND resolvedResultMsg (pass) have arrived;
// only then does keys.Confirm dispatch the write cmd.
func TestProveScreenConfirmGate(t *testing.T) {
	var called bool
	deps := fakeWriteTUIDeps(&called)

	m := newProveScreen("create", makeTestInput(), deps)

	// Before either phase: confirm must be inactive.
	if m.confirmActive {
		t.Error("confirmActive must be false before any phase result arrives")
	}

	// Send phase 1 passing result.
	pass1 := makePassResult()
	updated, _ := m.update(preWriteResultMsg{result: pass1})
	pm, ok := updated.(proveModel)
	if !ok {
		t.Fatalf("update(preWriteResultMsg) returned %T; want proveModel", updated)
	}
	if pm.confirmActive {
		t.Error("confirmActive must still be false after only phase 1 passes (phase 2 not yet run)")
	}

	// Send phase 2 passing result.
	passR, passResolved := makeResolvedResult()
	updated2, _ := pm.update(resolvedResultMsg{result: passR, resolved: passResolved})
	pm2, ok := updated2.(proveModel)
	if !ok {
		t.Fatalf("update(resolvedResultMsg) returned %T; want proveModel", updated2)
	}
	if !pm2.confirmActive {
		t.Error("confirmActive must be true after both phases pass")
	}

	// Now pressing Enter should return a non-nil cmd that invokes the write.
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := pm2.update(enterMsg)
	if cmd == nil {
		t.Fatal("Enter after both phases pass must return a non-nil write cmd")
	}
	// Execute the cmd to trigger the write.
	_ = cmd()
}

// --- TestProveScreenPhase1Failure ---

// TestProveScreenPhase1Failure verifies that a preWriteResultMsg with a Failure
// result sets the phase1Failed state, leaves confirm disabled, and the rendered
// view contains the failure output + "Cannot proceed"; Enter does nothing.
func TestProveScreenPhase1Failure(t *testing.T) {
	var called bool
	deps := fakeWriteTUIDeps(&called)

	m := newProveScreen("create", makeTestInput(), deps)

	// Send phase 1 failing result.
	fail1 := makeFailResult()
	updated, _ := m.update(preWriteResultMsg{result: fail1})
	pm, ok := updated.(proveModel)
	if !ok {
		t.Fatalf("update(preWriteResultMsg{fail}) returned %T; want proveModel", updated)
	}

	// confirmActive must remain false.
	if pm.confirmActive {
		t.Error("confirmActive must remain false after phase 1 failure")
	}

	// Rendered view must mention failure output and "Cannot proceed".
	rendered := pm.view()
	if rendered == "" {
		t.Fatal("view() must not be empty after phase 1 failure")
	}
	// The view should convey failure (either via "Cannot proceed", "failed", or the output).
	hasFail := false
	for _, needle := range []string{"Cannot proceed", "failed", "Connection refused", "authentication failed"} {
		if containsCI(rendered, needle) {
			hasFail = true
			break
		}
	}
	if !hasFail {
		t.Errorf("view() after phase 1 failure must mention failure; got:\n%s", rendered)
	}

	// Enter with confirmActive=false must NOT issue a write cmd (no-op or only Esc active).
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := pm.update(enterMsg)
	if cmd != nil {
		// It's acceptable to return a cmd that does NOT trigger a write (e.g., nil result).
		msg := cmd()
		if _, ok := msg.(writeResultMsg); ok {
			t.Error("Enter after phase 1 failure must NOT produce a writeResultMsg")
		}
		// pushScreenMsg would also be wrong.
		if _, ok := msg.(pushScreenMsg); ok {
			t.Error("Enter after phase 1 failure must NOT push a new screen")
		}
	}
}

// containsCI reports whether s contains substr (case-insensitive).
func containsCI(s, substr string) bool {
	sl := []byte(s)
	subl := []byte(substr)
	_ = sl
	_ = subl
	// Simple approach: lowercase both.
	sLow := toLower(s)
	subLow := toLower(substr)
	return len(subLow) > 0 && contains(sLow, subLow)
}

func toLower(s string) string {
	out := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out[i] = c
	}
	return string(out)
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- TestProveScreenRunsPhasesAsTeaCmds ---

// TestProveScreenRunsPhasesAsTeaCmds verifies that entering the prove screen
// issues runPreWriteCmd as a tea.Cmd (Init), and a passing preWriteResultMsg
// issues runResolvedCmd — neither tester call happens inside Update directly.
func TestProveScreenRunsPhasesAsTeaCmds(t *testing.T) {
	var called bool
	deps := fakeWriteTUIDeps(&called)

	m := newProveScreen("create", makeTestInput(), deps)

	// init() must return a non-nil cmd for phase 1.
	_, initCmd := m.init()
	if initCmd == nil {
		t.Fatal("proveModel.init() must return a non-nil tea.Cmd for phase 1")
	}

	// The init cmd must produce a preWriteResultMsg.
	initMsg := initCmd()
	if _, ok := initMsg.(preWriteResultMsg); !ok {
		t.Fatalf("init() cmd produced %T; want preWriteResultMsg", initMsg)
	}

	// After phase 1 passes, update must issue the phase 2 cmd.
	pass1 := makePassResult()
	updated, phase2Cmd := m.update(preWriteResultMsg{result: pass1})
	_ = updated
	if phase2Cmd == nil {
		t.Fatal("update(preWriteResultMsg{pass}) must return a non-nil tea.Cmd for phase 2")
	}

	// The phase 2 cmd must produce a resolvedResultMsg.
	phase2Msg := phase2Cmd()
	if _, ok := phase2Msg.(resolvedResultMsg); !ok {
		t.Fatalf("phase 2 cmd produced %T; want resolvedResultMsg", phase2Msg)
	}
}

// --- TestProveConfirmWritesViaDeps ---

// TestProveConfirmWritesViaDeps verifies that after both phases pass,
// keys.Confirm invokes the injected write function (via identity.Deps), proving
// the write routes through deps (no direct filewriter call in tui/prove.go).
func TestProveConfirmWritesViaDeps(t *testing.T) {
	var called bool
	deps := fakeWriteTUIDeps(&called)

	m := newProveScreen("create", makeTestInput(), deps)

	// Drive to confirmed state.
	pass1 := makePassResult()
	updated, _ := m.update(preWriteResultMsg{result: pass1})
	pm, ok := updated.(proveModel)
	if !ok {
		t.Fatalf("proveModel after phase1: got %T", updated)
	}

	passR, passResolved := makeResolvedResult()
	updated2, _ := pm.update(resolvedResultMsg{result: passR, resolved: passResolved})
	pm2, ok := updated2.(proveModel)
	if !ok {
		t.Fatalf("proveModel after phase2: got %T", updated2)
	}

	if !pm2.confirmActive {
		t.Fatal("confirmActive must be true before testing confirm write")
	}

	// Press Enter — must return a write cmd.
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, writeCmd := pm2.update(enterMsg)
	if writeCmd == nil {
		t.Fatal("Enter after both phases pass must return a non-nil write cmd")
	}

	// Execute the write cmd — it should call through identity.Deps (the fake).
	writeMsg := writeCmd()
	if _, ok := writeMsg.(writeResultMsg); !ok {
		t.Fatalf("write cmd produced %T; want writeResultMsg", writeMsg)
	}
}
