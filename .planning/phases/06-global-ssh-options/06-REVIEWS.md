---
phase: 6
reviewers: [codex-sol, xai-grok]
reviewed_at: 2026-08-26T20:19:11Z
plans_reviewed:
  - .planning/phases/06-global-ssh-options/06-01-PLAN.md
  - .planning/phases/06-global-ssh-options/06-02-PLAN.md
  - .planning/phases/06-global-ssh-options/06-03-PLAN.md
  - .planning/phases/06-global-ssh-options/06-04-PLAN.md
  - .planning/phases/06-global-ssh-options/06-05-PLAN.md
  - .planning/phases/06-global-ssh-options/06-06-PLAN.md
  - .planning/phases/06-global-ssh-options/06-07-PLAN.md
models:
  codex-sol: "gpt-5.6-sol (reasoning=low)"
  xai-grok: "xai/grok-4.6 (reasoning=low)"
model_sources:
  codex-sol: "banner"
  xai-grok: "pinned"
---

# Cross-AI Plan Review — Phase 6 (Cycle 4)

> Note on `codex-sol`: `.planning/config.json` pins this instance's model to
> `openai/gpt-5.6-sol-fast`, but the Codex CLI on this host authenticates via a
> ChatGPT account, which again rejected that model with `400
> invalid_request_error: "The 'openai/gpt-5.6-sol-fast' model is not supported
> when using Codex with a ChatGPT account."` — the same failure mode as Cycles
> 1, 2, and 3. The review below was produced by re-invoking the `codex` lane
> without the model override, so it ran on the account's default resolved
> model (`gpt-5.6-sol`, reasoning=low). The `review.reviewer_instances` entry
> for `codex-sol` should be corrected (or the account's available model list
> re-checked) before any future review run — this is now four cycles running
> into the same misconfiguration.

This is Cycle 4 — one cycle beyond the normal 3-cycle default, continued
because each cycle showed genuine convergence on well-defined mechanical
findings (14 → 9 → 4 → 1 new), not a stall or an ambiguous judgment call.
Cycle 3 found 1 HIGH (`06-04-PLAN.md` instructed reuse of an unexported,
signature-incompatible `sshconfig.aliasCollides` matcher) plus 3 MEDIUM
(probe-failure assertion over-scoped to all six option rows; a
diamond-shaped Include graph misclassified as a cycle; an unstated mutex
relationship between `txMu` and the pending-migration-plan slot that risked
a self-deadlock). The planner responded by: extracting a new exported
`sshconfig.HostPatternsMatch`/`HostLineMatches` matcher and refactoring
`aliasCollides` onto it; scoping the probe-failure assertion to the four
resolution-dependent rows with a load-bearing counter-fixture; splitting
`BuildGraph`'s cycle detection into an `active` recursion stack and an
`expanded` memo; and adding an explicit `<lock_contract>` section to
`06-05-PLAN.md` with a dedicated `pendingMigrationMu`, a fixed one-way
acquisition order, and an atomic consume-once accessor.

Both reviewers independently verified all four claimed fixes against the
current plan text and the actual source (`internal/sshconfig/validation.go`,
`internal/sshconfig/migrate.go`), not the summary of what changed. Both
confirm all four are real, source-grounded, and compile/behave as claimed —
the Cycle 3 HIGH is FULLY RESOLVED and all three Cycle 3 MEDIUMs are FULLY
RESOLVED. They diverge on one point: **codex-sol found one new MEDIUM** that
the exported-matcher fix's own doc-comment example does not go unnoticed by
xai-grok either, but xai-grok folds it into a non-blocking LOW alongside
codex-sol — both reviewers actually agree on that LOW. The substantive
divergence is codex-sol's separate, source-grounded new MEDIUM about
`06-05-PLAN.md`'s preview path (`SSHStorageMigrationPlan`) not being
serialized against the migration engine's two-file write. The orchestrator
independently verified this claim against `internal/sshconfig/migrate.go`
and `06-05-PLAN.md`'s current text; see "Orchestrator Verification" below.

## Codex Review (codex-sol)

# Phase 6 Cycle 4 Plan Review

## Summary

The four Cycle 3 fixes are substantially incorporated. The probe-failure scope and diamond/cycle distinction are explicit and regression-tested. The matcher extraction is structurally correct, although one type-conversion detail should be pinned. The new pending-plan mutex removes the original self-deadlock, but the storage preview itself remains outside `txMu`; it can read the two migration files while another transaction is between its two writes. The proposed concurrency test checks deadlock and Go data races, but not filesystem snapshot consistency. That is one actionable MEDIUM issue, so I would run one focused revision rather than close convergence now.

