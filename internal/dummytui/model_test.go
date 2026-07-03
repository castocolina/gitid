package dummytui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewModel_SeededOnIdentityManagerWithEmptyModalStack(t *testing.T) {
	m := NewModel()
	if m.nav.view != "identity-manager" {
		t.Fatalf("NewModel: view = %q, want identity-manager", m.nav.view)
	}
	if len(m.nav.modalStack) != 0 {
		t.Fatalf("NewModel: modalStack len = %d, want 0", len(m.nav.modalStack))
	}
}

func TestModel_ViewContainsBreadcrumbAndAltScreen(t *testing.T) {
	m := NewModel()
	v := m.View()
	if !v.AltScreen {
		t.Fatal("View: AltScreen not set")
	}
	s := v.Content
	if !strings.Contains(s, "identity-manager/") {
		t.Fatalf("View: output missing identity-manager breadcrumb:\n%s", s)
	}
}

func TestModel_UpdateAppliesRouteOnKeyMsg(t *testing.T) {
	m := NewModel()
	next, _ := m.Update(tea.KeyPressMsg{Code: '2'})
	nm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update: returned type %T, want Model", next)
	}
	if nm.nav.view != "global-ssh" {
		t.Fatalf("Update on '2': view = %q, want global-ssh", nm.nav.view)
	}
}

func TestModel_ModalLaunchThroughModel_BreadcrumbAndEscReverts(t *testing.T) {
	snapshotRegistry(t)
	const (
		source  = "test-model-modal-source"
		keyless = "test-model-modal-keyless"
	)
	Register(SurfaceDef{
		ID:      source,
		Screens: []ScreenDef{{ID: "entry", Render: func() string { return "source body" }}},
	})
	Register(SurfaceDef{
		ID:         keyless,
		LaunchFrom: source,
		LaunchKey:  "m",
		Screens:    []ScreenDef{{ID: "modal-entry", Render: func() string { return "modal body" }}},
	})

	m := Model{width: defaultWidth, height: defaultHeight, nav: navState{view: source, activeScreen: "entry"}}

	launched, _ := m.Update(tea.KeyPressMsg{Code: 'm'})
	lm := launched.(Model)
	view := lm.View().Content
	wantBreadcrumb := keyless + "/modal-entry"
	if !strings.Contains(view, wantBreadcrumb) {
		t.Fatalf("View after modal launch: missing breadcrumb %q:\n%s", wantBreadcrumb, view)
	}
	if !strings.Contains(view, "modal body") {
		t.Fatalf("View after modal launch: missing modal body content:\n%s", view)
	}

	popped, _ := lm.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	pm := popped.(Model)
	poppedView := pm.View().Content
	wantParentBreadcrumb := source + "/entry"
	if !strings.Contains(poppedView, wantParentBreadcrumb) {
		t.Fatalf("View after Esc pop: missing parent breadcrumb %q:\n%s", wantParentBreadcrumb, poppedView)
	}
	if strings.Contains(poppedView, wantBreadcrumb) {
		t.Fatalf("View after Esc pop: modal breadcrumb %q still present:\n%s", wantBreadcrumb, poppedView)
	}
}

func TestPlaceOverlay_ClampsOversizedModalWithoutPanic(t *testing.T) {
	bg := strings.Repeat("background line\n", 5)
	bg = strings.TrimSuffix(bg, "\n")
	fg := strings.Repeat("MODAL LINE THAT IS QUITE LONG\n", 20)
	fg = strings.TrimSuffix(fg, "\n")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("placeOverlay panicked on an oversized modal: %v", r)
		}
	}()
	out := placeOverlay(1000, 1000, fg, bg)
	if out == "" {
		t.Fatal("placeOverlay: unexpected empty output")
	}
}

func TestRenderScreen_FullShellFourRegionsAndBreadcrumb(t *testing.T) {
	out, err := RenderScreen("identity-manager", entryScreenID(mustSurface(t, "identity-manager")))
	if err != nil {
		t.Fatalf("RenderScreen: unexpected error: %v", err)
	}
	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		t.Fatalf("RenderScreen: full-shell output has %d lines, want at least 4 (header/body/status/keybar):\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], "identity-manager/") {
		t.Fatalf("RenderScreen: header line missing breadcrumb:\n%s", lines[0])
	}
	keybar := lines[len(lines)-1]
	if !strings.Contains(keybar, "Esc back") || !strings.Contains(keybar, "q quit") {
		t.Fatalf("RenderScreen: keybar line missing reserved-key hints:\n%s", keybar)
	}
}
