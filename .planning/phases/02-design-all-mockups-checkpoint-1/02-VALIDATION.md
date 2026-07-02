---
phase: 2
slug: design-all-mockups-checkpoint-1
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-02
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from `02-RESEARCH.md` § Validation Architecture. Task IDs are filled in by
> the planner; requirement-level signals below are the source of truth for Dimension 8.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`, build-tag-scoped (`screenshot`, `e2e`) — same convention as Phase 1. Mockup side is React+MUI v7 built with Vite (build config, not a test framework). |
| **Config file** | none for Go (stdlib testing); `vite.config.ts` for the mockup build only |
| **Quick run command** | `go build ./cmd/gitid-dummy/... ./internal/dummytui/...` (compile check, seconds) |
| **Full suite command** | `make screenshot-html-mockups && make screenshot-tui-mockups && make dummy-nav-e2e` |
| **Estimated runtime** | ~60–120 seconds (headless Chromium capture dominates) |

---

## Sampling Rate

- **After every task commit:** `go build ./cmd/gitid-dummy/... ./internal/dummytui/...` + the relevant single capture test (`-run TestCapture<Surface>`)
- **After every plan wave:** `make screenshot-html-mockups && make screenshot-tui-mockups && make dummy-nav-e2e`
- **Before `/gsd-verify-work`:** Full suite green AND every `CRITIQUE.md` at 0 open findings AND `APPROVAL.md` carries an `**APPROVED:**` line
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

> Task IDs (`02-NN-MM`) are assigned by the planner; rows are keyed to the requirement
> signal each task must satisfy. `File Exists` = ❌ W0 means the driving test/artifact is
> a Wave-0 gap created before capture is meaningful.

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | DLV-01 | Mockup loaded via local `file://` only (no remote) | automated (existence + count) | `find .planning/design/*/html -name '*.png' \| wc -l` matches FIELDS.md-derived expected count | ❌ W0 — `internal/screenshot/design_capture_test.go` | ⬜ pending |
| TBD | TBD | TBD | DLV-02 | ui-ux-designer critique resolved before approval | grep-checkable | `! grep -rq "OPEN" .planning/design/*/CRITIQUE.md` | ❌ W0 — per-surface CRITIQUE.md | ⬜ pending |
| TBD | TBD | TBD | DLV-05 (no backend) | Dummy import graph excludes all backend packages | automated (CI-checkable) | `go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/...` grep-checked vs backend-package list | ❌ W0 — new packages | ⬜ pending |
| TBD | TBD | TBD | DLV-05 (full nav) | Every screen reachable via documented keystrokes; zero writes under sandboxed `HOME` | e2e-observable | `go test -tags e2e -race -timeout 60s -run TestDummyNav ./e2e/...` | ❌ W0 — `e2e/dummy_nav_e2e_test.go` | ⬜ pending |
| TBD | TBD | TBD | DLV-08 | User approval recorded as the single hard checkpoint | human-approval (not automatable) | manual: `.planning/design/APPROVAL.md` contains `**APPROVED:**` | ❌ W0 — created at phase end via `checkpoint:human-verify` task | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `.planning/design/mockup-src/` — the React+MUI v7 workspace (package.json, pnpm-lock.yaml, vite.config.ts, src/), MUI pinned at `@mui/material@7.3.11` (NOT `@latest` → 9.x)
- [ ] `internal/dummytui/` — new nav-only package (hardcoded screen data, no backend imports)
- [ ] `cmd/gitid-dummy/main.go` — new dummy binary entry point
- [ ] `internal/screenshot/design_capture_test.go` — driving test enumerating all (surface, screen) tuples for BOTH html and tui capture
- [ ] `e2e/dummy_nav_e2e_test.go` — mirrors `e2e/ui_pty_e2e_test.go`'s creack/pty + x/vt harness
- [ ] `e2e/harness_test.go` — add `BuildDummyBinary` (or generalize `BuildBinaryFrom(pkgPath)`) for `cmd/gitid-dummy`
- [ ] Makefile targets: `screenshot-html-mockups`, `screenshot-tui-mockups`, `dummy-nav-e2e`
- [ ] `.planning/design/<surface>/FIELDS.md` + `CRITIQUE.md` templates (semantic parity manifest + critique log) — required before any capture is parity-meaningful

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Complete design approval (HTML + TUI-dummy screenshot set) | DLV-08 | Human aesthetic/copy/defaults judgment; the phase's single hard checkpoint by design | Present the full versioned screenshot set + per-surface FIELDS.md; user signs off on copy, field order, defaults, and safety affordances; record `**APPROVED:**` in `.planning/design/APPROVAL.md`. No backend logic for any surface may be written before this line exists. |
| HTML↔TUI semantic parity acceptance | DLV-02 | Field/label/copy parity across two media is a judgment call the ui-ux-designer critique frames but a human confirms | For each surface, confirm the CRITIQUE.md MUST-match list (fields, labels, verbatim copy, option sets, defaults, flow order, safety affordances) is satisfied and 0 findings remain OPEN. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies (DLV-08 approval + DLV-02 parity are the only human-gated items, both listed above)
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter (planner/checker sets once task IDs are mapped)

**Approval:** pending
