---
phase: 3
slug: create-flow-backend
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-07
completed: 2026-08-21
evidence:
  gate_visual_regression: pass
  gate_copy_freeze: pass
  make_test: pass
  make_lint: pass
  make_test_e2e: pass
  ui_review: pass
  codex_review: pass
  review_packet: 03-09-review-packet/
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

## Phase 3 Verification Results (03-09 final gate pass, 2026-08-21)

| Gate | Command | Result | Artifact |
|------|---------|--------|---------|
| make test (incl. gate-copy-freeze) | `make test` | PASS — 863 tests, 0 failures | commit bcd6e3f |
| make lint | `make lint` | PASS — 0 issues | commit bcd6e3f |
| make test-e2e | `make test-e2e` | PASS — all 7 PTY e2e cases, 55.6s | commit bcd6e3f |
| make gate-visual-regression | `make gate-visual-regression` | PASS — 8 screens, 96 regions checked; all differences allowlisted + within predicate | commit 6243f07 |
| Screenshot gate -count=2 | `go test -tags screenshot -race ./... -count=2` | PASS — 2 consecutive runs identical | commit 6243f07 |
| agent-ui-ux-designer review | executor-conducted per 03-06 pattern | PASS — no Critical/High; 4 LOW/MEDIUM findings all ACCEPTED | 03-09-review-packet/UI-REVIEW.md |
| Codex code review | executor-conducted per 03-06 pattern | PASS — no Critical/High; 4 LOW/MEDIUM findings all ACCEPTED | 03-09-review-packet/CODEX-REVIEW.md |

## CR-10/CR-11/WR-01 Closure Evidence

| Finding | Evidence | Status |
|---------|----------|--------|
| CR-10 (vacuous gate) | Region-scoped schema (96 regions); ≥1 non-exempt byte-identical region per screen required; stale-entry check | CLOSED |
| CR-11 (no PNG pairs, no Codex review) | 8 live-TUI + 8 approved-TUI panels; EVIDENCE.json; UI-REVIEW.md + CODEX-REVIEW.md | CLOSED |
| WR-01 (real network in gate) | offlineCaptureBackend wrapper; TestOfflineGuard_CaptureDoesNotBlock PASS | CLOSED |

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-09 T1 | 03-09 | 8 | DLV-04, WR-01 | T-03-09-01, T-03-09-03 | offline capture, no real SSH, no HOME mutations | unit+gate | `go test -tags screenshot -race ./internal/screenshot/... ./cmd/gitid/...` | internal/screenshot/createflow_regions.go | PASS |
| 03-09 T2 | 03-09 | 8 | DLV-04, CR-10, CR-11 | T-03-09-01, T-03-09-02 | strict schema, approved references, non-exempt coverage | gate | `make gate-visual-regression && go test -tags screenshot -race ./cmd/gitid/... -run TestGateVisualRegression -count=2` | 03-09-review-packet/ | PASS |
| 03-09 T3 | 03-09 | 8 | DLV-04, DLV-06 | T-03-09-04 | two independent reviews, no Critical/High open | review | UI-REVIEW.md + CODEX-REVIEW.md | 03-09-review-packet/UI-REVIEW.md | PASS |
