# Design decisions — checkpoint-2 feedback round (D1–D9 + affordance audit)

**Status:** BINDING — user-approved design contract from the 02-12 checkpoint feedback
round (2026-07-05). Produced by two `agent-ui-ux-designer` rounds against the live
code; reviewed and approved by the user. This document supersedes the parts of
`02-STYLE-SPEC.md` §3 (field-contour), §5 (stepper format), and §7 (row-budget "+2
box" math) that it contradicts — those sections and their pinning tests move in
lockstep with the implementation.

**Transversal principle (user, binding):** everything must be documented on screen —
especially if it is not obvious. Every non-obvious interaction in BOTH demos must
have a visible, always-discoverable affordance. Users never guess.

Geometry anchors: `frameBodyRows(30) = 25` body rows; detail pane = 62 cols at the
100-col minimum.

---

## D1 — Uniform single-row field, focus = color only

Kill the 3-row `renderFocusedFieldBox` entirely. Every field is ONE constant-height
row in both states. Focus = (a) accent color on value + bracket delimiters, (b) a
redundant non-color cue (`▸` marker TUI / 2px outline-width web). No reflow, ever.

TUI template (replaces `formFieldLine`):

```
▸ Alias prefix     [acme█             ]     focused: marker + accent brackets/value + cursor
  Real hostname    [ssh.github.com    ]     blurred: dim brackets/value, 2-space gutter
  Provider         github.com  (locked)     locked: dim value + faint "(locked)", no brackets
```

- Marker gutter is 2 cells in EVERY state (`▸ ` accent when focused, `  ` otherwise).
- Label: `styleBold` + `padRight(label, 16)` — bold in all states.
- Brackets `[…]` present focused AND blurred (the editable-slot affordance never
  appears/disappears). Focused wraps `input.View()` in accent; blurred wraps
  `input.Value()` in Faint.

Theme role change (`theme.go`): `FieldFocused` becomes
`Foreground(ANSI 4) + Bold`, NO border. `FieldBlurred` unchanged.
`renderFocusedFieldBox` is deleted along with its +2-row accounting.

Web: MUI outlined TextField already never reflows. Adjust `fieldSx` so rest state
uses `blurredField` (outline `#2a2d33`, input opacity 0.85) and focus tints value +
label with `roles.focusedField.color` and 2px outline. No new web role.

Row budget: **−2 rows per open form.**

## D2 — Radios: always-visible options + always-visible nav hint

Both radios (algorithm step 1, match strategy step 3-of-wizard) render ALL options,
ALWAYS — constant height, no expand/collapse. `●`/`○` vertical rows (horizontal
segmented control rejected: option copy is 40–48 chars, cannot fit 3-across in 62
cols). The `(←/→ change)` hint moves onto the group's HEADER line, visible in BOTH
focus states (today strategy shows it only when blurred — backwards).

```
▸ Match strategy  (←/→ change)
    ● gitdir (default) — applies inside ~/acme/
    ○ hasconfig — repos whose remote uses this alias
    ○ both — either condition (two includeIf blocks = OR)
  Determines which repos this Git identity applies to.
```

Selected `●` + label render through `FieldFocused` (accent) when group focused,
plain-bold when blurred. Remove the expand/collapse branch in `gitForm.view`.

Web: convert BOTH the strategy Select and the algorithm Select to visible
`RadioGroup`s (pattern already established by delete-scope and SSH storage).
Algorithm rows keep per-option availability sub-text + disabled. Keep the always-
rendered semantic helperText; append the nav hint to the group label.

Row budget: strategy 1→3 rows = +2, offset by D1's −2. Net neutral.

## D3 — Terminal-glyph checkbox and radio (web skinned to match TUI)

Frozen glyphs (both media): checkbox `☑`/`☐`; radio `●`/`○`. Promote to named Go
constants (`glyphCheckOn/Off`, `glyphRadioOn/Off`) shared by renderers and the
copy-freeze grep.

TUI: keep existing glyphs; checked/selected = `styleBold`, disabled = Faint. The
glyph is selection chrome — role-less by the STYLE-SPEC §1 scope-note exception.
Adjacent `!`/`✓` status glyphs keep `warning`/`healthy` roles.

Web: theme-level `components.MuiCheckbox.defaultProps` / `MuiRadio.defaultProps`
rendering monospace glyph spans (`☐`/`☑`, `○`/`●`), plus a disabled styleOverride →
`roles.hint.color`. This removes the stock Material icons everywhere at once.

## D4 — Main nav: brackets, plain ←/→ view switching, active-dimmed state

- `headerTabText`: `" %d %s "` → `" [%d] %s "` → renders `[1] Identities`. Web
  Frame nav mirrors: `[{i+1}] {label}`. Hit-testing follows automatically (spans
  derive from the same string).
- Four nav states with roles:

| state | when | TUI | web |
|---|---|---|---|
| active | current view, no capture | `ActiveNav` (bold, white-on-ANSI-4 bg) | `activeNav` |
| active-dimmed (NEW) | current view, pane captures keys | `ActiveNavDimmed` = bold + ANSI-4 foreground, NO bg fill | `activeNavDimmed` = accent text/border, transparent bg, 700 |
| inactive-dimmed | other view, capture | `DisabledNav` | `disabledNav` |
| inactive | other view, no capture | plain | text.secondary |

- Plain ←/→ switch views 1–4 at top level ONLY (fires when the active screen's
  handler returns unhandled; capturing panes and Global SSH's ←/→ sub-tabs are
  unaffected — precedence clause 4 preserved). TUI: `app.go` globals switch,
  clamp at 1 and 4. Web: DemoApp keydown, mirror the `'1'..'4'` block.
- Affordance: contextual footer (NOT the reserved line) on top-level non-capturing
  states. Frozen copy: `←→ switch view`. Global SSH keeps its existing
  `←→ Options / Storage` (that is where ←/→ actually go there).

## D5 — Wizard stepper restored + Shift chord surfaced

Revert the wizard stepper to `Step n/4 · <label> ● ○ ○ ○` (long labels; the
bracketed `[1] SSH` format was a misinterpretation and now lives on the MAIN NAV
per D4). Fix the original "dimmer than body" complaint: `Step 2/4` bold, `·` faint,
label = `styleStepperActive` (bold + accent), dots `●` accent for ≤ step / `○`
faint. The line is never Faint as a whole. Web `StepDots`: same revert with counter
bold + label 700/accent.

One always-visible faint line DIRECTLY UNDER the stepper carries the chord hint
(step-conditional — see D7). Row budget: +1, covered by D1.

## D6 — Git-step buttons on ONE row, hints below

```
 Back (Esc)  [ Skip Git ]  [ Continue ]
 Skip keeps this identity SSH-only and marks it incomplete.
 Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing.
```

Widths ≈ 43 cols — fits the 62-col pane. Both frozen hints ALWAYS visible below the
row (below, not above: the action row is the primary scan target; F-pattern). Focus/
enabled treatment unchanged; `[ Continue ]` disabled suffix per D7. Web: collapse
the nested Stacks into one row Stack + a hint Stack below. Row budget: net 0.

## D7 — Shift chord: root cause, one gate, condition visibility

**Root cause (verified in code):** the chord was bound ad-hoc per wizard step —
step 0 `shift+left` EXITS the wizard to detail; steps 1–2 work; **step 3 (review)
is DEAD** — the ceremony's `default:` swallows the key with no `shift+left` case
(non-destructive review ceremony returns `ceremonyNone`). That is the user's "does
not always return". Secondary: the chord is only tested with synthetic
`mustKey("shift+left")` — no raw-byte PTY coverage, so a terminal decode regression
would pass CI silently.

**Fix:** hoist ONE chord gate to the top of `handleWizardKey`, above the step
switch; delete the four scattered per-step cases:
- `shift+left`: step 0 → exit to detail (Esc parity, surfaced by the step-0 hint);
  otherwise `w.stepBack()` (uniform, includes 3→2).
- `shift+right`: validity-gated `w.stepForward(s)`; when blocked, emit the status
  note naming the gate (never a validity override).

Add a raw-byte PTY e2e injecting `\x1b[1;2D` / `\x1b[1;2C` at each step asserting
the step index moves.

**Condition visibility (frozen copy):**
- Step 0 hint: `Shift+→ next section · Shift+← exits the wizard`
- Steps 1–2 hint: `Shift+←/→ jump sections · forward needs a valid step`
- Step 3 hint: `Shift+← back to Git · Enter writes`
- `[ Continue ]` disabled suffix: `— needs user.name + a valid email` (replaces the
  generic `— disabled`)
- Blocked-forward status notes: step 0
  `Can't continue yet — check the alias prefix, hostname, and port.`; step 2
  `Can't continue yet — add user.name and a valid email.`
- Web: identical conditional sub-line under StepDots; reason text under the
  disabled Continue; `notify(...)` on blocked Shift+→.

## D8 — Click-to-focus on TUI form fields (milestone-replan requirement)

The ENTIRE rendered field row is the hit target (label + `[value]`; Fitts's law).
Hit-testing derives from the rendered label string (the established zones-derive-
from-rendered-strings pattern; never magic row numbers). Applies to: wizard steps,
Edit SSH pane, Configure Git pane, Clone pane.
- Radio option rows: click selects that option (reuse the delete-scope mechanism);
  strategy click also focuses `gitFieldStrategy`; disabled algorithm entries do not
  respond.
- Checkbox rows: click toggles (simulate-failure row keeps its needle; Global
  SSH/Git option rows toggle like the web's ListItemButton).
- On-screen affordance needed: NONE — explicitly decided. The visible field row and
  glyph IS the documentation; mouse mirrors the advertised keyboard path.

## D9 — Editable global Git user.email (conscious recipes/ divergence)

Promote from awareness-only to first-class editable + applicable field, framed as a
GLOBAL FALLBACK author. Recipes default (unset) preserved: unchecked/empty by
default; setting it is explicit opt-in.

- TUI: D1 single-row editable template + `☐/☑` apply checkbox joining the apply
  ceremony selection. Web: TextField (D1 fieldSx) + terminal-glyph Checkbox (D3)
  wired into applyChosen.
- Frozen copy — row label: `user.email (global fallback)`; helper:
  `Fallback author for repos no identity matches. Identities always override this
  through their includeIf fragment — setting it never changes an identity's author.`;
  advisory line: `Recipes leave this unset by default. Set it only if you want a
  catch-all author for unmatched repos.`
- Ceremony: heading `Set global fallback user.email`; target `~/.gitconfig` with
  backup; diff preview annotated `(global fallback — identities override via
  includeIf)`; result `Global fallback user.email set — used only where no identity
  matches; identity fragments still win.`
- Divergence documented: new FIELDS.md row (Global Git · user.email · EDITABLE,
  diverges from recipes/) + a "Conscious divergences from recipes/" note in
  02-STYLE-SPEC.md stating rationale and the preserved includeIf-precedence
  invariant.

## Affordance audit (transversal principle — additions)

Contextual-footer additions cost ZERO body rows (footer is chrome, not the 25-row
body). Deduplicated with D1–D7 copy.

| Screen / state | Interaction | Placement | Frozen copy |
|---|---|---|---|
| All top-level views | plain ←/→ switch view | contextual footer | `←→ switch view` |
| All ceremonies | Tab/←→ move Cancel↔Confirm | contextual footer | `Tab/←→ Cancel / Confirm` + `Enter confirm` |
| Edit SSH pane (has NO footer today) | field nav + submit | contextual footer | `Tab/↑↓ fields` + `Enter rewrite Host block` |
| Configure Git pane (has NO footer today) | field nav + submit | contextual footer | `Tab/↑↓ fields` + `Enter write Git identity` |
| Clone pane (has NO footer today) | switch + submit | contextual footer | `Tab switch · Enter clone` |
| Global SSH options | space toggles checkbox | contextual footer | `space toggle` |
| Global SSH storage | layout radio keys | contextual footer | `↑↓ layout` (keep `←→ Options / Storage`) |
| Global Git | space toggle | contextual footer | `space toggle` |
| Help overlay | close keys | overlay footer | `Esc/? close` |

Kept as-is (already visible): step-1 `(space toggles)`, `c` copy on failure,
delete-scope inline hint, Doctor `f`/`F`, palette placeholder, `(locked)` fields,
disabled-algorithm rationale.

Row-budget check: the tightest pane (wizard step 2) lands at ~21 of 25 body rows.
Ceremony footer (~46 cols) fits one keybar line at 100 cols; if a future ceremony
needs more, drop `Enter confirm` (the focused Confirm already renders `(Enter)`).

## Consolidated theme-role / frozen-copy delta

Roles (mirror by name in `theme.go` and `theme.ts`):
- `FieldFocused` — CHANGED: accent foreground + bold, no border (D1).
- `ActiveNavDimmed` / `activeNavDimmed` — NEW (D4).
- `FieldBlurred`, `DisabledNav`, `ActiveNav`, `ActiveArea`, `styleStepperActive` —
  unchanged, reused.

New frozen glyphs: `☑`/`☐`, `●`/`○` as named shared constants (D2/D3).

Web hide/reveal menus eliminated: algorithm Select + strategy Select → RadioGroups
(D2); Material checkbox/radio icons → terminal glyphs (D3).

Docs/tests that MUST move in lockstep: `02-STYLE-SPEC.md` §3 field-contour, §5
stepper format, §7 row-budget math, + new "Conscious divergences from recipes/"
note; FIELDS.md create-flow field-contour/stepper parity rows + global-git
user.email row; pins `TestWizardFieldContour*`, the §5 stepper assertion, e2e
mustSee stepper pins (revert `[1] SSH`-style pins to the `Step n/4` format), and
the copy-freeze grep gains the new frozen strings.
