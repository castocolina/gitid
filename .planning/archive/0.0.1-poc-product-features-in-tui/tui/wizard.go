package tui

// wizard.go — Shared prove-before-write modal seam (Plan 04) +
// Create/Add wizard modal (Plan 05).
//
// # Shared seam: wizardProveModel (Plan 04)
//
// wizardProveModel is a reusable sub-model that runs the two-phase SSH
// connectivity test before any structural write:
//
//  1. Phase 1 (pre-write): tester.PreWrite — test SSH reachability on the
//     current config BEFORE writing.
//  2. Phase 2 (resolved): tester.Resolved — test that the WRITTEN alias
//     resolves correctly via `ssh -G`.
//
// The write gate (confirmActive) opens ONLY after both phases PASS. The user
// must then press Enter to dispatch the write. Skip (+explicit confirm) proceeds
// with an "unauthenticated warning" notice. Quit keeps the key without writing.
//
// # Create/Add wizard modal: createWizardModel (Plan 05)
//
// createWizardModel is the full 4-step create/add wizard:
//
//  1. Form: 8-field textinput form (identity name + git details + provider/port + match + signing).
//  2. KeyGen: async ed25519 key generation (spinner).
//  3. Upload: clipboard copy + upload instructions (Enter to test, Esc to quit).
//  4. TestLoop: two-phase SSH prove loop (reuses wizardProveModel state machine).
//
// Persist gate (FIX-CREATE-01, T-05.6-15): PersistAll fires ONLY after both
// phases PASS and the user presses Enter. The skip-and-write path requires an
// explicit second confirm and surfaces an unauthenticated-write warning. 'q'
// dismisses the modal, keeps the key at ~/.ssh/gitid_<name>, and emits a header
// toast.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/upload"
)

// ─── Shared prove seam (Plan 04) ────────────────────────────────────────────

// wizardProvePhase tracks the current state of the two-phase prove-before-write loop.
type wizardProvePhase int

const (
	wizardProvePhase1Running wizardProvePhase = iota
	wizardProvePhase1Done
	wizardProvePhase2Running
	wizardProvePhase2Done
	wizardProvePhase1Failed
	wizardProvePhase2Failed
	wizardProveWritten
)

// wizardProveModel is the shared prove-before-write sub-model. It is embedded
// inside the structural-edit modal overlay (Plan 04) and the create wizard (Plan 05).
//
// Security invariant (D-07, T-05.6-10, T-write-gate):
// identity.Update / write cmd fires ONLY after both phases PASS and Enter confirm.
// The confirmActive field is the write gate — it is false until phase1+phase2 PASS.
type wizardProveModel struct {
	// Input: the existing + edited accounts for the Update call.
	existing identity.Account
	edited   identity.Account
	signing  bool // current signing state

	// Prove-loop state machine.
	phase wizardProvePhase

	// phase1Result holds the outcome of the pre-write SSH test.
	phase1Result tester.Result
	// phase2Result holds the outcome of the resolved config test.
	phase2Result   tester.Result
	phase2Resolved tester.ResolvedConfig

	// confirmActive is the write gate: true only after phase1+phase2 both PASS.
	// The write is dispatched only when the user presses Enter while confirmActive.
	confirmActive bool

	// skipConfirmPending is true after the user presses 's' on a failed phase.
	// A second Enter is required to proceed (explicit double-confirm, T-05.6-12).
	skipConfirmPending bool
	skipWarning        string // shown after skip is confirmed

	// backupPath is the timestamped backup path returned by the write.
	backupPath string

	// runID tracks the current test run to prevent stale results from a previous
	// attempt from overwriting fresh ones (Pitfall 4, Pattern B).
	runID int

	// sp is the spinner shown during phase1 and phase2 (Pattern C — seed Tick on init).
	sp spinner.Model

	// result holds the human-readable outcome after write or error.
	result string

	deps tuiDeps
}

// newWizardProveModel constructs a wizardProveModel for a structural identity edit.
func newWizardProveModel(existing, edited identity.Account, signing bool, deps tuiDeps) wizardProveModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return wizardProveModel{
		existing: existing,
		edited:   edited,
		signing:  signing,
		phase:    wizardProvePhase1Running,
		sp:       sp,
		deps:     deps,
	}
}

// init starts the prove loop: dispatch phase-1 cmd + seed the spinner Tick.
// MANDATORY: the spinner Tick must be seeded in init() (Pattern C) or the spinner
// animation never renders after the first Update.
func (m wizardProveModel) init() (wizardProveModel, tea.Cmd) {
	m.runID++
	return m, tea.Batch(
		runPreWriteCmd(m.deps.identity.PreWrite, m.existing.KeyPath, m.existing.Hostname, m.existing.Port),
		m.sp.Tick, // REQUIRED: seed initial spinner tick
	)
}

// update handles prove-loop messages and key presses.
func (m wizardProveModel) update(msg tea.Msg) (wizardProveModel, tea.Cmd) {
	switch msg := msg.(type) {

	case preWriteResultMsg:
		m.phase1Result = msg.result
		switch msg.result.Outcome {
		case tester.PASS:
			m.phase = wizardProvePhase1Done
			// Advance to phase 2.
			return m, runResolvedCmd(m.deps.identity.Resolved, m.edited.Alias)
		default:
			m.phase = wizardProvePhase1Failed
			m.confirmActive = false
		}
		return m, nil

	case resolvedResultMsg:
		m.phase2Result = msg.result
		m.phase2Resolved = msg.resolved
		switch msg.result.Outcome {
		case tester.PASS:
			m.phase = wizardProvePhase2Done
			// Both phases passed: open the write gate.
			m.confirmActive = true
		default:
			m.phase = wizardProvePhase2Failed
			m.confirmActive = false
		}
		return m, nil

	case writeResultMsg:
		if msg.err != nil {
			m.result = fmt.Sprintf("write failed: %v", msg.err)
		} else {
			m.phase = wizardProveWritten
			m.backupPath = msg.backupPath
			m.result = "written"
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		key := msg.String()
		switch key {
		case "r":
			// Retry: re-run phase 1.
			if m.phase == wizardProvePhase1Failed || m.phase == wizardProvePhase2Failed {
				m.phase = wizardProvePhase1Running
				m.confirmActive = false
				m.skipConfirmPending = false
				m.runID++
				return m, tea.Batch(
					runPreWriteCmd(m.deps.identity.PreWrite, m.existing.KeyPath, m.existing.Hostname, m.existing.Port),
					m.sp.Tick,
				)
			}

		case "s":
			// Skip test and proceed to write — requires an explicit second confirm.
			if !m.confirmActive && (m.phase == wizardProvePhase1Failed || m.phase == wizardProvePhase2Failed) {
				m.skipConfirmPending = true
				m.skipWarning = "! written without authentication verification [warning]"
			}

		case "q":
			// Quit: dismiss the modal, keep key without writing.
			return m, clearModalCmd()

		case "enter":
			if m.skipConfirmPending {
				// Second Enter after 's': proceed with write despite failed tests.
				m.skipConfirmPending = false
				return m, runProveWriteCmd(m.existing, m.edited, m.signing, m.deps)
			}
			if m.confirmActive {
				// Write gate open: dispatch the write.
				m.confirmActive = false
				return m, runProveWriteCmd(m.existing, m.edited, m.signing, m.deps)
			}

		case "esc":
			// Dismiss the modal; do NOT write.
			return m, clearModalCmd()
		}
	}

	return m, nil
}

// view renders the prove-modal box. w is the terminal width for sizing.
func (m wizardProveModel) view(w int) string {
	mw := modalWidth(w)
	var sb strings.Builder

	sb.WriteString(StyleModalTitle.Render("Verify Before Writing"))
	sb.WriteString("\n\n")

	// Phase 1.
	switch m.phase {
	case wizardProvePhase1Running:
		sb.WriteString(m.sp.View() + " Testing SSH reachability...\n")
	case wizardProvePhase1Done, wizardProvePhase2Running, wizardProvePhase2Done, wizardProveWritten:
		sb.WriteString(StylePass.Render("✓") + " SSH reachable\n")
	case wizardProvePhase1Failed:
		sb.WriteString(SeverityStyle(doctor.SeverityError).Render("✗") + " SSH test failed\n")
		sb.WriteString(renderProveActions(m.skipConfirmPending))
	}

	// Phase 2.
	if m.phase >= wizardProvePhase2Running && m.phase != wizardProvePhase1Failed {
		switch m.phase {
		case wizardProvePhase2Running:
			sb.WriteString(m.sp.View() + " Checking resolved config...\n")
		case wizardProvePhase2Done, wizardProveWritten:
			sb.WriteString(StylePass.Render("✓") + " Config resolves correctly\n")
		case wizardProvePhase2Failed:
			sb.WriteString(SeverityStyle(doctor.SeverityError).Render("✗") + " Config resolution failed\n")
			sb.WriteString(renderProveActions(m.skipConfirmPending))
		}
	}

	// Confirm gate.
	if m.confirmActive {
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("[Enter to write · Esc to cancel]"))
	}

	// Skip warning.
	if m.skipWarning != "" {
		sb.WriteString("\n")
		sb.WriteString(SeverityStyle(doctor.SeverityWarning).Render(m.skipWarning) + "\n")
		sb.WriteString(StyleFaint.Render("[Enter to confirm skip · Esc to cancel]"))
	}

	// Result.
	if m.result != "" {
		sb.WriteString("\n" + m.result + "\n")
		sb.WriteString(StyleFaint.Render("[Esc to close]"))
	}

	return StyleModal.Width(mw).Render(sb.String())
}

// renderProveActions renders the retry/skip/quit options shown on test failure.
func renderProveActions(skipPending bool) string {
	if skipPending {
		return StyleFaint.Render("  [Enter confirm skip · Esc cancel]") + "\n"
	}
	return StyleFaint.Render("  [r] retry · [c] copy key · [s] skip (not recommended) · [q] quit") + "\n"
}

// runPreWriteCmd constructs the async tea.Cmd for the phase-1 SSH pre-write test.
// It is the verbatim port from Phase 5.5 tui/prove.go (PATTERNS § Pattern H).
// When the PreWrite seam is nil, a defensive fallback produces a FAIL result.
func runPreWriteCmd(preWrite func(keyPath, hostname string, port int) tester.Result, keyPath, hostname string, port int) tea.Cmd {
	return func() tea.Msg {
		fn := preWrite
		if fn == nil {
			return preWriteResultMsg{result: tester.Result{Outcome: tester.Failure}, err: fmt.Errorf("PreWrite seam is nil")}
		}
		result := fn(keyPath, hostname, port)
		return preWriteResultMsg{result: result}
	}
}

