---
phase: 04-git-configuration-screen
reviewed: 2026-08-25T05:40:00Z
depth: standard
iteration: 3
files_reviewed: 29
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
  - internal/screenshot/createflow_packet.go
  - internal/screenshot/normalize_test.go
  - internal/screenshot/region_disposition_test.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/identities_test.go
  - internal/tuikit/identities.go
  - internal/tuikit/store.go
  - internal/tuikit/views.go
  - Makefile
findings:
  critical: 2
  warning: 12
  info: 0
  total: 14
status: issues_found
---

# Phase 4: Code Review Report (iteration 3 — final auto-fix iteration)

**Reviewed:** 2026-08-25T05:40:00Z
**Depth:** standard
**Files Reviewed:** 29
**Status:** issues_found

## Summary

Adversarial re-review of the 10 fixes recorded in `04-REVIEW-FIX.md` (iteration 2).
Every claim was re-verified against current source, and each of CR-05/CR-06/CR-07/
WR-19 was re-exercised with executable probes against the real backend and the real
`identitiesModel`/`wizardModel` (probe files created, run, deleted; `git status` is
clean — only `.planning/` review artifacts differ).

**Gate status on the current tree:** `go test -count=1 ./...` all green,
`make lint` 0 issues, `go test -tags screenshot ...` compiles. As with iteration 1,
green gates are not evidence: two of the defects below are exercised by tests that
pass *vacuously*, and one landed a regression the entire suite cannot see because
the affected package is outside every automated gate.

### What genuinely landed (verified)

- **CR-05 (create path) — fixed and proven.** `mutationJournal.ensureManagedDir`
  hardens gitid's own roots even when they pre-exist. Probe against the real
  backend with `~/.ssh`, `~/.ssh/config.d` and `~/.gitconfig.d` all seeded loose:
  all three end at `0700`; HOME is untouched (`755` before and after). The
  user-supplied gitdir still keeps its mode (`TestGitTransactionDoesNotChmodPreExistingGitDir`,
  0750, re-verified), and `GitDir: "~/"` is still refused.
- **WR-17 — fixed and proven.** A failed create's restoration list is
  `~/.ssh/config.d/gitid.config; ~/.ssh/config; …; ~/.ssh; ~/.ssh/config.d` —
  no `~:` line, HOME mode unchanged. Rollback scopes to `chmodDirs` only.
- **CR-07 (wizard gitdir) — fixed and proven.** `gitDirFor` re-derives live:
  driving the real wizard to step 2 with the identity renamed to `acme2` gives
  `gitSpec().GitDir == finishIdentity().GitDir == "~/git/acme2/"`. Preview and
  write agree *and* track the rename.
- **CR-06 (wizard Tab ring) — half fixed.** The wizard's ring is now
  `0,1,2,4,6,7,8` (Name→Email→Strategy→Force SSH→Back→Skip→Continue) and `space`
  on Force SSH toggles it. Every Tab-count assumption in `e2e/` and
  `internal/screenshot/createflow.go` was updated consistently.
- **WR-15, WR-18, WR-20, WR-21, WR-23** applied as described. `git config --file f -- key value`
  verified against git 2.55 to both work and reject `--global` as a key.

### What did not land, or regressed

1. **The Configure-Git pane — Phase 4's own screen — can no longer toggle Force SSH
   at all** (CR-08). CR-06 only added Force SSH to the *wizard's* ring. In the pane
   the Tab ring is still `% gitPaneFocusRing` (0–3) so slot 4 is unreachable, and the
   click table's `"Force SSH"` entry can never match a rendered row. Reproduced end
   to end: 6 Tab stops × `space` → no toggle; a real `MouseClickMsg` on the rendered
   `Force SSH` cell leaves `gitFocus` at 0. The fixer's own test comment
   (`identities_test.go:2301-2304`) records knowing this — it was neither fixed nor
   listed as skipped in `04-REVIEW-FIX.md`.
2. **`toDemoIdentity` never populates `ForceSSH`** (CR-09), so the pane always opens
   unchecked for a disk-loaded identity, and WR-05's ceremony note ("the shared
   provider-rewrite … is left in place") is therefore emitted unconditionally at the
   confirm-write gate — a statement about the user's `~/.gitconfig` that gitid never
   checked.
