---
phase: 01-foundations-spikes-ci
plan: 04
subsystem: identity
tags: [go, identity, state-taxonomy, classifier, inventory, mgr-02, dlv-07]

# Dependency graph
requires:
  - phase: 01-foundations-spikes-ci (plan 03, concurrent Wave 1)
    provides: canonical `config.d/*.config` glob literal in internal/sshconfig/include.go (mirrored, not imported, per ACCEPTED DUPLICATION)
provides:
  - internal/identity/state.go — State (8 locked MGR-02 labels), Problem, IdentityHealth{IdentityState, KeyState, Problems}, Classify (pure), ClassifyState (single-label precedence), crossReferenceUnusedKeys (pure)
  - internal/identity/inventory.go — Inventory{Identities, UnusedKeys}, InventoryDeps, BuildInventory(deps) (impure aggregator), BuildInventoryDeps() (real wiring, Include-aware ReadSSHConfig)
affects: [phase-5-identity-manager, 01-06-debug-list-command]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Two-layer classifier split: a PURE Classify/ClassifyState over Account+booleans (state.go) vs an IMPURE fact-gathering BuildInventory behind an injectable Deps seam (inventory.go) — mirrors internal/platform's Deps/BuildProbeDeps pattern"
    - "Orthogonal two-axis health report (IdentityState + KeyState + Problems) instead of one collapsed label, so co-occurring facts never hide each other; single-label ClassifyState retained via a DOCUMENTED precedence order for legacy single-state callers"
    - "Mirrored (not shared) config.d/*.config glob literal — ACCEPTED DUPLICATION (MEDIUM #4 option b) to preserve Wave-1 DAG independence from 01-03; both files carry a keep-in-sync comment naming their counterpart"
    - "BuildInventoryDeps() EXPORTED real-wiring constructor, test-exercised for all-non-nil fields, closing the project's documented injected-seam wiring blindspot"

key-files:
  created:
    - internal/identity/state.go
    - internal/identity/state_test.go
    - internal/identity/inventory.go
    - internal/identity/inventory_test.go
  modified: []

key-decisions:
  - "A key used only for git commit signing (keyUsedInGit=true, keyUsedInSSH=false) is bucketed key-used-both rather than mislabeled key-unused — the locked 8-label MGR-02 vocabulary has no dedicated 'git-signing-only' key state, and the key IS actively used"
  - "ClassifyState precedence is structural-before-key: fragment-path-missing > git-only > incomplete > key-missing > key-unused > key-used-ssh-only > complete — documented in a doc comment on ClassifyState so the ordering is a contract, not an implementation detail"
  - "BuildInventory calls deps.ReadFragment a second time per identity (in addition to the read Reconstruct already performs internally) to recover FragmentInfo.SigningKey/GPGFormat/CommitSign for the keyUsedInGit fact — Account does not carry those fields, and touching Reconstruct's signature/output shape was out of scope for this plan"
  - "listKeyFilesReal globs ~/.ssh/id_* and excludes .pub siblings, matching keygen.KeyPaths' private-key naming convention (id_<algo>_<identity>) — not a machine-wide key discovery, only gitid's own naming convention"

requirements-completed: [MGR-02, DLV-07]

# Metrics
duration: ~25min
completed: 2026-07-03
---

# Phase 1 Plan 4: Identity State Taxonomy + Include-Aware Inventory Builder Summary

