---
phase: 02-design-all-mockups-checkpoint-1
reviewed: 2026-07-03T00:00:00Z
depth: deep
files_reviewed: 34
files_reviewed_list:
  - Makefile
  - cmd/gitid-dummy/main.go
  - cmd/gitid-dummy/main_test.go
  - e2e/dummy_nav_e2e_test.go
  - e2e/harness_test.go
  - internal/dummytui/data.go
  - internal/dummytui/doc.go
  - internal/dummytui/keyowners_test.go
  - internal/dummytui/model.go
  - internal/dummytui/model_test.go
  - internal/dummytui/nobackend_test.go
  - internal/dummytui/overlay.go
  - internal/dummytui/registry.go
  - internal/dummytui/registry_test.go
  - internal/dummytui/shell.go
  - internal/dummytui/surface_createflow.go
  - internal/dummytui/surface_createflow_test.go
  - internal/dummytui/surface_fixer.go
  - internal/dummytui/surface_fixer_test.go
  - internal/dummytui/surface_gitscreen.go
  - internal/dummytui/surface_gitscreen_test.go
  - internal/dummytui/surface_globalgit.go
  - internal/dummytui/surface_globalgit_test.go
  - internal/dummytui/surface_globalssh.go
  - internal/dummytui/surface_globalssh_test.go
  - internal/dummytui/surface_health.go
  - internal/dummytui/surface_health_test.go
  - internal/dummytui/surface_identitymanager.go
  - internal/dummytui/surface_identitymanager_test.go
  - internal/screenshot/design_adapter.go
  - internal/screenshot/design_capture_test.go
  - internal/screenshot/html.go
  - internal/screenshot/manifest.go
  - internal/screenshot/manifest_test.go
findings:
  critical: 0
  warning: 2
  info: 2
  total: 5
status: issues_found
---

# Phase 2 (checkpoint 1): Code Review Report — Go correctness pass

**Reviewed:** 2026-07-03
**Depth:** deep (cross-file trace + live build/test/e2e execution, not just static read)
**Files Reviewed:** 34 (`internal/dummytui`, `internal/screenshot`, `cmd/gitid-dummy`, `e2e/dummy_nav_e2e_test.go`, `e2e/harness_test.go`, `Makefile`; diff `321884c..HEAD`)
**Status:** issues_found (no CRITICAL, 1 HIGH, 1 MEDIUM, 2 LOW)

## Summary

