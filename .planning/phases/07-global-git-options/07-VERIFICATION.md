---
phase: 07-global-git-options
verified: 2026-08-28T01:36:11Z
status: passed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
resolved_gaps:
  - truth: "The Global Git visual-regression gate's negative controls genuinely fail when perturbed (07-06's own framing: 'four negative controls and the exact failure each proved'; task instruction #4)."
    status: resolved
    resolution: >
      Fixed in the same session, immediately after this report was generated.
      TestNegativeControl_GlobalGitUnclassifiedDifference was rewritten to build the FULL real
      classified diff set via screenshot.BuildRegionDiffs across the merged registry, strip the
      classification off an already-known-divergent Global Git region, and assert
      screenshot.ValidateRegionDiffs then rejects the mutated evidence — mirroring
      TestNegativeControl_GitScreenUnclassifiedDifferenceRejected's shape exactly.
      TestNegativeControl_GlobalGitPerturbedComparableRegion was rewritten to call the SAME shared
      exhaustive helper (assertAllComparableEqualRegionsAreMutationSensitive) the other three
      surfaces in this file already use, scoped to globalGitScreenIDs — closing the missing
      TestNegativeControl_AllGlobalGitComparableEqualRegionsAreMutationSensitive counterpart this
      report flagged, rather than adding it as a separate test. Both rewritten tests independently
      re-run and confirmed passing (go test -tags screenshot ./cmd/gitid/... -run
      'TestNegativeControl_GlobalGit(UnclassifiedDifference|PerturbedComparableRegion)'), and the
      full make gate-visual-regression target passes.
    reason: >
      Two of the four Global Git negative controls (TestNegativeControl_GlobalGitUnclassifiedDifference
      and TestNegativeControl_GlobalGitPerturbedComparableRegion, cmd/gitid/gate_visual_regression_test.go)
      never invoke the real gate-failure path (screenshot.BuildRegionDiffs -> screenshot.ValidateRegionDiffs).
      They capture real/dummy text, confirm a real divergence or a nil-disposition spec exists, and then
      only t.Logf() a claim that "the real gate" would reject it — the claim is asserted in a code comment
      and a log line, never exercised. This is a materially weaker pattern than the established one: Phase 6's
      sibling tests (TestNegativeControl_GlobalSSHUnclassifiedDifferenceRejected and
      TestNegativeControl_AllGlobalSSHComparableEqualRegionsAreMutationSensitive, same file) and Phase 4/5's
      equivalents all call BuildRegionDiffs, mutate a real classification/region, marshal it, and assert
      ValidateRegionDiffs actually returns a non-nil error. Phase 7 has no
      TestNegativeControl_AllGlobalGitComparableEqualRegionsAreMutationSensitive counterpart at all. The other
      two Global Git negative controls (TestNegativeControl_GlobalGitMissingState,
      TestNegativeControl_GlobalGitCrossSurfaceAllowlistLeakage) ARE rigorous — they call
      screenshot.ValidateCapturedState / inspect the real allowlist files and assert a real failure/absence.
      07-06-SUMMARY.md's claim "All four PASS as tests (the gate fails as designed when their perturbation
      runs)" overstates what two of the four tests actually prove.
    artifacts:
      - path: "cmd/gitid/gate_visual_regression_test.go"
        issue: "TestNegativeControl_GlobalGitUnclassifiedDifference (line 2155) and TestNegativeControl_GlobalGitPerturbedComparableRegion (line 2209) assert only precondition facts and log an unverified claim; neither calls screenshot.BuildRegionDiffs/screenshot.ValidateRegionDiffs to observe an actual rejection."
    missing:
      - "Rewrite TestNegativeControl_GlobalGitUnclassifiedDifference to call screenshot.BuildRegionDiffs on the real+dummy Global Git captures, strip/mutate the classification for a genuinely-differing comparable region (mirroring TestNegativeControl_GlobalSSHUnclassifiedDifferenceRejected), marshal, and assert screenshot.ValidateRegionDiffs returns a non-nil error."
      - "Add a TestNegativeControl_AllGlobalGitComparableEqualRegionsAreMutationSensitive counterpart (mirroring the Global SSH test of the same shape, scoped via globalGitScreenIDs) that actually mutates a captured region and asserts BuildRegionDiffs/comparison flips from equal to unequal, rather than asserting on raw header strings outside the gate pipeline."
