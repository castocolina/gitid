---
phase: 02-design-all-mockups-checkpoint-1
plan: 09
subsystem: ui
tags: [react, mui, bubbletea-v2, lipgloss-v2, go, design-mockup, tui-dummy, screenshot, e2e, fan-out-surface, health, doctor-severity, read-only]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 01
    provides: "the shared four-region MUI shell + recipeFixtures.ts + route auto-discovery this plan's 5 health routes rely on"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 02
    provides: "internal/dummytui's Register/RegisterOrReplace registry and the 02-02 health placeholder (key 4, screen \"entry\") this plan replaces"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 03
    provides: "the hardened manifest.json schema/loader, design_capture_test.go's manifest-driven capture, and dummy_nav_e2e_test.go's manifest-driven PTY walker"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 04
    provides: "the proven per-surface pipeline (FIELDS -> manifest -> parity seed -> mockup -> dummy -> capture -> critique -> parity 0-unresolved) this plan replicates verbatim"
  - phase: 02-design-all-mockups-checkpoint-1
    plan: 06
    provides: "identityManagerRows fixtures (the 'legacy' identity, state fragment-path-missing) this plan reuses byte-identically for per-identity-health, tracing HLTH-05 to MGR-07"
  - phase: 01-foundations-spikes-ci
    plan: null
    provides: "internal/doctor/doctor.go's Severity/Family model (info/warning/error/critical; Permissions/Coherence/Orphans/Redundancy/Overlap), the substrate this plan's finding copy stays coherent with (HLTH-06)"
provides:
  - "The Health check screen (view 4, owned later by Phase 8) as 5 named states in BOTH media: /mui v7 routes under src/routes/health/*.route.tsx and internal/dummytui/surface_health.go, replacing the 02-02 placeholder as the SOLE owner of ActivationKey \"4\" via RegisterOrReplace"
  - ".planning/design/health/{FIELDS.md, manifest.json, parity.json, CRITIQUE.md, html/*.png (5), tui/*.png (5)} — the sixth complete per-surface pipeline artifact set"
  - "recipeFixtures.ts extended with health-only exports — the 4-level doctor severity model (info/warning/error/critical) under a locked glyph contract, 5 concrete findings covering HLTH-03 (duplicate Host *) and HLTH-04 (IdentitiesOnly contradiction; includeIf targeting a missing fragment), and the read-only-integrity banner shared by all 5 screens"
  - "The health surface's intra-flow keys (h/v/i/x) reachable on the real cmd/gitid-dummy binary from its own entry screen, proven by the surface-scoped dummy-nav e2e with zero writes to the sandboxed HOME"
affects: [02-10, 02-11, 02-12]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Replicated the create-flow/git-screen/identity-manager/global-ssh/global-git per-surface pipeline exactly (FIELDS.md -> manifest.json -> parity.json seed -> /mui mockup -> dummytui surface -> capture -> critique -> parity 0-unresolved) on a SIXTH surface — the fourth number-key ActivationKey surface after identity-manager/global-ssh/global-git, confirming RegisterOrReplace's placeholder-replacement pattern generalizes cleanly to key 4"
    - "First surface with ZERO write-ceremony screens by design: unlike global-ssh/global-git's advisory fix chain and identity-manager's delete ceremony, health has no confirm-write/backup-notice/result-applied screen at all — read-only integrity (§4.6/§5) is asserted POSITIVELY (a shared banner on all 5 screens) AND NEGATIVELY (a Go test asserting no confirm/backup/apply marker string ever appears in the rendered output), a new assertion shape none of the five prior fan-out surfaces needed"
    - "First surface to introduce a 4th severity level beyond the existing 3-tone (healthy/warning/error) color-semantics table: the doctor substrate's info level needed a cyan hue theme.ts's semanticColors table doesn't have — defined locally (healthInfoColor in recipeFixtures.ts, lipgloss.Color(\"6\") in surface_health.go) rather than editing the shared theme file (fan-out isolation, review MEDIUM-10)"
    - "Traceable per-identity computation: per-identity-health's Git finding is the SAME Go struct value (asserted by direct equality in TestHealth_PerIdentitySliceTracesTheSameFindingAsTheListView) as the includeIf-missing-fragment finding in the full findings list — HLTH-05's per-identity slice is proven to be a VIEW over the same data, not a re-derived duplicate, directly supporting MGR-07's Identity Manager row hookup"
    - "A THIRD live-PTY-viewport TUI compaction (git-screen's gsFieldsCompactLine precedent, global-ssh's options-list precedent, global-git's fix-preview/confirm-write precedent) — but this time it worked on the FIRST attempt with no overflow iteration needed: the one-line-per-finding render (glyph+word + title + [family]) was chosen up front based on the established pattern, and TestDummyNavReachesAllScreens/health passed immediately"
