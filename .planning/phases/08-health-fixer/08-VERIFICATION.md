---
phase: 08-health-fixer
verified: 2026-08-31T00:31:59Z
status: gaps_found
score: 9/10 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 9/10
  gaps_closed:
    - "DLV-06 real-PTY e2e coverage for the D-16 batch-walk halt-on-failure state — TestHealthFixer_RealPTYFixerBatchWalkHalt (e2e/health_fixer_pty_e2e_test.go, added in commit cbb5279) now genuinely drives the halt path through the real compiled binary and asserts the halt message, that fix 1 stands, and that fix 2's un-applied change never took effect. Independently re-run: PASS."
  gaps_remaining: []
  regressions: []
gaps:
  - truth: "The DLV-06 real-PTY e2e suite that closes the D-16 batch-walk halt-on-failure gap stays part of a green, cross-OS `make test-e2e` CI gate (Phase 1 BUILD-01/02/04: `make test-e2e` must pass on both macOS and Linux GitHub Actions runners on every PR and push to main, per `.github/workflows/ci.yml`'s `check` job matrix)."
    status: failed
    reason: >
      This is a NEW gap, not a recurrence of the original (now-closed) DLV-06
      coverage gap. TestHealthFixer_RealPTYFixerBatchWalkHalt's helper,
      makeImmutable (e2e/health_fixer_pty_e2e_test.go:306-314), unconditionally
      shells out to the `chflags` binary ("chflags uchg <path>") to force the
      mid-batch permission failure. `chflags` is a BSD/macOS-only utility — it
      does not exist on Linux (Ubuntu/glibc userland ships no such command; the
      nearest Linux analogue, `chattr +i`, is a different program with
      different semantics and typically requires CAP_LINUX_IMMUTABLE). The
      function has no `runtime.GOOS` guard, no `exec.LookPath` availability
      check, and no `t.Skip` fallback — contrast with this same codebase's own
      established patterns for exactly this problem: (1) a `runtime.GOOS ==
      "darwin"` conditional already exists for OS-specific assertions
      (e2e/create_flow_pty_e2e_test.go:1135), and (2) the "established
      precedent" the original gap report cited for forcing a real write
      failure — e2e/global_git_pty_e2e_test.go's
      TestGlobalGit_RealPTYMidTransactionFailureAndRetry — deliberately uses a
      portable stat-based obstruction (pre-creating the target path as a
      directory, rejected by mutationJournal.watchFile's non-regular-file
      check) specifically BECAUSE the OS-permission route the plan originally
      suggested ("chmod ~/.gitconfig.d to 0500") could not be made to fail
      reliably/portably. The new D-16 halt test does not follow either
      precedent.

      .github/workflows/ci.yml's `check` job runs `make test-e2e` on
      `ubuntu-latest` for EVERY pull_request and push to main (not gated
      behind a macOS-only condition — only the third runner,
      macos-15-intel, is push-only). `make test-e2e` runs
      `go test -tags e2e -race -timeout 900s ./e2e/...` with no per-file
      OS exclusion (the file is plain-named, not `_darwin.go`-suffixed), so
      TestHealthFixer_RealPTYFixerBatchWalkHalt compiles and runs
      unconditionally on the ubuntu-latest runner.

      Reproduced deterministically (not just reasoned about): running the
      test's underlying exec call with `chflags` absent from PATH (simulating
      a Linux runner) produces `exec: "chflags": executable file not found in
      $PATH` — a non-nil error from `exec.Command(...).Run()` — which
      `makeImmutable` turns into `t.Fatalf`, failing the test and therefore
      the whole `make test-e2e` step on Linux.

      I independently confirmed the test PASSES on this session's actual
      macOS/Darwin machine (both alone and as part of the full
      `TestHealthFixer_` and `TestHealthFixer_RealPTYFixerBatchWalkHalt` runs)
      — the underlying D-16 invariants (halt message, first fix stands,
      second fix's change never took effect) ARE genuinely proven there. The
      defect is narrowly the test's platform portability, not the invariant
      it proves or the production code under test.
    artifacts:
      - path: "e2e/health_fixer_pty_e2e_test.go"
        issue: "makeImmutable (lines 306-314) calls exec.Command(\"chflags\", \"uchg\", path) and exec.Command(\"chflags\", \"nouchg\", path) unconditionally, with no runtime.GOOS check, no exec.LookPath probe, and no test skip for non-Darwin/BSD platforms. TestHealthFixer_RealPTYFixerBatchWalkHalt (lines 324-354) is the sole caller and will fail on any CI runner where chflags is absent (ubuntu-latest, per .github/workflows/ci.yml)."
    missing:
      - "Gate makeImmutable so the test only runs where chflags exists (e.g. t.Skip on runtime.GOOS != \"darwin\" and != the BSD family, or an exec.LookPath(\"chflags\") probe that skips instead of fails), OR — preferably, to keep real Linux+macOS PTY coverage of this exact D-16 invariant rather than losing it on Linux — replace the OS-immutable-flag mechanism with the codebase's own established portable technique (a stat-based obstruction on the second fix's write target, mirroring e2e/global_git_pty_e2e_test.go's directory-in-place-of-file rejection) so the mid-batch failure is forced identically and deterministically on both macOS and Linux CI runners."
---

# Phase 8: Health + Fixer Verification Report (Re-verification)

**Phase Goal:** A developer opens a Health screen split into SSH and Git
sections, sees redundant/contradictory config and per-identity health, and
fixes problems in place.
**Verified:** 2026-08-31T00:31:59Z
**Status:** gaps_found (the originally reported gap IS closed; one NEW gap
found during independent re-verification — a Linux CI-portability defect in
the exact test that closed it)
**Re-verification:** Yes — after gap closure (commit cbb5279 for the code
fix + new test; commit 6881319 for the review/verification artifacts)

## Summary

I independently re-ran the full gate battery from a clean, current checkout
(`gsd/phase-09.5-full-ssh-git-properties-browser`, working tree clean,
`cbb5279` confirmed an ancestor of HEAD):

- `go build ./...` — exit 0.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — **2353 passed,
  22 packages**, zero failures (grew from the prior verification's 2156/21
  because Phase 9 has since completed and Phase 9.1-9.5 context work has
  landed on this branch; no regressions).
- `make lint` — 0 issues (includes the `lint-tagged` guard over all four
  `//go:build` tags: screenshot, smoke, e2e, realaccount).
- `make gate-visual-regression` — exit 0, full battery green (51
  RequiredScreenSpecs, all negative controls pass).
- `go test -tags e2e ./e2e/... -run TestHealthFixer_` — **8 passed** (up
  from 7 at the prior verification — the new
  `TestHealthFixer_RealPTYFixerBatchWalkHalt` is the addition).
- `go test -tags screenshot ./cmd/gitid/... -run HealthFixer` — 8 passed.
- `go test ./internal/tuikit/... -run
  'TestSingleFixFailureNoNonsensicalBatchMessage|TestBatchWalkHalt|TestHealthNegativeAssertion|TestFixerCompleteFixableSet'`
  — 8 passed (confirms both the CR-01 and WR-01 regression tests from
  cbb5279 pass, alongside the pre-existing D-16/read-only/fixable-set unit
  tests).

**The originally reported gap (DLV-06 real-PTY coverage of the D-16
batch-walk halt-on-failure state) is genuinely closed.**
`TestHealthFixer_RealPTYFixerBatchWalkHalt` (e2e/health_fixer_pty_e2e_test.go:324-354)
forces a real, OS-level `chmod(2)` failure (via `chflags uchg` on the
second fixable key) mid-batch through the real compiled binary, and asserts
every D-16 invariant: the halt message ("Fix 2 of...", "failed and was
rolled back", "Nothing else in this batch was attempted"), the real OS
error text ("operation not permitted"), that fix 1's permission change
stands (`0600`), and that fix 2's change never took effect (`0644`,
unchanged). I re-ran it directly — it passes, and it is not a shallow
existence check: it verifies real file-permission bytes on disk, not just
rendered text. The related CR-01 (Fixable signal wired from
`doctor.Finding.Fix != nil`, not `SuggestedFix != ""`) and WR-01 (the
batch-halt banner only renders when `m.batch != nil`) fixes from the same
commit are also confirmed present and wired (`internal/tuikit/doctor.go:105-111`,
`health_screen.go:187,207`, `fixer_screen.go:97-106,222,329`,
`cmd/gitid/wiring.go:4579`), each with a passing regression test.

**However, independent re-verification found a NEW gap the closure itself
introduced**, not disclosed in the commit message's own gate-battery
report: the new test's failure-forcing mechanism (`chflags`, a BSD/macOS-
only command) is not available on Linux, and `.github/workflows/ci.yml`
runs `make test-e2e` — which now includes this test unconditionally — on
`ubuntu-latest` for every PR and push to main. This will break the Linux
CI leg the very next time `make test-e2e` runs there. Full detail and
reproduction in the Gaps section below and in the frontmatter `gaps:`
entry.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The Health screen has SSH and Git sections, checks files exist/parse, and is genuinely READ-ONLY (HLTH-01, HLTH-02) | ✓ VERIFIED (regression-checked) | `internal/tuikit/health_screen.go` unchanged in substance since the prior verification; `TestHealthNegativeAssertion` re-run directly as part of the targeted tuikit run above — passes. Covered again by the full race suite (0 failures). |
| 2 | Detects repeated/overridden directives, duplicate managed/global blocks, and contradictions (HLTH-03, HLTH-04) | ✓ VERIFIED (regression-checked) | `internal/doctor/checks/coherence.go` unchanged; full race suite covers `TestMissingFragmentNoDuplicate` and the coherence check tests, 0 failures. |
| 3 | Health is computable for a single identity (feeds MGR-07) and globally, reusing all 9 doctor families (HLTH-05, HLTH-06) | ✓ VERIFIED (regression-checked) | `cmd/gitid/wiring.go`'s `buildDoctorDeps` still wires all 9 `CheckFn` families — confirmed by direct source read; `Fixable: f.Fix != nil` (line 4579) is the one substantive change here, and it is additive (a new field), not a removal. |
| 4 | The Fixer presents SSH/Git problems with severity+explanation+suggested fix, applies ONLY with confirmation and backup, fixed in place, including the D-16 batch-walk halt-on-failure invariant (FIX-01, FIX-02) | ✓ VERIFIED | `internal/tuikit/fixer_screen.go`'s `haltBatch` (now also correctly gated on `m.batch != nil` per WR-01) builds the exact D-16 message. `TestBatchWalkHalt` (unit-level, re-run directly, passes) AND, new since the prior verification, `TestHealthFixer_RealPTYFixerBatchWalkHalt` (real-PTY, re-run directly, passes) both independently prove the same invariants — the model-level proof is now corroborated end-to-end through the real binary (on platforms where the new test's mechanism works — see Truth 7/Gap). |
| 5 | `gitid health`/`gitid fix`/hidden `gitid doctor` alias are real CLI commands with documented flags | ✓ VERIFIED (regression-checked) | `cmd/gitid/health.go`, `fix.go`, `doctor_alias.go` unchanged since prior verification; still registered in `main.go`; full race suite covers their tests, 0 failures. |
| 6 | The visual-regression gate registers both new tabs with the two Known Divergences correctly treated as non-applicable | ✓ VERIFIED (regression-checked) | Re-ran directly: `make gate-visual-regression` exits 0 (51 RequiredScreenSpecs); `go test -tags screenshot ./cmd/gitid/... -run HealthFixer` — 8/8 pass, unchanged count from the prior verification (the D-16 halt state is proven via real-PTY text assertions, not a new screenshot checkpoint — consistent with how D-16 was always scoped). |
| 7 | Raw-keystroke PTY e2e drives the real compiled binary for every named Health/Fixer screen state, as part of a green cross-OS `make test-e2e` CI gate (DLV-06) | ✗ FAILED — see Gap | The PTY coverage itself is now complete: `e2e/health_fixer_pty_e2e_test.go` has 8 test functions (up from 7), and `TestHealthFixer_RealPTYFixerBatchWalkHalt` genuinely drives the D-16 halt path through the real binary — re-ran directly, PASS on this session's macOS machine. But the mechanism it uses (`chflags`, BSD/macOS-only, no `runtime.GOOS` guard) will fail `make test-e2e` on the `ubuntu-latest` CI runner that `.github/workflows/ci.yml` requires on every PR/push — reproduced deterministically by removing `chflags` from `PATH` and observing `exec.Command` return `exec: "chflags": executable file not found in $PATH`, which the test's own `t.Fatalf` turns into a failure. DLV-06's real intent — durable, CI-green real-PTY coverage — is not met by an addition that breaks the CI gate it is supposed to run inside. |
| 8 | `.planning/REQUIREMENTS.md` HLTH-01..06/FIX-01/02 closure corresponds to real, executed evidence, independently spot-checked | ✓ VERIFIED (re-confirmed) | Re-ran, did not just read the tables: `go build ./...` (exit 0), full race suite (2353 passed, 22 packages), `make lint` (0 issues), `make gate-visual-regression` (exit 0). HLTH-01..06/FIX-01 rows are `[x]`; FIX-02 is now `[~]` — see note below (informational, not a Phase 8 code gap). |
| 9 | The Fixer's flagship D-09 surgical single-directive rewrite genuinely edits ONE existing hand-written directive with typed-hostname confirm (D-11) | ✓ VERIFIED (regression-checked) | `internal/sshconfig/rewrite.go` unchanged; `TestHealthFixer_RealPTYFixerCeremonyWritesAndBacksUp` still passes as part of the 8/8 `TestHealthFixer_` run. |
| 10 | The false-positive-loop precedent is structurally closed (D-06.1 orphan downgrade, D-13 re-scan-after-fix, D-14 convergence alarm) | ✓ VERIFIED (regression-checked) | `internal/doctor/checks/orphans.go` and `cmd/gitid/wiring.go`'s convergence machinery unchanged; full race suite covers all `TestOrphan*` and convergence tests, 0 failures. |

