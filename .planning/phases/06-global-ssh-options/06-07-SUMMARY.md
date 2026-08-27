---
phase: 06-global-ssh-options
plan: 07
subsystem: screenshot
tags: [visual-regression, review-packet, exit-battery, phase-closure]
requires:
  - phase: 06-global-ssh-options
    provides: [everything built in 06-01 through 06-06]
provides:
  - "Every Global SSH screen state (7 specs, 34 frames total with the 27 pre-existing) registered in the visual-regression gate, each with a classified real-versus-dummy divergence disposition or a named non-applicability record pointing at an existing PTY frame"
  - "Four working negative controls proving the gate can actually fail (missing state, unclassified difference, a perturbed comparable-equal region, cross-surface allowlist leakage)"
  - "The cross-AI review packet: 17 captured PTY frames, the classified allowlist, and a manifest with an 82-row closure table indexing every finding from all seven per-plan <review_disposition> sections"
  - "The phase's full exit battery run and recorded with real output"
affects: [phase 06 completion / phase-close; the next phase's starting point]
actuals:
  tokens: 60000
  tasks: 2
  commits: 2
tech-stack:
  added: []
  patterns:
    - "A visual-regression spec that structurally cannot produce a comparable capture (an honest error state one side renders and the other cannot) is registered live-non-applicable pointing at a real PTY frame, never forced into a fabricated allowlist entry."
    - "A closure table indexed from the accumulating per-plan <review_disposition> sections, not from the review log itself (which only retains its latest cycle's content) — row count derived by a written, re-runnable parser and cross-checked, never hard-coded."
key-files:
  created:
    - .planning/design/global-ssh/visual-divergence-allowlist.txt
    - .planning/phases/06-global-ssh-options/review-packet/MANIFEST.md
    - .planning/phases/06-global-ssh-options/review-packet/frames/ (17 files)
    - .planning/phases/06-global-ssh-options/review-packet/visual-divergence-allowlist.txt
  modified:
    - internal/screenshot/createflow.go — 7 Global SSH RequiredScreenSpecs, region dispositions
    - internal/screenshot/createflow_regions.go — Global SSH region extractors
    - internal/screenshot/createflow_packet.go, createflow_packet_test.go, createflow_test.go — Global SSH capture merge into existing packet/registry test helpers
    - cmd/gitid/gate_visual_regression_test.go — Global SSH acceptance tests, four negative controls
    - Makefile — gate-visual-regression's -run filter extended to select the new tests
requirements-completed: [GSSH-01]
duration: 285min
completed: 2026-08-27
status: complete
---

# Phase 06-07: Visual-regression closure and review packet Summary

**Every Global SSH screen state is now pinned against the approved Bubble Tea dummy with a gate proven to fail on demand, and the phase's full exit battery — every gate it owns — is green with real recorded output, closing Phase 6 (Global SSH Options) end to end across all 7 waves.**

## Performance
- **Duration:** ~285 min across 2 dispatch cycles (1 initial + 1 targeted resume for Task 2)
- **Tasks:** 2
- **Files modified:** 27 (2482 insertions)

## Accomplishments
- Registered 7 new `RequiredScreenSpecs` (Options list, apply preview/receipt, Storage browse/preview, migrate preview/receipt) — 34 frames total checked as a classified real/dummy symmetric union.
- Every spec explicitly records the approved Bubble Tea dummy as its sole parity target and the approved-HTML surface as non-applicable, per `AGENTS.md`'s binding Phase 3-10 UI Reference rule — asserted per-spec by `TestGlobalSSHHTMLNonApplicabilityPerSpec`.
- Found and fixed a genuine bug in the gate's own comparison machinery: `BuildRegionDiffs`' JSON round-trip was lossy when the dummy's master-list truncation clipped a multi-byte `→` rune mid-sequence, producing invalid UTF-8 that `json.Marshal` silently replaced with U+FFFD — corrupting the content hash even when nothing had actually changed. Fixed with a `sanitizeRegion` normalization step.
- Correctly distinguished a genuine one-sided UI state (the real backend's honest "layout is already X — nothing to plan" error when browsing the current storage layout, versus the dummy's always-rendered frozen preview) from a classifiable divergence — registered live-non-applicable rather than forcing a fabricated allowlist entry for a state the real backend structurally cannot produce.
- Four negative controls proving the gate can fail: a missing state, an unclassified difference, a perturbed comparable-equal region (mutation-sensitivity), and cross-surface allowlist leakage — all pass, and a dedicated test asserts the Makefile's `-run` filter actually selects all four by name.
- Assembled the cross-AI review packet with a mechanically-derived, cross-checked 82-row closure table indexing every finding recorded across this phase's five review cycles (as recorded in the seven per-plan `<review_disposition>` sections — `06-REVIEWS.md` itself retains only its latest cycle's content at any point in time).

