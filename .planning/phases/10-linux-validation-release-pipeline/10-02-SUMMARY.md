---
phase: 10-linux-validation-release-pipeline
plan: 02
subsystem: infra
tags: [github-actions, ci, fedora, container-job, ssh, dnf, goreleaser-adjacent]

# Dependency graph
requires: []
provides:
  - "ci.yml `fedora` container job running make test/make lint/make test-e2e on push-to-main + release tags"
  - "Named ssh -V regression test for Fedora's un-suffixed OpenSSH version string"
affects: [10-03-bazzite-manual-uat, 10-04-release-pipeline]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
actuals:
  tokens: 2578
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "GitHub Actions container: image job (fedora:latest) on ubuntu-latest, dnf install as the mandatory first step before any uses: action"
    - "git config --global --add safe.directory before actions/checkout inside a root-uid container job"

key-files:
  created:
    - cmd/gitid/ci_fedora_test.go
  modified:
    - .github/workflows/ci.yml
    - internal/platform/version_test.go

key-decisions:
  - "fedora job placed as a container:-keyed job on ubuntu-latest (not a runs-on label — GitHub has no Fedora-hosted runners), inserted between check and release, per plan text"
  - "REVIEW C-6 fixes (safe.directory + fetch-depth: 0) implemented exactly as specified in the plan's action text, both verified via a dedicated test each"

patterns-established:
  - "Container-job Node.js bootstrap: dnf install must run before actions/checkout since GitHub-authored actions are Node.js-based and the base image ships none"

requirements-completed: [PLAT-03]

coverage:
  - id: D1
    description: "ci.yml fedora job: push/tag-only cadence, dnf prerequisites first, safe.directory + fetch-depth:0 (REVIEW C-6), no permissions: key, full make test/lint/test-e2e suite"
    requirement: PLAT-03
    verification:
      - kind: unit
        ref: "cmd/gitid/ci_fedora_test.go#TestFedoraJobExists"
        status: pass
      - kind: unit
        ref: "cmd/gitid/ci_fedora_test.go#TestFedoraJobRunsOnlyOnPush"
        status: pass
      - kind: unit
        ref: "cmd/gitid/ci_fedora_test.go#TestFedoraJobUsesFedoraLatestContainer"
        status: pass
      - kind: unit
        ref: "cmd/gitid/ci_fedora_test.go#TestFedoraJobDnfInstallIsFirstStep"
        status: pass
      - kind: unit
        ref: "cmd/gitid/ci_fedora_test.go#TestFedoraJobSafeDirectoryBeforeCheckout"
        status: pass
      - kind: unit
        ref: "cmd/gitid/ci_fedora_test.go#TestFedoraJobCheckoutFetchesFullTagHistory"
        status: pass
      - kind: unit
        ref: "cmd/gitid/ci_fedora_test.go#TestFedoraJobDeclaresNoPermissions"
        status: pass
      - kind: unit
        ref: "cmd/gitid/ci_fedora_test.go#TestFedoraJobRunsTheFullAutomatedSuite"
        status: pass
      - kind: other
        ref: "docker run --rm fedora:latest sh -c 'dnf install -y --setopt=install_weak_deps=False git openssh-clients make nodejs tar gzip' (exit 0)"
        status: pass
    human_judgment: false
  - id: D2
    description: "internal/platform TestParseSSHVersion gains a named 'Fedora, no distro suffix (OpenSSH 10.x)' regression case using the real fedora:latest ssh -V string, closing D-03's risk item"
    requirement: PLAT-03
    verification:
      - kind: unit
        ref: "internal/platform/version_test.go#TestParseSSHVersion/Fedora,_no_distro_suffix_(OpenSSH_10.x)"
        status: pass
    human_judgment: false

# Metrics
duration: ~25min
completed: 2026-09-04
status: complete
---

# Phase 10 Plan 02: Fedora CI container job + ssh -V regression fixture Summary

