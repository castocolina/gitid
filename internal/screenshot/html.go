//go:build screenshot

package screenshot

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// ChromiumRevision is the pinned Chromium snapshot revision used for every
// screenshot-html capture, on every OS. It is go-rod v0.116.2's own
// launcher.RevisionDefault; pinning it explicitly here (rather than trusting
// go-rod's "whatever RevisionDefault happens to be today") is what makes
// HTML captures reproducible across machines and CI runs -- see the
// provenance note in .planning/design/_spike/GOLDENS.md (T-01-SC2).
const ChromiumRevision = launcher.RevisionDefault

// HTMLOptions configures a single CaptureHTML render.
type HTMLOptions struct {
	// FixturePath is the local HTML file to capture, e.g.
	// .planning/design/_spike/fixture.html. Required. Loaded as a file://
	// URL -- no remote/untrusted content is ever fetched by this package
	// (threat model: "headless Chromium <- local fixture HTML").
	FixturePath string

	// OutDir is the directory the PNG is written under. Created if missing.
	OutDir string
	// Name is the output file's base name (without extension).
	Name string

	// ViewportWidth/ViewportHeight/DeviceScaleFactor are the fixed emulated
	// viewport (e.g. 1280x800, scale 1) so the same page always renders at
	// the same pixel dimensions regardless of the host display.
	ViewportWidth, ViewportHeight int
	DeviceScaleFactor             float64

	// ColorScheme pins prefers-color-scheme ("light" or "dark") so the
	// fixture never picks up the host OS's live color-scheme setting.
	// Defaults to "light" when empty.
	ColorScheme string

	// Timeout bounds the whole launch+navigate+wait+screenshot sequence
	// (browser tooling must always be time-bounded). Required (> 0).
	Timeout time.Duration

	// CacheDir is the fixed root directory the pinned Chromium revision is
	// downloaded into / read from. Defaults to launcher.DefaultBrowserDir
	// ($HOME/.cache/rod/browser, or %APPDATA%\rod\browser on Windows) when
	// empty.
	CacheDir string
	// Revision is the pinned Chromium snapshot revision. Defaults to
	// ChromiumRevision when zero.
	Revision int
	// AllowDownload controls offline/failure behavior: when false and the
	// pinned revision is not already cached at CacheDir, CaptureHTML fails
	// fast with an actionable error naming the revision + cache path,
	// rather than silently downloading or falling back to a different
	// browser (T-01-SC2). Set true for normal (online) capture runs.
	AllowDownload bool

	// URLFragment, when non-empty, is appended to the navigated file:// URL
	// after FixturePath is resolved and validated to exist (e.g.
	// "#/create/ssh" for a client-side HashRouter route). This lets ONE
	// built FixturePath — a single-page app's dist/index.html — serve as
	// the entry point for many distinct captured screens/routes without
	// CaptureHTML needing a literal per-route file on disk. The empty
	// string (the default) preserves the exact pre-existing single-fixture
	// behavior byte-for-byte. Added by Phase 2 (02-03) to extend, not
	// rebuild, this function — see 02-RESEARCH.md Pattern 1. Lets one
	// SPA build serve every captured route (e.g. re-capturing screens of
	// the interactive demo, or the future live TUI demo's HTML twin).
	URLFragment string

	// RequiredText, when non-empty, is checked against the rendered page's
	// <body> text AFTER navigation/load and BEFORE the screenshot is
	// captured or written to disk. If the text is absent, CaptureHTML fails
	// fast naming both RequiredText and the navigated URL — so a
	// wrong-route or blank capture is a hard error, never a silently wrong
	// image. Added by Phase 2 (02-03) alongside URLFragment: callers assert
	// a screen-identifying marker resolved correctly through the SAME
	// go-rod page CaptureHTML already owns, rather than a second capture
	// path (Pitfall 7: never re-instantiate a go-rod launcher elsewhere).
	RequiredText string

	// RequiredTexts, when non-empty, is checked the SAME way as
	// RequiredText (every entry must be present in the rendered <body> text
	// before any screenshot is captured/written), but requires ALL entries,
	// not just one. Added by the 02-review fix pass (review B1/T-02-FP):
	// RequiredText alone (a single breadcrumb-style marker) cannot catch a
	// "right route, wrong STATE" false positive — e.g. a route that renders
	// the correct breadcrumb but a stale/incorrect body underneath it. Pass
	// both a breadcrumb and a screen-specific signature here. When both
	// RequiredText and RequiredTexts are set, ALL of them (RequiredText
	// plus every RequiredTexts entry) must be present — additive, not a
	// replacement.
	RequiredTexts []string
}

