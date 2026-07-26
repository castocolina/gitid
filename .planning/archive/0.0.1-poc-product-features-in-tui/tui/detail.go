package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// detailField describes one editable field in the identity detail pane.
type detailField struct {
	label      string // e.g. "Name", "Git Email", "Alias"
	input      textinput.Model
	structural bool // true when this field is structural (alias/hostname/port/match strategy)
}

// identityDetailModel is the main-pane sub-model for View 1 (Identities detail).
//
// It renders the selected identity's fields per UI-SPEC View 1, supports inline
// editing ('e'), match-strategy live preview (D-06), and routes structural edits
// through the prove-before-write loop (D-07, T-05.6-10).
type identityDetailModel struct {
	account *identity.Account // nil = empty state

	// Inline-edit state (D-05).
	inlineEditMode bool
	editFields     []detailField
	focusedField   int

	// proveModalPending is set true when the user presses Enter on a structural
	// field. The root model reads this flag each cycle and opens proveModal.
	proveModalPending bool
	// editConfirmPending is set true when Enter is pressed on a non-structural field.
	editConfirmPending bool

	// signed tracks the current signing state for the account.
	signed bool

	deps tuiDeps
}

// newIdentityDetailModel constructs an empty detail model.
func newIdentityDetailModel() identityDetailModel {
	return identityDetailModel{}
}

// handleKey processes key presses for the detail pane.
// Returns the updated model and an optional command.
func (m identityDetailModel) handleKey(key string) (identityDetailModel, tea.Cmd) {
	if !m.inlineEditMode {
		switch key {
		case "e":
			if m.account == nil {
				return m, nil
			}
			m.inlineEditMode = true
			m.proveModalPending = false
			m.editConfirmPending = false
			m.editFields = buildDetailEditFields(*m.account)
			m.focusedField = 0
			if len(m.editFields) > 0 {
				m.editFields[0].input.Focus()
			}
			return m, nil
		}
		return m, nil
	}

	// Inside inline-edit mode.
	switch key {
	case "esc":
		m.inlineEditMode = false
		m.editFields = nil
		m.focusedField = 0
		m.proveModalPending = false
		m.editConfirmPending = false
		return m, nil

	case "tab":
		if len(m.editFields) == 0 {
			return m, nil
		}
		// Blur current.
		m.editFields[m.focusedField].input.Blur()
		// Advance.
		m.focusedField = (m.focusedField + 1) % len(m.editFields)
		m.editFields[m.focusedField].input.Focus()
		return m, nil

	case "shift+tab":
		if len(m.editFields) == 0 {
			return m, nil
		}
		m.editFields[m.focusedField].input.Blur()
		m.focusedField = (m.focusedField - 1 + len(m.editFields)) % len(m.editFields)
		m.editFields[m.focusedField].input.Focus()
		return m, nil

	case "enter":
		if len(m.editFields) == 0 || m.account == nil {
			return m, nil
		}
		focused := m.editFields[m.focusedField]
		if isStructuralField(focused.label) {
			// Structural field: signal the root model to open the prove loop.
			m.proveModalPending = true
			return m, nil
		}
		// Non-structural field: signal the root model for a simple confirm.
		m.editConfirmPending = true
		return m, nil
	}

	// Forward key to focused text input.
	if len(m.editFields) > 0 && m.focusedField < len(m.editFields) {
		var cmd tea.Cmd
		m.editFields[m.focusedField].input, cmd = m.editFields[m.focusedField].input.Update(
			tea.KeyPressMsg{Code: []rune(key)[0]},
		)
		return m, cmd
	}

	return m, nil
}

// view renders the detail pane at the given width.
func (m identityDetailModel) view(w int) string {
	pad := lipgloss.NewStyle().Padding(1, 2)
	_ = w

	if m.account == nil {
		return pad.Render(
			StyleBody.Render("Select an identity from the sidebar to view details."),
		)
	}

	acct := *m.account
	var sb strings.Builder

	// Header: name (bold) + health badge placeholder (top right).
	sb.WriteString(StyleTitle.Render(acct.Name))
	sb.WriteString("\n\n")

	// Field rows: StyleLabel (16-col) + StyleBody value.
	sb.WriteString(StyleLabel.Render("Name:") + " " + StyleBody.Render(acct.Name) + "\n")
	sb.WriteString(StyleLabel.Render("Git Name:") + " " + StyleBody.Render(acct.GitName) + "\n")

	if m.inlineEditMode {
		// Show editable fields, each as a single fixed-width bordered box with
		// the label vertically centered beside it (P0-1: no more offset boxes).
		for i, f := range m.editFields {
			sb.WriteString(renderFormField(f.label+":", f.input.View(), i == m.focusedField))
			sb.WriteString("\n")
		}
		sb.WriteString("\n" + StyleFaint.Render("[editing] Tab/Shift+Tab cycle fields · Enter commit · Esc cancel") + "\n")
	} else {
		sb.WriteString(StyleLabel.Render("Git Email:") + " " + StyleBody.Render(acct.GitEmail) + "\n")
		sb.WriteString(StyleLabel.Render("Provider:") + " " + StyleBody.Render(acct.Provider) + "\n")
		portStr := "22"
		if acct.Port != 0 {
			portStr = itoa(acct.Port)
		}
		sb.WriteString(StyleLabel.Render("Port:") + " " + StyleBody.Render(portStr) + "\n")
	}

	// Aliases / Sites block.
	sb.WriteString("\n")
	sb.WriteString(StyleHeader.Render("Aliases / Sites:") + "\n")
	if len(acct.Matches) == 0 {
		if acct.Alias != "" {
			sb.WriteString("• " + StyleBody.Render(acct.Alias) + "\n")
		} else {
			sb.WriteString(StyleFaint.Render("  (none configured)") + "\n")
		}
	} else {
		for _, match := range acct.Matches {
			condStr := conditionString(match)
			sb.WriteString("• " + StyleBody.Render(acct.Alias) + "     " + StyleFaint.Render(condStr) + "\n")
		}
	}

	// Signing.
	sb.WriteString("\n")
	signingStr := "disabled"
	if m.signed {
		signingStr = "enabled"
	}
	sb.WriteString(StyleLabel.Render("Signing:") + " " + StyleBody.Render(signingStr) + "\n")

	// Key path (faint).
	sb.WriteString(StyleLabel.Render("Key Path:") + " " + StyleFaint.Render(acct.KeyPath) + "\n")

	// Match-strategy live preview when in edit mode and match-strategy field is focused.
	if m.inlineEditMode {
		preview := m.renderMatchPreview()
		if preview != "" {
			sb.WriteString("\n")
			sb.WriteString(preview)
		}
	}

	return pad.Render(sb.String())
}

