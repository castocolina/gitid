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

It is implemented as a central Go `Theme` (`internal/dummytui/theme.go`)
mirrored 1:1 by role name with the web `theme.ts` role tokens
(`.planning/design/mockup-src/src/theme.ts`).

---

## 1. Role table

One row per semantic role. The TUI column names the `lipgloss` treatment;
the WEB column names the MUI/theme.ts token. The mapping is 1:1 by role
NAME — `label ↔ styleBold`, `hint ↔ styleFaint`, `warning ↔ styleWarning`,
`error ↔ styleError`, `preview ↔ Faint+dashed`, `focused-field ↔ accent
rounded border`, `disabled-nav ↔ faint tabs`, `active-area ↔ accent`.

> **Scope note (review-findings F8).** "1:1 by role name" describes this
> TABLE — the Go `Theme` struct and the web `roles` export carry the same
> role names. It is not a claim that every screen-level consumer already
> routes through the named role on both sides: the TUI centralizes EVERY
> renderer through `Theme` (frame.go's promotion made this true byte-for-
> byte); the web currently routes the MECHANICAL roles — `hint`, `preview`,
> `disabled-nav`, `focused-field`, `active-area`, `label`, `healthy` —
> through `roles.*`, while a few scattered screen-level `warning`/`error`/
> `info` usages still reach `semanticColors`/MUI defaults directly. Color
> VALUES are shared either way (`theme.ts`'s `semanticColors` is the single
> source both `roles` and those direct usages read from).

| Role | TUI (`internal/dummytui/theme.go`) | WEB (`theme.ts` `roles`) |
|---|---|---|
| `info` | `Foreground(ANSI 6)` — cyan | `roles.info.color` (`#3aa6a6`) |
| `label` | `Bold(true)` | `roles.label` — `fontWeight: 700` |
| `field` | plain (no styling — the value itself) | `roles.field` — a visible 1px border |
| `focused-field` | `Border(RoundedBorder()).BorderForeground(ANSI 4)` — the ONE full rounded contour | `roles.focusedField` — border + outline in the accent color |
| `blurred-field` | `Faint(true)` — a single-row dim contour (never a full box) | `roles.blurredField` — dim border, `opacity: 0.85` |
| `hint` | `Faint(true)` | `roles.hint` — `color: semanticColors.dim` |
| `warning` | `Foreground(ANSI 3)` — yellow | `roles.warning` — `semanticColors.warning` |
| `error` | `Foreground(ANSI 1)` — red | `roles.error` — `semanticColors.error` |
| `preview` | `Faint(true)` + the dashed border (`previewDashedBorder`) | `roles.preview` — dim, `opacity: 0.9` |
| `disabled-nav` | `Faint(true)` — header tabs dim while a pane captures keys | `roles.disabledNav` — dim, `opacity: 0.6` |
| `active-area` | `Foreground(ANSI 4)` — the accent, carried on the breadcrumb/divider line directly above the active pane | `roles.activeArea` — a 1px accent border |

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
  (faint) at the same time; the ACTIVE tab keeps its reverse-video
  treatment throughout — only the rest of the chrome dims.
- **WEB**: `Frame.tsx`'s nav tabs render through `roles.disabledNav` while a
  modal/edit/ceremony pane owns the keys, and the active pane's outline
  carries `roles.activeArea`.

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
5. **`Shift`+←/→ is a FOCUS-OVERRIDE chord** — it reaches wizard
   section-navigation even when focus is inside a text input or an expanded
   select, but it is **NEVER a validity override**: forward
   (`Shift`+`Right`) stays gated on step validity (cannot skip an unpassed
   two-stage test) and back (`Shift`+`Left`) is always allowed. Shift
   overrides **focus ownership only** — never validity.

> **Note — the deliberate Shift+Arrow tradeoff.** Clause 5's chord
> overrides the browser's native `Shift`+`Arrow` text-selection gesture
> inside web text inputs (e.g. extending a text selection in the Provider
> field). This is an intentional, documented power-user override, placed
> immediately after the cursor-keys-are-sacred clause [2] specifically so
> the tradeoff is visible next to the rule it appears to relax. It relaxes
> *focus ownership* only — the validity gate in clause 3/5 is never
> bypassed by any chord.

## 3. Parity dimensions (six new checkable rows)

