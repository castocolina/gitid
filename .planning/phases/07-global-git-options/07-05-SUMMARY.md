---
phase: 07-global-git-options
plan: 05
type: summary
wave: 5
---

# 07-05 Summary — frozen `gitid git` CLI, four JSON envelopes, exit-status table, parity docs, and headless e2e coverage

Every Global Git outcome is on the command line through the same ceremonies the screen uses: a frozen four-command tree, four versioned JSON envelopes emitted on every path, one exit-status table with the process status provably equal to the envelope's own field, a complete parity matrix (no deferred placeholder left for this noun), and the §J requirements correction D-11.1 owes. Tasks 1–3 committed; Task 1 was hand-finished by the orchestrator after a transient network error (see Deviations).

## Deviations

- **D-07-05-1 — Task 1's transient-network-error recovery (summary, full detail in commit `1bb740c`'s message).** The cross-AI session hit a transient network error partway through Task 1's own edits, leaving an interrupted edit with three small bugs behind. The orchestrator hand-finished and committed Task 1 after fixing those three bugs; the resulting tree, ceremonies, R-5 token validation, and adaptive-depth fallback were independently re-verified clean (build/vet/race). This session resumed at Task 2 with only the expected `TestParityMatrixResolvesAndCoversTree` failure (the stale deferred row + unnamed git commands) and did not re-touch Task 1.
- **D-07-05-2 — Task 3's "applies the same key twice … still succeeds" vs. the frozen already-set refusal.** The plan's Task 3 `<behavior>` says a second apply of the same key "still succeeds"; Task 1's own frozen contract (and its acceptance criterion) refuses a row that is already-set, naming the state. The plan contradicts itself; Task 1 is binding. The e2e idempotency case therefore asserts the second apply is a REFUSAL (exit 1, envelope error naming `already-set`) AND — the actual point of the case — that the written baseline file's bytes are identical after the second attempt (`TestGlobalGitCLI_ListApplyIdempotent`). The Task 3 acceptance criterion ("asserts the written file's bytes are identical after the second apply") holds.
- **D-07-05-3 — the schema document's captured example vs. the SUMMARY's verbatim envelope are two different real runs.** `docs/gitid-git-json-schema.md`'s apply example was captured from a unit-level run of the real ceremony (pre-seeded files → populated `backups`). The verbatim envelope below was captured from the e2e suite against the compiled binary with the fake git shim (fresh sandbox → `backups: []`; the substitution advisory identical in both). Both are real captures showing the advisory channel; the e2e one is the shim-driven, process-level proof.
- **D-07-05-4 — the full `make test-e2e` run rewrote committed PTY frame snapshots.** The suite's `capture*Frame` helpers overwrite the committed `ui-frames/*.txt` files on every full run. Most diffs were timestamp/temp-path noise; one was real: the `global-git-version-gate` frame now carries the plan-stage substitution advisory line that Task 1 (T-07-31) added to the ceremony — the committed 07-04-era frame predates it. Refreshed all 15 frames in a dedicated commit (`21aaa51`) so the repo is stable after a full run and the version-gate frame reflects current product output; kept it out of Task 3's commit to preserve that commit's test-only scope.

## Review

- **R-5 (cross-AI review cycle 1 — "`options apply` key identity")**: resolved by the frozen token-only argv surface. `gitid git options apply` resolves argv against `globalgit.Policy`'s frozen `Token` column only, exact and case-sensitive; a member config key is refused naming its owning token. Pinned by `TestGitOptionsApplyAcceptedTokensEqualPolicyTokens` and `TestGitOptionsApplyRefusesMemberKeysNamingOwningToken` (every member key of every multi-key row, plus the lower-cased `init.defaultbranch`).
- **R-1 (CLI half — nothing pre-selected)**: the adaptive-depth fallback opens the Global Git screen with an empty selection, asserted by reading the model's own selection set (`GlobalGitUIState`), not the rendered frame — `TestGitIncompleteBothTTYsOpensEmptyTUI`.
- **D-11.1 (obligation 1 — the §J substrate-note correction)**: landed in Task 2; exact before/after wording recorded below. GGIT-01's requirement-index status was deliberately NOT changed (plan 07-06 owns it).
- **D-04 (explicit set-versus-clear on the CLI)**: `fallback set`'s `--name`/`--email` vs `--clear-name`/`--clear-email`, empty-value and set-plus-clear refusals, all-flags-omitted refusal — landed in Task 1, exercised headlessly by Task 3's `TestGlobalGitCLI_FallbackShowSetClear` (set one half, other half stays unset, clear removes the managed block from the file).

## The final command tree as shipped

