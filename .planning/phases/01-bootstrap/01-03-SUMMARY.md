---
phase: 01-bootstrap
plan: 03
subsystem: infra
tags: [pre-commit, git-hooks, makefile, golangci-lint, gosec, uv, tdd]

requires:
  - phase: 01-bootstrap
    provides: "Makefile fmt/lint/test targets and .golangci.yml from plan 01-02"
provides:
  - ".pre-commit-config.yaml (repo: local) wiring fast pre-commit (fmt+lint) and full pre-push (test) to make targets"
  - "make setup-env completed: one-command bootstrap that installs tooling and wires both git hook types"
  - "Local commit/push gating proven: a malformed commit is blocked, a clean commit passes, pre-push runs the race+coverage suite"
affects: [02-first-identity, all future phases — every commit is now gated locally]

tech-stack:
  added: [pre-commit (4.6.0, installed via uv tool)]
  patterns: ["repo: local pre-commit hooks invoking make targets (D-07) — Makefile is the single source of truth shared with future CI"]

key-files:
  created:
    - .pre-commit-config.yaml
  modified:
    - Makefile
    - .gitignore

key-decisions:
  - "pre-commit installed via `uv tool install pre-commit` (not pip — pip absent from PATH; user standardizes on uv); uv bootstrapped from astral.sh if missing, never brew"
  - "repo: local hooks only — no upstream hook repos (TekWizely) per D-07; Makefile is the shared CI source of truth"
  - "No .github/workflows created (D-08) — CI deferred until a remote exists"

patterns-established:
  - "Local-only gating (D-08): pre-commit (fmt+lint+gosec) + pre-push (race+coverage) are the Phase-1 quality gate"
  - "All hooks use language: system + pass_filenames: false, delegating to make targets"

requirements-completed: [TOOL-02, TOOL-03]

duration: ~30min (incl. interactive human-verify checkpoint and 3 environment/bug fixes)
completed: 2026-06-08
---

# Phase 01: Bootstrap — Plan 03 Summary

**repo:local pre-commit + pre-push hooks wired to make targets; `make setup-env` now bootstraps a fresh clone end-to-end, and a malformed commit is demonstrably blocked while the full test suite gates every push.**

## Performance

- **Duration:** ~30 min (Task 1 autonomous + blocking human-verify checkpoint)
- **Completed:** 2026-06-08
- **Tasks:** 2/2 (Task 1 autonomous, Task 2 human-verify checkpoint)
- **Files modified:** 3 (`.pre-commit-config.yaml` created, `Makefile` + `.gitignore` edited)

## Accomplishments
- `.pre-commit-config.yaml` with a single `repo: local` entry: `go-fmt` (`make fmt`) and `go-lint` (`make lint`, gosec embedded) on the pre-commit stage; `go-test` (`make test`, race+coverage) on the pre-push stage.
- `make setup-env` completed via the `install-hooks` sub-target (`pre-commit install` + `pre-commit install --hook-type pre-push`) — TOOL-02 one-command bootstrap.
- Phase-1 Success Criterion 3 / TOOL-03 proven live: malformed commit blocked, clean commit passes, pre-push runs the full suite.
- No CI added (D-08); no upstream hook repos (D-07).

## Task Commits

1. **Task 1: Author repo:local pre-commit config + complete setup-env hook install** — `d210cd8` (chore)
2. **Task 2: Human-verify checkpoint** — no code commit (verification only); two bug fixes surfaced and committed (see Deviations)

## Files Created/Modified
- `.pre-commit-config.yaml` — repo:local hooks; pre-commit (fmt+lint) + pre-push (test) stages
- `Makefile` — `install-hooks` sub-target completed; `setup-env` calls it; pre-commit install switched from pip to uv
- `.gitignore` — anchored `gitid` → `/gitid`

## Decisions Made
- Installed pre-commit with `uv tool install pre-commit` rather than pip (pip not on PATH; project standardizes on uv). uv bootstrapped via the Astral installer (`astral.sh`) when absent — never a system package manager.

## Deviations from Plan

### Auto-fixed Issues

**1. [Blocking] `make setup-env` used pip, which is not installed**
- **Found during:** Task 2 (human-verify — `make setup-env` failed with `pip: No such file or directory`)
- **Issue:** The 01-02 Makefile installed pre-commit via `pip install --quiet pre-commit`; pip is not on this machine's PATH and the user rejected pip.
- **Fix:** Switched to `uv tool install pre-commit`, with uv self-bootstrapped from `https://astral.sh/uv/install.sh` (UV_INSTALL_DIR=~/.local/bin) if missing — explicitly not brew. Fixed a PATH gap so uv is callable in the same recipe after a fresh install.
- **Files modified:** Makefile
- **Verification:** `make -n setup-env` parses clean; `uv tool install pre-commit` installs pre-commit 4.6.0 to ~/.local/bin (on PATH).
- **Committed in:** `5d0dfc0`

**2. [Blocking] `.gitignore` pattern `gitid` swallowed the `cmd/gitid/` source directory**
- **Found during:** Task 2 (could not `git add` a test file under `cmd/gitid/` — refused as ignored)
- **Issue:** The unanchored `gitid` pattern (from plan 01-01) matched any path component named `gitid`, including the `cmd/gitid/` **source** directory — a latent footgun that would silently drop new source files there. `cmd/gitid/main.go` survived only because it was committed before the rule.
- **Fix:** Anchored the pattern to `/gitid` (root-level binary only), with an explanatory comment.
- **Files modified:** .gitignore
- **Verification:** `git check-ignore cmd/gitid/badfile.go` → not ignored; `git check-ignore gitid` → still ignored.
- **Committed in:** `4baee40`

---

**Total deviations:** 2 auto-fixed (both blocking, both pre-existing bugs from earlier plans surfaced by this checkpoint)
**Impact on plan:** Both fixes were prerequisites for `make setup-env` to work and for the hooks to be testable. No scope creep — the plan's own artifacts were unchanged in intent.

## Issues Encountered
- **`core.hooksPath` set (repo-local):** pre-commit refused to install ("Cowardly refusing to install hooks with `core.hooksPath` set"). The value was a behavior-neutral artifact of the Wave 1–2 worktree isolation (pointed at the default `.git/hooks`). Resolved with `git config --local --unset-all core.hooksPath`. Not a tracked-file change.

## Verification Evidence (human-verify checkpoint — user approved)
- **Clean tree:** `pre-commit run --all-files` → go-fmt Passed, go-lint Passed.
- **Clean commit:** the `.gitignore` fix commit (`4baee40`) passed through the active hooks and succeeded.
- **Malformed commit BLOCKED:** a bad `cmd/gitid/badfile.go` was rejected — go-fmt reformatted it and go-lint reported `errcheck` (unchecked `os.Open`) + `unused` (`badFn`); HEAD did not advance.
- **Pre-push:** `pre-commit run --hook-stage pre-push --all-files` → go-test (race + coverage) Passed, `coverage.out` produced.

## Next Phase Readiness
- Phase-1 goal met: `make setup-env` on a fresh clone leaves an engineer fully gated (commits and pushes are checked locally).
- Note for future worktree-based execution: `isolation="worktree"` runs may re-set repo-local `core.hooksPath`; if pre-commit install fails later, unset it again.
- Ready for Phase 2 (First Identity End-to-End).

---
*Phase: 01-bootstrap*
*Completed: 2026-06-08*
