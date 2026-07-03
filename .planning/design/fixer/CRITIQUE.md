# CRITIQUE.md — fixer (Phase 8 fan-out surface, the FINAL of 7)

**Reviewer note:** the executor's toolset in this session was limited to
`Read`/`Write`/`Edit`/`Bash` — no `Task`/subagent-dispatch tool was available
to spawn a fresh-context `agent-ui-ux-designer` subagent (same limitation
recorded in `create-flow/CRITIQUE.md` (02-04), `git-screen/CRITIQUE.md`
(02-05), `identity-manager/CRITIQUE.md` (02-06), `global-ssh/CRITIQUE.md`
(02-07), `global-git/CRITIQUE.md` (02-08), and `health/CRITIQUE.md` (02-09)).
In its place, this critique applies `agent-ui-ux-designer`'s documented
methodology (F-pattern/left-side bias, Fitts's/Hick's Law, accessibility,
distinctive-not-generic typography) directly against the captured
`.planning/design/fixer/html/*.png` and `.planning/design/fixer/tui/*.png`
screenshots (viewed and compared side by side, all 12 images), and fills
the structured parity findings log against `FIELDS.md` and `parity.json`.
**This does not substitute for a fresh-context `agent-ui-ux-designer`
pass** — flagging explicitly so the phase-level `/gsd-code-review` and the
external cross-vendor review can re-run one if the orchestrator has that
capability this session lacked.

---

## Surface: `fixer`

## A. Aesthetic / usability pass (HTML mockup only)

- **Reviewer:** this executor, applying `agent-ui-ux-designer`'s documented
  methodology directly (Task/subagent dispatch unavailable — see note above)
- **Screenshots reviewed:** all 6 `.planning/design/fixer/html/*.png`
- **Findings:**
  - **F-pattern / left-side bias:** `fixer-list` and `nothing-to-fix` use
    the master-detail archetype (§2) — the two-section problem list
    flush-left, a preview/summary pane to the right; the 4 single-column
    ceremony screens (`fix-preview`, `confirm-destructive`,
    `backup-notice`, `result-applied`) read top-to-bottom, left-aligned —
    same layout grammar as `health`'s own two-section screens and
    `global-git`'s ceremony chain. No finding.
  - **Fitts's Law (target size/reachability):** each problem row on
    `fixer-list` is a full-width `ListItemButton` with generous padding —
    no undersized targets, consistent with `health`'s own
    `health-with-findings` precedent. No finding.
  - **Hick's Law (choice count):** `fixer-list` shows 4 actionable
    problems (3 SSH, 1 Git) — well within a single viewport, no fold
    concern (unlike `global-git`'s 11-row `options-list`, which
    intentionally scrolls). No finding.
  - **Accessibility / never-color-alone:** every problem row pairs a
    severity glyph (`~`/`!`/`✗`) with the severity WORD, reusing health's
    LOCKED glyph contract byte-identically — verified legible with color
    mentally removed. `fix-preview`'s diff uses `-`/`+` TEXTUAL prefixes
    (not color alone) as the primary changed/unchanged signal — the
    correct choice for a true rewrite diff, stronger than global-git's/
    global-ssh's color-reinforced `+`-only convention because `-`/`+` are
    unambiguous even in monochrome. No finding.
  - **Distinctive-not-generic typography:** same terminal-skin theme as
    all six prior surfaces (monospace JetBrains Mono, flat borders, zero
    elevation) — consistent visual language across the now-complete
    seven-surface set, no surface-specific styling drift. No finding.
  - **Destructive-confirm visual weight (§4.7/§5, the surface's own
    highest-risk affordance):** `confirm-destructive.png` uses MUI's
    `error` (red) `Alert` severity — a deliberately STRONGER visual weight
    than `global-git`'s advisory `warning` (amber) chain, correctly
    reflecting that this beat is irreversible without the backup, mirroring
    `identity-manager`'s own "delete everything" `confirm-destructive`
    visual language (the same red-severity precedent, reused consistently
    for the SAME class of action: rewriting/removing something that
    already exists, not merely adding something new). No finding.
  - **Fix-in-place diff legibility (§4.7's flagship highest-risk
    affordance):** `fix-preview.png` renders the full Host block as
    two-space-indented context lines with the ONE changed line marked
    `-`/`+` — the changed line is visually distinguishable by the leading
    glyph alone (monochrome-safe), and the surrounding "Only the
    highlighted line changes" caption reinforces it in words too (never
    glyph-only). No finding.

