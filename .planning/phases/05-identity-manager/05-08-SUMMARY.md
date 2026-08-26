---
phase: 05-identity-manager
plan: 08
subsystem: cli
tags: [cli, parity, lifecycle, dry-run]
requires:
  - phase: 05-identity-manager
    provides: [tracer delete path, substrate primitives, rotate/repair ceremonies, everything-scope delete, clone pre-fill, rendered ceremonies, lifecycle chokepoint]
provides:
  - "Adaptive-depth CLI create, clone, rotate, new-key, and delete outcomes over shared lifecycle chokepoints"
  - "Requirement-keyed, machine-checked CLI parity matrix and per-verb dry-run contract"
  - "Compiled CLI/TUI parity e2e coverage with artifact-reference coherence assertions"
affects: [phase-05-ui-gates, phase-06-cli-matrix, phase-07-cli-matrix, phase-08-cli-matrix]
actuals:
  tokens: 31000
  tasks: 3
  commits: 4
tech-stack:
  added: []
  patterns:
    - "CLI handlers are thin adapters over the same lifecycle chokepoints as TUI commit seams"
    - "Adaptive depth requires independent stdin and stdout terminal facts"
    - "Parity is checked structurally through a matrix and normalized artifact comparison plus referential coherence"
key-files:
  created:
    - docs/cli-parity-matrix.md
  modified:
    - cmd/gitid/identity.go
    - cmd/gitid/identity_create.go
    - cmd/gitid/identity_clone.go
    - cmd/gitid/identity_key.go
    - cmd/gitid/identity_delete.go
    - cmd/gitid/identity_test.go
    - internal/tuikit/app.go
    - internal/tuikit/app_test.go
    - e2e/identity_cli_e2e_test.go
    - Makefile
key-decisions:
  - "CLI maps --yes only to confirmationBypassedWithYes; no --yes with both terminals maps to confirmationRequired with a prompt; no --yes without both terminals refuses before lifecycle invocation."
  - "Create and clone use --name, --provider, --git-name, --git-email, --ssh-host, --hostname, --port, --algorithm, --reuse-key, --strategy, --git-dir, --yes, and --dry-run; clone adds --new-key; delete selects exactly one of --git-only or --all."
  - "Rotate/new-key dry runs test only the current key and state the frozen post-rotation limitation."
patterns-established:
  - "Every future product outcome must add a matrix row; every future CLI write handler must reuse its TUI lifecycle chokepoint."
requirements-completed: [MGR-04, MGR-05, MGR-06, KEY-05, KEY-07, SHELL-02, SHELL-03]
duration: 45min
completed: 2026-08-26
status: complete
---

# Phase 05-08: CLI Parity Completion Summary

**Shipped all identity-manager outcomes through adaptive CLI commands that share the TUI lifecycle ceremonies, with mechanically checked parity and compiled end-to-end proof.**

## Performance
- **Duration:** 45min
- **Tasks:** 3
- **Files modified:** 17
- **`make test-e2e` wall-clock:** 318.68s (within the 900s budget and below the 500s projection)

## Accomplishments
- Added adaptive-depth create, clone, rotate, new-key, and completed delete CLI commands with explicit confirmation, unconditional backup, and per-verb dry-run semantics.
- Added `docs/cli-parity-matrix.md` and bidirectional command-tree tests covering shipped identity commands and deferred `ssh`, `git`, `health`, and `fix` nouns.
- Added paired compiled CLI/TUI e2e cases for rotate, new-key, clone, both delete scopes, JSON reads, reference coherence, and post-write re-test failure behavior.

## Task Commits
1. **Task 1: The adaptive-depth resolver and remaining write verbs (D-02)** - `48dca02` (feat)
2. **Task 2: The requirement-keyed parity matrix and its check (D-04)** - `7c97d8d` (feat)
3. **Task 3: Headless CLI end-to-end suite proving outcome parity with the TUI** - `df378c9` (feat)

**Plan metadata:** `docs(05-08): add plan summary`

## Files Created/Modified
- `cmd/gitid/identity.go` - shared depth resolver and CLI confirmation-mode mapping.
- `cmd/gitid/identity_create.go` - complete-flag create and pre-filled wizard launch path.
- `cmd/gitid/identity_clone.go` - clone creation path and normalized provider-derived aliases.
- `cmd/gitid/identity_key.go` - rotate/new-key lifecycle adapters and current-key dry-run caveat.
- `cmd/gitid/identity_delete.go` - explicit delete scope, authorization refusal, and plan rendering.
- `cmd/gitid/identity_test.go` - resolver, lifecycle, dry-run, matrix, and authorization tests.
- `internal/tuikit/app.go` - exported pre-filled application constructor.
- `internal/tuikit/app_test.go` - pre-filled application coverage.
- `docs/cli-parity-matrix.md` - requirement-keyed outcome matrix and dry-run contract.
- `e2e/identity_cli_e2e_test.go` - paired CLI/TUI artifact and coherence tests.
- `Makefile` - matrix test documentation and calibrated 900s e2e timeout.

## Decisions Made
- `--yes` maps exclusively to `confirmationBypassedWithYes`; interactive no-`--yes` uses `confirmationRequired`; non-interactive no-`--yes` refuses before a policy or lifecycle call is created.
- `confirmationAlreadyObtained` remains unreachable from all CLI handler sources.
- The 900s e2e timeout is deliberately sized from the 258.8s pre-Phase-5 baseline and a ~500s projection; the measured 318.68s result does not require Phase 05-09 to revisit the budget.

## Deviations from Plan
- **Rule 1 — cross-cutting parity fix:** markerless reconstructed providers used the short token `github`, causing the headless clone alias to diverge from the TUI's FQDN `github.com` alias. `internal/identity/clone.go` now normalizes through `RewriteProviderKey`; `internal/identity/clone_test.go` proves the behavior. Verified by the Task 3 e2e suite; committed in `df378c9`.
- **Rule 1 — lifecycle fingerprint fix:** headless clone inputs now preserve an explicit algorithm and resolved reuse-key path so the two test-stage fingerprint unlocks the same create transaction. Verified by `TestCloneCeremonyInputsFingerprintsMatchTestStage`; committed in `df378c9`.

## Issues Encountered
- The Task 3 commit hook ran `goimports` and reformatted one e2e line. The amended staged change was re-verified before committing normally.

## Next Phase Readiness
- Phase 05-09 can consume the established CLI/TUI lifecycle parity, matrix, and 900s e2e budget without blockers.
- Future phases adding `ssh`, `git`, `health`, or `fix` outcomes must replace the corresponding deferred matrix rows and preserve bidirectional checker coverage.

---
*Phase: 05-identity-manager*
*Completed: 2026-08-26*
