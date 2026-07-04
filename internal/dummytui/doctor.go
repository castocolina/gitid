package dummytui

// doctor.go is the Go mirror of
// .planning/design/mockup-src/src/demo/screens/Doctor.tsx per
// 02-REDESIGN-SPEC.md §5 — the Doctor absorbs the Fixer (FIX-02, no fifth
// tab). First entry auto-runs the scan; findings group `SSH ·
// <identity|global>` then `Git · …`, severity-ordered, with the LOCKED
// severity contract (~ info cyan · ! warning yellow · ✗ error AND critical
// red — the word disambiguates, NEVER ✗ for a warning). `f` fixes the
// selected finding and `F` walks EVERY fixable finding through the SAME
// per-fix ceremony with a `k / n fixed` counter — never a silent batch.
// Each success removes the finding LIVE, decrements the header chip, and
// heals identity states.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// doctorScanMsg completes the brief scanning state.
type doctorScanMsg struct{}

// doctorBatch tracks a Fix-all walk.
type doctorBatch struct {
	queue []string
	total int
}

// doctorModel is the Doctor tab child model.
type doctorModel struct {
	scanning   bool
	selectedID string
	fixing     bool
	batch      *doctorBatch
	ceremony   ceremonyModel
}

// newDoctorModel builds the Doctor tab (scan runs on first activation).
func newDoctorModel() doctorModel { return doctorModel{} }

// activate auto-runs the first scan — the view must show value
// immediately; later visits are instant.
func (m doctorModel) activate(s DemoState) (screenModel, tea.Cmd) {
	if !s.Scanned {
		m.scanning = true
		return m, tea.Tick(600*time.Millisecond, func(time.Time) tea.Msg { return doctorScanMsg{} })
	}
	m.scanning = false
	return m, nil
}

// handleMsg finishes the scan.
func (m doctorModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
	if _, ok := msg.(doctorScanMsg); ok && m.scanning {
		m.scanning = false
		return keyResult{model: m, actions: []Action{MarkScanned{}}}
	}
	return keyResult{model: m}
}

// severityRank orders findings critical > error > warning > info.
var severityRank = map[HealthSeverity]int{
	SeverityCritical: 0,
	SeverityError:    1,
	SeverityWarning:  2,
	SeverityInfo:     3,
}

// orderedFindings returns the live findings severity-sorted (stable).
func orderedFindings(s DemoState) []DemoFinding {
	out := append([]DemoFinding(nil), s.Findings...)
	sort.SliceStable(out, func(i, j int) bool {
		return severityRank[out[i].Severity] < severityRank[out[j].Severity]
	})
	return out
}

// doctorGroup is one `<Section> · <identity|global>` group.
type doctorGroup struct {
	label    string
	findings []DemoFinding
}

// groupFindings groups the ordered findings SSH-first then Git, one group
// per identity (or "global") in flat selection order.
func groupFindings(ordered []DemoFinding) []doctorGroup {
	var groups []doctorGroup
	for _, section := range []string{"SSH", "Git"} {
		var seen []string
		for _, f := range ordered {
			if f.Section != section {
				continue
			}
			id := f.Identity
			if id == "" {
				id = "global"
			}
			present := false
			for _, existing := range seen {
				if existing == id {
					present = true
				}
			}
			if !present {
				seen = append(seen, id)
			}
		}
		for _, id := range seen {
			group := doctorGroup{label: section + " · " + id}
			for _, f := range ordered {
				fid := f.Identity
				if fid == "" {
					fid = "global"
				}
				if f.Section == section && fid == id {
					group.findings = append(group.findings, f)
				}
			}
			groups = append(groups, group)
		}
	}
	return groups
}

// selectedFinding resolves the selected finding (falls back to the first).
func (m doctorModel) selectedFinding(ordered []DemoFinding) (DemoFinding, int, bool) {
	for i, f := range ordered {
		if f.ID == m.selectedID {
			return f, i, true
		}
	}
	if len(ordered) > 0 {
		return ordered[0], 0, true
	}
	return DemoFinding{}, -1, false
}

// fixableFindings filters the ordered findings that carry a suggested fix.
func fixableFindings(ordered []DemoFinding) []DemoFinding {
	var out []DemoFinding
	for _, f := range ordered {
		if f.SuggestedFix != "" {
			out = append(out, f)
		}
	}
	return out
}

// handleKey implements the Doctor key model.
func (m doctorModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	key := msg.String()
	ordered := orderedFindings(s)

	if m.fixing {
		sel, _, ok := m.selectedFinding(ordered)
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
			plan := PlanFor(sel)
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
							m.ceremony = fixCeremonyFor(f)
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
		_, idx, ok := m.selectedFinding(ordered)
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
		sel, _, ok := m.selectedFinding(ordered)
		if ok && sel.SuggestedFix != "" {
			m.selectedID = sel.ID
			m.ceremony = fixCeremonyFor(sel)
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
			m.ceremony = fixCeremonyFor(fixable[0])
			m.fixing = true
		}
		return keyResult{model: m, handled: true}
	}
	return keyResult{model: m}
}

// view implements screenModel.
func (m doctorModel) view(s DemoState, width, height int) screenView {
	_ = height
	ordered := orderedFindings(s)
	sel, selIdx, hasSel := m.selectedFinding(ordered)
	fixable := fixableFindings(ordered)

	if m.scanning {
		return screenView{
			body:   "\n " + styleFaint.Render("… running doctor scan…"),
			status: "Scanning ~/.ssh/config, ~/.gitconfig, fragments, keys, allowed_signers…",
		}
	}

	status := fmt.Sprintf("%d finding%s — Health only diagnoses; a fix runs right here, always previewed + confirmed + backed up.",
		len(ordered), pluralS(len(ordered)))
	tone := "info"
	for _, f := range ordered {
		if f.Severity != SeverityInfo {
			tone = "warning"
		}
	}

	// All green: scanned, zero findings.
	if s.Scanned && len(ordered) == 0 {
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

	listWidth := width * 44 / 100
	detailWidth := width - listWidth - 1

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
			fixNote := "info only"
			if f.SuggestedFix != "" {
				fixNote = "fixable"
			}
			rows = append(rows, truncLine(" "+marker+severityLabel(f.Severity)+" "+title, listWidth))
			rows = append(rows, truncLine("     "+styleFaint.Render(f.Family+" · "+fixNote), listWidth))
		}
	}
	list := strings.Join(rows, "\n")
	if m.fixing {
		list = styleFaint.Render(stripStyles(list))
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
		if sel.SuggestedFix != "" {
			d.WriteString(" " + styleInfo.Render("~ Suggested fix: "+sel.SuggestedFix) + "\n")
			d.WriteString(" " + styleSelected.Render(" f · Fix this… ") + "\n")
		} else {
			d.WriteString(" " + styleInfo.Render("~ Informational only — nothing to fix.") + "\n")
		}
	}
	detailPane := lipgloss.NewStyle().Width(detailWidth).Render(d.String())

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(listWidth).Render(list), " ", detailPane)
	return screenView{body: body, crumbs: crumbs, status: status, statusTone: tone, actions: actions}
}

// pluralS returns "s" for counts other than 1.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
