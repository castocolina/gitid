---
phase: 06-global-ssh-options
plan: 01
subsystem: sshconfig
tags: [tracer, globals, provenance]
requires:
  - phase: 05-identity-manager
    provides: [lifecycle chokepoint, mutation-journal rollback, ceremony architecture]
provides:
  - "internal/globalssh — D-01 probe set + D-10 policy table + proven provenance classes; never writes"
  - "sshconfig.EnsureGlobals — the ONE owner of the gitid Host * managed block (D-06), keyed global-ssh (D-08), last in its file (D-09)"
  - "cmd/gitid/lifecycle.go runGlobalSSHApply — the ONE global-SSH write ceremony, journal-backed, reused by 06-04 and 06-06"
  - "tuikit GlobalSSHPlanner seam + async apply ceremony whose heading names the resolved storage target (D-07)"
affects: [06-02 reserved-name registry, 06-03 option states and copy freeze, 06-04 simulation and ceremony, 06-05 storage tab, 06-06 CLI verb, 06-07 visual allowlist]
actuals:
  tokens: 43854
  tasks: 2
  commits: 5
tech-stack:
  added: []
  patterns: [single-owner EnsureGlobals, per-verb lifecycle ceremony, mutationJournal restore, tuikit DTO seam]
key-files:
  created:
    - internal/globalssh/doc.go
    - internal/globalssh/policy.go
    - internal/globalssh/probe.go
    - internal/globalssh/probe_test.go
    - internal/globalssh/classify.go
    - internal/globalssh/classify_test.go
    - internal/globalssh/isolation_contract_test.go
    - internal/sshconfig/globals.go
    - internal/sshconfig/globals_test.go
    - internal/sshconfig/directives.go
    - internal/sshconfig/directives_test.go
  modified:
    - internal/sshconfig/writer.go
    - internal/sshconfig/renderer.go
    - internal/sshconfig/renderer_test.go
    - internal/sshconfig/parser_test.go
    - internal/sshconfig/include.go
    - internal/identity/identity.go
    - internal/identity/rotate.go
    - internal/identity/repair.go
    - internal/identity/modes.go
    - cmd/gitid/lifecycle.go
    - cmd/gitid/lifecycle_test.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go
    - cmd/gitid/identity_create.go
    - internal/tuikit/views.go
    - internal/tuikit/backend.go
    - internal/tuikit/backend_stub_test.go
    - internal/tuikit/globalssh.go
    - internal/tuikit/globalssh_test.go
    - internal/tuikit/app.go
    - internal/dummytui/fixturebackend.go
    - Makefile
key-decisions:
  - "EnsureGlobals is the single globals-block owner; RenderGlobalBlock is deleted so no second renderer can reappear (D-06)."
  - "Placement follows the identity storage layout (D-07) and is forced last by ensureGlobalsLast, never by ReplaceBlock's in-place splice (D-09)."
  - "runGlobalSSHApply is the ONE write authority; Persist(ApplySSH) is re-read-only."
patterns-established:
  - "Pattern: one function per verb owns plan/confirm/backup/write and a mutationJournal; later plans extend that function rather than adding a second restore path."
  - "Pattern: tuikit receives rendered DTO strings (provenance labels, resolved target paths); backend enums stay in cmd/gitid."
requirements-completed: [GSSH-01]
duration: 50min
completed: 2026-08-26
status: complete
---

# Phase 06-01: End-to-end HashKnownHosts tracer Summary

**One live option (`HashKnownHosts`) now travels the whole stack — probed, labelled with a provenance gitid can prove, applied through one journal-backed ceremony, and written last into the same storage target the identities use.**

## Performance
- **Duration:** 50min (Task 2 + this summary in this session; Task 1 was already committed as `273c05c` / `43c2358` / `1ff5486`)
- **Tasks:** 2
- **Files modified:** 35 unique (11 created, 24 modified across the four implementation commits)

## Accomplishments
- gitid owns exactly one `Host *` managed block (`global-ssh`), shared by create, rotate, repair, and the global-SSH fix ceremony.
- The block lands in the resolved storage target and is the last gitid-managed block in that file.
- Applying the same option twice is byte-identical and still takes a new timestamped backup.

## Task Commits
1. **Task 1, commit (a): globalssh engine** - `273c05c` (feat)
2. **Task 1, commit (b): retarget call sites + apply ceremony** - `43c2358` (feat)
3. **Task 1, commit (c): tuikit seam** - `1ff5486` (feat)
4. **Task 2: placement/ordering** - `d4b07e3` (feat)

