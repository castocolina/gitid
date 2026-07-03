---
phase: 02-design-all-mockups-checkpoint-1
plan: 06
subsystem: ui
tags: [react, mui, bubbletea-v2, lipgloss-v2, go, design-mockup, tui-dummy, screenshot, e2e, fan-out-surface, identity-manager, nav-root]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 01
    provides: "the shared four-region MUI shell + recipeFixtures.ts + route auto-discovery this plan's 8 identity-manager routes build on"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 02
    provides: "internal/dummytui's Register/RegisterOrReplace registry, the placeOverlay compositing primitive (overlay.go), and the 02-02 identity-manager placeholder (key 1, screen \"entry\") this plan replaces"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 03
    provides: "the hardened manifest.json schema/loader, design_capture_test.go's manifest-driven capture, and dummy_nav_e2e_test.go's manifest-driven PTY walker"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 04
    provides: "the proven per-surface pipeline (FIELDS -> manifest -> parity seed -> mockup -> dummy -> capture -> critique -> parity 0-unresolved) this plan replicates verbatim"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 05
    provides: "the second proof of the per-surface pipeline (git-screen, a linear chain) and its recipeFixtures.ts additive-export precedent this plan follows"
provides:
  - "The Identity Manager — the app's HOME/view-1, the nav root every other surface's dummy-nav e2e originates from — as 8 named states in BOTH media: /mui v7 routes under src/routes/identity-manager/*.route.tsx and internal/dummytui/surface_identitymanager.go, replacing the 02-02 placeholder as the SOLE owner of ActivationKey \"1\" via RegisterOrReplace"
  - ".planning/design/identity-manager/{FIELDS.md, manifest.json, parity.json, CRITIQUE.md, html/*.png (8), tui/*.png (8)} — the third complete per-surface pipeline artifact set"
  - "recipeFixtures.ts extended with identity-manager-only exports — one fixture identity per MGR-02 8-label state taxonomy (internal/identity/state.go's locked vocabulary)"
  - "The identity-manager surface's intra-flow keys (a/c/d/v/e/w/x/y) reachable on the real cmd/gitid-dummy binary from its own entry screen, proven by the surface-scoped dummy-nav e2e"
  - "A fix to e2e/dummy_nav_e2e_test.go's reHome() so it no longer hardcodes the 02-02-placeholder-era literal breadcrumb \"identity-manager/entry\" — required for ANY fan-out plan that replaces the identity-manager placeholder, not identity-manager-specific, and verified backward-compatible with create-flow/git-screen"
