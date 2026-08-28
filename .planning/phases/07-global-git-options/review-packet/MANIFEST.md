# 07-06 Task 2 — Cross-AI Review Packet for Phase 7 (Global Git Options)

Assembled by the final plan's executor (07-06, Task 2) on `2026-08-27` for the
ORCHESTRATOR to consume at phase close, in the shape of Phase 6's own packet
(`.planning/phases/06-global-ssh-options/review-packet/MANIFEST.md`).

**The cross-AI reviews have NOT been run.** This executor has no
subagent-spawning tools and cannot invoke the reviewer agents. Per the plan
(07-06-PLAN.md Task 2 `<action>`: "Running the cross-AI reviews against the
assembled packet is an orchestrator obligation — an executor has no mechanism to
spawn reviewer subagents") and the standing project rule in `.planning/STATE.md`,
running `agent-ui-ux-designer` / Codex / xai-grok against this packet is owed by
the **orchestrator** at wave close. Nothing in this packet pretends a review
happened: the packet's readiness is the evidence the review CAN happen, not that
it did.

## Contents

- `frames/` — the 14 captured PTY frames from plan **07-04**, snapshotted from
  `.planning/phases/07-global-git-options/ui-frames/`. **Evidence class: real
  PTY frame** — raw terminal captures driven over a real PTY against the
  compiled `cmd/gitid` binary. These are the DLV-04 PTY obligation's existence
  proofs and the named evidence for the in-process gate's non-applicable states.
- `visual-divergence-allowlist.txt` — the classified divergence list from
  07-06 Task 1, snapshotted from
  `.planning/design/global-git/visual-divergence-allowlist.txt` (byte-identical
  to the file `TestGlobalGitAllowlistMatchesRegistry` enforces). **Evidence
  class: in-process render-capture parity** — the gate compares model Update/View
  captures of the real and dummy binaries; it answers render-parity-against-the-
  approved-design, a different question from the PTY frames' terminal-decoding
  correctness. A registry hash is never presented as a PTY frame, and vice versa.
- `derive-closure-table.go` — the written, re-runnable derivation behind the
  closure table's row count (Part D). Run with `go run
  derive-closure-table.go <phase-dir>`.
- `MANIFEST.md` — this file: frames index (Part A), allowlist summary (Part B),
  the deviations/review-findings source inventory (Part C), and the closure
  table with its mechanical derivation plus the reviewer-relevant phase evidence
  (Part D), and the reviewer checklist (Part E).

## Part A — Captured PTY frames (plan 07-04)

All 14 files are raw terminal captures driven over a real PTY against the
compiled `cmd/gitid` binary, per plan 07-04 Task 3 (`e2e/global_git_pty_e2e_test.go`).
Each is captured by a raw-keystroke case that also asserts on-disk outcomes, not
just rendered text. **Evidence class: real PTY.**

| Frame | Covers (real binary) | Produced by |
|---|---|---|
| `global-git-browse.txt` | Options list browse after booting the Global Git tab | `TestGlobalGit_RealPTYBrowse` |
| `global-git-scroll-down.txt` | The list scrolled (selection moved down); the twelve rows fit the fixed frame so no cue renders at the boundary | `TestGlobalGit_RealPTYScrollBothDirections` |
| `global-git-scroll-up.txt` | The list scrolled back up; same no-cue boundary property | `TestGlobalGit_RealPTYScrollBothDirections` |
| `global-git-empty-selection.txt` | Empty selection on screen entry (R-1: nothing pre-selected) | `TestGlobalGit_RealPTYEmptySelectionGuard` |
| `global-git-apply-cancel.txt` | Apply ceremony cancelled at the preview | `TestGlobalGit_RealPTYApplyCancel` |
| `global-git-apply-confirm.txt` | Apply ceremony CONFIRMED — **the receipt state**, named by `ggit-apply-receipt`'s non-applicability as the receipt evidence | `TestGlobalGit_RealPTYApplyConfirm` |
| `global-git-differs.txt` | A row in the `set, differs from recommendation` state — named by `ggit-options-differs-row`'s non-applicability | `TestGlobalGit_RealPTYDiffersRow` |
| `global-git-probe-failure.txt` | The probe-error state — named by `ggit-options-probe-error`'s live non-applicability (the asymmetric case the dummy structurally cannot produce) | `TestGlobalGit_RealPTYProbeFailureStaysNavigable` |
| `global-git-fallback-set.txt` | The D9 fallback-author ceremony's set outcome | `TestGlobalGit_RealPTYFallbackPairSet` |
| `global-git-fallback-clear.txt` | The D9 fallback-author ceremony's clear outcome | `TestGlobalGit_RealPTYFallbackPairClear` |
| `global-git-cross-warning.txt` | The D-07 cross-warning on the apply ceremony (fallback pair half-set + `user.useConfigOnly` selected) | `TestGlobalGit_RealPTYCrossWarning` |
| `global-git-version-gate.txt` | The below-gate conflict-style apply carrying the substitution advisory (07-05's plan-stage addition) | `TestGlobalGit_RealPTYBelowGateVersion` |
| `global-git-mid-transaction-failure.txt` | Mid-transaction write refusal (`"refusing non-regular transaction target"`) | `TestGlobalGit_RealPTYMidTransactionFailureAndRetry` |
| `global-git-mid-transaction-retry.txt` | The retry after the refusal, succeeding with correct on-disk content | `TestGlobalGit_RealPTYMidTransactionFailureAndRetry` |

Non-applicability pointers: `global-git-apply-confirm.txt` is the evidence the
in-process gate's `ggit-apply-receipt` spec names;
`global-git-differs.txt` is the evidence `ggit-options-differs-row` names;
`global-git-probe-failure.txt` is the evidence `ggit-options-probe-error`'s live
non-applicability names. `TestGlobalGitNonApplicabilityNamesExistingPTYFrame`
asserts each named file exists under the phase's `ui-frames/` directory.

## Part B — The classified divergence allowlist (07-06 Task 1)

`visual-divergence-allowlist.txt` records **9 entries**, every one classified
`improvement` (the real backend renders live, seeded-fixture facts; the dummy
renders its frozen Phase-2 fixture data). **Evidence class: in-process render
capture.** The six-field schema (`T-07-NAME screen-id:region:predicate:decision:classification:reason`)
mirrors Phase 6's; a `contains:`/`absent:` needle must actually match on the
real side while being absent on the dummy (or vice versa), so an entry cannot be
dead weight.

| Entry | Divergence | Classification |
|---|---|---|
| `T-07-OPTIONS-HEADER` | header status identity count (`0 ids` real vs `8 ids` dummy) | improvement |
| `T-07-OPTIONS-LIST-ROWS` | the Options-list rows (live probe vs frozen fixture `now:` values) | improvement |
| `T-07-PROVENANCE` | the Options body's live D-01/D-03 provenance labels and D-11/D-12 row states vs the dummy's frozen values (`absent:"not set ("`) | improvement |
| `T-07-OPTIONS-SCROLL-HEADER` / `T-07-OPTIONS-SCROLL-ROWS` / `T-07-PROVENANCE-SCROLL` | the same three classes on the scrolled window | improvement |
| `T-07-OPTIONS-SELECTION-HEADER` / `T-07-OPTIONS-SELECTION-ROWS` / `T-07-PROVENANCE-SELECTION` | the same three classes with a selection rendered | improvement |
| `T-07-GLOBALBLOCK` | the apply-ceremony managed-block diff (real `EnsureGlobalGit` composition vs the frozen fixture diff, shared `"Write global-git managed block to"` prefix) | improvement |
| `T-07-CEREMONYTARGET` | the apply-ceremony heading naming the RESOLVED baseline target (D-07, `GGIT-D-07`) | improvement |
| `T-07-APPLY-HEADER` | header status identity count on the apply-preview spec | improvement |

The approved-HTML surface is non-applicable on every Global Git spec, asserted
per-spec by `TestGlobalGitHTMLNonApplicabilityPerSpec` and citing `AGENTS.md`'s
standing Phase 3-10 UI-reference rule (the Bubble Tea dummy is the sole parity
target; HTML/MUI artifacts are Phase-2 design history).

## Part C — Deviations and review findings recorded by the five summaries

The closure table's source. Every 07-0N SUMMARY is required to emit two headings
spelled exactly — `## Deviations` and `## Review` — with keyed list items (the
binding heading convention in 07-01-PLAN.md's `<output>`, reproduced by every
plan in the phase). Plans 07-02 and 07-05 already carried both; plans 07-01,
07-03, and 07-04 were each missing `## Review` and were corrected AT THEIR SOURCE
in this plan, per 07-06-PLAN.md's `<authority>` contract ("a summary missing a
heading gets FIXED in that summary rather than worked around"). Two of them also
carried non-conforming Deviations bodies (07-01's unkeyed numbered items and
07-04's `### N.` headings); both were normalized to keyed bullets, and 07-03
gained the complete implemented row table its own `<output>` already required.
All ten headings now present; the derivation's fail-closed check passes.

The two headings are the ONLY sections the parser reads — the review log
(`07-REVIEWS.md`) is not a source, because it is overwritten to its latest cycle
while the per-plan sections accumulate.

## Part D — Closure table (38 rows, mechanically derived) and the phase evidence

### D.1 — The derivation (re-runnable, recorded)

Row count: **38** across **10 headings in 5 summaries** (07-01 … 07-05), each
heading present. The derivation is `derive-closure-table.go` in this directory;
it fails closed (non-zero exit) if any of the ten headings is missing or a
section yields zero list items, so an under-count can never be mistaken for a
clean derivation. Re-run it:

```
go run .planning/phases/07-global-git-options/review-packet/derive-closure-table.go \
    .planning/phases/07-global-git-options/
```

Observed output (this packet's row keys — the closure table below mirrors them
exactly):

```
== 07-01 ==
  ## Deviations (3 items):   D-07-01-1  D-07-01-2  D-07-01-3
  ## Review   (3 items):     R-1        R-2        R-3
== 07-02 ==
  ## Deviations (4 items):   D-07-02-1  D-07-02-2  D-07-02-3  D-07-02-4
  ## Review   (6 items):     R-4  D-04  D-05  D-06  T-07-10  Cycle-1
== 07-03 ==
  ## Deviations (2 items):   D-07-03-1  D-07-03-2
  ## Review   (3 items):     R-5        R-6        R-7
== 07-04 ==
  ## Deviations (5 items):   D-07-04-1  D-07-04-2  D-07-04-3  D-07-04-4  D-07-04-5
  ## Review   (4 items):     R-07-04-FAILCOMMIT  R-07-04-SHIM  R-07-04-VIEWPORT  R-07-04-CUE
== 07-05 ==
  ## Deviations (4 items):   D-07-05-1  D-07-05-2  D-07-05-3  D-07-05-4
  ## Review   (4 items):     R-5  R-1  D-11.1  D-04

TOTAL closure-table rows: 38 (10 headings across 5 summaries, all present)
```

**Cross-check (hand, independent of the parser):** 07-01 3+3=6, 07-02 4+6=10,
07-03 2+3=5, 07-04 5+4=9, 07-05 4+4=8; 6+10+5+9+8 = **38**. The table below has
exactly 38 FINDING rows, keyed D-07-0N-M / R-N / T-07-NN as printed above.

### D.2 — The closure table

For each finding: the plan that resolved it, the closing artifact, the test that
pins it, and the disposition. Everything recorded — recoveries, accepted
boundaries, disclosed gaps — is indexed, not just the fixes, so a partial
closure cannot hide.

**07-01 (6)**

| Key | Finding | Plan | Closing artifact | Pinned by | Disposition |
|---|---|---|---|---|---|
| D-07-01-1 | Cross-AI runtime crash after the `GlobalGitPlanner` seam; build left broken | 07-01 | Orchestrator implemented the three `realBackend` methods + `NoopGlobalGitPlanner` (`48c62dd`) | Full race suite + lint green before continuing | RECOVERED (crash-recovery) |
| D-07-01-2 | Second crash; uncommitted rewrite with 8 failing tests | 07-01 | Orchestrator fixed all 8: real regression (`s.SSHApplied` read), D9 ceremony two-step contract, `TestNoBackendAllowlist` violation, two test-authoring bugs, one truncation bug, three pre-existing mouse/footer tests; staticcheck QF1003 | Restored bool-sweep overlay + ceremony two-step; master-list-column-scoped assertions | RECOVERED (crash-recovery) |
| D-07-01-3 | The before/after checkbox byte-diff test was not added (needs a throwaway pre-rewrite build) | 07-01 | Disclosed; intent verified indirectly by three tests + 07-06's gate | `TestGlobalGitRendersAllElevenRows`, `TestGlobalGitMainVsMasterHighlight`, `TestGlobalGitCheckboxColumnIsUnchecked` | DISCLOSED (accepted with substitute evidence) |
| R-1 | cycle-1 HIGH — wave-1 apply of pre-checked fixture rows | 07-01 (CLI half: 07-05) | Selection empty on construct + every `activate()`; selectability requires `PolicyBacked` | `TestGlobalGitCheckboxColumnIsUnchecked`, `TestGlobalGitNonPolicyRowIsNotSelectable`, `TestGitIncompleteBothTTYsOpensEmptyTUI` (07-05) | FIXED |
| R-2 | cycle-1 HIGH — a one-key apply must not strip the rest of a real POC baseline | 07-01 | `EnsureGlobalGit` additive merge: existing keys preserved, selected keys overlaid, `core.excludesfile` the sole subtraction | Additive-merge + round-trip stability tests; pager half proven by 07-03's pager-survival test (R-6) | FIXED |
| R-3 | cycle-1 HIGH — idempotent skip vs required fresh backup | 07-01 | `ComposeBaselineInclude` extracted; ceremony writes via `filewriter.Write` with no byte-equality guard | Two-distinct-backup-paths AC (same selection applied twice) | FIXED |

**07-02 (10)**

| Key | Finding | Plan | Closing artifact | Pinned by | Disposition |
|---|---|---|---|---|---|
| D-07-02-1 | `includeIf gitdir:` matching only fires inside a repository | 07-02 | `findGitWorkTree` returns the dir if a work tree, else a direct child that is | `TestRunGitFallbackAuthorApply_VerifyNoPrecedenceAdvisory` | FIXED |
| D-07-02-2 | `GitFallbackAuthorPlanner` listed under Task 3 but Task 2 needs it | 07-02 | Interface, noop, and view DTOs landed in Task 2's commit (`a6c1d83`); Task 3 embedded + wired | No-embed reflection test (`wiring_test.go`) | ADDRESSED |
| D-07-02-3 | Verify-stage advisory on machines with no managed identity | 07-02 | `MatchedNotVerifiable` is the plan's required unverifiable outcome, not a pass | Verify-stage advisory sentence | ACCEPTED (as designed) |
| D-07-02-4 | First-run write pointing at an absent baseline file | 07-02 | git silently ignores a missing include target (real-git proof); ceremony writes exactly one file | Hermetic real-git read quoted in summary | ACCEPTED (verified) |
| R-4 | cycle-1 HIGH — missing floor-include anchor | 07-02 | Ceremony composes `ComposeBaselineInclude` into the same byte stream before `InsertBlockAfter`; anchor exists by construction | `TestRunGitFallbackAuthorApply_FirstRunCreatesAnchor` | FIXED |
| D-04 | Two-field fallback pane (one-field→two-field amendment) | 07-02 | Two independent fields, empty means unset, apply is a pair snapshot; TUI/CLI difference documented on the guard naming 07-05 | Pane/ceremony acceptance tests; before/after renderings in summary | ADDRESSED |
| D-05 | Own dedicated ceremony, never folded into the baseline block | 07-02 | `runGitFallbackAuthorApply` and `runGlobalGitApply` are distinct; neither calls the other | `TestRunGitFallbackAuthorApply_DistinctFromGlobalGitApply`, `TestGitFallbackCeremonyHeadingAndCommitAreDistinct`, `TestGitFallbackBaselineCeremonyNeverDispatchesFallbackCommit` | FIXED |
| D-06 | Post-write author-resolution precedence proof | 07-02 | Matched/unmatched `git config --show-origin --get user.email` quoted (identity fragment vs main config) | Hermetic real-git matched/unmatched read | FIXED |
| T-07-10/T-07-43/T-07-13/T-07-14 | Placement offset, first-run single write, reserved registration, unconditional backup once authorized | 07-02 | Block sits between floor include and first `includeIf` (byte offsets quoted); reserved names registered in the same commit as the owner; no-op short-circuit + unconditional authorized backup | Placement, first-run, reserved-survival, and backup tests | FIXED |
| Cycle-1 | TUI/CLI consistency flag on the pair-snapshot | 07-02 | Left open as a code comment, not a code change; both surfaces call `EnsureGitFallbackAuthor` with a resolved pair | Comment naming the constraint | KEPT OPEN (documented) |

**07-03 (5)**

| Key | Finding | Plan | Closing artifact | Pinned by | Disposition |
|---|---|---|---|---|---|
| D-07-03-1 | Agent stalled with zero progress; uncommitted salvageable work | 07-03 | Orchestrator killed it, verified and committed the corrections + `useConfigOnly` row as `1c910d5` | Build + tests clean on the committed tree | RECOVERED |
| D-07-03-2 | Scoped continuation crashed on a banned `/tmp` write, zero progress | 07-03 | Orchestrator answered the underlying question and hand-implemented the rest of Task 3 (parity test, new copy, `GateNotMet`, gate extension, no-colour test, baseline-untouched test) | Same-commit verification pass | RECOVERED (hand-finished) |
| R-5 | cycle-1 MEDIUM — `options apply` key identity | 07-03 (frozen) / 07-05 (argv) | CLI token is a THIRD identifier on the policy row; `options apply` accepts tokens and nothing else, member keys refused | `TestGlobalGitTokenSetMatchesAuthorityTable` (07-03); `TestGitOptionsApplyAcceptedTokensEqualPolicyTokens`, `TestGitOptionsApplyRefusesMemberKeysNamingOwningToken` (07-05) | FIXED |
| R-6 | cycle-1 MEDIUM — `core.pager` dropped | 07-03 | Pager preserved, never added, never a row; additive merge keeps an adopted block's line | Pager-survival test: `core.pager` survives a full twelve-row apply | FIXED |
| R-7 | cycle-1 MEDIUM — confirm `./...` after delete | 07-03 | `ScanConflicts`/`BaselineKeySet`/`Conflict` deleted; verified by compiler, not grep | Same-commit `go build`/`go vet`/`go test` pass + post-delete `rg` | FIXED |

**07-04 (9)**

| Key | Finding | Plan | Closing artifact | Pinned by | Disposition |
|---|---|---|---|---|---|
| D-07-04-1 | Cross-AI exited cleanly at 11/12 PTY cases | 07-04 | Orchestrator hand-finished the mid-transaction case from the self-authored handoff note | 12/12 PTY cases green | RECOVERED (clean exit, hand-finish) |
| D-07-04-2 | Three test-authoring bugs in the cross-AI's own PTY tests | 07-04 | Scroll assertion rewritten to the true no-cue boundary; differs assertion uses `"differs"`; probe-failure case opens without the success wait | Rewritten `TestGlobalGit_RealPTYScrollBothDirections`, `…DiffersRow`, `…ProbeFailureStaysNavigable` | FIXED |
| D-07-04-3 | The 12th case needed three iterations to find a working mechanism | 07-04 | `chmod 0500` does not work (`EnsureDir` restores 0700); directory-in-place-of-file rejected by `mutationJournal.watchFile`'s real stat check | `TestGlobalGit_RealPTYMidTransactionFailureAndRetry` | FIXED |
| D-07-04-4 | Pre-existing stale `TestDummyDemo_MouseAndGitApply` (Phase-3 era, pre-D-15/R-1) | 07-04 | Three assertions updated to the D-15/R-1 contract, committed separately (`d39806d`) | Fixed test passes in full `make test-e2e` | FIXED (unrelated pre-existing drift) |
| D-07-04-5 | Stale golangci-lint cache from a deleted sibling worktree | 07-04 | Worktree-local `GOLANGCI_LINT_CACHE="$PWD/.golangci-cache"` for every lint + commit | Clean lint with the override | WORKED AROUND (env) |
| R-07-04-FAILCOMMIT | cycle-1 MEDIUM — `failCommitAt` in compiled PTY | 07-04 | Real stat-based refusal: non-regular transaction target | `TestGlobalGit_RealPTYMidTransactionFailureAndRetry` | FIXED |
| R-07-04-SHIM | cycle-1 MEDIUM — shim vs real git writes | 07-04 | `FakeGitShimDir` delegates to real git, overrides `--version`, one failable subcommand | `TestFakeGitShimDelegatesOverridesVersionAndFailsSubcommand` (both halves) | FIXED |
| R-07-04-VIEWPORT | cycle-1 LOW — shared ceremony viewport | 07-04 | Global Git preview widened; Global SSH's stays at the default | `TestGlobalSSHCeremonyPreviewUsesUnchangedDefaultBudget` | FIXED |
| R-07-04-CUE | cycle-1 LOW — dummy six-row list may never cue | 07-06 (re-measured) | Re-measured at the real registry: both surfaces' twelve rows fit the 24–25-line budget, so NEITHER renders a cue — EQUAL, not a divergence and not a non-applicability | Real-vs-dummy scrolled-region comparison in `TestGateVisualRegression`; `TestGlobalGit_RealPTYScrollBothDirections` | RESOLVED (as equal) |

**07-05 (8)**

| Key | Finding | Plan | Closing artifact | Pinned by | Disposition |
|---|---|---|---|---|---|
| D-07-05-1 | Task 1 transient network error left three small bugs | 07-05 | Orchestrator fixed all three and committed `1bb740c`; tree re-verified | Build/vet/race clean after the fix | RECOVERED |
| D-07-05-2 | Task 3 "second apply still succeeds" contradicts Task 1's frozen already-set refusal | 07-05 | Task 1 is binding; the e2e case asserts the refusal AND byte-identical written file | `TestGlobalGitCLI_ListApplyIdempotent` | RESOLVED (plan contradiction, earlier contract wins) |
| D-07-05-3 | Schema doc example vs SUMMARY's envelope are two different real runs | 07-05 | Both are real captures showing the advisory channel; the e2e one is the shim-driven process-level proof | Verbatim envelopes in docs and summary | ADDRESSED (documented) |
| D-07-05-4 | Full `make test-e2e` rewrote committed PTY frames | 07-05 | Timestamp noise reverted; one real change (version-gate advisory) refreshed in `21aaa51` | Frames stable after a full run | RESOLVED |
| R-5 | cycle-1 MEDIUM — token-only argv (acceptance side) | 07-05 | `options apply` resolves argv against `Policy`'s frozen `Token` column only, exact and case-sensitive | `TestGitOptionsApplyAcceptedTokensEqualPolicyTokens`, `TestGitOptionsApplyRefusesMemberKeysNamingOwningToken` | FIXED |
| R-1 (CLI half) | cycle-1 HIGH — nothing pre-selected on the adaptive-depth fallback | 07-05 | Empty selection asserted via the model's own selection set, not the frame | `TestGitIncompleteBothTTYsOpensEmptyTUI` | FIXED |
| D-11.1 | §J substrate-note correction (obligation 1) | 07-05 | Note rewritten to name the owning phase per substrate requirement + D-11 pointer; GGIT-01 status deliberately left to 07-06 | Before/after wording quoted in summary | FIXED |
| D-04 | Explicit set-versus-clear on the CLI | 07-05 | `--name`/`--email` vs `--clear-name`/`--clear-email`, empty-value and set-plus-clear refusals | `TestGlobalGitCLI_FallbackShowSetClear` | FIXED |

### D.3 — The cycle-1 resolutions R-1 … R-7 as first-class rows

The cross-AI review cycle-1 findings, indexed by their resolution identifiers
(R-1 … R-7 as recorded in the 07-01, 07-02, 07-03, and 07-05 summaries/plans),
each with the plan that discharged it and the test that pins it. A review whose
findings were addressed but never indexed reads, to the next cycle, exactly like
a review whose findings were ignored.

| Resolution | Finding | Discharged by | Pinning test(s) |
|---|---|---|---|
| R-1 | HIGH — wave-1 apply of pre-checked fixture rows (selection must start empty; only policy-backed rows selectable) | 07-01 Task 3 (empty `chosen` + `PolicyBacked` selectability); CLI half by 07-05 Task 1/3 | `TestGlobalGitCheckboxColumnIsUnchecked`, `TestGlobalGitNonPolicyRowIsNotSelectable`, `TestGitIncompleteBothTTYsOpensEmptyTUI` |
| R-2 | HIGH — a one-key apply must not strip the rest of a real POC baseline (additive merge) | 07-01 Task 2 (`EnsureGlobalGit`) | Additive-merge + round-trip stability tests; pager half by 07-03's pager-survival test |
| R-3 | HIGH — "idempotent skip vs required fresh backup" (backup unconditional once authorized) | 07-01 Task 2 (`ComposeBaselineInclude` split, no equality guard in the ceremony) | Two-distinct-backup-paths acceptance criterion |
| R-4 | HIGH — missing floor-include anchor on first-run fallback write | 07-02 Task 1/2 (`InsertBlockAfter` + `ComposeBaselineInclude` in one stream) | `TestRunGitFallbackAuthorApply_FirstRunCreatesAnchor`, `TestRunGitFallbackAuthorApply_VerifyNoPrecedenceAdvisory` |
| R-5 | MEDIUM — `options apply` key identity (frozen CLI token, member keys refused) | 07-03 Task 1 (token frozen on the policy row); 07-05 Task 1 (argv = tokens only) | `TestGlobalGitTokenSetMatchesAuthorityTable`; `TestGitOptionsApplyAcceptedTokensEqualPolicyTokens`, `TestGitOptionsApplyRefusesMemberKeysNamingOwningToken` |
| R-6 | MEDIUM — `core.pager` dropped by the merge | 07-03 Task 1/3 (pager preserved, never a row) | Pager-survival test: `core.pager` survives a full twelve-row apply |
| R-7 | MEDIUM — confirm `./...` after the `ScanConflicts` deletion | 07-03 Task 1 (compiler-verified deletion in the same commit) | Same-commit `go build`/`go vet`/`go test` pass + post-delete `rg` |

### D.4 — Reviewer-relevant phase evidence (the surface, not just the index)

A reviewer needs these to judge the surface. **The measured row budget and
visible row count (07-04):** at the canonical fixed frame (100×30,
`frameBodyRows(minFrameHeight)`), the body budget is **25 rows without the
findings banner and 24 with it**; the twelve-row list renders **24 lines** (12
rows × 2 lines), so it fits in both — **the twelve-row list never scrolls on
either surface** at the real geometry, and no cue renders at either boundary
(measured facts recorded in 07-04-SUMMARY.md; the dummy's own fixture is also
twelve rows, so the same holds there). The scroll mechanics are proven at unit
level against an inflated stub row count.

**The final twelve-row table (07-03):** the complete implemented row table with
CLI token, member keys, recommended values, gates, and the hard-gate fallback —
now carried verbatim in 07-03-SUMMARY.md's "The complete twelve-row policy table
as implemented" (transcribed from `internal/globalgit.Policy`, pinned 12/12 by
`TestGlobalGitFixturePolicyParity`). The one hard gate is `merge.conflictstyle`
(writes `diff3` below git 2.35, `zdiff3` at/above); all other gates are
informational.

