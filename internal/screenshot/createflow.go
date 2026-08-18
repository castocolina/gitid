//go:build screenshot

package screenshot

// createflow.go drives the shared internal/tuikit render stack (either
// binary's Backend injected) through a FIXED script and captures the
// rendered View().Content text at explicitly enumerated create-flow screen
// checkpoints — the "capture" half of the DLV-04.1/D-24.1 visual-regression
// gate (plan 03-06, Task 2).
//
// The SAME script drives both cmd/gitid-dummy's FixtureBackend and the real
// cmd/gitid Backend, so the two capture sets are directly diffable, screen
// by screen, modulo the documented divergence allowlist
// (.planning/design/create-flow/visual-divergence-allowlist.txt). No PTY,
// no subprocess: tuikit.App.Update/.View are driven in-process via
// synthesized tea.Msg values — the SAME technique internal/tuikit's own
// test suite already uses (see internal/tuikit/identities_test.go), safe
// here because this gate is about RENDERED TEXT equivalence, not raw
// keystroke/terminal-decoding correctness (that is DLV-06's PTY e2e job,
// plan 03-06 Task 1).
//
// CaptureWidth/CaptureHeight match screenshot-tui's own fixed geometry
// (D-04) so a future PNG-based capture of these same screens stays
// apples-to-apples; this gate itself only needs the geometry (pane-width
// wrapping affects the TEXT), not the vendored font/theme (which only
// affects PNG pixel rendering, not text content).

import (
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/tuikit"
)

// CaptureWidth/CaptureHeight are the D-04/D-24 fixed capture geometry —
// the SAME 100x30 values screenshot-tui (internal/screenshot/tui.go,
// tui_capture_test.go) already uses.
const (
	CaptureWidth  = 100
	CaptureHeight = 30
)

// CreateFlowScreenIDs is the EXPLICIT, enumerated set of create-flow screen
// checkpoints the D-24.1 gate captures and diffs. A screen absent from this
// list is silently ungated — per review, the reuse-existing-key picker
// (populated), the manual-path row, and a mouse-focused field state are
// deliberately included alongside the wizard's ordinary steps and both
// connectivity-test stages.
var CreateFlowScreenIDs = []string{
	"ssh-form-filled",       // step 0: the default-filled SSH form + live Host-block preview
	"reuse-key-vs-generate", // step 0: D-10 picker, populated (backend.ScanReusableKeys())
	"reuse-manual-path",     // step 0: D-10 picker's trailing manual-path row selected
	"mouse-focused-field",   // step 0: a field focused via a REAL synthesized mouse click
	"test-stage1-direct",    // step 1: stage 1 (TEST-01) outcome rendered
	"test-stage2-by-alias",  // step 1: stage 2 (TEST-02) outcome rendered
	"git-form-demo",         // step 2: the demo'd Git-identity step (D-18/D-19)
	"confirm-write",         // step 3: the review/confirm-write ceremony (state A)
}

// step drives model with msg, then synchronously drains any cmd chain the
// Update call returns — recursively, so a multi-hop chain (e.g. a stage
// test's tea.Cmd delivering a WizardStageMsg) resolves fully before the
// next scripted step runs. Bubble Tea's own runtime does the same thing
// across render frames; here it happens inline since there is no real
// event loop driving this capture.
func step(model tea.Model, msg tea.Msg) tea.Model {
	m, cmd := model.Update(msg)
	if cmd != nil {
		if msg2 := cmd(); msg2 != nil {
			return step(m, msg2)
		}
	}
	return m
}

// keyRune sends a single printable-character key press (e.g. "n" to open
// the wizard, "y" — none needed here, but mirrors the e2e harness's
// keystroke-by-keystroke discipline).
func keyRune(model tea.Model, r rune) tea.Model {
	return step(model, tea.KeyPressMsg{Code: r, Text: string(r)})
}

func keyEnter(model tea.Model) tea.Model { return step(model, tea.KeyPressMsg{Code: tea.KeyEnter}) }
func keyTab(model tea.Model) tea.Model   { return step(model, tea.KeyPressMsg{Code: tea.KeyTab}) }
func keyRight(model tea.Model) tea.Model { return step(model, tea.KeyPressMsg{Code: tea.KeyRight}) }
func keyLeft(model tea.Model) tea.Model  { return step(model, tea.KeyPressMsg{Code: tea.KeyLeft}) }

