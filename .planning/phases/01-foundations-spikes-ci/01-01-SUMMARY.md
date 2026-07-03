---
phase: 01-foundations-spikes-ci
plan: 01
subsystem: platform-probing
tags: [go, exec.CommandContext, ssh, fido2, capability-probe, injectable-deps]

# Dependency graph
requires: []
provides:
  - "SSHVersion{OpenSSHVersion,SSLFlavor,SSLVersion,Raw} + ProbeSSHVersion()/parseSSHVersion (structured `ssh -V` parse, LibreSSL/OpenSSL flavor-aware)"
  - "ssh -Q key token -> catalog algorithm-name mapping incl. sk- FIDO2 variants: AlgorithmForToken()/SupportedAlgorithms()"
  - "Three-valued AgentStatus/FIDOStatus/KeychainStatus + Capabilities struct + injectable Deps + Probe(ctx, deps)"
  - "EXPORTED BuildProbeDeps() real-wiring constructor in internal/platform for cross-package use by cmd/gitid + e2e (01-06)"
  - "internal/platform.ProbeKeyTypes retrofit to exec.CommandContext with a bounded timeout"
  - "libfido2 InstallHint family entry (normalizeTool + libfido2InstallHint) in platform.go"
affects: [01-02-keygen-registry, 01-06-debug-command, 01-05-identity-doctor]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Injectable exec.CommandContext probe seam: thin I/O wrapper + pure parse function, package-level probeTimeout var (not const, so tests can shrink it)"
    - "EXPORTED BuildXDeps() real-wiring constructor pattern (first instance in this repo — internal/doctor's equivalent stays cmd-layer-local; platform.BuildProbeDeps is package-exported per plan requirement for 01-06 cross-package use)"
    - "Three-valued status enums with String() + a .Usable()-style bool-collapse helper for callers that don't need the fine-grained enum"

key-files:
  created:
    - internal/platform/version.go
    - internal/platform/version_test.go
    - internal/platform/keytypes.go
    - internal/platform/keytypes_test.go
    - internal/platform/capabilities.go
    - internal/platform/capabilities_test.go
  modified:
    - internal/platform/platform.go
    - internal/platform/platform_test.go

key-decisions:
  - "probeTimeout is a package-level var (3s default), not a const, so TestProbeTimeout can shrink it to 100ms and prove exec.CommandContext actually kills a hung `ssh-add` fake within bounds, rather than only asserting on injected-Deps behavior."
  - "probeFIDO re-runs `ssh -Q key` independently of ProbeKeyTypes (duplicate but cheap subprocess call) rather than sharing state, keeping each Deps field a fully independent, self-contained external effect — consistent with the existing Deps-closure pattern elsewhere in the repo (doctor.Deps, identity.Deps)."
  - "Deps.ProbeSSHVersion / Deps.ProbeKeyTypes take no ctx parameter (they reuse the existing zero-arg exported functions, each with its own internal timeout); Deps.ProbeAgent / Deps.ProbeFIDO take ctx explicitly since they are new probes introduced in this task and thread cancellation through via context.WithTimeout(ctx, probeTimeout)."
  - "ProbeKeyTypes' error-wrap message gained a `platform: ` prefix during the exec.CommandContext retrofit, aligning it with the per-package error-wrapping convention used by the new version.go/capabilities.go code — no test depended on the old unprefixed string."

patterns-established:
  - "Three-valued status types (AgentStatus/FIDOStatus/KeychainStatus) over coarse bools for probe results that later phases (doctor, debug command) need to render distinctly (e.g. 'no agent' vs 'agent locked')."
  - "Package-exported BuildXDeps() real-wiring constructor + a dedicated TestXDepsWiring test that exercises it end-to-end (not just a fake) — the fix for the project's documented 'injected-seam wiring blindspot'."

requirements-completed: [PLAT-01, PLAT-02, KEY-03]

# Metrics
duration: ~15min
completed: 2026-07-02
---

# Phase 1 Plan 1: Local Capability Probe Layer Summary

**Structured `ssh -V`/`ssh -Q key` parsing plus an injectable, three-valued agent/FIDO/keychain capability probe (`internal/platform`), every external probe bounded by an `exec.CommandContext` timeout.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-07-02
- **Tasks:** 2/2
- **Files modified:** 8 (6 created, 2 modified)

## Accomplishments

