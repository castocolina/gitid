---
phase: 03-full-identity-crud-multi-identity
plan: "03"
subsystem: identity-update
tags: [tdd, identity-update, writefragment, signing-toggle, structural-gate, ident-04]
dependency_graph:
  requires:
    - identity.Reconstruct (03-01)
    - identity.Account (03-01)
    - gitconfig.ReadFragment (03-01)
    - gitconfig.WriteFragment (phase-02)
    - gitconfig.RemoveAllowedSignersLine (03-01)
    - tester.Resolved (phase-02)
    - sshconfig.Write/RenderHostBlock (phase-02)
    - gitconfig.WriteIncludeIf (phase-02)
    - keygen.WriteAllowedSigners (phase-02)
  provides:
    - gitconfig.WriteFragment signing toggle (signing bool param)
    - identity.UpdateDeps struct
    - identity.UpdateResult struct
    - identity.Update(existing, edited, deps, signing)
    - cmd/gitid/update.go newUpdateCmd/runIdentityUpdate/buildUpdateDeps
    - gitid identity update <name> (IDENT-04)
  affects:
    - All callers of gitconfig.WriteFragment (updated to pass signing bool)
    - plans 04 (delete — reuses same UpdateDeps pattern)
tech_stack:
  added: []
  patterns:
    - WriteFragment signing toggle via conditional set + gitConfigUnsetAll (exit-5-safe)
    - UpdateDeps Deps-injection pattern mirrors identity.Deps (modes.go)
    - Update structural gate: alias/hostname/port change triggers Resolved re-test (D-05)
    - D-04 name immutability: edited.Name forced to existing.Name inside Update
    - ReadPub injectable dep in UpdateDeps (nil = default os.ReadFile)
    - runIdentityUpdate pre-fills prompts with current Account values via Reconstruct
    - buildUpdateDeps mirrors buildDeps wiring pattern from add.go
key_files:
  created:
    - internal/gitconfig/fragment_signing_test.go
    - internal/identity/update.go
    - internal/identity/update_test.go
    - cmd/gitid/update.go
    - cmd/gitid/update_test.go
  modified:
    - internal/gitconfig/fragment.go (WriteFragment signing bool param + gitConfigUnsetAll)
    - internal/gitconfig/fragment_test.go (updated call sites to pass signing=true)
    - internal/gitconfig/includeif_resolve_test.go (updated call site)
    - internal/gitconfig/reader_test.go (updated call site)
    - internal/identity/identity.go (Deps.WriteFragment type + runPipeline call)
    - internal/identity/identity_test.go (fake WriteFragment updated)
    - internal/identity/loader_test.go (WriteFragment call sites)
    - cmd/gitid/add.go (buildDeps WriteFragment closure with signing passthrough)
    - cmd/gitid/add_test.go (fake WriteFragment updated)
    - cmd/gitid/main.go (registered newUpdateCmd)
decisions:
  - WriteFragment gains signing bool param (not a separate function) — single write path for all fragment writes
  - gitConfigUnsetAll treats exit code 5 as success (key absent = no-op) — Pitfall C fix
  - signing=false does unset-all first, then sets user.name/email — authoritative end state
  - Update takes explicit signing bool parameter (not derived from edited Account) — clearer command-layer intent
  - ReadPub added as optional dep field (nil = default os.ReadFile) — fake-testable for tests
  - runIdentityUpdate loads current Account via identity.Reconstruct (no sidecar DB) — D-01 reconstruction
  - Match-strategy change rewrites gitconfig includeIf but does NOT trigger ssh -G re-test (A3)
metrics:
  duration: ~45 min
  completed: "2026-06-10"
  tasks: 3
  files_created: 5
  files_modified: 10
---

# Phase 3 Plan 03: `gitid identity update` Summary

**One-liner:** WriteFragment signing toggle (signing bool + exit-5-safe unset-all) + identity.Update orchestration with structural-change re-test gate (D-05) + `gitid identity update <name>` Cobra command with pre-filled prompts and single-confirm safe-write (D-06); IDENT-04 satisfied.

