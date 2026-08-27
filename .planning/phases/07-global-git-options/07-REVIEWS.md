---
phase: 7
reviewers: [codex-sol, xai-grok]
reviewed_at: 2026-08-27T12:39:09Z
plans_reviewed: [07-01-PLAN.md, 07-02-PLAN.md, 07-03-PLAN.md, 07-04-PLAN.md, 07-05-PLAN.md, 07-06-PLAN.md]
models:
  codex-sol: "openai/gpt-5.6-sol-fast (reasoning=low)"
  xai-grok: "xai/grok-4.6 (reasoning=low)"
model_sources:
  codex-sol: "pinned"
  xai-grok: "pinned"
---

# Cross-AI Plan Review — Phase 7 (CYCLE 2)

> **codex-sol lane failed again**: the configured instance model
> `openai/gpt-5.6-sol-fast` is rejected by the local Codex CLI's ChatGPT-account
> auth (`"The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex
> with a ChatGPT account."`, HTTP 400, returned twice). Identical failure to
> cycle 1 — no review content was produced for this lane; see
> `## Codex Review (codex-sol)` below for the captured stderr. This is a
> reviewer-instance configuration issue (`review.reviewer_instances.codex-sol`
> in `.planning/config.json`), not a plan defect, and it has now recurred across
> two cycles unaddressed. Repoint `codex-sol` at a model this Codex CLI
> installation's auth actually supports, or drop it from
> `review.default_reviewers` until it does.

## Consensus Summary

As in cycle 1, only one reviewer (xai-grok, via OpenCode / Grok-4.6) produced a
source-grounded review; codex-sol failed to run (see note above). Consensus in
the strict sense (2+ reviewers agreeing) cannot be established from a single
successful lane — the findings below are xai-grok's alone, weighted as one
grounded, source-cited voice, not as agreed-by-multiple-reviewers consensus.

xai-grok verified every cycle-1 finding (3 HIGH + 12 actionable) against the
revised plans' `<authority>` blocks and against the live code (`recipes/gitconfig.recipe:66-67`,
`internal/gitconfig/baseline.go:537-634`, `internal/tuikit/globalgit.go:48-58`,
`cmd/gitid/wiring.go:679-682`, `internal/sshconfig/include.go:94`,
`lifecycle.go:796`). **All 3 cycle-1 HIGH findings and all 12 cycle-1 actionable
findings are verdict RESOLVED** — each cites the specific R-1 through R-7
resolution and the code location it lands at. No cycle-1 HIGH remains open.

The independent fresh pass over the revised plans surfaced **1 new MEDIUM and
11 new/carried LOW** concerns, concentrated in 07-01/07-02 (write-path edge
cases) with the rest scattered as minor documentation/test-coverage gaps across
07-03 through 07-06. None block execution; the reviewer's explicit verdict is
**"Safe to execute 07-01"** with overall phase risk **LOW–MEDIUM**.

### Agreed Strengths
(Single grounded reviewer — not cross-reviewer agreement, but xai-grok's
verified strengths, each tied to a specific resolution or code location.)
- Policy-backed selectability for the pre-checked fixture hazard is now
  structural (R-1), matching the live hazard at `internal/tuikit/globalgit.go:48-54`.
- `ComposeBaselineInclude` splits the read/compose/write path so the ceremony
  keeps taking a fresh backup while doctor/dispatcher skip logic is preserved
  (`internal/gitconfig/baseline.go:562-564`, `624-626`) — resolves the cycle-1
  skip-write-vs-fresh-backup HIGH (R-3).
- The D9 fallback author (07-02) mirrors the SSH floor-include pattern exactly:
  one read → compose → write, no two-write window (R-4).
- `merge.conflictstyle` hard-gates to `diff3` when unreadable; pager is never a
  managed row; aliases/colors are recipe-sourced (07-03).
- 07-06's closure table explicitly indexes R-1 through R-7 against the
  allowlist and negative controls.

