---
phase: 04-doctor
plan: "07"
subsystem: doctor
tags: [go, doctor, ssh-agent, tty, exit-code, perms, TDD]

# Dependency graph
requires:
  - phase: 04-doctor
    provides: buildDoctorDeps wiring, CheckAgent/CheckSigning, applyFixes gate, perms check

provides:
  - RunSSHAdd wired to real ssh-add -l runner (arg-slice, G204-clean) in buildDoctorDeps
  - RunSSHKeygenFingerprint wired to real ssh-keygen -lf runner (arg-slice, G204-clean)
  - isTerminalInput TTY guard on the fix gate (non-interactive doctor skips Apply prompt)
  - Tiered 0/1/2/3 exit code propagated through main() via doctorExitCode var (IN-03)
  - gitconfig perms check corrected to warn on write-bits only (0644 default no longer SeverityError)
  - Permanent regression tests: wiring assertion, behavior, TTY gate, tiered exit, gitconfig perms

affects: [05-doctor-tui, phase-05]

# Tech tracking
tech-stack:
  added:
    - golang.org/x/term v0.44.0 (promoted from transitive to direct dependency for isTerminalInput)
  patterns:
    - arg-slice exec.Command pattern for ssh-add and ssh-keygen runners (G204-clean, no shell)
    - isTerminalInput using term.IsTerminal for stdin TTY detection (mirrors isTerminalOutput ModeCharDevice)
    - doctorExitCode package-level var: RunE sets, main() reads for os.Exit (tiered exit propagation)
    - checkGitconfigPath helper: only warns on group/world-writable bits (not read bits)

key-files:
  created:
    - cmd/gitid/doctor_agent_test.go
  modified:
    - cmd/gitid/doctor.go
    - cmd/gitid/main.go
    - internal/doctor/checks/perms.go
    - go.mod

key-decisions:
  - "DOC-GAP-02: RunSSHAdd and RunSSHKeygenFingerprint wired as package-level helpers runSSHAdd/runSSHKeygenFingerprint — small, testable, arg-slice only"
  - "DOC-GAP-03: TTY guard uses term.IsTerminal (golang.org/x/term) for cross-platform portability; fix flag preserves existing interactive semantics"
  - "IN-03: doctorExitCode package-level var — set by RunE before returning error; main() reads it with fallback to 1 for non-doctor errors"
  - "WR-03: checkGitconfigPath replaces the 0600 gitconfig check; only group/world-write bits (0o022 mask) trigger a SeverityWarning — read bits are acceptable per git's own 0644 default"

patterns-established:
  - "Pattern: pkg-level runner helpers for external commands — runSSHAdd/runSSHKeygenFingerprint are package-level funcs assigned into Deps literal (keeps buildDoctorDeps readable)"
  - "Pattern: tiered exit code via package-level var — doctorExitCode is the seam between RunE and main(); zero-value safe for all other commands"
  - "Pattern: checkXxxPath helper for non-standard mode rules — extract from CheckPermissions when a path class needs distinct logic"

requirements-completed: [DOC-05, DOC-06]

# Metrics
duration: 45min
completed: 2026-06-12
---

# Phase 04 Plan 07: Doctor Gap Closure (DOC-GAP-02/03, IN-03, WR-03) Summary

**Real ssh-add/ssh-keygen runners wired into buildDoctorDeps, non-interactive doctor TTY-guarded, tiered 0/1/2/3 exit code propagated through main(), and default 0644 gitconfig no longer flagged as a false-positive SeverityError**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-06-12T13:00:00Z
- **Completed:** 2026-06-12T13:45:00Z
- **Tasks:** 2 (RED + GREEN, TDD)
- **Files modified:** 4 (+ 1 created)

## Accomplishments

- Closed DOC-GAP-02: `RunSSHAdd` and `RunSSHKeygenFingerprint` now wired as real arg-slice exec closures in `buildDoctorDeps` — `CheckAgent` runs in production and will report a down ssh-agent or unloaded keys
- Closed DOC-GAP-03: `isTerminalInput` guard on the `applyFixes` gate so bare `gitid doctor` in CI/pipes never emits the `Apply N fix(es)?` prompt into machine-parsed output
- Closed IN-03: `main()` propagates the tiered 0/1/2/3 exit code via `doctorExitCode` package-level var instead of collapsing all errors to `os.Exit(1)`
- Closed WR-03: `checkGitconfigPath` replaces the 0600 gitconfig check; default 0644 files are clean; only group/world-writable files trigger a SeverityWarning

