package tuikit

// globalgit.go is the Go mirror of
// .planning/design/mockup-src/src/demo/screens/GlobalGit.tsx per
// 02-REDESIGN-SPEC.md §4 — GGIT-01 baseline master-detail with per-row
// apply checkboxes, the main-vs-master highlight, and a
// sentinel-preserving apply ceremony. gitid never writes user.email into
// the baseline managed block — identities own their author via includeIf
// fragments. D9 (02-DESIGN-DECISIONS-CHECKPOINT-2.md) promotes the
// user.email row from awareness-only to a first-class EDITABLE
// global-fallback field + apply checkbox (unchecked/empty by default —
// setting it is explicit opt-in), applied through its OWN dedicated
// ceremony — a DOCUMENTED, CONSCIOUS divergence from recipes/ (which leave
// it unset), with the includeIf-precedence invariant preserved: identity
// fragments always override this fallback.

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// globalGitModel is the Global Git tab child model.
type globalGitModel struct {
	// backend is the injected options/commit seam. The model fetches the
	// option states on activation and never renders fixture data for a row
	// the backend could have answered.
	backend Backend
	// detailKey is the selected option row's key.
	detailKey string
	chosen    map[string]bool
	// options is the live Options pane row set fetched from the backend on
	// activation; optionsErr carries the fetch failure the pane renders
	// instead of a blank body (GGIT-01 advisory posture — 07-UI-SPEC.md
	// RESOLVED "error" row: optionsErr rendered as an inline warning note).
	options    []GlobalGitOptionView
	optionsErr string
	// applyCommitPending gates the apply ceremony's receipt: CommitGlobalGit is
	// dispatched only from handleMsg once GlobalGitCommitMsg arrives with an
	// empty Err, never optimistically on ceremonyConfirmed (mirrors
	// identities.go's gitCommitPending and globalssh.go's applyCommitPending).
	// appliedKeys is the EXACT key set captured at ceremonyConfirmed.
	applyCommitPending bool
	appliedKeys        []string
	// fallbackCommitPending gates the D9 ceremony's receipt the same way
	// applyCommitPending gates the baseline ceremony. The two flags are
	// mutually exclusive: confirming one ceremony never dispatches the
	// other's commit method.
	fallbackCommitPending bool
	// ceremonyOpen flags the active ceremony; the D9 fallback ceremony
	// uses a separate heading check so it can never be confused with the
	// baseline one.
	ceremonyOpen bool
	ceremony     ceremonyModel
	// nameInput / emailInput are the D-04 two-field fallback pair —
	// independently editable, independently applicable. Seeded from
	// GitFallbackAuthorState on activate so "what is currently filled"
	// is always a truthful statement about the machine.
	nameInput  textinput.Model
	emailInput textinput.Model
	// fieldFocus is 0 = name, 1 = email while the fallback detail pane
	// is selected. Tab moves between them; Enter starts text-edit on
	// the focused field.
	fieldFocus int
	// fieldEditing: Enter on the selected fallback row enters text-edit
	// mode on the focused field (D1 focused rendering; every key but
	// Esc/Enter/Tab reaches the input) — this screen's single reserved-
	// letter shortcuts (space, a) would otherwise collide with typing
	// those same letters into the field; Esc/Enter exit editing back
	// to row navigation.
	fieldEditing bool
	// currentName / currentEmail are the block's contents at activate
	// time — the removal-case guard needs them independently of whatever
	// the user has since typed.
	currentName  string
	currentEmail string
}

// newGlobalGitModel returns a model with an EMPTY selection set (D-15, R-1):
// the selection starts empty on every entry to the screen, and the toggle key
// is the only way to add to it. The pre-chosen fixture set was the demo's
// scripted state, not the real default. Mirror newGlobalSSHModel's comment
// verbatim — this is 6/D-15, carried forward by 07-CONTEXT.md's <domain>
// block, and it is what makes an apply on the real binary a statement of
// intent rather than a fixture replay.
func newGlobalGitModel(b Backend) globalGitModel {
	return globalGitModel{
		backend:    b,
		detailKey:  "init.defaultBranch",
		chosen:     map[string]bool{},
		nameInput:  newTextInput(""),
		emailInput: newTextInput(""),
	}
}

