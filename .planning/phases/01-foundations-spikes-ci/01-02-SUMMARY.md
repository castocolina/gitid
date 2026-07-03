---
phase: 01-foundations-spikes-ci
plan: 02
subsystem: keygen
tags: [go, crypto-rsa, crypto-ed25519, golang.org/x/crypto/ssh, algorithm-registry]

# Dependency graph
requires:
  - phase: 01-foundations-spikes-ci (plan 01)
    provides: internal/platform capability probe (FIDOStatus.Usable(), ProbeKeyTypes token shape) that a future debug command will feed into ResolveAvailability
provides:
  - internal/keygen name-keyed algorithm registry (registry.go) replacing the hard-coded ed25519-only dispatch
  - Real rsa-4096 key generation (generateRSA4096) alongside unchanged ed25519 generation
  - Three registered-but-not-yet-implemented stubs (ecdsa-p256, ed25519-sk, ecdsa-sk) that never return key material
  - internal/keygen top-5 algorithm catalog (catalog.go: AlgoInfo, Catalog(), ResolveAvailability, Generatable) with per-OS metadata, decoupled from internal/platform
affects: [phase-3-create-flow, phase-5-rotate-new-key, 01-06-debug-list-command, phase-2-design-catalog-copy]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Name-keyed generatorFunc registry (map[string]generatorFunc + Register + init()) replacing hard-coded algorithm dispatch — new algorithms slot in via Register without touching GenerateMaterial"
    - "notYetImplemented(name) generator factory: registers a name in the dispatch table while guaranteeing it can never return partial key material"
    - "Implemented (static/build-time) vs Available (runtime/probe-time) as orthogonal AlgoInfo facts; Generatable() requires both — prevents a stub from ever being offered as generatable even if its wire token is locally present"

key-files:
  created:
    - internal/keygen/registry.go
    - internal/keygen/registry_test.go
    - internal/keygen/catalog.go
    - internal/keygen/catalog_test.go
  modified:
    - internal/keygen/keygen.go
    - internal/keygen/keygen_test.go

key-decisions:
  - "Registry populated via init()+Register() rather than a map literal, so Register is a real, testable extensibility seam (proven by TestRegistry_RegisterAddsDispatchEntry) rather than just a lookup table"
  - "generateRSA4096 passes the *rsa.PrivateKey pointer (never dereferenced) to ssh.MarshalPrivateKey/NewPublicKey per RESEARCH Pitfall 7, unlike ed25519's pass-by-value which already worked"
  - "KEY-04 permission re-proof persists GenerateMaterial output through the actual production chokepoint (filewriter.Write at 0600/0644, the same call cmd/gitid/add.go's buildDeps uses) rather than adding a new keygen.Generate function not requested by the plan's files_modified list"
  - "Catalog query tokens use the exact ssh -Q key wire format (sk-ssh-ed25519@openssh.com, sk-ecdsa-sha2-nistp256@openssh.com) per RESEARCH Pitfall 2, not the human-friendly -sk suffix names"
  - "ResolveAvailability takes plain (supportedTokens []string, fidoUsable bool) arguments and catalog.go has zero internal/platform import, preserving decoupling for 01-06's debug command to wire caps.FIDO.Usable()"

patterns-established:
  - "Pattern 1 (Algorithm Registry) from 01-RESEARCH.md, now implemented: map[string]generatorFunc dispatch with Register() as the only mutation point"

requirements-completed: [KEY-01, KEY-02, KEY-04]

# Metrics
duration: 25min
completed: 2026-07-03
---

# Phase 1 Plan 2: Algorithm Registry + Top-5 Catalog Summary

**Name-keyed keygen registry with real ed25519+rsa-4096 generation, three not-yet-implemented stubs that can never leak key material, and a 5-entry catalog with an Implemented∧Available Generatable guard.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-07-03T00:40:00Z (approx.)
- **Completed:** 2026-07-03T01:04:07Z
- **Tasks:** 2/2 completed
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments

- Refactored `keygen.GenerateMaterial` from a hard-coded `if p.Algo != "ed25519"` check into a name-keyed `registry` map with `Register`, dispatching to `generateEd25519` (unchanged) or the new `generateRSA4096`
- `generateRSA4096` generates a real 4096-bit RSA key via `crypto/rsa`, correctly passing the `*rsa.PrivateKey` pointer to `ssh.MarshalPrivateKey`/`ssh.NewPublicKey` (RESEARCH Pitfall 7)
- `ecdsa-p256`, `ed25519-sk`, `ecdsa-sk` are registered via `notYetImplemented(name)`: `GenerateMaterial` always returns a zero-value `Material` and a named "not yet implemented" error for these — proven by tests that registry presence never implies generation support
- Unknown algorithm names return a clear `keygen: unsupported algorithm %q` error (no panic)
- Added a 5-entry `Catalog()` (`ed25519` default, `rsa-4096`, `ecdsa-p256`, `ed25519-sk`, `ecdsa-sk`) with per-OS (`DarwinNote`/`LinuxNote`) and `Security` metadata
- Added `ResolveAvailability(cat, supportedTokens, fidoUsable)` that cross-references real `ssh -Q key` protocol tokens (including the `sk-...@openssh.com` wire format) against injected probe data, with no `internal/platform` import
- Added `Generatable(AlgoInfo) bool` requiring **both** `Implemented` and `Available`, so a stub is never presented as generatable even if its token is locally listed
- Re-proved KEY-04 (private 0600 / public 0644) for both real algorithms through the production `filewriter.Write` chokepoint

