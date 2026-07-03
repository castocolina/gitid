# Phase 1 (foundations-spikes-ci) — Security Audit

**Audited:** branch `gsd/phase-01-foundations-spikes-ci`, commit `d1c6347`
**Scope:** `.planning/phases/01-foundations-spikes-ci/01-01-PLAN.md` through
`01-07-PLAN.md` — 27 registered threats across 7 plans.
**Method:** adversarial — every threat assumed OPEN until a grep/code match in
the files cited by the plan's mitigation plan proved otherwise. Documentation
and SUMMARY.md self-reports were never accepted as evidence on their own;
every claim below is backed by a direct code citation re-verified in this
audit session. `go build ./...`, `go vet ./...`, and `go mod verify` were
re-run as corroborating evidence (all green).

**Threat Flags:** none. No `01-0N-SUMMARY.md` file contains a `## Threat
Flags` section, so there is no new attack surface reported by the executors
beyond the 27 threats already registered in the seven plans' `<threat_model>`
blocks.

## Verdict: SECURED — 27/27 threats CLOSED

---

## Threat Verification

### Plan 01-01 — Local capability probe layer (`internal/platform`)

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-01-01 | Tampering | mitigate | CLOSED | Arg-slice `exec.CommandContext` with `#nosec G204` on every probe: `internal/platform/capabilities.go:190` (`ssh-add -l`), `:221` (`ssh -Q key`); `internal/platform/version.go:52` (`ssh -V`); `internal/platform/platform.go:61` (`ssh -Q key`, `ProbeKeyTypes` retrofit). No shell string interpolation anywhere in the package. |
| T-01-02 | Spoofing | mitigate | CLOSED | `internal/platform/keytypes.go:9-23` — fixed `algorithmToken` map; `AlgorithmForToken` returns `""` for any token not in the map (verified: no substring/fuzzy match), so a manipulated PATH `ssh` cannot inject an unrecognized token as a false catalog entry. |
| T-01-03 | Denial of Service | mitigate | CLOSED | `internal/platform/version.go:17` `probeTimeout = 3 * time.Second` (package-wide bound, `var` so tests can shrink it); `context.WithTimeout` wraps every probe (`capabilities.go:187,218`; `version.go:50`; `platform.go:59`). `capabilities_test.go:148-160` proves a hung `ssh-add` returns promptly (bounded by `probeTimeout`) rather than blocking, and `probeFIDO`/`probeAgent` map absence/timeout to a non-fatal status (`FIDOAbsent`, `AgentLockedOrUnavailable`), never an error. |

### Plan 01-02 — Algorithm registry + catalog (`internal/keygen`)

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-01-04 | Information Disclosure | mitigate | CLOSED | `internal/keygen/keygen.go:38` — `PrivPEM is private key material and must never be logged or printed` doc contract preserved; the 01-06 debug command never references it (`! grep -q "PrivPEM\|Material{\|os.Environ" cmd/gitid/debug.go` — reconfirmed in this audit, zero matches). |
| T-01-05 | Information Disclosure | mitigate | CLOSED | Every real persistence call routes key material through the filewriter chokepoint at explicit modes: `cmd/gitid/add.go:558` (`filewriter.Write(finalPriv, mat.PrivPEM, 0o600)`), `:562` (pub, `0o644`), `:583`, `:586` (staged variant). `internal/keygen/keygen_test.go:238-266` (`TestPermissions_KeyFilesAfterRegistryRefactor`) re-proves 0600/0644 for both `ed25519` and `rsa-4096` post-refactor. |
| T-01-06 | Tampering | mitigate | CLOSED | `internal/keygen/keygen.go` uses only `ssh.MarshalPrivateKey`/`MarshalPrivateKeyWithPassphrase`/`ssh.NewPublicKey` from `golang.org/x/crypto/ssh` (no hand-rolled OpenSSH serialization); `go.mod:17` pins `golang.org/x/crypto v0.53.0` exactly, matching CLAUDE.md's stack pin. |
| T-01-21 | Spoofing | mitigate | CLOSED | `internal/keygen/catalog.go:59-60` — `Generatable(a) = a.Implemented && a.Available` (both facts required); `internal/keygen/registry.go:31-45` — `notYetImplemented` stubs registered for `ecdsa-p256`/`ed25519-sk`/`ecdsa-sk` always return a zero `Material` + a named error, so registry presence never implies generation support, confirmed by `catalog_test.go`'s per-Implemented=false-entry error assertion. |

