package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/identity"
)

// confirmKind identifies the action the confirm modal is gating.
// All three kinds are declared here so the type is stable across Plans 03-06
// without import changes. Only fixConfirm is driven in Plan 03; Plan 06 drives
// deleteConfirm and rotateConfirm.
type confirmKind int

const (
	fixConfirm    confirmKind = iota // Plan 03: in-app doctor fix
	deleteConfirm                    // Plan 06: identity delete (reserved)
	rotateConfirm                    // Plan 06: key rotation (reserved)
)

// confirmModel is the generic destructive-action confirm modal sub-model.
// It renders a consequence statement + Fix summary BEFORE the Enter prompt
// (D-09, UI-SPEC § Destructive Action Copy, Accessibility Contract item 5).
//
// Ported from tui/prove.go (Phase 5.5) confirmActive + writeResultMsg pattern
// (PATTERNS § tui/confirm.go, prove.go lines 210–255).
//
// Security invariant (T-05.6-06): the fix/delete/rotate action is dispatched
// ONLY when the user presses Enter and running is false. Esc is the safe default
// and NEVER dispatches any write.
type confirmModel struct {
	kind confirmKind

	// finding holds the fixable finding for fixConfirm; zero for delete/rotate.
	finding doctor.Finding

	// deleteAcct is the account being deleted (deleteConfirm kind only).
	deleteAcct *identity.Account
	// rotateAcct is the account being rotated (rotateConfirm kind only).
	rotateAcct *identity.Account

	// keepKey is the "Delete key files too? [y/N]" toggle for deleteConfirm.
	// Default true = keep key files (safe default, D-07). Set to false only when
	// user explicitly presses 'k' to toggle the delete-key-files option.
	keepKey bool

	// title and body are the consequence statement rendered in the modal box.
	// body must include the Fix.Summary BEFORE the Enter prompt (D-09).
	title string
	body  string

	// running is true while the async op is in flight (prevents double-dispatch).
	running bool
	// result holds the human-readable outcome after the op completes.
	// On success: a brief confirmation string.
	// On failure: "✗ fix failed: <err> [critical]" (shown until Esc).
	result string

	// sp is the spinner shown while running is true.
	sp spinner.Model

	deps tuiDeps
}

// newConfirmModel constructs a confirmModel for the given kind and finding.
// The consequence / Fix summary is pre-rendered into m.body so the view is
// stateless. For deleteConfirm, keepKey defaults to true (safe default per D-07).
func newConfirmModel(kind confirmKind, finding doctor.Finding, deps tuiDeps) confirmModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	title, body := buildConfirmContent(kind, finding)
	return confirmModel{
		kind:    kind,
		finding: finding,
		keepKey: true, // safe default: keep key files (D-07, SAFE-01); toggle with 'k'
		title:   title,
		body:    body,
		sp:      sp,
		deps:    deps,
	}
}

// buildConfirmContent produces the title and body for the confirm modal.
// The body renders the consequence / Fix.Summary statement BEFORE the Enter prompt
// so the user sees exactly what will happen before committing (D-09).
func buildConfirmContent(kind confirmKind, finding doctor.Finding) (title, body string) {
	switch kind {
	case fixConfirm:
		title = "Apply Fix"
		var sb strings.Builder
		sb.WriteString(StyleFaint.Render("Finding: "))
		sb.WriteString(finding.Title)
		sb.WriteString("\n\n")
		if finding.Fix != nil && finding.Fix.Summary != "" {
			sb.WriteString(StyleLabel.Render("Fix: "))
			sb.WriteString(finding.Fix.Summary)
			sb.WriteString("\n\n")
		}
		sb.WriteString(StyleFaint.Render("[Enter to apply / Esc to cancel]"))
		body = sb.String()

	case deleteConfirm:
		// UI-SPEC § Destructive Action Copy: consequence statement shown BEFORE
		// the Enter prompt (Accessibility Contract item 5, D-09).
		title = "Delete Identity"
		var sb strings.Builder
		sb.WriteString(SeverityStyle(doctor.SeverityError).Render("This will remove the following managed artifacts:"))
		sb.WriteString("\n")
		sb.WriteString("  · SSH Host block\n")
		sb.WriteString("  · gitconfig includeIf block\n")
		sb.WriteString("  · fragment file (~/.gitconfig.d/<name>)\n")
		sb.WriteString("  · allowed_signers block\n")
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("Delete key files too? [y/N]  "))
		sb.WriteString(StyleLabel.Render("N (keep)"))
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("Press 'k' to toggle · Enter to confirm · Esc to cancel"))
		body = sb.String()

	case rotateConfirm:
		// UI-SPEC § Destructive Action Copy: consequence statement shown BEFORE
		// the Enter prompt (D-09, KEY-01).
		title = "Rotate Key"
		var sb strings.Builder
		sb.WriteString(StyleLabel.Render("The current key will be replaced:"))
		sb.WriteString("\n")
		sb.WriteString("  · A new ed25519 key will be generated\n")
		sb.WriteString("  · Upload the new public key, then the test loop re-runs\n")
		sb.WriteString("  · Config is written ONLY after the test passes\n")
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("[Enter to start rotation · Esc to cancel]"))
		body = sb.String()
	}
	return title, body
}

