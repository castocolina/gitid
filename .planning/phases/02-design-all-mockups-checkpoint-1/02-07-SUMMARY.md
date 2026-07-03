---
phase: 02-design-all-mockups-checkpoint-1
plan: 07
subsystem: ui
tags: [react, mui, bubbletea-v2, lipgloss-v2, go, design-mockup, tui-dummy, screenshot, e2e, fan-out-surface, global-ssh, danger-aware, advisory]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 01
    provides: "the shared four-region MUI shell + recipeFixtures.ts + route auto-discovery this plan's 6 global-ssh routes build on"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 02
    provides: "internal/dummytui's Register/RegisterOrReplace registry and the 02-02 global-ssh placeholder (key 2, screen \"entry\") this plan replaces"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 03
    provides: "the hardened manifest.json schema/loader, design_capture_test.go's manifest-driven capture, and dummy_nav_e2e_test.go's manifest-driven PTY walker"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 04
    provides: "the proven per-surface pipeline (FIELDS -> manifest -> parity seed -> mockup -> dummy -> capture -> critique -> parity 0-unresolved) this plan replicates verbatim"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 06
    provides: "the RegisterOrReplace-replaces-a-number-key-placeholder pattern (identity-manager, key 1) this plan follows for key 2, and the reHome()/dummyReady() prefix-check fix that makes ANY placeholder replacement e2e-safe"
provides:
  - "The Global SSH options screen (view 2, owned later by Phase 6) as 6 named states in BOTH media: /mui v7 routes under src/routes/global-ssh/*.route.tsx and internal/dummytui/surface_globalssh.go, replacing the 02-02 placeholder as the SOLE owner of ActivationKey \"2\" via RegisterOrReplace"
  - ".planning/design/global-ssh/{FIELDS.md, manifest.json, parity.json, CRITIQUE.md, html/*.png (6), tui/*.png (6)} — the fourth complete per-surface pipeline artifact set"
  - "recipeFixtures.ts extended with global-ssh-only exports — the GSSH-01 6-option dangerous-by-default set (pinning REQUIREMENTS.md's previously-open option-list item), IdentitiesOnly's full contractual explanation, and the concrete 3-of-4-applied/ForwardAgent-declined advisory-not-blocking demonstration"
  - "The global-ssh surface's intra-flow keys (v/f/w/y/z) reachable on the real cmd/gitid-dummy binary from its own entry screen, proven by the surface-scoped dummy-nav e2e"