deferred:
  - truth: "Global gitignore (core.excludesfile + managed pattern file) is managed by the Global Git surface."
    addressed_in: "Phase 8"
    evidence: "07-CONTEXT.md D-11 explicitly defers GITIGNORE-01 to Phase 8's fixer (FIX-01); REQUIREMENTS.md §J's GGIT-01 note was corrected in this phase (07-05, D-11.1) to name Phase 8 as the owner, and Phase 8's roadmap goal is 'Health + Fixer' with FIX-01/FIX-02 success criteria. Not a Phase 7 gap."
---

# Phase 7: Global Git Options Verification Report

**Phase Goal:** A developer manages shared Git config — default branch, line
endings, case, email, and recipe defaults — each option explained.
**Verified:** 2026-08-28T01:36:11Z
**Status:** passed (the single gap this report found was fixed in the same
session immediately after this report was generated — see `resolved_gaps` in
the frontmatter above for the fix)
**Re-verification:** No — initial verification

## Summary

This is a very thoroughly executed phase. Every functional truth I checked —
the 12-row policy table, the four-state classifier, the version gate, the TUI
screen (scrolling master list, apply ceremony, separate D9 fallback-author
ceremony), the CLI (`gitid git options list/apply`, `gitid git fallback
show/set`), the single write chokepoint (`runGlobalGitApply` /
`runGitFallbackAuthorApply`), and essentially every D-01…D-11 decision in
`07-CONTEXT.md` — is genuinely implemented in the codebase, not just claimed
in SUMMARY.md prose. I independently ran (not trusted) `go build ./...`, the
full `go test -race -count=1 ./...` suite (2035 tests, 21 packages, all
green), `make gate-copy-freeze`, `make gate-visual-regression`, and targeted
test runs, all matching 07-06-SUMMARY.md's exit-battery table.

