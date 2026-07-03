---
phase: 02-design-all-mockups-checkpoint-1
plan: 11
subsystem: ui
tags: [bubbletea, dummy-tui, pty-e2e, screenshot, design-freeze, dlv-01, dlv-05, dlv-02]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    provides: all 7 surface manifest.json/FIELDS.md/parity.json/CRITIQUE.md files, the dummy's final surfaces (identity-manager/global-ssh/global-git/health/fixer/create-flow/git-screen), and the 02-03 manifest-driven TestDummyNav walker (02-04..02-10)
provides:
  - internal/dummytui/keyowners_test.go — final-owners assertion (keys 1-5 -> the 5 real surfaces; create-flow/git-screen keyless with a LaunchFrom/LaunchKey binding)
  - the comprehensive dummy-nav PTY e2e proven green across all 50 screens of all 7 surfaces on the REAL cmd/gitid-dummy binary
  - the complete frozen reference set (100 PNGs: 50 HTML + 50 TUI) with counts computed from the 7 manifests
  - .planning/design/REFERENCE-INDEX.md — the complete reference index
  - .planning/design/APPROVAL.md — the DLV-08 approval scaffold (§6 A-F checklist, no APPROVED line)
  - Makefile test-e2e timeout raised 60s -> 180s to accommodate the now-complete dummy-nav walk
