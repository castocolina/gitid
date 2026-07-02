---
phase: 1
slug: foundations-spikes-ci
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-02
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `01-RESEARCH.md` § Validation Architecture. Refined by the Nyquist auditor at execution time.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `go test -race` (no third-party test framework) |
| **Config file** | none — `go.mod` + `.golangci.yml`; e2e tests gated by the `e2e` build tag |
| **Quick run command** | `go test -race ./internal/keygen/... ./internal/platform/... ./internal/sshconfig/... ./internal/identity/...` (scoped to touched packages) |
| **Full suite command** | `make test` (unit, `-race`, all packages) + `make test-e2e` (real binary, `//go:build e2e`) |
| **Estimated runtime** | ~30–90 seconds (unit); e2e adds a build |

---

## Sampling Rate

- **After every task commit:** the scoped quick-run command for the package(s) the task touched (e.g. `go test -race ./internal/keygen/...`).
- **After every plan wave:** `make test` (`-race`) + `make lint` + `make test-e2e`.
- **Before `/gsd-verify-work`:** full suite green locally AND ≥1 real GitHub Actions run green on all three corrected runners (`ubuntu-latest`, `macos-15-intel`, `macos-15`).
- **Max feedback latency:** ~90 seconds (unit quick-run).

---

## Per-Task Verification Map

| Req ID | Behavior | Test Type | Automated Command | File Exists |
|--------|----------|-----------|-------------------|-------------|
| KEY-02 | rsa-4096 generation → valid OpenSSH PEM + correct pub line | unit | `go test ./internal/keygen/... -run TestGenerateRSA4096 -race` | ❌ W0 |
| KEY-02 | Algorithm registry dispatches by name; unknown algo errors cleanly | unit | `go test ./internal/keygen/... -run TestRegistry -race` | ❌ W0 |
| KEY-04 | Generated key files land at correct perms (600/644) | unit | `go test ./internal/keygen/... -run TestPermissions -race` | ✅ extend |
| PLAT-01 | `ssh -Q key` → algorithm mapping incl. `sk-ssh-ed25519@openssh.com` | unit | `go test ./internal/platform/... -run TestKeyTypeMapping -race` | ❌ W0 |
| PLAT-01 | `ssh -V` parses OpenSSH + SSL flavor (LibreSSL/OpenSSL) | unit | `go test ./internal/platform/... -run TestParseSSHVersion -race` | ❌ W0 |
| PLAT-01 | libfido2/agent/keychain probes injectable + mockable | unit | `go test ./internal/platform/... -run TestCapabilities -race` | ❌ W0 |
| STORE-01 | Include line placed as floor (top), idempotent on re-run | unit | `go test ./internal/sshconfig/... -run TestEnsureIncludeLine -race` | ❌ W0 |
| STORE-01 | Real `ssh -G` resolves through the Include'd file (first-match-wins) | integration | `go test ./internal/sshconfig/... -run TestIncludeResolution -race` (real `ssh` + `t.TempDir()`) | ❌ W0 |
| STORE-02 | Detect existing external Include directive, adopt its path | unit | `go test ./internal/sshconfig/... -run TestAdoptExistingInclude -race` | ❌ W0 |
| STORE-03 | Migrate in-file ↔ Include'd, backup created both directions | integration | `go test ./internal/sshconfig/... -run TestMigrate -race` | ❌ W0 |
| MGR-02 | All 8 states computed correctly from fixture managed-block configs | unit (table) | `go test ./internal/identity/... -run TestClassifyState -race` | ❌ W0 |
| TOOL-05 | `make screenshot-tui` → non-empty PNG from a fixture `View()` golden | smoke | `make screenshot-tui && test -s .planning/design/_spike/tui/*.png` | ❌ W0 |
| TOOL-05 | `make screenshot-html` → non-empty PNG from a fixture HTML page | smoke | `make screenshot-html && test -s .planning/design/_spike/html/*.png` | ❌ W0 |
| BUILD-01 | `make build` cross-compiles darwin/amd64, darwin/arm64, linux/amd64 | infra | `GOOS=… GOARCH=… make build` per target + CI matrix green | N/A infra |
| BUILD-02/04 | CI gates pass on `ubuntu-latest`, `macos-15-intel`, `macos-15` | infra | Push branch, observe GitHub Actions status | N/A infra |

*Status legend: ✅ green · ❌ red / to-write · W0 = Wave 0 stub required.*

---

## Wave 0 Requirements

- [ ] `internal/keygen/registry_test.go` (or extend `keygen_test.go`) — KEY-02 registry dispatch + rsa-4096
- [ ] `internal/platform/capabilities_test.go` — PLAT-01 libfido2/agent/keychain probes (injectable seam)
- [ ] `internal/platform/version_test.go` (or extend `platform_test.go`) — `ssh -V` parse (LibreSSL + OpenSSL fixtures)
- [ ] `internal/sshconfig/include_test.go` — STORE-01 Include-line floor placement + idempotency
- [ ] `internal/sshconfig/adopt_test.go` — STORE-02 detection
- [ ] `internal/sshconfig/migrate_test.go` — STORE-03 reversible migration (`t.TempDir()` real-FS fixtures)
- [ ] `internal/identity/state_test.go` — MGR-02, table-driven over all 8 states
- [ ] `Makefile` targets `screenshot-tui` / `screenshot-html` — do not exist yet
- [ ] `.github/workflows/ci.yml` — does not exist yet
- [ ] vendored monospace font for deterministic `freeze` rendering

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Cross-OS CI matrix green | BUILD-02 | Requires a real GitHub Actions run on hosted macOS + Linux runners | Push a branch; confirm all three runners pass test/lint/e2e |
| `ssh -G` real resolution | STORE-01 | Depends on the real local `ssh` binary; covered by an integration test but the live-machine result is the ground truth | `ssh -G -F <tmp> <alias>` shows the expected `identityfile` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