## What Was Built

**Task 1 — WriteFragment signing toggle:**
- `WriteFragment(fragmentPath, name, email, signingKeyPath string, signing bool) error` — new signature with explicit `signing` control
- `signing=true`: writes all five keys (user.name, user.email, gpg.format=ssh, user.signingkey, commit.gpgsign=true) — identical to prior Phase 2 behavior
- `signing=false`: removes the three signing keys via `gitConfigUnsetAll` (exit code 5 treated as success — Pitfall C), then writes only user.name and user.email — authoritative end state
- `gitConfigUnsetAll(path, key string) error` helper: `git config --file <path> --unset-all <key>` with exit-5-safe handling
- All existing callers updated to pass `signing=true` (create/reuse/rotate paths always sign): `identity.Deps.WriteFragment` type, `runPipeline` call, `buildDeps` closure in add.go, all test fakes and round-trip tests
- 5 new tests in `fragment_signing_test.go`: SigningTrue, SigningFalse, ToggleOnToOff, ToggleOffToOn, ValidationStillApplied

**Task 2 — identity.Update orchestration:**
- `UpdateDeps` struct with function fields: WriteSSH, WriteGitconfig, WriteFragment, WriteAllowedSigners, RemoveAllowedSigners, Resolved, ReadPub
- `UpdateResult` struct: Structural bool, Resolved, ResolvedTest, PreviewOnly
- `Update(existing, edited Account, deps UpdateDeps, signing bool) (UpdateResult, error)`:
  - D-04: `edited.Name = existing.Name` forced first (name immutability)
  - D-05 structural gate: `structural := edited.Alias != existing.Alias || edited.Hostname != existing.Hostname || edited.Port != existing.Port` — `deps.Resolved` called only when true
  - signing=true: reads pub line via deps.ReadPub, builds AllowedSignersLine, calls WriteAllowedSigners
  - signing=false: calls RemoveAllowedSigners with existing.GitEmail
  - All writes via injected deps (fake-testable)
  - TDD: RED commit (f74f26a) → GREEN commit (b112689)
- 7 tests: FragmentOnly (Resolved not called), Structural (called with alias), StructuralOnHostnameChange, StructuralOnPortChange, NameImmutable, SigningOffCallsRemoveAllowedSigners, SigningOnCallsWriteAllowedSigners

**Task 3 — `gitid identity update <name>` command:**
- `newUpdateCmd()`: `Use: "update <name>"`, `Args: cobra.ExactArgs(1)`, `--dry-run` flag
- `runIdentityUpdate(in, out, name, dryRun, depsFor)`: validates name via `sanitizeName`/`identityNameRe`, reconstructs current Account via `identity.Reconstruct` from disk, fills gitid-managed paths from HOME, prompts 8 fields pre-filled with current values, previews, single `confirm(reader, out, "Apply these changes now?")`, calls `identity.Update`
- `buildUpdateDeps(_)`: wires all 7 UpdateDeps fields to real sshconfig/gitconfig/keygen/tester packages
- Registered via `identity.AddCommand(newUpdateCmd())` in main.go
- D-06: preview → single confirm → write; `--dry-run` stops before confirm
- Shared helpers fp/confirm/prompt/identityNameRe/sanitizeName/promptPort/renderMatches reused, NOT redefined
- 5 cmd tests: NotFound, InvalidName, DryRun, CancelledOnDecline, Confirm, NoRedefinesSharedHelpers

## TDD Gate Compliance

| Gate | Task 1 | Task 2 | Task 3 |
|------|--------|--------|--------|
| RED commit | 199df05 (combined with GREEN — signature change breaks all callers, implemented GREEN directly) | f74f26a | N/A (type=auto, no tdd flag) |
| GREEN commit | 199df05 | b112689 | 62253cb |

