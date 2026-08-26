---
phase: 6
reviewers: [codex-sol, xai-grok]
reviewed_at: 2026-08-26T18:43:02Z
plans_reviewed:
  - .planning/phases/06-global-ssh-options/06-01-PLAN.md
  - .planning/phases/06-global-ssh-options/06-02-PLAN.md
  - .planning/phases/06-global-ssh-options/06-03-PLAN.md
  - .planning/phases/06-global-ssh-options/06-04-PLAN.md
  - .planning/phases/06-global-ssh-options/06-05-PLAN.md
models:
  codex-sol: "gpt-5.6-sol (reasoning=low)"
  xai-grok: "xai/grok-4.6 (reasoning=low)"
model_sources:
  codex-sol: "banner"
  xai-grok: "pinned"
---

# Cross-AI Plan Review — Phase 6

> Note on `codex-sol`: `.planning/config.json` pins this instance's model to
> `openai/gpt-5.6-sol-fast`, but the Codex CLI on this host authenticates via a
> ChatGPT account, which rejected that model with `400 invalid_request_error:
> "The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a
> ChatGPT account."` (captured in the run's `.err` log). The review below was
> produced by re-invoking the `codex` lane without the model override, so it
> ran on the account's default resolved model (`gpt-5.6-sol`, reasoning=low)
> instead of the configured `gpt-5.6-sol-fast`. The `review.reviewer_instances`
> entry for `codex-sol` should be corrected (or the account's available model
> list re-checked) before the next review run.

## Codex Review (codex-sol)

# Cross-AI Plan Review — Phase 6: Global SSH Options

## Executive assessment

The phase is thoughtfully decomposed and unusually strong on safety intent, TDD, PTY coverage, UI/backend separation, and verification. However, the plans are not ready to execute unchanged. Several mechanisms conflict with the existing code or cannot deliver their stated guarantees:

- The proposed three-probe provenance model cannot reliably distinguish system configuration from user configuration outside the parsed file.
- The shadow-source scanner is based on an API that does not expose directive values or line numbers.
- Storage migration currently excludes the global block, contradicting the intended layout migration.
- Concurrent-modification detection is placed too late if the existing "backup" step continues rewriting both files.
- Plan 01 creates a synchronous write path that Plan 03 replaces, unnecessarily exposing a security-sensitive interim design.
- The five plans total roughly 670k estimated tokens and expand GSSH-01 into CLI design, migration hardening, storage UI, visual-regression infrastructure, and review-packet assembly. That is substantial scope risk.

Overall phase risk: **HIGH** until those architecture issues are corrected.

---

# Plan 06-01 — Tracer and single globals owner

## Summary

The tracer-first strategy is sound: it attacks the most consequential integration risk—create subsequently erasing global fixes—before expanding to six options. The reserved-name consolidation and common `EnsureGlobals` owner are well motivated by the current code. The plan nevertheless depends on an unreliable provenance algorithm and proposes a lossy managed-block parser. It also introduces a synchronous persistence path that contradicts the asynchronous transactional model explicitly planned for Wave 3.

## Strengths

- The single-owner refactor addresses a real defect in the current implementation. `sshconfig.Write` currently replaces `_global` from the caller-supplied body every time an identity is written, which would erase later GSSH settings ([internal/sshconfig/writer.go:21](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/writer.go:21), [internal/sshconfig/writer.go:47](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/writer.go:47)).
- The placement requirement is grounded in existing behavior. The current writer documents that the wildcard block must be last and implements that by replacing it after the identity block ([internal/sshconfig/writer.go:10](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/writer.go:10), [internal/sshconfig/writer.go:47](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/writer.go:47)).
- The reserved-name consolidation closes a demonstrated safety gap. Today `_global` is registered, but other packages still contain direct comparisons ([internal/sshconfig/include.go:65](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/include.go:65), [internal/doctor/checks/overlap.go:21](/Users/ramon/git/personal/ssh-git-config/internal/doctor/checks/overlap.go:21)).
- The backend DTO boundary follows the repository architecture. `tuikit.Backend` deliberately accepts only UI-facing types and forbids backend imports ([internal/tuikit/backend.go:83](/Users/ramon/git/personal/ssh-git-config/internal/tuikit/backend.go:83)).
- The backup and idempotence assertions align with the real chokepoint. `filewriter.Write` backs up every pre-existing target, even when replacement bytes are identical, and uses collision-resistant names ([internal/filewriter/filewriter.go:47](/Users/ramon/git/personal/ssh-git-config/internal/filewriter/filewriter.go:47), [internal/filewriter/filewriter.go:130](/Users/ramon/git/personal/ssh-git-config/internal/filewriter/filewriter.go:130)).

## Concerns

- **HIGH — The proposed provenance classifier cannot prove "system-set."** `Deps.ReadConfig` reads one configured path, while the effective `ssh -G` result may incorporate user `Include` files, system configuration, command-line environment, or other sources. An effective value absent from that one file and different from the baseline does not establish `/etc/ssh/ssh_config` provenance. Existing SSH readers are explicitly single-file and Include-unaware in related APIs ([internal/sshconfig/reader.go:136](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/reader.go:136)). The plan's confident label "set in /etc/ssh/ssh_config" is therefore unsupported.
- **HIGH — The claim that `ssh -G -F /dev/null` yields "compiled defaults only" needs an executable proof.** Existing code uses `-F` to isolate a staged user configuration, but does not claim that it suppresses system configuration ([internal/tester/tester.go:181](/Users/ramon/git/personal/ssh-git-config/internal/tester/tester.go:181), [internal/tester/tester.go:205](/Users/ramon/git/personal/ssh-git-config/internal/tester/tester.go:205)). The entire default-vs-system classification rests on that unverified premise.
- **HIGH — Plan 01 introduces the wrong mutation architecture.** It changes `Persist(ApplySSH)` from demo reduction into a real synchronous disk write, while Plan 03 later adds `CommitGlobalSSH`, journaling, async execution, rollback, and result-message handling. The current architecture classifies real writes as already committed before `Persist` re-reads disk ([cmd/gitid/wiring.go:650](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/wiring.go:650)); `ApplySSH` is currently demo-only ([cmd/gitid/wiring.go:715](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/wiring.go:715)). Plan 01 would temporarily violate that ownership rule.
- **MEDIUM — `EnsureGlobals` cannot both parse into a key/value map and preserve arbitrary unknown block content.** Such a map loses duplicate directives, comments, blank-line grouping, and potentially multi-value directives. The plan promises that unrecognized directives are never dropped, but the specified representation preserves only one value per key.
- **MEDIUM — The real probe integration test is underspecified.** Setting `HOME` does not necessarily make a constructor given an explicit `sshConfigPath` use that home, and real `ssh -G` may still observe host-level system configuration. The test needs deterministic `-F` behavior or a fake executable on `PATH`.
- **LOW — The task asks to "do not generalise beyond HashKnownHosts" while adding all six policies and a full six-row probe.** This is manageable, but it weakens the tracer boundary and increases Wave 1's already-large blast radius.

## Suggestions

- Replace the provenance taxonomy with only claims the probes can establish: parsed in the resolved user-config graph with exact file/line; effective but source unknown; equal to isolated baseline; inconclusive. Only claim `/etc/ssh/ssh_config` when an explicit system-file parse supports it.
- Add a hermetic behavioral test proving exactly what `-F /dev/null` includes on supported OpenSSH implementations before using it as a compiled-default baseline.
- Introduce `CommitGlobalSSH` and journal-backed async ownership in Plan 01, even if only `HashKnownHosts` is supported. Plan 03 should extend the commit with simulation and re-verification, not replace its architecture.
- Model managed-block contents as parsed directive records retaining raw lines, comments, duplicates, and ordering. Overlay only recognized singleton directives.
- Split Task 1 into backend/core and TUI/wiring commits if necessary. Its listed file surface is very large for a tracer.

## Risk Assessment

**HIGH.** The single-owner refactor is necessary, but incorrect provenance and transitional synchronous writes affect the phase's central safety claims.

---

# Plan 06-02 — Six-option classification and rendering

## Summary

This plan does a good job isolating the exceptional semantics of UseKeychain, IdentitiesOnly, and version-gated `accept-new`. The four-state UI vocabulary and policy-to-fixture parity test are strong. Its main weakness is that the generic state model conflates an explicit user choice with a system-provided effective value, while several failure and version-gating rules remain internally inconsistent.

## Strengths

- The IdentitiesOnly special case is necessary. Existing managed-host parsing already exposes `IdentitiesOnly` per identity ([internal/sshconfig/reader.go:13](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/reader.go:13), [internal/sshconfig/reader.go:50](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/reader.go:50)).
- Keeping policy out of `tuikit` preserves the dependency boundary documented in [internal/tuikit/backend.go:83](/Users/ramon/git/personal/ssh-git-config/internal/tuikit/backend.go:83).
- A policy/fixture parity test in `cmd/gitid` is the correct location because it can import both packages without reversing the render-layer dependency.
- The numeric version-comparison requirement correctly anticipates versions such as `7.10`. Existing version parsing retains strings like `9.7p1`, so a dedicated normalizer is required ([internal/platform/version.go:19](/Users/ramon/git/personal/ssh-git-config/internal/platform/version.go:19), [internal/platform/version.go:40](/Users/ramon/git/personal/ssh-git-config/internal/platform/version.go:40)).
- The selection predicate centralization is good defensive design; it keeps keyboard, mouse, and checkbox behavior aligned.

## Concerns

- **HIGH — "Explicit value" is not the same as "effective value."** The proposed `stateFor(... hasExplicit bool ...)` says any explicitly present value differing from the recommendation is "your choice," regardless of provenance. A system-set value or a value from an unidentified external include is not necessarily the user's deliberate choice. This compounds Plan 01's provenance uncertainty.
- **HIGH — Probe-error behavior conflicts with platform and per-alias states.** The acceptance criteria say a `RunSSHG` error leaves *every* row `StateNeedsAction`, but UseKeychain is file-only and IdentitiesOnly is derived from managed blocks. A failure in generic `ssh -G` should not invalidate independent evidence.
- **MEDIUM — Zero managed identities as `AlreadySet` is semantically misleading.** "Nothing to verify" is not conformance. A separate informational/nothing-to-check note can coexist with a non-actionable state, but calling the recommendation already set is inaccurate.
- **MEDIUM — Unknown SSH version is treated as available.** This preserves advisory behavior but permits selecting and writing `accept-new` when compatibility is unproven. That conflicts with D-13's stated goal to gate the fix on OpenSSH ≥7.6.
- **MEDIUM — The plan introduces two different meanings for `NotApplicable`: wrong platform and unsupported OpenSSH version.** Those have different remediation and explanation needs. Reusing the enum is fine, but the DTO needs a reason code or message so copy cannot drift.
- **LOW — Running three probes on each activation may produce visible latency.** Each invocation is allowed up to three seconds. Sequential calls could approach six seconds before version probing and file work. The plan postpones the loading decision until implementation instead of specifying concurrency or caching.

## Suggestions

- Derive state from both source class and value: user-explicit recommended; user-explicit differs; externally effective recommended/differs; unset/default; not applicable/inconclusive. The UI can still map these into four visual states while preserving truthful wording.
- Treat probe failures per option and per evidence source. Do not downgrade file-only or per-alias results because an unrelated probe failed.
- Represent empty IdentitiesOnly inventory as non-selectable "nothing to verify," not "already set."
- Make unknown version an explicit "compatibility unverified" state and disable the write unless a separate safe fallback is defined.
- Run independent probes concurrently under one overall timeout, or cache results for the screen activation. Record worst-case and typical latency in tests.

## Risk Assessment

**MEDIUM-HIGH.** The UI design is strong, but state semantics could misattribute externally supplied values to the user and allow an unverified version-dependent write.

---

# Plan 06-03 — Shadow simulation, transaction, and Options PTY

## Summary

This plan captures the phase's most valuable concept: prove the recommendation before writing and re-verify afterward. Async commit ownership and journal rollback fit the existing architecture well. The proposed static shadow-source mechanism, however, cannot be implemented with the cited APIs, and simulating only a temporary target file will not reproduce the effective live configuration graph in Include-based layouts.

## Strengths

- Async ceremony handling is already supported and behaves as the plan expects: confirmation enters pending state, and success/failure are delivered explicitly ([internal/tuikit/ceremony.go:250](/Users/ramon/git/personal/ssh-git-config/internal/tuikit/ceremony.go:250), [internal/tuikit/ceremony.go:268](/Users/ramon/git/personal/ssh-git-config/internal/tuikit/ceremony.go:268)).
- Reusing `mutationJournal` is appropriate. It snapshots files before mutation and restores via `WriteNoBackup`, preserving timestamped recovery artifacts ([cmd/gitid/wiring.go:1576](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/wiring.go:1576), [cmd/gitid/wiring.go:1858](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/wiring.go:1858)).
- Rejecting IdentitiesOnly at the backend, rather than merely disabling it in the UI, is a valuable defense-in-depth measure.
- The plan correctly distinguishes inconclusive simulation from proven shadowing; it does not turn a failed probe into a factual warning.
- Real-binary PTY coverage is aligned with the project's established delivery rules.

## Concerns

- **HIGH — `shadowSourceFor` cannot be built from `AllHostStanzas` as specified.** `HostStanza` contains only `Alias` and `Hostname`; it exposes neither arbitrary directive text nor source line numbers ([internal/sshconfig/reader.go:101](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/reader.go:101), [internal/sshconfig/reader.go:117](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/reader.go:117)). The plan promises to name the option-setting directive and line, which requires a new parser API.
- **HIGH — Simulating only the candidate target file does not reproduce the live config graph.** Under the Include layout, the candidate is `config.d/gitid.config`, while the real entry point is `~/.ssh/config`, which may contain directives before or after the Include. Running `ssh -F <candidate>` cannot detect shadowing from that main file. The current storage resolver explicitly distinguishes the main entry point from the managed target ([cmd/gitid/wiring.go:2319](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/wiring.go:2319), [cmd/gitid/wiring.go:2340](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/wiring.go:2340)).
- **HIGH — The simulation API receives candidate bytes but no resolved layout or complete source graph.** It therefore cannot create a faithful temporary mirror containing the main config, included managed file, and relevant include paths.
- **MEDIUM — Static naming remains Include-unaware.** Existing matching helpers explicitly document single-file scope ([internal/sshconfig/reader.go:136](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/reader.go:136)). Even after adding directive values, the scan may name the wrong line when nested Includes or `Match` blocks participate.
- **MEDIUM — Navigation while a commit is in flight is described ambiguously.** The plan says recommendations never block navigation, then allows blocking tab changes while the commit is in flight. That is reasonable transaction safety, but the success criterion should distinguish "recommendations do not gate other workflows" from "an active mutation temporarily captures input."
- **LOW — Captured UI frames are omitted from frontmatter `files_modified`.** The action requires committing phase-frame files, but only the test source and Makefile are declared.

## Suggestions

- Simulate a complete temporary configuration graph: create a private mirror root; copy/rewrite the real main config; replace only the resolved managed target; preserve Include ordering; invoke `ssh -F <temporary-main-config>`.
- Add a dedicated parser API returning directive key, value, host/match scope, source path, and line number. Treat naming as best-effort and never promise a line unless it is actually known.
- Collapse Plan 01's write architecture into this async commit model so no synchronous interim implementation exists.
- Add PTY cases for probe timeout/inconclusive preview and commit failure/retry, not only clean and shadowed success.
- Explicitly list `ui-frames/*` in plan artifacts/frontmatter or document that generated evidence is intentionally discovered rather than statically enumerated.

## Risk Assessment

**HIGH.** The transaction model is strong, but the pre-write proof—the phase's defining guarantee—would not actually model the same configuration the live `ssh` process reads.

---

# Plan 06-04 — Storage migration and banner removal

## Summary

Wiring the existing migration engine is sensible, and the plan correctly recognizes its documented concurrency precondition. The proposed digest check is directionally right but must occur before the current backup writes, because those writes already replace both files. More importantly, the migration engine currently excludes `_global`; unless changed, the storage screen will migrate identities but strand the global options block in the old layout.

## Strengths

- The plan correctly traces a real documented blocker. `Migrate` explicitly says concurrent edits can be silently overwritten and must be addressed before interactive wiring ([internal/sshconfig/migrate.go:219](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:219)).
- Content digests are preferable to timestamps and correctly detect same-size/same-tick edits.
- Retaining the destination-first, source-second ordering preserves the existing crash-safety model ([internal/sshconfig/migrate.go:196](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:196), [internal/sshconfig/migrate.go:285](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:285)).
- Avoiding a second journal around `Migrate` is the right ownership choice because the engine already has rollback authority.
- The plan correctly reuses the layout resolver that currently determines the managed write target ([cmd/gitid/wiring.go:2332](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/wiring.go:2332)).

## Concerns

- **HIGH — The global block is explicitly excluded from migration.** `movableBlockNames` excludes `_global` and every reserved block ([internal/sshconfig/migrate.go:419](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:419)). The plan's earlier registry change makes `global-ssh` reserved too, so it will also be excluded. This directly contradicts "all gitid blocks move here," the real preview, and the expectation that future fixes and creates share one block in the selected layout.
- **HIGH — The proposed concurrent-modification check is too late if Step 2 remains unchanged.** Step 2 calls `WriteFile` on both files with their snapshotted bytes to obtain backups ([internal/sshconfig/migrate.go:265](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:265)). `WriteFile` is `filewriter.Write`, which both backs up and atomically replaces the target ([internal/sshconfig/migrate.go:89](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:89), [internal/filewriter/filewriter.go:47](/Users/ramon/git/personal/ssh-git-config/internal/filewriter/filewriter.go:47)). An external edit after preflight but before Step 2 can therefore be overwritten before the proposed pre-destination/pre-source checks run.
- **HIGH — Rollback on a detected external edit could erase that edit.** The plan says abort through the existing rollback and restore pre-transaction bytes. If the changed bytes belong to another process, restoring the stale snapshot destroys the very modification the detector found. Concurrency abort handling must distinguish gitid's own partial writes from untouched externally changed files.
- **MEDIUM — The "actual resulting preview" has no defined pure planning API.** Current compose helpers are unexported and embedded inside `Migrate` ([internal/sshconfig/migrate.go:443](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:443), [internal/sshconfig/migrate.go:455](/Users/ramon/git/personal/ssh-git-config/internal/sshconfig/migrate.go:455)). The backend cannot truthfully preview the result by merely reading current files; it needs an exported non-mutating migration plan or equivalent.
- **MEDIUM — Step 2 creates backups by rewriting unchanged content.** This unusual mechanism complicates digest reconciliation and concurrent-edit handling. A dedicated backup-only seam would make the transaction easier to reason about.
- **LOW — Scope attribution is muddled.** The plan says storage requirements were satisfied earlier and are not reopened, but it delivers a real storage UI, migration hardening, and PTY coverage under GSSH-01. Traceability should also cite the relevant STORE requirements, even if they were previously validated.

## Suggestions

- Change migration classification to distinguish: movable identity blocks; movable global block; non-movable Include wiring. Both `global-ssh` and legacy `_global` should migrate or be adopted into the destination, then ordered last.
- Add a true backup-only dependency to `Migrate`. Perform the digest comparison immediately before backup and immediately before each content-changing write.
- On concurrent modification, never restore a file that gitid has not yet changed. If gitid has already changed the other file, restore only gitid's writes while preserving the externally edited file.
- Extract `PlanMigration` as a pure function returning source/destination bytes, diffs, targets, and validation inputs. Use exactly that output for both preview and commit.
- Add acceptance tests specifically proving the global block moves in both directions and no copy remains in the old file.

## Risk Assessment

**HIGH.** The current migration code contradicts the planned layout semantics, and the proposed concurrency fix could itself overwrite external edits.

---

# Plan 06-05 — CLI, visual gate, and exit battery

## Summary

This plan closes important parity and delivery obligations, and it correctly reuses the existing command and visual-regression infrastructure. It is nevertheless oversized for a final wave and leaves several CLI contracts insufficiently pinned: exact command paths, JSON schema/versioning, exit-status behavior for advisory shadowing, and how Cobra commands receive the same backend instance as the TUI.

## Strengths

- Replacing the reserved noun is grounded in current source: `gitid ssh` is presently a placeholder ([cmd/gitid/main.go:95](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/main.go:95), [cmd/gitid/identity.go:97](/Users/ramon/git/personal/ssh-git-config/cmd/gitid/identity.go:97)).
- The bidirectional parity matrix is an established, machine-checked contract rather than documentation-only bookkeeping ([docs/cli-parity-matrix.md:3](/Users/ramon/git/personal/ssh-git-config/docs/cli-parity-matrix.md:3), [docs/cli-parity-matrix.md:25](/Users/ramon/git/personal/ssh-git-config/docs/cli-parity-matrix.md:25)).
- Reusing the TUI plan/commit seams is the right architectural goal and avoids a second mutation pipeline.
- Extending one visual registry is consistent with the existing implementation, which currently merges create, Git, and identity-manager specs ([internal/screenshot/createflow.go:535](/Users/ramon/git/personal/ssh-git-config/internal/screenshot/createflow.go:535)).
- Negative controls and explicit non-applicability records are strong defenses against vacuous visual gates.

## Concerns

- **HIGH — Exact CLI syntax remains ambiguous despite being declared contractual.** The plan describes "a read verb under an `options` sub-noun" and "a write verb," but does not pin names such as `list`, `apply`, `show`, or `fix`. The context examples suggest `options list` and `options fix`; the plan must state the exact tree before implementation and parity-matrix edits.
- **HIGH — Advisory shadowing exit semantics conflict with the phase posture.** The plan says post-write shadowing affects exit status, but D-04/D-14 characterize recommendations and shadowing as advisory, never blocking. It does not specify whether a successfully backed-up write with an advisory exits 0, a dedicated nonzero code, or an error. Scripts need a stable contract.
- **MEDIUM — "Same function" parity is weaker than "same transactional core."** TUI methods return `tea.Cmd`, which is UI transport. Having Cobra invoke a `tea.Cmd` directly is possible but awkward and couples CLI execution to Bubble Tea message types. A UI-free `runGlobalSSHCommit` core should be shared, with thin TUI and CLI adapters.
- **MEDIUM — Machine-readable schema is not frozen.** The plan lists required fields but does not define the JSON envelope, state serialization, ordering, versioning, or error representation. Existing parity documentation calls read JSON "frozen" for identity commands ([docs/cli-parity-matrix.md:30](/Users/ramon/git/personal/ssh-git-config/docs/cli-parity-matrix.md:30)); the new output deserves the same rigor.
- **MEDIUM — Visual-gate scope risks mixing historical HTML back into the target.** The existing registry supports live, approved TUI, and approved HTML surfaces ([internal/screenshot/createflow.go:551](/Users/ramon/git/personal/ssh-git-config/internal/screenshot/createflow.go:551)), while project policy says the Bubble Tea dummy is the Phase 3–10 authority. Global SSH specs must explicitly avoid making HTML parity a required outcome.
- **MEDIUM — The final wave is too broad.** It combines a new CLI hierarchy, JSON output, headless writes, shell completion, e2e, visual-regression registry changes, four negative controls, full gate execution, and review-packet generation. Failures will be difficult to attribute and commits difficult to keep coherent.
- **LOW — The plan uses legacy `grep` in acceptance criteria despite repository instructions preferring `rg`.** This is minor but should be cleaned up.

## Suggestions

- Freeze the command tree in the plan, e.g. `gitid ssh options list [--json]`, `gitid ssh options apply <keys...> [--dry-run] [--yes]`, `gitid ssh storage show [--json]`, `gitid ssh storage migrate --to <include|in-file> [--dry-run] [--yes]`.
- Extract UI-free domain operations: `PlanGlobalSSHApply`, `RunGlobalSSHApply`, `PlanStorageMigration`, `RunStorageMigration`. The TUI wraps results in `tea.Cmd`; Cobra calls the same core synchronously.
- Define and freeze the JSON schema, including enum strings and schema version.
- Specify exit codes: success with no advisory; success with advisory; validation/refusal; write/rollback failure.
- Split visual-gate work and CLI work into separate plans or at least separate tasks/commits with independent verification.
- Make approved HTML explicitly non-applicable for all Phase 6 real-vs-dummy comparisons unless retained only as historical evidence.

## Risk Assessment

**MEDIUM-HIGH.** The closing controls are valuable, but the wave is overpacked and leaves externally visible CLI behavior insufficiently specified.

---

## Cross-plan dependency and scope findings

### Strengths

- Sequential waves match the repository's whole-module hook constraint.
- The progression from tracer → full classifier → transactional ceremony → storage → parity/gates is conceptually coherent.
- Each plan includes focused tests, integration tests, and phase-level verification.
- Safety-sensitive changes generally include rollback, backup, and negative-control thinking.

### Concerns

- **HIGH — Plan 01 and Plan 03 assign the same write path two architectures.** Implement the final async transactional ownership from the tracer onward.
- **HIGH — Plan 01's reserved-name change causes Plan 04's current migration filter to exclude the new global block automatically.** This cross-wave interaction is not addressed.
- **HIGH — The provenance and simulation designs assume facts about `ssh -F` and configuration sources that the cited source does not prove.**
- **MEDIUM — The scope materially exceeds GSSH-01.** Storage migration hardening, full storage UI wiring, CLI taxonomy, visual-regression framework extension, and review-packet assembly may be justified, but requirements and roadmap traceability should reflect that expanded scope.
- **MEDIUM — Each plan has low confidence and 120k–150k estimated tokens.** Five sequential low-confidence plans create significant schedule and integration risk. Smaller plans would improve reviewability and rollback.

### Recommended disposition

**Request changes before execution.** The minimum blocking revisions:

1. Redesign provenance so labels never claim `/etc` without proof.
2. Prove or replace the `-F /dev/null` baseline assumption.
3. Implement async journal-backed commit ownership in the tracer.
4. Build simulation from a complete temporary config graph.
5. Add a directive-aware, line-aware parser for shadow-source naming.
6. Make the global block migrate with identities while keeping Include wiring stationary.
7. Move concurrency checks before backup writes and preserve external edits on abort.
8. Extract pure migration planning for truthful previews.
9. Freeze the exact CLI tree, JSON schema, and advisory exit-status contract.
10. Reduce wave size or split the two largest closing plans.

Until those are addressed, the phase has **overall HIGH implementation risk**, despite excellent verification intent and strong underlying decomposition.

---

## OpenCode Review (xai-grok)

## Review: Phase 6 plans 06-01 … 06-05

### Document Type
GSD · plan (five sequential waves)

### Files Read
- `internal/sshconfig/renderer.go`, `writer.go`, `include.go`, `migrate.go`
- `cmd/gitid/wiring.go` (`Persist`, `DemoBanner`, `storage()`, `writeSSHBlock`)
- `cmd/gitid/identity_create.go`, `main.go`
- `internal/tuikit/backend.go`, `store.go`, `design.go`
- `internal/doctor/checks/overlap.go`
- `.planning/phases/06-global-ssh-options/06-UI-SPEC.md`

### Summary
The five plans match GSSH-01, D-01–D-16, and the UI-SPEC 2-state ceremony (not FIELDS.md's 6 screens). Wave 1 correctly treats D-06 (single `Host *` owner) as the load-bearing tracer. Grounding against current code is mostly accurate: `RenderGlobalBlock` still emits `_global` with `IgnoreUnknown` *inside* `Host *` (`renderer.go:118-127`); `Persist` still reduces `ApplySSH`/`SetSSHStorage` in memory (`wiring.go:715-720`); `Migrate` still documents the concurrent-edit precondition (`migrate.go:219`); reserved `ssh` noun is still a placeholder (`main.go:101`). Two issues would mis-execute if left as written: **06-01 makes `Persist(ApplySSH)` the write path while 06-03 makes `CommitGlobalSSH` the write path** (existing Git/delete pattern is commit-then-re-read); **create still feeds `RenderGlobalBlock` from `identity_create.go:227` and `wiring.go:2594`, which 06-01 does not list**. The `"_global"` grep AC is also unachievable without rewriting every fixture.

### Strengths

- Tracer on `HashKnownHosts` is the right cut: no platform/version/per-alias special case; D-06 only shows up on a real probe+merge+write+reread.
- UI-SPEC Known Divergence is treated as binding; no resurrection of `v/f/w/y/z`.
- Seam split (`GlobalSSHPlanner` vs `SSHStoragePlanner`) matches `IdentityPlanner` (`backend.go:23-49`) and the existing "real backend must not embed Noop" rule.
- D-04 simulation vs static naming, D-08 dual reserved names, D-14 stateless classifier, D-15 empty opt-in, and 06-04 content-digest concurrent-mod check are correctly sequenced and threat-modeled.
- 06-05 CLI-through-same-chokepoint + parity-matrix + visual-gate negative controls match Phase 5.

### Concerns

**CRITICAL**
- **Two write paths for ApplySSH** — Location: 06-01 Layer 4 `persistApplySSH` vs 06-03 Task 2 `CommitGlobalSSH`. Required: one write authority. Why: `Persist(ConfigureGit)` is already re-read-only after `CommitGit` (`wiring.go:679-684`). 06-01 tests `Persist(ApplySSH{…})` as the disk write; 06-03 then writes in `CommitGlobalSSH` and *also* dispatches `ApplySSH`. Executed as written, success writes twice (second backup, race with ceremony receipt) or 06-03 tests fail if 06-01's persist still mutates.

**HIGH**
- **Create path not fully retargeted** — Location: 06-01 Layer 2 "repoint Write / writeSSHBlock". Required: every create site calls `EnsureGlobals`. Why: live callers still do `GlobalBlock: sshconfig.RenderGlobalBlock(...)` in `identity_create.go:227` and `wiring.go:2594`. `Write` still whole-replaces the `_global` body (`writer.go:47-49`). Missing those files leaves D-06 false: the next create still erases a GSSH fix.
- **`"_global"` grep AC vs fixtures** — Location: 06-01 Task 2 AC. Required: either narrow the grep to production comparisons or list every fixture. Why: `"_global"` appears in `e2e/harness_test.go`, `wiring_test.go`, `include_test.go`, `delete.go` comments, doctor tests, etc. Task 2 `files_modified` does not cover them; the stated grep would fail.
- **Linux empty-block contract** — Location: `writer.go:31-32`. Required: document whether `EnsureGlobals` always emits a `Host *` on Linux when overlaying `HashKnownHosts`. Why: today empty `globalBlock` writes nothing and leaves a prior `_global`. A Linux apply of one option must not drop darwin-only keys on a synced config, and must not skip `IgnoreUnknown` when `UseKeychain` is present (D-11).

**MEDIUM**
- 06-01 Task 1 is a full-stack tracer plus DTO/ceremony/wiring; 06-01 Task 3 then revisits `persistApplySSH` placement. Split is justified by L11 but the persist contract must be frozen in Task 1 as "temporary write, replaced by async commit in 06-03" or 06-03 as "Persist re-read only."
- 06-02 maps OpenSSH-too-old onto `StateNotApplicable` (same as darwin-only UseKeychain). Copy must not say "macOS-only" for that row; plan does not pin distinct words.
- 06-04: engine rollback is the sole restore authority (good) but `CommitSSHStorage` must not also open `mutationJournal` — plan says that; keep it in the AC.
- 06-05 receipt states may be non-applicable in the in-process visual gate (Phase 5 precedent). Plan allows it; ROADMAP success criterion 3 still needs PTY frames from 06-03/06-04 explicitly listed as the evidence surface.

**LOW**
- `RenderGlobalBlock` tests (`renderer_test.go:132-156`) will go red if the function is deleted; plan says keep-or-delete — pick delete+repoint in 06-01 to avoid two renderers.
- Recipe `Host *` at top vs gitid last-block (D-09) is well documented; do not "fix" toward the recipe.

### Suggestions
1. Rewrite 06-01 `persistApplySSH` to match Git: **or** mark it tracer-only and have 06-03 replace the Persist arm with `setPersistErr(nil); return b.InitialState()` and move the isolated-HOME write tests onto `CommitGlobalSSH`.
2. Add `cmd/gitid/identity_create.go` and `CreateInput.GlobalBlock` construction in `wiring.go:2594` to 06-01 files + AC: create after fix keeps `HashKnownHosts`.
3. Change Task 2 grep to production `== "_global"` / string compares, excluding `_test.go` and `.gitignore_global`.
4. Pin Linux `EnsureGlobals` body: always `IgnoreUnknown` first; overlay only requested keys; darwin defaults only when `SupportsUseKeychain`.
5. 06-02: separate not-applicable *word* for version-gate vs platform-gate.
6. 06-03: FakeSSHDir must answer both `ssh -G` and `ssh -G -F` or shadowing ACs go vacuous (plan already says this — keep it as a hard AC).
7. 06-05: assert CLI holds `*realBackend` and calls `CommitGlobalSSH` by construction; do not duplicate disk assertions as a second write path.

### Risk Assessment

**MEDIUM-HIGH** until the Persist vs Commit split and create-site retarget are fixed; **MEDIUM** after that.

Justification: architecture, advisory posture, shadowing honesty, migration precondition, CLI parity, and UI-wave gating are sound and well grounded in `renderer.go:118-127`, `writer.go:15-32`, `wiring.go:627-720`, `include.go:84-85`, `migrate.go:219`, `main.go:101`. The remaining defects are execution-order / ownership bugs that would silently violate D-06 (create erases fixes) or D-16 (double write / false receipt), which are exactly the failure modes this phase exists to close.

### Status: Issues Found — fix 06-01 persist ownership + create call sites + `_global` grep AC, then re-invoke

---

## Consensus Summary

Both reviewers independently obtained real repo access and grounded their findings in `path:line` citations (no `[reviewed-without-repo-access]` or `[reviewed-without-source-citations]` markers on either output). Both converge on the same root defect and reach overlapping but not identical risk verdicts: `codex-sol` calls overall phase risk **HIGH**; `xai-grok` calls it **MEDIUM-HIGH until fixed, MEDIUM after**. Treat the shared finding below as the phase's single highest-priority blocker.

### Agreed Strengths

- The tracer-first sequencing (Wave 1 = single `HashKnownHosts` end-to-end before the six-option classifier) is sound: it forces the highest-risk integration point — create silently erasing prior GSSH fixes — to be solved before the surface area grows.
- The backend/TUI dependency boundary is respected: `tuikit.Backend` stays UI-facing-DTO-only, and policy/classification logic is correctly kept out of `tuikit`.
- The transactional/ceremony machinery being reused (`mutationJournal`, async ceremony confirm→pending→success/failure) is architecturally appropriate and matches existing patterns rather than inventing a new one.
- Phase-closing verification posture (parity matrix, visual-gate negative controls, PTY coverage) is taken seriously and grounded in real infrastructure (`docs/cli-parity-matrix.md`, `internal/screenshot/createflow.go`).

### Agreed Concerns

- **HIGH/CRITICAL — Two competing write authorities for the global-SSH mutation.** `codex-sol`: "Plan 01 introduces the wrong mutation architecture… Plan 01 and Plan 03 assign the same write path two architectures" (citing `cmd/gitid/wiring.go:650`, `:715`). `xai-grok`: "**CRITICAL** — Two write paths for ApplySSH… executed as written, success writes twice (second backup, race with ceremony receipt) or 06-03 tests fail if 06-01's persist still mutates" (citing `wiring.go:679-684`). Both reviewers independently traced the same conflict between 06-01's `persistApplySSH`/`Persist(ApplySSH)` and 06-03's `CommitGlobalSSH`, and both call it a top-severity blocker with a concrete failure mode (double write / broken async ownership). This is the one finding that must be resolved — freeze the async, journal-backed commit as the ownership model from Wave 1 onward — before 06-01 and 06-03 are executed as currently written.
- **MEDIUM (shared theme, different angle) — Wave sizing and scope risk.** `codex-sol` flags cross-plan token/confidence risk ("Each plan has low confidence and 120k–150k estimated tokens… five sequential low-confidence plans create significant schedule and integration risk") and calls 06-05 "oversized for a final wave." `xai-grok` similarly flags 06-01 Task 1 as "a full-stack tracer plus DTO/ceremony/wiring" that later gets revisited. Neither treats this as blocking on its own, but both independently see the plan set as heavier than its GSSH-01 scope suggests.

### Divergent Views

- `codex-sol` raises several HIGH concerns `xai-grok` does not surface at all: the provenance classifier's inability to prove "system-set" without proof of `-F /dev/null` semantics (06-01/06-02), the `shadowSourceFor` API gap in 06-03 (`HostStanza` lacks directive/line data), the simulated-file-vs-live-graph gap in 06-03, and — notably — that `movableBlockNames` in `internal/sshconfig/migrate.go:419` currently **excludes** `_global` (and will exclude the new `global-ssh` reserved name), so 06-04's migration would silently strand the global block in the old layout. This last point is a concrete, citation-backed HIGH finding that `xai-grok` did not check and is worth independent verification before execution.
- `xai-grok` raises HIGH concerns `codex-sol` does not surface: the create-site retarget gap (`identity_create.go:227`, `wiring.go:2594` still call `RenderGlobalBlock` directly, so 06-01 leaves D-06 unfixed for the create path specifically), the `"_global"` grep acceptance criterion being unachievable against existing test fixtures, and the Linux empty-block/`IgnoreUnknown` ordering question in `writer.go:31-32`.
- Overall risk verdict differs in degree: `codex-sol` treats the phase as HIGH risk outright pending ten listed blocking revisions; `xai-grok` treats it as MEDIUM-HIGH now, downgradable to MEDIUM once the write-path and create-site issues are fixed. The divergence tracks each reviewer's respective focus (codex-sol weighted the provenance/simulation mechanisms more heavily; xai-grok weighted the create-path and migration-registry mechanics more heavily) rather than a disagreement about any single fact.

**Recommendation:** before executing 06-01/06-03, resolve the shared CRITICAL/HIGH write-path-ownership finding, and separately verify both reviewers' independent HIGH findings against source (`migrate.go:419` global-block exclusion per codex-sol; `identity_create.go:227` / `wiring.go:2594` create-site retarget per xai-grok) since neither reviewer cross-checked the other's citation.
