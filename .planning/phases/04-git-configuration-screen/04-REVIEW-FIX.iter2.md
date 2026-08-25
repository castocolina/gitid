---
phase: 04-git-configuration-screen
fixed_at: 2026-08-25T02:34:59Z
review_path: .planning/phases/04-git-configuration-screen/04-REVIEW.md
iteration: 1
findings_in_scope: 17
fixed: 17
skipped: 0
status: all_fixed
---

# Phase 4: Code Review Fix Report

**Fixed at:** 2026-08-25T02:34:59Z
**Source review:** .planning/phases/04-git-configuration-screen/04-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 17 (4 critical, 13 warning — `fix_scope: critical_warning`)
- Fixed: 17
- Skipped: 0

**Verification environment note:** every fix below was implemented and verified inside an
isolated git worktree (`workflow.use_worktrees=true`), NOT the main checkout — `git worktree
add -b gsd-reviewfix/04-61453`. The reported test/lint/gate/e2e results are reproducible by
checking out `gsd-reviewfix/04-61453` (or the fast-forwarded `gsd/phase-04-git-configuration-screen`
branch after this report is committed) and re-running the same commands from the main checkout;
they are not reproducible by inspecting the now-removed worktree directory itself.

## Fixed Issues

### CR-01: User-editable gitdir path is chmod'ed to 0700 — including HOME

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `573e93a`
**Applied fix:** `mutationJournal.ensureDir` no longer unconditionally `os.Chmod`s the final path
component. Only directories the transaction itself creates (recorded in `createdDirs`) get their
mode set — a pre-existing directory referenced via the editable gitdir field is watched (for
rollback bookkeeping) but never mutated. Added an explicit guard that refuses to manage HOME itself
(`clean == filepath.Clean(j.b.home)` returns an error), which also covers the reproduced `~/` case.
Added two regression tests: `TestGitTransactionDoesNotChmodPreExistingGitDir` (asserts a
pre-existing `~/Documents`-style directory keeps its original mode `0750` after a successful
commit) and `TestGitTransactionRejectsHomeAsGitDir` (asserts `GitDir: "~/"` fails and HOME's mode
is untouched).

### CR-02 / WR-02: Rollback deletes every timestamped backup, including after a failed restore

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `2a34abc`
**Applied fix:** `commitCreateTransaction.fail` no longer deletes any backup on the failure path —
mirrors `commitGitArtifacts.fail`'s existing (correct) behavior. When `restore()` itself fails, the
message now explicitly lists the retained backup paths ("timestamped backups retained: ..."),
resolving WR-02's dead-branch complaint (the two arms of the `if restoreErr != nil` check now
genuinely differ) in the same edit. Updated `TestCommitTransactionRollsBackAfterEveryInjectedFailure`
(which previously asserted the OLD, incorrect "no stale backup survives" behavior) to instead assert
the backup from the `host-block` step is retained. Added
`TestCombinedTransactionRetainsBackupsWhenRestorationFails`, which forces a restoration failure and
asserts the fragment's `.bak.*` file survives on disk.

### CR-03: Wizard Git preview shows a different gitdir than the one written

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `7cbbe0f`
**Applied fix:** `newGitForm` now takes a separate `identity` parameter (the alias/identity name)
distinct from `name` (the Git author display name), and seeds `gitDir` from `identity` — no longer
producing a path with a space in it. `finishIdentity` now sets `id.GitDir = gitSpec.GitDir` instead
of hardcoding `"~/git/" + name + "/"`, so the write always matches what the ceremony previewed. The
two consecutive `if w.configureGit {` blocks were merged into one. Per the review's request to "say
which [is correct] in the fix commit": the WRITE now matches the PREVIEW (both derive from the
identity name), not the reverse — this is the directionally-correct fix per the recipes' gitdir
contract (`~/git/<identity>/`, never an author display name with spaces). Added
`TestWizardGitDirPreviewMatchesWrite`, which asserts the previewed `GitSpec.GitDir` contains no
spaces and exactly equals `finishIdentity().GitDir`.

