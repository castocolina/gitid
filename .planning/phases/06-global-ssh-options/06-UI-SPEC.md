---
phase: 6
slug: global-ssh-options
status: draft
shadcn_initialized: false
preset: none
created: 2026-08-26
---

# Phase 6 — UI Design Contract

> Visual and interaction contract for Phase 6 (Global SSH Options). Generated
> by gsd-ui-researcher, verified by gsd-ui-checker.

**This is a derivation, not a design exploration** — with one load-bearing
correction the executor MUST apply before touching code. Read the "Known
Divergence" callout below FIRST; it changes what "frozen" means for this
phase.

This project's stack is a Go Bubble Tea v2 terminal app (not a web app);
template sections written for CSS/web design systems are translated to their
TUI equivalent (theme roles instead of hex/CSS tokens, 100×30 fixed capture
geometry instead of responsive breakpoints) — the same translation Phases
3–5's own `0N-UI-SPEC.md` established.

---

## ⚠ Known Divergence: FIELDS.md's 6-screen model is SUPERSEDED by the live dummy

`06-CONTEXT.md`/`06-DISCUSSION-LOG.md` describe the binding design as
`.planning/design/global-ssh/FIELDS.md`'s **6 named screens**
(`options-list` → `option-detail` → `fix-preview` → `confirm-write` →
`backup-notice` → `result-applied`) reached via keys `v`/`f`/`w`/`y`/`z`. That
document was written in Phase 2 **Wave 4** (02-07, the per-surface fan-out),
**before** Phase 2's own **Wave 7/8** checkpoint-feedback passes (02-14
"first-class TUI stepper... bounded previews", 02-15 "checkpoint-2
route-back") replaced every mutating flow project-wide with ONE shared,
reusable **2-state ceremony component** (`internal/tuikit/ceremony.go`,
header comment: *"the compressed 2-state write ceremony
(`02-REDESIGN-SPEC.md` §6) reused by every mutating flow (create, edit,
delete, global apply, fixes)"*). `internal/tuikit/globalssh.go` — the file
`cmd/gitid-dummy` **actually runs today** on view `2` — implements exactly
this compressed architecture, not FIELDS.md's 5-key screen chain:

- **No standalone `option-detail` screen / no `v` key.** The right pane of
  the `options-list` master-detail view is a **live, always-visible detail
  panel** — moving the list selection (`↑`/`↓`) updates it in place. This is
  the SAME information FIELDS.md's `option-detail` screen carried (current /
  risk / recommended / full explanation / advisory note), just never hidden
  behind a second screen.
