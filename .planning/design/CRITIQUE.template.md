# CRITIQUE.md template — per-surface findings log

**How to use this template:** copy this file to `.planning/design/<surface>/CRITIQUE.md`.
`agent-ui-ux-designer` fills the aesthetic section from the HTML screenshots, then both
`agent-ui-ux-designer` and this plan's mockup author fill the parity findings log
against `.planning/design/<surface>/FIELDS.md` and `.planning/design/<surface>/parity.json`
(copied from `PARITY.template.json`). DLV-02: `agent-ui-ux-designer` fills the
critique/parity, and the `/mui`-built mockup is the HTML side of every parity row.

**Gate contract:** the parity gate that actually blocks phase completion is
`.planning/design/<surface>/parity.json` — it PASSES only when **no row has
`status != "resolved"`**. This file is the human-readable narrative companion; a
finding logged here must have a matching row in `parity.json` before it can be
closed as resolved.

**Scope split** (per 02-RESEARCH.md § HTML↔TUI Parity Review): the aesthetic pass
below applies to the HTML mockup ONLY (Material HTML and a monospace terminal are
different media — no aesthetic equivalent on the TUI side). The structured parity
findings log applies to BOTH media, scoped to 02-UX-DIRECTION.md §3's semantic
MUST-match dimensions.

---

## Surface: `<surface-name>`

## A. Aesthetic / usability pass (HTML mockup only)

`agent-ui-ux-designer`'s full research-backed methodology (F-pattern, left-side
bias, Fitts's/Hick's Law, accessibility, distinctive-not-generic typography)
applied to this surface's HTML screenshots.

- **Reviewer:** agent-ui-ux-designer
- **Screenshots reviewed:** `.planning/design/<surface>/html/*.png`
- **Findings:**
  - (none yet)

## B. Structured parity findings log (HTML ↔ TUI, every named state)

One entry per divergence found while filling `.planning/design/<surface>/parity.json`.
Cross-reference the `dimension` key from `parity.json` and the field row from
`FIELDS.md` where applicable.

| Finding # | Dimension (parity.json key) | Screen | Description | Status | Resolution |
|-----------|------------------------------|--------|--------------|--------|------------|
| 1 | field-set-and-order | `<screen>` | (example) | open / resolved | |

**0 open findings required** before this surface's parity gate can close
(`.planning/design/<surface>/parity.json` must have no row with
`status != "resolved"`).
