package dummytui

// theme.go is the central semantic style contract for the live TUI demo,
// mirrored 1:1 by role name with the web
// .planning/design/mockup-src/src/theme.ts role tokens — see
// .planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md for
// the full role table and rationale. frame.go's package-level style vars
// (styleBold/styleFaint/styleHealthy/styleWarning/styleError/styleInfo) are
// promoted to derive from DefaultTheme below — a behavior-preserving
// refactor (TestThemePromotionIsBehaviorPreserving pins byte-identical
// output) so every pre-existing copy-pinning test stays green.
//
// ANSI-16 colors are kept deliberately (not truecolor/adaptive light-dark)
// so NO_COLOR and arbitrary terminal color schemes stay legible — every role
// also carries a glyph or word, per 02-UX-DIRECTION.md §2, never color
// alone.

import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
)

// Theme is the central semantic style contract: one lipgloss.Style per role
// (02-STYLE-SPEC.md's 12-role table), plus the shared accent color the
// focused-field contour and the active-area chrome both key from.
type Theme struct {
	Info         lipgloss.Style
	Label        lipgloss.Style
	Field        lipgloss.Style
	FieldFocused lipgloss.Style
	FieldBlurred lipgloss.Style
	Hint         lipgloss.Style
	Warning      lipgloss.Style
	Error        lipgloss.Style
	Healthy      lipgloss.Style
	Preview      lipgloss.Style
	DisabledNav  lipgloss.Style
	ActiveArea   lipgloss.Style
	// ActiveNav is the ACTIVE header nav tab: the shared accent as a
	// BACKGROUND (not a flat monochrome reverse-video invert), so the
	// current view clearly says "I am at 1/2/3/4" — checkpoint feedback U1.
	// Bold + bright-white (ANSI 15) foreground for contrast on the ANSI-4
	// blue background; still ANSI-16, still paired with the tab's number +
	// word (never color alone).
	ActiveNav lipgloss.Style
	// ActiveNavDimmed is the ACTIVE header nav tab while a pane/form/
	// ceremony captures keys (D4, checkpoint-2 contract): bold + the
	// shared accent as a FOREGROUND, NO background fill — distinct from
	// both the full ActiveNav background treatment (no pane capturing
	// keys) and DisabledNav (an INACTIVE tab while capturing), so the
	// current view stays legible without competing with the dimmed chrome.
	ActiveNavDimmed lipgloss.Style

	// Accent is the ONE shared blue (ANSI 4) accent color — the
	// focused-field contour's border foreground and the active-area chrome
	// both key from this single color (02-STYLE-SPEC.md role table).
	Accent color.Color
	// FieldBorder is an alias of Accent scoped to the focused-field contour
	// (the STYLE-SPEC's own role name for it) — same color, kept distinct so
	// callers can name their intent.
	FieldBorder color.Color
}

// Frozen glyph constants (D3, checkpoint-2 contract) — the ONE source of the
// checkbox/radio glyphs every dummytui render site draws through
// (review-findings F5: identities.go/globalssh.go/globalgit.go previously
// repeated these as inline string literals, risking silent drift between
// screens).
const (
	glyphCheckOn  = "☑"
	glyphCheckOff = "☐"
	glyphRadioOn  = "●"
	glyphRadioOff = "○"
)

// DefaultTheme is the ANSI-16 role palette every dummytui renderer draws
// through.
var DefaultTheme = Theme{
	Info:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	Label: lipgloss.NewStyle().Bold(true),
	Field: lipgloss.NewStyle(),
	// FieldFocused (D1, checkpoint-2 contract): accent foreground + bold,
	// NO border — every field is ONE constant-height row in every state;
	// focus is signalled by color + the redundant `▸` marker, never a
	// reflowing box (renderFocusedFieldBox is deleted).
	FieldFocused: lipgloss.NewStyle().
		Foreground(lipgloss.Color("4")).
		Bold(true),
	FieldBlurred: lipgloss.NewStyle().Faint(true),
	Hint:         lipgloss.NewStyle().Faint(true),
	Warning:      lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
	Error:        lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
	Healthy:      lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
	Preview:      lipgloss.NewStyle().Faint(true),
	DisabledNav:  lipgloss.NewStyle().Faint(true),
	ActiveArea:   lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
	ActiveNav: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("4")),
	ActiveNavDimmed: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4")),
	Accent:      lipgloss.Color("4"),
	FieldBorder: lipgloss.Color("4"),
}
