// Package tui — overlay.go
//
// # PlaceOverlay spike result (Wave-0 D-02 risk resolution)
//
// Command run: go doc charm.land/lipgloss/v2 PlaceOverlay
// Actual output: "doc: no symbol PlaceOverlay in package charm.land/lipgloss/v2"
//
// Conclusion: lipgloss v2.0.3 has NO PlaceOverlay function. Only Place,
// PlaceHorizontal, and PlaceVertical exist. Any call to lipgloss.PlaceOverlay
// would be a compile error. The documented line-by-line string-overlay
// FALLBACK is therefore the only viable approach for modal compositing at this
// pinned version.
//
// Implementation strategy (per UI-SPEC § Modal Overlay D-02):
//  1. Render the full persistent layout and apply StyleDimmed to it (the "bg"
//     string — dimmed background).
//  2. Render the modal box (the "fg" string — foreground overlay).
//  3. Call placeOverlay(x, y, fg, bg) to composite fg over bg at position (x, y).
//
// The compositing algorithm replaces background cells column-by-column using
// lipgloss.Width for ANSI-aware visible-width measurement. It never splits
// multi-byte glyphs or ANSI escape sequences by operating at the line-replacement
// level: each foreground line REPLACES the corresponding background line's
// columns [x, x+fgLineWidth), while background columns outside that span are
// preserved verbatim.
//
// T-05.6-01 (threat): covered by TestPlaceOverlayCompositesModal — asserts modal
// lines appear at centered rows/cols, untouched rows are preserved, and an
// oversized modal clamps without panic.
package tui

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// placeOverlayAvailable documents the spike result as a build-time constant.
// false = lipgloss.PlaceOverlay does NOT exist at v2.0.3; the fallback is required.
const placeOverlayAvailable = false

// placeOverlay composites the foreground string fg (a modal box render) over
// the background string bg (the dimmed full-layout render) at position (x, y).
//
// Algorithm:
//   - Split both fg and bg by "\n" into line slices.
//   - For each foreground line index i, replace background line at row y+i by
//     overwriting the visible columns [x, x+fgLineWidth) with fg line runes,
//     preserving background runes outside that span.
//   - Uses lipgloss.Width for ANSI-aware visible-width measurement so multi-byte
//     glyphs (✓ ✗ ! ~ › ○ •) and ANSI escape sequences in the background do not
//     corrupt the overlay (T-05.6-01).
//   - Rows/columns that would fall outside the background bounds are clamped
//     silently — no panic on oversized modals (T-05.6-02).
func placeOverlay(x, y int, fg, bg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	for i, fgLine := range fgLines {
		bgRow := y + i
		if bgRow < 0 || bgRow >= len(bgLines) {
			// Clamp: modal row falls outside background — skip.
			continue
		}
		bgLines[bgRow] = overlayLine(x, fgLine, bgLines[bgRow])
	}
	return strings.Join(bgLines, "\n")
}

// overlayLine composites fgLine onto bgLine at visible column x, ANSI-aware.
//
// It splices three segments by VISIBLE column (not rune index), so ANSI escape
// sequences and multi-byte glyphs in either line are preserved and never split:
//   - left:  background visible columns [0, x)        — via ansi.Truncate
//   - mid:   the foreground line verbatim (it "wins")  — fgLine
//   - right: background visible columns [x+fgWidth, ∞) — via ansi.TruncateLeft
//
// The previous implementation copied runes at index x, but bgLine carries
// StyleDimmed ANSI escapes, so rune-index ≠ visible-column and the modal landed
// shifted right by the number of escape runes. Slicing by visible width fixes
// that placement bug (P0-1) while keeping the dimmed background on both sides.
func overlayLine(x int, fgLine, bgLine string) string {
	if x < 0 {
		x = 0
	}
	fgWidth := lipgloss.Width(fgLine)
	if fgWidth == 0 {
		return bgLine
	}
	bgWidth := lipgloss.Width(bgLine)

	// Left: background columns [0, x), padded with spaces if bg is shorter.
	left := ansi.Truncate(bgLine, x, "")
	if lw := lipgloss.Width(left); lw < x {
		left += strings.Repeat(" ", x-lw)
	}

	// Right: background columns [x+fgWidth, end); empty if bg ends within the span.
	right := ""
	if bgWidth > x+fgWidth {
		right = ansi.TruncateLeft(bgLine, x+fgWidth, "")
	}

	return left + fgLine + right
}

// modalOrigin returns the centered top-left origin (x, y) for a modal of
// dimensions modalW×modalH centered within a terminal of dimensions termW×termH.
// Both x and y are floored at 0 so the modal never overflows the top-left corner.
func modalOrigin(termW, termH, modalW, modalH int) (x, y int) {
	x = (termW - modalW) / 2
	y = (termH - modalH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

// modalBox renders a styled modal box with the given title and body text.
// The box width is clamped to min(width-8, 72) columns per UI-SPEC § Modal box
// styling. StyleModal (added in styles.go) provides the rounded blue border and
// 1×2 internal padding. StyleModalTitle bolds the title line.
func modalBox(width int, title, body string) string {
	// Clamp modal width: min(width-8, 72); floor at 20 to stay usable.
	mw := width - 8
	if mw > 72 {
		mw = 72
	}
	if mw < 20 {
		mw = 20
	}

	content := StyleModalTitle.Render(title) + "\n\n" + body
	return StyleModal.Width(mw).Render(content)
}
