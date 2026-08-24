---
phase: 03-create-flow-backend
reviewed: 2026-08-22T05:07:36Z
depth: deep
files_reviewed: 9
files_reviewed_list:
  - cmd/gitid-evidence/main.go
  - cmd/gitid-evidence/main_test.go
  - internal/screenshot/createflow_packet.go
  - internal/screenshot/createflow_packet_test.go
  - internal/tuikit/ceremony.go
  - internal/tuikit/frame.go
  - internal/tuikit/frame_test.go
  - internal/tuikit/identities.go
  - internal/tuikit/identities_test.go
findings:
  critical: 6
  warning: 2
  info: 0
  total: 8
status: issues_found
---

# Phase 03-14: Code Review Report

**Reviewed:** 2026-08-22T05:07:36Z
**Depth:** deep
**Files Reviewed:** 9
**Status:** issues_found

## Summary

The implementation from `aed2c3e` and `46f0f30` remains materially incomplete against 03-14. Candidate generation uses disposable HOME directories and a PATH-shim fake SSH, and this review found no path that confirms a candidate write against the user's real SSH/Git configuration. However, the implementation does not provide finalization, retains the legacy eight-screen inventory, does not drive the production viewport during evidence capture, and does not validate review provenance or region evidence fail-closed. Two viewport input/rendering defects also break advertised behavior.

Verification run: `TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./internal/tuikit/...` passed (218 tests). The screenshot-tagged suite ran 50 passing tests and one environment-gated failure because `freeze` was unavailable.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01 [BLOCKER]: No review-gated finalization or publication path exists

**File:** `cmd/gitid-evidence/main.go:39-56,94-151`

**Issue:** `run` rejects every non-candidate invocation and exposes no `finalize` mode. The old `publish` function is now unreachable, and it is internally incompatible with the new candidate format: `runCandidateProcess` creates `CANDIDATE-MANIFEST.json`, while `publish` immediately calls `ValidatePacket`, which requires `MANIFEST.json`. Therefore no command can consume two reviews, create `REVIEW-PROVENANCE.json`, build a final manifest, or atomically publish the source-SHA destination required by 03-14 Task 3.

**Fix:** Implement an explicit `finalize` subcommand/mode that accepts a candidate directory plus two review records, validates all review and candidate bindings, builds a fresh staging directory with the final manifest/provenance inventory, validates it, and atomically renames it to the new source-SHA destination. Remove or rewrite the dead `publish` path so candidate and final manifest names are handled consistently.

### CR-02 [BLOCKER]: Candidate generation still uses the legacy eight-screen/24-panel inventory

**File:** `internal/screenshot/createflow_packet.go:130-147,195-200`

**Issue:** Completeness remains hard-coded to 8 IDs × 3 surfaces (`ValidatePanelCount = 24`) and `CreateFlowScreenIDs`. The required completed PASS, ReachableNotUploaded, hard-failure/retry, and navigated proof/confirmation frame variants are not represented as independent required members. `captureTUIPanels` likewise iterates only that legacy ID list (`cmd/gitid-evidence/main.go:407-409`). A candidate can therefore validate while omitting the semantic states and frame variants mandated by A-02/A-03/A-04 and Task 2.

**Fix:** Derive required members from the typed `ScreenSpec`/`FrameSpec` registry, including every required semantic state and viewport frame. Validate surface applicability and require PNG, rendered text, and raw ANSI/transcript members for each applicable frame instead of enforcing a fixed count of 24.

### CR-03 [BLOCKER]: Evidence capture never drives the raw-PTY proof or confirmation viewport

**File:** `cmd/gitid-evidence/main.go:532-585`

**Issue:** Stage captures call `runStage1Only`/`runStages` and then snapshot immediately. Confirmation reaches the ceremony and also snapshots immediately. No capture sends `Tab`/`v`, `PgDn`, `PgUp`, Left, or Right. At completion, `proofText` places multiline stage-2 resolution output before stage 1, while the viewport renders only eight rows (`internal/tuikit/identities.go:1078-1125`); the initial frame cannot expose all five `ssh -G` fields, the stage-1 proof, or horizontally hidden command suffixes. Thus the packet cannot contain the required raw-PTY evidence that advertised controls reveal every byte at 100×30.

