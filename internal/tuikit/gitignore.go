package tuikit

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// gitIgnoreModel is the Global Git Ignore tab child model. This plan ships
// a read-only body plus the review-before-write ceremony; editing arrives
// in plan 09.2-03.
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
}

func newGitIgnoreModel(b Backend) gitIgnoreModel {
	return gitIgnoreModel{backend: b}
}

func (m gitIgnoreModel) activate(DemoState) (screenModel, tea.Cmd) {
	m.stateErr = ""
	m.applyErr = ""
	m.ceremonyOpen = false
	m.commitPending = false
	view, err := m.backend.GlobalGitIgnoreState()
	m.state = view
	if err != nil {
		m.stateErr = err.Error()
	}
	return m, nil
}

func (m gitIgnoreModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
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
	plan, planErr := m.backend.GlobalGitIgnoreApplyPlan(m.state.Content)
	if planErr != nil {
		m.applyErr = planErr.Error()
		return keyResult{model: m, handled: true}
	}
	m.applyErr = ""
	m.appliedContent = m.state.Content
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
			body:         m.ceremony.view(width - 2),
			crumbs:       []string{GitIgnoreHeading},
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
	body := m.state.Content
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
	}
	actions := []FooterAction{}
	if m.stateErr == "" && m.state.Wiring != GitIgnoreNoBaselineBlock {
		actions = append(actions, FooterAction{Key: "a", Label: GitIgnoreApplyLabel})
	}
	wrapped := lipgloss.NewStyle().Width(maxInt(20, width-2)).Render(b.String())
	return screenView{
		body:    fitPane(wrapped, frameBodyRows(height)),
		crumbs:  []string{GitIgnoreHeading},
		actions: actions,
		status:  "Content is written as reviewed.",
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
