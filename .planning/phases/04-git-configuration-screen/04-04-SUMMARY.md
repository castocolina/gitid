---
phase: 04-git-configuration-screen
plan: 04
subsystem: testing
tags: [pty, tui, bubbletea, visual-regression, e2e, gate]

# Dependency graph
requires:
  - phase: 04-03
    provides: reusable git-form flow, SSH-only completion/edit paths, all-or-nothing rollback ceremony
provides:
  - Real-PTY coverage of every Configure-Git state through the compiled cmd/gitid binary (Task 1)
  - TestGitConfiguration_CompiledRealVsLiveDummyPTY — a semantic, real-vs-dummy PTY comparison gate (Task 2)
  - A Phase 4 git-screen ScreenSpec registry merged into the in-process visual-regression gate (Task 3)
  - .planning/design/git-screen/visual-divergence-allowlist.txt — the shared classification file both gates consume
affects: [phase-05-identity-manager, phase-07-global-git-options]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 8277
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "Semantic PTY comparison: normalize (identity token/email/gitdir path/timestamp placeholders) then compare structurally, never raw bytes"
    - "Two independent registries (create-flow, git-screen) merged into one ScreenSpecRegistry(), captured against SEPARATE seeded HOMEs to avoid fixture cross-contamination"
    - "RegionDisposition classifications embedded directly in Go (matches the create-flow precedent) rather than parsed at gate-run time from the .txt allowlist; the .txt allowlist is the PTY-level (Task 2) source of truth and a human-readable mirror of the Go dispositions"

key-files:
  created:
    - .planning/design/git-screen/visual-divergence-allowlist.txt
  modified:
    - e2e/git_configuration_pty_e2e_test.go
    - internal/screenshot/createflow.go
    - internal/screenshot/createflow_regions.go
    - internal/screenshot/createflow_test.go
    - cmd/gitid/gate_visual_regression_test.go
    - Makefile

key-decisions:
  - "Git-screen checkpoints are captured against a SEPARATE, dedicated temp HOME from create-flow's own capture pass — an initial design that shared one HOME was reverted after it shifted the wizard's sidebar layout and broke an unrelated create-flow screen's connectivity-output region (empirically discovered, not assumed)"
  - "Git-screen fixture identities are named \"gscreen\"/\"gscreenssh\", never \"acme\" — create-flow's wizard hardcodes \"acme\" as its own default alias prefix; reusing it would collide with the wizard's own alias-collision guard"
  - "RegionDisposition classifications live in Go (createflow.go's gitScreenSpecs()), mirroring the exact same CTX-D-NN citations and reasons as the .txt allowlist — the in-process gate consumes the Go dispositions (matching the create-flow precedent's actual, working mechanism); the .txt allowlist is Task 2's real-PTY gate's own source of truth and a transparent human-readable record of the same classifications"
  - "validDecisionRef generalizes decision-ref validation to accept bare D-NN/T-NN (create-flow's own vocabulary) alongside scoped CTX-D-NN/UI-D-NN (git-screen's vocabulary) — resolving the 04-CONTEXT.md/04-UI-SPEC.md D-NN namespace collision the plan's authority note calls out"

patterns-established:
  - "Region-scoped visual-regression comparison generalizes cleanly to a second surface (Configure-Git) by adding new named regions (git-form-fields/git-strategy/git-preview/git-ceremony) to the SAME ExtractRegion/AllRegionNames machinery create-flow already established"

requirements-completed: [GITUI-01, GITUI-02, GITUI-03, GITUI-04, GITUI-05, DLV-04, DLV-06]

