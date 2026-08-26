---
phase: 6
reviewers: [codex-sol, xai-grok]
reviewed_at: 2026-08-26T20:34:09Z
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

# Cross-AI Plan Review — Phase 6 (Cycle 5)

> Note on `codex-sol`: `.planning/config.json` pins this instance's model to
> `openai/gpt-5.6-sol-fast`, but the Codex CLI on this host authenticates via a
> ChatGPT account, which again rejected that model with the same
> `invalid_request_error` as Cycles 1-4. The review below was produced by
> re-invoking the `codex` lane without the model override, so it ran on the
> account's default resolved model (`gpt-5.6-sol`, reasoning=low). This is now
> five cycles running into the same misconfiguration; the
> `review.reviewer_instances` entry for `codex-sol` should be corrected before
> any future review run.

This is Cycle 5 — two cycles beyond the normal 3-cycle default, continued
because every prior cycle showed genuine convergence on well-defined,
verifiable findings (14 → 9 → 4 → 2 → 0). Cycle 4 found 0 HIGH and 2 small
items: a preview-path transient-read consistency gap in `06-05-PLAN.md`
(MEDIUM) and an implicit `host.Patterns` → `[]string` type-conversion detail
in `06-04-PLAN.md` (LOW). The planner responded by having
`SSHStorageMigrationPlan` acquire `txMu` before resolving layout and before
calling `PlanMigration` — reframing what `txMu` guards from "the transaction"
to "the file pair's coherence boundary" — releasing it before returning to
the UI, and forbidding `runSSHStorageMigrate` (which already holds `txMu`)
from ever calling the preview method (its CLI branch now calls
`sshconfig.PlanMigration` directly instead). The concurrency test was
strengthened to pause the migration engine between its destination write and
source trim, run a concurrent preview, and assert *content coherence*
(`SourceBefore`/`DestBefore` show each managed block in exactly one file) —
not merely the absence of a Go-level data race. `06-04-PLAN.md` now spells
out the `host.Patterns` → `[]string` conversion explicitly, including
negation-prefix (`!`) handling, and pins the zero-pattern/synthetic-wildcard
guards to stay at `aliasCollides`.

Both reviewers independently re-verified both fixes against the current plan
text and the live source (`internal/sshconfig/migrate.go`,
`internal/sshconfig/validation.go`, `cmd/gitid/lifecycle.go`,
`cmd/gitid/wiring.go`), including tracing every current `txMu` holder to
confirm none of them calls the preview method (which would self-deadlock).
Both confirm both Cycle 4 items are fully resolved with no new HIGH, MEDIUM,
or actionable LOW introduced. **This cycle found zero actionable findings.
The orchestrator independently re-verified both fixes against source as well
(see "Orchestrator Verification" below) and agrees. This is the last review
cycle for this phase.**

## Codex Review (codex-sol)

# Cycle 5 Plan Review

## Summary

The seven plans are implementation-ready and collectively achieve Phase 6's goal. I found no remaining actionable HIGH, MEDIUM, or LOW issue.

Both Cycle 4 findings are genuinely resolved:

- The storage preview now takes `txMu` across layout resolution and `PlanMigration`, preventing reads between migration's destination write and source trim. The lock order remains one-way: `txMu` → `pendingMigrationMu`. No planned `txMu` holder calls the preview method, so the change introduces no self-deadlock.
- The matcher extraction now specifies the required `[]*ssh_config.Pattern` → `[]string` projection, preserves `!` prefixes, and keeps parser-specific wildcard guards at the existing caller.

Overall risk remains MEDIUM because this is a large, security-sensitive phase involving SSH configuration and two-file migrations—not because the plans retain a known design defect.

## Plan-by-plan assessment

### 06-01 — End-to-end tracer and globals ownership

Strong foundation. It correctly identifies the existing split ownership: `sshconfig.Write` receives a separately rendered global block, while create, rotate, and repair reach it through different call patterns. The relevant mutation paths are visible in `internal/identity/identity.go` and `internal/sshconfig/writer.go`.

