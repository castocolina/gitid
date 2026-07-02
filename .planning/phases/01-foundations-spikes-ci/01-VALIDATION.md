---
phase: 1
slug: foundations-spikes-ci
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-02
updated: 2026-07-02
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `01-RESEARCH.md` § Validation Architecture. Refined by the Nyquist auditor at execution time.
> Updated after the Codex cross-AI review to cover the added transaction/inventory/determinism tests.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `go test -race` (no third-party test framework) |
| **Config file** | none — `go.mod` + `.golangci.yml`; e2e tests gated by the `e2e` build tag; screenshot tests gated by the `screenshot` build tag |
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
| KEY-02 | Algorithm registry dispatches by name; unknown algo errors cleanly; stubs return ZERO Material | unit | `go test ./internal/keygen/... -run TestRegistry -race` | ❌ W0 |
| KEY-01 | Stub (Implemented=false) algorithm is never Generatable and its generation path errors | unit | `go test ./internal/keygen/... -run 'TestGeneratable\|TestCatalog' -race` | ❌ W0 |
| KEY-04 | Generated key files land at correct perms (600/644) | unit | `go test ./internal/keygen/... -run TestPermissions -race` | ✅ extend |
| PLAT-01 | `ssh -Q key` → algorithm mapping incl. `sk-ssh-ed25519@openssh.com` | unit | `go test ./internal/platform/... -run TestKeyTypeMapping -race` | ❌ W0 |
| PLAT-01 | `ssh -V` parses OpenSSH + SSL flavor into an SSHVersion STRUCT (LibreSSL/OpenSSL) | unit | `go test ./internal/platform/... -run TestParseSSHVersion -race` | ❌ W0 |
| PLAT-01 | agent/FIDO/keychain probes injectable + three-valued statuses; real wiring non-nil | unit | `go test ./internal/platform/... -run 'TestCapabilities\|TestProbeDepsWiring' -race` | ❌ W0 |
| PLAT-01 | every external probe is CommandContext-timeout-bounded (hung probe returns promptly) | unit | `go test ./internal/platform/... -run TestProbeTimeout -race` | ❌ W0 |
| STORE-01 | Include line placed as floor (top), idempotent; config.d dir 0700 + Include file 0600 | unit | `go test ./internal/sshconfig/... -run 'TestEnsureIncludeLine\|TestEnsureIncludeDir' -race` | ❌ W0 |
| STORE-01 | Real `ssh -G` resolves through the Include'd file (first-match-wins) | integration | `go test ./internal/sshconfig/... -run TestIncludeResolution -race` (real `ssh` + `t.TempDir()`) | ❌ W0 |
| STORE-02 | Detect Includes in order; adopt only under selection rules (sentinel/caller-chosen, absolute/`~/.ssh`, non-symlink, non-broad-glob) | unit (table) | `go test ./internal/sshconfig/... -run 'TestDetectInclude\|TestAdopt' -race` | ❌ W0 |
| STORE-03 | Cross-file transactional migrate; both-file backup; behavior-preserving `ssh -G` | integration | `go test ./internal/sshconfig/... -run TestMigrate -race` | ❌ W0 |
| STORE-03 | Injected failure AFTER each write step → no block loss + recoverable (backup restore / idempotent re-run) | unit (fault-injection) | `go test ./internal/sshconfig/... -run TestMigrate -race` (afterStep hook rows) | ❌ W0 |
| MGR-02 | 8 labels + overlap cases computed as IdentityHealth (both axes + Problems) from fixtures | unit (table) | `go test ./internal/identity/... -run 'TestClassify\|TestClassifyState' -race` | ❌ W0 |
| MGR-02 | State-inventory builder gathers real facts behind an injectable seam; real wiring non-nil | unit | `go test ./internal/identity/... -run 'TestBuildInventory\|TestBuildInventoryDeps' -race` | ❌ W0 |
| TOOL-05 | `make screenshot-tui` → deterministic PNG (vendored font, fixed theme/geometry, stripped metadata) reproducing a golden hash | smoke + unit | `make screenshot-tui && go test -tags screenshot ./internal/screenshot/... -run TestDeterminism -race` | ❌ W0 |
| TOOL-05 | `make screenshot-html` → deterministic PNG (fixed viewport/scale/color, pinned Chromium revision) reproducing a golden hash | smoke | `make screenshot-html && test -s .planning/design/_spike/html/*.png` | ❌ W0 |
| TOOL-02 | Supply-chain: pinned versions verified against the Go checksum DB (no `@latest`) | infra | `go mod verify && ! grep -rq "@latest" go.mod Makefile` | ❌ W0 |
| DLV-07 | Debug command consumes BuildInventory + never leaks secrets (PEM/passphrase/PrivPEM/env) | unit + e2e | `go test ./cmd/gitid/... -run TestDebug -race` + `make test-e2e` | ❌ W0 |
| BUILD-01 | `make build-cross` cross-compiles darwin/amd64, darwin/arm64, linux/amd64 (+ linux/arm64) once on Linux | infra | `make build-cross && test -x bin/gitid-linux-amd64` + CI matrix green | N/A infra |
| BUILD-02/04 | SHA-pinned, least-privilege CI gates pass on `ubuntu-latest`, `macos-15-intel`, `macos-15` | infra | Push branch, observe GitHub Actions status | N/A infra |

