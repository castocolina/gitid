package dummytui

// identities.go is the Go mirror of
// .planning/design/mockup-src/src/demo/screens/Identities.tsx per
// 02-REDESIGN-SPEC.md §2–3 — live master-detail:
// the left sidebar (tone glyph + name + N⚑ finding flags + S/G capability
// pips + short note, with an inline legend line) re-renders the right
// detail pane IMMEDIATELY on arrow selection — no Enter, no view switch.
// The right pane also hosts every form: the 4-pane-state create wizard
// (SSH form → two-stage test with a demo failure toggle → full Git form →
// 2-state ceremony), edit-SSH (the SAME form with identity fields locked),
// the merged Git form, clone, delete with scope choice + typed destructive
// confirm, and per-finding fix ceremonies. The sidebar never disappears.
//
// Terminal adaptation (100x30): the web renders edit/git forms with their
// ceremony inline below; here Enter opens the ceremony as the pane's next
// state (Esc returns to the form) so both fit 30 rows — same §6 semantics,
// same copy. The step-3 dual previews stack vertically for the same reason.

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// stylePipNone renders the "–" capability pip (dim gray).
var stylePipNone = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

// styleFocusLink renders jump hints like "Edit in Global Git (3)".
var styleFocusLink = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Underline(true)

// identPane mirrors Identities.tsx's PaneMode union (plus the delete-scope
// chooser and the ceremony states of edit/git, which the web renders
// inline below the form).
type identPane int

const (
	paneDetail identPane = iota
	paneCreate
	paneEditSSH
	paneEditCeremony
	paneGit
	paneGitCeremony
	paneClone
	paneDeleteScope
	paneDelete
	paneFix
)

// wizardProviders are the create wizard's provider suggestions.
var wizardProviders = []string{"github.com", "gitlab.com", "bitbucket.org"}

// providerDefaults mirrors Identities.tsx's providerDefaults: github.com
// gets the port-443 alt-SSH endpoint, anything else defaults to itself:22.
func providerDefaults(provider string) (hostname, port string) {
	if provider == "github.com" {
		return "ssh.github.com", "443"
	}
	if provider == "" {
		return "github.com", "22"
	}
	return provider, "22"
}

// pips computes the S/G capability pips (spec §2): tone carries health,
// pips carry capability.
func pips(row DemoIdentity) (s, g string) {
	s = "–"
	if row.State == "key-missing" {
		s = "✗"
	} else if row.SSHHost != "" {
		s = "✓"
	}
	g = "–"
	if row.State == "fragment-path-missing" {
		g = "✗"
	} else if row.GitFragmentPath != "" {
		g = "✓"
	}
	return s, g
}

// pipStyle colors one capability pip: ✓ green · – dim gray · ✗ red.
func pipStyle(pip string) lipgloss.Style {
	switch pip {
	case "✓":
		return styleHealthy
	case "✗":
		return styleError
	default:
		return stylePipNone
	}
}

// newTextInput builds a bare text input (no prompt) with an initial value.
func newTextInput(value string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(value)
	return ti
}

// updateInput routes a key into an input and reports whether the visible
// value changed.
func updateInput(ti textinput.Model, msg tea.KeyMsg) (textinput.Model, bool) {
	before := ti.Value()
	ti, _ = ti.Update(msg)
	return ti, ti.Value() != before
}

// ---------------------------------------------------------------------------
// Shared SSH form (SSHUI-01 field order) — ONE component for both the
// create wizard and edit-SSH; "edit" is just data (lockIdentity), never a
// second copy of the fields (Identities.tsx SshFormFields).
// ---------------------------------------------------------------------------

// SSH form focus slots (wizard adds focus 5 = algorithm).
const (
	sshFieldProvider = iota
	sshFieldPrefix
	sshFieldHost
	sshFieldHostname
	sshFieldPort
)

// editFocusButton is the edit-SSH pane's extra focus slot: the `Rewrite
// Host block…` button after the three editable fields (batch 3 — the web's
// native Tab reaches every button).
const editFocusButton = sshFieldPort + 1

// editFocusRing is the edit-SSH Tab ring size (host, hostname, port,
// button).
const editFocusRing = 4

// sshForm is the shared SSH field set.
type sshForm struct {
	provider textinput.Model
	prefix   textinput.Model
	host     textinput.Model
	hostname textinput.Model
	port     textinput.Model
	// lockIdentity: edit mode — identity name/provider never change in
	// place (rename = clone).
	lockIdentity bool
	// hostTouched: the alias was manually edited — auto-join is off.
	hostTouched bool
	// endpointTouched: hostname/port were manually edited — provider
	// defaults stop applying.
	endpointTouched bool
}

// newSSHForm builds the form with initial values.
func newSSHForm(provider, prefix, host, hostname, port string, lockIdentity bool) sshForm {
	return sshForm{
		provider:     newTextInput(provider),
		prefix:       newTextInput(prefix),
		host:         newTextInput(host),
		hostname:     newTextInput(hostname),
		port:         newTextInput(port),
		lockIdentity: lockIdentity,
	}
}

// autoHost is the auto-joined `<prefix>.<provider>` alias; a blank prefix
// yields the provider host verbatim (WYSIWYG).
func (f sshForm) autoHost() string {
	prefix := strings.TrimSpace(f.prefix.Value())
	if prefix == "" {
		return f.provider.Value()
	}
	return prefix + "." + f.provider.Value()
}

// identityName is the identity name the form values produce.
func (f sshForm) identityName() string {
	prefix := strings.TrimSpace(f.prefix.Value())
	if prefix != "" {
		return prefix
	}
	parts := strings.Split(f.provider.Value(), ".")
	if parts[0] != "" {
		return parts[0]
	}
	return "github"
}

// sshHost is the effective alias (manual override wins over auto-join).
func (f sshForm) sshHost() string {
	if f.hostTouched {
		return f.host.Value()
	}
	return f.autoHost()
}

