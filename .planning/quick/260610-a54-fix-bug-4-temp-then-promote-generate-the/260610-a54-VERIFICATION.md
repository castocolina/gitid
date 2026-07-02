---
phase: quick-260610-a54
verified: 2026-06-10T00:00:00Z
status: passed
score: 6/6 must-haves verified
overrides_applied: 0
---

# Quick Task: BUG-4 Temp-Then-Promote Verification Report

**Task Goal:** Generate the key to a temp location, run the pre-write gate against it, and persist to ~/.ssh ONLY after the gate passes AND the user confirms — so `--dry-run` and gate-failure aborts leave ~/.ssh byte-for-byte untouched (no orphan key), while a confirmed create persists the key to its final ~/.ssh path. Generate paths (create-new + rotate) only; reuse/add-account (existing key) unchanged.

**Verified:** 2026-06-10
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A --dry-run create-new run leaves ~/.ssh byte-for-byte untouched while still producing the four artifact previews | VERIFIED | `runPipeline` returns early (`res.PreWriteOnly = true`) at line 247-250 of `identity.go` before `PersistKey` or any writer is called; `TestCreateDryRun_PersistKeyCountZero` asserts `log.persistKey == 0`, `log.cleanup == 1`, and all four previews non-empty; corroborated by live binary test (user-verified). |
| 2 | A gate-Failure abort on create-new leaves NO orphan key pair in ~/.ssh | VERIFIED | `runPipeline` returns error at line 222-226 before the `PersistKey` branch; `TestCreateGateFailure_PersistKeyCountZero` asserts `log.persistKey == 0`, `log.cleanup == 1`, all four writer counts == 0; corroborated by live binary test (user-verified). |
| 3 | A confirmed create-new run persists the key pair to the FINAL ~/.ssh path via filewriter (backup+atomic+chmod 0600/0644) | VERIFIED | `buildDeps.PersistKey` (add.go lines 354-373) guards `staged.PrivPEM == nil`, then calls `filewriter.Write(staged.FinalPrivatePath, staged.PrivPEM, 0o600)` and `filewriter.Write(staged.FinalPubPath, []byte(staged.PubLine), 0o644)` — no `os.Rename`, no `os.ReadFile`; `TestCreateConfirmed_PersistKeyCountOneAndFinalPaths` asserts count == 1 and fires before WriteSSH. |
| 4 | The rendered SSH host block and gitconfig fragment reference the FINAL ~/.ssh key path, never the temp staging path | VERIFIED | `runPipeline` builds `final := KeyResult{PrivatePath: staged.FinalPrivatePath, ...}` at line 229-233, then calls `sshconfig.RenderHostBlock(..., final.PrivatePath)` (line 234) and `renderFragmentPreview(..., final.PubPath)` (line 242); `TestCreateConfirmed_PersistKeyCountOneAndFinalPaths` asserts `SSHPreview` contains `finalPath` and does NOT contain `tempPath`; `TestCreateGate_UsesTempPath` confirms `PreWrite` receives `TempPrivatePath` (`/tmp/stage/key`) not the final path. |
| 5 | Reuse and AddAccount complete against an existing on-disk key with no generation and PersistKey fake count == 0 | VERIFIED | `Reuse` (modes.go lines 30-37) constructs `StagedKey{PrivPEM: nil, TempPrivatePath: existingKeyPath, FinalPrivatePath: existingKeyPath}`. `AddAccount` (modes.go lines 109-116) does likewise. `runPipeline` skips `PersistKey` when `staged.PrivPEM == nil` (identity.go line 255). `TestReuseNoPersistKey` and `TestAddAccountNoPersistKey` both assert `log.persistKey == 0`. |
| 6 | Rotate persists the new key only on the confirmed (gate-passed) path | VERIFIED | `Rotate` (modes.go lines 131-139) calls `deps.Generate(in)` then `defer deps.Cleanup(staged)` then `runPipeline`; the `PrivPEM != nil` guard at identity.go line 255 ensures `PersistKey` fires only in the confirmed branch; `TestRotatePersistKeyOnConfirm` sub-test "confirmed" asserts count == 1; sub-test "gate-failure" asserts count == 0. |

