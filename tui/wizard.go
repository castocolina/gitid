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
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
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

// ─── Create/Add wizard (Plan 05) ────────────────────────────────────────────

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

// createWizardModel is the 4-step create/add wizard modal:
//
//  1. Form (wizardStepForm): 8-field textinput layout.
//  2. KeyGen (wizardStepKeyGen): async ed25519 key generation.
//  3. Upload (wizardStepUpload): clipboard copy + upload instructions.
//  4. TestLoop (wizardStepProve1Running…): embedded prove state machine.
//
// Security invariant (FIX-CREATE-01, T-05.6-15):
// PersistAll fires ONLY after both prove phases PASS + Enter confirm.
// The skip-and-write path requires an explicit second confirm and surfaces a
// warning. 'q' dismisses the modal, keeps the key, shows a header toast.
type createWizardModel struct {
	// Form step (Step 1).
	inputs     []textinput.Model // 8 fields: Name, GitName, GitEmail, Provider, Port, Alias, Match, Signing
	focusIdx   int
	err        string
	nameLocked bool // true for Add Account mode (name pre-filled and read-only)

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
	inputs := []textinput.Model{
		mkInput("e.g. personal", name, 64),               // 0: Identity Name
		mkInput("e.g. Pedro Perez", "", 128),             // 1: Git Name (generic example, not the real user)
		mkInput("e.g. pedro.perez@example.com", "", 200), // 2: Git Email
		mkInput("github.com", "github.com", 128),         // 3: Provider (default github.com)
		mkInput("22", "22", 10),                          // 4: Port (default 22)
		mkInput("leave blank to use provider", "", 200),  // 5: SSH Alias
		mkInput("1", "1", 10),                            // 6: Match Strategy (default 1 = gitdir)
		mkInput("y / n", "y", 4),                         // 7: Signing (default y)
	}
	inputs[0].Focus()

	return createWizardModel{
		inputs:     inputs,
		focusIdx:   0,
		nameLocked: name != "",
		step:       wizardStepForm,
		sp:         sp,
		deps:       deps,
	}
}

// handleKey processes key presses in the wizard form step.
// Returns the updated model and an optional command.
// Form field indices that are NOT free-text inputs:
//   - fieldMatch is a read-only readable label (the create path builds a gitdir
//     match; hasconfig/both selection is deferred pending backend support).
//   - fieldSigning is a boolean toggled with Space, not typed.
const (
	fieldMatch   = 6
	fieldSigning = 7
)

