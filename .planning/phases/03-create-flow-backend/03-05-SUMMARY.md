---
phase: 03-create-flow-backend
plan: 05
subsystem: create-flow-tui
tags: [tuikit, bubbletea, tester, connectivity-test, clipboard, git-step, backend-seam, d-02, d-03, d-18, d-19]

# Dependency graph
requires:
  - phase: 03-03
    provides: "cmd/gitid/wiring.go composition root's toTestResultView/recordOutcome/storeUnlocked/CopyPublicKey (already mapping tester outcomes -> tuikit.TestOutcome and gating the D-01 store); internal/tuikit render stack + Backend seam + view DTOs from 03-02"
  - phase: 03-04
    provides: "the wizard's step-0/step-1/step-2 render structure this plan extends (stage1/stage2 TestResultView fields, wizardFooter, gitFocus button row)"
provides:
  - "D-02: TestOutcomeReachableNotUploaded renders the frozen yellow `! Reachable — key not uploaded yet` warning (never red), replacing the pass/failure outcome row on whichever stage answered it"
  - "D-04: stage 2 chains on Enter after EITHER a PASS or a ReachableNotUploaded stage-1 outcome (succeededOutcome helper) — a hard Failure is the only outcome that stops the wizard"
  - "TEST-01/02: stage1Cmd()/stage2Cmd() prefer the CAPTURED TestResultView.Command once a stage has answered — shown==run byte-for-byte, not a freshly recomputed string"
  - "D-01: reviewCeremony()'s heading/ResultMessage switch to the frozen `Stored — key not uploaded yet; this identity is not proven for Git yet` copy whenever either stage answered ReachableNotUploaded (wizardModel.keyUnused())"
  - "D-03: a Theme.Hint line naming the provider's key-settings page + a footer/keybar copy-.pub action, ReachableNotUploaded-only (absent at PASS/Failure); the real clipboard seam receives ONLY the .pub path"
  - "D-19: new Backend.GitStepDisabledReason() seam — the real binary ALWAYS disables the wizard's Git-step [ Continue ] with its own frozen reason; the dummy keeps its unchanged validity-gated reason"
  - "D-18: Skip Git proven functional end-to-end through the real seam — persistCreate never calls PersistGitconfig in Phase 3, so a create writes SSH artifacts only regardless of what Git fields the identity carries"
affects:
  - "03-06 (visual-regression allowlist for the new D-02/D-03 warning-state render + the D-19 real-binary git-step golden diff; raw-keystroke PTY proof of the warning/copy/skip paths)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "succeededOutcome(o TestOutcome) bool centralizes the D-01/D-04 'this outcome unlocks the store + chains the wizard' predicate — PASS and ReachableNotUploaded both true, only Failure false"
    - "Captured-command precedence: stage1Cmd()/stage2Cmd() return w.stageN.Command once populated, falling back to a freshly-computed Backend call only before the stage has run — closes TEST-01/02 shown==run without a second source of truth"
    - "Row-budget discipline for a new warning affordance: the copy-.pub action is a FOOTER/keybar entry (wizardFooter), never a second inline body row — the hint line is the ONLY new body row the D-02/D-03 warning state costs, and its copy (providerKeySettingsLabel) is kept deliberately short to fit the fixed 100x30 pane"
    - "gitContinueGate() (reason string, enabled bool) is the single choke point every Git-step Continue call site (render, stepForward, handleWizardKey enter/left-right, blockedForwardNote) routes through — a Backend method addition (GitStepDisabledReason) synchronized across all three tuikit.Backend implementers per the established 03-04 pattern"

key-files:
  created: []
  modified:
    - internal/tuikit/identities.go
    - internal/tuikit/backend.go
    - internal/tuikit/identities_test.go
    - internal/tuikit/backend_stub_test.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go
    - internal/dummytui/fixturebackend.go
    - Makefile
    - .planning/phases/03-create-flow-backend/deferred-items.md

