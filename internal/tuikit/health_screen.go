package tuikit

// health_screen.go is the Health tab child model (08-01-PLAN.md Task 1: the
// TabID 4→5 split's read-only half). For THIS task it is a MINIMAL shell
// reusing doctor.go's existing orderedFindings/groupFindings/severityRank
// helpers UNCHANGED and lists ALL findings read-only — Task 2 completes the
// full split (stripping doctorModel's f/F ceremony-trigger keys out of
// doctor.go entirely and moving the read-only rendering here for real).

import (
	tea "charm.land/bubbletea/v2"
)

// healthModel is the Health tab child model.
type healthModel struct {
	doctorModel
}

// newHealthModel builds the Health tab (scan runs on first activation, same
// as the pre-split doctorModel).
func newHealthModel() healthModel { return healthModel{} }

func (m healthModel) activate(s DemoState) (screenModel, tea.Cmd) {
	next, cmd := m.doctorModel.activate(s)
	m.doctorModel = next.(doctorModel)
	return m, cmd
}

func (m healthModel) handleMsg(msg tea.Msg, s DemoState) keyResult {
	res := m.doctorModel.handleMsg(msg, s)
	m.doctorModel = res.model.(doctorModel)
	res.model = m
	return res
}

func (m healthModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	res := m.doctorModel.handleKey(msg, s)
	m.doctorModel = res.model.(doctorModel)
	res.model = m
	return res
}

func (m healthModel) handleClick(x, y, width, height int, s DemoState) keyResult {
	res := m.doctorModel.handleClick(x, y, width, height, s)
	m.doctorModel = res.model.(doctorModel)
	res.model = m
	return res
}

func (m healthModel) view(s DemoState, width, height int) screenView {
	return m.doctorModel.view(s, width, height)
}
