---
phase: 10-linux-validation-release-pipeline
plan: 01
subsystem: build
tags: [version-stamping, ldflags, debug.ReadBuildInfo, cobra, tui]

requires: []
provides:
  - "internal/version.Resolve() — the D-09 hybrid ldflags/debug.ReadBuildInfo() version resolution package, table-tested per build path"
  - "Makefile VERSION default now a live, `--match \"v*\"`-filtered, leading-`v`-stripped `git describe` (D-10); LDFLAGS retargeted at internal/version's exported symbol path"
  - "cmd/gitid's composeVersion/versionString produce D-11's locked four-part `<version> (<commit>, <date>, <goos>/<goarch>)` stamp"
  - "`gitid version [--json]` subcommand sharing internal/version.Resolve() with `--version`"
  - "internal/tuikit.App.WithVersion(...) — additive help-overlay version line, zero regression risk for every existing caller"
affects: [10-02, 10-04, 10-05]

actuals:
  tokens: 7457
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Linker-target vars live in the package they belong to (internal/version), not in cmd/gitid — the Makefile's -X path now names github.com/castocolina/gitid/internal/version.<name> instead of main.<name>"
    - "A pure, table-testable fallback function (resolveFromBuildInfo) separated from the impure entry point (Resolve, which calls debug.ReadBuildInfo()) so every build-path case is exercised with a hand-constructed fixture, not a real subprocess build"

key-files:
  created:
    - internal/version/version.go
    - internal/version/version_test.go
    - cmd/gitid/version_cmd.go
    - cmd/gitid/version_cmd_test.go
  modified:
    - Makefile
    - cmd/gitid/main.go
    - cmd/gitid/main_test.go
    - cmd/gitid/release_plumbing_test.go
    - cmd/gitid/identity_test.go
    - e2e/release_e2e_test.go
    - internal/tuikit/app.go
    - internal/tuikit/app_test.go

key-decisions:
  - "internal/version's version/commit/buildDate vars carry NO literal default (grouped var block, zero-value empty string) so a non-empty compiled-in default never masks the debug.ReadBuildInfo() fallback path — matches D-09's letter exactly"
  - "Makefile VERSION default uses `$(patsubst v%,%,$(shell git describe --tags --match \"v*\" --always --dirty))` — the patsubst strip keeps the local unstamped-build default and the release pipeline's `${GITHUB_REF_NAME#v}` computation agreeing byte-for-byte on D-11's no-leading-`v` `--version` format (REVIEW C-1 in the plan text)"
  - "`gitid version --json`'s versionDocument field names (schema/version/commit/build_date/platform) mirror health.go's healthDocument pattern exactly, per 10-CONTEXT.md's Claude's-Discretion note"
  - "parityToolingExcluded now also excludes \"gitid version\" — a diagnostic readout like \"gitid debug\", not a parity-matrix write outcome"
  - "internal/tuikit.App.version is additive-only: zero value \"\" for every existing NewApp/NewAppPrefilled/NewAppOnGlobalSSH/NewAppOnGlobalGit caller, so renderHelp() stays byte-identical everywhere except cmd/gitid's own runApp(), which is the only caller of WithVersion"

patterns-established:
  - "Build-stamped version metadata: a single internal/version.Info flows through --version, `gitid version`, `gitid version --json`, and the TUI help overlay — one funnel, four surfaces"

requirements-completed: [BUILD-03]

duration: ~45min
completed: 2026-09-04
status: complete
---

# Phase 10 Plan 01: internal/version hybrid resolve, wired end-to-end Summary

**Stood up `internal/version`'s D-09 hybrid ldflags/debug.ReadBuildInfo() resolution and wired it through the Makefile's LDFLAGS, `cmd/gitid`'s `--version`/`gitid version [--json]`, and an additive TUI help-overlay line — the phase's tracer slice, fully green before any later Phase 10 plan builds on it.**

## Performance

- **Duration:** ~45 min
- **Tasks:** 2
- **Files modified:** 12 (4 created, 8 modified)

## Accomplishments

