# 02-STYLE-SPEC.md — the cross-media semantic style contract

Plan 02-14 absorbs the round-2 cross-AI consensus (`02-REVIEWS.md`) as a
single follow-up polish wave. Both reviewers independently found the same
gap: the 63-item semantic parity gate (content — fields, labels, copy,
options, defaults, flow order) had **no dimension for emphasis roles, focus
affordance, or keyboard-nav ergonomics** — exactly where the user's round-2
feedback clustered. This document is that missing contract: one role table
shared by both media, one written precedence rule for the contended arrow
keys, six new checkable parity dimensions, and the frozen copy both demos
must carry byte-identically.

**Plan 02-15 (checkpoint-2 route-back) SUPERSEDES the parts of this document
02-14 authored that `02-DESIGN-DECISIONS-CHECKPOINT-2.md`'s D1–D9 contract
overturns** — the 02-12 human checkpoint requested changes: D1 kills the
3-row `renderFocusedFieldBox`/rounded-contour field treatment (§1
`focused-field`, §3 `field-contour`, §7's "+2 box" row-budget math — all
rewritten below); D4 moves the bracketed `[N] Label` format from the wizard
stepper onto the MAIN NAV and adds the `ActiveNavDimmed`/`activeNavDimmed`
role (§1, §3 `dim-states`); D5 reverts the wizard stepper to `Step n/4 ·
<label> ● ○ ○ ○` using the long labels (§5, fully rewritten — the short↔long
map this document froze in 02-14 is gone). Where a section below still
describes 02-14's superseded shape, it is marked and the D-item that
overturns it is named — this document does not re-decide any D-item, it
records where the implementation moved.

It is implemented as a central Go `Theme` (`internal/dummytui/theme.go`)
mirrored 1:1 by role name with the web `theme.ts` role tokens
(`.planning/design/mockup-src/src/theme.ts`).

---

## 1. Role table

One row per semantic role. The TUI column names the `lipgloss` treatment;
the WEB column names the MUI/theme.ts token. The mapping is 1:1 by role
NAME — `label ↔ styleBold`, `hint ↔ styleFaint`, `warning ↔ styleWarning`,
`error ↔ styleError`, `preview ↔ Faint+dashed`, `focused-field ↔ accent
COLOR ONLY, no border` (D1, checkpoint-2 contract — supersedes the rounded
border 02-14 shipped), `disabled-nav ↔ faint tabs`, `active-area ↔ accent`,
`active-nav ↔ accent background`, `active-nav-dimmed ↔ accent
foreground-only` (NEW, D4).

> **Scope note (checkpoint feedback U2, upgrading review-finding F8).**
> Both live demos are IN SYNC ROLE-BY-ROLE: the TUI centralizes every
> renderer through the Go `Theme` (frame.go's promotion made this true
> byte-for-byte), and the web routes every semantic color the live demo
> renders through the named `roles.*` tokens — or through the MUI palette
> entries `createTheme` builds from the same `semanticColors` values (Alert
> severities, TextField error states). The deliberate, documented
> exceptions, IDENTICAL in kind on both sides:
>
> 1. **The focus/selection surface.** TUI: `styleReverse`/`styleSelected`
>    (focused buttons, selected rows, sub-tab strips) are deliberately
>    role-less — they mark focus ownership, not a semantic state. WEB: the
>    matching `semanticColors.focus` usages (Global SSH sub-tab strip,
>    inline link text) are equally role-less.
> 2. **Pure layout grays.** `#2a2d33` (borders/divider), `#5a5a5a` (the
>    "no capability" pip), `#8a8a8a` (pip letter tint) carry no semantic
>    meaning — chrome, not states. The TUI equivalent is the terminal's
>    default fg/bg, which is not themed either.