The one finding below is narrow and does not undermine the shipped feature:
two of the Global Git visual-regression gate's four "negative controls" are
weaker than the SUMMARY claims and weaker than their own Phase 4/5/6 sibling
tests — they assert real preconditions but never actually exercise the gate's
failure path. Everything else verified cleanly.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A global-git-options screen manages `init.defaultBranch` (main-vs-master), `core.ignorecase`, `core.autocrlf`/`core.eol`, global `user.email` fallback, and recipe defaults (`push.autoSetupRemote`, `pull.rebase`, `fetch.prune`, aliases, color, `merge.conflictstyle`, `diff.colorMoved`) — each explained (GGIT-01) | ✓ VERIFIED | `internal/globalgit/policy.go`'s `Policy` var: 12 rows byte-matching 07-CONTEXT.md D-08's pinned table (incl. the D-07 `user.useConfigOnly` 12th row), correct `merge.conflictstyle` hard gate (`zdiff3`/`diff3` fallback below git 2.35) and informational gates (`init.defaultBranch` ≥2.28, `push.autoSetupRemote` ≥2.37, `diff.colorMoved` ≥2.15). `internal/tuikit/globalgit.go` renders each row's explanation, provenance, the main-vs-master chip, the APFS ignorecase caveat, version-gate notes, per-key "yours differs" notes, and the D-07 cross-warning. `TestGlobalGitFixturePolicyParity` and `TestGlobalGitRendersAllElevenRows`/`TestGlobalGitMainVsMasterHighlight` pass (directly re-run). |
| 2 | The four-state classifier (needs-action / already-set / set-but-differs / not-applicable) and the version-gate logic are real, not stubbed | ✓ VERIFIED | `internal/globalgit/classify.go` (367 lines, scalar + bundle branch logic, `sourceClassFor` provenance) and `version.go` (`VersionGate`/`WriteValueFor`/`RealGateForRow`, numeric-not-string version comparison). 64 tests in `internal/globalgit` pass directly re-run. |
| 3 | Changes write through the backup + idempotent managed-block chokepoint with confirmation; content outside managed blocks is preserved verbatim (GGIT-01) | ✓ VERIFIED | `cmd/gitid/lifecycle.go`'s `runGlobalGitApply` (plan→confirm→backup→write→verify, journal-backed rollback, unconditional backup once authorized — R-3) and `runGitFallbackAuthorApply` (single-file read→compose→one write, D-05/R-4 anchor-by-construction). `internal/gitconfig/globalgit.go`'s `EnsureGlobalGit` performs an ADDITIVE merge (R-2) and explicitly refuses `core.excludesfile` in any selection (D-11 demotion, confirmed live). Confirmed the pre-Phase-7 `WriteBaselineFile`/`WriteBaselineInclude` substrate (which unconditionally emitted `core.excludesfile`) has **zero live callers** outside tests — no second write path exists. |
| 4 | Both the TUI and the CLI reach disk ONLY through the shared lifecycle functions — no second write path | ✓ VERIFIED | `internal/tuikit/globalgit.go`'s `handleKey` dispatches `m.backend.CommitGlobalGit`/`CommitGitFallbackAuthor`; `cmd/gitid/wiring.go`'s `CommitGlobalGit`/`CommitGitFallbackAuthor` call `b.runGlobalGitApply`/`b.runGitFallbackAuthorApply` directly. `cmd/gitid/git.go`'s CLI verbs call the exact same two functions via `cliGlobalGitApplyInto`/`cliGitFallbackAuthorApplyInto`. Grep confirms these are the only two call sites of each `run*Apply` function outside tests. |
| 5 | The frozen `gitid git options list/apply` and `gitid git fallback show/set` CLI commands exist and are wired correctly (R-5 token discipline) | ✓ VERIFIED | `cmd/gitid/git.go` (799 lines): `newGitOptionsListVerb`/`newGitOptionsApplyVerb`/`newGitFallbackShowVerb`/`newGitFallbackSetVerb`, all four frozen JSON schema identifiers, `validateGitApplyTokens` rejects member keys by name before any I/O. `TestParityMatrixResolvesAndCoversTree` passes (directly re-run). |
| 6 | The TUI screen provides the scrolling master list, the apply ceremony, and a separate fallback-author ceremony (D-05: never folded into the baseline block) | ✓ VERIFIED | `internal/tuikit/globalgit.go` (1058 lines): `gitVisibleRowCount`/`scrollWindowFor`/`gitComputeScrollWindow` implement the measured (not hardcoded) scrolling window; `baselineCeremonyFor` and `fallbackCeremonyFor` are two structurally distinct ceremony builders, and `handleKey`'s ceremony-confirm branch dispatches to `CommitGlobalGit` or `CommitGitFallbackAuthor` based on the ceremony heading, never both. |
| 7 | D-06's post-write author-resolution precedence invariant is proven programmatically, not just asserted | ✓ VERIFIED | `internal/globalgit/authorresolve.go`'s `VerifyAuthorResolution` (real `git config --show-origin --get`, matched/unmatched directories) is wired into `runGitFallbackAuthorApply`'s verify stage (`cmd/gitid/lifecycle.go:1495`, `cmd/gitid/wiring.go:133`). |
| 8 | `.planning/REQUIREMENTS.md` GGIT-01 closure corresponds to real, runnable evidence, spot-checked directly (not trusted from SUMMARY.md) | ✓ VERIFIED | Re-ran, did not just read the SUMMARY's table: `go build ./...` (clean), `TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./...` (2035 passed, 21 packages), `make gate-copy-freeze` (all frozen strings present, exit 0), `make gate-visual-regression` (all tests PASS, exit 0), `go test ./internal/dummytui/ -run TestNoBackendAllowlist` (pass), `go test ./cmd/gitid/... -run TestParityMatrixResolvesAndCoversTree` (pass). All match 07-06-SUMMARY.md's exit-battery table. |
| 9 | The visual-regression gate genuinely registers every Global Git screen state, with real negative controls that actually fail when perturbed (not vacuous) | ⚠️ PARTIAL — see Gap below | `internal/screenshot/createflow.go`'s `globalGitSpecs()` (7 specs, real region dispositions, correctly scoped non-applicability for the differs-row/probe-error/receipt states) is genuine and the full gate passes. However 2 of the 4 claimed Global Git "negative controls" do not exercise the real gate-failure path (`screenshot.BuildRegionDiffs`/`screenshot.ValidateRegionDiffs`) — see Gaps Summary below. |