// update handles key presses and result messages for the confirm modal.
//
// Security contract (T-05.6-06):
//   - Enter: effective only when !running. Sets running=true, dispatches async cmd.
//   - Esc: safe default; never dispatches any write; emits clearModalCmd.
//   - k: toggles keepKey for deleteConfirm (D-07 — key-deletion requires explicit opt-in).
func (m confirmModel) update(msg tea.Msg) (confirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEnter:
			if m.running {
				// Double-dispatch guard: Enter is a no-op while running.
				return m, nil
			}
			m.running = true
			return m, m.dispatchCmd()

		case tea.KeyEscape:
			m.running = false
			// Esc: safe default — emit clearModalCmd to dismiss without any write.
			return m, clearModalCmd()
		}

		// 'k' toggles the keep-key option for deleteConfirm (D-07).
		if msg.String() == "k" && m.kind == deleteConfirm {
			m.keepKey = !m.keepKey
			return m, nil
		}

	case fixResultMsg:
		m.running = false
		if msg.err != nil {
			m.result = fmt.Sprintf("✗ fix failed: %v [critical]", msg.err)
			// Keep modal open; user must Esc to dismiss after seeing the error.
			return m, nil
		}
		// Success: clear modal and re-run the affected family so findings update live.
		return m, tea.Batch(
			clearModalCmd(),
			makeFamilyCmd(0, msg.family, m.deps.doctor), // runID=0 — stale guard; root refreshes properly
		)

	case deleteResultMsg:
		m.running = false
		if msg.err != nil {
			m.result = fmt.Sprintf("✗ delete failed: %v [critical]", msg.err)
			return m, nil
		}
		// Success: clear modal; root model handles sidebar refresh.
		return m, clearModalCmd()

	case rotateResultMsg:
		m.running = false
		if msg.err != nil {
			m.result = fmt.Sprintf("✗ rotate failed: %v [critical]", msg.err)
			return m, nil
		}
		// Success: clear modal; root model handles sidebar refresh.
		return m, clearModalCmd()
	}

	return m, nil
}

// dispatchCmd returns the async tea.Cmd for the active confirmKind.
// For fixConfirm: calls runFixCmd.
// For deleteConfirm: calls runDeleteCmd (identity.Delete via deps.delete).
// For rotateConfirm: calls runRotateCmd (identity.Rotate via deps.identity).
func (m confirmModel) dispatchCmd() tea.Cmd {
	switch m.kind {
	case fixConfirm:
		return runFixCmd(m.finding, m.deps)
	case deleteConfirm:
		if m.deleteAcct == nil {
			return func() tea.Msg {
				return deleteResultMsg{err: fmt.Errorf("delete: no identity selected")}
			}
		}
		return runDeleteCmd(*m.deleteAcct, m.keepKey, m.deps)
	case rotateConfirm:
		if m.rotateAcct == nil {
			return func() tea.Msg {
				return rotateResultMsg{err: fmt.Errorf("rotate: no identity selected")}
			}
		}
		return runRotateCmd(*m.rotateAcct, m.deps)
	}
	return nil
}

