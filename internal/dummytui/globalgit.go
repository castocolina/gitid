package dummytui

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
	ceremonyOpen bool
	detailKey    string
	chosen       map[string]bool
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

// newGlobalGitModel mirrors GlobalGit.tsx's initial state: the
// main-vs-master row selected, every needs-action option pre-chosen EXCEPT
// the D9 global-fallback user.email row (opt-in only — recipes leave it
// unset by default).
func newGlobalGitModel() globalGitModel {
	chosen := map[string]bool{}
	for _, o := range GlobalGitOptions {
		if o.NeedsAction && o.Key != GlobalGitEmailFallbackKey {
			chosen[o.Key] = true
		}
	}
	return globalGitModel{detailKey: "init.defaultBranch", chosen: chosen, emailInput: newTextInput("")}
}

func (m globalGitModel) activate(DemoState) (screenModel, tea.Cmd) { return m, nil }
func (m globalGitModel) handleMsg(tea.Msg, DemoState) keyResult    { return keyResult{model: m} }

// overlaidGitOption is one option after the baseline-applied overlay.
type overlaidGitOption struct {
	GlobalGitOption
	applied bool
}

// overlaidGitOptions applies the baseline-applied overlay to the fixture
// options.
func overlaidGitOptions(s DemoState) []overlaidGitOption {
	out := make([]overlaidGitOption, 0, len(GlobalGitOptions))
	for _, o := range GlobalGitOptions {
		entry := overlaidGitOption{GlobalGitOption: o}
		switch {
		case o.Key == GlobalGitEmailFallbackKey:
			// review-findings F4: the email-fallback row has its OWN
			// dedicated apply ceremony and must NEVER join the generic
			// baseline-applied overlay — sweeping it in here (it also
			// carries NeedsAction: true) falsely marked it "Applied by
			// gitid" and, since `space` is gated on NeedsAction, made it
			// permanently untogglable the instant the baseline was applied.
			// Its Current instead reflects the ACTUALLY-applied
			// GitGlobalEmail (still "unset (recipes default)" until set),
			// and it stays toggleable no matter what the baseline state is.
			if s.GitGlobalEmail != "" {
				entry.Current = s.GitGlobalEmail
			}
		case s.GitBaselineApplied && o.NeedsAction:
			entry.Current = o.Recommended
			entry.NeedsAction = false
			entry.OneLiner = "Applied by gitid — " + o.OneLiner
			entry.applied = true
		}
		out = append(out, entry)
	}
	return out
}

// emailValid reports whether the D9 global-fallback field holds a plausible
// email (review-findings F10) — reusing the wizard git-form's own
// contains-@ check (gitForm.valid()) rather than inventing a second rule.
func (m globalGitModel) emailValid() bool {
	return strings.Contains(m.emailInput.Value(), "@")
}

// gitApplyChosen is the chosen ∩ pending key set, in fixture order — the
// D9 global-fallback user.email row is EXCLUDED (it has its own dedicated
// ceremony, never folded into the baseline managed-block apply).
func (m globalGitModel) gitApplyChosen(options []overlaidGitOption) []string {
	var keys []string
	for _, o := range options {
		if o.NeedsAction && o.Key != GlobalGitEmailFallbackKey && m.chosen[o.Key] {
			keys = append(keys, o.Key)
		}
	}
	return keys
}

// gitDetailIndex resolves the selected option row index.
func (m globalGitModel) gitDetailIndex(options []overlaidGitOption) int {
	for i, o := range options {
		if o.Key == m.detailKey {
			return i
		}
	}
	return 0
}

