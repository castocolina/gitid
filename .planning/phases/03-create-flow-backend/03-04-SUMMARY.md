---
phase: 03-create-flow-backend
plan: 04
subsystem: create-flow-tui
tags: [tuikit, bubbletea, keygen, identity, sshconfig, backend-seam, reuse-key, d-09, d-10, d-12, d-13, d-16, d-20, d-21]

# Dependency graph
requires:
  - phase: 03-03
    provides: "cmd/gitid/wiring.go real composition root (AliasCollision, ScanReusableKeys, keyOwners, ReadPub seam); internal/tuikit render stack + Backend seam + view DTOs from 03-02"
provides:
  - "D-16 Preview banner (chrome row, Theme.Warning) on every not-yet-wired tab, absent on Identities"
  - "SSHUI-01/03/D-20/D-21: provider-reactive autofill (full known-provider table via identity.DefaultHostname/DefaultPort) + recipe-faithful live Host-block preview + unknown-provider alt-SSH hint"
  - "D-09: alias-collision block against ALL parsed Host patterns (managed + hand-written), Include-aware"
  - "KEY-06/D-10/D-12/D-13: reuse-existing-key picker (scan + manual-path row + in-use label + non-catalog note), routing the create through the reused key instead of generating"
  - "internal/identity.StageReuse — the ensurePub staging half of Reuse, extracted for staged (test-before-write) callers"
  - "internal/keygen.ScanManualKey — symlink-rejecting single-candidate key scan for the picker's manual-path row"
  - "cmd/gitid/wiring.go Backend.ManualReusePath + D-12 InUseBy format '<identity> (<provider-host>)'"
  - "Bugfix: internal/sshconfig.ParseManagedHosts now skips ALL IsReservedBlockName blocks, not just _global"
affects:
  - "03-05 (surfaces realBackend.PersistError() and the ReachableNotUploaded warning state)"
  - "03-06 (raw-keystroke PTY e2e proof of the reuse-picker + alias-collision + D-16 banner wiring; visual-regression allowlist for the new picker render)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Backend-free reuse picker: tuikit renders []tuikit.ReusableKeyView only; the keygen.ReusableKey -> tuikit.ReusableKeyView conversion + D-12 InUseBy label formatting live exclusively in cmd/gitid/wiring.go"
    - "Combined-header row-budget trick: a new toggle group shares its header row with the section it selects between (generate vs reuse), instead of costing a second dedicated header row, to stay inside the 100x30 wizard pane"
    - "Staging-vs-pipeline split: identity.StageReuse extracted from identity.Reuse so a staged, test-before-write caller (the TUI wizard) can reuse the SAME ensurePub logic without running Reuse's write pipeline"
    - "os.Lstat symlink rejection for user-pointed paths (manual-path row) vs os.Stat-following for the trusted ~/.ssh directory scan — two different trust levels, two different keygen entry points"

key-files:
  created: []
  modified:
    - internal/tuikit/frame.go
    - internal/tuikit/identities.go
    - internal/tuikit/backend.go
    - internal/tuikit/views.go
    - internal/tuikit/store.go
    - internal/tuikit/identities_test.go
    - internal/tuikit/backend_stub_test.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go
    - internal/dummytui/fixturebackend.go
    - internal/identity/modes.go
    - internal/keygen/keyscan.go
    - internal/keygen/keyscan_test.go
    - internal/sshconfig/reader.go
    - internal/sshconfig/reader_test.go
    - Makefile

key-decisions:
  - "D-09 (alias-collision block) was found ALREADY fully wired by 03-03 (backend) + this plan's own Task 2 (tuikit gate + test) — Task 3's real remaining scope was the reuse-key picker; no new alias-collision code was needed, only verified"
  - "The generate-vs-reuse toggle and whichever body follows it (algorithm radios or the reuse picker) share ONE combined header row, not two, to stay inside the wizard step-0 pane's 100x30 budget — discovered via a real row-budget test regression, not assumed up front"
  - "identity.Reuse's ensurePub logic was extracted into an exported StageReuse rather than duplicated, so the wizard's staged two-test-stage flow and the single-shot Reuse function can never diverge on the D-11 encrypted-key rule"
  - "The D-12 'in use by' label format changed from a bare identity name to '<identity> (<provider-host>)' in wiring.go, the dummy FixtureBackend, and the tuikit stub — a coordinated three-implementer change, all three Backend implementers of tuikit.Backend"

