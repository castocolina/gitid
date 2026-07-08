---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: "02-15 (wave 8) operationalized the binding 02-DESIGN-DECISIONS-CHECKPOINT-2.md contract (D1–D9 + affordance audit) in BOTH demos, byte-for-byte: D1 single-row color-only fields (02-14's rounded box deleted), D2 always-expanded match-strategy/algorithm radios, D3 terminal-glyph checkbox/radio on the web, D4 bracketed main-nav format (`[N] Label`, moved off the wizard stepper) + a new ActiveNavDimmed/activeNavDimmed state + a top-level plain-arrow view switch, D5 the wizard stepper reverted to `Step n/4 · <label> ● ○ ○ ○`, D6 one-row git-step buttons, D7 ONE hoisted Shift+←/→ chord gate reaching every step including the previously-dead review ceremony (proven with a new raw-byte PTY e2e injecting real xterm CSI sequences), D8 click-to-focus on every form row, and D9 Global Git's user.email promoted to an editable, opt-in global-fallback field with its own dedicated write ceremony (a documented, scoped recipes/ divergence). 02-STYLE-SPEC.md + both FIELDS.md companions rewritten in lockstep; the full exit-gate battery is green (go test -race, the no-backend allowlist, the extended copy-freeze grep, make test/lint/test-e2e/gate-no-backend-files, pnpm typecheck+build) — see 02-15-SUMMARY.md. The two ORCHESTRATOR-run exit gates (a fresh agent-ui-ux-designer critique of both live demos + a fresh-context code review against 02-15's must_haves/acceptance_criteria) have since RUN and their findings (F1-F10 + one record-only item) are fixed — see 02-15-SUMMARY.md "Review findings resolution (post-plan fix pass)" and commits a335d80/f62c99e. Next is 02-12 (wave 9, the single DLV-08 approval checkpoint), unblocked."
stopped_at: Phase 6 context gathered
last_updated: "2026-07-08T02:19:36.046Z"
last_activity: "2026-07-06 -- Completed 02-12 (★ DLV-08): user approval recorded as `**APPROVED:** 2026-07-06 by Pepe`; Phase 2 COMPLETE — the approved live demos + 02-REDESIGN-SPEC.md/02-STYLE-SPEC.md/02-DESIGN-DECISIONS-CHECKPOINT-2.md + per-surface FIELDS.md are the binding design reference; Phases 3-9 backend work is UNBLOCKED"
progress:
  total_phases: 10
  completed_phases: 1
  total_plans: 29
  completed_plans: 22
  percent: 10
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-02)