## Strengths

- The matcher analysis accurately reflects current source:
  - `HostMatch` handles one pattern token in [validation.go:100](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/validation.go:100).
  - `AliasCollision` is only the exported path-based wrapper, while `aliasCollides` is unexported and performs file parsing and Include traversal in [validation.go:153](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/validation.go:153).
  - The stanza-level positive/negative rule really is inline at [validation.go:198](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/validation.go:198).
  - Plan 06-04 now includes both `validation.go` and `validation_test.go`, exports the required matcher, and explicitly moves `aliasCollides` onto it in [06-04-PLAN.md:147](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:147) and [06-04-PLAN.md:169](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:169). Existing `AliasCollision` tests are retained as the regression gate. This resolves the Cycle 3 HIGH at the architectural level.
- The probe-failure correction is real rather than aspirational. Plan 06-03 defines the evidence dependency map, restricts degradation to the four resolution-dependent options, and requires a fixture where a file-derived option remains `StateAlreadySet` despite resolution-probe failure. See [06-03-PLAN.md:153](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:153) and [06-03-PLAN.md:178](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:178). That test would catch regression to the former all-six assertion.
- The diamond/cycle correction is complete in the plan: `active` is a pushed-and-popped recursion stack, `expanded` is a persistent discovery memo, linearization deliberately does not deduplicate repeated diamond occurrences, and the acceptance test pairs a true cycle with an entry→{a,b}→shared diamond and requires the latter to remain conclusive. These mechanisms appear in [06-04-PLAN.md:264](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:264). The plan also correctly warns against copying the existing global `seen` behavior at [validation.go:164](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/validation.go:164).
- The dedicated `pendingMigrationMu` genuinely removes the previously identified reentrant-lock deadlock: `txMu` and `pendingMigrationMu` get separate ownership, a fixed one-way acquisition order, a ban on holding the slot lock across I/O, and mandatory access through atomic `putPendingMigration`/`takePendingMigration`. See [06-05-PLAN.md:108](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:108); the commit path follows that order at [06-05-PLAN.md:296](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:296), with a construction check prohibiting direct field access.
- Plans 06-01 and 06-02 still establish a coherent dependency base: one globals renderer/write authority first, then registry and migration classification. Plans 06-06 and 06-07 remain correctly downstream and do not reintroduce separate write paths.

## Concerns

- **MEDIUM — Storage preview planning is not serialized with filesystem transactions.** `SSHStorageMigrationPlan` is instructed to resolve storage and run `sshconfig.PlanMigration`, then acquire only `pendingMigrationMu` to store the result ([06-05-PLAN.md:306](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:306)). By contrast, `runSSHStorageMigrate` holds `txMu` across the whole transaction ([06-05-PLAN.md:296](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:296)). The current migration engine writes destination and source as separate steps — destination at [migrate.go:287](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:287), then source at [migrate.go:304](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:304). A concurrent preview can therefore read one file after the destination write and the other before the source trim, producing a plan from an intermediate cross-file state. The planned concurrency test only requires no Go race and no deadlock ([06-05-PLAN.md:339](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:339)). Filesystem I/O has no Go data race, so that test can pass while the preview is inconsistent.
- **LOW — The matcher extraction leaves the existing caller's type projection implicit.** The proposed API accepts `[]string`, while current `host.Patterns` is iterated as parser pattern objects whose values are obtained through `pat.String()` ([validation.go:190](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/validation.go:190)). Plan 06-04 says to replace the inline loop with `HostPatternsMatch`, but does not explicitly say to project `host.Patterns` to `[]string` first ([06-04-PLAN.md:159](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:159)). An experienced executor will infer the conversion, and the existing regression suite should expose mistakes, so this is not a design blocker.

## Suggestions

- Amend Plan 06-05 so `SSHStorageMigrationPlan` acquires `txMu` before resolving layout and calling `PlanMigration`, then calls `putPendingMigration` while retaining the documented `txMu` → `pendingMigrationMu` order. Release both before returning to the UI.
- Strengthen its concurrency test with an injected pause between destination write and source trim: start a migration and stop after the destination write; start `SSHStorageMigrationPlan`; assert planning blocks until the migration finishes; assert the resulting plan's `SourceBefore`/`DestBefore` describe the final coherent layout, not the intermediate duplicate-block state.
- In Plan 06-04, explicitly specify the `host.Patterns` → `[]string` conversion (via `pat.String()`) before the `HostPatternsMatch` call, so the extraction is executable without interpretation.
- Keep the existing Cycle 3 tests unchanged: the four-row probe failure case, load-bearing independent-evidence row, true-cycle/diamond pair, consume-once token test, bogus-token preservation test, and direct-field-access construction check are all valuable.

