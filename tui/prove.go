package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// provePhase is the state machine for the Prove-Before-Write screen (D-04).
type provePhase int

const (
	provePhase1Running provePhase = iota
	provePhase1Done
	provePhase2Running
	provePhase2Done
	provePhase1Failed
	provePhase2Failed
)

// proveModel is the Prove-Before-Write screen (Screen 6). It runs the two-phase
// SSH test as tea.Cmd goroutines (RESEARCH Pattern 5 / Pitfall 7: never block
// Update()) and gates the write behind explicit confirm only after both phases
// pass (D-04, T-05-15).
//
// Write path: only through identity.Deps (D-04 / T-05-14); no filewriter import.
type proveModel struct {
	phase          provePhase
	action         string // "create", "update", "add-account"
	input          identity.CreateInput
	keyPath        string // staged/existing key path for runPreWriteCmd
	phase1Result   tester.Result
	phase2Result   tester.Result
	phase2Resolved tester.ResolvedConfig
	backupPath     string
	confirmActive  bool // true only after BOTH phases pass (T-05-15)
	deps           tuiDeps
	sp             spinner.Model
	writeErr       error
}

// newProveScreen builds the Prove-Before-Write screen. The prove screen runs
// phase 1 (runPreWriteCmd) using keyPath (the staged or existing key path from
// the form). Call init() to start phase 1.
func newProveScreen(action string, input identity.CreateInput, deps tuiDeps) proveModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return proveModel{
		phase:   provePhase1Running,
		action:  action,
		input:   input,
		keyPath: input.SSHConfigPath, // forms set this; empty in tests (preWriteResultMsg injected directly)
		deps:    deps,
		sp:      sp,
	}
}

// init starts phase 1 by issuing runPreWriteCmd as a tea.Cmd.
// The root model or the push-screen handler calls this after pushing the screen.
// RESEARCH Pattern 5: the cmd runs in a goroutine; Update never blocks.
func (m proveModel) init() (proveModel, tea.Cmd) {
	return m, runPreWriteCmd(m.keyPath, m.input.Hostname, m.input.Port)
}

// runPreWriteCmd wraps tester.PreWrite in a tea.Cmd goroutine so it never
// blocks Update() (RESEARCH Pattern 5 / Pitfall 7). Result arrives as
// preWriteResultMsg.
func runPreWriteCmd(keyPath, hostname string, port int) tea.Cmd {
	return func() tea.Msg {
		result := tester.PreWrite(keyPath, hostname, port)
		return preWriteResultMsg{result: result}
	}
}

// runResolvedCmd wraps tester.Resolved in a tea.Cmd goroutine so it never
// blocks Update() (RESEARCH Pattern 5 / Pitfall 7). Result arrives as
// resolvedResultMsg.
func runResolvedCmd(alias string) tea.Cmd {
	return func() tea.Msg {
		result, resolved := tester.Resolved(alias)
		return resolvedResultMsg{result: result, resolved: resolved}
	}
}

// runWriteCmd dispatches the confirmed write through identity.Deps seams (D-04 /
// T-05-14). The TUI adds NO new write path — all writes go through the same
// filewriter-backed seams the CLI uses. The write cmd calls identity.Create
// with Confirmed=true so the full pipeline (PersistKey + 4 writers) runs.
func runWriteCmd(input identity.CreateInput, deps tuiDeps) tea.Cmd {
	return func() tea.Msg {
		in := input
		in.Confirmed = true
		_, err := identity.Create(in, deps.identity)
		return writeResultMsg{err: err}
	}
}