affects: [02-12 (the human approval checkpoint that adds the APPROVED line), Phase 3+ (all depend on this approval before writing backend logic)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Whole-dummy DLV-05 gates (allowlist, key-owners, comprehensive nav) run as a single assembly plan after all fan-out surfaces land, rather than per-surface"
    - "PNG-count invariants computed from manifests at verification time, scoped to the exact 7 Phase-2 surface dirs (excludes the unrelated Phase-1 _spike golden-hash dir)"
    - "No-backend-files gate scoped to all of .planning/ (GSD workflow bookkeeping) plus the 5 code dirs + Makefile, not just .planning/design/"

key-files:
  created:
    - internal/dummytui/keyowners_test.go
    - .planning/design/REFERENCE-INDEX.md
    - .planning/design/APPROVAL.md
  modified:
    - Makefile (test-e2e timeout 60s -> 180s)
    - .planning/phases/02-design-all-mockups-checkpoint-1/deferred-items.md
    - .planning/design/create-flow/html/result-success.png (re-capture, metadata-only diff)
    - .planning/design/health/html/per-identity-health.png (re-capture, metadata-only diff)

key-decisions:
  - "Scoped the manifest-computed PNG-count check to the 7 Phase-2 surface directories, excluding the pre-existing Phase-1 _spike golden-hash artifact that the plan's literal unscoped glob would also match (51 != 50 otherwise)"
  - "Widened the no-backend-files gate's allowlist to all of .planning/ (not just .planning/design/) since GSD's own required workflow bookkeeping (STATE.md/ROADMAP.md/PLAN.md/SUMMARY.md/REQUIREMENTS.md) is not backend logic — the actual threat (T-02-BEGATE) this gate defends against"
  - "Raised make test-e2e's package timeout 60s -> 180s: this plan is the point where the full 50-screen dummy-nav walk runs alongside the pre-existing real-TUI PTY suite for the first time in the shared ./e2e/... package, observed ~80s under -race"
  - "Performed the Task 2 final cross-surface consistency pass directly (no Task/subagent-dispatch tool available), following the same documented limitation and mitigation established in every prior 02-04..02-10 CRITIQUE.md"

patterns-established:
  - "Assembly-wave plans (Wave 5+) re-verify whole-system invariants (allowlist, key ownership, nav completeness, parity) rather than re-deriving them per surface"
  - "Reference-freeze artifacts (REFERENCE-INDEX.md, APPROVAL.md) are computed/generated from on-disk manifests and parity.json at verification time, never hand-maintained counts"

requirements-completed: [DLV-01, DLV-05, DLV-02]

# Metrics
duration: 30min
completed: 2026-07-03
---

# Phase 02 Plan 11: Comprehensive Dummy-Nav Proof + Reference Freeze Summary

**Proved the whole `cmd/gitid-dummy` binary navigable across all 50 screens of all 7 surfaces on a real PTY (breadcrumb + signature per frame, including the 19 keyless modal screens reached via the launch mechanism), then froze the complete 100-PNG reference set with computed (not hard-coded) counts and scaffolded the DLV-08 approval record.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-07-03T08:10Z (approx.)
- **Completed:** 2026-07-03T12:36Z
- **Tasks:** 2 completed
- **Files modified:** 8 (3 created, 5 modified — including 2 re-captured PNGs and 1 deviation-driven Makefile fix)

## Accomplishments

- `internal/dummytui/keyowners_test.go`: final-owners assertion — `Surfaces()` maps
  `ActivationKey` `"1"`-`"5"` to exactly `identity-manager`/`global-ssh`/`global-git`/
  `health`/`fixer`, and `create-flow`/`git-screen` are keyless with a
  `LaunchFrom`/`LaunchKey` binding. `go test -race ./internal/dummytui/... -run
  KeyOwners` passes.
- Ran the comprehensive `make dummy-nav-e2e` (the 02-03 manifest-driven
  `TestDummyNav` walker, already breadcrumb+signature-per-frame hardened): drives
  the real binary across all 50 screens of all 7 surfaces — including the ~19
  keyless modal screens (create-flow's 12 + git-screen's 7) reached via the 02-02
  launch mechanism — asserting zero writes under a sandboxed `HOME`. PASS (~46s).
- Reconfirmed the whole-dummy DLV-05 no-backend ALLOWLIST
  (`internal/dummytui/nobackend_test.go`) and the whole-set `parity.json` invariant
  (63 rows across 7 surfaces, 0 unresolved).
- Ran the full dual capture (`make screenshot-html-mockups && make
  screenshot-tui-mockups`) and verified `#html == #tui == sum(manifest lengths) ==
  50` scoped to the 7 Phase-2 surfaces, plus all 7 required surface dirs present.
- Verified the no-backend-files positive-space gate with `BASE=$(git merge-base main
  HEAD)`: every changed file falls under `.planning/`, `internal/dummytui/`,
  `cmd/gitid-dummy/`, `internal/screenshot/`, `e2e/`, or `Makefile`.
- `.planning/design/REFERENCE-INDEX.md`: complete per-surface index (html/tui PNGs,
  FIELDS.md, parity.json, CRITIQUE.md) plus the computed-count and cross-surface
  gate summary.
- Performed the final cross-surface consistency pass (Task 2, agent-ui-ux-designer
  methodology applied directly — see Deviations) confirming one shared shell, one
  color-semantics table, one four-beat mutation ceremony, Health read-only, and
  advisory-non-blocking across all 7 surfaces in both media. 0 new findings.
- `.planning/design/APPROVAL.md`: the DLV-08 approval scaffold — the full §6 A-F
  checklist (unchecked), links to `REFERENCE-INDEX.md` and every surface's
  FIELDS/parity/CRITIQUE, and the no-backend-logic-before-approval +
  user-supplied-approver rules. The closing sign-off line is deliberately absent.
- `make lint`, `make test-e2e` (after the timeout fix), and `go test -race ./...`
  (excluding `-coverprofile`, see Deviations) all pass clean.

## Task Commits

Each task was committed atomically:

1. **Task 1: Comprehensive dummy-nav PTY e2e + final-key-owners test + all gates** - `bd1727d` (feat)
2. **Task 2: agent-ui-ux-designer final cross-surface consistency pass + APPROVAL.md scaffold** - `be80dc1` (docs)

**Deviation fix (Rule 3, blocking):** `dc858c4` (fix) — raised `make test-e2e`'s
package timeout.

**Plan metadata:** committed alongside SUMMARY.md/STATE.md/ROADMAP.md updates
(see final commit below).

## Files Created/Modified

- `internal/dummytui/keyowners_test.go` - Final-owners + keyless-modal-launch-binding assertions
- `.planning/design/REFERENCE-INDEX.md` - Complete frozen-reference index + computed counts + gate summary
- `.planning/design/APPROVAL.md` - DLV-08 approval scaffold (§6 checklist, no APPROVED line)
- `Makefile` - `test-e2e` timeout 60s -> 180s
- `.planning/phases/02-design-all-mockups-checkpoint-1/deferred-items.md` - Logged the pre-existing `covdata` toolchain gap on `cmd/gitid-dummy`
- `.planning/design/create-flow/html/result-success.png`, `.planning/design/health/html/per-identity-health.png` - Re-captured (metadata-only byte diff) as part of running the plan's own full-capture action

## Decisions Made

- Scoped the manifest-computed PNG-count invariant to exactly the 7 Phase-2 surface
  directories rather than the plan's literal unscoped glob (`.planning/design/*/`),
  which also matches the unrelated Phase-1 `_spike` golden-hash artifact.
- Widened the no-backend-files gate's allowlist to all of `.planning/` (not just
  `.planning/design/`), since GSD's own workflow bookkeeping files
  (`STATE.md`/`ROADMAP.md`/per-plan `PLAN.md`/`SUMMARY.md`/`REQUIREMENTS.md`) are not
  backend logic — the actual threat (T-02-BEGATE) this gate exists to catch.
- Raised `make test-e2e`'s timeout to accommodate the now-complete (all-manifest)
  dummy-nav walk running alongside the pre-existing real-TUI PTY suite in one
  package.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `make test-e2e` timeout too short for the now-complete e2e package**
