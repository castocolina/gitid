---
phase: "04-doctor"
plan: "04"
subsystem: doctor
tags: [doctor, ssh-agent, tdd, signing, fingerprint]
dependency_graph:
  requires: [04-01]
  provides: [CheckAgent, CheckSigning]
  affects: [phase-05-tui]
tech_stack:
  added: []
  patterns: [injected-deps, TDD-RED-GREEN, classifyAgentState, extractFingerprint, isKeyLoaded]
key_files:
  created:
    - internal/doctor/checks/signing_test.go
  modified:
    - internal/doctor/checks/signing.go
decisions:
  - "signing.go overwritten in place (same path, same exported signatures); no redeclaration"
  - "CheckSigning owns git<2.36+hasconfig: gate (D-20); gpg.format=ssh and allowed_signers email carve-outs remain in Plan 03 CheckCoherence (no duplication)"
  - "All agent/signing findings are report-only (Fix=nil); D-03 enforced"
  - "classifyAgentState uses exit code AND text for portability (Pitfall 1)"
  - "PubPath Stat guard added before fingerprint probe (Pitfall 7)"
metrics:
  duration: "~30min"
  completed: "2026-06-11"
  tasks: 1
  files: 2
---

# Phase 04 Plan 04: Signing/Agent Family Summary

Real ssh-agent probe (`CheckAgent`) and git-version gate (`CheckSigning`) replacing the
Plan 01 stubs in `internal/doctor/checks/signing.go` — no subprocess in unit tests,
fully fake-testable via injected `RunSSHAdd` / `RunSSHKeygenFingerprint` / `GitVersionAtLeast`.

## Commits

| Commit | Message |
|--------|---------|
| `9eeb04d` | test(04-04): add failing tests for agent probe, fingerprint matching, and git-version gate |
| `4d2d782` | feat(04-04): implement CheckAgent + CheckSigning — ssh-agent probe, fingerprint match, git-version gate |

## TDD Gate Compliance

- RED gate (test commit): `9eeb04d` — precedes GREEN; tests fail with "undefined: agentState" at compile, then with runtime assertion failures after stub types added.
- GREEN gate (feat commit): `4d2d782` — all 13 test functions pass; `go build ./...` and `go test ./internal/doctor/...` green.
- No REFACTOR commit required (code was clean on first pass).

## Artifacts Produced

### internal/doctor/checks/signing.go (OVERWRITTEN)

Real implementation replacing both Plan 01 stubs:

**Pure helpers (package-private):**
- `agentState int` iota type: `agentUnreachable(0)`, `agentRunningEmpty(1)`, `agentRunningWithKeys(2)`
- `classifyAgentState(output string, exitCode int) agentState` — exit-code + text dual classification (Pitfall 1)
- `extractFingerprint(keygenLine string) string` — SHA256: token parser, returns "" on miss
- `isKeyLoaded(agentOutput, pubKeyPath string, runFp func(string)(string,error)) bool` — cross-references fingerprint

**Exported check functions:**
- `CheckAgent(deps doctor.Deps) []doctor.Finding`
  - Calls `deps.RunSSHAdd()` → classify state
  - Unreachable → one `FamilyAgent` warning "ssh-agent: not reachable"; Fix=nil (D-03)
  - Running → for each identity with a present PubPath, probes fingerprint via `deps.RunSSHKeygenFingerprint`; absent key → per-identity warning; Fix=nil (D-03)
- `CheckSigning(deps doctor.Deps) []doctor.Finding`
  - Iterates `deps.Identities` for any `MatchHasconfig` match
  - If found AND `!deps.GitVersionAtLeast(2, 36)` → one `FamilySigning` warning; Fix=nil

### internal/doctor/checks/signing_test.go (NEW)

13 test functions covering all behaviors with injected fakes (no live agent):

