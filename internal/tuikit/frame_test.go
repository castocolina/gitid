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

	for _, want := range []string{"[1] Identities", "[2] Global SSH", "[3] Global Git", "[4] Doctor"} {
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
	if !strings.Contains(raw, "\x1b[1;97;44m [3] Global Git ") {
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
