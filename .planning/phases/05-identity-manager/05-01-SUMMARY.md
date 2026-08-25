---
phase: 05-identity-manager
plan: 01
subsystem: cli-tui-identity-lifecycle
tags: [cobra, bubbletea, gitconfig, ssh-config, delete-lifecycle, json-api]

requires:
  - phase: 04-git-configuration-screen
    provides: the mutationJournal transaction pattern (watchFile/ensureManagedDir/restore) CommitGit established, reused here for CommitDelete's rollback
provides:
  - "identity.DeleteScope (git-only|everything) + identity.Delete's structural D-10 SSH skip"
  - "tuikit.Backend.CommitDelete + DeleteCommitMsg async delete seam"
  - "cmd/gitid's realBackend.runDelete — the ONE production delete writer both the TUI and the CLI call"
  - "cmd/gitid identity list/show — the frozen D-03 --json read surface"
  - "cmd/gitid's D-01 noun-verb command tree with flat aliases and reserved ssh/git/health/fix noun groups"
affects: [05-04-everything-scope-delete, 05-06-identity-planner, 05-07-full-delete-lifecycle, 05-08-remaining-cli-verbs, 05-09-read-surface-consumers]

actuals:
  tokens: 33373
  tasks: 3
  commits: 1

tech-stack:
  added: []
  patterns:
    - "identityVerb + newVerbCmd shared spec: one verb spec built once, consumed twice (noun child + flat alias) — never RunE delegation between the two command objects"
    - "Async ceremony + backend Commit<Verb> + Commit<Verb>Msg: CommitDelete/DeleteCommitMsg mirror CommitGit/GitCommitMsg exactly, so the pattern generalizes to every future lifecycle verb"
    - "runDelete as the single named production writer a later plan extends in place (signature-change, not a second function) — closes the two-writer-drift risk class before it can occur"

key-files:
  created:
    - cmd/gitid/identity.go
    - cmd/gitid/identity_delete.go
    - cmd/gitid/identity_read.go
    - cmd/gitid/identity_test.go
    - e2e/identity_manager_pty_e2e_test.go
    - e2e/identity_cli_e2e_test.go
  modified:
    - internal/identity/delete.go
    - internal/tuikit/backend.go
    - internal/tuikit/views.go
    - internal/tuikit/identities.go
    - internal/dummytui/fixturebackend.go
    - cmd/gitid/wiring.go
    - cmd/gitid/main.go

key-decisions:
  - "identity.Delete's scope-based signature (DeleteScope replacing keepKey bool) is a compile-time-checked refactor with no production caller yet — reversible, per the plan's own reversibility rating"
  - "runDelete is named for 05-07 to extend in place (signature + body), never a second, differently-named writer (R3-05)"
  - "MGR-02 taxonomy: two orthogonal axes (IdentityState x4, KeyState x4 = 8 labels), never eight mutually-exclusive rows; ClassifyState's collapse always folds key-used-both into complete (7 distinct collapsed values); Inventory.UnusedKeys is a third, separate collection, never joined into identity rows (review R-06, binding for 05-07/05-09)"
  - "Account.FragmentPath/KeyPath/PubPath are stored verbatim (often literal tilde) by Reconstruct; any caller turning them into a real filesystem path must expand against its own explicit home, never os.UserHomeDir()/$HOME (the WR-35 hermeticity lesson applied to path expansion)"

patterns-established:
  - "Pattern: identityVerb/newVerbCmd shared-spec CLI constructor (review R-15) — every future identity verb (05-08) should be added as a spec in identityVerbSpecs(), not a bespoke *cobra.Command pair"
  - "Pattern: DeleteCommitMsg-style async Backend seam (Commit<Verb> + <Verb>CommitMsg, gated by a <verb>CommitPending model field, dispatched from handleMsg only on success) — the template 05-06's CommitRotate/CommitNewKey should follow"

requirements-completed: [MGR-01, MGR-03, MGR-06, MGR-07, MGR-08, SHELL-01, SHELL-03]

