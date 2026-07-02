---
phase: 4
slug: doctor
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-11
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, table-driven) |
| **Config file** | none — `go.mod` toolchain; Makefile targets |
| **Quick run command** | `go test ./internal/doctor/...` |
| **Full suite command** | `make test` (`go test -race ./...` + coverage) |
| **Estimated runtime** | ~10–30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/doctor/...` (and the touched cmd package)
- **After every plan wave:** Run `make test`
- **Before `/gsd-verify-work`:** `make test` + `make lint` must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Every check family has observable input→output (a fake `doctor.Deps` in, a `[]Finding` out),
> so nearly all logic is unit-testable; CLI rendering + exit-code aggregation are testable via
> captured stdout + return code. Each row's Automated Command is copied from that task's
> `<verify><automated>` block. Wave 2 plans OVERWRITE the Plan 01 per-family stub files in place
> (no redeclaration), so each Wave 2 row also asserts `go build ./...` passes after the overwrite.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | DOC-06, DOC-07 | T-04-03 | Finding/Severity model + Deps; severity→exit-code aggregation (highest wins); write-free core; six per-family stub files compile | unit | `go test ./internal/doctor/... -run 'TestExitCode\|TestFindingFields\|TestRunCallsAllFamilies\|TestFamiliesFixedOrder\|TestSeverityString' && go build ./...` | ❌ W0 | ⬜ pending |
| 04-01-02 | 01 | 1 | DOC-02 | T-04-01, T-04-02 | Permissions check vs KEY-02 targets; G304 on Stat; Fix.Fn tightens only (never widens) | unit | `go test ./internal/doctor/checks/... -run TestCheckPerms` | ❌ W0 | ⬜ pending |
| 04-01-03 | 01 | 1 | DOC-07 | T-04-01, T-04-04 | `gitid doctor` registered + grouped renderer; NO_COLOR honored; tiered exit code; FixPerm injected (no filewriter in core) | unit | `go test ./cmd/gitid/... -run TestDoctor && go build ./... && go run ./cmd/gitid doctor >/dev/null` | ❌ W0 | ⬜ pending |
| 04-02-01 | 02 | 2 | DOC-01 | T-04-06 | CheckDeps stub OVERWRITTEN; required→error+per-OS hint, optional clipboard→info; install hints report-only | unit | `go test ./internal/platform/... ./internal/doctor/checks/... -run 'TestInstallHint\|TestCheckDeps' && go build ./... && go test ./internal/doctor/...` | ❌ W0 | ⬜ pending |
| 04-02-02 | 02 | 2 | DOC-01 (D-16) | T-04-05, T-04-07 | CheckBaseline stub OVERWRITTEN; D-16 four checks via ReadBaselineState; ignorecase=false carve-out; [fix] markers injected | unit | `go test ./internal/doctor/checks/... -run TestBaseline && go build ./... && go test ./internal/doctor/...` | ❌ W0 | ⬜ pending |
| 04-03-01 | 03 | 2 | DOC-03 | T-04-08, T-04-10 | CheckCoherence stub OVERWRITTEN; existence/resolution + gpg.format/email carve-outs; byte-exact email `==` (Pitfall 6) | unit | `go test ./internal/doctor/checks/... -run TestCoherence && go build ./... && go test ./internal/doctor/...` | ❌ W0 | ⬜ pending |
| 04-03-02 | 03 | 2 | DOC-04 | T-04-09, T-04-11 | CheckOrphans stub OVERWRITTEN; block-vs-disk cross-ref; unused-key warning (no [fix]); no known_hosts read; hand-written hosts spared | unit | `go test ./internal/doctor/checks/... -run TestOrphan && go build ./... && go test ./internal/doctor/...` | ❌ W0 | ⬜ pending |
| 04-04-01 | 04 | 2 | DOC-05 | T-04-12, T-04-13 | CheckSigning+CheckAgent stub OVERWRITTEN; agent probe (code AND text); fingerprint match; git<2.36 hasconfig gate; arg-slice exec (G204) | unit | `go test ./internal/doctor/checks/... -run 'TestClassifyAgentState\|TestExtractFingerprint\|TestAgent\|TestGitVersionGate' && go build ./... && go test ./internal/doctor/...` | ❌ W0 | ⬜ pending |
| 04-05-01 | 05 | 3 | DOC-06 | T-04-18 | D-04 gate/per-finding/--yes flow; perms batched, orphans/wiring individual; **pre-fix exit code** (--fix --yes on critical env exits 3) | unit | `go test ./cmd/gitid/... -run 'TestDoctorFix\|TestDoctorGate\|TestDoctorPerms\|TestDoctorOrphan'` | ❌ W0 | ⬜ pending |
| 04-05-02 | 05 | 3 | DOC-06 | T-04-16, T-04-17, T-04-21 | RemoveBlock + AddWiring via EXISTING writers only (no new sshconfig/gitconfig fn); filewriter chokepoint backup+atomic+idempotent; core write-free | unit/integration | `go test ./cmd/gitid/... -run 'TestFixer' && make test 2>&1 \| tail -5` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/doctor/*_test.go` — table-driven tests per check family, driven by a fake `doctor.Deps`
- [ ] Fake `doctor.Deps` fixtures (fake ssh-add output, fake FileInfo perms, fake reconstructed identities)
- [ ] Framework already present (`go test`) — no install needed

*Existing `go test` infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live ssh-agent state reflected in report | DOC-05 | Real agent socket state is environment-specific | `ssh-add -l; gitid doctor` — agent section matches actual loaded keys |
| Real permission finding + chmod fix on a real `~/.ssh` | DOC-02 | Touches the real home dir; CI fakes perms | Set a key to 0644, run `gitid doctor` (warns/criticals), `gitid doctor --fix` restores 600 |

*Most behaviors have automated verification via fakes; the two above are confirmed manually against a live environment.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending (Wave 0 test scaffolding created during execution)
