---
phase: 04-git-configuration-screen
fixed_at: 2026-08-25T03:50:00Z
review_path: .planning/phases/04-git-configuration-screen/04-REVIEW.md
iteration: 2
findings_in_scope: 11
fixed: 10
skipped: 1
status: partial
---

# Phase 4: Code Review Fix Report (iteration 2)

**Fixed at:** 2026-08-25T03:50:00Z
**Source review:** .planning/phases/04-git-configuration-screen/04-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope: 11 (3 critical, 8 warning)
- Fixed: 10
- Skipped: 1 (WR-16 — documented follow-up, see below)

**Verification performed on every commit:** `go build ./...`, `TERM=dumb SSH_AUTH_SOCK= go
test -count=1 -race ./...`, `go vet -tags screenshot ./...`, `go build -tags screenshot
./...` / `go build -tags "screenshot e2e" ./...`, `make gate-visual-regression`. The full
`make test-e2e` suite (real-PTY, real binary, `-race`) was additionally run after CR-05/06/07
and again after every remaining commit, both times green (`ok github.com/castocolina/gitid/e2e`,
~255s). `make lint` (`golangci-lint run ./...` via the pre-commit `go-lint` hook, which every
commit below passed) — note: invoking `golangci-lint` directly from a shell (not via the
pre-commit hook) fails in this sandbox with a Go-1.27/golangci-lint-v2.12.2 stdlib-typecheck
mismatch unrelated to any of these changes (confirmed identical on the pre-existing, unmodified
tree); the pre-commit hook's own toolchain resolution does not hit this and is what actually
gates every commit.

## Fixed Issues