The static `parity.json` machine file (the "63 rows") was **removed** with
the static reference set in commit `7453561`. These six new dimensions are
enforced by (a) the Go copy/behavior-pinning unit suite in
`internal/dummytui`, and (b) a fresh `agent-ui-ux-designer` critique of the
two LIVE demos — not a JSON file.

| Dimension | TUI expected behavior | WEB expected behavior | Backing |
|---|---|---|---|
| `typography-emphasis-roles` | Label bold, Hint faint, Warning/Error/Info/Healthy carry their ANSI colors | `label` bold, `hint` dim, warning/error carry `semanticColors` | `theme_test.go` role-SGR tests; critique |
| `field-contour` | Exactly the FOCUSED field carries a full rounded accent box; blurred fields carry a single-row dim contour; net cost ~+2 rows, fits 100×30 | Focused field carries an accent border/outline; blurred fields carry a dim border | `TestWizardFieldContour*`; `make test-e2e` (100×30 PTY walk); critique |
| `hint-persistence` | A reserved hint row under the focused field never collapses to zero; an expanded select PUSHES rows, never replaces the hint | Same — the strategy-select hint never disappears on focus | `TestWizardHintZone*`; critique |
| `arrow-nav` | The precedence rule (§2) implemented for the wizard's stepper/fields/selects | Same rule, same precedence order | `TestWizardArrowPrecedence*` (TUI); Identities.tsx `useLocalKeys` + DemoApp.tsx Shift chord (WEB); critique |
| `preview-sizing` | `PreviewBlock` bounded to pane width, optional fixed max height with the `… (+n more)` clip cue, title in the border top edge | `PreviewBlock`/`PreviewLabel` render through the `preview` role, sized consistently | `TestPreviewBlock*`; critique |
| `dim-states` | Header nav tabs dim (`DisabledNav`) while a pane captures keys; the active tab stays reverse-video; the active pane carries the `ActiveArea` accent | Nav tabs dim (`disabledNav`) while a modal/ceremony owns the keys; the active pane carries `activeArea` | `TestRenderHeaderDimsInactiveTabs*`, `TestRenderFrameActiveAreaAccent*`; critique |

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

## 5. Frozen stepper short↔long label map

The TUI's `renderStepper` draws the SHORT segments below; `wizardSteps`
(`identities.go:519`, the long labels) remains the source for
breadcrumbs/help text only — the two are NOT the same list, and the short
segments are **not derived** from the long ones:

| Short (rendered by `renderStepper`) | Long (`wizardSteps`, breadcrumbs/help only) |
|---|---|
| `SSH` | `SSH details` |
| `Test` | `Test connection` |
| `Git` | `Git identity` |
| `Review` | `Review & write` |

Rendered stepper: `[1] SSH · [2] Test · [3] Git · [4] Review` — the active
segment is bold + `Theme.ActiveArea` accent (asserted NOT `styleFaint` —
the old `Step n/4 · label ● ○` line read dimmer than body text, the
opposite of a navigation affordance); completed segments carry a ✓ glyph.

## 6. Copy-freeze atomicity gate

Task 3's exit gate greps the WHOLE surface the truth statement covers —
Go source, TSX source, AND the human-readable `FIELDS.md` companion (round-3
defect: both reviewers independently found the original gate too narrow):

```sh
! grep -rn 'Skip — SSH only\|Continue: review & write' \
    internal/dummytui .planning/design/mockup-src/src .planning/design/create-flow
```

Zero matches proves the copy is consistent across both demos, `FIELDS.md`,
and every Go/TSX test pin — including comments the button-copy pins
themselves don't enumerate.

## 7. Row-budget trap (HIGH, quantified)

A rounded border on all six SSH fields would cost ≈+12 rows — the demo
cannot fit that inside the 100×30 frame it was already compacted into
across four documented viewport rounds (02-13 SUMMARY). Only the FOCUSED
field gets the full rounded box (+2 rows net); blurred fields get a
single-row dim contour. This is why the ActiveArea mechanism (§1) reuses
the existing breadcrumb row instead of adding a frame-wide border, and why
`PreviewBlock`'s bounded/padded sizing (§3 `preview-sizing`) still must
never grow a pane beyond its budget — `make test-e2e`'s 100×30 raw-keystroke
PTY walk is the standing proof.
