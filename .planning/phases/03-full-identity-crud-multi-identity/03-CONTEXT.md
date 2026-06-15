# Phase 3: Full Identity CRUD + Multi-Identity - Context

**Gathered:** 2026-06-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 3 delivers the **read/manage** half of gitid's identity lifecycle, building
on Phase 2's create-only write path. It adds four capabilities over the *set* of
identities:

- **List (IDENT-03):** `gitid identity list` shows every identity with key path,
  alias, provider, port, and match strategy.
- **Update (IDENT-04):** edit an identity's git name/email, signing on/off,
  provider/alias/port, and match strategy.
- **Delete (IDENT-05):** remove an identity's managed blocks (key optional) with
  confirmation and backup, preserving everything outside its blocks.
- **Reconstruct (IDENT-07):** on a cold start with no running state, rebuild the
  exact identity/account list purely from the gitid-managed blocks in
  `~/.ssh/config` and `~/.gitconfig` — **no sidecar database**.

Multi-identity coexistence is proven here: two identities on the same provider
coexist via distinct aliases, and `ssh -G <alias-A>` / `ssh -G <alias-B>` each
resolve to their own distinct `IdentityFile` (Phase 2 already wrote the alias +
`IdentitiesOnly yes` mechanics; Phase 3 proves the *set* stays coherent).

**Phase is in MVP mode** (`**Mode:** mvp` in ROADMAP.md): plans are vertical
slices. The reconstruction reader is the foundational slice (list, update, and
delete all key off it).

**In scope (Phase 3 requirements):** IDENT-03, IDENT-04, IDENT-05, IDENT-07.

**Out of scope:**
- **Global/baseline git config + global gitignore** (`core.excludesfile`,
  `ignorecase`, push/pull/fetch/color toggles, aliases, `insteadOf` URL rewrites)
  — newly scoped as **Phase 3.1** (GLOBAL-01, URLRW-01, GITIGNORE-01). See
  Deferred Ideas.
- Doctor's deep coherence/drift/orphan health checks (Phase 4) — Phase 3 only
  surfaces a *light* incompleteness marker in `list`, never a diagnosis or fix.
- Full Cobra command surface + shell completion and the TUI (Phase 5) — Phase 3
  wires minimal real `gitid identity list/update/delete` subcommands, same as
  Phase 2 wired a minimal real `identity add`.
- Identity **rename** as an in-place operation — name is immutable in Phase 3
  (rename = delete + recreate).

</domain>

<decisions>
## Implementation Decisions

### Reconstruction (IDENT-07)
- **D-01:** The **identity name** — the token in the `# BEGIN gitid managed:
  <identity>` sentinel — is the **canonical correlation key**. It is the only
  token present across all four artifacts (SSH Host block, `includeIf` block,
  fragment filename `~/.gitconfig.d/<identity>`, and the `allowed_signers` line
  via that fragment). Reconstruction enumerates sentinel block names across the
  managed files, then gathers each identity's pieces by that key. The SSH Host
  *alias* is a per-account attribute, not the primary key (one identity may own
  several aliases — IDENT-06).
- **D-02:** Reconstruction is **best-effort**. When an identity has an incomplete
  set of managed blocks (e.g. an SSH block with no matching `includeIf`, or a
  fragment with no `includeIf`), gitid still builds the `Account` from whatever
  exists and surfaces it in `list` with a **light "incomplete" marker** naming
  what's missing. It never silently hides partial state. Deep diagnosis and fixes
  stay in `gitid doctor` (Phase 4).

### List (IDENT-03)
- **D-03:** `list` shows the success-criterion columns — key path, alias,
  provider, port, match strategy — plus the D-02 incompleteness marker. It is
  descriptive + light-health only; it does not run coherence checks.

### Update (IDENT-04)
- **D-04:** The identity **name is immutable** in Phase 3. `update` edits
  everything *except* the name: git name/email, signing on/off, provider, alias,
  port, and match strategy. Renaming is done by delete + recreate. Rationale: the
  name is the correlation key *and* names the key file (`id_<algo>_<name>`), the
  fragment file, and every sentinel block — an in-place rename is a coordinated
  re-key + file-move + four-block rewrite, a different (larger) risk class than a
  field edit, deferred out of Phase 3.
- **D-05:** `update` **re-runs the resolved verification test** (`ssh -T <alias>`
  + `ssh -G <alias>`, parsing the resolved `identityfile` / `identitiesonly` /
  `user` / `hostname` / `port`) **only when a structural field changed**
  (alias/provider/port/match — anything that can change SSH resolution or repo
  matching). Pure fragment edits (email, git name, signing toggle) skip the
  network round-trip because they cannot change resolution.
- **D-06:** `update` follows the **same safe-write pattern as create**: timestamped
  backup → unified preview (diff against current) → single explicit confirm →
  idempotent whole-block rewrite. Toggling **signing off** removes the identity's
  `allowed_signers` line and the fragment's signing keys; toggling **on** adds
  them back.