// activate syncs the Options pane rows from the backend and resets the
// selection to empty on every entry so returning to the screen never
// resurrects a stale selection (D-15, R-1). A non-nil fetch error stores an
// empty slice plus an error note the pane renders instead of a blank body.
//
// This is SYNCHRONOUS — 07-UI-SPEC.md resolved the "loading" row explicitly:
// activate() calls GlobalGitOptionStates(), stores the result, and returns
// (m, nil) with no tea.Cmd, no spinner, and no intermediate state — exactly
// as globalSSHModel.activate() already behaves (cited as the precedent for
// every other Global-* tab).
func (m globalGitModel) activate(DemoState) (screenModel, tea.Cmd) {
	m.chosen = map[string]bool{}
	options, err := m.backend.GlobalGitOptionStates()
	m.options = options
	if err != nil {
		m.options = nil
		m.optionsErr = err.Error()
	}
	state, stateErr := m.backend.GitFallbackAuthorState()
	if stateErr == nil {
		m.currentName = state.Name
		m.currentEmail = state.Email
		m.nameInput = newTextInput(state.Name)
		m.emailInput = newTextInput(state.Email)
	}
	return m, nil
}

// handleMsg completes the asynchronous apply commit once the backend's command
// has answered. The receipt is reachable ONLY from an explicit success;
// reducer actions are dispatched here, never optimistically.
func (m globalGitModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
	if commit, ok := msg.(GlobalGitCommitMsg); ok && m.ceremonyOpen && m.applyCommitPending {
		m.applyCommitPending = false
		if commit.Err != "" {
			message := commit.Err
			if len(commit.Restored) > 0 {
				message += " (restored: " + strings.Join(commit.Restored, "; ") + ")"
			}
			m.ceremony = m.ceremony.commitFailed(message)
			return keyResult{model: m}
		}
		plural := "s"
		if len(m.appliedKeys) == 1 {
			plural = ""
		}
		m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
		if len(commit.Advisories) > 0 {
			m.ceremony = m.ceremony.withResultExtra(strings.Join(commit.Advisories, "\n"))
		}
		return keyResult{
			model:   m,
			note:    fmt.Sprintf("%d global git option%s applied.", len(m.appliedKeys), plural),
			actions: []Action{ApplyGitBaseline{Backup: firstBackup(commit.Backups)}},
		}
	}
	if commit, ok := msg.(GitFallbackAuthorCommitMsg); ok && m.ceremonyOpen && m.fallbackCommitPending {
		m.fallbackCommitPending = false
		if commit.Err != "" {
			message := commit.Err
			if len(commit.Restored) > 0 {
				message += " (restored: " + strings.Join(commit.Restored, "; ") + ")"
			}
			m.ceremony = m.ceremony.commitFailed(message)
			return keyResult{model: m}
		}
		m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
		if len(commit.Advisories) > 0 {
			m.ceremony = m.ceremony.withResultExtra(strings.Join(commit.Advisories, "\n"))
		}
		m.currentName = m.nameInput.Value()
		m.currentEmail = m.emailInput.Value()
		return keyResult{
			model:   m,
			note:    GlobalGitEmailResultMessage,
			actions: []Action{ApplyGitGlobalEmail{Email: m.emailInput.Value(), Name: m.nameInput.Value(), Backup: firstBackup(commit.Backups)}},
		}
	}
	return keyResult{model: m}
}

