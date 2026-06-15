package tui

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/identity"
)

// fakeDocDeps returns a doctor.Deps that returns no findings for all families.
// Mirrors fakeDeps in cmd/gitid/add_test.go — same pattern, different type.
func fakeDocDeps() doctor.Deps {
	noFindings := func(_ doctor.Deps) []doctor.Finding { return nil }
	return doctor.Deps{
		CheckDeps:      noFindings,
		CheckPerms:     noFindings,
		CheckCoherence: noFindings,
		CheckOrphans:   noFindings,
		CheckSigning:   noFindings,
		CheckAgent:     noFindings,
		CheckBaseline:  noFindings,
	}
}

// fakeIdentityDeps returns an identity.Deps with no-op stubs.
func fakeIdentityDeps() identity.Deps {
	return identity.Deps{}
}

// fakeTUIDocDeps wraps fakeDocDeps in a tuiDeps for the screens whose
// constructors now take the full tuiDeps (dashboard, identity list) so the
// write chain can be threaded the real seams (CR-02).
func fakeTUIDocDeps() tuiDeps {
	return tuiDeps{doctor: fakeDocDeps()}
}

// newFakeRootModel builds a root model with all-fake deps for navigation tests.
func newFakeRootModel() rootModel {
	return newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{})
}

// TestRootModelViewNotPanic verifies that rootModel.View() does not panic
// on an empty stack and returns a tea.View (not string).
func TestRootModelViewNotPanic(t *testing.T) {
	t.Helper()
	m := newFakeRootModel()
	// should not panic
	v := m.View()
	// tea.View is a struct; this just verifies the return type compiles
	_ = v
}

// TestRootModelViewEmptyStack verifies View() returns an empty-ish view
// when stack is empty or has one screen.
func TestRootModelViewEmptyStack(t *testing.T) {
	t.Helper()
	m := newFakeRootModel()
	// Drain the stack to test empty behavior
	m.stack = nil
	v := m.View()
	// Must not panic and must return a tea.View
	_ = v
}

// TestRootModelPushScreen verifies that a pushScreenMsg appends to the stack.
func TestRootModelPushScreen(t *testing.T) {
	m := newFakeRootModel()
	initialLen := len(m.stack)

	stub := &stubScreen{}
	updated, _ := m.Update(pushScreenMsg{next: stub})
	rm := updated.(rootModel)

	if len(rm.stack) != initialLen+1 {
		t.Errorf("push: expected stack len %d, got %d", initialLen+1, len(rm.stack))
	}
}

// TestRootModelPushCmdHelper verifies that pushCmd returns a valid tea.Cmd.
func TestRootModelPushCmdHelper(t *testing.T) {
	stub := &stubScreen{}
	cmd := pushCmd(stub)
	if cmd == nil {
		t.Error("pushCmd must return a non-nil tea.Cmd")
	}
	msg := cmd()
	if _, ok := msg.(pushScreenMsg); !ok {
		t.Errorf("pushCmd() returned %T, want pushScreenMsg", msg)
	}
}

// TestRootModelPopCmdHelper verifies that popCmd returns a valid tea.Cmd.
func TestRootModelPopCmdHelper(t *testing.T) {
	cmd := popCmd()
	if cmd == nil {
		t.Error("popCmd must return a non-nil tea.Cmd")
	}
	msg := cmd()
	if _, ok := msg.(popScreenMsg); !ok {
		t.Errorf("popCmd() returned %T, want popScreenMsg", msg)
	}
}

// TestRootModelPopScreen verifies that a popScreenMsg removes the top screen
// but never empties below 1.
func TestRootModelPopScreen(t *testing.T) {
	m := newFakeRootModel()

	// Push a second screen so we can pop.
	stub := &stubScreen{}
	m.stack = append(m.stack, stub)
	beforeLen := len(m.stack)

	updated, _ := m.Update(popScreenMsg{})
	rm := updated.(rootModel)

	if len(rm.stack) != beforeLen-1 {
		t.Errorf("pop: expected stack len %d, got %d", beforeLen-1, len(rm.stack))
	}
	if len(rm.stack) < 1 {
		t.Error("pop: stack must never be emptied below 1")
	}
}

// TestRootModelPopNeverBelowOne verifies pop does not empty the stack below 1.
func TestRootModelPopNeverBelowOne(t *testing.T) {
	m := newFakeRootModel()
	// ensure exactly 1 item on stack
	if len(m.stack) > 1 {
		m.stack = m.stack[:1]
	}

	updated, _ := m.Update(popScreenMsg{})
	rm := updated.(rootModel)
	if len(rm.stack) < 1 {
		t.Errorf("pop on single-item stack must stay at 1, got %d", len(rm.stack))
	}
}

// TestRootModelWindowSizeMsg verifies that tea.WindowSizeMsg updates width and height.
func TestRootModelWindowSizeMsg(t *testing.T) {
	m := newFakeRootModel()

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updated, _ := m.Update(msg)
	rm := updated.(rootModel)

	if rm.width != 120 || rm.height != 40 {
		t.Errorf("WindowSizeMsg: want width=120 height=40, got width=%d height=%d", rm.width, rm.height)
	}
}

// TestKeysQuitMatchesQAndCtrlC verifies keys.Quit matches "q" and "ctrl+c".
func TestKeysQuitMatchesQAndCtrlC(t *testing.T) {
	qMsg := tea.KeyPressMsg{Text: "q", Code: 'q'}
	if !key.Matches(qMsg, keys.Quit) {
		t.Error("keys.Quit must match 'q'")
	}

	// ctrl+c: Code='c', Mod=ModCtrl per the v2 API (Key.String() returns "ctrl+c").
	ctrlCMsg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	if !key.Matches(ctrlCMsg, keys.Quit) {
		t.Error("keys.Quit must match 'ctrl+c'")
	}
}

// TestKeysBackMatchesEsc verifies keys.Back matches "esc".
func TestKeysBackMatchesEsc(t *testing.T) {
	escMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	if !key.Matches(escMsg, keys.Back) {
		t.Error("keys.Back must match 'esc'")
	}
}

// stubScreen is a minimal screenModel for testing the view stack.
type stubScreen struct{}

func (s *stubScreen) update(_ tea.Msg) (screenModel, tea.Cmd) { return s, nil }
func (s *stubScreen) view() string                            { return "stub" }
