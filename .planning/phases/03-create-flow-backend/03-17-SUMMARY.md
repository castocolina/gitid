---
phase: 03-create-flow-backend
plan: 17
subsystem: create-flow PTY presentation
tags: [tdd, pty, tuikit, visual-regression]
status: finalized
---

# Phase 3 Plan 17 Summary

## Outcome

Corrected the three converged PTY presentation defects without changing core
test, persistence, configuration, or external-account behavior:

1. The completion heading now appears only after stage two completes.
2. The final ReachableNotUploaded state presents one D-02 warning and one D-03
   instruction while retaining both stage outputs.
3. The compact Git preview selects the first includeIf condition rather than a
   managed-block sentinel.

The evidence candidate is prepared but remains unreviewed and unfinalized.
No reviewer, finalizer, or packet-publication command was invoked.

## TDD Evidence

| Defect | RED commit | RED result | GREEN commit | GREEN result |
|---|---|---|---|---|
| Running stage claimed completion | `0cb2b04` | `TestWizardRunningStageDoesNotClaimAllStagesComplete` failed with `stage-two-in-progress frame must not claim all stages completed`; exit 1. | `704d57c` | Focused race test passed. |
| Reachable warning duplicated | `b4a409f` | `TestReachableWarningAndActionRenderOnce` failed with `D-02 warning count = 3, want 1`; exit 1. | `df583ac` | Focused race test and existing warning-state tests passed. |
| Compact preview showed sentinel | `ef5197c` | `TestCompactIncludeIfPreviewShowsCondition` failed because the condition was absent and the sentinel was present; exit 1. | `9a09671` | Focused race test passed. |

Each RED commit is an ancestor of its paired GREEN commit. The final compiled
PTY regression is in `d729c44`, with its stage-two timing correction in
`3c6f499`.

## PTY Validation

`TestCreateFlow_PTYReviewCorrections` passed with compiled `cmd/gitid` and
live compiled `cmd/gitid-dummy` at 100x30. It used only disposable HOME
directories and test-owned fake SSH.

The test verifies:

- A delayed stage-two `ssh -G` leaves stage one visible with `running ssh` and
  without the completion heading; the heading and proof appear after completion.
- The completed denied-key path contains one `! Reachable — key not uploaded
  yet` warning, one `Press c to copy the .pub` instruction, both stage outputs,
  and the footer copy action.
- The real compact preview contains an includeIf condition and excludes the
  managed-block sentinel; the live dummy displays its compact includeIf
  condition.

## Delta Classification

| Corrected region | Real versus live dummy result | Classification |
|---|---|---|
| Stage-two-in-progress copy | Both use the shared corrected render and no longer claim completion early. | Defect fixed; no remaining delta. |
| ReachableNotUploaded aggregate state | Both use the shared corrected render and show one warning/action with preserved proof. | Defect fixed; no remaining delta. |
| Compact includeIf preview | Both expose an includeIf condition. The real backend retains its actual managed gitdir value while the dummy retains its illustrative fixture value. | Improvement: the real preview now exposes its meaningful production stanza instead of a sentinel. |

## Gates

All commands passed:

```text
TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./internal/tuikit/... -run 'Test(WizardRunningStageDoesNotClaimAllStagesComplete|ReachableWarningAndActionRenderOnce|CompactIncludeIfPreviewShowsCondition)$'
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -race -count=1 ./e2e/... -run '^TestCreateFlow_PTYReviewCorrections$'
TERM=dumb SSH_AUTH_SOCK= make test
make lint
TERM=dumb SSH_AUTH_SOCK= make test-e2e
make gate-copy-freeze
make gate-visual-regression
```

`make test-e2e` passed in 164.200 seconds. The visual-regression gate passed
all 18 classified real/dummy screen specifications and its protected-region
negative controls.

## Protected Paths

The following pre-existing dirty tracked paths had identical SHA-256 values
before and after all Task 1 and Task 2 commands:

| Path | SHA-256 |
|---|---|
| `.planning/STATE.md` | `804c761c2c0d7a508a69f08a68d3edb5fb0d4e820331c2ef2bd425592dd064ef` |
| `03-09-review-packet/EVIDENCE.json` | `b0801041eb3ce30f1556289a630cf8f9e06f51f4d651891458e6e007f8950ea8` |
| `03-09-review-packet/panel-pngs/reuse-key-vs-generate.png` | `378eeaa4db94a3734bcf147b6f89477afc31593a5f84a0ccb958ada7c515f0a4` |

