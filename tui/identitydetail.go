package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/identity"
)

// expandTildePath expands a leading "~/" (or bare "~") in path to the user home
// directory. Reconstruction copies IdentityFile/.pub paths verbatim from
// ~/.ssh/config, which commonly carry a literal tilde that os.ReadFile cannot
// resolve (mirrors identity.expandTilde, WR-02). Paths without a leading tilde
// are returned unchanged.
func expandTildePath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[len("~/"):]), nil
}

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
// It caches the public-key line for the copy action (WR-02) by reading the
// account's .pub via the injected doctor.ReadFile seam (falling back to the
// identity ReadPub/os path semantics); an empty or unreadable .pub leaves
// pubLine empty so the copy action can guard against an empty clipboard write.
func newIdentityDetailModel(acct identity.Account, deps tuiDeps) identityDetailModel {
	return identityDetailModel{
		account: acct,
		deps:    deps,
		pubLine: readPubLineForCopy(acct.PubPath, deps),
	}
}

// newIdentityDetailScreen returns the identity detail screenModel, delegating to
// newIdentityDetailModel with the real deps so the edit/add-host write chain is
// wired (CR-02) and the copy action has a populated pubLine (WR-02).
func newIdentityDetailScreen(acct identity.Account, deps tuiDeps) screenModel {
	return newIdentityDetailModel(acct, deps)
}

// readPubLineForCopy reads and trims the public-key line at pubPath using the
// injected doctor.ReadFile seam (trusted gitid-managed .pub path, G304). It
// returns "" when pubPath is empty, ReadFile is nil (test mode), or the read
// fails — callers guard the copy action against an empty result (WR-02).
func readPubLineForCopy(pubPath string, deps tuiDeps) string {
	if pubPath == "" || deps.doctor.ReadFile == nil {
		return ""
	}
	resolved, err := expandTildePath(pubPath)
	if err != nil {
		return ""
	}
	data, err := deps.doctor.ReadFile(resolved)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\n")
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
			// Guard against an empty pubLine: copying "" to the clipboard is
			// meaningless and the overlay would render a blank key (WR-02).
			if strings.TrimSpace(m.pubLine) == "" {
				m.overlay = StyleFinding.Render("  ! no public key available to copy for "+m.account.Name) + "\n" +
					StyleFaint.Render("Press any key to dismiss") + "\n"
				return m, nil
			}
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
