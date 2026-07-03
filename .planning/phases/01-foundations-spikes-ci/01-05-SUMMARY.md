---
phase: 01-foundations-spikes-ci
plan: 05
subsystem: testing
tags: [freeze, go-rod, chromium, screenshot, png, determinism, bubbletea, makefile]

# Dependency graph
requires:
  - phase: 01-foundations-spikes-ci
    provides: existing exec.Command probe/wrapper pattern (internal/platform, internal/tester) mirrored for the freeze/go-rod exec-wrapper shape
provides:
  - "make screenshot-tui — deterministic Bubble Tea View()-dump -> PNG via freeze (vendored font, fixed theme/geometry, stripped metadata, golden hash)"
  - "make screenshot-html — deterministic fixture HTML -> PNG via headless Chromium (go-rod, pinned revision, fixed viewport/scale/color-scheme, golden hash)"
  - "internal/screenshot package: CaptureTUI, CaptureHTML, StripPNGMetadata, HashPNG, Result — build-tag isolated (//go:build screenshot), never linked into the shipped gitid binary"
  - "make setup-env installs freeze@v0.2.2 and provisions the pinned Chromium revision (1321438)"
affects: [02-design-all-mockups]

# Tech tracking
tech-stack:
  added: [charmbracelet/freeze@v0.2.2 (dev tool, go install), github.com/go-rod/rod v0.116.2 (direct go.mod dep, build-tag isolated)]
  patterns: ["build-tag isolated dev/build-tool package (//go:build screenshot)", "exec-wrapper + pure-parse split mirrored from internal/platform", "PNG metadata-strip + SHA-256 golden hash for visual-regression determinism"]

key-files:
  created:
    - internal/screenshot/tui.go
    - internal/screenshot/html.go
    - internal/screenshot/determinism.go
    - internal/screenshot/determinism_test.go
    - internal/screenshot/tui_capture_test.go
    - internal/screenshot/html_capture_test.go
    - internal/screenshot/doc.go
    - .planning/design/fonts/JetBrainsMono-Regular.ttf
    - .planning/design/fonts/README.md
    - .planning/design/_spike/fixture.html
    - .planning/design/_spike/GOLDENS.md
  modified:
    - Makefile
    - go.mod
    - go.sum

key-decisions:
  - "freeze renders a captured View() golden via a bare positional file argument (freeze golden.txt -o out.png --font.file ... --theme ...), NOT --execute \"cat golden\" — confirmed empirically this session that freeze correctly interprets raw ANSI escape codes in a file, resolving RESEARCH.md Open Question 1"
  - "D-04's '100x30 geometry' is the Bubble Tea View() dump's terminal size (columns x rows), not a freeze pixel flag — freeze auto-sizes its output PNG to fit the fixed captured content, which is itself sufficient for reproducibility"
  - "ChromiumRevision pins go-rod v0.116.2's own launcher.RevisionDefault (1321438) as an explicit gitid constant, so a future go-rod upgrade can never silently change which Chromium build screenshot-html downloads"
  - "Browser provisioning (the one-time ~150MB download) is deliberately unbound from the per-capture Timeout — it uses context.Background(), analogous to `go mod download`; only launch+navigate+screenshot are bounded by the request-scoped timeout"
  - "Tasks 2+3 (TUI half + HTML half) were committed together, not as 3 separate per-task commits, per CLAUDE.md's explicit 'commit in logical groups, not per-step chunks' — they share the Makefile edit, the finalizePNG helper, and GOLDENS.md; Task 1 (go.mod/go.sum pin) was committed separately since it stands alone"

