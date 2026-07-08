# Phase 5: Identity Manager - Context

**Gathered:** 2026-07-07
**Status:** Ready for planning (plan AFTER Phases 3 and 4 execute — the manager
builds on `internal/tuikit`, the real wizard, and Phase 4's reusable git-form
flow)

<domain>
## Phase Boundary

Wire the real backend behind the approved identity-manager design — the app's
HOME view: per-row completeness/health list (MGR-02's 8-state taxonomy,
reconstructed by parsing managed blocks, no sidecar DB), SSH-first detail that
never fabricates git fields, and the per-identity lifecycle actions — clone,
new-key, rotate, delete-with-choice — each running the established mutation
ceremony (preview → confirm → backup notice → result). Phase 5 also rebuilds
the Cobra CLI surface (SHELL-03) with full outcome parity and shell
completions, and makes all five primary views reachable via palette + number
keys (SHELL-02; unbuilt views keep Phase 3 D-16's demo-content + warning-note
treatment).

Requirements: MGR-01, MGR-03..08, KEY-05, KEY-07, SHELL-01..03, plus the
DLV-04/DLV-06 UI-wave gates. Design FROZEN by Phase 2
(`identity-manager/FIELDS.md`, 8 states); scoped divergences decided below are
documented + allowlisted for the visual gate, per the D9/Phase-3-D-24
precedent.

**Explicitly NOT in this phase:** the real Global SSH / Global Git / Health /
Fixer view content (Phases 6–8), upload automation (Phase 9 — rotate/new-key
only emit the copy-.pub + hint pattern), layout migration UX (Phase 6/8).

</domain>

<decisions>
## Implementation Decisions

### CLI parity surface (SHELL-03)
- **D-01 — Noun-verb taxonomy + flat aliases.** `gitid identity
  create|list|show|clone|new-key|rotate|delete`, with noun groups `gitid ssh`,
  `gitid git`, `gitid health`, `gitid fix` reserved for Phases 6–8. Cobra
  aliases keep `gitid create` (etc.) working at top level. The SHELL-02
  five-view set supplies the noun groups; no flat-verb collisions when later
  phases add actions.
- **D-02 — Adaptive gh-style non-interactive depth.** Complete flags →
  headless run; incomplete flags + TTY → the TUI wizard opens pre-filled from
  the given flags; incomplete flags + non-TTY → error listing the missing
  flags. Reads (`list`, `show`, `health`) are always headless. `--yes`
  replaces ONLY the confirmation prompt; `--dry-run` runs the test + preview
  stage and stops; **timestamped backups are unconditional — no flag can skip
  them**; the post-write re-test result drives the exit code. Both CLI and
  TUI MUST call the same test → backup → write → re-test chokepoint (no
  behavioral fork).
- **D-03 — `--json` + TTY-aware plain output.** `list`/`show` render an
  aligned table on a TTY, tab-delimited when piped, and `--json` marshals the
  existing 8-state identity model. The JSON output is also the PTY-free e2e
  assertion surface (reduces PTY-scraping in tests — a documented CI pain
  point).
- **D-04 — SHELL-03 = outcome parity, verified by a parity matrix.** Every
  product OUTCOME (create, clone, new-key, rotate, delete `--git-only`/
  `--all`, list, show, health, fix) has a CLI command; ceremony steps are
  internal invariants, never separate commands. A requirement-keyed parity
  matrix (MGR-01..08 / KEY-05 / KEY-07 → command) makes ROADMAP success
  criterion 4 checkable; Phases 6–9 must update it when adding actions.

### Key lifecycle: rotate vs new-key (KEY-05, KEY-07, MGR-05)
- **D-05 — Rotate = retirement ceremony; new-key = repair action.** Rotate
  (healthy identities): archive the old key, provider-cleanup guidance,
  re-test. New-key (repair for `key-missing` / shared-key states): generate a
  fresh key and re-point, NEVER touching pre-existing key material (it may be
  absent or referenced by sibling identities). Two ceremonies sharing the
  keygen + re-point core.
