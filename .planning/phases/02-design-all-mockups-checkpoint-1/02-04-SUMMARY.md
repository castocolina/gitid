---
phase: 02-design-all-mockups-checkpoint-1
plan: 04
subsystem: ui
tags: [react, mui, bubbletea-v2, lipgloss-v2, go, design-mockup, tui-dummy, screenshot, e2e, pilot-surface]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 01
    provides: "the shared four-region MUI shell + recipeFixtures.ts + route auto-discovery this plan's 12 create-flow routes build on"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 02
    provides: "internal/dummytui's Register/RegisterOrReplace registry, the LaunchFrom/LaunchKey modal-launch contract, and RenderScreen — the launch mechanism create-flow's surface plugs into"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 03
    provides: "the hardened manifest.json schema/loader, design_capture_test.go's manifest-driven capture, and dummy_nav_e2e_test.go's manifest-driven PTY walker — create-flow is the first surface to actually exercise this infrastructure end-to-end"
provides:
  - "The create-identity flow (pilot surface) as 12 named states in BOTH media: /mui v7 routes under src/routes/create-flow/*.route.tsx and internal/dummytui/surface_createflow.go, both recipe-accurate (Port 443, IdentitiesOnly yes, ed25519 default, ssh -G IdentityFile proof)"
  - ".planning/design/create-flow/{FIELDS.md, manifest.json, parity.json, CRITIQUE.md, html/*.png (12), tui/*.png (12)} — the complete per-surface pipeline artifact set the six-surface fan-out (02-05..02-10) copies"
  - "recipeFixtures.ts extended with the algorithm catalog, SSH-form defaults, two-stage test commands/output, and mutation-ceremony copy every create-flow screen renders"
  - "A fix to internal/dummytui/model.go's modal-overlay compositing (pads the dimmed background to the real terminal height before compositing) — unblocks any FUTURE modal surface taller than its parent's placeholder body, not just create-flow"
