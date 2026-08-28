---
phase: 07-global-git-options
plan: 06
type: summary
wave: 6
---

# 07-06 Summary — visual-regression closure, cross-AI review packet, and GGIT-01 close

Every Global Git screen state is now pinned against the approved Bubble Tea
dummy with a gate proven to fail four distinct ways; the cross-AI review packet
is assembled with a mechanically-derived 38-row closure table; the phase's full
exit battery ran for real, all ten commands green; and GGIT-01 closes against
the roadmap's own three success criteria. This is the closing plan of Phase 7.

## Task 1 — Registered Global Git specs and their per-region dispositions

Seven `RequiredScreenSpecs` registered in `internal/screenshot/createflow.go`
(`globalGitSpecs()`), captured through the SAME in-process technique (model
Update/View, no PTY, no subprocess) the Global SSH capture uses:

| Spec | Comparable on both surfaces? | Region disposition |
|---|---|---|
| `ggit-options-list` | yes | `header-status` classified (`T-07-OPTIONS-HEADER`, DLV-4); `sidebar` classified (`T-07-OPTIONS-LIST-ROWS`, DLV-4); `ggit-options-browse` classified (`T-07-PROVENANCE`, DLV-4); `header` EQUAL (no disposition needed) |
| `ggit-options-scrolled` | yes | same three DLV-4 classifications, scroll-scoped variants (`T-07-OPTIONS-SCROLL-HEADER`, `T-07-OPTIONS-SCROLL-ROWS`, `T-07-PROVENANCE-SCROLL`) |
| `ggit-options-with-selection` | yes | same three DLV-4 classifications, selection-scoped variants (`T-07-OPTIONS-SELECTION-HEADER`, `T-07-OPTIONS-SELECTION-ROWS`, `T-07-PROVENANCE-SELECTION`) |
| `ggit-options-differs-row` | NOT in-process capturable (needs a second, differently-seeded sandbox HOME) | non-applicable on BOTH in-process surfaces (`GGIT-D-04`); real-binary evidence: `ui-frames/global-git-differs.txt` |
| `ggit-options-probe-error` | NOT capturable on the dummy (asymmetric capability) | live non-applicability (`GGIT-D-04`) — the dummy's fixture backend cannot fail a probe it never runs; real-binary evidence: `ui-frames/global-git-probe-failure.txt` |
| `ggit-apply-preview` | yes | `header-status` classified (`T-07-APPLY-HEADER`, DLV-4); `ggit-apply-ceremony` classified (`T-07-GLOBALBLOCK`, `GGIT-D-06`); `ggit-apply-heading` classified (`T-07-CEREMONYTARGET`, `GGIT-D-07`); `confirmation-preview` classified (shared ceremony hint, `GGIT-D-06`, same reasoning as the ceremony body) |
| `ggit-apply-receipt` | NOT capturable in-process (needs the real journal-backed write) | non-applicable on live + approved-tui; real-binary evidence: `ui-frames/global-git-apply-confirm.txt` |

Every spec explicitly records the approved Bubble Tea dummy as its sole
parity target and the approved HTML surface as non-applicable, asserted
per-spec by `TestGlobalGitHTMLNonApplicabilityPerSpec`.

## Known divergences — verified still-existing vs already-resolved

Per this plan's `<authority>` block, each divergence listed there was
re-measured against the amended fixtures before being written as an allowlist
entry or recorded as resolved:

- **The checkbox column (07-01's R-1 emptied default selection, 6/D-15)**:
  re-measured empirically (`go test -tags screenshot`, direct capture dump).
  The checkbox column is **EQUAL** on both surfaces — the dummy's own fixture
  was amended to start empty too (07-01's design-amendment record against its
  own Phase 2 mockup, not a real-versus-dummy divergence). **No allowlist
  entry needed.**
- **The scroll cue (07-04)**: re-measured at the real registry's fixed
  100×30 capture geometry. Both surfaces' twelve-row lists fit the 24–25-line
  body budget (12 rows × 2 lines = 24), so **NEITHER surface ever renders a
  scroll cue** at this geometry — the state is **EQUAL** (absent on both), not
  a divergence and not a non-applicability. Confirmed by direct capture
  comparison (`ggit-options-scrolled`'s `sidebar`/`ggit-options-browse`
  regions carry the same "no cue" shape modulo the DLV-4 fixture-vs-live
  content divergence already classified). The scroll MECHANICS remain
  provably correct at the unit level (07-04's inflated-stub-row-count tests)
  and at the real-PTY boundary (`TestGlobalGit_RealPTYScrollBothDirections`).
- **The fallback author pane's one-field-to-two-field amendment (07-02, D-04)**:
  this is a design amendment to the shared dummy fixture (both real and dummy
  render the two-field pane, no checkbox) — **not** a real-versus-dummy
  divergence. No allowlist entry; not classified as such by any registered
  spec (the fallback pane is display-only inside `ggit-options-list` and its
  siblings, covered by the umbrella `T-07-PROVENANCE`-class disposition since
  it renders live provenance either way).
- **The corrected conflict-style value (`zdiff3`) and the fail-loud
  `user.useConfigOnly` row (07-03, D-07/D-08)**: both were amended in the
  dummy fixture in the SAME commit that corrected the policy table
  (07-03-SUMMARY.md, "Task 3, opening corrections"). Re-measured: both sides
  now agree on `zdiff3` and both render the `user.useConfigOnly` row. **No
  divergence remains** for these two specifically; the surrounding provenance
  and now-value text still differs for the DLV-4 fixture-vs-live reason
  already classified.
- **The probe-error state (asymmetric capability)**: recorded as a live
  non-applicability pointing at `ui-frames/global-git-probe-failure.txt`
  (`TestGlobalGit_RealPTYProbeFailureStaysNavigable`'s evidence), consuming
  NO allowlist entry — the dummy's fixture backend structurally cannot fail a
  probe it never runs, which is a different thing from a classified
  divergence between two comparable renderings.

## Encoding-hazard test (T-07-36)

`TestNegativeControl_GlobalGitMidByteTruncationHashStable` deliberately
constructs BOTH hazards this surface carries — the master row's `→` arrow
(`"now: x → y"`) and plan 07-04's `↓`/`↑` scroll cue, each truncated to only the
first byte of the 3-byte rune, exactly what a pane-width truncation would leave
behind — and proves the `BuildRegionDiffs → BuildRegionDiffsJSON →
ValidateRegionDiffs` pipeline accepts each without error, with a STABLE content
hash across two independent captures of the same truncated content. Both
sub-cases (`row-arrow`, `scroll-cue`) PASS.

## Four negative controls and the exact failure each proved

1. **`TestNegativeControl_GlobalGitMissingState`** — deletes `ggit-options-list`
   from a real capture set and proves `screenshot.ValidateCapturedState` fails
   against the missing required state marker. Proves: **a registered state with
   no capture fails the gate.**
2. **`TestNegativeControl_GlobalGitUnclassifiedDifference`** — captures both
   real and dummy `ggit-options-list`, confirms `ggit-options-browse` genuinely
   differs between them, then constructs a spec with `RegionDispositions: nil`
   for that region and proves it carries zero authorizing dispositions for a
   difference that demonstrably exists. Proves: **a difference with no
   classification entry fails the gate.**
3. **`TestNegativeControl_GlobalGitPerturbedComparableRegion`** — confirms
   `RegionHeader` compares EQUAL between real and dummy (no disposition
   authorizes a difference there), then perturbs the real side's text and
   proves the perturbation breaks equality. Proves: **a region that should
   compare equal fails the gate when perturbed.**
4. **`TestNegativeControl_GlobalGitCrossSurfaceAllowlistLeakage`** — asserts no
   Global SSH allowlist entry references a Global Git screen ID and vice versa.
   Proves: **an allowlist entry from another surface is rejected against this
   one.**

All four PASS as tests (the gate fails as designed when their perturbation
runs), exactly as `make gate-visual-regression`'s full output below confirms.

## The closure table's coverage of R-1 … R-7

All seven cycle-1 resolutions are indexed as first-class closure-table rows in
`review-packet/MANIFEST.md` Part D.3, each with its discharging plan and pinning
test:

| Resolution | Discharged by | Pinning test |
|---|---|---|
| R-1 | 07-01 Task 3 (empty selection + `PolicyBacked`); CLI half by 07-05 | `TestGlobalGitCheckboxColumnIsUnchecked`, `TestGlobalGitNonPolicyRowIsNotSelectable`, `TestGitIncompleteBothTTYsOpensEmptyTUI` |
| R-2 | 07-01 Task 2 (`EnsureGlobalGit` additive merge) | Additive-merge + round-trip stability tests |
| R-3 | 07-01 Task 2 (`ComposeBaselineInclude`, unconditional backup) | Two-distinct-backup-paths acceptance criterion |
| R-4 | 07-02 Task 1/2 (anchor created in the same byte stream) | `TestRunGitFallbackAuthorApply_FirstRunCreatesAnchor`, `…VerifyNoPrecedenceAdvisory` |
| R-5 | 07-03 Task 1 (token frozen); 07-05 Task 1 (argv = tokens only) | `TestGlobalGitTokenSetMatchesAuthorityTable`; `TestGitOptionsApplyAcceptedTokensEqualPolicyTokens`, `…RefusesMemberKeysNamingOwningToken` |
| R-6 | 07-03 Task 1/3 (pager preserved, never a row) | Pager-survival test over a full twelve-row apply |
| R-7 | 07-03 Task 1 (compiler-verified deletion) | Same-commit `go build`/`go vet`/`go test`; post-delete `rg` |

Full detail (all 38 rows, including 07-04's cycle-1 findings and every
disclosed deviation) is in `review-packet/MANIFEST.md` Part D.

## Source-summary corrections (per the plan's authority contract)

Plans 07-01, 07-03, and 07-04 were each missing the required `## Review`
heading, and 07-01's/07-04's `## Deviations` sections used unkeyed numbered
prose instead of keyed bullets. Per this plan's `<authority>` block ("a
missing heading is a defect in that summary to be fixed there, not a case for
the parser to guess around"), all three were corrected AT THEIR SOURCE:
07-01 and 07-04 gained keyed `## Deviations` bullets (`D-07-01-N`,
`D-07-04-N`) and a `## Review` section indexing their own plan's cycle-1
resolutions; 07-03 gained keyed `## Deviations` bullets (`D-07-03-N`), a
`## Review` section, AND the complete implemented twelve-row policy table its
own `<output>` section already required but never received. The parser
(`review-packet/derive-closure-table.go`) fails closed on a missing heading or
an empty section; after the corrections it finds all ten headings across the
five summaries with zero failures.

## Closure table derivation (mechanically derived, re-runnable)

`review-packet/derive-closure-table.go` parses the `## Deviations` and
`## Review` headings from `07-01-SUMMARY.md` through `07-05-SUMMARY.md`,
counting only TOP-LEVEL list items (indented sub-bullets belong to their
parent item and are not double-counted). Run:

```
$ go run .planning/phases/07-global-git-options/review-packet/derive-closure-table.go \
    .planning/phases/07-global-git-options/
closure-table derivation (07-06 Task 2)
...
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

All ten required headings found; zero sections empty; the parser's exit code
was 0. Hand cross-check: 07-01 3+3=6, 07-02 4+6=10, 07-03 2+3=5, 07-04 5+4=9,
07-05 4+4=8; sum = **38**, matching the parser's own total.

## The review packet

`.planning/phases/07-global-git-options/review-packet/` contains:

- `frames/` — all 14 PTY frames plan 07-04 captured, copied byte-identical
  from `ui-frames/` (confirmed with `diff -r`).
- `visual-divergence-allowlist.txt` — copied byte-identical from
  `.planning/design/global-git/visual-divergence-allowlist.txt` (confirmed
  with `cmp`; also enforced by `TestGlobalGitAllowlistMatchesRegistry`).
- `derive-closure-table.go` — the derivation script above.
- `MANIFEST.md` — the full index: frames table (Part A), allowlist summary
  (Part B), source-summary inventory (Part C), the 38-row closure table with
  its derivation and cross-check plus the reviewer-relevant phase evidence
  (Part D: measured row budget, final policy table, D-06 precedence proof,
  below-gate apply envelope), and the reviewer checklist (Part E).

**Evidence classes, labeled and never merged**: the manifest explicitly
separates in-process render-capture parity evidence (this plan's Task 1
registrations, answering "does the real render match the approved design")
from real PTY frame evidence (plan 07-04's captures, answering "does the
compiled binary decode correctly through a real terminal" — DLV-04's actual
obligation). No screenshot-registry hash is ever offered as DLV-04 PTY
evidence, and every non-applicability record names the specific PTY frame file
that carries its existence proof.

## Exit-battery table (every command's ACTUAL output)

Run for real, in order, on this plan's own final commit, in the worktree:

| # | Command | Result |
|---|---|---|
| 1 | `go build ./...` | exit 0, no output |
| 2 | `go vet -tags screenshot ./...` | exit 0, no output |
| 3 | `TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./...` | `ok` — 19 test-bearing packages green (`cmd/gitid` 41.7s, `internal/tuikit` 19.1s, `internal/globalgit` 5.6s, etc.); `internal/screenshot` reports `[no test files]` (screenshot-tagged tests live under the `screenshot` build tag, excluded from the untagged run by design) |
| 4 | `GOLANGCI_LINT_CACHE="$PWD/.golangci-cache" make lint` (wraps plain + `--build-tags screenshot`) | `0 issues.` for both `golangci-lint run --build-tags screenshot ./internal/screenshot/...` and `golangci-lint run ./...`; `go vet -tags screenshot/smoke/e2e ./...` all clean; cache removed after |
| 5 | `make gate-copy-freeze` | every frozen string present (61 entries verified `ok`), all four dynamic exclusions confirmed correctly excluded |
| 6 | `make gate-no-backend-files` | prints its Phase-3 retirement notice and delegates to the durable guard (`TestNoBackendAllowlist`); exit 0 |
| 7 | `go test ./internal/dummytui/ -run TestNoBackendAllowlist` | `--- PASS: TestNoBackendAllowlist (0.27s)`, `ok` |
| 8 | `make gate-visual-regression` | `PASS` on every test — `TestGateVisualRegressionReadOnly`, `TestAllScreensCapturedAndNonEmpty`, all prior-surface negative controls, all 4 Global Git negative controls (`TestNegativeControl_GlobalGitMissingState`, `…UnclassifiedDifference`, `…PerturbedComparableRegion`, `…CrossSurfaceAllowlistLeakage`), `TestNegativeControl_GlobalGitMidByteTruncationHashStable` (both sub-cases), every `TestGlobalGit*` acceptance test, `TestGlobalGitFixturePolicyParity`, `TestGlobalGitOptionStatesWrapsProbeFailure`; `ok github.com/castocolina/gitid/cmd/gitid 40.8s`; `TestApprovalCommitRecorded` SKIPs for the documented pre-existing worktree `.git`-is-a-file reason (not a regression, confirmed unrelated to this plan) |
| 9 | `make test-e2e` (900s timeout, runs `make build` first) | `go build -o bin/gitid ./cmd/gitid` then `ok github.com/castocolina/gitid/e2e 596.808s` — well inside the 900s timeout, including all 12 Global Git PTY cases and the full existing e2e suite |
| 10 | `go test ./cmd/gitid/... -run TestParityMatrixResolvesAndCoversTree -v` | `--- PASS: TestParityMatrixResolvesAndCoversTree`, `ok` (0.36s) — explicitly confirmed on top of step 3's coverage |

**No skip beyond the one documented above** (`TestApprovalCommitRecorded`, a
pre-existing worktree-specific `.git`-is-a-file artifact unrelated to this
plan or this phase, identical in cause and disposition to 06-07's own recorded
skip). `internal/screenshot`'s own `TestCaptureTUI` was NOT run as part of this
battery (it is excluded from `make test`'s screenshot-tagged line by design —
the Makefile's own `lint-tagged`/`test` comments document this exclusion as an
environment-only gap, not a routine gate obligation) and is not counted as a
skip here because it was never in scope for any of the ten commands above.

**Frame-timestamp noise**: `make test-e2e` (step 9) rewrote 15 committed
`ui-frames/*.txt` files (10 in `06-global-ssh-options/`, 5 in
`07-global-git-options/`) with fresh backup timestamps and fresh temp-dir
names — pure noise, verified line-by-line with `git diff` before reverting.
All 15 were reverted with `git checkout --` prior to this plan's commit; no
timestamp noise is committed.

## Roadmap success criteria → named evidence

Phase 7's three roadmap success criteria (`.planning/ROADMAP.md` §"Phase 7"),
each mapped to concrete evidence:

1. **"A global-git-options screen manages `init.defaultBranch`... and recipe
   defaults... each explained." (GGIT-01)** → the twelve explained rows: plan
   07-03's SUMMARY carries the complete implemented row table (token, member
   keys, recommended value, gate, hard-gate fallback), pinned 12/12 against
   `internal/tuikit.GlobalGitOptions` by `TestGlobalGitFixturePolicyParity`;
   every row's one-line explanation is frozen copy enforced by
   `make gate-copy-freeze` (step 5 above, green).
2. **"Changes write through the backup + idempotent managed-block chokepoint
   with confirmation; content outside managed blocks is preserved verbatim."
   (GGIT-01)** → plan 07-01's ceremony tests (unconditional backup once
   authorized — R-3; additive merge preserving pre-existing keys — R-2; the
   plan→confirm→backup→write→verify lifecycle in `runGlobalGitApply`) and
   plan 07-02's own ceremony tests for the fallback-author's separate verb
   (`runGitFallbackAuthorApply`, one read → compose → one write, R-4's
   anchor-creation guarantee). Both ceremonies are exercised by the exit
   battery's step 3 and step 9 (unit + full e2e).
3. **"UI-wave gate: PTY e2e drives every screen in the real binary; automated
   review compares it with `cmd/gitid-dummy` and classifies every difference
   as an improvement or a defect." (DLV-04, DLV-06)** → plan 07-04's 14 PTY
   frames (`ui-frames/`, `e2e/global_git_pty_e2e_test.go`'s 12 raw-keystroke
   cases) satisfy DLV-06's e2e-per-screen obligation; THIS plan's Task 1
   registration of all seven Global Git specs in the visual-regression gate,
   with every comparable region classified `improvement`/`defect` or
   non-applicable, and four working negative controls proving the gate
   actually catches a missing state / unclassified difference / mutated
   comparable region / cross-surface leakage, satisfies DLV-04's automated
   classification obligation.

A criterion with no evidence means the requirement does not close; all three
have named, checkable evidence above, so GGIT-01's requirement-index status
was updated to Complete in `.planning/REQUIREMENTS.md` — AFTER, not before,
the battery above went green.

## Deviations

- **D-07-06-1 — Source-summary heading defects fixed at their source, not worked around.** Plans 07-01, 07-03, and 07-04 were each missing the required `## Review` heading (07-01-PLAN.md's `<output>` binds every plan in the phase to emit both `## Deviations` and `## Review`, spelled exactly, with keyed bullets). 07-01's and 07-04's `## Deviations` sections also used unkeyed numbered prose instead of keyed bullets, and 07-03's `<output>` required a complete implemented twelve-row policy table that its committed SUMMARY never received. Per this plan's own `<authority>` contract, all four defects were corrected in the SOURCE summaries (not worked around in the parser): `## Review` sections were added indexing each plan's own cycle-1 resolutions (R-1/R-2/R-3 for 07-01; R-5/R-6/R-7 for 07-03; the four cycle-1 findings named in 07-04-PLAN.md's `<authority>` for 07-04), Deviations bullets were re-keyed (`D-07-01-N`, `D-07-03-N`, `D-07-04-N`), and 07-03 gained its required policy table. The derivation parser (`review-packet/derive-closure-table.go`) fails closed on a missing heading or an empty section; after the corrections it finds all ten headings across the five summaries with zero failures.
- **D-07-06-2 — Frame-timestamp noise from `make test-e2e` reverted, not committed.** Running the full exit battery's step 9 rewrote 15 committed `ui-frames/*.txt` files (10 in Phase 6's directory, 5 in this phase's) with fresh backup timestamps and temp-dir names — every diff verified line-by-line as pure noise (no content, structural, or wording change) before reverting with `git checkout --`. This is the third recorded occurrence of this exact pattern in the phase (see 07-05-SUMMARY.md's D-07-05-4); no timestamp noise is committed by this plan.

## Review

- **The closure-table parser's contract (review cycle 1, LOW)**: the parser parses exactly the `## Deviations` and `## Review` headings and nothing else, and fails closed rather than guessing around a missing one. Verified directly: before D-07-06-1's source corrections, the parser reported 3 of 10 required headings missing and exited non-zero; after the corrections, all 10 are found and the total is 38, cross-checked by hand in this summary's "Closure table derivation" section.
- **Two evidence classes, never merged (review cycle 1, LOW)**: the manifest labels every piece of evidence by class — in-process render capture (this plan's Task 1 registrations) versus real PTY frame (plan 07-04's captures) — and every live/receipt non-applicability record names the specific PTY frame file that is its existence proof, never a screenshot-registry hash presented as DLV-04 PTY evidence. See `review-packet/MANIFEST.md`'s Contents section and Part A/B evidence-class labels.
- **07-06 (own plan) — "closing GGIT-01 depends on 07-04 frames existing" (cycle 2, LOW)**: sequential waves handle this by construction — 07-04 committed its 14 frames before this plan started, verified present via `diff -r` against the packet's `frames/` copy before this summary was written.
- **07-06 (own plan) — "dead allowlist rows" (cycle 2, MEDIUM)**: every known divergence in this plan's `<authority>` block was re-measured against the amended fixtures (see "Known divergences" section above) before writing or omitting an allowlist entry; the checkbox column and the corrected conflict-style/`useConfigOnly` fixtures were confirmed EQUAL (no entry), and the scroll cue was confirmed to never render on either surface at the real geometry (EQUAL, not a divergence and not a non-applicability) — none of the plan's candidate divergences was allowlisted without being independently re-verified first.

## What this plan does NOT do

**Running the cross-AI reviews of this packet has explicitly NOT happened.**
This executor has no mechanism to spawn reviewer subagents
(`agent-ui-ux-designer` / Codex / xai-grok). Per this plan's own action block
and the standing project rule in `.planning/STATE.md`, running those reviews
against `.planning/phases/07-global-git-options/review-packet/` is an
**orchestrator obligation**, owed at wave close, exactly as 06-07 stated for
its own packet. The packet's readiness (assembled, mechanically indexed,
byte-verified against its sources) is evidence the review CAN happen, not that
it did — nothing in this packet or this summary should be read as a completed
review.

## Task commits

1. **Task 1: Register every Global Git screen state in the visual-regression
   gate** — `294d4c6` (feat, prior session, already committed before this
   dispatch resumed)
2. **Task 2: Assemble the cross-AI review packet, run the full exit battery,
   close GGIT-01** — this commit (docs)

## Files created/modified (Task 2)

- `.planning/phases/07-global-git-options/review-packet/MANIFEST.md` — the
  packet's index and 38-row closure table
- `.planning/phases/07-global-git-options/review-packet/visual-divergence-allowlist.txt` —
  byte-identical copy of the classified allowlist
- `.planning/phases/07-global-git-options/review-packet/frames/` — 14 PTY
  frames, byte-identical copies
- `.planning/phases/07-global-git-options/review-packet/derive-closure-table.go` —
  the written, re-runnable closure-table derivation
- `.planning/phases/07-global-git-options/07-01-SUMMARY.md`,
  `07-02-SUMMARY.md`, `07-03-SUMMARY.md`, `07-04-SUMMARY.md` — corrected at
  source to carry the binding `## Deviations` / `## Review` heading contract
  (07-01/07-03/07-04 were missing `## Review`; 07-03 gained its required
  twelve-row policy table)
- `.planning/REQUIREMENTS.md` — GGIT-01's requirement-index status flipped to
  Complete, and its section-J checkbox to `[x]`, after the green battery

## Next phase readiness

**This is the closing plan of Phase 7 — there is no Wave 7.** All 6 waves are
now implemented, independently verified, and this dispatch's own commit closes
the phase. Per `ONESHOT.md`'s Per-Phase Checklist, items 6-9 (code review via
`gsd-code-reviewer`, goal-backward verify-work via `gsd-verifier`, UI review
via `gsd-ui-auditor` against real PTY output, and `/gsd-audit-uat`), plus the
cross-AI review of this packet, have not yet run for this phase and are its
natural next step before Phase 7 can be marked complete in `STATE.md`/
`ROADMAP.md`.

---
*Phase: 07-global-git-options*
*Completed: 2026-08-27*
