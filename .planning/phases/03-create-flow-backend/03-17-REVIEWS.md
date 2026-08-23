---
phase: 03-create-flow-backend
plan: 17
reviewed: 2026-08-23
reviewer: opencode-my-plan-review
cli: opencode
model: local-llm-env/my-plan-review
verdict: replan-required
findings:
  high: 3
  actionable: 8
source_grounding: grep
---

# Phase 3 Plan 17 Convergence Review

The sole configured reviewer returned `current_high=3` and
`current_actionable=8`. Its output did not cite source locations, so its
claims were source-grounded before replanning. This record preserves the
findings and dispositions used by the corrective revision.

## Findings And Dispositions

| ID | Reviewer finding | Disposition |
|----|------------------|-------------|
| HIGH-1 | Candidate canonical-manifest paths conflict with the final packet manifest. | Rejected as factually incorrect. `cmd/gitid-evidence/main.go:382-389` writes `CANDIDATE-MANIFEST.json`; `:1121-1138` writes the sibling `<candidate>.canonical-manifest.json`; finalization writes the distinct final packet manifest at `:146-152`. The plan will name and assert all three paths explicitly. |
| HIGH-2 | Task 3 relies on shell variables that do not survive the human checkpoint, and temporary candidate paths could disappear. | Accepted. The plan will create a durable source-bound handoff descriptor and candidate root, then derive and verify all finalizer inputs from it. |
| HIGH-3 | The plan does not explain how changed frames interact with copy-freeze and visual-regression gates. | Accepted with source correction. `Makefile:168-211` is a presence-only frozen-string gate and `:280-297` is read-only with no baseline regeneration. The plan will require an exact corrected-region classification and prohibit baseline or validator changes. |
| MEDIUM-1 | Expected RED output is not persisted. | Accepted. The plan will require per-defect RED logs, exit status, failing assertions, and commit ancestry evidence in the summary. |
| MEDIUM-2 | The compact includeIf expected string is vague. | Accepted. The plan will pin the dummy fixture condition `[includeIf "gitdir:~/personal/"]` from `internal/dummytui/data.go:168-170` and require a negative sentinel assertion. |
| MEDIUM-3 | New real/dummy differences are not explicitly classified. | Accepted. The plan will require a three-state classification table in the summary and reviewer handoff. |
| MEDIUM-4 | The review-directory contract is unspecified. | Accepted. The plan will name the existing finalizer's required files: `metadata.json`, `prompt.txt`, `raw-stdout.txt`, `raw-stderr.txt`, and `verdict.json` (`cmd/gitid-evidence/main.go:155-176`). |
| MEDIUM-5 | Three corrections lack explicit per-defect commit boundaries. | Accepted. The plan will require separate RED/GREEN pairs for each defect. |
| LOW-1 | Test existence uses a tag mismatch. | Accepted. The plan will prove focused-test names untagged in `internal/tuikit` and the PTY test with `-tags e2e` in `e2e`. |
| LOW-2 | Candidate nondeterminism has no constrained recovery. | Accepted. The plan will require a third capture to diagnose drift and permit fixture stabilization only, never normalizer/schema changes. |
| LOW-3 | Rejected candidate disposition is unspecified. | Accepted. The plan will retain rejected candidate/review material under its source-bound non-packet handoff root, never publish it. |

## Review Scope

The review covered the three corrective PTY UI defects: running stage-2 copy,
duplicated ReachableNotUploaded presentation, and the compact includeIf
preview. It also checked the binding TUI-only policy: compiled `cmd/gitid`
against live compiled `cmd/gitid-dummy` at 100x30 PTYs, every difference
classified, and no browser or substitute reviewer.

## Cycle Summary

CYCLE_SUMMARY: current_high=3 current_actionable=8

## Current HIGH Concerns

- HIGH-2: Persist the corrected source, candidate, manifest hash, evidence binary, and review directory contract across the blocking review checkpoint.
- HIGH-3: Define the read-only gate and classification response for the three intentional presentation corrections.
- HIGH-1: Clarify the separate candidate and final-packet manifest paths, despite the reviewer's incorrect claimed path conflict.

## Current Actionable Non-HIGH Concerns

- MEDIUM-1: Persist RED evidence and prove its commit ancestry.
- MEDIUM-2: Pin the exact dummy includeIf condition and reject the sentinel content row.
- MEDIUM-3: Classify every corrected real/dummy region delta before review.
- MEDIUM-4: Define the one-review directory files and validator-bound fields.
- MEDIUM-5: Require separate per-defect RED/GREEN commits.
- LOW-1: Match test-existence checks to their execution packages and tags.
- LOW-2: Constrain nondeterminism recovery to fixture stabilization with a third capture.
- LOW-3: Retain rejected material outside any final packet root.

## Cycle 2

The same configured reviewer rechecked the revised plan and reported
`CYCLE_SUMMARY: current_high=0 current_actionable=1`.

### Resolved

- HIGH-1: Candidate and final-packet manifest paths are explicitly separated.
- HIGH-2: A durable candidate and handoff descriptor replace checkpoint-scoped shell variables.
- HIGH-3: Read-only gate behavior and corrected-region classification are explicit.
- MEDIUM-1 through LOW-3: RED evidence, exact dummy condition, delta classification, review files, per-defect commits, matching test tags, bounded determinism recovery, and rejected-candidate retention are all covered.

### Current Actionable Finding

- MEDIUM-1: The Task 2 handoff prose named fields but did not pin the JSON keys
  or create and validate the configured-review directory that Task 3 reads.
  The next revision must write exactly `source_sha`, `evidence_bin`,
  `candidate_dir`, `candidate_manifest_sha256`, `canonical_manifest_sha256`, and
  `opencode_review_dir`, with the review directory under the source-bound
  handoff root rather than the final-packet root.

## Cycle 3

The same configured reviewer rechecked the final revision and reported
`CYCLE_SUMMARY: current_high=0 current_actionable=0`.

All Cycle 1 and Cycle 2 findings are resolved. The reviewer confirmed that the
plan has exact test-first coverage for the three PTY defects, a durable handoff
schema that matches Task 3's `jq` reads, a source-bound configured-review
directory outside the final-packet root, and the binding compiled-real versus
live-dummy TUI-only classification and no-defects policy.

## Convergence Result

Converged in 3 cycles with no unresolved HIGH concerns and no actionable
MEDIUM or LOW findings outside `03-17-PLAN.md`.

## Current HIGH Concerns

None.

## Current Actionable Non-HIGH Concerns

None.
