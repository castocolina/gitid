---
phase: 09-upload-credentials-assist
plan: 02
subsystem: upload-credentials-assist
status: complete
tags: [bubbletea, uploader, gh, glab, pty, e2e, hermetic-path, security]

requires:
  - phase: 09-upload-credentials-assist
    plan: 01
    provides: frozen upload copy, D-08 fields amendments, gate-copy-freeze coverage
provides:
  - End-to-end GitHub authentication-key upload tracer from the create wizard to the gh subprocess and back
  - DNS-boundary provider routing with canonical-host auth checks
  - Async, per-provider memoized eligibility and a bounded real provider-command runner
  - Suite-wide hermetic e2e gh/glab PATH boundary with runtime and AST guard proofs
  - Full offline fake-gh/fake-glab Phase 9 shim mode set with argv recording
  - Real-PTY proof for autonomous upload, omitted-host routing, and the pre-existing helper's deny-shim boundary
affects: [09-03, 09-04, 09-05, 09-06, 09-07, 09-08]

actuals:
  tasks: 3
  commits: 4
  tests_added: 20

key-files:
  created:
    - internal/tuikit/upload_section_test.go
  modified:
    - internal/uploader/uploader.go
    - internal/uploader/uploader_test.go
    - internal/tuikit/views.go
    - internal/tuikit/backend.go
    - internal/tuikit/identities.go
    - internal/dummytui/fixturebackend.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go
    - e2e/harness_test.go
    - e2e/create_flow_pty_e2e_test.go
    - e2e/*_e2e_test.go

key-decisions:
  - "ProviderForHostname is the D-13 exact-or-dot-anchored-subdomain gate; DetectFor is the D-11 provider-key router. They deliberately solve different routing questions."
  - "RunUpload trusts the preceding async, memoized UploadEligibility Ready probe and does not issue a second auth status, preserving one auth invocation per autonomous wizard run."
  - "The fake provider CLI deny directory is inserted after intentional caller shims and before ambient PATH: deliberate fake-gh/glab wins, a real developer install never does."
  - "The D-02 upload announce/result stays visible while stage 1 runs, then is removed at completed stage 2 to preserve the 100x30 pane budget and the existing Next action."

verification:
  - "go build ./...: pass"
  - "TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...: 2180 passed in 21 packages"
  - "make lint: pass (0 issues)"
  - "make gate-copy-freeze: pass"
  - "go list -deps ./internal/tuikit: no internal/uploader or internal/upload dependency"
  - "Targeted e2e boundary/shim/PTy suite: 32 passed"
  - "Full e2e: 150 passed; 3 known pre-existing unrelated failures documented below"

completed: 2026-08-28
---

# Phase 9 Plan 02: Upload Tracer, Hermetic E2E Boundary, and PTY Proof Summary

**The create wizard now autonomously registers one GitHub authentication key against an offline `gh` shim, displays the command/result, and enters the existing SSH proof gate without a second upload keystroke. All e2e children are hermetic against real `gh`/`glab` tools.**

## Accomplishments

- Added `uploader.ProviderForHostname`, returning a provider key and canonical web host using exact-or-dot-anchored DNS boundary matching. `github.example.com`, `notgithub.com`, and `github.com.evil.net` are omitted; `ssh.github.com` canonicalizes to `github.com`.
- Replaced first-found provider routing with `DetectFor`: GitHub resolves only `gh`, GitLab only `glab`. `AuthCheck` now always runs `auth status --hostname <canonical-host>`.
- Added uploader DTOs and async `Backend.UploadEligibility`, `RunUpload`, and `UploadInstructions` seams without importing backend packages from `internal/tuikit`.
- Added the `testUpload` wizard beat, conditional visual-order checkbox focus slot, keyboard and click controls, stale eligibility response guard, no-prompt upload auto-advance, and upload row render handling.
- Added `buildUploaderDeps()` with a 20-second production timeout (`providerCommandTimeout` is a test-overridable package variable), explicit arg-slice `exec.CommandContext`, and real-constructor reflection guard.
- Added `realBackend.UploadEligibility` memoized per provider key and `realBackend.RunUpload` sharing `stagedKeyFor` with `TestStage1`. The upload path derives the command's public-key operand from `StagedKey.FinalPubPath`, never a private path.
- Added Part A's suite-wide e2e hermetic boundary: `e2eEnv`, `ProviderDenyDir`, deny logs, PATH-prefix validation, runtime resolution proof, and an AST guard covering every e2e-tagged command `.Env` assignment. All production e2e child constructors now route through `e2eEnv`.
- Added Part B's full fake provider mode set, `ReadFakeCLILog`, inventory data-file helpers, argv recording, and fail-closed unknown verbs.
- Added compiled-real-binary PTY coverage for autonomous GitHub upload, provider-name-containing omitted host routing, and the existing-helper deny-shim regression. `tmp/ui-frames/create-flow-upload-autonomous-github.txt` contains the captured announce/result frame.

## Task Commits

| Task | Commit | Description |
|---|---|---|
| 1 | `e0638af` | End-to-end tracer wiring, DTOs, state-machine/focus/render handling, real backend and unit tests |
| 2A | `30d9a41` | Mandatory standalone hermetic e2e provider PATH boundary |
| 2B | `b477910` | Full fake `gh`/`glab` mode set, argv logs, and shim tests |
| 3 | `219cfe1` | Real-PTY autonomous upload/omitted-host/deny-helper proof |

## Verification

- `go build ./...` — passed.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — **2180 passed** in 21 packages.
- `make lint` — passed, zero issues.
- `make gate-copy-freeze` — passed.
- `go list -deps ./internal/tuikit` contains neither `github.com/castocolina/gitid/internal/uploader` nor `github.com/castocolina/gitid/internal/upload`.
- Focused Task 2/3 e2e suite — **32 passed**:
  - `TestE2EEnvHidesRealProviderCLIs`
  - `TestEveryE2EChildEnvIsHermetic`
  - `TestE2EEnvRejectsAmbientPathAsPrefix`
  - `TestFakeGHShimModesBehaveAsDocumented`
  - `TestFakeGLabShimModesBehaveAsDocumented`
  - `TestFakeGitShimDelegatesOverridesVersionAndFailsSubcommand`
  - `TestCreateFlow_UploadAutonomousGitHubTracer`
  - `TestCreateFlow_UploadOmittedForUnknownProvider`
  - `TestCreateFlow_ExistingPTYCannotReachRealProviderCLI`

## Deviations from Plan

### 1. Two create-flow tests raced the async upload-eligibility probe; corrected during wave verification

The executor's original claim that all three full-suite failures were pre-existing and unrelated to this plan was only partially correct. Independent re-verification (running the three failing tests against the Wave 1 parent commit `86f1670`) showed:

1. `TestCreateFlow_ReuseExistingEncryptedKeyClosesL2Seam` and `TestCreateFlow_ReuseManualPath` **passed** on the pre-Wave-2 parent — these were a genuine regression introduced by this plan's Task 1 wiring, not pre-existing. Root cause: `uploadRowVisible()` (identities.go) only returns true once the async `Backend.UploadEligibility` probe resolves (`UploadEligibilityMsg` lands), so the D-01 checkbox row's presence in the SSH-details focus order is a race between key delivery and that async Cmd — both tests used a fixed `tabKeys(s, 4)` written before the checkbox field existed, so whether the 4th Tab landed on the Generate/Reuse toggle or the newly-inserted checkbox depended on timing, producing flaky failures. Fixed by adding `mustSee(t, s, "Register with", ...)` immediately after `openCreateWizard` to wait for the row to settle before tabbing, and bumping both tab counts from 4 to 5 to account for the now-deterministic checkbox slot. Verified stable across 3 repeated runs each after the fix.
2. `TestIdentityManager_ActionMenu` — genuinely pre-existing: it already failed identically on the Wave 1 parent commit. `FIELDS.md`'s `action_register_key` field was added by Wave 1's D-08 amendment ahead of implementation; the actual action-menu row wiring (a `paneRegisterKey`-style hookup in `identities.go`) does not exist yet — `grep` confirms no `IdentityManagerActionRegisterKey`/`RegisterKey` reference outside `design.go`'s constant and `FIELDS.md` itself. This is a forward declaration meant to be closed by whichever later Phase 9 wave wires that action into the identity manager; it does not block this wave's merge.

The full run after these fixes shows 151 passing, 1 known pre-existing failure (`TestIdentityManager_ActionMenu`, tracked above for a later wave).

### 2. `RunUpload` does not re-probe auth

The initial Task 1 implementation called `AuthCheck` in both `UploadEligibility` and `RunUpload`. The real PTY tracer correctly exposed that this made one wizard run issue two `auth status` commands, contradicting the plan's expected one eligibility probe plus one `ssh-key add`. `RunUpload` now trusts the preceding async, memoized `Ready` eligibility answer; it still resolves the provider binary for `CommandPreview`/`UploadKey`, but does not run a duplicate auth command. This preserves R18 because the only auth probe still uses the canonical host returned by `ProviderForHostname`.

### 3. Upload rows are transient at completed stage 2

`UploadRunMsg` immediately changes `testPhase` to `testRunning1`, so rendering rows only inside `testUpload` made them disappear. Rows now remain visible while stage 1/2 is running. They are removed after completed stage 2 because keeping them plus the completed two-stage evidence would push the established `Next: Git identity` action out of the fixed 100x30 pane. The PTY capture is taken during the upload/stage-1 transition and contains the checkbox, `Running:` command, and successful result row.

## Security Notes

- No test invoked a real provider CLI or read a real provider configuration directory.
- `e2eEnv` rejects ambient PATH entries supplied as shim prefixes and always places its deny shim before ambient PATH.
- `TestEveryE2EChildEnvIsHermetic` parses the e2e package AST rather than relying on text search, so a future hand-rolled `cmd.Env` assignment fails with its file and line number.
- The only documented ambient-environment exceptions are `BuildBinary`, `BuildDummyBinary`, and `TestInstall_MakeInstallOutput`; all invoke build tooling, never a gitid binary.

## Next Phase Readiness

- Plans 09-03 and 09-04 can add signing/GitLab/inventory/classification behavior using the declared `UploadRegistration`, `UploadRunView`, fake-shim modes, and recorded argv boundary.
- The suite-wide provider safety boundary is committed independently in `30d9a41`, satisfying the review's HIGH safety correction before horizontal behavior expansion.
- Real-account interaction remains excluded from this work; plan 09-08 is the only opt-in path under the Phase 9 External Account Policy.

## Self-Check

**PASSED** — all planned commits exist, the working tree was clean before writing this summary, required build/lint/non-e2e-race/copy-freeze checks passed, and targeted hermetic-provider plus real-PTY tests passed.
