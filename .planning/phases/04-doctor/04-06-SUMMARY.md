---
phase: 04-doctor
plan: "06"
subsystem: doctor
tags: [go, doctor, auto-fix, filewriter, ssh, gitconfig, TDD]

# Dependency graph
requires:
  - phase: 04-doctor
    provides: buildDoctorDeps wiring, filewriter chokepoint, RemoveBlock/AddWiring closures

provides:
  - Real RemoveBlock Fix.Fn in orphans check (deps.RemoveBlock calling filewriter)
  - Real AddWiring Fix.Fn in coherence check (IdentitiesOnly + allowed_signers)
  - Real AddWiring Fix.Fn in baseline check (baseline-include restore)
  - Path-derived mode in RemoveBlock closure (0644 for allowed_signers, 0600 otherwise)
  - All-candidate findSignerLine scan (WR-01: exact match wins over earlier case-fold match)
  - Integration tests driving real buildDoctorDeps wiring with on-disk assertions

affects: [04-07, 05-doctor-tui]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - buildSignersFix helper reads pub key via deps.ReadFile then constructs signers: payload
    - Fix guards: nil check on deps.RemoveBlock/AddWiring before creating FixDescriptor (avoids panic on nil call)
    - Orphan class 1/2: incompleteNames guard removed — single-sided managed block IS an orphan

key-files:
  created:
    - cmd/gitid/doctor_realwiring_test.go
  modified:
    - internal/doctor/checks/orphans.go
    - internal/doctor/checks/coherence.go
    - internal/doctor/checks/baseline.go
    - cmd/gitid/doctor.go
    - internal/doctor/checks/orphans_test.go
    - internal/doctor/checks/coherence_test.go
    - internal/doctor/checks/baseline_test.go

key-decisions:
  - "incompleteNames guard removed from orphan Classes 1+2: a single-sided managed block is both Incomplete (for coherence) and Orphan (for removal) — the guard prevented any orphan finding from ever triggering"
  - "curated-gitignore restore is Fix=nil (report-only): no safe single-call restore exists via the AddWiring string-payload protocol — patterns list cannot be encoded; user must run gitid baseline setup"
  - "buildSignersFix reads pub key via deps.ReadFile(acct.PubPath) and returns nil Fix when pub key is unreadable (graceful degradation, D-03)"
  - "WR-01 fix: findSignerLine now uses a two-pass scan — records first case-fold match but continues looking for an exact match; exact match wins unconditionally"
  - "WR-02 fix: RemoveBlock closure derives file mode from path comparison (allowedSignersPath → 0644, else 0600)"

patterns-established:
  - "Pattern: nil-guard Fix construction — check deps.RemoveBlock/deps.AddWiring != nil before building FixDescriptor.Fn to prevent nil-dereference panics in unit tests with partial deps"
  - "Pattern: real-wiring integration tests — TestDoctorRealWiringN tests use buildDoctorDeps against t.TempDir() and assert on-disk effects, never fake seam call counts"

requirements-completed: [DOC-04, DOC-06]

# Metrics
duration: 90min
completed: 2026-06-12
---

# Phase 04 Plan 06: Doctor Real-Wiring Gap Closure Summary

**Real RemoveBlock/AddWiring closures wired through doctor check Fix.Fn fields, closing DOC-GAP-01: gitid doctor --fix now actually removes orphaned managed blocks and re-adds missing allowed_signers/IdentitiesOnly/baseline-include wiring on disk**

## Performance

- **Duration:** ~90 min
- **Started:** 2026-06-12T10:30:00Z
- **Completed:** 2026-06-12T12:20:00Z
- **Tasks:** 2 (RED + GREEN, TDD)
- **Files modified:** 7 (+ 1 created)

## Accomplishments

- Closed DOC-GAP-01: `gitid doctor --fix` now actually mutates files on disk for orphan/coherence/baseline findings instead of silently no-oping
- Fixed WR-01 (findSignerLine false positive): all-candidate scan ensures an exact email match later in allowed_signers wins over a case-differing earlier line
- Fixed WR-02 (RemoveBlock mode): allowed_signers removal now uses 0644 mode (not 0600), preserving the public readable permission
- Removed design bug in orphan check: `incompleteNames` guard prevented Class 1/2 from ever triggering; now single-sided managed blocks are correctly reported as orphans
- Curated-gitignore finding upgraded from no-op stub to explicit `Fix: nil` (report-only) per plan advisory

## Task Commits

1. **Task 1: RED — integration tests** - `9237a5f` (test)
2. **Task 2: GREEN — plumb real wiring** - `43d723e` (feat)

## Files Created/Modified

- `cmd/gitid/doctor_realwiring_test.go` - Four integration tests driving real buildDoctorDeps wiring with on-disk assertions
- `internal/doctor/checks/orphans.go` - Class 1/2 Fix.Fn calls deps.RemoveBlock; incompleteNames guard removed; nil-guard on RemoveBlock
- `internal/doctor/checks/coherence.go` - IdentitiesOnly Fix.Fn calls deps.AddWiring(ssh-host:); allowed_signers Fix.Fn calls deps.AddWiring(signers:); findSignerLine WR-01 all-candidate scan
- `internal/doctor/checks/baseline.go` - baseline-include Fix.Fn calls deps.AddWiring(baseline-include:); curated-gitignore Fix changed from no-op stub to nil
- `cmd/gitid/doctor.go` - RemoveBlock closure: path-derived mode (WR-02); no other changes
- `internal/doctor/checks/orphans_test.go` - Added RemoveBlock/SSHConfigPath to test deps requiring Fix != nil
- `internal/doctor/checks/coherence_test.go` - Added AddWiring/SSHConfigPath to test deps requiring Fix != nil
- `internal/doctor/checks/baseline_test.go` - Added AddWiring to fakeBaselineDeps; curated excludes test expects Fix=nil