| Role | TUI (`internal/dummytui/theme.go`) | WEB (`theme.ts` `roles`) |
|---|---|---|
| `info` | `Foreground(ANSI 6)` — cyan | `roles.info.color` (`#3aa6a6`) |
| `label` | `Bold(true)` | `roles.label` — `fontWeight: 700` |
| `field` | plain (no styling — the value itself) | `roles.field` — a visible 1px border |
| `focused-field` | **D1 (checkpoint-2 contract, SUPERSEDES 02-14):** `Foreground(ANSI 4) + Bold`, NO border — every field is ONE constant-height row in every state; focus = accent color + a redundant `▸` marker, never a reflowing box (`renderFocusedFieldBox` is DELETED) | **D1:** `roles.focusedField` tints VALUE + LABEL with the accent color + a 2px accent outline — no layout/height change on focus |
| `blurred-field` | `Faint(true)` — a single-row dim contour (unchanged by D1 — this role was already single-row) | `roles.blurredField` — dim border, `opacity: 0.85` |
| `hint` | `Faint(true)` | `roles.hint` — `color: semanticColors.dim` |
| `warning` | `Foreground(ANSI 3)` — yellow | `roles.warning` — `semanticColors.warning` |
| `error` | `Foreground(ANSI 1)` — red | `roles.error` — `semanticColors.error` |
| `preview` | `Faint(true)` + the dashed border (`previewDashedBorder`) | `roles.preview` — dim, `opacity: 0.9` |
| `disabled-nav` | `Faint(true)` — header tabs dim while a pane captures keys | `roles.disabledNav` — dim, `opacity: 0.6` |
| `active-area` | `Foreground(ANSI 4)` — the accent, carried on the breadcrumb/divider line directly above the active pane | `roles.activeArea` — a 1px accent border |
| `active-nav` | `Bold + Foreground(ANSI 15) + Background(ANSI 4)` — the ACTIVE header tab (no pane capturing keys) carries the accent as a BACKGROUND (checkpoint feedback U1: a flat monochrome reverse-video invert did not clearly say "I am at 1/2/3/4") | `roles.activeNav` — accent background + accent border, terminal-background text, `fontWeight: 700` |
| `active-nav-dimmed` | **NEW (D4, checkpoint-2 contract):** `Bold + Foreground(ANSI 4)`, NO background — the ACTIVE tab while a pane/form/ceremony captures keys; distinct from BOTH the full `active-nav` background (no capture) and `disabled-nav` (an INACTIVE tab while capturing) | **NEW (D4):** `roles.activeNavDimmed` — accent text/border, TRANSPARENT background, `fontWeight: 700` |

Per-medium contrast note for `active-nav`: the TUI pairs bright-white text
with the dark ANSI-4 blue; the web pairs dark (terminal-background) text
with its lighter `#5aa9e6` accent — the ROLE (accent-as-background on the
active nav item) is identical, the contrast pairing adapts to each medium's
accent luminance.

The TUI stays ANSI-16 (no truecolor, no adaptive light/dark detection) —
the more portable choice per the round-2 consensus's own "theming question"
ruling; every role also carries a glyph or word (02-UX-DIRECTION.md §2),
never color alone.

### ActiveArea mechanism (resolves round-3 defect D5)

The active-area accent is NOT a border wrapped around the whole 100×30
frame (that would cost rows the 30-row budget cannot spare — see the Row
Budget trap below) and it is not a hue change to body content. It is
carried on the **breadcrumb/divider line directly between the header and
the active pane's body** — the one chrome row every screen already
renders, at zero extra row cost:

- **TUI**: `RenderFrame`'s crumb line renders through `Theme.ActiveArea`
  (accent) instead of the default `Hint` (faint) whenever `capturesKeys` is
  true. The header's INACTIVE nav tabs render through `Theme.DisabledNav`
  (faint) at the same time; the ACTIVE tab renders `Theme.ActiveNavDimmed`
  (D4, checkpoint-2 contract — SUPERSEDES 02-14: the active tab no longer
  keeps its `active-nav` background while a pane captures keys — it dims to
  accent-foreground-only, distinct from `DisabledNav`).
- **WEB**: `Frame.tsx`'s nav tabs render through `roles.disabledNav` while a
  modal/edit/ceremony pane owns the keys; the ACTIVE tab renders
  `roles.activeNavDimmed` (D4) instead of `roles.activeNav`; the active
  pane's outline carries `roles.activeArea`.

## 2. Arrow-key precedence rule (verbatim, numbered)

Implemented **identically in both media** — this is the HIGH-severity trap
both reviewers flagged (←/→ is already claimed by the field/button ring, the
match-strategy/algorithm selects, and the Global SSH sub-tabs before this
plan adds wizard-step navigation on top).

1. **If an expanded select / option list owns focus**, ←/→ (and ↑/↓) change
   the option — unchanged from existing behavior.
2. **Else if focus is inside a text input**, ←/→ move the cursor and are
   **NEVER intercepted** — this is non-negotiable in both media.
3. **Else** (stepper, buttons, or a non-editing pane region), ←/→ navigate
   wizard sections — **forward is gated on step validity** (cannot arrow
   past an unpassed two-stage test), **back is always allowed**.
