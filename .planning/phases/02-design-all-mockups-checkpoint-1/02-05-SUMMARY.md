---
phase: 02-design-all-mockups-checkpoint-1
plan: 05
subsystem: ui
tags: [react, mui, bubbletea-v2, lipgloss-v2, go, design-mockup, tui-dummy, screenshot, e2e, fan-out-surface, git-config]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 01
    provides: "the shared four-region MUI shell + recipeFixtures.ts + route auto-discovery this plan's 7 git-screen routes build on"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 02
    provides: "internal/dummytui's Register/RegisterOrReplace registry, the LaunchFrom/LaunchKey modal-launch contract, and RenderScreen — the launch mechanism git-screen's surface plugs into"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 03
    provides: "the hardened manifest.json schema/loader, design_capture_test.go's manifest-driven capture, and dummy_nav_e2e_test.go's manifest-driven PTY walker"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 04
    provides: "the proven per-surface pipeline (FIELDS -> manifest -> parity seed -> mockup -> dummy -> capture -> critique -> parity 0-unresolved) this plan replicates verbatim, plus the model.go modal-overlay height fix this plan's taller screens exercise"
provides:
  - "The git-configuration screen (Phase 4's future backend surface) as 7 named states in BOTH media: /mui v7 routes under src/routes/git-screen/*.route.tsx and internal/dummytui/surface_gitscreen.go, both recipe/requirements-accurate (gpg.format=ssh, user.signingkey as a path, gitdir-default match strategy, allowed_signers byte-identical to user.email per GITUI-04)"
  - ".planning/design/git-screen/{FIELDS.md, manifest.json, parity.json, CRITIQUE.md, html/*.png (7), tui/*.png (7)} — the second complete per-surface pipeline artifact set (after create-flow), replicable by the remaining fan-out plans (02-06..02-10)"
  - "recipeFixtures.ts extended with git-screen-only exports (gitScreenFragmentPath = ~/.gitconfig.d/<identity> per the already-built GITUI-02, the includeIf previews per match strategy, the confirm-write targets, and the backup/result copy)"
  - "The git-screen surface reachable on the real cmd/gitid-dummy binary from Identities via the LaunchKey 'g' (allocated in UX-DIRECTION.md §2's key-allocation table / doc.go), proven by the surface-scoped dummy-nav e2e"
