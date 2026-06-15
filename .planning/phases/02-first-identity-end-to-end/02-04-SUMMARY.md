---
phase: 02-first-identity-end-to-end
plan: 04
subsystem: internal/sshconfig
tags: [ssh, renderer, parser, writer, managed-block, idempotent, macos-keychain]
requires:
  - internal/filewriter (ReplaceBlock, Write — 02-01)
  - internal/platform (SupportsUseKeychain — 02-02)
  - github.com/kevinburke/ssh_config@v1.6.0
provides:
  - sshconfig.RenderHostBlock (SSH-01 host stanza)
  - sshconfig.RenderGlobalBlock (SSH-03 macOS Host * block)
  - sshconfig.Parse (round-trip parse wrapper)
  - sshconfig.Write (idempotent managed-block composer + atomic write)
affects:
  - go.mod (pins kevinburke/ssh_config v1.6.0)
tech-stack:
  added:
    - github.com/kevinburke/ssh_config v1.6.0
  patterns:
    - sentinel-delimited managed blocks (host keyed by account, global keyed _global last)
    - parse -> compose -> parse round-trip validation before write
    - all writes delegated to filewriter chokepoint (no direct config writes)
key-files:
  created:
    - internal/sshconfig/renderer.go
    - internal/sshconfig/renderer_test.go
    - internal/sshconfig/parser.go
    - internal/sshconfig/parser_test.go
    - internal/sshconfig/writer.go
  modified:
    - go.mod
    - go.sum
  deleted:
    - internal/sshconfig/sshconfig_stub_test.go
decisions:
  - "02-04: macOS Host * block emits IgnoreUnknown UseKeychain first so a synced config does not break Linux ssh -G (Pitfall 4)"
  - "02-04: global block keyed _global and ReplaceBlock-appended last so first-match-wins keeps aliases authoritative (Pitfall 5)"
  - "02-04: renderer/parser/writer commit RED+GREEN combined — lint-gated pre-commit hook (revive unused-parameter, gosec) rejects signature-bearing zero-value stubs, so a standalone compilable RED commit is impossible for pure renderers; RED proven via local test run"
  - "02-04: writer validates composed bytes with a second Parse pass before the atomic write (parse->compose->parse stability gate)"
metrics:
  duration_min: 4
  completed: 2026-06-09T18:26:48Z
  tasks: 2
  files: 7
---

# Phase 02 Plan 04: internal/sshconfig Summary

Renders managed SSH `Host <alias>` stanzas and the macOS `Host *` keychain block, parses them round-trip via kevinburke/ssh_config, and writes them as idempotent sentinel blocks through the filewriter chokepoint — the SSH artifact for the create-new slice.

## What Was Built

- `RenderHostBlock(alias, hostname, port, identityFile)` — emits, in order, `Host`, `Hostname`, `Port`, `User git`, `IdentityFile`, `IdentitiesOnly yes` (SSH-01). Body-only; writer wraps in sentinels. The same function renders both a real-provider-host default identity and an `<identity>.<provider>` alias (SSH-02).
- `RenderGlobalBlock(os)` — on darwin emits `Host *` with `IgnoreUnknown UseKeychain` → `UseKeychain yes` → `AddKeysToAgent yes` (SSH-03); empty string on every other OS, guarded by `platform.SupportsUseKeychain`.
- `Parse(content)` — wraps `ssh_config.Decode` for comment-preserving round-trips; empty input parses to a valid empty config.
- `Write(configPath, accountName, hostBlock, globalBlock)` — reads existing config (missing = empty), `filewriter.ReplaceBlock` for the account block then for the `_global` block (appended last), validates the result with a second `Parse`, then `filewriter.Write` (atomic temp→rename, 0600, timestamped backup). Returns a non-empty backup path when the config pre-existed.

## Requirements Satisfied

- **SSH-01** — host block has all five directives, order asserted.
- **SSH-02** — alias and real-host forms both renderable (single function, tested with alias form).
- **SSH-03** — macOS Host * order (IgnoreUnknown→UseKeychain→AddKeysToAgent) and placement-last asserted; Linux-empty asserted.
- **SAFE-02** — round-trip stable, foreign content preserved byte-identical, idempotent double-write yields empty diff — all asserted.

## Threat Mitigations Implemented