affects: [02-08, 02-09, 02-10, 02-11, 02-12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Replicated the create-flow/git-screen/identity-manager per-surface pipeline exactly (FIELDS.md -> manifest.json -> parity.json seed -> /mui mockup -> dummytui surface -> capture -> critique -> parity 0-unresolved) on a FOURTH surface — the second number-key ActivationKey surface after identity-manager, confirming RegisterOrReplace's placeholder-replacement pattern generalizes cleanly to key 2"
    - "GSSH-01's previously-open 'dangerous-by-default option list' REQUIREMENTS.md item is pinned by this plan to the exact 6-option set the plan frontmatter specifies (StrictHostKeyChecking, ForwardAgent, HashKnownHosts, IdentitiesOnly, AddKeysToAgent, UseKeychain), acceptable per REQUIREMENTS.md's own 'Still Open' note (pin during the design phase)"
    - "Advisory-never-blocking (§4.4/§5's highest-risk affordance) demonstrated CONCRETELY, not just asserted: the fixture data has the user apply 3 of 4 'needs action' recommendations and deliberately leave ForwardAgent unchanged, and that declined choice stays visible — named explicitly — through fix-preview -> confirm-write -> backup-notice -> result-applied in both media, machine-checked by a dedicated Go test (TestGlobalSSH_AdvisoryNeverBlocking)"
    - "A live-PTY-viewport TUI compaction on options-list (git-screen's gsFieldsCompactLine precedent, generalized to a full-screen list rather than a single review block): 6 options originally rendered as 4 lines each overflowed the real, fixed 80x24 terminal cmd/gitid-dummy runs in; compacted to 1 line/option, keeping the full field set/values while moving the per-row prose explanation to option-detail"
key-files:
  created:
    - .planning/design/global-ssh/FIELDS.md
    - .planning/design/global-ssh/manifest.json
    - .planning/design/global-ssh/parity.json
    - .planning/design/global-ssh/CRITIQUE.md
    - .planning/design/global-ssh/html/*.png (6 files)
    - .planning/design/global-ssh/tui/*.png (6 files)
    - .planning/design/dummy-nav-frames/dummy-nav-global-ssh-*.txt (6 files, e2e evidence)
    - .planning/design/mockup-src/src/routes/global-ssh/options-list.route.tsx
    - .planning/design/mockup-src/src/routes/global-ssh/option-detail.route.tsx
    - .planning/design/mockup-src/src/routes/global-ssh/fix-preview.route.tsx
    - .planning/design/mockup-src/src/routes/global-ssh/confirm-write.route.tsx
    - .planning/design/mockup-src/src/routes/global-ssh/backup-notice.route.tsx
    - .planning/design/mockup-src/src/routes/global-ssh/result-applied.route.tsx
    - internal/dummytui/surface_globalssh.go
    - internal/dummytui/surface_globalssh_test.go
  modified:
    - .planning/design/mockup-src/src/data/recipeFixtures.ts (additive global-ssh section)
key-decisions:
  - "GSSH-01's dangerous-by-default option set is pinned to StrictHostKeyChecking/ForwardAgent/HashKnownHosts/IdentitiesOnly/AddKeysToAgent/UseKeychain, with AddKeysToAgent/UseKeychain already recipe-recommended (recipes/ssh-config.recipe's Host * block) and the other 4 needing action — a deliberate mix so options-list demonstrates BOTH the ✓-already-fine and !-needs-action rows, not just a wall of warnings"
  - "option-detail targets IdentitiesOnly (the single highest-risk option) with the full contractual explanation, mirroring identity-manager's detail-ssh-first single-target precedent — every other option's risk is still explained via its options-list one-liner (HTML) / current-recommended-risk triple (TUI compact row)"
  - "fix-preview/confirm-write/backup-notice/result-applied all carry the SAME concrete scenario (3 of 4 recommendations applied, ForwardAgent explicitly declined) end-to-end, rather than an abstract 'apply everything' preview — this makes the advisory-not-blocking affordance a machine-checkable, screenshot-verifiable fact instead of just banner copy"
  - "global-ssh's ScreenDef.Keys allocate a fresh linear ceremony chain v/f/w/y/z (view-detail, fix-preview, write, backup, continue) — mirroring git-screen's own f/m/r/w/y/z chain shape rather than reusing identity-manager's a/c/d/v/e/w/x/y letters, since keys are scoped per top-level surface and no collision exists against the two globally-reserved LaunchKeys (n, g)"
  - "options-list's TUI render is a one-line-per-option compaction (dropping the per-row prose one-liner and the master-detail preview pane) — a Rule 1 auto-fix discovered when the original 4-line-per-option layout overflowed the real 80x24 live PTY viewport; the full option data (key/current/recommended/risk) and the full prose explanation (option-detail) are unaffected, documented in FIELDS.md/CRITIQUE.md as an accepted §3 'widget mechanics MAY differ' divergence"
  - "DLV-01/DLV-02/DLV-05 are NOT marked complete in REQUIREMENTS.md, matching the 02-04/02-05/02-06 precedent: this plan ships the FOURTH of seven surfaces; full-coverage completion is deferred to whichever later plan closes out Phase 2 (likely 02-11/02-12)"

requirements-completed: []  # DLV-01/DLV-02/DLV-05 phase-spanning — see key-decisions; this plan ships 4/7 surfaces, not full coverage

# Metrics
duration: ~75min
completed: 2026-07-03
---

# Phase 2 Plan 07: Global SSH Options Fan-Out Surface (MUI Mockup + TUI Dummy + Capture + Parity) Summary

**The Global SSH options screen — a danger-aware, explained-not-gatekept review of the 6 GSSH-01 dangerous-by-default SSH options — built as 6 recipe-accurate named states in both /mui v7 and the TUI dummy, replacing the 02-02 placeholder as the sole owner of number key `2` via `RegisterOrReplace`, captured as 6+6 PNGs, and driven through the running `cmd/gitid-dummy` binary with a 0-unresolved structured parity gate — including a concrete, end-to-end proof that recommendations stay advisory (never blocking) by having the user apply 3 of 4 recommendations and visibly decline the fourth.**

## Performance

- **Duration:** ~75 min
- **Tasks:** 3 completed
- **Files modified:** 21 files across the 3 task commits (10 + 2 + 22 minus the 3 files touched in both Task 2 and Task 3 — see Task Commits)

## Accomplishments

- Authored `.planning/design/global-ssh/FIELDS.md` (per-screen field/order/label table for all 6 named states from 02-UX-DIRECTION.md §4.4) and `manifest.json` (6 hardened-schema entries — unique screen/signature, `keysFromHome` absolute from the number-key `2` entry).
- Pinned GSSH-01's previously-open dangerous-by-default option set to StrictHostKeyChecking/ForwardAgent/HashKnownHosts/IdentitiesOnly/AddKeysToAgent/UseKeychain in `recipeFixtures.ts`, each with a recipe-accurate current value, risk level, recommended value, and a one-line explanation.
- Built all 6 `/mui` v7 route files under `src/routes/global-ssh/` — the master-detail archetype for `options-list` (all 6 options as glyph+word rows with an advisory banner), `option-detail` (IdentitiesOnly's full contractual explanation), `fix-preview` (the exact diff, 3-of-4 applied), `confirm-write` (sentinel-visible managed-block text targeting `~/.ssh/config`), `backup-notice`, `result-applied` — all pulling real copy from new global-ssh-only `recipeFixtures.ts` exports.
- Built `internal/dummytui/surface_globalssh.go`: the number-key `2` primary surface, registered via `RegisterOrReplace` to cleanly replace the 02-02 `data.go` placeholder (no edit to `data.go`/`model.go`), with 6 `ScreenDef`s mirroring the mockup byte-for-byte on labels/copy/defaults.
- Captured 6 HTML + 6 TUI PNGs (`TestCaptureAllMockupScreens/global-ssh`) and proved every screen reachable on the REAL `cmd/gitid-dummy` binary with zero writes (`TestDummyNavReachesAllScreens/global-ssh`), after fixing a viewport-overflow bug discovered on the first e2e run (see Deviations).
- Ran the structured HTML↔TUI parity review against all 6 screenshot pairs (applying `agent-ui-ux-designer`'s methodology directly — see Issues Encountered) and closed `parity.json`'s 9 rows (the seven §3 dimensions + `per-option-explanation-verbatim` + `advisory-not-blocking`) to `status: resolved`, 0 unresolved.
- Re-ran the full, unfiltered `TestDummyNavReachesAllScreens` (no surface filter) after global-ssh's changes: all 33 screens across all 4 now-registered surfaces (create-flow 12, git-screen 7, identity-manager 8, global-ssh 6) pass — confirming no regression to the other three fan-out surfaces.

## Task Commits

Each task was committed atomically:

1. **Task 1: global-ssh FIELDS.md + manifest.json + parity.json seed + MUI mockup (6 states)** - `b92db57` (feat)
2. **Task 2: global-ssh TUI dummy surface (6 screens, RegisterOrReplace key 2, backend-free)** - `7ae0437` (feat)
3. **Task 3: Capture (both media) + parity critique -> 0 unresolved + options-list viewport-overflow fix** - `102f45d` (feat, includes the compaction fix)

**Plan metadata:** pending (this commit, created after this SUMMARY)

## Files Created/Modified

- `.planning/design/global-ssh/FIELDS.md` - per-screen field/label/order table for all 6 states
- `.planning/design/global-ssh/manifest.json` - 6 hardened-schema `{surface, screen, htmlRoute, keysFromHome, signature}` entries
- `.planning/design/global-ssh/parity.json` - 9 rows (7 §3 dimensions + 2 highest-risk-affordance rows), all `status: resolved`
- `.planning/design/global-ssh/CRITIQUE.md` - aesthetic pass (0 findings) + structured parity findings log (3 non-blocking observations, 2 the same class as prior surfaces' shared-infrastructure findings, 1 the viewport compaction)
- `.planning/design/global-ssh/html/*.png` (6) / `.planning/design/global-ssh/tui/*.png` (6) - captured screenshots
- `.planning/design/dummy-nav-frames/dummy-nav-global-ssh-*.txt` (6) - PTY e2e evidence frames
- `.planning/design/mockup-src/src/routes/global-ssh/*.route.tsx` (6) - the /mui mockup screens
- `.planning/design/mockup-src/src/data/recipeFixtures.ts` - extended with `GlobalSSHOption`/`GlobalSSHRiskLevel`, `globalSshOptions` (6 fixtures), `globalSshDetailTarget`, `globalSshDetailExplanation`, `globalSshAdvisoryNote`, `globalSshChosenToApply`/`globalSshDeclinedOption`, `globalSshHostStarBlockText`/`globalSshManagedBlockText`, `globalSshFixPreviewLines`, `globalSshBackupPath`, `globalSshResultMessage` (all new, additive exports)
- `internal/dummytui/surface_globalssh.go` (+ `_test.go`) - the TUI dummy global-ssh surface, 6 screens, `RegisterOrReplace`d as the sole owner of key `2`

## Decisions Made

See `key-decisions` in the frontmatter for the full rationale on: the GSSH-01 option-set pinning, why `option-detail` targets IdentitiesOnly, the concrete 3-of-4-applied/ForwardAgent-declined scenario carried through the whole ceremony, the v/f/w/y/z key allocation, the options-list viewport compaction, and why DLV-01/02/05 are not marked complete in REQUIREMENTS.md by this fan-out plan alone.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed options-list's TUI render overflowing the real 80x24 live PTY viewport**
- **Found during:** Task 3, first `TestDummyNavReachesAllScreens/global-ssh/options-list` run
- **Issue:** The initial `renderGSSHOptionsList` rendered each of the 6 options as 4 lines (glyph+key+status chip, current/recommended, one-liner) plus a heading, an un-wrapped advisory banner, and a 5-line "Preview" block — roughly 30+ logical lines. `cmd/gitid-dummy` runs inside a REAL, fixed 80x24 PTY with no scroll region (`e2e/ui_pty_e2e_test.go`'s `ptyTermWidth`/`ptyTermHeight`), unlike the static `RenderScreen()`→`freeze` capture path used by `TestCaptureAllMockupScreens`, which has no height limit — so the static TUI capture (Task 1/2's own `go test -race ./internal/dummytui/... -run GlobalSSH`) passed cleanly while the live e2e walk failed: the frame captured at the 5-second timeout showed the header and 4 of 6 option rows, cut off mid-word, with neither `UseKeychain` nor the trailing manifest signature ever reaching the visible screen.
- **Fix:** Compacted `options-list`'s render to ONE line per option (`{glyph} {key}   {current} -> {recommended}` or `{current} (already set)`, bracketed with `[{risk}]`), dropping the per-row prose one-liner and the master-detail "Preview" block, replaced with a single one-line footer hint naming the option-detail target and the next key. This is the same class of live-PTY-viewport compaction `git-screen`'s `gsFieldsCompactLine1/2/3` already established for `review-readonly`/`confirm-write` (documented in that file's own comments) — the field SET/ORDER/VALUES (all 6 keys, current values, recommended values, risk levels) are unchanged; only the supplementary prose moved to `option-detail`, one keystroke away in both media.
- **Verification:** `go test -tags e2e -race -run 'TestDummyNavReachesAllScreens/global-ssh' ./e2e/...` passes (6/6 subtests). Re-ran the full, unfiltered `TestDummyNavReachesAllScreens` (no surface filter, 60s timeout) to confirm no regression to create-flow/git-screen/identity-manager: all 33 screens across all 4 registered surfaces pass. `go test -race ./internal/dummytui/... -run GlobalSSH` and `go test -tags screenshot -run 'TestCaptureAllMockupScreens/global-ssh' ./internal/screenshot/...` (freeze capture, unaffected by the fix but re-run to confirm) both still pass.
- **Files modified:** `internal/dummytui/surface_globalssh.go`
- **Committed in:** `102f45d` (Task 3)
- **Documented as an accepted divergence in:** `FIELDS.md` rows 6/8 (`options-list`) and `CRITIQUE.md` finding #2

**2. [Rule 3 - Blocking issue] Provisioned `freeze` and `go env GOPATH/bin` onto PATH for this session**
- **Found during:** Task 3, first capture attempt
- **Issue:** This session's shell PATH did not include `$(go env GOPATH)/bin` (where `freeze` was already installed from a prior session's `make setup-env`) by default, matching 02-06's own Deviation #2.
- **Fix:** Exported `PATH="$HOME/go/bin:$PATH"` for the Bash calls that needed it (screenshot capture, e2e) — no repo file changed, no reinstall performed (the pinned `freeze` was already present from prior 02-04/02-05/02-06 sessions).
- **Verification:** `which freeze` resolves; `TestCaptureAllMockupScreens/global-ssh` passes with all 12 PNGs captured.
- **Committed in:** N/A (local environment/session PATH only, no repo file changed)

---

**Total deviations:** 2 auto-fixed (1 Rule-1 TUI-viewport bug fix required by this plan's own Task 3 acceptance criteria, 1 local session PATH provisioning)
**Impact on plan:** The viewport-overflow fix was necessary for this plan's own Task 3 acceptance criteria (`TestDummyNavReachesAllScreens/global-ssh`, an explicit verify command) to pass, and is a reminder for future fan-out plans (02-08 global-git, 02-09 health, 02-10 fixer — all master-detail/list-shaped surfaces like global-ssh, per 02-UX-DIRECTION.md §2) that any multi-row list screen must budget its TUI line count against the real 80x24 live-PTY viewport (git-screen's compact-line precedent, now demonstrated on a full list screen, not just a single review block), NOT just the unconstrained static `RenderScreen()`→`freeze` capture path. Confirmed backward-compatible via the full 33-screen, 4-surface e2e re-run.

## Issues Encountered

- **Task/subagent-dispatch tool unavailable in this executor's environment**, same limitation recorded in 02-01 through 02-06's SUMMARY.md files. Task 3 calls for spawning `agent-ui-ux-designer` for two passes (an HTML-only aesthetic pass and the structured HTML↔TUI parity review). This executor's toolset was limited to `Read`/`Write`/`Edit`/`Bash` — no way to spawn a fresh-context subagent. In its place, this executor applied `agent-ui-ux-designer`'s documented methodology (F-pattern/left-side bias, Fitts's/Hick's Law, accessibility, distinctive typography) directly against all 12 captured screenshots and recorded the results in `CRITIQUE.md`. **This does not substitute for a fresh-context `agent-ui-ux-designer` pass** — flagging explicitly so the phase-level `/gsd-code-review` and the external cross-vendor (Codex) review the execution loop runs are not skipped for this plan's content.
- The `superpowers:requesting-code-review` skill referenced by this plan's `<success_criteria>` was similarly unavailable for the same reason (no subagent-dispatch tool). Every task's `<acceptance_criteria>` was instead re-verified directly via its exact automated command (see Task Commits' verification notes and the Deviations section above) — all green, plus a full-repo `go build ./...`, `go test -race ./...`, and `make lint` pass beyond what the plan's own per-task verify commands required, and a plan-scoped `git diff` proof (commits `b92db57~1..102f45d`) that `registry.go`/`model.go`/`data.go`/`App.tsx`/`package.json`/`pnpm-lock.yaml`/`Makefile`/`e2e/dummy_nav_e2e_test.go` were never touched by this plan.

## User Setup Required

None — no external service configuration required. `freeze`/`pnpm` were already provisioned from prior 02-04/02-05/02-06 sessions; only this session's shell `PATH` needed adjusting (see Deviations #2).

## Next Phase Readiness

- The per-surface pipeline is now proven on FOUR independent surfaces: create-flow's branching 12-screen tree, git-screen's linear 7-screen chain, identity-manager's number-key nav-root with 5 `placeOverlay`-composited intra-surface modals, and global-ssh's number-key master-detail + linear ceremony chain — the remaining fan-out plans (02-08 global-git, 02-09 health, 02-10 fixer) can follow whichever shape their own UX-DIRECTION.md §4 manifest dictates, and should expect the SAME `RegisterOrReplace`-replaces-a-placeholder pattern for their own number keys (3/4/5).
- **Load-bearing reminder for 02-08/02-09/02-10:** any list-shaped screen with more than ~3-4 rows (global-git's `options-list`, health's `health-with-findings`, etc.) should budget its TUI content against the REAL 80x24 live-PTY viewport from the start (git-screen's compact-line precedent + this plan's own options-list fix) rather than discovering the overflow at e2e time — the static `RenderScreen()`→`freeze` capture path has no height limit and will NOT catch this class of bug; only the PTY-driven e2e walk will.
- `internal/dummytui/doc.go`'s key-allocation table already had `2` pre-allocated to global-ssh since 02-02's initial table authoring and is now actually claimed in code, matching the table exactly — no `doc.go` edit was needed or made.
- Outstanding, not blocking this plan: two fresh-context reviews this session's toolset could not run (`agent-ui-ux-designer` subagent pass, `superpowers:requesting-code-review`) — recommend the orchestrator run both before or alongside the phase-level review gate, as recorded in Issues Encountered.
- DLV-01/DLV-02/DLV-05 remain incomplete in REQUIREMENTS.md pending the remaining 3 surfaces (02-08..02-10) — do not mark them complete on any single fan-out plan; the closing plan (likely 02-11/02-12) is where full-coverage completion should be recorded.

---
*Phase: 02-design-all-mockups-checkpoint-1*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 12 spot-checked created/modified files verified present on disk (`FIELDS.md`,
`manifest.json`, `parity.json`, `CRITIQUE.md`, `html/options-list.png`,
`tui/options-list.png`, `options-list.route.tsx`, `result-applied.route.tsx`,
`surface_globalssh.go`, `surface_globalssh_test.go`, `recipeFixtures.ts`, this
SUMMARY — 12/12 FOUND). All 3 task commit hashes (`b92db57`, `7ae0437`,
`102f45d`) verified present in `git log --oneline --all`. `go build ./...`,
`go test -race ./...`, `make lint`, `pnpm exec tsc --noEmit`, `pnpm build`,
`TestCaptureAllMockupScreens/global-ssh` (screenshot tag), and the full
unfiltered `TestDummyNavReachesAllScreens` (e2e tag, all 4 registered
surfaces, 33 screens) all pass with zero issues at the time this summary was
written.