// baselineCeremony builds the apply ceremony previewing the
// sentinel-delimited managed block.
func baselineCeremony() ceremonyModel {
	return newCeremony(ceremonyConfig{
		Heading:       "Write baseline managed block to ~/.gitconfig",
		Targets:       []string{"~/.gitconfig"},
		Backups:       []string{NewBackupPath("~/.gitconfig")},
		Preview:       GlobalGitFullManagedBlockText,
		ResultMessage: GlobalGitResultMessage,
		ConfirmLabel:  "Apply baseline",
	})
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
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.ceremonyOpen = false
		case ceremonyFinished:
			m.ceremonyOpen = false
			if m.ceremony.cfg.Heading == GlobalGitEmailCeremonyHeading {
				return keyResult{model: m, handled: true,
					note:    "Global fallback user.email set — identities override it via includeIf.",
					actions: []Action{ApplyGitGlobalEmail{Email: m.emailInput.Value(), Backup: NewBackupPath("~/.gitconfig")}}}
			}
			return keyResult{model: m, handled: true,
				note:    "Global git baseline applied — user.email untouched.",
				actions: []Action{ApplyGitBaseline{Backup: NewBackupPath("~/.gitconfig")}}}
		case ceremonyNone, ceremonyConfirmed:
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

	options := overlaidGitOptions(s)
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
		if o.NeedsAction {
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
		if len(m.gitApplyChosen(options)) > 0 {
			m.ceremony = baselineCeremony()
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
	options := overlaidGitOptions(s)
	row := (y - gitTopLines(s)) / optionRowLines
	if row >= len(options) {
		return keyResult{model: m}
	}
	if options[row].NeedsAction {
		body := m.view(s, width, height).body
		if hitNeedle(body, x, y, glyphCheckOff) || hitNeedle(body, x, y, glyphCheckOn) {
			m.chosen = withToggled(m.chosen, options[row].Key)
			return keyResult{model: m, handled: true}
		}
	}
	m.detailKey = options[row].Key
	return keyResult{model: m, handled: true}
}

// view implements screenModel.
func (m globalGitModel) view(s DemoState, width, height int) screenView {
	options := overlaidGitOptions(s)
	var pending int
	for _, o := range options {
		// D9: the global-fallback user.email row is EXCLUDED from the
		// generic "baseline options not set" banner — it is a separate,
		// opt-in concern with its own detail render and ceremony.
		if o.NeedsAction && o.Key != GlobalGitEmailFallbackKey {
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
			capturesKeys: true, // the ceremony consumes every plain key
		}
	}

	listWidth := masterListWidth(width)
	detailWidth := width - listWidth - masterDetailGutter
	selIdx := m.gitDetailIndex(options)

	var rows []string
	for i, o := range options {
		marker := "  "
		if i == selIdx {
			marker = styleBold.Render("▸ ")
		}
		box := "   "
		if o.NeedsAction {
			box = glyphCheckOff + " "
			if m.chosen[o.Key] {
				box = glyphCheckOn + " "
			}
		} else if o.applied {
			box = "✓ "
		}
		toneGlyph := styleHealthy.Render("✓")
		if o.NeedsAction {
			toneGlyph = styleWarning.Render("!")
		}
		name := styleBold.Render(o.Key)
		if i == selIdx {
			name = styleSelected.Render(o.Key)
		}
		chip := ""
		if o.Highlight {
			chip = "  " + styleWarning.Render("[main vs master]")
		}
		rows = append(rows, truncLine(" "+marker+box+toneGlyph+" "+name+chip, listWidth))
		rows = append(rows, truncLine("      "+styleFaint.Render("now: "+o.Current+" → "+o.Recommended), listWidth))
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
	}
	// Wrap to the pane width, then clip with a VISIBLE cue — long option
	// explanations must never be silently cut mid-sentence (H3).
	bodyRows := frameBodyRows(height) - gitTopLines(s)
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
	actions := []FooterAction{{Key: "↑↓", Label: "select option"}, {Key: "space", Label: "toggle"}}
	if m.detailKey == GlobalGitEmailFallbackKey {
		actions = append(actions, FooterAction{Key: "Enter", Label: "edit"})
	}
	switch {
	case len(m.gitApplyChosen(options)) > 0:
		actions = append(actions, FooterAction{Key: "a", Label: fmt.Sprintf("apply %d selected", len(m.gitApplyChosen(options)))})
	case m.detailKey == GlobalGitEmailFallbackKey && m.chosen[GlobalGitEmailFallbackKey] && m.emailValid():
		actions = append(actions, FooterAction{Key: "a", Label: "set global fallback email"})
	}
	return screenView{body: body, crumbs: []string{"Options"}, status: status, statusTone: tone, actions: actions}
}
