package dummytui

import (
	"strings"

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

// currentViewport is the REAL terminal geometry, mirrored here (in addition
// to Model.width/height) so surface files that self-composite an
// INTRA-surface modal (surface_identitymanager.go's imOverlay — the
// action-menu/clone-name-prompt/delete-choice/confirm-destructive/
// backup-notice screens) can center/bound against the actual terminal
// instead of the fixed defaultWidth/defaultHeight capture canvas (review
// HIGH-1 / HI-01: on a real 80x24 terminal, centering against the 100x30
// default canvas positions the modal's right edge six columns past the
// real edge, clipping every line and never closing the border).
//
// Update() below sets this on every tea.WindowSizeMsg, exactly mirroring
// how it sets m.width/m.height. It is seeded at defaultWidth/defaultHeight
// (model.go's own pre-first-resize default) so every STATIC capture caller
// that never sends a WindowSizeMsg — RenderScreen (registry.go),
// screenshot-tui-mockups, design_capture_test.go, manifest_test.go — keeps
// today's exact deterministic 100x30 capture geometry (D-04) unchanged.
// Only Model's LIVE navigation path (a real running terminal, cmd/gitid-dummy)
// ever mutates it.
var currentViewport = struct{ w, h int }{defaultWidth, defaultHeight}

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
		currentViewport.w, currentViewport.h = msg.Width, msg.Height
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

	// Pad the dimmed background to the ACTUAL terminal height (m.height)
	// before compositing, mirroring tui/model.go's renderPersistentLayout —
	// the real product always renders its body to exactly `m.height - 2`
	// rows (see contentH there), so its background never runs out of rows
	// for placeOverlay to draw into. The dummy's renderShell (shell.go) is
	// NOT height-aware — it renders exactly as many rows as a screen's
	// natural content needs, which was indistinguishable from "pad to
	// m.height" while every registered screen was a short single-line
	// placeholder (02-02/02-03), but silently truncates any modal taller
	// than the PARENT surface's current natural height once a real
	// surface (e.g. create-flow, 02-04) is registered: placeOverlay can
	// only write into rows that already exist in bg (see placeOverlay's
	// `bgRow >= len(bgLines)` clamp in overlay.go), so a background with
	// fewer physical rows than the terminal silently ate the tail of any
	// taller modal (and, when the background was shorter than the modal
	// height even after boundModalToViewport, pushed the visible origin up
	// to row 0, overwriting the header instead of the body). padToHeight is
	// the minimal, additive fix: it does not change baseLayout's own
	// rendering, only pads copies used for the modal-compositing path.
	dimmed = padToHeight(dimmed, m.height)

	modalContent := modalScr.Render()

	// Reserve a vertical margin (mirroring tui/model.go's verticalMargin=4
	// for header+footer+margin) so the modal origin is never flush with the
	// terminal edge, and measure against the REAL terminal height — not the
	// parent surface's natural (possibly much shorter) content height.
	const verticalMargin = 4
	available := m.height - verticalMargin
	if available < 1 {
		available = 1
	}
	modalContent = boundModalToViewport(modalContent, available, 0)

	mw := modalWidth(m.width)
	mh := lipgloss.Height(modalContent)
	x, y := modalOrigin(m.width, m.height, mw, mh)
	return placeOverlay(x, y, modalContent, dimmed)
}

// padToHeight appends blank lines to s until it has at least height rows
// (split on "\n"), leaving s unchanged if it already has enough. Rows are
// not padded to a fixed WIDTH — overlayLine (overlay.go) already handles
// per-row width padding when compositing at a given column.
func padToHeight(s string, height int) string {
	lines := strings.Split(s, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
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
