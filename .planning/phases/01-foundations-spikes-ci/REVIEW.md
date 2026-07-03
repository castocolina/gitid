---
phase: 01-foundations-spikes-ci
reviewed: 2026-07-03T07:49:02Z
depth: deep
files_reviewed: 43
files_reviewed_list:
  - .github/workflows/ci.yml
  - Makefile
  - cmd/gitid/debug.go
  - cmd/gitid/debug_test.go
  - cmd/gitid/doctor_test.go
  - cmd/gitid/main.go
  - e2e/debug_e2e_test.go
  - go.mod
  - go.sum
  - internal/doctor/checks/orphans.go
  - internal/doctor/checks/orphans_test.go
  - internal/filewriter/filewriter.go
  - internal/filewriter/filewriter_remove_test.go
  - internal/filewriter/filewriter_test.go
  - internal/identity/inventory.go
  - internal/identity/inventory_test.go
  - internal/identity/state.go
  - internal/identity/state_test.go
  - internal/keygen/catalog.go
  - internal/keygen/catalog_test.go
  - internal/keygen/keygen.go
  - internal/keygen/keygen_test.go
  - internal/keygen/registry.go
  - internal/keygen/registry_test.go
  - internal/platform/capabilities.go
  - internal/platform/capabilities_test.go
  - internal/platform/keytypes.go
  - internal/platform/keytypes_test.go
  - internal/platform/platform.go
  - internal/platform/platform_test.go
  - internal/platform/version.go
  - internal/platform/version_test.go
  - internal/screenshot/determinism.go
  - internal/screenshot/determinism_test.go
  - internal/screenshot/doc.go
  - internal/screenshot/html.go
  - internal/screenshot/html_capture_test.go
  - internal/screenshot/tui.go
  - internal/screenshot/tui_capture_test.go
  - internal/sshconfig/adopt.go
  - internal/sshconfig/adopt_test.go
  - internal/sshconfig/include.go
  - internal/sshconfig/include_test.go
  - internal/sshconfig/migrate.go
  - internal/sshconfig/migrate_test.go
  - tui/health_test.go
  - tui/sidebar_test.go
findings:
  critical: 0
  warning: 5
  info: 2
  total: 7
status: issues_found
---

# Phase 01: Code Review Report — FINAL state (incl. CI-fix and Codex-fix commits)

**Reviewed:** 2026-07-03T07:49:02Z
**Depth:** deep (full `git diff f1c4be8..HEAD -- ':!.planning'` read in full, cross-file, plus the three fix commits `2a642f0`, `4ff0e85`, `b17b399` diffed individually)
**Files Reviewed:** 43 (all non-planning files changed since the pre-Phase-1 baseline)
**Status:** issues_found (no CRITICAL/HIGH survives; see explicit statement below)

## Summary

This review covers the complete Phase 1 diff (7 plans, 01-01..01-07) against baseline
`f1c4be8`, including the two Codex cross-vendor fix commits (`4ff0e85` HIGH #1/HIGH #2,
`b17b399` process-group-kill + `BackupAndRemove` MEDIUM) and the CI-portability fix
commit (`2a642f0`).

**Verification of the three previously-identified Codex findings — all sound:**

- **HIGH #1 (backup destruction)** — `internal/filewriter.backupExistingTarget` now
  names backups `<path>.bak.<UnixNano>` and creates them via `O_CREATE|O_EXCL`
  (`copyFileExclusive`), retrying up to `maxBackupCollisionAttempts` on a same-nanosecond
  collision. `sshconfig.Migrate`'s `rollback`/`restoreSnapshot` now restores from the
  **in-memory** pristine bytes captured at preflight (`backupSnapshot.content`) via the
  new no-backup `filewriter.WriteNoBackup` seam, never re-entering `Write`'s own
  backup-creation step. Verified against `TestMigrateRollbackDoesNotClobberPristineBackup`
  and `TestWriteBackupNamesAreCollisionProof` — both exercise the exact regression and
  pass under `-race`. **Sound.**
