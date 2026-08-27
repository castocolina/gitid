---
phase: 06-global-ssh-options
verified: 2026-08-27T11:02:59Z
status: passed
score: 10/10 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 6: Global SSH Options Verification Report

**Phase Goal:** A developer reviews and safely fixes global SSH options that are dangerous when unset/misconfigured, with every option explained.
**Verified:** 2026-08-27T11:02:59Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 (SC1) | A global-SSH-options screen surfaces the 6 dangerous-by-default options and explains each option's risk and recommended value. | ✓ VERIFIED | `internal/globalssh/policy.go` `Policy` table has all six D-10 rows (`StrictHostKeyChecking`, `ForwardAgent`, `HashKnownHosts`, `IdentitiesOnly`, `AddKeysToAgent`, `UseKeychain`) each with `Risk`/`Recommended`. `internal/tuikit/design.go`'s `GlobalSSHOptions` fixture and `GlobalSSHDetailExplanation` supply the risk + one-liner + IdentitiesOnly detail copy; `internal/tuikit/globalssh.go:778-779` renders a `[Risk]` chip on every list row. Real captured PTY frame `ui-frames/global-ssh-browse.txt` shows all 6 rows with risk chips, `now: X → Y` lines, and the advisory detail pane text from the **compiled real binary**, not the dummy. |
| 2 (SC2) | Recommendations are advisory and fixable, never blocking; applying writes through the backup + idempotent managed-block chokepoint with confirmation. | ✓ VERIFIED | `cmd/gitid/lifecycle.go:796-974` `runGlobalSSHApply` — plan → simulate → **confirm** (`confirmGate`, l.875) → unconditional backup → write via `filewriter.Write` (timestamped backup + atomic temp→rename) → post-write verify; shadow/verify findings are appended to `res.Advisories` and never block (write already succeeded, l.954-971). Live-executed against a real compiled binary in a sandboxed HOME: `gitid ssh options apply HashKnownHosts` without `--yes` refuses ("refusing to apply ... without --yes"); with `--yes` it writes, and a second identical apply is idempotent (same managed-block bytes) and produces a NEW timestamped backup (`gitid.config.bak.<ts>`). Confirmation in the TUI is the ceremony's `ceremonyConfirmed` outcome (`internal/tuikit/globalssh.go:490-513`) — `CommitGlobalSSH` is dispatched only after that keypress. `EnsureGlobals` (`internal/sshconfig/globals.go:84-113`) is the single idempotent managed-block chokepoint (existing values win, round-trip parse-safety check before returning). |
| 3 (SC3) | UI-wave gate: PTY e2e drives every screen in the real binary; automated review compares it with `cmd/gitid-dummy` and classifies every difference as an improvement or a defect. | ✓ VERIFIED | 17 real-PTY/CLI e2e tests pass against the compiled binary (`go test -tags e2e ./e2e/... -run TestGlobalSSH`, 80.35s, all PASS) covering browse, empty-selection, apply cancel/confirm, shadowed fix, non-shadowing negative control, probe-inconclusive, commit-failure/retry, storage browse/migrate cancel/confirm/round-trip/changed-since-preview, and 3 CLI e2e cases. `internal/screenshot/createflow.go` registers 7 new `RequiredScreenSpecs`; `.planning/design/global-ssh/visual-divergence-allowlist.txt` (96 lines) classifies every real-vs-dummy divergence; `cmd/gitid/gate_visual_regression_test.go` has all `TestGlobalSSH*` acceptance tests AND all 4 required negative controls (`TestNegativeControl_MissingGlobalSSHState`, `..._GlobalSSHUnclassifiedDifferenceRejected`, `..._AllGlobalSSHComparableEqualRegionsAreMutationSensitive`, `..._GlobalSSHCrossRegistryLeakage`) — all ran and PASSED (`go test -tags screenshot ./cmd/gitid/... -run GlobalSSH`, 7.65s). HTML is explicitly recorded non-applicable per `TestGlobalSSHHTMLNonApplicabilityPerSpec`. |
| 4 (06-01) | Exactly ONE write authority (`runGlobalSSHApply`) and exactly ONE `Host *` owner (`sshconfig.EnsureGlobals`); `RenderGlobalBlock` deleted. | ✓ VERIFIED | `rg -n 'RenderGlobalBlock'` across `internal cmd e2e` (excluding test names) returns no production hits. `cmd/gitid/wiring.go:1354` `CommitGlobalSSH` calls only `b.runGlobalSSHApply`. `internal/identity/identity.go` create/rotate/repair route through `EnsureGlobals` via `CreateInput.GlobalsGOOS`. |
| 5 (06-02) | Reserved-name registry (`global-ssh` + legacy) consolidated; migration MOVES the globals block with identities; both-files-globals aborts at preflight. | ✓ VERIFIED | `internal/sshconfig/migrate.go:730` migration classification keys on `IsGlobalBlockName`; `internal/doctor/checks/orphans.go:59` and `overlap.go` route through `sshconfig.IsReservedBlockName`. `TestMigrationClasses`, `TestMigrateBothFilesGlobalsAbortsAtPreflight` pass (part of the full `go test -race ./...` run — all packages green). |
| 6 (06-03) | All six options render real value/provenance in one of four states; IdentitiesOnly is verify-only; `VersionGate` withholds `accept-new` until OpenSSH is verified. | ✓ VERIFIED | `cmd/gitid/wiring.go:1384-1424` `GlobalSSHOptionStates` builds the view from `globalssh.Statuses` + `globalssh.VersionGate`; `lifecycle.go:825-830` refuses to apply a version-gated key when `VersionGate` doesn't return `VersionAvailable`. `TestGlobalSSHFixturePolicyParity`, `TestGlobalSSHOptionStatesVersionUnverified`, `TestGlobalSSHOptionStatesVersionTooOld` all PASS. |
| 7 (06-04) | Whole-graph shadow simulation runs before write; post-write re-verification runs after; ONE combined ceremony; selection starts empty. | ✓ VERIFIED | `lifecycle.go:843-861` (simulate stage, pre-write) and `:953-971` (verify stage, post-write, advisory-only) inside the same `runGlobalSSHApply`. Real PTY frames `global-ssh-shadowed-preview.txt` / `global-ssh-shadowed-receipt.txt` and the non-shadowing negative-control test (`TestGlobalSSH_RealPTYLaterDirectiveDoesNotShadow`) both pass. |
| 8 (06-05) | `PlanMigration`/`MigrateWithPlan` share one digest-carrying plan object; commit refuses on a changed-since-preview digest mismatch; Storage sub-tab is wired to the real engine; demo banner is off. | ✓ VERIFIED | `internal/sshconfig/migrate.go` `MigrateWithPlan` re-checks `plan.Digests` before any backup (`checkDigestMatch`, l.368-379). `cmd/gitid/wiring.go:679-685` `DemoBanner` excludes `TabGlobalSSH` explicitly. Live-executed: `gitid ssh storage migrate --to in-file --yes` against a real sandbox HOME correctly moved the `global-ssh` block from `config.d/gitid.config` into `~/.ssh/config`, took 2 timestamped backups, and reported `moved_globals: true`. `TestGlobalSSHStorage_RealPTYChangedSincePreview` passes. |
| 9 (06-06) | Frozen `gitid ssh` command tree (4 verbs), 4 versioned JSON envelopes, frozen exit-status table; CLI write verbs call the SAME ceremonies the TUI uses. | ✓ VERIFIED | `cmd/gitid/ssh.go:47,50` calls `b.runGlobalSSHApply` / `b.runSSHStorageMigrate` directly — no second write path. Live-executed against the compiled binary: `gitid ssh options list --json` → `gitid.ssh.options/v1`; `gitid ssh options apply HashKnownHosts --yes --json` → `gitid.ssh.apply/v1` with `exit_code: 0` even with an advisory present (never blocking); `gitid ssh storage show/migrate --json` → `gitid.ssh.storage/v1` / `gitid.ssh.migrate/v1`. `docs/cli-parity-matrix.md` has all 4 rows tagged `GSSH-01, SHELL-03`, `shipped`. |
| 10 (06-07) | Every Global SSH screen state is registered in the visual-regression gate with a classified allowlist; 4 working negative controls; full exit battery green; cross-AI review packet assembled. | ✓ VERIFIED | See truth 3's evidence. `.planning/phases/06-global-ssh-options/review-packet/MANIFEST.md` (163 table rows) closes every finding from the 5 review cycles as FIXED or an explicitly-reasoned deviation. `review-packet/frames/` (17 files) and `visual-divergence-allowlist.txt` are present. |