### Delete (IDENT-05)
- **D-07:** Default behavior **keeps the private key**. Delete removes the
  identity's four per-identity artifacts (see D-08); a **separate explicit prompt**
  (default *no*) offers to also delete the key files. Rationale: block removal is
  reversible (re-run create), private-key deletion is not — the irreversible
  action must require a deliberate second confirmation.
- **D-08:** Delete's removal scope is **per-identity only**: its SSH `Host` block,
  its `includeIf` block, its fragment **file** (`~/.gitconfig.d/<identity>`, a
  whole file, not an in-file block), and its `allowed_signers` **line**. Shared /
  global blocks — the macOS `Host *` block and the global
  `gpg.ssh.allowedSignersFile` wiring — are **never touched**, even when deleting
  the last identity (doctor flags any now-orphaned global wiring later). Delete
  shows an explicit **"will remove" manifest** before a single confirm + backup.

### Claude's Discretion
- **List layout:** exact rendering is open. Default intent: **grouped by identity**
  (identity header with name/key path/git name+email, accounts/aliases nested
  beneath) for the human `list` view, with an optional **flat/parseable** flag
  (one row per account) if it earns its keep. Pick during planning/UI design.
- **Incompleteness marker copy** (exact wording/glyph of the "incomplete" flag) —
  planner's wording; keep it light (a marker + what's missing), not a diagnosis.
- **Minimal CLI subcommand shape** for `list`/`update`/`delete` — follow the
  Phase 2 pattern (real, minimal `gitid identity …` Cobra commands that the Phase
  5 surface builds on; not throwaway). Interactive prompts with sensible defaults,
  consistent with create.
- **New read-side primitives** — a managed-block **lister/reader** (enumerate
  block names + bodies) and a **block-remover** (`RemoveBlock`, the delete-side
  counterpart to the existing `filewriter.ReplaceBlock`) are needed; their exact
  package placement and signatures are the planner's call. Keep all mutation on
  the existing `internal/filewriter` safe-write chokepoint.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project intent, scope & locked decisions
- `.planning/ROADMAP.md` §"Phase 3: Full Identity CRUD + Multi-Identity" — goal +
  4 success criteria + MVP mode flag.
- `.planning/REQUIREMENTS.md` §IDENT (IDENT-03/04/05/06/07) + Acceptance Criteria
  + Definition of Done — the Phase 3 requirement IDs and what "done" means.
- `.planning/PROJECT.md` — Core Value, Constraints (Safety, Engineering, Signing,
  Platform), Key Decisions table.
- `CLAUDE.md` — working method (hypothesis → test → implement; test→confirm+backup
  →re-test for config), safe-write rules, managed-block sentinel format,
  `git config`-via-exec strategy, English-only, TDD.

### Prior phase (direct carry-forward — Phase 3 reads back what Phase 2 wrote)
- `.planning/phases/02-first-identity-end-to-end/02-CONTEXT.md` — Phase 2 write
  decisions Phase 3 must reconstruct: sentinel format `# BEGIN/END gitid managed:
  <name>` (D-130-equiv), alias form `<identity>.<provider>` (D-12), default match
  `gitdir:~/git/<identity>/` (D-13), four-artifact model, two-phase test flow
  (D-01..D-03), safe-write pattern.

### Architecture & pitfalls (for the new read-side + delete primitives)
- `.planning/research/ARCHITECTURE.md` — `internal/` package seams; `identity`,
  `sshconfig`, `gitconfig`, `filewriter` are the packages Phase 3 extends.
- `.planning/research/PITFALLS.md` — atomic-write + permission + round-trip
  pitfalls the reconstruction parser and `RemoveBlock` must honor.
- `.planning/research/STACK.md` — `kevinburke/ssh_config` (SSH parse), `git config`
  via `os/exec` (gitconfig reads via `--get-regexp`/`--list`).