### CR-05: `~/.ssh` (and gitid's other managed roots) stay world-writable when pre-existing

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `c730027`
**Applied fix:** Added `mutationJournal.ensureManagedDir`, used only for gitid's own managed
roots (`b.sshDir`, `b.fragmentDir`, `b.includeDir`): it still hardens a pre-existing directory
to the documented mode. The user-supplied `gitdir` keeps plain `ensureDir` (CR-01's actual
scope — never mode-mutated as a side effect of being referenced). This finding was combined
with WR-17 in one commit — see WR-17 below for why (same code path; splitting them would have
left an intermediate state that reintroduces WR-17's bug or fails lint on an unused field).
Adds `TestCommitCreateHardensPreExistingSSHDir` (seeds `~/.ssh` at 0777, asserts 0700 after a
confirmed create).

### CR-06: Wizard's Force-SSH row — no toggle, wrong Tab order, Enter writes

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`,
`internal/tuikit/batch3_test.go`, `e2e/create_flow_pty_e2e_test.go`,
`internal/screenshot/createflow.go`
**Commit:** `4219119` (combined with CR-07 — see that entry for the shared-file rationale)
**Applied fix:** Option (b) from the review: Force SSH becomes a first-class wizard field.
`gitFieldForceSSH` now sits immediately after `gitPaneFocusRing` so it can never numerically
alias `gitPaneFocusButton` (CR-04's original concern, still enforced). The wizard's own Tab
ring is no longer raw modulo arithmetic over contiguous enum values (impossible now that
`gitFieldForceSSH` is deliberately non-contiguous with `gitFieldStrategy` — that slot already
belongs to the pane's button) — it is explicit slice-based cycling via `wizardGitFocusOrder` +
`wizardGitFocusStep`, and field-vs-button routing is an exhaustive `isWizardGitField`
membership switch, not an ordinal `<`/`>=` comparison. `TestWizardClickTableEntriesAreAllWizardRingMembers`
is the load-bearing guard the review asked for in place of the old zero-length-array
compile-time trick (which only caught a negative difference against one threshold): it fails
for ANY `gitFormFieldSlots` entry that is not also a `wizardGitFocusOrder` member, in either
direction. Every wizard Tab-count assumption in the test suite and the screenshot capture
script was updated for the new ring size (Name → Email → Strategy → Force SSH → Back → Skip →
Continue).

### CR-07: Wizard's gitdir preview/write converge on a stale `"acme"` seed instead of the real identity

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `4219119` (combined with CR-06)
**Applied fix:** Added `gitForm.gitDirEdited`, tracking whether the field carries an
AUTHORITATIVE value — set in `handleEdit`'s `gitFieldGitDir` case (a real keystroke) and in
`openGitForm` when the configure-Git pane loads a genuine stored path in edit mode. While
unset, `gitDirFor(identity)` re-derives `"~/git/<identity>/"` live from whatever identity name
is passed in at read time. The wizard never renders a gitdir row, so `gitDirEdited` is always
unset there — both the preview (`gitSpec()`) and the write (`finishIdentity()`) call
`w.git.spec(w.form.identityName(), ...)` live, so they now track a renamed identity and agree
with each other. Adds `TestWizardGitDirTracksRenamedIdentity` (renames the wizard's identity
prefix mid-flow, asserts preview == write == the live-derived default).

**Why CR-06 and CR-07 share one commit:** both required extensive, interleaved changes to the
exact same `gitForm`/wizard focus code in `internal/tuikit/identities.go` (the const block CR-06
rewrites sits immediately above the `gitForm` struct CR-07 extends); a mechanical split risked
an intermediate non-compiling or semantically-wrong state. Per `CLAUDE.md`'s own commit
guidance ("let the buildable boundary, not file count, set the commit granularity"), this was
judged the correct atomic unit.

### WR-17: Rollback chmods every watched directory (including HOME) the transaction never touched

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `c730027` (combined with CR-05 — see that entry)
**Applied fix:** Added `mutationJournal.chmodDirs`, populated only by `ensureManagedDir` when it
actually chmods a pre-existing directory. `restore()`'s directory-mode loop now iterates
`chmodDirs` instead of the full `dirs` watch-snapshot, so rollback reverts exactly the
directories this transaction changed — never HOME or an ancestor merely walked over while
resolving a path.

### WR-18: ~110 lines of dead allowlist-parsing machinery invisible to `make lint`

**Files modified:** `cmd/gitid/gate_visual_regression_test.go`
**Commit:** `f7b7745`
**Applied fix:** Deleted `allowlistEntry`, `parseAllowlist`, `splitAllowlistLine`, and the also-dead
`allowlistPath` const (zero call sites, confirmed via `grep -rn` across the whole repo before
deleting), plus the now-unused `"bufio"` import. Verified with `go vet -tags screenshot ./...`
and `go build -tags "screenshot e2e" ./...`, both clean, before and after.

### WR-19: `RequiredRegions` gates presence only — acceptance is unbounded blanket dispositions

**Files modified:** `internal/screenshot/createflow.go`, `internal/screenshot/createflow_packet.go`,
`internal/screenshot/region_disposition_test.go` (new)
**Commit:** `96a09d0`
**Applied fix:** Added an optional `Predicate` field to `RegionDisposition`, reusing the SAME
`contains:<text>`/`absent:<text>` grammar the e2e git-screen allowlist already parses/enforces
(`gitScreenPredicateSatisfied`). `BuildRegionDiffs` now rejects a differing region whose
live/approved text does not satisfy its disposition's predicate (checked against either side,
mirroring the e2e allowlist's own rule); `ValidateScreenSpecs` rejects a malformed predicate at
registry-load time. Predicate is optional — empty (every existing disposition today) preserves
current blanket acceptance exactly, so this is backward compatible.
**Scope note (documented per the review's own guidance):** this lands the MECHANISM only.
Retroactively narrowing the ~12 existing blanket dispositions to specific predicates would
require per-screen visual inspection of the actual live-vs-approved diff text for each one —
guessing a predicate without that inspection risks either breaking the gate (too narrow) or
providing no real scoping (too permissive, e.g. matching a substring present in both frames
regardless of the real divergence). That narrowing is better done as a follow-up with visual
review, not blind pattern-guessing in this pass. New dispositions can opt into scoping
immediately via the new `uxRegionDifferenceScoped` constructor.

### WR-20: `ConfigureGit` reduced GitName/GitEmail/MatchStrategy live off the pane, not the committed spec

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`
**Commit:** `081e1fe`
**Applied fix:** `GitName`/`GitEmail`/`MatchStrategy` now read from `m.gitCommitSpec` (the exact
spec captured at `ceremonyConfirmed`), matching what WR-14 already did for `GitDir`/`ForceSSH`/
`PublicKeyPath`. Adds `TestConfigureGitNameEmailStrategyReduceExactCommittedSpec`, which mutates
`gitPaneForm` AFTER dispatch (simulating the pane having moved on before the async result
arrives) and asserts `ConfigureGit` still reduces the committed values.

### WR-21: `commitCreateTransaction.fail` always returned `nil` backups

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `cad1042`
**Applied fix:** `fail()` now returns `journal.backups` instead of `nil`. `CommitCreate` maps
them through `b.displayPath` and surfaces them in `WizardCommitMsg` on both outcomes, the same
way `CommitGit` already does. Adds
`TestCombinedTransactionSurfacesBackupsOnFailureEvenWhenRestorationSucceeds`, which fails a
transaction after a real backup is taken with restoration succeeding normally (the common
failure shape — the pre-existing `TestCombinedTransactionRetainsBackupsWhenRestorationFails`
only covered the rarer restore-also-fails case) and asserts `msg.Backups` is non-empty and
display-shortened.

### WR-22: `extractGitCeremony`'s marker phrase is not wrap-proof

**Files modified:** `internal/screenshot/createflow_regions.go`, `internal/screenshot/createflow_test.go`
**Commit:** `d80c7b9`
**Applied fix:** `extractGitCeremony` now checks a 2-row sliding window (current row + next,
whitespace-collapsed) instead of one row at a time, so a wrap landing at either of the phrase's
two internal spaces still matches. Adds
`TestExtractRegion_GitCeremonyMatchesReceiptHeadingAcrossWrap` with an identity name long enough
to force the wrap exactly inside the phrase (the pre-existing wrap-example test happens to wrap
AFTER the phrase, so it never exercised this failure mode).

### WR-23: Wrapped errors still leak absolute sandbox paths into the failure receipt

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `53690a4`
**Applied fix:** Added `realBackend.displayMessage` (a `strings.ReplaceAll(msg, b.home, "~")`
pass) applied to every `Err` string `CommitGit`/`CommitCreate` can return, plus `CommitGit`'s
`Restored` outcome lines. Adds `TestCommitGitErrorMessageIsDisplayShortened` and
`TestCombinedTransactionErrorMessageIsDisplayShortened`, which force a REAL `gitConfigSet`
failure (seeding the fragment path as a directory so `git config --file <dir> ...` fails with a
genuine error embedding that absolute path) rather than a synthetic injected error, and assert
the resulting `Err` never contains the raw sandbox HOME path.

## Skipped Issues

### WR-16: Directory creation is still never disclosed in the ceremony `Targets`/`Preview`

**File:** `internal/tuikit/identities.go:1921` (`gitCeremonyFor`'s `Targets`), `cmd/gitid/wiring.go:1049-1055`
**Reason:** Implementing this correctly requires a genuinely new capability, not a mechanical
bug fix: `internal/tuikit` is intentionally UI-free / no direct filesystem access (CLAUDE.md —
all I/O goes through the injected `Backend` seam), so disclosing "will this create a new
directory" requires a NEW `Backend` interface method, implemented in BOTH the real backend
(`cmd/gitid`, a straightforward `os.Stat` check) AND the dummy/fixture backend
(`internal/dummytui`), whose answer must stay deterministic for `make gate-visual-regression`'s
symmetric real/dummy comparison. That comparison already golden-pins the exact `confirm-write`
and `git-form-filled`/`review-readonly` frame text across the gate's allowlist; adding a new
`+ create directory ~/git/<identity>/ (does not exist yet)` preview line changes those golden
frames on MULTIPLE screens, and `04-UI-SPEC.md` D-08 marks the exact copy as DRAFT, explicitly
requiring `agent-ui-ux-designer` + Codex review before it is authoritative. That is a
UI-copy-and-fixture design task with real design-review dependencies, not something safe to
guess-and-ship in an automated fix pass — doing so risks landing wrong copy that then has to be
re-reviewed and re-fixed anyway, or silently breaking the visual-regression gate's goldens.
**Original issue:** `commitGitArtifacts` `mkdir`s the gitdir on confirmation, but the
ceremony's `Targets` list is a fixed three-file list — the user confirms three files and gets a
fourth mutation, with no diff line disclosing it, violating `04-UI-SPEC.md` D-08.
**Recommendation:** Route through `/gsd-ui-phase` or a dedicated design pass to finalize the
exact D-08 copy (currently DRAFT), then implement as a normal phase task — not squeeze into a
review-fix iteration.

---

_Fixed: 2026-08-25T03:50:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
