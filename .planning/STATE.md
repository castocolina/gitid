---
gsd_state_version: "1.0"
milestone: v1.0
milestone_name: TUI-First Redesign
current_phase: 10
current_phase_name: READY TO EXECUTE; urgent TUI-consistency fixes queued as Phase 9.6 and 9.7
status: executing
stopped_at: Phase 10 code-complete, human_needed (Bazzite manual UAT); quick task 260922-brh completed; Phase 9.8 (app-wide TUI key model + field consistency + defaults-warning root causes) agreed with the user, not yet inserted
last_updated: "2026-09-22T00:00:00.000Z"
last_activity: 2026-09-22
last_activity_desc: Completed quick task 260922-brh — fixed red CI (SSH Include home leak, stale diff3 test, fedora gcc). Previously, quick task 260921-t6g repinned goreleaser to v2.17.0 to fix CI/Nightly (red since 2026-09-05, goreleaser v2.18.0 requires go>=1.27.0 vs Makefile's pinned GOTOOLCHAIN=go1.26.4), added a regression-guard test. NOTE (2026-09-21): this frontmatter and the narrative body below were found stale on session resume — Phase 9.6 and 9.7 are actually Complete (verified via ROADMAP.md + phase artifacts), and Phase 10 is code/test/review-complete but never formally marked complete (blocked only on the human-only Bazzite hardware UAT). A full STATE.md reconciliation pass is still owed.
state_head: "0bd017bbfa74ada120b6aa83d3b4442a567e2474"
progress:
  total_phases: 17
  completed_phases: 14
  total_plans: 105
  completed_plans: 105
  percent: 82
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-02)

**Core value:** Managing a Git identity produces coordinated, coherent SSH + Git artifacts that are proven to authenticate and resolve correctly (`ssh -G`) before any file is written, and existing hand-written config is never corrupted.
**Current focus:** Phase 09.5 — Full SSH/Git Properties Browser & Custom Key Entry

## Current Position

Phase: 10 (linux-validation-release-pipeline) — READY TO EXECUTE; urgent TUI-consistency fixes queued as Phase 9.6 and 9.7
Status: Ready to execute
Last activity: 2026-09-07 - Inserted Phase 9.6 (Global Git/SSH Options Consistency Fixes) and Phase 9.7 (New-Identity Wizard Consistency Fixes) after a user UX audit of Options screens and the New-Identity wizard

### Phase 8 (COMPLETE) — historical record