### Plan 01-03 — Dual SSH-config storage (`internal/sshconfig`, `internal/doctor/checks`)

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-01-07 | Tampering | mitigate | CLOSED | `internal/sshconfig/adopt.go:191-193` (`isAcceptablePathForm` — absolute or `~/.ssh/`-relative only), `:253-294` (`candidateTarget` — sentinel-bearing check via `filewriter.ListBlocks`, ambiguous-glob rejection unless caller-chosen, `os.Lstat` symlink guard at `:286-292`). All four selection rules present and enforced at the boundary. |
| T-01-08 | Tampering | mitigate | CLOSED | `internal/sshconfig/include.go:88` (`filewriter.Write(configPath, composed, includeFileMode)`) with a pre-write `Parse` round-trip check (`:84-86`); `internal/sshconfig/migrate.go` routes every write through `deps.WriteFile` (wired to `filewriter.Write`/`filewriter.WriteNoBackup` in `RealMigrateDeps`, `:149,190`). `grep -q "os.WriteFile" internal/sshconfig/include.go internal/sshconfig/migrate.go internal/sshconfig/adopt.go` returns no matches (only a negative doc-comment reference in include.go:71). |
| T-01-22 | Tampering (integrity) | mitigate | CLOSED | `internal/sshconfig/migrate.go` implements the full 5-step transaction (preflight `:231-253`, both-file backup `:255-273`, destination-write-first `:275-292`, source-trim-second `:294-313`, commit `:315-327`) with add-before-remove ordering. Rollback (`:488-499`) restores from **in-memory** pristine bytes captured at preflight via the dedicated `RestoreFile` (`filewriter.WriteNoBackup`) seam — never re-entering `Write`'s own backup step — so a rollback can never clobber the on-disk step-2 backup it is restoring from (Codex HIGH #1, closed). `RealMigrateDeps.ResolveAlias` (`:150-183`) runs `ssh -G` under `exec.CommandContext` + `context.WithTimeout(migrateResolveTimeout)`, with a process-group `Setpgid`+`SIGKILL` kill and a 500ms `cmd.WaitDelay` belt-and-suspenders bound against a grandchild holding the stdout pipe past the deadline (Linux-observed hang, fixed per 01-07-SUMMARY.md's CI defect log). |
| T-01-09 | Denial of Service | mitigate | CLOSED | `internal/doctor/checks/orphans.go:59` — `if sshconfig.IsReservedBlockName(name) { continue }` at the top of the Class 1 loop, mirroring the pre-existing Class 2 gitconfig guard (`:96`), added in the same change as `sshconfig.IsReservedBlockName` (`internal/sshconfig/include.go:42-44`). |
| T-01-10 | Information Disclosure | mitigate | CLOSED | `t.TempDir()` used 11× in `include_test.go`, 7× in `adopt_test.go`, 5× in `migrate_test.go`; no reference to a real `~/.ssh` path found in any of the three test files. |

### Plan 01-04 — Identity state taxonomy (`internal/identity`)

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-01-11 | Tampering | **accept** | CLOSED (logged below) | `internal/identity/state.go` contains no `os.Stat`/`os.ReadFile`/`os.Open` (re-grepped, zero matches) — `Classify`/`ClassifyState` are pure functions over caller-supplied facts; no write, no exec, no sidecar DB. Accepted-risk rationale logged in "Accepted Risks" below. |
| T-01-12 | Information Disclosure | mitigate | CLOSED | `internal/identity/state_test.go` and `inventory_test.go` contain no `PrivPEM`/`-----BEGIN` PEM literal (re-grepped, zero matches); fixtures use synthetic paths under `t.TempDir()`/literal placeholder bytes (`inventory_test.go:215`, `"fake-private-key-material\n"`). |
| T-01-23 | Tampering | mitigate | CLOSED | `internal/identity/inventory.go:27-44` (`InventoryDeps` — every effect an injected function field), `:132` (`BuildInventoryDeps`, real wiring); `inventory_test.go:168` (`TestBuildInventoryDeps`) asserts every returned function field is non-nil, closing the injected-seam blindspot for the real constructor (not just fakes). |

