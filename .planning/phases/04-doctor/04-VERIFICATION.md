---
phase: 04-doctor
verified: 2026-06-12T00:00:00Z
status: passed
reverified: 2026-06-12 — all 3 gaps closed by gap-closure plans 04-06 + 04-07; confirmed in production code (not just tests)
score: 7/7 must-haves verified after gap closure (initial pass found 3 critical production-wiring gaps; now resolved)
mode: standard
source: [04-REVIEW.md]
gap_closure_plans: [04-06, 04-07]
gaps_resolution:
  - id: DOC-GAP-01
    status: resolved
    by: 04-06
    evidence: "orphans.go Fix.Fn → deps.RemoveBlock (lines 55,85); coherence.go Fix.Fn → deps.AddWiring (lines 127,260); no surviving `func() error { return nil }` stubs; real-wiring test (cmd/gitid/doctor_realwiring_test.go) drives buildDoctorDeps against temp $HOME and asserts on-disk block removal/addition + 0644 allowed_signers mode"
  - id: DOC-GAP-02
    status: resolved
    by: 04-07
    evidence: "buildDoctorDeps sets RunSSHAdd + RunSSHKeygenFingerprint (cmd/gitid/doctor.go:216-217), arg-slice exec; wiring assertion test in cmd/gitid/doctor_agent_test.go"
  - id: DOC-GAP-03
    status: resolved
    by: 04-07
    evidence: "fix gate guarded by `fix || isTerminalInput(os.Stdin)` (cmd/gitid/doctor.go:116); non-interactive stdin skips the prompt"
  - id: IN-03
    status: resolved
    by: 04-07
    evidence: "main() propagates tiered doctorExitCode via os.Exit (cmd/gitid/main.go:18,22)"
  - id: WR-03
    status: resolved
    by: 04-07
    evidence: "checkGitconfigPath warns only on group/world-write bits; default 0644 ~/.gitconfig no longer falsely flagged"
final_gate: "go build ./... OK; go test ./... all 13 packages pass (exit 0); golangci-lint 0 issues"
method: >
  Code review (gsd-code-reviewer, standard depth, 22 source files) surfaced 3 critical
  findings; each was independently confirmed against the live code by the orchestrator
  with concrete file:line evidence and grep-verified call-site analysis (not test runs,
  which pass because they inject the Deps seams directly and never exercise the production
  buildDoctorDeps/main wiring).
gaps:
  - id: DOC-GAP-01
    requirements: [DOC-04, DOC-06]
    severity: critical
    source_finding: CR-01
    title: "Auto-fix is a silent no-op for orphans, coherence, and baseline families"
    evidence: >
      applyFixes invokes f.Fix.Fn() (cmd/gitid/doctor.go:454, :469), but the Fix.Fn closures
      for orphan/coherence/baseline findings are hardcoded `func() error { return nil }`
      (internal/doctor/checks/orphans.go:69,92; coherence.go:118,192,212; baseline.go:63,120).
      The real chokepoint closures wired into Deps in buildDoctorDeps — RemoveBlock and AddWiring
      — have ZERO production call sites: `deps.AddWiring(` is never called anywhere, and
      `deps.RemoveBlock(` is referenced only in doctor_test.go:457,487. So `gitid doctor --fix --yes`
      prints "fixed: ..." and increments the applied tally while touching zero files for every
      fixable family except permissions.
    fix_direction: >
      The check functions (which already receive deps) must build their Fix.Fn to call
      deps.RemoveBlock / deps.AddWiring with the finding's path/name/line, OR the cmd layer must
      rewrite each finding's Fn from finding metadata before applyFixes runs. Add a test that drives
      the REAL buildDoctorDeps wiring (e.g. against a temp ~/.ssh + ~/.gitconfig) and asserts the
      managed block is actually removed/added — not an injected-seam fake.
  - id: DOC-GAP-02
    requirements: [DOC-05]
    severity: critical
    source_finding: CR-02
    title: "Agent family is dead in production — RunSSHAdd never wired"
    evidence: >
      buildDoctorDeps wires CheckAgent/CheckSigning (cmd/gitid/doctor.go:284-285) but never sets
      the Deps.RunSSHAdd field. CheckAgent guards `if deps.RunSSHAdd == nil { return nil }`
      (internal/doctor/checks/signing.go:84), so it always returns no findings and the Agent
      section unconditionally reports healthy — masking a down ssh-agent or unloaded keys. The only
      RunSSHAdd assignments in the tree are in signing_test.go.
    fix_direction: >
      Wire Deps.RunSSHAdd (and RunSSHKeygenFingerprint if CheckSigning needs it) in buildDoctorDeps
      to real `ssh-add -l` / `ssh-keygen -lf` runners (arg-slice, no shell). Add a wiring test that
      asserts the fields are non-nil after buildDoctorDeps.
  - id: DOC-GAP-03
    requirements: [DOC-06]
    severity: warning
    source_finding: CR-03
    title: "No TTY guard before the interactive fix gate"
    evidence: >
      runDoctor reads bufio.NewReader(os.Stdin) (cmd/gitid/doctor.go:103) and presents the
      "Apply N fix(es)?" gate (doctor.go:431) with no terminal/IsTerminal check, so a piped or CI
      invocation of bare `gitid doctor` emits an unanswerable prompt into machine-parsed output.
    fix_direction: >
      Detect a non-interactive stdin (term.IsTerminal / file mode) and, when not a TTY, skip the
      interactive gate (treat as decline / report-only) rather than blocking on a read.