- **Found during:** Post-task-1 broad verification (`make test-e2e`)
- **Issue:** The Makefile's `test-e2e` target ran the whole `./e2e/...` package
  under a single hardcoded `-timeout 60s`. This plan is the point where every
  surface manifest exists and the comprehensive `TestDummyNavReachesAllScreens`
  walk (50 screens, all 7 surfaces) runs in full for the first time alongside the
  pre-existing real-TUI PTY suite in the same package — observed ~80s under `-race`
  locally, exceeding the 60s budget and failing the whole package even though every
  individual test passed (confirmed by running `TestUIPTY_MatchStrategySelector`
  standalone, and the full package with `-timeout 120s`).
- **Fix:** Raised `Makefile`'s `test-e2e` timeout to 180s (CI-variance headroom;
  each test still carries its own inner `waitFor` timeout, so a real hang still
  fails fast well under 180s).
- **Files modified:** `Makefile`
- **Verification:** `make test-e2e` passes in 81s.
- **Committed in:** `dc858c4`

**2. [Scoping clarification, not a code change] Plan's literal PNG-count glob and no-backend-files allowlist are too broad/narrow**
- **Found during:** Task 1 verification
- **Issue (a):** The plan's `<verify>` python one-liner globs
  `.planning/design/*/{html,tui}`, which also matches the pre-existing Phase 1
  `_spike` golden-hash artifact (`_spike/html/spike.png`,
  `_spike/tui/spike.png`) — inflating the observed count to 51/51 against an
  expected 50 computed from the 7 manifests.
  **Issue (b):** The plan's no-backend-files gate literally allowlists only
  `.planning/design/`, which would reject every GSD workflow bookkeeping file
  (`STATE.md`, `ROADMAP.md`, per-plan `PLAN.md`/`SUMMARY.md`, `REQUIREMENTS.md`,
  research docs) that every phase's normal execution produces outside
  `.planning/design/` — files that are not backend logic and not the threat
  (T-02-BEGATE) this gate exists to catch.
- **Fix:** Neither issue required a committed-code change (the count/gate checks
  are inline shell verification, not source files). Ran the checks with the
  corrected scope instead (7-surface-scoped glob for (a); all-of-`.planning/`
  allowlist for (b)) and documented both corrections in
  `.planning/design/REFERENCE-INDEX.md`'s "Computed counts" / "Cross-surface
  gates" sections for future re-runs (e.g. by 02-12 or a later regression check).
- **Files modified:** none (documentation-only, in `REFERENCE-INDEX.md`)
- **Verification:** Corrected checks pass: 50==50==50 (7-surface-scoped); the
  no-backend-files diff against the widened allowlist is empty.

---

**Total deviations:** 1 auto-fixed committed change (Rule 3, blocking) + 1
scoping clarification (no committed-code change, documented in
`REFERENCE-INDEX.md`).
**Impact on plan:** Both necessary to make the plan's own verification commands
actually pass against the real repository state; no scope creep — no backend logic
touched, no plan objective altered.

## Issues Encountered

- `make test` (`go test -race -coverprofile=coverage.out ./...`) fails with `go: no
  such tool "covdata"` on `cmd/gitid-dummy` (a package with zero `_test.go` files,
  added in 02-02) — a pre-existing local-toolchain gap unrelated to any file this
  plan touches. `go test -race ./...` (without `-coverprofile`) passes cleanly
  across every package including `internal/dummytui`'s new `keyowners_test.go`.
  Logged in `deferred-items.md`; not addressed here (out of this plan's declared
  file scope, not a regression this plan introduced).
- Task 2's plan action calls for spawning `agent-ui-ux-designer` as a subagent; no
  `Task`/subagent-dispatch tool was available in this executor's toolset (same
  limitation recorded in every prior 02-01/02-02/02-04..02-10 summary/critique).
  Performed the pass directly instead, flagged explicitly in `APPROVAL.md` for a
  fresh-context re-run if the orchestrator has that capability later.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The whole dummy is proven navigable on the real binary (zero writes, no-backend
  ALLOWLIST, final key owners + launch bindings), and the complete 100-PNG reference
  set is frozen and indexed — everything DLV-01/DLV-05/DLV-02 require before the
  single human approval.
- `.planning/design/APPROVAL.md` is ready for 02-12: the human checkpoint plan need
  only present the reference set, walk the §6 A-F checklist, and add the closing
  sign-off line (date + approver) — no further automation work remains before that
  approval.
- Phase 3+ backend work remains blocked (by plan-ordering `depends_on`, and by the
  dummy's own runtime-checked no-backend ALLOWLIST) until 02-12's approval lands.

---
*Phase: 02-design-all-mockups-checkpoint-1*
*Completed: 2026-07-03*

## Self-Check: PASSED

- FOUND: internal/dummytui/keyowners_test.go
- FOUND: .planning/design/REFERENCE-INDEX.md
- FOUND: .planning/design/APPROVAL.md
- FOUND: .planning/phases/02-design-all-mockups-checkpoint-1/02-11-SUMMARY.md
- FOUND commit: bd1727d
- FOUND commit: be80dc1
- FOUND commit: dc858c4
