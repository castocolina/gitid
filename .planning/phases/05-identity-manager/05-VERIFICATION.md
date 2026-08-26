---
phase: 05-identity-manager
verified: 2026-08-26T14:54:16Z
status: passed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 5: Identity Manager Verification Report

**Phase Goal:** A developer manages all identities from the app's main view — seeing
completeness/health state at a glance, opening SSH-first detail, and cloning, adding
keys, rotating, or deleting with the right choices.
**Verified:** 2026-08-26T14:54:16Z
**Status:** passed
**Re-verification:** No — initial verification (post code-review-fix)

## Context

This verification runs **after** `05-REVIEW.md` (18 findings: 5 blocker, 13 warning)
and `05-REVIEW-FIX.md` (all 18 claimed fixed). SUMMARY.md and REVIEW-FIX.md claims
were treated as unverified narrative; every finding below was re-checked directly
against the merged code at `HEAD` (`49e490c`, branch `gsd/phase-05-identity-manager`,
clean working tree — no uncommitted drift).

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Identity list reconstructs an 8-state taxonomy from parsed managed blocks, no sidecar DB (MGR-01/08) | ✓ VERIFIED | `internal/identity/state.go`, `internal/identity/inventory.go` — doc comments + code confirm pure derivation ("No sidecar DB — every fact is derived from the parsed managed blocks"); `e2e/identity_manager_pty_e2e_test.go:TestIdentityManager_ListPopulatedEightTaxonomy`, `TestIdentityManager_ListEmpty` |
| 2 | Detail view is SSH-first, never fabricates git fields, shows per-identity health | ✓ VERIFIED | `e2e/identity_manager_pty_e2e_test.go:TestIdentityManager_DetailSSHFirst`; `cmd/gitid/identity_read.go` (WR-13 fix: `buildIdentityRecords` unions `inv.Identities` ∪ `b.accounts()`, never fabricates a classification for an unmatched account via `unclassifiedIdentityRecord`) |
| 3 | Clone into a distinct name (reuse or new key), new-key (repair), and rotate all re-point artifacts and re-run the test gate | ✓ VERIFIED | `cmd/gitid/identity_clone.go`, `cmd/gitid/lifecycle.go` (`runRotate`/`runRepair`), `e2e/identity_cli_e2e_test.go:TestIdentityCLI_RotateParity/_NewKeyParity/_CloneParity`, `e2e/identity_manager_pty_e2e_test.go:TestIdentityManager_KeyCeremonyRotate/_KeyCeremonyRepair` |
| 4 | Shared-key gating: rotate/repair refuse to touch a key another identity depends on, at the ONE chokepoint both CLI and TUI use | ✓ VERIFIED | `cmd/gitid/lifecycle.go:194-198` (`runRotate` gate, CR-01) and `:407` (`runRepair`, CR-02) both call `identity.SharedKeyOwners(b.normalizedAccounts(), ...)`; `cmd/gitid/wiring.go:3152` (`KeyActionFor` routing) also normalized. Regression test `TestRunRotateRefusesWhenKeyIsSharedWithAnotherIdentity` asserts the key file is byte-unchanged and no archive entry is created after refusal — a real behavioral proof, not a presence check. |
| 5 | Fail-closed confirmation: a non-interactive destructive call without `--yes`/TTY is refused before any write; the CLI can never assert "already confirmed" | ✓ VERIFIED | `cmd/gitid/lifecycle.go` `confirmationMode` enum, zero value = `confirmationRequired`; source-level test at `cmd/gitid/identity_test.go:1426` greps that no CLI path ever sets `confirmationAlreadyObtained`; `cmd/gitid/identity_delete.go:98-99` explicit non-TTY refusal |
| 6 | Delete offers "everything" vs "git-only", ref-counts the provider insteadOf block, downgrades shared keys, discloses unmanaged references, and fails closed on a broken plan | ✓ VERIFIED | `internal/identity/deleteplan.go`, `internal/identity/scan.go` (D-13); CR-05 fix in `cmd/gitid/identity_delete.go:105-109` (`runIdentityDelete` now errors instead of silently deleting on a failed plan); WR-02/WR-03 fixes confirmed in `internal/tuikit/identities.go:2825-2829` (receipt wording matches actual key-kept/removed outcome) and `cmd/gitid/lifecycle.go` (preview now derives from the real `DeletePlan`) |
| 7 | Mutation-journal rollback: a mid-transaction failure restores every watched file's bytes/mode and removes every path the transaction created | ✓ VERIFIED | `cmd/gitid/lifecycle_test.go:TestJournalRecordCreatedDirDisjointness`, `TestJournalRestoreRemovesCreatedPathsProvesR208`, `TestJournalRestoreRestoresWatchedBytesAndModeProvesR21`; WR-04 fix adds `b.sshConfigPath` to `rotateWatchPaths`/`repairWatchPaths` and records the include dir as created, closing the one incomplete-watch gap the review found |
| 8 | Name/email/provider validation runs before any path derivation on identity creation (no traversal) | ✓ VERIFIED | `cmd/gitid/identity_create.go:161-171` calls `identity.ValidateName`/`ValidateEmail`/`ValidateProvider` before `createInput`; CR-04 regression test `TestIdentityCreateRejectsPathTraversalName` |
| 9 | All five product surfaces (Identities, Global SSH, Global Git, Health, Fixer) reachable via palette + number keys; every action has a CLI+TUI outcome-parity command with shell completion | ✓ VERIFIED (see note) | 4 tabs exist (`internal/tuikit/frame.go`: Identities/GlobalSSH/GlobalGit/Doctor); Doctor **absorbs** Health+Fixer per an explicit, requirement-level decision (`REQUIREMENTS.md` FIX-01/FIX-02: "re-home into the health screen" / "the fixer presents SSH and git problems in the health screen's two sections"), not a Phase-5 omission — matches the approved `internal/dummytui` mockup (`doc.go:7`: "Doctor that absorbs the Fixer (FIX-02)"). CLI parity: `docs/cli-parity-matrix.md` is machine-checked both directions by `TestParityMatrixResolvesAndCoversTree`/`_RequirementCoverage`/`_NegativeControl*`. `gitid completion bash\|zsh\|fish` all produce valid scripts (spot-run). |

