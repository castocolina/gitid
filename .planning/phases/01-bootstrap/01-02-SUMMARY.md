---
phase: 01-bootstrap
plan: 02
subsystem: toolchain
tags: [makefile, golangci-lint, lint, build, test, fmt, toolchain, quality]
dependency_graph:
  requires: [01-01]
  provides: [Makefile, .golangci.yml]
  affects: [01-03]
tech_stack:
  added:
    - golangci-lint v2.12.2 (binary install via official install.sh)
    - goimports (go install golang.org/x/tools/cmd/goimports@latest)
    - gosec standalone (go install github.com/securego/gosec/v2/cmd/gosec@latest)
    - pre-commit (pip install)
  patterns:
    - Makefile as single source of truth for build/test/lint/fmt
    - golangci-lint v2 config with formatters section separate from linters
    - Hard-fail lint: no issue-count suppression (D-04)
    - goimports via find+exec (does not accept ./... wildcard)
key_files:
  created:
    - Makefile
    - .golangci.yml
  modified:
    - cmd/gitid/main_test.go (Rule 1 fix: unused test parameter renamed to _)
decisions:
  - goimports formatter registered under formatters.enable in .golangci.yml (v2 API distinction)
  - fmt target uses find+goimports then gofmt -w . (neither accepts ./... glob syntax)
  - gosec config.global.audit:false (no gosec exclusions for G204/G304/G306)
metrics:
  duration_minutes: 5
  completed_date: "2026-06-09T00:18:27Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 1
---

# Phase 1 Plan 2: Makefile Toolchain Surface Summary

**One-liner:** Makefile with seven .PHONY targets + golangci-lint v2.12.2 config; build/test/lint/fmt green; hard-fail lint demonstrated with planted/removed finding.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Author golangci-lint v2 curated config with hard-fail | d630423 | .golangci.yml |
| 2 | Author the Makefile target surface and prove targets run | af1fd75 | Makefile, .golangci.yml, cmd/gitid/main_test.go |

## What Was Built

### Makefile (`Makefile`)

Seven `.PHONY` targets as required by D-10 and TOOL-01:

- `setup-env` — installs goimports, golangci-lint v2.12.2 (via pinned binary installer, NOT `go install`), gosec standalone binary, and pre-commit; calls `install-hooks` placeholder (completed in 01-03)
- `build` — `go build -o bin/gitid ./cmd/gitid`
- `install` — `go install ./cmd/gitid`
- `uninstall` — removes `$(GOPATH)/bin/gitid`
- `test` — `go test -race -coverprofile=coverage.out ./...` (TDD harness command, D-06; coverage report-only, D-09)
- `lint` — `golangci-lint run ./...` (reads `.golangci.yml`, hard-fail, D-04)
- `fmt` — `find . -name "*.go" ... -exec goimports -w {} +` then `gofmt -w .`

### golangci-lint v2 Config (`.golangci.yml`)

- `version: "2"` (mandatory key)
- `linters.default: none` + explicit enable list (curated set, D-03): `govet`, `errcheck`, `staticcheck`, `gosec`, `unused`, `revive`, `misspell`, `ineffassign`
- `formatters.enable: [goimports]` (v2 API: formatters are separate from linters)
- No `max-issues-per-linter` or `max-same-issues` suppression (hard-fail, D-04)
- gosec: no exclusions — G204/G304/G306 fully active to guard Phase-2 filewriter

## Verification Results

| Check | Result |
|-------|--------|
| All 7 targets in Makefile | PASS |
| `make fmt` exit 0 | PASS |
| `make fmt` idempotent (second run exit 0, no changes) | PASS |
| `make build` exit 0, `bin/gitid` executable | PASS |
| `make test` exit 0, `coverage.out` written | PASS |
| `make lint` exit 0 on clean tree | PASS |
| `make lint` fails on planted errcheck/unused finding | PASS (exit 2, 2 issues) |
| `make lint` green after removing planted finding | PASS |
| golangci-lint v2.12.2 installed via binary installer | PASS |
| `.golangci.yml` version: "2" present | PASS |
| All curated linters enabled | PASS |
| No issue-count suppression keys set | PASS |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Unused test parameter `t` in `TestRunDoesNotPanic`**
- **Found during:** Task 2 — first `make lint` run
- **Issue:** `cmd/gitid/main_test.go` `TestRunDoesNotPanic(t *testing.T)` had parameter `t` unused; revive linter reported `unused-parameter`
- **Fix:** Renamed `t` to `_` per Go convention for unused parameters
- **Files modified:** `cmd/gitid/main_test.go`
- **Commit:** af1fd75

**2. [Rule 3 - Blocking] goimports does not accept `./...` wildcard**
- **Found during:** Task 2 — first `make fmt` attempt
- **Issue:** `goimports -w ./...` exits with `stat ./...: no such file or directory`; goimports takes file/directory paths, not Go package patterns
- **Fix:** Changed `fmt` recipe to `find . -name "*.go" -not -path "./.planning/*" -exec goimports -w {} +` then `gofmt -w .`
- **Commit:** af1fd75

**3. [Rule 3 - Blocking] golangci-lint v2 rejects `goimports` in `linters.enable`**
- **Found during:** Task 2 — first `make lint` run after installing golangci-lint
- **Issue:** v2 separates linters from formatters; `goimports` in `linters.enable` caused `Error: can't load config: goimports is a formatter`
- **Fix:** Moved `goimports` from `linters.enable` to `formatters.enable` section in `.golangci.yml`
- **Commit:** af1fd75

## Known Stubs

None — this plan produces toolchain config files only (Makefile, .golangci.yml). No application logic or UI stubs.

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes introduced. Threat mitigations as planned:

| Threat ID | Mitigation Status |
|-----------|-------------------|
| T-01-04 | golangci-lint install.sh pinned to v2.12.2, HTTPS, official URL |
| T-01-05 | No `eval` in Makefile recipes; all tool paths from `$(shell go env GOPATH)` |
| T-01-06 | gosec enabled in .golangci.yml with G204/G304/G306 active, no exclusions |
| T-01-SC | golangci-lint pinned; gosec/pre-commit install in setup-env (01-03 completes wiring) |

## Self-Check: PASSED

- [x] `.golangci.yml` exists and passes grep checks
- [x] `Makefile` exists with all 7 targets
- [x] Commits d630423 and af1fd75 exist in git log
- [x] `make build`, `make test`, `make lint`, `make fmt` all exit 0 on clean tree
- [x] Hard-fail demonstrated and reverted
