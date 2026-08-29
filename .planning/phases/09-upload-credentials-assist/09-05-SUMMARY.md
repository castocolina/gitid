---
phase: 09-upload-credentials-assist
plan: 05
subsystem: upload-credentials-assist
tags: [cli, cobra, github-cli, gitlab-cli, upload, headless]
requires:
  - phase: 09-upload-credentials-assist
    plan: 04
    provides: the complete TUI upload section (checkbox states, full testUpload beat, D-17 confirmation)
provides:
  - The one upload orchestration (upload_run.go) both the TUI and CLI call, split into planUpload/executeUpload so each surface composes the same decision logic differently
  - The one CLI outcome printer (printUploadOutcome), using only frozen tuikit copy
  - gitid identity register-key / gitid register-key — the manual re-run CLI surface, with its own exit contract (R10)
  - --no-upload on create, clone, rotate, and new-key, plus derived autonomous upload by default on all four
  - Headless CLI upload e2e coverage against the fake gh/glab shims
affects: [09-06, 09-07, 09-08]
actuals:
  tasks: 3
  commits: 1
tech-stack:
  added: []
  patterns: [plan-then-execute orchestration split for two different call sequencings, exit contract computed from result-row Command population rather than a side channel]
key-files:
  created:
    - cmd/gitid/upload_run.go
    - cmd/gitid/upload_run_test.go
    - cmd/gitid/identity_upload.go
    - cmd/gitid/identity_upload_test.go
  modified:
    - cmd/gitid/wiring.go
    - cmd/gitid/identity.go
    - cmd/gitid/identity_create.go
    - cmd/gitid/identity_clone.go
    - cmd/gitid/identity_key.go
    - cmd/gitid/main_test.go
    - cmd/gitid/identity_test.go
    - internal/tuikit/views.go
    - docs/cli-parity-matrix.md
    - e2e/identity_cli_e2e_test.go
key-decisions:
  - "All three tasks landed as ONE commit rather than three: identity_upload.go/identity_upload_test.go mix Task 2's register-key verb with Task 3's --no-upload constant and step, and several pre-existing tests needed both the new register-key parity row and NoUpload safety fixes in the same edit. CLAUDE.md's own rule (buildable boundary sets commit granularity, not task count) made a clean 3-way split impossible without breaking compilation at an intermediate commit."
  - "register-key's exit contract computes 'attempted' from whether a result row's Command field is non-empty (set only when requirePublicKey passed and buildArgs ran) rather than a side-channel counter — the same signal is already there, so no new state was needed."
  - "The dry-run upload preview for rotate/new-key previews the CURRENT key (no new key exists yet in a dry run); for create/clone it previews the freshly-staged key via a temp .pub sibling, mirroring the wizard's own TempPrivatePath pattern, written and read before the ceremony's own staging cleanup runs."
patterns-established:
  - "planUpload/executeUpload: a two-phase split lets two callers compose identical decision logic with different sequencing (async two-message for the TUI's R7 announce gap, synchronous for the CLI) without a second implementation existing anywhere."
requirements-completed: [UP-01, UP-02, UP-03]
coverage:
  - id: D1
    description: One upload orchestration (planUpload/executeUpload/runUploadFor) and one CLI outcome printer (printUploadOutcome), structurally enforced
    requirement: UP-01
    verification:
      - kind: unit
        ref: cmd/gitid/upload_run_test.go
        status: pass
    human_judgment: false
  - id: D2
    description: register-key manual re-run verb with its own R10 exit contract
    requirement: UP-02
    verification:
      - kind: unit
        ref: cmd/gitid/identity_upload_test.go#TestRegisterKeyExitContract
        status: pass
      - kind: e2e
        ref: e2e/identity_cli_e2e_test.go#TestIdentityCLI_RegisterKeyPartialScopeReportsBothTypes
        status: pass
    human_judgment: false
  - id: D3
    description: --no-upload and derived autonomous upload on create/clone/rotate/new-key, upload never changing exit code, headless e2e proof at the process level
    requirement: UP-03
    verification:
      - kind: unit
        ref: cmd/gitid/identity_upload_test.go#TestNoUploadFlagIsBoundOnAllFourWriteVerbs
        status: pass
      - kind: e2e
        ref: e2e/identity_cli_e2e_test.go#TestIdentityCLI_CreateSucceedsWhenUploadFails
        status: pass
    human_judgment: false
completed: 2026-08-29
status: complete
---

# Phase 09 Plan 05: CLI Upload Surface Summary

