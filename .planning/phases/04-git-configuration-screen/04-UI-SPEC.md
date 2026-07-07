---
phase: 4
slug: git-configuration-screen
status: draft
shadcn_initialized: false
preset: none
created: 2026-07-07
---

# Phase 4 — UI Design Contract

> Visual and interaction contract for Phase 4 (Git Configuration Screen). Generated
> by gsd-ui-researcher, verified by gsd-ui-checker.

**This is a derivation, not a design exploration.** The design was approved
2026-07-06 by Pepe (DLV-08, `.planning/design/APPROVAL.md`) and is FROZEN. The
git-screen's 7 named states (`git-form-empty` → `result-success`) are BINDING,
lifted verbatim from `.planning/design/git-screen/FIELDS.md`. This document
consolidates that binding contract for Phase 4's scope (real backend behind the
git-config screen, insteadOf write, gitdir derivation, the reusable create/edit
git-form flow, the combined write ceremony, and all-or-nothing rollback) and
specifies, in full, the **only** new visual surfaces Phase 4 is allowed to
introduce — the twelve scoped divergences D-01..D-12 from
`.planning/phases/04-git-configuration-screen/04-CONTEXT.md`, plus the D-19
removal Phase 4 performs on Phase 3's contract. Every other value below is cited
from an approved upstream artifact, not re-decided. This project's stack is a Go
Bubble Tea v2 terminal app (not a web app); template sections written for
CSS/web design systems are translated to their TUI equivalent (theme roles
instead of hex/CSS tokens, 100×30 fixed capture geometry instead of responsive
breakpoints) — identical translation Phase 3's `03-UI-SPEC.md` established.

---

## Design System

Same as Phase 3 — no shadcn / npm component registry. The "design system" is
the central Go `Theme` (`internal/dummytui/theme.go`), mirrored 1:1 by role
name with the web reference skin's `theme.ts` per `02-STYLE-SPEC.md` §1. Both
already exist and are BINDING — Phase 4 consumes them, it does not create a
new one, and introduces zero new theme roles.

