---
phase: 04-git-configuration-screen
reviewed: 2026-08-25T04:20:00Z
depth: standard
iteration: 2
files_reviewed: 28
files_reviewed_list:
  - .gitignore
  - cmd/gitid/gate_visual_regression_test.go
  - cmd/gitid/wiring_test.go
  - cmd/gitid/wiring.go
  - e2e/create_flow_pty_e2e_test.go
  - e2e/git_configuration_pty_e2e_test.go
  - e2e/ui_pty_e2e_test.go
  - internal/doctor/checks/reserved_test.go
  - internal/dummytui/fixturebackend.go
  - internal/gitconfig/reader_test.go
  - internal/gitconfig/reader.go
  - internal/gitconfig/renderer_test.go
  - internal/gitconfig/renderer.go
  - internal/gitconfig/fragment.go
  - internal/gitconfig/fragment_test.go
  - internal/identity/loader_test.go
  - internal/identity/loader.go
  - internal/keygen/signers.go
  - internal/screenshot/createflow_regions.go
  - internal/screenshot/createflow_test.go
  - internal/screenshot/createflow.go
  - internal/screenshot/normalize_test.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/identities_test.go
  - internal/tuikit/identities.go
  - internal/tuikit/store.go
  - internal/tuikit/views.go
  - Makefile
findings:
  critical: 3
  warning: 8
  info: 0
  total: 11
status: issues_found
---

# Phase 4: Code Review Report (iteration 2 — post-fix re-review)

**Reviewed:** 2026-08-25T04:20:00Z
**Depth:** standard
**Files Reviewed:** 28
**Status:** issues_found

## Summary

This is an adversarial re-review of the 17 fixes recorded in `04-REVIEW-FIX.md`.
Every claim was verified against the current source, and the four CRITICAL fixes
were re-exercised with executable probes against the real backend / real model
(probe files were created, run, and deleted; `git status` is clean).

**What genuinely landed.** CR-02 is fixed and proven: with an injected
`host-block` failure *and* an injected restore failure, `~/.ssh/config.bak.<nanos>`
survives on disk and is named in the error message. WR-03 (dead legacy
transaction), WR-04 (`txMu`, no lock-ordering hazard — no path takes `b.mu`
before `txMu`), WR-06 (redundant third `~/.gitconfig` backup removed; the
remaining `WriteIncludeIf` backup still captures pre-transaction bytes),
WR-07 (`gitDir.CursorEnd()`), WR-09 (anchored `\.bak\.\d+` + row-scoped
fallback), WR-11 (two-identity precondition + `git-form-empty != git-form-filled`),
WR-12 (`saveFrame` → gitignored `/tmp/ui-frames`), WR-13 (`min`,
`currentGitCommit`, `io` deleted) and WR-15 (`--` end-of-options terminator on
both `git config` call sites) are all correctly and completely applied.
`go test ./...`, `make lint` (0 issues) and `make gate-visual-regression` (23
frames) all pass on the current tree.

**What did not land, or regressed.** Three of the four CRITICAL fixes introduced
new defects that no test in the tree catches:

1. **CR-01 over-corrected.** Removing the unconditional `os.Chmod` also removed
   it for gitid's *own* managed roots. A pre-existing `~/.ssh` at `0777`
   now survives a full confirmed create untouched — gitid writes a private key
   into a world-writable SSH directory and no longer hardens it
   (**reproduced**). The user-supplied gitdir half of CR-01 *is* correctly
   fixed (verified against a pre-existing `~/Documents` at `0750` and a nested
   `~/Projects` at `0755`: parents keep their mode, only the created leaf gets
   `0700`; `GitDir: "~/"` is refused).
2. **CR-04's renumbering left a live collision consumer behind.** The enum
   ranges no longer overlap, but `gitFormFieldSlots` — the wizard's *own*
   mouse click-routing table — still maps the wizard's rendered `Force SSH` row
   to `gitFieldForceSSH`, which is now `6`, outside the wizard's `% 6` ring.
   Clicking it and pressing `space` no longer toggles anything, `tab` jumps to
   *Email* (skipping Name), and `enter`/`right` advance straight to the write
   ceremony (**all three reproduced**). The new compile-time guard does not
   cover this, and the new test only pins the *pane's* table.
