---
phase: 05-identity-manager
plan: 09
subsystem: testing
tags: [pty-e2e, visual-regression, real-vs-dummy, gitid, bubbletea, taxonomy-bugs]

# Dependency graph
requires:
  - phase: 05-identity-manager
    provides: "plans 05-01 through 05-08 — the full identity-manager backend (delete lifecycle, rotate/repair ceremonies, D-03 read surface, D-01 CLI tree, CLI parity matrix) this plan proves end-to-end through the compiled binary"
provides:
  - "e2e/identity_manager_pty_e2e_test.go — raw-keystroke PTY coverage of every approved manager state, both key-ceremony modes, the D-13 backstop, and the FIELDS.md manifest backstop (review R-26)"
  - "the paired compiled-real-vs-live-dummy PTY comparison (DLV-04) plus its in-process gate-visual-regression counterpart, both classifying against .planning/design/identity-manager/visual-divergence-allowlist.txt"
  - "3 pre-existing production bug fixes in the tilde-vs-absolute-path family, blocking key-used-both/D-12 shared-key detection for every recipe-shaped identity"
  - "the frozen list-empty first-run landing state (internal/tuikit/identities.go), previously unimplemented anywhere in the render stack"
affects: [phase-06-global-ssh-options, phase-07-global-git-options, phase-08-health-fixer, gsd-verify-work]

actuals:
  tokens: 41700
  tasks: 3
  commits: 2

tech-stack:
  added: []
  patterns:
    - "errorRecorder-interface negative controls: swap a *testing.T-shaped interface for a non-propagating fake recorder so a 'prove this correctly fails' test can observe a would-be failure without Go's subtest-failure-propagation marking the negative-control test itself as failed"
    - "ShortSandboxHome (os.MkdirTemp under /tmp, never t.TempDir()) wherever an assertion needs a literal, un-wrapped absolute path inside a bounded-width PTY pane — t.TempDir()'s test-name-embedding path reliably exceeds the ~55-column clip/wrap budget"
    - "b.normalizedAccounts() — a Backend helper returning every reconstructed account run through normalizeAccountForWrite, the single fix point for every cross-identity path-equality comparison (SharedKeyOwners, ProviderRefCount) that must compare against an already-normalized target account"

key-files:
  created:
    - .planning/design/identity-manager/visual-divergence-allowlist.txt
  modified:
    - e2e/identity_manager_pty_e2e_test.go
    - e2e/harness_test.go
    - internal/screenshot/createflow.go
    - internal/screenshot/createflow_regions.go
    - internal/screenshot/createflow_test.go
    - internal/screenshot/createflow_packet_test.go
    - cmd/gitid/gate_visual_regression_test.go
    - cmd/gitid/identity_read.go
    - cmd/gitid/wiring.go
    - internal/identity/inventory.go
    - internal/tuikit/design.go
    - internal/tuikit/identities.go
    - Makefile

key-decisions:
  - "rotate-result/repair-result checkpoints are registered in the e2e PTY comparison (Task 2) but DELIBERATELY EXCLUDED from the in-process gate-visual-regression registry (Task 3): both need a real (or FakeSSHDir-substituted) SSH connectivity probe that an in-process, no-subprocess gate has no way to inject without defeating its own determinism/speed purpose"
  - "identity-manager's in-process gate decision refs use bare D-NN (this phase's own 05-CONTEXT.md, per 05-09-PLAN.md's <authority> convention) plus two new prefixes: DLV- for the fixture-set-size divergence class (a property of the comparison mechanism, not a numbered decision) and MGR-D- for a genuine Phase 5 decision cited inside the SHARED createflow.go registry, where bare D-11 would collide with Phase 3's own D-11"
  - "8 taxonomy identities in seedEightTaxonomyIdentities cover all 8 MGR-02 labels via 7 identities (key-used-both is axis-only, unreachable as a collapsed row word) — kun's key-unused state requires a hand-written duplicate Host stanza preceding the managed block (the one real-world condition breaking BuildInventory's own-block self-reference)"
  - "the git-only-delete receipt's 'Wrote → ' line legitimately names the fragment file the transaction REMOVES (D-10) — assertReceiptPathsExist (review R-28) is therefore wired into rotate/repair (pure-write ceremonies) only, not delete, and this scope boundary is documented in-test rather than silently applied everywhere"