coverage:
  - id: D1
    description: "Real PTY tests drive every approved Git-screen state in the compiled real binary at 100x30 — keyboard, mouse, create, edit, success, and rollback failure paths (Task 1, prior session)"
    requirement: DLV-06
    verification:
      - kind: e2e
        ref: "e2e/git_configuration_pty_e2e_test.go#TestGitConfiguration_RealPTYCompleteEditFlow, TestGitConfiguration_RealPTYSSHOnlyCompletionFlow, TestGitConfiguration_RealPTYWriteFailureRollback, TestGitConfiguration_RealPTYMouseFieldFocus"
        status: pass
    human_judgment: false
  - id: D2
    description: "A compiled-real-vs-live-dummy Configure-Git PTY comparison classifies every field/label/control/default/ceremony-beat difference as ux-improvement or defect"
    requirement: DLV-04
    verification:
      - kind: e2e
        ref: "e2e/git_configuration_pty_e2e_test.go#TestGitConfiguration_CompiledRealVsLiveDummyPTY"
        status: pass
    human_judgment: false
  - id: D3
    description: "The repeatable in-process visual gate registers all Phase 4 git-screen states alongside the existing create-flow registry, fails closed on missing states/stale classifications/cross-registry decision-ref leakage"
    requirement: DLV-04
    verification:
      - kind: unit
        ref: "cmd/gitid/gate_visual_regression_test.go#TestGateVisualRegression, TestNegativeControl_MissingGitScreenState, TestNegativeControl_StaleGitScreenClassification, TestNegativeControl_CrossRegistryLeakage"
        status: pass
    human_judgment: false

duration: ~90min
completed: 2026-08-24
status: complete
---

# Phase 4 Plan 4: Git-Configuration PTY Coverage + Real/Dummy Semantic Comparison Summary

**Real-vs-dummy Configure-Git PTY comparison (raw keystrokes, both compiled binaries) plus an in-process Phase 4 registry merged into the repeatable visual-regression gate — zero unclassified/defect divergences, no HTML/MUI/browser/PNG parity anywhere.**

## Performance

