---
phase: 06-global-ssh-options
plan: 05
subsystem: sshconfig
tags: [migration, locking, concurrency, storage-layout]
requires:
  - phase: 06-global-ssh-options
    provides: [globalssh engine, name registry, classifier, shadow simulation (06-01..06-04)]
provides:
  - "PlanMigration/MigrateWithPlan split with a plan-digest lock contract closing the preview-to-confirm race"
  - "filewriter.Backup, a pure timestamped-copy primitive that never replaces its target"
  - "The Storage & preview sub-tab wired to the real migration engine via SSHStoragePlanner, ending the view's demo banner"
  - "runSSHStorageMigrate, the single write authority for storage-layout migration, mirroring runGlobalSSHApply's shape"
  - "The <lock_contract> rule 5 transient-window proof: txMu genuinely serializes a concurrent preview against a migration's two-write window"
  - "Real-PTY coverage of both Global SSH sub-tabs' storage migration ceremony against the compiled binary"
affects: [06-06 (CLI verb reuses runSSHStorageMigrate), 06-07 (dummy/real parity gate)]
actuals:
  tokens: 145000
  tasks: 3
  commits: 4
tech-stack:
  added: []
  patterns:
    - "Plan/commit split with digest verification: a pure planning function computes content digests; the write entry point re-reads and compares before taking any backup, aborting with a distinct sentinel error rather than silently recomposing."
    - "A dedicated backup-only primitive (never call the write path with unchanged bytes to fake a backup — that makes the backup itself a content-changing write)."
    - "A separate, narrower mutex (pendingMigrationMu) for a short-lived cross-request slot, distinct from the long-held transaction mutex (txMu) — with the acquisition order and the non-reentrancy hazard stated in both call sites' doc comments."
    - "An opaque digest token crossing the tuikit/backend seam to carry plan IDENTITY without a backend type crossing into the render package."
key-files:
  created:
    - internal/globalssh — untouched this plan (already existed)
    - cmd/gitid/wiring_storage_test.go — the Task 2 acceptance-criteria test suite (24+ tests)
    - e2e/global_ssh_storage_pty_e2e_test.go — Task 3's 6-case real-PTY suite
  modified:
    - internal/sshconfig/migrate.go — PlanMigration/MigrateWithPlan, ErrConfigChangedSincePreview, per-step digest checks
    - internal/filewriter/filewriter.go — Backup primitive
    - internal/tuikit/backend.go, views.go, globalssh.go — SSHStoragePlanner seam, SSHStorageMigrationView/SSHStorageCommitMsg, async storage ceremony
    - cmd/gitid/lifecycle.go — runSSHStorageMigrate
    - cmd/gitid/wiring.go — SSHStorageMigrationPlan, CommitSSHStorage, putPendingMigration/takePendingMigration, DemoBanner, Persist(SetSSHStorage)
    - internal/dummytui/fixturebackend.go — dummy implementation of the new seam, byte-identical fixture output
key-decisions:
  - "The plan-digest re-check runs BEFORE any backup — an external edit between preview and confirm is detected while the disk is still untouched, not after a backup has already been taken."
  - "pendingMigrationMu is a separate mutex from txMu specifically because takePendingMigration is called from inside a txMu hold (runSSHStorageMigrate); Go's sync.Mutex is not reentrant, so reusing txMu there would self-deadlock — stated explicitly in both call sites' doc comments as the cycle-3-style hazard reappearing at a new call site."
  - "The CLI branch (empty planToken, plan 06-06's future caller) calls sshconfig.PlanMigration directly rather than through b.SSHStorageMigrationPlan, for the same non-reentrancy reason — pinned by a construction-check test that greps for the absence of that call site."
patterns-established:
  - "Content-digest lock contract for any future two-file (or multi-file) transactional migration: plan once with digests, verify before backup, verify again before each write."
requirements-completed: [GSSH-01]
duration: 210min
completed: 2026-08-27
status: complete
---

# Phase 06-05: Storage & preview sub-tab hardening Summary

**The Storage & preview sub-tab is now real — driven by a migration engine whose preview and commit share one digest-verified plan object, whose locking is proven (not just asserted) to close the transient two-file window a naive implementation would silently expose, and whose ceremony is covered end-to-end by real-PTY keystrokes against the compiled binary.**

## Performance
- **Duration:** ~210 min across 4 dispatch cycles (1 initial + 3 resumes, see Issues Encountered)
- **Tasks:** 3
- **Files modified:** 34 (4096 insertions, 195 deletions)