## Per-plan Assessment

- **06-01:** Strong but large tracer. No Cycle 4 regression found.
- **06-02:** Correct dependency placement for registry and migration classification. No new issue found.
- **06-03:** Cycle 3 probe-scoping fix confirmed.
- **06-04:** Diamond/cycle fix confirmed; matcher extraction needs the small type-projection clarification.
- **06-05:** Original mutex deadlock is fixed, but preview planning needs `txMu` serialization.
- **06-06:** Frozen CLI, JSON, and exit contracts remain coherent with the shared lifecycle functions.
- **06-07:** Visual evidence and closure packet remain appropriately downstream; no new issue found.

## Risk Assessment

**MEDIUM.** Most previously identified risks have converged, and no HIGH issue remains. However, an interactive preview can still be computed from an intermediate two-file migration state. Because this affects the truthfulness of a user-confirmed mutation preview, it should be fixed and pinned before execution. After that focused correction and the matcher conversion clarification, the plans appear ready for final convergence.

---

## OpenCode Review (xai-grok)

Verifying cycle-3 claims against source, then reviewing the plans.

# Phase 6 plan review — Cycle 4

Verified against current source (`internal/sshconfig/validation.go`, `migrate.go`, `cmd/gitid/wiring.go`). Plans are not yet implemented; claims were checked as "would an executor following the text compile and preserve behavior?"

## Summary

Cycle-3's four fixes are real in the plan text and match the code they cite. `HostPatternsMatch` extraction lines up with `aliasCollides`'s unexported loop at `validation.go:198-216`, `AliasCollision` at `:160`, and `HostMatch` at `:103`. The lock contract names two mutexes and forbids the deadlock path. Probe-failure scoping and diamond-vs-cycle are pinned by tests that would fail the old rules. I found **no new HIGH or actionable MEDIUM**. This should be the last review cycle.

## Strengths

- **Matcher extraction is executable.** `aliasCollides` is unexported (`validation.go:164`); `AliasCollision` is path-shaped (`:160`); stanza negation is inline (`:198-216`); `HostMatch` is one token (`:103`). Exporting `HostPatternsMatch`/`HostLineMatches`, refactoring the loop onto them, and keeping `AliasCollision` tests unmodified is the only compile-and-preserve path.
- **Lock contract is mechanically sound.** `runSSHStorageMigrate` holding `txMu` cannot re-lock it; a dedicated `pendingMigrationMu` plus atomic `takePendingMigration` (never takes `txMu`) removes the cycle-3 deadlock. Acquisition order is one-way.
- **Probe-failure and diamond tests are load-bearing**, not restated intent: four named resolution-dependent rows; at least one file-derived row stays `StateAlreadySet`; diamond asserts `Inconclusive == false` and `shared.config` once.
- **Source still shows the migration trap 06-02 exists to close:** `movableBlockNames` at `migrate.go:423-427` skips `globalBlockName` **and** `IsReservedBlockName`. Registry + classification in one wave is still required.
- Precedence pairs (unit + PTY), one write authority, plan-object + digests, frozen CLI contract, HTML non-applicability are intact from earlier cycles.

## Concerns

None new at HIGH or MEDIUM.

**LOW (non-blocking):** `HostPatternsMatch([]string)` vs `aliasCollides`'s `[]*ssh_config.Pattern` needs an on-the-spot conversion (`pat.String()`, strip `!`). The plan implies it via "replace the inline loop"; an executor who calls `HostPatternsMatch` without collecting strings would not compile. The existing `AliasCollision` suite is the right gate; no extra plan text required if execution follows the loop extraction literally.

Cycle 1–3 items that remain **owned, not regressed:** globals exclusion (`migrate.go:426`), `RenderGlobalBlock` still present until 06-01, Wave-1 size, lossy in-block comments, Include-unaware static naming.

## Suggestions

- Executor: convert `host.Patterns` to `[]string` (with `!`) then call `HostPatternsMatch`; do not pass kevinburke types across the new API.
- Quote the `rg` of `putPendingMigration`/`takePendingMigration` in `06-05-SUMMARY.md` as specified; that is the deadlock proof, not the concurrent test alone.

## Risk Assessment

**LOW.** Remaining risk is execution size (Wave 1 ~35 files, three staged commits) and reversible-but-costly on-disk contracts (block rename, migration classification, public CLI schema) — all already classified. No unclosed mechanical defect that would justify Cycle 5.

**Cycle-3 claim check**

