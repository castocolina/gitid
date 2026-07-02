---
phase: quick-260609-s0m
plan: 01
subsystem: tester / identity / cmd
tags: [bug-fix, tdd, connectivity, ssh, pre-write]
dependency_graph:
  requires: []
  provides: [correct-pre-write-args, hostname-routing-fix]
  affects: [internal/tester, internal/identity, cmd/gitid/add]
tech_stack:
  added: [strconv (stdlib, for port formatting)]
  patterns: [fake-runner seam for unit tests, 3-arg PreWrite signature]
key_files:
  created: []
  modified:
    - internal/tester/tester.go
    - internal/tester/tester_test.go
    - internal/identity/identity.go
    - internal/identity/identity_test.go
    - cmd/gitid/add.go
    - cmd/gitid/add_test.go
decisions:
  - "Rename `port` to `_` in RED to satisfy revive unused-parameter lint while keeping the old body for genuine RED failures"
  - "defer strconv import to GREEN to avoid unused-import lint error in RED (per plan strategy)"
metrics:
  duration: ~10min
  completed: 2026-06-09
  tasks_completed: 2
  files_changed: 6
---

# Phase quick-260609-s0m Plan 01: Fix create-new pre-write connectivity gate Summary

**One-liner:** Fixed three bugs that caused `gitid identity add` to abort on any real provider: pre-write now dials the real hostname (not the unwritten SSH alias), passes `-p <port>` for port-443 endpoints, and adds `-o StrictHostKeyChecking=accept-new` for first-contact hosts.

## Objective

Fix the create-new pre-write connectivity gate (BUG-1, BUG-2, BUG-3) so it dials the real provider endpoint instead of the not-yet-written SSH alias.

## Tasks Completed

| # | Task | Type | Commit | Files |
|---|------|------|--------|-------|
| 1 | RED: widen PreWrite to 3-arg signature; add failing tests | TDD RED | a6bd3da | tester.go, tester_test.go, identity.go, identity_test.go, add.go, add_test.go |
| 2 | GREEN: build correct pre-write args (-p, accept-new) and route in.Hostname/in.Port | TDD GREEN | cb88a10 | tester.go, identity.go |

## Bugs Fixed

### BUG-1 (Critical): Wrong host in PreWrite call

**File:** `internal/identity/identity.go` line ~179

**Root cause:** `runPipeline` called `deps.PreWrite(key.PrivatePath, in.Alias, ...)`. The SSH alias (e.g. "work.github.com") is defined in `~/.ssh/config` which has NOT been written yet at the pre-write gate — DNS resolution fails → `Could not resolve hostname` → classified Failure → abort.

**Fix:** Changed call to `deps.PreWrite(key.PrivatePath, in.Hostname, in.Port)`. The real hostname (e.g. "ssh.github.com") is always DNS-resolvable.

### BUG-2 (Critical): Missing -p flag

**File:** `internal/tester/tester.go` `preWriteArgs`

**Root cause:** The arg slice never included `-p <port>`. gitid's defaults are ssh.github.com:443 and altssh.gitlab.com:443 — without `-p 443`, ssh defaults to port 22 which is refused by these endpoints.

**Fix:** Added `"-p", strconv.Itoa(port)` to the arg slice.

### BUG-3 (Important): Missing StrictHostKeyChecking=accept-new

**File:** `internal/tester/tester.go` `preWriteArgs`

**Root cause:** The arg slice had no `StrictHostKeyChecking` option. On first contact with a never-seen host, ssh prompts "Are you sure you want to continue connecting?" which in non-interactive mode → `Host key verification failed` → classified Failure → abort.

**Fix:** Added `"-o", "StrictHostKeyChecking=accept-new"` to the arg slice.

## TDD Gate Compliance

RED commit `a6bd3da`:
- `go build ./...`: ok
- `make lint`: 0 issues
- New tests FAIL at runtime (genuine RED):
  - `TestPreWriteArgs_ContainsRequiredFlags/github_port_443`: missing StrictHostKeyChecking=accept-new, -p, 443
  - `TestPreWriteArgs_ContainsRequiredFlags/gitlab_port_443`: same
  - `TestPreWriteWith_ClassifiesAndCapturesPortAndAcceptNew`: Result.Command missing -p/443/StrictHostKeyChecking
  - `TestCreatePassesHostnameNotAlias`: PreWrite called with hostname="work.github.com" (alias), want "ssh.github.com"

GREEN commit `cb88a10`:
- `make lint`: 0 issues
- `make test -race`: all 12 packages ok

## Signature Changes

`tester.PreWrite` / `tester.preWriteWith` / `tester.preWriteArgs`:
- Before: `(keyPath, host string)`
- After: `(keyPath, hostname string, port int)`

`identity.Deps.PreWrite` field:
- Before: `func(keyPath, host string) tester.Result`
- After: `func(keyPath, hostname string, port int) tester.Result`

All callers updated: `add.go buildDeps`, `add_test.go fakeDeps`, `identity_test.go newFakeDeps`.

## Verified Final State

```
$ grep "deps.PreWrite(key.PrivatePath, in.Hostname, in.Port)" internal/identity/identity.go
	pre := deps.PreWrite(key.PrivatePath, in.Hostname, in.Port)

$ grep "StrictHostKeyChecking=accept-new" internal/tester/tester.go
		"-o", "StrictHostKeyChecking=accept-new",

$ grep "strconv.Itoa(port)" internal/tester/tester.go
		"-p", strconv.Itoa(port),

$ make lint && make test -race
0 issues.
all packages ok
```

## Deviations from Plan

None - plan executed exactly as written. The one minor adaptation was using `_` for the unused `port` parameter in RED's `preWriteArgs` and in the `newFakeDeps` PreWrite fake to satisfy `revive unused-parameter` lint — this is the documented strategy from memory `tdd-red-stub-under-strict-lint`.

## Self-Check: PASSED

- [x] `internal/tester/tester.go` modified — confirmed
- [x] `internal/identity/identity.go` modified — confirmed
- [x] RED commit `a6bd3da` exists
- [x] GREEN commit `cb88a10` exists
- [x] `make lint`: 0 issues
- [x] `make test -race`: all packages ok
- [x] No test opens a network socket or references real `$HOME/.ssh`
