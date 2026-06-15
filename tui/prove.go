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
	// provePhaseWritten is the terminal success state after a confirmed write:
	// the backup path is displayed and the screen waits for Esc/Enter to pop
	// (WR-05).
	provePhaseWritten
)

// proveModel is the Prove-Before-Write screen (Screen 6). It runs the two-phase
// SSH test as tea.Cmd goroutines (RESEARCH Pattern 5 / Pitfall 7: never block
// Update()) and gates the write behind explicit confirm only after both phases
// pass (D-04, T-05-15).
//
// Write path: only through identity.Deps (D-04 / T-05-14); no filewriter import.
type proveModel struct {
	phase    provePhase
	action   string               // "create", "update", "add-account"
	input    identity.CreateInput // create/add-account write input
	account  identity.Account     // edited identity for update/add-account (with resolved managed paths)
	original identity.Account     // PRE-EDIT identity for the "update" action only; zero otherwise (FIX-2)
	// signing carries the PRESERVED signing state for the "update" action,
	// computed by reading the existing fragment (FIX-1). It mirrors the CLI's
	// currentSigning so an update neither silently enables signing for a
	// non-signing identity nor removes it when the email is cleared.
	signing        bool
	keyPath        string // staged/existing PRIVATE-KEY path for runPreWriteCmd (CR-04)
	phase1Result   tester.Result
	phase2Result   tester.Result
	phase2Resolved tester.ResolvedConfig
	backupPath     string
	confirmActive  bool // true only after BOTH phases pass (T-05-15)
	deps           tuiDeps
	sp             spinner.Model
	writeErr       error
}

// newProveScreen builds the Prove-Before-Write screen. keyPath is the staged
// (create) or existing (update/add-account) PRIVATE-KEY path — NOT the ssh
// config path (CR-04); phase 1 runs the pre-write authentication gate against
// it. account carries the existing identity (with resolved managed target
// paths) for the update/add-account write modes (CR-03). Call init() to start
// phase 1.
func newProveScreen(action string, input identity.CreateInput, account identity.Account, keyPath string, deps tuiDeps) proveModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return proveModel{
		phase:   provePhase1Running,
		action:  action,
		input:   input,
		account: account,
		keyPath: keyPath,
		deps:    deps,
		sp:      sp,
	}
}

// withUpdateContext records the PRE-EDIT identity (original) and the PRESERVED
// signing state for the "update" action (FIX-1/FIX-2). The update form calls
// this so runWriteCmd can pass a DISTINCT original vs edited account to
// identity.Update (enabling structural re-test detection) and the preserved
// signing flag (avoiding the email-presence heuristic). Create/add-account do
// not call this; their original stays zero and signing is irrelevant to their
// dispatch.
func (m proveModel) withUpdateContext(original identity.Account, signing bool) proveModel {
	m.original = original
	m.signing = signing
	return m
}

// init starts phase 1 by issuing runPreWriteCmd as a tea.Cmd and seeds the
// spinner tick so the "testing..." spinner animates while the gate runs (WR-01).
// The push-screen handler (CR-01) calls this when the screen is pushed.
// RESEARCH Pattern 5: the cmd runs in a goroutine; Update never blocks.
func (m proveModel) init() (proveModel, tea.Cmd) {
	return m, tea.Batch(
		runPreWriteCmd(m.deps.identity.PreWrite, m.keyPath, m.input.Hostname, m.input.Port),
		m.sp.Tick,
	)
}

// initScreen satisfies the initializer interface (CR-01) so the root push
// handler can start phase 1 when the prove screen is pushed onto the stack. It
// returns the screen as a screenModel plus the phase-1 startup cmd.
func (m proveModel) initScreen() (screenModel, tea.Cmd) {
	updated, cmd := m.init()
	return updated, cmd
}

// runPreWriteCmd wraps the injected PreWrite seam in a tea.Cmd goroutine so it
// never blocks Update() (RESEARCH Pattern 5 / Pitfall 7) AND so the phase-1 gate
// runs through the same injected identity.Deps seam the write path uses (CR-04):
// routing it through deps makes the gate observable/testable and keeps the
// "prove before write" invariant honest. Falls back to the package tester when
// the seam is nil (defensive). Result arrives as preWriteResultMsg.
func runPreWriteCmd(preWrite func(keyPath, hostname string, port int) tester.Result, keyPath, hostname string, port int) tea.Cmd {
	return func() tea.Msg {
		fn := preWrite
		if fn == nil {
			fn = tester.PreWrite
		}
		result := fn(keyPath, hostname, port)
		return preWriteResultMsg{result: result}
	}
}

// runResolvedCmd wraps the injected Resolved seam in a tea.Cmd goroutine so it
// never blocks Update() (RESEARCH Pattern 5 / Pitfall 7) and routes phase 2
// through the same injected identity.Deps seam (CR-04). Falls back to the
// package tester when the seam is nil. Result arrives as resolvedResultMsg.
func runResolvedCmd(resolved func(alias string) (tester.Result, tester.ResolvedConfig), alias string) tea.Cmd {
	return func() tea.Msg {
		fn := resolved
		if fn == nil {
			fn = tester.Resolved
		}
		result, cfg := fn(alias)
		return resolvedResultMsg{result: result, resolved: cfg}
	}
}

