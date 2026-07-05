---
phase: 2
round: 2 (checkpoint-feedback absorption; round 1 preserved at 02-REVIEWS-2026-07-02-planning-round.md)
reviewers: [claude, codex]
reviewed_at: 2026-07-05T15:50:00Z
plans_reviewed: [02-01-PLAN.md,02-02-PLAN.md,02-03-PLAN.md,02-04-PLAN.md,02-05-PLAN.md,02-06-PLAN.md,02-07-PLAN.md,02-08-PLAN.md,02-09-PLAN.md,02-10-PLAN.md,02-11-PLAN.md,02-12-PLAN.md,02-13-PLAN.md]
claude_model: default (claude CLI -p, separate non-interactive session)
codex_model: default (codex-cli 0.142.5)
user_feedback_included: true
verdict: MEDIUM — absorb via new 02-14 polish plan before 02-12 approval
---

# Cross-AI Plan Review — Phase 2, Round 2 (Checkpoint Feedback)

> Round 2 reviews the executed Phase 2 plans **against the user's checkpoint 02-12
> feedback** (arrow navigation, TUI form styling, hint persistence, preview bounding,
> button copy, semantic style system, dim states, theming). The user's feedback was
> embedded verbatim in the review prompt. Claude ran as a separate `claude -p`
> session at the user's explicit request (`--claude`); Codex ran via `codex exec`.
> Feed back with `/gsd-plan-phase 2 --reviews`.

## Claude Review

I've grounded the review in the actual implementation (`internal/dummytui/frame.go`, `identities.go`, the web demo's key handlers, and the bubbles v2.1.0 textinput API). Here is the structured review.

`★ Insight ─────────────────────────────────────`
- The TUI demo already has a *de-facto* design token set (`frame.go:99-111` — `styleBold/styleFaint/styleHealthy/...` on ANSI colors 1/2/3/6/8), but it was never promoted to a named, documented theme contract shared with the web's `theme.ts`. Most of the user's feedback (labels vs hints vs fields, dim semantics) is a symptom of that missing contract, not of missing capability.
- `bubbles/v2@v2.1.0` textinput natively supports focus/blur style states (`Styles{Focused, Blurred StyleState}` via `SetStyles`), but it styles *text*, not boxes — a single-line component cannot draw its own 3-row border. Borders must come from a lipgloss wrapper around `input.View()`, which is exactly what the old POC's `StyleModal` (rounded blue border) did at the modal level.
`─────────────────────────────────────────────────`

# Cross-AI Plan Review — Phase 2 Checkpoint Feedback Absorption

## 1. Summary

The 13 plans are a disciplined, well-gated design phase whose executed state (converged web + TUI demos, 63/63 parity, no-backend allowlist, zero-write PTY e2e) is genuinely strong — but the checkpoint feedback lands almost entirely in a layer the plans never modeled: a **cross-media interaction-and-typography contract** (arrow navigation semantics, field affordance/contour, label/hint/field role distinction, dim/active chrome states, a central theme). The parity gate (UX-DIRECTION §3) freezes *fields, labels, copy, options, defaults, and flow order*, and the phase passed it; it never had a dimension for *emphasis roles, focus affordance, or keyboard-nav ergonomics*, which is precisely where the user's items cluster. None of the feedback invalidates the executed architecture — every item is absorbable as one follow-up wave (a new plan, e.g. 02-14, plus a small STYLE-SPEC artifact) sitting between 02-13 and the still-open 02-12 checkpoint. The one real design tension is the input-border ask vs. the 100×30 row budget the TUI demo was carefully compacted into (four separate compaction rounds are recorded in STATE.md); a naive "border every field" implementation will not fit and will regress the e2e frame assertions.

## 2. Feedback Coverage Matrix