patterns-established:
  - "FIELDS.md manifest backstop: parse the design manifest's Field column at test time (never transcribe field names into the test), map each field id to its CURRENT literal substring in a small living dictionary, and fail loudly on an unmapped field — the drift-detection mechanism review R-26 requires"
  - "Real-vs-dummy PTY comparison + in-process semantic-capture comparison as a MATCHED PAIR reusing ONE allowlist file and ONE decision-ref vocabulary, exactly mirroring Phase 4's git-screen precedent for Phase 5's identity-manager"

requirements-completed: [MGR-01, MGR-03, MGR-04, MGR-05, MGR-06, MGR-07, SHELL-01, SHELL-02, KEY-05, KEY-07, DLV-04, DLV-06]

coverage:
  - id: D1
    description: "Full per-state raw-keystroke PTY suite against the compiled real binary: list-populated (8-label taxonomy, 7 row words, orphan-key), list-empty, detail-ssh-first, action-menu, delete-choice + mouse focus, rotate/repair ceremonies, delete-everything (planted D-13 hits + zero-hits control + D-12 shared-key note + D-09 provider survival)"
    requirement: "DLV-06"
    verification:
      - kind: e2e
        ref: "e2e/identity_manager_pty_e2e_test.go#TestIdentityManager_(ListPopulatedEightTaxonomy|ListEmpty|DetailSSHFirst|ActionMenu|KeyCeremonyRotate|KeyCeremonyRepair|MouseCloneAndDeleteChoiceFocus|DeleteEverything.*)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Independent FIELDS.md manifest backstop (review R-26) — parsed at test time, asserted against the real binary alone, with a field-removal negative control"
    requirement: "DLV-06"
    verification:
      - kind: e2e
        ref: "e2e/identity_manager_pty_e2e_test.go#TestIdentityManager_ManifestBackstopNegativeControl"
        status: pass
    human_judgment: false
  - id: D3
    description: "Paired compiled-real-vs-live-dummy PTY comparison across 6 checkpoints, classified against a decision-cited allowlist with stale/unclassified hard-failure and 2 negative controls"
    requirement: "DLV-04"
    verification:
      - kind: e2e
        ref: "e2e/identity_manager_pty_e2e_test.go#TestIdentityManager_CompiledRealVsLiveDummyPTY"
        status: pass
      - kind: unit
        ref: "e2e/identity_manager_pty_e2e_test.go#TestIdentityManager_Allowlist(UnclassifiedDifferenceRejected|StaleEntryRejected)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Phase 5 registered in the repeatable in-process gate-visual-regression, 7 new regions, 4 checkpoints, 4 negative controls, no regression to the existing create-flow/git-screen registries"
    requirement: "DLV-04"
    verification:
      - kind: unit
        ref: "cmd/gitid/gate_visual_regression_test.go#TestGateVisualRegression"
        status: pass
      - kind: unit
        ref: "cmd/gitid/gate_visual_regression_test.go#TestNegativeControl_(MissingIdentityManagerState|IdentityManagerUnclassifiedDifferenceRejected|AllIdentityManagerComparableEqualRegionsAreMutationSensitive|IdentityManagerCrossRegistryLeakage)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The previously-planned manual walk (review R-28) converted to automated assertions: receipt-path filesystem existence (with negative control), delete-confirm key-copy phrasing scope, success-never-claimed-without-the-work"
    requirement: "DLV-06"
    verification:
      - kind: e2e
        ref: "e2e/identity_manager_pty_e2e_test.go#TestIdentityManager_(ReceiptPathsNegativeControl|SuccessNeverClaimedWithoutTheWork)"
        status: pass
      - kind: unit
        ref: "internal/tuikit/identities_test.go#TestDeleteBackupNoticeKeyCopyAndCannotBeUndoneScope"
        status: pass
    human_judgment: false
  - id: D6
    description: "Full battery green: build, race unit tests, tagged lint, end-to-end PTY suite, copy freeze, no-backend boundary, visual gate"
    verification:
      - kind: other
        ref: "make gate-visual-regression && make test && make lint && make test-e2e && make gate-no-backend-files"
        status: pass
    human_judgment: false

