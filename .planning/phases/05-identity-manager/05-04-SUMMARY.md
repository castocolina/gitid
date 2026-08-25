---
phase: 05-identity-manager
plan: 04
subsystem: identity-lifecycle-delete
tags: [delete-lifecycle, provider-rewrite, key-archive, scan, gitconfig, ssh-config]

requires:
  - phase: 05-identity-manager
    provides: "05-01's Git-only delete tracer + ErrScopeNotAvailable refusal for everything scope; 05-02's key-archive primitives (CopyKeyPairToArchive/MoveKeyPairToArchive) and RemoveProviderRewrite mechanism; 05-03's archiveKeyPairSeam/depsForTransaction precedent this plan mirrors for the delete-side archive seam"
provides:
  - "identity.RewriteProviderKey / ProviderHostForSSHHostname / ProviderKeyForHost — one provider normalizer for both the write path and the D-09 count path, covering GitHub/GitLab/Bitbucket"
  - "identity.ProviderRefCount / DeleteDeps.ForeignProviderRefs — the D-09 managed + hand-written reference count gating provider-rewrite removal"
  - "identity.SharedKeyOwners + Delete's D-12 shared-key auto-downgrade (computed once, gates both the archive step and the live-key removal)"
  - "identity.ScanUnmanagedReferences / SplitScanRegions / UnmanagedScanDisclaimer — the D-13 three-region, whole-token advisory scan"
  - "identity.DeletePlan / PlanDelete / DeleteTarget / deleteTargets — the pure preview both a future confirm screen and CLI dry-run render, structurally guaranteed to equal what Delete's write path touches (DeleteResult.Modified)"
  - "cmd/gitid's copyKeyPairSeam / deleteDepsForTransaction — the archive-copy seam bound to a transaction's journal, mirroring rotation's archiveKeyPairSeam/depsForTransaction precedent"
affects: [05-06-identity-planner, 05-07-full-delete-lifecycle]

actuals:
  tokens: 28766
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "One normalizer, two consumers (review R-05): RewriteProviderKey backs both the write path (Reconstruct's ForceSSH lookup) and the count path (ProviderRefCount) — never two independently-drifting comparisons of provider hostnames"
    - "keySurvives/providerSurvives computed ONCE, passed down (review R2-01/R3-03): Delete derives both booleans at the top of the call from the same deps.Accounts() read, and the shared deleteTargets helper takes them as required parameters rather than re-deriving either — the plan/write equality test is the observable proof, not a call-count assertion on a pure function"
    - "Logical targets, not bare file paths (review R-11): DeleteTarget{File, Block, Label} names a managed-block region or a whole-file target, so a provider rewrite, an SSH Host block, and an allowed_signers entry inside SHARED files can each be asserted precisely"
    - "Three-region scan model (review R-09): ScanRegionOwnManaged (dropped) / ScanRegionOtherManaged (reported, a sibling's own reference) / ScanRegionUnmanaged (reported, foreign text) — never a flat managed/unmanaged split that would silently drop a sibling's reference"
    - "Delete-side archive-copy seam mirrors rotation's archive-move seam exactly: copyKeyPairSeam/deleteDepsForTransaction reuse archiveKeyPairSeam/depsForTransaction's fail-closed backend-wide-refuses / transaction-scoped-works structure (review R3-01), never a second, weaker pattern"

key-files:
  created:
    - internal/identity/scan.go
    - internal/identity/scan_test.go
    - internal/identity/deleteplan.go
    - internal/identity/deleteplan_test.go
  modified:
    - internal/identity/delete.go
    - internal/identity/delete_test.go
    - internal/identity/loader.go
    - internal/identity/loader_test.go
    - internal/sshconfig/reader.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go