// overlaidGitOptions projects the model's LIVE backend views through the
// applied overlay: keys the user applied render as current=recommended,
// one-liner prefixed "Applied by gitid — ".
//
// Rows come from the fetched GlobalGitOptionView slice — they MUST travel
// through the seam, never around it. The fixture path lives only in
// internal/dummytui/fixturebackend.go's GlobalGitOptionStates, which is what
// the dummy binary serves through this same seam. Any value that looks like a
// fixture but is not fetched from the backend is a fixture leak: a comment
// naming the source of truth must be visible at every such site.
func (m globalGitModel) overlaidGitOptions(s DemoState) []GlobalGitOptionView {
	out := make([]GlobalGitOptionView, 0, len(m.options))
	for _, o := range m.options {
		entry := o
		if o.Key == GlobalGitEmailFallbackKey {
			// D9: the fallback row has its OWN dedicated apply ceremony
			// and must NEVER join the generic baseline overlay.
			if s.GitGlobalName != "" || s.GitGlobalEmail != "" {
				entry.CurrentValue = fallbackCurrentLabel(s.GitGlobalName, s.GitGlobalEmail)
			}
			out = append(out, entry)
			continue
		}
		// review-findings F4 precedent (carried from the original demo):
		// the overlay sweeps every still-NeedsAction row to applied once the
		// single GitBaselineApplied flag flips, mirroring the coarse
		// single-shot semantics this screen has always had — there is no
		// per-key applied list on DemoState (unlike SSHApplied), because
		// the real backend re-probes on the next activate() and this
		// overlay only needs to simulate "just applied" for the STUB/dummy
		// path between a commit and the next activation.
		if s.GitBaselineApplied && o.State == GlobalGitNeedsAction {
			entry.CurrentValue = o.Recommended
			entry.State = GlobalGitAlreadySet
			entry.OneLiner = "Applied by gitid — " + o.OneLiner
		}
		out = append(out, entry)
	}
	return out
}

// gitNeedsAttention is the ONE tally predicate for the Global Git options
// pane (D-10, pinned in 07-03-PLAN.md's <authority> block). It counts a ROW
// at most once, and counts only rows in the needs-action state — a row whose
// State is set-but-differs is EXCLUDED (D-02: under the floor + last-wins
// model, a write into a key the user set later is provably a no-op, so the
// row has no action to offer), not-applicable rows are excluded, and a bundle
// row counts once when at least one member key is unset, whatever the other
// members do. The fallback-author row is excluded because it carries no
// writable member keys — it has its own dedicated ceremony (D-05).
//
// The list's status line and the apply ceremony's count BOTH read this one
// predicate; nothing re-derives a second count.
//
// This DIVERGES from Phase 6's own tally (06-03-SUMMARY.md), which counts
// set-but-differs rows as needing attention. The divergence is deliberate and
// asymmetric because the precedence arrow is reversed: on the SSH side gitid's
// `Host *` block can genuinely change a differing value, on the git side it
// provably cannot. Cite both D-02 and 06-03's own rule so a future reader
// sees the asymmetry is intended.
func gitNeedsAttention(o GlobalGitOptionView) bool {
	return o.Selectable()
}

// gitApplyChosen is the chosen ∩ selectable key set, in row order. The
// selectable predicate (needs-action + writable member keys + no probe error)
// is what excludes the differs, not-applicable and fallback-author rows — there
// is no per-key exclusion list and no second counting loop here.
func (m globalGitModel) gitApplyChosen(options []GlobalGitOptionView) []string {
	var keys []string
	for _, o := range options {
		if o.Selectable() && m.chosen[o.Key] {
			keys = append(keys, o.Key)
		}
	}
	return keys
}