Strengths:

- Establishes one write ceremony and one globals renderer.
- Deletes the competing `RenderGlobalBlock` route rather than leaving a deprecated alternative.
- Preserves the existing `txMu` transaction pattern used by rotate, repair, delete, create, and Git writes (`cmd/gitid/lifecycle.go:165-167`, `:327-329`, `:552-554`; `cmd/gitid/wiring.go:1232-1235`, `:2800-2803`).
- Correctly treats provenance as evidence-dependent rather than inferring `/etc/ssh/ssh_config`.
- Pins rollback through the existing journal mechanism.

No actionable concerns.

### 06-02 — Reserved names and migration classification

The co-location of registry consolidation and migration correction is sound. The current migration classifies and composes blocks in one transaction, so changing reserved-name behavior without changing movement behavior would strand globals.

Strengths:

- Separates "reserved" from "movable global," avoiding accidental movement of Include wiring.
- Tests both migration directions and ambiguous duplicate-global states.
- Preserves existing migration ordering rather than changing registry behavior in isolation.

No actionable concerns.

### 06-03 — Six-option classifier and rendering

The state model now cleanly separates effective value, visual state, and attribution.

Strengths:

- Equality precedes source-based branching, preventing `ForwardAgent no` from becoming a false warning merely because it came from OpenSSH's baseline.
- Probe failures invalidate only dependent rows.
- `UseKeychain` and `IdentitiesOnly` receive dedicated evidence paths.
- Version-unverified `accept-new` is refused rather than assumed compatible.
- Fixture-policy parity and enum parity tests reduce drift across package boundaries.

No actionable concerns.

### 06-04 — Whole-graph simulation and matcher extraction

The graph simulation is detailed and appropriately conservative. It models Include splice order and first-obtained-value semantics rather than scanning flat files.

The Cycle 4 matcher issue is resolved:

- The existing parser exposes `host.Patterns` as parser pattern objects and currently calls `pat.String()` inside the inline negation loop (`internal/sshconfig/validation.go:190-216`).
- The revised plan explicitly collects those strings into a capacity-sized `[]string`, retaining `!`, before calling `HostPatternsMatch` (`06-04-PLAN.md:161-168`, `:223-232`).
- It correctly leaves the zero-pattern and synthetic-wildcard guards at `aliasCollides`, matching their current location (`internal/sshconfig/validation.go:190-197`).
- Regression tests retain the existing `AliasCollision` behavior while adding direct tests for the shared matcher.

Other strengths:

- Distinguishes recursion-stack cycle detection from global expansion memoization.
- Linearizes files around Include directives, including line offsets.
- Makes inconclusive simulation explicit.
- Pairs positive and negative precedence controls at unit and PTY levels.

No actionable concerns.

### 06-05 — Migration hardening and Storage UI

The Cycle 4 preview-consistency issue is resolved without introducing a new deadlock.

Source evidence for the original gap:

- Migration writes the destination first (`internal/sshconfig/migrate.go:285-301`).
- It trims the source later (`internal/sshconfig/migrate.go:304-320`).
- Therefore a concurrent two-file read can observe blocks in both files.

The revised lock design is mechanically consistent:

- `txMu` is explicitly redefined as the file-pair coherence boundary (`06-05-PLAN.md:113-120`).
- `SSHStorageMigrationPlan` takes `txMu` before `b.storage()` and retains it through `PlanMigration` and pending-plan publication (`06-05-PLAN.md:319-321`).
- Pending-plan access uses a separate `pendingMigrationMu`; acquisition order is always `txMu` then `pendingMigrationMu` (`06-05-PLAN.md:124-128`).
- Pending-plan helpers never acquire `txMu` or hold their mutex across I/O (`06-05-PLAN.md:124-133`).
- `runSSHStorageMigrate`, which already holds `txMu`, is forbidden from calling `SSHStorageMigrationPlan`; its CLI branch calls `sshconfig.PlanMigration` directly.
- The plan adds a construction check proving `lifecycle.go` contains no call to the preview method (`06-05-PLAN.md:359-361`).

