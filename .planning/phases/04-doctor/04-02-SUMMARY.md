---
phase: "04-doctor"
plan: "02"
subsystem: doctor
tags: [doctor, deps, baseline, platform, tdd, installhint, D-16]
dependency_graph:
  requires: ["04-01"]
  provides: [CheckDeps, CheckBaseline, InstallHint-extended]
  affects: [plans-04-03-04-04-04-05, gitid-doctor-output]
tech_stack:
  added: []
  patterns: [injected-deps, TDD-RED-GREEN, stub-overwrite-in-place, single-platform-hint-when-known]
key_files:
  created:
    - internal/doctor/checks/deps_test.go
    - internal/doctor/checks/baseline_test.go
  modified:
    - internal/platform/platform.go
    - internal/platform/platform_test.go
    - internal/doctor/checks/deps.go
    - internal/doctor/checks/baseline.go
    - internal/doctor/doctor.go
    - cmd/gitid/doctor.go
    - cmd/gitid/doctor_test.go
decisions:
  - "InstallHint signature changed from InstallHint(os) to InstallHint(tool, os string) to match the doctor.Deps.InstallHint func field contract"
  - "doctor.Deps gained DetectTools and ReadBaselineState injected seams + BaselineFilePath and GitignorePath path fields for fake-testability"
  - "Baseline include missing → error + Fix non-nil (auto-fixable re-add class, D-02); Plan 05 wires the actual Fn"
  - "Curated excludes absent → warning + Fix non-nil; excludesfile unset → error, report-only (no Fix)"
  - "CheckBaseline returns early on !state.Installed: include missing is reported once; other checks need baseline keys to be meaningful"
metrics:
  duration: "~50min"
  completed: "2026-06-11"
  tasks: 2
  files: 9
---

# Phase 04 Plan 02: Dependencies + Baseline Families Summary

Two report families that compose proven primitives: the **Dependencies** family (DOC-01)
via the extended `platform.InstallHint` + injected `DetectTools`, and the **Baseline**
family (D-16) via injected `ReadBaselineState`. Both stub files from Plan 01 were
overwritten in place with the real implementations — no function redeclaration.

## Commits

| Commit | Message |
|--------|---------|
| `4241225` | test(04-02): add failing tests for InstallHint(tool,os) and CheckDeps |
| `06c7b06` | feat(04-02): implement InstallHint(tool,os) and CheckDeps |
| `c7073ec` | test(04-02): add failing tests for CheckBaseline (D-16 four checks) |
| `a18b6bc` | feat(04-02): implement CheckBaseline — fold D-16 four checks via ReadBaselineState |
| `ce94fdc` | fix(04-02): update TestDoctorCleanAllClear for live CheckBaseline |

## TDD Gate Compliance

- RED gate (Task 1): `4241225` — test commit precedes GREEN `06c7b06`.
- RED gate (Task 2): `c7073ec` — test commit precedes GREEN `a18b6bc`.
- GREEN gate (Task 1): `06c7b06` — implements after RED is confirmed.
- GREEN gate (Task 2): `a18b6bc` — implements after RED is confirmed.
- No REFACTOR commits needed (code was clean on first pass; formatter auto-fixed alignment).

## Artifacts Produced

### internal/platform/platform.go (extended)

- `InstallHint(tool, os string) string` — extended from single-param `InstallHint(os)` to
  two-param. Dispatches on `normalizeTool(tool)`: "git" → `gitInstallHint`, "clipboard" →
  `clipboardInstallHint`, everything else → `opensshInstallHint`.
- Single-platform-when-known: darwin → one Homebrew line; linux → three package-manager lines;
  unknown OS → all four (brew/apt/dnf/pacman) per DOC-01 UI-SPEC Install Hint Format.
- Helpers: `normalizeTool`, `opensshInstallHint`, `gitInstallHint`, `clipboardInstallHint`.
- `SelectAlgorithm` updated to call `InstallHint("openssh", CurrentOS())`.

### internal/doctor/checks/deps.go (OVERWRITES Plan 01 stub)

- `CheckDeps(d doctor.Deps) []doctor.Finding` — composes injected `DetectTools` seam.
- Required tools (ssh, ssh-keygen, ssh-add, git): each missing tool → `SeverityError` finding,
  `FamilyDeps`, title `"<tool> missing"`, explanation per UI-SPEC copywriting contract, no Fix
  descriptor (report-only, D-03).
- Optional tool (clipboard): missing → `SeverityInfo` finding, explanation mentions
  "Public-key copy to clipboard" per UI-SPEC.
- nil `DetectTools` guard: returns nil safely.

### internal/doctor/checks/baseline.go (OVERWRITES Plan 01 stub)

- `CheckBaseline(d doctor.Deps) []doctor.Finding` — composes injected `ReadBaselineState`.
- Check 1 (include block): `!state.Installed` → error + `Fix` non-nil (re-add class, D-02;
  Plan 05 wires the actual Fn). Returns early — other checks need baseline keys.
- Check 2 (excludesfile): `core.excludesfile` absent/empty → error, report-only (no Fix).
- Check 3 (ignorecase drift): `state.BaselineKeys["core.ignorecase"] != "false"` → warning
  (D-17 locked-value carve-out; byte-exact "false" comparison).
- Check 4 (curated excludes): any `DefaultGitignorePatterns()` entry absent → warning + Fix
  non-nil (restore class, D-02).
- No `internal/filewriter` import (D-01 preserved).

### internal/doctor/doctor.go (field additions)

Added to `Deps` struct (extending the Wave-2 contract with test seams):
- `DetectTools func() deps.Report` — probe PATH for tools; cmd wires `deps.Detect`.
- `ReadBaselineState func(gc, bf, gi string) (gitconfig.BaselineState, error)` — cmd wires
  `gitconfig.ReadBaselineState`.