// runResolvedCmd constructs the async tea.Cmd for the phase-2 resolved config test.
func runResolvedCmd(resolved func(alias string) (tester.Result, tester.ResolvedConfig), alias string) tea.Cmd {
	return func() tea.Msg {
		fn := resolved
		if fn == nil {
			return resolvedResultMsg{result: tester.Result{Outcome: tester.Failure}}
		}
		result, rc := fn(alias)
		return resolvedResultMsg{result: result, resolved: rc}
	}
}

// runResolvedViaCmd dispatches the phase-2 alias test against a STAGED temp config.
// It (1) stages the identity's Host block (with the staged temp key as
// IdentityFile) into a throwaway config file via StageTestConfig, then (2) runs
// `ssh -F <tempconfig> -i <key> -T git@<alias>` via ResolvedVia. Neither step
// touches ~/.ssh/config — the live file is written only at the final confirmed
// write. This is the safe alternative to writing a provisional block into the
// live file (UAT G-5: a typed alias is unresolvable until a Host stanza exists).
func runResolvedViaCmd(m createWizardModel) tea.Cmd {
	in := m.buildCreateInput(m.inputs[3].Value())
	staged := m.staged
	deps := m.deps.identity
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = resolvedResultMsg{result: tester.Result{Outcome: tester.Failure}, err: fmt.Errorf("alias test panicked: %v", r)}
			}
		}()
		if deps.StageTestConfig == nil || deps.ResolvedVia == nil {
			return resolvedResultMsg{result: tester.Result{Outcome: tester.Failure}, err: fmt.Errorf("alias-test seams are nil")}
		}
		configPath, err := deps.StageTestConfig(in, staged)
		if err != nil {
			return resolvedResultMsg{result: tester.Result{Outcome: tester.Failure}, err: fmt.Errorf("staging test config: %w", err)}
		}
		result, rc := deps.ResolvedVia(configPath, staged.TempPrivatePath, in.Alias)
		return resolvedResultMsg{result: result, resolved: rc}
	}
}

// runProveWriteCmd dispatches the identity.Update call after the prove loop passes.
// The write result is returned as writeResultMsg.
func runProveWriteCmd(existing, edited identity.Account, signing bool, deps tuiDeps) tea.Cmd {
	return func() tea.Msg {
		res, err := identity.Update(existing, edited, deps.update, signing)
		if err != nil {
			return writeResultMsg{err: err}
		}
		return writeResultMsg{backupPath: res.SSHBackup}
	}
}

// ─── Match-strategy selector types (Phase 5.7, Plan 05) ────────────────────

// matchStrategy enumerates the three includeIf selection strategies the wizard
// and create flow support. gitdir is the default per D-02.
type matchStrategy int

const (
	strategyGitdir    matchStrategy = iota // strategy 1: gitdir (default, D-02)
	strategyHasconfig                      // strategy 2: hasconfig repo URL
	strategyBoth                           // strategy 3: both (OR-applied by git)
)

// defaultHasconfigPattern derives the suggested hasconfig URL pattern from an
// SSH alias: git@<alias>:*/** (recipe canonical form, D-03).
// Returns "" when alias is empty (caller must validate before write).
func defaultHasconfigPattern(alias string) string {
	if alias == "" {
		return ""
	}
	return "git@" + alias + ":*/**"
}

// liveIncludeIfPreview renders the live includeIf preview text for the given
// match strategy. It calls gitconfig.RenderIncludeIf for ALL conditions so no
// [includeIf "..."] string is ever hand-built in this package (T-05.7-05-01).
//
// name is the identity name; the second parameter (alias) is reserved for
// future callers that pass the raw alias for derivation — current implementation
// uses gitdirVal and hasconfigVal directly. hasconfigVal is the bare URL pattern
// (the "remote.*.url:" prefix is prepended by gitconfig.Match via RenderIncludeIf).
func liveIncludeIfPreview(strategy matchStrategy, name, _ string, gitdirVal, hasconfigVal string) string {
	fragPath := "~/.gitconfig.d/" + name
	switch strategy {
	case strategyHasconfig:
		m := gitconfig.Match{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:" + hasconfigVal}
		return gitconfig.RenderIncludeIf(name, fragPath, []gitconfig.Match{m})
	case strategyBoth:
		matches := []gitconfig.Match{
			{Kind: gitconfig.MatchGitdir, Value: gitdirVal},
			{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:" + hasconfigVal},
		}
		return gitconfig.RenderIncludeIf(name, fragPath, matches)
	default: // strategyGitdir
		m := gitconfig.Match{Kind: gitconfig.MatchGitdir, Value: gitdirVal}
		return gitconfig.RenderIncludeIf(name, fragPath, []gitconfig.Match{m})
	}
}

// ─── Create/Add wizard (Plan 05, extended in Plan 10) ───────────────────────

// wizardScreen identifies the multi-screen staged flow introduced in Plan 10.
// Each screen corresponds to a distinct user journey phase:
//
//   - screenSSHIdentity (Screen 1): LEG-1 SSH identity fields (this plan).
//   - screenConnectivity (Screen 2): upload + SSH prove loop (wired to existing machinery).
//   - screenGitConfig (Screen 3): Git Name / Email / Match / Signing (Plan 13 stub).
//   - screenReview (Screen 4): review before final write (Plan 13 stub).
type wizardScreen int

const (
	screenSSHIdentity  wizardScreen = iota // Screen 1: LEG-1 SSH identity (Plan 10)
	screenConnectivity                     // Screen 2: upload + prove loop
	screenGitConfig                        // Screen 3: Git config (Plan 13)
	screenReview                           // Screen 4: review (Plan 13)
)

// wizardStep tracks the current step of the create/add wizard.
type wizardStep int

const (
	wizardStepForm          wizardStep = iota // Step 1: form fields
	wizardStepKeyGen                          // Step 2: keygen spinner
	wizardStepUpload                          // Step 3: upload instructions
	wizardStepProve1Running                   // Step 4: phase 1 test
	wizardStepProve1Done
	wizardStepProve2Running
	wizardStepProve2Done
	wizardStepProve1Failed
	wizardStepProve2Failed
	wizardStepWritten
)

// keygenResultMsg carries the result of an async key generation.
type keygenResultMsg struct {
	staged identity.StagedKey
	err    error
}

// wizardCreateResultMsg carries the result of an async PersistAll call.
type wizardCreateResultMsg struct {
	result identity.CreateResult
	err    error
}

// createWizardModel is the staged create/add wizard modal (Plan 05, extended Plan 10):
//
//	Screen 1 (screenSSHIdentity): LEG-1 SSH-identity form (Plan 10).
//	Screen 2 (screenConnectivity): keygen + upload + prove (existing machinery).
//	Screen 3 (screenGitConfig): Git config — stub for Plan 13.
//	Screen 4 (screenReview): review — stub for Plan 13.
//
// Within each screen the wizardStep sub-state tracks the async steps (keygen,
// prove phases). Screen 2 reuses wizardStepUpload / wizardStepProve* directly.
//
// Security invariant (FIX-CREATE-01, T-05.6-15):
// PersistAll fires ONLY after both prove phases PASS + Enter confirm.
// The skip-and-write path requires an explicit second confirm and surfaces a
// warning. 'q' dismisses the modal, keeps the key, shows a header toast.
type createWizardModel struct {
	// Staged-flow screen (Plan 10): which screen is currently displayed.
	// Defaults to screenSSHIdentity (Screen 1).
	screen wizardScreen

	// Form step (Step 1 / Screen 1).
	inputs     []textinput.Model // 8 fields: Name, GitName, GitEmail, Provider, Port, Alias, Match, Signing
	focusIdx   int
	err        string
	nameLocked bool // true for Add Account mode (name pre-filled and read-only)

	// Screen 1 extended field (Plan 10): Hostname is a first-class top-level
	// editable row on Screen 1. (Folder was also here originally but moved to
	// Screen 3 — it drives the ~/.gitconfig includeIf match, not the SSH config.)
	//
	// hostnameVal: the Hostname field pre-filled from identity.DefaultHostname(provider).
	// hostnameEdited: true once the user has typed into the Hostname field; when false,
	// the hostname auto-tracks Provider changes via refreshHostnameIfUnedited.
	hostnameVal    textinput.Model
	hostnameEdited bool

	// Screen 1 focus mapping (Plan 10):
	// focusIdx encodes both inputs[] indices and the virtual slots for the new
	// standalone rows (hostname, folder). See screen1FocusCount and
	// screen1FocusToField for the mapping.

	// Match-strategy selector state (Phase 5.7, Plan 05). These fields replace
	// the cryptic "1"/"2"/"3" value in inputs[fieldMatch] for interactive use;
	// inputs[fieldMatch] still holds the value for backward compatibility with
	// the build path (buildCreateInput reads inputs[6]).
	matchSel          matchStrategy   // currently selected strategy (default strategyGitdir)
	matchGitdirVal    textinput.Model // gitdir path sub-field (also Screen-1 Folder row)
	matchHasconfigVal textinput.Model // hasconfig URL pattern sub-field

	// KeyGen step (Step 2).
	sp     spinner.Model
	staged identity.StagedKey // result of Generate

	// Upload step (Step 3).
	copyErr error // nil = copied; non-nil = clipboard failure

	// Prove steps (Step 4 — mirrored state from wizardProvePhase).
	step           wizardStep
	phase1Result   tester.Result
	phase2Result   tester.Result
	phase2Resolved tester.ResolvedConfig
	confirmActive  bool

	// skipConfirmPending: user pressed 's' on failed phase; second Enter required.
	skipConfirmPending bool
	skipWarning        string

	// runID guards against stale results from a previous run (Pattern B).
	runID int

	// backupPath from PersistAll on success.
	backupPath string

	// result holds the human-readable outcome after write or error.
	result string

	// LEG artifacts for Screen 4 (review). leg1SSHPreview is the Host block written
	// by Screen 2 (LEG 1); leg2Result carries the gitconfig includeIf / fragment /
	// allowed_signers previews + backups written by Screen 3 (LEG 2).
	leg1SSHPreview string
	leg2Result     identity.CreateResult

	deps tuiDeps
}

// newCreateWizardModel constructs a wizard for create-new (name="") or add-account
// (name pre-filled and locked). Implements the 8-field form from UI-SPEC § Step 1.
func newCreateWizardModel(name string, deps tuiDeps) createWizardModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	mkInput := func(placeholder, value string, charLimit int) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.SetValue(value)
		ti.SetWidth(formFieldWidth) // fixed width → single, aligned border (P0-1)
		if charLimit > 0 {
			ti.CharLimit = charLimit
		}
		return ti
	}

	// 8-field layout per UI-SPEC § Wizard: Create / Add Account Step 1.
	// Port default changed from 22 to identity.DefaultPort() (443) per recipe alt-SSH
	// (Plan 10 Task 2: Hostname/Port pre-filled from alt-SSH helper, never hard-coded).
	portDefault := fmt.Sprintf("%d", identity.DefaultPort())
	inputs := []textinput.Model{
		mkInput("e.g. personal", name, 64),               // 0: Identity Name
		mkInput("e.g. Pedro Perez", "", 128),             // 1: Git Name (Screen 3, Plan 13)
		mkInput("e.g. pedro.perez@example.com", "", 200), // 2: Git Email (Screen 3, Plan 13)
		mkInput("github.com", "github.com", 128),         // 3: Provider (default github.com)
		mkInput("443", portDefault, 10),                  // 4: Port (default 443, alt-SSH)
		mkInput("leave blank to use default", "", 200),   // 5: SSH Alias
		mkInput("1", "1", 10),                            // 6: Match Strategy (Screen 3, Plan 13)
		mkInput("y / n", "y", 4),                         // 7: Signing (Screen 3, Plan 13)
	}
	inputs[0].Focus()

	// Screen 1 Hostname field (Plan 10): pre-filled from identity.DefaultHostname.
	// The default provider is "github.com"; hostnameVal auto-tracks provider changes
	// unless the user edits it (hostnameEdited = true).
	hostnameTI := mkInput("e.g. ssh.github.com", identity.DefaultHostname("github.com"), 256)

	// Match strategy selector sub-fields (Phase 5.7, Plan 05).
	// matchGitdirVal also serves as the Screen-1 Folder (gitdir) row (Plan 10).
	// Default gitdir path: ~/git/<name>/ when name is set at construction time.
	gitdirDefault := ""
	if name != "" {
		gitdirDefault = "~/git/" + name + "/"
	}
	gitdirTI := textinput.New()
	gitdirTI.SetWidth(formFieldWidth)
	gitdirTI.Placeholder = "e.g. ~/git/personal/"
	if gitdirDefault != "" {
		gitdirTI.SetValue(gitdirDefault)
	}

	hasconfigTI := textinput.New()
	hasconfigTI.SetWidth(formFieldWidth)
	hasconfigTI.Placeholder = "e.g. git@personal.github.com:*/**"

	return createWizardModel{
		screen:            screenSSHIdentity,
		inputs:            inputs,
		focusIdx:          0,
		nameLocked:        name != "",
		step:              wizardStepForm,
		sp:                sp,
		deps:              deps,
		matchSel:          strategyGitdir,
		hostnameVal:       hostnameTI,
		hostnameEdited:    false,
		matchGitdirVal:    gitdirTI,
		matchHasconfigVal: hasconfigTI,
	}
}