**Score:** 8/9 truths fully verified; 1 partial (narrow test-rigor gap, not a functional defect).

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | Global gitignore (`core.excludesfile` + managed pattern file) | Phase 8 | `07-CONTEXT.md` D-11 explicitly defers this to Phase 8's FIX-01/FIX-02; `REQUIREMENTS.md` §J was corrected in 07-05 (D-11.1) to name Phase 8; confirmed the write path (`EnsureGlobalGit`) actively refuses `core.excludesfile` today, matching the deferral. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/globalgit/policy.go` | 12-row pinned option policy table | ✓ VERIFIED | 284 lines, `Policy` var matches D-08 byte-for-byte; `PolicyFor`/`PolicyForToken`/`TokenOwningMember` all present and used by both TUI and CLI |
| `internal/globalgit/classify.go` | Four-state classifier | ✓ VERIFIED | 367 lines, scalar + bundle branch logic, source-class provenance |
| `internal/globalgit/version.go` | Version gate (informational vs. hard) | ✓ VERIFIED | 124 lines, `GateHard` substitution logic for `merge.conflictstyle` only |
| `internal/globalgit/authorresolve.go` | D-06 post-write invariant probe | ✓ VERIFIED | 126 lines, wired into the fallback-author ceremony's verify stage |
| `internal/tuikit/globalgit.go` | TUI screen: scrolling list, apply ceremony, D9 ceremony | ✓ VERIFIED | 1058 lines, all claimed mechanisms present and wired |
| `cmd/gitid/git.go` | `gitid git options/fallback` CLI command tree | ✓ VERIFIED | 799 lines, both write verbs route through the shared lifecycle functions |
| `cmd/gitid/lifecycle.go` (`runGlobalGitApply`, `runGitFallbackAuthorApply`) | Single write chokepoint | ✓ VERIFIED | Journal-backed, unconditional backup, additive merge, D-11 excludesfile refusal |
| `internal/gitconfig/globalgit.go` (`EnsureGlobalGit`) | Managed-block composer | ✓ VERIFIED | 292+ lines, adoption of the legacy `baseline` sentinel name, additive merge, `core.excludesfile` guard |
| `internal/screenshot/createflow.go` (`globalGitSpecs`) | Visual-regression spec registration | ✓ VERIFIED | 7 specs, correct non-applicability scoping for asymmetric states |
| `cmd/gitid/gate_visual_regression_test.go` (4 Global Git negative controls) | Real negative controls | ⚠️ PARTIAL | 2/4 rigorous (`MissingState`, `CrossSurfaceAllowlistLeakage`); 2/4 do not invoke the real validator (`UnclassifiedDifference`, `PerturbedComparableRegion`) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/tuikit/globalgit.go` (`CommitGlobalGit`/`CommitGitFallbackAuthor` dispatch) | `cmd/gitid/wiring.go` | `m.backend.CommitGlobalGit`/`CommitGitFallbackAuthor` | WIRED | Grep-confirmed single call path |
| `cmd/gitid/wiring.go` (`CommitGlobalGit`/`CommitGitFallbackAuthor`) | `cmd/gitid/lifecycle.go` (`runGlobalGitApply`/`runGitFallbackAuthorApply`) | direct call | WIRED | Confirmed by reading `wiring.go:1783-1855` |
| `cmd/gitid/git.go` (CLI apply/set verbs) | `cmd/gitid/lifecycle.go` (`runGlobalGitApply`/`runGitFallbackAuthorApply`) | `cliGlobalGitApplyInto`/`cliGitFallbackAuthorApplyInto` | WIRED | Same functions the TUI calls — confirmed no divergence |
| `cmd/gitid/lifecycle.go` (`runGlobalGitApply`) | `internal/gitconfig/globalgit.go` (`EnsureGlobalGit`) | direct call | WIRED | Confirmed at `lifecycle.go:1273` |
| `cmd/gitid/lifecycle.go` (`runGitFallbackAuthorApply`) | `internal/globalgit/authorresolve.go` (`VerifyAuthorResolution`) | `probe :=` seam, real function by default | WIRED | Confirmed at `lifecycle.go:1495`, `wiring.go:133` |
| Legacy `internal/gitconfig/baseline.go` (`WriteBaselineFile`/`WriteBaselineInclude`) | (nothing) | — | NOT WIRED (intentionally dormant) | Zero live callers outside tests — confirms no second write path undermines D-11's excludesfile demotion |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Module builds clean | `go build ./...` | exit 0, no output | ✓ PASS |
| Full race suite | `TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./...` | 2035 tests passed, 21 packages | ✓ PASS |
| `internal/globalgit` package alone | `go test ./internal/globalgit/...` | 64 tests passed | ✓ PASS |
| Global Git TUI tests | `go test ./internal/tuikit/... -run GlobalGit` | 55 tests passed | ✓ PASS |
| Copy-freeze gate | `make gate-copy-freeze` | all frozen strings present, exit 0 | ✓ PASS |
| No-backend gate | `go test ./internal/dummytui/ -run TestNoBackendAllowlist` | PASS (0.27s) | ✓ PASS |
| Visual-regression gate (full) | `make gate-visual-regression` | all PASS, exit 0, 43.2s | ✓ PASS |
| Parity matrix | `go test ./cmd/gitid/... -run TestParityMatrixResolvesAndCoversTree` | PASS | ✓ PASS |
| Mid-transaction PTY e2e (read source, not run — 900s full suite skipped for time) | `e2e/global_git_pty_e2e_test.go:347` | genuine file-level assertions (`os.Stat`, `readFileE2E`, content checks) on a real stat-based obstruction | ✓ PASS (source-verified non-vacuous) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| GGIT-01 | 07-01…07-06 | Global git options screen + recipe defaults, each explained; backup + managed-block chokepoint | ✓ SATISFIED | Truths 1-8 above; `REQUIREMENTS.md` §J marked Complete, confirmed accurate |
| DLV-04 (this phase's contribution) | 07-04, 07-06 | Automated classification of every real-vs-dummy difference | ✓ SATISFIED (project-wide DLV-04.2 cross-AI review remains a standing, project-wide deferred obligation since Phase 3 — not specific to Phase 7, not blocking per `.planning/ONESHOT.md`'s checklist ordering) | Task 1 registration verified; see Gap for the 2 narrow negative-control tests |
| DLV-06 | 07-04 | Real PTY e2e per screen | ✓ SATISFIED | 14 frames, 12 raw-keystroke test functions in `e2e/global_git_pty_e2e_test.go`; one case (mid-transaction failure/retry) read in full and confirmed non-vacuous |

No orphaned requirements found — REQUIREMENTS.md maps only GGIT-01 to Phase 7.

### Anti-Patterns Found

Scanned all core Phase 7 files (`internal/globalgit/*.go`, `internal/tuikit/globalgit.go`, `cmd/gitid/git.go`, `cmd/gitid/lifecycle.go`, `internal/gitconfig/globalgit.go`, `internal/gitconfig/fallbackauthor.go`) for TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER markers.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/gitconfig/globalgit.go` | 184 | "placeholder for future policy additions" | ℹ️ Info | Benign comment describing forward-extensibility of an `[extras]` section; not a stub or debt marker |

No debt markers (TBD/FIXME/XXX) found in any Phase 7 core file. No unreferenced-follow-up markers requiring the debt-marker gate.

### Human Verification Required

None. The one finding below (negative-control rigor) is a definitively-determined code fact, not something requiring subjective/visual human judgment — it is reported as a structured gap instead.

### Note: concurrent uncommitted change observed during verification

While verifying, an uncommitted working-tree change appeared in
`internal/tuikit/globalgit.go`/`globalgit_test.go` (a checkbox-column
neutral-marker rendering fix, citing an in-progress
`.planning/phases/07-global-git-options/07-UI-REVIEW.md`) — consistent with a
concurrently running UI-review process (checklist item 8). This is NOT part
of the phase's committed submission (`f429bba`) and was not treated as
evidence either way in this report; my findings above are based on the
committed tree. Flagging only so the orchestrator knows a second process is
touching this file concurrently.

### Gaps Summary

**One narrow gap, non-blocking to the shipped feature:** two of the four
Global Git visual-regression negative controls
(`TestNegativeControl_GlobalGitUnclassifiedDifference`,
`TestNegativeControl_GlobalGitPerturbedComparableRegion` in
`cmd/gitid/gate_visual_regression_test.go`) assert only that a precondition
exists (a real divergence, or a disposition-free spec) and then `t.Logf()` an
unverified claim that "the real gate" would reject it. They never call
`screenshot.BuildRegionDiffs`/`screenshot.ValidateRegionDiffs` — the actual
gate-failure path — the way their own Phase 4/5/6 sibling tests in the same
file do (e.g. `TestNegativeControl_GlobalSSHUnclassifiedDifferenceRejected`,
`TestNegativeControl_AllGlobalSSHComparableEqualRegionsAreMutationSensitive`).
Phase 7 also has no `TestNegativeControl_AllGlobalGitComparableEqualRegionsAreMutationSensitive`
counterpart at all — the established mutation-sensitivity pattern used by
every prior UI-wave phase.

This does not mean the current Global Git classifications are wrong — I
independently confirmed they are correct (`make gate-visual-regression`
passes cleanly, `TestGlobalGitAllowlistMatchesRegistry` passes, and the
underlying `BuildRegionDiffs`/`ValidateRegionDiffs` mechanism is proven
rigorous elsewhere in the same test file using a merged capture set that
includes Global Git data, just scoped to `globalSSHScreenIDs`'s own
assertions). What is unproven is whether a *future* regression that silently
misclassifies (or fails to classify) a Global Git difference would actually
be caught — which is exactly what "negative control" tests exist to
guarantee, and exactly what 07-06-SUMMARY.md claims ("All four PASS as tests
— the gate fails as designed when their perturbation runs").

Suggested fix (small, scoped): rewrite the two weak tests to call
`screenshot.BuildRegionDiffs` on the real+dummy Global Git captures (as the
two rigorous Global Git controls' Phase 6 siblings already do), mutate a
classification/region the same way, and assert `ValidateRegionDiffs` returns
a non-nil error — closing the gap with the established, already-proven
pattern rather than inventing a new one.

**This looks like it could be an accepted deviation** if the team judges the
existing coverage (2 rigorous Global Git controls + the cross-surface
Phase 4/5/6 controls that already exercise the shared validator machinery) as
sufficient risk coverage. To accept this deviation instead of closing it, add
to this file's frontmatter:

```yaml
overrides:
  - must_have: "Global Git visual-regression gate's negative controls genuinely fail when perturbed"
    reason: "<reasoning>"
    accepted_by: "<name>"
    accepted_at: "<ISO timestamp>"
```

---

_Verified: 2026-08-28T01:36:11Z_
_Verifier: Claude (gsd-verifier)_