// CaptureHTML renders a local fixture HTML page to a deterministic PNG via
// headless Chromium (go-rod), at a fixed viewport/scale/color-scheme, using
// a PINNED Chromium revision + a fixed cache path and a context timeout,
// then strips timestamp metadata and records a SHA-256 golden hash exactly
// like CaptureTUI (D-03/D-04).
func CaptureHTML(opts HTMLOptions) (Result, error) {
	if opts.FixturePath == "" {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: FixturePath is required")
	}
	if opts.OutDir == "" || opts.Name == "" {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: OutDir and Name are required")
	}
	if opts.Timeout <= 0 {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: Timeout must be > 0 (browser tooling must always be time-bounded)")
	}

	absFixture, err := filepath.Abs(opts.FixturePath)
	if err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: resolving fixture path %q: %w", opts.FixturePath, err)
	}
	if _, err := os.Stat(absFixture); err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: fixture not found at %q: %w", absFixture, err)
	}

	revision := opts.Revision
	if revision == 0 {
		revision = ChromiumRevision
	}
	cacheDir := opts.CacheDir
	if cacheDir == "" {
		cacheDir = launcher.DefaultBrowserDir
	}

	binPath, err := resolveBrowserBinary(cacheDir, revision, opts.AllowDownload)
	if err != nil {
		return Result{}, err
	}

	if err := os.MkdirAll(opts.OutDir, 0o750); err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: creating output dir %q: %w", opts.OutDir, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// allow-file-access-from-files: without it, Chromium refuses to fetch a
	// file:// page's OWN same-directory ES module imports (a <script
	// type="module"> served from file:// is treated as cross-origin from a
	// null origin and the import fetch is blocked), so a built SPA's JS
	// never executes and the page silently stays at its pre-render HTML
	// shell (an empty <div id="root">) -- CaptureHTML would otherwise
	// "succeed" and save a blank PNG with no error. A single trivial
	// fixture HTML page (Phase 1's own _spike/fixture.html, no <script
	// type="module">) never hit this, which is why Phase 1 didn't need the
	// flag; Phase 2 (02-03) needs it for its Vite-built ES module SPA.
	// Still ONLY ever navigates a local file:// URL (T-02-CAP; never
	// relaxes cross-origin behavior for a remote origin).
	l := launcher.New().Bin(binPath).Headless(true).Context(ctx).
		Set("allow-file-access-from-files")
	defer l.Cleanup()

	controlURL, err := l.Launch()
	if err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: launching headless Chromium (revision %d, bin %q): %w", revision, binPath, err)
	}

	browser := rod.New().ControlURL(controlURL).Context(ctx)
	if err := browser.Connect(); err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: connecting to headless Chromium: %w", err)
	}
	defer func() { _ = browser.Close() }()

	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: opening a page: %w", err)
	}
	page = page.Context(ctx)

	dsf := opts.DeviceScaleFactor
	if dsf == 0 {
		dsf = 1
	}
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             opts.ViewportWidth,
		Height:            opts.ViewportHeight,
		DeviceScaleFactor: dsf,
	}); err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: setting fixed viewport: %w", err)
	}

	colorScheme := opts.ColorScheme
	if colorScheme == "" {
		colorScheme = "light"
	}
	mediaReq := proto.EmulationSetEmulatedMedia{
		Features: []*proto.EmulationMediaFeature{{Name: "prefers-color-scheme", Value: colorScheme}},
	}
	if err := mediaReq.Call(page); err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: pinning prefers-color-scheme=%s: %w", colorScheme, err)
	}

	fileURL := "file://" + filepath.ToSlash(absFixture) + opts.URLFragment
	if err := page.Navigate(fileURL); err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: navigating to %s: %w", fileURL, err)
	}
	if err := page.WaitLoad(); err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: waiting for page load: %w", err)
	}

	required := opts.RequiredTexts
	if opts.RequiredText != "" {
		required = append([]string{opts.RequiredText}, required...)
	}
	if len(required) > 0 {
		// review B1: poll for ALL required text markers until present or
		// this capture's own Timeout expires, rather than checking the body
		// text exactly once immediately after WaitLoad. A React SPA can
		// commit its route body milliseconds AFTER the "load" event fires
		// (WaitLoad only proves the initial HTML shell + JS bundle loaded,
		// not that client-side routing/rendering has committed) -- a
		// single immediate check risks a flaky false-negative (or, if ever
		// relaxed to "pass on empty", a false positive) on a slower CI
		// runner. Deterministic: still bounded by ctx's Timeout, and the
		// poll interval is short enough not to add meaningful capture
		// latency on the common case where the text is already present.
		const pollInterval = 25 * time.Millisecond
		var bodyText string
		for {
			body, elErr := page.Element("body")
			if elErr != nil {
				return Result{}, fmt.Errorf("screenshot: CaptureHTML: locating <body> to verify required text %q at %s: %w", required, fileURL, elErr)
			}
			text, textErr := body.Text()
			if textErr != nil {
				return Result{}, fmt.Errorf("screenshot: CaptureHTML: reading rendered body text to verify required text %q at %s: %w", required, fileURL, textErr)
			}
			bodyText = text

			allPresent := true
			for _, want := range required {
				if !strings.Contains(bodyText, want) {
					allPresent = false
					break
				}
			}
			if allPresent {
				break
			}

			select {
			case <-ctx.Done():
				return Result{}, fmt.Errorf("screenshot: CaptureHTML: rendered page at %s does not contain all required text markers %q within the capture timeout -- never saving a wrong-route/wrong-state/blank capture (got body text: %.200q): %w", fileURL, required, bodyText, ctx.Err())
			case <-time.After(pollInterval):
			}
		}
	}

	shot, err := page.Screenshot(true, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: capturing screenshot: %w", err)
	}

	pngPath := filepath.Join(opts.OutDir, opts.Name+".png")
	if err := os.WriteFile(pngPath, shot, 0o600); err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureHTML: writing PNG to %q: %w", pngPath, err)
	}

	return finalizePNG(pngPath)
}