## Task Commits
1. **Task 1: Register every Global SSH screen state in the visual-regression gate** - `37fe5ba` (feat)
2. **Task 2: Assemble the cross-AI review packet** - `1e2b5a3` (docs)

**Plan metadata:** this commit (docs: add plan summary)

## Files Created/Modified
- `internal/screenshot/createflow.go` — 7 Global SSH `RequiredScreenSpecs`, their region dispositions (improvement/defect classifications), and the live-non-applicability record for the current-layout Storage browse state
- `internal/screenshot/createflow_regions.go` — Global SSH region extractors (Options list, apply ceremony/heading, Storage panes)
- `internal/screenshot/createflow_packet.go`, `createflow_packet_test.go`, `createflow_test.go` — merged the Global SSH captures into the existing cross-cutting registry/packet test helpers (`captureCombined` and equivalents), following the Phase 4/5 precedent
- `cmd/gitid/gate_visual_regression_test.go` — `TestGlobalSSH*` acceptance tests (HTML non-applicability, PTY-frame evidence existence, four-state fixture coverage, allowlist schema, Makefile filter selection, cross-run determinism, prior-surface stability) plus the four `TestNegativeControl_GlobalSSH*` controls
- `Makefile` — `gate-visual-regression`'s `-run` filter extended with the `GlobalSSH|NegativeControl_` alternation
- `.planning/design/global-ssh/visual-divergence-allowlist.txt` — the classified real-versus-dummy divergences for this surface (6-field schema: spec, region, classification, requirement ID, rationale)
- `.planning/phases/06-global-ssh-options/review-packet/` — `MANIFEST.md`, `frames/` (17 PTY frames from plans 06-04 and 06-05), `visual-divergence-allowlist.txt`

## Decisions Made
- The visual-regression gate's JSON-round-trip content hash must operate on sanitized (valid-UTF-8) region text, not raw extracted bytes — an encoding artifact from truncation must never be indistinguishable from a real content change in the hash used to detect drift.
- A screen state that one backend can honestly produce and the other structurally cannot (not a design choice, a correctness fact) is registered live-non-applicable rather than forced into the classified-divergence path — the classification machinery is for differences between two comparable renderings, not for asymmetric capability.
- The review packet's closure table is indexed from the per-plan `<review_disposition>` sections (which accumulate across the phase's history) rather than from `06-REVIEWS.md` directly (which is overwritten to the latest cycle only) — this matches the plan's own stated design ("The per-plan `<review_disposition>` sections are the source; this is their index").

## Deviations from Plan
None in scope — both tasks match the plan's `<action>`/`<acceptance_criteria>` blocks. Process deviations, documented for institutional memory:

1. **Orchestrator fix after a crash left Task 1 uncommitted but code-complete:** `TestGlobalSSHMakefileFilterSelectsControls` had an off-by-one relative path (`../Makefile` from `cmd/gitid`, which is one level short of repo root; every other test in the same file correctly uses `../../`) that made it silently `t.Skipf` instead of running. Fixing the path surfaced a second, genuine bug in the same test: it matched the FIRST `go test -tags screenshot -run` line anywhere in the whole Makefile (there are three — `TestProvisionPinnedChromium`, `TestCaptureTUI`, and this gate's own) instead of scanning specifically from the `gate-visual-regression:` target. Rewrote the scan to walk from that target line and stop at the recipe's end. Both bugs verified real (reproduced, fixed, re-verified) before committing — not test-authoring artifacts.
2. **The crash itself:** a sandboxed `/tmp` write attempt (backing up the allowlist file to `/tmp/allowlist-backup.txt` while hand-proving one of the four negative controls) was auto-rejected by the harness's `external_directory` permission gate, per this dispatch's own explicit prohibition on writing outside the repo. The actual gate code was already correct and complete at that point — only the manual proof-of-fail-path exercise was interrupted, and the four negative controls' own automated tests (which don't touch `/tmp`) already covered that exact property.
3. **`TestApprovalCommitRecorded` SKIPs in the worktree environment** (`cannot read .git/HEAD: open ../../.git/HEAD: not a directory`) — this is a pre-existing test, unrelated to this plan, that reads `.git/HEAD` directly; in a git *worktree* `.git` is a file (pointing at the real gitdir), not a directory, so the read fails there. This resolves in the actual (non-worktree) repository, confirmed by the orchestrator: the merged branch's own `git log`/build/test verification happens in the main checkout, not a worktree.