**Core value:** Managing a Git identity produces coordinated, coherent SSH + Git artifacts that are proven to authenticate and resolve correctly (`ssh -G`) before any file is written, and existing hand-written config is never corrupted.
**Current focus:** Phase 02 COMPLETE (★ CHECKPOINT #1 passed) — next: Phase 03 (create-flow-backend)

## Current Position

Phase: 02 (design-all-mockups-checkpoint-1) — COMPLETE (all 15 plans done; ★ DLV-08 approval recorded)
Plan: 02-12 (wave 9, the single DLV-08 human checkpoint) — COMPLETE. The user approved both live demos and supplied the approver name; `.planning/design/APPROVAL.md` now carries `**APPROVED:** 2026-07-06 by Pepe` (Status: APPROVED, all §A-F/E2/E3 items ticked). See 02-12-SUMMARY.md for the checkpoint record (first presentation → 11-question feedback round → binding 02-DESIGN-DECISIONS-CHECKPOINT-2.md contract → 02-15 route-back + F1-F10 review fix pass → micro-fix d6438bd → approval).
Status: 02-15 (wave 8) operationalized the binding 02-DESIGN-DECISIONS-CHECKPOINT-2.md contract (D1–D9 + affordance audit) in BOTH demos, byte-for-byte: D1 single-row color-only fields (02-14's rounded box deleted), D2 always-expanded match-strategy/algorithm radios, D3 terminal-glyph checkbox/radio on the web, D4 bracketed main-nav format (`[N] Label`, moved off the wizard stepper) + a new ActiveNavDimmed/activeNavDimmed state + a top-level plain-arrow view switch, D5 the wizard stepper reverted to `Step n/4 · <label> ● ○ ○ ○`, D6 one-row git-step buttons, D7 ONE hoisted Shift+←/→ chord gate reaching every step including the previously-dead review ceremony (proven with a new raw-byte PTY e2e injecting real xterm CSI sequences), D8 click-to-focus on every form row, and D9 Global Git's user.email promoted to an editable, opt-in global-fallback field with its own dedicated write ceremony (a documented, scoped recipes/ divergence). 02-STYLE-SPEC.md + both FIELDS.md companions rewritten in lockstep; the full exit-gate battery is green (go test -race, the no-backend allowlist, the extended copy-freeze grep, make test/lint/test-e2e/gate-no-backend-files, pnpm typecheck+build) — see 02-15-SUMMARY.md. The two ORCHESTRATOR-run exit gates (a fresh agent-ui-ux-designer critique of both live demos + a fresh-context code review against 02-15's must_haves/acceptance_criteria) have since RUN and their findings (F1-F10 + one record-only item) are fixed — see 02-15-SUMMARY.md "Review findings resolution (post-plan fix pass)" and commits a335d80/f62c99e. Next is 02-12 (wave 9, the single DLV-08 approval checkpoint), unblocked.
Last activity: 2026-07-06 -- Completed 02-12 (★ DLV-08): user approval recorded as `**APPROVED:** 2026-07-06 by Pepe`; Phase 2 COMPLETE — the approved live demos + 02-REDESIGN-SPEC.md/02-STYLE-SPEC.md/02-DESIGN-DECISIONS-CHECKPOINT-2.md + per-surface FIELDS.md are the binding design reference; Phases 3-9 backend work is UNBLOCKED

Progress: [██████████] 100% (Phase 2: 15/15 plans — phase complete)

## Performance Metrics

**Velocity:** reset for v1.0 (prior POC velocity archived under 0.0.1).

- Total plans completed: 0
- Average duration: — min
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 1. Foundations, Spikes & CI | 0 | - | - |
| 2. DESIGN — All Mockups (★) | 0 | - | - |
| 3. Create Flow Backend | 0 | - | - |
| 4. Git Configuration Screen | 0 | - | - |
| 5. Identity Manager | 0 | - | - |
| 6. Global SSH Options | 0 | - | - |
| 7. Global Git Options | 0 | - | - |
| 8. Health + Fixer | 0 | - | - |
| 9. Upload / Credentials Assist | 0 | - | - |
| 10. Linux Validation + Release | 0 | - | - |

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

### Roadmap Evolution

- 2026-07-02: Prior build reframed as archived **0.0.1 POC** (never released) under `.planning/archive/0.0.1-poc-product-features-in-tui/`; phase numbering **reset** for the real v1.0. New 10-phase roadmap derived 1:1 from the PRD "Execution Phases" (Phase 0→1 … Phase 9→10). Existing Go packages are reusable substrate, not a behavior contract. Loop vehicle: `.planning/ONESHOT-LOOP-PROMPT.md`.

### Pending Todos

None yet.

### Blockers/Concerns

- 3 items intentionally open until their phase (documented in REQUIREMENTS.md "Still Open"): GSSH-01 dangerous-options list, KEY-01 catalog ordering/copy, screenshot-tooling mechanism (Phase 1 spike).
- Phase 2 VERIFICATION.md W1 (non-blocking): `insteadOf` URL rewriting (recipes/ wiring #3) is not rendered in either live demo — only an unused fixture constant. Cover it in Phase 4/7 design or document as a scoped divergence next to D9.
- Phase 2 VERIFICATION.md W2 (non-blocking): `internal/dummytui/nobackend_test.go` was deleted in 7453561 and never restored — the no-backend truth was re-proven directly (`go list -deps`) and `gate-no-backend-files` holds, but consider restoring an import-graph test before/during Phase 3.
- ~~02-14: fresh agent-ui-ux-designer critique of both live demos (DLV-02 exit gate) and the superpowers:requesting-code-review pass are outstanding~~ -- RESOLVED 2026-07-05: the orchestrator ran both reviews; findings F1-F11 are fixed/recorded (see 02-14-SUMMARY.md "Review findings resolution (post-plan fix pass)"). No outstanding blocker for 02-12 sign-off from 02-14.
- ~~02-15: a fresh agent-ui-ux-designer critique of both live demos (checkpoint-2 dimensions) and a fresh-context superpowers:requesting-code-review pass (against 02-15's must_haves + every task's acceptance_criteria) are OUTSTANDING orchestrator-run exit gates — the executor could not run them. Their CRITICAL/HIGH findings must be resolved before 02-12 (the DLV-08 re-presentation) proceeds. See 02-15-SUMMARY.md.~~ -- RESOLVED 2026-07-05: the orchestrator ran both reviews (a fresh agent-ui-ux-designer parity critique + a fresh-context code review). Findings F1-F10 (CRITICAL: F1 stale-tab closure breaking web view-switching; HIGH: F2 web ceremony/pane footers never mirrored; IMPORTANT: F3 TUI click-hijack from whole-line substring matching, F4 D9 email row swept into the baseline overlay in both media, F5 inline glyph literals, F9 web `a`-key precedence; MEDIUM/MINOR: F6/F7/F8/F10) plus one record-only item (re-measure the row budget, don't assume) are all fixed/recorded across two commits — see 02-15-SUMMARY.md "Review findings resolution (post-plan fix pass)". All 7 gates (go test -race, make test/lint/test-e2e/gate-no-backend-files, the copy-freeze greps, pnpm typecheck+build) are green. No outstanding blocker for 02-12 sign-off from 02-15.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260705-f9t | Add `make demo-web` target: relaunch the web mockup Vite dev server on dedicated port 45173 and open the browser | 2026-07-05 | 9ecfbb4 | [260705-f9t-add-make-target-to-relaunch-the-web-mock](./quick/260705-f9t-add-make-target-to-relaunch-the-web-mock/) |

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-07-08T02:19:36.033Z
Stopped at: Phase 6 context gathered
Resume file: .planning/phases/06-global-ssh-options/06-CONTEXT.md
