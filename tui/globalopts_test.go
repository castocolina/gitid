package tui

// globalopts_test.go — Tests for the Global Options view (Plan 04 GREEN).
//
// Tests are locked by VALIDATION.md.

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
)

// fakeGlobalDeps returns a tuiDeps with baseline paths that ReadBaselineState
// will read as "not installed" (files don't exist).
func fakeGlobalDeps() tuiDeps {
	return tuiDeps{
		doctor: fakeTUIDocDepsForHealth(),
	}
}

// TestGlobalOptionsRendersBaseline verifies that switching to view 3 dispatches
// refresh → baselineLoadedMsg; globalOptions.view() renders the baseline state
// (shows key sections) and shows the empty-state when baseline is not configured.
// Requirement: TUI-04 (Global Options view, ReadBaselineState).
// Closes: Plan 04.
func TestGlobalOptionsRendersBaseline(t *testing.T) {
	deps := fakeGlobalDeps()
	m := newGlobalOptionsModel(deps)

	// Before loading: shows loading state.
	out := m.view(80, 24)
	if !strings.Contains(out, "Loading") {
		t.Errorf("before load, view must show loading; got: %q", out)
	}

	// Simulate a loaded BaselineState that is not installed.
	notInstalled := baselineLoadedMsg{
		state: gitconfig.BaselineState{Installed: false},
		err:   nil,
	}
	m2, _ := m.update(notInstalled)
	out2 := m2.view(80, 24)
	if !strings.Contains(out2, "not been set up") {
		t.Errorf("when baseline not installed, view must show empty-state; got: %q", out2)
	}

	// Simulate a loaded BaselineState that IS installed.
	installed := baselineLoadedMsg{
		state: gitconfig.BaselineState{
			Installed: true,
			BaselineKeys: map[string]string{
				"core.ignorecase":      "false",
				"core.excludesfile":    "~/.gitignore_global",
				"push.autosetupremote": "true",
				"pull.rebase":          "true",
				"fetch.prune":          "true",
				"color.ui":             "auto",
			},
			GitignorePatterns: []string{".DS_Store", "*.log"},
		},
		err: nil,
	}
	m3, _ := m.update(installed)
	out3 := m3.view(80, 24)

	checks := []string{
		"Global Git Config",
		"Core:",
		"ignorecase",
		"excludesfile",
		"Push / Pull / Fetch:",
		"Color:",
		"Global Gitignore",
	}
	for _, want := range checks {
		if !strings.Contains(out3, want) {
			t.Errorf("installed baseline view must contain %q; got: %q", want, out3)
		}
	}
}

// TestGlobalOptionsInlineEdit verifies that pressing 'e' enters inline edit mode,
// showing the [editing] indicator; Esc exits edit mode.
// Requirement: TUI-04 (inline edit contract, D-05).
// Closes: Plan 04.
func TestGlobalOptionsInlineEdit(t *testing.T) {
	deps := fakeGlobalDeps()
	m := newGlobalOptionsModel(deps)

	// Simulate an installed baseline.
	loadMsg := baselineLoadedMsg{
		state: gitconfig.BaselineState{
			Installed: true,
			BaselineKeys: map[string]string{
				"core.ignorecase": "false",
			},
		},
	}
	m, _ = m.update(loadMsg)

	// Press 'e' → enters editing mode.
	m2, _ := m.handleKey("e")
	if !m2.editing {
		t.Error("pressing 'e' must set editing=true")
	}
	out := m2.view(80, 24)
	if !strings.Contains(out, "editing") {
		t.Errorf("view in edit mode must contain 'editing'; got: %q", out)
	}

	// Esc → exits editing mode.
	m3, _ := m2.handleKey("esc")
	if m3.editing {
		t.Error("Esc must clear editing=false")
	}
	out3 := m3.view(80, 24)
	if strings.Contains(out3, "[editing]") {
		t.Errorf("view after Esc must not contain '[editing]'; got: %q", out3)
	}
}