**Score:** 6/6 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/keygen/keygen.go` | `GenerateMaterial(Params) (Material, error)` — pure in-memory (no disk) + `KeyPaths` helper | VERIFIED | `func GenerateMaterial` at line 49; performs no `filewriter.Write`; `func KeyPaths` at line 87; old disk-writing `Generate` absent (grep found no `func Generate\b`). |
| `internal/identity/identity.go` | `StagedKey` type, `Deps.Generate/PersistKey/Cleanup` fields; `runPipeline` gates on temp, renders final, persists only when `PrivPEM != nil && Confirmed` | VERIFIED | `StagedKey` struct lines 93-106; `Deps` fields `Generate`, `PersistKey`, `Cleanup` lines 117-125; `runPipeline` PreWrite call line 221 uses `staged.TempPrivatePath`; `final` built at lines 229-233; `if staged.PrivPEM != nil { deps.PersistKey(staged) }` at lines 255-259. |
| `cmd/gitid/add.go` | `buildDeps` wires `Generate` to `keygen.GenerateMaterial` + hermetic temp staging; `PersistKey` writes final paths via `filewriter`; `Cleanup` removes temp dir guarded for existing keys | VERIFIED | `Generate` closure lines 321-352 calls `keygen.GenerateMaterial`, `os.MkdirTemp`, `filewriter.Write` to stage; `PersistKey` lines 354-373 guards `PrivPEM == nil` then calls `filewriter.Write` for both final paths; `Cleanup` lines 374-380 guards `PrivPEM == nil || TempPrivatePath == FinalPrivatePath` before `os.RemoveAll`. |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `runPipeline` | `deps.PreWrite` | `staged.TempPrivatePath` | WIRED | identity.go line 221: `pre := deps.PreWrite(staged.TempPrivatePath, in.Hostname, in.Port)` |
| `runPipeline` | `deps.PersistKey` | confirmed branch only, before the four writers | WIRED | identity.go lines 255-259: `if staged.PrivPEM != nil { if _, perr := deps.PersistKey(staged); ... }` — positioned before `WriteSSH` call at line 261 |
| `runPipeline` | `sshconfig.RenderHostBlock` | `final.PrivatePath` (never temp path) | WIRED | identity.go line 234: `hostBlock := sshconfig.RenderHostBlock(in.Alias, in.Hostname, in.Port, final.PrivatePath)` where `final.PrivatePath = staged.FinalPrivatePath` |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| dry-run PersistKey count 0 + previews produced | `go test ./internal/identity/... -run TestCreateDryRun_PersistKeyCountZero -race` | PASS (all packages green) | PASS |
| gate-Failure PersistKey count 0, no writers ran | `go test ./internal/identity/... -run TestCreateGateFailure_PersistKeyCountZero -race` | PASS | PASS |
| confirmed PersistKey count 1 to FINAL path, no temp in previews | `go test ./internal/identity/... -run TestCreateConfirmed_PersistKeyCountOneAndFinalPaths -race` | PASS | PASS |
| PreWrite invoked with TempPrivatePath | `go test ./internal/identity/... -run TestCreateGate_UsesTempPath -race` | PASS | PASS |
| Reuse/AddAccount PersistKey count 0 | `go test ./internal/identity/... -run TestReuseNoPersistKey/TestAddAccountNoPersistKey -race` | PASS | PASS |
| Rotate PersistKey count 1 confirmed / 0 failure | `go test ./internal/identity/... -run TestRotatePersistKeyOnConfirm -race` | PASS | PASS |
| Full suite, all packages, race detector | `go test -race ./...` | All 12 packages ok | PASS |
| Lint (gosec, staticcheck, unused) | `make lint` | 0 issues | PASS |

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | No debt markers (TBD/FIXME/XXX), no `os.Rename`, no `os.ReadFile` of temp, no hardcoded empty returns found in modified files | — | — |

---

### Requirements Coverage

| Requirement | Source | Description | Status | Evidence |
|-------------|--------|-------------|--------|----------|
| BUG-4 | PLAN | Gate runs pre-write; orphan key impossible on abort/dry-run | SATISFIED | Temp staging + PersistKey-only-on-confirm; 4 hermetic tests |
| SAFE-03 | PLAN | `--dry-run` writes nothing | SATISFIED | `Confirmed=false` path skips PersistKey and all four writers; `TestCreateDryRun_PersistKeyCountZero` asserts zero writes |

---

### Human Verification Required

None. The user has already provided live binary corroboration: `--dry-run` and a gate-failure abort both left `~/.ssh` byte-for-byte unchanged with no orphan key, and the gate dialed a `/var/folders` temp key path — consistent with the `os.MkdirTemp("", "gitid-key-*")` staging in `buildDeps.Generate`.

---

### Gaps Summary

No gaps. All 6 must-have truths are verified by direct source reading and by the race-enabled test suite (12/12 packages pass, 0 lint issues).

---

_Verified: 2026-06-10_
_Verifier: Claude (gsd-verifier)_
