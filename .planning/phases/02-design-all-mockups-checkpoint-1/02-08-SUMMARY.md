---
phase: 02-design-all-mockups-checkpoint-1
plan: 08
subsystem: ui
tags: [react, mui, bubbletea-v2, lipgloss-v2, go, design-mockup, tui-dummy, screenshot, e2e, fan-out-surface, global-git, baseline-config, advisory]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 01
    provides: "the shared four-region MUI shell + recipeFixtures.ts (including the pre-existing globalGitDefaults/globalGitDefaultsBlockText fixture this plan builds directly on) + route auto-discovery this plan's 6 global-git routes rely on"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 02
    provides: "internal/dummytui's Register/RegisterOrReplace registry and the 02-02 global-git placeholder (key 3, screen \"entry\") this plan replaces"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 03
    provides: "the hardened manifest.json schema/loader, design_capture_test.go's manifest-driven capture, and dummy_nav_e2e_test.go's manifest-driven PTY walker"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 04
    provides: "the proven per-surface pipeline (FIELDS -> manifest -> parity seed -> mockup -> dummy -> capture -> critique -> parity 0-unresolved) this plan replicates verbatim"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 07
    provides: "the closest sibling surface: the 6-state options-list/option-detail/fix-preview/confirm-write/backup-notice/result-applied shape, the v/f/w/y/z key chain, the advisory-not-blocking visual language, and the git-screen-precedented gsFieldsCompactLine viewport-compaction technique this plan reuses directly for its own fix-preview/confirm-write compaction"
provides:
  - "The Global Git options screen (view 3, owned later by Phase 7) as 6 named states in BOTH media: /mui v7 routes under src/routes/global-git/*.route.tsx and internal/dummytui/surface_globalgit.go, replacing the 02-02 placeholder as the SOLE owner of ActivationKey \"3\" via RegisterOrReplace"
  - ".planning/design/global-git/{FIELDS.md, manifest.json, parity.json, CRITIQUE.md, html/*.png (6), tui/*.png (6)} — the fifth complete per-surface pipeline artifact set"
  - "recipeFixtures.ts extended with global-git-only exports — the GGIT-01 11-option baseline set (interpolating the existing globalGitDefaults fixture directly for every overlapping value, never duplicated), init.defaultBranch's main-vs-master contractual explanation, the sentinel-wrapped full managed-block text, and the global-user.email-is-never-written (D-04b) affordance"
  - "The global-git surface's intra-flow keys (v/f/w/y/z) reachable on the real cmd/gitid-dummy binary from its own entry screen, proven by the surface-scoped dummy-nav e2e"