---

# Phase 4: Doctor — Verification Report

**Phase Goal:** `gitid doctor` performs deep health checks across dependencies, permissions,
coherence/drift, orphans, signing wiring, and agent reachability, classifies each finding by
severity, and offers safe opt-in auto-fix for the repairable findings.

**Verified:** 2026-06-12
**Status:** gaps_found
**Mode:** standard

## Summary

All 5 plans executed with green unit tests, `go build ./...` clean, full `go test ./...` green,
and `golangci-lint run ./...` at 0 issues. However, the test suite proves the *check primitives*
and the *applyFixes consent/batching/exit-code flow* in isolation by injecting the `Deps` seams —
it never exercises the production `buildDoctorDeps`/`main` wiring. Code review plus orchestrator
re-verification found that wiring incomplete in three places that defeat the phase goal.

## What Passed

| Must-have | Requirement | Status | Evidence |
|-----------|-------------|--------|----------|
| Read-only Finding/Severity/Family model + Run dispatch | DOC-07 | ✓ | internal/doctor/doctor.go; doctor_test.go green |
| Permissions family (and its real fix) | DOC-02 | ✓ | CheckPermissions + FixPerm wired in buildDoctorDeps (the one fix path that works) |
| Dependencies family | DOC-01 | ✓ | CheckDeps + platform.InstallHint per-OS |
| Coherence + Orphans + Baseline *detection* | DOC-03/04 (detect) | ✓ | check logic sound and tested; only the *fix* path is broken (see gaps) |
| Signing detection + applyFixes consent/batching/pre-fix exit code | DOC-06 (flow) | ✓ | applyFixes D-04 flow + D-07 pre-fix ExitCode capture verified |

## Gaps (block completion)

See frontmatter `gaps:` for full evidence. In brief:

1. **DOC-GAP-01 (critical, CR-01)** — `--fix` silently no-ops for orphans/coherence/baseline; reports success while changing nothing. Defeats DOC-04/DOC-06.
2. **DOC-GAP-02 (critical, CR-02)** — Agent check never runs in production (`RunSSHAdd` unwired); always reports healthy. Defeats DOC-05.
3. **DOC-GAP-03 (warning, CR-03)** — interactive fix gate has no TTY guard; breaks piped/CI use of `doctor`.

## Other Findings

6 warnings + 3 info in 04-REVIEW.md, notably: WR-01 `findSignerLine` first-hit false mismatch,
WR-02 `RemoveBlock` closure hardcodes 0600 (wrong for the 0644 allowed_signers file), WR-03 perms
check flags a default 0644 ~/.gitconfig as a 0600 error, IN-03 `main()` collapses the tiered
0/1/2/3 exit code to flat `os.Exit(1)`. These should be triaged during gap-closure planning.

## Recommendation

Route through gap closure: `/gsd-plan-phase 04 --gaps` → creates `gap_closure: true` plans for
DOC-GAP-01..03 (with tests that drive the real wiring) → `/gsd-execute-phase 04 --gaps-only` →
re-verify. Do not mark the phase complete until DOC-GAP-01 and DOC-GAP-02 are resolved.

## Re-Verification (2026-06-12) — PASSED

Gap-closure plans **04-06** and **04-07** executed (4 commits: `9237a5f`, `43d723e`, `fa8f72f`,
`2425f9a`). All three gaps confirmed closed in the production `buildDoctorDeps`/`runDoctor`/`main`
wiring by direct code inspection (see `gaps_resolution` in frontmatter), not merely by green tests —
and the new RED tests are real-wiring integration tests that drive `buildDoctorDeps` against a temp
`$HOME` and assert on-disk effects, so the original injected-seam blind spot is now structurally
covered. Bonus: 04-06 also removed a latent `incompleteNames` guard bug that prevented orphan
Classes 1+2 from ever firing. Final gate green (build + 13/13 test packages + 0 lint issues).

**Status: passed.** Phase 4 goal achieved.