## Accomplishments
- Closed the cycle-2 HIGH: `PlanMigration` is now pure and digest-carrying; `MigrateWithPlan` is the one writing entry point, re-verifying both files against the plan's digests before taking a single backup and aborting with `ErrConfigChangedSincePreview` (naming the file) if anything moved under the preview.
- Retired the "backup by calling the write path with the same bytes" trick (`filewriter.Backup` is a pure copy) — the old mechanism made the backup step itself capable of silently destroying an external edit made between preflight and backup.
- Wired the Storage & preview sub-tab to the real engine: `SSHStoragePlanner` (a seam deliberately separate from `GlobalSSHPlanner`), an async migration ceremony mirroring the Global SSH apply ceremony's shape, and `runSSHStorageMigrate` as the single write authority holding `txMu` for the whole transaction.
- **Proved** (not just implemented) the `<lock_contract>` rule 5 claim that `go test -race` cannot see: a concurrent preview opened while a migration sits between its destination write and its source trim never observes a managed block present in both files at once, because `SSHStorageMigrationPlan` takes `txMu` before any disk read and blocks until the transaction releases it.
- Real-PTY coverage of both Global SSH sub-tabs' migration flow: browse, cancel, confirm, round-trip, and the config-changed-since-preview refusal, against the compiled real binary.

## Task Commits
1. **Task 1: Migration engine hardening — plan/commit digest lock contract** - `903f7ff` (feat)
2. **Task 2a: Wire the Storage sub-tab to the real engine** - `adc31cc` (feat)
2. **Task 2b: Pin the lock contract with the acceptance-criteria test suite** - `1e95fdb` (feat)
3. **Task 3: Raw-keystroke PTY coverage of the Storage & preview sub-tab** - `b6bfc68` (feat)

**Plan metadata:** this commit (docs: add plan summary)

## Files Created/Modified
- `internal/sshconfig/migrate.go` — `PlanMigration`, `MigrateWithPlan`, `ErrConfigChangedSincePreview`, per-step pre-backup/pre-write digest checks, `filewriter.Backup`-based snapshotting
- `internal/filewriter/filewriter.go` — `Backup`, the pure timestamped-copy primitive
- `internal/tuikit/backend.go` — `SSHStoragePlanner` interface, `NoopSSHStoragePlanner`
- `internal/tuikit/views.go` — `SSHStorageMigrationView`, `SSHStorageCommitMsg`
- `internal/tuikit/globalssh.go` — `renderStorage` sourced from the view (not the fixture helpers), async `storageCeremonyFor`, `activate` fetches the view, layout-selection movement clears/refetches it
- `internal/dummytui/fixturebackend.go` — implements the new seam from the same fixtures, byte-identical dummy render (golden-text pinned)
- `cmd/gitid/lifecycle.go` — `runSSHStorageMigrate`, registered `storage-migrate` stages
- `cmd/gitid/wiring.go` — `SSHStorageMigrationPlan` (txMu-guarded read), `CommitSSHStorage` (async commit seam), `putPendingMigration`/`takePendingMigration` (pendingMigrationMu), `DemoBanner` excludes `TabGlobalSSH`, `Persist(SetSSHStorage)` is real-owned
- `cmd/gitid/wiring_storage_test.go` — the Task 2 acceptance-criteria suite (~24 tests): round trips, concurrent-modification abort, config-changed-since-preview (positive + unrelated-file negative), plan-token consume-once/stale/bogus-eviction, the deadlock regression pair, and the rule-5 transient-window proof
- `e2e/global_ssh_storage_pty_e2e_test.go` — 6 real-PTY cases plus an Include-aware fake `ssh` proof test
- `cmd/gitid/wiring_test.go` — updated two pre-existing tests whose assertions predated this plan (`TestDemoBannerOnlyIdentitiesIsWired`, `TestPersistDemoOnlyActionsPreserveTuikitReduce`)

## Decisions Made
- Digest comparison runs against `plan.Digests` computed once at preview time, never recomputed at commit — carrying the plan OBJECT (not just re-calling the planning function) is what closes the race a naive "call PlanMigration again at commit" would reopen.
- `pendingMigrationMu` is intentionally a separate, narrower mutex from `txMu`, guarding only the single-slot pending-plan handoff — never held across I/O, never taken from inside a `txMu` hold by any caller other than the two helper functions, which themselves never take `txMu`.
- The CLI path (plan 06-06, empty `planToken`) calls `sshconfig.PlanMigration` directly rather than routing through `b.SSHStorageMigrationPlan`, specifically to avoid `runSSHStorageMigrate` self-deadlocking on `txMu`'s non-reentrancy — documented at both call sites and pinned by a construction-check test.

## Deviations from Plan
None in scope — all three tasks match the plan's `<action>`/`<acceptance_criteria>` blocks. Process deviations (not scope deviations), documented for institutional memory:

