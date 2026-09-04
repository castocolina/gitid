//go:build screenshot

package screenshot

import (
	"os"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/tuikit"
)

// TestExtractGSSApplyHeadingAbsorbsWrappedContinuationRow is the WR-12
// regression: extractGSSApplyHeading's doc comment promises it "absorbs an
// immediately-following wrapped continuation row if the resolved target
// wraps past the ceremony's width" — before the fix, the wrapped
// continuation line was never appended to out; the function always returned
// after the FIRST matching line (or, when the very next row was the
// "Touches" row, broke out and returned that first line too, with no
// continuation content either way). A resolved target long enough to wrap
// (the common case for a sandbox HOME like
// /tmp/h4042748673/.ssh/config.d/gitid.config) silently dropped its tail
// from the T-06-CEREMONYTARGET comparison.
func TestExtractGSSApplyHeadingAbsorbsWrappedContinuationRow(t *testing.T) {
	lines := []string{
		"some unrelated preceding line",
		"Write Host * managed block to /tmp/h4042748673/.ssh/config.d/",
		"gitid.config",
		"Touches: personal, work",
		"some unrelated trailing line",
	}
	got := extractGSSApplyHeading(lines)
	if !strings.Contains(got, "Write Host * managed block to") {
		t.Fatalf("extracted heading missing the anchor line: %q", got)
	}
	if !strings.Contains(got, "gitid.config") {
		t.Errorf("extracted heading dropped the wrapped continuation row; WR-12 regressed: %q", got)
	}
	if strings.Contains(got, "Touches") {
		t.Errorf("extracted heading leaked the Touches row, which is not part of the heading: %q", got)
	}
	if strings.Contains(got, "unrelated") {
		t.Errorf("extracted heading leaked an unrelated line: %q", got)
	}
}

// TestExtractGIGNBodyDoesNotFalsePositiveOnOtherScreens is the 09.2-REVIEW.md
// WR-02 regression: gignCrumbIndex previously matched the bare string
// "Global Git Ignore" ANYWHERE in the frame via strings.Contains, not just
// on the breadcrumb row. That string also appears in the Ctrl+P palette row
// (rendered on every tab) and in internal/doctor/checks/baseline.go's
// SuggestedFix prose (rendered by the Health and Identities panes) — so a
// non-gign frame containing either could false-positive into a non-empty
// gign-body region and break BuildRegionDiffs. The fix anchors on the exact
// breadcrumb ROW (index 1) instead of a frame-wide substring search.
func TestExtractGIGNBodyDoesNotFalsePositiveOnOtherScreens(t *testing.T) {
	t.Run("palette row mentioning the screen name", func(t *testing.T) {
		lines := []string{
			" gitid   [1] Identities · [2] SSH · [3] Git · [4] Doctor · [5] Ignore ",
			" Identities",
			" Ctrl+P palette: 1 Identities · 2 Global SSH · 3 Global Git · 4 Health · 5 Fixer · 6 Global Git Ignore",
			" some identity list content",
		}
		if got := ExtractRegion(strings.Join(lines, "\n"), RegionGIGNBody); got != "" {
			t.Errorf("gign-body false-positived on a palette row mentioning the screen name: %q", got)
		}
	})

	t.Run("Health SuggestedFix prose mentioning the screen name", func(t *testing.T) {
		lines := []string{
			" gitid   [1] Identities · [2] SSH · [3] Git · [4] Doctor · [5] Ignore ",
			" Health",
			" Health",
			" ~ Suggested fix: use the Global Git Ignore screen to seed the curated defaults",
		}
		if got := ExtractRegion(strings.Join(lines, "\n"), RegionGIGNBody); got != "" {
			t.Errorf("gign-body false-positived on a Health SuggestedFix line mentioning the screen name: %q", got)
		}
	})

	t.Run("genuine gign frame still extracts", func(t *testing.T) {
		lines := []string{
			" gitid   [1] Identities · [2] SSH · [3] Git · [4] Doctor · [5] Ignore ",
			" Global Git Ignore",
			" Global Git Ignore",
			" ~/.gitignore_global",
			" ✓ Wired — core.excludesfile points at this file; Git reads it.",
		}
		got := ExtractRegion(strings.Join(lines, "\n"), RegionGIGNBody)
		if got == "" {
			t.Fatal("gign-body must extract from a genuine Global Git Ignore frame")
		}
		if !strings.Contains(got, "Wired") {
			t.Errorf("extracted region missing expected content: %q", got)
		}
	})
}

