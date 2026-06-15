package tui

import "charm.land/bubbles/v2/key"

// keyMap holds all shared key bindings for the TUI (D-13). Screens share
// these bindings so they appear consistently in the help bar and handle
// key presses uniformly.
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
	Next    key.Binding // Tab
	Prev    key.Binding // Shift+Tab
	Submit  key.Binding
	Confirm key.Binding
	Top     key.Binding
	Bottom  key.Binding
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
	Rotate:  key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rotate (CLI)")),
	AddHost: key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "add host")),
	Next:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
	Prev:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev field")),
	Submit:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
	Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm write")),
	Top:     key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom:  key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
}