**Fix:** Add registry-declared frame interactions that focus the real production viewport and send raw PTY navigation bytes. Capture enough vertical and horizontal frames to cover the complete command/output/resolution transcript and complete confirmation block, asserting unique byte markers before each snapshot. Add the plan-required real-binary e2e tests rather than only model-level key routing tests.

### CR-04 [BLOCKER]: Final provenance validation checks filenames only and can accept fabricated reviews

**File:** `internal/screenshot/createflow_packet.go:601-624`

**Issue:** `ValidateProvenanceRecords` only checks that three paths appear in `pkt.Members`. It never parses `REVIEW-PROVENANCE.json`, prompts, raw stdout/stderr, or verdicts; never checks `candidate_manifest_sha256`/`source_commit`; never requires distinct reviewer/tool/provider identities; and never rejects nonzero exits or open Critical/High findings. Any arbitrary bytes named `REVIEW-PROVENANCE.json` satisfy this gate.

**Fix:** Define and strictly decode the provenance schema, reject unknown/missing fields, hash and verify every declared prompt/raw stream/verdict member, bind both reviews to the exact source and candidate-manifest hash, require distinct identities, successful exits, and zero open Critical/High findings.

### CR-05 [BLOCKER]: Region evidence silently accepts missing regions and unapproved divergences

**File:** `internal/screenshot/createflow_packet.go:754-823`

**Issue:** `BuildRegionDiffs` skips a region whenever either side is empty and never returns an error if a required frame ends with zero regions. Unknown whole-screen differences are labeled `unknown`, and unexpected region differences receive `No accepted divergence is declared`, but neither condition is rejected. `ValidateCandidate` only hashes `REGION-DIFFS.json`; it never parses these records. Candidates can therefore pass with empty required evidence or unexplained differences, contrary to A-06 and Task 3's fail-closed contract.

**Fix:** Make region construction/validation return errors. Use each frame's declared required regions, require nonempty normalized live/approved content where applicable, and reject every unequal region without an allowlisted D-XX justification. Parse and validate `REGION-DIFFS.json` during candidate and final validation.

### CR-06 [BLOCKER]: The viewport focus key makes some destructive confirmations impossible

**File:** `internal/tuikit/ceremony.go:176-179,229-232`

**Issue:** `handleKey` consumes every `v` to toggle viewport focus before the destructive confirmation input receives text. An identity whose exact confirm word contains `v` (for example, `dev`) can never be typed, so delete/fix confirmation remains permanently disabled. This regression affects every ceremony because the new viewport was added to the shared component.

**Fix:** Do not reserve a printable character while a destructive text input owns focus. Use a non-text chord, require an explicit preview-focus control in the ceremony focus ring, or route `v` to `typed.Update` until the input loses focus. Add a destructive ceremony test whose confirm word contains `v`.

## Warnings

### WR-01 [WARNING]: Horizontal-only cues exceed `VisibleLines` and destabilize viewport geometry

**File:** `internal/tuikit/frame.go:786-812`

**Issue:** `View` reserves a content row for the cue only when `hiddenBelow > 0`. With horizontally hidden content but no vertical overflow, it emits all `VisibleLines` content rows and then appends the cue, producing `VisibleLines+1` rows. This violates the type's documented exact-row contract and can push controls or proof content out of the fixed 100×30 body.

**Fix:** Reserve one row whenever `cueLine != ""`, or render the horizontal cue within a fixed separate budget controlled by the caller. Add a test asserting exact line count for horizontal-only overflow.

### WR-02 [WARNING]: The production proof viewport advertises `v focus`, but `v` is not routed

**File:** `internal/tuikit/identities.go:1121-1125,2004-2042`

**Issue:** The proof pane tells users to press `v` to focus, but its handler toggles focus only on `Tab`; `v` falls through and is swallowed. This violates the plan's requirement that only actually routed controls be advertised and makes the raw-PTY interaction contract misleading.

**Fix:** Either route `v` to `w.proof.Focused` or change the displayed hint to `Tab focus`. Keep the body hint, footer, tests, and capture driver on one control contract.

---

_Reviewed: 2026-08-22T05:07:36Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: deep_