- **No separate `fix-preview` / `confirm-write` / `backup-notice` screens, no
  `f`/`w`/`y` keys.** Pressing **`a`** ("apply N selected") opens the shared
  `ceremonyModel` directly in **state A**, which renders the diff preview,
  the target file(s), and the backup path **on one screen** (this is
  `ceremony.go`'s documented state A: *"Preview + confirm — the exact
  diff/managed-block, the target files, and the timestamped backup shown as
  a PROMISE inline"*). `Enter` commits; `Esc` cancels without writing.
- **No separate `result-applied` screen with its own key (`z`).** Confirming
  moves the SAME ceremony to **state B** (the result receipt: message +
  `Wrote →` + `Backed up →`), which the user acknowledges to return to
  `options-list`.
- **Multi-select toggle is `space`, not baked into the detail screen** —
  `options-list` rows carry a `☐`/`☑` checkbox glyph (frozen glyph constants,
  `theme.go`) directly in the master list; there is no dedicated selection
  step.

**What is genuinely still binding from FIELDS.md / `06-CONTEXT.md`,
unaffected by the screen-count correction:**
- The exact **6-option GSSH-01 set** and its top-to-bottom order
  (`StrictHostKeyChecking`, `ForwardAgent`, `HashKnownHosts`,
  `IdentitiesOnly`, `AddKeysToAgent`, `UseKeychain`) — verified byte-identical
  in the live `internal/tuikit/design.go`'s `GlobalSSHOptions`.
- **Glyph + word, never color alone** (`!` amber "needs action" / `✓` green
  "already set"), the **advisory-not-blocking** posture, and the
  **3-of-4-applied / ForwardAgent-declined** concrete demonstration.
- Every `D-01`..`D-16` decision in `06-CONTEXT.md` that is about DATA,
  BEHAVIOR, or COPY (provenance detection, shadowing simulation, managed-block
  merge, platform-aware rows, advisory tone) — none of those decisions assume
  a particular screen count; they still bind exactly as written.

**Action for the planner/executor:** treat `internal/tuikit/globalssh.go` +
`ceremony.go` as the authoritative interaction model (per `CLAUDE.md`/
`AGENTS.md`: *"the approved Bubble Tea mockup (`cmd/gitid-dummy`) is the sole
UI/UX reference for Phases 3–10"* — the live binary outranks a historical
Wave-4 fan-out doc it has since evolved past, the same way `05-UI-SPEC.md`
already reused this project's established "scoped divergence" discipline for
similar drift). No design re-freeze is required — this is a **screen-count
compaction that already shipped and was already used successfully by Phases
3–5**, not a new proposal. Do not attempt to resurrect a `v`/`f`/`w`/`y`/`z`
5-screen chain; wire the real backend behind the 2-state ceremony that
already exists and already renders the same information.

---

## ⚠ Second correction: this phase's scope is BOTH GSSH-01 (Options) AND STORE-01 migration (Storage & preview)

`06-CONTEXT.md`'s `<domain>` section frames the phase as GSSH-01 only ("review
the 6 pinned dangerous-by-default options... apply user-selected fixes"). But
view `2`'s live dummy has **two sub-tabs** — `[Options]` and
`[Storage & preview]` — and `cmd/gitid/wiring.go`'s exhaustive `Persist`
switch has an explicit, pre-existing marker: `case tuikit.SetSSHStorage: //
Demo-only: Phase 6 owns STORE-01 migration.` (line 718-720, alongside the
matching `ApplySSH` marker at line 715-717: `// Demo-only: Phase 6 will make
the global-ssh ceremony real.`). Both actions currently fall through to the
in-memory `tuikit.Reduce` (dummy behavior) in the REAL binary — both are
explicitly this phase's job per the code's own comments, even though
`06-CONTEXT.md` only discusses the Options sub-tab in depth.

**Action for the planner:** confirm this scope reading with the user/
orchestrator before planning (the `<domain>` "Explicitly NOT in this phase"
list does not mention Storage & preview either way — it is silent, not
excluding). If confirmed, the Storage & preview sub-tab's migration ceremony
(`m.storageCeremonyFor()`, `internal/tuikit/globalssh.go:241-260`) needs the
SAME real-backend treatment as the Options sub-tab: a real `Persist` case
that calls `internal/sshconfig`'s existing STORE-02/STORE-03 migration
functions (already built, Phase 1) instead of `tuikit.Reduce`. This UI-SPEC
covers BOTH sub-tabs' visual contract below so planning is not blocked on
that scope confirmation.

---

## Design System

Same as Phases 3–5 — no shadcn / npm component registry. The "design system"
is the central Go `Theme` (`internal/tuikit/theme.go`), used unmodified.
Phase 6 introduces **zero new theme roles and zero new glyphs**.

| Property | Value |
|----------|-------|
| Tool | none (no shadcn — Go TUI, not React/Next/Vite) |
| Preset | not applicable |
| Component library | central Go `Theme` struct (`internal/tuikit/theme.go`) — 12 semantic roles + `Accent`/`FieldBorder`, ANSI-16 palette, used unmodified; the shared `ceremonyModel` (`internal/tuikit/ceremony.go`) and master-detail helpers (`joinMasterDetail`, `masterListWidth`, `frameBodyRows` — `internal/tuikit/frame.go`) |
| Icon library | none — frozen glyph constants only: `glyphCheckOn/Off` (`☑`/`☐`, multi-select rows), `glyphRadioOn/Off` (`●`/`○`, Storage sub-tab layout choice), status glyphs `✓` healthy/already-set, `!` warning/needs-action. Phase 6 introduces **zero new glyphs** — D-12's new "set, differs" state (below) MUST reuse `!` (never a bespoke third glyph), distinguished by its WORD only |
| Font | terminal's own monospace |
| Capture geometry | fixed **100 cols × 30 rows** (`frameBodyRows(30) = 25` body rows), unchanged — kept identical for the visual-regression gate |

---

## Spacing / Layout Scale (TUI-adapted: rows and columns, not px)

Global SSH is the project's **master–detail** body archetype (same as
Identity Manager), PLUS a **sub-tab strip** (`[Options] [Storage & preview]`,
one line) above it — the one architectural element unique to this surface
among the master-detail screens shipped so far.

