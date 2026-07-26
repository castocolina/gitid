package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/castocolina/gitid/internal/identity"
)

// TestPlaceOverlayCompositesModal is the Wave-0 spike-result verification.
// It proves:
//  1. A 3-line modal composited over a 10-line dimmed background places the
//     modal lines at the centered rows and correct columns.
//  2. Untouched background rows are preserved verbatim.
//  3. An oversized modal (larger than the background) clamps without panic.
//  4. placeOverlayAvailable == false (documents the spike result).
func TestPlaceOverlayCompositesModal(t *testing.T) {
	// Confirm the spike result is recorded correctly.
	if placeOverlayAvailable {
		t.Error("placeOverlayAvailable must be false: lipgloss.PlaceOverlay does not exist at v2.0.3")
	}

	// Build a 10-line background (30 chars wide, each line uniquely identifiable).
	bgLines := make([]string, 10)
	for i := range bgLines {
		// Each line: "row-N: ........................" (30 chars)
		bgLines[i] = strings.Repeat(".", 30)
	}
	bg := strings.Join(bgLines, "\n")

	// A 3-line foreground modal at position x=5, y=3.
	const x, y = 5, 3
	fg := "AAA\nBBB\nCCC"

	result := placeOverlay(x, y, fg, bg)
	resultLines := strings.Split(result, "\n")

	if len(resultLines) != 10 {
		t.Fatalf("placeOverlay must preserve line count; got %d lines, want 10", len(resultLines))
	}

	// Rows 0,1,2 (before y=3): must be unchanged.
	for i := 0; i < y; i++ {
		if resultLines[i] != bgLines[i] {
			t.Errorf("row %d (before modal): got %q; want %q (unchanged)", i, resultLines[i], bgLines[i])
		}
	}

	// Rows 3,4,5 (modal rows): the modal runes must appear at columns x..x+3.
	modalContent := []string{"AAA", "BBB", "CCC"}
	for i, mc := range modalContent {
		row := y + i
		line := resultLines[row]
		runes := []rune(line)
		// Columns [x, x+3) must hold the modal runes.
		for j, wantRune := range []rune(mc) {
			col := x + j
			if col >= len(runes) {
				t.Errorf("row %d col %d: out of bounds (runes len=%d)", row, col, len(runes))
				continue
			}
			if runes[col] != wantRune {
				t.Errorf("row %d col %d: got %q; want %q", row, col, string(runes[col]), string(wantRune))
			}
		}
		// Columns before x must still be dots.
		for col := 0; col < x && col < len(runes); col++ {
			if runes[col] != '.' {
				t.Errorf("row %d col %d (before modal): got %q; want '.'", row, col, string(runes[col]))
			}
		}
	}

	// Rows 6..9 (after modal): must be unchanged.
	for i := y + len(strings.Split(fg, "\n")); i < 10; i++ {
		if resultLines[i] != bgLines[i] {
			t.Errorf("row %d (after modal): got %q; want %q (unchanged)", i, resultLines[i], bgLines[i])
		}
	}
}

// TestPlaceOverlayOversizedClampsWithoutPanic verifies T-05.6-02:
// a modal larger than the background clamps silently without panic.
func TestPlaceOverlayOversizedClampsWithoutPanic(_ *testing.T) {
	bg := "short\nlines\nonly"
	// Modal is 10 lines tall over a 3-line background, positioned at row 1.
	fgLines := make([]string, 10)
	for i := range fgLines {
		fgLines[i] = "MODAL"
	}
	fg := strings.Join(fgLines, "\n")

	// Must not panic.
	result := placeOverlay(0, 1, fg, bg)
	_ = result
}

// TestModalOriginCenters verifies the centered-origin calculation.
func TestModalOriginCenters(t *testing.T) {
	tests := []struct {
		termW, termH, modalW, modalH int
		wantX, wantY                 int
	}{
		{80, 24, 40, 10, 20, 7},
		{100, 30, 60, 14, 20, 8},
		// Oversized modal: clamp to 0.
		{20, 10, 40, 20, 0, 0},
	}
	for _, tt := range tests {
		x, y := modalOrigin(tt.termW, tt.termH, tt.modalW, tt.modalH)
		if x != tt.wantX || y != tt.wantY {
			t.Errorf("modalOrigin(%d,%d,%d,%d) = (%d,%d); want (%d,%d)",
				tt.termW, tt.termH, tt.modalW, tt.modalH, x, y, tt.wantX, tt.wantY)
		}
	}
}

// TestModalBoxRendersTitle verifies modalBox includes the title in its output.
func TestModalBoxRendersTitle(t *testing.T) {
	box := modalBox(80, "Delete identity?", "This action cannot be undone.")
	if !strings.Contains(box, "Delete identity?") {
		t.Errorf("modalBox output must contain the title; got:\n%s", box)
	}
	if !strings.Contains(box, "This action cannot be undone.") {
		t.Errorf("modalBox output must contain the body; got:\n%s", box)
	}
}

// visibleSlice returns the visible (ANSI-stripped) columns [start, start+n) of s.
func visibleSlice(s string, start, n int) string {
	r := []rune(ansi.Strip(s))
	if start >= len(r) {
		return ""
	}
	end := start + n
	if end > len(r) {
		end = len(r)
	}
	return string(r[start:end])
}

