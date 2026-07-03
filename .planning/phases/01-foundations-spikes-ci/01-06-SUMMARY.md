---
phase: 01-foundations-spikes-ci
plan: 06
subsystem: cli
tags: [go, cobra, cli, debug-command, key-01, plat-01, mgr-02, dlv-07, e2e]

# Dependency graph
requires:
  - phase: 01-foundations-spikes-ci (plan 01)
    provides: internal/platform/capabilities.go — Probe, Deps, EXPORTED BuildProbeDeps() real wiring, AgentStatus/FIDOStatus/KeychainStatus, KeyTypes vs Algorithms
  - phase: 01-foundations-spikes-ci (plan 02)
    provides: internal/keygen/catalog.go — Catalog(), ResolveAvailability(cat, supportedTokens, fidoUsable), Generatable(a)
  - phase: 01-foundations-spikes-ci (plan 04)
    provides: internal/identity/inventory.go + state.go — BuildInventory(deps), EXPORTED BuildInventoryDeps(), IdentityHealth
provides:
  - cmd/gitid/debug.go — newDebugCmd()/newDebugCapsCmd() (Cobra thin glue), runDebugCaps/runDebugCapsWithDeps, printCapabilities/printCatalog/printInventory
  - e2e/debug_e2e_test.go — real-binary e2e proof of the platform.BuildProbeDeps + identity.BuildInventoryDeps wiring, closing the injected-seam blindspot for this command
