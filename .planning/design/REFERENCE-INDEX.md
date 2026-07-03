# Reference Index — the frozen screenshot reference set (DLV-01)

Assembled by 02-11 (Wave 5) after the comprehensive dummy-nav PTY e2e proved the whole
`cmd/gitid-dummy` binary navigable and the full dual capture ran clean. This is the
complete, final reference set: every screen named in each surface's `manifest.json`,
captured in **both** media (HTML mockup + TUI dummy), frame-for-frame — the literal
"reference set for every later UI wave" DLV-08 requires.

**These counts are COMPUTED from the 7 surface manifests, not hard-coded** — see
"Computed counts" below.

## Interactive demo (checkpoint-feedback addition)

The static reference set above proves every screen's LOOK; the **interactive demo**
proves the WORKFLOWS. The mockup SPA's index route (`/`) is now a keyboard-driven,
stateful walkthrough with dummy data — no backend, nothing on disk is touched:

- Serve the build: `cd .planning/design/mockup-src && pnpm install && pnpm build &&
  python3 -m http.server 8747 --directory dist`, then open `http://localhost:8747/`.
- Keys mirror the TUI dummy exactly: `1` identities (home) · `2` global-ssh ·
  `3` global-git · `4` health · `5` fixer · `n` new identity · `g` configure Git ·
  `?` help · `Ctrl+P` command palette · `Esc` back · `q` quit(→home).
- Workflows to exercise: create identity end-to-end (SSH form → algorithm → two-stage
  test with a simulate-failure toggle → Git details → match strategy with live
  includeIf preview → review → confirm + backup + result); the live list with
  state/git/findings flags; per-identity detail (SSH-first) with action menu, clone,
  new key, and delete (scope choice → typed destructive confirm); global-ssh/global-git
  option review with advisory apply ceremony; health scan → finding detail → fixer
  hand-off; every fix updates the same state the list and header chip render.
- The `Ctrl+P` palette also opens each of the 50 static reference screens
  (`ref: <surface>/<screen>` entries — browser Back returns to the demo).
- Implementation: `mockup-src/src/demo/` (state store seeded from
  `recipeFixtures.ts`, shared `MutationCeremony` four-beat write component, one
  screen component per surface). The 50 static reference routes are untouched.

## Per-surface index

### create-flow (12 screens)

- Manifest: `.planning/design/create-flow/manifest.json`
- FIELDS: `.planning/design/create-flow/FIELDS.md`
- Parity: `.planning/design/create-flow/parity.json`
- Critique: `.planning/design/create-flow/CRITIQUE.md`
- Screens: algo-catalog, ssh-form-empty, ssh-form-filled, ssh-form-blank-prefix,
  reuse-key-vs-generate, macos-globals-block, test-stage1-direct, test-stage2-by-alias,
  test-fail, confirm-write, backup-notice, result-success
- HTML PNGs (12): `.planning/design/create-flow/html/{algo-catalog,ssh-form-empty,ssh-form-filled,ssh-form-blank-prefix,reuse-key-vs-generate,macos-globals-block,test-stage1-direct,test-stage2-by-alias,test-fail,confirm-write,backup-notice,result-success}.png`
- TUI PNGs (12): `.planning/design/create-flow/tui/{algo-catalog,ssh-form-empty,ssh-form-filled,ssh-form-blank-prefix,reuse-key-vs-generate,macos-globals-block,test-stage1-direct,test-stage2-by-alias,test-fail,confirm-write,backup-notice,result-success}.png`
- Access: keyless modal surface, launched from `identity-manager` via `LaunchKey "n"`
  (`LaunchFrom: identity-manager`)

### git-screen (7 screens)

- Manifest: `.planning/design/git-screen/manifest.json`
- FIELDS: `.planning/design/git-screen/FIELDS.md`
- Parity: `.planning/design/git-screen/parity.json`
- Critique: `.planning/design/git-screen/CRITIQUE.md`
- Screens: git-form-empty, git-form-filled, match-strategy-select, review-readonly,
  confirm-write, backup-notice, result-success
- HTML PNGs (7): `.planning/design/git-screen/html/{git-form-empty,git-form-filled,match-strategy-select,review-readonly,confirm-write,backup-notice,result-success}.png`
- TUI PNGs (7): `.planning/design/git-screen/tui/{git-form-empty,git-form-filled,match-strategy-select,review-readonly,confirm-write,backup-notice,result-success}.png`
- Access: keyless modal surface, launched from `identity-manager` via `LaunchKey "g"`
  (`LaunchFrom: identity-manager`)