3. **CR-03 made preview and write agree on the wrong value.** `newGitForm` is
   now called from `newWizard` with `form.identityName()` evaluated *once*, at
   construction, when the alias prefix is still the hardcoded default `acme`.
   The gitdir input is never re-derived. Renaming the identity to `work`
   produces `~/git/acme/` in **both** the preview and the written `includeIf`
   (**reproduced**) — violating `04-UI-SPEC.md` D-07 (`~/git/<identity>/`) and
   silently regressing behaviour that was correct before the fix
   (`finishIdentity` used to hardcode `"~/git/" + name + "/"`). The wizard
   never renders the gitdir row, so the user cannot correct it. The fixer's
   `TestWizardGitDirPreviewMatchesWrite` never renames the identity, so it
   cannot see this.

Two fixes are only partially applied (WR-08's substance, WR-14's field set), and
the CR-01 remediation the prior review asked for alongside the chmod change —
disclosing the directory creation in the ceremony's `Targets`, which is also
`04-UI-SPEC.md` D-08 — was not done and was not reported as skipped.

## Critical Issues

### CR-05: CR-01's fix stopped hardening gitid's own managed directories — `~/.ssh` stays world-writable

**File:** `cmd/gitid/wiring.go:853-886` (`mutationJournal.ensureDir`), call sites `:1046`, `:1668`
**Severity:** BLOCKER

**Issue:** The CR-01 fix deleted the trailing unconditional
`os.Chmod(path, mode)` and now only chmods entries recorded in `createdDirs`.
That is correct for the user-editable gitdir, but `ensureDir` is *also* the
enforcement point for gitid's own roots:

```go
journal.ensureDir(b.sshDir, sshDirMode)      // wiring.go:1668 (0700)
journal.ensureDir(b.fragmentDir, 0o700)      // wiring.go:1046
```

`commitCreateTransaction`'s own doc comment (`wiring.go:1611`) still promises
"0. Real SSH directory creation/**mode**". It no longer does.

Reproduced against the real backend (temp HOME, `~/.ssh` seeded `0777`, full
confirmed `commitCreateTransaction`):

```
PROBE create err=<nil>
PROBE ~/.ssh mode after commitCreateTransaction = 777 (seeded 0777, sshDirMode is 0700)
```

gitid then writes the new private key into that directory. The key file itself
is `0600`, but a group/world-**writable** `~/.ssh` lets any local user replace
`config`, `known_hosts`, or `authorized_keys`, and trips `sshd` StrictModes.
For a tool whose stated purpose is to "safely and auditably manage" `~/.ssh`,
silently accepting an insecure `~/.ssh` is a security regression, not a
neutral no-op.

**Fix:** Distinguish tool-owned roots from user-named paths instead of dropping
the chmod for both. Keep `ensureDir` mode-neutral for arbitrary paths and add an
explicit hardening call for the roots gitid owns:

```go
// ensureDir stays as-is (created dirs only).

// ensureManagedDir hardens a root gitid itself owns, and records the prior
// mode so rollback can restore it.
func (j *mutationJournal) ensureManagedDir(path string, mode os.FileMode) error {
	if err := j.ensureDir(path, mode); err != nil {
		return err
	}
	// watchDir already snapshotted the prior mode → restore() will revert it.
	return os.Chmod(path, mode)
}

// wiring.go:1668 / :1046 — use ensureManagedDir for b.sshDir and b.fragmentDir.
// wiring.go:1053   — the user-supplied gitdir keeps plain ensureDir.
```

Add a regression test that seeds `~/.ssh` at `0777` and asserts it is `0700`
after a confirmed create (the mirror of the existing
`TestGitTransactionDoesNotChmodPreExistingGitDir`).

---

### CR-06: CR-04's renumbering broke the wizard's Force-SSH row — no toggle, wrong Tab order, Enter writes

**File:** `internal/tuikit/identities.go:2653-2657` (`gitFormFieldSlots`), `:2818-2823` (wizard click routing), `:2449-2451` (the new guard), `:528-536` (the new enum + guard)
**Severity:** BLOCKER

