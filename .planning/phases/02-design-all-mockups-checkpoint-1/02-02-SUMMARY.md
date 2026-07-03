---
phase: 02-design-all-mockups-checkpoint-1
plan: 02
subsystem: ui
tags: [bubbletea-v2, lipgloss-v2, tui-dummy, go, tdd, no-backend-allowlist]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 01
    provides: "the shared four-region shell + <surface>/<screen> breadcrumb parity source (the MUI mockup) this plan's TUI dummy must match"
provides:
  - "internal/dummytui: Register/RegisterOrReplace surface registry, the pure route() nav reducer over navState.modalStack (number-key views 1-5 + target-owned LaunchFrom/LaunchKey modal launch + Esc pop), a registration-time LaunchKey collision guard, and RenderScreen(surface,screen) — the full-shell capture entry point later screenshot plans call"
  - "cmd/gitid-dummy: a physically separate, Cobra-free binary proven by an import-graph ALLOWLIST to contain zero first-party backend packages"
  - "The five FINAL placeholder surfaces (identity-manager/global-ssh/global-git/health/fixer) on their FINAL surface IDs, replaceable via RegisterOrReplace without editing model.go/data.go"
  - "The LaunchFrom/LaunchKey modal-launch contract + registration-time collision guard the create-flow (02-04) and git-screen (02-05) fan-out plans plug into"
affects: [02-04, 02-05, 02-06, 02-07, 02-08, 02-09, 02-10, 02-11, 02-12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Package-level surface registry (map[string]SurfaceDef) populated via Register/RegisterOrReplace from each surface file's init(), with a registration-time collision guard instead of a runtime-only check"
    - "Pure navState reducer (route()) separating nav state transitions from tea.Model plumbing — testable without spinning up a tea.Program"
    - "Target-owned modal-launch binding (SurfaceDef.LaunchFrom/LaunchKey) so a keyless modal surface wires its own launch point without editing its source surface's file"
    - "go list -deps ALLOWLIST (not denylist) as a CI-enforceable no-backend-import proof, mirroring Phase 1's freeze/go-rod exclusion check"

key-files:
  created:
    - internal/dummytui/doc.go
    - internal/dummytui/registry.go
    - internal/dummytui/registry_test.go
    - internal/dummytui/data.go
    - internal/dummytui/overlay.go
    - internal/dummytui/shell.go
    - internal/dummytui/model.go
    - internal/dummytui/model_test.go
    - internal/dummytui/nobackend_test.go
    - cmd/gitid-dummy/main.go
  modified: []

key-decisions:
  - "Register/RegisterOrReplace signal collisions via panic (not a returned error): surfaces call them from init(), so a must-never-conflict registration contract that fails loudly at program-load time (test-detectable via recover()) is more idiomatic than threading an error return through every init()"
  - "Task 1's RenderScreen used a self-contained minimal inline shell composition (header+body+status+keybar joined directly in registry.go, no shell.go dependency) so internal/dummytui compiled and Task 1's tests passed standalone before shell.go existed (review C2); Task 2 rewired RenderScreen's body to delegate to shell.go's renderShell, replacing the inline composition entirely — both tasks touch registry.go by design"
  - "model.go's modal-overlay centers the modal against the ACTUAL rendered content height (lipgloss.Height of the composited base layout), not the window's full m.height: the dummy's four-region shell renders exactly as many rows as its regions need (no padding to fill the terminal), so centering against the terminal's total row count would push a short modal below the visible content and placeOverlay's per-row bounds clamp would silently skip every overlay row — this was caught by TestModel_ModalLaunchThroughModel_BreadcrumbAndEscReverts failing on the first GREEN run and fixed before commit"
  - "renderShellKeybar computes context-sensitive hints (intra-surface ScreenDef.Keys, sorted; keyless surfaces launchable via LaunchFrom==this surface; reserved keys) rather than a single hardcoded keybar string, satisfying UX-DIRECTION section 2's 'keybar shows only keys valid in the current context' while staying deterministic (sorted map keys) for RenderScreen's byte-identical-output contract"
  - "DLV-05/DLV-02 are NOT marked complete in REQUIREMENTS.md despite this plan's frontmatter listing them, matching 02-01's precedent: both are phase-spanning requirements (DLV-05's full per-surface HTML->dummy->approval->backend order; DLV-02's every-UI-task agent+skill engagement) and this plan ships only the front-half skeleton (2/12 plans). Deferred to whichever later Phase 2 plan closes out full coverage."

requirements-completed: []  # DLV-05/DLV-02 phase-spanning — see key-decisions; not marked complete by this skeleton plan alone

# Metrics
duration: ~15min
completed: 2026-07-03
---

# Phase 2 Plan 02: TUI Dummy Skeleton (internal/dummytui + cmd/gitid-dummy) Summary

**A physically separate, import-graph-ALLOWLISTED `cmd/gitid-dummy` binary running a Bubble Tea v2 nav-only TUI over `internal/dummytui`'s Register/RegisterOrReplace surface registry, target-owned LaunchFrom/LaunchKey modal-launch mechanism, and a deterministic full-shell `RenderScreen` — zero backend logic, TDD-proven.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-07-03T04:14:49-04:00 (approx, first file write after 02-01's completion commit)
- **Completed:** 2026-07-03T04:29:14-04:00 (Task 2 commit)
- **Tasks:** 2 completed
- **Files modified:** 10 created (0 modified)

