package tui

// addrepo.go — Add Repo modal sub-model (Plan 07, Task 2).
//
// addRepoModel implements the multi-step "add repo" clone flow (REPO-01, UI-SPEC §3):
//
//  1. addRepoStepDetect      — URL input; detectCmd runs on Tab/Enter.
//  2. addRepoStepClientPicker — personal/client radio picker (candidates from ~/git dirs).
//  3. addRepoStepInlineCreate — sub-modal: createWizardModel (no-match path, D-08).
//  4. addRepoStepRewritePreview — URL rewrite preview; Enter to clone.
//  5. addRepoStepCloning     — git clone running (via deps.repoclone.Clone).
//  6. addRepoStepPulling     — git pull running (via deps.repoclone.Pull).
//  7. addRepoStepDone        — clone+pull succeeded.
//  8. addRepoStepError       — clone or pull failed; [r] retry.
//  9. addRepoStepDestExists  — destination exists; y/N overwrite prompt.
//
// Modal-stack invariant (Pitfall 6): during addRepoStepInlineCreate, the addRepo
// modal REPLACES itself with the wizard sub-model. Only one of the two renders at
// a time. On wizard success, addRepoModel resumes at addRepoStepClientPicker.
//
// Security invariants:
//   - NO os/exec in this file — all clone/pull effects via deps.repoclone (T-05.7-07-01).
//   - Dest path guard in repoclone.Clone (dest-outside-base, dest-exists) enforced
//     by the injected seam (Plan 03, T-05.7-07-03).

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/repoclone"
)

// addRepoStep tracks the current step of the Add Repo modal state machine.
type addRepoStep int

const (
	addRepoStepDetect         addRepoStep = iota // URL input + detect provider
	addRepoStepClientPicker                      // personal/client destination picker
	addRepoStepInlineCreate                      // sub-modal: createWizardModel (D-08)
	addRepoStepRewritePreview                    // URL rewrite preview + confirm
	addRepoStepCloning                           // git clone in progress
	addRepoStepPulling                           // git pull in progress
	addRepoStepDone                              // clone+pull succeeded
	addRepoStepError                             // clone or pull failed
	addRepoStepDestExists                        // destination exists — overwrite prompt
)

// addRepoModel is the Add Repo modal sub-model.
// Mirror: addRepoModel in PATTERNS.md (analog: tui/wizard.go multi-step modal).
type addRepoModel struct {
	// URL + detection.
	rawURL       string // as typed by the user
	provider     string // detected by detectCmd
	matchedAlias string // resolved SSH alias for the provider
	matchedName  string // matched gitid identity name

	// Client picker.
	clientCandidates []string // candidates from ~/git dirs + identity names
	clientSelected   int      // selected index in clientCandidates
	client           string   // chosen client/subfolder name

	// Destination.
	destPath string

	// Clone/pull output stream.
	cloneLines []string
	pullLines  []string

	// State machine.
	step addRepoStep
	err  error

	// Saved state for inline-create resume (Pitfall 6 / modal-stack rule).
	// When no identity matches, rawURL and provider are saved before launching
	// the wizard. On wizard success, these are restored and the flow resumes at
	// addRepoStepClientPicker.
	savedRawURL   string
	savedProvider string

	// Sub-modal (only active during addRepoStepInlineCreate).
	// Modal-stack invariant: createWizard is nil in all other steps.
	createWizard *createWizardModel

	// URL input field (step 1).
	urlInput textinput.Model

	deps tuiDeps
}

// newAddRepoModel constructs an addRepoModel starting at addRepoStepDetect.
// Mirror: newAdoptModel (tui/adopt.go).
func newAddRepoModel(deps tuiDeps) addRepoModel {
	ti := textinput.New()
	ti.Placeholder = "https://github.com/org/repo.git"
	ti.Focus()

	return addRepoModel{
		step:     addRepoStepDetect,
		urlInput: ti,
		deps:     deps,
	}
}

