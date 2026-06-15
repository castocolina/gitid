---
phase: 03-full-identity-crud-multi-identity
verified: 2026-06-10T00:00:00Z
status: passed
score: 4/4 success criteria verified; 4/4 requirements satisfied
overrides_applied: 0
re_verification: false
---

# Phase 3: Full Identity CRUD + Multi-Identity Verification Report

> **Addendum 2026-06-10 (post-verification independent review).** After this PASSED
> verdict, an independent code review (superpowers:requesting-code-review) found **3
> Critical + 1 Important bug this verification missed** — CRLF-silent-no-op delete,
> orphan allowed_signers sentinels, missing-fragment delete leaving the signer line,
> and a dropped update Provider prompt. All four were confirmed and fixed (commits
> `41b86a7`, `2386b6e`, `7f1bf98`); see `03-REVIEW.md` "Second review round". The
> phase goal verdict below stands, but note this verification's blind spot: it did not
> exercise CRLF inputs or write/remove symmetry for `allowed_signers`. Suite + lint
> re-confirmed green after the fixes.

**Phase Goal:** Users can list, update, and delete identities; two identities on the same provider coexist via distinct aliases and each resolves to its own key; the tool reconstructs all state from managed blocks with no sidecar database.
**Verified:** 2026-06-10
**Status:** PASSED (with post-verification fixes — see addendum)
**Re-verification:** No — initial verification

## Test Suite Result

```
go test ./... -count=1  (all 12 packages)
ok  github.com/castocolina/gitid/cmd/gitid
ok  github.com/castocolina/gitid/internal/filewriter
ok  github.com/castocolina/gitid/internal/gitconfig
ok  github.com/castocolina/gitid/internal/identity
ok  github.com/castocolina/gitid/internal/sshconfig
(+ 7 other packages — all green)

go test ./... -race -count=1  — all 12 packages PASS with race detector.
make lint — 0 issues (golangci-lint v2 + gosec).
go build ./... — clean.
```

## Goal Achievement

### Observable Truths (Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC-1 | `gitid identity list` displays all identities with key path, alias, provider, port, match strategy | VERIFIED | `cmd/gitid/list.go:runIdentityList` calls `identity.Reconstruct` then `printAccounts`, which renders `acct.KeyPath`, `acct.Alias`, `provider`, `acct.Port` (when != 0), and `renderMatches(acct.Matches)`. Incomplete marker printed when `acct.Incomplete != ""`. Test: `TestRunIdentityList_EmptyConfigs` PASS |
| SC-2 | Two identities on the same provider coexist; `ssh -G <alias-A>` / `<alias-B>` resolve to distinct IdentityFiles | VERIFIED | `internal/sshconfig/coexistence_test.go:TestMultiIdentityCoexistence` — hermetic `ssh -G -F <configPath>` for two aliases on `ssh.github.com:443`; assertion `personalRC.IdentityFiles[0] != workRC.IdentityFiles[0]`. Test PASS |
| SC-3 | Deleting an identity removes its managed blocks from all four artifacts while preserving all content outside those blocks verbatim | VERIFIED | `internal/identity/delete.go:Delete` calls `filewriter.RemoveBlock(bytes, acct.Name)` for SSH and gitconfig (never `_global`). `RemoveBlock` (block.go:59-93, WR-04 fixed) does NOT consume a trailing blank line — foreign separator preserved byte-for-byte. Tests: `TestDelete_GlobalAndForeignPreserved` and `TestDelete_RemoveBlockUsedForSSHAndGitconfig` PASS |
| SC-4 | Cold-start reconstruction from managed blocks with no sidecar DB | VERIFIED | `internal/identity/loader.go:Reconstruct` joins `sshconfig.ParseManagedHosts` + `gitconfig.ParseManagedIncludeIf` by identity name (D-01), calls `readFrag` for fragment data, no database dependency. Definitive proof: `TestReconstruct_RoundTrip` writes two identities via Phase 2 pipeline then reconstructs and asserts all fields — PASS |

**Score:** 4/4 success criteria verified

### Required Artifacts