// ─── Screen 1 focus mapping (Plan 10) ───────────────────────────────────────
//
// Screen 1 (screenSSHIdentity) has its own Tab focus cycle that uses focusIdx
// in the range [0, screen1FocusCount). The mapping is:
//
//	0 → inputs[0]   Identity Name
//	1 → inputs[3]   Provider
//	2 → inputs[5]   SSH Alias
//	3 → hostnameVal Hostname
//	4 → inputs[4]   Port
//
// Key Algorithm (ed25519) is read-only and not Tab-reachable on Screen 1.
// Folder (gitdir) belongs to ~/.gitconfig's includeIf match, NOT the SSH config,
// so it lives on Screen 3 (Git config), not here — matchGitdirVal remains in the
// model for Screen 3 to bind. Git Name (1) / Git Email (2) / Match (6) /
// Signing (7) are likewise Screen 3 fields, kept in inputs[] for Plan 13 to reuse.

const screen1FocusCount = 5 // number of Tab stops on Screen 1 (SSH-only; no Folder)

// screen1InputIdx maps a Screen-1 focus position to the backing inputs[] index.
// Returns -1 for the virtual hostname slot (hostnameVal, not in inputs[]).
func screen1InputIdx(pos int) int {
	switch pos {
	case 0:
		return 0 // Identity Name
	case 1:
		return 3 // Provider
	case 2:
		return 5 // SSH Alias
	case 3:
		return -1 // Hostname (hostnameVal — virtual)
	case 4:
		return 4 // Port
	}
	return -1
}

// hostnameFocusIdx returns the Screen-1 focus position for the Hostname field.
func hostnameFocusIdx() int { return 3 }

// handleKey processes key presses in the wizard form step.
// Returns the updated model and an optional command.
// Form field indices that are NOT free-text inputs:
//   - fieldMatch is the match-strategy selector; ↑/↓ move options, space/enter select.
//   - fieldSigning is a boolean toggled with Space, not typed.
const (
	fieldMatch   = 6
	fieldSigning = 7
)

// refreshHostnameIfUnedited auto-updates hostnameVal to the alt-SSH default for
// the current provider when the user has not yet manually edited the field
// (hostnameEdited == false). Called when the Provider field changes.
func (m createWizardModel) refreshHostnameIfUnedited() createWizardModel {
	if m.hostnameEdited {
		return m
	}
	provider := m.inputs[3].Value()
	if provider == "" {
		provider = "github.com"
	}
	m.hostnameVal.SetValue(identity.DefaultHostname(provider))
	return m
}

// screen1BlurAll unfocuses every Screen-1 interactive element.
func (m *createWizardModel) screen1BlurAll() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	m.hostnameVal.Blur()
	m.matchGitdirVal.Blur()
}

// screen1Focus focuses the Screen-1 element at focus position pos.
func (m *createWizardModel) screen1Focus(pos int) {
	m.screen1BlurAll()
	idx := screen1InputIdx(pos)
	if idx >= 0 {
		m.inputs[idx].Focus()
	} else {
		if pos == hostnameFocusIdx() {
			m.hostnameVal.Focus()
		}
	}
}

// screen1Next advances the Screen-1 focus to the next Tab stop.
func (m *createWizardModel) screen1Next() {
	m.screen1BlurAll()
	m.focusIdx = (m.focusIdx + 1) % screen1FocusCount
	// Skip locked name field in add-account mode.
	if m.nameLocked && m.focusIdx == 0 {
		m.focusIdx = 1
	}
	m.screen1Focus(m.focusIdx)
}

// screen1Prev moves the Screen-1 focus to the previous Tab stop.
func (m *createWizardModel) screen1Prev() {
	m.screen1BlurAll()
	m.focusIdx = (m.focusIdx - 1 + screen1FocusCount) % screen1FocusCount
	// Skip locked name field in add-account mode.
	if m.nameLocked && m.focusIdx == 0 {
		m.focusIdx = screen1FocusCount - 1
	}
	m.screen1Focus(m.focusIdx)
}

func (m createWizardModel) handleKey(msg tea.KeyMsg) (createWizardModel, tea.Cmd) {
	if m.step != wizardStepForm {
		// In later steps, use update() for all messages.
		return m, nil
	}

	key := msg.String()

	// Each staged screen owns its focus cycle:
	//   Screen 1 (SSH identity) → handleKeyScreen1
	//   Screen 3 (Git config / LEG 2) → handleKeyScreen3
	//   Screen 4 (review, read-only) → handleKeyScreen4
	switch m.screen {
	case screenSSHIdentity:
		return m.handleKeyScreen1(msg, key)
	case screenGitConfig:
		return m.handleKeyScreen3(msg, key)
	case screenReview:
		return m.handleKeyScreen4(msg, key)
	}

	// Fallback: the pre-Plan-10 full form (kept for backward-compatible tests).
	return m.handleKeyLegacy(msg, key)
}

// handleKeyScreen4 handles key events for Screen 4 (read-only review). Nothing is
// editable; Enter or Esc closes the wizard — the identity is already on disk.
func (m createWizardModel) handleKeyScreen4(_ tea.KeyMsg, key string) (createWizardModel, tea.Cmd) {
	if key == "enter" || key == "esc" {
		return m, clearModalCmd()
	}
	return m, nil
}

