---
phase: 05-identity-manager
plan: 02
subsystem: identity-lifecycle-substrate
tags: [keygen, sshconfig, gitconfig, allowed-signers, doctor, archive]

requires:
  - phase: 05-identity-manager
    provides: "05-01's tracer delete path, D-03 read surface, D-01 command tree — this plan builds the substrate primitives 05-01's ceremonies did not yet need"
provides:
  - "keygen.CopyKeyPairToArchive / MoveKeyPairToArchive / RemoveArchivedPair — the D-06 archive primitives with a required onCreated journal observer (R3-01)"
  - "sshconfig.ArchiveDirName / ArchiveDir + extended ReservedPaths/IsReservedPath — the archive directory registered in the ONE reserved-path registry, reserved at any nesting depth (R-19)"
  - "identity.InventoryDeps.IsReservedKeyPath — the causal reserved-path exclusion applied inside BuildInventory to the RESULT of ListKeyFiles (R-04)"
  - "keygen.AppendAllowedSigners — the D-07 append-not-replace allowed_signers writer for rotate/repair"
  - "gitconfig.RemoveProviderRewrite — the D-09 per-provider insteadOf remover (mechanism only; ref-counting is 05-04)"
affects: [05-03-key-rotation, 05-04-everything-scope-delete, 05-07-full-delete-lifecycle]

actuals:
  tokens: 17712
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Copy/Move/Remove archive primitive split (review R-03): CopyKeyPairToArchive never touches sources, MoveKeyPairToArchive is a literal copy-then-remove composition, RemoveArchivedPair is the rollback undo — one shared copy step, two distinct removal contracts, never overloaded into one function"
    - "Required onCreated CreatedFunc observer, fail-closed on both ends (nil refused, observer-error aborts before any removal) — the archive can never outrun the journal that has to undo it (review R3-01)"
    - "Reserved-path exclusion applied to the RESULT of an enumerator (InventoryDeps.ListKeyFiles inside BuildInventory), never inside a specific globber — causal for any future enumeration source, proven with a negative control (review R-04)"

key-files:
  created:
    - internal/keygen/archive.go
    - internal/keygen/archive_test.go
  modified:
    - internal/sshconfig/include.go
    - internal/sshconfig/include_test.go
    - internal/identity/inventory.go
    - internal/identity/inventory_test.go
    - internal/doctor/checks/orphans_test.go
    - internal/filewriter/filewriter_test.go
    - internal/keygen/signers.go
    - internal/keygen/signers_test.go
    - internal/gitconfig/renderer.go
    - internal/gitconfig/renderer_test.go

key-decisions:
  - "IsReservedPath's archive containment is a cleaned, separator-bounded prefix check, not a filepath.Dir equality test — covers any future nesting depth by construction (review R-19)"
  - "internal/keygen reproduces filewriter's four-line exclusive-open copy idiom locally instead of importing/exporting the unexported copyFileExclusive — the one shared property (existing destination is a hard error) is pinned by a mirrored test in both packages (review R2-07)"
  - "MoveKeyPairToArchive takes remove as an explicit function parameter, never a package-level mutable seam, so a caller (a future transaction test) can drive a deterministic second-source-removal failure independent of platform or root (review R2-06, revised cycle 3)"
  - "AppendAllowedSigners is scoped to rotate/repair only; create/update/clone keep WriteAllowedSigners/WriteAllowedSignersReplacing's single-line semantics — documented on the function so a future caller does not reach for the wrong writer"

requirements-completed: [KEY-05, MGR-06]