| Artifact | Expected | Status | Evidence |
|----------|----------|--------|----------|
| `internal/filewriter/block.go` | `ListBlocks` + `RemoveBlock` | VERIFIED | `func ListBlocks` at line 18; `func RemoveBlock` at line 59; WR-04 fix: no blank-line consumption after `afterEnd` |
| `internal/filewriter/filewriter.go` | `BackupAndRemove` | VERIFIED | `func BackupAndRemove` at line 91; uses `os.Rename` (atomic); missing file returns `("", nil)` |
| `internal/sshconfig/reader.go` | `ParseManagedHosts` + implicit Host * guard | VERIFIED | `func ParseManagedHosts` at line 27; Pitfall A guard at line 58: `len(host.Patterns)==1 && host.Patterns[0].String()=="*"` |
| `internal/gitconfig/reader.go` | `ParseManagedIncludeIf` + `ReadFragment` + `RemoveAllowedSignersLine` | VERIFIED | All three present; CR-01 fix at line 127: `fields[0] == identityEmail` (exact principal, not substring); `namespaces="git"` also required |
| `internal/identity/loader.go` | `Reconstruct` + `nameUnion` | VERIFIED | `func Reconstruct` at line 17; `func nameUnion` at line 83; joins by identity name |
| `internal/identity/identity.go` | `Account.Incomplete string` field | VERIFIED | Line 49: `Incomplete string` — additive, non-breaking |
| `cmd/gitid/list.go` | `newListCmd` + `runIdentityList` | VERIFIED | Both present; calls `identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)` |
| `internal/sshconfig/coexistence_test.go` | `TestMultiIdentityCoexistence` | VERIFIED | Present; uses `ssh -G -F` hermetic pattern; asserts `IdentityFiles[0] !=` |
| `internal/identity/update.go` | `Update` + `UpdateDeps` + `UpdateResult` | VERIFIED | All three; D-04: `edited.Name = existing.Name` at line 50; D-05: structural gate at lines 53-55 |
| `cmd/gitid/update.go` | `newUpdateCmd` + `runIdentityUpdate` + `buildUpdateDeps` | VERIFIED | All three present; WR-02 tilde expansion in `readPubLine`/`expandTilde` |
| `internal/gitconfig/fragment.go` | `WriteFragment(signing bool)` with `unset-all` | VERIFIED | Signature includes `signing bool`; `gitConfigUnsetAll` at line 105 handles Pitfall C (exit 5) |
| `internal/identity/delete.go` | `Delete` + `DeleteDeps` + `DeleteResult` | VERIFIED | All three; CR-02 fix: `RemoveKeyFiles` signature returns `(keyBackup, pubBackup string, err error)` — wired to `filewriter.BackupAndRemove` in cmd layer |
| `cmd/gitid/delete.go` | `newDeleteCmd` + `runIdentityDelete` + `buildDeleteDeps` | VERIFIED | All three; two `confirm(` calls (lines 125 + 133); manifest print at lines 107-113 |
| `cmd/gitid/main.go` | All commands registered | VERIFIED | Lines 40-45: `newAddCmd`, `newListCmd`, `newTestCmd`, `newRotateCmd`, `newUpdateCmd`, `newDeleteCmd` all registered |

### Key Link Verification