patterns-established:
  - "Backend method additions (e.g. ManualReusePath) require synchronized updates across all THREE tuikit.Backend implementers: cmd/gitid/wiring.go (real), internal/dummytui/fixturebackend.go (design demo), internal/tuikit/backend_stub_test.go (internal unit-test stub) — grep 'var _ tuikit.Backend =' / 'var _ Backend =' to find all three before touching the interface"

requirements-completed: [SSHUI-01, SSHUI-02, SSHUI-03, KEY-06]

# Metrics
duration: "Task 1 ~1 session (2026-07-25); Task 2 ~1 session (2026-08-17); Task 3 ~1 session (2026-08-17)"
completed: 2026-08-17
---

# Phase 3 Plan 04: SSH Form Correctness + D-16 Demo Banner Summary

**Real-binary SSH create form (provider-reactive autofill, recipe-faithful live preview, alias-collision block) plus a KEY-06 reuse-existing-key picker that routes the create through an existing key instead of generating — all three tuikit.Backend implementers (real, dummy, test stub) kept in lockstep.**

## Performance

- **Tasks:** 3 completed across 3 sessions (multi-session plan; Tasks 1/2 landed in prior sessions, Task 3 this session)
- **Files modified:** 16 total across the plan (6 for Task 1, 3 for Task 2, 14 for Task 3, with `internal/tuikit/identities.go` and `cmd/gitid/wiring.go`/`wiring_test.go` touched by more than one task)

## Accomplishments

- **Task 1 (D-16):** every not-yet-wired tab carries a persistent, honest "Preview — demo data, not wired to your system yet" chrome-row banner (Theme.Warning, existing `!` glyph); the Identities tab (which Phase 3 wires) carries none. Copy-frozen via the `gate-copy-freeze` Makefile target.
- **Task 2 (SSHUI-01/03, D-20/D-21):** the real SSH form's Real-hostname/Port autofill re-resolves on EVERY keystroke of the SSH Host field from the Backend's provider-defaults seam (`identity.DefaultHostname`/`DefaultPort` for the real binary), not just on blur; a manually edited alias's own suffix wins over the provider field once touched; an unknown provider keeps port 22 with an inline, never-a-new-row hint explaining why.
- **Task 3 (D-09, KEY-06/D-10/D-12/D-13):**
  - Verified D-09's alias-collision block is fully wired end-to-end (backend `AliasCollision` Include-aware against ALL parsed Host patterns; tuikit `nameTaken`/`step0Valid` blocks advance with an inline error naming the conflict) — this was already complete from 03-03 + this plan's Task 2, confirmed via `TestAliasCollisionIsIncludeAware` and `TestWizardDuplicatePrefixBlocksNext`.
  - Built the reuse-existing-key picker: a D-10 "Generate a new key" / "Reuse an existing key" toggle in the create wizard's step 0, sharing one combined header row with whichever body follows (row-budget discipline). Reuse mode lists every `tuikit.ReusableKeyView` the Backend's key-scan seam returns (filename + algorithm + fingerprint), an "in use by: `<identity>` (`<provider>`)" label for the highlighted row (D-12), a non-blocking informational note for a non-catalog algorithm (D-13), and a trailing manual-path row with inline Backend-validated error feedback.
  - Selecting a candidate routes the create through `CreateSpec.ReuseKeyPath`; `wizardModel.keyPath()`/`spec()` resolve to the picked key everywhere (preview, test-stage commands, the committed write) instead of a generated path. `step0Valid` blocks advance until the reuse selection actually resolves to a usable key — silently falling back to "generate under a name the user never asked for" was rejected as a correctness gap (Rule 2).
  - A same-provider reuse selection (the form's current SSH Host suffix appears in the candidate's InUseBy label) shows an advisory warning but never blocks (D-12); an encrypted key with an existing `.pub` sibling is a normal, selectable entry (D-11/KEY-06), proven through the REAL constructor end to end.
  - `internal/identity.Reuse`'s ensurePub logic was extracted into an exported `StageReuse` (no write pipeline) so the wizard's staged two-test-stage flow shares the exact same D-11 logic Reuse itself uses.
  - `internal/keygen.ScanManualKey` applies `os.Lstat` symlink rejection to the manual-path row's candidate before parsing (T-03-13) — deliberately stricter than the directory scan, which legitimately follows symlinks inside the user's own `~/.ssh`.
  - `cmd/gitid/wiring.go` gained `Backend.ManualReusePath` and reformatted the D-12 `keyOwners()` label to `"<identity> (<provider-host>)"`, coordinated across all three `tuikit.Backend` implementers (real, dummy, tuikit-internal test stub).