- **HIGH #2 (`ssh -G` timeout)** — `RealMigrateDeps.ResolveAlias` runs `ssh -G` under
  `exec.CommandContext` bounded by `migrateResolveTimeout`, in its own process group
  (`SysProcAttr.Setpgid`), SIGKILL'd as a group via `cmd.Cancel`, with `cmd.WaitDelay`
  bounding any residual pipe wait. This correctly addresses the Linux-specific
  grandchild-holds-stdout-pipe failure mode (a forked `Match exec`/`sleep &` child
  surviving the direct child's kill). Verified against
  `TestMigrateReturnsTimeoutErrorWhenSSHHangs`, whose fixture backgrounds `sleep 30 &`
  and `wait`s specifically to reproduce the grandchild-pipe class on every platform.
  **Sound.**
- **MEDIUM (`BackupAndRemove` overwrite-rename)** — now reuses the same collision-proof
  `backupExistingTarget` (copy-then-remove instead of rename-onto-backup), so a
  delete/recreate/delete cycle within the same instant can no longer clobber a still-live
  backup. Verified against `TestBackupAndRemove_ExistingFile`. **Sound.**

All three fixes are correct, test-proven, and consistent with the rest of the codebase's
`filewriter` chokepoint discipline. `go build ./...`, `go vet ./...`, and
`go test ./internal/sshconfig/... ./internal/filewriter/... ./internal/platform/... -race`
all pass locally as part of this review.

**No CRITICAL or HIGH finding survives this review.** The issues below are genuine but
are all either (a) not yet reachable from any `cmd/` entry point in this phase — `Migrate`
and `Adopt` are foundation primitives that Phase 1 deliberately does not wire to a command
yet — or (b) defense-in-depth/consistency gaps rather than demonstrated incorrect behavior
today. They are flagged now, before Phase 3+ wires these primitives into live commands,
because that is the cheapest point to fix them.

Acceptance criteria for 01-01 through 01-07 were spot-checked against the final diff
(regex widening, `BuildProbeDeps`/`BuildInventoryDeps` exported constructors and their
real-wiring tests, the 8-label state vocabulary + 2 overlap rows, the `AdoptDeps`/
`MigrateDeps` naming split, the SHA-pinned/least-privilege/cost-tiered CI workflow, the
`build-cross` target) and all match what the plans require.

## Warnings

### WR-01: `Migrate` reuses a single stale in-memory snapshot for every content-changing write — concurrent external edits are silently discarded (MEDIUM, latent — not yet wired to a command)

**File:** `internal/sshconfig/migrate.go:237-269` (capture), `:256-260` (step-2 write-back), `:282`, `:303` (step-3/4 writes)

**Issue:** `Migrate` reads `sourceContent`/`destContent` exactly once, at the very start
(step 1, lines 237/241), then reuses those same in-memory byte slices for every
subsequent write: the step-2 "backup-trigger" write-back (lines 256/260, which writes
the *unmodified* `sourceContent`/`destContent` straight back to disk purely to make
`filewriter.Write` create a pristine backup) and the step-3/4 composed writes
(`composeDestination(destContent, sourceContent, ...)` / `composeSource(direction,
sourceContent, ...)`) that actually move blocks. There is no re-read and no
"has this file changed since I looked at it?" check anywhere between step 1 and the
final commit.

If the user (or another process — a second `gitid` invocation, the future TUI, or simply
a text editor with autosave) modifies `~/.ssh/config` or the Include'd file *after*
`Migrate`'s step-1 read but *before* the step-2/3/4 writes land, that edit is silently
overwritten with no error, no warning, and — critically — it is not necessarily present
in the pristine on-disk backup either: the step-2 backup is created from the file's
**current on-disk content at that moment** (`filewriter.Write`'s `backupExistingTarget`
reads straight from `targetPath`, not from the `content` parameter passed to `Write`), so
an edit that lands *after* step 2 but before the step-3/4 writes is invisible to both the
backup snapshot and the final write — it is unrecoverable through gitid's own recovery
path.

Concrete failure scenario: user runs `gitid <migrate-command>` (once wired in a later
phase). While the multiple real `ssh -G` round-trips of preflight/validation are running
(each individually bounded by `migrateResolveTimeout`, default 3s, times up to 3 calls
per alias), the user switches to another terminal/editor and adds a new hand-written
`Host` entry to `~/.ssh/config`, saving before `Migrate`'s step-3/4 writes land. The new
entry is silently discarded by the final composed write, with `Migrate` reporting
success. This directly conflicts with CLAUDE.md's "Never write to a user's `~/.ssh/config`
... without a timestamped backup, idempotent managed blocks, and explicit confirmation" —
the backup guarantee is present for gitid's *own* changes but not for a concurrently
introduced foreign edit.

This is not covered by the existing `T-01-22` threat-model entry (01-03-PLAN.md), which
is explicitly scoped to "cross-file migration crash between the two writes," not to
concurrent external modification — this is a genuinely new gap, not a regression of an
already-mitigated threat.

**Mitigating factor:** `sshconfig.Migrate` has no caller anywhere in `cmd/` yet (confirmed
via `grep -rl sshconfig.Migrate cmd/ internal/` — only `migrate_test.go` calls it), so
there is no user-reachable path to this bug today. It is flagged now because it is
foundational, TDD-locked code that a later phase will wire directly into a user-facing
command.

**Fix:** Before each content-changing write, re-read the file's current on-disk bytes and
compare them against the previously-captured snapshot (or against what the immediately
prior write left in place); if they differ, abort the migration with a clear
"~/.ssh/config was modified concurrently, aborting" error rather than silently
overwriting. At minimum, re-read `sourcePath`/`destPath` immediately before the step-2
backup-trigger write (rather than reusing the step-1 read), and re-validate immediately
before the step-3/4 composed writes so the composition itself is built from fresh bytes,
not a decision made once at the top of the function.

### WR-02: The process-group-kill hardening (Codex HIGH #2) was applied only to `migrate.go`'s `ssh -G` call, not to the other 4 `exec.CommandContext` probes (LOW-MEDIUM, consistency/defense-in-depth)

**File:** `internal/platform/version.go:52` (`ssh -V`), `internal/platform/platform.go:61`
(`ssh -Q key`), `internal/platform/capabilities.go:190` (`ssh-add -l`),
`internal/platform/capabilities.go:221` (`ssh -Q key`)

**Issue:** The root cause behind Codex HIGH #2 (documented in commit `b17b399`) is that
plain `exec.CommandContext(...).Output()` only kills the *direct* child on context
cancellation; on Linux, `/bin/sh` forks (rather than exec's) a child such as a `Match
exec` command or a backgrounded `sleep`, and that grandchild can keep holding the
command's stdout pipe open after the direct child is killed, blocking `.Output()`/`.Run()`
past the context deadline. `migrate.go`'s `ssh -G -F <configPath> <alias>` call was fixed
with `SysProcAttr.Setpgid` + a `cmd.Cancel` that SIGKILLs the whole process group + a
`cmd.WaitDelay` belt-and-suspenders bound.

None of the other four `ssh`/`ssh-add` subprocess calls in `internal/platform` received
the same treatment: `ProbeSSHVersion` (`ssh -V`), `ProbeKeyTypes` (`ssh -Q key`),
`probeAgent` (`ssh-add -l`), and `probeFIDO` (`ssh -Q key`) all still rely on the plain
`exec.CommandContext(...).Output()/.Run()/.CombinedOutput()` shape. Today none of these
four pass a `-F <user-controlled-config>` flag, so they do not read a user's `~/.ssh/config`
and are not exposed to the specific `Match exec`-forking-grandchild class that caused the
migrate.go hang (`ssh -V`/`-Q key` are standalone query modes that never consult a config
file; `ssh-add -l` talks to the agent over a socket, not a forked shell). `TestProbeTimeout`
in `capabilities_test.go` confirms `probeAgent` returns promptly today, but its fixture is
a direct `sleep 30` with no forked grandchild — it would not catch a future regression of
this class.

The asymmetry is real but currently benign; the risk is that it is undocumented, so a
future contributor who, e.g., adds a `-F <path>` variant to any of these four probes (a
plausible future need — per-identity `ssh -Q key -F` resolution, for instance) would
silently reintroduce exactly the hang class that took 3 CI iterations to root-cause here,
with no test or comment warning them.

**Fix:** Either apply the same `SysProcAttr.Setpgid` + `cmd.Cancel` group-kill +
`cmd.WaitDelay` pattern uniformly to every `ssh`/`ssh-add` subprocess call in this
codebase (cheapest long-term fix — one shared helper, e.g. `platform.runSSHBounded(ctx,
args...)`, used by all 5 call sites plus `migrate.go`), or add an explicit comment at each
of the 4 un-hardened sites stating why it is safe today (no `-F <user-config>` argument)
and that the group-kill treatment MUST be added the moment that changes.

### WR-03: `migrateResolveTimeout`/`probeTimeout` are unsynchronized package-level mutable `var`s, monkey-patched by tests (LOW, latent race)

**File:** `internal/sshconfig/migrate.go:138`, `internal/platform/version.go:17`

**Issue:** Both timeouts are deliberately `var`, not `const`, specifically so tests can
shrink them (`migrate_test.go:448-450`, `capabilities_test.go:148-150`). Today this is
safe because no test in either package calls `t.Parallel()` (confirmed via grep), so all
tests in a package run sequentially and the mutate-then-restore pattern
(`oldTimeout := X; X = short; t.Cleanup(func(){ X = oldTimeout })`) never overlaps another
goroutine reading the same var. However, this is fragile: it is a plain, unguarded
package-global write from a test goroutine, and the moment any test in `internal/sshconfig`
or `internal/platform` adds `t.Parallel()` (a very ordinary thing to do to speed up a test
suite), `go test -race` will start flagging a real data race between the timeout mutation
and a concurrently-running probe/resolve call reading it.

**Fix:** Prefer passing the timeout as an explicit field on `MigrateDeps`/`Deps` (already
injectable structs) rather than a shared package var, or add a `//nolint`-adjacent comment
at the var declaration stating "tests in this package MUST NOT use t.Parallel()" so the
constraint is enforced by convention until it can be threaded through Deps.

### WR-04: `adopt.go`'s sentinel-bearing candidate is read before its symlink check runs (LOW, TOCTOU ordering)

**File:** `internal/sshconfig/adopt.go:270-292` (`candidateTarget`, `AdoptSentinelBearing` branch)

**Issue:** For the `AdoptSentinelBearing` method, `candidateTarget` calls
`deps.ReadFile(target)` (line 275, to check for a gitid sentinel block) **before** the
symlink guard (`deps.Lstat(target)`, lines 286-292) runs. The doc comment on the symlink
guard states it "mirrors the sibling gitconfig-fragment adopter's symlink guard" — the
intent is clearly to reject a symlinked target outright, but as written a symlinked
candidate is still opened and read into memory (to search for a sentinel) before being
rejected.

Impact is low: gitid is a single-user CLI operating on files the invoking user already
has read access to (there is no privilege boundary being crossed — the "attacker" would
need write access to the user's own `~/.ssh/config` to plant a malicious `Include` line
pointing at a symlink, at which point they already have far more direct attacks
available), and the read result is discarded immediately (never surfaced to the caller)
when the sentinel check or the subsequent Lstat check fails. `Adopt` also has no caller in
`cmd/` yet, so this is not user-reachable today either.

**Fix:** Move the `Lstat`/symlink check to run before `ReadFile`, so a symlinked candidate
is rejected without ever being opened — cheap to fix now, before this code is wired to a
live adoption flow, and it removes the discrepancy between the code's actual order and the
"mirrors the ... symlink guard" doc comment's implied intent.

### WR-05: `TestDoctorFixYesHealsMissingBaselineFromScratch` assertion was weakened from `code == 0` to `code < 2` (informational — deliberate, justified, but worth flagging as a coverage reduction)

**File:** `cmd/gitid/doctor_test.go:396-410`

**Issue:** The CI-portability fix commit (`2a642f0`) changed this test's assertion from
requiring a full `exit 0` heal to accepting any exit code below the "error" tier (`< 2`),
because headless CI runners legitimately have no `ssh-agent` (a warning gitid's `--fix`
cannot heal) and no clipboard tool (an info-level finding). The commit message explains
this clearly and the change is reasonable — but it is worth recording explicitly that this
test's coverage is now strictly weaker than before: it no longer proves `--fix --yes`
converges to a fully clean doctor run on a fully-provisioned host, only that baseline
*errors* are healed. This is a deliberate, documented tradeoff, not a defect, but a
regression test with a loosened bound is worth a second pair of eyes confirming the
justification holds (it does, based on the commit message and the surrounding assertions
that still check the concrete baseline artifacts were created).

## Info

### IN-01: `Migrate`'s step-3 `validateResolution` call cannot discriminate a real failure (LOW, dead-weight check)

**File:** `internal/sshconfig/migrate.go:286-288`

**Issue:** After writing the destination file (step 3), `Migrate` calls
`validateResolution(deps, preSnapshot)`, which re-resolves every alias via
`deps.ResolveAlias(deps.ConfigPath, alias)` — always against `~/.ssh/config`. At the
step-3 point in the transaction, `deps.ConfigPath`'s on-disk bytes are still byte-identical
to the preflight snapshot (only the *destination* file has been written so far; the
*source*, which for `MigrateToInclude` is `~/.ssh/config` itself, is not touched until
step 4). Because the file being resolved hasn't changed, this call can, in practice, never
detect a real regression — it is a real `ssh -G` subprocess round-trip per alias that adds
latency and an extra failure-injection point (`StepDestinationWritten`) without adding
discriminating power. The genuine behavior-preservation proof only happens at step 4's
`validateResolution` call, after the source has actually been trimmed and the Include line
floored.

**Fix:** Not a correctness bug — no incorrect result flows from this — but consider either
documenting explicitly in the step-3 comment that this call is a structural sanity check
only (not a resolution-equivalence proof), or dropping it in favor of a cheaper "does the
destination file still parse" check, reserving the real `ssh -G` proof for step 4 where it
actually means something.

### IN-02: `MigrateToInFile` leaves the `Include ~/.ssh/config.d/*.config` line in place after moving all blocks back in-line (LOW, cosmetic incomplete cleanup)

**File:** `internal/sshconfig/migrate.go:452-460` (`composeSource`)

**Issue:** `composeSource` only floors/adds the Include line for the `MigrateToInclude`
direction; it never removes the Include line from `~/.ssh/config` for the reverse
`MigrateToInFile` direction. After a full reverse-migration (all managed blocks moved back
in-line), `~/.ssh/config` still carries `Include ~/.ssh/config.d/*.config`, now pointing at
a directory that may be empty. This is harmless (an Include of an empty/non-matching glob
is a no-op for `ssh -G`) and is not required by any 01-03 acceptance criterion, but it is
an asymmetry worth a one-line doc comment: `MigrateToInclude` is a symmetric two-file
transaction (add the Include line + move blocks), while `MigrateToInFile` is not (move
blocks only, Include line is left dangling) — a future "un-adopt"/cleanup command will need
to know this line is not self-cleaning.

---

_Reviewed: 2026-07-03T07:49:02Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
