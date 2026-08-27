---
phase: 7
slug: global-git-options
status: draft
shadcn_initialized: false
preset: none
created: 2026-08-27
---

# Phase 7 — UI Design Contract

> Visual and interaction contract for Phase 7 (Global Git Options). Generated
> by gsd-ui-researcher, verified by gsd-ui-checker.

**This is a derivation, not a design exploration** — same discipline as
`06-UI-SPEC.md` (the closest sibling: same master-detail archetype, same
advisory posture, same compressed ceremony). Read the "Known Divergence"
callout below FIRST; it changes what "frozen" means for this phase. Read the
"Data corrections" section SECOND — three of the 11 pinned rows carry
mismatches between `07-CONTEXT.md`'s D-08 (approved AFTER the fixture was
authored) and the live `internal/tuikit/design.go` fixture that must be
fixed before any new copy is drafted.

This project's stack is a Go Bubble Tea v2 terminal app (not a web app);
template sections written for CSS/web design systems are translated to their
TUI equivalent (theme roles instead of hex/CSS tokens, 100×30 fixed capture
geometry instead of responsive breakpoints) — the same translation Phases
3–6's own `0N-UI-SPEC.md` established.

---

## ⚠ Known Divergence: FIELDS.md's 6-screen model is SUPERSEDED by the live dummy

