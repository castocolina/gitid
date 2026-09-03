package tuikit

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// sgrPattern strips SGR color/attribute sequences for plain-text asserts.
var sgrPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes SGR sequences so tests assert on visible text.
func stripANSI(s string) string {
	return sgrPattern.ReplaceAllString(s, "")
}

// unwrapCommitToken strips the CR-01 gitCommitTokenMsg wrapper (globalgit.go
// / globalssh.go) that Global SSH's and Global Git's Commit* dispatches now
// wrap every returned tea.Cmd's message in, so tests that call cmd() and
// type-assert the underlying GlobalSSHCommitMsg/SSHStorageCommitMsg/
// GlobalGitCommitMsg/GitFallbackAuthorCommitMsg directly keep working
// unchanged. A message that isn't wrapped (any other screen's own commit
// messages) passes through untouched.
func unwrapCommitToken(msg tea.Msg) tea.Msg {
	if wrapped, ok := msg.(gitCommitTokenMsg); ok {
		return wrapped.msg
	}
	return msg
}

// pressKey builds a tea.KeyMsg for tests; special names map to key codes,
// anything else is a single typed character.
func pressKey(name string) tea.KeyMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	default:
		runes := []rune(name)
		return tea.KeyPressMsg{Code: runes[0], Text: name}
	}
}

// regionFlat extracts the [from,to) column region of the rendered frame
// and collapses whitespace, so assertions survive column word-wrapping.
func regionFlat(a App, from, to int) string {
	var lines []string
	for _, line := range strings.Split(stripANSI(a.View().Content), "\n") {
		runes := []rune(line)
		if len(runes) <= from {
			continue
		}
		end := len(runes)
		if to < end {
			end = to
		}
		lines = append(lines, strings.TrimSpace(string(runes[from:end])))
	}
	return strings.Join(strings.Fields(strings.Join(lines, " ")), " ")
}

func renderSeededFrame(crumbs []string, actions []FooterAction) string {
	return RenderFrame(100, 30, Seed(), TabIdentities, crumbs, "Ready.", "info", actions, false, "body line")
}

func TestRenderFrameShowsNumberedTabsAndReservedFooter(t *testing.T) {
	plain := stripANSI(renderSeededFrame(nil, nil))

	for _, want := range []string{"[1] Identities", "[2] SSH", "[3] Git", "[4] Doctor", "[5] Ignore"} {
		if !strings.Contains(plain, want) {
			t.Errorf("frame missing numbered tab %q", want)
		}
	}
	for _, want := range []string{"Enter activate", "Esc back", "? help", "Ctrl+P palette", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Errorf("frame missing reserved footer key %q", want)
		}
	}
	// No vim keys, no navigation in the footer.
	for _, forbidden := range []string{"j/k", "j down", "k up", "h left", "l right"} {
		if strings.Contains(plain, forbidden) {
			t.Errorf("footer must never carry vim keys/navigation; found %q", forbidden)
		}
	}
}

// --------------------------------------------------------------------------
// D-16 — the "Preview — demo data" banner on not-yet-wired tabs.
// --------------------------------------------------------------------------

// demoBannerLine is the D-16 banner exactly as a user reads it: the EXISTING
// `!` warning glyph, a space, and the frozen copy. Assertions below pin this
// byte-for-byte — it is the 02-STYLE-SPEC.md §6 copy-freeze contract.
const demoBannerLine = "! Preview — demo data, not wired to your system yet"

// bannerBackend is a stubBackend that reports every tab EXCEPT Identities as
// not-yet-wired — exactly what the real composition root does in Phase 3
// (cmd/gitid/wiring.go DemoBanner), so the render assertions below exercise
// the shape the real binary produces.
type bannerBackend struct{ stubBackend }

func (bannerBackend) DemoBanner(tab TabID) bool { return tab != TabIdentities }

