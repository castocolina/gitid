package tuikit

import (
	"fmt"
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
	for _, want := range []string{"gitid", "[1] Identities", "[2] SSH", "[3] Git", "[4] Health", "[5] Fixer", "[6] Ignore", "8 ids"} {
		if !strings.Contains(view, want) {
			t.Errorf("initial frame missing %q", want)
		}
	}
	if a.tab != TabIdentities {
		t.Errorf("initial tab = %v, want Identities", a.tab)
	}
}

func TestNewAppOnGlobalSSHOpensEmptyOptionsAndStorage(t *testing.T) {
	opts := NewAppOnGlobalSSH(stubBackend{}, false)
	if opts.ActiveTab() != TabGlobalSSH {
		t.Fatalf("options fallback tab = %v, want TabGlobalSSH", opts.ActiveTab())
	}
	storage, chosen, _ := opts.GlobalSSHUIState()
	if storage {
		t.Fatal("options fallback must open the Options sub-tab")
	}
	if chosen != 0 {
		t.Fatalf("options fallback selection = %d, want empty", chosen)
	}

	stor := NewAppOnGlobalSSH(stubBackend{}, true)
	if stor.ActiveTab() != TabGlobalSSH {
		t.Fatalf("storage fallback tab = %v, want TabGlobalSSH", stor.ActiveTab())
	}
	storage, chosen, radio := stor.GlobalSSHUIState()
	if !storage {
		t.Fatal("storage fallback must open the Storage sub-tab")
	}
	if chosen != 0 {
		t.Fatalf("storage fallback option selection = %d, want empty", chosen)
	}
	if radio != StorageSentinel {
		t.Fatalf("storage radio = %q, want the stub's current layout %q", radio, StorageSentinel)
	}
}

