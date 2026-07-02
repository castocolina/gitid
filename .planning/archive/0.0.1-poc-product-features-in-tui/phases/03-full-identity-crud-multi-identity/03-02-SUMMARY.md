---
phase: 03-full-identity-crud-multi-identity
plan: "02"
subsystem: identity-list-and-coexistence
tags: [cobra, identity-list, reconstruction, coexistence, sc-2, ident-03]
dependency_graph:
  requires:
    - identity.Reconstruct (03-01)
    - identity.Account.Incomplete (03-01)
    - gitconfig.ReadFragment (03-01)
    - sshconfig.Write / RenderHostBlock (02-04)
    - tester.ParseResolved (02-05)
  provides:
    - cmd/gitid/list.go newListCmd/runIdentityList
    - identity list IDENT-03 (key path, alias, provider, port, match, incomplete marker)
    - internal/sshconfig TestMultiIdentityCoexistence (SC-2 proof)
  affects:
    - plans 03, 04 (update, delete — identity list is the read half of CRUD)
tech_stack:
  added: []
  patterns:
    - newListCmd follows newAddCmd Cobra factory pattern (RunE delegates, zero business logic in RunE)
    - runIdentityList reads missing config files as empty []byte (T-03-07 threat mitigated)
    - printAccounts grouped-by-identity layout with provider A1 hostname fallback
    - TestMultiIdentityCoexistence uses ssh -G -F hermetic config (no real ~/.ssh read, T-03-08)
    - t.Skip guard when ssh not found in environment
key_files:
  created:
    - cmd/gitid/list.go
    - cmd/gitid/list_test.go
    - internal/sshconfig/coexistence_test.go
  modified:
    - cmd/gitid/main.go
decisions:
  - runIdentityList is read-only — no Deps struct; calls identity.Reconstruct directly with gitconfig.ReadFragment
  - printAccounts grouped-by-identity: identity header (name/key/git), then alias/provider/port/match per account
  - A1 provider fallback: when Provider empty, display Hostname as provider column value
  - Incomplete marker wording: "! incomplete: missing <csv>" — light marker, no diagnosis (D-02)
  - TestMultiIdentityCoexistence uses ssh -G -F <hermetic config> to avoid reading real ~/.ssh/config (T-03-08)
metrics:
  duration: ~15 min
  completed: "2026-06-10"
  tasks: 2
  files_created: 3
  files_modified: 1
---

# Phase 3 Plan 02: `gitid identity list` + Multi-Identity Coexistence Summary

**One-liner:** `newListCmd`/`runIdentityList` reconstructs identities from disk via `identity.Reconstruct` and renders key path/alias/provider/port/match/incomplete-marker; SC-2 proven by hermetic `ssh -G -F` coexistence test.

## What Was Built

**Task 1 — `gitid identity list` command (IDENT-03):**

- `cmd/gitid/list.go`: `newListCmd()` builds the Cobra command; `runIdentityList(in io.Reader, out io.Writer)` orchestrates the read-only flow — resolves HOME, reads `~/.ssh/config` + `~/.gitconfig` (missing = empty `[]byte`, not an error, T-03-07), calls `identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)`, and renders.
- `printAccounts` renders each `Account` grouped by identity: identity header (name, key path, git name/email), then per-account fields (alias, provider with A1 hostname fallback when `Provider` is empty, port, match strategy rendered as `gitdir:<path>/` or `hasconfig:<value>`).
- When `acct.Incomplete != ""`, a light marker line is appended: `"  ! incomplete: missing <csv>"` (D-02 — marker + what's missing, never a diagnosis).
- Empty result prints `"no gitid-managed identities found"` (T-03-07 friendly empty message).
- Reuses `fp()` from `add.go` — not redefined.
- Registered in `cmd/gitid/main.go` via `identity.AddCommand(newListCmd())`.
- `cmd/gitid/list_test.go`: table-driven tests for empty, single full account, incomplete marker, provider A1 fallback, and multi-identity separator.

**Task 2 — Multi-identity coexistence proof (SC-2):**

- `internal/sshconfig/coexistence_test.go`: `TestMultiIdentityCoexistence` — writes two same-provider identities ("personal" → `personal.github.com` and "work" → `work.github.com`) into a hermetic `t.TempDir()` HOME via `sshconfig.Write`. Asserts `ssh -G -F <configPath> personal.github.com` and `ssh -G -F <configPath> work.github.com` each produce non-empty `IdentityFiles` and that `personalRC.IdentityFiles[0] != workRC.IdentityFiles[0]`.
- The `-F` flag pins the config path so the developer's real `~/.ssh/config` is never read (T-03-08).
- Guards with `t.Skip` when `ssh` is absent from `$PATH`.

## Commits

| Hash    | Type | Description                                                                |
|---------|------|----------------------------------------------------------------------------|
| a49e4ee | feat | Implement gitid identity list — reconstruct from disk + render (IDENT-03) |
| 0e398d2 | feat | Multi-identity coexistence proof via hermetic ssh -G -F (SC-2)             |

## Verification Evidence

- `go test ./cmd/... -run 'TestRunIdentityList|TestPrintAccounts' -count=1` — all 6 tests PASS
- `go test ./internal/sshconfig/... -run 'TestMultiIdentityCoexistence' -count=1` — PASS
- `make test` (race + coverage) — all 12 packages pass, no race conditions
- `make lint` — 0 issues (golangci-lint v2 + gosec)
- `go build ./...` — exits 0

**Coverage by package (unchanged from Plan 01 baseline for non-modified packages):**
- `cmd/gitid`: 62.5% (new list.go code included)
- `internal/sshconfig`: 84.1% (coexistence test runs against real ssh binary)

## Deviations from Plan

None — plan executed exactly as written.

Both tasks are `type="auto"` with no TDD gate requirement (the plan does not mark them `tdd="true"`). Tests and implementation were developed together in each task.

## Threat Mitigations Applied

| Threat ID | Status | Evidence |
|-----------|--------|---------|
| T-03-06 | Mitigated | `list` prints key PATHS only — `runIdentityList` calls `os.ReadFile` on the config files, never on key file bytes |
| T-03-07 | Mitigated | `os.IsNotExist(err)` check treats missing config as empty `[]byte`; empty result prints friendly message |
| T-03-08 | Mitigated | `ssh -G -F <tempConfig>` in coexistence test; real `~/.ssh/config` never read |
| T-03-SC  | Mitigated | Zero new external dependencies added in this plan |

## Known Stubs

None — all functions are fully implemented.

## Self-Check: PASSED