```
gitid git options  list                  [--json]
gitid git options  apply  <key>...       [--dry-run] [--yes] [--fail-on-advisory] [--json]
gitid git fallback show                  [--json]
gitid git fallback set    [--name <n>] [--email <e>] [--clear-name] [--clear-email]
                                         [--dry-run] [--yes] [--json]
```

Exactly the plan's `<authority>` block. No extra verbs, no flat root-level aliases. Pinned by `TestGitCmdTreeExactlyFourFrozenPaths`.

## The accepted row-token set as shipped (R-5), and the observed member-key refusal

The argv vocabulary is the policy's frozen `Token` column and nothing else. Observed from the compiled binary's help text:

```
Accepted tokens (exact, case-sensitive): init.defaultBranch, core.ignorecase, core.lineEndings, user.useConfigOnly, push.autoSetupRemote, pull.rebase, fetch.prune, alias, color, merge.conflictstyle, diff.colorMoved.
```

The three prose-display rows are reachable only by their token (`core.lineEndings` for the line-endings pair, `alias`, `color`). A member key is refused, naming the owning token. Observed (exit 1, envelope emitted):

```
$ gitid git options apply alias.lg --yes --json
gitid: "alias.lg" is a member config key; use the row token "alias" instead
```

(the refusal message appears both as the envelope's `error` field and on stderr; `declined: ["alias.lg"]`, `exit_code: 1`).

## The four schema identifiers and each envelope's exact key set

| Identifier | Command | Exact key set |
|---|---|---|
| `gitid.git.options/v1` | `gitid git options list --json` | `schema`, `options` (each option: `key`, `token`, `current_value`, `provenance`, `recommended_value`, `state`, `probe_error`) |
| `gitid.git.apply/v1` | `gitid git options apply --json` | `schema`, `dry_run`, `applied`, `declined`, `target_path`, `backups`, `restored`, `advisories`, `error`, `exit_code` |
| `gitid.git.fallback/v1` | `gitid git fallback show --json` | `schema`, `name`, `email`, `name_status`, `email_status` |
| `gitid.git.fallbackset/v1` | `gitid git fallback set --json` | `schema`, `dry_run`, `set_name`, `set_email`, `cleared_name`, `cleared_email`, `target_path`, `backups`, `restored`, `advisories`, `error`, `exit_code` |

The `state` enum is exactly the render layer's taxonomy — `needs-action`, `already-set`, `differs`, `not-applicable`, `probe-error` — shared string constants used by both the JSON and the refusal messages, pinned by `TestGitJSONStateEnumsAreRenderLayerTaxonomy`. Key sets are pinned by `TestGitJSONEnvelopesExactKeySets` / `TestGitJSONOptionsListExactKeySetAndEnums` / `TestGitJSONFallbackShowExactKeySetAndUnsetStatuses` (an envelope that grows a key silently is a test failure). `exit_code` is produced by the same helper as the process status.

## Observed exit-status table (with the command that produced each row)

| Code | Case | Produced by |
|---|---|---|
| 0 | success, including a success carrying advisories | `gitid git options apply init.defaultBranch --yes` (clean) and `gitid git options apply merge.conflictstyle --yes --json` below the gate (advisories populated, exit 0) |
| 0 | dry run, even with the advisory opt-in | `gitid git options apply init.defaultBranch --dry-run --json` (e2e) and the unit table row `dry-run-advisory-stays-zero` in `TestGitExitCodeEqualsEnvelopeForEveryRow` with `--fail-on-advisory` |
| 1 | refusal: unknown/member/non-selectable token, contradictory flags, missing confirmation | `gitid git options apply alias.lg` (member key); `gitid git options apply init.defaultBranch --yes` on an already-set row (e2e idempotency, second apply); `gitid git fallback set` with no flags |
| 2 | a write attempted and rolled back | unit `TestGitJSONApplyAndFallbackSetEnvelopesOnEveryPath` rolled-back subtests (injected write failure through the lifecycle seam; envelope `restored` populated, exit 2) |
| 3 | the advisory opt-in on a successful advisory-carrying write | `gitid git options apply merge.conflictstyle --yes --fail-on-advisory --json` below the gate (e2e `TestGlobalGitCLI_BelowGateAdvisoryExitCodes`, exit 3) |

Process status == envelope `exit_code` on every row, pinned by the `TestGitExitCodeEqualsEnvelopeForEveryRow` table and asserted structurally in every e2e envelope.

## Captured apply envelope for the below-gate conflict-style write (verbatim)

From `TestGlobalGitCLI_BelowGateAdvisoryExitCodes` — the compiled binary with `FakeGitShimDir(t, "2.34.1", "")` (below the 2.35 hard gate), a fresh sandbox, `--yes --json`. A dry run against the same sandbox was asserted first to carry the substitution advisory, so the case cannot pass vacuously (06-06's recorded trap).

```json
{
  "schema": "gitid.git.apply/v1",
  "dry_run": false,
  "applied": [
    "merge.conflictstyle"
  ],
  "declined": [],
  "target_path": "~/.gitconfig.d/00-baseline",
  "backups": [],
  "restored": [],
  "advisories": [
    "advisory: merge.conflictstyle is below the git version gate — wrote \"diff3\" instead of \"zdiff3\"",
    "advisory: merge.conflictstyle was applied but the effective value is \"diff3\" — a later setting in your config may override it"
  ],
  "error": "",
  "exit_code": 0
}
```

The file-level assertion backs the envelope: the baseline file on disk contains `conflictstyle = diff3` and not `zdiff3` — the fallback value was actually written. With `--fail-on-advisory` the same apply exits 3.

## Parity-matrix rows added

Replaced the `deferred Phase 7` placeholder with one shipped row per outcome (all keyed `GGIT-01, SHELL-03`), in the existing row format:

```
| GGIT-01, SHELL-03 | List the global Git options | `gitid git options list` | `[--json]` | shipped | ...
| GGIT-01, SHELL-03 | Apply named global Git options | `gitid git options apply` | `<key>... [--dry-run] [--yes] [--fail-on-advisory] [--json]` | shipped | ...
| GGIT-01, SHELL-03 | Show the global fallback author | `gitid git fallback show` | `[--json]` | shipped | ...
| GGIT-01, SHELL-03 | Set or clear the global fallback author | `gitid git fallback set` | `--name <n> --email <e> --clear-name --clear-email [--dry-run] [--yes] [--json]` | shipped | ...
```

`rg -n 'deferred Phase 7' docs/cli-parity-matrix.md` returns no match; the two `deferred Phase 8` rows are untouched. The checker passes in BOTH directions — every shipped row resolves to a real writer, every runnable tree command is named by a row — confirmed by `TestParityMatrixResolvesAndCoversTree` under `make test`.

## REQUIREMENTS §J — exact wording before and after (D-11.1)

Before:

> *(GLOBAL-01/GITIGNORE-01/URLRW-01 built as substrate to fold in.)*

After:

> *(Substrate note: GLOBAL-01 is built here and folded in; URLRW-01 is Phase 4's insteadOf rewrite; GITIGNORE-01 belongs to Phase 8's fixer — BOTH `core.excludesfile` AND the managed pattern file are written together there, because a key-only fold-in would leave git silently tolerating a dangling excludesfile (D-11).)*

The note now names the owning phase for each substrate requirement and carries a one-line pointer to D-11's reasoning (key-only fold-in → silently dangling pointer → invisibly broken half-state), so a future reader cannot re-litigate it.

## GGIT-01 requirement-index status — deliberately NOT changed

`.planning/REQUIREMENTS.md`'s index table still reads `| GGIT-01 | Phase 7 | Pending |`. This phase is not complete until plan 07-06 closes, so the status change is deliberately left to **plan 07-06**; this plan touched only the §J substrate note, never the requirement-index status.

## Commits (in order)

1. `1bb740c` `feat(07-05): the frozen gitid git command tree and shared ceremonies` (Task 1, orchestrator hand-finished after the transient network error)
2. `da38cdd` `feat(07-05): four versioned JSON envelopes, exit-status table, and parity docs` (Task 2)
3. `3c0df10` `test(07-05): headless CLI e2e coverage with file-level assertions` (Task 3)
4. `21aaa51` `test(07-05): refresh PTY frames for the plan-stage version-gate advisory` (frame refresh, Deviation D-07-05-4)
5. `(this commit)` `docs(07-05): add plan summary`

## Exit battery results (real output)

- `go build ./...` — clean.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — green (all packages, including `cmd/gitid`).
- `GOLANGCI_LINT_CACHE="$PWD/.golangci-cache" make lint` — 0 issues (plain + `-tags screenshot`), including `go vet -tags e2e ./...`.
- `make test` — green, including `gate-copy-freeze` (all frozen strings present; D-13 / 07-03 / D-09 / result-message exclusions hold) and the parity-matrix checker in both directions.
- `make test-e2e` (build + full `-tags e2e -race ./e2e/...`) — green, 596.7s, including the five new `TestGlobalGitCLI_*` cases and the full PTY suites.
- Task 2 verify: `TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./cmd/gitid/... -run 'GitJSON|GitExitCode|ParityMatrix|GitEnvelope'` — green.
- Task 3 verify: `go test -tags e2e ./e2e -run 'TestGlobalGitCLI' -count=1` — green.