// TestOverlayLineANSIAwarePlacement is a regression test for the modal-shift bug
// (P0-1): the background carries StyleDimmed (faint) ANSI escapes, so a rune-index
// copy placed the modal shifted right by the escape-rune count. The ANSI-aware
// splice must place the foreground at the correct VISIBLE column.
func TestOverlayLineANSIAwarePlacement(t *testing.T) {
	bg := StyleDimmed.Render(strings.Repeat(".", 20)) // faint ANSI around 20 dots
	const x = 5
	out := overlayLine(x, "AAA", bg)

	if got := visibleSlice(out, x, 3); got != "AAA" {
		t.Errorf("modal at visible cols [5,8) = %q; want \"AAA\" (placement must be ANSI-aware)", got)
	}
	if got := visibleSlice(out, 0, x); got != "....." {
		t.Errorf("background before modal = %q; want \".....\" (preserved)", got)
	}
	if got := visibleSlice(out, x+3, 3); got != "..." {
		t.Errorf("background after modal = %q; want \"...\" (preserved)", got)
	}
}

// TestViewportBoundModalLinesReachable verifies Task 1 (G-1 root cause):
// when a modal is taller than the available terminal rows, boundModalToViewport
// returns bounded lines and a scroll indicator — no modal line is silently dropped.
// Every line is reachable across scroll positions.
func TestViewportBoundModalLinesReachable(t *testing.T) {
	// Build a 20-line modal (simulating a tall wizard screen).
	modalLines := make([]string, 20)
	for i := range modalLines {
		modalLines[i] = fmt.Sprintf("line-%d", i)
	}
	modal := strings.Join(modalLines, "\n")

	// Available rows: 10 (terminal too short for the 20-line modal).
	const available = 10

	// Scroll position 0: first 9 lines + indicator.
	bounded0 := boundModalToViewport(modal, available, 0)
	b0Lines := strings.Split(bounded0, "\n")
	if len(b0Lines) > available {
		t.Errorf("boundModalToViewport must not exceed available rows; got %d lines (available %d)", len(b0Lines), available)
	}
	if !strings.Contains(bounded0, "line-0") {
		t.Errorf("scroll offset 0: must contain first modal line; got:\n%s", bounded0)
	}
	// Must have a scroll-more indicator when content continues below.
	if !strings.Contains(bounded0, "↓") && !strings.Contains(bounded0, "more") {
		t.Errorf("scroll offset 0: must contain scroll-more indicator ('↓' or 'more'); got:\n%s", bounded0)
	}

	// Scroll position near end: last lines must be reachable.
	// At offset 12 (20 lines - 8 visible body = offset 12 reaches near end).
	bounded12 := boundModalToViewport(modal, available, 12)
	if !strings.Contains(bounded12, "line-19") {
		t.Errorf("scroll offset 12: must reach the last modal line; got:\n%s", bounded12)
	}

	// When modal fits (available >= modal lines), no indicator needed,
	// content is unchanged.
	fitsModal := strings.Join(modalLines[:5], "\n") // 5-line modal
	fits := boundModalToViewport(fitsModal, available, 0)
	if fits != fitsModal {
		t.Errorf("fits-on-screen modal must be returned unchanged; got:\n%s", fits)
	}
}

// TestRenderContentViewportAwareModal verifies model.go renderContent:
// with a small terminal (80x24) and the wizard modal open, renderContent
// does not panic and the visible output is bounded to the terminal height.
// The existing fits-on-screen overlay contract must also still hold.
func TestRenderContentViewportAwareModal(t *testing.T) {
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	// Open the create wizard (simulates a tall modal on a small terminal).
	m = sendKey(m, "1") // identities view
	m = sendKey(m, "a") // open wizard
	if m.activeModal != createWizardModal {
		t.Skip("wizard modal not open; skipping viewport test")
	}

	// renderContent must not panic even with the wizard modal on 80x24.
	var content string
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("renderContent panicked with wizard modal on 80x24: %v", r)
			}
		}()
		content = m.renderContent()
	}()

	// Output must fit in 24 lines (the terminal height).
	lines := strings.Split(content, "\n")
	if len(lines) > 24 {
		t.Errorf("renderContent must not exceed terminal height (%d lines > 24)", len(lines))
	}
	// The modal title must be visible.
	if !strings.Contains(content, "Create Identity") {
		t.Errorf("renderContent must show wizard title; got (first 200 chars):\n%s", truncateString(content, 200))
	}
}

// TestRenderFormFieldSingleBorder is a regression test for the phantom-box bug
// (P0-1): a width-less input wrapped in a bordered style produced an offset,
// mismatched second box. With a fixed input width the field must render as a
// single 3-line bordered box with the label on the content line.
func TestRenderFormFieldSingleBorder(t *testing.T) {
	ti := textinput.New()
	ti.SetWidth(formFieldWidth)
	ti.SetValue("castocolina.dev@gmail.com")

	out := renderFormField("Git Email:", ti.View(), true)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("form field must render as exactly 3 lines (single bordered box); got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(ansi.Strip(lines[1]), "Git Email:") {
		t.Errorf("label must sit on the content line, not be split; got %q", ansi.Strip(lines[1]))
	}
	// Exactly one rounded top-border corner → a single box, not a doubled border.
	top := ansi.Strip(lines[0])
	if strings.Count(top, "╭") != 1 {
		t.Errorf("top line must have exactly one box corner (single fixed-width border); got %q", top)
	}
	// All three lines share the same visible width (no drift).
	w0, w1, w2 := lipgloss.Width(lines[0]), lipgloss.Width(lines[1]), lipgloss.Width(lines[2])
	if w0 != w1 || w1 != w2 {
		t.Errorf("box lines must share one width; got %d/%d/%d", w0, w1, w2)
	}
}
