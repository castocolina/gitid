---
phase: 09-upload-credentials-assist
fixed_at: 2026-08-31T05:39:05Z
review_path: .planning/phases/09-upload-credentials-assist/09-REVIEW.md
iteration: 3
findings_in_scope: 8
fixed: 6
skipped: 2
status: partial
---

# Phase 9: Code Review Fix Report

**Fixed at:** 2026-08-31T05:39:05Z
**Source review:** .planning/phases/09-upload-credentials-assist/09-REVIEW.md (iteration 5)
**Iteration:** 3 (final automated round per the review/fix loop's 3-iteration cap)

**Summary:**
- Findings in scope: 8 (1 Critical, 7 Warnings — `fix_scope: critical_warning`, the 24 Info findings excluded)
- Fixed: 6 (CR-01, WR-01, WR-02, WR-03, WR-04, WR-05 sub-defect)
- Skipped: 2 (WR-06, WR-07 — deliberately deferred design/copy decisions; WR-05's own design decision, the D-08 no-confirm contract, is also correctly left open — see "Deferred design decisions" below)

Every fix was written test-first per CLAUDE.md's hypothesis → test →
implementation loop: a regression test was added, confirmed to **FAIL**
against the pre-fix code (verified directly by temporarily reverting the
fix in-tree, not assumed), then confirmed to **PASS** once the fix landed.

**A self-correction happened mid-pass, exactly because this round's full
gate battery was run and taken seriously.** The first WR-03/WR-04 commit
applied `fitPane` at the Identities screen's single shared render point —
matching the review's own suggested fix verbatim — which also reshaped an
unrelated pane (`paneDelete`'s "delete everything" ceremony, which carries
its own independently-scrollable preview viewport) under long-`$HOME`-path
conditions, breaking 14 `internal/screenshot` visual-regression negative-
control tests. A follow-up commit narrowed the fix to exactly the
demonstrated defect (`paneKeyCeremony`), verified clean against the full
gate battery again, and documents the reasoning in both the commit message
and the code comment. This is recorded so it is visible, not silently
folded into a single "clean" commit.

**Full gate battery, run against the final commit (`0204678`):**

```
go build ./...                                                          # clean, exit 0
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...                   # 2384 passed, 22 packages, exit 0
make lint                                                               # golangci-lint: 0 issues (untagged + screenshot-tagged)
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -count=1 -timeout 40m ./e2e/...
                                                                         # 175 passed, 0 failed, exit 0 (full clean run,
                                                                         #   captured against the pre-narrowing WR-03/WR-04
                                                                         #   commit; re-verified after the narrowing commit
                                                                         #   via targeted re-runs covering the identity-
                                                                         #   manager, upload, create-flow, key-ceremony,
                                                                         #   rotate-delete-offer, health/fixer, git-
                                                                         #   configuration, and CLI test groups — no
                                                                         #   failures, only this fixer's own too-tight
                                                                         #   -timeout on unrelated screens during manual
                                                                         #   spot-checks, never a real assertion failure)
TERM=dumb SSH_AUTH_SOCK= go test -tags screenshot -count=1 -timeout 30m \
    ./internal/screenshot/... ./cmd/gitid/...                          # 833 passed, 1 failed, 1 skipped, exit 1
                                                                         #   TestCaptureTUI: "freeze binary not found on
                                                                         #   PATH" — environmental, unchanged from the
                                                                         #   review's own baseline (832/1 before this pass;
                                                                         #   +1 from this pass's own new coverage)
```

## Fixed Issues

### CR-01: D-17's post-upload confirmation result is never rendered

**Files modified:** `cmd/gitid/upload_run.go`, `internal/tuikit/design.go`,
`internal/tuikit/identities.go`, `internal/tuikit/views.go`,
`cmd/gitid/upload_run_test.go`, `cmd/gitid/wiring_test.go`,
`internal/tuikit/upload_copy_test.go`, `internal/tuikit/upload_section_test.go`
**Commit:** `e254070`
**Applied fix:** `confirmUpload`'s D-17 confirmation note
(`uploadUnconfirmedReasonFmt`, previously a `cmd/gitid`-local constant no
renderer ever read) is now `tuikit.UploadUnconfirmedReasonFmt`, moved beside
the other frozen `Upload*` copy in `design.go` and added to
`upload_copy_test.go`'s byte-exact contract table. Both renderers
(`renderUploadSection` in the TUI, `printUploadOutcome` on the CLI) now
render `row.Reason` as an extra continuation row under the row's own
success line when set on a non-failed (`UploadRowUploaded`/
`UploadRowAlreadyPresent`) outcome — never in place of it, and never for the
common confirmed case (no continuation row when `Reason` is empty).
Regression coverage: TUI (`TestUploadResultRowsRenderConfirmationReason`,
`TestUploadResultRowsOmitReasonRowWhenConfirmed`), CLI (extended
`TestPrintUploadOutcomeRendersEachSection`), and an end-to-end test driving
the real still-missing-after-retry scenario through `confirmUpload` then
`printUploadOutcome`, asserting the **rendered text** (not just the struct
field) carries the phrase
(`TestPostUploadConfirmationUnconfirmedReasonIsActuallyRendered`) — the
assertion that would have caught this across three prior review rounds.

### WR-01: `RotateDeleteCommitMsg` had no stale-guard

**Files modified:** `internal/tuikit/views.go`, `internal/tuikit/identities.go`,
`cmd/gitid/wiring.go`, `internal/dummytui/fixturebackend.go`,
`internal/tuikit/backend_stub_test.go`,
`internal/tuikit/identity_manager_upload_test.go`,
`e2e/identity_manager_pty_e2e_test.go`
**Commit:** `5a77e08`
**Applied fix:** added `Name` to `RotateDeleteCommitMsg`, guarded the
consumer on `commit.Name == m.selected`, and set `Name` at every producer
(`realBackend.CommitRotateDeleteOldKey`'s five return sites,
`FixtureBackend.CommitRotateDeleteOldKey`, and `stubBackend.
CommitRotateDeleteOldKey` — the same fixture-producer gap the prior fix pass
left behind for `UploadRunMsg`, closed in the same commit here so it cannot
recur). Regression coverage: `TestRotateDeleteCommitMsgStaleReplyDiscarded`
drives a genuine delete dispatch, feeds a reply named for a different
identity, and asserts nothing on the model changed (verified RED against
the pre-fix guard by temporarily reverting it and re-running the test). The
dummy leg of `TestRegisterKeyModal_CompiledRealVsLiveDummyPTY`'s
rotate-delete-offer subtest now also drives the delete choice to completion
and asserts the removal result row renders, per the review's requested
anti-drift guard.

### WR-02: dummy backend fabricated GitHub success for provider-less identities

**Files modified:** `internal/dummytui/fixturebackend.go`,
`internal/dummytui/fixturebackend_test.go`
**Commit:** `359f6b0`
**Applied fix:** `FixtureBackend.RunUploadForIdentity` now mirrors
`RegisterKeyPlan`'s three-way branch (gitlab / github / default) instead of
falling through to the GitHub success shape for anything that isn't gitlab
— the default case now returns `Skipped: true` with zero rows, matching
`planUpload`'s real D-13 gate for `provider == ""`. Regression coverage:
`TestRunUploadForIdentityOmitsForNonGatedHost` asserts both provider-less
fixture identities (`opensource`, `archived`) return `Skipped: true` with no
fabricated rows.

### WR-03: key-ceremony overflow backstop overran the frame at `budget <= 2`

**Files modified:** `internal/tuikit/identities.go`,
`internal/tuikit/identity_manager_upload_test.go`
**Commits:** `29ec14f` (initial fix), `0204678` (scope correction found by
this round's own full-gate mandate — see the self-correction note above)
**Applied fix:** the final state adds a `fitPane(lipgloss.NewStyle().
Width(detailWidth).Render(...), frameBodyRows(height))` wrap scoped to
`case paneKeyCeremony:` in `identitiesModel.view()` — a true outer safety
net for the cases `renderKeyCeremony`'s own local budget math cannot cover
on its own (`budget <= 2`, and the `tail.Len() == 0` base case, which
previously returned the receipt completely unbounded). Scoped to
`paneKeyCeremony` specifically (not every pane state this shared `view()`
renders), after full-gate verification showed a screen-wide wrap also
reshaped `paneDelete`'s unrelated, independently-scrollable ceremony
preview under long-path conditions. Regression coverage:
`TestKeyCeremonyOverflowBackstopClampsWholePaneWhenReceiptAloneOverflows`
forces the ceremony's own receipt (via direct state injection — 6 long
`Targets`/`Backups` entries) to 55 rendered lines, well past
`frameBodyRows(30)`, and asserts the top-level `view()`'s rendered body
stays within budget — verified RED (55 > budget) before the fix.

### WR-04: overflow backstop misused a scrollable viewport for a one-shot clip

**Files modified:** `internal/tuikit/identities.go`
**Commit:** `29ec14f`
**Applied fix:** replaced `ExactTextViewport` (a scrollable, focusable
component never wired to any key handler in this pane) with `fitPane` —
this file's existing helper for a non-scrollable, visible-cue clip —
in `renderKeyCeremony`'s local tail overflow branch. This removes the
double/disagreeing truncation cue (the viewport's own "PgDn/PgUp" cue,
which does nothing here, stacked on the function's own "N more line(s)
hidden" cue), the off-by-one hidden-line count, and the silent horizontal
truncation (the viewport sliced 4 columns off every clipped line with its
own horizontal cue suppressed) in one change. The sibling backstop in
`renderUploadSection` (`identities.go:4784-4788`, cited by the review as
sharing the same three defects) was deliberately **not** touched in this
pass — it is covered by frozen, passing tests
(`TestUploadSectionClampsFallbackEvenWhenBudgetIsNegative` asserts on its
specific `"more line(s) hidden"` cue text) that a `fitPane` swap there would
break, and unifying the two backstops into one shared helper is IN-24
(Info-tier, filed by the review as future cleanup, out of this round's
`critical_warning` scope). Regression coverage: the existing
`TestKeyCeremonyOverflowBackstopStaysWithinFrameBudget` (WR-01, iteration 4)
continues to pass unchanged, confirming the swap stays within budget for
the positive-budget case it covers.

### WR-05 (sub-defect only — the design decision itself remains open, see below)

**Files modified:** `internal/tuikit/identities.go`,
`internal/tuikit/identity_manager_upload_test.go`
**Commit:** `c41ee3b`
**Applied fix:** `handleRegisterKeyKey`'s `esc` case now returns
`registerKeyAbandonNote(pending, name)` — `""` unless a registration beat
was genuinely in flight at the moment of Esc, otherwise a note naming the
identity, mirroring the wizard's existing `wizardAbandonUploadNote`
(WR-13, iteration 3) mitigation for the byte-for-byte identical situation.
Regression coverage: `TestRegisterKeyPaneEscWhilePendingSurfacesAbandonNote`
drives to the Ready-plan-dispatched-upload state and asserts Esc surfaces a
note naming the identity (verified RED against the pre-fix esc handler);
`TestRegisterKeyPaneEscWithoutPendingBeatSurfacesNoNote` is the negative
control proving the common case (nothing in flight) stays silent.

## Deferred design decisions

These are **not** skipped due to fixer failure — each is a documented,
intentional product/design decision the review itself classified as
requiring a frozen-artifact amendment (`FIELDS.md`, `design.go`'s R22
copy), not a mechanical code fix. Re-verified in source this pass; nothing
changed since the prior fixer round's assessment.

### WR-05 (design decision half): D-08 register-key pane mutates the provider account with no confirm step

**File:** `internal/tuikit/identities.go:2669-2677`,
`.planning/design/identity-manager/FIELDS.md:87`
**Reason:** `FIELDS.md:87` records "opening the modal IS the explicit
opt-in" as an intentional contract, restated in
`TestIdentityManager_RegisterKeyModalRuns`'s doc comment. Changing it means
amending `FIELDS.md`, `design.go`'s frozen copy, the visual-divergence
allowlist, and at least three real-binary e2e tests. Does not violate
CLAUDE.md's literal confirmation rule (scoped to `~/.ssh/config`/
`~/.gitconfig`). Route through `/gsd-discuss-phase` or a tracked design
amendment — not blocking Phase 9's own goal.

### WR-06: the multi-line `ManualCommand` is interpolated into a single-line sentence

**File:** `cmd/gitid/upload_run.go:552-558`, `internal/tuikit/design.go:695`,
`internal/tuikit/identities.go:2812`, `:2821`
**Reason:** the fix is either an R22 frozen-copy amendment in `design.go`
(render the commands as an indented block) or a producer change to join
with `" && "` — both deliberate copy/rendering decisions requiring review,
not a mechanical edit. File as a tracked design/backlog item.

### WR-07: the upload checkbox's actionable copy is truncated at the only width production uses

**File:** `internal/tuikit/identities.go:4820-4828`, `:4835`,
`internal/tuikit/design.go:582`, `:586`
**Reason:** shortening the frozen unauth/disabled labels is an R22
copywriting decision; widening the row breaks the wizard's one-line row
budget. File as a ROADMAP/backlog item.

---

_Fixed: 2026-08-31T05:39:05Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 3 (final automated round)_