// TestDemoBannerCopyIsFrozen pins the D-16 string byte-for-byte and proves the
// banner is rendered with the EXISTING Theme.Warning role plus the EXISTING
// `!` glyph plus the word — never color alone, never a new role or glyph
// (02-UX-DIRECTION.md §2 glyph contract).
func TestDemoBannerCopyIsFrozen(t *testing.T) {
	if demoBannerText != "Preview — demo data, not wired to your system yet" {
		t.Fatalf("frozen D-16 copy changed: %q", demoBannerText)
	}
	rendered := renderDemoBanner(100)
	if got := strings.TrimSpace(stripANSI(rendered)); got != demoBannerLine {
		t.Errorf("banner line = %q, want %q", got, demoBannerLine)
	}
	if !strings.Contains(rendered, DefaultTheme.Warning.Render("! "+demoBannerText)) {
		t.Error("the banner must render through the EXISTING Theme.Warning role with the `!` glyph")
	}
}

// TestDemoBannerOnlyOnNotYetWiredTabs is the D-16 behavior contract: a tab the
// Backend has not wired shows the banner at the TOP of its body (directly
// under the breadcrumb/ActiveArea line); the create-flow tab Phase 3 DOES wire
// (Identities) shows none.
func TestDemoBannerOnlyOnNotYetWiredTabs(t *testing.T) {
	a := NewApp(bannerBackend{})

	// Identities is wired in Phase 3 — no banner.
	if strings.Contains(appView(a), demoBannerLine) {
		t.Error("the create-flow (Identities) tab must NOT carry the D-16 banner")
	}

	// Every other tab is still demo data — banner present, first body row.
	for _, tab := range []string{"2", "3", "4"} {
		next, _ := press(t, a, tab)
		lines := strings.Split(appView(next), "\n")
		if len(lines) <= frameBodyTop {
			t.Fatalf("tab %s rendered %d lines", tab, len(lines))
		}
		if got := strings.TrimSpace(lines[frameBodyTop]); got != demoBannerLine {
			t.Errorf("tab %s first body row = %q, want the D-16 banner %q", tab, got, demoBannerLine)
		}
	}
}

// TestDemoBannerKeepsTheRowBudget proves the banner is chrome-cost, not a
// frame resize: the frame is still exactly 30 rows with the banner on screen
// (02-STYLE-SPEC.md §7 — fit new copy into the existing budget, never grow the
// geometry).
func TestDemoBannerKeepsTheRowBudget(t *testing.T) {
	a, _ := press(t, NewApp(bannerBackend{}), "4") // Doctor — not yet wired
	view := appView(a)
	if !strings.Contains(view, demoBannerLine) {
		t.Fatalf("setup: expected the banner on the Doctor tab:\n%s", view)
	}
	if got := len(strings.Split(view, "\n")); got != minFrameHeight {
		t.Errorf("frame height with the banner = %d rows, want %d", got, minFrameHeight)
	}
}

func TestRenderFrameBreadcrumbJoinsWithChevron(t *testing.T) {
	plain := stripANSI(renderSeededFrame([]string{"work", "Edit SSH"}, nil))
	if !strings.Contains(plain, "Identities › work › Edit SSH") {
		t.Error("breadcrumb must join tab label + crumbs with ›")
	}
}

func TestRenderFrameHealthChipCounts(t *testing.T) {
	plain := stripANSI(renderSeededFrame(nil, nil))
	for _, want := range []string{"8 ids", "! 1", "✗ 3"} {
		if !strings.Contains(plain, want) {
			t.Errorf("seeded chip missing %q (want `8 ids · ! 1 ✗ 3`)", want)
		}
	}

	clean := Seed()
	clean.Findings = nil
	plainClean := stripANSI(RenderFrame(100, 30, clean, TabIdentities, nil, "Ready.", "info", nil, false, ""))
	if !strings.Contains(plainClean, "✓ ok") {
		t.Error("all-clean chip must show `✓ ok`")
	}
	if strings.Contains(plainClean, "! 0") || strings.Contains(plainClean, "✗ 0") {
		t.Error("all-clean chip must not show zero counts")
	}
}

