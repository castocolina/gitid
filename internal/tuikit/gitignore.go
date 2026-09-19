package tuikit

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// gitIgnoreModel is the Global Git Ignore tab child model. The textarea's
// default line-previous chord includes ctrl+p, which the app reserves for the
// command palette before this screen sees it; the arrow key remains the
// working line-previous binding while editing.
type gitIgnoreModel struct {
	backend        Backend
	state          GlobalGitIgnoreView
	stateErr       string
	applyErr       string
	ceremonyOpen   bool
	ceremony       ceremonyModel
	commitPending  bool
	appliedContent string
	appliedToken   string
	editor         textarea.Model
	editing        bool
}

// seedEditor uses MoveToBegin after SetValue because bubbles' multiline insert
// path leaves the cursor on the final inserted row.
func (m *gitIgnoreModel) seedEditor(content string) {
	m.editor.SetValue(content)
	m.editor.MoveToBegin()
}

func newGitIgnoreModel(b Backend) gitIgnoreModel {
	return gitIgnoreModel{backend: b, editor: textarea.New()}
}

// activate re-reads the machine and discards unsaved edits on every visit.
func (m gitIgnoreModel) activate(DemoState) (screenModel, tea.Cmd) {
	m.stateErr = ""
	m.applyErr = ""
	m.ceremonyOpen = false
	m.commitPending = false
	m.editing = false
	m.editor.Blur()
	view, err := m.backend.GlobalGitIgnoreState()
	m.state = view
	m.seedEditor(view.Content)
	if err != nil {
		m.stateErr = err.Error()
	}
	return m, nil
}

func (m gitIgnoreModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
	// Persist editor geometry on resize (09.2-REVIEW.md WR-03): view() has a
	// VALUE receiver, so m.editor.SetWidth/SetHeight there only ever mutate
	// view()'s own throwaway copy — the model actually stored in
	// App.screens[tab] (the one handleKey/this func's own m.editor.Update
	// calls below feed keystrokes to) never left textarea.New()'s 40x6
	// defaults before this fix, so soft-wrap, MoveToBegin, up/down-by-visual-
	// line, and page-scroll (repositionView) were all computed against the
	// wrong geometry. App.Update forwards tea.WindowSizeMsg to every screen
	// (app.go) specifically so this case can size the PERSISTED model.
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		bodyBudget := frameBodyRows(sz.Height)
		editorWidth := maxInt(20, sz.Width-2)
		m.editor.SetWidth(editorWidth)
		m.editor.SetHeight(m.editorHeight(bodyBudget, editorWidth, m.headerText()))
		return keyResult{model: m}
	}
	if m.editing {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return keyResult{model: m, cmd: cmd}
	}
	commit, ok := msg.(GlobalGitIgnoreCommitMsg)
	if !ok || !m.ceremonyOpen || !m.commitPending {
		return keyResult{model: m}
	}
	m.commitPending = false
	if commit.ChangedSincePreview {
		message := GitIgnoreReceiptChangedSincePreview
		if commit.Err != "" {
			message = commit.Err
		}
		m.ceremony = m.ceremony.commitFailed(message)
		return keyResult{model: m, note: GitIgnoreReceiptChangedSincePreview}
	}
	if commit.Err != "" {
		message := commit.Err
		if len(commit.Restored) > 0 {
			message += " (restored: " + strings.Join(commit.Restored, "; ") + ")"
		}
		m.ceremony = m.ceremony.commitFailed(message)
		return keyResult{model: m}
	}
	m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
	note := m.successReceipt()
	if len(commit.Backups) == 0 {
		note = note + " " + GitIgnoreReceiptNoBackup
		m.ceremony = m.ceremony.withResultExtra(" " + GitIgnoreReceiptNoBackup + "\n")
	}
	return keyResult{model: m, note: note}
}

func (m gitIgnoreModel) successReceipt() string {
	switch m.state.Wiring {
	case GitIgnorePointsElsewhere:
		return GitIgnoreReceiptWrongTarget(m.state.ExcludesFile)
	case GitIgnoreKeyUnset:
		return GitIgnoreReceiptTwoTarget
	default:
		return GitIgnoreReceiptSingleTarget
	}
}

