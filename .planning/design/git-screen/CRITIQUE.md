# CRITIQUE.md — git-screen (Phase 4 fan-out surface)

**Reviewer note:** the executor's toolset in this session was limited to
`Read`/`Write`/`Edit`/`Bash` — no `Task`/subagent-dispatch tool was available
to spawn a fresh-context `agent-ui-ux-designer` subagent (same limitation
recorded in 02-01-SUMMARY.md / 02-02-SUMMARY.md / 02-04's
`create-flow/CRITIQUE.md`). In its place, this critique applies
`agent-ui-ux-designer`'s documented methodology (F-pattern/left-side bias,
Fitts's/Hick's Law, accessibility, distinctive-not-generic typography)
directly against the captured `.planning/design/git-screen/html/*.png` and
`.planning/design/git-screen/tui/*.png` screenshots, and fills the
structured parity findings log against `FIELDS.md` and `parity.json`.
**This does not substitute for a fresh-context `agent-ui-ux-designer`
pass** — flagging explicitly so the phase-level `/gsd-code-review` and the
external cross-vendor review can re-run one if the orchestrator has that
capability this session lacked.

---

## Surface: `git-screen`

## A. Aesthetic / usability pass (HTML mockup only)

- **Reviewer:** this executor, applying `agent-ui-ux-designer`'s documented
  methodology directly (Task/subagent dispatch unavailable — see note above)
- **Screenshots reviewed:** all 7 `.planning/design/git-screen/html/*.png`
- **Findings:**
  - **F-pattern / left-side bias:** every screen's primary content (form
    fields, match-strategy options, review panels, confirm/backup/result
    text) starts flush-left at the same `px: 2` inset as the shared shell
    body, with the live-preview / secondary panel to the right on the two
    guided-form screens (`git-form-empty`, `git-form-filled`,
    `match-strategy-select`) — matches the two-archetype layout contract
    (UX-DIRECTION.md §2). No finding.
  - **Fitts's Law (target size/reachability):** form fields and the
    match-strategy `Paper` option rows are full-width, generously padded
    (`p: 1.5`/`p: 2`) targets — no undersized click targets. No finding.
  - **Hick's Law (choice count):** the match-strategy picker presents
    exactly 3 options with the default (`gitdir`) visually distinguished by
    a green border + "✓ default" marker — reduces the effective decision to
    a single obvious default plus 2 clearly-secondary alternatives, not an
    undifferentiated 3-way pick. No finding.
  - **Accessibility / never-color-alone:** every colored state (success
    green, warning yellow, error/mismatch red) pairs a glyph (✓/!/✗) with a
    word, matching UX-DIRECTION.md §2's color-semantics table — verified on
    `review-readonly.png` (green ✓ + "Byte-identical" when emails match, or
    a red-bordered ✗ Alert on a hypothetical mismatch — the `emailsMatch`
    conditional in `review-readonly.route.tsx`), `confirm-write.png`
    (yellow warning triangle + "Nothing has changed yet"), `backup-notice.png`
    / `result-success.png` (green check + word). No finding.
  - **Distinctive-not-generic typography:** the terminal-skin theme
    (monospace JetBrains Mono, flat borders, zero elevation) reads as a
    screenshot of a real terminal tool rather than a generic Material
    dashboard, consistent with the anti-slop stance (UX-DIRECTION.md §1),
    and consistent with the already-approved create-flow pilot's visual
    language (same `Shell`/`Header`/`Keybar`/`StatusLine` components, no
    surface-specific styling drift). No finding.
  - **Minor observation (not a parity finding — HTML-only cosmetic):**
    `git-form-empty.png`, `match-strategy-select.png`, and
    `review-readonly.png` all leave a large empty area below their content
    column on the 1280px capture viewport (the guided-form archetype's
    body doesn't fill the vertical space at these state's content length).
    This is consistent with `create-flow/CRITIQUE.md`'s own noted
    observation on `ssh-form-blank-prefix.png` and is explicitly NOT a §3
    MUST-match dimension (§3: "exact spacing, pixel layout... MAY differ").
    No action needed; noted for awareness only.

## B. Structured parity findings log (HTML ↔ TUI, every named state)

Reviewed all 7 `html/*.png` against their `tui/*.png` counterpart,
cross-referenced against `FIELDS.md` and `parity.json`'s nine rows (the
seven §3 dimensions + the `allowed-signers-byte-identity` and
`match-strategy-default-gitdir` highest-risk-affordance rows).

| Finding # | Dimension (parity.json key) | Screen | Description | Status | Resolution |
|-----------|------------------------------|--------|--------------|--------|------------|
| 1 | (observation, not a §3 dimension) | all 7 | Same pre-existing 02-02 shell-infrastructure characteristic noted in `create-flow/CRITIQUE.md` finding #1: the TUI shell header renders only the app name + breadcrumb, not the HTML `Header.tsx`'s "N identities · ✓ healthy" context chip. Not introduced by git-screen, not fixable within this surface's own files, and not one of §3's seven MUST-match dimensions. | resolved (no action required — out of §3 scope) | Logged for awareness; not a `parity.json` row for the same reason `create-flow`'s finding #1 was not one. |
| 2 | safety-affordances-presence / layout only | `confirm-write` | The TUI's `confirm-write` screen shows the new fragment file's contents as condensed `field=value` lines wrapped in sentinels, rather than the HTML mockup's full multi-line INI block (with blank-line section separators) wrapped in sentinels. Driven by the fixed 80x24 PTY viewport (20 available body rows, `model.go`'s `verticalMargin=4`) needing to fit THREE files' previews on one screen — more content than any single create-flow screen ever showed. | resolved | §3 explicitly permits this ("MAY differ: exact spacing, pixel layout... provided the terminal skin keeps them close"; "Widget mechanics ... as long as the option set and default match"). All the same fields, values, and both sentinel markers are present in both media — only the line-wrapping/spacing of the fragment preview differs. The ~/.gitconfig include block (the file with genuinely pre-existing content to preserve, T-02-CONT's higher-value case) is shown as the exact full sentineled block text in both media, unabridged. |

No other divergences found. Every §3 dimension (field set/order,
labels/copy, option sets, defaults, flow order, safety affordances,
keybindings) and the surface's highest-risk affordances
(allowed_signers byte-identity to user.email, GITUI-04; match-strategy
default gitdir, GITUI-03) matched between media on direct screenshot-pair
review — see `parity.json` for the per-dimension resolution notes.

**0 open findings** — `.planning/design/git-screen/parity.json` has no row
with `status != "resolved"` (verified:
`python3 -c "import json; r=json.load(open('.planning/design/git-screen/parity.json')); assert r and all(x['status']=='resolved' for x in r)"`).
