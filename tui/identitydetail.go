package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/identity"
)

// identityDetailModel is the Identity Detail screen (TUI-02, Screen 3).
// It displays the full two-column metadata block per UI-SPEC and handles
// e/h/c/d/r key actions.
type identityDetailModel struct {
	account identity.Account
	deps    tuiDeps
	overlay string // inline overlay for copy confirmation or CLI handoff
	pubLine string // cached public key line for the copy action
}

// newIdentityDetailModel builds an identity detail model for the given account.
func newIdentityDetailModel(acct identity.Account, deps tuiDeps) identityDetailModel {
	return identityDetailModel{
		account: acct,
		deps:    deps,
	}
}

// newIdentityDetailScreen returns the identity detail screenModel (replaces stub from 05-03).
func newIdentityDetailScreen(acct identity.Account) screenModel {
	return identityDetailModel{account: acct}
}

// update handles key events for the identity detail screen.
func (m identityDetailModel) update(msg tea.Msg) (screenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Dismiss any overlay on any key (except the ones that produce it).
		if m.overlay != "" {
			// If overlay is shown and user presses a neutral key, dismiss it.
			switch {
			case key.Matches(msg, keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, keys.Back):
				m.overlay = ""
				return m, popCmd()
			default:
				m.overlay = ""
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			return m, popCmd()
		case key.Matches(msg, keys.Edit):
			// 'e' → push the update form.
			updateForm := newUpdateFormModel(m.account, m.deps)
			return m, pushCmd(updateForm)
		case key.Matches(msg, keys.AddHost):
			// 'H' → push the add-account form.
			addForm := newAddAccountFormModel(m.account, m.deps)
			return m, pushCmd(addForm)
		case key.Matches(msg, keys.Copy):
			// 'c' → run clipboard copy cmd (does not push a screen — D-06).
			return m, runClipboardCopyCmd(m.pubLine)
		case key.Matches(msg, keys.Delete):
			// 'd' → show CLI handoff (D-03, no write).
			m.overlay = deleteHandoffMsg(m.account.Name)
			return m, nil
		case key.Matches(msg, keys.Rotate):
			// 'R' → show rotate CLI handoff (D-03, no write).
			m.overlay = rotateHandoffMsg(m.account.Name)
			return m, nil
		}
	case clipboardResultMsg:
		// Clipboard result arrives: set the copy overlay.
		m.overlay = renderCopyOverlay(m.pubLine, m.account.Provider, msg.err)
		return m, nil
	case tea.WindowSizeMsg:
		return m, nil
	}
	return m, nil
}

// deleteHandoffMsg returns the inline CLI handoff copy for delete (D-03).
func deleteHandoffMsg(name string) string {
	var sb strings.Builder
	sb.WriteString(StyleFaint.Render("Delete and Rotate run from the CLI to preserve the full safe-write flow.") + "\n")
	sb.WriteString(StyleBody.Render(fmt.Sprintf("To delete:  gitid identity delete %s", name)) + "\n")
	sb.WriteString(StyleFaint.Render("Press any key to dismiss") + "\n")
	return sb.String()
}

// rotateHandoffMsg returns the inline CLI handoff copy for rotate (D-03).
func rotateHandoffMsg(name string) string {
	var sb strings.Builder
	sb.WriteString(StyleFaint.Render("Delete and Rotate run from the CLI to preserve the full safe-write flow.") + "\n")
	sb.WriteString(StyleBody.Render(fmt.Sprintf("To rotate:  gitid rotate %s", name)) + "\n")
	sb.WriteString(StyleFaint.Render("Press any key to dismiss") + "\n")
	return sb.String()
}

// view renders the identity detail screen (Screen 3 layout per UI-SPEC).
func (m identityDetailModel) view() string {
	var sb strings.Builder
	title := fmt.Sprintf("gitid — Identity: %s", m.account.Name)
	sb.WriteString(StyleTitle.Render(title) + "\n\n")

	// Two-column metadata block: StyleLabel (16-wide) + StyleBody/StyleFaint.
	row := func(label, value string) string {
		return StyleLabel.Render(fmt.Sprintf("%-16s", label)) + StyleBody.Render(value) + "\n"
	}
	rowFaint := func(label, value string) string {
		return StyleLabel.Render(fmt.Sprintf("%-16s", label)) + StyleFaint.Render(value) + "\n"
	}

	sb.WriteString(row("Name:", m.account.Name))
	sb.WriteString(row("Git Name:", m.account.GitName))
	sb.WriteString(row("Git Email:", m.account.GitEmail))
	sb.WriteString(row("Provider:", m.account.Provider))
	port := m.account.Port
	if port == 0 {
		port = 22
	}
	sb.WriteString(rowFaint("Port:", fmt.Sprintf("%d", port)))
	sb.WriteString(rowFaint("SSH Alias:", m.account.Alias))
	sb.WriteString(rowFaint("Key Path:", m.account.KeyPath))
	if m.account.Incomplete != "" {
		sb.WriteString(StyleFinding.Render("  ~ incomplete: "+m.account.Incomplete) + "\n")
	}

	if m.overlay != "" {
		sb.WriteString("\n" + m.overlay)
	} else {
		sb.WriteString("\n" + StyleFaint.Render("e: edit  H: add host  c: copy pubkey  d: delete (CLI)  R: rotate (CLI)  Esc: back"))
	}
	return sb.String()
}
