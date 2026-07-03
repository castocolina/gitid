---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: executing
stopped_at: Completed 02-06-PLAN.md
last_updated: "2026-07-03T10:37:40.599Z"
last_activity: 2026-07-03 -- Phase 02 execution in progress (02-05 complete)
progress:
  total_phases: 10
  completed_phases: 1
  total_plans: 19
  completed_plans: 13
  percent: 10
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-02)

**Core value:** Managing a Git identity produces coordinated, coherent SSH + Git artifacts that are proven to authenticate and resolve correctly (`ssh -G`) before any file is written, and existing hand-written config is never corrupted.
**Current focus:** Phase 02 — design-all-mockups-checkpoint-1

## Current Position

Phase: 02 (design-all-mockups-checkpoint-1) — EXECUTING
Plan: 6 of 12
Status: Executing — 02-05 complete (git-screen fan-out surface: MUI mockup + TUI dummy + capture + 0-unresolved parity, both media, 7 states); next is Wave 4 fan-out continuation (02-06 identity-manager through 02-10)
Last activity: 2026-07-03 -- Phase 02 execution in progress (02-05 complete)

Progress: [██████░░░░] 63%

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

### Roadmap Evolution

- 2026-07-02: Prior build reframed as archived **0.0.1 POC** (never released) under `.planning/archive/0.0.1-poc-product-features-in-tui/`; phase numbering **reset** for the real v1.0. New 10-phase roadmap derived 1:1 from the PRD "Execution Phases" (Phase 0→1 … Phase 9→10). Existing Go packages are reusable substrate, not a behavior contract. Loop vehicle: `.planning/ONESHOT-LOOP-PROMPT.md`.

### Pending Todos

None yet.

### Blockers/Concerns

- 3 items intentionally open until their phase (documented in REQUIREMENTS.md "Still Open"): GSSH-01 dangerous-options list, KEY-01 catalog ordering/copy, screenshot-tooling mechanism (Phase 1 spike).

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-07-03T10:37:40.591Z
Stopped at: Completed 02-06-PLAN.md
Resume file: None
