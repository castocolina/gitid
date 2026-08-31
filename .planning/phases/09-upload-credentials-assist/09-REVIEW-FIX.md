---
phase: 09-upload-credentials-assist
fixed_at: 2026-08-31T03:46:50Z
review_path: .planning/phases/09-upload-credentials-assist/09-REVIEW.md
iteration: 2
findings_in_scope: 8
fixed: 5
skipped: 3
status: partial
---

# Phase 9: Code Review Fix Report

**Fixed at:** 2026-08-31T03:46:50Z
**Source review:** .planning/phases/09-upload-credentials-assist/09-REVIEW.md (iteration 4)
**Iteration:** 2

**Summary:**
- Findings in scope: 8 (1 Critical, 7 Warnings — `fix_scope: critical_warning`, the 16 Info findings excluded)
- Fixed: 5 (CR-01, WR-01, WR-02, WR-03, WR-04)
- Skipped: 3 (WR-05, WR-06, WR-07 — carried-forward design/copy decisions, explicitly marked "do-not-mechanically-fix" by the review; unchanged from iteration 1)

This pass fixes a **regression the iteration-1 fixer pass introduced**: its
own WR-01 fix (`UploadRunMsg.Name` stale-guard) was applied to the real
backend and both consumers, but never to any of the three fixture backends,
silently breaking `gitid-dummy`'s D-08/D-04 surfaces and taking 13 gate
tests red. Every fix below was written test-first per CLAUDE.md's
hypothesis → test → implementation loop: a regression test was added,
confirmed to FAIL against the pre-fix code, then confirmed to PASS once the
fix landed. Per the task's explicit instruction, **both tagged gates that
missed the iteration-1 regression were run this time**, not just the
untagged suite:

```
go build ./...                                                          # clean
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...                   # 2373 passed, 22 packages, exit 0
make lint                                                                # golangci-lint: 0 issues (both untagged and screenshot-tagged)
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e ./e2e/ -timeout 30m           # 175 passed, 0 failed
TERM=dumb SSH_AUTH_SOCK= go test -tags screenshot ./internal/screenshot/... ./cmd/gitid/... -timeout 20m
                                                                          # 831 passed, 1 failed (environmental), 1 skipped
```

**Note on `-timeout`:** this sandbox is slow enough that the default 10m
`go test` package timeout was not enough to finish the full ~175-test
real-PTY e2e suite or the ~832-test screenshot-tagged suite; two runs at
the default timeout each aborted mid-suite on `panic: test timed out after
10m0s`, on a *different* test each time (`TestIdentityManager_
CLIAndTUIProduceByteIdenticalGitconfig` the first run,
`TestIdentityManager_ListPopulatedEightTaxonomy` the second) — a
package-wide timeout artifact, not a specific test failure. Re-running each
individually-"failed" test in isolation passed in a few seconds; re-running
the full suites with `-timeout 30m`/`-timeout 20m` completed cleanly with
zero non-environmental failures. The one screenshot-tagged failure
(`TestCaptureTUI`) is the same environmental failure the review itself
flagged (`freeze` binary not on PATH in this sandbox) — not a regression.
`TestRegisterKeyModal_CompiledRealVsLiveDummyPTY` (the review's own named
proof test) and `TestRegionDiffCoverage` (the bisection target) were also
run individually and pass (3/3 and 1/1 respectively).

All work was done in an isolated git worktree (`gsd-reviewfix/09-<pid>`,
per `workflow.use_worktrees: true`) and fast-forwarded into
`gsd/phase-09.5-full-ssh-git-properties-browser` by the orchestrator's
cleanup tail.

## Fixed Issues

### CR-01: `UploadRunMsg.Name` stale-guard added to consumers but not to any fixture backend — every dummy/screenshot upload reply silently discarded

