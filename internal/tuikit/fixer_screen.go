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

// fixerSuggestedFixHandoff is the trailing clause every SuggestedFix string
// carries so Health's read-only detail pane can point the user at the
// Fixer tab (HLTH-04's own "available on the Fixer screen" hand-off,
// internal/dummytui/data.go). On the Fixer screen itself the SAME clause
// is stale -- the user is already here, and the "f · Fix this…" affordance
// immediately below the suggested-fix line already states the action --
// so fixerSuggestedFixText strips it before rendering (08-08 UX review
// finding F6).
const fixerSuggestedFixHandoff = " -- available on the Fixer screen."

// fixerSuggestedFixText returns text with the Fixer hand-off clause
// stripped, for the Fixer tab's own detail pane. Health's detail pane
// renders the SuggestedFix field unchanged (fixerSuggestedFixHandoff's
// doc comment).
func fixerSuggestedFixText(text string) string {
	return strings.TrimSuffix(text, fixerSuggestedFixHandoff)
}

// fixerModel is the Fixer tab child model.
type fixerModel struct {
	backend    Backend
	scanning   bool
	selectedID string
	fixing     bool
	batch      *doctorBatch
	ceremony   ceremonyModel
	// pendingFixID/pendingFixName name the fix this handleKey call just
	// dispatched a FixFinding action for (D-16). Backend.Persist runs
	// SYNCHRONOUSLY inside App.apply, called right after handleKey returns
	// in the SAME Update cycle -- there is no async message round trip for
	// this action (unlike the globalgit/globalssh/identities ceremonies,
	// which dispatch a tea.Cmd and later call commitFailed/commitSucceeded
	// from a delivered Msg). App.handleKey checks Backend.PersistError()
	// immediately after a.apply() returns and, on failure, replaces the
	// screen's optimistically-advanced model with one built from the
	// PRE-dispatch snapshot via haltBatch -- see app.go's FixFinding
	// post-apply check.
	pendingFixID   string
	pendingFixName string
	// batchSucceeded lists the titles of every fix that has ALREADY applied
	// and verified successfully in the CURRENT batch walk (D-16's "the
	// first N-1 fixes already applied stand").
	batchSucceeded []string
	// batchHalt is the D-16 queue-halt message, non-empty only while the
	// batch walk has stopped on a verified failure. batchFailedName names
	// the fix that failed.
	batchHalt       string
	batchFailedName string
}

// newFixerModel builds the Fixer tab (scan runs on first activation). b is
// the injected Backend seam Backend.FixPlanFor routes through (08-02-PLAN.md
// Task 2) — the real backend renders a true diff from actual file content,
// FixtureBackend delegates unchanged to the frozen free PlanFor switch.
func newFixerModel(b Backend) fixerModel { return fixerModel{backend: b} }

// haltBatch handles a real Persist failure after a fix ceremony's confirm
// (08-06-PLAN.md Task 3). It is called on the PRE-dispatch fixerModel
// snapshot (the receiver, m, as it stood right before ceremonyFinished's
// optimistic queue-advance) so m.batch/m.selectedID/m.batchSucceeded
// already correctly identify the fix that just failed and every fix that
// already succeeded before it — App's own optimistic post-handleKey model
// (which already advanced past this fix assuming success) is discarded by
// the caller in favor of this one.
//
// m.ceremony (also the PRE-transition ceremony) is always put into the SAME
// retryable-error state every other ceremony in this codebase uses
// (ceremonyModel.commitFailed) — Wave 2's D-10 auto-restore already
// guarantees the failed fix's own backed-up file is back to its pre-fix
// state; this only adds the user-facing halt message and Retry affordance,
// not a new commit-failure detection mechanism.
//
// The D-16 batch-shaped banner ("Fix N of M failed...") is ONLY added when
// this failure happened mid-`F`-walk (m.batch != nil) — a single `f` fix
// failing outside a batch is not "Fix 1 of 0" (08-08 code review WR-01:
// that message previously rendered nonsensically, mentioning "this batch"
// and "0 of 0", for a fix that was never part of one).
func (m fixerModel) haltBatch(failedName, errMsg string) fixerModel {
	m.ceremony = m.ceremony.commitFailed(errMsg)
	if m.batch == nil {
		return m
	}
	total := m.batch.total
	n := len(m.batchSucceeded) + 1
	m.batchFailedName = failedName
	m.batchHalt = fmt.Sprintf(
		"Fix %d of %d failed and was rolled back from its own backup -- the first %d fixes already applied stand. Nothing else in this batch was attempted.",
		n, total, n-1)
	m.batch = nil
	return m
}

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
			m.batchHalt = ""
			m.batchFailedName = ""
			m.batchSucceeded = nil
		case ceremonyFinished:
			plan := m.backend.FixPlanFor(sel)
			action := FixFinding{ID: sel.ID, Backup: NewBackupPath(plan.File)}
			// D-16: name what this dispatch is FOR so App.handleKey can check
			// Backend.PersistError() after Persist runs and, on failure,
			// build the halt state from the pre-dispatch snapshot it also
			// keeps (see app.go).
			m.pendingFixID = sel.ID
			m.pendingFixName = sel.Title
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
		if ok && sel.Fixable {
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
			// A fresh batch walk starts with no halt/success history from
			// any earlier walk.
			m.batchHalt = ""
			m.batchFailedName = ""
			m.batchSucceeded = nil
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
		if hasSel && sel.Fixable {
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
	if m.batchHalt != "" {
		d.WriteString(" " + styleError.Render(m.batchHalt) + "\n")
	} else if m.batch != nil && m.fixing {
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
		d.WriteString(" " + styleInfo.Render("~ Suggested fix: "+fixerSuggestedFixText(sel.SuggestedFix)) + "\n")
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
