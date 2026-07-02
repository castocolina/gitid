---
phase: 02-first-identity-end-to-end
plan: 06
subsystem: identity + cmd/gitid
tags: [orchestration, four-writers, cobra, identity-add, identity-test, upload, dry-run, mvp-slice]
requires:
  - filewriter
  - platform
  - keygen
  - clipboard
  - sshconfig
  - gitconfig
  - tester
provides:
  - "identity.Create — create-new orchestration invoking FOUR writers (sshconfig, gitconfig includeIf, gitconfig fragment, keygen.WriteAllowedSigners) on confirmed write"
  - "gitid identity add — interactive create-new command (probe→keygen→clipboard→pre-write test→preview→confirm→write→ssh-add→resolved test→upload steps); --dry-run previews only"
  - "gitid identity test <name> — reusable resolved two-phase test (ssh -T + ssh -G), prints input+output"
  - "uploadInstructions(provider) — GitHub two registrations (auth+signing); GitLab usage-type both"
affects:
  - "02-07 (reuse/alias/rotate reuse identity.Create pipeline + cmd root)"
tech-stack:
  added:
    - "github.com/spf13/cobra v1.10.2 (pflag/mousetrap transitive)"
  patterns:
    - "deps-injected orchestration (identity.Deps) — fakes-testable, no business logic in cmd/"
    - "thin Cobra handlers (≤~30 lines) over the tested core; arg-slice exec only"
    - "pre-write gate by output substring (D-01); single y/N confirm or --dry-run (SAFE-03)"
key-files:
  created:
    - internal/identity/identity.go
    - internal/identity/identity_test.go
    - cmd/gitid/add.go
    - cmd/gitid/add_test.go
    - cmd/gitid/test.go
    - cmd/gitid/test_test.go
    - cmd/gitid/upload.go
  modified:
    - cmd/gitid/main.go
    - go.mod
    - go.sum
  deleted:
    - internal/identity/identity_stub_test.go
decisions:
  - "identity.Create invokes all four writers exactly once on a confirmed write; aborts with zero writes on pre-write Failure (D-01); previews-only when unconfirmed/dry-run (SAFE-03)"
  - "cobra root preserves thin main()->Execute() + version line; identity add/test registered as subcommands (Phase 5 CLI foundation, D-04)"
  - "interactive prompts with defaults shown (D-05): alias pre-selected (D-12), gitdir default (D-13), optional passphrase (D-07); ssh-add --apple-use-keychain on macOS, warn+continue if missing (D-08)"
  - "GitHub auth + signing are SEPARATE registrations (same .pub added twice); GitLab one key usage-type both (UP-01/UP-02)"
checkpoint:
  type: human-verify
  gate: blocking
  resolution: "accepted — manual end-to-end (auth/signing/clipboard/upload, network+provider-dependent) DEFERRED per 02-VALIDATION.md Manual-Only"
metrics:
  duration: ~10 min
  completed: 2026-06-09
---

# Phase 2 Plan 06: identity.Create + gitid identity add slice Summary

The first user-facing vertical slice: a real `gitid identity add` that takes a
new identity from key generation through the two-phase test flow and the four
coordinated safe writes, plus a reusable `gitid identity test` command. Built on
the tested Wave 1–3 core via dependency injection; thin Cobra handlers carry no
business logic (Phase 5 CLI foundation).

## What Was Built

- `internal/identity/identity.go` — `Create` orchestrates probe → keygen →
  clipboard copy → pre-write `ssh -i` test (classified by output substring,
  D-01) → unified four-artifact preview → single confirm → **four** writes
  (sshconfig Host block, gitconfig `includeIf`, per-identity fragment,
  `keygen.WriteAllowedSigners` to `~/.ssh/allowed_signers`) + global
  `gitconfig.SetAllowedSignersFile` pointer → `ssh-add` → resolved
  `ssh -T`/`ssh -G` test → upload steps. Deps injected as an `identity.Deps`
  struct (fakes-testable).
- `cmd/gitid/{main,add,test,upload}.go` — Cobra root + `identity add`
  (interactive, `--dry-run`) + `identity test <name>` + `uploadInstructions`.

## TDD / Verification

- `make test` green (identity 83.3%, cmd/gitid 59.0% coverage); `make lint` 0
  issues (gosec-clean, arg-slice exec only — `grep -rEc '(sh|bash) -c' cmd/gitid/`
  == 0). `go.mod` pins cobra v1.10.2.
- Orchestration test asserts all FOUR writers invoked exactly once on a confirmed
  write (incl. `WriteAllowedSigners`, SIGN-01), aborts with no writes on pre-write
  `Failure`, previews-only when unconfirmed/`--dry-run`. `uploadInstructions("github")`
  asserts both Authentication and Signing guidance. Handler panic-guards green.
- **TDD gate note:** both tasks landed as `feat(02-06)` commits (`f8094f4`,
  `5d03fe5`) — no standalone `test(02-06)` RED commit (same lint-driven constraint
  as 02-04: signature-bearing handler/orchestration funcs can't form a revive-clean
  zero-value RED stub). RED was proven via local test runs before each GREEN.
  Surfaced at the phase TDD review.

## Checkpoint Resolution

Final blocking end-to-end checkpoint **accepted with the manual proof deferred**
(user decision). The auth + signing + clipboard + upload end-to-end is
network/upload-dependent and Manual-Only per `02-VALIDATION.md`; it is recorded as
a deferred verification to run against a throwaway provider identity (capture via
`/gsd-verify-work`). All automated tests use a temp HOME/fakes — the real
`~/.ssh/config`, `~/.gitconfig`, `~/.ssh/allowed_signers` were never touched.

## Deferred Manual Verification (Success Criteria 1, 2, 4, 5; CLIP-01)

Run `gitid identity add` for a throwaway identity; confirm preview+backups+
`allowed_signers` block; upload `.pub` as GitHub auth+signing; re-run
`gitid identity test` → "successfully authenticated"; signed commit →
`git log --show-signature` "Good signature"; `.pub` on clipboard.

## Self-Check: PASSED (automated scope)

- key-files exist; `feat(02-06)` ×2 present; `make test`/`make lint` green; real home clean.
