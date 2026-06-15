---
phase: 03-full-identity-crud-multi-identity
plan: "01"
subsystem: identity-reconstruction
tags: [tdd, reconstruction, filewriter, sshconfig, gitconfig, identity]
dependency_graph:
  requires: []
  provides:
    - filewriter.ListBlocks
    - filewriter.RemoveBlock
    - filewriter.BackupAndRemove
    - sshconfig.ParseManagedHosts
    - gitconfig.ParseManagedIncludeIf
    - gitconfig.ReadFragment
    - gitconfig.RemoveAllowedSignersLine
    - gitconfig.FragmentInfo
    - gitconfig.IncludeIfInfo
    - sshconfig.SSHHostInfo
    - identity.Account.Incomplete
    - identity.Reconstruct
  affects:
    - plans 02, 03, 04 (list, update, delete — all key off Reconstruct + ListBlocks/RemoveBlock)
tech_stack:
  added: []
  patterns:
    - ListBlocks/RemoveBlock mirror ReplaceBlock splice pattern in filewriter
    - ReadFragment via git config --file --list (arg-slice, nolint:gosec, G204-clean)
    - Reconstruct joins ParseManagedHosts + ParseManagedIncludeIf by identity name (D-01)
    - Incomplete csv marker for partial block sets (D-02)
    - nameUnion sorted slice for deterministic Account order
key_files:
  created:
    - internal/filewriter/block_list_test.go
    - internal/filewriter/filewriter_remove_test.go
    - internal/sshconfig/reader.go
    - internal/sshconfig/reader_test.go
    - internal/gitconfig/reader.go
    - internal/gitconfig/reader_test.go
    - internal/identity/loader.go
    - internal/identity/loader_test.go
  modified:
    - internal/filewriter/block.go
    - internal/filewriter/filewriter.go
    - internal/identity/identity.go
decisions:
  - ListBlocks returns nil (not empty slice) on empty/no-block input — consistent with Go convention
  - RemoveBlock consumes exactly one trailing blank line after END marker to prevent blank-line accumulation (Pitfall B)
  - BackupAndRemove uses atomic os.Rename — backup and removal in one syscall
  - parseHostBlockBody skips len(host.Patterns)==1 && host.Patterns[0].String()=="*" guard (Pitfall A)
  - ReadFragment treats unreadable fragment same as missing (best-effort D-02)
  - RemoveAllowedSignersLine requires BOTH email AND namespaces="git" match (T-03-01, Pitfall D)
  - RemoveAllowedSignersLine writes result via filewriter.Write at 0o600 (T-03-05)
  - Reconstruct returns nil (not empty slice) when no identity names found
  - Provider derived via TrimPrefix(alias, name+"."); left empty on non-default alias form (A1)
  - Incomplete field is comma-separated list of missing piece identifiers
metrics:
  duration: ~35 min
  completed: "2026-06-10"
  tasks: 3
  files_created: 8
  files_modified: 3
---

# Phase 3 Plan 01: Read-Side Reconstruction Foundation Summary

**One-liner:** ListBlocks/RemoveBlock/BackupAndRemove primitives + SSH/gitconfig readers + identity.Reconstruct join by sentinel name with Incomplete markers; proven by round-trip test (IDENT-07 + TOOL-04).

## What Was Built

This plan builds the Phase 3 read-side foundation — the three primitive layers that plans 02/03/04 all depend on:

**Layer 1 — filewriter primitives (Task 1):**
- `ListBlocks(content []byte) []NamedBlock`: scans content for all complete gitid managed blocks in file order; CRLF normalised; incomplete blocks (BEGIN without END) silently skipped
- `RemoveBlock(content []byte, name string) []byte`: splices out named block idempotently; consumes one trailing blank line to prevent blank-line accumulation on repeated add/delete cycles (Pitfall B)
- `BackupAndRemove(path string) (backupPath string, err error)`: atomic rename-based whole-file backup+remove; idempotent on missing file

**Layer 2 — SSH + gitconfig reconstruction readers (Task 2):**
- `sshconfig.ParseManagedHosts`: calls ListBlocks, parses each SSH block body via kevinburke/ssh_config; skips `_global` block; implicit `Host *` guard (Pitfall A)
- `gitconfig.ParseManagedIncludeIf`: calls ListBlocks, line-scans includeIf condition + path pairs
- `gitconfig.ReadFragment`: `git config --file <path> --list` (arg-slice, G204-clean); returns `FragmentInfo{Missing: true}` on absent or unreadable file (D-02 best-effort)
- `gitconfig.RemoveAllowedSignersLine`: requires BOTH email AND `namespaces="git"` to match before removing a line (T-03-01 / Pitfall D); writes result via `filewriter.Write` at mode 0o600 (T-03-05)

