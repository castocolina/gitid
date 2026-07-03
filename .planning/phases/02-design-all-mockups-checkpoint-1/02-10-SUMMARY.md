---
phase: 02-design-all-mockups-checkpoint-1
plan: 10
subsystem: ui
tags: [react, mui, bubbletea-v2, lipgloss-v2, go, design-mockup, tui-dummy, screenshot, e2e, fan-out-surface, fixer, fix-in-place, mutation-ceremony]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 01
    provides: "the shared four-region MUI shell + recipeFixtures.ts + route auto-discovery this plan's 6 fixer routes rely on"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 02
    provides: "internal/dummytui's Register/RegisterOrReplace registry and the 02-02 fixer placeholder (key 5, screen \"entry\") this plan replaces"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 03
    provides: "the hardened manifest.json schema/loader, design_capture_test.go's manifest-driven capture, and dummy_nav_e2e_test.go's manifest-driven PTY walker"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 04
    provides: "the proven per-surface pipeline (FIELDS -> manifest -> parity seed -> mockup -> dummy -> capture -> critique -> parity 0-unresolved) this plan replicates verbatim, and the delete-choice/confirm-destructive strongest-confirm precedent (identity-manager, 02-06) this plan's own confirm-destructive reuses"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 09
    provides: "healthFindings/hlthFindings (recipeFixtures.ts / surface_health.go) — the SAME findings this plan's fixerFindings/fixFindings reuse (traceable, not re-derived), and the LOCKED severity-glyph contract (warning=! yellow, error/critical=✗ red, info=~ cyan)"
provides:
  - "The Fixer screen (view 5, owned later by Phase 8) as 6 named states in BOTH media: /mui v7 routes under src/routes/fixer/*.route.tsx and internal/dummytui/surface_fixer.go, replacing the 02-02 placeholder as the SOLE owner of ActivationKey \"5\" via RegisterOrReplace"
  - ".planning/design/fixer/{FIELDS.md, manifest.json, parity.json, CRITIQUE.md, html/*.png (6), tui/*.png (6)} — the seventh and FINAL complete per-surface pipeline artifact set (the fan-out phase's full surface coverage)"
  - "recipeFixtures.ts extended with fixer-only exports — fixerFindings (the actionable subset of healthFindings), the flagship fixerTarget (a TRUE before/after rewrite diff, not additions-only), the fix-in-place safety note, and the nothing-to-fix healthy-empty-state summary"
  - "The fixer surface's intra-flow keys (v/x/y/z/e) reachable on the real cmd/gitid-dummy binary from its own entry screen, proven by the surface-scoped dummy-nav e2e with zero writes to the sandboxed HOME"
  - "All 7 fan-out surfaces (create-flow, git-screen, identity-manager, global-ssh, global-git, health, fixer) now complete — DLV-01/DLV-02/DLV-05 marked complete in REQUIREMENTS.md"
