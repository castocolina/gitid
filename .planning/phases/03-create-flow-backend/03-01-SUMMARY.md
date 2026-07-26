---
phase: 03-create-flow-backend
plan: 01
subsystem: create-flow backend seams
tags: [keygen, identity, tester, key-reuse, encrypted-keys, test-command-parity]
requires: []
provides:
  - "keygen.ScanReusableKeys — the D-10 key-reuse picker's backend enumeration"
  - "identity.Deps.ReadPub — nil-guarded seam that makes encrypted-key reuse work (D-11/KEY-06)"
  - "tester.ResolvedViaCommand — display-only stage-2 command with argv parity to ResolvedVia (TEST-01)"
affects:
  - cmd/gitid (composition root must wire ReadPub — plan 03-03)
  - internal/tuikit (consumes these through the Backend seam via view DTOs)
tech-stack:
  added: []
  patterns:
    - "Injected seam + nil-guard fallback (identity.Deps.ReadPub)"
    - "Single arg-slice builder shared by the executing and the display path (tester.resolvedViaArgs)"
    - "Tolerant scan: surface, never drop, unparseable/encrypted entries (D-13)"
key-files:
  created:
    - internal/keygen/keyscan.go
    - internal/keygen/keyscan_test.go
    - internal/tester/tester_command_test.go
  modified:
    - internal/identity/identity.go
    - internal/identity/modes.go
    - internal/identity/modes_test.go
    - internal/tester/tester.go
decisions:
  - "ensurePub returns an existing .pub VERBATIM instead of re-deriving it — re-deriving parses the private key, which is impossible for a passphrase-protected key"
  - "ReadPub is nil-guarded rather than required, so callers not yet migrated keep the previous behavior instead of nil-panicking"
  - "The stage-2 display string is built from the same arg slice ResolvedVia executes, never a hand-retyped literal"
metrics:
  requirements: [KEY-06, TEST-01, TEST-02]
  commit: 3dd4f47
  completed: 2026-07-25
---

# Phase 3 Plan 01: Create-flow backend seams Summary

Three UI-free backend gaps closed: a key-reuse scanner, a `ReadPub` seam that
makes reusing an encrypted key work without a passphrase prompt, and a stage-2
command string that is provably the command that runs.

## What is actually in the tree

Verified by reading the committed files, not by trusting the plan.

### 1. `internal/keygen/keyscan.go` — `ScanReusableKeys` (D-10 / D-13)

`ScanReusableKeys(sshDir string) ([]ReusableKey, error)` globs `id_*` (the
`keygen.KeyPaths` convention plus the OpenSSH default names), parses each
candidate in memory and returns one `ReusableKey` per entry, sorted by path so
the picker's rows never reshuffle between renders.

`ReusableKey` carries `Path`, `Algorithm` (the OpenSSH key-type token),
`Fingerprint` (`SHA256:…`, from the PUBLIC half), `HasPub`, `Encrypted` and a
non-fatal `ParseError`. It carries **no key material** — no private bytes, no
PEM, no passphrase (T-03-01), so nothing here can reach a log, a preview or a
command string.

Tolerance is the point: an encrypted key is still offered (its public half is
all reuse needs, and gitid never prompts for a passphrase — D-11), and one
unparseable file never aborts the scan (D-13). Helpers: `scanKeyCandidate`,
`pubMetadata`, `isPassphraseMissing`, `fileExists`.

Tests (`keyscan_test.go`): `TestScanReusableKeys`,
`TestScanReusableKeysEncryptedWithUnparseablePub`,
`TestScanReusableKeysMissingDir`, `TestScanReusableKeysIsDeterministic`,
`TestReusableKeyCarriesNoPrivateMaterial`, `TestKeyscanExecutesNothing` — the
last two pin the security properties directly rather than by inspection.

### 2. `internal/identity` — the `ReadPub` seam (KEY-06 / D-11)

`Deps` gains `ReadPub func(pubPath string) (pubLine string, err error)`.
`ensurePub` now branches: when `PubExists` reports the `.pub` is there, the
line is read back **verbatim** and the private key is never parsed. Previously
it re-derived the line via `DerivePub`, which calls `ssh.ParsePrivateKey` and
therefore fails outright on a passphrase-protected key — that was the KEY-06
bug.

The seam is nil-guarded on the same line as `PubExists`: an unwired caller
falls back to `DerivePub` (the previous behavior) instead of nil-panicking.
The doc comment is explicit that the fallback exists for backward
compatibility, not as an acceptable production wiring — `cmd/gitid` must wire
it (plan 03-03).

Tests (`modes_test.go`): `TestReuseEncryptedKeyWithExistingPub` stubs
`DerivePub` to fail exactly the way `ssh.ParsePrivateKey` fails on an
encrypted key, so the test can only pass if the `.pub` path is taken; it also
asserts the resulting `allowed_signers` line carries the existing public key.
`TestEnsurePubNilReadPubFallsBackToDerivePub` is the L2 nil-guard obligation.
`TestEnsurePubReadPubErrorIsWrapped` and `TestEnsurePubMissingPubStillDerives`
cover the error and absent-`.pub` paths. The shared fake
(`newFakeModeDeps`) deliberately leaves `ReadPub` nil so every other test in
the file also exercises the fallback branch.

### 3. `internal/tester` — `ResolvedViaCommand` (TEST-01 for stage 2)

`resolvedViaArgs(configPath, keyPath, alias)` was extracted as the single
source of truth for the stage-2 connectivity argv (`-F`, `-i`,
`IdentitiesOnly=yes`, `BatchMode=yes`, `ConnectTimeout=10`, `-T git@alias`).
`ResolvedVia` executes it; the new display-only `ResolvedViaCommand` renders
it via `exec.Command(...).String()` without executing. The command SHOWN can
no longer drift from the command RUN — the same contract `PreWriteCommand`
already gave stage 1. The `ssh -G` resolution call is deliberately excluded
(it takes no `-i`). Arguments stay in slice form, never a shell string
(T-03-03, gosec G204-clean).

Tests (`tester_command_test.go`):
`TestResolvedViaCommandMatchesResolvedViaArgv` compares against the shared
builder itself (so any future flag change moves both sides or fails here),
plus `TestResolvedViaCommandShape` and `TestResolvedViaUsesSharedArgBuilder`.

## Deviations from Plan

None in this plan's own code — it was already complete in the working tree
when this executor took over.

**Commit granularity deviation:** 03-01's files could not be committed on
their own. The pre-commit hooks run `make fmt` + `make lint` over the whole
module (`pass_filenames: false`) and stash unstaged changes first, which
reconstructs the half-finished 03-02 rename and fails to compile. Per
CLAUDE.md ("let the buildable boundary, not file count, set the commit
granularity") and LEARNINGS L11, 03-01 and 03-02 landed as one reconciled
commit, `3dd4f47`.

## Self-Check: PASSED

- `internal/keygen/keyscan.go` — FOUND (`func ScanReusableKeys` present)
- `internal/keygen/keyscan_test.go` — FOUND (6 tests)
- `internal/tester/tester.go` — FOUND (`func ResolvedViaCommand` present)
- `internal/tester/tester_command_test.go` — FOUND (3 tests)
- `internal/identity/identity.go` — FOUND (`ReadPub` on `Deps`)
- `internal/identity/modes.go` — FOUND (nil-guarded `ReadPub` branch in `ensurePub`)
- Commit `3dd4f47` — FOUND
- `go test -race ./internal/keygen/... ./internal/identity/... ./internal/tester/...` — PASS
