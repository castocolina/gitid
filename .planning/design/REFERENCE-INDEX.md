# Reference Index — the Phase 2 design reference

**The interactive web demo is the authoritative design reference.** The earlier
static reference set (100 PNGs — one HTML + one TUI capture per screen — plus the
static Go dummy TUI that produced the TUI half) was removed as stale after the
design checkpoint rejected the static paradigm; everything removed is recoverable
from git history. A live, executable Go TUI demo will replace the static Go dummy
in a separately replanned task.

## Interactive demo (authoritative)

The mockup SPA's index route (`/`) is a keyboard-driven, stateful walkthrough with
dummy data — no backend, nothing on disk is touched:

- Serve the build: `cd .planning/design/mockup-src && pnpm install && pnpm build &&
  python3 -m http.server 8747 --directory dist`, then open `http://localhost:8747/`.
- Keys mirror the planned TUI exactly: `1` identities (home) · `2` global-ssh ·
  `3` global-git · `4` health · `5` fixer · `n` new identity · `g` configure Git ·
  `?` help · `Ctrl+P` command palette · `Esc` back · `q` quit(→home).
- Workflows to exercise: create identity end-to-end (SSH form → algorithm → two-stage
  test with a simulate-failure toggle → Git details → match strategy with live
  includeIf preview → review → confirm + backup + result); the live list with
  state/git/findings flags; per-identity detail (SSH-first) with action menu, clone,
  new key, and delete (scope choice → typed destructive confirm); global-ssh/global-git
  option review with advisory apply ceremony; health scan → finding detail → fixer
  hand-off; every fix updates the same state the list and header chip render.
- Implementation: `mockup-src/src/demo/` (state store seeded from
  `recipeFixtures.ts`, shared `MutationCeremony` four-beat write component, one
  screen component per surface).

## Static per-screen reference routes (kept)

The 50 static HTML reference routes are untouched and remain available per-screen:
open the demo and press `Ctrl+P` — the command palette lists every static screen as
a `ref: <surface>/<screen>` entry (browser Back returns to the demo). Each route
renders its `<surface>/<screen>` breadcrumb plus a screen-specific `SIG-...` marker
(`mockup-src/src/data/screenSignatures.ts`).

Fixture data lives in two byte-mirrored sources:

- `mockup-src/src/data/recipeFixtures.ts` — the SPA's typed fixture source
  (recipe-accurate; derived from `recipes/`, the North Star).
- `internal/dummytui/data.go` — the Go mirror the upcoming live Go TUI demo will
  seed from.

## Per-surface index

Per-surface field contracts and critiques are kept alongside each surface:
`.planning/design/<surface>/FIELDS.md` and `.planning/design/<surface>/CRITIQUE.md`.

### create-flow (12 screens)

- Screens: algo-catalog, ssh-form-empty, ssh-form-filled, ssh-form-blank-prefix,
  reuse-key-vs-generate, macos-globals-block, test-stage1-direct, test-stage2-by-alias,
  test-fail, confirm-write, backup-notice, result-success
- Access in the demo: `n` from identities (home)

### git-screen (7 screens)

- Screens: git-form-empty, git-form-filled, match-strategy-select, review-readonly,
  confirm-write, backup-notice, result-success
- Access in the demo: `g` from identities (home)

### identity-manager (8 screens)

- Screens: list-populated, list-empty, detail-ssh-first, action-menu, clone-name-prompt,
  delete-choice, confirm-destructive, backup-notice
- Access in the demo: `1` (home / entry surface)

### global-ssh (6 screens)

- Screens: options-list, option-detail, fix-preview, confirm-write, backup-notice,
  result-applied
- Access in the demo: `2`

### global-git (6 screens)

- Screens: options-list, option-detail, fix-preview, confirm-write, backup-notice,
  result-applied
- Access in the demo: `3`

### health (5 screens)

- Screens: health-with-findings, health-all-green, finding-detail, per-identity-health,
  parse-error
- Access in the demo: `4` — visibly read-only (no ceremony beats)

### fixer (6 screens)

- Screens: fixer-list, fix-preview, confirm-destructive, backup-notice, result-applied,
  nothing-to-fix
- Access in the demo: `5`

Total: **50 screens across 7 surfaces**.

## Removed as stale (superseded by the interactive demo)

Removed from the working tree — all recoverable from git history on this branch:

- The 100 static reference PNGs (`.planning/design/<surface>/{html,tui}/*.png`)
  and `GALLERY.html`.
- The static Go dummy TUI: `cmd/gitid-dummy/` and `internal/dummytui`'s screen
  machinery (registry/model/shell/overlay/surface files). Only the shared fixture
  data survives, in `internal/dummytui/data.go`.
- The per-surface capture `manifest.json`/`parity.json` files,
  `PARITY.template.json`, the PTY navigation frame dumps
  (`dummy-nav-frames/`), the `dummy-nav-e2e` / `screenshot-html-mockups` /
  `screenshot-tui-mockups` make targets, and the capture/e2e code that consumed
  them (`internal/screenshot/{manifest,design_adapter}.go` + tests,
  `e2e/dummy_nav_e2e_test.go`).

Kept deliberately:

- `.planning/design/_spike/` — Phase 1's screenshot-tooling spike (golden hashes
  for `make screenshot-tui` / `make screenshot-html`); it validates the capture
  pipeline, which stays.
- `internal/screenshot/` — the generic TUI/HTML capture pipeline (plus the vendored
  fonts under `.planning/design/fonts/`). Screenshots will be re-captured from the
  upcoming live Go TUI demo as development checks, as needed.
- `APPROVAL.md`, every surface's `FIELDS.md`/`CRITIQUE.md`, the
  `FIELDS.template.md`/`CRITIQUE.template.md` templates, and ALL of `mockup-src/`
  (the demo plus the 50 static reference routes).
- `make gate-no-backend-files` — still enforces the design-only-file invariant on
  this branch, now as a standalone target.
