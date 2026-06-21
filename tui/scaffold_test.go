package tui

// scaffold_test.go — compile-only guard for Wave-0 scaffold symbols.
//
// The types and functions below are defined in this plan as foundations for
// Plans 02-06. They have no callers yet, which triggers the `unused` linter.
// This file references each scaffold symbol exactly once so the package compiles
// cleanly under strict-lint pre-commit hooks — it does not test behaviour.
//
// When each plan's implementation is merged, the callers in model.go / health.go /
// wizard.go / confirm.go etc. will own the references and this guard can be pruned
// or left as additional coverage.

import (
	"testing"
	"time"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// TestScaffoldSymbolsCompile confirms that all Wave-0 scaffold symbols are
// reachable at compile time. It does not assert any behavior — the behavioral
// assertions live in the per-plan test files that each plan's GREEN commit
// will turn from skip to pass.
func TestScaffoldSymbolsCompile(t *testing.T) {
	t.Helper()

	// --- messages.go scaffold types: read every field to satisfy unused linter ---

	// Retained Phase 5 types — read every exported field to prevent unused-field lint.
	fm := familyResultMsg{runID: 1, family: doctor.FamilyDeps, findings: nil, err: nil}
	_, _, _, _ = fm.runID, fm.family, fm.findings, fm.err

	pw := preWriteResultMsg{result: tester.Result{}, err: nil}
	_, _ = pw.result, pw.err

	rr := resolvedResultMsg{result: tester.Result{}, resolved: tester.ResolvedConfig{}}
	_, _ = rr.result, rr.resolved

	wr := writeResultMsg{backupPath: "", err: nil}
	_, _ = wr.backupPath, wr.err

	cb := clipboardResultMsg{err: nil}
	_ = cb.err

	// Phase 5.6 new types:
	_ = clearModalMsg{}
	_ = refreshSidebarMsg{}

	st := setToastMsg{text: "", style: lipgloss.NewStyle()}
	_, _ = st.text, st.style

	_ = clearToastMsg{}

	dr := deleteResultMsg{result: identity.DeleteResult{}, err: nil}
	_, _ = dr.result, dr.err

	fr := fixResultMsg{family: doctor.FamilyDeps, err: nil}
	_, _ = fr.family, fr.err

	rotr := rotateResultMsg{result: identity.CreateResult{}, err: nil}
	_, _ = rotr.result, rotr.err

	bl := baselineLoadedMsg{state: gitconfig.BaselineState{}, err: nil}
	_, _ = bl.state, bl.err

	// --- messages.go helper cmds: capture return values ---
	cmd1 := clearModalCmd()
	cmd2 := refreshSidebarCmd()
	cmd3 := setToastCmd("", lipgloss.NewStyle())
	cmd4 := clearToastAfter(time.Millisecond)
	// Verify they return non-nil cmds (they are closures over valid msgs).
	if cmd1 == nil || cmd2 == nil || cmd3 == nil || cmd4 == nil {
		t.Error("scaffold cmd constructors must return non-nil tea.Cmd")
	}

	// --- styles.go scaffold symbols ---
	_ = StyleTabActive
	_ = StyleTabInactive
	_ = StyleReadOnly
	_ = StyleModalTitle
	_ = StyleModal
	_ = StyleDimmed
	_ = StyleSidebarSection
	_ = StyleSidebarItem
	_ = StyleSidebarUnmanaged
	_ = StyleSidebarBadge
	_ = asciiMode()
	_ = SeverityGlyph(doctor.SeverityInfo, asciiMode())

	// --- keymap.go scaffold bindings ---
	_ = keys.Palette
	_ = keys.View1
	_ = keys.View2
	_ = keys.View3
	_ = keys.SidebarToggle
	_ = keys.Focus
	_ = keys.FocusRev
	_ = keys.Fix
	_ = keys.Retry
	_ = keys.Skip

	t.Log("all Wave-0 scaffold symbols compile and are reachable")
}
