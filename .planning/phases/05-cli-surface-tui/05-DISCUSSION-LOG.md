# Phase 5: CLI Surface + TUI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-12
**Phase:** 5-CLI Surface + TUI
**Areas discussed:** TUI scope & depth, Command surface (SC-4), Dashboard launch behavior, Navigation & key model

---

## TUI scope & depth

### Q1 — How deep should the TUI go on mutating operations?

| Option | Description | Selected |
|--------|-------------|----------|
| Hybrid | In-app forms for add/edit; heavy flows (keygen + two-phase test + backup + confirm + re-test) driven by the SAME core via identity.Deps, surfaced as TUI steps | ✓ |
| Navigator + delegate | Dashboard + read-only viewer; all mutations drop to the CLI (does not satisfy SC-2 literally) | |
| Full in-app CRUD | Every operation has a complete native TUI form/flow (largest scope) | |

**User's choice:** Hybrid
**Notes:** One core, two front-ends; no forked logic.

### Q2 — Which mutating operations get an in-app TUI form in the MVP? (multiSelect)

| Option | Description | Selected |
|--------|-------------|----------|
| Create identity | Full create flow in-app, driven by identity.Create | ✓ |
| Update fields | Lighter `identity update` flow in-app | ✓ |
| Add-account / alias | IDENT-06 add-account flow in-app | ✓ |
| Copy pubkey | On-demand clipboard copy from detail view (CLIP-02) | ✓ |

**User's choice:** Create identity, Update fields, Add-account / alias, Copy pubkey
**Notes:** Delete and Rotate (unselected) stay CLI-only; TUI hands off.

### Q3 — How should the prove-before-write flow appear inside the TUI?

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated test+confirm screen | Separate screen showing exact command + real output + backup notice + explicit confirm | |
| Inline status in the form | Run test/backup/confirm inline within the same form view | |
| You decide | Planner/UI-SPEC chooses; must show real cmd+output + explicit pre-write confirm | ✓ |

**User's choice:** You decide
**Notes:** Presentation is Claude/UI-SPEC discretion; the real-cmd+output + explicit pre-write confirm contract is non-negotiable.

---

## Command surface (SC-4)

### Q1 — How should the final command tree be shaped vs SC-4's named commands?

| Option | Description | Selected |
|--------|-------------|----------|
| Nested + top-level aliases | Keep `identity ...` canonical; add top-level `gitid rotate`/`copy`/`host add` aliases | ✓ |
| Strictly nested | No new top-level commands; SC-4 reachable via `identity ...` | |
| Flatten common ones | Promote frequent ops to primary top-level commands | |

**User's choice:** Nested + top-level aliases
**Notes:** Best discoverability; SC-4 literal names present.

### Q2 — How should `copy` (CLIP-02) and upload (UP-02) be exposed?

| Option | Description | Selected |
|--------|-------------|----------|
| copy cmd + reuse upload | `copy <name>` copies pubkey + prints existing upload instructions (reuse uploadInstructions()) | ✓ |
| Separate copy and upload | Distinct `gitid copy` (clipboard) and `gitid upload` (steps) commands | |
| copy only | Just `copy <name>`; leave upload where it surfaces today | |

**User's choice:** copy cmd + reuse upload
**Notes:** One command covers CLIP-02 + UP-02 on demand.

### Q3 — How to reconcile SC-4's `host add` name with today's `identity add` mode 3?

| Option | Description | Selected |
|--------|-------------|----------|
| Add `host add` alias | Expose `gitid host add` as alias to the add-account/alias flow | ✓ |
| Keep as identity add mode | Treat SC-4 `host add` as satisfied by `identity add` add-account | |
| You decide | Planner picks cleanest naming | |

**User's choice:** Add `host add` alias
**Notes:** Matches SC-4 wording without forking logic.

---

## Dashboard launch behavior

### Q1 — When should the six check families run on TUI launch?

