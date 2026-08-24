---
status: awaiting_human_verify
trigger: "Diagnose and fix the Phase 3 raw-PTY E2E regression introduced after 03-11 Tasks 1-2. make test-e2e fails TestCreateFlow_TestStagePass, TestCreateFlow_TestStageReachableNotUploaded, TestCreateFlow_GitStepDisabledReasonAndConfirmWrite, and TestCreateFlow_ReuseExistingEncryptedKeyClosesL2Seam."
created: 2026-08-21T23:15:39Z
updated: 2026-08-21T23:31:00Z
---

## Current Focus
<!-- OVERWRITE on each update - reflects NOW -->

bug_class: Bohrbug (four repeatable raw-PTY cases fail after one commit series)
hypothesis: Confirmed — Task 1 correctly made production TestStage2 require the staged temporary `IdentityFile`, but FakeSSHDir falsely returns a fixed `/tmp/fake/.ssh/id_ed25519_testid` for every `ssh -F <staged-config> -G <alias>` request. The strict validator rejects that mismatch, causing all paths which require stage two to fail.
test: The fix is committed and all automated gates passed. Await confirmation that the original Phase 3 raw-PTY E2E workflow is resolved in the user's environment.
expecting: The user confirms `make test-e2e` is green without a real HOME/config or external account mutation.
next_action: Await "confirmed fixed" or a remaining failure report; do not archive the session until confirmation.

reasoning_checkpoint:
  hypothesis: "FakeSSHDir causes realBackend.TestStage2 to fail because it emits a fixed IdentityFile rather than the first IdentityFile in the staged -F config that production validates."
  confirming_evidence:
    - "The isolated raw-PTY TestCreateFlow_TestStagePass reaches stage 1 then enters the hard-failure state before any stage-2 proof is displayed."
    - "TestStage2 compares ssh -G's first IdentityFile with staged.TempPrivatePath; FakeSSHDir instead emits /tmp/fake/.ssh/id_ed25519_testid for every -G call."
    - "TestFakeSSHDirResolvesIdentityFileFromStagedConfig fails before the patch with the fixed fake path."
  falsification_test: "If fake ssh -F <config> -G <alias> emits the config's exact required fields and TestStagePass still fails, this hypothesis is false."
  fix_rationale: "Parsing the test-owned staged config in FakeSSHDir makes the external fake emulate the effective configuration that real ssh -G would report, preserving rather than bypassing production's strict five-field validation."
  blind_spots: "The fake's minimal parser models only the managed Host-block syntax emitted by RenderCheckedHostBlock; full OpenSSH Include/Match semantics remain owned by real sshconfig tests."
  candidate_causes:
    - "code: Task 1's new correct first-IdentityFile production validation exposed a prior inaccurate fixture."
    - "environment: the e2e fake executable on PATH does not behave like real ssh -G for the supplied -F config."
  and_gate: "yes — the regression requires both the new strict production validation and the stale fixed-path fake-SSH output; either condition alone did not fail these tests."

## Symptoms
<!-- Written during gathering, then IMMUTABLE -->

expected: Phase 3 raw-PTY E2E create-flow cases pass while preserving strict CR-05/06 real stage-two validation of effective ssh -G fields and rendering both commands and outputs through the fake-SSH harness.
actual: make test-e2e fails TestCreateFlow_TestStagePass, TestCreateFlow_TestStageReachableNotUploaded, TestCreateFlow_GitStepDisabledReasonAndConfirmWrite, and TestCreateFlow_ReuseExistingEncryptedKeyClosesL2Seam after 03-11 Tasks 1-2.
errors: Full output at /Users/ramon/Library/Application Support/rtk/tee/1787354045_make_test-e2e.log.
reproduction: Run make test-e2e; isolate each named TestCreateFlow case with the repository's normal E2E command and environment.
started: After 03-11 Tasks 1-2.

## Eliminated
<!-- APPEND only - prevents re-investigating -->

## Evidence
<!-- APPEND only - facts discovered -->

- timestamp: 2026-08-21T23:15:39Z
  checked: User report, project state, and Phase 3 roadmap context
  found: Four deterministic raw-PTY E2E cases regressed immediately after 03-11 Tasks 1-2; the project explicitly requires fake-SSH harness use and non-bypassable stage-two ssh -G proof validation.
  implication: Treat this as a Bohrbug and trace the production create-flow transition rather than weakening assertions or substituting a fake result.