| Claim | Verdict |
|---|---|
| `HostPatternsMatch` extraction compiles vs callers | **Yes** — matches `validation.go:103,160,164,198-216`; `validation.go` is on 06-04's file list |
| `<lock_contract>` eliminates deadlock | **Yes** — dedicated mutex, never held across I/O, consume-once helper never takes `txMu`; no leftover "same mutex discipline" contradiction in the current 06-05 text |
| Probe-failure scoping | **Yes** — four named rows + already-set file-derived fixture |
| Diamond vs cycle | **Yes** — two-set rule + diamond AC that a single `seen` map fails |

Cross-AI reviews remain an orchestrator obligation after 06-07's packet; that is already stated.

---

## Orchestrator Verification

Both reviewers agree, independently and with matching `file:line` citations against `internal/sshconfig/validation.go`, that all four Cycle 3 fixes are real: the `HostPatternsMatch`/`HostLineMatches` extraction is executable and preserves `AliasCollision`'s regression gate; the `<lock_contract>` in `06-05-PLAN.md` genuinely eliminates the reentrant-lock deadlock rather than relocating it (dedicated `pendingMigrationMu`, one-way acquisition order, atomic consume-once accessor that never takes `txMu`); the probe-failure assertion is correctly scoped to the four resolution-dependent rows with a counter-fixture that would fail the old all-six assertion; and the diamond/cycle split (`active` recursion stack vs `expanded` memo) is pinned by a diamond-graph acceptance test the old single-`seen`-set rule would fail. **All four Cycle 3 findings (1 HIGH + 3 MEDIUM) are FULLY RESOLVED.**

Both reviewers also independently flag the same LOW: the `HostPatternsMatch([]string)` extraction requires converting `host.Patterns` (`[]*ssh_config.Pattern`) to `[]string` via `pat.String()` before the call, and `06-04-PLAN.md:159` states the replacement without spelling out that conversion. Both reviewers agree the existing `AliasCollision` regression suite would catch a mistake here and neither treats it as blocking — this is a genuine but non-actionable-at-MEDIUM LOW; a one-line addition to the plan removes ambiguity but its absence would not derail execution.

**codex-sol's new MEDIUM — independently confirmed real.** `06-05-PLAN.md:306` (the `SSHStorageMigrationPlan` preview method) acquires only `pendingMigrationMu` to store its computed plan; it does not take `txMu`. `06-05-PLAN.md:296` (`runSSHStorageMigrate`, the commit path) DOES take `txMu` for the whole transaction. `internal/sshconfig/migrate.go` confirms the underlying engine writes the two migration-target files as two SEPARATE, sequential steps — destination first (`migrate.go:270-299`, the "add-before-remove" write), source second (`migrate.go:304-320`, the trim). Nothing in the migration engine makes those two writes atomic across files. A concurrent `SSHStorageMigrationPlan` call — reachable whenever the TUI is re-entered or refreshed while another goroutine's `runSSHStorageMigrate` is mid-transaction — is not blocked by any lock the plan specifies, and can therefore call `sshconfig.PlanMigration` (which reads both files fresh) between the destination write and the source trim, producing a preview view built from a transient, self-contradictory on-disk state (the moved directive present in both files at once). The plan's own concurrency acceptance criterion at `06-05-PLAN.md:339` asserts only "no data race, bounded timeout" under `go test -race` — Go's race detector instruments memory accesses, not filesystem I/O, so it cannot see this. This is a genuine, source-grounded, NEW finding this cycle, not a restatement of the already-resolved Cycle 3 deadlock MEDIUM (that one was about `pendingMigrationMu` vs `txMu` reentrancy; this one is about `SSHStorageMigrationPlan` never taking `txMu` at all).

Severity assessment: MEDIUM, matching codex-sol's classification. It is a correctness defect in a **user-facing preview**, not in the actual committed write — `MigrateWithPlan`'s digest check re-reads and re-verifies both files against the plan's `Digests` immediately before any backup, so a stale/inconsistent preview cannot silently commit inconsistent bytes; at worst the user sees a transient, momentarily wrong diff and, if they confirm against it, gets a same-cycle `ErrConfigChangedSincePreview` refusal rather than a corrupted write. The window is also narrow (the two writes in `migrate.go` are back-to-back with no I/O between them) and requires a second concurrent caller in a single-user TUI, which is possible (e.g., two terminal sessions against the same home) but not the primary usage pattern. It is real and should be fixed, but it does not rise to HIGH because no incorrect state is ever persisted to disk.

## Consensus Summary