affects: [02-09, 02-10, 02-11, 02-12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Replicated the create-flow/git-screen/identity-manager/global-ssh per-surface pipeline exactly (FIELDS.md -> manifest.json -> parity.json seed -> /mui mockup -> dummytui surface -> capture -> critique -> parity 0-unresolved) on a FIFTH surface — the third number-key ActivationKey surface after identity-manager/global-ssh, confirming RegisterOrReplace's placeholder-replacement pattern generalizes cleanly to key 3"
    - "GGIT-01's 11-option baseline + recipe-defaults set is built by INTERPOLATING the pre-existing, unmodified globalGitDefaults fixture (02-01/02-02) directly wherever a value overlaps (init.defaultBranch, core.ignorecase, push.autoSetupRemote, pull.rebase, fetch.prune, merge.conflictstyle, diff.colorMoved), rather than duplicating those values as new constants — a single source of truth across two design phases' worth of fixture authoring; only the fields the shared fixture does not cover (core.autocrlf/eol, global user.email, alias, color) are new exports"
    - "Advisory-never-blocking (§5's 'the TWO Global-options surfaces' rule, shared with global-ssh) reused verbatim: the same yellow ! glyph, the same 'Recommended, not required' banner text, applied to a DIFFERENT kind of highest-risk affordance (managed-block containment, GGIT-01, rather than global-ssh's per-option decline) — proving the advisory visual language generalizes across affordance types, not just across screens of the same affordance"
    - "Global user.email is modeled as read/explained-but-never-written, matching the REAL backend's own documented behavior (internal/gitconfig/baseline.go's RenderBaselineBlock: 'No user section is ever emitted (D-04b)') rather than inventing a write gitid's Phase 7 implementation will never perform — verified against the actual shipped Go source, not just the recipe"
    - "A SECOND live-PTY-viewport TUI compaction discovery (git-screen's gsFieldsCompactLine precedent, global-ssh's options-list precedent, now applied to fix-preview/confirm-write instead of options-list): the original per-section, per-key full-block TUI render (30 logical lines for 11 options + 8 aliases) overflowed the real, fixed 80x24 terminal on the FIRST e2e attempt for BOTH fix-preview and confirm-write; compacted to 5 grouped key=value lines, keeping the full value set while moving the full literal block to the HTML mockup only"
key-files:
  created:
    - .planning/design/global-git/FIELDS.md
    - .planning/design/global-git/manifest.json
    - .planning/design/global-git/parity.json
    - .planning/design/global-git/CRITIQUE.md
    - .planning/design/global-git/html/*.png (6 files)
    - .planning/design/global-git/tui/*.png (6 files)
    - .planning/design/dummy-nav-frames/dummy-nav-global-git-*.txt (6 files, e2e evidence)
    - .planning/design/mockup-src/src/routes/global-git/options-list.route.tsx
    - .planning/design/mockup-src/src/routes/global-git/option-detail.route.tsx
    - .planning/design/mockup-src/src/routes/global-git/fix-preview.route.tsx
    - .planning/design/mockup-src/src/routes/global-git/confirm-write.route.tsx
    - .planning/design/mockup-src/src/routes/global-git/backup-notice.route.tsx
    - .planning/design/mockup-src/src/routes/global-git/result-applied.route.tsx
    - internal/dummytui/surface_globalgit.go
    - internal/dummytui/surface_globalgit_test.go
  modified:
    - .planning/design/mockup-src/src/data/recipeFixtures.ts (additive global-git section)
key-decisions:
  - "GGIT-01's baseline+recipe-defaults set is pinned to the exact 11-row order §4.5 specifies (init.defaultBranch, core.ignorecase, core.autocrlf/eol, global user.email, push.autoSetupRemote, pull.rebase, fetch.prune, alias, color, merge.conflictstyle, diff.colorMoved), with 10 rows needsAction=true (nothing written yet) and 1 (user.email) needsAction=false/informational — a deliberate contrast so options-list demonstrates both states, mirroring global-ssh's own already-fine/needs-action mix"
  - "option-detail targets init.defaultBranch (the option carrying the dedicated main-vs-master highlight) with the full contractual explanation, mirroring global-ssh's detail-IdentitiesOnly single-target precedent — every other option's rationale is still explained via its options-list one-liner (HTML) / TUI current-recommended compact row"
  - "Global user.email is deliberately modeled as NOT written by gitid (needsAction=false, framed as 'informational only' rather than a 4th declined recommendation) because that matches the REAL backend's documented behavior (D-04b, internal/gitconfig/baseline.go) — the mockup does not invent a write the Phase 7 implementation will never perform; this affordance is carried through fix-preview/confirm-write/result-applied exactly like global-ssh's declined ForwardAgent, but framed as a structural rule rather than a user choice"
  - "fix-preview/confirm-write/backup-notice/result-applied all demonstrate the SAME concrete scenario (10 of 10 baseline options applied in one batch write, user.email intentionally absent) rather than a per-option selective-apply flow like global-ssh — because GGIT-01's own highest-risk affordance (managed-block containment) is about preserving content OUTSIDE the block, not about selective application within it"
  - "global-git's ScreenDef.Keys reuse the exact SAME letters (v/f/w/y/z) as global-ssh's own linear ceremony chain — safe because ScreenDef.Keys is scoped per top-level surface and only one primary surface is ever active at a time; no registry collision, verified by TestGlobalGit_IntraSurfaceKeysNeverReuseLaunchKeysNOrG and the registration-time collision guard"
  - "fix-preview/confirm-write's TUI render is a 5-line grouped-key=value compaction (git-screen's gsFieldsCompactLine1/2/3 precedent) rather than the full per-section literal block — a Rule 1 auto-fix discovered when the FIRST e2e attempt failed both subtests outright (the original render never reached the breadcrumb/signature marker within the 24-row PTY); the sentinel lines and every option's value are still present, byte-identical, just grouped"
  - "DLV-01/DLV-02/DLV-05 are NOT marked complete in REQUIREMENTS.md, matching the 02-04/02-05/02-06/02-07 precedent: this plan ships the FIFTH of seven surfaces; full-coverage completion is deferred to whichever later plan closes out Phase 2 (likely 02-11/02-12)"

requirements-completed: []  # DLV-01/DLV-02/DLV-05 phase-spanning — see key-decisions; this plan ships 5/7 surfaces, not full coverage

# Metrics
duration: ~70min
completed: 2026-07-03
---

# Phase 2 Plan 08: Global Git Options Fan-Out Surface (MUI Mockup + TUI Dummy + Capture + Parity) Summary

**The Global Git options screen — an explained, advisory review of the 11 GGIT-01 baseline + recipe-default git config options, with a dedicated main-vs-master highlight on `init.defaultBranch` — built as 6 recipe-accurate named states in both /mui v7 and the TUI dummy, replacing the 02-02 placeholder as the sole owner of number key `3` via `RegisterOrReplace`, captured as 6+6 PNGs, and driven through the running `cmd/gitid-dummy` binary with a 0-unresolved structured parity gate — including a concrete demonstration that writes preserve content outside the managed block verbatim, with global `user.email` explicitly modeled as never-written (matching the real backend's own D-04b rule).**

## Performance

- **Duration:** ~70 min
- **Tasks:** 3 completed
- **Files modified:** 22 files across the 3 task commits (10 + 2 + 22, with `FIELDS.md`, `parity.json`, and `surface_globalgit.go` touched in both Task 1/2 and Task 3 — see Task Commits)

## Accomplishments

- Authored `.planning/design/global-git/FIELDS.md` (per-screen field/order/label table for all 6 named states from 02-UX-DIRECTION.md §4.5) and `manifest.json` (6 hardened-schema entries — unique screen/signature, `keysFromHome` absolute from the number-key `3` entry).
- Pinned GGIT-01's 11-option baseline + recipe-defaults set in `recipeFixtures.ts`, interpolating the pre-existing `globalGitDefaults` fixture (unmodified) directly for every overlapping value (init.defaultBranch, core.ignorecase, push.autoSetupRemote, pull.rebase, fetch.prune, merge.conflictstyle, diff.colorMoved) and adding new exports only for the fields it doesn't cover (core.autocrlf/eol, global user.email, alias, color).
- Built all 6 `/mui` v7 route files under `src/routes/global-git/` — the master-detail archetype for `options-list` (11 options as glyph+word rows, `init.defaultBranch` carrying a dedicated "main vs master" chip), `option-detail` (init.defaultBranch's full contractual explanation), `fix-preview` (the exact diff, 10 of 10 applied, user.email explicitly absent), `confirm-write` (sentinel-visible managed-block text targeting `~/.gitconfig`), `backup-notice`, `result-applied` — all pulling real copy from new global-git-only `recipeFixtures.ts` exports.
- Built `internal/dummytui/surface_globalgit.go`: the number-key `3` primary surface, registered via `RegisterOrReplace` to cleanly replace the 02-02 `data.go` placeholder (no edit to `data.go`/`model.go`), with 6 `ScreenDef`s mirroring the mockup byte-for-byte on labels/copy/defaults.
- Captured 6 HTML + 6 TUI PNGs (`TestCaptureAllMockupScreens/global-git`) and proved every screen reachable on the REAL `cmd/gitid-dummy` binary with zero writes (`TestDummyNavReachesAllScreens/global-git`), after fixing a viewport-overflow bug discovered on the first e2e run for BOTH `fix-preview` and `confirm-write` (see Deviations).
- Ran the structured HTML↔TUI parity review against all 6 screenshot pairs (applying `agent-ui-ux-designer`'s methodology directly — see Issues Encountered) and closed `parity.json`'s 9 rows (the seven §3 dimensions + `main-vs-master-highlight` + `managed-block-containment`) to `status: resolved`, 0 unresolved.
- Re-verified the no-backend DLV-05 ALLOWLIST and the no-shared-file-edit invariant (`data.go`/`model.go`/`App.tsx`/`package.json`/`pnpm-lock.yaml`/`Makefile` all untouched by this plan's diff) after every task.

## Task Commits

Each task was committed atomically:

1. **Task 1: global-git FIELDS.md + manifest.json (hardened) + parity.json seed + MUI mockup (6 states)** - `d84441e` (feat)
2. **Task 2: global-git TUI dummy surface (6 screens, RegisterOrReplace key 3, backend-free)** - `f2061e4` (feat)
3. **Task 3: Capture (both media) + parity critique -> 0 unresolved + fix-preview/confirm-write viewport-overflow fix** - `56334c3` (feat, includes the compaction fix)

**Plan metadata:** pending (this commit, created after this SUMMARY)

## Files Created/Modified

- `.planning/design/global-git/FIELDS.md` - per-screen field/label/order table for all 6 states
- `.planning/design/global-git/manifest.json` - 6 hardened-schema `{surface, screen, htmlRoute, keysFromHome, signature}` entries
- `.planning/design/global-git/parity.json` - 9 rows (7 §3 dimensions + 2 highest-risk-affordance rows), all `status: resolved`
- `.planning/design/global-git/CRITIQUE.md` - aesthetic pass (0 findings) + structured parity findings log (5 findings, all resolved — 2 the same class as prior surfaces' shared-infrastructure findings, 1 the viewport compaction, 2 the surface's own highest-risk-affordance rows)
- `.planning/design/global-git/html/*.png` (6) / `.planning/design/global-git/tui/*.png` (6) - captured screenshots
- `.planning/design/dummy-nav-frames/dummy-nav-global-git-*.txt` (6) - PTY e2e evidence frames
- `.planning/design/mockup-src/src/routes/global-git/*.route.tsx` (6) - the /mui mockup screens
- `.planning/design/mockup-src/src/data/recipeFixtures.ts` - extended with `GlobalGitOption`, `globalGitOptions` (11 fixtures), `globalGitDetailTarget`, `globalGitDetailExplanation`, `globalGitAdvisoryNote`, `globalGitTargetFile`, `globalGitManagedBlockSentinels`, `globalGitAliases`, `globalGitColorSettings`, `globalGitAutocrlf`/`globalGitEol`, `globalGitFullManagedBlockText`, `globalGitFixPreviewLines`, `globalGitResultMessage` (all new, additive exports)
- `internal/dummytui/surface_globalgit.go` (+ `_test.go`) - the TUI dummy global-git surface, 6 screens, `RegisterOrReplace`d as the sole owner of key `3`

## Decisions Made

See `key-decisions` in the frontmatter for the full rationale on: the GGIT-01 option-set pinning and its 10-needsAction/1-informational split, why `option-detail` targets `init.defaultBranch`, why global `user.email` is modeled as never-written rather than a declined recommendation, the batch-apply (10 of 10) scenario versus global-ssh's selective-apply scenario, the v/f/w/y/z key reuse across surfaces, the fix-preview/confirm-write viewport compaction, and why DLV-01/02/05 are not marked complete in REQUIREMENTS.md by this fan-out plan alone.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed fix-preview/confirm-write's TUI render overflowing the real 80x24 live PTY viewport**
- **Found during:** Task 3, first `TestDummyNavReachesAllScreens/global-git` run
- **Issue:** The initial `renderGGITFixPreview`/`renderGGITConfirmWrite` rendered the full per-section, per-key managed-block text literally (mirroring the `/mui` mockup's `globalGitFullManagedBlockText` HTML `<pre>` block) — roughly 20-30 logical lines for 11 options plus 8 aliases across 7 `[section]` headers, on top of the 2-line banner and heading. `cmd/gitid-dummy` runs inside a REAL, fixed 80x24 PTY with no scroll region (`e2e/ui_pty_e2e_test.go`'s `ptyTermWidth`/`ptyTermHeight`), unlike the static `RenderScreen()`→`freeze` capture path, which has no height limit — so the static TUI capture (Task 1/2's own `go test -race ./internal/dummytui/... -run GlobalGit`) passed cleanly while the live e2e walk failed both `fix-preview` and `confirm-write` subtests: the frames captured at the 5-second timeout each showed the header, banner, and roughly the first 15 lines of the block, cut off mid-section, with neither the sentinel-end line nor the trailing manifest signature ever reaching the visible screen.
- **Fix:** Compacted both screens' body to 5 grouped `key=value` lines (`ggitCompactValueLines`: `defaultBranch=main ignorecase=false autocrlf=input eol=lf`, `autoSetupRemote=true rebase=true prune=true`, `color: ui=auto branch=auto diff=auto status=auto`, `conflictstyle=diff3 colorMoved=zebra`, `alias (8): st,co,br,ci,df,lg,unstage,last`), keeping the literal `# BEGIN/END gitid managed: global-git` sentinel lines around them on `confirm-write` and the `+` diff-line prefix on `fix-preview`. This is the same class of live-PTY-viewport compaction `git-screen`'s `gsFieldsCompactLine1/2/3` established for `review-readonly`/`confirm-write` and `global-ssh`'s `options-list` one-line-per-option compaction already established for a different screen shape — the field SET/VALUES (all 10 written options, all 8 aliases, all 4 color settings) are unchanged; only the layout is grouped, and the full literal block remains fully present in the HTML mockup.
- **Verification:** `go test -tags e2e -race -run 'TestDummyNavReachesAllScreens/global-git' ./e2e/...` passes (6/6 subtests). `go test -race ./internal/dummytui/... -run GlobalGit` (8/8 subtests, including the new `TestGlobalGit_ManagedBlockContainmentShown`) and `go test -tags screenshot -run 'TestCaptureAllMockupScreens/global-git' ./internal/screenshot/...` (freeze capture, unaffected by the fix but re-run to confirm) both still pass. `make lint` clean (0 issues) after the fix removed the now-unused long-form Go constants (`ggitFullManagedBlockText`, `ggitFixPreviewLines`).
- **Files modified:** `internal/dummytui/surface_globalgit.go`
- **Committed in:** `56334c3` (Task 3)
- **Documented as an accepted divergence in:** `FIELDS.md` rows 2/3 (`fix-preview`/`confirm-write`) and `CRITIQUE.md` finding #2

**2. [Rule 3 - Blocking issue] Provisioned `freeze` and `go env GOPATH/bin` onto PATH for this session**
- **Found during:** Task 3, first capture attempt
- **Issue:** This session's shell PATH did not include `$(go env GOPATH)/bin` (where `freeze` was already installed from a prior session's `make setup-env`) by default, matching 02-06's and 02-07's own equivalent deviation.
- **Fix:** Exported `PATH="$HOME/go/bin:$PATH"` for the Bash calls that needed it (screenshot capture, e2e) — no repo file changed, no reinstall performed (the pinned `freeze` was already present from prior sessions).
- **Verification:** `which freeze` resolves; `TestCaptureAllMockupScreens/global-git` passes with all 12 PNGs captured.
- **Committed in:** N/A (local environment/session PATH only, no repo file changed)

---

**Total deviations:** 2 auto-fixed (1 Rule-1 TUI-viewport bug fix required by this plan's own Task 3 acceptance criteria, 1 local session PATH provisioning)
**Impact on plan:** The viewport-overflow fix was necessary for this plan's own Task 3 acceptance criteria (`TestDummyNavReachesAllScreens/global-git`, an explicit verify command) to pass, and confirms 02-07's own "load-bearing reminder for 02-08" prediction: any list-shaped or long-block-rendering screen must budget its TUI content against the real 80x24 live-PTY viewport from the start. The remaining fan-out plans (02-09 health, 02-10 fixer — both list-shaped per 02-UX-DIRECTION.md §2) should treat this as confirmed, not hypothetical: TWO of the last two fan-out surfaces (global-ssh's `options-list`, global-git's `fix-preview`+`confirm-write`) have now hit this exact class of bug on their FIRST e2e attempt.

## Issues Encountered

- **Task/subagent-dispatch tool unavailable in this executor's environment**, same limitation recorded in 02-01 through 02-07's SUMMARY.md files. Task 3 calls for spawning `agent-ui-ux-designer` for two passes (an HTML-only aesthetic pass and the structured HTML↔TUI parity review). This executor's toolset was limited to `Read`/`Write`/`Edit`/`Bash` — no way to spawn a fresh-context subagent. In its place, this executor applied `agent-ui-ux-designer`'s documented methodology (F-pattern/left-side bias, Fitts's/Hick's Law, accessibility, distinctive typography) directly against all 12 captured screenshots and recorded the results in `CRITIQUE.md`. **This does not substitute for a fresh-context `agent-ui-ux-designer` pass** — flagging explicitly so the phase-level `/gsd-code-review` and the external cross-vendor (Codex) review the execution loop runs are not skipped for this plan's content.
- The `superpowers:requesting-code-review` skill referenced by this plan's `<success_criteria>` was similarly unavailable for the same reason (no subagent-dispatch tool). Every task's `<acceptance_criteria>` was instead re-verified directly via its exact automated command (see Task Commits' verification notes and the Deviations section above) — all green, plus a full-repo `go build ./...`, `go test -race ./internal/dummytui/...`, and `make lint` pass beyond what the plan's own per-task verify commands required, and a plan-scoped `git diff` proof that `data.go`/`model.go`/`App.tsx`/`package.json`/`pnpm-lock.yaml`/`Makefile` were never touched by this plan.
- One design ambiguity resolved without a checkpoint (Rule 2-adjacent, judgment call within scope): the recipe's commented example (`recipes/gitconfig.recipe`) shows `merge.conflictstyle = diff3`, matching the pre-existing `globalGitDefaults` fixture, while the REAL shipped backend (`internal/gitconfig/baseline.go`'s `DefaultBaselineConfig`) has since chosen `zdiff3` (a C4 git-version-gated Tier-2 default). Per the plan's own explicit instruction ("Values MUST match recipes/gitconfig.recipe + the existing globalGitDefaults fixture + the shipped gitconfig.DefaultURLRewrites/baseline"), and since directly reusing the existing, unmodified fixture (rather than inventing a parallel, drifting value) was both instructions' shared intent, this plan interpolates `globalGitDefaults.mergeConflictstyle` (`diff3`) rather than hardcoding `zdiff3` — keeping a single source of truth with the established fixture. Flagging this recipe-vs-shipped-backend divergence explicitly for whoever implements the real Phase 7 feature, since it predates this plan and is out of this plan's scope to resolve.

## User Setup Required

None — no external service configuration required. `freeze`/`pnpm` were already provisioned from prior sessions; only this session's shell `PATH` needed adjusting (see Deviations #2).

## Next Phase Readiness

- The per-surface pipeline is now proven on FIVE independent surfaces: create-flow's branching 12-screen tree, git-screen's linear 7-screen chain, identity-manager's number-key nav-root with 5 modals, global-ssh's number-key master-detail + linear ceremony chain, and global-git's number-key master-detail (11-row list) + linear ceremony chain — the remaining fan-out plans (02-09 health, 02-10 fixer) can follow whichever shape their own UX-DIRECTION.md §4 manifest dictates, and should expect the SAME `RegisterOrReplace`-replaces-a-placeholder pattern for their own number keys (4/5).
- **Load-bearing reminder for 02-09/02-10 (now confirmed twice, not hypothetical):** any list-shaped screen or any screen rendering a multi-section literal config block must budget its TUI content against the REAL 80x24 live-PTY viewport from the start (git-screen's compact-line precedent, global-ssh's options-list fix, and this plan's own fix-preview/confirm-write fix) rather than discovering the overflow at e2e time — the static `RenderScreen()`→`freeze` capture path has no height limit and will NOT catch this class of bug; only the PTY-driven e2e walk will. `health-with-findings` (02-09) and `fixer-list` (02-10) are both explicitly list-shaped per 02-UX-DIRECTION.md §4.6/§4.7 and should design their TUI compaction UP FRONT.
- `internal/dummytui/doc.go`'s key-allocation table already had `3` pre-allocated to global-git since 02-02's initial table authoring and is now actually claimed in code, matching the table exactly — no `doc.go` edit was needed or made.
- Outstanding, not blocking this plan: two fresh-context reviews this session's toolset could not run (`agent-ui-ux-designer` subagent pass, `superpowers:requesting-code-review`) — recommend the orchestrator run both before or alongside the phase-level review gate, as recorded in Issues Encountered.
- DLV-01/DLV-02/DLV-05 remain incomplete in REQUIREMENTS.md pending the remaining 2 surfaces (02-09/02-10) — do not mark them complete on any single fan-out plan; the closing plan (likely 02-11/02-12) is where full-coverage completion should be recorded.

---
*Phase: 02-design-all-mockups-checkpoint-1*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 12 spot-checked created/modified files verified present on disk (`FIELDS.md`,
`manifest.json`, `parity.json`, `CRITIQUE.md`, `html/options-list.png`,
`tui/options-list.png`, `options-list.route.tsx`, `result-applied.route.tsx`,
`surface_globalgit.go`, `surface_globalgit_test.go`, `recipeFixtures.ts`, this
SUMMARY — 12/12 FOUND). All 3 task commit hashes (`d84441e`, `f2061e4`,
`56334c3`) verified present in `git log --oneline --all`. `go build ./...`,
`go test -race ./internal/dummytui/...`, `make lint`, `pnpm exec tsc --noEmit`,
`pnpm build`, `TestCaptureAllMockupScreens/global-git` (screenshot tag), and
`TestDummyNavReachesAllScreens/global-git` (e2e tag) all pass with zero issues
at the time this summary was written.
