# Phase 2: DESIGN — All Mockups (★ CHECKPOINT #1) - Research

**Researched:** 2026-07-02
**Domain:** Design-first UI delivery — React/MUI v7 static mockups, Bubble Tea v2 navigation-only TUI dummy, dual screenshot capture (reusing Phase 1 tooling), structured HTML↔TUI parity review, PTY-driven navigation proof, single human approval checkpoint.
**Confidence:** MEDIUM-HIGH (Go/TUI substrate and screenshot-tooling contract: HIGH, verified against the actual codebase and Phase 1 plans; MUI/npm ecosystem versions: HIGH, verified against the npm registry directly; HTML↔TUI parity methodology and approval mechanics: MEDIUM, these are process designs without a single authoritative external source — treated as prescriptive recommendations)

## Summary

Phase 2 has zero backend logic and two deliverables per surface: a static HTML/MUI-v7
mockup (built with Vite, screenshotted with Phase 1's `go-rod` pipeline) and a
navigation-only Go TUI "dummy" (a Bubble Tea v2 program with hardcoded screen data,
screenshotted with Phase 1's `freeze` pipeline). Both artifact sets already have their
capture *tooling* built in Phase 1 (`make screenshot-tui` / `make screenshot-html`,
`internal/screenshot/{tui,html,determinism}.go`) — Phase 2 does not rebuild that
tooling, it **extends** it: parameterizes the existing capture functions over N screens
per surface instead of one fixture, and adds a surface enumeration loop.

The two hardest design problems are (1) how to compare an HTML mockup against a
monospace TUI screenshot when they are visually incommensurable media, and (2) how to
prove "no backend logic" is not just a promise but an enforced, checkable property of
the TUI dummy. For (1), the recommendation is: do **not** pixel-diff HTML against TUI;
instead require a structured field-parity manifest per screen (same fields, same order,
same labels/copy, same primary actions) that `agent-ui-ux-designer` and the plan's
review step both check against, with visual side-by-side screenshot review as a
secondary/aesthetic check layered on top. For (2), the recommendation is a **physically
separate Go binary** (`cmd/gitid-dummy`, distinct from the shipped `cmd/gitid`) whose
package import graph is asserted (via `go list -deps`) to contain none of the backend
packages (`internal/identity`, `internal/keygen`, `internal/sshconfig`,
`internal/gitconfig`, `internal/filewriter`, `internal/tester`, `internal/doctor`,
`internal/adopter`, `internal/uploader`) — this is a grep/CI-enforceable proof, not a
promise, and gives DLV-05 an automated signal.

The existing (pre-Phase-1-execution) POC codebase already contains two directly
reusable, **verified** patterns this phase should copy rather than reinvent: a raw-byte
PTY-driving e2e harness (`e2e/ui_pty_e2e_test.go`, using `creack/pty` +
`charmbracelet/x/vt`, both already in `go.mod`) for the dummy-nav PTY proof, and a
manual line-compositing modal-overlay algorithm (`tui/overlay.go`) that exists
specifically because `lipgloss v2.0.3` has **no** `PlaceOverlay` function (confirmed via
`go doc charm.land/lipgloss/v2 PlaceOverlay` in that file's header comment) — any modal
screens in the TUI dummy (clone / new-key / delete-choice) will hit this same gap and
need the same fallback, reimplemented backend-free inside the dummy's own package.

**Primary recommendation:** Build the MUI mockup as a Vite + React 19 + TypeScript +
MUI v7.3.11 SPA under `.planning/design/mockup-src/` (pnpm workspace, `base: './'`,
`HashRouter`, zero Google-Fonts-CDN network dependency) with one route per screen;
build the TUI dummy as a new `internal/dummytui` package + `cmd/gitid-dummy` binary
(Bubble Tea v2, hardcoded static data, screen enum + key-routing `Update`, no backend
imports, import-graph-checked); extend Phase 1's `internal/screenshot` capture
functions to iterate over all screens of both; add a new `//go:build e2e` PTY test
(`e2e/dummy_nav_e2e_test.go`) mirroring `ui_pty_e2e_test.go`'s harness exactly, driving
the real `gitid-dummy` binary; and gate the whole phase on a `.planning/design/APPROVAL.md`
marker (a standard GSD `checkpoint:human-verify` task) that later phases' DLV-04
visual-regression gate diffs against.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| HTML/MUI mockup rendering | Dev/build tooling (Vite static SPA, design-review artifact only) | — | Explicitly Out of Scope in REQUIREMENTS.md ("Shippable Web UI" — living design doc, never shipped) |
| HTML mockup screenshot capture | Dev/build tooling (Makefile + go-rod, extends Phase 1) | — | Same headless-Chromium driver Phase 1 built; capture happens outside the shipped binary |
| TUI dummy navigation model | UI-free-of-backend TUI tier (`internal/dummytui`, `cmd/gitid-dummy`) | — | A real Bubble Tea v2 `tea.Model`, but with zero business-logic imports — a distinct package from the real product `tui/` |
| TUI dummy screenshot capture | Dev/build tooling (Makefile + freeze, extends Phase 1) | — | Reuses the `View()`-dump → freeze → PNG path Phase 1 built |
| Dummy-nav PTY proof | Dev/test tooling (`e2e/` package, `//go:build e2e`) | — | Drives the real `gitid-dummy` binary via raw keystrokes; proves navigability before human presentation, distinct from DLV-06's backend e2e (Phase 3+) |
| HTML↔TUI parity check | Design-review artifact (per-screen manifest + `agent-ui-ux-designer` critique) | — | Not a runtime capability — a review-process artifact consumed by DLV-02/DLV-04 |
| Approval record | GSD workflow checkpoint (`.planning/design/APPROVAL.md` + `checkpoint:human-verify`) | — | The single hard stop (DLV-08); gates Phase 3+ backend work |

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DLV-01 | Every UI-bearing phase produces an HTML mockup (React + `/mui`) BEFORE Go/TUI code; encodes layout, field order, labels, copy, flow | § MUI Mockup Harness, § Surface Inventory — Vite+React+MUI v7 SPA structure, per-screen route/manifest pattern |
| DLV-02 | `agent-ui-ux-designer` AND `/mui` skill engaged on every UI task (plan/execute/review) | § HTML↔TUI Parity Review — critique methodology (aesthetic layer + structured parity layer) |
| DLV-05 | Per-surface order: HTML mockup → screenshots → Go TUI dummy (full nav, no backend) → screenshots → user approval → backend. Backend never written before dummy approval | § Go TUI Dummy, § Don't Hand-Roll — separate binary + import-graph enforcement of "no backend" |
| DLV-08 | Single human checkpoint: design approval (HTML + TUI screenshots); credential upload auto-runs elsewhere, not a checkpoint | § Approval Mechanics — `.planning/design/APPROVAL.md` + GSD `checkpoint:human-verify` |
| (uses) DLV-03 | Screenshot pipeline (built Phase 1) | § Dual Screenshot Capture — extending, not rebuilding, `internal/screenshot` |
| (adds) — | Dummy-nav PTY e2e (user's explicit ask, distinct from DLV-06) | § Dummy-Nav PTY E2E — reuse `e2e/ui_pty_e2e_test.go` harness verbatim pattern |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|---------------|
| `react` | 19.2.7 | UI runtime for the mockup SPA | `[VERIFIED: npm registry]` — latest per `registry.npmjs.org/react` dist-tags; MUI v7 peer-deps accept `^19.0.0` |
| `react-dom` | 19.2.7 | DOM renderer | `[VERIFIED: npm registry]` — matches `react` |
| `@mui/material` | **7.3.11** (NOT `9.1.2`) | Component library, `/mui` skill target | `[VERIFIED: npm registry]` — this is the **last release in the 7.x line**; MUI's own numbering **skipped v8 and jumped straight to v9** (confirmed: `7.3.11` published 2026-05-07, `9.1.2` published 2026-06-23, no `8.x` versions exist on the registry). The project's `/mui` skill and this milestone's explicit brief both target v7 conventions (slots/slotProps, sx prop). **Never install `@latest`** — it resolves to v9, a different major with its own breaking changes not covered by the `/mui` skill. |
| `@mui/icons-material` | 7.3.11 | Icon set | `[VERIFIED: npm registry]` — pin to the SAME major/minor as `@mui/material` (mismatched majors between the two packages is a common MUI breakage) |
| `@emotion/react` | 11.14.0 | MUI's default styling engine (peer dep) | `[VERIFIED: npm registry]` — `@mui/material@7.3.11`'s `peerDependencies` require `^11.5.0`, satisfied |
| `@emotion/styled` | 11.14.1 | MUI's `styled()` API (peer dep) | `[VERIFIED: npm registry]` — peer-dep requires `^11.3.0`, satisfied |
| `react-router-dom` | 7.18.1 | Client-side routing, one route per mockup screen | `[VERIFIED: npm registry]` — use **`HashRouter`**, not `BrowserRouter` (see Pitfall: file:// + BrowserRouter) |
| `vite` | 8.1.3 | Build tool / static bundler | `[VERIFIED: npm registry]` — latest stable; no compatibility constraint pins an older major here |
| `@vitejs/plugin-react` | 6.0.3 | Vite's React JSX/Fast-Refresh plugin | `[VERIFIED: npm registry]` |
| `typescript` | 6.0.3 | Static typing for the mockup source | `[VERIFIED: npm registry]` — MUI v7's peer-deps require TS ≥4.9; 6.x is well above that floor |
| `@types/react` | 19.2.17 | React type defs | `[VERIFIED: npm registry]` |
| `@types/react-dom` | 19.2.3 | React-DOM type defs | `[VERIFIED: npm registry]` |
| `pnpm` | 11.5.3 (already installed on the dev machine) | Package manager | `[VERIFIED: local `pnpm --version`]` — CLAUDE.md explicitly bans `npm`: "install pre-commit via uv... never brew" convention extends to this project's tooling note that `npm` is aliased-blocked on this machine (`tools-installer: npm is banned — use pnpm`). Use `pnpm` for ALL installs in this phase. |

### Supporting (Go side — no new Go dependencies)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `charm.land/bubbletea/v2` | v2.0.7 (already in `go.mod`) | Elm-architecture event loop for the TUI dummy | Already pinned project-wide (CLAUDE.md stack table); the dummy is a NEW `tea.Model`, not new tooling |
| `charm.land/lipgloss/v2` | v2.0.3 (already in `go.mod`) | TUI styling for the dummy's static screens | Already pinned; **note the `PlaceOverlay` gap below** |
| `charm.land/bubbles/v2` | v2.1.0 (already in `go.mod`) | List/textinput/etc. components for hardcoded-data screens | Already pinned |
| `github.com/creack/pty` | v1.1.24 (already in `go.mod`) | Raw-keystroke PTY driving for the dummy-nav e2e | `[VERIFIED: go.mod + e2e/ui_pty_e2e_test.go]` — already used by the existing POC's `e2e/ui_pty_e2e_test.go`; reuse verbatim |
| `github.com/charmbracelet/x/vt` | v0.0.0-20260621… (already in `go.mod`) | Terminal emulator decoding PTY output into text frames for assertions | `[VERIFIED: go.mod + e2e/ui_pty_e2e_test.go]` — same file, same pattern |
| `internal/screenshot` (Phase 1) | n/a (in-repo) | `tui.go` (freeze wrapper), `html.go` (go-rod wrapper), `determinism.go` (metadata-strip + SHA-256) | Phase 2 EXTENDS these, does not reimplement — see § Dual Screenshot Capture |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `HashRouter` (react-router-dom) | `BrowserRouter` + a local static file server | `BrowserRouter` relies on the History API and a server that resolves deep paths to `index.html`; loading the built `dist/index.html` directly via `file://` (no server) 404s/blanks on any route past `/`. A local Go `httptest.Server`/`http.FileServer` would also work but adds a server-lifecycle concern to the capture test for no benefit — `HashRouter` needs zero infrastructure. |
| `@mui/material` v7.3.11 (pinned) | `@mui/material@latest` (currently resolves to v9.1.2) | v9 has its own breaking changes beyond the v6→v7 migration the `/mui` skill documents; installing `@latest` silently drifts the mockup off the skill's guidance. Pin exactly. |
| Vite static build (`base: './'`) | A dev-server (`vite dev`) kept running during capture | A running dev server is a live process go-rod would need to manage/wait-for/tear-down around every capture run — more moving parts, less deterministic for CI. A static `dist/` build loaded via `file://` has no server lifecycle at all. |
| A single combined screenshot-capture test per surface | One test per screen (7 surfaces × ~3-5 screens each) | Per-surface batching (one driving test enumerating that surface's screen/route list) balances CI runtime against the "concrete runnable entry point" pattern Phase 1 established (`TestCaptureHTML`/`TestCaptureTUI` per surface, not per screen) |

**Installation:**
```bash
# From .planning/design/mockup-src/ (a pnpm workspace, NOT part of the Go module):
pnpm init
pnpm add react@19.2.7 react-dom@19.2.7 \
  @mui/material@7.3.11 @mui/icons-material@7.3.11 \
  @emotion/react@11.14.0 @emotion/styled@11.14.1 \
  react-router-dom@7.18.1
pnpm add -D vite@8.1.3 @vitejs/plugin-react@6.0.3 typescript@6.0.3 \
  @types/react@19.2.17 @types/react-dom@19.2.3

# Go side: no new dependencies. cmd/gitid-dummy and internal/dummytui import ONLY
# charm.land/bubbletea/v2, charm.land/lipgloss/v2, charm.land/bubbles/v2 — already
# in go.mod, no `go get` needed.
```

**Version verification:** All npm versions above were confirmed live against
`registry.npmjs.org` (`curl` + dist-tags/time fields) during this research session, not
from training-data memory of "MUI v7" — this caught the v8-skip-to-v9 trap that stale
training knowledge would have missed. Re-verify immediately before install if this
research is more than ~2 weeks old (npm churns faster than the 30-day validity window
below implies).

## Package Legitimacy Audit

All ten new npm packages were checked with `slopcheck scan --pkg npm <name> --json`
(installed via `pip install slopcheck` this session). All Go-side dependencies are
already present in `go.mod` from the existing POC substrate — no new Go packages are
introduced by this phase.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `react` | npm | Long-established (Meta) | Very high | github.com/facebook/react | `[OK]` | Approved |
| `react-dom` | npm | Long-established (Meta) | Very high | github.com/facebook/react | `[OK]` | Approved |
| `@mui/material` | npm | Established (MUI org) | Very high | github.com/mui/material-ui | `[OK]` | Approved (pin 7.3.11, not `@latest`) |
| `@mui/icons-material` | npm | Established (MUI org) | Very high | github.com/mui/material-ui | `[OK]` | Approved (pin 7.3.11) |
| `@emotion/react` | npm | Established | High | github.com/emotion-js/emotion | `[OK]` | Approved |
| `@emotion/styled` | npm | Established | High | github.com/emotion-js/emotion | `[OK]` | Approved |
| `react-router-dom` | npm | Established | Very high | github.com/remix-run/react-router | `[OK]` | Approved |
| `vite` | npm | Established (Evan You / VoidZero) | Very high | github.com/vitejs/vite | `[OK]` | Approved |
| `@vitejs/plugin-react` | npm | Established | High | github.com/vitejs/vite-plugin-react | `[OK]` | Approved |
| `typescript` | npm | Established (Microsoft) | Very high | github.com/microsoft/TypeScript | `[OK]` | Approved |

**Packages removed due to slopcheck `[SLOP]` verdict:** none
**Packages flagged as suspicious `[SUS]`:** none

**Package-name provenance note:** package names above were sourced from the project
brief / `/mui` skill / training knowledge, then independently confirmed to exist and be
current via the live npm registry (not just `slopcheck`, which alone would not catch a
slopsquat). Per the provenance rule, name-correctness for `react-router-dom`,
`@vitejs/plugin-react`, and `typescript` (not independently cross-referenced against an
official doc page in this session, only registry + slopcheck) remain `[ASSUMED]`-adjacent
for name-correctness even though they passed both checks — these are extremely
well-known packages, so risk is low, but the planner should still let `pnpm install`
run against the committed lockfile in CI as the final gate rather than treating this
audit as sufficient on its own.

**New supply-chain surface note:** this is the **first time** a Node.js/npm toolchain
enters this all-Go repository. Recommend: commit `.planning/design/mockup-src/package.json`
and `pnpm-lock.yaml` (the mockup is a "living design doc," REQUIREMENTS.md Out of Scope
section — versioned like any other design artifact), gitignore `node_modules/`, and add
`pnpm install --frozen-lockfile` (never a bare `pnpm install` in CI/setup-env) so the
lockfile — not registry drift — is the source of truth for what gets installed.

## Architecture Patterns

### System Architecture Diagram

```
 ┌────────────────────────────┐        ┌─────────────────────────────────┐
 │  .planning/design/         │        │  internal/dummytui (NEW)          │
 │  mockup-src/  (pnpm, Vite) │        │  + cmd/gitid-dummy (NEW binary)   │
 │  React 19 + MUI v7 SPA     │        │  Bubble Tea v2, hardcoded data,   │
 │  one route per screen      │        │  screen-enum + key-routing Update │
 │  (HashRouter, base:'./')   │        │  ZERO backend imports             │
 └──────────────┬─────────────┘        └───────────────┬────────────────┘
                │ `vite build` → dist/                  │ `go build`
                ▼                                        ▼
 ┌────────────────────────────┐        ┌─────────────────────────────────┐
 │ internal/screenshot/html.go │        │ internal/screenshot/tui.go        │
 │ (Phase 1, EXTENDED)         │        │ (Phase 1, EXTENDED)               │
 │ go-rod → file://dist/…#/rt  │        │ View() string → freeze → PNG      │
 │ per (surface,screen) tuple  │        │ per (surface,screen) tuple        │
 └──────────────┬─────────────┘        └───────────────┬────────────────┘
                │                                        │
                ▼                                        ▼
     .planning/design/<surface>/html/*.png    .planning/design/<surface>/tui/*.png
                │                                        │
                └───────────────────┬────────────────────┘
                                     ▼
                     agent-ui-ux-designer CRITIQUE
                     (aesthetic pass on HTML  +
                      structured field-parity matrix
                      HTML screen ↔ TUI screen)
                                     │
                                     ▼
                     .planning/design/<surface>/CRITIQUE.md
                     (0 unresolved findings required)
                                     │
                 ┌───────────────────┴────────────────────┐
                 ▼                                          ▼
     e2e/dummy_nav_e2e_test.go (NEW, //go:build e2e)   Human reviews all
     drives REAL gitid-dummy binary via raw PTY         screenshots + parity
     keystrokes (creack/pty + x/vt, reused verbatim     matrices
     from e2e/ui_pty_e2e_test.go) — proves every
     screen reachable BEFORE presenting to the user
                 │                                          │
                 └───────────────────┬────────────────────┘
                                     ▼
                     .planning/design/APPROVAL.md
                     ★ single human checkpoint (DLV-08)
                     — gates ALL Phase 3+ backend work
```

### Recommended Project Structure

```
.planning/design/
├── mockup-src/                    # pnpm workspace, NOT a Go module member
│   ├── package.json                # pinned versions (§ Standard Stack)
│   ├── pnpm-lock.yaml              # committed — frozen-lockfile installs only
│   ├── vite.config.ts              # base: './' (file:// compatibility)
│   ├── src/
│   │   ├── theme.ts                 # MUI theme: transitions.create → 0ms (determinism)
│   │   ├── routes/                  # one file per screen, one route per screen
│   │   │   ├── create-flow/         # algorithm-catalog, ssh-screen, test-screen, review
│   │   │   ├── git-screen/          # git-fields, match-strategy, review
│   │   │   ├── identity-manager/    # list, detail, clone, new-key, delete-choice
│   │   │   ├── global-ssh/
│   │   │   ├── global-git/
│   │   │   ├── health/
│   │   │   └── fixer/
│   │   └── App.tsx                  # HashRouter + route table
│   └── dist/                        # `vite build` output — go-rod's file:// target
├── <surface>/
│   ├── html/*.png                   # captured screenshots (DLV-03 layout, unchanged)
│   ├── tui/*.png
│   ├── FIELDS.md                    # per-screen field/order/label/copy manifest (both media)
│   └── CRITIQUE.md                  # agent-ui-ux-designer findings, must reach 0 open
├── fonts/                            # Phase 1 vendored TTF (TUI) — reused, not duplicated
└── APPROVAL.md                       # DLV-08 checkpoint record

internal/dummytui/                    # NEW — Bubble Tea v2 model, hardcoded data only
├── screens.go                        # screen enum (7 surfaces × N screens each)
├── model.go                          # tea.Model: Update routes on screen enum + keys
├── overlay.go                        # re-implemented placeOverlay (see Pitfall — no backend import of tui/)
├── data.go                           # hardcoded fixture data per screen (catalog entries, identity rows, …)
└── *_test.go

cmd/gitid-dummy/                      # NEW — separate main package, NEVER in `make build`
└── main.go                           # tea.NewProgram(dummytui.NewModel()).Run()

e2e/
└── dummy_nav_e2e_test.go             # NEW, //go:build e2e — mirrors ui_pty_e2e_test.go's harness
```

### Pattern 1: Extend, don't rebuild, Phase 1's screenshot capture functions

**What:** Phase 1's `internal/screenshot/html.go` and `tui.go` (build-tag `screenshot`)
already implement the pinned-Chromium-revision / vendored-font / fixed-geometry /
metadata-stripped / golden-hashed capture path against ONE fixture each. Phase 2 adds a
thin iteration layer — NOT a second capture implementation — that calls the same
underlying functions once per `(surface, screen)` tuple.
**When to use:** Any time Phase 2 needs a new screenshot. Never duplicate the go-rod
launcher config or the freeze exec-wrapper; import and call Phase 1's functions.
**Example:**
```go
// internal/screenshot/design_capture_test.go (NEW, //go:build screenshot)
// Source: pattern derived from Phase 1's TestCaptureHTML/TestCaptureTUI shape
// (.planning/phases/01-foundations-spikes-ci/01-05-PLAN.md Task 2/3)
var htmlScreens = []struct{ surface, screen, route string }{
    {"create-flow", "algorithm-catalog", "/create/algorithm"},
    {"create-flow", "ssh-screen", "/create/ssh"},
    {"create-flow", "test-screen", "/create/test"},
    // ... one row per screen across all 7 surfaces
}

func TestCaptureAllMockupScreens(t *testing.T) {
    distIndex := filepath.Join(repoRoot(t), ".planning/design/mockup-src/dist/index.html")
    for _, s := range htmlScreens {
        url := "file://" + distIndex + "#" + s.route
        out := filepath.Join(repoRoot(t), ".planning/design", s.surface, "html", s.screen+".png")
        // captureHTML is the SAME Phase-1 function, called per-URL/out-path
        if err := captureHTML(url, out); err != nil {
            t.Fatalf("%s/%s: %v", s.surface, s.screen, err)
        }
    }
}
```

### Pattern 2: Import-graph-enforced "no backend logic" (DLV-05)

**What:** `go list -deps` is used as a compile-time-adjacent, CI-checkable proof that
`internal/dummytui` and `cmd/gitid-dummy` never import any backend package — turning
"the dummy has no backend logic" from a promise into a grep-checkable acceptance
criterion, mirroring exactly how Phase 1 proves `freeze`/`go-rod` never enter the
shipped `cmd/gitid` binary's dependency graph.
**When to use:** As the primary DLV-05 acceptance check on every dummy-tui plan/task.
**Example:**
```bash
# Source: pattern mirrors Phase 1's `go list -deps ./cmd/gitid | grep -q charmbracelet/freeze`
# check (01-05-PLAN.md Task 2 acceptance_criteria) — same technique, opposite direction.
for pkg in identity keygen sshconfig gitconfig filewriter tester doctor adopter uploader; do
  if go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/... | grep -q "internal/${pkg}"; then
    echo "FAIL: dummy imports backend package internal/${pkg}"
    exit 1
  fi
done
```

### Pattern 3: HashRouter + relative base for file:// mockup capture

**What:** Vite's default `base: '/'` emits absolute asset paths (`/assets/index-xxx.js`)
that a browser cannot resolve against a `file://` origin (no root to resolve `/` against
except the OS filesystem root). `react-router-dom`'s `BrowserRouter` similarly depends
on a server resolving any path to `index.html`; opening `dist/index.html#/create/ssh`
directly has no such server. Both are fixed by treating the mockup as a
"portable/offline" build.
**When to use:** Any Vite SPA that must be screenshotted via `file://` rather than served.
**Example:**
```typescript
// vite.config.ts
// Source: Vite docs — base config option (CITED: vite.dev/config/shared-options.html#base)
export default defineConfig({
  base: './',   // emits ./assets/… so file://…/dist/index.html loads its own JS/CSS
  plugins: [react()],
});
```
```typescript
// src/App.tsx
import { HashRouter, Routes, Route } from 'react-router-dom';
// HashRouter needs no server: file://…/index.html#/create/ssh works directly.
```

### Anti-Patterns to Avoid

- **Pixel-diffing HTML against TUI screenshots:** they are different media (raster
  RGBA web render vs. monospace character grid); a naive pixel/SSIM diff will report
  100% divergence on every screen regardless of actual field/flow parity. Use the
  structured field-parity manifest instead (§ HTML↔TUI Parity Review).
- **Letting the TUI dummy import the real `tui/` package "just for the overlay helper":**
  `tui/` (the POC's real product package) transitively imports `internal/doctor`,
  `internal/identity`, etc. via `tui/deps.go` — importing ANY symbol from `tui/` would
  break the DLV-05 "no backend logic" import-graph check. Reimplement the small,
  backend-free `placeOverlay` compositing function inside `internal/dummytui` instead.
- **Running a live `vite dev` server during capture:** adds process-lifecycle
  management (start, wait-for-ready, teardown, port collision) to what should be a
  deterministic, static capture step. Build once (`vite build`), capture from the
  static `dist/`.
- **Installing `@mui/material@latest`:** resolves to v9.1.2 (a different major), not
  v7. Always pin `7.3.11` explicitly.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| ANSI terminal output → PNG | A custom ANSI parser + rasterizer | `charmbracelet/freeze` (Phase 1, already pinned `@v0.2.2`) | Purpose-built, already spiked and proven in this exact repo (real 640×332 PNG produced this session per 01-RESEARCH.md) |
| Headless-browser screenshot | Shelling out to a system `chromium --headless` binary manually | `go-rod` (Phase 1, already pinned `v0.116.2`) with its pinned-revision launcher | Already solves reproducible-Chromium-provisioning (BUILD-04) via its own launcher; a hand-rolled `exec.Command("chromium", ...)` would need to reinvent that provisioning |
| Raw-keystroke PTY driving + terminal decoding | A custom pty wrapper + ANSI state machine | `creack/pty` + `charmbracelet/x/vt` (already in `go.mod`, already proven in `e2e/ui_pty_e2e_test.go`) | This exact repo already solved the goroutine-ownership/deadlock hazards (single-owner `emu`, response-drainer goroutine) — reuse the harness, don't re-derive it |
| Modal/overlay compositing in the TUI dummy | A new overlay library or a naive string-concat overlay | The `placeOverlay` line-replacement algorithm pattern from `tui/overlay.go` (reimplemented, backend-free, inside `internal/dummytui`) | `lipgloss v2.0.3` genuinely has no `PlaceOverlay` (verified via `go doc` in that file) — this repo already did the research and built the fallback; copying the *algorithm* (not the package import) avoids re-deriving ANSI-width-safe line compositing |
| MUI transition/animation timing control for deterministic screenshots | Custom CSS overrides scattered per-component | A single theme-level override: `theme.transitions.create` return `0ms` durations, or MUI's `theme.transitions.duration.{shortest,...} = 0` | MUI already exposes a global transitions config; overriding it once in `theme.ts` is simpler and more complete than chasing every `Fade`/`Grow`/`Collapse` component individually |

**Key insight:** every hard problem this phase touches (ANSI→PNG, headless-Chromium
screenshot, PTY-driven terminal automation, TUI modal compositing) was **already solved
in this exact repository** — either by the not-yet-executed Phase 1 plan or by the
existing (pre-reset) POC codebase still on disk. The research risk in this phase is not
"which library" but "don't accidentally rebuild something that already has a proven,
gotcha-documented implementation two directories away."

## Common Pitfalls

### Pitfall 1: `lipgloss v2.0.3` has no `PlaceOverlay`
**What goes wrong:** Code written assuming `lipgloss.PlaceOverlay(x, y, fg, bg)` exists
(it exists in some lipgloss v1 forks/examples found via web search) fails to compile.
**Why it happens:** lipgloss v2's public API dropped/never shipped `PlaceOverlay`;
only `Place`, `PlaceHorizontal`, `PlaceVertical` exist at v2.0.3.
**How to avoid:** `[VERIFIED: codebase]` — confirmed via `go doc charm.land/lipgloss/v2
PlaceOverlay` (documented in `tui/overlay.go`'s header, returns "no symbol
PlaceOverlay"). Any modal screen in the TUI dummy (clone/new-key/delete-choice
overlays in the Identity Manager surface) needs the manual line-replacement
compositing algorithm — reimplement it inside `internal/dummytui`, not by importing
`tui/`.
**Warning signs:** A compile error on `lipgloss.PlaceOverlay` during dummy-tui
development.

### Pitfall 2: Vite's default `base: '/'` breaks under `file://`
**What goes wrong:** `vite build` with default config emits `<script src="/assets/index-xxxx.js">`;
opening `dist/index.html` via `file://` cannot resolve a root-absolute path against the
filesystem root, so the page loads blank with a console 404.
**Why it happens:** Vite assumes the build is deployed to a web server root by default.
**How to avoid:** Set `base: './'` in `vite.config.ts` (§ Pattern 3).
**Warning signs:** go-rod's HTML capture produces a screenshot of a blank white page.

### Pitfall 3: `BrowserRouter` 404s/blanks on any non-root route under `file://`
**What goes wrong:** Same root cause class as Pitfall 2 — `BrowserRouter` needs a
server that rewrites any deep path to `index.html`; there is none under `file://`.
**How to avoid:** Use `HashRouter` — the route lives in the URL fragment
(`index.html#/create/ssh`), which the browser resolves client-side with no server.
**Warning signs:** Screenshots of every screen look identical (all showing the
default/first route).

### Pitfall 4: `@mui/material@latest` silently resolves to v9, not v7
**What goes wrong:** A bare `pnpm add @mui/material` (no version pin) installs
`9.1.2` — different breaking changes than the v6→v7 migration the `/mui` skill
documents (v8 was skipped entirely).
**Why it happens:** MUI's own release numbering jumped v7→v9 to align with MUI X
versioning (confirmed via registry `time` field: no `8.x` versions exist).
**How to avoid:** Always pin `@mui/material@7.3.11` and `@mui/icons-material@7.3.11`
explicitly in `package.json`; never `@latest` or a caret range crossing a major.
**Warning signs:** Components using deep imports or v6-era APIs the `/mui` skill
doesn't mention start failing — a symptom of accidentally being on v9.

### Pitfall 5: MUI transitions make screenshot capture non-deterministic
**What goes wrong:** `Dialog`, `Snackbar`, `Collapse`, `Fade`, `Grow` all animate by
default (CSS transitions with real durations); if go-rod screenshots mid-transition,
two runs of the same capture can produce different pixel content even with identical
input, breaking the golden-hash reproducibility Phase 1 established for TUI.
**Why it happens:** MUI's default theme ships non-zero transition durations.
**How to avoid:** Override `theme.transitions.create` (or all
`theme.transitions.duration.*` values) to `0` in the mockup's theme, and/or use
go-rod's `MustWaitStable()` before every screenshot call (Phase 1 already used a
context timeout + wait-for-load pattern for the fixture — extend it with a stability
wait, not just a load wait).
**Warning signs:** Re-running `make screenshot-html-mockups` twice produces different
SHA-256 hashes for the same screen with no source change.

### Pitfall 6: Google Fonts CDN link breaks offline/CI-sandboxed capture
**What goes wrong:** `agent-ui-ux-designer`'s own default guidance (`ui-ux-designer.md`)
recommends `<link href="https://fonts.googleapis.com/...">` for distinctive typography.
A network-dependent font load is a determinism/CI-offline risk analogous to the exact
problem Phase 1's D-02 (vendored TUI font) and pinned-Chromium-revision (fail-fast, no
silent fallback) decisions were built to avoid on the TUI side.
**Why it happens:** The design-critic agent's built-in advice is generically web-first,
not aware of this project's offline/deterministic-capture constraint.
**How to avoid:** Bundle the chosen font locally via an `@fontsource/*` npm package
(self-hosted, no network call at capture time) instead of a Google Fonts `<link>`.
**Warning signs:** Screenshots captured with no network access render with a fallback
system font instead of the intended typeface; CI runs differ from local runs.

### Pitfall 7: `go-rod`'s pinned-Chromium-revision cache from Phase 1 is per-fixture, not per-surface
**What goes wrong:** If Phase 2's capture code re-derives its own launcher/cache-path
config instead of calling Phase 1's `internal/screenshot/html.go` function, it risks
using a DIFFERENT (unpinned) Chromium revision or cache path — silently reintroducing
the non-determinism Phase 1's T-01-SC2 threat mitigation specifically closed.
**How to avoid:** Import and call the Phase 1 function; never re-instantiate a
`launcher.Launcher` config directly in Phase 2 code (§ Pattern 1).
**Warning signs:** `go list -deps` (or a code review) reveals a second `launcher.New()`
call site outside `internal/screenshot/html.go`.

### Pitfall 8: PTY e2e and screenshot capture are DIFFERENT TUI-rendering paths — don't conflate them
**What goes wrong:** Assuming the PTY e2e's raw-terminal-decoded frames (via `x/vt`) are
also what gets screenshotted for the approval set. Phase 1's D-01 explicitly chose
`View()`-dump → freeze for screenshots (deterministic, no real terminal timing) and
reserved real-PTY driving for e2e assertions only.
**How to avoid:** Keep the two paths separate: `internal/dummytui`'s `View()` states
feed `freeze` for screenshots; the SAME dummy binary, run under a real PTY, feeds the
`e2e/dummy_nav_e2e_test.go` navigability proof. Neither path's artifacts substitute for
the other.
**Warning signs:** A plan task tries to derive PNG screenshots FROM the PTY e2e's
captured frames (e.g., `x/vt`'s `emu.String()` text) instead of from `View()` + freeze.

## Code Examples

### Verified: existing PTY e2e harness (reuse verbatim for dummy-nav proof)
```go
// Source: e2e/ui_pty_e2e_test.go (existing POC codebase, read this session)
// Reuse exactly: startPTY(cmd), s.sendKey(raw, delay), s.waitFor(timeout, predicate),
// s.snapshot(), s.close(t). Only the binary built and the keystroke script change.
func TestDummyNavReachesAllScreens(t *testing.T) {
    bin := BuildDummyBinary(t) // NEW helper, mirrors BuildBinary() in harness_test.go
                                // but `go build -o bin ./cmd/gitid-dummy`
    home := SandboxHome(t)
    cmd := exec.Command(bin) //nolint:gosec // arg-slice, binary from BuildDummyBinary
    cmd.Env = append(os.Environ(), "HOME="+home)
    s := startPTY(t, cmd)
    defer s.close(t)

    // Number-key view switching already exists in the real tui/model.go convention
    // (case "1"/"2"/"3" — SHELL-02's palette+number-key routing); dummy mirrors it.
    s.sendKey([]byte("2"), 100*time.Millisecond) // -> Global SSH options view
    if _, ok := s.waitFor(2*time.Second, func(txt string) bool {
        return strings.Contains(txt, "StrictHostKeyChecking")
    }); !ok {
        t.Fatal("dummy nav: Global SSH options screen not reached on '2'")
    }
    saveFrame(t, "global-ssh-reached", s)
}
```

### Verified: real, empirically-run `freeze` invocation (Phase 1, reused unchanged)
```bash
# Source: run this session per 01-RESEARCH.md § Code Examples (macOS, Darwin 23.6.0)
go build -o /tmp/freezebin github.com/charmbracelet/freeze
/tmp/freezebin --execute "cat /tmp/sample.txt" -o /tmp/sample.png
# WROTE  /tmp/sample.png — "PNG image data, 640 x 332, 8-bit/color RGBA, non-interlaced"
```

### go-rod capture call shape (CITED: go-rod API surface, MEDIUM confidence)
```go
// Source: pkg.go.dev/github.com/go-rod/rod (CITED, not independently re-verified via
// Context7 this session — go-rod's method names MustConnect/MustPage/MustWaitStable/
// MustScreenshot are stable, widely-documented public API as of v0.116.x)
browser := rod.New().ControlURL(launcherURL).MustConnect()
page := browser.MustPage(fileURL) // "file:///.../dist/index.html#/create/ssh"
page.MustWaitStable()             // let MUI transitions settle (Pitfall 5)
page.MustScreenshot(outPath)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|-------------------|---------------|--------|
| `@mui/material` v6 deep imports (`@mui/material/Button/Button`) | Package-exports-field-only imports | v7 (March 2025) | Any mockup code copy-pasted from pre-v7 MUI examples with deep imports will fail to resolve |
| MUI v7 as "latest" | MUI v9 is latest (v8 skipped) | v9 released before 2026-06-23 (confirmed via registry) | Training-data assumptions of "v7 = current" are now ~2 majors stale; this phase must pin explicitly, not rely on `@latest` or general knowledge |
| `onBackdropClick` on `Modal` | `onClose` (with a `reason` param) | v7 | Any delete-confirmation/clone modal in the mockup must use `onClose`, not the removed prop |

**Deprecated/outdated:**
- MUI v6-and-earlier deep-import paths — removed in v7, will hard-fail the build if
  copied from stale examples/training data.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|----------------|
| A1 | The HTML↔TUI parity check should be a structured field/order/label manifest rather than a pixel/SSIM diff | § HTML↔TUI Parity Review (below), Anti-Patterns | If the planner instead expects a visual pixel-diff tool, plans built on this recommendation would need rework; this is a process design, not a verified external standard — flagged for discuss-phase confirmation |
| A2 | A separate `cmd/gitid-dummy` binary (not a build-tag on `cmd/gitid`) is the right isolation boundary for "no backend logic" | § Go TUI Dummy, Pattern 2 | If the team prefers a single binary with a `--dummy` flag instead, the import-graph check would need a different scoping (e.g., a build tag rather than a separate `go list` target); functionally equivalent but changes plan file-list |
| A3 | `.planning/design/APPROVAL.md` + a GSD `checkpoint:human-verify` task is the right approval-recording mechanism | § Approval Mechanics | This project's actual GSD checkpoint primitive/mechanics were not independently verified against `gsd-core` workflow docs in this session — the planner should confirm the exact checkpoint task syntax against `$HOME/.claude/gsd-core/` conventions rather than trusting this file-name/marker convention verbatim |
| A4 | react-router-dom's `HashRouter` (vs. a hand-rolled state-based screen switch with no router library at all) is worth the dependency | § Standard Stack | A simpler React `useState`-based screen switch (no router) would also satisfy DLV-01/03 and removes one dependency; `HashRouter` was chosen for URL-addressability (go-rod navigates directly to a screen via URL fragment, useful for ad hoc re-capture of a single screen) but this is a judgment call, not a hard requirement |
| A5 | Vite `base: './'` + a static `dist/` build (vs. keeping a dev server running during capture) is preferred | § Pattern 1, Pitfall 2 | If the team wants live-reload-driven mockup iteration during agent-driven design work (not just final capture), a dev-server-based flow might be preferred for that interactive phase, with static build only for the final capture pass — worth confirming with the user during discuss-phase |

**If this table is empty:** N/A — see rows above.

## Open Questions

1. **Does the Upload/Credentials surface (Phase 9) get its own Phase-2 mockup?**
   - What we know: ROADMAP.md's Phase 2 goal explicitly enumerates 7 surfaces ("create
     flow, git screen, identity manager, global SSH options, global git options,
     health, fixer"). Phase 9's own success criteria include a "UI-wave gate" (`/mui` +
     `agent-ui-ux-designer` + PTY e2e + visual-regression diff vs. approved
     screenshots) — the same gate every other UI phase has.
   - What's unclear: whether Phase 9's UI-wave gate diffs against a Phase-2-produced
     mockup (meaning Phase 2 should ship an 8th surface) or reuses an existing
     confirmation/modal pattern already covered by another surface (e.g., the
     create-flow's final confirm screen, or a manager-triggered action sharing the
     Identity Manager's modal-overlay pattern).
   - Recommendation: confirm with the user during discuss-phase; if undecided, default
     to NOT adding an 8th surface now (matches the ROADMAP's literal 7-surface
     enumeration) and let Phase 9 either mock a minimal upload-confirmation screen
     itself or explicitly reuse the Identity Manager's overlay pattern with a note in
     its own phase research.

2. **Exact screen count per surface — final flow granularity is a design decision, not a research fact.**
   - What we know: REQUIREMENTS.md derives a rough per-surface screen list (§ Surface
     Inventory below) from the SSHUI/GITUI/MGR/GSSH/GGIT/HLTH/FIX requirement text.
   - What's unclear: whether e.g. the create-flow's algorithm-catalog and SSH-field
     screen should be one screen or two, or whether Global SSH and Global Git should
     each be single-screen or split into "list + detail" — REQUIREMENTS.md explicitly
     defers "KEY-01 catalog ordering... to the design phase" and doesn't fully pin
     screen boundaries.
   - Recommendation: treat § Surface Inventory's screen list as a starting proposal
     for the design work itself (this IS the design phase), not a locked spec; the
     mockup build is where final screen boundaries get decided, screenshotted, and
     then locked by approval.

3. **`agent-ui-ux-designer`'s research-backed aesthetic critique (fonts/color/F-pattern/
   left-side-bias) doesn't map cleanly onto a monospace TUI.**
   - What we know: the agent's methodology (`ui-ux-designer.md`) is written for
     general web UI — typography choices, color gradients, hover micro-interactions —
     none of which the TUI dummy can express (16-color/256-color terminal palette,
     monospace grid, no hover states).
   - What's unclear: the exact division of labor — should the agent's aesthetic
     critique apply ONLY to the HTML mockup (with the TUI side judged solely on
     structural/field parity), or should it also weigh in on TUI-appropriate concerns
     (information density, keyboard-navigation clarity, Fitts's-Law-adjacent
     keybinding ergonomics)?
   - Recommendation: scope the agent's aesthetic critique to the HTML mockup; scope
     its role on the TUI side to the structured parity matrix (same fields/order/labels)
     plus keyboard-navigation-specific usability heuristics (recognition-over-recall
     for keybindings, discoverability of the palette). Confirm this split explicitly
     in the CRITIQUE.md template so findings aren't miscategorized.

## Surface Inventory

Seven surfaces (per ROADMAP.md Phase 2 goal), with a proposed screen/flow breakdown
derived from REQUIREMENTS.md — **not locked**, see Open Question 2:

| Surface | Owning later phase | Proposed screens/flow steps | Source requirements |
|---------|--------------------|------------------------------|----------------------|
| `create-flow` | Phase 3 | 1) Algorithm catalog (top-5, default ed25519) 2) SSH identity screen (Alias prefix → SSH Host → Real hostname → Port, live `Host` preview) 3) Two-stage connectivity test (direct, then targeted; command+output shown per stage) 4) Store/confirm | KEY-01, SSHUI-01..03, TEST-01..03 |
| `git-screen` | Phase 4 | 1) Git fields (user.name/email, gpg.format=ssh, signingkey path, commit.gpgsign) 2) Match strategy (gitdir:/hasconfig:, live `includeIf` preview) 3) Review → confirm | GITUI-01..05 |
| `identity-manager` | Phase 5 | 1) Identity list (per-row state, 8-label taxonomy) 2) Detail (SSH-first, then Git, per-identity health) 3) Clone (new name + reuse/new-key choice) 4) New-key-for-existing 5) Delete-choice (all vs. git-only) | MGR-01..08, KEY-05/07 |
| `global-ssh` | Phase 6 | 1) Options list with risk + recommended-value explanations, advisory/fixable | GSSH-01 (option list itself still open — "Still Open" in REQUIREMENTS.md) |
| `global-git` | Phase 7 | 1) Baseline settings (defaultBranch, ignorecase, autocrlf, email, recipe defaults) | GGIT-01 |
| `health` | Phase 8 | 1) Two-section (SSH / Git) health view | HLTH-01..06 |
| `fixer` | Phase 8 | 1) Two-section problems + severity + suggested fix + apply | FIX-01/02 |

The app shell's 5 primary views (SHELL-02: Identities / Global SSH / Global Git /
Health / Fixer, reachable via palette + number keys 1-5) map onto `identity-manager`,
`global-ssh`, `global-git`, `health`, and `fixer` respectively. `create-flow` and
`git-screen` are entered as a modal/wizard sequence FROM the Identity Manager (an
action, not one of the 5 numbered views) — this ordering should be reflected in both
the mockup's route structure and the dummy's screen-enum transitions.

## HTML↔TUI Parity Review (DLV-02 methodology)

Two separate, non-substitutable checks, both required before CRITIQUE.md can show 0
open findings:

1. **Aesthetic/usability pass (HTML mockup only).** `agent-ui-ux-designer` applies its
   full research-backed methodology (F-pattern, left-side bias, Fitts's/Hick's Law,
   accessibility, distinctive-not-generic typography — see its own agent file) to the
   HTML screenshots. This has no TUI equivalent; scope it to HTML only (Open Question 3).

2. **Structured field-parity matrix (HTML ↔ TUI, every screen).** For each screen,
   author a `FIELDS.md` entry BEFORE building either mockup (it doubles as the
   mockup's own spec, satisfying DLV-01's "encodes layout, field order, labels,
   copy, flow"):
   ```markdown
   ## create-flow / ssh-screen
   | # | Field | Label | Order | HTML present | TUI present | Notes |
   |---|-------|-------|-------|---------------|--------------|-------|
   | 1 | alias_prefix | "Alias prefix" | 1st | ✓ | ✓ | |
   | 2 | ssh_host | "SSH Host" | 2nd | ✓ | ✓ | auto-joined, editable both media |
   | 3 | real_hostname | "Real hostname" | 3rd | ✓ | ✓ | |
   | 4 | port | "Port" | 4th, default 443 | ✓ | ✓ | |
   ```
   `agent-ui-ux-designer` (or a human reviewer) fills the HTML-present/TUI-present
   columns AFTER both screenshots exist, and any row with a mismatch is an open
   CRITIQUE.md finding until resolved (either by fixing the divergent mockup or by an
   explicit, documented "this field is HTML-only / TUI-only by design" note).

This gives DLV-04 (Phase 3+ visual-regression gate) a concrete, versioned reference —
not just PNGs to eyeball, but a field-level contract each later phase's real TUI is
checked against.

## Approval Mechanics (DLV-08)

Recommend `.planning/design/APPROVAL.md`, written after every surface's CRITIQUE.md
shows 0 open findings, listing:
- every `.planning/design/<surface>/{html,tui}/*.png` path (the complete, final
  screenshot set — the literal "reference set for every later UI wave" DLV-08
  requires),
- the resolved CRITIQUE.md findings per surface (or a link to each),
- a closing `**APPROVED:** <date> by <user>` line.

This should be implemented as a standard GSD `checkpoint:human-verify` task in the
Phase 2 plan (the project's existing single-checkpoint mechanism per DLV-08 /
REQUIREMENTS.md "Resolved Decisions" #1) — `[ASSUMED]`, see A3: the exact GSD task
syntax for this checkpoint type was not independently re-verified against
`gsd-core` workflow references in this research session; the planner should confirm
against those before finalizing plan tasks. Enforcement of "no backend logic written
before this approval" (DLV-05's back half) is a process/plan-ordering guarantee (Phase
3's plans simply `depends_on` this phase's completion), not a runtime code check —
unlike DLV-05's "no backend logic IN THE DUMMY," which IS runtime/import-graph checked
(§ Pattern 2).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Node.js | Building the MUI mockup | ✓ | v22.22.3 (local dev machine) | — |
| pnpm | Installing mockup npm deps (CLAUDE.md forbids `npm`) | ✓ | 11.5.3 (local dev machine) | — |
| Go | TUI dummy, screenshot capture, PTY e2e | ✓ | go1.26.0 darwin/amd64 (project pins `go 1.26` in `go.mod`; CLAUDE.md notes 1.26.4 as latest patch — not currently installed, but the `go 1.26` toolchain directive is satisfied by 1.26.0) | Run `go install golang.org/dl/go1.26.4@latest && go1.26.4 download` if a patch-exact match is required by CI |
| `freeze` (Phase 1 dev tool) | TUI screenshot capture | Not yet installed (Phase 1 not executed) | pinned `@v0.2.2` per Phase 1 plan | Blocked on Phase 1 executing first (already a stated Phase 2 dependency — "Depends on: Phase 1" in ROADMAP.md) |
| Pinned-revision headless Chromium (go-rod launcher) | HTML screenshot capture | Not yet provisioned (`~/.cache/rod` absent on this machine) | pinned revision recorded in Phase 1's `GOLDENS.md` (not yet written — Phase 1 unexecuted) | Same — blocked on Phase 1 |
| `slopcheck` | Package legitimacy audit (this research) | ✓ | installed this session via `pip install slopcheck` | already used, no fallback needed |

**Missing dependencies with no fallback:**
- Phase 1's `freeze`/pinned-Chromium provisioning must exist before Phase 2's capture
  tests can run — this is already an explicit ROADMAP dependency ("Phase 2 Depends on:
  Phase 1"), not a gap introduced by this research.

**Missing dependencies with fallback:**
- Go patch version (1.26.0 vs. CLAUDE.md's noted 1.26.4) — the `go 1.26` directive in
  `go.mod` accepts either; only re-pin if CI enforces an exact patch.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` package, build-tag-scoped (`screenshot`, `e2e`) — same convention as Phase 1 |
| Config file | none (Go stdlib testing; no separate config) — mockup side has `vite.config.ts` but it is a build config, not a test-framework config |
| Quick run command | `go build ./cmd/gitid-dummy/... ./internal/dummytui/...` (compile check, seconds) |
| Full suite command | `make screenshot-html-mockups && make screenshot-tui-mockups && make dummy-nav-e2e` (NEW targets, extending Phase 1's `screenshot-tui`/`screenshot-html`/`test-e2e` pattern) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| DLV-01 | HTML mockup screenshots exist, one per enumerated screen, per surface | automated (existence + count) | `find .planning/design/*/html -name '*.png' \| wc -l` matches the FIELDS.md-derived expected count | ❌ Wave 0 — `internal/screenshot/design_capture_test.go` |
| DLV-02 | `CRITIQUE.md` per surface has 0 unresolved findings | human-approval-adjacent (grep-checkable once authored) | `! grep -rq "OPEN" .planning/design/*/CRITIQUE.md` | ❌ Wave 0 — template + per-surface files created during execution |
| DLV-05 (no backend) | `internal/dummytui`/`cmd/gitid-dummy` import graph excludes all backend packages | automated (unit-adjacent, CI-checkable) | `go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/...` grep-checked against the backend-package list (§ Pattern 2) | ❌ Wave 0 — new packages |
| DLV-05 (full nav) | Every screen reachable from the dummy's entry point via documented keystrokes | e2e-observable | `go test -tags e2e -race -timeout 60s -run TestDummyNav ./e2e/...` | ❌ Wave 0 — `e2e/dummy_nav_e2e_test.go` |
| DLV-08 | User approval recorded | human-approval (not automatable) | manual: `.planning/design/APPROVAL.md` contains `**APPROVED:**` line | ❌ Wave 0 — created at the end of the phase's execution, gated by a `checkpoint:human-verify` plan task |

### Sampling Rate
- **Per task commit:** `go build ./cmd/gitid-dummy/... ./internal/dummytui/...` (fast compile-only check) + the relevant single screenshot test (`-run TestCapture<Surface>`)
- **Per wave merge:** full `make screenshot-html-mockups && make screenshot-tui-mockups && make dummy-nav-e2e`
- **Phase gate:** full suite green AND every `CRITIQUE.md` at 0 open findings AND `APPROVAL.md` `**APPROVED:**` line present, before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `.planning/design/mockup-src/` — the entire pnpm workspace does not exist yet (package.json, vite.config.ts, src/)
- [ ] `internal/dummytui/` — new package, does not exist
- [ ] `cmd/gitid-dummy/main.go` — new binary entry point, does not exist
- [ ] `internal/screenshot/design_capture_test.go` (or per-surface equivalents) — the driving test(s) enumerating all (surface, screen) tuples for BOTH html and tui capture
- [ ] `e2e/dummy_nav_e2e_test.go` — new, mirrors `e2e/ui_pty_e2e_test.go`'s harness
- [ ] `e2e/harness_test.go`'s `BuildBinary` needs a sibling `BuildDummyBinary` (or a generalized `BuildBinaryFrom(pkgPath)`) for the new `cmd/gitid-dummy` target
- [ ] New Makefile targets: `screenshot-html-mockups`, `screenshot-tui-mockups`, `dummy-nav-e2e` (or fold the latter into `test-e2e` with a `-run` filter, per team preference)
- [ ] `.planning/design/<surface>/FIELDS.md` + `CRITIQUE.md` templates — do not exist yet, needed before any capture is meaningful for parity review

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|--------------------|
| V2 Authentication | No | Phase 2 has no auth surface — mockups and dummy are local, unauthenticated design artifacts |
| V3 Session Management | No | No sessions in a static mockup or a hardcoded-data TUI dummy |
| V4 Access Control | No | Local single-user dev tool; no access-control surface introduced |
| V5 Input Validation | Marginal | The TUI dummy's key-routing `Update` only switches on a fixed, hardcoded set of `tea.KeyMsg` values against static screen data — no user-supplied data is parsed or persisted; standard Bubble Tea v2 `Update(msg tea.Msg)` pattern is sufficient, no custom validation layer needed |
| V6 Cryptography | No | Zero cryptographic operations in this phase (KEY-* generators are Phase 1/3 concerns, out of scope here) |

### Known Threat Patterns for this phase's stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| npm/pnpm supply-chain drift (first Node.js toolchain in an all-Go repo) | Tampering | Pin exact versions (§ Standard Stack), commit `pnpm-lock.yaml`, CI/setup-env uses `pnpm install --frozen-lockfile` only, slopcheck-audited (§ Package Legitimacy Audit) |
| go-rod loading remote/untrusted content during capture | Information Disclosure / Tampering | Mockup is ALWAYS loaded via a local `file://` path to the repo's own `dist/` build — never a remote URL; mirrors Phase 1's T-01-SC2 mitigation (pinned Chromium, no silent fallback) extended to Phase 2's capture targets |
| TUI dummy accidentally touching real `~/.ssh`/`~/.gitconfig` | Tampering / Elevation of Privilege | Enforced by the SAME import-graph check that proves "no backend logic" (§ Pattern 2) — the dummy imports no `internal/filewriter`, so it has no code path capable of writing those files at all; additionally, the PTY e2e should run under a sandboxed `HOME` (`SandboxHome(t)`, already an established e2e pattern) and assert zero files created there as a belt-and-suspenders runtime check |
| Freeze/exec.Command invocation from the extended capture code | Tampering | Reuse Phase 1's arg-slice `exec.Command` + `#nosec G204` annotation convention (`internal/platform/platform.go` style, already cited in Phase 1's plan) — never build a shell string from screen/surface names |

## Sources

### Primary (HIGH confidence)
- `/Users/ramon/git/personal/ssh-git-config/CLAUDE.md` — North Star, stack table, engineering/commit rules
- `/Users/ramon/git/personal/ssh-git-config/recipes/README.md`, `recipes/ssh-config.recipe`, `recipes/gitconfig.recipe` — canonical config end-state
- `.planning/ROADMAP.md` — Phase 2 goal/success criteria, Phase 3-9 UI-wave gates, surface enumeration
- `.planning/REQUIREMENTS.md` §A DLV, §D SSHUI, §E TEST, §F STORE, §G GITUI, §H MGR, §I GSSH, §J GGIT, §K HLTH, §L FIX, §N SHELL — exact requirement text and Resolved Decisions/Still Open sections
- `.planning/phases/01-foundations-spikes-ci/01-05-PLAN.md` — screenshot tooling contract (`freeze`/`go-rod` versions, build-tag isolation, artifact layout, determinism requirements) this phase extends
- `.planning/phases/01-foundations-spikes-ci/01-CONTEXT.md` — D-01 (View()-dump not real-PTY for screenshots), D-02 (freeze choice), D-03 (go-rod via make target not Playwright MCP), D-04 (artifact layout + fixed geometry)
- `.planning/phases/01-foundations-spikes-ci/01-RESEARCH.md` — go-rod/freeze package legitimacy audit, architecture diagram, code examples (empirically-run freeze invocation)
- `.planning/phases/01-foundations-spikes-ci/01-04-PLAN.md` — 8-label identity state taxonomy (informs `identity-manager` mockup's per-row state display)
- `e2e/ui_pty_e2e_test.go`, `e2e/harness_test.go` (existing POC codebase) — the PTY e2e harness reused verbatim for the dummy-nav proof
- `tui/overlay.go` (existing POC codebase) — verified `lipgloss v2.0.3` has no `PlaceOverlay`; the fallback compositing algorithm
- `tui/model.go`, `tui/tui.go` (existing POC codebase) — number-key view-switching convention, `tea.Program` bootstrapping pattern
- `go.mod` — confirmed pinned versions of `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`, `github.com/creack/pty`, `github.com/charmbracelet/x/vt` already present
- `registry.npmjs.org` (direct `curl` queries this session) — `react`, `react-dom`, `@mui/material`, `@mui/icons-material`, `@emotion/react`, `@emotion/styled`, `react-router-dom`, `vite`, `@vitejs/plugin-react`, `typescript`, `@types/react`, `@types/react-dom` — versions and publish dates
- `slopcheck scan --pkg npm <name> --json` (this session) — all 10 new npm packages `[OK]`
- `/Users/ramon/.claude/plugins/marketplaces/agent-toolkit/skills/mui/SKILL.md` — `/mui` skill conventions (sx prop, slots/slotProps, v6→v7 breaking changes)
- `/Users/ramon/.claude/plugins/marketplaces/agent-toolkit/agents/ui-ux-designer.md` — `agent-ui-ux-designer` methodology and scope

### Secondary (MEDIUM confidence)
- WebSearch "MUI v7 latest version npm" — corroborated the v8-skip-to-v9 finding, independently confirmed via direct registry `curl`
- WebSearch "go-rod rod.Page MustNavigate MustScreenshot" — go-rod's public API method names (`MustConnect`, `MustPage`, `MustWaitStable`, `MustScreenshot`), not independently re-verified via Context7 in this session (Context7 MCP tools were not available/invoked)
- WebSearch "Vite React TypeScript scaffold" — confirmed `npm create vite@latest -- --template react-ts` as the standard scaffold command (adapted to `pnpm create vite` per this project's pnpm-only convention)

### Tertiary (LOW confidence)
- None — every claim above traces to either a live registry query, a `slopcheck` run, a file read from this repository, or a corroborated WebSearch result.

## Metadata

**Confidence breakdown:**
- Standard stack (npm versions): HIGH — every version verified directly against `registry.npmjs.org` this session, not training-data recall
- Go-side substrate (bubbletea/pty/vt, screenshot tooling contract): HIGH — verified by reading the actual `go.mod`, Phase 1 plan files, and existing POC source
- HTML↔TUI parity methodology, approval mechanics: MEDIUM — these are this research's own process design (no single authoritative external source), explicitly flagged in the Assumptions Log for discuss-phase confirmation
- go-rod exact API method names: MEDIUM — WebSearch-corroborated, not Context7-verified (Context7 was unavailable this session)

**Research date:** 2026-07-02
**Valid until:** ~14 days for the npm version pins (npm ecosystem moves fast — MUI's v7→v9 jump is itself evidence a re-check is cheap insurance); ~30 days for the Go-side/architectural findings (stable, verified against pinned `go.mod` and an unexecuted-but-locked Phase 1 plan)
