---
phase: 03-full-identity-crud-multi-identity
plan: "04"
subsystem: identity-delete
tags: [tdd, identity-delete, ident-05, d-07, d-08, safe-01, safe-03]
dependency_graph:
  requires:
    - filewriter.RemoveBlock (03-01)
    - filewriter.BackupAndRemove (03-01)
    - gitconfig.RemoveAllowedSignersLine (03-01)
    - identity.Reconstruct (03-01)
    - identity.Account (03-01)
  provides:
    - identity.DeleteDeps struct
    - identity.DeleteResult struct
    - identity.Delete(acct Account, keepKey bool, deps DeleteDeps) (DeleteResult, error)
    - cmd/gitid/delete.go newDeleteCmd/runIdentityDelete/buildDeleteDeps
    - gitid identity delete <name> (IDENT-05)
  affects:
    - identity CRUD complete (all four operations: add, list, update, delete)
tech_stack:
  added: []
  patterns:
    - DeleteDeps Deps-injection pattern mirrors identity.Deps (modes.go)
    - Delete passes ONLY acct.Name to RemoveBlock — never "_global" (D-08)
    - keepKey=false gates irreversible RemoveKeyFiles call (D-07)
    - Two-step confirm: first for blocks/file (reversible), second for key (irreversible, default no)
    - buildDeleteDeps mirrors buildDeps wiring pattern from add.go
    - Reconstruction-from-disk load pattern (same as update.go) to find Account before delete
key_files:
  created:
    - internal/identity/delete.go
    - internal/identity/delete_test.go
    - cmd/gitid/delete.go
    - cmd/gitid/delete_test.go
  modified:
    - cmd/gitid/main.go (registered newDeleteCmd)
decisions:
  - Delete passes ONLY acct.Name to RemoveBlock; never references "_global" in implementation (D-08)
  - keepKey logic: keepKey := !confirm(...) — single expression, staticcheck QF1007 compliant
  - buildDeleteDeps captures HOME at construction time (not per-call) — consistent with update/rotate
  - RemoveKeyFiles uses os.Remove with IsNotExist=ok (idempotent on missing files)
  - Two-step confirm: first prompt removes blocks/fragment (reversible backups); second explicitly prompts for key (default no = keep)
metrics:
  duration: ~5 min
  completed: "2026-06-10"
  tasks: 2
  files_created: 4
  files_modified: 1
---

# Phase 3 Plan 04: `gitid identity delete` Summary

**One-liner:** identity.Delete orchestration via RemoveBlock(acct.Name only)/BackupAndRemove/RemoveAllowedSignersLine with keepKey default-true gate (D-07) + `gitid identity delete <name>` Cobra command with will-remove manifest, two-step confirm, and buildDeleteDeps wiring (D-08); IDENT-05 satisfied, identity CRUD complete.

## What Was Built

**Task 1 — identity.Delete orchestration (RED→GREEN):**

`DeleteDeps` struct with seven function fields (ReadSSH, ReadGitconfig, WriteSSH, WriteGitconfig, RemoveFragment, RemoveAllowedSigners, RemoveKeyFiles) — same Deps-injection convention as UpdateDeps/Deps from identity.go.

`DeleteResult` struct holding the four backup paths (SSHBackup, GitconfigBackup, FragmentBackup, AllowedSignersBackup).

`Delete(acct Account, keepKey bool, deps DeleteDeps) (DeleteResult, error)`:
- Reads SSH config bytes via `deps.ReadSSH`, calls `filewriter.RemoveBlock(sshBytes, acct.Name)` (ONLY acct.Name — never `_global`), writes via `deps.WriteSSH`, stores backup path.
- Same pattern for gitconfig via `deps.ReadGitconfig` / `filewriter.RemoveBlock` / `deps.WriteGitconfig`.
- Removes fragment whole-file via `deps.RemoveFragment(acct.FragmentPath)`.
- Removes allowed_signers line via `deps.RemoveAllowedSigners(acct.AllowedSignersPath, acct.GitEmail)`.
- When `keepKey=false` only: calls `deps.RemoveKeyFiles(acct.KeyPath, acct.PubPath)` — the irreversible path.
- All errors wrapped `fmt.Errorf("identity: <action>: %w", err)`.
- `RemoveBlock` is idempotent — absent block returns input unchanged, no error.

8 tests covering: KeepKey (D-07), DeleteKey (D-07), GlobalAndForeignPreserved (D-08, SC-3), RemoveBlockUsedForSSHAndGitconfig (proves only acct.Name is used), AllowedSignersArgs, FragmentArgs, ReadSSHError (error propagation), Idempotent.

**Task 2 — `gitid identity delete <name>` command:**

`newDeleteCmd()`: `Use: "delete <name>"`, `Args: cobra.ExactArgs(1)`, `--dry-run` flag, `RunE` delegates to `runIdentityDelete`.

