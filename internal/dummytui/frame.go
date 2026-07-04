package dummytui

// frame.go renders the common chrome every view sits inside — the Go
// mirror of .planning/design/mockup-src/src/demo/Frame.tsx per
// 02-REDESIGN-SPEC.md §1 (k9s/lazygit/Textual style):
//
//	header:  brand · numbered nav tabs (1..4, active = reverse video) ·
//	         right health chip (`N ids · ! w · ✗ e`, `✓ ok` when clean)
//	subline: thin faint breadcrumb ("Identities › New identity › Test")
//	body:    the view's own master-detail content
//	status:  transient feedback line
//	footer:  CONTEXTUAL actions only + the reserved keys
//	         (Enter · Esc · ? · Ctrl+P · q) — never navigation, never vim.
//
// All functions here are pure renderers over lipgloss/v2 — no state, no
// I/O — so unit tests can assert the chrome without a terminal.

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Design-target minimum geometry (100x30) — the demo renders adaptively
// above it and shows a plain guard below it.
const (
	minFrameWidth  = 100
	minFrameHeight = 30
)

// Frame chrome geometry — shared by RenderFrame and the mouse hit-testing
// in app.go so click routing can never drift from what is drawn.
const (
	// frameBodyTop is how many rows render above the body (header + crumb).
	frameBodyTop = 2
	// frameChromeBelow is how many rows render below the body
	// (status + contextual footer + reserved footer).
	frameChromeBelow = 3
)

// masterListWidth is the master-list column of every 44/56 master-detail
// screen (Doctor, Global SSH options, Global Git options) — shared by the
// renderers and their click hit-testing.
func masterListWidth(width int) int { return width * 44 / 100 }

// masterDetailGutter is how many columns joinMasterDetail spends between the
// master column and the detail pane: the │ divider plus one space of right
// gutter so wrapped detail lines never butt against the divider (UX
// re-verification R1). Every detail-pane width and click hit-test derives
// from this same constant.
const masterDetailGutter = 2

// frameBodyRows is how many body rows RenderFrame gives a view at height.
func frameBodyRows(height int) int { return height - frameBodyTop - frameChromeBelow }

// tabID indexes the four primary views (SHELL-01 as redesigned: the Fixer
// is NOT a tab — FIX-02 re-homed it into Doctor).
type tabID int

// The four primary views, in header order.
const (
	tabIdentities tabID = iota
	tabGlobalSSH
	tabGlobalGit
	tabDoctor
)

// tabLabels are the nav tab labels, indexed by tabID.
var tabLabels = [...]string{"Identities", "Global SSH", "Global Git", "Doctor"}

// FooterAction is one contextual footer hint (key + label).
type FooterAction struct {
	Key   string
	Label string
}

// reservedFooter is the always-present reserved key set (spec §1) —
// contextual actions render before it, navigation never renders at all.
var reservedFooter = []FooterAction{
	{Key: "Enter", Label: "activate"},
	{Key: "Esc", Label: "back"},
	{Key: "?", Label: "help"},
	{Key: "Ctrl+P", Label: "palette"},
	{Key: "q", Label: "quit"},
}

// reservedFooterInput replaces the reserved keys while the active pane
// state captures plain keys — text inputs, selects, test/ceremony states,
// and choosers all swallow `q` and `?`, so advertising them would lie
// (review batch 2 L1; batch 3 follow-up extends this beyond text inputs).
// Esc and Ctrl+P still work from inside any of those states.
var reservedFooterInput = []FooterAction{
	{Key: "Esc", Label: "back"},
	{Key: "Ctrl+P", Label: "palette"},
}

// Shared style tokens. lipgloss.NewStyle() per v2 (renderer.NewStyle does
// not exist); colors are basic ANSI so NO_COLOR/terminal themes degrade
// legibly — every meaning is also carried by a glyph + word.
var (
	styleBold     = lipgloss.NewStyle().Bold(true)
	styleFaint    = lipgloss.NewStyle().Faint(true)
	styleReverse  = lipgloss.NewStyle().Reverse(true)
	styleHealthy  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleWarning  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleInfo     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleSelected = lipgloss.NewStyle().Bold(true).Reverse(true)
	styleSection  = lipgloss.NewStyle().Bold(true).Underline(true)
)