// handleKeyScreen1 handles key events for Screen 1 (SSH-identity form).
func (m createWizardModel) handleKeyScreen1(msg tea.KeyMsg, key string) (createWizardModel, tea.Cmd) {
	switch key {
	case "esc":
		return m, clearModalCmd()

	case "tab":
		m.screen1Next()
		// Refresh hostname default on every Tab (no-op when hostnameEdited or unchanged).
		m = m.refreshHostnameIfUnedited()
		return m, nil

	case "shift+tab":
		m.screen1Prev()
		m = m.refreshHostnameIfUnedited()
		return m, nil

	case "enter":
		// On the last Screen-1 field (Port, pos 4), advance to Screen 2 / keygen.
		if m.focusIdx == screen1FocusCount-1 {
			return m.advanceFromForm()
		}
		// Validate name on the name field.
		if m.focusIdx == 0 && !m.nameLocked {
			if err := identity.ValidateName(m.inputs[0].Value()); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.err = ""
		}
		// Advance to the next field.
		m.screen1Next()
		m = m.refreshHostnameIfUnedited()
		return m, nil
	}

	// Forward printable keys to the focused Screen-1 element.
	// Detect user-typed text via KeyPressMsg.Text (the interface does not expose Text).
	var keyText string
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		keyText = kp.Text
	}
	switch m.focusIdx {
	case hostnameFocusIdx():
		// Mark user has edited hostname so provider changes stop auto-tracking.
		if keyText != "" {
			m.hostnameEdited = true
		}
		// Ensure hostnameVal is focused before forwarding the key event (the
		// textinput Update early-returns when !m.focus, so focus must be set
		// before the first keypress, not only during Tab transitions).
		if !m.hostnameVal.Focused() {
			m.hostnameVal.Focus()
		}
		var cmd tea.Cmd
		m.hostnameVal, cmd = m.hostnameVal.Update(msg)
		return m, cmd

	default:
		idx := screen1InputIdx(m.focusIdx)
		if idx >= 0 {
			// Ensure the target input is focused.
			if !m.inputs[idx].Focused() {
				m.inputs[idx].Focus()
			}
			var cmd tea.Cmd
			m.inputs[idx], cmd = m.inputs[idx].Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// ─── Screen 3 focus mapping (Plan 13 — Git Configuration / LEG 2) ────────────
//
// Screen 3 (screenGitConfig) has its own Tab cycle over the git-side fields:
//
//	0 → inputs[1]      Git Name
//	1 → inputs[2]      Git Email
//	2 → fieldMatch     Match Strategy selector (gitdir / hasconfig / both) with
//	                   editable Folder (gitdir) + hasconfig URL sub-fields
//	3 → fieldSigning   Signing toggle
//
// The SSH-side fields (Provider / Alias / Hostname / Port) are NOT shown here —
// they were collected and written on Screens 1–2. Folder lives here, not on
// Screen 1, because it drives the ~/.gitconfig includeIf match.

const screen3FocusCount = 4 // Tab stops on Screen 3

// screen3InputIdx maps a Screen-3 focus position to its inputs[] index.
func screen3InputIdx(pos int) int {
	switch pos {
	case 0:
		return 1 // Git Name
	case 1:
		return 2 // Git Email
	case 2:
		return fieldMatch // Match selector
	case 3:
		return fieldSigning // Signing toggle
	}
	return -1
}

// screen3BlurAll unfocuses every Screen-3 interactive element.
func (m *createWizardModel) screen3BlurAll() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	m.matchGitdirVal.Blur()
	m.matchHasconfigVal.Blur()
}

// screen3Focus focuses the Screen-3 element at focus position pos. Only Git Name
// and Git Email are free-text inputs that need Focus; the Match selector and the
// Signing toggle are driven inline (no textinput focus).
func (m *createWizardModel) screen3Focus(pos int) {
	m.screen3BlurAll()
	if idx := screen3InputIdx(pos); idx == 1 || idx == 2 {
		m.inputs[idx].Focus()
	}
}

// screen3Next / screen3Prev advance the Screen-3 focus cycle.
func (m *createWizardModel) screen3Next() {
	m.focusIdx = (m.focusIdx + 1) % screen3FocusCount
	if m.nameLocked && m.focusIdx == 0 {
		m.focusIdx = 1 // Git Name remains editable; this guard is defensive parity
	}
	m.screen3Focus(m.focusIdx)
}

func (m *createWizardModel) screen3Prev() {
	m.focusIdx = (m.focusIdx - 1 + screen3FocusCount) % screen3FocusCount
	m.screen3Focus(m.focusIdx)
}

// handleKeyScreen3 handles key events for Screen 3 (Git Configuration, LEG 2).
func (m createWizardModel) handleKeyScreen3(msg tea.KeyMsg, key string) (createWizardModel, tea.Cmd) {
	switch key {
	case "esc":
		return m, clearModalCmd()
	case "tab":
		m.screen3Next()
		return m, nil
	case "shift+tab":
		m.screen3Prev()
		return m, nil
	case "enter":
		// On the last field (Signing, pos 3), validate and write LEG 2.
		if m.focusIdx == screen3FocusCount-1 {
			return m.confirmGitConfig()
		}
		m.screen3Next()
		return m, nil
	}

	// Field-specific input handling.
	switch m.focusIdx {
	case 2: // Match Strategy selector
		switch key {
		case "up", "k":
			if m.matchSel > strategyGitdir {
				m.matchSel--
				m.syncMatchInput()
			}
		case "down", "j":
			if m.matchSel < strategyBoth {
				m.matchSel++
				m.syncMatchInput()
			}
		case "space", " ", "enter":
			// Selection is tracked live by matchSel; ↑/↓ navigate, Tab advances.
		default:
			// Forward typing to the active sub-field. Folder (gitdir) is editable
			// for gitdir/both; the hasconfig URL is editable for hasconfig.
			if m.matchSel == strategyHasconfig {
				var cmd tea.Cmd
				m.matchHasconfigVal.Focus()
				m.matchHasconfigVal, cmd = m.matchHasconfigVal.Update(msg)
				return m, cmd
			}
			var cmd tea.Cmd
			m.matchGitdirVal.Focus()
			m.matchGitdirVal, cmd = m.matchGitdirVal.Update(msg)
			return m, cmd
		}
		return m, nil

	case 3: // Signing toggle (Space)
		if key == "space" || key == " " {
			if m.inputs[fieldSigning].Value() == "n" {
				m.inputs[fieldSigning].SetValue("y")
			} else {
				m.inputs[fieldSigning].SetValue("n")
			}
		}
		return m, nil

	default: // Git Name (inputs[1]) / Git Email (inputs[2])
		idx := screen3InputIdx(m.focusIdx)
		if idx == 1 || idx == 2 {
			if !m.inputs[idx].Focused() {
				m.inputs[idx].Focus()
			}
			var cmd tea.Cmd
			m.inputs[idx], cmd = m.inputs[idx].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// confirmGitConfig validates the git-side inputs and dispatches the LEG-2 write
// (identity.PersistGitconfig). Git Email is required and validated here so a
// malformed address is caught on Screen 3, not deep in the fragment write. On a
// validation error, focus moves to the Git Email field (Screen-3 position 1) so
// the user lands on the field to fix.
func (m createWizardModel) confirmGitConfig() (createWizardModel, tea.Cmd) {
	if err := identity.ValidateEmail(strings.TrimSpace(m.inputs[2].Value())); err != nil {
		m.err = err.Error()
		m.focusIdx = 1 // Git Email
		m.screen3Focus(m.focusIdx)
		return m, nil
	}
	m.err = ""
	return m, runWizardPersistGitconfigCmd(m)
}

// handleKeyLegacy handles key events for non-Screen-1 screens (backward
// compatibility with the pre-Plan-10 8-field form; used by Screen 3 in Plan 13).
func (m createWizardModel) handleKeyLegacy(msg tea.KeyMsg, key string) (createWizardModel, tea.Cmd) {
	switch key {
	case "esc":
		return m, clearModalCmd()

	case "tab", "shift+tab":
		// Cycle fields (skip locked name field in add-account mode).
		m.inputs[m.focusIdx].Blur()
		if key == "tab" {
			m.focusIdx = (m.focusIdx + 1) % len(m.inputs)
		} else {
			m.focusIdx = (m.focusIdx - 1 + len(m.inputs)) % len(m.inputs)
		}
		// Skip the name field when it is locked (add-account mode).
		if m.nameLocked && m.focusIdx == 0 {
			if key == "tab" {
				m.focusIdx = 1
			} else {
				m.focusIdx = len(m.inputs) - 1
			}
		}
		m.inputs[m.focusIdx].Focus()
		return m, nil

	case "enter":
		// On last field, validate and advance to keygen.
		if m.focusIdx == len(m.inputs)-1 || m.focusIdx == len(m.inputs) {
			return m.advanceFromForm()
		}
		// On other fields, try to validate name and advance.
		if m.focusIdx == 0 && !m.nameLocked {
			if err := identity.ValidateName(m.inputs[0].Value()); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.err = ""
		}
		// Advance to next field.
		m.inputs[m.focusIdx].Blur()
		m.focusIdx = (m.focusIdx + 1) % len(m.inputs)
		m.inputs[m.focusIdx].Focus()
		return m, nil
	}

	// Signing is a boolean toggle (Space), not a free-text field — replaces the
	// cryptic "> y" with a readable [x] enabled / [ ] disabled control (P0-3).
	if m.focusIdx == fieldSigning {
		// A real TTY reports the space bar as "space" (not " "); accept both so the
		// toggle works live, not only under the literal-" " test path (D-1/D-16).
		if key == "space" || key == " " {
			if m.inputs[fieldSigning].Value() == "n" {
				m.inputs[fieldSigning].SetValue("y")
			} else {
				m.inputs[fieldSigning].SetValue("n")
			}
		}
		return m, nil
	}
	// Match Strategy selector: ↑/↓/j/k cycle options; space/enter selects; typing
	// goes into the active sub-field (gitdir path or hasconfig URL pattern).
	if m.focusIdx == fieldMatch {
		switch key {
		case "up", "k":
			if m.matchSel > strategyGitdir {
				m.matchSel--
				m.syncMatchInput()
			}
		case "down", "j":
			if m.matchSel < strategyBoth {
				m.matchSel++
				m.syncMatchInput()
			}
		case "space", " ", "enter":
			// Selection already tracked in matchSel — no-op: space/enter here just
			// confirms the current option (nav is ↑/↓). Tab advances to Signing.
		default:
			// Forward typing to the active sub-field.
			switch m.matchSel {
			case strategyHasconfig:
				var cmd tea.Cmd
				m.matchHasconfigVal, cmd = m.matchHasconfigVal.Update(tea.KeyPressMsg{
					Code: rune(key[0]), Text: key,
				})
				return m, cmd
			case strategyBoth:
				// Forward to the gitdir sub-field when 'both' is active.
				var cmd tea.Cmd
				m.matchGitdirVal, cmd = m.matchGitdirVal.Update(tea.KeyPressMsg{
					Code: rune(key[0]), Text: key,
				})
				return m, cmd
			}
		}
		return m, nil
	}

	// Forward the ORIGINAL key event to the focused input. Rebuilding it from the
	// string (Code only) dropped the Text field, and bubbles v2 textinput inserts
	// from msg.Text — so every printable key was silently swallowed (D-1).
	var cmd tea.Cmd
	m.inputs[m.focusIdx], cmd = m.inputs[m.focusIdx].Update(msg)
	return m, cmd
}

// syncMatchInput updates the numeric value in inputs[fieldMatch] to match
// the current matchSel so buildCreateInput always reads a consistent strategy.
func (m *createWizardModel) syncMatchInput() {
	switch m.matchSel {
	case strategyHasconfig:
		m.inputs[fieldMatch].SetValue("2")
	case strategyBoth:
		m.inputs[fieldMatch].SetValue("3")
	default: // strategyGitdir
		m.inputs[fieldMatch].SetValue("1")
	}
}

// advanceFromForm validates all form fields and, if valid, initiates keygen.
// This is the boundary between form step and keygen step.
func (m createWizardModel) advanceFromForm() (createWizardModel, tea.Cmd) {
	// Validate identity name.
	nameVal := m.inputs[0].Value()
	if err := identity.ValidateName(nameVal); err != nil {
		m.err = "! invalid identity name: " + err.Error()
		m.focusIdx = 0
		m.inputs[0].Focus()
		return m, nil
	}

	// Email validation: on Screen 1 (screenSSHIdentity) the email field lives on
	// Screen 3 (Plan 13), so validation is deferred. On other screens (legacy form /
	// Screen 3) validate early so a malformed address is caught before keygen.
	if m.screen != screenSSHIdentity {
		if err := identity.ValidateEmail(m.inputs[2].Value()); err != nil {
			m.err = "! " + err.Error()
			m.inputs[m.focusIdx].Blur()
			m.focusIdx = 2
			m.inputs[2].Focus()
			return m, nil
		}
	}

	// Validate provider (optional but must be safe charset if set).
	provider := m.inputs[3].Value()
	if provider == "" {
		provider = "github.com"
	}

	m.err = ""
	m.step = wizardStepKeyGen
	// Seed the spinner.
	m.runID++
	return m, tea.Batch(
		runGenerateCmd(m.buildCreateInput(provider), m.deps),
		m.sp.Tick, // REQUIRED: seed initial spinner tick (Pattern C)
	)
}

// buildCreateInput constructs the identity.CreateInput from current form values.
// Plan 10 changes:
//   - Hostname: from hostnameVal (when user edited it) else identity.DefaultHostname(provider).
//     The hard-coded `Hostname: provider` literal is removed (T-05.7-10-02 / recipe alignment).
//   - Port: from inputs[4] parsed as int, fallback to identity.DefaultPort() (443).
//     The hard-coded `port = 22` default is removed.
func (m createWizardModel) buildCreateInput(provider string) identity.CreateInput {
	name := m.inputs[0].Value()
	gitName := m.inputs[1].Value()
	gitEmail := m.inputs[2].Value()
	if provider == "" {
		provider = m.inputs[3].Value()
	}
	if provider == "" {
		provider = "github.com"
	}

	// Port: prefer typed value; fallback to identity.DefaultPort() (443).
	portStr := m.inputs[4].Value()
	port := identity.DefaultPort()
	if portStr != "" {
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
			port = identity.DefaultPort()
		}
	}

	// EffectiveAlias: typed alias wins; blank → provider host (e.g. github.com),
	// never an invented <name>.<provider> suffix (UAT G-5 alias honesty).
	alias := identity.EffectiveAlias(m.inputs[5].Value(), provider)

	// Hostname: use the user-edited value when hostnameEdited is true; otherwise
	// derive from the recipe alt-SSH helper (e.g. ssh.github.com for github).
	// This replaces the former `Hostname: provider` literal (T-05.7-10-02).
	hostname := identity.DefaultHostname(provider)
	if m.hostnameEdited {
		hostname = m.hostnameVal.Value()
	} else if v := m.hostnameVal.Value(); v != "" {
		// Pre-filled but not user-edited: still use the pre-filled alt-SSH value.
		hostname = v
	}

	signing := strings.ToLower(strings.TrimSpace(m.inputs[7].Value())) != "n"

	// Build match list from the interactive matchSel selector (Phase 5.7, Plan 05).
	// matchGitdirVal doubles as the Screen-1 Folder row (Plan 10).
	gitdirVal := m.matchGitdirVal.Value()
	if gitdirVal == "" {
		gitdirVal = "~/git/" + name + "/"
	}
	hasconfigVal := m.matchHasconfigVal.Value()
	if hasconfigVal == "" {
		// Auto-derive from the effective alias (D-03: git@<alias>:*/**).
		hasconfigVal = defaultHasconfigPattern(alias)
	}
	var matches []gitconfig.Match
	switch m.matchSel {
	case strategyHasconfig:
		matches = []gitconfig.Match{
			{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:" + hasconfigVal},
		}
	case strategyBoth:
		matches = []gitconfig.Match{
			{Kind: gitconfig.MatchGitdir, Value: gitdirVal},
			{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:" + hasconfigVal},
		}
	default: // strategyGitdir — default (D-02)
		// Use the typed gitdir path from matchGitdirVal for the match (Plan 10:
		// the Folder row pre-fills with ~/git/<name>/ and the user can override).
		if gitdirVal != "" {
			matches = []gitconfig.Match{
				{Kind: gitconfig.MatchGitdir, Value: gitdirVal},
			}
		} else {
			matches = []gitconfig.Match{identity.DefaultMatch(name)}
		}
	}

	_ = signing // signing stored in staged, applied at PersistAll

	return identity.CreateInput{
		Name:     name,
		GitName:  gitName,
		GitEmail: gitEmail,
		Provider: provider,
		Algo:     "ed25519",
		Alias:    alias,
		Hostname: hostname,
		Port:     port,
		Matches:  matches,
	}
}

// proveHostnamePort derives the alt-SSH hostname and port for the pre-write test.
// It uses the same sources as buildCreateInput so the command shown, the test run,
// and the eventual write all agree on the same endpoint (T-05.7-12-04).
//
// Hostname: hostnameVal (edited) else identity.DefaultHostname(provider).
// Port: inputs[4] parsed as int, fallback to identity.DefaultPort() (443).
func (m createWizardModel) proveHostnamePort() (hostname string, port int) {
	provider := m.inputs[3].Value()
	if provider == "" {
		provider = "github.com"
	}
	hostname = identity.DefaultHostname(provider)
	if m.hostnameEdited {
		hostname = m.hostnameVal.Value()
	} else if v := m.hostnameVal.Value(); v != "" {
		hostname = v
	}
	port = identity.DefaultPort()
	if _, err := fmt.Sscanf(m.inputs[4].Value(), "%d", &port); err != nil {
		port = identity.DefaultPort()
	}
	return hostname, port
}

// initProve transitions the wizard to the prove step and seeds the prove loop.
// Called when the user presses Enter on the upload step.
//
// The pre-write test runs against the alt-SSH endpoint (hostname/port from
// proveHostnamePort, not the raw provider hostname) so the command shown, the
// test executed, and the eventual write all agree (T-05.7-12-04).
func (m createWizardModel) initProve() (createWizardModel, tea.Cmd) {
	m.step = wizardStepProve1Running
	m.runID++
	keyPath := m.staged.TempPrivatePath
	hostname, port := m.proveHostnamePort()
	return m, tea.Batch(
		runPreWriteCmd(m.deps.identity.PreWrite, keyPath, hostname, port),
		m.sp.Tick,
	)
}

// update handles all non-form messages for the wizard.
func (m createWizardModel) update(msg tea.Msg) (createWizardModel, tea.Cmd) {
	switch msg := msg.(type) {

	case keygenResultMsg:
		if msg.err != nil {
			m.err = fmt.Sprintf("keygen failed: %v", msg.err)
			m.step = wizardStepForm
			return m, nil
		}
		m.staged = msg.staged
		m.step = wizardStepUpload
		// Auto-copy pub key to clipboard on upload step entry.
		return m, runClipboardCopyCmd(m.staged.PubLine)

	case clipboardResultMsg:
		m.copyErr = msg.err
		return m, nil

	case preWriteResultMsg:
		m.phase1Result = msg.result
		switch msg.result.Outcome {
		case tester.PASS:
			m.step = wizardStepProve1Done
			// Phase 2 (resolved alias test): stage the identity's Host block into a
			// throwaway config file (with the staged key as IdentityFile) and test
			// the alias against it via `ssh -F <tempconfig> -i <key>`. A typed alias
			// is not a DNS name, so it can only resolve once a Host stanza exists; we
			// make it exist in a temp file instead of mutating ~/.ssh/config for a
			// test (UAT G-5). The real config is written only at the final confirmed
			// write. See runResolvedViaCmd.
			return m, runResolvedViaCmd(m)
		default:
			m.step = wizardStepProve1Failed
			m.confirmActive = false
		}
		return m, nil

	case resolvedResultMsg:
		m.phase2Result = msg.result
		m.phase2Resolved = msg.resolved
		if msg.err != nil && msg.result.Output == "" {
			// Staging the temp config failed before any ssh ran — surface the reason
			// in the test output so the failure screen explains it.
			m.phase2Result.Output = msg.err.Error()
		}
		switch msg.result.Outcome {
		case tester.PASS:
			m.step = wizardStepProve2Done
			m.confirmActive = true
		default:
			m.step = wizardStepProve2Failed
			m.confirmActive = false
		}
		return m, nil

	case wizardCreateResultMsg:
		if msg.err != nil {
			m.result = fmt.Sprintf("write failed: %v", msg.err)
		} else {
			if msg.result.SSHBackup != "" {
				m.backupPath = msg.result.SSHBackup
			}
			// Capture the written SSH Host block for the Screen-4 review.
			m.leg1SSHPreview = msg.result.SSHPreview
			// LEG-1 write (PersistSSH from Screen 2) succeeded: advance to Screen 3
			// (Git config, LEG 2). A mid-exit after this point leaves a valid
			// SSH-only identity (doctor flags it incomplete; accepted design per the
			// Screen-2 mandate). Reset the focus cycle to the first git-config field.
			m.step = wizardStepForm
			m.screen = screenGitConfig
			m.focusIdx = 0
			m.screen3Focus(0)
			m.result = ""
		}
		return m, nil

	case wizardLeg2ResultMsg:
		if msg.err != nil {
			// Stay on Screen 3 and surface the failure; nothing partial is left
			// (PersistGitconfig writes each artifact with its own backup).
			m.err = fmt.Sprintf("git config write failed: %v", msg.err)
		} else {
			m.leg2Result = msg.result
			m.err = ""
			// LEG-2 written: advance to Screen 4 (read-only review of all artifacts).
			m.screen = screenReview
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		key := msg.String()
		return m.handleWizardKey(key)
	}

	return m, nil
}

// handleWizardKey processes key presses in steps 2-4 of the wizard.
func (m createWizardModel) handleWizardKey(key string) (createWizardModel, tea.Cmd) {
	// [c] re-copies the public key from every post-keygen screen (upload AND the
	// inline test screen). One screen, no view switch: a mis-paste is always
	// recoverable in place without restarting the wizard (D-16 round 3, Q5).
	if key == "c" && m.step >= wizardStepUpload {
		return m, runClipboardCopyCmd(m.staged.PubLine)
	}

	switch m.step {
	case wizardStepKeyGen:
		// No user input during keygen; Esc cancels.
		if key == "esc" {
			return m, clearModalCmd()
		}

	case wizardStepUpload:
		switch key {
		case "esc":
			// Keep the key at ~/.ssh/gitid_<name>, dismiss modal, show toast.
			name := m.inputs[0].Value()
			toast := "Key kept at ~/.ssh/gitid_" + name + " — run 'gitid doctor' when ready."
			return m, tea.Batch(clearModalCmd(), setToastCmd(toast, StyleFaint))
		case "enter":
			// Advance to prove loop.
			return m.initProve()
		}

	case wizardStepProve1Running, wizardStepProve2Running, wizardStepProve1Done:
		// No user interaction during running/phase1-done phases; Esc cancels.
		if key == "esc" {
			return m, clearModalCmd()
		}

	case wizardStepProve2Done:
		// Both phases passed — write gate open. On Enter, write LEG 1 via PersistSSH
		// (key + Host block), then advance to Screen 3. LEG 2 (gitconfig) is Plan 13.
		switch key {
		case "enter":
			if m.confirmActive {
				m.confirmActive = false
				return m, runWizardPersistSSHCmd(m)
			}
		case "esc":
			return m, clearModalCmd()
		}

	case wizardStepProve1Failed, wizardStepProve2Failed:
		switch key {
		case "r":
			// Retry: re-run phase 1 using the alt-SSH endpoint (same as initProve).
			m.confirmActive = false
			m.skipConfirmPending = false
			m.runID++
			keyPath := m.staged.TempPrivatePath
			hostname, port := m.proveHostnamePort()
			m.step = wizardStepProve1Running
			return m, tea.Batch(
				runPreWriteCmd(m.deps.identity.PreWrite, keyPath, hostname, port),
				m.sp.Tick,
			)
		case "s":
			// Skip test — requires an explicit second confirm (double-confirm gate,
			// FIX-CREATE-01, T-05.7-12-01). The warning is shown after [s] and a
			// second Enter is required to proceed.
			if !m.confirmActive {
				m.skipConfirmPending = true
				m.skipWarning = "! Written without authentication verification. [warning]"
			}
		case "q":
			// Quit: nothing has been written yet. Show a recovery toast that
			// explains the no-write state and how to finish (G-2 no-write clarity).
			name := m.inputs[0].Value()
			keyPath := m.staged.FinalPrivatePath
			if keyPath == "" {
				keyPath = "~/.ssh/id_ed25519_" + name
			}
			toast := "Nothing written. Key kept at " + keyPath +
				". Re-run 'gitid create' and press [s] to write offline, or upload the key then re-run."
			return m, tea.Batch(clearModalCmd(), setToastCmd(toast, StyleFaint))
		case "esc":
			return m, clearModalCmd()
		case "enter":
			if m.skipConfirmPending {
				// Explicit skip confirm (second Enter): write LEG 1 via PersistSSH
				// without auth verification. The double-confirm gate is preserved
				// (T-05.7-12-01 / FIX-CREATE-01).
				m.skipConfirmPending = false
				return m, runWizardPersistSSHCmd(m)
			}
		}

	case wizardStepWritten:
		if key == "esc" || key == "enter" {
			return m, clearModalCmd()
		}
	}

	return m, nil
}

// view renders the wizard modal at the given terminal width.
func (m createWizardModel) view(w int) string {
	mw := modalWidth(w)
	var sb strings.Builder

	switch m.step {
	case wizardStepForm:
		m.viewForm(&sb, mw)
	case wizardStepKeyGen:
		m.viewKeygen(&sb)
	case wizardStepUpload:
		m.viewUpload(&sb)
	default:
		m.viewProve(&sb)
	}

	return StyleModal.Width(mw).Render(sb.String())
}

// readOnlyRow renders a label + read-only value row.
func readOnlyRow(label, value string) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		StyleLabel.Render(label),
		" ",
		StyleReadOnly.Render(value),
	)
}

// viewForm renders the form step, dispatching by staged screen:
//
//	Screen 1 (screenSSHIdentity) → SSH-identity rows (Plan 10)
//	Screen 3 (screenGitConfig)   → Git-config form / LEG 2 (Plan 13)
//	Screen 4 (screenReview)      → read-only review (Plan 13)
func (m createWizardModel) viewForm(sb *strings.Builder, mw int) {
	switch m.screen {
	case screenSSHIdentity:
		m.viewScreen1(sb, mw)
	case screenGitConfig:
		m.viewScreen3(sb, mw)
	case screenReview:
		m.viewScreen4(sb, mw)
	default:
		m.viewLegacyForm(sb, mw)
	}
}

// viewScreen3 renders Screen 3 (Git Configuration, LEG 2): Git Name / Git Email,
// the match-strategy selector (with editable Folder/gitdir + hasconfig sub-fields)
// and a live includeIf preview, and the signing toggle. SSH fields are not shown
// (collected on Screens 1–2). On confirm it writes LEG 2 via PersistGitconfig.
func (m createWizardModel) viewScreen3(sb *strings.Builder, _ int) {
	name := m.inputs[0].Value()
	title := "Create Identity — Git Configuration"
	if name != "" {
		title = "Create Identity: " + name + " — Git Configuration"
	}
	sb.WriteString(StyleModalTitle.Render(title))
	sb.WriteString("\n\n")

	sb.WriteString(StyleFaint.Render("SSH config written ✓  — now the git-side settings (LEG 2).") + "\n\n")

	// Row 0: Git Name (inputs[1]).
	sb.WriteString(renderFormField("Git Name:       ", m.inputs[1].View(), m.focusIdx == 0) + "\n")
	// Row 1: Git Email (inputs[2]).
	sb.WriteString(renderFormField("Git Email:      ", m.inputs[2].View(), m.focusIdx == 1) + "\n")
	// Row 2: Match Strategy selector (gitdir / hasconfig / both) + live includeIf preview.
	sb.WriteString(m.renderMatchSelector("Match Strategy: ", m.focusIdx == 2))
	// Row 3: Signing toggle (inputs[fieldSigning]).
	signState := "[ ] disabled  (space toggles)"
	if m.inputs[fieldSigning].Value() != "n" {
		signState = "[x] enabled   (space toggles)"
	}
	sb.WriteString(renderFormField("Signing:        ", signState, m.focusIdx == 3) + "\n")

	if m.err != "" {
		sb.WriteString("\n")
		sb.WriteString(SeverityStyle(doctor.SeverityWarning).Render("! " + m.err))
		sb.WriteString("\n")
	}

	var footer string
	if m.focusIdx == 2 {
		footer = "[↑↓ select strategy · type to edit folder/URL · Tab next · Enter write · Esc cancel]"
	} else {
		footer = "[Tab cycle fields · Space toggle signing · Enter write git config · Esc cancel]"
	}
	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render(footer))
}

// viewScreen4 renders Screen 4 (read-only review): a trustworthy summary of every
// artifact written to disk — the SSH Host block (LEG 1), the gitconfig includeIf +
// fragment + allowed_signers line (LEG 2), and the live ssh -G resolution. Nothing
// is editable; Enter or Esc closes the wizard.
func (m createWizardModel) viewScreen4(sb *strings.Builder, _ int) {
	name := m.inputs[0].Value()
	sb.WriteString(StyleModalTitle.Render("Identity Created — Review") + "\n\n")
	sb.WriteString(StylePass.Render("✓") + " Identity " + StyleBody.Render(name) + " is configured. Everything below is on disk.\n\n")

	section := func(heading, body string) {
		if strings.TrimSpace(body) == "" {
			return
		}
		sb.WriteString(StyleBody.Render(heading) + "\n")
		for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
			sb.WriteString(StyleFaint.Render("  "+line) + "\n")
		}
		sb.WriteString("\n")
	}

	section("~/.ssh/config — Host block:", m.leg1SSHPreview)
	section("~/.gitconfig — includeIf:", m.leg2Result.GitconfigPreview)
	section("~/.gitconfig.d/"+name+" — fragment:", m.leg2Result.FragmentPreview)
	if m.leg2Result.AllowedSignersLine != "" {
		section("~/.ssh/allowed_signers:", m.leg2Result.AllowedSignersLine)
	}

	// Backups written (CLAUDE.md safe-write invariant — show where each landed).
	var backups []string
	if m.backupPath != "" {
		backups = append(backups, "ssh/config → "+m.backupPath)
	}
	if m.leg2Result.GitconfigBackup != "" {
		backups = append(backups, "gitconfig → "+m.leg2Result.GitconfigBackup)
	}
	if m.leg2Result.AllowedSignersBackup != "" {
		backups = append(backups, "allowed_signers → "+m.leg2Result.AllowedSignersBackup)
	}
	if len(backups) > 0 {
		section("Backups:", strings.Join(backups, "\n"))
	}

	sb.WriteString(StyleFaint.Render("[Enter or Esc to close]"))
}

// viewScreen1 renders Screen 1: the SSH-identity form with always-visible editable
// rows for name / key algorithm / provider / SSH alias / hostname / port / folder.
// Git Name / Git Email / Match Strategy / Signing are NOT rendered here (Plan 10).
func (m createWizardModel) viewScreen1(sb *strings.Builder, _ int) {
	title := "Create Identity — SSH Setup"
	if m.nameLocked {
		name := m.inputs[0].Value()
		if name != "" {
			title = "Add Account: " + name + " — SSH Setup"
		} else {
			title = "Add Account — SSH Setup"
		}
	}
	sb.WriteString(StyleModalTitle.Render(title))
	sb.WriteString("\n\n")

	// Row 0: Identity Name.
	if m.nameLocked {
		sb.WriteString(readOnlyRow("Identity Name:  ", m.inputs[0].Value()) + "\n")
	} else {
		sb.WriteString(renderFormField("Identity Name:  ", m.inputs[0].View(), m.focusIdx == 0) + "\n")
	}

	// Row: Key Algorithm (read-only; ed25519 is the only supported algo).
	sb.WriteString(readOnlyRow("Key Algorithm:  ", "ed25519") + "\n")

	// Row 1: Provider (inputs[3]).
	sb.WriteString(renderFormField("Provider:       ", m.inputs[3].View(), m.focusIdx == 1) + "\n")

	// Row 2: SSH Alias (inputs[5]). Blank → provider host (e.g. github.com); the
	// live preview below reflects the effective value (WYSIWYG, UAT G-5 honesty).
	name := m.inputs[0].Value()
	sb.WriteString(renderFormField("SSH Alias:      ", m.inputs[5].View(), m.focusIdx == 2) + "\n")

	// Row 3: Hostname (hostnameVal).
	sb.WriteString(renderFormField("Hostname:       ", m.hostnameVal.View(), m.focusIdx == hostnameFocusIdx()) + "\n")

	// Row 4: Port (inputs[4]) — last Screen-1 field. Folder (gitdir) moved to
	// Screen 3 (Git config): it drives the ~/.gitconfig includeIf match, not the
	// SSH config, and is not needed for the alias connectivity test.
	sb.WriteString(renderFormField("Port:           ", m.inputs[4].View(), m.focusIdx == 4) + "\n")

	// Live Host-block preview (Task 3, G-2 preview half): always shown when name
	// is non-empty so the user sees the exact alias block before any write.
	if name != "" {
		sb.WriteString("\n")
		m.renderSSHBlockPreview(sb)
	}

	if m.err != "" {
		sb.WriteString("\n")
		sb.WriteString(SeverityStyle(doctor.SeverityWarning).Render("! " + m.err))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render("[Tab cycle fields · Enter advance · Esc cancel]"))
}

// renderSSHBlockPreview writes a live preview of the exact `Host <alias>` block
// that WILL be written to ~/.ssh/config via sshconfig.RenderHostBlock — the same
// renderer PersistSSH uses (T-05.7-10-02: no preview/write drift).
//
// The preview derives alias, hostname, port, and finalKeyPath from the SAME
// sources buildCreateInput uses, so the preview is byte-faithful to the write.
// It uses staged.FinalPrivatePath when set; otherwise derives the deterministic
// key path (~/.ssh/id_<algo>_<name>) as Generate would name it.
//
// Guard: caller must ensure name != "" before calling (no preview for empty name).
func (m createWizardModel) renderSSHBlockPreview(sb *strings.Builder) {
	name := m.inputs[0].Value()
	provider := m.inputs[3].Value()
	if provider == "" {
		provider = "github.com"
	}

	// Alias: typed value wins; blank → provider host (WYSIWYG, UAT G-5 honesty).
	alias := identity.EffectiveAlias(m.inputs[5].Value(), provider)

	// Hostname: use the same source as buildCreateInput (T-05.7-10-02 parity).
	hostname := identity.DefaultHostname(provider)
	if m.hostnameEdited {
		hostname = m.hostnameVal.Value()
	} else if v := m.hostnameVal.Value(); v != "" {
		hostname = v
	}

	// Port: prefer typed value, fallback to identity.DefaultPort().
	port := identity.DefaultPort()
	if portStr := m.inputs[4].Value(); portStr != "" {
		var p int
		if _, err := fmt.Sscanf(portStr, "%d", &p); err == nil {
			port = p
		}
	}

	// IdentityFile path: use staged.FinalPrivatePath when already generated;
	// otherwise derive the deterministic final path the Generate seam uses.
	finalKeyPath := m.staged.FinalPrivatePath
	if finalKeyPath == "" {
		finalKeyPath = "~/.ssh/id_ed25519_" + name
	}

	// Render via sshconfig.RenderHostBlock — same call PersistSSH makes.
	// No hand-built Host block strings (T-05.7-10-02, discipline from T-05.7-05-01).
	hostBlock := sshconfig.RenderHostBlock(alias, hostname, port, finalKeyPath, provider)

	sb.WriteString(StyleFaint.Render("Will write to ~/.ssh/config:"))
	sb.WriteString("\n")
	// Render each line with a 2-space indent via StyleFaint for visual consistency
	// with the includeIf preview (StyleFaint/StyleBody pattern).
	for _, line := range strings.Split(strings.TrimRight(hostBlock, "\n"), "\n") {
		sb.WriteString(StyleFaint.Render("  " + line))
		sb.WriteString("\n")
	}
}

// viewLegacyForm renders the full 8-field form for non-Screen-1 screens.
// Kept for Plan 13 compatibility (Screen 3: Git Name / Email / Match / Signing).
func (m createWizardModel) viewLegacyForm(sb *strings.Builder, _ int) {
	title := "Create Identity"
	if m.nameLocked {
		name := m.inputs[0].Value()
		if name != "" {
			title = "Add Account: " + name
		} else {
			title = "Add Account"
		}
	}
	sb.WriteString(StyleModalTitle.Render(title))
	sb.WriteString("\n\n")

	labels := []string{
		"Identity Name:  ",
		"Git Name:       ",
		"Git Email:      ",
		"Provider:       ",
		"Port:           ",
		"SSH Alias:      ",
		"Match Strategy: ",
		"Signing:        ",
	}

	for i, inp := range m.inputs {
		switch {
		case m.nameLocked && i == 0:
			sb.WriteString(readOnlyRow(labels[i], inp.Value()) + "\n")
		case i == fieldSigning:
			state := "[ ] disabled  (space toggles)"
			if inp.Value() != "n" {
				state = "[x] enabled   (space toggles)"
			}
			sb.WriteString(renderFormField(labels[i], state, i == m.focusIdx) + "\n")
		case i == fieldMatch:
			sb.WriteString(m.renderMatchSelector(labels[i], i == m.focusIdx))
		default:
			sb.WriteString(renderFormField(labels[i], inp.View(), i == m.focusIdx) + "\n")
		}
	}

	if m.err != "" {
		sb.WriteString("\n")
		sb.WriteString(SeverityStyle(doctor.SeverityWarning).Render("! " + m.err))
		sb.WriteString("\n")
	}

	var footer string
	if m.focusIdx == fieldMatch {
		footer = "[↑↓ select  space choose  tab next field  esc cancel]"
	} else {
		footer = "[Tab cycle fields · space toggle · Enter advance · Esc cancel]"
	}
	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render(footer))
}

// renderMatchSelector renders the expanded match-strategy radio selector block
// (UI-SPEC §1a). It returns the complete rendered string for the Match Strategy
// field row including options, active sub-fields, and the live includeIf preview.
//
// All includeIf conditions are produced via liveIncludeIfPreview → RenderIncludeIf
// so no [includeIf "..."] string is hand-built here (T-05.7-05-01).
func (m createWizardModel) renderMatchSelector(label string, focused bool) string {
	var sb strings.Builder

	// Header row: label + collapsed summary when not focused, expanded when focused.
	if !focused {
		summaries := map[matchStrategy]string{
			strategyGitdir:    "gitdir (folder)",
			strategyHasconfig: "hasconfig (repo URL)",
			strategyBoth:      "both (gitdir + hasconfig)",
		}
		row := lipgloss.JoinHorizontal(
			lipgloss.Center,
			StyleLabel.Render(label),
			" ",
			StyleReadOnly.Render(summaries[m.matchSel]),
		)
		sb.WriteString(row + "\n")
		// Show compact live includeIf preview even when collapsed (D-06: strategy is never opaque).
		name := m.inputs[0].Value()
		if name != "" {
			alias := m.inputs[5].Value()
			if alias == "" {
				alias = identity.DefaultAlias(name, m.inputs[3].Value())
			}
			gitdirVal := m.matchGitdirVal.Value()
			if gitdirVal == "" {
				gitdirVal = "~/git/" + name + "/"
			}
			hasconfigVal := m.matchHasconfigVal.Value()
			if hasconfigVal == "" {
				hasconfigVal = defaultHasconfigPattern(alias)
			}
			preview := liveIncludeIfPreview(m.matchSel, name, alias, gitdirVal, hasconfigVal)
			sb.WriteString(StyleFaint.Render("includeIf preview:") + "\n")
			sb.WriteString(lipgloss.NewStyle().PaddingLeft(4).Render(StyleFaint.Render(preview)) + "\n")
		}
		return sb.String()
	}

	// Expanded selector (focused) — UI-SPEC §1a layout.
	sb.WriteString(StyleLabel.Render(label) + "\n\n")

	radio := func(sel matchStrategy, desc string) string {
		if m.matchSel == sel {
			return StylePass.Render("[x]") + " " + StyleBody.Render(desc)
		}
		return StyleFaint.Render("[ ]") + " " + StyleBody.Render(desc)
	}

	pad2 := lipgloss.NewStyle().PaddingLeft(2)
	pad4 := lipgloss.NewStyle().PaddingLeft(4)

	// Option 1: gitdir
	sb.WriteString(pad2.Render(radio(strategyGitdir, "gitdir (folder)  — matches repos under a directory")) + "\n")
	if m.matchSel == strategyGitdir || m.matchSel == strategyBoth {
		sb.WriteString(pad4.Render(StyleFaint.Render("gitdir path:  ")+m.matchGitdirVal.View()) + "\n")
	}
	sb.WriteString("\n")

	// Option 2: hasconfig
	sb.WriteString(pad2.Render(radio(strategyHasconfig, "hasconfig (repo URL) — matches repos by remote URL")) + "\n")
	if m.matchSel == strategyHasconfig || m.matchSel == strategyBoth {
		sb.WriteString(pad4.Render(StyleFaint.Render("URL pattern:  ")+m.matchHasconfigVal.View()) + "\n")
	}
	sb.WriteString("\n")

	// Option 3: both
	sb.WriteString(pad2.Render(radio(strategyBoth, "both (gitdir + hasconfig, OR-applied by git)")) + "\n\n")

	// Live includeIf preview — calls RenderIncludeIf exclusively (T-05.7-05-01).
	name := m.inputs[0].Value()
	alias := m.inputs[5].Value()
	if alias == "" {
		alias = identity.DefaultAlias(name, m.inputs[3].Value())
	}
	gitdirVal := m.matchGitdirVal.Value()
	if gitdirVal == "" {
		gitdirVal = "~/git/" + name + "/"
	}
	hasconfigVal := m.matchHasconfigVal.Value()
	if hasconfigVal == "" {
		hasconfigVal = defaultHasconfigPattern(alias)
	}

	if name != "" {
		preview := liveIncludeIfPreview(m.matchSel, name, alias, gitdirVal, hasconfigVal)
		sb.WriteString(StyleHeader.Render("Preview:") + "\n")
		sb.WriteString(pad4.Render(StyleFaint.Render(preview)) + "\n")
		if m.matchSel == strategyBoth {
			sb.WriteString(pad4.Render(StyleFaint.Render("(git applies these as OR — either match activates the fragment)")) + "\n")
		}
	}

	// Recipe alignment note (UI-SPEC copywriting).
	sb.WriteString(StyleFaint.Render("  Recipe: hasconfig is the primary match; gitdir is the alternative.") + "\n")
	sb.WriteString(StyleFaint.Render("  gitid default for new identities: gitdir.") + "\n")

	return sb.String()
}

// viewKeygen renders the keygen step.
func (m createWizardModel) viewKeygen(sb *strings.Builder) {
	sb.WriteString(StyleModalTitle.Render("Create Identity — Generating Key"))
	sb.WriteString("\n\n")
	sb.WriteString(m.sp.View() + " Generating ed25519 key...\n")
	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render("Once complete, the public key will be shown for upload."))
}

