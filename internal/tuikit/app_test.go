package tuikit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// press sends one keystroke to the app and returns the updated app + cmd.
func press(t *testing.T, a App, name string) (App, tea.Cmd) {
	t.Helper()
	model, cmd := a.Update(pressKey(name))
	next, ok := model.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", model)
	}
	return next, cmd
}

// appView renders the app frame as plain text.
func appView(a App) string {
	return stripANSI(a.View().Content)
}

func TestNewAppRendersTheFrame(t *testing.T) {
	a := NewApp(stubBackend{})
	view := appView(a)
	for _, want := range []string{"gitid", "[1] Identities", "[2] Global SSH", "[3] Global Git", "[4] Doctor", "8 ids"} {
		if !strings.Contains(view, want) {
			t.Errorf("initial frame missing %q", want)
		}
	}
	if a.tab != TabIdentities {
		t.Errorf("initial tab = %v, want Identities", a.tab)
	}
}

func TestNumberKeysSwitchTabs(t *testing.T) {
	a := NewApp(stubBackend{})
	a, _ = press(t, a, "3")
	if a.tab != TabGlobalGit {
		t.Fatalf("tab = %v after pressing 3, want Global Git", a.tab)
	}
	if !strings.Contains(appView(a), "Global Git") {
		t.Error("breadcrumb should show the active tab label")
	}
	a, _ = press(t, a, "1")
	if a.tab != TabIdentities {
		t.Errorf("tab = %v after pressing 1, want Identities", a.tab)
	}
}

// TestTabsAndPaletteReachAllPrimaryViews pins SHELL-02 against the frozen
// FIX-02 four-tab shell: number keys 1–4 switch the four primary views,
// the palette offers those four plus Help (five entries), and a fifth
// number key does not invent a tab the approved design absorbed into Doctor.
func TestTabsAndPaletteReachAllPrimaryViews(t *testing.T) {
	want := []TabID{TabIdentities, TabGlobalSSH, TabGlobalGit, TabDoctor}
	a := NewApp(stubBackend{})
	for i, tab := range want {
		key := string(rune('1' + i))
		a, _ = press(t, a, key)
		if a.tab != tab {
			t.Errorf("key %s → tab %v, want %v", key, a.tab, tab)
		}
	}
	if len(paletteEntries) != 5 {
		t.Errorf("palette entries = %d, want 5 (four views + help)", len(paletteEntries))
	}
	a, _ = press(t, a, "ctrl+p")
	view := appView(a)
	for _, e := range paletteEntries {
		if !strings.Contains(view, e.label) {
			t.Errorf("palette missing %q", e.label)
		}
	}
}

func TestHelpOverlayShowsFullLegend(t *testing.T) {
	a := NewApp(stubBackend{})
	a, _ = press(t, a, "?")
	if a.overlay != overlayHelp {
		t.Fatal("? must open the help overlay")
	}
	view := appView(a)

	// All 8 MGR-02 state words.
	for _, state := range []string{
		"complete", "key-used-both", "key-used-ssh-only", "incomplete",
		"git-only", "key-unused", "key-missing", "fragment-path-missing",
	} {
		if !strings.Contains(view, state) {
			t.Errorf("help legend missing state word %q", state)
		}
	}
	// The S/G pip legend header.
	if !strings.Contains(view, "S/G pips = capability (✓ wired · – none · ✗ broken)") {
		t.Error("help missing the S/G pip legend")
	}
	// Key rows.
	for _, want := range []string{"Ctrl+P", "Switch view", "fix all"} {
		if !strings.Contains(view, want) {
			t.Errorf("help key table missing %q", want)
		}
	}

	a, _ = press(t, a, "esc")
	if a.overlay != overlayNone {
		t.Error("esc must close the help overlay")
	}
}

func TestQuitPromptEnterQuitsEscStays(t *testing.T) {
	a := NewApp(stubBackend{})
	a, _ = press(t, a, "q")
	if a.overlay != overlayQuit {
		t.Fatal("q must open the quit prompt")
	}
	if !strings.Contains(appView(a), "Quit gitid?") {
		t.Error("quit prompt body missing")
	}

	// Esc stays.
	stay, _ := press(t, a, "esc")
	if stay.overlay != overlayNone {
		t.Error("esc must dismiss the quit prompt and stay")
	}

	// Enter quits for real (unlike the browser demo).
	_, cmd := press(t, a, "enter")
	if cmd == nil {
		t.Fatal("enter on the quit prompt must return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("enter on the quit prompt must produce tea.Quit")
	}
}

func TestPaletteFiltersAndOpensFirstMatch(t *testing.T) {
	a := NewApp(stubBackend{})
	a, _ = press(t, a, "ctrl+p")
	if a.overlay != overlayPalette {
		t.Fatal("ctrl+p must open the palette")
	}
	if !strings.Contains(appView(a), "Command palette") {
		t.Error("palette body missing")
	}

	for _, r := range "doctor" {
		a, _ = press(t, a, string(r))
	}
	matches := a.paletteMatches()
	if len(matches) != 1 || matches[0].tab != TabDoctor {
		t.Fatalf("palette matches for 'doctor' = %v", matches)
	}
	a, _ = press(t, a, "enter")
	if a.overlay != overlayNone || a.tab != TabDoctor {
		t.Errorf("enter must open the first match; overlay=%v tab=%v", a.overlay, a.tab)
	}
}

func TestWindowSizeGuard(t *testing.T) {
	a := NewApp(stubBackend{})
	model, _ := a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a = model.(App)
	if !strings.Contains(appView(a), "resize to at least 100x30") {
		t.Error("undersized terminals must render the resize guard")
	}
}
