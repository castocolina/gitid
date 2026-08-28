package tuikit

// fixer_screen.go is the Fixer tab child model (08-01-PLAN.md Task 1: the
// TabID 4→5 split's write half). For THIS task it is a MINIMAL shell
// reusing doctor.go's existing orderedFindings/groupFindings/
// fixableFindings/severityRank helpers UNCHANGED, filtered to
// fixableFindings only — Task 2 completes the full split (forking the f/F
// ceremony logic out of doctor.go into this file for real, keeping it
// unchanged, and Health stops carrying any fix affordance).

import (
	tea "charm.land/bubbletea/v2"
)

// fixerModel is the Fixer tab child model.
type fixerModel struct {
	doctorModel
}

// newFixerModel builds the Fixer tab (scan runs on first activation, same
// as the pre-split doctorModel).
func newFixerModel() fixerModel { return fixerModel{} }

// fixableState returns s with Findings narrowed to fixableFindings(ordered)
// only — the Fixer tab's list scope. All doctorModel logic (ordering,
// grouping, ceremony) is reused unchanged against this narrowed state.
func fixableState(s DemoState) DemoState {
	s.Findings = fixableFindings(orderedFindings(s))
	return s
}

func (m fixerModel) activate(s DemoState) (screenModel, tea.Cmd) {
	next, cmd := m.doctorModel.activate(fixableState(s))
	m.doctorModel = next.(doctorModel)
	return m, cmd
}

func (m fixerModel) handleMsg(msg tea.Msg, s DemoState) keyResult {
	res := m.doctorModel.handleMsg(msg, fixableState(s))
	m.doctorModel = res.model.(doctorModel)
	res.model = m
	return res
}

func (m fixerModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	res := m.doctorModel.handleKey(msg, fixableState(s))
	m.doctorModel = res.model.(doctorModel)
	res.model = m
	return res
}

func (m fixerModel) handleClick(x, y, width, height int, s DemoState) keyResult {
	res := m.doctorModel.handleClick(x, y, width, height, fixableState(s))
	m.doctorModel = res.model.(doctorModel)
	res.model = m
	return res
}

func (m fixerModel) view(s DemoState, width, height int) screenView {
	return m.doctorModel.view(fixableState(s), width, height)
}
