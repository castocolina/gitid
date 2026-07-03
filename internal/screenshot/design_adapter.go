//go:build screenshot

package screenshot

// design_adapter.go is the thin, stable Phase-2-facing wrapper around
// Phase 1's actual capture functions (html.go's CaptureHTML, tui.go's
// CaptureTUI). design_capture_test.go calls ONLY CaptureHTMLScreen and
// CaptureTUIScreen below — never CaptureHTML/CaptureTUI directly, and never
// a second go-rod launcher or freeze exec-wrapper (Pitfall 7 in
// 02-RESEARCH.md: "never re-instantiate the go-rod launcher in Phase 2
// code — call Phase 1's function").
//
// PREFLIGHT (02-03-PLAN.md): this file consumes Phase 1 Plan 05's
// CaptureHTML/HTMLOptions and CaptureTUI/TUIOptions exported symbols. If
// those are ever renamed or removed, THIS FILE FAILS TO COMPILE — that is
// the intended fail-fast signal naming "Phase 1 Plan 05
// internal/screenshot must land first," rather than a silent duplicate
// capture implementation living here instead.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Capture parameters shared by every CaptureHTMLScreen/CaptureTUIScreen
// call (D-03/D-04 style: fixed across every mockup screen so captures stay
// comparable and deterministic). Viewport/color-scheme/timeout mirror
// Phase 1's own html_capture_test.go golden capture; the TUI theme mirrors
// tui_capture_test.go's golden capture (both already proven deterministic
// by Phase 1's recorded golden hashes).
const (
	mockupViewportWidth  = 1280
	mockupViewportHeight = 800
	mockupColorScheme    = "light"
	mockupCaptureTimeout = 60 * time.Second

	mockupTUITheme = "dracula"
)

// mockupFontFile is the vendored monospace TTF every TUI mockup capture
// pins via freeze's --font.file (Pitfall 6: system-font discovery is not
// CI-deterministic). Resolved relative to this package's source directory
// — go test's working directory is always the package's source directory,
// regardless of invocation cwd (same convention as tui_capture_test.go).
func mockupFontFile() string {
	return filepath.Join("..", "..", ".planning", "design", "fonts", "JetBrainsMono-Regular.ttf")
}

// CaptureHTMLScreen renders ONE mockup screen to a PNG through Phase 1's
// CaptureHTML.
//
// url MUST be a "file://" URL, optionally carrying a "#..." HashRouter
// fragment (e.g. "file:///abs/dist/index.html#/create/ssh") — the exact
// shape design_capture_test.go builds from the mockup's built
// dist/index.html plus a manifest entry's HTMLRoute. CaptureHTML's
// FixturePath field alone has no room for a URL fragment (it os.Stat's a
// literal file path), so this adapter splits the fragment off before
// calling CaptureHTML and passes it through HTMLOptions.URLFragment — a
// small, additive Phase 1 extension that preserves Phase 1's own
// single-fixture callers byte-for-byte (see html.go's URLFragment field
// doc and 02-03-SUMMARY.md's Deviations section for the rationale). The
// SAME go-rod navigate/screenshot path Phase 1 built is still the only one
// ever exercised — no second browser-driving implementation lives here.
//
// requiredTexts, when non-empty, is passed through as
// HTMLOptions.RequiredTexts so CaptureHTML asserts the rendered page's body
// contains EVERY one of them BEFORE ever writing a PNG (review HIGH-3b/d +
// B1, T-02-FP — never a blank, wrong-route, OR right-route/wrong-state
// capture silently passing). design_capture_test.go passes BOTH the
// "<surface>/<screen>" breadcrumb AND the manifest entry's own Signature —
// breadcrumb alone cannot catch a same-route-different-content false
// positive, the same gap e2e/dummy_nav_e2e_test.go's PTY walker already
// closed on its own side by checking breadcrumb+Signature together.
func CaptureHTMLScreen(url, outPath string, requiredTexts ...string) error {
	const filePrefix = "file://"
	if !strings.HasPrefix(url, filePrefix) {
		return fmt.Errorf("screenshot: CaptureHTMLScreen: url must start with %q (never a remote origin -- T-02-CAP), got %q", filePrefix, url)
	}
	rest := strings.TrimPrefix(url, filePrefix)
	fixturePath, fragment := rest, ""
	if idx := strings.IndexByte(rest, '#'); idx >= 0 {
		fixturePath, fragment = rest[:idx], rest[idx:]
	}

	outDir, name, err := splitCaptureOutPath(outPath)
	if err != nil {
		return fmt.Errorf("screenshot: CaptureHTMLScreen: %w", err)
	}

	if _, err := CaptureHTML(HTMLOptions{
		FixturePath:       fixturePath,
		URLFragment:       fragment,
		RequiredTexts:     requiredTexts,
		OutDir:            outDir,
		Name:              name,
		ViewportWidth:     mockupViewportWidth,
		ViewportHeight:    mockupViewportHeight,
		DeviceScaleFactor: 1,
		ColorScheme:       mockupColorScheme,
		Timeout:           mockupCaptureTimeout,
		AllowDownload:     true,
	}); err != nil {
		return fmt.Errorf("screenshot: CaptureHTMLScreen: %w", err)
	}
	return nil
}

// CaptureTUIScreen renders ONE mockup screen's dummytui.RenderScreen()
// golden to a PNG through Phase 1's CaptureTUI. view is the full-shell
// View() dump (the exact string design_capture_test.go got back from
// dummytui.RenderScreen(surface, screen), already asserted to contain the
// "<surface>/<screen>" breadcrumb before this is ever called).
func CaptureTUIScreen(view, outPath string) error {
	outDir, name, err := splitCaptureOutPath(outPath)
	if err != nil {
		return fmt.Errorf("screenshot: CaptureTUIScreen: %w", err)
	}

	if _, err := CaptureTUI(view, TUIOptions{
		FontFile: mockupFontFile(),
		Theme:    mockupTUITheme,
		OutDir:   outDir,
		Name:     name,
	}); err != nil {
		return fmt.Errorf("screenshot: CaptureTUIScreen: %w", err)
	}
	return nil
}

// splitCaptureOutPath splits a full "<dir>/<name>.png" output path into
// Phase 1's OutDir/Name pair (CaptureHTML/CaptureTUI append ".png"
// themselves).
func splitCaptureOutPath(outPath string) (outDir, name string, err error) {
	if outPath == "" {
		return "", "", fmt.Errorf("outPath is required")
	}
	ext := filepath.Ext(outPath)
	if ext != ".png" {
		return "", "", fmt.Errorf("outPath %q must end in .png", outPath)
	}
	base := strings.TrimSuffix(filepath.Base(outPath), ext)
	if base == "" {
		return "", "", fmt.Errorf("outPath %q has an empty file name", outPath)
	}
	return filepath.Dir(outPath), base, nil
}