key-decisions:
  - "The copy-.pub action (D-03) is rendered as a footer/keybar affordance only, never a second inline body row — discovered via a real row-budget overflow (the wizard's fixed 100x30 pane clipped the stage-2 block once the warning state grew by 2 body rows instead of 1), matching the plan's own 'footer/keybar action' wording and the 03-04 row-budget precedent"
  - "providerKeySettingsLabel's copy was kept deliberately SHORT ('GitHub key settings' not 'GitHub → Settings → SSH and GPG keys') for the same row-budget reason — Claude's Discretion per 03-CONTEXT.md ('Exact copy for the new warning-state line... draft during the UI wave')"
  - "The D-01 key-unused frozen result copy fully REPLACES the normal ResultMessage/Heading (not appended) for a ReachableNotUploaded-derived store, per the plan's explicit 'no silent success framing' requirement — the identity taxonomy's own 'key-unused' 8-state label (internal/identity/state.go) is NOT touched; a freshly-created identity's Host block genuinely references its key, so forcing that unrelated classification would be wrong. The frozen copy is a WIZARD/ceremony-level acknowledgment only"
  - "All three tasks landed in ONE commit (c680fde) rather than three — CLAUDE.md's 'let the buildable boundary, not file count, set the commit granularity' takes precedence over the plan's per-task default, because the code for all three tasks is tightly intermixed inside the same shared functions (renderStageOutcome, gitContinueGate, the Makefile gate) and a clean per-task split would not compile in isolation"

patterns-established:
  - "Backend method additions still require synchronized updates across all THREE tuikit.Backend implementers (cmd/gitid/wiring.go, internal/dummytui/fixturebackend.go, internal/tuikit/backend_stub_test.go) — GitStepDisabledReason followed this exactly, with stubBackend gaining two zero-value-safe fields (gitStepAlwaysDisabled/gitStepReason) so every EXISTING stubBackend{} call site keeps its prior behavior unchanged"

requirements-completed: [TEST-01, TEST-02]

# Metrics
duration: "~1 session"
completed: 2026-08-17
---

# Phase 3 Plan 05: Connectivity-Test Outcome Screens + Scoped Divergences Summary

**ReachableNotUploaded now renders as an honest yellow warning (never red) with a captured-command shown==run guarantee, a footer copy-.pub affordance, and a frozen "not proven for Git yet" store receipt; the real binary's Git-step Continue is unconditionally, honestly disabled while Skip Git is proven SSH-only end to end.**

## Performance

- **Tasks:** 3 completed in 1 session
- **Files modified:** 9 (identities.go, backend.go, identities_test.go, backend_stub_test.go, wiring.go, wiring_test.go, fixturebackend.go, Makefile, deferred-items.md)

## Accomplishments

- **Task 1 (D-02, TEST-01/02, Pitfall 6):** `succeededOutcome()` now treats `TestOutcomePass` and `TestOutcomeReachableNotUploaded` identically for both the D-04 stage-1→stage-2 chain and the D-01 store gate — only a hard `TestOutcomeFailure` stops the wizard. `renderStageOutcome()` replaces the outcome row with the frozen yellow `! Reachable — key not uploaded yet` line (`Theme.Warning`, existing `!` glyph) on that outcome, never the red `✗` — red is now reserved exclusively for `TestOutcomeFailure`, rendered from the REAL captured `Detail` output rather than a hand-built "Permission denied" string. `stage1Cmd()`/`stage2Cmd()` now prefer the captured `TestResultView.Command` once a stage has answered, closing TEST-01/02's shown==run contract byte-for-byte. `reviewCeremony()`'s heading and `ResultMessage` switch to the new frozen `Stored — key not uploaded yet; this identity is not proven for Git yet` copy whenever `wizardModel.keyUnused()` is true (either stage answered ReachableNotUploaded) — no silent "success" framing over an unauthenticated key.
- **Task 2 (D-03):** At ReachableNotUploaded, ONE `Theme.Hint` line names the provider's key-settings page (`providerKeySettingsLabel`, a short presentational lookup table mirroring the existing `wizardProviders` list — never a backend call). The copy-.pub action is offered as a footer/keybar entry only (never a second inline body row, to stay inside the fixed 100×30 row budget); `copyable()` gates both the `c` key and the footer entry to ReachableNotUploaded on whichever stage is currently shown, and is explicitly false at PASS and at a hard Failure. A dedicated `recordingCopyBackend` test proves the clipboard seam receives exactly the `.pub` path.
- **Task 3 (D-19/D-18):** New `Backend.GitStepDisabledReason() (reason string, alwaysDisabled bool)` seam, implemented identically across all three `tuikit.Backend` implementers. The real binary (`cmd/gitid/wiring.go`) always returns `("— Git configuration arrives with the next build", true)`; the dummy and every `internal/tuikit` test double return `("", false)`, preserving the UNCHANGED validity-gated `— needs user.name + a valid email` reason. `wizardModel.gitContinueGate()` centralizes the enabled/reason decision and every Continue call site (render, `stepForward`, both `handleWizardKey` branches, `blockedForwardNote`) now routes through it — the real binary's Continue is provably unreachable even with a fully valid Git form. Skip Git was already structurally SSH-only in the real backend (`persistCreate` never calls `PersistGitconfig` in Phase 3); a new wiring test proves this end-to-end even when the identity carries populated Git fields, closing the loop that D-19 forcing Continue off makes Skip the only functional path forward.