## Candidate Handoff

The deterministic, TUI-only candidate was generated from
`3c6f49946da953fa37058213e6a507bc2da9bddf` with the evidence binary and
paths bound in `03-17-HANDOFF.json`.

| Artifact | SHA-256 |
|---|---|
| `candidate/CANDIDATE-MANIFEST.json` | `f9521f6fa3e1077e5153201450f77878db4da6b99525b403c7f6a148ba42bc9b` |
| `candidate.canonical-manifest.json` | `84dfebaf4c483f44b6b53f23ca0a06ff2d9cac70bfba9b28e094c1505355a90c` |

The source-bound `opencode-review` directory exists but is intentionally empty.
The exact next step is Task 3: run only `opencode-my-plan-review` through
`opencode` with `local-llm-env/my-plan-review` against this handoff, then
finalize only if that review clears all three dispositions with zero Critical or
High findings.

## Deviations

The first PTY timing assertion sampled `testRunning1`. It was corrected to wait
for completed stage-one output and a running stage-two indicator before making
the assertion. No production behavior changed for that correction.

## Self-Check: PASSED

- All three focused regressions were RED before their paired production change
  and GREEN afterward.
- Compiled real and dummy PTY validation passed at 100x30.
- Required full gates passed.
- Existing dirty paths remained unchanged.
- No real user configuration, key, account, network, browser, HTML, MUI, or
  Chromium surface was used.
- Review, finalization, and publication evidence is recorded below.

## Orchestrator Review And Finalization

The sole configured review was preserved outside the repository at
`/var/folders/5w/2d0vm3b96_qdc9q9x1_m59tw0000gn/T/opencode/gitid-03-17-review.y8tJax/orchestrator-review`.
It contains the exact review request, the unmodified direct-review output log,
an empty observed stderr stream, timestamps, exit status, metadata, and a
machine-readable verdict.

| Binding | Value |
|---|---|
| Reviewer / CLI / provider / model | `opencode-my-plan-review` / `opencode` / `local-llm-env` / `local-llm-env/my-plan-review` |
| Session | `ses_fd0d7e845ffePr6WJm6HasDnQo` |
| Started / ended | `2026-08-23T15:06:05Z` / `2026-08-23T15:16:09Z` |
| Exit status | `0` |
| Source commit | `1543cff2bde6c367b9da1945c0b141b4f6debee6` |
| Candidate manifest file SHA-256 | `7b2d69b4d677978cd6e537f055f9ac423442d4c21ee6699fea79d3c177e41a8b` |
| Candidate manifest self-hash | `cc6d42d40fe9669883b46b0e0b359e9fee99a21855ee7646cc4f7baf7b8a2af1` |
| Canonical manifest file SHA-256 | `ac4733fabb95eb746a2a01a5b5f8fb446a0b87c0f373c68dbba826eff9716663` |
| Canonical manifest self-hash | `ea9535d1074843beca5d29bf1fba816c7bea9a3c785e9fa48ca0e35cb0c17077` |
| Verdict | Zero Critical, zero High; all differences classified and no defects. |

The previously supplied canonical hash belonged to a prior candidate. The
freshly verified canonical file and self-hash above are authoritative for this
candidate.

Finalization command and result:

```text
gitid-evidence finalize --source-commit 1543cff2bde6c367b9da1945c0b141b4f6debee6 --candidate-dir <external-candidate> --review-dir <external-orchestrator-review> --output-root .planning/phases/03-create-flow-backend/03-17-review-packet
PASS: published .planning/phases/03-create-flow-backend/03-17-review-packet/1543cff2bde6c367b9da1945c0b141b4f6debee6
```

The final packet's manifest self-hash is
`f0671195fa171cc07de1ae36074fc2252c3c3c99771481416299d862b18f10ca`.
An independent post-publication scan verified all 77 declared member hashes,
the single `03-16.1` review provenance record, the configured
reviewer/CLI/provider/model/session/exit-status bindings, the bound candidate
self-hash, the zero-blocker verdict, and byte-identical embedded prompt,
stdout, stderr, and verdict assets.