coverage:
  - id: D1
    description: "Key-pair archive primitives (Copy/Move/Remove) land material at ~/.ssh/gitid-archive/ (0700 dir, 0600 priv, 0644 pub), split by move-vs-copy semantics, with a required onCreated journal observer that fires before any source removal and aborts the archive on nil/error"
    requirement: "KEY-05"
    verification:
      - kind: unit
        ref: "internal/keygen/archive_test.go#TestCopyKeyPairToArchive_ModesAndNaming"
        status: pass
      - kind: unit
        ref: "internal/keygen/archive_test.go#TestMoveKeyPairToArchive_SecondSourceRemovalFailure"
        status: pass
      - kind: unit
        ref: "internal/keygen/archive_test.go#TestMoveKeyPairToArchive_ObserverErrorAbortsBeforeRemoval"
        status: pass
      - kind: unit
        ref: "internal/keygen/archive_test.go#TestArchivers_NilObserverRefused"
        status: pass
    human_judgment: false
  - id: D2
    description: "The archive directory is registered in the one reserved-path registry and is structurally invisible to the doctor's orphan check via a causal exclusion inside BuildInventory, proven with a negative control that goes red when the guard is disabled"
    requirement: "KEY-05"
    verification:
      - kind: unit
        ref: "internal/doctor/checks/orphans_test.go#TestOrphansArchivedKeyExcludedByBuildInventory"
        status: pass
      - kind: unit
        ref: "internal/doctor/checks/orphans_test.go#TestOrphansArchivedKeyNegativeControl"
        status: pass
      - kind: unit
        ref: "internal/sshconfig/include_test.go#TestIsReservedPath"
        status: pass
    human_judgment: false
  - id: D3
    description: "AppendAllowedSigners appends a rotation's new signer line to an identity's allowed_signers block without dropping the prior line(s), routed through the existing CR-18 comma-injection guard, idempotent on re-run"
    requirement: "KEY-05"
    verification:
      - kind: unit
        ref: "internal/keygen/signers_test.go#TestAppendAllowedSigners_AppendsSecondLine"
        status: pass
      - kind: unit
        ref: "internal/keygen/signers_test.go#TestAppendAllowedSigners_IdempotentSameLine"
        status: pass
      - kind: unit
        ref: "internal/keygen/signers_test.go#TestAppendAllowedSigners_RejectsCommaEmail"
        status: pass
      - kind: unit
        ref: "internal/keygen/signers_test.go#TestAppendAllowedSigners_ThenBlockRemovalRemovesAllLines"
        status: pass
    human_judgment: false
  - id: D4
    description: "RemoveProviderRewrite removes exactly the named provider's per-identity-provider insteadOf block, leaving a sibling provider's rewrite and the global baseline url-rewrites block untouched, idempotent on re-run"
    requirement: "MGR-06"
    verification:
      - kind: unit
        ref: "internal/gitconfig/renderer_test.go#TestRemoveProviderRewrite"
        status: pass
      - kind: unit
        ref: "internal/gitconfig/renderer_test.go#TestRemoveProviderRewrite_NoSuchBlockIsNilError"
        status: pass
    human_judgment: false

duration: ~90min
completed: 2026-08-25
status: complete
---

# Phase 5 Plan 02: Key Archive + Append-Signers + Provider-Rewrite Removal Substrate Summary

**Three isolated, fully-tested substrate primitives (D-06 key archive with a fail-closed journal observer, D-07 append-not-replace allowed_signers writer, D-09 per-provider insteadOf remover) plus the mandatory doctor-reserved registration for the archive directory, built ahead of the Phase 5 orchestration plans that compose them.**

## Performance

- **Duration:** ~90 min
- **Tasks:** 3 (Task 1: archive primitives + doctor-reserved registration; Task 2: AppendAllowedSigners; Task 3: RemoveProviderRewrite)
- **Files modified:** 12 (2 new, 10 modified)

## Accomplishments

- **The archive primitives are split by contract, not overloaded (review R-03).** `internal/keygen/archive.go` ships `CopyKeyPairToArchive` (never touches sources — delete-everything's primitive), `MoveKeyPairToArchive` (a literal copy-then-remove composition — rotation's primitive), and `RemoveArchivedPair` (the rollback undo, review R-10). Both archivers require a caller-supplied `onCreated` observer that fires the instant each archive copy lands, always before any source removal is attempted: a nil observer is refused with `ErrNoCreatedObserver` before any copy, and an observer that returns an error aborts the archive — removing the copies made so far and touching no source. This closes the cycle-3 finding (R3-01): a second-source-removal failure can never leave an archive entry the caller's rollback journal never heard about, because the journal already heard about BOTH copies before removal was ever attempted. A test drives a deterministic failure removing the SECOND source (via an injected `remove` parameter, not a platform permission trick — review R2-06) and asserts, via one shared ordered log, that both `onCreated` calls precede the first `remove` call.
- **The archive directory is registered exactly once and proven invisible to the doctor (D-06 mandatory).** `sshconfig.ArchiveDir`/`ArchiveDirName` is the single source of the archive location; `ReservedPaths`/`IsReservedPath` are extended with a cleaned, separator-bounded prefix check (not `filepath.Dir` equality) so the archive directory is reserved at ANY nesting depth (review R-19). The exclusion is wired causally: `identity.InventoryDeps.IsReservedKeyPath` filters the RESULT of `ListKeyFiles` inside `BuildInventory`, before the unused-key cross-reference runs — not inside any specific globber, because the production `id_*` glob is non-recursive and would never even match a path under `gitid-archive/` regardless of any guard (05-RESEARCH.md's point exactly). A doctor regression test plants an archived key and proves zero orphan findings through this real seam, with a negative control (the guard disabled) proving the SAME key DOES surface a finding — the guard is load-bearing, not vacuously green.
- **`AppendAllowedSigners` gives rotation a line that survives the old key's departure (D-07).** It composes the existing block body (read via `filewriter.ListBlocks`) plus a new line built exclusively through `AllowedSignersLine` — so the CR-18 comma-injection guard applies to this write path exactly as it applies to the existing one — deduping by exact match so a byte-identical re-run is a true no-op. A test proves the existing block-keyed `gitconfig.RemoveAllowedSignersBlock` still removes ALL accumulated lines together after two appends (review R-22): removal is keyed by identity name, not by line, so N rotations leaving N+1 lines is not a defect.
- **`RemoveProviderRewrite` completes the per-provider insteadOf pair (D-09).** It mirrors `WriteProviderRewrite`'s shape (same `ProviderRewriteBlockName` sentinel resolution, same hostname validation), and is explicitly documented as NOT `gitconfig.RemoveURLRewritesBlock` (the global baseline's block — a Phase 7 concern) per 05-RESEARCH.md Pitfall 4. A test writes two providers' rewrites plus the global baseline block, removes one provider's, and asserts the sibling provider AND the baseline block both survive.