// conditionString returns the human-readable match condition for a gitconfig.Match.
func conditionString(m gitconfig.Match) string {
	switch m.Kind {
	case gitconfig.MatchGitdir:
		return "gitdir:" + m.Value
	case gitconfig.MatchHasconfig:
		return "hasconfig:" + m.Value
	default:
		return m.Value
	}
}

// renderMatchPreview renders the live [includeIf "..."] preview for the
// match-strategy field (D-06). Returns an empty string when not applicable.
func (m identityDetailModel) renderMatchPreview() string {
	if !m.inlineEditMode || m.account == nil {
		return ""
	}
	// Find the match-strategy field.
	for i, f := range m.editFields {
		if f.label != "Match Strategy" {
			continue
		}
		if i != m.focusedField {
			return ""
		}
		// Get the current gitdir value from the account's matches.
		gitdirVal := "~/git/" + m.account.Name + "/"
		for _, match := range m.account.Matches {
			if match.Kind == gitconfig.MatchGitdir {
				gitdirVal = match.Value
				break
			}
		}

		fragPath := m.account.FragmentPath
		if fragPath == "" {
			fragPath = "~/.gitconfig.d/" + m.account.Name
		}

		// Build the includeIf preview using the existing gitconfig renderer.
		matches := []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: gitdirVal}}
		preview := gitconfig.RenderIncludeIf(m.account.Name, fragPath, matches)

		indent := lipgloss.NewStyle().PaddingLeft(4)
		return StyleFaint.Render("Preview:") + "\n" + indent.Render(preview) + "\n"
	}
	return ""
}

// buildDetailEditFields constructs the textinput.Model slice for inline editing.
// Fields follow the order: Git Email, Provider, Port, Alias, Match Strategy.
// The Alias and Port fields are structural.
func buildDetailEditFields(acct identity.Account) []detailField {
	mkInput := func(placeholder, value string, charLimit int) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.SetValue(value)
		ti.SetWidth(formFieldWidth) // fixed width → single, aligned border (P0-1)
		if charLimit > 0 {
			ti.CharLimit = charLimit
		}
		return ti
	}

	portStr := "22"
	if acct.Port > 0 {
		portStr = fmt.Sprintf("%d", acct.Port)
	}

	// Derive current gitdir and URL from existing matches.
	gitdirVal := "~/git/" + acct.Name + "/"
	for _, m := range acct.Matches {
		if m.Kind == gitconfig.MatchGitdir {
			gitdirVal = m.Value
			break
		}
	}

	return []detailField{
		{label: "Git Email", input: mkInput("e.g. user@example.com", acct.GitEmail, 200), structural: false},
		{label: "Provider", input: mkInput("e.g. github.com", acct.Provider, 200), structural: false},
		{label: "Alias", input: mkInput("e.g. personal.github.com", acct.Alias, 200), structural: true},
		{label: "Port", input: mkInput("22", portStr, 10), structural: true},
		{label: "Match Strategy", input: mkInput("gitdir value", gitdirVal, 500), structural: true},
	}
}

// isStructuralField reports whether a field name is structural (changes that
// affect SSH resolution or includeIf match routing).
// Structural fields: Alias, Hostname, Port, Match Strategy.
func isStructuralField(label string) bool {
	switch label {
	case "Alias", "Hostname", "Port", "Match Strategy":
		return true
	default:
		return false
	}
}

// editedAccountFromFields reconstructs an identity.Account from the current
// edit field values, overlaying the original account's immutable fields.
func (m identityDetailModel) editedAccountFromFields() identity.Account {
	if m.account == nil {
		return identity.Account{}
	}
	edited := *m.account
	for _, f := range m.editFields {
		val := f.input.Value()
		switch f.label {
		case "Git Email":
			edited.GitEmail = val
		case "Provider":
			edited.Provider = val
		case "Alias":
			edited.Alias = val
		case "Port":
			port := 22
			if _, err := fmt.Sscanf(val, "%d", &port); err != nil {
				port = 22
			}
			edited.Port = port
		case "Match Strategy":
			// Strategy field holds the gitdir value; rebuild matches.
			edited.Matches = []gitconfig.Match{
				{Kind: gitconfig.MatchGitdir, Value: val},
			}
		}
	}
	return edited
}
