---
phase: 04-git-configuration-screen
reviewed: 2026-08-24T21:35:00Z
depth: standard
files_reviewed: 25
files_reviewed_list:
  - .gitignore
  - cmd/gitid/gate_visual_regression_test.go
  - cmd/gitid/wiring_test.go
  - cmd/gitid/wiring.go
  - e2e/create_flow_pty_e2e_test.go
  - e2e/git_configuration_pty_e2e_test.go
  - internal/doctor/checks/reserved_test.go
  - internal/dummytui/fixturebackend.go
  - internal/gitconfig/reader_test.go
  - internal/gitconfig/reader.go
  - internal/gitconfig/renderer_test.go
  - internal/gitconfig/renderer.go
  - internal/identity/loader_test.go
  - internal/identity/loader.go
  - internal/keygen/signers.go
  - internal/screenshot/createflow_regions.go
  - internal/screenshot/createflow_test.go
  - internal/screenshot/createflow.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/identities_test.go
  - internal/tuikit/identities.go
  - internal/tuikit/store.go
  - internal/tuikit/views.go
  - Makefile
findings:
  critical: 4
  warning: 13
  info: 0
  total: 17
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-08-24T21:35:00Z
**Depth:** standard
**Files Reviewed:** 25
**Status:** issues_found

## Summary

Phase 4 wires the Git leg of identity creation: default Git artifacts on create,
`includeIf` gitdir/hasconfig matching, a provider-level `insteadOf` rewrite, a
standalone Configure-Git flow, and a shared `mutationJournal` transaction with
rollback. The transaction design (snapshot → mutate → restore-in-reverse) is
sound in outline, and the containment/symlink guard (`containedRegularPath`) is
a real improvement over the legacy path.

The implementation nonetheless ships four defects that reach the user's real
filesystem or the truth of the confirm ceremony. Two were reproduced with
executable probes against the real backend:

1. The user-editable **gitdir** value is fed straight into `os.Chmod` — entering
   `~` in that field permanently changes the user's HOME to `0700`
   (**reproduced**, evidence below). No preview, no confirmation, no restore on
   success.
2. Rollback in `commitCreateTransaction` **deletes every timestamped backup it
   took**, including in the case where the in-memory restoration itself failed
   (**reproduced**), which is exactly the case the backups exist for. This
   directly contradicts `mutationJournal`'s own doc comment ("backups remain
   durable safety artifacts") and `CLAUDE.md`'s backup requirement.
3. The wizard's Git-step preview and the bytes actually written **disagree** on
   the `gitdir` condition — evidenced by frames already committed to
   `.planning/.../ui-frames/`.
4. The Git form's field enum and its button-ring enum **collide numerically**, so
   keystrokes aimed at the Skip button silently edit a hidden path field that
   feeds the `includeIf`.

Beyond correctness, the phase weakened two visual-regression gates
(`RequiredRegions` downgraded, a blanket `\d{6,}` normalizer), retained ~200
lines of dead legacy transaction code behind a `//nolint:unused` + package-level
`var _`, and left the file-write transactions unserialized against each other.

## Critical Issues

### CR-01: User-editable gitdir path is chmod'ed to 0700 — including HOME

**File:** `cmd/gitid/wiring.go:830-858` (`mutationJournal.ensureDir`), `cmd/gitid/wiring.go:958-964`, `cmd/gitid/wiring.go:1018-1024`
**Severity:** BLOCKER

**Issue:** `commitGitArtifacts` resolves the user-typed gitdir
(`spec.GitDir`, editable via the "gitdir path" field / `ctrl+g` / mouse click)
and hands it to `journal.ensureDir(gitDirPath, 0o700)`. `ensureDir` ends with an
unconditional `os.Chmod(path, mode)` on the *final* component, whether or not
this transaction created it. Any pre-existing directory the user names is
silently tightened to `0700`, and the mode is **only** restored on rollback —
on success it stays changed forever.

Because `normalizeGitDir` appends a trailing slash (`identities.go:596-605`),
typing a bare `~` becomes `~/`, which `resolveKeyPath` expands to HOME itself,
which `containedRegularPath` accepts (`cleanPath == cleanRoot`), which
`ensureDir` then chmods.

Reproduced against the real backend (`commitGitTransaction`, temp HOME):