| Token | Value | Usage | Source |
|-------|-------|-------|--------|
| Frame | 100 cols × 30 rows, fixed | capture + PTY e2e geometry | `02-STYLE-SPEC.md` §7 |
| Body rows | 25 | rows available below header/breadcrumb chrome | checkpoint-2 geometry anchor |
| Sub-tab strip | 1 row, `" [Options] [Storage & preview] "`-shaped, reverse-video on the active tab | `subTabStrip()`, `globalssh.go:356-365` | existing, unchanged |
| Findings banner (conditional) | +1 row, only when the doctor has SSH findings beyond this screen's own 6 | `findingsBanner(s, "SSH", gssBannerBeyond)`, `globalssh.go:466-482` | existing, unchanged |
| Body archetype | master–detail: list ~44% width (left), detail ~56% width minus a 2-col gutter (right) | `masterListWidth`/`masterDetailGutter`, `frame.go:43-53` | existing, unchanged |
| Option row height | exactly **2 lines** per option on `options-list` (`optionRowLines = 2`): line 1 = marker + checkbox/tick + glyph + key name + `[Risk]` chip; line 2 = indented `now: <current> → <recommended>` | `optionRow()`, `globalssh.go:485-514` | existing, unchanged — do not grow to 3+ lines; a per-row one-liner does NOT fit inline (that content lives in the always-visible right-pane detail instead, per the Known Divergence above) |
| Detail pane | right pane, bounded to pane width, wrapped + a VISIBLE `…` clip cue (`fitPane`) if content overflows the row budget | `renderOptions()`, `globalssh.go:576-607` | existing, unchanged |
| Ceremony preview | `ExactTextViewport{VisibleLines: 10, Width: 58}` inside the ceremony's state A | `ceremony.go:145` | existing, unchanged |
| Storage sub-tab left pane | STORE-01 label + 2 radio rows + a 2-line explainer + a conditional `Migrate layout… (Enter)` action row | `renderStorage()`, `globalssh.go:610-635` | existing, unchanged |
| Storage sub-tab right pane | ONE resulting-config preview (sentinel layout) OR TWO stacked previews (Include layout: main-file fragment + owned-file fragment) | `renderStorage()`, `globalssh.go:636-646` | existing, unchanged |

**Phase 6 additions — every one budgeted against an EXISTING slot, none
silently added:**

