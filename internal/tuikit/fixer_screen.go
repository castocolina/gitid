package tuikit

// fixer_screen.go is the Fixer tab child model (08-01-PLAN.md Task 2's full
// split): the write half of the former single Doctor tab, scoped to
// fixableFindings(ordered) only and keeping the f/F ceremony-trigger keys —
// unchanged logic from the pre-split doctorModel, moved here verbatim and
// scoped to the fixable subset via fixableState.

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// fixerModel is the Fixer tab child model.
type fixerModel struct {
	backend    Backend
	scanning   bool
	selectedID string
	fixing     bool
	batch      *doctorBatch
	ceremony   ceremonyModel
}

// newFixerModel builds the Fixer tab (scan runs on first activation). b is
// the injected Backend seam Backend.FixPlanFor routes through (08-02-PLAN.md
// Task 2) — the real backend renders a true diff from actual file content,
// FixtureBackend delegates unchanged to the frozen free PlanFor switch.
func newFixerModel(b Backend) fixerModel { return fixerModel{backend: b} }

// fixableState returns s with Findings narrowed to fixableFindings(ordered)
// only — the Fixer tab's list scope. Every handler below operates on this
// narrowed state, never the raw App-level state, so Fixer never surfaces an
// info-only (non-fixable) finding.
func fixableState(s DemoState) DemoState {
	s.Findings = fixableFindings(orderedFindings(s))
	return s
}

// activate auto-runs the first scan — the view must show value
// immediately; later visits are instant.
func (m fixerModel) activate(s DemoState) (screenModel, tea.Cmd) {
	if !s.Scanned {
		m.scanning = true
		return m, tea.Tick(600*time.Millisecond, func(time.Time) tea.Msg { return doctorScanMsg{} })
	}
	m.scanning = false
	return m, nil
}

// handleMsg finishes the scan.
func (m fixerModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
	if _, ok := msg.(doctorScanMsg); ok && m.scanning {
		m.scanning = false
		return keyResult{model: m, actions: []Action{MarkScanned{}}}
	}
	return keyResult{model: m}
}

// handleKey implements the Fixer key model: navigate, `f` fixes the
// selected finding, `F` walks EVERY fixable finding through the SAME
// per-fix ceremony with a `k / n fixed` counter — never a silent batch.
func (m fixerModel) handleKey(msg tea.KeyMsg, rawState DemoState) keyResult {
	s := fixableState(rawState)
	key := msg.String()
	ordered := orderedFindings(s)

	if m.fixing {
		sel, _, ok := selectFinding(ordered, m.selectedID)
		if !ok {
			m.fixing = false
			m.batch = nil
			return keyResult{model: m, handled: true}
		}
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			// Esc cancels this fix AND the remainder of a Fix-all walk.
			m.fixing = false
			m.batch = nil
		case ceremonyFinished:
			plan := m.backend.FixPlanFor(sel)
			action := FixFinding{ID: sel.ID, Backup: NewBackupPath(plan.File)}
			if m.batch != nil {
				queue := m.batch.queue[:0]
				for _, id := range m.batch.queue {
					if id != sel.ID {
						queue = append(queue, id)
					}
				}
				m.batch.queue = queue
				if len(queue) > 0 {
					// Stay in fixing mode — the NEXT ceremony renders for the
					// next finding (never a silent batch).
					m.selectedID = queue[0]
					for _, f := range ordered {
						if f.ID == queue[0] {
							m.ceremony = fixCeremonyFor(m.backend, f)
						}
					}
					return keyResult{model: m, handled: true, note: plan.Result, actions: []Action{action}}
				}
				m.batch = nil
			}
			m.fixing = false
			m.selectedID = ""
			return keyResult{model: m, handled: true, note: plan.Result, actions: []Action{action}}
		case ceremonyNone, ceremonyConfirmed:
		}
		return keyResult{model: m, handled: true}
	}

	if m.scanning {
		return keyResult{model: m}
	}

	switch key {
	case "up", "down":
		_, idx, ok := selectFinding(ordered, m.selectedID)
		if !ok {
			return keyResult{model: m, handled: true}
		}
		if key == "down" && idx < len(ordered)-1 {
			idx++
		}
		if key == "up" && idx > 0 {
			idx--
		}
		m.selectedID = ordered[idx].ID
		return keyResult{model: m, handled: true}
	case "f":
		sel, _, ok := selectFinding(ordered, m.selectedID)
		if ok && sel.SuggestedFix != "" {
			m.selectedID = sel.ID
			m.ceremony = fixCeremonyFor(m.backend, sel)
			m.fixing = true
		}
		return keyResult{model: m, handled: true}
	case "F":
		fixable := fixableFindings(ordered)
		if len(fixable) > 0 {
			ids := make([]string, 0, len(fixable))
			for _, f := range fixable {
				ids = append(ids, f.ID)
			}
			m.batch = &doctorBatch{queue: ids, total: len(ids)}
			m.selectedID = ids[0]
			m.ceremony = fixCeremonyFor(m.backend, fixable[0])
			m.fixing = true
		}
		return keyResult{model: m, handled: true}
	}
	return keyResult{model: m}
}

