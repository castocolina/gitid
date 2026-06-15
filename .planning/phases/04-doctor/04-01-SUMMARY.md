---
phase: "04-doctor"
plan: "01"
subsystem: doctor
tags: [doctor, permissions, tdd, cobra, ansi]
dependency_graph:
  requires: []
  provides: [doctor-model, CheckPermissions, gitid-doctor-cmd]
  affects: [phase-05-tui, plans-04-02-04-03-04-04-04-05]
tech_stack:
  added: []
  patterns: [injected-deps, TDD-RED-GREEN, per-family-check-files, import-cycle-avoidance]
key_files:
  created:
    - internal/doctor/doctor.go
    - internal/doctor/doctor_test.go
    - internal/doctor/checks/perms.go
    - internal/doctor/checks/perms_test.go
    - internal/doctor/checks/deps.go
    - internal/doctor/checks/baseline.go
    - internal/doctor/checks/coherence.go
    - internal/doctor/checks/orphans.go
    - internal/doctor/checks/signing.go
    - cmd/gitid/doctor.go
    - cmd/gitid/doctor_test.go
  modified:
    - cmd/gitid/main.go
    - internal/doctor/doc.go (preserved, no change needed)
decisions:
  - "CheckFn type alias in doctor package; cmd layer wires checks.* into Deps fields to avoid doctor→checks import cycle"
  - "KeyPaths/PubKeyPaths []string fields added to Deps for identity-specific key paths (populated from identity.Reconstruct in buildDoctorDeps)"
  - "Wave-2 Deps contract frozen: 18 fields documented below"
metrics:
  duration: "~40min"
  completed: "2026-06-11"
  tasks: 3
  files: 12
---

# Phase 04 Plan 01: Doctor Foundation Summary

Vertical slice for `gitid doctor`: the read-only Finding/Severity/Family data model,
the `doctor.Deps` injected-function struct (Wave-2 locked contract), the `Run()` check
orchestrator, `ExitCode()` severity aggregation, the real `CheckPermissions` family, and
a runnable `gitid doctor` Cobra command that renders a grouped-by-family report.

## Commits

| Commit | Message |
|--------|---------|
| `9d84e7c` | test(04-01): add failing tests for Finding/Severity/Family/ExitCode/Families model |
| `031e1fc` | feat(04-01): implement Finding/Severity/Family/ExitCode/Families model + Run dispatch |
| `eae2822` | test(04-01): add failing tests for CheckPermissions (perms family RED) |
| `a4b3795` | feat(04-01): implement CheckPermissions — KEY-02 mode checks with injected FixPerm |
| `f8b9ac8` | feat(04-01): add gitid doctor command with grouped renderer and root registration |

## TDD Gate Compliance

- RED gate (test commits): `9d84e7c` and `eae2822` — both precede their GREEN commits.
- GREEN gate (feat commits): `031e1fc` and `a4b3795` — implement only after RED is confirmed.
- No REFACTOR commit required (code was clean on first pass).

## Artifacts Produced

### internal/doctor/doctor.go

Types and orchestrator:
- `Severity int` iota: `SeverityInfo(0)`, `SeverityWarning(1)`, `SeverityError(2)`, `SeverityCritical(3)` with `String()`
- `Family string` consts: `FamilyDeps`, `FamilyPerms`, `FamilyCoherence`, `FamilyOrphans`, `FamilySigning`, `FamilyAgent`, `FamilyBaseline`
- `FixDescriptor{Summary string, Fn func() error}`
- `Finding{Family, Severity, Title, Explanation, SuggestedFix string, Fix *FixDescriptor}`
- `CheckFn` type alias: `func(Deps) []Finding`
- `Deps` struct (see frozen field set below)
- `Run(Deps) []Finding` — dispatches through Deps.Check* fields
- `ExitCode([]Finding) int` — highest-wins, 0/1/2/3 tiers
- `Families() []Family` — fixed UI-SPEC order

### internal/doctor/checks/perms.go