3. **WR-22's fix introduced an off-by-one** (WR-26): `extractGitCeremony`'s 2-row
   sliding window makes `i = k-1` match first, so `RegionGitCeremony` now always
   starts one rendered row *above* the heading. Reproduced with a direct
   `ExtractRegion` probe.
4. **WR-19 landed a mechanism with zero call sites** (WR-27): all 32 dispositions
   are still blanket `uxRegionDifference(...)`; `uxRegionDifferenceScoped` has no
   production caller; comparison gating is byte-for-byte what it was before the fix.
5. **CR-05's fix is create-path only** (WR-24): the standalone Git flow — the Phase 4
   screen's own write path — still neither hardens nor creates `~/.ssh` while writing
   `allowed_signers` into it.

---

## Critical Issues

### CR-08: Force SSH is unreachable in the Configure-Git pane — no Tab stop, no working click, no keybinding

**File:** `internal/tuikit/identities.go:2079` and `:2090` (pane Tab ring), `:2762` (`gitFormFieldSlots`), `:2663` (pane click routing), `:850` (the row's render), `:2780` (`anchoredLabelMatch`)
**Severity:** BLOCKER

**Issue:** `gitFieldForceSSH` is now `4`. The configure-Git pane's ring is unchanged:

```go
// identities.go:2079 / :2090
m.gitFocus = (m.gitFocus + 1) % gitPaneFocusRing   // gitPaneFocusRing == 4 → gitFocus ∈ {0,1,2,3}
```

so Tab can never reach slot `4`. `ctrl+g` (`:2083`) routes to `gitFieldGitDir`, not Force SSH.
That leaves the click table as the only path — and it cannot fire. The row is rendered
as the tail of a helper line:

```go
// identities.go:850
b.WriteString(helperLine("Kept byte-identical to ~/.ssh/allowed_signers (GITUI-04) · gpg.format=ssh · signingkey="+
    …+" · "+forceStyle.Render(forceMarker+" Force SSH"), false) + "\n")
```

while `hitFieldRow`/`anchoredLabelMatch` (`:2780`) require the *row* to START with the
label after stripping only gutter/marker glyphs. The rendered row starts with
`gpgsign=true · ☐ Force SSH`, so `anchoredLabelMatch(line, "Force SSH")` is always false.

Reproduced against the real `identitiesModel` (`g` → paneGit, 100×30):

```
PROBE pane stop 0 gitFocus=0 -> space forceSSH false->false
PROBE pane stop 1 gitFocus=1 -> space forceSSH false->false
PROBE pane stop 2 gitFocus=2 -> space forceSSH false->false
PROBE pane stop 3 gitFocus=3 -> space forceSSH false->false
PROBE pane stop 4 gitFocus=0 ...            (ring wraps — slot 4 never visited)
PROBE pane Force SSH row 7: "… │ gpgsign=true · ☐ Force SSH" (click x=55)
PROBE pane after click: gitFocus=0 forceSSH=false
PROBE gitFieldForceSSH reachable via gitFormFieldSlots anywhere in the pane body: false
```

The same click is inert in the wizard (`gitFocus` stays `0` after clicking the rendered
cell) — only the wizard's *Tab* path works, so `gitFormFieldSlots`' third entry is dead
in both surfaces.

**This is a regression introduced by the fix loop.** Before `46b68bd` (CR-04),
`gitFieldForceSSH == gitPaneFocusButton == 3`, so Tab-focusing the pane's `Write it…`
button and pressing `space` fell through `handleGitKey`'s `default` into
`handleEdit(msg, 3)` and toggled it. CR-04 removed that path; CR-06 replaced it only
for the wizard.

**Fix:** give the pane a ring that contains the field it renders, and make the row
actually hit-testable.

```go
// 1) Pane ring: cycle by explicit slice position, like the wizard already does.
var paneGitFocusOrder = []int{gitFieldName, gitFieldEmail, gitFieldStrategy, gitFieldForceSSH, gitPaneFocusButton}

// identities.go:2078-2093
case "tab", "down":
    m.gitFocus = paneGitFocusStep(m.gitFocus, 1)
case "shift+tab", "up":
    m.gitFocus = paneGitFocusStep(m.gitFocus, -1)

// 2) Render the checkbox on its OWN row so anchoredLabelMatch can anchor it:
b.WriteString(helperLine("Kept byte-identical to …", false) + "\n")
b.WriteString("     " + forceStyle.Render(forceMarker+" Force SSH") + "  " +
    styleFaint.Render("(space toggles · rewrites https:// → git@ for this provider)") + "\n")
```

**Required regression test (must fail before the fix):** drive the model to `paneGit`,
render, locate the `Force SSH` row in the rendered body, send a real
`tea.MouseClickMsg` at that cell, then `space`, and assert `gitPaneForm.forceSSH`
flipped. A membership assertion over `gitFormFieldSlots`
(`TestWizardClickTableEntriesAreAllWizardRingMembers`) provably cannot catch this —
it passes today.

---

### CR-09: `ForceSSH` is never loaded from disk, so the confirm-write ceremony asserts an unverified fact about `~/.gitconfig`

**File:** `cmd/gitid/wiring.go:1269-1305` (`toDemoIdentity`), `internal/tuikit/identities.go:2002` (`openGitForm`), `:2020-2023` (the WR-05 note)
**Severity:** BLOCKER

**Issue:** `toDemoIdentity` is the only place real on-disk state becomes a
`DemoIdentity`. It projects `GitDir`, `MatchStrategy`, `GitName`, `GitEmail`,
`PublicKeyPath` … and never sets `ForceSSH` (grep confirms: the only writers of
`DemoIdentity.ForceSSH` are `finishIdentity` and the in-memory `ConfigureGit`
reducer). So for any identity read from disk, `sel.ForceSSH == false`, always.

`openGitForm` then computes:

```go
// identities.go:2002
m.gitPaneForm.forceSSH = sel.ForceSSH || !m.gitExisting   // == false for every existing Git identity
```

and `gitCeremonyFor` appends, unconditionally for every such identity:

```go
// identities.go:2020-2023
if !m.gitPaneForm.forceSSH && m.gitPaneForm.provider != "" {
    preview += "\n\nForce SSH off: the shared provider-rewrite:" + m.gitPaneForm.provider +
        " block is left in place because other identities may use it."
}
```

`provider` is always non-empty for a real identity (`openGitForm:1985-1987` derives it
from the SSH host when unset). Consequences:

1. The pane misreports stored state: an identity whose `[url "git@github.com:"] insteadOf`
   block IS present in `~/.gitconfig` renders `☐ Force SSH`.
2. The confirm-write ceremony — the one screen CLAUDE.md requires to be honest — tells
   the user a provider-rewrite block "is left in place" even when no such block exists.
   gitid never reads `[url …] insteadOf` to answer this.
3. Per CR-08 the user cannot correct the checkbox, so `commitGitArtifacts:1150`
   (`if spec.ForceSSH && …`) can never add the rewrite from this screen. The Force SSH
   feature is unreachable *and* misreported on Phase 4's headline screen.

**Fix:** read the real state and gate the note on it.

```go
// internal/gitconfig: add a reader for the managed provider-rewrite block.
func HasProviderRewrite(gitconfigPath, provider string) (bool, error)

// identity.Account gains ForceSSH, populated during Reconstruct from that reader.
// cmd/gitid/wiring.go toDemoIdentity:
row.ForceSSH = acct.ForceSSH

// identities.go:2002 — trust the loaded value in edit mode:
m.gitPaneForm.forceSSH = sel.ForceSSH || !m.gitExisting

// identities.go:2020 — only claim the block is retained when it actually exists:
if !m.gitPaneForm.forceSSH && m.gitPaneForm.provider != "" && sel.ForceSSH {
```

Add a test that seeds `~/.gitconfig` WITH and WITHOUT the `insteadOf` block and asserts
the checkbox state and the presence/absence of the note in each case.

---

## Warnings

### WR-16: The directory the transaction creates is still never disclosed in the ceremony (carried forward)

**File:** `internal/tuikit/identities.go:2026` (`gitCeremonyFor` `Targets`), `cmd/gitid/wiring.go:1115-1125`
**Severity:** WARNING
**Issue:** Unchanged from iteration 2, and confirmed still live: my create probe shows
`~/git/personal` and `~/git` both created at `0700` by a confirmed write, while the
ceremony's `Targets` remains the fixed three-file list. The user confirms three files
and gets a fourth mutation. `grep -rn "create directory" internal/ cmd/` still returns
nothing.

**Verdict on the fixer's skip (explicitly requested):** **I agree with deferring the
implementation, and disagree with treating it as closed.** The reasoning is sound in
substance — `internal/tuikit` has no filesystem access by design, D-08's exact copy is
DRAFT in `04-UI-SPEC.md`, and adding a preview line changes golden frames on multiple
screens that `make gate-visual-regression` pins. Two corrections to the account:
(a) the "new Backend capability" is smaller than described — `CreateWritePlan(spec, git)`
is already the established seam that computes targets/backups from the real filesystem
(`wiring.go:678-710`, using `fileExists`), so a `GitWritePlan(spec)` sibling is an
extension of an existing pattern, not a novel capability; and (b) an *unconditional*
disclosure (`"~/git/<identity>/ (created if missing)"` in `Targets`) needs no stat at
all and would already close the honesty gap. It still changes goldens, so deferring is
defensible — but this must remain an OPEN tracked finding routed to a design pass, not
a resolved one.

### WR-24: CR-05's hardening is create-path only — the standalone Git flow neither creates nor secures `~/.ssh`

**File:** `cmd/gitid/wiring.go:1072-1076` (watch-only), `:1112` (fragmentDir is hardened), `:1181` (writes into `~/.ssh`)
**Severity:** WARNING
**Issue:** `commitGitArtifacts` `watchDir`s `filepath.Dir(b.allowedSigners)` but never
`ensureDir`s or `ensureManagedDir`s it, even though `WriteAllowedSignersReplacing`
writes into that directory. Reproduced with `commitGitTransaction` on a sandbox HOME:

```
PROBE ~/.ssh after standalone git = 755 (seeded loose)     ← NOT hardened
PROBE ~/.gitconfig.d              = 700 (seeded loose)     ← hardened
```

and with `~/.ssh` absent entirely the whole Git configuration fails at the LAST step,
after the fragment and includeIf have already been written and must be rolled back:

```
PROBE standalone git (no ~/.ssh) err=gitid: mutation allowed-signers failed:
  gitid: writing allowed signers: keygen: writing allowed_signers:
  creating temp file in …/.ssh: … no such file or directory
```

CR-05's finding was "gitid's own managed roots must end at the documented mode"; that
now holds for `CommitCreate` and not for `CommitGit`.
**Fix:** in `commitGitArtifacts`, replace the watch-only loop with
`journal.ensureManagedDir(filepath.Dir(b.allowedSigners), sshDirMode)` (keeping the
plain `watchDir` for any other path), and mirror
`TestCommitCreateHardensPreExistingSSHDir` for `commitGitTransaction`.

### WR-25: Rollback re-loosens a hardened managed root even when file restoration failed

**File:** `cmd/gitid/wiring.go:964-1015` (`restore()`)
**Severity:** WARNING
**Issue:** `restore()` runs the file loop first, collecting `failures`, then runs the
`chmodDirs` loop **unconditionally**. If removing/restoring a file failed — e.g. the
freshly written private key could not be removed — the transaction still reverts
`~/.ssh` to its loose pre-transaction mode, leaving a `0600` private key inside a
group/world-writable `~/.ssh`. This is a narrow window, but it is exactly the security
posture CR-05 was raised to protect.
**Fix:** skip (or defer) the mode revert for a managed root when any file under it
failed to restore, and say so in the outcome line:

```go
for i := len(j.chmodDirs) - 1; i >= 0; i-- {
    s := j.chmodDirs[i]
    if anyFailureUnder(failures, s.path) {
        outcomes = append(outcomes, j.b.displayPath(s.path)+
            ": mode kept at "+s.mode.String()+" — a file under it could not be restored")
        continue
    }
    …
}
```

### WR-26: WR-22's sliding window made `extractGitCeremony` start one row EARLY

**File:** `internal/screenshot/createflow_regions.go:281-289`
**Severity:** WARNING
**Issue:** The window is `rp[i] + " " + rp[i+1]` and `start = i` on the first match. When
the marker phrase is fully intact on row `k` (the common, unwrapped case), the window at
`i = k-1` already contains it, so the loop breaks one row too early. Reproduced:

```
PROBE region:
     UNRELATED PRECEDING ROW          ← should not be in the region
     Write Git identity for "work"
     Exact change
```

`RegionGitCeremony` therefore now absorbs one extra, arbitrary row into a region that
feeds `BuildRegionDiffs`. Both surfaces run the same extractor so the comparison still
passes, but the region's boundary is wrong and can now capture unrelated (potentially
nondeterministic) content — the same "region no longer means what it says" failure WR-22
was raised about, inverted.
**Fix:** keep the window for detection, but resolve the start to the row that actually
carries the phrase.

