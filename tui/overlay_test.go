package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