coverage:
  - id: D1
    description: "Git-only delete from the real TUI removes the includeIf block + fragment, survives a binary restart, and leaves SSH/key/allowed_signers byte-identical"
    requirement: "MGR-06"
    verification:
      - kind: e2e
        ref: "e2e/identity_manager_pty_e2e_test.go#TestIdentityManager_DeleteGitOnly"
        status: pass
    human_judgment: false
  - id: D2
    description: "CLI --git-only and TUI ceremony call the same runDelete chokepoint, producing byte-identical ~/.gitconfig"
    requirement: "SHELL-03"
    verification:
      - kind: e2e
        ref: "e2e/identity_manager_pty_e2e_test.go#TestIdentityManager_CLIAndTUIProduceByteIdenticalGitconfig"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestRunDeleteAndCLIVerbProduceByteIdenticalGitconfig"
        status: pass
    human_judgment: false
  - id: D3
    description: "CLI --all and TUI everything option refuse identically via identity.ErrScopeNotAvailable"
    requirement: "MGR-06"
    verification:
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestCLIAllAndTUIEverythingRefuseIdentically"
        status: pass
    human_judgment: false
  - id: D4
    description: "identity list/show render TTY table, piped tab-delimited, and frozen --json ({identities, unused_keys}) with correct two-axis health + empty Git fields for SSH-only identities"
    requirement: "MGR-01"
    verification:
      - kind: unit
        ref: "cmd/gitid/identity_test.go#TestIdentityRecordEightLabelsAcrossAxes"
        status: pass
      - kind: unit
        ref: "cmd/gitid/identity_test.go#TestIdentityRecordSSHOnlyEmptyGitFields"
        status: pass
      - kind: e2e
        ref: "e2e/identity_cli_e2e_test.go#TestIdentityCLI_ListShow"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-01 command tree: identity noun group, flat aliases built from a shared spec (never RunE delegation), reserved ssh/git/health/fix noun groups"
    requirement: "SHELL-03"
    verification:
      - kind: unit
        ref: "cmd/gitid/identity_test.go#TestVerbSpecsProduceIdenticalNounAndFlatCommands"
        status: pass
      - kind: unit
        ref: "cmd/gitid/identity_test.go#TestVerbNounAndFlatCommandsDoNotDelegateToEachOther"
        status: pass
      - kind: unit
        ref: "cmd/gitid/identity_test.go#TestReservedNounGroupsReturnPhaseNamedErrors"
        status: pass
    human_judgment: false

duration: ~100min
completed: 2026-08-25
status: complete
---

# Phase 5 Plan 01: Tracer — Git-Only Delete + D-03 Read Surface + D-01 Command Tree Summary

