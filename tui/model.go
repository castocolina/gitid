package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
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

// initializer is the optional interface a pushed screen implements when it needs
// its init() invoked on push (CR-01). The prove screen relies on init() to issue
// phase 1 (runPreWriteCmd); without this hook a pushed prove screen sits in
// provePhase1Running forever because nothing in the live program ever calls its
// init(). The push handler invokes initScreen() when the next screen implements
// this. initScreen returns the (possibly updated) screen plus its startup cmd.
type initializer interface {
	initScreen() (screenModel, tea.Cmd)
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

// tuiDeps holds doctor.Deps, identity.Deps (create/add-account write path), and
// identity.UpdateDeps (in-place edit write path) for the TUI. It is built once
// by buildTUIDeps and threaded through the root model and its screens so every
// write screen receives the real, filewriter-backed seams (CR-02).
type tuiDeps struct {
	doctor   doctor.Deps
	identity identity.Deps
	update   identity.UpdateDeps
	// readFragment reads a per-identity gitconfig fragment so the update path can
	// PRESERVE the existing signing state instead of inferring it from the
	// presence of an email (FIX-1, mirrors cmd/gitid/update.go currentSigning).
	// Defaults to gitconfig.ReadFragment; injectable for tests.
	readFragment func(fragPath string) (gitconfig.FragmentInfo, error)
}

// newRootModel constructs the root model with the doctor dashboard as the
// home screen pre-pushed onto the stack (TUI-01). The dashboard's async
// family cmds are started by Init(). The full tuiDeps (doctor + identity +
// update) is threaded from here through every screen that performs a write.
func newRootModel(docDeps doctor.Deps, idDeps identity.Deps, upDeps identity.UpdateDeps) rootModel {
	d := tuiDeps{
		doctor:       docDeps,
		identity:     idDeps,
		update:       upDeps,
		readFragment: gitconfig.ReadFragment,
	}
	home := newDashboardModel(d)
	return rootModel{
		stack: []screenModel{home},
		deps:  d,
	}
}

// Init satisfies the tea.Model interface. It delegates to the dashboard's
// init() to start the Batch of 7 async per-family tea.Cmds (D-09, TUI-01).
func (m rootModel) Init() tea.Cmd {
	if len(m.stack) == 0 {
		return nil
	}
	if dash, ok := m.stack[0].(dashboardModel); ok {
		updated, cmd := dash.init()
		m.stack[0] = updated
		return cmd
	}
	return nil
}

// Update handles root-level messages (WindowSizeMsg, pushScreenMsg,
// popScreenMsg) and delegates all other messages to the top of the stack.
// familyResultMsg, preWriteResultMsg, resolvedResultMsg,
// writeResultMsg, and clipboardResultMsg are delegated to the active screen;
// they are listed here as documentation and to ensure the type-switch is
// exhaustive when sub-screens are added in subsequent plans.
func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case pushScreenMsg:
		// CR-01: invoke init() on the pushed screen when it supports it, so the
		// prove screen's phase 1 (runPreWriteCmd) actually starts. Without this,
		// a pushed prove screen never leaves provePhase1Running in the live
		// program (only stack[0]'s init was ever called).
		next := msg.next
		var cmd tea.Cmd
		if in, ok := next.(initializer); ok {
			next, cmd = in.initScreen()
		}
		m.stack = append(m.stack, next)
		return m, cmd
	case popScreenMsg:
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
		return m, nil
	case familyResultMsg:
		// Handled by the active screen (dashboard). Delegate below.
		_, _, _, _ = msg.runID, msg.family, msg.findings, msg.err
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