// globalGitRowLine2 renders the option row's second line: "now: <current> →
// <recommended>" plus the state WORD. The D-02 differs vocabulary deliberately
// reuses Phase 6's own frozen sentences (GlobalSSHWordDiffersUser/Outside) —
// one vocabulary across both Global-* screens, already inside the copy-freeze
// gate. Not-applicable rows replace the now-line with their reason sentence.
func globalGitRowLine2(o GlobalGitOptionView) string {
	if o.State == GlobalGitNotApplicable {
		return globalGitNotApplicableSentence(o.NotApplicableReason)
	}
	now := "now: " + o.CurrentValue + " → " + o.Recommended
	switch o.State {
	case GlobalGitSetButDiffers:
		if o.AttributedToUser {
			return now + "  " + GlobalSSHWordDiffersUser
		}
		return now + "  " + GlobalSSHWordDiffersOutside
	case GlobalGitAlreadySet:
		if o.CurrentValue != "" {
			return now + "  " + GlobalSSHWordAlreadySet
		}
	}
	return now
}

// globalGitNotApplicableSentence renders the per-reason not-applicable line.
func globalGitNotApplicableSentence(r GlobalGitNotApplicableReason) string {
	switch r {
	case GlobalGitReasonProbeFailed:
		// Byte-identical to Phase 6's own probe-failed sentence (already in
		// the freeze gate) — one vocabulary across both Global-* screens.
		return GlobalSSHNAProbeFailed
	default:
		return GlobalSSHNAProbeFailed
	}
}

// gitDetailIndex resolves the selected option row index.
func (m globalGitModel) gitDetailIndex(options []GlobalGitOptionView) int {
	for i, o := range options {
		if o.Key == m.detailKey {
			return i
		}
	}
	return 0
}

func fallbackCurrentLabel(name, email string) string {
	switch {
	case name != "" && email != "":
		return name + " <" + email + ">"
	case name != "":
		return name
	case email != "":
		return email
	default:
		return "unset (recipes default)"
	}
}

// emailValid reports whether the D9 email field is empty (unset, valid) or
// holds a plausible email — reusing the wizard git-form's own contains-@
// check rather than inventing a second rule. A non-empty malformed email
// is the only invalid state.
func (m globalGitModel) emailValid() bool {
	email := m.emailInput.Value()
	return email == "" || strings.Contains(email, "@")
}

// fallbackApplyOffered is the D-04 / 07-UI-SPEC.md resolved "partial" row
// apply guard. Pressing `a` applies a PAIR SNAPSHOT of whatever the two
// seeded fields currently hold, in ONE ceremony instance. Clearing one
// field and applying also re-asserts the other's seeded value — that is
// correct for a TUI whose fields are visible and pre-filled. Plan 07-05's
// CLI uses explicit set/clear flags because a CLI has no screen, so an
// omitted flag must mean "leave it alone" rather than "clear it". Both
// call the SAME EnsureGitFallbackAuthor with a resolved pair; do not
// "fix" the TUI into per-field flags or the CLI into a snapshot.
//
// Offer the action when the email is empty or valid, AND at least one of
// these is true: a field is non-empty, or the machine's current block is
// non-empty (the removal case — the one place a naive "both empty means
// nothing to do" guard is wrong, and exactly how the user clears a
// fallback they previously set). Do not offer it when the email is
// non-empty and malformed, or when both fields are empty and the current
// block is empty.
func (m globalGitModel) fallbackApplyOffered() bool {
	if !m.emailValid() {
		return false
	}
	name := m.nameInput.Value()
	email := m.emailInput.Value()
	if name != "" || email != "" {
		return true
	}
	return m.currentName != "" || m.currentEmail != ""
}

