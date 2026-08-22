---
phase: 03-create-flow-backend
plan: 14
status: corrective-implementation-complete
source_commit: f37431605a1d9ab91dc22ab770794edcab27c7b8
---

# Phase 03-14 Summary

## Outcome

Implemented the reviewed code corrections. This summary does not declare Phase 3 complete. No candidate, final review packet, raw review input, real HOME configuration, account, key, or network smoke command was created or used.

## Protected Artifacts

The pre-existing dirty 03-09 artifacts were not modified by this work. Their before/after SHA-256 values were unchanged:

| Path | SHA-256 |
|---|---|
| `03-09-review-packet/EVIDENCE.json` | `b0801041eb3ce30f1556289a630cf8f9e06f51f4d651891458e6e007f8950ea8` |
| `03-09-review-packet/panel-pngs/reuse-key-vs-generate.png` | `378eeaa4db94a3734bcf147b6f89477afc31593a5f84a0ccb958ada7c515f0a4` |

## RED And GREEN

RED command:

```sh
TERM=dumb SSH_AUTH_SOCK= go test -count=1 ./internal/tuikit/... -run 'Test(ExactTextViewport_HorizontalOnlyCueKeepsExactRowBudget|CeremonyDestructiveConfirmWordCanContainViewportKey|CompletedProofViewportRoutesAdvertisedControls)'
```

RED result: `0 passed, 3 failed`. The failures proved the horizontal cue exceeded the viewport row budget, `v` was removed from the destructive word `dev`, and the advertised proof `v` key was not routed.

GREEN command:

```sh
TERM=dumb SSH_AUTH_SOCK= go test -count=1 ./internal/tuikit/... -run 'Test(ExactTextViewport_HorizontalOnlyCueKeepsExactRowBudget|CeremonyDestructiveConfirmWordCanContainViewportKey|CompletedProofViewportRoutesAdvertisedControls)'
```

GREEN result: `3 passed`.

Protocol and registry GREEN command:

```sh
TERM=dumb SSH_AUTH_SOCK= go test -tags screenshot -race -count=1 ./internal/screenshot/... ./cmd/gitid-evidence/... -run 'Test.*(ScreenSpec|Semantic|Raw|Region|Final|Candidate|Provenance)'
```

GREEN result: `17 passed`.

Final quality gates:

```sh
TERM=dumb SSH_AUTH_SOCK= make test
make lint
TERM=dumb SSH_AUTH_SOCK= make test-e2e
```

Results: `make test` passed including `gate-copy-freeze`; `make lint` reported `0 issues`; `make test-e2e` passed using the hermetic fake SSH harness. No network smoke target was invoked.

## Corrections

- Candidate generation now runs two isolated candidate processes and compares their complete inventories. Candidate output remains outside the phase packet destination.
- `finalize` requires exactly two distinct review directories containing `metadata.json`, `prompt.txt`, `raw-stdout.txt`, `raw-stderr.txt`, and `verdict.json`.
- Finalization validates strict provenance and verdict schemas, source/candidate-manifest bindings, hashes for every raw asset, distinct reviewer/tool/provider identities, distinct stdout, zero exits, and zero open Critical/High findings. It builds a fresh staging directory and atomically renames only the validated final packet.
- The visual inventory is derived from `ScreenSpecRegistry`, with explicit live applicability and non-applicability reasons. It covers resolved manual reuse, completed pass proof frames, warning/copy, hard-failure/retry, D-19, and navigated confirmation frames. Every applicable member requires PNG, rendered text, and raw PTY transcript evidence.
- Region evidence now requires every declared region, rejects missing live/approved content, parses the stored region document during candidate and final validation, and rejects unexplained divergences.
- The capture driver sends raw `v`, PgDn, and Right navigation bytes for proof/confirmation variants and saves the captured raw PTY transcript.
- The viewport reserves the cue row without exceeding the row budget. The production proof handler routes its advertised `v` control. Destructive confirmation input keeps printable `v` text.

## Registry Inventory

The required semantic frames include `reuse-manual-resolved`, `test-stage1-pass`, `test-stage2-proof-top`, `test-stage2-proof-bottom`, `test-stage2-proof-right`, `test-reachable-not-uploaded`, `test-hard-failure-retry`, `confirm-summary`, and `confirm-managed-block`, in addition to the base form, reuse, mouse, Git, and confirmation frames.

## Candidate And Finalization Commands

Candidate command, after `freeze`, pinned Chromium, and `pnpm` are available:

```sh
TERM=dumb SSH_AUTH_SOCK= go run -tags screenshot ./cmd/gitid-evidence --candidate --source-commit f37431605a1d9ab91dc22ab770794edcab27c7b8 --output-dir /tmp/gitid-03-14-candidate
```

Finalization command, only after two real independent reviews of that exact candidate:

```sh
TERM=dumb SSH_AUTH_SOCK= go run -tags screenshot ./cmd/gitid-evidence finalize --source-commit f37431605a1d9ab91dc22ab770794edcab27c7b8 --candidate-dir /tmp/gitid-03-14-candidate --review-dir /path/to/ui-review --review-dir /path/to/codex-review --output-root .planning/phases/03-create-flow-backend/03-14-review-packet
```

## Remaining Blocker

Publication is correctly blocked. `freeze` was unavailable during the screenshot suite, and two real distinct independent raw reviews bound to one generated candidate do not exist. No fake reviews or final packet were generated.
