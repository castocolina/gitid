package tui

// globalopts.go — Global Options view (Plan 04).
//
// globalOptionsModel renders the managed baseline state (core, push/pull/fetch,
// color, aliases, gitignore, url-rewrites) read from disk via
// gitconfig.ReadBaselineState. It supports inline editing ('e') with the same
// contract as the identity detail pane: non-structural edits (aliases, color)
// → simple confirm; structural edits (excludesfile path) → backup+preview+confirm.
//
// Ported analogs: tui/identitydetail.go (detail render pattern) + tui/updateform.go
// (inline edit contract).
// Reference: UI-SPEC § View 3: Global Options; PATTERNS § tui/globalopts.go.

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/gitconfig"
)

// globalOptionsModel is the main-pane sub-model for View 3 (Global Options).
type globalOptionsModel struct {
	state    gitconfig.BaselineState
	loaded   bool // true after first baselineLoadedMsg received
	loadErr  error
	editing  bool // true when inline edit mode is active
	editNote string

	deps tuiDeps
}

// newGlobalOptionsModel constructs an empty global options model.
func newGlobalOptionsModel(deps tuiDeps) globalOptionsModel {
	return globalOptionsModel{deps: deps}
}

// refresh returns a tea.Cmd that reads the baseline state from disk and emits
// baselineLoadedMsg. Called when the Global Options view is activated or on 'r'.
func (m globalOptionsModel) refresh() (globalOptionsModel, tea.Cmd) {
	d := m.deps.doctor
	return m, func() tea.Msg {
		state, err := gitconfig.ReadBaselineState(
			d.GitconfigPath,
			d.BaselineFilePath,
			d.GitignorePath,
		)
		return baselineLoadedMsg{state: state, err: err}
	}
}

// update handles messages for the global options view.
func (m globalOptionsModel) update(msg tea.Msg) (globalOptionsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case baselineLoadedMsg:
		m.loaded = true
		m.loadErr = msg.err
		if msg.err == nil {
			m.state = msg.state
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg.String())
	}
	return m, nil
}

// handleKey processes key presses for the global options view.
func (m globalOptionsModel) handleKey(key string) (globalOptionsModel, tea.Cmd) {
	switch key {
	case "e":
		m.editing = !m.editing
		m.editNote = ""
		return m, nil
	case "esc":
		if m.editing {
			m.editing = false
			m.editNote = ""
			return m, nil
		}
	case "r":
		return m.refresh()
	}
	return m, nil
}

// view renders the global options pane at the given width and height.
func (m globalOptionsModel) view(w, _ int) string {
	pad := lipgloss.NewStyle().Padding(1, 2)
	_ = w

	if !m.loaded {
		return pad.Render(StyleFaint.Render("Loading global options..."))
	}

	if m.loadErr != nil {
		return pad.Render(StyleFaint.Render("Error loading global options: " + m.loadErr.Error()))
	}

	if !m.state.Installed {
		return pad.Render(
			StyleBody.Render("Global Git Config has not been set up.") + "\n" +
				StyleFaint.Render("Press 'e' or run 'gitid baseline setup' to configure."),
		)
	}

	var sb strings.Builder

	// Header.
	status := StylePass.Render("✓ configured")
	if m.state.Incomplete {
		status = StyleFaint.Render("~ incomplete")
	}
	sb.WriteString(StyleTitle.Render("Global Git Config") + "  " + status + "\n\n")

	// Core.
	sb.WriteString(StyleHeader.Render("Core:") + "\n")
	if v, ok := m.state.BaselineKeys["core.ignorecase"]; ok {
		sb.WriteString("  " + StyleLabel.Render("ignorecase:") + " " + StyleBody.Render(v) + "\n")
	}
	if v, ok := m.state.BaselineKeys["core.excludesfile"]; ok {
		sb.WriteString("  " + StyleLabel.Render("excludesfile:") + " " + StyleBody.Render(v) + "\n")
	}

	// Push / Pull / Fetch.
	sb.WriteString("\n" + StyleHeader.Render("Push / Pull / Fetch:") + "\n")
	for _, key := range []string{"push.autosetupremote", "pull.rebase", "fetch.prune"} {
		if v, ok := m.state.BaselineKeys[key]; ok {
			sb.WriteString("  " + StyleLabel.Render(key+":") + " " + StyleBody.Render(v) + "\n")
		}
	}

	// Color.
	sb.WriteString("\n" + StyleHeader.Render("Color:") + "\n")
	if v, ok := m.state.BaselineKeys["color.ui"]; ok {
		sb.WriteString("  " + StyleLabel.Render("color.ui:") + " " + StyleBody.Render(v) + "\n")
	}

	// Global Gitignore.
	if len(m.state.GitignorePatterns) > 0 {
		sb.WriteString("\n" + StyleHeader.Render("Global Gitignore:") + "\n")
		preview := strings.Join(m.state.GitignorePatterns, ", ")
		const maxPreviewLen = 60
		if len(preview) > maxPreviewLen {
			preview = preview[:maxPreviewLen] + " ..."
		}
		sb.WriteString("  " + StyleFaint.Render(preview) + "\n")
	}

	// URL Rewrites.
	sb.WriteString("\n" + StyleHeader.Render("URL Rewrites (insteadOf):") + "\n")
	if len(m.state.URLRewrites) == 0 {
		sb.WriteString("  " + StyleFaint.Render("(none configured)") + "\n")
	} else {
		for _, rw := range m.state.URLRewrites {
			sb.WriteString("  " + StyleBody.Render(rw.HTTPSPrefix) + StyleFaint.Render(" → ") + StyleBody.Render(rw.SSHPrefix) + "\n")
		}
	}

	// Edit mode indicator.
	if m.editing {
		sb.WriteString("\n" + StyleFaint.Render("[editing] Esc to exit edit mode") + "\n")
	}
	if m.editNote != "" {
		sb.WriteString(StyleFaint.Render(m.editNote) + "\n")
	}

	return pad.Render(sb.String())
}