### Agreed Concerns
(Single grounded reviewer; listed by severity, not cross-reviewer weight.)
- **MEDIUM** — `ComposeBaselineInclude` on the first-run fallback path (07-02)
  writes a floor include pointing at a possibly absent baseline file. The plan
  verifies git ignores a missing include target, but if git *warns* instead,
  creating an empty managed block becomes a second watched file — journal/
  restore tests must cover that branch, not only the happy one-write path.
- 11 LOW-severity items (fixture-literal drift risk, first-run receipt
  distinguishability, TUI-vs-CLI half-apply framing, help-text token naming,
  classifier test pins, harness FS quirks, empty-`chosen` assertion scope,
  wave-sequencing dependencies, and parser fail-closed behavior) — see
  per-plan detail in the `## OpenCode Review (xai-grok)` section below.

### Divergent Views
None — only one reviewer lane produced content this cycle, so there is nothing
to diverge from. The absence of a second independent voice (codex-sol) is
itself the main open risk of this review cycle, not a plan-quality divergence.

---

## Codex Review (codex-sol)

**LANE FAILED — no review content produced.** Captured stderr from the Codex
CLI (model `openai/gpt-5.6-sol-fast`, reasoning=low):

```
ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a ChatGPT account."}}
ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a ChatGPT account."}}
```

