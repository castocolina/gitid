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
)

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

// renderHeader renders the single header row: brand · numbered flat tabs
// (active reverse-video, the number part of the label) · health chip.
func renderHeader(width int, s DemoState, active tabID) string {
	segments := make([]string, 0, len(tabLabels))
	for i, label := range tabLabels {
		text := fmt.Sprintf(" %d %s ", i+1, label)
		if tabID(i) == active {
			segments = append(segments, styleReverse.Render(text))
		} else {
			segments = append(segments, text)
		}
	}
	left := " " + styleBold.Render("gitid") + "  " + strings.Join(segments, styleFaint.Render("·"))
	chip := healthChip(s) + " "
	pad := width - ansi.StringWidth(left) - ansi.StringWidth(chip)
	if pad < 1 {
		pad = 1
	}
	return ansi.Truncate(left+strings.Repeat(" ", pad)+chip, width, "")
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
// actions, then the reserved keys). Pure function — safe to unit test.
func RenderFrame(width, height int, s DemoState, tab tabID, crumbs []string, status, statusTone string, actions []FooterAction, body string) string {
	if width < minFrameWidth || height < minFrameHeight {
		return fmt.Sprintf("Terminal too small — resize to at least %dx%d", minFrameWidth, minFrameHeight)
	}

	header := renderHeader(width, s, tab)
	crumbLine := " " + styleFaint.Render(ansi.Truncate(strings.Join(append([]string{tabLabels[tab]}, crumbs...), " › "), width-2, "…"))
	statusLine := " " + statusToneStyle(statusTone).Render(ansi.Truncate(status, width-2, "…"))
	footerContextual := renderFooterLine(width, actions)
	footerReserved := renderFooterLine(width, reservedFooter)

	bodyHeight := height - 5 // header + crumb + status + 2 footer lines
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
		Width(inner + 2).
		Render(strings.Join(styled, "\n"))
	return block
}