**The observed author-resolution outputs (07-02) proving the D-06 precedence
invariant** — quoted verbatim in 07-02-SUMMARY.md:

```
MATCHED (cwd = <home>/git/work/repo)
  stdout: 'file:<home>/.gitconfig.d/work\twork@example.com\n'
  rc: 0

UNMATCHED (cwd = <home>/unmatched)
  stdout: 'file:<home>/.gitconfig\tfallback@example.com\n'
  rc: 0
```

The matched read names the identity fragment; the unmatched read names the main
config — floor placement plus git's last-wins rule means an identity's `includeIf`
fragment wins, and the fallback only applies where no identity matches.

**The captured below-gate apply envelope (07-05)** — quoted verbatim in
07-05-SUMMARY.md, from `TestGlobalGitCLI_BelowGateAdvisoryExitCodes` against a
fake git at 2.34.1 (below the 2.35 hard gate): `gitid.git.apply/v1` with
`applied: ["merge.conflictstyle"]`, the substitution advisory (`wrote "diff3"
instead of "zdiff3"`), the effective-value advisory, and `exit_code: 0`; with
`--fail-on-advisory` the same apply exits 3. The file-level assertion backs it:
the baseline file contains `conflictstyle = diff3`, not `zdiff3`.

## Part E — Reviewer checklist

