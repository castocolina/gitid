---
phase: quick-260907-eda
plan: 01
subsystem: infra
tags: [makefile, go-toolchain, bootstrap, ci, dev-environment]

# Dependency graph
requires: []
provides:
  - "setup-env self-heals on a Go-less machine (Homebrew, then golang.org/dl tarball fallback)"
  - "GOPATH_BIN never collapses to the unwritable /bin on a from-cold-start run"
  - "Regression test locking the go-presence-check-before-first-go-install ordering invariant"
affects: [dev-environment-bootstrap, makefile-setup-env]

# Actuals (#2632)
actuals:
  tokens: 2192
  tasks: 2
  commits: 1
plan_head_before: c01fad74e4a50ae7cb1e3bdac1cf49f90bcc67d9

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "GO_BOOTSTRAP_VERSION derives the tarball-fallback pinned version from GOTOOLCHAIN — one version to keep in sync, never a second hardcoded literal"
    - "GOPATH_BIN's $(if $(shell command -v go ...),...,$(HOME)/go/bin) fallback pattern for Make variables computed at parse time before a bootstrap-installed toolchain exists"

key-files:
  created: []
  modified:
    - "Makefile — GO_BOOTSTRAP_VERSION var, GOPATH_BIN fallback fix, PATH export extended with $(HOME)/.local/go/bin, setup-env recipe gains a command-v-go bootstrap step as its first two lines, doc comments updated"
    - "cmd/gitid/release_plumbing_test.go — new TestSetupEnvBootstrapsGoToolchainBeforeFirstGoInstall"

key-decisions:
  - "Landed Task 1 (RED test) and Task 2 (GREEN Makefile fix) as ONE commit per CLAUDE.md's commit-granularity rule (one coherent logical change), rather than separate RED/GREEN commits"
  - "Installed golangci-lint and goimports locally (via the Makefile's own documented installer/go install mechanism) so the real pre-commit hooks (make fmt + make lint) could run and pass — required by CLAUDE.md's 'never --no-verify' rule"

patterns-established:
  - "Pattern: a Makefile bootstrap conditional (command -v <tool> && no-op || { brew branch; else tarball branch }) using $$(...) command substitution and $$VAR shell vars, matching the file's existing uv-bootstrap precedent"

requirements-completed: [QUICK-260907-eda]

coverage:
  - id: D1
    description: "setup-env bootstraps a Go toolchain (brew, else pinned golang.org/dl tarball) before its first go install line, and is a no-op when go is already on PATH"
    requirement: "QUICK-260907-eda"
    verification:
      - kind: unit
        ref: "cmd/gitid/release_plumbing_test.go#TestSetupEnvBootstrapsGoToolchainBeforeFirstGoInstall"
        status: pass
      - kind: other
        ref: "PATH=/usr/bin make -n setup-env (real dry run, go absent from PATH)"
        status: pass
      - kind: other
        ref: "make -n setup-env (real dry run, go present on PATH)"
        status: pass
    human_judgment: false
  - id: D2
    description: "GOPATH_BIN falls back to $(HOME)/go/bin instead of the unwritable /bin when go is absent from PATH at Makefile parse time"
    requirement: "QUICK-260907-eda"
    verification:
      - kind: other
        ref: "PATH=/usr/bin make -n setup-env | grep -q -- '-b \"$HOME/go/bin\"' (real dry run)"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-09-07
status: complete
---

# Quick Task 260907-eda: Go Toolchain Bootstrap for setup-env Summary

**`make setup-env` now self-heals on a Go-less machine (brew, then the pinned golang.org/dl tarball) instead of dying at its first `go install` line, and the `GOPATH_BIN` cold-start bug this exposed (silently collapsing to the unwritable `/bin`) is fixed.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-09-07T13:40:18Z
- **Tasks:** 2 (RED test, GREEN implementation)
- **Files modified:** 2

