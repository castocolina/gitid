package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// pushScreenMsg signals the root model to push a new screen onto the stack.
// Sub-models return this via a tea.Cmd to keep the root model in control
// of the navigation stack (RESEARCH.md Pattern 4).
type pushScreenMsg struct{ next screenModel }

// popScreenMsg signals the root model to pop the current screen off the stack.
type popScreenMsg struct{}

// pushCmd returns a tea.Cmd that emits a pushScreenMsg for the given screen.
func pushCmd(s screenModel) tea.Cmd {
	return func() tea.Msg { return pushScreenMsg{next: s} }
}

// popCmd returns a tea.Cmd that emits a popScreenMsg.
func popCmd() tea.Cmd {
	return func() tea.Msg { return popScreenMsg{} }
}

// familyResultMsg is the async result from one doctor check family (D-09).
// runID prevents stale results from a previous refresh overwriting fresh ones
// (RESEARCH.md Pitfall 4).
type familyResultMsg struct {
	runID    int
	family   doctor.Family
	findings []doctor.Finding
	err      error
}

// identityListResultMsg carries the reconstructed identity list.
type identityListResultMsg struct {
	accounts []identity.Account
	err      error
}

// preWriteResultMsg carries the SSH pre-write test result (Screen 6 Phase 1).
type preWriteResultMsg struct {
	result tester.Result
	err    error
}

// resolvedResultMsg carries the SSH resolved config test result (Screen 6 Phase 2).
type resolvedResultMsg struct {
	result   tester.Result
	resolved tester.ResolvedConfig
}

// writeResultMsg carries the outcome of an identity write operation.
type writeResultMsg struct {
	backupPath string
	err        error
}

// clipboardResultMsg carries the outcome of a clipboard copy operation.
type clipboardResultMsg struct {
	err error
}