`runIdentityDelete(in, out, name, dryRun, depsFor)`:
- Validates name via `sanitizeName` + `identityNameRe` (shared helpers, not redefined).
- Resolves HOME, reads SSH/gitconfig bytes, calls `identity.Reconstruct` to find the account.
- Fills gitid-managed paths from HOME when reconstruction left them empty (mirrors update.go pattern).
- Prints the "will remove" manifest: [1] SSH Host block, [2] gitconfig block, [3] fragment file, [4] allowed_signers line (omit [4] when GitEmail is empty).
- Under `--dry-run`: prints manifest and returns early.
- First `confirm(reader, out, "Remove these managed blocks...")` — blocks/file are reversible (SAFE-03).
- Second `confirm(reader, out, "Also delete the private key files?")` — irreversible; default no (D-07). `keepKey := !confirm(...)`.
- Calls `identity.Delete(acct, keepKey, depsFor(out))`, prints backup paths and "kept/deleted key" line.

`buildDeleteDeps(_ io.Writer)`: wires all seven DeleteDeps fields to real packages:
- ReadSSH/ReadGitconfig: `os.ReadFile` with `IsNotExist → empty` (idempotent on missing files).
- WriteSSH/WriteGitconfig: `filewriter.Write(path, content, 0o600)`.
- RemoveFragment: `filewriter.BackupAndRemove` (direct assignment).
- RemoveAllowedSigners: closure around `gitconfig.RemoveAllowedSignersLine`.
- RemoveKeyFiles: `os.Remove(keyPath)` + `os.Remove(pubPath)` with `IsNotExist=ok`.

Registered via `identity.AddCommand(newDeleteCmd())` in main.go.

8 cmd tests: NotFound, InvalidName, DryRun, CancelledOnDecline, ConfirmKeepKey, ConfirmDeleteKey, ManifestContent, TwoConfirmCalls (D-07 two-step proven), NoRedefinesSharedHelpers.

## TDD Gate Compliance

| Gate | Task 1 | Task 2 |
|------|--------|--------|
| RED commit | 36ab6dc (`test(03-04)`) | N/A (type=auto, no tdd flag) |
| GREEN commit | 211fd2e (`feat(03-04)`) | 8c21f55 (`feat(03-04)`) |

RED→GREEN properly separated for Task 1 (tdd="true" task). RED stub used underscore-param form `Delete(_ Account, _ bool, _ DeleteDeps)` returning sentinel error — passes strict lint without `--no-verify`.

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 36ab6dc | test | Failing tests for identity.Delete (RED) — 8 tests covering D-07, D-08, SC-3 |
| 211fd2e | feat | Implement identity.Delete — per-identity artifact removal (GREEN) |
| 8c21f55 | feat | gitid identity delete command + buildDeleteDeps wiring (IDENT-05) |

## Verification Evidence

- `go test ./internal/identity/... -run 'TestDelete' -count=1` — 8 tests PASS
- `go test ./cmd/... -run 'TestRunIdentityDelete' -count=1` — 8 tests PASS
- `go test ./... -count=1` — all 12 packages GREEN
- `make test` (race + coverage) — all 12 packages pass, no race conditions
- `make lint` (golangci-lint v2 + gosec) — 0 issues
- `go build ./...` — exits 0

**Coverage by package:**
- `internal/identity`: 82.4%
- `cmd/gitid`: 64.6%

**Key acceptance criteria verified:**
- `grep -q 'func Delete(acct Account, keepKey bool, deps DeleteDeps)' internal/identity/delete.go` ✓
- `grep -q 'type DeleteDeps struct' internal/identity/delete.go` ✓
- `grep -q 'filewriter.RemoveBlock(' internal/identity/delete.go` ✓
- `_global` only appears in doc comment, never passed to RemoveBlock ✓
- `grep -c 'confirm(' cmd/gitid/delete.go` returns 2 (D-07 two-step) ✓
- `grep -c 'func confirm(' cmd/gitid/delete.go` returns 0 (shared helpers reused) ✓
- `grep -q 'AddCommand(newDeleteCmd())' cmd/gitid/main.go` ✓

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] staticcheck QF1007: merge conditional assignment**
- **Found during:** Task 2 lint
- **Issue:** `keepKey := true; if confirm(...) { keepKey = false }` triggers `staticcheck QF1007: could merge conditional assignment into variable declaration`
- **Fix:** Changed to `keepKey := !confirm(...)` — single expression, no behavior change
- **Files modified:** cmd/gitid/delete.go
- **Commit:** 8c21f55 (included in same commit, fix applied before commit)

None — plan executed as written apart from the staticcheck fix above.

## Threat Mitigations Applied

| Threat ID | Status | Evidence |
|-----------|--------|---------|
| T-03-14 | Mitigated | `sanitizeName`+`identityNameRe` in runIdentityDelete before name is used for anything |
| T-03-15 | Mitigated | Delete passes ONLY `acct.Name` to `RemoveBlock`; `_global` never referenced in impl (D-08); TestDelete_GlobalAndForeignPreserved proves foreign+global content byte-for-byte intact |
| T-03-16 | Mitigated | Key deletion gated behind second confirm (default no = keep); block/file removals backed up first (reversible) |
| T-03-17 | Mitigated | RemoveAllowedSignersLine writes at 0o600; filewriter.Write/BackupAndRemove use 0o600 for backups |
| T-03-18 | Mitigated | identity.Reconstruct load errors with "no gitid-managed identity named %q"; RemoveBlock on absent block = no-op (idempotent) |
| T-03-SC | Mitigated | Zero new external dependencies added |

## Known Stubs

None — all functions are fully implemented.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes beyond what the plan's threat model covers.

## Self-Check: PASSED
