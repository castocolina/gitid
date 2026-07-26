package tui

import "charm.land/bubbles/v2/key"

// keyMap holds all shared key bindings for the TUI (D-13). Screens share
// these bindings so they appear consistently in the help bar and handle
// key presses uniformly.
//
// Tab routing note (Pitfall 9): Focus (pane cycle) and Next (form field) both
// bind "tab". The root model routes by activeModal != noModal — when a modal
// is open, "tab" goes to the active sub-model's Next binding; otherwise it
// goes to Focus (pane cycle). Plan 02 wires this routing.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Select  key.Binding
	Back    key.Binding
	Quit    key.Binding
	Help    key.Binding
	Refresh key.Binding
	Add     key.Binding
	Edit    key.Binding
	Copy    key.Binding
	Delete  key.Binding
	Rotate  key.Binding
	AddHost key.Binding
	Next    key.Binding // Tab — form field navigation; also used for Focus when no modal is open
	Prev    key.Binding // Shift+Tab
	Submit  key.Binding
	Confirm key.Binding
	Top     key.Binding
	Bottom  key.Binding

	// --- New bindings added for Phase 5.6 two-pane + modal architecture ---
	// Source: UI-SPEC § Keymap Contract (D-13).

	// Palette opens the Ctrl+P command palette (D-14).
	Palette key.Binding

	// View1/View2/View3 switch the active view via numeric keys (D-04/TUI-04).
	View1 key.Binding
	View2 key.Binding
	View3 key.Binding

	// SidebarToggle shows/hides the sidebar in collapsed single-pane mode (D-03).
	// Bound to backslash (\).
	SidebarToggle key.Binding

	// Focus cycles forward through panes (Tab). Distinct from Next (form field)
	// but shares the "tab" key — routing is by activeModal != noModal (Pitfall 9).
	Focus key.Binding

	// FocusRev cycles backward through panes (Shift+Tab).
	FocusRev key.Binding

	// Fix triggers an in-app doctor fix on the focused fixable finding (x).
	Fix key.Binding

	// Retry re-runs the prove-before-write loop from a failure state (r).
	Retry key.Binding

	// Skip skips the current prove phase and continues (s).
	Skip key.Binding

	// --- Phase 5.7 extended bindings (Plan 07: Adopt modal + Add Repo modal) ---
	// Source: UI-SPEC § Keymap Contract (Plan 07 extension, ADOPT-01, REPO-01).

	// Adopt opens the Adopt modal for the focused kindFragment unmanaged row (A).
	Adopt key.Binding

	// AddRepo opens the Add Repo clone modal from the Identities view (ctrl+r).
	AddRepo key.Binding
}

// keys is the shared keymap instance used by all screens. Bindings follow
// the UI-SPEC Keymap Contract (D-13): arrows + vim j/k/h/l navigation,
// global q/ctrl+c/esc/enter/?/r actions, and per-screen a/e/c/d/h actions.
var keys = keyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
	Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
	Select:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Add:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
	Edit:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Copy:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy pubkey")),
	Delete:  key.NewBinding(key.WithKeys("d", "delete"), key.WithHelp("d", "delete (CLI)")),
	Rotate:  key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "new key")),
	AddHost: key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "add host")),
	Next:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
	Prev:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev field")),
	Submit:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
	Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm write")),
	Top:     key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom:  key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),

	// Phase 5.6 extended bindings (UI-SPEC § Keymap Contract):
	Palette:       key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "palette")),
	View1:         key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "identities")),
	View2:         key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "health")),
	View3:         key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "global options")),
	SidebarToggle: key.NewBinding(key.WithKeys("\\"), key.WithHelp("\\", "toggle sidebar")),
	Focus:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus pane")),
	FocusRev:      key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "focus prev pane")),
	Fix:           key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "fix")),
	Retry:         key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "retry")),
	Skip:          key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "skip")),

	// Phase 5.7 extended bindings (Plan 07, ADOPT-01, REPO-01):
	Adopt:   key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "adopt fragment")),
	AddRepo: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "add repo")),
}
