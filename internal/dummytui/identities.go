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

// formFieldLine renders one "one field per row" line (round-3 feedback:
// a single column keeps every editable box unmistakable).
func formFieldLine(label string, input textinput.Model, focused, locked bool) string {
	marker := "  "
	if focused {
		marker = styleBold.Render("▸ ")
	}
	name := styleBold.Render(padRight(label+":", 18))
	value := input.View()
	if !focused {
		value = input.Value()
	}
	if locked {
		value = styleFaint.Render(input.Value() + "  (locked)")
	}
	return " " + marker + name + value
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
	b.WriteString(" " + styleFaint.Render("Signing: gpg.format = ssh (fixed) · signingkey = "+keyPath+".pub — a PATH, never key material.") + "\n")

	marker := "  "
	if focus == gitFieldStrategy {
		marker = styleBold.Render("▸ ")
	}
	b.WriteString(" " + marker + styleBold.Render("Match strategy — when does this Git identity apply?") + "\n")
	if focus == gitFieldStrategy {
		for i, s := range matchStrategies {
			dot := "○"
			if i == g.strategyIdx {
				dot = "●"
			}
			b.WriteString("     " + dot + " " + strategyCopy(s, name) + "\n")
		}
	} else {
		b.WriteString("     ● " + strategyCopy(g.strategy(), name) + "  " + styleFaint.Render("(←/→ change)") + "\n")
	}

	b.WriteString(" " + PreviewLabel("~/.gitconfig.d/"+name+" (fragment file — preview)") + "\n")
	b.WriteString(previewBlockClipped(g.fragmentPreview(keyPath), false, width, 4) + "\n")
	b.WriteString(" " + PreviewLabel("~/.gitconfig (includeIf block — preview)") + "\n")
	b.WriteString(previewBlockClipped(g.includeIfPreview(name), false, width, 2) + "\n")
	b.WriteString(baseline + "\n")
	return b.String()
}

// ---------------------------------------------------------------------------
// Create wizard — 4 pane-states in the detail pane (spec §3).
// ---------------------------------------------------------------------------

// wizardSteps are the slim `Step n/4` dot labels.
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

// hostBlockPreview is the live Host-block preview, rebuilt on every
// keystroke — written exactly like this on confirm.
func (w wizardModel) hostBlockPreview() string {
	return "Host " + w.form.sshHost() + "\n    Hostname " + w.form.hostname.Value() +
		"\n    Port " + w.form.port.Value() + "\n    User git\n    IdentityFile " + w.keyPath() +
		"\n    IdentitiesOnly yes"
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

	gitPaneForm  gitForm
	gitFocus     int
	gitCeremony  ceremonyModel
	gitExisting  bool
	deleteScope  string
	deleteCerem  ceremonyModel
	cloneInput   textinput.Model
	fixFindingID string
	fixCeremony  ceremonyModel
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
	preview := "Host " + m.editForm.host.Value() + "\n    Hostname " + m.editForm.hostname.Value() +
		"\n    Port " + m.editForm.port.Value() + "\n    User git\n    IdentityFile " + keyPath +
		"\n    IdentitiesOnly yes"
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
		m.editCeremony = m.editCeremonyFor(sel)
		m.pane = paneEditCeremony
		return keyResult{model: m, handled: true}
	case "tab", "down":
		m.editFocus = sshFieldHost + (m.editFocus-sshFieldHost+1)%3
		m.editForm = m.editForm.setFocus(m.editFocus)
		return keyResult{model: m, handled: true}
	case "shift+tab", "up":
		m.editFocus = sshFieldHost + (m.editFocus-sshFieldHost+2)%3
		m.editForm = m.editForm.setFocus(m.editFocus)
		return keyResult{model: m, handled: true}
	default:
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
		if m.gitPaneForm.valid() {
			m.gitCeremony = m.gitCeremonyFor(sel)
			m.pane = paneGitCeremony
		}
		return keyResult{model: m, handled: true}
	case "tab", "down":
		m.gitFocus = (m.gitFocus + 1) % 3
		m.gitPaneForm = m.gitPaneForm.setFocus(m.gitFocus)
		return keyResult{model: m, handled: true}
	case "shift+tab", "up":
		m.gitFocus = (m.gitFocus + 2) % 3
		m.gitPaneForm = m.gitPaneForm.setFocus(m.gitFocus)
		return keyResult{model: m, handled: true}
	default:
		m.gitPaneForm = m.gitPaneForm.handleEdit(msg, m.gitFocus)
		return keyResult{model: m, handled: true}
	}
}