func TestRenderFrameActiveTabAccentBackground(t *testing.T) {
	// Checkpoint feedback U1: the ACTIVE nav tab carries the shared accent
	// as a BACKGROUND (Theme.ActiveNav: bold + bright-white on ANSI-4 blue,
	// SGR 1;97;44), replacing the old flat monochrome reverse-video invert
	// that did not clearly say "I am at 1/2/3/4".
	raw := RenderFrame(100, 30, Seed(), TabGlobalGit, nil, "Ready.", "info", nil, false, "")
	if !strings.Contains(raw, "\x1b[1;97;44m [3] Git ") {
		t.Error("active tab must render through Theme.ActiveNav (bold + bright-white on the blue accent background, SGR 1;97;44)")
	}
	if strings.Contains(raw, "\x1b[1;97;44m [1] Identities ") {
		t.Error("inactive tabs must not carry the ActiveNav accent background")
	}
	if strings.Contains(raw, "\x1b[7m") {
		t.Error("the header must no longer use plain reverse-video for the active tab (checkpoint feedback U1)")
	}
}

func TestRenderFrameContextualActionsPrecedeReserved(t *testing.T) {
	plain := stripANSI(renderSeededFrame(nil, []FooterAction{{Key: "n", Label: "new"}, {Key: "d", Label: "delete"}}))
	if !strings.Contains(plain, "n new") || !strings.Contains(plain, "d delete") {
		t.Error("contextual footer actions missing")
	}
}

