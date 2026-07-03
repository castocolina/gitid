---
phase: 01-foundations-spikes-ci
plan: 03
subsystem: sshconfig
tags: [go, kevinburke/ssh_config, filewriter, ssh-config, transactional-migration, doctor]

# Dependency graph
requires:
  - phase: 01-foundations-spikes-ci (plan 01)
    provides: injectable exec.CommandContext probe seam pattern (mirrored by MigrateDeps/AdoptDeps' injectable-function-field seams)
provides:
  - internal/sshconfig/include.go — EnsureIncludeLine (floored, idempotent Include line), EnsureIncludeDir (0700), IsReservedBlockName; canonical config.d/*.config glob literal
  - internal/sshconfig/adopt.go — DetectInclude (pure text scan, order-preserving) + Adopt (rule-based selection: sentinel-bearing/caller-chosen, absolute/~/.ssh-relative, non-symlink, unambiguous) via AdoptDeps
  - internal/sshconfig/migrate.go — Migrate cross-file transactional reversible migration (backup both → write destination → validate → trim source → validate → commit) with auto-rollback via MigrateDeps
  - internal/doctor/checks/orphans.go — SSH-side sshconfig.IsReservedBlockName guard in the Class 1 loop, closing the reserved-block false-positive loop on the SSH side
affects: [phase-3-create-flow, phase-5-identity-manager, 01-04-inventory-reconstruction]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Managed-block-as-floor for the SSH Include line (filewriter.PrependBlockIfNotFound), mirroring gitconfig.WriteBaselineInclude's [include] placement exactly"
    - "DetectInclude as a deliberate pure text scan, NOT ssh_config.Decode — Decode performs real filesystem glob+read as a side effect of resolving Include directives (Pitfall 5); Adopt does its own controlled, injectable glob resolution instead"
    - "os.UserHomeDir (respects $HOME) used for gitid's own tilde-expansion instead of os/user.Current (ignores $HOME on darwin) — verified empirically both ways this session"
    - "Cross-file transaction: add-to-destination-before-remove-from-source ordering + both-file backup-before-any-write + per-step ssh -G validation + auto-rollback-on-any-failure (including an injected afterStep hook), converging idempotently on re-run"
    - "AdoptDeps / MigrateDeps (never a bare Deps) — two same-named `type Deps` in one Go package is a compile error; this convention is now established for internal/sshconfig"

key-files:
  created:
    - internal/sshconfig/include.go
    - internal/sshconfig/include_test.go
    - internal/sshconfig/adopt.go
    - internal/sshconfig/adopt_test.go
    - internal/sshconfig/migrate.go
    - internal/sshconfig/migrate_test.go
  modified:
    - internal/doctor/checks/orphans.go
    - internal/doctor/checks/orphans_test.go

key-decisions:
  - "config.d/*.config glob literal is CANONICAL in include.go, deliberately duplicated (not shared) by 01-04's internal/identity/inventory.go — a shared exported symbol would force a re-wave of the Phase 1 DAG (MEDIUM #4 option b, ACCEPTED DUPLICATION); both files carry a 'keep in sync' comment naming their counterpart"
  - "Adopt's selection rule for a bare-relative or non-~/.ssh-relative Include path is stricter than real OpenSSH's own resolution (which treats bare-relative as implicitly ~/.ssh-relative) — a deliberate, defensive gitid-specific safety boundary, not a claim about what ssh itself would do"
  - "Migrate always validates ssh -G resolution against the REAL ~/.ssh/config entry point (never an isolated destination-only staged file) — the entry point ssh itself reads by default, keeping validation semantics uniform across both directions"
  - "An empty backupPath from filewriter.Write means the file did NOT pre-exist before migration, not 'nothing to restore' — rollback must RemoveFile (not no-op) to reach the true pre-migration absent state; found and fixed during GREEN via a genuinely failing test (TestMigrateInjectedFailureAfterSourceTrimmedRollsBack)"
  - "movableBlockNames excludes both the reserved ssh-include block AND the macOS _global Host * block — only per-identity content migrates; reorderGlobalLast preserves the existing 'global block always last' first-match-wins invariant after ReplaceBlock may append new blocks after it"

patterns-established:
  - "Pattern 3 (Managed-Block-as-Floor) and Pattern 4 (real ssh -G proof) from 01-RESEARCH.md, now implemented verbatim in include.go/migrate_test.go/adopt_test.go"

requirements-completed: [STORE-01, STORE-02, STORE-03, STORE-04, TOOL-04]

# Metrics
duration: ~35min
completed: 2026-07-03
---

# Phase 1 Plan 3: Dual SSH-Config Storage — Include, Adopt, Migrate Summary

**Gitid-owned Include'd SSH config layout (floored `Include` line, 0700 dir / 0600 file) with rule-based adoption of an existing external Include and a cross-file transactional, reversible migration between in-file and Include'd layouts — every claim proven with real `ssh -G`, not faked.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-07-03T00:55:00Z (approx.)
- **Completed:** 2026-07-03T01:30:24Z
- **Tasks:** 3/3 completed
- **Files modified:** 8 (6 created, 2 modified)

## Accomplishments

- `EnsureIncludeLine` floors a single gitid-managed `Include ~/.ssh/config.d/*.config` line at the TOP of `~/.ssh/config` via `filewriter.PrependBlockIfNotFound`, idempotent (re-running does not duplicate), re-parsing the composed config before writing (refuse-to-corrupt)
- `EnsureIncludeDir` wraps `filewriter.EnsureDir` at 0700, chmod'ing an already-existing looser directory back to 0700; the Include'd file is proven at 0600 — permission bits proven via `os.Stat().Mode().Perm()`, not just file presence
- `IsReservedBlockName` mirrors `gitconfig.IsReservedBlockName`; `internal/doctor/checks/orphans.go`'s Class 1 loop now skips it, closing the SSH-side half of the documented reserved-block false-positive loop (Pitfall 4) in the SAME change that introduces the reserved block
- Real `ssh -G -F <tmpconfig> personal.github.com` resolves the `IdentityFile` from a filesystem-backed `config.d/gitid.config` fixture under a hermetic `t.TempDir()` HOME — first-match-wins through the Include'd file, proven with the real binary
- `DetectInclude` scans `~/.ssh/config`'s raw text for every `Include` directive in file order (a deliberate pure text scan, not `ssh_config.Decode`, which performs real filesystem I/O as a side effect of resolving `Include` — Pitfall 5), parsing quoted and unquoted, absolute and `~`-relative path tokens
- `Adopt` applies the selection rules end-to-end: a candidate is adoptable only if its path is absolute or `~/.ssh`-relative, it is not a symlink (`os.Lstat` guard), and either it carries a gitid sentinel and its glob resolves unambiguously to exactly one file (`AdoptSentinelBearing`) or the caller explicitly confirmed it (`AdoptCallerChosen`); no qualifying directive falls back to `AdoptCreateConfigD`
- `Migrate` implements the five-step cross-file transaction (preflight snapshot → backup both files → write destination first → validate → trim source (+ floor the Include line in the SAME write for `MigrateToInclude`) → validate final state → commit), proven behavior-preserving both directions with real `ssh -G` before/after snapshot equality
- Any post-write validation failure, or an injected `afterStep` failure (the test seam simulating a mid-transaction crash), triggers automatic rollback of BOTH files from their step-2 backups; re-running `Migrate` after a crash-induced duplicate (or after a complete migration) converges idempotently

## Task Commits

Each task followed the RED→GREEN TDD cycle; RED and GREEN were folded into one logical commit per CLAUDE.md's "logical groups, not small chunks" commit policy (explicitly permitted by the TDD execution flow):

1. **Task 1: Include-line floor placement + config.d dir(0700)/file(0600) perms + reserved-block guard + orphans SSH-side skip (STORE-01, STORE-04)** - `a85482f` (feat)
2. **Task 2: Detect + rule-based adoption of an existing external Include directive (STORE-02)** - `5e51f2d` (feat)
3. **Task 3: Cross-file transactional, reversible, backed-up in-file ↔ Include'd migration with recovery (STORE-03, STORE-04, TOOL-04)** - `40bda87` (feat)

**Plan metadata:** committed separately as part of this step's final commit (see below).

## TDD Gate Compliance

All three tasks followed the RED→GREEN cycle but were committed as single `feat` commits, not separate `test`/`feat` commits, per CLAUDE.md's explicit commit-granularity rule and the plan's own permission to fold RED+GREEN of one logical change into one commit.

RED was verified as a genuine failing/uncompiling state before implementation, with real command output for each task:
- Task 1: `go test ./internal/sshconfig/... -run 'TestEnsureIncludeLine|...' -race` failed (`Include block is not floored`, `IsReservedBlockName("ssh-include") = false, want true`, etc.) and `go test ./internal/doctor/... -run TestOrphan...` failed `TestOrphanReservedSSHIncludeNotFlagged` before the guard existed.
- Task 2: `go test ./internal/sshconfig/... -run 'TestDetectInclude|TestAdopt' -race` failed every non-trivial assertion (empty-result cases passed trivially against the stub, as expected) before `adopt.go`'s real logic existed.
- Task 3: `go test ./internal/sshconfig/... -run TestMigrate -race` failed every assertion that exercised real migration behavior against the RED stub (one weak assertion — foreign-content preservation — passed trivially against a no-op stub, as expected for that specific check).

No RED-state assertion ever passed unexpectedly in a way that indicated a broken test (fail-fast rule respected — the isolated trivial passes above are inherent to a no-op stub, not signals of test defects). All three tasks reached full GREEN (every listed test passing, `-race` clean) before commit.

## Files Created/Modified

- `internal/sshconfig/include.go` - `EnsureIncludeLine`, `EnsureIncludeDir`, `IsReservedBlockName`, canonical `config.d/*.config` glob literal
- `internal/sshconfig/include_test.go` - floor placement, idempotency, missing-file tolerance, 0700/0600 permission proof, real `ssh -G` Include-resolution proof
- `internal/sshconfig/adopt.go` - `IncludeDirective`, `DetectInclude`, `AdoptMethod`, `AdoptDeps`, `RealAdoptDeps`, `Adopt`, `AdoptResult`
- `internal/sshconfig/adopt_test.go` - `DetectInclude` order/quoting/expansion tests + a table-driven `Adopt` selection-rule matrix (9 rows) using real `t.TempDir()` fixtures (including a real symlink)
- `internal/sshconfig/migrate.go` - `MigrateDirection`, `MigrateStep`, `MigrateResult`, `MigrateDeps`, `RealMigrateDeps`, `Migrate` + the composeDestination/composeSource/rollback/reorderGlobalLast helpers
- `internal/sshconfig/migrate_test.go` - both-direction migration + resolution-preservation tests, idempotent re-run test, foreign-content preservation test, two injected-failure rollback tests (after destination write, after source trim)
- `internal/doctor/checks/orphans.go` - Class 1 loop now skips `sshconfig.IsReservedBlockName` blocks
- `internal/doctor/checks/orphans_test.go` - `TestOrphanReservedSSHIncludeNotFlagged` (SSH-side mirror of the existing gitconfig-side reserved-block test)

## Decisions Made

- The `config.d/*.config` glob literal is defined here as CANONICAL and deliberately duplicated (not shared via an exported symbol) by 01-04's `internal/identity/inventory.go` — extracting a shared constant would force 01-04 to `depend_on` 01-03, cascading a re-wave of the Phase 1 DAG (01-04→wave2, 01-06→wave3, 01-07→wave4). Both files carry a `keep in sync` comment naming their counterpart instead, per MEDIUM #4 option b.
- `Adopt`'s selection rule rejects a bare-relative Include path (no `~/.ssh/` or absolute prefix) even though real OpenSSH's own `Include` resolution would treat a bare-relative path as implicitly `~/.ssh`-relative — this is a deliberate, stricter gitid-specific safety boundary (never auto-adopt on an assumption), not a claim about actual `ssh` behavior.
- `expandIncludePath`/`Adopt` use `os.UserHomeDir()` (which respects `$HOME`) rather than `os/user.Current()` (which the `kevinburke/ssh_config` library itself uses internally and which ignores `$HOME` on darwin, verified empirically this session with a standalone Go program) — this keeps gitid's own path expansion hermetic and testable via `t.Setenv("HOME", ...)`, while the REAL `ssh` binary (used for all resolution-proof assertions) was separately verified to honour `$HOME` for `Include` tilde-expansion.
- `Migrate` always validates `ssh -G` resolution against the real `deps.ConfigPath` entry point (never an isolated staged copy of just the destination file) — this keeps validation semantics identical for both `MigrateToInclude` and `MigrateToInFile` and matches exactly what the real `ssh` client would see.
- Bug found and fixed during Task 3's GREEN phase: `filewriter.Write`'s `backupPath` return is empty both when "nothing needs restoring" and when "the file did not pre-exist" — these are NOT the same case. `restoreFromBackup` now calls a new `MigrateDeps.RemoveFile` seam (wired to `os.Remove`, tolerating already-missing) when `backupPath == ""`, so rollback correctly deletes a file that was created mid-transaction rather than leaving it with whatever a later step wrote. Caught by a genuinely failing test (`TestMigrateInjectedFailureAfterSourceTrimmedRollsBack`) before being fixed — not a silently-passing gap.
- `movableBlockNames` excludes both `IsReservedBlockName` blocks and the macOS `_global` (`Host *`) block from migration — only per-identity content moves. `reorderGlobalLast` re-positions `_global` to the end of the composed file when present, preserving the existing "global block always last" first-match-wins invariant that `ReplaceBlock`'s append-on-not-found behavior could otherwise disturb.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] `Migrate` must create the destination's parent directory before its first write**
- **Found during:** Task 3, writing `TestMigrateToIncludeMovesBlockAndPreservesResolution`
- **Issue:** On a first-ever `MigrateToInclude` run, `~/.ssh/config.d/` does not exist yet; `filewriter.Write`'s `os.CreateTemp(dir, ...)` would fail with "no such file or directory" since the plan's Task 3 `<action>` text describes composing with `EnsureIncludeDir` from Task 1 but the RED stub had no such call
- **Fix:** `Migrate` now calls `EnsureIncludeDir(filepath.Dir(destPath))` up front when `direction == MigrateToInclude`, before step 1, guaranteeing the destination has somewhere to land at the correct 0700 mode
- **Files modified:** internal/sshconfig/migrate.go
- **Verification:** `TestMigrateToIncludeMovesBlockAndPreservesResolution` and `TestMigrateIdempotentReRunConverges` (both start from a config.d-less fixture) pass
- **Committed in:** `40bda87` (Task 3 commit)

**2. [Rule 1 - Bug] Rollback left mid-transaction content in place when a file did not pre-exist**
- **Found during:** Task 3, `TestMigrateInjectedFailureAfterSourceTrimmedRollsBack` failing during GREEN
- **Issue:** `restoreFromBackup` treated an empty `backupPath` as "nothing to restore," but an empty `backupPath` from `filewriter.Write` actually means "the file did not pre-exist" — after a successful migration the Include'd file DOES exist (created by step 3), so a no-op rollback left the migrated block behind instead of restoring the true pre-migration (absent) state
- **Fix:** Added `MigrateDeps.RemoveFile` (wired to `os.Remove`, idempotent on already-missing) and changed `restoreFromBackup` to call it when `backupPath == ""` instead of no-op-ing
- **Files modified:** internal/sshconfig/migrate.go
- **Verification:** `TestMigrateInjectedFailureAfterSourceTrimmedRollsBack` passes; re-ran the full `-race` suite for the package afterward
- **Committed in:** `40bda87` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 Rule 2 - missing critical functionality, 1 Rule 1 - bug fix)
**Impact on plan:** Both fixes were necessary for the transaction's correctness/recoverability guarantees explicitly required by the plan's `must_haves` and threat model (T-01-22); no scope creep beyond what Task 3's own acceptance criteria already demanded.

## Issues Encountered

None beyond the two auto-fixed items above, both caught by genuinely failing tests during GREEN (not discovered after the fact).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 3 (create flow) can call `EnsureIncludeDir`/`EnsureIncludeLine` to establish the Include'd layout, or continue writing in-file via the existing `sshconfig.Write`, for a newly created identity.
- Phase 5 (identity manager) can call `DetectInclude`/`Adopt` to offer adoption of a user's existing external `Include`, and `Migrate` to let a user switch layouts for an existing identity reversibly.
- 01-04 (`internal/identity/inventory.go`, running concurrently in Wave 1) independently pins the identical `config.d/*.config` glob literal per the ACCEPTED DUPLICATION note — no shared symbol, no re-wave.
- The doctor Orphans check now correctly ignores the `ssh-include` reserved block on both the SSH and gitconfig sides, closing the documented reserved-block false-positive loop for this new managed block from day one.
- No blockers.

---
*Phase: 01-foundations-spikes-ci*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 8 created/modified files confirmed present on disk; all three task commits (`a85482f`, `5e51f2d`, `40bda87`) confirmed present in `git log --oneline --all`.