affects: [02-07, 02-08, 02-09, 02-10, 02-11, 02-12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Replicated the create-flow/git-screen per-surface pipeline exactly (FIELDS.md -> manifest.json -> parity.json seed -> /mui mockup -> dummytui surface -> capture -> critique -> parity 0-unresolved) on a THIRD surface, this time the nav-ROOT surface (ActivationKey, not a keyless LaunchFrom modal) — confirming the pipeline generalizes to the number-key surfaces, not just the two keyless modal flows"
    - "RegisterOrReplace used for the first time by a fan-out plan (review HIGH-2's designed purpose): identity-manager's 8 real screens replace the 02-02 data.go placeholder's single \"entry\" screen at init()-time, with registry.go's registration-time collision guard proving no LaunchKey/ScreenDef.Keys clash against the already-registered create-flow (\"n\") / git-screen (\"g\") surfaces"
    - "placeOverlay (overlay.go) called DIRECTLY from a surface file for the first time, for INTRA-surface modal-shaped screens (action-menu/clone-name-prompt/delete-choice/confirm-destructive/backup-notice composited over a dimmed identity list) — distinct from model.go's own use of the same primitive for CROSS-surface keyless-modal launches (create-flow/git-screen pushed onto navState.modalStack); both share the same overlay.go helpers but are invoked from different call sites for different reasons"
    - "8-label MGR-02 state taxonomy (internal/identity/state.go's locked vocabulary) rendered as ONE fixture identity per label, letting a single list-populated screen exercise every state at once rather than requiring 8 separate near-duplicate screens"

key-files:
  created:
    - .planning/design/identity-manager/FIELDS.md
    - .planning/design/identity-manager/manifest.json
    - .planning/design/identity-manager/parity.json
    - .planning/design/identity-manager/CRITIQUE.md
    - .planning/design/identity-manager/html/*.png (8 files)
    - .planning/design/identity-manager/tui/*.png (8 files)
    - .planning/design/dummy-nav-frames/dummy-nav-identity-manager-*.txt (8 files, e2e evidence)
    - .planning/design/mockup-src/src/routes/identity-manager/list-empty.route.tsx
    - .planning/design/mockup-src/src/routes/identity-manager/list-populated.route.tsx
    - .planning/design/mockup-src/src/routes/identity-manager/detail-ssh-first.route.tsx
    - .planning/design/mockup-src/src/routes/identity-manager/action-menu.route.tsx
    - .planning/design/mockup-src/src/routes/identity-manager/clone-name-prompt.route.tsx
    - .planning/design/mockup-src/src/routes/identity-manager/delete-choice.route.tsx
    - .planning/design/mockup-src/src/routes/identity-manager/confirm-destructive.route.tsx
    - .planning/design/mockup-src/src/routes/identity-manager/backup-notice.route.tsx
    - internal/dummytui/surface_identitymanager.go
    - internal/dummytui/surface_identitymanager_test.go
  modified:
    - .planning/design/mockup-src/src/data/recipeFixtures.ts (additive identity-manager section)
    - e2e/dummy_nav_e2e_test.go (Rule 3 blocking-issue fix, see Deviations)
    - .planning/design/dummy-nav-frames/*.txt (18 files regenerated as an evidence side-effect of the full dummy-nav-e2e re-run after the reHome fix — not semantically authored content)

key-decisions:
  - "identity-manager's ScreenDef.Keys allocate a=action-menu / c=clone-name-prompt / d=delete-choice from the UX-DIRECTION §2 key-allocation table (the single authority) on the list-populated/action-menu screens; v/e/w/x/y are additional intra-surface-only keys (view-detail, empty-state demo toggle, clone-write, delete-continue, confirm-write) that never appear in the central table — the same 'extra letters beyond the central table' precedent create-flow's/git-screen's own b/c/f/r/m/t/w/x/y/z and f/m/r/w/y/z letters already established"
  - "detail-ssh-first deliberately targets the SSH-only 'work' fixture identity (not the fully-populated 'personal' one) so the screen proves MGR-03/MGR-07's rule with a real absence (an explicit 'No Git identity configured' note) rather than a hypothetical caveat next to fields that ARE populated"
  - "The five modal-shaped screens (action-menu, clone-name-prompt, delete-choice, confirm-destructive, backup-notice) call overlay.go's placeOverlay/modalOrigin/boundModalToViewport DIRECTLY from surface_identitymanager.go's own render functions, using model.go's defaultWidth/defaultHeight (100x30) as a fixed, deterministic capture geometry — rather than going through navState.modalStack (which is reserved for CROSS-surface keyless-modal launches, per doc.go's modal-launch contract); this is a same-package, same-primitive, different-call-site pattern, not a new compositing mechanism"
  - "8 fixture identities (one per MGR-02 label) reuse the 'personal'/personal.github.com alias already canonical in create-flow/git-screen for the 'complete' row, and introduce 7 new names (work/opensource/archived/staging/clientA/clientB/legacy) for the other 7 labels — kept in recipeFixtures.ts as an additive-only section, matching git-screen's own additive-export precedent (no edit to any create-flow- or git-screen-owned export)"
  - "DLV-01/DLV-02/DLV-05 are NOT marked complete in REQUIREMENTS.md, matching the 02-04/02-05 precedent: this plan ships the THIRD of seven surfaces; full-coverage completion is deferred to whichever later plan closes out Phase 2 (likely 02-11/02-12)"

requirements-completed: []  # DLV-01/DLV-02/DLV-05 phase-spanning — see key-decisions; this plan ships 3/7 surfaces, not full coverage

# Metrics
duration: ~90min
completed: 2026-07-03
---

# Phase 2 Plan 06: Identity Manager Fan-Out Surface (MUI Mockup + TUI Dummy + Capture + Parity) Summary

**The Identity Manager — the app's HOME/view-1 and navigation root — built as 8 recipe/requirements-accurate named states in both /mui v7 and the TUI dummy, replacing the 02-02 placeholder as the sole owner of number key `1` via `RegisterOrReplace`, captured as 8+8 PNGs, and driven through the running `cmd/gitid-dummy` binary with a 0-unresolved structured parity gate — including the MGR-06 delete-choice safe-default affordance, the 8-label MGR-02 taxonomy rendered NO_COLOR-legibly (glyph+word, never color alone), and the MGR-03/MGR-07 SSH-first/never-fabricate-Git-attributes rule.**

## Performance

- **Duration:** ~90 min
- **Tasks:** 3 completed
- **Files modified:** 47 files across the 3 task commits (12 + 2 + 47, see Task Commits — Task 3's count includes 18 regenerated e2e-evidence `.txt` frames, a side-effect of the `reHome` fix's full-suite re-run)

## Accomplishments

- Authored `.planning/design/identity-manager/FIELDS.md` (per-screen field/order/label table for all 8 named states from 02-UX-DIRECTION.md §4(3)) and `manifest.json` (8 hardened-schema entries — unique screen/signature, `keysFromHome` absolute from the number-key `1` entry).
- Built all 8 `/mui` v7 route files under `src/routes/identity-manager/` — the master-detail archetype for `list-populated` (all 8 MGR-02 states as glyph+WORD rows, never color alone) and `detail-ssh-first` (SSH shown first, Git section explicitly absent for the SSH-only `work` identity, MGR-03/MGR-07); the true first-run `list-empty` landing; `action-menu`; `clone-name-prompt` with a name distinct from the source (MGR-04); `delete-choice` with the safer "Git identity only" option default-focused and the irreversible "everything" option never default-focused (MGR-06, §5); `confirm-destructive`'s strongest-confirm copy; `backup-notice`'s two timestamped paths — all pulling real copy from new identity-manager-only `recipeFixtures.ts` exports.
- Built `internal/dummytui/surface_identitymanager.go`: the number-key `1` primary surface, registered via `RegisterOrReplace` to cleanly replace the 02-02 `data.go` placeholder (no edit to `data.go`/`model.go`), with 8 `ScreenDef`s mirroring the mockup byte-for-byte on labels/copy/defaults. Five modal-shaped screens composite a bordered box over a dimmed identity list via `placeOverlay` (`overlay.go`), called directly since these are intra-surface transitions, not cross-surface keyless-modal launches. The `ScreenDef.Keys` graph connects all 8 screens from the `list-populated` entry screen.
- Captured 8 HTML + 8 TUI PNGs (`TestCaptureAllMockupScreens/identity-manager`) and proved every screen — including all 5 `placeOverlay`-composited ones — reachable on the REAL `cmd/gitid-dummy` binary with zero writes (`TestDummyNavReachesAllScreens/identity-manager`).
- Ran the structured HTML↔TUI parity review against all 8 screenshot pairs (applying `agent-ui-ux-designer`'s methodology directly — see Issues Encountered) and closed `parity.json`'s 10 rows (the seven §3 dimensions + `delete-choice-safe-default` + `no_color-row-health` + `ssh-first-detail`) to `status: resolved`, 0 unresolved.
- Discovered and fixed a genuine pre-existing blocking issue in shared e2e infrastructure (see Deviations): `e2e/dummy_nav_e2e_test.go`'s `reHome()` hardcoded the literal breadcrumb `"identity-manager/entry"`, an assumption baked in by 02-03 while identity-manager was still the 02-02 placeholder. This plan is the first to actually replace that placeholder's entry screen, so the exact-match wait started failing for every identity-manager e2e subtest.

## Task Commits

Each task was committed atomically:

1. **Task 1: identity-manager FIELDS.md + manifest.json + parity.json seed + MUI mockup (8 states)** - `dc14c6c` (feat)
2. **Task 2: identity-manager TUI dummy surface (8 screens, key 1, placeOverlay modals)** - `ef49651` (feat)
3. **Task 3: Capture (both media) + parity critique -> 0 unresolved + e2e reHome fix** - `661e5a7` (feat, includes the reHome deviation fix)

**Plan metadata:** pending (this commit, created after this SUMMARY)

## Files Created/Modified

- `.planning/design/identity-manager/FIELDS.md` - per-screen field/label/order table for all 8 states
- `.planning/design/identity-manager/manifest.json` - 8 hardened-schema `{surface, screen, htmlRoute, keysFromHome, signature}` entries
- `.planning/design/identity-manager/parity.json` - 10 rows (7 §3 dimensions + 3 highest-risk-affordance rows), all `status: resolved`
- `.planning/design/identity-manager/CRITIQUE.md` - aesthetic pass (0 findings) + structured parity findings log (2 non-blocking observations, both the same class as create-flow's/git-screen's own accepted shared-infrastructure findings)
- `.planning/design/identity-manager/html/*.png` (8) / `.planning/design/identity-manager/tui/*.png` (8) - captured screenshots
- `.planning/design/dummy-nav-frames/dummy-nav-identity-manager-*.txt` (8) - PTY e2e evidence frames
- `.planning/design/mockup-src/src/routes/identity-manager/*.route.tsx` (8) - the /mui mockup screens
- `.planning/design/mockup-src/src/data/recipeFixtures.ts` - extended with `IdentityManagerRow`/`IdentityManagerState`, `identityManagerRows` (8 fixtures), `identityManagerStateGlyph`/`identityManagerStateTone`, `identityManagerDetailTarget`, `identityManagerActionTarget`, `identityManagerCloneSuggestedName`, `identityManagerDeleteChoices`, `identityManagerBackupPaths` (all new, additive exports)
- `internal/dummytui/surface_identitymanager.go` (+ `_test.go`) - the TUI dummy identity-manager surface, 8 screens, `RegisterOrReplace`d as the sole owner of key `1`
- `e2e/dummy_nav_e2e_test.go` - `reHome()`/`dummyReady()` doc-comment fix (see Deviations)

## Decisions Made

See `key-decisions` in the frontmatter for the full rationale on: the a/c/d central-table key allocation plus the v/e/w/x/y surface-local keys, why `detail-ssh-first` targets the SSH-only `work` identity, the direct (same-package, different-call-site) use of `placeOverlay` for intra-surface modals vs. `model.go`'s cross-surface use of the same primitive, the 8-fixture-identity design, and why DLV-01/02/05 are not marked complete in REQUIREMENTS.md by this fan-out plan alone.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] Fixed `e2e/dummy_nav_e2e_test.go`'s `reHome()` hardcoded breadcrumb literal, which broke on this plan's own placeholder replacement**
- **Found during:** Task 3, first `TestDummyNavReachesAllScreens/identity-manager` run
- **Issue:** `reHome()` (added by 02-03, while identity-manager was still the 02-02 `data.go` placeholder whose sole screen ID was literally `"entry"`) waited for the EXACT breadcrumb string `"identity-manager/entry"` before proceeding to each manifest entry's `keysFromHome` walk. This plan is the first fan-out plan that actually replaces that placeholder (`RegisterOrReplace`) with a real entry screen — `list-populated`, per UX-DIRECTION.md §4(3)'s state ordering, not `"entry"`. Every one of this plan's 8 e2e subtests (`list-populated` through `backup-notice`) timed out after 3s in `reHome`, never reaching the actual `keysFromHome` walk.
- **Fix:** Changed `reHome`'s wait condition from the exact string `"identity-manager/entry"` to the SAME prefix check `dummyReady()` (a few lines above, in the same file) already uses: `strings.Contains(text, "identity-manager/")`. This is screen-ID-agnostic by design — it does not care WHICH screen the currently-registered identity-manager surface treats as its entry, so it stays correct regardless of which future plan's screen set is active. Updated both functions' doc comments to explain the prefix-check rationale and point at each other.
- **Verification:** `go test -tags e2e -race -run 'TestDummyNavReachesAllScreens/identity-manager' ./e2e/...` passes (8/8 subtests). Re-ran the FULL suite (`go test -tags e2e -race -timeout 60s -run TestDummyNav ./e2e/...`, no surface filter) to prove the fix does not regress create-flow or git-screen, which also depend on `reHome` for their own re-home-before-each-entry step: all 25 screens across all 3 registered surfaces (create-flow 12, git-screen 7, identity-manager 8) pass.
- **Files modified:** `e2e/dummy_nav_e2e_test.go`
- **Committed in:** `661e5a7` (Task 3)

**2. [Rule 3 - Blocking issue] Provisioned `freeze` and `pnpm` onto PATH for this session**
- **Found during:** Task 3, first capture attempt
- **Issue:** This session's shell PATH did not include `$(go env GOPATH)/bin` (where `freeze` was already installed from a prior session's `make setup-env`) or the system `pnpm` binary's directory by default.
- **Fix:** Exported `PATH="$(go env GOPATH)/bin:$HOME/.local/bin:$PATH"` for the Bash calls that needed it (screenshot capture, e2e, lint) — no repo file changed, no reinstall performed (the pinned `freeze@v0.2.2` and `pnpm` were already present from prior 02-04/02-05 sessions).
- **Verification:** `which freeze` resolves; `TestCaptureAllMockupScreens/identity-manager` passes with all 16 PNGs captured.
- **Committed in:** N/A (local environment/session PATH only, no repo file changed)

---

**Total deviations:** 2 auto-fixed (1 blocking shared-test-infrastructure fix required by this plan's own placeholder replacement, 1 local session PATH provisioning)
**Impact on plan:** The `reHome` fix was necessary for this plan's own Task 3 acceptance criteria (`TestDummyNavReachesAllScreens/identity-manager`, an explicit verify command) to pass, and is a correctness fix for ANY future fan-out plan that replaces a number-key placeholder (global-ssh `2`, global-git `3`, health `4`, fixer `5` — 02-07 through 02-10 will each hit the exact same class of bug when they replace their OWN placeholder's `"entry"` screen ID, unless they happen to also name their entry screen `"entry"`). Confirmed backward-compatible via the full 25-screen, 3-surface e2e re-run.

## Issues Encountered

- **Task/subagent-dispatch tool unavailable in this executor's environment**, same limitation recorded in 02-01 through 02-05's SUMMARY.md files. Task 3 calls for spawning `agent-ui-ux-designer` for two passes (an HTML-only aesthetic pass and the structured HTML↔TUI parity review). This executor's toolset was limited to `Read`/`Write`/`Edit`/`Bash` — no way to spawn a fresh-context subagent. In its place, this executor applied `agent-ui-ux-designer`'s documented methodology (F-pattern/left-side bias, Fitts's/Hick's Law, accessibility, distinctive typography) directly against all 16 captured screenshots and recorded the results in `CRITIQUE.md`. **This does not substitute for a fresh-context `agent-ui-ux-designer` pass** — flagging explicitly so the phase-level `/gsd-code-review` and the external cross-vendor (Codex) review the execution loop runs are not skipped for this plan's content.
- The `superpowers:requesting-code-review` skill referenced by this plan's `<success_criteria>` was similarly unavailable for the same reason (no subagent-dispatch tool). Every task's `<acceptance_criteria>` was instead re-verified directly via its exact automated command (see Task Commits' verification notes and the Deviations section above) — all green, plus a full-repo `go build ./...`, `go test -race ./internal/dummytui/...`, and `make lint` pass beyond what the plan's own per-task verify commands required, and a plan-scoped `git diff` proof (commits `dc14c6c~1..661e5a7`) that `registry.go`/`model.go`/`data.go`/`App.tsx`/`package.json`/`pnpm-lock.yaml`/`Makefile` were never touched by this plan.

## User Setup Required

None — no external service configuration required. `freeze`/`pnpm` were already provisioned from prior 02-04/02-05 sessions; only this session's shell `PATH` needed adjusting (see Deviations #2).

## Next Phase Readiness

- The per-surface pipeline is now proven on THREE independent surfaces: create-flow's branching 12-screen tree, git-screen's linear 7-screen chain, and identity-manager's number-key nav-root with 5 `placeOverlay`-composited intra-surface modals — the remaining fan-out plans (02-07 global-ssh through 02-10 fixer) can follow whichever shape their own UX-DIRECTION.md §4 manifest dictates, and are all number-key surfaces like identity-manager (not keyless `LaunchFrom` modals like create-flow/git-screen), so they should expect to hit the SAME `RegisterOrReplace`-replaces-a-placeholder pattern this plan did.
- **Load-bearing fix for 02-07 through 02-10:** `e2e/dummy_nav_e2e_test.go`'s `reHome()` no longer assumes the identity-manager placeholder's `"entry"` screen ID — but `reHome()` only re-homes to identity-manager (key `1`), it does not need to know about global-ssh/global-git/health/fixer's OWN entry screen IDs, since those surfaces don't need a "re-home" step (they ARE the target of the manifest-entry's own `keysFromHome`, driven fresh from the identity-manager re-home each time). No further fix should be needed for 02-07..02-10 on this front, but each should double check their own capture/e2e run cleanly against the now-real identity-manager surface (no more `"Identity Manager — placeholder"` text anywhere in the registry).
- `internal/dummytui/doc.go`'s key-allocation table already had `1`/`a`/`c`/`d` pre-allocated to identity-manager since 02-02's initial table authoring and is now actually claimed in code, matching the table exactly — no `doc.go` edit was needed or made.
- Outstanding, not blocking this plan: two fresh-context reviews this session's toolset could not run (`agent-ui-ux-designer` subagent pass, `superpowers:requesting-code-review`) — recommend the orchestrator run both before or alongside the phase-level review gate, as recorded in Issues Encountered.
- DLV-01/DLV-02/DLV-05 remain incomplete in REQUIREMENTS.md pending the remaining 4 surfaces (02-07..02-10) — do not mark them complete on any single fan-out plan; the closing plan (likely 02-11/02-12) is where full-coverage completion should be recorded.

---
*Phase: 02-design-all-mockups-checkpoint-1*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 13 spot-checked created/modified files verified present on disk (`FIELDS.md`,
`manifest.json`, `parity.json`, `CRITIQUE.md`, `html/list-empty.png`,
`tui/list-empty.png`, `list-populated.route.tsx`, `backup-notice.route.tsx`,
`surface_identitymanager.go`, `surface_identitymanager_test.go`, `recipeFixtures.ts`,
`e2e/dummy_nav_e2e_test.go`, this SUMMARY — 13/13 FOUND). All 3 task commit hashes
(`dc14c6c`, `ef49651`, `661e5a7`) verified present in `git log --oneline --all`.
`go build ./...`, `go test -race ./internal/dummytui/...`, `make lint`,
`pnpm exec tsc --noEmit`, `pnpm build`, `TestCaptureAllMockupScreens/identity-manager`
(screenshot tag), and `TestDummyNavReachesAllScreens/identity-manager` (e2e tag, plus
the full unfiltered `TestDummyNavReachesAllScreens` across all 3 registered surfaces)
all pass with zero issues at the time this summary was written.