| Addition | Row cost | Slot |
|---|---|---|
| D-03 provenance label (three-tier: "set by you at line N" / "set in /etc/ssh/ssh_config" / "not set (default: X)") | 0 extra rows | replaces the fixture's flat `Current` string in the SAME `now: <current> → <recommended>` list-row line and the SAME detail-pane current/risk/recommended block — the label is longer text in an existing slot, not a new row |
| D-12 "set, differs from recommendation" word state | 0 extra rows | SAME row, SAME `!` glyph — only the WORD changes (`differs` vs `recommended`) |
| D-11 UseKeychain not-applicable-on-Linux row state | 0 extra rows | SAME row — current-value text becomes "not applicable (macOS-only setting)" instead of "yes (macOS only)" |
| D-13 dynamic "your OpenSSH: X.Y" version line | +1 row, StrictHostKeyChecking's detail pane ONLY, conditional (only when relevant to gating `accept-new`) | appended below the existing contractual explanation text in the right-pane detail block — the SAME slot `option-detail`'s explanation already occupies, one more line |
| D-04 shadowed-fix warning ("this fix will be shadowed and do nothing") | +1 row, conditional (only when the pre-write simulation detects shadowing) | inside the ceremony's state-A preview block, appended to the existing diff text — reuses the ceremony's own `Preview`/`PreviewDiff` slot, no new UI element |
| D-04 post-write "applied but shadowed" advisory finding | +1 row, conditional | inside the ceremony's state-B result message — reuses the EXISTING `ResultMessage` slot (`ceremony.go`'s `ResultMessage string` field), text grows, no new row |

No divergence changes the 100×30 frame. Every addition is absorbed inside the
existing row budget, per the established `maxLines`/clip-cue method.

---

## Typography (TUI-adapted: semantic roles, not px sizes / weights)

Unchanged from Phases 3–5 / `02-STYLE-SPEC.md` §1 — Phase 6 introduces
**zero new roles**.

| Role | TUI treatment | Usage in Phase 6 |
|------|----------------|-------------------|
| Label | `Bold(true)` | option key name (`styleBold`), detail-pane heading, `STORE-01 —` explainer heading |
| Field (value) | plain, no styling | provenance detail text, `now: <current> → <recommended>` values |
| Selected | `styleSelected` (accent) | the currently highlighted option row's key name |
| Hint / Faint | `Faint(true)` | `now:` value line, `[Risk]` chip, Storage explainer paragraph, D-13 dynamic version line |
| Info | `Foreground(ANSI 6)` — cyan | the advisory note line (`~ Recommended, not required…`) on the detail pane |
| Warning | `Foreground(ANSI 3)` — yellow | `!` glyph for needs-action AND D-12 "set, differs" rows; the findings banner; D-04's shadowed-fix warning |
| Healthy | `Foreground(ANSI 2)` — green | `✓` glyph for already-set rows; the applied-checkmark after a successful ceremony |
| Preview | `Faint(true)` | the ceremony's diff/text viewport, both Storage-sub-tab resulting-config previews |
| Reverse (tab strip) | `styleReverse` | the active sub-tab label |

**Non-negotiable, carried forward unchanged:** every colored state pairs
with a glyph AND a word — never color alone. D-12's new "set, differs" state
MUST NOT introduce a new glyph or a new color; it is the existing `!` +
`Warning` role with new WORD text only, per `02-UX-DIRECTION.md` §2's rule
that Phase 5's own `05-UI-SPEC.md` already restated for its own additions.

---

## Color

ANSI-16 only, unchanged. Phase 6 reuses the existing role table exactly; no
new hue, no new role.

| Role | Value | Usage in Phase 6 |
|------|-------|-------------------|
| Dominant surface | terminal default fg/bg | body text, option key names, detail-pane copy |
| Secondary | `Faint`/`Hint` (dim) | `now:` value lines, Storage explainer, `[Risk]` chip |
| Accent (ANSI 4, blue) | `styleSelected`, `styleFocusLink`, active sub-tab reverse-video | selected option row, the `Open Doctor (4)` findings-banner link |
| Advisory (ANSI 3, yellow, via `Theme.Warning`) | `styleWarning` | needs-action `!` glyph, D-12 "set, differs" `!` glyph, findings banner, D-04 shadowed-fix warning — **never red**: advisory options are never a compliance gate |
| Healthy (ANSI 2, green, via `Theme.Healthy`) | `styleHealthy` | already-set `✓` glyph, applied-option `✓` marker after the ceremony commits |
| Info (ANSI 6, cyan, via `Theme.Info`) | `styleInfo` | the detail pane's advisory note line |

