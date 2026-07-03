// Package screenshot provides the repeatable, deterministic capture tooling
// used by `make screenshot-tui` and `make screenshot-html` (TOOL-05, DLV-03).
// It renders a Bubble Tea View() dump and a fixture HTML page to versioned,
// reproducible PNGs so later phases (starting with Phase 2's design mockups)
// have a stable golden-image workflow.
//
// # Build-tag isolation
//
// Every file in this package other than this one carries the build constraint
//
//	//go:build screenshot
//
// The two capture backends — charmbracelet/freeze (invoked as an external
// binary, no Go import) and github.com/go-rod/rod (a real Go module
// dependency, headless-Chromium driver) — are dev/build-tool concerns only.
// They must never enter the shipped gitid binary's dependency graph. The
// `screenshot` build tag guarantees that: `go build ./cmd/gitid` (no tags)
// never compiles tui.go, html.go, or determinism.go, so go-rod is never
// linked into the shipped binary and `go list -deps ./cmd/gitid` never
// mentions it.
//
// Only the `screenshot`-tagged driving tests (TestCaptureTUI, TestCaptureHTML)
// and the `make screenshot-tui` / `make screenshot-html` targets that invoke
// them (via `go test -tags screenshot`) ever compile this package's real
// implementation files. This file has no build tag so `go doc` and IDE
// tooling can always see the package documentation, even without the tag.
package screenshot