- **D-06 — Old key archives to `~/.ssh/gitid-archive/<file>.<timestamp>`**
  (dir 700, keys 600) inside the all-or-nothing transaction, vacating the
  canonical `id_ed25519_<name>` path for the new key. **MANDATORY: register
  the archive directory as doctor-reserved** (the known destructive `--fix`
  false-positive loop — same class as Phase 4 D-04).
- **D-07 — `allowed_signers` APPENDS on rotate.** The new pubkey line is
  added and the old line KEPT (same email, old blob) so
  `git log --show-signature` keeps verifying pre-rotation commits.
  Managed-line logic must tolerate two lines per email. Pruning archived
  keys/lines is a later fixer concern, not Phase 5.
- **D-08 — Post-rotate test gate reuses Phase 3 D-01/D-02/D-03 verbatim.**
  Expected landing state is ReachableNotUploaded (yellow `!` "Reachable — key
  not uploaded yet") + copy-.pub + provider hint. One added grace-window hint
  line: old key remains valid at <provider>; upload the new key, verify, then
  remove the old one. Copy extends (never alters) the frozen D-02/D-03
  strings; new strings join the §6 copy-freeze grep. IdentityFile and
  user.signingkey stay textually unchanged (same canonical path); only the
  allowed_signers block + key files change.

### Delete semantics (MGR-06)
- **D-09 — Per-provider insteadOf block is reference-counted.** Deleting the
  LAST identity for a provider removes the block, named explicitly in the
  confirm screen's file list (no extra ceremony question — the frozen
  two-option delete-choice stands). An orphaned rewrite would break ALL
  https clones for the provider. Ref-count must account for hand-written
  aliases targeting the same provider (do not remove if unmanaged config
  still needs it — tie into D-13's scan).
- **D-10 — `allowed_signers` line is KEPT in "Git identity only" mode**,
  removed only in "delete everything". The key survives git-only delete, so
  historic signature verification keeps working and the safer option stays
  actually safer. The doctor/health taxonomy must tolerate a fragment-less
  principal without flagging it as drift.
- **D-11 — "Delete everything" backup-copies the key pair into the
  timestamped backup dir before deletion** (backup dir 700, key copies 600).
  filewriter gains backup-of-deleted-file semantics (extends the
  empty-backupPath = did-not-pre-exist convention). Confirm copy must be
  precise: "removed from active use; a copy exists at <backup path>" — never
  overclaim irreversibility. Fragment + includeIf are both removed in
  git-only mode too, with the fragment backed up before unlink (orphaned
  fragments are the known doctor-loop bug class).
- **D-12 — Shared key auto-downgrades "delete everything".** When the
  identity's key is referenced by another identity (the `key-used-*`
  taxonomy states already compute this), SSH + Git artifacts are deleted but
  the key is kept, with an explicit note naming the sibling identity that
  still uses it.
- **D-13 — Unmanaged-reference scan, warn-never-block.** Before delete, the
  alias is scanned across the unmanaged regions of files gitid already
  parses (`~/.ssh/config` foreign text, `~/.gitconfig`, fragments,
  allowed_signers); hits are named in the confirm screen, plus a fixed
  honest warning that repo remotes using `git@<alias>:` cannot be scanned
  and will break.

### Clone semantics (MGR-04)
- **D-14 — Copy + re-derive pre-fill.** SSH fields copied with the new alias
  substituted; `user.name`/`user.email` pre-filled but visually flagged
  "copied from <source> — review" (small copy divergence, documented +
  allowlisted); gitdir (`~/git/<new-name>/`, Phase 4 D-07), `hasconfig:`
  pattern, signingkey, and the allowed_signers line are ALL re-derived from
  the NEW name and key choice — never copied verbatim.