**Upload now has full CLI parity: one shared orchestration the TUI and CLI both call, a scriptable register-key manual re-run verb with its own exit contract, and derived autonomous upload by default on every write verb that produces new key material — none of which can ever change that verb's own exit code.**

## Accomplishments

- Extracted `planUpload`/`executeUpload` (`cmd/gitid/upload_run.go`) from `realBackend.RunUpload`'s closure: the CLI's `runUploadFor` is plan-then-execute with no gap; the TUI composes the SAME two functions with the R7 announce-before-run gap by returning `UploadStartedMsg` first and calling `executeUpload` from its `FollowUp`.
- Added `printUploadOutcome`, the one CLI text printer, using exclusively `internal/tuikit`'s frozen `Upload*` copy constants — `UploadRunView` gained a `ProviderName` field so the printer needs no second data source.
- Added `gitid identity register-key <name>` / `gitid register-key <name>` (Task 2), built from one `identityVerb` spec, with its own R10 exit contract: non-zero only when at least one registration was attempted and every attempted registration failed — everything else (full success, partial success, already-complete, ineligible provider, dry run) exits 0.
- Added `--no-upload` (Task 3) identically on create, clone, rotate, and new-key via one shared help-text constant, plus derived autonomous upload by default on all four — the upload step runs after the key material exists and never alters the calling function's control flow (it has no return value).
- Added 5 headless CLI e2e tests against the fake gh shims, all hermetic via `e2eEnv` (R1).

## Deviations from Plan

### 1. One commit instead of three

The plan's own Task 1/2/3 boundaries didn't survive contact with the actual files: `identity_upload.go`/`identity_upload_test.go` ended up holding BOTH Task 2's register-key verb and Task 3's `--no-upload` constant and step (they're genuinely the same file's natural home), and several pre-existing tests needed edits driven by both tasks at once. A real attempt to split the commit failed at the pre-commit hook (an intermediate commit didn't compile). Landed as one commit; the commit message documents the intended task boundaries internally.

### 2. Five pre-existing tests needed a NoUpload safety fix — a real finding

Independent verification (not part of any single task's own scope) found that making derived upload unconditional broke five pre-existing tests that never anticipated a code path touching `uploaderDeps`: `TestIdentityCreateDryRunPrintsGateOutcomesAndPreviews`, `TestIdentityCloneDryRunPrintsGateOutcomesAndPreviews`, `TestIdentityKeyVerbDryRunNoYesWritesNothing`, `TestIdentityRotateAndRepairRecordingDoubleInvokedExactlyOnce`, `TestIdentityCloneRecordingDoubleInvokedExactlyOnce`. Without a fake `uploaderDeps` installed, the new upload step reached the REAL dependencies and shelled out to this machine's actually-installed `gh`, writing `~/.local/state/gh/device-id` inside the test's sandboxed `$HOME`. This is the same class of hazard R1 (hermetic e2e PATH) addressed for e2e tests, but for CLI unit tests, which have no equivalent hermetic-by-construction guarantee — a unit test that exercises a real code path touching `uploaderDeps` must stub it explicitly. Fixed by adding `NoUpload: true` to each (they test ceremony/lifecycle behavior, not upload).

### 3. The R3 structural test's exact mechanism changed

Plan 09-04's `TestRunUploadDoesNotCallProviderCommandsOutsideATeaCmd` scanned only `wiring.go` for the three guarded identifiers nested inside a `FuncLit`. After the extraction, the decision logic lives in `upload_run.go` as ordinary methods (not literals), so the literal-nesting check no longer matches reality. Rewritten to assert the real invariant: `wiring.go` contains zero occurrences of the guarded calls (positive control: `upload_run.go` does contain them). `AuthCheck` is no longer part of this specific guard — `UploadEligibility`'s own call is unchanged and unrelated to this extraction.

## Security Notes

- `uploadRequestForAccount` expands the account's tilde-form `PubPath` before any file read — the same pattern `identity_clone.go` already established for exactly this reason.
- No new module dependency; `internal/uploader` remains stdlib-only.
- Every new e2e case routes through `e2eEnv`; `TestEveryE2EChildEnvIsHermetic` covers the whole `e2e` package after this plan's commit.

## Next Phase Readiness

- Plan 09-06 (Identity Manager) can drive the same `runUploadFor`/`printUploadOutcome` surface this plan built.
- Plan 09-07 (divergence classification) has a concrete CLI-vs-TUI behavior set to classify.
- Plan 09-08 (the one real-account wave) can rely on `register-key`'s exit contract and `--no-upload`'s safety already being proven against fakes.

---
*Phase: 09-upload-credentials-assist*
*Completed: 2026-08-29*