**Plan metadata:** `[this commit]` (docs: add plan summary)

## Files Created/Modified
- `internal/globalssh/*` — D-01 probe set, D-10 policy table, provenance classifier, isolation-contract proof
- `internal/sshconfig/globals.go` — `EnsureGlobals`, last-block reposition (`ensureGlobalsLast`)
- `internal/sshconfig/directives.go` — `ScanDirectives` (line-aware, Include-unaware)
- `internal/sshconfig/writer.go` / `renderer.go` — `Write` platform-argument contract; `RenderGlobalBlock` deleted
- `internal/identity/{identity,rotate,repair,modes}.go` — `GlobalsGOOS` empty-means-do-not-touch
- `cmd/gitid/lifecycle.go` — `runGlobalSSHApply` + journal
- `cmd/gitid/wiring.go` — `CommitGlobalSSH`, `GlobalSSHOptionStates`, `GlobalSSHApplyPlan`, `globalsTargetPath`
- `internal/tuikit/{backend,views,globalssh,app}.go` — planner seam + async ceremony
- `Makefile` — copy-freeze heading `Write Host * managed block to `

## Decisions Made
Followed the plan. The one placement rule that is not incidental: after composing, `EnsureGlobals` asserts the globals block's start offset is greater than every other gitid-managed block and, if not, removes and re-appends it. `filewriter.ReplaceBlock` updates in place, so an adopted legacy block (or a create that appended an identity after a prior globals write) would otherwise stay first.

## Isolation-contract observation (this machine)

`TestIsolatedConfigContract` ran the real `ssh` binary. Observed:

- with planted file (`ssh -G -F <temp>/config gitid-probe.invalid`): `hashknownhosts yes`
- isolated (`ssh -G -F /dev/null gitid-probe.invalid`): `hashknownhosts no`

