---
phase: 04-git-configuration-screen
verified: 2026-08-25T00:00:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 4: Git Configuration Screen Verification Report

**Phase Goal:** After the SSH screens, a developer configures the per-identity Git fragment on its own screen, reviews it, and confirms the write of fragment + `includeIf` + `allowed_signers`.
**Verified:** 2026-08-25
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + plan must_haves)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A separate Git-config screen (after the SSH screens) collects `user.name`/`user.email`/`gpg.format=ssh`/`user.signingkey`/`commit.gpgsign`, written to `~/.gitconfig.d/<identity>` (GITUI-01, GITUI-02) | ✓ VERIFIED | `cmd/gitid/wiring.go:808` `GitStepDisabledReason` now always returns `("", false)` (the Phase-3 D-19 always-disabled gate is retired). Wizard reaches `Step 3/4` (Git) after the two-stage SSH proof (`e2e/create_flow_pty_e2e_test.go:354-450` `TestCreateFlow_GitConfigurationDefaultTracer`). `internal/gitconfig/fragment.go:35-72` `WriteFragment` emits exactly `user.name`/`user.email`/`gpg.format=ssh`/`user.signingkey`(path)/`commit.gpgsign` to `~/.gitconfig.d/<identity>`, proven by `cmd/gitid/wiring_test.go:891` `TestCommitCreateWritesDefaultGitArtifacts` and the real-PTY fragment read-back in both e2e files. Standalone entry (`g` on an identity) is the same reusable flow (D-07), proven by `internal/tuikit/identities_test.go:155` `TestGitFlowAliasCollisionResume` and `e2e/git_configuration_pty_e2e_test.go:159` `TestGitConfiguration_RealPTYCompleteEditFlow`/`:245` `TestGitConfiguration_RealPTYSSHOnlyCompletionFlow`. |
| 2 | User chooses the match strategy (`gitdir:`/`hasconfig:remote.*.url`, default `gitdir`, combinable) with a live `includeIf` preview (GITUI-03) | ✓ VERIFIED | `internal/gitconfig/renderer.go:34-73` renders `gitdir`/`hasconfig`/`both` (two OR'd `[includeIf]` sections) exactly per D-02/D-03; round-trip proven by `internal/gitconfig/renderer_test.go`/`reader_test.go` (`TestIncludeIfStrategies`, `TestRenderParseRoundTrip`, both pass in the full suite run). Live preview refresh is wired through `gitForm.includeIfPreview`/`matchesFor`, exercised in the real PTY via `match-strategy-select` checkpoints in `TestGitConfiguration_RealPTYCompleteEditFlow` (cycles gitdir→hasconfig→both→gitdir with live label changes). |
| 3 | `~/.ssh/allowed_signers` line is written with the email byte-identical to `user.email` (GITUI-04) | ✓ VERIFIED | `internal/keygen/signers.go:31-40` `AllowedSignersLine` uses the email argument byte-identically as the principal (and, post-CR-18, rejects comma-injection). `WriteAllowedSignersReplacing` (`:69-75`) replaces rather than appends the identity's block, proven at both unit (`TestGitFlowEditDiffReplacesSignerEmail`, `internal/tuikit/identities_test.go:2553`) and real-PTY level (`TestGitConfiguration_RealPTYCompleteEditFlow` asserts the new principal is present and the stale one is absent after an edited email). |
| 4 | A read-only review screen precedes the write; on confirm, fragment + `includeIf` + `allowed_signers` are written with backup and idempotent managed blocks (GITUI-05) | ✓ VERIFIED | `internal/tuikit/identities.go:1580` `reviewCeremony` renders the combined SSH Host block + fragment + includeIf preview and is read-only until an explicit confirm keystroke (asserted pre-confirm-untouched in `TestCreateFlow_GitConfigurationDefaultTracer` at both the Git-step and review-ceremony checkpoints, and in `TestGitConfiguration_RealPTYCompleteEditFlow`'s `assertGitBytesUnchanged` calls before confirm). `commitCreateTransaction`/`commitGitTransaction` (`cmd/gitid/wiring.go`) back up every pre-existing target, journal every created file/dir, and roll back in reverse order on failure — proven by `TestGitTransactionRollbackMatrixPreservesSnapshotsAndSafetyBackups`, `TestCombinedTransactionRollsBackEverySSHAndGitTarget`, and the real symlink-escape rejection in `TestGitConfiguration_RealPTYWriteFailureRollback` (write fails closed, symlink and all watched files byte-unchanged, retry offered). |
| 5 | UI-wave gate: PTY e2e drives every screen in the real binary; automated review compares it with `cmd/gitid-dummy` and classifies every difference as an improvement or a defect (DLV-04, DLV-06) | ✓ VERIFIED | `e2e/git_configuration_pty_e2e_test.go` drives every named Git-screen state (git-form-empty/filled, match-strategy-select, review-readonly, confirm-write, result-success/failure, mouse focus) through the compiled real binary at 100×30 (`TestGitConfiguration_RealPTY*`, 4 functions). `TestGitConfiguration_CompiledRealVsLiveDummyPTY` drives both `cmd/gitid` and `cmd/gitid-dummy` compiled binaries and classifies all 8 real/dummy divergences as `ux-improvement` (zero `defect`, zero unclassified) against `.planning/design/git-screen/visual-divergence-allowlist.txt`. `make gate-visual-regression` (run live during this verification) passes with 23 `RequiredScreenSpecs` (18 create-flow + 5 git-screen) and its 9 sub-tests including 3 negative controls proving the check is non-vacuous. No HTML/MUI/browser/Chromium/pixel/PNG import found in either file (`grep` for chromedp/playwright/puppeteer/selenium returns nothing). |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified).

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/tuikit/identities.go` | Enabled post-SSH Git flow, combined review, reusable seven-state create/edit flow | ✓ VERIFIED | `reviewCeremony`, `gitForm`, `changedLines`, alias-collision resume (`TestGitFlowAliasCollisionResume`) all present and wired; 3798 lines, exercised by 300+ unit tests and 5 real-PTY e2e functions. |
| `cmd/gitid/wiring.go` | Real default Git artifact transaction behind `CommitCreate`/`CommitGit` | ✓ VERIFIED | `commitCreateTransaction` (id.GitConfigured-gated), `commitGitTransaction`, `CommitGit`, `GitStepDisabledReason` (neutered) all present; unit + real-PTY coverage confirmed. |
| `e2e/create_flow_pty_e2e_test.go` / `e2e/git_configuration_pty_e2e_test.go` | Compiled-real-TUI default-path + per-state tracer proof | ✓ VERIFIED | Both files present; all named `TestCreateFlow_Git*` and `TestGitConfiguration_*` functions found and pass under `make test-e2e` (full run, 256.353s, exit 0). |
| `internal/gitconfig/renderer.go`, `reader.go` | Canonical includeIf/provider-rewrite render/write/parse APIs | ✓ VERIFIED | `RenderIncludeIf`, `WriteIncludeIf`, `ProviderRewriteBlockName`, `RenderProviderRewrite`, `WriteProviderRewrite`, `HasProviderRewrite`, `IsReservedBlockName`, `ParseManagedIncludeIf` all present and covered by the full `internal/gitconfig` package test suite (9.144s, pass). |
| `internal/doctor/checks/reserved_test.go` | Doctor non-destruction regression for provider rewrite blocks | ✓ VERIFIED | `TestOrphansReservedGitRewriteSurvivesFix`, `TestOrphansUnreservedGitBlockControl` present and pass (non-vacuous — a real orphan control is removed while the reserved block survives). |
| `.planning/design/git-screen/visual-divergence-allowlist.txt` | Named, decision-cited semantic classifications | ✓ VERIFIED | Exists; consumed by `TestGitConfiguration_CompiledRealVsLiveDummyPTY`'s `loadGitScreenAllowlist`. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `internal/tuikit/identities.go` (wizard) | `cmd/gitid/wiring.go` `CommitCreate` | `GitSpec` carried through `finishIdentity`/`reviewCeremony` | ✓ WIRED | `id.GitConfigured`/`GitName`/`GitEmail`/`GitDir`/`ForceSSH` populated only when `w.configureGit` is true; `commitCreateTransaction` gates Git writes on `id.GitConfigured`. |
| `cmd/gitid/wiring.go` | `internal/gitconfig`, `internal/keygen` | `WriteFragment`/`WriteIncludeIf`/`WriteAllowedSignersReplacing` | ✓ WIRED | Called in both `commitCreateTransaction` (combined) and `commitGitTransaction` (standalone), both going through the shared `filewriter` atomic/backup chokepoint. |
| `internal/gitconfig/renderer.go` | `internal/filewriter/block.go` | provider-keyed `ReplaceBlock` writes | ✓ WIRED | `WriteProviderRewrite` and `WriteIncludeIf` both route through `filewriter.ReplaceBlock`/`filewriter.Write`. |
| `internal/doctor/checks/orphans.go` | `internal/gitconfig/reader.go` | `IsReservedBlockName` excludes non-identity blocks | ✓ WIRED | Confirmed via `TestOrphansReservedGitRewriteSurvivesFix` passing with a real orphan present as a non-vacuous control. |
| `e2e/git_configuration_pty_e2e_test.go` | `cmd/gitid`/`cmd/gitid-dummy` | `BuildBinary`/`BuildDummyBinary`/`startPTYAt` at 100×30 | ✓ WIRED | Both binaries built and driven with raw keystrokes/SGR mouse in the same test file. |

### Behavioral Spot-Checks / Commands Run Live

| Command | Result | Status |
|---|---|---|
| `TERM=dumb SSH_AUTH_SOCK= go build ./...` | exit 0 | ✓ PASS |
| `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` | 19 packages ok, 0 failures | ✓ PASS |
| `TERM=dumb SSH_AUTH_SOCK= make lint` | 0 issues (default + `-tags screenshot`) | ✓ PASS |
| `TERM=dumb SSH_AUTH_SOCK= make test-e2e` | ok, 256.353s | ✓ PASS |
| `TERM=dumb SSH_AUTH_SOCK= make gate-visual-regression` | ok — 23 `RequiredScreenSpecs`, 9 sub-tests incl. 3 negative controls | ✓ PASS |
| `go test -tags e2e -race -count=1 ./e2e -run '^TestCreateFlow_Git(ConfigurationDefaultTracer\|SkipCreatesNoGit\|CancelCreatesNoGit)$'` | matched only `TestCreateFlow_GitConfigurationDefaultTracer` (1 test) — see Gaps/Notes below | ⚠️ NOTE |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| GITUI-01 | 04-01, 04-03, 04-04 | Separate post-SSH per-identity screen | ✓ SATISFIED | `Step 3/4` reached only after the real two-stage SSH proof; standalone `g`-launch reuses the same component (D-07). |
| GITUI-02 | 04-01, 04-02, 04-03 | Fragment fields at `~/.gitconfig.d/<identity>` | ✓ SATISFIED | `WriteFragment` exact-field contract, unit + e2e proven. |
| GITUI-03 | 04-02, 04-03 | gitdir/hasconfig/both + live preview | ✓ SATISFIED | Renderer round-trip + real-PTY match-strategy-select checkpoints. |
| GITUI-04 | 04-01, 04-03 | Email-identical `allowed_signers`, edit replaces not appends | ✓ SATISFIED | `AllowedSignersLine`/`WriteAllowedSignersReplacing` + `TestGitFlowEditDiffReplacesSignerEmail` + real-PTY stale-principal-removed assertion. |
| GITUI-05 | 04-01, 04-03, 04-04 | Read-only review, confirm, backup, idempotent write | ✓ SATISFIED | `reviewCeremony` + transactional writers + rollback matrix + real symlink-escape rejection test. |
| DLV-04 | 04-04 | Live real TUI vs approved Bubble Tea mockup; classify every difference | ✓ SATISFIED | `TestGitConfiguration_CompiledRealVsLiveDummyPTY` + `make gate-visual-regression`, zero defect/unclassified. CR-15/CR-16 (a documented, user-accepted structural limitation of what a real-vs-dummy differential gate can detect — it cannot catch a defect that is identical on both sides because both binaries share the `internal/tuikit` renderer) is a recorded, closed scope decision per `04-REVIEW-FIX.md`'s "Resolution" section — not treated as an open gap here per this verification's explicit brief. |
| DLV-06 | 04-01, 04-04 | At least one real PTY e2e path per Git screen/state | ✓ SATISFIED | 5 real-PTY functions across every named Git-screen state (create, SSH-only completion, complete edit, mouse, rollback failure). |

No orphaned requirements found — REQUIREMENTS.md's Phase-4 row set (GITUI-01..05) matches what the plans declared; DLV-04/DLV-06 recur here as documented per REQUIREMENTS.md's "DLV-01..06 additionally recur as UI-wave success criteria in every UI-bearing phase" convention.

### Anti-Patterns Found

None. Scanned every phase-modified core file (`internal/tuikit/identities.go`, `cmd/gitid/wiring.go`, `internal/gitconfig/renderer.go`, `internal/gitconfig/reader.go`, `internal/keygen/signers.go`, both `e2e/*git*_pty_e2e_test.go` files, `internal/screenshot/createflow*.go`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`/"not yet implemented"/"coming soon" — zero hits inside Phase 4 scope. (One `"not yet implemented by gitid"` string exists at `internal/tuikit/identities.go:3327`, but it is the KEY-01 algorithm-catalog disabled-row copy from Phase 1/3, unrelated to Git configuration.)

### Gaps Summary

No blocking gaps. One process-level (non-blocking) note:

- **Test-name deviation, not a functional gap.** Plan `04-01-PLAN.md`'s Task 1/2 verify commands name `TestCreateFlow_GitSkipCreatesNoGit` and `TestCreateFlow_GitCancelCreatesNoGit` as the negative-control e2e tests proving "Esc before confirmation and Skip Git create none of the Git artifacts" (D-08/D-10). Neither test exists under those names, and `04-01-SUMMARY.md`'s "Deviations from Plan" section states "None" despite this. The underlying guarantee is still verified, through different evidence: (1) `commitCreateTransaction`'s Git-write branch is structurally gated on `id.GitConfigured` (`cmd/gitid/wiring.go`, confirmed by direct source read) so Skip Git cannot reach the Git writers; (2) `cmd/gitid/wiring_test.go:1745` `TestPersistSkipGitWritesSSHOnlyNoGitArtifacts` proves the real backend's write path (the same `commitCreateTransaction`) leaves `.gitconfig.d/<identity>`, `.gitconfig`, and `allowed_signers` absent for a Skip-Git identity; (3) `internal/tuikit/identities_test.go:1036` `TestWizardSkipCreatesIncompleteIdentity` proves the wizard-level Skip Git path at the UI-state level; (4) `TestCreateFlow_GitConfigurationDefaultTracer` asserts the live SSH config is untouched both before entering the Git step and again at the review ceremony (pre-confirm), and Git artifacts are written strictly after the SSH host block in the same transaction, so the same "nothing written before confirm" guarantee applies transitively. This is recorded here for traceability, not as a blocking finding — the truth it was meant to prove does hold.

No deferred items (nothing in this phase maps to work explicitly re-scoped to a later phase).

---

*Verified: 2026-08-25*
*Verifier: Claude (gsd-verifier)*