// renderUploadHeader renders the persistent upload region shown on BOTH the
// upload step and the inline test step (D-16 round 3: one screen, no view
// switch). It prints the clipboard status, the FULL public key (never truncated,
// so a manual copy is always complete), a [c] copy affordance, and the provider
// upload instructions. Keeping these visible during and after the test means a
// mis-paste is re-copyable in place without restarting the wizard (Q2/Q3/Q5).
func (m createWizardModel) renderUploadHeader(sb *strings.Builder) {
	if m.copyErr != nil {
		sb.WriteString(SeverityStyle(doctor.SeverityInfo).Render("! clipboard copy failed [info] — copy the full key below manually."))
	} else {
		sb.WriteString(StylePass.Render("✓") + " Public key copied to clipboard.")
	}
	sb.WriteString("\n\n")

	// Full public key — never truncated, so a manual copy is always complete.
	sb.WriteString(StyleBody.Render(strings.TrimRight(m.staged.PubLine, "\n")) + "\n")
	sb.WriteString(StyleFaint.Render("[c] copy key") + "\n\n")

	// Key file locations (D-16 round 3): the test runs `ssh -i <private>` against
	// the STAGED temp path — the key is not in ~/.ssh until the test passes — so
	// show both paths to make the test command verifiable and the key findable.
	if m.staged.TempPrivatePath != "" {
		sb.WriteString(StyleFaint.Render("Key files (staged until the test passes, then installed):") + "\n")
		sb.WriteString(StyleFaint.Render("  private (tested now): "+m.staged.TempPrivatePath) + "\n")
		sb.WriteString(StyleFaint.Render("  installs to: "+m.staged.FinalPrivatePath+"  (+ .pub)") + "\n\n")
	}

	// Upload instructions for the provider (hostname only; strip port/path).
	provider := m.inputs[3].Value()
	if provider == "" {
		provider = "github.com"
	}
	providerHost := strings.SplitN(provider, ":", 2)[0]
	sb.WriteString(StyleFaint.Render(upload.Instructions(providerHost)))
	sb.WriteString("\n")
}