Both reviewers agree, independently and with matching `file:line` citations, that all four Cycle 3 findings (the `06-04-PLAN.md` unexported-matcher HIGH, plus the three Cycle 3 MEDIUMs — probe-failure over-scoping, diamond/cycle conflation, unstated mutex contract) are FULLY RESOLVED in the current plan text with real, verified mechanism changes rather than reworded promises. Both also independently flag the same non-blocking LOW (the `host.Patterns` → `[]string` conversion is implicit rather than spelled out in `06-04-PLAN.md`).

They diverge on one point: codex-sol raises a new MEDIUM — `06-05-PLAN.md`'s preview path (`SSHStorageMigrationPlan`) is not serialized against the migration engine's two-file write, so a concurrent preview can read a transient cross-file state between the destination write and the source trim, and the plan's own `-race`-based concurrency test cannot catch this because it is a filesystem consistency issue, not a Go memory race. xai-grok's review does not surface this. The orchestrator independently traced the claim against `internal/sshconfig/migrate.go`'s two-step write (`:270-299` destination, `:304-320` source) and `06-05-PLAN.md`'s current lock usage (`:296` `runSSHStorageMigrate` takes `txMu`; `:306` `SSHStorageMigrationPlan` does not) and confirms it is real. It does not rise to HIGH because `MigrateWithPlan`'s digest re-verification prevents any inconsistent state from actually being committed — the exposure is a transient, momentarily-wrong preview render, not data corruption — but it is carried forward as an unresolved, actionable MEDIUM.

### Agreed Strengths
- The `HostPatternsMatch`/`HostLineMatches` extraction is executable, matches the cited source lines exactly, refactors `aliasCollides` onto the shared implementation, and keeps the existing `AliasCollision` regression suite as an unmodified gate.
- The `<lock_contract>` in `06-05-PLAN.md` genuinely eliminates the Cycle 3 reentrant-lock deadlock: a dedicated `pendingMigrationMu`, a fixed one-way acquisition order (`txMu` then `pendingMigrationMu`, never reversed), a ban on holding the plan-slot lock across I/O, and an atomic consume-once accessor that never takes `txMu`.
- The probe-failure assertion is correctly scoped to the four resolution-dependent option rows, with a counter-fixture proving the scoping is load-bearing (not an incidental pass).
- The diamond-vs-cycle split (`active` recursion stack vs `expanded` memo) is pinned by a diamond-graph acceptance test that the old single-`seen`-set rule would fail.

### Agreed Concerns
- LOW (both reviewers, non-blocking) — `06-04-PLAN.md:159` does not spell out the `host.Patterns` (`[]*ssh_config.Pattern`) → `[]string` conversion needed before calling `HostPatternsMatch`; an executor following the extraction loop literally will still need to infer `pat.String()` per token. The existing `AliasCollision` suite would catch a mistake here.

### Divergent Views — resolved by orchestrator verification
- **`06-05-PLAN.md` preview/commit filesystem-consistency gap**: codex-sol MEDIUM, xai-grok not flagged. **Orchestrator confirms codex-sol is correct** — `SSHStorageMigrationPlan` (`06-05-PLAN.md:306`) takes only `pendingMigrationMu`, `runSSHStorageMigrate` (`:296`) takes `txMu`, and the migration engine's two-file write (`migrate.go:270-320`) is not atomic across files, so a concurrent preview can read a transient cross-file state that the plan's `-race`-only concurrency test cannot detect. Not HIGH: `MigrateWithPlan`'s pre-backup digest re-check prevents this from ever reaching a committed write.

### New Actionable (non-HIGH) Findings This Cycle
- MEDIUM — `06-05-PLAN.md`'s `SSHStorageMigrationPlan` preview path does not acquire `txMu` (or any lock the migration engine's writer respects), so it can read `sshconfig.PlanMigration`'s two source files between the engine's sequential destination-write and source-trim steps (`migrate.go:270-299` vs `:304-320`), producing a preview from a transient, self-contradictory on-disk state. The plan's concurrency acceptance criterion (`06-05-PLAN.md:339`) only asserts no Go data race under `-race`, which does not exercise filesystem-level consistency. Needs: `SSHStorageMigrationPlan` to acquire `txMu` for the resolve-and-plan step (release before returning to the UI, per the documented `txMu` → `pendingMigrationMu` order), and a strengthened concurrency test that injects a pause between the destination write and the source trim and asserts a concurrent preview either blocks or reflects the final coherent layout, not the intermediate one.
- LOW — `06-04-PLAN.md:159` should explicitly state the `host.Patterns` → `[]string` conversion (via `pat.String()`, stripping a leading `!`) before the `HostPatternsMatch` call, so the matcher-extraction instruction is one mechanical step closer to compiling without executor inference. Low priority: the existing `AliasCollision` regression suite is the safety net either way.