- timestamp: 2026-08-21T23:17:30Z
  checked: Full make test-e2e output and 03-11 Task 1 summary/plan
  found: All four cases reach stage 1 but render the hard-failure retry state before the expected stage-2 proof. Task 1 made TestStage2 validate effective User, Hostname, Port, IdentitiesOnly, and first IdentityFile before recording an accepted outcome.
  implication: The regression is in the new strict production stage-two proof path or its fake-SSH test fixture compatibility, not in the Git step or key reuse persistence paths.
- timestamp: 2026-08-21T23:19:30Z
  checked: Isolated `TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -race -count=1 -timeout 180s ./e2e -run '^TestCreateFlow_TestStagePass$'`, FakeSSHDir, tester.ResolvedVia, and realBackend.TestStage2
  found: The isolated test deterministically fails after stage 1. Production stages a real temporary private key, writes it into a throwaway `-F` config, and validates the first `ssh -G` IdentityFile against that temporary path. FakeSSHDir recognizes `-G` but emits the unrelated fixed `/tmp/fake/.ssh/id_ed25519_testid` path.
  implication: The production strict proof is functioning as intended; the D-22 fake-SSH harness is now semantically stale and must resolve the staged config rather than fabricate an identity path.
- timestamp: 2026-08-21T23:21:30Z
  checked: RED command `TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -race -count=1 ./e2e -run '^TestFakeSSHDirResolvesIdentityFileFromStagedConfig$'`
  found: The test failed because `ssh -F <config> -G <alias>` returned `identityfile /tmp/fake/.ssh/id_ed25519_testid` instead of the unique IdentityFile written to the test config.
  implication: Direct, isolated reproduction confirms the fake-SSH fixture violates the production stage-two effective-config contract.
- timestamp: 2026-08-21T23:23:30Z
  checked: Focused harness GREEN attempt and the four named raw-PTY tests
  found: The new fake script has an unclosed resolution `if`, producing `syntax error: unexpected end of file`; the focused config also intentionally supplied only IdentityFile while the new fail-closed fake requires all five effective fields.
  implication: This is an implementation defect in the unverified patch, not contrary evidence against the confirmed root cause; correct the script and complete the fixture before retesting.
- timestamp: 2026-08-21T23:25:00Z
  checked: Focused harness and named raw-PTY regression commands after correcting the script
  found: `TestFakeSSHDirResolvesIdentityFileFromStagedConfig` passed, and all four reported `TestCreateFlow_*` cases passed under `-tags e2e -race -count=1`.
  implication: The fake now supplies the same effective fields the production validator checks, restoring the real stage-two flow without weakening CR-05/CR-06.
- timestamp: 2026-08-21T23:27:00Z
  checked: `make test-e2e`, `make test`, and `make lint`
  found: Complete raw-PTY E2E suite passed in 58.884s; the full race/coverage suite passed; golangci-lint reported 0 issues.
  implication: The targeted D-22 fixture correction has no observed regression across the project gates.
- timestamp: 2026-08-21T23:29:00Z
  checked: Intended diff, worktree status, and SHA-256 of protected dirty 03-09 evidence
  found: Only e2e/harness_test.go is an intended source change. 03-09 EVIDENCE.json remains b0801041eb3ce30f1556289a630cf8f9e06f51f4d651891458e6e007f8950ea8 and reuse-key-vs-generate.png remains 378eeaa4db94a3734bcf147b6f89477afc31593a5f84a0ccb958ada7c515f0a4.
  implication: The repair has preserved the user-protected dirty evidence and unrelated untracked directory.
- timestamp: 2026-08-21T23:31:00Z
  checked: Commit of intended source and documentation files
  found: Commit c5656fe contains only e2e/harness_test.go and .planning/phases/03-create-flow-backend/03-11-SUMMARY.md; its hooks passed.
  implication: The fix is buildable and isolated; protected dirty evidence and unrelated untracked paths remain outside the commit.

## Resolution
<!-- OVERWRITE as understanding evolves -->

root_cause: "The D-22 FakeSSHDir fixture emitted a hard-coded ssh -G IdentityFile after 03-11 made realBackend.TestStage2 correctly validate the first effective IdentityFile from its staged config; strict validation therefore rejected every fake stage-two proof."
fix: "FakeSSHDir now requires and parses the supplied staged -F config, emitting its User, Hostname, Port, IdentitiesOnly, and first IdentityFile; it fails closed when the config is missing or incomplete. Added a focused harness regression test."
verification: "Focused harness test passed; all four named raw-PTY regressions passed; make test-e2e passed (58.884s); make test passed; make lint reported 0 issues. Commit c5656fe passed hooks. Awaiting user confirmation."
files_changed:
  - e2e/harness_test.go
