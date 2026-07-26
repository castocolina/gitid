---
phase: 03-create-flow-backend
plan: 02
subsystem: shared render stack (internal/tuikit) + Backend seam
tags: [tuikit, dummytui, backend-seam, view-dtos, d-17, extraction]
requires:
  - "internal/keygen.ScanReusableKeys / internal/tester.ResolvedViaCommand (03-01) — consumed later through the seam, never imported by tuikit"
provides:
  - "internal/tuikit — the shared, backend-free render stack both binaries draw through"
  - "tuikit.Backend — the ONE injected seam (data + create-flow effects)"
  - "tuikit.ReusableKeyView / TestOutcome / TestResultView / CreateSpec / GitSpec / WritePlanView — the view DTOs the seam speaks"
  - "tuikit.AlgorithmCatalog + the frozen design vocabulary (design.go)"
  - "dummytui.FixtureBackend — the fixture implementation the live demo runs on"
affects:
  - cmd/gitid-dummy (now tuikit.NewApp(dummytui.NewFixtureBackend()))
  - cmd/gitid (plan 03-03 adds the real composition root against the same seam)
tech-stack:
  added: []
  patterns:
    - "One injected seam per package boundary; nil Backend panics at construction rather than dying silently"
    - "View DTOs at the boundary so the render package can never name a backend type"
    - "Test-only second Backend implementation inside the package under test, to break an import cycle without weakening assertions"
key-files:
  created:
    - internal/tuikit/backend.go
    - internal/tuikit/views.go
    - internal/tuikit/design.go
    - internal/tuikit/doc.go
    - internal/tuikit/backend_stub_test.go
    - internal/dummytui/fixturebackend.go
  modified:
    - internal/tuikit/app.go (moved from internal/dummytui)
    - internal/tuikit/identities.go (moved; every create-flow effect routed through Backend)
    - internal/tuikit/store.go (moved; Seed removed, Reset delegated to the Backend)
    - internal/tuikit/{ceremony,globalssh,globalgit,doctor,frame,theme,fixplans}.go (moved, no logic change)
    - internal/dummytui/data.go
    - internal/dummytui/data_test.go
    - cmd/gitid-dummy/main.go
    - cmd/gitid-dummy/main_test.go
decisions:
  - "Reduce no longer answers Reset — only a Backend knows what 'initial' means, so Persist must; both halves of that contract are asserted"
  - "tuikit's internal tests get a test-only stubBackend instead of importing dummytui (which would be an import cycle); _test.go files never appear in go list -deps, so the no-backend gate is unaffected"
  - "DemoBanner returns false for every tab in the dummy — the whole dummy IS demo data, so a per-tab banner would be noise and would change the frozen render"
metrics:
  requirements: [DLV-04, SSHUI-03]
  commit: 3dd4f47
  completed: 2026-07-25
---

# Phase 3 Plan 02: internal/tuikit + Backend seam Summary

The whole approved render stack now lives in `internal/tuikit` behind one
injected `Backend`; the live demo renders byte-identically through
`dummytui.FixtureBackend`, and the package's import graph contains zero
first-party backend packages.

## What was built

### `internal/tuikit` — the shared render stack

Moved verbatim out of `internal/dummytui`: `app.go`, the four screen models
(`identities.go` including the create wizard, `globalssh.go`, `globalgit.go`,
`doctor.go`), `ceremony.go`, `frame.go`, `theme.go`, `fixplans.go`, and the
`DemoState`/`Action`/`Reduce` machinery (`store.go`). The three
non-create-flow screens moved with **zero logic change** — they remain pure
`DemoState` renderers.

The package reads no file, shells out to nothing, and imports no first-party
backend package. Three new files define the boundary:

- **`backend.go`** — the `Backend` interface: `InitialState`, `DemoBanner`,
  `Persist`, plus the create-flow effects (`AlgorithmCatalog`,
  `ProviderDefaults`, `DefaultMatchStrategy`, `HostBlockPreview`,
  `GitFragmentPreview`, `IncludeIfPreview`, `AliasCollision`,
  `ScanReusableKeys`, `TestConfigPath`, `Stage1Command`, `Stage2Command`,
  `TestStage1`, `TestStage2`, `ResolvedStorageTarget`, `CreateWritePlan`,
  `CopyPublicKey`) and the `WizardStageMsg` both test stages deliver.
- **`views.go`** — the tuikit-LOCAL DTOs every signature speaks:
  `ReusableKeyView`, `TestOutcome` (exactly three constants),
  `TestResultView`, `CreateSpec`, `GitSpec`, `WritePlanView`. These are what
  close Codex HIGH #5: the seam can never name a `keygen`/`tester` type, so
  the no-backend gate holds by construction rather than by allowlist.