### identity-manager (8 screens)

- Manifest: `.planning/design/identity-manager/manifest.json`
- FIELDS: `.planning/design/identity-manager/FIELDS.md`
- Parity: `.planning/design/identity-manager/parity.json`
- Critique: `.planning/design/identity-manager/CRITIQUE.md`
- Screens: list-populated, list-empty, detail-ssh-first, action-menu, clone-name-prompt,
  delete-choice, confirm-destructive, backup-notice
- HTML PNGs (8): `.planning/design/identity-manager/html/{list-populated,list-empty,detail-ssh-first,action-menu,clone-name-prompt,delete-choice,confirm-destructive,backup-notice}.png`
- TUI PNGs (8): `.planning/design/identity-manager/tui/{list-populated,list-empty,detail-ssh-first,action-menu,clone-name-prompt,delete-choice,confirm-destructive,backup-notice}.png`
- Access: primary surface, `ActivationKey "1"` (home / entry surface)

### global-ssh (6 screens)

- Manifest: `.planning/design/global-ssh/manifest.json`
- FIELDS: `.planning/design/global-ssh/FIELDS.md`
- Parity: `.planning/design/global-ssh/parity.json`
- Critique: `.planning/design/global-ssh/CRITIQUE.md`
- Screens: options-list, option-detail, fix-preview, confirm-write, backup-notice,
  result-applied
- HTML PNGs (6): `.planning/design/global-ssh/html/{options-list,option-detail,fix-preview,confirm-write,backup-notice,result-applied}.png`
- TUI PNGs (6): `.planning/design/global-ssh/tui/{options-list,option-detail,fix-preview,confirm-write,backup-notice,result-applied}.png`
- Access: primary surface, `ActivationKey "2"`

### global-git (6 screens)

- Manifest: `.planning/design/global-git/manifest.json`
- FIELDS: `.planning/design/global-git/FIELDS.md`
- Parity: `.planning/design/global-git/parity.json`
- Critique: `.planning/design/global-git/CRITIQUE.md`
- Screens: options-list, option-detail, fix-preview, confirm-write, backup-notice,
  result-applied
- HTML PNGs (6): `.planning/design/global-git/html/{options-list,option-detail,fix-preview,confirm-write,backup-notice,result-applied}.png`
- TUI PNGs (6): `.planning/design/global-git/tui/{options-list,option-detail,fix-preview,confirm-write,backup-notice,result-applied}.png`
- Access: primary surface, `ActivationKey "3"`

### health (5 screens)

- Manifest: `.planning/design/health/manifest.json`
- FIELDS: `.planning/design/health/FIELDS.md`
- Parity: `.planning/design/health/parity.json`
- Critique: `.planning/design/health/CRITIQUE.md`
- Screens: health-with-findings, health-all-green, finding-detail, per-identity-health,
  parse-error
- HTML PNGs (5): `.planning/design/health/html/{health-with-findings,health-all-green,finding-detail,per-identity-health,parse-error}.png`
- TUI PNGs (5): `.planning/design/health/tui/{health-with-findings,health-all-green,finding-detail,per-identity-health,parse-error}.png`
- Access: primary surface, `ActivationKey "4"` — visibly read-only (no ceremony beats)

### fixer (6 screens)

- Manifest: `.planning/design/fixer/manifest.json`
- FIELDS: `.planning/design/fixer/FIELDS.md`
- Parity: `.planning/design/fixer/parity.json`
- Critique: `.planning/design/fixer/CRITIQUE.md`
- Screens: fixer-list, fix-preview, confirm-destructive, backup-notice, result-applied,
  nothing-to-fix
- HTML PNGs (6): `.planning/design/fixer/html/{fixer-list,fix-preview,confirm-destructive,backup-notice,result-applied,nothing-to-fix}.png`
- TUI PNGs (6): `.planning/design/fixer/tui/{fixer-list,fix-preview,confirm-destructive,backup-notice,result-applied,nothing-to-fix}.png`
- Access: primary surface, `ActivationKey "5"`

## Computed counts

Computed via `sum(len(json.load(open(p))) for p in glob('.planning/design/*/manifest.json'))`
over the 7 surface manifests above (create-flow 12 + git-screen 7 + identity-manager 8 +
global-ssh 6 + global-git 6 + health 5 + fixer 6):