**Issue:** The fix moved `gitFieldForceSSH`/`gitFieldGitDir` from `3`/`4` to
`6`/`7`, on the stated premise that "the wizard never renders or edits them"
(`identities.go:516-519`). That premise is false in two places that were not
updated:

- `gitForm.view` **does** render the Force-SSH checkbox in the wizard's Git
  step and bolds it when `focus == gitFieldForceSSH` (`identities.go:751-759`).
- `gitFormFieldSlots` — explicitly documented as "shared by the wizard step 2
  **and** Configure-Git click handlers" (`identities.go:2651-2657`) — still
  maps `"Force SSH"` to `gitFieldForceSSH`, and the wizard's click handler
  assigns it straight into the wizard ring:

```go
// identities.go:2818
if slot, ok := hitAnyFieldRow(body, x, y, gitFormFieldSlots); ok {
	w.gitFocus = slot   // now 6 — outside the wizard's `% wizardGitFocusSlots` (6) ring
```

Reproduced (real `identitiesModel`, wizard driven to step 2, focus set exactly
as `handleWizardClick` sets it):

```
PROBE gitFocusBack=3 gitFocusSkip=4 gitFocusContinue=5 wizardGitFocusSlots=6
PROBE gitFieldForceSSH=6 gitFieldGitDir=7
PROBE gitFormFieldSlots "Force SSH" -> 6
PROBE after space on Force-SSH focus: forceSSH true -> true   (want toggled)
PROBE after tab   on Force-SSH focus: gitFocus=1              (want 0 / gitFieldName)
PROBE after enter on Force-SSH focus: step=3 configureGit=true (want a toggle, got the WRITE ceremony)
PROBE after right on Force-SSH focus: step=3                   (same)
```

So: the checkbox is rendered and clickable but inert (the new guard at `:2449`,
`if w.gitFocus < gitFocusBack`, blocks slot 6 from ever reaching `handleEdit`);
Tab re-enters the ring at the wrong index; and `Enter` on what looks like a
checkbox jumps to the confirm-write ceremony. Before this fix the collision made
`space` accidentally work, so this is a user-visible regression, not merely an
unchanged latent bug.

The two new tests miss it by construction:
`TestWizardGitStepButtonFocusNeverEditsHiddenFields` only ever reaches focus via
Tab (never via a click), and `TestGitFormFieldSlotsNeverAliasPaneWriteButton`
pins the table against the *pane's* button only. The compile-time guard
`var _ [gitFieldForceSSH - wizardGitFocusSlots]struct{}` (`:536`) is a
zero-length array that only fires when the field range drops *below* the button
ring — it cannot detect that a click table feeds an out-of-ring value into the
wizard.

**Fix:** Either (a) give the wizard its own click table that omits the
pane-only rows, or (b) — better, since the wizard *does* render the checkbox —
make Force-SSH a first-class wizard slot inside the ring. Minimal version of (a):

```go
// gitFormFieldSlots stays the PANE's table.
// wizardGitFieldSlots omits every pane-only row.
var wizardGitFieldSlots = []fieldSlot{
	{"user.name", gitFieldName},
	{"user.email", gitFieldEmail},
}

// identities.go:2818
if slot, ok := hitAnyFieldRow(body, x, y, wizardGitFieldSlots); ok {
```

and stop rendering the Force-SSH checkbox in the wizard's view (or render it
`styleFaint` and non-focusable) so nothing clickable is inert. Add a guard that
is actually load-bearing:

```go
func init() { // or a test
	for _, s := range wizardGitFieldSlots {
		if s.slot >= gitFocusBack {
			panic("wizard click table routes a non-wizard slot into gitFocus")
		}
	}
}
```

---

### CR-07: CR-03 aligned preview and write on a stale seed — the written `gitdir` no longer tracks the identity name

**File:** `internal/tuikit/identities.go:878` (`newWizard`), `:586-597` (`newGitForm`), `:624-633` (`normalizeGitDir`), `:1485-1486` (`finishIdentity`)
**Severity:** BLOCKER

