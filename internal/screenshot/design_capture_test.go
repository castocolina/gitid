//go:build screenshot

package screenshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/dummytui"
)

// repoRootFromPackage resolves the repository root from this package's
// source directory. go test's working directory is always the package's
// source directory regardless of invocation cwd — same convention as
// tui_capture_test.go/html_capture_test.go's ".." "..".
func repoRootFromPackage(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repoRootFromPackage: %v", err)
	}
	return dir
}

// countPNGs returns the number of *.png files directly under dir (0, not
// an error, if dir does not exist yet).
func countPNGs(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("countPNGs: ReadDir(%q): %v", dir, err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".png") {
			n++
		}
	}
	return n
}

// TestCaptureAllMockupScreens is the runnable entry point
// `make screenshot-html-mockups` / `make screenshot-tui-mockups` invoke
// (via `-run 'TestCaptureAllMockupScreens/.*/html'` /
// `-run 'TestCaptureAllMockupScreens/.*/tui'`). It loads every
// .planning/design/*/manifest.json entry (LoadManifests), groups by
// surface into per-surface subtests, and for each screen runs:
//
//   - an "html" subtest: builds
//     file://<repoRoot>/.planning/design/mockup-src/dist/index.html#<HTMLRoute>
//     and calls CaptureHTMLScreen, which -- via the SAME Phase-1 CaptureHTML
//     path -- asserts the rendered page's body contains the
//     "<surface>/<screen>" breadcrumb BEFORE ever saving a PNG (review
//     HIGH-3b/d + MEDIUM-9: a missing/wrong route fails hard, never a blank
//     PNG), then saves .planning/design/<surface>/html/<screen>.png.
//   - a "tui" subtest: calls dummytui.RenderScreen(surface, screen) --
//     which renders any registered screen, including keyless modal
//     screens, standalone -- asserts the SAME breadcrumb is present in the
//     returned string, then calls CaptureTUIScreen to save
//     .planning/design/<surface>/tui/<screen>.png.
//
// After each surface's html/tui screens are captured, it asserts the PNG
// COUNT on disk for that surface/kind equals the manifest-computed count
// (the fan-out-safe coverage invariant: N screens in the manifest -> N
// PNGs on disk, never fewer from a silently-skipped capture and never more
// from stale leftovers).
//
// With no manifest.json files checked in yet (this plan ships the loader,
// not the manifests -- surfaces add their own manifest.json in later
// plans), LoadManifests returns zero entries and this is a no-op PASS:
// zero subtests run, zero PNGs written, zero count assertions. That is the
// documented behavior (02-03-PLAN.md acceptance criteria: "passes as a
// no-op ... never a silent blank-PNG pass" -- an empty manifest set is not
// a blank-PNG pass, it is zero captures).
func TestCaptureAllMockupScreens(t *testing.T) {
	root := repoRootFromPackage(t)
	designDir := filepath.Join(root, ".planning", "design")

	entries, err := LoadManifests(designDir)
	if err != nil {
		t.Fatalf("LoadManifests(%q): %v", designDir, err)
	}

	bySurface := SurfacesByEntries(entries)
	distIndex := filepath.Join(designDir, "mockup-src", "dist", "index.html")

	for _, surface := range SortedSurfaceNames(bySurface) {
		screens := bySurface[surface]
		t.Run(surface, func(t *testing.T) {
			htmlDir := filepath.Join(designDir, surface, "html")
			tuiDir := filepath.Join(designDir, surface, "tui")

			t.Run("html", func(t *testing.T) {
				if _, statErr := os.Stat(distIndex); statErr != nil {
					t.Fatalf("mockup dist not built at %s (run `pnpm build` under .planning/design/mockup-src first): %v", distIndex, statErr)
				}
				for _, e := range screens {
					e := e
					t.Run(e.Screen, func(t *testing.T) {
						url := "file://" + filepath.ToSlash(distIndex) + "#" + e.HTMLRoute
						out := filepath.Join(htmlDir, e.Screen+".png")
						if err := CaptureHTMLScreen(url, out, ScreenID(e)); err != nil {
							t.Fatalf("CaptureHTMLScreen(%s): %v", ScreenID(e), err)
						}
					})
				}
				if got, want := countPNGs(t, htmlDir), len(screens); got != want {
					t.Errorf("html PNG count invariant: got %d PNGs under %s, want %d (one per manifest screen)", got, htmlDir, want)
				}
			})

			t.Run("tui", func(t *testing.T) {
				for _, e := range screens {
					e := e
					t.Run(e.Screen, func(t *testing.T) {
						view, err := dummytui.RenderScreen(e.Surface, e.Screen)
						if err != nil {
							t.Fatalf("dummytui.RenderScreen(%s): %v", ScreenID(e), err)
						}
						if !strings.Contains(view, ScreenID(e)) {
							t.Fatalf("dummytui.RenderScreen(%s) output is missing the %q breadcrumb -- refusing to save a wrong-screen PNG", ScreenID(e), ScreenID(e))
						}
						out := filepath.Join(tuiDir, e.Screen+".png")
						if err := CaptureTUIScreen(view, out); err != nil {
							t.Fatalf("CaptureTUIScreen(%s): %v", ScreenID(e), err)
						}
					})
				}
				if got, want := countPNGs(t, tuiDir), len(screens); got != want {
					t.Errorf("tui PNG count invariant: got %d PNGs under %s, want %d (one per manifest screen)", got, tuiDir, want)
				}
			})
		})
	}

	if len(entries) == 0 {
		t.Logf("TestCaptureAllMockupScreens: no manifest.json files found under %s -- 0 screens captured (expected until later Phase 2 plans add manifests)", designDir)
	}
}
