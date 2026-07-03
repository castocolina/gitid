# CRITIQUE.md — global-git (Phase 7 fan-out surface)

**Reviewer note:** the executor's toolset in this session was limited to
`Read`/`Write`/`Edit`/`Bash` — no `Task`/subagent-dispatch tool was available
to spawn a fresh-context `agent-ui-ux-designer` subagent (same limitation
recorded in `create-flow/CRITIQUE.md` (02-04), `git-screen/CRITIQUE.md`
(02-05), `identity-manager/CRITIQUE.md` (02-06), and `global-ssh/CRITIQUE.md`
(02-07)). In its place, this critique applies `agent-ui-ux-designer`'s
documented methodology (F-pattern/left-side bias, Fitts's/Hick's Law,
accessibility, distinctive-not-generic typography) directly against the
captured `.planning/design/global-git/html/*.png` and
`.planning/design/global-git/tui/*.png` screenshots (viewed and compared
side by side, all 12 images), and fills the structured parity findings log
against `FIELDS.md` and `parity.json`. **This does not substitute for a
fresh-context `agent-ui-ux-designer` pass** — flagging explicitly so the
phase-level `/gsd-code-review` and the external cross-vendor review can
re-run one if the orchestrator has that capability this session lacked.

---

## Surface: `global-git`

## A. Aesthetic / usability pass (HTML mockup only)

- **Reviewer:** this executor, applying `agent-ui-ux-designer`'s documented
  methodology directly (Task/subagent dispatch unavailable — see note above)
- **Screenshots reviewed:** all 6 `.planning/design/global-git/html/*.png`
- **Findings:**
  - **F-pattern / left-side bias:** `options-list` uses the master-detail
    archetype (§2) — the 11-option list flush-left, the highlighted
    option's preview to the right; the 5 single-column ceremony screens
    (`option-detail`, `fix-preview`, `confirm-write`, `backup-notice`,
    `result-applied`) read top-to-bottom, left-aligned — same layout
    grammar as `global-ssh`. No finding.
  - **Fitts's Law (target size/reachability):** each option row on
    `options-list` is a full-width `ListItemButton` with generous padding
    — no undersized targets, consistent with `global-ssh`'s own
    `options-list` precedent. The highlighted `init.defaultBranch` row
    carries a subtle selection background plus its own dedicated
    "main vs master" outlined chip, distinguishing it from the plain
    "recommended"/"informational" chip every other row gets. No finding.
  - **Hick's Law (choice count):** 11 rows is markedly more than
    `global-ssh`'s 6, so `options-list.png` runs past the 1280px capture
    fold (the list continues below the visible viewport, `user.email`'s
    row is the last one partially visible). This is the SAME class of
    observation `identity-manager/CRITIQUE.md` logged for its own
    8-row `list-populated` state (not a parity finding — an HTML-only
    scroll characteristic, and the live app is a scrollable `<main>`
    region per `Shell.tsx`, `overflow: 'auto'`). No finding requiring a
    fix; noted for awareness only, same disposition as the
    `identity-manager` precedent.
  - **Accessibility / never-color-alone:** every option row pairs a glyph
    (`!`/`✓`) with the state WORD ("recommended"/"informational") inside an
    outlined `Chip`, never a color-only severity cue — verified legible
    with color mentally removed. The dedicated "main vs master" chip on
    `init.defaultBranch` is ALSO a labeled chip (word, not color-only).
    `fix-preview`'s diff uses `+`/blank prefix glyphs (not color) as the
    primary "changed vs. unchanged" signal, with color as a secondary
    reinforcement only. No finding.
  - **Distinctive-not-generic typography:** same terminal-skin theme as
    create-flow/git-screen/identity-manager/global-ssh (monospace
    JetBrains Mono, flat borders, zero elevation) — consistent visual
    language across all five now-built surfaces, no surface-specific
    styling drift. No finding.
  - **Advisory-not-blocking visual weight (§4.5/§5, the "advisory, never
    blocking" rule shared with global-ssh):** every "needs action" cue on
    `options-list`, `option-detail`, `fix-preview`, and `confirm-write`
    uses MUI's `warning` (amber/yellow) `Alert` severity and the `!` glyph
    — never `error` (red). `result-applied`'s success state is a clean
    `success` green with no lingering amber, and `user.email`'s
    "informational" status is stated factually (never flagged as an
    unresolved problem, unlike the 10 "recommended" rows). This is the
    correct visual hierarchy for an advisory, reused consistently from
    `global-ssh`. No finding.
  - **Managed-block containment visual weight (GGIT-01, the surface's own
    highest-risk affordance):** `confirm-write.png` renders the
    `# BEGIN/END gitid managed: global-git` sentinels as plain monospace
    text inside a bordered `Paper` labeled "Will append to ~/.gitconfig" —
    visually distinct from the warning `Alert` above it, correctly framing
    "this exact text will be added" as a neutral fact rather than a
    warning. `result-applied.png` explicitly restates "Everything outside
    the sentinels ... was preserved verbatim", closing the ceremony loop.
    No finding.

## B. Structured parity findings log (HTML ↔ TUI, every named state)

Reviewed all 6 `html/*.png` against their `tui/*.png` counterpart,
cross-referenced against `FIELDS.md` and `parity.json`'s nine rows (the
seven §3 dimensions + the `main-vs-master-highlight` and
`managed-block-containment` rows).

