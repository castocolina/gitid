---
phase: 02-design-all-mockups-checkpoint-1
plan: 01
subsystem: ui
tags: [react, mui, vite, typescript, pnpm, design-mockup, terminal-skin]

# Dependency graph
requires:
  - phase: 01-foundations-spikes-ci
    provides: "font vendoring precedent (.planning/design/fonts/) and the automated-supply-chain-gate pattern reused here (01-05 Task 1)"
provides:
  - "Vite + React 19 + TypeScript + MUI v7.3.11 pnpm workspace at .planning/design/mockup-src/, building to a static file://-loadable dist/ with zero network requests"
  - "Terminal-skin MUI theme (monospace, flat, zero-transition) shared by every later mockup surface"
  - "The shared four-region app shell (Header/Body/StatusLine/Keybar) with a <surface>/<screen> breadcrumb screen-ID marker"
  - "Route auto-discovery (import.meta.glob) + a build-time route-uniqueness/shape gate (scripts/verify-routes.mjs)"
  - "Typed, recipe-accurate copy fixtures (src/data/recipeFixtures.ts) every surface reuses"
  - "FIELDS.template.md / CRITIQUE.template.md (human) + PARITY.template.json (machine-checkable) parity templates"
affects: [02-02, 02-03, 02-04, 02-05, 02-06, 02-07, 02-08, 02-09, 02-10]

# Tech tracking
tech-stack:
  added: ["react@19.2.7", "react-dom@19.2.7", "@mui/material@7.3.11", "@mui/icons-material@7.3.11", "@emotion/react@11.14.0", "@emotion/styled@11.14.1", "react-router-dom@7.18.1", "vite@8.1.3", "@vitejs/plugin-react@6.0.3", "typescript@6.0.3", "@fontsource/jetbrains-mono@5.2.8", "pnpm workspace (first Node.js toolchain in this Go repo)"]
  patterns: ["import.meta.glob route auto-discovery with a build-time verify-routes.mjs shape/uniqueness gate", "terminal-skin MUI theme (borderRadius 0, shadows none, transitions 0ms) for deterministic screenshot capture", "Shell/Header/StatusLine/Keybar four-region composition every surface reuses without editing shared files"]

key-files:
  created:
    - .planning/design/mockup-src/package.json
    - .planning/design/mockup-src/pnpm-lock.yaml
    - .planning/design/mockup-src/vite.config.ts
    - .planning/design/mockup-src/tsconfig.json
    - .planning/design/mockup-src/index.html
    - .planning/design/mockup-src/.gitignore
    - .planning/design/mockup-src/README.md
    - .planning/design/mockup-src/src/main.tsx
    - .planning/design/mockup-src/src/App.tsx
    - .planning/design/mockup-src/src/theme.ts
    - .planning/design/mockup-src/src/shell/Shell.tsx
    - .planning/design/mockup-src/src/shell/Header.tsx
    - .planning/design/mockup-src/src/shell/StatusLine.tsx
    - .planning/design/mockup-src/src/shell/Keybar.tsx
    - .planning/design/mockup-src/src/data/recipeFixtures.ts
    - .planning/design/mockup-src/src/routes/_shell/shell-demo.route.tsx
    - .planning/design/mockup-src/scripts/verify-routes.mjs
    - .planning/design/FIELDS.template.md
    - .planning/design/CRITIQUE.template.md
    - .planning/design/PARITY.template.json
  modified: []

key-decisions:
  - "sshIdentityAliasBlockText in recipeFixtures.ts is written as a literal (not interpolated) so recipe-critical field values (Port 443, IdentitiesOnly yes) are byte-visible in source, satisfying grep-based acceptance checks as a static contract"
  - "Header's <surface>/<screen> breadcrumb is passed as a per-route `title` prop threaded through Shell, not derived via a data-router's useMatches() — kept App.tsx a plain HashRouter+Routes tree (simpler, matches RESEARCH's HashRouter recommendation) while still giving every route a stable screen-id"
  - "verify-routes.mjs uses Node 22's built-in fs.globSync (no extra npm glob dependency) since the project's pinned toolchain (Volta) is Node 22.22.3"

requirements-completed: []  # DLV-01/DLV-02 are phase-spanning (all 12 plans, all 7 surfaces) — NOT marked complete in REQUIREMENTS.md by this foundation plan alone (1/12). This plan lays the foundation both requirements depend on; see "Decisions Made".

# Metrics
duration: 40min
completed: 2026-07-03
---

# Phase 2 Plan 01: MUI Mockup Foundation Summary

**Terminal-skinned Vite + React 19 + MUI v7.3.11 SPA foundation (shared shell, breadcrumb screen-ID, glob route auto-discovery + build-time validation, recipe-accurate typed fixtures, and the FIELDS/CRITIQUE/PARITY parity templates) that every later mockup surface builds on without editing shared files.**