| Option | Description | Selected |
|--------|-------------|----------|
| Live on launch + refresh key | Run all families immediately with a loading state; `r` re-runs | |
| Lazy / on-demand | Open empty/cached; run only on trigger | |
| Async / progressive | Launch instantly; stream each family's result in as it completes | ✓ |

**User's choice:** Async / progressive
**Notes:** Drives Bubble Tea architecture — per-family tea.Cmd, partial render on Msg arrival.

### Q2 — How should the dashboard render the doctor findings?

| Option | Description | Selected |
|--------|-------------|----------|
| TUI-native view | lipgloss-styled view consuming the same doctor.Run structured findings | ✓ |
| Embed CLI text output | Run the cmd-layer renderer, show text in a viewport | |
| You decide | Planner/UI-SPEC chooses, consuming structured findings | |

**User's choice:** TUI-native view
**Notes:** Keeps findings individually selectable; reuses Phase 4 D-06 UI-agnostic model.

### Q3 — Apply doctor auto-fixes in the TUI, or keep fixes CLI-only?

| Option | Description | Selected |
|--------|-------------|----------|
| Show findings, hand off to CLI | Dashboard shows findings + fix; applying routes to `gitid doctor --fix` | ✓ |
| In-app per-finding apply | Per-finding confirm + apply in the TUI, reusing the Phase 4 fixer | |
| You decide | Planner chooses based on fixer-seam reuse | |

**User's choice:** Show findings, hand off to CLI
**Notes:** Consistent with keeping Delete/Rotate CLI-only — blast-radius mutations on one audited path.

---

## Navigation & key model

### Q1 — What screen flow should the TUI use?

| Option | Description | Selected |
|--------|-------------|----------|
| Drill-down stack | Dashboard home; Enter drills in, Esc pops back up the stack | ✓ |
| Tabbed + drill-down | Top-level tabs (Dashboard/Identities/Baseline) + drill-down | |
| You decide | Planner/UI-SPEC chooses | |

**User's choice:** Drill-down stack
**Notes:** One focused screen at a time; simplest mental model.

### Q2 — What keybinding style?

| Option | Description | Selected |
|--------|-------------|----------|
| Arrows + vim + global keys | Arrows + j/k/h/l + q/Esc/Enter/?/r | ✓ |
| Arrows + global only | Arrows + global map, no vim | |
| You decide | Planner/UI-SPEC defines keymap | |

**User's choice:** Arrows + vim + global keys
**Notes:** Familiar to terminal devs; bubbles supports both natively; visible help bar.

### Q3 — Handle the visual contract (layout/colors/styling) how?

| Option | Description | Selected |
|--------|-------------|----------|
| Run /gsd-ui-phase 5 after | Capture behavior here; produce UI-SPEC.md via /gsd-ui-phase 5 before planning | ✓ |
| Capture visuals inline now | Decide styling here, skip a separate UI-SPEC | |
| You decide | Workflow recommends at the end | |

**User's choice:** Run /gsd-ui-phase 5 after
**Notes:** Consistent with phases 03.1 and 04; keeps this discussion behavioral.

---

## Claude's Discretion

- Prove-before-write presentation (dedicated screen vs inline) — must show real cmd+output + explicit pre-write confirm.
- Exact command/flag naming and help copy for new aliases and `copy`.
- Bubble Tea model decomposition; how async per-family commands are structured; how identity.Deps seams surface to in-app forms.
- Whether to add a Baseline tab/section to the dashboard later (out of MVP scope).

## Deferred Ideas

- In-TUI Delete and Rotate forms — CLI-only for the MVP; revisit later.
- In-TUI doctor auto-fix (per-finding apply reusing the Phase 4 fixer) — deferred; MVP hands off to `gitid doctor --fix`.
- Baseline management in the TUI (Baseline tab over `baseline setup/show`) — not in MVP form set.
- PowerShell completion — out of scope for v1 (macOS + Linux only).
