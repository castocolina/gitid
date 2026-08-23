---
phase: 03-create-flow-backend
plan: 16
subsystem: TUI evidence candidate determinism
tags: [tdd, screenshot, pty, evidence]
source_commit: 5f2dfe12d29976a2b82a20a12576eda203ce2e6d
status: candidate-ready-unfinalized
---

# Phase 3 Plan 16 Summary

## Outcome

The Phase 3 evidence candidate remains TUI-only and unfinalized. Finalization now
uses the configured single-review wording, canonical comparison excludes only
known volatile PTY artifacts, and each candidate receives a sibling canonical
manifest after its original integrity-checked manifest has been validated.

No review record, final packet, or publication was created.

## Hypothesis And TDD

Hypothesis: the remaining candidate variance was limited to real-PTY viewport
transcripts, while semantic text, required-screen inventory, and TUI-only
provenance remained stable.

- RED: `TestCanonicalManifestIgnoresRawPTYTranscriptVariation` first failed to
  compile because `writeCanonicalManifest` did not exist.
- RED: the same test then failed because a canonical manifest inside the
  candidate directory was correctly rejected as an undeclared candidate member.
- GREEN: the canonical manifest is now a sibling artifact; both candidates stay
  valid and their canonical manifests compare byte-for-byte.
- A real candidate capture exposed a stage-one proof-viewport timing race.
  `TestCaptureTUIScreenExactProofFrames` passed 10 consecutive runs after the
  capture waits for `Stage 1 command:` before sending the raw `v` focus key.

## Verification

| Command | Result |
|---|---|
| Task 2 screenshot/evidence selected tests, after exact-name `go test -list` checks | PASS, 18 tests in 3 packages |
| Task 2 TUI viewport selected tests, after exact-name checks | PASS, 3 tests |
| Task 2 real-PTY selected tests, after exact-name checks | PASS, 2 tests |
| Task 3 routine-gate selected tests, after exact-name checks | PASS, 5 tests |
| `TERM=dumb SSH_AUTH_SOCK= make test` | PASS |
| `make lint` | PASS, 0 issues |
| `TERM=dumb SSH_AUTH_SOCK= make test-e2e` | PASS, 136.162s |
| `make gate-copy-freeze` | PASS |
| `make gate-visual-regression` | PASS, 18 classified TUI frames; protected-region negative controls PASS |
| `go build -tags screenshot -o <handoff>/gitid-evidence ./cmd/gitid-evidence` | PASS |
| Two independent `<handoff>/gitid-evidence --candidate` runs for source `5f2dfe12d29976a2b82a20a12576eda203ce2e6d` | PASS |
| `cmp candidate-a.canonical-manifest.json candidate-b.canonical-manifest.json` | PASS |

## Candidate Handoff

Retained candidate: `/var/folders/5w/2d0vm3b96_qdc9q9x1_m59tw0000gn/T/gitid-03-16-handoff.6OcdZW/candidate-a`

Second isolated candidate: `/var/folders/5w/2d0vm3b96_qdc9q9x1_m59tw0000gn/T/gitid-03-16-handoff.6OcdZW/candidate-b`

| Artifact | SHA-256 |
|---|---|
| `candidate-a/CANDIDATE-MANIFEST.json` | `41cb6baeb51f10bfc9940ce3842be8b11fc3ec0947e01408259518dd6d914b67` |
| `candidate-b/CANDIDATE-MANIFEST.json` | `be8dfda8115dd390e4c67cb8b5cbe7d694f9f6e15326678798da1e1548441fff` |
| `candidate-a.canonical-manifest.json` | `a76a248f402353aa9356386acc0c12c722a1fb092cc934d01eb79023c3e4c8d4` |
| `candidate-b.canonical-manifest.json` | `a76a248f402353aa9356386acc0c12c722a1fb092cc934d01eb79023c3e4c8d4` |

The original manifests intentionally retain raw-PTY file hashes for integrity
validation. Their sibling canonical manifests normalize only documented
volatile members and have identical bytes and member-hash inventories.

## Protected Paths

The pre-existing dirty tracked paths retained their initial SHA-256 values
before and after every capture/gate operation:

| Path | SHA-256 |
|---|---|
| `.planning/STATE.md` | `804c761c2c0d7a508a69f08a68d3edb5fb0d4e820331c2ef2bd425592dd064ef` |
| `.planning/phases/03-create-flow-backend/03-09-review-packet/EVIDENCE.json` | `b0801041eb3ce30f1556289a630cf8f9e06f51f4d651891458e6e007f8950ea8` |
| `.planning/phases/03-create-flow-backend/03-09-review-packet/panel-pngs/reuse-key-vs-generate.png` | `378eeaa4db94a3734bcf147b6f89477afc31593a5f84a0ccb958ada7c515f0a4` |

All pre-existing untracked planning paths remained unmodified and unstaged.
Only the committed Phase 3 evidence source/tests and this summary were staged.
