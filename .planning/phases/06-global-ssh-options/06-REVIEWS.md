---
phase: 6
reviewers: [codex-sol, xai-grok]
reviewed_at: 2026-08-26T19:25:19Z
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

# Cross-AI Plan Review — Phase 6 (Cycle 2)

> Note on `codex-sol`: `.planning/config.json` pins this instance's model to
> `openai/gpt-5.6-sol-fast`, but the Codex CLI on this host authenticates via a
> ChatGPT account, which rejected that model with `400 invalid_request_error:
> "The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a
> ChatGPT account."` (captured in the run's `.err` log, same failure mode as
> Cycle 1). The review below was produced by re-invoking the `codex` lane
> without the model override, so it ran on the account's default resolved
> model (`gpt-5.6-sol`, reasoning=low) instead of the configured
> `gpt-5.6-sol-fast`. The `review.reviewer_instances` entry for `codex-sol`
> should be corrected (or the account's available model list re-checked)
> before the next review run.

This is Cycle 2 — the planner revised the plan set from 5 plans to 7 plans to
address Cycle 1's 8 HIGH + 6 actionable findings. Both reviewers were asked to
(1) independently verify each Cycle-1 finding's disposition against the
revised plan text and the actual code, and (2) flag any new issue the
revision introduced.

**The orchestrator (this session) independently re-verified three of the
disputed findings below against the actual plan text and source** —
`06-04-PLAN.md`'s shadow-precedence example, `06-03-PLAN.md`'s `stateFor`
baseline rule, and `06-05-PLAN.md`'s preview/commit plan identity — since the
two reviewers disagreed sharply on their severity. All three are confirmed
real, independent of either reviewer's framing; see "Orchestrator
Verification" below.

## Codex Review (codex-sol)

# Cycle 2 Review — Phase 6: Global SSH Options

## Summary

The revision substantially improves the plan set. The single-write-authority conflict, create-call-site omissions, migration classification, migration backup/concurrency design, CLI contract, and HTML-parity ambiguity are now addressed explicitly and generally with appropriate tests.

However, the plans are not ready for execution unchanged. Two new/high-risk defects remain:

1. The shadowing model reverses OpenSSH's first-obtained-value precedence: it expects a directive after the floored `Include` to shadow the included `Host *`, when the included value actually wins.
2. The storage preview and commit do not use the same immutable migration plan despite repeatedly claiming they do; the plan is recomputed at preview, lifecycle planning, and commit, leaving a confirmation-time TOCTOU gap.

The purported hermetic `HOME` test for default SSH configuration isolation also remains unreliable, and nested Includes are not faithfully mirrored.

Overall risk: **HIGH until the simulation precedence and migration preview/commit identity are corrected.**

---

## Plan 06-01 — Tracer, provenance, and single globals owner

**Strengths:** Single write authority correctly established (`runGlobalSSHApply` sole mutation owner, `Persist(ApplySSH)` reread-only, [06-01-PLAN.md:391](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-01-PLAN.md:391)); both missed create sites now retargeted ([identity_create.go:227](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/identity_create.go:227), [wiring.go:2594](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/wiring.go:2594)); `RenderGlobalBlock` correctly deleted; directive-aware scanner closes the API gap; provenance wording bounded to evidence.

**Concerns:** HIGH — the isolation-contract test's evidentiary strength is disputed (see Orchestrator Verification: judged sufficient on independent check). MEDIUM — lossy managed-block normalization (duplicate directives/comments) remains under-tested. LOW — Wave 1 remains very large (120k tokens across probe/renderer/writer/identity/lifecycle/TUI/dummy).

**Risk:** MEDIUM-HIGH per this reviewer; downgraded to MEDIUM by orchestrator verification of the isolation test.

## Plan 06-02 — Reserved registry and migration classification

**Strengths:** Correctly separates `IsGlobalBlockName` from the broader reserved set so Include wiring stays put; migration exclusion defect accurately grounded against [migrate.go:419](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:419); both-direction tests, last-block ordering, legacy-name migration, both-files-ambiguity refusal all cover the important cases.

**Concerns:** MEDIUM — both-files preflight rejects even identical global blocks (conservative, undocumented whether intentional). LOW — legacy-name renaming timing unclear.

**Risk:** MEDIUM.

## Plan 06-03 — Six-option classification

**Strengths:** Source attribution correctly separated from state (`AttributedToUser`); UseKeychain/IdentitiesOnly no longer depend on generic probe success; empty IdentitiesOnly inventory → not-applicable, not false conformance; unknown OpenSSH version → explicit refusal.

**Concerns:** **HIGH — confirmed by orchestrator** — `stateFor`'s stated rule ("baseline class... whatever its source" for equality) contradicts its own ordering: `src is baseline class → StateNeedsAction` fires BEFORE the equality check, so a safe OpenSSH default equal to the recommendation (e.g. `ForwardAgent no`, matching [design.go:214](/Users/ramon/git/personal/ssh-git-config/internal/tuikit/design.go:214)'s fixture) is incorrectly flagged as needing action. MEDIUM — "baseline" conflates safe default with unset recommendation generally. LOW — concurrent-latency test may be flaky under `-race`.