// baselineCeremonyFor builds the apply ceremony using the plan view returned
// by the backend: heading, targets, backups, and preview all come from
// GlobalGitApplyPlan. The heading is the frozen prefix plus Targets[0] —
// the resolved include'd baseline path — which is the D-01 allowlisted
// divergence from the fixture's hardcoded main-config path. The ceremony is
// ASYNC: confirmation dispatches the backend commit and the receipt is
// reachable only from that commit's explicit success (ceremony.go's Async
// contract), exactly like the global-SSH apply ceremony.
func (m globalGitModel) baselineCeremonyFor(keys []string) (ceremonyModel, error) {
	plan, planErr := m.backend.GlobalGitApplyPlan(keys)
	if planErr != nil {
		return ceremonyModel{}, planErr
	}
	targets := plan.Targets
	if len(targets) == 0 {
		// The backend's plan always resolves the baseline target (D-01); this
		// fallback keeps the ceremony buildable for a stub that answers an empty plan.
		targets = []string{"~/.gitconfig.d/00-baseline"}
	}
	backups := plan.Backups
	if len(backups) == 0 {
		backups = []string{NewBackupPath(targets[0])}
	}
	preview := plan.Diff
	if preview == "" {
		preview = GlobalGitFullManagedBlockText
	}
	return newCeremony(ceremonyConfig{
		Heading:       "Write global-git managed block to " + targets[0],
		Targets:       targets,
		Backups:       backups,
		Preview:       preview,
		ResultMessage: GlobalGitResultMessage,
		ConfirmLabel:  "Apply selected",
		Async:         true,
	}), nil
}

// fallbackCeremonyFor builds the D9 dedicated apply ceremony from the
// planner's plan view — heading, targets, backups, and preview all come
// from GitFallbackAuthorPlan so the ceremony names the resolved file and
// the real promised backup. It is a SECOND, independent ceremony instance:
// the baseline ceremony branch is untouched and neither branch can reach
// the other's commit method.
func (m globalGitModel) fallbackCeremonyFor(name, email string) (ceremonyModel, error) {
	plan, planErr := m.backend.GitFallbackAuthorPlan(name, email)
	if planErr != nil {
		return ceremonyModel{}, planErr
	}
	targets := plan.Targets
	if len(targets) == 0 {
		targets = []string{"~/.gitconfig"}
	}
	backups := plan.Backups
	if len(backups) == 0 && !plan.Removal {
		backups = []string{NewBackupPath(targets[0])}
	}
	preview := plan.Diff
	if preview == "" {
		preview = fallbackPreview(name, email)
	}
	heading := GlobalGitEmailCeremonyHeading
	if plan.Removal {
		heading = "Remove global fallback author"
	}
	return newCeremony(ceremonyConfig{
		Heading:       heading,
		Targets:       targets,
		Backups:       backups,
		Preview:       preview,
		ResultMessage: GlobalGitEmailResultMessage,
		ConfirmLabel:  "Apply",
		Async:         true,
	}), nil
}

