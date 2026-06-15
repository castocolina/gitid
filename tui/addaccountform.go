package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// addAccountFormModel is the TUI form for adding a new host alias to an
// existing identity (D-02, Screen 5b). The identity name and key path are
// pre-filled and read-only.
//
// Editable fields:
//
//	[0] Provider   [1] SSH Alias   [2] Port   [3] Match Strategy
type addAccountFormModel struct {
	account  identity.Account
	inputs   []textinput.Model
	focusIdx int
	err      string
	deps     tuiDeps
}

var addAccountFormLabels = []string{
	"Provider",
	"SSH Alias",
	"Port",
	"Match Strategy",
}

// newAddAccountFormModel builds the Add Account form pre-filled from account.
func newAddAccountFormModel(acct identity.Account, deps tuiDeps) addAccountFormModel {
	defaults := []string{
		acct.Provider,
		identity.DefaultAlias(acct.Name, acct.Provider),
		"22",
		"gitdir:~/git/" + acct.Name + "/",
	}
	inputs := make([]textinput.Model, len(addAccountFormLabels))
	for i := range inputs {
		ti := textinput.New()
		ti.SetValue(defaults[i])
		inputs[i] = ti
	}
	_ = inputs[0].Focus()
	return addAccountFormModel{
		account:  acct,
		inputs:   inputs,
		focusIdx: 0,
		deps:     deps,
	}
}

// update handles key events for the add-account form.
func (m addAccountFormModel) update(msg tea.Msg) (screenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			return m, popCmd()
		case msg.Code == tea.KeyTab:
			return m.advanceFocus(1)
		case msg.String() == "shift+tab":
			return m.advanceFocus(-1)
		case key.Matches(msg, keys.Submit):
			if m.focusIdx == len(m.inputs)-1 {
				return m.trySubmit()
			}
			return m.advanceFocus(1)
		}
	}

	if m.focusIdx < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focusIdx], cmd = m.inputs[m.focusIdx].Update(msg)
		return m, cmd
	}
	return m, nil
}

// advanceFocus moves focus by delta, wrapping around.
func (m addAccountFormModel) advanceFocus(delta int) (screenModel, tea.Cmd) {
	m.inputs[m.focusIdx].Blur()
	m.focusIdx = ((m.focusIdx+delta)%len(m.inputs) + len(m.inputs)) % len(m.inputs)
	cmd := m.inputs[m.focusIdx].Focus()
	return m, cmd
}

// trySubmit validates inputs, then pushes the prove screen.
func (m addAccountFormModel) trySubmit() (screenModel, tea.Cmd) {
	// T-05-13: validate the account name (already set from existing identity, but verify).
	if err := identity.ValidateName(m.account.Name); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.err = ""

	port, _ := strconv.Atoi(m.inputs[2].Value())
	if port <= 0 {
		port = 22
	}

	provider := m.inputs[0].Value()
	alias := m.inputs[1].Value()

	// WR-04: parse the Match Strategy field (inputs[3]) into the includeIf
	// matches instead of silently discarding the user's gitdir scoping.
	matches := []gitconfig.Match{parseMatchStrategy(m.inputs[3].Value(), m.account.Name)}

	// Build the existing-account snapshot AddAccount writes against, overriding
	// the per-account fields the form edits (provider/alias/port/matches). The
	// shared key path and managed target paths come from the reconstructed
	// account (filled by fillAccountPaths, CR-02/CR-03).
	existing := m.account
	existing.Port = port
	existing.Matches = matches

	// CreateInput drives the prove-screen display/gate; the write dispatches via
	// identity.AddAccount(existing, provider, alias) (CR-03).
	in := identity.CreateInput{
		Name:               m.account.Name,
		GitName:            m.account.GitName,
		GitEmail:           m.account.GitEmail,
		Provider:           provider,
		Port:               port,
		Alias:              alias,
		Hostname:           m.account.Hostname,
		Matches:            matches,
		FragmentPath:       m.account.FragmentPath,
		GitconfigPath:      m.account.GitconfigPath,
		SSHConfigPath:      m.account.SSHConfigPath,
		AllowedSignersPath: m.account.AllowedSignersPath,
		Confirmed:          false,
	}
	// CR-04: phase 1 gates on the existing shared PRIVATE-KEY path.
	proveScreen := newProveScreen("add-account", in, existing, m.account.KeyPath, m.deps)
	return m, pushCmd(proveScreen)
}

// view renders the add-account form (Screen 5b layout).
func (m addAccountFormModel) view() string {
	var sb strings.Builder
	title := fmt.Sprintf("gitid — Add Account: %s", m.account.Name)
	sb.WriteString(StyleTitle.Render(title) + "\n\n")
	// Identity and key path are read-only.
	sb.WriteString(StyleFaint.Render(fmt.Sprintf("%-16s %s", "Identity:", m.account.Name)) + "\n")
	sb.WriteString(StyleFaint.Render(fmt.Sprintf("%-16s %s", "Key Path:", m.account.KeyPath)) + "\n\n")
	for i, inp := range m.inputs {
		label := addAccountFormLabels[i]
		if i == m.focusIdx {
			sb.WriteString(StyleLabel.Render(fmt.Sprintf("%-16s", label)))
			sb.WriteString(StyleInputActive.Render(inp.View()) + "\n")
		} else {
			sb.WriteString(StyleLabel.Render(fmt.Sprintf("%-16s", label)))
			sb.WriteString(inp.View() + "\n")
		}
	}
	if m.err != "" {
		sb.WriteString(StyleFinding.Render("  ! "+m.err) + "\n")
	}
	sb.WriteString("\n" + StyleFaint.Render("Tab: next field  Shift+Tab: prev field  Enter: submit  Esc: cancel"))
	return sb.String()
}