### CR-04: Git form field enum collides with the button-ring enum

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `46b68bd`
**Applied fix:** Renumbered so the two enum spaces can never collide. `gitFieldName/Email/Strategy`
(the wizard's three actually-editable fields, unchanged at 0/1/2) and `gitFocusBack/Skip/Continue`
(unchanged at 3/4/5 — preserving every existing Tab-order test) now live in one contiguous range;
`gitFieldForceSSH`/`gitFieldGitDir` (PANE-only fields the wizard never renders) were moved to sit
immediately AFTER the wizard's button ring (`wizardGitFocusSlots + iota` = 6/7), so they can never
again numerically alias `gitFocusBack`/`gitFocusSkip`. `gitPaneFocusButton` (3) no longer collides
with `gitFieldForceSSH` (now 6). Two compile-time array-length guards
(`var _ [gitFieldForceSSH - wizardGitFocusSlots]struct{}`) fail the build if the ranges ever
overlap again. Added a defense-in-depth runtime gate (`if w.gitFocus < gitFocusBack` before calling
`gitForm.handleEdit`) in the wizard's step-2 default key branch. Note: an earlier attempt that
naively moved `gitFocusBack` to start after `gitFieldGitDir` broke the wizard's Tab-cycling
contiguity (6 existing tests failed — `TestWizardGitButtonsArrowNavigatesWizardSteps`,
`TestWizardSkipCreatesIncompleteIdentity`, etc.) because it introduced unreachable "gap" values in
the modulus ring; the final fix instead moves the FIELD side of the collision, preserving every
pre-existing wizard button-focus value. Added
`TestWizardGitStepButtonFocusNeverEditsHiddenFields` (typing on Back/Skip focus no longer mutates
gitdir or Force-SSH) and `TestGitFormFieldSlotsNeverAliasPaneWriteButton` (pins the pane's
click-routing table against the button slot).

### WR-01: Standalone Git receipt prints raw absolute backup paths

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `c76efd1`
**Applied fix:** `mutationJournal.restore()`'s outcome strings, `commitCreateTransaction.fail`'s
"timestamped backups retained" list, and `CommitGit`'s returned `Backups`/`Restored` fields are all
now mapped through `b.displayPath` before reaching the user — matching what
`commitCreateTransaction`'s success path already did. Added
`TestCommitGitReturnsDisplayShortenedBackupPaths`, which asserts every backup path in a standalone
`CommitGit` result starts with `~/`, never the sandbox's absolute prefix.

### WR-03: ~200 lines of dead legacy transaction kept alive to defeat the linter

**Files modified:** `cmd/gitid/wiring.go`
**Commit:** `d6c54a5`
**Applied fix:** Deleted `commitCreateTransactionLegacy` in full (its rollback semantics duplicated
`commitCreateTransaction`'s with the OLD `os.Rename(backup, target)` model), the
`var _ = (*realBackend).commitCreateTransactionLegacy` reference, and the `//nolint:unused`
directive. Its doc comment (which actually described the CURRENT function, not the legacy one) was
relocated to sit above `commitCreateTransaction`. Net -212 lines.

### WR-04: File transactions are not serialized against each other

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `455c16c`
**Applied fix:** Added a dedicated `txMu sync.Mutex` field to `realBackend`, held for the whole of
`commitCreateTransaction` and `commitGitTransaction` (the two entry points; `commitGitArtifacts`
itself is never called from outside them, so this covers every mutation path). Added
`TestTransactionsAreSerializedAgainstEachOther`, which holds `txMu` externally, launches a
concurrent `commitGitTransaction` in a goroutine, asserts it stays blocked for 100ms, then releases
the lock and asserts the call completes promptly — proving mutual exclusion, not merely a declared
field. Verified under `go test -race`.

### WR-05: Unchecking "Force SSH" silently does nothing

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `31e3893`
**Applied fix:** `gitCeremonyFor` now appends an explicit note to the write ceremony's preview when
Force SSH is off and a provider is set: "Force SSH off: the shared provider-rewrite:<host> block is
left in place because other identities may use it." — the exact copy the review suggested. Added
`TestGitCeremonyNotesSharedProviderRewriteWhenForceSSHOff`, asserting the note is absent when Force
SSH is on and present with the exact text when toggled off.

### WR-06: Redundant full-file rewrite of ~/.gitconfig purely to force a backup

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `5b04d60`
**Applied fix:** Deleted the "allowed-signers-file-backup" step entirely (its read-then-rewrite of
`~/.gitconfig` existed only to mint a third `.bak.<nanos>` path with no new content — the journal's
in-memory pre-transaction snapshot is what actually drives rollback, not these display-only
timestamped files). Removed the now-nonexistent `"allowed-signers-file-backup"` boundary from the
two parametrized rollback-matrix tests' step lists. Added
`TestGitTransactionTakesAtMostTwoGitconfigBackups`, asserting a full ForceSSH-true Configure-Git
write takes at most 2 `~/.gitconfig.bak.*` files (includeIf + provider-rewrite), never a third.

### WR-07: gitDir caret is left at column 0 after SetValue

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `45f21e3`
**Applied fix:** Added `m.gitPaneForm.gitDir.CursorEnd()` immediately after the `SetValue` call in
`openGitForm`, matching the same pattern already documented and used for hostname/port. Added
`TestOpenGitFormHomesGitDirCaretAfterSetValue`, asserting `Position()` equals the value's rune
length (not 0) after opening the pane.

### WR-08: Visual-regression gate weakened rather than the divergence fixed

**Files modified:** `internal/screenshot/createflow.go`, `internal/screenshot/createflow_regions.go`
**Commit:** `bf5a614`
**Applied fix:** Restored `RegionFormFields`/`RegionHostPreview` to `mouse-focused-field`'s
`RequiredRegions` (it is a `VariantOf` `ssh-form-filled`, which already requires them) — the
pre-existing `RegionDispositions` entries already declared the accepted D-16 divergence, so they
were dead metadata without the region actually being required; the gate now genuinely checks them
again. Verified `TestGateVisualRegression` still passes with the narrower allowlist enforced.
**Investigated but reverted:** re-wiring the orphaned `extractContinueDisabledReason` extractor
(dead code, unreferenced by any `RegionName`) as a required region on `git-form-demo` was attempted
per the review's second example, but `git-form-demo` captures the wizard's Git step with valid,
filled fields — `[ Continue ]` is enabled and no disabled-reason line is ever rendered at that
checkpoint. Adding the requirement broke `TestGateVisualRegression` with "missing required region"
on a screen it does not describe. The extractor and a new `RegionContinueDisabledReason` constant
were still wired into `ExtractRegion`'s switch (no longer orphaned/dead), documented as available
for a FUTURE screen spec that captures the disabled-Continue state, but not forced onto
`git-form-demo`.

### WR-09: Blanket \d{6,} normalizer can mask real divergences

**Files modified:** `internal/screenshot/createflow.go`, `internal/screenshot/normalize_test.go`
**Commit:** `32b83ac`
**Applied fix:** Split the single blanket `\d{6,}` pattern into two: `backupSuffixPattern`
(`(\.bak\.)\d+`), anchored to the literal `.bak.` prefix, handles every unwrapped occurrence
precisely; `wrappedDigitRowPattern` is a narrower fallback that only matches a digit run
constituting an ENTIRE rendered row's content (after the pane border/padding) — the specific shape a
19-digit nanosecond suffix takes when it wraps mid-number across the fixed 100-column pane. An
inline byte count, key size, or future numeric ID embedded alongside other text on the same row is
no longer normalized away. Added a new internal test file, `normalize_test.go`, with three tests:
anchored-match, wrapped-fragment normalization (using the exact wrapped shape captured in
`git-configuration-mouse-field-focus.txt`), and — the core guarantee — that inline numeric content
survives normalization.

### WR-10: extractGitCeremony start marker is over-broad

**Files modified:** `internal/screenshot/createflow_regions.go`, `internal/screenshot/createflow_test.go`
**Commit:** `0db87fc`
**Applied fix:** Narrowed the marker from a bare `"configured"` substring to
`"configured — applies via"` — the exact receipt heading `gitCeremonyFor` builds — mirroring the
same hardening `extractConnectivityOutput`'s `"ssh "` → `"ssh -"` fix already applied to this exact
class of bug. Added `TestExtractRegion_GitCeremonySkipsUnrelatedConfiguredMention` (the sidebar note
"no Git identity configured for this alias" no longer starts the region early) and
`TestExtractRegion_GitCeremonyMatchesReceiptHeading` (the narrowed marker still fires on the real
receipt text).

### WR-11: CaptureGitScreenScreens degrades silently when the backend has <2 identities

**Files modified:** `internal/screenshot/createflow.go`, `internal/screenshot/createflow_test.go`
**Commit:** `d9d5d3e`
**Applied fix:** Added the exact precondition assertion the review suggested
(`if n := len(backend.InitialState().Identities); n < 2 { return nil, fmt.Errorf(...) }`), plus a
second assertion that `out["git-form-empty"] != out["git-form-filled"]` after capture, catching the
case where 2+ identities are seeded but the second happens to render identically (or a future script
change stops advancing to it). Added `TestCaptureGitScreenScreens_RequiresTwoIdentities` with a new
`singleIdentityBackend` test fixture (wraps `dummytui.FixtureBackend`, truncates
`InitialState().Identities` to one).

### WR-12: make test-e2e writes non-deterministic frames into the tracked .planning/ tree

**Files modified:** `e2e/ui_pty_e2e_test.go`
**Commit:** `989ccc5`
**Applied fix:** `saveFrame` now writes to `<repoRoot>/tmp/ui-frames/` instead of
`.planning/phases/05.7-.../ui-frames/`. `/tmp/` at the repo root is ALREADY gitignored specifically
for this purpose (`.gitignore`: "Local scratch directory (screenshots, notes — not part of the
repo)"), and — unlike `t.TempDir()`, which Go removes at test end — it survives the run for manual
inspection, matching the review's suggested alternatives. Ran the FULL `make test-e2e` suite
(`go test -tags e2e -race -timeout 360s ./e2e/...`, ~257s) to verify: all e2e tests pass, `git status
--short .planning/` is clean after the run, and the frames land in `tmp/ui-frames/` (confirmed
gitignored via `git check-ignore -v`).

### WR-13: Dead code and a panicking slice in the gate test helper

**Files modified:** `cmd/gitid/gate_visual_regression_test.go`
**Commit:** `2ba5fa5`
**Applied fix:** Deleted `min` (unreferenced anywhere in `cmd/gitid`, and shadows the Go 1.21+
builtin), `currentGitCommit` (also unreferenced; its body additionally contained the dead
`io.ReadAll(strings.NewReader(""))` call the review flagged, and the panicking
`strings.TrimSpace(string(hashBytes))[:7]` slice with no length check), and the now-unused `io`
import. Ran `gofmt -l`/`-w` to confirm clean formatting after the deletion.

### WR-14: Reduced ConfigureGit state is rebuilt with an empty key path

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `f700651`
**Applied fix:** Added a `gitCommitSpec GitSpec` field to `identitiesModel`, captured at
`ceremonyConfirmed` (the exact spec passed to `backend.CommitGit`). `handleMsg`'s `GitCommitMsg`
reducer now reuses `m.gitCommitSpec` verbatim instead of recomputing
`m.gitPaneForm.spec(m.selected, "")` with an empty keyPath — eliminating the class of bug where the
reduced state could describe a different write than the one actually performed. Added
`TestConfigureGitReducesExactCommittedSpec`, which drives a full ceremony-confirm → commit-success
cycle for an identity with no stored `PublicKeyPath` and asserts the resulting
`ConfigureGit.PublicKeyPath` equals the exact committed spec's value, never the empty-keyPath
literal `".pub"`.

### WR-15: git config --file values starting with - are parsed as git options

**Files modified:** `internal/gitconfig/fragment.go`, `internal/gitconfig/fragment_test.go`
**Commit:** `66bdf00`
**Applied fix:** Both `gitConfigSet` and `gitConfigUnsetAll` now insert a `--` end-of-options marker
before the key (and, transitively, the value) in the `exec.Command` arg slice — the review's
suggested fix. This is a broader, more robust primary defense than adding a leading-`-` rejection to
`validateValue` (which risked falsely rejecting an unusual-but-legitimate user name); `--` protects
every positional argument at the actual injection boundary, regardless of which value carries the
risky prefix. Added `TestWriteFragment_LeadingDashValueIsLiteralNotGitOption` and
`TestSetAllowedSignersFile_LeadingDashValueIsLiteral`, each asserting a `--`-prefixed value round-trips
as the literal string via `git config --get`, for both `gitConfigSet` call sites.

## Skipped Issues

None — all 17 in-scope findings were fixed.

## Verification Summary

Every commit above was verified individually (targeted regression test + package build + `go vet`)
before being committed, and the full suite was re-verified after all 17 fixes landed:

- `go build ./...`, `go build -tags screenshot ./...`, `go build -tags e2e ./...` — all pass.
- `go vet -tags "screenshot e2e" ./...` — clean.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — **1040 tests passed**, 0 failed.
- `make gate-visual-regression` — passes (`TestApprovalCommitRecorded` SKIPs with
  `cannot read .git/HEAD: open ../../.git/HEAD: not a directory` — a worktree-structural artifact
  unrelated to any fix here; `.git` is a file, not a directory, in every git worktree checkout).
- `make lint` (`golangci-lint run ./...`) — **0 issues**.
- `make test-e2e` (`go test -tags e2e -race -timeout 360s ./e2e/...`) — **all pass** (~257s), run once
  after WR-12 to confirm the tracked `.planning/` tree stays clean.
- `go test -tags screenshot ./internal/screenshot/...` — 82 passed, 11 pre-existing environment
  failures (confirmed via `git stash` against the pre-fix tree before any changes: `freeze` binary
  not on `PATH`, and several tests requiring a prior `GenerateTextPacket`/live-capture artifact that
  is not part of this fix's scope). No new failures introduced.

All verification ran inside the isolated worktree (`gsd-reviewfix/04-61453`, created under
`workflow.use_worktrees=true`); the worktree's commits fast-forward
`gsd/phase-04-git-configuration-screen` on cleanup, so the same commands are reproducible from the
main checkout after that fast-forward completes.

---

_Fixed: 2026-08-25T02:34:59Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