// portValid reports whether the port is all digits (and non-empty).
func (f sshForm) portValid() bool {
	v := f.port.Value()
	if v == "" {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// handleEdit routes one keystroke into the focused field and applies the
// auto-join / provider-default logic (Identities.tsx CreateWizard).
func (f sshForm) handleEdit(msg tea.KeyMsg, focus int) sshForm {
	switch focus {
	case sshFieldProvider:
		if f.lockIdentity {
			return f
		}
		var changed bool
		f.provider, changed = updateInput(f.provider, msg)
		if changed {
			if !f.endpointTouched {
				hostname, port := providerDefaults(f.provider.Value())
				f.hostname.SetValue(hostname)
				f.port.SetValue(port)
			}
			if !f.hostTouched {
				f.host.SetValue(f.autoHost())
			}
		}
	case sshFieldPrefix:
		if f.lockIdentity {
			return f
		}
		var changed bool
		f.prefix, changed = updateInput(f.prefix, msg)
		if changed && !f.hostTouched {
			f.host.SetValue(f.autoHost())
		}
	case sshFieldHost:
		var changed bool
		f.host, changed = updateInput(f.host, msg)
		if changed {
			f.hostTouched = true
		}
	case sshFieldHostname:
		var changed bool
		f.hostname, changed = updateInput(f.hostname, msg)
		if changed {
			f.endpointTouched = true
		}
	case sshFieldPort:
		// Port accepts digits only.
		if text := msg.Key().Text; text != "" {
			for _, r := range text {
				if r < '0' || r > '9' {
					return f
				}
			}
		}
		var changed bool
		f.port, changed = updateInput(f.port, msg)
		if changed {
			f.endpointTouched = true
		}
	}
	return f
}

// setFocus focuses exactly the input at focus (so it receives keys and
// renders its cursor).
func (f sshForm) setFocus(focus int) sshForm {
	inputs := []*textinput.Model{&f.provider, &f.prefix, &f.host, &f.hostname, &f.port}
	for i, ti := range inputs {
		if i == focus {
			ti.Focus()
		} else {
			ti.Blur()
		}
	}
	return f
}

// formFieldLine renders one field as a SINGLE constant-height row in every
// state (D1, checkpoint-2 contract — supersedes 02-14's renderFocusedFieldBox
// 3-row contour). The marker gutter is 2 cells in every state (`▸ ` accent
// when focused, `  ` otherwise); the label is bold + padRight(label,16) in
// every state; brackets `[…]` are present focused AND blurred (the
// editable-slot affordance never appears/disappears) — focus is signalled
// by color + the marker, never a reflowing box. Locked fields render the
// dim value + faint "(locked)", no brackets.
func formFieldLine(label string, input textinput.Model, focused, locked bool) string {
	name := styleBold.Render(padRight(label, 16))
	switch {
	case focused:
		return " " + styleBold.Render("▸ ") + name + DefaultTheme.FieldFocused.Render("["+input.View()+"]")
	case locked:
		return "   " + name + DefaultTheme.FieldBlurred.Render(input.Value()+"  (locked)")
	default:
		return "   " + name + DefaultTheme.FieldBlurred.Render("["+input.Value()+"]")
	}
}

// helperLine renders a field helper (faint) or error (red) line.
func helperLine(text string, isError bool) string {
	if isError {
		return "     " + styleError.Render(text)
	}
	return "     " + styleFaint.Render(text)
}

// view renders the shared field set. prefixError (if non-empty) replaces
// the prefix helper; hostHelper is the auto-join state helper. Contract
// helpers (locked fields, prefix WYSIWYG/duplicate, auto-join state)
// always render; purely descriptive ones render for the focused field
// only, keeping the pane inside the 30-row frame.
func (f sshForm) view(focus int, prefixError, hostHelper string) string {
	var b strings.Builder
	b.WriteString(formFieldLine("Provider", f.provider, focus == sshFieldProvider, f.lockIdentity) + "\n")
	if f.lockIdentity {
		b.WriteString(helperLine("Locked — the provider comes from the Host alias", false) + "\n")
	} else if focus == sshFieldProvider {
		b.WriteString(helperLine(strings.Join(wizardProviders, " · ")+" — or type any host", false) + "\n")
	}
	b.WriteString(formFieldLine("Alias prefix", f.prefix, focus == sshFieldPrefix, f.lockIdentity) + "\n")
	switch {
	case f.lockIdentity:
		b.WriteString(helperLine("Locked — the identity name never changes in place; use Clone to rename", false) + "\n")
	case prefixError != "":
		b.WriteString(helperLine(prefixError, true) + "\n")
	default:
		b.WriteString(helperLine("Blank prefix → SSH Host = the provider host itself", false) + "\n")
	}
	b.WriteString(formFieldLine("SSH Host (alias)", f.host, focus == sshFieldHost, false) + "\n")
	if hostHelper != "" {
		b.WriteString(helperLine(hostHelper, false) + "\n")
	}
	b.WriteString(formFieldLine("Real hostname", f.hostname, focus == sshFieldHostname, false) + "\n")
	if focus == sshFieldHostname {
		b.WriteString(helperLine("The true SSH endpoint", false) + "\n")
	}
	portLine := formFieldLine("Port", f.port, focus == sshFieldPort, false)
	if !f.portValid() {
		portLine += "  " + styleError.Render("digits only")
	}
	b.WriteString(portLine + "\n")
	// Port had no hint on either side (review-findings F9) — a short,
	// focused-only helper (matching the Hostname field's pattern above)
	// costs no extra row while blurred, and at most +1 while focused, which
	// the 100x30 budget still absorbs (the field-contour/hint-zone work
	// left ~2 rows of headroom at this step).
	if focus == sshFieldPort {
		b.WriteString(helperLine("Default 22; 443 for alt-SSH", false) + "\n")
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Merged Git form (author + signing + match strategy + dual preview) —
// used by wizard state 3 and by "Configure Git" (Identities.tsx
// GitFormFields).
// ---------------------------------------------------------------------------

// Git form focus slots.
const (
	gitFieldName = iota
	gitFieldEmail
	gitFieldStrategy
)

// Wizard Git-step focus ring — the three fields above, then the three REAL
// focusable controls the web renders as buttons (review batch 2, M2:
// Back / Skip Git / Continue; Ctrl+S is gone — it collides with
// XOFF flow control on IXON terminals).
const (
	gitFocusBack = iota + gitFieldStrategy + 1
	gitFocusSkip
	gitFocusContinue
	wizardGitFocusSlots // ring size: 3 fields + 3 buttons
)

// gitPaneFocusButton is the configure-Git pane's extra focus slot: the
// `Write it…` button after the three fields (batch 3 — Tab reaches every
// button); its ring size is gitPaneFocusRing.
const (
	gitPaneFocusButton = gitFieldStrategy + 1
	gitPaneFocusRing   = 4
)

// matchStrategies are the includeIf strategies in select order.
var matchStrategies = []string{"gitdir", "hasconfig", "both"}

// strategyCopy renders one match-strategy option with the exact web copy.
func strategyCopy(strategy, name string) string {
	switch strategy {
	case "gitdir":
		return "gitdir (default) — applies inside ~/" + name + "/"
	case "hasconfig":
		return "hasconfig — repos whose remote uses this alias"
	default:
		return "both — either condition (two includeIf blocks = OR)"
	}
}

// gitForm is the merged Git identity form.
type gitForm struct {
	name        textinput.Model
	email       textinput.Model
	strategyIdx int
}

// newGitForm builds the form with initial values.
func newGitForm(name, email, strategy string) gitForm {
	idx := 0
	for i, s := range matchStrategies {
		if s == strategy {
			idx = i
		}
	}
	return gitForm{name: newTextInput(name), email: newTextInput(email), strategyIdx: idx}
}

// strategy is the selected match strategy.
func (g gitForm) strategy() string { return matchStrategies[g.strategyIdx] }

// valid mirrors the web gating: name non-empty + email has @.
func (g gitForm) valid() bool {
	return strings.TrimSpace(g.name.Value()) != "" && strings.Contains(g.email.Value(), "@")
}

// setFocus focuses exactly the input at focus.
func (g gitForm) setFocus(focus int) gitForm {
	if focus == gitFieldName {
		g.name.Focus()
	} else {
		g.name.Blur()
	}
	if focus == gitFieldEmail {
		g.email.Focus()
	} else {
		g.email.Blur()
	}
	return g
}

// handleEdit routes one keystroke into the focused field; ←/→ cycle the
// strategy when it is focused.
func (g gitForm) handleEdit(msg tea.KeyMsg, focus int) gitForm {
	key := msg.String()
	switch focus {
	case gitFieldName:
		g.name, _ = updateInput(g.name, msg)
	case gitFieldEmail:
		g.email, _ = updateInput(g.email, msg)
	case gitFieldStrategy:
		if key == "left" {
			g.strategyIdx = (g.strategyIdx + len(matchStrategies) - 1) % len(matchStrategies)
		}
		if key == "right" {
			g.strategyIdx = (g.strategyIdx + 1) % len(matchStrategies)
		}
	}
	return g
}

// fragmentPreview is the per-identity fragment file content preview.
func (g gitForm) fragmentPreview(keyPath string) string {
	return "[user]\n    name = " + g.name.Value() + "\n    email = " + g.email.Value() +
		"\n    signingkey = " + keyPath + ".pub\n\n[gpg]\n    format = ssh\n\n[commit]\n    gpgsign = true"
}

// includeIfPreview is the ~/.gitconfig includeIf block preview for the
// selected strategy, aliased to name.
func (g gitForm) includeIfPreview(name string) string {
	return strings.ReplaceAll(GitScreenMatchStrategyPreview[g.strategy()], "personal", name)
}

// view renders the merged Git form with the dual dim previews (stacked —
// terminal-width adaptation of the web's side-by-side pair).
func (g gitForm) view(name, keyPath string, focus int, width int, baseline string) string {
	var b strings.Builder
	b.WriteString(formFieldLine("user.name", g.name, focus == gitFieldName, false) + "\n")
	b.WriteString(formFieldLine("user.email", g.email, focus == gitFieldEmail, false))
	if !strings.Contains(g.email.Value(), "@") {
		b.WriteString("  " + styleError.Render("needs @"))
	}
	b.WriteString("\n")
	b.WriteString(helperLine("Kept byte-identical to ~/.ssh/allowed_signers (GITUI-04)", false) + "\n")
	// Row-budget trap (02-STYLE-SPEC.md §7): the separate "Signing: ..."
	// line was dropped (its signingkey-is-a-path fact is already visible in
	// the fragment preview block below) to make room for the field-contour
	// box and the frozen hint copy the button row now always carries.

	// D2 (checkpoint-2 contract): the (←/→ change) hint moves onto the
	// header line, visible in BOTH focus states (it used to show only
	// while blurred — backwards).
	marker := "  "
	if focus == gitFieldStrategy {
		marker = styleBold.Render("▸ ")
	}
	b.WriteString(" " + marker + styleBold.Render("Match strategy — when does this Git identity apply?") +
		" " + styleFaint.Render("(←/→ change)") + "\n")
	// STABLE HINT ZONE (02-STYLE-SPEC.md hint-persistence): this reserved
	// hint row is ALWAYS drawn — expanding the select below PUSHES the
	// option rows down, it never replaces this line (the "hint vanishes on
	// focus" report this fixes).
	b.WriteString(helperLine("Determines which repos this Git identity applies to.", false) + "\n")
	// D2: ALL options render ALWAYS — no expand/collapse branch. The
	// selected ● + label render through FieldFocused (accent) when the
	// group is focused, plain-bold when blurred.
	for i, s := range matchStrategies {
		dot := glyphRadioOff
		text := strategyCopy(s, name)
		if i == g.strategyIdx {
			dot = glyphRadioOn
			if focus == gitFieldStrategy {
				text = DefaultTheme.FieldFocused.Render(text)
			} else {
				text = styleBold.Render(text)
			}
		}
		b.WriteString("     " + dot + " " + text + "\n")
	}

	// fragLines/includeIf maxLines stay at the tightest previously-used
	// value (1) regardless of focus — the row-budget trap forced this
	// tradeoff to make room for the field-contour box and the frozen hint
	// lines the button row now always carries. Routed through the bounded,
	// titled PreviewBlock (review-findings F1) — the title is spliced into
	// the border's top edge instead of a separate PreviewLabel row, saving
	// one row per preview.
	b.WriteString(PreviewBlock("~/.gitconfig.d/"+name+" (fragment file — preview)", g.fragmentPreview(keyPath), false, width, 1) + "\n")
	b.WriteString(PreviewBlock("~/.gitconfig (includeIf block — preview)", g.includeIfPreview(name), false, width, 1) + "\n")
	b.WriteString(baseline + "\n")
	return b.String()
}

// ---------------------------------------------------------------------------
// Create wizard — 4 pane-states in the detail pane (spec §3).
// ---------------------------------------------------------------------------

// wizardSteps are the LONG step labels — the breadcrumb/help source AND
// (D5, checkpoint-2 contract) the renderStepper segment source. The
// bracketed short-segment stepper 02-14 shipped (bracket-number + short
// word per step, joined by middots) is SUPERSEDED: D5 reverts the stepper
// to `Step n/4 · <label> ● ○ ○ ○` using these long labels, and the bracket
// format moves onto the MAIN NAV instead (D4).
var wizardSteps = []string{"SSH details", "Test connection", "Git identity", "Review & write"}

// Test phases (wizard state 2).
const (
	testIdle     = "idle"
	testRunning1 = "running1"
	testStage1   = "stage1"
	testRunning2 = "running2"
	testStage2   = "stage2"
	testFailed   = "failed"
)

// wizardStageMsg completes a "running ssh…" stage after its tick.
type wizardStageMsg struct{ stage int }

// runStageCmd schedules a stage completion (the brief running state).
func runStageCmd(stage int) tea.Cmd {
	return tea.Tick(350*time.Millisecond, func(time.Time) tea.Msg {
		return wizardStageMsg{stage: stage}
	})
}

// wizardModel is the 4-pane-state create wizard.
type wizardModel struct {
	step         int
	form         sshForm
	focus        int // step 0: 0..4 form fields, 5 = algorithm select
	algoIdx      int
	testPhase    string
	simulateFail bool
	configureGit bool
	git          gitForm
	gitFocus     int
	ceremony     ceremonyModel
}

// newWizard builds the wizard with the web demo's defaults.
func newWizard() wizardModel {
	form := newSSHForm("github.com", "acme", "acme.github.com", "ssh.github.com", "443", false)
	form = form.setFocus(sshFieldPrefix) // web: Alias prefix autoFocus
	return wizardModel{
		form:      form,
		focus:     sshFieldPrefix,
		testPhase: testIdle,
		git:       newGitForm("Acme Identity", "you@acme.example", GitScreenMatchStrategyDefault).setFocus(gitFieldName),
	}
}

// keyPath is the per-identity ed25519 key the wizard will "generate".
func (w wizardModel) keyPath() string {
	return "~/.ssh/id_ed25519_" + w.form.identityName()
}

// algo is the selected key algorithm id.
func (w wizardModel) algo() string { return AlgorithmCatalog[w.algoIdx].ID }

// algoDisabled reports whether a catalog entry is unavailable on this
// machine (the demo simulates: no FIDO2 key plugged in).
func algoDisabled(entry AlgorithmCatalogEntry) bool {
	return strings.HasPrefix(entry.MacOS, "Needs libfido2")
}

// hostBlockText renders the managed Host block for the given values — the
// ONE source of the block shape, shared by the wizard preview/ceremony and
// the edit-SSH preview/ceremony (M1: reuse, never duplicate).
func hostBlockText(host, hostname, port, keyPath string) string {
	return "Host " + host + "\n    Hostname " + hostname +
		"\n    Port " + port + "\n    User git\n    IdentityFile " + keyPath +
		"\n    IdentitiesOnly yes"
}

// renderHostBlockPreview renders the live Host-block preview through the
// bounded, titled PreviewBlock — the title is spliced into the border's top
// edge instead of a separate label row (02-STYLE-SPEC.md "preview-sizing";
// review-findings F1: PreviewBlock was dead code, every wizard preview still
// used the untitled previewBlockClipped + a separate PreviewLabel row; this
// SAVES one row per preview). The title is shortened from the original
// PreviewLabel wording ("Live Host-block preview — written exactly like
// this on confirm") to fit the border budget (detailWidth=62 leaves 58
// columns for the title once the corner/fill chars are reserved). Rebuilt
// on every keystroke — the SAME rendering under the wizard's SSH form and
// the edit-SSH form (review batch 2, M1; the web shows it simultaneously in
// both places).
func renderHostBlockPreview(host, hostname, port, keyPath string, width int) string {
	return PreviewBlock("Live Host-block preview (written on confirm)",
		hostBlockText(host, hostname, port, keyPath), false, width, 6)
}

// hostBlockPreview is the wizard's live Host-block preview text — written
// exactly like this on confirm.
func (w wizardModel) hostBlockPreview() string {
	return hostBlockText(w.form.sshHost(), w.form.hostname.Value(), w.form.port.Value(), w.keyPath())
}

// stage1Cmd is the stage-1 direct test command (TEST-01) with the
// CONSISTENT flag order pinned by data.go's CreateFlowTestStage1Command.
func (w wizardModel) stage1Cmd() string {
	return "ssh -T -F " + CreateFlowTestTmpConfig + " -p " + w.form.port.Value() +
		" -i " + w.keyPath() + " git@" + w.form.hostname.Value()
}

// stage2Cmd is the stage-2 by-alias test (TEST-02) — no -i BY DESIGN.
func (w wizardModel) stage2Cmd() string {
	return "ssh -G -F " + CreateFlowTestTmpConfig + " " + w.form.sshHost() + " | grep identityfile"
}

// step0Valid mirrors the web gating for wizard state 1.
func (w wizardModel) step0Valid(s DemoState) bool {
	return !w.nameTaken(s) && strings.TrimSpace(w.form.hostname.Value()) != "" &&
		w.form.portValid() && strings.TrimSpace(w.form.sshHost()) != ""
}

// nameTaken reports whether the produced identity name already exists.
func (w wizardModel) nameTaken(s DemoState) bool {
	return hasIdentityNamed(s, w.form.identityName())
}

// stepBack (D7, checkpoint-2 contract) uniformly decrements the wizard step
// — the SAME behavior at every step, INCLUDING 3→2 (back from the
// previously-DEAD review ceremony to the Git step). Step 0 has no lower
// step; its caller (the hoisted Shift gate) handles leaving the wizard
// entirely before calling this.
func (w wizardModel) stepBack() wizardModel {
	if w.step > 0 {
		w.step--
	}
	return w
}

// stepForward (D7, checkpoint-2 contract) advances the wizard step ONLY
// when the current step's own validity gate passes — sharing the EXACT
// same predicate plain Right/Enter already use, so Shift is a
// FOCUS-OVERRIDE only, never a validity override. Reports whether it
// advanced.
func (w wizardModel) stepForward(s DemoState) (wizardModel, bool) {
	switch w.step {
	case 0:
		if w.step0Valid(s) {
			w.step = 1
			w.testPhase = testIdle
			return w, true
		}
	case 1:
		if w.testPhase == testStage2 {
			w.step = 2
			w.gitFocus = gitFieldName
			w.git = w.git.setFocus(w.gitFocus)
			return w, true
		}
	case 2:
		if w.git.valid() {
			w.configureGit = true
			w.step = 3
			w.ceremony = w.reviewCeremony()
			return w, true
		}
	}
	return w, false
}

// blockedForwardNote is the frozen status note (D7) emitted when
// Shift+→ is blocked at a step — naming the gate, never a validity
// override. Step 1 (the test step) has no frozen note: its own stage
// output already explains what is missing.
func blockedForwardNote(step int) string {
	switch step {
	case 0:
		return "Can't continue yet — check the alias prefix, hostname, and port."
	case 2:
		return "Can't continue yet — add user.name and a valid email."
	default:
		return ""
	}
}

func hasIdentityNamed(s DemoState, name string) bool {
	for _, row := range s.Identities {
		if row.Name == name {
			return true
		}
	}
	return false
}

// reviewCeremony builds the wizard's state-4 ceremony (state A: combined
// review preview + backup promises).
func (w wizardModel) reviewCeremony() ceremonyModel {
	name := w.form.identityName()
	begin, end := ManagedBlockSentinels(name)
	managedBlock := begin + "\n" + w.hostBlockPreview() + "\n" + end
	review := managedBlock
	targets := []string{"~/.ssh/config"}
	backups := []string{NewBackupPath("~/.ssh/config")}
	summary := "SSH: " + w.form.sshHost() + " → " + w.form.hostname.Value() + ":" + w.form.port.Value() +
		" · key " + w.keyPath() + " · Git: skipped"
	if w.configureGit {
		review = managedBlock + "\n\n# ~/.gitconfig.d/" + name + "\n" + w.git.fragmentPreview(w.keyPath()) +
			"\n\n# ~/.gitconfig\n" + w.git.includeIfPreview(name)
		targets = []string{"~/.ssh/config", "~/.gitconfig.d/" + name, "~/.gitconfig", "~/.ssh/allowed_signers"}
		backups = []string{NewBackupPath("~/.ssh/config"), NewBackupPath("~/.gitconfig")}
		summary = "SSH: " + w.form.sshHost() + " → " + w.form.hostname.Value() + ":" + w.form.port.Value() +
			" · key " + w.keyPath() + " · Git: " + w.git.name.Value() + " <" + w.git.email.Value() + ">, strategy " + w.git.strategy()
	}
	return newCeremony(ceremonyConfig{
		Heading:       `Create identity "` + name + `" — ` + w.algo() + ", test passed ✓",
		Targets:       targets,
		Backups:       backups,
		Preview:       summary + "\n" + review,
		ResultMessage: `Identity "` + name + `" created — ` + w.form.sshHost() + " now resolves to " + w.keyPath() + ".",
		ConfirmLabel:  "Write it",
	})
}

// finishIdentity builds the DemoIdentity the wizard's Done dispatches
// (Identities.tsx finish()).
func (w wizardModel) finishIdentity() DemoIdentity {
	name := w.form.identityName()
	id := DemoIdentity{
		Name:     name,
		SSHHost:  w.form.sshHost(),
		KeyPath:  w.keyPath(),
		Hostname: w.form.hostname.Value(),
		Port:     atoiSafe(w.form.port.Value()),
	}
	if w.configureGit {
		id.State = "complete"
		id.GitFragmentPath = "~/.gitconfig.d/" + name
		id.GitName = w.git.name.Value()
		id.GitEmail = w.git.email.Value()
		id.MatchStrategy = w.git.strategy()
		id.Note = "SSH Host block and Git fragment both present."
	} else {
		id.State = "incomplete"
		id.Note = "SSH Host block present; no Git identity configured for this alias."
	}
	return id
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// ---------------------------------------------------------------------------
// The Identities tab child model.
// ---------------------------------------------------------------------------

// identitiesModel is the Identities tab: live master-detail plus every
// in-pane form/ceremony state.
type identitiesModel struct {
	selected string
	pane     identPane

	wizard wizardModel

	editForm     sshForm
	editFocus    int
	editCeremony ceremonyModel

	gitPaneForm gitForm
	gitFocus    int
	gitCeremony ceremonyModel
	gitExisting bool
	deleteScope string
	deleteCerem ceremonyModel
	cloneInput  textinput.Model
	// cloneOnButton: the clone pane's 2-slot focus ring sits on the Clone
	// button instead of the name input (batch 3 focus-ring parity).
	cloneOnButton bool
	fixFindingID  string
	fixCeremony   ceremonyModel
}

// newIdentitiesModel starts on the first seeded row's detail.
func newIdentitiesModel() identitiesModel {
	return identitiesModel{selected: IdentityManagerRows[0].Name, pane: paneDetail, deleteScope: "git-only"}
}

// activate implements screenModel (no entry hook needed here).
func (m identitiesModel) activate(DemoState) (screenModel, tea.Cmd) { return m, nil }

// selectedIdentity resolves the selected row (falls back to the first).
func (m identitiesModel) selectedIdentity(s DemoState) (DemoIdentity, bool) {
	for _, row := range s.Identities {
		if row.Name == m.selected {
			return row, true
		}
	}
	if len(s.Identities) > 0 {
		return s.Identities[0], true
	}
	return DemoIdentity{}, false
}

// selectedIndex is the selected row's index (or 0).
func (m identitiesModel) selectedIndex(s DemoState) int {
	for i, row := range s.Identities {
		if row.Name == m.selected {
			return i
		}
	}
	return 0
}

// firstFixableFinding returns the selected identity's first finding that
// carries a suggested fix.
func firstFixableFinding(s DemoState, name string) (DemoFinding, bool) {
	for _, f := range FindingsFor(s, name) {
		if f.SuggestedFix != "" {
			return f, true
		}
	}
	return DemoFinding{}, false
}

// handleMsg completes wizard test stages after their running tick.
func (m identitiesModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
	if stage, ok := msg.(wizardStageMsg); ok && m.pane == paneCreate {
		switch {
		case stage.stage == 1 && m.wizard.testPhase == testRunning1:
			if m.wizard.simulateFail {
				m.wizard.testPhase = testFailed
			} else {
				m.wizard.testPhase = testStage1
			}
		case stage.stage == 2 && m.wizard.testPhase == testRunning2:
			m.wizard.testPhase = testStage2
		}
	}
	return keyResult{model: m}
}

// handleKey implements the whole Identities key model. Non-detail panes
// consume every key (forms own their keys); detail mode leaves unknown
// keys to the app globals.
func (m identitiesModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	switch m.pane {
	case paneDetail:
		return m.handleDetailKey(msg, s)
	case paneCreate:
		return m.handleWizardKey(msg, s)
	case paneEditSSH, paneEditCeremony:
		return m.handleEditKey(msg, s)
	case paneGit, paneGitCeremony:
		return m.handleGitKey(msg, s)
	case paneClone:
		return m.handleCloneKey(msg, s)
	case paneDeleteScope, paneDelete:
		return m.handleDeleteKey(msg, s)
	case paneFix:
		return m.handleFixKey(msg, s)
	}
	return keyResult{model: m}
}

// handleDetailKey: arrows move the selection (the detail re-renders
// immediately); n/e/g/c/d/f open the panes.
func (m identitiesModel) handleDetailKey(msg tea.KeyMsg, s DemoState) keyResult {
	sel, ok := m.selectedIdentity(s)
	switch msg.String() {
	case "down":
		idx := m.selectedIndex(s)
		if idx < len(s.Identities)-1 {
			m.selected = s.Identities[idx+1].Name
		}
		return keyResult{model: m, handled: true}
	case "up":
		idx := m.selectedIndex(s)
		if idx > 0 {
			m.selected = s.Identities[idx-1].Name
		}
		return keyResult{model: m, handled: true}
	case "n":
		m.pane = paneCreate
		m.wizard = newWizard()
		return keyResult{model: m, handled: true}
	case "e":
		if !ok {
			return keyResult{model: m, handled: true}
		}
		m = m.openEditSSH(sel)
		return keyResult{model: m, handled: true}
	case "g":
		if !ok {
			return keyResult{model: m, handled: true}
		}
		m = m.openGitForm(sel)
		return keyResult{model: m, handled: true}
	case "c":
		if !ok {
			return keyResult{model: m, handled: true}
		}
		m.pane = paneClone
		m.cloneInput = newTextInput(sel.Name + "-clone")
		m.cloneInput.Focus()
		m.cloneOnButton = false
		return keyResult{model: m, handled: true}
	case "d":
		if !ok {
			return keyResult{model: m, handled: true}
		}
		m.pane = paneDeleteScope
		m.deleteScope = "git-only" // safer scope default-focused
		return keyResult{model: m, handled: true}
	case "f":
		if !ok {
			return keyResult{model: m, handled: true}
		}
		if finding, found := firstFixableFinding(s, sel.Name); found {
			m.pane = paneFix
			m.fixFindingID = finding.ID
			m.fixCeremony = fixCeremonyFor(finding)
		}
		return keyResult{model: m, handled: true}
	}
	return keyResult{model: m}
}

// fixCeremonyFor builds the compressed per-finding fix ceremony from its
// fix plan.
func fixCeremonyFor(finding DemoFinding) ceremonyModel {
	plan := PlanFor(finding)
	return newCeremony(ceremonyConfig{
		Heading:       "Fix: " + finding.Title,
		Targets:       []string{plan.File},
		Backups:       []string{NewBackupPath(plan.File)},
		Preview:       plan.Diff,
		PreviewDiff:   true,
		Destructive:   plan.Destructive,
		ResultMessage: plan.Result,
		ConfirmLabel:  "Apply fix",
	})
}

// openEditSSH mirrors Identities.tsx openEditSsh: the SAME form with
// lockIdentity=true, prefilled from the row.
func (m identitiesModel) openEditSSH(sel DemoIdentity) identitiesModel {
	sshHost := sel.SSHHost
	if sshHost == "" {
		sshHost = sel.Name + ".github.com"
	}
	parts := strings.Split(sshHost, ".")
	provider := "github.com"
	if len(parts) >= 2 {
		provider = strings.Join(parts[len(parts)-2:], ".")
	}
	hostname := sel.Hostname
	if hostname == "" {
		hostname = "ssh.github.com"
	}
	port := sel.Port
	if port == 0 {
		port = 443
	}
	m.editForm = newSSHForm(provider, sel.Name, sshHost, hostname, strconv.Itoa(port), true)
	m.editFocus = sshFieldHost
	m.editForm = m.editForm.setFocus(m.editFocus)
	m.pane = paneEditSSH
	return m
}

// editCeremonyFor builds the edit-SSH rewrite ceremony from the current
// form values.
func (m identitiesModel) editCeremonyFor(sel DemoIdentity) ceremonyModel {
	keyPath := sel.KeyPath
	if keyPath == "" {
		keyPath = "~/.ssh/id_ed25519_" + sel.Name
	}
	preview := hostBlockText(m.editForm.host.Value(), m.editForm.hostname.Value(), m.editForm.port.Value(), keyPath)
	return newCeremony(ceremonyConfig{
		Heading:       `Rewrite the managed Host block for "` + sel.Name + `"`,
		Targets:       []string{"~/.ssh/config"},
		Backups:       []string{NewBackupPath("~/.ssh/config")},
		Preview:       preview,
		ResultMessage: `Host block for "` + sel.Name + `" rewritten.`,
		ConfirmLabel:  "Save changes",
	})
}

// handleEditKey drives the edit-SSH form and its ceremony state.
func (m identitiesModel) handleEditKey(msg tea.KeyMsg, s DemoState) keyResult {
	sel, _ := m.selectedIdentity(s)
	key := msg.String()

	if m.pane == paneEditCeremony {
		var outcome ceremonyOutcome
		m.editCeremony, outcome = m.editCeremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.pane = paneEditSSH
		case ceremonyFinished:
			m.pane = paneDetail
			return keyResult{model: m, handled: true, note: `SSH settings of "` + sel.Name + `" updated.`, actions: []Action{EditSSH{
				Name:     sel.Name,
				SSHHost:  m.editForm.host.Value(),
				Hostname: m.editForm.hostname.Value(),
				Port:     atoiSafe(m.editForm.port.Value()),
				Backup:   NewBackupPath("~/.ssh/config"),
			}}}
		case ceremonyNone, ceremonyConfirmed:
		}
		return keyResult{model: m, handled: true}
	}

	switch key {
	case "esc":
		m.pane = paneDetail
		return keyResult{model: m, handled: true}
	case "enter":
		// Enter on a field falls through to the primary action (web
		// parity); Enter on the focused Rewrite button activates it — the
		// same ceremony either way.
		m.editCeremony = m.editCeremonyFor(sel)
		m.pane = paneEditCeremony
		return keyResult{model: m, handled: true}
	case "tab", "down":
		m.editFocus = sshFieldHost + (m.editFocus-sshFieldHost+1)%editFocusRing
		m.editForm = m.editForm.setFocus(m.editFocus)
		return keyResult{model: m, handled: true}
	case "shift+tab", "up":
		m.editFocus = sshFieldHost + (m.editFocus-sshFieldHost+editFocusRing-1)%editFocusRing
		m.editForm = m.editForm.setFocus(m.editFocus)
		return keyResult{model: m, handled: true}
	default:
		// ←/→ on the single Rewrite button have no adjacent button to move
		// to; fields keep them for the input cursor via handleEdit.
		m.editForm = m.editForm.handleEdit(msg, m.editFocus)
		return keyResult{model: m, handled: true}
	}
}

// openGitForm mirrors Identities.tsx openGitForm (defaults when the row
// has no Git side yet).
func (m identitiesModel) openGitForm(sel DemoIdentity) identitiesModel {
	name := sel.GitName
	if name == "" {
		name = sel.Name + " identity"
	}
	email := sel.GitEmail
	if email == "" {
		email = "you@" + sel.Name + ".example"
	}
	strategy := sel.MatchStrategy
	if strategy == "" {
		strategy = GitScreenMatchStrategyDefault
	}
	m.gitPaneForm = newGitForm(name, email, strategy)
	m.gitFocus = gitFieldName
	m.gitPaneForm = m.gitPaneForm.setFocus(m.gitFocus)
	m.gitExisting = sel.GitFragmentPath != ""
	m.pane = paneGit
	return m
}

// gitCeremonyFor builds the configure-Git write ceremony.
func (m identitiesModel) gitCeremonyFor(sel DemoIdentity) ceremonyModel {
	return newCeremony(ceremonyConfig{
		Heading: `Write Git identity for "` + sel.Name + `"`,
		Targets: []string{"~/.gitconfig.d/" + sel.Name, "~/.gitconfig", "~/.ssh/allowed_signers"},
		Backups: []string{NewBackupPath("~/.gitconfig"), NewBackupPath("~/.ssh/allowed_signers")},
		Preview: m.gitPaneForm.includeIfPreview(sel.Name),
		ResultMessage: `Git identity "` + sel.Name + `" configured — applies via the ` +
			m.gitPaneForm.strategy() + ` strategy.`,
		ConfirmLabel: "Write it",
	})
}

// handleGitKey drives the configure-Git form and its ceremony state.
func (m identitiesModel) handleGitKey(msg tea.KeyMsg, s DemoState) keyResult {
	sel, _ := m.selectedIdentity(s)
	key := msg.String()

	if m.pane == paneGitCeremony {
		var outcome ceremonyOutcome
		m.gitCeremony, outcome = m.gitCeremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.pane = paneGit
		case ceremonyFinished:
			m.pane = paneDetail
			return keyResult{model: m, handled: true, note: `Git identity "` + sel.Name + `" configured.`, actions: []Action{ConfigureGit{
				Name:          sel.Name,
				GitName:       m.gitPaneForm.name.Value(),
				GitEmail:      m.gitPaneForm.email.Value(),
				MatchStrategy: m.gitPaneForm.strategy(),
				Backup:        NewBackupPath("~/.gitconfig"),
			}}}
		case ceremonyNone, ceremonyConfirmed:
		}
		return keyResult{model: m, handled: true}
	}

	switch key {
	case "esc":
		m.pane = paneDetail
		return keyResult{model: m, handled: true}
	case "enter":
		// Enter on a field falls through to the primary action; Enter on
		// the focused Write-it button activates the same path.
		if m.gitPaneForm.valid() {
			m.gitCeremony = m.gitCeremonyFor(sel)
			m.pane = paneGitCeremony
		}
		return keyResult{model: m, handled: true}
	case "tab", "down":
		m.gitFocus = (m.gitFocus + 1) % gitPaneFocusRing
		m.gitPaneForm = m.gitPaneForm.setFocus(m.gitFocus)
		return keyResult{model: m, handled: true}
	case "shift+tab", "up":
		m.gitFocus = (m.gitFocus + gitPaneFocusRing - 1) % gitPaneFocusRing
		m.gitPaneForm = m.gitPaneForm.setFocus(m.gitFocus)
		return keyResult{model: m, handled: true}
	default:
		m.gitPaneForm = m.gitPaneForm.handleEdit(msg, m.gitFocus)
		return keyResult{model: m, handled: true}
	}
}

// handleCloneKey drives the clone pane (no ceremony, matching the web).
// The 2-slot focus ring is name input ↔ Clone button; Enter on the input
// keeps its primary-action fall-through (clone), Enter on the button
// activates the same path.
func (m identitiesModel) handleCloneKey(msg tea.KeyMsg, s DemoState) keyResult {
	sel, _ := m.selectedIdentity(s)
	name := m.cloneInput.Value()
	switch msg.String() {
	case "esc":
		m.pane = paneDetail
		return keyResult{model: m, handled: true}
	case "tab", "shift+tab":
		m.cloneOnButton = !m.cloneOnButton
		if m.cloneOnButton {
			m.cloneInput.Blur()
		} else {
			m.cloneInput.Focus()
		}
		return keyResult{model: m, handled: true}
	case "enter":
		if strings.TrimSpace(name) == "" || hasIdentityNamed(s, name) {
			return keyResult{model: m, handled: true}
		}
		m.pane = paneDetail
		m.selected = name
		return keyResult{model: m, handled: true,
			note:    `Identity "` + name + `" cloned from "` + sel.Name + `".`,
			actions: []Action{CloneIdentity{Source: sel.Name, CloneName: name}}}
	default:
		// ←/→ on the single Clone button have no adjacent button; typing
		// only reaches the name input while it is the focused slot.
		if !m.cloneOnButton {
			m.cloneInput, _ = updateInput(m.cloneInput, msg)
		}
		return keyResult{model: m, handled: true}
	}
}

// deleteCeremonyFor builds the delete ceremony for the chosen scope —
// everything is destructive (typed identity name).
func deleteCeremonyFor(sel DemoIdentity, scope string) ceremonyModel {
	fragment := sel.GitFragmentPath
	if fragment == "" {
		fragment = "~/.gitconfig.d/" + sel.Name
	}
	keyPath := sel.KeyPath
	if keyPath == "" {
		keyPath = "~/.ssh/id_ed25519_" + sel.Name
	}
	sshHost := sel.SSHHost
	if sshHost == "" {
		sshHost = sel.Name + ".github.com"
	}
	if scope == "everything" {
		return newCeremony(ceremonyConfig{
			Heading: `Delete EVERYTHING for "` + sel.Name + `" (SSH + Git + key)`,
			Targets: []string{"~/.ssh/config", "~/.gitconfig", fragment, keyPath},
			Backups: []string{NewBackupPath("~/.ssh/config"), NewBackupPath("~/.gitconfig")},
			Preview: "- Host " + sshHost + " (managed block removed)\n- [includeIf] → " + fragment +
				" (removed)\n- " + keyPath + " (key file removed)",
			PreviewDiff: true,
			Destructive: &FixDestructive{
				ConfirmWord: sel.Name,
				Warning: `This removes the key file too — it cannot be regenerated. Type the identity name "` +
					sel.Name + `" to confirm.`,
			},
			ResultMessage: `Identity "` + sel.Name + `" deleted — SSH block, Git fragment, and key removed (backups kept).`,
			ConfirmLabel:  "Delete",
		})
	}
	return newCeremony(ceremonyConfig{
		Heading: `Delete the Git identity of "` + sel.Name + `" (SSH stays)`,
		Targets: []string{"~/.gitconfig", fragment, "~/.ssh/allowed_signers"},
		Backups: []string{NewBackupPath("~/.ssh/config"), NewBackupPath("~/.gitconfig")},
		Preview: "- [includeIf] → " + fragment + " (removed)\n- " + fragment +
			" (fragment removed)\n  Host " + sshHost + " (unchanged)",
		PreviewDiff:   true,
		ResultMessage: `Git identity of "` + sel.Name + `" deleted — the SSH side is untouched (state: incomplete).`,
		ConfirmLabel:  "Delete",
	})
}

// handleDeleteKey drives the scope chooser then the delete ceremony.
func (m identitiesModel) handleDeleteKey(msg tea.KeyMsg, s DemoState) keyResult {
	sel, _ := m.selectedIdentity(s)
	key := msg.String()

	if m.pane == paneDeleteScope {
		switch key {
		case "esc":
			m.pane = paneDetail
		case "up", "down", "tab", "shift+tab", "left", "right":
			// The two scope options ARE the focus ring (batch 3): Tab and
			// ←/→ move it exactly like ↑/↓.
			if m.deleteScope == "git-only" {
				m.deleteScope = "everything"
			} else {
				m.deleteScope = "git-only"
			}
		case "enter":
			m.deleteCerem = deleteCeremonyFor(sel, m.deleteScope)
			m.pane = paneDelete
		}
		return keyResult{model: m, handled: true}
	}

	var outcome ceremonyOutcome
	m.deleteCerem, outcome = m.deleteCerem.handleKey(msg)
	switch outcome {
	case ceremonyCancelled:
		m.pane = paneDetail
	case ceremonyFinished:
		deleted := sel.Name
		scope := m.deleteScope
		m.pane = paneDetail
		note := `Git identity of "` + deleted + `" deleted — SSH kept.`
		backup := NewBackupPath("~/.gitconfig")
		if scope == "everything" {
			note = `Identity "` + deleted + `" deleted (backups kept).`
			backup = NewBackupPath("~/.ssh/config")
			for _, row := range s.Identities {
				if row.Name != deleted {
					m.selected = row.Name
					break
				}
			}
		}
		return keyResult{model: m, handled: true, note: note,
			actions: []Action{DeleteIdentity{Name: deleted, Scope: scope, Backup: backup}}}
	case ceremonyNone, ceremonyConfirmed:
	}
	return keyResult{model: m, handled: true}
}

// handleFixKey drives the per-finding fix ceremony (detail pane Fix…).
func (m identitiesModel) handleFixKey(msg tea.KeyMsg, s DemoState) keyResult {
	var outcome ceremonyOutcome
	m.fixCeremony, outcome = m.fixCeremony.handleKey(msg)
	switch outcome {
	case ceremonyCancelled:
		m.pane = paneDetail
	case ceremonyFinished:
		id := m.fixFindingID
		m.pane = paneDetail
		// The backup targets the finding's OWN plan file (fixplans.go),
		// exactly like doctor.go's fix dispatch — never a hardcoded path.
		plan := FixPlan{File: "~/.ssh/config"} // fallback for an already-gone finding
		for _, f := range s.Findings {
			if f.ID == id {
				plan = PlanFor(f)
			}
		}
		return keyResult{model: m, handled: true, note: plan.Result,
			actions: []Action{FixFinding{ID: id, Backup: NewBackupPath(plan.File)}}}
	case ceremonyNone, ceremonyConfirmed:
	}
	return keyResult{model: m, handled: true}
}

// handleWizardKey drives the 4-pane-state create wizard.
func (m identitiesModel) handleWizardKey(msg tea.KeyMsg, s DemoState) keyResult { //nolint:gocyclo // one branch per wizard pane-state key, mirroring CreateWizard's handler
	key := msg.String()
	w := m.wizard

	// D7 (checkpoint-2 contract): ONE hoisted Shift+←/→ chord gate, ABOVE
	// the step switch — replaces the four scattered per-step cases (which
	// left step-3's review ceremony DEAD: its own handleKey's default:
	// swallowed the chord). Shift is a FOCUS-OVERRIDE reaching wizard
	// step-nav from EVERY step, including from inside a field, the
	// expanded strategy select, and now the review ceremony too.
	if key == "shift+left" {
		if w.step == 0 {
			// Step 0 back means leaving the wizard — Esc parity.
			m.pane = paneDetail
			return keyResult{model: m, handled: true}
		}
		m.wizard = w.stepBack()
		return keyResult{model: m, handled: true}
	}
	if key == "shift+right" {
		next, ok := w.stepForward(s)
		m.wizard = next
		note := ""
		if !ok {
			note = blockedForwardNote(w.step)
		}
		return keyResult{model: m, handled: true, note: note}
	}

	switch w.step {
	case 0:
		switch key {
		case "esc":
			m.pane = paneDetail
			return keyResult{model: m, handled: true}
		case "enter":
			if w.step0Valid(s) {
				w.step = 1
				w.testPhase = testIdle
			}
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "tab", "down":
			w.focus = (w.focus + 1) % 6
			w.form = w.form.setFocus(w.focus)
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "shift+tab", "up":
			w.focus = (w.focus + 5) % 6
			w.form = w.form.setFocus(w.focus)
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "left", "right":
			if w.focus == 5 { // algorithm select cycles over ENABLED entries
				delta := 1
				if key == "left" {
					delta = len(AlgorithmCatalog) - 1
				}
				idx := w.algoIdx
				for {
					idx = (idx + delta) % len(AlgorithmCatalog)
					if !algoDisabled(AlgorithmCatalog[idx]) {
						break
					}
				}
				w.algoIdx = idx
				m.wizard = w
				return keyResult{model: m, handled: true}
			}
			fallthrough
		default:
			if w.focus <= sshFieldPort {
				w.form = w.form.handleEdit(msg, w.focus)
			}
			m.wizard = w
			return keyResult{model: m, handled: true}
		}
	case 1:
		switch key {
		case "esc":
			w.step = 0
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "space":
			if w.testPhase == testIdle || w.testPhase == testFailed {
				w.simulateFail = !w.simulateFail
			}
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "c":
			if w.testPhase == testFailed {
				m.wizard = w
				return keyResult{model: m, handled: true, note: "Public key copied to clipboard (demo)."}
			}
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "enter":
			switch w.testPhase {
			case testIdle:
				w.testPhase = testRunning1
				m.wizard = w
				return keyResult{model: m, handled: true, cmd: runStageCmd(1)}
			case testStage1:
				w.testPhase = testRunning2
				m.wizard = w
				return keyResult{model: m, handled: true, cmd: runStageCmd(2)}
			case testFailed:
				w.simulateFail = false
				w.testPhase = testIdle
			case testStage2:
				w.step = 2
				w.gitFocus = gitFieldName
				w.git = w.git.setFocus(w.gitFocus)
			}
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "left":
			// Arrow-key precedence clause 3: the test step has no
			// field/select focus to contend with, so plain back always
			// goes to step 0 (the hoisted Shift gate handles shift+left
			// identically via stepBack()).
			w.step = 0
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "right":
			// Forward is validity-gated on the two-stage test having passed
			// — never bypassed (the hoisted Shift gate shares this exact
			// gate via stepForward()).
			if w.testPhase == testStage2 {
				w.step = 2
				w.gitFocus = gitFieldName
				w.git = w.git.setFocus(w.gitFocus)
			}
			m.wizard = w
			return keyResult{model: m, handled: true}
		default:
			return keyResult{model: m, handled: true}
		}
	case 2:
		switch key {
		case "esc":
			w.step = 1
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "enter":
			// Enter activates the focused button; on a FIELD it keeps
			// meaning Continue (web parity: Enter falls through from
			// single-line inputs as the primary action).
			switch w.gitFocus {
			case gitFocusBack:
				w.step = 1
			case gitFocusSkip:
				w.configureGit = false
				w.step = 3
				w.ceremony = w.reviewCeremony()
			default: // fields + Continue
				if w.git.valid() {
					w.configureGit = true
					w.step = 3
					w.ceremony = w.reviewCeremony()
				}
			}
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "tab", "down":
			w.gitFocus = (w.gitFocus + 1) % wizardGitFocusSlots
			w.git = w.git.setFocus(w.gitFocus)
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "shift+tab", "up":
			w.gitFocus = (w.gitFocus + wizardGitFocusSlots - 1) % wizardGitFocusSlots
			w.git = w.git.setFocus(w.gitFocus)
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "left", "right":
			// Arrow-key precedence (02-STYLE-SPEC.md §2): a button slot is a
			// NON-EDITING focus region, so <-/-> here now perform WIZARD-STEP
			// navigation (replacing the old button-ring-arrow behavior —
			// the field/button ring moves via Tab/Shift+Tab/Up/Down only).
			// A field or the expanded strategy select still owns <-/-> via
			// the fallthrough to handleEdit below (clauses 1/2).
			if w.gitFocus >= gitFocusBack {
				if key == "left" {
					w.step = 1
				} else if w.git.valid() {
					w.configureGit = true
					w.step = 3
					w.ceremony = w.reviewCeremony()
				}
				m.wizard = w
				return keyResult{model: m, handled: true}
			}
			fallthrough
		default:
			w.git = w.git.handleEdit(msg, w.gitFocus)
			m.wizard = w
			return keyResult{model: m, handled: true}
		}
	default: // step 3 — the ceremony owns the keys
		var outcome ceremonyOutcome
		w.ceremony, outcome = w.ceremony.handleKey(msg)
		m.wizard = w
		switch outcome {
		case ceremonyCancelled:
			w.step = 2
			m.wizard = w
		case ceremonyFinished:
			identity := w.finishIdentity()
			note := `Identity "` + identity.Name + `" created — SSH + Git configured (` + w.git.strategy() + `).`
			if !w.configureGit {
				note = `Identity "` + identity.Name + `" created — SSH only (incomplete). Configure Git from its detail.`
			}
			m.pane = paneDetail
			m.selected = identity.Name
			return keyResult{model: m, handled: true, note: note,
				actions: []Action{AddIdentity{Identity: identity, Backup: NewBackupPath("~/.ssh/config")}}}
		case ceremonyNone, ceremonyConfirmed:
		}
		return keyResult{model: m, handled: true}
	}
}

// ---------------------------------------------------------------------------
// Rendering.
// ---------------------------------------------------------------------------

// sidebarWidth is ~38% of the frame (spec §2 two panes ~38/62; 36% at the
// 100-col minimum so the detail pane keeps 63 usable columns).
func sidebarWidth(width int) int { return width * 36 / 100 }

// Sidebar row geometry — renderSidebar draws exactly these line counts and
// handleClick hit-tests against them, so the two can never drift apart.
const (
	sidebarLegendLines = 1 // the inline "S ssh · G git" legend line
	sidebarRowLines    = 2 // head line + faint note line per identity
)

// handleClick implements mouseTarget. A left click on a sidebar row (either
// of its two lines) selects that identity — detail mode only; while a
// form/ceremony pane is open the sidebar is dimmed and inert, exactly like
// the keyboard model. Clicks right of the divider hit-test the pane's
// button controls against the very body this model renders (batch-1
// pattern: zones derive from rendered strings) and dispatch the same key
// path the button advertises.
func (m identitiesModel) handleClick(x, y, width, height int, s DemoState) keyResult {
	if x < sidebarWidth(width) {
		if m.pane != paneDetail || y < sidebarLegendLines {
			return keyResult{model: m}
		}
		row := (y - sidebarLegendLines) / sidebarRowLines
		if row >= len(s.Identities) {
			return keyResult{model: m}
		}
		m.selected = s.Identities[row].Name
		return keyResult{model: m, handled: true}
	}

	body := m.view(s, width, height).body
	switch m.pane {
	case paneDetail:
		return m.handleDetailClick(body, x, y, s)
	case paneCreate:
		return m.handleWizardClick(body, x, y, s)
	case paneEditSSH:
		// Only the non-locked fields (Host/Hostname/Port) are focusable —
		// Provider/Alias prefix stay locked in edit mode.
		if slot, ok := hitAnyFieldRow(body, x, y, sshFormFieldSlots); ok && slot >= sshFieldHost {
			m.editFocus = slot
			m.editForm = m.editForm.setFocus(slot)
			return keyResult{model: m, handled: true}
		}
		if hitNeedle(body, x, y, " "+identEditRewriteButton+" ") {
			return m.handleEditKey(mustKey("Enter"), s)
		}
	case paneEditCeremony:
		if next, key, ok := ceremonyClickKey(m.editCeremony, body, x, y); ok {
			m.editCeremony = next
			return m.handleEditKey(key, s)
		}
	case paneGit:
		// review-findings F3: check the button needle FIRST — the
		// Write-it… button's own row can carry the disabled-suffix prose
		// ("...needs user.name...") when the form is invalid, and (belt +
		// suspenders alongside anchoredLabelMatch) a button match must never
		// be shadowed by a field-row match on the same rendered line.
		if hitNeedle(body, x, y, " "+identGitWriteButton+" ") {
			return m.handleGitKey(mustKey("Enter"), s)
		}
		sel, ok := m.selectedIdentity(s)
		if ok {
			if slot, hit := hitAnyFieldRow(body, x, y, gitFormFieldSlots); hit {
				m.gitFocus = slot
				m.gitPaneForm = m.gitPaneForm.setFocus(slot)
				return keyResult{model: m, handled: true}
			}
			if idx, hit := hitStrategyRow(body, x, y, sel.Name); hit {
				m.gitPaneForm.strategyIdx = idx
				m.gitFocus = gitFieldStrategy
				m.gitPaneForm = m.gitPaneForm.setFocus(gitFieldStrategy)
				return keyResult{model: m, handled: true}
			}
		}
	case paneGitCeremony:
		if next, key, ok := ceremonyClickKey(m.gitCeremony, body, x, y); ok {
			m.gitCeremony = next
			return m.handleGitKey(key, s)
		}
	case paneClone:
		if hitFieldRow(body, x, y, "New identity name") {
			m.cloneOnButton = false
			m.cloneInput.Focus()
			return keyResult{model: m, handled: true}
		}
		if hitNeedle(body, x, y, " "+identCloneButton+" ") {
			return m.handleCloneKey(mustKey("Enter"), s)
		}
	case paneDeleteScope:
		// Clicking a scope row chooses that scope (radio semantics).
		if line, ok := blockLine(body, y); ok {
			if strings.Contains(line, IdentityManagerDeleteChoiceGitOnly) {
				m.deleteScope = "git-only"
				return keyResult{model: m, handled: true}
			}
			if strings.Contains(line, IdentityManagerDeleteChoiceEverything) {
				m.deleteScope = "everything"
				return keyResult{model: m, handled: true}
			}
		}
	case paneDelete:
		if next, key, ok := ceremonyClickKey(m.deleteCerem, body, x, y); ok {
			m.deleteCerem = next
			return m.handleDeleteKey(key, s)
		}
	case paneFix:
		if next, key, ok := ceremonyClickKey(m.fixCeremony, body, x, y); ok {
			m.fixCeremony = next
			return m.handleFixKey(key, s)
		}
	}
	return keyResult{model: m}
}

// handleDetailClick resolves detail-pane clicks: the `[Configure now (g)]`
// button dispatches g, and a per-finding `Fix…` opens THAT finding's fix
// ceremony (the row's rendered line carries its title).
func (m identitiesModel) handleDetailClick(body string, x, y int, s DemoState) keyResult {
	sel, ok := m.selectedIdentity(s)
	if !ok {
		return keyResult{model: m}
	}
	if hitNeedle(body, x, y, identConfigureNowLabel) {
		return m.handleDetailKey(mustKey("g"), s)
	}
	if hitNeedle(body, x, y, identFixLinkLabel) {
		line, _ := blockLine(body, y)
		for _, f := range FindingsFor(s, sel.Name) {
			if f.SuggestedFix != "" && strings.Contains(line, f.Title) {
				m.pane = paneFix
				m.fixFindingID = f.ID
				m.fixCeremony = fixCeremonyFor(f)
				return keyResult{model: m, handled: true}
			}
		}
	}
	return keyResult{model: m}
}

// fieldSlot pairs a rendered field label with the focus slot clicking it
// selects (D8 click-to-focus, checkpoint-2 contract).
type fieldSlot struct {
	label string
	slot  int
}

// sshFormFieldSlots are the SSH form's field rows, in render order — shared
// by the wizard step 0 and edit-SSH click handlers.
var sshFormFieldSlots = []fieldSlot{
	{"Provider", sshFieldProvider},
	{"Alias prefix", sshFieldPrefix},
	{"SSH Host (alias)", sshFieldHost},
	{"Real hostname", sshFieldHostname},
	{"Port", sshFieldPort},
}

// gitFormFieldSlots are the Git form's field rows, in render order — shared
// by the wizard step 2 and Configure-Git click handlers.
var gitFormFieldSlots = []fieldSlot{
	{"user.name", gitFieldName},
	{"user.email", gitFieldEmail},
}

// anchoredLabelMatch reports whether label anchors row line (D8 click-to-
// focus; review-findings F3): the row must START WITH label — after
// skipping only the row's OWN leading gutter/marker glyphs (spaces, "▸",
// and the checkbox/radio glyphs) — either immediately followed by a
// formFieldLine's "[value]" brackets (the ONLY row shape with brackets
// adjacent to a padded label) or by a genuine word boundary. A bare
// strings.Contains previously let THREE things falsely hijack a click:
// (a) a button row's disabled-suffix prose (e.g. "...needs user.name...")
// contains "user.name" mid-sentence, but never at the row's own start;
// (b) a disabled algorithm row like "ed25519-sk" contains the ENABLED
// "ed25519" as a substring — requiring a word boundary immediately after
// the label (not '-', not a letter/digit) rejects it; (c) a bordered
// PreviewBlock/config-preview line (e.g. "Port 443", "id_ed25519_acme")
// ALWAYS starts with its own border glyph ("┊"), which is never stripped
// as a gutter char, so preview text can never satisfy the prefix check.
func anchoredLabelMatch(line, label string) bool {
	// Every rendered row is actually sidebar-content + "│" divider +
	// detail-pane content (joinMasterDetail) — a field/radio/checkbox row's
	// OWN gutter/marker glyphs start right after the divider, not at column
	// 0 of the full physical row. Slicing at the LAST "│" (the divider is
	// the only place this glyph is drawn) isolates the detail-pane's own
	// text before anchoring, so a coincidental label-shaped substring in
	// the sidebar can never be mistaken for the field row itself.
	region := line
	if idx := strings.LastIndex(line, "│"); idx >= 0 {
		region = line[idx+len("│"):]
	}
	trimmed := strings.TrimLeft(region, " ")
	trimmed = strings.TrimPrefix(trimmed, "▸")
	for _, marker := range []string{glyphCheckOn, glyphCheckOff, glyphRadioOn, glyphRadioOff} {
		if strings.HasPrefix(trimmed, marker) {
			trimmed = strings.TrimPrefix(trimmed, marker)
			break
		}
	}
	trimmed = strings.TrimLeft(trimmed, " ")
	if !strings.HasPrefix(trimmed, label) {
		return false
	}
	rest := trimmed[len(label):]
	if strings.HasPrefix(strings.TrimLeft(rest, " "), "[") {
		return true // a formFieldLine's padded label + bracketed value
	}
	if rest == "" {
		return true
	}
	switch rest[0] {
	case ' ', '\t':
		return true
	default:
		return false // e.g. "ed25519" would falsely match inside "ed25519-sk"
	}
}

// hitFieldRow reports whether (x, y) falls anywhere on the rendered row
// ANCHORED by label — D8's "the ENTIRE rendered field row is the hit
// target" (Fitts's law), not just the label span; anchoredLabelMatch keeps
// this from matching a label that merely appears somewhere else on the same
// display row (review-findings F3).
func hitFieldRow(body string, x, y int, label string) bool {
	line, ok := blockLine(body, y)
	if !ok || !anchoredLabelMatch(line, label) {
		return false
	}
	return x >= 0 && x < ansi.StringWidth(line)
}

// hitAnyFieldRow resolves which of the given field rows (x, y) falls on.
func hitAnyFieldRow(body string, x, y int, fields []fieldSlot) (int, bool) {
	for _, f := range fields {
		if hitFieldRow(body, x, y, f.label) {
			return f.slot, true
		}
	}
	return 0, false
}

// hitAlgorithmRow resolves which ENABLED algorithm catalog row (x, y) falls
// on; entries algoDisabled reports as unavailable stay inert (D8).
func hitAlgorithmRow(body string, x, y int) (int, bool) {
	for i, entry := range AlgorithmCatalog {
		if algoDisabled(entry) {
			continue
		}
		if hitFieldRow(body, x, y, entry.ID) {
			return i, true
		}
	}
	return 0, false
}

// hitStrategyRow resolves which match-strategy option row (x, y) falls on
// (D8 — reused by the wizard Git step and the Configure-Git pane).
func hitStrategyRow(body string, x, y int, identityName string) (int, bool) {
	for i, strategyID := range matchStrategies {
		if hitFieldRow(body, x, y, strategyCopy(strategyID, identityName)) {
			return i, true
		}
	}
	return 0, false
}

// handleWizardClick resolves the wizard's per-step field/radio/button clicks.
func (m identitiesModel) handleWizardClick(body string, x, y int, s DemoState) keyResult {
	w := m.wizard
	switch w.step {
	case 0:
		if slot, ok := hitAnyFieldRow(body, x, y, sshFormFieldSlots); ok {
			w.focus = slot
			w.form = w.form.setFocus(slot)
			m.wizard = w
			return keyResult{model: m, handled: true}
		}
		if idx, ok := hitAlgorithmRow(body, x, y); ok {
			w.focus = 5
			w.algoIdx = idx
			w.form = w.form.setFocus(5)
			m.wizard = w
			return keyResult{model: m, handled: true}
		}
	case 1:
		for _, needle := range []string{
			" Run stage 1 (Enter) ", " Retry (Enter) ",
			" Run stage 2 (Enter) ", " Next: Git identity (Enter) ",
		} {
			if hitNeedle(body, x, y, needle) {
				return m.handleWizardKey(mustKey("Enter"), s)
			}
		}
		if hitNeedle(body, x, y, "simulate a provider failure") {
			return m.handleWizardKey(mustKey("space"), s)
		}
		if hitNeedle(body, x, y, "Copy public key") {
			return m.handleWizardKey(mustKey("c"), s)
		}
	case 2:
		// review-findings F3(a): check the button needles FIRST — Back /
		// Skip Git / Continue share ONE row, and when the form is invalid
		// Continue's own disabled-suffix prose ("...needs user.name...")
		// used to satisfy a bare strings.Contains match against the
		// user.name FIELD's label, hijacking a click meant for Back/Skip
		// (belt + suspenders alongside anchoredLabelMatch's anchoring fix).
		buttons := []struct {
			needle string
			slot   int
		}{
			{" " + wizardBackButton + " ", gitFocusBack},
			{" " + wizardSkipButton + " ", gitFocusSkip},
			{" " + wizardContinueButton + " ", gitFocusContinue},
		}
		for _, b := range buttons {
			if hitNeedle(body, x, y, b.needle) {
				// Click = focus that button, then activate it — the same
				// Enter path the keyboard uses.
				m.wizard.gitFocus = b.slot
				m.wizard.git = m.wizard.git.setFocus(b.slot)
				return m.handleWizardKey(mustKey("Enter"), s)
			}
		}
		if slot, ok := hitAnyFieldRow(body, x, y, gitFormFieldSlots); ok {
			w.gitFocus = slot
			w.git = w.git.setFocus(slot)
			m.wizard = w
			return keyResult{model: m, handled: true}
		}
		if idx, ok := hitStrategyRow(body, x, y, w.form.identityName()); ok {
			w.git.strategyIdx = idx
			w.gitFocus = gitFieldStrategy
			w.git = w.git.setFocus(gitFieldStrategy)
			m.wizard = w
			return keyResult{model: m, handled: true}
		}
	case 3:
		if next, key, ok := ceremonyClickKey(w.ceremony, body, x, y); ok {
			m.wizard.ceremony = next
			return m.handleWizardKey(key, s)
		}
	}
	return keyResult{model: m}
}

// renderSidebar renders the identity list: inline legend, then one row per
// identity (tone glyph + bold name + N⚑ + S/G pips) with a faint note.
// The sidebar dims while a form/ceremony pane is open.
func (m identitiesModel) renderSidebar(s DemoState, width int, dimmed bool) string {
	var lines []string
	lines = append(lines, styleFaint.Render("S ssh · G git  ✓ ok ! attn ✗ broken"))
	sel, _ := m.selectedIdentity(s)
	for _, row := range s.Identities {
		pS, pG := pips(row)
		glyph := toneStyle(IdentityManagerStateTone[row.State]).Render(IdentityManagerGlyphByState[row.State])
		marker := "  "
		name := styleBold.Render(row.Name)
		if row.Name == sel.Name {
			marker = styleBold.Render("▸ ")
			name = styleSelected.Render(row.Name)
		}
		flags := ""
		if n := len(FindingsFor(s, row.Name)); n > 0 {
			flags = styleWarning.Render(fmt.Sprintf(" %d⚑", n))
		}
		pipText := stylePipNone.Render("S") + pipStyle(pS).Render(pS) + " " + stylePipNone.Render("G") + pipStyle(pG).Render(pG)
		head := marker + glyph + " " + name + flags
		pad := width - ansi.StringWidth(head) - ansi.StringWidth(pipText) - 1
		if pad < 1 {
			pad = 1
		}
		lines = append(lines, ansi.Truncate(head+strings.Repeat(" ", pad)+pipText, width, ""))
		lines = append(lines, ansi.Truncate("    "+styleFaint.Render(row.Note), width, "…"))
	}
	if dimmed {
		return dimPane(strings.Join(lines, "\n"))
	}
	return strings.Join(lines, "\n")
}

// baselineStrip renders the read-only inherited global-baseline strip
// (GITUI-01 kept intact) with the Global Git jump hint. The long value
// line word-wraps to the pane width.
func baselineStrip(s DemoState) string {
	applied := " — not applied yet"
	if s.GitBaselineApplied {
		applied = ", applied ✓"
	}
	return " " + styleFaint.Render("Global baseline (inherited"+applied+"): "+GlobalGitBaselineStripText) +
		"\n   " + styleFocusLink.Render("Edit in Global Git (3)")
}

// baselineStripCompact is the one-line variant for tall form panes.
func baselineStripCompact(s DemoState, width int) string {
	applied := " — not applied yet"
	if s.GitBaselineApplied {
		applied = ", applied ✓"
	}
	return " " + styleFaint.Render(ansi.Truncate("Global baseline (inherited"+applied+"): "+GlobalGitBaselineStripText, width-6, "…")) +
		" " + styleFocusLink.Render("(3)")
}

// renderDetail renders the identity detail: SSH section FIRST, Git section
// (or Configure now), the baseline strip, and the findings sub-panel.
func (m identitiesModel) renderDetail(s DemoState, sel DemoIdentity) string {
	var b strings.Builder
	tone := toneStyle(IdentityManagerStateTone[sel.State])
	b.WriteString(" " + styleBold.Render(sel.Name) + "  " +
		tone.Render(IdentityManagerGlyphByState[sel.State]+" "+sel.State) + "\n\n")

	b.WriteString(sectionHeader("SSH — shown first, always") + "\n")
	if sel.SSHHost != "" {
		hostname := sel.Hostname
		if hostname == "" {
			hostname = "ssh.github.com"
		}
		port := sel.Port
		if port == 0 {
			port = 443
		}
		keyPath := sel.KeyPath
		if keyPath == "" {
			keyPath = "— missing"
		}
		b.WriteString("   Host alias: " + sel.SSHHost + "\n")
		b.WriteString("   Hostname: " + hostname + " · Port " + strconv.Itoa(port) + " · User git\n")
		b.WriteString("   IdentityFile: " + keyPath + "\n")
		b.WriteString("   IdentitiesOnly: yes\n")
	} else {
		b.WriteString("   " + styleWarning.Render("! No gitid-managed Host block — relies on the global SSH config.") + "\n")
	}

	b.WriteString("\n" + sectionHeader("Git") + "\n")
	if sel.GitFragmentPath != "" {
		b.WriteString("   Fragment: " + sel.GitFragmentPath + "\n")
		b.WriteString("   Author: " + sel.GitName + " <" + sel.GitEmail + ">\n")
		signing := "   Signing: gpg.format=ssh · signingkey " + orDefault(sel.KeyPath, "?") + ".pub"
		if sel.MatchStrategy != "" {
			signing += " · strategy " + sel.MatchStrategy
		}
		b.WriteString(signing + "\n")
	} else {
		b.WriteString("   " + styleWarning.Render("! Git not configured — no fabricated values shown.") +
			"  " + styleBold.Render(identConfigureNowLabel) + "\n")
	}
	b.WriteString(baselineStrip(s) + "\n")

	findings := FindingsFor(s, sel.Name)
	b.WriteString("\n" + sectionHeader(fmt.Sprintf("Findings (%d) — same data the Doctor shows (4)", len(findings))) + "\n")
	if len(findings) == 0 {
		b.WriteString("   " + styleHealthy.Render(`✓ No findings for "`+sel.Name+`".`) + "\n")
	} else {
		for _, f := range findings {
			fix := styleFaint.Render("info only")
			if f.SuggestedFix != "" {
				fix = styleBold.Render("f") + " " + styleFaint.Render(identFixLinkLabel)
			}
			b.WriteString("   " + severityLabel(f.Severity) + "  " + f.Title + "  " + fix + "\n")
		}
		b.WriteString("   " + styleFocusLink.Render("Open the Doctor (4) for the global picture") + "\n")
	}
	return b.String()
}

// Button labels shared by the renderers and the click hit-tests (batch 3 —
// zones derive from the exact rendered strings, never magic numbers).
const (
	identEditRewriteButton = "Rewrite Host block… (Enter)"
	identGitWriteButton    = "Write it… (Enter)"
	identCloneButton       = "Clone (Enter)"
	identConfigureNowLabel = "[Configure now (g)]"
	identFixLinkLabel      = "Fix…"
	wizardBackButton       = "Back (Esc)"
	// wizardSkipButton/wizardContinueButton carry the FROZEN slide-3 copy
	// (02-STYLE-SPEC.md §4) — the long explanation moved OFF the button
	// onto the adjacent hint line below (wizardSkipHint/wizardContinueHint),
	// never re-derived here or in the web demo.
	wizardSkipButton     = "[ Skip Git ]"
	wizardContinueButton = "[ Continue ]"
	// wizardSkipHint/wizardContinueHint are the frozen adjacent hint lines
	// (02-STYLE-SPEC.md §4), always rendered next to their button (Theme.Hint).
	wizardSkipHint     = "Skip keeps this identity SSH-only and marks it incomplete."
	wizardContinueHint = "Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing."
)

// wizardButton renders one focusable wizard control: reverse-video when
// focused (the same focus treatment the ceremony buttons carry), bold when
// merely enabled, faint + an explicit disabledSuffix when disabled (M2). The
// suffix is caller-supplied (D7, checkpoint-2 contract) — the generic
// "— disabled" text is gone from every caller; pass "" for buttons that are
// always enabled (the suffix branch is then unreachable).
func wizardButton(label string, focused, enabled bool, disabledSuffix string) string {
	text := " " + label + " "
	switch {
	case focused && enabled:
		return styleSelected.Render(text)
	case focused:
		return lipgloss.NewStyle().Faint(true).Reverse(true).Render(text + disabledSuffix)
	case enabled:
		return styleBold.Render(text)
	default:
		return styleFaint.Render(text + disabledSuffix)
	}
}

// gitFormDisabledSuffix is the frozen D7 disabled-suffix copy for any
// button gated on gitForm.valid() (the wizard's [ Continue ] and the
// Configure-Git pane's Write-it… button share the same validity predicate).
const gitFormDisabledSuffix = "— needs user.name + a valid email"

// cloneDisabledSuffix is the Clone button's disabled-suffix copy (gated on
// a unique, non-empty clone name) — not frozen by the copy-freeze grep, but
// still never the generic "— disabled" text (D7 forbids it everywhere).
const cloneDisabledSuffix = "— needs a unique, non-empty name"

// styleStepperActive is the active stepper segment's treatment: bold +
// Theme.Accent (NOT styleFaint — the old `Step n/4` line read dimmer than
// body text, the opposite of a navigation affordance; 02-STYLE-SPEC.md §5).
var styleStepperActive = lipgloss.NewStyle().Bold(true).Foreground(DefaultTheme.Accent)

// renderStepper renders `Step n/4 · <label> ● ○ ○ ○` (D5, checkpoint-2
// contract — REVERTS 02-14's bracketed short-segment stepper (bracket-number
// + short word per step); the bracket format moved onto the MAIN NAV per
// D4). The counter is bold, the `·` separator is faint, the active LONG label
// (wizardSteps) carries styleStepperActive (bold + accent — the line is
// NEVER faint as a whole), and the step dots render `●` accent for indices
// ≤ step / `○` faint for the rest.
func renderStepper(step int) string {
	dots := make([]string, len(wizardSteps))
	for i := range wizardSteps {
		if i <= step {
			dots[i] = DefaultTheme.ActiveArea.Render(glyphRadioOn)
		} else {
			dots[i] = styleFaint.Render(glyphRadioOff)
		}
	}
	return " " + styleBold.Render(fmt.Sprintf("Step %d/%d", step+1, len(wizardSteps))) +
		styleFaint.Render(" · ") + styleStepperActive.Render(wizardSteps[step]) +
		" " + strings.Join(dots, " ")
}

// wizardChordHint renders the ALWAYS-visible, step-conditional faint line
// directly under the stepper (D5/D7, checkpoint-2 contract) advertising the
// hoisted Shift+←/→ chord gate — frozen copy, verbatim.
func wizardChordHint(step int) string {
	switch step {
	case 0:
		return "Shift+→ next section · Shift+← exits the wizard"
	case len(wizardSteps) - 1:
		return "Shift+← back to Git · Enter writes"
	default:
		return "Shift+←/→ jump sections · forward needs a valid step"
	}
}

// renderWizard renders the active wizard pane-state.
func (m identitiesModel) renderWizard(s DemoState, width int) string {
	w := m.wizard
	var b strings.Builder
	b.WriteString(renderStepper(w.step) + "\n")
	b.WriteString(" " + styleFaint.Render(wizardChordHint(w.step)) + "\n")

	switch w.step {
	case 0:
		prefixError := ""
		if w.nameTaken(s) {
			prefixError = `"` + w.form.identityName() + `" already exists — pick another prefix.`
		}
		hostHelper := "Auto-joined: <prefix>.<provider> — editable"
		if w.form.hostTouched {
			hostHelper = "Manually edited — auto-join off"
		}
		b.WriteString(w.form.view(w.focus, prefixError, hostHelper))

		marker := "  "
		if w.focus == 5 {
			marker = styleBold.Render("▸ ")
		}
		b.WriteString(" " + marker + styleBold.Render("Key algorithm") + " " + styleFaint.Render("(←/→ change)") + "\n")
		for i, entry := range AlgorithmCatalog {
			dot := glyphRadioOff
			if i == w.algoIdx {
				dot = glyphRadioOn
			}
			label := entry.ID
			if entry.Recommended {
				label += " — ★ recommended"
			}
			if algoDisabled(entry) {
				b.WriteString("     " + styleFaint.Render(dot+" "+label+" — Disabled: needs libfido2 + a FIDO2 security key — none detected on this machine") + "\n")
			} else {
				b.WriteString("     " + dot + " " + label + "\n")
			}
		}
		if w.focus == 5 {
			// Verbatim web helper copy (Identities.tsx) — spec-bearing (L2).
			b.WriteString(helperLine("gitid probes the local toolchain (ssh-keygen, libfido2, FIDO2 key present?) and disables what this machine cannot generate, with the reason shown per option (KEY-03/PLAT-01). Demo simulates: no FIDO2 key plugged in.", false) + "\n")
		}
		b.WriteString(renderHostBlockPreview(w.form.sshHost(), w.form.hostname.Value(), w.form.port.Value(), w.keyPath(), width))
	case 1:
		b.WriteString(" " + styleInfo.Render("Key "+w.keyPath()+" generated ("+w.algo()+").") + "\n")
		b.WriteString(" " + styleInfo.Render("Both stages run against "+CreateFlowTestTmpConfig+" — your live ~/.ssh/config is untouched until the final confirm.") + "\n\n")

		check := glyphCheckOff
		if w.simulateFail {
			check = glyphCheckOn
		}
		toggle := check + " Demo control — simulate a provider failure (key not registered) to preview the error path"
		if w.testPhase != testIdle && w.testPhase != testFailed {
			// Locked (running / passed) — compact single line reclaims rows
			// for the stage-2 output at the 30-row minimum.
			b.WriteString(" " + styleFaint.Render(check+" Demo failure control — locked (stage running or test passed)") + "\n")
		} else {
			b.WriteString(" " + toggle + "  " + styleFaint.Render("(space toggles)") + "\n")
			b.WriteString(helperLine("Review aid only, not part of the real flow. It locks while a stage is running and once the test has passed — there is nothing left to simulate then.", false) + "\n")
		}
		b.WriteString("\n")

		// Routed through the bounded, titled PreviewBlock (review-findings
		// F1) — the "Stage 1 — ..." description moves into the border's top
		// edge instead of a separate description row.
		b.WriteString(PreviewBlock("Stage 1 — key DIRECT against the provider (TEST-01)", "$ "+w.stage1Cmd(), false, width, 2) + "\n")
		switch w.testPhase {
		case testIdle:
			b.WriteString(" " + styleSelected.Render(" Run stage 1 (Enter) ") + "\n")
		case testRunning1, testRunning2:
			b.WriteString(" " + styleFaint.Render("… running ssh…") + "\n")
		case testFailed:
			b.WriteString(" " + styleError.Render("✗ git@"+w.form.hostname.Value()+": Permission denied (publickey).") + "\n")
			b.WriteString(" " + styleError.Render("The provider rejected the key — usually it is not registered yet. Copy the public key,") + "\n")
			b.WriteString(" " + styleError.Render("add it to your provider account, then retry.") + "\n")
			b.WriteString(" " + styleBold.Render("c") + " " + styleFaint.Render("Copy public key") + "   " + styleSelected.Render(" Retry (Enter) ") + "\n")
		case testStage1, testStage2:
			b.WriteString(" " + styleHealthy.Render("✓ Hi "+w.form.identityName()+"! You've successfully authenticated, but GitHub does not provide shell access.") + "\n")
		}
		if w.testPhase == testStage1 || w.testPhase == testStage2 {
			// The short "Stage 2 — ..." title moves into the border's top
			// edge (review-findings F1); the longer no-`-i`-on-purpose
			// rationale stays as an adjacent hint line (regionFlat/paneFlat
			// normalize whitespace, so the exact wrap point does not matter
			// for the pinned assertion).
			b.WriteString("\n " + styleFaint.Render("No -i here on purpose: the config must supply the key; that is exactly what this stage proves.") + "\n")
			b.WriteString(PreviewBlock("Stage 2 — resolve BY ALIAS (TEST-02)", "$ "+w.stage2Cmd(), false, width, 2) + "\n")
			if w.testPhase == testStage1 {
				b.WriteString(" " + styleSelected.Render(" Run stage 2 (Enter) ") + "\n")
			} else {
				b.WriteString(" " + styleHealthy.Render("✓ identityfile "+w.keyPath()) + "\n")
				b.WriteString(" " + styleSelected.Render(" Next: Git identity (Enter) ") + "\n")
			}
		}
	case 2:
		b.WriteString(w.git.view(w.form.identityName(), w.keyPath(), w.gitFocus, width, baselineStripCompact(s, width)))
		// D6 (checkpoint-2 contract): all THREE real buttons (M2) share ONE
		// row — Back / Skip / Continue — with both frozen hints ALWAYS
		// visible BELOW the row (Theme.Hint), never on the button itself.
		b.WriteString(" " + wizardButton(wizardBackButton, w.gitFocus == gitFocusBack, true, "") + "  " +
			wizardButton(wizardSkipButton, w.gitFocus == gitFocusSkip, true, "") + "  " +
			wizardButton(wizardContinueButton, w.gitFocus == gitFocusContinue, w.git.valid(), gitFormDisabledSuffix) + "\n")
		b.WriteString(" " + styleFaint.Render(wizardSkipHint) + "\n")
		b.WriteString(" " + styleFaint.Render(wizardContinueHint))
	default:
		b.WriteString(w.ceremony.view(width))
	}
	return b.String()
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// view implements screenModel: sidebar + the active right-pane state.
func (m identitiesModel) view(s DemoState, width, height int) screenView {
	sel, _ := m.selectedIdentity(s)
	sbWidth := sidebarWidth(width)
	detailWidth := width - sbWidth - masterDetailGutter

	var body string
	var crumbs []string
	var actions []FooterAction
	status := fmt.Sprintf("%d identities — selection renders the detail live; every action is dummy but really changes this state.", len(s.Identities))

	var pane string
	switch m.pane {
	case paneDetail:
		pane = m.renderDetail(s, sel)
		crumbs = []string{sel.Name}
		actions = []FooterAction{
			{Key: "↑↓", Label: "select identity"},
			{Key: "n", Label: "new"},
			{Key: "e", Label: "edit SSH"},
			{Key: "g", Label: "configure Git"},
			{Key: "c", Label: "clone"},
			{Key: "d", Label: "delete"},
		}
		if _, found := firstFixableFinding(s, sel.Name); found {
			actions = append(actions, FooterAction{Key: "f", Label: "fix finding"})
		}
	case paneCreate:
		pane = m.renderWizard(s, detailWidth)
		// wizardSteps (the LONG labels) is the breadcrumb/help source —
		// renderStepper draws the frozen SHORT segments independently
		// (02-STYLE-SPEC.md §5).
		crumbs = []string{"New identity", wizardSteps[m.wizard.step]}
		actions = m.wizardFooter(s)
		status = "Esc returns to the identity detail without writing anything."
	case paneEditSSH:
		// The SAME live preview the create wizard renders, rebuilt on every
		// keystroke (M1) — the confirm ceremony stays the pane's next state
		// (deviation #8: designer-arbitrated; the preview is inline).
		editKeyPath := orDefault(sel.KeyPath, "~/.ssh/id_ed25519_"+sel.Name)
		pane = " " + styleBold.Render("Edit SSH — "+sel.Name) + "\n" +
			m.editForm.view(m.editFocus, "", "") +
			renderHostBlockPreview(m.editForm.host.Value(), m.editForm.hostname.Value(),
				m.editForm.port.Value(), editKeyPath, detailWidth) +
			"\n\n " + wizardButton(identEditRewriteButton, m.editFocus == editFocusButton, true, "")
		crumbs = []string{sel.Name, "Edit SSH"}
		actions = []FooterAction{{Key: "Tab/↑↓", Label: "fields"}, {Key: "Enter", Label: "rewrite Host block"}}
		status = "Esc returns to the identity detail without writing anything."
	case paneEditCeremony:
		pane = m.editCeremony.view(detailWidth)
		crumbs = []string{sel.Name, "Edit SSH"}
		actions = ceremonyFooterActions()
		status = "Esc returns to the identity detail without writing anything."
	case paneGit:
		suffix := " (completes this identity)"
		if m.gitExisting {
			suffix = " (editing existing fragment)"
		}
		pane = " " + styleBold.Render("Git identity — "+sel.Name+suffix) + "\n" +
			m.gitPaneForm.view(sel.Name, orDefault(sel.KeyPath, "~/.ssh/id_ed25519_"+sel.Name), m.gitFocus, detailWidth, baselineStripCompact(s, detailWidth)) +
			" " + wizardButton(identGitWriteButton, m.gitFocus == gitPaneFocusButton, m.gitPaneForm.valid(), gitFormDisabledSuffix)
		crumbs = []string{sel.Name, "Configure Git"}
		actions = []FooterAction{{Key: "Tab/↑↓", Label: "fields"}, {Key: "Enter", Label: "write Git identity"}}
		status = "Esc returns to the identity detail without writing anything."
	case paneGitCeremony:
		pane = m.gitCeremony.view(detailWidth)
		crumbs = []string{sel.Name, "Configure Git"}
		actions = ceremonyFooterActions()
		status = "Esc returns to the identity detail without writing anything."
	case paneClone:
		taken := hasIdentityNamed(s, m.cloneInput.Value())
		helper := "Creates " + m.cloneInput.Value() + ".github.com + ~/.ssh/id_ed25519_" + m.cloneInput.Value()
		if taken {
			helper = "That name already exists."
		}
		cloneValid := !taken && strings.TrimSpace(m.cloneInput.Value()) != ""
		pane = " " + styleBold.Render(`Clone "`+sel.Name+`"`) + "\n" +
			" " + styleFaint.Render("The clone gets its own new key and Host alias; the Git author is copied (MGR-04).") + "\n\n" +
			formFieldLine("New identity name", m.cloneInput, !m.cloneOnButton, false) + "\n" +
			helperLine(helper, !cloneValid) + "\n\n" +
			" " + wizardButton(identCloneButton, m.cloneOnButton, cloneValid, cloneDisabledSuffix)
		crumbs = []string{sel.Name, "Clone"}
		actions = []FooterAction{{Key: "Tab", Label: "switch"}, {Key: "Enter", Label: "clone"}}
		status = "Esc returns to the identity detail without writing anything."
	case paneDeleteScope:
		// The chosen scope IS the focused control (the 2-option ring):
		// reverse-video marks it like every other focused button.
		gitOnlyText := IdentityManagerDeleteChoiceGitOnly + " (safer — SSH stays)"
		everythingText := IdentityManagerDeleteChoiceEverything + " — irreversible"
		gitOnlyLine := glyphRadioOff + " " + gitOnlyText
		everythingLine := glyphRadioOff + " " + styleError.Render(everythingText)
		if m.deleteScope == "git-only" {
			gitOnlyLine = glyphRadioOn + " " + styleSelected.Render(gitOnlyText)
		} else {
			everythingLine = glyphRadioOn + " " + styleError.Reverse(true).Render(everythingText)
		}
		pane = " " + styleBold.Render(`Delete "`+sel.Name+`" — choose scope`) + "\n\n" +
			"  " + gitOnlyLine + "\n" +
			"  " + everythingLine + "\n\n" +
			" " + styleFaint.Render("↑↓/Tab choose · Enter continue · Esc cancel")
		crumbs = []string{sel.Name, "Delete"}
		status = "Esc returns to the identity detail without writing anything."
	case paneDelete:
		pane = m.deleteCerem.view(detailWidth)
		crumbs = []string{sel.Name, "Delete"}
		actions = ceremonyFooterActions()
		status = "Esc returns to the identity detail without writing anything."
	case paneFix:
		pane = m.fixCeremony.view(detailWidth)
		crumbs = []string{sel.Name, "Fix"}
		actions = ceremonyFooterActions()
		status = "Esc returns to the identity detail without writing anything."
	}

	sidebar := m.renderSidebar(s, sbWidth, m.pane != paneDetail)
	// Word-wrap the pane at the detail width so long spec copy flows to
	// continuation lines instead of being hard-truncated by the frame; the
	// shared full-height divider separates master from detail (H2).
	body = joinMasterDetail(sidebar, sbWidth,
		lipgloss.NewStyle().Width(detailWidth).Render(pane), frameBodyRows(height))
	return screenView{body: body, crumbs: crumbs, status: status, statusTone: "info",
		actions: actions, capturesKeys: m.paneCapturesKeys()}
}

// paneCapturesKeys reports whether the active pane state consumes plain
// keys, so the frame renders the honest reserved footer (Esc/Ctrl+P only).
// Every non-detail pane does: forms and typed-confirm inputs swallow text,
// and the wizard test step, selects, choosers, and ceremonies consume every
// plain key too (batch 3 follow-up — q/? never reach the globals there).
func (m identitiesModel) paneCapturesKeys() bool {
	return m.pane != paneDetail
}

// ceremonyFooterActions renders the audit-table contextual footer shared by
// EVERY write ceremony (frozen copy, verbatim: `Tab/←→ Cancel / Confirm` +
// `Enter confirm`) — chrome, costs zero body rows.
func ceremonyFooterActions() []FooterAction {
	return []FooterAction{{Key: "Tab/←→", Label: "Cancel / Confirm"}, {Key: "Enter", Label: "confirm"}}
}

// wizardFooter renders the wizard's contextual footer hints per pane-state.
func (m identitiesModel) wizardFooter(s DemoState) []FooterAction {
	w := m.wizard
	switch w.step {
	case 0:
		next := FooterAction{Key: "Enter", Label: "next: test connection"}
		if !w.step0Valid(s) {
			next.Label = "next (fix fields first)"
		}
		return []FooterAction{{Key: "Tab/↑↓", Label: "fields"}, next}
	case 1:
		switch w.testPhase {
		case testIdle:
			return []FooterAction{{Key: "Enter", Label: "run stage 1"}, {Key: "space", Label: "toggle failure demo"}}
		case testFailed:
			return []FooterAction{{Key: "Enter", Label: "retry"}, {Key: "c", Label: "copy public key"}}
		case testStage1:
			return []FooterAction{{Key: "Enter", Label: "run stage 2"}}
		case testStage2:
			return []FooterAction{{Key: "Enter", Label: "next: Git identity"}}
		}
		return nil
	case 2:
		return []FooterAction{
			{Key: "Enter", Label: "activate focused / continue"},
			{Key: "Tab/↑↓", Label: "fields & buttons"},
		}
	default: // step 3 — the review ceremony owns the keys
		return ceremonyFooterActions()
	}
}