// view renders the confirm modal box. w is the terminal width for sizing.
// The modal follows UI-SPEC § Modal box styling and § Destructive Action Copy:
// consequence statement BEFORE [Enter to apply / Esc to cancel].
func (m confirmModel) view(w int) string {
	mw := modalWidth(w)
	var sb strings.Builder

	sb.WriteString(StyleModalTitle.Render(m.title))
	sb.WriteString("\n\n")

	if m.running {
		switch m.kind {
		case deleteConfirm:
			sb.WriteString(m.sp.View())
			sb.WriteString(" deleting identity...\n")
		case rotateConfirm:
			sb.WriteString(m.sp.View())
			sb.WriteString(" rotating key...\n")
		default:
			sb.WriteString(m.sp.View())
			sb.WriteString(" applying fix...\n")
		}
	} else if m.result != "" {
		// Show success/failure result.
		sb.WriteString(m.result)
		sb.WriteString("\n\n")
		sb.WriteString(StyleFaint.Render("[Esc to close]"))
	} else if m.kind == deleteConfirm {
		// Render consequence statement with live keepKey toggle state.
		sb.WriteString(SeverityStyle(doctor.SeverityError).Render("This will remove the following managed artifacts:"))
		sb.WriteString("\n")
		sb.WriteString("  · SSH Host block\n")
		sb.WriteString("  · gitconfig includeIf block\n")
		sb.WriteString("  · fragment file (~/.gitconfig.d/<name>)\n")
		sb.WriteString("  · allowed_signers block\n")
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("Delete key files too? [y/N]  "))
		if m.keepKey {
			sb.WriteString(StyleLabel.Render("N (keep)"))
		} else {
			sb.WriteString(SeverityStyle(doctor.SeverityError).Render("Y (delete — irreversible)"))
		}
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("Press 'k' to toggle · Enter to confirm · Esc to cancel"))
	} else {
		sb.WriteString(m.body)
	}

	return StyleModal.Width(mw).Render(sb.String())
}

// runDeleteCmd constructs the async tea.Cmd that calls identity.Delete through
// a goroutine with a recover() wrap (T-05.6-19, T-05.6-20).
//
// Security invariant (T-05.6-19, D-09): delete is dispatched ONLY via
// confirmModel.dispatchCmd() which is called only when !running and the user
// pressed Enter. No direct filewriter call from tui/; all removal routes through
// deps.delete (filewriter.BackupAndRemove / gitconfig.RemoveAllowedSignersBlock).
//
// Key-file deletion is gated behind keepKey=false (D-07). The default is
// keepKey=true (safe default) so key files are kept unless the user explicitly
// toggles with 'k'.
func runDeleteCmd(acct identity.Account, keepKey bool, deps tuiDeps) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = deleteResultMsg{
					err: fmt.Errorf("delete for %q panicked: %v", acct.Name, r),
				}
			}
		}()
		result, err := identity.Delete(acct, keepKey, deps.delete)
		return deleteResultMsg{result: result, err: err}
	}
}

// runRotateCmd constructs the async tea.Cmd that calls identity.Rotate through
// a goroutine with a recover() wrap (T-05.6-21, KEY-01).
//
// Security invariant (T-05.6-21): rotate generates a new key via identity.Rotate
// which reuses the shared pipeline (generate → pre-write → write). The write gate
// inside the pipeline fires only after the prove loop passes (KEY-01). The TUI
// confirm gate is an additional layer — the user must press Enter before any
// write is attempted.
func runRotateCmd(acct identity.Account, deps tuiDeps) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = rotateResultMsg{
					err: fmt.Errorf("rotate for %q panicked: %v", acct.Name, r),
				}
			}
		}()
		result, err := identity.Rotate(acct, deps.identity)
		return rotateResultMsg{result: result, err: err}
	}
}

// runFixCmd constructs the async tea.Cmd that calls the finding's Fix.Fn through
// a goroutine with a recover() wrap (T-05.6-07 / T-05.6-08).
//
// Security invariant (T-05.6-07): the fix is applied ONLY via finding.Fix.Fn,
// which in the live wiring calls doctor.Deps.FixPerm / RemoveBlock / AddWiring
// already wired in buildTUIDoctorDeps. No direct os.Chmod or filewriter call.
//
// Panic safety (T-05.6-08): recover() converts any panic inside the fix goroutine
// into a fixResultMsg{err: ...} so the UI shows an error instead of crashing.
func runFixCmd(finding doctor.Finding, _ tuiDeps) tea.Cmd {
	fam := finding.Family
	fix := finding.Fix
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = fixResultMsg{
					family: fam,
					err:    fmt.Errorf("fix for %q panicked: %v", string(fam), r),
				}
			}
		}()
		if fix == nil || fix.Fn == nil {
			return fixResultMsg{
				family: fam,
				err:    fmt.Errorf("finding %q has no Fix.Fn wired", finding.Title),
			}
		}
		err := fix.Fn()
		return fixResultMsg{family: fam, err: err}
	}
}