// sectionHeader renders a detail-pane group heading — the terminal
// stand-in for the web's outlined Paper section cards (review batch 2,
// H2): bold + underlined so SSH / Git / Findings read as separate groups.
func sectionHeader(text string) string {
	return " " + styleSection.Render(text)
}

// joinMasterDetail joins a master column and its detail pane with the
// full-height vertical divider every master-detail screen shares (review
// batch 2, H2 — the web outlines both panes as Paper cards). The divider
// column renders `│ ` — divider plus one space of right gutter so wrapped
// detail continuation lines never butt against the divider (R1); the pair
// occupies exactly masterDetailGutter columns (leftWidth + 2 + detail), the
// same budget every detail-width computation and click hit-test uses. rows
// is the divider height — the body rows the master-detail region occupies.
func joinMasterDetail(left string, leftWidth int, detail string, rows int) string {
	if rows < 1 {
		rows = 1
	}
	leftCol := lipgloss.NewStyle().Width(leftWidth).Height(rows).Render(left)
	div := make([]string, rows)
	for i := range div {
		div[i] = styleFaint.Render("│") + " "
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, strings.Join(div, "\n"), detail)
}

// dimPane re-renders an already-styled block faint, line by line — the ONE
// dim treatment for a master list while a form/ceremony pane owns the keys
// (web: opacity 0.75), shared by the Identities sidebar and the Doctor
// findings list (review batch 2, L3).
func dimPane(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = styleFaint.Render(ansi.Strip(line))
	}
	return strings.Join(lines, "\n")
}

// fitPane clips a rendered pane to maxLines, replacing the overflow with a
// visible faint "… (+n more lines)" cue — free prose is never silently cut
// mid-sentence (review batch 2, H3; preview blocks already carry the same
// cue via previewBlockClipped).
func fitPane(pane string, maxLines int) string {
	if maxLines < 2 {
		return pane
	}
	lines := strings.Split(strings.TrimRight(pane, "\n"), "\n")
	if len(lines) <= maxLines {
		return pane
	}
	hidden := len(lines) - (maxLines - 1)
	lines = append(lines[:maxLines-1], " "+styleFaint.Render(fmt.Sprintf("… (+%d more lines)", hidden)))
	return strings.Join(lines, "\n")
}

// toneStyle maps an identity health tone (IdentityManagerStateTone) to its
// color style.
func toneStyle(tone string) lipgloss.Style {
	switch tone {
	case "success":
		return styleHealthy
	case "warning":
		return styleWarning
	default:
		return styleError
	}
}

// severityStyle maps a health severity to its LOCKED color: ~ info cyan ·
// ! warning yellow · ✗ error/critical red (the word disambiguates).
func severityStyle(severity HealthSeverity) lipgloss.Style {
	switch severity {
	case SeverityInfo:
		return styleInfo
	case SeverityWarning:
		return styleWarning
	case SeverityError, SeverityCritical:
		return styleError
	default:
		return styleError
	}
}

// severityLabel renders the glyph + WORD pair for a severity, colored per
// the locked contract (never a glyph or color alone).
func severityLabel(severity HealthSeverity) string {
	return severityStyle(severity).Render(HealthSeverityGlyph[severity] + " " + string(severity))
}

// healthChip renders the header's right-aligned live health chip:
// `N ids · ✓ ok` when clean, else `N ids · ! w ✗ e`.
func healthChip(s DemoState) string {
	counts := CountFindings(s)
	prefix := fmt.Sprintf("%d ids", len(s.Identities)) + styleFaint.Render(" · ")
	if counts.Warnings+counts.Errors == 0 {
		return prefix + styleHealthy.Render("✓ ok")
	}
	return prefix +
		styleWarning.Render(fmt.Sprintf("! %d", counts.Warnings)) + " " +
		styleError.Render(fmt.Sprintf("✗ %d", counts.Errors))
}

