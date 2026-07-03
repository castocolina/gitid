# CRITIQUE.md — health (Phase 8 fan-out surface)

**Reviewer note:** the executor's toolset in this session was limited to
`Read`/`Write`/`Edit`/`Bash` — no `Task`/subagent-dispatch tool was available
to spawn a fresh-context `agent-ui-ux-designer` subagent (same limitation
recorded in `create-flow/CRITIQUE.md` (02-04), `git-screen/CRITIQUE.md`
(02-05), `identity-manager/CRITIQUE.md` (02-06), `global-ssh/CRITIQUE.md`
(02-07), and `global-git/CRITIQUE.md` (02-08)). In its place, this critique
applies `agent-ui-ux-designer`'s documented methodology
(F-pattern/left-side bias, Fitts's/Hick's Law, accessibility,
distinctive-not-generic typography) directly against the captured
`.planning/design/health/html/*.png` and `.planning/design/health/tui/*.png`
screenshots (viewed and compared side by side, all 10 images), and fills
the structured parity findings log against `FIELDS.md` and `parity.json`.
**This does not substitute for a fresh-context `agent-ui-ux-designer`
pass** — flagging explicitly so the phase-level `/gsd-code-review` and the
external cross-vendor review can re-run one if the orchestrator has that
capability this session lacked.

---

## Surface: `health`

## A. Aesthetic / usability pass (HTML mockup only)

- **Reviewer:** this executor, applying `agent-ui-ux-designer`'s documented
  methodology directly (Task/subagent dispatch unavailable — see note above)
- **Screenshots reviewed:** all 5 `.planning/design/health/html/*.png`
- **Findings:**
  - **F-pattern / left-side bias:** `health-with-findings` uses the
    master-detail archetype (§2) — the two stacked SSH/Git finding lists
    flush-left, the highlighted finding's full explanation to the right;
    the 4 single-column screens (`health-all-green`, `finding-detail`,
    `per-identity-health`, `parse-error`) read top-to-bottom, left-aligned.
    No finding.
  - **Fitts's Law (target size/reachability):** each finding row on
    `health-with-findings` is a full-width `ListItemButton` with generous
    padding — no undersized targets. The highlighted `IdentitiesOnly`
    contradiction carries a subtle background highlight consistent with
    `global-ssh`'s `options-list` selection precedent. No finding.
  - **Hick's Law (choice count):** `health-with-findings` presents exactly
    5 findings split into two clearly-labeled sections (3 SSH, 2 Git),
    each section internally severity-sorted (critical → error → warning
    → info) — this ordering reduces scan cost versus an unordered mixed
    list and matches `internal/doctor/doctor.go`'s own severity urgency
    order. No finding.
  - **Accessibility / never-color-alone:** every finding row and every
    detail screen pairs a glyph (`✗`/`!`/`~`/`✓`) with the severity WORD
    ("critical"/"error"/"warning"/"info"/"healthy") — verified legible
    with color mentally removed. `health-all-green`'s success state uses
    the MUI `success` `Alert` (green, `✓`); `parse-error`'s failure state
    uses the MUI `error` `Alert` (red, `✗`) — the severity is never
    color-only. No finding.
  - **Distinctive-not-generic typography:** same terminal-skin theme as
    create-flow/git-screen/identity-manager/global-ssh/global-git
    (monospace JetBrains Mono, flat borders, zero elevation) — consistent
    visual language across all six now-built surfaces, no surface-specific
    styling drift. No finding.
  - **Read-only-integrity visual weight (the surface's own highest-risk
    affordance, §4.6):** the `healthReadOnlyNote` banner uses MUI's
    NEUTRAL `info` `Alert` severity (blue), not `warning` or `error` — a
    deliberate choice: the read-only statement is a STRUCTURAL fact about
    the surface, not itself a risk to flag. Every one of the 5 screens
    carries it in the SAME position (directly under the heading), and NO
    screen anywhere on this surface uses `success`/`warning`/`error`
    styling for a CONFIRM/APPLY/WRITE affordance, because none exists —
    verified by direct screenshot review AND
    `TestHealth_NoWriteCeremonyMarkerAnywhere` (Go). This is the correct
    visual hierarchy for a diagnose-only surface. No finding.
  - **Minor observation (not a parity finding — HTML-only cosmetic):**
    `health-with-findings.png`'s two-section list (5 rows total) plus the
    right-pane preview both fit comfortably within the 1280px capture
    viewport with no clipping (unlike `identity-manager/list-populated`'s
    8-row list, which that surface's own CRITIQUE flagged as slightly
    exceeding the fold) — no finding to log here, noted only for
    contrast.

## B. Structured parity findings log (HTML ↔ TUI, every named state)

Reviewed all 5 `html/*.png` against their `tui/*.png` counterpart,
cross-referenced against `FIELDS.md` and `parity.json`'s nine rows (the
seven §3 dimensions + the `ssh-git-two-section` and `read-only-integrity`
rows).