Identical failure mode to cycle 1 (see prior `07-REVIEWS.md` history). This is
a reviewer-instance configuration problem (the configured model is
incompatible with this Codex CLI installation's ChatGPT-account auth), not a
finding about the plans.

---

## OpenCode Review (xai-grok)

I'll verify cycle-1 HIGH findings against the live code and the revised R-1–R-7 resolutions.
# Phase 7 plan review — cycle 2 (07-01 … 07-06)

Verified against `recipes/gitconfig.recipe:66-67` (floor `[include]`), `internal/gitconfig/baseline.go:537-634` (skip-on-equal still live), `internal/tuikit/globalgit.go:48-58` (pre-checked fixture + noop `activate`), `cmd/gitid/wiring.go:679-682` (`DemoBanner` still covers Global Git), `internal/sshconfig/include.go:94` / `lifecycle.go:796` (SSH include-create sibling).

---

### Resolution check (cycle 1)

| Finding | Verdict | Evidence |
|---|---|---|
| **07-01 HIGH — pre-checked fixture apply** | **RESOLVED** | **R-1**: empty `chosen` on construct + every `activate()`; selectability requires `PolicyFor`. Matches live hazard at `globalgit.go:48-54`. |
| **07-01 HIGH — skip-write vs fresh backup** | **RESOLVED** | **R-3**: ceremony composes then `filewriter.Write` with no equality skip; `WriteBaselineInclude` keep-skip via extracted `ComposeBaselineInclude`. Matches `baseline.go:562-564`, `624-626`. |
| **07-02 HIGH — missing floor-include anchor** | **RESOLVED** | **R-4**: one read → `ComposeBaselineInclude` → `EnsureGitFallbackAuthor` → one write; first-run lifecycle test. `InsertBlockAfter` still errors if caller skips compose (correct). |
| **07-01 MEDIUM — URL-rewrites half of `WriteBaselineFile`** | **RESOLVED** | 07-01 Task 2: keep `RenderURLRewritesBlock` / rewrite `ReplaceBlock` / skip; only baseline half retargeted. |
| **07-01 MEDIUM — `filewriter` on never-writes allowlist** | **RESOLVED** | 07-01 Task 1: allowlist is `internal/deps` only; `filewriter` excluded. |
| **07-01 LOW — `-z` layout not a golden** | **RESOLVED** | Explicit `-z` golden + real-binary parse in isolation-contract tests. |
| **07-02 MEDIUM — TUI pair-snapshot vs CLI flags** | **RESOLVED** | 07-02 Task 3 guard comment naming 07-05; CLI explicit set/clear in 07-05 authority. |
| **07-02 MEDIUM — `global-git-author` collision** | **RESOLVED** | Reserved registry + orphans fixture; no "collision-proof by name" claim. |
| **07-02 LOW — T-07-12 guessed-name deferred** | **RESOLVED** | Sequential waves; T-07-12 says warning moves here if order changes. |
| **07-03 MEDIUM — delete `ScanConflicts` vs archive** | **RESOLVED** | **R-7**: `go build`/`vet`/tests in same commit; archive not in `go list`; do not edit archive. |
| **07-03 MEDIUM — `core.pager` dropped** | **RESOLVED** | **R-2** additive merge (07-01) + **R-6** full-selection pager-survival test (07-03). |
| **07-03 LOW — bundle "yours wins" unproven** | **RESOLVED** | 07-03 Task 2: post-apply `git config --show-origin` lifecycle test. |
| **07-03 LOW — origin path vs membership** | **RESOLVED** | Classifier pinned to origin path; `useConfigOnly` two-home test. |
| **07-04 MEDIUM — `failCommitAt` in compiled PTY** | **RESOLVED** | Unwritable second target (`~/.gitconfig.d` 0500); skip-as-skip if root; drop with rationale if unworkable. |
| **07-04 MEDIUM — shim vs real git writes** | **RESOLVED** | Harness proof: version override **and** `git config --file --list` delegation. |
| **07-04 LOW — shared ceremony viewport** | **RESOLVED** | Widen Git only; assert SSH preview byte-identical. |
| **07-05 MEDIUM — apply key identity** | **RESOLVED** | **R-5**: policy CLI tokens only; member keys refused by test. |
| **07-05 MEDIUM — incomplete invocation vs pre-checks** | **RESOLVED** | R-1 emptied TUI; `NewAppOnGlobalGit` asserts empty `chosen`. |
| **07-05 LOW — `--clear-email` needs readable file** | **RESOLVED** | Missing file = empty pair; unreadable existing file = refusal naming path. |
| **07-06 MEDIUM — dead allowlist rows** | **RESOLVED** | Re-measure before allowlisting; checkbox/zdiff3/12th-row may be equal. |
| **07-06 LOW — in-process vs PTY evidence** | **RESOLVED** | Manifest labels classes; PTY-only for DLV-04. |
| **07-06 LOW — SUMMARY parser drift** | **RESOLVED** | Binding `## Deviations` / `## Review`; missing heading is a source defect. |

No cycle-1 HIGH remains open.

---

### 07-01 — Tracer / floor include / ceremony

**Summary**
R-1/R-2/R-3 close the three ways a real POC baseline could be wrecked. Architecture still correctly mirrors Phase 6 on the last-wins floor (`gitconfig.recipe:66-67`).

**Strengths**
- Additive merge documented as mandatory because D-15 never re-selects already-set keys.
- `ComposeBaselineInclude` split preserves doctor/dispatcher skip (`baseline.go:624-626`) while the ceremony can still take a second backup.
- Policy-backed selectability is structural, not a wave-1 hack.

**Concerns**
- **LOW — `WriteBaselineFile` still uses sentinel `"baseline"` (`baseline.go:551`) until this plan lands.** Adoption tests must keep that literal in fixtures; plan already says so.
- **LOW — First-run "no backup" vs empty skip path** is distinguishable only if the receipt records empty-path-as-created. Plan says it; easy to drop in mapping.

**Suggestions**
- Pin a test that `runGlobalGitApply` does not call `WriteBaselineInclude` (source-level, like SSH write-verb construction tests).

**Risk**: **LOW–MEDIUM** (write path still costly; rules are now pinned).

---

### 07-02 — D9 two-field fallback

**Summary**
R-4 is the right SSH analog: same file, one compose stream, no two-write window. Empty-means-unset + seed-from-block reconciles D-04 and UI-SPEC.

**Strengths**
- Missing-anchor error stays on `InsertBlockAfter`; caller owns the floor include.
- Real-git proof that a missing include target is ignored (or compensating empty baseline file).
- TUI snapshot vs CLI flags documented so 07-05 cannot "fix" either.

**Concerns**
- **MEDIUM — `ComposeBaselineInclude` on first-run fallback writes a floor include pointing at a possibly absent baseline file.** Plan verifies git ignore-missing, but if git *warns*, creating an empty managed block is a second watched file — journal/restore tests must cover that branch, not only the happy one-write path.
- **LOW — Independent per-field apply is TUI-visible only as "apply both fields as shown."** Correct; keep 07-05 from adding a TUI clear-one-half without applying the other.

**Suggestions**
- If compensating baseline create is needed, add it to the "exactly ONE write to main config" assertion (two files, still one gitconfig write).

**Risk**: **LOW–MEDIUM** (first-run include-target).

---

### 07-03 — Twelve honest rows

**Summary**
GGIT-01 becomes true here. R-5/R-6/R-7 and D-10 tally (differs excluded because floor writes are no-ops) match git last-wins vs SSH Host-* overwrite.

**Strengths**
- Hard gate only on `merge.conflictstyle`; unreadable → `diff3`.
- Recipe-sourced aliases/colors; pager never a row.
- Origin-path attribution for two `[user]` sections.

**Concerns**
- **LOW — Line-endings CLI token `core.lineEndings` is not a git key.** Help text must list tokens; plan does.
- **LOW — `user.useConfigOnly` in included baseline vs later user key:** classifier test is the right pin.

**Risk**: **MEDIUM** on values/gates (inherent); plan coverage is **LOW** residual.

---

### 07-04 — Scroll, error, PTY

**Summary**
Click-at-offset, measured `frameBodyRows`, unwritable-dir rollback, shim delegation — cycle-1 holes are closed.

**Strengths**
- Cue both-edges → down; SSH advisory sentence reused.
- Banner drop last, after honesty.
- File-level PTY assertions, not frames-only.

**Concerns**
- **LOW — Dummy six-row list may never cue** — 07-06 non-applicability, not an allowlist row.
- **LOW — chmod-unwritable may fail on some FS**; drop-with-rationale is allowed.

**Risk**: **LOW–MEDIUM** (harness), design **LOW**.

---

### 07-05 — CLI + §J

**Summary**
Frozen four-path tree, tokens-only apply, explicit set/clear, §J D-11.1 without flipping GGIT-01 status.

**Strengths**
- Construction test: verbs call ceremonies only.
- `Selectable()` refusals; empty `--name` refused.
- Version substitution advisory from ceremony plan stage.

**Concerns**
- **LOW — Adaptive-depth `NewAppOnGlobalGit` must exist** (06-06 surprise already named).
- **LOW — Incomplete `options apply` with no keys opens TUI empty** — assert `chosen`, not the frame.

**Risk**: **LOW**.

---

### 07-06 — Visual gate + close

**Summary**
Correct closer: re-measure allowlist, four negative controls, packet from `## Deviations`/`## Review`, battery recorded, orchestrator-not-executor for packet review.

**Strengths**
- Probe-error live non-applicability.
- Multi-byte truncation hash; Makefile root + target-scan.
- R-1…R-7 indexed in closure table.

**Concerns**
- **LOW — Closing GGIT-01 depends on 07-04 frames existing.** Sequential waves handle this.
- **LOW — Parser fails closed on missing headings** — good.

**Risk**: **LOW**.

---

### Phase-level

| Goal | Covered? |
|------|----------|
| GGIT-01 explained options | 07-03 + 07-04 |
| Backup + block + confirm + verbatim outside | 07-01/07-02 + R-3 |
| DLV-04/06 PTY + dummy compare | 07-04 PTY, 07-06 gate |
| Recipe floor include | 07-01 D-01 |
| GITIGNORE not falsely closed | 07-05 D-11.1, status in 07-06 |
| Cycle-1 HIGHs | R-1, R-2/R-3, R-4 |

**Do not execute until:** none of the cycle-1 HIGHs remain; optional: first-run fallback compensating baseline-file branch if git errors on missing include.

**Overall phase risk: LOW–MEDIUM** — residual is write/adoption execution fidelity, not missing rules. Safe to execute 07-01.