**Risk:** MEDIUM-HIGH — confirmed HIGH by orchestrator.

## Plan 06-04 — Whole-graph simulation and Options PTY

**Strengths:** Explicit graph object with line-aware naming API; scanner correctly limited to naming, real `ssh -G` probe decides winner; mirror uses 0700/0600 with cleanup; PTY covers clean/shadowed/inconclusive/failed-write.

**Concerns:** **HIGH — confirmed by orchestrator** — the plan's shadowing example ([06-04-PLAN.md:115](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:115)) has the main config directive win over the Include'd candidate when it appears textually after the Include line. `internal/sshconfig/include.go`'s own doc comment ([include.go:19-22](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/include.go:19)) states the Include is floored at the top specifically so it is "first-match-wins ahead of any later hand-written Host block" — i.e. the Included content (gitid's candidate) wins, not the later main-config directive. The plan's expected test outcome is backwards from both OpenSSH's documented first-obtained-value semantics and the codebase's own existing design rationale. HIGH — `shadowSourceFor` picks "last hit before the managed block" as culprit, which is the wrong side under first-obtained-value: the actual culprit is an earlier matching assignment, not a later one. HIGH — the mirror only copies the entry point's direct Includes; nested Includes inside those files are neither discovered nor rewritten, so the mirror is not a faithful isolated copy of the resolved graph ([include.go:183](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/include.go:183) shows existing Include discovery is non-recursive too, so this isn't a regression, but the plan's "whole-graph" claim overstates what it delivers).

**Risk:** HIGH — the phase's central prove-before-write mechanism currently models the primary precedence case incorrectly.

## Plan 06-05 — Migration hardening and Storage UI

**Strengths:** `filewriter.Backup` correctly separates backup from write ([filewriter.go:47](/Users/ramon/git/personal/ssh-git-config/internal/filewriter/filewriter.go:47) shows the old always-write-then-backup pattern); `PlanMigration` extracts pure composition; content digests catch same-tick edits; per-file written-by-us rollback tracking fixes the current restore-both defect ([migrate.go:488](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:488)).

