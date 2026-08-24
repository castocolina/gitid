---
phase: 03-create-flow-backend
reviewed: 2026-08-23T00:00:00Z
depth: focused
files_reviewed: 3
files_reviewed_list:
  - Makefile
  - cmd/gitid/gate_visual_regression_test.go
  - cmd/gitid/wiring.go
  - cmd/gitid/wiring_cr_test.go
  - e2e/create_flow_pty_e2e_test.go
  - internal/identity/modes.go
  - internal/screenshot/createflow.go
  - internal/screenshot/createflow_regions.go
  - internal/screenshot/createflow_test.go
  - internal/sshconfig/validation.go
  - internal/tester/tester.go
  - internal/tester/tester_command_test.go
  - internal/tuikit/identities.go
  - internal/tuikit/store.go
  - internal/tuikit/views.go
  - .planning/design/create-flow/visual-divergence-allowlist.txt
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 3: Historical Code Review Report

**Reviewed:** 2026-08-21T21:41:10Z  
**Depth:** deep  
**Files Reviewed:** 16  
**Status:** historical — superseded by the focused clean review below

## Summary

Current HEAD `8dbf1aa` remains blocked after gap-closure commits `4130dcf` and `a2b6461`. The code does remove the generated-key pre-confirm `~/.ssh` directory mutation, propagates algorithm/provider fields into the commit DTO, and adds stricter IdentityFile validation. Those changes do not close the phase, however.

The explicit packet publisher is a successful no-op, no `03-10-review-packet` exists, no 24-panel packet is produced, and no authenticated independent reviews were run. The routine visual gate compares current-HEAD real and dummy text only; it never captures approved HTML or approved TUI from approval commit `3c3130e`, never produces PNG evidence, and still permits broad region drift. More seriously, the production stage-two path still discards the `ssh -G` error/output, never calls the newly added validator, records the connectivity outcome before checking resolution, and cannot render both proof commands and outputs. Green Make targets therefore do not establish the claimed behavior.

**Verdict: BLOCKED. Phase 3 is not clean and must not advance.**

**Commands actually run at current HEAD:**

- `TERM=dumb SSH_AUTH_SOCK= make test` — passed (Go packages reported cached).
- `make lint` — passed, 0 issues.
- `TERM=dumb SSH_AUTH_SOCK= make test-e2e` — passed (reported cached).
- `make gate-visual-regression` — passed, but the gate defects below make this result non-probative.
- `make generate-visual-review-packet SOURCE_COMMIT=a2b646138c8e57c9e203b1e64c9adfddae47b171 OUTPUT_DIR=<new temp path>` — exited 0 while creating no directory or files (`created=no`).

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01 [BLOCKER]: The required immutable packet publisher is a successful no-op

**File:** `Makefile:291-302`

**Issue:** `generate-visual-review-packet` validates only that two variables are non-empty and that an existing destination is not non-empty, then prints two lines and exits successfully. It does not validate the source commit, create the destination, capture anything, hash anything, or publish the required 8 live + 8 approved-TUI + 8 approved-HTML panels. A direct run returned exit 0 with `created=no`. There is also no tracked `03-10-review-packet` at HEAD. Consequently original CR-05 and CR-06 remain open: there is no immutable evidence and no evidence-bound independent review.

**Fix:** Implement a real publisher that rejects dirty/unknown/short source commits and existing destinations, captures all three eight-panel sets, writes a canonical manifest and provenance, hashes every member, validates exactly 24 panels, and atomically creates a content-addressed destination. Exit nonzero on every missing tool/input/output or write failure.

### CR-02 [BLOCKER]: Approval commit provenance is asserted as a constant but never consumed

**File:** `cmd/gitid/gate_visual_regression_test.go:54-57,326-337,518-542`