- **`design.go`** — the frozen design vocabulary both binaries must render
  identically (state/severity taxonomies, the Global SSH/Git option catalogs,
  the KEY-01 algorithm catalog, managed-block sentinels), split out of the
  fixtures with its values unchanged byte for byte.

`NewApp(b Backend)` seeds from `b.InitialState()`, hands `b` to the screens
that own effects, and routes every committed mutation through `b.Persist`. A
nil `Backend` **panics at construction** — the project's recurring
injected-seam wiring blindspot means a dead seam must fail loudly, not
silently.

### `internal/dummytui` — fixtures + `FixtureBackend`

`data.go` keeps the recipe fixtures (identity rows, health findings, the
create-flow and Git-screen literals) and now references the moved taxonomies
as `tuikit.*`. New `fixturebackend.go` implements every seam method from
those fixtures, reproducing the pre-extraction behavior value for value:
`InitialState` is the old `Seed()` (rows + findings + the
`findingIdentityAttribution` map), `Persist` answers `Reset` with
`InitialState()` and delegates everything else to `tuikit.Reduce`, the preview
methods return the exact former inline strings, `TestStage1`/`TestStage2`
deliver a `WizardStageMsg` after the same 350 ms tick, `CreateWritePlan`
returns the review ceremony's former targets/backups, and `CopyPublicKey`
returns the demo receipt while touching nothing. `DemoBanner` is `false` for
every tab.

`ScanReusableKeys` is derived from the same `IdentityManagerRows` the Identity
Manager renders, so the picker's D-12 "in use by" labels are traceably the
same data rather than a second, divergent list.

### `cmd/gitid-dummy`

Now `tea.NewProgram(tuikit.NewApp(dummytui.NewFixtureBackend()))`. Its smoke
test was rewritten to construct through that **same real wiring** and to
assert the fixture seed actually reaches the frame (`8 ids`), so a seam that
is declared but never wired cannot pass silently (L2).

## Deviations from Plan

### 1. [Rule 3 — Blocking] Test-only `stubBackend` inside `internal/tuikit`

**Found during:** finishing Task 2.
**Issue:** `internal/tuikit`'s ~48 existing tests are `package tuikit` — they
reach unexported models and helpers, so they cannot move to an external test
package. They call `NewApp()` with no argument, and they cannot import
`internal/dummytui` for a Backend because `dummytui` imports `tuikit`
(import cycle). The plan reasoned about the production import graph but not
the test one (recorded as LEARNINGS L12).
**Fix:** added `internal/tuikit/backend_stub_test.go` defining `stubBackend`,
a second `Backend` implementation mirroring the fixtures, plus a package-local
`Seed()` helper the store tests already used. All 44 `NewApp()` call sites
became `NewApp(stubBackend{})`. Being a `_test.go` file it never appears in
`go list -deps ./internal/tuikit`, so the no-backend import graph is
unaffected. **No existing assertion was weakened, skipped or deleted.**
The duplication risk is covered from both ends: `dummytui`'s
`TestFixtureConsistency` keeps the fixture side coherent, and the e2e PTY walk
drives the REAL `FixtureBackend` wiring end to end.

### 2. [Rule 1 — Contract moved, not weakened] `TestReduceReset` → `TestReset`

**Found during:** running the moved store tests.
**Issue:** `store.go` (as extracted) deliberately removed `Reset` from
`Reduce` — only a Backend knows what "initial" means, so `Persist` must answer
it. The old `TestReduceReset` asserted `Reduce(s, Reset{})` restores the seed
and therefore failed.
**Fix:** renamed to `TestReset` and asserted the **same** contract at the seam
that now owns it, plus the new half: `Persist(s, Reset{})` restores the seeded
state AND `Reduce(s, Reset{})` leaves state untouched. Strictly stronger than
before.

### 3. Two fixture references repointed after the split

`internal/dummytui/data_test.go` used `IdentityManagerGlyphByState`, which
moved to `tuikit/design.go`; it now asserts the cross-package invariant
(`tuikit.IdentityManagerGlyphByState` has a glyph for every fixture row's
state), which is exactly what keeps the two halves coherent.
`internal/tuikit/store_test.go`'s two `len(IdentityManagerRows)` /
`len(HealthFindings)` comparisons now reference the stub's fixture slices —
same assertion, same counts.

### 4. Commit granularity

