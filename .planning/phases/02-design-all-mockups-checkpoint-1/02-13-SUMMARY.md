---
phase: 02-design-all-mockups-checkpoint-1
plan: 13
subsystem: ui
tags: [bubbletea-v2, lipgloss-v2, bubbles-v2, dummy-tui, live-demo, reducer, pty-e2e, dlv-05, dlv-02]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    provides: internal/dummytui/data.go (the preserved recipe-faithful Go fixture mirror of recipeFixtures.ts), the interactive web demo at .planning/design/mockup-src/src/demo/ (the approved-direction reference), 02-REDESIGN-SPEC.md (the frame spec), and the e2e PTY harness patterns (02-11 and earlier)
provides:
  - cmd/gitid-dummy — the LIVE interactive Bubble Tea v2 demo binary (alt-screen + mouse cell motion via tea.View fields), the TUI half of the 02-12 presentation and the seed frame for Phases 3-9
  - internal/dummytui/store.go — DemoState + 12 typed actions + pure Reduce() mirroring the web store.ts transition-for-transition, seeded exclusively from data.go
  - internal/dummytui/fixplans.go — PlanFor per finding id (the contradiction fix reuses FixerFixPreviewLines verbatim, destructive with typed Host-name confirm)
  - internal/dummytui/frame.go — pure §1 chrome renderers (numbered reverse-video tabs, LIVE health chip, ›-joined breadcrumb, contextual-only footer + reserved keys, PreviewLabel/PreviewBlock dimmer than fields)
  - internal/dummytui/ceremony.go — the shared 2-state mutation ceremony (§6) with backup PROMISE, typed-confirm destructive gating (affirmative never default-focused), Wrote →/Backed up → receipt
  - internal/dummytui/app.go — root tea.Model with DemoApp.tsx key-routing precedence (overlays > screen handler stack > globals), help overlay with the full 8-state legend, Ctrl+P palette, real q quit prompt
  - internal/dummytui/identities.go — live master-detail + 4-pane-state create wizard + edit-SSH (same form, locked identity fields) + clone + delete (scope chooser, typed destructive confirm) + per-finding fix with live healing
  - internal/dummytui/globalssh.go — [Options]+[Storage & preview] sub-tabs, apply ceremony with declined-line semantics, STORE-01 dual-layout previews + reversible migration
  - internal/dummytui/globalgit.go — 11-row GGIT-01 baseline, main-vs-master highlight, awareness-only user.email row, sentinel-preserving baseline ceremony
  - internal/dummytui/doctor.go — Doctor absorbs the Fixer (FIX-02): auto-scan, grouped findings under the locked severity contract, f fix-this / F fix-all with the k / n counter, live chip decrement + identity healing
  - e2e/dummy_demo_e2e_test.go + restored BuildDummyBinary — raw-keystroke PTY walk of the REAL binary at 100x30 with the DLV-05 zero-writes sandbox assertion
