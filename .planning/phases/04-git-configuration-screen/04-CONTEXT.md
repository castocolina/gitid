# Phase 4: Git Configuration Screen - Context

**Gathered:** 2026-07-07
**Status:** Ready for planning (plan AFTER Phase 3 executes — this phase builds on `internal/tuikit` and the real wizard from Phase 3)

<domain>
## Phase Boundary

Wire the real backend behind the approved git-screen design: per-identity Git
fragment form (`user.name`, `user.email`, `gpg.format=ssh` fixed,
`user.signingkey` path, `commit.gpgsign`) with live fragment preview →
match-strategy select (`gitdir:` default / `hasconfig:` / both) with live
`includeIf` preview → read-only review (byte-identity affordance for
`allowed_signers` vs `user.email`) → confirm → write fragment +
`~/.gitconfig` blocks + `~/.ssh/allowed_signers` with backups. Unlocks the
wizard's `[ Continue ]` (removes Phase 3's D-19 disabled reason).

Requirements: GITUI-01..05, DLV-04, DLV-06. Design FROZEN by Phase 2
(`git-screen/FIELDS.md` 7 states); the only new design surface is the scoped
divergences decided below, each documented and allowlisted for the visual
gate.

**Explicitly NOT in this phase:** the Identity Manager list/detail UI and its
`g`-launch surface (Phase 5 — but it REUSES this phase's flow), global git
options / remaining recipe defaults (Phase 7), health checks on git artifacts
(Phase 8).

</domain>

<decisions>
## Implementation Decisions

### insteadOf URL rewriting (resolves the W1 carry-over)
- **D-01 — insteadOf ships in Phase 4's ceremony.** ⚠ User override of the
  deferral recommendation: the git write ceremony also writes the `[url]`
  `insteadOf` block. W1 is CLOSED by this phase, not Phase 7.
- **D-02 — Recipe-literal, per provider.** The block is
  `[url "git@<provider>:"] insteadOf = https://<provider>/` — one managed
  block per PROVIDER (not per identity), written the first time any identity
  for that provider gets git config, idempotent for later identities. The
  ceremony trigger is per-identity; the content is per-provider. Never
  alias-targeted (two identities on one provider must not fight over the
  https prefix).
- **D-03 — Toggle, default ON.** A checkbox row on the git form ("Force SSH
  over HTTPS for <provider>", frozen ☑ glyph pattern), default checked.
  Declining skips the insteadOf write for that ceremony. Scoped divergence:
  one new form row + one new preview block.
- **D-04 — MANDATORY: register the insteadOf managed block as a RESERVED
  gitconfig block** so the doctor never classifies it as an identity/orphan
  (the known destructive `--fix` false-positive loop — see project memory
  "doctor reserved-block false-positive loop").

### SSH-only completion path (and edit)
- **D-05 — Wizard resume affordance.** When the create form's alias collides
  with an existing gitid-managed identity, Phase 3's D-09 inline error
  becomes an offer to jump straight to the git step for that identity (SSH
  steps skipped). Small copy divergence, documented.
- **D-06 — ONE reusable git-form flow, create-mode AND edit-mode (DRY —
  user's explicit direction).** The collision offer reads "complete its Git
  config" for SSH-only identities and "edit its Git config" for complete
  ones (edit-mode pre-fills from the parsed fragment). Phase 5's manager
  `g`-launch reuses this exact flow — build it as a reusable component, not
  wizard-internal code. Edit invariants: idempotent whole-block rewrite of
  all three targets; a changed `user.email` REPLACES the identity's
  `allowed_signers` line (keyed by identity), never appends beside the stale
  one; byte-identity (GITUI-04) holds after every edit.

### gitdir path derivation (GITUI-03)
- **D-07 — Auto-derive `~/git/<identity>/` + editable row.** The default
  follows the user's `~/git/<client>/` layout convention. An editable
  single-row field appears under the radio group when `gitdir`/`both` is
  selected, live-reflected in the `includeIf` preview. Scoped divergence:
  one new field row. gitdir paths keep the trailing slash (gitdir semantics).
- **D-08 — Missing gitdir directory is CREATED by the ceremony**, listed in
  the confirm-write preview ("create directory ~/git/personal/").
- **D-09 — `hasconfig:` writes the SSH pattern only:**
  `hasconfig:remote.*.url:git@<ssh-host>:*/**`. The recipe's https variant is
  intentionally dropped (dead weight with D-03 default-ON; matches the
  approved demo preview). "both" = the gitdir block + this one block.

### Write ceremony shape
- **D-10 — ONE combined ceremony in the wizard.** The final review stacks all
  previews (SSH `Host` block + fragment + `~/.gitconfig` includeIf/insteadOf
  blocks + `allowed_signers` line + mkdir note); ONE explicit confirm writes
  everything; one backup notice lists all backup paths; one result screen.
  The Skip-Git path stays SSH-only (Phase 3 D-18). The standalone/edit flow
  (D-05/D-06) runs the git-screen's own approved 7-state ceremony.
- **D-11 — All-or-nothing rollback.** On any failure mid-write, every
  already-written file is restored from the backups taken at ceremony start
  (new files removed — Phase 1's "empty backupPath = did not pre-exist"
  convention); the failure screen names exactly what failed. The user's
  system is never left half-configured.
- **D-12 — Edit mode renders a true before/after `-/+` diff** of changed
  lines (the fixer's approved highest-risk-affordance pattern; e.g. an email
  change shows old and new `allowed_signers` lines). Create mode keeps plain
  previews. This ships the exact review UI Phase 5's manager reuses.

### Claude's Discretion
- Exact copy for: the insteadOf toggle row, the gitdir path row label, the
  collision-offer text ("complete" vs "edit" variants), combined-review
  section headers. Draft during the UI wave; freeze via the 02-STYLE-SPEC §6
  grep mechanism.
- `allowed_signers` line mechanics (managed-line identification keyed by
  identity; file has no sentinel-block convention — pick and document one).
- Where the reusable git-form flow component lives inside `internal/tuikit`
  (or a sibling package) — must stay consistent with Phase 3's extraction.
- Preview row-budget fitting for the combined review at 100×30 (bounded
  previews with clip cues are the established pattern).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### North Star — canonical config end state
- `recipes/gitconfig.recipe` — canonical `~/.gitconfig` shape: `includeIf` blocks, per-identity fragment fields, per-provider `insteadOf` (D-02's exact source).
- `recipes/ssh-config.recipe` — alias model the `hasconfig:` pattern derives from.
- `recipes/README.md` — wiring overview; structure not key type.

### Requirements / roadmap (authoritative)
- `.planning/ROADMAP.md` §"Phase 4" — goal + 5 success criteria.
- `.planning/REQUIREMENTS.md` §G (GITUI-01..05; GITUI-02/03/04 substrate built, 01/05 pending), §A (DLV-04/06).

### Approved design (BINDING)
- `.planning/design/git-screen/FIELDS.md` — the 7 git-screen states + parity rows (`allowed-signers-byte-identity`, `match-strategy-default-gitdir`).
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md` — theme roles, copy freeze §6, arrow-key precedence.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-DESIGN-DECISIONS-CHECKPOINT-2.md` — D1–D9 contract (divergence-documentation precedent).
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-UX-DIRECTION.md` — §4(2) git-screen states, §5 mutation ceremony beats, key-allocation table (`g` launch).

### Phase 3 hand-off (read its SUMMARYs once executed)
- `.planning/phases/03-create-flow-backend/03-CONTEXT.md` — D-18 (Skip functional), D-19 (Continue disabled reason this phase REMOVES), D-16 (demo-banner treatment), D-17 (`internal/tuikit`), D-24/D-25 (visual-gate mechanics Phase 4's UI wave reuses).
- `.planning/phases/03-create-flow-backend/03-UI-SPEC.md` — TUI-adapted contract format to mirror for Phase 4's UI wave.

### Substrate
- `internal/gitconfig/` — fragment renderer/reader, includeIf resolve, baseline.go (existing insteadOf write logic to reuse/refit).
- `internal/adopter/` — gitconfig-fragment adopt (NOT the SSH-layout detector — known naming trap).
- `internal/keygen/` — allowed_signers derivation.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/gitconfig/fragment.go` + `renderer.go` + `reader.go` — fragment
  render/parse (GITUI-02 "built"); reader powers D-06 edit-mode pre-fill.
- `internal/gitconfig/baseline.go` — existing insteadOf block logic (POC) to
  refit for D-02's per-provider managed block.
- `internal/gitconfig/includeif_resolve_test.go` — includeIf resolution proof
  pattern to extend for gitdir/hasconfig/both.
- `internal/filewriter/` — backup + idempotent block rewrite + rollback
  substrate for D-11 (rollback treats empty backupPath as did-not-pre-exist).
- `internal/keygen/signers.go` — allowed_signers line derivation (GITUI-04).
- `internal/tuikit` (created by Phase 3) — shell frame, theme, form rows,
  ceremony screens; the reusable git-form flow (D-06) extends it.
- Fixer's `-/+` diff render pattern (Phase 2, 02-10) — the D-12 edit diff.

### Established Patterns
- Mutation ceremony beats (§5): preview → confirm → backup notice → result;
  bounded previews with clip cues at 100×30.
- Copy freeze via repo-wide grep gates; new frozen strings (D-03 toggle,
  D-05 offer copy, D-07 row label) join 02-STYLE-SPEC §6.
- Scoped-divergence discipline: document next to the D9 precedent + per-screen
  allowlist entries for the golden-text visual gate (Phase 3 D-24 mechanics).
- Reserved-block registration for any NEW managed gitconfig block type (D-04).

### Integration Points
- Wizard git step (Phase 3 leaves it demo'd): D-19 reason removed; Continue →
  real git form; combined final review (D-10).
- Alias-collision validator (Phase 3 D-09): extended into the D-05 offer.
- Phase 5 manager: consumes the D-06 reusable flow via `g`; consumes D-12's
  edit review. Phase 7 Global Git: no insteadOf work left (D-01 closes W1) —
  its GGIT-01 scope note should be updated accordingly at its discuss step.
- UI wave: Phase 4 runs its own `/gsd-ui-phase 4` (UI-SPEC for the divergence
  set) + PTY e2e per screen + golden-text gate + agent-ui-ux-designer + Codex
  review (per Phase 3's D-24/D-25 conventions).

</code_context>

<specifics>
## Specific Ideas

- User override, verbatim intent: insteadOf belongs WITH the identity
  ceremony (Phase 4), not deferred to Phase 7 — W1 closes here.
- "Why not create the same workflow for both … we will apply DRY principle" —
  the git-form flow is ONE component with create/edit modes, built in
  Phase 4, reused by Phase 5's manager.
- gitdir default follows the user's real directory convention
  `~/git/<client>/` → `~/git/<identity>/`.

</specifics>

<deferred>
## Deferred Ideas

- **Manager list/detail + `g`-launch surface** — Phase 5 (reuses this phase's
  D-06 flow and D-12 review; do not build list UI here).
- **CLI `gitid git <identity>` command** — Phase 5 (SHELL-03 rebuilds the CLI
  surface; the resume affordance covers Phase 4's need).
- **Global recipe defaults (aliases, pull.rebase, fetch.prune, etc.)** —
  Phase 7 (GGIT-01). insteadOf is no longer part of Phase 7's scope (D-01).
- **hasconfig https:// variants** — intentionally dropped (D-09); revisit
  only if a real user declines D-03's toggle AND clones over https.
- **Git-artifact health checks** (fragment exists, includeIf targets valid) —
  Phase 8 (HLTH/FIX).

</deferred>

---

*Phase: 4-Git Configuration Screen*
*Context gathered: 2026-07-07*
