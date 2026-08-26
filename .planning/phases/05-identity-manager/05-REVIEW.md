---
phase: 05-identity-manager
reviewed: 2026-08-26T00:00:00Z
depth: standard
files_reviewed: 26
files_reviewed_list:
  - Makefile
  - cmd/gitid/identity.go
  - cmd/gitid/identity_clone.go
  - cmd/gitid/identity_create.go
  - cmd/gitid/identity_delete.go
  - cmd/gitid/identity_key.go
  - cmd/gitid/identity_read.go
  - cmd/gitid/lifecycle.go
  - cmd/gitid/main.go
  - cmd/gitid/wiring.go
  - docs/cli-parity-matrix.md
  - internal/gitconfig/fragment.go
  - internal/gitconfig/renderer.go
  - internal/identity/delete.go
  - internal/identity/deleteplan.go
  - internal/identity/identity.go
  - internal/identity/inventory.go
  - internal/identity/loader.go
  - internal/identity/modes.go
  - internal/identity/repair.go
  - internal/identity/rotate.go
  - internal/identity/scan.go
  - internal/identity/validate.go
  - internal/keygen/archive.go
  - internal/keygen/signers.go
  - internal/sshconfig/include.go
  - internal/sshconfig/validation.go
  - internal/tuikit/backend.go
  - internal/tuikit/identities.go
findings:
  critical: 5
  warning: 13
  info: 0
  total: 18
status: resolved
resolution: see 05-REVIEW-FIX.md (2026-08-26) — all 18 findings fixed
  (CR-01..05, WR-01..13), each with a test-first RED/GREEN regression test
  and its own atomic commit. Two findings (WR-10, WR-11) and part of a third
  (WR-13) had no reproducible live divergence in the current codebase on
  investigation; the review's suggested code was still applied as a
  defensive consolidation, documented inline in 05-REVIEW-FIX.md. Final
  verification (go build, make lint, go test -race ./..., make test-e2e)
  all passed.
---

# Phase 5: Code Review Report

**Reviewed:** 2026-08-26
**Depth:** standard
**Files Reviewed:** 26 (production source; `*_test.go` and screenshot/dummy tooling read only for cross-reference)
**Status:** issues_found

## Summary

The single-lifecycle-chokepoint property holds structurally: `runRotate` /
`runRepair` / `runDelete` are the only write ceremonies, and every CLI verb and
TUI commit seam is a thin adapter over them. The `confirmationMode` enum does
fail closed (zero value = `confirmationRequired`, unknown value refused), the
CLI never mentions `confirmationAlreadyObtained` (source-level test enforces
it), and the mutation-journal / archive rollback design (created-file vs
watched-file disjointness, `onCreated` fired before any source removal,
`RemoveArchivedPair` containment check) is sound.

The defects are concentrated where wave 9's shared-key fix stopped and where the
new headless CLI surface bypasses guards the TUI has:

- **Shared-key protection is incomplete.** Wave 9 normalized account paths for
  the delete preview and the delete write, but the two remaining
  cross-identity path comparisons (`runRepair`'s fail-closed gate,
  `KeyActionFor`'s rotate/repair router) still compare an **absolute** path
  against **verbatim tilde** paths, so the same "shared key seen as unshared"
  bug survives in the key ceremonies. Worse, `gitid identity rotate` has **no
  shared-key gate at all** — it goes straight to `runRotate`, which MOVES the
  key pair into the archive.
- **The TUI's rotate/repair ceremony fabricates its two test stages** — it
  renders `✓ Stage 1 test passed.` without calling any backend seam.
- **The new CLI create verb never validates the identity name**, so a name
  containing `../` lands the fragment and the key pair on arbitrary in-home
  paths.
- **`identity delete` discards a failed `DeletePlan`** and deletes anyway with
  no disclosure, defeating the fail-closed preview the TUI enforces.

No CR-18-class injection was found in the allowed_signers / gitconfig / ssh-config
composition paths for values that pass through the existing validators — but
`AllowedSignersLine`, documented as "the write-time hard gate", only rejects a
comma (see WR-08), and the CLI never runs `identity.ValidateEmail` either.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: `gitid identity rotate` has no shared-key gate — it archives (moves) a key another identity depends on — BLOCKER

**File:** `cmd/gitid/lifecycle.go:164-279`, `cmd/gitid/identity_key.go:137`
**Issue:** `runRotate` never computes `SharedKeyOwners`. It calls
`identity.Rotate(acct, deps)`, which at `internal/identity/rotate.go:92` calls
`deps.ArchiveKeyPair(existing.KeyPath, existing.PubPath)` — the **MOVE**
primitive (`keygen.MoveKeyPairToArchive`). The only thing that keeps a shared
key out of `runRotate` is `KeyActionFor`'s `ownerCount > 1 -> repair` routing
(`internal/identity/state.go:320`), and that router exists **only in the TUI
path** (`identitiesModel.openKeyCeremony`). The CLI verb calls
`cliRotateInto -> b.runRotate` directly:

```go
res, lerr = cliRotateInto(b, name, policy)   // identity_key.go:137 — no routing, no gate
```

So `gitid identity rotate work --yes`, where `work` and `personal` share
`~/.ssh/id_ed25519_personal`, removes that key from disk and re-points only
`work` — `personal` is silently left with an `IdentityFile` pointing at a file
that no longer exists. This is the exact loss class `ErrRepairTargetShared`
exists to prevent on the repair side, and there is no test for it (`grep
SharedKeyOwners cmd/gitid/*_test.go` returns nothing).
Secondary consequence of the same missing gate: rotating an identity whose key
is missing produces `algoFromKeyPath("") == ""` and an archive of `""` rather
than the `repair` the classifier would have chosen.
**Fix:** put the gate inside `runRotate` (the chokepoint), not in each caller,
so both skins inherit it:

```go
// runRotate, after normalizeAccountForWrite:
if owners := identity.SharedKeyOwners(b.normalizedAccounts(), acct.KeyPath, name); len(owners) > 0 {
    return res, fmt.Errorf(
        "gitid: refusing to rotate %q: its key pair is also used by %s — use `gitid identity new-key %s` instead: %w",
        name, strings.Join(owners, ", "), name, identity.ErrRepairTargetShared)
}
if acct.KeyPath == "" || !fileExists(acct.KeyPath) {
    return res, fmt.Errorf("gitid: refusing to rotate %q: no current key pair to retire — use `gitid identity new-key %s`", name, name)
}
```

### CR-02: the shared-key comparison is still un-normalized at the two key-ceremony call sites — BLOCKER

**File:** `cmd/gitid/lifecycle.go:360`, `cmd/gitid/wiring.go:3136`
**Issue:** `normalizedAccounts()` was introduced in wave 9 precisely because
`b.accounts()` returns `KeyPath` verbatim from `Reconstruct` (usually the
recipe-shaped literal `~/.ssh/id_ed25519_<name>`), while every lifecycle
function's own `acct` has already been expanded to an absolute path by
`normalizeAccountForWrite`. Its doc comment (`wiring.go:2450-2463`) names both
sites it fixed — `buildDeleteDeps.Accounts` and `DeletePlan`'s `PlanDeps` — but
two comparisons were left on the raw list:

```go
// lifecycle.go:360 — privTarget is ABSOLUTE (derived from the normalized acct)
otherOwners := identity.SharedKeyOwners(b.accounts(), privTarget, name)
```

`SharedKeyOwners` compares with plain `==` (`internal/identity/delete.go:418`),
so for any recipe-shaped identity the absolute `privTarget` can never equal a
sibling's tilde `KeyPath`. `RepairKey`'s documented fail-closed refusal
(`ErrRepairTargetShared`) is therefore unreachable, and `repair` will
`filewriter.Write` a fresh private key over a path a sibling identity
references.

```go
// wiring.go:3136 — feeds KeyActionFor's rotate-vs-repair routing
ownerCount := len(identity.SharedKeyOwners(b.accounts(), acct.KeyPath, name)) + 1
```

Here both sides are verbatim, so it works only while every identity in the file
spells the path the same way; a config mixing a gitid-written absolute
`IdentityFile` with a recipe-written tilde one yields `ownerCount == 1` for a
genuinely shared key and routes the TUI to **rotate** — CR-01's destructive path.
**Fix:** use the normalized list (and a normalized account) at both sites:

```go
otherOwners := identity.SharedKeyOwners(b.normalizedAccounts(), privTarget, name)
...
acct = b.normalizeAccountForWrite(acct)
ownerCount := len(identity.SharedKeyOwners(b.normalizedAccounts(), acct.KeyPath, name)) + 1
```

Then add a regression test that seeds two identities with tilde `IdentityFile`
values pointing at one key and asserts (a) `KeyActionFor == "repair"`, (b)
`runRepair` returns `ErrRepairTargetShared`, (c) `runRotate` refuses (CR-01).

### CR-03: the TUI key ceremony fabricates its stage-1 and stage-2 test results — BLOCKER

**File:** `internal/tuikit/identities.go:2320-2329`
**Issue:** The rotate/repair ceremony's two "test" beats are hardcoded:

```go
case "stage1":
    m.keyCeremonyStage1 = TestResultView{Outcome: TestOutcomePass, Detail: "Stage 1 test passed."}
    m.keyCeremonyPhase = "stage2"
case "stage2":
    m.keyCeremonyStage2 = TestResultView{Outcome: TestOutcomePass, Detail: "Stage 2 test passed."}
    m.keyCeremonyPhase = "review"
```

No backend seam is invoked (contrast the create wizard, which dispatches
`backend.TestStage1/TestStage2` and reacts to `WizardStageMsg`).
`renderStageOutcome` then renders `✓ Stage 1 test passed.` in the healthy style
(`identities.go:1532`). The Identities tab carries **no** D-16 demo banner
(`realBackend.DemoBanner` returns `tab != TabIdentities`), so the user is shown
a green connectivity proof for a test that never ran, immediately before
authorizing a key rotation. `keyCeremonyResult` is likewise seeded to
`TestOutcomeReachableNotUploaded` (`identities.go:2240`) with no measurement, so
the grace-window hint is also unconditioned on reality.
This violates the phase's own `test → confirm → write` premise; the lifecycle's
`test` stage is explicitly advisory (`_, _ = b.deps.Resolved(acct.Alias)`), so
nothing else compensates.
**Fix:** either drive the two beats from real seams (add
`TestKeyStage1/TestKeyStage2` to `IdentityPlanner`, dispatch them as `tea.Cmd`
and route the result through `handleMsg` like `WizardStageMsg`), or — if the
real probe is out of scope for this phase — delete the fake stages and label the
beat honestly, e.g.
`m.keyCeremonyStage1 = TestResultView{Outcome: TestOutcomeReachableNotUploaded, Detail: "Not tested — the current key's reachability is not checked before a rotation."}`.
Silence is acceptable; a fabricated `✓` is not.

### CR-04: `identity create` never validates the identity name — in-home path traversal into arbitrary files — BLOCKER

**File:** `cmd/gitid/identity_create.go:141-219`
**Issue:** `createInputFromCreateFlags` validates only the host block
(`sshconfig.ValidateHostBlock(alias, hostname, port, keyForValidation)`).
`identity.ValidateName` — which exists for exactly this and restricts names to
`[A-Za-z0-9._-]` — is never called on this path (`grep ValidateName` shows only
`internal/identity/clone.go` and `internal/adopter`). `validateToken` rejects
whitespace, `*`, `?`, `!`, `,` and line breaks, but **not** `/` or `..`, so the
name flows unchecked into path composition:

```go
FragmentPath: filepath.Join(b.fragmentDir, name),   // identity_create.go:195
// and, via createInput/keygen.KeyPaths, into ~/.ssh/id_<algo>_<name>
```

`--name '../.bashrc'` resolves the fragment to `~/.bashrc`; `containedRegularPath`
only rejects paths **outside** `$HOME`, so the write is permitted and
`gitconfig.WriteFragment` runs `git config --file ~/.bashrc user.name ...`
against the user's shell rc. The key pair similarly lands at
`~/.ssh/.bashrc`-style locations. The same unvalidated `--git-email` /
`--git-name` reach `WriteFragment` without `identity.ValidateEmail`; they are
caught later by `gitconfig.validateEmail`, but only after the SSH block and key
have been written and must be rolled back.
**Fix:** validate at the flag boundary, before anything is derived:

```go
if err := identity.ValidateName(name); err != nil {
    return identity.CreateInput{}, tuikit.DemoIdentity{}, fmt.Errorf("gitid: identity create: %w", err)
}
if err := identity.ValidateEmail(strings.TrimSpace(flags.GitEmail)); err != nil {
    return identity.CreateInput{}, tuikit.DemoIdentity{}, fmt.Errorf("gitid: identity create: %w", err)
}
if err := identity.ValidateProvider(provider); err != nil { ... }
```

Additionally harden the chokepoint: make `containedRegularPath` (or a new
`containedManagedPath`) require containment in the specific managed root
(`b.fragmentDir` / `b.sshDir`), not merely in `$HOME` — that would have blocked
this class regardless of the caller.

### CR-05: `identity delete` swallows a failed `DeletePlan` and deletes with no disclosure — BLOCKER

**File:** `cmd/gitid/identity_delete.go:105-109`
**Issue:**

```go
if plan, perr := b.DeletePlan(name, string(scope)); perr == nil {
    if rerr := renderDeletePlan(...); rerr != nil { return rerr }
}
// perr != nil: silently fall through to the delete
```

`PlanDelete` is deliberately fail-closed — its doc comment states a plan that
failed to read a file "must never be indistinguishable from a legitimately small
plan" — and the TUI honors that (`refreshDeletePlan` blanks the plan, sets
`deletePlanErr`, and the confirm control is disabled). The CLI does the
opposite: on any plan error (unreadable scan source, unparsable gitconfig,
unknown provider host) it prints **nothing** and proceeds to the irreversible
`--all` delete. With `--yes` no human sees anything at all, which is precisely
the T-05-38 disclosure the code comment above it claims to satisfy.
**Fix:** fail closed, matching the TUI:

```go
plan, perr := b.DeletePlan(name, string(scope))
if perr != nil {
    return fmt.Errorf("gitid: refusing to delete %q: the delete plan could not be built: %w", name, perr)
}
if rerr := renderDeletePlan(cmd.OutOrStdout(), b, plan, "will delete"); rerr != nil {
    return rerr
}
```

## Warnings

### WR-01: `--all` delete accepts `yes` where the TUI demands the typed identity name — WARNING

**File:** `cmd/gitid/identity_delete.go:137-142` vs `internal/tuikit/identities.go:2765-2770`
**Issue:** For the identical irreversible everything-scope delete, the TUI
requires `FixDestructive{ConfirmWord: plan.Name}` (type the identity name), while
the CLI's interactive prompt accepts the generic `"yes"` used by every other
verb. The stronger gate is dropped exactly where the blast radius is largest.
**Fix:** in `confirmDelete`, require the name for `DeleteScopeEverything`:
`want := "yes"; if scope == identity.DeleteScopeEverything { want = name }`, and
say so in the prompt.

### WR-02: the everything-delete receipt claims "key removed" when the shared-key downgrade kept it — WARNING

**File:** `internal/tuikit/identities.go:2771`
**Issue:** `cfg.ResultMessage` is fixed at ceremony-build time:
`"... — SSH block, Git fragment, and key removed (backups kept)."` It is used
verbatim even when `plan.SharedKeyOwners` is non-empty, i.e. when D-12 kept the
key pair for a sibling. The confirm screen's hint says "kept", the receipt says
"removed" — the user is told their key is gone when it is not.
**Fix:**

