---
phase: 03-create-flow-backend
plan: 11
status: complete
tasks_completed: 2
tasks_total: 3
commits: 2
duration_minutes: 90
completed_date: 2026-08-21
subsystem: create-flow-backend
tags: [tdd, production-proof, checked-render, fingerprint, rollback, packet-publisher]
requirements: [SSHUI-01, SSHUI-02, SSHUI-03, SSHUI-04, SSHUI-05, TEST-01, TEST-02, TEST-03, KEY-06, DLV-04, DLV-06]

key_decisions:
  - "RenderCheckedHostBlock added as the authoritative final render boundary including unicode.IsSpace — all production callers (HostBlockPreview, StageTestConfig, commitCreateTransaction) route through it"
  - "specFingerprint includes ReuseKeyPath (normalized absolute) so generate vs reuse and different reuse paths produce different fingerprints (CR-07)"
  - "TestStage2 validates all ssh -G resolution fields before recordOutcomeFor — a wrong user/hostname/port/identitiesonly/identityfile cannot unlock persistence (CR-05)"
  - "TestResultView gains ResolutionCommand + ResolutionOutput fields for complete CR-06 proof rendering"
  - "Directory rollback iterates createdDirs in reverse order (child before parent) fixing the fresh-machine SSH dir removal race (CR-09)"
  - "HostBlockPreview uses spec.Provider directly (WR-01) eliminating providerFromAlias truncation for multi-label providers"
  - "gitid-evidence publisher validates source commit format then existence then destination before any temp work — fail-closed ordering produces precise error messages"
  - "Task 3 (publish + independent reviews) deferred — requires committed clean source SHA and external reviewer authentication"

actuals:
  tokens: 98000
  tasks: 2
  commits: 2
---

# Phase 03 Plan 11: Convergent Phase 3 Gap Closure — Tasks 1 and 2 Summary

**One-liner:** Production stage-2 proof validation-before-record, unicode-safe checked render boundary, canonical fingerprint with reuse path, reverse-order rollback, spec.Provider preview, and fail-closed immutable packet publisher infrastructure.

## Tasks Executed

### Task 1: End-to-end production stage-two proof, fingerprint, checked render, and rollback tracer

**Commit:** `185bf31`

#### RED Test Phase — Recorded Failing Outputs

```
# CR-08: RenderCheckedHostBlock undefined
$ go test -count=1 ./internal/sshconfig/... -run 'TestRenderCheckedHostBlock'
FAIL  github.com/castocolina/gitid/internal/sshconfig [build failed]
internal/sshconfig/renderer_test.go:22:12: undefined: RenderCheckedHostBlock

# CR-06: TestResultView missing ResolutionCommand/ResolutionOutput
$ go test -count=1 ./cmd/gitid/ -run 'TestTestResultViewHasResolutionFields'
FAIL  github.com/castocolina/gitid/cmd/gitid [build failed]
cmd/gitid/wiring_cr_test.go:760:3: unknown field ResolutionCommand in struct literal

# CR-05: stage records before validation (would have detected at runtime)
# CR-07: specFingerprint omits reuse path (would have detected at runtime)
# CR-09: forward dir removal order (would have detected at runtime)
# WR-01: providerFromAlias truncation (would have detected at runtime)
```

#### GREEN Test Phase — All Pass

```
$ go test -race -count=1 ./internal/tester/... ./internal/sshconfig/... ./internal/tuikit/... ./cmd/gitid/
ok  github.com/castocolina/gitid/internal/tester     1.471s
ok  github.com/castocolina/gitid/internal/sshconfig  5.102s
ok  github.com/castocolina/gitid/internal/tuikit     15.048s
ok  github.com/castocolina/gitid/cmd/gitid            7.456s
```

#### New Tests Added (Task 1)

