# Phase 5: Identity Manager - Research

**Researched:** 2026-08-25
**Domain:** Go TUI/CLI backend wiring — identity lifecycle mutations (clone/rotate/new-key/delete), a shared Bubble Tea v2 render stack, and a Cobra CLI tree, all layered on an already-built config-parsing/write substrate.
**Confidence:** HIGH (substrate) / MEDIUM (new lifecycle logic design) / LOW (exact CLI flag/JSON shape — explicitly Claude's Discretion)

## Summary

Phase 5 is almost entirely **backend wiring on top of existing packages** — no
new external dependency is needed (`go.mod` already carries Cobra v1.10.2,
Bubble Tea v2, `golang.org/x/crypto/ssh`, `kevinburke/ssh_config`). The work
is: (1) implement four new identity lifecycle operations correctly against
the recipes' canonical shape (rotate-with-archive, new-key-repair, clone,
delete-with-choice), (2) wire them into the REAL `cmd/gitid` Backend so they
stop silently falling through to the in-memory demo reducer, and (3) build a
Cobra noun-verb CLI tree that is currently a nearly blank slate.

The single most important finding for planning: **`cmd/gitid/wiring.go`'s
`realBackend.Persist` currently only special-cases `AddIdentity` and
`Reset`** [VERIFIED: cmd/gitid/wiring.go:352-361] — every other Action
(`CloneIdentity`, `DeleteIdentity`, `NewKey`, `EditSSH`, …) falls through to
`tuikit.Reduce`, the **pure in-memory reducer shared with the dummy
fixture backend** [VERIFIED: internal/tuikit/store.go:228-396]. This means
MGR-04/05/06 today produce **zero real writes** in the live binary — they
only look like they work because the reducer updates the in-memory
`DemoState` the same way the dummy does. Phase 5's core job is closing this
gap, following the async `CommitCreate`/`CommitGit` → `WizardCommitMsg`/
`GitCommitMsg` pattern Phases 3-4 already established
[VERIFIED: internal/tuikit/backend.go:148-158, cmd/gitid/wiring.go:816,1808].

A second load-bearing finding: the approved Phase-2 design
(`identity-manager/FIELDS.md`) has **exactly one** key-generation menu row —
`action_new_key` ("Generate new key") — and explicitly notes MGR-05 is
"referenced, not a separate named state in §4(3)"
[VERIFIED: .planning/design/identity-manager/FIELDS.md:71]. CONTEXT.md's D-05
(rotate vs new-key as two distinct ceremonies) is therefore a **backend
routing decision behind one UI entry point**, not a new screen — the menu
item must dispatch to Rotate (healthy identity) or the repair path
(key-missing / shared-key state) based on the identity's current MGR-02
state, using `identity.ClassifyState`'s existing precedence
[VERIFIED: internal/identity/state.go:209-236].

