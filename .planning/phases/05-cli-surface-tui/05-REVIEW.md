---
phase: 05-cli-surface-tui
reviewed: 2026-06-12T00:00:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - cmd/gitid/copy.go
  - cmd/gitid/main.go
  - cmd/gitid/upload.go
  - internal/identity/validate.go
  - internal/upload/upload.go
  - tui/tui.go
  - tui/model.go
  - tui/messages.go
  - tui/keymap.go
  - tui/styles.go
  - tui/deps.go
  - tui/dashboard.go
  - tui/identitylist.go
  - tui/identitydetail.go
  - tui/createform.go
  - tui/updateform.go
  - tui/addaccountform.go
  - tui/prove.go
  - tui/copy.go
findings:
  critical: 4
  warning: 5
  info: 3
  total: 12
status: resolved
resolved_at: 2026-06-13
resolution: "CR-01..CR-04 + WR-01..WR-05 fixed in commit a8bdf86 (live TUI write/navigation flow wired end-to-end; new tui/wiring_test.go exercises the real program path). Info IN-01..03 left as tracked debt. Build/race tests/lint all green."
---

# Phase 5: Code Review Report

**Reviewed:** 2026-06-12
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Phase 5 adds the Bubble Tea v2 TUI (`tui/`) and finalizes the Cobra CLI surface
(`cmd/gitid/`). The **security posture is sound**: all `exec` calls use arg-slice
form with fixed/trusted args (no shell interpolation, G204-clean), all
`os.ReadFile`/`os.Stat` calls are constrained to home-relative gitid-managed
paths (G304), name validation is applied before filesystem access in the CLI
`copy` path, and the write path correctly routes through `identity.Deps` seams
rather than bypassing the `filewriter` chokepoint. No hardcoded secrets, no
`eval`, no v1-only lipgloss `NewRenderer` usage. `View()` correctly returns
`tea.View`; key handling uses `tea.KeyPressMsg`.

However, the **TUI write/navigation wiring is fundamentally broken**. Four
distinct Critical defects make the entire interactive create/update/add-account
flow either inert or crash-prone, and the real `identity.Deps` built at startup
never reaches any screen that performs a write. These are correctness blockers,
not style nits.

## Critical Issues

### CR-01: Pushed screens never get `init()` called — Prove-Before-Write phase 1 never starts

**File:** `tui/model.go:72-74` (and `tui/prove.go:68-70`)
**Issue:** The root model's `pushScreenMsg` handler appends the screen and returns
`m, nil` — it never invokes `init()` on the pushed `screenModel`. The prove screen
relies on `init()` to issue `runPreWriteCmd` (phase 1). Because nothing calls
`proveModel.init()` in the live program (only `dashboardModel.init()` is called,
and only for `stack[0]` in `rootModel.Init()`), a pushed prove screen sits in
`provePhase1Running` forever: phase 1 never runs, phase 2 never runs,
`confirmActive` never becomes true, and no write can ever occur. The spinner does
not even animate (no tick scheduled). The entire create/update/add-account flow is
dead in the actual TUI. Tests pass only because they call `m.init()` directly.
**Fix:** Have the push handler invoke `init()` when the pushed screen supports it.
Introduce an optional initializer interface and call it on push:
```go
type initializer interface{ init() (screenModel, tea.Cmd) }

case pushScreenMsg:
    next := msg.next
    var cmd tea.Cmd
    if in, ok := next.(initializer); ok {
        next, cmd = in.init()
    }
    m.stack = append(m.stack, next)
    return m, cmd
```

### CR-02: Real `identity.Deps` is never threaded to any form/prove screen — confirmed write nil-panics

**File:** `tui/model.go:40-46`, `tui/identitylist.go:39,82,85`, `tui/createform.go:83`, `tui/identitydetail.go:32`
**Issue:** `newRootModel` builds `tuiDeps{doctor: docDeps, identity: idDeps}` and stores
it in `rootModel.deps` (model.go:40), but `rootModel.Update`/`View` never read
`m.deps` again. The navigation chain only ever propagates `doctor.Deps`:
`dashboard → newIdentityListScreen(m.doctorDeps)` carries no identity deps;
`identityList → newCreateFormScreen()` builds the form with `tuiDeps{}` (createform.go:83);
`identityList → newIdentityDetailScreen(item.account)` drops deps entirely
(identitydetail.go:32 returns `identityDetailModel{account: acct}` with zero-value
`tuiDeps{}`). Consequently every prove screen receives `deps.identity == identity.Deps{}`
with all function fields nil. When the user confirms a write, `runWriteCmd` calls
`identity.Create(in, deps.identity)`, which immediately invokes `deps.Generate(in)`
on a nil func → **runtime panic / crash**. Even if it did not panic, no write
would route through the real filewriter-backed seams. The deps-carrying constructor
`newIdentityDetailModel` (identitydetail.go:24) is dead code, never called.
**Fix:** Thread `tuiDeps` (or at least `identity.Deps`) through the full navigation
chain. Pass `m.deps` from the root into the dashboard, then into
`newIdentityListScreen`, then into `newCreateFormScreen(deps)` and
`newIdentityDetailScreen(acct, deps)`. Delete the unused `newIdentityDetailModel`
or make `newIdentityDetailScreen` delegate to it with real deps.

