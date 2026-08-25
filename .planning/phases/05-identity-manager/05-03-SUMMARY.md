---
phase: 05-identity-manager
plan: 03
subsystem: identity-lifecycle-ceremonies
tags: [key-rotation, key-repair, allowed-signers, archive, mutation-journal, state-taxonomy]

requires:
  - phase: 05-identity-manager
    provides: "05-02's key archive primitives (keygen.MoveKeyPairToArchive/CopyKeyPairToArchive), the D-07 AppendAllowedSigners writer, and the D-06 archive directory's doctor-reserved registration — the substrate this plan's Rotate/RepairKey compose"
provides:
  - "identity.go's four named pipeline phases — preWriteGate, persistStagedKey, writeArtifacts, resolvedPhase — that runPipeline, Rotate, and RepairKey each compose, making a double-persist structurally impossible"
  - "identity.Rotate(existing Account, deps Deps) (RotateResult, error) — the D-05/D-06/D-07/D-08 key RETIREMENT ceremony (archive with MOVE semantics, persist once, append the new signer line while keeping the old, re-point to the SAME canonical paths)"
  - "identity.RepairKey(existing Account, otherOwnersOfTarget []string, deps Deps) (CreateResult, error) — the D-05/KEY-07/MGR-05 key REPAIR ceremony (generate at THIS identity's own canonical path, never touch pre-existing key material, fail closed on a shared target)"
  - "identity.KeyActionFor(h IdentityHealth, keyOwnerCount int) KeyAction — the pure D-05 router behind the single approved key-row menu entry"
  - "cmd/gitid/wiring.go's archiveKeyPairSeam + depsForTransaction(j) — the ONE archive implementation, bound to a transaction's mutationJournal so an archive copy no journal is watching can never be created; the backend-wide binding fails CLOSED"
  - "mutationJournal.recordCreatedFile — the created-file half of the transaction journal, landed in this wave rather than waiting for plan 05-07, so a wave-3 rotation is already all-or-nothing"
affects: [05-06-identity-planner, 05-07-full-delete-lifecycle]

actuals:
  tokens: 6577
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Four-phase pipeline decomposition (review R-01): preWriteGate/persistStagedKey/writeArtifacts/resolvedPhase are the ONLY path to deps.PersistKey, deps.WriteSSH, etc. — every orchestrator (Create/Reuse/AddAccount via runPipeline; Rotate; RepairKey) composes them directly and never re-enters a phase, making a double-write defect structurally impossible rather than merely tested-against"
    - "Explicit signersWriter parameter (review R-01) instead of a Deps boolean: writeArtifacts takes a signersWriter function argument so a caller selects the replacing writer (deps.WriteAllowedSigners) or an adapter over the appending one (deps.AppendAllowedSigners) by composition, never by a flag on the shared Deps struct"
    - "Fail-closed transaction-bound seam (review R3-01): the backend-wide identity.Deps.ArchiveKeyPair binding refuses with a sentinel error; only realBackend.depsForTransaction(j) — a copy of the ONE wiring, since identity.Deps is a value type — rebinds it to a seam whose onCreated observer is j.recordCreatedFile. An archive copy no journal is watching can never be created by construction."
    - "Result-carries-what-it-created-even-on-error (review R3-01): RotateResult's ArchivedPrivatePath/ArchivedPublicPath are assigned BEFORE the archive seam's error is inspected, at every step from the archive call onward — never the `if err != nil { return Zero, err }` shape that silently discards recoverable information"

key-files:
  created:
    - internal/identity/rotate.go
    - internal/identity/rotate_test.go
    - internal/identity/repair.go
    - internal/identity/repair_test.go
  modified:
    - internal/identity/identity.go
    - internal/identity/identity_test.go
    - internal/identity/modes.go
    - internal/identity/modes_test.go
    - internal/identity/state.go
    - internal/identity/state_test.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go

