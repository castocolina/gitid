package tui

import (
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"testing"
)

// benchWizardModel returns a 120x40 root model with the create wizard open and a
// few characters typed — the exact state the user reports as slow.
func benchWizardModel() rootModel {
	m := buildModel()
	m.activeView = identitiesView
	m = sendKey(m, "a") // open create wizard
	for _, r := range "personal" {
		m = sendMsg(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

// BenchmarkWizardView measures one full render frame (what runs per keystroke).
func BenchmarkWizardView(b *testing.B) {
	m := benchWizardModel()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.renderContent()
	}
}

// BenchmarkWizardKeystroke measures Update+View together — a single typed char.
func BenchmarkWizardKeystroke(b *testing.B) {
	m := benchWizardModel()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m = sendMsg(m, tea.KeyPressMsg{Code: 'x', Text: "x"})
		_ = m.renderContent()
	}
}

// BenchmarkDimBackground isolates the StyleDimmed.Render over the full layout.
func BenchmarkDimBackground(b *testing.B) {
	m := benchWizardModel()
	layout := m.renderPersistentLayout()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StyleDimmed.Render(layout)
	}
}

// BenchmarkPlaceOverlay isolates the ANSI-aware modal compositing.
func BenchmarkPlaceOverlay(b *testing.B) {
	m := benchWizardModel()
	layout := m.renderPersistentLayout()
	dimmed := StyleDimmed.Render(layout)
	modal := m.wizard.view(m.width)
	mw := modalWidth(m.width)
	mh := lipgloss.Height(modal)
	x, y := modalOrigin(m.width, m.height, mw, mh)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = placeOverlay(x, y, modal, dimmed)
	}
}