| Finding # | Dimension (parity.json key) | Screen | Description | Status | Resolution |
|-----------|------------------------------|--------|--------------|--------|------------|
| 1 | (observation, not a §3 dimension) | all 6 | Same pre-existing 02-02 shell-infrastructure characteristic noted in every prior surface's own CRITIQUE.md finding #1 (`create-flow`, `git-screen`, `identity-manager`, `global-ssh`): the TUI shell's status line is a single static empty region (`shell.go`'s `renderShellStatus` always returns `""`), so the live-nav status line never shows the per-screen wording the `/mui` `StatusLine` component does (e.g. "11 git options reviewed — 10 recommended, 1 informational. Advisory only."). Not introduced by global-git, not fixable within this surface's own files (`shell.go` is shared, out of scope per fan-out isolation), and not one of §3's seven MUST-match dimensions. | resolved (no action required — out of §3 scope) | Logged for awareness; not a `parity.json` row for the same reason every other surface's finding #1 was not a row. |
| 2 | field-set-and-order / labels-and-helper-copy-verbatim | `options-list` | The live gitid-dummy binary renders inside a REAL, fixed 80×24 PTY with no scroll region (unlike the static `RenderScreen()`→`freeze` capture path, which has no height limit) — the original per-option 4-line-equivalent TUI row overflowed the terminal for 11 options plus the header/status/keybar chrome (this is the SAME class of overflow `global-ssh`'s own `options-list` hit at 6 options, but worse at 11: the FIRST attempt at `fix-preview`/`confirm-write` — which render the full 30-line sentineled block — failed `TestDummyNavReachesAllScreens/global-git/{fix-preview,confirm-write}` outright, never reaching the breadcrumb/signature marker). Compacted `options-list` to ONE line per option (glyph + key + `current -> recommended`/`(recommended-value)` + an optional `[main vs master]` bracket), dropping the per-row one-liner explanation from the TUI's `options-list` specifically; compacted `fix-preview`/`confirm-write` to 5 grouped `key=value` lines (`ggitCompactValueLines`, mirroring `git-screen`'s own `gsFieldsCompactLine1/2/3` precedent) instead of the full per-section, per-key block. | resolved | This is the SAME class of accepted divergence `git-screen`'s `review-readonly`/`confirm-write` and `global-ssh`'s `options-list` already established (§3 explicitly allows "exact spacing, pixel layout... provided the terminal skin keeps them close" and "MAY differ" cases include widget/layout compaction as long as the option SET/ORDER/VALUES match). Re-verified after the fix: `TestDummyNavReachesAllScreens/global-git/*` PASSES for all 6 screens (`go test -tags e2e -race -run 'TestDummyNavReachesAllScreens/global-git' ./e2e/...`), and the field set (all 11 option keys/current/recommended values, all 8 aliases, all 4 color settings) is unchanged — only the layout/grouping is TUI-compacted; the same values are present, byte-identical, in the HTML mockup's full block text and in `option-detail`'s (the one full-explanation target) contractual copy. |
| 3 | keybindings-surfaced / label wording only | `options-list`, `option-detail`, `fix-preview`, `confirm-write`, `backup-notice` | Same shared-infrastructure characteristic as `git-screen`'s, `identity-manager`'s, and `global-ssh`'s own keybar label-wording divergence (`shell.go`'s `renderShellKeybar` renders each intra-flow key's label as the literal TARGET SCREEN ID, e.g. "v option-detail", "f fix-preview", "w confirm-write", "y backup-notice", "z result-applied", while the HTML `Keybar` entries use a semantic action phrase, e.g. "View full explanation", "Preview fix", "Yes, write", "Continue"). Not introduced by global-git, `shell.go` is out of scope for a fan-out surface to edit. | resolved | The KEY itself (the letter) is identical in both media on every screen, and every key's DESTINATION is unambiguous from context (the breadcrumb after pressing it, or the target name embedded in the TUI's own hint line, e.g. "v full explanation (init.defaultBranch)"). §3 permits "Widget mechanics ... as long as the option set and default match" — label WORDING differing while the key and destination match is the same class of allowed medium difference already accepted for `git-screen`/`identity-manager`/`global-ssh`. |
| 4 | main-vs-master-highlight | `options-list`, `option-detail` | GGIT-01's own dedicated highlight for `init.defaultBranch` (the "main vs master" naming-convention explanation). HTML renders it as an outlined amber `Chip` labeled "main vs master" next to the option key on `options-list` (`options-list.png`), and again beside the current/recommended values on `option-detail` (`option-detail.png`); TUI renders it as a trailing `[main vs master]` bracket on the same compact line (`options-list.png`/`option-detail.png` under `tui/`), and the full explanatory paragraph on `option-detail` in both media states, verbatim, that Git's own compiled-in default is still "master" and explains why "main" is now recommended without ever implying existing repositories are renamed. | resolved | The highlight is present, labeled with the same two words ("main vs master") in both media, on both screens where GGIT-01 requires it; `TestGlobalGit_MainVsMasterHighlighted` (Go) asserts both "main" and "master" and the exact "main vs master" phrase appear on both screens in the live TUI render. |
| 5 | managed-block-containment | `fix-preview`, `confirm-write`, `result-applied` | GGIT-01's highest-risk affordance: writes must preserve content outside the managed block verbatim. `fix-preview` and `confirm-write` both state "gitid only owns the block between its sentinels — everything else in ~/.gitconfig ... is preserved verbatim" (HTML: `fix-preview.png`/`confirm-write.png`; TUI: the SAME sentence, unabbreviated, in `renderGGITFixPreview`); `confirm-write` renders the literal `# BEGIN gitid managed: global-git` / `# END gitid managed: global-git` sentinel lines around the (HTML: full, TUI: compacted-but-complete-field-set) block text in BOTH media; `result-applied` restates "Everything outside the sentinels — including any hand-written [user]/[includeIf]/[url] sections — was preserved verbatim" in both media, naming concrete example sections a user is likely to have hand-written. `global user.email`'s absence from the managed block is called out explicitly on every ceremony screen (`fix-preview`, `confirm-write`, `result-applied`) in both media — not a silent omission. | resolved | `TestGlobalGit_ManagedBlockContainmentShown` (Go) asserts both sentinel lines, the target file, and the "preserved verbatim" phrase are present in the live TUI render of `confirm-write`/`fix-preview`/`result-applied`; `TestGlobalGit_AdvisoryNeverBlocking` additionally asserts `user.email` stays visible through `fix-preview`→`confirm-write`→`result-applied`, mirroring `global-ssh`'s own declined-option-stays-visible proof (`TestGlobalSSH_AdvisoryNeverBlocking`). |

No other divergences found. Every §3 dimension (field set/order,
labels/copy, option sets, defaults, flow order, safety affordances,
keybindings) and the surface's two additional highest-risk-affordance rows
(`main-vs-master-highlight`: the "main vs master" chip/bracket and the
init.defaultBranch explanation match, byte-for-byte, between media;
`managed-block-containment`: the sentinel-visible managed block, the
"preserved verbatim" language, and the explicit user.email-absence note all
match, carried through fix-preview → confirm-write → backup-notice →
result-applied in both media) matched between media on direct
screenshot-pair review — see `parity.json` for the per-dimension resolution
notes.

**0 open findings** — `.planning/design/global-git/parity.json` has no row
with `status != "resolved"` (verified:
`python3 -c "import json; r=json.load(open('.planning/design/global-git/parity.json')); assert r and all(x['status']=='resolved' for x in r)"`).