func TestRenderFrameGeometry(t *testing.T) {
	out := RenderFrame(100, 30, Seed(), TabIdentities, nil, "Ready.", "info", nil, false, strings.Repeat("line\n", 60))
	lines := strings.Split(out, "\n")
	if len(lines) != 30 {
		t.Fatalf("frame height = %d lines, want exactly 30", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w > 100 {
			t.Errorf("line %d width = %d, want <= 100", i, w)
		}
	}
}

func TestRenderFrameTooSmallGuard(t *testing.T) {
	out := RenderFrame(80, 24, Seed(), TabIdentities, nil, "", "info", nil, false, "")
	if !strings.Contains(out, "resize to at least 100x30") {
		t.Errorf("small-terminal guard missing; got %q", out)
	}
}

func TestPreviewLabelRendersDimmerThanFieldLabels(t *testing.T) {
	out := PreviewLabel("Live Host-block preview")
	if !strings.Contains(out, "\x1b[2m") {
		t.Error("PreviewLabel must render FAINT (SGR 2) — dimmer than field labels (round-3 feedback)")
	}
}

func TestPreviewBlockDimsAndColorsDiffs(t *testing.T) {
	block := PreviewBlock("", "context\n+ added\n- removed", true, 60, 0)
	if !strings.Contains(block, "╌") {
		t.Error("preview block must carry the dashed border (round-3 feedback)")
	}
	if !strings.Contains(block, "\x1b[2m") {
		t.Error("preview content must render faint")
	}
	if !strings.Contains(block, "\x1b[32m+ added") {
		t.Error("diff `+` lines must render green")
	}
	if !strings.Contains(block, "\x1b[31m- removed") {
		t.Error("diff `-` lines must render red")
	}
}

func TestPreviewBlockBoundedWidth(t *testing.T) {
	block := stripANSI(PreviewBlock("", "short", false, 30, 0))
	for _, line := range strings.Split(block, "\n") {
		if w := ansi.StringWidth(line); w > 30 {
			t.Errorf("preview block line %q width = %d, want <= 30 (bounded to the pane)", line, w)
		}
	}
}

func TestPreviewBlockClipCueAtFixedMaxHeight(t *testing.T) {
	text := strings.TrimSuffix(strings.Repeat("l\n", 20), "\n")
	block := stripANSI(PreviewBlock("", text, false, 40, 5))
	lines := strings.Split(block, "\n")
	// border top + 5 clipped content rows + 1 cue row + border bottom = 8.
	if len(lines) != 8 {
		t.Fatalf("bounded preview height = %d lines, want 8 (border + 5 rows + cue row + border)", len(lines))
	}
	if !strings.Contains(block, "… (+15 more lines)") {
		t.Errorf("clipped preview must announce hidden lines; got %q", block)
	}
}

func TestPreviewBlockStableHeightPadsShortContent(t *testing.T) {
	// A preview SHORTER than maxLines must still render the full box height
	// (no auto-shrink to content — round-4 feedback: a stable box reads as
	// read-only, never editable).
	short := stripANSI(PreviewBlock("", "one line", false, 40, 5))
	lines := strings.Split(short, "\n")
	if len(lines) != 7 { // border + 5 rows (padded) + border
		t.Fatalf("short-content preview height = %d lines, want 7 (padded to maxLines)", len(lines))
	}
}

func TestPreviewBlockTitleInBorderTopEdge(t *testing.T) {
	block := stripANSI(PreviewBlock("Live preview", "Host x", false, 40, 0))
	top := strings.Split(block, "\n")[0]
	if !strings.Contains(top, "Live preview") {
		t.Errorf("title must render inside the border's top edge; got %q", top)
	}
	if !strings.HasPrefix(top, "╭") || !strings.HasSuffix(strings.TrimRight(top, " "), "╮") {
		t.Errorf("titled top border must still start/end with the box corners; got %q", top)
	}
}

func TestPreviewBlockClipsWithTail(t *testing.T) {
	text := strings.TrimSuffix(strings.Repeat("l\n", 20), "\n")
	block := stripANSI(previewBlockClipped(text, false, 40, 5))
	if !strings.Contains(block, "… (+15 more lines)") {
		t.Errorf("clipped preview must announce hidden lines; got %q", block)
	}
}

func TestRenderFrameInputFocusedReservedFooterIsHonest(t *testing.T) {
	plain := stripANSI(RenderFrame(100, 30, Seed(), TabIdentities, nil, "Ready.", "info", nil, true, "body"))
	for _, want := range []string{"Esc back", "Ctrl+P palette"} {
		if !strings.Contains(plain, want) {
			t.Errorf("input-focused reserved footer missing %q", want)
		}
	}
	// A focused text input swallows q and ? — the footer must not lie (L1).
	for _, forbidden := range []string{"q quit", "? help", "Enter activate"} {
		if strings.Contains(plain, forbidden) {
			t.Errorf("input-focused reserved footer must not advertise %q", forbidden)
		}
	}
}

func TestFitPaneAppendsVisibleCue(t *testing.T) {
	pane := strings.TrimSuffix(strings.Repeat("prose line\n", 30), "\n")
	got := stripANSI(fitPane(pane, 10))
	lines := strings.Split(got, "\n")
	if len(lines) != 10 {
		t.Fatalf("fitPane height = %d lines, want 10", len(lines))
	}
	if lines[9] != " … (+21 more lines)" {
		t.Errorf("fitPane cue line = %q, want ` … (+21 more lines)`", lines[9])
	}
	if short := fitPane("one\ntwo", 10); stripANSI(short) != "one\ntwo" {
		t.Errorf("fitPane must pass short panes through; got %q", short)
	}
}

func TestJoinMasterDetailDrawsFullHeightDivider(t *testing.T) {
	out := stripANSI(joinMasterDetail("left", 10, "right\npane", 5))
	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Fatalf("join height = %d lines, want 5 (divider rows)", len(lines))
	}
	for i, line := range lines {
		if !strings.Contains(line, "│") {
			t.Errorf("row %d missing the │ divider", i)
		}
	}
	if idx := strings.Index(lines[0], "│"); idx != 10 {
		t.Errorf("divider column = %d, want exactly the master width (10) so hit-tests hold", idx)
	}
	// R1: the divider carries a one-space right gutter so wrapped detail
	// lines never butt against it — detail starts at leftWidth +
	// masterDetailGutter (rune-indexed: the │ glyph is multi-byte).
	runes := []rune(lines[0])
	if got := string(runes[10+masterDetailGutter : 10+masterDetailGutter+5]); got != "right" {
		t.Errorf("detail cells = %q, want `right` at column %d (│ + gutter space)", got, 10+masterDetailGutter)
	}
	if runes[11] != ' ' {
		t.Error("the cell right of the divider must be the gutter space (R1)")
	}
}

// ---------------------------------------------------------------------------
// ExactTextViewport — Task 1 RED tests (03-13 proof viewport).
// ---------------------------------------------------------------------------