### Target artifact structure (the end-state shape being read back)
- `.planning/references/target-sshconfig.md` — target `~/.ssh/config` structure.
- `.planning/references/target-gitconfig.md` — target `~/.gitconfig` structure
  (`include` + `includeIf`, both `gitdir:` and `hasconfig:` strategies).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/identity` `Account` struct** (`identity.go:25`) — already documented
  as "reconstructable from the managed blocks … the filesystem is the source of
  truth." This is the target model for IDENT-07; the in-memory translation already
  exists, the *reader* does not. Includes the gitid-managed target paths
  (FragmentPath/GitconfigPath/SSHConfigPath/AllowedSignersPath) the lifecycle
  modes use.
- **`internal/sshconfig.Parse`** (`parser.go:17`) — returns `*ssh_config.Config`;
  the SSH-side parse foundation for reconstruction (enumerate Host blocks → alias,
  hostname, port, IdentityFile, IdentitiesOnly).
- **`internal/filewriter.ReplaceBlock`** (`block.go:29`) + `BeginPrefix`/`EndPrefix`
  sentinels — the write/upsert primitive. Phase 3 needs its **read** counterpart
  (enumerate block names + bodies) and **remove** counterpart (`RemoveBlock`).
- **`internal/filewriter.Write`** (`filewriter.go:33`) — the single safe-write
  chokepoint (backup → temp → rename → chmod). All Phase 3 mutations (update,
  delete) route through it; no `os.WriteFile` elsewhere.
- **`internal/tester.Resolved` / `ParseResolved`** (`tester.go:113/126`) — the
  resolved `ssh -T`/`ssh -G` verification reused by update's structural re-test
  (D-05).
- **`internal/identity` modes** (`modes.go` — `Reuse`, `AddAccount`, `Rotate`) —
  existing lifecycle operations that prove the Deps-injection pattern Phase 3's
  update/delete should follow.

### Established Patterns
- Core is **test-first (TDD)**; config parse → render → parse must be round-trip
  stable (TOOL-04). Reconstruction is a natural round-trip test: write (Phase 2) →
  reconstruct (Phase 3) → assert the `Account` set matches.
- Every external effect is an **injected Deps function field** on the operation,
  so logic is fake-testable and TUI-reusable; no business logic in `cmd/`.
- `make test` / `make lint` (golangci-lint + gosec, hard-fail) gate every commit;
  pre-push runs `go test -race` + coverage. All content in English. Commits go to
  `main`; history squashed/compacted at each plan close + review.
- `gsd-tools.cjs` needs Volta's bin on PATH in non-interactive shells (GSD scripts
  only — not the Go build).

### Integration Points
- New `gitid identity list` / `update` / `delete` Cobra commands in `cmd/gitid`,
  each gathering input + building Deps from the real internal packages and calling
  an `identity` operation — mirroring how `add` orchestrates create.
- Reconstruction reads `~/.ssh/config` (via `sshconfig.Parse` + a managed-block
  lister) and `~/.gitconfig` / `~/.gitconfig.d/*` (via `git config` reads + block
  lister) and joins by the D-01 identity-name key into `[]Account`.

</code_context>

<specifics>
## Specific Ideas

- Reconstruction join key: the `# BEGIN gitid managed: <identity>` sentinel name,
  enumerated across `~/.ssh/config`, `~/.gitconfig`, and the fragment files (D-01).
- `list` columns: key path, alias, provider, port, match strategy + light
  "incomplete" marker (D-02/D-03).
- Update structural-change set that triggers re-test: alias, provider, port, match
  strategy (D-05).
- Delete removal manifest lists exactly: SSH Host block, includeIf block, fragment
  file path, allowed_signers line — then keep-key-by-default prompt (D-07/D-08).
- Multi-identity proof: two identities on one provider; `ssh -G <alias-A>` and
  `ssh -G <alias-B>` resolve to distinct `IdentityFile`s (Success Criterion 2).

</specifics>

<deferred>
## Deferred Ideas

- **Baseline / global git config + global gitignore (now Phase 3.1).** Surfaced
  during this discussion from the user's reference gists. gitid manages a shared
  baseline git config (`core.ignorecase=false`, `editor`/`autocrlf`/`pager`,
  `push.autoSetupRemote`, `pull.rebase`, `fetch.prune`, `color`, curated aliases)
  and a curated global gitignore via `core.excludesfile`, plus optional HTTPS→SSH
  `insteadOf` rewrites — all in idempotent managed blocks under the same
  backup→preview→confirm safety, readable back from disk. Routed out of Phase 3
  into the new **Phase 3.1** (requirements GLOBAL-01 + URLRW-01 promoted v2→v1,
  GITIGNORE-01 added). Canonical refs saved to `samples/gist-60f2f1d-gitconfig`
  and `samples/gist-2c98cff-ssh-config`. **Not** part of Phase 3.
- **Doctor checks the baseline (Phase 4).** "Nice to have": doctor detects
  `core.ignorecase` off, a missing/empty `excludesfile`, or absent curated
  excludes (`.log`, `.bak`, `.thumbs`, …). Belongs to Phase 4 (Doctor) once Phase
  3.1 establishes the baseline; noted to fold into the Phase 4 discussion.
- **Identity rename as an in-place operation.** Deliberately excluded from Phase 3
  (D-04) — rename = delete + recreate for now. A coordinated re-key + file-move +
  four-block rewrite could be a later enhancement.
- **TUI view/edit of identities and the baseline (Phase 5).**
- **`add repo`, adopt-fragments (ADOPT-01), automatic key upload (AUTOUP-01)** — v2.

### Reviewed Todos (not folded)
None — no pending todos matched this phase (todo.match-phase returned 0).

</deferred>

---

*Phase: 3-full-identity-crud-multi-identity*
*Context gathered: 2026-06-10*
