package tuikit

// frame.go renders the common chrome every view sits inside — the Go
// mirror of .planning/design/mockup-src/src/demo/Frame.tsx per
// 02-REDESIGN-SPEC.md §1 (k9s/lazygit/Textual style):
//
//	header:  brand · numbered nav tabs (1..4, active = accent background) ·
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

// TabID indexes the four primary views (SHELL-01 as redesigned: the Fixer
// is NOT a tab — FIX-02 re-homed it into Doctor).
type TabID int

// The four primary views, in header order.
const (
	TabIdentities TabID = iota
	TabGlobalSSH
	TabGlobalGit
	TabDoctor
)

// tabLabels are the nav tab labels, indexed by TabID.
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

// Shared style tokens — PROMOTED from ad-hoc lipgloss styles to derive from
// DefaultTheme (theme.go); the promotion is behavior-preserving (rendered
// output is byte-identical, pinned by TestThemePromotionIsBehaviorPreserving)
// so every pre-existing copy-pinning test in this package stays green.
// styleReverse/styleSelected/styleSection have no Theme role (they are
// focus/selection treatments, not semantic roles) and stay ad-hoc.
var (
	styleBold     = DefaultTheme.Label
	styleFaint    = DefaultTheme.Hint
	styleReverse  = lipgloss.NewStyle().Reverse(true)
	styleHealthy  = DefaultTheme.Healthy
	styleWarning  = DefaultTheme.Warning
	styleError    = DefaultTheme.Error
	styleInfo     = DefaultTheme.Info
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

// headerTabText is the exact (unstyled) text of nav tab segment i — the
// bracketed `[N] Label` format (D4, checkpoint-2 contract); the bracket
// format previously lived on the wizard stepper (superseded by D5's revert).
func headerTabText(i int) string {
	return fmt.Sprintf(" [%d] %s ", i+1, tabLabels[i])
}

// renderHeader renders the single header row: brand · numbered flat tabs ·
// health chip. Four nav states (D4): the ACTIVE tab with no pane capturing
// keys renders Theme.ActiveNav (the shared accent as a BACKGROUND — bold +
// bright-white — checkpoint feedback U1); the ACTIVE tab while a pane
// captures keys renders the NEW Theme.ActiveNavDimmed (bold + accent
// FOREGROUND, no background fill — the current view stays legible without
// competing with the dimmed chrome); an INACTIVE tab while a pane captures
// keys renders Theme.DisabledNav (faint); otherwise plain
// (02-STYLE-SPEC.md "dim-states").
func renderHeader(width int, s DemoState, active TabID, capturesKeys bool) string {
	segments := make([]string, 0, len(tabLabels))
	for i := range tabLabels {
		text := headerTabText(i)
		switch {
		case TabID(i) == active && capturesKeys:
			segments = append(segments, DefaultTheme.ActiveNavDimmed.Render(text))
		case TabID(i) == active:
			segments = append(segments, DefaultTheme.ActiveNav.Render(text))
		case capturesKeys:
			segments = append(segments, DefaultTheme.DisabledNav.Render(text))
		default:
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
func headerTabAt(x int) (TabID, bool) {
	cursor := ansi.StringWidth(" " + headerBrand + "  ")
	for i := range tabLabels {
		w := ansi.StringWidth(headerTabText(i))
		if x >= cursor && x < cursor+w {
			return TabID(i), true
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
func RenderFrame(width, height int, s DemoState, tab TabID, crumbs []string, status, statusTone string, actions []FooterAction, capturesKeys bool, body string) string {
	if width < minFrameWidth || height < minFrameHeight {
		return fmt.Sprintf("Terminal too small — resize to at least %dx%d", minFrameWidth, minFrameHeight)
	}

	header := renderHeader(width, s, tab, capturesKeys)
	// The breadcrumb line sits directly between the header and the active
	// pane's body — it is the ActiveArea mechanism (02-STYLE-SPEC.md §3):
	// while a pane captures keys it carries the accent color instead of the
	// default faint dim, giving the active region an accent-colored divider
	// without spending an extra row (the row-budget trap, 02-REVIEWS.md).
	crumbStyle := styleFaint
	if capturesKeys {
		crumbStyle = DefaultTheme.ActiveArea
	}
	crumbLine := " " + crumbStyle.Render(ansi.Truncate(strings.Join(append([]string{tabLabels[tab]}, crumbs...), " › "), width-2, "…"))
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

// demoBannerText is the D-16 notice a tab renders when the injected Backend
// reports it is not yet wired to live data. FROZEN COPY, byte-exact: it joins
// the 02-STYLE-SPEC.md §6 copy-freeze grep mechanism (see the
// `gate-copy-freeze` make target and TestDemoBannerCopyIsFrozen), so it may
// not be reworded without re-freezing it there.
//
// The dummy never shows it (the whole dummy IS demo data, so a banner would be
// noise); the real binary shows it on every tab whose backend wiring has not
// landed yet, and each later phase removes it from the ONE view it wires.
const demoBannerText = "Preview — demo data, not wired to your system yet"

// renderDemoBanner is the one-row D-16 banner (02-UI-SPEC.md "Scoped
// Divergences → D-16"): the EXISTING Theme.Warning role plus the EXISTING `!`
// warning glyph plus the word — never color alone, never a new role or glyph.
// RenderFrame draws it directly under the breadcrumb/ActiveArea line as the
// first row of the body region, so it costs chrome, not screen content.
func renderDemoBanner(width int) string {
	return " " + DefaultTheme.Warning.Render("! "+fitLine(demoBannerText, width-3))
}

// fitLine clips text to width display columns (the banner is ASCII copy).
func fitLine(text string, width int) string {
	if width < 1 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width < 2 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
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

// PreviewBlock renders a bounded, optionally-titled monospace config/diff
// preview — faint text plus a dashed dim border, deliberately dimmer than
// editable fields (round-3 feedback; mirror of MutationCeremony.tsx's
// PreviewBlock). In diff mode leading `+` lines render green and `-` lines
// red. If title is non-empty it is carried in the border's TOP EDGE (never a
// separate row — round-4 feedback: a preview block always looks like a
// titled read-only box, never an editable field). If maxLines > 0 the block
// is CLIPPED (overflow → the `… (+n more lines)` cue) OR PADDED (short
// content → blank rows) to exactly maxLines content rows, so the box height
// is STABLE — a preview never auto-shrinks to its content in a way that
// makes it read as an input (02-STYLE-SPEC.md "preview-sizing").
func PreviewBlock(title, text string, diff bool, width, maxLines int) string {
	lines := strings.Split(text, "\n")
	if maxLines > 0 {
		switch {
		case len(lines) > maxLines:
			hidden := len(lines) - maxLines
			lines = append(lines[:maxLines], fmt.Sprintf("… (+%d more lines)", hidden))
		case len(lines) < maxLines:
			for len(lines) < maxLines {
				lines = append(lines, "")
			}
		}
	}
	// PreviewBlock (unlike the shared previewBlockClipped path) fills the
	// box out to the full pane width — "bounded to the pane" means the
	// title always has room in the top border, and the box reads as a
	// stable, fixed-size region rather than shrink-wrapping to content.
	return renderPreviewBoxFilled(title, lines, diff, width)
}

// previewBlockClipped is the pre-existing (untitled, non-padding) preview
// renderer every OTHER screen still calls — an optional maxLines cap
// (0 = unlimited); clipped previews end with a faint "… (+n more lines)".
// Kept byte-identical to preserve every existing copy-pinning test outside
// this task's scope (identities.go/globalssh.go/ceremony.go).
func previewBlockClipped(text string, diff bool, width int, maxLines int) string {
	lines := strings.Split(text, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		hidden := len(lines) - maxLines
		lines = append(lines[:maxLines], fmt.Sprintf("… (+%d more lines)", hidden))
	}
	return renderPreviewBox("", lines, diff, width)
}

// previewBorderColor is the dim gray (ANSI 8) every preview border/title
// renders in — shared by the box border and the spliced title.
var previewBorderColor = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

// renderPreviewBox is the pre-existing preview-box renderer (shrink-wraps to
// its longest content line, never wider than width-4) used by every OTHER
// screen via previewBlockClipped — unchanged so those call sites' rendered
// output (and their pinned tests) stay byte-identical.
func renderPreviewBox(title string, lines []string, diff bool, width int) string {
	inner := width - 4 // border + one space of padding each side
	if inner < 10 {
		inner = 10
	}
	styled := stylePreviewLines(lines, diff, inner)
	block := lipgloss.NewStyle().
		Border(previewDashedBorder).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Render(strings.Join(styled, "\n"))
	if title == "" {
		return block
	}
	return spliceTitleIntoTopBorder(block, title)
}

// renderPreviewBoxFilled is PreviewBlock's own renderer: it additionally
// pads every content line out to inner width (lipgloss.Width) so the
// rendered box always fills the full pane width — "bounded to the pane"
// means both capped AND filled, giving the title-in-border-top-edge room to
// render regardless of how short the content is.
func renderPreviewBoxFilled(title string, lines []string, diff bool, width int) string {
	inner := width - 4 // border + one space of padding each side
	if inner < 10 {
		inner = 10
	}
	styled := stylePreviewLines(lines, diff, inner)
	block := lipgloss.NewStyle().
		Border(previewDashedBorder).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(inner).
		Render(strings.Join(styled, "\n"))
	if title == "" {
		return block
	}
	return spliceTitleIntoTopBorder(block, title)
}

// stylePreviewLines truncates each line to innerWidth and applies the
// diff/faint styling shared by both preview-box renderers.
func stylePreviewLines(lines []string, diff bool, innerWidth int) []string {
	styled := make([]string, 0, len(lines))
	for _, line := range lines {
		line = ansi.Truncate(line, innerWidth, "…")
		switch {
		case diff && strings.HasPrefix(line, "+"):
			styled = append(styled, styleHealthy.Render(line))
		case diff && strings.HasPrefix(line, "-"):
			styled = append(styled, styleError.Render(line))
		default:
			styled = append(styled, styleFaint.Render(line))
		}
	}
	return styled
}

// ---------------------------------------------------------------------------
// ExactTextViewport — byte-preserving, scrollable proof/review viewport.
//
// Unlike PreviewBlock (which truncates with ansi.Truncate), ExactTextViewport
// retains the source string byte-for-byte and computes visible slices without
// ever mutating the stored text. It is used ONLY for create-flow proof and
// confirm review — call sites that previously used PreviewBlock are unchanged.
//
// State is stored by value (immutable update pattern), so the caller (wizardModel)
// holds focus and offset as plain fields and replaces them on key events.
// ---------------------------------------------------------------------------

// ExactTextViewport is a byte-preserving two-axis scrollable window over a
// multi-line text string. It renders exactly VisibleLines content rows
// starting at LineOffset, and each line is displayed starting at
// HorizontalOffset display columns. Truncation is at terminal edge — never
// ansi.Truncate's ellipsis — so every source byte is reachable by scrolling.
//
// A faint range cue on the last rendered row announces remaining hidden lines
// when the content exceeds the window. A column cue is emitted when hidden
// columns exist to the right.
//
// Navigation keys are handled externally by the caller; the viewport exposes
// clamp helpers for both axes.
type ExactTextViewport struct {
	// Text is the complete source content, stored byte-for-byte.
	Text string
	// LineOffset is the first visible line (0-based).
	LineOffset int
	// HorizontalOffset is the first visible column (display-width units, 0-based).
	HorizontalOffset int
	// VisibleLines is the number of content rows to render.
	VisibleLines int
	// Width is the column budget for each rendered line.
	Width int
	// Focused is whether this viewport currently owns key input.
	Focused bool
}

// lines splits the text into individual lines.
func (v ExactTextViewport) lines() []string {
	if v.Text == "" {
		return nil
	}
	return strings.Split(v.Text, "\n")
}

// TotalLines returns the total number of lines in the source text.
func (v ExactTextViewport) TotalLines() int { return len(v.lines()) }

// Clamp returns a copy with LineOffset clamped so the viewport never scrolls
// past the last screenful of content.
func (v ExactTextViewport) Clamp() ExactTextViewport {
	total := v.TotalLines()
	maxOffset := total - v.VisibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.LineOffset > maxOffset {
		v.LineOffset = maxOffset
	}
	if v.LineOffset < 0 {
		v.LineOffset = 0
	}
	maxColumns := v.maxLineWidth()
	maxHorizontalOffset := maxColumns - v.Width
	if maxHorizontalOffset < 0 || v.Width <= 0 {
		maxHorizontalOffset = 0
	}
	if v.HorizontalOffset > maxHorizontalOffset {
		v.HorizontalOffset = maxHorizontalOffset
	}
	if v.HorizontalOffset < 0 {
		v.HorizontalOffset = 0
	}
	return v
}

// ScrollDown moves the viewport down by n lines (clamped).
func (v ExactTextViewport) ScrollDown(n int) ExactTextViewport {
	v.LineOffset += n
	return v.Clamp()
}

// ScrollUp moves the viewport up by n lines (clamped).
func (v ExactTextViewport) ScrollUp(n int) ExactTextViewport {
	v.LineOffset -= n
	return v.Clamp()
}

// ScrollRight moves the horizontal offset right by n display columns (clamped
// to the maximum useful position so the last non-whitespace column is visible).
func (v ExactTextViewport) ScrollRight(n int) ExactTextViewport {
	v.HorizontalOffset += n
	return v.Clamp()
}

// ScrollLeft moves the horizontal offset left by n display columns (clamped to 0).
func (v ExactTextViewport) ScrollLeft(n int) ExactTextViewport {
	v.HorizontalOffset -= n
	return v.Clamp()
}

// AtBottom reports whether the viewport is at or past the last screenful.
func (v ExactTextViewport) AtBottom() bool {
	clamped := v.Clamp()
	return v.LineOffset >= clamped.LineOffset && clamped.LineOffset+clamped.VisibleLines >= v.TotalLines()
}

// maxLineWidth returns the maximum display width of any line in the source text,
// used to determine whether hidden columns exist to the right.
func (v ExactTextViewport) maxLineWidth() int {
	maxWidth := 0
	for _, line := range v.lines() {
		if w := ansi.StringWidth(line); w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

// sliceColumns returns the substring of line starting at startCol display
// columns, up to startCol+maxCols display columns, using rune-by-rune
// display-width counting. This enables horizontal scrolling without corrupting
// multi-byte or wide characters.
func sliceColumns(line string, startCol, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}
	// The viewport source remains byte-for-byte in Text. Rendering strips SGR
	// before slicing because selecting a substring of an escape sequence would
	// corrupt the terminal stream; callers apply presentation styles outside it.
	runes := []rune(ansi.Strip(line))
	// Skip startCol display columns.
	col := 0
	start := 0
	for start < len(runes) && col < startCol {
		col += ansi.StringWidth(string(runes[start]))
		start++
	}
	// Collect up to maxCols display columns.
	w := 0
	end := start
	for end < len(runes) {
		rw := ansi.StringWidth(string(runes[end]))
		if w+rw > maxCols {
			break
		}
		w += rw
		end++
	}
	return string(runes[start:end])
}

// View renders the viewport: exactly VisibleLines rows of content starting at
// LineOffset, each sliced from HorizontalOffset and truncated at Width display
// columns (hard-truncate, no ellipsis — so every source byte is reachable by
// scrolling both vertically and horizontally).
//
// When more content exists below the window, the final row is replaced by a
// faint range cue ("↓ lines N–M of Total  PgDn↓ PgUp↑").
// When hidden columns exist to the right of the current view, a column
// position cue is included in the range line ("→ cols C–C+W of Max  ←→scroll").
// The cue is itself truncated at Width if necessary.
func (v ExactTextViewport) View() string {
	lines := v.lines()
	total := len(lines)
	if v.VisibleLines < 1 || total == 0 {
		return ""
	}
	v = v.Clamp()
	start := v.LineOffset
	end := start + v.VisibleLines
	if end > total {
		end = total
	}
	visible := lines[start:end]
	// Determine whether there are hidden lines below.
	hiddenBelow := total - (start + len(visible))

	// Determine hidden columns (for column cue).
	maxWidth := v.maxLineWidth()
	hiddenRight := maxWidth - (v.HorizontalOffset + v.Width)
	if hiddenRight < 0 {
		hiddenRight = 0
	}

	cueLine := ""
	showHorizontalCue := len(visible) < v.VisibleLines && (hiddenRight > 0 || v.HorizontalOffset > 0)
	if hiddenBelow > 0 || showHorizontalCue {
		// Build a combined navigation cue.
		parts := []string{}
		if hiddenBelow > 0 {
			firstHidden := start + len(visible) + 1
			lastLine := total
			parts = append(parts, fmt.Sprintf("↓ lines %d–%d of %d  PgDn↓ PgUp↑", firstHidden, lastLine, total))
		} else if start > 0 {
			parts = append(parts, "↑ top at line 1  PgUp↑")
		}
		if showHorizontalCue {
			colEnd := v.HorizontalOffset + v.Width
			if colEnd > maxWidth {
				colEnd = maxWidth
			}
			parts = append(parts, fmt.Sprintf("→ cols %d–%d of %d  ←→scroll", v.HorizontalOffset+1, colEnd, maxWidth))
		}
		if len(parts) > 0 {
			cueLine = strings.Join(parts, "  ")
		}
	}

	out := make([]string, 0, v.VisibleLines)
	// Reserve the final row whenever a cue is present. This keeps horizontal-only
	// navigation from growing the viewport beyond its declared row budget.
	contentRows := v.VisibleLines
	if cueLine != "" && v.VisibleLines > 1 {
		contentRows = v.VisibleLines - 1
	} else if v.VisibleLines == 1 {
		// A one-row viewport must remain inspectable; callers with room for a
		// cue use two or more rows and retain the fixed cue reservation above.
		cueLine = ""
	}
	for i, line := range visible {
		if i >= contentRows {
			break
		}
		// Apply horizontal slicing: start at HorizontalOffset, show Width cols.
		if v.HorizontalOffset > 0 || v.Width > 0 {
			w := v.Width
			if w <= 0 {
				w = 0x7FFFFFFF // unlimited
			}
			line = sliceColumns(line, v.HorizontalOffset, w)
		}
		out = append(out, line)
	}
	if cueLine != "" {
		if v.Width > 0 && ansi.StringWidth(cueLine) > v.Width {
			cueLine = ansi.Truncate(cueLine, v.Width, "")
		}
		out = append(out, styleFaint.Render(cueLine))
	}
	for len(out) < v.VisibleLines {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// ViewportControlsHint returns the advertised navigation hint for the footer
// when this viewport has focus. Callers include it in their FooterAction list.
func ViewportControlsHint() []FooterAction {
	return []FooterAction{
		{Key: "PgDn/PgUp", Label: "scroll proof"},
	}
}

// spliceTitleIntoTopBorder rewrites the first (top-border) line of an
// already-rendered preview block to carry " title " between its corners,
// bounded to the block's own width — the title-in-border-top-edge affordance
// (02-STYLE-SPEC.md "preview-sizing"; round-4 feedback).
func spliceTitleIntoTopBorder(block, title string) string {
	lines := strings.Split(block, "\n")
	if len(lines) == 0 {
		return block
	}
	plain := ansi.Strip(lines[0])
	runes := []rune(plain)
	total := len(runes)
	if total < 6 { // corners + at least one dash each side + a 1-char label
		return block
	}
	label := ansi.Truncate(" "+title+" ", total-4, "")
	labelWidth := ansi.StringWidth(label)
	inner := total - 2 // width excluding both corners
	rightDashes := inner - 1 - labelWidth
	if rightDashes < 1 {
		rightDashes = 1
	}
	newTop := string(runes[0]) + string(runes[1]) + label + strings.Repeat(string(runes[1]), rightDashes) + string(runes[total-1])
	newTop = ansi.Truncate(newTop, total, "")
	lines[0] = previewBorderColor.Render(newTop)
	return strings.Join(lines, "\n")
}