1. **`internal/sshconfig/migrate.go` dead-code cleanup (Task 1):** the cross-AI run's first pass left an old `rollback` helper unused after introducing `rollbackTracked`. Removed by the orchestrator; `restoreSnapshot` (which `rollback` called and `rollbackTracked` also uses) was kept. Verified no other call sites existed before deleting.
2. **Two lint-formatting fixes** applied directly by the orchestrator across the dispatch cycles (goimports via the full `$(go env GOPATH)/bin/goimports` path — `rtk goimports` does not exist and was never the right invocation).
3. **`renderStorage` build breakage recovered mid-flight (Task 2):** an earlier crash left three preview helpers renamed to exported names (for the dummy fixture backend's exclusive use) but `renderStorage` still calling the old lowercase names. This was NOT a naming mistake to revert — it was the correct first half of wiring `renderStorage` to `SSHStorageMigrationPlan`'s real bytes instead. A relaunch finished the wiring correctly.
4. **Task 2's acceptance-criteria test suite was initially missing entirely** after the wiring-only commit (`adc31cc`) — the orchestrator verified this gap explicitly (near-zero diff in the test files) before relaunching specifically for the test suite, rather than accepting the wiring alone as "Task 2 complete."
5. **Fake `ssh` script bug found and fixed by the orchestrator (Task 3):** `find_identity_file`'s `case` pattern matched `IdentityFile*` against raw config lines, but every managed directive is indented under its `Host` block (`"  IdentityFile ..."`) — shell `case` patterns anchor at the string start, so the match silently failed and resolution fell back to a stub default. Verified with a standalone shell reproduction before patching (strip leading whitespace before the `case` match).
6. **PTY navigation bug found and fixed by the orchestrator (Task 3):** `startStoragePTY` sent `dummyKeyShiftRight` (`\x1b[1;2C`, Shift+Right) to switch from the Options to the Storage sub-tab, but the screen's key binding is a plain right arrow (`\x1b[C`) — every test after the browse case timed out waiting for "Storage & preview" to appear. Fixed to the plain-arrow sequence `e2e/create_flow_pty_e2e_test.go` already uses for the same purpose.

## Issues Encountered
This was the most operationally difficult plan in the phase so far, needing 4 dispatch cycles (1 initial + 3 targeted resumes) due to a recurring environment glitch (an intermittent path-corruption bug in the cross-AI runtime unrelated to this plan's own code, previously seen on plans 06-01 and 06-04) that crashed the agent process mid-task several times — never mid-corruption of the actual git working tree, always leaving buildable-or-near-buildable intermediate states the orchestrator could inspect, fix small issues in directly, and resume from. One resume dispatch briefly overlapped with a not-yet-dead prior process on the same worktree; the orchestrator caught this via `ps aux` before any edit collision, killed the stale process, and verified `git status`/`git diff --stat` were unchanged before continuing — no corruption resulted. All fixes described in "Deviations from Plan" above were verified (build, lint, full race test suite, and — for Task 3 — the full `make test-e2e` suite) before being committed.

## Next Phase Readiness
06-06 (CLI verb, `depends_on: ["06-05"]`) has everything it needs: `runSSHStorageMigrate` accepts an empty `planToken` specifically for the CLI's plan-then-commit-in-one-hold pattern, documented at its call site; the single write authority and the digest lock contract are both proven, not just asserted.

## Construction checks (quoted per the plan's own requirement)

`rg -n 'b\.pendingMigration' cmd/gitid/` — matches only inside `putPendingMigration`/`takePendingMigration`'s bodies (`wiring.go:1555,1557,1558,1571,1572,1576,1578`) plus test-file references that assert this very property; no other production call site touches the field.

`rg -n 'txMu' cmd/gitid/` inside `putPendingMigration`/`takePendingMigration` themselves: no match — confirmed by `TestStorageConstructionCheck_Rule5TxMuEnclosesReadsAndNeverCallsPlan` in `wiring_storage_test.go`, which strips comments before asserting.

`rg -n 'txMu' cmd/gitid/wiring.go` shows the acquisition inside `SSHStorageMigrationPlan` (`wiring.go:1626-1627`) enclosing both `b.storage()`'s layout resolution and the `sshconfig.PlanMigration` call.

`rg -n 'SSHStorageMigrationPlan' cmd/gitid/lifecycle.go` — the two hits (`lifecycle.go:1000,1051`) are both inside comments explaining WHY `runSSHStorageMigrate` must never call that method; there is no actual Go call site — `runSSHStorageMigrate` never self-deadlocks by calling the `txMu`-taking method from inside its own `txMu` hold.

---
*Phase: 06-global-ssh-options*
*Completed: 2026-08-27*