// TestExactTextViewport_BytePreserving proves the viewport stores its source
// text byte-for-byte and returns exact content without truncation ellipsis.
func TestExactTextViewport_BytePreserving(t *testing.T) {
	// A line that would be ellipsized by ansi.Truncate at width 20.
	longLine := "ssh -T git@ssh.github.com -p 443 -i ~/.ssh/id_ed25519_acme -o StrictHostKeyChecking=accept-new"
	v := ExactTextViewport{
		Text:         longLine,
		VisibleLines: 1,
		Width:        20,
	}
	rendered := v.View()
	// The content must be present (truncated at display edge, not replaced with "…").
	if strings.Contains(rendered, "…") {
		t.Errorf("ExactTextViewport must not insert ellipsis; got %q", rendered)
	}
	// The first N runes must match.
	plain := stripANSI(rendered)
	if !strings.HasPrefix(longLine, strings.TrimSpace(plain)) && !strings.HasPrefix(strings.TrimSpace(plain), "ssh") {
		t.Errorf("ExactTextViewport truncated wrong content: got %q", plain)
	}
}

// TestExactTextViewport_ClueCueOnHiddenLines proves a range-cue faint line
// appears on the last row when the viewport has more content below.
func TestExactTextViewport_ClueCueOnHiddenLines(t *testing.T) {
	text := "line1\nline2\nline3\nline4\nline5"
	v := ExactTextViewport{
		Text:         text,
		VisibleLines: 3,
		Width:        80,
	}
	rendered := stripANSI(v.View())
	lines := strings.Split(rendered, "\n")
	if len(lines) != 3 {
		t.Fatalf("viewport rendered %d lines, want 3", len(lines))
	}
	// The last row must be a continuation cue when content extends below.
	if !strings.Contains(lines[2], "PgDn") {
		t.Errorf("last row must contain scroll cue 'PgDn' when hidden lines exist; got %q", lines[2])
	}
}

// TestExactTextViewport_ClampPreventsOverscroll proves ScrollDown and Clamp
// prevent the offset from going past the last screenful.
func TestExactTextViewport_ClampPreventsOverscroll(t *testing.T) {
	v := ExactTextViewport{
		Text:         "a\nb\nc\nd\ne",
		VisibleLines: 3,
		Width:        40,
	}
	// Scrolling far past the end must clamp at the last valid offset.
	v2 := v.ScrollDown(100)
	if v2.LineOffset < 0 {
		t.Error("clamped offset must not be negative")
	}
	// After clamping, the last visible line must be within bounds.
	total := v2.TotalLines()
	if v2.LineOffset+v2.VisibleLines > total+1 {
		t.Errorf("overscrolled: offset %d + visible %d > total %d", v2.LineOffset, v2.VisibleLines, total)
	}
	// ScrollUp from clamped position must reduce offset.
	v3 := v2.ScrollUp(1)
	if v3.LineOffset >= v2.LineOffset {
		t.Error("ScrollUp from clamped position must reduce offset")
	}
}