```go
normalized := strings.Join(strings.Fields(rp[i]), " ")
if containsMarker(normalized) {
    start = i
    break
}
if i+1 < len(rp) {
    joined := strings.Join(strings.Fields(rp[i]+" "+rp[i+1]), " ")
    if containsMarker(joined) {
        start = i // only when the phrase genuinely spans the boundary
        break
    }
}
```

Add the negative case to the test suite: a frame with an unrelated row directly above the
heading, asserting the region does NOT contain it.

### WR-27: WR-19's predicate gating has zero production call sites — acceptance is unchanged

**File:** `internal/screenshot/createflow.go:198-206` (`uxRegionDifferenceScoped`), `:140-147` (`RegionDisposition.Predicate`)
**Severity:** WARNING
**Issue:** Verified: `grep -rn "uxRegionDifferenceScoped\|Predicate:" internal/screenshot/*.go`
outside tests returns only the definition. All 32 dispositions still use blanket
`uxRegionDifference(...)` with an empty `Predicate`, and `regionPredicateSatisfied("")`
returns `true` unconditionally — so `BuildRegionDiffs`' new rejection branch
(`createflow_packet.go:1322`) is never taken for any real disposition. The gate accepts
exactly the same set of divergences it accepted before the fix. `uxRegionDifferenceScoped`
is now dead production code kept alive only by a build-tagged test (see WR-28), which is
the `min`/`currentGitCommit`/`parseAllowlist` pattern WR-03/WR-13/WR-18 were each raised
about. Secondary: `absent:X` is satisfied when X is missing from *either* side, so
`contains:X` and `absent:X` both pass for any region that differs w.r.t. X — the grammar
is weak as a gate even once used.
**Fix:** narrow at least the dispositions whose divergence text is already documented in
`.planning/design/git-screen/visual-divergence-allowlist.txt` (that file names the exact
strings), converting them to `uxRegionDifferenceScoped`, and add a test that drives
`BuildRegionDiffs` with a mutation the predicate is supposed to catch and asserts the
error. Until at least one production disposition carries a predicate, WR-19 should not
be recorded as fixed.