key-decisions:
  - "Repair signer semantics: APPEND, not REPLACE (05-RESEARCH.md Open Question 1, resolved during planning — see the dedicated section below, carried verbatim from the plan)"
  - "rotateInput derives CreateInput.Algo from the account's existing KeyPath via a new algoFromKeyPath helper (parsing keygen.KeyPaths' own id_<algo>_<name> convention), fixing a real latent bug in the prior skeleton where Algo was never set — without it, the real composition root's Generate would either fail outright (unsupported algorithm \"\") or land the new key at the wrong filename, breaking D-08's same-canonical-path guarantee"
  - "RepairKeyPath always generates ed25519 (repairAlgo constant), never an algorithm parsed from a pre-existing key — repair never reads prior key material (D-05), so unlike Rotate there is no prior algorithm to preserve"
  - "keyDirFor resolves the ~/.ssh directory from the identity's own KeyPath (or SSHConfigPath's directory when KeyPath is empty) rather than requiring a new Account field — every identity's key lives in the SAME ~/.ssh directory by construction, so this covers both the key-missing and shared-key cases without a schema change"

requirements-completed: [KEY-05, KEY-07, MGR-05]

coverage:
  - id: D1
    description: "runPipeline is decomposed into four named phases (preWriteGate, persistStagedKey, writeArtifacts, resolvedPhase); a call-order recording test proves Create/Reuse/AddAccount are byte-identical in behavior before and after the decomposition, and persistStagedKey fires exactly once per Create/Rotate ceremony"
    requirement: "KEY-05"
    verification:
      - kind: unit
        ref: "internal/identity/identity_test.go#TestCallOrderCreate"
        status: pass
      - kind: unit
        ref: "internal/identity/identity_test.go#TestPersistStagedKeyExactlyOnce_CreateAndRotate"
        status: pass
      - kind: unit
        ref: "internal/identity/identity_test.go#TestWriteArtifactsUsesProvidedSignerAdapter"
        status: pass
    human_judgment: false
  - id: D2
    description: "Rotate archives the previous key pair with MOVE semantics before persisting the new one at the SAME canonical path, appends the new signer line while keeping the old one, and reports exactly what it created (including on a failure inside the archive step itself) so a failed rotation is fully undoable"
    requirement: "KEY-05"
    verification:
      - kind: unit
        ref: "internal/identity/rotate_test.go#TestRotateGeneratesNewKeyAndRepointsAllFour"
        status: pass
      - kind: unit
        ref: "internal/identity/rotate_test.go#TestRotateArchiveStepFailureCarriesBothPaths"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestRotateSecondSourceRemovalRollback"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestRotateEndToEndThroughRealConstructor"
        status: pass
    human_judgment: false
  - id: D3
    description: "The production archive seam is bound to a transaction's journal, not to the backend: the backend-wide binding refuses to archive outside a transaction, and only depsForTransaction(j) produces a working seam that records every archive copy in j before any source removal"
    requirement: "KEY-05"
    verification:
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestArchiveKeyPairBackendWideBindingRefuses"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestJournalCreatedAndWatchedSetsAreDisjoint"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestArchiveKeyPairSeamStampCollisionRetry"
        status: pass
    human_judgment: false
  - id: D4
    description: "RepairKey targets THIS identity's own canonical key path (never Account.KeyPath), never touches pre-existing key material, and fails closed with ErrRepairTargetShared when its own target path is referenced by another identity"
    requirement: "KEY-07"
    verification:
      - kind: unit
        ref: "internal/identity/repair_test.go#TestRepairKeyPathTargetsOwnNameNeverAccountKeyPath"
        status: pass
      - kind: unit
        ref: "internal/identity/repair_test.go#TestRepairKeyRepairsIdentAWithoutTouchingIdentB"
        status: pass
      - kind: unit
        ref: "internal/identity/repair_test.go#TestRepairKeySharedTargetRefuses"
        status: pass
      - kind: unit
        ref: "internal/identity/repair_test.go#TestRepairKeyNeverCallsArchiveSeam"
        status: pass
    human_judgment: false
  - id: D5
    description: "One pure predicate (KeyActionFor) routes the single approved key-row menu entry from classified state AND key-owner count, never a collapsed label, covering the full 8-State x 3-owner-count cross product"
    requirement: "MGR-05"
    verification:
      - kind: unit
        ref: "internal/identity/state_test.go#TestKeyActionFor"
        status: pass
      - kind: unit
        ref: "internal/identity/state_test.go#TestKeyActionFor_OwnerCountTwoForcesRepairEvenForKeyUsedBoth"
        status: pass
      - kind: unit
        ref: "internal/identity/state_test.go#TestKeyActionFor_NoIO"
        status: pass
    human_judgment: false