## Decisions Made

1. **incompleteNames guard removal (Rule 1 - Bug):** The existing guard in CheckOrphans prevented Classes 1 and 2 from ever producing a finding, because any SSH-only or gitconfig-only managed block is also marked Incomplete by Reconstruct. Coherence and Orphans are orthogonal: Coherence covers the incomplete-wiring angle, Orphans covers the removable-block angle. Both can apply to the same identity. Guard removed.

2. **curated-gitignore is Fix=nil (plan advisory):** The `AddWiring` dispatcher uses a string payload protocol that cannot encode a patterns list. No safe single-call restore exists. Per plan advisory: "only fall back to Fix=nil (never a no-op stub) if not." Fix=nil is the correct choice.

3. **buildSignersFix nil-guard:** If `deps.AddWiring` or `deps.ReadFile` is nil, or `acct.PubPath` is empty, `buildSignersFix` returns nil. This prevents panics in unit tests with partial deps and gracefully degrades the coherence finding to report-only.

4. **WR-01 two-pass scan:** findSignerLine now records the first case-fold match but continues scanning. An exact byte-match anywhere in the file returns immediately and wins. Only after exhausting all lines does the case-fold match win (if no exact match found). This prevents an earlier case-differing line from masking a correct later entry.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed incompleteNames guard from orphan Classes 1+2**
- **Found during:** Task 1 (RED test writing) and Task 2 (GREEN implementation)
- **Issue:** The existing `incompleteNames` guard in CheckOrphans caused Class 1 and Class 2 orphan findings to never fire: any SSH-only or gitconfig-only managed block is also Incomplete per Reconstruct. The test seeded an SSH-only block expecting an orphan finding, but got none.
- **Fix:** Removed `incompleteNames` guard from Classes 1 and 2. The guard was incorrect — Coherence and Orphans report different aspects of the same issue. Added nil-guard on deps.RemoveBlock instead.
- **Files modified:** `internal/doctor/checks/orphans.go`
- **Verification:** `TestDoctorRealWiring1_OrphanBlockRemoval` passes; `TestOrphanNotIncomplete` still passes (no regression — that test has both SSH and gitconfig blocks so no orphan fires)
- **Committed in:** `43d723e` (Task 2 commit)

**2. [Rule 1 - Bug] Updated unit tests expecting no-op Fix stubs**
- **Found during:** Task 2 (GREEN) — existing unit tests in baseline_test.go and coherence_test.go expected Fix != nil but the test deps lacked AddWiring/RemoveBlock, causing buildSignersFix to return nil
- **Fix:** Added AddWiring/RemoveBlock/SSHConfigPath to fake deps; updated curated-gitignore test to expect Fix=nil (now report-only, not fixable)
- **Files modified:** `internal/doctor/checks/baseline_test.go`, `internal/doctor/checks/coherence_test.go`, `internal/doctor/checks/orphans_test.go`
- **Committed in:** `43d723e` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 - Bug)
**Impact on plan:** Both fixes necessary for correctness. The incompleteNames removal is a design correction that makes orphan detection actually work. No scope creep.

## Known Stubs

None. All previously no-op `func() error { return nil }` stubs have been replaced:
- Orphan Classes 1+2: real `deps.RemoveBlock` calls
- Coherence IdentitiesOnly: real `deps.AddWiring(ssh-host:...)` call
- Coherence allowed_signers missing/mismatch: real `deps.AddWiring(signers:...)` call
- Baseline include-missing: real `deps.AddWiring(baseline-include:...)` call
- Curated-gitignore: changed to `Fix: nil` (report-only, not a stub)

`grep -rn 'Fn:.*func() error { return nil }' internal/doctor/checks/` returns empty.

## Threat Flags

No new security-relevant surface introduced. The existing T-04-16/T-04-17/T-04-19/T-04-21 mitigations all hold:
- T-04-16: Only sentinel-delimited managed blocks are mutated; foreign content preserved
- T-04-19: WR-02 fix ensures allowed_signers stays 0644 after RemoveBlock
- T-04-21: No filewriter import in internal/doctor; D-01 preserved

## Self-Check: PASSED

Files verified:
- `cmd/gitid/doctor_realwiring_test.go`: FOUND
- `internal/doctor/checks/orphans.go`: FOUND (contains deps.RemoveBlock)
- `internal/doctor/checks/coherence.go`: FOUND (contains deps.AddWiring)
- `internal/doctor/checks/baseline.go`: FOUND (contains deps.AddWiring)
- `cmd/gitid/doctor.go`: FOUND (path-derived mode)

Commits verified:
- `9237a5f`: FOUND (test(04-06): failing real-wiring integration tests)
- `43d723e`: FOUND (feat(04-06): plumb real RemoveBlock/AddWiring)

Success criteria verification:
- `go test ./cmd/gitid/... -run TestDoctorRealWiring`: ALL PASS (4/4)
- `grep -rn 'func() error { return nil }' internal/doctor/checks/`: EMPTY (no surviving stubs)
- `grep -rn '"github.com/castocolina/gitid/internal/filewriter"' internal/doctor/`: EMPTY (D-01 preserved)
- `go build ./...`: EXIT 0
- `go test ./...`: ALL PASS
- `make lint`: 0 ISSUES

---
*Phase: 04-doctor*
*Completed: 2026-06-12*
