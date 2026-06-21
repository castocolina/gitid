package tui

import (
	"os"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/doctor"
)

// Style tokens for the TUI. All use lipgloss.NewStyle() — NOT renderer.NewStyle()
// which does not exist in lipgloss v2 (RESEARCH.md Pitfall 1, Pattern 6).
// Within a tea.Program, style.Render() inside View() methods renders correctly;
// the program's internal renderer handles color profile detection automatically.
var (
	// StyleTitle renders section titles in bold.
	StyleTitle = lipgloss.NewStyle().Bold(true)
	// StyleHeader renders column headers or section headers in bold.
	StyleHeader = lipgloss.NewStyle().Bold(true)
	// StyleSelected highlights the currently selected list item.
	StyleSelected = lipgloss.NewStyle().Bold(true).Reverse(true)
	// StyleBody is the default body text style (no decorations).
	StyleBody = lipgloss.NewStyle()
	// StyleFaint renders secondary/de-emphasized text.
	StyleFaint = lipgloss.NewStyle().Faint(true)
	// StyleLabel renders field labels at a fixed width for form alignment.
	StyleLabel = lipgloss.NewStyle().Bold(true).Width(16)

	// StylePass renders success/passing indicators in green (ANSI color 2).
	StylePass = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	// StyleFinding is the base style for findings; color is applied per-severity
	// via SeverityStyle (see below).
	StyleFinding = lipgloss.NewStyle()

	// StyleInputActive renders an active (focused) form input with a blue border.
	StyleInputActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("4"))

	// StyleInputInactive renders an inactive (blurred) form input with a dim border.
	StyleInputInactive = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("8"))

	// StyleHelpKey renders key names in the help bar.
	StyleHelpKey = lipgloss.NewStyle().Faint(true).Bold(true)
	// StyleHelpDesc renders key descriptions in the help bar.
	StyleHelpDesc = lipgloss.NewStyle().Faint(true)

	// StylePanel renders a content panel with rounded border and dim foreground.
	StylePanel = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8"))

	// StylePanelFocused renders a focused content panel with a blue border.
	StylePanelFocused = lipgloss.NewStyle().
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("4"))

	// --- New tokens added for Phase 5.6 two-pane + modal architecture ---

	// StyleTabActive renders the active view tab: bold + underline.
	// Source: UI-SPEC § Lipgloss Style Token Reference.
	StyleTabActive = lipgloss.NewStyle().Bold(true).Underline(true)

	// StyleTabInactive renders inactive view tabs: faint.
	StyleTabInactive = lipgloss.NewStyle().Faint(true)

	// StyleReadOnly renders read-only / immutable fields: faint.
	// "Italic" is terminal-dependent and not universally supported; faint is the
	// reliable downgrade (UI-SPEC § Typography, "Faint + Italic (where terminal
	// supports)"). Callers may apply Italic() additionally on capable terminals.
	StyleReadOnly = lipgloss.NewStyle().Faint(true)

	// StyleModalTitle renders the first line inside a modal box: bold.
	StyleModalTitle = lipgloss.NewStyle().Bold(true)

	// StyleModal renders the modal box frame: rounded border + blue border foreground
	// + 1×2 internal padding (UI-SPEC § Modal box styling, D-02).
	StyleModal = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("4"))

	// StyleDimmed dims the behind-modal persistent layout render (D-02).
	StyleDimmed = lipgloss.NewStyle().Faint(true)

	// StyleSidebarSection renders sidebar section labels (Identities, Unmanaged)
	// in bold (UI-SPEC § View 1 Sidebar content).
	StyleSidebarSection = lipgloss.NewStyle().Bold(true)

	// StyleSidebarItem renders a non-selected sidebar identity row (default fg).
	StyleSidebarItem = lipgloss.NewStyle()

	// StyleSidebarUnmanaged renders unmanaged sidebar entries: faint (read-only
	// signal per D-12/D-13).
	StyleSidebarUnmanaged = lipgloss.NewStyle().Faint(true)

	// StyleSidebarBadge is the base style for per-identity health badges in
	// sidebar rows. Severity color is applied per-call via SeverityStyle.
	StyleSidebarBadge = lipgloss.NewStyle()
)

// formFieldWidth is the fixed content width (inside the border) of every
// inline-edit and wizard input box. Inputs are given this width explicitly
// (textinput.SetWidth) so each bordered box is exactly one known width and
// never drifts with content length — the fix for the phantom-box render bug
// where width-less inputs produced offset, mismatched borders (P0-1).
const formFieldWidth = 32

// renderFormField renders a label and a single, fixed-width bordered input box,
// vertically centering the label against the (3-line) box so the border aligns
// cleanly beside the label. focused selects the border color (active = blue,
// inactive = dim). inputView is the already-rendered textinput.View() (which
// MUST have been given an explicit width via SetWidth(formFieldWidth)).
func renderFormField(label, inputView string, focused bool) string {
	box := StyleInputInactive
	if focused {
		box = StyleInputActive
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		StyleLabel.Render(label),
		" ",
		box.Render(inputView),
	)
}

// SeverityStyle returns a lipgloss foreground style for the given severity.
// Mirrors severityCode() in cmd/gitid/doctor.go but uses lipgloss style values
// instead of raw ANSI codes (UI-SPEC color tokens):
//   - critical/error → red (ANSI 1)
//   - warning        → yellow (ANSI 3)
//   - info           → cyan (ANSI 6)
func SeverityStyle(s doctor.Severity) lipgloss.Style {
	switch s {
	case doctor.SeverityCritical, doctor.SeverityError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	case doctor.SeverityWarning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	default: // SeverityInfo
		return lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	}
}

// SeverityGlyph returns the display glyph for a doctor.Severity level with
// an ASCII fallback for degraded terminals (D-10, UI-SPEC § Glyph Contract).
//
// Severity → glyph mapping (LOCKED by UI-SPEC Eval #2 / D-10):
//   - SeverityWarning → "!" (NEVER "✗" — must be visually distinct from error)
//   - SeverityError / SeverityCritical → "✗"
//   - SeverityInfo → "~"
//   - pass (zero/unknown) → "✓"
//
// ASCII fallbacks (when asciiMode() returns true):
//   - SeverityError / SeverityCritical → "FAIL"
//   - SeverityWarning → "!"
//   - SeverityInfo → "i"
//   - pass → "OK"
func SeverityGlyph(s doctor.Severity, ascii bool) string {
	switch s {
	case doctor.SeverityCritical, doctor.SeverityError:
		if ascii {
			return "FAIL"
		}
		return "✗"
	case doctor.SeverityWarning:
		if ascii {
			return "!"
		}
		return "!"
	case doctor.SeverityInfo:
		if ascii {
			return "i"
		}
		return "~"
	default:
		if ascii {
			return "OK"
		}
		return "✓"
	}
}

// asciiMode reports true when the terminal does not support UTF-8 glyphs.
// Glyph degradation gate: $TERM == "dumb" or $TERM is unset.
// Non-TTY piped output does NOT degrade glyphs (consistent with Phase 4 / UI-SPEC
// § Glyph selection rule: "use UTF-8 glyphs unless $TERM == 'dumb' or $TERM is
// unset. Non-TTY piped output does NOT degrade glyphs").
func asciiMode() bool {
	term := os.Getenv("TERM")
	return term == "" || term == "dumb"
}
