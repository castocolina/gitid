---
phase: 09-upload-credentials-assist
plan: 04
subsystem: upload-credentials-assist
tags: [tui, bubbletea, wizard, github-cli, gitlab-cli, upload, inventory]
requires:
  - phase: 09-upload-credentials-assist
    plan: 03
    provides: per-registration upload engine, provider inventory, failure classification
provides:
  - All four D-01 upload-checkbox states (ready, unauth, disabled, omitted) for both providers, hard-bound to one physical line
  - The complete testUpload beat: inventory-driven missing-type diff, per-type result rows, classified failure reasons, degraded/already-complete notes, the byte-identical manual-fallback block
  - The R7 announce-before-run two-message split (UploadStartedMsg then UploadRunMsg)
  - The R9 panic-as-defect policy (a recovered panic renders as a distinct gitid-defect row, never laundered into a provider-failure row)
  - D-17 post-upload confirmation with exactly one bounded retry, phase-aware (R8) so the confirmation-read count is provable apart from the pre-upload dedupe read
  - D-18's mechanical no-persisted-state proof (a before/after recursive HOME snapshot)
affects: [09-05, 09-06, 09-07, 09-08]
actuals:
  tasks: 3
  commits: 5
tech-stack:
  added: []
  patterns: [phase-tagged confirmation reads for provable retry counts, announce-then-execute two-message Bubble Tea beats, recovered-panic-as-distinct-row]
key-files:
  created: []
  modified:
    - internal/tuikit/identities.go
    - internal/tuikit/views.go
    - internal/tuikit/upload_section_test.go
    - internal/dummytui/fixturebackend.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go
    - e2e/create_flow_pty_e2e_test.go
key-decisions:
  - "The D-17 retry interval is 2s in production (uploadConfirmRetryInterval), reachable through a test-only uploadConfirmSleep override following the archiveClockNow precedent, chosen to comfortably absorb ordinary GitHub/GitLab read-after-write propagation lag without making the wizard feel stalled."
  - "An unconfirmed registration keeps Outcome=Uploaded (no fourth UploadRowOutcome value) and only gains an unconfirmed Reason — the provider genuinely accepted it; only the confirmation read hasn't caught up."
  - "Confirmation-read phase attribution uses a test-visible uploadPhase field the backend sets to dedupe/confirmation immediately before each of its two uploader.Inventory call sites, so a test fake can attribute each recorded invocation correctly (R8) without a raw total conflating the two reasons Inventory is called."
patterns-established:
  - "Two-message announce-then-execute Bubble Tea beat: UploadStartedMsg carries the commands about to run and is delivered BEFORE the subprocess executes; the caller's FollowUp() then runs it and delivers the terminal message."
  - "Recovered panics are marked as a distinct internal-defect row (Reason contains 'gitid internal defect'), never normalized into an ordinary provider-failure row."
requirements-completed: [UP-01, UP-02, UP-03]
coverage:
  - id: D1
    description: All four D-01 checkbox states render correctly for both providers, gated on one toggleUploadCheckbox() method, hard-bound to one physical line, legible with color stripped
    requirement: UP-01
    verification:
      - kind: unit
        ref: internal/tuikit/upload_section_test.go
        status: pass
    human_judgment: false
  - id: D2
    description: The complete testUpload beat (inventory diff, per-type rows, classified reasons, manual fallback, R7 two-message split, R9 panic policy)
    requirement: UP-02
    verification:
      - kind: unit
        ref: cmd/gitid/wiring_test.go
        status: pass
      - kind: e2e
        ref: e2e/create_flow_pty_e2e_test.go#TestCreateFlow_UploadAutonomousGitHubTracer
        status: pass
    human_judgment: false
  - id: D3
    description: D-17 post-upload confirmation with exactly one bounded retry (R8 phase-aware counting) and D-18's no-persisted-state snapshot proof
    requirement: UP-03
    verification:
      - kind: unit
        ref: cmd/gitid/wiring_test.go#TestPostUploadConfirmationRetriesExactlyOnce
        status: pass
      - kind: unit
        ref: cmd/gitid/wiring_test.go#TestNoPersistedUploadState
        status: pass
    human_judgment: false
completed: 2026-08-29
status: complete
---

# Phase 09 Plan 04: Complete Upload Section Summary

**The create wizard's autonomous-upload section now handles every combination of provider, tool state, and prior registration — checkbox states, the inventory-driven missing-type diff, classified failures, the byte-identical manual fallback, an observable announce-before-run beat, panic-safe defect reporting, and a phase-provable post-upload confirmation — without ever gating identity creation.**

## Accomplishments