**Files modified:** `internal/dummytui/fixturebackend.go`, `internal/screenshot/createflow.go`, `e2e/identity_manager_pty_e2e_test.go`
**Commit:** `cc37af7`
**Applied fix:** Set `Name` at the three fixture dispatch sites the review named — `FixtureBackend.RunUpload` (`spec.Identity`), `FixtureBackend.RunUploadForIdentity` (`name`), and `offlineCaptureBackend.RunUpload` (`spec.Identity`, mirroring the real backend's `wiring.go` dispatch sites) — exactly as the review's Fix section specified. `offlineCaptureBackend.RunUploadForIdentity` needed no separate change: it embeds `tuikit.Backend` and does not override that method, so it inherits `FixtureBackend`'s fix automatically. Also added the anti-drift guard the review's Fix section requested: `TestRegisterKeyModal_CompiledRealVsLiveDummyPTY`'s dummy leg (the "dummy sibling" of `TestIdentityManager_RegisterKeyModalRuns`) now asserts `mustSee(t, dummy, "Authentication key registered", …)` in addition to the pre-existing announce-line-only check, so a dropped fixture reply fails loudly instead of silently degrading to "Registering…" forever.
**Verified (test-first):** confirmed RED — the new anti-drift assertion, and the pre-existing `mustSee(t, dummy, "Running:", …)` above it, both failed against the pre-fix worktree exactly as the review describes ("dummy: 'Running:' never appeared"). After the fix: `go test -tags e2e ./e2e/ -run TestRegisterKeyModal_CompiledRealVsLiveDummyPTY` → 3/3 passed; `go test -tags screenshot ./internal/screenshot/... -run TestRegionDiffCoverage` → 1/1 passed (was red from this same defect per the review's bisection).

### WR-02: the dummy backend's register-key preview commands were still unquoted — the exact surface iteration-3's WR-05 named but left half-applied

**Files modified:** `internal/dummytui/fixturebackend.go`, `internal/dummytui/fixturebackend_test.go` (new)
**Commit:** `dbd1c08`
**Applied fix:** Applied the same hardcoded-quoted shape `RunUpload` already used to `RunUploadForIdentity`'s three preview commands (`--title '%s'` / `-t '%s'` instead of `--title %s` / `-t %s`) — the D-07 title always contains spaces, so the unquoted form would run a different command than the one gitid actually runs if pasted into a shell. This is the method that feeds the D-08 register-key pane, i.e. the `register-key-modal` surface the review's Fix section named explicitly.
**Verified (test-first):** added `TestRunUploadForIdentityQuotesTitle` (new test file — no prior unit test exercised `RunUploadForIdentity`'s command shape at all), confirmed RED against the pre-fix code (`row.Command` contained the unquoted title), then GREEN after the fix. Full `internal/dummytui`/`internal/tuikit`/`internal/screenshot` suites (523 tests) still pass.

### WR-01: WR-08's truncation cue pushed `renderKeyCeremony`'s overflow backstop one row past its own frame budget

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identity_manager_upload_test.go`
**Commit:** `7f57663`
**Applied fix:** The review's literal Fix snippet (`lines := maxInt(1, budget-1)`) was tried first, adapted from its suggestion, and empirically instrumented against the real render output — it reduced the overrun from 2 rows to 1 but did not fully close it in this codebase's actual arithmetic (verified by temporary `fmt.Printf` instrumentation of `rendered`/`budget`/`lines`/`tailLines`/total-row-count against a real overflowing render, not derived on paper). `lines := maxInt(1, budget-2)` is what makes the invariant `strings.Count(out, "\n") + 1 <= frameBodyRows(minFrameHeight)` hold exactly, verified against real output. This reserves room for both the appended cue's own line and the trailing `"\n"` this overflow branch appends after it (an artifact of the pre-cue return already ending in a bare `"\n"`, now pushed one row further by the cue line inserted before it).
**Verified (test-first):** added `TestKeyCeremonyOverflowBackstopStaysWithinFrameBudget`, which forces a genuinely long tail (two upload rows with 140+-column `gh ssh-key add` command lines, plus a 6-line `ManualCommand`) so `tailLines > budget` while `budget` stays positive — the exact branch the review notes the tracked rotate fixtures never enter, so a byte-identical A/B check against them could not see this. Confirmed RED (27 lines, then 26 with `budget-1`, want ≤ 25) before landing `budget-2`, then GREEN. Debug instrumentation was removed before commit. Full `internal/tuikit` suite (520 tests) passes; the existing `TestRotateDeleteOfferFitsTheFrameBudget` (which measures via the wrapped `view()`/`sv.body` — always exactly `frameBodyRows` rows because `joinMasterDetail` pads/clamps to that length, so it cannot itself catch this class of defect) still passes unchanged.

### WR-03: `extractUploadedTitle` reported the pub path (or a truncated identity name) as the registered title whenever an earlier argv element needed shell-quoting

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identity_manager_upload_test.go`
**Commit:** `303e5a6`
**Applied fix:** Adapted, not blindly applied, the review's suggested `strings.Fields`-based rewrite: the literal snippet in the review's Fix section (`strings.Join(fields[i+1:], " ")`, trimmed of `'`) would itself have swallowed the trailing `--type`/`--usage-type` flag into the returned "title" for the normal case, since `buildArgs` always places `--type`/`--usage-type` immediately after the title. Implemented a small hand-rolled scanner instead: locate `--title `/`-t ` by substring position (immune to what's quoted earlier in the line — the review's core insight), then parse exactly the one operand that follows — either the next whitespace-delimited bare token, or, if quoted, the matching closing `'` while correctly unescaping shellQuote's `'\''` embedded-single-quote convention (rather than stopping at the *first* `'` found, which is what let an apostrophe in an identity name truncate the result before).
**Verified (test-first):** added `TestExtractUploadedTitle` with 6 cases covering every scenario the review named (a $HOME with a space quoting the pub path first, an identity name with an apostrophe, a spaced toolPath quoted before every other argument) plus a no-flag-present case and a plain-command sanity case. Confirmed RED against the pre-fix code — the 3 review-named cases failed exactly as described (`"/Users/John Smith/.ssh/id_ed25519_work.pub"` returned instead of the title; `"~/.ssh/id_ed25519_o"` truncated at the escaped quote; the spaced tool path returned instead of the title) — then GREEN after the fix. Full `internal/tuikit` suite passes.
**Note:** a stray Unicode right-double-quotation-mark character (`”`) appeared in two of this fix's doc comments after the pre-commit hook's `goimports -w`/`gofmt -w` pass ran (both instances replaced a `` `'\''` `` backtick-quoted code span containing two adjacent single quotes). Neither `goimports` nor `gofmt` perform textual quote substitution, so the exact mechanism is unclear; both corrupted comments were rewritten to avoid the risky adjacent-apostrophes-before-backtick pattern (spelled out in prose instead) and reverified clean (`grep` for `”`/`“`, rebuild, retest) before the commit that actually landed. Flagging this for visibility in case it recurs on a future fix — it is a comment-only corruption (verified via `go build`/`go vet`/`go test` all passing throughout), not a functional defect, but worth a human's attention if seen again.

### WR-04: the iteration-1 fix report asserted a gate result the bisect disproves, and its close-out never ran the two tagged gates where the damage landed

**Files modified:** none (documentation-only finding — corrected by this report itself)
**Commit:** none
**Applied fix:** No source change applies to this finding; it is about `09-REVIEW-FIX.md`'s own prior claims and process. Addressed by (1) this report replacing the prior one (overwritten, not appended, as instructed) with accurate content, and (2) actually running both tagged gates (`-tags e2e`, `-tags screenshot`) as part of this pass's own close-out — see the gate battery at the top of this report. The prior report's WR-05 entry claimed `TestRegionDiffCoverage` was "pre-existing before this change" based on an A/B stash comparison against a single file (`be319ab`); the iteration-4 review's bisection proved it was actually introduced five commits earlier at `cdc6932` (the same fixer pass's own first commit) — a claim this fixer cannot retroactively correct in the old artifact, only supersede.

## Skipped Issues

### WR-05 (carried from iteration 3's WR-03 — DEFERRED, do-not-mechanically-fix): the D-08 register-key pane mutates on open with no confirmation or cancel

**File:** `internal/tuikit/identities.go:2528-2534`, `:2650-2662`, `:2360-2370`, `:2665-2675`
**Reason:** Unchanged from the iteration-1 skip rationale. This is a tracked, documented design contract, not an oversight: `.planning/design/identity-manager/FIELDS.md:87` states "the D-01 checkbox is OMITTED here — opening the modal IS the explicit opt-in, so it announces-and-runs immediately when the provider matches and the tool is authenticated," restated in `TestIdentityManager_RegisterKeyModalRuns`'s own doc comment. It genuinely tensions with CLAUDE.md's confirmation rule, but changing it requires amending `FIELDS.md`, the frozen copy in `design.go`, the visual allowlist, and at least three real-binary e2e tests — a design amendment, not a mechanical fix. The review explicitly marks this do-not-mechanically-fix and recommends routing through `/gsd-discuss-phase` or a tracked design amendment.

### WR-06 (carried from iteration 3's WR-10 — DEFERRED, do-not-mechanically-fix): the multi-line `ManualCommand` is interpolated into a single-line sentence

**File:** `cmd/gitid/upload_run.go:552-558`, `internal/tuikit/design.go:695`, `internal/tuikit/identities.go:2812`, `:2821`
**Reason:** Unchanged. Requires a `design.go` R22 frozen-copy amendment (an indented block, or a `" && "` join) — a deliberate copy/rendering decision the review explicitly says must be filed, not patched in isolation.

### WR-07 (carried from iteration 3's WR-11 — DEFERRED, do-not-mechanically-fix): the upload checkbox's actionable copy is unreadable at the only width production uses

**File:** `internal/tuikit/identities.go:4747`, `:4779-4783`, `internal/tuikit/design.go:582`, `:586`
**Reason:** Unchanged. Requires amending frozen copy to fit 60 columns (a copywriting decision) or widening the row — both R22 amendments the review says to file as a ROADMAP/backlog item, not patch mechanically.

---

_Fixed: 2026-08-31T03:46:50Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