affects: [02-05, 02-06, 02-07, 02-08, 02-09, 02-10, 02-11, 02-12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-surface pipeline proven end-to-end on the pilot: FIELDS.md -> manifest.json (hardened schema) -> parity.json seed -> /mui mockup (12 routes) -> dummytui surface (12 ScreenDefs, keyless modal + LaunchFrom/LaunchKey) -> capture (html+tui PNGs) -> agent-ui-ux-designer critique -> parity.json 0-unresolved"
    - "Keyless modal surface registered via a connected ScreenDef.Keys tree rooted at the entry screen, with manifest.json's keysFromHome walking the SAME transitions absolute-from-startup (re-home '1' + LaunchKey 'n' + intra-modal keys)"
    - "Recipe-critical copy kept as byte-visible literals in THREE places kept in sync by convention, not by import: recipeFixtures.ts (TS), the route files (which import from it), and Go string constants in surface_createflow.go (which cannot import TS) — parity is a maintained contract, not a shared runtime dependency"
    - "Screen-specific signature markers (manifest.json's `signature` field) embedded in every dummytui Render() output, distinct from the '<surface>/<screen>' breadcrumb, so a capture/e2e assertion proves the RIGHT screen state was reached, not just the right route"

key-files:
  created:
    - .planning/design/create-flow/FIELDS.md
    - .planning/design/create-flow/manifest.json
    - .planning/design/create-flow/parity.json
    - .planning/design/create-flow/CRITIQUE.md
    - .planning/design/create-flow/html/*.png (12 files)
    - .planning/design/create-flow/tui/*.png (12 files)
    - .planning/design/dummy-nav-frames/*.txt (13 files, e2e evidence)
    - .planning/design/mockup-src/src/routes/create-flow/algo-catalog.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/ssh-form-empty.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/ssh-form-filled.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/ssh-form-blank-prefix.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/reuse-key-vs-generate.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/macos-globals-block.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/test-stage1-direct.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/test-stage2-by-alias.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/test-fail.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/confirm-write.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/backup-notice.route.tsx
    - .planning/design/mockup-src/src/routes/create-flow/result-success.route.tsx
    - internal/dummytui/surface_createflow.go
    - internal/dummytui/surface_createflow_test.go
  modified:
    - .planning/design/mockup-src/src/data/recipeFixtures.ts
    - internal/dummytui/model.go

key-decisions:
  - "create-flow's LaunchKey is 'n' (from identity-manager), matching the single-authority key-allocation table in 02-UX-DIRECTION.md §2 / internal/dummytui/doc.go — not re-chosen independently"
  - "The 12 screens form a tree rooted at algo-catalog (the entry screen), not a strict linear chain: ssh-form-empty branches to ssh-form-blank-prefix (dead-end demo) and ssh-form-filled; ssh-form-filled branches to reuse-key-vs-generate and macos-globals-block (dead-end demos) plus the linear test->confirm->backup->result path; test-stage2-by-alias branches to test-fail (dead-end demo) plus the success path onward. Intra-surface keys (c/f/b/r/m/t/a/w/x/y/z) were chosen to avoid every reserved/number key and every already-claimed LaunchKey ('n'/'g') per registry.go's collision guard"
  - "recipeFixtures.ts was extended (not just read) with algorithmCatalog, SSH-form defaults, the two-stage test commands/output, and the mutation-ceremony copy — this content didn't exist yet anywhere in the fixture file, and Task 1's own action mandates copy comes from recipeFixtures.ts (see Deviations)"
  - "DLV-01/DLV-02/DLV-05 are NOT marked complete in REQUIREMENTS.md despite this plan's frontmatter listing them, matching the established 02-01/02-02/02-03 precedent for phase-spanning requirements: this plan ships the FIRST of seven surfaces (the pilot); DLV-01's 'every UI-bearing phase produces an HTML mockup' and DLV-05's full per-surface build order both require coverage across all seven surfaces, which is not complete until 02-10. Deferred to whichever later plan closes out full Phase 2 coverage (likely 02-11/02-12)."

requirements-completed: []  # DLV-01/DLV-02/DLV-05 phase-spanning — see key-decisions; this plan ships 1/7 surfaces, not full coverage

# Metrics
duration: ~23min
completed: 2026-07-03
---

# Phase 2 Plan 04: Create-Flow Pilot Surface (MUI Mockup + TUI Dummy + Capture + Parity) Summary

**The create-identity flow — the most complex of the seven surfaces — built as 12 recipe-accurate named states in both /mui v7 and the TUI dummy, captured as 12+12 PNGs, and driven through the launch mechanism on the real `cmd/gitid-dummy` binary with a 0-unresolved structured parity gate, proving the full per-surface pipeline before the six-surface fan-out.**

## Performance

- **Duration:** ~23 min
- **Started:** 2026-07-03T09:07:39Z
- **Completed:** 2026-07-03T09:31:07Z
- **Tasks:** 3 completed
- **Files modified:** 40 created/modified across the 3 task commits (16 + 2 + 22, see Task Commits)

## Accomplishments

- Authored `.planning/design/create-flow/FIELDS.md` (per-screen field/order/label table for all 12 named states from 02-UX-DIRECTION.md §4.1) and `manifest.json` (12 hardened-schema entries — unique screen/signature, `keysFromHome` absolute from startup through the launch key `n`).
- Built all 12 `/mui` v7 route files under `src/routes/create-flow/` — the algorithm catalog (KEY-01's top-5, ed25519 default), the SSH form in all three named states (empty/filled/blank-prefix, SSHUI-01/03), reuse-vs-generate (KEY-06), the macOS globals block (SSHUI-05), the two-stage connectivity test with `ssh -G` proof (TEST-01/02) plus its failure state, and the full four-beat mutation ceremony (confirm-write/backup-notice/result-success) — all pulling real copy from an extended `recipeFixtures.ts`.
- Built `internal/dummytui/surface_createflow.go`: a keyless modal surface (`LaunchFrom: "identity-manager"`, `LaunchKey: "n"`) with 12 `ScreenDef`s mirroring the mockup byte-for-byte on labels/copy/defaults, each embedding its manifest signature; the `ScreenDef.Keys` graph is a connected tree reachable from the `algo-catalog` entry screen, matching manifest.json's `keysFromHome` walk.
- Captured 12 HTML + 12 TUI PNGs (`make screenshot-html-mockups`/`make screenshot-tui-mockups`) and proved every screen reachable on the REAL `cmd/gitid-dummy` binary through the launch key with zero writes (`make dummy-nav-e2e`).
- Ran the structured HTML↔TUI parity review against all 12 screenshot pairs (applying `agent-ui-ux-designer`'s methodology directly — see Issues Encountered) and closed `parity.json`'s 8 rows (the seven §3 dimensions + `test-confirm-backup-boundary`) to `status: resolved`, 0 unresolved.
- Found and fixed a pre-existing bug in the shared `internal/dummytui/model.go` modal-overlay compositing that `make dummy-nav-e2e` exposed for the first time (see Deviations).

## Task Commits

Each task was committed atomically:

1. **Task 1: create-flow FIELDS.md + manifest.json + parity.json seed + MUI mockup (12 states)** - `468ef41` (feat)
2. **Task 2: create-flow TUI dummy surface (12 screens, keyless modal, launch binding)** - `6ec34c5` (feat)
3. **Task 3: Capture (both media) + parity critique -> 0 unresolved** - `c7a13f7` (feat)

**Plan metadata:** pending (this commit, created after this SUMMARY)

## Files Created/Modified

- `.planning/design/create-flow/FIELDS.md` - per-screen field/label/order table for all 12 states
- `.planning/design/create-flow/manifest.json` - 12 hardened-schema `{surface, screen, htmlRoute, keysFromHome, signature}` entries
- `.planning/design/create-flow/parity.json` - 8 §3-dimension rows, all `status: resolved`
- `.planning/design/create-flow/CRITIQUE.md` - aesthetic pass (0 findings) + structured parity findings log (1 non-blocking observation, out of §3 scope)
- `.planning/design/create-flow/html/*.png` (12) / `.planning/design/create-flow/tui/*.png` (12) - captured screenshots
- `.planning/design/dummy-nav-frames/*.txt` (13) - PTY e2e evidence frames (identity-manager entry + one per create-flow screen)
- `.planning/design/mockup-src/src/routes/create-flow/*.route.tsx` (12) - the /mui mockup screens
- `.planning/design/mockup-src/src/data/recipeFixtures.ts` - extended with `algorithmCatalog`, SSH-form defaults, two-stage test commands/output, mutation-ceremony copy
- `internal/dummytui/surface_createflow.go` (+ `_test.go`) - the TUI dummy create-flow surface, 12 screens, keyless modal with a LaunchFrom/LaunchKey binding
- `internal/dummytui/model.go` - **modified** (deviation): pads the dimmed modal background to the real terminal height before compositing

## Decisions Made

See `key-decisions` in the frontmatter for the full rationale on: the LaunchKey allocation (`n`, from the single-authority table), the 12-screen tree shape and intra-surface key choices, why `recipeFixtures.ts` was extended rather than only read, and why DLV-01/02/05 are not marked complete in REQUIREMENTS.md by this pilot plan alone.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Extended recipeFixtures.ts with the algorithm catalog, test commands, and mutation-ceremony copy**
- **Found during:** Task 1
- **Issue:** Task 1's own action mandates "copy/fields MUST come from the shared recipeFixtures.ts," but `recipeFixtures.ts` (as shipped by 02-01) only contained the SSH identity block, macOS globals, git-config fragment, and backup-path fixtures — it had no algorithm catalog (KEY-01), no test-stage commands/output (TEST-01/02), and no mutation-ceremony copy for the create flow specifically. The plan's `files_modified` list did not name `recipeFixtures.ts`.
- **Fix:** Extended the file additively (no existing export changed) with `algorithmCatalog` (the 5-entry KEY-01/KEY-03 catalog), `sshFormFilled`/`sshFormBlankPrefixHost` (SSHUI-01 field defaults), the two-stage test command/output constants (TEST-01/02, including the `ssh -G` proof), and the confirm/backup/result copy — all recipe-accurate, matching `recipes/ssh-config.recipe` and the file's own "structure, not key type" / "real values only, no placeholder option lists" convention (02-UX-DIRECTION.md §0 Risk 3).
- **Files modified:** `.planning/design/mockup-src/src/data/recipeFixtures.ts`
- **Verification:** `pnpm exec tsc --noEmit` and `pnpm build` both clean; the plan's exact grep/python acceptance checks for recipe-accurate copy all pass.
- **Committed in:** `468ef41` (Task 1)

**2. [Rule 1 - Bug] Fixed modal-overlay compositing truncating/overlapping a modal taller than its parent's natural content height**
- **Found during:** Task 3, first `make dummy-nav-e2e` run
- **Issue:** `internal/dummytui/model.go`'s `renderContent` centered and clamped the modal overlay against `lipgloss.Height(dimmed)` (the DIMMED PARENT's own rendered row count) instead of the real terminal height (`m.height`). This was a correct-looking fix in 02-02 (see that plan's key-decisions) because every registered screen at the time was a one-line placeholder, so the parent's natural height and the modal's natural height happened to coincide. Once create-flow registered real multi-line screens as a modal launched from the `identity-manager` PLACEHOLDER (still a 1-line body), the background (`dimmed`) — which `placeOverlay` physically cannot write past — had only ~4 rows total, so any modal screen with more content got silently truncated by `boundModalToViewport`, and once the clamped modal height collapsed the centering math to `y=0`, the overlay began overwriting the header row itself instead of the body. First surfaced as `make dummy-nav-e2e` hanging past its 60s budget (goroutine dump showed the harness still stuck mid-first-screen); manual PTY inspection showed frames like `giti4. Test connectivity...` — the header's first 4 characters followed directly by the modal's own first body line, with no row boundary between them.
- **Fix:** Compared against the real product's `tui/model.go`, which always pads its persistent-layout body to exactly `m.height - 2` rows before compositing (so its background never runs out of rows). Added `padToHeight` (pads the dimmed background with blank lines to `m.height` rows before compositing) and switched `available`/`modalOrigin` to measure against the real `m.height` with a 4-row vertical margin (mirroring `tui/model.go`'s own `verticalMargin` constant), instead of the parent's natural content height.
- **Files modified:** `internal/dummytui/model.go`
- **Verification:** Reproduced the timeout with the original code (goroutine dump confirmed the hang location); with the fix, `go test -tags e2e -race -timeout 180s -run TestDummyNavReachesAllScreens ./e2e/...` completes in ~14s (previously did not complete within 60s even after 5 screens); `make dummy-nav-e2e` (the plan's exact target, at the ORIGINAL 60s budget) passes cleanly; all pre-existing `internal/dummytui` tests (including `TestModel_ModalLaunchThroughModel_BreadcrumbAndEscReverts`, which exercises the same code path with a short placeholder modal) still pass; full `go test -race ./...`, `make fmt`, `make lint` all clean.
- **Committed in:** `c7a13f7` (Task 3)

---

**Total deviations:** 2 auto-fixed (1 missing-critical-functionality, 1 bug in shared pre-existing infrastructure)
**Impact on plan:** Both were necessary for correctness — the first to satisfy the plan's own "copy must come from recipeFixtures.ts" mandate and REQUIREMENTS.md KEY-01/SSHUI-01..03/TEST-01/02, the second to make `make dummy-nav-e2e` (an explicit Task 3 acceptance criterion) actually pass for a real (non-placeholder) surface. The model.go fix benefits every future modal surface, not just create-flow — no scope creep beyond what was required to complete this plan's own acceptance criteria.

## Issues Encountered

- **Task/subagent-dispatch tool unavailable in this executor's environment**, same limitation recorded in 02-01/02-02/02-03-SUMMARY.md. Task 3 calls for spawning `agent-ui-ux-designer` for two passes (an HTML-only aesthetic pass and the structured HTML↔TUI parity review). This executor's toolset was limited to `Read`/`Write`/`Edit`/`Bash` — no way to spawn a fresh-context subagent. In its place, this executor applied `agent-ui-ux-designer`'s documented methodology (F-pattern/left-side bias, Fitts's/Hick's Law, accessibility, distinctive typography) directly against all 24 captured screenshots and recorded the results in `CRITIQUE.md`, including one non-blocking observation (the TUI shell header lacks the HTML header's identity-count/health chip — a pre-existing, surface-uniform 02-02 characteristic outside §3's scope, not a create-flow-specific divergence). **This does not substitute for a fresh-context `agent-ui-ux-designer` pass** — flagging explicitly so the phase-level `/gsd-code-review` and the external cross-vendor (Codex) review the execution loop runs are not skipped for this plan's content, and so a follow-up fresh-context pass can be run against this plan specifically if the orchestrator has the capability this session lacked.
- The `superpowers:requesting-code-review` skill referenced by this plan's `<success_criteria>` was similarly unavailable for the same reason (no subagent-dispatch tool). Every task's `<acceptance_criteria>` was instead re-verified directly via its exact automated command (see Task Commits' verification notes and the Deviations section above) — all green.

## User Setup Required

None — no external service configuration required. `make screenshot-html-mockups`/`make screenshot-tui-mockups`/`make dummy-nav-e2e` all ran with only what `make setup-env` already provisions (pinned Chromium, freeze, pnpm).

## Next Phase Readiness

- The full per-surface pipeline (FIELDS → manifest → parity seed → mockup → dummy → capture → critique → parity 0-unresolved) is now proven end-to-end on a REAL surface (not a placeholder) — the six-surface fan-out (02-05 git-screen through 02-10) can follow this plan's exact shape.
- `internal/dummytui/model.go`'s modal-overlay height fix unblocks EVERY future modal/tall-screen surface, not just create-flow's own screens — no further action needed by fan-out plans on this front.
- `internal/dummytui/doc.go`'s key-allocation table now has `n` (create-flow) actually claimed in code, matching the table; `g` (git-screen) remains the next fan-out plan's LaunchKey to wire.
- Outstanding, not blocking this plan: two fresh-context reviews this session's toolset could not run (`agent-ui-ux-designer` subagent pass, `superpowers:requesting-code-review`) — recommend the orchestrator run both before or alongside the phase-level review gate, as recorded in Issues Encountered.
- DLV-01/DLV-02/DLV-05 remain incomplete in REQUIREMENTS.md pending the remaining 6 surfaces (02-05..02-10) — do not mark them complete on any single fan-out plan; the closing plan (likely 02-11/02-12) is where full-coverage completion should be recorded.

---
*Phase: 02-design-all-mockups-checkpoint-1*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 13 spot-checked created/modified files verified present on disk (`FIELDS.md`,
`manifest.json`, `parity.json`, `CRITIQUE.md`, `html/algo-catalog.png`,
`tui/algo-catalog.png`, `algo-catalog.route.tsx`, `result-success.route.tsx`,
`surface_createflow.go`, `surface_createflow_test.go`, `model.go`,
`recipeFixtures.ts`, this SUMMARY — 13/13 FOUND). All 3 task commit hashes
(`468ef41`, `6ec34c5`, `c7a13f7`) verified present in `git log --oneline --all`.
`go build ./...`, `go build -tags screenshot ./...`, `go build -tags e2e ./...`,
`go test -race ./...`, `make fmt`, `make lint`, `pnpm exec tsc --noEmit`,
`pnpm build`, `make screenshot-html-mockups`, `make screenshot-tui-mockups`, and
`make dummy-nav-e2e` all pass with zero issues at the time this summary was
written.
