---
phase: 2
slug: first-identity-end-to-end
status: ratified
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-09
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `02-RESEARCH.md` §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (+ `-race`) |
| **Config file** | none — `go test` convention; Phase-1 `_stub_test.go` files exist per package |
| **Quick run command** | `go test ./internal/<pkg>/...` |
| **Full suite command** | `make test` (`go test -race -coverprofile=coverage.out ./...`) |
| **Estimated runtime** | ~10–30 seconds (unit tier; integration excluded) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/<pkg>/...` for the touched package
- **After every plan wave:** Run `make test` (full `-race` suite)
- **Before `/gsd-verify-work`:** `make test` + `make lint` (golangci-lint + gosec) green
- **Max feedback latency:** ~30 seconds (unit tier)

---

## Per-Task Verification Map

| Requirement | Behavior | Test Type | Automated Command | File Exists |
|-------------|----------|-----------|-------------------|-------------|
| KEY-02 / SAFE-03 | backup + temp→rename→chmod; modes 0600/0644; restore on error | unit | `go test ./internal/filewriter/...` | ✅ |
| SAFE-01 | timestamped backup created before write | unit | `go test ./internal/filewriter/... -run Backup` | ✅ |
| SAFE-02 | idempotent block rewrite; second write = identical bytes; foreign content preserved | unit | `go test ./internal/sshconfig/... -run Idempotent` | ✅ |
| IDENT-01 / KEY-01(gen) | ed25519 gen → valid OpenSSH PEM + authorized line | unit | `go test ./internal/keygen/...` | ✅ |
| SIGN-01 (line) | allowed_signers line `<email> namespaces="git" ssh-ed25519 …`, email byte-match | unit | `go test ./internal/keygen/... -run Signers` | ✅ |
| SIGN-01 (file write) | line persisted to `~/.ssh/allowed_signers` (0644) in idempotent per-identity managed block; re-run = empty diff; other identities preserved | unit | `go test ./internal/keygen/... -run AllowedSigners` | ✅ |
| SIGN-01 (orchestration) | `identity.Create` invokes all FOUR writers incl. WriteAllowedSigners on a confirmed write | unit | `go test ./internal/identity/... -run Create` | ✅ |
| SIGN-02 | user.signingkey is a path, never inline | unit | `go test ./internal/gitconfig/... -run SigningKey` | ✅ |
| SSH-01/02 | rendered Host block has Hostname/Port/User git/IdentityFile/IdentitiesOnly yes | unit | `go test ./internal/sshconfig/... -run Render` | ✅ |
| SSH-03 | macOS Host* block: IgnoreUnknown→UseKeychain→AddKeysToAgent, ordered last | unit | `go test ./internal/sshconfig/... -run Global` | ✅ |
| GIT-01/02 | includeIf block (gitdir trailing slash + hasconfig) renders, points to fragment | unit | `go test ./internal/gitconfig/... -run Include` | ✅ |
| GIT-03 | fragment sets user.name/email, gpg.format=ssh, signingkey, commit.gpgsign | unit | `go test ./internal/gitconfig/... -run Fragment` | ✅ |
| D-09 | `ssh -Q key` probe parsing + fallback chain selection | unit (parse fixed output) | `go test ./internal/platform/... -run Probe` | ✅ |
| TEST-01/02 | output-substring classifier maps the 3 D-01 outcomes; ssh -G key parse | unit (fixture strings) | `go test ./internal/tester/...` | ✅ |
| TEST-02 (entry point) | `gitid identity test <name>` re-runs the resolved test (handler buildable + panic-guarded) | unit | `go test ./cmd/gitid/... -run Test` | ✅ |
| TEST-03 | result carries input command string + raw output | unit | `go test ./internal/tester/... -run Echo` | ✅ |
| CLIP-02 | graceful no-tool failure path | unit | `go test ./internal/clipboard/...` | ✅ |
| SSH-03 / Pitfall 4 | generated config does not error `ssh -G` on **Linux** | integration | `ssh -G testalias` exit 0 in Linux container | ❌ manual/CI → Phase 6 |
| GIT-02 / Pitfall 7 | `git config user.email` resolves inside `~/git/<id>/repo/` via gitid's rendered includeIf block | integration | `go test ./internal/gitconfig/... -run TestIncludeIfGitdir_ResolvesViaRealGit` | ✅ |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky — task IDs assigned by the planner per PLAN.md.*

---

## Wave 0 Requirements

- [x] `internal/filewriter/filewriter_test.go` — backup, atomic rename, chmod, restore (SAFE-01/03, KEY-02) + `block_test.go` ReplaceBlock idempotency
- [x] `internal/keygen/keygen_test.go` — ed25519 gen, PEM shape, authorized line (IDENT-01) + `derive_test.go`
- [x] `internal/keygen/signers_test.go` — allowed_signers line byte-match + idempotent `~/.ssh/allowed_signers` file write (SIGN-01, SAFE-02)
- [x] `internal/sshconfig/{renderer,parser}_test.go` — block render, idempotency, Host* ordering, round-trip (SSH-01/02/03, SAFE-02)
- [x] `internal/gitconfig/{renderer,fragment}_test.go` — includeIf, fragment, no-`[remote]` guard (GIT-01/02/03, SIGN-02)
- [x] `internal/platform/platform_test.go` — `ssh -Q key` parse + fallback selection (D-09); per-OS hint (D-14)
- [x] `internal/tester/tester_test.go` — output-substring classifier on fixtures, ssh -G parse (TEST-01/02/03)
- [x] `internal/clipboard/clipboard_test.go` — graceful failure (CLIP-02)
- [x] `internal/identity/identity_test.go` — Create orchestration with injected fakes; asserts all four writers (incl. WriteAllowedSigners) invoked (SIGN-01 orchestration) + `modes_test.go`
- [x] `cmd/gitid/test_test.go` — recover panic-guard for the `gitid identity test` handler (TEST-02 entry point)
- [ ] Linux integration check for `ssh -G` non-error (Pitfall 4) — **deferred to Phase 6 (Linux cross-platform validation)**; cannot run meaningfully on macOS
- [x] `gitdir:` resolution integration check with real `git` (Pitfall 7) — `internal/gitconfig/includeif_resolve_test.go::TestIncludeIfGitdir_ResolvesViaRealGit` (added 2026-06-09)
- [x] Framework: none to install — Go stdlib `testing` is in place; stub tests already green

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Full create → resolved `ssh -G` shows expected identityfile against a real provider | TEST-02 (e2e) | Requires a real provider account + network; D-02 gates the resolved test on the user uploading the key first | Create identity, upload `.pub`, run `gitid identity test <name>`; confirm `ssh -G <alias>` resolves the expected key and `ssh -T` shows "successfully authenticated" |
| `git log --show-signature` shows "Good signature" on a test commit | SIGN (e2e) | Requires an uploaded signing key on the provider AND the written `~/.ssh/allowed_signers` line | Inside `~/git/<id>/repo`, make a commit, run `git log --show-signature`; confirm "Good signature" (depends on the allowed_signers file written by `WriteAllowedSigners`) |
| Clipboard contains the `.pub` after generate / on demand | CLIP-01 | Reading the OS clipboard is environment-dependent | After create, paste; confirm it matches `<key>.pub` |
| Upload steps are followable without external docs | UP-01/UP-02 | Human judgment of instruction clarity | Follow printed steps to add auth + signing keys on GitHub/GitLab |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (incl. the new `keygen/signers_test.go` for SIGN-01 file write and `cmd/gitid/test_test.go` for the `identity test` entry point)
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ratified — 2026-06-09 (after adding the allowed_signers Wave-0 file-write test and the `identity test` entry-point test). `wave_0_complete: true` reflects that all Wave-0 test files are listed and assigned in the plans (test files are created RED-first during execution per TDD).

---

## Validation Audit 2026-06-09

Post-execution audit (`/gsd-validate-phase 2`). Cross-referenced every requirement
against the test files that now exist and run green (`make test -race`, full suite
12/12 packages ok; `make lint` 0 issues).

| Metric | Count |
|--------|-------|
| Requirements audited | 19 |
| COVERED (automated, green) | 18 |
| Gaps found | 1 (GIT-02 / Pitfall 7 — was manual/CI) |
| Resolved | 1 — `TestIncludeIfGitdir_ResolvesViaRealGit` (commit d1487ba) |
| Escalated → manual-only | 0 |
| Remaining manual-only / deferred | 1 (SSH-03 / Pitfall 4 — Linux `ssh -G`, deferred to Phase 6) + the 4 provider-network e2e items below |

**Outcome: VALIDATED (PARTIAL)** — 18/19 requirements automated; the single
remaining integration gap (Linux `ssh -G` non-error) is environment-bound and
belongs to Phase 6. The plan-time `❌ W0` placeholders are now all ✅ (test files
created during execution, verified green here). Naming note: several plan-time
`-run` hints differed from the final test names (e.g. `-run Backup` →
`TestWriteBacksUpExistingTarget`, `-run Probe` → `TestParseKeyTypes`/
`TestSelectAlgorithm`, `-run Echo` → `TestPreWrite_CapturesCommandAndOutput`) —
coverage confirmed by function, not filename.