- `SSHVersion{OpenSSHVersion, SSLFlavor, SSLVersion, Raw}` + `ProbeSSHVersion()`/`parseSSHVersion` parse both the LibreSSL (macOS) and OpenSSL (Linux) `ssh -V` formats into a structured value — never a lossy pre-formatted string.
- Exact-match `ssh -Q key` token → catalog algorithm-name mapping (`AlgorithmForToken`/`SupportedAlgorithms`), correctly resolving the FIDO2 `sk-ssh-ed25519@openssh.com` / `sk-ecdsa-sha2-nistp256@openssh.com` protocol tokens (never the informal `"ed25519-sk"` shorthand via substring match).
- Three-valued `AgentStatus`/`FIDOStatus`/`KeychainStatus` (each with `String()`) plus a `Capabilities` struct and fully injectable `Deps`/`Probe(ctx, deps)` orchestration — no bare bools for states later phases (doctor, debug command) need to distinguish finely.
- `BuildProbeDeps()` is EXPORTED (capital B) in `internal/platform/capabilities.go` and is exercised end-to-end by `TestProbeDepsWiring` — closing the project's documented "injected-seam wiring blindspot" for this probe surface.
- Every new/retrofitted external probe (`ssh -V`, `ssh -Q key`, `ssh-add -l`) runs under a bounded `exec.CommandContext` timeout (shared `probeTimeout` var); `TestProbeTimeout` proves a hung fake `ssh-add` returns `AgentLockedOrUnavailable` promptly instead of blocking.
- `libfido2InstallHint` + a `"libfido2"`/`"ssh-sk-helper"` `normalizeTool` case extend the existing `InstallHint` family in `platform.go` (not `install.go`) with per-OS troubleshooting guidance (KEY-03).

## Task Commits

Each task was committed atomically (RED+GREEN folded into one logical commit per CLAUDE.md's commit-grouping rule):

1. **Task 1: ssh -V version parse (struct return) + ssh -Q key token→algorithm mapping (PLAT-01)** - `654e5ed` (feat)
2. **Task 2: Injectable agent/FIDO/keychain probe (three-valued statuses) under CommandContext timeouts + libfido2 install hint (PLAT-02, KEY-03)** - `b036a79` (feat)

_TDD authoring order (RED failing tests first, then GREEN implementation) was followed within each task; both states are folded into the single task commit per CLAUDE.md's "commits in logical groups, not per-tiny-step" rule and the plan's own "you may fold RED+GREEN of one logical change into one commit" allowance._

## Files Created/Modified

- `internal/platform/version.go` - `SSHVersion` struct, `ProbeSSHVersion()`, pure `parseSSHVersion`, package-level `probeTimeout` var
- `internal/platform/version_test.go` - LibreSSL/OpenSSL/malformed/empty fixtures + real `ProbeSSHVersion()` struct-field assertion
- `internal/platform/keytypes.go` - `algorithmToken` map, `AlgorithmForToken`, `SupportedAlgorithms`
- `internal/platform/keytypes_test.go` - exact-match FIDO2 token mapping tests (rejects the wrong `"ed25519-sk"` shorthand) + `SupportedAlgorithms` set test
- `internal/platform/capabilities.go` - `AgentStatus`/`FIDOStatus`/`KeychainStatus` (+ `Usable()`), `Capabilities`, `Deps`, `Probe`, `BuildProbeDeps`, `probeAgent`, `probeFIDO`, `probeKeychain`
- `internal/platform/capabilities_test.go` - `Probe` orchestration tests, `TestProbeDepsWiring` (real constructor), `TestProbeTimeout` (real hung-fake-binary proof)
- `internal/platform/platform.go` - `ProbeKeyTypes` retrofit to `exec.CommandContext` with `probeTimeout`; `normalizeTool` + `libfido2InstallHint` additions
- `internal/platform/platform_test.go` - `TestLibfido2Hint`

## Decisions Made

- `probeTimeout` is a package-level `var` (not `const`), letting `TestProbeTimeout` shrink it to prove the exec.CommandContext timeout actually terminates a hung external process, rather than only asserting against injected fakes.
- `probeFIDO` independently re-runs `ssh -Q key` rather than sharing the `Capabilities.KeyTypes` result, keeping each `Deps` field a fully self-contained external effect (consistent with existing `Deps`-closure conventions elsewhere in the repo).
- `Deps.ProbeAgent`/`Deps.ProbeFIDO` take `ctx context.Context` (new probes, ctx threaded through `context.WithTimeout(ctx, probeTimeout)`); `Deps.ProbeSSHVersion`/`Deps.ProbeKeyTypes` stay zero-arg, reusing the existing exported functions which manage their own internal timeout — this keeps `ProbeSSHVersion()`/`ProbeKeyTypes()`'s existing signatures unchanged per the plan's explicit constraint.

## Deviations from Plan

None - plan executed exactly as written. The `ProbeKeyTypes` error-wrap message gained a `"platform: "` prefix (previously unprefixed) as part of the mandated `exec.CommandContext` retrofit, to align with the per-package error-wrapping convention applied to the rest of this task's exec calls — no test or caller depended on the old string, and this falls within the task's own explicit "retrofit ProbeKeyTypes" scope, not a separate deviation.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/platform` now exposes everything 01-02 (algorithm registry/catalog) needs to resolve per-OS availability: `SupportedAlgorithms`, `Capabilities.FIDO.Usable()`, `Capabilities.Keychain`.
- `BuildProbeDeps()` is ready for `cmd/gitid`'s new debug/list command (01-06) to wire directly, with the real-wiring path already test-exercised in this plan.
- No blockers for the next wave.

---
*Phase: 01-foundations-spikes-ci*
*Completed: 2026-07-02*

## Self-Check: PASSED

All created files verified present on disk; both task commits (`654e5ed`, `b036a79`) verified present in git log.
