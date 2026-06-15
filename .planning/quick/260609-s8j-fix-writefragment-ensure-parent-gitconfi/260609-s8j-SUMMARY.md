---
quick_id: 260609-s8j
slug: fix-writefragment-ensure-parent-gitconfig-dir
status: complete
tasks_completed: 2
tasks_total: 2
completed_date: 2026-06-09
duration_minutes: 5

key_files:
  modified:
    - internal/gitconfig/fragment.go
    - internal/gitconfig/fragment_test.go

decisions:
  - Use filewriter.EnsureDir (0o700) as the single dir-creation chokepoint, matching the project's conservative posture for managed dirs

tags: [bug-fix, tdd, gitconfig, filewriter, BUG-5]
---

# Quick 260609-s8j: WriteFragment must ensure its parent dir exists — Summary

## One-liner

Added `filewriter.EnsureDir(filepath.Dir(fragmentPath), 0o700)` to `WriteFragment` so git config fragment writes succeed on a fresh machine where `~/.gitconfig.d/` does not yet exist (E2E BUG-5).

## Tasks Completed

| # | Name | Commit | Files |
|---|------|--------|-------|
| 1 | RED: add failing TestWriteFragment_CreatesParentDir | 23388d2 | internal/gitconfig/fragment_test.go |
| 2 | GREEN: ensure fragment parent dir in WriteFragment | 5532352 | internal/gitconfig/fragment.go |

## What Was Done

### Task 1 (RED)

Added `TestWriteFragment_CreatesParentDir` to `internal/gitconfig/fragment_test.go`:

- `fragPath` is set to `filepath.Join(t.TempDir(), "gitconfig.d", "work")` — the `gitconfig.d` intermediate directory does NOT exist when the test starts.
- Calls `WriteFragment(fragPath, "Work User", "work@example.com", "~/.ssh/id_ed25519_work.pub")`.
- Asserts `err == nil`, the fragment file exists, and `gitGet(t, fragPath, "user.email")` returns `work@example.com`.
- Reuses the existing `gitGet` helper; hermetic via `t.TempDir()` only.

**RED verification:**

Input:
```
go test ./internal/gitconfig/ -run TestWriteFragment_CreatesParentDir -v
```

Output:
```
=== RUN   TestWriteFragment_CreatesParentDir
    fragment_test.go:92: WriteFragment: git config --file .../gitconfig.d/work user.name:
        exit status 255: error: could not lock config file .../gitconfig.d/work: No such file or directory
--- FAIL: TestWriteFragment_CreatesParentDir (0.01s)
FAIL
```

`make lint`: 0 issues. Test compiles, `WriteFragment` signature unchanged.

### Task 2 (GREEN)

Modified `internal/gitconfig/fragment.go`:

- Added imports `path/filepath` and `github.com/castocolina/gitid/internal/filewriter`.
- Added `filewriter.EnsureDir(filepath.Dir(fragmentPath), 0o700)` after the three value validations and before the `settings` loop.
- Updated the `WriteFragment` doc comment to document the parent-dir creation behavior.

**GREEN verification:**

Input:
```
go test ./internal/gitconfig/ -run TestWriteFragment -v
```

Output:
```
=== RUN   TestWriteFragment_RoundTrips        --- PASS (0.12s)
=== RUN   TestWriteFragment_SigningKeyIsPathNotInline --- PASS (0.07s)
=== RUN   TestWriteFragment_RejectsRemoteSection --- PASS (0.00s)
=== RUN   TestWriteFragment_RejectsInvalidEmail  --- PASS (0.00s)
=== RUN   TestWriteFragment_CreatesParentDir  --- PASS (0.07s)
PASS
ok  github.com/castocolina/gitid/internal/gitconfig 0.785s
```

`make lint`: 0 issues.
`make test` (includes -race): all 12 packages ok.

## Deviations from Plan

None — plan executed exactly as written.

## TDD Gate Compliance

- RED gate: `test(quick-260609-s8j): ...` commit `23388d2` — present.
- GREEN gate: `fix(quick-260609-s8j): ...` commit `5532352` — present.
- Both pre-commit hooks (goimports + golangci-lint) passed on each commit.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes introduced. The `EnsureDir` call is bounded to the fragment's parent dir, consistent with existing filewriter usage across the project.

## Self-Check

- `internal/gitconfig/fragment.go` — modified: present.
- `internal/gitconfig/fragment_test.go` — modified: present.
- RED commit `23388d2`: verified in git log.
- GREEN commit `5532352`: verified in git log.

## Self-Check: PASSED