03-01 and 03-02 could not be committed separately: pre-commit runs
`make fmt` + `make lint` over the whole module and stashes unstaged changes
first, which reconstructs the half-finished rename and fails to compile. Per
CLAUDE.md ("let the buildable boundary, not file count, set the commit
granularity") and LEARNINGS L11, both plans landed as one reconciled commit,
`3dd4f47`.

## Not done (out of this executor's scope)

**Task 3 was NOT executed.** `internal/dummytui/nobackend_test.go` was not
restored and the `Makefile`'s `gate-no-backend-files` allowlist was not
widened with `internal/tuikit/`. This executor was scoped to getting the
interrupted refactor to a green module, not to starting new plan tasks.

The gate's underlying property is nevertheless **proven green** by the
import-graph check (see below): `go list -deps ./internal/tuikit` contains
exactly one first-party package — `internal/tuikit` itself.

`make gate-no-backend-files` currently FAILS on this branch, and would fail
even with Task 3's widening. It compares the branch's changed files against a
**Phase 2 design-only** allowlist; a Phase 3 branch legitimately changes
`internal/identity`, `internal/keygen` and `internal/tester` (plan 03-01's own
deliverables), which that allowlist can never accept. The file-level gate has
outlived its phase; the import-graph assertion is the part that still carries
meaning.

## Gate results (real output)

```
$ go build ./...
(no output — exit 0)

$ TERM=dumb SSH_AUTH_SOCK= go test -race ./...
ok  github.com/castocolina/gitid/cmd/gitid              4.418s
ok  github.com/castocolina/gitid/cmd/gitid-dummy        3.176s
ok  github.com/castocolina/gitid/internal/adopter       3.637s
ok  github.com/castocolina/gitid/internal/clipboard     4.020s
ok  github.com/castocolina/gitid/internal/deps          2.297s
ok  github.com/castocolina/gitid/internal/doctor        1.491s
ok  github.com/castocolina/gitid/internal/doctor/checks 1.915s
ok  github.com/castocolina/gitid/internal/dummytui      1.418s
ok  github.com/castocolina/gitid/internal/filewriter    5.299s
ok  github.com/castocolina/gitid/internal/gitconfig     6.870s
ok  github.com/castocolina/gitid/internal/identity      5.886s
ok  github.com/castocolina/gitid/internal/keygen       18.074s
ok  github.com/castocolina/gitid/internal/platform      5.998s
ok  github.com/castocolina/gitid/internal/repoclone     5.286s
?   github.com/castocolina/gitid/internal/screenshot    [no test files]
ok  github.com/castocolina/gitid/internal/sshconfig     8.056s
ok  github.com/castocolina/gitid/internal/tester        5.181s
ok  github.com/castocolina/gitid/internal/tuikit        8.158s
ok  github.com/castocolina/gitid/internal/upload        4.558s
ok  github.com/castocolina/gitid/internal/uploader      4.759s
ok  github.com/castocolina/gitid/tui                    4.583s

$ make lint
/Users/ramon/go/bin/golangci-lint run ./...
0 issues.

$ make test-e2e
go build -o bin/gitid ./cmd/gitid
go test -tags e2e -race -timeout 180s ./e2e/...
ok  github.com/castocolina/gitid/e2e  49.920s

$ go list -deps ./internal/tuikit | grep -E 'castocolina/gitid/internal/(identity|tester|sshconfig|keygen|filewriter|doctor|adopter|platform|clipboard|uploader)'
(no matches — exit 1)

$ go list -deps ./internal/tuikit | grep castocolina/gitid
github.com/castocolina/gitid/internal/tuikit

$ make gate-no-backend-files
gate-no-backend-files: FAILED -- file(s) outside the Phase 2 design-only allowlist changed since main:
  internal/identity/*, internal/keygen/*, internal/tester/*, internal/tuikit/*
```

## Self-Check: PASSED

- `internal/tuikit/backend.go` — FOUND (`type Backend interface`)
- `internal/tuikit/views.go` — FOUND (`type ReusableKeyView struct`, `type TestOutcome int`, `type TestResultView struct`; exactly three outcome constants)
- `internal/tuikit/design.go` — FOUND
- `internal/tuikit/app.go` — FOUND (`func NewApp(b Backend) App`, nil-guard panic)
- `internal/tuikit/backend_stub_test.go` — FOUND
- `internal/dummytui/fixturebackend.go` — FOUND (`func NewFixtureBackend`, `var _ tuikit.Backend = FixtureBackend{}`)
- `cmd/gitid-dummy/main.go` — FOUND (`tuikit.NewApp(dummytui.NewFixtureBackend())`)
- Commit `3dd4f47` — FOUND