### Plan 01-05 — Screenshot capture tooling (`internal/screenshot`)

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-01-SC | Tampering | mitigate | CLOSED | `go.mod:13` pins `github.com/go-rod/rod v0.116.2` exactly (no range); `go mod verify` re-run in this audit reports `all modules verified`; `.planning/design/_spike/GOLDENS.md` § 1 records the provenance review (slopcheck, isolated-compile, Codex independent re-confirmation) for both `freeze@v0.2.2` and `go-rod v0.116.2`. |
| T-01-SC2 | Tampering | mitigate | CLOSED | `internal/screenshot/html.go:19-25` — `ChromiumRevision = launcher.RevisionDefault`, pinned and re-exported explicitly (never trusts a future go-rod default drift); `:57-65` — fixed `CacheDir` + `AllowDownload` gate; `:180-183+` (`resolveBrowserBinary`) fails fast with an actionable error naming the revision + cache path when `AllowDownload=false` and the revision is not cached — never silently falls back to a different browser. GOLDENS.md records the exact pinned revision (`1321438`) and cache path. |
| T-01-13 | Elevation of Privilege | mitigate | CLOSED | `//go:build screenshot` on `internal/screenshot/tui.go:1`, `html.go:1`, `determinism.go:1` (and documented in `doc.go:11`). Re-verified in this audit: `go list -deps ./cmd/gitid \| grep -i "charmbracelet/freeze\|go-rod/rod"` returns no matches — neither dev dependency enters the shipped binary's dependency graph. |
| T-01-14 | Tampering | mitigate | CLOSED | `internal/screenshot/tui.go:97` — `exec.Command(freezeBin, args...)` arg-slice form with a `//nolint:gosec` annotation naming the G204 mitigation (fixed args/paths, `freezeBin` resolved via `exec.LookPath`/explicit config, never user input). |

