---
status: resolved
trigger: "Fix the P1 finding from the review of 98fa2ab: ValidateRegionDiffs accepts REGION-DIFFS documents that omit formerly non-required visible named regions."
created: 2026-08-22
updated: 2026-08-22
---

# Validate REGION-DIFFS Complete Inventory

## Symptoms

- Expected behavior: `ValidateRegionDiffs` rejects a stored document that omits any name returned by `AllRegionNames`, including a visible region outside `RequiredRegions`.
- Actual behavior: validation requires only `ScreenSpec.RequiredRegions`, so an optional visible region can be removed from an otherwise valid stored document.
- Error messages: none; the malformed document is accepted.
- Timeline: introduced by commit `98fa2ab` while expanding generation beyond required regions.
- Reproduction: build a valid region document, remove a non-required visible named region, marshal it, and call `ValidateRegionDiffs`.

## Current Focus

- hypothesis: Region generation stores a nonempty union and validation checks only required-region membership, leaving stored inventories incomplete and omission-tolerant.
- test: Add a regression test that removes a non-required visible region from generated evidence and expects `ValidateRegionDiffs` to reject it.
- expecting: The new test fails before implementation because the malformed document is accepted.
- next_action: None; fix and requested verification are complete.
- reasoning_checkpoint: The requested behavior requires complete `AllRegionNames` inventories, while `RequiredRegions` remains the separate mandatory-nonempty subset.
- tdd_checkpoint: GREEN confirmed; the regression and focused screenshot tests pass.

## Evidence

- timestamp: 2026-08-22
  observation: `BuildRegionDiffs` skips a region when both extracted sides are empty, and `ValidateRegionDiffs` checks membership only for `spec.RequiredRegions`.
  implication: Stored evidence is neither complete nor validator-enforced for all named regions.
- timestamp: 2026-08-22
  observation: `GOCACHE=/tmp/gitid-go-cache go test -tags screenshot ./internal/screenshot -run '^TestValidateRegionDiffsRejectsMissingVisibleNonRequiredRegion$' -count=1` fails with `ValidateRegionDiffs accepted a document missing a visible non-required named region`.
  implication: The regression test directly proves the reported omission vulnerability.
- timestamp: 2026-08-22
  observation: The exact regression test passes after the fix, and the focused region/packet selection passes 12 tests.
  implication: Complete inventory generation and omission rejection work without weakening disposition or packet validation.
- timestamp: 2026-08-22
  observation: `GOCACHE=/tmp/gitid-go-cache make gate-visual-regression` passes all five requested gate tests for 18 `RequiredScreenSpecs` frames.
  implication: The complete stored inventory remains compatible with the production visual-regression gate.

## Eliminated

- hypothesis: The issue is caused by unknown-region handling.
  reason: Unknown and duplicate stored regions are already rejected; omission is the uncovered case.

## Resolution

- root_cause: `BuildRegionDiffs` omitted names empty on both surfaces, while `ValidateRegionDiffs` enforced membership only for `RequiredRegions`.
- fix: Store every `AllRegionNames` entry and require every name during validation, retaining separate required-region nonempty and exact unequal-region disposition checks.
- verification: Exact regression PASS; 12 focused screenshot tests PASS; `make gate-visual-regression` PASS; `git diff --check` PASS.
- files_changed: `internal/screenshot/createflow_packet.go`, `internal/screenshot/createflow_packet_test.go`