// update handles messages for the prove screen.
func (m proveModel) update(msg tea.Msg) (screenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case preWriteResultMsg:
		m.phase1Result = msg.result
		if msg.result.Outcome == tester.Failure {
			m.phase = provePhase1Failed
			m.confirmActive = false
			return m, nil
		}
		// Phase 1 passed — start phase 2 immediately (RESEARCH Pattern 5).
		m.phase = provePhase2Running
		return m, runResolvedCmd(m.input.Alias)

	case resolvedResultMsg:
		m.phase2Result = msg.result
		m.phase2Resolved = msg.resolved
		if msg.result.Outcome == tester.Failure {
			m.phase = provePhase2Failed
			m.confirmActive = false
			return m, nil
		}
		// Both phases passed — enable confirm (T-05-15, D-04).
		m.phase = provePhase2Done
		m.confirmActive = true
		return m, nil

	case writeResultMsg:
		if msg.err == nil {
			m.backupPath = msg.backupPath
		} else {
			m.writeErr = msg.err
		}
		// Pop back to list/detail on successful write.
		return m, popCmd()

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			return m, popCmd()
		case key.Matches(msg, keys.Confirm):
			if m.confirmActive {
				// Route write through identity.Deps (D-04/T-05-14).
				return m, runWriteCmd(m.input, m.deps)
			}
			// Confirm is inert until both phases pass (T-05-15).
			return m, nil
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		return m, nil
	}
	return m, nil
}

// view renders the Prove-Before-Write screen (Screen 6) exactly per UI-SPEC.
func (m proveModel) view() string {
	var sb strings.Builder
	title := fmt.Sprintf("gitid — Confirm: %s identity %q", m.action, m.input.Name)
	sb.WriteString(StyleTitle.Render(title) + "\n\n")

	// Phase 1 section.
	sb.WriteString(StyleHeader.Render("Phase 1: Testing key authentication") + "\n")
	if m.phase1Result.Command != "" {
		sb.WriteString(StyleLabel.Render("Command:") + " " + StyleBody.Render(m.phase1Result.Command) + "\n")
		sb.WriteString(StyleLabel.Render("Output:") + "  " + StyleFaint.Render(strings.TrimRight(m.phase1Result.Output, "\n")) + "\n")
	}

	switch m.phase {
	case provePhase1Running:
		sb.WriteString(StyleBody.Render(m.sp.View()+" testing SSH authentication...") + "\n\n")
		return sb.String()

	case provePhase1Failed:
		sb.WriteString(SeverityStyle(doctor.SeverityCritical).Render("Status:  ✗ authentication failed [critical]") + "\n\n")
		sb.WriteString(StyleBody.Render("Cannot proceed — SSH authentication failed.") + "\n")
		sb.WriteString(StyleFaint.Render("Press Esc to go back and review the identity configuration.") + "\n")
		return sb.String()
	}

	// Phase 1 passed status line.
	sb.WriteString(StylePass.Render("Status:  ✓ authenticated") + "\n\n")

	// Phase 2 section (only visible after phase 1 runs).
	if m.phase >= provePhase2Running {
		sb.WriteString(StyleHeader.Render("Phase 2: Testing resolved config") + "\n")
		if m.phase2Result.Command != "" {
			sb.WriteString(StyleLabel.Render("Command:") + " " + StyleBody.Render(m.phase2Result.Command) + "\n")
			sb.WriteString(StyleLabel.Render("Output:") + "  " + StyleFaint.Render(strings.TrimRight(m.phase2Result.Output, "\n")) + "\n")
		}

		switch m.phase {
		case provePhase2Running:
			sb.WriteString(StyleBody.Render(m.sp.View()+" testing resolved config...") + "\n\n")

		case provePhase2Done:
			sb.WriteString(StylePass.Render("Status:  ✓ resolves correctly") + "\n\n")
			action := fmt.Sprintf(
				"Write 4 artifacts (SSH Host block, includeIf, fragment, allowed_signers) for %q",
				m.input.Name,
			)
			sb.WriteString(StyleLabel.Render("Action:") + " " + StyleBody.Render(action) + "\n\n")
			sb.WriteString(StyleBody.Render("Write changes? [Enter to confirm / Esc to cancel]") + "\n")

		case provePhase2Failed:
			sb.WriteString(SeverityStyle(doctor.SeverityError).Render("Status:  ✗ config resolution failed [error]") + "\n\n")
			sb.WriteString(StyleBody.Render("Cannot proceed — config resolution failed.") + "\n")
			sb.WriteString(StyleFaint.Render("Press Esc to go back.") + "\n")
		}
	}

	if m.writeErr != nil {
		sb.WriteString("\n" + SeverityStyle(doctor.SeverityCritical).Render(fmt.Sprintf("✗ write failed: %v [critical]", m.writeErr)) + "\n")
		sb.WriteString(StyleBody.Render("No changes were written. Press Esc to go back.") + "\n")
	}

	return sb.String()
}
