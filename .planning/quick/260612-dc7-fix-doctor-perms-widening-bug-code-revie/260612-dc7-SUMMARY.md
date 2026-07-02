---
phase: quick-260612-dc7
plan: 01
subsystem: doctor/checks
tags: [bug-fix, security, perms, tdd]
dependency_graph:
  requires: []
  provides: [tighten-only-checkPath, no-widening-FixPerm-contract]
  affects: [internal/doctor/checks/perms.go, cmd/gitid/doctor.go]
tech_stack:
  added: []
  patterns: [tighten-only bitmask predicate (got &^ want), safe fix target (got & want)]
key_files:
  modified:
    - internal/doctor/checks/perms.go
    - internal/doctor/checks/perms_test.go
    - cmd/gitid/doctor.go
    - .planning/phases/04-doctor/04-SECURITY.md
decisions:
  - "Tighten-only predicate (got &^ want != 0) replaces exact-equality check — stricter-than-target modes (e.g. 0400 private key) no longer produce false findings"
  - "Fix mode is got & want (safe target) rather than fixed want — chmod never adds a bit the file lacked"
  - "TestCheckPermsPubWarning updated from 0600 (too restrictive — not caught by tighten-only) to 0666 (genuinely loose — correctly caught); 0600 pub key is not flagged by design"
metrics:
  duration: 15min
  completed: 2026-06-12
  tasks_completed: 2
  files_modified: 4
---

# Quick Task 260612-dc7: Fix Doctor Perms Widening Bug

**One-liner:** Tighten-only `checkPath` using `got &^ want` flag predicate and `got & want` fix target, closing the false-positive 0400-key finding and the permission-widening bug found in Phase-4 code review.

## What Was Fixed

`checkPath` in `internal/doctor/checks/perms.go` used an exact-equality predicate (`got == want`) that:

1. **Falsely flagged** private keys hardened to 0400 (stricter than the 0600 target) — 0400 != 0600, so a finding was raised.
2. **Widened permissions** via the fix: the closure called `deps.FixPerm(p, want)` using the fixed `want` constant (0600), which adds the owner-write bit to a 0400 key — contradicting the "never widens" intent.

## Fix Applied

### Task 1 (TDD RED then GREEN): `internal/doctor/checks/perms.go`

Replaced the exact-equality predicate with a tighten-only guard:

```go
// Before:
if got == want {
    return nil
}
fix := fmt.Sprintf("chmod %04o %s", want, path)
p, m := path, want
deps.FixPerm(p, m)

// After:
if got&^want == 0 {
    return nil  // at target or already stricter — no loosening bits
}
safe := got & want
fix := fmt.Sprintf("chmod %04o %s", safe, path)
p, s := path, safe
deps.FixPerm(p, s)
```

**Manual reasoning verified:**
- `0400 &^ 0600 == 0` → no finding (0400 key not flagged)
- `0644 &^ 0600 == 0o044 != 0` → finding raised; `0644 & 0600 == 0600` → fix tightens to 0600, no widen
- `0755 &^ 0700 == 0o055 != 0` → finding raised; `0755 & 0700 == 0700` → fix tightens to 0700

`checkGitconfigPath` (WR-03 write-mask guard) was left unchanged.

### Task 2: `cmd/gitid/doctor.go` and `04-SECURITY.md`

- Updated FixPerm comment: replaced the misleading "never widens" claim on the closure itself with an accurate description that the guarantee is enforced upstream in `checks/perms.go` via the `got &^ want` predicate and `got & want` fix mode.
- Updated T-04-02 and T-04-19 evidence in `04-SECURITY.md` to reference the tighten-only predicate, `got & want` fix target, and the closed over-tightened (0400 key) edge case.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestCheckPermsPubWarning test used wrong mode for new contract**

- **Found during:** Task 1 GREEN phase — test failed after implementing the tighten-only predicate
- **Issue:** `TestCheckPermsPubWarning` set a `.pub` file at 0600 (too restrictive — stricter than the 0644 target). Under the old exact-equality predicate this was flagged; under the new tighten-only predicate `0600 &^ 0644 == 0` → no finding (correct: a pub key that is stricter than 0644 poses no security risk). The test expectation conflicted with the new contract.
- **Fix:** Changed the test to use mode 0666 (genuinely loose: has group-write + world-write bits that 0644 lacks; `0666 &^ 0644 == 0o022 != 0` → correctly flagged). Added a comment explaining why 0600 is not flagged by design.
- **Files modified:** `internal/doctor/checks/perms_test.go`
- **Commit:** 860636d

## TDD Gate Compliance

| Gate | Commit | Message |
|------|--------|---------|
| RED | 8cb095e | test(260612-dc7): add RED tests for tighten-only checkPath behavior |
| GREEN | 860636d | fix(260612-dc7): make checkPath tighten-only (got &^ want guard + got&want fix mode) |

RED gate: `TestCheckPermsStricterKeyNotFlagged` failed against the old code, confirming the predicate change was necessary. `TestCheckPermsLooseKeyTightensNotWidens` passed even against the old code (0644 & 0600 == 0600 = want, so the FixPerm call was already correct for that case).

## Verification

- `go build ./...` — passed
- `go test ./...` — 13 packages, all passed
- `make lint` — 0 issues (gosec G306 annotation updated to reflect caller-supplied tighten-only mode)

## Known Stubs

None.

## Threat Flags

None — this fix closes existing threat surface (T-04-02, T-04-19); no new surface introduced.

## Self-Check: PASSED

Files created/modified:
- `internal/doctor/checks/perms.go` — exists, contains `got &^ want`
- `internal/doctor/checks/perms_test.go` — exists, contains `TestCheckPermsStricterKeyNotFlagged`
- `cmd/gitid/doctor.go` — exists, contains `caller-supplied`
- `.planning/phases/04-doctor/04-SECURITY.md` — exists, contains `got & want`

Commits:
- 8cb095e (RED test)
- 860636d (GREEN fix)
- 34f15c2 (comment + security doc)
