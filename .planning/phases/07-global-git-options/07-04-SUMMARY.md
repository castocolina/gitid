---
phase: 07-global-git-options
plan: 04
type: summary
wave: 4
---

# 07-04 Summary — scrolling master list, probe-failure rendering, and the real-PTY suite (with orchestrator hand-finish)

## What was built

Four commits total: Task 1 and Task 2 landed cleanly from the cross-AI opencode session; Task 3 was substantially hand-finished by the orchestrator after the cross-AI session exited cleanly at 11 of 12 required PTY cases. A fifth, unrelated pre-existing-bug fix (found only by this wave's own full `make test-e2e` run) landed as its own commit.

### Task 1 — scroll the master list safely (`e827511`)

`internal/tuikit/globalgit.go` gained `listWindowStart int` on `globalGitModel`, plus `gitVisibleRowCount`, `scrollWindowFor`, `gitComputeScrollWindow`, `gitCueLine`, and `gitRowForScreenRow` — the 07-UI-SPEC.md RESOLVED "overflow" row's scrolling master list and scroll-aware click mapping (the plan's own named highest-risk item, a bug class this project has hit twice before). The row budget is computed canonically from `frameBodyRows(minFrameHeight)` — the CANONICAL fixed frame height (30), never the caller-supplied one — so `handleKey` (which has no width/height parameter), `view`, and `handleClick` can never disagree about the window position. Cue format: `"↓ (+%d more options)"` / `"↑ (+%d more options)"`, registered in `gate-copy-freeze`. Click row index resolves as `windowStart + row`, bound-checked against the VISIBLE window, checkbox hit-test routed through `Selectable()`. `activate()` resets `listWindowStart = 0`. ~14 new unit tests cover scroll progression, cue rendering both directions, the both-edges-hidden tie-break (DOWN wins), click-at-nonzero-offset (the highest-risk test), click-on-cue-line (inert), click-on-non-selectable-row, width bounds at any scroll offset, and no-colour cue legibility — all against an INFLATED stub row count, the only way to force real overflow without violating the frozen frame size or the frozen 12-row policy table (see Task 3's Deviations below for why this distinction matters).

### Task 2 — render Global Git probe failures (`6185067`)

`handleKey` short-circuits with `return keyResult{model: m}` (unhandled) when `m.optionsErr != ""`, so tab/global navigation stays live on a probe failure — never a dead end. `activate` resets `m.optionsErr = ""` on each activation. `baselineCeremonyFor` now computes the D-07 cross-warning (`GlobalGitCrossWarningNameMissing`/`EmailMissing`) and passes it as the ceremony's `Hint`, and sets `PreviewMaxLines: len(strings.Split(preview, "\n"))` to avoid clipping the closing sentinel of the (now 43-line) full managed-block preview. `cmd/gitid/wiring.go`'s `GlobalGitOptionStates` wraps failures as `fmt.Errorf("git probe failed: %w", ...)`. `TestGlobalSSHCeremonyPreviewUsesUnchangedDefaultBudget` guards that Global SSH's own ceremony's `PreviewMaxLines` stays at the shared default (0) — proving Task 2's preview-widening is scoped to Global Git only. `DemoBanner`'s switch drops `TabGlobalGit` (only `TabDoctor` remains wired-demo-banner-true); `TestDemoBannerOnlyIdentitiesIsWired` renamed to `TestDemoBannerOnlyDoctorIsUnwired`, and `TestStorageDemoBannerGlobalSSHIsOff` updated to match.

### Task 3 — the real-PTY suite (`1a921bd`, hand-finished — see Deviations)

`e2e/harness_test.go` gained `FakeGitShimDir(t, version, failSubcommand)` — a fake `git` script that delegates to the real binary for everything except an overridable `--version` string and one failable named subcommand — proven by `TestFakeGitShimDelegatesOverridesVersionAndFailsSubcommand` (both halves: `git config --file <seeded> --list` returns real keys through the shim, AND `git --version` reports the overridden string). `e2e/global_git_pty_e2e_test.go` (new, 393 lines) carries the 12 required raw-keystroke PTY cases: Browse, ScrollBothDirections, EmptySelectionGuard, ApplyCancel, ApplyConfirm, DiffersRow, ProbeFailureStaysNavigable, FallbackPairSet, FallbackPairClear, CrossWarning, BelowGateVersion, and MidTransactionFailureAndRetry — each asserting on-disk outcomes (not just rendered text) and capturing a frame into `.planning/phases/07-global-git-options/ui-frames/` (14 frames total: the 12 cases plus both directions of the scroll test).

## Deviations

- **D-07-04-1 — The cross-AI session exited cleanly at 11/12 cases — orchestrator hand-finished Task 3**:

The opencode session (pid 95254) committed Tasks 1 and 2 cleanly, then made real, non-repeating progress on Task 3 for roughly 90 minutes before exiting on its own — not a crash — leaving a detailed, self-authored "Work State / Next Move" handoff note in its own log (a context-budget exit, not a stall or error). It left 11 of 12 required test functions written (missing only the mid-transaction case), plus the shim and its proof test, all uncommitted but building cleanly. Given the substantial salvageable, un-committed work and a clean (not crashed) exit, the orchestrator chose to hand-finish directly rather than relaunch a scoped continuation — consistent with this session's established judgment: relaunch for a crash with an unclear remaining scope, hand-finish for a clean exit with a well-understood, small remaining gap.

- **D-07-04-2 — Three test-authoring bugs found by actually running the 11 pre-existing cases**:

Running the suite standalone (`go test -tags e2e -race ./e2e/... -run 'TestGlobalGit_|TestFakeGitShim'`) surfaced three real bugs in the cross-AI's own tests — none in the implementation code, which had already been independently verified clean after Tasks 1 and 2:

  - **`TestGlobalGit_RealPTYScrollBothDirections`** asserted a scroll cue would appear at the bottom/top of the real 12-row fixture. It never can: `gitVisibleRowCount` (Task 1) deliberately computes its budget from the canonical `minFrameHeight` (30, matching `dummyTermWidth`/`dummyTermHeight`), and 12 rows × 2 lines = 24 always fits inside the 24–25 line budget (24 with the findings banner, 25 without) — **measured facts**, not assumptions (plan 02-15's standing lesson: record measured numbers). This is a permanent property of the frozen 07-UI-SPEC.md design, confirmed by reading the code, not a bug. Rewrote the test to assert the TRUE observable behavior instead: no cue ever renders at either boundary, and moving to the last row and back to the first is stable. The scroll/click-mapping mechanics themselves stay proven at the unit level (Task 1) against an inflated stub row count — the only way to force real overflow without violating the frozen frame size or the frozen policy table.
  - **`TestGlobalGit_RealPTYDiffersRow`** asserted the untruncated sentence `"set, differs from recommendation"`; the master row column truncates it. Switched to the substring `"differs"`, mirroring the project's existing unit-test workaround (`TestGlobalGitDiffersRowRendersWordNotNewGlyph`).
  - **`TestGlobalGit_RealPTYProbeFailureStaysNavigable`** called the shared `startGlobalGitPTY` helper, which unconditionally waits for `"init.defaultBranch"` before returning — impossible when the probe genuinely fails from the start, which is the entire point of this case. Added `startGlobalGitPTYExpectingProbeFailure`, opening the tab without that success wait.

- **D-07-04-3 — The 12th case (mid-transaction failure + retry) needed three iterations to find a real, working mechanism**:

The plan's suggested mechanism — `chmod ~/.gitconfig.d 0500` between opening the apply preview and confirming — does **not** work against this codebase: `filewriter.EnsureDir(baselineDir, 0o700)` (called by `runGlobalGitApply` immediately before the second write) unconditionally `os.Chmod`s the directory back to 0700 as a defensive property, not a bug. Confirmed by reading `internal/filewriter/filewriter.go`'s `EnsureDir`, and empirically: a real run against the chmod'd directory still produced `"Wrote → ~/.gitconfig.d/00-baseline"`.

The first substitute mechanism — pre-creating `~/.gitconfig.d/00-baseline` as a real directory (not a file) *before* the PTY session starts — also failed, for a different reason: the screen's own initial probe (`globalgit.Statuses`) reads the same baseline path to determine current option states, and chokes on the directory immediately, producing an unrelated probe-failure frame before the test could even reach the apply preview.

**Final, working mechanism**: open the apply preview against a genuinely absent baseline file (the same precondition `TestGlobalGit_RealPTYApplyConfirm` already uses successfully), then create the bogus directory right *after* the preview opens and *before* confirming. `mutationJournal.watchFile` (`cmd/gitid/wiring.go`) rejects the non-regular target with `"gitid: refusing non-regular transaction target"` — a real stat-based check, not a test-only hook — before either write lands. The test asserts the ceremony renders `"✗"` and the exact rejection message and `"Retry (Enter)"` (mirroring the SSH-side's existing symlink-based precedent, `TestGitConfiguration_RealPTYWriteFailureRollback`), asserts neither watched file was touched, removes the directory, retries, and asserts the retry succeeds with the correct on-disk content.

- **D-07-04-4 — A genuinely pre-existing, unrelated stale test — found only because this was the first full `make test-e2e` run in this phase**:

The orchestrator's independent verification battery includes `make test-e2e` (the FULL e2e suite across every phase), which earlier waves of this phase apparently never ran (their own SUMMARY files record only package-scoped `go test` and `pnpm` checks). This first full run surfaced one failure, `TestDummyDemo_MouseAndGitApply` in `e2e/dummy_demo_e2e_test.go` — confirmed via `git log` to be a Phase-3-era test (`8887c8c`, "test(03-19): cover dummy confirmation ceremony") that predates Phase 7's D-15/R-1 rule (the Global Git selection starts EMPTY on every screen entry, no demo pre-selection — 07-CONTEXT.md, citing 06-D-15). It assumed space would "unchoose" a demo-preselected row landing at "9 selected"; under the current, correct D-15/R-1 contract, space instead SELECTS the focused row from empty, landing at 1. Two further assertions were also stale relative to current copy (`"Write baseline managed block to ~/.gitconfig"` → current `"Write global-git managed block to "`; `"Global git baseline applied"` → current, empirically observed `"Baseline applied. user.email stays untouched — identities own their author."`, the frozen `GlobalGitResultTail` sentence). Fixed all three assertions and committed separately as `d39806d`, since this is a fix to unrelated, pre-existing drift, not part of this wave's own PTY-suite scope.

- **D-07-04-5 — Minor infrastructure note — stale golangci-lint cache**:

`make lint`'s default cache location retained stale file paths from a deleted sibling worktree (`../crossai-07-03`, removed after Wave 3 merged), causing a spurious lint failure unrelated to any code in this wave. Worked around with `GOLANGCI_LINT_CACHE="$PWD/.golangci-cache"` (a worktree-local cache, removed after each use) for every `make lint` invocation and every commit in this wave (the pre-commit hook re-runs lint on `pass_filenames: false`).

## Review

- **R-07-04-FAILCOMMIT (cycle 1 MEDIUM — "`failCommitAt` in compiled PTY")**: the plan's suggested `chmod ~/.gitconfig.d 0500` mechanism does not work (`filewriter.EnsureDir` unconditionally chmods it back to 0700); the working mechanism is pre-creating `~/.gitconfig.d/00-baseline` as a REAL DIRECTORY after the apply preview opens — `mutationJournal.watchFile` rejects the non-regular target with `"gitid: refusing non-regular transaction target"` before either write lands. Pinned by `TestGlobalGit_RealPTYMidTransactionFailureAndRetry` (failure renders, no file touched, remove + retry succeeds with correct on-disk content).
- **R-07-04-SHIM (cycle 1 MEDIUM — "shim vs real git writes")**: `FakeGitShimDir` delegates to the real `git` binary for everything except an overridable `--version` string and one failable named subcommand — the PTY harness controls machine properties without faking the writes. Pinned by `TestFakeGitShimDelegatesOverridesVersionAndFailsSubcommand` (both halves: real `git config --list` delegation AND `--version` override).
- **R-07-04-VIEWPORT (cycle 1 LOW — "shared ceremony viewport")**: the Global Git ceremony preview is widened (`PreviewMaxLines: len(strings.Split(preview, "\n"))`) so the 43-line managed-block preview is not clipped; Global SSH's own ceremony keeps the shared default budget. Pinned by `TestGlobalSSHCeremonyPreviewUsesUnchangedDefaultBudget` (`PreviewMaxLines == 0`, `VisibleLines == 10` for Global SSH).
- **R-07-04-CUE (cycle 1 LOW — "dummy six-row list may never cue")**: re-measured in 07-06 (Task 1, against the real registry): at the fixed 100×30 capture geometry BOTH surfaces' twelve-row lists fit the 24–25-line body budget (12 rows × 2 lines = 24), so NEITHER renders a scroll cue — the cue is EQUAL (absent on both), not a divergence and not a non-applicability. The PTY-level scroll mechanics stay proven by the unit tests against an inflated stub row count and by `TestGlobalGit_RealPTYScrollBothDirections` asserting the true no-cue boundary behavior.

## Real-PTY suite: full passing output

```
$ go test -tags e2e -race -timeout 300s ./e2e/... -run 'TestGlobalGit_|TestFakeGitShim' -v
=== RUN   TestGlobalGit_RealPTYBrowse
--- PASS: TestGlobalGit_RealPTYBrowse (7.19s)
=== RUN   TestGlobalGit_RealPTYScrollBothDirections
--- PASS: TestGlobalGit_RealPTYScrollBothDirections (6.95s)
=== RUN   TestGlobalGit_RealPTYEmptySelectionGuard
--- PASS: TestGlobalGit_RealPTYEmptySelectionGuard (5.23s)
=== RUN   TestGlobalGit_RealPTYApplyCancel
--- PASS: TestGlobalGit_RealPTYApplyCancel (5.40s)
=== RUN   TestGlobalGit_RealPTYApplyConfirm
--- PASS: TestGlobalGit_RealPTYApplyConfirm (5.45s)
=== RUN   TestGlobalGit_RealPTYDiffersRow
--- PASS: TestGlobalGit_RealPTYDiffersRow (5.32s)
=== RUN   TestGlobalGit_RealPTYProbeFailureStaysNavigable
--- PASS: TestGlobalGit_RealPTYProbeFailureStaysNavigable (5.56s)
=== RUN   TestGlobalGit_RealPTYFallbackPairSet
--- PASS: TestGlobalGit_RealPTYFallbackPairSet (8.47s)
=== RUN   TestGlobalGit_RealPTYFallbackPairClear
--- PASS: TestGlobalGit_RealPTYFallbackPairClear (7.72s)
=== RUN   TestGlobalGit_RealPTYCrossWarning
--- PASS: TestGlobalGit_RealPTYCrossWarning (5.74s)
=== RUN   TestGlobalGit_RealPTYBelowGateVersion
--- PASS: TestGlobalGit_RealPTYBelowGateVersion (6.52s)
=== RUN   TestGlobalGit_RealPTYMidTransactionFailureAndRetry
--- PASS: TestGlobalGit_RealPTYMidTransactionFailureAndRetry (5.54s)
=== RUN   TestFakeGitShimDelegatesOverridesVersionAndFailsSubcommand
--- PASS: TestFakeGitShimDelegatesOverridesVersionAndFailsSubcommand (0.28s)
PASS
ok  	github.com/castocolina/gitid/e2e	76.875s
```

## Exit Battery Results (independently run by the orchestrator, real output, on the final commit `d39806d`)

```
$ go build ./...
(clean)

$ go vet ./...
(clean)

$ TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...
ok  	github.com/castocolina/gitid/cmd/gitid	42.9s
ok  	github.com/castocolina/gitid/cmd/gitid-dummy	2.1s
ok  	github.com/castocolina/gitid/internal/adopter	2.5s
ok  	github.com/castocolina/gitid/internal/clipboard	2.7s
ok  	github.com/castocolina/gitid/internal/deps	3.0s
ok  	github.com/castocolina/gitid/internal/doctor	3.4s
ok  	github.com/castocolina/gitid/internal/doctor/checks	3.4s
ok  	github.com/castocolina/gitid/internal/dummytui	4.1s
ok  	github.com/castocolina/gitid/internal/filewriter	4.7s
ok  	github.com/castocolina/gitid/internal/gitconfig	8.7s
ok  	github.com/castocolina/gitid/internal/globalgit	5.5s
ok  	github.com/castocolina/gitid/internal/globalssh	5.7s
ok  	github.com/castocolina/gitid/internal/identity	4.6s
ok  	github.com/castocolina/gitid/internal/keygen	18.2s
ok  	github.com/castocolina/gitid/internal/platform	4.6s
?   	github.com/castocolina/gitid/internal/screenshot	[no test files]
ok  	github.com/castocolina/gitid/internal/sshconfig	10.5s
ok  	github.com/castocolina/gitid/internal/tester	6.8s
ok  	github.com/castocolina/gitid/internal/tuikit	21.7s
ok  	github.com/castocolina/gitid/internal/upload	3.9s
ok  	github.com/castocolina/gitid/internal/uploader	3.7s

$ GOLANGCI_LINT_CACHE="$PWD/.golangci-cache" make lint
0 issues (both plain and -tags screenshot)

$ make gate-copy-freeze
(all frozen strings present, all four dynamic exclusions confirmed not-frozen — the three from 07-03 plus the two new Task 1 cue formats and Task 2's advisory sentence)

$ go test ./internal/dummytui/ -run TestNoBackendAllowlist -v
--- PASS: TestNoBackendAllowlist (0.38s)

$ make test-e2e
go build -o bin/gitid ./cmd/gitid
go test -tags e2e -race -timeout 900s ./e2e/...
ok  	github.com/castocolina/gitid/e2e	597.330s
```

(The first `make test-e2e` run, before the `d39806d` fix, failed exactly one test — `TestDummyDemo_MouseAndGitApply` — for the pre-existing, unrelated reason documented in Deviation 4 above. The second run, on the fixed commit, is fully green.)

## Confirmation: the Wave 4 must_haves all hold

- **Scroll-offset-aware click mapping**: `internal/tuikit/globalgit.go`'s `handleClick` routes through `gitComputeScrollWindow`/`gitRowForScreenRow`, proven at the unit level against an inflated stub row count at a non-zero window start (Task 1's `e827511`), and proven stable at the boundary against the real 12-row fixture (Task 3's `TestGlobalGit_RealPTYScrollBothDirections`).
- **Global SSH ceremony preview untouched**: `TestGlobalSSHCeremonyPreviewUsesUnchangedDefaultBudget` asserts `PreviewMaxLines == 0` and `preview.VisibleLines == 10` for Global SSH's own ceremony — Task 2's preview-widening (`PreviewMaxLines: len(strings.Split(preview, "\n"))`) is scoped to Global Git's own `baselineCeremonyFor` call site only.
- **`DemoBanner` drops only Global Git**: the switch in `cmd/gitid/wiring.go` no longer includes `TabGlobalGit`; only `TabDoctor` remains wired-demo-banner-true, confirmed by `TestDemoBannerOnlyDoctorIsUnwired`.
- **PTY harness shim proves both halves**: `TestFakeGitShimDelegatesOverridesVersionAndFailsSubcommand` asserts real delegation (`git config --file <seeded> --list` returns real keys through the shim) AND version override (`git --version` reports the overridden string) in one test.
- **Every PTY case's frame is committed**: 14 files under `.planning/phases/07-global-git-options/ui-frames/` (12 required cases, with the scroll test contributing both `global-git-scroll-down.txt` and `global-git-scroll-up.txt`).