`.planning/design/global-git/FIELDS.md` (authored Phase 2 Wave 4, 02-08,
BEFORE Phase 2's own Wave 7/8 checkpoint-feedback passes) describes 6 named
screens (`options-list` → `option-detail` → `fix-preview` → `confirm-write`
→ `backup-notice` → `result-applied`) reached via keys `v`/`f`/`w`/`y`/`z`,
plus a separate D9 "email fallback" ceremony section. The live
`internal/tuikit/globalgit.go` — the file `cmd/gitid-dummy` **actually
runs today** on view `3` — implements the SAME compressed architecture
`06-UI-SPEC.md` already documented for global-ssh:

- **No standalone `option-detail` screen / no `v` key.** The right pane of
  the master-detail `options-list` view is a **live, always-visible detail
  panel** — moving the list selection (`↑`/`↓`) updates it in place. This
  carries the SAME information FIELDS.md's `option-detail` screen would have
  (current / recommended / explanation / advisory note), never hidden
  behind a second screen.
- **No separate `fix-preview` / `confirm-write` / `backup-notice` screens, no
  `f`/`w`/`y` keys.** Pressing **`a`** ("apply N selected") opens the
  shared `ceremonyModel` directly in **state A** (`baselineCeremony()`,
  `globalgit.go:130-139`), which renders the diff preview
  (`GlobalGitFullManagedBlockText`), the target file, and the backup path
  on ONE screen. `Enter` commits; `Esc` cancels without writing.
- **No separate `result-applied` screen with its own key (`z`).** Confirming
  moves the SAME ceremony to **state B** (the result receipt:
  `GlobalGitResultMessage`), acknowledged to return to `options-list`.
- **Multi-select toggle is `space`**, on the master list directly — no
  dedicated selection step.
- **D9's ceremony is a SECOND, independent instance of the SAME compressed
  ceremony shape** (`emailCeremonyFor()`, `globalgit.go:145-154`) — its own
  heading (`GlobalGitEmailCeremonyHeading`), its own diff annotation
  (`GlobalGitEmailDiffAnnotation`), its own result message
  (`GlobalGitEmailResultMessage`) — never folded into the baseline
  ceremony, exactly as `07-CONTEXT.md` D-05 requires ("own dedicated
  ceremony, never folded into the baseline block").

**What is genuinely still binding from FIELDS.md, unaffected by the
screen-count correction:** the exact 11-row option set + verbatim order
(§4.5, corrected per D-08 below), glyph+word-never-color-alone, the
main-vs-master highlight chip, the advisory-not-blocking posture, and the
D9 field/helper/advisory copy (amended by D-04/D-07 below).

**Action for the planner/executor:** treat `internal/tuikit/globalgit.go` +
`ceremony.go` as the authoritative interaction model. No design re-freeze is
required — this compaction already shipped project-wide (Phases 3–6). Do not
attempt to resurrect a `v`/`f`/`w`/`y`/`z` 5-screen chain.

---

## Data corrections required before drafting new copy (D-08)

`07-CONTEXT.md` D-08's approved 11-row pinned table (2026-07-07, via a
dedicated research round, AFTER `design.go`'s fixture was authored in Phase
2 Wave 4) disagrees with the live fixture on the following rows. Fix the
fixture FIRST (same discipline `06-UI-SPEC.md` applied to its own two-row
GSSH-01 mismatch), then re-run the visual-regression gate:

| Option | Fixture today (`design.go`) | D-08 pinned (BINDING) | Action |
|---|---|---|---|
| `merge.conflictstyle` | `Recommended: "diff3"`, unconditional | `Recommended: "zdiff3"`, **fallback `diff3` below git 2.35** (HARD gate — the only row where the version gate changes the WRITTEN value; old git ERRORS at merge time on the unknown `zdiff3` value) | Rewrite `GlobalGitOptions`' `merge.conflictstyle` row + `GlobalGitFullManagedBlockText`'s `conflictstyle = diff3` line to `zdiff3` (with the version-gated fallback path implemented in the write layer); this is the SAME fixture amendment `07-CONTEXT.md` D-08 flags against `internal/dummytui/data.go`'s `GlobalGitFullManagedBlockText`/strip-text — amend BOTH in ONE commit, before the copy freeze |
| `init.defaultBranch` | one-liner explains the 2028 history but carries no dynamic version signal | `≥ 2.28` **informational** gate | No value change (main is always written); ADD the D-13-pattern dynamic "your git: X.Y" non-contractual line to `option-detail`'s explanation slot, reusing `internal/deps.GitVersionAtLeast` — do not add a new probe |
| `core.ignorecase` | one-liner has no filesystem caveat | frozen copy MUST state the APFS caveat: `git init`/`clone` probe the filesystem and write repo-local `true`, which beats global — "per-repo detection overrides this" | DRAFT a new sentence appended to the existing one-liner/detail explanation (new copy, see Copywriting Contract below) |
| `push.autoSetupRemote` | one-liner has no version note | `≥ 2.37` **informational** gate ("old git silently ignores unknown KEYS — harmless to write") | Optional dynamic version line, LOWER priority than the two above (informational only, no behavior change either way) — Claude's discretion whether to render it given the row budget |
| `diff.colorMoved` | one-liner has no version note | `≥ 2.15` **informational** gate | Same as `push.autoSetupRemote` — optional, Claude's discretion |

All other rows (`core.autocrlf`/`core.eol`, `fetch.prune`, `pull.rebase`,
`alias`, `color`) already match D-08 byte-for-byte (value, no gate) —
verified directly against `design.go:305-327`.

**D-07 addition (NOT yet in the fixture):** a 12th advisory row,
`user.useConfigOnly`, default value `true` recommended / unchecked by
default (own opt-in row — recipes leave it unset, awareness-first). This is
a NEW row, not a correction — see "New row: `user.useConfigOnly` (D-07)"
below.

---

## Design System

Same as Phases 3–6 — no shadcn / npm component registry. The "design
system" is the central Go `Theme` (`internal/tuikit/theme.go`), used
unmodified. Phase 7 introduces **zero new theme roles and zero new glyphs**
(the `Error` role and its `styleError` alias already exist — reused for the
D9 email-validation inline message, not new).

| Property | Value |
|----------|-------|
| Tool | none (no shadcn — Go TUI, not React/Next/Vite) |
| Preset | not applicable |
| Component library | central Go `Theme` struct (`internal/tuikit/theme.go`) — reused unmodified; the shared `ceremonyModel` (`internal/tuikit/ceremony.go`), master-detail helpers (`joinMasterDetail`, `masterListWidth`, `frameBodyRows` — `internal/tuikit/frame.go`), and the create-flow wizard's `formFieldLine`/`helperLine` field-row renderers (`internal/tuikit/identities.go:413,426`, reused as-is for the D9 field) |
| Icon library | none — frozen glyph constants only: `glyphCheckOn/Off` (`☑`/`☐`, per-row apply checkboxes), status glyphs `✓` (already-applied/healthy), `!` (needs-action/advisory). Phase 7 introduces **zero new glyphs** |
| Font | terminal's own monospace |
| Capture geometry | fixed **100 cols × 30 rows** (`frameBodyRows(30)` body rows), unchanged — kept identical for the visual-regression gate |

---

## Spacing / Layout Scale (TUI-adapted: rows and columns, not px)

Global Git is the project's **master–detail** body archetype (same as
Identity Manager and Global SSH), with **no sub-tab strip** (unlike Global
SSH's Options/Storage split — global-git has a single body) and **no
findings-banner-plus-sub-tab stacking** to budget for.

| Token | Value | Usage | Source |
|-------|-------|-------|--------|
| Frame | 100 cols × 30 rows, fixed | capture + PTY e2e geometry | `02-STYLE-SPEC.md` §7 |
| Findings banner (conditional) | +1 row, only when the doctor has Git findings beyond this screen's own baseline (`gitBannerBeyond = "this baseline"`) | `findingsBanner(s, "Git", gitBannerBeyond)`, `globalgit.go:249-253` | existing, unchanged |
| Body archetype | master–detail: list ~44% width (left), detail ~56% width minus a 2-col gutter (right) | `masterListWidth`/`masterDetailGutter`, `frame.go` | existing, unchanged |
| Option row height | exactly **2 lines** per option on the master list (`optionRowLines`, shared constant with global-ssh): line 1 = selection marker + checkbox glyph + tone glyph + key name + optional `[main vs master]` chip; line 2 = indented `now: <current> → <recommended>` | `view()`, `globalgit.go:322-351` | existing, unchanged — **11 rows total** (was 6 in global-ssh) means the master list is roughly TWICE as tall; verify at PTY e2e time whether all 11 rows fit the 100×30 body budget without a scroll affordance, or whether the list needs to scroll/paginate — this is an OPEN layout question this phase must resolve (see UI Considerations, "overflow" row) |
| Detail pane | right pane, bounded to pane width, wrapped + a VISIBLE `…` clip cue (`fitPane`) if content overflows the row budget | `view()`, `globalgit.go:381-382` | existing, unchanged |
| D9 detail pane (email fallback row selected) | field row (`formFieldLine`) + inline validation error (conditional) + 2 always-visible helper/advisory lines | `view()`, `globalgit.go:356-369` | existing, unchanged |
| Ceremony preview (baseline) | full `GlobalGitFullManagedBlockText` (12 sections after the D-07 useConfigOnly addition) rendered inside the ceremony's existing preview slot | `baselineCeremony()`, `ceremony.go` viewport | existing, unchanged — verify the LONGER block (D-07's new row + D-08's version-gate annotations) still fits the ceremony's fixed viewport (`ExactTextViewport`, same 10-line/58-col budget global-ssh uses) without truncating a sentinel line |
| Ceremony preview (D9 email) | one annotated diff line (`+ [user]` / `+     email = …  (global fallback — …)`) | `emailCeremonyFor()`, `globalgit.go:145-154` | existing, unchanged |

**Phase 7 additions — every one budgeted against an EXISTING slot unless
flagged otherwise:**

| Addition | Row cost | Slot |
|---|---|---|
| D-03 provenance label (three-tier, mirroring Phase 6's D-03 exactly: "set by you" / "set at a scope gitid cannot change" / "not set") | 0 extra rows | replaces the fixture's flat `Current` string in the SAME `now: <current> → <recommended>` list-row line and the SAME detail-pane current/recommended block |
| D-02 "set, differs" word state (conflict policy: informational, no fix) | 0 extra rows | SAME row — reuses the `!`/Warning glyph+role global-ssh's own D-12 state established, new WORD only ("set, differs — your choice"), and EXCLUDED from the "N of M" apply tally per D-02 |
| D-08 dynamic version lines (`init.defaultBranch`, optionally `push.autoSetupRemote`/`diff.colorMoved`) | +1 row each, detail pane ONLY, conditional | appended below the existing contractual explanation text, the SAME slot global-ssh's own D-13 used |
| D-08 APFS ignorecase caveat | 0 extra rows (folds into the existing one-liner/explanation text) | `core.ignorecase`'s one-liner + detail explanation |
| D-07 `user.useConfigOnly` 12th row | +2 rows on the master list (one option row, same 2-line-per-row cost as every other option) | new list entry, appended after `diff.colorMoved` (last row) or per Claude's discretion on final ordering — see "New row" section below |
| D-07 cross-warning copy (useConfigOnly selected while the D9 fallback pair is partial) | +1-2 rows, conditional, detail pane or ceremony preview | Claude's discretion on exact slot — must render BEFORE the ceremony commits (a warning inside the apply ceremony's existing preview/note slot, mirroring global-ssh's D-04 shadowed-fix warning placement) |
| D-06 post-write invariant proof | 0 visual rows (this is a backend post-write check, not a rendered element) | n/a — verified programmatically, not shown on screen |

No divergence changes the 100×30 frame. Row-budget growth from 6→11(+1)
option rows is the primary open layout risk this phase carries (flagged in
UI Considerations below) — everything else is absorbed inside the existing
row-budget method (`maxLines`/clip-cue).

---

## Typography (TUI-adapted: semantic roles, not px sizes / weights)

Unchanged from Phases 3–6 / `02-STYLE-SPEC.md` §1 — Phase 7 introduces
**zero new roles**.

| Role | TUI treatment | Usage in Phase 7 |
|------|----------------|-------------------|
| Label | `Bold(true)` | option key name (`styleBold`), detail-pane heading |
| Field (value) | plain, no styling | provenance detail text, `now: <current> → <recommended>` values, D9's editable email input text |
| Selected | `styleSelected` (accent) | the currently highlighted option row's key name |
| Hint / Faint | `Faint(true)` | `now:` value line |
| Info | `Foreground(ANSI 6)` — cyan, `styleInfo` | the advisory note line (`~ Recommended, not required…`) on the detail pane |
| Warning | `Foreground(ANSI 3)` — yellow, `styleWarning` | `!` glyph for needs-action rows AND D-02 "set, differs" rows; the `[main vs master]` highlight chip; the findings banner |
| Healthy | `Foreground(ANSI 2)` — green, `styleHealthy` | `✓` glyph for already-applied rows; the result message's "✓ N of M applied" line |
| Error | `Foreground(ANSI 1)` — red, `styleError` | D9's inline `needs @` validation message ONLY (existing role, existing usage pattern — `identities.go:426-428`'s `helperLine(text, isError=true)`) |
| Preview | `Faint(true)` | the ceremony's diff/text viewport (both baseline and D9 ceremonies) |

**Non-negotiable, carried forward unchanged:** every colored state pairs
with a glyph AND a word — never color alone. D-02's "set, differs" state
MUST NOT introduce a new glyph or a new color; it is the existing `!` +
`Warning` role with new WORD text only, per the same rule `06-UI-SPEC.md`
already restated for its own D-12.

---

## Color

ANSI-16 only, unchanged. Phase 7 reuses the existing role table exactly; no
new hue, no new role.

| Role | Value | Usage in Phase 7 |
|------|-------|-------------------|
| Dominant surface | terminal default fg/bg | body text, option key names, detail-pane copy |
| Secondary | `Faint`/`Hint` (dim) | `now:` value lines |
| Accent (ANSI 4, blue) | `styleSelected` | selected option row |
| Advisory (ANSI 3, yellow, via `Theme.Warning`) | `styleWarning` | needs-action `!` glyph, D-02 "set, differs" `!` glyph, `[main vs master]` highlight chip, findings banner, D-07 cross-warning — **never red**: advisory options are never a compliance gate |
| Healthy (ANSI 2, green, via `Theme.Healthy`) | `styleHealthy` | already-applied `✓` glyph/marker, result-message success line |
| Info (ANSI 6, cyan, via `Theme.Info`) | `styleInfo` | the detail pane's advisory note line |
| Error (ANSI 1, red, via `Theme.Error`) | `styleError` | D9's `needs @` inline validation ONLY — the ONE place in this screen a compliance-style red is correct, because it gates a malformed email from being applied at all, not a config recommendation |

Accent reserved for (unchanged from prior phases, Phase 7 adds no new
element): the focused/selected list row only — **never used to color the
option-state taxonomy itself** (that is glyph + word + Warning/Healthy,
matching the project's NO_COLOR-legible contract).

**Explicit note on D-02's color (same discipline `06-UI-SPEC.md` applied to
its own D-12):** "set, differs — your choice" stays `Warning` (yellow `!`),
never a neutral/info color — the row still names a value the user is
knowingly diverging from the recommendation on. It differs from a plain
needs-action row only in WORD, never glyph or hue.

---

## Copywriting Contract

Frozen copy is governed by the `02-STYLE-SPEC.md` §6 grep-gate mechanism,
the SAME mechanism `internal/tuikit/design.go`'s `GlobalGitOptions`/
`GlobalGitDetailExplanation`/`GlobalGitAdvisoryNote`/the D9 constant block
already sit behind. Phase 7 must NOT invent new copy for anything already
frozen there — it drafts only the strings below (all NEW, none amend
existing frozen D9 strings — `07-CONTEXT.md` D-04's field-model amendment
is a BEHAVIOR change, not a copy change; the 4 existing D9 strings
(`GlobalGitEmailFallbackHelper`/`Advisory`/`CeremonyHeading`/
`DiffAnnotation`/`ResultMessage`) are already correct for the two-field
model and need no rewrite).

| Element | Copy | Status |
|---------|------|--------|
| 11 option one-liners + `GlobalGitDetailExplanation` | Existing `design.go` text, THREE rows need the D-08 correction (see Data corrections table above: `merge.conflictstyle` value rewrite, `core.ignorecase` APFS caveat sentence, `init.defaultBranch` dynamic version line) | CITED, 3 rows DRAFT (rewrite/append) |
| Advisory note | `Recommended, not required -- you can leave any option unchanged. This is advisory, never a compliance gate.` | CITED — `GlobalGitAdvisoryNote`, unchanged |
| D-03 provenance labels (three-tier, mirroring global-ssh's own D-03 wording) | `set by you (line <N> of ~/.gitconfig)` / `set at a scope gitid cannot change` / `not set (git's built-in default: <X>)` | DRAFT, per `07-CONTEXT.md` D-03 |
| D-02 "set, differs" word state | `set, differs from recommendation — your choice` | DRAFT (byte-identical wording to global-ssh's own D-12 string, same posture) |
| D-08 APFS ignorecase caveat | `git init and git clone probe the filesystem and may write a repo-local core.ignorecase = true, which overrides this global setting per-repo.` | DRAFT |
| D-08 dynamic version line (`init.defaultBranch` detail only, non-contractual) | `Your git: <X.Y>` (+ a short note only if below 2.28 — pre-2.28 git ignores this setting's ergonomic benefit but still accepts it, so likely no hard warning needed; Claude's discretion on exact wording) | DRAFT — MUST be excluded from the copy-freeze grep (dynamic, not static, per the D-13 precedent) |
| D-08 `merge.conflictstyle` version-gated value note | `zdiff3 requires git ≥ 2.35 — your git: <X.Y> (uses diff3 instead)` when below the gate, silent (no note) when at/above it | DRAFT — the ONLY row where the gate changes the WRITTEN value, so this note is higher-priority than the other two informational version lines |
| D-07 `user.useConfigOnly` row label + one-liner | `user.useConfigOnly` / `Turns a commit with no matching identity into a hard error instead of git silently guessing an author from your OS account — the fail-loud companion to includeIf setups.` | DRAFT |
| D-07 cross-warning (useConfigOnly selected + D9 fallback pair partial: name-only or email-only) | `With user.useConfigOnly enabled and only <name/email> set as a fallback, an unmatched repo's commit will hard-fail instead of guessing the missing half — that is the point, but confirm you want this.` | DRAFT — render exact wording per which half is missing |
| D-07 guessed-name warning (name-empty + email-set state, independent of useConfigOnly) | `With no fallback name set, git will guess a commit author name from your OS account when only the fallback email applies.` | DRAFT |
| Fix-selected footer action | `a` → `apply N selected` (existing, `globalgit.go:401`) | CITED, unchanged |
| Result message template | `10 of 10 baseline options applied to ~/.gitconfig. Global user.email was left alone, as always -- each identity's commits use their own includeIf fragment.` (existing, `GlobalGitResultMessage`) — count changes to `N of N` once the D-07 row (a 12th advisory row, separate from the 10-value baseline tally per D-10) lands; verify whether D-07 joins this count or stays outside it, per the D-10 tally rule below | CITED, count subject to the D-10 rule |
| D9 field/helper/advisory/ceremony strings | Existing `GlobalGitEmailFallbackHelper`/`Advisory`/`CeremonyHeading`/`DiffAnnotation`/`ResultMessage` | CITED, unchanged (D-04's amendment is behavioral only — see below) |
| D9 inline validation error | `needs @` (existing, `identities.go`'s `helperLine(text, isError=true)` idiom reused verbatim) | CITED, unchanged |

---

## D-04: D9 becomes a two-field pair (behavior note, not a copy change)

`07-CONTEXT.md` D-04 is a **USER OVERRIDE** on the checkpoint-2 D9 model:
the frozen design's single `user.email (global fallback)` row + checkbox
becomes TWO independent fields — `user.name` and `user.email` — each with
its own empty-means-unset apply semantics (an empty field removes/omits
that key; a non-empty field is written; independent per-field apply). This
is a **behavioral/structural amendment**, not new prose — the existing D9
helper/advisory/ceremony copy already generalizes to "the fallback pair"
without a rewrite (Claude's discretion whether `Fallback author for repos
no identity matches` still reads correctly for two fields or needs a
one-word pluralization; if changed, route through the copy-freeze
mechanism as an amendment, same discipline as D-04/D-07 in `06-UI-SPEC.md`).

**Visual consequence:** the D9 detail pane row-cost roughly doubles (one
`formFieldLine` becomes two, each independently focusable/editable, each
with its own inline validation slot — email validation reuses the existing
`emailValid()`/`needs @` idiom; name validation has no format constraint,
only presence). Both fields sit in the SAME always-visible detail-pane slot
`globalgit.go:356-369` already occupies — no new screen, no new row
elsewhere.

---

## New row: `user.useConfigOnly` (D-07)

A 12th advisory row, own opt-in, default UNCHECKED (recipes leave it unset
— awareness-first, per `07-CONTEXT.md` D-07). Renders with the SAME
row template every other option row uses (checkbox + tone glyph + key name
+ `now: <current> → <recommended>` line) — no new visual pattern. Its
detail-pane explanation is the D-07 one-liner above. It participates in the
SAME `space`-toggle / apply-selection mechanism as every other row, joins
the baseline ceremony's diff preview when checked (a 13th line in
`GlobalGitFullManagedBlockText`, e.g. `[user]\n    useConfigOnly = true`),
and carries the MANDATORY cross-warning copy (above) when selected while
the D9 fallback pair is partial.

**Open placement question (Claude's discretion, per D-10):** whether this
row sorts after `diff.colorMoved` (last in the pinned §4.5 order) or
immediately after the D9 fallback pair (thematically adjacent — both are
about the author-fallback story). Either is visually valid; pick one and
apply it consistently across the fixture, the copy-freeze strings, and any
PTY e2e row-index assertions.

---

## UI Considerations

State coverage for Phase 7, using the project's standard checklist.

| Category | Element(s) | Status | Resolution / Reason |
|----------|------------|--------|---------------------|
| overflow | **11(+1) option rows in the master list vs global-ssh's 6** | ⚠ unresolved | at 2 lines/row, 12 rows = 24 body lines, close to or exceeding the measured ~24-of-25-row body budget (`02-STYLE-SPEC.md`'s corrected measurement, 02-15). This phase MUST resolve, at PTY e2e time, whether the full option list fits the fixed 100×30 frame without a scroll affordance, or whether the master list needs to scroll/paginate (a NEW capability neither global-ssh nor any prior master-detail screen needed) — planner's discretion on mechanism, but this is NOT optional and must be decided before implementation, not discovered at capture time |
| zero-one-many | doctor findings banner (Git section) | ✅ covered | existing `findingsBanner()` already handles 0 (absent) / 1 (singular) / N (plural), same as Phase 6 |
| empty | zero options selected on `a` | ✅ covered | existing guard: `a` only opens the baseline ceremony when `len(m.gitApplyChosen(options)) > 0` (`globalgit.go:233`) |
| empty | D9 email field empty + `a` pressed | ✅ covered | existing guard: the email ceremony only opens when `m.chosen[...] && m.emailValid()` (`globalgit.go:228`) — an empty/invalid field cannot trigger `a` |
| partial | D9 becomes TWO fields (D-04) — name-only, email-only, both-empty, both-set are 4 distinct states | ⚠ unresolved | D-04 defines the SEMANTICS (independent per-field apply, empty=unset) but not every render detail — e.g. can the user apply just the name while leaving email untouched in one ceremony, or does `a` always apply "whatever is currently filled"? Planner's discretion, must be an explicit decision per D-07's cross-warning requiring it to be knowable which half is missing |
| partial | D-02 "set, differs" vs plain "needs action" vs "already applied" — three states on one row (mirrors global-ssh's D-12 exactly) | ⚠ unresolved | the current `overlaidGitOption` model (`globalgit.go:62-96`) is binary (`applied bool` derived from `NeedsAction`); Phase 7 must extend it to represent "user has set a conflicting value" distinctly from "unset" — same extension global-ssh's D-12 required, RENDER contract fully specified above |
| n/a state | D-11's `core.excludesfile` Tier-1 demotion | ✅ covered (backend-only, zero visual surface) | `07-CONTEXT.md` D-11 confirms this is a write-path gate (`baseline.go:424`), not a UI row — the global-gitignore surface itself is OUT OF SCOPE this phase (Phase 8), so no render decision needed here |
| loading | D-03's provenance three-probe diff (`git config --show-origin --show-scope --list` × non-repo cwd + `--file … --list`) | ⚠ unresolved | same open question `06-UI-SPEC.md` flagged for its own provenance probes — no loading state exists in the current fixture (data is static); confirm the real backend's read happens before first render (same pattern as every other tab) or reuse the existing test-running pattern if not |
| error | git probe failure (missing `git` binary, malformed repo state at probe cwd) | ⚠ unresolved | not modeled anywhere in `07-CONTEXT.md` or the fixture; likely surfaces as a doctor-style finding rather than blocking the screen (advisory-fail-open convention, same as Phase 6's equivalent gap), but must be an explicit decision |
| overflow | D-07's longer `GlobalGitFullManagedBlockText` (12 sections after the new row) inside the ceremony's fixed preview viewport | 🧪 backstop | verify at PTY e2e/visual-gate time that the ceremony's `ExactTextViewport` (same 10-line/58-col budget as global-ssh) still shows every sentinel line without truncating — if it overflows, widen the viewport's `VisibleLines`, do not silently clip a managed-block boundary |

---

## Approved Base States (`internal/tuikit/globalgit.go` — BINDING, live code)

Cited from the actually-running dummy, corrected against `FIELDS.md`'s
stale screen list per the Known Divergence callout above.

| State | Goal | Phase 7 addition |
|---|---|---|
| `options-list` (browse mode) | master-detail: 11-row (12 after D-07) option list (left) + live detail panel for the selected row (right) | + D-03 real provenance detection replaces the static fixture `Current` string; + D-02 "set, differs" state rendering; + D-08 fixture value/copy corrections; + D-07 new 12th row |
| `options-list`, D9 fallback row selected | detail pane: editable field(s) + checkbox + always-visible helper/advisory | + D-04 two-field model (name AND email, independently editable/applicable) replaces the single email field |
| Apply ceremony, state A (baseline preview + confirm) | diff of the chosen options' managed-block text, target file, backup path shown as a promise | + D-08's `zdiff3`/version corrections in the rendered text; + D-07's cross-warning when applicable |
| Apply ceremony, state B (baseline result receipt) | `Wrote →` / `Backed up →` + the existing result-message template | count updated per the D-10 tally rule (whether D-07's row and D-02's "differs" rows count toward "N of M") |
| Apply ceremony, state A/B (D9 fallback, SEPARATE ceremony instance) | own heading, own annotated diff, own result message — never the baseline ceremony | + D-04's two-field diff (both name and email lines when both are set, only the set half's line when partial) |

**Focal points (per-screen primary focus, for executor clarity):**
- `options-list` — primary focus: this is the FIRST master-detail screen in
  the project to carry more than 6-8 rows; row-budget verification (the
  overflow item above) is the single highest-risk visual unknown this
  phase carries, higher priority than any individual copy correction.
- D9 detail pane — primary focus: D-04's two-field model must read as
  "two independent, small commitments" (each field optional, each applies
  on its own), not as a single compound field that happens to hold two
  values — the empty-means-unset semantics must be visually unambiguous
  per field, not just documented in prose.
- Apply ceremony state A (baseline) — primary focus: the D-07 cross-warning
  and the D-08 version-gate note must both render HONESTLY when applicable
  — never a silent "applied" outcome that hides a hard-fail risk (useConfigOnly)
  or a version-dependent value substitution (zdiff3→diff3).

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
from its binding source and must not be re-litigated during Phase 7
planning or execution.

- **Master–detail archetype** — list ~44% width left, detail ~56% width
  right — `frame.go`'s existing helpers, do not reimplement.
- **Key-allocation:** `3` (ActivationKey, already registered on view `3`),
  `a` (apply selected, intra-surface), `space` (toggle selection),
  `↑`/`↓` (list navigation), `Enter` (start text-editing the D9 field(s)),
  `Esc`/`Enter` (exit text-editing) — all pre-existing in `globalgit.go`;
  Phase 7 claims no new key.
- **Glyph contract:** every colored state pairs with a glyph AND a word,
  never color alone — the 11(+1)-option taxonomy (now extended to a third
  state per row by D-02) is this surface's canonical example, mirroring
  global-ssh's own D-11/D-12.
- **Compressed 2-state ceremony** (preview+confirm+backup-promise → result)
  for every write, no exceptions — `ceremony.go`'s documented contract,
  instantiated TWICE on this screen (baseline + D9, never merged). Do NOT
  reintroduce FIELDS.md's separate fix-preview/confirm-write/backup-notice/
  result-applied screens.
- **Advisory, never blocking** — GGIT-01's own highest-risk affordance
  alongside managed-block containment. No recommendation may ever gate
  navigation, and D-02's "differs" state must never read as an error.
- **"The backup is the undo story"** — both ceremonies' state-A backup-path
  promise and state-B `Backed up →` line must stay first-class, never
  buried.
- **Managed-block containment is THIS surface's own highest-risk
  affordance** (distinct from global-ssh's shadowing risk): every write
  must visibly preserve content OUTSIDE the sentinel block verbatim — the
  confirm ceremony's preview showing `# BEGIN/END gitid managed: global-git`
  around the diff, never around the whole file, is the concrete render of
  this promise.
- **includeIf-precedence invariant, stated on screen at every D9 beat**
  (field, helper, ceremony heading annotation, and result message) — an
  identity's own `includeIf` fragment ALWAYS overrides the global fallback;
  D-04's field-model amendment does not relax this, it only changes how
  many fields carry the promise.
- **Copy-freeze grep gate**, extended per this document's DRAFT strings
  once approved — `02-STYLE-SPEC.md` §6 — with the D-08 dynamic version
  lines explicitly EXCLUDED (non-contractual, per the D-13 precedent
  Phase 6 established).
- **Recipe fidelity, with a DOCUMENTED divergence:** the baseline block's
  shape matches `recipes/gitconfig.recipe`'s `[include] path = ` idiom
  (D-01), but the D9 fallback pair's OWN early-positioned sentinel block is
  a deliberate structural choice `07-CONTEXT.md` D-05 verified empirically
  (a plain `git config --global user.email` lands AFTER includeIf blocks
  and hijacks every identity's author) — this is not a recipe violation,
  it is the mechanism that makes the recipe's own includeIf precedence
  actually hold.
- **100×30 capture geometry** is kept identical for the visual-regression
  gate — do not change frame dimensions to fit the longer 11(+1)-row list;
  resolve the overflow question (UI Considerations) within the existing
  frame instead.
- **NO_COLOR legibility:** every row (including the new D-02/D-07 states)
  must remain fully legible under `NO_COLOR` — glyph + word carries the
  meaning, color is additive only.

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: PASS
- [ ] Dimension 2 Visuals: PASS
- [ ] Dimension 3 Color: PASS
- [ ] Dimension 4 Typography: PASS
- [ ] Dimension 5 Spacing: PASS
- [ ] Dimension 6 Registry Safety: PASS

**Approval:** pending
