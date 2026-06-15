---
phase: 02-first-identity-end-to-end
plan: 02
subsystem: platform + deps
tags: [probe, ssh-Q-key, fallback, install-hint, lookpath, tdd, mini-doc-01]
requires: []
provides:
  - "platform.ProbeKeyTypes — runs `ssh -Q key`, returns supported key-type tokens"
  - "platform.SelectAlgorithm — ed25519 default; fallback ed25519->rsa->ecdsa with warned flag (D-09)"
  - "platform.InstallHint — per-OS OpenSSH install/upgrade guidance (D-14)"
  - "platform.SupportsUseKeychain — darwin-only predicate (SSH-03 seam)"
  - "platform.CurrentOS — runtime.GOOS passthrough seam"
  - "deps.Detect / deps.Report.MissingRequired — tool availability report (ssh/ssh-keygen/git required; ssh-add/clipboard optional)"
affects:
  - keygen
  - sshconfig
  - clipboard
  - doctor
tech-stack:
  added: []
  patterns:
    - "arg-slice exec.Command(\"ssh\",\"-Q\",\"key\") probe (NOT ssh-keygen -Q key); gosec G204 clean"
    - "pure parse helper over fixture string (no live shell-out in unit tests)"
    - "fixed fallback chain as membership tests; warned=true on any non-ed25519 selection"
key-files:
  created:
    - internal/platform/platform.go
    - internal/platform/platform_test.go
    - internal/deps/deps.go
    - internal/deps/deps_test.go
  modified:
    - internal/platform/doc.go
  deleted:
    - internal/platform/platform_stub_test.go
    - internal/deps/deps_stub_test.go
decisions:
  - "Probe uses `ssh -Q key` per RESEARCH Pitfall 1 correction (D-09's `ssh-keygen -Q key` was wrong)"
  - "No-algorithm case returns an error carrying InstallHint(CurrentOS()) instead of failing opaquely (D-14, mini-DOC-01 seam)"
  - "Weaker fallback selection sets warned=true so the orchestrator can surface the downgrade (T-02-08)"
  - "deps optional tools (ssh-add, clipboard helpers) never appear in MissingRequired"
metrics:
  duration: ~20 min (incl. session-limit resume)
  completed: 2026-06-09
---

# Phase 2 Plan 02: platform + deps (toolchain seam) Summary

The OS/toolchain seam for Phase 2: probe the local SSH toolchain to pick the key
algorithm (ed25519 default, single deliberate fallback, per-OS install guidance
when none is usable), expose the macOS `UseKeychain` guard predicate, and report
external-tool availability. Built test-first; no new dependency.

## What Was Built

- `internal/platform/platform.go`
  - `ProbeKeyTypes()` — arg-slice `exec.Command("ssh","-Q","key")`; pure
    `parseKeyTypes` helper splits/trims tokens (unit-tested via the verified
    multi-line fixture, no live shell-out).
  - `SelectAlgorithm(supported)` — walks the fixed chain ed25519 → rsa(4096) →
    ecdsa as membership tests; `warned=true` on any non-ed25519 selection; on no
    match returns an error carrying `InstallHint(CurrentOS())` (D-14).
  - `InstallHint(os)` — darwin: `brew install openssh`; linux: apt/dnf/pacman +
    OpenSSH project link.
  - `CurrentOS()` / `SupportsUseKeychain(os)` — `runtime.GOOS` seam; darwin-only
    UseKeychain predicate consumed by the sshconfig `Host *` block (SSH-03).
- `internal/deps/deps.go`
  - `Detect()` — `exec.LookPath` for ssh/ssh-keygen/git (required) and
    ssh-add + clipboard helpers (pbcopy/wl-copy/xclip/xsel, optional) into a
    `Report`.
  - `Report.MissingRequired()` — pure method returning required gaps in fixed
    order ssh, ssh-keygen, git; optional tools excluded.

## TDD Gate Sequence

- **platform** (committed before the session-limit interruption): test + impl
  landed together in `feat(02-02)` (`25a5e1b`); RED was verified at runtime
  during that run.
- **deps** (resume close-out): proper RED → GREEN —
  `test(02-02)` `0d10f8c` (failing tests + compiling panic stub so the
  lint pre-commit hook passes; `--no-verify` is prohibited) → `feat(02-02)`
  GREEN implementing `Detect`/`MissingRequired`.

## Verification

- `go test ./internal/platform/... ./internal/deps/... -race` green.
- `make test` (full module, `-race` + coverage) green: deps 100.0%,
  platform 84.6%.
- `make lint` (golangci-lint + gosec) — 0 issues. `grep -c 'ssh-keygen'
  internal/platform/platform.go` == 0 (probe uses `ssh -Q key`).

## Requirements / Decisions

- Contributes to IDENT-01 (algorithm selection feeds keygen). D-09 (probe +
  fallback) and D-14 (per-OS install guidance) fully implemented and asserted.
- STRIDE: T-02-06 (arg-slice exec, no shell), T-02-07 (correct probe + actionable
  D-14 guidance), T-02-08 (warned downgrade surfaced).

## Notes

- This plan was interrupted by a session limit after the `platform` commit with
  `deps` mid-RED (test written, impl absent, uncommitted). Resumed via the
  safe-resume "close out manually" path: finished `deps` with a clean RED→GREEN,
  ran the full suite + lint, and recorded this SUMMARY + tracking.

## Self-Check: PASSED

- key-files.created exist on disk; `feat(02-02)` + `test(02-02)` commits present.
- `make test` + `make lint` green.
