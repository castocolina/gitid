package tuikit

// health_screen.go is the Health tab child model (08-01-PLAN.md Task 2's
// full split): the READ-ONLY half of the former single Doctor tab. It lists
// ALL findings (not just fixable ones) and carries no f/F ceremony-trigger
// keys and no "Fix this…" action line — Health only diagnoses; a fix is
// applied on the Fixer tab.

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// healthModel is the Health tab child model.
type healthModel struct {
	scanning   bool
	selectedID string
}

// newHealthModel builds the Health tab (scan runs on first activation).
func newHealthModel() healthModel { return healthModel{} }

// activate auto-runs the first scan — the view must show value
// immediately; later visits are instant.
func (m healthModel) activate(s DemoState) (screenModel, tea.Cmd) {
	if !s.Scanned {
		m.scanning = true
		return m, tea.Tick(600*time.Millisecond, func(time.Time) tea.Msg { return doctorScanMsg{} })
	}
	m.scanning = false
	return m, nil
}

// handleMsg finishes the scan.
func (m healthModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
	if _, ok := msg.(doctorScanMsg); ok && m.scanning {
		m.scanning = false
		return keyResult{model: m, actions: []Action{MarkScanned{}}}
	}
	return keyResult{model: m}
}

// handleKey implements the Health key model: navigation only — no f/F.
func (m healthModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	key := msg.String()
	ordered := orderedFindings(s)

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
	}
	return keyResult{model: m}
}

// handleClick implements mouseTarget: a left click on a finding row (either
// of its two lines) selects that finding. It walks the same groupFindings
// layout the view renders — one group-label line, then two lines per
// finding — so hit-testing cannot drift from the drawn list. The detail
// pane carries no clickable fix affordance (read-only) and the scanning
// state is inert.
func (m healthModel) handleClick(x, y, width, _ int, s DemoState) keyResult {
	if m.scanning {
		return keyResult{model: m}
	}
	if x >= masterListWidth(width) {
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
func (m healthModel) view(s DemoState, width, height int) screenView {
	ordered := orderedFindings(s)
	sel, selIdx, hasSel := selectFinding(ordered, m.selectedID)

	if m.scanning {
		return screenView{
			body:   "\n " + styleFaint.Render("… running doctor scan…"),
			status: "Scanning ~/.ssh/config, ~/.gitconfig, fragments, keys, allowed_signers…",
		}
	}

	status := fmt.Sprintf("%d finding%s — read-only diagnostics; switch to Fixer to apply a fix.",
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

	actions := []FooterAction{{Key: "↑↓", Label: "select finding"}}

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
			fixNote := "info only"
			if f.SuggestedFix != "" {
				fixNote = "fixable"
			}
			rows = append(rows, truncLine(" "+marker+severityLabel(f.Severity)+" "+title, listWidth))
			rows = append(rows, truncLine("     "+styleFaint.Render(f.Family+" · "+fixNote), listWidth))
		}
	}
	list := strings.Join(rows, "\n")

	var d strings.Builder
	if hasSel {
		d.WriteString(" " + severityLabel(sel.Severity) + "  " + styleBold.Render(sel.Title) + "\n")
		chips := " " + styleFaint.Render("["+sel.Family+"]")
		if sel.Identity != "" {
			chips += " " + styleFaint.Render("["+sel.Identity+"]")
		}
		d.WriteString(chips + "\n\n")
		d.WriteString(" " + sel.Explanation + "\n\n")
		if sel.SuggestedFix != "" {
			d.WriteString(" " + styleInfo.Render("~ Suggested fix: "+sel.SuggestedFix) + "\n")
			d.WriteString(" " + styleInfo.Render("~ Switch to Fixer to apply this.") + "\n")
		} else {
			d.WriteString(" " + styleInfo.Render("~ Informational only — nothing to fix.") + "\n")
		}
	}
	// Wrap to the pane width, then clip with a VISIBLE cue — finding
	// explanations must never be silently cut mid-sentence (H3).
	bodyRows := frameBodyRows(height)
	detailPane := fitPane(lipgloss.NewStyle().Width(detailWidth).Render(d.String()), bodyRows)

	body := joinMasterDetail(list, listWidth, detailPane, bodyRows)
	return screenView{body: body, status: status, statusTone: tone, actions: actions}
}