// runWriteCmd dispatches the confirmed write through the correct identity mode
// based on action (CR-03): "update" → identity.Update (edit in place, no new
// key), "add-account" → identity.AddAccount (second alias sharing the existing
// key, no keygen), default ("create") → identity.Create (full create-new
// pipeline). Routing update/add-account through Create would mint a brand-new
// key and overwrite the existing identity — a data-corruption blocker. The TUI
// adds NO new write path: every mode funnels through the same filewriter-backed
// identity.Deps / identity.UpdateDeps seams the CLI uses (D-04 / T-05-14). The
// backup path of the rewritten ssh config is carried back for the success
// confirmation (WR-05).
func runWriteCmd(action string, input identity.CreateInput, original, edited identity.Account, signing bool, deps tuiDeps) tea.Cmd {
	return func() tea.Msg {
		switch action {
		case "update":
			// FIX-1: PRESERVE the existing signing state (computed by the update
			// form via gitconfig.ReadFragment, mirroring the CLI's currentSigning)
			// instead of inferring it from the presence of an email. FIX-2: pass the
			// DISTINCT pre-edit original vs edited account so identity.Update can
			// detect an alias/hostname/port change and run the D-05 resolved re-test.
			res, err := identity.Update(original, edited, deps.update, signing)
			if err != nil {
				return writeResultMsg{err: err}
			}
			return writeResultMsg{backupPath: res.SSHBackup}
		case "add-account":
			res, err := identity.AddAccount(edited, input.Provider, input.Alias, deps.identity)
			if err != nil {
				return writeResultMsg{err: err}
			}
			return writeResultMsg{backupPath: res.SSHBackup}
		default:
			in := input
			in.Confirmed = true
			res, err := identity.Create(in, deps.identity)
			if err != nil {
				return writeResultMsg{err: err}
			}
			return writeResultMsg{backupPath: res.SSHBackup}
		}
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
		return m, runResolvedCmd(m.deps.identity.Resolved, m.input.Alias)

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
			// Surface the backup path on success and hold the screen so the user
			// sees where the timestamped backup went (WR-05, CLAUDE.md safe-write
			// invariant) before pressing Esc/Enter to pop back.
			m.backupPath = msg.backupPath
			m.phase = provePhaseWritten
			m.confirmActive = false
			return m, nil
		}
		m.writeErr = msg.err
		m.confirmActive = false
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			return m, popCmd()
		case key.Matches(msg, keys.Confirm):
			if m.confirmActive {
				// Route write through the action-appropriate identity mode
				// (CR-03), all via the injected deps (D-04/T-05-14). For "update"
				// the pre-edit original and preserved signing flag drive structural
				// detection and signing parity (FIX-1/FIX-2).
				return m, runWriteCmd(m.action, m.input, m.original, m.account, m.signing, m.deps)
			}
			if m.phase == provePhaseWritten {
				// Enter on the post-write confirmation pops back (WR-05).
				return m, popCmd()
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

	// Phase 1 passed status line. FIX-3: for "create" the private key does NOT
	// exist yet (it is generated by identity.Create on confirm), so the push-time
	// gate runs ssh -i against a not-yet-existent key and is classified
	// ReachableNotUploaded (not Failure). Claiming "✓ authenticated" there would
	// be false. The genuine pre-write gate still runs inside the create pipeline
	// against the staged temp key, so the SAFETY invariant is unchanged — only
	// the on-screen label differs. update/add-account gate a key that exists, so
	// they keep "✓ authenticated".
	if m.action == "create" {
		sb.WriteString(StylePass.Render("Status:  ✓ host reachable — key will be generated on confirm") + "\n\n")
	} else {
		sb.WriteString(StylePass.Render("Status:  ✓ authenticated") + "\n\n")
	}

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

	if m.phase == provePhaseWritten {
		sb.WriteString("\n" + StylePass.Render("✓ changes written") + "\n")
		if strings.TrimSpace(m.backupPath) != "" {
			sb.WriteString(StyleLabel.Render("Backup:") + " " + StyleBody.Render(m.backupPath) + "\n")
		} else {
			sb.WriteString(StyleFaint.Render("(no prior file to back up)") + "\n")
		}
		sb.WriteString(StyleFaint.Render("Press Enter or Esc to return.") + "\n")
	}

	if m.writeErr != nil {
		sb.WriteString("\n" + SeverityStyle(doctor.SeverityCritical).Render(fmt.Sprintf("✗ write failed: %v [critical]", m.writeErr)) + "\n")
		sb.WriteString(StyleBody.Render("No changes were written. Press Esc to go back.") + "\n")
	}

	return sb.String()
}