// handleClick implements mouseTarget: a left click on a finding row (either
// of its two lines) selects that finding. It walks the same groupFindings
// layout the view renders — one group-label line, then two lines per
// finding — so hit-testing cannot drift from the drawn list. The detail
// pane's `f · Fix this…` button dispatches f, and an open fix ceremony's
// buttons click through the shared ceremony zones. Group labels and the
// scanning state are inert.
func (m fixerModel) handleClick(x, y, width, height int, rawState DemoState) keyResult {
	s := fixableState(rawState)
	if m.scanning {
		return keyResult{model: m}
	}
	if m.fixing {
		body := m.view(rawState, width, height).body
		if next, key, ok := ceremonyClickKey(m.ceremony, body, x, y); ok {
			m.ceremony = next
			return m.handleKey(key, rawState)
		}
		return keyResult{model: m}
	}
	if x >= masterListWidth(width) {
		if hitNeedle(m.view(rawState, width, height).body, x, y, " f · Fix this… ") {
			return m.handleKey(mustKey("f"), rawState)
		}
		return keyResult{model: m}
	}
	line := 0
	for _, group := range groupFindings(orderedFindings(s)) {
		line++ // the group's faint label line
		for _, f := range group.findings {
			if y == line || y == line+1 {
				m.selectedID = f.ID
				return keyResult{model: m, handled: true}
			}
			line += 2
		}
	}
	return keyResult{model: m}
}

// view implements screenModel.
func (m fixerModel) view(rawState DemoState, width, height int) screenView {
	if finding, ok := parseErrorFinding(orderedFindings(rawState)); ok {
		return parseErrorScreenView(finding)
	}
	s := fixableState(rawState)
	ordered := orderedFindings(s)
	sel, selIdx, hasSel := selectFinding(ordered, m.selectedID)
	fixable := fixableFindings(ordered)

	if m.scanning {
		return screenView{
			body:   "\n " + styleFaint.Render("… running doctor scan…"),
			status: "Scanning ~/.ssh/config, ~/.gitconfig, fragments, keys, allowed_signers…",
		}
	}

	status := fmt.Sprintf("%d fixable finding%s — every fix is previewed + confirmed + backed up before it writes.",
		len(ordered), pluralS(len(ordered)))
	tone := "info"
	for _, f := range ordered {
		if f.Severity != SeverityInfo {
			tone = "warning"
		}
	}

	// All green: scanned, zero fixable findings.
	if rawState.Scanned && len(ordered) == 0 {
		body := "\n " + styleHealthy.Render("✓ "+FixerNothingToFixSSH) + "\n " + styleHealthy.Render("✓ "+FixerNothingToFixGit)
		return screenView{body: body, status: status, statusTone: "success"}
	}

	var crumbs []string
	var actions []FooterAction
	if m.fixing && hasSel {
		crumbs = []string{"Fix", sel.Title}
		actions = []FooterAction{{Key: "Esc", Label: "cancel fix"}}
	} else {
		actions = []FooterAction{{Key: "↑↓", Label: "select finding"}}
		if hasSel && sel.SuggestedFix != "" {
			actions = append(actions, FooterAction{Key: "f", Label: "fix this"})
		}
		if len(fixable) > 1 {
			actions = append(actions, FooterAction{Key: "F", Label: fmt.Sprintf("fix all (%d)", len(fixable))})
		}
	}

	listWidth := masterListWidth(width)
	detailWidth := width - listWidth - masterDetailGutter

	var rows []string
	for _, group := range groupFindings(ordered) {
		rows = append(rows, " "+styleFaint.Render(group.label))
		for _, f := range group.findings {
			marker := "  "
			title := styleBold.Render(f.Title)
			if hasSel && f.ID == ordered[selIdx].ID {
				marker = styleBold.Render("▸ ")
				title = styleSelected.Render(f.Title)
			}
			rows = append(rows, truncLine(" "+marker+severityLabel(f.Severity)+" "+title, listWidth))
			rows = append(rows, truncLine("     "+styleFaint.Render(f.Family+" · fixable"), listWidth))
		}
	}
	list := strings.Join(rows, "\n")
	if m.fixing {
		// Same dim treatment as the Identities sidebar while a form pane is
		// open (web: opacity 0.75 during the fix ceremony, L3).
		list = dimPane(list)
	}

	var d strings.Builder
	if m.batch != nil && m.fixing {
		fixed := m.batch.total - len(m.batch.queue)
		d.WriteString(" " + styleInfo.Render(fmt.Sprintf("Fix all — %d / %d fixed; each change still previews its own diff and backup before writing.", fixed, m.batch.total)) + "\n")
	}
	if m.fixing {
		d.WriteString(m.ceremony.view(detailWidth))
	} else if hasSel {
		d.WriteString(" " + severityLabel(sel.Severity) + "  " + styleBold.Render(sel.Title) + "\n")
		chips := " " + styleFaint.Render("["+sel.Family+"]")
		if sel.Identity != "" {
			chips += " " + styleFaint.Render("["+sel.Identity+"]")
		}
		d.WriteString(chips + "\n\n")
		d.WriteString(" " + sel.Explanation + "\n\n")
		d.WriteString(" " + styleInfo.Render("~ Suggested fix: "+sel.SuggestedFix) + "\n")
		d.WriteString(" " + styleSelected.Render(" f · Fix this… ") + "\n")
	}
	// Wrap to the pane width, then clip with a VISIBLE cue — finding
	// explanations must never be silently cut mid-sentence (H3).
	bodyRows := frameBodyRows(height)
	detailPane := fitPane(lipgloss.NewStyle().Width(detailWidth).Render(d.String()), bodyRows)

	body := joinMasterDetail(list, listWidth, detailPane, bodyRows)
	return screenView{body: body, crumbs: crumbs, status: status, statusTone: tone,
		actions: actions, capturesKeys: m.fixing}
}