duration: ~110min
completed: 2026-08-25
status: complete
---

# Phase 5 Plan 03: Rotate + Repair Key-Lifecycle Ceremonies Summary

**The two key-lifecycle ceremonies (rotate/retirement and repair/new-key) built as UI-free domain orchestration on top of a `runPipeline` decomposed into four composable phases, closing the double-persist defect cross-AI review found and wiring the archive seam to a transaction journal so an untracked archive copy is structurally impossible.**

## Performance

- **Duration:** ~110 min
- **Tasks:** 3 (Task 1: four-phase decomposition; Task 2: Rotate retirement ceremony + wiring; Task 3: RepairKey + state router)
- **Files modified:** 12 (4 new, 8 modified)

## Repair signer semantics (carried verbatim from the plan)

**Resolution: APPEND. Repair uses the same `keygen.AppendAllowedSigners` writer rotate uses.**

The previous revision of this plan carried a `checkpoint:decision` with `gate="blocking"` for this question. Cross-AI review raised two things about it, and both are honored here:

1. **DLV-08 conflict (review R-24).** `.planning/REQUIREMENTS.md` DLV-08 states the autonomous build loop runs unattended except for ONE hard stop — design approval, already recorded in Phase 2. A second blocking human checkpoint in wave 3 breaks that contract. The reviewer's recommendation was explicit: resolve the semantics during planning and remove the extra checkpoint. That is what this section does. The checkpoint is REMOVED and this plan is `autonomous: true`.

2. **The research recommendation was based on a factual error.** 05-RESEARCH.md recommended REPLACE on the grounds that "a stale line pointing at an absent key verifies nothing today". That is wrong. An `allowed_signers` entry verifies a signature from the PUBLIC key blob recorded in the line; the private key's presence on disk is irrelevant to verification. A line whose private key has been archived, rotated away, or deleted still verifies every commit that was signed with it.

Given that, APPEND wins on every axis that matters here:

