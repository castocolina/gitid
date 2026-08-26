---
phase: 6
reviewers: [codex-sol, xai-grok]
reviewed_at: 2026-08-26T15:52:00Z
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

# Cross-AI Plan Review — Phase 6 (Cycle 3)

> Note on `codex-sol`: `.planning/config.json` pins this instance's model to
> `openai/gpt-5.6-sol-fast`, but the Codex CLI on this host authenticates via a
> ChatGPT account, which again rejected that model with `400
> invalid_request_error: "The 'openai/gpt-5.6-sol-fast' model is not supported
> when using Codex with a ChatGPT account."` — same failure mode as Cycles 1
> and 2. The review below was produced by re-invoking the `codex` lane without
> the model override, so it ran on the account's default resolved model
> (`gpt-5.6-sol`, reasoning=low). The `review.reviewer_instances` entry for
> `codex-sol` should be corrected (or the account's available model list
> re-checked) before the next review run.

This is Cycle 3 — the final allowed cycle before escalation. Cycle 2 found 3
HIGH findings (all confirmed by orchestrator verification: `06-04`'s
shadowing-precedence reversal, `06-03`'s `stateFor` baseline-ordering
self-contradiction, `06-05`'s preview/commit plan-identity TOCTOU gap) plus 6
actionable MEDIUM/LOW findings. The planner revised the plan text to address
all 9 (shadowing-precedence algorithm rewrite, `stateFor` branch reorder, a
single digest-bearing `MigrationPlan` object carried preview-to-commit via an
opaque token).

Both reviewers were asked to independently verify each claimed fix against
the current plan text and the actual source, not to trust the summary of
what changed. **The orchestrator (this session) additionally re-verified the
one point where the two reviewers diverged** — codex-sol's new HIGH finding
that `06-04-PLAN.md` requires calling an unexported cross-package matcher —
directly against `internal/sshconfig/validation.go` and `06-04-PLAN.md`'s
file list. See "Orchestrator Verification" below.

## Codex Review (codex-sol)

# Cycle 3 Plan Review

## Summary

The three Cycle-2 HIGH findings are substantively corrected in the current plans:

- The shadowing model now follows OpenSSH's first-obtained-value rule and includes paired positive/negative tests.
- `stateFor` now checks recommendation equality before source classification.
- Migration preview and commit now share one digest-bearing `MigrationPlan`, carried through an opaque token and verified before backup.

The six actionable non-HIGH findings are also represented with concrete mechanisms and acceptance tests. However, Cycle 3 exposes one new implementation blocker in Plan 06-04: it requires `internal/globalssh` to reuse the unexported `sshconfig.aliasCollides` matcher without modifying or exporting anything from `validation.go`. That mechanism cannot compile as written. There are also smaller inconsistencies in probe-failure acceptance criteria and concurrency ownership around the pending migration plan. Overall risk remains MEDIUM until the matcher/API issue is resolved.

## Strengths

- The shadowing correction is real and directionally precise. [06-04-PLAN.md:99](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:99) states first-obtained-value as binding, then correctly distinguishes: main-file directive below the floored Include (no shadowing), above the Include (shadowing), an earlier-sorting included file (shadowing), an earlier matching stanza in the same file (shadowing). This agrees with the existing floor behavior in [include.go:19](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/include.go:19) and [EnsureIncludeLine, include.go:217](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/include.go:217).
- The culprit-selection correction is also real: the revised plan linearizes files around each Include, preserves true line offsets, filters nonmatching host patterns, and returns the first surviving hit ([06-04-PLAN.md:221](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:221)) — this directly fixes the earlier "last hit" mistake.
- Recursive Include handling is genuinely incorporated: [06-04-PLAN.md:204](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:204) specifies recursive discovery, cycle detection, a depth cap, rewriting Includes in every mirrored file, and inconclusive results when fidelity cannot be proven, improving on current single-hop discovery in [include.go:183](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/include.go:183).
- The `stateFor` fix is explicit and testable: the branch order at [06-03-PLAN.md:137](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:137) is now (1) platform applicability, (2) missing value, (3) value equals recommendation, (4) unequal baseline value, (5) unequal explicitly-sourced value. The exact `ForwardAgent=no` baseline case and equality across every `SourceClass` are pinned at [06-03-PLAN.md:170](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:170). This closes the false-alarm defect rather than merely changing copy.
- The migration TOCTOU fix is real and complete in design: [06-05-PLAN.md:147](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:147) defines a digest-bearing `MigrationPlan`; [06-05-PLAN.md:169](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:169) makes `MigrateWithPlan` verify those digests before backup and write the plan's bytes verbatim. The plan explicitly tests the human-shaped preview window ([06-05-PLAN.md:194](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:194): plan → external edit → commit, `ErrConfigChangedSincePreview`, no backup, no writes). The opaque-token bridge preserves package boundaries ([06-05-PLAN.md:261](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:261)).
- Cycle-2's remaining actionable findings are incorporated: apply rollback is consistently `mutationJournal` around `filewriter.Write` ([06-01-PLAN.md:242](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-01-PLAN.md:242)); production-only `rg` scopes avoid archived-source false positives ([06-01-PLAN.md:298](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-01-PLAN.md:298)); the `IgnoreUnknown` visual divergence is explicitly assigned to `T-06-GLOBALBLOCK` ([06-07-PLAN.md:116](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-07-PLAN.md:116)); both write commands have frozen JSON envelopes ([06-06-PLAN.md:145](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-06-PLAN.md:145)); adaptive-depth behavior is frozen per verb ([06-06-PLAN.md:93](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-06-PLAN.md:93)).

## Concerns

- **HIGH — Plan 06-04 requires calling an unexported matcher across package boundaries.** The plan tells `internal/globalssh/shadow.go` to reuse `sshconfig.aliasCollides` at [06-04-PLAN.md:121](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:121) and again at [06-04-PLAN.md:226](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:226). But `aliasCollides` is unexported in [validation.go:164](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/validation.go:164), and its exported wrapper `AliasCollision` accepts a config PATH (it reads and parses a whole file itself) rather than an individual host-pattern list already extracted by the scanner. Plan 06-04 does not list `validation.go` among files to modify. The executor must either duplicate the negation-aware matching logic — expressly forbidden by the plan's own wording — or deviate from the plan as written.
- **MEDIUM — The probe-failure acceptance criteria can contradict the independent-evidence requirement.** Plan 06-03 correctly says UseKeychain and IdentitiesOnly remain unchanged when the resolution probe fails ([06-03-PLAN.md:177](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:177)). The next criterion says none of all SIX rows is `StateAlreadySet` under that failure ([06-03-PLAN.md:179](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:179)). If either independently-derived row (UseKeychain/IdentitiesOnly) is already-set in the all-success fixture, both conditions cannot hold simultaneously. The latter assertion should cover only the four resolution-dependent rows.
- **MEDIUM — `BuildGraph`'s cycle rule conflates cycles with repeated DAG references.** [06-04-PLAN.md:204](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:204) says to "track visited absolute paths" and report a cycle when revisited. A file included from two separate branches (a diamond-shaped Include graph) is a repeated node, not necessarily a recursion cycle. A global visited set would produce an unnecessary INCONCLUSIVE result on a legitimate, non-cyclic layout. The current recursive implementation in [validation.go:164](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/validation.go:164) has the same limitation, so copying it verbatim would preserve that false positive rather than fix it.
- **MEDIUM — Pending-plan lock ownership is underspecified.** [06-05-PLAN.md:263](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:263) says `runSSHStorageMigrate` takes `txMu`, while [06-05-PLAN.md:274](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:274) says `pendingMigration` is protected by "the same mutex discipline." If that means `txMu` itself, looking up or consuming the pending plan through a helper that also locks it can deadlock (Go's `sync.Mutex` is not reentrant). If it means a separate mutex, the token lookup/invalidation ordering relative to `txMu` should be stated explicitly so the executor does not have to guess.
- **LOW — Some source citations in the plan are descriptive rather than operative.** Plan 06-04 cites [renderer.go:112](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/renderer.go:112) as evidence that the global block is placed last, but those lines only document the requirement; actual placement comes from call order in [writer.go:47](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/writer.go:47) and `reorderGlobalLast` in [migrate.go:473](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:473). Does not invalidate the corrected model; the evidence citation should point to operative code.
- **LOW — The plan set remains very large for a single phase.** Plans 06-01, 06-03, 06-04, and 06-05 each span many packages with low-confidence estimates. Sequential waves reduce merge risk, but execution risk remains meaningful because intermediate refactors alter core SSH mutation and migration contracts before the final full battery.

## Suggestions

- Export a narrow pure matcher from `internal/sshconfig`, e.g. `func HostPatternsMatch(patterns []string, candidate string) bool`, add `validation.go` and its tests to Plan 06-04's file list, and have both `AliasCollision` and `shadowSourceFor` use it so negation behavior has one implementation.
- Change the resolution-probe failure criterion to: "none of the FOUR resolution-dependent rows is `StateAlreadySet`," preserving UseKeychain and IdentitiesOnly exactly as independently derived.
- Use two sets during Include traversal — an `active` recursion stack for true cycle detection and an `expanded`/memoized set for repeated non-cyclic includes — and add a diamond-shaped Include graph test to distinguish shared references from cycles.
- Specify pending-plan synchronization concretely: a dedicated `pendingMigrationMu`, atomic lookup-and-consume, invalidation points (selection change, activation, successful commit, failed digest validation), and never holding that mutex during filesystem I/O.
- Correct the shadowing-precedence evidence citation to the operative ordering code in `writer.go` and `migrate.go`.
- Consider a mandatory integration checkpoint after Waves 1, 4, and 5, not only the final Wave-7 battery, since those waves alter the single write authority, shadow simulation, and two-file migration transaction respectively.

## Risk Assessment

**Overall risk: MEDIUM.** The phase goal is achievable, and all three Cycle-2 HIGH defects are genuinely corrected in the current plan text with strong directional acceptance tests. The six non-HIGH items are also materially incorporated. Risk is not LOW because Plan 06-04 currently specifies an impossible cross-package call to an unexported matcher, and the phase combines several security-sensitive refactors across SSH precedence, backups, rollback, and multi-file migration.

## Cycle-2 HIGH Disposition (codex-sol)

| Cycle-2 finding | Disposition | Evidence |
|---|---|---|
| Shadowing precedence reversed | FULLY RESOLVED | [06-04-PLAN.md:99-122](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:99), `shadowSourceFor` returns FIRST hit |
| `stateFor` baseline-ordering self-contradiction | FULLY RESOLVED | [06-03-PLAN.md:137-144](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:137) |
| Migration preview/commit plan-identity TOCTOU | FULLY RESOLVED | [06-05-PLAN.md:147-173](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:147) |

## New Issues (codex-sol)

| Finding | Severity | Evidence |
|---|---:|---|
| `06-04` requires reusing an unexported `sshconfig.aliasCollides` matcher across a package boundary — cannot compile as written | HIGH | [06-04-PLAN.md:121](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:121), [06-04-PLAN.md:226](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:226) vs [validation.go:160-164](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/validation.go:160) |
| Probe-failure "none of six rows AlreadySet" criterion can contradict the independent-evidence requirement for UseKeychain/IdentitiesOnly | MEDIUM | [06-03-PLAN.md:177-179](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:177) |
| `BuildGraph`'s global visited-path cycle rule flags legitimate diamond Include graphs as cycles | MEDIUM | [06-04-PLAN.md:204](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:204) |
| `pendingMigration`'s mutex relationship to `txMu` is unstated — potential deadlock if they are the same lock | MEDIUM | [06-05-PLAN.md:263](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:263) vs [274](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:274) |

---

## OpenCode Review (xai-grok)

Verifying cycle-3 HIGH fixes against plan text and source, not the summary.

### Document Type
GSD · plan (seven sequential waves)

### Files Read
- `.planning/phases/06-global-ssh-options/06-0{1–7}-PLAN.md`
- `internal/sshconfig/{include.go,migrate.go,renderer.go}`
- `cmd/gitid/{wiring.go,lifecycle.go}` (`Persist` `ApplySSH` still DEMO-ONLY at `wiring.go:715`; `runDelete` journal at `lifecycle.go:605`)
- `internal/sshconfig/include.go:19-28` (Include floored at top), `include.go:84-86` (`IsReservedBlockName` still only `ssh-include` + `_global`)
- `internal/sshconfig/migrate.go:423-428` (`movableBlockNames` still excludes globals)
- `internal/sshconfig/renderer.go:118-127` (`RenderGlobalBlock` still emits `IgnoreUnknown` *inside* `Host *`)

### Cycle-3 HIGH verification (against plan text, not the prompt)

**1. 06-04 first-obtained-value — CONFIRMED FIXED.** Binding `<resolution_order>` at `06-04-PLAN.md:99-122` quotes `ssh_config(5)` and matches source: Include prepend (`include.go:19-28`, `EnsureIncludeLine` + `PrependBlockIfNotFound`), globals last (`renderer.go:112-114`, `migrate.go:473-486`). Row 1 (directive **below** floored Include → **no** shadow) is a required negative control in behavior (`:147`), unit ACs (`:238`), wiring pair (`:337`), and PTY cases 5/6 (`:390`). `shadowSourceFor` is rewritten to linearise, filter, return **FIRST** hit (`:221`). This is a mechanism rewrite, not a caption change.

**2. 06-03 `stateFor` order — CONFIRMED FIXED.** Numbered branches at `06-03-PLAN.md:138-144`: (1) platform (2) no value (3) **value equals recommended → AlreadySet regardless of `src`** (4) baseline ∧ unequal → NeedsAction (5) set ∧ unequal → Differs. Pinned by ForwardAgent/`"no"`/baseline → AlreadySet (`:171`) and equality over **every** `SourceClass` (`:172`). Matches `design.go:214` + D-10. Cycle-2 self-contradiction is gone.

**3. 06-05 plan object + digest — CONFIRMED FIXED.** `MigrationPlan.Digests` + `MigrateWithPlan` as sole writer (`06-05-PLAN.md:35-36`, `:164-173`). First action: re-read vs `plan.Digests`, `ErrConfigChangedSincePreview` **before any backup**. TUI: one `PlanMigration`, hold in `pendingMigration`, opaque `PlanToken` (`:261-266`). Sequence AC: plan → mutate → `MigrateWithPlan` (`:204`). Call-count AC (`:313` area). Not "same function twice."

### Cycle-2 non-HIGH (6) incorporation

| Item | Status in current plans |
|---|---|
| Journal vs `filewriter.Write` | Incorporated. 06-01 Layer 4 names `mutationJournal` wrapping `filewriter.Write` (`:242`); 06-04 Task 2 says REUSE, no second restore (`:305`). Matches `lifecycle.go:605`. |
| `rg RenderGlobalBlock` vs archive | Incorporated. Scoped to `internal cmd e2e` (`06-01-PLAN.md:298`). Same for `persistApplySSH` (`06-04-PLAN.md:345`). |
| `IgnoreUnknown` vs dummy / visual gate | Incorporated. Required allowlist `T-06-GLOBALBLOCK` in 06-07 (`:116`); 06-01 output requires side-by-side renderings. Matches live `renderer.go:123-125` (guard inside stanza today). |
| Write-verb `--json` / T-06-43 | Incorporated. Four frozen envelopes including `gitid.ssh.apply/v1` with `advisories` + `exit_code` (06-06 `<frozen_contract>`). |
| Adaptive-depth / incomplete apply | Incorporated. Per-verb table: both TTYs → named view+sub-tab, empty selection; else exit 1. Routes through existing `depthResolver`. |
| Nested Include / `shadowSourceFor` last-hit | Folded into HIGH #1 (recursion + FIRST hit). |

### Summary

Cycle-3 plans close the three cycle-2 HIGHs with executable mechanisms and paired tests, not restated intent. Source still matches cycle 1 (unexecuted): `ApplySSH` is demo (`wiring.go:715-717`), `movableBlockNames` strands globals (`migrate.go:423-428`), `RenderGlobalBlock` still exists (`renderer.go:118`). Residual risk is Wave-1 size and a few execution traps, not inverted precedence, false-alarm `stateFor`, or preview/commit TOCTOU.

### Strengths

- One write authority from Wave 1: `runGlobalSSHApply`; `persistApplySSH` forbidden; `Persist(ApplySSH)` re-read-only — matches existing `ConfigureGit`/`DeleteIdentity` (`wiring.go:679-690`).
- D-06/D-08 co-located with migration classification in 06-02 so reserved registration cannot strand `Host *` (`migrate.go:426` is the live trap).
- Provenance: `/etc/ssh/ssh_config` named only after parse; isolation contract test; hedged class when system read fails.
- Advisory posture consistent: empty opt-in, zero-on-advisory CLI default + `--fail-on-advisory`, simulation inconclusive ≠ "safe."
- Threat model IDs map to ACs (T-06-48 pairs, T-06-41 plan-hold sequence, T-06-51 equality-first).

### Concerns

- LOW — Wave 1 still ~35 files / 120k. Accepted with three staged commits; pre-commit still lints the whole module (L11). Execution risk, not a correctness hole.
- LOW — 06-04 `read_first` cites `renderer.go:103-128` after 06-01 deletes it. Correct as *current* substrate; executor must not treat those lines as post-06-01 API.
- LOW — CLI empty-token migrate still plans+commits in one `txMu` (microseconds). Documented as honest; not the preview-to-confirm window.
- MEDIUM (execution, not plan defect) — PTY fake must read `-F` path. Vacuous shadowing if the fake is a constant. Plan pins this (T-06-38 + dedicated test). Do not weaken.

No new HIGH found by this reviewer. No remaining cycle-2 HIGH.

### Suggestions

- Keep 06-01 Task 1 as three hook-clean commits; do not re-slice the tracer.
- 06-07 must wait for 06-01 SUMMARY's quoted `EnsureGlobals` vs `managedHostStar` text before writing `T-06-GLOBALBLOCK` needles.
- Do not "clarify" exit code 2 into a fourth code; `restored`/`error` in JSON already distinguish pre vs post write.

### Risk Assessment

**LOW–MEDIUM.** Correctness of the three former HIGHs is pinned. Remaining risk is Wave-1 blast radius and PTY-fake fidelity, both already mitigated in the plans.

**Verdict:** Cycle-3 claims are real. Ready to execute Wave 1 — *this reviewer did not check `06-04`'s cross-package reuse of `sshconfig.aliasCollides` for exportedness; see Orchestrator Verification below, where the orchestrator confirms codex-sol's HIGH finding on this point.*

---

## Orchestrator Verification

The two reviewers agree on all three former Cycle-2 HIGH dispositions (all FULLY RESOLVED, both source-grounded with matching file:line citations) and on all 6 actionable items being incorporated. They diverge on one new finding: codex-sol raises a HIGH that `06-04-PLAN.md` requires `internal/globalssh/shadow.go` to reuse `sshconfig.aliasCollides`, which is unexported; xai-grok's review does not mention this function at all.

**Independently confirmed REAL.** `internal/sshconfig/validation.go:160-164`:

```go
func AliasCollision(configPath, candidate string) (bool, error) {
	return aliasCollides(configPath, candidate, nil)
}

func aliasCollides(configPath, candidate string, seen map[string]bool) (bool, error) {
```

`aliasCollides` (lowercase) is unexported and therefore invisible outside `internal/sshconfig`. Its exported wrapper, `AliasCollision`, takes a `configPath` and does its own file read + parse + recursive Include walk internally — it is not a per-pattern matcher usable against directives the shadow scanner has already extracted via `ScanDirectivesMulti`. `06-04-PLAN.md:121` and `:226` both instruct: "Reuse the negation-aware matching `internal/sshconfig/validation.go:155-215` (`aliasCollides`) already implements rather than writing a second matcher." `06-04-PLAN.md`'s Task 1 file list (`internal/globalssh/shadow.go, internal/globalssh/shadow_test.go, internal/sshconfig/directives.go, internal/sshconfig/directives_test.go`) does not include `validation.go`, so nothing in the plan exports the needed symbol. As written, an executor following the plan literally cannot compile `shadowSourceFor`'s host-pattern filtering step without either (a) violating the plan's explicit "don't write a second matcher" instruction by duplicating the negation logic, or (b) silently deviating from the plan to add an export the plan never authorizes. This is a genuine, confirmed HIGH — not a matter of interpretation, the same class of defect (a plan instruction that cannot execute as literally written) as the Cycle-2 HIGHs.

The MEDIUM findings (probe-failure "none of six rows" over-broad assertion at `06-03-PLAN.md:179`; the global-visited-set cycle/diamond conflation at `06-04-PLAN.md:204`; the unstated `pendingMigration`/`txMu` mutex relationship at `06-05-PLAN.md:263` vs `:274`) were spot-checked against the cited plan text and are all real, textually-grounded ambiguities or over-broad test assertions — none is blocking in the way the HIGH is, but all three would benefit from being tightened before the corresponding wave executes.

---

## Consensus Summary

Both reviewers agree, independently and with matching file:line citations, that the 5→7→(same 7, revised) plan set genuinely closes all three Cycle-2 HIGH findings with real mechanism changes, not reworded promises: the shadowing model now correctly implements OpenSSH's first-obtained-value rule with the culprit-selection direction fixed too; `stateFor`'s branch order now checks value-equality before any source-class branch, closing the false-alarm-on-safe-default defect; and migration preview/commit now share one digest-bearing `MigrationPlan` object carried through an opaque token, closing the TOCTOU window. Both reviewers also independently confirm all 6 Cycle-2 actionable (non-HIGH) findings are incorporated with concrete mechanisms.

Where they diverge is a single new finding: codex-sol identifies a genuine HIGH — `06-04-PLAN.md` instructs reuse of an unexported `sshconfig.aliasCollides` function without adding it (or an exported equivalent) to the plan's file list, which cannot compile as literally written. xai-grok's review does not surface this. The orchestrator independently traced the claim against `internal/sshconfig/validation.go` and `06-04-PLAN.md`'s file list and confirms it is real: this is carried forward as an active, unresolved HIGH for this cycle.

### Agreed Strengths
- The shadowing-precedence model is now directionally correct and matches `include.go`'s own documented design and OpenSSH's `ssh_config(5)` first-obtained-value rule, with a required negative control.
- `stateFor`'s branch order now checks value-equality before any source-class test, closing the false-alarm-on-safe-default defect (pinned by an every-`SourceClass` equality test).
- The migration engine now carries one `MigrationPlan` object (with content digests) from preview to commit via an opaque token, verified against disk before any backup — closing the TOCTOU window with a sequence test that mutates disk between plan and commit.
- All 6 Cycle-2 actionable findings (journal/`filewriter.Write` consistency, archive-scoped `rg` greps, `IgnoreUnknown` visual classification, write-verb JSON envelopes, adaptive-depth freeze, dual-enum numeric pin) are incorporated with concrete mechanisms and acceptance criteria.

### Agreed Concerns
- Wave 1 remains large (~120k tokens, ~35 files across three staged commits) — accepted with mitigation (three hook-clean commits) rather than re-sliced further, since the module-wide pre-commit lint forbids finer splitting without breaking the tracer property.
- The PTY fake `ssh -G` stub must actually read the `-F` isolated-config path rather than returning a constant, or the shadowing tests are vacuous; the plan already pins this but it is an execution-fidelity risk worth flagging again.

### Divergent Views — resolved by orchestrator verification
- **06-04 unexported `aliasCollides` reuse**: codex-sol HIGH, xai-grok not flagged. **Orchestrator confirms codex-sol is correct** — `aliasCollides` is unexported in `internal/sshconfig/validation.go:164`, its exported wrapper `AliasCollision` has an incompatible signature (config path, not pattern list), and `06-04-PLAN.md`'s Task 1 file list omits `validation.go`, so the plan as written cannot compile.

### New Actionable (non-HIGH) Findings This Cycle
- MEDIUM — `06-03-PLAN.md:179`'s "none of six rows is `StateAlreadySet`" probe-failure assertion can contradict the independently-derived UseKeychain/IdentitiesOnly rows; should scope to the four resolution-dependent rows.
- MEDIUM — `06-04-PLAN.md:204`'s global visited-path cycle detection in `BuildGraph` will flag a legitimate diamond-shaped (non-cyclic) Include graph as a cycle, producing an unearned INCONCLUSIVE; needs a recursion-stack-vs-memoization split.
- MEDIUM — `06-05-PLAN.md:263` vs `:274`: the relationship between `txMu` and whatever guards `pendingMigration` is unstated; if they are the same lock, a lookup-and-consume helper called from inside `runSSHStorageMigrate` could deadlock on Go's non-reentrant `sync.Mutex`.