func fallbackPreview(name, email string) string {
	var b strings.Builder
	b.WriteString("+ [user]\n")
	if name != "" {
		b.WriteString("+     name = " + name + "  " + GlobalGitEmailDiffAnnotation + "\n")
	}
	if email != "" {
		b.WriteString("+     email = " + email + "  " + GlobalGitEmailDiffAnnotation + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// handleKey implements the Global Git key model.
func (m globalGitModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	key := msg.String()

	if m.ceremonyOpen {
		if m.applyCommitPending || m.fallbackCommitPending {
			// In flight: keys are inert until the commit result arrives.
			return keyResult{model: m, handled: true}
		}
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.ceremonyOpen = false
		case ceremonyConfirmed:
			if m.ceremony.cfg.Heading == GlobalGitEmailCeremonyHeading ||
				strings.HasPrefix(m.ceremony.cfg.Heading, "Remove global fallback") {
				m.fallbackCommitPending = true
				return keyResult{model: m, handled: true, cmd: m.backend.CommitGitFallbackAuthor(m.nameInput.Value(), m.emailInput.Value())}
			}
			keys := m.gitApplyChosen(m.overlaidGitOptions(s))
			m.appliedKeys = keys
			m.applyCommitPending = true
			return keyResult{model: m, handled: true, cmd: m.backend.CommitGlobalGit(keys)}
		case ceremonyFinished:
			m.ceremonyOpen = false
		case ceremonyNone:
		}
		return keyResult{model: m, handled: true}
	}

	// D9: while text-editing a fallback field, every key but Esc/Enter/Tab
	// reaches the input — this screen's single-letter shortcuts (space, a)
	// would otherwise collide with typing those same letters.
	if m.fieldEditing {
		switch key {
		case "esc", "enter":
			m.fieldEditing = false
			m.nameInput.Blur()
			m.emailInput.Blur()
			return keyResult{model: m, handled: true}
		case "tab":
			m.fieldFocus = 1 - m.fieldFocus
			m.focusFallbackField()
			return keyResult{model: m, handled: true}
		default:
			if m.fieldFocus == 0 {
				m.nameInput, _ = updateInput(m.nameInput, msg)
			} else {
				m.emailInput, _ = updateInput(m.emailInput, msg)
			}
			return keyResult{model: m, handled: true}
		}
	}

	options := m.overlaidGitOptions(s)
	if len(options) == 0 {
		return keyResult{model: m, handled: true}
	}
	switch key {
	case "up", "down":
		idx := m.gitDetailIndex(options)
		if key == "down" && idx < len(options)-1 {
			idx++
		}
		if key == "up" && idx > 0 {
			idx--
		}
		m.detailKey = options[idx].Key
		return keyResult{model: m, handled: true}
	case "space":
		o := options[m.gitDetailIndex(options)]
		// Selectable() is the ONE predicate: a row that cannot be toggled
		// (including the fallback-author row, which carries no writable member
		// keys under D-04/D-05) is refused here, renders no checkbox glyph, and
		// clicks at its checkbox position do nothing — there is no path by
		// which the three can disagree.
		if o.Selectable() {
			m.chosen = withToggled(m.chosen, o.Key)
		}
		return keyResult{model: m, handled: true}
	case "tab":
		if m.detailKey == GlobalGitEmailFallbackKey {
			m.fieldFocus = 1 - m.fieldFocus
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m}
	case "enter":
		// D9/D8: Enter on the selected fallback row starts text-editing
		// the focused field.
		if m.detailKey == GlobalGitEmailFallbackKey {
			m.fieldEditing = true
			m.focusFallbackField()
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m}
	case "a":
		if m.detailKey == GlobalGitEmailFallbackKey && m.fallbackApplyOffered() {
			cer, cerErr := m.fallbackCeremonyFor(m.nameInput.Value(), m.emailInput.Value())
			if cerErr != nil {
				m.optionsErr = cerErr.Error()
				return keyResult{model: m, handled: true}
			}
			m.ceremony = cer
			m.ceremonyOpen = true
			return keyResult{model: m, handled: true}
		}
		chosen := m.gitApplyChosen(options)
		if len(chosen) > 0 {
			cer, cerErr := m.baselineCeremonyFor(chosen)
			if cerErr != nil {
				m.optionsErr = cerErr.Error()
				return keyResult{model: m, handled: true}
			}
			m.ceremony = cer
			m.ceremonyOpen = true
		}
		return keyResult{model: m, handled: true}
	}
	return keyResult{model: m}
}

// gitBannerBeyond is the findingsBanner tail for this screen — shared by
// view and gitTopLines.
const gitBannerBeyond = "this baseline"

// gitTopLines counts the body lines rendered above the first option row
// (the optional findings banner) — shared by view and handleClick.
func gitTopLines(s DemoState) int {
	if findingsBanner(s, "Git", gitBannerBeyond) != "" {
		return 1
	}
	return 0
}

// handleClick implements mouseTarget: a click on an option row's checkbox
// glyph TOGGLES it like space (GlobalGit.tsx:127 — Checkbox onClick stops
// propagation), a click elsewhere in the row selects it, and the ceremony's
// buttons click through the shared ceremony zones. The banner and the
// detail pane are inert.
func (m globalGitModel) handleClick(x, y, width, height int, s DemoState) keyResult {
	if m.ceremonyOpen {
		body := m.view(s, width, height).body
		if next, key, ok := ceremonyClickKey(m.ceremony, body, x, y); ok {
			m.ceremony = next
			return m.handleKey(key, s)
		}
		return keyResult{model: m}
	}
	if x >= masterListWidth(width) || y < gitTopLines(s) {
		return keyResult{model: m}
	}
	options := m.overlaidGitOptions(s)
	row := (y - gitTopLines(s)) / optionRowLines
	if row >= len(options) {
		return keyResult{model: m}
	}
	o := options[row]
	// Checkbox hit-test: only selectable rows carry a checkbox — the same ONE
	// predicate that gates the toggle key and the checkbox glyph (D-02, D-05).
	if o.Selectable() {
		body := m.view(s, width, height).body
		if hitNeedle(body, x, y, glyphCheckOff) || hitNeedle(body, x, y, glyphCheckOn) {
			m.chosen = withToggled(m.chosen, o.Key)
			return keyResult{model: m, handled: true}
		}
	}
	m.detailKey = options[row].Key
	return keyResult{model: m, handled: true}
}

// view implements screenModel.
func (m globalGitModel) view(s DemoState, width, height int) screenView {
	options := m.overlaidGitOptions(s)

	var pending int
	for _, o := range options {
		if gitNeedsAttention(o) {
			pending++
		}
	}

	status := "Baseline applied. user.email stays untouched — identities own their author."
	tone := "info"
	if pending > 0 {
		status = fmt.Sprintf("%d baseline options not set — %s", pending, GlobalGitAdvisoryNote)
		tone = "warning"
	}

	if m.ceremonyOpen {
		return screenView{
			body:         m.ceremony.view(width - 2),
			crumbs:       []string{"Options"},
			status:       status,
			actions:      ceremonyFooterActions(),
			capturesKeys: true,
		}
	}

	if m.optionsErr != "" {
		body := ""
		if banner := findingsBanner(s, "Git", gitBannerBeyond); banner != "" {
			body = banner + "\n"
		}
		body += " " + styleWarning.Render("! "+m.optionsErr) + "\n\n " +
			styleFaint.Render("The option states could not be read from this machine.")
		return screenView{
			body:    body,
			crumbs:  []string{"Options"},
			status:  status,
			actions: []FooterAction{{Key: "↑↓", Label: "select option"}},
		}
	}

	listWidth := masterListWidth(width)
	detailWidth := width - listWidth - masterDetailGutter
	selIdx := m.gitDetailIndex(options)
	bodyRows := frameBodyRows(height) - gitTopLines(s)

	var rows []string
	for i, o := range options {
		marker := "  "
		if i == selIdx {
			marker = styleBold.Render("▸ ")
		}
		// Checkbox: only selectable rows get a checkbox glyph — driven by the
		// ONE Selectable predicate (needs-action + writable member keys + no
		// probe error), so a does/render mismatch cannot exist (D-02, D-05).
		box := "   "
		if o.Selectable() {
			box = glyphCheckOff + " "
			if m.chosen[o.Key] {
				box = glyphCheckOn + " "
			}
		}
		toneGlyph := styleHealthy.Render("✓")
		switch o.State {
		case GlobalGitNeedsAction, GlobalGitSetButDiffers:
			toneGlyph = styleWarning.Render("!")
		case GlobalGitNotApplicable:
			// A neutral marker, not a health-tone glyph (mirrors globalssh):
			// every other row carries a visible tone glyph, so a blank cell
			// here would read as a missing/broken row.
			toneGlyph = styleFaint.Render("·")
		}
		name := styleBold.Render(o.Key)
		if i == selIdx {
			name = styleSelected.Render(o.Key)
		}
		chip := ""
		if o.Key == "init.defaultBranch" {
			chip = "  " + styleWarning.Render("[main vs master]")
		}
		rows = append(rows, truncLine(" "+marker+box+toneGlyph+" "+name+chip, listWidth))
		rows = append(rows, truncLine("      "+styleFaint.Render(globalGitRowLine2(o)), listWidth))
	}
	list := strings.Join(rows, "\n")

	detail := options[selIdx]
	var d strings.Builder
	if detail.Key == GlobalGitEmailFallbackKey {
		nameFocused := m.fieldFocus == 0 && m.fieldEditing
		emailFocused := m.fieldFocus == 1 && m.fieldEditing
		d.WriteString(formFieldLine(GlobalGitNameFallbackKey, m.nameInput, nameFocused, false) + "\n")
		d.WriteString(formFieldLine(GlobalGitEmailFallbackKey, m.emailInput, emailFocused, false))
		if !m.emailValid() {
			d.WriteString("  " + styleError.Render("needs @"))
		}
		d.WriteString("\n")
		d.WriteString(helperLine(GlobalGitEmailFallbackHelper, false) + "\n")
		d.WriteString(helperLine(GlobalGitEmailFallbackAdvisory, false) + "\n")
	} else {
		explanation := detail.OneLiner
		if detail.Key == "init.defaultBranch" {
			explanation = GlobalGitDetailExplanation
		}
		d.WriteString(" " + styleBold.Render(detail.Key) + "\n")
		d.WriteString(" " + styleInfo.Render("~ "+GlobalGitAdvisoryNote) + "\n\n")
		d.WriteString(" " + explanation + "\n")
		for _, note := range detail.BundlePerKeyNotes {
			// Per-key "yours differs — yours wins" notes for a bundle row's
			// members the user set differently (D-09).
			d.WriteString(" " + styleFaint.Render(note) + "\n")
		}
		if detail.VersionNote != "" {
			// The NON-contractual dynamic version line (D-13 precedent) —
			// excluded from the copy-freeze gate.
			d.WriteString(" " + styleFaint.Render(" "+detail.VersionNote) + "\n")
		}
		if detail.Provenance != "" {
			d.WriteString(" " + styleFaint.Render(detail.Provenance) + "\n")
		}
		if detail.ProbeError != "" {
			d.WriteString(" " + styleWarning.Render("! "+detail.ProbeError) + "\n")
		}
	}
	// Wrap to the pane width, then clip with a VISIBLE cue — long option
	// explanations must never be silently cut mid-sentence (H3).
	detailPane := fitPane(lipgloss.NewStyle().Width(detailWidth).Render(d.String()), bodyRows)

	body := ""
	if banner := findingsBanner(s, "Git", gitBannerBeyond); banner != "" {
		body = banner + "\n"
	}
	body += joinMasterDetail(list, listWidth, detailPane, bodyRows)

	if m.fieldEditing {
		return screenView{body: body, crumbs: []string{"Options"}, status: status, statusTone: tone,
			actions:      []FooterAction{{Key: "Esc/Enter", Label: "done editing"}, {Key: "Tab", Label: "next field"}},
			capturesKeys: true}
	}
	chosen := m.gitApplyChosen(options)
	actions := []FooterAction{{Key: "↑↓", Label: "select option"}, {Key: "space", Label: "toggle"}}
	if m.detailKey == GlobalGitEmailFallbackKey {
		actions = []FooterAction{{Key: "↑↓", Label: "select option"}, {Key: "Tab", Label: "next field"}, {Key: "Enter", Label: "edit"}}
	}
	switch {
	case len(chosen) > 0:
		actions = append(actions, FooterAction{Key: "a", Label: fmt.Sprintf("apply %d selected", len(chosen))})
	case m.detailKey == GlobalGitEmailFallbackKey && m.fallbackApplyOffered():
		actions = append(actions, FooterAction{Key: "a", Label: "set global fallback author"})
	}
	return screenView{body: body, crumbs: []string{"Options"}, status: status, statusTone: tone, actions: actions}
}

func (m *globalGitModel) focusFallbackField() {
	if m.fieldFocus == 0 {
		m.nameInput.Focus()
		m.emailInput.Blur()
		return
	}
	m.nameInput.Blur()
	m.emailInput.Focus()
}