// tabN sends n Tab keypresses in sequence.
func tabN(model tea.Model, n int) tea.Model {
	for i := 0; i < n; i++ {
		model = keyTab(model)
	}
	return model
}

// freshWizard boots a new App around backend at the fixed capture geometry
// and opens the create wizard from the identities pane (the SAME 'n'
// keystroke a real user presses) — the common starting point every
// captured screen scripts forward from, so no screen's capture can leak
// state from a PRIOR screen's navigation.
func freshWizard(backend tuikit.Backend) tea.Model {
	var model tea.Model = tuikit.NewApp(backend)
	model = step(model, tea.WindowSizeMsg{Width: CaptureWidth, Height: CaptureHeight})
	model = keyRune(model, 'n')
	return model
}

// anyView extracts the plain rendered text from model's current View() —
// the tea.Model interface's View() already returns a concrete
// tea.View{Content string, ...} (charm.land/bubbletea/v2).
func anyView(model tea.Model) string {
	return model.View().Content
}

// clickField locates label's row in text (the SAME "search the decoded
// frame for a label, click its row" technique e2e/create_flow_pty_e2e_test.go's
// clickLabelRow uses for real xterm SGR sequences) and sends a REAL
// tea.MouseClickMsg at that position — proving the mouse-click code path
// itself, not merely re-deriving the same result a keyboard Tab would.
func clickField(model tea.Model, label string) tea.Model {
	text := anyView(model)
	x, y, ok := locateLabel(text, label)
	if !ok {
		return model
	}
	return step(model, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

// locateLabel finds label's first occurrence in text and returns its
// (column, row) — 0-based, matching tea.Mouse's coordinate space.
func locateLabel(text, label string) (x, y int, ok bool) {
	lines := splitLines(text)
	for row, line := range lines {
		if idx := indexOf(line, label); idx >= 0 {
			return idx + 1, row, true
		}
	}
	return 0, 0, false
}

// splitLines and indexOf avoid importing "strings" twice across this small
// file's helpers — trivial local implementations, no external behavior.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func indexOf(s, substr string) int {
	if substr == "" {
		return -1
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// CaptureCreateFlowScreens drives backend's create-flow wizard through the
// fixed script above and returns the rendered text at every
// CreateFlowScreenIDs checkpoint, keyed by screen ID.
func CaptureCreateFlowScreens(backend tuikit.Backend) map[string]string {
	out := make(map[string]string, len(CreateFlowScreenIDs))

	// ssh-form-filled: the wizard's default-filled step 0.
	m := freshWizard(backend)
	out["ssh-form-filled"] = anyView(m)

	// reuse-key-vs-generate: Tab from Alias prefix (focus 1) to the
	// Generate/Reuse toggle (focus 5) — 4 Tabs — then flip to reuse.
	m = freshWizard(backend)
	m = tabN(m, 4)
	m = keyRight(m)
	out["reuse-key-vs-generate"] = anyView(m)

	// reuse-manual-path: from reuseIdx=0, a single Left wraps DIRECTLY to
	// the trailing manual-path row (D-10's own wraparound arithmetic),
	// regardless of how many candidates the backend's scan returns.
	m = keyLeft(m)
	out["reuse-manual-path"] = anyView(m)

	// mouse-focused-field: a REAL synthesized mouse click on the Port row,
	// from a fresh generate-mode wizard (keeps this screen's SSH-field
	// layout directly comparable to ssh-form-filled).
	m = freshWizard(backend)
	m = clickField(m, "Port")
	out["mouse-focused-field"] = anyView(m)

	// test-stage1-direct / test-stage2-by-alias: advance to step 1 and run
	// both stages in sequence, capturing the rendered outcome after each.
	m = freshWizard(backend)
	m = keyEnter(m) // step 0 -> step 1
	m = keyEnter(m) // run stage 1 (drains the WizardStageMsg chain via step())
	out["test-stage1-direct"] = anyView(m)
	m = keyEnter(m) // run stage 2
	out["test-stage2-by-alias"] = anyView(m)

	// git-form-demo: advance to step 2 (only reachable once both stages
	// have answered — the SAME model instance carries that state forward).
	m = keyEnter(m)
	out["git-form-demo"] = anyView(m)

	// confirm-write: Skip Git (4 Tabs from user.name to the Skip button,
	// then Enter) reaches the review ceremony (state A, unconfirmed).
	m = tabN(m, 4)
	m = keyEnter(m)
	out["confirm-write"] = anyView(m)

	return out
}