// renderTestDetail prints the exact ssh command and raw output for a test phase
// (TEST-03) so a failure is self-diagnosing — e.g. a truncated/mis-pasted key
// surfaces here as "Permission denied (publickey)" under the command that ran.
func renderTestDetail(sb *strings.Builder, res tester.Result) {
	if res.Command != "" {
		sb.WriteString(StyleFaint.Render("  $ "+res.Command) + "\n")
	}
	if res.Output != "" {
		sb.WriteString(StyleFaint.Render("  "+strings.TrimRight(res.Output, "\n")) + "\n")
	}
}

// viewUpload renders the upload + test screen before the test is started.
// Shows the FULL pre-run ssh command (G-3: consistent command display on pre-run)
// derived from tester.PreWriteCommand so the pre-run display is byte-identical to
// what initProve will run.
func (m createWizardModel) viewUpload(sb *strings.Builder) {
	sb.WriteString(StyleModalTitle.Render("Create Identity — Upload & Test"))
	sb.WriteString("\n\n")
	m.renderUploadHeader(sb)

	// Pre-run command: show the exact command that WILL run (G-3 pre-run half).
	// Derives hostname/port from the same sources as initProve (proveHostnamePort).
	if m.staged.TempPrivatePath != "" {
		hostname, port := m.proveHostnamePort()
		preCmd := tester.PreWriteCommand(m.staged.TempPrivatePath, hostname, port)
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("Test command that will run:") + "\n")
		sb.WriteString(StyleFaint.Render("  $ "+preCmd) + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render("[Enter] paste the key into the page above, then test · [Esc] keep key, write nothing"))
}