**Concerns:** **HIGH — confirmed by orchestrator** — preview (`SSHStorageMigrationPlan` → `sshconfig.PlanMigration`) and commit (`runSSHStorageMigrate` → `sshconfig.Migrate`, which "CALLS `PlanMigration`" again internally per the plan's own text) are two separate `PlanMigration` invocations against disk read at two different times. The plan's concurrency detection only guards `Migrate`'s own internal preflight-to-write window; it never compares against what the earlier preview call showed the user. A disk change between preview and confirm silently commits different bytes than the user approved, without tripping the concurrency abort. MEDIUM — backups may not correspond to displayed bytes for the same reason.

**Risk:** HIGH — confirmed by orchestrator; the "what you saw is what gets written" guarantee is not actually achieved by this plan's mechanism.

## Plan 06-06 — CLI and parity contract

**Strengths:** Command tree frozen before implementation; Cobra calls UI-free lifecycle functions; JSON schema/enum/ordering explicitly versioned and tested; zero-on-advisory posture resolved.

**Concerns:** MEDIUM — advisory JSON promised in the threat model but no write verb has `--json` in the frozen tree; write-result schema undefined. MEDIUM — interactive fallback for `options apply` with missing args underspecified. LOW — exit code 2 wording doesn't distinguish pre-write vs post-write failure.

**Risk:** MEDIUM.

## Plan 06-07 — Visual gate and review packet

**Strengths:** HTML non-applicability explicit; named PTY-frame evidence; four negative controls; separate fixture homes; visual closeout split from CLI work.

**Concerns:** MEDIUM — shared-renderer blind spot (live/dummy can share the same defect) not eliminated, only reduced. LOW — exact frame-count assertion is brittle across unrelated registry changes. LOW — closure table hard-coded to 8 HIGH / 6 non-HIGH; should derive from review IDs so Cycle 2 findings are included.

**Risk:** MEDIUM-LOW in isolation; cannot compensate for upstream simulation/migration defects.

## Cycle-1 Finding Disposition (codex-sol)

| Cycle-1 finding | Disposition | Evidence |
|---|---|---|
| Two competing global-SSH write authorities | FULLY RESOLVED | `runGlobalSSHApply` sole owner; `Persist` reread-only ([06-01-PLAN.md:391](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-01-PLAN.md:391)) |
| Provenance falsely claims `/etc` | FULLY RESOLVED | Explicit system-file parse required ([06-01-PLAN.md:193](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-01-PLAN.md:193)) |
| `-F /dev/null` baseline assumption unproven | PARTIALLY RESOLVED (orchestrator: RESOLVED, see below) | isolation_contract_test.go added ([06-01-PLAN.md:191](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-01-PLAN.md:191)) |
| Create paths not all retargeted | FULLY RESOLVED | Both sites named and retargeted |
| Managed-block map may drop unknown content | PARTIALLY RESOLVED | Unknown keys retained; duplicates/comments still lossy |
| Shadow scanner API lacks directive values/lines | FULLY RESOLVED | `ScanDirectives`/`ScanDirectivesMulti` replaces `HostStanza` |
| Candidate-file-only simulation misses main graph | PARTIALLY RESOLVED | Entry point + direct Includes mirrored; nested Includes not recursive |
| Migration excludes global block | FULLY RESOLVED | Plan 06-02 fixes `migrate.go:419` exclusion |
| Concurrency check occurred after backup writes | FULLY RESOLVED | Backup-only seam + pre-backup check |
| Rollback could erase external edits | FULLY RESOLVED | Per-file written-by-us tracking |
| No pure migration preview | PARTIALLY RESOLVED (see new HIGH above) | `PlanMigration` added but not shared between preview and commit |
| CLI syntax, schema, exit semantics unfrozen | FULLY RESOLVED | Frozen in 06-06-PLAN.md |
| HTML might become a Phase 6 parity target | FULLY RESOLVED | Every spec marks HTML non-applicable |
| Oversized closing wave / overall scope | PARTIALLY RESOLVED | Split into CLI + visual plans; Waves 1/3/4/5 still 120k-140k tokens |

## New Issues (codex-sol)

| Finding | Severity | Evidence |
|---|---:|---|
| Shadowing example reverses first-obtained-value precedence | HIGH — confirmed | [06-04-PLAN.md:115](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:115) vs [include.go:19](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/include.go:19) |
| Confirmed migration preview can differ from committed plan | HIGH — confirmed | [06-05-PLAN.md:212](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:212) vs [06-05-PLAN.md:134](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:134) |
| Nested Includes can escape or disappear from the mirror | HIGH | `SimulationGraph` entry-point-only rewriting |
| Baseline safe defaults incorrectly forced to needs-action | HIGH — confirmed | [06-03-PLAN.md:134](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:134) vs [design.go:214](/Users/ramon/git/personal/ssh-git-config/internal/tuikit/design.go:214) |
| Advisory JSON promised without a write JSON schema/flag | MEDIUM | Frozen tree gives JSON only to read verbs |

---

## OpenCode Review (xai-grok)

Verified against live source. Cycle-1 findings were checked in the revised plan text and in the code those plans still claim to change.

## Executive assessment

The 5→7 replan closes the Cycle-1 blockers in the plans. Write ownership, provenance honesty, whole-graph simulation, migration classification, backup-before-check, frozen CLI, and wave split are all written into the right waves with acceptance criteria that pin the original defects.

The code still matches Cycle 1 (expected — nothing executed). Residual risk is execution of Wave 1's huge tracer, plus a few new plan-level nits (IgnoreUnknown placement vs current renderer, `rg RenderGlobalBlock` vs archive, journal vs `filewriter.Write` on apply). None re-open the old dual-write or strand-globals defects.

**Overall risk: MEDIUM** (was HIGH). Ready to execute 06-01.

## Plan 06-01 — Tracer + single owner

**Strengths:** Single write authority matches existing `ConfigureGit` ownership pattern (`wiring.go:650-684`); create-path HIGH confirmed real and fixed; `HostStanza` confirmed lacking directive value/line; isolation-contract test answers the `/etc` spoofing finding.

**Concerns:** MEDIUM — `rg -n 'RenderGlobalBlock'` will also hit `.planning/archive/0.0.1-poc-...`; scope the grep to production packages. MEDIUM — recipe/dummy put `IgnoreUnknown` before `Host *`; live `RenderGlobalBlock` puts it inside ([renderer.go:123-125](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/renderer.go:123)); dummy visual baselines will move — classify in 06-07 or the gate fails. LOW — Task 1 still spans ~35 files across three commits.

**Risk:** MEDIUM.

## Plan 06-02 — Registry + migration classes

**Strengths:** Filter excludes globals two ways today (`migrate.go:419-427`, `include.go:84-85`) — registering `global-ssh` without Task 2 would strand it; narrow `IsGlobalBlockName` correctly motivated.

**Concerns:** LOW — `reorderGlobalLast` must handle both names during adopt-in-flight; keep as an explicit acceptance criterion.

**Risk:** LOW-MEDIUM.

## Plan 06-03 — Six-option classifier

**Strengths:** Attribution split fixes the explicit≠effective HIGH; per-source probe errors, empty IdentitiesOnly, `VersionUnverified`, reason codes, concurrent probes all pinned.

**Concerns:** LOW — dual `NotApplicableReason` enums (package + tuikit) can drift; pin numeric values in a test. LOW — latency test against a fake sleeper doesn't prove real concurrency.

*(This reviewer did not flag the `stateFor` baseline-ordering self-contradiction the orchestrator independently confirmed — see "Orchestrator Verification".)*

**Risk:** MEDIUM.

## Plan 06-04 — Whole-graph simulate + ceremony + PTY

**Strengths:** `SimulationGraph` + mirrored entry point addresses the vacuous Include-layout proof; extends `runGlobalSSHApply` rather than adding a second writer; fake-ssh "read the `-F` path" AC prevents vacuous shadow tests.

**Concerns:** MEDIUM — 06-01 says apply writes via `filewriter.Write`; 06-04 says journal-backed rollback; `runDelete` uses `mutationJournal` ([lifecycle.go:605](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/lifecycle.go:605)) but 06-01 Layer 4 doesn't name a journal — pin the atomicity story in 06-01 so 06-04 doesn't invent a second restore model. LOW — `ScanDirectivesMulti` best-effort (accepted).

*(This reviewer did not flag the shadowing-precedence reversal the orchestrator independently confirmed — see "Orchestrator Verification".)*

**Risk:** MEDIUM.

## Plan 06-05 — Storage + engine harden

**Strengths:** `filewriter.Backup`, pre-backup digest, restore-only-what-we-wrote, `PlanMigration` close all three Cycle-1 engine HIGHs against the live defects at `migrate.go:265-270` and `migrate.go:219-226`.

**Concerns:** LOW — round-trip "same key via resolution" needs the fake `ssh` to be Include-aware after a layout change; state that in the test setup.

*(This reviewer did not flag the preview/commit plan-identity gap the orchestrator independently confirmed — see "Orchestrator Verification".)*

**Risk:** MEDIUM.

## Plan 06-06 — Frozen CLI

**Strengths:** Tree, JSON `gitid.ssh.options/v1`, exit 0/1/2/3 + `--fail-on-advisory` actually frozen; shared `run*` ceremonies match `identity_delete.go`.

**Concerns:** LOW — "incomplete args on a TTY opens the TUI" is extra surface; keep identical to identity verbs or drop from acceptance criteria.

**Risk:** LOW-MEDIUM.

## Plan 06-07 — Visual gate + battery

**Strengths:** Split from CLI is correct; HTML non-applicability + named PTY files close the hollow-frame and HTML-parity findings.

**Concerns:** LOW — closure table depends on 06-01…06-06 summaries existing.

**Risk:** LOW.

## Cycle-1 Finding Disposition (xai-grok)

All 18 tracked Cycle-1 findings (the 8 HIGH plus 10 of the itemized sub-findings) are recorded as FULLY RESOLVED at the plan level, cross-checked against current source (e.g. `wiring.go:715` DEMO-ONLY confirmed for Persist/Commit; `migrate.go:419-427` confirmed for the strand-globals filter; `reader.go:101-108`/`105-108` confirmed for `HostStanza`'s missing value/line fields). Full table in the run's `gsd-review-xai-grok.md` capture.

## New Issues (xai-grok)

| Finding | Severity |
|---|---:|
| Apply rollback story unstated: `filewriter.Write` (06-01) vs journal (06-04) | MEDIUM |
| `RenderGlobalBlock` grep will hit archive; `IgnoreUnknown` placement vs dummy baseline | MEDIUM |
| Dual reason enums; TTY→TUI apply depth | LOW |

## Recommendation

Approve with nits — do not re-cycle the whole set. Before executing 06-01, tighten: (1) apply journal vs `Write`, (2) grep scope, (3) `IgnoreUnknown` visual allowlist note. Then run Wave 1.

---

## Orchestrator Verification

The two reviewers diverged sharply on severity (codex-sol: HIGH risk, 4 new HIGHs; xai-grok: MEDIUM risk, 0 new HIGHs, "approve with nits"). Given the stakes — this phase writes to `~/.ssh/config` — the orchestrator independently re-checked the three highest-impact disputed claims against the plan text and the actual source files, rather than taking either reviewer's word.

1. **06-04 shadowing example reversed — CONFIRMED REAL.** `06-04-PLAN.md`'s behavior spec ([06-04-PLAN.md:115](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-04-PLAN.md:115)) states: "Simulating a graph whose MAIN config sets the same option after the Include line reports shadowing, even though the candidate file itself contains the requested value." `internal/sshconfig/include.go:19-22` documents gitid's own design rationale for flooring the Include at the top of the file: "first-match-wins ahead of any later hand-written Host block" — i.e., OpenSSH's first-obtained-value semantics mean the Included content (gitid's candidate) wins over anything appearing later in the main file, not the reverse. The plan's stated test expectation is backwards relative to both OpenSSH's documented behavior and the codebase's own existing comment. This will either produce a test that fails against real `ssh -G`, or force the fake to encode incorrect precedence. **Confirmed HIGH.**

2. **06-03 `stateFor` baseline rule contradicts itself — CONFIRMED REAL.** `06-03-PLAN.md`'s behavior list states "An option whose effective value equals the policy's recommended value classifies as already-set, whatever its source" ([06-03-PLAN.md:99](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:99)) — but the algorithm immediately below it orders the checks as: platform gate first, then "`src` is the baseline class or there is no value at all → `StateNeedsAction`" ([06-03-PLAN.md:134](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-03-PLAN.md:134)), and only THEN checks value equality. A baseline-sourced value is routed to `StateNeedsAction` unconditionally, before the equality check ever runs — directly contradicting the "whatever its source" promise two paragraphs earlier. `internal/tuikit/design.go:214`'s existing fixture records `ForwardAgent`'s OpenSSH default as `no`, which equals the D-10 recommendation of `no` — exactly the case this ordering bug would mis-flag as needing action. **Confirmed HIGH — a genuine self-contradiction in the plan's own text, not a matter of interpretation.**

3. **06-05 preview/commit plan-identity gap — CONFIRMED REAL.** `06-05-PLAN.md` promises "Preview and commit read the SAME plan" ([06-05-PLAN.md:35](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:35)) via one `PlanMigration` call. Tracing the actual call chain in the plan text: `SSHStorageMigrationPlan` (preview, backend) calls `sshconfig.PlanMigration` once ([06-05-PLAN.md:212](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:212)); separately, `Migrate` (commit path, reached via `runSSHStorageMigrate` → `sshconfig.Migrate`) "CALLS `PlanMigration`" again ([06-05-PLAN.md:134](/Users/ramon/git/personal/ssh-git-config/.planning/phases/06-global-ssh-options/06-05-PLAN.md:134)) against disk read at commit time. These are two independent reads of disk at two different points in time. The plan's concurrency detection (06-05's Task 1) only guards `Migrate`'s own internal preflight-to-write window — it never compares against the bytes the earlier preview call rendered to the user. A disk change between the preview screen and the confirm action produces a self-consistent-but-different plan at commit time, with no abort, silently writing something other than what was previewed. **Confirmed HIGH — the plan's own "what you saw is what gets written" guarantee is not met by the described mechanism.** (Note: the isolation_contract_test.go design for a *different* Cycle-1 finding — the `$HOME`-based OpenSSH proof in 06-01 — was also checked and judged adequate: it is an executable, empirical comparison of plain-vs-isolated `ssh -G` output rather than an assumption, so that particular disputed claim resolves in xai-grok's favor, not codex-sol's.)

