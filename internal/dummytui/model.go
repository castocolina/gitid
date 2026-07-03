package dummytui

import (
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

const (
	// defaultWidth/defaultHeight seed the model before the first
	// tea.WindowSizeMsg arrives, mirroring tui/model.go's approach of
	// starting from a sane fixed geometry.
	defaultWidth  = 100
	defaultHeight = 30
)

// Model is the dummy's tea.Model: a thin wrapper around navState that
// dispatches key presses through route() (registry.go) and renders the
// active (or topmost-modal) surface/screen through the four-region shell
// (shell.go), compositing an open modal over the dimmed parent via
// placeOverlay (overlay.go) — the dummy's generalization of tui/model.go's
// single-activeModal dim-then-composite dispatch into a modalStack.
type Model struct {
	width, height int
	nav           navState
}

// NewModel returns a tea.Model seeded from the registry with
// identity-manager active and an empty modalStack.
func NewModel() Model {
	sd, _ := lookupSurface("identity-manager")
	return Model{
		width:  defaultWidth,
		height: defaultHeight,
		nav: navState{
			view:         "identity-manager",
			activeScreen: entryScreenID(sd),
		},
	}
}

// Init satisfies tea.Model. The dummy has no backend work to kick off.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update satisfies tea.Model: window-size messages resize the shell; "q"
// and "ctrl+c" are the globally reserved quit keys (doc.go's key-allocation
// table: "q ? / j k (arrows) | (all) | reserved | quit / help / filter /
// move", mirroring tui/model.go's real quit handling) and are intercepted
// here — BEFORE route() — so they always quit regardless of nav state,
// including while a modal is open; every other key message is dispatched
// through route().
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if k := msg.String(); k == "q" || k == "ctrl+c" {
			return m, tea.Quit
		}
		m.nav = route(m.nav, msg)
		return m, nil
	}
	return m, nil
}

// View satisfies tea.Model. Returns the rendered content with AltScreen
// enabled (tea.WithAltScreen() does not exist in v2 — set via View.AltScreen,
// mirroring tui/model.go's rootModel.View()).
func (m Model) View() tea.View {
	v := tea.NewView(m.renderContent())
	v.AltScreen = true
	return v
}

// activeFrame resolves the currently active (surface, screen): the topmost
// modalStack frame when a modal is open, else the top-level view/activeScreen.
func (m Model) activeFrame() (surfaceID, screenID string) {
	if len(m.nav.modalStack) > 0 {
		top := m.nav.modalStack[len(m.nav.modalStack)-1]
		return top.Surface, top.Screen
	}
	return m.nav.view, m.nav.activeScreen
}

// renderContent renders the persistent shell around the PARENT surface's
// active screen (m.nav.view/activeScreen) always, with the header breadcrumb
// and keybar reflecting the ACTIVE frame (review C3) — the topmost modalStack
// frame when a modal is open, else the parent itself. When a modal is open,
// the parent body is dimmed and the modal surface's active-screen BODY is
// composited over it via placeOverlay (review C3), the dummy's generalization
// of tui/model.go's single-activeModal dim-then-composite dispatch into a
// modalStack; Esc pops the modal (route(), registry.go) and the breadcrumb
// reverts to the parent on the next render.
func (m Model) renderContent() string {
	parentSD, ok := lookupSurface(m.nav.view)
	if !ok {
		return "dummytui: unknown top-level surface " + m.nav.view
	}
	parentScr, ok := findScreen(parentSD, m.nav.activeScreen)
	if !ok {
		return "dummytui: unknown screen " + m.nav.activeScreen + " on surface " + m.nav.view
	}

	activeSurfaceID, activeScreenID := m.activeFrame()
	activeSD, activeScr := parentSD, parentScr
	if len(m.nav.modalStack) > 0 {
		if sd, ok := lookupSurface(activeSurfaceID); ok {
			if scr, ok := findScreen(sd, activeScreenID); ok {
				activeSD, activeScr = sd, scr
			}
		}
	}

	header := renderShellHeader(activeSurfaceID, activeScreenID)
	status := renderShellStatus()
	keybar := renderShellKeybar(activeSD, activeScr)
	baseLayout := lipgloss.JoinVertical(lipgloss.Left, header, parentScr.Render(), status, keybar)

	if len(m.nav.modalStack) == 0 {
		return baseLayout
	}

	modalSD, ok := lookupSurface(activeSurfaceID)
	if !ok {
		return baseLayout
	}
	modalScr, ok := findScreen(modalSD, activeScreenID)
	if !ok {
		return baseLayout
	}

	dimmed := styleShellDimmed.Render(baseLayout)
	modalContent := modalScr.Render()

	// Center the modal within the ACTUAL rendered content height (not the
	// window's full m.height): the dummy's shell renders exactly as many
	// rows as its four regions need, so centering against m.height (the
	// terminal's total rows) would push a short modal below the visible
	// content and placeOverlay would clamp it away entirely (rows out of
	// bounds are skipped, never negative-index panics — see overlay.go).
	baseHeight := lipgloss.Height(dimmed)
	available := baseHeight - 1
	if available < 1 {
		available = 1
	}
	modalContent = boundModalToViewport(modalContent, available, 0)

	mw := modalWidth(m.width)
	mh := lipgloss.Height(modalContent)
	x, y := modalOrigin(m.width, baseHeight, mw, mh)
	return placeOverlay(x, y, modalContent, dimmed)
}

// modalWidth returns the clamped modal width: min(width-8, 72), floored at
// 20, mirroring tui/model.go's modalWidth.
func modalWidth(w int) int {
	mw := w - 8
	if mw > 72 {
		mw = 72
	}
	if mw < 20 {
		mw = 20
	}
	return mw
}