| Test | Finding | Status |
|------|---------|--------|
| TestRenderCheckedHostBlockRejectsUnicodeSpace | CR-08 unicode gate | PASS |
| TestRenderCheckedHostBlockRejectsAllUnicodeIsSpaceChars | CR-08 | PASS |
| TestRenderCheckedHostBlockRejectsUnicodeControlChars | CR-08 | PASS |
| TestRenderCheckedHostBlockPassesSafeIdentityFile | CR-08 positive | PASS |
| TestTestResultViewHasResolutionFields | CR-05/CR-06 | PASS |
| TestStage2RecordsOutcomeAfterValidation | CR-05 | PASS |
| TestSpecFingerprintIncludesKeySource | CR-07 | PASS |
| TestSpecFingerprintIncludesNormalizedReusePath | CR-07 | PASS |
| TestCheckedRendererUsedAtPreviewBoundary | CR-08 positive | PASS |
| TestHostBlockPreviewUsesSpecProvider | WR-01 | PASS |
| TestRollbackCreatedDirsInReverseOrder | CR-09 | PASS |

#### Production Fixes (Task 1)

**CR-08 — `RenderCheckedHostBlock` in `internal/sshconfig/renderer.go`:**
- Added `validateIdentityFileStrict` which calls `unicode.IsSpace` and `unicode.IsControl` in addition to the ASCII checks in `ValidateHostBlock`
- Added `RenderCheckedHostBlock(alias, hostname string, port int, identityFile, provider string) (string, error)` as the authoritative final boundary
- All three production callers routed: `HostBlockPreview`, `StageTestConfig`, `commitCreateTransaction`

**WR-01 — `HostBlockPreview` uses `spec.Provider`:**
- Changed from `providerFromAlias(spec.Alias)` to `spec.Provider` — preserves multi-label providers like `company.co.uk` that `providerFromAlias` would truncate to `co.uk`

**CR-05/CR-06 — `TestStage2` validate-before-record:**
- Added `ResolutionCommand` and `ResolutionOutput` to `tuikit.TestResultView`
- `TestStage2` attaches `tester.ResolvedViaGCommand(configPath, in.Alias)` and `formatResolvedConfig(resolved)` to the view
- Calls `tester.ValidateResolvedConfig(resolved, expected)` before `recordOutcomeFor` — a wrong user/hostname/port/identitiesonly/first-identityfile downgrades to `TestOutcomeFailure` without recording

**CR-07 — `specFingerprint` includes reuse path:**
- Added `ReuseKeyPath string` field to `identity.CreateInput`
- `createInput` populates it from `DemoIdentity.ReuseKeyPath` via `resolveKeyPath` (normalizes `~/...` to absolute)
- `specFingerprint` format updated: adds `|%s` for `in.ReuseKeyPath` as seventh field

**CR-09 — Reverse directory rollback:**
- Changed `for _, d := range createdDirs` to `for i := len(createdDirs) - 1; i >= 0; i--` so children are removed before parents (config.d before .ssh)

### Task 2: Fail-closed approval-commit 24-panel generation and immutable publication

**Commit:** `ed6d2d0`

#### RED Test Phase — Recorded Failing Outputs

```
# CR-01: publisher was a no-op
$ make generate-visual-review-packet SOURCE_COMMIT=a2b646... OUTPUT_DIR=/tmp/p
Publication target: Task 3 generates the review packet via this Make entry point.
created=no members=0

# Tests compile-fail before fixes:
# TestGenerateTextPacket_* — no GenerateTextPacket function
# TestPublisherRejectsEmptySourceCommit — no cmd/gitid-evidence package
```

#### GREEN Test Phase — All Pass

```
$ go test -tags screenshot -race -count=1 ./internal/screenshot/... -run 'TestGenerateTextPacket|TestValidatePacket|TestCompareManifests|TestCompareTextCaptures|TestPacketApprovalCommit'
ok  github.com/castocolina/gitid/internal/screenshot  7.357s

$ go test -tags screenshot -race -count=1 ./cmd/gitid-evidence/...
ok  github.com/castocolina/gitid/cmd/gitid-evidence   1.579s
```

#### New Files (Task 2)

