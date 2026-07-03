---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: executing
stopped_at: Completed 01-04-PLAN.md
last_updated: "2026-07-03T01:48:25.071Z"
last_activity: 2026-07-03 -- Phase 01 execution started
progress:
  total_phases: 10
  completed_phases: 0
  total_plans: 7
  completed_plans: 4
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-02)

**Core value:** Managing a Git identity produces coordinated, coherent SSH + Git artifacts that are proven to authenticate and resolve correctly (`ssh -G`) before any file is written, and existing hand-written config is never corrupted.
**Current focus:** Phase 01 — foundations-spikes-ci

## Current Position

Phase: 01 (foundations-spikes-ci) — EXECUTING
Plan: 5 of 7
Status: Ready to execute
Last activity: 2026-07-03 -- Phase 01 execution started

Progress: [░░░░░░░░░░] 0%

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

Last session: 2026-07-03T01:48:25.063Z
Stopped at: Completed 01-04-PLAN.md
Resume file: None