// Header composition shared by renderHeader and the click zones below —
// hit-testing derives every span from the exact strings the header renders.
const (
	headerBrand        = "gitid"
	headerTabSeparator = "·"
)

// headerTabText is the exact (unstyled) text of nav tab segment i.
func headerTabText(i int) string {
	return fmt.Sprintf(" %d %s ", i+1, tabLabels[i])
}

// renderHeader renders the single header row: brand · numbered flat tabs
// (active reverse-video, the number part of the label) · health chip.
func renderHeader(width int, s DemoState, active tabID) string {
	segments := make([]string, 0, len(tabLabels))
	for i := range tabLabels {
		text := headerTabText(i)
		if tabID(i) == active {
			segments = append(segments, styleReverse.Render(text))
		} else {
			segments = append(segments, text)
		}
	}
	left := " " + styleBold.Render(headerBrand) + "  " + strings.Join(segments, styleFaint.Render(headerTabSeparator))
	chip := healthChip(s) + " "
	pad := width - ansi.StringWidth(left) - ansi.StringWidth(chip)
	if pad < 1 {
		pad = 1
	}
	return ansi.Truncate(left+strings.Repeat(" ", pad)+chip, width, "")
}

// headerTabAt resolves which nav tab label covers header-row column x,
// deriving each span from the same segment strings renderHeader renders.
func headerTabAt(x int) (tabID, bool) {
	cursor := ansi.StringWidth(" " + headerBrand + "  ")
	for i := range tabLabels {
		w := ansi.StringWidth(headerTabText(i))
		if x >= cursor && x < cursor+w {
			return tabID(i), true
		}
		cursor += w + ansi.StringWidth(headerTabSeparator)
	}
	return 0, false
}

// headerChipAt reports whether header-row column x falls on the health
// chip — the same right-aligned string (plus trailing space) renderHeader
// places.
func headerChipAt(width int, s DemoState, x int) bool {
	chip := healthChip(s) + " "
	return x >= width-ansi.StringWidth(chip) && x < width
}

// renderFooterLine renders one footer keybar line (bold key + faint label
// pairs joined with faint dots).
func renderFooterLine(width int, actions []FooterAction) string {
	parts := make([]string, 0, len(actions))
	for _, a := range actions {
		parts = append(parts, styleBold.Render(a.Key)+" "+styleFaint.Render(a.Label))
	}
	return ansi.Truncate(" "+strings.Join(parts, styleFaint.Render(" · ")), width, "…")
}

// footerActionAt resolves which footer action covers column x on a keybar
// line, deriving each `<key> <label>` span from the exact strings
// renderFooterLine renders (spec §7 — footer hints are real buttons in the
// web demo, Frame.tsx onActivate).
func footerActionAt(actions []FooterAction, x int) (FooterAction, bool) {
	cursor := 1 // the leading space
	for _, a := range actions {
		w := ansi.StringWidth(a.Key + " " + a.Label)
		if x >= cursor && x < cursor+w {
			return a, true
		}
		cursor += w + ansi.StringWidth(" · ")
	}
	return FooterAction{}, false
}

// blockLine returns the plain (ANSI-stripped) text of line y inside a
// rendered block, or false when the block has no such line.
func blockLine(block string, y int) (string, bool) {
	lines := strings.Split(block, "\n")
	if y < 0 || y >= len(lines) {
		return "", false
	}
	return ansi.Strip(lines[y]), true
}

// needleSpan locates needle inside a plain line and returns its display-cell
// span [start, start+width). Spans are derived from rendered text so click
// zones can never drift from what is drawn (batch-1 pattern).
func needleSpan(plainLine, needle string) (start, width int, ok bool) {
	idx := strings.Index(plainLine, needle)
	if idx < 0 {
		return 0, 0, false
	}
	return ansi.StringWidth(plainLine[:idx]), ansi.StringWidth(needle), true
}

// hitNeedle reports whether cell (x, y) — coordinates relative to the
// block's top-left — falls on the first occurrence of needle on that line
// of the rendered (possibly ANSI-styled and Width-wrapped) block.
func hitNeedle(block string, x, y int, needle string) bool {
	line, ok := blockLine(block, y)
	if !ok {
		return false
	}
	start, width, ok := needleSpan(line, needle)
	return ok && x >= start && x < start+width
}