Verified platform fact (already recorded in the test's doc comment): `ssh` resolves the per-user config from the passwd entry (`getpwuid`), not `$HOME`. The hermetic proof therefore feeds the planted file via `-F <file>`; seeding `$HOME` alone is invisible to `ssh -G`.

## IgnoreUnknown placement correction

The retired `RenderGlobalBlock` emitted `IgnoreUnknown UseKeychain` as the **first indented directive inside** `Host *`. `EnsureGlobals` emits it as the body's **first line, before** the `Host *` stanza — matching `recipes/ssh-config.recipe` and the frozen dummy. Unconditional on darwin AND linux (D-11). Pinned by `TestEnsureGlobalsGuardFirstOnEveryPlatform`.

## RenderGlobalBlock deletion

Deleted from `internal/sshconfig/renderer.go`. Call sites retargeted onto `EnsureGlobals` / `Write(..., globalsGOOS)`:

- `sshconfig.Write` — non-empty platform argument normalises through `EnsureGlobals`; empty means do not touch
- `internal/identity` create / rotate / repair / modes — `CreateInput.GlobalsGOOS`
- `cmd/gitid/identity_create.go` and `cmd/gitid/wiring.go` — pass `platform.CurrentOS()`
- `renderer_test.go` / `parser_test.go` fixtures rebuilt from `EnsureGlobals`; test names no longer embed the retired function

`rg -n 'RenderGlobalBlock' --glob '*.go' internal cmd e2e` is empty.

## Ceremony-heading copy divergence (D-07)

The apply ceremony heading is `Write Host * managed block to ` + `GlobalSSHApplyPlanView.Targets[0]`, so it names the file the write actually touches (Include'd `~/.ssh/config.d/gitid.config` or in-file `~/.ssh/config`). This is the same scoped-divergence shape as Phase-3 D-05 (resolved storage target, never a hardcoded path). Registered in `Makefile` `gate-copy-freeze` as `'Write Host * managed block to '`. Precedent named: Phase-3 D-05.

## mutationJournal wiring for `runGlobalSSHApply` (for 06-04 reuse)

Watched paths, in order:

1. `st.targetPath` — always (`journal.watchFile`)
2. when `st.needsIncludeLine`:
   - `b.sshConfigPath` (`~/.ssh/config`) — the floored Include line is a distinct file from the Include'd target
   - `b.includeDir` recorded via `recordCreatedDir` only if it did not pre-exist

Writes still go through `filewriter.Write` (timestamped backup + atomic temp→rename at 0600). Any error after the first write calls `journal.restore()`. A file that did not exist before the transaction is **removed**, never restored to empty.

Observed restore output from `TestRunGlobalSSHApplyInjectedFailureRestoresWatchedFiles` (failure injected at `global-ssh-write`, after the Include-line write on a fresh HOME):

```
err=gitid: applying global SSH options: injected failure after the include-line write
restored=["~/.ssh/config: restored" "~/.ssh/config.d/gitid.config: restored" "~/.ssh/config.d: restored"]
backups=[]
```

(`: restored` here is the journal's outcome verb for both "bytes put back" and "created path removed" — `TestRunGlobalSSHApplyFreshHomeRemovesCreatedConfigOnFailure` asserts the files are gone, not left empty.)

plan 06-04 EXTENDS this function and REUSES this journal. It must not add a second restore path.

## Real-vs-dummy globals-block text (REQUIRED for 06-07 Task 1 allowlist)

Quoted side by side. 06-07 Task 1 consumes this verbatim.

**`EnsureGlobals` (real writer, darwin + explicit `HashKnownHosts yes` — observed by calling `EnsureGlobals(nil, {"HashKnownHosts": "yes"}, "darwin")`):**

```
# BEGIN gitid managed: global-ssh
IgnoreUnknown UseKeychain

Host *
  HashKnownHosts yes
  AddKeysToAgent yes
  UseKeychain yes
# END gitid managed: global-ssh
```

**`managedHostStar` fixture (`internal/tuikit/globalssh.go`, applied=`["HashKnownHosts"]`):**

```
# BEGIN gitid managed: global-ssh
IgnoreUnknown UseKeychain

Host *
    HashKnownHosts yes
    UseKeychain yes
    AddKeysToAgent yes
# END gitid managed: global-ssh
```

Callouts for the 06-07 allowlist:

1. **Guard-line placement** — both now put `IgnoreUnknown UseKeychain` BEFORE `Host *` (the retired renderer had it indented inside the stanza; that is the documented behavior change above). This axis matches.
2. **Indent** — real uses two-space `hostIndent` (`internal/sshconfig/renderer.go`); the fixture uses a four-space literal (`"    "+o.Key`).
3. **Key sequence** — real renders `GlobalHostStarOrder` / `Policy` order, emitting only keys present in the merged map: `StrictHostKeyChecking`, `ForwardAgent`, `HashKnownHosts`, `IdentitiesOnly`, `AddKeysToAgent`, `UseKeychain`. The darwin dump above therefore shows `HashKnownHosts`, then `AddKeysToAgent`, then `UseKeychain`. The fixture appends applied keys in `GlobalSSHOptions` selection order, then always appends `UseKeychain yes` / `AddKeysToAgent yes` — so with `HashKnownHosts` applied the fixture order is `HashKnownHosts`, `UseKeychain`, `AddKeysToAgent`.

## Deviations from Plan
None — plan executed exactly as written.

Orchestrator fixes already committed in Task 1 `1ff5486` (not reintroduced here):

1. Two test call sites in `internal/tuikit/globalssh_test.go` used a stale `a, _ = a.Update(msg)` assignment that does not type-check against `(tea.Model, tea.Cmd)` — fixed to `model, _ := ...; a = model.(App)`.
2. `overlaidOptions` only overwrote `OneLiner` with the "Applied by gitid — " prefix, but `renderOptions` prefers non-empty `Explanation`, so an applied row's detail pane kept the pre-apply text. Fixed by also overwriting `Explanation`.

Task 2's fresh-HOME Include-line assertion compares the Include line against every gitid-managed block **other than** `ssh-include` (the Include line itself lives inside that reserved wiring block). The acceptance criterion's "smaller than that of any gitid block in the main file" is satisfied for identity/globals blocks; the include block is the floor that contains the line.

## Issues Encountered
None that changed the design. The existing-block D-09 test (`TestEnsureGlobalsRepositionsExistingBlockAfterIdentity`) failed on the Task-1 `EnsureGlobals` (in-place `ReplaceBlock` left globals first) and went green after `ensureGlobalsLast`.

## Next Phase Readiness
06-02 (depends_on 06-01) has the constants (`GlobalBlockName`, `LegacyGlobalBlockName`), the single owner, last-position, and the apply ceremony it needs. It still owns reserved-name registry consolidation and migration classification of the legacy block — 06-01 registered both names in `IsReservedBlockName` additively so a machine carrying `_global` is not reported as a phantom identity in the meantime.

---
*Phase: 06-global-ssh-options*
*Completed: 2026-08-26*