func TestNewAppOnGlobalGitOpensEmptySelection(t *testing.T) {
	app := NewAppOnGlobalGit(stubBackend{})
	if app.ActiveTab() != TabGlobalGit {
		t.Fatalf("fallback tab = %v, want TabGlobalGit", app.ActiveTab())
	}
	if chosen := app.GlobalGitUIState(); chosen != 0 {
		t.Fatalf("fallback selection = %d, want empty", chosen)
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

// TestTabsAndPaletteReachAllPrimaryViews pins SHELL-02 against the 6-tab
// shell (09.2-01-PLAN.md Task 1): number keys 1–6 switch the six primary
// views, and the palette offers those six plus Help (seven entries).
func TestTabsAndPaletteReachAllPrimaryViews(t *testing.T) {
	want := []TabID{TabIdentities, TabGlobalSSH, TabGlobalGit, TabHealth, TabFixer, TabGitIgnore}
	a := NewApp(stubBackend{})
	for i, tab := range want {
		key := string(rune('1' + i))
		a, _ = press(t, a, key)
		if a.tab != tab {
			t.Errorf("key %s → tab %v, want %v", key, a.tab, tab)
		}
	}
	if len(paletteEntries) != 7 {
		t.Errorf("palette entries = %d, want 7 (six views + help)", len(paletteEntries))
	}
	a, _ = press(t, a, "ctrl+p")
	view := appView(a)
	for _, e := range paletteEntries {
		if !strings.Contains(view, e.label) {
			t.Errorf("palette missing %q", e.label)
		}
	}
}

// TestNewScreensExhaustiveSwitchOverTabID closes newScreens's own coverage
// gap (08-01-PLAN.md Task 3): asserts newScreens returns exactly one
// screenModel of the correct concrete type per TabID value, in TabID order —
// no existing test iterated every TabID against its returned screen array.
func TestNewScreensExhaustiveSwitchOverTabID(t *testing.T) {
	screens := newScreens(stubBackend{}, DemoState{})
	want := []struct {
		tab  TabID
		kind string
	}{
		{TabIdentities, fmt.Sprintf("%T", identitiesModel{})},
		{TabGlobalSSH, fmt.Sprintf("%T", globalSSHModel{})},
		{TabGlobalGit, fmt.Sprintf("%T", globalGitModel{})},
		{TabHealth, fmt.Sprintf("%T", healthModel{})},
		{TabFixer, fmt.Sprintf("%T", fixerModel{})},
		{TabGitIgnore, fmt.Sprintf("%T", gitIgnoreModel{})},
	}
	if len(screens) != len(want) {
		t.Fatalf("newScreens returned %d screens, want %d", len(screens), len(want))
	}
	for _, w := range want {
		got := fmt.Sprintf("%T", screens[w.tab])
		if got != w.kind {
			t.Errorf("screens[%v] = %s, want %s", w.tab, got, w.kind)
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

	for _, r := range "health" {
		a, _ = press(t, a, string(r))
	}
	matches := a.paletteMatches()
	if len(matches) != 1 || matches[0].tab != TabHealth {
		t.Fatalf("palette matches for 'health' = %v", matches)
	}
	a, _ = press(t, a, "enter")
	if a.overlay != overlayNone || a.tab != TabHealth {
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

// TestNewAppPrefilledNilBehavesLikeNewApp pins the no-prefill contract of the
// D-02 pre-filled entry point: a nil pre-fill value must behave byte-for-byte
// like NewApp (the create wizard stays closed, the pane is the detail pane).
func TestNewAppPrefilledNilBehavesLikeNewApp(t *testing.T) {
	a := NewApp(stubBackend{})
	prefilled := NewAppPrefilled(stubBackend{}, nil)
	if prefilled.View().Content != a.View().Content {
		t.Error("NewAppPrefilled(b, nil) rendered a different frame than NewApp(b)")
	}
	im := prefilled.screens[TabIdentities].(identitiesModel)
	if im.pane != paneDetail {
		t.Errorf("pane = %v with nil pre-fill, want paneDetail", im.pane)
	}
}

// TestNewAppPrefilledOpensWizardPrefilled proves the D-15 pre-filled entry
// the CLI's D-02 flag pre-fill consumes: NewAppPrefilled(b, pre) opens the
// Identities screen's create pane with the wizard populated from the pre-fill
// values — SSH alias prefix/host/endpoint from the derivation, author fields
// on the Git form — while the test gate stays at the fresh wizard's idle
// value (D-16: a pre-filled clone still re-runs the FULL two-stage test).
func TestNewAppPrefilledOpensWizardPrefilled(t *testing.T) {
	pre := ClonePrefillView{
		AliasPrefix:   "acme-work",
		Hostname:      "ssh.github.com",
		Port:          "443",
		GitName:       "Acme Work",
		GitEmail:      "work@acme.example",
		MatchStrategy: "gitdir",
		GitDir:        "~/git/acme-work/",
	}
	a := NewAppPrefilled(stubBackend{}, &pre)
	im := a.screens[TabIdentities].(identitiesModel)
	if im.pane != paneCreate {
		t.Fatalf("pane = %v, want paneCreate", im.pane)
	}
	if got := im.wizard.form.identityName(); got != "acme-work" {
		t.Errorf("wizard identityName = %q, want acme-work (pre-filled alias prefix)", got)
	}
	if got := im.wizard.form.sshHost(); got != "acme-work.github.com" {
		t.Errorf("wizard sshHost = %q, want acme-work.github.com (provider reconstructed from the re-derived hostname)", got)
	}
	if got := im.wizard.form.hostname.Value(); got != "ssh.github.com" {
		t.Errorf("wizard hostname = %q, want ssh.github.com", got)
	}
	if got := im.wizard.form.port.Value(); got != "443" {
		t.Errorf("wizard port = %q, want 443", got)
	}
	if got := im.wizard.git.name.Value(); got != "Acme Work" {
		t.Errorf("wizard git name = %q, want Acme Work", got)
	}
	if got := im.wizard.git.email.Value(); got != "work@acme.example" {
		t.Errorf("wizard git email = %q, want work@acme.example", got)
	}
	if im.wizard.testPhase != testIdle {
		t.Errorf("pre-filled wizard testPhase = %q, want %q (D-16: the gate still runs)", im.wizard.testPhase, testIdle)
	}
}

// TestNewAppPrefilledReuseKeyPopulatesPicker proves the D-10 reuse-path
// pre-fill rides through the D-02 entry point: a pre-fill carrying a source
// key path seeds the wizard's key-source picker onto that path.
func TestNewAppPrefilledReuseKeyPopulatesPicker(t *testing.T) {
	pre := ClonePrefillView{
		AliasPrefix:   "acme-work",
		Hostname:      "ssh.github.com",
		Port:          "443",
		GitName:       "Acme Work",
		GitEmail:      "work@acme.example",
		ReuseKeyPath:  stubManualReusePath,
		MatchStrategy: "gitdir",
	}
	a := NewAppPrefilled(stubBackend{}, &pre)
	im := a.screens[TabIdentities].(identitiesModel)
	if im.wizard.keySource != keySourceReuse {
		t.Fatalf("keySource = %v, want reuse", im.wizard.keySource)
	}
	if got := im.wizard.reuseKeyPath(); got != stubManualReusePath {
		t.Errorf("reuseKeyPath = %q, want the pre-filled source key path", got)
	}
}