## Task Commits

1. **Task 1: Key archive directory + mandatory doctor-reserved registration (D-06)** - `75fcd72` (feat)
2. **Task 2: Append-not-replace allowed_signers writer (D-07)** - `7ff41dd` (feat)
3. **Task 3: RemoveProviderRewrite — missing half of the per-provider insteadOf pair (D-09)** - `825325a` (feat)

## Files Created/Modified

- `internal/keygen/archive.go` (new) - `ArchivedPair`, `CreatedFunc`/`IgnoreCreated`/`ErrNoCreatedObserver`/`ErrArchiveIncomplete`, `CopyKeyPairToArchive`, `MoveKeyPairToArchive`, `RemoveArchivedPair`, `prepareArchiveDir`, `copyExclusive`.
- `internal/keygen/archive_test.go` (new) - full acceptance-criteria coverage: modes/naming, dir-tighten, symlink rejection, sources-untouched (Copy) / sources-removed (Move), second-source-removal-failure ordering proof, observer-error abort, nil-observer refusal, destination collision, absent-public-half, `RemoveArchivedPair` containment/tolerance, mirrored exclusive-open test.
- `internal/sshconfig/include.go` - `ArchiveDirName`, `ArchiveDir`; `ReservedPaths`/`IsReservedPath` extended with the archive entries (appended after config.d, prefix-containment at any depth).
- `internal/sshconfig/include_test.go` - `TestReservedPaths`/`TestIsReservedPath` extended with the D-06 archive cases (dir, direct file, nested-one-deeper file, plus the plan's literal `foo.txt`/`foo.config` cases).
- `internal/identity/inventory.go` - `InventoryDeps.IsReservedKeyPath`; `filterReservedKeyPaths` applied to `ListKeyFiles`'s result in `BuildInventory`; wired in `BuildInventoryDeps`/`InventoryDepsForHome`; defense-in-depth filter in `listKeyFilesRealForHome`.
- `internal/identity/inventory_test.go` - causal-exclusion unit test, nil-predicate no-op test, `TestBuildInventoryDeps` extended for the new field.
- `internal/doctor/checks/orphans_test.go` - the D-06 regression test + its negative control, both driven through the real `identity.BuildInventory` seam.
- `internal/filewriter/filewriter_test.go` - `TestCopyFileExclusiveRefusesExistingDestination`, mirroring `internal/keygen`'s own assertion of the same property (review R2-07).
- `internal/keygen/signers.go` - `AppendAllowedSigners`.
- `internal/keygen/signers_test.go` - create/append-order/idempotent/foreign-preservation/comma-rejection/mode/block-removal coverage.
- `internal/gitconfig/renderer.go` - `RemoveProviderRewrite`.
- `internal/gitconfig/renderer_test.go` - two-provider isolation + baseline survival, invalid-hostname rejection, idempotency, no-such-block-is-nil-error.

## Decisions Made

See `key-decisions` in the frontmatter. The most consequential: the `onCreated` observer contract on both archivers is now the load-bearing rollback-recoverability guarantee plan 05-03's transaction journal and plan 05-07's lifecycle rollback proof both depend on — neither may bind a nil or best-effort observer.

## Deviations from Plan

None — plan executed exactly as written, including the exact split-by-owner language in the plan for the stamp-collision retry (this plan proves the primitive's hard-error half only; the composition-root retry belongs to plan 05-03 Task 2, per the plan's own review R2-05 resolution).

## Issues Encountered

None. All three tasks' `<verify>` commands passed on first run after implementation; `make lint` was 0 issues on every task (after clearing a stale `golangci-lint` cache reference to a since-removed sibling worktree, unrelated to this plan's code); the full `make test` (including the `-tags screenshot` half) passed at wave close.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `keygen.MoveKeyPairToArchive`'s `remove` parameter and `onCreated` observer are ready for plan 05-03 Task 2 to bind: the composition root's stamp-collision retry and the transaction journal's observer registration both consume this plan's exported signatures unchanged.
- `keygen.CopyKeyPairToArchive` is ready for plan 05-04's delete-everything path to call directly.
- `keygen.AppendAllowedSigners` is ready for both plan 05-03 (rotate) and the repair ceremony (05-06/05-08 territory) to call; `WriteAllowedSigners`/`WriteAllowedSignersReplacing` remain untouched for create/update/clone.
- `gitconfig.RemoveProviderRewrite` is ready for plan 05-04's ref-counting decision (whether to call it) — this plan ships the mechanism only, no policy.
- No blockers identified for plans 05-03/05-04.

---
*Phase: 05-identity-manager*
*Completed: 2026-08-25*

## Self-Check: PASSED