These three are carried into the Consensus Summary below as active, unresolved HIGH concerns for this cycle, regardless of either individual reviewer's overall risk label.

---

## Consensus Summary

Both reviewers agree the 5→7 replan genuinely closes the large majority of Cycle 1's findings — the write-authority conflict, provenance overclaiming, missed create sites, migration-exclusion, concurrency-check timing, rollback-erasure, CLI freeze, and HTML-parity ambiguity are all FULLY RESOLVED by both independent accounts, each with matching file:line citations against current source. Neither reviewer's verdict on these should be discounted; both are source-grounded (`[reviewed-without-source-citations]` does not apply to either).

Where they diverge is severity of what's left. codex-sol treats the shadowing-precedence reversal (06-04), the stateFor baseline contradiction (06-03), and the preview/commit plan-identity gap (06-05) as blocking HIGH defects; xai-grok treats the plan set as ready to execute with only MEDIUM/LOW nits and does not surface any of the three. The orchestrator independently traced all three against the plan text and cited source and confirms all three are real — the 06-04 and 06-03 findings in particular are not judgment calls: the plan's own stated intent (in each case, quoted in the plan) is directly contradicted by the mechanism it specifies two paragraphs later.

### Agreed Strengths
- Single write authority (`runGlobalSSHApply`) correctly replaces the dual-write conflict; `Persist` is reread-only.
- Both missed create-site call sites (`identity_create.go:227`, `wiring.go:2594`) are retargeted.
- `RenderGlobalBlock` is correctly deleted rather than left as a second renderer.
- Migration classification now includes the globals block (`06-02` fixes `migrate.go:419`'s exclusion).
- `filewriter.Backup` correctly separates backup from write, closing the concurrency-detection-after-corruption defect.
- Per-file written-by-us rollback tracking prevents external-edit erasure.
- CLI command tree, JSON schema, and exit codes are genuinely frozen.
- HTML non-applicability is explicit per-spec.

### Agreed Concerns
- Wave 1 (06-01) remains large (~120k tokens, ~35 files across three staged commits) — both reviewers flag this as execution risk even though the architecture is correct.
- The apply-path rollback/atomicity story is inconsistently stated between 06-01 (`filewriter.Write`) and 06-04 (journal-backed rollback) — both reviewers independently flag this; pin it in 06-01 before 06-04 is planned in detail.
- `RenderGlobalBlock` deletion verification via `rg` needs to be scoped to production packages, or it will false-positive against `.planning/archive/`.

### Divergent Views — now resolved by orchestrator verification
- **06-04 shadowing precedence**: codex-sol HIGH, xai-grok not flagged. **Orchestrator confirms codex-sol is correct** — the plan's example contradicts `include.go`'s own documented design.
- **06-03 stateFor baseline rule**: codex-sol HIGH, xai-grok not flagged. **Orchestrator confirms codex-sol is correct** — a direct textual self-contradiction in the plan.
- **06-05 preview/commit plan identity**: codex-sol HIGH, xai-grok not flagged (xai-grok credits `PlanMigration` with closing the Cycle-1 "no pure preview" finding without checking whether preview and commit share one instance). **Orchestrator confirms codex-sol is correct** — they do not share one instance.
- **06-01 isolation_contract_test.go**: codex-sol HIGH ("not reliably hermetic"), xai-grok FULLY RESOLVED. **Orchestrator sides with xai-grok** — the test's plain-vs-isolated comparison is an executable, empirical proof, not an unverified assumption.