// TestExtractGSSApplyHeadingSingleLineNoWrap proves the non-wrapped case
// (heading immediately followed by "Touches") still returns just the one
// heading line — the fix must not over-absorb when there is no wrap.
func TestExtractGSSApplyHeadingSingleLineNoWrap(t *testing.T) {
	lines := []string{
		"Write Host * managed block to ~/.ssh/config",
		"Touches: personal",
	}
	got := extractGSSApplyHeading(lines)
	want := "Write Host * managed block to ~/.ssh/config"
	if got != want {
		t.Errorf("extractGSSApplyHeading() = %q, want %q", got, want)
	}
}

// TestExtractUploadSectionExcludesTheUnauthCheckboxLabel is the 09-REVIEW.md
// WR-12 regression: extractUploadSection's own doc comment says it
// deliberately excludes "Register with " (the D-01 checkbox's OWN label,
// rendered on every gated step-0 screen) because anchoring on it would
// collide with every pre-existing step-0 spec that has no disposition for
// it — but the marker list still included "not logged in to", which is a
// substring of that SAME label (UploadCheckboxLabelUnauthFmt = "Register
// with %s automatically — not logged in to %s; ..."). Any step-0 screen in
// the unauth state anchored RegionUploadSection on the ordinary form body,
// exactly the collision the comment claims to avoid.
func TestExtractUploadSectionExcludesTheUnauthCheckboxLabel(t *testing.T) {
	lines := []string{
		"some earlier form line",
		`☐ Register with GitHub automatically — not logged in to github.com; run "gh auth login" first, or check anyway`,
		"a later, ordinary form line",
	}
	got := extractUploadSection(lines)
	if got != "" {
		t.Errorf("extractUploadSection matched the D-01 unauth checkbox's OWN label; got:\n%q", got)
	}
}

// TestExtractUploadSectionStillMatchesRealUploadBeatContent is the positive
// control for the WR-12 fix: dropping "not logged in to" must not disable
// the region entirely — a real announce/result screen (the beat that
// actually ran) must still be found via its OTHER markers.
func TestExtractUploadSectionStillMatchesRealUploadBeatContent(t *testing.T) {
	lines := []string{
		"some earlier form line",
		"Running:",
		"gh ssh-key add /path/key.pub --title gitid: acme @ mbp --type authentication",
		"✓ Authentication key registered",
	}
	got := extractUploadSection(lines)
	if got == "" {
		t.Fatal("extractUploadSection found nothing for real upload-beat content")
	}
	if !strings.Contains(got, "gh ssh-key add") {
		t.Errorf("got=%q, want it to include the announced command", got)
	}
	if !strings.Contains(got, "Authentication key registered") {
		t.Errorf("got=%q, want it to include the result row", got)
	}
}