### Plan 01-06 — `gitid debug caps` command (`cmd/gitid`, `e2e`)

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-01-15 | Information Disclosure | mitigate | CLOSED | `cmd/gitid/debug.go` never references `PrivPEM`/`Material{`/`os.Environ` (re-grepped, zero matches); output is limited to catalog metadata, structured probe status strings, and `IdentityHealth` state/problem labels (`printCapabilities`/`printCatalog`/`printInventory`, `:86-149`). Asserted twice: unit test `cmd/gitid/debug_test.go:153-156` (absence of "PRIVATE KEY", "passphrase", "Passphrase", "PrivPEM") and e2e test `e2e/debug_e2e_test.go:95-98` (same four substrings, against the real built binary's stdout). |
| T-01-16 | Spoofing | mitigate | CLOSED | `e2e/debug_e2e_test.go:13-14` states the contract ("must exercise the REAL platform.BuildProbeDeps + identity.BuildInventoryDeps wiring... not fakes") and drives the real built binary (`cmd.Env` at `:46`); `cmd/gitid/debug.go:57` wires `platform.BuildProbeDeps()` + `identity.BuildInventoryDeps()` directly in `runDebugCaps` (the non-test entry point). |
| T-01-17 | Tampering | mitigate | CLOSED | `cmd/gitid/debug.go` contains zero `exec.` calls of its own (re-grepped) — the command is thin glue that inherits every exec surface (and its T-01-01 timeout/arg-slice mitigations) from `internal/platform`, adding no new exec surface. |

### Plan 01-07 — Cross-OS CI (`.github/workflows/ci.yml`, `Makefile`)

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-01-SC3 | Tampering | mitigate | CLOSED | `Makefile:34` pins a `GOLANGCI_LINT_VERSION` var (v2.12.2 per CLAUDE.md); `:109` `go install github.com/charmbracelet/freeze@v0.2.2` (exact pin, not `@latest`); `go.mod:13` pins `go-rod v0.116.2`. CI (`ci.yml:83-84`) reuses `make setup-env` verbatim — no separate/drifted CI-only install path. |
| T-01-18 | Tampering | mitigate | CLOSED | `.github/workflows/ci.yml:51,54,73,76` — every `uses:` step (`actions/checkout`, `actions/setup-go`, both jobs) is pinned to a full 40-hex commit SHA with a trailing `# vX.Y.Z` comment. Re-verified in this audit: `grep -E "uses: .*@[0-9a-f]{40}"` matches all four `uses:` lines; `grep -E "uses: .*@v[0-9]+$"` (tag-only form) matches none. |
| T-01-24 | Elevation of Privilege | mitigate | CLOSED | `ci.yml:37-38` — top-level `permissions: { contents: read }` block declared; `grep -c "secrets\."` on the workflow file returns `0`. |
| T-01-19 | Elevation of Privilege | **accept** | CLOSED (logged below) | No mitigation code applicable by design (accepted risk) — rationale logged in "Accepted Risks" below, matching the disposition text in `01-07-PLAN.md`'s `<threat_model>`. |
| T-01-20 | Denial of Service | mitigate | CLOSED | `ci.yml:70` matrix uses `[ubuntu-latest, macos-15-intel, macos-15]`; `grep -E "macos-1[34]"` on the workflow file returns no matches. `01-07-SUMMARY.md` records a real green run on all three runners (https://github.com/castocolina/gitid/actions/runs/28645640620), confirming the corrected labels actually schedule successfully — not just a lint-level absence check. |

---

## Accepted Risks Log

Per `<threat_model>` disposition `accept`, the following two threats have no
mitigation code by design. Logging them here satisfies the "accept" disposition's
verification requirement (an entry present in this log).

### T-01-11 — Tampering — `internal/identity` Classify/ClassifyState inputs

**Component:** `internal/identity/state.go` — `Classify`/`ClassifyState` pure
functions.
**Rationale (from `01-04-PLAN.md`):** Read-only pure functions; no writes, no
exec, no sidecar DB — a malformed `Account` input yields a (possibly
inaccurate) report, never a mutation of any file or system state. Low risk
for a local single-user tool: the worst outcome is a misleading debug-output
label, not data loss or privilege escalation.
**Accepted by:** phase-level plan authorship (`01-04-PLAN.md` `<threat_model>`).
**Status:** Accepted, no further action required for Phase 1.

### T-01-19 — Elevation of Privilege — untrusted PR code running in CI

**Component:** `.github/workflows/ci.yml` — `pull_request` trigger runs
untrusted contributor code (`make test`, `make lint`, `make test-e2e`) on
GitHub-hosted runners.
**Rationale (from `01-07-PLAN.md`):** Solo-developer, local-use tool;
standard GitHub-hosted-runner isolation applies; the workflow token is
already read-only (`permissions: contents: read`, T-01-24); no secrets are
referenced or required for any Phase-1 gate (release/publish with secrets is
deferred to Phase 10, out of scope here).
**Accepted by:** phase-level plan authorship (`01-07-PLAN.md` `<threat_model>`).
**Status:** Accepted, no further action required for Phase 1. Re-evaluate
if/when Phase 10 introduces secrets to the workflow (a repository secret
would change this threat's risk profile and should be re-scored, not
inherited as still-accepted).

---

## Corroborating Checks (not threat-specific, cross-cutting)

- **No shell command injection anywhere in the module:** every `exec.Command`/
  `exec.CommandContext` call site in the repository (28 call sites checked,
  spanning `cmd/gitid`, `internal/platform`, `internal/sshconfig`,
  `internal/keygen`, `internal/gitconfig`, `internal/tester`,
  `internal/repoclone`, `internal/deps`, `internal/screenshot`, `tui/`) uses
  the arg-slice form with a `#nosec`/`nolint:gosec` G204 annotation. None
  interpolate untrusted input into a shell string.
- **Build health:** `go build ./...`, `go vet ./...`, and `go mod verify`
  all re-run clean in this audit session (2026-07-03).
- **Filewriter collision-proofing (supports T-01-08/T-01-22):**
  `internal/filewriter/filewriter.go:130-149` (`backupExistingTarget`) uses
  a `<path>.bak.<UnixNano>` naming scheme with an `O_EXCL` exclusive create
  (`:220`, `copyFileExclusive`), retried up to `maxBackupCollisionAttempts`
  (100) on a same-nanosecond collision — a backup can never silently
  overwrite an existing backup/recovery snapshot.

## Out-of-scope observation (non-blocking, not part of this phase's threat register)

`cmd/gitid/doctor.go:217` (`exec.Command("ssh-add", "-l")`) and `:236`
(`exec.Command("ssh-keygen", "-lf", path)`) use the arg-slice form (no
injection risk) but do **not** run under `exec.CommandContext` with a bounded
timeout, unlike the 01-01 probe layer. `doctor.go` is not in any of the 7
Phase-1 plans' `files_modified` lists and is not named in any Phase-1
`<threat_model>` mitigation plan — it predates this phase and is out of this
audit's scope. Flagged here for future-phase awareness only; not a Phase 1
BLOCKER.

---

## Summary

| Disposition | Count | Closed | Open |
|---|---|---|---|
| mitigate | 25 | 25 | 0 |
| accept | 2 | 2 (logged) | 0 |
| transfer | 0 | — | — |
| **Total** | **27** | **27** | **0** |

**Verdict: SECURED.** All 27 declared threats across the 7 Phase 1 plans
resolve to CLOSED — 25 by verified mitigation code (file:line evidence
above) and 2 by a logged accepted-risk entry matching the plan's own
disposition and rationale. No unregistered new attack surface was reported
by any plan's SUMMARY.md. No BLOCKER findings.
