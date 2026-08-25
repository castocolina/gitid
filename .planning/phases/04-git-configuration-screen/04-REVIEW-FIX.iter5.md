---
phase: 04-git-configuration-screen
fixed_at: 2026-08-25T05:22:00Z
review_path: .planning/phases/04-git-configuration-screen/04-REVIEW.md
iteration: 3
findings_in_scope: 14
fixed: 7
skipped: 7
status: partial
---

# Phase 4: Code Review Fix Report (iteration 3)

**Fixed at:** 2026-08-25T05:22:00Z
**Source review:** `.planning/phases/04-git-configuration-screen/04-REVIEW.md`
**Iteration:** 3

**Summary:**
- Findings in scope (REVIEW.md, `critical_warning`): 14 (2 critical, 12 warning)
- Fixed: 7 — WR-28, CR-08, CR-09, WR-24, WR-26, WR-27 (the orchestrator's
  explicit directed list, in the specified order), plus WR-29 as a direct,
  unavoidable side effect of CR-08 actually working (the old vacuous e2e
  assertion had to change or `make test-e2e` would go red)
- Skipped: 7 — WR-16, WR-25, WR-30, WR-31, WR-32, WR-33, WR-34 (explicitly
  out of scope for this pass; see "Skipped Issues" below)
- Status: partial (by design — this pass was scoped narrowly and
  deliberately, not by running out of time or ability)