## Issues Encountered
This wave needed 2 dispatch cycles. The first (Task 1) crashed mid-implementation on the recurring environment glitch documented across this session's other waves (an intermittent path-corruption bug in the cross-AI runtime, unrelated to this plan's own code) — but left the actual gate implementation complete and correct; the orchestrator independently verified it, found and fixed the two test bugs above, and committed. The second dispatch (Task 2) completed the review packet and full exit battery in one pass without further intervention, then also crashed at the very end on the `/tmp`-write rejection described above — but by that point Task 2's real deliverable (the manifest, the packet, the exit battery) was already complete and uncommitted; the orchestrator verified it, discarded only the incidental e2e-run timestamp artifacts in `ui-frames/`, and committed.

## Exit Battery Results

Every command below was run for real by the orchestrator (not the executor's self-report) against this plan's own final commit, in the worktree, at Task 1's close and again — independently, a second time, fresh — at Task 2's close:

| Command | Result |
|---|---|
| `go build ./...` | exit 0, no output |
| `go vet -tags screenshot ./...` | exit 0, no output |
| `TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./...` | `ok` — 19 packages (18 with test files + `internal/screenshot` excluded by build tag from the untagged run) |
| `golangci-lint run` (plain + `--build-tags screenshot`) | `0 issues` both |
| `make gate-copy-freeze` | all frozen strings present, D-13 dynamic-prefix exclusion holds |
| `make gate-no-backend-files` | prints its Phase-3 retirement notice and delegates to the durable guard below; exit 0 |
| `go test ./internal/dummytui/ -run TestNoBackendAllowlist` | `--- PASS`, `ok` |
| `make gate-visual-regression` | `PASS` — `ok github.com/castocolina/gitid/cmd/gitid 31.320s`; 34 `RequiredScreenSpecs` frames (27 pre-existing + 7 Global SSH), all four Global SSH negative controls pass, all `TestGlobalSSH*` acceptance tests pass; `TestApprovalCommitRecorded` SKIPs for the worktree-specific `.git`-is-a-file reason documented above |
| `make test-e2e` | `ok github.com/castocolina/gitid/e2e 523.639s` (both PTY and headless CLI suites, well inside the 900s timeout) |

All four parity-matrix tests pass (verified as part of the `go test -race ./...` run above, `cmd/gitid` package); `docs/cli-parity-matrix.md` carries no deferred row for the `ssh` noun (verified in wave 06-06, unchanged since).

## Next Phase Readiness

**This is the closing plan of Phase 6 — there is no Wave 8.** All 7 waves are now implemented, independently verified, and merged into `gsd/phase-06-global-ssh-options`.

**The cross-AI reviews of this packet have explicitly NOT been run.** Running them is an orchestrator obligation this executor cannot fulfill — an executor has no mechanism to spawn reviewer subagents. The packet at `.planning/phases/06-global-ssh-options/review-packet/` is assembled and ready for that review pass.

Per `ONESHOT.md`'s Per-Phase Checklist, items 6-9 (code review via `gsd-code-reviewer`, goal-backward verify-work via `gsd-verifier`, UI review via `gsd-ui-auditor` against real PTY output, and `/gsd-audit-uat`) have not yet run for this phase and are its natural next step before Phase 6 can be marked complete in `STATE.md`/`ROADMAP.md`.

---
*Phase: 06-global-ssh-options*
*Completed: 2026-08-27*
