---
phase: 08-health-fixer
verified: 2026-08-28T00:00:00Z
status: gaps_found
score: 9/10 must-haves verified
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "The DLV-06 real-PTY e2e suite covers every Health/Fixer screen state, including the D-16 batch-walk halt-on-failure state (a mid-batch failure rolls back, halts naming the failure, and never attempts the rest of the queue)."
    status: partial
    reason: >
      08-08-PLAN.md's own Task 1 <action> text explicitly calls for exercising
      "the batch walk's halt-on-failure path (Wave 6)" via the real compiled
      binary. e2e/health_fixer_pty_e2e_test.go's TestHealthFixer_RealPTYFixerBatchWalk
      only drives the HAPPY-path batch walk (two fixes, both succeed,
      confirmed via real file-permission byte checks) -- there is zero
      occurrence of "halt"/"rolled back"/"Halt" anywhere in the file. The
      halt-on-failure behavior itself IS genuinely implemented and IS proven
      by a real behavioral test (TestBatchWalkHalt, internal/tuikit/fixer_screen_test.go,
      re-run directly and passing) that exercises the actual state transition
      with a stub backend forcing a mid-batch Persist failure -- so the
      underlying D-16 truth is not FAILED, only its real-PTY (end-to-end
      binary) coverage. This is a narrower gap than a functional defect: the
      08-08-PLAN.md must_haves.truths frontmatter itself only lists "batch
      walk" as one generic state (satisfied by the happy-path PTY test), not
      "batch walk (halt)" as a distinct required state -- so this is a gap
      against the PLAN's own <action> prose and the ROADMAP's DLV-06
      "every screen" success-criterion language, not against the PLAN's
      literal must_haves contract.
    artifacts:
      - path: "e2e/health_fixer_pty_e2e_test.go"
        issue: "TestHealthFixer_RealPTYFixerBatchWalk (line 274) only exercises the batch walk's success path; no test drives a mid-batch Persist failure through the real binary and asserts the haltBatch message/rollback/queue-stop via PTY."
    missing:
      - "A real-PTY test that forces a mid-batch fix failure against the compiled binary (this codebase already has precedent for forcing a real failure via a permission/stat obstruction -- e2e/global_git_pty_e2e_test.go's mid-transaction failure/retry case, cited in 07-VERIFICATION.md) and asserts the exact D-16 halt message, that the earlier-succeeded fix's file change stands, and that the un-attempted fix's file is untouched."
---

# Phase 8: Health + Fixer Verification Report

**Phase Goal:** A developer opens a Health screen split into SSH and Git
sections, sees redundant/contradictory config and per-identity health, and
fixes problems in place.
**Verified:** 2026-08-28T00:00:00Z
**Status:** gaps_found (one narrow, non-blocking test-coverage gap; every
functional truth checked is genuinely implemented and independently
re-verified in the actual codebase, not just claimed in SUMMARY.md prose)
**Re-verification:** No — initial verification

## Summary

