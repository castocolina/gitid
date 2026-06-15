package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
)

// defaultAlgo is the gitid key algorithm. Ed25519 is the only supported algo
// (CLAUDE.md crypto stack); the create form does not expose an algo selector.
const defaultAlgo = "ed25519"

// createFormModel is the TUI form for creating a new identity (D-02, Screen 4).
// Fields follow the exact order from cmd/gitid/add.go gatherCreateInput():
//
//	[0] Identity Name   [1] Git Name   [2] Git Email   [3] Provider
//	[4] Port            [5] SSH Alias  [6] Match Strategy  [7] Passphrase
type createFormModel struct {
	inputs   []textinput.Model
	focusIdx int
	err      string // inline validation error for name field
	deps     tuiDeps
}

// createFormFields names each input field index.
var createFormLabels = []string{
	"Identity Name",
	"Git Name",
	"Git Email",
	"Provider",
	"Port",
	"SSH Alias",
	"Match Strategy",
	"Passphrase",
}

var createFormPlaceholders = []string{
	"e.g. personal",
	"e.g. Ramon Colina",
	"e.g. user@example.com",
	"github.com",
	"22",
	"leave blank to use provider",
	"gitdir:~/git/",
	"(empty for none)",
}

var createFormDefaults = []string{
	"",
	"",
	"",
	"github",
	"22",
	"",
	"gitdir:",
	"",
}

// newCreateFormModel builds the Create Identity form with one textinput per
// field. The name field (index 0) is focused initially.
func newCreateFormModel(deps tuiDeps) createFormModel {
	inputs := make([]textinput.Model, len(createFormLabels))
	for i := range inputs {
		ti := textinput.New()
		ti.Placeholder = createFormPlaceholders[i]
		if createFormDefaults[i] != "" {
			ti.SetValue(createFormDefaults[i])
		}
		inputs[i] = ti
	}
	_ = inputs[0].Focus()
	return createFormModel{
		inputs:   inputs,
		focusIdx: 0,
		deps:     deps,
	}
}

// newCreateFormScreen returns the create-form screenModel threaded with the
// real write seams so the confirmed create routes through identity.Deps (CR-02).
func newCreateFormScreen(deps tuiDeps) screenModel {
	return newCreateFormModel(deps)
}

// update handles key events for the create form.
func (m createFormModel) update(msg tea.Msg) (screenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			return m, popCmd()
		case msg.String() == "shift+tab":
			return m.advanceFocus(-1)
		case msg.Code == tea.KeyTab:
			return m.advanceFocus(1)
		case key.Matches(msg, keys.Submit):
			if m.focusIdx == len(m.inputs)-1 {
				return m.trySubmit()
			}
			return m.advanceFocus(1)
		}
	case tea.WindowSizeMsg:
		return m, nil
	}

	// Delegate to active textinput.
	if m.focusIdx < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focusIdx], cmd = m.inputs[m.focusIdx].Update(msg)
		return m, cmd
	}
	return m, nil
}

// advanceFocus moves focus by delta, wrapping around.
func (m createFormModel) advanceFocus(delta int) (screenModel, tea.Cmd) {
	m.inputs[m.focusIdx].Blur()
	m.focusIdx = ((m.focusIdx+delta)%len(m.inputs) + len(m.inputs)) % len(m.inputs)
	cmd := m.inputs[m.focusIdx].Focus()
	return m, cmd
}

// trySubmit validates the name field and, if valid, builds the CreateInput
// and pushes the prove screen. If invalid, sets the error message.
func (m createFormModel) trySubmit() (screenModel, tea.Cmd) {
	// T-05-13: validate name before building CreateInput.
	name := strings.TrimSpace(m.inputs[0].Value())
	if err := identity.ValidateName(name); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.err = ""

	port, _ := strconv.Atoi(m.inputs[4].Value())
	if port <= 0 {
		port = 22
	}

	provider := m.inputs[3].Value()
	alias := m.inputs[5].Value()
	if strings.TrimSpace(alias) == "" {
		alias = identity.DefaultAlias(name, provider)
	}

	// Resolve the gitid-managed target paths so the confirmed write lands in the
	// real files instead of empty/zero paths (CR-03). Mirrors buildIdentityDeps
	// path conventions; home failure falls back to relative paths (the prove
	// screen's pre-write gate will surface any resulting key-path problem).
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	keyPath, pubPath := keygen.KeyPaths(sshDir, defaultAlgo, name)

	in := identity.CreateInput{
		Name:               name,
		GitName:            m.inputs[1].Value(),
		GitEmail:           m.inputs[2].Value(),
		Provider:           provider,
		Algo:               defaultAlgo,
		Port:               port,
		Alias:              alias,
		Hostname:           provider, // SSH connectivity gate dials the provider host (CR-04)
		Passphrase:         m.inputs[7].Value(),
		Matches:            []gitconfig.Match{parseMatchStrategy(m.inputs[6].Value(), name)},
		FragmentPath:       filepath.Join(home, ".gitconfig.d", name),
		GitconfigPath:      filepath.Join(home, ".gitconfig"),
		SSHConfigPath:      filepath.Join(sshDir, "config"),
		AllowedSignersPath: filepath.Join(sshDir, "allowed_signers"),
	}
	_ = pubPath // pub path is derived by the Generate dep; retained for symmetry.

	// CR-04: the pre-write gate must run against the STAGED private key path,
	// not the ssh config path. For create the key does not exist yet; pass the
	// conventional final key path so the gate exercises the right key once
	// PersistKey has staged it (the Generate dep gates on the temp staging path).
	proveScreen := newProveScreen("create", in, identity.Account{}, keyPath, m.deps)
	return m, pushCmd(proveScreen)
}

// parseMatchStrategy parses a Match Strategy form value (e.g. "gitdir:~/git/foo/"
// or "hasconfig:remote.*.url:...") into a gitconfig.Match. A bare/empty value
// falls back to the D-13 default gitdir:~/git/<name>/. The gitconfig renderer
// normalizes the trailing slash for gitdir conditions (WR-04 / D-13).
func parseMatchStrategy(raw, name string) gitconfig.Match {
	v := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(v, "hasconfig:"):
		return gitconfig.Match{Kind: gitconfig.MatchHasconfig, Value: strings.TrimPrefix(v, "hasconfig:")}
	case strings.HasPrefix(v, "gitdir:"):
		return gitconfig.Match{Kind: gitconfig.MatchGitdir, Value: strings.TrimPrefix(v, "gitdir:")}
	case v == "" || v == "gitdir:":
		return identity.DefaultMatch(name)
	default:
		return gitconfig.Match{Kind: gitconfig.MatchGitdir, Value: v}
	}
}

// view renders the create form (Screen 4 layout).
func (m createFormModel) view() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("gitid — Create Identity") + "\n\n")
	for i, inp := range m.inputs {
		label := createFormLabels[i]
		if i == m.focusIdx {
			sb.WriteString(StyleLabel.Render(fmt.Sprintf("%-16s", label)))
			sb.WriteString(StyleInputActive.Render(inp.View()) + "\n")
		} else {
			sb.WriteString(StyleLabel.Render(fmt.Sprintf("%-16s", label)))
			sb.WriteString(inp.View() + "\n")
		}
		if i == 0 && m.err != "" {
			sb.WriteString(StyleFinding.Render("  ! "+m.err) + "\n")
		}
	}
	sb.WriteString("\n" + StyleFaint.Render("Tab: next field  Shift+Tab: prev field  Enter: submit  Esc: cancel"))
	return sb.String()
}