duration: ~5h (single long session)
completed: 2026-08-26
status: complete
---

# Phase 5 Plan 09: Full Per-State PTY Suite + Real-vs-Dummy Comparison + Repeatable Gate Summary

**Every approved identity-manager state (plus both new key-ceremony modes) is now proven through the compiled binary with raw keystrokes and real filesystem evidence, backed by an independent FIELDS.md manifest backstop, a paired real-vs-dummy PTY comparison, and its in-process gate-visual-regression counterpart — and along the way, three pre-existing tilde-vs-absolute-path bugs that silently broke key-used-both classification and D-12 shared-key detection for every recipe-shaped identity were found and fixed.**

## Performance

- **Duration:** ~5h (single long session; this is the phase's FINAL plan — a real-PTY suite exercising the whole phase's surface, explicitly scoped as high-effort by the plan's own `confidence: low` estimate)
- **Completed:** 2026-08-26
- **Tasks:** 3
- **Files modified:** 14 (1 created, 13 modified)

## Accomplishments

- **Task 1 — the full per-state PTY suite.** `e2e/identity_manager_pty_e2e_test.go` grew from the 05-01 tracer (2 tests) to 20 test functions covering every FIELDS.md-approved state plus the phase's own scoped additions: `seedEightTaxonomyIdentities` seeds 7 identities whose axis pairs union to all 8 locked MGR-02 labels and whose collapsed `ClassifyState` words are exactly the 7 reachable values (never 8 — review R-06's binding resolution); the D-13 backstop plants exactly 2 unmanaged-reference hits at KNOWN `(file, line)` identities (review R-27) and a zero-hits control; the D-12 shared-key note and D-09 provider-rewrite survival each get a dedicated fixture and test. The independent FIELDS.md manifest backstop (review R-26) parses `.planning/design/identity-manager/FIELDS.md` at test time — never transcribing field names into the test source — and asserts every required field against the REAL binary's decoded frame alone, with a field-removal negative control proving it is not vacuous.
- **Three real, pre-existing bugs found and fixed (Rule 1/3), all in the same family: a normalized (absolute) path compared against an un-normalized (literal-tilde) one.** These blocked the phase's own acceptance criteria and, more importantly, silently broke production correctness for the recipe-shaped `~/.ssh/id_ed25519_<name>` convention this ENTIRE project uses:
  1. `cmd/gitid/identity_read.go`'s `identity list/show --json` built its inventory via the bare `identity.InventoryDepsForHome(home)` instead of the backend's own tilde-expanding `b.inventoryDeps()` — every identity's key silently reported `key-missing` regardless of reality.
  2. `internal/identity/inventory.go`'s `resolveKeyUsedInGit` read a fragment via the UNEXPANDED `acct.FragmentPath`, unlike `Reconstruct`'s own tilde-expand-then-read — making `key-used-both` (the git-signing half of the KeyState axis) unreachable for any recipe-shaped identity, in BOTH the CLI and the live TUI.
  3. `cmd/gitid/wiring.go`'s `DeletePlan`/`buildDeleteDeps` compared a normalized (absolute) `acct.KeyPath` against OTHER accounts' un-normalized (tilde) `KeyPath` when computing `SharedKeyOwners` — silently reporting a genuinely shared key as unshared, both in the D-12 confirm-screen PREVIEW and in `identity.Delete`'s own real `keySurvives` WRITE decision. This is the most severe of the three: it meant a real delete-everything could have destroyed a key another identity still needed. Fixed via a new `b.normalizedAccounts()` helper both call sites now use.