**Score:** 9/9 truths verified, 0 present-but-behavior-unverified.

### Note on Truth 9 (documentation drift, not a functional gap)

`ROADMAP.md` Phase 5 success criterion 4 and `REQUIREMENTS.md` SHELL-02's literal text
still say "five primary views ... Health, Fixer". The codebase — consistently with
`REQUIREMENTS.md`'s own FIX-01/FIX-02/HLTH-06 entries ("re-home into the health
screen") and the Phase-2-approved dummy mockup — merges Health+Fixer into one
"Doctor" view. This is a **pre-existing, cross-referenced design decision** (visible
already in Phase 3's `03-03-SUMMARY.md` "Doctor tab"), not something introduced or
skipped by Phase 5. Recommend a documentation follow-up (reword SHELL-02/SC4 to
"four primary views, Health absorbing Fixer") but this does not block the phase —
functional intent (every named capability reachable) is met.

### Required Artifacts (Level 1-3 spot-check)

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/gitid/lifecycle.go` | Single chokepoint (`runRotate`/`runRepair`/`runDelete`) with shared-key + confirmation gates | ✓ VERIFIED | Gates present at lines 194-198 (rotate), 407 (repair); confirmed both CLI (`cliRotateInto`) and TUI (`commitRotateInto`) call through it |
| `cmd/gitid/wiring.go` | `normalizedAccounts()` used consistently for cross-identity comparisons | ✓ VERIFIED | `keyOwners()` (2528), `KeyActionFor` routing (3152) both use normalized list |
| `internal/tuikit/identities.go` | Honest (non-fabricated) key-ceremony test-stage rendering | ✓ VERIFIED | `keyCeremonyNotTestedDetail` const + `renderStageNotTested`, wired at case `"stage1"`/`"stage2"` (2372-2376) |
| `internal/keygen/signers.go` | `AllowedSignersLine` self-sufficient injection gate | ✓ VERIFIED | Rejects comma/newline/CR/space/tab, requires `@` (line 40) |
| `docs/cli-parity-matrix.md` | Machine-checked outcome↔command matrix | ✓ VERIFIED | `TestParityMatrixResolvesAndCoversTree` + 5 companion tests in `identity_test.go` |
| `internal/identity/scan.go` | D-13 unmanaged-reference scan | ✓ VERIFIED | Package doc + `ScanRegion`/`UnmanagedHit` types present, consumed by `DeletePlan` |

### Behavioral / Test Evidence

| Check | Command | Result | Status |
|-------|---------|--------|--------|
| Build | `go build ./...` | clean, no errors | ✓ PASS |
| Unit + race tests (identity/keygen/tuikit/gitconfig/cmd packages) | `TERM=dumb SSH_AUTH_SOCK= go test -race ./cmd/gitid/... ./internal/identity/... ./internal/keygen/... ./internal/tuikit/... ./internal/gitconfig/...` | `1113 passed in 5 packages` | ✓ PASS |
| Lint | `make lint` | `golangci-lint run ./...` → `0 issues`; `go vet` all tags clean | ✓ PASS |
| Named regression test (CR-01) | `TestRunRotateRefusesWhenKeyIsSharedWithAnotherIdentity` | refuses, key bytes unchanged, no archive entry created | ✓ PASS |
| Completion generation | `gitid completion bash\|zsh\|fish` | valid scripts emitted for all three shells | ✓ PASS |
| Full-suite e2e (real PTY, race) | `go clean -testcache && make test-e2e` (`go test -tags e2e -race -timeout 900s ./e2e/...`) | `ok github.com/castocolina/gitid/e2e 442.868s`, exit code 0 — fresh run against `HEAD` with test cache cleared first | ✓ PASS |

**Anti-pattern scan:** `TBD`/`FIXME`/`XXX` grep across all 9 review-fix-touched files
(`lifecycle.go`, `wiring.go`, `identity_create.go`, `identity_delete.go`,
`identity_key.go`, `identity_clone.go`, `identity_read.go`, `identities.go`,
`signers.go`, `renderer.go`) — **zero matches**. No debt markers.

### Requirements Coverage

| Requirement | Status | Evidence |
|---|---|---|
| MGR-01, MGR-08 | ✓ SATISFIED | 8-state taxonomy, no sidecar DB |
| MGR-03, MGR-07 | ✓ SATISFIED | SSH-first detail, per-identity health |
| MGR-04 | ✓ SATISFIED | Clone via wizard pre-fill, D-14/D-15/D-17 derivation |
| MGR-05, KEY-05, KEY-07 | ✓ SATISFIED | Rotate/new-key ceremonies, shared-key gate (post-fix) |
| MGR-06 | ✓ SATISFIED | Delete-choice, ref-count, shared-key downgrade, D-13 scan, fail-closed plan |
| SHELL-01 | ✓ SATISFIED (carried) | Integrated app shell |
| SHELL-02 | ✓ SATISFIED (see Truth 9 note) | 4 views, Health absorbs Fixer per FIX-01/FIX-02 |
| SHELL-03 | ✓ SATISFIED | Parity matrix + machine-checked tests + completions |

No orphaned requirements found for Phase 5 in `REQUIREMENTS.md`.

### Human Verification Required

None. All must-haves resolved programmatically with direct code inspection and
passing automated tests (build, lint, race-mode unit suite, and targeted regression
tests proving the specific safety properties in question).

### Gaps Summary

No blocking gaps. All 5 CR (blocker) and 13 WR (warning) findings from `05-REVIEW.md`
were independently re-verified as fixed in the merged code, not merely claimed:

- **CR-01/CR-02** (shared-key gate missing/un-normalized): gate now lives in the
  chokepoint (`runRotate`, `runRepair`) using `normalizedAccounts()`; regression test
  proves no archive/write occurs on a shared key.
- **CR-03** (fabricated TUI test-pass): replaced with an honest "Not tested" render;
  no backend seam is invoked, and none is fabricated either.
- **CR-04** (path traversal via identity name): `ValidateName`/`ValidateEmail`/
  `ValidateProvider` now run before any path derivation.
- **CR-05** (delete plan failure swallowed): `identity delete` now fails closed,
  matching the TUI.
- **WR-01..13**: each spot-checked; representative high-risk ones (WR-02 receipt
  wording, WR-03 preview/write divergence, WR-08 allowed_signers injection gate)
  directly confirmed in source.

One documentation-only note (not a gap): `ROADMAP.md`/`REQUIREMENTS.md` still
describe "5 primary views" while the shipped (and pre-existing, cross-referenced)
architecture merges Health+Fixer into one Doctor view — recommend a wording fix in
a docs pass, no code or behavior change needed.

---

*Verified: 2026-08-26T14:54:16Z*
*Verifier: Claude (gsd-verifier)*