**Issue:** The only approval handling is a hardcoded SHA plus a test that checks length/hex syntax. The gate constructs `dummytui.NewFixtureBackend()` from current HEAD. `TestApprovalCommitRecorded` explicitly does not invoke Git and even skips when `.git` is unavailable. No approved HTML or approved TUI source is read from `3c3130e`, and no approved artifact hash is reproduced. Original CR-03 is therefore unchanged in substance.

**Fix:** Export the approved HTML and TUI source trees from full commit `3c3130e404329cf42baafdf63a6c22758437edc6` into isolated temporary storage, run the pinned capture tools against those files, and record separate approval/live commands, SHAs, panel IDs, and hashes. Fail if either approved surface cannot be reproduced.

### CR-03 [BLOCKER]: The visual gate validates only 16 current-HEAD text renders, not the required 24 PNG panels

**File:** `cmd/gitid/gate_visual_regression_test.go:299-455,545-570`; `internal/screenshot/createflow.go:248-352`

**Issue:** The gate creates two maps of eight rendered text strings (real backend and current dummy backend). It does not invoke Chromium, Freeze, font/theme checks, HTML capture, TUI PNG capture, or packet hash validation. `TestAllScreensCapturedAndNonEmpty` explicitly expects only 8 real + 8 dummy text screens. Thus missing tools, missing approved panels, stale PNGs, and incomplete packet contents cannot make the routine gate fail. This leaves original CR-05 open and does not satisfy DLV-04/DLV-06.

**Fix:** Make the routine gate generate two complete candidate bundles in temporary directories, require byte-identical inventories, verify 8 live + 8 approved-TUI + 8 approved-HTML PNGs and all hashes/provenance, and compare against an immutable committed packet without writing tracked files.

### CR-04 [BLOCKER]: Region policy still permits arbitrary drift and its negative controls do not fail

**File:** `cmd/gitid/gate_visual_regression_test.go:275-290,388-438,573-640`; `.planning/design/create-flow/visual-divergence-allowlist.txt:53-126`

**Issue:** There is no explicit per-screen required-region schema. Any region empty on both sides is silently skipped. For a differing broad region, `contains:` and `absent:` inspect only the real region for one token; all other bytes may change arbitrarily. Examples such as `header-status:contains:"ids"`, `connectivity-output:contains:"ssh"`, and `host-preview:contains:"Host acme.github.com"` do not constrain the actual divergence. The purported exhaustive negative control mutates an arbitrary prefix, then either `continue`s or merely `Logf`s when extraction is mutation-insensitive; it never runs the gate against each mutation and requires rejection. Original CR-04 remains open.

**Fix:** Define mandatory non-empty regions for each screen, compare exact before/after values except for narrowly specified normalization functions, and table-test every protected subregion by applying a mutation and asserting the real gate rejects it. Mutation-insensitive extraction must be fatal.

### CR-05 [BLOCKER]: Production stage two still accepts incomplete or failed `ssh -G` proof

**File:** `internal/tester/tester.go:195-204,207-271`; `cmd/gitid/wiring.go:574-597`; `internal/tuikit/views.go:60-74`

**Issue:** `ResolvedVia` still discards the `ssh -G` execution error and returns only parsed stdout. `realBackend.TestStage2` never calls `ValidateResolvedConfig`, never checks nonempty raw output, and records the connectivity outcome at line 590 before doing any resolution validation. The newly added `ResolvedViaGCommand`, `ExpectedResolution`, and `ValidateResolvedConfig` are disconnected helpers used only by tests. `TestResultView` still has room for only one command and one detail string, so the real renderer cannot carry both exact commands and both raw outputs. A wrong, empty, or failed `ssh -G` can still unlock persistence. Original CR-07 remains fully open in the production call chain.

**Fix:** Replace the `ResolvedVia` return contract with an error-bearing stage-two proof containing connectivity command/output/error, resolution command/raw output/error, and parsed fields. In `TestStage2`, validate User, Hostname, Port, IdentitiesOnly, and first effective IdentityFile before recording an accepted outcome. Extend the backend-free view DTO and renderer to show both command/output pairs.