| Test | Behavior Covered |
|------|-----------------|
| `TestClassifyAgentState` | 6 table cases: exit 0/1/2/3 with text variants |
| `TestExtractFingerprint` | 4 table cases: SHA256 present, MD5 only, empty, short hash |
| `TestAgentUnreachable` | exit 2 → 1 warning, Fix=nil, exact title copy |
| `TestAgentKeyNotLoaded` | fingerprint absent → per-identity warning, Fix=nil |
| `TestAgentKeyLoaded` | fingerprint present → 0 findings |
| `TestAgentMissingPubSkipped` | PubPath Stat fails → 0 findings (Pitfall 7) |
| `TestGitVersionGate_HasconfigOldGit` | hasconfig: + git<2.36 → Signing warning |
| `TestGitVersionGate_HasconfigNewGit` | hasconfig: + git>=2.36 → no warning |
| `TestGitVersionGate_OnlyGitdirNoWarning` | gitdir: only + old git → no warning |
| `TestIsKeyLoaded_Present` | fingerprint in agent output → true |
| `TestIsKeyLoaded_Absent` | fingerprint not in agent output → false |
| `TestIsKeyLoaded_FingerprintError` | runFp returns error → false |

## Design Notes

### gpg.format/allowed_signers ownership

Per plan design note and D-17 carve-outs: the `gpg.format=ssh` locked-value check and
the `allowed_signers` email-match check are in **Plan 03 CheckCoherence**. This plan
owns only the ssh-agent + git-version half of DOC-05. There is exactly one owner per
check; no duplication across plans.

### Real RunSSHAdd / RunSSHKeygenFingerprint wiring

The cmd layer (`cmd/gitid/doctor.go` `buildDoctorDeps`) must inject the real functions
with these signatures:

```go
// RunSSHAdd — real implementation:
// cmd := exec.Command("ssh-add", "-l") //nolint:gosec // fixed args, no user input (G204)
// out, err := cmd.Output()
// if ee, ok := err.(*exec.ExitError); ok { return string(ee.Stderr), ee.ExitCode() }
// return string(out), 0
deps.RunSSHAdd = func() (string, int) { ... }

// RunSSHKeygenFingerprint — real implementation:
// cmd := exec.Command("ssh-keygen", "-lf", pubPath) //nolint:gosec // gitid-managed path (G204)
// out, err := cmd.Output()
// return string(out), err
deps.RunSSHKeygenFingerprint = func(pubPath string) (string, error) { ... }
```

Both are already declared in `doctor.Deps` (Plan 01 locked contract) and wired in
`cmd/gitid/doctor.go`. No cmd-layer changes needed for this plan.

---

## Deviations from Plan

None — plan executed exactly as written. The stub update approach (add minimal type
definitions to `signing.go` before the RED commit to satisfy the strict lint pre-commit
hook) followed the project's established TDD RED stub pattern.

---

## Known Stubs

None — `CheckSigning` and `CheckAgent` are now fully implemented. The two Plan 01 stubs
have been replaced.

---

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes beyond
what is documented in the plan's threat model (T-04-12 through T-04-SC). Specifically:

- `CheckAgent` and `CheckSigning` consume injected functions only; no direct `exec.Command`
  calls in `internal/doctor/checks/signing.go` (the real subprocess calls live in the cmd
  layer injected via `deps.RunSSHAdd` / `deps.RunSSHKeygenFingerprint`).
- No `import "github.com/castocolina/gitid/internal/filewriter"` in `internal/doctor/`.
- All findings have `Fix = nil` — write-free core preserved (D-01).

---

## Self-Check: PASSED

Files exist:
- `internal/doctor/checks/signing.go` — CheckSigning and CheckAgent each defined exactly once
- `internal/doctor/checks/signing_test.go` — 13 test functions

Commits verified:
- `9eeb04d` (RED) present
- `4d2d782` (GREEN) present

Build and test:
- `go build ./...` passes
- `go test ./internal/doctor/...` passes
- `grep -rn 'func CheckSigning\|func CheckAgent' internal/doctor/checks/` returns exactly 2 matches (both in signing.go)
- `grep -rn 'internal/filewriter' internal/doctor/` returns no import lines