**Two-layer MGR-02 identity health taxonomy: a pure `Classify`/`ClassifyState` returning orthogonal IdentityState/KeyState axes plus a Problems list, and an impure `BuildInventory` aggregator with an Include-aware `ReadSSHConfig` seam so identities classify correctly in either SSH storage layout (in-file or the STORE-01 Include'd `config.d/gitid.config`).**

## Performance

- **Duration:** ~25 min
- **Tasks:** 3/3 completed
- **Files created:** 4 (state.go, state_test.go, inventory.go, inventory_test.go)

## Accomplishments

- `State` declares exactly the 8 locked MGR-02 labels (complete, incomplete, git-only, key-unused, key-used-ssh-only, key-used-both, key-missing, fragment-path-missing) as named constants, proven by a set-equality test
- `crossReferenceUnusedKeys(keyPaths, referencedIdentityFiles)` is a pure set-difference mirroring the doctor Orphans check's Class 3 unused-key logic, extracted once here (not duplicated in doctor) per RESEARCH.md Open Question 2
- `IdentityHealth{Name, IdentityState, KeyState, Problems}` reports two orthogonal axes plus a Problems list, so an identity that is BOTH fragment-path-missing AND key-missing surfaces both facts distinctly instead of collapsing into one label — proven by two dedicated overlap rows in the table test (in addition to one dedicated row per locked label)
- `Classify(acct, keyExists, keyUsedInSSH, keyUsedInGit)` is a pure decision function over `Reconstruct`'s `Account` output — no filesystem access, no sidecar DB; `ClassifyState` collapses the two axes into one label via a documented precedence order (structural blockers before key-axis problems) for legacy single-state callers
- `BuildInventory(deps)` is the impure aggregation layer: it reads the managed SSH/gitconfig bytes, calls `Reconstruct`, resolves keyExists/keyUsedInSSH/keyUsedInGit per identity via the injected `InventoryDeps` seam, calls `Classify`, and computes the global `UnusedKeys` list — this is what 01-06's debug/list command will consume instead of rebuilding identity logic in `cmd/gitid`
- `BuildInventoryDeps()`'s real `ReadSSHConfig` is Include-aware: it reads `~/.ssh/config` then globs+merges every `~/.ssh/config.d/*.config` file's bytes, so `ParseManagedHosts` (a raw sentinel-byte scan that does not resolve `Include`) sees managed blocks in EITHER storage layout — proven end-to-end by `TestBuildInventoryIncludeLayout`, which seeds an identity ONLY inside `~/.ssh/config.d/gitid.config` (main config has just the `Include` line) under a sandboxed `HOME` and asserts it classifies correctly through the REAL `BuildInventoryDeps()` + `BuildInventory` (upholds D-11, no layout carve-out)
- The `config.d/*.config` glob is the IDENTICAL literal 01-03's `internal/sshconfig/include.go` defines as canonical, mirrored (not imported as a shared symbol) per the ACCEPTED DUPLICATION note (MEDIUM #4 option b) — both files carry a `keep in sync` comment naming their counterpart, verified via the acceptance grep on both files
- `BuildInventoryDeps()` real wiring is test-exercised for all-non-nil function fields, closing the project's documented injected-seam wiring blindspot

## Task Commits

Each task followed the RED→GREEN TDD cycle; RED and GREEN were folded into one logical commit per CLAUDE.md's "logical groups, not small chunks" commit policy (explicitly permitted by the TDD execution flow):

1. **Task 1: State vocabulary (8 labels) + key cross-reference/existence helpers (MGR-02 inputs, interface-first)** - `ac19768` (test)
2. **Task 2: IdentityHealth report (orthogonal axes + Problems) + single-label ClassifyState precedence (MGR-02, DLV-07)** - `faaaf0c` (feat)
3. **Task 3: State-inventory builder — gather real facts (Include-aware) + call classifier behind an injectable seam (MGR-02, DLV-07)** - `20bee89` (feat)

**Plan metadata:** committed separately as part of this step's final commit (see below).

## TDD Gate Compliance

All three tasks were written test-and-implementation together (not shipped as separate failing-then-passing commits) per CLAUDE.md's explicit commit-granularity rule and the plan's own permission to fold RED+GREEN of one logical change into one commit. Every test set passed on its FIRST run against the corresponding implementation (no debugging iterations needed):

- Task 1: `go test ./internal/identity/... -run 'TestCrossReferenceUnusedKeys|TestStateConstants' -race` — 4/4 pass on first run.
- Task 2: `go test ./internal/identity/... -run 'TestClassify|TestClassifyState' -race` — 10/10 subtests pass on first run (9-row table + 1 dedicated precedence test).
- Task 3: `go test ./internal/identity/... -run 'TestBuildInventory|TestBuildInventoryDeps|TestBuildInventoryIncludeLayout' -race` — 5/5 pass on first run, including the real-wiring `TestBuildInventoryIncludeLayout` end-to-end proof.

Gate-sequence commit prefixes are present in order: `test(01-04): ...` (`ac19768`) precedes both `feat(01-04): ...` commits (`faaaf0c`, `20bee89`).

## Files Created

- `internal/identity/state.go` - `State` (8 constants), `Problem` (5 constants), `IdentityHealth`, `Classify`, `ClassifyState`, `crossReferenceUnusedKeys`, `missingSet`
- `internal/identity/state_test.go` - `TestStateConstants`, `TestCrossReferenceUnusedKeys_{None,SomeUnused,Empty}`, `TestClassify` (9-row table: 7 single-label + 2 overlap), `TestClassifyState_PrecedenceStructuralBeforeKey`
- `internal/identity/inventory.go` - `Inventory`, `InventoryDeps`, `BuildInventory`, `resolveKeyUsedInGit`, `configDirGlob` (mirrored literal), `BuildInventoryDeps`, `readSSHConfigIncludeAware`, `readGitconfigReal`, `listKeyFilesReal`
- `internal/identity/inventory_test.go` - `TestBuildInventory` (5-identity fake-deps fixture, all axes + global UnusedKeys), `TestBuildInventory_{ReadSSHConfigError,ListKeyFilesError}`, `TestBuildInventoryDeps` (non-nil fields), `TestBuildInventoryIncludeLayout` (real-wiring D-11 proof)

## Decisions Made

- A key used only for git commit signing (`keyUsedInGit=true`, `keyUsedInSSH=false`) is bucketed `key-used-both` rather than mislabeled `key-unused` — the locked 8-label MGR-02 vocabulary has no dedicated "git-signing-only" key state, and the key IS actively used; documented in `Classify`'s doc comment.
- `ClassifyState`'s precedence order (structural axis before key axis: fragment-path-missing > git-only > incomplete > key-missing > key-unused > key-used-ssh-only > complete) is written as a numbered doc comment on the function itself, making the ordering a contract for future callers rather than an implicit implementation detail.
- `BuildInventory` calls `deps.ReadFragment` a second time per identity (in addition to the read `Reconstruct` already performs internally) to recover `FragmentInfo.SigningKey`/`GPGFormat`/`CommitSign` for the `keyUsedInGit` fact — `Account` does not carry those fields, and changing `Reconstruct`'s signature/output shape was out of scope for this plan (it is a Wave-1-shared function other tasks/tests already depend on byte-for-byte).
- `listKeyFilesReal` globs `~/.ssh/id_*` and excludes `.pub` siblings, matching `keygen.KeyPaths`' private-key naming convention (`id_<algo>_<identity>`) rather than attempting a general-purpose key discovery — consistent with gitid only ever managing its own `id_*`-named keys.
- The `config.d/*.config` glob literal in `inventory.go` is the IDENTICAL string 01-03's `include.go` defines as canonical, deliberately duplicated rather than imported as a shared exported symbol — importing it would force `01-04` to `depend_on: [01-03]`, cascading a re-wave of the Phase 1 DAG (per the plan's own ACCEPTED DUPLICATION note, MEDIUM #4 option b). Both files carry a `keep in sync` comment naming their counterpart; the acceptance grep on both files was verified during execution.

## Deviations from Plan

None — plan executed exactly as written. `resolveKeyUsedInGit` and the double `ReadFragment` call are direct, in-scope implementations of the plan's own `<action>` text ("keyUsedInGit = the fragment enables ssh signing for that key"); no architectural changes, no scope additions beyond the plan's `must_haves`/`acceptance_criteria`.

## Issues Encountered

None. All test suites (state.go's Task 1/2 table tests, inventory.go's Task 3 fake-deps + real-wiring tests) passed on their first execution against the implementation. Two minor lint/format fixups were applied before commit (a `goimports` line-wrap re-run and a `//nolint:revive` directive on the plan-locked `IdentityHealth` name, which intentionally does not follow the revive "stutters" suggestion) — neither changed behavior.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- 01-06 (debug/list command) can call `identity.BuildInventory(identity.BuildInventoryDeps())` directly to render per-identity health without rebuilding any classification logic in `cmd/gitid`.
- Phase 5 (identity manager) can render `IdentityHealth`'s two axes as separate UI rows/badges, or use `ClassifyState` for a single-label summary view, per its own UX needs.
- 01-03 (concurrent Wave 1) independently pins the identical `config.d/*.config` glob literal as canonical in `internal/sshconfig/include.go`; both plans' acceptance greps confirm the literals match — no shared symbol, no re-wave, no cross-file consistency drift detected during this session.
- The doctor package is untouched by this plan (deliberately, to avoid a Wave-1 file conflict with 01-03's `orphans.go` change) — any future doctor-side dedup against this taxonomy is a Phase 8 concern, as noted in the plan's objective.
- No blockers.

---
*Phase: 01-foundations-spikes-ci*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 4 created files confirmed present on disk (`internal/identity/state.go`, `state_test.go`, `inventory.go`, `inventory_test.go`); all three task commits (`ac19768`, `faaaf0c`, `20bee89`) confirmed present in `git log --oneline --all`.
