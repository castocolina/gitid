---
phase: 05-identity-manager
fixed_at: 2026-08-26T14:30:00Z
review_path: .planning/phases/05-identity-manager/05-REVIEW.md
iteration: 1
findings_in_scope: 18
fixed: 18
skipped: 0
status: all_fixed
---

# Phase 5: Code Review Fix Report

**Fixed at:** 2026-08-26
**Source review:** `.planning/phases/05-identity-manager/05-REVIEW.md`
**Scope:** Critical (5) + Warning (13) — all findings in the report; no Info
findings existed.

**Summary:**
- Findings in scope: 18 (CR-01..05, WR-01..13)
- Fixed: 18
- Skipped: 0

**Method.** Each finding was fixed test-first: a failing regression test was
written to prove the bug (RED), the fix was applied, and the test was
re-run to confirm it passes (GREEN). For most findings the RED state was a
genuine failing assertion against the pre-fix behavior; for four findings
(WR-10, WR-11, and partially WR-13) the RED state was a compile failure
against a not-yet-existing symbol (`--force-ssh` flag, `unclassifiedIdentityRecord`,
`orDefault`) rather than a runtime assertion failure — those cases are
called out explicitly below along with an "investigation note" documenting
that no live behavioral divergence could be reproduced in the current
codebase for that specific finding, even though the code change matches the
review's own suggested fix and closes the documented gap defensively. Each
fix was committed atomically (`fix(05): <what> (<finding-id>)`), one finding
per commit, never batched.

## Fixed Issues

### CR-01 + CR-02: shared-key gate missing/un-normalized at rotate, repair, and KeyActionFor

**Files modified:** `cmd/gitid/lifecycle.go`, `cmd/gitid/wiring.go`, `cmd/gitid/lifecycle_test.go`, `cmd/gitid/wiring_test.go`
**Commit:** `91b139e`
**Applied fix:** Added the `identity.SharedKeyOwners` gate directly inside
`runRotate` (the one chokepoint both CLI and TUI call through), refusing
with a wrapped `identity.ErrRepairTargetShared` when the current key is
shared, and refusing separately when the key is missing (`no current key
pair to retire`). Fixed the two remaining un-normalized comparisons
wave 9 left behind: `runRepair`'s `otherOwners` computation now uses
`b.normalizedAccounts()` instead of raw `b.accounts()`, and `KeyActionFor`
now normalizes both the account and the comparison list before computing
`ownerCount`.
**Tests:** `TestRunRotateRefusesWhenKeyIsSharedWithAnotherIdentity`,
`TestRunRotateRefusesWhenKeyIsMissing`,
`TestRunRepairRefusesWhenSharedKeyComparisonIsNormalized`,
`TestKeyActionForDetectsSharedKeyAcrossMixedPathSpelling` — all RED before,
GREEN after. New `seedSharedKeyFixture` test helper (tilde and
mixed-spelling variants).

### CR-03: TUI key ceremony fabricated stage-1/stage-2 test results

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `16513cc`
**Applied fix:** Replaced the hardcoded `TestOutcomePass, "Stage N test
passed."` with an honest not-tested disclosure
(`keyCeremonyNotTestedDetail`), rendered through a new `renderStageNotTested`
helper that suppresses the misleading "Reachable" warning banner too. Left
`m.keyCeremonyResult`'s pre-existing seeding as-is and documented why
(an existing approved test, `TestKeyCeremonyRotateRendersGraceAndArchive`,
encodes the resulting grace-window hint as intended generic post-rotate
advisory, not itself a fabricated test result) — flagged inline for
follow-up alongside adding a real pre-write probe seam.
**Tests:** `TestKeyCeremonyStagesDoNotFabricateAPassResult` — RED before,
GREEN after.

### CR-04: `identity create` never validated the identity name (path traversal)

