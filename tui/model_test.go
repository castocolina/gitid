package tui

// model_test.go — tests for the Phase 5.6 two-pane root model.
//
// These tests were RED scaffolds in Plan 01. Plan 02 removes the t.Skip calls
// and implements real assertions against the two-pane shell.
//
// Test names are LOCKED by VALIDATION.md — do not rename.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/identity"
)

// keyCtrlP constructs the tea.KeyPressMsg for Ctrl+P.
// In bubbletea v2, Ctrl modifier keys are tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}.
// msg.String() returns "ctrl+p" which is what handleKey switches on.
func keyCtrlP() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
}

// keyEsc constructs the tea.KeyPressMsg for Escape.
func keyEsc() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEscape}
}

// sendMsg drives the root model through one Update cycle.
func sendMsg(m rootModel, msg tea.Msg) rootModel {
	next, _ := m.Update(msg)
	return next.(rootModel)
}

// sendKey drives the root model through one key-press Update.
// The key string is the character(s) to press (e.g. "1", "2", "?", "q").
// Single printable characters are sent as tea.KeyPressMsg with the Code set.
func sendKey(m rootModel, key string) rootModel {
	runes := []rune(key)
	if len(runes) == 0 {
		return m
	}
	return sendMsg(m, tea.KeyPressMsg{Code: runes[0]})
}