### WR-28: `internal/screenshot` is outside both automated gates, so WR-19's and WR-22's regression tests never run

**File:** `internal/screenshot/createflow.go:1` (`//go:build screenshot`), `Makefile:159,165-166,270,284,316`, `.github/workflows/ci.yml:84-98`
**Severity:** WARNING
**Issue:** `make lint` is `golangci-lint run ./...` with no build tags, and `make test` is
`go test -race ./...` with no build tags — so every `//go:build screenshot` file in
`internal/screenshot` is invisible to both. No make target runs the package's tests
either: `screenshot-tui` is `-run TestCaptureTUI`, `screenshot-html` is
`-run TestCaptureHTML`, `gate-visual-regression` targets `./cmd/gitid/...` only, and CI
runs only `make test`, `make lint`, `make test-e2e`. Consequently
`internal/screenshot/region_disposition_test.go` (WR-19's entire test evidence) and
`TestExtractRegion_GitCeremonyMatchesReceiptHeadingAcrossWrap` (WR-22's) execute in **no**
gate — which is why WR-26 shipped undetected. WR-18 was fixed by deleting the dead symbols
rather than by closing this blindspot, so the blindspot remains.
**Fix:** add the tags to the lint config and a package test target:

```yaml
# .golangci.yml
run:
  build-tags: [screenshot, e2e, smoke]
```

```make
test: gate-copy-freeze
	go test -race -coverprofile=coverage.out ./...
	go test -tags screenshot -race ./internal/screenshot/...
```

### WR-29: the e2e assertion that "proves" the Force-SSH click passes vacuously

**File:** `e2e/git_configuration_pty_e2e_test.go:423-425`
**Severity:** WARNING
**Issue:**

```go
clickLabelRow(t, s, "Force SSH")
s.sendKey([]byte(" "), keystrokeDelay)
mustSee(t, s, "☐ Force SSH", "mouse click focused the Force-SSH toggle; space toggled it off")
```

`seedGitPTYIdentity` creates a Git-configured identity, so `gitExisting` is true and
(per CR-09) `sel.ForceSSH` is false — the checkbox is already `☐` before the click. The
assertion therefore holds whether or not the click and the space do anything, which is
precisely why the dead path in CR-08 sails through the real-PTY suite. This is the same
class of vacuous assertion WR-11 was raised about.
**Fix:** assert the transition, not the state: `mustSee` `"☑ Force SSH"` first (seed an
identity whose rewrite block exists once CR-09 is fixed), then click + space, then assert
`"☐ Force SSH"` — and add the reverse leg.

### WR-30: `ConfigureGit.Name` and the status note still read `m.selected` live after WR-20

**File:** `internal/tuikit/identities.go:1716-1721`
**Severity:** WARNING
**Issue:** WR-20 correctly moved `GitName`/`GitEmail`/`MatchStrategy` onto
`m.gitCommitSpec`, so five of six payload fields now come from the committed spec — but
`Name: m.selected` (the field that decides WHICH identity row the reducer mutates) and
the note text still read the live model. `m.gitCommitSpec.Identity` is the committed
identity and is already captured at `:2052-2056`. Today the `gitCommitPending` early
return (`:2042-2044`) prevents `m.selected` from moving, so this is latent rather than
live — but it leaves the one field with the largest blast radius on the code path WR-14
and WR-20 were both raised to eliminate.
**Fix:** `Name: spec.Identity` and `note: 'Git identity "' + spec.Identity + '" configured.'`.

### WR-31: the match-strategy option copy contradicts the gitdir default it describes

**File:** `internal/tuikit/identities.go:613-621` (`strategyCopy`), `:696-701` (`gitDirFor`)
**Severity:** WARNING
**Issue:** `strategyCopy("gitdir", name)` renders `gitdir (default) — applies inside ~/<name>/`
while the value actually written is `~/git/<name>/` (D-07, and now `gitDirFor`'s live
default). Confirmed in a rendered wizard frame:

```
● gitdir (default) — applies inside ~/acme2/
┊ [includeIf "gitdir:~/acme2/"]        ← dummy backend; the real backend writes ~/git/acme2/
```

The real backend's `matchesFor` (`wiring.go:1914-1922`) uses `spec.GitDir`, so the option
label understates the path by one segment on the live binary. This predates the fix loop
but is now squarely the surface CR-03/CR-07 exist to make truthful.
**Fix:** derive the copy from the same source as the write:

```go
func strategyCopy(strategy, name, gitDir string) string {
    case "gitdir":
        return "gitdir (default) — applies inside " + gitDir
```

Also make the stub/fixture backends honour `spec.GitDir` in `IncludeIfPreview`
(`internal/dummytui/fixturebackend.go:164-166`, `internal/tuikit/backend_stub_test.go:256-258`)
so the dummy and real previews cannot silently disagree about the gitdir.

### WR-32: Enter on the wizard's Force-SSH row still opens the write ceremony instead of toggling, and `handleEdit`'s `enter` branch is dead

**File:** `internal/tuikit/identities.go:2499-2518` (wizard `enter`), `:785-788` (`handleEdit`)
**Severity:** WARNING
**Issue:** CR-06 reported three symptoms; two are fixed. The third is not — Enter on
`gitFieldForceSSH` falls into the `default: // fields + Continue` arm and advances to the
review ceremony:

```
PROBE enter on ForceSSH: forceSSH=true step=3 configureGit=true   (want a toggle)
```

Meanwhile `handleEdit`'s `case gitFieldForceSSH` accepts `key == "enter"`, but both the
wizard (`:2499`) and the pane (`:2070`) intercept `enter` before `handleEdit` is ever
reached — so that branch is unreachable dead code that reads as if the behaviour were
implemented. A checkbox that jumps to a write ceremony on Enter is a hazardous default on
a confirmation-gated flow.
**Fix:** add `case gitFieldForceSSH: w.git = w.git.handleEdit(msg, w.gitFocus)` to the
wizard's `enter` switch (before `default`), do the same in `handleGitKey`, and delete the
now-genuinely-reachable-or-not `"enter"` clause accordingly.

### WR-33: stale cross-reference to the symbol WR-18 deleted

**File:** `e2e/git_configuration_pty_e2e_test.go:715`
**Severity:** WARNING
**Issue:** The comment still points readers at
`cmd/gitid/gate_visual_regression_test.go`'s `splitAllowlistLine`, which WR-18 deleted in
`f7b7745`. `grep -rn "splitAllowlistLine" cmd/ internal/ e2e/` now returns only this
comment. A reader following it finds nothing, and the comment implies a shared
implementation that no longer exists.
**Fix:** rewrite the comment to describe the parser inline, or drop the cross-reference.

### WR-34: `displayMessage`'s substring replace is fragile at both ends

**File:** `cmd/gitid/wiring.go:1985-1990`
**Severity:** WARNING
**Issue:** `strings.ReplaceAll(msg, b.home, "~")` fails open in two directions.
(a) `b.home` comes from `os.UserHomeDir()` (`:134`) and is never symlink-resolved, while
errors surfaced by `os`/`git`/`exec` can carry the resolved path — on a machine where
HOME is a symlink (`/home/u → /mnt/data/u`, or the `/tmp → /private/tmp` shape this
project's own sandboxes hit) nothing is scrubbed and the receipt shows absolute paths
again, exactly the WR-01/WR-23 symptom. (b) A sibling directory sharing the prefix
(`/Users/ramon` vs `/Users/ramonaldo`) is rewritten to `~aldo`, producing a path that
looks valid and is not.
**Fix:** anchor on a path separator and cover both spellings:

```go
func (b *realBackend) displayMessage(msg string) string {
    if msg == "" || b.home == "" {
        return msg
    }
    for _, root := range b.homeSpellings() { // b.home + its EvalSymlinks form, longest first
        msg = strings.ReplaceAll(msg, root+string(os.PathSeparator), "~"+string(os.PathSeparator))
        msg = strings.ReplaceAll(msg, root, "~")
    }
    return msg
}
```

Add a test with a symlinked sandbox HOME and one with a prefix-sharing sibling directory.

---

## Notes for the escalation

This is the third and final auto-fix iteration; the loop has now produced a regression in
each of iterations 1 → 2 → 3 in the same `internal/tuikit` focus/enum area (CR-04 → CR-06 →
CR-08) and a new one in `internal/screenshot` (WR-22 → WR-26). The common cause is
mechanical: **both regressions live in code that no gate can observe** — CR-08 because the
only tests touching Force-SSH routing call `handleEdit` directly or assert a pre-existing
state (WR-29), and WR-26 because the package is excluded from lint and from every test
target (WR-28). Closing WR-28 and adding the two behaviour-level tests named in CR-08 and
WR-29 would have caught all four. Recommend fixing WR-28 first, in a human-reviewed pass,
before any further automated iteration on this phase.

---

_Reviewed: 2026-08-25T05:40:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
_Iteration: 3 (re-review of 04-REVIEW-FIX.md, final loop iteration)_