// handleCloneKey drives the clone pane (no ceremony, matching the web).
func (m identitiesModel) handleCloneKey(msg tea.KeyMsg, s DemoState) keyResult {
	sel, _ := m.selectedIdentity(s)
	name := m.cloneInput.Value()
	switch msg.String() {
	case "esc":
		m.pane = paneDetail
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
		m.cloneInput, _ = updateInput(m.cloneInput, msg)
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
		case "up", "down":
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
		default:
			return keyResult{model: m, handled: true}
		}
	case 2:
		switch key {
		case "esc":
			w.step = 1
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "ctrl+s":
			// Skip — SSH only (identity stays incomplete). Ctrl-chord so
			// plain `s` still types into the author fields.
			w.configureGit = false
			w.step = 3
			w.ceremony = w.reviewCeremony()
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "enter":
			if w.git.valid() {
				w.configureGit = true
				w.step = 3
				w.ceremony = w.reviewCeremony()
			}
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "tab", "down":
			w.gitFocus = (w.gitFocus + 1) % 3
			w.git = w.git.setFocus(w.gitFocus)
			m.wizard = w
			return keyResult{model: m, handled: true}
		case "shift+tab", "up":
			w.gitFocus = (w.gitFocus + 2) % 3
			w.git = w.git.setFocus(w.gitFocus)
			m.wizard = w
			return keyResult{model: m, handled: true}
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

// handleClick implements mouseTarget: a left click on a sidebar row (either
// of its two lines) selects that identity — detail mode only; while a
// form/ceremony pane is open the sidebar is dimmed and inert, exactly like
// the keyboard model. The detail pane's controls stay keyboard-driven.
func (m identitiesModel) handleClick(x, y, width int, s DemoState) keyResult {
	if m.pane != paneDetail || x >= sidebarWidth(width) || y < sidebarLegendLines {
		return keyResult{model: m}
	}
	row := (y - sidebarLegendLines) / sidebarRowLines
	if row >= len(s.Identities) {
		return keyResult{model: m}
	}
	m.selected = s.Identities[row].Name
	return keyResult{model: m, handled: true}
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
		for i, line := range lines {
			lines[i] = styleFaint.Render(stripStyles(line))
		}
	}
	return strings.Join(lines, "\n")
}

// stripStyles drops SGR sequences (used to re-dim an already-styled line).
func stripStyles(s string) string {
	return ansi.Strip(s)
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

	b.WriteString(" " + styleFaint.Render("SSH — shown first, always") + "\n")
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

	b.WriteString("\n " + styleFaint.Render("Git") + "\n")
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
			"  " + styleBold.Render("[Configure now (g)]") + "\n")
	}
	b.WriteString(baselineStrip(s) + "\n")

	findings := FindingsFor(s, sel.Name)
	b.WriteString("\n " + styleFaint.Render(fmt.Sprintf("Findings (%d) — same data the Doctor shows (4)", len(findings))) + "\n")
	if len(findings) == 0 {
		b.WriteString("   " + styleHealthy.Render(`✓ No findings for "`+sel.Name+`".`) + "\n")
	} else {
		for _, f := range findings {
			fix := styleFaint.Render("info only")
			if f.SuggestedFix != "" {
				fix = styleBold.Render("f") + " " + styleFaint.Render("Fix…")
			}
			b.WriteString("   " + severityLabel(f.Severity) + "  " + f.Title + "  " + fix + "\n")
		}
		b.WriteString("   " + styleFocusLink.Render("Open the Doctor (4) for the global picture") + "\n")
	}
	return b.String()
}