```
PROBE: ~/Documents mode after commit = -rwx------ (was 0755)   # GitDir "~/Documents/"
PROBE: err = <nil>
PROBE: HOME mode after commit = -rwx------ (was 0755)          # GitDir "~/"
```

None of this appears in the ceremony's disclosed `Targets`
(`identities.go:1860`), so the user confirms three files and gets a fourth,
silent, irreversible permission mutation.

**Fix:** Do not chmod directories the transaction did not create, and never
chmod a path that resolves to HOME (or any ancestor of the managed roots):

```go
func (j *mutationJournal) ensureDir(path string, mode os.FileMode) error {
	clean := filepath.Clean(path)
	if clean == filepath.Clean(j.b.home) {
		return fmt.Errorf("gitid: refusing to manage the home directory itself: %s", path)
	}
	// ... walk-up / create loop unchanged ...
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], mode); err != nil { ... }
		j.createdDirs = append(j.createdDirs, missing[i])
		if err := os.Chmod(missing[i], mode); err != nil { ... }
	}
	// REMOVED: unconditional os.Chmod(path, mode) on a pre-existing directory.
	return nil
}
```

Additionally reject a gitdir that is not strictly *below* `~` (require at least
one path component), and surface "create directory `<path>`" in the ceremony's
`Targets` so the mutation is confirmed rather than assumed.

---

### CR-02: Rollback deletes every timestamped backup, including after a failed restore

**File:** `cmd/gitid/wiring.go:1805-1819` (`commitCreateTransaction.fail`)
**Severity:** BLOCKER

**Issue:** `fail()` runs `journal.restore()` and then unconditionally removes
every backup the transaction produced:

```go
outcomes, restoreErr := journal.restore()
for _, backup := range journal.backups {
	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) { ... }
}
```

If `restore()` partially failed (`restoreErr != nil`, e.g. a read-only mount, a
disk-full condition, or an `os.Chmod`/`WriteNoBackup` error mid-loop), the
file on disk is now in an unknown, half-written state **and** the only durable
copy of the original content has just been deleted. This is precisely the
data-loss scenario the `CLAUDE.md` backup rule exists to prevent, and it
contradicts `mutationJournal`'s own doc comment (`wiring.go:750-752`): "Its
rollback intentionally restores from in-memory snapshots, not by moving
timestamped backups: **backups remain durable safety artifacts**."

The two transaction entry points also disagree: `commitGitArtifacts.fail`
(`wiring.go:995-1005`) keeps its backups; `commitCreateTransaction.fail`
deletes them.

Reproduced (injected failure at `host-block`, pre-existing `~/.ssh/config`):

```
PROBE ~/.ssh after rollback: [config]
PROBE: NO .bak.* backup remains after rollback
```

**Fix:** Never delete backups on the failure path. At most, delete them only
when `restoreErr == nil` *and* a post-restore content comparison confirms every
file matches its snapshot — and even then, prefer keeping them:

```go
outcomes, restoreErr := journal.restore()
message := fmt.Sprintf("gitid: mutation %s failed: %v; restoration results: %s",
	target, cause, strings.Join(outcomes, "; "))
if restoreErr != nil {
	// Restoration itself failed: the backups are the ONLY recovery path.
	message += "; timestamped backups retained: " + strings.Join(journal.backups, ", ")
}
return journal.backups, fmt.Errorf("%s", message)
```

---

### CR-03: Wizard Git preview shows a different gitdir than the one written

**File:** `internal/tuikit/identities.go:1451-1456` (`finishIdentity`), `internal/tuikit/identities.go:565-569` (`newGitForm`)
**Severity:** BLOCKER

**Issue:** Two independent bugs combine into a preview/write divergence on the
project's core WYSIWYG contract:

1. `newGitForm(b, name, email, strategy)` seeds the gitdir input from the Git
   **display name**, not the identity name:
   `gitDir: newTextInput("~/git/" + name + "/")`. In the wizard, `newWizard`
   calls `newGitForm(b, "Acme Identity", ...)`, so the default gitdir becomes
   `~/git/Acme Identity/` — a path with a space, derived from a human name.
2. `finishIdentity` computes `gitSpec := w.gitSpec()` and then **discards**
   `gitSpec.GitDir`, hardcoding `id.GitDir = "~/git/" + name + "/"`.

