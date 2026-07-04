package dummytui

// globalgit.go is the Go mirror of
// .planning/design/mockup-src/src/demo/screens/GlobalGit.tsx per
// 02-REDESIGN-SPEC.md §4 — GGIT-01 baseline master-detail with per-row
// apply checkboxes, the main-vs-master highlight, and a
// sentinel-preserving apply ceremony. gitid never writes a [user] section
// here — identities own their author via includeIf fragments; the
// user.email row is awareness-only, never checkable, never applied.

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// globalGitModel is the Global Git tab child model.
type globalGitModel struct {
	ceremonyOpen bool
	detailKey    string
	chosen       map[string]bool
	ceremony     ceremonyModel
}

// newGlobalGitModel mirrors GlobalGit.tsx's initial state: the
// main-vs-master row selected, every needs-action option pre-chosen.
func newGlobalGitModel() globalGitModel {
	chosen := map[string]bool{}
	for _, o := range GlobalGitOptions {
		if o.NeedsAction {
			chosen[o.Key] = true
		}
	}
	return globalGitModel{detailKey: "init.defaultBranch", chosen: chosen}
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
		if s.GitBaselineApplied && o.NeedsAction {
			entry.Current = o.Recommended
			entry.NeedsAction = false
			entry.OneLiner = "Applied by gitid — " + o.OneLiner
			entry.applied = true
		}
		out = append(out, entry)
	}
	return out
}

// gitApplyChosen is the chosen ∩ pending key set, in fixture order.
func (m globalGitModel) gitApplyChosen(options []overlaidGitOption) []string {
	var keys []string
	for _, o := range options {
		if o.NeedsAction && m.chosen[o.Key] {
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
			return keyResult{model: m, handled: true,
				note:    "Global git baseline applied — user.email untouched.",
				actions: []Action{ApplyGitBaseline{Backup: NewBackupPath("~/.gitconfig")}}}
		case ceremonyNone, ceremonyConfirmed:
		}
		return keyResult{model: m, handled: true}
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
		if o.NeedsAction { // the user.email awareness row is never checkable
			m.chosen = withToggled(m.chosen, o.Key)
		}
		return keyResult{model: m, handled: true}
	case "a":
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
		if hitNeedle(body, x, y, "☐") || hitNeedle(body, x, y, "☑") {
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
		if o.NeedsAction {
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
			actions:      []FooterAction{{Key: "Esc", Label: "cancel"}},
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
			box = "☐ "
			if m.chosen[o.Key] {
				box = "☑ "
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
	explanation := detail.OneLiner
	if detail.Key == "init.defaultBranch" {
		explanation = GlobalGitDetailExplanation
	}
	var d strings.Builder
	d.WriteString(" " + styleBold.Render(detail.Key) + "\n")
	d.WriteString(" " + styleInfo.Render("~ "+GlobalGitAdvisoryNote) + "\n\n")
	d.WriteString(" " + explanation + "\n")
	// Wrap to the pane width, then clip with a VISIBLE cue — long option
	// explanations must never be silently cut mid-sentence (H3).
	bodyRows := frameBodyRows(height) - gitTopLines(s)
	detailPane := fitPane(lipgloss.NewStyle().Width(detailWidth).Render(d.String()), bodyRows)

	body := ""
	if banner := findingsBanner(s, "Git", gitBannerBeyond); banner != "" {
		body = banner + "\n"
	}
	body += joinMasterDetail(list, listWidth, detailPane, bodyRows)

	actions := []FooterAction{{Key: "↑↓", Label: "select option"}, {Key: "space", Label: "choose"}}
	if chosen := m.gitApplyChosen(options); len(chosen) > 0 {
		actions = append(actions, FooterAction{Key: "a", Label: fmt.Sprintf("apply %d selected", len(chosen))})
	}
	return screenView{body: body, crumbs: []string{"Options"}, status: status, statusTone: tone, actions: actions}
}