## Task Commits

All three tasks landed in one commit — see "Decisions Made" for the CLAUDE.md-driven rationale.

1. **Tasks 1+2+3: D-02 warning render + exact-command shown==run + D-03 copy-pub hint + D-19/D-18 real git-step** - `c680fde` (feat)

**Plan metadata:** (this commit — SUMMARY + STATE + ROADMAP)

## Files Created/Modified

- `internal/tuikit/identities.go` - `succeededOutcome`, `copyable`, `keyUnused`, `providerKeySettingsLabel`, `renderStageOutcome`, `gitContinueGate`, captured-command `stage1Cmd`/`stage2Cmd`, `blockedForwardNote` D-19 branch, `reviewCeremony` frozen-copy branch, `wizardFooter` copy-action entries
- `internal/tuikit/backend.go` - `Backend.GitStepDisabledReason()` interface method
- `internal/tuikit/identities_test.go` - rewrote `TestWizardSimulateFailToggleAndRetry` for the corrected warning behavior; added `TestWizardHardFailureRendersRedAndRetriesNoCopy`, `TestReachableNotUploadedStoresKeyUnusedCopy`, `TestCopyPubSeamReceivesOnlyThePubPath`, `TestFrozenReachableWarningAndKeyUnusedCopy`, `TestWizardGitContinueForcedDisabledByBackend`; extended `TestWizardTestStageCommandsAndFlagOrder` with the shown==run assertion + the PASS-state copy-absence assertion
- `internal/tuikit/backend_stub_test.go` - `stubBackend.GitStepDisabledReason()` + configurable `gitStepAlwaysDisabled`/`gitStepReason` fields (zero-value-safe)
- `cmd/gitid/wiring.go` - `gitStepDisabledReason` const + `realBackend.GitStepDisabledReason()`
- `cmd/gitid/wiring_test.go` - `TestGitStepDisabledReasonIsAlwaysDisabledInRealBinary`, `TestPersistSkipGitWritesSSHOnlyNoGitArtifacts`
- `internal/dummytui/fixturebackend.go` - `FixtureBackend.GitStepDisabledReason()` + comment accuracy fix on `CopyPublicKey`
- `Makefile` - `gate-copy-freeze` gains the two D-02/D-01 strings plus a separate `cmd/gitid`-scoped grep for the D-19 real string
- `.planning/phases/03-create-flow-backend/deferred-items.md` - re-flagged the 03-04-carried algorithm-catalog item as still open (not in 03-05's scope) and newly logged the `PersistError()` render gap (also out of scope)

## Decisions Made

See `key-decisions` in frontmatter. In short: the copy-.pub action is footer-only (not a body row) after a real row-budget overflow surfaced during testing; the warning-state hint copy was kept deliberately short for the same reason; the D-01 key-unused ceremony copy is a wizard-level acknowledgment, not a forced change to the unrelated 8-state identity taxonomy; and all three tasks were committed together per CLAUDE.md's buildable-boundary commit-granularity rule rather than split into three commits that would not compile in isolation.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Row-budget overflow silently clipped the stage-2 prompt once the warning state grew by 2 body rows**

- **Found during:** Task 2, writing the copy-.pub + hint tests
- **Issue:** The initial implementation rendered the warning line, the hint line, AND an inline `c Copy public key` body row (3 rows total vs. the original 1-row PASS/Failure outcome line). In the fixed 100×30 wizard pane this pushed the "Run stage 2 (Enter)" / `✓ identityfile …` line entirely off the rendered terminal content — not a crash, but a silently missing affordance a real user would never see.
- **Fix:** Moved the copy-.pub action to a footer/keybar-only entry (matching the plan's own "a footer/keybar action" wording for D-03) and shortened `providerKeySettingsLabel`'s copy, bringing the warning state's net cost down to 1 new body row (the hint line) instead of 2.
- **Files modified:** `internal/tuikit/identities.go`
- **Verification:** `TestWizardSimulateFailToggleAndRetry` (rewritten) now completes the full stage-1-warning → stage-2-pass chain and asserts the `✓ identityfile …` line is actually rendered.
- **Committed in:** `c680fde`

**2. [Rule 1 - Bug] Test assertions against wrapped/footer text were checking the wrong render region**

- **Found during:** Task 1/2, debugging the new tests
- **Issue:** Long wrapped body text (e.g. the hard-failure `Connection refused` line) breaks across physical terminal rows, so a raw `appView(a)` substring match silently fails even when the content is correct; conversely, the flattened `paneFlat(a)` helper column-slices to the detail-pane region only and excludes the full-width footer row, so footer-only text (like the copy-public-key keybar entry) is invisible to it.
- **Fix:** Used `paneFlat(a)` for wrapped body-content assertions and raw `appView(a)` for footer/keybar assertions, matching each helper to the region it actually covers.
- **Files modified:** `internal/tuikit/identities_test.go`
- **Verification:** All rewritten/new tests pass; `go test -race ./internal/tuikit/...` green (187 tests).
- **Committed in:** `c680fde`

---

**Total deviations:** 2 auto-fixed (both Rule 1 bugs surfaced by this plan's own new tests, not pre-existing)
**Impact on plan:** Both fixes are correctness requirements for the plan's own new behavior (the warning state must actually be visible and testable) — no scope creep.

## Issues Encountered

- The plan's Task 3 action asked for a Makefile grep scoped to `internal/dummytui` proving the dummy's original reason string is unchanged. Architecturally, `internal/tuikit`'s `gitFormDisabledSuffix` constant is the SOLE owner of that string (the dummy's `FixtureBackend.GitStepDisabledReason()` returns `("", false)`, letting tuikit fall back to its own constant) — duplicating the literal string into `internal/dummytui` for no functional reason would be decorative, not a real invariant. The existing `gate-copy-freeze` grep against `internal/tuikit` already proves the string is present and unchanged (it is asserted there both before and after this plan); the NEW D-19 real string gets its own grep scoped to `cmd/gitid` (where it actually lives). This is a reasoned, documented divergence from the plan's literal grep-path wording, not a correctness gap.
- Could not engage `/mui` or `agent-ui-ux-designer` on the new warning-state visual — this executor has no access to spawn sub-agents or slash commands (Read/Write/Edit/Bash tools only). Flagging for the orchestrator, matching the project's established pattern where certain review/design gates are run by the orchestrator rather than the plan executor (e.g. 02-14/02-15's "ORCHESTRATOR-run exit gates").

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The connectivity-test outcome screens (D-02/D-03) and the two remaining Phase-3 scoped divergences (D-18/D-19) are fully wired through the real backend seam, with the tuikit/backend-free boundary intact (`go list -deps ./internal/tuikit` has no first-party backend import; `TestNoBackendAllowlist` green).
- Carried into 03-06 (visual-regression wave): the new D-02/D-03 warning-state render and the D-19 real-binary git-step golden text are NOT yet in the visual-regression allowlist — the approved Phase 2 dummy goldens never exercised either state (D-02 is a new outcome the demo never had; D-19's real-binary string only exists in `cmd/gitid`'s own capture). 03-06 needs either allowlist entries or equivalent dummy-side fixture states. A raw-keystroke PTY proof of the warning/copy/skip paths through the REAL binary is also still owed per the project's L2 closure contract (a unit/wiring test never substitutes for a live PTY proof).
- Still outstanding (re-flagged, not newly introduced by this plan — see `deferred-items.md`): `realBackend.PersistError()` is exposed but nothing in `internal/tuikit` reads it after a committed write, so a real write failure currently shows the same "created" note as a success. Candidate owner: 03-06 or a dedicated fix pass.
- No blockers for 03-06.

---
*Phase: 03-create-flow-backend*
*Completed: 2026-08-17*

## Self-Check: PASSED

- `.planning/phases/03-create-flow-backend/03-05-SUMMARY.md` — FOUND
- commit `c680fde` (Tasks 1+2+3) — FOUND
- commit `2c4e328` (SUMMARY docs) — FOUND
- `internal/tuikit/backend.go` contains `GitStepDisabledReason` — FOUND
- `cmd/gitid/wiring.go` contains `gitStepDisabledReason` const — FOUND
