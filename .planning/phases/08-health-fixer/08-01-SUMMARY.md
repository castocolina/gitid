# 08-01 SUMMARY — Tracer + full split: doctor.Run() convergence, Health/Fixer, `gitid health`

## Outcome

All 3 tasks complete. `internal/doctor.Run(deps)` is now the SOLE findings
source for the Health tab, the Fixer tab, and `gitid health --json` — the
retired `identity.Problem`-to-`DemoFinding` synthesis loop is gone;
`identity.BuildInventory`'s `Problem` taxonomy survives only as an INPUT to
`doctor.Deps`. `TabID` carries 5 values (`Identities`, `GlobalSSH`,
`GlobalGit`, `Health`, `Fixer`) with Health (read-only, all findings) and
Fixer (fix-ceremony, fixable findings only) as real, structurally distinct
screen models — no shared embedding, no cross-capability leakage.

This wave was executed as a **hand-recovery**: the cross-AI executor
(`opencode`/`local-llm-env`) completed Task 1 cleanly (committed `e52e5a6`),
then crashed mid-Task-2 after hitting a banned `/tmp`-write permission wall
while diagnosing a test failure. I (the orchestrator) read the crashed
agent's uncommitted diff, cross-referenced the plan's own `<action>`/`<done>`
blocks for what was done vs. missing, and finished Tasks 2 and 3 directly.

## Task 1 (already committed as `e52e5a6`)

- `doctor.Finding` gains `Target string` (`"SSH"`/`"Git"`); every
  `checks/*.go` literal sets it explicitly or via `defaultTargetForFamily`.
- `cmd/gitid/wiring.go`'s `buildDoctorDeps(home)` builds a real
  `doctor.Deps` from `realBackend`'s resolved fields; only `CheckCoherence`
  wired (Task 1's "one real finding" scope — `doctor.Run` skips nil
  `CheckFn`s by design).
- `doctorFindings(home)` is the sole `doctor.Finding` → `tuikit.DemoFinding`
  conversion site, living in `cmd/gitid` only — `internal/tuikit` imports
  zero `internal/doctor` identifiers.
- `TabID` split 4→5; `healthModel`/`fixerModel` initially wrapped the
  pre-split `doctorModel` unchanged (Task 2 completes the real split).
- `internal/screenshot/createflow_regions.go`'s header-region extractors
  (`extractHeader`, `extractHeaderStatus`) updated from the retired
  `"Doctor "` marker to `"Fixer "` — fixed a visual-regression gate
  false-positive where the header region silently absorbed the status chip
  once the marker stopped matching.
- `mouse_test.go`'s header-dead-space probe now derives its click column
  from `headerTabAt`/`headerChipAt` instead of a hardcoded offset.

## Task 2 (this session)

- **Full split**: `doctorModel` and its methods removed from
  `internal/tuikit/doctor.go` (now holds only shared helpers:
  `doctorScanMsg`, `doctorBatch`, `severityRank`, `orderedFindings`,
  `doctorGroup`, `groupFindings`, `selectFinding` — extracted as a free
  function since neither model owns the other, `fixableFindings`,
  `pluralS`). `internal/tuikit/health_screen.go`'s `healthModel` is now an
  independent, read-only model: navigation only, no `f`/`F` key handling,
  no "Fix this…" action line or click target, no `fixing`/`batch`/`ceremony`
  fields at all. `internal/tuikit/fixer_screen.go`'s `fixerModel` carries
  the full fix-ceremony logic (moved, not delegated), scoped to
  `fixableFindings(orderedFindings(s))` via an internal `fixableState(s)`
  narrowing applied at the top of every handler (App always passes raw,
  unfiltered `DemoState` to every screen — confirmed by reading `app.go`'s
  dispatch sites).
- **All 9 `CheckFn` families wired** in `cmd/gitid/wiring.go`'s
  `buildDoctorDeps`: `CheckDeps`, `CheckPerms` (→ `checks.CheckPermissions`
  — name mismatch, not a typo), `CheckCoherence`, `CheckOrphans`,
  `CheckSigning`, `CheckAgent`, `CheckBaseline`, `CheckOverlap`,
  `CheckRedundancy`.
- **`cmd/gitid/health.go` created** — Task 1's plan said to wire this but
  never actually created the file or registered the command; `main.go:105`
  still routed `health` through the reserved-noun stub. Fixed: `gitid
  health [--json]` now calls the same `buildDoctorDeps`/`doctorFindings`
  construction the TUI consumes. `docs/cli-parity-matrix.md`'s Health row
  updated from `deferred Phase 8` to `shipped` (HLTH-01/03/06); the
  `--json` shape is explicitly documented as provisional (Wave 7
  supersedes it with the versioned-envelope convention).
  `cmd/gitid/identity_test.go`'s `TestReservedNounGroupsReturnPhaseNamedErrors`
  updated to drop `"health"` from its reserved-noun cases (it's a real
  command now).