### CR-03: Prove screen always calls `identity.Create`, ignoring update/add-account semantics

**File:** `tui/prove.go:96-103`
**Issue:** `runWriteCmd` unconditionally calls `identity.Create(in, deps.identity)`
regardless of `m.action` ("create" | "update" | "add-account"). The domain layer
has dedicated `identity.Update` and `identity.AddAccount` mode functions
(`internal/identity/modes.go:77,131`) with distinct semantics. Routing an "update"
or "add-account" through `Create` runs the full create-new pipeline: it calls
`deps.Generate` to mint a **brand-new SSH key**, re-runs key persistence, and
rewrites all four artifacts as if creating from scratch — overwriting/duplicating
the existing identity's key and config rather than editing in place. This is a
data-corruption / wrong-behavior blocker for the edit and add-host flows.
Additionally, the `CreateInput` built by the forms never sets `Algo`,
`FragmentPath`, `GitconfigPath`, `SSHConfigPath`, `AllowedSignersPath`, or
`GlobalBlock`, so even the create path writes to empty/zero paths.
**Fix:** Branch on `m.action` and dispatch to the correct mode function:
```go
switch m.action {
case "update":      _, err = identity.Update(existingAccount, deps.identity)
case "add-account": _, err = identity.AddAccount(existingAccount, newProvider, newAlias, deps.identity)
default:            _, err = identity.Create(in, deps.identity)
}
```
Carry the `identity.Account` (with resolved managed paths) into the prove screen
for the update/add-account cases, and populate the create-path target paths.

### CR-04: `proveModel.keyPath` is set from `SSHConfigPath`, which forms never populate — phase 1 tests an empty key path

**File:** `tui/prove.go:59` and `tui/createform.go:142-150`, `tui/updateform.go:110-118`, `tui/addaccountform.go:112-120`
**Issue:** `newProveScreen` sets `keyPath: input.SSHConfigPath` (prove.go:59), then
phase 1 runs `tester.PreWrite(m.keyPath, m.input.Hostname, m.input.Port)`. But
`SSHConfigPath` is the path to `~/.ssh/config`, not a private key — and none of the
three forms set `SSHConfigPath` or `Hostname` on the `CreateInput` they build
(they set only Name/GitName/GitEmail/Provider/Port/Alias). So in production phase 1
executes `tester.PreWrite("", "", port)`: an empty key path against an empty
hostname. The pre-write authentication gate is therefore meaningless — it cannot
exercise the real key, defeating the D-04 "prove before write" safety invariant.
(Masked at runtime today by CR-01, but it is an independent correctness defect and
becomes live once CR-01 is fixed.)
**Fix:** Populate `Hostname` and the staged/existing key path in the forms and pass
the actual private-key path (not the ssh config path) into `newProveScreen`. For
create, derive the staged key path from `keygen.KeyPaths`; for update/add-account,
use the existing `account.KeyPath`.

## Warnings

### WR-01: Dashboard spinners never animate (no tick scheduled)

**File:** `tui/dashboard.go:67-74` (and `tui/prove.go:68-70`)
**Issue:** `dashboardModel.init()` batches only the seven family cmds; it never
issues an initial `spinner.Tick` (e.g. `m.spinners[i].Tick`). The
`spinner.TickMsg` case (dashboard.go:149) only re-arms a tick if one already
arrived, so no spinner ever animates while families load. The prove screen has the
same gap (no spinner tick cmd anywhere). Purely cosmetic, but the loading UX is
broken.
**Fix:** Include the spinner Tick cmds in the Batch returned by `init()`:
`cmds = append(cmds, m.spinners[i].Tick)` (or the v2 equivalent) for each spinner.

### WR-02: `identityDetailModel.pubLine` is never populated — copy action copies empty string

**File:** `tui/identitydetail.go:20,70,82`
**Issue:** The `pubLine` field is documented as "cached public key line for the copy
action" but is never assigned anywhere in production. Pressing `c` runs
`runClipboardCopyCmd(m.pubLine)` with `pubLine == ""`, copying an empty string to
the clipboard, and the overlay renders an empty key preview. The detail screen
never reads the `.pub` file for the account.
**Fix:** Read the account's `.pub` (via the injected `ReadFile`/`PubExists` seam or
`os.ReadFile(acct.PubPath)`) when constructing the detail model, store the trimmed
line in `pubLine`, and guard the copy action when it is empty.