// statusToneStyle maps a status tone name to its style.
func statusToneStyle(tone string) lipgloss.Style {
	switch tone {
	case "warning":
		return styleWarning
	case "error":
		return styleError
	case "success":
		return styleHealthy
	default:
		return styleFaint
	}
}

// RenderFrame composes the full §1 chrome around body at width x height:
// header row, faint breadcrumb line, the body (clipped/padded to fit), a
// transient status line, and the two footer keybar lines (contextual
// actions, then the reserved keys — the honest variant while the pane
// captures plain keys). Pure function — safe to unit test.
func RenderFrame(width, height int, s DemoState, tab tabID, crumbs []string, status, statusTone string, actions []FooterAction, capturesKeys bool, body string) string {
	if width < minFrameWidth || height < minFrameHeight {
		return fmt.Sprintf("Terminal too small — resize to at least %dx%d", minFrameWidth, minFrameHeight)
	}

	header := renderHeader(width, s, tab)
	crumbLine := " " + styleFaint.Render(ansi.Truncate(strings.Join(append([]string{tabLabels[tab]}, crumbs...), " › "), width-2, "…"))
	statusLine := " " + statusToneStyle(statusTone).Render(ansi.Truncate(status, width-2, "…"))
	footerContextual := renderFooterLine(width, actions)
	reserved := reservedFooter
	if capturesKeys {
		reserved = reservedFooterInput
	}
	footerReserved := renderFooterLine(width, reserved)

	bodyHeight := frameBodyRows(height)
	lines := strings.Split(body, "\n")
	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "")
	}
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}

	rows := make([]string, 0, height)
	rows = append(rows, header, crumbLine)
	rows = append(rows, lines...)
	rows = append(rows, statusLine, footerContextual, footerReserved)
	return strings.Join(rows, "\n")
}

// PreviewLabel renders the label of a preview area — deliberately DIMMER
// (faint) than field labels so read-only previews never read as editable
// inputs (round-3 feedback; mirror of MutationCeremony.tsx's PreviewLabel).
func PreviewLabel(text string) string {
	return styleFaint.Render(text)
}

// previewDashedBorder is the dashed border that visually distinguishes
// read-only preview blocks from editable fields (round-3 feedback).
var previewDashedBorder = lipgloss.Border{
	Top:         "╌",
	Bottom:      "╌",
	Left:        "┊",
	Right:       "┊",
	TopLeft:     "╭",
	TopRight:    "╮",
	BottomLeft:  "╰",
	BottomRight: "╯",
}

// PreviewBlock renders a monospace config/diff preview — faint text plus a
// dashed dim border, deliberately dimmer than editable fields (round-3
// feedback; mirror of MutationCeremony.tsx's PreviewBlock). In diff mode
// leading `+` lines render green and `-` lines red.
func PreviewBlock(text string, diff bool, width int) string {
	return previewBlockClipped(text, diff, width, 0)
}

// previewBlockClipped is PreviewBlock with an optional maxLines cap
// (0 = unlimited); clipped previews end with a faint "… (+n more lines)".
func previewBlockClipped(text string, diff bool, width int, maxLines int) string {
	lines := strings.Split(text, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		hidden := len(lines) - maxLines
		lines = append(lines[:maxLines], fmt.Sprintf("… (+%d more lines)", hidden))
	}
	inner := width - 4 // border + one space of padding each side
	if inner < 10 {
		inner = 10
	}
	styled := make([]string, 0, len(lines))
	for _, line := range lines {
		line = ansi.Truncate(line, inner, "…")
		switch {
		case diff && strings.HasPrefix(line, "+"):
			styled = append(styled, styleHealthy.Render(line))
		case diff && strings.HasPrefix(line, "-"):
			styled = append(styled, styleError.Render(line))
		default:
			styled = append(styled, styleFaint.Render(line))
		}
	}
	block := lipgloss.NewStyle().
		Border(previewDashedBorder).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Render(strings.Join(styled, "\n"))
	return block
}
