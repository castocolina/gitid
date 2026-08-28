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

## Post-executor findings (orchestrator independent verification)

Independent verification went beyond the executor's own gates (which never
actually round-tripped "apply the fix, then re-scan and confirm
convergence" — only that the fix WRITES something and that findings render)
and found **two real, connected bugs** in the executor's implementation,
both verified empirically (real fixture before/after) and fixed:

1. **Read/write location mismatch (non-convergence).** `CheckBaseline`'s
   gitignore-pair check read `core.excludesfile` via
   `RunGitConfigGet(d.GitconfigPath, "core.excludesfile")` — `git config
   --file <path> <key>` does **not** follow `[include]` directives when
   resolving a key (verified directly: `git config --file ~/.gitconfig
   core.excludesfile` exits 1 even when the key is set inside a fragment
   `~/.gitconfig` includes). Since gitid's own baseline setup always places
   `core.excludesfile` inside the included fragment (never directly in
   `~/.gitconfig`), this made the check report "not configured" for EVERY
   correctly-configured baseline — a false positive of exactly the class
   this whole phase exists to close, and a straight regression of the
   check's OWN pre-Wave-4 implementation, which already correctly read
   `state.BaselineKeys["core.excludesfile"]` (parsed from the fragment's
   own block body by `gitconfig.ReadBaselineState`). Fixed: `CheckBaseline`
   reads from `state.BaselineKeys` again; `checkGitConfig`'s ERROR branch
   narrowed to `!fileExists` only (a genuinely dangling pointer) — the
   original code ALSO escalated a merely-incomplete-content file to the
   same "missing" ERROR, double-reporting the same condition Check 4
   ("curated entries missing", WARNING) already covers correctly.
2. **Fix write lands in the wrong file, and outside the managed block.**
   `Deps.FixExcludesfile`'s real implementation wrote `core.excludesfile`
   via a bare `git config --file gitconfigPath core.excludesfile <path>` —
   this would (a) land the write in `~/.gitconfig`, the file the corrected
   check no longer reads from, so the fix could never converge, and (b)
   even redirected to the fragment, a bare `git config --file --set` writes
   a PLAIN, unmanaged directive appended after the file's content —
   OUTSIDE the `# BEGIN/END gitid managed: baseline` sentinels, invisible
   to `parseGitconfigBlockBody`. Fixed: `fixExcludesfile` now patches the
   EXISTING "baseline" managed block's body in place (insert-or-replace
   the `excludesfile` line under `[core]`, mirroring
   `gitconfig.RenderBaselineBlock`'s own Tier-1 key ordering), preserving
   every other line — including the user's own Tier-2 choices
   (`init.defaultBranch`, a custom `[alias]` section) — byte-for-byte, via
   `filewriter.ListBlocks`/`ReplaceBlock`/`Write`, the same chokepoint
   every other managed-block mutation in this codebase uses. A naive
   `gitconfig.WriteBaselineFile` re-render was considered and rejected: it
   would reset the user's OTHER Tier-2 settings to defaults, a severe
   regression the "set, differs" check (Task 2) explicitly promises never
   to happen ("gitid will not override it").

Verified via a full apply→re-scan round trip against a real fixture
(`TestBaselineGitignoreFixPreservesOtherBaselineSettings`,
`cmd/gitid/fix_test.go`): a baseline fragment with a custom
`init.defaultBranch` and a hand-written `[alias]` section, missing only
`core.excludesfile` — after `gitid fix --yes`, both the custom branch name
and the alias section survive byte-for-byte, `core.excludesfile` is
correctly patched into the fragment's own block body, and a fresh doctor
re-scan produces zero Baseline findings. `TestBaselineGitignoreFixViaCLI`
and `TestCheckBaselineGitignorePair`/`TestFixExcludesfileCallsInjectedEffect`
updated to match the corrected read location and severity mapping. Full
gate battery re-run after both fixes: `go build`, `TERM=dumb
SSH_AUTH_SOCK= go test -count=1 -race ./...` (2115 passed), `make lint` (0
issues), `make gate-visual-regression` (PASS, 57.8s).