Note for Task 1: The RED and GREEN were combined because the `WriteFragment` signature change is a cross-cutting breaking change that requires updating all callers (identity.go, identity_test.go, add.go, add_test.go, fragment_test.go, reader_test.go, includeif_resolve_test.go, loader_test.go) simultaneously to compile. A standalone RED commit with stub implementation would require all callers to be updated before the test file compiles — making the RED commit identical in scope to the GREEN. The RED failure was verified via local `go test` before implementing the full GREEN. Consistent with the established project pattern from Phase 2 (STATE.md decision: "02-04: sshconfig render/parse/write commit RED+GREEN combined").

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 199df05 | feat | WriteFragment signing toggle — add signing bool param |
| f74f26a | test | Add failing tests for identity.Update structural-change gate (RED) |
| b112689 | feat | Implement identity.Update with structural-change gate (GREEN) |
| 62253cb | feat | gitid identity update command + buildUpdateDeps wiring (IDENT-04) |

## Verification Evidence

- `go test ./internal/gitconfig/... -run 'TestWriteFragment' -count=1` — 10 tests PASS
- `go test ./internal/identity/... -run 'TestUpdate' -count=1` — 7 tests PASS
- `go test ./cmd/... -run 'TestRunIdentityUpdate' -count=1` — 5 tests PASS
- `go test ./... -count=1` — all 12 packages GREEN
- `make test` (race + coverage) — all 12 packages pass, no race conditions
- `make lint` (golangci-lint v2 + gosec) — 0 issues
- `go build ./...` — exits 0

**Coverage by package:**
- `internal/gitconfig`: 89.2% (up from 87.7%)
- `internal/identity`: 83.0% (up from 87.9%, note: update.go has dead-code path in readPubLine not exercised by internal tests)
- `cmd/gitid`: 63.7% (up from 62.5%)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] WriteFragment call site pattern in buildDeps**
- **Found during:** Task 1
- **Issue:** `WriteFragment: gitconfig.WriteFragment` direct assignment would work for the 4-param signature but the 5-param signature required a closure wrapper to match the function type
- **Fix:** Changed to closure `func(fragPath, name, email, signingKeyPath string, signing bool) error { return gitconfig.WriteFragment(..., signing) }` in buildDeps
- **Files modified:** cmd/gitid/add.go

**2. [Rule 2 - Enhancement] ReadPub dep added to UpdateDeps**
- **Found during:** Task 2 GREEN
- **Issue:** `identity.Update` needed to read the pub key file to build the AllowedSignersLine when signing=on, but direct `os.ReadFile` inside the function would not be fake-testable
- **Fix:** Added `ReadPub func(pubPath string) (string, error)` optional field to UpdateDeps (nil = default os.ReadFile). This keeps the Deps-injection pattern consistent and all signing-on tests fake-testable without disk I/O
- **Files modified:** internal/identity/update.go, internal/identity/update_test.go

**3. [Rule 1 - Fix] Test prompt count correction**
- **Found during:** Task 3
- **Issue:** Initial update_test.go used 9 newlines for 8 prompts, causing the confirm "y" to arrive at the signing prompt instead of the confirm prompt
- **Fix:** Corrected to 8 newlines (8 editable fields) + "y\n" for confirm
- **Files modified:** cmd/gitid/update_test.go

## Threat Mitigations Applied

| Threat ID | Status | Evidence |
|-----------|--------|---------|
| T-03-09 | Mitigated | sanitizeName + identityNameRe in runIdentityUpdate before name keys any path/block |
| T-03-10 | Mitigated | WriteFragment validates via validateValue/validateEmail; arg-slice exec (no shell) |
| T-03-11 | Mitigated | Single explicit confirm before any write; --dry-run previews without writing; filewriter.Write backs up on each mutated file |
| T-03-12 | Mitigated | Signing-off path calls RemoveAllowedSignersLine (matched by email+namespace="git") |
| T-03-13 | Mitigated | edited.Name = existing.Name forced inside Update (D-04) |
| T-03-SC  | Mitigated | Zero new external dependencies added in this plan |

## Known Stubs

None — all functions are fully implemented.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes beyond what the plan's threat model covers.

## Self-Check: PASSED
