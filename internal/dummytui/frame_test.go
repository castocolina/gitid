package dummytui

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
	return RenderFrame(100, 30, Seed(), tabIdentities, crumbs, "Ready.", "info", actions, false, "body line")
}

func TestRenderFrameShowsNumberedTabsAndReservedFooter(t *testing.T) {
	plain := stripANSI(renderSeededFrame(nil, nil))

	for _, want := range []string{"1 Identities", "2 Global SSH", "3 Global Git", "4 Doctor"} {
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
	plainClean := stripANSI(RenderFrame(100, 30, clean, tabIdentities, nil, "Ready.", "info", nil, false, ""))
	if !strings.Contains(plainClean, "✓ ok") {
		t.Error("all-clean chip must show `✓ ok`")
	}
	if strings.Contains(plainClean, "! 0") || strings.Contains(plainClean, "✗ 0") {
		t.Error("all-clean chip must not show zero counts")
	}
}

func TestRenderFrameActiveTabReverseVideo(t *testing.T) {
	raw := RenderFrame(100, 30, Seed(), tabGlobalGit, nil, "Ready.", "info", nil, false, "")
	if !strings.Contains(raw, "\x1b[7m 3 Global Git ") {
		t.Error("active tab must render reverse-video (SGR 7 around the active label)")
	}
	if strings.Contains(raw, "\x1b[7m 1 Identities ") {
		t.Error("inactive tabs must not render reverse-video")
	}
}

func TestRenderFrameContextualActionsPrecedeReserved(t *testing.T) {
	plain := stripANSI(renderSeededFrame(nil, []FooterAction{{Key: "n", Label: "new"}, {Key: "d", Label: "delete"}}))
	if !strings.Contains(plain, "n new") || !strings.Contains(plain, "d delete") {
		t.Error("contextual footer actions missing")
	}
}

func TestRenderFrameGeometry(t *testing.T) {
	out := RenderFrame(100, 30, Seed(), tabIdentities, nil, "Ready.", "info", nil, false, strings.Repeat("line\n", 60))
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
	out := RenderFrame(80, 24, Seed(), tabIdentities, nil, "", "info", nil, false, "")
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
	plain := stripANSI(RenderFrame(100, 30, Seed(), tabIdentities, nil, "Ready.", "info", nil, true, "body"))
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