func (m gitIgnoreModel) handleKey(msg tea.KeyMsg, _ DemoState) keyResult {
	if m.ceremonyOpen {
		if m.commitPending {
			return keyResult{model: m, handled: true}
		}
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.ceremonyOpen = false
		case ceremonyConfirmed:
			m.commitPending = true
			return keyResult{
				model:   m,
				handled: true,
				cmd:     m.backend.CommitGlobalGitIgnore(m.appliedContent, m.appliedToken),
			}
		case ceremonyFinished:
			m.ceremonyOpen = false
		case ceremonyNone:
		}
		return keyResult{model: m, handled: true}
	}

	if m.editing {
		if msg.String() == "esc" {
			m.editing = false
			m.editor.Blur()
			return keyResult{model: m, handled: true}
		}
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return keyResult{model: m, handled: true, cmd: cmd}
	}

	switch msg.String() {
	case "enter":
		m.editing = true
		return keyResult{model: m, handled: true, cmd: m.editor.Focus()}
	case "r":
		m.seedEditor(m.state.DefaultContent)
		return keyResult{model: m, handled: true}
	}

	if msg.String() != "a" {
		return keyResult{model: m}
	}
	if m.stateErr != "" {
		return keyResult{model: m, handled: true}
	}
	if m.state.Wiring == GitIgnoreNoBaselineBlock {
		m.applyErr = GitIgnoreWiringNoBaseline
		return keyResult{model: m, handled: true}
	}
	content := m.editor.Value()
	plan, planErr := m.backend.GlobalGitIgnoreApplyPlan(content)
	if planErr != nil {
		m.applyErr = planErr.Error()
		return keyResult{model: m, handled: true}
	}
	m.applyErr = ""
	m.appliedContent = content
	m.appliedToken = plan.PlanToken
	m.ceremony = newApplyCeremony(ceremonyConfig{
		Heading:       GitIgnoreCeremonyHeading,
		Targets:       plan.Targets,
		Backups:       plan.Backups,
		Preview:       plan.Diff,
		ResultMessage: m.successReceipt(),
		ConfirmLabel:  "Confirm write",
		Async:         true,
	})
	m.ceremonyOpen = true
	return keyResult{model: m, handled: true}
}

func (m gitIgnoreModel) view(_ DemoState, width, height int) screenView {
	if m.ceremonyOpen {
		return screenView{
			body: m.ceremony.view(width - 2),
			// No extra breadcrumb segment: unlike Options-style screens
			// (globalgit.go, globalssh.go) this is a single-pane screen with
			// no sub-view, so the breadcrumb reads "Global Git Ignore" once
			// — RenderFrame already prepends tabLabels[tab] — matching
			// 09.2-UI-SPEC.md's Screen Identity table (09.2-UI-REVIEW.md
			// finding 1: passing the tab's own label here triple-duplicated
			// the heading).
			crumbs:       []string{},
			actions:      ceremonyFooterActions(),
			capturesKeys: true,
		}
	}

	bodyBudget := frameBodyRows(height)
	editorWidth := maxInt(20, width-2)
	header := m.headerText()
	// Re-apply the SAME geometry handleMsg's tea.WindowSizeMsg case already
	// persisted onto the stored model (WR-03) — width is normally already
	// correct here (it only changes on resize), but the HEIGHT budget below
	// is recomputed on every render because header can grow or shrink
	// between resizes (stateErr/applyErr/the two-target note appear and
	// disappear without a resize event). Re-applying is idempotent when
	// nothing changed and is required to avoid the clipping WR-04 describes.
	m.editor.SetWidth(editorWidth)
	m.editor.SetHeight(m.editorHeight(bodyBudget, editorWidth, header))
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(m.editor.View())
	actions := []FooterAction{}
	status := GitIgnoreDiscardedEditsStatus
	if m.editing {
		actions = append(actions, FooterAction{Key: "Esc", Label: GitIgnoreDoneEditingLabel})
	} else {
		actions = append(actions,
			FooterAction{Key: "Enter", Label: GitIgnoreEditLabel},
			FooterAction{Key: "r", Label: GitIgnoreResetLabel},
		)
		if m.stateErr == "" && m.state.Wiring != GitIgnoreNoBaselineBlock {
			actions = append(actions, FooterAction{Key: "a", Label: GitIgnoreApplyLabel})
		}
	}
	wrapped := lipgloss.NewStyle().Width(maxInt(20, width-2)).Render(b.String())
	return screenView{
		body: fitPane(wrapped, bodyBudget),
		// No extra breadcrumb segment — see the identical note on the
		// ceremony branch above (09.2-UI-REVIEW.md finding 1).
		crumbs:       []string{},
		actions:      actions,
		status:       status,
		capturesKeys: m.editing,
	}
}