**Added a `fedora:latest` container job to ci.yml (push/tag-only, full `make test`/`make lint`/`make test-e2e` suite, REVIEW C-6's safe.directory + fetch-depth:0 fixes) and a named Fedora regression case to the existing ssh -V parser test suite.**

## Performance

- **Duration:** ~25 min
- **Tasks:** 2/2 completed
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments

- New `fedora` job in `.github/workflows/ci.yml`: `runs-on: ubuntu-latest`, `container: { image: fedora:latest }`, gated `if: github.event_name == 'push'` (D-02 cost-tier cadence — Ubuntu already triple-covered per-PR via the `check` matrix).
- `dnf install -y --setopt=install_weak_deps=False git openssh-clients make nodejs tar gzip` is the job's first step, before any `uses:` action, since fedora:latest ships no Node.js and every GitHub-authored action (including `actions/checkout`) needs it to execute at all inside the container.
- REVIEW C-6 fixes implemented exactly as the plan's action text specified: `git config --global --add safe.directory "$GITHUB_WORKSPACE"` runs before `actions/checkout` (root-in-container vs. host-owned-checkout UID mismatch), and `actions/checkout`'s `with:` sets `fetch-depth: 0` so `git describe --tags` (used by both the Makefile's `VERSION` default and D-10's tag-based version stamping) has real tag history to match instead of silently degrading to a bare-SHA form.
- No job-level `permissions:` block on the `fedora` job — inherits the workflow's top-level `contents: read`, matching the `build-cross`/`check` jobs' existing undeclared-permissions precedent.
- New `cmd/gitid/ci_fedora_test.go` (8 test functions) asserting the job's exact shape, reusing `workflowPath`/`jobBlock`/`readRepoFile` helpers from `release_plumbing_test.go` (same package, no import needed).
- New named table-test case in `internal/platform/version_test.go`'s `TestParseSSHVersion`: "Fedora, no distro suffix (OpenSSH 10.x)" using the exact RESEARCH-verified fedora:latest `ssh -V` output (`OpenSSH_10.2p1, OpenSSL 3.5.7 9 Jun 2026`), proving the existing `sshVersionPattern` regex already handles Fedora's un-suffixed form correctly — no production code change was needed, only the regression-guard test.

## Task Commits

1. **Task 2 (authored/committed first, TDD-authoring-order-neutral test-only addition): close D-03's ssh -V regression risk with a named Fedora fixture case** - `ca54530` (test)
2. **Task 1: fedora container job in ci.yml** - `0be89b0` (feat)

_Note: Task 2 was committed first since it is a smaller, independently-buildable test-only change; both tasks compile and pass hooks standalone, satisfying CLAUDE.md's buildable-boundary commit-granularity rule._

## Files Created/Modified

- `.github/workflows/ci.yml` - New `fedora` container job (D-01/D-02), inserted after `check`, before the `release` job's preceding comment block.
- `cmd/gitid/ci_fedora_test.go` - 8 new test functions asserting the fedora job's shape (existence, cadence, container image, step ordering, safe.directory/fetch-depth fixes, no permissions:, the three make targets).
- `internal/platform/version_test.go` - One new named table-test case ("Fedora, no distro suffix (OpenSSH 10.x)") in `TestParseSSHVersion`.

## Decisions Made

- Followed the plan's action text verbatim for both REVIEW C-6 fixes (safe.directory placement, fetch-depth: 0) since the plan already fully specified their exact form and rationale — no interpretation needed.
- Committed Task 2 (the smaller, self-contained test-only change) before Task 1 in the final commit sequence — an ordering choice, not a scope deviation; both commits individually compile and pass `make fmt`/`make lint` per CLAUDE.md's buildable-boundary rule.

## Deviations from Plan

None - plan executed exactly as written. Both REVIEW C-6 fixes and the `TestFedoraJob*` assertions match the plan's `<action>` text field-for-field; the ssh -V regression case's expected parsed values match the plan's literal `SSHVersion{...}` spec exactly.

## Issues Encountered

- The first commit attempt for the `fedora` job hit a transient `Error: parallel golangci-lint is running` lock-contention failure in the pre-commit hook (no other `golangci-lint` process was actually running at the time — verified via `ps aux`). Retried the identical `git commit` immediately; it succeeded cleanly on the second attempt. Not a code issue; no fix required beyond the retry.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The `fedora` job is wired to run on the next push to `main` or a release tag; it cannot be exercised locally end-to-end (no local GitHub Actions runner), so its first real signal will come from the orchestrator's own push/merge of this wave.
- Plan 10-03 (the manual Bazzite UAT) can proceed independently — it covers the container-invisible residue (SELinux, Wayland, gcr-ssh-agent) this plan's D-03 risk checklist deliberately leaves to the human-in-the-loop half of D-01's two-part validation vehicle.
- No blockers for Plan 10-04 (release pipeline / goreleaser) — this plan did not touch `release.yml`, `.goreleaser.yaml`, or `scripts/install.sh`.

---
*Phase: 10-linux-validation-release-pipeline*
*Completed: 2026-09-04*