**Layer 3 — identity.Reconstruct join (Task 3):**
- `Account.Incomplete string` field added to Account struct (additive only, D-02)
- `identity.Reconstruct(sshBytes, gcBytes, readFrag)`: joins ParseManagedHosts + ParseManagedIncludeIf by identity name (D-01); derives Provider via `TrimPrefix(alias, name+".")` (A1); populates KeyPath/PubPath from IdentityFile; sets Incomplete csv for missing SSH block / includeIf block / fragment file
- `nameUnion`: sorted unique identity names across both maps (deterministic Account order)

## TDD Gate Compliance

All three tasks followed RED→GREEN:

| Gate | Task 1 | Task 2 | Task 3 |
|------|--------|--------|--------|
| RED commit | d57b0f3 (`test(03-01)`) | n/a (tests + impl together) | 596a82a (`test(03-01)`) |
| GREEN commit | a422255 (`feat(03-01)`) | ed5589a (`feat(03-01)`) | d746f90 (`feat(03-01)`) |

Note for Task 2: The RED stub for ParseManagedHosts had `parseHostBlockBody` unused under the unused linter (the helper was the full implementation but the exported function was a nil stub). The cleanest lint-compliant resolution was to commit RED test files + implement GREEN in the same commit — the RED was proven via local test runs before implementation (tests failed as expected with the nil stub). The `test(03-01)` commit for Task 2 was omitted and tests+implementation were committed together as `feat(03-01)`.

## Commits

| Hash | Type | Description |
|------|------|-------------|
| d57b0f3 | test | Failing tests for ListBlocks, RemoveBlock, BackupAndRemove (RED) |
| a422255 | feat | Implement ListBlocks, RemoveBlock, BackupAndRemove (GREEN) |
| ed5589a | feat | Implement ParseManagedHosts, ParseManagedIncludeIf, ReadFragment, RemoveAllowedSignersLine (GREEN) |
| 596a82a | test | Failing tests for Account.Incomplete and Reconstruct join (RED) |
| d746f90 | feat | Implement Reconstruct join — nameUnion + Incomplete markers (GREEN) |

## Verification Evidence

- `go test ./internal/filewriter/... ./internal/sshconfig/... ./internal/gitconfig/... ./internal/identity/... -count=1` — all packages GREEN
- `make test` (race + coverage) — all 11 packages pass, no race conditions
- `make lint` — 0 issues (golangci-lint v2 + gosec)
- `TestReconstruct_RoundTrip` PASS — IDENT-07 + TOOL-04 definitively proven

**Coverage by package:**
- `internal/filewriter`: 77.7%
- `internal/gitconfig`: 87.7%
- `internal/identity`: 87.9%
- `internal/sshconfig`: 84.1%

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written.

**Minor deviation (TDD gate):** Task 2's RED commit was not staged separately because the exported `ParseManagedHosts` stub caused the `unused` linter to flag `parseHostBlockBody` (the helper contained the full GREEN implementation but the stub returned nil without calling it). The test file was committed together with the GREEN implementation as `feat(03-01)`. The RED failure was verified via a local test run before implementing GREEN. This is consistent with the project's established pattern from Phase 2 (STATE.md decision: "02-04: sshconfig render/parse/write commit RED+GREEN combined — lint-gated hook rejects signature-bearing zero-value stubs").

## Threat Mitigations Applied

All T-03-0x threats from the plan's threat register are implemented:

| Threat ID | Status | Evidence |
|-----------|--------|---------|
| T-03-01 | Mitigated | RemoveAllowedSignersLine requires BOTH email AND `namespaces="git"` token |
| T-03-02 | Mitigated | ReadFragment uses `exec.Command("git","config","--file",path,"--list")` arg-slice + `//nolint:gosec` |
| T-03-03 | Mitigated | Error messages name path identifier only; no key material in logs |
| T-03-04 | Mitigated | ListBlocks skips BEGIN-without-END; ParseManagedHosts returns zero-value SSHHostInfo on parse error |
| T-03-05 | Mitigated | RemoveAllowedSignersLine calls `filewriter.Write(path, result, 0o600)` |

## Known Stubs

None — all functions are fully implemented.

## Self-Check: PASSED
