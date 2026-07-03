# gitid design mockup (`.planning/design/mockup-src/`)

**This is a living design doc, not a shipped web UI.** Per `REQUIREMENTS.md`'s
"Out of Scope" section, the HTML/MUI mockup is a design-review artifact only — the
shipped `gitid` product is the terminal app (`cmd/gitid`, `tui/`). This workspace
exists to lock content and flow (fields, labels, copy, order, states) BEFORE any
Go/TUI code is written for a surface (DLV-01), and to give the HTML↔TUI parity
review (02-UX-DIRECTION.md §3) something concrete to compare against.

It is a standalone `pnpm` workspace, **not** a member of the Go module — it has its
own `package.json`/`pnpm-lock.yaml` and is never built or tested by `make build`,
`make test`, or `make lint`.

## Build

```bash
pnpm install --frozen-lockfile
pnpm build
```

This produces a static `dist/` directory. `dist/index.html` is loadable directly
via `file://` with **zero network requests** — no CDN, no remote fonts, no dev
server. It is the target `internal/screenshot`'s go-rod capture opens for the
design screenshot set.

Never run a bare `pnpm install` in any CI/make step — only
`pnpm install --frozen-lockfile`. The committed `pnpm-lock.yaml` — not registry
drift — is the source of truth for what gets installed.

## Supply-chain: automated-verified, not a second human checkpoint

This is the **first Node.js/npm toolchain in this all-Go repository**
(`internal/*` and `cmd/*` remain 100% Go; this workspace never appears in
`go.mod`). Mirroring Phase 1 Plan 05 Task 1's decision to replace a human
supply-chain gate with AUTOMATED verification (so the milestone's single human
checkpoint stays reserved for design approval — DLV-08), the 11 npm dependencies
below are pinned exactly, committed via `pnpm-lock.yaml`, and installed only from
that frozen lockfile. This adds **no second human checkpoint**.

All 11 verdicts below were re-confirmed live this session via
`slopcheck scan --pkg npm <name> --json` (all returned `"status": "OK"`, no
`flags`) — not just carried over from 02-RESEARCH.md's prior audit of the original
10 (which pre-dates the 11th, `@fontsource/jetbrains-mono`, added by review
MEDIUM-6).

| Package | Pinned version | Provenance | slopcheck verdict |
|---|---|---|---|
| `react` | 19.2.7 | github.com/facebook/react | `[OK]` |
| `react-dom` | 19.2.7 | github.com/facebook/react | `[OK]` |
| `@mui/material` | 7.3.11 (NOT `@latest` → resolves to 9.x) | github.com/mui/material-ui | `[OK]` |
| `@mui/icons-material` | 7.3.11 | github.com/mui/material-ui | `[OK]` |
| `@emotion/react` | 11.14.0 | github.com/emotion-js/emotion | `[OK]` |
| `@emotion/styled` | 11.14.1 | github.com/emotion-js/emotion | `[OK]` |
| `react-router-dom` | 7.18.1 | github.com/remix-run/react-router | `[OK]` |
| `vite` | 8.1.3 | github.com/vitejs/vite | `[OK]` |
| `@vitejs/plugin-react` | 6.0.3 | github.com/vitejs/vite-plugin-react | `[OK]` |
| `typescript` | 6.0.3 | github.com/microsoft/TypeScript | `[OK]` |
| `@fontsource/jetbrains-mono` | 5.2.8 | github.com/fontsource/fontsource (verified on npmjs.com/package/@fontsource/jetbrains-mono; self-hosts the JetBrains Mono OFL-licensed typeface, no Google Fonts CDN call) | `[OK]` |

`@types/react` (19.2.17) and `@types/react-dom` (19.2.3) are dev-only type
definitions from the same audited `react`/`react-dom` provenance and are not
separately re-audited.

**Fail-clean install behavior (registry immutability is NOT assumed):**
`pnpm install --frozen-lockfile` fails hard — with an actionable error naming the
offending package — if any pinned version in `pnpm-lock.yaml` is unavailable in
the execution environment (e.g., unpublished, yanked, or a registry mirror gap).
If this happens:

1. Do **not** fall back to a bare `pnpm install` (that would let the resolver
   silently pick a different version).
2. Do **not** silently bump the pin to whatever the registry offers instead.
3. Re-run the Package Legitimacy Gate (see `02-RESEARCH.md` § Package Legitimacy
   Audit) for the affected package, choose a new exact pin, and re-commit
   `pnpm-lock.yaml`.

## Terminal skin

The MUI theme (`src/theme.ts`) is deliberately constrained so the mockup reads as
*a screenshot of a terminal*, not a generic Material dashboard: monospace font
(self-hosted JetBrains Mono), `shape.borderRadius: 0`, all shadows `'none'`,
`transitions.duration.*` at `0` (deterministic screenshot capture), and an
ANSI-safe semantic color palette. See `02-UX-DIRECTION.md` §0-§2.

## Route auto-discovery

`src/App.tsx` uses `import.meta.glob('./routes/**/*.route.tsx', { eager: true })`
to discover screen routes — adding a new surface means adding a `*.route.tsx`
file under `src/routes/`, never editing `App.tsx`. `scripts/verify-routes.mjs` is
a build-time gate (wired into `pnpm build`) that fails on duplicate `path`s or a
malformed route module shape, so a bad or colliding route fails loudly at build
time rather than silently at capture time.
