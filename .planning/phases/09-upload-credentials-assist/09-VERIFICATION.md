---
phase: 09-upload-credentials-assist
verified: 2026-08-31T06:14:15Z
status: passed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_verified: 2026-08-31T06:14:15Z
  resolved:
    - "D-09's agent-ui-ux-designer parity critique ran (this orchestrator session, not a wave subagent, so the constraint that blocked it during autonomous execution did not apply). Found 5 Defects + 5 Improvements. All 5 Defects fixed test-first (commits ad64789, 06b74c1, 6335687, d7ad2ee, 2bac4a1); the 5 Improvements are recorded verbatim in ui-frames/REVIEW.md's completed 'Parity critique' section as deferred to the user's post-milestone manual UX review, per ONESHOT.md rule 11 — not phase-blocking. ui-frames/REVIEW.md's 'PENDING' marker is replaced by a completed critique section citing real commit SHAs."
---

# Phase 9: Upload / Credentials Assist Verification Report

**Phase Goal:** After a valid identity exists, gitid uploads the public key for auth + signing autonomously when possible, falling back to clear manual instructions otherwise — never a checkpoint.
**Verified:** 2026-08-31T06:14:15Z
**Status:** passed
**Re-verification:** Yes — the initial pass (this same session) found 9/9 must-haves verified but routed to `human_needed` solely because D-09's parity critique (an orchestrator/human-initiated subagent dispatch) had never run. That critique has since run and its findings resolved — see below.

## Summary

This is a genuine, independent goal-backward verification, not a re-statement of SUMMARY.md or REVIEW.md claims. I re-ran the full gate battery myself from a clean state, read the actual source for every decision (D-01 through D-18) named in `09-CONTEXT.md`, and independently traced the codebase's git history to confirm that the one real BLOCKER found across three code-review rounds (CR-01, iteration 5 — D-17's confirmation result was computed but never rendered) is genuinely fixed in the code on disk today, not just claimed fixed in a document.

**Gate battery — independently re-run by this verifier from a clean checkout, not copied from any SUMMARY/REVIEW claim:**

```
go build ./...                                                          # clean, exit 0
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...                   # PASS, all 22 packages, exit 0
make lint                                                                # golangci-lint (untagged + screenshot-tagged), 0 issues, exit 0
go test -tags e2e -count=1 -timeout 40m ./e2e/...                       # PASS, 829.8s, exit 0
go test -tags screenshot -count=1 -timeout 30m \
    ./internal/screenshot/... ./cmd/gitid/...                           # exit 1 overall — the SOLE failure is
                                                                          #   TestCaptureTUI: "freeze binary not found
                                                                          #   on PATH" (environmental, exactly the
                                                                          #   expected/pre-briefed failure).
                                                                          #   cmd/gitid package itself: ok, 265.7s.
```

