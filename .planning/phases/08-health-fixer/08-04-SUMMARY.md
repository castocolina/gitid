# 08-04 SUMMARY — Baseline global gitignore pair and informational overrides

## Outcome

All three tasks are complete. `CheckBaseline` now detects the global gitignore as a pair: an unset `core.excludesfile` with no managed pattern file is a fixable Git/Baseline warning; a configured key whose target is absent or lacks the managed default patterns is a fixable error; a correct key plus managed default-pattern file produces no pair finding. The fix is injected through `doctor.Deps.FixExcludesfile`; the checks layer does not call the mutating gitconfig writer.

Baseline values that differ from `internal/globalgit.Policy` are now report-only `SeverityInfo` findings with no fix. The check reuses the Phase 7 policy table, so every policy member is covered without duplicating the recommendation list.

## Task 1 — Gitignore pair check

- Added `Deps.FixExcludesfile func(path string) error` beside the existing injected doctor effects.
- `CheckBaseline` reads `core.excludesfile` through `RunGitConfigGet` and checks the resolved target through the injected `Stat` seam.
- Added table-driven coverage for unset, dangling, and correct pair states plus a test proving the descriptor calls only the injected effect.
- Manual fresh-home validation built the real binary and ran `HOME=<fresh-home> gitid health --json`. It produced the existing actionable Baseline error and the new warning, never a critical/paused finding.

## Task 2 — Set-differs information cap

- Reused `globalgit.Policy` to enumerate covered baseline keys.
- Every explicitly set, non-recommended member produces a Git/Baseline `SeverityInfo` finding with `Fix: nil`.
- A table-driven hard-cap test exercises every policy member and fails if any expected differs finding is absent or exceeds info severity.

## Task 3 — Real wiring and regression tests

- `buildDoctorDeps` now supplies the real `FixExcludesfile` closure.
- The closure writes the managed gitignore via `gitconfig.WriteGlobalGitignore(path, gitconfig.DefaultGitignorePatterns())`, then writes `core.excludesfile` into the fixture global gitconfig via an argument-slice `git config --file` invocation.
- Real CLI tests prove `gitid health --json` reports both the missing pair and an `init.defaultBranch=trunk` override, while `gitid fix --yes` writes both the config key and managed pattern file.

## Bug found and fixed

The first full race-suite run exposed two real test assumptions invalidated by the new behavior: the curated-pattern test selected the new pair finding because it matched the generic word "gitignore", and the no-fixable-findings fixture seeded the baseline but not the newly-required pair. Both tests failed before correction and pass afterward. The curated test now identifies only its own title, and the no-fixable fixture explicitly seeds the gitignore pair.

## Verification

- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./internal/doctor/... ./internal/doctor/checks/... -run 'TestCheckBaselineGitignore|TestFixExcludesfile'` — 5 passed in 2 packages.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./internal/doctor/checks/... -run 'TestCheckBaselineSetDiffers|TestSeverityNeverEscalates'` — 24 passed in 1 package.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./cmd/gitid/... -run 'TestBaselineGitignoreRealWiring|TestSetDiffersRealWiring|TestFixExcludesfileRealWiring|TestBaselineGitignoreFixViaCLI'` — 4 passed in 1 package.
- `go build ./...` — exit 0.
- `go vet -tags e2e ./...` — exit 0.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — 2114 passed in 21 packages.
- `GOLANGCI_LINT_CACHE="$PWD/.golangci-cache" make lint` — 0 issues; worktree-local cache removed afterward.
- `make gate-visual-regression` — PASS; 41 required frames checked.
- `make test` — PASS, including `gate-copy-freeze`.
- `make test-e2e` — PASS: `ok github.com/castocolina/gitid/e2e 598.327s`.

The visual and E2E gates rewrote old UI frame snapshots only because the new finding changes fixture totals. Inspected diffs were real count-only changes outside this plan's render scope, so all snapshot files were reverted before commit.