The preview the user reads therefore comes from (1) while the bytes written
come from (2). This is already visible in captured frames committed to the
repo:

```
.planning/phases/05.7-.../ui-frames/create-flow-test-stage-pass.txt:21
  │ ┊ [includeIf "gitdir:~/git/Acme Identity/"]              ┊
```

…while `CommitCreate` writes `[includeIf "gitdir:~/git/acme/"]`.

The same function also contains two consecutive `if w.configureGit {` blocks
that should be one.

**Fix:** Seed the gitdir from the identity name, not the author name, and honour
the user's edited value on write:

```go
// newGitForm — take the identity separately from the author display name.
func newGitForm(b Backend, identity, name, email, strategy string) gitForm {
	...
	gitDir: newTextInput("~/git/" + identity + "/"),
}

// finishIdentity — one block, and the user's value survives.
if w.configureGit {
	gitSpec := w.gitSpec()
	id.GitDir = gitSpec.GitDir
	id.ForceSSH = gitSpec.ForceSSH
	id.PublicKeyPath = gitSpec.PublicKeyPath
	id.State = "complete"
	id.GitConfigured = true
	...
}
```

---

### CR-04: Git form field enum collides with the button-ring enum — keystrokes hit the wrong control

**File:** `internal/tuikit/identities.go:497-522`, `internal/tuikit/identities.go:2379`, `internal/tuikit/identities.go:1924-1930`, `internal/tuikit/identities.go:2581-2584`
**Severity:** BLOCKER

**Issue:** The two `const` blocks now overlap numerically:

```
gitFieldName=0  gitFieldEmail=1  gitFieldStrategy=2  gitFieldForceSSH=3  gitFieldGitDir=4
gitFocusBack   = iota + gitFieldStrategy + 1 = 3
gitFocusSkip   = 4
gitFocusContinue = 5
gitPaneFocusButton = gitFieldStrategy + 1 = 3
```

Consequences in the wizard's Git step, whose `default:` branch is
`w.git.handleEdit(msg, w.gitFocus)` (`identities.go:2379`):

- With focus on **Back** (`gitFocus == 3`), `space`/`enter` reach
  `case gitFieldForceSSH` and toggle the Force-SSH checkbox.
- With focus on **Skip Git** (`gitFocus == 4`), every printable keystroke
  reaches `case gitFieldGitDir` and edits the gitdir text input — a field the
  wizard **never renders** (the "gitdir path" row exists only in the
  configure-Git pane, `identities.go:3393-3395`). The invisible value feeds
  `matchesFor()` and the `includeIf` preview.

In the configure-Git pane, `gitPaneFocusButton == gitFieldForceSSH == 3` means a
mouse click on the "Force SSH" row (`gitFormFieldSlots`) focuses the **Write
it…** button; `enter` there opens the write ceremony instead of toggling.

**Fix:** Give the button ring its own non-overlapping numbering (or a distinct
type), and gate `handleEdit` on field slots only:

```go
const (
	gitFocusBack = iota + gitFieldGitDir + 1 // first slot AFTER every field
	gitFocusSkip
	gitFocusContinue
	wizardGitFocusSlots
)

// wizard step 2, default branch:
default:
	if w.gitFocus <= gitFieldStrategy { // never route a button slot into a field
		w.git = w.git.handleEdit(msg, w.gitFocus)
	}
```

Add a compile-time guard so the two rings can never overlap again:

```go
var _ = [1]struct{}{}[boolToInt(gitFocusBack <= gitFieldGitDir)] // fails to compile on overlap
```

## Warnings

### WR-01: Standalone Git receipt prints raw absolute backup paths

**File:** `cmd/gitid/wiring.go:730-741` (`CommitGit`), `cmd/gitid/wiring.go:746-748`
**Issue:** `CommitGit` returns `journal.backups` verbatim, while
`commitCreateTransaction` maps them through `b.displayPath` (`wiring.go:1914-1917`).
The Git ceremony therefore renders full absolute paths that wrap across three
terminal rows. Captured evidence
(`ui-frames/git-configuration-mouse-field-focus.txt`):

```
│ Wrote → ~/.gitconfig
│ Backed up →
│ /var/folders/5w/.../001/.gitconfig.bak.17
│ 87620713903758000
```