### WR-03: `familyResultMsg.err` is never set, so `familyError` state is unreachable

**File:** `tui/dashboard.go:97-103,118-121` and `tui/messages.go:32-37`
**Issue:** `makeFamilyCmd` constructs `familyResultMsg{runID, family, findings}`
and never sets `err` (the check fns return only `[]Finding`, never an error). The
`update` handler branches on `msg.err != nil` to set `familyError`, and `view()`
renders a "✗ check failed" panel for that state — but the branch is dead: a check
that panics inside the goroutine crashes the program rather than surfacing
`familyError`. The error-handling path is illusory.
**Fix:** Either drop the unused `err` field and `familyError` state, or wrap the
check call in `recover()` and convert a panic into `familyResultMsg{err: ...}` so
the error UI is actually reachable.

### WR-04: `addAccountFormModel` ignores the SSH Alias and Match Strategy fields it collects

**File:** `tui/addaccountform.go:30-35,98-123`
**Issue:** The form presents four editable fields (Provider, SSH Alias, Port, Match
Strategy) but `trySubmit` only consumes `inputs[0]` (Provider), `inputs[1]` (Alias),
and `inputs[2]` (Port). `inputs[3]` (Match Strategy, default `gitdir:~/git/<name>/`)
is collected and displayed but never read into the `CreateInput.Matches`, so the
user's gitdir scoping is silently discarded. Combined with CR-03, the whole add-host
write is wrong, but even in isolation this drops user input.
**Fix:** Parse `inputs[3]` into a `gitconfig.Match` and set `in.Matches`, and route
through `identity.AddAccount` (see CR-03).

### WR-05: `proveModel.backupPath` assigned from always-empty `writeResultMsg.backupPath`

**File:** `tui/prove.go:101,132-136` and `tui/messages.go:57-61`
**Issue:** `runWriteCmd` returns `writeResultMsg{err: err}` and never sets
`backupPath`; `identity.Create`'s `CreateResult` (which carries backup paths) is
discarded (`_, err := identity.Create(...)`). The prove screen then assigns
`m.backupPath = msg.backupPath` (always `""`). The safe-write invariant in
CLAUDE.md emphasizes timestamped backups; the TUI never surfaces the backup path to
the user, so a confirmed write gives no confirmation of where the backup went.
**Fix:** Capture `CreateResult` from `identity.Create`/`Update`/`AddAccount`, carry
its backup path(s) into `writeResultMsg`, and display them on success before popping.

## Info

### IN-01: Dead/unreachable branch in `ValidateName`

**File:** `internal/identity/validate.go:28-30`
**Issue:** `trimmed := strings.TrimSpace(name)` is computed first, so by line 28
`trimmed` is already whitespace-free; the `trimmed != name` branch only fires when
the *caller* passes untrimmed input. The CLI `copy` caller already calls
`sanitizeName` (TrimSpace) before `ValidateName` (`cmd/gitid/copy.go:64-65`), so the
branch is unreachable from that path. Harmless defense-in-depth, but the comment at
lines 26-27 is misleading ("already rejected above via empty check").
**Fix:** Either remove the redundant branch or correct the comment; if the goal is
to reject untrimmed input, validate `name` (not `trimmed`) against the regex.

### IN-02: `noArgsAction` `out` parameter is unused (placeholder)

**File:** `cmd/gitid/main.go:24,33`
**Issue:** `noArgsAction` accepts `out io.Writer` but only does `_ = out` with a
"reserved for future use" comment. Unused parameter; the `tui.Run` callback also
ignores `out`. Acceptable as a documented seam, but flagged for cleanliness.
**Fix:** Drop the parameter until it is needed, or wire TUI success output through it.

### IN-03: Duplicated `tuiRunSSHAdd`/`tuiRunSSHKeygenFingerprint` and deps wiring mirror cmd/gitid

**File:** `tui/deps.go:28-303` (mirrors `cmd/gitid/add.go`/`cmd/gitid/doctor.go`)
**Issue:** `buildIdentityDeps`, `buildTUIDoctorDeps`, `tuiRunSSHAdd`, and
`tuiRunSSHKeygenFingerprint` are near-verbatim copies of the cmd/gitid wiring,
acknowledged in comments as necessary because tui cannot import package main. This
is a maintenance hazard: a security or behavior fix in one copy (e.g. the recent
tighten-only `FixPerm` change) can silently diverge in the other.
**Fix:** Extract the shared deps-building logic into an internal package (e.g.
`internal/wiring`) that both `cmd/gitid` and `tui` import, eliminating the
duplication and the divergence risk. Out of strict v1 scope but worth tracking.

---

_Reviewed: 2026-06-12_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