I also traced the current `txMu` holders. They are rotate, repair, delete, Git commit, and create commit (`cmd/gitid/lifecycle.go:165-167`, `:327-329`, `:552-554`; `cmd/gitid/wiring.go:1232-1235`, `:2800-2803`). None currently calls a storage-preview method. The planned global-apply and storage-migrate holders likewise do not call it. No reverse `pendingMigrationMu` → `txMu` path is specified.

The strengthened test is appropriate: it pauses between the two migration writes, starts preview concurrently, verifies that preview remains blocked, then asserts each managed block occurs in exactly one file after preview completes. That checks filesystem coherence, which `go test -race` alone cannot observe.

No actionable concerns.

### 06-06 — CLI parity and frozen contracts

The CLI plan is precise enough for a public interface.

Strengths:

- Freezes command paths, flags, schemas, array ordering, enum strings, and exit semantics.
- Routes Cobra directly through the UI-free ceremonies rather than through `tea.Cmd`.
- Makes successful-but-advisory writes exit zero by default, with an explicit stricter flag.
- Uses exact JSON key-set tests, preventing undocumented wire-format drift.
- Covers adaptive TTY behavior through the existing resolver.

No actionable concerns.

### 06-07 — Visual gate and phase closure

The closing plan correctly treats the Bubble Tea dummy as the only parity target and uses PTY evidence for states requiring real writes.

Strengths:

- Registers explicit HTML non-applicability.
- Requires specific PTY frame files for uncapturable in-process states.
- Includes missing-state, unclassified-difference, mutation-sensitivity, and cross-surface leakage controls.
- Requires a manual allowlist-removal failure check.
- Separates evidence assembly from the orchestrator's responsibility to run cross-AI review.

No actionable concerns.

## Concerns

No actionable findings.

Residual execution risks:

- The phase remains large: several plans touch transaction code, SSH parsing, TUI state, CLI contracts, and extensive PTY tests.
- Plans 06-04 and 06-05 contain the highest implementation complexity: recursive Include mirroring, resolution-order linearization, rollback behavior, and concurrent plan identity.
- PTY timing and subprocess tests may require careful timeout tuning, but the plans already require bounded waits, deterministic fakes, and explicit timeout reporting.

These are execution risks already addressed by staged waves, TDD, isolated homes, negative controls, and full exit gates.

## Suggestions

- Execute the plans sequentially as specified; do not parallelize changes touching `internal/sshconfig`, `cmd/gitid/lifecycle.go`, or `cmd/gitid/wiring.go`.
- Treat the construction checks in 06-04 and 06-05 as required acceptance evidence, not optional summary material.
- During 06-05 execution, preserve the documented lock order literally and keep `SSHStorageMigrationPlan` out of every transaction-held call graph.
- Require the final 06-07 review packet to include Cycle 5's zero-actionable conclusion alongside prior-cycle closures.

## Risk Assessment

**MEDIUM**

The design is converged, with zero remaining actionable findings. Risk is driven by implementation breadth and the consequences of mistakes in SSH configuration and cross-file migration—not by unresolved plan defects. The two Cycle 4 issues are fully and verifiably resolved, and the fixes introduce no new deadlock or type-conversion problem.

---

## OpenCode Review (xai-grok)

Verifying cycle-4 claims against the plans and live source.

# Phase 6 Plan Review — Cycle 5

## Summary