## B. Structured parity findings log (HTML ↔ TUI, every named state)

Reviewed all 6 `html/*.png` against their `tui/*.png` counterpart,
cross-referenced against `FIELDS.md` and `parity.json`'s nine rows (the
seven §3 dimensions + the `fix-in-place-diff-and-backup` and
`nothing-to-fix-empty-state` rows).

| Finding # | Dimension (parity.json key) | Screen | Description | Status | Resolution |
|-----------|------------------------------|--------|--------------|--------|------------|
| 1 | (observation, not a §3 dimension) | all 6 | Same pre-existing 02-02 shell-infrastructure characteristic noted in every prior surface's own CRITIQUE.md finding #1 (`create-flow`, `git-screen`, `identity-manager`, `global-ssh`, `global-git`, `health`): the TUI shell's status line is a single static empty region (`shell.go`'s `renderShellStatus` always returns `""`), so the live-nav status line never shows the per-screen wording the `/mui` `StatusLine` component does (e.g. "4 fixable problems across SSH and Git."). Not introduced by fixer, not fixable within this surface's own files (`shell.go` is shared, out of scope per fan-out isolation), and not one of §3's seven MUST-match dimensions. | resolved (no action required — out of §3 scope) | Logged for awareness; not a `parity.json` row, for the same reason every other surface's finding #1 was not a row. Related, but distinct region: the header CONTEXT CHIP (a different region from this status line) was **FIXED** by the 02-review-fixes pass (finding A2) — `renderShellHeader` now shows "8 identities · ! needs action" on every surface including fixer. The status-line gap this row describes remains open and unaffected. |
| 2 | field-set-and-order / labels-and-helper-copy-verbatim | `fixer-list` | HTML's right-pane master-detail preview (highlighted problem's full explanation + suggested fix, `detail_preview` in FIELDS.md) is TUI-omitted in favor of a one-line "v preview fix (IdentitiesOnly no contradicts an explicit IdentityFile)" keybar hint — the SAME accepted compaction class `health`'s `health-with-findings` (CRITIQUE.md #2, 02-09) and `global-git`'s `options-list` (CRITIQUE.md #2, 02-08) already established for the fixed 80×24 live-PTY viewport with no scroll region. | resolved | The same divergence class §3 explicitly allows ("MAY differ: exact spacing, pixel layout... provided the terminal skin keeps them close"); the full explanation is present, byte-identical, in both media's `fixer-list` LIST rows (title + suggestedFix on every row, both media) and the target's full title is named in the TUI hint. Re-verified: `TestFixer_SeverityExplanationSuggestedFixPresent` (Go) asserts every finding's title AND suggestedFix appear in the live TUI render. |
| 3 | keybindings-surfaced / label wording only | `fixer-list`, `fix-preview`, `confirm-destructive`, `backup-notice` | Same shared-infrastructure characteristic as every prior surface's own keybar label-wording divergence (`shell.go`'s `renderShellKeybar` renders each intra-flow key's label as the literal TARGET SCREEN ID, e.g. "x confirm-destructive", "y backup-notice", "z result-applied", while the HTML `Keybar` entries use a semantic action phrase, e.g. "Confirm (rewrites an existing directive)", "Apply the fix"). Not introduced by fixer, `shell.go` is out of scope for a fan-out surface to edit. | resolved | The KEY itself (the letter) is identical in both media on every screen, and every key's DESTINATION is unambiguous from context (the breadcrumb after pressing it, or the target name embedded in the TUI's own hint line, e.g. "v preview fix (IdentitiesOnly no contradicts an explicit IdentityFile)"). §3 permits "Widget mechanics... as long as the option set and default match" — label WORDING differing while the key and destination match is the same class of allowed medium difference already accepted for every prior surface. |
| 4 | fix-in-place-diff-and-backup | `fix-preview`, `confirm-destructive`, `backup-notice`, `result-applied` | §4.7's own flagship highest-risk affordance (T-02-FIX): fix-in-place rewrites of an EXISTING directive show a before/after diff and name the backup path before applying. `fix-preview` renders a TRUE `-`/`+` rewrite diff (not additions-only, unlike global-ssh's/global-git's fix-preview) in BOTH media, byte-identical field values (`fixerFixPreviewLines`/`fixFixPreviewLines`); `confirm-destructive` uses the strongest confirm short of a typed confirmation, default-focused "No" in both media (never "Yes"); `backup-notice` names the timestamped SSH-config backup path (`sampleBackupPath`/`fixBackupPath`, byte-identical) BEFORE the fix is applied, in both media; `result-applied` restates what changed and the restore path. | resolved | `TestFixer_FixInPlaceDiffShowsRewriteNotAddition` (asserts the `-`/`+` lines and the REWRITES framing), `TestFixer_ConfirmDestructiveNeverDefaultsToYes` (asserts default-focused-No, never Yes), `TestFixer_BackupNoticeNamesPathBeforeApplying`, and `TestFixer_ResultAppliedNamesRestorePath` (Go) all pass against the live TUI render; direct screenshot-pair review confirms the SAME sequence, copy, and field values in HTML. |
| 5 | nothing-to-fix-empty-state | `nothing-to-fix` | §4.7's healthy empty state — both SSH and Git sections report zero fixable problems, mirroring `health`'s own `health-all-green` two-section-even-when-empty precedent. Both media show a green-bordered `SSH`/`Git` pair, each with a `✓`-prefixed zero-findings summary sentence, plus the SAME safety banner (`fixerSafetyNote`/`fixSafetyNote`) every other fixer screen carries. | resolved | `TestFixer_NothingToFixIsTheHealthyEmptyState` and `TestFixer_ListAndNothingToFixHaveBothSSHAndGitSections` (Go) both pass against the live TUI render; direct screenshot-pair review (`nothing-to-fix.png` html/tui) confirms the same two-section, green-✓, safety-banner-present layout in both media. |
| 6 | (traceability, not a §3 dimension) | `fixer-list`, `fix-preview`, `confirm-destructive` | The fixer's flagship walk-through target (`fixerTarget`/`fixTarget`) is the SAME `ssh-identitiesonly-contradiction` finding health/finding-detail deep-dives — not a re-derived duplicate. This is the concrete Health→Fixer hand-off HLTH-04's `suggestedFix` text ("available on the Fixer screen") promises. | resolved (traceability proof, not a `parity.json` row — this is a cross-surface consistency check, not an HTML↔TUI parity dimension) | `TestFixer_TargetFindingTracesTheSameFindingAsHealth` (Go) asserts `fixTarget.id`/`title`/`explanation` are byte-identical to `hlthFindingDetailTarget`'s. Both `recipeFixtures.ts`'s `fixerTarget` and `fixerFindings` are also literal `healthFindings.find(...)`/`.filter(...)` calls, not copy-pasted duplicates, so the HTML mockup's own source enforces the same invariant at the TypeScript level. |

No other divergences found. Every §3 dimension (field set/order,
labels/copy, option sets, defaults, flow order, safety affordances,
keybindings) and the surface's two additional highest-risk-affordance rows
(`fix-in-place-diff-and-backup`: the true rewrite diff, the strongest
confirm, and the backup-before-apply sequence match, byte-for-byte, between
media; `nothing-to-fix-empty-state`: the two-section healthy summary matches
between media) matched between media on direct screenshot-pair review — see
`parity.json` for the per-dimension resolution notes.

**0 open findings** — `.planning/design/fixer/parity.json` has no row
with `status != "resolved"` (verified:
`python3 -c "import json; r=json.load(open('.planning/design/fixer/parity.json')); assert r and all(x['status']=='resolved' for x in r)"`).
