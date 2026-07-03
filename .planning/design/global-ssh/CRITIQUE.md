# CRITIQUE.md — global-ssh (Phase 6 fan-out surface)

**Reviewer note:** the executor's toolset in this session was limited to
`Read`/`Write`/`Edit`/`Bash` — no `Task`/subagent-dispatch tool was available
to spawn a fresh-context `agent-ui-ux-designer` subagent (same limitation
recorded in `create-flow/CRITIQUE.md` (02-04), `git-screen/CRITIQUE.md`
(02-05), and `identity-manager/CRITIQUE.md` (02-06)). In its place, this
critique applies `agent-ui-ux-designer`'s documented methodology
(F-pattern/left-side bias, Fitts's/Hick's Law, accessibility,
distinctive-not-generic typography) directly against the captured
`.planning/design/global-ssh/html/*.png` and `.planning/design/global-ssh/tui/*.png`
screenshots (viewed and compared side by side, all 12 images), and fills the
structured parity findings log against `FIELDS.md` and `parity.json`. **This
does not substitute for a fresh-context `agent-ui-ux-designer` pass** —
flagging explicitly so the phase-level `/gsd-code-review` and the external
cross-vendor review can re-run one if the orchestrator has that capability
this session lacked.

---

## Surface: `global-ssh`

## A. Aesthetic / usability pass (HTML mockup only)

- **Reviewer:** this executor, applying `agent-ui-ux-designer`'s documented
  methodology directly (Task/subagent dispatch unavailable — see note above)
