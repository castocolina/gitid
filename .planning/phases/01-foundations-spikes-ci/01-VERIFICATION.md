---
phase: 01-foundations-spikes-ci
verified: 2026-07-03T07:46:43Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Phase 1: Foundations, Spikes & CI — Verification Report

**Phase Goal:** Every non-UI capability, tool, and CI gate that later phases depend on
exists and is test-proven — with **no product UI** yet.
**Verified:** 2026-07-03T07:46:43Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Repeatable capture step produces versioned PNG screenshots of a TUI screen and an HTML page, build-tag isolated from the shipped binary | ✓ VERIFIED | Ran `make screenshot-tui` and `make screenshot-html` live (freeze v0.2.2 installed, Chromium 1321438 cached) — both `ok`. Golden hashes reproduce (asserted inside `TestCaptureTUI`/`TestCaptureHTML`). Versioned PNGs exist on disk: `.planning/design/_spike/tui/spike.png` (1773x400 PNG) and `.planning/design/_spike/html/spike.png` (1280x800 PNG). `go list -deps ./cmd/gitid \| grep -i "go-rod\|freeze"` returns nothing (exit 1) — capture backends never reach the shipped binary. |
| 2 | Real ed25519 + rsa-4096 keygen with correct perms, top-5 algorithm catalog with per-OS availability/troubleshooting, surfaced by a debug command | ✓ VERIFIED | Built `/tmp/gitidv` and ran `gitid debug caps`: printed structured Capabilities (openssh 9.7p1/LibreSSL 3.3.6, agent/fido/keychain three-valued statuses), a 5-row Algorithm Catalog (ed25519 default + rsa-4096 real/generatable; ecdsa-p256/ed25519-sk/ecdsa-sk registered-not-generatable stubs) with per-darwin notes, and a real Identities section reconstructed from my actual `~/.ssh/config`. `crypto/ed25519.GenerateKey` and `crypto/rsa.GenerateKey(…,4096)` confirmed in `internal/keygen/keygen.go`. Perms wired through the `filewriter` chokepoint: `EnsureDir(sshDir, 0o700)`, `Write(finalPriv, …, 0o600)`, `Write(finalPub, …, 0o644)` in `cmd/gitid/add.go`. `go test ./internal/keygen/... ./internal/platform/... -race` → both `ok`. |
| 3 | Dual SSH-config storage (in-file OR Include'd, adopt, reversible migrate) with backups + real `ssh -G` resolution | ✓ VERIFIED | `go test ./internal/sshconfig/... -race` → `ok`. Code has `Adopt`, `DetectInclude`, `Migrate` (both directions: `TestMigrateToIncludeMovesBlockAndPreservesResolution`, `TestMigrateToInFileMovesBlockBack`), idempotent re-run (`TestMigrateIdempotentReRunConverges`), rollback-on-injected-failure tests (`TestMigrateInjectedFailureAfterDestinationWrittenRollsBack`, `…AfterSourceTrimmedRollsBack`, `…RollbackDoesNotClobberPristineBackup`), and a real timeout test for a hung `ssh -G` (`TestMigrateReturnsTimeoutErrorWhenSSHHangs`). `snapshotResolution`/`validateResolution` in `migrate.go` drive real `ssh -G` before/after comparison. |
| 4 | Identity 8-state taxonomy computed by UI-free, TDD core from parsed managed blocks (no sidecar DB) | ✓ VERIFIED | `go test ./internal/identity/... -race` → `ok`. `internal/identity/state.go` defines exactly the 8 locked labels: `complete, incomplete, git-only, key-unused, key-used-ssh-only, key-used-both, key-missing, fragment-path-missing`. No `charm`/`bubbletea` imports anywhere in the package (only comments asserting UI-freedom). States are computed from `Reconstruct`/`BuildInventory` over parsed managed blocks — no DB/ORM import in the package. |
| 5 | GitHub Actions builds darwin/amd64, darwin/arm64, linux/amd64 (+arm64) and runs `make test` (race) + `make lint` + `make test-e2e` GREEN on macOS + Linux runners, reproducible from a fresh clone via `make setup-env`; workflow is SHA-pinned/least-privilege | ✓ VERIFIED | `gh run view 28645640620 --repo castocolina/gitid` (live query, not SUMMARY narration) shows: `check (ubuntu-latest)` ✓, `check (macos-15)` ✓, `check (macos-15-intel)` ✓, `build-cross (ubuntu-latest)` ✓ — all green, push-to-main event. `.github/workflows/ci.yml`: all 5 `uses:` steps pinned by 40-hex commit SHA (`grep -vE "@[0-9a-f]{40} #"` over `uses:` lines returns zero matches); `permissions: contents: read` at top level; no `secrets.` reference anywhere; every job step literally runs `make setup-env` / `make test` / `make lint` / `make test-e2e` / `make build-cross` (no inlined go/golangci commands). Locally ran `make build-cross` → 4 real cross-compiled binaries (darwin amd64 Mach-O x86_64, darwin arm64 Mach-O arm64, linux amd64 ELF x86-64, linux arm64 ELF aarch64). |

**Score:** 5/5 truths verified

### Phase Gates (run directly, not taken from SUMMARY)

| Gate | Command | Result |
|------|---------|--------|
| Unit/integration tests (race) | `make test` | `ok` on all 18 packages (cmd/gitid, internal/adopter, clipboard, deps, doctor(+checks), filewriter, gitconfig, identity, keygen, platform, repoclone, sshconfig, tester, upload, uploader, tui) — 0 failures |
| Lint | `make lint` | `golangci-lint run ./...` → `0 issues.` |
| E2E | `make test-e2e` | `go build -o bin/gitid ./cmd/gitid` + `go test -tags e2e -race -timeout 60s ./e2e/...` → `ok` (34.2s) |
| Screenshot tooling | `make screenshot-tui` / `make screenshot-html` | Both `ok` (golden hash reproduced) |
| Static vet | `go vet ./...` | clean, no output |
| Cross-compile | `make build-cross` | 4/4 correct-architecture binaries produced |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/platform/version.go`, `capabilities.go`, `keytypes.go` | Structured `SSHVersion`, three-valued `AgentStatus`/`FIDOStatus`/`KeychainStatus`, `BuildProbeDeps()` exported | ✓ VERIFIED | Present, substantive, exercised by passing tests and live `debug caps` output |
| `internal/keygen/*.go` | Real ed25519 + rsa-4096 generation, catalog, `Generatable()` guard | ✓ VERIFIED | `GenerateMaterial`/`generateEd25519`/`generateRSA4096` present; catalog surfaced live |
| `internal/sshconfig/*.go` | Adopt, Include, Migrate, backups | ✓ VERIFIED | All present, round-trip + rollback tests green |
| `internal/identity/state.go`, `inventory.go` | 8-state taxonomy, `BuildInventory` | ✓ VERIFIED | 8 labels exact match; no sidecar DB; UI-free |
| `internal/screenshot/{tui,html,determinism}.go` | Build-tag isolated capture backends | ✓ VERIFIED | `//go:build screenshot`; excluded from `go list -deps ./cmd/gitid` |
| `cmd/gitid/debug.go` | `debug caps` command | ✓ VERIFIED | Live-run output confirms catalog + probe + identity state |
| `.github/workflows/ci.yml`, `Makefile` (`build-cross`) | 3-runner CI + cross-build matrix | ✓ VERIFIED | Live green run (run 28645640620) + local `make build-cross` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `cmd/gitid debug caps` | `internal/platform.BuildProbeDeps()` / `internal/keygen` catalog / `internal/identity` inventory | direct Go calls | WIRED | Live binary run prints real probe + catalog + reconstructed identity from local `~/.ssh/config` |
| `.github/workflows/ci.yml` | `Makefile` targets | `run: make <target>` | WIRED | Every CI step is a bare `make` invocation; confirmed by `grep` and live `gh run view` |
| `cmd/gitid add` | `internal/filewriter` | `filewriter.Write(...)` with explicit perm args | WIRED | 0o700 dir / 0o600 private key / 0o644 public key applied at the actual write call sites |
| `internal/sshconfig.Migrate` | real `ssh -G` | `snapshotResolution`/`validateResolution` | WIRED | Timeout test (`TestMigrateReturnsTimeoutErrorWhenSSHHangs`) proves the real resolution path is exercised, not stubbed |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|---|---|---|---|
| PLAT-01, PLAT-02, KEY-03 | 01-01 | ✓ SATISFIED | `debug caps` prints SSHVersion, three-valued statuses, per-OS notes |
| KEY-01, KEY-02, KEY-04 | 01-02 | ✓ SATISFIED | Real keygen + catalog + perms confirmed |
| STORE-01..04, TOOL-04 | 01-03 | ✓ SATISFIED | Adopt/Include/Migrate + round-trip tests green |
| MGR-02, DLV-07 | 01-04 | ✓ SATISFIED | 8-state taxonomy, UI-free package |
| TOOL-05, DLV-03, TOOL-02 | 01-05 | ✓ SATISFIED | `make screenshot-tui`/`screenshot-html` run green, PNGs versioned |
| KEY-01, PLAT-01, MGR-02, DLV-07 | 01-06 | ✓ SATISFIED | Live `debug caps` output surfaces all three |
| BUILD-01, BUILD-02, BUILD-04, TOOL-01, TOOL-03 | 01-07 | ✓ SATISFIED | Live green CI run (28645640620), SHA-pinned workflow, `make build-cross` local repro |

**Note (documentation-only gap, non-blocking):** `.planning/REQUIREMENTS.md` still
shows `BUILD-01`, `BUILD-02`, `BUILD-04` as `[ ]` / `Pending` in its checklist and
coverage table, even though the underlying functionality is independently verified
above (live green CI run, SHA-pinned workflow, working `build-cross`). This is a
bookkeeping staleness in `REQUIREMENTS.md` (the `d1c6347` "finalize CI plan" commit
touched `ROADMAP.md` and the 01-07 SUMMARY but not `REQUIREMENTS.md`), not a
functional gap — the goal-backward evidence for Success Criterion 5 is solid. Flagged
for the requirements-tracking pass before Phase 1 is marked done in ROADMAP.md.

### Anti-Patterns Found

None. Scanned every file touched on this branch under `internal/platform`,
`internal/keygen`, `internal/sshconfig`, `internal/identity`, `internal/screenshot`,
`cmd/gitid`, `Makefile`, `.github` for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER|not yet
implemented|coming soon` — zero matches.

### Human Verification Required

None. Phase 1 is entirely non-UI (explicit goal constraint: "no product UI yet"), so
every success criterion is machine-checkable and was checked with real, observed
command output above.

### Gaps Summary

No gaps. All 5 ROADMAP Success Criteria are independently verified against the
codebase with live command output (not SUMMARY narration): screenshot tooling runs
and produces build-tag-isolated, versioned PNGs; keygen + catalog + probe are real
and surfaced by a working `debug caps` binary; dual SSH-config storage is round-trip
and rollback tested with real `ssh -G` resolution; the identity 8-state taxonomy is
UI-free and exactly matches the locked vocabulary; and CI is independently confirmed
green on all three runners via a live `gh run view` query, with a SHA-pinned,
least-privilege, no-secrets workflow that literally invokes the same `make` targets
verified locally (`make test` -race, `make lint` 0 issues, `make test-e2e`, `make
build-cross`). The only issue found is a non-blocking documentation staleness in
`REQUIREMENTS.md`'s BUILD-01/02/04 checkboxes, noted above for cleanup.

---

_Verified: 2026-07-03T07:46:43Z_
_Verifier: Claude (gsd-verifier)_