**Issue:** `newGitForm`'s new `identity` parameter is supplied by `newWizard` as
`form.identityName()`, evaluated **once at wizard construction**, when the alias
prefix is still the hardcoded default `"acme"` (`identities.go:870`):

```go
git: newGitForm(b, form.identityName(), "Acme Identity", ...)   // identities.go:878
```

Nothing re-seeds `git.gitDir` when the user edits the prefix / SSH Host, and
`normalizeGitDir` only substitutes the identity-derived default when the value
is **empty** (`identities.go:624-628`) — it never is, because `newTextInput`
seeded it. `finishIdentity` now honours that stale value verbatim
(`id.GitDir = gitSpec.GitDir`, `:1486`), where it previously recomputed
`"~/git/" + name + "/"` correctly.

Reproduced (real `wizardModel`, prefix edited to `work`):

```
PROBE identityName="work"
PROBE preview gitSpec().GitDir="~/git/acme/"
PROBE write   finishIdentity().GitDir="~/git/acme/"
PROBE keyPath="~/.ssh/id_ed25519_work"
```

CR-03's stated goal (preview == write) is met, but both now carry the wrong
path. Consequences:

- `04-UI-SPEC.md` D-07 mandates the default be `~/git/<identity>/`; the
  screenshot registry's own disposition text repeats it verbatim
  (`internal/screenshot/createflow.go:320-321`: "the real binary derives the
  gitdir default as `~/git/<identity>/` per D-02"). Both are now violated for
  any identity not named `acme`.
- `matchesFor` (`cmd/gitid/wiring.go:1823-1831`) writes
  `[includeIf "gitdir:~/git/acme/"]` into the user's real `~/.gitconfig` for an
  identity called `work`, so the Git identity never activates in
  `~/git/work/`.
- `commitGitArtifacts` then `ensureDir`s `~/git/acme/` (`wiring.go:1049-1055`),
  creating a directory named after the wizard's default seed.
- The wizard never renders the gitdir row (`identities.go:3465-3467` is
  `paneGit`-only), so the user cannot see or correct the value.

Every existing test uses the default prefix (`e2e/create_flow_pty_e2e_test.go:441`
asserts `gitdir:~/git/acme/`; `TestWizardGitDirPreviewMatchesWrite` never
renames), which is why this passes green.

**Fix:** Derive the gitdir at read time, not at construction, so it tracks the
identity unless the user has explicitly overridden it. Track "user edited" state
rather than relying on emptiness:

```go
type gitForm struct {
	// ...
	gitDirEdited bool // set in handleEdit's gitFieldGitDir case
}

// gitDirFor returns the effective gitdir: the user's edit if there is one,
// otherwise the live identity-derived default (D-07).
func (g gitForm) gitDirFor(identity string) string {
	if g.gitDirEdited {
		return normalizeGitDir(g.gitDir.Value(), identity)
	}
	return "~/git/" + identity + "/"
}

// spec():
GitDir: g.gitDirFor(identity),
```

Regression test: build the wizard, `w.form.prefix.SetValue("work")`, and assert
`w.gitSpec().GitDir == w.finishIdentity().GitDir == "~/git/work/"`.

## Warnings

### WR-16: The directory the transaction creates is still never disclosed in the ceremony

**File:** `internal/tuikit/identities.go:1921` (`gitCeremonyFor` `Targets`), `cmd/gitid/wiring.go:1049-1055`
**Issue:** CR-01's fix note asked for two things; only the chmod half was
applied. `commitGitArtifacts` still `mkdir`s the gitdir on confirmation, and the
ceremony's `Targets` list is a fixed three files:

```go
Targets: []string{"~/.gitconfig.d/" + sel.Name, "~/.gitconfig", "~/.ssh/allowed_signers"},
```

`04-UI-SPEC.md` D-08 specifies the exact missing affordance —
`+ create directory ~/git/<identity>/ (does not exist yet)` as a `+`-prefixed
diff line inside the confirm-write preview — and it is implemented nowhere
(`grep -rn "create directory" internal/ cmd/` returns nothing). The user
confirms three files and gets a fourth mutation. This was not listed as a
skipped finding in `04-REVIEW-FIX.md`.
**Fix:** In `gitCeremonyFor` (and `reviewCeremony`), stat the resolved gitdir and
append `"+ create directory " + gitDir + " (does not exist yet)"` to `Preview`
when it is missing, per D-08.

### WR-17: Rollback chmods every watched directory — including HOME — that the transaction never touched

**File:** `cmd/gitid/wiring.go:930-949` (`restore()` dir loop), `:859-871` (`ensureDir`'s walk-up)
**Issue:** `ensureDir` `watchDir`s every ancestor up to and including HOME, and
`restore()` unconditionally `os.Chmod`s every watched directory back to its
snapshot mode, whether or not this transaction ever changed it. Visible in a
rollback probe:

```
~: restoration failed: ...
~/.ssh: restoration failed: ...
```

gitid therefore writes permission metadata to the user's home directory on every
failed transaction, and would silently revert a legitimate concurrent
permission change made between snapshot and rollback. It also inflates the
user-facing "restoration results" list with lines for paths that were never
mutated.
**Fix:** Only restore the mode of a directory the transaction actually changed —
record `(path, oldMode)` at the point of an actual `os.Chmod`, and iterate that
list in `restore()` instead of the whole `dirs` snapshot.

### WR-18: ~110 lines of dead allowlist machinery survive WR-13, hidden from `make lint`

**File:** `cmd/gitid/gate_visual_regression_test.go:36-160` (`allowlistEntry`, `parseAllowlist`, `splitAllowlistLine`)
**Issue:** WR-13 removed `min` and `currentGitCommit` but left the much larger
dead block next to them. `parseAllowlist` has **no call sites**
(`grep -n "parseAllowlist(" cmd/gitid/gate_visual_regression_test.go` → only the
definition), `splitAllowlistLine` is reachable only from it, and
`allowlistEntry.used` is never set. It duplicates the *live* loader in
`e2e/git_configuration_pty_e2e_test.go:783`, which actually reads
`.planning/design/git-screen/visual-divergence-allowlist.txt`. The file carries
`//go:build screenshot`, and `make lint` runs `golangci-lint run ./...` without
that tag, so the `unused` linter never sees it — the exact "dead code kept alive
past the linter" pattern WR-03/WR-13 were raised about.
**Fix:** Delete the three symbols, or add `screenshot,e2e` to the lint build tags
in `.golangci.yml` so tagged files are analysed and the dead code fails the gate.

### WR-19: WR-08 restored presence checking but left the acceptance predicate unbounded

**File:** `internal/screenshot/createflow.go:242-258` (`mouse-focused-field`), `:297-322` (`git-form-demo`), `:180-184` (`uxRegionDifference`)
**Issue:** The fix restored `RegionFormFields`/`RegionHostPreview` to
`RequiredRegions`. But `RequiredRegions` is **presence-only** — the gate's own
comment says so (`cmd/gitid/gate_visual_regression_test.go:406-408`: "every named
region that is nonempty on either side is gated. RequiredRegions remains the
mandatory-presence subset, not the comparison inventory"), and `BuildRegionDiffs`
iterates `AllRegionNames()` regardless
(`internal/screenshot/createflow_packet.go:1285`). Acceptance is decided by
`RegionDispositions`, and every one of these is a blanket
`uxRegionDifference(...)` with no text predicate, so *any* future drift in those
regions is still auto-classified `ux-improvement` and passes. The prior review's
actual request — pin it with a narrow `contains:`/`absent:` predicate — was not
implemented. `git-form-demo`'s `RegionKeybar`-only `RequiredRegions` is likewise
unchanged.
**Fix:** Add an optional `Predicate string` to `RegionDisposition` (reusing the
`contains:`/`absent:` grammar the e2e allowlist already parses) and reject a
differing region whose diff does not satisfy it, so an accepted divergence is
scoped to the text that was actually approved.

### WR-20: WR-14 applied to three of six `ConfigureGit` fields

**File:** `internal/tuikit/identities.go:1617-1622`
**Issue:** The reducer now reuses `m.gitCommitSpec` for `GitDir`, `ForceSSH` and
`PublicKeyPath`, but `GitName`, `GitEmail` and `MatchStrategy` are still read
live off `m.gitPaneForm`:

```go
spec := m.gitCommitSpec
return keyResult{..., actions: []Action{ConfigureGit{
	Name: m.selected, GitName: m.gitPaneForm.name.Value(), GitEmail: m.gitPaneForm.email.Value(),
	MatchStrategy: m.gitPaneForm.strategy(), GitDir: spec.GitDir, ...
```

`spec.Name`, `spec.Email` and `spec.Strategy` are already captured and carry the
exact committed values. Leaving three fields on the live form keeps the same
class of divergence WR-14 was raised to eliminate, and makes the intent of the
line ambiguous to the next reader.
**Fix:** `GitName: spec.Name, GitEmail: spec.Email, MatchStrategy: spec.Strategy`.

### WR-21: `commitCreateTransaction.fail` still returns `nil` backups; the two failure paths disagree

**File:** `cmd/gitid/wiring.go:1635-1655` vs `:1026-1036`
**Issue:** CR-02 correctly stopped *deleting* the backups, but `fail()` returns
`nil, fmt.Errorf(...)` — the retained backup paths exist only inside the error
string, and only when `restoreErr != nil`. When rollback succeeds, the stale
`.bak.<nanos>` files are retained on disk and never mentioned anywhere, so the
user has no way to find or clean them. `commitGitArtifacts.fail` (`:1029`,
`:1035`) does return `journal.backups` on the same kind of failure, so the two
transaction entry points still report differently — the asymmetry the prior
review flagged.
**Fix:** `return journal.backups, fmt.Errorf(...)` and have `CommitCreate` surface
them in `WizardCommitMsg` (mapped through `b.displayPath`), the same way
`CommitGit` does at `:749-754`.

### WR-22: WR-10's narrowed marker is a contiguous phrase that can wrap out of existence

**File:** `internal/screenshot/createflow_regions.go:268`
**Issue:** `strings.Contains(rpPlain, "configured — applies via")` matches a
**24-character contiguous run** against a *single rendered row* of a
width-constrained detail pane. The source line it targets is
`Git identity "<name>" configured — applies via the <strategy> strategy.`
(`identities.go:1924-1925`), which wraps as soon as the identity name is long
enough — and the phrase can split at either of its two internal spaces. When it
does, `extractGitCeremony` returns `""` and the region silently degrades to
empty on both surfaces, so the comparison passes vacuously. The previous bare
`"configured"` marker was wrap-proof but over-broad; the replacement trades one
failure mode for another.
**Fix:** Match on a token that cannot wrap and cannot false-positive, e.g. the
`ResultMessage`'s stable prefix `Git identity "` combined with a negative check,
or normalise the row before matching:

```go
plain := strings.Join(strings.Fields(rpPlain), " ")
if strings.Contains(plain, "Write Git identity") || strings.Contains(plain, "configured — applies via") {
```

(joining the *whole pane* rather than a single row, or matching on the
already-unwrapped `ResultMessage` constant, removes the wrap dependency
entirely). Add a test with an identity name long enough to force the wrap.

### WR-23: WR-01 shortened the backup list but wrapped errors still leak absolute sandbox paths into the receipt

**File:** `cmd/gitid/wiring.go:1026-1036` (`fail`), `internal/gitconfig/fragment.go:105`, `:124`
**Issue:** `CommitGit` now maps `Backups` through `displayPath` and `restore()`'s
outcomes are shortened, but the `Err` string that reaches
`GitCommitMsg`/`ceremony.commitFailed` is built from wrapped errors that embed
raw absolute paths — e.g. `gitConfigSet` returns
`fmt.Errorf("git config --file %s %s: ...", path, key, ...)` with an absolute
`path`, and `filewriter`/`sshconfig` errors do the same. The three-row wrapping
problem WR-01 described therefore still occurs on the failure receipt, which is
the screen where legibility matters most.
**Fix:** Map the message through a home-shortening pass before it leaves the
backend, e.g. `strings.ReplaceAll(msg, b.home, "~")` in `fail()`/`CommitGit`, or
have the low-level packages return display-relative paths.

---

_Reviewed: 2026-08-25T04:20:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
_Iteration: 2 (re-review of 04-REVIEW-FIX.md)_
