---
phase: 3
slug: full-identity-crud-multi-identity
status: planned
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-10
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`) |
| **Config file** | none — Makefile targets + golangci-lint v2 config (already installed in Phase 1) |
| **Quick run command** | `go test ./internal/... ./cmd/...` |
| **Full suite command** | `make test` (runs `go test -race` + coverage, matching the pre-push hook) |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/filewriter/... ./internal/sshconfig/... ./internal/gitconfig/... ./internal/identity/... ./cmd/...` (the packages this phase touches)
- **After every plan wave:** Run `make test`
- **Before `/gsd-verify-work`:** Full suite (`make test`) must be green and `make lint` (golangci-lint + gosec) must pass
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

> Every task verifying an IDENT requirement maps to an automated `go test`
> command. TDD tasks create their own RED test (Wave-0 gap closed in-task).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01-T1 | 01 | 1 | IDENT-07 | T-03-04 | ListBlocks skips incomplete blocks; RemoveBlock preserves foreign content byte-for-byte; idempotent | unit (tdd) | `go test ./internal/filewriter/... -count=1` | ❌ W0 (created in-task) | ⬜ pending |
| 03-01-T2 | 01 | 1 | IDENT-07 | T-03-01,T-03-02 | implicit `Host *` skipped; allowed_signers match requires email+namespace; arg-slice git exec | unit (tdd) | `go test ./internal/sshconfig/... ./internal/gitconfig/... -count=1` | ❌ W0 (created in-task) | ⬜ pending |
| 03-01-T3 | 01 | 1 | IDENT-07 | T-03-03 | reconstruct join by name; partial sets flagged (Incomplete) not dropped; round-trip fidelity | unit (tdd) | `go test ./internal/identity/... -run TestReconstruct -count=1` | ❌ W0 (created in-task) | ⬜ pending |
| 03-02-T1 | 02 | 2 | IDENT-03 | T-03-06,T-03-07 | list prints key PATHS only; missing config read as empty (no crash) | unit | `go test ./cmd/... -run TestRunIdentityList -count=1` | ❌ W0 (created in-task) | ⬜ pending |
| 03-02-T2 | 02 | 2 | SC-2 | T-03-08 | two same-provider aliases resolve to distinct IdentityFiles via hermetic `ssh -G -F` | integration | `go test ./internal/sshconfig/... -run TestMultiIdentityCoexistence -count=1` | ❌ W0 (created in-task) | ⬜ pending |
| 03-03-T1 | 03 | 3 | IDENT-04 | T-03-10 | WriteFragment omits signing keys when signing=false (exit-5-safe unset) | unit (tdd) | `go test ./internal/gitconfig/... -run TestWriteFragment -count=1` | ❌ W0 (created in-task) | ⬜ pending |
| 03-03-T2 | 03 | 3 | IDENT-04 | T-03-12,T-03-13 | resolved re-test only on structural change (D-05); name immutable (D-04); signing-off removes line | unit (tdd) | `go test ./internal/identity/... -run TestUpdate -count=1` | ❌ W0 (created in-task) | ⬜ pending |
| 03-03-T3 | 03 | 3 | IDENT-04 | T-03-09,T-03-11 | name validated; single confirm before write; dry-run previews | unit | `go test ./cmd/... -run TestRunIdentityUpdate -count=1` | ❌ W0 (created in-task) | ⬜ pending |
| 03-04-T1 | 04 | 4 | IDENT-05 | T-03-15,T-03-16 | only acct.Name removed; `_global`/foreign preserved; key kept unless keepKey=false | unit (tdd) | `go test ./internal/identity/... -run TestDelete -count=1` | ❌ W0 (created in-task) | ⬜ pending |
| 03-04-T2 | 04 | 4 | IDENT-05 | T-03-14,T-03-16 | will-remove manifest before single confirm; separate default-no key-delete prompt | unit | `go test ./cmd/... -run TestRunIdentityDelete -count=1` | ❌ W0 (created in-task) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

All Wave-0 test files are created in-task by their owning TDD/auto task (RED first
for `type: tdd` tasks), so no separate Wave-0 plan is needed. Files created:

- [ ] `internal/filewriter/block_list_test.go` + `internal/filewriter/filewriter_remove_test.go` — ListBlocks/RemoveBlock/BackupAndRemove (03-01-T1)
- [ ] `internal/sshconfig/reader_test.go` — ParseManagedHosts (03-01-T2)
- [ ] `internal/gitconfig/reader_test.go` — ParseManagedIncludeIf/ReadFragment/RemoveAllowedSignersLine (03-01-T2)
- [ ] `internal/identity/loader_test.go` — Reconstruct complete/partial/empty + round-trip (03-01-T3)
- [ ] `cmd/gitid/list_test.go` — runIdentityList (03-02-T1)
- [ ] `internal/sshconfig/coexistence_test.go` — TestMultiIdentityCoexistence (03-02-T2)
- [ ] `internal/gitconfig/fragment_signing_test.go` — WriteFragment signing toggle (03-03-T1)
- [ ] `internal/identity/update_test.go` — Update fragment-only/structural (03-03-T2)
- [ ] `cmd/gitid/update_test.go` — runIdentityUpdate (03-03-T3)
- [ ] `internal/identity/delete_test.go` — Delete keep-key/scope (03-04-T1)
- [ ] `cmd/gitid/delete_test.go` — runIdentityDelete (03-04-T2)

*Framework already present — no install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `ssh -G <alias-A>` vs `ssh -G <alias-B>` resolve to distinct IdentityFiles | IDENT-04 / SC-2 | Automated via `ssh -G -F <fixtureConfig>` (03-02-T2) where `ssh` is available; manual ONLY if `ssh` absent in CI | Create two identities on one provider; run `ssh -G -F <config> <alias>` for each; assert different `identityfile` lines |
| `gitid identity list` columns match real config | IDENT-03 | Optional end-of-phase human smoke against the developer's real config | Run `gitid identity list`; confirm key path/alias/provider/port/match per identity |

*Most behaviors have automated verification via injected Deps + fixture configs.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (created in-task)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (closed in-task by owning task)
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** planned (pending execution)