- **Screenshots reviewed:** all 6 `.planning/design/global-ssh/html/*.png`
- **Findings:**
  - **F-pattern / left-side bias:** `options-list` uses the master-detail
    archetype (§2) — the 6-option list flush-left, the highlighted option's
    preview to the right; the 5 single-column ceremony screens
    (`option-detail`, `fix-preview`, `confirm-write`, `backup-notice`,
    `result-applied`) read top-to-bottom, left-aligned. No finding.
  - **Fitts's Law (target size/reachability):** each option row on
    `options-list` is a full-width `ListItemButton` with generous padding —
    no undersized targets. The highlighted `IdentitiesOnly` row carries a
    subtle background highlight consistent with `identity-manager`'s
    `list-populated` selection precedent. No finding.
  - **Hick's Law (choice count):** `options-list` presents exactly 6 options
    grouped visually by state (4 yellow `!` "recommended" rows, then 2 green
    `✓` "already set" rows) — the severity-first, already-fine-last ordering
    reduces the effective scan cost versus an unordered list. No finding.
  - **Accessibility / never-color-alone:** every option row pairs a glyph
    (`!`/`✓`) with the state WORD ("recommended"/"already set") inside an
    outlined `Chip`, and the risk level is a plain-text word
    ("Low"/"Medium"/"High risk"), never a color-only severity cue — verified
    legible with color mentally removed. `fix-preview`'s diff uses `+`/blank
    prefix glyphs (not color) as the primary "changed vs. unchanged" signal,
    with color as a secondary reinforcement only. No finding.
  - **Distinctive-not-generic typography:** same terminal-skin theme as
    create-flow/git-screen/identity-manager (monospace JetBrains Mono, flat
    borders, zero elevation) — consistent visual language across all four
    now-built surfaces, no surface-specific styling drift. No finding.
  - **Advisory-not-blocking visual weight (the surface's own highest-risk
    affordance, §4.4):** every "needs action" cue on `options-list`,
    `option-detail`, `fix-preview`, and `confirm-write` uses MUI's `warning`
    (amber/yellow) `Alert` severity and the `!` glyph — never `error` (red).
    `result-applied`'s success state is a clean `success` green with no
    lingering amber, and `ForwardAgent`'s declined status is stated
    factually ("was left unchanged, as chosen") rather than flagged as an
    unresolved problem. This is the correct visual hierarchy for an advisory
    (not a compliance gate). No finding.
  - **CORRECTED by the 02-review-fixes pass (finding C1, Codex cross-vendor
    review) — this claim was WRONG, not a minor observation:**
    ~~`options-list.png`'s 6-row list plus the right-pane preview both fit
    comfortably within the 1280px capture viewport with no clipping (unlike
    `identity-manager/list-populated`'s 8-row list, which the identity-manager
    CRITIQUE flagged as slightly exceeding the fold) — no finding to log
    here, noted only for contrast.`~~ **This was false.** Row 6
    (`UseKeychain`, including its "(macOS only)" current-value qualifier) WAS
    clipped below the fold in the pre-fix capture — the same fixed-height
    `Shell` (`height: '100vh'` + `main`'s `overflow: 'auto'`) that clipped
    `identity-manager/list-populated`'s 8th row also clipped this screen's
    6th row; the original review simply never scrolled the captured PNG to
    check. Root cause and fix: `Shell.tsx`'s fixed-height + inner-scroll
    layout clipped ANY body taller than 800px INSIDE its own scroll
    container, which a full-page screenshot (`page.Screenshot(true, ...)`)
    cannot see past, since it only captures the OUTER document's scroll
    height. Fixed by letting the shell grow to its natural content height
    (`minHeight: '100vh'`, no inner `overflow: 'auto'`) — re-captured
    `options-list.png` is now 1280×992 (vs. the fixed 1280×800 before) and
    shows all 6 rows including `UseKeychain`'s "(macOS only)" qualifier,
    which was ALREADY present in `recipeFixtures.ts`'s `currentValue` field
    (`'yes (macOS only)'`) and in `surface_globalssh.go`'s `gsshOptions`
    (`current: "yes (macOS only)"`) the whole time — the qualifier was never
    actually missing from either medium's source, only invisible in the
    clipped capture.

## B. Structured parity findings log (HTML ↔ TUI, every named state)

Reviewed all 6 `html/*.png` against their `tui/*.png` counterpart,
cross-referenced against `FIELDS.md` and `parity.json`'s nine rows (the
seven §3 dimensions + the `per-option-explanation-verbatim` and
`advisory-not-blocking` rows).

| Finding # | Dimension (parity.json key) | Screen | Description | Status | Resolution |
|-----------|------------------------------|--------|--------------|--------|------------|
| 1 | (observation, not a §3 dimension) | all 6 | Same pre-existing 02-02 shell-infrastructure characteristic noted in `create-flow/CRITIQUE.md` finding #1, `git-screen/CRITIQUE.md` finding #1, and `identity-manager/CRITIQUE.md` finding #1: the TUI shell's status line is a single static empty region (`shell.go`'s `renderShellStatus` always returns `""`), so the live-nav status line never shows the per-screen wording the `/mui` `StatusLine` component does (e.g. "6 SSH options reviewed — 4 recommended, 2 already set. Advisory only."). Not introduced by global-ssh, not fixable within this surface's own files (`shell.go` is shared, out of scope per fan-out isolation), and not one of §3's seven MUST-match dimensions. | resolved (no action required — out of §3 scope) | Logged for awareness; not a `parity.json` row for the same reason the other three surfaces' finding #1 were not rows. Related, but distinct region: `create-flow`/`git-screen`/`identity-manager`'s own finding #1 (the missing header CONTEXT CHIP, a different region from this status line) was **FIXED** by the 02-review-fixes pass (finding A2) — `renderShellHeader` now shows "8 identities · ! needs action" on every surface including global-ssh (confirmed via the re-captured global-ssh/tui/*.png set). The status-line gap THIS row describes remains open and unaffected. |
| 2 | field-set-and-order / labels-and-helper-copy-verbatim | `options-list` | The live gitid-dummy binary renders inside a REAL, fixed 80×24 PTY with no scroll region (unlike the static `RenderScreen()`→`freeze` capture path, which has no height limit) — the original 4-line-per-option TUI row (glyph+key+status chip, current/recommended, one-liner) overflowed the terminal (verified empirically: `TestDummyNavReachesAllScreens/global-ssh/options-list` failed with the signature and 2 of 6 rows never reaching the visible frame). Compacted to ONE line per option (glyph + key + `current -> recommended`/`(already set)` + `[Risk]`), dropping the per-row one-liner explanation and the right-pane "Preview" block from the TUI's `options-list` specifically. | resolved | This is the SAME class of accepted divergence `git-screen`'s `review-readonly`/`confirm-write` already established (`gsFieldsCompactLine*`, justified in `surface_gitscreen.go`'s own comments) — §3 explicitly allows "exact spacing, pixel layout... provided the terminal skin keeps them close" and "MAY differ" cases include widget/layout compaction as long as the option SET/ORDER/VALUES match. Re-verified after the fix: `TestDummyNavReachesAllScreens/global-ssh/options-list` PASSES, and the field set (all 6 option keys, current values, recommended values, risk levels) is unchanged — only the one-liner explanation and the master-detail preview pane are TUI-omitted on this ONE screen, and the SAME one-liner content is available (byte-identical wording is not required by §3 for this supplementary layer) via `option-detail`'s full contractual explanation, one keystroke (`v`) away in both media. |
| 3 | keybindings-surfaced / label wording only | `options-list`, `option-detail`, `fix-preview`, `confirm-write`, `backup-notice` | Same shared-infrastructure characteristic as `git-screen`'s and `identity-manager`'s own keybar label-wording divergence (`shell.go`'s `renderShellKeybar` renders each intra-flow key's label as the literal TARGET SCREEN ID, e.g. "v option-detail", "f fix-preview", "w confirm-write", "y backup-notice", "z result-applied", while the HTML `Keybar` entries use a semantic action phrase, e.g. "View full explanation", "Preview fix", "Yes, write", "Continue"). Not introduced by global-ssh, `shell.go` is out of scope for a fan-out surface to edit. | resolved | The KEY itself (the letter) is identical in both media on every screen, and every key's DESTINATION is unambiguous from context (the breadcrumb after pressing it, or the target name embedded in the TUI's own hint line, e.g. "v full explanation (IdentitiesOnly)"). §3 permits "Widget mechanics ... as long as the option set and default match" — label WORDING differing while the key and destination match is the same class of allowed medium difference already accepted for `git-screen`/`identity-manager`. |

No other divergences found. Every §3 dimension (field set/order,
labels/copy, option sets, defaults, flow order, safety affordances,
keybindings) and the surface's two additional highest-risk-affordance rows
(`per-option-explanation-verbatim`: the six GSSH-01 options' current/risk/
recommended values and IdentitiesOnly's full contractual explanation match
byte-for-byte between media; `advisory-not-blocking`: the yellow `!`
advisory treatment, the "may leave any option unchanged" copy, and the
concrete 3-of-4-applied/ForwardAgent-declined demonstration all match,
carried through fix-preview → confirm-write → backup-notice →
result-applied in both media) matched between media on direct
screenshot-pair review — see `parity.json` for the per-dimension resolution
notes.

**0 open findings** — `.planning/design/global-ssh/parity.json` has no row
with `status != "resolved"` (verified:
`python3 -c "import json; r=json.load(open('.planning/design/global-ssh/parity.json')); assert r and all(x['status']=='resolved' for x in r)"`).