**Score:** 10/10 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/globalssh/policy.go`, `probe.go`, `classify.go`, `peralias.go`, `version.go`, `shadow.go` | D-01/D-10 engine: probe set, policy table, four-state classifier, per-alias check, version gate, whole-graph shadow simulation | ✓ VERIFIED | All exist, substantive, wired (confirmed via `gsd-tools query verify.artifacts` for each plan — 6/6, 5/5, 5/5 artifacts passed with no issues) and covered by passing unit tests. |
| `internal/sshconfig/globals.go`, `migrate.go`, `directives.go`, `validation.go` | Single `Host *` owner (`EnsureGlobals`), plan/commit-digest migration engine, directive scanner, exported Host-pattern matcher | ✓ VERIFIED | All exist and substantive; `EnsureGlobals` read in full (l.84-113); migration digest-lock contract read in full. |
| `internal/filewriter/filewriter.go` (`Backup`) | Pure timestamped-copy primitive, never replaces target | ✓ VERIFIED | Exists; consumed by `MigrateWithPlan`'s pre-write backup step. |
| `cmd/gitid/lifecycle.go` (`runGlobalSSHApply`, `runSSHStorageMigrate`) | The two per-verb write ceremonies | ✓ VERIFIED | Both read in full; both txMu-guarded; both are the only production call sites for their respective writes (confirmed by grep across `cmd/gitid/*.go`). |
| `cmd/gitid/wiring.go` (`CommitGlobalSSH`, `GlobalSSHOptionStates`, `SSHStorageMigrationPlan`, `CommitSSHStorage`) | TUI seam DTO conversion sites | ✓ VERIFIED | All present, exist, wired to `internal/tuikit` view types. |
| `cmd/gitid/ssh.go` | Real `gitid ssh` command group | ✓ VERIFIED | Exists; live-executed all 4 verbs against the compiled binary with correct JSON envelopes and exit codes. |
| `internal/tuikit/globalssh.go`, `views.go`, `design.go` | Options + Storage sub-tab rendering, view DTOs, GSSH-01 copy fixture | ✓ VERIFIED | All present; risk chips, four states, not-applicable reasons all render (confirmed in real PTY frame). |
| `.planning/design/global-ssh/visual-divergence-allowlist.txt` | Classified real-vs-dummy divergences | ✓ VERIFIED | 96 lines, consumed by `TestGlobalSSHAllowlistMatchesRegistry`/`TestGlobalSSHAllowlistFormat` (both pass). |
| `cmd/gitid/gate_visual_regression_test.go` | Global SSH capture merge + 4 negative controls | ✓ VERIFIED | All `TestGlobalSSH*` and `TestNegativeControl_GlobalSSH*` tests present and pass. |
| `.planning/phases/06-global-ssh-options/review-packet/` | Cross-AI review evidence: frames, allowlist, manifest | ✓ VERIFIED (manual — `gsd-tools query verify.artifacts` errors on directory paths, `EISDIR`) | Directory confirmed present with `MANIFEST.md` (163 rows), `frames/` (17 files), `visual-divergence-allowlist.txt` via direct `ls`/`grep`. |
| `docs/gitid-ssh-json-schema.md`, `docs/cli-parity-matrix.md` | Frozen JSON schema doc, parity-matrix rows | ✓ VERIFIED | Both exist; parity-matrix has all 4 `gitid ssh` rows tagged `GSSH-01, SHELL-03`. |
| `e2e/global_ssh_pty_e2e_test.go`, `global_ssh_storage_pty_e2e_test.go`, `global_ssh_cli_e2e_test.go` | Real-PTY and CLI e2e coverage | ✓ VERIFIED | 644/644/644 lines respectively; 17 tests total, all pass. |

### Key Link Verification

The `gsd-tools query verify.key-links` helper could not machine-parse most of this phase's `key_links` (their `from:` fields are descriptive, e.g. `"internal/tuikit/globalssh.go options list + detail pane"`, not bare file paths — the tool wants a literal path). Every link was instead verified manually by reading the source and/or grepping for the named symbol at both ends.

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/tuikit/globalssh.go` options list + detail pane | `Backend.GlobalSSHOptionStates` | `GlobalSSHOptionView` | ✓ WIRED | `globalssh.go:139` calls `m.backend.GlobalSSHOptionStates()`. |
| `cmd/gitid/wiring.go CommitGlobalSSH` | `cmd/gitid/lifecycle.go runGlobalSSHApply` | single ceremony chokepoint | ✓ WIRED | `wiring.go:1354-1367` `CommitGlobalSSH` calls `b.runGlobalSSHApply` exclusively. |
| `cmd/gitid/lifecycle.go runGlobalSSHApply` | `sshconfig.EnsureGlobals` | resolved storage target + filewriter backup | ✓ WIRED | `lifecycle.go:838,941` both call `sshconfig.EnsureGlobals`. |
| `internal/identity` CreateInput write seam | `sshconfig.EnsureGlobals` | single globals owner | ✓ WIRED | `internal/identity/identity.go:97-101` documents/uses `EnsureGlobals` via `GlobalsGOOS`. |
| `internal/sshconfig/migrate.go` migration classification | `sshconfig.IsGlobalBlockName` | narrow wildcard predicate | ✓ WIRED | `migrate.go:730` `case IsGlobalBlockName(b.Name):`. |
| `internal/doctor/checks/orphans.go` reserved-skip guard | `sshconfig.IsReservedBlockName` | single registry | ✓ WIRED | `orphans.go:59` `if sshconfig.IsReservedBlockName(name)`. |
| `internal/tuikit/globalssh.go optionRow + renderOptions` | `GlobalSSHOptionView.State/.NotApplicableReason` | four-state glyph/word/tone | ✓ WIRED | Rendered in real PTY evidence (`global-ssh-browse.txt`). |
| `cmd/gitid/wiring.go GlobalSSHOptionStates` | `globalssh.Statuses` + per-alias + `VersionGate` | one conversion site | ✓ WIRED | `wiring.go:1384,1413` call both; per-alias reached indirectly via `globalssh.Statuses` → `perAliasFromContent` (production-unused wrapper removed per WR-10, live call site confirmed at `classify.go:240`). |
| `cmd/gitid/wiring.go GlobalSSHApplyPlan` | `globalssh.Simulate` | `SimulationGraph` | ✓ WIRED | `wiring.go:1493` and `lifecycle.go:852` both call `globalssh.Simulate`. |
| `cmd/gitid/lifecycle.go runGlobalSSHApply` post-write | `globalssh.Verify` | live resolution probe re-run | ✓ WIRED | `lifecycle.go:957`. |
| `internal/globalssh/shadow.go shadowSourceFor` | `sshconfig.HostPatternsMatch` | exported negation-aware matcher | ✓ WIRED | Shared implementation confirmed via `internal/sshconfig/validation.go` exports + `internal/globalssh/shadow_test.go` doc comment pinning the shared call. |
| `internal/tuikit/globalssh.go renderStorage + storageCeremonyFor` | `Backend.SSHStorageMigrationPlan` | `SSHStorageMigrationView` | ✓ WIRED | `globalssh.go:164,235` call `m.backend.SSHStorageMigrationPlan`. |
| `cmd/gitid/lifecycle.go runSSHStorageMigrate` | `sshconfig.MigrateWithPlan`/`Migrate` | `RealMigrateDeps` | ✓ WIRED | `lifecycle.go:1095` calls `sshconfig.MigrateWithPlan(plan, deps)`. |
| `cmd/gitid/ssh.go apply verb` | `cmd/gitid/lifecycle.go runGlobalSSHApply` | shared ceremony | ✓ WIRED | `ssh.go:47`. |
| `cmd/gitid/ssh.go migrate verb` | `cmd/gitid/lifecycle.go runSSHStorageMigrate` | shared ceremony | ✓ WIRED | `ssh.go:50`. |
| `cmd/gitid/gate_visual_regression_test.go` | `internal/screenshot RequiredScreenSpecs` | merged registry | ✓ WIRED (tool-verified) | `gsd-tools query verify.key-links` confirmed this one automatically (`"Pattern found in source"`). |

### Data-Flow Trace (Level 4)

Live-executed against the compiled `gitid` binary in an isolated sandbox `HOME` (not the dummy, not a fixture):

- `gitid ssh options list --json` → real `Statuses`/`VersionGate` probe output (`OpenSSH_9.9p2` version note, real `ask`/`no` current values read from the live SSH config resolution) — **✓ FLOWING**, not a static fixture.
- `gitid ssh options apply HashKnownHosts --yes --json` → wrote `# BEGIN gitid managed: global-ssh ... HashKnownHosts yes ...` into a real file on disk, verified by reading the file back — **✓ FLOWING**.
- Re-running the same apply produced a **new** timestamped backup and byte-identical managed-block content — **✓ FLOWING**, idempotency proven by direct observation, not just by unit test.
- `gitid ssh storage migrate --to in-file --yes --json` moved the `global-ssh` block from `~/.ssh/config.d/gitid.config` into `~/.ssh/config`, confirmed by reading both files before/after — **✓ FLOWING**.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Options list renders real probe data | `gitid ssh options list --json` (real sandbox HOME) | Valid `gitid.ssh.options/v1` envelope, real `ssh -V`-derived version note | ✓ PASS |
| Apply refuses without `--yes` in non-interactive mode | `gitid ssh options apply HashKnownHosts` (no `--yes`) | `Error: gitid: refusing to apply ... without --yes`, exit 1 | ✓ PASS |
| Apply writes + backs up + is idempotent | `gitid ssh options apply HashKnownHosts --yes --json` × 2 | First write: file created, no backup (nothing to back up). Second write: byte-identical managed block, new timestamped backup. | ✓ PASS |
| Advisory shadowing never blocks the write | apply with a shadow-warning advisory present | `exit_code: 0`, advisory text present in JSON `advisories[]` | ✓ PASS |
| Storage show / migrate real round trip | `gitid ssh storage show/migrate --json` | Real layout detection; real migration moved the block; refuses a no-op migrate to the current layout with a named error | ✓ PASS |
| A single named behavior-dependent test: post-migration refetch uses the CONFIRMED (not stale) layout | `go test -run TestGlobalSSHStoragePostMigrationRefetchUsesConfirmedLayout ./internal/tuikit/...` | PASS (part of the full package race run) | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` convention is used by this project; this phase's runnable-verification is the e2e/PTY suite and the visual-regression gate, both executed directly above (see Behavioral Spot-Checks and Observable Truths #3).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| GSSH-01 | 06-01..06-07 (all 7 plans) | Danger-aware global-SSH-options screen, advisory + fixable, never blocking | ✓ SATISFIED | See Observable Truths #1, #2 and live CLI evidence. |
| DLV-04 (re-exercised) | 06-04, 06-05, 06-07 | Visual-regression gate: automated comparison classifies every difference | ✓ SATISFIED | Canonically owned by Phase 3 in REQUIREMENTS.md; Phase 6 re-exercises it per the roadmap's own SC3 wording. Gate passes for Global SSH's 7 registered specs + 4 negative controls. |
| DLV-06 (re-exercised) | 06-04, 06-05 | e2e per screen, driving real keystrokes in the real binary | ✓ SATISFIED | 17 real-PTY/CLI tests, all pass. |
| SHELL-03 (re-exercised) | 06-06 | CLI parity for new actions | ✓ SATISFIED | 4 new parity-matrix rows, machine-checked, all `shipped`. |

No orphaned requirements: `.planning/REQUIREMENTS.md` maps GSSH-01 to Phase 6 only, and all 7 plans declare it.

### Decision Coverage

`gsd-tools query check.decision-coverage-verify` could not fully parse `06-CONTEXT.md`'s `<decisions>` block (multi-line `- **D-NN — ...**` bullet bodies broke its single-line bullet parser for D-06/D-08/D-09/D-12) and returned a non-blocking `could-not-parse` warning (12 decisions detected, 0 auto-matched). This is a tooling formatting limitation, not a phase defect — every one of D-06 (single `Host *` owner), D-08 (rename + registry), D-09 (identities-first/globals-last ordering), D-11 (`IgnoreUnknown` placement), D-12 (state words), D-13 (VersionGate), D-14 (no decline state), D-15 (empty selection), D-16 (one combined ceremony) was independently confirmed present in the shipped code during this verification (see Observable Truths above). Non-blocking per gate spec; does not affect phase status.

### Anti-Patterns Found

None. Scanned all key phase files (`internal/globalssh/*.go`, `internal/sshconfig/globals.go`, `migrate.go`, `directives.go`, `internal/tuikit/globalssh.go`, `design.go`, `views.go`, `cmd/gitid/lifecycle.go`, `wiring.go`, `ssh.go`) for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` and "not implemented"/"coming soon" phrasing — zero matches. No stub returns, no hardcoded-empty stubs feeding rendered output found in the reviewed code paths (`DemoBanner` explicitly excludes `TabGlobalSSH`, confirming the view is real-wired, not demo-fixture-wired).

### Known, Accepted, Non-Blocking Deviations (carried from 06-REVIEW-FIX.md — not re-litigated)

- **WR-02** — `txMu` is acquired synchronously on the Bubble Tea update goroutine for the option-probe and storage-plan reads, which can freeze the UI briefly under lock contention. Documented as a genuine architectural change out of safe scope for the code-review-fix pass; a UX/responsiveness concern, not a correctness or safety defect (no incorrect data, no wrong writes). Recommended: a dedicated, human-reviewed follow-up plan.
- **WR-16** — 2 of 7 visual-regression checkpoints (`gss-apply-receipt`, `gss-storage-migrate-receipt`) are exempt from the in-process comparison gate because a real journal-backed write can't be captured in-process without new plumbing; both are asserted to have real PTY-frame evidence by name (`TestGlobalSSHNonApplicabilityNamesExistingPTYFrame`, confirmed passing).

### Additional Observation (informational, not a gap)

Migrating from the Include'd layout to in-file (`sshconfig.MigrateToInFile`/`composeSource`) deliberately and by design (pinned by `TestMigrateToInFileCarriesGlobalsBlock`, `internal/sshconfig/migrate_test.go:364-412`) leaves the `ssh-include` managed block (the `Include ~/.ssh/config.d/*.config` line) in `~/.ssh/config`, and leaves `~/.ssh/config.d/gitid.config` on disk emptied of gitid content rather than deleted. This was confirmed live: a real `gitid ssh storage migrate --to in-file --yes` left both artifacts behind. This is consistent with 06-02's own decision ("Include wiring stays stationary ... never relocated by a migration") and is harmless (an `Include` of an emptied file changes nothing on resolution) and honestly reported — the REAL backend's preview/dry-run text (`plan.Diff` via `buildMigrationDiff`) never claims the Include line is removed; only an unreachable dummy-only fallback string in `internal/tuikit/globalssh.go:477` (used solely when `view.Diff == ""`, which the real backend never produces) contains that claim. Since the real backend always populates `view.Diff`, this fallback string is never shown to a real user. No action required; flagged for awareness only.

### Human Verification Required

None. This is a user-facing TUI phase, but the phase's own success criterion 3 defines its UI-verification gate as automated (real-binary PTY e2e + automated real-vs-dummy comparison with negative controls) rather than a manual checkpoint — and that gate was executed directly in this verification pass with real, observed output (not SUMMARY.md claims). No behavior-dependent truth was left unexercised: the one state-transition-shaped truth checked (post-migration refetch using the confirmed, not stale, layout) has a passing named regression test that was run directly.

### Gaps Summary

No gaps. All 3 ROADMAP.md success criteria and a representative cross-section of the 7 plans' `must_haves` were verified against the actual codebase (not SUMMARY.md prose) via: direct source reading, `go build`/`go vet` (clean), the full workspace `go test -race -count=1 ./...` (19 packages, all green), the real-PTY/CLI e2e suite (17 tests, all green, 80s), the visual-regression gate including all 4 required negative controls (all green), `make gate-copy-freeze` (green), and live hands-on execution of the compiled `gitid` binary's `ssh options list/apply` and `ssh storage show/migrate` verbs against a real sandboxed `$HOME`, observing real file writes, real backups, real confirmation refusal, and real idempotency.

One environment-only observation: `golangci-lint` could not run cleanly in this verification sandbox because the installed Go toolchain (1.27.0) is newer than the one `golangci-lint` v2.12.2 was built with (1.26.2), causing a stdlib self-typecheck failure unrelated to any Phase 6 code. `06-REVIEW-FIX.md` already recorded a clean `golangci-lint run` (0 issues, plain + `--build-tags screenshot`) from the orchestrator's own environment; this is a local toolchain-version mismatch in the verification sandbox, not a phase defect.

---

_Verified: 2026-08-27T11:02:59Z_
_Verifier: Claude (gsd-verifier)_