## Accomplishments

- Built `internal/dummytui`'s surface registry (`registry.go`): `ScreenDef`/`SurfaceDef` types, `Register` (rejects a duplicate non-empty `ActivationKey`, exempts empty/keyless `ActivationKey`), `RegisterOrReplace` (single-owner replacement for fan-out surfaces), `Surfaces()`, and `RenderScreen(surface,screen)` — the deterministic full-shell capture entry point.
- Implemented the pure `route()` nav reducer over `navState.modalStack`: number-key view switching (1-5), intra-surface `ScreenDef.Keys` transitions, target-owned `LaunchFrom`/`LaunchKey` modal launch (push) + Esc (pop), with an explicit precedence (intra-surface keys -> launch key -> number key) and a registration-time collision guard that panics on any key clash across those three claim types, a number key, or a reserved key — in both directions (a new keyless surface colliding with an existing source surface's `ScreenDef.Keys`, and a `RegisterOrReplace` on a source surface colliding with an existing keyless `LaunchKey`).
- Seeded `data.go` with the five FINAL placeholder surfaces (`identity-manager`/`global-ssh`/`global-git`/`health`/`fixer` on activation keys `1`-`5`) so `LaunchFrom` bindings from later fan-out plans resolve both now and after `RegisterOrReplace`.
- Reimplemented `tui/overlay.go`'s `placeOverlay`/`overlayLine`/`modalOrigin`/`boundModalToViewport` verbatim, backend-free, in `overlay.go` (lipgloss v2.0.3 has no `PlaceOverlay`).
- Built `shell.go`'s four-region composition (header with the `<surface>/<screen>` breadcrumb, body, status, context-sensitive keybar) and rewired `RenderScreen` to delegate to it, replacing Task 1's self-contained minimal inline shell (review C2).
- Built `model.go`'s `Model` (`tea.Model`): seeded on `identity-manager` with an empty `modalStack`, `Update` dispatches `tea.KeyMsg` through `route()`, `View()` sets `AltScreen = true` and composites an open modal's active-screen body over the dimmed parent via `placeOverlay`, with the header breadcrumb reflecting the topmost modal frame.
- Built `cmd/gitid-dummy/main.go`: a thin, Cobra-free, isTTY-gated entry point calling `dummytui.NewModel()` directly.
- Proved DLV-05's no-backend ALLOWLIST (`nobackend_test.go`): `go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/...` contains no `github.com/castocolina/gitid/*` package other than exactly `internal/dummytui` and `cmd/gitid-dummy` — verified empty diff, and confirmed `go build ./cmd/gitid` (the real product) is unaffected.

## Task Commits

Each task was committed atomically:

1. **Task 1: Surface registry + nav state machine + RenderScreen (TDD)** - `051021e` (feat)
2. **Task 2: Bubble Tea model + shell + overlay + gitid-dummy binary + no-backend gate (TDD)** - `ed282f1` (feat)

**Plan metadata:** pending (this commit, created after this SUMMARY)

## Files Created/Modified

- `internal/dummytui/doc.go` - DLV-05 no-backend ALLOWLIST rule, modal-launch contract, and the UX-DIRECTION section 2 key-allocation table mirrored as the single authority
- `internal/dummytui/registry.go` - `ScreenDef`/`SurfaceDef`, `Register`/`RegisterOrReplace`, `Surfaces()`, `RenderScreen`, `navState`/`modalFrame`, the `route()` pure reducer, and the LaunchKey collision guard
- `internal/dummytui/registry_test.go` - table-driven tests covering every Task 1 `<behavior>` bullet
- `internal/dummytui/data.go` - the five FINAL placeholder surfaces on activation keys 1-5
- `internal/dummytui/overlay.go` - backend-free `placeOverlay`/`overlayLine`/`modalOrigin`/`boundModalToViewport`, reimplemented (not imported) from `tui/overlay.go`
- `internal/dummytui/shell.go` - four-region shell composition (header/body/status/context-sensitive keybar)
- `internal/dummytui/model.go` - the `tea.Model`: `NewModel`, `Update`, `View`, modal-stack dim-then-composite dispatch
- `internal/dummytui/model_test.go` - model-level tests: seeding, breadcrumb, Update routing, modal launch through the model, overlay clamp, full-shell RenderScreen assertion
- `internal/dummytui/nobackend_test.go` - the DLV-05 ALLOWLIST proof (`go list -deps`, arg-slice `exec.Command`, `#nosec G204`)
- `cmd/gitid-dummy/main.go` - the nav-only dummy binary entry point

## Decisions Made

- Register/RegisterOrReplace panic (rather than return an error) on any collision — surfaces call them from `init()`, so a fail-loudly-at-load contract fits better than threading error returns through every surface file's `init()`; all collision tests assert via `recover()`.
- Task 1 shipped a self-contained minimal `RenderScreen` (no `shell.go` dependency) so the package compiled and Task 1's tests passed standalone before `shell.go` existed (review C2's independently-buildable-task requirement); Task 2 fully replaced that composition with a delegate call to `shell.go`'s `renderShell`.
- The modal-overlay centers against the actual rendered content height, not the terminal's full window height — the dummy's shell renders exactly as many rows as its four regions need (no fill-to-height padding), so the first GREEN run of `TestModel_ModalLaunchThroughModel_BreadcrumbAndEscReverts` caught the mismatch (the modal was being placed at a row past the end of the composited string, so `placeOverlay`'s bounds clamp silently dropped it) — fixed before commit.
- `renderShellKeybar` derives context-sensitive hints (sorted intra-surface `ScreenDef.Keys`, launchable keyless surfaces, reserved keys) instead of a static string, honoring UX-DIRECTION section 2's "keybar shows only valid keys" while remaining deterministic (sorted keys, sorted `Surfaces()`) for `RenderScreen`'s byte-identical contract.
- **DLV-05 and DLV-02 are NOT marked complete in `REQUIREMENTS.md`** despite this plan's frontmatter listing them, matching 02-01's established precedent for phase-spanning requirements: DLV-05's full per-surface order (HTML -> dummy -> approval -> backend) and DLV-02's every-UI-task agent+skill engagement both span all 7 surfaces / 12 plans of this phase. This plan ships only the dummy skeleton (2/12 plans) — marking either requirement complete here would falsely declare phase-wide coverage. Deferred to the plan that closes out full Phase 2 coverage (likely 02-11/02-12, matching the precedent already recorded for 02-01/DLV-01/DLV-02).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Modal-overlay centering used the wrong height reference**
- **Found during:** Task 2, first GREEN run of `TestModel_ModalLaunchThroughModel_BreadcrumbAndEscReverts`
- **Issue:** `renderContent`'s modal compositing initially centered the modal against `m.height` (the terminal's full window height, defaulting to 30), but the dummy's four-region shell only ever renders as many lines as its regions produce (no fill-to-height padding). This meant `modalOrigin`'s computed row fell past the end of the composited background string, so `placeOverlay`'s per-row bounds check (`bgRow >= len(bgLines)`) silently skipped every modal row — the modal never appeared in `View()`'s output even though the breadcrumb correctly reflected the modal frame.
- **Fix:** Center the modal against `lipgloss.Height(dimmed)` (the actual rendered base-layout height) instead of `m.height`.
- **Files modified:** `internal/dummytui/model.go`
- **Commit:** `ed282f1` (folded into Task 2's single commit — caught and fixed before commit, no separate fix commit)

No other deviations — the rest of both tasks executed as written.

## Issues Encountered

- **Code-review-skill tool unavailable in this executor's environment**, same limitation recorded in 02-01-SUMMARY.md. The plan's `<success_criteria>` requires "a fresh-context code review via the `superpowers:requesting-code-review` skill" before the plan is marked complete. This executor's toolset in this session was limited to `Read`/`Write`/`Edit`/`Bash` — no `Task`/subagent-dispatch tool was available to spawn a fresh-context reviewer. In its place, this executor re-verified every `<acceptance_criteria>` bullet across both tasks via the exact automated commands the plan specifies (`go test -race`, the `go list -deps` allowlist diff, every `grep -q` structural check, `go build ./cmd/gitid`), plus a full-repo `go build ./...` and `go test -race ./...` pass (all green, no regressions) and `make lint`/`make fmt` (0 issues). **This does not substitute for the plan's required fresh-context review** — flagging explicitly so the phase-level `/gsd-code-review` and the external cross-vendor (Codex) review the execution loop runs are not skipped for this plan's content, and so a follow-up fresh-context review can be run against this plan specifically if the orchestrator has the capability this session lacked.

## User Setup Required

None — no external service configuration required. `go build ./cmd/gitid-dummy` produces a runnable local binary with no setup.

## Next Phase Readiness

- The registry contracts (`Register`/`RegisterOrReplace`, `LaunchFrom`/`LaunchKey`) are proven fan-out-safe: 02-04 (create-flow) and 02-05 (git-screen) register keyless modal surfaces against `identity-manager`'s `n`/`g` LaunchKeys without editing `model.go` or `data.go`; 02-06 through 02-10 replace the five placeholders via `RegisterOrReplace` without editing each other's files.
- `doc.go`'s key-allocation table is the single authority the three fan-out plans (02-04/02-05/02-06) allocate keys against; the registration-time collision guard will fail loudly (a test) if any of them picks a colliding key, rather than surfacing as a confusing 02-11 PTY e2e failure.
- `RenderScreen(surface,screen)` is the concrete entry point 02-11's screenshot-capture plan and the dummy-nav PTY e2e test will call/drive.
- Outstanding, not blocking this plan: a fresh-context `superpowers:requesting-code-review` pass on this plan's diff specifically (see Issues Encountered) — recommend the orchestrator run one before or alongside the phase-level review gate.

---
*Phase: 02-design-all-mockups-checkpoint-1*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 10 created files verified present on disk (`test -f` per file, 10/10 FOUND). Both task commit hashes (`051021e`, `ed282f1`) verified present in `git log --oneline --all`. `go build ./...`, `go test -race ./...`, and `make lint` all pass with zero issues at the time this summary was written.