## Task Commits

Each task's TDD RED+GREEN cycle was folded into a single logical commit per CLAUDE.md's "logical groups, not small chunks" commit policy (explicitly permitted by the TDD execution flow):

1. **Task 1: Algorithm registry + rsa-4096 generator (KEY-02, KEY-04)** - `dcac0b6` (feat)
2. **Task 2: Top-5 algorithm catalog + availability resolver + generatable-guard (KEY-01)** - `e9e0835` (feat)

**Plan metadata:** committed separately as part of this step's final commit (see below).

## TDD Gate Compliance

Both tasks followed the RED→GREEN cycle but were committed as single `feat` commits, not separate `test`/`feat` commits, per CLAUDE.md's explicit commit-granularity rule ("Do not split the `test`/`feat`/`docs` of the same logical change into separate commits") and the plan's own permission ("You may fold RED+GREEN of one logical change into one commit").

RED was still verified as a genuine failing/uncompiling state before implementation, with real command output:
- Task 1: `go test ./internal/keygen/... -run 'TestRegistry|...' -race` failed with `undefined: Register` / `undefined: registry` before `registry.go` existed.
- Task 2: `go test ./internal/keygen/... -run 'TestCatalog|...' -race` failed with `undefined: Catalog` / `undefined: ResolveAvailability` before `catalog.go` existed.

No RED-state assertion ever passed unexpectedly (fail-fast rule respected). Both tasks then reached GREEN (all listed tests passing) before commit, per the transcript of `go test ./internal/keygen/... -race -v` runs above.

## Files Created/Modified

- `internal/keygen/registry.go` - `generatorFunc` type, `registry` map, `Register`, `notYetImplemented`, `init()` wiring all 5 algorithms
- `internal/keygen/registry_test.go` - stub-never-returns-material tests, unsupported-algo test, Register dispatch test, ed25519-unchanged test
- `internal/keygen/keygen.go` - `GenerateMaterial` now dispatches through `registry[p.Algo]`; `generateEd25519` (extracted verbatim) and `generateRSA4096` (new) added
- `internal/keygen/keygen_test.go` - added `TestGenerateRSA4096`, `TestGenerateRSA4096_Passphrase`, `TestPermissions_KeyFilesAfterRegistryRefactor` (0600/0644 re-proof via `filewriter.Write`)
- `internal/keygen/catalog.go` - `AlgoInfo` struct, `Catalog()`, `ResolveAvailability`, `Generatable`, `isHardwareBacked`
- `internal/keygen/catalog_test.go` - 5-entries/one-default/Implemented-flags/protocol-token/stub-generation-errors/availability/Generatable tests

## Decisions Made

- Registry populated via `init()` + `Register()` calls (not a bare map literal) so `Register` is exercised as a real extensibility point, matching the plan's `grep -q "func Register"` acceptance criterion and RESEARCH Pattern 1's intent.
- KEY-04 permission re-proof reuses the existing production write path (`filewriter.Write` at explicit `0o600`/`0o644`, mirroring `cmd/gitid/add.go`'s `buildDeps`) rather than inventing a new `keygen.Generate` function — the plan's `files_modified` list did not include adding a disk-writing `Generate` to this package, and `doc.go`'s description of a "Generate" function is aspirational/future (no such function exists in the codebase today outside the `buildDeps` closure in `cmd/gitid/add.go`).
- Catalog query tokens for the `-sk` entries use the exact OpenSSH wire format (`sk-ssh-ed25519@openssh.com`, `sk-ecdsa-sha2-nistp256@openssh.com`) per RESEARCH Pitfall 2, verified against the research session's real `ssh -Q key` output — never the human-friendly `ed25519-sk`/`ecdsa-sk` shorthand.
- `gosec` G101 false-positived on the algorithm-identifier string literals in `catalog.go` (same class of false positive already documented and `nolint`'d in `internal/platform/platform.go`); annotated each catalog entry's opening brace with the same `//nolint:gosec // G101 false positive: public algorithm identifier, not a credential` convention rather than suppressing gosec more broadly.

## Deviations from Plan

None - plan executed exactly as written. All `must_haves`, `key_links`, and both tasks' `acceptance_criteria` were met without needing Rule 1-4 deviations.

## Issues Encountered

None beyond the expected gosec G101 false positive on public algorithm-identifier string literals, resolved with the repo's existing `nolint` annotation convention (see Decisions Made).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/keygen` is ready for Phase 3 (create flow) and Phase 5 (rotate/new-key) to call `Catalog()`, `ResolveAvailability()`, and `Generatable()` to drive algorithm selection, and `GenerateMaterial` for the two real algorithms.
- 01-06 (debug/list command) can wire `internal/platform.ProbeKeyTypes()` + `caps.FIDO.Usable()` directly into `ResolveAvailability` without any `internal/keygen` changes, per the decoupling proven by `! grep -q "internal/platform" internal/keygen/catalog.go`.
- Phase 2 design owns final catalog ordering and marketing copy (Security/DarwinNote/LinuxNote wording) — the current strings are accurate but explicitly placeholder per D-06; a future design pass may reorder `Catalog()`'s slice or rewrite these fields without touching the Implemented/Available/Generatable machinery.
- No blockers.

---
*Phase: 01-foundations-spikes-ci*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 6 created/modified files confirmed present on disk; both task commits (`dcac0b6`, `e9e0835`) confirmed present in `git log --oneline --all`.