| Finding # | Dimension (parity.json key) | Screen | Description | Status | Resolution |
|-----------|------------------------------|--------|--------------|--------|------------|
| 1 | (observation, not a §3 dimension) | all 5 | Same pre-existing 02-02 shell-infrastructure characteristic noted in every prior surface's own CRITIQUE finding #1: the TUI shell's status line is a single static empty region (`shell.go`'s `renderShellStatus` always returns `""`), so the live-nav status line never shows the per-screen wording the `/mui` `StatusLine` component does (e.g. "5 findings across SSH and Git — severity-sorted. Diagnosis only."). Not introduced by health, not fixable within this surface's own files (`shell.go` is shared, out of scope per fan-out isolation), and not one of §3's seven MUST-match dimensions. | resolved (no action required — out of §3 scope) | Logged for awareness; not a `parity.json` row, for the same reason the prior five surfaces' finding #1 were not rows. Related, but distinct region: the header CONTEXT CHIP (a different region from this status line) was **FIXED** by the 02-review-fixes pass (finding A2) — `renderShellHeader` now shows "8 identities · ! needs action" on every surface including health. The status-line gap this row describes remains open and unaffected. |
| 2 | field-set-and-order / labels-and-helper-copy-verbatim | `health-with-findings` | The live gitid-dummy binary renders inside a REAL, fixed 80×24 PTY with no scroll region (unlike the static `RenderScreen()`→`freeze` capture path, which has no height limit). The original design considered a per-finding one-liner-plus-detail-preview render (matching the HTML's right-pane preview); verified empirically that the ONE-LINE-per-finding compaction (glyph+word + title + `[family]`) is required to fit the SSH section (3 findings) + Git section (2 findings) + banner + header/status/keybar chrome inside 24 rows — confirmed via `TestDummyNavReachesAllScreens/health/health-with-findings` passing on the FIRST attempt with this compaction (no iteration needed, unlike `global-ssh`'s `options-list` and `global-git`'s `fix-preview`/`confirm-write`, which both required a post-hoc fix after an initial overflow). The right-pane "Preview" block is TUI-omitted on `health-with-findings` specifically, replaced by a one-line keybar hint naming the highlighted finding's title. | resolved | Same class of accepted divergence `global-ssh`'s `options-list` and `global-git`'s `fix-preview`/`confirm-write` already established — §3 explicitly allows "exact spacing, pixel layout... provided the terminal skin keeps them close" and "MAY differ" cases include widget/layout compaction as long as the option SET/ORDER/VALUES match. The field set (all 5 findings' severity, title, and family) is unchanged in both media — only the master-detail right-pane preview is TUI-omitted on this ONE screen; the SAME full explanation is one keystroke (`v`) away on `finding-detail` in both media. |
| 3 | keybindings-surfaced / label wording only | `health-with-findings`, `health-all-green` | Same shared-infrastructure characteristic as every prior surface's own keybar label-wording divergence (`shell.go`'s `renderShellKeybar` renders each intra-flow key's label as the literal TARGET SCREEN ID, e.g. "h health-all-green", "v finding-detail", "i per-identity-health", "x parse-error", while the HTML `Keybar` entries use a semantic action phrase, e.g. "All-green example", "View full detail", "Per-identity health", "Parse-error example"). Not introduced by health, `shell.go` is out of scope for a fan-out surface to edit. | resolved | The KEY itself (the letter) is identical in both media on every screen, and every key's DESTINATION is unambiguous from context (the breadcrumb after pressing it, or — on `health-with-findings` — the target finding's title embedded directly in the TUI's own hint line, e.g. "v full detail (IdentitiesOnly no contradicts an explicit IdentityFile)"). §3 permits "Widget mechanics ... as long as the option set and default match" — label WORDING differing while the key and destination match is the same class of allowed medium difference already accepted for every prior fan-out surface. |

No other divergences found. Every §3 dimension (field set/order,
labels/copy, option sets, defaults, flow order, safety affordances,
keybindings) and the surface's two additional highest-risk-affordance rows
(`ssh-git-two-section`: HLTH-01's SSH/Git split holds on every screen that
presents a health snapshot, and is explicitly named on the two
single-finding deep-dives; `read-only-integrity`: the explicit
`healthReadOnlyNote` banner appears on all 5 screens in both media, and NO
confirm/backup/apply write-ceremony marker appears anywhere — verified by
direct screenshot-pair review AND
`TestHealth_ReadOnlyIntegrityBannerOnEveryScreen` /
`TestHealth_NoWriteCeremonyMarkerAnywhere` (Go)) matched between media on
direct screenshot-pair review — see `parity.json` for the per-dimension
resolution notes.

**0 open findings** — `.planning/design/health/parity.json` has no row
with `status != "resolved"` (verified:
`python3 -c "import json; r=json.load(open('.planning/design/health/parity.json')); assert r and all(x['status']=='resolved' for x in r)"`).