- **Correctness:** historic `git log --show-signature` verification keeps working after a repair, exactly as D-07 requires it to after a rotation.
- **Reversibility:** appending is the reversible choice. A surplus line can be pruned later (Phase 8's fixer already owns that story). A replaced line is gone from the live file with no signal to the user that historic verification just stopped working.
- **Consistency:** one rule across both key ceremonies. `AppendAllowedSigners` is idempotent on a repeated line, so a repair that regenerates the same public line is a no-op rather than a duplicate.
- **Consistency with D-07's own reasoning:** D-07 is scoped to rotate in CONTEXT.md, but its stated rationale applies verbatim to repair. Extending it is the conservative reading of the locked decision, not a new decision.

**Cost accepted:** a repaired identity whose old key never existed accumulates one line that verifies nothing. That is bounded (one line per ceremony), invisible, and prunable in Phase 8.

## Task 1 / Task 2 boundary note (review R2-09, carried per plan instruction)

Task 1 explicitly left `Rotate` on its pre-Task-2 signature and composition — a temporary test (`TestTask1EndRotateStillReachesRunPipeline`) pinned that Rotate still reached `runPipeline` and produced the REPLACING `WriteAllowedSigners` writer at the end of Task 1. Task 2 changed `Rotate`'s signature to `RotateResult`, moved it to `rotate.go`, composed the four phases directly with an inserted archive step, and deleted that temporary test in the SAME commit that removed the `runPipeline` call — so the boundary was checked by a compiling/passing test, not merely described in prose. A comment-stripped grep gate over `rotate.go` (and, in Task 3, `repair.go`) confirms zero `runPipeline` references survive in either ceremony's file.

## Accomplishments

- **Task 1 — the double-persist defect is now structurally impossible, not merely tested against (review R-01).** `internal/identity/identity.go`'s `runPipeline` split into four unexported, individually-callable phases — `preWriteGate` (clipboard copy, signer-line build, pre-write gate, preview rendering), `persistStagedKey` (the sole path to `deps.PersistKey`, a guaranteed no-op when `staged.PrivPEM` is nil), `writeArtifacts` (the four writers, now taking an explicit `signersWriter` parameter instead of a boolean on `Deps` so a caller selects the replacing or appending writer by composition), and `resolvedPhase` (the closing test). `runPipeline` itself became their straight-line composition. A call-order recording test (`TestCallOrderCreate`/`Reuse`/`AddAccount`) pins the EXACT seam invocation sequence for every current caller and was verified to pass against the pre-decomposition code first — a true characterization test, not an assumed-equivalent refactor.
- **Task 2 — Rotate is now a real retirement ceremony (D-05/D-06/D-07/D-08).** `internal/identity/rotate.go`'s `Rotate` composes the four phases directly with an inserted archive step: `deps.Generate` → `preWriteGate` → `deps.ArchiveKeyPair` (MOVE semantics, vacating the canonical path) → `persistStagedKey` (exactly once) → `writeArtifacts` with the APPENDING writer → `resolvedPhase`. `RotateResult` carries `ArchivedPrivatePath`/`ArchivedPublicPath`/`ProviderHost` — populated from the archive step onward, BEFORE the seam's error is ever inspected, closing review R3-01's gap where a failure inside the archive step itself left the caller with an error and no paths to clean up. `rotateInput` now derives `CreateInput.Algo` from the account's existing key filename (`algoFromKeyPath`) so the real composition root reconstructs the IDENTICAL canonical path rather than failing on an empty algorithm — a real bug in the prior skeleton, fixed under deviation Rule 1 (see below).
- **Task 2 — the archive seam is bound to a transaction, and only a transaction (review R3-01).** `cmd/gitid/wiring.go`'s `buildIdentityDeps` binds the backend-wide `ArchiveKeyPair` to a REFUSING closure (`errArchiveOutsideTransaction`) — a closure built once at backend construction has no transaction in scope and can never reach a rollback journal, so failing closed here is what makes an untracked archive copy structurally impossible. `archiveKeyPairSeam` is the ONE archive implementation (only its `onCreated` observer varies); it owns the D-06 archive directory resolution and a monotonic stamp-collision retry (review R2-05/R-21) with an injectable clock for deterministic testing. `depsForTransaction(j)` returns a COPY of the one wiring (`identity.Deps` is a value type) with `ArchiveKeyPair` rebound to a seam whose observer is `j.recordCreatedFile`. `mutationJournal` gained the created-file half of its lifecycle (`recordCreatedFile`, a disjointness guard against `watchFile`, and `restore()` removing every created path after restoring every watched file) — landed in this wave rather than waiting for plan 05-07, so a wave-3 rotation is already all-or-nothing. A real-constructor test drives a rotation end-to-end over a hermetic fake home through `depsForTransaction`, and a dedicated regression test proves a second-source-removal failure is fully discoverable and rollback-able (both archive paths recorded in the journal BEFORE the failing removal, `restore()` leaves neither the archive entry nor a stale canonical key).
- **Task 3 — Repair targets its own path, never a sibling's (review R-02).** `internal/identity/repair.go`'s `RepairKeyPath(existing Account)` derives the target from THIS identity's OWN name via `keygen.KeyPaths`, never `Account.KeyPath` — the fix for the naive "repair at the canonical path" reading, which would overwrite a sibling's key when the identity currently borrows one. `RepairKey` composes the four phases minus the retirement steps (no archive, no pre-existing-key handling of any kind) and selects the SAME appending signer writer Rotate uses, per this plan's signer-semantics resolution. When the repair target itself is referenced by another identity, `RepairKey` fails closed with `ErrRepairTargetShared`, naming every owner, and performs NO effect at all.
- **Task 3 — one pure router, two orthogonal inputs (review R-07/R-25).** `internal/identity/state.go`'s `KeyActionFor(h IdentityHealth, keyOwnerCount int) KeyAction` requires the owner count as a SEPARATE input from the health classification, because `IdentityHealth`'s key axis reports USAGE, not ownership cardinality — a router taking only `IdentityHealth` cannot distinguish one owner from several and would misroute a shared-key identity into the destructive retirement ceremony. The router deliberately consumes `h.KeyState` directly rather than `ClassifyState`'s collapsed label, which is lossy on the key axis. A full 8-State × 3-owner-count cross-product table test pins every combination.

## Task Commits

1. **Task 1: Decompose runPipeline into four named phases (review R-01)** - `f663507` (feat)
2. **Task 2: Rotate becomes a retirement ceremony — archive (move), append, grace facts (KEY-05)** - `c61ff03` (feat)
3. **Task 3: Repair with a non-shared key target, plus the state-and-ownership router (KEY-07, MGR-05)** - `97e2c12` (feat)

## Files Created/Modified

- `internal/identity/identity.go` — `signersWriter` type; `preWriteGate`/`persistStagedKey`/`writeArtifacts`/`resolvedPhase` phases; `runPipeline` rewritten as their composition; two new `Deps` fields (`ArchiveKeyPair`, `AppendAllowedSigners`).
- `internal/identity/identity_test.go` — `orderRecorder`/`newOrderRecordingDeps`/`assertOrder` shared test helpers; call-order tests for Create; exactly-once/no-op persist tests; `writeArtifacts` adapter-selection test.
- `internal/identity/modes.go` — `Rotate`/`rotateInput` removed (moved to rotate.go); `fragmentPathFor` untouched.
- `internal/identity/modes_test.go` — Rotate-specific tests removed (moved to rotate_test.go); call-order tests added for Reuse/AddAccount.
- `internal/identity/rotate.go` (new) — `RotateResult`, `Rotate`, `rotateInput`, `algoFromKeyPath`.
- `internal/identity/rotate_test.go` (new) — full acceptance-criteria coverage: generate/archive/persist/append ordering, gate-abort, persist-exactly-once, archive-step-own-failure, algorithm preservation, two-rotation accumulation.
- `internal/identity/repair.go` (new) — `repairAlgo`, `ErrRepairTargetShared`, `RepairKeyPath`, `keyDirFor`, `RepairKey`, `repairInput`.
- `internal/identity/repair_test.go` (new) — own-path-vs-sibling proof, shared-target refusal, never-archives proof, persist-exactly-once, append-writer selection, two-line accumulation, gate-abort, key-missing repair.
- `internal/identity/state.go` — `KeyAction`, `KeyActionRotate`, `KeyActionRepair`, `KeyActionFor`.
- `internal/identity/state_test.go` — 8×3 cross-product table test, owner-count-forces-repair proof, no-I/O proof.
- `cmd/gitid/wiring.go` — `errArchiveOutsideTransaction`; `realBackend.archiveKeyPairSeam`/`archiveClock`/`archiveRemove`/`depsForTransaction`; two new `realBackend` test-only fields (`failArchiveRemoveAt`, `archiveClockNow`); `buildIdentityDeps` wires the two new `Deps` fields; `mutationJournal` gains `createdFiles`/`seenCreated`/`recordCreatedFile`, `watchFile`'s disjointness guard, and `restore()`'s created-file removal pass.
- `cmd/gitid/wiring_test.go` — real-constructor rotation end-to-end test, backend-wide-refusal test, journal disjointness test, restore-removes-created-files test, the second-source-removal rollback regression, and the stamp-collision retry test.

## Decisions Made

See `key-decisions` in the frontmatter. The most consequential: the `algoFromKeyPath` fix (Rule 1 — see Deviations) is what makes Rotate's D-08 "same canonical path" guarantee actually hold through the real composition root, not just in fake-deps unit tests.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `rotateInput` never set `CreateInput.Algo`, which the real composition root's `Generate` requires to reconstruct the identical canonical key path**
- **Found during:** Task 2, while designing the real-constructor wiring test
- **Issue:** The pre-existing `rotateInput` (inherited from the original `Rotate` skeleton) built a `CreateInput` without setting `Algo`. `cmd/gitid/wiring.go`'s real `Generate` closure derives the final key path via `keygen.KeyPaths(b.sshDir, in.Algo, in.Name)`, and `keygen.GenerateMaterial` treats an empty `Algo` as an unsupported-algorithm error. Left unfixed, a real rotation would either fail outright or (had the registry accepted an empty key) land the new key at a different filename than the identity's existing key — directly breaking this plan's own D-08 must_have ("Rotate leaves the SSH `IdentityFile` value... textually unchanged, because the new key takes the same canonical path").
- **Fix:** Added `algoFromKeyPath(keyPath, identityName string) string` in `rotate.go`, parsing the algorithm segment out of the existing key's filename (`id_<algo>_<name>`) by stripping the known `id_` prefix and the exact `_<identityName>` suffix — correct even when the identity name itself contains underscores or the algorithm segment contains a hyphen (`rsa-4096`). `rotateInput` now sets `Algo: algoFromKeyPath(a.KeyPath, a.Name)`.
- **Files modified:** `internal/identity/rotate.go`
- **Verification:** `TestRotateInputPreservesAlgorithm`, `TestAlgoFromKeyPath` (table test including the underscore-in-name and hyphen-in-algo edge cases); proven end-to-end through the real composition root by `cmd/gitid/wiring_test.go#TestRotateEndToEndThroughRealConstructor`, which seeds a real ed25519 key pair and asserts the rotated key lands at the SAME canonical path.
- **Committed in:** `c61ff03`

### CLAUDE.md-driven commit-granularity note (not a deviation rule, but worth recording)

Every task in this plan carries `tdd="true"`, and the GSD default is to author tests first (RED) then implementation (GREEN) as separate commits where practical. CLAUDE.md's commit rule ("one coherent change — its implementation, its tests, and its docs — is a single commit... let the buildable boundary, not file count, set the commit granularity") takes precedence: each task's tests and implementation were authored test-first (in Task 1, the call-order recording test was written and run against the PRE-decomposition code first, confirming it captured real behavior before any refactor began) but landed as ONE commit per task, matching the precedent set by 05-01/05-02. No RED/GREEN split commits exist in this plan's history for the same reason those two prior plans have none.

## Issues Encountered

None beyond the one auto-fixed bug above. Every task's `<verify>` command passed on the first run after implementation (all call-order predictions and rollback-ordering assertions matched actual `go test` output without any adjustment needed). `make lint` required one `golangci-lint cache clean` at the start of Task 1 to clear a stale cache reference to a since-removed sibling worktree (the SAME pre-existing, unrelated issue 05-02's SUMMARY documented) — 0 issues on every run afterward. The full module suite (`go test -race ./...`, 1181 tests across 20 packages) and `make test` (including the `-tags screenshot` half and the copy-freeze gate) are green.

