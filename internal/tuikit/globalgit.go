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
	// ceremonyOpen flags the active ceremony; D9 email ceremony uses a
	// separate heading check.
	ceremonyOpen bool
	ceremony     ceremonyModel
	// emailInput is the D9 editable global-fallback user.email field —
	// unset/empty by default (recipes default preserved; setting it is
	// explicit opt-in).
	emailInput textinput.Model
	// emailEditing: Enter on the selected fallback row enters text-edit
	// mode (D1 focused rendering; every key but Esc/Enter reaches the
	// input) — this screen's single reserved-letter shortcuts (space, a)
	// would otherwise collide with typing those same letters into the
	// field; Esc/Enter exit editing back to row navigation.
	emailEditing bool
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
			// D9: the email-fallback row has its OWN dedicated apply ceremony
			// and must NEVER join the generic baseline overlay — see globalssh.go
			// for the structural reason (the row stays independently selectable
			// regardless of the baseline state, per review-findings F4 precedent).
			if s.GitGlobalEmail != "" {
				entry.CurrentValue = s.GitGlobalEmail
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

// gitApplyChosen is the chosen ∩ selectable key set, in row order — the
// D9 global-fallback user.email row is EXCLUDED (it has its own dedicated
// ceremony, never folded into the baseline managed-block apply).
func (m globalGitModel) gitApplyChosen(options []GlobalGitOptionView) []string {
	var keys []string
	for _, o := range options {
		if o.Key != GlobalGitEmailFallbackKey && o.Selectable() && o.State == GlobalGitNeedsAction && m.chosen[o.Key] {
			keys = append(keys, o.Key)
		}
	}
	return keys
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

// emailValid reports whether the D9 global-fallback field holds a plausible
// email (review-findings F10) — reusing the wizard git-form's own
// contains-@ check (gitForm.valid()) rather than inventing a second rule.
func (m globalGitModel) emailValid() bool {
	return strings.Contains(m.emailInput.Value(), "@")
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

// emailCeremonyFor builds the D9 dedicated apply ceremony for the
// global-fallback user.email — separate from the baseline managed-block
// ceremony (its own heading/target/annotated diff/result), because gitid
// NEVER folds a fallback author into the baseline managed block.
func emailCeremonyFor(email string) ceremonyModel {
	return newCeremony(ceremonyConfig{
		Heading:       GlobalGitEmailCeremonyHeading,
		Targets:       []string{"~/.gitconfig"},
		Backups:       []string{NewBackupPath("~/.gitconfig")},
		Preview:       "+ [user]\n+     email = " + email + "  " + GlobalGitEmailDiffAnnotation,
		ResultMessage: GlobalGitEmailResultMessage,
		ConfirmLabel:  "Apply",
	})
}

// handleKey implements the Global Git key model.
func (m globalGitModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	key := msg.String()

	if m.ceremonyOpen {
		if m.applyCommitPending {
			// In flight: keys are inert until the commit result arrives.
			return keyResult{model: m, handled: true}
		}
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.ceremonyOpen = false
		case ceremonyConfirmed:
			if m.ceremony.cfg.Heading == GlobalGitEmailCeremonyHeading {
				// D9 email ceremony — SYNCHRONOUS (no backend seam yet;
				// plan 07-02 owns the real D9 write seam). Confirming only
				// transitions the ceremony to its receipt state (state B,
				// rendered internally by ceremony.go with the frozen
				// GlobalGitEmailResultMessage) — the state mutation and
				// ceremonyOpen close happen on the FOLLOWING
				// ceremonyFinished, matching the two-step confirm→dismiss
				// contract every synchronous ceremony in this codebase
				// follows.
				return keyResult{model: m, handled: true}
			}
			// Baseline apply ceremony — dispatch the async commit.
			keys := m.gitApplyChosen(m.overlaidGitOptions(s))
			m.appliedKeys = keys
			m.applyCommitPending = true
			return keyResult{model: m, handled: true, cmd: m.backend.CommitGlobalGit(keys)}
		case ceremonyFinished:
			m.ceremonyOpen = false
			if m.ceremony.cfg.Heading == GlobalGitEmailCeremonyHeading {
				return keyResult{model: m, handled: true,
					note:    "Global fallback user.email set — identities override it via includeIf.",
					actions: []Action{ApplyGitGlobalEmail{Email: m.emailInput.Value(), Backup: NewBackupPath("~/.gitconfig")}}}
			}
		case ceremonyNone:
		}
		return keyResult{model: m, handled: true}
	}

	// D9: while text-editing the fallback field, every key but Esc/Enter
	// reaches the input — this screen's single-letter shortcuts (space, a)
	// would otherwise collide with typing those same letters.
	if m.emailEditing {
		switch key {
		case "esc", "enter":
			m.emailEditing = false
			m.emailInput.Blur()
			return keyResult{model: m, handled: true}
		default:
			m.emailInput, _ = updateInput(m.emailInput, msg)
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
		if o.Key == GlobalGitEmailFallbackKey {
			// D9: the email-fallback row uses the same space toggle but is
			// never gated by the policy table — it is always opt-in selectable.
			m.chosen = withToggled(m.chosen, o.Key)
			return keyResult{model: m, handled: true}
		}
		if o.Selectable() {
			m.chosen = withToggled(m.chosen, o.Key)
		}
		return keyResult{model: m, handled: true}
	case "enter":
		// D9/D8: Enter on the selected fallback row starts text-editing.
		if m.detailKey == GlobalGitEmailFallbackKey {
			m.emailEditing = true
			m.emailInput.Focus()
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m}
	case "a":
		// D9: the global-fallback checkbox, when chosen, applies through
		// its OWN dedicated ceremony — never folded into the baseline.
		// review-findings F10: gate the apply on a plausible email (reusing
		// the wizard's contains-@ check) — an empty/invalid fallback email
		// must never be applicable.
		if m.detailKey == GlobalGitEmailFallbackKey && m.chosen[GlobalGitEmailFallbackKey] && m.emailValid() {
			m.ceremony = emailCeremonyFor(m.emailInput.Value())
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
	// Checkbox hit-test: only rows whose key the policy table resolves carry a
	// checkbox — the same Selectable predicate that gates the toggle key.
	// D9 email-fallback row is always checkboxable (independent of policy).
	checkboxable := o.Selectable() || o.Key == GlobalGitEmailFallbackKey
	if checkboxable {
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
		if o.Key != GlobalGitEmailFallbackKey && o.Selectable() && o.State == GlobalGitNeedsAction {
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
		// Checkbox: only policy-backed rows get a checkbox glyph. The D9
		// email-fallback row is always independently toggleable.
		// Rows with no policy entry render a blank placeholder — honest
		// "not yet actionable" rendering indistinguishable structurally
		// from later not-applicable states.
		box := "   "
		if o.Key == GlobalGitEmailFallbackKey {
			box = glyphCheckOff + " "
			if m.chosen[o.Key] {
				box = glyphCheckOn + " "
			}
		} else if o.Selectable() {
			box = glyphCheckOff + " "
			if m.chosen[o.Key] {
				box = glyphCheckOn + " "
			}
		}
		toneGlyph := styleHealthy.Render("✓")
		switch o.State {
		case GlobalGitNeedsAction, GlobalGitSetButDiffers:
			toneGlyph = styleWarning.Render("!")
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
		rows = append(rows, truncLine("      "+styleFaint.Render("now: "+o.CurrentValue+" → "+o.Recommended), listWidth))
	}
	list := strings.Join(rows, "\n")

	detail := options[selIdx]
	var d strings.Builder
	if detail.Key == GlobalGitEmailFallbackKey {
		// D9: the promoted, editable global-fallback row — D1 single-row
		// field template + apply checkbox, its own always-visible
		// helper/advisory lines (byte-exact, verbatim).
		d.WriteString(formFieldLine(GlobalGitEmailFallbackKey, m.emailInput, m.emailEditing, false))
		// review-findings F10: the same "needs @" inline-error idiom the
		// wizard's Git-step user.email field already carries (gitForm.view)
		// — gates the apply action on a plausible email.
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

	if m.emailEditing {
		return screenView{body: body, crumbs: []string{"Options"}, status: status, statusTone: tone,
			actions:      []FooterAction{{Key: "Esc/Enter", Label: "done editing"}},
			capturesKeys: true}
	}
	chosen := m.gitApplyChosen(options)
	actions := []FooterAction{{Key: "↑↓", Label: "select option"}, {Key: "space", Label: "toggle"}}
	if m.detailKey == GlobalGitEmailFallbackKey {
		actions = append(actions, FooterAction{Key: "Enter", Label: "edit"})
	}
	switch {
	case len(chosen) > 0:
		actions = append(actions, FooterAction{Key: "a", Label: fmt.Sprintf("apply %d selected", len(chosen))})
	case m.detailKey == GlobalGitEmailFallbackKey && m.chosen[GlobalGitEmailFallbackKey] && m.emailValid():
		actions = append(actions, FooterAction{Key: "a", Label: "set global fallback email"})
	}
	return screenView{body: body, crumbs: []string{"Options"}, status: status, statusTone: tone, actions: actions}
}