key-decisions:
  - "Planner resolution (per <output>'s instruction): a Git-only delete NEVER removes a provider rewrite — the identity's SSH alias survives that scope, so it still counts as a user of the provider; the ref-count question only arises on the everything path. Recorded as a doc comment on Delete and enforced structurally (the git-only branch returns before the provider-rewrite step even exists)."
  - "Planner resolution (per <output>'s instruction): delete-everything reuses the D-06 archive directory (0700 dir, key copies 0600/0644) as D-11's backup location, rather than inventing a second backup convention — the SAME sshconfig.ArchiveDir(sshDir) plan 05-02 registered as doctor-reserved and plan 05-03's rotation already writes to."
  - "buildDeleteDeps.RemoveKeyFiles was changed from a filewriter.BackupAndRemove (creates a SECOND, timestamped sibling backup) to a plain os.Remove: under DeleteScopeEverything it now runs strictly AFTER CopyKeyPairToArchive has already landed the recoverable copy, so a second backup would be redundant, not additive. This is a deliberate correction of the plan 05-01-era stub, not a plan deviation, since scope-everything had no real implementation until this plan."
  - "'internal/identity's own providerHostname switch' (Task 1's acceptance-criteria wording) is DefaultHostname (identity.go) — no function literally named providerHostname exists in this package. TestProviderTableRoundTripsWizardProviders documents this mapping explicitly and drives its round-trip assertion from DefaultHostname's own switch."
  - "The D-13 scan's real composition-root wiring (assembling the four ScanSource entries from the user's actual ~/.ssh/config, ~/.gitconfig, fragment, and allowed_signers) is intentionally NOT part of this plan's file scope (files_modified excludes wiring.go for Task 2) — the source_audit table marks D-13 COVERED by 05-04 (the domain primitives) + 05-06 (the real wiring, alongside the confirm screen that renders the hits). Same split applies to DeletePlan/PlanDelete: no CLI/TUI caller exists yet (05-06/05-07's job per MGR-06's source_audit row)."

requirements-completed: [MGR-06]

coverage:
  - id: D1
    description: "DeleteScopeEverything is a full implementation: SSH Host block, gitconfig includeIf block, fragment file, allowed_signers block, provider rewrite (ref-counted), and key pair (archived-then-removed) are all handled, ordered so a partial failure never destroys recoverable material"
    requirement: "MGR-06"
    verification:
      - kind: unit
        ref: "internal/identity/delete_test.go#TestDelete_Everything_NoLongerRefuses"
        status: pass
      - kind: unit
        ref: "internal/identity/delete_test.go#TestDelete_Everything_ArchivesThenRemovesKeyLast"
        status: pass
      - kind: unit
        ref: "internal/identity/delete_test.go#TestDelete_Everything_ArchiveFailureLeavesLiveKeyIntact"
        status: pass
    human_judgment: false
  - id: D2
    description: "Provider rewrite reference-counting spans managed AND hand-written aliases, comparing NORMALIZED provider keys (short-form providers, alt-SSH endpoints, and dotless alias/provider pairs all resolve correctly), covering GitHub, GitLab, and Bitbucket"
    requirement: "MGR-06"
    verification:
      - kind: unit
        ref: "internal/identity/loader_test.go#TestRewriteProviderKey"
        status: pass
      - kind: unit
        ref: "internal/identity/loader_test.go#TestProviderKeyForHost"
        status: pass
      - kind: unit
        ref: "internal/identity/loader_test.go#TestProviderTableRoundTripsWizardProviders"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestRunDeleteEverything_TwoIdentitiesSameProvider"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestRunDeleteEverything_HandWrittenAliasKeepsRewrite"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestRunDeleteEverything_BitbucketHandWrittenAliasKeepsRewrite"
        status: pass
    human_judgment: false
  - id: D3
    description: "The D-11 key archive is a copy-first, remove-last transaction bound to a mutationJournal (backend-wide binding refuses outside a transaction), mirroring rotation's precedent exactly"
    requirement: "MGR-06"
    verification:
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestBuildDeleteDepsCopyKeyPairToArchiveBackendWideBindingRefuses"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-12 shared-key auto-downgrade: a key referenced by a sibling identity is kept (not archived-then-removed), and every sibling is named; a sole-owner key is archived and removed normally"
    requirement: "MGR-06"
    verification:
      - kind: unit
        ref: "internal/identity/delete_test.go#TestDelete_Everything_SharedKeyDowngrade_SkipsArchiveKeepsFiles"
        status: pass
      - kind: unit
        ref: "internal/identity/delete_test.go#TestDelete_Everything_NoSharedKey_ArchivesAndRemoves"
        status: pass
      - kind: unit
        ref: "internal/identity/scan_test.go#TestSharedKeyOwners_MultipleSiblingsSortedOrder"
        status: pass
    human_judgment: false
  - id: D5
    description: "The D-13 unmanaged-reference scan matches whole tokens only (never a substring), reports hits in a sibling's own managed block and in foreign text, drops hits inside the identity's own block, and always carries the fixed disclaimer"
    requirement: "MGR-06"
    verification:
      - kind: unit
        ref: "internal/identity/scan_test.go#TestLineReferencesAlias_WholeTokenOnly"
        status: pass
      - kind: unit
        ref: "internal/identity/scan_test.go#TestScanUnmanagedReferences_DropsOwnManagedReportsOthers"
        status: pass
      - kind: unit
        ref: "internal/identity/scan_test.go#TestUnmanagedScanDisclaimer_Fixed"
        status: pass
    human_judgment: false
  - id: D6
    description: "DeletePlan is a pure, error-returning preview built from the SAME deleteTargets helper Delete's write path uses, so the plan and the write can never diverge — proven across three fixture classes plus negative controls"
    requirement: "MGR-06"
    verification:
      - kind: unit
        ref: "internal/identity/deleteplan_test.go#TestPlanDeleteAndDelete_AgreeAcrossThreeFixtures"
        status: pass
      - kind: unit
        ref: "internal/identity/deleteplan_test.go#TestPlanDeleteAndDelete_KeySurvivesInvertedNegativeControl"
        status: pass
      - kind: unit
        ref: "internal/identity/deleteplan_test.go#TestDeleteTargets_NeverCallsSharedKeyOwners"
        status: pass
      - kind: unit
        ref: "internal/identity/deleteplan_test.go#TestPlanDelete_ReadFailureReturnsErrorAndZeroPlan"
        status: pass
    human_judgment: false