- **D-15 — Clone customization runs inside the create wizard, entered
  pre-filled.** clone-name-prompt (frozen screen) answers name + key choice,
  then the full Phase 3/4 wizard opens with initial model state injected.
  This inherits the D-01 test gate, D-04 auto-chain, D-09 collision
  blocking, the D-06 reusable git form, the D-10 combined ceremony, and
  D-11 rollback — ONE write path, no standalone clone pipeline. The wizard
  gains a pre-filled-entry mode (also required by CLI D-02's flag pre-fill).
- **D-16 — Same-key clones re-run the FULL two-stage test gate.** The new
  Host block's `ssh -G` resolution is the genuinely unproven artifact;
  auto-chaining makes the cost invisible and gate semantics stay identical
  between create and clone.
- **D-17 — Suggested name `<source>-clone`, auto-bumped** to
  `<source>-clone-2` when the suggestion itself collides, with live Phase-3
  D-09 validation of the derived alias against ALL parsed Host patterns —
  the prompt never opens in an error state.

### Claude's Discretion
- Exact Cobra flag names/shorthands and the parity-matrix document location/
  format.
- JSON schema shape for `list`/`show` (marshal the existing identity model;
  keep it stable once shipped).
- gitid-archive filename convention details and the reserved-registration
  mechanics (mirror Phase 4 D-04's approach).
- Exact copy for: the grace-window hint, shared-key downgrade note,
  unmanaged-reference warning, "copied from <source> — review" flag, and the
  precise delete-everything confirm phrasing — draft in the UI wave, freeze
  via 02-STYLE-SPEC §6.
- Scan implementation for D-13 (substring false-positive guard — the
  superstring-principal pattern in `reader_test.go` is the analog).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### North Star — canonical config end state
- `recipes/ssh-config.recipe` + `recipes/gitconfig.recipe` — the alias model,
  fragment fields, and per-provider insteadOf every lifecycle action must
  keep coherent.
- `recipes/README.md` — wiring overview; structure not key type.

### Requirements / roadmap (authoritative)
- `.planning/ROADMAP.md` §"Phase 5" — goal + 5 success criteria.
- `.planning/REQUIREMENTS.md` §H (MGR-01..08), §C (KEY-05/06/07), §N
  (SHELL-01..03), §A (DLV-04/06).

### Approved design (BINDING)
- `.planning/design/identity-manager/FIELDS.md` — the 8 frozen manager
  screens (list-populated/list-empty/detail-ssh-first/action-menu/
  clone-name-prompt/delete-choice/confirm-destructive/backup-notice) +
  parity rows (`delete-choice-safe-default`, `no_color-row-health`,
  `ssh-first-detail`); key allocation `a`/`c`/`d`, launch keys `n`/`g`
  belong to create-flow/git-screen.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md` —
  theme roles, copy freeze §6, arrow-key precedence.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-UX-DIRECTION.md` —
  §4(3) manager states, §5 mutation ceremony beats, §2 key-allocation table.

### Upstream phase contracts (read their CONTEXT now; SUMMARYs once executed)
- `.planning/phases/03-create-flow-backend/03-CONTEXT.md` — D-01/D-02/D-03
  test-gate semantics this phase reuses for rotate/clone; D-09 collision
  validator; D-16 demo-banner treatment for unbuilt views; D-17
  `internal/tuikit`; D-24/D-25 visual-gate + Codex review mechanics.
- `.planning/phases/04-git-configuration-screen/04-CONTEXT.md` — D-02
  per-provider insteadOf (D-09 here ref-counts it); D-04 reserved-block
  registration precedent; D-06 reusable git-form flow + D-12 edit diff the
  manager's `g`-launch consumes; D-11 all-or-nothing rollback.

### Substrate
- `internal/identity/state.go` — the 8-state taxonomy (MGR-02 built);
  `key-used-*` states drive D-12's ref-count.
- `internal/keygen/` — registry, derive.go, signers.go (rotate/new-key core).
- `internal/filewriter/` — backup + rollback substrate; D-11 extends it with
  backup-of-deleted-file.
- `internal/gitconfig/baseline.go` (`RemoveURLRewritesBlock` — needs
  per-provider granularity for D-09) + `reader.go`
  (`RemoveAllowedSignersLine`, superstring-principal guard, reserved-block
  names).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/identity/` (inventory + state) — MGR-01/MGR-08 list
  reconstruction already built; feeds rows + ref-counts.
- `internal/tuikit` (Phase 3) + the reusable git-form flow (Phase 4 D-06) —
  the manager's detail/edit surfaces compose these; no new form machinery.
- `internal/tester` — read-only two-stage gate, rewired verbatim into
  rotate/clone ceremonies.
- Phase 3's PATH-shim fake `ssh` + PTY e2e harness — extended to the manager
  screens; D-03's `--json` adds a PTY-free assertion channel.
- Cobra + `InitDefaultCompletionCmd` — completion generation is free once
  the noun-verb tree exists (stack doc: no extra library).

### Established Patterns
- Mutation ceremony beats (§5) apply to rotate/new-key/delete exactly as to
  create; delete adds the strongest-confirm + backup-notice screens (frozen).
- Reserved-block/reserved-path registration for anything new gitid owns
  (D-06 archive dir; Phase 4 D-04 precedent).
- Copy-freeze via §6 grep gates; scoped divergences documented + allowlisted
  for the golden-text visual gate (Phase 3 D-24 mechanics).
- One write chokepoint: CLI and TUI both call the same
  test → backup → write → re-test core (D-02) — mirrors the UI-free-core
  TDD rule in CLAUDE.md.

### Integration Points
- Identities view replaces its Phase 3 demo content; Global SSH/Git, Health,
  Fixer keep D-16 demo + warning banners until Phases 6–8.
- Manager `g`-launch → Phase 4's git-form flow (edit mode, `-/+` diff).
- Delete's insteadOf ref-count (D-09) reads the same managed blocks Phase 4
  D-02 writes.
- Phase 9 upload assist later replaces the copy-.pub + hint pattern rotate
  emits (D-08) — no Phase 5 coupling beyond the hint copy.
- UI wave: `/gsd-ui-phase 5` for the divergence set ("review" flag, grace
  hint, downgrade note) + PTY e2e per screen + golden-text gate +
  agent-ui-ux-designer + Codex review (Phase 3 D-24/D-25 conventions).

</code_context>

<specifics>
## Specific Ideas

- CLI modeled on gh/glab (the project's own Phase-9 peer tools): TTY prompts,
  non-TTY demands flags, `--json` for scripts — not on lazygit/k9s (those are
  companions to an external CLI; gitid's TUI and CLI are two skins over one
  core).
- "The backup is the undo story" (frozen copy) must hold for EVERY artifact —
  that is what forces D-11's key backup-copy and the precise confirm wording.
- Rotation follows the overlap convention: never destroy the only proven
  credential before its replacement is uploaded and verified (no auto-upload
  until Phase 9 makes this structural, not stylistic).

</specifics>

<deferred>
## Deferred Ideas

- **Pruning archived keys / stale allowed_signers lines** (`gitid key purge`
  or a fixer action) — Phase 8 (Health + Fixer); D-07/D-06 deliberately
  accumulate.
- **Compromised-key path** (immediate-invalidation guidance + true no-copy
  delete variant) — revisit post-v1.0 or Phase 8; Phase 5's delete/rotate
  default to the safe-overlap model.
- **gh-style `--json fields` + `--jq` field selection** — only if an
  automation ecosystem emerges; plain `--json` ships now.
- **Upload automation on rotate** (auto-upload the new .pub) — Phase 9
  (UP-01..03).
- **Real Global SSH / Global Git / Health / Fixer views** — Phases 6–8; the
  manager only routes to their D-16 demo screens.

</deferred>

---

*Phase: 5-Identity Manager*
*Context gathered: 2026-07-07*