patterns-established:
  - "Build-tag isolated dev/build-tool package: every screenshot capture file (tui.go, html.go, determinism.go, and their _test.go files) carries //go:build screenshot; only doc.go is untagged so `go doc` always resolves. Verified: `go list -deps ./cmd/gitid` contains neither go-rod nor freeze."
  - "PNG determinism helpers (StripPNGMetadata + HashPNG) are shared by both capture backends via a single finalizePNG(pngPath) function in tui.go, called from both CaptureTUI and CaptureHTML."
  - "Driving _test.go per capture surface (TestCaptureTUI / TestCaptureHTML) is the concrete runnable entry point a `make` target invokes via `go test -tags screenshot -run TestCapture...` — the make target itself never shells out to freeze/go-rod directly."

requirements-completed: [TOOL-05, DLV-03, TOOL-02]

# Metrics
duration: 55min
completed: 2026-07-03
---

# Phase 1 Plan 05: Screenshot Capture Tooling Summary

**Deterministic `make screenshot-tui` (Bubble Tea View()-dump -> freeze -> PNG) and `make screenshot-html` (fixture HTML -> pinned headless Chromium via go-rod -> PNG), both build-tag isolated from the shipped gitid binary and backed by recorded, reproducing golden SHA-256 hashes.**

## Performance

- **Duration:** 55 min
- **Started:** 2026-07-03T01:56:00Z
- **Completed:** 2026-07-03T02:12:34Z
- **Tasks:** 3
- **Files modified:** 17 (3 Go source, 4 Go test, 1 Makefile, 2 go.mod/go.sum, 7 new asset/doc files)

## Accomplishments
- `make screenshot-tui` renders a trivial Bubble Tea `View()` dump to a deterministic PNG via `freeze`, using a vendored monospace font (JetBrains Mono, OFL-licensed, provenance recorded), a fixed theme, and a fixed 100x30 capture geometry — verified byte-identical across 3 consecutive runs
- `make screenshot-html` renders a trivial fixture HTML page to a deterministic PNG via headless Chromium (go-rod, pinned revision 1321438), at a fixed 1280x800/scale-1/light-color-scheme viewport, bounded by a context timeout — verified byte-identical across 3 consecutive runs (one cold provisioning run + two warm re-runs)
- Both capture backends are isolated behind `//go:build screenshot`; `go list -deps ./cmd/gitid` confirmed neither `go-rod` nor `freeze` ever reaches the shipped binary's dependency graph
- Supply-chain legitimacy for both new dependencies verified automatically (`go mod verify`: all modules verified; exact pins; no `@latest`/checksum-bypass flags), replacing the former blocking-human checkpoint per the plan's Codex-driven `autonomous: true` resolution
- An offline/failure path (`TestCaptureHTML_OfflineFailurePath`) proves `screenshot-html` fails fast with an actionable error naming the pinned revision + cache path when the browser isn't cached and downloading is disabled — never a silent fallback to a different browser

## Task Commits

Tasks 2 and 3 were committed together (see Deviations) since they share the Makefile edit, the `finalizePNG` helper, and `GOLDENS.md` — splitting them would have fragmented one coherent "screenshot tooling" change.

1. **Task 1: Automated supply-chain verification for freeze + go-rod** - `256eb1c` (feat)
2. **Tasks 2+3: Deterministic TUI capture + Deterministic HTML capture** - `e3ace86` (feat)

_No separate "docs: complete plan" metadata commit was made yet — STATE.md/ROADMAP.md/REQUIREMENTS.md updates and this SUMMARY are committed in the final metadata commit that follows._