```go
removedKey := "and key removed"
if len(plan.SharedKeyOwners) > 0 {
    removedKey = "(key kept — still used by " + strings.Join(plan.SharedKeyOwners, ", ") + ")"
}
cfg.ResultMessage = `Identity "` + plan.Name + `" deleted — SSH block, Git fragment ` + removedKey + ` (backups kept).`
```

### WR-03: `deletePlanPreview` is a second, contradicting delete preview — WARNING

**File:** `cmd/gitid/lifecycle.go:629-642`
**Issue:** `runDelete` builds its confirmation preview by hand instead of from
`identity.PlanDelete`, and unconditionally names the key pair:

```go
if acct.KeyPath != "" {
    targets = append(targets, b.displayPath(acct.KeyPath), b.displayPath(acct.PubPath))
}
```

There is no `keySurvives` consultation, so this preview promises to delete a key
the write will keep — the exact preview/write divergence R-11 and the shared
`deleteTargets` helper were built to make impossible. It is currently mostly
invisible (the CLI prompt closure ignores the `preview` argument, and the TUI
uses `confirmationAlreadyObtained`), which makes it a live trap for the next
caller that does render it.
**Fix:** derive it from the one plan — `plan, err := b.DeletePlan(name, string(scope))`
— and render with the same `renderDeletePlan` helper, or delete the function and
pass the rendered plan into `runDelete`.

### WR-04: rotate/repair can rewrite `~/.ssh/config` without watching it — incomplete rollback — WARNING

