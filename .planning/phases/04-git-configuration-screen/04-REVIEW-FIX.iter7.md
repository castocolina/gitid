---
phase: 04-git-configuration-screen
fixed_at: 2026-08-25T06:51:30Z
review_path: .planning/phases/04-git-configuration-screen/04-REVIEW.md
iteration: 4
findings_in_scope: 16
fixed: 9
skipped: 7
status: partial
---

# Phase 4: Code Review Fix Report (iteration 4)

**Fixed at:** 2026-08-25T06:51:30Z
**Source review:** `.planning/phases/04-git-configuration-screen/04-REVIEW.md`
**Iteration:** 4

**Summary:**
- Findings in scope (REVIEW.md, `critical_warning`): 16 — CR-10, CR-11, CR-12,
  CR-13, CR-14, BL-15 (5 CR-prefixed critical findings + 1 BLOCKER-severity
  security finding the frontmatter's `critical: 5` count does not separately
  tally), plus WR-35 through WR-44 (10 warnings)
- Fixed: 9 — CR-13, CR-10, CR-11, CR-12, CR-14, BL-15 (the orchestrator's
  explicit directed order), plus WR-35, WR-36, WR-37 (also explicitly
  directed: "test hermeticity leak" and "dead code / stale comments")
- Skipped: 7 — WR-38 through WR-44 (not part of the orchestrator's explicit
  task list for this pass; see "Skipped Issues" below)
- Status: partial (by design — the orchestrator's task narrowed this pass to
  a specific, explicit list of findings, not the full `critical_warning`
  scope)

**Isolation.** All work happened in an isolated git worktree
(`gsd-reviewfix/04-47343`, branched from `gsd/phase-04-git-configuration-screen`
at `00dde23`) per `workflow.use_worktrees=true`. Every commit hash below is
stable across the cleanup tail's fast-forward (fast-forward never rewrites
commits, only advances the branch pointer), so they resolve identically once
merged into the main checkout.

**Verification discipline.** For every fix below I manually traced the
runtime behavior (not just a new test), and for 5 of the 9 — CR-10, CR-11,
CR-12, CR-14, BL-15, WR-35 — I ran an explicit red-before-fix / green-after-fix
check: reverted the fix in place, re-ran the new regression test to confirm it
actually fails without the fix, then restored the fix and confirmed it passes.
This directly answers the orchestrator's "could this test pass even if the fix
were wrong?" instruction — evidence is quoted per finding below. CR-14's fix
also had a second-order consequence (the CTX-D-02 `git-preview` divergence it
used to authorize converged away entirely, requiring a disposition
reclassification) and surfaced a stale Phase-3 e2e assertion pinned to the old,
wrong dummy fixture shape — both traced and fixed, documented under CR-14.

Gates run at the END of the full pass (after all 9 fixes), in the isolated
worktree:

| Gate | Result |
|---|---|
| `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` | PASS (18 packages) |
| `make lint` | PASS, 0 issues |
| `make test` | PASS |
| `make test-e2e` (full suite, ~256s) | PASS |
| `make gate-visual-regression` | PASS (9 tests, 1 environment-only SKIP — see note) |

Note: `TestApprovalCommitRecorded` SKIPs with `cannot read .git/HEAD: open
../../.git/HEAD: not a directory` — this is an artifact of running inside a
git *worktree* (where `.git` is a file pointing at the real gitdir, not a
directory) and is expected to PASS once merged into the main checkout, where
`.git` is a real directory. Not a defect introduced by this pass.

## Fixed Issues

### CR-13: WR-28 closed the `screenshot` blindspot, left `smoke` open

