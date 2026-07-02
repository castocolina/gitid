# Phase 1: Foundations, Spikes & CI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-02
**Phase:** 1-Foundations, Spikes & CI
**Areas discussed:** Screenshot tooling, Keygen + probing scope, Include'd SSH layout, CI matrix + gate depth

> **Session note:** The user selected all four gray areas (multiSelect), then stepped
> away before the first per-area question was answered (>60s no response). Per harness
> guidance, Claude proceeded with best-judgment recommended defaults grounded in
> `recipes/`, the existing Go substrate, and the Charm/Go ecosystem. All four areas are
> therefore recorded as **Claude's discretion (recommended)** pending user confirmation.

---

## Area selection (answered)

| Option | Description | Selected |
|--------|-------------|----------|
| Screenshot tooling | TUI + HTML capture mechanism, storage (TOOL-05/DLV-03) | ✓ |
| Keygen + probing scope | Which algos generate + probe depth (KEY/PLAT) | ✓ |
| Include'd SSH layout | Single vs per-identity file; adopt/migrate (STORE) | ✓ |
| CI matrix + gate depth | Runners + native-vs-cross gate depth (BUILD) | ✓ |

**User's choice:** All four.
**Notes:** User stepped away immediately after; per-area questions answered by Claude.

---

## Screenshot tooling

| Option | Description | Selected |
|--------|-------------|----------|
| View()-dump → freeze PNG | Deterministic View() capture (teatest-style) → charmbracelet/freeze PNG + txt golden | ✓ (recommended) |
| Real PTY + terminal snapshot | Drive real binary in PTY, screenshot emulator — higher fidelity, flaky/slow | |
| Text golden only | Skip PNG — fails DLV-03 image requirement | |

**User's choice:** Not answered — Claude recommended View()-dump → freeze PNG.
**Notes:** HTML capture = scripted headless Chromium via a `make` target (loop-callable),
not the Playwright MCP (agent-only). Artifacts under `.planning/design/<surface>/{html,tui}/`.

---

## Keygen + probing scope

| Option | Description | Selected |
|--------|-------------|----------|
| ed25519 + rsa-4096 only, registry for the rest | KEY-02 floor; -sk/ecdsa additive without redesign | ✓ (locked + recommended) |
| Attempt all 5 generators in Phase 1 | Over-scopes; -sk needs hardware | |

**User's choice:** Not answered — Claude recommended ed25519+rsa-4096 generators, all-5
catalog entries, probe = `ssh-keygen -Q` + `ssh -V` + libfido2 + agent + keychain, surfaced
by a debug/list command behind an injectable seam.
**Notes:** KEY-01 catalog ordering/copy deferred to Phase 2 design per REQUIREMENTS.

---

## Include'd SSH layout

| Option | Description | Selected |
|--------|-------------|----------|
| Single gitid-owned file + glob Include near top | `~/.ssh/config.d/gitid.config`, parity with in-file block model | ✓ (recommended) |
| Per-identity files | `config.d/<identity>.config` — sprawl, ordering complexity | |

**User's choice:** Not answered — Claude recommended the single-file layout with
`Include ~/.ssh/config.d/*.config` near the top; adopt via `internal/adopter`; reversible
backed-up migration proven by round-trip + real `ssh -G`.
**Notes:** Include paths absolute or `~/.ssh`-relative only (verified); glob leaves room
for per-identity files later.

---

## CI matrix + gate depth

| Option | Description | Selected |
|--------|-------------|----------|
| 3 native runners, full gates on all | ubuntu + macos-13 Intel + macos-14 ARM; test-race/lint/e2e native | ✓ (recommended) |
| Cross-compile + gate on a subset | Cheaper, but misses PLAT divergences BUILD-02 exists to catch | |

**User's choice:** Not answered — Claude recommended 3 native runners with full gates;
cost lever noted (gate e2e to push/main only if PR minutes bite). Build matrix
cross-compiles all targets; linux/arm64 build-only.
**Notes:** Release/tag publishing (BUILD-03) is Phase 10.

---

## Claude's Discretion

All four areas were resolved at Claude's discretion (user away). Recommended-default
decisions tagged **◆** in CONTEXT.md: D-01, D-02, D-03, D-07, D-08, D-09, D-12, D-13.
User should confirm or override before `/gsd-plan-phase 1`.

## Deferred Ideas

- Real-PTY visual capture (DLV-06 e2e already covers real-binary driving).
- Per-identity Include files (`config.d/<identity>.config`).
- ecdsa-p256 / ed25519-sk / ecdsa-sk real generators (catalog lists them; additive later).
- Native-gated `linux/arm64` CI (Phase 1 = build-only cross-compile).
- KEY-01 final catalog ordering + copy (Phase 2 design).
- Visual-regression diff engine (DLV-04, Phase 3).
