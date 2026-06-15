# Phase 1: Bootstrap - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-08
**Phase:** 01-bootstrap
**Areas discussed:** Module path & Go floor, Lint/security strictness, Hooks vs CI split, Scaffolding extent

---

## Module path & Go floor

| Option | Description | Selected |
|--------|-------------|----------|
| github.com/castocolina/gitid | Matches GitHub username from the gists | ✓ |
| github.com/castocolina/ssh-git-config | Matches repo name rather than tool name | |

| Option | Description | Selected |
|--------|-------------|----------|
| 1.23 | Research-recommended floor; broadest compatibility | |
| 1.26 | Latest (1.26.4); current branch | ✓ |
| 1.25 | Previous stable middle ground | |

**User's choice:** `github.com/castocolina/gitid`, Go **1.26** floor
**Notes:** User opted for the newest Go branch over the research's conservative 1.23 floor. All selected libraries support 1.26.

---

## Lint/security strictness

| Option | Description | Selected |
|--------|-------------|----------|
| Curated | Locked set + errcheck/ineffassign/misspell/revive/govet | ✓ |
| Strict (enable ~all) | linters.default: all, disable noisy ones | |
| Minimal | gofmt + govet + staticcheck + gosec only | |

| Option | Description | Selected |
|--------|-------------|----------|
| Block on any finding | lint/gosec failures block commit & CI | ✓ |
| Block lint, gosec high-only | gosec blocks only on HIGH severity | |

**User's choice:** Curated linter set; hard-fail on any finding
**Notes:** Aligns with the clean-history / clean-main standard. Stricter than config's `security_block_on: high`, accepted because the curated gosec set is expected to be quiet.

---

## Hooks vs CI split

| Option | Description | Selected |
|--------|-------------|----------|
| Fast commit, full pre-push | pre-commit: fmt+lint+gosec; pre-push: go test -race + coverage | ✓ |
| Full on every commit | fmt+lint+gosec+test on every commit | |
| Format + lint only | tests left to CI/manual | |

| Option | Description | Selected |
|--------|-------------|----------|
| Add GitHub Actions now | CI runs make lint + make test on push/PR | |
| Defer CI until remote exists | Makefile + pre-commit only for Phase 1 | ✓ |

**User's choice:** Fast pre-commit / full pre-push; defer CI
**Notes:** No git remote exists yet, so hosted CI is deferred to a later phase; the gates are encoded in the Makefile so adding CI later is trivial.

---

## Scaffolding extent

| Option | Description | Selected |
|--------|-------------|----------|
| Toolchain + minimal main | go.mod, Makefile, configs, trivial main.go + one test | |
| Full package skeleton | Also pre-create all internal/ packages + tui/ with placeholders | ✓ |
| Bare toolchain only | No main.go; just tooling + a sample test | |

**User's choice:** Full package skeleton
**Notes:** User chose to lay out the full `cmd/internal/tui` structure up front (slightly more horizontal than the Vertical-MVP default), so Phase 2 starts with the architecture's package layout already in place. Each package gets a `doc.go` + passing stub test to keep the harness green.

## Claude's Discretion

- `make install` target location (default `go install` → `$GOBIN`).
- Coverage threshold — report-only, no hard floor in Phase 1.
- LICENSE / README stub — minimal, planner's call.
- golangci-lint binary-install mechanism and pre-commit wiring details.

## Deferred Ideas

- Hosted CI (GitHub Actions) once a remote exists.
- Hard coverage-threshold gate after real code lands (Phase 2+).
- `linters.default: all` strict mode if the curated set proves insufficient.