- **A fourth gap: the list-empty first-run state did not exist anywhere in the render stack.** `internal/tuikit/identities.go`'s `renderDetail` unconditionally rendered the normal SSH-first detail path even for a zero-value `DemoIdentity` when `len(s.Identities) == 0`, producing confusing copy ("relies on the global SSH config") instead of the frozen "No identities yet" / "Press n to create your first identity" landing state FIELDS.md specifies. Added the frozen copy constants (`internal/tuikit/design.go`) and the branch (Rule 2 — auto-added missing production behavior explicitly required by this plan's own must_haves).
- **Task 2 — the paired real-vs-dummy PTY comparison (DLV-04).** Reuses Phase 4's git-screen mechanism exactly: 6 checkpoints (action-menu, delete-choice, confirm-destructive, detail-ssh-first, rotate-result, repair-result), normalized semantic region comparison, a strict 5-field allowlist schema in `.planning/design/identity-manager/visual-divergence-allowlist.txt`, stale/unclassified hard-failure, and 2 negative controls implemented via a new `errorRecorder` interface (lets a test observe a would-be failure without Go's subtest-failure propagation marking the negative-control test itself failed).
- **Task 3 — Phase 5 registered in the repeatable in-process `gate-visual-regression` gate.** 7 new regions (identity list rows reuse the existing `RegionSidebar`), 4 checkpoints captured in-process (rotate-result/repair-result deliberately excluded — see Decisions), 4 negative controls mirroring the git-screen ones exactly, and 8 pre-existing tests updated to merge identity-manager captures the same way they already merge git-screen's. The previously-planned manual human-verification block (review R-28, which directly contradicted this plan's own `autonomous: true`) is now three mechanical assertions instead.

## Task Commits

1. **Task 1 + Task 2 (per-state PTY suite + paired comparison, committed together per CLAUDE.md's buildable-boundary rule — Task 2 depends on Task 1's fixture helpers in the SAME file)** - `f68e8d0` (feat)
2. **Task 3 (gate registration + manual-walk automation)** - `ced59e8` (feat)

## Files Created/Modified

- `e2e/identity_manager_pty_e2e_test.go` — grew from 222 to 1,855 lines: the full per-state suite, the FIELDS.md manifest backstop, the paired real-vs-dummy comparison + allowlist parser, and the R-28 receipt-path/success-never-claimed automation.
- `e2e/harness_test.go` — `seedEightTaxonomyIdentities`, `seedSharedKeyIdentities`, `seedTwoIdentitiesSameProviderE2E`, `seedDeleteEverythingTargetWithPlantedHits`/`Clean`, `ShortSandboxHome`, and their small composable building-block helpers (`sshHostBlock`, `gitconfigIncludeIfBlock`, `signingFragment`/`plainFragment`).
- `internal/tuikit/design.go` — `IdentityManagerEmptyStateCopy`/`IdentityManagerEmptyStateCTA` frozen constants.
- `internal/tuikit/identities.go` — `renderDetail`'s list-empty branch.
- `cmd/gitid/identity_read.go` — `buildIdentityRecords` now uses `b.inventoryDeps()` (tilde-expanding Stat).
- `internal/identity/inventory.go` — `resolveKeyUsedInGit` now tilde-expands `acct.FragmentPath` before reading it.
- `cmd/gitid/wiring.go` — `b.normalizedAccounts()`; both `Accounts:` closures (`buildDeleteDeps`, `DeletePlan`) now use it.
- `internal/screenshot/createflow.go` — `CaptureIdentityManagerScreens`, `identityManagerSpecs`, `isIdentityManagerScreenID`, `validDecisionRef` widened for `DLV-`/`MGR-D-`.
- `internal/screenshot/createflow_regions.go` — 7 new `RegionName`s + extractors, `identRightOfDivider`/`extractIdentityRightOfDividerBetween` shared helpers.
- `internal/screenshot/createflow_test.go`, `internal/screenshot/createflow_packet_test.go` — capture helpers merge identity-manager captures.
- `cmd/gitid/gate_visual_regression_test.go` — `deterministicIdentityManagerFixture`, `mergeIdentityManagerCaptures`, wired into `TestGateVisualRegression`/`ReadOnly`/`TestAllScreensCapturedAndNonEmpty`/2 git-screen negative controls, plus 4 new Phase-5-scoped negative controls.
- `.planning/design/identity-manager/visual-divergence-allowlist.txt` — new registry file (12 fixture-size entries + 4 content-divergence entries).
- `Makefile` — `test-e2e`/`gate-visual-regression` comments name the Phase 5 registration and the rotate/repair exclusion rationale.

## Decisions Made

See `key-decisions` in the frontmatter for the three load-bearing ones. Additionally:

