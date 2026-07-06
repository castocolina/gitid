package dummytui

// theme_test.go pins the semantic style contract (02-STYLE-SPEC.md): the
// per-role SGR every Theme field must render, the theme-var PROMOTION
// (frame.go's old package-level vars must stay byte-identical to the new
// Theme roles), the DisabledNav-vs-full-brightness header-tab dimming while
// a pane captures keys (with the active tab keeping its ActiveNav accent
// background — checkpoint feedback U1), and the ActiveArea accent carried
// on the frame chrome while a pane captures keys.

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultThemeRolesRenderExpectedSGR(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Label bold", DefaultTheme.Label.Render("x"), "\x1b[1m"},
		{"Hint faint", DefaultTheme.Hint.Render("x"), "\x1b[2m"},
		{"Warning ANSI 3 (yellow)", DefaultTheme.Warning.Render("x"), "\x1b[33m"},
		{"Error ANSI 1 (red)", DefaultTheme.Error.Render("x"), "\x1b[31m"},
		{"Info ANSI 6 (cyan)", DefaultTheme.Info.Render("x"), "\x1b[36m"},
		{"Healthy ANSI 2 (green)", DefaultTheme.Healthy.Render("x"), "\x1b[32m"},
		{"DisabledNav faint", DefaultTheme.DisabledNav.Render("x"), "\x1b[2m"},
		{"Preview faint", DefaultTheme.Preview.Render("x"), "\x1b[2m"},
		{"FieldBlurred faint", DefaultTheme.FieldBlurred.Render("x"), "\x1b[2m"},
		{"ActiveArea accent (blue)", DefaultTheme.ActiveArea.Render("x"), "\x1b[34m"},
		// Checkpoint feedback U1: the active nav tab carries the accent as
		// a BACKGROUND (bold + bright-white ANSI 15 on ANSI-4 blue).
		{"ActiveNav accent background (bold, bright-white on blue)", DefaultTheme.ActiveNav.Render("x"), "\x1b[1;97;44m"},
	}
	for _, tc := range cases {
		if !strings.Contains(tc.got, tc.want) {
			t.Errorf("%s: rendered %q, want to contain %q", tc.name, tc.got, tc.want)
		}
	}
}

// TestThemeFieldFocusedIsColorOnlyNoBorder pins D1 (checkpoint-2 contract):
// FieldFocused is accent foreground + bold, NO border — the rounded-contour
// treatment 02-14 shipped is deleted (every field is one constant-height
// row; focus is signalled by color + the redundant `▸` marker, never a box).
func TestThemeFieldFocusedIsColorOnlyNoBorder(t *testing.T) {
	out := DefaultTheme.FieldFocused.Render("x")
	if !strings.Contains(out, "\x1b[1;34m") {
		t.Errorf("FieldFocused must carry the blue (ANSI 4) accent + bold; got %q", out)
	}
	if strings.Contains(out, "╭") || strings.Contains(out, "╯") {
		t.Errorf("FieldFocused must NEVER render a border (D1 kills the box); got %q", out)
	}
}

// TestThemeFieldFocusedHasNoRoundedBorderAnywhereInTheme is the acceptance
// grep, expressed as a Go test: theme.go must contain zero RoundedBorder
// references (the only prior user was FieldFocused).
func TestThemeFieldFocusedHasNoRoundedBorderAnywhereInTheme(t *testing.T) {
	src, err := os.ReadFile("theme.go")
	if err != nil {
		t.Fatalf("reading theme.go: %v", err)
	}
	if strings.Contains(string(src), "RoundedBorder") {
		t.Error("theme.go must not reference RoundedBorder — D1 deleted the focused-field box")
	}
}

func TestThemeAccentAndFieldBorderShareTheSameColor(t *testing.T) {
	if DefaultTheme.Accent != DefaultTheme.FieldBorder {
		t.Errorf("Accent (%v) and FieldBorder (%v) must be the SAME color — one accent, two role names (focused-field, active-area)", DefaultTheme.Accent, DefaultTheme.FieldBorder)
	}
}