**Files modified:** `cmd/gitid/identity_create.go`, `cmd/gitid/identity_test.go`
**Commit:** `7783ad3`
**Applied fix:** Added `identity.ValidateName` / `ValidateEmail` /
`ValidateProvider` calls at the top of `createInputFromCreateFlags`, before
any path derivation — closing the `--name '../.bashrc'` traversal
(`ValidateName`'s charset rejects `/` outright) and the deep-write-then-roll
back email validation gap.
**Tests:** `TestIdentityCreateRejectsPathTraversalName`,
`TestIdentityCreateRejectsInvalidGitEmail` — RED before (proved via a real
rollback with 12 restored paths pre-fix), GREEN after.

### CR-05: `identity delete` swallowed a failed `DeletePlan`

**Files modified:** `cmd/gitid/identity_delete.go`, `cmd/gitid/identity_test.go`
**Commit:** `687bab4`
**Applied fix:** `runIdentityDelete` now fails closed with an explicit
"refusing to delete ... the delete plan could not be built" error when
`b.DeletePlan` errors, instead of silently falling through to the delete.
**Tests:** `TestIdentityDeleteRefusesWhenPlanFails` — RED before, GREEN
after.

### WR-01: CLI everything-delete accepted generic "yes"

**Files modified:** `cmd/gitid/identity_delete.go`, `cmd/gitid/identity_test.go`
**Commit:** `24bab53`
**Applied fix:** `confirmDelete` now requires the typed identity name for
`DeleteScopeEverything` (matching the TUI's `FixDestructive{ConfirmWord:
plan.Name}`), keeping "yes" for git-only.
**Tests:** `TestConfirmDeleteRequiresTypedNameForEverythingScope` — RED
before, GREEN after.

### WR-02: everything-delete receipt claimed "key removed" despite shared-key downgrade

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `fb1d8e7`
**Applied fix:** `deleteCeremonyFor`'s `ResultMessage` now branches on
`plan.SharedKeyOwners`, saying "key kept — still used by ..." instead of
claiming removal.
**Tests:** `TestDeleteEverythingReceiptReflectsSharedKeyDowngrade` — RED
before, GREEN after.

### WR-03: `deletePlanPreview` was a second, contradicting delete preview

**Files modified:** `cmd/gitid/lifecycle.go`, `cmd/gitid/lifecycle_test.go`
**Commit:** `d4b958b`
**Applied fix:** `runDelete`'s plan stage now calls the real
`b.DeletePlan(name, scope)` (failing closed on error) and renders the
confirmation preview from that plan's `Targets` — which already respects
`keySurvives` — instead of a hand-rolled list that unconditionally named
the key pair.
**Tests:** `TestDeletePlanPreviewOmitsSharedKeyNeverToBeRemoved` — RED
before, GREEN after.

### WR-04: rotate/repair could rewrite `~/.ssh/config` unwatched

**Files modified:** `cmd/gitid/lifecycle.go`, `cmd/gitid/lifecycle_test.go`
**Commit:** `405e831`
**Applied fix:** Added `b.sshConfigPath` to both `rotateWatchPaths` and
`repairWatchPaths`, and record `b.includeDir` as a created directory
(mirroring the existing archive-directory pattern) when the include layout
needs its first Include line.
**Tests:** `TestRotateAndRepairWatchPathsIncludeSSHConfigPath`, built over a
hand-seeded include-dir-layout fixture (via the real
`EnsureIncludeDir`/`EnsureIncludeLine` seams) so `storageTargetPath()` and
`sshConfigPath` are genuinely distinct — RED before, GREEN after.

### WR-05: `keyOwners()` never labeled tilde-path keys

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `ed297b3`
**Applied fix:** `keyOwners()` now iterates `b.normalizedAccounts()` instead
of raw `b.accounts()`.
**Tests:** `TestScanReusableKeysLabelsTildeSpelledIdentityKey` — RED before,
GREEN after.

### WR-06: headless create/clone always forced the provider URL rewrite

**Files modified:** `cmd/gitid/identity_create.go`, `cmd/gitid/identity_clone.go`, `cmd/gitid/identity_test.go`, `docs/cli-parity-matrix.md`
**Commit:** `408b8fa`
**Applied fix:** Added `--force-ssh` to `identity create` (default false);
`identity clone` mirrors the clone source's own current `ForceSSH` setting
instead of forcing it on. Updated the CLI parity matrix rows.
**Tests:** `TestCreateInputFromCreateFlagsForceSSHDefaultsOffAndFlagEnables`,
`TestCloneCeremonyInputsMirrorsSourceForceSSH` — RED before (compile
failure until the flag existed), GREEN after.

### WR-07: `refreshDeletePlan` silently dropped the second plan's error

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `c1139e1`
**Applied fix:** Added a dedicated `deleteChoiceOwnersErr` field (kept
separate from `deletePlanErr` to avoid conflating the primary-plan and
everything-scope-probe failure modes) and render it inline on the
scope-choice screen.
**Tests:** `TestDeleteChoiceDisclosesSecondPlanFailure` — RED before
(compile failure until the field existed), GREEN after.

### WR-08: `AllowedSignersLine`'s gate only rejected a comma

**Files modified:** `internal/keygen/signers.go`, `internal/keygen/signers_test.go`
**Commit:** `fd55004`
**Applied fix:** The gate now rejects comma, newline, CR, space, and tab,
and requires the value to contain `@` — self-sufficient, independent of any
upstream validator.
**Tests:** `TestAllowedSignersLine_RejectsNewlineLineInjection`,
`TestAllowedSignersLine_RejectsSpaceFieldInjection`,
`TestAllowedSignersLine_RequiresBareAddress` — RED before, GREEN after.

### WR-09: `gate-copy-freeze` did not actually protect `rotateDryRunCaveat`

**Files modified:** `Makefile`
**Commit:** `f693fe8`
**Applied fix:** Extended the gate's grep roots to include `cmd/gitid` and
registered the caveat's byte-exact text in the frozen-copy list.
**Verification:** `grep -c "This dry run tests only" Makefile` was 0 before
this change; `make gate-copy-freeze` now passes with the new entry present
(not a `go test`-driven RED/GREEN, since the mechanism itself is a Makefile
gate — verified via the gate's own pass/fail behavior instead).

### WR-10: headless clone GitDir diverged from the wizard's derivation

**Files modified:** `cmd/gitid/identity_clone.go`, `cmd/gitid/identity_test.go`
**Commit:** `6a88770`
**Applied fix:** `GitDir` now derives via
`orDefault(gitDirFromMatches(in.Matches), "~/git/"+in.Name+"/")`, matching
`ClonePrefill`'s own derivation, per the review's exact suggested code.
**Tests:** `TestCloneCeremonyInputsGitDirMatchesClonePrefillDerivation`.
**Investigation note:** `identity.DeriveCloneInput`'s Matches rebuild is
kind-only (review R-12) — a gitdir-kind source always yields the default
`~/git/<cloneName>/` match regardless of the source's actual gitdir value —
so on every input constructible today, the old hardcoded literal and the
new derivation already coincide byte-for-byte. This test passes both before
and after the change; it is a defensive consolidation against future drift
between the two call sites, not a fix for an observed live divergence.

### WR-11: dead `PublicKeyPath` assignment in `cloneCeremonyInputs`

**Files modified:** `cmd/gitid/identity_clone.go`, `cmd/gitid/identity_test.go`
**Commit:** `9939e80`
**Applied fix:** Removed the dead struct-literal assignment, keeping only
the explicit if/else reassignment as the single source of truth.
**Tests:** `TestCloneCeremonyInputsPublicKeyPathNeverBarePubSuffix` guards
both branches directly. Dead-code removal — passes identically before and
after, since the reassignment already ran unconditionally; guards against
future reordering regressing to the bare `.pub` value.

### WR-12: `RemoveProviderRewrite` wrote (and minted a backup) even when nothing changed

**Files modified:** `internal/gitconfig/renderer.go`, `internal/gitconfig/renderer_test.go`
**Commit:** `bf15f3e`
**Applied fix:** Added the same `bytes.Equal(composed, existing)` guard the
two equality-guarded removals in `identity.Delete` already use — a true
no-op now performs no write and returns an empty backup path.
**Tests:** Strengthened `TestRemoveProviderRewrite`'s idempotent-second-call
assertions and `TestRemoveProviderRewrite_NoSuchBlockIsNilError` to check
the backup path is empty AND that no `.bak.<nanos>` file is minted on disk
(new `countBackupFiles` helper) — RED before, GREEN after.

### WR-13: `identity show` could not resolve an identity the inventory omits; dead local in `toIdentityRecord`

**Files modified:** `cmd/gitid/identity_read.go`, `cmd/gitid/identity_test.go`
**Commit:** `6969b14`
**Applied fix:** `buildIdentityRecords` now builds its record set from the
UNION of `inv.Identities` and `b.accounts()`, keyed by name; an account with
no matching health entry gets a new `unclassifiedIdentityRecord` fallback
that carries every Account-shaped fact but never fabricates a
classification. Also fixed the adjacent dead local: `matchStrategyFor` now
returns only `strategy`, not the unused `gitDir`.
**Tests:** `TestUnclassifiedIdentityRecordNeverFabricatesState` (direct unit
proof, RED before — compile failure until the function existed — GREEN
after), `TestBuildIdentityRecords_UnionNeverDoubleCountsAClassifiedIdentity`
(non-regression proof).
**Investigation note:** `identity.BuildInventory`'s current implementation
appends exactly one `IdentityHealth` per reconstructed account
unconditionally (no skip/error branch exists today), so no fixture could be
constructed where `inv.Identities` and `b.accounts()` actually diverge in
the present codebase. This closes the documented gap defensively, matching
the review's own suggested "union, keyed by name" fix.

## Skipped Issues

None — all 18 in-scope findings were fixed.

## Final Verification

Run in the isolated worktree (`gsd-reviewfix/05-33032`, branched from
`gsd/phase-05-identity-manager` at `91b32bb`):

| Gate | Result |
|---|---|
| `go build ./...` | PASS |
| `make lint` (golangci-lint + gosec + go vet, all build tags) | PASS, 0 issues |
| `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` | PASS (18 packages, 1 no-test-files) |
| `make test-e2e` (real-PTY suite) | PASS |

---

_Fixed: 2026-08-26_
_Fixer: Claude (gsd-code-fixer)_