| # | Feedback item | Coverage | Evidence | Smallest coherent change |
|---|---|---|---|---|
| 1 | **Web:** ←/→ navigates wizard sections 1–4 | **Gap** | Only `GlobalSsh.tsx:91` handles ArrowLeft/Right (sub-tabs); the wizard has no step-level arrow handler | Amend `mockup-src/src/demo/screens/Identities.tsx` CreateWizard: ArrowLeft/Right switch pane-states when focus is not inside an input/select; forward-nav gated on step validity (can't arrow past an unpassed test) |
| 2 | **TUI:** same ←/→ across wizard sections; nav items visually distinct (bold numbers, `[1]`, color) | **Gap / partial** | `identities.go:1717` `stepDots` renders `Step n/4 · label ● ○` entirely in `styleFaint` — the current stepper is *dimmer* than body text, the opposite of a nav affordance. ←/→ inside the wizard is already claimed at `identities.go:1165/1261/1364` (field ring, strategy select, button ring) | New plan task: promote `stepDots` to a first-class stepper — `[1] SSH · [2] Test · [3] Git · [4] Review`, active segment bold+accent (reverse-video, matching the header-tab convention), completed segments ✓-marked; wire ←/→ step nav at a defined precedence (see Suggestions) |
| 3 | **TUI:** text inputs need visible contours (rounded blue borders like the 0.0.1 POC) | **Gap** | `identities.go:102` `newTextInput` is a bare input (`Prompt=""`, `DefaultDarkStyles`); `formFieldLine` (line 290) marks focus with only `▸ ` + bold label. The POC's remembered "rounded blue border" was `tui/overlay.go` `StyleModal`, never per-field | New plan task: focused field rendered inside a `lipgloss.RoundedBorder()` with blue/accent `BorderForeground`; blurred fields get a lighter contour (dim `[ value ]` brackets or underline) — **not** a border on all six fields (row budget, see Concerns). Pair with `textinput.SetStyles` focused/blurred states |
| 4 | **TUI:** 'Match strategy' hint vanishes on focus; hidden options appear; label/field/hint zones unclear | **Gap (deliberate prior decision to revisit)** | 02-13 key decision: "descriptive helpers render for the focused field only" (row-budget compaction) — the user is reporting exactly that behavior as disorienting; the strategy select expands options on focus (`identities.go:492-499`) while the hint disappears | Amend the form layout contract: helper zone becomes *stable* (reserved line under the focused field that never collapses to zero; expanding selects push content, never replace the hint); codify label=bold / field=contoured / hint=faint in the STYLE-SPEC |
| 5 | **TUI:** preview area has no clear fitted size | **Partial** | `frame.go:391-438` `previewDashedBorder` + Faint exists (round-3 fix), but the block auto-sizes to content width — no fixed bounding to the pane | Amend `PreviewBlock`: fixed `Width(paneWidth-…)` + optional fixed height with the existing `… (+n more)` clip cue; title embedded in the border top edge |
| 6 | **Both:** shrink slide-3 "Skip Git"/"Continue w/ Git" buttons; move explanations to a hint line near the buttons | **Gap** | Web + TUI both render long button labels (`Skip — SSH only (identity stays incomplete)` / `Continue: review & write (Enter)`); TUI tests pin this copy | Copy amendment in both media (buttons: `[ Skip Git ]` `[ Continue ]`; explanation as a faint hint line adjacent) + update `FIELDS.md`, the copy-pinning tests, and the parity rows. Legitimate pre-approval change since the freeze (§6.C) has not been signed |
| 7 | **Both:** consistent typographic/semantic system (info / labels / fields / warnings / hints) | **Partial** | TUI has implicit tokens (`frame.go:99-111`); web has `theme.ts`; UX-DIRECTION §2 defines a *color* semantics table but no *typography/emphasis-role* table, and no parity dimension checks it | New artifact: `02-STYLE-SPEC.md` (or a §2 extension) defining the 5 roles per medium (web: font-size/weight/color; TUI: bold/faint/contour/hue) + a new parity row `typography-emphasis-roles` per surface; implement as a central Go `Theme` struct + matching `theme.ts` tokens |
| 8 | **Both:** dim-all on modal is good; disabled main nav should also dim; active area needs accent color | **Partial** | `dimPane` dims the sidebar during forms/ceremonies (opacity-0.75 mirror), but header tabs/chrome stay full-brightness even while a capturing pane swallows the 1–4 keys (`capturesKeys` already tracks this state for the footer) | Amend `RenderFrame`: when the active screen `capturesKeys`, render header tabs through `styleFaint`; give the active pane a subtle accent (colored divider/border or accent section headers) so dim-vs-active contrast is legible |
| 9 | **Question:** is theming possible for Bubble Tea / Lip Gloss? | **Answerable — yes** | Not a plan gap; a knowledge item | Answer in the follow-up plan (see Suggestions): central palette struct + `lipgloss.LightDark`-style adaptive colors + bubbles `SetStyles`; the codebase already half-does this |

**Verdict:** 2 items partially covered by executed work, 6 genuine gaps, 1 question. All are absorbable in **one new plan (02-14, wave 6.5)** amending `mockup-src/src/demo/` + `internal/dummytui/` + a STYLE-SPEC doc, re-running the existing gates, before re-presenting 02-12. No executed plan needs to be reopened.

## 3. Strengths

- **The feedback loop is structurally supported.** 02-12 explicitly routes rejections back ("report which plan the loop must route back to"), and the checkpoint has already absorbed one full paradigm rejection (static PNGs → live demos, 02-13) without destabilizing the phase — this second, much smaller round fits the same mechanism.
- **Single-source copy discipline** (`data.go` ↔ `recipeFixtures.ts`, pinned by tests) means the button-copy change (item 6) is a contained, greppable edit with test coverage telling you every place it touches.
- **The key-routing precedence stack** (overlays → screen handler → globals, mirrored from `DemoApp.tsx`) gives arrow-key step-nav a principled place to slot in without breaking existing bindings.
- **Gates are re-runnable as-is**: no-backend allowlist, zero-write PTY e2e, `gate-no-backend-files`, and lint all apply unchanged to the follow-up work — the safety story doesn't need replanning.
- **`capturesKeys` already exists** (batch-3 footer-honesty fix) — it is exactly the signal item 8's "dim the disabled nav" needs; the state model anticipated the feedback even though the rendering didn't.

## 4. Concerns

- **HIGH — Input borders vs. the 30-row frame.** A rounded border adds 2 rows per field; 6 SSH-form fields × 3 rows ≈ 18 rows of fields alone, plus stepper, preview, helpers, and chrome — it cannot fit 100×30. The demo has been through **four documented viewport-compaction rounds** already. If 02-14 borders every field, the wizard will clip and the PTY e2e signatures will break. The design must be "focused field gets the full rounded-blue box; blurred fields get a 1-row contour" (or the frame's minimum height rises, which changes the e2e geometry and the spec).
- **HIGH — ←/→ is already a contended key in the TUI.** It currently moves the field/button ring (`identities.go:1165,1261,1364`), changes the strategy select (`(←/→ change)` is rendered UI copy), and switches Global-SSH sub-tabs. Adding wizard-step navigation on the same key without an explicit precedence rule reintroduces exactly the class of collision the 02-02 registration guard existed to prevent — but this layer (in-pane focus semantics) has *no* guard. The precedence must be written down before implementation (see Suggestions).
- **MEDIUM — No typographic parity dimension exists.** The §3 MUST-match list and all 63 parity rows check *content*; none check *emphasis roles*. The two demos can (and per the feedback, do) drift in visual semantics while "passing parity." Absorbing item 7 without adding a machine-checkable `typography-emphasis-roles` parity row per surface would leave the same blind spot open for Phases 3–9's visual-regression gate (DLV-04).
- **MEDIUM — Copy freeze vs. item 6.** Shortening button labels changes copy that FIELDS.md and Go/TS tests pin. Fine now (approval unsigned), but the change must flow through FIELDS.md + tests + both media *atomically*, or 02-12's checklist item C ("copy freeze") will be signed against inconsistent artifacts.
- **MEDIUM — Web/TUI parity re-verification is a hard requirement.** All 8 change items touch both media (or one medium's behavior the other must mirror). The 63/63 pass predates these changes; the follow-up plan must re-run the parity pass (and the ui-ux-designer critique) as an exit gate, not assume additivity.
- **LOW — Forward arrow-nav can skip gates.** ArrowRight from step 2 must stay disabled until the two-stage test passes (TEST-01/02 is the product's credibility pattern); a generic "←/→ moves sections" implementation that ignores step validity would undermine the flagship safety affordance.
- **LOW — Scope creep pressure.** The demos are design artifacts. A full theming engine (user-configurable palettes, runtime theme switching) would be over-engineering; a *central palette struct consumed by both demos* is the right size. Answer the user's theming question with the pattern, implement only the tokens.

## 5. Suggestions

**Theming in Bubble Tea / Lip Gloss (answer to the user's question): yes, and cheaply.**
- Define one central theme in Go: `type Theme struct { Accent, Healthy, Warning, Error, Info, Dim, FieldBorder lipgloss.Color; Label, Hint, Field, WarningText, InfoText lipgloss.Style }` and derive every style in `frame.go` from it — the package-level `styleBold/styleFaint/...` vars (frame.go:99-111) are already 80% of this; they just need to be grouped, named by *role*, and documented. Swapping palettes then = swapping one struct literal.
- Adaptive light/dark: Lip Gloss v2 removed automatic background detection; the v2 pattern is to request the terminal background via Bubble Tea (`tea.BackgroundColorMsg`) and pick colors with the `lipgloss.LightDark(isDark)` helper. For this demo, sticking to the ANSI 16-color palette (as now) is the *more* portable choice — the theme struct still pays off as the single semantic authority.
- Mirror the same role names in `theme.ts` (which already exists) so the STYLE-SPEC maps 1:1: `label ↔ styleBold`, `hint ↔ styleFaint`, `warning ↔ styleWarning`, etc.

**Focus/blur input styling (item 3), concretely:**
- Use bubbles v2's native states: `ti.SetStyles(s)` where `Styles.Focused.Text/Prompt` carry the accent and `Styles.Blurred` the dim variant (verified present in `bubbles/v2@v2.1.0/textinput` — `Styles`, `StyleState`, `DefaultDarkStyles`).
- For the contour: wrap only the *focused* field's `input.View()` in `lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.Accent).Width(fieldWidth)`; render blurred fields as a single row with a dim bracket contour (`[ value ]` in `theme.Dim`). This gives the "clear editable place + clear focus/unfocus distinction" for a net cost of +2 rows total, not +12.

**Arrow-key precedence (items 1–2) — write this rule into the STYLE-SPEC and implement it identically in both media:**
1. If an *expanded select / option list* owns focus (algorithm, match strategy): ←/→ (or ↑/↓) change the option — unchanged.
2. Else if focus is inside a *text input*: ←/→ move the cursor — never intercepted (this is the non-negotiable; stealing cursor keys from a text field is a worse defect than the one being fixed).
3. Else (focus on the stepper, buttons, or a non-editing pane region): ←/→ navigate wizard sections, gated on step validity going forward, always allowed going back.
4. Sub-tab surfaces (Global SSH) keep their existing ←/→ meaning; no wizard exists there, so no conflict.
   On the web, the same rule falls out of `document.activeElement` checks (skip when `INPUT`/`SELECT` focused). Optionally add `Shift+←/→` as an *unconditional* section-nav chord so power users can move steps even from inside a field.

**Structural recommendation:** author this as **one new plan `02-14` (wave 6.5, `depends_on: [02-13]`, blocking 02-12)** with three tasks — (1) STYLE-SPEC + central theme tokens in both media + new parity dimension; (2) web demo changes (arrow nav, button copy, hint zones); (3) TUI changes (stepper, input contours, stable hint zone, preview bounding, chrome dimming) + re-run all gates + a fresh ui-ux-designer parity pass — rather than amending seven executed plans. Update 02-12's §6 checklist to add the arrow-nav and style-system items so what the user asked for is what they sign.

## 6. Risk Assessment

**Overall: MEDIUM.**

The phase's architecture, gates, and feedback-routing mechanism absorb this cleanly — nothing executed needs to be undone, the changes are localized to the two demo codebases plus design docs, and the checkpoint goal (user-approved design for every surface) remains achievable in one follow-up wave. The risk is not *whether* but *how*: two items carry genuine implementation traps — the input-border ask collides with the 100×30 row budget (HIGH if implemented naively; the demo has already needed four compaction rounds), and ←/→ is a triply-claimed key whose new meaning needs an explicit, both-media precedence rule (HIGH if left implicit). Both are fully de-riskable at plan-authoring time with the rules above. The residual medium risk is process: the copy freeze, FIELDS.md contracts, copy-pinning tests, and the 63/63 parity pass must all be re-synchronized atomically, or the approval will be signed against artifacts that no longer agree with each other.

---

## Codex Review

## Summary

The phase plans are strong on process, isolation, and verification, but the checkpoint feedback exposes a real design-system gap: the plans prove parity and no-backend behavior, yet they do not define a sufficiently precise shared interaction/style contract for wizard navigation, form-field contours, hint persistence, preview sizing, and disabled/dimmed navigation states. Because most plans are already executed and 63/63 parity items currently pass, the cleanest absorption path is a new checkpoint-feedback polish wave before 02-12 approval, not ad hoc edits to old plans. Call it something like `02-14-PLAN.md`, depending on `02-13`, updating both live demos, adding new parity rows, rerunning TUI/web checks, and then returning to 02-12.

## Feedback Coverage Matrix

| Feedback item | Coverage | Existing plan | Smallest coherent change |
|---|---:|---|---|
| Web ←/→ navigate wizard sections 1-4 | Gap | 02-12 says live web demo is keyboard-driven, but no explicit arrow wizard nav | Add to follow-up plan: web wizard stepper handles ←/→ only when focus is not inside text input/select, or when stepper/nav has focus. Add e2e/manual checklist. |
| TUI ←/→ navigate wizard sections 1-4 | Gap | 02-13 wizard uses Enter/Esc and Global SSH uses ←/→ sub-tabs only | Add to follow-up plan: TUI wizard-level left/right step navigation with focus guards so text-field cursor movement is not stolen. |
| TUI nav items visually distinct without web-like borders | Partial | 02-13 header active tab reverse-video; 02-02 key allocation | Add explicit TUI stepper style: `[1] SSH`, `[2] Test`, `[3] Git`, `[4] Review`, active number bold/reverse/accent, completed check glyph, disabled dim. |
| TUI text fields need visible contours like old POC blue outlines | Gap | 02-13 mentions one field per row and dim previews, but not bordered inputs | Add central form-field style: focused blue rounded border, blurred dim border, invalid red border. Use `lipgloss` borders around `bubbles/textinput` values. |
| Hints disappear on focus; unclear label/input/help | Gap | 02-13 has helpers, but not persistent hierarchy | Add rule: labels always visible and bold; editable area always contoured; help always visible in dim text; focus can add an extra hint but cannot replace the base help. |
| Preview area lacks fitted size | Partial | 02-13 has `PreviewBlock` dimmer/dashed border | Extend `PreviewBlock` contract with fixed min/max width/height, scroll/clamp behavior, title label, and consistent border. |
| Slide 3 Skip/Continue buttons too wordy; move explanations to help area | Gap | 02-13 explicitly uses `Skip — SSH only...` and `Continue: review & write...` | Change both web/TUI copy: buttons become `Skip Git` and `Continue`; explanatory copy moves to dim help text immediately before/after actions. |
| Consistent typographic/style system for info, labels, fields, warnings, help | Partial | 02-01 terminal theme, 02-13 frame/preview helpers | Add a semantic style contract across web + TUI: `Info`, `Label`, `Field`, `Help`, `Warning`, `Error`, `Preview`, `DisabledNav`, `ActiveArea`. Make parity rows assert them. |
| Dim-all modal treatment good, but disabled main nav should also dim; active area needs color | Partial | 02-13 says sidebar dimmed during forms; overlay dimming exists | Add explicit dim-disabled-nav rule for modal/edit states and active area accent color. Verify in web and TUI screenshots/live demo. |
| Are Bubble Tea/Lip Gloss theme colors possible? | Not a plan item, but yes | 02-01/02-13 imply theme helpers | Add central `Theme` struct in `internal/dummytui`, with palette + semantic styles. Lip Gloss supports central reusable styles and adaptive colors. |

## Strengths

- The plans have a strong no-backend boundary: separate `cmd/gitid-dummy`, import allowlist, sandboxed HOME zero-write e2e.
- The recipe fidelity is well represented: Port 443, `IdentitiesOnly yes`, `includeIf hasconfig/gitdir`, `insteadOf`, `allowed_signers`, and ed25519 structure are carried into fixtures.
- The move from static screenshots to live web + live TUI demos is directionally correct for the checkpoint.
- The parity system is better than pixel comparison: semantic `parity.json` rows are the right mechanism.
- The PTY e2e against the real dummy binary is a strong guard against unreachable screens.
- The plans correctly keep the web UI as design artifact only, not product scope.

## Concerns

- **HIGH:** Current parity can pass while the user-visible design still fails. The missing feedback items are mostly style/interaction semantics, not field-presence semantics. Add parity rows for field contour, hint persistence, wizard arrow navigation, preview sizing, and modal/nav dimming.
- **HIGH:** Arrow-key navigation can conflict with text editing. In both web and TUI, do not globally intercept ←/→ when focus is inside a text input, select, text area, or editable field.
- **HIGH:** The style system is under-specified. “Terminal skin” and “dim preview” are not enough; labels/help/fields/warnings/previews need named reusable styles in both MUI and Lip Gloss.
- **MEDIUM:** TUI form polish is lagging behind the earlier POC expectation. The checkpoint explicitly compares it to previous blue-outlined forms, so this is likely approval-blocking.
- **MEDIUM:** Button-copy changes affect both live demos and parity docs. This should be treated as design freeze work, not cosmetic cleanup.
- **MEDIUM:** 02-12 should not proceed until the feedback polish has its own verification summary and updated `APPROVAL.md` checklist.
- **LOW:** “Rounded blue borders” in terminal are approximations. Lip Gloss can draw rounded borders, but terminal/font support varies; the acceptance should be “visible contour and focused blue accent,” not exact pixel parity.

## Suggestions

- Add `02-14-PLAN.md — checkpoint feedback polish`, depends on `02-13`, before `02-12`.
- Update web demo and TUI demo together, then update `parity.json` rows and `REFERENCE-INDEX.md`.
- Add a shared semantic style contract:
  - Web: MUI theme tokens/components for label, help, field, warning, preview, disabled nav, active region.
  - TUI: `internal/dummytui/theme.go` with `Palette` and `Styles`.
- Bubble Tea / Lip Gloss theming is absolutely possible:
  - Use a central Go struct, e.g. `type Theme struct { Colors Palette; Label, Help, Field, FieldFocused, FieldBlurred, Warning, Error, Preview, DisabledNav lipgloss.Style }`.
  - Use `lipgloss.AdaptiveColor{Light: "...", Dark: "..."}` or ANSI-safe colors if needed.
  - Pass the theme into render helpers, or keep a package-level `DefaultTheme` for the dummy.
- For TUI text inputs:
  - `bubbles/v2/textinput` supports focused/blurred styling patterns; wrap the rendered input in Lip Gloss borders.
  - Focused: blue border + bold label.
  - Blurred: dim border + normal label.
  - Invalid: red border + warning/help line.
- For arrow wizard navigation:
  - Web: ignore ←/→ when `event.target` is input/textarea/select/contenteditable.
  - TUI: only treat ←/→ as section nav when focus is on the wizard stepper, action row, preview pane, or non-text field; text inputs keep cursor movement.
- For slide 3 buttons:
  - Buttons: `Skip Git`, `Continue`.
  - Help text: “Skip keeps this identity SSH-only and marks it incomplete.” / “Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing.”
- For modal dimming:
  - Dim all inactive navigation and parent panes.
  - Add an accent border/header to the active modal/edit area so the contrast is obvious.

## Risk Assessment

Overall risk: **MEDIUM-HIGH** until the checkpoint feedback is absorbed. The architecture and verification story are strong, but the remaining issues are approval-critical design issues, not implementation details. A focused follow-up polish wave should reduce this to **LOW-MEDIUM**, provided it updates both demos, adds semantic parity rows, verifies arrow-key behavior without text-field conflicts, and reruns the no-backend/e2e gates before returning to 02-12.

---

## Consensus Summary

Both reviewers independently reached the same structural verdict: **nothing executed
needs reopening; absorb all feedback as ONE new plan `02-14` (wave 6.5,
`depends_on: [02-13]`) that must complete before 02-12 approval is re-presented.**

### Agreed Strengths
- The no-backend boundary (separate `cmd/gitid-dummy`, import allowlist, zero-write
  PTY e2e, `gate-no-backend-files`) is strong and applies unchanged to follow-up work.
- Semantic `parity.json` rows beat pixel comparison and are the right mechanism to
  extend for the new style dimensions.
- The checkpoint's feedback-routing design works: this is the second absorbed
  rejection round (first was static PNGs → live demos) without destabilizing the phase.
- Single-source copy discipline (`data.go` ↔ `recipeFixtures.ts`, test-pinned) makes
  the button-copy change contained and greppable.

### Agreed Concerns (highest priority)
1. **Arrow-key precedence must be written down before implementation (both: HIGH).**
   ←/→ is already claimed in the TUI (field ring, strategy select, Global-SSH
   sub-tabs) and must NEVER be stolen from a focused text input in either medium.
   Agreed rule: expanded select > text-input cursor > wizard-step navigation
   (validity-gated forward, always allowed back).
2. **The style system is under-specified (both: HIGH/MEDIUM).** A semantic
   style contract is needed — named roles (info, label, field, focused-field, hint,
   warning, error, preview, disabled-nav, active-area) defined per medium (MUI theme
   tokens ↔ central Go `Theme` struct) — plus new machine-checkable parity rows for
   emphasis roles, field contour, hint persistence, arrow nav, preview sizing, and
   dim states. Current 63/63 parity can pass while the visible design fails.
3. **Field contours: contour + blue focused accent, not literal borders everywhere.**
   Claude quantified the trap: rounded borders on all 6 fields ≈ +12 rows and cannot
   fit the 100×30 frame (4 compaction rounds already recorded). Agreed shape: focused
   field gets the full rounded blue box; blurred fields get a 1-row dim contour.
4. **Copy freeze atomicity (both: MEDIUM).** Slide-3 button copy (`[ Skip Git ]`
   `[ Continue ]` + adjacent faint hint line) must flow through FIELDS.md, the
   copy-pinning Go/TS tests, both demos, and parity docs in one atomic change.
5. **Re-run everything (both: MEDIUM).** The 63/63 parity pass, ui-ux-designer
   critique, PTY e2e, and no-backend gates predate these changes and must be re-run
   as 02-14 exit gates, not assumed additive.

### Answer to the user's theming question (both reviewers agree)
Yes — Bubble Tea/Lip Gloss theming is idiomatic and cheap: a central
`Theme` struct (palette + role-named `lipgloss.Style`s) in `internal/dummytui`;
`frame.go:99-111`'s existing `styleBold/styleFaint/...` vars are already ~80% of it
and only need grouping and role names, mirrored 1:1 with the web's `theme.ts`.
Lip Gloss v2 light/dark adaptation uses `tea.BackgroundColorMsg` +
`lipgloss.LightDark`; staying on the ANSI-16 palette (as now) remains the most
portable choice. `bubbles/v2` textinput supports `SetStyles` with
`Focused`/`Blurred` states; box contours come from a lipgloss wrapper around
`input.View()` (the POC's remembered blue rounded border was `StyleModal` at the
modal level).

### Divergent Views
- **Risk level:** Codex says MEDIUM-HIGH until absorbed (form polish likely
  approval-blocking); Claude says MEDIUM (mechanism absorbs it cleanly; risk is in
  two implementation traps, both de-riskable at plan-authoring time). Practical
  difference: none — both block 02-12 on 02-14.
- **Emphasis:** Claude grounded findings in current code (`stepDots` renders the
  stepper *fainter* than body text — the opposite of a nav affordance; `capturesKeys`
  already exists as the signal for dimming header chrome) and adds `Shift+←/→` as an
  unconditional section-nav chord; Codex adds an invalid-field red-border state and
  stresses updating `REFERENCE-INDEX.md` alongside parity rows.