**Files modified:** `Makefile`
**Commit:** `5a6051d`
**Applied fix:** Renamed `lint-screenshot` to `lint-tagged` and widened it to
run `go vet -tags <tag> ./...` for every isolated build tag in the tree
(`screenshot`, `smoke`, `e2e`), plus a grep-based guard that fails the moment a
new `//go:build <tag>` line appears anywhere without a matching `go vet -tags
<tag>` line in this target — so the exact blindspot WR-28 fixed for
`screenshot` cannot silently reopen for a future tag the way it did for
`smoke`. `lint` now depends on `lint-tagged`.
**Verification:** Reproduced the reviewer's exact probe — injected
`func smokeProbeBreak() { var x int = "nope"; _ = x }` into
`cmd/gitid/smoke_network_test.go` — and confirmed `make lint-tagged` now
fails with `vet: cmd/gitid/smoke_network_test.go:75:38: cannot use "nope"...`.
Reverted the injection (file restored, no diff). Grepped every `//go:build`
tag across the tree (excluding `.planning/`) and confirmed only
`screenshot`/`smoke`/`e2e` exist, all now gated.

### CR-10: `regionPredicateSatisfied`/`gitScreenPredicateSatisfied`'s `absent:`/`contains:` grammar was vacuous

**Files modified:** `internal/screenshot/createflow.go`,
`internal/screenshot/region_disposition_test.go`,
`e2e/git_configuration_pty_e2e_test.go`
**Commit:** `495d2d7`
**Applied fix:** Changed both predicate-evaluation functions from "holds on
EITHER side" (`||`) to the shape-correct grammar: `contains:X` now requires
X on **both** sides; `absent:X` now requires the presence/absence
**asymmetry** itself (`!=`, i.e. exactly one side carries X — both-present
AND both-absent are now rejected). Rewrote
`TestRegionPredicateSatisfiedMatchesEitherSide` (renamed
`TestRegionPredicateSatisfiedRejectsSymmetricCases`) to cover the fixed
grammar, and added `TestBuildRegionDiffsRejectsUnrelatedLiveRegressionUnderProductionPredicate`,
which extracts the REAL, shipped `fixtureSidebarDisposition` from
`gitScreenSpecs()` (not a hand-typed predicate copy) and reproduces the
review's exact probe fixture.
**Verification:** Red-before-fix: reverted the grammar to the old `||`/`!...||!...`
form and re-ran the new regression test — it failed with `CR-10 regression:
BuildRegionDiffs accepted an unrelated live-side regression under the
production gitPreviewDisposition predicate — the predicate is vacuous again`.
Restored the fix, test passes. All pre-existing predicate tests
(`TestBuildRegionDiffsAcceptsDivergenceSatisfyingScopedPredicate`,
`...Rejects...`) still pass under the new grammar since every shipped
predicate authorizes a genuinely asymmetric divergence.

### CR-11: Negative controls mutated one metadata field on one region and stopped

**Files modified:** `cmd/gitid/gate_visual_regression_test.go`, `Makefile`
**Commit:** `965b0ca`
**Applied fix:** Renamed `TestNegativeControls_AllProtectedRegionsDetectMutation`
to `TestNegativeControl_UnclassifiedDifferenceRejected` (kept as the narrow
classification-requirement check it actually is) and
`TestNegativeControl_StaleGitScreenClassification` to
`TestNegativeControl_GitScreenUnclassifiedDifferenceRejected` (same). Added
two NEW exhaustive tests —
`TestNegativeControl_AllComparableEqualRegionsAreMutationSensitive` and its
git-screen-scoped sibling — that iterate EVERY comparable, currently-equal
region across every screen, deep-copy the evidence, mutate the region's
rendered TEXT (append `"\nGATE-CANARY"`, recompute the hash), and assert
`ValidateRegionDiffs` rejects each one individually. Updated the Makefile's
`-run` regex and doc comments to match the renames.
**Verification:** The exhaustive tests checked 115 regions (52 scoped to
git-screen) — confirmed via a temporary `t.Logf` probe, removed before
commit. Red-before-fix: forced the exhaustive test's inner loop to always
`continue` (simulating zero coverage) and confirmed it correctly fails with
"no comparable, currently-equal region was available to mutate"; reverted.

### CR-12: Configure-Git ceremony disclosed backups that would never exist, undercounted them, omitted directory creation