- **Duration:** ~90 min (this session — Tasks 2 and 3; Task 1 was completed in a prior session)
- **Tasks:** 3 (Task 1 prior session, Tasks 2–3 this session)
- **Files modified:** 15 (6 code/test files touched this session + 4 refreshed PTY evidence snapshots from Task 1 re-runs + 1 new allowlist file, across both sessions' commits)

## Accomplishments

- **Task 1 (prior session, commit `51c7421`):** `TestGitConfiguration_RealPTYCompleteEditFlow`, `TestGitConfiguration_RealPTYSSHOnlyCompletionFlow`, `TestGitConfiguration_RealPTYWriteFailureRollback`, and `TestGitConfiguration_RealPTYMouseFieldFocus` — all 4 real-PTY Git-configuration cases pass, resolving 3 findings from the prior RED commit (see `51c7421`'s own message: "fix(04-04): resolve 3 real-PTY Git-configuration findings").
- **Task 2 (`fe48a7b`):** `TestGitConfiguration_CompiledRealVsLiveDummyPTY` drives two REAL PTY sessions — the compiled `cmd/gitid` binary (seeded via `seedGitPTYIdentity`/`removeGitSide`) and the compiled `cmd/gitid-dummy` binary (its own frozen `personal`/`work` fixture identities) — across 5 semantic checkpoints (git-form-filled, git-form-empty, match-strategy-select, review-readonly, result-success). Each checkpoint is split into 6 named regions and compared after normalization (identity token / email / gitdir path / timestamp placeholders); real divergences are classified in `.planning/design/git-screen/visual-divergence-allowlist.txt`.
- **Task 3 (`51226ed`):** Extended `internal/screenshot`'s `ScreenSpecRegistry()`, `RegionName`/`AllRegionNames()`, and capture helpers with a git-screen registry (`CaptureGitScreenScreens`, `gitScreenSpecs()`), merged into `cmd/gitid/gate_visual_regression_test.go`'s in-process gate. Generalized `validDecisionRef` to accept both create-flow's bare `D-NN`/`T-NN` and git-screen's scoped `CTX-D-NN`/`UI-D-NN`. Added 3 negative controls (missing state, stale classification, cross-registry leakage), the leakage control hand-verified.
- Both gates classify divergences against the SAME allowlist file's reasoning (the .txt file is Task 2's real-PTY source of truth and a human-readable mirror; the in-process gate consumes the identical classifications embedded as Go `RegionDisposition`s).

## Task Commits

1. **Task 1: Drive all Git states in the compiled real TUI** — `0627b75` (test, RED) → `c4482ee` (test, RED + 3 findings) → `51c7421` (fix, all 4 cases GREEN) — completed in a **prior session**, not this one.
2. **Task 2: Compare compiled real and live dummy PTY workflows** — `fe48a7b` (test)
3. **Task 3: Register the Phase 4 workflow in the repeatable visual gate** — `51226ed` (feat)

**Plan metadata:** this commit (docs: complete plan)

## Files Created/Modified

- `.planning/design/git-screen/visual-divergence-allowlist.txt` — new; strict 5-field schema (`checkpoint:region:predicate:decision-ref:reason`), CTX-D-NN decision refs, consumed by Task 2's e2e test parser (`loadGitScreenAllowlist`/`splitGitScreenAllowlistLine`)
- `e2e/git_configuration_pty_e2e_test.go` — `TestGitConfiguration_CompiledRealVsLiveDummyPTY` and its region-extraction/normalization/allowlist-parsing helpers (~612 new lines)
- `internal/screenshot/createflow_regions.go` — 4 new `RegionName` constants (git-form-fields, git-strategy, git-preview, git-ceremony) + extractors
- `internal/screenshot/createflow.go` — `CaptureGitScreenScreens`, `gitScreenSpecs()`, `isGitScreenID`, `validDecisionRef`; `longDigitRunPattern` added to `normalizeTimestamps`
- `cmd/gitid/gate_visual_regression_test.go` — `deterministicGitIdentityFixture`, `mergeGitScreenCaptures`, 3 new negative-control tests
- `internal/screenshot/createflow_test.go` — `captureCombined` helper wired into the 3 registry-completeness tests
- `Makefile` — `gate-visual-regression`/`test-e2e` doc comments state their Phase 4 relationship explicitly; `test-e2e` timeout raised 180s → 360s

## Decisions Made

See `key-decisions` in the frontmatter above. In short: git-screen captures use an isolated HOME (not shared with create-flow's own capture pass), fixture identities avoid the wizard's own "acme" default, and classifications live in Go (matching the create-flow precedent) while the `.txt` allowlist stays the real-PTY gate's authoritative, human-readable source.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `extractConnectivityOutput`'s bare "ssh " marker false-positived on unrelated content**
- **Found during:** Task 3, first `TestGateVisualRegression` run against the merged registry
- **Issue:** `internal/screenshot/createflow_regions.go`'s pre-existing `extractConnectivityOutput` triggered on the substring `"ssh "`, which also appears inside the git-form's unrelated `"gpg.format=ssh "` metadata line. Once triggered it never closed (no `"Esc returns"` line follows on the git-form-demo screen), so it captured the REST of that screen's content, producing a spurious real-vs-dummy divergence on a region with no actual test-stage content.
- **Fix:** Narrowed the marker to `"ssh -"` (the actual command-invocation prefix, e.g. `"ssh -T -F ..."`), which never appears in the unrelated metadata line.
- **Files modified:** `internal/screenshot/createflow_regions.go`
- **Verification:** `TestGateVisualRegression` passes cleanly on the create-flow `git-form-demo` screen with no false positive.
- **Committed in:** `51226ed` (Task 3 commit)

**2. [Rule 1 - Bug] `normalizeTimestamps` didn't normalize nanosecond-suffixed backup paths, breaking CR-01 determinism**
- **Found during:** Task 3, `TestGateVisualRegression`'s two-run determinism check on the new `result-success` checkpoint
- **Issue:** The real binary's actual commit backups use `internal/filewriter`'s `".bak.<unix-nanoseconds>"` suffix (distinct from `tuikit.NewBackupPath`'s ISO-8601 `".backup."` convention, which `timestampPattern` already normalized). The 19-digit nanosecond value also wraps mid-number across the fixed 100-column pane, splitting into two independent digit runs in the rendered text.
- **Fix:** Added `longDigitRunPattern` (`\d{6,}` → `<digits>`) to `normalizeTimestamps`, which normalizes each wrapped fragment independently (a single contiguous-match pattern would have missed the wrap).
- **Files modified:** `internal/screenshot/createflow.go`
- **Verification:** Two independent `CaptureGitScreenScreens` runs against the real backend now produce byte-identical `result-success` text; `TestGateVisualRegression`'s CR-01 determinism check passes.
- **Committed in:** `51226ed` (Task 3 commit)

**3. [Rule 4 → resolved without a design change - Architectural false alarm] Sharing the create-flow HOME for git-screen captures**
- **Found during:** Task 3, an early implementation attempt that merged `CaptureGitScreenScreens` INTO `CaptureCreateFlowScreens` against the SAME backend/HOME
- **Issue:** Seeding the create-flow capture's HOME with the two git-screen fixture identities shifted the wizard's sidebar layout (more identities → different column widths → different line-wrap points throughout the pane), breaking an UNRELATED create-flow screen's `connectivity-output` region with a spurious divergence.
- **Fix:** Reverted the merge; git-screen captures now run against a SEPARATE, dedicated temp HOME (`cmd/gitid/gate_visual_regression_test.go`'s `mergeGitScreenCaptures`), explicitly merged into the caller's capture maps AFTER both capture passes complete independently. No architectural change to the registry model itself — `ScreenSpecRegistry()` still merges both registries' SPECS (structural), only the CAPTURE step stays isolated per surface's own precondition.
- **Files modified:** `internal/screenshot/createflow.go`, `cmd/gitid/gate_visual_regression_test.go`
- **Verification:** `TestGateVisualRegression` passes with both registries' screens present and no cross-contamination.
- **Committed in:** `51226ed` (Task 3 commit)

---

**Total deviations:** 3 auto-fixed (2 pre-existing bugs discovered while editing files already in Task 3's scope, 1 design correction made during implementation before any commit). No scope creep — all three were required for the merged registry to produce correct, deterministic evidence.

## Deferred Issues

**Real commit receipt's backup-path naming diverges from the rest of the UI's convention.** The real binary's actual `CommitGit` transaction (`internal/filewriter`) produces backup paths suffixed `.bak.<unix-nanoseconds>`, while every OTHER backup-path promise in the UI (the ceremony's pre-confirm ".Backups" declaration, the dummy's mock receipt) uses `tuikit.NewBackupPath`'s ISO-8601 `.backup.<timestamp>` convention. Both are correct, real backups — only the DISPLAY FORMAT differs between the "promise" (pre-write) and "receipt" (post-write) for the real binary specifically. This is classified as a real-vs-dummy `ux-improvement` divergence in this plan's scope (the real receipt is more complete, listing every backup actually taken); the internal naming-convention inconsistency itself is out of THIS task's scope (a pre-existing product behavior, not introduced by Task 2 or Task 3's changes) and is flagged here for a future cleanup pass, not auto-fixed.

## Known Stubs

None — no stub or placeholder content was introduced by this plan.

## Threat Flags

None — no new network endpoint, auth path, or trust-boundary-crossing file access was introduced. All new code is test/gate infrastructure operating on hermetic sandbox HOMEs.

## Gate Results

All final gates run and confirmed GREEN this session:

| Gate | Command | Result |
|---|---|---|
| Build | `TERM=dumb SSH_AUTH_SOCK= go build ./...` | exit 0 |
| Unit + race | `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` | 1028 passed, 19 packages |
| Lint | `make lint` | 0 issues |
| E2E | `TERM=dumb SSH_AUTH_SOCK= make test-e2e` | ok, 256.3s (includes the new `TestGitConfiguration_CompiledRealVsLiveDummyPTY`) |
| Visual regression | `TERM=dumb SSH_AUTH_SOCK= make gate-visual-regression` | ok — 23 `RequiredScreenSpecs` frames checked as a classified real/dummy symmetric union (18 create-flow + 5 git-screen) |

**Task 2's own verify command**, run independently: `TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -race -count=1 -timeout 180s ./e2e -run '^TestGitConfiguration_CompiledRealVsLiveDummyPTY$'` — PASS, all 5 subtests (git-form-filled, git-form-empty, match-strategy-select, review-readonly, result-success).

**No HTML, MUI, browser, Chromium, pixel, PNG-byte, or historical-artifact parity participated in Phase 4 acceptance**, per D-12. Confirmed by inspection: neither Task 2's e2e test nor Task 3's registry additions import or invoke any browser/Chromium/PNG-capture code path; every git-screen `ScreenSpec` sets `ApplicableApprovedHTML: false` with an explicit `NonApplicability` record citing CTX-D-12 ("cmd/gitid-dummy is the sole Phase 4 UI/UX reference — no HTML/MUI/browser capture participates in Phase 4 acceptance").

**Zero unclassified/defect differences in the divergence registry.** All 8 real-vs-dummy divergences discovered across the 5 git-screen checkpoints (sidebar/header-status fixture differences, gitdir-default derivation, author-name-template, breadcrumb/git-strategy identity-name substitution, sentinel-wrapped ceremony preview, backup-receipt completeness) are classified `ux-improvement` — none is a `defect`. `TestGateVisualRegression` confirms every non-equal region carries a valid classification; `TestNegativeControl_StaleGitScreenClassification` proves the check is not vacuous (a mutation IS caught).

## Issues Encountered

See "Deviations from Plan" above — the two auto-fixed pre-existing bugs (`extractConnectivityOutput`'s marker false-positive, `normalizeTimestamps`'s missing long-digit-run normalization) and the HOME-isolation design correction were all discovered and resolved iteratively while bringing `TestGateVisualRegression` to green, following the project's hypothesis → test → implementation discipline (each fix was proven by a real, observable before/after test run, not assumed).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Phase 4's Git Configuration Screen is now fully proven through both a real-keystroke PTY comparison (DLV-06) and a repeatable in-process visual gate (DLV-04), matching the project's established two-gate pattern from Phase 3's create-flow work.
- The git-screen registry's pattern (isolated fixture HOME, `gscreen`/`gscreenssh` identity naming to avoid wizard-default collisions, Go-embedded `RegionDisposition`s mirroring a human-readable `.txt` allowlist) is directly reusable by Phase 5's Identity Manager, which reuses the SAME `internal/tuikit` Configure-Git flow in edit mode from its own `g`-launch entry point (per `04-CONTEXT.md`'s code_context "Integration Points").
- No blockers. The deferred backup-path-naming-convention inconsistency (see "Deferred Issues") is a candidate for a future Phase 8 (Health + Fixer) or dedicated cleanup pass — it affects display consistency only, not correctness.

---
*Phase: 04-git-configuration-screen*
*Completed: 2026-08-24*

## Self-Check: PASSED

- FOUND: `e2e/git_configuration_pty_e2e_test.go`
- FOUND: `.planning/design/git-screen/visual-divergence-allowlist.txt`
- FOUND: `internal/screenshot/createflow.go`
- FOUND: `internal/screenshot/createflow_regions.go`
- FOUND: `internal/screenshot/createflow_test.go`
- FOUND: `cmd/gitid/gate_visual_regression_test.go`
- FOUND: `Makefile`
- FOUND commit: `51c7421` (Task 1, prior session)
- FOUND commit: `fe48a7b` (Task 2)
- FOUND commit: `51226ed` (Task 3)