**A note on confidence, per the orchestrator's explicit request.** This is the
fourth attempt at fixing Force-SSH-related code in this exact
`internal/tuikit`/`internal/screenshot` area (CR-04 → CR-06 → CR-08 →
this pass), and the review's own framing warns that the same kind of agent
that produced each prior regression is the one fixing it now. I traced every
fix's actual runtime path by hand rather than trusting a green test alone,
and in two cases (CR-08's render change, WR-26's window fix) my OWN first
attempt introduced a new bug that I caught only by re-running the full
suite before committing — documented per-finding below. I am reasonably
confident in all six fixed findings, backed by tests that drive the real
dispatch path (`tea.MouseClickMsg`/`tea.KeyMsg` through `App.Update()`, real
PTY e2e, and `make gate-visual-regression` against the real+dummy backends
rather than synthetic fixtures alone). I flag the one place I am NOT fully
confident (WR-27's `RegionConfirmationPreview` gap) explicitly rather than
rounding it up to "done."

## Fixed Issues

### WR-28: `internal/screenshot` was outside every automated gate

**Files modified:** `Makefile`, `cmd/gitid/smoke_network_test.go`,
`internal/screenshot/createflow.go`, `internal/screenshot/createflow_packet.go`,
`internal/screenshot/createflow_packet_test.go`, `internal/screenshot/createflow_test.go`
**Commits:** `4bf7dc7`, `f4e2578`

**Applied fix:** Added a `lint-screenshot` Make target (`go vet -tags
screenshot ./...` + `golangci-lint run --build-tags screenshot
./internal/screenshot/...`) that `lint` depends on, and appended `go test
-tags screenshot -skip 'TestCaptureTUI|TestCaptureHTML|TestProvisionPinnedChromium'
./internal/screenshot/...` to the `test` target. Wiring the gate surfaced a
real pre-existing compile break (`cmd/gitid/smoke_network_test.go` called
`tester.PreWrite` with a stale 3-arg signature — never compiled by any gate
since `smoke` is deliberately excluded from CI) and ~10 pre-existing
golangci-lint findings plus 10 pre-existing test failures in
`internal/screenshot` (mostly the Phase-4 merged-registry gap: several
tests built their live/approved capture maps from `CaptureCreateFlowScreens`
alone, but `RequiredScreenSpecs()` is the merged create-flow + git-screen
registry). All of these had to be fixed for the new gate to be meaningfully
green — documented in commit `4bf7dc7`.

**Deviation from the review's literal suggestion, and why:** the review
suggested wiring `go test -tags screenshot` directly into `lint-screenshot`
(which `lint` depends on). My first attempt did exactly that, and it made
`make lint` — the pre-commit hook's own gate — take ~74s instead of ~3s
(the screenshot package's test suite takes ~65s). That would make every
future commit's pre-commit hook noticeably slower and risks hook timeouts
(this literally broke two of my own commit attempts mid-session — see
`git log` message on `f4e2578`). Follow-up commit `f4e2578` moves the `go
test` execution to the `test` target instead (pre-push stage, per
`.pre-commit-config.yaml`), keeping `lint`/`lint-screenshot` to vet+lint
only (~3s). This still satisfies the review's own acceptance criterion —
"execute somewhere in `make lint` or `make test`" — verbatim.

**Verification:** `make lint` completes in ~3s and is 0 issues; `make test`
runs the full race suite then the screenshot package's 100+ tests and
passes; `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` passes
(1054 tests, 19 packages, run repeatedly throughout this session).

### CR-08: Force SSH is unreachable in the Configure-Git pane

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`,
`e2e/git_configuration_pty_e2e_test.go`
**Commit:** `d97985e`

**Applied fix:**
- `paneGitFocusOrder` (a new explicit-slice-position ring, mirroring the
  wizard's existing `wizardGitFocusOrder`) replaces the pane's raw `%
  gitPaneFocusRing` modulo Tab/Shift+Tab arithmetic, so `gitFieldForceSSH`
  is now a real Tab stop in the pane, not just the wizard.
- `gitForm.view()`: Force SSH now LEADS its rendered row (checkbox first,
  descriptive text after) instead of trailing a helper line, so
  `anchoredLabelMatch`'s row-start anchoring can actually hit-test it. This
  fixes the click path in BOTH the pane and the wizard, since `gitForm.view()`
  is shared.

**Deviation from the review's illustrative fix, and why (self-caught
regression):** the review's example fix added a NEW row for the Force SSH
checkbox. My first attempt did that — and it broke
`TestWizardGitStepButtonsAreFocusable` and two other tests, because the
pane/wizard body is budgeted against a fixed 30-row frame (an existing,
explicitly documented constraint — see `sshForm.view`'s own comment on the
same tradeoff) and the extra row pushed the frozen "Continue reviews the
Git fragment..." hint text below the visible frame. I caught this by
running the full `internal/tuikit` suite before committing, not by
trusting the isolated new tests. The final fix reorders Force SSH onto the
SAME row instead (checkbox leads, description follows) — zero added rows,
same anchoring fix.

**Tests added:** `TestGitFlowPaneTabRingVisitsForceSSH` and
`TestGitFlowPaneMouseClickThenSpaceTogglesForceSSH` drive the real
dispatch path (`App.Update()` with real `tea.MouseClickMsg`/`tea.KeyMsg`)
end to end and assert `gitPaneForm.forceSSH` actually flips — per the
review's own explicit note that a `gitFormFieldSlots` membership assertion
cannot catch this class of regression (it passed throughout CR-04/CR-06/CR-08).

**WR-29 fixed as a direct consequence (not independently scoped this
pass):** `e2e/git_configuration_pty_e2e_test.go`'s
`TestGitConfiguration_RealPTYMouseFieldFocus` asserted `"☐ Force SSH"`
after a click+space — a state that already held before any interaction
(the seeded identity's real ForceSSH state is false, per CR-09), so it
passed whether or not the click/space did anything. Once CR-08 made the
control actually reachable, this assertion started FAILING on my own
change (the checkbox now genuinely flips to `☑`) — proving CR-08 works,
and forcing the fix. Now asserts the real `☐ → ☑ → ☐` round trip.

**Verification:** `go test ./internal/tuikit/...` (256 tests), `go test
-tags e2e -race -timeout 350s ./e2e/...` (42 tests, run twice — once before
CR-09, once after), `make gate-visual-regression` — all green.

### CR-09: `ForceSSH` was never loaded from disk

**Files modified:** `cmd/gitid/wiring.go`, `internal/gitconfig/renderer.go`,
`internal/identity/identity.go`, `internal/identity/loader.go`,
`internal/identity/loader_test.go`, `internal/tuikit/identities.go`,
`internal/tuikit/identities_test.go`
**Commit:** `8ed0292`

**Applied fix:**
- New `gitconfig.HasProviderRewrite(gcBytes, provider)`: checks whether
  `WriteProviderRewrite`'s own managed block name
  (`provider-rewrite:<host>`) is present via `filewriter.ListBlocks`, so it
  can never drift from what the write path actually produces.
- `identity.Account` gains `ForceSSH`, populated during `Reconstruct` from
  the gitconfig bytes it already parses (no extra file read).
- `cmd/gitid/wiring.go` `toDemoIdentity`: straight passthrough of
  `acct.ForceSSH`.
- `identities.go` `gitCeremonyFor`'s WR-05 note is now additionally gated
  on `sel.ForceSSH` (the real, original disk state) — otherwise the note
  would fire for every identity that simply never had a rewrite block,
  since a fresh identity's checkbox also defaults to `true` via `sel.ForceSSH
  || !m.gitExisting`.

**A real bug I found and fixed while implementing this (not in the
original review text):** the managed block `WriteProviderRewrite` writes is
keyed by an FQDN-shaped provider host (e.g. `"github.com"`), but
`identity.Account.Provider` can ALSO be the short `hostnameToProvider` form
(`"github"`, no dot) when no `"# gitid: provider="` marker is present on
the SSH host block — and that short form fails `gitconfig`'s hostname
validation outright, which would have silently made `HasProviderRewrite`
return an error (swallowed, defaulting to `ForceSSH: false`) for every
identity reconstructed via the hostname-map fallback rather than an
explicit marker. Added `rewriteLookupProvider`, mirroring
`internal/tuikit`'s own `providerFromSSHHost` fallback, so the lookup key
always matches what the write path actually used regardless of which
provider form `acct.Provider` holds.

**Tests added:** `TestReconstruct_LoadProviderRewrite` (extended to assert
`ForceSSH == true`), `TestReconstruct_ForceSSHFalseWithoutRewriteBlock`
(negative counterpart), `TestGitCeremonyOmitsSharedProviderRewriteNoteWhenBlockNeverExisted`
(tuikit-level negative counterpart to the existing
`TestGitCeremonyNotesSharedProviderRewriteWhenForceSSHOff`). Both loader
tests exercise the SHORT-form provider path (no explicit marker,
`hostnameToProvider` fallback) — the trickiest case, and the one that would
have silently broken without `rewriteLookupProvider`.

**Verification:** `go test ./internal/identity/... ./internal/gitconfig/...
./internal/tuikit/... ./cmd/gitid/...` (650 tests), `go test -tags e2e -race
-timeout 350s ./e2e/...` (42 tests), `make gate-visual-regression` — all
green.

### WR-24: standalone Configure-Git commit path never hardened `~/.ssh`

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `d2ce5c5`

**Applied fix:** `commitGitArtifacts` now calls `journal.ensureManagedDir(b.sshDir,
sshDirMode)` (a new `"ssh-dir"` fault-injection boundary) right before the
allowed-signers write, mirroring `createStagedKey`'s own placement of the
same call on the CREATE path. Pulled `filepath.Dir(b.allowedSigners)`
(== `b.sshDir`) out of the early watch-only loop entirely.

**Tests added:** `TestCommitGitTransactionHardensPreExistingSSHDir` (mirrors
the existing `TestCommitCreateHardensPreExistingSSHDir` for the standalone
path), `TestCommitGitArtifactsCreatesAbsentSSHDir` (proves the
from-scratch-HOME failure mode from the review's own PROBE is fixed — calls
`commitGitArtifacts` directly with an explicit `pubLine` so the test
isolates `~/.ssh` absence from the unrelated concern of where the signing
key lives). `TestGitTransactionRollbackMatrixPreservesSnapshotsAndSafetyBackups`'s
fault-injection matrix gained the new `"ssh-dir"` boundary.

**Verification:** `go test -run 'TestCommitGit|TestGitTransactionRollbackMatrix|
TestCombinedTransaction|TestCommitCreateHardensPreExistingSSHDir' ./cmd/gitid/...`
(33 tests), full suite, `make gate-visual-regression` — all green.

### WR-26: `extractGitCeremony`'s sliding window started one row early

**Files modified:** `internal/screenshot/createflow_regions.go`,
`internal/screenshot/createflow_test.go`
**Commit:** `69d8d27`

**Applied fix:** Check row `i` alone FIRST (resolving `start` correctly for
the common unwrapped case); only fall through to the 2-row window
(`rp[i]+" "+rp[i+1]`) when the phrase is not already on row `i` alone.

**A second-order version of the same bug I found while fixing the first
one:** a naive "check row `i` alone, else check the window" still
false-positives when row `i+1` ALONE already contains the full marker (row
`i` is unrelated content, row `i+1` is the complete heading) — the
concatenated window contains the marker as a substring even though nothing
spans the boundary, reintroducing the exact off-by-one one iteration later.
I caught this because my own first version of the fix failed my own new
negative test. Fixed by skipping the window match when row `i+1` alone
already satisfies the marker (that row's own iteration will set `start`
correctly).

**Test added:** `TestExtractRegion_GitCeremonyStartsExactlyOnTheHeadingRowNotOneEarly`
— the review's own probe shape (an unrelated row directly above the
heading), asserting the region does not absorb it.

**Verification:** `go test -tags screenshot -run TestExtractRegion_GitCeremony
-v ./internal/screenshot/...` (4 tests), full screenshot suite (101 tests),
`make gate-visual-regression` — all green.

### WR-27: WR-19's predicate gating had zero production call sites

**Files modified:** `internal/screenshot/createflow.go`,
`internal/screenshot/createflow_packet_test.go`
**Commit:** `00dde23`

**Applied fix:** Converted the six git-screen dispositions whose divergence
text is already documented in
`.planning/design/git-screen/visual-divergence-allowlist.txt` to
`uxRegionDifferenceScoped`, porting the SAME predicate strings that file's
schema already enforces for the e2e gate's identical checkpoint/region
pairs: `fixtureSidebarDisposition` (`absent:"clientB"`),
`fixtureHeaderStatusDisposition` (`contains:"ids"`), `gitPreviewDisposition`
(`absent:"gitdir:~/git/"`), `formFieldsDisposition` (`absent:"User"`), the
review-readonly `git-ceremony` sentinel-wrapped-preview disposition
(`absent:"BEGIN gitid managed"`), and the result-success `git-ceremony`
backup-receipt-completeness disposition (`absent:".bak."`).

**Verified against real captures, not just synthetic fixtures:** ran `make
gate-visual-regression` (drives both the real `cmd/gitid` `Backend` and
`cmd/gitid-dummy`'s `FixtureBackend` in-process) after wiring the six
predicates, to confirm they hold against actual rendered text, not just my
guessed strings — it still passes.

**Explicitly NOT fully closed — read this before treating WR-27 as
done:** the `RegionConfirmationPreview` sentinel-wrapped-preview
disposition on `review-readonly` (a near-duplicate of the `RegionGitCeremony`
one, per its own code comment: "Same divergence, same reason as
RegionGitCeremony above") is left BLANKET/unscoped. The allowlist file's
own schema does not list `"confirmation-preview"` as a valid region at all
— only `"git-ceremony"` is — so there is no allowlist-sourced predicate
string to port without independently deriving and verifying one, which was
outside this pass's time budget. This is a genuine, real, tracked gap, not
a rounding-up. A future pass should either add a `confirmation-preview`
entry to the allowlist schema first (so both gates share one source of
truth for it) or independently verify a predicate against real captures
before scoping it.

**Tests added:** `TestBuildRegionDiffsAcceptsDivergenceSatisfyingScopedPredicate`
(a scoped predicate still accepts the divergence it's meant to authorize)
and `TestBuildRegionDiffsRejectsDivergenceViolatingScopedPredicate` — the
review's explicit requirement: a mutation the predicate is supposed to
catch now produces a real `BuildRegionDiffs` error, not a silent pass.

**Verification:** `go test -tags screenshot -run TestBuildRegionDiffs -v
./internal/screenshot/...` (6 tests), full screenshot suite (103 tests),
`make gate-visual-regression` — all green.

## Skipped Issues

All seven skips below are **out-of-scope-for-this-pass**, not
could-not-fix. The orchestrator's dispatch prompt for this iteration named
an explicit, ordered fix list (WR-28 first, then CR-08, CR-09, WR-24,
WR-26, WR-27) and asked for unusual care given three consecutive
regressions in this exact focus/enum area (CR-04 → CR-06 → CR-08). None of
the findings below were in that list, and none of them are prerequisites
for the six that were. They remain open in `04-REVIEW.md` for a future,
separately scoped pass.

### WR-16: managed-directory creation not disclosed in the ceremony

**File:** `internal/tuikit/identities.go:2026`, `cmd/gitid/wiring.go:1115-1125`
**Reason:** The review itself explicitly recommends deferring this one to a
design pass (agrees with the prior fixer's skip) — adding even the
unconditional disclosure line changes golden frames on multiple screens
`make gate-visual-regression` pins, which needs a deliberate review pass,
not a mechanical fix folded into this one. Not in this pass's directed list.

### WR-25: rollback re-loosens a hardened managed root even when file restoration failed

**File:** `cmd/gitid/wiring.go:964-1015` (`restore()`)
**Reason:** Not in this pass's directed list. A narrow, security-relevant
fix (skip the mode revert for a managed root when a file under it failed to
restore) that deserves its own dedicated verification pass rather than
being folded in alongside six other changes in the same fragile area.

### WR-30: `ConfigureGit.Name` and its note still read `m.selected` live

**File:** `internal/tuikit/identities.go:1716-1721`
**Reason:** Not in this pass's directed list. The review itself notes this
is latent (the `gitCommitPending` early return already prevents `m.selected`
from moving today), lowering urgency relative to the six directed findings.

### WR-31: match-strategy copy contradicts the gitdir default it describes

**File:** `internal/tuikit/identities.go:613-621`, `:696-701`
**Reason:** Not in this pass's directed list. Touches `strategyCopy`'s
signature (adds a `gitDir` parameter) and the dummy/fixture backend's
`IncludeIfPreview`, which is a wider blast radius than this pass's six
targeted fixes.

### WR-32: Enter on the wizard's Force-SSH row opens the write ceremony instead of toggling

**File:** `internal/tuikit/identities.go:2499-2518`, `:785-788`
**Reason:** Not in this pass's directed list — and notably, the SAME
"Enter always falls through to the primary action" pattern exists in the
CONFIGURE-GIT PANE's `handleGitKey` too (not just the wizard the review
names), which I noticed while implementing CR-08 but did not fix, since it
was not in the directed list and this is precisely the kind of
adjacent-but-unscoped change the orchestrator asked me to avoid this pass.
Flagging it here so it is not lost: fixing WR-32 should also cover the pane,
not just the wizard.

### WR-33: stale cross-reference to a deleted symbol in a comment

**File:** `e2e/git_configuration_pty_e2e_test.go:715`
**Reason:** Not in this pass's directed list. Cosmetic (comment-only), zero
runtime risk either way; genuinely lowest priority of the remaining items.

### WR-34: `displayMessage`'s substring replace is fragile at both ends

**File:** `cmd/gitid/wiring.go:1985-1990`
**Reason:** Not in this pass's directed list. Needs a symlinked-HOME test
fixture and a prefix-sharing-sibling-directory fixture to verify correctly
— a properly scoped fix in its own right, not something to fold in
alongside six other changes.

## Full Verification Run (this pass, on the final commit `00dde23`)

- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — 1054 tests, 19
  packages, all pass.
- `make lint` — 0 issues, ~3s (now actually exercises `-tags screenshot`
  via `go vet` + scoped `golangci-lint`, per WR-28).
- `make test` — full race suite (all packages) + `internal/screenshot`'s
  100+ test suite under `-tags screenshot`, all pass.
- `make test-e2e` — 42 tests (`-tags e2e -race -timeout 350s`), all pass
  (run twice during this session: once after CR-08, once after CR-09).
- `make gate-visual-regression` — `TestGateVisualRegression` +
  `TestGateVisualRegressionReadOnly` + `TestAllScreensCapturedAndNonEmpty` +
  `TestNegativeControls_AllProtectedRegionsDetectMutation` +
  `TestNegativeControl_MissingGitScreenState` +
  `TestNegativeControl_StaleGitScreenClassification` +
  `TestNegativeControl_CrossRegistryLeakage` all pass;
  `TestApprovalCommitRecorded` SKIPs on `.git/HEAD` (an artifact of running
  inside a git worktree, not a regression — `.git` is a file, not a
  directory, in a worktree checkout).

## Commits (this pass, in order)

- `4bf7dc7` fix(04): WR-28 wire the screenshot build tag into make lint
- `f4e2578` fix(04): WR-28 follow-up — keep make lint fast, run screenshot tests at make test
- `d97985e` fix(04): CR-08 make Force SSH reachable in the configure-Git pane
- `8ed0292` fix(04): CR-09 populate ForceSSH from real on-disk gitconfig state
- `d2ce5c5` fix(04): WR-24 harden ~/.ssh on the standalone Configure-Git commit path
- `69d8d27` fix(04): WR-26 fix extractGitCeremony's sliding-window off-by-one
- `00dde23` fix(04): WR-27 wire real predicates onto the git-screen dispositions the allowlist already documents

---

_Fixed: 2026-08-25T05:22:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 3_