**Files modified:** `internal/tuikit/backend.go`, `internal/tuikit/views.go`,
`internal/tuikit/ceremony.go`, `internal/tuikit/identities.go`,
`internal/tuikit/identities_test.go`, `internal/tuikit/backend_stub_test.go`,
`internal/dummytui/fixturebackend.go`, `cmd/gitid/wiring.go`,
`cmd/gitid/wiring_test.go`
**Commit:** `7d94031`
**Applied fix:** Added `Backend.GitWritePlan(spec) WritePlanView` — the
sibling seam `CreateWritePlan` already established — implemented in
`realBackend` by mirroring `commitGitArtifacts`' exact write order and
existence checks (including the non-obvious case where `~/.gitconfig` gets
backed up TWICE in one transaction: once by `WriteIncludeIf`, again by
`WriteProviderRewrite`, because by the time the second write runs the first
has already created the file). Added `WritePlanView.CreatedDirs` and a new
`ceremonyConfig.Creates` field, rendered as an explicit "Creates …" line in
state A. `gitCeremonyFor` now sources Targets/Backups/Creates entirely from
`GitWritePlan` instead of hardcoded UI-layer strings (which named
`~/.gitconfig.backup.<ISO>` — a path `filewriter` never actually creates —
and always declared exactly 2 backups).
**Verification:** Three new `realBackend`-level unit tests in
`wiring_test.go` prove the plan matches reality: fresh home → 0 backups + 3
created dirs; `ForceSSH` on a fresh home → exactly 1 backup (the
provider-rewrite one); fully pre-existing identity → exactly 3 backups, 0
created dirs. A temporary probe test (removed before commit) drove
`b.commitGitTransaction` for real and confirmed the plan's predicted backup
count matched the actual commit's backup count exactly, for both
`ForceSSH` states. A `fixedGitWritePlanBackend` test proves `gitCeremonyFor`
renders EXACTLY what the backend reports, red-before-fix confirmed. Full
`make gate-visual-regression` and the compiled real-vs-dummy PTY e2e test
both pass with the corrected copy (golden frames changed as an intended
consequence of the accuracy fix, per the review's own framing).

### CR-14: Match-strategy radio label showed a different gitdir than every other widget on the same frame

**Files modified:** `internal/tuikit/identities.go`,
`internal/tuikit/identities_test.go`, `internal/tuikit/backend_stub_test.go`,
`internal/dummytui/fixturebackend.go`, `internal/screenshot/createflow.go`,
`internal/screenshot/region_disposition_test.go`,
`.planning/design/git-screen/visual-divergence-allowlist.txt`,
`e2e/create_flow_pty_e2e_test.go` (follow-up commit `8e53ab3`)
**Commit:** `c500013` (+ `8e53ab3` follow-up)
**Applied fix:** `strategyCopy(strategy, name)` hardcoded `"~/" + name + "/"`
(missing the `git/` segment every other widget used); changed its signature
to `strategyCopy(strategy, gitDir string)` and pass
`g.gitDirFor(name)`/`w.git.gitDirFor(...)` at both render and click-hit-test
call sites (`hitStrategyRow` updated to match). Also fixed
`FixtureBackend.IncludeIfPreview`/`stubBackend.IncludeIfPreview`, which
substituted the identity name into a frozen `~/personal/` literal — now
substitutes `spec.GitDir` for the gitdir condition specifically, so the
dummy's preview derives the same `"~/git/<identity>/"` shape D-02 requires.
**Second-order consequence, traced and fixed:** because the dummy's
`IncludeIfPreview` now derives the correct shape, the CTX-D-02
`gitPreviewDisposition` predicate (`absent:"gitdir:~/git/"`) it used to
authorize became symmetric (both sides now carry the marker) — which the
JUST-FIXED CR-10 grammar correctly rejects. Reclassified the disposition
(and the matching e2e allowlist entries) from CTX-D-02 "gitdir-default" to
CTX-D-12 "identity-name", using `contains:"gitdir:~/git/"` instead of a
blanket predicate — this actively guards against the CR-14 regression
recurring. The `git-form-empty` allowlist entry was removed (not
re-predicated): its checkpoint renders no includeIf preview on either side,
so an entry there is stale by construction. Also found and fixed a stale
Phase-3 `e2e/create_flow_pty_e2e_test.go` assertion pinned to the OLD wrong
dummy shape (`[includeIf "gitdir:~/acme/"]`), surfaced only once the dummy
fixture was corrected.
**Verification:** New test
`TestStrategyLabelAgreesWithIncludeIfPreviewAndGitDirField` asserts all
THREE widgets (radio label, includeIf preview, gitdir field) show the same
string on one rendered frame. Red-before-fix: reverted `strategyCopy`'s
gitdir case to a hardcoded `~/BUGGY/` and confirmed the test fails with the
mismatch printed verbatim; restored, test passes. `make gate-visual-regression`,
the compiled real-vs-dummy PTY e2e suite, and the full `make test-e2e` suite
(256s) all pass.

### BL-15 (was WR-25, escalated): rollback re-loosened a hardened managed root even when a file under it failed to restore

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `6891363`
**Applied fix:** `restore()`'s file-restoration loop now tracks the raw
(undisplayed) path of every file it fails to restore/remove
(`failedFilePaths`). The `chmodDirs` loop — which reverts a managed root
(e.g. `~/.ssh`) back to its pre-transaction mode — now checks `under(dirPath)`
first: if any failed file lives at or under that directory, the mode revert
is SKIPPED (the root stays at its secured mode) and a
`"<dir>: mode kept at <mode> — a file under it could not be restored"`
outcome line is emitted instead, without counting as an additional
restoration failure.
**Verification:** New fault-injection test
`TestRollbackKeepsHardenedRootSecuredWhenAFileUnderItFailsToRestore` seeds
`~/.ssh` at a loose `0755`, forces the transaction to fail at the last step
AND forces the `allowed_signers` restore itself to fail, then asserts
`~/.ssh` is STILL `0700` afterward (not reverted to `0755`) and the outcome
list names it with "mode kept at". Red-before-fix: stubbed out the `under()`
guard and confirmed the test fails with `~/.ssh mode after rollback =
-rwxr-xr-x, want -rwx------`; restored, test passes. All 22 pre-existing
rollback-matrix subtests (`TestGitTransactionRollbackMatrix...`,
`TestCombinedTransactionRollsBack...`) still pass unchanged.

### WR-35: `newBackendForHome`'s `accounts()` silently read the real developer's `$HOME`

**Files modified:** `internal/identity/inventory.go`, `cmd/gitid/wiring.go`,
`cmd/gitid/wiring_test.go`
**Commit:** `a23fced`
**Applied fix:** `identity.BuildInventoryDeps()`'s three real-filesystem
functions (`readSSHConfigIncludeAware`, `readGitconfigReal`,
`listKeyFilesReal`) all called `os.UserHomeDir()` internally, ignoring any
`home` the caller already owned. Refactored each into a thin `$HOME`-derived
wrapper plus a home-parameterized core
(`readSSHConfigIncludeAwareForHome`/`readGitconfigRealForHome`/`listKeyFilesRealForHome`),
and added `identity.InventoryDepsForHome(home)`, which never touches
`os.UserHomeDir()`/`$HOME` at all. `realBackend.accounts()` now calls
`identity.InventoryDepsForHome(b.home)` instead of
`identity.BuildInventoryDeps()`.
**Verification:** New test
`TestNewBackendForHomeAccountsIsHermeticWithoutSetenvHOME` seeds a sandboxed
home with a sentinel SSH alias and constructs `newBackendForHome(home)`
**without** `t.Setenv("HOME", home)` — exactly the gap the reviewer's CR-09
probe found by accident. Red-before-fix: reverted `accounts()` to call
`identity.BuildInventoryDeps()` and re-ran the test — it failed with
`InitialState().Identities[0].SSHHost = "github.com"`, i.e. it read MY real
developer `~/.ssh/config` identity, not the sandboxed sentinel. Restored the
fix, test passes.

### WR-36 / WR-37: dead code documented as live; stale focus-ring comments contradicting the code

**Files modified:** `internal/screenshot/createflow_regions.go`,
`internal/screenshot/createflow.go`, `internal/tuikit/identities.go`
**Commit:** `37d667e`
**Applied fix:**
- WR-36: deleted the unreachable `RegionContinueDisabledReason` constant and
  its `extractContinueDisabledReason` extractor (absent from
  `AllRegionNames()`, so no gate ever compared it — the extractor had zero
  reachable call sites despite a comment claiming it was "now re-wired").
  Rewrote the `git-form-demo` spec's comment to state plainly that the
  region was DELETED, not re-wired, and to describe where a future
  disabled-Continue screen spec should reintroduce it.
- WR-37: rewrote the three comment blocks around `gitPaneFocusButton`/
  `gitFieldForceSSH`/`gitFieldGitDir`/`paneGitFocusOrder` that described a
  `(m.gitFocus+1) % gitPaneFocusRing` modulo arithmetic that has not existed
  since CR-08, and claimed ForceSSH is "never Tab-cycled" in the pane (false
  since CR-08). Dropped the now-single-purpose `gitPaneFocusRing` constant
  entirely (`gitFieldForceSSH = gitPaneFocusButton + 1 + iota` directly) so
  there is no longer a constant whose name asserts a property ("ring size")
  it does not have. The comments now state plainly that `paneGitFocusOrder`
  and `wizardGitFocusOrder` are the SOLE ring definitions.
**Verification:** Full build + `go vet -tags screenshot ./...` clean. Ran
every focus/strategy/click test in `internal/tuikit` (30 tests, including
`TestGitFlowPaneTabRingVisitsForceSSH`, the CR-08 regression test) — all
pass unchanged, confirming the comment-only + dead-code-removal changes did
not alter runtime behavior.

## Skipped Issues

The following WR-38 through WR-44 findings are in the `critical_warning`
fix_scope but were **not** part of the orchestrator's explicit task list for
this pass (which named CR-13, CR-10/CR-11, CR-12/CR-14/BL-15, the test
hermeticity leak, and dead code/stale comments specifically). They remain
open for a future pass.

### WR-38: Force-SSH checkbox desyncs from disk on uncheck

**File:** `internal/tuikit/identities.go:1768-1772`, `internal/tuikit/store.go:244`
**Reason:** Not in the orchestrator's explicit task list for this pass.

### WR-39: Enter on the Force-SSH checkbox opens the write ceremony instead of toggling (pane + wizard)

**File:** `internal/tuikit/identities.go:2132-2139`, `:2499-2518`, `:824-827`
**Reason:** Not in the orchestrator's explicit task list for this pass.

### WR-40: `ConfigureGit.Name` still reads `m.selected` live after WR-20

**File:** `internal/tuikit/identities.go:1768-1769`
**Reason:** Not in the orchestrator's explicit task list for this pass.

### WR-41: `handleWizardClick` routes the algorithm-row click through a bare `5`

**File:** `internal/tuikit/identities.go:2940-2946`
**Reason:** Not in the orchestrator's explicit task list for this pass.

### WR-42: `displayMessage`'s substring replace is fragile at both ends

**File:** `cmd/gitid/wiring.go:2010-2020`
**Reason:** Not in the orchestrator's explicit task list for this pass.

### WR-43: stale cross-reference to a symbol WR-18 deleted

**File:** `e2e/git_configuration_pty_e2e_test.go:730`
**Reason:** Not in the orchestrator's explicit task list for this pass.

### WR-44: `keygen.AllowedSignersLine` accepts an unvalidated principal

**File:** `internal/keygen/signers.go:22-28`
**Reason:** Not in the orchestrator's explicit task list for this pass. This
is a security-relevant finding (unvalidated principal in
`~/.ssh/allowed_signers`); recommend prioritizing it in the next pass, since
this iteration's review is explicitly adversarial about security-path
findings (see BL-15's escalation from a prior skip).

---

_Fixed: 2026-08-25T06:51:30Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 4_
