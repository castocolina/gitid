# CRITIQUE.md — identity-manager (Phase 5 fan-out surface)

**Reviewer note:** the executor's toolset in this session was limited to
`Read`/`Write`/`Edit`/`Bash` — no `Task`/subagent-dispatch tool was available
to spawn a fresh-context `agent-ui-ux-designer` subagent (same limitation
recorded in 02-01-SUMMARY.md / 02-02-SUMMARY.md / 02-04's
`create-flow/CRITIQUE.md` / 02-05's `git-screen/CRITIQUE.md`). In its place,
this critique applies `agent-ui-ux-designer`'s documented methodology
(F-pattern/left-side bias, Fitts's/Hick's Law, accessibility,
distinctive-not-generic typography) directly against the captured
`.planning/design/identity-manager/html/*.png` and
`.planning/design/identity-manager/tui/*.png` screenshots (viewed and
compared side by side, all 16 images), and fills the structured parity
findings log against `FIELDS.md` and `parity.json`. **This does not
substitute for a fresh-context `agent-ui-ux-designer` pass** — flagging
explicitly so the phase-level `/gsd-code-review` and the external
cross-vendor review can re-run one if the orchestrator has that capability
this session lacked.

---

## Surface: `identity-manager`

## A. Aesthetic / usability pass (HTML mockup only)

- **Reviewer:** this executor, applying `agent-ui-ux-designer`'s documented
  methodology directly (Task/subagent dispatch unavailable — see note above)
- **Screenshots reviewed:** all 8 `.planning/design/identity-manager/html/*.png`
- **Findings:**
  - **F-pattern / left-side bias:** the master-detail archetype places the
    identity list flush-left at the shared `px: 2` inset with the preview/
    detail panel to the right (`list-populated`, `detail-ssh-first`) —
    matches the two-archetype layout contract (UX-DIRECTION.md §2). The
    5 focused single-column screens (`action-menu`, `clone-name-prompt`,
    `delete-choice`, `confirm-destructive`, `backup-notice`) also read
    top-to-bottom, left-aligned. No finding.
  - **Fitts's Law (target size/reachability):** the delete-choice option
    `Paper`s are full-width, generously padded (`p: 2`) targets, with the
    default option's 2px green border making it the single largest visual
    weight on the screen — no undersized targets. No finding.
  - **Hick's Law (choice count):** `delete-choice` presents exactly 2
    options with the safer one visually dominant (green 2px border + "✓
    default"), reducing the effective decision to "confirm the obvious
    safe default, or deliberately opt into the riskier one" rather than an
    undifferentiated 2-way pick — directly mirrors `match-strategy-select`'s
    already-approved default-highlighting pattern from git-screen. No
    finding.
  - **Accessibility / never-color-alone:** every one of the 8 MGR-02 state
    labels on `list-populated`/`detail-ssh-first` pairs a glyph (✓/!/✗)
    with the label's own WORD inside a bordered `Chip` — verified this is
    legible with color entirely removed (the word alone disambiguates all
    8 states; glyph is a redundant, not sole, cue). `delete-choice`'s
    default marker is "✓ default" (glyph+word), and `confirm-destructive`'s
    warning is "✗ This action is irreversible." (glyph+word, updated by
    02-review-fixes finding A5 — was "✗ This cannot be undone" at the time
    of this aesthetic pass; see the Structured parity findings log below).
    No finding.
  - **Distinctive-not-generic typography:** same terminal-skin theme as
    create-flow/git-screen (monospace JetBrains Mono, flat borders, zero
    elevation) — consistent visual language across all three now-built
    surfaces, no surface-specific styling drift. No finding.
  - **RESOLVED by the 02-review-fixes pass (finding C1, Codex cross-vendor
    review) — previously a "minor observation," now actually fixed:**
    `list-populated.png`'s 8-row list previously exceeded the 1280px
    capture viewport's visible height (the 8th row,
    `legacy`/fragment-path-missing, was clipped at the fold). This WAS
    correctly logged as non-blocking at the time (all 8 rows were present
    in the DOM, just not in the captured PNG — the "Scroll (HTML) vs
    paginate (TUI)" §3 MAY-differ case), but the Codex review flagged it as
    worth fixing so reference screenshots do not require a human to
    remember "some content is below the fold." Root cause: `Shell.tsx`'s
    fixed `height: '100vh'` + `main`'s inner `overflow: 'auto'` clipped any
    body taller than 800px INSIDE its own scroll region, invisible to a
    full-page screenshot capture. Fixed by letting the shell grow to its
    natural height (`minHeight: '100vh'`, no inner scroll) — the
    re-captured `list-populated.png` now shows all 8 rows in full.

## B. Structured parity findings log (HTML ↔ TUI, every named state)

Reviewed all 8 `html/*.png` against their `tui/*.png` counterpart,
cross-referenced against `FIELDS.md` and `parity.json`'s ten rows (the
seven §3 dimensions + the `delete-choice-safe-default`,
`no_color-row-health`, and `ssh-first-detail` highest-risk-affordance
rows).

| Finding # | Dimension (parity.json key) | Screen | Description | Status | Resolution |
|-----------|------------------------------|--------|--------------|--------|------------|
| 1 | (observation, not a §3 dimension) | all 8 | Same pre-existing 02-02 shell-infrastructure characteristic noted in `create-flow/CRITIQUE.md` finding #1 and `git-screen/CRITIQUE.md` finding #1: the TUI shell header renders only the app name + breadcrumb, not the HTML `Header.tsx`'s "N identities · ✓/!/✗ <word>" context chip text inline (the TUI's own status line differs in wording from the HTML `StatusLine`, e.g. "8 identities — every MGR-02 state label represented." vs the HTML header chip). Not introduced by identity-manager, not fixable within this surface's own files (`shell.go` is shared, out of scope per fan-out isolation), and not one of §3's seven MUST-match dimensions. | **FIXED** (02-review-fixes, finding A2) | Logged for awareness; not a `parity.json` row for the same reason `create-flow`'s and `git-screen`'s finding #1 were not rows. **Update:** the 02-review-fixes pass added the missing chip to `renderShellHeader` (shell.go) — a static "8 identities · ! needs action" fixture matching the HTML header's semantic content — rendered on every surface including identity-manager (confirmed via the re-captured identity-manager/tui/*.png set). The TUI's own per-screen status-line wording difference (e.g. "8 identities — every MGR-02 state label represented.") is unaffected and remains a separate, still-open, out-of-§3-scope observation. |
| 2 | keybindings-surfaced / label wording only | `action-menu`, `clone-name-prompt`, `delete-choice`, `confirm-destructive` | The TUI keybar renders each intra-flow key's label as the literal TARGET SCREEN ID (`shell.go`'s `renderShellKeybar` uses `scr.Keys[k]` verbatim, e.g. "y backup-notice"), while the HTML `Keybar` entries use a semantic action phrase (e.g. "Yes, delete everything (typed confirm)"). Same shared-infrastructure characteristic as `git-screen`'s `f`/`m`/`r`/`w`/`y`/`z` keybar labels — not introduced by identity-manager, `shell.go` is out of scope for a fan-out surface to edit. | resolved | The KEY itself (the letter) is identical in both media on every screen, and every key's DESTINATION is unambiguous from context (the screen breadcrumb after pressing it). §3 permits "Widget mechanics ... as long as the option set and default match" — the label WORDING differing while the key and destination match is the same class of allowed medium difference. Note (02-review-fixes, finding A6): `shell.go` gained an OPT-IN `ScreenDef.KeyLabels` override mechanism, used by create-flow's `confirm-write` to show "y Yes, write" — identity-manager's own screens were left unchanged (out of this fix pass's scope); a future pass could apply the same override here if desired. |
| 3 | labels-and-helper-copy-verbatim | `clone-name-prompt`, `confirm-destructive` | 02-review-fixes findings A5 (external cross-vendor review): clone-name-prompt.route.tsx's explainer reads "...the key material itself is not copied; a new key is generated for the clone." — `renderIMCloneNamePrompt` dropped that clause entirely. confirm-destructive.route.tsx opens with "This action is irreversible." — `renderIMConfirmDestructive` said "This cannot be undone." instead (same meaning, different words). | **FIXED** | Both TUI screens now carry the HTML's exact wording. Re-verified word-for-word against the route files and the re-captured `clone-name-prompt.png`/`confirm-destructive.png` html/tui pairs. See `parity.json`'s `labels-and-helper-copy-verbatim` row for the full before/after text. |
| 4 | (color-semantics correction, not a copy divergence) | `detail-ssh-first` | 02-review-fixes finding A7: the SSH-only Git note ("No Git identity configured for this alias — SSH-only.") used the WARNING glyph (`!`, yellow) in the TUI, but this note is informational per MGR-03/MGR-07 (SSH-only is an expected, healthy state for some identities), matching the HTML's `severity="info"` Alert treatment — the LOCKED severity-glyph contract (warning=`!` yellow, error/critical=`✗` red, info=`~` cyan, established by `surface_health.go`/`surface_fixer.go`) was violated. | **FIXED** | `renderIMDetailSSHFirst` now uses the info glyph `~` (cyan, `styleIMInfo`) instead of the warning glyph. The note's WORDS were already verbatim and remain unchanged — this was a glyph/semantic-tone correction only. Re-verified against the re-captured `detail-ssh-first.png` tui/html pair. |

No other divergences found. Every §3 dimension (field set/order,
labels/copy, option sets, defaults, flow order, safety affordances,
keybindings) and the surface's highest-risk affordances (delete-choice's
safe default, the 8-label taxonomy's NO_COLOR legibility, and
detail-ssh-first's SSH-first/no-fabricated-Git-attributes rule) matched
between media on direct screenshot-pair review — see `parity.json` for the
per-dimension resolution notes.

**0 open findings** — `.planning/design/identity-manager/parity.json` has
no row with `status != "resolved"` (verified:
`python3 -c "import json; r=json.load(open('.planning/design/identity-manager/parity.json')); assert r and all(x['status']=='resolved' for x in r)"`).