// stepDots renders the slim `Step n/4 · <label>` dots line.
func stepDots(step int) string {
	dots := make([]string, len(wizardSteps))
	for i := range wizardSteps {
		if i == step {
			dots[i] = "●"
		} else {
			dots[i] = "○"
		}
	}
	return " " + styleFaint.Render(fmt.Sprintf("Step %d/%d · %s   %s", step+1, len(wizardSteps), wizardSteps[step], strings.Join(dots, " ")))
}

// renderWizard renders the active wizard pane-state.
func (m identitiesModel) renderWizard(s DemoState, width int) string {
	w := m.wizard
	var b strings.Builder
	b.WriteString(stepDots(w.step) + "\n")

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
			dot := "○"
			if i == w.algoIdx {
				dot = "●"
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
			b.WriteString(helperLine("gitid probes the local toolchain and disables what this machine cannot generate (KEY-03/PLAT-01). Demo simulates: no FIDO2 key plugged in.", false) + "\n")
		}
		b.WriteString(" " + PreviewLabel("Live Host-block preview — written exactly like this on confirm") + "\n")
		b.WriteString(previewBlockClipped(w.hostBlockPreview(), false, width, 6))
	case 1:
		b.WriteString(" " + styleInfo.Render("Key "+w.keyPath()+" generated ("+w.algo()+").") + "\n")
		b.WriteString(" " + styleInfo.Render("Both stages run against "+CreateFlowTestTmpConfig+" — your live ~/.ssh/config is untouched until the final confirm.") + "\n\n")

		check := "☐"
		if w.simulateFail {
			check = "☑"
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

		b.WriteString(" " + styleFaint.Render("Stage 1 — key DIRECT against the provider (TEST-01)") + "\n")
		b.WriteString(previewBlockClipped("$ "+w.stage1Cmd(), false, width, 2) + "\n")
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
			b.WriteString("\n " + styleFaint.Render("Stage 2 — resolve BY ALIAS (TEST-02). No -i here on purpose: the config must supply") + "\n")
			b.WriteString(" " + styleFaint.Render("the key; that is exactly what this stage proves.") + "\n")
			b.WriteString(previewBlockClipped("$ "+w.stage2Cmd(), false, width, 2) + "\n")
			if w.testPhase == testStage1 {
				b.WriteString(" " + styleSelected.Render(" Run stage 2 (Enter) ") + "\n")
			} else {
				b.WriteString(" " + styleHealthy.Render("✓ identityfile "+w.keyPath()) + "\n")
				b.WriteString(" " + styleSelected.Render(" Next: Git identity (Enter) ") + "\n")
			}
		}
	case 2:
		b.WriteString(w.git.view(w.form.identityName(), w.keyPath(), w.gitFocus, width, baselineStripCompact(s, width)))
		b.WriteString(" " + styleFaint.Render("Esc back") + " · " + styleBold.Render("Ctrl+S") + " " +
			styleFaint.Render("Skip — SSH only (identity stays incomplete)") + " · ")
		cont := " Continue: review & write (Enter) "
		if w.git.valid() {
			b.WriteString(styleSelected.Render(cont))
		} else {
			b.WriteString(styleFaint.Render(cont + "— disabled"))
		}
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
	detailWidth := width - sbWidth - 1

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
		crumbs = []string{"New identity"}
		actions = m.wizardFooter(s)
		status = "Esc returns to the identity detail without writing anything."
	case paneEditSSH:
		pane = " " + styleBold.Render("Edit SSH — "+sel.Name) + "\n" +
			m.editForm.view(m.editFocus, "", "") +
			"\n " + styleSelected.Render(" Rewrite Host block… (Enter) ")
		crumbs = []string{sel.Name, "Edit SSH"}
		status = "Esc returns to the identity detail without writing anything."
	case paneEditCeremony:
		pane = m.editCeremony.view(detailWidth)
		crumbs = []string{sel.Name, "Edit SSH"}
		status = "Esc returns to the identity detail without writing anything."
	case paneGit:
		suffix := " (completes this identity)"
		if m.gitExisting {
			suffix = " (editing existing fragment)"
		}
		pane = " " + styleBold.Render("Git identity — "+sel.Name+suffix) + "\n" +
			m.gitPaneForm.view(sel.Name, orDefault(sel.KeyPath, "~/.ssh/id_ed25519_"+sel.Name), m.gitFocus, detailWidth, baselineStripCompact(s, detailWidth)) +
			" " + styleSelected.Render(" Write it… (Enter) ")
		crumbs = []string{sel.Name, "Configure Git"}
		status = "Esc returns to the identity detail without writing anything."
	case paneGitCeremony:
		pane = m.gitCeremony.view(detailWidth)
		crumbs = []string{sel.Name, "Configure Git"}
		status = "Esc returns to the identity detail without writing anything."
	case paneClone:
		taken := hasIdentityNamed(s, m.cloneInput.Value())
		helper := "Creates " + m.cloneInput.Value() + ".github.com + ~/.ssh/id_ed25519_" + m.cloneInput.Value()
		if taken {
			helper = "That name already exists."
		}
		pane = " " + styleBold.Render(`Clone "`+sel.Name+`"`) + "\n" +
			" " + styleFaint.Render("The clone gets its own new key and Host alias; the Git author is copied (MGR-04).") + "\n\n" +
			formFieldLine("New identity name", m.cloneInput, true, false) + "\n" +
			helperLine(helper, taken || strings.TrimSpace(m.cloneInput.Value()) == "") + "\n\n" +
			" " + styleSelected.Render(" Clone (Enter) ")
		crumbs = []string{sel.Name, "Clone"}
		status = "Esc returns to the identity detail without writing anything."
	case paneDeleteScope:
		gitOnly := "○ "
		everything := "○ "
		if m.deleteScope == "git-only" {
			gitOnly = "● "
		} else {
			everything = "● "
		}
		pane = " " + styleBold.Render(`Delete "`+sel.Name+`" — choose scope`) + "\n\n" +
			"  " + gitOnly + IdentityManagerDeleteChoiceGitOnly + " (safer — SSH stays)\n" +
			"  " + everything + styleError.Render(IdentityManagerDeleteChoiceEverything+" — irreversible") + "\n\n" +
			" " + styleFaint.Render("↑↓ choose · Enter continue · Esc cancel")
		crumbs = []string{sel.Name, "Delete"}
		status = "Esc returns to the identity detail without writing anything."
	case paneDelete:
		pane = m.deleteCerem.view(detailWidth)
		crumbs = []string{sel.Name, "Delete"}
		status = "Esc returns to the identity detail without writing anything."
	case paneFix:
		pane = m.fixCeremony.view(detailWidth)
		crumbs = []string{sel.Name, "Fix"}
		status = "Esc returns to the identity detail without writing anything."
	}

	sidebar := m.renderSidebar(s, sbWidth, m.pane != paneDetail)
	// Word-wrap the pane at the detail width so long spec copy flows to
	// continuation lines instead of being hard-truncated by the frame.
	body = lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(sbWidth).Render(sidebar),
		" ",
		lipgloss.NewStyle().Width(detailWidth).Render(pane))
	_ = height
	return screenView{body: body, crumbs: crumbs, status: status, statusTone: "info", actions: actions}
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
			{Key: "Enter", Label: "continue: review & write"},
			{Key: "Ctrl+S", Label: "skip — SSH only"},
			{Key: "Tab/↑↓", Label: "fields"},
		}
	default:
		return nil
	}
}