Accent reserved for (unchanged from prior phases, Phase 6 adds no new
element): the focused/selected list row, the active sub-tab, cross-surface
links (`Open Doctor (4)`) — **never used to color the option-state taxonomy
itself** (that is glyph + word + Warning/Healthy, matching the project's
NO_COLOR-legible contract).

**Explicit note on D-12's color (carried from Phase 5's own D-12 precedent,
same discipline applied to a different letter-numbered decision):** "set,
differs from recommendation" stays `Warning` (yellow `!`), not a neutral/info
color, because GSSH-01 is explicitly **danger-aware** — the row still names a
real risk the user is knowingly carrying. It differs from a plain
needs-action row only in WORD ("differs from recommendation" vs
"recommended"), never in glyph or hue, so a user scanning by color alone
still sees "this needs my attention" for both states — while the word
clarifies that one is a deliberate, already-made choice and the other is
still open.

---

## Copywriting Contract

Frozen copy is governed by the `02-STYLE-SPEC.md` §6 grep-gate mechanism, the
SAME mechanism `internal/tuikit/design.go`'s `GlobalSSHOptions`/
`GlobalSSHDetailExplanation`/`GlobalSSHAdvisoryNote` constants already sit
behind. Phase 6 must NOT invent new copy for anything already frozen there.
It drafts only the strings below.

**Required data correction before any new copy is drafted** — the live
`GlobalSSHOptions` fixture (`internal/tuikit/design.go:212-219`) predates
`06-CONTEXT.md`'s D-10 pinned table (approved 2026-07-07, via a dedicated
research round, AFTER the fixture was authored in Phase 2 Wave 4) and
disagrees with it on two rows:

| Option | Fixture today (`design.go`) | D-10 pinned (BINDING) | Action |
|---|---|---|---|
| `StrictHostKeyChecking` | `Recommended: "ask"`, `Risk: "Medium"` | `Recommended: "accept-new"`, `Risk: "Medium"` | Update the fixture's `Recommended` value AND the one-liner to name `accept-new` + the OpenSSH ≥7.6 requirement (D-13) |
| `ForwardAgent` | `Risk: "Medium"` | `Risk: "High"` | Update the fixture's `Risk` value |

All other 4 rows (`HashKnownHosts`, `IdentitiesOnly`, `AddKeysToAgent`,
`UseKeychain`) already match D-10 byte-for-byte (value AND risk level) —
verified directly against `design.go:212-219`. Update `design.go` (the dummy
fixture) to match D-10 exactly so the dummy and the real backend stay
byte-identical, per this project's established parity discipline (never let
"which one is right" become a live-vs-dummy divergence when the correct
value is simply a data fix) — then re-run the visual-regression gate.