// TestExtractSubTabStripDerivesSiblingLabelsFromFrozenConstants is the
// WR-11 regression: extractSubTabStrip used to gate on the LITERAL strings
// "All directives", "Storage & preview", and "Set keys" — restating text
// both TUI files (globalssh.go, globalgit.go) are required to derive from
// tuikit's own frozen design.go constants ("ONE source for the label text,
// never restated"). If a frozen label is ever reworded, a hardcoded literal
// here silently stops matching (extractSubTabStrip fails OPEN, returning
// "") rather than tracking the rename — mirrors the established
// tuikit.LastHeaderNavLabel precedent (extractHeader, headerNavAnchor)
// already used elsewhere in this same file.
//
// Asserted at the SOURCE level (a purely behavioral test cannot distinguish
// "derives from the constant" from "a restated literal that currently
// happens to equal it").
func TestExtractSubTabStripDerivesSiblingLabelsFromFrozenConstants(t *testing.T) {
	src, err := os.ReadFile("createflow_regions.go")
	if err != nil {
		t.Fatalf("reading createflow_regions.go: %v", err)
	}
	body := string(src)
	for _, want := range []string{
		"tuikit.PropsSSHSubTabLabel",
		"tuikit.PropsSSHStorageSubTabLabel",
		"tuikit.PropsGitSubTabLabel",
		// WR-06 (09.5-REVIEW.md round 3): the WR-11 fix's own MANDATORY
		// guard — the first check extractSubTabStrip runs before any of the
		// three sibling labels — was left restating "Options" as a literal,
		// so renaming the Options sub-tab would silently make
		// extractSubTabStrip return "" for every frame, exactly the failure
		// mode WR-11 was raised to prevent, on the one label the fix skipped.
		"tuikit.PropsOptionsSubTabLabel",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("createflow_regions.go must reference %s, not a restated literal (WR-11/WR-06)", want)
		}
	}
	// The literals still appear legitimately in doc comments elsewhere in
	// this file — only the ACTUAL comparison expression extractSubTabStrip
	// used to gate on is forbidden, not every prose mention of the words.
	for _, forbidden := range []string{
		`strings.Contains(plain, "All directives")`,
		`strings.Contains(plain, "Storage & preview")`,
		`strings.Contains(plain, "Set keys")`,
		`strings.Contains(plain, "Options")`,
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("createflow_regions.go must not restate %s — derive it from the frozen tuikit constant instead (WR-11/WR-06)", forbidden)
		}
	}
}

// TestExtractSubTabStripMatchesTheProductionRenderedLabels is
// TestExtractSubTabStripDerivesSiblingLabelsFromFrozenConstants's
// behavioral sibling: a frame built from tuikit's OWN frozen constants
// (not a literal restated in the test either) must still be found.
func TestExtractSubTabStripMatchesTheProductionRenderedLabels(t *testing.T) {
	for _, label := range []string{tuikit.PropsSSHSubTabLabel, tuikit.PropsSSHStorageSubTabLabel, tuikit.PropsGitSubTabLabel} {
		lines := []string{
			"╭╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╮",
			"┊ " + tuikit.PropsOptionsSubTabLabel + " │ " + label + " ┊",
			"╰╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╯",
		}
		got := extractSubTabStrip(lines)
		if got == "" {
			t.Errorf("extractSubTabStrip returned empty for a strip carrying the production label %q", label)
		}
	}
}

// TestExtractHeaderKeepsLastNavSegmentOutOfStatusRegion is the regression
// for WR-06: the header/header-status split used to anchor mid-list (a
// literal "Doctor "/"Fixer " label), leaving the LAST nav tab ("[5] Ignore")
// inside RegionHeaderStatus — the region every registry allowlists as a
// fixture-vs-live divergence. A regression in that last tab's label,
// spacing, or styling could never fail the byte-compared gate. The anchor
// must now be derived from the actual last label (tuikit.LastHeaderNavLabel)
// so every nav segment, including the last one, stays inside the
// byte-compared RegionHeader.
func TestExtractHeaderKeepsLastNavSegmentOutOfStatusRegion(t *testing.T) {
	lines := []string{
		" gitid   [1] Identities · [2] SSH · [3] Git · [4] Doctor · [5] Ignore               3 ids · ✓ ok",
	}
	header := extractHeader(lines)
	if !strings.Contains(header, "[5] Ignore") {
		t.Errorf("extractHeader must include the last nav segment [5] Ignore, got %q", header)
	}
	status := extractHeaderStatus(lines)
	if strings.Contains(status, "[5] Ignore") {
		t.Errorf("extractHeaderStatus must NOT include [5] Ignore — it belongs to the byte-compared header, not the allowlisted status region; got %q", status)
	}
	if !strings.Contains(status, "3 ids") {
		t.Errorf("extractHeaderStatus must still capture the status summary; got %q", status)
	}
}