### Real bug found and fixed (not hypothesized — empirically verified)

Manually running `gitid health --json` against a real fixture (a managed
identity whose `includeIf` fragment file is missing) showed **two** Coherence
findings for the identical root cause: `CheckCoherence`'s `Incomplete`
branch ("identity … incomplete — missing fragment-file") AND its separate
fragment-existence check ("includeIf fragment … does not exist") both fired,
because `identity.Reconstruct` (`internal/identity/loader.go`) already marks
the account `Incomplete = "fragment-file"` when `readFrag` reports the file
missing, but `coherenceForAccount`'s Check 2 re-checked the same file via
`deps.Stat` unconditionally. This directly contradicted Task 1's own `<done>`
contract ("never zero, never two"). Fixed in
`internal/doctor/checks/coherence.go`: Check 2 is now skipped when
`acct.Incomplete` already contains `"fragment-file"`. Verified before/after
with the exact real fixture (see `TestCoherenceMissingFragmentReportsExactlyOnce`
below) and confirmed the fix does not regress the ordinary case where a
fragment exists and simply drifts.

### Stale "Doctor" text — cleanup (4 rounds, driven by real e2e failures)

The TabID split renamed the user-facing tab from "Doctor" to "Health"/
"Fixer", but text elsewhere in the codebase still referenced the old name.
Each round below was caught by an actual test failure, not by a static
sweep alone — the final "no more matches" grep only followed after `make
test-e2e` genuinely passed:

1. `internal/tuikit/globalssh.go`'s `findingsBanner` (shared by Global SSH
   and Global Git) — `"Open Doctor (4)"` → `"Open Health (4)"`.
2. `internal/tuikit/identities.go`'s per-identity Findings sub-panel —
   `"same data the Doctor shows (4)"` → `"same data Health shows (4)"`,
   `"Open the Doctor (4)"` → `"Open Health (4)"`; its unit test
   (`internal/tuikit/identities_test.go`) updated to match.
3. `e2e/ui_pty_e2e_test.go` (nav-tab list + comments), `e2e/dummy_demo_e2e_test.go`
   (three separate spots: launch-time `"[4] Doctor"` tab assertion, the
   mouse-click-target search + its "Health only diagnoses" status
   assertion → `"read-only diagnostics"`, and the fix-ceremony flow which
   pressed `"4"` then `"f"` — Health has no `f`/`F` handling anymore, so
   this now presses `"5"` (Fixer) instead), `e2e/git_configuration_pty_e2e_test.go`
   and `e2e/identity_manager_pty_e2e_test.go`'s header-status region
   extractors (`"Doctor"` marker → `"Fixer"`, the new last nav tab — this
   was the root cause of `TestIdentityManager_CompiledRealVsLiveDummyPTY`'s
   "allowlist entry never triggered" failures: the stale marker made
   `header-status` always extract as empty, so no real divergence was ever
   exercised to trigger the allowlist entries), `e2e/global_git_pty_e2e_test.go`'s
   post-navigation `mustSee(..., "Doctor", ...)` → `"Health"`.

A final repo-wide grep across `internal/tuikit/*.go`, `cmd/gitid/*.go`,
`internal/dummytui/*.go`, `e2e/*.go` confirms no remaining user-visible
`"Doctor"` string outside doc comments, internal identifier names
(`buildDoctorDeps`, `runDoctorSSHAdd`), and two negative-control tests'
arbitrary synthetic-frame filler text (`identity_manager_pty_e2e_test.go`'s
`TestIdentityManager_AllowlistUnclassifiedDifferenceRejected`/
`TestIdentityManager_AllowlistStaleEntryRejected`, which fabricate literal
frame strings unrelated to real render output).

## Task 3 — guard tests