| Element | Copy | Status |
|---------|------|--------|
| Six option one-liners + `GlobalSSHDetailExplanation` | Existing `design.go` text (StrictHostKeyChecking's one-liner needs a `accept-new`-naming rewrite per the table above; the other five stand) | CITED, one row DRAFT (rewrite) |
| Advisory note | `Recommended, not required -- you can leave any option unchanged. This is advisory, never a compliance gate.` | CITED — `GlobalSSHAdvisoryNote`, unchanged |
| D-03 provenance labels (three-tier) | `set by you at ~/.ssh/config line <N>` / `set in /etc/ssh/ssh_config — gitid cannot change this` / `not set (OpenSSH default: <X>)`; hedge as `set outside your config` when sources disagree | DRAFT, per `06-CONTEXT.md` D-03 |
| D-12 "set, differs" word state | `set, differs from recommendation — your choice` | DRAFT |
| D-11 UseKeychain not-applicable (Linux) | `not applicable (macOS-only setting)` | DRAFT |
| D-04 shadowed-fix warning (ceremony preview) | `This fix will be shadowed by "<line>" and will not take effect until that line is resolved.` | DRAFT |
| D-04 post-write "applied but shadowed" advisory | `Applied, but currently shadowed by "<line>" — re-check after resolving it.` | DRAFT |
| D-13 dynamic version line (StrictHostKeyChecking detail only, non-contractual) | `Your OpenSSH: <X.Y>` (+ `— accept-new is available` / `— accept-new needs OpenSSH 7.6+, upgrade to use it` depending on the probe result) | DRAFT — MUST be excluded from the copy-freeze grep (dynamic, not static, per D-13) |
| Fix-selected footer action | `a` → `apply N selected` (existing, `globalssh.go:558`) | CITED, unchanged |
| Result message template | `%d of %d recommended options applied to Host *.%s` (existing, `applyCeremonyFor`, `globalssh.go:234`) | CITED — the `%s` tail already renders `" The rest were left unchanged, as chosen."` when applicable; unchanged |
| Storage sub-tab explainer | `Include paths must be absolute or ~/.ssh-relative; the Include line goes NEAR THE TOP of ~/.ssh/config. Migration between layouts is backed-up and reversible (STORE-03).` | CITED, unchanged (`globalssh.go:631`) |

---

## UI Considerations

State coverage for Phase 6, using the project's standard checklist.

| Category | Element(s) | Status | Resolution / Reason |
|----------|------------|--------|---------------------|
| empty | zero options selected on `a` | ✅ covered | `a` is already guarded — `applyChosen(options)` must be non-empty (`globalssh.go:558`), existing behavior |
| zero-one-many | D-04 shadowing simulation hits | 🧪 backstop | 0 hits → no warning line renders (block absent, same convention as Phase 5's D-13); 1 hit → one warning line naming the shadowing directive; verify at PTY e2e time against a fixture with a planted shadowing line, not assumed from spec alone |
| zero-one-many | doctor findings banner (SSH section) | ✅ covered | existing `findingsBanner()` already handles 0 (absent) / 1 (singular "finding") / N (plural "findings") |
| overflow | detail-pane explanation length | ✅ covered | existing `fitPane` wrap + VISIBLE `…` clip cue (no silent truncation, per H3 in `globalssh.go`'s own comments) |
| overflow | Storage sub-tab's Include-layout two stacked previews | ✅ covered | existing `previewBlockClipped(..., rightWidth, 4)` / `(..., rightWidth, 10)` fixed `maxLines` caps |
| partial | D-12 "differs" vs plain "needs action" vs "already set" — three states on one row | ⚠ unresolved | the current `appliedOption`/`overlaidOptions` model (`globalssh.go:108-131`) is binary (`NeedsAction bool`); Phase 6 must extend it to a three-state model (unset/needs-action, explicitly-set-and-matches, explicitly-set-and-differs) without adding a new UI element — planner's discretion on the exact Go type, the RENDER contract (glyph/word/color) is fully specified above |
| loading | provenance three-probe diff (`ssh -G` × 2 + file parse) | ⚠ unresolved | no loading state exists in the current fixture (data is static); the real backend's `InitialState()` read happens before the pane ever renders (same pattern as every other tab), so this is likely a non-issue, but the planner must confirm the three-probe read is fast enough not to need a spinner — if not, reuse the project's EXISTING `test-running` pattern rather than inventing one |
| error | `ssh -G` probe failure (e.g. no `ssh` binary, malformed temp config) | ⚠ unresolved | not modeled anywhere in `06-CONTEXT.md` or the fixture; planner's discretion — likely surfaces as a doctor-style finding rather than blocking the screen (advisory posture extends to the detection layer too, per the project's general fail-open-advisory convention), but this must be an explicit decision, not a silent gap |
| n/a state | UseKeychain on Linux (D-11) | ⚠ unresolved (spec exists, render extension needed) | `06-CONTEXT.md` D-11 is clear on the COPY; the render-state model needs the same three/four-state extension as D-12's row (recommended-but-unset / already-set / differs / not-applicable) |

---

## Approved Base States (`internal/tuikit/globalssh.go` — BINDING, live code)

Cited from the actually-running dummy, corrected against `FIELDS.md`'s stale
screen list per the Known Divergence callout above.

| State | Goal | Phase 6 addition |
|---|---|---|
| `options-list` / Options sub-tab (browse mode) | master-detail: 6-row option list (left) + live detail panel for the selected row (right) | + D-01/D-03 real provenance detection replaces the static fixture `Current` string; + D-12/D-11 three/four-state row rendering; zero new rows/screens |
| `options-list` / Storage & preview sub-tab (browse mode) | STORE-01 layout radio choice + resulting-config preview | scope-confirmation pending (see Known Divergence #2) — if confirmed in scope, real `Persist` wiring for `SetSSHStorage`, zero visual change |
| Apply ceremony, state A (preview + confirm) | diff of the chosen options' `Host *` lines, target file, backup path shown as a promise | + D-04 pre-write shadowing simulation, rendered as an appended warning line when triggered |
| Apply ceremony, state B (result receipt) | `Wrote →` / `Backed up →` + the existing result-message template | + D-04 post-write "applied but shadowed" advisory, appended to the existing `ResultMessage` |
| Storage migration ceremony, state A/B | same 2-state shape, STORE-03 migration diff | scope-confirmation pending; zero visual change if in scope |

**Focal points (per-screen primary focus, for executor clarity):**
- `options-list` (Options sub-tab) — primary focus: the always-visible
  detail pane must carry D-03's real provenance line (not just the
  current/recommended/risk it already shows) — GSSH-01 requires the option
  to be **explained**, and provenance ("why does this show as unset") is part
  of that explanation, not an optional extra.
- Apply ceremony state A — primary focus: D-04's shadowing simulation must
  render HONESTLY when a fix would do nothing — never a silent "applied"
  outcome that isn't true.
- D-12's row state — primary focus: a deliberately-set non-recommended value
  must read as **respected, not nagged** (word says "your choice"), while
  still visually flagged as risk (same `!` + `Warning` as an unset option) —
  both things true at once, per `06-CONTEXT.md`'s D-14 stateless-advisory
  design.

---

## Scoped Additions (D-01 through D-16, mapped to visual surfaces)

Every one reuses an EXISTING theme role, an EXISTING glyph, and an EXISTING
row slot (see the Spacing table above) — none adds a new color, a new glyph,
or a new screen. Each is a documented, allowlisted departure from
`FIELDS.md`'s stale screen count under the project's visual-regression gate,
exactly like the Known Divergence callout above and Phase 4/5's own D-xx
precedents.

### Provenance & shadowing (D-01 through D-05)
Lands entirely on the existing Options sub-tab: D-01/D-03 replace the static
`Current` string with a real three-tier provenance read (list-row `now:`
line + detail-pane block, same slots); D-02 is a copy-only hedge for
`UseKeychain`'s provenance line (no `ssh -G` signal exists for it, ever); D-04
adds the shadowed-fix warning to the ceremony's EXISTING preview/result slots
(Spacing table); D-05 is a backend-only detail (the probe host used), no
visual surface.

### Managed-block model (D-06 through D-09)
Zero visual surface — these are `internal/sshconfig` write-path decisions
(single `Host *` block, key-union merge, sentinel rename, ordering). The
ceremony's diff preview (existing slot) reflects whatever the merged block
ends up containing; no new UI element renders the merge logic itself.

### Platform set + recommended values (D-10 through D-13)
D-10 is the data-correction table above (Copywriting Contract). D-11 extends
the UseKeychain row to a not-applicable state (Spacing/Color tables). D-12
adds the "set, differs" word state to every row (Spacing/Typography/Color
tables). D-13 adds the dynamic OpenSSH-version line to StrictHostKeyChecking's
detail pane only (Spacing table) — MUST be excluded from the copy-freeze grep.

### Advisory posture (D-14 through D-16)
Zero visual surface beyond what already exists — D-14 (stateless, no decline
record) and D-15 (empty-start, opt-in selection via `space`) are BOTH already
the live dummy's actual behavior (`newGlobalSSHModel()`'s pre-chosen set,
`space` toggling `m.chosen`) — no change needed. D-16 (one combined
ceremony) is likewise already the live dummy's actual behavior (the 2-state
`ceremonyModel`, not a chain of screens) — this is the SAME correction the
Known Divergence callout already makes: D-16's "linear f→w→y→z chain"
framing in `06-CONTEXT.md` describes FIELDS.md's stale model, but its
underlying REQUIREMENT ("one combined ceremony... one backup notice, one
write, one result") is already satisfied by the current architecture without
any change.

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
from its binding source and must not be re-litigated during Phase 6 planning
or execution.

- **Master–detail archetype** — list ~44% width left, detail ~56% width
  right, PLUS the sub-tab strip unique to this surface — `frame.go`'s
  existing helpers, do not reimplement.
- **Key-allocation:** `2` (ActivationKey, already registered on view `2`),
  `a` (apply selected, intra-surface), `space` (toggle selection),
  `←`/`→` (Options ↔ Storage sub-tab), `↑`/`↓` (list navigation /
  Storage layout radio) — all pre-existing in `globalssh.go`; Phase 6 claims
  no new key.
- **Glyph contract:** every colored state pairs with a glyph AND a word,
  never color alone — the 6-option taxonomy (now extended to 3-4 states per
  row by D-11/D-12) is this surface's canonical example.
- **Compressed 2-state ceremony** (preview+confirm+backup-promise → result)
  for every write, no exceptions — `ceremony.go`'s documented contract. Do
  NOT reintroduce FIELDS.md's separate confirm-write/backup-notice/
  result-applied screens.
- **Advisory, never blocking** — GSSH-01's own highest-risk affordance. No
  recommendation may ever gate navigation, and D-12's "differs" state must
  never read as an error.
- **"The backup is the undo story"** — the ceremony's state-A backup-path
  promise and state-B `Backed up →` line must both stay first-class, never
  buried, for every write this phase adds.
- **Copy-freeze grep gate**, extended per this document's DRAFT strings once
  approved — `02-STYLE-SPEC.md` §6 — with D-13's dynamic version line
  explicitly EXCLUDED (non-contractual, per D-13 itself).
- **Recipe fidelity:** the `Host *` block this phase's ceremony writes must
  match `recipes/ssh-config.recipe`'s shape (`IgnoreUnknown UseKeychain` as
  the literal first directive, `UseKeychain`/`AddKeysToAgent` inside `Host *`)
  — `recipes/README.md`; D-11's lexical-ordering requirement
  (`IgnoreUnknown UseKeychain` before `UseKeychain`) is a hard constraint on
  the merge/render code, not just documentation.
- **100×30 capture geometry** is kept identical for the visual-regression
  gate — do not change frame dimensions to fit new copy; tighten
  `maxLines`/clip-cue instead.
- **NO_COLOR legibility:** every row (including the new D-11/D-12 states)
  must remain fully legible under `NO_COLOR` — glyph + word carries the
  meaning, color is additive only.
- **DemoBanner still true for `TabGlobalSSH` today** (`wiring.go:630-632`,
  `return tab != tuikit.TabIdentities`) — Phase 6's real-backend wiring must
  flip this OFF for `TabGlobalSSH` specifically (mirroring how earlier
  phases flipped it off for their own tab) once `ApplySSH`/`SetSSHStorage`
  are real.

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: PASS
- [ ] Dimension 2 Visuals: PASS
- [ ] Dimension 3 Color: PASS
- [ ] Dimension 4 Typography: PASS
- [ ] Dimension 5 Spacing: PASS
- [ ] Dimension 6 Registry Safety: PASS

**Approval:** pending