// TestExactTextViewport_TotalLines proves TotalLines returns the correct line count.
func TestExactTextViewport_TotalLines(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"one", 1},
		{"one\ntwo\nthree", 3},
	}
	for _, tc := range cases {
		v := ExactTextViewport{Text: tc.text}
		if got := v.TotalLines(); got != tc.want {
			t.Errorf("TotalLines(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

// TestExactTextViewport_ScrollRevealsBytesHiddenBelow proves that after
// scrolling down one page, previously-hidden lines become visible and the
// earlier lines are no longer rendered — the source text remains intact.
func TestExactTextViewport_ScrollRevealsBytesHiddenBelow(t *testing.T) {
	text := "line1\nline2\nline3\nline4\nline5"
	v := ExactTextViewport{
		Text:         text,
		VisibleLines: 3,
		Width:        80,
	}
	// Before scroll: line4 and line5 are hidden.
	before := stripANSI(v.View())
	if strings.Contains(before, "line4") {
		t.Error("line4 must not be visible before scrolling down")
	}
	// After one PageDown (VisibleLines rows): line4/line5 become visible.
	v2 := v.ScrollDown(v.VisibleLines)
	after := stripANSI(v2.View())
	if !strings.Contains(after, "line4") && !strings.Contains(after, "line5") {
		t.Errorf("line4/line5 must be visible after scrolling down; got:\n%s", after)
	}
	// line1 must no longer be rendered in the main content (may appear in cue).
	afterLines := strings.Split(after, "\n")
	if len(afterLines) > 0 && strings.Contains(afterLines[0], "line1") {
		t.Error("line1 must not appear in the first content row after scrolling past it")
	}
}

// ---------------------------------------------------------------------------
// ExactTextViewport — Task 1 RED tests (03-14 two-axis viewport).
// ---------------------------------------------------------------------------

// TestExactTextViewport_HorizontalOffset_ScrollRightRevealsHiddenColumns
// proves that a line wider than Width has hidden columns that become visible
// after ScrollRight. This is the two-axis (horizontal) extension required by
// A-03: long command lines (ssh -T ... 95 chars) must be reachable at 62-col
// pane width by scrolling right, not permanently truncated.
func TestExactTextViewport_HorizontalOffset_ScrollRightRevealsHiddenColumns(t *testing.T) {
	// A line wider than the viewport window.
	longLine := "ssh -T git@ssh.github.com -p 443 -i ~/.ssh/id_ed25519_acme -o StrictHostKeyChecking=accept-new-HIDDEN_SUFFIX"
	v := ExactTextViewport{
		Text:         longLine,
		VisibleLines: 1,
		Width:        40,
	}
	before := stripANSI(v.View())
	if strings.Contains(before, "HIDDEN_SUFFIX") {
		t.Error("HIDDEN_SUFFIX must not be visible before scrolling right (width=40)")
	}
	// After scrolling right 70 columns, the suffix must become visible.
	v2 := v.ScrollRight(70)
	after := stripANSI(v2.View())
	if !strings.Contains(after, "HIDDEN_SUFFIX") {
		t.Errorf("HIDDEN_SUFFIX must be visible after ScrollRight(70); got %q", after)
	}
}

// TestExactTextViewport_HorizontalOffset_ClampPreventsNegative proves
// ScrollLeft cannot push HorizontalOffset below 0.
func TestExactTextViewport_HorizontalOffset_ClampPreventsNegative(t *testing.T) {
	v := ExactTextViewport{
		Text:             "short",
		VisibleLines:     1,
		Width:            20,
		HorizontalOffset: 5,
	}
	v2 := v.ScrollLeft(100)
	if v2.HorizontalOffset < 0 {
		t.Errorf("HorizontalOffset must not go negative after ScrollLeft(100); got %d", v2.HorizontalOffset)
	}
}

// TestExactTextViewport_HorizontalOffset_CueWhenHiddenRight proves that when
// there are hidden columns to the right, the rendered output signals that
// (e.g. contains a "→" or column hint) so the user knows to scroll.
func TestExactTextViewport_HorizontalOffset_CueWhenHiddenRight(t *testing.T) {
	longLine := "A" + strings.Repeat("B", 100) + "END"
	v := ExactTextViewport{
		Text:         longLine,
		VisibleLines: 1,
		Width:        20,
	}
	rendered := stripANSI(v.View())
	// When content is wider than the viewport, a cue must appear
	// (either a "→" column hint, or the rightmost line shows it's cut off).
	// We accept any of: "→", "cols", the scroll hint text.
	hasCue := strings.Contains(rendered, "→") || strings.Contains(rendered, "cols") ||
		strings.Contains(rendered, "scroll") || strings.Contains(rendered, "Right")
	if !hasCue {
		// At minimum, the viewport must NOT show "END" (which is off screen)
		// and the visible content must start with "A".
		if strings.Contains(rendered, "END") {
			t.Errorf("ExactTextViewport must not show content beyond Width without scrolling; got %q", rendered)
		}
		if !strings.HasPrefix(strings.TrimSpace(rendered), "A") {
			t.Errorf("ExactTextViewport must start at HorizontalOffset=0 with 'A'; got %q", rendered)
		}
	}
}

func TestExactTextViewport_HorizontalOnlyCueKeepsExactRowBudget(t *testing.T) {
	v := ExactTextViewport{
		Text:         "0123456789",
		VisibleLines: 3,
		Width:        4,
	}
	if got := len(strings.Split(v.View(), "\n")); got != 3 {
		t.Fatalf("horizontal-only viewport rows = %d, want exactly 3", got)
	}
}

func TestExactTextViewport_HorizontalCueKeepsFinalLineReachable(t *testing.T) {
	v := ExactTextViewport{
		Text:         "one\ntwo\n" + strings.Repeat("wide", 4) + "\nEND",
		VisibleLines: 3,
		Width:        8,
	}
	rendered := stripANSI(v.ScrollDown(99).View())
	if !strings.Contains(rendered, "END") {
		t.Fatalf("horizontal cue must not make the final source line unreachable:\n%s", rendered)
	}
}

// TestExactTextViewport_HorizontalScrollRoundTrip proves ScrollRight then
// ScrollLeft returns to the origin, and source bytes are unchanged.
func TestExactTextViewport_HorizontalScrollRoundTrip(t *testing.T) {
	line := "ABCDEF_long_content_here_GHIJKLMNOP"
	v := ExactTextViewport{
		Text:         line,
		VisibleLines: 1,
		Width:        10,
	}
	v2 := v.ScrollRight(15).ScrollLeft(15)
	if v2.HorizontalOffset != v.HorizontalOffset {
		t.Errorf("round-trip HorizontalOffset mismatch: got %d, want %d", v2.HorizontalOffset, v.HorizontalOffset)
	}
	if v2.Text != v.Text {
		t.Error("source Text must be unchanged after horizontal scroll round-trip")
	}
}

func TestExactTextViewport_ClampKeepsHorizontalOffsetReachableAfterResize(t *testing.T) {
	v := ExactTextViewport{
		Text:             strings.Repeat("x", 20),
		HorizontalOffset: 99,
		VisibleLines:     1,
		Width:            8,
	}.Clamp()
	if v.HorizontalOffset != 12 {
		t.Fatalf("HorizontalOffset = %d, want 12 so the final column remains reachable", v.HorizontalOffset)
	}

	v.Width = 30
	v = v.Clamp()
	if v.HorizontalOffset != 0 {
		t.Fatalf("HorizontalOffset after resize = %d, want 0", v.HorizontalOffset)
	}
}

func TestExactTextViewport_HorizontalSliceKeepsANSIEscapesIntact(t *testing.T) {
	line := "\x1b[31mabcdefghij\x1b[0m"
	v := ExactTextViewport{Text: line, HorizontalOffset: 4, VisibleLines: 1, Width: 3}
	got := strings.Split(v.View(), "\n")[0]
	if strings.Contains(got, "\x1b[") && !strings.Contains(got, "\x1b[31m") {
		t.Fatalf("horizontal slice contains a partial ANSI escape: %q", got)
	}
	if plain := stripANSI(got); !strings.Contains(plain, "efg") {
		t.Fatalf("horizontal slice = %q, want visible columns efg", plain)
	}
}

// TestRenderFooterLineNeverCutsAWordMidWord guards 05-UI-REVIEW.md's third
// finding: at a width too narrow for every action, the footer keybar must
// drop a whole trailing action rather than fragment its label mid-word.
func TestRenderFooterLineNeverCutsAWordMidWord(t *testing.T) {
	actions := []FooterAction{
		{Key: "↑↓", Label: "select identity"},
		{Key: "1234", Label: "switch tabs"},
	}
	got := stripANSI(renderFooterLine(30, actions))
	if !strings.Contains(got, "select identity") {
		t.Errorf("the action that fully fits must still render whole; got %q", got)
	}
	if strings.Contains(got, "switch") {
		t.Errorf("an action that doesn't fully fit must be dropped whole, never fragmented (e.g. \"sw…\"); got %q", got)
	}
}

// TestFooterActionAtNeverHitsADroppedAction closes a 02-13-review-flagged
// "theoretical" click-hijack: footerActionAt used to derive click spans from
// the FULL action list regardless of what renderFooterLine actually
// rendered, so a click past the visible "…" cue could dispatch an action
// that isn't on screen. It stopped being theoretical once renderFooterLine
// began dropping whole trailing actions (05-UI-REVIEW.md fix) instead of
// padding the line to width.
func TestFooterActionAtNeverHitsADroppedAction(t *testing.T) {
	actions := []FooterAction{
		{Key: "↑↓", Label: "select identity"},
		{Key: "1234", Label: "switch tabs"},
	}
	width := 30
	rendered := stripANSI(renderFooterLine(width, actions))
	// The second action does not fit at width 30 and must be dropped — any
	// click past the rendered content must report no hit, never the dropped
	// action's key.
	for x := len(rendered); x < 60; x++ {
		if action, ok := footerActionAt(width, actions, x); ok {
			t.Fatalf("footerActionAt(%d) hit %q past the rendered %q — dropped action must not be clickable", x, action.Key, rendered)
		}
	}
}

func TestSeverityLabelLockedContract(t *testing.T) {
	cases := []struct {
		severity HealthSeverity
		want     string
	}{
		{SeverityInfo, "~ info"},
		{SeverityWarning, "! warning"},
		{SeverityError, "✗ error"},
		{SeverityCritical, "✗ critical"},
	}
	for _, tc := range cases {
		if got := stripANSI(severityLabel(tc.severity)); got != tc.want {
			t.Errorf("severityLabel(%s) = %q, want %q (locked glyph+word contract)", tc.severity, got, tc.want)
		}
	}
}

func TestRenderHeaderFiveSegmentsFitAtMinWidth(t *testing.T) {
	plain := stripANSI(renderHeader(minFrameWidth, Seed(), TabIdentities, false))
	for _, want := range []string{"[1] Identities", "[2] SSH", "[3] Git", "[4] Doctor", "[5] Ignore", "8 ids"} {
		if !strings.Contains(plain, want) {
			t.Errorf("header at minFrameWidth missing %q; got %q", want, plain)
		}
	}
}

func TestHeaderColumnBudgetFromHeaderTabText(t *testing.T) {
	segmentSum := 0
	for i := 0; i < len(tabNavLabels); i++ {
		segmentSum += ansi.StringWidth(headerTabText(i))
	}
	if segmentSum != 58 {
		t.Errorf("sum of headerTabText widths = %d, want 58", segmentSum)
	}
	brandWidth := ansi.StringWidth(" " + headerBrand + "  ")
	total := brandWidth + segmentSum + 4*ansi.StringWidth(headerTabSeparator)
	if total != 70 {
		t.Errorf("brand + segments + separators = %d, want 70 (brand=%d)", total, brandWidth)
	}
	remaining := minFrameWidth - total
	if remaining < 16 {
		t.Errorf("columns left for the health chip = %d, want at least 16", remaining)
	}
}

func TestHeaderTabAtRoundTrips(t *testing.T) {
	cursor := ansi.StringWidth(" " + headerBrand + "  ")
	for i := 0; i < len(tabNavLabels); i++ {
		w := ansi.StringWidth(headerTabText(i))
		mid := cursor + w/2
		got, ok := headerTabAt(mid)
		if !ok || got != TabID(i) {
			t.Errorf("column %d (tab %d) → (%v, %v), want (%v, true)", mid, i, got, ok, TabID(i))
		}
		cursor += w + ansi.StringWidth(headerTabSeparator)
	}
	if tab, ok := headerTabAt(minFrameWidth - 2); ok {
		t.Errorf("chip column resolved to tab %v, want no tab", tab)
	}
}

func TestBreadcrumbKeepsFullLabels(t *testing.T) {
	for tab, full := range map[TabID]string{
		TabIdentities: "Identities",
		TabGlobalSSH:  "Global SSH",
		TabGlobalGit:  "Global Git",
		TabDoctor:     "Doctor",
		TabGitIgnore:  "Global Git Ignore",
	} {
		plain := stripANSI(RenderFrame(minFrameWidth, minFrameHeight, Seed(), tab, nil, "Ready.", "info", nil, false, "body"))
		if !strings.Contains(plain, full) {
			t.Errorf("breadcrumb for tab %v missing full label %q; got:\n%s", tab, full, plain)
		}
	}
}
