//go:build screenshot

package screenshot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

// Fixed capture parameters (D-03/D-04). These must never change without
// also re-recording the golden hash in .planning/design/_spike/GOLDENS.md.
const (
	htmlFixtureViewportWidth  = 1280
	htmlFixtureViewportHeight = 800
	htmlFixtureColorScheme    = "light"
	htmlCaptureTimeout        = 60 * time.Second

	// htmlGoldenSHA256 is the recorded golden hash from
	// .planning/design/_spike/GOLDENS.md. A re-run of TestCaptureHTML on
	// the same machine (same pinned Chromium revision, same fixture, same
	// viewport/scale/color-scheme) must reproduce this exact value.
	htmlGoldenSHA256 = "74f9bebb57c67ba2ee12493ca3fd2b230fd6c257ad0284ebb9eccfb66b570645"
)

// TestCaptureHTML is the runnable entry point `make screenshot-html`
// invokes (via `go test -tags screenshot -run TestCaptureHTML
// ./internal/screenshot/...`). It renders the trivial fixture HTML page
// through the real CaptureHTML -> go-rod (pinned Chromium) path, writes the
// PNG under .planning/design/_spike/html/, and asserts the golden hash
// reproduces on re-run (recorded in .planning/design/_spike/GOLDENS.md).
func TestCaptureHTML(t *testing.T) {
	fixture := filepath.Join("..", "..", ".planning", "design", "_spike", "fixture.html")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("TestCaptureHTML: fixture missing at %s: %v", fixture, err)
	}
	outDir := filepath.Join("..", "..", ".planning", "design", "_spike", "html")

	result, err := CaptureHTML(HTMLOptions{
		FixturePath:       fixture,
		OutDir:            outDir,
		Name:              "spike",
		ViewportWidth:     htmlFixtureViewportWidth,
		ViewportHeight:    htmlFixtureViewportHeight,
		DeviceScaleFactor: 1,
		ColorScheme:       htmlFixtureColorScheme,
		Timeout:           htmlCaptureTimeout,
		AllowDownload:     true,
	})
	if err != nil {
		t.Fatalf("CaptureHTML: %v", err)
	}

	info, statErr := os.Stat(result.PNGPath)
	if statErr != nil {
		t.Fatalf("CaptureHTML: rendered PNG missing at %s: %v", result.PNGPath, statErr)
	}
	if info.Size() == 0 {
		t.Fatalf("CaptureHTML: rendered PNG at %s is empty", result.PNGPath)
	}

	if result.SHA256 != htmlGoldenSHA256 {
		t.Errorf("CaptureHTML: golden hash mismatch -- got %s, want %s (recorded in "+
			".planning/design/_spike/GOLDENS.md); re-run is not reproducing the recorded golden",
			result.SHA256, htmlGoldenSHA256)
	}
}

// TestCaptureHTML_OfflineFailurePath proves the fail-fast offline/failure
// behavior (T-01-SC2): an empty cache dir with provisioning disabled must
// yield a clear, actionable error naming the pinned revision + cache path --
// never a silent download, and never a silent fallback to a different
// browser.
func TestCaptureHTML_OfflineFailurePath(t *testing.T) {
	fixture := filepath.Join("..", "..", ".planning", "design", "_spike", "fixture.html")
	emptyCache := t.TempDir()

	_, err := CaptureHTML(HTMLOptions{
		FixturePath:       fixture,
		OutDir:            t.TempDir(),
		Name:              "offline-probe",
		ViewportWidth:     htmlFixtureViewportWidth,
		ViewportHeight:    htmlFixtureViewportHeight,
		DeviceScaleFactor: 1,
		ColorScheme:       htmlFixtureColorScheme,
		Timeout:           htmlCaptureTimeout,
		CacheDir:          emptyCache,
		AllowDownload:     false,
	})
	if err == nil {
		t.Fatal("CaptureHTML: expected an error for an empty cache dir with AllowDownload=false, got nil")
	}
	if !strings.Contains(err.Error(), emptyCache) {
		t.Errorf("CaptureHTML: expected the error to name the cache path %q, got: %v", emptyCache, err)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("%d", ChromiumRevision)) {
		t.Errorf("CaptureHTML: expected the error to name the pinned revision %d, got: %v", ChromiumRevision, err)
	}
}

// TestProvisionPinnedChromium is the entry point `make setup-env` invokes to
// pre-download the pinned Chromium revision into the fixed cache path ahead
// of time, so a later `make screenshot-html` never has to pay the download
// cost (or fail offline) on a fresh clone. It is deliberately scoped to
// provisioning only -- it does not capture a screenshot (that is
// TestCaptureHTML's job).
func TestProvisionPinnedChromium(t *testing.T) {
	binPath, err := resolveBrowserBinary(launcher.DefaultBrowserDir, ChromiumRevision, true)
	if err != nil {
		t.Fatalf("TestProvisionPinnedChromium: failed to provision pinned Chromium revision %d into %q: %v",
			ChromiumRevision, launcher.DefaultBrowserDir, err)
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("TestProvisionPinnedChromium: provisioned binary missing at %s: %v", binPath, err)
	}
}