- Completed all four D-01 checkbox states (ready, unauth, disabled, omitted) for github.com and gitlab.com, gated on one `toggleUploadCheckbox()` method for all four toggle affordances (space, arrow, `u`, click), hard-bound to one physical line at the wizard's detail-pane width.
- Completed `realBackend.RunUpload`'s full beat: resolves eligibility, stages the key (sharing `stagedKeyFor` with `TestStage1`), reads the provider inventory, computes the missing-type diff via `uploader.MissingRegistrations`, uploads only the missing set with per-registration requests carrying the product's D-07 title, and converts results through classified remediation reasons.
- Split the announce beat into two Bubble Tea messages (R7): `UploadStartedMsg` carries every command about to run and is delivered BEFORE any subprocess executes; its `FollowUp()` then runs `UploadKeys` and delivers the terminal `UploadRunMsg`.
- Wrapped the upload closure with a `recover()` that still reaches `UploadRunMsg` (never-gates) but marks a recovered panic as a distinct "gitid internal defect" row (R9), never laundered into an ordinary provider-failure row.
- Added D-17's post-upload confirmation: one `uploader.Inventory` read confirms every registration this run reported as uploaded/already-present is actually present (including the signing registration `ssh -T` cannot see), with one bounded retry (2s production interval) on a still-missing registration before marking it unconfirmed (Outcome stays Uploaded — no fourth `UploadRowOutcome` value).
- Made the confirmation-read count phase-aware (R8): a test-visible `uploadPhase` field distinguishes dedupe-phase from confirmation-phase `Inventory` calls, so `TestPostUploadConfirmationRetriesExactlyOnce` proves "exactly one bounded retry" without conflating it with the unrelated pre-upload dedupe read.
- Added `TestNoPersistedUploadState` (D-18): a recursive path+content-SHA256 snapshot of the seeded HOME before and after a full `RunUpload`, asserted byte-identical.

## Task Commits

- `a50b2ad` feat(09): complete upload checkbox states (Task 1)
- `48dc558` fix(09): serialize upload eligibility probes — a real data race found and fixed along the way: `UploadEligibility`'s memo mutex was released before the check-then-set was atomic, so concurrent probes for the same provider could all miss the cache and issue redundant `LookPath` calls. Fixed and covered by `TestUploadEligibilityMemoizesConcurrentProviderProbes` (8 concurrent goroutines, asserts exactly one `LookPath` call).
- `59b4886` feat(09): complete testUpload beat — inventory diff, per-type rows, R7/R9 (Task 2)
- `e807e8a` feat(09): D-17 post-upload confirmation with one bounded retry (Task 3)
- `ce70c0a` test(09): update the tracer PTY test for Task 2/3's completed upload beat — see Deviations below.

## Deviations from Plan

### 1. Execution required a hand-recovered resume (orchestrator, not the plan)

The first executor attempt completed the race-condition fix and Task 1's implementation but stopped before committing Task 1, blocked by a stale `golangci-lint` cache holding absolute-path references to a sibling worktree (`gitid-phase09-wave3`) the orchestrating session had already deleted during the prior wave's cleanup. Fixed via `golangci-lint cache clean` (an environment artifact, not a code defect). The orchestrator then verified and committed Task 1 itself (adding the one still-missing required test, `TestUploadCheckboxIsLegibleWithoutColor`) and dispatched a resume for Task 2/3, which completed both tasks and their full verification pass before running out of execution budget one step short of writing this SUMMARY — which the orchestrator completed.

### 2. `TestCreateFlow_UploadAutonomousGitHubTracer` needed updating, not a product fix

Post-Task-3 full-e2e verification found this Wave-2-era tracer test failing: it asserted the fake gh log held exactly 2 invocations (auth status + one `ssh-key add`), a count that predated this plan's Task 2 (which correctly uploads BOTH GitHub registrations, not one) and Task 3 (which correctly adds a post-upload confirmation read with one bounded retry). The real, new, CORRECT call count is 9: auth status (1) + the D-15 dedupe read (`uploader.Inventory` = 2 `api` calls) + `ssh-key add` × 2 (authentication + signing) + the D-17 confirmation read (2 `api` calls) + its one bounded retry (2 more, since the fake gh's "ok" fixture mode always answers an empty inventory, so confirmation never converges). Updated the test's assertions and doc comment to match the new, intended behavior; verified stable across 3 repeated runs.

## Security Notes

- No provider call sits outside a `tea.Cmd` closure (R3, inherited from 09-02); confirmed by re-reading every `uploader.Inventory`/`uploader.UploadKeys` call site in `RunUpload`/`confirmUpload`.
- A recovered panic's redacted text still routes through `RedactCLIOutput` — the same bounded, token-free, home-path-free treatment every other last-resort reason gets.
- `TestNoPersistedUploadState` is the mechanical enforcement that this plan introduces zero new persisted state, closing off the exact class of footgun (`Doctor reserved-block false-positive loop`) a derived-state file would otherwise risk.

## Next Phase Readiness

- Plans 09-05 (CLI) and 09-06 (Identity Manager) can drive the same `RunUpload`/`UploadEligibility` surface the wizard now uses in full.
- Plan 09-07 (divergence classification) has a concrete, complete real-vs-fixture behavior to classify.
- Plan 09-08 (the one real-account wave) can rely on the confirmation/retry/phase-tagging machinery this plan built being already proven against fakes.

---
*Phase: 09-upload-credentials-assist*
*Completed: 2026-08-29*