This is a navigation-only Bubble Tea "dummy" TUI plus a screenshot/manifest
capture harness — no backend logic, no writes to real user files. The
architecture is disciplined and unusually well self-documented: the
allowlist-based no-backend guard (`nobackend_test.go`) is real and I could
not find a bypass (verified independently by running `go list -deps
./cmd/gitid-dummy/... ./internal/dummytui/...` myself — it returns exactly
the two allowed packages). The registry/routing state machine
(`registry.go`) has a genuine registration-time collision guard (not just a
runtime one), and the test suite (`internal/dummytui/*_test.go`,
`internal/screenshot/manifest_test.go`) is substantive — real navigation
graphs, real breadcrumb/signature assertions, real negative checks (e.g.
health's LOW-11 "never a write-ceremony marker" test) — not hollow
assert-true placeholders.

I did not stop at reading the diff: I built the binaries, ran `go vet`, ran
`go test -race` on `internal/dummytui` and `cmd/gitid-dummy`, and ran the
**full** `dummy-nav-e2e` PTY walk (all 50 manifest screens, real 80×24 PTY,
real `cmd/gitid-dummy` binary) to verify behavior empirically rather than by
inspection alone. That run is what surfaced the one HIGH finding below: it
is a real, reproducible rendering defect that every existing test (unit,
manifest cross-validation, and the e2e PTY walker itself) is structurally
blind to, because all of them only substring-match a breadcrumb/signature
and never bound line width against the real terminal.

An independent Codex review is running in parallel per the task brief;
findings below are mine, arrived at by direct execution and inspection of
this codebase, not inference from the other review.

## Critical Issues

None found.

## High

### HI-01: identity-manager's own 5 "modal" screens render for a hardcoded 100×30 canvas and get clipped on the real, documented-minimum 80×24 terminal

**File:** `internal/dummytui/surface_identitymanager.go:229-244` (`imOverlay`), contrast with the correct live-geometry path at `internal/dummytui/model.go:99-177` (`renderContent`)

**Issue:** `action-menu`, `clone-name-prompt`, `delete-choice`,
`confirm-destructive`, and `backup-notice` are the identity-manager
surface's own intra-surface "modal-style" screens. Unlike the two
cross-surface keyless modals (`create-flow`, `git-screen`), which are
composited by `model.go`'s `renderContent()` using the model's **live**
`m.width`/`m.height` (correctly updated from `tea.WindowSizeMsg`, see
`model.go:57-60`), these five screens composite themselves **inside their
own `Render()` function** via `imOverlay()`, which hardcodes
`defaultWidth`/`defaultHeight` (100×30, `model.go:11-16`) regardless of the
real terminal size:

```go
// internal/dummytui/surface_identitymanager.go:229-244
func imOverlay(title, sig string, lines ...string) string {
	modal := styleIMModal.Render(imBody(title, sig, lines...))
	bg := padToHeight(styleIMDim.Render(imListBody()), defaultHeight)
	...
	mw := modalWidth(defaultWidth)   // always min(100-8,72)=72, NOT the real terminal width
	mh := lipgloss.Height(bounded)
	x, y := modalOrigin(defaultWidth, defaultHeight, mw, mh)  // centers for a 100-col canvas
	return placeOverlay(x, y, bounded, bg)
}
```

`ui_pty_e2e_test.go` itself documents 80×24 as "the minimum size gitid
supports." On that terminal, `modalWidth(100)` still returns 72, and
`modalOrigin(100, 30, 72, mh)` centers the box at `x=14`, so the box spans
columns 14–86 — six columns past the actual 80-column edge, on **every**
line the box renders.

**Proof (not inference — I built and ran this):**

```
$ go test -tags e2e -run TestDummyNavReachesAllScreens -v ./e2e/...
...
--- PASS: TestDummyNavReachesAllScreens (44.91s)
```

The suite passes (it only substring-checks breadcrumb + signature), but the
saved PTY frames tell a different story. Max raw line length per captured
frame (80-col real terminal; ~4 bytes/char is normal ANSI-code overhead, so
80-86 is "fits", 212+ is not):

```
84  dummy-nav-identity-manager-list-populated.txt   (fits)
80  dummy-nav-identity-manager-detail-ssh-first.txt (fits)
212 dummy-nav-identity-manager-action-menu.txt        <-- overflow
212 dummy-nav-identity-manager-backup-notice.txt       <-- overflow
212 dummy-nav-identity-manager-clone-name-prompt.txt   <-- overflow
212 dummy-nav-identity-manager-delete-choice.txt        <-- overflow
214 dummy-nav-identity-manager-confirm-destructive.txt  <-- overflow
```

All 45 other screens across the other 6 surfaces (create-flow, git-screen,
global-ssh, global-git, health, fixer) stay within 80-86 raw bytes — this
defect is isolated to identity-manager's 5 self-composited screens. Visual
inspection of the captured `action-menu` frame confirms real, visible
truncation, not just an internal width miscalculation:

```
              ┌─────────────────────────────────────────────────────────────────
              │ Action menu — personal
              │ View SSH-first detail
              │ Clone (c) — create a new identity from this one, under a distinc
              │ Generate new key — rotate this identity's key (MGR-05).
              │ Delete (d) — choose Git-identity-only, or delete everything (MGR
              │
              │ [SIG-IM-ACTION-MENU]
              └─────────────────────────────────────────────────────────────────
```
(box right border never closes; every content line is cut off mid-word)

This isn't cosmetic-only: `confirm-destructive` is exactly the "cannot be
undone" ceremony copy §5 calls the strongest confirm short of a typed
confirmation — that's the text most likely to be partially unreadable on
the tool's own documented minimum terminal size.

**Why it slipped through:** every layer of test coverage for this surface
(unit tests in `surface_identitymanager_test.go`, `manifest_test.go`'s
`TestManifestCrossValidation`, and `dummy_nav_e2e_test.go`'s PTY walker)
only asserts `strings.Contains(output, breadcrumb/signature)`. None bounds
rendered line width against a real or even a nominal terminal width, so a
100-column-wide render passes every existing check even on an 80-column
terminal.

**Fix:** Thread the real viewport into identity-manager's self-composited
screens instead of hardcoding `defaultWidth`/`defaultHeight`. The cleanest
option given `ScreenDef.Render` is a zero-arg `func() string`: give the
package a `currentViewport` (width, height) that `model.go`'s `Update()`
sets on every `tea.WindowSizeMsg` (defaulting to `defaultWidth`/
`defaultHeight` for the static `RenderScreen()`/manifest-capture callers,
matching today's behavior there), and have `imOverlay` read it instead of
the package constants:

```go
var currentViewport = struct{ w, h int }{defaultWidth, defaultHeight}

func imOverlay(title, sig string, lines ...string) string {
	...
	mw := modalWidth(currentViewport.w)
	...
	x, y := modalOrigin(currentViewport.w, currentViewport.h, mw, mh)
	...
}
```
and in `model.go`'s `Update()`:
```go
case tea.WindowSizeMsg:
	m.width, m.height = msg.Width, msg.Height
	currentViewport.w, currentViewport.h = msg.Width, msg.Height
	return m, nil
```
Also add a width-bound assertion to
`TestIdentityManager_ModalScreensUsePlaceOverlayWithoutPanic` (or a new
test) so a future regression is caught by `go test`, not by a manual PTY
frame inspection.

## Medium

### MED-01: the Signature field's documented "never a same-shaped-but-wrong-state false positive" verification is not actually implemented in the offline capture/cross-validation tests — only in the e2e PTY walker

**File:** `internal/screenshot/design_capture_test.go:107,125-126`, `internal/screenshot/manifest_test.go:166-167`; contrast with `e2e/dummy_nav_e2e_test.go:215-216`

**Issue:** Every one of the seven `surface_*.go` files repeats a comment
along these lines (e.g. `surface_health.go:42-46`):

> "Each screen's Render also embeds its manifest.json 'signature'... so
> **design_capture_test.go's TUI subtest and the PTY dummy-nav e2e** can
> both assert a capture landed on the RIGHT screen, never a
> same-shaped-but-wrong-state false positive (review HIGH-3c, T-02-FP)."

In practice only the e2e PTY walker honors that contract:

```go
// e2e/dummy_nav_e2e_test.go:215-216
last, ok := s.waitFor(5*time.Second, func(text string) bool {
	return strings.Contains(text, screenID) && strings.Contains(text, e.Signature)
})
```

`design_capture_test.go`'s TUI subtest (the one the comment explicitly
names) only checks the breadcrumb:

```go
// internal/screenshot/design_capture_test.go:125-126
if !strings.Contains(view, ScreenID(e)) {
	t.Fatalf(... missing the %q breadcrumb ...)
}
```

...and its HTML subtest passes `ScreenID(e)` (not `e.Signature`) as
`CaptureHTMLScreen`'s `requiredText` (`design_capture_test.go:107`).
`manifest_test.go`'s `TestManifestCrossValidation` does the same
(`manifest_test.go:166-167`). Both are gated behind the `screenshot`
build tag so I could not execute them here (no Chromium/`freeze` in this
sandbox), but this is a static, directly-readable code fact, not an
inference: `e.Signature` is loaded by `LoadManifests` and never referenced
anywhere in `design_capture_test.go` or `manifest_test.go`.

Since `Screen` IDs are already 1:1 with UI state (e.g. `list-populated` vs.
`list-empty` are different `Screen` values, hence different breadcrumbs),
the actual functional risk from this specific gap is low — but the
repeated doc comment is factually inaccurate about what the offline
(non-e2e) test suite checks, and the golden-screenshot pipeline
(`screenshot-tui-mockups`/`screenshot-html-mockups`, the artifacts a human
reviewer actually looks at) has strictly weaker protection against a
same-route-different-content bug than the e2e walker does, contrary to
what every surface file claims.

**Fix:** Either (a) add the `e.Signature` check to both subtests in
`design_capture_test.go` and to `manifest_test.go`'s
`TestManifestCrossValidation` so the code matches the seven repeated doc
comments, or (b) correct the doc comments in all seven `surface_*.go` files
to say the signature is verified only by the e2e PTY walker. (a) is
strictly cheaper and closes a real (if currently low-probability) gap for
free — `LoadManifests` already guarantees signature uniqueness.

## Low

### LOW-01: `NewModel()` silently swallows a missing "identity-manager" registration instead of failing loudly

**File:** `internal/dummytui/model.go:31-41`

**Issue:**
```go
func NewModel() Model {
	sd, _ := lookupSurface("identity-manager")
	return Model{
		width:  defaultWidth,
		height: defaultHeight,
		nav: navState{
			view:         "identity-manager",
			activeScreen: entryScreenID(sd),
		},
	}
}
```
If `identity-manager` were ever not registered (e.g. a future refactor
drops its `init()`, or file-ordering assumptions change), `sd` is the zero
`SurfaceDef{}`, `entryScreenID(sd)` returns `""` (its own `len(sd.Screens)
== 0` guard), and the app silently boots into a broken home screen
(`renderContent()` would then hit its own `findScreen` failure path and
show `"dummytui: unknown screen ... on surface identity-manager"` instead
of crashing at startup with a clear cause). Today this can't actually
happen (package `init()` order guarantees registration before `NewModel()`
runs), so this is not currently exploitable — it's a latent landmine, not a
live bug.

**Fix:** `sd, ok := lookupSurface("identity-manager"); if !ok { panic(...) }`
— fail fast and loud at startup rather than degrading to a confusing
runtime error string, consistent with how `registry.go`'s own
`Register`/`RegisterOrReplace` already prefer panicking over silent
degradation elsewhere in this package.

### LOW-02: `hlthParseErrorSnippet` (Go) diverges cosmetically from `healthParseErrorTarget.snippet` (TS fixture)

**File:** `internal/dummytui/surface_health.go:154`, compare `.planning/design/mockup-src/src/data/recipeFixtures.ts:1104`

**Issue:** The TS fixture keeps `line` and `snippet` as separate fields:
```ts
line: 4,
snippet: '    signingkey = "~/.ssh/id_ed25519_work.pub',
```
The Go mirror folds the line number into the snippet string itself:
```go
hlthParseErrorSnippet = "line 4:     signingkey = \"~/.ssh/id_ed25519_work.pub"
```
This is presentational grouping, not a value/order divergence (the parity
rubric in `02-UX-DIRECTION.md` §3 explicitly allows layout differences
between media), so it is not a correctness bug — but every other
recipe-critical constant in this file is called out in comments as
"byte-identical... not derived," and this one silently isn't, which could
confuse a future contributor diffing the two fixture sets for drift.

**Fix:** Either add a one-line comment at `hlthParseErrorSnippet`'s
declaration noting the intentional `line 4:` prefix (so it reads as a
deliberate TUI-only compaction, matching the precedent already documented
for `gsFieldsCompactLine1-3` and `ggitCompactValueLines`), or split it into
`hlthParseErrorLine = 4` + a bare snippet to mirror the TS shape exactly.

## Areas checked with no findings worth reporting

- **DLV-05 no-backend allowlist** (`nobackend_test.go`): verified real,
  not just asserted — I independently ran `go list -deps
  ./cmd/gitid-dummy/... ./internal/dummytui/...` and it returns exactly
  `internal/dummytui` and `cmd/gitid-dummy`, nothing else first-party.
- **Registry/routing correctness**: `route()`/`routeModal()`/
  `routeTopLevel()` precedence (intra-surface keys > LaunchKey > number
  keys) is correct and covered by real tests
  (`TestRoutePrecedence_ScreenKeysBeforeLaunchKey`,
  `TestModalLaunch_NumberKeysIgnoredWhileModalActive`); the
  registration-time `collisionCheck` correctly rejects both directions of
  LaunchKey/ScreenDef.Keys collision (verified by table-driven panic
  tests). No key-owner ambiguity found; `TestKeyOwners_FinalFiveOwnNumberKeys`
  and my own `go list`/build confirm exactly 5 activation-key owners.
- **`overlay.go` compositing**: `placeOverlay`/`overlayLine` correctly
  clamp out-of-bounds rows (no panic on an oversized modal, verified by
  `TestPlaceOverlay_ClampsOversizedModalWithoutPanic` and by my own read of
  the ANSI-aware truncation logic) — the HI-01 defect above is a geometry
  **input** problem (wrong width/height fed in from
  `surface_identitymanager.go`), not a bug in `overlay.go`'s own math.
- **`internal/screenshot/manifest.go`**: schema validation
  (`validateEntry`) and the three uniqueness checks (ScreenID, HTMLRoute,
  Signature) are correct and covered by real negative tests including a
  cross-manifest duplicate case. `LoadManifests`'s "no manifests found ->
  empty slice, nil error" contract is intentional and documented, not a
  silent failure mode.
- **`internal/screenshot/design_adapter.go` / `html.go` additions**:
  `URLFragment`/`RequiredText` are additive, backward-compatible
  (`URLFragment` defaults to `""`, preserving the exact pre-existing
  single-fixture path). `RequiredText` is checked strictly before the PNG
  is captured/written (no wrong-route or blank capture can silently
  succeed). `allow-file-access-from-files` is scoped to a `file://`-only
  navigation path (`CaptureHTMLScreen` rejects any `url` not starting with
  `file://`) — no relaxation of cross-origin behavior toward a remote
  origin. Confirmed no `go-rod`/`freeze` import reaches `cmd/gitid` or any
  non-`screenshot`-tagged file (`grep` across `cmd/` and `internal/`
  excluding `internal/screenshot/*` and `_test.go` files returns nothing).
- **`recipeFixtures.ts` / recipe coherence**: spot-checked against
  `recipes/ssh-config.recipe` / `recipes/gitconfig.recipe`. Alias-per-identity
  block (`Port 443`, `IdentitiesOnly yes`), `includeIf` (both `hasconfig:`
  and `gitdir:` variants), `allowed_signers` email byte-match with
  `user.email`, and the `insteadOf` block are all recipe-accurate. The
  Wave-1 HIGH fix (insteadOf using the provider host `git@github.com:`
  rather than the identity alias) is confirmed still in place
  (`recipeFixtures.ts:82-83`, mirrored in
  `surface_createflow.go`'s `cfMacGlobalsBlock`/related constants). The
  one intentional, explicitly-documented divergence — git-screen's
  `~/.gitconfig.d/<identity>` fragment path vs. the recipe's own
  `~/.gitconfig_<identity>` — is called out in-file per CLAUDE.md's
  "surface any divergence explicitly" instruction, not silently reused.
- **Race/build health**: `go build ./...`, `go vet` (dummytui/screenshot/
  gitid-dummy), and `go test -race ./internal/dummytui/... ./cmd/gitid-dummy/...`
  all pass clean. Full `dummy-nav-e2e` (50/50 manifest screens, real PTY,
  real binary) passes functionally (see HI-01 for the caveat: it passes
  while being blind to the rendering defect it should be catching).

---

_Reviewed: 2026-07-03_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