key-files:
  created:
    - .planning/design/health/FIELDS.md
    - .planning/design/health/manifest.json
    - .planning/design/health/parity.json
    - .planning/design/health/CRITIQUE.md
    - .planning/design/health/html/*.png (5 files)
    - .planning/design/health/tui/*.png (5 files)
    - .planning/design/dummy-nav-frames/dummy-nav-health-*.txt (5 files, e2e evidence)
    - .planning/design/mockup-src/src/routes/health/health-with-findings.route.tsx
    - .planning/design/mockup-src/src/routes/health/health-all-green.route.tsx
    - .planning/design/mockup-src/src/routes/health/finding-detail.route.tsx
    - .planning/design/mockup-src/src/routes/health/per-identity-health.route.tsx
    - .planning/design/mockup-src/src/routes/health/parse-error.route.tsx
    - internal/dummytui/surface_health.go
    - internal/dummytui/surface_health_test.go
  modified:
    - .planning/design/mockup-src/src/data/recipeFixtures.ts (health-only exports appended; nothing above modified)

key-decisions:
  - "4-level doctor severity model (info/warning/error/critical) with a locked glyph contract (warning=! yellow, error/critical=✗ red — distinguished by word not glyph, info=~ cyan) — cyan defined locally (healthInfoColor) rather than editing the shared theme.ts, keeping fan-out isolation"
  - "per-identity-health targets the 'legacy' identity (identityManagerRows, state fragment-path-missing) reused byte-identically, tracing HLTH-05 to the SAME finding health-with-findings' Git section shows — MGR-07's Manager row derives from this exact slice, not a re-derived duplicate"
  - "Zero write-ceremony screens by design (read-only integrity) — negatively asserted in Go (TestHealth_NoWriteCeremonyMarkerAnywhere) as well as positively documented, a new assertion shape this surface introduces"
  - "finding-detail and parse-error are deliberate single-finding deep-dives (mirroring global-ssh/global-git's option-detail pattern) rather than repeating the full SSH+Git section layout — each still names its own section explicitly (TestHealth_DetailScreensNameTheirOwnSection)"
  - "Entry screen is health-with-findings (not health-all-green, despite UX-DIRECTION §4.6 listing health-all-green first in prose) — mirrors identity-manager's own precedent (list-populated, not list-empty, is the entry screen despite UX-DIRECTION's prose order) of choosing the richer, more illustrative default state as the entry point"
  - "DLV-01/DLV-02/DLV-05 remain incomplete in REQUIREMENTS.md pending the remaining surface (02-10, fixer) — matching the 02-04 through 02-08 precedent of not marking a phase-spanning requirement complete on any single fan-out plan"

patterns-established:
  - "Read-only-integrity double assertion: a shared banner constant (healthReadOnlyNote/hlthReadOnlyNote) rendered first-in-body on every screen (positive check) PLUS a negative-marker-string test (no confirm/backup/apply phrase anywhere) — reusable for the Fixer surface's own OPPOSITE affordance (it DOES mutate, so it needs the FULL four-beat ceremony, not this pattern)"
  - "4th semantic hue (info=cyan) added locally per-surface rather than centrally in theme.ts, preserving fan-out isolation while still meeting the doctor substrate's real severity granularity"

requirements-completed: []  # DLV-01/DLV-02/DLV-05 phase-spanning — see key-decisions; this plan ships 6/7 surfaces, not full coverage

# Metrics
duration: 45min
completed: 2026-07-03
---

# Phase 2 Plan 09: Health Check Screen (MUI + TUI, 5 states, read-only) Summary

**The Health check screen — SSH and Git sections, 4-level doctor severity model, zero write-ceremony screens — mocked in MUI v7 and dummied in the TUI across all 5 named states from UX-DIRECTION §4.6, with a Go negative assertion proving read-only integrity holds.**

## Performance

- **Duration:** ~45 min
- **Tasks:** 3 completed
- **Files modified:** 22 (16 created under `.planning/design/health/` + 5 route files + `internal/dummytui/surface_health.go`/`_test.go`, plus 1 shared-fixture append to `recipeFixtures.ts`)

## Accomplishments

- All 5 named states (`health-with-findings`, `health-all-green`, `finding-detail`, `per-identity-health`, `parse-error`) built in both MUI v7 and the TUI dummy, sharing byte-identical copy via `recipeFixtures.ts`'s `health*` exports and `surface_health.go`'s `hlth*` Go constants
- HLTH-01 (SSH + Git sections) and HLTH-03/HLTH-04 (duplicate `Host *`; `IdentitiesOnly no` + explicit `IdentityFile` contradiction; `includeIf` targeting a missing fragment) demonstrated with concrete, recipe-accurate copy
- HLTH-05 traceability proven directly in Go: `per-identity-health`'s Git finding for the `legacy` identity is the SAME struct value as the finding in the full findings list (`TestHealth_PerIdentitySliceTracesTheSameFindingAsTheListView`), not a re-derived duplicate — the exact slice MGR-07's Identity Manager row would derive from
- Read-only integrity (§4.6/§5 highest-risk affordance) verified end-to-end: a shared banner on all 5 screens (positive check) plus a Go negative assertion that no confirm/backup/apply write-ceremony marker string ever appears (`TestHealth_NoWriteCeremonyMarkerAnywhere`) — this surface has zero write-ceremony screens by design, the first of the six built so far
- 5 HTML + 5 TUI PNGs captured; the surface-scoped dummy-nav e2e reaches every screen through the real `cmd/gitid-dummy` binary with zero writes to the sandboxed HOME; `parity.json`'s 9 rows all resolved

## Task Commits

1. **Task 1: health FIELDS.md + manifest.json (hardened) + parity.json seed + MUI mockup (5 states)** - `e6fd72b` (feat)
2. **Task 2: health TUI dummy surface (5 screens, RegisterOrReplace key 4, read-only, backend-free)** - `b4c3a37` (feat)
3. **Task 3: Capture health (both media) + agent-ui-ux-designer critique -> parity.json 0-unresolved** - `9525ef2` (docs)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS)

## Files Created/Modified

- `.planning/design/health/FIELDS.md` - per-screen field-parity manifest, 5 states, read-only integrity pinned
- `.planning/design/health/manifest.json` - hardened 5-entry schema (unique screen/htmlRoute/signature, absolute keysFromHome)
- `.planning/design/health/parity.json` - 9 rows (7 §3 dimensions + `ssh-git-two-section` + `read-only-integrity`), all resolved
- `.planning/design/health/CRITIQUE.md` - aesthetic pass + 3 structured findings, all resolved
- `.planning/design/health/html/*.png`, `.planning/design/health/tui/*.png` - 5+5 captured screenshots
- `.planning/design/dummy-nav-frames/dummy-nav-health-*.txt` - 5 PTY frame captures, e2e evidence
- `.planning/design/mockup-src/src/routes/health/*.route.tsx` - 5 MUI v7 route files, terminal-skin `<Shell>`, master-detail entry screen
- `.planning/design/mockup-src/src/data/recipeFixtures.ts` - appended `health*` exports (severity model, 5 findings, all-green summary, per-identity target, parse-error target, read-only banner); nothing above the appended section modified
- `internal/dummytui/surface_health.go` - registers health as view 4 via `RegisterOrReplace`, 5 `ScreenDef`s, no backend import
- `internal/dummytui/surface_health_test.go` - 12 test functions: registration/sole-ownership, per-screen render+signature+breadcrumb, signature uniqueness, SSH/Git section presence, 4-severity-level + locked-glyph-contract assertion, HLTH-03/04 example presence, read-only banner presence (positive), no-write-ceremony-marker (negative, LOW-11), key-graph connectivity, no n/g key reuse, per-identity traceability

## Decisions Made

See `key-decisions` in the frontmatter for the full rationale on: the 4-level severity model and its locally-defined cyan hue, why `per-identity-health` targets `legacy` and how its traceability is proven, why `finding-detail`/`parse-error` are single-finding deep-dives rather than repeating both sections, why `health-with-findings` (not `health-all-green`) is the entry screen, and why DLV-01/02/05 are not marked complete in REQUIREMENTS.md by this fan-out plan alone.

## Deviations from Plan

**None (Rules 1–3) — plan executed exactly as written.** One self-correction during Task 2 authoring, caught by this plan's OWN test suite before commit (not a plan deviation, a normal TDD-style fix during development):

- **Test design correction (not a Rule 1–4 deviation):** the first draft of `TestHealth_EveryScreenHasBothSSHAndGitSections` asserted BOTH "SSH" and "Git" literal words on ALL 5 screens, including `finding-detail` and `parse-error` — but those two screens are deliberate single-finding deep-dives (mirroring `global-ssh`/`global-git`'s own `option-detail` pattern) that only show ONE section's content by design. Caught immediately by the test itself failing (`go test -race ./internal/dummytui/... -run Health`), before any commit. Split into `TestHealth_ListAndSummaryScreensHaveBothSSHAndGitSections` (the 3 full-snapshot screens) and `TestHealth_DetailScreensNameTheirOwnSection` (the 2 deep-dive screens, each naming its own section) — both committed together in Task 2's single commit (`b4c3a37`), so history shows the correct, intentional test design, not a broken-then-fixed sequence.

## Auth Gates Encountered

None.

## Issues Encountered

- **`freeze` binary not installed** (Task 3): `TestCaptureAllMockupScreens/health/tui` initially failed with `freeze binary not found on PATH (run make setup-env)`. Installed via the EXACT command the project's own `Makefile` `setup-env` target specifies (`go install github.com/charmbracelet/freeze@v0.2.2`, pinned version, provenance already recorded in `.planning/design/_spike/GOLDENS.md` from Phase 1) — this is the project's own documented, pinned dev tool, not a new/unverified package choice, so it did not trigger the Rule 3 package-install exclusion. Re-ran the capture; all 5 TUI PNGs produced successfully.
- **Task/subagent-dispatch tool unavailable in this executor's environment**, same limitation recorded in 02-01 through 02-08's SUMMARY.md files. Task 3 calls for spawning `agent-ui-ux-designer` for two passes (an HTML-only aesthetic pass and the structured HTML↔TUI parity review). This executor's toolset was limited to `Read`/`Write`/`Edit`/`Bash` — no way to spawn a fresh-context subagent. In its place, this executor applied `agent-ui-ux-designer`'s documented methodology (F-pattern/left-side bias, Fitts's/Hick's Law, accessibility, distinctive typography) directly against all 10 captured screenshots and recorded the results in `CRITIQUE.md`. **This does not substitute for a fresh-context `agent-ui-ux-designer` pass** — flagging explicitly so the phase-level `/gsd-code-review` and the external cross-vendor (Codex) review the execution loop runs are not skipped for this plan's content.
- The `superpowers:requesting-code-review` skill referenced by this plan's `<success_criteria>` was similarly unavailable for the same reason (no subagent-dispatch tool). Every task's `<acceptance_criteria>` was instead re-verified directly via its exact automated command (see Task Commits' verification notes and the Deviations section above) — all green, plus a full-repo `go build ./...`, `go test -race ./...`, and `make lint` pass beyond what the plan's own per-task verify commands required, and a plan-scoped `git diff` proof (commits `e6fd72b~1..9525ef2`) that `data.go`/`model.go`/`registry.go`/`App.tsx`/`package.json`/`pnpm-lock.yaml`/`Makefile`/`e2e/dummy_nav_e2e_test.go` were never touched by this plan.

## Verification

- `pnpm exec tsc --noEmit` clean; `pnpm build` (with `verify-routes.mjs`) exits 0, 45 routes total (5 new)
- `manifest.json`: exactly 5 hardened entries, unique screen/signature, non-empty `keysFromHome`
- `parity.json`: 9 rows, 0 unresolved
- `go build ./cmd/gitid-dummy/...` clean; `go test -race ./internal/dummytui/... -run Health` passes (12 test funcs)
- No-backend ALLOWLIST holds: `go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/...` reports only `internal/dummytui`/`cmd/gitid-dummy` first-party packages
- No shared-file edit: `git diff --name-only e6fd72b~1..9525ef2 -- internal/dummytui/data.go internal/dummytui/model.go internal/dummytui/registry.go .planning/design/mockup-src/src/App.tsx .planning/design/mockup-src/package.json .planning/design/mockup-src/pnpm-lock.yaml Makefile e2e/dummy_nav_e2e_test.go` is empty
- 5 HTML + 5 TUI PNGs captured (`TestCaptureAllMockupScreens/health`); surface-scoped dummy-nav e2e passes with zero writes to the sandboxed HOME (`TestDummyNavReachesAllScreens/health`)
- Full-repo `go build ./...`, `go test -race ./...`, and `make lint` (0 issues) all clean beyond the plan's own per-task verify commands

## Next Steps

- 02-10 (Fixer, the surface with the OPPOSITE affordance — full write ceremony) is the last of the 7 fan-out surfaces; that plan (or 02-11/02-12) is where DLV-01/DLV-02/DLV-05 should be marked complete in REQUIREMENTS.md
- Outstanding, not blocking this plan: two fresh-context reviews this session's toolset could not run (`agent-ui-ux-designer` subagent pass, `superpowers:requesting-code-review`) — recommend the orchestrator run both before or alongside the phase-level review gate, as recorded in Issues Encountered

## Self-Check: PASSED