affects: [02-06, 02-07, 02-08, 02-09, 02-10, 02-11, 02-12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Replicated the create-flow pilot's per-surface pipeline exactly (FIELDS.md -> manifest.json -> parity.json seed -> /mui mockup -> dummytui surface -> capture -> critique -> parity 0-unresolved) on a second, independent surface — confirming the pipeline generalizes"
    - "Fixed 80x24 PTY viewport budget discovered: a modal has only 20 available body rows (model.go verticalMargin=4); a screen previewing multiple files' full raw content (git-screen's confirm-write/review-readonly, unlike create-flow's single-target screens) must use a condensed field=value display to stay within budget — §3's parity rubric explicitly permits this as a 'MAY differ' spacing/layout difference, not a content-presence divergence"
    - "A brand-new managed file (the git fragment) and an append to an existing file with prior content (~/.gitconfig's includeIf block) are both still shown sentinel-wrapped in the TUI confirm-write preview, matching the HTML mockup — T-02-CONT's containment mitigation applies uniformly regardless of whether the target file is new or pre-existing"

key-files:
  created:
    - .planning/design/git-screen/FIELDS.md
    - .planning/design/git-screen/manifest.json
    - .planning/design/git-screen/parity.json
    - .planning/design/git-screen/CRITIQUE.md
    - .planning/design/git-screen/html/*.png (7 files)
    - .planning/design/git-screen/tui/*.png (7 files)
    - .planning/design/dummy-nav-frames/dummy-nav-git-screen-*.txt (7 files, e2e evidence)
    - .planning/design/mockup-src/src/routes/git-screen/git-form-empty.route.tsx
    - .planning/design/mockup-src/src/routes/git-screen/git-form-filled.route.tsx
    - .planning/design/mockup-src/src/routes/git-screen/match-strategy-select.route.tsx
    - .planning/design/mockup-src/src/routes/git-screen/review-readonly.route.tsx
    - .planning/design/mockup-src/src/routes/git-screen/confirm-write.route.tsx
    - .planning/design/mockup-src/src/routes/git-screen/backup-notice.route.tsx
    - .planning/design/mockup-src/src/routes/git-screen/result-success.route.tsx
    - internal/dummytui/surface_gitscreen.go
    - internal/dummytui/surface_gitscreen_test.go
  modified:
    - .planning/design/mockup-src/src/data/recipeFixtures.ts
    - .planning/design/dummy-nav-frames/identity-manager-entry.txt (regenerated evidence, not semantically modified)

key-decisions:
  - "git-screen's LaunchKey is 'g' (from identity-manager), matching the single-authority key-allocation table in 02-UX-DIRECTION.md §2 / internal/dummytui/doc.go (already pre-allocated there since 02-02, confirming that table's role as the single source of truth ahead of any surface's own implementation)"
  - "The fragment target file is ~/.gitconfig.d/<identity> (REQUIREMENTS.md GITUI-02, already marked built), NOT recipes/gitconfig.recipe's own ~/.gitconfig_<identity> literal naming that create-flow's pilot fixtures (includeIfHasconfigLine/includeIfGitdirLine/gitconfigFragmentPath) happen to reuse verbatim from the recipe. New, git-screen-only recipeFixtures.ts exports (gitScreenFragmentPath, gitScreenIncludeIf*Line, etc.) were added rather than editing the existing create-flow-owned exports, per this plan's explicit fan-out-isolation instruction and CLAUDE.md's 'surface any divergence between current behavior and the recipes explicitly'"
  - "The 7 screens form a strict linear chain (git-form-empty -> git-form-filled -> match-strategy-select -> review-readonly -> confirm-write -> backup-notice -> result-success), unlike create-flow's branching tree — git-screen has no demo/dead-end detours in its UX-DIRECTION.md §4(2) state list"
  - "confirm-write shows sentinels around BOTH the new fragment file (~/.gitconfig.d/personal) and the ~/.gitconfig append — even though only the latter has pre-existing content to visibly preserve — for consistency with STORE-04's general 'every mutation = idempotent sentinel-block rewrite' invariant and to keep the TUI/HTML content-presence parity exact, not just 'good enough'"
  - "DLV-01/DLV-02/DLV-05 are NOT marked complete in REQUIREMENTS.md despite this plan's frontmatter listing them, matching the 02-04 precedent: this plan ships the SECOND of seven surfaces; full-coverage completion is deferred to whichever later plan closes out Phase 2 (likely 02-11/02-12)"

requirements-completed: []  # DLV-01/DLV-02/DLV-05 phase-spanning — see key-decisions; this plan ships 2/7 surfaces, not full coverage

# Metrics
duration: ~50min
completed: 2026-07-03
---

# Phase 2 Plan 05: Git-Configuration Screen Fan-Out Surface (MUI Mockup + TUI Dummy + Capture + Parity) Summary

**The git-configuration screen — the second of the six fan-out surfaces — built as 7 recipe/requirements-accurate named states in both /mui v7 and the TUI dummy, captured as 7+7 PNGs, and driven through the launch mechanism (key `g` from Identities) on the real `cmd/gitid-dummy` binary with a 0-unresolved structured parity gate, including the GITUI-04 allowed_signers-byte-identical-to-user.email safety affordance and GITUI-03's default-gitdir match strategy.**

## Performance

- **Duration:** ~50 min (including initial pilot-pattern study of 02-04's artifacts)
- **Tasks:** 3 completed
- **Files modified:** 34 created/modified across the 3 task commits (11 + 2 + 25, see Task Commits)

## Accomplishments

- Authored `.planning/design/git-screen/FIELDS.md` (per-screen field/order/label table for all 7 named states from 02-UX-DIRECTION.md §4(2)) and `manifest.json` (7 hardened-schema entries — unique screen/signature, `keysFromHome` absolute from startup through the launch key `g`).
- Built all 7 `/mui` v7 route files under `src/routes/git-screen/` — the Git fragment form in empty/filled states (user.name/user.email/gpg.format=ssh/user.signingkey-as-path/commit.gpgsign, GITUI-02), the match-strategy picker (gitdir/hasconfig/both, default gitdir with a live `includeIf` preview, GITUI-03), the read-only review showing fragment+includeIf+allowed_signers together with the allowed_signers email shown side-by-side with user.email (GITUI-04), and the full four-beat mutation ceremony across all three files this screen writes (the fragment, `~/.gitconfig`, `~/.ssh/allowed_signers` — GITUI-05) — all pulling real copy from new git-screen-only `recipeFixtures.ts` exports.
- Built `internal/dummytui/surface_gitscreen.go`: a keyless modal surface (`LaunchFrom: "identity-manager"`, `LaunchKey: "g"`) with 7 `ScreenDef`s mirroring the mockup byte-for-byte on labels/copy/defaults, each embedding its manifest signature; the `ScreenDef.Keys` graph is a connected linear chain reachable from the `git-form-empty` entry screen, matching manifest.json's `keysFromHome` walk.
- Captured 7 HTML + 7 TUI PNGs (`TestCaptureAllMockupScreens/git-screen`) and proved every screen reachable on the REAL `cmd/gitid-dummy` binary through the `g` launch key with zero writes (`TestDummyNavReachesAllScreens/git-screen`).
- Ran the structured HTML↔TUI parity review against all 7 screenshot pairs (applying `agent-ui-ux-designer`'s methodology directly — see Issues Encountered) and closed `parity.json`'s 9 rows (the seven §3 dimensions + `allowed-signers-byte-identity` + `match-strategy-default-gitdir`) to `status: resolved`, 0 unresolved.
- Discovered and fixed a viewport-budget overflow the create-flow pilot never hit (see Deviations): git-screen's `review-readonly` and `confirm-write` screens preview content from up to three files at once, which initially exceeded the fixed PTY's 20-available-body-row budget and scrolled the signature marker off-screen, failing the surface-scoped e2e.

## Task Commits

Each task was committed atomically:

1. **Task 1: git-screen FIELDS.md + manifest.json + parity.json seed + MUI mockup (7 states)** - `fd06a6e` (feat)
2. **Task 2: git-screen TUI dummy surface (7 screens, keyless modal, launch binding)** - `e9dd86b` (feat)
3. **Task 3: Capture (both media) + parity critique -> 0 unresolved** - `837857a` (feat, includes the viewport-budget fix)

**Plan metadata:** pending (this commit, created after this SUMMARY)

## Files Created/Modified

- `.planning/design/git-screen/FIELDS.md` - per-screen field/label/order table for all 7 states
- `.planning/design/git-screen/manifest.json` - 7 hardened-schema `{surface, screen, htmlRoute, keysFromHome, signature}` entries
- `.planning/design/git-screen/parity.json` - 9 rows (7 §3 dimensions + 2 highest-risk-affordance rows), all `status: resolved`
- `.planning/design/git-screen/CRITIQUE.md` - aesthetic pass (0 findings) + structured parity findings log (1 non-blocking pre-existing observation carried from create-flow's own finding #1, 1 resolved layout-only divergence explained by the §3 "MAY differ" rubric)
- `.planning/design/git-screen/html/*.png` (7) / `.planning/design/git-screen/tui/*.png` (7) - captured screenshots
- `.planning/design/dummy-nav-frames/dummy-nav-git-screen-*.txt` (7) - PTY e2e evidence frames
- `.planning/design/mockup-src/src/routes/git-screen/*.route.tsx` (7) - the /mui mockup screens
- `.planning/design/mockup-src/src/data/recipeFixtures.ts` - extended with `gitScreenFragmentPath`, `gitScreenIncludeIf*Line`, `gitScreenMatchStrategyPreview`, `gitScreenConfirmTargets`, `gitScreenManagedFragmentText`, `gitScreenGitconfigIncludeBlockText`, `gitScreenAllowedSignersBackupPath`, `gitScreenResultSuccessMessage` (all new, additive exports)
- `internal/dummytui/surface_gitscreen.go` (+ `_test.go`) - the TUI dummy git-screen surface, 7 screens, keyless modal with a LaunchFrom/LaunchKey binding

## Decisions Made

See `key-decisions` in the frontmatter for the full rationale on: the LaunchKey allocation (`g`, from the single-authority table), the `~/.gitconfig.d/<identity>` fragment-path convention vs. the recipe's own literal naming, the linear 7-screen chain shape, why confirm-write sentinel-wraps both write targets, and why DLV-01/02/05 are not marked complete in REQUIREMENTS.md by this fan-out plan alone.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] Installed the `freeze` binary needed for TUI screenshot capture**
- **Found during:** Task 3, first `TestCaptureAllMockupScreens/git-screen` run
- **Issue:** The TUI capture subtest failed for all 7 screens with `freeze binary not found on PATH (run make setup-env): exec: "freeze": executable file not found in $PATH`. This session's shell PATH did not include `$(go env GOPATH)/bin` where dev tools live.
- **Fix:** Ran `go install github.com/charmbracelet/freeze@v0.2.2` — the exact pinned version `Makefile`'s `setup-env` target installs — rather than running the full `setup-env` (which also reinstalls golangci-lint/gosec/pre-commit/hooks, unnecessary side effects for this fix). This is a `go install` of a project-documented, version-pinned dev tool (not a slopsquatting risk under the package-manager-install exclusion in the deviation rules, since the exact module path + version is already named in the Makefile).
- **Verification:** `which freeze` resolves; `TestCaptureAllMockupScreens/git-screen` passes with all 14 PNGs captured.
- **Committed in:** N/A (local environment provisioning only, no repo file changed)

**2. [Rule 1 - Bug] Fixed the confirm-write/review-readonly signature-marker scroll-off caused by exceeding the modal's fixed viewport row budget**
- **Found during:** Task 3, first `TestDummyNavReachesAllScreens/git-screen` run
- **Issue:** The fixed 80x24 PTY terminal gives an open modal only `m.height - verticalMargin` = 20 available body rows (`model.go`, unchanged from the 02-04 pilot's own fix). `review-readonly`'s and `confirm-write`'s initial renders (each previewing content from up to three files — the fragment, the `~/.gitconfig` includeIf block, and the `allowed_signers` line, plus the ~90-column-wide `allowed_signers` line itself) totaled 26 and 30 body lines respectively, well over budget. `boundModalToViewport` (pre-existing, correct infrastructure) clipped both to the top 19 rows plus a `↓ more` indicator, scrolling the trailing signature marker out of the visible frame — `TestDummyNavReachesAllScreens/git-screen/review-readonly` and `/confirm-write` both failed with "breadcrumb+signature not reached". create-flow's pilot never hit this because none of its 12 screens preview more than one file's content at once.
- **Fix:** Condensed the fragment-field display on both screens to `field=value` lines (all the same fields/values/order the `/mui` mockup shows, just without the INI format's blank-line section spacers — explicitly a "MAY differ: exact spacing, pixel layout" divergence per 02-UX-DIRECTION.md §3, not a content-presence one) and word-wrapped the 90-column `allowed_signers` line onto two rows (`gsAllowedSignersLineDisplay`) so no single row exceeds the 72-column modal width. `confirm-write` was further trimmed (dropped a redundant explainer line) to land at exactly 20/20 body rows while STILL showing full `# BEGIN/END gitid managed:` sentinels around both the new fragment file and the `~/.gitconfig` append (T-02-CONT containment, matching the HTML mockup's presentation for both targets). `review-readonly` landed at 17/20 rows with margin to spare.
- **Files modified:** `internal/dummytui/surface_gitscreen.go`
- **Verification:** `TestDummyNavReachesAllScreens/git-screen` (all 7 subtests) passes; `TestCaptureAllMockupScreens/git-screen` (14 PNGs) passes; `go test -race ./internal/dummytui/... -run GitScreen` passes; `make lint` 0 issues; visually reviewed both `confirm-write.png`/`review-readonly.png` pairs (html+tui) to confirm the condensed TUI layout still shows every field/value/sentinel the HTML mockup shows.
- **Committed in:** `837857a` (Task 3)

---

**Total deviations:** 2 auto-fixed (1 blocking local-environment tool gap, 1 bug discovered by this surface's own higher content density than the pilot)
**Impact on plan:** Both were necessary for the plan's own acceptance criteria (`TestCaptureAllMockupScreens/git-screen` and `TestDummyNavReachesAllScreens/git-screen` both explicit Task 3 verify commands) to pass. The viewport-budget fix is git-screen-scoped (its own render functions only) — it does not touch the shared `model.go`/`overlay.go`/`registry.go` infrastructure the 02-04 pilot already fixed, and documents a reusable pattern (condensed field=value display for multi-file previews) later fan-out plans previewing more than one file can reuse if they hit the same budget.

## Issues Encountered

- **Task/subagent-dispatch tool unavailable in this executor's environment**, same limitation recorded in 02-01/02-02/02-03/02-04-SUMMARY.md. Task 3 calls for spawning `agent-ui-ux-designer` for two passes (an HTML-only aesthetic pass and the structured HTML↔TUI parity review). This executor's toolset was limited to `Read`/`Write`/`Edit`/`Bash` — no way to spawn a fresh-context subagent. In its place, this executor applied `agent-ui-ux-designer`'s documented methodology (F-pattern/left-side bias, Fitts's/Hick's Law, accessibility, distinctive typography) directly against all 14 captured screenshots and recorded the results in `CRITIQUE.md`, including a note that the one layout divergence found (finding #2, the condensed fragment display) is resolved under §3's explicit "MAY differ" rubric. **This does not substitute for a fresh-context `agent-ui-ux-designer` pass** — flagging explicitly so the phase-level `/gsd-code-review` and the external cross-vendor (Codex) review the execution loop runs are not skipped for this plan's content.
- The `superpowers:requesting-code-review` skill referenced by this plan's `<success_criteria>` was similarly unavailable for the same reason (no subagent-dispatch tool). Every task's `<acceptance_criteria>` was instead re-verified directly via its exact automated command (see Task Commits' verification notes and the Deviations section above) — all green, plus a full-repo `go build ./...`, `go test -race ./...`, and `make lint` pass beyond what the plan's own per-task verify commands required.

## User Setup Required

None — no external service configuration required. `freeze` (needed for `screenshot-tui`/`screenshot-tui-mockups`) was provisioned locally during Task 3 (see Deviations #1); everything else `make setup-env` already provisions (pinned Chromium, pnpm) was already present in this session.

## Next Phase Readiness

- The per-surface pipeline is now proven on TWO independent surfaces (create-flow's branching 12-screen tree, git-screen's linear 7-screen chain) — the remaining fan-out plans (02-06 identity-manager through 02-10) can follow either shape as their own UX-DIRECTION.md §4 manifest dictates.
- **New pattern for later fan-out plans to reuse:** if a screen previews content from MORE than one target file at once (as git-screen's `review-readonly`/`confirm-write` do), budget for the fixed 80x24 PTY's 20-available-body-row modal viewport (`model.go` `verticalMargin=4`) up front — use a condensed `field=value` display rather than full raw block text where the §3 rubric's "MAY differ: spacing/layout" clause allows it, and word-wrap any single line wider than 72 columns.
- `internal/dummytui/doc.go`'s key-allocation table already had `g` (git-screen) pre-allocated since 02-02 and is now actually claimed in code, matching the table exactly — no doc.go edit was needed or made.
- Outstanding, not blocking this plan: two fresh-context reviews this session's toolset could not run (`agent-ui-ux-designer` subagent pass, `superpowers:requesting-code-review`) — recommend the orchestrator run both before or alongside the phase-level review gate, as recorded in Issues Encountered.
- DLV-01/DLV-02/DLV-05 remain incomplete in REQUIREMENTS.md pending the remaining 5 surfaces (02-06..02-10) — do not mark them complete on any single fan-out plan; the closing plan (likely 02-11/02-12) is where full-coverage completion should be recorded.

---
*Phase: 02-design-all-mockups-checkpoint-1*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 12 spot-checked created/modified files verified present on disk (`FIELDS.md`,
`manifest.json`, `parity.json`, `CRITIQUE.md`, `html/git-form-empty.png`,
`tui/git-form-empty.png`, `git-form-empty.route.tsx`, `result-success.route.tsx`,
`surface_gitscreen.go`, `surface_gitscreen_test.go`, `recipeFixtures.ts`, this
SUMMARY — 12/12 FOUND). All 3 task commit hashes (`fd06a6e`, `e9dd86b`, `837857a`)
verified present in `git log --oneline --all`. `go build ./...`,
`go test -race ./...`, `make lint`, `pnpm exec tsc --noEmit`, `pnpm build`,
`TestCaptureAllMockupScreens/git-screen` (screenshot tag), and
`TestDummyNavReachesAllScreens/git-screen` (e2e tag) all pass with zero issues
at the time this summary was written.