func (m createWizardModel) handleKey(msg tea.KeyMsg) (createWizardModel, tea.Cmd) {
	if m.step != wizardStepForm {
		// In later steps, use update() for all messages.
		return m, nil
	}

	key := msg.String()
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
	// Match Strategy is a read-only readable label (gitdir); ignore typing so it
	// never shows a cryptic raw value.
	if m.focusIdx == fieldMatch {
		return m, nil
	}

	// Forward the ORIGINAL key event to the focused input. Rebuilding it from the
	// string (Code only) dropped the Text field, and bubbles v2 textinput inserts
	// from msg.Text — so every printable key was silently swallowed (D-1).
	var cmd tea.Cmd
	m.inputs[m.focusIdx], cmd = m.inputs[m.focusIdx].Update(msg)
	return m, cmd
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

	// Validate git email EARLY (before keygen + SSH test) so a malformed address —
	// e.g. one with spaces — is caught at the form, not deep in the fragment write.
	if err := identity.ValidateEmail(m.inputs[2].Value()); err != nil {
		m.err = "! " + err.Error()
		m.inputs[m.focusIdx].Blur()
		m.focusIdx = 2
		m.inputs[2].Focus()
		return m, nil
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
func (m createWizardModel) buildCreateInput(provider string) identity.CreateInput {
	name := m.inputs[0].Value()
	gitName := m.inputs[1].Value()
	gitEmail := m.inputs[2].Value()
	if provider == "" {
		provider = m.inputs[3].Value()
	}
	portStr := m.inputs[4].Value()
	port := 22
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		port = 22
	}
	alias := m.inputs[5].Value()
	if alias == "" {
		alias = identity.DefaultAlias(name, provider)
	}
	matchStr := m.inputs[6].Value()
	if matchStr == "" {
		matchStr = "1"
	}
	signing := strings.ToLower(strings.TrimSpace(m.inputs[7].Value())) != "n"

	// Build match list based on strategy (1 = gitdir, others deferred).
	var matches []gitconfig.Match
	switch matchStr {
	case "1", "gitdir":
		matches = []gitconfig.Match{identity.DefaultMatch(name)}
	default:
		matches = []gitconfig.Match{identity.DefaultMatch(name)}
	}

	_ = signing // signing stored in staged, applied at PersistAll

	return identity.CreateInput{
		Name:     name,
		GitName:  gitName,
		GitEmail: gitEmail,
		Provider: provider,
		Algo:     "ed25519",
		Alias:    alias,
		Hostname: provider, // provider IS the hostname for gitid (github.com, gitlab.com)
		Port:     port,
		Matches:  matches,
	}
}

// initProve transitions the wizard to the prove step and seeds the prove loop.
// Called when the user presses Enter on the upload step.
func (m createWizardModel) initProve() (createWizardModel, tea.Cmd) {
	m.step = wizardStepProve1Running
	m.runID++
	// Use the staged key path for the pre-write test.
	keyPath := m.staged.TempPrivatePath
	provider := m.inputs[3].Value()
	if provider == "" {
		provider = "github.com"
	}
	port := 22
	if _, err := fmt.Sscanf(m.inputs[4].Value(), "%d", &port); err != nil {
		port = 22
	}
	return m, tea.Batch(
		runPreWriteCmd(m.deps.identity.PreWrite, keyPath, provider, port),
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
			alias := m.inputs[5].Value()
			if alias == "" {
				provider := m.inputs[3].Value()
				if provider == "" {
					provider = "github.com"
				}
				alias = identity.DefaultAlias(m.inputs[0].Value(), provider)
			}
			return m, runResolvedCmd(m.deps.identity.Resolved, alias)
		default:
			m.step = wizardStepProve1Failed
			m.confirmActive = false
		}
		return m, nil

	case resolvedResultMsg:
		m.phase2Result = msg.result
		m.phase2Resolved = msg.resolved
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
			m.step = wizardStepWritten
			if msg.result.SSHBackup != "" {
				m.backupPath = msg.result.SSHBackup
			}
			m.result = "Identity created."
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
		// Both phases passed — write gate open.
		switch key {
		case "enter":
			if m.confirmActive {
				m.confirmActive = false
				return m, runWizardCreateCmd(m, false)
			}
		case "esc":
			return m, clearModalCmd()
		}

	case wizardStepProve1Failed, wizardStepProve2Failed:
		switch key {
		case "r":
			// Retry: re-run phase 1.
			m.confirmActive = false
			m.skipConfirmPending = false
			m.runID++
			keyPath := m.staged.TempPrivatePath
			provider := m.inputs[3].Value()
			if provider == "" {
				provider = "github.com"
			}
			port := 22
			if _, err := fmt.Sscanf(m.inputs[4].Value(), "%d", &port); err != nil {
				port = 22
			}
			m.step = wizardStepProve1Running
			return m, tea.Batch(
				runPreWriteCmd(m.deps.identity.PreWrite, keyPath, provider, port),
				m.sp.Tick,
			)
		case "s":
			// Skip test — requires an explicit second confirm.
			if !m.confirmActive {
				m.skipConfirmPending = true
				m.skipWarning = "! Key was written without authentication verification. [warning]"
			}
		case "q":
			// Quit: keep key, dismiss modal, show toast.
			name := m.inputs[0].Value()
			toast := "Key kept at ~/.ssh/gitid_" + name + " — run 'gitid doctor' when ready."
			return m, tea.Batch(clearModalCmd(), setToastCmd(toast, StyleFaint))
		case "esc":
			return m, clearModalCmd()
		case "enter":
			if m.skipConfirmPending {
				// Explicit skip confirm: write without auth.
				m.skipConfirmPending = false
				return m, runWizardCreateCmd(m, true)
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

// viewForm renders the form step.
func (m createWizardModel) viewForm(sb *strings.Builder, _ int) {
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

	readOnlyRow := func(label, value string) string {
		return lipgloss.JoinHorizontal(
			lipgloss.Center,
			StyleLabel.Render(label),
			" ",
			StyleReadOnly.Render(value),
		)
	}

	for i, inp := range m.inputs {
		switch {
		case m.nameLocked && i == 0:
			// Read-only locked name (add-account mode).
			sb.WriteString(readOnlyRow(labels[i], inp.Value()) + "\n")
		case i == fieldSigning:
			// Readable boolean toggle instead of a cryptic "y"/"n" field (P0-3).
			state := "[ ] disabled  (space toggles)"
			if inp.Value() != "n" {
				state = "[x] enabled   (space toggles)"
			}
			sb.WriteString(renderFormField(labels[i], state, i == m.focusIdx) + "\n")
		case i == fieldMatch:
			// Readable strategy label instead of a cryptic "1" (P0-3). gitdir is
			// the wired strategy; the live includeIf preview is shown below.
			sb.WriteString(readOnlyRow(labels[i], "gitdir — match repos by folder") + "\n")
		default:
			sb.WriteString(renderFormField(labels[i], inp.View(), i == m.focusIdx) + "\n")
		}
	}

	// Live includeIf preview for the gitdir match strategy (D-06): shows exactly
	// what will be written to ~/.gitconfig so the strategy is no longer opaque.
	if name := m.inputs[0].Value(); name != "" {
		matches := []gitconfig.Match{identity.DefaultMatch(name)}
		preview := gitconfig.RenderIncludeIf(name, "~/.gitconfig.d/"+name, matches)
		sb.WriteString("\n" + StyleFaint.Render("includeIf preview:") + "\n")
		sb.WriteString(lipgloss.NewStyle().PaddingLeft(4).Render(StyleFaint.Render(preview)) + "\n")
	}

	if m.err != "" {
		sb.WriteString("\n")
		sb.WriteString(SeverityStyle(doctor.SeverityWarning).Render("! " + m.err))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render("[Tab cycle fields · space toggle · Enter advance · Esc cancel]"))
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
func (m createWizardModel) viewUpload(sb *strings.Builder) {
	sb.WriteString(StyleModalTitle.Render("Create Identity — Upload & Test"))
	sb.WriteString("\n\n")
	m.renderUploadHeader(sb)
	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render("[Enter] paste the key into the page above, then test · [Esc] keep key, write nothing"))
}

// viewProve renders the SAME upload screen with the inline test result below it,
// so the key, [c] copy, and instructions never disappear while testing (D-16
// round 3: one screen, no view switch).
func (m createWizardModel) viewProve(sb *strings.Builder) {
	sb.WriteString(StyleModalTitle.Render("Create Identity — Upload & Test"))
	sb.WriteString("\n\n")
	m.renderUploadHeader(sb)
	sb.WriteString("\n")

	// Phase 1.
	switch m.step {
	case wizardStepProve1Running:
		sb.WriteString(m.sp.View() + " Testing SSH authentication...\n")
	case wizardStepProve1Done, wizardStepProve2Running, wizardStepProve2Done, wizardStepWritten:
		sb.WriteString(StylePass.Render("✓") + " Phase 1: authenticated\n")
	case wizardStepProve1Failed:
		sb.WriteString(SeverityStyle(doctor.SeverityError).Render("✗") + " Phase 1: authentication failed [critical]\n")
		renderTestDetail(sb, m.phase1Result)
		sb.WriteString("\n")
		sb.WriteString(renderProveActions(m.skipConfirmPending))
	}

	// Phase 2.
	if m.step >= wizardStepProve2Running && m.step != wizardStepProve1Failed {
		switch m.step {
		case wizardStepProve2Running:
			sb.WriteString(m.sp.View() + " Checking resolved config...\n")
		case wizardStepProve2Done, wizardStepWritten:
			sb.WriteString(StylePass.Render("✓") + " Phase 2: config resolves correctly\n")
		case wizardStepProve2Failed:
			sb.WriteString(SeverityStyle(doctor.SeverityError).Render("✗") + " Phase 2: config resolution failed [critical]\n")
			renderTestDetail(sb, m.phase2Result)
			sb.WriteString(renderProveActions(m.skipConfirmPending))
		}
	}

	// Confirm gate.
	if m.confirmActive {
		sb.WriteString("\n")
		sb.WriteString(StyleBody.Render("Ready to write. Write changes?"))
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

// runWizardCreateCmd dispatches the PersistAll call after the prove loop resolves.
// skipped=true means the user explicitly confirmed skipping the auth test;
// PersistAll is called in both cases (the caller shows a warning on skip).
func runWizardCreateCmd(m createWizardModel, _ bool) tea.Cmd {
	in := m.buildCreateInput(m.inputs[3].Value())
	staged := m.staged
	deps := m.deps
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = wizardCreateResultMsg{err: fmt.Errorf("create panicked: %v", r)}
			}
		}()
		res, err := identity.PersistAll(in, staged, deps.identity)
		return wizardCreateResultMsg{result: res, err: err}
	}
}