Real implementation:
- `CheckPermissions(deps doctor.Deps) []doctor.Finding`
- Checks: `SSHDir` (0700/error), `KeyPaths` (0600/critical), `PubKeyPaths` (0644/warning), `SSHConfigPath`+`GitconfigPath` (0600/error)
- `os.ErrNotExist` paths silently skipped (coherence's concern)
- `Fix.Fn` closes over `deps.FixPerm` — no `os.Chmod` in `internal/doctor`
- gosec G304 annotated on every `Stat` call

### internal/doctor/checks/{deps,baseline,coherence,orphans,signing}.go

Compiling stubs — each contains exactly one `func Check<Family>(deps doctor.Deps) []doctor.Finding { _ = deps; return nil }`. Ready for Wave 2 overwrite.

### cmd/gitid/doctor.go

- `newDoctorCmd()` — Cobra command `Use:"doctor"`, registered on root
- `--fix`/`--yes` flags declared; `--yes requires --fix` guard
- `runDoctor(out io.Writer, fix, yes bool) int` — resolves home, reads configs, builds deps, runs, renders, returns exit code
- `renderReport(out, findings, colorEnabled)` — iterates `Families()` in fixed order, `=== Family ===` headers, ✓ pass lines, finding blocks
- `renderFinding(f, colorEnabled)` — glyph + title (severity-colored) + explanation + `fix:` (dim) + `[fix]` marker
- `buildDoctorDeps(home, sshBytes, gcBytes) doctor.Deps` — wires all 18 Deps fields from real packages; `FixPerm = os.Chmod` closure
- `ansi`, `severityCode`, `isTerminalOutput` helpers (D-08: NO_COLOR + ModeCharDevice)

---

## FROZEN doctor.Deps Field Set (Wave-2 Contract)

**WARNING: This is the locked contract that Plans 02, 03, 04, and 05 wire against.
Any change to this field set after this SUMMARY is published requires notifying all Wave-2 plans.**

### Read fields

| Field | Type | Purpose |
|-------|------|---------|
| `ReadFile` | `func(path string) ([]byte, error)` | Read a trusted gitid-managed file by path |
| `Stat` | `func(path string) (os.FileInfo, error)` | Stat a trusted gitid-managed path (perms, coherence) |

### Process fields

| Field | Type | Purpose |
|-------|------|---------|
| `RunSSHAdd` | `func() (string, int)` | Run `ssh-add -l`, return (output, exitCode) |
| `RunSSHKeygenFingerprint` | `func(path string) (string, error)` | Run `ssh-keygen -lf <path>`, return fingerprint line |
| `RunGitConfigGet` | `func(file, key string) (string, error)` | Run `git config --file <file> <key>` |

### Injected data and seams

| Field | Type | Purpose |
|-------|------|---------|
| `GitVersionAtLeast` | `func(major, minor int) bool` | Git version gate (D-20 hasconfig check) |
| `CurrentOS` | `func() string` | runtime.GOOS seam |
| `InstallHint` | `func(tool, os string) string` | Per-OS per-tool install hint |

### Path fields

| Field | Type | Purpose |
|-------|------|---------|
| `SSHDir` | `string` | Absolute path to `~/.ssh` |
| `SSHConfigPath` | `string` | Absolute path to `~/.ssh/config` |
| `GitconfigPath` | `string` | Absolute path to `~/.gitconfig` |
| `AllowedSignersPath` | `string` | Absolute path to `~/.ssh/allowed_signers` |
| `KeyPaths` | `[]string` | Gitid-managed private key paths (0600 targets) |
| `PubKeyPaths` | `[]string` | Gitid-managed .pub file paths (0644 targets) |

### Fix fields (D-01 — cmd layer only, doctor core never calls directly)

| Field | Type | Purpose |
|-------|------|---------|
| `FixPerm` | `func(path string, mode os.FileMode) error` | chmod to KEY-02 target (wired as `os.Chmod`) |
| `RemoveBlock` | `func(path, name string) error` | Remove a sentinel-delimited managed block |
| `AddWiring` | `func(path, name, line string) error` | Re-add a missing wiring line |

### Check function fields (wired by cmd layer, avoids doctor→checks cycle)

| Field | Type | Wired to |
|-------|------|----------|
| `CheckDeps` | `CheckFn` | `checks.CheckDeps` (STUB, Plan 02) |
| `CheckPerms` | `CheckFn` | `checks.CheckPermissions` (REAL, this plan) |
| `CheckCoherence` | `CheckFn` | `checks.CheckCoherence` (STUB, Plan 03) |
| `CheckOrphans` | `CheckFn` | `checks.CheckOrphans` (STUB, Plan 03) |
| `CheckSigning` | `CheckFn` | `checks.CheckSigning` (STUB, Plan 04) |
| `CheckAgent` | `CheckFn` | `checks.CheckAgent` (STUB, Plan 04) |
| `CheckBaseline` | `CheckFn` | `checks.CheckBaseline` (STUB, Plan 02) |

---

## Seven Per-Family File Map (Wave-2 Overwrite Targets)

| File | Plan that overwrites | Functions |
|------|---------------------|-----------|
| `internal/doctor/checks/deps.go` | Plan 02 | `CheckDeps` |
| `internal/doctor/checks/baseline.go` | Plan 02 | `CheckBaseline` |
| `internal/doctor/checks/coherence.go` | Plan 03 | `CheckCoherence` |
| `internal/doctor/checks/orphans.go` | Plan 03 | `CheckOrphans` |
| `internal/doctor/checks/signing.go` | Plan 04 | `CheckSigning`, `CheckAgent` |
| `internal/doctor/checks/perms.go` | (already real — Plan 01) | `CheckPermissions` |

**Wave 2 protocol:** Each plan overwrites its file(s) IN PLACE (same path, same exported
function signature). No function redeclaration occurs because each plan replaces the stub
body. `doctor.Run` and `cmd/gitid/doctor.go` do NOT need to change at any wave boundary.

---

## Deviations from Plan

### Architecture Decision: Import Cycle Resolution

**Found during:** Task 1 GREEN

**Issue:** The plan described `doctor.Run` calling `checks.CheckDeps` etc. directly, but `checks` package imports `doctor` (for `Deps`/`Finding` types), creating a circular import (`doctor` → `checks` → `doctor`).

**Fix:** Introduced `CheckFn` type alias in `doctor` package and added seven `Check*` function fields to `Deps`. The cmd layer (`buildDoctorDeps`) wires `checks.CheckPermissions` etc. into these fields. `doctor.Run` dispatches via field calls, never importing `checks`. This satisfies D-01 (write-free core), keeps all seven stable function names in `checks`, and allows the Phase 5 TUI to rewire check functions independently.

**Files modified:** `internal/doctor/doctor.go` (added `CheckFn` type + 7 Deps fields), `cmd/gitid/doctor.go` (wires fields in `buildDoctorDeps`)

### Architecture Decision: KeyPaths/PubKeyPaths vs identity.Account

**Found during:** Task 2 RED

**Issue:** The plan referenced iterating `identity.Account` structs for the perms check. Importing `internal/identity` from `internal/doctor` introduces a dependency on the identity package's full type set, which is heavier than needed for perms.

**Fix:** Added `KeyPaths []string` and `PubKeyPaths []string` fields to `doctor.Deps`. The cmd layer's `buildDoctorDeps` extracts these from the reconstructed identity list before calling `Run`. The core remains clean of identity imports.

**Files modified:** `internal/doctor/doctor.go` (added 2 fields), `internal/doctor/checks/perms_test.go` (uses new fields)

---

## Known Stubs

| File | Function | Reason |
|------|----------|--------|
| `internal/doctor/checks/deps.go` | `CheckDeps` | Returns nil — real implementation in Plan 02 |
| `internal/doctor/checks/baseline.go` | `CheckBaseline` | Returns nil — real implementation in Plan 02 |
| `internal/doctor/checks/coherence.go` | `CheckCoherence` | Returns nil — real implementation in Plan 03 |
| `internal/doctor/checks/orphans.go` | `CheckOrphans` | Returns nil — real implementation in Plan 03 |
| `internal/doctor/checks/signing.go` | `CheckSigning`, `CheckAgent` | Returns nil — real implementation in Plan 04 |

These stubs are intentional by plan design. The `gitid doctor` command is runnable and shows grouped output; the stub families show "all checks passed" until Wave 2 overwrites them.

---

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes beyond what is documented in the plan's threat model (T-04-01 through T-04-SC). All `os.Stat` and `os.ReadFile` calls carry gosec G304 annotations. The `os.Chmod` call in `buildDoctorDeps` carries gosec G306 annotation and is restricted to KEY-02 target modes only.

## Self-Check: PASSED

Files exist:
- `internal/doctor/doctor.go` ✓
- `internal/doctor/doctor_test.go` ✓
- `internal/doctor/checks/perms.go` ✓
- `internal/doctor/checks/perms_test.go` ✓
- `internal/doctor/checks/deps.go` ✓
- `internal/doctor/checks/baseline.go` ✓
- `internal/doctor/checks/coherence.go` ✓
- `internal/doctor/checks/orphans.go` ✓
- `internal/doctor/checks/signing.go` ✓
- `cmd/gitid/doctor.go` ✓
- `cmd/gitid/doctor_test.go` ✓

Commits verified:
- `9d84e7c` (RED 1) ✓
- `031e1fc` (GREEN 1) ✓
- `eae2822` (RED 2) ✓
- `a4b3795` (GREEN 2) ✓
- `f8b9ac8` (Task 3) ✓
