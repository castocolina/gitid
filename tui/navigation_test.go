package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/identity"
)

// TestDashboardEnterNavigates verifies that pressing Enter (keys.Select) from
// the dashboard returns a cmd that produces a pushScreenMsg whose screen is
// the identity list.
func TestDashboardEnterNavigates(t *testing.T) {
	m := newDashboardModel(fakeDocDeps())

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, cmd := m.update(enterMsg)
	_ = updated

	if cmd == nil {
		t.Fatal("Enter from dashboard must return a non-nil tea.Cmd (pushCmd)")
	}
	msg := cmd()
	push, ok := msg.(pushScreenMsg)
	if !ok {
		t.Fatalf("Enter from dashboard cmd produced %T; want pushScreenMsg", msg)
	}
	if push.next == nil {
		t.Error("pushScreenMsg.next must not be nil (should be identity list screen)")
	}
	if _, ok := push.next.(identityListModel); !ok {
		t.Errorf("pushScreenMsg.next is %T; want identityListModel", push.next)
	}
}

// TestIdentityListEscPops verifies that pressing Esc (keys.Back) from the
// identity list screen returns a popCmd producing a popScreenMsg.
func TestIdentityListEscPops(t *testing.T) {
	m := newIdentityListScreen(fakeDocDeps())

	escMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	updated, cmd := m.update(escMsg)
	_ = updated

	if cmd == nil {
		t.Fatal("Esc from identity list must return a non-nil tea.Cmd (popCmd)")
	}
	msg := cmd()
	if _, ok := msg.(popScreenMsg); !ok {
		t.Fatalf("Esc from identity list cmd produced %T; want popScreenMsg", msg)
	}
}

// TestIdentityListEscPopsStack verifies the full push+pop round-trip at root
// level: after Enter from dashboard pushes the identity list, Esc pops back
// so the stack length returns to 1 (dashboard only).
func TestIdentityListEscPopsStack(t *testing.T) {
	root := newRootModel(fakeDocDeps(), fakeIdentityDeps())
	initialLen := len(root.stack)

	// Push the identity list via pushScreenMsg.
	listScreen := newIdentityListScreen(fakeDocDeps())
	updated, _ := root.Update(pushScreenMsg{next: listScreen})
	root = updated.(rootModel)
	if len(root.stack) != initialLen+1 {
		t.Fatalf("after push: stack len = %d; want %d", len(root.stack), initialLen+1)
	}

	// Pop via popScreenMsg.
	updated, _ = root.Update(popScreenMsg{})
	root = updated.(rootModel)
	if len(root.stack) != initialLen {
		t.Fatalf("after pop: stack len = %d; want %d", len(root.stack), initialLen)
	}
}

// TestIdentityListAddKey verifies that pressing 'a' (keys.Add) from the
// identity list returns a pushScreenMsg toward the create form screen.
func TestIdentityListAddKey(t *testing.T) {
	m := newIdentityListScreen(fakeDocDeps())

	aMsg := tea.KeyPressMsg{Text: "a"}
	updated, cmd := m.update(aMsg)
	_ = updated

	if cmd == nil {
		t.Fatal("'a' from identity list must return a non-nil tea.Cmd (pushCmd create form)")
	}
	msg := cmd()
	push, ok := msg.(pushScreenMsg)
	if !ok {
		t.Fatalf("'a' from identity list cmd produced %T; want pushScreenMsg", msg)
	}
	if push.next == nil {
		t.Error("pushScreenMsg.next must not be nil for create form")
	}
}

// TestIdentityListEnterPushesDetail verifies that pressing Enter (keys.Select)
// on a list item from the identity list screen returns a pushScreenMsg toward
// the identity detail screen (placeholder factory).
func TestIdentityListEnterPushesDetail(t *testing.T) {
	// Build a list model with one item so SelectedItem() returns it.
	d := fakeDocDeps()
	d.Identities = []identity.Account{
		{Name: "personal", Provider: "github.com"},
	}
	m := newIdentityListScreen(d)

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updated, cmd := m.update(enterMsg)
	_ = updated

	if cmd == nil {
		t.Fatal("Enter on item from identity list must return a non-nil tea.Cmd (pushCmd detail)")
	}
	msg := cmd()
	push, ok := msg.(pushScreenMsg)
	if !ok {
		t.Fatalf("Enter on item cmd produced %T; want pushScreenMsg", msg)
	}
	if push.next == nil {
		t.Error("pushScreenMsg.next must not be nil for identity detail")
	}
}

// TestWindowSizePropagation verifies that tea.WindowSizeMsg propagates correctly
// to both the dashboard and the identity list screen (dimensions updated).
func TestWindowSizePropagation(t *testing.T) {
	// Dashboard propagation.
	dash := newDashboardModel(fakeDocDeps())
	wsMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updated, _ := dash.update(wsMsg)
	dm := updated.(dashboardModel)
	if dm.width != 120 || dm.height != 40 {
		t.Errorf("dashboard WindowSizeMsg: want 120x40, got %dx%d", dm.width, dm.height)
	}

	// Identity list propagation.
	il := newIdentityListScreen(fakeDocDeps())
	updated2, _ := il.update(wsMsg)
	ilm, ok := updated2.(identityListModel)
	if !ok {
		t.Fatalf("identityList WindowSizeMsg: updated is %T; want identityListModel", updated2)
	}
	if ilm.width != 120 || ilm.height != 40 {
		t.Errorf("identityList WindowSizeMsg: want 120x40, got %dx%d", ilm.width, ilm.height)
	}
}

// TestQuitFromAnyScreen verifies that pressing 'q' (keys.Quit) returns a
// tea.Quit command from both the dashboard and the identity list screen.
func TestQuitFromAnyScreen(t *testing.T) {
	qMsg := tea.KeyPressMsg{Text: "q"}

	// Dashboard.
	dash := newDashboardModel(fakeDocDeps())
	_, cmd := dash.update(qMsg)
	if cmd == nil {
		t.Fatal("'q' from dashboard must return a non-nil tea.Cmd (tea.Quit)")
	}
	// tea.Quit is a sentinel function value; verify it produces tea.QuitMsg.
	quitResult := cmd()
	if _, ok := quitResult.(tea.QuitMsg); !ok {
		t.Errorf("'q' from dashboard: cmd produced %T; want tea.QuitMsg", quitResult)
	}

	// Identity list.
	il := newIdentityListScreen(fakeDocDeps())
	_, cmd2 := il.update(qMsg)
	if cmd2 == nil {
		t.Fatal("'q' from identity list must return a non-nil tea.Cmd (tea.Quit)")
	}
	quitResult2 := cmd2()
	if _, ok := quitResult2.(tea.QuitMsg); !ok {
		t.Errorf("'q' from identity list: cmd produced %T; want tea.QuitMsg", quitResult2)
	}
}

// fakeListItem returns a list.Item-compatible identity item for test setup.
// Used by future tests that need a pre-built list item without a full deps build.
var _ = fakeListItem // suppress unused lint until called by future tests

func fakeListItem(name, provider string) list.Item {
	return identityItem{account: identity.Account{Name: name, Provider: provider}}
}
