package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/identity"
)

// updateFormModel is the TUI form for updating an existing identity (D-02,
// Screen 5). The identity name is read-only (not focusable).
//
// Fields (all except name are editable):
//
//	[0] Git Name   [1] Git Email   [2] Provider   [3] Port   [4] SSH Alias
type updateFormModel struct {
	account  identity.Account
	inputs   []textinput.Model
	focusIdx int
	err      string
	deps     tuiDeps
}

var updateFormLabels = []string{
	"Git Name",
	"Git Email",
	"Provider",
	"Port",
	"SSH Alias",
}

// newUpdateFormModel builds the Update Identity form pre-filled from account.
func newUpdateFormModel(acct identity.Account, deps tuiDeps) updateFormModel {
	portStr := fmt.Sprintf("%d", acct.Port)
	if acct.Port == 0 {
		portStr = "22"
	}
	defaults := []string{acct.GitName, acct.GitEmail, acct.Provider, portStr, acct.Alias}
	inputs := make([]textinput.Model, len(updateFormLabels))
	for i := range inputs {
		ti := textinput.New()
		ti.SetValue(defaults[i])
		inputs[i] = ti
	}
	_ = inputs[0].Focus()
	return updateFormModel{
		account:  acct,
		inputs:   inputs,
		focusIdx: 0,
		deps:     deps,
	}
}

// update handles key events for the update form.
func (m updateFormModel) update(msg tea.Msg) (screenModel, tea.Cmd) {
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
func (m updateFormModel) advanceFocus(delta int) (screenModel, tea.Cmd) {
	m.inputs[m.focusIdx].Blur()
	m.focusIdx = ((m.focusIdx+delta)%len(m.inputs) + len(m.inputs)) % len(m.inputs)
	cmd := m.inputs[m.focusIdx].Focus()
	return m, cmd
}

// trySubmit validates and builds the updated account, then pushes the prove screen.
func (m updateFormModel) trySubmit() (screenModel, tea.Cmd) {
	port, _ := strconv.Atoi(m.inputs[3].Value())
	if port <= 0 {
		port = 22
	}
	updated := m.account
	updated.GitName = m.inputs[0].Value()
	updated.GitEmail = m.inputs[1].Value()
	updated.Provider = m.inputs[2].Value()
	updated.Port = port
	updated.Alias = m.inputs[4].Value()

	// Build a CreateInput for the prove-screen display/gate; the actual write
	// dispatches through identity.Update with the edited Account (CR-03).
	in := identity.CreateInput{
		Name:               updated.Name,
		GitName:            updated.GitName,
		GitEmail:           updated.GitEmail,
		Provider:           updated.Provider,
		Port:               updated.Port,
		Alias:              updated.Alias,
		Hostname:           updated.Hostname,
		Matches:            updated.Matches,
		FragmentPath:       updated.FragmentPath,
		GitconfigPath:      updated.GitconfigPath,
		SSHConfigPath:      updated.SSHConfigPath,
		AllowedSignersPath: updated.AllowedSignersPath,
		Confirmed:          false,
	}
	// CR-04: phase 1 gates on the existing PRIVATE-KEY path, not the ssh config.
	proveScreen := newProveScreen("update", in, updated, updated.KeyPath, m.deps)
	return m, pushCmd(proveScreen)
}

// view renders the update form (Screen 5 layout).
func (m updateFormModel) view() string {
	var sb strings.Builder
	title := fmt.Sprintf("gitid — Update Identity: %s", m.account.Name)
	sb.WriteString(StyleTitle.Render(title) + "\n\n")
	// Name is read-only.
	sb.WriteString(StyleFaint.Render(fmt.Sprintf("%-16s %s", "Name (immutable):", m.account.Name)) + "\n\n")
	for i, inp := range m.inputs {
		label := updateFormLabels[i]
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