- **The list-populated taxonomy fixture uses 7 identities, not 8**, because `kun`'s `key-unused` state requires a hand-written duplicate Host stanza (an OpenSSH "first obtained value wins" trick) rather than a distinct 8th identity — verified empirically via a scratch `identity.BuildInventory` call before being written into the fixture (see the "Issues Encountered" section for the empirical method).
- **`assertReceiptPathsExist` (review R-28) is deliberately NOT called on the git-only-delete receipt**, whose "Wrote → " target list legitimately names the fragment file the transaction REMOVES (D-10) — the claim "every receipt path exists" only holds for create-like receipts (rotate/repair), documented in-test rather than silently over-applied.
- **rotate-result/repair-result are e2e-PTY-only, never in-process-gate-registered** — both need a real SSH connectivity probe (`cmd/gitid/lifecycle.go`'s `preWriteGate`) an in-process, no-subprocess gate cannot substitute (no PATH injection point for `FakeSSHDir`), and forcing one in would defeat the gate's own determinism/speed purpose.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `identity list/show --json` misreported every identity's key as missing**
- **Found during:** Task 1, writing the list-populated eight-taxonomy test
- **Issue:** `cmd/gitid/identity_read.go`'s `buildIdentityRecords` called `identity.BuildInventory(identity.InventoryDepsForHome(home))` directly instead of the backend's own `b.inventoryDeps()`, which wraps `Stat` with tilde expansion. `os.Stat("~/.ssh/id_ed25519_x")` never expands `~` itself, so every recipe-shaped identity's key silently reported `key-missing` via the CLI JSON surface.
- **Fix:** `buildIdentityRecords` now calls `identity.BuildInventory(b.inventoryDeps())`.
- **Files modified:** `cmd/gitid/identity_read.go`
- **Verification:** `TestIdentityManager_ListPopulatedEightTaxonomy` (e2e), full `cmd/gitid` unit suite unchanged (24.6s, no regressions).
- **Committed in:** `f68e8d0`

**2. [Rule 1 - Bug] `key-used-both` unreachable for any recipe-shaped identity (real AND dummy render stack)**
- **Found during:** Task 1, same test
- **Issue:** `internal/identity/inventory.go`'s `resolveKeyUsedInGit` called `readFragment(acct.FragmentPath)` with the VERBATIM (often tilde-form) `FragmentPath`, never expanding it — unlike `Reconstruct`'s own internal fragment read, which tilde-expands first. The fragment read failed silently (`FragmentInfo{Missing: true}`), so `keyUsedInGit` was always `false`.
- **Fix:** `resolveKeyUsedInGit` now expands the tilde before calling `readFragment`, mirroring `Reconstruct`'s pattern exactly.
- **Files modified:** `internal/identity/inventory.go`
- **Verification:** `TestIdentityManager_ListPopulatedEightTaxonomy`; `TERM=dumb SSH_AUTH_SOCK= go test -race ./internal/identity/...` (2.6s, no regressions).
- **Committed in:** `f68e8d0`

**3. [Rule 1 - Bug, security-adjacent] `SharedKeyOwners` silently reported a genuinely shared key as unshared**
- **Found during:** Task 1, `TestIdentityManager_DeleteEverythingSharedKeyNote`
- **Issue:** `cmd/gitid/wiring.go`'s `DeletePlan`/`buildDeleteDeps` passed `b.accounts()` (un-normalized, literal-tilde `KeyPath`) as the `Accounts` seam to `identity.PlanDelete`/`identity.Delete`, which compare `SharedKeyOwners(accounts, acct.KeyPath, ...)` against the TARGET account's `acct.KeyPath` — already normalized to an absolute path by `normalizeAccountForWrite`. An absolute path never string-equals a literal-tilde path, so the shared-key check silently reported `keySurvives = false` for every recipe-shaped fixture, in BOTH the confirm-screen preview AND `identity.Delete`'s real write decision.
- **Fix:** New `b.normalizedAccounts()` helper (runs every `b.accounts()` entry through `normalizeAccountForWrite`); both `Accounts:` closures now use it.
- **Files modified:** `cmd/gitid/wiring.go`
- **Verification:** `TestIdentityManager_DeleteEverythingSharedKeyNote`; full `cmd/gitid`/`internal/identity` unit suites unchanged.
- **Committed in:** `f68e8d0`

**4. [Rule 2 - Missing critical] list-empty first-run state was unimplemented**
- **Found during:** Task 1, writing `TestIdentityManager_ListEmpty`
- **Issue:** No code path anywhere in `internal/tuikit/identities.go` rendered the FIELDS.md-frozen "No identities yet" / "Press n to create your first identity" copy — an empty sandbox home rendered `renderDetail`'s normal (misleading) SSH-first path against a zero-value identity instead. This plan's own must_haves truth explicitly requires it to render correctly against the compiled binary.
- **Fix:** Added `IdentityManagerEmptyStateCopy`/`IdentityManagerEmptyStateCTA` frozen constants (`internal/tuikit/design.go`) and an early branch in `renderDetail` for `len(s.Identities) == 0`.
- **Files modified:** `internal/tuikit/design.go`, `internal/tuikit/identities.go`
- **Verification:** `TestIdentityManager_ListEmpty`; full `internal/tuikit` unit suite unchanged (19.2s, no regressions).
- **Committed in:** `f68e8d0`

**5. [Rule 3 - Blocking, cross-registry collision] `validDecisionRef` rejected the identity-manager registry's own decision refs**
- **Found during:** Task 3, first `TestGateVisualRegression` run
- **Issue:** `internal/screenshot/createflow.go`'s `validDecisionRef` only accepted `D-`/`T-`/`CTX-D-`/`UI-D-` prefixes. The identity-manager registry's fixture-size divergence class needed a NEW citation (`DLV-NN`, since it is a property of the comparison mechanism, not any single decision), and its own `D-11` citation collided with Phase 3 create-flow's OWN `D-11` in the SAME shared registry (different decisions, same bare number).
- **Fix:** Widened `validDecisionRef` to accept `DLV-` and `MGR-D-` (the latter scoping Phase 5's OWN decisions the same way `CTX-D-` already scopes Phase 4's, inside this shared file).
- **Files modified:** `internal/screenshot/createflow.go`
- **Verification:** `TestGateVisualRegression`, `TestNegativeControl_IdentityManagerCrossRegistryLeakage`.
- **Committed in:** `ced59e8`

**6. [Rule 3 - Blocking] Registering identity-manager in the shared registry broke 8 pre-existing tests**
- **Found during:** Task 3, first full `go test -tags screenshot ./...` run after registration
- **Issue:** `RequiredScreenSpecs()` is a merged registry every consumer iterates; several pre-existing tests (`captureCombined`, `makeTestCaptures`, `TestAllScreensCapturedAndNonEmpty`, 4 negative controls, `TestRegionDiffCoverage`, `TestCaptureCreateFlowScreensFailsOnMissingRequiredFrame`) built their capture maps from `CaptureCreateFlowScreens`/`CaptureGitScreenScreens` only, unaware a THIRD registry now existed, and failed with "required live frame missing" for every new identity-manager screen ID.
- **Fix:** Each fixed to also merge `CaptureIdentityManagerScreens`, mirroring how they already merge `CaptureGitScreenScreens`.
- **Files modified:** `internal/screenshot/createflow_test.go`, `internal/screenshot/createflow_packet_test.go`, `cmd/gitid/gate_visual_regression_test.go`
- **Verification:** Full `go test -tags screenshot ./...` — all pass except the pre-existing, unrelated `TestCaptureTUI` (see Issues Encountered).
- **Committed in:** `ced59e8`

**7. [Rule 1 - Bug] Two new regions collided with unrelated create-flow/git-screen content**
- **Found during:** Task 3, iterating `TestGateVisualRegression`
- **Issue:** `BuildRegionDiffs` extracts EVERY named region from EVERY registered screen regardless of that screen's own `RequiredRegions` — a bare "Git" heading marker and a bare "Backup → " marker (both plausible-looking, narrowly-scoped-sounding choices) matched unrelated create-flow ("test-stage1-direct") and git-screen ("confirm-write") frames, producing spurious "differs with no disposition" failures on screens Phase 5 never touched.
- **Fix:** `extractDetailGitSection` now requires an EXACT trimmed-line match ("Git" alone, not a substring) instead of `Contains`; `extractBackupPathList` now requires the frame to also carry the "Delete EVERYTHING for" anchor before scanning for backup lines at all.
- **Files modified:** `internal/screenshot/createflow_regions.go`
- **Verification:** `TestGateVisualRegression` passes with zero collateral divergences on the 23 pre-existing create-flow/git-screen specs.
- **Committed in:** `ced59e8`

---

**Total deviations:** 7 auto-fixed (3 pre-existing production bugs — Rule 1, one security-adjacent; 1 missing critical production behavior — Rule 2; 3 blocking issues discovered while wiring the new registry into the shared gate — Rule 3).
**Impact on plan:** All auto-fixes were necessary for correctness (the 3 tilde-path bugs silently broke real product behavior this project's own recipe-shape convention relies on everywhere) or to make this plan's own acceptance criteria achievable (list-empty, the cross-registry collisions). No scope creep beyond fixing what blocked this plan's explicit must_haves.

## Issues Encountered

- **The `key-used-both`/`key-unused` reachability investigation required empirical experimentation**, not just code reading: a scratch `identity.BuildInventory` call (built, run, then deleted — never committed) against a hand-written fixture confirmed the tilde-expansion bugs directly before they were fixed in production code, and confirmed the "hand-written duplicate Host stanza" mechanism as the one real-world path to `key-unused`. This is the `CLAUDE.md` "hypothesis → test → implementation" loop applied to a genuine ambiguity in how the classifier behaves, not assumed from documentation.
- **`TestCaptureTUI` (internal/screenshot) fails independent of this plan's changes** — confirmed by stashing all Task 3 changes and rerunning: `freeze` binary not on PATH (`make setup-env` installs it). Pre-existing environment/tooling gap, out of scope for this plan.
- **`TestApprovalCommitRecorded` SKIPs inside a worktree** (`.git` is a file, not a directory, so `open ../../.git/HEAD` fails its directory-read assumption) — a known worktree-context limitation of that specific test, not a regression; the orchestrator's own non-worktree gate run is unaffected.
- **A real terminal word-wrap constraint required a dedicated fixture helper.** `t.TempDir()`'s test-name-embedding absolute path reliably exceeds the ~55-column budget a bounded `PreviewBlock` clips/wraps at, breaking exact `(file, line)` substring assertions and the receipt-path parser alike. `ShortSandboxHome` (rooted at `/tmp` directly, bypassing the long default `$TMPDIR`) resolves this and is reused across 4 tests.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- **Phase 5 (Identity Manager) is functionally complete** — all 9 waves (05-01 through 05-09) landed. The full battery is green: `make gate-visual-regression` (16.6s), `make test` (unit + tagged screenshot, ~140s), `make lint` (0 issues), `make test-e2e` (444.8s, well under the 900s budget), `make gate-no-backend-files` (retired at Phase 3 per D-17; the durable `TestNoBackendAllowlist` guard passes), plus the plan's own explicit verification (`go list -deps ./internal/tuikit` — zero first-party imports, the render-stack boundary holds).
- **Phase-level human acceptance is still owed at `/gsd-verify-work`**, per DLV-08's convention (the phase's one hard stop was Phase 2's design approval, already recorded) — this plan's own `autonomous: true` holds; nothing here substitutes for that UAT pass.
- **The three tilde-path bug fixes are load-bearing for Phase 6-8**: any future plan touching `internal/identity/inventory.go`'s classification or `cmd/gitid/wiring.go`'s delete-plan machinery should be aware `b.normalizedAccounts()` is now the ONE sanctioned way to get cross-identity-comparable accounts — a new `Accounts:` closure built from bare `b.accounts()` would silently reintroduce bug #3.
- **No blockers identified** for Phase 6 (Global SSH Options).

---
*Phase: 05-identity-manager*
*Completed: 2026-08-26*

## Self-Check: PASSED

All files listed above confirmed present on disk (`git show f68e8d0 --stat` / `git show ced59e8 --stat`); both commit hashes confirmed present in `git log`; the full required battery (`make gate-visual-regression && make test && make lint && make test-e2e && make gate-no-backend-files`) confirmed green in this session.