**Score:** 9/10 truths verified — 1 failed (Truth 7: the DLV-06 gap-closure
test itself is not cross-OS portable and will break the mandatory Linux
`make test-e2e` CI gate). This is a different failure reason than the
prior verification's Truth 7 (which was "coverage missing entirely" — that
specific problem IS resolved).

### Note on FIX-02 (not a Phase 8 gap)

`.planning/REQUIREMENTS.md` line 297 now marks FIX-02 `[~]` ("Reopened in
Phase 9.4 per UXP-05"): a later user audit (documented under Phase 9.4,
whose plans are still `TBD` — not yet executed) found that the standalone
Fixer tab's "only auto-fixable findings" count can read as contradicting
Health's "every finding" count, and proposes merging Fixer back into a
single "Doctor" tab. This is a **forward-looking UX requirement change**
discovered after Phase 8 shipped, tracked against a specific future phase
(9.4) — it is not evidence that Phase 8's own goal or FIX-02's original
wording ("presents SSH and Git problems... and fixes them in place") is
unmet today. Per Step 9b, this is correctly a **deferred** item, not a
Phase 8 gap.

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | FIX-02's original "two-section fixer UX" split is reopened as a UX-consistency concern (Fixer-vs-Health count parity) | Phase 9.4 | ROADMAP.md "Phase 9.4: TUI UX Consistency & Doctor/Fixer Parity", success criterion 5: "Health and Fixer are merged back into a single 'Doctor' tab... reversing Phase 8's FIX-02 split..." (UXP-05). REQUIREMENTS.md line 297 cross-references the same. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `e2e/health_fixer_pty_e2e_test.go` | DLV-06 real-PTY coverage, all named states, CI-safe | ⚠️ PRESENT BUT DEFECTIVE | 8 test functions now cover all named states including the D-16 halt path — genuinely proven on macOS — but `makeImmutable`'s `chflags` dependency (lines 306-314) is not gated for non-Darwin platforms and will fail `make test-e2e` on the `ubuntu-latest` CI runner. See Gap. |
| `internal/tuikit/doctor.go` (`Fixable` field, CR-01) | Fixable signal decoupled from SuggestedFix | ✓ VERIFIED | `HealthFinding.Fixable`, set from `doctor.Finding.Fix != nil` (`cmd/gitid/wiring.go:4579`); consumed in `health_screen.go` and `fixer_screen.go`, not `SuggestedFix != ""` anywhere in the fixability-gating paths. |
| `internal/tuikit/fixer_screen.go` (`haltBatch`, WR-01) | No nonsensical batch-halt banner on a single non-batch fix failure | ✓ VERIFIED | `haltBatch` (lines ~97-106) returns early when `m.batch == nil`; `TestSingleFixFailureNoNonsensicalBatchMessage` re-run directly, passes. |
| All 13 artifacts from the prior verification (health_screen.go, fixer_screen.go, frame.go TabID, rewrite.go, coherence.go, files.go, orphans.go, health.go, fix.go, doctor_alias.go, createflow.go healthFixerSpecs, visual-divergence-allowlist.txt, 08-08-REVIEWS.md) | Unchanged in substance | ✓ VERIFIED (regression-checked) | No diffs against these files since the prior verification beyond the additive `Fixable` field and `haltBatch` guard already covered above; full race + lint + visual-regression gates all green. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `cmd/gitid/wiring.go` (`runDoctorAndConvert`) | `internal/tuikit/doctor.go` (`HealthFinding.Fixable`) | direct field assignment `Fixable: f.Fix != nil` | WIRED | Confirmed at `wiring.go:4579`. |
| `internal/tuikit/health_screen.go` / `fixer_screen.go` fixability gating | `HealthFinding.Fixable` | direct field read | WIRED | `health_screen.go:187,207`, `fixer_screen.go:222,329` all key off `.Fixable`, not `.SuggestedFix`. |
| `internal/tuikit/fixer_screen.go` (`checkFixBatchHalt`) | `haltBatch` | `m.batch != nil` guard | WIRED | Confirmed at `fixer_screen.go:97-106`; regression test passes. |
| `.github/workflows/ci.yml` (`check` job, `ubuntu-latest` matrix entry) | `make test-e2e` → `e2e/health_fixer_pty_e2e_test.go`'s `TestHealthFixer_RealPTYFixerBatchWalkHalt` | `go test -tags e2e -race -timeout 900s ./e2e/...` | ✗ NOT SAFELY WIRED | The CI job unconditionally includes this test on the Linux runner; the test's own `chflags` dependency is absent there. See Gap. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Module builds clean | `go build ./...` | exit 0 | ✓ PASS |
| Full race suite | `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` | 2353 passed, 22 packages | ✓ PASS |
| Lint (full + all 4 build-tags vetted) | `make lint` | 0 issues | ✓ PASS |
| Visual-regression gate (full) | `make gate-visual-regression` | exit 0 | ✓ PASS |
| Health/Fixer real-PTY e2e (all) | `go test -tags e2e ./e2e/... -run TestHealthFixer_` | 8 passed | ✓ PASS |
| D-16 halt real-PTY test alone | `go test -tags e2e ./e2e/... -run TestHealthFixer_RealPTYFixerBatchWalkHalt -v` | 1 passed | ✓ PASS (macOS only — see Gap) |
| Health/Fixer gate-specific screenshot tests | `go test -tags screenshot ./cmd/gitid/... -run HealthFixer` | 8 passed | ✓ PASS |
| CR-01/WR-01 + D-16/read-only/fixable-set unit regression tests | `go test ./internal/tuikit/... -run 'TestSingleFixFailureNoNonsensicalBatchMessage\|TestBatchWalkHalt\|TestHealthNegativeAssertion\|TestFixerCompleteFixableSet'` | 8 passed | ✓ PASS |
| Reproduction: `chflags` absent from PATH (Linux simulation) | `PATH=<go-bin-only> go run <exec.Command("chflags",...)>` | `exec: "chflags": executable file not found in $PATH` | ✗ CONFIRMS THE GAP |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| HLTH-01 | 08-01, 08-06 | Two sections (SSH/Git), read-only | ✓ SATISFIED | Truth 1 |
| HLTH-02 | 08-03 | Files exist + parse | ✓ SATISFIED | Truth 1 |
| HLTH-03 | 08-01, 08-04, 08-05 | Redundancy/override detection | ✓ SATISFIED | Truth 2 |
| HLTH-04 | 08-02, 08-04, 08-05 | Contradiction detection | ✓ SATISFIED | Truth 2 |
| HLTH-05 | 08-06 | Per-identity health (MGR-07) | ✓ SATISFIED | Truth 3 |
| HLTH-06 | 08-01, 08-03, 08-05 | All 9 doctor families | ✓ SATISFIED | Truth 3 |
| FIX-01 | 08-02, 08-04, 08-06 | Confirmed, backed-up fixes | ✓ SATISFIED | Truth 4 |
| FIX-02 | 08-01, 08-06, 08-07 | Two-section fixer UX | ✓ SATISFIED (as originally scoped); reopened for a UX redesign in Phase 9.4 (deferred, not a Phase 8 gap) | Truth 4, 5; Deferred Items |
| DLV-04 | 08-08 | Visual-regression registration | ✓ SATISFIED | Truth 6 |
| DLV-06 | 08-08 | Real-PTY e2e per screen, part of the green CI gate | ✗ NOT SAFELY SATISFIED | Truth 7 — see Gap |