## Task Commits

1. **Task 1: D-16 Preview banner on not-yet-wired tabs** - `76fb231` (feat)
2. **Task 2: Provider-reactive autofill + recipe-faithful live Host preview (SSHUI-01/03, D-20/D-21)** - `cc1f614` (feat)
3. **Task 3: Alias-collision block + reuse-existing-key picker (D-09, KEY-06/D-10/D-12/D-13)** - `5df3e2d` (feat)

**Plan metadata:** (this commit — SUMMARY + STATE + ROADMAP)

## Files Created/Modified

- `internal/tuikit/frame.go` - D-16 banner render (Task 1)
- `internal/tuikit/identities.go` - provider-reactive autofill (Task 2); D-10 key-source toggle + reuse picker, step0Valid reuse gate, spec()/keyPath()/finishIdentity ReuseKeyPath threading (Task 3)
- `internal/tuikit/backend.go` - `Backend.ManualReusePath` interface method (Task 3)
- `internal/tuikit/views.go` - `CreateSpec.ReuseKeyPath` (Task 3)
- `internal/tuikit/store.go` - `DemoIdentity.ReuseKeyPath` (Task 3)
- `internal/tuikit/identities_test.go` - provider autofill tests (Task 2); 8 new reuse-picker tests + 3 focus-renumbering fixes (Task 3)
- `internal/tuikit/backend_stub_test.go` - `stubBackend.ManualReusePath` + fixture reuse-key data with an encrypted and a non-catalog-algorithm row (Task 3)
- `cmd/gitid/wiring.go` - `Backend.ManualReusePath`, D-12 `keyOwners()` label format, `stagedKeyFor` reuse routing (Task 3)
- `cmd/gitid/wiring_test.go` - 5 new reuse-picker/collision tests (Task 3)
- `internal/dummytui/fixturebackend.go` - `ManualReusePath` + D-12 label format parity with the real backend (Task 3)
- `internal/identity/modes.go` - `StageReuse` extracted from `Reuse` (Task 3)
- `internal/keygen/keyscan.go` - `ScanManualKey` (Task 3)
- `internal/keygen/keyscan_test.go` - 4 new `ScanManualKey` tests (Task 3)
- `internal/sshconfig/reader.go` - `ParseManagedHosts` reserved-block fix (Task 3 deviation)
- `internal/sshconfig/reader_test.go` - regression test for the fix (Task 3 deviation)
- `Makefile` - `gate-copy-freeze` target (Task 1)

## Decisions Made

See `key-decisions` in frontmatter. In short: D-09 needed no new code (already wired); the key-source/reuse-picker header was merged into one row to stay inside the wizard's fixed row budget; `identity.Reuse` was split into a staging half (`StageReuse`) and its existing write pipeline so the TUI's staged test-then-write flow can share the encrypted-key logic instead of duplicating it; the D-12 label format was coordinated across all three Backend implementers in one pass.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `sshconfig.ParseManagedHosts` surfaced a phantom "ssh-include" identity**