// headerText renders everything the browse/editor pane shows ABOVE the
// editor — heading, path, wiring line, and whichever advisories currently
// apply. Shared by view() (which renders it) and editorHeight (which
// measures how many rows it consumes) so the two can never disagree about
// how many rows the header takes (09.2-REVIEW.md WR-04).
func (m gitIgnoreModel) headerText() string {
	var b strings.Builder
	b.WriteString(" " + styleBold.Render(GitIgnoreHeading) + "\n")
	if m.state.Path != "" {
		b.WriteString(" " + styleFaint.Render(m.state.Path) + "\n")
	}
	b.WriteString(" " + m.wiringLine() + "\n")
	if m.stateErr != "" {
		b.WriteString(" " + styleWarning.Render(ensureGitIgnoreGlyph(m.stateErr)) + "\n")
	}
	if m.applyErr != "" {
		// The sentinel-rejection message is this screen's ONE styleError
		// (Error-red) state and already carries its own "✗ " glyph
		// (GitIgnoreSentinelRejectedMessage). Every other applyErr — e.g. a
		// malformed-file refusal reached defensively through the apply path
		// — is a warning, not an error, and must get the SAME glyph
		// guarantee stateErr gets above; rendering it via styleError with no
		// glyph would reintroduce the exact NO_COLOR-legibility defect
		// 09.2-UI-REVIEW.md finding 3 fixed on this screen (09.2-REVIEW.md
		// WR-05).
		if strings.HasPrefix(m.applyErr, "✗ ") {
			b.WriteString(" " + styleError.Render(m.applyErr) + "\n")
		} else {
			b.WriteString(" " + styleWarning.Render(ensureGitIgnoreGlyph(m.applyErr)) + "\n")
		}
	}
	if !m.state.Managed && m.stateErr == "" {
		b.WriteString(" " + styleFaint.Render(GitIgnoreNoManagedBlock) + "\n")
	}
	if m.state.Wiring == GitIgnoreKeyUnset {
		b.WriteString(" " + styleWarning.Render(GitIgnoreTwoTargetNote) + "\n")
	}
	return b.String()
}

// editorHeight derives the editor's row budget from the rows the header
// ACTUALLY consumes at editorWidth, rather than the fixed `bodyBudget-5`
// guess the previous version used — the header writes between 4 and 8 rows
// depending on which advisories are showing, and several wrap to two rows at
// typical widths, so a fixed guess overflows the pane and fitPane silently
// truncates the bottom of the editor (09.2-REVIEW.md WR-04: every promoted
// frame showed the symptom, e.g. "… (+3 more lines)"). editorWidth MUST be
// the same width passed to editor.SetWidth so this measurement matches what
// the editor itself renders at.
func (m gitIgnoreModel) editorHeight(bodyBudget, editorWidth int, header string) int {
	used := lipgloss.Height(lipgloss.NewStyle().Width(editorWidth).Render(header))
	return maxInt(1, bodyBudget-used-1)
}

// ensureGitIgnoreGlyph prefixes msg with "! " unless it already begins with
// a known glyph (a frozen constant that already bakes one in, e.g.
// GitIgnoreWiringNoBaseline's "! " or GitIgnoreSentinelRejectedMessage's
// "✗ "). This is the single place that guarantees "every colored state
// pairs with a glyph AND a word" for stateErr/applyErr regardless of which
// gitconfig error path produced the text (09.2-REVIEW.md WR-05).
func ensureGitIgnoreGlyph(msg string) string {
	for _, glyph := range []string{"✓ ", "! ", "✗ "} {
		if strings.HasPrefix(msg, glyph) {
			return msg
		}
	}
	return "! " + msg
}

func (m gitIgnoreModel) wiringLine() string {
	switch m.state.Wiring {
	case GitIgnoreWiredAtManaged:
		return styleHealthy.Render(GitIgnoreWiringWired)
	case GitIgnoreKeyUnset:
		return styleWarning.Render(GitIgnoreWiringKeyUnset)
	case GitIgnorePointsElsewhere:
		return styleWarning.Render(GitIgnoreWiringPointsElsewhere(m.state.ExcludesFile))
	case GitIgnoreNoBaselineBlock:
		return styleWarning.Render(GitIgnoreWiringNoBaseline)
	default:
		return ""
	}
}
