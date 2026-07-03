// Package dummytui — overlay.go reimplements tui/overlay.go's
// modal-compositing algorithm backend-free, inside package dummytui. It is
// NOT imported from tui/ — tui/ transitively imports internal/doctor,
// internal/identity, etc. via tui/deps.go, which would break the DLV-05
// no-backend import-graph allowlist (see doc.go, nobackend_test.go).
//
// # PlaceOverlay spike result (ported verbatim from tui/overlay.go)
//
// Command run: go doc charm.land/lipgloss/v2 PlaceOverlay
// Actual output: "doc: no symbol PlaceOverlay in package charm.land/lipgloss/v2"
//
// Conclusion: lipgloss v2.0.3 has NO PlaceOverlay function. Only Place,
// PlaceHorizontal, and PlaceVertical exist. The line-by-line string-overlay
// fallback below is the only viable approach for modal compositing at this
// pinned version (RESEARCH.md Pitfall 1).
package dummytui

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// placeOverlay composites the foreground string fg (a modal surface's
// rendered screen) over the background string bg (the dimmed parent shell
// render) at position (x, y).
//
// Algorithm:
//   - Split both fg and bg by "\n" into line slices.
//   - For each foreground line index i, replace background line at row y+i by
//     overwriting the visible columns [x, x+fgLineWidth) with fg line runes,
//     preserving background runes outside that span.
//   - Uses lipgloss.Width for ANSI-aware visible-width measurement so
//     multi-byte glyphs and ANSI escape sequences in the background do not
//     corrupt the overlay.
//   - Rows/columns that would fall outside the background bounds are clamped
//     silently — no panic on oversized modals.
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
// It splices three segments by VISIBLE column (not rune index), so ANSI
// escape sequences and multi-byte glyphs in either line are preserved and
// never split:
//   - left:  background visible columns [0, x)        — via ansi.Truncate
//   - mid:   the foreground line verbatim (it "wins")  — fgLine
//   - right: background visible columns [x+fgWidth, ∞) — via ansi.TruncateLeft
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

// boundModalToViewport clips a rendered modal to the available terminal rows
// so overflowing content is never silently dropped. When the modal height
// exceeds available rows it returns the visible window at the given scroll
// offset with an explicit "↓ more"/"↑ scrolled" indicator on the last
// visible row, ensuring all modal lines are reachable across scroll
// positions. When the modal fits within available rows it returns modal
// unchanged.
//
// scrollOffset is the number of modal lines to skip from the top (0 =
// start). available is the number of terminal rows the modal may occupy
// (must be >= 1).
func boundModalToViewport(modal string, available, scrollOffset int) string {
	if available < 1 {
		available = 1
	}
	lines := strings.Split(modal, "\n")
	total := len(lines)

	// Fits on screen: return unchanged.
	if total <= available {
		return modal
	}

	// Reserve one row for the scroll indicator.
	bodyRows := available - 1
	if bodyRows < 1 {
		bodyRows = 1
	}

	// Clamp scroll offset so we never skip past the last possible window.
	maxOffset := total - bodyRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	if scrollOffset > maxOffset {
		scrollOffset = maxOffset
	}

	end := scrollOffset + bodyRows
	if end > total {
		end = total
	}
	window := lines[scrollOffset:end]

	// Build scroll indicator.
	atTop := scrollOffset == 0
	atBottom := end >= total
	var indicator string
	switch {
	case atTop && !atBottom:
		indicator = "  ↓ more"
	case !atTop && atBottom:
		indicator = "  ↑ scrolled"
	case !atTop && !atBottom:
		indicator = "  ↑↓ scroll"
	default:
		indicator = ""
	}

	return strings.Join(append(window, indicator), "\n")
}