The same raw-path leak appears in the rollback error string
(`wiring.go:1814`), which concatenates every restoration outcome — each with a
full absolute path — into one unbounded message.
**Fix:** `backups = append(backups, b.displayPath(backup))` before returning from
`commitGitTransaction`, and use `b.displayPath` in the `fail()` message builder.

### WR-02: Dead branch in `commitCreateTransaction.fail` — restore failure is indistinguishable

**File:** `cmd/gitid/wiring.go:1814-1818`
**Issue:** Both arms of `if restoreErr != nil { ... } return ...` construct and
return the identical `fmt.Errorf("%s", message)`. `restoreErr` is computed and
then effectively unused, so a caller cannot tell "rolled back cleanly" from
"rollback itself failed" — the single most important distinction on this path.
**Fix:** Wrap `restoreErr` into the returned error (see CR-02's snippet) or
delete the branch.

### WR-03: ~200 lines of dead legacy transaction kept alive to defeat the linter

**File:** `cmd/gitid/wiring.go:118`, `cmd/gitid/wiring.go:1594-1803`
**Issue:** `commitCreateTransactionLegacy` is superseded by
`commitCreateTransaction` and is referenced only by
`var _ = (*realBackend).commitCreateTransactionLegacy` plus a
`//nolint:unused` directive — i.e. two suppressions whose only purpose is to
keep dead code compiling. It duplicates the rollback semantics (with the *old*
`os.Rename(backup, target)` model) and will silently rot out of sync with the
journal.
**Fix:** Delete `commitCreateTransactionLegacy`, the `var _` reference, and the
`//nolint:unused`. If any behaviour there is still needed, move it into the
journal with a test.

### WR-04: File transactions are not serialized against each other

**File:** `cmd/gitid/wiring.go:88` (`mu`), `cmd/gitid/wiring.go:730-741`, `cmd/gitid/wiring.go:1557-1580`
**Issue:** `b.mu` guards only `staged*`/`stage*Outcome`/`persistErr`. Both
`CommitCreate` and `CommitGit` return `tea.Cmd`s that Bubble Tea executes in
separate goroutines, and both perform read-modify-write cycles on
`~/.gitconfig` (`WriteIncludeIf`, `WriteProviderRewrite`,
`SetAllowedSignersFile`) and `~/.ssh/allowed_signers`. Two overlapping
transactions interleave into a lost update, and both journals snapshot the same
pre-state, so rollback would resurrect a stale file.
**Fix:** Add a dedicated `txMu sync.Mutex` held for the whole of
`commitCreateTransaction` / `commitGitTransaction`.

### WR-05: Unchecking "Force SSH" silently does nothing

**File:** `internal/gitconfig/renderer.go:180-187`, `cmd/gitid/wiring.go:1050-1059`
**Issue:** `WriteProviderRewrite(..., enabled=false)` validates the provider and
then returns `("", nil)` without touching the file, and `commitGitArtifacts`
skips the step entirely when `!spec.ForceSSH`. In the edit flow the checkbox is
pre-populated from the identity's stored `ForceSSH`, so a user who unchecks it
and confirms sees a success receipt while the `[url ...] insteadOf` block stays
in `~/.gitconfig`. The rationale (shared provider block, don't remove another
identity's rewrite) is sound; the silence is not.
**Fix:** Render the checkbox as informational when a shared rewrite exists, or
surface an explicit note in the ceremony
("Force SSH off: the shared `provider-rewrite:github.com` block is left in place
because other identities use it").

### WR-06: Redundant full-file rewrite of `~/.gitconfig` purely to force a backup

**File:** `cmd/gitid/wiring.go:1060-1075`
**Issue:** The `allowed-signers-file-backup` step reads `~/.gitconfig` and writes
the *identical bytes* back through `filewriter.Write` just to obtain a second
backup path — moments after `WriteIncludeIf` already backed the same file up. A
single Configure-Git write therefore produces 3-4 `.bak.<nanos>` files (visible
in `ui-frames/git-configuration-mouse-field-focus.txt`: two `.gitconfig.bak.*`
in one receipt), and adds an extra non-atomic-window rewrite of the user's
config for no new information.
**Fix:** The journal already holds the pre-transaction snapshot
(`journal.file(b.gitconfigPath)`); take exactly one backup for `~/.gitconfig`
at the start of the Git phase and reuse it, instead of one per mutation step.

### WR-07: `gitDir` caret is left at column 0 after `SetValue`

**File:** `internal/tuikit/identities.go:1847`
**Issue:** `m.gitPaneForm.gitDir.SetValue(orDefault(sel.GitDir, "~/git/"+sel.Name+"/"))`
is not followed by `CursorEnd()`. This is the exact defect the codebase already
documents and fixes for hostname/port (`identities.go:270-277`: "textinput.SetValue
only re-homes the caret when the field was EMPTY … the user's first Backspace
would delete nothing and their typing would PREPEND"). Here the field is
non-empty at construction (`newTextInput("~/git/"+name+"/")`), so the caret
never moves.
**Fix:** `m.gitPaneForm.gitDir.CursorEnd()` after `SetValue`. Note the e2e
mouse test (`git_configuration_pty_e2e_test.go:433-437`) only asserts
`mustSee "extra"`, which passes for both prepend and append — it does not pin
the resulting path.

### WR-08: Visual-regression gate weakened rather than the divergence fixed

**File:** `internal/screenshot/createflow.go:225-232`, `internal/screenshot/createflow.go:271-287`
**Issue:** Two specs had their `RequiredRegions` downgraded to
`RegionKeybar` — `create-flow-ssh-form` from `RegionFormFields`, and
`git-form-demo` from `RegionContinueDisabledReason` (the region was deleted
outright from `createflow_regions.go`). The form-fields divergence is now
recorded as an accepted `ux-improvement` disposition instead of being gated.
The net effect is that the gate no longer fails when the real binary's SSH form
fields drift from the approved design.
**Fix:** If the divergence is genuinely approved, keep the region in
`RequiredRegions` and pin it with a narrow `contains:`/`absent:` allowlist
predicate (the mechanism `parseAllowlist` already enforces) rather than removing
it from the mandatory set.

### WR-09: Blanket `\d{6,}` normalizer can mask real divergences

**File:** `internal/screenshot/createflow.go:48-66`
**Issue:** `normalizeTimestamps` now replaces *any* run of six or more digits
with `<digits>` before comparison, justified as covering wrapped `UnixNano`
backup suffixes. It also erases any other long numeric content (byte counts,
key sizes, future numeric IDs) from both the real and dummy captures, so a
genuine real-vs-dummy numeric difference is normalized into equality.
**Fix:** Anchor the pattern to the backup suffix it targets, e.g.
`\.bak\.\d+` plus an explicit wrapped-fragment rule, instead of a bare
`\d{6,}` over the whole frame.

### WR-10: `extractGitCeremony` start marker is over-broad

**File:** `internal/screenshot/createflow_regions.go` (`extractGitCeremony`)
**Issue:** The region starts at the first right-pane line containing
`"Write Git identity"` **or** the bare substring `"configured"`. Any other line
mentioning "configured" (e.g. the sidebar note
`"no Git identity configured for this alias"`, or a future "Not configured"
status) starts the region early and shifts the whole comparison. The sibling
extractors in this same file were just hardened against exactly this class of
over-broad marker (`"ssh "` → `"ssh -"`, lines 472-483).
**Fix:** Match the full receipt shape, e.g.
`strings.Contains(rpPlain, "Write Git identity") || strings.Contains(rpPlain, `" configured — applies via"`)`.

### WR-11: `CaptureGitScreenScreens` degrades silently when the backend has <2 identities

**File:** `internal/screenshot/createflow.go` (`CaptureGitScreenScreens`)
**Issue:** The doc comment states "backend must expose at least two
identities", but nothing checks it. With a single identity, `keyDown` is a no-op
and `git-form-empty` captures the *same* identity as `git-form-filled`; the
completeness loop only asserts non-emptiness, so the gate passes while silently
losing the SSH-only checkpoint.
**Fix:** Assert the precondition before scripting:

```go
if len(backend.InitialState().Identities) < 2 {
	return nil, fmt.Errorf("screenshot: CaptureGitScreenScreens requires >= 2 seeded identities, got %d", n)
}
```

and additionally assert `out["git-form-empty"] != out["git-form-filled"]`.

### WR-12: `make test-e2e` writes non-deterministic frames into the tracked `.planning/` tree

**File:** `e2e/git_configuration_pty_e2e_test.go:238`, `:309`, `:385`, `:446` (via `e2e/ui_pty_e2e_test.go` `saveFrame`)
**Issue:** Phase 4 adds four more `saveFrame` calls that `os.WriteFile` into
`.planning/phases/05.7-.../ui-frames/`, a tracked directory. The written frames
embed absolute sandbox paths
(`/var/folders/5w/.../TestGitConfiguration_RealPTYMouseFieldFocus2762737905/001/...`)
and nanosecond backup suffixes, so every run on every machine produces a
different file and dirties the working tree. This is the same failure mode
`TestGateVisualRegressionReadOnly` was written to prevent for the sibling gate.
Confirmed on this working tree: six `ui-frames/*.txt` files show as modified
after an e2e run.
**Fix:** Write frames to `t.TempDir()` (or a gitignored `.gsd/`/`artifacts/`
path) and attach the path via `t.Logf`; publish approved frames through an
explicit make target, as the visual gate already does.

### WR-13: Dead code and a panicking slice in the gate test helper

**File:** `cmd/gitid/gate_visual_regression_test.go:831-860`
**Issue:** Three problems in adjacent lines:
- `hash, err := io.ReadAll(strings.NewReader("")); _ = hash` is pure dead code
  (and the only reason `io` is imported).
- `strings.TrimSpace(string(hashBytes))[:7]` panics with index-out-of-range if
  the ref file is shorter than 7 bytes.
- `min` and `currentGitCommit` are both unreferenced anywhere in
  `cmd/gitid`.

**Fix:** Delete `min`, `currentGitCommit`, and the `io` import; if the helper is
needed later, restore it with a length check:

```go
h := strings.TrimSpace(string(hashBytes))
if len(h) < 7 { return "unknown" }
return h[:7]
```

### WR-14: Reduced `ConfigureGit` state is rebuilt with an empty key path

**File:** `internal/tuikit/identities.go:1573-1578`
**Issue:** After a successful `GitCommitMsg`, the reducer rebuilds the spec with
`m.gitPaneForm.spec(m.selected, "")` — an empty `keyPath` — while the write
itself used `m.gitPaneForm.spec(sel.Name, sel.KeyPath)`. Since
`PublicKeyPath: orDefault(g.publicKeyPath, keyPath+".pub")`, an identity whose
`PublicKeyPath` is empty (no SSH block reconstructed) reduces to the literal
string `".pub"` in `DemoState`, disagreeing with what was actually written.
**Fix:** Capture the spec used for the commit on the model
(`m.gitCommitSpec = spec` at `ceremonyConfirmed`) and reuse it when reducing,
so the state can never describe a different write than the one performed.

### WR-15: `git config --file` values starting with `-` are parsed as git options

**File:** `internal/gitconfig/fragment.go:97-103` and `:124-132` (reached from `cmd/gitid/wiring.go:1039`)
**Issue:** Phase 4 newly routes user-typed `user.name`, `user.email`, and
`user.signingkey` values into `gitConfigSet`, which builds
`exec.Command("git", "config", "--file", path, key, value)`. `validateValue`
rejects only newlines and `[remote`, so a value beginning with `-` (e.g.
`--global`, `--unset-all`, `--type=bool`) is handed to git's option parser
rather than treated as a value. Impact is bounded (no shell, and `git config`
rejects a second config-file option), but it is argument injection into a
process gitid runs against the user's config, and it produces a confusing
mid-transaction failure rather than a validation error.
**Fix:** Reject leading `-` in `validateValue`, or terminate option parsing:

```go
cmd := exec.Command("git", "config", "--file", path, "--", key, value)
```

## Notes on scope

- `internal/gitconfig/renderer.go`'s `validProviderHostname`, `safeInline`, and
  `validSSHHasconfig` are sound: `%q` escaping plus the control-character reject
  closes the `includeIf`-injection path I probed for.
- `internal/identity/loader.go`'s tilde expansion for the fragment read is
  correct and correctly scoped (`acct.FragmentPath` stays verbatim).
- `internal/gitconfig/reader.go`'s `IsReservedBlockName` extension is correct;
  the `provider-rewrite:<host>` prefix is validated, not merely prefix-matched.

---

_Reviewed: 2026-08-24T21:35:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