// buildModel returns a rootModel sized to 120×40 using fake deps.
func buildModel() rootModel {
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// TestRootModelTwoPaneRenders verifies the two-pane layout renders without panic
// at 80×24 (minimum) and 120×40 (comfortable) terminal sizes.
// Requirement: TUI-03 (persistent two-pane layout).
func TestRootModelTwoPaneRenders(t *testing.T) {
	sizes := []tea.WindowSizeMsg{
		{Width: 80, Height: 24},
		{Width: 120, Height: 40},
	}
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	for _, sz := range sizes {
		m2 := sendMsg(m, sz)
		view := m2.View()
		if view.Content == "" {
			t.Errorf("view must not be empty at %dx%d", sz.Width, sz.Height)
		}
		if !strings.Contains(view.Content, "gitid") {
			t.Errorf("view at %dx%d must contain app name 'gitid'", sz.Width, sz.Height)
		}
		// Footer must always render at valid sizes.
		if !strings.Contains(view.Content, "quit") {
			t.Errorf("view at %dx%d must contain 'quit' in the footer", sz.Width, sz.Height)
		}
	}
}

// TestSmallTerminal verifies that receiving a WindowSizeMsg with width < 80 or
// height < 24 renders the plain "Terminal too small" guard, halting normal rendering.
// Requirement: TUI-03 (responsive collapse), UI-SPEC § Responsive Collapse D-03.
func TestSmallTerminal(t *testing.T) {
	cases := []struct{ w, h int }{
		{40, 24},
		{79, 23},
		{79, 24},
		{80, 23},
	}
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	for _, c := range cases {
		m2 := sendMsg(m, tea.WindowSizeMsg{Width: c.w, Height: c.h})
		content := m2.renderContent()
		if content != "Terminal too small — resize to at least 80x24" {
			t.Errorf("at %dx%d expected small-terminal guard, got: %q", c.w, c.h, content)
		}
	}
}

// TestSidebarVisible verifies the sidebar is rendered when terminal width >= 80.
// Requirement: TUI-03 (sidebar always visible above breakpoint, D-01).
func TestSidebarVisible(t *testing.T) {
	m := buildModel() // 120×40
	content := m.renderPersistentLayout()
	if !strings.Contains(content, "Identities") {
		t.Error("sidebar 'Identities' section must be visible at width >= 80")
	}
}

// TestSidebarCollapse verifies the sidebar collapses (is hidden from layout)
// when terminal width < 80, per the responsive breakpoint D-03.
// Requirement: TUI-03.
func TestSidebarCollapse(t *testing.T) {
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	m = sendMsg(m, tea.WindowSizeMsg{Width: 70, Height: 30})

	if !m.sidebarCollapsed {
		t.Error("sidebarCollapsed must be true when width < 80")
	}
	// In collapsed mode, the layout should not embed the sidebar section header
	// inline with the main content; it may still appear in a sidebar overlay
	// triggered by \ key — but the base layout omits it.
	content := m.renderPersistentLayout()
	// The main pane fills the full width; the sidebar "Identities" section
	// header should NOT appear in the base collapsed layout.
	if strings.Contains(content, "Identities") {
		t.Error("sidebar 'Identities' must not appear in collapsed-mode base layout")
	}
	// The footer must contain the toggle-sidebar hint when collapsed.
	if !strings.Contains(content, "toggle sidebar") {
		t.Error("footer must contain 'toggle sidebar' hint in collapsed mode")
	}
}

// TestViewSwitch verifies that pressing keys 1/2/3 switches the active view
// without pushing a new screen.
// Requirement: TUI-04 (view switcher + keys.View1/View2/View3).
func TestViewSwitch(t *testing.T) {
	m := buildModel()

	// Default is identitiesView.
	if m.activeView != identitiesView {
		t.Errorf("default activeView must be identitiesView, got %d", m.activeView)
	}

	m2 := sendKey(m, "2")
	if m2.activeView != healthView {
		t.Errorf("pressing '2' must set activeView = healthView, got %d", m2.activeView)
	}

	m3 := sendKey(m2, "3")
	if m3.activeView != globalOptionsView {
		t.Errorf("pressing '3' must set activeView = globalOptionsView, got %d", m3.activeView)
	}

	m4 := sendKey(m3, "1")
	if m4.activeView != identitiesView {
		t.Errorf("pressing '1' must set activeView = identitiesView, got %d", m4.activeView)
	}
}

// TestPaletteOpen verifies that pressing Ctrl+P opens the command palette modal
// and that renderContent composites the palette via placeOverlay.
// Requirement: TUI-04 (Ctrl+P command palette, D-14).
func TestPaletteOpen(t *testing.T) {
	m := buildModel()
	m2 := sendMsg(m, keyCtrlP())

	if m2.activeModal != paletteModal {
		t.Errorf("ctrl+p must set activeModal = paletteModal, got %d", m2.activeModal)
	}
	content := m2.renderContent()
	if !strings.Contains(content, "Command Palette") {
		t.Error("renderContent must include 'Command Palette' when paletteModal is active")
	}
}

// TestHelpOverlay verifies that pressing '?' opens the help overlay modal and
// Esc clears it back to noModal.
// Requirement: TUI-06 (help overlay).
func TestHelpOverlay(t *testing.T) {
	m := buildModel()
	m2 := sendKey(m, "?")
	if m2.activeModal != helpModal {
		t.Errorf("'?' must set activeModal = helpModal, got %d", m2.activeModal)
	}
	content := m2.renderContent()
	if !strings.Contains(content, "Keyboard Shortcuts") {
		t.Error("renderContent must include 'Keyboard Shortcuts' when helpModal is active")
	}

	// Esc must dismiss the help overlay.
	m3 := sendMsg(m2, keyEsc())
	if m3.activeModal != noModal {
		t.Errorf("Esc must set activeModal = noModal, got %d", m3.activeModal)
	}
}

// TestFooterAlwaysRendered verifies the footer key hints bar is present in the
// rendered view for every active view and modal state.
// Requirement: TUI-06 (footer always rendered, closes G-02/G-04).
func TestFooterAlwaysRendered(t *testing.T) {
	m := buildModel()

	views := []struct {
		name    string
		setView func(rootModel) rootModel
	}{
		{"identitiesView", func(m rootModel) rootModel { return sendKey(m, "1") }},
		{"healthView", func(m rootModel) rootModel { return sendKey(m, "2") }},
		{"globalOptionsView", func(m rootModel) rootModel { return sendKey(m, "3") }},
	}

	for _, v := range views {
		vm := v.setView(m)
		footer := vm.renderFooter()
		if footer == "" {
			t.Errorf("renderFooter must not be empty for view %s", v.name)
		}
		// The "quit" key hint must always appear in the footer.
		if !strings.Contains(footer, "quit") {
			t.Errorf("renderFooter for %s must contain 'quit'", v.name)
		}
	}

	// Footer must also be non-empty when a modal is open.
	withHelp := sendKey(m, "?")
	if withHelp.renderFooter() == "" {
		t.Error("renderFooter must not be empty when helpModal is active")
	}

	withPalette := sendMsg(m, keyCtrlP())
	if withPalette.renderFooter() == "" {
		t.Error("renderFooter must not be empty when paletteModal is active")
	}
}

// TestTabsAreNumbered verifies the header tabs advertise their switch keys
// (P1-4): "1 Identities", "2 Health", "3 Global Options" — turning dead labels
// into a visible keymap.
func TestTabsAreNumbered(t *testing.T) {
	// The active tab uses Underline, which lipgloss emits per-character, so strip
	// ANSI before matching the visible text.
	head := ansi.Strip(buildModel().renderHeader())
	for _, want := range []string{"1 Identities", "2 Health", "3 Global Options"} {
		if !strings.Contains(head, want) {
			t.Errorf("header must number tabs; missing %q in:\n%s", want, head)
		}
	}
}

// TestFooterPrioritizesEssentialsWhenTight verifies P0-2: at a tight width the
// footer keeps the load-bearing hints (views, quit, add) and points to the help
// overlay with "· more in ?" instead of silently truncating the primary action.
func TestFooterPrioritizesEssentialsWhenTight(t *testing.T) {
	m := buildModel()
	m.width = 50 // tight — forces collapse
	foot := m.renderFooter()
	for _, want := range []string{"views", "quit", "add", "more in ?"} {
		if !strings.Contains(foot, want) {
			t.Errorf("tight footer must keep %q; got %q", want, foot)
		}
	}
}

// TestFooterAdvertisesViewKeys verifies the view-switch hint is always present
// (P1-4) — the discoverability gap the user hit ("only Ctrl+P switched views").
func TestFooterAdvertisesViewKeys(t *testing.T) {
	if foot := buildModel().renderFooter(); !strings.Contains(foot, "views") {
		t.Errorf("footer must advertise the 1·2·3 view-switch keys; got %q", foot)
	}
}

// TestRefreshSidebarSelectsFirstIdentity verifies P1-5: when identities arrive
// and nothing is selected, the first one is really selected so the detail pane
// populates on launch without a keystroke. Empty results keep no selection.
func TestRefreshSidebarSelectsFirstIdentity(t *testing.T) {
	m := buildModel()
	m = sendMsg(m, refreshSidebarMsg{accounts: []identity.Account{{Name: "alpha-id"}, {Name: "beta-id"}}})
	if m.sidebar.selected != 0 {
		t.Fatalf("first identity must be selected on load; got selected=%d", m.sidebar.selected)
	}
	if main := ansi.Strip(m.renderMainPane(60, 20)); !strings.Contains(main, "alpha-id") {
		t.Errorf("detail pane must show the first identity on load; got:\n%s", main)
	}

	empty := sendMsg(buildModel(), refreshSidebarMsg{accounts: nil})
	if empty.sidebar.selected != -1 {
		t.Errorf("no selection when there are no identities; got %d", empty.sidebar.selected)
	}
}

// TestEnterDrillsIntoDetail verifies P1-6: Enter on a selected sidebar row moves
// focus to the detail pane instead of being a dead key.
func TestEnterDrillsIntoDetail(t *testing.T) {
	m := buildModel()
	m = sendMsg(m, refreshSidebarMsg{accounts: []identity.Account{{Name: "x"}}})
	m.focused = "sidebar"
	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.focused != "main" {
		t.Errorf("Enter on a selected sidebar row must focus the detail pane; got focus=%q", m.focused)
	}
}

// TestInitSeedsSidebarAndHealth verifies P1-5: Init seeds both the sidebar
// refresh and the doctor run (non-nil batched command).
func TestInitSeedsSidebarAndHealth(t *testing.T) {
	if buildModel().Init() == nil {
		t.Error("Init must seed sidebar refresh + the health doctor run")
	}
}

// TestFamilyResultMarksHealthReady verifies the seeded doctor run marks health
// ready, so opening the Health view does not redundantly re-run the families.
func TestFamilyResultMarksHealthReady(t *testing.T) {
	m := buildModel()
	if m.healthReady {
		t.Fatal("healthReady should start false")
	}
	m = sendMsg(m, familyResultMsg{runID: m.health.runID, family: doctor.FamilyDeps})
	if !m.healthReady {
		t.Error("a familyResultMsg must mark healthReady (badges seeded at launch, no re-run on view 2)")
	}
}

// TestHealthRailNavigable is a regression test (D-5): the Health view advertises
// "↑↓ move" in the footer, so ↑/↓ must actually move a selection in the family
// rail (clamped at the ends). Reported on the real TTY: "The health view show me
// that is possible move with up/down arrows, that is not true."
func TestHealthRailNavigable(t *testing.T) {
	m := buildModel()
	m = sendKey(m, "2") // switch to Health view
	if m.activeView != healthView {
		t.Fatalf("expected healthView after '2'; got %d", m.activeView)
	}
	if m.health.selected != 0 {
		t.Fatalf("health selection must start at 0; got %d", m.health.selected)
	}

	down := func(m rootModel) rootModel { return sendMsg(m, tea.KeyPressMsg{Code: tea.KeyDown}) }
	up := func(m rootModel) rootModel { return sendMsg(m, tea.KeyPressMsg{Code: tea.KeyUp}) }

	// Up at the top clamps.
	m = up(m)
	if m.health.selected != 0 {
		t.Errorf("↑ at the top must clamp to 0; got %d", m.health.selected)
	}

	// Down advances the selection.
	m = down(m)
	if m.health.selected != 1 {
		t.Errorf("↓ must advance the family selection to 1; got %d", m.health.selected)
	}

	// Down past the last family clamps.
	n := len(doctor.Families())
	for i := 0; i < n+3; i++ {
		m = down(m)
	}
	if m.health.selected != n-1 {
		t.Errorf("↓ past the end must clamp to %d; got %d", n-1, m.health.selected)
	}

	// ↑ moves back up.
	m = up(m)
	if m.health.selected != n-2 {
		t.Errorf("↑ must move the selection back to %d; got %d", n-2, m.health.selected)
	}
}

// TestArrowKeysCycleViews verifies P2-9 (D-04 extended): ←/→ cycle through the
// three views, additive to 1/2/3, and switching to Health via arrow inits it.
func TestArrowKeysCycleViews(t *testing.T) {
	m := buildModel() // starts on identitiesView

	right := func(m rootModel) rootModel { return sendMsg(m, tea.KeyPressMsg{Code: tea.KeyRight}) }
	left := func(m rootModel) rootModel { return sendMsg(m, tea.KeyPressMsg{Code: tea.KeyLeft}) }

	m = right(m)
	if m.activeView != healthView {
		t.Fatalf("→ from identities must go to health; got %d", m.activeView)
	}
	if !m.healthReady {
		t.Error("switching to Health via → must init the doctor run")
	}
	m = right(m)
	if m.activeView != globalOptionsView {
		t.Fatalf("→ from health must go to global; got %d", m.activeView)
	}
	m = right(m)
	if m.activeView != identitiesView {
		t.Fatalf("→ from global must wrap to identities; got %d", m.activeView)
	}
	// ← reverses.
	if m = left(m); m.activeView != globalOptionsView {
		t.Fatalf("← from identities must wrap to global; got %d", m.activeView)
	}
}

// TestRailIsContextual verifies WP-5 (D-01 reopened): the left rail content
// follows the active view instead of always showing the identity list — the
// "scope lie" the user reported. The rail stays present (Tab-focusable, 18c).
func TestRailIsContextual(t *testing.T) {
	m := buildModel()
	m = sendMsg(m, refreshSidebarMsg{accounts: []identity.Account{{Name: "castocolina"}}})

	// Identities view: rail shows the identity list.
	id := ansi.Strip(m.renderPersistentLayout())
	if !strings.Contains(id, "Identities") || !strings.Contains(id, "castocolina") {
		t.Errorf("Identities view rail must list identities; got:\n%s", id)
	}

	// Health view: rail shows the doctor family index, NOT the identity list.
	h := ansi.Strip(sendKey(m, "2").renderPersistentLayout())
	if !strings.Contains(h, "Health") || !strings.Contains(h, "Dependencies") {
		t.Errorf("Health view rail must list doctor families; got:\n%s", h)
	}
	if strings.Contains(h, "castocolina") {
		t.Errorf("Health view rail must NOT show the identity list (scope lie); got:\n%s", h)
	}

	// Global Options view: rail shows config sections.
	g := ansi.Strip(sendKey(m, "3").renderPersistentLayout())
	if !strings.Contains(g, "Core") || !strings.Contains(g, "URL Rewrites") {
		t.Errorf("Global Options rail must list config sections; got:\n%s", g)
	}
}