| Property | Value |
|----------|-------|
| Tool | none (no shadcn — Go TUI, not React/Next/Vite) |
| Preset | not applicable |
| Component library | central Go `Theme` struct (`internal/dummytui/theme.go`) — the same 12 semantic roles, ANSI-16 palette, used unmodified |
| Icon library | none — frozen glyph constants only: `glyphCheckOn/Off` (`☑`/`☐`), `glyphRadioOn/Off` (`●`/`○`), status glyphs `✓` healthy, `!` warning, `✗` error, `~` info. Phase 4 introduces **zero new glyphs** — the D-12 diff and D-08 mkdir note both reuse the EXISTING `+`/`-` diff-line convention (`internal/dummytui/frame.go` `stylePreviewLines`), which is color-only (no glyph prefix beyond the literal `+`/`-` diff character itself) |
| Font | terminal's own monospace |
| Capture geometry | fixed **100 cols × 30 rows** (`frameBodyRows(30) = 25` body rows), unchanged — kept identical for the D-24 visual-regression gate (Phase 3 mechanics, reused verbatim by Phase 4's UI wave per `04-CONTEXT.md` code_context "Integration Points") |

---

## Spacing / Layout Scale (TUI-adapted: rows and columns, not px)

The frozen row/column budget from `02-STYLE-SPEC.md`/`02-DESIGN-DECISIONS-CHECKPOINT-2.md`
stays in force unchanged. Phase 4 adds exactly the row slots the divergences
below require — no divergence adds an unbudgeted row; each is accounted for
explicitly.

| Token | Value | Usage | Source |
|-------|-------|-------|--------|
| Frame | 100 cols × 30 rows, fixed | capture + PTY e2e geometry | `02-STYLE-SPEC.md` §7 |
| Body rows | 25 | rows available below header/breadcrumb chrome | checkpoint-2 geometry anchor |
| Tightest pane on record | ~24 of 25 body rows (wizard step 2 / Git identity step, pre-Phase-4) | 1 row headroom before Phase 4's additions | `02-STYLE-SPEC.md` §7 |
| Field row height | exactly 1 row, EVERY state | D1 — fields never reflow; the D-07 gitdir row and D-03 toggle row both use this template | checkpoint-2 D1 |
| Marker gutter | 2 cells | `▸ ` (accent) focused, `  ` blank otherwise | checkpoint-2 D1 |
| Label column | `padRight(label, 16)` | bold in all states — `gitdir path` (11 chars) and the D-03 toggle label both fit | checkpoint-2 D1 |
| Radio group | 1 header row + 3 option rows, ALWAYS rendered | match-strategy group — UNCHANGED by Phase 4; D-07's gitdir row is a SEPARATE conditional row below the group, not a 4th radio option | checkpoint-2 D2 |
| Preview block | bounded to pane width, `maxLines` cap + `… (+n more)` clip cue, title spliced into the top border edge | `PreviewBlock(title, text, diff, width, maxLines)` — the D-10 combined review stacks FIVE of these; each MUST pass an explicit `maxLines` low enough that the stack fits 25 rows (see D-10 below) | `internal/dummytui/frame.go`; `02-STYLE-SPEC.md` §3 `preview-sizing` |

**Exceptions for this phase — every one budgeted against an EXISTING slot or an
explicit new row, none silently added:**

| Divergence | Row cost | Slot |
|---|---|---|
| D-03 toggle row | +1 row | new row 6 on `git-form-empty`/`git-form-filled` (git-form's field list, after `commit.gpgsign`) |
| D-07 gitdir path row | +1 row, CONDITIONAL (only when `gitdir`/`both` selected) | new row directly under the match-strategy radio group's 3 options, on `match-strategy-select` |
| D-08 mkdir preview line | 0 extra rows | folded into the EXISTING confirm-write preview block as one more `+`-prefixed diff line, not a new block |
| D-05/D-06 collision offer | 0 extra rows | reuses Phase 3 D-09's EXISTING inline-error row on the SSH form's alias field — copy changes, slot does not |
| D-10 combined review | stacks 5 `PreviewBlock`s (Host / fragment / includeIf+insteadOf / allowed_signers) + 1 confirm + 1 backup-notice line, ALL within the SAME 25-row body budget the wizard already uses | reuses the wizard's EXISTING step-3/4 review pane; each block's `maxLines` is tuned down (not the frame grown) — see D-10 |
| D-11 failure screen | new screen, budgeted identically to `result-success` (git-screen/FIELDS.md's own row shape: glyph + message + restore-hint = 3 rows) | new state `result-failure`, same row shape as the approved `result-success` |
| D-12 edit diff | 0 extra rows | renders through the SAME `PreviewBlock(..., diff=true, ...)` call `review-readonly`/`confirm-write` already make; only changed lines gain a `-`/`+` pair — row count is a function of how many fields changed, capped by the same `maxLines`/clip-cue mechanism as every other preview |

The measured acceptance number for the combined review (D-10) is **not
estimated here** — per `02-STYLE-SPEC.md` §7's own precedent ("empirically
re-verified against the final implementation, not the plan's original
estimate"), the executor re-measures against `make test-e2e`'s 100×30
raw-keystroke PTY walk once the fragment/includeIf/insteadOf/allowed_signers
blocks are real. If the stack does not fit, tighten each block's `maxLines`
(clip cues are the established absorption mechanism) — never grow the frame.

---

## Typography (TUI-adapted: semantic roles, not px sizes / weights)

Unchanged from Phase 3 / `02-STYLE-SPEC.md` §1 — Phase 4 introduces **zero new
roles**. The one notable reuse: D-08 (mkdir note) and D-12 (edit diff) both
render through the SAME `+`/`-` diff-line styling already shipped for the
Fixer (`internal/dummytui/fixplans.go`'s `Diff` field,
`internal/dummytui/frame.go` `stylePreviewLines`) — `+`-prefixed lines render
`styleHealthy` (green), `-`-prefixed lines render `styleError` (red), every
other line renders `styleFaint`. This is an EXISTING, already-tested
mechanism (`PreviewBlock(title, text, diff=true, width, maxLines)`); Phase 4
does not invent a new diff renderer, it calls the existing one with real data.

| Role | TUI treatment | Usage in Phase 4 |
|------|----------------|-------------------|
| Label | `Bold(true)` | D-03 toggle label, D-07 gitdir-row label — same template as every other field label |
| Field (value) | plain, no styling | git-form fields, gitdir path value |
| Field — focused | `Foreground(ANSI 4) + Bold`, no border (D1) | D-03/D-07 rows when focused |
| Field — blurred | `Faint(true)` | D-03/D-07 rows when blurred |
| Hint | `Faint(true)` | D-05 "complete/edit its Git config" collision offer's action hint |
| Warning | `Foreground(ANSI 3)` — yellow | not used by any Phase 4 divergence (D-05's collision stays `Error`-tier — see Color below) |
| Error | `Foreground(ANSI 1)` — red | D-05/D-06 collision offer (still blocks the SSH-collision path); D-11 failure screen; `-`-prefixed diff lines (D-08, D-12) |
| Healthy | `Foreground(ANSI 2)` — green | `result-success` (unchanged); `+`-prefixed diff lines (D-08, D-12) |
| Info | `Foreground(ANSI 6)` — cyan | not used by any Phase 4 divergence |
| Preview | `Faint(true)` + dashed border | every `PreviewBlock` in D-03/D-09/D-10/D-11/D-12 |

**Non-negotiable, carried forward unchanged:** every colored state pairs with
a glyph AND a word — never color alone. D-08's mkdir line and D-12's diff
lines are the one narrow exception already accepted project-wide (the `+`/`-`
diff character itself IS the glyph, paired with the literal changed text as
the "word" — identical to the Fixer's existing `fix-preview` state, which
already passed the checker under this same convention).

---

## Color

ANSI-16 only, unchanged. Phase 4 reuses the existing role table exactly;
no new hue, no new role.

| Role | Value | Usage in Phase 4 |
|------|-------|-------------------|
| Dominant surface | terminal default fg/bg | body text, plain field values |
| Secondary | `Faint`/`Hint` (dim) | hint copy, blurred fields, the D-05 action hint |
| Accent (ANSI 4, blue) | `FieldFocused`, `ActiveArea`, `ActiveNav`/`ActiveNavDimmed` | D-03/D-07 rows when focused — no new accent usage |
| Warning (ANSI 3, yellow) | `Theme.Warning` | **not used** by any D-01..D-12 divergence — see the D-05 note below |
| Error/Destructive (ANSI 1, red) | `Theme.Error` | D-05/D-06 collision offer (reuses Phase 3 D-09's existing Error-tier row — the alias collision still blocks THIS SSH-creation attempt, it now also offers an escape hatch); D-11 failure screen; diff `-` lines |
| Healthy (ANSI 2, green) | `Theme.Healthy` | `result-success` (unchanged); diff `+` lines (D-08, D-12) |
| Info (ANSI 6, cyan) | `Theme.Info` | not used |

**Explicit note on D-05's color choice (documented reasoning, not a new
decision):** the collision offer could plausibly read as advisory (`Warning`,
yellow) since it now offers a path forward. It stays `Error` (red) because
the underlying fact is unchanged from Phase 3's D-09 — the CURRENT SSH-form
submission is still blocked; the offer is a way OUT of the block (jump to a
different flow), not a downgrade of the block's severity. Same role, same
glyph (`✗`), only the trailing copy changes.

Accent reserved for (unchanged list, Phase 4 adds no new element):
focused-field marker + value color, the active header nav tab, the
breadcrumb/divider line above an active pane, the wizard stepper's active
segment + dots-so-far.

---

## Copywriting Contract

Frozen copy is governed by the `02-STYLE-SPEC.md` §6 grep-gate mechanism.
Phase 4 must NOT invent new copy for anything already frozen (`FIELDS.md`'s 7
git-screen state labels, the four-beat ceremony language, `Write it (Enter)`,
etc.). It drafts only the strings below, which join that same mechanism once
approved — each marked **DRAFT** is Claude's-discretion copy per
`04-CONTEXT.md`, to be frozen via the §6 grep once the UI wave's screenshots
are approved.

| Element | Copy | Status |
|---------|------|--------|
| Primary CTA (combined ceremony + standalone git-screen ceremony) | `Write it` (rendered `Write it (Enter)`) | CITED — `identities.go` `reviewCeremony()`; `FIELDS.md` `confirm_action` row; unchanged for create AND edit mode |
| D-03 toggle row label | `Force SSH over HTTPS for <provider>` | DRAFT |
| D-03 preview block title | `~/.gitconfig (insteadOf block — preview)` | DRAFT |
| D-05/D-06 collision offer — SSH-only variant (replaces Phase 3 D-09's plain inline error, same row) | `✗ Alias already used by "<name>" (SSH-only) — complete its Git config instead? [ Jump to Git step ]` | DRAFT |
| D-05/D-06 collision offer — complete variant | `✗ Alias already used by "<name>" (complete) — edit its Git config instead? [ Jump to Git step ]` | DRAFT |
| D-07 gitdir path row label | `gitdir path` | DRAFT |
| D-07 gitdir path default value | `~/git/<identity>/` (trailing slash kept — gitdir semantics) | CITED from D-07, not invented |
| D-08 mkdir preview line (rendered as a `+`-prefixed diff line inside the confirm-write preview) | `+ create directory ~/git/<identity>/ (does not exist yet)` | DRAFT |
| D-09 hasconfig-only includeIf preview | `[includeIf "hasconfig:remote.*.url:git@<ssh-host>:*/**"]` — the recipe's `https://` variant is DELIBERATELY OMITTED (default-ON D-03 makes it dead weight) | CITED from D-09, not invented |
| D-10 preview block titles (new/extended) | `~/.ssh/config (Host block — preview)` · `~/.gitconfig.d/<identity> (fragment file — preview)` · `~/.gitconfig (includeIf + insteadOf — preview)` · `~/.ssh/allowed_signers (signing line — preview)` | DRAFT (titles) — content is recipe-literal, not invented |
| D-11 failure heading | `✗ Write failed — nothing was changed` | DRAFT |
| D-11 body — what failed | exact target file + exact error, e.g. `Writing ~/.ssh/allowed_signers failed: permission denied` | DRAFT, pattern-matches the existing `test-fail` convention (exact command/output, never a generic message) |
| D-11 body — restored-list intro | `All files restored to their prior state:` followed by each restored target + the backup path it was restored from | DRAFT |
| D-12 edit-mode diff | reuses the Fixer's existing `-`/`+` line convention verbatim on real changed fields, e.g. `- <email>@old.example namespaces="git" ssh-ed25519 …` / `+ <email>@new.example namespaces="git" ssh-ed25519 …` | CITED mechanism (`fixplans.go`'s `Diff` field pattern), DRAFT content shape |
| Empty state | not applicable — Phase 4's own scope has no list surface; `git-form-empty` is a field-emptiness state (already named/approved), not a list-empty landing | CITED, `FIELDS.md` |
| Destructive confirmation | **NONE in Phase 4.** Create-mode is a non-destructive create; edit-mode is a non-destructive REWRITE of an existing identity's Git config (never a delete). Cancel stays default-focused per the universal ceremony rule, but the strongest-confirm pattern stays reserved for Identity Manager's full-delete (Phase 5) | CITED, `02-UX-DIRECTION.md` §5 |

### D-19 removal (Phase 3's contract, closed by Phase 4)

Phase 3's D-19 wrote a REAL-binary-only disabled reason under `[ Continue ]`:
`— Git configuration arrives with the next build` (`cmd/gitid`'s wizard render
path only; `internal/dummytui`'s demo string
`— needs user.name + a valid email` was explicitly untouched). Phase 4 makes
`[ Continue ]` LIVE in the real binary — the git backend now exists — so:

- The executor DELETES `— Git configuration arrives with the next build` from
  `cmd/gitid`'s wizard render path entirely (no replacement string; the button
  becomes enabled/disabled purely on the SAME client-side validity check the
  demo already uses — `— needs user.name + a valid email`, now shared by both
  binaries for the same reason for the first time).
- The `02-STYLE-SPEC.md` §6 copy-freeze grep's assertion (a) (`cmd/gitid` must
  contain the D-19 string) is REMOVED; assertion (b) (`internal/dummytui` must
  still contain `needs user.name + a valid email`) is UNCHANGED — the demo
  string was never Phase-3-only, it is now also true of the real binary.
- **D-16 removal, same phase:** per Phase 3 D-16's own removal contract, Phase
  4 also deletes the `! Preview — demo data, not wired to your system yet`
  banner from EVERY git-screen view it wires (`git-form-empty` through
  `result-success`, plus the two Phase-4-new states `result-failure` and the
  edit-mode variants) — the banner stays on every OTHER still-unwired view
  (Identity Manager, Global SSH, Global Git, Health, Fixer), untouched.

---

## Approved Base States (`git-screen/FIELDS.md` — BINDING, unmodified)

Cited as-is, not re-decided. Each state below is unchanged by Phase 4 except
where a "+ Phase 4 addition" note names the exact divergence touching it.

| State | Goal | Phase 4 addition |
|---|---|---|
| `git-form-empty` | fragment form before any field filled | + D-03 toggle row (row 6) |
| `git-form-filled` | fragment form filled + live fragment preview | + D-03 toggle row (row 6), value defaults checked |
| `match-strategy-select` | choose `gitdir`/`hasconfig`/`both`, live `includeIf` preview | + D-07 gitdir path row (conditional); + D-09 hasconfig-only pattern in the preview |
| `review-readonly` | fragment + includeIf + allowed_signers together, byte-identity affordance | + D-03 insteadOf block joins the stack; + D-12 diff rendering in edit-mode only |
| `confirm-write` | preview + confirm across all 3 files | + D-03 insteadOf as a 4th target; + D-08 mkdir line; + D-10 combined-ceremony stacking when reached via the wizard |
| `backup-notice` | timestamped backup paths | + D-10's "one backup notice lists all paths" framing when combined |
| `result-success` | success result | unchanged; + sibling NEW state `result-failure` (D-11) for the failure path |

**Focal points (per-screen primary focus, for executor clarity):**
- `review-readonly` — primary focus: the byte-identity affordance — the
  `allowed_signers` email paired side-by-side with `user.email`.
- `confirm-write` — primary focus: the stacked preview blocks in D-10 order
  (SSH `Host` block → fragment → includeIf + insteadOf → `allowed_signers` →
  mkdir line), with the confirm action last.
- `result-failure` — primary focus: the result glyph (`✗` + word) and the
  message naming the exact target file + exact error, followed by the
  "everything restored" assurance.

**Highest-risk affordance carried forward unmodified (GITUI-04):** the
`allowed_signers` email shown byte-identical, side by side with `user.email`,
in `review-readonly`. D-06's edit-mode invariant extends this: after an edit
that changes `user.email`, the `allowed_signers` line is REPLACED (never
appended beside the stale one), and the byte-identity assertion must hold
AFTER the edit exactly as it holds after a create. This is the single
highest-risk correctness property in the whole phase — treat any UI that
could show a stale/duplicate `allowed_signers` line as a contract violation,
not a cosmetic issue.

---

## Scoped Divergences (D-01 through D-12)

These are the **only** new visual surfaces Phase 4 may introduce. Every one
reuses an EXISTING theme role, an EXISTING glyph, and either an EXISTING row
slot or an explicitly-budgeted new one (see the Spacing table above) — none
adds a new color, a new glyph, or an unbudgeted screen. Each is a documented,
allowlisted departure from the approved `FIELDS.md` states under the D-24
visual-regression gate (Phase 3 mechanics, reused verbatim), exactly like
D9's precedent in Phase 2 and D-02/D-16/D-19's precedent in Phase 3.

### D-01/D-02/D-03/D-04 — insteadOf URL rewriting (closes W1)

- **D-01 (scope statement):** the insteadOf write ships in Phase 4's ceremony,
  not deferred to Phase 7. No visual surface of its own — governs D-02/D-03.
- **D-02 (content, recipe-literal):** `[url "git@<provider>:"] insteadOf =
  https://<provider>/` — ONE managed block per PROVIDER, not per identity.
  Trigger is per-identity (any identity's ceremony can write it the first
  time); content is per-provider (never alias-targeted — two identities on
  the same provider share the block, never fight over it). Source:
  `recipes/gitconfig.recipe` lines 42-58 (the `[url "git@github.com:"]`
  family) — copy the SAME pattern for whichever provider the identity's `Host`
  block targets.
- **D-03 (the one new form row + one new preview block):**
  - **Row.** New row 6 on `git-form-empty`/`git-form-filled`, directly after
    `commit.gpgsign` (row 5). Uses the EXISTING D1 single-row field template
    with the EXISTING D3 frozen checkbox glyphs `☑`/`☐`. Default state:
    **checked** (`☑`).
    ```
    ▸ Force SSH over HTTPS for github.com                          ☑
    ```
  - **Preview block.** Appears starting at `review-readonly` (stacked
    alongside the fragment/includeIf/allowed_signers blocks) and shown fully
    at `confirm-write`. Title: `~/.gitconfig (insteadOf block — preview)`
    (DRAFT, see Copywriting). Uses the EXISTING non-diff `PreviewBlock`
    styling (`Faint` + dashed border) — create-mode never renders this block
    as a diff; only D-12's edit-mode review does.
  - Declining the toggle skips the insteadOf write for THIS ceremony only —
    does not remove a block another identity on the same provider already
    wrote.
- **D-04 (non-visual, MANDATORY executor note):** register the insteadOf
  managed block as a RESERVED gitconfig block-type so the doctor never
  classifies it as an identity/orphan — the known destructive `--fix`
  false-positive loop (project memory "doctor reserved-block false-positive
  loop"). This has no UI surface of its own; it is a data-model requirement
  the git-screen's write path must satisfy so Phase 8's doctor stays correct.

**D-24 allowlist entry:** `git-form-empty`/`git-form-filled` golden text gain
one new row (D-03's toggle) vs. the approved dummy goldens; `review-readonly`
and `confirm-write` golden text gain one new preview block. Register all four
screen IDs in the D-24.1 automated-gate allowlist; flag all four for D-24.2
reviewer critique (`agent-ui-ux-designer` + Codex).

### D-05/D-06 — SSH-only completion path and the reusable git-form flow

- **D-05 (copy divergence on an EXISTING row):** when the create flow's alias
  field collides with an existing gitid-managed identity (Phase 3 D-09's
  inline-error row), the copy changes from a plain block to an offer — see
  the two DRAFT variants in Copywriting. SAME slot, SAME `Error`/`✗`
  role+glyph, SAME "form won't advance as typed" behavior — the divergence is
  strictly the trailing sentence + a new keystroke affordance
  (`[ Jump to Git step ]`, bound to `Enter` while this row is showing).
  Pressing it launches D-06's reusable flow directly at its git-form step for
  the colliding identity, skipping all SSH steps.
- **D-06 (component contract, not a new visual surface):** ONE reusable
  git-form flow component serves BOTH create-mode (fresh identity, empty
  form) and edit-mode (existing identity, form PRE-FILLED from the parsed
  fragment via `internal/gitconfig/reader.go`). The component renders the
  SAME 7 approved `FIELDS.md` states in both modes; the ONLY visual
  difference between the two modes is D-12's diff rendering in
  `review-readonly`/`confirm-write` (edit-mode only — create-mode keeps plain
  previews, unchanged). Collision-offer copy variant selects the mode:
  "complete its Git config" → create-mode entered mid-flow (SSH-only
  identity, Git fields still empty); "edit its Git config" → edit-mode
  (complete identity, fields pre-filled).
  - **Component placement (Claude's discretion, resolved):** lives in
    `internal/tuikit` (Phase 3's D-17 extraction target), NOT wizard-internal
    code — Phase 5's Identity Manager `g`-launch (`04-CONTEXT.md` code_context
    "Integration Points") consumes the exact same component in edit-mode.
  - **Edit-mode invariants (behavioral, verified through the UI, not just
    backend):** idempotent whole-block rewrite of all three targets
    (fragment, `~/.gitconfig` includeIf, `allowed_signers`); a changed
    `user.email` REPLACES the identity's `allowed_signers` line (keyed by
    identity), never appends beside the stale one; GITUI-04 byte-identity
    holds after every edit, shown via D-12's diff in `review-readonly`.

**D-24 allowlist entry:** the SSH form's alias-collision error row (Phase 3's
own golden, not a Phase-4-named screen) gains new trailing copy + a new
keystroke affordance — register as an allowlisted diff on that specific row,
scoped "SSH-form alias-collision row, REAL-binary + dummy" (both binaries
carry this behavior since it's not a not-yet-wired placeholder).

### D-07/D-08 — gitdir path derivation (GITUI-03)

- **D-07 (the one new field row):** default value auto-derived as
  `~/git/<identity>/` (trailing slash kept, matching `gitdir:` includeIf
  semantics — see `recipes/gitconfig.recipe` lines 132-136,
  `[includeIf "gitdir:~/work/"]`). Editable single-row field appears
  CONDITIONALLY, directly under the match-strategy radio group's 3 always-
  rendered options, ONLY when `gitdir` (the default) or `both` is selected —
  never for `hasconfig` alone. Uses the EXISTING D1 field template:
  ```
  ▸ Match strategy  (←/→ change)
      ● gitdir (default) — applies inside ~/<identity>/
      ○ hasconfig — repos whose remote uses this alias
      ○ both — either condition (two includeIf blocks = OR)
  ▸ gitdir path      [~/git/<identity>/                ]
  ```
  Live-reflected in the `includeif_preview` field (`FIELDS.md` row 4 on
  `match-strategy-select`) — editing this row updates the `[includeIf
  "gitdir:…"]` block's path in real time, same live-preview mechanism the
  fragment form already uses.
- **D-08 (write-time consequence, zero new rows):** if the derived/edited
  gitdir directory does not exist, the ceremony creates it. Surfaced as ONE
  additional `+`-prefixed diff line inside the EXISTING confirm-write
  preview block (reuses the `stylePreviewLines` `+` = `styleHealthy` green
  convention — see Typography) — not a new preview block, not a new screen
  row.

**D-24 allowlist entry:** `match-strategy-select` golden text gains one new
conditional row (D-07) and the `includeif_preview` field's rendered text
changes shape (D-07's live path substitution); `confirm-write` golden text
gains one new diff line (D-08) inside the existing preview block content.
Register both screen IDs; flag both for reviewer critique.

### D-09 — hasconfig SSH-only pattern (content-only, zero new rows)

`hasconfig:remote.*.url:git@<ssh-host>:*/**` is the ONLY block written for
`hasconfig`/`both` — the recipe's `https://` variant
(`recipes/gitconfig.recipe` lines 93-94, 114-115) is deliberately dropped
(dead weight given D-03's default-ON insteadOf rewrite). This changes the
CONTENT of the `includeif_preview` field on `match-strategy-select` and the
corresponding preview block on `review-readonly`/`confirm-write` — one block
per selected strategy component, never two for the same component. No new
row, no new theme role; purely a content-fidelity note the executor must not
regress (do not port the recipe's https includeIf verbatim).

**D-24 allowlist entry:** same two screen IDs as D-07/D-08 (the content
change is additive to the same golden-text diff already registered there) —
no separate allowlist entry needed.

### D-10 — ONE combined write ceremony in the wizard

The wizard's final review (Phase 3's already-existing `reviewCeremony()`,
which today stacks the SSH `Host` block + fragment + includeIf into one
preview when `configureGit` is true) is EXTENDED, not replaced, to also stack:

1. `~/.ssh/config` — Host block preview (existing)
2. `~/.gitconfig.d/<identity>` — fragment preview (existing)
3. `~/.gitconfig` — includeIf **+ insteadOf** block preview (D-03 addition)
4. `~/.ssh/allowed_signers` — signing-line preview (NEW — the standalone
   git-screen ceremony already shows this per `FIELDS.md` `confirm-write`
   row 3; the wizard's combined ceremony gains it here for parity)
5. the D-08 mkdir note, folded into block 3's diff-line convention if the
   gitdir directory needs creating

ONE explicit confirm (`Write it (Enter)`, unchanged) writes everything. ONE
backup notice lists ALL backup paths (`~/.ssh/config`, `~/.gitconfig`,
`~/.ssh/allowed_signers` — `~/.gitconfig.d/<identity>` and a freshly-created
gitdir directory carry no backup, per the project's "empty backupPath = did
not pre-exist" convention). ONE result screen. The Skip-Git path (Phase 3
D-18) is UNCHANGED — it stays SSH-only, the combined ceremony never appears
for it. The standalone D-05/D-06 flow (collision resume, or Phase 5's
`g`-launch) runs the git-screen's OWN approved 7-state ceremony instead — it
does not stack the SSH `Host` block (no SSH work happens in that flow).

**Row-budget fitting (Claude's discretion, resolved as a method, not a
number):** each of the 4-5 stacked `PreviewBlock` calls passes an explicit
`maxLines` tuned low enough (2-4 lines + clip cue each is the established
pattern — see the Fixer's `fix-preview` state for precedent) that the total,
plus the heading/confirm/hint chrome, fits the 25-row body budget. This is
verified empirically against `make test-e2e`'s 100×30 PTY walk, not
pre-computed here (per `02-STYLE-SPEC.md` §7's own precedent of correcting
its row-budget number only after real implementation).

**D-24 allowlist entry:** the wizard's step-3/4 review-ceremony golden text
(currently a 3-block stack when `configureGit` is true) gains 1-2 new blocks
(insteadOf, allowed_signers) and, when applicable, one new diff line (mkdir).
Register as an allowlisted diff scoped "wizard combined-review ceremony,
`configureGit=true` path only" — the Skip-Git path's golden is unaffected.

### D-11 — All-or-nothing rollback (new screen: `result-failure`)

On ANY failure mid-write (of either the combined ceremony or the standalone
git-screen ceremony), every already-written file is restored from the
backups taken at ceremony start (files that did not pre-exist are REMOVED,
not "restored to empty" — the project's existing `filewriter` convention).
This is a NEW state, sibling to the approved `result-success` — `FIELDS.md`
only named the success path; `result-failure` is Phase 4's addition, same
row shape:

| # | Field | Order | Notes |
|---|-------|-------|-------|
| 1 | `result_glyph` | 1st | red `✗`, glyph + word per the standing contract |
| 2 | `result_message` | 2nd | names EXACTLY what failed — target file + real error, never a generic "something went wrong" (mirrors the existing `test-fail` state's "exact command + real output" convention) |
| 3 | `restore_hint` | 3rd | `All files restored to their prior state:` + each restored target + the backup path it came from |

Uses ONLY existing roles (`Theme.Error`, `Theme.Healthy` is NOT used here —
this is a failure result, not success) and the existing 3-row result-screen
shape `result-success` already established. No new glyph.

**D-24 allowlist entry:** `result-failure` is a NEW screen ID — not a diff
against an existing golden, a NEW golden to establish and register in the
manifest. Flag for full reviewer critique (D-24.2) since it has no prior
approved reference to diff against.

### D-12 — Edit-mode true before/after diff

Edit-mode's `review-readonly` and `confirm-write` states render CHANGED
fields as a `-`/`+` diff pair; UNCHANGED fields render plain (unchanged
`Faint`/preview styling, no diff markers). This reuses the Fixer's approved
`fix-preview` pattern EXACTLY — same mechanism, same roles, same call
(`PreviewBlock(title, text, diff=true, width, maxLines)`), only the data is
git-fragment/includeIf/allowed_signers content instead of an SSH `Host`
block or file-permission fix:

```
[user]
    name = Personal Identity
-   email = you@personal.example
+   email = you@newpersonal.example
    ...
- you@personal.example namespaces="git" ssh-ed25519 AAAA...
+ you@newpersonal.example namespaces="git" ssh-ed25519 AAAA...
```

Source of the role/glyph contract: `internal/dummytui/fixplans.go`'s `Diff`
field (e.g. `"- mode 0644 (world-readable)\n+ mode 0600 (owner only)"`) and
`internal/dummytui/frame.go`'s `stylePreviewLines` (`+` → `styleHealthy`,
`-` → `styleError`, no diff prefix → `styleFaint`). Create-mode NEVER uses
this — its previews stay plain, exactly as `FIELDS.md` already specifies.
This is the exact review UI Phase 5's manager reuses for its own edit path
(`04-CONTEXT.md` code_context).

**D-24 allowlist entry:** `review-readonly` and `confirm-write`, EDIT-MODE
CAPTURE ONLY, differ from the approved create-mode dummy goldens (which show
plain, non-diff previews) at exactly the lines that changed. Register as an
allowlisted diff scoped "git-screen edit-mode capture only" — the
create-mode capture is unaffected since D-12 never fires there.

---

## Registry Safety

Not applicable. This project has no shadcn / npm component registry (Go
Bubble Tea v2 TUI, no third-party UI blocks). Table included only for
template-shape completeness.

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| n/a | n/a | not applicable — no component registry in this stack |

---

## Inherited Non-Negotiables (quick reference for the executor)

Restated for convenience only — NOT decided by this document; each is cited
from its binding source and must not be re-litigated during Phase 4 planning
or execution.

- **Arrow-key precedence rule** (expanded-select > text-input-cursor >
  wizard-step-nav validity-gated forward/always-allowed back > Shift+←/→
  focus-override) — `02-STYLE-SPEC.md` §2. D-07's gitdir row is a plain text
  field (clause 2 applies — ←/→ move the cursor, never intercepted); the
  match-strategy radio group above it keeps clause 1.
- **Glyph contract:** every colored state pairs with a glyph AND a word,
  never color alone — `02-UX-DIRECTION.md` §2. D-08/D-12's diff lines satisfy
  this via the `+`/`-` character itself paired with the real changed text.
- **Four-beat mutation ceremony** (preview → confirm → backup notice →
  result) for every write, no exceptions — `02-UX-DIRECTION.md` §5. D-10's
  combined ceremony is still exactly four beats — it stacks MORE previews
  into beat 1, it does not add a beat. D-11's failure path is a variant of
  beat 4, not a new beat.
- **Copy-freeze grep gate**, extended per this document's DRAFT strings once
  approved — `02-STYLE-SPEC.md` §6. D-19's removal SHRINKS the gate (one
  assertion deleted); every other divergence GROWS it.
- **Recipe fidelity:** the live `Host` block, `includeIf`, and `insteadOf`
  preview text must match `recipes/ssh-config.recipe` /
  `recipes/gitconfig.recipe` structure exactly (alias per identity, `Port
  443`, `IdentitiesOnly yes`, per-provider `insteadOf`, `hasconfig:`/`gitdir:`
  — ed25519 not RSA) — `recipes/README.md`.
- **Click-to-focus (D8, checkpoint-2):** the ENTIRE rendered field/radio/
  checkbox row is the mouse hit target — applies unchanged to D-03's toggle
  row and D-07's gitdir row (no new hit-testing mechanism).
- **100×30 capture geometry** is kept identical for the D-24 gate — do not
  change frame dimensions to fit new copy; tighten `PreviewBlock` `maxLines`
  instead (D-10's row-budget note above).
- **Byte-identity affordance (GITUI-04)** — `allowed_signers` shown
  byte-identical to `user.email`, side by side, in `review-readonly` — holds
  in BOTH create-mode and edit-mode (D-06's replace-not-append invariant is
  what keeps it true post-edit).

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: PASS
- [ ] Dimension 2 Visuals: PASS
- [ ] Dimension 3 Color: PASS
- [ ] Dimension 4 Typography: PASS
- [ ] Dimension 5 Spacing: PASS
- [ ] Dimension 6 Registry Safety: PASS

**Approval:** pending