### CR-06 [BLOCKER]: Visual stage-two evidence is synthetic and omits the complete proof

**File:** `internal/screenshot/createflow.go:197-229,288-329`; `internal/tuikit/identities.go:2901-2938`

**Issue:** The capture wrapper fabricates stage two as one connectivity command plus `Detail: "identityfile <path>"`. It does not carry the real `ssh -G` command, raw `ssh -G` output, errors, or the required User/Hostname/Port/IdentitiesOnly fields. The TUI renders only `stage2Cmd()` (the connectivity call) and one truncated detail line. The state-transition bug from original CR-02 was improved, but the resulting panel still cannot prove stage-two correctness and cannot demonstrate the required real backend/render contract.

**Fix:** Drive a controllable offline implementation of the same complete stage-two proof type used in production and assert the captured panel contains both exact commands, both raw outputs, and every validated effective field.

### CR-07 [BLOCKER]: Staged-key cache identity still ignores algorithm/provider and permits stale key reuse

**File:** `cmd/gitid/wiring.go:91-98,965-996,1007-1012`

**Issue:** Algorithm/provider now reach `DemoIdentity` and `createInput`, but `stagedKeyFor` reuses cached material based only on identity name and reuse path. If the user stages/tests one algorithm, navigates back, and selects another algorithm for the same identity, the cache can return the old key material while the proof fingerprint and commit input claim the new algorithm. The fingerprint also omits generate-vs-reuse source and reuse path, despite the gap-closure plan requiring both. Existing tests directly manufacture accepted outcomes and do not exercise a raw-PTY RSA flow. Original CR-10 is only partially fixed.

**Fix:** Key staged material and accepted proof on a canonical fingerprint that includes algorithm, provider, key source, and normalized reuse path. Invalidate staged material and both outcomes whenever any of those fields changes. Add the required real-binary raw-PTY RSA create test through confirmation and persisted Host block.

### CR-08 [BLOCKER]: IdentityFile validation is not enforced at the final render/execute boundary

**File:** `cmd/gitid/wiring.go:240-255,443-454,472-481,1257-1264`; `internal/sshconfig/validation.go:47-97`

**Issue:** Character validation is stricter, but it is called only through the UI's earlier `ValidateHostBlock` method. Both the staged test config and confirmed transaction call `sshconfig.RenderHostBlock` directly without authoritative revalidation immediately before render/execution. The preview also renders directly. This does not satisfy the stated CR-11 trust-boundary fix and leaves non-UI/backend callers able to interpolate malformed tokens. Unicode whitespace above the C1 range is also accepted despite the comment claiming all Unicode whitespace is rejected.

**Fix:** Introduce a checked render function (or validate immediately before every render) and make staged config, preview, and confirmed persistence use it. Reject `unicode.IsSpace` and all controls, then retain parser plus real `ssh -G` round-trip tests at that checked boundary.

### CR-09 [BLOCKER]: Rollback can leave a newly created real `~/.ssh` directory behind

**File:** `cmd/gitid/wiring.go:1124-1148,1169-1185,1237-1249`

**Issue:** On a fresh HOME, rollback records `~/.ssh` first and `~/.ssh/config.d` later. The rollback loop removes `createdDirs` in creation order. Removing `~/.ssh` first fails because `config.d` still exists; the loop then removes `config.d` but never retries `~/.ssh`. A failure after Include-directory creation can therefore leave the real SSH directory behind, contradicting the all-or-nothing transaction guarantee associated with CR-08/CR-09.

**Fix:** Remove created directories in reverse order and treat rollback failures as observable errors. Add failure injection after every mutation and compare the complete pre/post filesystem snapshot, including directory existence and modes.

### CR-10 [BLOCKER]: Determinism is limited to one test process and excludes the evidence that matters