1. Run `make gate-visual-regression` — expect the full merged registry (the
   34-spec pre-Global-Git set + 7 Global SSH + 7 Global Git = 48
   `RequiredScreenSpecs`) compared as a classified real/dummy symmetric union;
   all four Global Git negative controls pass
   (`TestNegativeControl_GlobalGitMissingState`,
   `TestNegativeControl_GlobalGitUnclassifiedDifference`,
   `TestNegativeControl_GlobalGitPerturbedComparableRegion`,
   `TestNegativeControl_GlobalGitCrossSurfaceAllowlistLeakage`) plus the
   mid-byte-truncation hash-stability test
   (`TestNegativeControl_GlobalGitMidByteTruncationHashStable`).
   `TestApprovalCommitRecorded` may SKIP (worktree `.git` is a file, not a
   directory — an environment artifact, not a gate failure); the probe-error
   and receipt Global Git specs are deliberately registered non-applicable on
   both in-process surfaces and consume no allowlist entry.
2. Compare the real binary against the approved dummy using `frames/` +
   `visual-divergence-allowlist.txt` (the gate's machine-readable source of
   truth). Every entry states `improvement`/`defect`; the DLV-4 fixture-vs-live
   class and the `GGIT-D-06`/`GGIT-D-07` ceremony entries are present by name.
   Evidence-class rule: a PTY frame proves terminal-decoding existence (DLV-04);
   a registry hash proves in-process render parity; never offer one as the other.
3. Check the closure table (Part D) against the per-plan `## Deviations` /
   `## Review` sections — 38 rows derived by `derive-closure-table.go`, count
   cross-checked by hand above.
4. Record your findings and disposition in the phase's review section, as Phase
   6's wave-close pass did.