| Threat | Mitigation |
|--------|-----------|
| T-02-13 (wrong key offered) | every host block emits `IdentitiesOnly yes` + explicit `IdentityFile` |
| T-02-14 (UseKeychain breaks Linux ssh -G) | `IgnoreUnknown UseKeychain` emitted before the Apple-only directive |
| T-02-15 (wildcard shadows aliases) | `_global` block always ordered LAST via separate sentinel key |
| T-02-16 (non-atomic write corrupts config) | all writes via `filewriter.Write`; production code never writes the config directly |
| T-02-17 (blind append duplicates blocks) | idempotent `ReplaceBlock`; foreign content byte-identical (verified) |

## Verification

- `go test ./internal/sshconfig/... -race` — green, 87.5% coverage.
- `make test` (full module, `-race`) — all packages green.
- `make lint` (golangci-lint v2: revive, gosec, staticcheck, unused, ...) — 0 issues.
- `grep -rn 'os.WriteFile' internal/sshconfig/*.go | grep -v _test.go` — NONE (production code writes only through filewriter).
- go.mod pins `github.com/kevinburke/ssh_config v1.6.0` exactly (pre-approved dependency checkpoint).

## Deviations from Plan

### Process deviations (TDD gate vs. lint-gated hook)

**1. [Rule 3 - Blocking] RED and GREEN committed combined per task**
- **Found during:** Task 1, confirmed in Task 2.
- **Issue:** The plan's runtime-note RED pattern requires a compilable failing-test commit. The pre-commit hook runs `make lint` and `--no-verify` is prohibited. For signature-bearing pure functions (`RenderHostBlock`, `RenderGlobalBlock`, `Parse`, `Write`) a zero-value stub leaves the declared parameters unused, which `revive`'s `unused-parameter` rule rejects (exit 2). A standalone RED commit is therefore impossible without weakening the public signature contract.
- **Fix:** RED was demonstrated at runtime locally (zero-value stub produced genuine test failures — captured during execution), then GREEN was implemented and the working RED+GREEN committed together. The RED→GREEN ordering of the TDD cycle was honored in execution; only the commit boundary was merged.
- **Files:** internal/sshconfig/renderer.go, parser.go, writer.go.
- **Commits:** b8f7530, 891e272.

**2. [Rule 3 - Blocking] go.mod ssh_config pin landed in Task 2, not Task 1**
- **Found during:** Task 1.
- **Issue:** `go get` after the checkpoint added the dependency, but Task 1's renderer imports only `internal/platform`, so `go mod tidy` correctly dropped the unused ssh_config require. The plan listed go.mod/go.sum under Task 1.
- **Fix:** Re-added and pinned `kevinburke/ssh_config v1.6.0` in Task 2 where `parser.go` imports it; go.mod/go.sum committed with Task 2. Net result matches the plan (v1.6.0 pinned, both files updated).
- **Commit:** 891e272.

**3. [Rule 1 - Lint] reworded writer.go comment + nolint annotations on test reads**
- **Found during:** Task 2 lint pass.
- **Issue:** (a) writer.go comment literally contained `os.WriteFile`, tripping the plan's `grep -rc 'os.WriteFile'` guard; (b) test fixture `os.ReadFile(path)` calls tripped gosec G304.
- **Fix:** Reworded the comment to "never writes the config file directly"; added per-line `//nolint:gosec` annotations (matching the filewriter_test.go convention) to the four TempDir-path test reads.
- **Files:** internal/sshconfig/writer.go, parser_test.go.
- **Commit:** 891e272.

## Out of Scope / Untouched

- `internal/deps/deps_stub_test.go` (deleted) and `internal/deps/deps_test.go` (untracked) were present in the working tree at plan start and belong to a different plan. Left untouched.

## Known Stubs

None. All functions are fully implemented and exercised by tests.

## TDD Gate Compliance

Plan `type: tdd`. The RED→GREEN cycle was executed for both tasks (RED proven via local test runs before GREEN), but the lint-gated pre-commit hook prevents committing a separate compilable `test(...)` RED commit for signature-bearing pure functions (see Deviation 1). Git log therefore shows `feat(...)` commits only; there is no standalone `test(...)` RED gate commit. This is a known constraint of this phase's hook configuration, consistent with the 02-03 decision (RED stubs must compile and pass lint).

## Self-Check: PASSED

All 5 created source files exist on disk; both task commits (b8f7530, 891e272) are present in git history.