This is an exceptionally thoroughly executed phase, on par with Phase 7. I
independently re-ran (not trusted) `go build ./...`, `make lint`, the full
`TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` suite (2156 tests,
21 packages, all green — matching every SUMMARY's claimed count exactly),
`make gate-visual-regression` (exit 0), and the Health/Fixer-scoped real-PTY
e2e tests (`TestHealthFixer_*`, 7/7 pass) and gate-visual-regression tests
(`*HealthFixer*`, 8/8 pass) directly. Every artifact named in the Context/UI-
SPEC/plans — the 5-tab `TabID` split, the genuinely read-only Health screen,
the Fixer's D-09 surgical single-directive rewrite with D-10 verify/auto-
restore, D-13 re-scan-after-every-fix, D-14 convergence alarm, D-16 batch-
halt, the three real CLI commands (`health`/`fix`/hidden `doctor` alias) with
all documented flags, the `gitid.health/v1` envelope, the D-06 tolerance
fixes (orphan Class-1 downgrade, reserved archive-path registry), the new
Files/Baseline/Coherence checks, the MGR-07/HLTH-05 per-identity deep-link,
and the visual-regression gate registration with its classified divergence
allowlist — all exist, are wired, and behave as claimed when exercised
directly.

The two documented, pre-approved UI-SPEC divergences (#1 the tab split, #2
the compressed 2-state ceremony) were correctly treated as resolved
decisions, not gaps, per the task instructions — and the codebase confirms
both are implemented exactly as 08-UI-SPEC.md resolves them (5 real tabs;
`fixCeremonyFor` genuinely builds one `ceremonyConfig`, not a 4-screen
chain).

**One narrow gap found**, disclosed in the frontmatter above: the D-16
batch-walk halt-on-failure state — a behavior the 08-08-PLAN.md Task 1
`<action>` text explicitly names as something the real-PTY suite should
exercise — is only proven at the unit/model level (a real, rigorous
behavioral test, `TestBatchWalkHalt`), not via the real compiled binary. The
underlying D-16 behavior itself is genuinely correct and well-tested; what's
missing is the real-PTY (end-to-end) proof for specifically the failure path,
which prior UI-wave phases (e.g. Phase 7's mid-transaction Global Git
failure/retry PTY case) have already established a working pattern for in
this codebase.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The Health screen has SSH and Git sections, checks files exist/parse, and is genuinely READ-ONLY (no write/confirm/backup affordance ever renders there) (HLTH-01, HLTH-02) | ✓ VERIFIED | `internal/tuikit/health_screen.go`'s own doc comment ("no f/F ceremony-trigger keys... Health only diagnoses"); grep for write-ceremony markers (`Persist`, `Apply fix`, `Confirm write`, `Backed up ->`, `Wrote ->`) returns zero matches in the file. `TestHealthNegativeAssertion` (`internal/tuikit/health_screen_test.go`) asserts this across all 4 render states (with-findings, all-green, per-identity, parse-error) — re-run directly, passes. `internal/doctor/checks/files.go`'s `CheckFiles` (added `doctor.FamilyFiles`) validates SSH config + Git config/fragments exist and parse, rendering the dedicated critical parse-error frame. |
| 2 | Detects repeated/overridden directives, duplicate managed/global blocks, and contradictions (`IdentitiesOnly no` + `IdentityFile`; `includeIf` → missing fragment) (HLTH-03, HLTH-04) | ✓ VERIFIED | `internal/doctor/checks/coherence.go`'s `checkHandWrittenIdentitiesOnly` (D-09 flagship, hand-written stanza), `CheckRedundancy` (duplicate `Host *`), the existing `Incomplete`/fragment-missing branch (dedup-guarded against `CheckOrphans` in Wave 5, `TestMissingFragmentNoDuplicate` re-run, passes). New Wave 5 checks (`checkShadowedGlobalOptions`, `checkDirectiveAboveManagedBlock`, `checkAuthorResolution`) all reuse existing Phase 6/7 probes (`internal/globalssh.Verify`, `internal/globalgit.VerifyAuthorResolution`) unchanged. |
| 3 | Health is computable for a single identity (feeds MGR-07) and globally, reusing all 9 doctor families (HLTH-05, HLTH-06) | ✓ VERIFIED | `internal/tuikit/store.go`'s `FindingsFor(state, identityName)`, called from `identities.go` (MGR-07 row badges) and both the TUI `h`-key deep-link (`identities.go:2200-2204` → `app.go:433-436,534-537`) and `gitid health --identity` (`cmd/gitid/health.go`, with shell completion). `cmd/gitid/wiring.go`'s `buildDoctorDeps` wires all 9 `CheckFn` families (`Deps`, `Perms`, `Coherence`, `Orphans`, `Signing`, `Agent`, `Baseline`, `Overlap`, `Redundancy`) — confirmed by direct source read, not SUMMARY claim. |
| 4 | The Fixer presents SSH/Git problems with severity+explanation+suggested fix, applies ONLY with confirmation and backup, fixed in place, including the D-16 batch-walk halt-on-failure invariant (a mid-batch failure rolls back from its OWN backup, halts naming the failure, and never attempts the rest of the queue) (FIX-01, FIX-02) | ✓ VERIFIED | `internal/sshconfig/rewrite.go`'s `ApplyVerifiedHostDirective` (D-10: parse→render→re-parse stability + real `ssh -G` re-verify, auto-restore from backup on mismatch). `internal/tuikit/fixer_screen.go`'s `haltBatch` builds the exact frozen D-16 message and clears `m.batch` so the queue cannot resume; `app.go`'s `checkFixBatchHalt` wires `Backend.PersistError()` into it. `TestBatchWalkHalt` (`internal/tuikit/fixer_screen_test.go`) is a real, rigorous behavioral test — 3-finding batch, forced failure on fix 2 via a stub backend, asserts fix 1's Reduce-applied state stands, fix 2/3 remain untouched, the exact halt-message substrings render, and `batch == nil` (fix 3 never attempted) — re-run directly, passes. See Gap below: this exact scenario is NOT additionally proven via the real-PTY suite, only at this behavioral-model level. |
| 5 | `gitid health`/`gitid fix`/hidden `gitid doctor` alias are real CLI commands with documented `--json`/`--identity`/`--yes`/`--dry-run` flags | ✓ VERIFIED | `cmd/gitid/health.go` (`--json`, `--identity` with shell completion), `cmd/gitid/fix.go` (`--yes`, `--dry-run`), `cmd/gitid/doctor_alias.go` (hidden, forwards `--fix`/`--yes`/`--dry-run`/`--json`/`--identity` to the same `runHealth`/`runFixCommand` helpers). All three registered in `cmd/gitid/main.go:105-107`. `gitid.health/v1` versioned envelope confirmed in `health.go` (`healthSchema`, `healthDocument`). Tiered exit codes confirmed via `doctor.ExitCode`/`severityToCode`. |
| 6 | The visual-regression gate registers both new tabs with the two Known Divergences correctly treated as non-applicable (not allowlist rows) and real negative controls | ✓ VERIFIED | `internal/screenshot/createflow.go`'s `healthFixerSpecs()` (3 checkpoints: `health-findings`, `fixer-list`, `fixer-ceremony-preview`) and `CaptureHealthFixerScreens`, wired into 12+ call sites in `cmd/gitid/gate_visual_regression_test.go` (`mergeHealthFixerCaptures`). `.planning/design/health-fixer/visual-divergence-allowlist.txt` (14 entries, all DLV-4, all `improvement`) explicitly documents why Known Divergence #1/#2 produce zero allowlist rows (both sides of the real-vs-dummy gate already implement the corrected shape identically). Re-ran directly: `make gate-visual-regression` exits 0; `go test -tags screenshot ./cmd/gitid/... -run HealthFixer` — 8/8 pass. |
| 7 | Raw-keystroke PTY e2e drives the real compiled binary for every named Health/Fixer screen state (DLV-06) | ⚠️ PARTIAL — see Gap | `e2e/health_fixer_pty_e2e_test.go` (305 lines, 7 test functions) covers health-with-findings+inline-detail, health-all-green, per-identity-health, parse-error, fixer-list, ceremony state A/B (the flagship rewrite, byte-verified against the real file AND its real backup — `TestHealthFixer_RealPTYFixerCeremonyWritesAndBacksUp`), the batch-walk HAPPY path, and nothing-to-fix. Re-ran directly: `go test -tags e2e ./e2e/... -run TestHealthFixer_` — 7/7 pass. The D-16 halt-on-failure state is the one named scenario in 08-08-PLAN.md's own Task 1 `<action>` text NOT covered here (zero occurrences of "halt"/"rolled back" in the file) — narrow gap, not a functional defect (Truth 4 above). |
| 8 | `.planning/REQUIREMENTS.md` HLTH-01..06/FIX-01/02 closure corresponds to real, executed evidence, independently spot-checked (not trusted from SUMMARY.md) | ✓ VERIFIED | Re-ran, did not just read the tables: `go build ./...` (exit 0), `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` (2156 passed, 21 packages — exact match to 08-08-SUMMARY.md's claimed count), `make lint` (0 issues), `make gate-visual-regression` (exit 0, full output confirms `OK`). All 8 rows are `[x]`/`Complete`, no orphaned Phase-8 requirement IDs found in REQUIREMENTS.md. |
| 9 | The Fixer's flagship D-09 surgical single-directive rewrite genuinely edits ONE existing hand-written directive with typed-hostname confirm (D-11), never touching anything else | ✓ VERIFIED | `internal/sshconfig/rewrite.go`'s `RewriteHostDirective` (single-line splice, CR-18 control-byte validation, chokepoint-write) + `DiffHostDirective` (true diff). `internal/doctor/checks/coherence.go`'s `checkHandWrittenIdentitiesOnly` sets `Destructive: &FixDestructive{ConfirmWord: "clientb.github.com", ...}` (D-11). `e2e/health_fixer_pty_e2e_test.go`'s `TestHealthFixer_RealPTYFixerCeremonyWritesAndBacksUp` byte-diffs the real file before/after through the real binary and confirms exactly one directive changed, plus asserts exactly one `.bak.*` backup file exists whose bytes equal the pre-fix file — re-run directly, passes. `CLAUDE.md`'s D-09 scoped exception is present and documents this narrow carve-out. |
| 10 | The false-positive-loop precedent is structurally closed (D-06.1 orphan downgrade, D-13 re-scan-after-fix, D-14 convergence alarm) | ✓ VERIFIED | `internal/doctor/checks/orphans.go`'s Class-1 downgrade to `SeverityInfo`/`Fix: nil` for SSH-only blocks (confirmed via 08-03-SUMMARY + direct grep of the reserved-path/orphan tests, `TestOrphan*` 16/16 pass on direct re-run). `cmd/gitid/wiring.go`'s `convergenceAlarmed`/`convergenceAlarmedMu` (D-14) genuinely re-scans via a FRESH `buildDoctorDeps` call after every fix (the stale-`Deps` bug found and fixed in Wave 2, confirmed by reading the surrounding code) and withdraws a non-converging fix from re-offer. |

**Score:** 9/10 truths fully verified; 1 partial (Truth 7, and correspondingly qualifying Truth 4's PTY-specific coverage) — a narrow DLV-06 test-coverage gap, not a functional defect in the underlying D-16 behavior itself.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/tuikit/health_screen.go` | Read-only Health tab | ✓ VERIFIED | 219 lines; no ceremony/write fields, `TestHealthNegativeAssertion` passes |
| `internal/tuikit/fixer_screen.go` | Fix-ceremony Fixer tab with D-16 batch halt | ✓ VERIFIED | 382 lines; `haltBatch`, `TestBatchWalkHalt` passes |
| `internal/tuikit/frame.go` (`TabID`) | 5-value tab split | ✓ VERIFIED | `TabIdentities..TabFixer`, `tabLabels` = 5 entries |
| `internal/sshconfig/rewrite.go` | D-09 surgical rewrite + D-10 verify/restore | ✓ VERIFIED | `RewriteHostDirective`, `ApplyVerifiedHostDirective`, `DiffHostDirective` all present |
| `internal/doctor/checks/coherence.go` | D-09 flagship + Wave 5 report-only checks | ✓ VERIFIED | `checkHandWrittenIdentitiesOnly`, `checkShadowedGlobalOptions`, `checkDirectiveAboveManagedBlock`, `checkAuthorResolution` |
| `internal/doctor/checks/files.go` | HLTH-02 parse gates (Files family) | ✓ VERIFIED | `CheckFiles`, tolerant of missing files (the fresh-home bug fixed in 08-03) |
| `internal/doctor/checks/orphans.go` | D-06.1 Class-1 downgrade + archive-path exclusion | ✓ VERIFIED | Confirmed via `TestOrphan*` re-run (16/16 pass) |
| `internal/gitconfig` (`WriteGlobalGitignore` wiring) | D-05 gitignore pair fix | ✓ VERIFIED | `Deps.FixExcludesfile`, patches the EXISTING managed block in place (Wave 4's read/write-location bugs found and fixed) |
| `cmd/gitid/health.go` | `gitid health [--json] [--identity]` | ✓ VERIFIED | 175 lines, `gitid.health/v1` envelope |
| `cmd/gitid/fix.go` | `gitid fix [--yes] [--dry-run]` | ✓ VERIFIED | 162 lines, `fixMaxPasses` backstop |
| `cmd/gitid/doctor_alias.go` | Hidden `doctor` + `doctor --fix` shim | ✓ VERIFIED | 35 lines, forwards to `runHealth`/`runFixCommand` |
| `internal/screenshot/createflow.go` (`healthFixerSpecs`) | DLV-04 registration | ✓ VERIFIED | 3 checkpoints, wired into 12+ gate call sites |
| `.planning/design/health-fixer/visual-divergence-allowlist.txt` | Classified divergence allowlist | ✓ VERIFIED | 14 entries, all DLV-4/improvement |
| `e2e/health_fixer_pty_e2e_test.go` | DLV-06 real-PTY coverage | ⚠️ PARTIAL | 7/9 named states fully covered (happy paths); D-16 halt-on-failure state absent (see Gap) |
| `.planning/phases/08-health-fixer/08-08-REVIEWS.md` | Cross-AI review + exit battery + closure draft | ✓ VERIFIED | 15 UX findings dispositioned (14 deferred with cited evidence, 1 fixed with regression test), exit battery reproduced independently |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/tuikit/identities.go` (`"h"` key) | `internal/tuikit/health_screen.go` (scoped) | `app.go`'s `healthIdentity` field | WIRED | Confirmed at `identities.go:2200-2204`, `app.go:433-436,534-537` |
| `internal/tuikit/fixer_screen.go` (`f`/`F`) | `internal/tuikit/backend.go` (`Backend.Persist`) | `app.go`'s `apply()` (synchronous) | WIRED | `checkFixBatchHalt` reads `PersistError()` right after `apply()` in the same Update cycle |
| `cmd/gitid/wiring.go` (`realBackend.Persist`, `FixFinding` case) | `internal/doctor` (`Finding.Fix.Fn`) | `persistFixFinding` | WIRED | Re-runs a FRESH `buildDoctorDeps` scan (D-13), tracked via `convergenceAlarmed` (D-14) |
| `cmd/gitid/health.go`/`fix.go`/`doctor_alias.go` | `cmd/gitid/wiring.go` (`buildDoctorDeps`, `doctorFindings`) | direct call | WIRED | Same construction path the TUI consumes — no second read path |
| `internal/doctor/checks/coherence.go` (`checkHandWrittenIdentitiesOnly`) | `internal/sshconfig/rewrite.go` (`ApplyVerifiedHostDirective`) | `Finding.Fix.Interactive`/`Rewrite` | WIRED | D-09/D-10/D-11 flagship chain confirmed end-to-end via real-PTY byte diff |
| `internal/screenshot/createflow.go` (`healthFixerSpecs`) | `cmd/gitid/gate_visual_regression_test.go` | `mergeHealthFixerCaptures` (12+ call sites) | WIRED | `make gate-visual-regression` passes; `HealthFixer`-scoped tests 8/8 pass |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Module builds clean | `go build ./...` | exit 0 | ✓ PASS |
| Full race suite | `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` | 2156 passed, 21 packages | ✓ PASS |
| Lint (full + screenshot-tagged) | `make lint` | 0 issues | ✓ PASS |
| Health/Fixer unit behavioral tests | `go test ./internal/tuikit/... -run 'TestBatchWalkHalt\|TestHealthNegativeAssertion\|TestFixerCompleteFixableSet'` | 7 passed | ✓ PASS |
| Health/Fixer real-PTY e2e | `go test -tags e2e ./e2e/... -run TestHealthFixer_` | 7 passed | ✓ PASS |
| Visual-regression gate (full) | `make gate-visual-regression` | exit 0 | ✓ PASS |
| Health/Fixer gate-specific tests | `go test -tags screenshot ./cmd/gitid/... -run HealthFixer` | 8 passed | ✓ PASS |
| Convergence/rewrite tests | `go test ./cmd/gitid/... -run 'TestPersistFixFindingAppliesRealRewrite\|Converge'` | 4 passed | ✓ PASS |
| Orphan/archive tolerance tests | `go test ./internal/doctor/checks/... -run TestOrphan` | 16 passed | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| HLTH-01 | 08-01, 08-06 | Two sections (SSH/Git), read-only | ✓ SATISFIED | Truth 1 |
| HLTH-02 | 08-03 | Files exist + parse | ✓ SATISFIED | Truth 1 |
| HLTH-03 | 08-01, 08-04, 08-05 | Redundancy/override detection | ✓ SATISFIED | Truth 2 |
| HLTH-04 | 08-02, 08-04, 08-05 | Contradiction detection | ✓ SATISFIED | Truth 2 |
| HLTH-05 | 08-06 | Per-identity health (MGR-07) | ✓ SATISFIED | Truth 3 |
| HLTH-06 | 08-01, 08-03, 08-05 | All 9 doctor families | ✓ SATISFIED | Truth 3 |
| FIX-01 | 08-02, 08-04, 08-06 | Confirmed, backed-up fixes | ✓ SATISFIED (D-16 real-PTY coverage narrow gap — see Gap) | Truth 4 |
| FIX-02 | 08-01, 08-06, 08-07 | Two-section fixer UX | ✓ SATISFIED | Truth 4, 5 |
| DLV-04 | 08-08 | Visual-regression registration | ✓ SATISFIED | Truth 6 |
| DLV-06 | 08-08 | Real-PTY e2e per screen | ⚠️ PARTIAL | Truth 7 — see Gap |

No orphaned Phase-8 requirement IDs found — `.planning/REQUIREMENTS.md` maps exactly HLTH-01..06/FIX-01/02 to Phase 8, all present in plan `requirements:` fields.

**Minor documentation staleness (not a code gap):** `.planning/REQUIREMENTS.md`'s FIX-01/FIX-02 prose (lines 295, 298) still reads "re-home into the health screen" / "the health screen's two sections" — pre-Known-Divergence-#1 wording (written before the tab-split resolution). The actual implementation correctly ships Fixer as its OWN tab (view 5), matching 08-UI-SPEC.md's binding resolution. This is stale prose only; the checkbox/status-table closure itself is accurate.

### Anti-Patterns Found

Scanned core Phase 8 files (`internal/tuikit/health_screen.go`, `fixer_screen.go`, `doctor.go`, `internal/doctor/**`, `cmd/gitid/health.go`, `fix.go`, `doctor_alias.go`, `internal/sshconfig/rewrite.go`) for TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER markers.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | none found | — | No debt markers in any Phase 8 core file |

No unreferenced follow-up markers requiring the debt-marker gate.

### Human Verification Required

None. The one finding (DLV-06 PTY coverage of the D-16 halt state) is a definitively-determined, directly-observable code fact (absence of "halt"/"rolled back" strings and any Persist-failure-forcing setup in the e2e file), not a subjective/visual judgment call — reported as a structured gap instead.

### Gaps Summary

**One narrow, non-blocking gap:** the real-PTY e2e suite (`e2e/health_fixer_pty_e2e_test.go`) does not exercise the D-16 batch-walk halt-on-failure state — the one scenario 08-08-PLAN.md's own Task 1 `<action>` text explicitly named ("the batch walk's halt-on-failure path (Wave 6)"). `TestHealthFixer_RealPTYFixerBatchWalk` only drives the happy path (both fixes in a two-item batch succeed, verified via real file-permission byte checks). No PTY test forces a mid-batch write failure and asserts the halt message, the rollback, and that the un-attempted fix's file is untouched — through the actual compiled binary.

This does not mean the D-16 behavior is broken or unproven: `TestBatchWalkHalt` (`internal/tuikit/fixer_screen_test.go`) is a real, rigorous behavioral test that forces exactly this scenario with a stub backend and asserts every claimed invariant (fix 1 stands, fix 2/3 remain untouched, the exact halt message renders, the batch queue is nil so fix 3 is never attempted) — re-run directly, it passes. The underlying `ApplyVerifiedHostDirective` auto-restore machinery (D-10) that a real mid-batch failure would invoke is itself proven in Wave 2's own regression tests. What's specifically missing is proof of the SAME invariant through the real compiled binary end-to-end, which is what DLV-06 ("PTY e2e drives every screen in the real binary") and the plan's own action text called for.

This project already has a working precedent for forcing a genuine write failure through a real-PTY test against the compiled binary (a stat-based obstruction), cited in `07-VERIFICATION.md`'s evidence for Global Git's mid-transaction failure/retry case (`e2e/global_git_pty_e2e_test.go:347`) — closing this gap would follow that established pattern (e.g., making the second finding's target file/directory temporarily unwritable between the two batch confirms), not invent a new one.

**This looks like it could be an accepted deviation** if the team judges the existing coverage (a rigorous model-level behavioral test plus proven D-10 auto-restore infrastructure) as sufficient risk coverage for this specific failure-path state. To accept this deviation instead of closing it, add to this file's frontmatter:

```yaml
overrides:
  - must_have: "The DLV-06 real-PTY e2e suite covers every Health/Fixer screen state, including the D-16 batch-walk halt-on-failure state"
    reason: "<reasoning>"
    accepted_by: "<name>"
    accepted_at: "<ISO timestamp>"
```

---

_Verified: 2026-08-28T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