A third finding narrows scope: `internal/identity/modes.go` **already has a
`Rotate` function** [VERIFIED: internal/identity/modes.go:181-190], but it
writes `allowed_signers` via `deps.WriteAllowedSigners` →
`keygen.WriteAllowedSigners` → `filewriter.ReplaceBlock`, which **replaces**
the identity's whole managed block with one line
[VERIFIED: internal/keygen/signers.go:51-64]. This **directly conflicts**
with CONTEXT D-07 ("rotate APPENDS, old line KEPT, tolerate two lines per
email"). The existing `Rotate` also has no archive-before-overwrite step
(D-06) and no grace-window hint (D-08). Existing `Rotate` is a correct
*skeleton* (staging, gate, four-writer pipeline) but its allowed_signers and
key-persistence steps must be extended, not reused verbatim.

**Primary recommendation:** Build the four lifecycle flows as new orchestration
functions in `internal/identity` (mirroring `Delete`/`Update`/`Rotate`'s
existing Deps-injection shape), reusing `internal/filewriter`'s
`mutationJournal`-style all-or-nothing rollback pattern already proven in
`cmd/gitid/wiring.go`'s `commitGitTransaction`
[VERIFIED: cmd/gitid/wiring.go:863,1041], then wire each through a new async
`Commit<Verb>` Backend method + typed `<Verb>CommitMsg`, and build the Cobra
tree from scratch inside the already-empty `newRootCmd()`
[VERIFIED: cmd/gitid/main.go:74-87].

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Identity state classification (MGR-02) | Domain (`internal/identity`) | — | Pure function, already built, no I/O (`Classify`/`ClassifyState`) |
| Rotate/new-key/clone/delete orchestration | Domain (`internal/identity`) | — | Effects injected via Deps structs (DLV-07 UI-free core); mirrors `Create`/`Update`/`Delete` |
| Managed-block read/write (SSH, gitconfig, allowed_signers) | Domain (`internal/sshconfig`, `internal/gitconfig`, `internal/keygen`, `internal/filewriter`) | — | Sentinel-block idempotent writers; no new library, extend existing functions |
| Real Backend composition + DTO conversion | CLI/TUI composition root (`cmd/gitid/wiring.go`) | — | The ONLY place `internal/tuikit` DTOs meet backend types (documented file-header contract) |
| TUI rendering (identity-manager screens) | TUI (`internal/tuikit/identities.go`) | — | Backend-free; clone/delete panes already exist as UI scaffolding, awaiting real wiring |
| Cobra CLI tree (SHELL-03) | CLI (`cmd/gitid/*.go`, new files) | Domain (`internal/identity`) | Command handlers gather flags/prompt, call the SAME domain functions the TUI calls (D-04 outcome parity — no ceremony-step commands) |
| JSON/table output for `list`/`show` | CLI | — | New view-serialization layer over the existing `identity.Account`/`IdentityHealth` types |

## User Constraints (from CONTEXT.md)

<user_constraints>

### Locked Decisions

**CLI parity surface (SHELL-03)**
- D-01 — Noun-verb taxonomy + flat aliases: `gitid identity create|list|show|clone|new-key|rotate|delete`; noun groups `gitid ssh`/`gitid git`/`gitid health`/`gitid fix` reserved for Phases 6-8; Cobra aliases keep `gitid create` etc. working at top level.
- D-02 — Adaptive gh-style non-interactive depth: complete flags → headless; incomplete flags + TTY → pre-filled wizard; incomplete flags + non-TTY → error listing missing flags. Reads (`list`/`show`/`health`) always headless. `--yes` replaces ONLY the confirmation prompt; `--dry-run` runs test+preview and stops; timestamped backups are UNCONDITIONAL (no flag skips them); post-write re-test drives exit code. CLI and TUI MUST call the same test → backup → write → re-test chokepoint.
- D-03 — `--json` + TTY-aware plain output: aligned table on TTY, tab-delimited when piped, `--json` marshals the existing 8-state identity model; JSON output is also the PTY-free e2e assertion surface.
- D-04 — SHELL-03 = outcome parity, verified by a parity matrix: every product OUTCOME (create, clone, new-key, rotate, delete `--git-only`/`--all`, list, show, health, fix) has a CLI command; ceremony steps are internal invariants, never separate commands. A requirement-keyed parity matrix (MGR-01..08/KEY-05/KEY-07 → command) makes ROADMAP success criterion 4 checkable.

**Key lifecycle: rotate vs new-key (KEY-05, KEY-07, MGR-05)**
- D-05 — Rotate = retirement ceremony; new-key = repair action. Rotate (healthy identities): archive old key, provider-cleanup guidance, re-test. New-key (repair for key-missing/shared-key states): generate a fresh key and re-point, NEVER touching pre-existing key material. Two ceremonies sharing the keygen + re-point core.
- D-06 — Old key archives to `~/.ssh/gitid-archive/<file>.<timestamp>` (dir 700, keys 600) inside the all-or-nothing transaction, vacating the canonical `id_ed25519_<name>` path. MANDATORY: register the archive directory as doctor-reserved.
- D-07 — `allowed_signers` APPENDS on rotate. New pubkey line added, old line KEPT (same email, old blob) so `git log --show-signature` keeps verifying pre-rotation commits. Managed-line logic must tolerate two lines per email. Pruning is a later fixer concern (Phase 8), not Phase 5.
- D-08 — Post-rotate test gate reuses Phase 3 D-01/D-02/D-03 verbatim. Expected landing state is ReachableNotUploaded (yellow "!" "Reachable — key not uploaded yet") + copy-.pub + provider hint. One added grace-window hint line: old key remains valid at <provider>; upload the new key, verify, then remove the old one. Copy extends (never alters) the frozen D-02/D-03 strings; new strings join the §6 copy-freeze grep. IdentityFile and user.signingkey stay textually unchanged; only allowed_signers block + key files change.

**Delete semantics (MGR-06)**
- D-09 — Per-provider insteadOf block is reference-counted. Deleting the LAST identity for a provider removes the block, named explicitly in the confirm screen's file list. Ref-count must account for hand-written aliases targeting the same provider.
- D-10 — `allowed_signers` line is KEPT in "Git identity only" mode, removed only in "delete everything". The doctor/health taxonomy must tolerate a fragment-less principal without flagging it as drift.
- D-11 — "Delete everything" backup-copies the key pair into the timestamped backup dir before deletion (backup dir 700, key copies 600). Confirm copy: "removed from active use; a copy exists at <backup path>" — never overclaim irreversibility. Fragment + includeIf are both removed in git-only mode too, with the fragment backed up before unlink.
- D-12 — Shared key auto-downgrades "delete everything". When the identity's key is referenced by another identity, SSH + Git artifacts are deleted but the key is kept, with an explicit note naming the sibling identity.
- D-13 — Unmanaged-reference scan, warn-never-block. Before delete, the alias is scanned across unmanaged regions of files gitid already parses; hits are named in the confirm screen, plus a fixed honest warning that repo remotes using `git@<alias>:` cannot be scanned.

**Clone semantics (MGR-04)**
- D-14 — Copy + re-derive pre-fill. SSH fields copied with the new alias substituted; `user.name`/`user.email` pre-filled but flagged "copied from <source> — review"; gitdir, `hasconfig:` pattern, signingkey, and allowed_signers line are ALL re-derived from the NEW name and key choice — never copied verbatim.
- D-15 — Clone customization runs inside the create wizard, entered pre-filled. clone-name-prompt answers name + key choice, then the full Phase 3/4 wizard opens with initial model state injected. ONE write path, no standalone clone pipeline.
- D-16 — Same-key clones re-run the FULL two-stage test gate. The new Host block's `ssh -G` resolution is the genuinely unproven artifact.
- D-17 — Suggested name `<source>-clone`, auto-bumped to `<source>-clone-2` when the suggestion itself collides, with live Phase-3 D-09 validation against ALL parsed Host patterns.

### Claude's Discretion
- Exact Cobra flag names/shorthands and the parity-matrix document location/format.
- JSON schema shape for `list`/`show` (marshal the existing identity model; keep stable once shipped).
- gitid-archive filename convention details and the reserved-registration mechanics (mirror Phase 4 D-04's approach).
- Exact copy for: the grace-window hint, shared-key downgrade note, unmanaged-reference warning, "copied from <source> — review" flag, and the precise delete-everything confirm phrasing — draft in the UI wave, freeze via 02-STYLE-SPEC §6.
- Scan implementation for D-13 (substring false-positive guard — the superstring-principal pattern in `reader_test.go` is the analog).

### Deferred Ideas (OUT OF SCOPE)
- Pruning archived keys / stale allowed_signers lines (`gitid key purge` or a fixer action) — Phase 8; D-07/D-06 deliberately accumulate.
- Compromised-key path (immediate-invalidation guidance + true no-copy delete variant) — revisit post-v1.0 or Phase 8.
- gh-style `--json fields` + `--jq` field selection — only if an automation ecosystem emerges.
- Upload automation on rotate (auto-upload the new .pub) — Phase 9 (UP-01..03).
- Real Global SSH / Global Git / Health / Fixer views — Phases 6-8; the manager only routes to their D-16 demo screens.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MGR-01 | Completeness in list (complete vs incomplete flag per row) | Already fed by `identity.BuildInventory`/`Reconstruct`/`ClassifyState` [VERIFIED: internal/identity/state.go, inventory.go]; `toDemoIdentity` in wiring.go already converts `Account`→`DemoIdentity` for `InitialState` — MGR-01 is largely READ-side, already wired [VERIFIED: cmd/gitid/wiring.go:1409,323-335]. |
| MGR-03 | SSH-first detail, no fabricated Git fields for SSH-only identity | `identitiesModel` already has a detail render surface (Phase 2/3 dummy scaffold); `Account.Incomplete` already flags missing pieces [VERIFIED: internal/identity/identity.go:56-60]. Wiring work is ensuring the REAL DemoIdentity conversion leaves Git fields empty (not zero-valued placeholders) when GitConfigured is false. |
| MGR-04 | Clone into new name, same key or new key, customize before write | `identity.AddAccount` (same-key path) exists [VERIFIED: internal/identity/modes.go:129-168]; a new-key clone path needs `Rotate`-style key generation composed with `AddAccount`-style artifact re-derivation. `CloneIdentity` Action + `paneClone` UI scaffold already exist [VERIFIED: internal/tuikit/store.go:123-126, identities.go:1912-1921]. D-15 requires routing clone into the EXISTING create-wizard pre-filled, not a standalone flow. |
| MGR-05 | New key for existing identity (repair, distinct from rotate) | No repair-specific domain function exists yet; closest existing action is `tuikit.NewKey` (Action only, reduced in-memory) [VERIFIED: internal/tuikit/store.go:136-140,307-323]. Needs a new `identity` function that regenerates + re-points WITHOUT archiving old material (per D-05). |
| MGR-06 | Delete choice: everything vs Git-only | `identity.Delete` exists with a `keepKey bool` parameter that already implements the git-only/everything split at the KEY level [VERIFIED: internal/identity/delete.go:56-114], but needs: (a) D-09 per-provider insteadOf ref-count + removal (`gitconfig.RemoveProviderRewrite` does not yet exist — only `WriteProviderRewrite`/`HasProviderRewrite` do [VERIFIED: internal/gitconfig/renderer.go:159-223]), (b) D-11 key backup-BEFORE-delete (current `RemoveKeyFiles` dep is a plain backup-then-remove, needs archive-dir routing to match D-06's convention), (c) D-12 shared-key detection (`keyOwners()` in wiring.go already computes this map [VERIFIED: cmd/gitid/wiring.go:1594-1607]), (d) D-13 unmanaged-reference scan (net new). |
| MGR-07 | Per-identity health | `identity.Classify` already returns per-identity `IdentityHealth` with `Problems` [VERIFIED: internal/identity/state.go:100-120,161-207] — this is largely a rendering/wiring task, not new domain logic. |
| MGR-08 | No sidecar DB | Already the architecture: `Reconstruct` parses managed blocks fresh every time [VERIFIED: internal/identity/loader.go:28]. Preserve this — no new persistent state for Phase 5. |
| KEY-05 | Rotate: re-point artifacts, re-run test | `identity.Rotate` exists as a skeleton [VERIFIED: internal/identity/modes.go:181-212] but needs D-06 (archive) + D-07 (append allowed_signers) extension — see Common Pitfalls. |
| KEY-07 | New key for existing identity (repair) | Net new domain function — see MGR-05. |
| SHELL-01 | Integrated Bubble Tea v2 app (built, Phase 3) | Already built [VERIFIED: internal/tuikit package structure, cmd/gitid/main.go:44]; no new work beyond wiring the new panes' Backend calls. |
| SHELL-02 | 5 primary views reachable via palette + number keys | `TabID`/number-key nav exists per Phase 2 design (`identity-manager` is nav root on key `1` [VERIFIED: .planning/design/identity-manager/FIELDS.md:16-18]); confirm the other 4 tabs (Global SSH/Global Git/Health/Fixer) are reachable and keep their D-16 demo banners (`DemoBanner` already returns true for every non-Identities tab [VERIFIED: cmd/gitid/wiring.go:340-342]). |
| SHELL-03 | CLI parity, shell completion | `newRootCmd()` is currently a near-blank slate — only `debug` + Cobra's auto `completion` exist [VERIFIED: cmd/gitid/main.go:63-87]. This is greenfield work within an already-scaffolded Cobra root. |

</phase_requirements>

## Standard Stack

### Core (already in go.mod — no new install)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.10.2 [VERIFIED: go.mod:16] | CLI noun-verb tree, shell completion | Already the project's chosen CLI framework (CLAUDE.md); `newRootCmd()` is ready for `AddCommand` calls. |
| `charm.land/bubbletea/v2` | (pinned in go.mod) | TUI event loop | Already wired via `tuikit.NewApp` [VERIFIED: cmd/gitid/main.go:44]; no version change needed for Phase 5. |
| `golang.org/x/crypto/ssh` | (pinned in go.mod) | Ed25519 keygen, `ssh.MarshalAuthorizedKey`/`ParsePrivateKey` | Already used throughout `internal/keygen`; rotate/new-key reuse `keygen.GenerateMaterial`/`Registry` unchanged. |
| `github.com/kevinburke/ssh_config` | (pinned in go.mod) | `~/.ssh/config` round-trip parse | Already used by `internal/sshconfig`; no new parsing logic needed for delete/clone/rotate (all go through the existing `filewriter.ReplaceBlock`/`RemoveBlock` sentinel-block layer, not the ssh_config AST). |

**Version verification:** `go.mod` was read directly this session; no `npm view`/`pip index` equivalent applies since this phase adds zero new dependencies. `go list -m all` was not re-run because no import changes are anticipated — flag this to the planner: if a task discovers a genuine new dependency need (unlikely), verify it with `go list -m -versions <module>` before adding.

### Supporting (existing internal packages Phase 5 extends, not replaces)

| Package | Purpose | Extension needed |
|---------|---------|-------------------|
| `internal/identity` | Domain orchestration (Create/Update/Delete/Reuse/AddAccount/Rotate) | Add: archive-aware Rotate extension, new repair (new-key) function, clone-with-new-key composition, D-09/D-12/D-13 delete extensions. |
| `internal/keygen` | Key generation, `AllowedSignersLine`, `WriteAllowedSigners`, `KeyPaths` | Add: an append-not-replace allowed_signers writer for D-07 (see Common Pitfalls #2); an archive-to-`gitid-archive/` helper distinct from `filewriter.BackupAndRemove`'s sibling-`.bak` convention. |
| `internal/gitconfig` | `WriteProviderRewrite`/`HasProviderRewrite`/`RemoveURLRewritesBlock` | Add: `RemoveProviderRewrite(gitconfigPath, provider)` mirroring `WriteProviderRewrite` but calling `filewriter.RemoveBlock` — does NOT exist yet [VERIFIED: internal/gitconfig/renderer.go:159-223 has Write/Has but no Remove]. |
| `internal/sshconfig` | `ReservedPaths`/`IsReservedPath`, `IsReservedBlockName` | Add a `gitid-archive` entry to the reserved-path registry (D-06 MANDATORY requirement), mirroring the existing `config.d` registration [VERIFIED: internal/sshconfig/include.go:71-100]. |
| `internal/filewriter` | `ReplaceBlock`/`RemoveBlock`/`BackupAndRemove`/`Write` | No new primitives required — the archive-dir copy can be built from `filewriter.CopyFile`/`Write` composed with a caller-chosen destination directory, distinct from `BackupAndRemove`'s sibling-path convention [VERIFIED: internal/filewriter/filewriter.go:130-175,198-213]. |
| `internal/tuikit` | `Backend` interface, `Action`/`DemoState`/`Reduce`, `identities.go` panes | Add: `RotateIdentity`/repair Action type(s) distinct from the current simplistic `NewKey` [VERIFIED: internal/tuikit/store.go:136-140] — see Common Pitfalls #3; wire real `Commit<Verb>` Backend methods. |
| `cmd/gitid/wiring.go` | The one composition root (`realBackend`) | Add real `Persist`/`Commit*` cases for Clone/Delete/NewKey/Rotate — currently absent [VERIFIED: cmd/gitid/wiring.go:352-361]. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Extending `filewriter.ReplaceBlock`-based allowed_signers writer to support append | A brand-new multi-line-aware writer format | Rejected in favor of reading the existing block body via `filewriter.ListBlocks`, appending the new line client-side, and re-writing via `ReplaceBlock` with the combined body — `ReplaceBlock`'s body is opaque text, so no format change is needed, only a caller-side compose step. |
| A new `gitid-archive` file-copy helper | Reusing `filewriter.BackupAndRemove` directly | Rejected: `BackupAndRemove` backs up to a SIBLING path (`<path>.bak.<nano>`) in the SAME directory [VERIFIED: internal/filewriter/filewriter.go:130-149], but D-06 requires a DEDICATED `~/.ssh/gitid-archive/` directory that must be doctor-reserved — a distinct destination directory needs a small new helper, though it can reuse `filewriter.CopyFile`'s exclusive-create + explicit-mode pattern. |
| A separate `RotateIdentity` Cobra/TUI action type | Overloading the existing `tuikit.NewKey` Action with a `bool` "isRotate" field | Recommend a distinct Action type (`RotateIdentity`) rather than overloading `NewKey`: CONTEXT D-05 treats them as two ceremonies with different backup/append semantics, and the dummy's `Reduce` function's `NewKey` case already has a specific, narrow, frozen-shape effect [VERIFIED: internal/tuikit/store.go:307-323] that must not silently branch on a boolean flag readers won't notice. |

**Installation:** none — no `go get` needed for this phase's anticipated scope.

## Package Legitimacy Audit

Not applicable — Phase 5 adds zero new external packages. All work extends
already-vetted, already-imported modules (`go.mod`, read this session). If a
plan task later discovers a genuine new-package need, the planner must insert
a `checkpoint:human-verify` gate and run the standard `npm view`/`go list -m
-versions` + `gsd-tools query package-legitimacy check` protocol before
adding it — none is anticipated from this research.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────┐
                    │   cmd/gitid (Cobra CLI, Phase 5 new)     │
                    │   identity create|list|show|clone|       │
                    │   new-key|rotate|delete   (+ aliases)    │
                    └───────────────┬───────────────────────────┘
                                    │  (D-02: complete flags → headless;
                                    │   incomplete + TTY → pre-filled wizard)
                                    ▼
   ┌───────────────────────────────────────────────────────────────┐
   │       cmd/gitid/wiring.go — realBackend (composition root)     │
   │  InitialState() ─┐                                             │
   │  Persist(state,Action) ── Reset/AddIdentity (existing)          │
   │                  │        Clone/Delete/NewKey/Rotate (NEW)      │
   │  Commit<Verb>(...) tea.Cmd → async, delivers <Verb>CommitMsg   │
   │        (mirrors existing CommitCreate/CommitGit pattern)        │
   └───────────────┬───────────────────────────────┬─────────────────┘
                    │ (DTO conversion, the ONLY      │
                    │  backend<->tuikit boundary)    │
                    ▼                                ▼
   ┌───────────────────────────┐    ┌──────────────────────────────────┐
   │ internal/tuikit            │    │ internal/identity (domain,        │
   │ identities.go panes:        │    │  UI-free, DLV-07)                 │
   │  list / detail / action-    │    │  Reconstruct → Classify/          │
   │  menu / clone-name-prompt / │    │  ClassifyState (MGR-02)           │
   │  delete-choice /            │    │  Create/Update/Delete/Reuse/      │
   │  confirm-destructive /      │    │  AddAccount/Rotate (existing)     │
   │  backup-notice              │    │  + NEW: repair(new-key),          │
   │  (already scaffolded)       │    │    rotate-with-archive,           │
   └───────────────────────────┘    │    clone-with-new-key,             │
                                     │    delete-with-refcount/scan       │
                                     └──────────────┬─────────────────────┘
                                                     │ injected Deps (fakeable)
                    ┌────────────────────────────────┼─────────────────────┐
                    ▼                                ▼                     ▼
     internal/sshconfig            internal/gitconfig            internal/keygen
     (Host block writer,           (includeIf/fragment/          (Generate, Archive[NEW],
      ReservedPaths+archive[NEW])   provider-rewrite ref-count[NEW]) AllowedSignersLine,
                    │                        │                       AppendAllowedSigners[NEW])
                    └────────────┬───────────┴──────────┬────────────┘
                                 ▼                       ▼
                         internal/filewriter (sentinel-block ReplaceBlock/
                          RemoveBlock, atomic Write, BackupAndRemove)
                                 │
                                 ▼
                     ~/.ssh/config, ~/.gitconfig, ~/.gitconfig.d/<id>,
                     ~/.ssh/allowed_signers, ~/.ssh/id_ed25519_<id>[.pub],
                     ~/.ssh/gitid-archive/<file>.<timestamp>  [NEW, D-06]
```

### Recommended Project Structure

No new top-level packages — extend existing files/packages:

```
internal/identity/
├── modes.go        # extend Rotate (D-06/D-07/D-08); add repair (KEY-07/MGR-05)
├── delete.go        # extend Delete for D-09 refcount, D-11 key-archive-before-delete,
│                     #   D-12 shared-key downgrade, D-13 unmanaged scan
├── clone.go         # NEW: compose AddAccount-style re-derivation + optional keygen
internal/keygen/
├── signers.go        # add an append-aware allowed_signers writer (D-07)
├── archive.go         # NEW: archive-to-gitid-archive/ helper (D-06)
internal/gitconfig/
├── renderer.go        # add RemoveProviderRewrite (D-09)
internal/sshconfig/
├── include.go          # extend ReservedPaths/IsReservedPath with gitid-archive (D-06)
internal/tuikit/
├── store.go             # add RotateIdentity Action (distinct from NewKey), extend
│                        #   DeleteIdentity/CloneIdentity handling if fields are missing
├── identities.go         # wire real Backend calls behind existing panes (paneClone,
│                         #   paneDeleteScope, paneDelete, action-menu)
cmd/gitid/
├── wiring.go               # add Persist cases + Commit<Verb> async methods
├── identity_create.go       # NEW Cobra subcommands (one file per verb or grouped)
├── identity_list.go
├── identity_show.go
├── identity_clone.go
├── identity_newkey.go
├── identity_rotate.go
├── identity_delete.go
```

### Pattern 1: Async Commit with typed result Msg (established, reuse verbatim)

**What:** A `Backend.Commit<Verb>` method returns a `tea.Cmd` that performs
the real, backed-up write off the Update loop and eventually delivers a
typed `<Verb>CommitMsg{Backups []string, Err string}` (or richer, e.g.
`GitCommitMsg` also carries `Restored []string`). The UI's `handleMsg`
switches on the concrete Msg type and only then dispatches the display
Action.
**When to use:** Every Phase 5 mutation (clone/rotate/new-key/delete) — this
is the established pattern, not a new one to invent.
**Example (existing, to mirror):**
```go
// Source: cmd/gitid/wiring.go:816 (verified, read this session)
func (b *realBackend) CommitGit(spec tuikit.GitSpec) tea.Cmd {
    return func() tea.Msg {
        if b.initErr != nil {
            return tuikit.GitCommitMsg{Err: b.displayMessage(b.initErr.Error())}
        }
        // ... commitGitTransaction via mutationJournal, then:
        return tuikit.GitCommitMsg{Backups: displayBackups}
    }
}
```

### Pattern 2: All-or-nothing rollback via `mutationJournal` (established, reuse)

**What:** `wiring.go` already has a `mutationJournal` type (`watchFile`,
`watchDir`, `addBackup`, `restore`) used by `commitGitTransaction` to snapshot
every file/dir the transaction is about to touch, so a mid-transaction
failure can roll every artifact back
[VERIFIED: cmd/gitid/wiring.go:863,892,896,923,1026,1032,1041].
**When to use:** Delete (D-11, multiple artifacts + key backup) and Rotate
(D-06/D-07, key archive + allowed_signers append + host block re-point) both
touch 3-5 files atomically — reuse this journal rather than hand-rolling
per-verb rollback.

### Pattern 3: Deps-injection domain functions (established, DLV-07)

**What:** Every `internal/identity` orchestration function (`Create`,
`Update`, `Delete`, `Rotate`, `AddAccount`) takes a `...Deps` struct of
function fields for every external effect, is unit-testable with fakes, and
performs zero I/O itself.
**When to use:** All new Phase 5 domain logic (repair/new-key, D-09/D-12/D-13
delete extensions, clone-with-new-key). Do not special-case Phase 5 logic
directly in `cmd/gitid/wiring.go` — wiring.go only builds the real Deps and
calls the domain function, per the file's own documented contract
[VERIFIED: cmd/gitid/wiring.go:1-33 file header].

### Anti-Patterns to Avoid

- **Reusing `identity.Rotate` unmodified for KEY-05:** it currently
  overwrites the allowed_signers block (no append) and never archives the
  old key — shipping it as-is violates D-06/D-07. Extend it explicitly.
- **Overloading `tuikit.NewKey` for both rotate and repair:** the two
  ceremonies have different backup/append/hint semantics (D-05); collapsing
  them into one Action with a boolean flag hides the distinction from every
  future reader of `store.go`'s `Reduce` switch.
- **Building a standalone clone write pipeline:** D-15 is explicit — clone
  must open the EXISTING create wizard pre-filled, not a second write path.
- **Hand-rolling a second reserved-path list for `gitid-archive`:** extend
  the existing `sshconfig.ReservedPaths`/`IsReservedPath` functions
  [VERIFIED: internal/sshconfig/include.go:71-100] — a second list is exactly
  the "Doctor reserved-block false-positive loop" bug class this project has
  hit before (see project memory).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Managed-block idempotent write | A custom line-splice/regex rewriter | `filewriter.ReplaceBlock`/`RemoveBlock`/`ListBlocks` | Already handles CRLF, foreign-content preservation, idempotency — proven by ~15 existing tests. |
| Principal-exact-match line removal | A new substring-based email matcher | The existing `RemoveAllowedSignersLine` exact-first-field + `namespaces="git"` pattern [VERIFIED: internal/gitconfig/reader.go:185-206] | Already closes the CR-01 superstring-principal vulnerability class (`alice@corp.com` vs `alice@corp.com.attacker.example`) — D-13's scan guard should reuse this exact pattern, not a fresh regex. |
| Timestamped backup naming | A custom timestamp format | `filewriter.backupExistingTarget`'s `<path>.bak.<UnixNano>` collision-proof convention [VERIFIED: internal/filewriter/filewriter.go:130-149] for in-place backups; for the D-06 `gitid-archive/` directory, compose the SAME collision-proof exclusive-create idiom with a caller-chosen destination dir | Consistency with the project's one backup-naming convention; avoids inventing a second timestamp format (note: `tuikit.NewBackupPath`'s ISO-8601-with-dashes format is a DISPLAY-side dummy-fixture convention, NOT what the real filewriter produces — do not copy it into real backend code). |
| Shared-key detection | A fresh key-usage scanner | `realBackend.keyOwners()` [VERIFIED: cmd/gitid/wiring.go:1594-1607] — already maps `KeyPath → "name (provider)"` across all accounts | Directly answers D-12's "name the sibling identity" requirement with zero new code. |
| Per-identity health | A new health computation | `identity.Classify` [VERIFIED: internal/identity/state.go:161-207] | Already returns the orthogonal IdentityState/KeyState axes + Problems list MGR-07 needs. |

**Key insight:** almost every "Don't Hand-Roll" item in this phase is "don't
hand-roll — the substrate to reuse already exists in this repo," not "use an
external library." The domain is narrow (parsing/writing OpenSSH and
gitconfig text) and the project has already built and tested the primitives;
Phase 5's risk is skipping past them and reinventing, not picking the wrong
external dependency.

## Common Pitfalls

### Pitfall 1: `Persist`'s silent fallthrough to the demo reducer
**What goes wrong:** A plan that "wires MGR-04/05/06" by adding UI-side
handling alone will appear to work in manual testing (the in-memory
`Reduce` updates the list correctly) while producing **zero real file
writes** — indistinguishable from success until the app restarts and
`InitialState()` re-reads disk.
**Why it happens:** `realBackend.Persist`'s `default:` case calls
`tuikit.Reduce(state, action)` [VERIFIED: cmd/gitid/wiring.go:352-361],
exactly like `dummytui`'s FixtureBackend does — the two backends are
deliberately symmetric for actions neither has wired yet.
**How to avoid:** Every plan task for Clone/Delete/NewKey/Rotate must include
an explicit acceptance criterion "the action performs a real write (verified
by re-reading the file after `InitialState()` reload), not just an in-memory
state update" — mirror the PTY e2e pattern already used for AddIdentity/
ConfigureGit (raw keystrokes against the real built binary, DLV-06).
**Warning signs:** A test that only asserts against `tuikit.DemoState` after
calling `Persist`/`Reduce` without ever touching the filesystem.

### Pitfall 2: `WriteAllowedSigners` replaces, D-07 requires append
**What goes wrong:** Calling the existing `Rotate`→`runPipeline`→
`WriteAllowedSigners` path as-is on rotate loses the OLD identity's signing
line, breaking `git log --show-signature` on pre-rotation commits — exactly
what D-07 exists to prevent.
**Why it happens:** `keygen.WriteAllowedSigners` composes the block via
`filewriter.ReplaceBlock(existing, identity, line)`
[VERIFIED: internal/keygen/signers.go:51-64], and `ReplaceBlock` REPLACES
the named block's body wholesale (it is not additive by design — that is
exactly what makes it correct for the normal create/update case, where one
identity should have exactly one signing line).
**How to avoid:** For the rotate path only, read the identity's CURRENT block
body first (`filewriter.ListBlocks` → find by name), append the new line to
the existing body (guard against duplicate lines on idempotent re-run), then
call `ReplaceBlock` with the COMBINED body — do not call the plain
single-line `WriteAllowedSigners` for rotate. Keep the existing single-line
behavior for create/update/new-key (D-05: new-key does NOT touch
pre-existing key material or old signing lines, but it is not archiving an
old key either — confirm with the user during planning whether new-key
should also append or should behave like a normal single-line write, since
D-07's append rule is scoped to "rotate" in CONTEXT.md and new-key's target
states — key-missing / shared-key — likely have no valid prior key to
preserve a signature for).
**Warning signs:** A rotate test that checks the NEW line is present but
never asserts the OLD line survives.

### Pitfall 3: No distinct Rotate Action type exists yet
**What goes wrong:** The `tuikit.Action` union has `NewKey` (heals
key-missing, simple re-point, no archive) but no `RotateIdentity`
[VERIFIED: internal/tuikit/store.go:99-203]. A plan that assumes "Rotate" is
already representable in the shared render-stack vocabulary will discover
mid-implementation that `store.go`, `Reduce`, and the dummy's own
FixtureBackend all need a new case added — this is `internal/tuikit`
package surface, shared by BOTH binaries, so the change is NOT confined to
`cmd/gitid`.
**Why it happens:** The approved Phase 2 design only names ONE menu item
("Generate new key") — Rotate-vs-repair is a Phase 5 backend decision layered
on top, and nobody has touched `store.go` since Phase 2.
**How to avoid:** Plan an explicit task to add `RotateIdentity` (or
equivalent) to `internal/tuikit/store.go`'s `Action` union + `Reduce` switch
+ the dummy's fixture reduction path, BEFORE wiring the real backend's
handling of it — this is shared, backend-free package surface and belongs to
its own task/commit per the file's "no first-party backend import" contract.
**Warning signs:** A type switch in `wiring.go` that has no corresponding
case in `store.go`'s `Action` interface (won't compile) — but also watch for
silently reusing `NewKey` with a bolted-on bool field, which WILL compile
and hide the distinction.

### Pitfall 4: No `RemoveProviderRewrite` function exists for D-09
**What goes wrong:** A plan that assumes "just call the existing insteadOf
removal function" for the last-identity-for-provider case will find
`RemoveURLRewritesBlock` [VERIFIED: internal/gitconfig/baseline.go:145-160]
removes the WRONG thing — the GLOBAL baseline's static `url-rewrites` block
(Phase 7/GGIT-01 concern), not the PER-PROVIDER `provider-rewrite:<host>`
block `WriteProviderRewrite` creates per identity
[VERIFIED: internal/gitconfig/renderer.go:159-223].
**Why it happens:** Two different insteadOf mechanisms coexist in the
codebase for two different phases' requirements; only the writer half
(`WriteProviderRewrite`) and a presence-check (`HasProviderRewrite`) exist
for the per-provider one — there is no remover.
**How to avoid:** Add `gitconfig.RemoveProviderRewrite(gitconfigPath,
provider string) (backupPath string, err error)` mirroring
`WriteProviderRewrite`'s shape but calling `filewriter.RemoveBlock` instead
of `ReplaceBlock`, keyed by the SAME `ProviderRewriteBlockName(provider)`
sentinel [VERIFIED: internal/gitconfig/renderer.go:159-165]. Ref-counting
itself (D-09) is new logic: count accounts sharing `Provider` across
`b.accounts()` before deciding whether to call it.
**Warning signs:** A delete test that never asserts the insteadOf block
SURVIVES when a sibling identity on the same provider still exists (the
opposite bug — over-eager removal — is just as real a risk as under-removal).

### Pitfall 5: `BackupAndRemove`'s convention is a sibling `.bak`, not an archive dir
**What goes wrong:** Using `filewriter.BackupAndRemove` directly for D-06's
key archival produces `~/.ssh/id_ed25519_<name>.bak.<nano>` sitting NEXT TO
the live keys — not inside `~/.ssh/gitid-archive/` as D-06 explicitly
requires (a separate, doctor-reserved directory).
**Why it happens:** `BackupAndRemove` and `Write`'s backup path is always
`<targetPath>.bak.<UnixNano>` in the SAME directory
[VERIFIED: internal/filewriter/filewriter.go:130-149] — there is no
directory-redirect parameter.
**How to avoid:** Write a small new helper (`internal/keygen/archive.go` or
similar) that composes `filewriter.EnsureDir` (create `gitid-archive/` at
0700) + a copy-then-remove using the SAME collision-proof exclusive-create
idiom `copyFileExclusive` uses [VERIFIED: internal/filewriter/filewriter.go:
198-213], targeting `filepath.Join(archiveDir, filepath.Base(keyPath)+"."+
timestamp)`. Register the directory via an extension to
`sshconfig.ReservedPaths` (D-06 MANDATORY).
**Warning signs:** grep for `gitid-archive` returning zero hits after a
"D-06 done" claim.

### Pitfall 6: `newRootCmd()` is a near-blank slate — SHELL-03 has no prior CLI to extend
**What goes wrong:** REQUIREMENTS.md marks `SHELL-03` `[x]` "carried" from
the pre-redesign POC, which could mislead a planner into assuming a CLI tree
exists to extend. The POC's Cobra tree (`identity add/list/test/rotate/
update/delete/copy`, `baseline`, `doctor`, `adopt`, `host`, `add repo`) was
**archived at D-14**, explicitly named in `newRootCmd`'s own doc comment
[VERIFIED: cmd/gitid/main.go:69-73: "D-14 archived the 0.0.1 POC command
surface... the v1.0 CLI surface is rebuilt deliberately in Phase 5
(SHELL-03)"].
**Why it happens:** REQUIREMENTS.md's status legend `[x]` means "built
substrate to reuse," but for SHELL-03 the "substrate" is Cobra itself + the
`debug`/`completion` precedent, not a command tree.
**How to avoid:** Plan SHELL-03 as genuinely greenfield Cobra work: 7+ new
subcommands (`create|list|show|clone|new-key|rotate|delete`) each calling
into the SAME domain functions the TUI calls (D-04 outcome parity), with the
D-02 adaptive TTY-detection logic and D-03 `--json`/table output as
cross-cutting concerns.
**Warning signs:** A plan task titled "extend the existing identity CLI
commands" — there is nothing to extend yet beyond the root/debug/completion
scaffold.

## Code Examples

### Existing Rotate skeleton (extend, do not discard)
```go
// Source: internal/identity/modes.go:181-212 (verified, read this session)
func Rotate(existing Account, deps Deps) (CreateResult, error) {
    in := rotateInput(existing)
    staged, err := deps.Generate(in)
    if err != nil {
        return CreateResult{}, fmt.Errorf("identity: generating rotation key: %w", err)
    }
    defer deps.Cleanup(staged)
    return runPipeline(in, staged, deps)
}
```
`runPipeline` is the shared four-writer pipeline used by `Reuse`/`AddAccount`/
`Rotate` [VERIFIED: internal/identity/identity.go:470-559] — its
`WriteAllowedSigners` call at line 549 is the exact spot D-07's append
behavior must be injected (either a new Deps field, e.g. `AppendAllowedSigners`,
or a Rotate-specific pre-step that pre-composes the combined body before
calling the existing writer).

### Existing shared-key detection (reuse for D-12)
```go
// Source: cmd/gitid/wiring.go:1594-1607 (verified, read this session)
func (b *realBackend) keyOwners() map[string]string {
    owners := make(map[string]string)
    for _, acct := range b.accounts() {
        if acct.KeyPath == "" {
            continue
        }
        label := acct.Name
        if provider := providerFromAlias(acct.Alias); provider != "" {
            label += " (" + provider + ")"
        }
        owners[acct.KeyPath] = label
    }
    return owners
}
```

### Existing per-provider rewrite writer (mirror for the D-09 remover)
```go
// Source: internal/gitconfig/renderer.go:180-202 (verified, read this session)
func WriteProviderRewrite(gitconfigPath, provider string, enabled bool) (string, error) {
    name, err := ProviderRewriteBlockName(provider)
    if err != nil {
        return "", err
    }
    if !enabled {
        return "", nil
    }
    body, err := RenderProviderRewrite(provider)
    if err != nil {
        return "", err
    }
    existing, err := os.ReadFile(gitconfigPath)
    if err != nil && !os.IsNotExist(err) {
        return "", fmt.Errorf("reading %s: %w", gitconfigPath, err)
    }
    backupPath, err := filewriter.Write(gitconfigPath, filewriter.ReplaceBlock(existing, name, body), gitconfigMode)
    if err != nil {
        return "", fmt.Errorf("writing provider rewrite block to %s: %w", gitconfigPath, err)
    }
    return backupPath, nil
}
```
The needed `RemoveProviderRewrite` is the same shape with
`filewriter.RemoveBlock(existing, name)` in place of the `ReplaceBlock` call
and no `enabled`/`body` parameters.

### Existing superstring-principal-safe matching (reuse for D-13's scan guard)
```go
// Source: internal/gitconfig/reader.go:185-206 (verified, read this session)
fields := strings.Fields(line)
if len(fields) >= 1 && fields[0] == identityEmail && strings.Contains(line, `namespaces="git"`) {
    continue // remove this exact-principal line
}
```
CONTEXT.md's D-13 discretion note ("the superstring-principal pattern in
`reader_test.go` is the analog") points at exactly this: an unmanaged-
reference scan for D-13 must match the alias as a whole token (e.g. word- or
line-boundary match), never a bare `strings.Contains`.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| POC Cobra tree (`identity add/list/test/rotate/update/delete/copy`, `baseline`, `doctor`, `adopt`, `host`, `add repo`) | Archived wholesale at D-14 (Phase 3) | 2026-07-25 (03-03) | Phase 5's SHELL-03 CLI is a from-scratch rebuild against the NEW noun-verb taxonomy (D-01), not a port of the old flat-verb commands — do not reuse old command names/flags as a spec. |
| Single-line `allowed_signers` per identity (create/update model) | Two-line-per-identity tolerated on rotate only (D-07) | This phase (Phase 5) | The `RemoveAllowedSignersLine`/`RemoveAllowedSignersBlock` removal helpers already handle multiple lines correctly (block-keyed removal takes the WHOLE block regardless of line count) — no removal-side change needed, only the WRITE side needs the append extension. |

**Deprecated/outdated:** none within this phase's scope — all extended
functions are current, actively-tested code, not legacy to retire.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|----------------|
| A1 | New-key (KEY-07/MGR-05 repair path) should use a SINGLE-line allowed_signers write (like the normal create/update path), not an append — because its target states (key-missing, shared-key) have no valid prior signing material worth preserving. | Common Pitfalls #2 | If wrong, a repaired identity could lose OR duplicate a signing line incorrectly; this needs explicit confirmation during planning/discuss, since CONTEXT.md's D-07 text says "on rotate" without explicitly ruling on new-key's allowed_signers behavior. |
| A2 | The `gitid-archive` directory should be registered via an ADDITIVE extension to the existing `sshconfig.ReservedPaths`/`IsReservedPath` functions (mirroring the `config.d` precedent) rather than a wholly separate registry. | Standard Stack, Pitfall 5 | Low risk — this mirrors CONTEXT.md's own "MANDATORY: register the archive directory as doctor-reserved" instruction and the project's documented Phase-4 D-04 precedent, but the exact mechanics (a new `ReservedPaths` call site vs. a parallel function) are Claude's Discretion per CONTEXT.md. |
| A3 | A distinct `tuikit.RotateIdentity` Action type (not an overload of `NewKey`) is the correct extension to `internal/tuikit/store.go`. | Common Pitfalls #3, Standard Stack | Medium risk if wrong direction chosen — this is architecture-level and affects both binaries' shared package; flag for explicit sign-off in the plan review since it touches package surface neither binary owns exclusively. |
| A4 | The exact Cobra command file layout (one file per verb vs. grouped) is Claude's Discretion per CONTEXT.md and not independently verified against any external convention beyond "gh/glab as the model" already named in CONTEXT.md. | Recommended Project Structure | Low risk — purely organizational; CONTEXT.md explicitly defers this. |

**If this table is empty:** N/A — see above; none of these blockers are load-bearing for whether the phase is achievable, only for exact shape.

## Open Questions

1. **Does D-07's append rule extend to new-key (KEY-07/MGR-05), or is it rotate-only?**
   - What we know: CONTEXT.md's D-07 text is scoped explicitly to "rotate."
   - What's unclear: New-key's target states (key-missing, shared-key-downgrade)
     could theoretically also have a stale allowed_signers line pointing at a
     dead/absent key that new-key should either replace or append to.
   - Recommendation: Default new-key to the existing single-line
     `WriteAllowedSigners`/`ReplaceBlock` semantics (replace, matching the
     current create/update behavior) unless the user says otherwise during
     `/gsd-plan-phase`'s review — flagged as A1 above.

2. **Should the Cobra CLI's `--json` output schema for `list`/`show` include the raw `Problems []Problem` slice from `IdentityHealth`, or only the collapsed `ClassifyState` label?**
   - What we know: D-03 says "marshals the existing 8-state identity model" —
     ambiguous between the single-label `ClassifyState` output and the richer
     two-axis `Classify`/`IdentityHealth` output.
   - What's unclear: Whether downstream automation (the phase's own
     stated PTY-free e2e assertion use case) needs the finer-grained Problems
     detail or just the single label.
   - Recommendation: Explicitly Claude's Discretion per CONTEXT.md — the
     planner should pick the richer `IdentityHealth` shape (it is a strict
     superset and can always be flattened by a CLI consumer), and record it
     as the frozen schema per D-03's "keep it stable once shipped" note.

## Environment Availability

No new external tool/service dependency for this phase. `git`, `ssh`,
`ssh-keygen` are already required runtime dependencies established in prior
phases (Phase 1 `platform.ProbeKeyTypes`, already used by
`AlgorithmCatalog` [VERIFIED: cmd/gitid/wiring.go:416-421]). `gh`/`glab`
upload automation is explicitly out of scope for Phase 5 (Phase 9, D-08's
copy-.pub + hint pattern only).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `go test -race` [VERIFIED: Makefile test targets grepped this session] |
| Config file | none — plain `go test`, orchestrated via `Makefile` (`test`, `test-e2e`, `lint` targets) |
| Quick run command | `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./internal/identity/... ./internal/gitconfig/... ./internal/keygen/... ./internal/tuikit/...` |
| Full suite command | `make test && make test-e2e && make lint` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| KEY-05 | Rotate archives old key, appends allowed_signers line, keeps old line | unit | `go test ./internal/identity/... -run TestRotate` | ❌ Wave 0 (extend `modes_test.go`) |
| KEY-07 | New-key repairs key-missing without touching pre-existing material | unit | `go test ./internal/identity/... -run TestRepair` (name TBD) | ❌ Wave 0 (new file) |
| MGR-04 | Clone re-derives all identity-scoped fields, never copies verbatim | unit | `go test ./internal/identity/... -run TestClone` | ❌ Wave 0 (new `clone_test.go`) |
| MGR-06 | Delete `--all` ref-counts insteadOf, backs up key before removal, downgrades on shared key | unit | `go test ./internal/identity/... -run TestDelete` (extend `delete_test.go`) | ⚠️ partial — `delete_test.go` exists (432 lines) but predates D-09/D-11/D-12/D-13 |
| SHELL-03 | Every CLI verb produces the same on-disk result as the equivalent TUI action | e2e | raw-keystroke PTY (`e2e/*_pty_e2e_test.go`) + a new CLI-only e2e suite | ❌ Wave 0 — no `cmd/gitid` command e2e tests exist yet beyond `debug`/`completion` |
| DLV-06 | Every new screen (clone-name-prompt, delete-choice, confirm-destructive, backup-notice, rotate ceremony) has a live PTY e2e | e2e | `e2e/git_configuration_pty_e2e_test.go`-style new file(s) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** package-scoped `go test -race ./internal/...` for the touched package(s).
- **Per wave merge:** `make test && make test-e2e && make lint` (orchestrator-run, per project's ground-rule-4 convention — never trust an executor's self-reported PASS, per project memory "Review gate -race vs executor non-race").
- **Phase gate:** full suite green + `make gate-visual-regression` (existing golden-text gate) before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/identity/modes_test.go` — extend for D-06/D-07/D-08 Rotate behavior (archive, append, grace hint surfaced in result).
- [ ] `internal/identity/repair_test.go` (or similar new file) — KEY-07/MGR-05 new-key repair.
- [ ] `internal/identity/clone_test.go` — MGR-04 clone re-derivation (same-key and new-key variants).
- [ ] `internal/identity/delete_test.go` — extend for D-09 (ref-count), D-11 (key archive path), D-12 (shared-key downgrade), D-13 (unmanaged scan).
- [ ] `internal/gitconfig/renderer_test.go` — new `TestRemoveProviderRewrite`.
- [ ] `internal/keygen/archive_test.go` (new) — the D-06 archive-to-dedicated-dir helper.
- [ ] `internal/sshconfig/include_test.go` — extend `TestReservedPaths`/`TestIsReservedPath` for the `gitid-archive` addition.
- [ ] `internal/tuikit/store_test.go`/`identities_test.go` — new `RotateIdentity` Action + Reduce case, plus wiring the existing `paneClone`/`paneDeleteScope`/`paneDelete` panes to real Backend calls.
- [ ] `cmd/gitid/*_test.go` (new, per Cobra subcommand) — flag parsing, D-02 TTY-branch logic, D-03 `--json`/table output.
- [ ] `e2e/` — new PTY e2e files for the rotate/clone/delete-choice/confirm-destructive/backup-notice screens (DLV-06), and a CLI-only e2e suite exercising each SHELL-03 command headlessly.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|---------------------|
| V2 Authentication | no | gitid is a local single-user CLI/TUI; no auth surface. |
| V3 Session Management | no | Not applicable — no sessions. |
| V4 Access Control | partial | File permission enforcement is the analog: keys 0600, `.pub` 0644, `allowed_signers` 0600/0644 per existing convention [VERIFIED: internal/keygen/signers.go:13 `allowedSignersMode = 0o644`; internal/gitconfig/reader.go:173 `0o600` on write]; the archive dir must be 0700 / archived keys 0600 per CONTEXT D-06 and D-11. |
| V5 Input Validation | yes | `sshconfig.ValidateHostBlock`, `gitconfig.validateIncludeIf`/`validateEmail`, and CR-18's comma-injection guard in `keygen.AllowedSignersLine` [VERIFIED: internal/keygen/signers.go:31-40] are the existing standard controls — every NEW string interpolated into a managed block (clone's re-derived alias, the archive filename) must route through the SAME validators, never a new ad hoc check. |
| V6 Cryptography | yes | Never hand-roll — `internal/keygen`'s existing `GenerateMaterial`/registry (ed25519 via `golang.org/x/crypto/ssh`) is the only key-generation path; Rotate/new-key call the SAME `Generate` dep, no new crypto code. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|------------------------|
| Comma-injection into `allowed_signers` PRINCIPALS field via a crafted email | Tampering/Elevation of Privilege | Already fixed project-wide via `AllowedSignersLine`'s comma rejection (CR-18, commit `8c5936b` per project memory) — any NEW code path that builds an allowed_signers line (the D-07 append helper) MUST go through `AllowedSignersLine`, never construct the line string directly. |
| Substring/superstring principal match deleting the wrong identity's trust | Tampering | Exact first-field match + `namespaces="git"` check, already implemented [VERIFIED: internal/gitconfig/reader.go:185-206] — D-13's new unmanaged-reference scan MUST use the same exact-token-boundary discipline, not `strings.Contains`. |
| Symlink-based path traversal on a user-typed manual key path | Tampering | Already rejected before parsing for the reuse-key picker (T-03-13, per CONTEXT canonical refs) — any new user-typed path input (e.g. a manual clone-key-choice) must apply the same symlink rejection. |
| Irreversible delete presented as safe / backup claim overclaiming | Repudiation (of the user's own informed consent) | D-11's exact required phrasing ("removed from active use; a copy exists at <backup path>" — never "cannot be undone" for the key when a backup exists) is a project-level trust invariant, not just UX polish; the confirm-destructive screen's stronger "cannot be undone" language is reserved for the truly-irreversible parts of the everything-delete (the SSH Host block / gitconfig block removal, which via git-only mode ARE recoverable from backup, but the ceremony must not conflate "backed up" with "reversible in one click"). |

## Sources

### Primary (HIGH confidence — read directly this session)
- `/Users/ramon/git/personal/ssh-git-config/.planning/phases/05-identity-manager/05-CONTEXT.md` — locked decisions D-01..D-17.
- `/Users/ramon/git/personal/ssh-git-config/.planning/REQUIREMENTS.md` — MGR/KEY/SHELL/DLV requirement text + traceability table.
- `/Users/ramon/git/personal/ssh-git-config/recipes/ssh-config.recipe`, `recipes/gitconfig.recipe`, `recipes/README.md` — canonical config shape.
- `/Users/ramon/git/personal/ssh-git-config/internal/identity/{state,identity,delete,modes,update,loader,inventory}.go` — domain substrate read in full.
- `/Users/ramon/git/personal/ssh-git-config/internal/tuikit/{backend,store}.go` — Backend contract + Action/DemoState/Reduce read in full.
- `/Users/ramon/git/personal/ssh-git-config/internal/keygen/signers.go`, `internal/gitconfig/{reader,renderer,baseline}.go`, `internal/sshconfig/include.go`, `internal/filewriter/{block,filewriter}.go` — writer/reader/reserved-path substrate.
- `/Users/ramon/git/personal/ssh-git-config/cmd/gitid/{main,wiring}.go` — composition root, Cobra scaffold, `Persist`/`Commit*` async pattern.
- `/Users/ramon/git/personal/ssh-git-config/.planning/design/identity-manager/FIELDS.md` — frozen Phase-2 screen/field manifest.
- `/Users/ramon/git/personal/ssh-git-config/.planning/STATE.md`, `05-DISCUSSION-LOG.md` — decision history and current project position.
- `/Users/ramon/git/personal/ssh-git-config/CLAUDE.md`, `.planning/config.json` — project conventions and workflow toggles.

### Secondary (MEDIUM confidence)
- None used this session — all findings verified directly against source.

### Tertiary (LOW confidence)
- None — no WebSearch was needed; this phase's research question was entirely answerable from the repository's own substrate and locked CONTEXT.md decisions.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; every referenced package/function was read directly this session.
- Architecture: HIGH for what exists (Backend/Action/Reduce/mutationJournal patterns, all read directly); MEDIUM for the shape of NEW functions (Rotate extension, repair, clone-with-new-key, D-09/12/13 delete extensions) since these are design recommendations, not yet-written code.
- Pitfalls: HIGH — every pitfall traces to a specific file:line read this session, not inference.

**Research date:** 2026-08-25
**Valid until:** 30 days (stable internal substrate; re-verify if Phase 4's circuit-breaker follow-up or any interim hotfix touches `internal/identity`, `internal/keygen`, `internal/gitconfig`, or `cmd/gitid/wiring.go` before Phase 5 planning begins).
