# FIELDS.md template — per-surface semantic field-parity manifest

**How to use this template:** copy this file to `.planning/design/<surface>/FIELDS.md`
and fill one `## <surface> / <screen>` section per named state in that surface's
02-UX-DIRECTION.md §4 manifest (lift the state names verbatim). Author this file
BEFORE building either mockup — it doubles as the mockup's own spec, satisfying
DLV-01's "encodes layout, field order, labels, copy, flow." `agent-ui-ux-designer`
(or a human reviewer) fills the **HTML present** / **TUI present** columns AFTER
both screenshots exist.

**Gate contract (human companion to the machine gate):** this document is the
human-readable field-level record; the machine-checkable gate that actually blocks
the phase is `.planning/design/<surface>/parity.json` (copied from
`PARITY.template.json`, filled per §3 dimension). A row here with mismatched
HTML/TUI presence is an open finding in `CRITIQUE.md` until resolved — either by
fixing the divergent mockup or by an explicit, documented "this field is HTML-only /
TUI-only by design" note in the **Notes** column.

---

## Surface: `<surface-name>`

<!-- One ## heading per named state from 02-UX-DIRECTION.md §4, e.g.: -->
<!-- ## create-flow / ssh-form-filled -->

## `<surface>` / `<screen>`

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `field_key` | "Exact label copy" | 1st | ✓ / ✗ | ✓ / ✗ | |
| 2 | `field_key` | "Exact label copy" | 2nd | ✓ / ✗ | ✓ / ✗ | |

<!-- Repeat the ## <surface> / <screen> + table block for every named state. -->

---

## Example (filled, from 02-UX-DIRECTION.md §4(1) create-identity flow)

## create-flow / ssh-form-filled

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | alias_prefix | "Alias prefix" | 1st | ✓ | ✓ | |
| 2 | ssh_host | "SSH Host" | 2nd | ✓ | ✓ | auto-joined, editable both media |
| 3 | real_hostname | "Real hostname" | 3rd | ✓ | ✓ | |
| 4 | port | "Port" | 4th, default 443 | ✓ | ✓ | |