No orphaned Phase-8 requirement IDs found.

### Anti-Patterns Found

Scanned all files touched by commit cbb5279 (`cmd/gitid/wiring.go`,
`e2e/health_fixer_pty_e2e_test.go`, `internal/dummytui/data.go`,
`internal/tuikit/backend_stub_test.go`, `internal/tuikit/design.go`,
`internal/tuikit/doctor.go`, `internal/tuikit/fixer_screen.go`,
`internal/tuikit/fixer_screen_test.go`) for TBD/FIXME/XXX/TODO/HACK/
PLACEHOLDER markers.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | none found | — | No debt markers in any file touched by the gap-closure commit |

**Portability defect (not a debt marker, a functional CI-breaking bug)** —
see the `gaps` frontmatter entry and Truth 7 above. This is reported as a
structured gap rather than an anti-pattern-table row because it is not a
TODO/stub — it is a deterministic, reproducible cross-platform failure in
otherwise complete, well-written code.

### Human Verification Required

None. The gap found is a definitively-determined, directly-observable code
fact (an unconditional `exec.Command("chflags", ...)` call with no OS
guard, reproduced deterministically by removing `chflags` from `PATH`),
not a subjective or visual judgment call — reported as a structured gap
instead.

### Gaps Summary

**The originally reported gap is closed.** Commit cbb5279 added
`TestHealthFixer_RealPTYFixerBatchWalkHalt`, which genuinely drives the
D-16 batch-walk halt-on-failure path through the real compiled binary and
proves every claimed invariant (halt message, first fix stands, second
fix's change never took effect) with real file-permission byte checks, not
text-only assertions. Independently re-run directly on this session's
machine — it passes.

**One new gap was found during independent re-verification, not disclosed
in the closing commit's own reported gate-battery** (the commit message's
`make test-e2e - PASS (676s)` line reflects a single, unspecified-OS run —
consistent with having been run on the same macOS development machine used
for this session, not on the Linux CI runner where the defect surfaces).
`makeImmutable`'s `chflags` dependency is BSD/macOS-only, has no
`runtime.GOOS` guard or `exec.LookPath` availability check, and
`.github/workflows/ci.yml` runs `make test-e2e` (which now includes this
test unconditionally, since the file carries only the shared `e2e` build
tag) on `ubuntu-latest` for every pull request and every push to `main`.
The next CI run that reaches this test on Linux will fail with `exec:
"chflags": executable file not found in $PATH` — confirmed by reproducing
that exact error locally with `chflags` removed from `PATH`.