| From | To | Via | Status | Evidence |
|------|----|-----|--------|----------|
| `cmd/gitid/list.go` | `identity.Reconstruct` | `os.ReadFile` → `Reconstruct(.., gitconfig.ReadFragment)` | WIRED | list.go:54: `identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)` |
| `cmd/gitid/main.go` | `newListCmd` | `identity.AddCommand(newListCmd())` | WIRED | main.go:41 |
| `cmd/gitid/update.go` | `identity.Update` | `buildUpdateDeps` → `identity.Update(existing, edited, deps, signing)` | WIRED | update.go:183 |
| `cmd/gitid/main.go` | `newUpdateCmd` | `identity.AddCommand(newUpdateCmd())` | WIRED | main.go:44 |
| `internal/identity/update.go` | `tester.Resolved` | structural-change gate | WIRED | update.go:100: `deps.Resolved(edited.Alias)` — only when `structural == true` |
| `cmd/gitid/delete.go` | `identity.Delete` | `buildDeleteDeps` → `identity.Delete(acct, keepKey, deps)` | WIRED | delete.go:136 |
| `cmd/gitid/main.go` | `newDeleteCmd` | `identity.AddCommand(newDeleteCmd())` | WIRED | main.go:45 |
| `internal/identity/delete.go` | `filewriter.RemoveBlock + BackupAndRemove + gitconfig.RemoveAllowedSignersLine` | per-identity artifact removal | WIRED | delete.go:61 (RemoveBlock SSH), 73 (RemoveBlock GC), 81 (RemoveFragment via dep), 88 (RemoveAllowedSigners via dep); buildDeleteDeps wires `filewriter.BackupAndRemove` and `gitconfig.RemoveAllowedSignersLine` |
| `internal/identity/loader.go` | `sshconfig.ParseManagedHosts + gitconfig.ParseManagedIncludeIf` | join by identity-name key | WIRED | loader.go:22 (`ParseManagedHosts`), 26 (`ParseManagedIncludeIf`) |
| `internal/sshconfig/reader.go` | `filewriter.ListBlocks` | enumerate managed blocks | WIRED | reader.go:28: `filewriter.ListBlocks(content)` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| `cmd/gitid/list.go` `printAccounts` | `accounts []identity.Account` | `identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)` where bytes are read from real `~/.ssh/config` / `~/.gitconfig` | Yes — `ParseManagedHosts` parses real ssh_config blocks; `ReadFragment` calls `git config --file --list`; `TestReconstruct_RoundTrip` proves the full chain | FLOWING |
| `cmd/gitid/delete.go` manifest | `acct identity.Account` | Same `Reconstruct` pipeline; account found by name match | Yes — same flow | FLOWING |
| `cmd/gitid/update.go` | `existing identity.Account` | Same `Reconstruct` pipeline | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Reconstruct round-trip | `go test ./internal/identity/... -run TestReconstruct_RoundTrip -v` | PASS (0.25s) | PASS |
| Multi-identity coexistence (SC-2) | `go test ./internal/sshconfig/... -run TestMultiIdentityCoexistence -v` | PASS (0.09s) | PASS |
| Delete: keepKey gate (D-07) | `go test ./internal/identity/... -run TestDelete_KeepKey -v` | PASS | PASS |
| Delete: global + foreign preserved (SC-3, D-08) | `go test ./internal/identity/... -run TestDelete_GlobalAndForeignPreserved -v` | PASS | PASS |
| Update: fragment-only no re-test (D-05) | `go test ./internal/identity/... -run TestUpdate_FragmentOnly -v` | PASS | PASS |
| Update: structural triggers re-test (D-05) | `go test ./internal/identity/... -run TestUpdate_Structural -v` | PASS | PASS |
| Update: name immutable (D-04) | `go test ./internal/identity/... -run TestUpdate_NameImmutable -v` | PASS | PASS |
| WriteFragment signing toggle | `go test ./internal/gitconfig/... -run TestWriteFragment -v` | 10 subtests PASS | PASS |
| Full suite with race detector | `go test ./... -race -count=1` | all 12 packages ok | PASS |
| Lint | `make lint` | 0 issues | PASS |

### Code-Review Fix Verification

All 5 fixes declared in 03-REVIEW.md (status: issues_fixed) are confirmed present in the actual code:

| Finding | Commit | Fix Present in Code | Verified |
|---------|--------|---------------------|---------|
| CR-01: `RemoveAllowedSignersLine` substring over-match | 4a92045 | `gitconfig/reader.go:127` — exact `fields[0] == identityEmail` first-field principal comparison; also requires `namespaces="git"` | YES |
| CR-02: key deletion not backed up | db79a13 | `cmd/gitid/delete.go:205-211` — `buildDeleteDeps.RemoveKeyFiles` routes both private and public key through `filewriter.BackupAndRemove`; `DeleteResult` carries `KeyBackup`/`PubBackup` fields | YES |
| WR-02: tilde not expanded before reading pub | 159dc5d | `internal/identity/update.go:117-144` — `readPubLine` calls `expandTilde(pubPath)` before `os.ReadFile`; `expandTilde` documented and exported | YES |
| WR-04: `RemoveBlock` consumed foreign blank-line separator | 9c13048 | `internal/filewriter/block.go:87-91` — `afterEnd = endIdx + 1` only; no extra line consumed; comment explicitly documents WR-04 fix | YES |
| WR-06: port defaulted to 22 instead of 0 | 7ef012f | `internal/sshconfig/reader.go:71` — `port := 0`; Atoi only overwrites on success; comment: "Port 0 means unset"; `list.go:98` — `if acct.Port != 0` before printing | YES |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| IDENT-03 | 03-02-PLAN.md | User can list identities with wiring | SATISFIED | `cmd/gitid/list.go` + `identity.Reconstruct`; `[x]` in REQUIREMENTS.md |
| IDENT-04 | 03-03-PLAN.md | User can update identity fields | SATISFIED | `cmd/gitid/update.go` + `internal/identity/update.go`; D-04/D-05/D-06 enforced; `[x]` in REQUIREMENTS.md |
| IDENT-05 | 03-04-PLAN.md | User can delete identity with backup | SATISFIED | `cmd/gitid/delete.go` + `internal/identity/delete.go`; two-step confirm; BackupAndRemove for keys (CR-02); `[x]` in REQUIREMENTS.md |
| IDENT-07 | 03-01-PLAN.md | Reconstruct identity list from managed blocks, no sidecar DB | SATISFIED | `internal/identity/loader.go:Reconstruct`; `TestReconstruct_RoundTrip` is the definitive proof; `[x]` in REQUIREMENTS.md |

No orphaned requirements — REQUIREMENTS.md traceability table lists all four as Phase 3 / Complete.

### Anti-Patterns Found

No TBD, FIXME, or XXX markers found in any of the 11 phase-modified source files.

No stub patterns detected — all functions produce real data flow.

### Context Decisions (D-01..D-08) Honored

| Decision | What it requires | Honored |
|----------|-----------------|---------|
| D-01 | Join key is identity name across ssh + gitconfig | YES — `nameUnion` over both maps |
| D-02 | Partial identity sets carry `Incomplete` marker, never silently dropped | YES — `missing` slice joined and set on `acct.Incomplete` |
| D-03 | list columns: key path, alias, provider, port, match strategy | YES — all rendered in `printAccounts` |
| D-04 | Identity name is immutable in update | YES — `edited.Name = existing.Name` forced in `Update` |
| D-05 | Re-test only on structural change | YES — `structural` gate; `TestUpdate_FragmentOnly` proves Resolved NOT called |
| D-06 | Backup → preview → single confirm → idempotent whole-block rewrite | YES — both update and delete follow this; `confirm` before any write |
| D-07 | Private key kept by default; separate explicit second prompt to delete | YES — `delete.go:133`: second `confirm` for key deletion |
| D-08 | Only acct.Name passed to RemoveBlock; shared/global blocks never touched | YES — `delete.go:61,73` pass only `acct.Name`; `_global` never referenced in delete.go |

### Human Verification Required

None — all success criteria are verifiable programmatically and have been verified above. The only human-relevant check (visual appearance of `gitid identity list` output against a real config) is informational, not a blocker for goal achievement.

### Deferred Items (Known Follow-Ups from Review)

The following findings from 03-REVIEW.md were explicitly deferred as non-blocking by the review. They are not gaps for this phase:

| Item | Finding | Why Deferred |
|------|---------|-------------|
| WR-01 | Delete not transactional — partial failure leaves half-deleted identity | No rollback required by Phase 3 scope; partial-progress backups still exist |
| WR-03 | `RemoveAllowedSignersLine` always rewrites even on no-op | Minor inefficiency; idempotency claim is functionally correct (removes the right line) |
| WR-05 | Fragment-only identities invisible to reconstruction | Out of scope per D-01/D-02; Phase 4 doctor can surface orphans |
| WR-07 | `promptPort` silently discards invalid input | UX quality; does not affect correctness of written config |
| IN-01..IN-05 | Error prefix duplication, label wording, code duplication, ReadFragment error collapse, defaultHostname fallback | All cosmetic or minor DX; no correctness impact |

---

_Verified: 2026-06-10_
_Verifier: Claude (gsd-verifier)_