4. **Sub-tab surfaces** (Global SSH's Options/Storage sub-tabs) keep their
   existing ←/→ meaning, unaffected by this rule.
4a. **NEW (D4, checkpoint-2 contract) — top-level view switch.** Plain ←/→
   ALSO switch the MAIN NAV view (Identities/Global SSH/Global Git/Doctor)
   1–4, clamped at the ends — but ONLY when clauses 1–4 above all decline
   the key (the active screen's own handler returns unhandled). This fires
   from Identities' detail pane, Global Git, and Doctor's top-level states;
   it NEVER fires from Global SSH (clause 4 always claims the key there) or
   from any capturing pane/field/select (clauses 1–3 claim it first).
5. **`Shift`+←/→ is a FOCUS-OVERRIDE chord** — it reaches wizard
   section-navigation even when focus is inside a text input or an expanded
   select, but it is **NEVER a validity override**: forward
   (`Shift`+`Right`) stays gated on step validity (cannot skip an unpassed
   two-stage test) and back (`Shift`+`Left`) is always allowed. Shift
   overrides **focus ownership only** — never validity.

> **D7 fix (checkpoint-2 contract) — the review step was DEAD.** Both media
> previously bound this chord ad-hoc PER WIZARD STEP; step 3 (the review
> ceremony) had no `Shift+Left` case at all — the ceremony's own local key
> handler swallowed it (TUI: `ceremony.handleKey`'s `default:`; WEB:
> `useLocalKeys`'s `if (step === 3) return false` early-return). Both media
> now hoist ONE gate ABOVE every step/ceremony branch (TUI:
> `handleWizardKey`'s top; WEB: `useLocalKeys`'s callback, checked before
> the step-3 early-return) so `Shift+Left` reaches `stepBack()`
> UNIFORMLY at every step, including 3→2 back to the Git step. A raw-byte
> PTY e2e (`TestDummyDemo_ShiftChordRawBytes`, injecting the real xterm CSI
> sequences `\x1b[1;2D`/`\x1b[1;2C`) proves this survives real terminal byte
> decoding, not just a synthetic key-message unit test.

> **Note — the deliberate Shift+Arrow tradeoff.** Clause 5's chord
> overrides the browser's native `Shift`+`Arrow` text-selection gesture
> inside web text inputs (e.g. extending a text selection in the Provider
> field). This is an intentional, documented power-user override, placed
> immediately after the cursor-keys-are-sacred clause [2] specifically so
> the tradeoff is visible next to the rule it appears to relax. It relaxes
> *focus ownership* only — the validity gate in clause 3/5 is never
> bypassed by any chord.

## 3. Parity dimensions

The static `parity.json` machine file (the "63 rows") was **removed** with
the static reference set in commit `7453561`. These dimensions are enforced
by (a) the Go copy/behavior-pinning unit suite in `internal/dummytui`, and
(b) a fresh `agent-ui-ux-designer` critique of the two LIVE demos — not a
JSON file. `field-contour` and `dim-states` below are REWRITTEN by
02-15/checkpoint-2 (D1/D4); the rest are unchanged from 02-14.

| Dimension | TUI expected behavior | WEB expected behavior | Backing |
|---|---|---|---|
| `typography-emphasis-roles` | Label bold, Hint faint, Warning/Error/Info/Healthy carry their ANSI colors | `label` bold, `hint` dim, warning/error carry `semanticColors` | `theme_test.go` role-SGR tests; critique |
| `field-contour` | **D1 (SUPERSEDES 02-14's rounded-box contour):** every field is ONE constant-height row in EVERY state — focus = accent color + bracket delimiters + a redundant `▸` marker, NEVER a box; `renderFocusedFieldBox` is DELETED; net cost **−2 rows per open form** vs. 02-14 | **D1:** MUI TextField never reflows (unchanged mechanically); focus tints value+label with the accent color + a 2px outline, no new layout | `TestWizardFieldContour*` / `TestThemeFieldFocusedIsColorOnlyNoBorder` / `TestWizardFocusedFieldIsSingleRowColorOnlyNoBox`; `make test-e2e` (100×30 PTY walk); critique |
| `always-expanded-radios` | **NEW (D2):** BOTH the match-strategy and algorithm groups render ALL options ALWAYS — no expand/collapse branch; the `(←/→ change)` hint sits on the group HEADER line, visible in BOTH focus states | **D2:** both Selects replaced by MUI `RadioGroup`s; same always-rendered options + header hint | `TestGitFormStrategyAlwaysExpandedWithHeaderHint`; critique |
| `glyph-checkbox-radio` | Frozen glyphs `☑`/`☐` (checkbox), `●`/`○` (radio) — pre-existing, unchanged | **NEW (D3):** theme-level `MuiCheckbox`/`MuiRadio` `defaultProps` render the SAME frozen glyphs, replacing stock Material icons everywhere at once | `theme.ts` `MuiCheckbox`/`MuiRadio` overrides; critique |
| `hint-persistence` | A reserved hint row under the focused field never collapses to zero; the always-expanded radio group (D2) can never push it away either | Same — the strategy-select hint never disappears | `TestGitFormStrategyAlwaysExpandedWithHeaderHint`; critique |
| `arrow-nav` | The precedence rule (§2) implemented for the wizard's stepper/fields/selects, PLUS the D4 top-level plain-arrow view switch (clause 4a) | Same rule, same precedence order, PLUS the D4 top-level switch (`DemoApp.tsx`) | `TestWizardArrowPrecedence*`, `TestWizardHoistedShiftGateReachesEveryStepIncludingTheReviewCeremony` (TUI); `Identities.tsx` `useLocalKeys` + `DemoApp.tsx` Shift chord + top-level ←/→ (WEB); critique |
| `click-to-focus` | **NEW (D8):** the ENTIRE rendered field/radio/checkbox row is the click hit target (wizard steps, Edit SSH, Configure Git, Clone); disabled algorithm rows are inert | Native MUI click targets (unchanged — already row-sized) | `TestMouseWizardStep0FieldRowClickFocuses` and siblings (batch3_test.go); critique |
| `preview-sizing` | `PreviewBlock` bounded to pane width, optional fixed max height with the `… (+n more)` clip cue, title in the border top edge | `PreviewBlock`/`PreviewLabel` render through the `preview` role, sized consistently | `TestPreviewBlock*`; critique |
| `dim-states` | **D4 (SUPERSEDES 02-14):** header nav tabs render `[N] Label` (bracket format moved here from the wizard stepper); INACTIVE tabs dim (`DisabledNav`) while a pane captures keys; the ACTIVE tab now dims to the NEW `ActiveNavDimmed` (accent foreground, NO background) instead of keeping `ActiveNav`'s background; the active pane carries the `ActiveArea` accent | **D4:** nav tabs read `[{i+1}] {label}`; INACTIVE tabs dim (`disabledNav`); the ACTIVE tab renders the NEW `activeNavDimmed` while a modal/ceremony owns the keys; the active pane carries `activeArea` | `TestRenderHeaderActiveTabDimsToForegroundOnlyWhenCapturesKeys`, `TestRenderFrameActiveAreaAccent*`, `TestRenderFrameActiveTabAccentBackground`; critique |
| `chord-visibility` | **NEW (D5/D7):** a single always-visible faint line directly under the stepper carries the step-conditional Shift-chord hint; blocked-forward emits a frozen status note naming the gate | **D5/D7:** the identical conditional sub-line under `StepDots`; `notify(...)` on blocked `Shift+→` | `TestWizardChordHintIsStepConditionalAndAlwaysVisible`, `TestWizardHoistedShiftGateReachesEveryStepIncludingTheReviewCeremony`; critique |

## 4. Frozen copy — slide-3 (Git identity step) buttons and hints

The **single source of truth** for Task 2 (web) and Task 3 (TUI). Neither
task re-derives these strings — they are typed/copied verbatim.

- Skip button: `[ Skip Git ]`
- Continue button: `[ Continue ]`
- Skip hint (adjacent faint/dim hint line, always visible):
  `Skip keeps this identity SSH-only and marks it incomplete.`
- Continue hint (adjacent faint/dim hint line, always visible):
  `Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing.`

These replace the old long labels `Skip — SSH only (identity stays
incomplete)` and `Continue: review & write (Enter)` — the explanation moves
from the button label itself to the adjacent hint line. No artifact (Go,
TSX, `FIELDS.md`) may still contain the old strings after Task 3's
atomicity gate runs (§6 below).

**D6/D7 additions (checkpoint-2 contract)** — also frozen, verbatim:

- `[ Continue ]` disabled suffix: `— needs user.name + a valid email`
  (replaces the generic `— disabled`, which is now FORBIDDEN anywhere in
  either demo — §6's grep was extended to catch it).
- Blocked-forward status notes: step 0
  `Can't continue yet — check the alias prefix, hostname, and port.`; step 2
  `Can't continue yet — add user.name and a valid email.`
- Step-conditional chord hint (always visible, directly under the stepper):
  step 0 `Shift+→ next section · Shift+← exits the wizard`; steps 1–2
  `Shift+←/→ jump sections · forward needs a valid step`; step 3
  `Shift+← back to Git · Enter writes`.
- Audit-table contextual footers (chrome, zero body-row cost):
  `←→ switch view` (top-level, non-Global-SSH); `Tab/←→ Cancel / Confirm` +
  `Enter confirm` (every ceremony); `Tab/↑↓ fields` + `Enter rewrite Host
  block` (Edit SSH); `Tab/↑↓ fields` + `Enter write Git identity` (Configure
  Git); `Tab switch · Enter clone` (Clone); `space toggle` (Global SSH/Git
  option rows, renamed from the old `choose`); `↑↓ layout` (Global SSH
  Storage, renamed from `choose layout`); `Esc/? close` (Help overlay).

**D9 additions** — the promoted global-fallback `user.email` row (also
frozen, verbatim): row label `user.email (global fallback)`; helper
`Fallback author for repos no identity matches. Identities always override
this through their includeIf fragment — setting it never changes an
identity's author.`; advisory `Recipes leave this unset by default. Set it
only if you want a catch-all author for unmatched repos.`; ceremony heading
`Set global fallback user.email`; diff annotation `(global fallback —
identities override via includeIf)`; result `Global fallback user.email
set — used only where no identity matches; identity fragments still win.`

## 5. Frozen stepper format (D5, checkpoint-2 contract — REVERTED, fully rewritten)

**02-14's bracketed short-segment stepper (`[1] SSH · [2] Test · [3] Git ·
[4] Review`) is SUPERSEDED.** The 02-12 checkpoint-2 human review found the
bracket format was a misinterpretation of the original spec — it now lives
on the MAIN NAV instead (§1 `active-nav`/`active-nav-dimmed`, D4). The
wizard stepper REVERTS to:

```
Step 2/4 · Test connection ● ● ○ ○
```

- The counter (`Step n/4`) is bold; the `·` separator is faint.
- The active segment uses the LONG label (`wizardSteps`/`WIZARD_STEPS` —
  the SAME list both media already used for breadcrumbs/help text; there is
  no separate short-label list anymore — `stepShortLabels`/
  `STEP_SHORT_LABELS` are DELETED) rendered bold + the accent
  (`styleStepperActive` / `roles.activeArea`) — the line is NEVER `Faint`
  as a whole (the original review-findings F3 fix this preserves: the old
  `Step n/4` line read dimmer than body text, the opposite of a navigation
  affordance).
- The step dots render `●` (accent) for indices ≤ the current step, `○`
  (faint/dim) for the rest.
- Directly UNDER the stepper, an ALWAYS-visible faint line carries the
  step-conditional Shift-chord hint (§4's D6/D7 frozen copy above).

TUI: `renderStepper` (`identities.go`) + `wizardChordHint`. WEB: `StepDots`
(`Identities.tsx`) + its inline `wizardChordHint` helper. Both draw from the
SAME `wizardSteps`/`WIZARD_STEPS` long-label list — no derivation, no
separate short list.

## 6. Copy-freeze atomicity gate (EXTENDED, checkpoint-2 contract)

Task 3's exit gate greps the WHOLE surface the truth statement covers —
Go source, TSX source, AND the human-readable `FIELDS.md` companion (round-3
defect: both reviewers independently found the original gate too narrow).
02-15 EXTENDS the pattern to also forbid the 02-14 wizard-stepper bracket
strings and the generic `— disabled` suffix (both superseded by D5/D7) —
this is a superset of 02-14's original gate, not a replacement:

```sh
! grep -rn 'Skip — SSH only\|Continue: review & write\|\[1\] SSH\|\[2\] Test\|\[3\] Git\|\[4\] Review\| — disabled' \
    internal/dummytui .planning/design/mockup-src/src .planning/design/create-flow .planning/design/global-git
grep -rq 'user.email (global fallback)' internal/dummytui .planning/design/mockup-src/src
grep -rq 'needs user.name + a valid email' internal/dummytui .planning/design/mockup-src/src
```

Zero matches on the first (negated) grep proves the superseded copy —
02-14's own frozen strings (`Skip — SSH only`, `Continue: review & write`)
AND the bracket-stepper/`— disabled` strings THIS plan supersedes — is gone
from both demos + `FIELDS.md`. The two PRESENT-copy assertions prove the
NEW frozen strings (D9's row label, D7's disabled-suffix) made it into both
demos. Together they prove the copy is consistent across both demos,
`FIELDS.md`, and every Go/TSX test pin — including comments the button-copy
pins themselves don't enumerate.

## 7. Row-budget (checkpoint-2 contract — REWRITTEN, "+2 box" math retired)

**The "+2 box" math 02-14 introduced is GONE with the box itself (D1).**
02-14's rounded focused-field contour cost +2 rows per open form over a
single-row blurred field; D1 deletes that contour entirely — EVERY field
(focused or blurred) is now exactly one row, a flat **−2 rows per open
form** versus 02-14. This headroom absorbs the row-additions the rest of
the checkpoint-2 contract introduces on the SAME panes:

- D2 (always-expanded match-strategy radios): 1 row (collapsed) → 3 rows
  (always) = **+2 rows**, net NEUTRAL against D1's −2 on the Git step.
- D5 (the chord-hint line under the stepper): **+1 row**, absorbed by D1's
  −2 on every wizard step.
- D6 (git-step buttons collapse from two rows to one, hints stay below):
  **net 0** (unchanged row count, different arrangement).

**Measured acceptance number (empirically re-verified against the final
implementation, not the plan's original ~21-row estimate):** after the
reflow removal, the tightest pane (wizard step 2, Git identity — the step
carrying D1's field collapse AND D2's always-expanded radios AND D6's
button row AND D5's chord hint simultaneously, with the Git form fully
populated and both dual previews rendered) lands at **~24 of 25 body
rows** at the 100×30 minimum — 1 row of headroom, tighter than the plan's
original ~21-row estimate because D2's always-expanded radios add their
+2 rows on EVERY render (not only while focused, as 02-14 shipped it) and
the fragment/includeIf preview boxes plus the frozen git-step hints
together consume more of D1's −2-row savings than the original estimate
assumed. Still fits with no clipping — `make test-e2e`'s 100×30
raw-keystroke PTY walk (including the
NEW raw-byte `\x1b[1;2D`/`\x1b[1;2C` Shift-chord test,
`TestDummyDemo_ShiftChordRawBytes`) is the standing proof this holds. The
`ActiveArea` mechanism (§1) still reuses the existing breadcrumb row
instead of adding a frame-wide border, and `PreviewBlock`'s bounded/padded
sizing (§3 `preview-sizing`) still must never grow a pane beyond budget.

## Conscious divergences from recipes/

**D9 — the editable global-fallback `user.email` field.** `recipes/`
(the North Star, CLAUDE.md) leaves global `user.email` UNSET by default —
gitid's per-identity `includeIf` fragments are the ONLY source of commit
author identity, and the recipes never show a `[user]` section at the
`~/.gitconfig` top level. D9 promotes this row from an awareness-only,
never-checkable display to a first-class EDITABLE field + apply checkbox
that CAN write a global `[user] email = ...` fallback — but:

- **The recipes default is PRESERVED.** The row is unchecked and empty by
  default in both demos; setting it is explicit, deliberate opt-in — never
  a recommended/pre-chosen action (contrast every OTHER GGIT-01 baseline
  option, which defaults pre-chosen).
- **The includeIf-precedence invariant is preserved and stated on screen.**
  The row's own helper copy (`Identities always override this through
  their includeIf fragment...`), the ceremony's diff annotation (`(global
  fallback — identities override via includeIf)`), and the result message
  (`...identity fragments still win.`) all pin the SAME fact: an identity's
  own `includeIf` fragment ALWAYS wins over this fallback. The fallback can
  never be presented as overriding an identity's author (T-02-15-D9WRITE in
  the plan's threat register).
- **The demo stays 100% in-memory (DLV-05)** — no real file is written by
  either demo; `make test-e2e` re-asserts zero files created under the
  sandboxed HOME.
- **Scope: ONE field only.** gitid still uses ed25519 (not the recipes'
  RSA) and the recipes' alias/`IdentitiesOnly`/`includeIf`/`insteadOf`
  structure is otherwise unchanged — this divergence is scoped to a single,
  clearly-labeled, opt-in fallback field, not a change to the identity
  model or the write strategy.