*Status legend: ✅ green · ❌ red / to-write · W0 = Wave 0 stub required.*

---

## Wave 0 Requirements

- [ ] `internal/keygen/registry_test.go` — KEY-02 registry dispatch + rsa-4096 + stub-returns-zero-Material
- [ ] `internal/keygen/catalog_test.go` — KEY-01 catalog + Generatable(Implemented AND Available) + stub-generation-errors
- [ ] `internal/platform/capabilities_test.go` — PLAT-01 agent/FIDO/keychain three-valued statuses (injectable seam) + CommandContext timeout behavior
- [ ] `internal/platform/version_test.go` — `ssh -V` parse into SSHVersion struct (LibreSSL + OpenSSL fixtures)
- [ ] `internal/sshconfig/include_test.go` — STORE-01 Include-line floor + idempotency + config.d dir(0700)/file(0600) perms
- [ ] `internal/sshconfig/adopt_test.go` — STORE-02 table-driven selection rules (multi-Include order, glob, quoted, `~/.ssh`-relative, bare-relative reject, symlink reject)
- [ ] `internal/sshconfig/migrate_test.go` — STORE-03 cross-file transaction + `afterStep` fault-injection (no loss + recoverable), `t.TempDir()` real-FS fixtures
- [ ] `internal/identity/state_test.go` — MGR-02 IdentityHealth over all 8 labels + ≥2 overlap rows
- [ ] `internal/identity/inventory_test.go` — MGR-02 BuildInventory (fakes) + BuildInventoryDeps real-wiring non-nil
- [ ] `internal/screenshot/determinism_test.go` — TOOL-05 metadata-strip idempotence + stable SHA-256 (build tag: screenshot)
- [ ] `cmd/gitid/debug_test.go` / `e2e/debug_e2e_test.go` — DLV-07 inventory consumption + broadened no-leak assertions
- [ ] `Makefile` targets `screenshot-tui` / `screenshot-html` / `build-cross` — do not exist yet
- [ ] `.github/workflows/ci.yml` — does not exist yet (SHA-pinned, least-privilege, cost-tiered)
- [ ] vendored monospace font for deterministic `freeze` rendering + `.planning/design/_spike/GOLDENS.md` golden hashes

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Cross-OS CI matrix green | BUILD-02 | Requires a real GitHub Actions run on hosted macOS + Linux runners | Push a branch; confirm the check job passes on all three runners (test/lint everywhere; e2e per PR/push tier) |
| `ssh -G` real resolution | STORE-01 | Depends on the real local `ssh` binary; covered by an integration test but the live-machine result is the ground truth | `ssh -G -F <tmp> <alias>` shows the expected `identityfile` |

*Note: the former 01-05 supply-chain HUMAN checkpoint was replaced by automated `go mod verify` + pinned-version provenance review; the CI-green confirmation (BUILD-02) is now the single human checkpoint of the milestone.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