**File:** `cmd/gitid/gate_visual_regression_test.go:236-273,311-349`; `internal/screenshot/createflow.go:37-49`

**Issue:** `sync.OnceValue` wraps a fresh cryptographic key generation, so bytes are stable only within one test process, not reproducible across independent invocations or machines. The two-run check compares text only and performs no PNG or packet hash comparison. Temporary path normalization is not general; only timestamp-shaped strings are replaced. Because the publisher is absent, there is no independently repeatable bundle inventory. Original CR-01 remains open despite the ordinary gate now avoiding tracked writes.

**Fix:** Use committed public-only deterministic fixture data or a deterministic DTO, normalize all disposable paths and injected clock/backup values, run two independent complete bundle generations, and require identical text, PNG, manifest, and per-file hashes before publication.

## Warnings

### WR-01 [WARNING]: Multi-label provider metadata still diverges between preview and persisted output

**File:** `cmd/gitid/wiring.go:443-454,901-942,1257-1264`

**Issue:** The commit DTO now preserves `DemoIdentity.Provider`, closing the original truncation in the persistence input. However `HostBlockPreview` still calls `providerFromAlias`, which truncates `enterprise.company.co.uk` to `co.uk`, while confirmed persistence renders `in.Provider` (`company.co.uk`). The UI's “written exactly like this on confirm” claim is false for the exact multi-label case WR-01 targeted.

**Fix:** Render previews from `spec.Provider` after validation, using the same checked render function and provider value as the confirmed transaction. Remove suffix reconstruction from this path.

## Original Finding Disposition

| Original finding | Code-level verdict | Remaining evidence |
|---|---|---|
| CR-01 | OPEN | CR-10: cross-process/full-bundle determinism absent. |
| CR-02 | PARTIAL | State transitions differ now, but CR-06 shows incomplete synthetic proof. |
| CR-03 | OPEN | CR-02: approval commit never loaded or captured. |
| CR-04 | OPEN | CR-04: broad predicates, skipped regions, ineffective controls. |
| CR-05 | OPEN | CR-01/CR-03: no publisher, PNG gate, 24-panel count, or packet validation. |
| CR-06 | OPEN | CR-01: no new packet and no authenticated independent reviews. |
| CR-07 | OPEN | CR-05: new helpers are disconnected from production and render. |
| CR-08 | CLOSED for pre-confirm mutation | Generated and reused staging paths reviewed; no real SSH-directory write/chmod found before confirmation. Transaction rollback remains broken under CR-09. |
| CR-09 | PARTIAL | Reused private-key chmod/restore exists; full filesystem rollback fails under CR-09. |
| CR-10 | PARTIAL | DTO propagation exists; stale staged cache and missing real-PTY proof remain under CR-07. |
| CR-11 | PARTIAL | Token checks improved; final render boundary remains unchecked under CR-08. |
| WR-01 | PARTIAL | Commit uses preserved provider; preview still reconstructs/truncates it. |

---

_Reviewed: 2026-08-21T21:41:10Z_  
_Reviewer: the agent (gsd-code-reviewer)_  
_Depth: deep_  
_Verdict: historical BLOCKED report; superseded after 03-16 and 03-17 remediation._

---

## Current Focused Code Review

**Reviewed:** 2026-08-23  
**Model:** `openai/gpt-5.6-sol-fast`  
**Session:** `ses_fce27e8b2ffeU87pitC1Rie7rM`  
**Scope:** Phase 3 commits `8801551..32ec1f6`, focused on the 03-17 production
presentation changes and their direct unit and PTY regression tests.  
**Verdict:** clean

The reviewer inspected `internal/tuikit/identities.go`,
`internal/tuikit/identities_test.go`, and `e2e/create_flow_pty_e2e_test.go`.
It found no BLOCKER or WARNING. The review was invoked directly through
`opencode run --model openai/gpt-5.6-sol-fast`; no plan-review model was used.