// viewProve renders the SAME upload screen with the inline test result below it,
// so the key, [c] copy, and instructions never disappear while testing (D-16
// round 3: one screen, no view switch).
//
// G-3 (command+path visibility): the exact ssh command is shown in ALL states —
// pre-run (viewUpload shows it), Phase-1 PASS (renderTestCommandLine after the
// ✓ line), Phase-2 PASS (renderTestCommandLine), and failure (renderTestDetail,
// which prints command+output). The tested key path is kept visible via
// renderUploadHeader on every sub-step.
func (m createWizardModel) viewProve(sb *strings.Builder) {
	sb.WriteString(StyleModalTitle.Render("Create Identity — Upload & Test"))
	sb.WriteString("\n\n")
	m.renderUploadHeader(sb)
	sb.WriteString("\n")

	provider := m.inputs[3].Value()
	if provider == "" {
		provider = "github.com"
	}

	// Test 1 (general): the key reaches the provider's SSH endpoint directly.
	// Runs first; Test 2 is only triggered after it passes.
	switch m.step {
	case wizardStepProve1Running:
		sb.WriteString(m.sp.View() + " Test 1/2 (general): checking key & endpoint reachability...\n")
	case wizardStepProve1Done, wizardStepProve2Running, wizardStepProve2Done, wizardStepWritten:
		sb.WriteString(StylePass.Render("✓") + " Test 1/2 (general): key & endpoint reachable\n")
		// G-3: show the exact command even on PASS (not only failure).
		renderTestCommandLine(sb, m.phase1Result)
	case wizardStepProve1Failed:
		sb.WriteString(SeverityStyle(doctor.SeverityError).Render("✗") + " Test 1/2 (general): key/endpoint unreachable [critical]\n")
		renderTestDetail(sb, m.phase1Result)
		renderNoWriteGuidance(sb, m.phase1Result, provider)
		sb.WriteString("\n")
		sb.WriteString(renderProveActions(m.skipConfirmPending))
	}

	// Test 2 (alias): the alias resolves through the staged config to the same
	// endpoint. Only shown/triggered after Test 1 passes.
	if m.step >= wizardStepProve2Running && m.step != wizardStepProve1Failed {
		switch m.step {
		case wizardStepProve2Running:
			sb.WriteString(m.sp.View() + " Test 2/2 (alias): checking the alias resolves through your config...\n")
		case wizardStepProve2Done, wizardStepWritten:
			sb.WriteString(StylePass.Render("✓") + " Test 2/2 (alias): alias config resolves correctly\n")
			// G-3: show test-2 command on PASS.
			renderTestCommandLine(sb, m.phase2Result)
		case wizardStepProve2Failed:
			sb.WriteString(SeverityStyle(doctor.SeverityError).Render("✗") + " Test 2/2 (alias): alias config resolution failed [critical]\n")
			renderTestDetail(sb, m.phase2Result)
			renderNoWriteGuidance(sb, m.phase2Result, provider)
			sb.WriteString(renderProveActions(m.skipConfirmPending))
		}
	}

	// Confirm gate.
	if m.confirmActive {
		sb.WriteString("\n")
		sb.WriteString(StyleBody.Render("Both tests passed. Write SSH config (LEG 1)?"))
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("[Enter to write · Esc to cancel]"))
	}

	// Skip warning (double-confirm gate, FIX-CREATE-01, T-05.7-12-01).
	if m.skipWarning != "" {
		sb.WriteString("\n")
		sb.WriteString(SeverityStyle(doctor.SeverityWarning).Render(m.skipWarning) + "\n")
		sb.WriteString(StyleFaint.Render("[Enter to confirm skip · Esc to cancel]"))
	}

	// Result.
	if m.result != "" {
		sb.WriteString("\n" + m.result + "\n")
		sb.WriteString(StyleFaint.Render("[Esc to close]"))
	}
}

