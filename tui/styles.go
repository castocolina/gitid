package tui

import (
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
)

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