## Known Stubs

None. Every deliverable this plan promises (the four-phase decomposition, `Rotate`, `RepairKey`, `KeyActionFor`, the transaction-bound archive seam) is fully implemented and tested, including through the real composition root. `Rotate` and `RepairKey` have no production CALLER yet (no CLI verb, no TUI ceremony wires to them) — that is explicitly out of this plan's scope per its own objective ("UI-free domain orchestration... the two ceremonies D-05 splits apart") and is plan 05-06/05-07's job, not a stub.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `identity.Rotate` and `identity.RepairKey` are ready for plan 05-06 (the TUI ceremony screens) and plan 05-07 (the real CLI/TUI wiring, mirroring `runDelete`'s precedent) to call — both take a `deps Deps` built via `b.depsForTransaction(j)`, never `b.deps` directly (whose `ArchiveKeyPair` binding refuses by design).
- `identity.KeyActionFor` is ready for the single approved key-row menu entry (FIELDS.md's `action_new_key`) to route through — it needs `IdentityHealth` (already available from `identity.Classify`) plus a `keyOwnerCount` the caller computes from the SAME key-path cross-reference `identity.SharedKeyOwners`-shaped logic 05-04/05-07 will need for `RepairKey`'s `otherOwnersOfTarget` argument.
- `mutationJournal.recordCreatedFile`/`restore()`'s created-file removal are ready for plan 05-07's full lifecycle to reuse without further extension to the file half (05-07 was already scoped to extend directories and mode restoration, per this plan's Task 2 action item).
- No blockers identified for plans 05-04/05-06/05-07.

---
*Phase: 05-identity-manager*
*Completed: 2026-08-25*

## Self-Check: PASSED

All 13 files listed in "Files Created/Modified" confirmed present on disk; commits `f663507`, `c61ff03`, `97e2c12` confirmed present in `git log`.
