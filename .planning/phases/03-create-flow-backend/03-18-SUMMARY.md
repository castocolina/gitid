---
phase: 03-create-flow-backend
plan: 18
subsystem: stage-two SSH proof transport
tags: [tdd, tester, pty, tuikit, raw-output]
status: complete
---

# Phase 3 Plan 18 Summary

## Outcome

Stage two now retains the exact stdout from its one staged `ssh -F <config> -G
<alias>` execution in `tester.Result.ResolutionOutput`. `Resolved` and
`ResolvedVia` parse validation fields from that same retained string. The real
backend projects it directly into `TestResultView.ResolutionOutput` while
preserving the real stage-two connectivity output in `Detail`.

The focused proof viewport continues to label Stage 1 and Stage 2, but both
outputs occur in its source unchanged. The raw-only fake-SSH marker proves that
reconstruction, sorting, trimming, or field selection cannot replace the real
resolution transcript.

No candidate, finalization, packet, baseline, allowlist, store gate, staged-key
cache, transaction, or candidate-policy artifact changed. No real user files,
keys, configuration, network provider, or external account was accessed.

## TDD Evidence

| Stage | Commit | Command and result |
|---|---|---|
| RED | `64e22fa` | `TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -race -count=1 ./e2e -run '^TestCreateFlow_Stage2RendersExactRawSSHOutput$'` failed: validation completed, but the focused proof viewport exposed neither the raw-only `gitidrawmarker proof-retained-verbatim` line nor the stage-two connectivity output. |
| GREEN | `8802f48` | The same compiled-real PTY command passed after retaining raw `ssh -G` stdout and leaving connectivity output in `Detail`. |
| Focused contracts | pending commit | `TERM=dumb SSH_AUTH_SOCK= go test -v -race -count=1 ./internal/tester ./cmd/gitid ./internal/tuikit -run 'Test(ResolvedViaRetainsRawResolutionOutput|ResolvedRetainsRawResolutionOutput|Stage2RetainsConnectivityAndRawResolutionOutput|FocusedProofContainsRawStage2Outputs|Stage2RecordsOutcomeAfterValidation|ToTestResultViewMapsEveryOutcome)$'` passed: 9 tests in 3 packages. |

The RED commit is an ancestor of the GREEN commit. The RED failure was
behavioral, not a fixture, compile, or setup failure.

## Focused Coverage

- `TestResolvedViaRetainsRawResolutionOutput` and
  `TestResolvedRetainsRawResolutionOutput` retain a marker, duplicate
  `identityfile`, deliberate spacing, unknown field, and trailing newline while
  parsing validation fields from the same stdout.
- `TestStage2RetainsConnectivityAndRawResolutionOutput` prevents either output
  from being substituted by `ResolvedConfig` data.
- `TestFocusedProofContainsRawStage2Outputs` requires contiguous connectivity
  and resolution transcripts inside the existing labeled viewport source.
- `TestCreateFlow_Stage2RendersExactRawSSHOutput` drives compiled `cmd/gitid`
  with a disposable HOME and test-owned fake SSH, focuses and pages the proof
  viewport, and observes the raw-only marker.
- Existing real create-flow PTY resolution checks now use the focused proof
  viewport rather than treating the compact `Detail` field as an IdentityFile
  oracle.

## UI Delta Classification

The only changed compiled-real versus live-`cmd/gitid-dummy` PTY region is the
real stage-two proof viewport. The real app now exposes raw staged `ssh -G`
stdout and keeps its actual stage-two connectivity output; the dummy keeps its
compact Phase-2 fixture summary.

Classification: **improvement**, required by TEST-01 and TEST-02. The dummy
remains the sole reference. No HTML, MUI, browser, Chromium, screenshot
baseline, or pixel comparison was used. `make gate-visual-regression` remains a
protected offline surrounding-behavior regression gate; it does not execute
production stage two and therefore does not prove the raw marker.

## Gates

All commands passed from the final source tree:

```text
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -race -count=1 ./e2e -run '^TestCreateFlow_'
TERM=dumb SSH_AUTH_SOCK= make test
TERM=dumb SSH_AUTH_SOCK= make lint
TERM=dumb SSH_AUTH_SOCK= make test-e2e
TERM=dumb SSH_AUTH_SOCK= make gate-copy-freeze
TERM=dumb SSH_AUTH_SOCK= make gate-visual-regression
```

`make test-e2e` passed in 168.927 seconds. The bounded proof helper stops after
all required markers are observed and uses a short PTY decode pause; the suite
stays within its existing 180-second timeout without weakening the proof.

## Protected Paths

The pre-existing dirty tracked paths retained their initial SHA-256 values
before every commit:

| Path | SHA-256 |
|---|---|
| `.planning/LEARNINGS.md` | `1914461a3d40dad8aae9b0a9766b8ca4dade0af8921c9b2f07edd83e4d801300` |
| `.planning/STATE.md` | `804c761c2c0d7a508a69f08a68d3edb5fb0d4e820331c2ef2bd425592dd064ef` |
| `.planning/config.json` | `e1ee8ff11b8316ecdd3834d3437e1fe18df1b03734d2fbe6d53423c860be14b0` |
| `03-09-review-packet/EVIDENCE.json` | `b0801041eb3ce30f1556289a630cf8f9e06f51f4d651891458e6e007f8950ea8` |
| `03-09-review-packet/panel-pngs/reuse-key-vs-generate.png` | `378eeaa4db94a3734bcf147b6f89477afc31593a5f84a0ccb958ada7c515f0a4` |
| `03-REVIEW.md` | `7633d9f77cf1945f622a77037e8c49233afb62d27cda11ebf1a788ac69f7e703` |

## Self-Check: PASSED

- Raw stage-two stdout and parsed validation facts are derived from one command
  execution and remain separate through the production composition boundary.
- The compiled real TUI proves raw-output retention with an unparsed marker.
- D-01/D-04 outcomes, automatic chaining, staging, fail-closed validation, and
  persistence gates remain protected by the focused and full suites.
- No review packet was finalized and Phase 3 was not marked complete.