// renderTestCommandLine renders the `$ <command>` line for a completed test phase
// when res.Command is non-empty. Called after Phase-1 PASS and Phase-2 PASS to
// keep the exact tested command visible on success (G-3: consistent command display).
// Output is NOT printed for failing phases — that is renderTestDetail's job (failure
// prints both command and output).
func renderTestCommandLine(sb *strings.Builder, res tester.Result) {
	if res.Command == "" {
		return
	}
	sb.WriteString(StyleFaint.Render("  $ "+res.Command) + "\n")
}

// renderNoWriteGuidance writes the no-write state explanation for a failed prove
// phase. It states that nothing has been written yet, explains WHY (keyed off
// res.Outcome), and lists the two ways forward — including the discoverable [s]
// offline escape (G-2: clear no-write feedback + discoverable skip).
func renderNoWriteGuidance(sb *strings.Builder, res tester.Result, provider string) {
	if provider == "" {
		provider = "provider"
	}
	sb.WriteString("\n")
	sb.WriteString(SeverityStyle(doctor.SeverityWarning).Render("Nothing has been written yet.") + "\n")

	// WHY line: keyed off the outcome.
	switch res.Outcome {
	case tester.ReachableNotUploaded:
		sb.WriteString(StyleFaint.Render("Why: the public key is not on "+provider+" yet (authentication key + signing key).") + "\n")
	default: // tester.Failure
		sb.WriteString(StyleFaint.Render("Why: could not reach "+provider+" — check network or port.") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render("How to proceed:") + "\n")
	sb.WriteString(StyleFaint.Render("  1. Upload the public key to "+provider+" (see instructions above), then [r] retry.") + "\n")
	sb.WriteString(StyleFaint.Render("  2. [s] skip & write the SSH alias block offline — you can authenticate later.") + "\n")
}

// runWizardPersistSSHCmd dispatches the async PersistSSH (LEG-1) write: persists
// the staged keypair to ~/.ssh/ and writes the Host block to ~/.ssh/config via
// identity.PersistSSH. It does NOT write gitconfig/fragment/allowed_signers (LEG 2
// is dispatched by Plan 13 on Screen 3).
//
// Mirroring runWizardCreateCmd in structure (recover()-guarded goroutine,
// wizardCreateResultMsg return). T-05.7-12-02: all writes go through PersistSSH →
// deps.WriteSSH → filewriter (timestamped backup + atomic + idempotent managed
// block); no os.WriteFile added.
func runWizardPersistSSHCmd(m createWizardModel) tea.Cmd {
	in := m.buildCreateInput(m.inputs[3].Value())
	staged := m.staged
	deps := m.deps
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = wizardCreateResultMsg{err: fmt.Errorf("persist-ssh panicked: %v", r)}
			}
		}()
		res, err := identity.PersistSSH(in, staged, deps.identity)
		return wizardCreateResultMsg{result: res, err: err}
	}
}

// runWizardPersistGitconfigCmd dispatches the LEG-2 write (Screen 3 confirm):
// identity.PersistGitconfig writes the ~/.gitconfig.d/<name> fragment, the
// includeIf block in ~/.gitconfig, and the ~/.ssh/allowed_signers line — each
// with a timestamped backup, idempotent. It does NOT re-write the SSH config
// (that was LEG 1, Screen 2). The fragment/gitconfig/allowed_signers paths are
// resolved to absolute locations here (mirroring the CLI, which writes absolute
// fragment paths so `git config --file` can create the file).
func runWizardPersistGitconfigCmd(m createWizardModel) tea.Cmd {
	in := m.buildCreateInput(m.inputs[3].Value())
	home, herr := os.UserHomeDir()
	if herr == nil && home != "" {
		in.FragmentPath = filepath.Join(home, ".gitconfig.d", in.Name)
		in.GitconfigPath = filepath.Join(home, ".gitconfig")
		in.AllowedSignersPath = filepath.Join(home, ".ssh", "allowed_signers")
	}
	staged := m.staged
	deps := m.deps
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = wizardLeg2ResultMsg{err: fmt.Errorf("persist-gitconfig panicked: %v", r)}
			}
		}()
		if herr != nil {
			return wizardLeg2ResultMsg{err: fmt.Errorf("resolving home dir: %w", herr)}
		}
		res, err := identity.PersistGitconfig(in, staged, deps.identity)
		return wizardLeg2ResultMsg{result: res, err: err}
	}
}

// runGenerateCmd dispatches the async ed25519 key generation command.
// Wraps the Generate seam in a goroutine with recover() for safety (T-05.6-17).
func runGenerateCmd(in identity.CreateInput, deps tuiDeps) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = keygenResultMsg{err: fmt.Errorf("keygen panicked: %v", r)}
			}
		}()
		fn := deps.identity.Generate
		if fn == nil {
			return keygenResultMsg{err: fmt.Errorf("generate seam is nil")}
		}
		staged, err := fn(in)
		return keygenResultMsg{staged: staged, err: err}
	}
}