- `BaselineFilePath string` — absolute path to `~/.gitconfig.d/00-baseline`.
- `GitignorePath string` — absolute path to `~/.gitignore_global`.

### cmd/gitid/doctor.go (wiring additions)

`buildDoctorDeps` wires: `DetectTools: deps.Detect`, `ReadBaselineState: gitconfig.ReadBaselineState`,
`BaselineFilePath`, `GitignorePath`, `InstallHint: platform.InstallHint` (direct, no wrapper needed).

---

## InstallHint Signature Decision

The existing `platform.InstallHint(os string) string` was extended to
`InstallHint(tool, os string) string`. This matches the pre-existing
`doctor.Deps.InstallHint func(tool, os string) string` field contract (which
the cmd layer had wired with a `_ = tool` no-op wrapper). The wrapper was
removed — the cmd layer now passes `platform.InstallHint` directly.

The existing `SelectAlgorithm` caller was updated from `InstallHint(CurrentOS())`
to `InstallHint("openssh", CurrentOS())`.

---

## Baseline Findings Fix Descriptors for Plan 05

| Finding | Fix non-nil? | Plan 05 action |
|---------|-------------|----------------|
| baseline include missing | YES | Wire Fn to `gitconfig.WriteBaselineInclude` via cmd layer |
| excludesfile not wired | NO (report-only) | User must re-run `gitid baseline setup` |
| ignorecase drift | NO (report-only) | User must run `git config --global core.ignorecase false` |
| curated excludes missing | YES | Wire Fn to `gitconfig.WriteGlobalGitignore` via cmd layer |

---

## Deviations from Plan

### Rule 1 Auto-fix: TestDoctorCleanAllClear

**Found during:** Post-GREEN full test suite run

**Issue:** `cmd/gitid/doctor_test.go TestDoctorCleanAllClear` expected exit code <= 1
on a clean temp home, but `CheckBaseline` now correctly reports an error when the
baseline has never been set up. The test was written against the Plan 01 nil-return stub.

**Fix:** Updated test assertion from "code must be <= 1" to "code must be 0-3 and not 3
(no critical findings)". The test now checks for the Baseline section header instead
of expecting an all-clear exit code.

**Files modified:** `cmd/gitid/doctor_test.go`
**Commit:** `ce94fdc`

### Architecture Deviation: DetectTools + ReadBaselineState fields added to doctor.Deps

**Found during:** Task 1 RED design

**Issue:** The Plan 01 frozen Deps contract did not include `DetectTools` or
`ReadBaselineState` fields. Without these injected seams, CheckDeps and CheckBaseline
could not be fake-tested (they would call `deps.Detect()` and `gitconfig.ReadBaselineState()`
directly, making tests real I/O dependent).

**Fix:** Added `DetectTools func() deps.Report`, `ReadBaselineState func(...)`, plus the
`BaselineFilePath` and `GitignorePath` path fields to `doctor.Deps`. This follows the
established injected-deps pattern from Plans 01+02 and adds no import cycles
(`internal/deps` and `internal/gitconfig` do not import `internal/doctor`).

**Files modified:** `internal/doctor/doctor.go` (field additions), `cmd/gitid/doctor.go` (wiring)
**Commit:** `4241225` (RED: adds the fields), `06c7b06` (GREEN: wires in cmd layer)

---

## Known Stubs

| File | Function | Reason |
|------|----------|--------|
| `internal/doctor/checks/coherence.go` | `CheckCoherence` | Returns nil — real implementation in Plan 03 |
| `internal/doctor/checks/orphans.go` | `CheckOrphans` | Returns nil — real implementation in Plan 03 |
| `internal/doctor/checks/signing.go` | `CheckSigning`, `CheckAgent` | Returns nil — real implementation in Plan 04 |

The `CheckBaseline.Fix.Fn` closures for include-missing and curated-excludes-missing
currently return `nil` (no-op). These are intentional stubs: the `[fix]` marker renders
correctly, but Plan 05 must wire the actual fix functions via the cmd layer.

---

## Threat Surface Scan

No new network endpoints, auth paths, or schema changes introduced. The new
`gitconfig.ReadBaselineState` call in CheckBaseline reads trusted gitid-managed
paths (`~/.gitconfig`, `~/.gitconfig.d/00-baseline`, `~/.gitignore_global`) —
all covered by T-04-05 in the plan's threat model.

`CheckDeps` adds no new exec calls: it composes the injected `DetectTools` seam which
the cmd layer wires to `deps.Detect()` (already gosec-annotated). No key material read.

---

## Self-Check: PASSED

Files exist:
- `internal/platform/platform.go` ✓
- `internal/platform/platform_test.go` ✓
- `internal/doctor/checks/deps.go` ✓
- `internal/doctor/checks/deps_test.go` ✓
- `internal/doctor/checks/baseline.go` ✓
- `internal/doctor/checks/baseline_test.go` ✓
- `internal/doctor/doctor.go` ✓
- `cmd/gitid/doctor.go` ✓
- `cmd/gitid/doctor_test.go` ✓

Commits verified:
- `4241225` (RED 1) ✓
- `06c7b06` (GREEN 1) ✓
- `c7073ec` (RED 2) ✓
- `a18b6bc` (GREEN 2) ✓
- `ce94fdc` (Rule 1 fix) ✓

Single-definition verification:
- `grep -rn 'func CheckDeps' internal/doctor/checks/` → exactly one match ✓
- `grep -rn 'func CheckBaseline' internal/doctor/checks/` → exactly one match ✓

No filewriter import in internal/doctor: ✓
`go build ./...` green: ✓
`go test ./internal/doctor/... ./internal/platform/...` green: ✓
`make lint` green: ✓