duration: ~140min
completed: 2026-08-25
status: complete
---

# Phase 5 Plan 04: Everything-Scope Delete — Provider Ref-Count, Key Archive, Shared-Key Downgrade, Advisory Scan Summary

**`DeleteScopeEverything` is now a real, transaction-safe implementation: reference-counted `provider-rewrite` cleanup across managed AND hand-written SSH aliases (GitHub/GitLab/Bitbucket), a copy-then-remove-last key archive bound to a journal, a structurally-guaranteed D-12 shared-key downgrade, a D-13 whole-token advisory reference scan, and a pure `DeletePlan` preview that can never disagree with what `Delete` actually writes.**

## Performance

- **Duration:** ~140 min
- **Tasks:** 3 (Task 1: provider ref-count + key archive; Task 2: shared-key downgrade + scan; Task 3: DeletePlan)
- **Files modified:** 12 (4 new, 8 modified — including an unrelated `make fmt`-driven import fix to an archived, still `go:build e2e`-tagged POC test file, see Deviations)

## Accomplishments

- **One normalizer closes the provider-comparison gap (review R-05/R2-04/R2-11).** `internal/identity/loader.go`'s `RewriteProviderKey` is the promoted, exported form of the prior unexported `rewriteLookupProvider` — now the SAME function backs both the write path (`Reconstruct`'s `ForceSSH` lookup) and the count path (`ProviderRefCount`). `ProviderHostForSSHHostname` maps every recipe-canonical alt-SSH endpoint (`ssh.github.com`, `altssh.gitlab.com`, `altssh.bitbucket.org`) plus the bare FQDN hostnames to the FQDN provider key, and `ProviderKeyForHost` gives one documented precedence (marker → hostname → alias-suffix fallback) for resolving a single Host stanza. `hostnameToProvider` gained the Bitbucket pair it was missing (R2-04), and `RewriteProviderKey` gained a short-to-FQDN lookup inserted BEFORE the alias-suffix fallback, so a dotless provider (`"github"`) paired with a dotless alias (`"mygh"`) now resolves to `"github.com"` instead of the invalid one-label host `"mygh"` the old logic returned (R2-11) — a real correctness fix to the WRITE path too, pinned by its own test.
- **The everything scope is a real transaction, not a refusal.** `identity.Delete` no longer returns `ErrScopeNotAvailable` for `DeleteScopeEverything`. `deleteEverything` archives the key pair FIRST (via the new `DeleteDeps.CopyKeyPairToArchive`, the D-11 copy-only primitive — never touches sources), removes the managed SSH Host block and the `allowed_signers` block, unlinks the fragment, removes the provider rewrite ONLY when both the managed reference count (`ProviderRefCount`) and the hand-written reference count (`DeleteDeps.ForeignProviderRefs`, which strips every managed block's text from the SSH config before parsing the remainder — `cmd/gitid`'s `countForeignProviderRefs`) are zero, and removes the LIVE key files LAST. Any failure before that last step leaves both the archive copy and the live key intact — proven by `TestDelete_Everything_ArchiveFailureLeavesLiveKeyIntact`.
- **The archive-copy seam mirrors rotation's precedent exactly (review R3-01).** `cmd/gitid/wiring.go`'s `copyKeyPairSeam`/`deleteDepsForTransaction` reuse the SAME fail-closed shape `archiveKeyPairSeam`/`depsForTransaction` established for rotation in plan 05-03: `buildDeleteDeps`'s backend-wide `CopyKeyPairToArchive` binding REFUSES with `errArchiveOutsideTransaction`, and only `deleteDepsForTransaction(j)` rebinds it to a seam whose `onCreated` observer is `j.recordCreatedFile`. `runDelete` now calls `deleteDepsForTransaction(journal)` for every scope (not `buildDeleteDeps` directly), and extends its file-watching to the SSH config and `allowed_signers` under the everything scope.
- **The D-12 shared-key downgrade is structurally impossible to get wrong (review R2-01).** `identity.SharedKeyOwners` returns every OTHER identity referencing a key path, sorted. `Delete` computes `accounts` and `keySurvives` EXACTLY ONCE, before any effect, and passes `keySurvives` into `deleteEverything`'s key-archive step AND its final live-key-removal step — the same boolean gates both, so a shared key can never be archived-and-removed. `DeleteResult.KeyKeptFor` names every surviving sibling.
- **The D-13 scan is a three-region, whole-token, advisory-only primitive (review R-09).** `internal/identity/scan.go`'s `SplitScanRegions` splits a file into `ScanRegionOwnManaged` (the identity's OWN block — dropped), `ScanRegionOtherManaged` (a SIBLING's own block — reported, so a sibling's reference is disclosed rather than silently dropped alongside the identity's own block), and `ScanRegionUnmanaged` (foreign text — reported). `ScanUnmanagedReferences` matches whole tokens only — a bare token, a `Host <alias>` value, or a `git@<alias>:...`/`<alias>:...` URL form — never a bare substring, with absolute 1-based line numbers tracked per source file across multiple same-file sources. `UnmanagedScanDisclaimer` is the ONE frozen definition of the D-13 disclaimer sentence (review R-27) for plan 05-06 to register and render by reference.
- **`DeletePlan` cannot promise what `Delete` will not do (review R-11/R2-01/R3-03).** `internal/identity/deleteplan.go`'s `deleteTargets(acct, scope, providerSurvives, keySurvives)` is the ONE target-derivation helper both `PlanDelete` and `Delete` call; `keySurvives` is a REQUIRED fourth parameter, symmetric with `providerSurvives` — key targets are emitted only when the scope is everything AND the key does not survive. `DeleteTarget{File, Block, Label}` names a LOGICAL region (a managed block inside a shared file, or a whole-file target) rather than a bare file path, so a preview/write comparison cannot pass while the wrong block inside the right file was touched. `Delete`'s `DeleteResult.Modified` is set from the SAME `deleteTargets` call its own write path just acted on. `TestPlanDeleteAndDelete_AgreeAcrossThreeFixtures` drives `PlanDelete` and `Delete` over the SAME accounts slice across sole-owner/shared-key/shared-provider fixtures and asserts the plan's targets equal the write's `Modified` as sets of `(File, Block)` pairs, backed by an inverted-`keySurvives` negative control and a comment-stripped grep gate proving `deleteTargets` never calls `SharedKeyOwners` itself.

## Task Commits

1. **Task 1: Reference-counted provider cleanup and recoverable key removal (D-09, D-11)** - `e6cb36c` (feat)
2. **Task 2: Shared-key auto-downgrade and the advisory unmanaged-reference scan (D-12, D-13)** - `411e0d2` (feat)
3. **Task 3: DeletePlan — one pure preview both the confirm screen and the CLI dry-run render** - `6934ec1` (feat)

## Files Created/Modified

- `internal/identity/loader.go` — `RewriteProviderKey` (promoted + R2-11 guard), `ProviderHostForSSHHostname`, `ProviderKeyForHost`, `ProviderRefCount`, `hostnameToProvider`/`shortToFQDNProvider` (Bitbucket added).
- `internal/identity/loader_test.go` — full coverage for the four new normalizer functions plus the wizard-provider round-trip gate.
- `internal/identity/delete.go` — `DeleteDeps` gains `Accounts`/`ForeignProviderRefs`/`RemoveProviderRewrite`/`CopyKeyPairToArchive`; `DeleteResult` gains `ProviderRewriteRemoved`/`Backup`, `KeyKeptFor`, `ArchivedKeyPaths`, `Modified`; `Delete`/`deleteEverything` implement the full everything-scope transaction with the D-12 downgrade; `SharedKeyOwners`.
- `internal/identity/delete_test.go` — everything-scope ref-count/archive/downgrade/allowed-signers coverage.
- `internal/identity/scan.go` (new) — `ScanRegion`, `UnmanagedHit`, `ScanSource`, `UnmanagedScanDisclaimer`, `SplitScanRegions`, `ScanUnmanagedReferences`.
- `internal/identity/scan_test.go` (new) — whole-token matching table, three-region split/drop/report coverage, disclaimer pin.
- `internal/identity/deleteplan.go` (new) — `DeleteTarget`, `DeletePlan`, `PlanDeps`, `PlanDelete`, `deleteTargets`, `providerRewriteDeleteTarget`.
- `internal/identity/deleteplan_test.go` (new) — plan/write equality across three fixtures, negative controls, grep gate, no-write proof, error-propagation proof.
- `internal/sshconfig/reader.go` — `HostStanza`/`AllHostStanzas` (the D-09 hand-written-alias data source).
- `cmd/gitid/wiring.go` — `copyKeyPairSeam`, `deleteDepsForTransaction`, `countForeignProviderRefs`; `buildDeleteDeps` gains `Accounts`/`ForeignProviderRefs`/`RemoveProviderRewrite`/`CopyKeyPairToArchive`, and `RemoveKeyFiles` becomes a plain remove (the archive copy is the backup, see Deviations); `runDelete` now uses the transaction-bound delete deps and sets `acct.AllowedSignersPath`.
- `cmd/gitid/wiring_test.go` — reflection guard (already covered the new fields), the backend-wide-refuses/transaction-works archive-copy proof, real-constructor two-identity/hand-written-alias/Bitbucket regression tests, and the CLI/TUI everything-scope parity test rewritten for the now-real implementation.

## Decisions Made

See `key-decisions` in the frontmatter. The two most consequential: (1) `RemoveKeyFiles` was corrected from a redundant `filewriter.BackupAndRemove` to a plain `os.Remove`, since the D-11 archive copy already serves as the recoverable backup by the time it runs; (2) `keySurvives`/`providerSurvives` are computed exactly once per `Delete` call and threaded as parameters into the shared `deleteTargets` helper, which is what makes the plan/write equality guarantee structural rather than merely tested-for.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `make fmt`'s whole-module `goimports` pass added missing imports to an archived, still-compiled e2e test file**
- **Found during:** Task 2, at commit time (the pre-commit hook's `make fmt` step)
- **Issue:** `.planning/archive/0.0.1-poc-product-features-in-tui/e2e/ui_pty_poc_e2e_test.go` carries `//go:build e2e` and `package e2e`, so despite living under `.planning/archive/`, it is still part of the Go module's `e2e`-tagged build. It was pre-existing missing four stdlib imports (`context`, `os`, `strings`, `testing`, `time`) unrelated to this plan's own code; CLAUDE.md's hook forbids `--no-verify`, and `make fmt` runs across the whole module (`pass_filenames: false`), so the fix landed in whichever commit ran the hook first.
- **Fix:** `goimports -w` added the four missing imports; no logic change.
- **Files modified:** `.planning/archive/0.0.1-poc-product-features-in-tui/e2e/ui_pty_poc_e2e_test.go`
- **Verification:** `go vet -tags e2e ./...` (part of `make lint-tagged`) passes.
- **Committed in:** `411e0d2` (part of the Task 2 commit, since that was the first commit after the drift was discovered)

**2. [Rule 1 - Bug] gosec G703 flagged a pre-existing test helper once new call sites exercised its taint path**
- **Found during:** Task 1, at `make lint` time
- **Issue:** `cmd/gitid/wiring_test.go`'s pre-existing `writeFile` helper (`os.WriteFile` with a variable `path`) had no `//nolint:gosec` annotation, unlike its `readFile` sibling. gosec's G703 taint-analysis rule apparently only surfaces once enough new call sites exist to trace a path it considers worth flagging — my new fixture-building tests tipped it over.
- **Fix:** Added the same `//nolint:gosec // hermetic t.TempDir() fixture path (G703)` annotation `readFile` already carries.
- **Files modified:** `cmd/gitid/wiring_test.go`
- **Verification:** `make lint` reports 0 issues.
- **Committed in:** `e6cb36c` (part of the Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking hook-driven fix, 1 lint bug fix). No scope creep — both are exactly what the buildable-boundary commit rule and the zero-tolerance lint gate require.

## Issues Encountered

None beyond the two deviations above. Every task's `<verify>` command passed on first run after implementation. `make lint` (golangci-lint 2.12.2 + gosec) is 0 issues at every commit. The full `make test` (including the `-tags screenshot` half and the copy-freeze gate) is green at every commit.

## Known Stubs

- **The D-13 scan's real four-source composition-root wiring is not part of this plan.** `internal/identity/scan.go` ships the pure primitives (`SplitScanRegions`, `ScanUnmanagedReferences`); assembling the real `[]ScanSource` from the user's actual `~/.ssh/config`, `~/.gitconfig`, the identity's fragment, and `~/.ssh/allowed_signers` is 05-06's job (per this plan's own `source_audit` table, D-13 is COVERED by 05-04 + 05-06 jointly). `internal/identity/scan_test.go#TestScanUnmanagedReferences_NoPrivateKeySources` documents the four-source contract at the domain level but does not exercise a real backend.
- **`DeletePlan`/`PlanDelete` has no CLI or TUI caller yet.** No confirm screen or `--dry-run` path consumes it in this plan — that is 05-06 (confirm screen) and 05-07 (CLI dry-run)'s job, per `source_audit`'s MGR-06 row listing `05-04, 05-06, 05-07`.
- Neither stub blocks this plan's own acceptance criteria or `<success_criteria>` — both are the intentional wave boundary the plan's own dependency notes describe, not gaps introduced by this execution.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `identity.Delete`'s everything scope, `identity.PlanDelete`, and `internal/identity/scan.go`'s primitives are all ready for plan 05-06 (confirm screen + real scan-source wiring) and 05-07 (CLI dry-run + full lifecycle) to consume unchanged.
- `cmd/gitid`'s `deleteDepsForTransaction`/`copyKeyPairSeam` follow the exact same pattern plan 05-03 established for rotation, so 05-07's full-lifecycle extension has one pattern to generalize, not two.
- The `RewriteProviderKey`/`ProviderHostForSSHHostname`/`ProviderKeyForHost` normalizer trio is ready for any future provider-aware feature (Phase 6+) to reuse without re-deriving hostname-to-provider logic a third time.
- No blockers identified for plans 05-06/05-07.

---
*Phase: 05-identity-manager*
*Completed: 2026-08-25*

## Self-Check: PASSED

All 11 created/modified files confirmed present on disk; commits `e6cb36c`, `411e0d2`, `6934ec1` confirmed present in `git log`.