**One lifecycle verb (Git-only identity delete) wired end-to-end through domain -> Backend async seam -> composition root -> Cobra CLI, closing RESEARCH Pitfall 1 (Persist's silent Reduce fallthrough) and proving D-02's one-chokepoint claim; plus the frozen `identity list/show --json` read surface and the D-01 noun-verb command tree with flat aliases.**

## Performance

- **Duration:** ~100 min
- **Completed:** 2026-08-25
- **Tasks:** 3 (Tracer delete; D-03 read surface; D-01 command tree — executed as one coherent, tightly-coupled change per CLAUDE.md's buildable-boundary commit rule, see Deviations)
- **Files modified:** 19 (7 new, 12 modified)

## Accomplishments

- **The tracer proves the seam works.** `internal/identity/delete.go`'s `Delete` now takes a typed `DeleteScope`. Under `DeleteScopeGitOnly` the SSH branch is *structurally* skipped — `deps.ReadSSH`/`deps.WriteSSH` are never called (proven by a fail-on-invoke test) — so an unchanged `~/.ssh/config` can never acquire a spurious timestamped backup (review R-13). `deps.RemoveAllowedSigners`/`deps.RemoveKeyFiles` are likewise never called, so the allowed_signers line and the key pair survive untouched (D-10). `DeleteScopeEverything` returns `identity.ErrScopeNotAvailable` before touching any dependency, so the CLI `--all` path and the TUI's everything option refuse *identically* (review R-16) — both surface the exact same error text because both call the exact same function.
- **Real writes, not Reduce illusions.** `tuikit.Backend.CommitDelete` + `DeleteCommitMsg` mirror the existing `CommitGit`/`GitCommitMsg` async pattern exactly. The delete ceremony is now `Async: true`, gated by a new `deleteCommitPending` model field; `DeleteIdentity` is dispatched from `handleMsg` only once `DeleteCommitMsg` arrives with an empty `Err` — never optimistically on `ceremonyFinished`. `realBackend.Persist`'s new `tuikit.DeleteIdentity` case re-reads disk via `InitialState()` instead of falling through to `tuikit.Reduce` — the exact countermeasure to RESEARCH.md Pitfall 1, proven by a PTY test that restarts the compiled binary after the delete and asserts the identity is *still* healed to incomplete (not back to complete).
- **One production writer.** `cmd/gitid/wiring.go`'s `runDelete` is the ONE delete writer both `realBackend.CommitDelete` (TUI) and `cmd/gitid/identity_delete.go`'s `runIdentityDelete` (CLI `--git-only`) call — named exactly `runDelete` per review R3-05 so plan 05-07 can extend its signature and body in place rather than risk a second, drifted writer. It wraps `identity.Delete` in a `mutationJournal`-backed transaction: files are watched (snapshotted) before the write, and a mid-transaction failure calls `journal.restore()` and reports the restored paths.
- **The D-03 read surface is frozen.** `cmd/gitid/identity_read.go`'s `identity list`/`show` render a TTY-aligned table (`text/tabwriter`), a piped tab-delimited stream (no header), or a single `--json` document. The JSON schema emits BOTH health axes (`identity_state`, `key_state`) plus `problems`, alongside the collapsed `state` — the richer superset D-03 requires so later needs are satisfied by adding fields, never renaming. `list --json` carries `identities` AND a separate `unused_keys` array (review R-06 — see the taxonomy resolution below). Records are built fresh from `identity.BuildInventory` + `b.accounts()` on every call — no sidecar cache (MGR-08).
- **The D-01 command tree is built from one spec, twice.** `cmd/gitid/identity.go`'s `identityVerb` + `newVerbCmd` is the shared spec-based constructor: each verb (`list`/`show`/`delete`) is specified ONCE and consumed TWICE — once as an `identity <verb>` child, once as a flat `gitid <verb>` root alias — as two genuinely distinct `*cobra.Command` objects, never one delegating to the other's `RunE` (review R-15, proven by a recording-closure test). The reserved `ssh`/`git`/`health`/`fix` noun groups claim their taxonomy slot now, each returning a phase-named "not yet implemented" error, so Phases 6-8 can never collide with a flat alias.

## MGR-02 Taxonomy Resolution (binding for plans 05-07 and 05-09)

Recorded verbatim per the plan's instruction, since later plans assert against this:

- The eight MGR-02 labels are a VOCABULARY spanning TWO orthogonal axes, not eight mutually exclusive row values. `IdentityState` ranges over `complete`, `incomplete`, `git-only`, `fragment-path-missing`; `KeyState` ranges over `key-missing`, `key-unused`, `key-used-ssh-only`, `key-used-both`. All eight labels are observable per identity through `IdentityHealth`, and `--json` emits both axes so every label is reachable in the read surface.
- `ClassifyState`'s single collapsed row word can produce only SEVEN of the eight: its second switch returns `StateKeyMissing`, `StateKeyUnused`, and `StateKeyUsedSSHOnly`, and lets `StateKeyUsedBoth` fall through to `StateComplete`. This collapse is deliberate and already documented in `state.go`. It was NOT changed in this phase, and no acceptance criterion anywhere claims eight distinct collapsed row words — proven directly by `cmd/gitid/identity_test.go#TestIdentityRecordEightLabelsAcrossAxes`, which asserts exactly 7 distinct collapsed values with `key-used-both` absent.
- `Inventory.UnusedKeys` is a THIRD, separate thing: private key files on disk that no Host block references, belonging to no identity. They cannot be produced by joining `Inventory.Identities` to `b.accounts()`. They surface as the `unused_keys` array in `list --json`; their UI treatment is a Phase 8 doctor concern, out of scope here.

## Task Commits

Executed and committed as ONE coherent commit rather than three per-task commits — see "Deviations from Plan" for why.

1. **All three tasks (tracer + read surface + command tree)** — `58127ca` (feat)

## Files Created/Modified

- `internal/identity/delete.go` — `DeleteScope`/`DeleteScopeGitOnly`/`DeleteScopeEverything`/`DeleteScopeFrom`/`ErrScopeNotAvailable`; `Delete`'s scope-aware structural SSH skip + equality-guarded writers; `DeleteResult.SSHUntouched`.
- `internal/identity/delete_test.go` — full rewrite for the scope-based API: structural-skip proof, idempotency (equality guard + full-listing diff at the wiring layer), everything-scope refusal, `DeleteScopeFrom` coverage.
- `internal/tuikit/backend.go` — `Backend.CommitDelete(name, scope string) tea.Cmd`, with the R2-10 ownership rule documented in the interface doc comment.
- `internal/tuikit/views.go` — `DeleteCommitMsg{Backups, Restored, Removed, Err}`.
- `internal/tuikit/identities.go` — `deleteCeremonyFor` now `Async: true`; new `deleteCommitPending` field; `handleDeleteKey` dispatches `CommitDelete` on confirm instead of reducing optimistically; `handleMsg` reduces `DeleteIdentity` only on a successful `DeleteCommitMsg`.
- `internal/tuikit/identities_test.go` — the two existing full-App delete tests updated to run the async `CommitDelete` command via a new `pressAndRun` helper.
- `internal/tuikit/backend_stub_test.go`, `internal/dummytui/fixturebackend.go` — `CommitDelete` implemented for both non-real Backends.
- `cmd/gitid/wiring.go` — `buildDeleteDeps`, `realBackend.CommitDelete`/`runDelete`/`findAccount`, `expandTildeForHome`, the `Persist` `DeleteIdentity` case, `injectDeleteFailures` fault-injection wrapper.
- `cmd/gitid/wiring_test.go` — `TestDeleteDepsEveryFieldIsWired` (L2 guard extended), git-only preserve/idempotent/rollback proofs, CLI-vs-TUI parity and byte-identical-gitconfig proofs.
- `cmd/gitid/identity.go` (new) — `identityVerb`, `newVerbCmd`, `identityVerbSpecs`, `newIdentityCmd`, `registerFlatAliases`, `newReservedNounCmd`.
- `cmd/gitid/identity_delete.go` (new) — `identity delete <name>` with `--git-only`/`--all`/`--yes`/`--dry-run`.
- `cmd/gitid/identity_read.go` (new) — `identityRecord`, `identityListDocument`, `list`/`show` verbs, `writeTable`/`writeTabDelimited`/`writeJSON`, `collapseState`.
- `cmd/gitid/identity_test.go` (new) — Task 2 + Task 3 unit coverage.
- `cmd/gitid/main.go` — registers the identity tree, flat aliases, and reserved noun groups.
- `cmd/gitid/main_test.go`, `cmd/gitid/completion_test.go` — updated for the new D-01 surface (see Deviations).
- `e2e/identity_manager_pty_e2e_test.go` (new) — raw-keystroke PTY tracer proof + restart proof + CLI-vs-TUI byte-identical gitconfig proof.
- `e2e/identity_cli_e2e_test.go` (new) — headless `identity list --json` proof.

## Decisions Made

See `key-decisions` in the frontmatter. The most consequential: `runDelete`'s name is now load-bearing for plan 05-07 (it must extend this function, not add a second one), and the MGR-02 taxonomy resolution above is binding for plans 05-07/05-09.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed a literal-tilde path bug in the real delete writer, found via manual smoke test**
- **Found during:** Task 1, after the domain/backend/CLI layers all compiled and unit tests passed
- **Issue:** `Account.FragmentPath`/`KeyPath`/`PubPath` are stored VERBATIM by `identity.Reconstruct` — often a literal `"~/.gitconfig.d/<name>"` string, since git itself expands `~` at includeIf-resolution time but this process never does (documented on `loader.go`'s Reconstruct). `runDelete` passed these verbatim into `mutationJournal.watchFile` and `identity.Delete`'s deps, which called `containedRegularPath` — that function resolves `filepath.Abs` on the literal tilde string (never expanding it), producing a path outside the managed home and failing with `"gitid: refusing path outside managed home: ~/.gitconfig.d/work"`. No unit test caught this because every unit-level fixture used absolute paths directly; only a real end-to-end smoke test against a config file written the way a real user's config actually looks (`IdentityFile ~/.ssh/id_ed25519_work`) reproduced it.
- **Fix:** Added `expandTildeForHome(path, home string) string` in `cmd/gitid/wiring.go` — deliberately NOT `internal/identity`'s own unexported `expandTilde` (which resolves against `os.UserHomeDir()`/`$HOME`): `runDelete` already owns an explicit, possibly-sandboxed `b.home` and must never silently re-derive it from the process environment (the same WR-35 hermeticity lesson this codebase already learned once, applied here to path expansion instead of config reads). `runDelete` now normalizes `acct.FragmentPath`/`KeyPath`/`PubPath` against `b.home` before they become real filesystem paths anywhere.
- **Files modified:** `cmd/gitid/wiring.go`
- **Verification:** Manual smoke test (a real built binary against a hand-written config with a literal `~/.gitconfig.d/work` includeIf target) now completes the delete; `cmd/gitid/wiring_test.go`'s `seedDeleteFixture` reproduces this exact shape and is exercised by 6 new tests, all passing.
- **Committed in:** `58127ca` (part of the combined commit)

**2. [Rule 2 - Missing critical] `identityVerb`/`newVerbCmd` framework required for Task 1's own CLI delete verb**
- **Found during:** Task 1, writing `cmd/gitid/identity_delete.go`
- **Issue:** The plan's Task 1 action item 7 says `identity delete` is a real Cobra command, but Task 3's shared-spec constructor (`identityVerb`/`newVerbCmd`, review R-15) is the ONLY sanctioned way to add a verb without risking noun/flat-alias drift. Building Task 1's delete verb as a bespoke, unshared command and only later retrofitting it into Task 3's framework would have meant either two divergent code paths mid-plan or a rewrite.
- **Fix:** Built `identityVerb`/`newVerbCmd` (Task 3's core contribution) as part of Task 1's own delete-verb work, then reused it for Task 2's list/show verbs. This is why all three tasks landed in one commit — see below.
- **Files modified:** `cmd/gitid/identity.go`, `cmd/gitid/identity_delete.go`, `cmd/gitid/identity_read.go`
- **Verification:** `TestVerbSpecsProduceIdenticalNounAndFlatCommands`, `TestVerbNounAndFlatCommandsDoNotDelegateToEachOther`
- **Committed in:** `58127ca`

### CLAUDE.md-driven commit-granularity adjustment (not a deviation rule, but worth recording)

The GSD workflow's default is one commit per task. CLAUDE.md's commit rule ("let the buildable boundary, not file count, set the commit granularity") takes precedence here: Task 1's CLI delete verb, Task 2's list/show verbs, and Task 3's `identityVerb` framework are mutually load-bearing within `cmd/gitid` (Task 3's shared constructor is consumed by BOTH Task 1's and Task 2's verbs; `main_test.go`'s full-surface assertion only holds once all three are registered). A clean per-task commit boundary that still compiles and passes its own tests does not exist between them, so the whole plan landed as one commit (`58127ca`).

### Empirical finding — `gitid completion bash|zsh|fish` does not statically name `identity`

The plan's must_haves truth "`gitid completion bash|zsh|fish` emits a script naming the identity subcommands" does not hold LITERALLY for Cobra V2's generated scripts: verified by running `go run ./cmd/gitid completion bash` and grepping its 400+ line output for `"identity"` — zero matches, for ANY subcommand, not just this one. Cobra V2's bash/zsh/fish completion scripts are DYNAMIC: at Tab-press time they shell back out to the compiled binary's `__complete` machinery, which is where the actual subcommand list lives (confirmed with `go run ./cmd/gitid __complete ""`, which DOES list `identity`). `cmd/gitid/completion_test.go` documents this finding in a file-level comment, keeps the "script is non-empty and names gitid" assertion (true), and adds `TestCompletionDynamicListIncludesIdentity` driving the real `__complete` mechanism directly — the functionally-equivalent, empirically-true proof that Tab-completing `gitid <Tab>` really does surface `identity`.

---

**Total deviations:** 2 auto-fixed (1 bug, 1 missing-critical) + 1 CLAUDE.md-driven commit-granularity note + 1 empirical finding documented in-test.
**Impact on plan:** The tilde-expansion fix was necessary for correctness — without it, every real-world config (which legitimately uses literal `~/...` paths) would fail to delete. The `identityVerb` framework landing inside Task 1 is a sequencing necessity, not scope creep — Task 3's own acceptance criteria are unaffected. No scope creep beyond what the plan itself specifies.

## Issues Encountered

None beyond the deviations above. `go build`, `go vet`, `make lint` (golangci-lint 2.12.2, 0 issues), the full unit suite (`go test -race ./...`, 1090 passed across 19 packages), and the full e2e suite (`go test -tags e2e -race ./e2e/...`, 45 passed) are all green.

## Known Stubs

- `tuikit.DeleteCommitMsg.Removed` and `realBackend.CommitDelete`'s corresponding field are left unpopulated (always empty) in this plan — no acceptance criterion consumes it yet. `runDelete`'s own return signature is exactly `(backups, restored []string, err error)` per the plan's action item 5, so `Removed` has no natural source inside `runDelete` today. Candidate owner: plan 05-07's lifecycle extension, which is the same place the plan says `runDelete`'s signature grows anyway.
- `cmd/gitid/identity_delete.go`'s interactive confirmation prompt (`confirmDelete`, reached only when `--yes` is absent on a TTY) is minimal — a single `bufio.Reader.ReadString('\n')` compared against the literal string `"yes"`. It is functional and untested by any automated test in this plan (every acceptance criterion drives `--yes`). Candidate owner: whichever later plan adds interactive-confirmation PTY coverage for the CLI surface, if ever needed — the TUI already provides the confirmed-write experience for interactive users.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The tracer's chokepoint (`runDelete`) is ready for plan 05-07 to extend in place for the full delete lifecycle (everything scope, key removal, allowed_signers removal) — its name and current signature are the load-bearing contract 05-07 must preserve/extend, not replace.
- The D-03 read surface's JSON schema is frozen and ready for plan 05-09 (and any other consumer) to build against; the MGR-02 taxonomy resolution above is binding.
- The D-01 command tree's reserved noun groups (`ssh`/`git`/`health`/`fix`) are claimed and ready for Phases 6-8 to implement in place.
- `identity.DeleteScopeEverything` still returns `ErrScopeNotAvailable` — plan 05-04 is the one authorized to replace that branch with the real implementation (per this plan's own scope).
- No blockers identified for the remaining Phase 5 plans.

---
*Phase: 05-identity-manager*
*Completed: 2026-08-25*

## Self-Check: PASSED

All 10 created/modified files listed above confirmed present on disk; commit `58127ca` confirmed present in `git log`.