**File:** `cmd/gitid/lifecycle.go:403-430`, `cmd/gitid/wiring.go:2401-2415`
**Issue:** `rotateWatchPaths`/`repairWatchPaths` watch `b.storageTargetPath()`
but never `b.sshConfigPath`. `deps.WriteSSH` routes to `writeSSHBlock`, which
for `st.needsIncludeLine` calls `sshconfig.EnsureIncludeDir(b.includeDir)` and
`sshconfig.EnsureIncludeLine(b.sshConfigPath)` — both mutate paths the journal
never snapshotted (and the created `config.d` directory is not recorded via
`recordCreatedDir`). A mid-transaction failure therefore leaves the injected
Include line and the new directory behind, contradicting "a mid-transaction
failure restores every watched file to its pre-transaction bytes AND mode".
**Fix:** add `b.sshConfigPath` to both watch lists, and record `b.includeDir` as
a created directory when it does not pre-exist (same `os.Stat` +
`recordCreatedDir` shape `runRotate` already uses for the archive directory).

### WR-05: `keyOwners()` never labels tilde-path keys — the D-12 "in use by" warning disappears — WARNING

**File:** `cmd/gitid/wiring.go:2517-2530`
**Issue:** the map is keyed on the **verbatim** `acct.KeyPath`
(`owners[acct.KeyPath] = label`), but it is consumed with **absolute** paths:
`toReusableKeyViews(keys, b.keyOwners())` looks up `owners[k.Path]`, and
`keygen.ScanReusableKeys` builds `Path` from `filepath.Glob(filepath.Join(sshDir, ...))`.
For any recipe-shaped identity the lookup misses, so the reuse picker shows a
key already owned by another identity with an empty `InUseBy` label — the same
class of miss wave 9 fixed for delete, in the safety label that warns a user
before they point a second identity at an existing key.
**Fix:** `for _, acct := range b.normalizedAccounts() { ... }` (or key the map on
`expandTildeForHome(acct.KeyPath, b.home)`).

### WR-06: headless `identity create` and `identity clone` always write the provider URL rewrite — WARNING

**File:** `cmd/gitid/identity_create.go:213`, `cmd/gitid/identity_clone.go:162`
**Issue:** both set `ForceSSH: true` unconditionally, and no `--force-ssh` /
`--no-force-ssh` flag exists (see `docs/cli-parity-matrix.md`). Every headless
create/clone therefore writes
`[url "git@<provider>:"] insteadOf = https://<provider>/` into `~/.gitconfig`
(`gitconfig.WriteProviderRewrite`) — a **machine-global** rewrite of every HTTPS
clone URL for that provider, for all repositories, including ones unrelated to
gitid. In the TUI this is a user-visible toggle; `Reconstruct` even goes out of
its way (CR-09) never to *default* this value when reading. Writing it by
default on the CLI is an unrequested global side effect, and for clone it also
contradicts "copy the author fields, re-derive the rest".
**Fix:** add `--force-ssh` (default `false`, or default to the clone source's
`src.ForceSSH`) and pass it through instead of the literal `true`.

### WR-07: `refreshDeletePlan` silently drops the second plan's error — WARNING

**File:** `internal/tuikit/identities.go:2224-2226`
**Issue:**

```go
if everything, eerr := m.backend.DeletePlan(sel.Name, "everything"); eerr == nil {
    m.deleteChoiceOwners = everything.SharedKeyOwners
}
```

When the everything-scope plan fails while git-only succeeds, the scope-choice
screen renders with **no** shared-key note and no error — the user is not told
that the sibling-key disclosure could not be computed. Same fail-closed rule as
the primary plan (R-07) should apply.
**Fix:** set `m.deletePlanErr = eerr.Error()` (or a dedicated
`deleteChoiceOwnersErr` rendered inline) instead of ignoring `eerr`.

### WR-08: `AllowedSignersLine`'s "write-time hard gate" only rejects a comma — WARNING

**File:** `internal/keygen/signers.go:31-40`
**Issue:** the doc comment calls this "the write-time hard gate ... independent
of whether an upstream form/config validator already rejected the comma", but
the check is `strings.Contains(email, ",")` alone. A principal containing a
newline injects an entire additional `allowed_signers` line (an
attacker-chosen principal + key); a principal containing a space or `namespaces=`
silently changes the field layout. Today only `gitconfig.validateEmail` running
earlier in `WriteFragment` prevents that — i.e. the "independent" gate is not
independent, and the CLI does not run `identity.ValidateEmail` at all (CR-04).
**Fix:** make the gate self-sufficient:

```go
if email == "" || strings.ContainsAny(email, ",\n\r \t") || !strings.Contains(email, "@") {
    return "", fmt.Errorf("keygen: allowed_signers principal is not a single bare address (CR-18): %q", email)
}
```

### WR-09: `rotateDryRunCaveat` claims a copy-freeze registration that does not exist — WARNING

**File:** `cmd/gitid/identity_key.go:88-91`, `Makefile:278-313`
**Issue:** the comment states "It is registered in Makefile gate-copy-freeze;
reword it and the gate fails." The gate's string list does not contain the
sentence, and the gate only greps `internal/tuikit internal/identity` — it can
never see a constant in `cmd/gitid`. `grep -c "This dry run tests only" Makefile`
returns 0. A false verification claim is worse than no claim: the next editor
will reword the frozen sentence believing a gate protects it.
**Fix:** either add the literal to the `gate-copy-freeze` list **and** extend the
grep roots to include `cmd/gitid`, or delete the sentence from the comment.

### WR-10: headless clone re-derives a different gitdir than the pre-filled clone — WARNING

**File:** `cmd/gitid/identity_clone.go:160`
**Issue:** `cloneCeremonyInputs` hardcodes `GitDir: "~/git/" + in.Name + "/"`,
while `realBackend.ClonePrefill` — used by the TUI clone and by the very same
verb's `resolvePrefilledTUI` branch a few lines above — carries the derived
`gitDirFromMatches(in.Matches)`. Cloning a source whose gitdir is
`~/work/acme/` produces `~/git/<clone>/` headless and `~/work/acme/` in the
wizard: two different includeIf conditions from one command.
**Fix:** `GitDir: orDefault(gitDirFromMatches(in.Matches), "~/git/"+in.Name+"/")`
and drop the duplicated derivation.

### WR-11: dead assignment to `id.PublicKeyPath` in `cloneCeremonyInputs` — WARNING

**File:** `cmd/gitid/identity_clone.go:161` (overwritten at 170-176)
**Issue:** `PublicKeyPath: in.ReuseKeyPath + ".pub"` is set inside the struct
literal and then unconditionally reassigned by the `if in.ReuseKeyPath != ""`
/`else` block eight lines later. When `ReuseKeyPath` is empty the literal
produces the bare string `".pub"` — the exact value WR-14 in `identities.go`
was written to eliminate — before being replaced. Harmless today, a trap on the
next edit that reorders these blocks.
**Fix:** delete the field from the struct literal and keep only the explicit
if/else assignment.

### WR-12: `RemoveProviderRewrite` writes (and mints a backup) even when nothing changed — WARNING

**File:** `internal/gitconfig/renderer.go:225-245`
**Issue:** unlike the two equality-guarded removals in `identity.Delete`
(`if !bytes.Equal(updated, original)`), this function always calls
`filewriter.Write` with the `RemoveBlock` result. When the block is absent the
file is rewritten byte-for-byte and a spurious `.bak.<nanos>` is minted and
reported as a backup of the delete transaction — the doc comment's own
idempotence claim ("a second call leaves the file byte-identical to the first
result") is true of content but not of side effects.
**Fix:**

```go
composed := filewriter.RemoveBlock(existing, name)
if bytes.Equal(composed, existing) {
    return "", nil // nothing to remove — no write, no backup
}
```

### WR-13: `identity show` can never display an identity the inventory omits; dead local in `toIdentityRecord` — WARNING

**File:** `cmd/gitid/identity_read.go:122-129`, `:195-196`
**Issue:** `show` searches only the records built from
`identity.BuildInventory(...).Identities`; an account that `Reconstruct` returns
but `BuildInventory` drops (or a `BuildInventory` failure mode that returns
fewer identities) reports `no such identity` even though `runDelete`/`runRotate`
would resolve it via `findAccount` — the read and write surfaces disagree about
what exists. Adjacent, `gitDir, strategy := matchStrategyFor(acct)` followed by
`_ = gitDir` is dead code carrying a comment that explains why it is dead.
**Fix:** build the record set from the union of `b.accounts()` and the inventory
(keyed by name), as `buildIdentityRecords` already does for the account fields;
and change `matchStrategyFor` to return only `strategy` (add a separate accessor
if `gitDir` is needed later).

---

_Reviewed: 2026-08-26_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
