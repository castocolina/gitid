package tui

// copy.go — Copy-public-key modal (Plan 05).
//
// copyPubkeyModel displays the public key, copies it to the clipboard, and shows
// provider-specific upload instructions. The private key is NEVER copied —
// it is displayed as a faint read-only path only (D-13, T-05.6-14, locked).
//
// Security invariant (D-13, locked):
// The ONLY value passed to clipboard.Copy is the .pub line (pubLine field).
// The private key path (privKeyPath) is displayed as faint text only.
// TestCopyNeverTouchesPrivateKey asserts this invariant by construction.

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/clipboard"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/upload"
)

// copyPubkeyModel is the copy-public-key modal sub-model.
//
// It displays the public key (truncated), copies it to the system clipboard,
// and shows provider-specific upload instructions. The private key path is
// displayed as faint text only — it is NEVER passed to clipboard.Copy.
type copyPubkeyModel struct {
	// pubLine is the SSH public key line (.pub content). This is the ONLY value
	// ever sent to clipboard.Copy (D-13 security invariant).
	pubLine string

	// privKeyPath is the private key file path. Displayed as faint text only.
	// NEVER passed to clipboard.Copy or any write operation.
	privKeyPath string

	// provider is the hosting provider (github.com, gitlab.com, etc.) used to
	// select the correct upload.Instructions template.
	provider string

	// copied is true after init() dispatched the clipboard copy cmd.
	copied bool

	// copyErr is non-nil when the clipboard copy failed. The modal degrades
	// gracefully: shows the key for manual copy (CLIP-02).
	copyErr error

	deps tuiDeps
}

// newCopyPubkeyModel constructs a copyPubkeyModel for the given identity.
// pubLine is the .pub content (the ONLY value that will be copied to clipboard).
// privKeyPath is the private key path — displayed as faint text, never copied.
func newCopyPubkeyModel(pubLine, privKeyPath, provider string, deps tuiDeps) copyPubkeyModel {
	if provider == "" {
		provider = "github.com"
	}
	return copyPubkeyModel{
		pubLine:     pubLine,
		privKeyPath: privKeyPath,
		provider:    provider,
		deps:        deps,
	}
}

// init dispatches the initial clipboard copy cmd and returns the updated model.
func (m copyPubkeyModel) init() (copyPubkeyModel, tea.Cmd) {
	m.copied = true
	// Security invariant: copy ONLY m.pubLine — never m.privKeyPath.
	return m, runClipboardCopyCmd(m.pubLine)
}

// update handles messages for the copy modal.
func (m copyPubkeyModel) update(msg tea.Msg) (copyPubkeyModel, tea.Cmd) {
	switch msg := msg.(type) {

	case clipboardResultMsg:
		m.copyErr = msg.err
		return m, nil

	case tea.KeyPressMsg:
		key := msg.String()
		switch key {
		case "c":
			// Copy again — same pubLine only, never the private key.
			return m, runClipboardCopyCmd(m.pubLine)
		case "esc":
			return m, clearModalCmd()
		}
	}

	return m, nil
}

// view renders the copy modal at the given terminal width.
func (m copyPubkeyModel) view(w int) string {
	mw := modalWidth(w)
	var sb strings.Builder

	// Title.
	sb.WriteString(StyleModalTitle.Render("Copy Public Key"))
	sb.WriteString("\n\n")

	// Clipboard status line.
	if m.copied && m.copyErr != nil {
		// Check for the specific ErrNoClipboard case.
		sb.WriteString(SeverityStyle(doctor.SeverityInfo).Render(
			"! clipboard copy failed [info] — key printed above, copy manually.",
		))
	} else if m.copied {
		sb.WriteString(StylePass.Render("Public key copied to clipboard."))
	} else {
		sb.WriteString(StyleBody.Render("Public key:"))
	}
	sb.WriteString("\n\n")

	// Truncated public key + "[c] copy again" hint.
	sb.WriteString(StyleFaint.Render(truncatePubLine(m.pubLine)))
	sb.WriteString("    ")
	sb.WriteString(StyleFaint.Render("[c] copy again"))
	sb.WriteString("\n\n")

	// Upload instructions.
	providerHost := strings.SplitN(m.provider, ":", 2)[0]
	instructions := upload.Instructions(providerHost)
	sb.WriteString(StyleBody.Render(instructions))
	sb.WriteString("\n")

	// Private key path line (faint, display only — NEVER COPIED).
	sb.WriteString(StyleFaint.Render("Private key path: " + m.privKeyPath + "  (never copied)"))
	sb.WriteString("\n")

	return StyleModal.Width(mw).Render(sb.String())
}

// runClipboardCopyCmd constructs the async tea.Cmd that copies pubLine to the
// system clipboard. The ONLY value passed to clipboard.Copy is pubLine.
// Per PATTERNS § tui/copy.go Pattern H — keep verbatim.
//
// Security invariant (D-13, T-05.6-14): this function receives pubLine only;
// the private key path is never in scope here.
func runClipboardCopyCmd(pubLine string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.Copy(pubLine)
		return clipboardResultMsg{err: err}
	}
}

// truncatePubLine truncates a public key line to at most 60 characters,
// appending "..." when truncated. Verbatim from PATTERNS § tui/copy.go.
func truncatePubLine(line string) string {
	const maxLen = 60
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen] + "..."
}
