package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/identity"
)

// screenModel is the interface all TUI screens must implement. Sub-models
// return strings from view() (not tea.View); the root model wraps with
// tea.NewView. This keeps sub-model helpers simple while satisfying the v2
// tea.Model contract at the root (RESEARCH.md Pattern 4, Pitfall 2).
type screenModel interface {
	update(msg tea.Msg) (screenModel, tea.Cmd)
	view() string
}

// rootModel is the top-level Bubble Tea model. It holds a view-stack of
// sub-screens and delegates Update to the top of the stack. It handles
// push/pop navigation messages directly (RESEARCH.md Pattern 4).
type rootModel struct {
	stack  []screenModel
	width  int
	height int
	deps   tuiDeps
}

// tuiDeps holds both doctor.Deps and identity.Deps for the TUI. It is built
// once by buildTUIDeps and threaded through the root model and its screens.
type tuiDeps struct {
	doctor   doctor.Deps
	identity identity.Deps
}

// newRootModel constructs the root model with the home screen (a placeholder
// dashboard stub) pre-pushed onto the stack. Downstream plans replace the
// placeholder with real screen models.
func newRootModel(docDeps doctor.Deps, idDeps identity.Deps) rootModel {
	d := tuiDeps{doctor: docDeps, identity: idDeps}
	home := &homeStubScreen{}
	return rootModel{
		stack: []screenModel{home},
		deps:  d,
	}
}

// homeStubScreen is a placeholder home screen. It is replaced by the real
// dashboard in 05-03. It implements screenModel with identity-returning stubs
// (RED-stub-under-strict-lint convention).
type homeStubScreen struct{}

func (h *homeStubScreen) update(_ tea.Msg) (screenModel, tea.Cmd) { return h, nil }
func (h *homeStubScreen) view() string                            { return "" }

// Init satisfies the tea.Model interface. It returns nil because the home stub
// screen has no async initialization. The real dashboard (05-03) replaces this.
func (m rootModel) Init() tea.Cmd {
	return nil
}

// Update handles root-level messages (WindowSizeMsg, pushScreenMsg,
// popScreenMsg) and delegates all other messages to the top of the stack.
// familyResultMsg, identityListResultMsg, preWriteResultMsg, resolvedResultMsg,
// writeResultMsg, and clipboardResultMsg are delegated to the active screen;
// they are listed here as documentation and to ensure the type-switch is
// exhaustive when sub-screens are added in subsequent plans.
func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case pushScreenMsg:
		m.stack = append(m.stack, msg.next)
		return m, nil
	case popScreenMsg:
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
		return m, nil
	case familyResultMsg:
		// Handled by the active screen (dashboard). Delegate below.
		_, _, _, _ = msg.runID, msg.family, msg.findings, msg.err
	case identityListResultMsg:
		// Handled by identity list screen. Delegate below.
		_, _ = msg.accounts, msg.err
	case preWriteResultMsg:
		// Handled by prove screen. Delegate below.
		_, _ = msg.result, msg.err
	case resolvedResultMsg:
		// Handled by prove screen. Delegate below.
		_, _ = msg.result, msg.resolved
	case writeResultMsg:
		// Handled by form screens. Delegate below.
		_, _ = msg.backupPath, msg.err
	case clipboardResultMsg:
		// Handled by copy action. Delegate below.
		_ = msg.err
	}

	if len(m.stack) == 0 {
		return m, tea.Quit
	}
	top := m.stack[len(m.stack)-1]
	updated, cmd := top.update(msg)
	m.stack[len(m.stack)-1] = updated
	return m, cmd
}

// View renders the top screen's content, wrapped in a tea.View.
// CRITICAL: returns tea.View (not string) — v2 breaking change (RESEARCH Pitfall 2).
// Alt-screen is enabled via AltScreen: true (RESEARCH "State of the Art":
// tea.WithAltScreen() is a v1 option; v2 uses the view field).
func (m rootModel) View() tea.View {
	if len(m.stack) == 0 {
		return tea.NewView("")
	}
	v := tea.NewView(m.stack[len(m.stack)-1].view())
	v.AltScreen = true
	return v
}