- `cmd/gitid/wiring_test.go`:
  - `TestDoctorFindingsAlwaysHaveTarget` — real multi-family fixture (loose
    `~/.ssh` perms, duplicate `Host *`, missing baseline include); asserts
    every finding's `Section` (Target) is exactly `"SSH"` or `"Git"`, never
    empty, and that Permissions/Baseline/Redundancy all genuinely fired.
  - `TestCoherenceMissingFragmentReportsExactlyOnce` — the two-pipeline
    convergence regression test; the exact real fixture that caught the
    coherence.go bug above, now a permanent regression guard.
  - `TestMGR07TwoSignalsResolveFromConvergedSource` — proves MGR-07's
    per-identity badge is two signals: `row.State` (glyph, inventory-only,
    non-"complete" for a genuinely incomplete fixture) and
    `FindingsFor(state, name)`'s count (flag), asserted equal to the
    converged per-identity finding count (not any prior pipeline's count).
- `internal/tuikit/app_test.go`: `TestNewScreensExhaustiveSwitchOverTabID`
  — asserts `newScreens` returns exactly one `screenModel` of the correct
  concrete type per `TabID` value, in order (closes the coverage gap the
  plan named — no prior test iterated every `TabID`).

## Gates (every command run for real by the orchestrator, output inspected —
none trusted from a self-report)

- `go build ./...` — exit 0.
- `go vet -tags e2e ./...` — clean.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — **2046 passed**,
  0 failed, 21 packages.
- `GOLANGCI_LINT_CACHE=$PWD/.golangci-cache make lint` (cache removed after)
  — **0 issues** (fixed 2 real findings along the way: an unused
  `allowedSignersPath` parameter in `doctorAddWiring` traced to a genuine
  bug — the `"signers:"` case was writing to the generic per-call `path`
  instead of the closed-over `allowedSignersPath`, fixed; an unused
  `newDoctorModel` after the split, removed; an unused-parameter revive
  finding in `healthModel.handleClick`, renamed to `_`; a gosec G301 on a
  deliberately-loose-permissions test fixture, `nolint`-annotated with
  rationale).
- `go list -deps ./internal/tuikit` — no `internal/doctor` import (no-backend
  gate holds).
- `make gate-visual-regression` — PASS, 41 `RequiredScreenSpecs` frames,
  ~55s.
- `make test` (includes `gate-copy-freeze`) — PASS.
- `make test-e2e` — **PASS**, 597.1s, after 4 rounds of real fixes (see
  above); every failure this wave hit was a genuine regression from the
  TabID split rippling into e2e key-sequences/copy assertions the plan's
  own file list did not anticipate, never a flake.
- Manual `gitid health --json` runs against real, deliberately-misconfigured
  throwaway homes (`newBackendForHome`-equivalent via a real built binary)
  — proved the Redundancy finding (duplicate `Host *`), the Baseline
  finding (missing `[include]`), the Permissions finding (loose `.ssh`
  mode), and the Coherence single-finding-not-double behavior, all
  end-to-end through the real CLI binary, not just unit tests.

## Files modified

`internal/doctor/doctor.go`, `internal/doctor/checks/coherence.go`,
`cmd/gitid/wiring.go`, `cmd/gitid/main.go`, `cmd/gitid/health.go` (new),
`cmd/gitid/identity_test.go`, `cmd/gitid/wiring_test.go`,
`docs/cli-parity-matrix.md`, `internal/tuikit/frame.go`,
`internal/tuikit/frame_test.go`, `internal/tuikit/app.go`,
`internal/tuikit/app_test.go`, `internal/tuikit/doctor.go`,
`internal/tuikit/doctor_test.go`, `internal/tuikit/health_screen.go`,
`internal/tuikit/fixer_screen.go`, `internal/tuikit/globalssh.go`,
`internal/tuikit/identities.go`, `internal/tuikit/identities_test.go`,
`internal/tuikit/batch3_test.go`, `internal/tuikit/mouse_test.go`,
`internal/screenshot/createflow_regions.go`,
`internal/dummytui/fixturebackend.go`, `e2e/ui_pty_e2e_test.go`,
`e2e/dummy_demo_e2e_test.go`, `e2e/git_configuration_pty_e2e_test.go`,
`e2e/identity_manager_pty_e2e_test.go`, `e2e/global_git_pty_e2e_test.go`,
plus regenerated `.planning/phases/{06-global-ssh-options,07-global-git-options}/ui-frames/*.txt`
snapshots (legitimate content changes: 5-tab header, corrected advisory
text, real finding counts — not timestamp noise).

## Next Phase Readiness

Wave 2 (08-02-PLAN.md: flagship fix-in-place — hand-written-directive
contradiction check, D-09 surgical rewrite, D-10 verification loop, D-11
confirm, D-13 re-run-all, D-14 convergence alarm, `gitid fix` CLI) can
proceed. Recommend: after implementing, run a repo-wide grep for stale
references to anything renamed/split, and run the FULL gate battery
including `make test-e2e` (not just unit tests) before considering any
wave done — this wave's dominant lesson is that a rename/split ripples into
e2e key-sequences and copy assertions the plan's own file list does not
anticipate, and only a real end-to-end PTY run catches that class of bug.
