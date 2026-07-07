---
phase: 3
slug: create-flow-backend
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-07
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (with -race), golangci-lint + gosec, PTY e2e harness |
| **Config file** | Makefile (single task runner) + .golangci.yml |
| **Quick run command** | `go test -race ./internal/... ./cmd/...` |
| **Full suite command** | `make test && make lint && make test-e2e && make gate-no-backend-files` |
| **Estimated runtime** | ~60s quick; ~240s full (test-e2e has a 180s budget) |

---

## Sampling Rate

- **After every task commit:** Run `go test -race ./internal/... ./cmd/...`
- **After every plan wave:** Run `make test && make lint && make test-e2e && make gate-no-backend-files`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 240 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (filled by planner — one row per task, see plan frontmatter) | | | | | | | | | |
