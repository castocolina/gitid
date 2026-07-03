---
phase: 01-foundations-spikes-ci
plan: 07
subsystem: ci
tags: [ci, github-actions, cross-os, build-matrix, build-01, build-02, build-04, tool-01, tool-02, tool-03, tool-04]

# Dependency graph
requires:
  - phase: 01-foundations-spikes-ci (plans 01-06)
    provides: the make targets CI invokes (setup-env, test, lint, test-e2e, build-cross) and the code they exercise
provides:
  - .github/workflows/ci.yml — 3-runner PR/push gate (ubuntu-latest, macos-15-intel, macos-15) + build-cross, SHA-pinned actions, least-privilege permissions
  - Makefile — build-cross target (darwin amd64/arm64, linux amd64/arm64) + inline-PATH uv bootstrap in setup-env
affects: [phase-10-release-pipeline]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CI calls the SAME make targets a human runs locally (single source of truth) — no inlined go/golangci commands"
    - "All GitHub Actions pinned by 40-hex commit SHA with a trailing # vX.Y.Z comment; top-level permissions: contents: read; no secrets in Phase 1"
    - "Cost-tiered e2e: ubuntu+macos-15 on PR, all three on push to main (D-13)"
---

# Plan 01-07 — Cross-OS GitHub Actions CI + build matrix

## What was delivered

Task 1 (CI authoring, commit 773466a): `.github/workflows/ci.yml` with a 3-runner
`check` job (ubuntu-latest, macos-15-intel, macos-15) running `make setup-env` +
`test` (-race) + `lint` + tiered `test-e2e`, plus a `build-cross` job cross-compiling
the darwin amd64/arm64 + linux amd64/arm64 matrix once on Linux. All `uses:` pinned by
commit SHA, top-level `permissions: contents: read`, no secrets. Locally verified:
`actionlint` clean, `make build-cross` produces all four binaries, SHA-pin + permissions
greps pass.

Task 2 (a REAL green CI run on all three runners — the plan's human-checkpoint criterion):
achieved autonomously via `gh` with real observed evidence. The first runs went red and
surfaced eight latent cross-OS defects that macOS-only local runs had masked; each was
root-caused, reproduced under a CI-simulating env (`TERM=dumb SSH_AUTH_SOCK= go test -race
./...`), and fixed:

| Defect | Fix commit |
|--------|-----------|
| Linux `ssh -V` distro-suffix parse (01-01) | 2a642f0 |
| `$TERM`-unset ASCII-glyph tests (tui) | 2a642f0 |
| Headless doctor exit-0 assumption (no ssh-agent) | 2a642f0 |
| Codex HIGH: migrate rollback destroys its own backups | 4ff0e85 |
| Codex HIGH: migrate `ssh -G` had no timeout | 4ff0e85 |
| `uv` not on PATH in setup-env (macos-15-intel) | 451cb18 |
| Linux `ssh -G` grandchild-pipe hang past the deadline | b17b399 |
| Codex MEDIUM: BackupAndRemove overwrite-rename | b17b399 |

**Final green run (push to main, full tier incl. e2e on all three):**
https://github.com/castocolina/gitid/actions/runs/28645640620 —
check (ubuntu-latest / macos-15 / macos-15-intel) + build-cross all success.

## Verification (observed)

- Real GitHub Actions run green on all three runners (URL above) — BUILD-02 satisfied.
- `make build-cross` → 4 binaries incl. linux/arm64 (ELF aarch64).
- `actionlint` rc=0; SHA-pin, permissions, no-secrets greps all pass.
- Whole-module `make test` (-race) + `make lint` (0 issues) + `make test-e2e` green,
  locally and in CI.

## Requirements

BUILD-01, BUILD-02, BUILD-04, TOOL-01, TOOL-02, TOOL-03, TOOL-04 — complete.