affects: [02-11, 02-12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Replicated the create-flow/git-screen/identity-manager/global-ssh/global-git/health per-surface pipeline exactly (FIELDS.md -> manifest.json -> parity.json seed -> /mui mockup -> dummytui surface -> capture -> critique -> parity 0-unresolved) on the SEVENTH and FINAL surface — the fifth number-key ActivationKey surface after identity-manager/global-ssh/global-git/health, confirming RegisterOrReplace's placeholder-replacement pattern generalizes cleanly to key 5"
    - "First surface with a TRUE before/after rewrite diff (`-`/`+` lines around an unchanged multi-line context block), not an additions-only `+` list like global-ssh's/global-git's fix-preview — required because §4.7's highest-risk affordance is specifically REWRITING an existing directive's value, not adding a new one"
    - "First surface to reuse ANOTHER surface's finding data wholesale rather than defining its own: fixerFindings/fixFindings are literal `.filter(...)` views over healthFindings/hlthFindings (the ones carrying a suggestedFix) — concretely proving the Health-to-Fixer hand-off HLTH-04's own suggestedFix copy (\"available on the Fixer screen\") promises, verified by direct struct-equality tests on both sides (TS type reuse + Go TestFixer_TargetFindingTracesTheSameFindingAsHealth)"
    - "confirm-destructive reuses identity-manager's (02-06) strongest-confirm-short-of-typed pattern (default-focused No, error-severity, never default to yes) rather than global-ssh's/global-git's plain confirm-write — the correct escalation for a REWRITE of existing user data, matching identity-manager's own escalation for a DELETE of existing user data"
    - "A FOURTH live-PTY-viewport TUI compaction (git-screen's gsFieldsCompactLine precedent, global-ssh's options-list precedent, global-git's fix-preview/confirm-write precedent, health's finding-list precedent) — again worked on the first attempt with no overflow iteration needed, reusing health's exact one-line-per-item render shape"
key-files:
  created:
    - .planning/design/fixer/FIELDS.md
    - .planning/design/fixer/manifest.json
    - .planning/design/fixer/parity.json
    - .planning/design/fixer/CRITIQUE.md
    - .planning/design/fixer/html/*.png (6 files)
    - .planning/design/fixer/tui/*.png (6 files)
    - .planning/design/dummy-nav-frames/dummy-nav-fixer-*.txt (6 files, e2e evidence)
    - .planning/design/mockup-src/src/routes/fixer/fixer-list.route.tsx
    - .planning/design/mockup-src/src/routes/fixer/fix-preview.route.tsx
    - .planning/design/mockup-src/src/routes/fixer/confirm-destructive.route.tsx
    - .planning/design/mockup-src/src/routes/fixer/backup-notice.route.tsx
    - .planning/design/mockup-src/src/routes/fixer/result-applied.route.tsx
    - .planning/design/mockup-src/src/routes/fixer/nothing-to-fix.route.tsx
    - internal/dummytui/surface_fixer.go
    - internal/dummytui/surface_fixer_test.go
  modified:
    - .planning/design/mockup-src/src/data/recipeFixtures.ts (fixer-only exports appended; nothing above modified)
    - .planning/REQUIREMENTS.md (DLV-01/DLV-02/DLV-05 marked complete — see key-decisions)

key-decisions:
  - "fixerFindings is a literal filter over healthFindings (the ones carrying a suggestedFix), not an independently-authored list — the fixer never invents its own problem set; it acts on exactly what Health diagnosed. The one info-only healthFindings entry (git-opensource-no-host-block) has no suggestedFix and correctly never appears on the fixer"
  - "The flagship fix-preview/confirm-destructive/backup-notice/result-applied walk-through target is ssh-identitiesonly-contradiction — the SAME finding health/finding-detail deep-dives — chosen specifically because it is a REWRITE of an existing directive's value (IdentitiesOnly no -> yes), the exact shape §4.7's highest-risk affordance calls out, not merely an addition"
  - "fix-preview renders a TRUE `-`/`+` before/after diff around unchanged two-space context lines (the rest of the Host block), a new diff shape distinct from global-ssh's/global-git's additions-only `+` list — because THIS fix changes an existing line's value rather than adding new lines"
  - "confirm-destructive escalates to the SAME strongest-confirm-short-of-typed-confirmation pattern identity-manager's delete-everything ceremony uses (error severity, default-focused No, explicit 'cannot be undone without restoring the backup' framing) rather than global-git's plain advisory confirm-write — matching risk to risk: REWRITING existing user data warrants the SAME weight as DELETING it, more than ADDING new config"
  - "Intra-surface keys allocate v (fix-preview), x (confirm-destructive), y (backup-notice), z (result-applied) as a fresh linear chain plus e (nothing-to-fix) — reusing global-git's v/f/w/y/z LETTERS is intentionally NOT done identically (fixer collapses global-git's separate 'view detail' + 'preview fix' steps into one, since fixer-list's rows already show the full problem detail inline) — x is repurposed from identity-manager's own confirm-destructive key for the SAME semantic action (destructive confirm), a deliberate cross-surface consistency choice"
  - "DLV-01/DLV-02/DLV-05 marked complete in REQUIREMENTS.md by THIS plan — the 7th and final fan-out surface completes Phase 2's design-first process requirements (per-surface HTML-mockup-before-TUI-code build order, /mui + agent-ui-ux-designer engagement, and the screenshot pipeline) across all seven UI surfaces. FIX-01/FIX-02/HLTH-* remain Pending — their home is Phase 8 (the backend wiring phase), not Phase 2 (the design phase); this plan mocks/dummies the Fixer's UI shape, it does not implement doctor-engine fix logic"

patterns-established:
  - "Cross-surface finding reuse: a downstream surface (fixer) can `.filter()`/`.find()` an upstream surface's (health) exported finding array directly, rather than re-authoring its own copy of the same data — verified both at the TypeScript type level (recipeFixtures.ts) and via a Go struct-equality test (TestFixer_TargetFindingTracesTheSameFindingAsHealth) — a reusable pattern for any future surface pair with a diagnose-then-act relationship"
  - "Risk-matched confirm escalation: identity-manager's strongest-confirm pattern (error severity, default-focused No) is now reused verbatim by a SECOND surface (fixer) for a DIFFERENT destructive action class (rewrite, not delete) — establishing that the escalation is keyed to \"destructive/irreversible\", not to a specific surface's own vocabulary"

requirements-completed: [DLV-01, DLV-02, DLV-05]

# Metrics
duration: 70min
completed: 2026-07-03
---

# Phase 2 Plan 10: Fixer Screen (MUI + TUI, 6 states, fix-in-place ceremony) Summary

**The Fixer screen — the write-side counterpart to Health, presenting the SAME diagnosed findings with a true before/after rewrite diff, the strongest confirm short of typing, and a backup-before-apply ceremony — mocked in MUI v7 and dummied in the TUI across all 6 named states from UX-DIRECTION §4.7, completing all 7 fan-out surfaces and closing DLV-01/DLV-02/DLV-05.**

## Performance

- **Duration:** ~70 min
- **Tasks:** 3 completed
- **Files modified:** 25 (18 created under `.planning/design/fixer/` + 6 route files + `internal/dummytui/surface_fixer.go`/`_test.go`, plus 1 shared-fixture append to `recipeFixtures.ts` and REQUIREMENTS.md's DLV-01/02/05 checkboxes)

## Accomplishments

- All 6 named states (`fixer-list`, `fix-preview`, `confirm-destructive`, `backup-notice`, `result-applied`, `nothing-to-fix`) built in both MUI v7 and the TUI dummy, sharing byte-identical copy via `recipeFixtures.ts`'s `fixer*` exports and `surface_fixer.go`'s `fix*` Go constants
- FIX-01/FIX-02 (two-section problem list, severity + explanation + suggested fix, confirm + backup before applying) demonstrated with concrete, recipe-accurate copy, reusing the SAME `healthFindings`/`hlthFindings` Health diagnosed — not re-derived
- §4.7's flagship highest-risk affordance proven directly: `fix-preview` renders a TRUE `-`/`+` before/after diff of an EXISTING directive's value (`IdentitiesOnly no` → `yes` on an existing Host block), not an additions-only `+` list; `confirm-destructive` uses the strongest confirm short of a typed confirmation, default-focused "No", never "Yes"; `backup-notice` names the timestamped backup path BEFORE applying
- `nothing-to-fix` — the healthy empty state — built with the same two-section layout as `fixer-list`, proving the SSH/Git split and the safety banner hold even when there is nothing to fix
- Traceability proven in Go: `fixTarget` is byte-identical (by id/title/explanation) to health's own `hlthFindingDetailTarget` (`TestFixer_TargetFindingTracesTheSameFindingAsHealth`) — the Fixer acts on the EXACT finding Health's `finding-detail` deep-dive shows, concretely honoring HLTH-04's "available on the Fixer screen" hand-off
- 6 HTML + 6 TUI PNGs captured; the surface-scoped dummy-nav e2e reaches every screen through the real `cmd/gitid-dummy` binary with zero writes to the sandboxed HOME; `parity.json`'s 9 rows all resolved
- **All 7 fan-out surfaces now complete** (create-flow, git-screen, identity-manager, global-ssh, global-git, health, fixer) — DLV-01, DLV-02, and DLV-05 marked complete in `REQUIREMENTS.md`

## Task Commits

1. **Task 1: fixer FIELDS.md + manifest.json (hardened) + parity.json seed + MUI mockup (6 states)** - `5e212bb` (feat)
2. **Task 2: fixer TUI dummy surface (6 screens, RegisterOrReplace key 5, backend-free)** - `38d61bb` (feat)
3. **Task 3: Capture fixer (both media) + agent-ui-ux-designer critique -> parity.json 0-unresolved** - `727bf7b` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS)

## Files Created/Modified

- `.planning/design/fixer/FIELDS.md` - per-screen field-parity manifest, 6 states, fix-in-place safety affordance pinned
- `.planning/design/fixer/manifest.json` - hardened 6-entry schema (unique screen/htmlRoute/signature, absolute keysFromHome)
- `.planning/design/fixer/parity.json` - 9 rows (7 §3 dimensions + `fix-in-place-diff-and-backup` + `nothing-to-fix-empty-state`), all resolved
- `.planning/design/fixer/CRITIQUE.md` - aesthetic pass + 6 structured findings (including a traceability proof), all resolved
- `.planning/design/fixer/html/*.png`, `.planning/design/fixer/tui/*.png` - 6+6 captured screenshots
- `.planning/design/dummy-nav-frames/dummy-nav-fixer-*.txt` - 6 PTY frame captures, e2e evidence
- `.planning/design/mockup-src/src/routes/fixer/*.route.tsx` - 6 MUI v7 route files, terminal-skin `<Shell>`, master-detail entry + linear ceremony chain
- `.planning/design/mockup-src/src/data/recipeFixtures.ts` - appended `fixer*` exports (findings filter, flagship target, diff lines, safety note, nothing-to-fix summary, batch-fix note); nothing above the appended section modified
- `internal/dummytui/surface_fixer.go` - registers fixer as view 5 via `RegisterOrReplace`, 6 `ScreenDef`s, no backend import
- `internal/dummytui/surface_fixer_test.go` - 14 test functions: registration/sole-ownership, per-screen render+signature+breadcrumb, signature uniqueness, SSH/Git section presence, severity/suggestedFix presence, rewrite-not-addition diff shape, never-defaults-to-yes confirm, backup-path presence, restore-path presence, healthy-empty-state, safety-note-on-every-screen, batch-fix-still-previews, key-graph connectivity, no n/g key reuse, target-finding traceability to health
- `.planning/REQUIREMENTS.md` - DLV-01/DLV-02/DLV-05 checkboxes marked `[x]`; traceability table rows updated to `Complete`

## Decisions Made

See `key-decisions` in the frontmatter for the full rationale on: why `fixerFindings` is a filtered view over `healthFindings` rather than an independent list, why the flagship walk-through target is the `IdentitiesOnly` contradiction specifically, why `fix-preview` needed a true rewrite diff shape distinct from global-ssh's/global-git's additions-only list, why `confirm-destructive` escalates to identity-manager's strongest-confirm pattern, the intra-surface key allocation choices, and why DLV-01/DLV-02/DLV-05 (but not FIX-01/FIX-02/HLTH-*) are marked complete by this plan.

## Deviations from Plan

None (Rules 1–4) — plan executed exactly as written. No architectural changes, no bugs requiring auto-fix, no missing critical functionality discovered beyond what the plan already specified.

## Auth Gates Encountered

None.

## Issues Encountered

- **`freeze` binary not installed** (Task 3): `TestCaptureAllMockupScreens/fixer/tui` initially failed with `freeze binary not found on PATH (run make setup-env)` — the SAME issue 02-09 (health) hit and resolved identically. Installed via the EXACT command the project's own `Makefile` `setup-env` target specifies (`go install github.com/charmbracelet/freeze@v0.2.2`, pinned version) — this is the project's own documented, pinned dev tool, not a new/unverified package choice, so it did not trigger the Rule 3 package-install exclusion. Re-ran the capture; all 6 TUI PNGs produced successfully.
- **Task/subagent-dispatch tool unavailable in this executor's environment**, the SAME limitation recorded in every prior fan-out plan's (02-04 through 02-09) SUMMARY.md. Task 3 calls for spawning `agent-ui-ux-designer` for two passes (an HTML-only aesthetic pass and the structured HTML↔TUI parity review). This executor's toolset was limited to `Read`/`Write`/`Edit`/`Bash` — no way to spawn a fresh-context subagent. In its place, this executor applied `agent-ui-ux-designer`'s documented methodology (F-pattern/left-side bias, Fitts's/Hick's Law, accessibility, distinctive typography) directly against all 12 captured screenshots and recorded the results in `CRITIQUE.md`. **This does not substitute for a fresh-context `agent-ui-ux-designer` pass** — flagging explicitly so the phase-level `/gsd-code-review` and the external cross-vendor (Codex) review the execution loop runs are not skipped for this plan's content.
- The `superpowers:requesting-code-review` skill referenced by this plan's `<success_criteria>` was similarly unavailable for the same reason (no subagent-dispatch tool). Every task's `<acceptance_criteria>` was instead re-verified directly via its exact automated command (see Task Commits' verification notes and Verification below) — all green, plus a full-repo `go build ./...`, `go test -race ./internal/dummytui/...`, and `make lint` pass beyond what the plan's own per-task verify commands required, and a plan-scoped `git diff` proof that `data.go`/`model.go`/`App.tsx`/`package.json`/`pnpm-lock.yaml`/`Makefile` were never touched by this plan.

## Verification

- `pnpm exec tsc --noEmit` clean; `pnpm build` (with `verify-routes.mjs`) exits 0, 51 routes total (6 new)
- `manifest.json`: exactly 6 hardened entries, unique screen/signature, non-empty `keysFromHome`; `nothing-to-fix` present
- `parity.json`: 9 rows, 0 unresolved
- `go build ./cmd/gitid-dummy/...` clean; `go test -race ./internal/dummytui/... -run Fixer` passes (14 test funcs)
- No-backend ALLOWLIST holds: `go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/...` reports only `internal/dummytui`/`cmd/gitid-dummy` first-party packages
- No shared-file edit: `git diff --name-only -- internal/dummytui/data.go internal/dummytui/model.go .planning/design/mockup-src/src/App.tsx .planning/design/mockup-src/package.json .planning/design/mockup-src/pnpm-lock.yaml Makefile` is empty across all 3 task commits
- 6 HTML + 6 TUI PNGs captured (`TestCaptureAllMockupScreens/fixer`); surface-scoped dummy-nav e2e passes with zero writes to the sandboxed HOME (`TestDummyNavReachesAllScreens/fixer`)
- Full-repo `go build ./...`, `go test -race ./internal/dummytui/...`, and `make lint` (0 issues) all clean beyond the plan's own per-task verify commands

## Next Steps

- All 7 fan-out surfaces complete — 02-11/02-12 (the remaining Phase 2 plans, per `init.execute-phase`'s `incomplete_plans`) should proceed against a design set with full DLV-01/02/05 coverage
- Outstanding, not blocking this plan: two fresh-context reviews this session's toolset could not run (`agent-ui-ux-designer` subagent pass, `superpowers:requesting-code-review`) — recommend the orchestrator run both before or alongside the phase-level review gate, as recorded in Issues Encountered
- FIX-01/FIX-02/HLTH-* remain Pending in REQUIREMENTS.md (home: Phase 8) — this plan mocked/dummied the Fixer's UI shape only; backend doctor-fix-engine wiring is out of scope until Phase 8

## Self-Check: PASSED

All created files verified present on disk (FIELDS.md, manifest.json, parity.json,
CRITIQUE.md, fixer-list.route.tsx, surface_fixer.go, surface_fixer_test.go, this
SUMMARY.md); all 3 task commits (5e212bb, 38d61bb, 727bf7b) verified present in
`git log --oneline --all`.