Cycle-4 items are **closed in the plan text and match live source**. `aliasCollides` is unexported at `internal/sshconfig/validation.go:164`; the stanza loop is inline at `:198–216`; `AliasCollision` is path-shaped at `:160`. Migration still writes dest then source as two `WriteFile`s (`migrate.go:292` then `:313`), so the mid-transaction both-files window is real. Preview `txMu` plus "never call `SSHStorageMigrationPlan` from `runSSHStorageMigrate`" is consistent with current lock sites (`lifecycle.go:166/328/553`, `wiring.go:1233/2801`). **No new HIGH/MEDIUM. Converged.**

---

## Cycle-4 verification

### 1. `06-05` preview transient read — **resolved**

- Source: dest write `:270–299`, source trim `:304–320` (writes at `:292` and `:313`).
- Plan: `SSHStorageMigrationPlan` takes `txMu` across `storage()` + `PlanMigration`; lock released before UI; CLI branch calls `sshconfig.PlanMigration` directly.
- Pause-injected content test is the right check (`-race` cannot see this).

### 2. `06-04` `host.Patterns` → `[]string` — **resolved**

- Live loop: `pat.String()`, `HasPrefix(s, "!")`, then `HostMatch` (`validation.go:200–206`).
- Plan: keep `!` on tokens; trim only inside `HostPatternsMatch`; keep zero-pattern and synthesized-`*` guards at `aliasCollides` (`:191–196`). `HostMatch("*", …)` is true (`:106–107`), so folding the `*` skip into the matcher would break `Host *` shadow naming.

### Deadlock (new `txMu` on preview)

Current `txMu` holders: `runRotate`/`runRepair`/`runDelete` (`lifecycle.go:166/328/553`), `commitCreateTransaction`/`commitGitTransaction` (`wiring.go:1233/2801`). None call a storage preview today. Plan forbids `runSSHStorageMigrate` → `SSHStorageMigrationPlan`. `takePendingMigration` uses only `pendingMigrationMu`. Order `txMu` then `pendingMigrationMu` is consistent. **No new deadlock if those comments stay.**

---

## Strengths

- Tracer + single write authority (`runGlobalSSHApply`) closes D-06 before the six-option surface.
- Provenance limited to what probes prove; isolation contract test is executable.
- Migration three-way classification matches `movableBlockNames` at `migrate.go:419–427` (excludes `globalBlockName` **and** reserved).
- Shadowing table matches floored Include (`include.go`) + last `Host *`.
- One plan object + digests + backup-only seam match the current "backup by rewrite" bug (`migrate.go:266–273`).
- Frozen CLI tree / JSON / exit codes / empty TUI fallback.

---

## Concerns

None actionable at HIGH/MEDIUM.

**LOW (non-blocking):** Wave 1 is still ~35 files / 120k tokens; already accepted (three staged commits). Executor must not invent a second `txMu` helper that wraps `SSHStorageMigrationPlan`.

---

## Suggestions

None required for another plan cycle. Execute sequentially; keep `06-07` closure table derived from `06-REVIEWS.md`, not a hard-coded count.

---

## Risk Assessment

**LOW.** Phase goals (GSSH-01 + DLV-04/06) are covered. Cycle-4 fixes match source. Ready to execute Wave 1.

---

## Orchestrator Verification

The orchestrator independently re-verified both Cycle 4 items against the live repository, before trusting either reviewer's claim, rather than accepting the plan's own summary of what changed:

- **`internal/sshconfig/validation.go:189-197`** confirms `aliasCollides`'s inline stanza loop: `pat.String()` per pattern, `strings.HasPrefix(s, "!")` detection with `strings.TrimPrefix`, and `HostMatch(s, candidate)` — exactly what both reviewers cite, and exactly the projection `06-04-PLAN.md` now spells out explicitly for its extracted `HostPatternsMatch` call.
- **`internal/sshconfig/migrate.go:285-317`** confirms the migration engine performs two separate `deps.WriteFile` calls — the composed destination write (Step 3) followed later by the composed source write/trim (Step 4) — with no cross-file atomicity between them, which is exactly the transient window the Cycle 4 MEDIUM identified and the mid-transaction preview risk both reviewers re-confirmed this cycle.
- **`06-05-PLAN.md`'s `<lock_contract>` section (lines 108-133)** now states explicitly, in binding prose: `txMu` is redefined as "the coherence boundary of the file PAIR," `SSHStorageMigrationPlan` acquires it and holds it across both `b.storage()` and `PlanMigration`, the acquisition order is fixed one-way (`txMu` then `pendingMigrationMu`, never reversed), `pendingMigrationMu` is never held across I/O, and — the specific deadlock-avoidance clause — `runSSHStorageMigrate` (which already holds `txMu` for the whole transaction) is forbidden from calling `SSHStorageMigrationPlan`, with its CLI branch (06-06) instead calling `sshconfig.PlanMigration` directly (lines 319-321). A construction-check acceptance criterion (lines 359-361) requires `rg -n 'SSHStorageMigrationPlan' cmd/gitid/lifecycle.go` to report no match, which is exactly the check that would catch a future regression reintroducing the self-deadlock at a new call site.
- The orchestrator traced every current `txMu` holder cited by both reviewers (`cmd/gitid/lifecycle.go:165-167` rotate, `:327-329` repair, `:552-554` delete; `cmd/gitid/wiring.go:1232-1235` create commit, `:2800-2803` Git commit) and confirms none of them calls a storage-preview method today, and the plan text does not add one that does. No reverse `pendingMigrationMu` → `txMu` acquisition path exists anywhere in the plan text.

**Both Cycle 4 items are FULLY RESOLVED, independently confirmed by two external reviewers and by direct orchestrator inspection of the plan text and live source. No new deadlock, race, or type-conversion defect was introduced by either fix.**

## Consensus Summary

Both reviewers agree, independently and with matching `file:line` citations against `internal/sshconfig/migrate.go`, `internal/sshconfig/validation.go`, `cmd/gitid/lifecycle.go`, and `cmd/gitid/wiring.go`, that both Cycle 4 findings are FULLY RESOLVED:

1. The `06-05-PLAN.md` preview-path transient-read gap is closed: `SSHStorageMigrationPlan` now acquires `txMu` across layout resolution and `PlanMigration`, `runSSHStorageMigrate` is forbidden from calling it (eliminating the self-deadlock risk that would otherwise follow from Go's non-reentrant mutex), and the concurrency test now asserts filesystem content coherence rather than only the absence of a Go-level data race.
2. The `06-04-PLAN.md` `host.Patterns` → `[]string` conversion is now spelled out explicitly, including negation-prefix (`!`) handling, matching the live `aliasCollides` loop it replaces.

Neither reviewer found a new HIGH, MEDIUM, or actionable LOW this cycle. The orchestrator independently re-verified both fixes against source and agrees. **This cycle converges with zero actionable findings — this is the last review cycle for Phase 6.**

### Agreed Strengths
- The `06-05-PLAN.md` `<lock_contract>` redefinition of `txMu` as the file-pair coherence boundary is mechanically sound: fixed one-way acquisition order, no cross-I/O hold of `pendingMigrationMu`, and an explicit, enforceable ban on the one call path that would self-deadlock.
- The `06-04-PLAN.md` matcher-extraction now states the exact type projection required, with negation handling preserved and the zero-pattern/synthetic-wildcard guards correctly left at the existing caller.
- The strengthened 06-05 concurrency test (pause between destination write and source trim, assert content coherence) is the correct fix for a filesystem-consistency defect that `-race` structurally cannot see.
- Both reviewers independently traced every current `txMu` holder and confirm none calls the preview method, closing the deadlock question definitively rather than by assertion.

### Agreed Concerns
None. No HIGH, MEDIUM, or actionable LOW findings this cycle.

### Divergent Views
None. Both reviewers converge on zero actionable findings, and the orchestrator's independent source verification agrees.

### New Actionable (non-HIGH) Findings This Cycle
None.