- **Found during:** Task 3, writing `TestReuseEncryptedKeyWithExistingPubSucceeds`
- **Issue:** `ParseManagedHosts` special-cased only the literal `"_global"` block name when deciding what to skip, even though `sshconfig.IsReservedBlockName` already registers BOTH `_global` and `ssh-include` as reserved. `internal/identity/loader.go`'s `Reconstruct` has no other exclusion chokepoint for reserved blocks, so the very first create on a fresh D-06 machine — which writes the `ssh-include` Include-line block into `~/.ssh/config` — caused the reconstructed identity list to contain a phantom `"ssh-include"` row alongside the real identity.
- **Fix:** `ParseManagedHosts` now skips any block for which `IsReservedBlockName` returns true, not just the `_global` literal.
- **Files modified:** `internal/sshconfig/reader.go`, `internal/sshconfig/reader_test.go` (new `TestParseManagedHosts_SSHIncludeSkipped` regression test; the existing `TestParseManagedHosts_GlobalSkipped` still passes unchanged, proving the fix is additive, not a behavior swap).
- **Verification:** `go test -race ./internal/sshconfig/...` and the full module suite (`go test -race ./...`, 826 passed) both green; `TestReuseEncryptedKeyWithExistingPubSucceeds` now asserts exactly one identity after a real committed create.
- **Committed in:** `5df3e2d` (part of the Task 3 commit — the teardown/fix and the feature that surfaced it landed together per CLAUDE.md's "let the buildable boundary set commit granularity").

**2. [Rule 2 - Missing critical validation] `step0Valid` blocks advance on an unresolved reuse selection**

- **Found during:** Task 3, designing the picker's manual-path row
- **Issue:** Without an explicit check, choosing "Reuse an existing key" and leaving the manual-path row empty (or unresolved) would let `w.reuseKeyPath()` return `""`, and `w.keyPath()` would then silently fall back to the GENERATE path — creating a fresh key under a name the user explicitly chose NOT to generate, with no error shown.
- **Fix:** `step0Valid` now returns `false` when `keySource == keySourceReuse` and `reuseKeyPath() == ""`, blocking advance until the reuse selection is actually usable (a scanned entry, or a manual path the Backend has validated).
- **Files modified:** `internal/tuikit/identities.go`
- **Verification:** `TestReusePickerManualPathRow` asserts the empty-manual-path case blocks and the resolved case unblocks.
- **Committed in:** `5df3e2d`

---

**Total deviations:** 2 auto-fixed (1 bug, 1 missing critical validation)
**Impact on plan:** Both fixes are correctness requirements surfaced by the new tests this task added, not scope creep — the alternative in each case was silently wrong behavior (a phantom identity row; a reuse choice that silently generated instead).

## Issues Encountered

- Inserting the D-10 key-source toggle as its own header row initially pushed the live Host-block preview's `IdentitiesOnly yes` line off the fixed 100x30 wizard pane, failing `TestLivePreviewIsRecipeFaithful`. Resolved by merging the key-source toggle and the section body (algorithm radios or reuse picker) under ONE combined header row instead of two, restoring generate-mode's row count to exactly what it was before this task and confining the net row growth to reuse mode only (a new state that did not exist before).
- Renumbering the algorithm select from focus slot 5 to slot 6 (to make room for the key-source toggle at slot 5) broke three pre-existing focus-index-hardcoded tests (`TestWizardSKAlgorithmsDisabledWithRationale`, `TestWizardArrowKeyPrecedenceStep0`, `TestWizardAlgorithmSelectionMirrorsStrategyFocusAccent`). Updated their expected focus constants/tab counts; no assertion was weakened, only the numeric expectation of an intentional focus-ring renumbering.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The create-flow SSH form is now correctness-complete against the real backend for Phase 3's scope: provider autofill, recipe-faithful preview, alias collision, and the reuse-existing-key picker all exercise real `internal/identity`/`internal/keygen`/`internal/sshconfig` logic through the `cmd/gitid/wiring.go` composition root.
- Carried into 03-06 (per 03-03's hand-off, still open): the raw-keystroke PTY proof that the reuse-existing-key path (case 6, encrypted key + existing `.pub`) works through the REAL binary — this plan's wiring-level test (`TestReuseEncryptedKeyWithExistingPubSucceeds`) exercises the real constructor and real filesystem, but not a live keystroke-driven PTY session. No unit or wiring test substitutes for that PTY proof per the L2 closure contract.
- Carried into 03-06's visual-regression allowlist: the new D-10 "Key" combined-header row and the reuse-picker body are render deltas not present in the approved Phase 2 dummy goldens (the dummy demo does not exercise this picker at all yet) — will need a divergence allowlist entry or an equivalent dummy-side picker fixture screen, at 03-06's discretion.
- No blockers for 03-05 or 03-06.

---
*Phase: 03-create-flow-backend*
*Completed: 2026-08-17*

## Self-Check: PASSED

- `.planning/phases/03-create-flow-backend/03-04-SUMMARY.md` — FOUND
- commit `76fb231` (Task 1) — FOUND
- commit `cc1f614` (Task 2) — FOUND
- commit `5df3e2d` (Task 3) — FOUND
- `internal/identity/modes.go` — FOUND
- `internal/keygen/keyscan.go` — FOUND
- `cmd/gitid/wiring.go` — FOUND
