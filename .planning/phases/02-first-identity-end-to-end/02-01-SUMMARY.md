---
phase: 02-first-identity-end-to-end
plan: 01
subsystem: filewriter
tags: [safe-write, atomic, backup, chmod, managed-block, tdd]
requires: []
provides:
  - "filewriter.Write — atomic backup/temp/fsync/chmod/rename safe-write"
  - "filewriter.EnsureDir — explicit-mode directory creation (~/.ssh 0700)"
  - "filewriter.ReplaceBlock — idempotent sentinel managed-block scan/replace"
  - "filewriter.BeginPrefix / EndPrefix — managed-block sentinel constants"
affects:
  - sshconfig/writer
  - gitconfig/fragment
  - keygen
tech-stack:
  added: []
  patterns:
    - "stdlib safe-write recipe (os.CreateTemp -> Sync -> Chmod -> os.Rename); no google/renameio"
    - "idempotent sentinel managed-block splice (bounded line range, no whole-file regex)"
key-files:
  created:
    - internal/filewriter/filewriter.go
    - internal/filewriter/filewriter_test.go
    - internal/filewriter/block.go
    - internal/filewriter/block_test.go
  modified:
    - internal/filewriter/doc.go
  deleted:
    - internal/filewriter/filewriter_stub_test.go
decisions:
  - "Backups durably fsynced and chmod 0600 immediately after copy (T-02-01)"
  - "Restore-on-error: any failure after temp creation removes the temp and leaves the target untouched"
  - "ReplaceBlock trims trailing newlines in the body so a second identical write is byte-identical"
metrics:
  duration: 18 min
  completed: 2026-06-09
---

# Phase 2 Plan 01: filewriter (safe-write chokepoint) Summary

The single safe-write chokepoint for Phase 2: timestamped backup, atomic
temp -> fsync -> chmod -> rename, explicit-mode directory creation, and an
idempotent sentinel managed-block scan/replace — all built test-first with no
new dependency beyond the four pinned libs.

## What Was Built

- `internal/filewriter/filewriter.go`
  - `Write(targetPath, content, mode) (backupPath, err)` — backs up any existing
    target to `<target>.bak.<20060102-150405>` at mode 0600, then writes content
    to a unique `gitid-*.tmp` in the same dir, `Sync`s, `Close`s, `os.Chmod`s to
    the exact requested mode, and `os.Rename`s atomically into place. On any
    error after temp creation it removes the temp and leaves the original target
    intact. `backupPath` is empty when the target did not pre-exist.
  - `EnsureDir(dirPath, mode)` — `os.MkdirAll` + explicit `os.Chmod` (enforces
    `~/.ssh` 0700 without relying on umask).
  - `copyFile` helper — durable, mode-exact backup copy.
- `internal/filewriter/block.go`
  - `ReplaceBlock(existing, name, blockBody) []byte` — bounded line-range scan
    for `# BEGIN gitid managed: <name>` … `# END gitid managed: <name>`; replaces
    only that range (or appends when absent); foreign content byte-identical.
  - Exported sentinel constants `BeginPrefix` / `EndPrefix`.

## How It Was Verified

- `go test ./internal/filewriter/... -race` — green (filewriter 66.2% coverage).
- `make test` (full module, race + coverage) — all packages green.
- `make lint` (golangci-lint + gosec) — 0 issues.
- `grep -v '^//' internal/filewriter/filewriter.go | grep -c 'os.WriteFile'` — 0.

### TDD Gate Compliance

Both tasks followed the RED -> GREEN sequence. A compiling not-implemented stub
(panic) was committed in each RED step so the failing-test commit passes the
project's `make lint` pre-commit hook (lint compiles the whole module; a
test-only commit referencing undefined symbols cannot lint). RED was verified to
fail at runtime (panic: not implemented) before each GREEN implementation.

- Task 1 RED: `test(02-01): add failing safe-write tests …` (21cab3c)
- Task 1 GREEN: `feat(02-01): implement filewriter.Write …` (16588cd)
- Task 2 RED: `test(02-01): add failing ReplaceBlock …` (0b3e02e)
- Task 2 GREEN: `feat(02-01): implement idempotent ReplaceBlock …` (0b1e802)

## Requirements Satisfied

- SAFE-01: existing target backed up to `<target>.bak.<ts>` (0600) before overwrite — asserted by `TestWriteBacksUpExistingTarget`.
- SAFE-02: `ReplaceBlock` idempotent (`out1 == out2`); foreign content byte-identical — asserted by `TestReplaceBlockIdempotent` / `TestReplaceBlockPreservesForeignContent`.
- SAFE-03: atomic temp -> rename; restore-on-error leaves target intact — asserted by `TestWriteRestoreOnError`.
- KEY-02: exact modes 0600/0644 and `~/.ssh` 0700 via explicit chmod — asserted by `TestWriteCreatesNewTargetWithExactMode` / `TestEnsureDir`.

## Threat Mitigations Implemented

| Threat ID | Mitigation |
|-----------|------------|
| T-02-01 | backup `os.Chmod` 0600 immediately after copy |
| T-02-02 | atomic temp -> fsync -> rename; restore-on-error path |
| T-02-03 | explicit `os.Chmod(target, mode)` after rename; never umask |
| T-02-04 | `os.CreateTemp(dir, "gitid-*.tmp")` unique name, no fixed `.tmp` |
| T-02-05 | gosec G304/G306 documented `//nolint` with trust rationale; modes explicit |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] RED commit must pass `make lint` pre-commit hook**
- **Found during:** Task 1 (and again Task 2)
- **Issue:** The project's `make lint` pre-commit hook compiles the whole module. A pure test-only RED commit references undefined `Write`/`EnsureDir`/`ReplaceBlock` and fails to compile, so the hook blocks the commit. `--no-verify` is prohibited by the run contract.
- **Fix:** Each RED commit ships a compiling not-implemented stub (`panic("… not implemented")`) alongside the failing test. The stub lints clean and the test runs and fails at runtime — preserving a genuine RED gate while satisfying the hook.
- **Files modified:** internal/filewriter/filewriter.go, internal/filewriter/block.go
- **Commits:** 21cab3c, 0b3e02e

**2. [Rule 3 - Blocking] gosec G301/G302 on test directory chmod**
- **Found during:** Task 1 (pre-commit lint hook blocked the RED commit)
- **Issue:** The restore-on-error test chmods a fixture dir to 0500/0700 and seeds a 0755 dir; gosec flags directory modes > 0600/0750.
- **Fix:** Added targeted `//nolint:gosec` with a justification comment on each test-fixture chmod/mkdir (these intentionally exercise the 0700 dir contract and the forced-failure path).
- **Files modified:** internal/filewriter/filewriter_test.go
- **Commit:** folded into 21cab3c

### Doc update (not a deviation)

`internal/filewriter/doc.go` "Implementation lands in a later phase" line replaced with a one-line description of the now-implemented API (commit 8e7137f).

## Known Stubs

None. The package is fully implemented; the only `panic("not implemented")` placeholders were transient RED-commit stubs, both replaced in their GREEN commits.

## Self-Check: PASSED

- FOUND: internal/filewriter/filewriter.go
- FOUND: internal/filewriter/filewriter_test.go
- FOUND: internal/filewriter/block.go
- FOUND: internal/filewriter/block_test.go
- FOUND commit 21cab3c (Task 1 RED)
- FOUND commit 16588cd (Task 1 GREEN)
- FOUND commit 0b3e02e (Task 2 RED)
- FOUND commit 0b1e802 (Task 2 GREEN)
- FOUND commit 8e7137f (doc.go)