This codebase already has two established, working patterns for exactly
this problem that the new test does not follow: a `runtime.GOOS ==
"darwin"` conditional (`e2e/create_flow_pty_e2e_test.go:1135`), and — more
directly on point — the portable stat-based obstruction technique
`e2e/global_git_pty_e2e_test.go`'s `TestGlobalGit_RealPTYMidTransactionFailureAndRetry`
deliberately adopted specifically because an OS-permission-based failure
mechanism could not be made to fail reliably/portably in that case either.
Closing this gap should follow the same portable pattern (or, at minimum,
gate the new test to skip cleanly on non-Darwin platforms) so the D-16
real-PTY proof survives on both of the project's two CI operating systems,
per Phase 1's BUILD-01/02/04 contract.

**This looks like it could be an accepted deviation** only if the team
judges losing real-PTY D-16 coverage on Linux (via a `runtime.GOOS` skip)
as acceptable, given the invariant is still proven at the unit level
(`TestBatchWalkHalt`) and at the real-PTY level on macOS. If so, to accept
this deviation instead of closing it fully, add to this file's frontmatter:

```yaml
overrides:
  - must_have: "The DLV-06 real-PTY e2e suite that closes the D-16 batch-walk halt-on-failure gap stays part of a green, cross-OS make test-e2e CI gate"
    reason: "<reasoning — e.g. accept a macOS-only real-PTY halt test (with an explicit runtime.GOOS skip added first, since the current unconditional call would still break Linux CI as-is) given unit-level + macOS-PTY coverage is judged sufficient>"
    accepted_by: "<name>"
    accepted_at: "<ISO timestamp>"
```

Note: even accepting this deviation requires the code change (an explicit
skip on non-Darwin), not just an override entry — as written today, the
test does not skip on Linux, it fails.

---

_Verified: 2026-08-31T00:31:59Z_
_Verifier: Claude (gsd-verifier)_