// resolveBrowserBinary locates the pinned Chromium revision inside cacheDir.
// If it is already valid there, its path is returned immediately -- no
// network access. Otherwise:
//   - allowDownload == false: fails fast with an actionable error naming the
//     revision and cache path (never silently falls back to a different
//     browser -- T-01-SC2).
//   - allowDownload == true: downloads the pinned revision into cacheDir
//     (what `make setup-env` does ahead of time, and what an interactive
//     capture run does on first use).
func resolveBrowserBinary(cacheDir string, revision int, allowDownload bool) (string, error) {
	b := &launcher.Browser{
		// Provisioning (the one-time download) is deliberately NOT bound to
		// the per-capture Timeout: it is a separate, potentially slow,
		// bootstrap concern (what `make setup-env` normally does ahead of
		// time), analogous to `go mod download`.
		Context:  context.Background(),
		RootDir:  cacheDir,
		Revision: revision,
		Hosts:    []launcher.Host{launcher.HostGoogle, launcher.HostNPM},
		Logger:   log.New(io.Discard, "", 0),
	}

	if err := b.Validate(); err == nil {
		return b.BinPath(), nil
	}

	if !allowDownload {
		return "", fmt.Errorf(
			"screenshot: pinned Chromium revision %d not found in cache %q and provisioning is disabled: "+
				"run `make setup-env` to provision it (never silently substitutes a different browser)",
			revision, cacheDir,
		)
	}

	if err := b.Download(); err != nil {
		return "", fmt.Errorf("screenshot: failed to provision pinned Chromium revision %d into cache %q: %w", revision, cacheDir, err)
	}
	return b.BinPath(), nil
}
