---
phase: 02-first-identity-end-to-end
plan: 07
subsystem: keygen + identity modes + cmd/gitid
tags: [reuse, add-account, rotate, derive-pubkey, mode-selection, fast-follow]
requires:
  - identity
  - keygen
  - filewriter
provides:
  - "keygen.DerivePublicKey — authorized-key line from an existing private key (ssh.ParsePrivateKey)"
  - "identity.Reuse — reuse an existing key (derive+write .pub if missing) through the four-writer pipeline (IDENT-02)"
  - "identity.AddAccount — second Host alias + includeIf sharing an existing key (IDENT-06)"
  - "identity.Rotate — fresh key re-pointing ALL FOUR artifacts, idempotent (no duplicate old refs), re-tested (KEY-01)"
  - "gitid identity rotate <name> command; three-mode selection in gitid identity add (D-10)"
affects: []
tech-stack:
  added: []
  patterns:
    - "single write path: runPipeline extracted from Create; Reuse/AddAccount/Rotate all reuse it (no parallel writer)"
    - "ReplaceBlock keyed by identity name → rotation replaces old key refs, never duplicates (SAFE-02)"
key-files:
  created:
    - internal/keygen/derive.go
    - internal/keygen/derive_test.go
    - internal/identity/modes.go
    - internal/identity/modes_test.go
    - cmd/gitid/rotate.go
    - cmd/gitid/rotate_test.go
  modified:
    - internal/identity/identity.go
    - cmd/gitid/add.go
    - cmd/gitid/add_test.go
    - cmd/gitid/main.go
decisions:
  - "Reuse derives a missing .pub via ssh.ParsePrivateKey (private body never leaves the function, T-02-28) and writes 0644"
  - "Rotate re-points the SSH host block, includeIf, fragment signingkey, and allowed_signers via runPipeline keyed by identity name — old refs replaced not duplicated (T-02-29), each file backed up (SAFE-01)"
  - "identity name charset validated (^[A-Za-z0-9._-]+$) before any ssh/git arg use (T-02-32); confirm gate on rotate (SAFE-03)"
checkpoint:
  type: human-verify
  gate: blocking
  resolution: "accepted — manual end-to-end (reuse/alias-coexistence/rotation against a real provider) DEFERRED per 02-VALIDATION.md Manual-Only (consistent with 02-06)"
metrics:
  duration: ~10 min
  completed: 2026-06-09
---

# Phase 2 Plan 07: reuse / alias / rotate fast-follow Summary

Completes Phase 2's create/lifecycle modes on top of the 02-06 create-new slice:
reuse an existing key, add a second account/alias for an existing identity, and
rotate a key with all four artifacts re-pointed and re-tested — all routed
through the single `runPipeline` write path extracted from `identity.Create`.

## What Was Built

- `keygen.DerivePublicKey` — `ssh.ParsePrivateKey` → authorized-key line (same
  format as Generate); used by Reuse when `<key>.pub` is absent.
- `identity.Reuse` (IDENT-02), `identity.AddAccount` (IDENT-06), `identity.Rotate`
  (KEY-01) in `modes.go`, each reusing `runPipeline` (the four-writer sequence).
- `gitid identity rotate <name>` command + three-mode selection (new / reuse /
  add-account) in `gitid identity add` (D-10).

## Verification (automated) — PASS

- `make test` green (identity 81.5%, keygen 80.5%, cmd/gitid 62.5%); `make lint`
  0 issues; `go vet` clean; no new deps; single-write-path invariant holds
  (no `os.WriteFile` in non-test identity/keygen source).
- Tests assert: reuse derives+writes missing `.pub`; AddAccount coexists via a
  distinct alias resolving to the same key; Rotate re-points all four artifacts
  with no duplicated old references; rotate handler panic-guarded + rejects
  injection-style names.
- **TDD gate note:** both tasks landed as `feat(02-07)` (`4048fd6`, `01f44cf`) —
  no standalone `test(02-07)` RED commit (same lint constraint as 02-04/02-06).
  RED proven via local runs before GREEN. Surfaced at the phase TDD review.

## Checkpoint Resolution

Final blocking end-to-end checkpoint **accepted with the manual proof deferred**
(consistent with the 02-06 decision): the reuse/alias/rotation auth proofs are
network/upload-dependent and Manual-Only per `02-VALIDATION.md`. All automated
tests used a temp HOME/fakes; the real `~/.ssh`/`~/.gitconfig` were never touched.

## Deferred Manual Verification (IDENT-02, IDENT-06, KEY-01)

Reuse (derive missing `.pub` + clipboard); add second alias → `ssh -G` resolves
same key; rotate → backups + four artifacts re-point to new key + resolved
`ssh -G` shows new identityfile; after upload, `ssh -T` authenticates.

## Self-Check: PASSED (automated scope)

- key-files exist; `feat(02-07)` ×2 present; `make test`/`make lint` green; real home clean.