## Task Commits

1. **Task 1: RED — agent wiring, TTY gate, tiered exit, gitconfig perms tests** - `fa8f72f` (test)
2. **Task 2: GREEN — wire runners, TTY guard, tiered exit, gitconfig fix** - `2425f9a` (feat)

## Files Created/Modified

- `cmd/gitid/doctor_agent_test.go` - Five tests: wiring assertion, behavior w/ bad SSH_AUTH_SOCK, non-interactive gate, tiered exit code seam, 0644 gitconfig perms
- `cmd/gitid/doctor.go` - Added `runSSHAdd`/`runSSHKeygenFingerprint` helpers + Deps wiring; `isTerminalInput` TTY guard; `doctorExitCode` var; `os/exec`, `errors`, `golang.org/x/term` imports
- `cmd/gitid/main.go` - `main()` reads `doctorExitCode` for `os.Exit`; falls back to 1 for non-doctor errors
- `internal/doctor/checks/perms.go` - Added `modeGitconfig = 0o644`, `checkGitconfigPath` helper (warns on write-bits only, not read); removed flat 0600 gitconfig check
- `go.mod` - Promoted `golang.org/x/term v0.44.0` from indirect to direct dependency

## Decisions Made

1. **runSSHAdd returns (string, int)**: captures combined output and exit code via `exec.ExitError.ExitCode()`; non-ExitError (binary not found) returns `("", 2)` so `classifyAgentState` treats it as unreachable. Consistent with classifyAgentState semantics.

2. **isTerminalInput uses `term.IsTerminal`**: mirrors the `isTerminalOutput` ModeCharDevice approach but uses the stdlib-extension term package for cross-platform correctness (Windows, plan for Phase 5 TUI). The fix flag still bypasses the guard.

3. **doctorExitCode pattern**: a package-level `var doctorExitCode int` set by doctor's RunE before returning a non-nil error. main() reads it with fallback `if code == 0 { code = 1 }` for non-doctor commands that also error. This is simpler and more portable than changing the `Execute()` signature.

4. **checkGitconfigPath write-mask check**: mask `0o022` (group-write + world-write) is the meaningful threat; read access to ~/.gitconfig is intentional and harmless. `git config` reads it without elevated permissions by design.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None. All four gap closures produce real observable behavior.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: cmd-injection-mitigated | cmd/gitid/doctor.go | Two new external process invocations (ssh-add, ssh-keygen) use arg-slice form only; no shell expansion possible (T-04-22 mitigated as planned) |

## Self-Check: PASSED

Files verified:
- `cmd/gitid/doctor_agent_test.go`: FOUND
- `cmd/gitid/doctor.go`: FOUND (contains RunSSHAdd:, RunSSHKeygenFingerprint:, isTerminalInput, doctorExitCode)
- `cmd/gitid/main.go`: FOUND (contains doctorExitCode reference)
- `internal/doctor/checks/perms.go`: FOUND (contains checkGitconfigPath)
- `go.mod`: FOUND (golang.org/x/term direct)

Commits verified:
- `fa8f72f`: FOUND (test(04-07): failing tests for agent wiring, TTY gate, tiered exit, gitconfig perms)
- `2425f9a`: FOUND (feat(04-07): wire ssh-add/ssh-keygen, TTY-guard fix gate, tiered exit code, gitconfig perms)

Success criteria verification:
- `go test ./cmd/gitid/... -run 'TestDoctorAgent|TestDoctorGate|TestDoctorExitCode|TestDoctorGitconfigPerms|TestDoctorNonInteractive|TestDoctorTiered'`: ALL PASS (7/7)
- `grep -n 'RunSSHAdd:' cmd/gitid/doctor.go`: MATCH (line 216)
- `grep -n 'RunSSHKeygenFingerprint:' cmd/gitid/doctor.go`: MATCH (line 217)
- `go build ./...`: EXIT 0
- `go test ./...`: ALL PASS
- `make lint`: 0 ISSUES

---
*Phase: 04-doctor*
*Completed: 2026-06-12*