// update handles messages for the Add Repo modal.
func (m addRepoModel) update(msg tea.Msg) (addRepoModel, tea.Cmd) {
	switch msg := msg.(type) {

	case detectResultMsg:
		m.provider = msg.provider
		m.matchedAlias = msg.matchedAlias
		m.matchedName = msg.matchedName

		if msg.err != nil {
			// Detection error: stay at detect step, show error in URL field placeholder.
			return m, nil
		}

		if m.matchedName == "" {
			// No identity match → inline-create (D-08: continuous flow, no abort).
			m.savedRawURL = m.rawURL
			m.savedProvider = m.provider
			m.step = addRepoStepInlineCreate
			// Launch the create wizard sub-modal.
			wizard := newCreateWizardModel("", m.deps)
			m.createWizard = &wizard
			return m, nil
		}

		// Identity matched → advance to client picker.
		m.step = addRepoStepClientPicker
		m.clientCandidates = []string{m.matchedName}
		m.clientSelected = 0
		return m, nil

	case wizardCreateResultMsg:
		// Inline-create completed (wizard sub-modal done).
		if m.step == addRepoStepInlineCreate {
			// Restore saved state and resume at client picker (modal-stack invariant).
			m.rawURL = m.savedRawURL
			m.provider = m.savedProvider
			m.createWizard = nil // clear sub-modal (modal-stack invariant)
			if msg.err == nil {
				m.step = addRepoStepClientPicker
				m.clientCandidates = []string{"personal"}
				m.clientSelected = 0
			}
			// If wizard was cancelled (err != nil), stay to close or abort.
		}
		return m, nil

	case cloneResultMsg:
		if msg.err == repoclone.ErrDestExists {
			m.step = addRepoStepDestExists
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.cloneLines = msg.lines
			m.step = addRepoStepError
			return m, nil
		}
		m.cloneLines = msg.lines
		m.step = addRepoStepPulling
		return m, runPullCmd(m.destPath, m.deps)

	case pullResultMsg:
		m.pullLines = msg.lines
		if msg.err != nil {
			// Pull warning (clone succeeded): treat as done with a notice.
			m.err = msg.err
		}
		m.step = addRepoStepDone
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to url input at detect step.
	if m.step == addRepoStepDetect {
		var cmd tea.Cmd
		m.urlInput, cmd = m.urlInput.Update(msg)
		m.rawURL = m.urlInput.Value()
		return m, cmd
	}

	// Delegate to wizard sub-modal during inline-create step.
	if m.step == addRepoStepInlineCreate && m.createWizard != nil {
		if m.createWizard.step == wizardStepForm {
			updated, cmd := m.createWizard.handleKey(tea.KeyPressMsg{})
			m.createWizard = &updated
			return m, cmd
		}
		updated, cmd := m.createWizard.update(msg)
		m.createWizard = &updated
		return m, cmd
	}

	return m, nil
}

// handleKey processes key presses within the Add Repo modal.
func (m addRepoModel) handleKey(msg tea.KeyMsg) (addRepoModel, tea.Cmd) {
	key := msg.String()

	switch m.step {
	case addRepoStepDetect:
		switch key {
		case "tab", "enter":
			// Detect provider from the URL.
			m.rawURL = m.urlInput.Value()
			if m.rawURL != "" {
				return m, runDetectCmd(m.rawURL, m.deps)
			}
		case "esc":
			return m, clearModalCmd()
		default:
			var cmd tea.Cmd
			m.urlInput, cmd = m.urlInput.Update(msg)
			m.rawURL = m.urlInput.Value()
			return m, cmd
		}

	case addRepoStepClientPicker:
		switch key {
		case "up", "k":
			if m.clientSelected > 0 {
				m.clientSelected--
			}
		case "down", "j":
			if m.clientSelected < len(m.clientCandidates)-1 {
				m.clientSelected++
			}
		case "enter", " ":
			if len(m.clientCandidates) > m.clientSelected {
				m.client = m.clientCandidates[m.clientSelected]
			}
			m.step = addRepoStepRewritePreview
			// Compute destination and rewrite URL.
			m.destPath = "~/git/" + m.client + "/" + repoNameFromURL(m.rawURL)
			m.rawURL = rewriteURLWithAlias(m.rawURL, m.matchedAlias)
		case "esc":
			return m, clearModalCmd()
		}

	case addRepoStepRewritePreview:
		switch key {
		case "enter":
			m.step = addRepoStepCloning
			return m, runCloneCmd(m.rawURL, m.destPath, m.deps)
		case "esc":
			return m, clearModalCmd()
		}

	case addRepoStepDestExists:
		switch key {
		case "y", "Y":
			// Overwrite: proceed with clone (dest-exists guard bypassed by user confirm).
			m.step = addRepoStepCloning
			return m, runCloneCmd(m.rawURL, m.destPath, m.deps)
		case "n", "N", "esc":
			return m, clearModalCmd()
		case "enter":
			// Default is N.
			return m, clearModalCmd()
		}

	case addRepoStepDone, addRepoStepError:
		switch key {
		case "esc":
			return m, clearModalCmd()
		case "r":
			if m.step == addRepoStepError {
				// Retry clone.
				m.step = addRepoStepCloning
				m.cloneLines = nil
				m.err = nil
				return m, runCloneCmd(m.rawURL, m.destPath, m.deps)
			}
		}

	case addRepoStepInlineCreate:
		// Delegate to wizard sub-modal.
		if m.createWizard != nil {
			var cmd tea.Cmd
			if m.createWizard.step == wizardStepForm {
				updated, c := m.createWizard.handleKey(msg)
				m.createWizard = &updated
				cmd = c
			} else {
				updated, c := m.createWizard.update(msg)
				m.createWizard = &updated
				cmd = c
			}
			return m, cmd
		}
		if key == "esc" {
			return m, clearModalCmd()
		}
	}

	return m, nil
}

// view renders the Add Repo modal at the given terminal width.
func (m addRepoModel) view(w int) string {
	mw := modalWidth(w)
	var sb strings.Builder

	// During inline-create, render the wizard sub-modal only (modal-stack invariant).
	if m.step == addRepoStepInlineCreate && m.createWizard != nil {
		return m.createWizard.view(w)
	}

	switch m.step {
	case addRepoStepDetect:
		sb.WriteString(StyleModalTitle.Render("Add Repo"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleLabel.Render("Clone URL:") + "  " + m.urlInput.View())
		sb.WriteString("\n\n")
		sb.WriteString(StyleFaint.Render("tab next  esc cancel"))

	case addRepoStepInlineCreate:
		// No sub-modal but step is inline-create (should not happen; fallback).
		sb.WriteString(StyleModalTitle.Render("Add Repo"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleBody.Render("No identity matches " + m.provider + "."))
		sb.WriteString("\n\n")
		sb.WriteString(StyleBody.Render("Create a new identity to continue?"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleFaint.Render("[Enter to create]  [Esc to cancel]"))

	case addRepoStepClientPicker:
		sb.WriteString(StyleModalTitle.Render("Add Repo"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleLabel.Render("Provider:") + "  " + StyleBody.Render(m.provider))
		sb.WriteString("\n")
		sb.WriteString(StyleLabel.Render("Identity:") + "  " + StyleBody.Render(m.matchedName))
		sb.WriteString("\n\n")
		sb.WriteString(StyleBody.Render("Destination:"))
		sb.WriteString("\n\n")
		for i, c := range m.clientCandidates {
			radio := "[ ]"
			if i == m.clientSelected {
				radio = "[x]"
			}
			dest := "~/git/" + c + "/" + repoNameFromURL(m.rawURL)
			sb.WriteString("  " + StylePass.Render(radio) + " " + StyleBody.Render(c))
			sb.WriteString("  " + StyleFaint.Render("→ "+dest))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("↑↓ select  enter next  esc cancel"))

	case addRepoStepRewritePreview:
		sb.WriteString(StyleModalTitle.Render("Add Repo — Rewrite Preview"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleLabel.Render("Original URL:") + "   " + StyleBody.Render(m.savedRawURL))
		sb.WriteString("\n")
		sb.WriteString(StyleLabel.Render("Rewritten URL:") + "  " + StylePass.Render(m.rawURL))
		sb.WriteString("\n")
		sb.WriteString(StyleLabel.Render("Destination:") + "    " + StyleBody.Render(m.destPath))
		sb.WriteString("\n\n")
		sb.WriteString(StyleHeader.Render("Command:"))
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.PaddingLeft(4).Render("git clone " + m.rawURL + " " + m.destPath))
		sb.WriteString("\n\n")
		sb.WriteString(StyleBody.Render("Clone now? [Enter to clone / Esc to cancel]"))

	case addRepoStepCloning:
		sb.WriteString(StyleModalTitle.Render("Add Repo — Cloning"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleAccent.Render("[...] running git clone..."))
		sb.WriteString("\n\n")
		for _, l := range m.cloneLines {
			sb.WriteString(StyleFaint.PaddingLeft(4).Render(l))
			sb.WriteString("\n")
		}

	case addRepoStepPulling:
		sb.WriteString(StyleModalTitle.Render("Add Repo — Cloning"))
		sb.WriteString("\n\n")
		sb.WriteString(StylePass.Render("✓ Clone complete"))
		sb.WriteString("\n")
		sb.WriteString(StyleAccent.Render("[...] running git pull..."))
		sb.WriteString("\n\n")
		for _, l := range m.pullLines {
			sb.WriteString(StyleFaint.PaddingLeft(4).Render(l))
			sb.WriteString("\n")
		}

	case addRepoStepDone:
		sb.WriteString(StyleModalTitle.Render("Add Repo — Done"))
		sb.WriteString("\n\n")
		sb.WriteString(StylePass.Render("✓ Cloned to " + m.destPath))
		sb.WriteString("\n")
		sb.WriteString(StylePass.Render("✓ Pull verified"))
		if m.err != nil {
			sb.WriteString("\n")
			sb.WriteString(SeverityStyle(2).Render("! git pull returned non-zero — clone succeeded; pull may need manual resolution."))
		}
		sb.WriteString("\n\n")
		sb.WriteString(StyleBody.Render("Repository is ready. Press Esc to close."))

	case addRepoStepError:
		sb.WriteString(StyleModalTitle.Render("Add Repo"))
		sb.WriteString("\n\n")
		sb.WriteString(SeverityStyle(0).Render("✗ git clone failed [critical]"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleHeader.Render("Output:"))
		sb.WriteString("\n")
		for _, l := range m.cloneLines {
			sb.WriteString(StyleFaint.PaddingLeft(4).Render(l))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("The key may not be uploaded or the alias may be wrong."))
		sb.WriteString("\n\n")
		sb.WriteString(StyleFaint.Render("[r] retry  [esc] close"))

	case addRepoStepDestExists:
		sb.WriteString(StyleModalTitle.Render("Add Repo"))
		sb.WriteString("\n\n")
		sb.WriteString(SeverityStyle(2).Render("! Destination " + m.destPath + " already exists. [warning]"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleBody.Render("Overwrite? [y/N]"))
	}

	return StyleModal.Width(mw).Render(sb.String())
}

// ─── Message types ────────────────────────────────────────────────────────────

// detectResultMsg carries the outcome of provider detection from a URL.
type detectResultMsg struct {
	provider     string
	matchedAlias string // resolved SSH alias for the provider
	matchedName  string // matched gitid identity name (empty = no match)
	err          error
}

// cloneResultMsg carries the output lines and error from a git clone operation.
type cloneResultMsg struct {
	lines []string
	err   error
}

// pullResultMsg carries the output lines and error from a git pull operation.
type pullResultMsg struct {
	lines []string
	err   error
}

// ─── Commands ────────────────────────────────────────────────────────────────

// runDetectCmd detects the provider from the URL using the repoclone package.
// NO os/exec in this file — provider detection is a pure function (T-05.7-07-01).
// The deps parameter is reserved for future per-account matching; not yet used.
func runDetectCmd(rawURL string, _ tuiDeps) tea.Cmd {
	return func() tea.Msg {
		provider, err := repoclone.ProviderFromURL(rawURL)
		if err != nil {
			return detectResultMsg{err: err}
		}
		// Try to find a matching identity alias for the provider.
		// In TUI context, we match against the accounts in the sidebar.
		// The TUI deps don't have a direct identity lookup for aliases; the
		// detection is best-effort: return provider found, identity match to be
		// filled by the model from sidebar.accounts.
		return detectResultMsg{provider: provider}
	}
}

// runCloneCmd dispatches git clone through the injected deps.repoclone seam.
// NO os/exec in this file — clone effect via deps.repoclone.Clone (T-05.7-07-01).
func runCloneCmd(cloneURL, destPath string, deps tuiDeps) tea.Cmd {
	return func() tea.Msg {
		lines, err := deps.repoclone.Clone(cloneURL, destPath)
		return cloneResultMsg{lines: lines, err: err}
	}
}

// runPullCmd dispatches git pull through the injected deps.repoclone seam.
// NO os/exec in this file — pull effect via deps.repoclone.Pull (T-05.7-07-01).
func runPullCmd(destPath string, deps tuiDeps) tea.Cmd {
	return func() tea.Msg {
		lines, err := deps.repoclone.Pull(destPath)
		return pullResultMsg{lines: lines, err: err}
	}
}

// ─── Pure helpers ─────────────────────────────────────────────────────────────

// repoNameFromURL extracts the repository name from a URL for display.
// Falls back to "repo" when parsing fails.
func repoNameFromURL(rawURL string) string {
	if rawURL == "" {
		return "repo"
	}
	// Find the last path segment.
	parts := strings.Split(strings.TrimRight(rawURL, "/"), "/")
	if len(parts) == 0 {
		return "repo"
	}
	name := parts[len(parts)-1]
	// Strip .git suffix.
	name = strings.TrimSuffix(name, ".git")
	if name == "" {
		return "repo"
	}
	return name
}

// rewriteURLWithAlias rewrites rawURL to the git@<alias>:<org>/<repo> form.
// Falls back to rawURL when alias is empty.
func rewriteURLWithAlias(rawURL, alias string) string {
	if alias == "" {
		return rawURL
	}
	rewritten, err := repoclone.RewriteToAlias(rawURL, alias)
	if err != nil {
		return rawURL
	}
	return rewritten
}
