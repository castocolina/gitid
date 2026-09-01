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
	m.ceremony = newCeremony(ceremonyConfig{
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

	var b strings.Builder
	b.WriteString(" " + styleBold.Render(GitIgnoreHeading) + "\n")
	if m.state.Path != "" {
		b.WriteString(" " + styleFaint.Render(m.state.Path) + "\n")
	}
	b.WriteString(" " + m.wiringLine() + "\n")
	if m.stateErr != "" {
		b.WriteString(" " + styleWarning.Render("! "+m.stateErr) + "\n")
	}
	if m.applyErr != "" {
		b.WriteString(" " + styleError.Render(m.applyErr) + "\n")
	}
	if !m.state.Managed && m.stateErr == "" {
		b.WriteString(" " + styleFaint.Render(GitIgnoreNoManagedBlock) + "\n")
	}
	if m.state.Wiring == GitIgnoreKeyUnset {
		b.WriteString(" " + styleWarning.Render(GitIgnoreTwoTargetNote) + "\n")
	}
	bodyBudget := frameBodyRows(height)
	m.editor.SetWidth(maxInt(20, width-2))
	m.editor.SetHeight(maxInt(1, bodyBudget-5))
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
