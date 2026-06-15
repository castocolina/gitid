package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/clipboard"
	"github.com/castocolina/gitid/internal/upload"
)

// runClipboardCopyCmd wraps clipboard.Copy in a tea.Cmd so it does not block
// Update(). The result is delivered as a clipboardResultMsg. (D-06, RESEARCH
// Pitfall 7: never block Update with I/O.)
func runClipboardCopyCmd(pubLine string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.Copy(pubLine)
		return clipboardResultMsg{err: err}
	}
}

// renderCopyOverlay builds the inline copy confirmation block (UI-SPEC §Copy
// Pubkey Action). On success it shows the pass line, key preview, and
// provider-specific upload instructions. On clipboard failure it shows an info
// advisory and prompts manual copy.
func renderCopyOverlay(pubLine, provider string, copyErr error) string {
	var s string
	if copyErr != nil {
		s += StyleFinding.Render("! clipboard copy failed [info]") + "\n"
		s += StyleFaint.Render(copyErr.Error()) + "\n"
		s += StyleFaint.Render("Key is printed above — copy manually.") + "\n"
	} else {
		s += StylePass.Render("Public key copied to clipboard.") + "\n"
	}
	s += StyleFaint.Render("Key: "+truncatePubLine(pubLine)) + "\n\n"
	s += upload.Instructions(provider)
	s += "\n" + StyleFaint.Render("Press any key to dismiss") + "\n"
	return s
}

// truncatePubLine truncates a public key line for display, keeping the key
// type prefix and enough of the key material to be recognisable.
func truncatePubLine(line string) string {
	const maxLen = 60
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen] + "..."
}