## Accomplishments
- `setup-env`'s recipe now opens with a `command -v go` check: a silent no-op when go is already on PATH (CI's `actions/setup-go`, a prior local install), otherwise installs via `brew install go` when Homebrew is present, or falls back to downloading/extracting the pinned `go$(GO_BOOTSTRAP_VERSION)` tarball from `https://go.dev/dl/` (OS/arch resolved via `uname`, fails closed with a clear message on an unsupported architecture).
- `GOPATH_BIN` fixed to fall back to `$(HOME)/go/bin` (Go's own documented default GOPATH) instead of the empirically-confirmed unwritable `/bin` when `go env GOPATH` fails silently with no go on PATH.
- New `GO_BOOTSTRAP_VERSION := $(patsubst go%,%,$(GOTOOLCHAIN))` Make variable so the tarball fallback's pinned Go version is always derived from the single existing `GOTOOLCHAIN` pin, never a second literal that could drift.
- `export PATH` extended to include `$(HOME)/.local/go/bin` first, so a toolchain the bootstrap step just extracted resolves within the same `make setup-env` invocation.
- New regression test `TestSetupEnvBootstrapsGoToolchainBeforeFirstGoInstall` locks: (1) `setup-env` checks for an existing Go toolchain before doing anything else, (2) that check appears strictly before the target's first `go install` line, and (3) both the Homebrew and tarball fallback paths are present.
- Doc comments updated (top-of-file target summary, `## setup-env:` Tools-installed list) to describe the new bootstrap step.

## Task Commits

Both tasks landed as a single coherent commit per CLAUDE.md's commit-granularity rule (test-first authoring order does not require separate RED/GREEN commits in final history — one buildable, coherent logical change):

1. **Task 1 (RED) + Task 2 (GREEN): Go-toolchain bootstrap + GOPATH_BIN fix** - `0fdc648` (feat)

## Files Created/Modified
- `Makefile` - `GO_BOOTSTRAP_VERSION` variable, `GOPATH_BIN` fallback fix, extended `PATH` export, `setup-env` recipe's new bootstrap step (first two lines), updated doc comments (top-of-file target summary + `## setup-env:` Tools-installed list)
- `cmd/gitid/release_plumbing_test.go` - new `TestSetupEnvBootstrapsGoToolchainBeforeFirstGoInstall`

## Decisions Made
- Landed the RED test and GREEN implementation as one commit (CLAUDE.md's commit-granularity rule takes precedence over separate-per-task commits for this class of coherent change).
- Locally installed `golangci-lint` (v2.12.2, via the Makefile's own official binary installer) and `goimports` (via `go install`, matching `setup-env`'s own mechanism) in this execution environment, since neither was present and `make lint`/`make fmt` — the repo's real pre-commit hooks — needed to run and pass for real, per CLAUDE.md's "never `--no-verify`" rule.

## Deviations from Plan

None - plan executed exactly as written (Task 1 RED test, Task 2 GREEN Makefile fix, both verified with the plan's own `<verify>` commands producing real `RED_CONFIRMED` / `GREEN_CONFIRMED` output).

## Issues Encountered
- This execution environment had neither `golangci-lint` nor `goimports` installed, so `make lint`/`make fmt` (invoked by the repo's real pre-commit hooks) initially failed with "No such file or directory". Both were installed using the exact same mechanism `setup-env` itself already documents (golangci-lint's official binary installer script pinned to v2.12.2; `goimports` via `go install`), after which `make lint` (0 issues) and `make fmt` (no reformatting needed) both passed, and the commit went through the real pre-commit hook without `--no-verify`.
- `.planning/graphs/*` files (codegraph cache artifacts) were already modified in the working tree at task start, unrelated to this task. Left untouched and unstaged throughout — not part of this task's commit.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `setup-env` is now robust to a completely Go-less fresh clone (the originally reported Bazzite/immutable-Fedora failure mode) with no CI or `setup-env-release` changes needed — both continue to rely on `actions/setup-go` provisioning `go` before either target runs.
- No blockers or follow-up work identified for this task.

---
*Phase: quick-260907-eda*
*Completed: 2026-09-07*

## Self-Check: PASSED

- FOUND: Makefile
- FOUND: cmd/gitid/release_plumbing_test.go
- FOUND: .planning/quick/260907-eda-add-a-go-toolchain-bootstrap-step-to-set/260907-eda-SUMMARY.md
- FOUND: 0fdc648 (git log --oneline --all)