| Metric | Value |
|---|---|
| Manifest-computed expected screen count | **50** |
| HTML PNGs captured (7 surfaces) | **50** |
| TUI PNGs captured (7 surfaces) | **50** |
| Required surface directories present | **7 / 7** (create-flow, git-screen, identity-manager, global-ssh, global-git, health, fixer) |

`#HTML == #TUI == sum(manifest lengths) == 50` — verified via a count scoped to
exactly these 7 surface directories (`find .planning/design/{create-flow,git-screen,identity-manager,global-ssh,global-git,health,fixer}/{html,tui} -name '*.png'`).

**Note on scope:** `.planning/design/_spike/{html,tui}/spike.png` (one PNG per medium)
is a pre-existing Phase 1 (`01-05-PLAN.md`) golden-hash artifact for the
`screenshot`-tooling spike (`make screenshot-tui`/`make screenshot-html`,
`_spike/GOLDENS.md`) — it predates Phase 2's 7-surface manifest set, is not one of
the 7 Phase-2 surfaces, and is intentionally excluded from the computed counts above.
A verify command that globs `.planning/design/*/{html,tui}` unscoped (matching `_spike`
too) will observe 51, not 50; the count check must be scoped to exactly the 7
Phase-2 surface directories, as this index does. Logged as a deviation in
`02-11-SUMMARY.md`.

## Cross-surface gates (all green)

- **Comprehensive dummy-nav e2e** (`make dummy-nav-e2e`): the REAL `cmd/gitid-dummy`
  binary drives all 50 screens across all 7 surfaces under a PTY, re-homing before each
  entry, asserting the exact `<surface>/<screen>` breadcrumb AND the entry's
  screen-specific signature per frame, reaching the ~19 keyless modal screens
  (create-flow's 12 + git-screen's 7) through the 02-02 launch mechanism
  (`n`/`g` LaunchKeys from `identity-manager`) — never a direct `RenderScreen` call.
  Asserts zero files written under a sandboxed `HOME` after the walk completes.
- **DLV-05 no-backend ALLOWLIST**: `go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/...`
  contains exactly `internal/dummytui` and `cmd/gitid-dummy` as first-party
  `github.com/castocolina/gitid/...` packages — no backend package (`internal/identity`,
  `internal/keygen`, `internal/sshconfig`, `internal/gitconfig`, `internal/filewriter`,
  `internal/tester`, `internal/doctor`, `internal/adopter`, `internal/uploader`,
  `internal/repoclone`, ...) ever enters the dummy's import graph. Enforced by
  `internal/dummytui/nobackend_test.go` (`TestNoBackendAllowlist`, added 02-03).
- **Final key owners**: `internal/dummytui/keyowners_test.go` (added this plan) asserts
  `Surfaces()` maps `ActivationKey` `"1"`–`"5"` to exactly `identity-manager`,
  `global-ssh`, `global-git`, `health`, `fixer` — and that `create-flow`/`git-screen`
  are registered keyless (empty `ActivationKey`) with a non-empty
  `LaunchFrom`/`LaunchKey` binding.
- **Whole-set parity**: every surface's `parity.json` has 0 rows with
  `status != "resolved"` (63 total rows across all 7 surfaces, all resolved).
- **No-backend-files gate**: with `BASE=$(git merge-base main HEAD)`, every file
  changed in Phase 2 falls under `.planning/` (GSD workflow bookkeeping —
  `STATE.md`/`ROADMAP.md`/`REQUIREMENTS.md`/per-plan `PLAN.md`/`SUMMARY.md`/research
  docs — plus `.planning/design/`), `internal/dummytui/`, `cmd/gitid-dummy/`,
  `internal/screenshot/`, `e2e/`, or `Makefile`. No backend Go source
  (`internal/identity`, `internal/keygen`, `internal/sshconfig`, ...) was touched
  before approval. See `02-11-SUMMARY.md` for the deviation note on the gate's
  scope (the plan's literal verify command greps only `.planning/design/`; this
  index's check widens that to all of `.planning/` since the plan's own
  `PLAN.md`/`SUMMARY.md`/`STATE.md`/`ROADMAP.md` bookkeeping is GSD workflow
  overhead, not a backend-logic change — the threat this gate defends against
  (T-02-BEGATE)).
- **`make lint`**: 0 issues.