## Performance

- **Duration:** ~40 min
- **Started:** 2026-07-03T03:59:00-04:00 (approx, first file write)
- **Completed:** 2026-07-03T04:08:24-04:00 (last task commit)
- **Tasks:** 3 completed
- **Files modified:** 20 created (0 modified)

## Accomplishments

- Scaffolded a pnpm workspace with all 11 npm dependencies pinned to exact versions (MUI 7.3.11, not the `@latest`-resolved 9.x), a committed `pnpm-lock.yaml`, and a documented fail-clean install policy — confirmed reproducible via a full clean-room `rm -rf node_modules dist && pnpm install --frozen-lockfile && pnpm build` re-run at the end of the session.
- Built a terminal-skin MUI v7 theme (self-hosted JetBrains Mono, `borderRadius: 0`, all 25 shadows `'none'`, `transitions.duration.*` at `0`, ripple disabled, ANSI-safe semantic palette) and the shared four-region shell (`Header`/body/`StatusLine`/`Keybar`), with the Header rendering a `<surface>/<screen>` breadcrumb screen-ID marker.
- Implemented `import.meta.glob`-based route auto-discovery in `App.tsx` with a runtime duplicate-path/malformed-shape validator, plus a standalone `scripts/verify-routes.mjs` build-time gate wired into `pnpm build` — both were empirically exercised against an injected duplicate-path route file and confirmed to fail the build (exit 1) before being reverted.
- Authored `recipeFixtures.ts`, a typed module carrying real, recipe-derived copy (SSH `Host` block, macOS Keychain globals, `includeIf hasconfig:`/`gitdir:`, `insteadOf`, per-identity git fragment with `gpg.format=ssh`, `allowed_signers` byte-identical to `user.email`, global git defaults, managed-block sentinels) — no lorem/placeholder text anywhere.
- Authored the `FIELDS.template.md` / `CRITIQUE.template.md` (human) and `PARITY.template.json` (machine-checkable, one row per 02-UX-DIRECTION.md §3 dimension plus a highest-risk-affordance placeholder) parity templates.

## Task Commits

Each task was committed atomically:

1. **Task 1: Scaffold the pinned pnpm + Vite + MUI v7 workspace** - `31cfdbc` (feat)
2. **Task 2: Terminal-skin theme + shared shell + route auto-discovery/validation** - `6d2c236` (feat)
3. **Task 3: Recipe fixtures + FIELDS/CRITIQUE/PARITY templates** - `f9eea9a` (feat)