- `internal/version.Resolve()` implements D-09's hybrid resolution: ldflags-stamped vars win when present; otherwise a pure, table-tested `resolveFromBuildInfo()` helper derives `Info` from `debug.ReadBuildInfo()` (`Main.Version`, `vcs.revision`, `vcs.time`, `vcs.modified` → `+dirty` suffix), covering the go-install-stamped, plain-build-clean, plain-build-dirty, and not-ok cases without a real subprocess build.
- Makefile's `VERSION` now defaults to a live, `--match "v*"`-filtered `git describe`, patsubst-stripped of its leading `v` — verified live: unstamped `make build` produced `0.1.0-rc.9-147-gac26d92-dirty (none, unknown, darwin/amd64)`, no leading `v`. `LDFLAGS` retargeted from `-X main.<name>=` to `-X github.com/castocolina/gitid/internal/version.<name>=`.
- `cmd/gitid`'s `composeVersion`/`versionString` produce D-11's locked four-part stamp. Verified live: `make build VERSION=1.0.0 COMMIT=abc1234 DATE=2026-07-08 && ./bin/gitid --version` → exactly `gitid version 1.0.0 (abc1234, 2026-07-08, darwin/amd64)`.
- New `gitid version [--json]` subcommand: no-flags mode reuses `versionString()`'s composed stamp; `--json` emits a single-line `versionDocument` (schema `gitid.version/v1`) sourced from the SAME `version.Resolve()` call, so all three surfaces (`--version`, `version`, `version --json`) can never disagree.
- `internal/tuikit.App` gained an additive `version` field + `WithVersion(v string) App` (value receiver/return); `renderHelp()` appends exactly one conditional line, only rendered when `a.version != ""` — every existing caller (cmd/gitid-dummy, the create-flow wizard's own `tea.NewProgram`, every screenshot/PTY capture entry point) renders byte-identically to before. Only `cmd/gitid`'s `runApp()` calls `WithVersion(versionString())`.
- `parityToolingExcluded` extended to treat `"gitid version"` like `"gitid debug"` — a diagnostic readout excluded from the requirement-keyed parity matrix, with both doc comments updated.

## Task Commits

1. **Task 1: internal/version hybrid resolve, wired through the Makefile and cmd/gitid end-to-end (D-09, D-10, D-11)** - `ac26d92` (feat)
2. **Task 2: gitid version subcommand + --json, parity-matrix tooling exclusion, and additive TUI help-overlay version line (D-11, D-09)** - `2c26f7b` (feat)

_Both tasks are `tdd="true"`; TDD's RED/GREEN discipline was applied at authoring time (each table-test case and its assertion were written to fail first, then made to pass), collapsed into one commit per task per CLAUDE.md's buildable-boundary rule — the code is tightly intermixed inside the same functions and the repo's pre-commit hooks lint/build the whole module._

## Files Created/Modified

- `internal/version/version.go` - D-09 hybrid resolve: unexported linker-target vars (no literal default), `Info` struct, `Resolve()`, `resolveFromBuildInfo()`
- `internal/version/version_test.go` - table tests for every build-path case + an ldflags-preference test + a real-`debug.ReadBuildInfo()` smoke test
- `Makefile` - `VERSION` default now live git-describe (patsubst-stripped of leading `v`); `LDFLAGS` retargeted at `internal/version`'s exact var names
- `cmd/gitid/main.go` - `composeVersion`/`versionString` rewritten to consume `version.Info` + `runtime.GOOS`/`GOARCH`; registers `newVersionCmd()`; `runApp()` wires `WithVersion(versionString())`
- `cmd/gitid/main_test.go` - `TestVersionNonEmpty`→`TestVersionStringNonEmpty`; `TestComposeVersion` updated to the new signature; `TestComposeVersionUsesTheDevDefaults`→`TestVersionStringIncludesPlatformSuffix`; `TestNewRootCmdSurfaceIsPhase5CLI`'s want map gains `"version": true`
- `cmd/gitid/release_plumbing_test.go` - `TestLdflagsSymbolsAreDeclaredInMain`→`TestLdflagsSymbolsAreDeclaredInVersionPackage`, regex/lookup retargeted to `internal/version/version.go`
- `cmd/gitid/identity_test.go` - `parityToolingExcluded` also excludes `"gitid version"`; both doc comments updated to "completion / help / debug / version"
- `e2e/release_e2e_test.go` - `TestRelease_UnstampedBuildKeepsDevDefaults` now regex-matches the shape (VERSION's default is live/non-deterministic) with a `[^v]`-anchored version field (REVIEW C-1 regression guard)
- `cmd/gitid/version_cmd.go` - `gitid version [--json]`, `versionDocument`, `versionSchema`
- `cmd/gitid/version_cmd_test.go` - output-parity, JSON schema/field, and no-args tests
- `internal/tuikit/app.go` - `App.version` field, `WithVersion(v string) App`, additive `renderHelp()` line
- `internal/tuikit/app_test.go` - `WithVersion` additivity (exactly one new line, no lost lines) + non-mutation of the receiver

## Decisions Made

- No literal default for `internal/version`'s vars (matches D-09's letter — a non-empty compiled-in default would always take the ldflags branch even when the linker never touched the vars).
- `VERSION ?= $(patsubst v%,%,$(shell git describe --tags --match "v*" --always --dirty))` — the `patsubst` strip is required per the plan's REVIEW C-1: without it, a routine unstamped local `make build` would violate D-11 (no leading `v`) while the release path (which already computes `${GITHUB_REF_NAME#v}`) would not — a silent, untested inconsistency. Verified live both ways (stamped override and unstamped default).
- `versionDocument` field names mirror `healthDocument`'s exact shape (`schema`/`version`/`commit`/`build_date`/`platform`) per 10-CONTEXT.md's Claude's-Discretion note on `gitid version --json` field names.
- `parityToolingExcluded("gitid version")` treats the new subcommand as tooling (like `debug`), not a parity-matrix write outcome — consistent with D-11 describing it as a diagnostic readout.

## Deviations from Plan

None — plan executed exactly as written. Both tasks' `<action>` and `<verify>` blocks were followed literally; no Rule 1/2/3 auto-fixes or Rule 4 architectural questions arose.

## Issues Encountered

**Pre-acknowledged, plan-scoped e2e breakage (not fixed — explicitly out of scope for this plan):** `e2e/release_e2e_test.go`'s `TestRelease_BuildCrossStampsEveryTarget` (a DIFFERENT, same-pattern-named test from `release_plumbing_test.go`'s `TestBuildCrossStampsEveryTarget`) asserts an exact 3-part literal `e2eStampLine = "gitid version 9.9.9-e2e (deadbee, 2001-02-03)"`. D-11's new 4-part format (adding the `<goos>/<goarch>` platform suffix) makes this literal stale — confirmed live: `go test -tags e2e -run '^TestRelease_BuildCrossStampsEveryTarget$' ./e2e/...` now FAILs with `gitid-darwin-amd64 --version = "gitid version 9.9.9-e2e (deadbee, 2001-0..."` not matching the 3-part `e2eStampLine`.