// TestThemePromotionIsBehaviorPreserving proves the promotion of frame.go's
// package-level style vars to DefaultTheme roles changed nothing observable:
// every pre-existing copy-pinning test (frame/identities/doctor) stays green
// because the rendered output is byte-identical.
func TestThemePromotionIsBehaviorPreserving(t *testing.T) {
	cases := []struct {
		name string
		old  string
		new  string
	}{
		{"styleBold == Label", styleBold.Render("x"), DefaultTheme.Label.Render("x")},
		{"styleFaint == Hint", styleFaint.Render("x"), DefaultTheme.Hint.Render("x")},
		{"styleHealthy == Healthy", styleHealthy.Render("x"), DefaultTheme.Healthy.Render("x")},
		{"styleWarning == Warning", styleWarning.Render("x"), DefaultTheme.Warning.Render("x")},
		{"styleError == Error", styleError.Render("x"), DefaultTheme.Error.Render("x")},
		{"styleInfo == Info", styleInfo.Render("x"), DefaultTheme.Info.Render("x")},
	}
	for _, tc := range cases {
		if tc.old != tc.new {
			t.Errorf("%s: %q != %q — promotion must be behavior-preserving", tc.name, tc.old, tc.new)
		}
	}
}

// TestRenderHeaderActiveTabDimsToForegroundOnlyWhenCapturesKeys pins D4
// (checkpoint-2 contract): the ACTIVE tab renders the NEW ActiveNavDimmed
// role while a pane captures keys — bold + accent FOREGROUND, NO
// background fill — distinct from the full ActiveNav background treatment
// (superseding the prior "active tab keeps its background" behavior).
func TestRenderHeaderActiveTabDimsToForegroundOnlyWhenCapturesKeys(t *testing.T) {
	s := Seed()
	full := renderHeader(100, s, tabIdentities, false)
	dimmed := renderHeader(100, s, tabIdentities, true)
	if full == dimmed {
		t.Fatal("header rendering must differ between capturesKeys states")
	}
	if !strings.Contains(full, "\x1b[1;97;44m") {
		t.Error("the active tab with NO pane capturing keys must render ActiveNav (bold bright-white on blue background)")
	}
	if strings.Contains(dimmed, "\x1b[1;97;44m") {
		t.Error("the active tab must NOT keep the ActiveNav background while a pane captures keys — it renders ActiveNavDimmed instead (D4)")
	}
	if !strings.Contains(dimmed, DefaultTheme.ActiveNavDimmed.Render(headerTabText(int(tabIdentities)))) {
		t.Error("the active tab while capturesKeys must render through Theme.ActiveNavDimmed")
	}
	if !strings.Contains(dimmed, "\x1b[2m") {
		t.Error("inactive header tabs must render through DisabledNav (faint) while a pane captures keys")
	}
	// The tab separator (headerTabSeparator) is ALWAYS faint, in both
	// states — so assert on the COUNT of faint segments, not mere presence:
	// capturesKeys=true adds one faint-wrapped span per INACTIVE tab (3 of
	// the 4 tabs here) on top of the 3 always-faint separators.
	fullFaintCount := strings.Count(full, "\x1b[2m")
	dimmedFaintCount := strings.Count(dimmed, "\x1b[2m")
	if dimmedFaintCount <= fullFaintCount {
		t.Errorf("capturesKeys=true must add MORE faint (DisabledNav) spans than capturesKeys=false: dimmed=%d full=%d", dimmedFaintCount, fullFaintCount)
	}
}

// TestRenderFrameActiveAreaAccentWhileCapturesKeys asserts the SECOND part of
// round-3 defect D5: the active pane region must carry the ActiveArea accent
// itself (not only the DisabledNav dimming) while a pane captures keys — here
// via the breadcrumb divider line directly above the body (02-STYLE-SPEC.md
// §3 "ActiveArea mechanism").
func TestRenderFrameActiveAreaAccentWhileCapturesKeys(t *testing.T) {
	active := RenderFrame(100, 30, Seed(), tabIdentities, nil, "Ready.", "info", nil, true, "body")
	if !strings.Contains(active, "\x1b[34m") {
		t.Error("RenderFrame must carry the ActiveArea accent (blue, SGR 34) while a pane captures keys")
	}
	quiet := RenderFrame(100, 30, Seed(), tabIdentities, nil, "Ready.", "info", nil, false, "body")
	if strings.Contains(quiet, "\x1b[34m") {
		t.Error("RenderFrame must NOT carry the accent while no pane captures keys (detail mode)")
	}
}
