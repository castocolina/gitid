---
phase: 03-create-flow-backend
verified: 2026-08-24T06:01:23Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 4/5
  gaps_closed:
    - "User runs the two-stage connectivity test with exact commands and real output, including ssh -G proof of the resolved IdentityFile, entirely against throwaway files until confirmation."
  gaps_remaining: []
  regressions: []
---

# Phase 3: Create Flow Backend Verification Report

**Phase Goal:** A developer creates an identity end-to-end — pick an algorithm, fill the SSH screen, test it against throwaway configs with the exact commands shown, and store it — with the live TUI matching the approved design.
**Verified:** 2026-08-24T06:01:23Z
**Status:** passed
**Re-verification:** Yes — after 03-18 gap closure

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|---|---|---|
| 1 | User can select an algorithm, complete the four-field SSH form by mouse/keyboard, and see the live recipe-shaped Host preview. | ✓ VERIFIED | Existing real-PTY coverage remains green in `make test-e2e`; `HostBlockPreview` uses the checked renderer. |
| 2 | User can reuse an existing key and confirmed persistence emits guarded macOS globals. | ✓ VERIFIED | Prior focused reuse/globals coverage remains in the passing suite; no 03-18 change touched this path. |
| 3 | Two-stage testing shows exact commands and real outputs, including an `ssh -G` IdentityFile proof, against throwaway files before confirmation. | ✓ VERIFIED | `ResolvedVia` retains the one process's `ssh -G` stdout in `Result.ResolutionOutput`; `TestStage2` directly projects it to `TestResultView`; `proofText` supplies it to the focused `ExactTextViewport`. Focused backend/view and compiled-PTY tests passed. |
| 4 | A confirmed passing create persists to the selected SSH target with timestamped backup and transactional rollback. | ✓ VERIFIED | No 03-18 change touched persistence; `make test` passed, including the existing transaction/rollback coverage. |
| 5 | Real compiled-TUI PTY coverage exists and real-vs-dummy differences are classified with no unresolved defects. | ✓ VERIFIED | `make test-e2e` passed. The raw-proof viewport delta is a documented TEST-01/02 improvement: real output replaces the dummy's compact fixture summary; it is not allowlisted away (`03-18-SUMMARY.md:56-67`). |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Former Gap: Raw Stage-Two Proof

The prior failure is closed by source and executable evidence, not by the 03-18 summary claim:

1. **Retained:** `internal/tester/tester.go:205-207` assigns bytes from the sole staged `ssh -F <config> -G <alias>` execution directly to `res.ResolutionOutput`, then parses that same string. `Resolved` follows the same rule (`lines 176-178`).
2. **Directly wired:** `cmd/gitid/wiring.go:614-622` receives that result, validates the parsed fields fail-closed, and assigns `view.ResolutionOutput = res.ResolutionOutput`. The former `formatResolvedConfig` reconstruction was removed.
3. **Rendered in the focused viewport:** `internal/tuikit/identities.go:1114-1145` appends `ResolutionOutput` unchanged to the labeled proof text and assigns it to `ExactTextViewport.Text`; `renderProof` exposes that viewport through `v` and PgDn/PgUp.
4. **Protected against substitution:** tests inject unparsed marker, duplicate `identityfile`, spacing, and trailing newline data. `TestStage2RetainsConnectivityAndRawResolutionOutput` requires byte equality at the composition boundary; `TestFocusedProofContainsRawStage2Outputs` requires contiguous unchanged proof-text containment; compiled-real-PTY `TestCreateFlow_Stage2RendersExactRawSSHOutput` uses fake staged SSH output containing `gitidrawmarker proof-retained-verbatim` and observes it only after focusing/paging the proof viewport. A parsed-field reconstruction cannot create that marker.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/tester/tester.go` | Raw stage-two proof plus parsed validation facts | ✓ VERIFIED | L1 exists; L2 executes and preserves raw `ssh -G` stdout; L3 is consumed by `realBackend.TestStage2`. |
| `cmd/gitid/wiring.go` | Direct raw-proof projection and fail-closed validation | ✓ VERIFIED | `ResolutionOutput` is assigned directly before `ValidateResolvedConfig`; parsed validation cannot unlock store on mismatch. |
| `internal/tuikit/identities.go` | Focused proof viewport source | ✓ VERIFIED | `proofText` retains labeled contiguous outputs and `refreshProof` wires them to `ExactTextViewport`. |
| `e2e/create_flow_pty_e2e_test.go` | Real compiled-TUI anti-substitution proof | ✓ VERIFIED | Drives the real binary, focuses with `v`, pages with raw PgDn, and requires the raw-only marker. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| staged `ssh -G` process | `tester.Result.ResolutionOutput` | direct `string(gOut)` assignment | ✓ WIRED | No formatter or parsed-field reconstruction remains. |
| `tester.Result` | `tuikit.TestResultView` | `realBackend.TestStage2` direct assignment | ✓ WIRED | `wiring.go:621-622`. |
| `TestResultView.ResolutionOutput` | focused proof viewport | `proofText` → `refreshProof` → `ExactTextViewport` | ✓ WIRED | Byte-containment and PTY focus/scroll tests pass. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|---|---|---|---|---|
| `tester.go` | `ResolutionOutput` | stdout of staged `ssh -F <config> -G <alias>` | Yes | ✓ FLOWING |
| `wiring.go` | `view.ResolutionOutput` | `res.ResolutionOutput` | Yes | ✓ FLOWING |
| `identities.go` | `ExactTextViewport.Text` | labeled concatenation containing raw `ResolutionOutput` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Raw-only stage-two marker reaches the focused real-TUI proof viewport | `TERM=dumb SSH_AUTH_SOCK= rtk go test -v -tags e2e -race -count=1 ./e2e -run '^TestCreateFlow_Stage2RendersExactRawSSHOutput$'` | 1 passed | ✓ PASS |
| Raw retention, composition separation, viewport containment, and fail-closed validation | `TERM=dumb SSH_AUTH_SOCK= rtk go test -v -race -count=1 ./internal/tester ./cmd/gitid ./internal/tuikit -run 'Test(ResolvedViaRetainsRawResolutionOutput|ResolvedRetainsRawResolutionOutput|Stage2RetainsConnectivityAndRawResolutionOutput|FocusedProofContainsRawStage2Outputs|Stage2RecordsOutcomeAfterValidation)$'` | 5 passed | ✓ PASS |
| Workspace regression gates | `TERM=dumb SSH_AUTH_SOCK= rtk make test && rtk make lint && TERM=dumb SSH_AUTH_SOCK= rtk make test-e2e` | test, lint, and e2e passed | ✓ PASS |

### Requirements Coverage

| Requirement | Status | Evidence |
|---|---|---|
| SSHUI-01, SSHUI-02, SSHUI-03 | ✓ SATISFIED | Existing form/preview PTY coverage remains green. |
| KEY-06, SSHUI-05 | ✓ SATISFIED | Existing reuse/globals coverage remains green. |
| SSHUI-04, TEST-01, TEST-02 | ✓ SATISFIED | Staged config path and actual raw `ssh -G` stdout are retained, validated, and rendered through the focused viewport. |
| TEST-03 | ✓ SATISFIED | Passing workspace transaction/rollback coverage; no persistence-path change in 03-18. |
| DLV-04, DLV-06 | ✓ SATISFIED | Real compiled PTY suite passes; the raw-proof delta is explicitly classified as a required improvement rather than suppressed. |

### Test Quality Audit

| Test File | Linked Requirement | Active | Skipped | Circular | Assertion Level | Verdict |
|---|---|---:|---:|---|---|---|
| `internal/tester/tester_test.go` | TEST-02 | 2 focused | 0 | No | Value | ✓ Valid raw-byte assertions |
| `cmd/gitid/wiring_cr_test.go` | TEST-01/02 | 2 focused | 0 | No | Value/behavioral | ✓ Valid direct-projection assertion |
| `internal/tuikit/identities_test.go` | TEST-01/02 | 1 focused | 0 | No | Value | ✓ Valid contiguous-proof assertion |
| `e2e/create_flow_pty_e2e_test.go` | DLV-06, TEST-01/02 | 1 focused | 0 | No | Behavioral | ✓ Compiled-real PTY proof |

### Anti-Patterns Found

No blocker-level debt markers or output-reconstruction code were found in the 03-18 proof transport. `git diff --check` passed. The `return nil` matches in inspected Go files are normal error/control-flow returns, not empty render or proof stubs.

### Decision Coverage

`check.decision-coverage-verify` reported a non-blocking parser warning because several multiline D-03/D-08/D-16/D-19 bullets in `03-CONTEXT.md` are not parseable by that heuristic. This does not contradict the verified raw-proof implementation.

### Gaps Summary

None. The former blocker was raw-output substitution. The current path retains the raw staged stdout, validates fields from that same data, renders the retained transcript through the focused proof viewport, and has both byte-level and real-PTY marker regressions that fail if the output is reconstructed.

---

_Verified: 2026-08-24T06:01:23Z_
_Verifier: the agent (gsd-verifier)_