**Supplemental (post-Task-1 fixup):** `215fc05` (docs) — re-ran `slopcheck` live against all 11 pinned packages (including `@fontsource/jetbrains-mono`, added by review MEDIUM-6 after 02-RESEARCH.md's original 10-package audit) and recorded the confirmed `[OK]` verdicts in the README, rather than relying on inference alone.

**Plan metadata:** pending (this commit, created after this SUMMARY)

## Files Created/Modified

- `.planning/design/mockup-src/package.json` - pinned dependency manifest (11 exact-pinned npm deps), `build` script chains `verify-routes.mjs` then `vite build`
- `.planning/design/mockup-src/pnpm-lock.yaml` - committed frozen lockfile
- `.planning/design/mockup-src/vite.config.ts` - `base: './'` for file:// static capture compatibility
- `.planning/design/mockup-src/tsconfig.json` - strict TS config, react-jsx
- `.planning/design/mockup-src/index.html` - root div + `src/main.tsx` module script
- `.planning/design/mockup-src/.gitignore` - excludes `node_modules/`, `dist/`
- `.planning/design/mockup-src/README.md` - living-design-doc note, build instructions, 11-package supply-chain audit table with live slopcheck verdicts, fail-clean install policy
- `.planning/design/mockup-src/src/main.tsx` - `ThemeProvider` + `CssBaseline` + self-hosted JetBrains Mono imports
- `.planning/design/mockup-src/src/App.tsx` - `HashRouter` + `import.meta.glob` route discovery + duplicate-path/shape validation
- `.planning/design/mockup-src/src/theme.ts` - terminal-skin `createTheme` (monospace, flat, zero-transition, ANSI-safe palette)
- `.planning/design/mockup-src/src/shell/Shell.tsx` - composes the four shared regions
- `.planning/design/mockup-src/src/shell/Header.tsx` - app name + `<surface>/<screen>` breadcrumb + context chip
- `.planning/design/mockup-src/src/shell/StatusLine.tsx` - transient feedback line with semantic tone
- `.planning/design/mockup-src/src/shell/Keybar.tsx` - always-visible keybindings incl. reserved keys
- `.planning/design/mockup-src/src/data/recipeFixtures.ts` - typed recipe-accurate copy for all surfaces
- `.planning/design/mockup-src/src/routes/_shell/shell-demo.route.tsx` - the only route this plan ships
- `.planning/design/mockup-src/scripts/verify-routes.mjs` - build-time route-uniqueness/shape gate
- `.planning/design/FIELDS.template.md` - per-surface field-parity manifest template
- `.planning/design/CRITIQUE.template.md` - agent-ui-ux-designer findings-log template
- `.planning/design/PARITY.template.json` - machine-checkable parity-row template (7 §3-dimension rows + 1 highest-risk-affordance placeholder)

## Decisions Made

- `sshIdentityAliasBlockText` is a literal string, not built via template interpolation from `sshIdentityAlias` — this keeps recipe-critical field text (`Port 443`, `IdentitiesOnly yes`) statically greppable in source, at the cost of a documented (comment-flagged) manual-sync requirement between the literal and the structured `sshIdentityAlias` object. A follow-up plan could add a unit test asserting they stay in sync.
- The `<surface>/<screen>` breadcrumb is threaded as a `title` prop on `<Shell>` per route (not derived from a data-router's `useMatches()`), keeping `App.tsx` a simple `HashRouter` + `Routes` tree as 02-RESEARCH.md recommended, while still satisfying the "stable screen-id both HTML capture and TUI e2e assert against" requirement.
- `scripts/verify-routes.mjs` uses Node 22's built-in `fs.globSync` rather than adding a `glob` npm dependency, since the project's pinned toolchain (Volta) is Node 22.22.3, which ships it.
- **DLV-01 and DLV-02 are NOT marked complete in `REQUIREMENTS.md`** despite this plan's frontmatter listing them: both are phase-spanning requirements ("every UI-bearing phase" / "every UI-related task" across all 7 surfaces), and this is only Wave 1's foundation plan (1 of 12 in Phase 2). Marking them complete here would falsely declare phase-wide requirements satisfied by the foundation alone. Deferred to whichever later Phase 2 plan actually closes out full coverage (likely 02-11/02-12).

## Deviations from Plan

None — plan executed exactly as written across all 3 tasks. One supplemental fixup commit (`215fc05`) strengthened Task 1's supply-chain evidence (live `slopcheck` run against all 11 packages, not just documented reasoning) but did not change any Task 1 acceptance-criteria outcome — all had already passed.

## Issues Encountered

- **Code-review-skill tool unavailable in this executor's environment.** The plan's `<success_criteria>` requires "a fresh-context code review via the `superpowers:requesting-code-review` skill" before the plan is marked complete. This executor's toolset in this session was limited to `Read`/`Write`/`Edit`/`Bash` — no `Task`/`Skill`-invocation tool was available to spawn a fresh-context reviewer subagent. In its place, this executor performed an exhaustive self-review pass against every `<acceptance_criteria>` bullet across all 3 tasks and every `must_haves` entry (both `truths` and `artifacts`), re-running every automated verification command listed in the plan plus a full clean-room `rm -rf node_modules dist && pnpm install --frozen-lockfile && pnpm build` reproduction. All checks pass (see command output history in this session). **This does not substitute for the plan's required fresh-context review** — flagging explicitly so the phase-level `/gsd-code-review` and the external cross-vendor (Codex) review the execution loop runs are not skipped for this plan's content, and so a follow-up fresh-context review can be run against this plan specifically if the orchestrator has the capability this session lacked.

## User Setup Required

None - no external service configuration required. (`pnpm install --frozen-lockfile` is the only setup step, documented in the workspace README.)

## Next Phase Readiness

- The foundation is fan-out-safe: 02-04 through 02-10 (pilot + six surfaces) add `*.route.tsx` files under `src/routes/<surface>/` and copy `FIELDS.template.md`/`CRITIQUE.template.md`/`PARITY.template.json` to their own `.planning/design/<surface>/` directories, without editing any file this plan created.
- `recipeFixtures.ts` gives every surface real copy to build against immediately — no surface plan needs to re-derive recipe text.
- Outstanding, not blocking this plan: a fresh-context `superpowers:requesting-code-review` pass on this plan's diff specifically (see Issues Encountered) — recommend the orchestrator run one before or alongside the phase-level review gate.

---
*Phase: 02-design-all-mockups-checkpoint-1*
*Completed: 2026-07-03*

## Self-Check: PASSED

All 20 created files verified present on disk (`test -f` per file, 20/20 FOUND). All 4 task/fixup commit hashes (`31cfdbc`, `6d2c236`, `f9eea9a`, `215fc05`) verified present in `git log --oneline --all`. No missing items.