Phase: 08 (Health + Fixer) — COMPLETE (8 plans/waves; HLTH-01..06, FIX-01, FIX-02, DLV-04, DLV-06 closed in REQUIREMENTS.md)
Status: All 9 Per-Phase Checklist items evidenced — see `.planning/phases/08-health-fixer/{08-CONTEXT.md, 08-UI-SPEC.md, 08-REVIEWS.md (pre-execution cross-AI review + appended post-close HIGH-concerns resolution), 08-0{1..8}-SUMMARY.md, 08-08-REVIEWS.md (UX review packet), 08-REVIEW.md (code review), 08-VERIFICATION.md, 08-UI-REVIEW.md}`.
Waves 1-8 delivered Health + Fixer screens, doctor/fix wiring, real batch-walk-with-rollback semantics (D-16), CLI parity (`doctor`/`doctor --fix`, JSON envelope, tiered exit codes), raw-keystroke PTY e2e per screen state, and visual-regression gate registration. Two NEW cross-AI executor failure modes were hit and hand-recovered during Wave 8 (a stall, an unbounded 40+min zero-write exploration loop) — see `08-08-SUMMARY.md`.
Closeout (this session, worked directly on `gsd/phase-08-health-fixer`, no separate worktree): code review found 1 Critical (CR-01: the Fixer/Health TUI's fixability signal was `SuggestedFix != ""` instead of `Fix != nil`, letting report-only findings show a fake success ceremony with a fabricated backup path — fixed via a new `Fixable bool` field threaded from `doctor.Finding.Fix != nil`) and 1 actionable Warning (WR-01: a single non-batch fix failure rendered a nonsensical "Fix N of M failed" banner — fixed, `haltBatch` now only builds that banner when `m.batch != nil`). verify-work found one gap (DLV-06: the D-16 mid-batch-halt-and-rollback path had no real-PTY coverage — closed via `TestHealthFixer_RealPTYFixerBatchWalkHalt`, which forces a genuine OS-level chmod failure with `chflags uchg` through the compiled binary). UI review: 21/24, no new regressions. `/gsd-audit-uat` found nothing Phase-8-scoped outstanding. All fixes committed as `cbb5279`; review artifacts + appended HIGH-concerns resolution as `6881319`. Full gate battery re-verified green after every fix: `go test -race` 2157 passed, `make lint` 0 issues, `make gate-visual-regression` PASS 129.5s, `make test-e2e` PASS 676s.

### Phase 7 (COMPLETE) — historical record

Phase: 07 (global-git-options) — COMPLETE (6 plans/waves; GGIT-01 closed in REQUIREMENTS.md)
Status: All 9 Per-Phase Checklist items evidenced — see .planning/phases/07-global-git-options/{07-CONTEXT.md, 07-UI-SPEC.md, 07-REVIEWS.md (0 cycle-1 HIGH remaining), 07-0{1..6}-SUMMARY.md, 07-VERIFICATION.md (passed, 9/9), 07-UI-REVIEW.md (21/24, both findings fixed), review-packet/MANIFEST.md}.
Last activity: 2026-08-28 — closeout checklist items 6-9 completed and their findings fixed (see commits a7a3112, 4f5f136, c185ba7, 540b6ed on gsd/phase-07-global-git-options).

### Phase 2 (COMPLETE) — historical record

Phase: 02 (design-all-mockups-checkpoint-1) — COMPLETE (all 15 plans done; ★ DLV-08 approval recorded)
Plan: Not started
Status: 02-15 (wave 8) operationalized the binding 02-DESIGN-DECISIONS-CHECKPOINT-2.md contract (D1–D9 + affordance audit) in BOTH demos, byte-for-byte: D1 single-row color-only fields (02-14's rounded box deleted), D2 always-expanded match-strategy/algorithm radios, D3 terminal-glyph checkbox/radio on the web, D4 bracketed main-nav format (`[N] Label`, moved off the wizard stepper) + a new ActiveNavDimmed/activeNavDimmed state + a top-level plain-arrow view switch, D5 the wizard stepper reverted to `Step n/4 · <label> ● ○ ○ ○`, D6 one-row git-step buttons, D7 ONE hoisted Shift+←/→ chord gate reaching every step including the previously-dead review ceremony (proven with a new raw-byte PTY e2e injecting real xterm CSI sequences), D8 click-to-focus on every form row, and D9 Global Git's user.email promoted to an editable, opt-in global-fallback field with its own dedicated write ceremony (a documented, scoped recipes/ divergence). 02-STYLE-SPEC.md + both FIELDS.md companions rewritten in lockstep; the full exit-gate battery is green (go test -race, the no-backend allowlist, the extended copy-freeze grep, make test/lint/test-e2e/gate-no-backend-files, pnpm typecheck+build) — see 02-15-SUMMARY.md. The two ORCHESTRATOR-run exit gates (a fresh agent-ui-ux-designer critique of both live demos + a fresh-context code review against 02-15's must_haves/acceptance_criteria) have since RUN and their findings (F1-F10 + one record-only item) are fixed — see 02-15-SUMMARY.md "Review findings resolution (post-plan fix pass)" and commits a335d80/f62c99e. Next is 02-12 (wave 9, the single DLV-08 approval checkpoint), unblocked.
Last activity: 2026-07-06 -- Completed 02-12 (★ DLV-08): user approval recorded as `**APPROVED:** 2026-07-06 by Pepe`; Phase 2 COMPLETE — the approved live demos + 02-REDESIGN-SPEC.md/02-STYLE-SPEC.md/02-DESIGN-DECISIONS-CHECKPOINT-2.md + per-surface FIELDS.md are the binding design reference; Phases 3-9 backend work is UNBLOCKED

Progress: [████████░░] 82% (30/32 plans complete — Phase 2: 15/15; Phase 3: 6/9 plans, Wave 5 of 6 IN PROGRESS — 03-06 not yet counted complete: Task 3's cross-AI review is still owed; Wave 6 (03-07) COMPLETE)

## Performance Metrics

**Velocity:** reset for v1.0 (prior POC velocity archived under 0.0.1).

- Total plans completed: 85
- Average duration: — min
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 7 | - | - |
| 02 | 16 | - | - |
| 03 | 15 | - | - |
| 4 | 4 | - | - |
| 5 | 9 | - | - |
| 6 | 7 | - | - |
| 07 | 0 | - | - |
| 08 | 8 | - | - |
| 9 | 8 | - | - |
| 10 | 0 | - | - |
| 09.3 | 2 | - | - |
| 09.4 | 4 | - | - |
| 09.5 | 5 | - | - |

*Updated after each plan completion*
| Phase 01-foundations-spikes-ci P01 | 15 | 2 tasks | 8 files |
| Phase 01 P02 | 25min | 2 tasks | 6 files |
| Phase 01-foundations-spikes-ci P03 | 35min | 3 tasks | 8 files |
| Phase 01-foundations-spikes-ci P04 | 25min | 3 tasks | 4 files |
| Phase 01-foundations-spikes-ci P05 | 55min | 3 tasks | 17 files |
| Phase 01-foundations-spikes-ci P06 | 35min | 2 tasks | 4 files |
| Phase 02 P01 | 40min | 3 tasks | 20 files |
| Phase 02 P02 | ~15min | 2 tasks | 10 files |
| Phase 02 P03 | 75min | 3 tasks | 9 files |
| Phase 02 P04 | 23min | 3 tasks | 40 files |
| Phase 02 P05 | ~50min | 3 tasks | 34 files |
| Phase 02 P06 | ~90min | 3 tasks | 47 files |
| Phase 02 P07 | 75min | 3 tasks | 21 files |
| Phase 02 P08 | ~70min | 3 tasks | 22 files |
| Phase 02 P09 | 45min | 3 tasks | 22 files |
| Phase 02 P10 | 70min | 3 tasks | 25 files |
| Phase 02 P11 | 30min | 2 tasks | 8 files |
| Phase 02 P13 | 65min | 3 tasks | 24 files |
| Phase 02 P14 | 100min | 3 tasks | 14 files |
| Phase 02 P15 | 180min | 3 tasks | 27 files |
| Phase 02 P12 | multi-session (checkpoint) | 1 task | 1 file |
| Phase 03-create-flow-backend P04 | ~1 session (Task 3) | 3 tasks | 14 files |
| Phase 03-create-flow-backend P05 | 1 session | 3 tasks | 9 files |
| Phase 03-create-flow-backend P06 | 1 session (2/3 tasks + packet half) | 3 tasks (2 complete, 1 partial) | 10 files |
| Phase 03-create-flow-backend P07 | 1 session | 3 tasks | 17 files |
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 03-create-flow-backend P07 | 120 | 3 tasks | 17 files |
| Phase 03 P10 | 53min | 2 tasks | 12 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- v1.0 (2026-07-02): Design-first, screenshot-verified delivery — HTML/`mui` mockup → TUI dummy → visual-regression gate; `agent-ui-ux-designer` + `/mui` on every UI task.
- v1.0 (2026-07-02): ONE human checkpoint = design approval (Phase 2); credential upload auto-runs when `gh`/`glab` authenticated + valid identity exists.
- v1.0 (2026-07-02): Algorithm picker (ed25519 default + rsa-4096), local-use, macOS/Linux variant-aware via local capability probing.
- v1.0 (2026-07-02): SSH storage dual — in-file blocks / gitid-owned `Include` file / adopt external (verified with real `ssh -G`).
- v1.0 (2026-07-02): Build CI/CD for macOS Intel/ARM + Linux (GitHub Actions) + CI gates on both OSes.
- [Phase 01-foundations-spikes-ci]: Injectable exec.CommandContext probe seam with a shrinkable probeTimeout var; EXPORTED BuildProbeDeps() constructor for cross-package real wiring — Closes the project's documented injected-seam wiring blindspot and satisfies the 01-06 e2e cross-package requirement
- [Phase 01-foundations-spikes-ci, plan 02]: Registry populated via init()+Register() calls rather than a map literal, so Register is a real testable extensibility point
- [Phase 01-foundations-spikes-ci, plan 02]: generateRSA4096 passes the *rsa.PrivateKey pointer directly (never dereferenced) to ssh.MarshalPrivateKey/NewPublicKey per RESEARCH Pitfall 7
- [Phase 01-foundations-spikes-ci, plan 02]: Catalog Implemented (build-time) and Available (runtime probe) are orthogonal AlgoInfo facts; Generatable() requires both so a registered-but-stubbed algorithm is never offered as generatable
- [Phase 01-foundations-spikes-ci]: config.d/*.config glob literal is CANONICAL in sshconfig/include.go, deliberately duplicated (not shared) by 01-04's identity/inventory.go to preserve Wave-1 DAG independence (MEDIUM #4 option b)
- [Phase 01-foundations-spikes-ci]: Migrate always validates ssh -G against the real ~/.ssh/config entry point; rollback treats an empty filewriter.Write backupPath as 'file did not pre-exist' (RemoveFile), not 'nothing to restore'
- [Phase 01-foundations-spikes-ci, plan 04]: A key used only for git commit signing (no SSH Host block reference) is bucketed key-used-both, not key-unused — the locked 8-label MGR-02 vocabulary has no dedicated git-signing-only key state
- [Phase 01-foundations-spikes-ci, plan 04]: ClassifyState precedence is structural-before-key (fragment-path-missing > git-only > incomplete > key-missing > key-unused > key-used-ssh-only > complete), documented as a contract on the function itself
- [Phase 01-foundations-spikes-ci, plan 04]: BuildInventoryDeps().ReadSSHConfig is Include-aware (globs+merges config.d/*.config), verified end-to-end against 01-03's identical canonical glob literal with no cross-file symbol coupling (D-11, MEDIUM #4 option b)
- [Phase 01-foundations-spikes-ci]: freeze renders a static View() golden via a bare positional file argument, not --execute 'cat golden' -- confirmed empirically that freeze reads raw ANSI escape codes with correct color from a plain file
- [Phase 01-foundations-spikes-ci]: D-04's 100x30 screenshot-tui geometry is the Bubble Tea View() terminal size (cols x rows), not a freeze pixel flag -- freeze auto-sizes its PNG to the fixed captured content
- [Phase 01-foundations-spikes-ci]: screenshot.ChromiumRevision re-pins go-rod's own launcher.RevisionDefault (1321438) as an explicit gitid constant so a future go-rod upgrade can never silently change the downloaded Chromium build
- [Phase 01-foundations-spikes-ci]: debug caps prints three sections (Capabilities, Algorithm Catalog, Identities) via dedicated print helpers taking only already-resolved data — no classification logic lives in cmd/gitid
- [Phase 01-foundations-spikes-ci]: runDebugCapsWithDeps is a testability seam distinct from runDebugCaps (which wires the real EXPORTED platform.BuildProbeDeps/identity.BuildInventoryDeps constructors), so the unit suite can assert the probe-error path propagates instead of being silently swallowed
- [Phase 01-foundations-spikes-ci]: debug caps e2e test uses a plain exec.Command harness (adopt_e2e_test.go pattern) rather than the raw-keystroke PTY harness — the command is non-interactive (prints and exits), so PTY emulation adds no additional proof of real wiring
- [Phase 02]: recipeFixtures.ts's sshIdentityAliasBlockText is a literal (not interpolated) so recipe-critical text (Port 443, IdentitiesOnly yes) is statically greppable
- [Phase 02]: verify-routes.mjs uses Node 22's built-in fs.globSync instead of adding a glob npm dependency (project's pinned Volta toolchain is Node 22.22.3)
- [Phase 02]: DLV-01/DLV-02 NOT marked complete in REQUIREMENTS.md yet — both are phase-spanning (every surface, all 12 plans) and this is only Wave 1's foundation plan (1/12); deferred to the plan that closes out Phase 2
- [Phase 02, plan 02]: internal/dummytui's Register/RegisterOrReplace panic (not return an error) on a collision — surfaces call them from init(), so a fail-loudly-at-load contract fits better than threading error returns through every init(); collision tests assert via recover()
- [Phase 02, plan 02]: cmd/gitid-dummy + internal/dummytui import-graph is proven backend-free via an ALLOWLIST (go list -deps fails on any first-party pkg other than exactly those two), strictly stronger than a denylist — catches new/renamed backend packages by construction
- [Phase 03-create-flow-backend, plan 07]: Keep Backend.Persist synchronous while the create path uses async CommitCreate; both share the same rollback-capable commitCreateTransaction
- [Phase 03-create-flow-backend, plan 07]: Offer the D-03 copy-public-key action whenever keyUnused() is true, because stage-2 auto-chain means the user may only see the final stage
- [Phase 03-create-flow-backend, plan 07]: Bind stage outcomes to a spec fingerprint and require accepted stage-1 AND stage-2 proofs for the current spec before persistence (fail-closed store gate)
- [Phase 02, plan 02]: DLV-05/DLV-02 NOT marked complete in REQUIREMENTS.md yet — both are phase-spanning; this plan ships only the dummy skeleton (2/12 plans); deferred to the plan that closes out Phase 2 (same precedent as 02-01/DLV-01)
- [Phase 02, plan 03]: internal/screenshot/html.go extended (additive, backward-compatible) with URLFragment + RequiredText + the allow-file-access-from-files launcher flag -- CaptureHTML's FixturePath had no room for a HashRouter fragment or a pre-save breadcrumb assertion, and Chromium silently blocks a file://-loaded ES-module SPA's own imports without the flag
- [Phase 02, plan 03]: internal/dummytui/model.go gained q/ctrl+c quit handling in Update() -- doc.go always documented both as reserved but nothing ever implemented tea.Quit, hanging any PTY-driven test of the dummy
- [Phase 02, plan 03]: Zero manifest.json files shipped: the MUI mockup (02-01, one route) and the TUI dummy (02-02, five placeholder entry screens) have no overlapping screen IDs yet, so any manifest entry now would fail cross-validation by design; verified positively end-to-end via a temporary uncommitted manifest, then removed before committing
- [Phase 02, plan 03]: DLV-01/DLV-02/DLV-05 NOT marked complete in REQUIREMENTS.md yet -- this plan ships loader/adapter/driver infrastructure only (3/12 plans), no per-surface screens; deferred to the plan that closes out Phase 2 (same precedent as 02-01/02-02)
- [Phase 02]: create-flow (pilot surface) built as 12 named states in both /mui v7 and internal/dummytui, proving the full per-surface pipeline (FIELDS->manifest->parity->mockup->dummy->capture->critique) before the 6-surface fan-out
- [Phase 02]: Fixed internal/dummytui/model.go's modal-overlay compositing to pad the dimmed background to the real terminal height (was clamping/truncating against the parent surface's own natural content height, invisible until create-flow registered real multi-line screens over the identity-manager placeholder)
- [Phase 02]: git-screen's LaunchKey is 'g' (from identity-manager), matching the single-authority key-allocation table in 02-UX-DIRECTION.md / doc.go
- [Phase 02]: Git fragment target file is ~/.gitconfig.d/<identity> (REQUIREMENTS.md GITUI-02, already built), distinct from recipes/gitconfig.recipe's own ~/.gitconfig_<identity> naming; new git-screen-only recipeFixtures.ts exports added rather than editing create-flow's existing ones
- [Phase 02]: identity-manager (02-06): a/c/d intra-surface keys from the central table, RegisterOrReplace replaces the 02-02 placeholder as sole owner of key 1, placeOverlay called directly for 5 intra-surface modal screens — Third fan-out surface, first to be a number-key nav-root rather than a keyless LaunchFrom modal; proves RegisterOrReplace's placeholder-replacement design and placeOverlay's reuse outside model.go's cross-surface modalStack path
- [Phase 02]: Fixed e2e/dummy_nav_e2e_test.go reHome() to prefix-match "identity-manager/" instead of the literal "identity-manager/entry", since identity-manager's real entry screen is list-populated, not the 02-02 placeholder's entry ID — Rule 3 blocking-issue fix required for this plan's own Task 3 acceptance criteria; will also unblock 02-07..02-10 if they hit the same class of assumption
- [Phase 02]: [Phase 02, plan 07]: GSSH-01's dangerous-by-default option set pinned to StrictHostKeyChecking/ForwardAgent/HashKnownHosts/IdentitiesOnly/AddKeysToAgent/UseKeychain (mix of already-recommended and needs-action rows); advisory-not-blocking demonstrated concretely via a 3-of-4-applied/ForwardAgent-declined scenario carried end-to-end through both media
- [Phase 02]: [Phase 02, plan 07]: global-ssh options-list TUI render compacted to one line per option (git-screen's gsFieldsCompactLine precedent generalized to a full list) after the original 4-line-per-option layout overflowed the real 80x24 live PTY viewport at e2e time
- [Phase 02]: global-git (02-08): GGIT-01's 11-option baseline set interpolates the existing globalGitDefaults fixture directly (never duplicated); global user.email is modeled as never-written (D-04b), matching the real backend, not a declined recommendation
- [Phase 02]: global-git (02-08): fix-preview/confirm-write TUI compacted to 5 grouped key=value lines (git-screen's gsFieldsCompactLine precedent) after the full literal managed-block render overflowed the fixed 80x24 live PTY on the first e2e attempt
- [Phase 02]: health (02-09): 4-level doctor severity model (info/warning/error/critical) with a locked glyph contract (warning=! yellow, error/critical=✗ red, info=~ cyan) as the FIRST surface needing a 4th (cyan) semantic hue not in theme.ts's semanticColors table -- defined locally (healthInfoColor) rather than editing the shared theme file — Fan-out isolation (review MEDIUM-10): a fan-out surface writes only its own files; adding a shared theme role would touch a file every other surface also depends on
- [Phase 02]: health (02-09): per-identity-health reuses the 'legacy' identity from identityManagerRows byte-identically, tracing HLTH-05's per-identity computation to the SAME finding health-with-findings' Git section shows -- proving the slice that feeds a Manager row (MGR-07) is not re-derived data
- [Phase 02]: health (02-09): zero write-ceremony screens by design (read-only integrity, §4.6/§5) -- unlike every other primary surface, negatively asserted in Go (no confirm/backup/apply marker string anywhere) rather than only positively documented
- [Phase 02, plan 10]: fixer's fixerFindings is a filtered view over health's healthFindings (the ones carrying a suggestedFix), not an independent list -- the fixer only acts on what Health diagnosed (traceable, not re-derived).
- [Phase 02, plan 10]: fix-preview renders a TRUE before/after -/+ rewrite diff (not additions-only) because §4.7's highest-risk affordance is rewriting an EXISTING directive's value; confirm-destructive escalates to identity-manager's strongest-confirm pattern (default-focused No) for the same reason.
- [Phase 02, plan 10]: DLV-01/DLV-02/DLV-05 marked complete in REQUIREMENTS.md -- the 7th and final fan-out surface (fixer) completes Phase 2's design-first process across all seven UI surfaces. FIX-01/FIX-02/HLTH-* remain Pending (home: Phase 8, backend wiring).
- [Phase 02]: 02-11: scoped the manifest-computed PNG-count check to the 7 Phase-2 surfaces (excludes the unrelated Phase-1 _spike dir) and widened the no-backend-files gate allowlist to all of .planning/ (GSD workflow bookkeeping is not backend logic)
- [Phase 02]: 02-11: raised make test-e2e timeout 60s -> 180s once the full 50-screen dummy-nav walk runs alongside the real-TUI PTY suite in one package
- [Phase 02, review-fixes]: internal/dummytui/model.go gained a package-level currentViewport (mirrors m.width/m.height, updated on tea.WindowSizeMsg) so identity-manager's self-composited modal screens (imOverlay) can center against the REAL live terminal instead of the fixed defaultWidth/defaultHeight capture canvas -- fixes HI-01 without changing the static RenderScreen/screenshot-tui-mockups capture geometry (currentViewport defaults to the same 100x30 constants until the first resize, which static callers never send)
- [Phase 02, review-fixes]: registry_test.go/model_test.go gained a snapshotRegistry(t) helper (snapshot + t.Cleanup restore) called by every test that Register()s a test-scoped surface -- closes a proven go test -shuffle=on -count=10 failure caused by the package-level registry map leaking state across test iterations
- [Phase 02, review-fixes]: ScreenDef gained an optional, additive KeyLabels map[string]string (shell.go's renderShellKeybar consults it, falling back to the raw target screen ID) so a screen can show a semantic keybar action label ("y Yes, write") without changing what the key routes to -- used by create-flow's confirm-write only this pass
- [Phase 02, review-fixes]: HTMLOptions gained RequiredTexts []string (additive to RequiredText) and CaptureHTML now polls (25ms) for all required texts until present or Timeout expires, closing both the missing-signature-check gap (Codex B1) and the single-point-in-time-check flakiness risk (Codex B3) in one change
- [Phase 02, review-fixes]: added .planning/design/mockup-src/src/data/screenSignatures.ts (byte-identical mirror of every manifest.json's signature field, keyed by ScreenID) + wired into Shell.tsx as a rendered [SIG-...] marker -- the HTML mockup previously had no signature marker anywhere in its DOM (signatures were TUI-only), so design_capture_test.go could not require one on the HTML side; zero per-route file edits needed since Shell.tsx already receives the ScreenID as its title prop
- [Phase 02, review-fixes]: Shell.tsx changed from a fixed height:'100vh' + main's overflow:'auto' to minHeight:'100vh' + natural flow -- the fixed height's inner scroll region clipped any body taller than the 800px viewport INSIDE itself, invisible to go-rod's fullPage screenshot capture (which only sees the outer document's scroll height), causing 3 reference PNGs (global-ssh/global-git options-list, identity-manager list-populated) to be fold-clipped despite the content existing in the DOM
- [Phase 02, review-fixes]: added a gate-no-backend-files Makefile target (git merge-base main HEAD diff against the Phase-2 allowlist), wired as a dummy-nav-e2e prerequisite -- closes SECURITY.md Finding 1 (T-02-BEGATE had no persisted/automated enforcement, only a one-off plan-file shell line)
- [Phase ?]: 02-13: Bubble Tea v2 retained for the live gitid-dummy demo; go-tui evaluated and rejected as too immature
- [Phase ?]: 02-13: terminal 100x30 adaptations (edit/git ceremonies as next pane-state, stacked step-3 previews, Ctrl+S skip, 36% sidebar) keep the web demo's semantics and copy, pinned by tests
- [Phase 02]: 02-14: Central Go Theme + web theme.ts roles export mirror each other 1:1 by name; frame.go's promotion to DefaultTheme proven behavior-preserving by a byte-identical render test
- [Phase 02]: 02-14: ActiveArea accent renders on the breadcrumb/divider line (zero extra rows) rather than a frame-wide border -- the 100x30 budget could not absorb a bordered active-pane region
- [Phase 02]: 02-14: Arrow-key precedence rule (expanded-select > text-input-cursor > wizard-step-nav validity-gated forward/always-allowed back > Shift+left/right focus-override) is written once in 02-STYLE-SPEC.md and implemented identically in Go and TypeScript
- [Phase 02]: 02-14: TUI field contour costs stayed within 100x30 by dropping the redundant git-form Signing line and tightening preview maxLines -- a documented row-budget tradeoff, not scope creep
- [Phase 02-15]: D9's global-fallback user.email applies through its own dedicated ceremony, never folded into the baseline managed-block apply
- [Phase 02-15]: Row-budget number in 02-STYLE-SPEC.md corrected to the measured ~24 of 25 body rows (tightest wizard pane), replacing the plan's original ~21-row estimate
- [Phase 02-15]: e2e/ui_pty_e2e_test.go's ptySession.close() gained a bounded ctrl+c grace period + SIGKILL fallback -- fixes a real test-hang unrelated to this plan's own feature changes
- [Phase 03-create-flow-backend]: D-09 alias-collision was already fully wired by 03-03 + this plan's Task 2; Task 3's real scope was the KEY-06 reuse-existing-key picker
- [Phase 03-create-flow-backend]: identity.Reuse split into StageReuse (ensurePub staging) + the existing write pipeline so the TUI wizard's staged test-then-write flow shares the D-11 encrypted-key logic instead of duplicating it
- [Phase 03-create-flow-backend]: D-12 in-use-by label reformatted to '<identity> (<provider-host>)' across all three tuikit.Backend implementers (real, dummy, tuikit test stub)
- [Phase 03-create-flow-backend, plan 05]: D-03's copy-.pub action is a footer/keybar affordance only (never a second inline body row) to stay inside the fixed 100x30 wizard pane row budget; the D-01 key-unused ceremony copy fully replaces (never appends to) the normal ResultMessage/Heading, and does NOT touch the unrelated 8-state identity taxonomy
- [Phase 03-create-flow-backend, plan 06]: D-24's "approved dummy golden" is the LIVE cmd/gitid-dummy FixtureBackend's rendered text, computed fresh in-process at gate time (no static per-screen golden files survive Phase 2 — REFERENCE-INDEX.md); the D-24.1 gate is text-only (no PNG/freeze rendering), diffed at SCREEN granularity (byte-exact-or-allowlisted per named screen) rather than line granularity
- [Phase 03-create-flow-backend, plan 06]: the visual-divergence allowlist carries a 4th entry (T-03-HOSTBLOCK, realBackend.HostBlockPreview's sshconfig.RenderHostBlock format vs. the dummy's unrelated 4-space markerless literal) beyond the plan's stated D-02/D-16/D-19 — a pre-existing divergence carried from 03-03/03-04, discharged as an allowlist entry rather than a code change (the dummy fixture is frozen Phase-2 surface)
- [Phase 03-create-flow-backend, plan 06]: internal/screenshot/createflow.go drives the shared tuikit render stack IN-PROCESS (tea.Model.Update/.View, no PTY/subprocess) for the visual-regression gate — PTY stays DLV-06's job (raw keystroke/terminal decoding correctness); the in-process technique is reusable by future phases' own dummy-vs-real gates
- [Phase ?]: Keep Backend.Persist synchronous while create path uses async CommitCreate; both share commitCreateTransaction
- [Phase ?]: Offer copy-public-key action whenever keyUnused() is true due to stage-2 auto-chain
- [Phase 03]: CR-08: Generate no longer touches sshDir; confirmed transaction creates/chmods ~/.ssh as step 0 with rollback
- [Phase 03]: CR-09: modeOp rollback for reused private key chmod(0600) in confirmed transaction; failure injection restores prior mode
- [Phase 03]: CR-10/WR-01: DemoIdentity.Algorithm/Provider fields; finishIdentity populates both; createInput reads directly without reconstruction
- [Phase 03]: CR-04: 'differs' predicate removed from visual gate allowlist; all entries use contains:/absent: with specific needles
- [Phase 03]: RenderCheckedHostBlock is the authoritative render boundary for SSH Host blocks — validates unicode.IsSpace and controls — all three production callers route through it (CR-08)
- [Phase 03]: specFingerprint includes ReuseKeyPath (normalized absolute path) so generate vs reuse and different reuse paths produce distinct fingerprints invalidating staged material (CR-07)

### Roadmap Evolution

- 2026-07-02: Prior build reframed as archived **0.0.1 POC** (never released) under `.planning/archive/0.0.1-poc-product-features-in-tui/`; phase numbering **reset** for the real v1.0. New 10-phase roadmap derived 1:1 from the PRD "Execution Phases" (Phase 0→1 … Phase 9→10). Existing Go packages are reusable substrate, not a behavior contract. Loop vehicle: `.planning/ONESHOT-LOOP-PROMPT.md`.
- Phase 9.6 inserted after Phase 9: Global Git/SSH Options Consistency Fixes: type-aware rendering, e-to-edit, orange-! clarification, user.email/name duplicate-read-path bug fix (found in user audit of Options/Set-keys screens) (URGENT)
- Phase 9.7 inserted after Phase 9: New-Identity Wizard Consistency Fixes: Key/Git-identity section grouping, Test-connection color/type + warning persistence, typed log, text-input width/hint consistency (found in same user audit) (URGENT)

### Pending Todos

None yet.

### Blockers/Concerns

- ~~**BLOCKING 2026-08-25 — Phase 04 code-review circuit breaker:** the phase 4
  code-review/fix convergence loop ran 6 review passes + 5 fix passes on
  branch `gsd/phase-04-git-configuration-screen` (see
  `04-git-configuration-screen/04-REVIEW.md` + `04-REVIEW-FIX.md` +
  `.iter2..iter7.md` post-mortem trail). It fixed 4 original CRITICAL
  findings, a 3-defect regression, a 5-defect regression (incl. a repeated
  gate-blindspot class on a second build tag), and a security finding
  (CR-18, comma-injection into `~/.ssh/allowed_signers` principals — fixed
  in `8c5936b`, verified against real `ssh-keygen -Y verify`). It stopped
  with 2 structural findings open: **CR-15** (the visual-regression gate
  cannot detect a shared-renderer defect, since `cmd/gitid` and
  `cmd/gitid-dummy` render Configure-Git through the same `internal/tuikit`
  code — an architectural property of the D-12 comparison approach) and
  **CR-16** (44/160 comparable regions accept arbitrary live-side mutation).
  Also open: **CR-17** (a fixer's claimed red-before-fix test did not
  reproduce under source reversion — a process-trust concern for this loop
  specifically) and **WR-38 through WR-43** (6 non-critical findings,
  documented with file:line in `04-REVIEW-FIX.md`). Every commit this loop
  produced is independently gate-verified by the orchestrator (go build, go
  test -race, make lint incl. -tags screenshot/smoke/e2e, make test, make
  test-e2e, make gate-visual-regression) — the open items require a human
  design decision (CR-15/CR-16: how the D-12 gate should catch
  shared-renderer defects) and a scope/priority call (WR-38..43), not
  another autonomous fix attempt. Precedent: Phase 3's own 2026-08-21
  circuit breaker (below). Resume only after a human reviews and decides;
  do not advance Phase 4 to verify-work/UI-review/audit-uat or start Phase
  5 until this is resolved.~~ -- RESOLVED 2026-08-25: user reviewed and
  chose to accept CR-15/CR-16 as a documented scoped divergence (same class
  of decision as D9/T-04-HOSTBLOCK) rather than design a new gate mechanism.
  CR-18 (security) was already fixed. CR-17 recorded as a process note
  (verify future fixer red/green claims via source reversion when
  CRITICAL/security-relevant). WR-38..43 carried forward as non-blocking.
  See `04-REVIEW-FIX.md` "Resolution" section. Phase 4 code-review checklist
  item CLOSED; proceeding to verify-work/UI-review/audit-uat.

- 3 items intentionally open until their phase (documented in REQUIREMENTS.md "Still Open"): GSSH-01 dangerous-options list, KEY-01 catalog ordering/copy, screenshot-tooling mechanism (Phase 1 spike).
- Phase 2 VERIFICATION.md W1 (non-blocking): `insteadOf` URL rewriting (recipes/ wiring #3) is not rendered in either live demo — only an unused fixture constant. Cover it in Phase 4/7 design or document as a scoped divergence next to D9.
- Phase 2 VERIFICATION.md W2 (non-blocking): `internal/dummytui/nobackend_test.go` was deleted in 7453561 and never restored — the no-backend truth was re-proven directly (`go list -deps`) and `gate-no-backend-files` holds, but consider restoring an import-graph test before/during Phase 3.
- ~~02-14: fresh agent-ui-ux-designer critique of both live demos (DLV-02 exit gate) and the superpowers:requesting-code-review pass are outstanding~~ -- RESOLVED 2026-07-05: the orchestrator ran both reviews; findings F1-F11 are fixed/recorded (see 02-14-SUMMARY.md "Review findings resolution (post-plan fix pass)"). No outstanding blocker for 02-12 sign-off from 02-14.
- ~~02-15: a fresh agent-ui-ux-designer critique of both live demos (checkpoint-2 dimensions) and a fresh-context superpowers:requesting-code-review pass (against 02-15's must_haves + every task's acceptance_criteria) are OUTSTANDING orchestrator-run exit gates — the executor could not run them. Their CRITICAL/HIGH findings must be resolved before 02-12 (the DLV-08 re-presentation) proceeds. See 02-15-SUMMARY.md.~~ -- RESOLVED 2026-07-05: the orchestrator ran both reviews (a fresh agent-ui-ux-designer parity critique + a fresh-context code review). Findings F1-F10 (CRITICAL: F1 stale-tab closure breaking web view-switching; HIGH: F2 web ceremony/pane footers never mirrored; IMPORTANT: F3 TUI click-hijack from whole-line substring matching, F4 D9 email row swept into the baseline overlay in both media, F5 inline glyph literals, F9 web `a`-key precedence; MEDIUM/MINOR: F6/F7/F8/F10) plus one record-only item (re-measure the row budget, don't assume) are all fixed/recorded across two commits — see 02-15-SUMMARY.md "Review findings resolution (post-plan fix pass)". All 7 gates (go test -race, make test/lint/test-e2e/gate-no-backend-files, the copy-freeze greps, pnpm typecheck+build) are green. No outstanding blocker for 02-12 sign-off from 02-15.
- ~~03-05: the plan's Task 1 asked the executor to engage `/mui` + `agent-ui-ux-designer` on the new D-02/D-03 warning-state visual and record the input in the SUMMARY — the plan executor has no access to spawn sub-agents or slash commands (Read/Write/Edit/Bash tools only), so this did NOT happen. Flagging as an orchestrator-run exit gate still owed, same class as 02-14/02-15's pattern above. See 03-05-SUMMARY.md "Issues Encountered".~~ -- RESOLVED 2026-08-17: the orchestrator ran the review (`agent-ui-ux-designer:ui-ux-designer` against the D-02/D-03/D-19 render code). 5 findings (F1/F2/F3/F4.2/a11y) fixed directly (commit pending this session); 2 findings (F5/F5b, below) carried forward. See 03-05-SUMMARY.md "DLV-02 retroactive design critique".
- 03-05 design-review F5 (non-blocking, carried forward): `wizardContinueHint` ("Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing.") contradicts the real binary's always-disabled `[ Continue ]` reason ("Git configuration arrives with the next build") — a user reads both lines together and the copy disagrees with itself. `wizardContinueHint` is a Phase-2 LOCKED design contract (D6, checkpoint-2: "both frozen hints ALWAYS visible below the row"), so it cannot be suppressed/reworded without a documented scoped-divergence decision (the project's own D9 precedent — see 02-DESIGN-DECISIONS-CHECKPOINT-2.md). Candidate owner: Phase 4 (Git Configuration Screen), which is exactly when this hint's promise becomes true.
- 03-05 design-review F5b (non-blocking, carried forward): the real binary's Git-step form fields stay focusable/editable even though `[ Continue ]` can never submit them at this phase — a user can type `user.name`/email into a form whose only submit path is permanently disabled, and nothing at field level signals the inertness. Medium effort, no frozen-copy conflict. Candidate owner: same as F5 (Phase 4) or a dedicated Phase-3 fix pass if surfaced again before then.
- ~~03-06 (was BLOCKING): the DLV-04.2 cross-AI review (`agent-ui-ux-designer` + Codex) has NOT run.~~ **RESOLVED 2026-08-21 by plan 03-09 Task 3**: UI-REVIEW.md + CODEX-REVIEW.md both PASS (no Critical/High). CR-10/CR-11/WR-01 all closed. Phase 3 gates are green.
- **REMAINING for Phase 3 ONESHOT checklist (steps 6-9):** gsd-code-review (03-REVIEW.md from 2026-08-18 had 14 CRITICAL — these were addressed by 03-07/03-08/03-09; a fresh code review is needed to verify), verify-work (03-VERIFICATION.md needs the passing evidence), UI-review (03-UI-REVIEW.md if required), audit-uat. These are ORCHESTRATOR obligations before Phase 4 starts. Plan 03-06's Task 3 is explicitly split (orchestrator-run exit gate, DLV-08 unattended-loop convention) — the executor assembled the review packet (`.planning/phases/03-create-flow-backend/03-06-review-packet/` — `text-diffs.txt` + `MANIFEST.md`, commit `9ddd102`) but has no subagent-spawning tools, same class as 02-14/02-15/03-05's own gaps above. The orchestrator must run both reviews against the packet, record findings + disposition in `03-06-SUMMARY.md`'s "Cross-AI visual-regression review" section (currently a placeholder scaffold), and resolve any CRITICAL/HIGH finding before Phase 3 is marked complete.
- **BLOCKING 2026-08-21 — Phase 03 circuit breaker:** 03-10 was the single authorized gap-closure attempt. Independent re-review (`03-REVIEW.md`, 2026-08-21T21:41:10Z) found 10 critical defects still open: the immutable-packet publisher is a successful no-op; approval-commit HTML/TUI evidence is never captured; the routine gate checks only current-head text; its region policy remains broad; production stage 2 does not consume or display complete `ssh -G` proof; staged-key cache identity is incomplete; final render validation is bypassable; rollback can leave a newly created `~/.ssh`; and determinism is process-local only. The run must not advance to Phase 4. Resume only after a human resolves the blocker and requests a new Phase 3 closure pass.
- 03-06 (non-blocking, carried forward): `internal/tuikit/identities.go`'s create-wizard SSH form renders an unused "Provider" field row (focus slot 0, reachable via Tab) that FIELDS.md's approved 4-field contract does not include and D-20 explicitly says should not exist ("no new field"). Flagged, not fixed — see 03-06-SUMMARY.md "Issues Encountered". Candidate owner: a dedicated Phase-3 fix pass or a documented scoped-divergence decision, whichever the user prefers.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260705-f9t | Add `make demo-web` target: relaunch the web mockup Vite dev server on dedicated port 45173 and open the browser | 2026-07-05 | 9ecfbb4 | [260705-f9t-add-make-target-to-relaunch-the-web-mock](./quick/260705-f9t-add-make-target-to-relaunch-the-web-mock/) |
| 260831-3a9 | Fix 5 UX-critique defects (D1-D5) in Phase 9 upload/register-key/rotate-delete screens; re-promote stale PTY frames; close REVIEW.md's parity-critique obligation | 2026-08-31 | 0ad0ed7 | [260831-3a9-fix-5-ux-critique-defects-d1-d5-in-phase](./quick/260831-3a9-fix-5-ux-critique-defects-d1-d5-in-phase/) |
| 260907-eda | Add a Go-toolchain bootstrap step to setup-env so a fresh clone with no go on PATH gets one installed automatically | 2026-09-07 | 0fdc648 | [260907-eda-add-a-go-toolchain-bootstrap-step-to-set](./quick/260907-eda-add-a-go-toolchain-bootstrap-step-to-set/) |
| 260919-jnl | Non-destructive Git/SSH/Ignore apply ceremonies confirm with Enter without typing yes. Keep the exact-change preview. Typed-confirm stays only for destructive rewrites. | 2026-09-19 | 3dfdb15 | [260919-jnl-non-destructive-git-ssh-ignore-apply-cer](./quick/260919-jnl-non-destructive-git-ssh-ignore-apply-cer/) |
| 260921-t6g | Fix broken CI/Nightly: goreleaser v2.18.0 requires go>=1.27.0 but Makefile exports GOTOOLCHAIN:=go1.26.4; repinned to v2.17.0, added a regression-guard test, swept stale version references | 2026-09-21 | e24ee8c | [260921-t6g-fix-broken-ci-nightly-workflows-goreleas](./quick/260921-t6g-fix-broken-ci-nightly-workflows-goreleas/) |
| 260922-brh | Fix red CI after the goreleaser repin: SSH Include `~` now expands against the backend's managed home, not the process $HOME (production leak behind 6 tests failing on populated homes); stale diff3 source-grep replaced with policy-derived AST checks; fedora CI job installs gcc for -race/cgo | 2026-09-22 | f1d4e99 | [260922-brh-fix-7-red-ci-tests-stale-diff3-source-gr](./quick/260922-brh-fix-7-red-ci-tests-stale-diff3-source-gr/) |

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-09-07T23:02:55.384Z
Stopped at: Phase 9.7 context gathered
Resume file: .planning/phases/09.7-new-identity-wizard-consistency-fixes-visual-grouping-for-ke/09.7-CONTEXT.md
Wave structure: W1 = 03-01 + 03-02 (DONE) -> W2 = 03-03 (DONE) -> W3 = 03-04 (DONE) -> W4 = 03-05 (DONE) -> W5 = 03-06 (Tasks 1+2 DONE, Task 3 PARTIAL — orchestrator review owed)
**All remaining waves run SEQUENTIALLY (one executor at a time) per LEARNINGS L11** — the pre-commit hooks lint the whole module, so a parallel executor's mid-refactor tree blocks every other commit. Do NOT run plans in parallel inside this Go module again.

WAVE 1 CLOSED 2026-07-25 — orchestrator-verified gates (NOT executor claims):

- go build ./... exit 0
- TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./... -> 1122 passed, 21 packages
- make lint -> 0 issues
- make test-e2e -> ok 47.3s
- go list -deps ./internal/tuikit -> NO first-party backend import (boundary holds)
- test integrity: 929 -> 942 test funcs, no new t.Skip, one rename only
  (TestReduceReset -> TestReset, verified strictly STRONGER: keeps the original
  contract at the Backend seam that now owns Reset AND adds a negative
  assertion that Reduce must leave Reset alone)
Commits: 3dd4f47 (tuikit extraction behind the Backend seam), 3fd1568 (summaries + L11/L12), 465739c (03-02 Task 3: restored import-graph allowlist test, retired the obsolete Phase-2 path gate).
Wave-1 cost: 4 agent deaths (1 API/session-limit, 3 watchdog stalls); orchestrator repaired 6 call-site errors inline and closed Task 3 inline.

WAVE 2 CLOSED 2026-07-25 (plan 03-03) — orchestrator-verified gates:

- go build exit 0; make build exit 0 (the pre-Wave-3 obligation)
- TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./... -> 796 passed, 19 packages
- make lint 0 issues; make test-e2e ok 20.9s
- TestNoBackendAllowlist PASS; go list -deps ./internal/tuikit -> no backend import
- internal/adopter UNTOUCHED (git diff empty) — the engine survived the DROP
- L4 BLOCKING guard verified live: TestOrphansReservedArtifactsSurviveFix AND
  TestOrphansNonIncludeAwareDepsAreDestructive both PASS. The destructive
  CONTROL proves the guard is not vacuous; the executor also observed real RED
  (reverting the _global registration made the fix path delete the macOS
  globals block outright — precisely the destructive loop L4 exists to stop).

- Real ReadPub wired non-nil in cmd/gitid/wiring.go:257 (L2 real-constructor half)

Commits: e75e32d (L4 registration + doctor proof), d60a4d7 (POC DROP + D-15 rewire + composition root + persist + REQUIREMENTS STORE-01), 2369750 (03-03-SUMMARY.md), plus the orchestrator's correction below.

Test count moved 1122 -> 796 because the archived POC packages took their own
tests with them. Verified legitimate, with ONE exception the orchestrator
caught and fixed: e2e/install_e2e_test.go was archived wholesale, but only its
TestInstall_PathFeedback half drove an archived Cobra command (`gitid doctor`);
its TestInstall_MakeInstallOutput half tests the `install` MAKEFILE TARGET,
which survives Phase 3 untouched (Makefile:183, still echoing the install path
and PATH hint the test pins). That test was RESTORED — a real coverage
regression the executor's own justification did not cover.

CARRIED INTO 03-06 (flagged by the 03-03 executor): real-backend values
legitimately differ from the frozen dummy fixtures and need visual-divergence
allowlist entries — most notably HostBlockPreview now renders via
sshconfig.RenderHostBlock (2-space indent + `# gitid: provider=...` marker)
rather than the dummy's 4-space markerless text, because "written exactly like
this on confirm" requires the preview to BE the written text. Full table in
03-03-SUMMARY.md.
CARRIED INTO 03-05: realBackend.Persist has no error channel; a failed write
returns the PREVIOUS state and records the cause on persistErr, exposed via
PersistError() for 03-05 to render. Not a swallow — must be surfaced in the UI.

WAVE 3 CLOSED 2026-08-17 (plan 03-04) — executor-run gates (orchestrator
independent re-verification per ground rule 4 still applies at the wave-close
review, not yet run by this session):

- go build ./... exit 0
- TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./... -> 826 passed, 19 packages
- make lint -> 0 issues
- make test-e2e -> ok
- make test (incl. gate-copy-freeze) -> ok
- go list -deps ./internal/tuikit -> NO first-party backend import (boundary holds)
- go test ./internal/dummytui/ -run TestNoBackendAllowlist -> PASS
- grep for a keygen import line in internal/tuikit -> no match

Commits: 76fb231 (Task 1, D-16 banner), cc1f614 (Task 2, SSHUI-01/03 autofill),
5df3e2d (Task 3, D-09 verify + KEY-06 reuse picker + the ParseManagedHosts
reserved-block bugfix). Full record: 03-04-SUMMARY.md.

CARRIED INTO 03-06 (flagged by the 03-04 executor): the D-10 reuse-picker's
combined "Key" header row + its body (algorithm radios vs the reuse list) is
a NEW render surface with no dummy-side equivalent yet — the approved Phase 2
goldens never exercised this picker. 03-06's visual-regression gate will need
either a divergence allowlist entry or an equivalent dummy-side fixture
screen; Claude's discretion per 03-04-SUMMARY.md "Next Phase Readiness".
CARRIED INTO 03-06 (L2 remainder): TestReuseEncryptedKeyWithExistingPubSucceeds
(cmd/gitid/wiring_test.go) proves the encrypted-key-with-existing-.pub reuse
path through the REAL constructor and real filesystem, but NOT through a live
raw-keystroke PTY session — 03-06's case 6 (reuse-existing-key/ReadPub) is
still the only thing that closes the L2 obligation per project convention (a
unit/wiring test never substitutes for the PTY proof).

WAVE 4 CLOSED 2026-08-17 (plan 03-05) — executor-run gates (orchestrator
independent re-verification per ground rule 4 still applies at the wave-close
review, not yet run by this session):

- go build ./... exit 0
- TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./... -> 833 passed, 19 packages
- make lint -> 0 issues
- make test-e2e -> ok
- make test (incl. gate-copy-freeze, both new D-02/D-01 strings + the
  cmd/gitid-scoped D-19 real string) -> ok

- go list -deps ./internal/tuikit -> NO first-party backend import (boundary holds)
- go test ./internal/dummytui/ -run TestNoBackendAllowlist -> PASS
- grep for a tester import line in internal/tuikit -> no match (only doc comments name tester.Outcome/tester.Result)

Commits: c680fde (all 3 tasks, one commit per CLAUDE.md's buildable-boundary
commit-granularity rule — the code for D-02/D-03/D-19/D-18 is tightly
intermixed inside the same shared functions, e.g. renderStageOutcome and
gitContinueGate), 2c4e328 (03-05-SUMMARY.md). Full record: 03-05-SUMMARY.md.

CARRIED INTO 03-06 (flagged by the 03-05 executor, same class as 03-04's
carry-over): the new D-02/D-03 warning-state render (yellow "!" outcome row +
Theme.Hint key-settings line) and the D-19 real-binary git-step disabled
reason are NEW render surfaces/text with no approved Phase-2 dummy golden to
diff against (D-02 is an outcome the demo never had; D-19's real string only
exists in cmd/gitid's own capture). 03-06's visual-regression gate needs
either allowlist entries or equivalent dummy-side fixture states.
CARRIED INTO 03-06 (still open, re-flagged from 03-03/03-04):
realBackend.PersistError() is exposed but nothing in internal/tuikit reads it
after a committed write — a real write failure currently shows the SAME
"created" note as a success. See deferred-items.md. Candidate owner: 03-06 or
a dedicated fix pass.
CARRIED INTO 03-06: a raw-keystroke PTY proof of the D-02/D-03 warning +
copy-pub + D-18 Skip paths through the REAL binary is still owed per the L2
closure contract — 03-05's own tests are internal/tuikit unit tests + one
cmd/gitid wiring test, never a live PTY session.

WAVE 5 (plan 03-06) — Tasks 1+2 CLOSED 2026-08-18, Task 3 PARTIAL
(executor-run gates; orchestrator independent re-verification per ground
rule 4 still applies at the wave-close review, not yet run by this
session):

- go build ./... exit 0
- TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./... -> 836 passed, 19 packages
- make lint -> 0 issues
- make test (incl. gate-copy-freeze) -> ok
- make test-e2e -> ok (-race; all 7 new PTY cases included)
- make gate-visual-regression -> ok (8/8 screens; fail-path hand-verified
  by removing the git-form-demo allowlist entry, observing a red FAIL, then
  reverting)

- make smoke-network-test -> ok (real network this session; outcome
  ReachableNotUploaded)

- go list -deps ./internal/tuikit -> NO first-party backend import (boundary holds)

Commits: 57bda7b (Task 1 — 7 new PTY e2e cases + 2 Rule-1 auto-fixes: the
FakeSSHDir -G fixture-script bug, and 3 real-binary-only row-budget
overflows in the wizard's step-1 pane), 4a9c939 (Task 2 — the golden-text
gate + allowlist + D-23 smoke test + 1 Rule-2 auto-fix: a 4th,
pre-existing HostBlockPreview-format allowlist entry the plan's literal
D-02/D-16/D-19 scope did not anticipate), 9ddd102 (Task 3 — review-packet
assembly ONLY, docs-only). Full record: 03-06-SUMMARY.md.

Task 3's review-EXECUTION half (agent-ui-ux-designer + Codex against the
assembled packet) is an ORCHESTRATOR obligation, NOT done this session —
see Blockers/Concerns above and 03-06-SUMMARY.md "Cross-AI
visual-regression review" for the placeholder section to fill in.
Phase 3 is NOT complete until that review lands and any CRITICAL/HIGH
finding is resolved.

Current branch: gsd/phase-03-create-flow-backend
PHASE-3 BASE SHA: 61d98fa (anchors external code-review range in Step 6d)
CIRCUIT BREAKER: replans_used=1/2; review_fix_loops=1/3

VERIFICATION ROUND 1 RESULTS (both consumed):

- Codex re-check: 6/7 RESOLVED, finding #6 PARTIAL, verdict BLOCK on 3 new problems (encrypted-key fingerprint contradiction; mouse-test field-scope mismatch; stale 03-06 Task cross-ref). ALL THREE FIXED in e59d285. Second Codex re-check launched (bg id bev1zm0oc, output scratchpad/codex-recheck2-3.out).
- gsd-plan-checker: PASS-WITH-CONCERNS. Goal achievable; 11/11 requirement IDs covered; dependency/wave graph correct (03-03 -> [03-01,03-02] confirmed, no file collisions in a wave); 18/18 tasks have read_first + checkable acceptance_criteria; tuikit DTO boundary consistent; 03-03's 4-task size explicitly JUDGED ACCEPTABLE (L4 registration must ship with the code creating the artifacts). Two LOW concerns: (a) run `make test && make build` right after 03-03 before Wave 3 starts — ORCHESTRATOR obligation at wave close (ground rule 4 already covers it); (b) 03-05's "§6 copy-freeze grep gate" was ambiguous — FIXED in e59d285 by naming the real test-based mechanism.

EXECUTION OBLIGATIONS carried into Step 3.5 (from the verification round):

- Ground rule 4 at EVERY wave close: orchestrator personally runs make test, make test-e2e, make lint (never trust an executor PASS claim). Additionally run `make build` immediately after 03-03 completes, before Wave 3 starts.
- 03-05 executor must name the frozen-string test function it adds/extends in its SUMMARY (both new frozen strings asserted byte-exactly).

Replan resolution map (52fc7d9):

- #1 L2 ReadPub seam -> 03-01 nil-guard + fallback test; 03-06 raw-keystroke PTY reuse-key case (encrypted key w/ existing .pub)
- #2 L4 doctor-reserved -> 03-03 BLOCKING Task 4 (IsReservedPath/ReservedPaths + TestOrphansReservedArtifactsSurviveFix)
- #3 legacy DROP -> 03-03 drops internal/repoclone + addrepo/adopt as ONE build-safe unit; internal/adopter KEPT
- #4 dep edge -> 03-03 depends_on ["03-01","03-02"]
- #5 tuikit backend-free -> 03-02 defines views.go DTOs; 03-04/03-05 consume views; conversion only in cmd/gitid/wiring.go; allowlist-widening forbidden
- #6 mouse -> 03-06 xterm SGR mouse-CSI PTY case + visual gate enumerates reuse picker/manual-path/mouse-focus
- #7 STORE-01 -> 03-03 updates REQUIREMENTS.md with supersession note

## Rebuild Log

- timestamp: 2026-08-22T18:02:31.155Z
  kind: by-phase-table-reconciled
  section: ## Performance Metrics
  before: | Phase | Plans | Total | Avg/Plan | \n |-------|-------|-------|----------| \n | 1. Foundations, Spikes & CI | 0 | - | - | \n | 2. DESIGN — All Mockups (★) | 0 | - | - | \n | 3. Create Flow Backend | 0 | - | - | \n | 4. Git Configuration Screen | 0 | - | - | \n | 5. Identity Manager | 0 | - | - | \n | 6. Global SSH Options | 0 | - | - | \n | 7. Global Git Options | 0 | - | - | \n | 8. Health + Fixer | 0 | - | - | \n | 9. Upload / Credentials Assist | 0 | - | - | \n | 10. Linux Validation + Release | 0 | - | - |
  after: | Phase | Plans | Total | Avg/Plan | \n |-------|-------|-------|----------| \n | 01 | 7 | - | - | \n | 02 | 16 | - | - | \n | 03 | 15 | - | - | \n | 04 | 0 | - | - | \n | 05 | 0 | - | - | \n | 06 | 0 | - | - | \n | 07 | 0 | - | - | \n | 08 | 0 | - | - | \n | 09 | 0 | - | - | \n | 10 | 0 | - | - |
  reason: phase dirs on disk are canonical; rows for missing phases dropped, missing phases added