affects: [phase-5-identity-manager]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Cobra thin-glue command surface (mirrors newDoctorCmd): RunE only gathers input from internal/* packages and prints — no classification/aggregation logic lives in cmd/"
    - "Testable-deps split: runDebugCaps(ctx, out) wires the two real EXPORTED closures (platform.BuildProbeDeps, identity.BuildInventoryDeps); runDebugCapsWithDeps(ctx, out, probeDeps, invDeps) is the same orchestration with deps injected, so unit tests exercise identical logic against fakes while the e2e test exercises the real wiring through the built binary"
    - "Raw-token availability resolution: keygen.ResolveAvailability(cat, caps.KeyTypes, caps.FIDO.Usable()) — the RAW `ssh -Q key` protocol tokens, never the already-mapped caps.Algorithms (which would silently resolve every catalog entry Available=false)"

key-files:
  created:
    - cmd/gitid/debug.go
    - cmd/gitid/debug_test.go
    - e2e/debug_e2e_test.go
  modified:
    - cmd/gitid/main.go

key-decisions:
  - "debug caps prints three sections (Capabilities, Algorithm Catalog, Identities) via three dedicated print* helpers, each taking only the already-resolved data structure (platform.Capabilities / []keygen.AlgoInfo / identity.Inventory) — no re-derivation of any classification fact in cmd/, satisfying the plan's 'does NOT re-derive identity/key state' and 'no logic lives in the command itself' constraints"
  - "runDebugCapsWithDeps is a plan-adjacent testability seam (not in the plan's file list as a named symbol, but required to unit-test the orchestration's error path per Rule 2 — a probe error must propagate, not be silently swallowed) — runDebugCaps (the function referenced by acceptance-criteria greps) still wires the two real EXPORTED constructors directly"
  - "The e2e test uses a plain exec.Command against the harness's sandboxed HOME + built binary (the adopt_e2e_test.go pattern), not the raw-keystroke PTY harness (ui_pty_e2e_test.go) — debug caps is non-interactive (prints and exits), so PTY emulation adds no additional proof of real wiring over a direct stdout capture"
  - "printCatalog prints the per-OS note for platform.CurrentOS() only (not both Darwin and Linux notes) — the local machine's toolchain is the only one relevant to the local capability readout this command reports"

requirements-completed: [KEY-01, PLAT-01, MGR-02, DLV-07]

# Metrics
duration: ~35min
completed: 2026-07-03
---

# Phase 1 Plan 6: `gitid debug caps` Command — Catalog + Probe + Identity Inventory Summary

**A thin-glue `gitid debug caps` Cobra command surfaces the KEY-01 algorithm catalog (availability resolved from raw `ssh -Q key` tokens), the PLAT-01 structured capability probe, and per-identity MGR-02 `IdentityHealth` via `identity.BuildInventory` — proven end-to-end by a real-binary e2e test that exercises the actual `platform.BuildProbeDeps`/`identity.BuildInventoryDeps` wiring, with unit and e2e tests both asserting zero secret leakage.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 2/2 completed
- **Files created:** 3 (debug.go, debug_test.go, debug_e2e_test.go)
- **Files modified:** 1 (main.go)

## Accomplishments

- `gitid debug caps` prints three ordered sections: `=== Capabilities ===` (structured `SSHVersion` fields, SSL flavor/version, and the `AgentStatus`/`FIDOStatus`/`KeychainStatus` STATUS strings — never raw bools), `=== Algorithm Catalog ===` (all 5 top algorithms with `Implemented`/`Available`/`Generatable`/`Default` flags, security note, and the current-OS note), and `=== Identities ===` (every gitid-managed identity's `IdentityHealth` — both axes + Problems — plus the global `UnusedKeys` list)
- Catalog availability is resolved via `keygen.ResolveAvailability(keygen.Catalog(), caps.KeyTypes, caps.FIDO.Usable())` — bound to the RAW `ssh -Q key` protocol tokens (`caps.KeyTypes`), not the already-mapped `caps.Algorithms` — proven by both the unit test (fake `ProbeKeyTypes` returning `ssh-ed25519`) and the e2e test (the sandbox's real `ssh -Q key`) resolving the `ed25519` entry `available: true`
- Local capabilities are gathered via `platform.Probe(ctx, platform.BuildProbeDeps())`, using the EXPORTED real-wiring constructor from 01-01 (capital B) so no fake probe implementation lives in `cmd/gitid`
- Identity state is consumed (never re-derived) via `identity.BuildInventory(identity.BuildInventoryDeps())`, the EXPORTED real aggregation layer from 01-04 — `cmd/gitid/debug.go` contains no `Classify`/`ClassifyState` call or redefinition
- Zero secret leakage: the command never references `keygen.Material`/`PrivPEM`, never prints passphrase fields, never dumps raw private-key contents, and never dumps the process environment — verified by three separate assertions across the unit test (`TestDebugCaps_NoSecretLeakage`) and the e2e test (`TestDebugCaps_RealWiring`), each checking for `"PRIVATE KEY"`, `"passphrase"`/`"Passphrase"`/`"PrivPEM"`, and a `"PATH="` full-env-dump marker
- `e2e/debug_e2e_test.go` drives the REAL built `gitid` binary's `debug caps` subcommand against a sandboxed `HOME` seeded with one managed identity (`seedMinimalIdentity`, reused from `ui_pty_e2e_test.go`), closing the project's recurring injected-seam wiring blindspot for this new command surface (mirrors `doctor_realwiring_test.go`'s "real wiring, not stubs" pattern) — asserts the real `ssh -Q key`-derived catalog, the real capability probe, and the real per-identity `IdentityHealth`, all end-to-end through the actual binary
- `runDebugCaps(ctx, out)` wires the two real EXPORTED constructors directly; `runDebugCapsWithDeps(ctx, out, probeDeps, invDeps)` is the same orchestration with both dependency sets injected, so `TestDebugCaps_ProbeError` can assert a probe failure propagates as a wrapped error instead of being silently swallowed (Rule 1/2 correctness addition, not in the plan's original `must_haves` but required for a meaningful error-path test)

## Task Commits

1. **Task 1: `gitid debug caps` command wiring catalog + probe + identity inventory (KEY-01, PLAT-01, MGR-02)** - `3689528` (feat)
2. **Task 2: PTY/e2e test driving the REAL binary's debug command (injected-seam closure, DLV-07)** - `4666f17` (test)

**Plan metadata:** committed separately as part of this step's final commit (see below).

## Files Created/Modified

- `cmd/gitid/debug.go` - `newDebugCmd`, `newDebugCapsCmd`, `runDebugCaps`, `runDebugCapsWithDeps`, `printCapabilities`, `printCatalog`, `osNote`, `printInventory`, `renderProblems`, `joinStrings`
- `cmd/gitid/debug_test.go` - `TestDebugCommand_Registered`, `fakeCapsDeps`, `fakeInventoryDeps`, `TestDebugCaps_PrintsAllThreeSections`, `TestDebugCaps_NoSecretLeakage`, `TestDebugCaps_ProbeError`
- `cmd/gitid/main.go` - registers `newDebugCmd()` on the root command (D-08)
- `e2e/debug_e2e_test.go` - `TestDebugCaps_RealWiring` (real-binary end-to-end proof)

## Decisions Made

- Print helpers each accept only their already-resolved data structure (`platform.Capabilities`, `[]keygen.AlgoInfo`, `identity.Inventory`) — the command layer performs zero classification, exactly matching 01-PATTERNS.md's "command layer only gathers input/prints output; all logic lives in `internal/*` packages" rule.
- Added `runDebugCapsWithDeps` as a testability seam distinct from `runDebugCaps` (the latter is what the acceptance-criteria greps for `platform.BuildProbeDeps`/`BuildInventory` target) so the unit suite can assert the probe-error path without needing real exec calls.
- Chose a plain `exec.Command` e2e harness pattern (same as `adopt_e2e_test.go`) over the raw-keystroke PTY harness (`ui_pty_e2e_test.go`) because `debug caps` is a non-interactive print-and-exit command — PTY emulation is reserved for surfaces that require terminal input decoding, which this command does not have.
- `printCatalog` reports the per-OS note only for `platform.CurrentOS()` (the local machine), not both `DarwinNote` and `LinuxNote` — the debug readout is scoped to "what does THIS machine support," matching the command's own local-capability-probe framing.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Probe-error propagation test**
- **Found during:** Task 1
- **Issue:** The plan's `must_haves`/`acceptance_criteria` did not explicitly require a probe-failure test, but a command that silently swallows a `platform.Probe` error (printing an empty/misleading report) would violate the general correctness bar and 01-PATTERNS.md's per-package error-wrapping convention (`fmt.Errorf("<packagename>: <action>: %w", err)`, already applied in `runDebugCaps`/`runDebugCapsWithDeps`).
- **Fix:** Added `runDebugCapsWithDeps` (deps-injected variant of `runDebugCaps`) and `TestDebugCaps_ProbeError`, asserting a failing `ProbeSSHVersion` propagates as a wrapped error via `errors.Is`.
- **Files modified:** `cmd/gitid/debug.go`, `cmd/gitid/debug_test.go`
- **Verification:** `go test ./cmd/gitid/... -run TestDebugCaps_ProbeError -race` passes.
- **Committed in:** `3689528` (part of Task 1 commit)

**2. [Rule 1 - Bug] Acceptance-grep false positive on doc comment**
- **Found during:** Task 1 (self-verification against acceptance criteria)
- **Issue:** An early doc comment on `runDebugCaps` explaining what the command must NEVER print used the literal substring `PrivPEM`, which the plan's own acceptance grep (`! grep -q "PrivPEM\|Material{\|os.Environ" cmd/gitid/debug.go`) matches against the whole file, including comments — the comment itself would have failed the grep it was documenting.
- **Fix:** Reworded the comment to describe the constraint without using the literal forbidden substring ("gitid's private-key-material type" instead of naming `keygen.Material`/`PrivPEM` directly).
- **Files modified:** `cmd/gitid/debug.go`
- **Verification:** `! grep -q "PrivPEM\|Material{\|os.Environ" cmd/gitid/debug.go` exits 0.
- **Committed in:** `3689528` (part of Task 1 commit)

## Auth Gates Encountered

None — this command reads local process state only (`ssh -Q key`, `ssh -V`, `ssh-add -l`, config files); no external service authentication is involved.

## Issues Encountered

None beyond the two auto-fixes documented above. Both `TestDebugCaps_PrintsAllThreeSections`/`TestDebugCaps_NoSecretLeakage` (unit) and `TestDebugCaps_RealWiring` (e2e) passed on their first run after implementation — no debugging iterations needed. `make lint` (which does not apply the `e2e` build tag by default, matching the rest of the `e2e/` package) and `golangci-lint run --build-tags e2e ./e2e/...` (manually verified, showing pre-existing unrelated findings only in other e2e files, none in `debug_e2e_test.go`) both confirm no new lint issues.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Phase 5 (Identity Manager) can reuse `identity.BuildInventory`/`IdentityHealth` rendering conventions established here (per-identity state + problems list) for its own UI surface.
- The `gitid debug caps` surface is Phase 1's permanent proof point for KEY-01/PLAT-01/MGR-02; no further Phase 1 work depends on it, but it remains available as a diagnostic tool for later phases' manual verification.
- No blockers.

---
*Phase: 01-foundations-spikes-ci*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 4 created/modified files confirmed present on disk (`cmd/gitid/debug.go`, `cmd/gitid/debug_test.go`, `e2e/debug_e2e_test.go`, `cmd/gitid/main.go`); both task commits (`3689528`, `4666f17`) confirmed present in `git log --oneline --all`.
