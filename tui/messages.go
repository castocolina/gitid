package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// NOTE: pushScreenMsg, popScreenMsg, pushCmd, and popCmd are REMOVED in Phase 5.6.
// The screen-stack architecture (D-15) is replaced by a persistent two-pane
// layout with a single activeView enum and a single activeModal field. Plan 02
// wires the new root model; Plans 02-06 port the proven logic from the old
// screen-stack files. The old files (identitylist.go, identitydetail.go,
// createform.go, updateform.go, addaccountform.go, dashboard.go, prove.go,
// copy.go, model.go) are deleted in this plan (Task 2) since their push/pop
// usage would fail to compile after this removal.

// --- Retained message types (keep verbatim from Phase 5) ---

// familyResultMsg is the async result from one doctor check family (D-09).
// runID prevents stale results from a previous refresh overwriting fresh ones
// (RESEARCH.md Pitfall 4).
type familyResultMsg struct {
	runID    int
	family   doctor.Family
	findings []doctor.Finding
	err      error
}

// preWriteResultMsg carries the SSH pre-write test result (wizard Phase 1).
type preWriteResultMsg struct {
	result tester.Result
	err    error
}

// resolvedResultMsg carries the SSH resolved config test result (wizard Phase 2).
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

// --- New message types for Phase 5.6 two-pane + modal architecture ---

// clearModalMsg signals the root model to dismiss the active modal.
// Emitted by modal sub-models (wizard, confirm, palette, help) on Esc or
// post-completion.
type clearModalMsg struct{}

// refreshSidebarMsg signals the root model to rebuild the sidebar from the
// latest identity.Reconstruct result. Emitted after a successful write
// (create, update, delete, rotate) so the sidebar reflects the new state.
// accounts and unmanaged carry the freshly reconstructed data; the root
// model replaces sidebar.accounts and sidebar.unmanaged from these fields.
type refreshSidebarMsg struct {
	accounts  []identity.Account
	unmanaged []unmanagedEntry
}

// setToastMsg displays a transient toast notification in the header/footer
// for a short duration (e.g., "✓ identity created", "✗ write failed").
type setToastMsg struct {
	text  string
	style lipgloss.Style
}

// clearToastMsg clears the active toast notification.
type clearToastMsg struct{}

// deleteResultMsg carries the outcome of an in-app identity delete operation
// (Plan 06 confirms and dispatches identity.Delete through tuiDeps.delete).
type deleteResultMsg struct {
	result identity.DeleteResult
	err    error
}

// fixResultMsg carries the outcome of an in-app doctor fix operation (D-09).
// The fix was applied to the given family's finding(s); a follow-up refresh
// re-evaluates the family to update the badge and health view.
type fixResultMsg struct {
	family doctor.Family
	err    error
}

// rotateResultMsg carries the outcome of an in-app key rotation (Plan 05/06).
// On success, result holds the new identity.CreateResult (new key paths,
// backup paths); the wizard re-starts from the upload + test loop.
type rotateResultMsg struct {
	result identity.CreateResult
	err    error
}

// baselineLoadedMsg carries the result of reading the global baseline state
// (gitconfig.ReadBaselineState) when the Global Options view is switched to.
// err is non-nil when the read fails; callers render an error notice in that case.
type baselineLoadedMsg struct {
	state gitconfig.BaselineState
	err   error
}

// --- Helper command constructors ---

// clearModalCmd returns a tea.Cmd that emits clearModalMsg{}, dismissing the
// active modal. Sub-models return this from their update() when the user
// presses Esc or when the modal completes its flow.
func clearModalCmd() tea.Cmd {
	return func() tea.Msg { return clearModalMsg{} }
}

// refreshSidebarCmd returns a tea.Cmd that emits refreshSidebarMsg{}, triggering
// a sidebar rebuild from the latest identity.Reconstruct result.
func refreshSidebarCmd() tea.Cmd {
	return func() tea.Msg { return refreshSidebarMsg{} }
}

// setToastCmd returns a tea.Cmd that emits setToastMsg{text, style}, displaying
// a transient toast notification. Pair with clearToastAfter to auto-dismiss.
func setToastCmd(text string, style lipgloss.Style) tea.Cmd {
	return func() tea.Msg { return setToastMsg{text: text, style: style} }
}

// clearToastAfter returns a tea.Cmd that waits d duration and then emits
// clearToastMsg{}, auto-dismissing the active toast. Uses tea.Tick so it runs
// in a goroutine and never blocks Update().
func clearToastAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return clearToastMsg{}
	})
}