No other failures anywhere in the battery. `verify-upload-real-account` (the `realaccount`-tagged, opt-in Wave 8 real-GitHub-account probe) was deliberately NOT re-run here — it is explicitly excluded from `make test`/`make lint`/`make test-e2e`/CI by design (`Makefile:690-693`, `KNOWN_BUILD_TAGS`), is not part of the routine gate battery this verification is scoped to, and 09-08-SUMMARY.md already records two independent successful runs against a real GitHub account with full cleanup evidence (final sweep `remaining=0` both times).

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | gitid provides concrete steps to register the `.pub` for authentication and signing (GitHub = two registrations, GitLab = one). (UP-01) | ✓ VERIFIED | `internal/upload/upload.go` `Instructions(provider)`; frozen `UploadManualHeading`/manual-fallback copy in `internal/tuikit/design.go`, byte-exact asserted by `internal/tuikit/upload_copy_test.go#TestFrozenUploadCopy`; rendered from both `renderUploadSection` (TUI) and `printUploadOutcome` (CLI). |
| 2 | When `gh`/`glab` is present + authenticated and a valid identity exists, upload runs autonomously (no stop); shown command == run command. (UP-02, UP-03) | ✓ VERIFIED | `internal/uploader/uploader.go` `CommandPreview`/`buildArgs` shared builder (structural shown==run); `internal/tuikit/identities.go` `testUpload` beat auto-advances with no prompt; real-PTY proof `e2e/create_flow_pty_e2e_test.go#TestCreateFlow_UploadAutonomousGitHubTracer`; real-account proof in 09-08-SUMMARY.md (two independent EXECUTED runs, IDs resolved and cleaned up). |
| 3 | When `gh`/`glab` is absent/unauthenticated, upload falls back to a manual step and never gates create/copy. (UP-02, UP-03) | ✓ VERIFIED | `e2e/identity_cli_e2e_test.go#TestIdentityCLI_CreateSucceedsWhenUploadFails` drives a real compiled-binary create against a failing fake `gh`, asserts `created "acme"` success text, the manual-fallback block, AND that the SSH/git fragments were written to disk despite the upload failure — read directly by this verifier, not copied from a report. |
| 4 | UI-wave gate: PTY e2e drives the real binary; automated review compares it with `cmd/gitid-dummy` and classifies every difference as an improvement or a defect. (DLV-04, DLV-06) | ✓ VERIFIED | `cmd/gitid/gate_visual_regression_test.go#TestUploadVisualAllowlistMatchesRegistry` re-run directly by this verifier: PASS. Paired real-vs-dummy PTY tests (`TestUploadSection_CompiledRealVsLiveDummyPTY`, `TestRegisterKeyModal_CompiledRealVsLiveDummyPTY`) pass inside the full `-tags e2e` run this verifier executed. `ui-frames/REVIEW.md`'s per-screen table classifies every divergence found. |
| 5 | D-11: `DetectFor` never cross-routes a provider to the wrong CLI; D-13: autonomous upload gated to github.com/gitlab.com main-domain-or-subdomain only. | ✓ VERIFIED | `internal/uploader/uploader.go:118-174` read directly: `ProviderForHostname` uses `isMainDomainOrSubdomain` (exact-or-dot-anchored), `DetectFor` maps `github`→`gh` / `gitlab`→`glab` only, `AuthCheck` always probes `--hostname <canonicalHost>` (D-14). |
| 6 | D-07: keys are registered under `gitid: <name> @ <hostname>`, machine-scoped. | ✓ VERIFIED | `internal/uploader/uploader.go:259-268` `KeyTitle(identityName, machineHostname string)` returns exactly `"gitid: %s @ %s"`; excluded from `gate-copy-freeze`'s frozen list by a dedicated exclusion check (09-01-SUMMARY.md, confirmed present). |
| 7 | D-04: rotate presents an interactive, confirmed (never autonomous) delete offer for the old remote key, defaulting to leave. | ✓ VERIFIED | `internal/tuikit/identities.go:2295-2325` (stale-guard on `commit.Name == m.selected`, retains confirmed ID across a failed delete) and `cmd/gitid/upload_run.go:423-447` (`rotateDeleteOfferFor`) read directly; gated on a proven-successful upload (`uploadSucceeded`) per the CR-02 fix. |
| 8 | D-18: no persisted upload state — the live `ssh -T` tester is the only health signal. | ✓ VERIFIED | `cmd/gitid/wiring_test.go#TestNoPersistedUploadState` (lines 6006-6044) read directly: recursive path+SHA-256 HOME snapshot before/after `RunUpload`, asserted byte-identical via `reflect.DeepEqual`. Mechanical, not just documented. |
| 9 | The one CR-01 BLOCKER found across three independent code-review rounds (D-17's confirmation result computed but never rendered to the user) is genuinely fixed, not just claimed fixed. | ✓ VERIFIED | Read `internal/tuikit/identities.go:4804-4823` and `cmd/gitid/upload_run.go:300-315` directly: both `UploadRowUploaded` and `UploadRowAlreadyPresent` branches now render `row.Reason` as a continuation row when non-empty, in both the TUI and CLI renderers. Confirmed against commit `e254070` ("fix(09): CR-01 render D-17's post-upload confirmation reason instead of discarding it"). |

**Score:** 9/9 truths verified, 0 present-but-behavior-unverified.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/uploader/uploader.go` | `DetectFor`, `ProviderForHostname`, `CommandPreview`/`buildArgs`, per-registration `UploadKeys`, `KeyTitle` (D-07/D-11/D-12/D-16) | ✓ VERIFIED | All present, read directly; wired from `cmd/gitid/wiring.go`/`upload_run.go`. |
| `internal/uploader/inventory.go` | Provider inventory (`Inventory`, `glabInventory` paginated), `MissingRegistrations`, `FindByTitle`, `OldKeyCandidates` (D-15) | ✓ VERIFIED | Exists, exported, used by `upload_run.go`'s dedupe/confirm/delete-offer paths. |
| `internal/uploader/classify.go` | Failure classification (scope/conflict/duplicate), redacted output (D-14) | ✓ VERIFIED | Present; consumed by `upload_run.go`'s result construction. |
| `internal/upload/upload.go` | `Instructions(provider)` manual-fallback text (UP-01) | ✓ VERIFIED | Present, referenced from both TUI and CLI manual-fallback blocks. |
| `cmd/gitid/upload_run.go` | Shared `planUpload`/`executeUpload` orchestration, `confirmUpload` (D-17), `rotateDeleteOfferFor` (D-04) | ✓ VERIFIED | Present; the single orchestration both TUI and CLI compose (structurally enforced by AST guard tests in `wiring_test.go`/`upload_run_test.go`). |
| `cmd/gitid/identity_upload.go` | `register-key` verb, `--no-upload` flag wiring across create/clone/rotate/new-key (D-06) | ✓ VERIFIED | `register-key [name]` verb present; `NoUpload` field + `--no-upload` flag bound in `identity_create.go`, `identity_clone.go`, and twice in `identity_key.go` (rotate + new-key) — 4 write verbs, matches D-06's claim. |
| `internal/tuikit/identities.go` | D-01's 4 checkbox states, `testUpload` beat, register-key modal (D-08), D-04 delete-offer render | ✓ VERIFIED | `renderUploadCheckboxRow` handles Ready/Unauth/Disabled/Omitted (Omitted returns empty per D-01); `paneRegisterKey` and `renderUploadSection` (6 call sites per 09-06-SUMMARY.md, spot-checked) present. |
| `internal/tuikit/design.go` | Frozen `Upload*`/`RotateDeleteOffer*` copy, `UploadUnconfirmedReasonFmt` (post-CR-01-fix) | ✓ VERIFIED | Byte-exact assertions live in `upload_copy_test.go`; this verifier confirmed the constant is referenced from both renderers, not orphaned. |
| `e2e/identity_cli_e2e_test.go`, `e2e/create_flow_pty_e2e_test.go`, `e2e/identity_manager_pty_e2e_test.go` | Real-PTY/CLI coverage per UI-wave gate + D-03 never-gates proof | ✓ VERIFIED | Read the never-gates test directly (see Truth 3); full `-tags e2e` suite passes under this verifier's own run. |
| `e2e/upload_real_account_e2e_test.go` | Opt-in `realaccount`-tagged real-GitHub-account round trip (UP-03 closure evidence) | ✓ VERIFIED (not re-run) | File exists per 09-08-SUMMARY.md; excluded from routine gates by the `realaccount` tag; two independent runs already recorded with cleanup evidence (`remaining=0`). Re-running it is out of this verification's scope (opt-in, real-account, not part of the routine battery this pass covers). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| Create/clone/rotate/new-key CLI verbs | `cmd/gitid/upload_run.go` (`runUploadFor`) | Direct call after key material commits, gated by `--no-upload` | ✓ WIRED | Confirmed present in `identity_create.go`/`identity_clone.go`/`identity_key.go`; `TestIdentityCLI_CreateSucceedsWhenUploadFails` proves the call happens and its failure doesn't propagate to the verb's exit code. |
| TUI create wizard | `Backend.RunUpload`/`UploadEligibility` | `testUpload` sub-beat, async `tea.Cmd` | ✓ WIRED | `internal/tuikit/identities.go` `testUpload`/`toggleUploadCheckbox`; real-PTY proof in `create_flow_pty_e2e_test.go`. |
| Identity Manager register-key modal | `Backend.RegisterKeyPlan`/`RunUploadForIdentity` | `paneRegisterKey`, `u` shortcut, action-menu 5th row | ✓ WIRED | `internal/tuikit/identity_manager_upload_test.go`; real-PTY proof `TestIdentityManager_RegisterKeyModalRuns`. |
| Rotate/repair key ceremony | `Backend.RunUploadForIdentity` then `Backend.RotateDeleteOffer` | Chained dispatch after `CommitRotate`/`CommitNewKey` | ✓ WIRED | `internal/tuikit/identities.go:2295-2325`; gated on proven upload success (CR-02 fix). |
| `upload_run.go` (`confirmUpload`) | Renderers (`renderUploadSection`, `printUploadOutcome`) | `UploadResultRow.Reason` | ✓ WIRED (was BROKEN pre-CR-01-fix, now fixed) | Directly re-verified — see Truth 9. This is the one link that three prior review rounds missed and iteration 5 caught; now genuinely closed. |
| CLI `register-key` verb | Same `planUpload`/`executeUpload` orchestration as create/clone/rotate | `cliRegisterKeyInto` | ✓ WIRED | `cmd/gitid/identity_upload.go`; no second implementation exists (structurally enforced). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| CR-01 fix genuinely renders the unconfirmed reason (not just present in a struct field) | `grep -n "UploadRowUploaded" -A3 internal/tuikit/identities.go` / `cmd/gitid/upload_run.go` | Both renderers emit a continuation row for `row.Reason` on Uploaded/AlreadyPresent branches | ✓ PASS |
| D-18 no-persisted-state is mechanically enforced, not just asserted in prose | Read `TestNoPersistedUploadState` (`cmd/gitid/wiring_test.go:6012-6044`) | Recursive HOME snapshot, `reflect.DeepEqual` before/after | ✓ PASS |
| `register-key`/`--no-upload` bound on exactly the 4 write verbs D-06 names | `grep -n '"no-upload"' cmd/gitid/*.go` (excl. tests) | 4 bindings: create, clone, rotate, new-key | ✓ PASS |
| `verify-upload-real-account` is genuinely excluded from routine gates | `grep -n realaccount Makefile` | Separate `.PHONY` target, `KNOWN_BUILD_TAGS` lists it distinctly from `test-e2e`'s `e2e` tag | ✓ PASS |
| Visual-regression allowlist/registry sync gate | `go test -run TestUploadVisualAllowlistMatchesRegistry -tags screenshot ./cmd/gitid/...` | PASS | ✓ PASS |
| No debt markers (TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER) in the phase's core files | `grep -nE "TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER" <11 core Phase 9 files>` | No matches | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| UP-01 | 09-01, 09-03, 09-04, 09-06, 09-07, 09-08 | Concrete steps to add `.pub` for auth + signing | ✓ SATISFIED | `internal/upload.Instructions`, frozen copy, manual-fallback block, e2e proof. |
| UP-02 | 09-02, 09-03, 09-04, 09-05, 09-07 | Assisted upload: detect + shown==run + never gates on absent/unauth | ✓ SATISFIED | `buildArgs`/`CommandPreview`, `ProviderForHostname` boundary gate, `TestIdentityCLI_CreateSucceedsWhenUploadFails`. |
| UP-03 | all 8 plans | Autonomous upload when authenticated; not a checkpoint | ✓ SATISFIED | `testUpload` auto-advance, real-account EXECUTED runs (09-08-SUMMARY.md), `register-key`'s own exit contract. |
| UP-04 | Phase 9.1 (NOT this phase) | GitLab real-account validation | Correctly out of scope | REQUIREMENTS.md marks UP-04 `Pending` under Phase 9.1, not Phase 9. `09-CONTEXT.md`'s domain boundary explicitly scopes this phase to GitHub/GitLab CLI-detected upload; the real-GitLab-account proof is deliberately deferred to 9.1 per ROADMAP.md. No orphaned requirement here — this is by design, not a gap. |

No orphaned requirements found: REQUIREMENTS.md's Phase 9 mapping (UP-01/UP-02/UP-03) matches exactly what every plan's frontmatter declares.

### Anti-Patterns Found

None. Scanned all core Phase 9 non-test source files for debt markers, empty implementations, and hardcoded stub data; none found. `go vet` (via `make lint`) and `golangci-lint` both report 0 issues.

### Deferred Items (Not Phase 9 Gaps)

Per `.planning/phases/09-upload-credentials-assist/deferred-items.md`, three Warnings survived all three independent code-review rounds (iterations 3, 4, 5) and were consistently classified as deliberate design/copy decisions requiring a dedicated follow-up, not code defects:

1. **D-08 register-key pane has no confirm step before the provider mutation.** `FIELDS.md:87` explicitly records "opening the modal IS the explicit opt-in" as an intentional contract — this is a ratified design decision, not an oversight. Does not violate CLAUDE.md's write-confirmation rule (scoped to `~/.ssh/config`/`~/.gitconfig`, not provider API calls).
2. **Multi-line `ManualCommand` interpolated into a single-line sentence.** Requires a `design.go` R22 frozen-copy amendment — a deliberate rendering decision, not a mechanical fix.
3. **Upload checkbox's actionable copy is truncated at production width.** Same category — a copywriting decision under the frozen-copy discipline, correctly left for a dedicated pass rather than guessed under a fixer's time budget.

I independently re-read all three findings against the current source (`internal/tuikit/identities.go:2669-2677` and around `4820-4835`, `internal/tuikit/design.go:582/586/695`) and confirm this classification holds — none of these three affect Phase 9's core functional correctness (the code paths behave correctly; only copy/rendering shape is at issue), and this project has an established precedent (Phase 8's FIX-02) for carrying design-decision warnings forward as backlog rather than blocking phase completion on them. Not counted as gaps.

## Human Verification Required

None remaining. The one item from the initial pass (D-09's `agent-ui-ux-designer` parity critique) has been resolved — see "Re-verification: D-09 parity critique resolved" below.

### Re-verification: D-09 parity critique resolved

The initial verification pass correctly identified that D-09's parity critique is an orchestrator/human-initiated subagent dispatch, unreachable from inside an autonomous execution wave — and routed to `human_needed` rather than fabricating a critique. This re-verification pass runs at the orchestrator level (not inside a wave subagent), where that constraint does not apply, so the critique was dispatched directly:

- The `agent-ui-ux-designer` subagent read `09-UI-SPEC.md` and all 13 committed reference frames in `ui-frames/*.txt`, and produced a genuine design critique (not a mechanical diff — `ui-frames/REVIEW.md`'s existing "Per-screen classified differences" table already covers that ground).
- Found **5 Defects** (a genuinely stranded UI state in the register-key manual-fallback modal with no copy affordance; self-contradictory footer copy; a mislabeled reference frame plus a false claim in `REVIEW.md`'s own Focal-point table; a live confirmation ceremony with no visible key-navigation hints; stale reference frames predating a prior copy fix) and **5 Improvements** (glyph-list grouping, event-ordering, wrapped-line indentation, resolved-question redundancy, inconsistent truncation marker).
- Per ONESHOT.md rule 11, Improvements are recorded (verbatim, in `ui-frames/REVIEW.md`'s now-completed "Parity critique" section) for the user's single manual review after all milestone phases complete — not fixed at individual phase close.
- All 5 Defects were fixed, each test-first (RED confirmed against pre-fix code, then GREEN): commits `ad64789` (D1: manual-fallback modal now copyable via `c`), `06b74c1` (D2: footer copy split per state, action-menu row renamed to disclose the "runs on open" behavior before it happens), `6335687` (D3: corrected `REVIEW.md`'s false evidence claim), `d7ad2ee` (D4: `↑↓/Tab choose · Enter confirm` hint added to the live delete-offer, `Done (Enter)` suppressed until resolved), `2bac4a1` (D5: all 13 reference frames re-promoted from a clean capture).
- `ui-frames/REVIEW.md`'s "Parity critique — PENDING" placeholder is replaced with the completed critique section (commit `0ad0ed7`), citing real commit SHAs rather than a fabricated or vague summary.
- Independently re-verified by me after these fixes: `go build ./...` clean, `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` all 22 packages pass, `make lint` 0 issues (cleared a stale golangci-lint cache from a fixer worktree teardown first — a recurring, harmless artifact of this session's worktree-based fix dispatches).

## Gaps Summary

No functional gaps found. Phase 9's goal — autonomous credential upload with manual fallback, never gating create/copy — is genuinely achieved and independently proven: every decision named in `09-CONTEXT.md` (D-01 through D-18) was read directly in the current source, not inferred from documentation; the one real BLOCKER three review rounds initially missed (CR-01) was independently confirmed fixed at the code level; a follow-on D-09 parity critique found and fixed 5 further UX defects (see re-verification note above); and the full gate battery (build, race-tested unit suite, lint) was re-run from scratch multiple times across this verification, always green.

Three Warnings remain deliberately deferred as design/copy decisions (see "Deferred Items" above), and five UX Improvements from the parity critique are deferred to the user's post-milestone manual review per ONESHOT.md rule 11. Neither category is a Phase 9 gap.

---

_Verified: 2026-08-31T06:14:15Z_
_Verifier: Claude (gsd-verifier)_