The plan's own Task 1 action text explicitly anticipates and scopes this out (REVIEW C-5): *"that e2e test is untouched by THIS task and is fully rewritten by Plan 10-05 (Task 1) for the tar.gz artifact shape, not silently left stale."* Per the plan's own file list and action text, only `TestRelease_UnstampedBuildKeepsDevDefaults` (in the same file) was in scope for this plan, and it now passes (regex-shape match). `TestRelease_BuildCrossStampsEveryTarget` and the other raw-binary-shaped e2e tests remain untouched, pending Plan 10-05's tar.gz-artifact rewrite of the whole file — this is a known, deliberate, sequenced handoff between plans, not a regression introduced here. `make test-e2e` will show this ONE test failing until Plan 10-05 lands; every other verification this plan's own `<verification>` block names (`go build`, `go test -race` on the three listed packages, both `make build` invocations, both `gitid version[/--json]` checks) is green.

The `.planning/WINDOWS.md` cross-phase defect ledger does not yet exist in this repository and `gsd-tools.cjs` is not present in this worktree to create it — the ledger append was attempted and is unavailable (best-effort, non-blocking per the workflow). This paragraph is the durable record of the deferred item in lieu of a ledger row.

## Known Stubs

None. No hardcoded empty/placeholder values were introduced; `gitid version --json`'s fields are all live-sourced from `version.Resolve()` and `runtime.GOOS`/`GOARCH`.

## Threat Flags

None. This plan's only new surface (`gitid version --json`, build-time ldflags injection) is exactly what the plan's own `<threat_model>` (T-10-01-01, T-10-01-02) already covers — both accepted, low severity, no new attacker-reachable input.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/version.Resolve()` is a stable, importable foundation: Plan 10-04 (release pipeline / goreleaser) can reference the SAME `internal/version.<name>` -X path and `VERSION`/`COMMIT`/`DATE` Makefile vars with zero drift.
- Plan 10-02 (Linux/Fedora validation) inherits a working, race-tested `gitid --version`/`gitid version [--json]` surface with no further wiring needed.
- Plan 10-05 (tar.gz artifact migration) owns the acknowledged follow-up: rewriting `e2e/release_e2e_test.go`'s raw-binary-shaped tests (including `TestRelease_BuildCrossStampsEveryTarget`'s stale 3-part `e2eStampLine` literal) for both the new archive format AND D-11's 4-part version stamp in one pass.
- No blockers for 10-02/10-04 depending on this plan's `internal/version` package or Makefile LDFLAGS path.

## Self-Check: PASSED

- FOUND: internal/version/version.go
- FOUND: internal/version/version_test.go
- FOUND: cmd/gitid/version_cmd.go
- FOUND: cmd/gitid/version_cmd_test.go
- FOUND: .planning/phases/10-linux-validation-release-pipeline/10-01-SUMMARY.md
- FOUND: commit ac26d92
- FOUND: commit 2c26f7b

---
*Phase: 10-linux-validation-release-pipeline*
*Completed: 2026-09-04*