affects: [02-12 (the human checkpoint presents this live demo), Phase 3+ (the real product TUI grows out of this frame)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Elm-pure child screens: every tab implements screenModel (handleKey/handleMsg/view/activate) over (model, DemoState); reducer actions flow back to the App — the single Reduce caller — so unit tests drive the whole app without a terminal"
    - "Bubble Tea v2 idioms: View() tea.View with AltScreen/MouseMode fields (WithAltScreen/WithMouse* do not exist in v2); tea.KeyMsg matched via msg.String()"
    - "Wrap-safe copy testing: regionFlat/paneFlat extract a column region of the rendered frame and collapse whitespace so long spec copy survives pane word-wrapping in assertions"
    - "PTY e2e at non-default geometry: startPTYAt parameterizes the 3-goroutine single-owner vt-emulator loop (the emulator dimensions are fixed at construction)"

key-files:
  created:
    - cmd/gitid-dummy/main.go
    - cmd/gitid-dummy/main_test.go
    - internal/dummytui/store.go
    - internal/dummytui/store_test.go
    - internal/dummytui/fixplans.go
    - internal/dummytui/fixplans_test.go
    - internal/dummytui/frame.go
    - internal/dummytui/frame_test.go
    - internal/dummytui/ceremony.go
    - internal/dummytui/ceremony_test.go
    - internal/dummytui/app.go
    - internal/dummytui/app_test.go
    - internal/dummytui/identities.go
    - internal/dummytui/identities_test.go
    - internal/dummytui/globalssh.go
    - internal/dummytui/globalssh_test.go
    - internal/dummytui/globalgit.go
    - internal/dummytui/globalgit_test.go
    - internal/dummytui/doctor.go
    - internal/dummytui/doctor_test.go
    - e2e/dummy_demo_e2e_test.go
  modified:
    - internal/dummytui/data.go (pure-data extensions the web demo seeds from — state tone map, severity glyphs, match-strategy previews, ManagedBlockSentinels helper, full global-git managed block, baseline strip text)
    - internal/dummytui/doc.go (package now HOLDS the live demo; recipes/North-Star provenance kept)
    - e2e/harness_test.go (BuildDummyBinary restored with its own sync.Once cache)

key-decisions:
  - "Bubble Tea v2 retained (charm.land v2 stack per CLAUDE.md); github.com/grindlemire/go-tui evaluated at the user's suggestion and REJECTED as too immature (recorded in the plan objective)"
  - "Edit-SSH and Configure-Git ceremonies open as the pane's NEXT state instead of inline below the form (the web renders both simultaneously; 30 terminal rows cannot) — same §6 two-state semantics, same copy, Esc returns to the form"
  - "Wizard Skip is Ctrl+S, not plain s: s must type into the user.name/user.email fields; the full 'Skip — SSH only (identity stays incomplete)' copy still renders in-pane"
  - "Sidebar is 36% at the 100-col minimum (spec ~38/62) so the detail pane keeps 63 usable columns; step-3 dual previews stack vertically for the same reason"
  - "Contract-bearing helpers (locked fields, prefix WYSIWYG/duplicate error, auto-join state) always render; purely descriptive helpers render for the focused field only, keeping every wizard pane inside the 30-row frame"
  - "cmd/gitid-dummy carries a smoke main_test.go: NOT required by the plan for behavior (all behavior is tested in internal/dummytui + the PTY e2e) but required in practice — a buildable no-test package makes `go test -coverprofile` reach for the covdata tool, which the auto-downloaded Go 1.26 toolchain does not ship"

patterns-established:
  - "Live-demo state mutations only via typed Actions through pure Reduce(); helpers (FindingCounts/HealthRollup/FindingsFor/NewBackupPath) mirror the web store helpers byte-for-byte where copy is contractual"
  - "Every rendered string comes from internal/dummytui/data.go or is mirrored verbatim from the named demo sources (T-02-13-DRIFT mitigation), pinned by unit tests (flag order, no -i in stage 2, severity words, advisory notes, locked-field helpers)"

requirements-completed: [DLV-05, DLV-02]

# Metrics
duration: ~65min
completed: 2026-07-04
---

# Phase 02 Plan 13: Live Interactive gitid-dummy TUI Demo Summary

**Rebuilt `cmd/gitid-dummy` as a LIVE, fully interactive Bubble Tea v2 demo — 4 header tabs with a live health chip, live master-detail Identities with the 4-pane-state create wizard, Global SSH (Options + STORE-01 Storage), Global Git with the main-vs-master highlight, and a Doctor that absorbs the Fixer — all driving a pure reducer over dummy in-memory state seeded from the recipe-faithful data.go, proven backend-free (import allowlist) and write-free (PTY e2e zero-writes sandbox walk at 100x30).**

## What was built

- **Task 1 (`6ea89a5`)** — Foundation: `store.go` (DemoState + 12 actions + pure `Reduce()` mirroring `store.ts`), `fixplans.go`, `frame.go` (§1 chrome: numbered reverse-video tabs, live `N ids · ! w ✗ e` chip, ›-breadcrumb, contextual-only footer + reserved keys, dim PreviewLabel/PreviewBlock), `ceremony.go` (2-state §6 ceremony with typed destructive confirm, Cancel default-focused), `app.go` (key precedence: overlays > screen stack > globals; help with the full 8-state legend; Ctrl+P palette; real q quit), `cmd/gitid-dummy/main.go`.
- **Task 2 (`5fb05f8`)** — Identities: sidebar with tone glyph + S/G pips + N⚑ flags + legend; arrow selection re-renders the detail immediately; detail shows SSH first, never fabricates Git values, carries the read-only baseline strip and the findings sub-panel; create wizard (auto-join alias, provider defaults, duplicate-prefix block, digits-only port, -sk algorithms disabled with the libfido2 rationale, live Host-block preview, two-stage test with the pinned flag order and the no `-i` stage-2 rationale, demo failure toggle with lock semantics, full Git form with dual previews, review ceremony); edit-SSH via the SAME form with locked identity fields; clone; delete with safer-default scope chooser + typed-name confirm; per-finding fix with live healing.
- **Task 3 (`3a63249`)** — Global SSH (sub-tabs, apply ceremony with `+`/context/declined lines, storage previews + reversible migration), Global Git (11 rows, highlight chip, awareness-only user.email, baseline ceremony → `GlobalGitResultMessage`), Doctor (auto-scan, `SSH · <identity|global>` then `Git · …` grouping, locked `~ info / ! warning / ✗ error / ✗ critical` glyph+word contract, `f`/`F` with the `k / n fixed` banner, live chip decrement + legacy healing), plus `TestDummyDemo_LiveWalk` and the restored `BuildDummyBinary`.

## Verification (all gates observed green)

| Gate | Command | Result |
| --- | --- | --- |
| Import allowlist | `go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/...` filtered to first-party | only `internal/dummytui` + `cmd/gitid-dummy` |
| Unit tests | `go test -race ./internal/dummytui/...` | ok (coverage 90.3%) |
| Module tests | `make test` | ok, all packages |
| Lint | `make lint` | 0 issues |
| E2E | `make test-e2e` (includes `TestDummyDemo_LiveWalk`, 100x30 PTY, raw keystrokes) | ok in 39.1s (walk itself 4.9s); zero files created under sandbox HOME |
| Design-only branch | `make gate-no-backend-files` | OK |

Run the demo: `go build -o bin/gitid-dummy ./cmd/gitid-dummy && bin/gitid-dummy` in a terminal of at least 100x30.

## Deviations from Plan

### Auto-fixed issues

**1. [Rule 3 - Blocking] `make test` broke on the new no-test main package**
- **Found during:** Task 3 gates
- **Issue:** `go test -race -coverprofile` on `cmd/gitid-dummy` (buildable, no test files) fails with `go: no such tool "covdata"` — the auto-downloaded Go 1.26 toolchain ships no covdata tool, and coverage synthesis for no-test packages needs it
- **Fix:** added `cmd/gitid-dummy/main_test.go`, a one-assertion smoke test (NewApp().View() renders the frame); the plan said a main_test.go was "not required", not forbidden — behavior coverage still lives in internal/dummytui + the PTY e2e
- **Files modified:** cmd/gitid-dummy/main_test.go
- **Commit:** 3a63249

### Terminal-geometry adaptations (documented in-file, same semantics/copy as the web demo)

1. **Edit-SSH / Configure-Git ceremonies are the pane's next state** (Enter opens, Esc returns to the form) instead of rendering inline below the form — 30 rows cannot hold both; §6 two-state semantics and all copy preserved.
2. **Wizard step-3 dual previews stack vertically** (fragment above includeIf) — the 63-col detail pane cannot hold two legible side-by-side blocks.
3. **Wizard Skip is `Ctrl+S`** — plain `s` must type into the author fields; the full skip copy still renders.
4. **Sidebar 36% at the 100-col minimum** (spec ~38/62) and the sidebar legend uses single-space separators, so the detail pane keeps 63 columns.
5. **Descriptive field helpers render focused-only**; contract helpers (locked fields, prefix error/WYSIWYG, auto-join state) always render — keeps every wizard pane inside the 30-row frame.
6. **Per-identity `Fix…` binds `f` to the first fixable finding** of the selected identity (the web has one button per finding row; identities carry at most one fixable finding in the seed, and the Doctor covers the general case).

None of these change reducer behavior, action semantics, or the contractual copy the tests pin.

### Code-review batch 1 fixes (post-review, uncontested findings)

1. **Plan-file backup dispatch** — the identity-pane fix ceremony dispatched `FixFinding` with a hardcoded `~/.ssh/config` backup; it now backs up the finding's own `PlanFor(f).File` (e.g. `~/.gitconfig.d/legacy`), matching doctor.go and Identities.tsx. Pinned by `TestFixFromIdentityPaneBacksUpThePlanFile`.
2. **Real mouse routing** (MouseMode was enabled with zero handling) — left clicks on `tea.MouseClickMsg` now route: header tab labels 1–4 switch tabs, the health chip opens the Doctor, Identities sidebar rows select, Doctor finding rows select, Global SSH/Git option rows select, and the Global SSH sub-tab labels switch sub-tabs. Hit-testing derives from the same strings/constants the renderers use (headerTabText, frameBodyTop/frameChromeBelow, masterListWidth, sidebar/option row-line constants). **Consciously bounded scope:** clicks are select/navigate only — in-pane buttons (e.g. "Fix this…", "Apply baseline"), ceremony controls, form fields, footer hints, and overlays stay keyboard-driven, and wheel/motion/drag/right-click are ignored; the web demo's every-control clickability is not fully mirrored in the terminal.
3. **Init() no longer discards the activated screen** — the initial tab's activation runs in NewApp (model retained), Init just returns the stored command.
4. **Elm purity for `chosen` maps** — Global SSH/Git toggles are copy-on-write (`withToggled`), so value-copied models never share map storage. Pinned by the two `…SpaceToggleIsCopyOnWrite` tests.
5. **E2E Doctor assertion strengthened** — the tab-4 check now asserts the Doctor-body status line ("Health only diagnoses") instead of the always-present "Doctor" header label.

## Known Stubs

None — every surface is live and mutates the shared reducer state; no placeholder data paths remain. (The demo is itself dummy/in-memory BY DESIGN — that is DLV-05's requirement, not a stub.)

## Threat Flags

None — no new network endpoints, auth paths, file access, or schema surface. The plan's threat register mitigations all hold: T-02-13-NB (allowlist gate green), T-02-13-WRITE (zero-writes PTY assertion green), T-02-13-DRIFT (copy pinned by unit tests), T-02-13-BEGATE (gate-no-backend-files green), T-02-13-SC (no new dependencies).

## Self-Check: PASSED

- cmd/gitid-dummy/main.go, internal/dummytui/{store,fixplans,frame,ceremony,app,identities,globalssh,globalgit,doctor}.go + tests, e2e/dummy_demo_e2e_test.go — all present on disk
- Commits 6ea89a5, 5fb05f8, 3a63249 present on the branch
- All six verification gates re-run and observed green in this session