**`internal/screenshot/createflow_packet.go`:**
- `PacketMember`, `Packet`, `PacketOptions`, `PacketResult` types (canonical JSON manifest)
- `GenerateTextPacket`: creates packet in new directory, writes live + approved-tui text, canonical MANIFEST.json with SHA-256 of every member and self-referential manifest hash
- `ValidatePacket`: reads MANIFEST.json, verifies every declared member hash, rejects missing members
- `CompareManifests`: byte-identical manifest comparison (excluding wall-clock and self-hash fields)
- `CompareTextCaptures`: byte-identical per-screen text comparison for determinism
- `PacketApprovalCommit = "3c3130e404329cf42baafdf63a6c22758437edc6"`
- `ValidatePanelCount = 24` (8 live + 8 approved-TUI + 8 approved-HTML)

**`cmd/gitid-evidence/main.go`:**
- `--source-commit <40-hex>` and `--output-root <dir>` flags
- `validateSourceCommitFormat`: 40-hex check
- `validateSourceCommitExists`: `git cat-file -e <commit>^{commit}`
- Destination check BEFORE git verification (precise error messages)
- Two-candidate generation in temp dirs → `CompareManifests` → atomic `os.Rename` → `ValidatePacket`
- On any failure: removes temp candidates, returns nonzero

**`Makefile` — real publisher:**
```makefile
generate-visual-review-packet:
    @if [ -z "$(SOURCE_COMMIT)" ]; then echo "ERROR: SOURCE_COMMIT=<full-sha> required"; exit 1; fi
    @if [ -z "$(OUTPUT_DIR)" ]; then echo "ERROR: OUTPUT_DIR=<new-empty-dir> required"; exit 1; fi
    go run -tags screenshot ./cmd/gitid-evidence \
        --source-commit "$(SOURCE_COMMIT)" \
        --output-root   "$(OUTPUT_DIR)"
```

**Verified fail-closed:**
```
$ make generate-visual-review-packet SOURCE_COMMIT=0000000000000000000000000000000000000000 OUTPUT_DIR=/tmp/test
gitid-evidence: source commit "0000000000000000000000000000000000000000" not found in local repo
exit status 1
make: *** [generate-visual-review-packet] Error 1
```

## Baseline Safety Hashes (unchanged throughout)

| Artifact | SHA-256 |
|----------|---------|
| `03-09-review-packet/EVIDENCE.json` | `b0801041eb3ce30f1556289a630cf8f9e06f51f4d651891458e6e007f8950ea8` |
| `03-09-review-packet/panel-pngs/reuse-key-vs-generate.png` | `378eeaa4db94a3734bcf147b6f89477afc31593a5f84a0ccb958ada7c515f0a4` |

Untracked `.planning/phases/05.7-*/` directory untouched throughout.
Real `~/.ssh`, `~/.gitconfig*`, external accounts untouched (verified: only `t.TempDir()` used).

## Final Gate Results

