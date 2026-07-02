---
phase: quick-260609-qd6
plan: "01"
subsystem: cmd/gitid
tags: [security, tdd, input-validation, T-02-23]
dependency_graph:
  requires: []
  provides: [name-charset-validation-add-go]
  affects: [cmd/gitid/add.go, cmd/gitid/add_test.go]
tech_stack:
  added: []
  patterns: [reuse-package-var, sanitize-then-validate]
key_files:
  created: []
  modified:
    - cmd/gitid/add.go
    - cmd/gitid/add_test.go
decisions:
  - "Reuse rotate.go's identityNameRe + sanitizeName (same package main scope) — zero duplication, zero new regexp.MustCompile"
  - "Apply sanitizeName before the empty-name check so trimmed input participates in the same guard chain as rotate"
  - "Error message mirrors rotate.go's wording exactly: 'invalid identity name %q (allowed: letters, digits, ., _, -)'"
metrics:
  duration: "~8 min"
  completed: "2026-06-09T23:03:22Z"
  tasks_completed: 2
  files_modified: 2
---

# Phase quick-260609-qd6 Plan 01: Fix Security Gap T-02-23 — Apply identityNameRe to add.go Gatherers Summary

**One-liner:** Identity name charset allowlist (`^[A-Za-z0-9._-]+$`) applied to `gatherCreateInput` and `gatherAddAccount` via `rotate.go`'s existing `identityNameRe` + `sanitizeName`, closing the path-escape vector T-02-23.

## What Was Built

Closed security gap T-02-23 (Tampering/Elevation — command/path injection via user-entered identity name) on the two interactive-input paths in `cmd/gitid/add.go`:

- **`gatherCreateInput`** (add.go ~265): unsafe names like `../evil`, `a/b`, `foo bar`, `name;rm` now return `fmt.Errorf("identity add: invalid identity name %q ...")` before the name ever reaches `filepath.Join(home, ".gitconfig.d", name)` or key paths.
- **`gatherAddAccount`** (add.go ~216): identical guard applied to the existing identity name input.

Both guards call `sanitizeName` (trim whitespace) then `identityNameRe.MatchString` — the same two symbols already declared in `rotate.go`. No new regex literal, no duplication.

## TDD Gate Compliance

| Gate | Commit | Status |
|------|--------|--------|
| RED (`test(...)`) | `6953a54` | Passed — both tests compiled, lint clean, FAILED at runtime (unsafe names accepted) |
| GREEN (`fix(...)`) | `6711bb1` | Passed — `make lint` + `make test -race` fully green |

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Table-driven failing tests for name validation | `6953a54` | cmd/gitid/add_test.go |
| 2 (GREEN) | Apply identityNameRe + sanitizeName to both gatherers | `6711bb1` | cmd/gitid/add.go |

## Final Verification

```
make lint  → 0 issues (golangci-lint v2 clean)
make test  → ok github.com/castocolina/gitid/cmd/gitid  (all tests pass, -race)
grep -n "regexp.MustCompile" cmd/gitid/add.go → (empty — no duplication)
git diff --stat HEAD~2 HEAD -- cmd/gitid/rotate.go → (empty — rotate.go untouched)
```

New tests pass:
- `TestGatherCreateInputRejectsUnsafeName` — 4 reject cases + 3 accept cases all GREEN
- `TestGatherAddAccountRejectsUnsafeName` — 4 reject cases + 3 accept cases all GREEN

## Deviations from Plan

None — plan executed exactly as written.

## Threat Flags

None — this plan only adds a guard to existing code paths; no new network endpoints, auth paths, file-access patterns, or schema changes introduced.

## Known Stubs

None.

## Self-Check: PASSED

- `cmd/gitid/add.go` modified and committed at `6711bb1` — verified
- `cmd/gitid/add_test.go` modified and committed at `6953a54` — verified
- `rotate.go` untouched — verified via `git diff`
- No `regexp.MustCompile` in `add.go` — verified via grep
- `make lint` passes — verified
- `make test` passes — verified
