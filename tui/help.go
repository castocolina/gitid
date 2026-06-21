package tui

import "strings"

// renderHelpModal renders the static keyboard-shortcuts help overlay.
// The modal is rendered at min(width-8, 72) columns per UI-SPEC § Help Overlay.
// Uses modalBox from overlay.go for consistent styling and compositing.
//
// UI-SPEC § Help Overlay shortcut table (TUI-06, closes G-02/G-03/G-04):
//
//	Key               Action
//	─────────────────────────────────────
//	q / ctrl+c        quit
//	1 / 2 / 3         switch view
//	ctrl+p            command palette
//	tab               cycle pane focus
//	shift+tab         reverse focus
//	↑/k  ↓/j          move selection
//	e                 edit (identities / global options)
//	c                 copy public key
//	a                 add identity
//	d                 delete identity (CLI)
//	R                 rotate key (CLI)
//	x                 apply fix (health view)
//	r                 refresh
//	\                 toggle sidebar (collapsed mode)
//	?                 toggle this help overlay
//	esc               close / cancel
func renderHelpModal(termW int) string {
	title := "Keyboard Shortcuts"

	rows := []struct{ key, desc string }{
		{"q / ctrl+c", "quit"},
		{"1 / 2 / 3  ←/→", "switch view (Identities / Health / Global Options)"},
		{"ctrl+p", "command palette"},
		{"tab", "cycle pane focus"},
		{"shift+tab", "reverse pane focus"},
		{"↑/k  ↓/j", "move selection"},
		{"enter", "select identity — drill into the detail pane"},
		{"e", "edit (Identities view or Global Options)"},
		{"c", "copy public key to clipboard"},
		{"a", "add identity"},
		{"d", "delete identity"},
		{"R", "new key — generate a fresh key for this identity, replacing the old one"},
		{"x", "apply fix (Health view)"},
		{"r", "refresh"},
		{"\\", "toggle sidebar (collapsed-width mode)"},
		{"?", "toggle this help overlay"},
		{"esc", "close / cancel"},
	}

	var sb strings.Builder
	for _, r := range rows {
		key := StyleHelpKey.Render(r.key)
		padW := 18 - len([]rune(r.key))
		if padW < 1 {
			padW = 1
		}
		sb.WriteString(key + strings.Repeat(" ", padW) + StyleHelpDesc.Render(r.desc) + "\n")
	}

	body := strings.TrimRight(sb.String(), "\n")
	return modalBox(termW, title, body)
}