```
make test          — PASS (19 packages, 0 failures)
make lint          — PASS (0 issues)
make gate-copy-freeze   — PASS (11/11 frozen strings verified)
make gate-visual-regression — PASS (8 screens, all differences allowlisted)
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestHostBlockPreviewIsTheWrittenBlock needed Provider field**
- **Found during:** Task 1 GREEN verification
- **Issue:** Existing test created `CreateSpec` without `Provider` field; after WR-01 fix HostBlockPreview uses `spec.Provider` directly instead of `providerFromAlias`, so the test's `want` (which passed `"github.com"` as provider) didn't match the actual preview (empty provider → no marker)
- **Fix:** Added `Provider: "github.com"` to the test's `spec` — the test now documents the correct WR-01 contract
- **Files modified:** `cmd/gitid/wiring_test.go`
- **Commit:** `185bf31`

**2. [Rule 1 - Bug] wiring_cr_test.go: unused-parameter lint error**
- **Found during:** Task 1 lint
- **Issue:** `revive: parameter 'keyPath' seems to be unused` in the injected `ResolvedVia` closure
- **Fix:** Renamed to `_` per Go convention
- **Files modified:** `cmd/gitid/wiring_cr_test.go`
- **Commit:** `185bf31`

**3. [Rule 3 - Deviation] StripANSIExported already declared in createflow_regions.go**
- **Found during:** Task 2 build
- **Issue:** Initial `createflow_packet.go` draft included a duplicate implementation
- **Fix:** Removed the duplicate — existing `StripANSIExported` in `createflow_regions.go` already exports the needed function
- **Files modified:** `internal/screenshot/createflow_packet.go`
- **Commit:** `ed6d2d0`

### Scope Limits Applied (per plan)

**Task 3 (publish + two independent reviews) not executed** per user instruction: _"Do NOT execute Task 3's independent reviews yet."_ Task 3 requires a committed clean source SHA plus external reviewer authentication (Claude UI/UX reviewer + Codex CLI), which are out of scope for this execution.

**PNG capture (approval commit)** not implemented: `freeze` and `chromium` are absent from this environment. The publisher is correctly fail-closed when these tools are absent. The `TestCaptureTUI` failure in `internal/screenshot` is pre-existing (freeze binary not installed) and was verified unchanged before and after this plan.

## Remaining Blockers

1. **Task 3 not executed** — requires user-initiated session for independent review authentication
2. **PNG capture** — `freeze` and `chromium` not installed in this environment; publisher will correctly fail when invoked without these tools
3. **Phase 3 review** — CR-01 through CR-10 and WR-01 production fixes are implemented; the 03-REVIEW.md remains as the blocking review record until Task 3 produces an immutable packet with two fresh authenticated reviews

## Post-Task-2 Raw-PTY Regression Repair

The strict Task-1 stage-two validator correctly compares the first effective
`ssh -G` `IdentityFile` with the generated temporary staged key. The D-22
`FakeSSHDir` test executable still emitted a fixed unrelated IdentityFile, so
it falsely failed every raw-PTY flow that reached stage two.

`FakeSSHDir` now requires the supplied readable `-F` staged config and emits
its `User`, `Hostname`, `Port`, `IdentitiesOnly`, and first `IdentityFile`.
It fails closed when any required managed Host field is absent. The new
`TestFakeSSHDirResolvesIdentityFileFromStagedConfig` regression test was RED
with the old fixed path and is GREEN with the config-derived fixture.

Verification after the repair:

```
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -race -count=1 ./e2e -run '^TestFakeSSHDirResolvesIdentityFileFromStagedConfig$' — PASS
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -race -count=1 -timeout 180s ./e2e -run '^(TestCreateFlow_TestStagePass|TestCreateFlow_TestStageReachableNotUploaded|TestCreateFlow_GitStepDisabledReasonAndConfirmWrite|TestCreateFlow_ReuseExistingEncryptedKeyClosesL2Seam)$' — PASS (4/4)
TERM=dumb SSH_AUTH_SOCK= make test-e2e — PASS (58.884s)
TERM=dumb SSH_AUTH_SOCK= make test — PASS
make lint — PASS (0 issues)
```

The pre-existing dirty 03-09 evidence remained byte-identical:

- `EVIDENCE.json`: `b0801041eb3ce30f1556289a630cf8f9e06f51f4d651891458e6e007f8950ea8`
- `reuse-key-vs-generate.png`: `378eeaa4db94a3734bcf147b6f89477afc31593a5f84a0ccb958ada7c515f0a4`

## Known Stubs

None — all implemented behavior is production wiring, not placeholder values.

## Threat Flags

None new — all changes stay within the established trust boundaries documented in the 03-11-PLAN.md threat model.

## Self-Check

### Created Files Exist

- [x] `internal/sshconfig/renderer.go` — `RenderCheckedHostBlock` present
- [x] `internal/tuikit/views.go` — `ResolutionCommand`, `ResolutionOutput` fields present
- [x] `internal/screenshot/createflow_packet.go` — canonical packet infrastructure
- [x] `cmd/gitid-evidence/main.go` — fail-closed publisher
- [x] `cmd/gitid-evidence/main_test.go` — publisher tests
- [x] `.planning/phases/03-create-flow-backend/03-11-SUMMARY.md` — this file

### Commits Exist

```
185bf31 feat(03-11): Task 1 — production stage-2 proof, fingerprint, checked render, rollback
ed6d2d0 feat(03-11): Task 2 — fail-closed packet publisher and canonical 24-panel infrastructure
```

## Self-Check: PASSED