## Files Created/Modified
- `internal/screenshot/tui.go` - `//go:build screenshot`; `CaptureTUI` (View()-dump golden -> temp .txt -> freeze -> PNG), `finalizePNG` (shared strip+hash), `writeGoldenTempFile`
- `internal/screenshot/html.go` - `//go:build screenshot`; `CaptureHTML` (fixture -> go-rod pinned Chromium -> PNG), `resolveBrowserBinary` (fail-fast offline path), `ChromiumRevision` constant (pins `launcher.RevisionDefault` = 1321438)
- `internal/screenshot/determinism.go` - `//go:build screenshot`; `StripPNGMetadata` (removes tIME/tEXt/zTXt/iTXt chunks, idempotent), `HashPNG` (SHA-256 golden hash)
- `internal/screenshot/determinism_test.go` - table/fixture-driven tests: chunk removal, idempotence, bad-signature rejection, hash stability, hash-unaffected-by-timestamp-metadata
- `internal/screenshot/tui_capture_test.go` - `TestCaptureTUI` (the runnable entry point `make screenshot-tui` invokes) + trivial `fixtureModel` (NOT product UI)
- `internal/screenshot/html_capture_test.go` - `TestCaptureHTML` (runnable entry point `make screenshot-html` invokes), `TestCaptureHTML_OfflineFailurePath`, `TestProvisionPinnedChromium` (runnable entry point `make setup-env` invokes)
- `internal/screenshot/doc.go` - package doc + build-tag isolation rule (untagged, so `go doc` always resolves)
- `.planning/design/fonts/JetBrainsMono-Regular.ttf` + `OFL.txt` - vendored monospace font, tag v2.304, SHA-256 recorded in README
- `.planning/design/fonts/README.md` - font provenance, license, regeneration/verification instructions
- `.planning/design/_spike/fixture.html` - trivial local HTML capture fixture
- `.planning/design/_spike/GOLDENS.md` - supply-chain provenance note (Task 1) + recorded TUI/HTML golden SHA-256 hashes (Tasks 2/3)
- `.planning/design/_spike/tui/spike.png`, `.planning/design/_spike/html/spike.png` - versioned reference PNG artifacts (D-04 layout)
- `Makefile` - `screenshot-tui`/`screenshot-html` targets; `setup-env` extended to install `freeze@v0.2.2` and provision the pinned Chromium revision; `FREEZE_VERSION`/`SCREENSHOT_FONT`/`SCREENSHOT_THEME` vars
- `go.mod`/`go.sum` - `github.com/go-rod/rod v0.116.2` pinned as a direct dependency (via `GOFLAGS=-tags=screenshot go mod tidy`, since plain `go mod tidy` doesn't see build-tag-gated imports)

## Decisions Made
- freeze's exact invocation for a static golden: a bare positional file argument (`freeze <file> -o <png> --font.file <ttf> --theme <name>`), NOT `--execute "cat <file>"` — verified empirically this session that freeze renders raw ANSI escape codes with correct color from a plain file, closing RESEARCH.md's Open Question 1
- D-04's "100x30 geometry" refers to the terminal size (cols x rows) the Bubble Tea `View()` was captured at, not a freeze pixel-output flag; freeze's own `--width`/`--height` flags map to output *pixel* dimensions (empirically confirmed — passing literal `100`/`30` there produced a useless 100x30px image), so they are deliberately NOT passed; freeze auto-sizes its PNG to the fixed captured content instead
- `ChromiumRevision` re-pins go-rod's `launcher.RevisionDefault` (1321438) as an explicit gitid constant rather than trusting whatever go-rod's own default happens to be at build time, so a future go-rod version bump can never silently change the downloaded Chromium build
- Browser provisioning (the download step) uses an unbound `context.Background()`, separate from the per-capture `Timeout` — provisioning is a one-time, potentially slow bootstrap step (like `go mod download`), while launch/navigate/screenshot are what the timeout is meant to bound
- Both `nolint:gosec` annotations follow this repo's established `//nolint:gosec // <reason> (G-code)` convention (matching `internal/tester`, `internal/filewriter`, `internal/gitconfig`) rather than the older `// #nosec` style seen in `internal/platform`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `go mod tidy` (untagged) strips build-tag-gated imports**
- **Found during:** Task 1 (pinning go-rod)
- **Issue:** `go get github.com/go-rod/rod@v0.116.2` followed by a plain `go mod tidy` removed the go-rod requirement entirely, because nothing imports it yet under the default (no-tag) build — `go mod tidy` only scans files whose build constraints are satisfied
- **Fix:** Used `GOFLAGS="-tags=screenshot" go mod tidy` once html.go actually imports go-rod (Task 3), which correctly promotes it to a direct dependency without needing a plain `go get` re-run each task
- **Files modified:** go.mod, go.sum
- **Verification:** `go mod verify` passes; `grep -q "github.com/go-rod/rod v0.116.2" go.mod` succeeds; `go list -deps ./cmd/gitid` still excludes go-rod
- **Committed in:** 256eb1c (Task 1), e3ace86 (Task 3's tidy)

**2. [Rule 1 - Bug] `launcher.Browser.Context` nil-parent panic during provisioning**
- **Found during:** Task 3 (first real `CaptureHTML` run)
- **Issue:** `resolveBrowserBinary`'s `launcher.Browser{}` struct literal left the `Context` field at its zero value (`nil`); go-rod's `fetchup.FastestURL` calls `context.WithCancel(nil)`, which panics ("cannot create context from nil parent")
- **Fix:** Explicitly set `Context: context.Background()` on the `launcher.Browser` used for provisioning (deliberately separate from the per-capture request-scoped `Timeout` context — see Decisions)
- **Files modified:** internal/screenshot/html.go
- **Verification:** `TestCaptureHTML` and `TestProvisionPinnedChromium` both pass; provisioning downloaded and cached the pinned Chromium revision successfully
- **Committed in:** e3ace86

**3. [Rule 1 - Bug] gosec findings under the `screenshot` build tag (invisible to default `make lint`)**
- **Found during:** Task 2/3 close-out (ran `golangci-lint run --build-tags screenshot ./internal/screenshot/...` proactively, since default `make lint` never compiles build-tag-gated files)
- **Issue:** G306 (WriteFile permissions `0o640` flagged; repo convention is `0o600` or less), G115 (int->uint32 conversion in a test fixture helper), G703 (taint-tracked path-traversal warning on a `os.WriteFile` call whose path originates from a function parameter)
- **Fix:** Changed both PNG `WriteFile` calls to `0o600`; added `//nolint:gosec` annotations with G-code + rationale, matching this repo's established convention (`internal/tester`, `internal/filewriter`, `internal/gitconfig`)
- **Files modified:** internal/screenshot/tui.go, internal/screenshot/html.go, internal/screenshot/determinism_test.go
- **Verification:** `golangci-lint run --build-tags screenshot ./internal/screenshot/...` reports 0 issues; `make lint` (untagged) also still passes
- **Committed in:** e3ace86

---

**Total deviations:** 3 auto-fixed (all Rule 1 — bugs found and fixed during real end-to-end execution, not design changes)
**Impact on plan:** All three were necessary for the tooling to actually work / pass lint under its own build tag. No scope creep — no architectural changes, no new files beyond what the plan specified.

## Issues Encountered
None beyond the auto-fixed items above. The pinned Chromium revision (1321438) downloaded successfully from `storage.googleapis.com/chromium-browser-snapshots` (~150MB, ~75s cold) and cached correctly at `launcher.DefaultBrowserDir` for fast (~2.5s) warm re-runs.

## User Setup Required
None - no external service configuration required. `make setup-env` (run on a fresh clone) installs `freeze@v0.2.2` and provisions the pinned Chromium revision automatically; both are dev/build-tool concerns only.

## Next Phase Readiness
Phase 2 (DESIGN — All Mockups) can call `internal/screenshot.CaptureTUI` / `CaptureHTML` directly against its real design-approved TUI models and HTML/`mui` mockups — the exported signatures (`CaptureTUI(golden string, opts TUIOptions) (Result, error)`, `CaptureHTML(opts HTMLOptions) (Result, error)`) and the `make screenshot-tui`/`make screenshot-html` entry points are stable and proven end-to-end against trivial fixtures. No blockers.

---
*Phase: 01-foundations-spikes-ci*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 16 created/modified files verified present on disk; both task commits
(`256eb1c`, `e3ace86`) verified present in `git log`.
