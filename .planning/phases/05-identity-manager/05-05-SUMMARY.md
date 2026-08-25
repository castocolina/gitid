---
phase: 05-identity-manager
plan: 05
subsystem: identity-lifecycle-clone
tags: [clone, create-wizard, ssh-config, gitconfig, review-flag, pattern-matching]

requires:
  - phase: 05-identity-manager
    provides: "05-01's create-flow backend (Deps/CreateInput/runPipeline), 05-03's Rotate/RepairKey ceremony pattern this plan's D-16 gate proof reuses (specFingerprint, storeUnlockedFor), and Phase 3's create wizard (wizardModel/sshForm/gitForm) this plan's D-15 pre-fill reuses unchanged"
provides:
  - "identity.SuggestCloneName / DeriveCloneInput / CloneNotices / GitdirMatch / HasconfigMatch — the D-14/D-17 re-derivation core (Task 1, already committed at 082571e before this dispatch)"
  - "tuikit.ClonePrefillView — the pre-fill DTO carrying copied-field markers into the create wizard (Task 2)"
  - "tuikit.Backend.SuggestCloneName / .ClonePrefill — new seams on all three Backends (stub, dummy, real)"
  - "tuikit.newWizardPrefilled — the shared-construction wizard entry point clone (and D-02's future CLI flag pre-fill) both need"
  - "sshconfig.MatchingHostStanzas — the OpenSSH-pattern-semantics (never hand-rolled) shadowing-check data source (review R-29)"
  - "cmd/gitid/wiring.go realBackend.ClonePrefill/SuggestCloneName — the one backend-to-view conversion site, plus the availability/shadowing two-check composition"
affects: [05-06-identity-planner, 05-07-full-delete-lifecycle]

actuals:
  tokens: 5651
  tasks: 2
  commits: 1

tech-stack:
  added: []
  patterns:
    - "Pre-fill, never a second write pipeline (D-15): newWizardPrefilled shares newWizardBase with newWizard — the ONE wizard constructor two entry points apply values onto, so a pre-filled wizard inherits the collision gate, step navigation, and ceremony byte-for-byte."
    - "Two DISTINCT checks for one collision question (review R-29): availability (literal taken-name scan, silently bumped by SuggestCloneName) and pattern shadowing (the derived ALIAS checked against every parsed Host pattern via REAL OpenSSH semantics, sshconfig.MatchingHostStanzas / *Host.Matches — never string-equality or a hand-rolled globber) are two separate functions with two separate outcomes (silent bump vs typed refusal), proven distinct by a dedicated test."
    - "Zero-row inline flag (D-14): formFieldLineFlagged appends the review flag as a trailing suffix on the SAME row the field already occupies, never a new row — the 05-UI-SPEC.md Spacing table's explicit 0-row budget for this addition, verified by a line-count-equality test against an unflagged render."
    - "D-16's gate holds by construction, not by a new check: newWizardPrefilled inherits newWizardBase's idle testPhase (the same shared construction that makes D-15 hold), and specFingerprint already binds Name+Alias — Task 3 required zero production code, only tests confirming both guarantees."

key-files:
  created: []
  modified:
    - internal/tuikit/views.go
    - internal/tuikit/backend.go
    - internal/tuikit/identities.go
    - internal/tuikit/identities_test.go
    - internal/tuikit/backend_stub_test.go
    - internal/tuikit/batch3_test.go
    - internal/dummytui/fixturebackend.go
    - internal/sshconfig/reader.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go

key-decisions:
  - "ClonePrefillView's AliasPrefix field carries the identity/prefix half only (matching the wizard's own 'Alias prefix' field label); the full SSH Host alias is reconstructed inside newWizardPrefilled via a new tuikit-local providerFromHostname inverse-lookup over the three known alt-SSH pairings (ssh.github.com/altssh.gitlab.com/altssh.bitbucket.org), rather than adding a fourth DTO field the plan's literal field list didn't name — an unknown provider falls back to itself, matching the project's existing 'unknown provider keeps itself' convention."
  - "The clone-name prompt's Enter branch always calls ClonePrefill with reuseSourceKey=true (the pre-fill defaults to reusing the source key); the wizard's OWN existing D-10 generate-vs-reuse picker is what lets the user switch to a fresh key afterward — the clone prompt itself gained no new key-choice control, per D-15's 'no second pipeline, no new screen'."
  - "sshconfig.MatchingHostStanzas is Include-unaware (the same single-file scope AllHostStanzas already uses) rather than replicating AliasCollision's recursive Include-walk — sufficient for the D-17/R-29 shadowing check's scope and consistent with the existing D-09 hand-written-alias data source precedent, at the cost of not following gitid-managed config.d Include chains for THIS specific check (acceptable: the availability check via b.accounts() already covers every gitid-managed identity regardless of layout)."

requirements-completed: [MGR-04]

coverage:
  - id: D7
    description: "Clone opens the EXISTING create wizard pre-filled from the source (D-15) — no second write pipeline, no new screen; the SSH form's alias prefix/hostname/port and the Git form's name/email hold the derived/copied pre-fill values"
    requirement: "MGR-04"
    verification:
      - kind: unit
        ref: "internal/tuikit/identities_test.go#TestCloneOpensPrefilledWizard"
        status: pass
      - kind: unit
        ref: "internal/tuikit/identities_test.go#TestCloneEnterEmitsNoCloneIdentityAction"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestClonePrefill_CopiesAuthorFieldsAndRederivesEverythingElse"
        status: pass
    human_judgment: false
  - id: D8
    description: "The D-14 'copied from <source> — review' flag renders on exactly the Git step's user.name/user.email rows, costs zero extra rows, and never appears on any SSH, gitdir, or strategy row"
    requirement: "MGR-04"
    verification:
      - kind: unit
        ref: "internal/tuikit/identities_test.go#TestCloneReviewFlagOnAuthorRowsOnly"
        status: pass
      - kind: unit
        ref: "internal/tuikit/identities_test.go#TestCloneReviewFlagNeverOnSSHOrStrategyRows"
        status: pass
    human_judgment: false
  - id: D9
    description: "D-17/R-29: the suggested clone name is silently auto-bumped past every LITERAL taken alias, while a WILDCARD Host pattern that shadows the derived alias is caught as a typed, non-bumpable error naming the pattern — two distinct checks with two distinct outcomes"
    requirement: "MGR-04"
    verification:
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestSuggestCloneName_BumpsPastLiteralHostAlias"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestClonePrefill_WildcardPatternShadowingReturnsTypedError"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestSuggestCloneNameAndShadowing_AreDistinctCodePaths"
        status: pass
    human_judgment: false
  - id: D10
    description: "D-16: a same-key clone re-runs the FULL two-stage test gate — the pre-filled wizard's initial test phase equals a fresh wizard's, specFingerprint distinguishes two clones of one source by name AND alias, and an accepted outcome recorded for the source specification does not unlock the clone's write"
    requirement: "MGR-04"
    verification:
      - kind: unit
        ref: "internal/tuikit/identities_test.go#TestClonePrefilledWizardTestPhaseMatchesFreshWizard"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestSpecFingerprint_DistinguishesByIdentityNameAndAlias"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestCloneStoreGate_LockedUntilBothStagesAccepted"
        status: pass
      - kind: unit
        ref: "cmd/gitid/wiring_test.go#TestCloneStoreGate_SourceOutcomeDoesNotUnlockClone"
        status: pass
    human_judgment: false

duration: ~95min
completed: 2026-08-25
status: complete
---

# Phase 5 Plan 05: Clone Derivation, Pre-Filled Wizard Entry, and the D-16 Two-Stage Gate Summary

**Cloning an identity now opens the existing create wizard pre-filled — copying only `user.name`/`user.email` with a zero-extra-row "copied from" flag, re-deriving every SSH/Git field via kind-specific constructors, silently bumping the suggested name past literal collisions while a typed error catches wildcard Host-pattern shadowing (review R-29), and re-running the full two-stage connectivity gate even when the clone reuses the source's key.**

## Performance

- **Duration:** ~95 min (Task 2 + Task 3; Task 1 — clone derivation — was already implemented and committed at `082571e` before this dispatch)
- **Tasks:** 2 of this plan's 3 (Task 1 pre-existing; Task 2 and Task 3 executed in this session)
- **Files modified:** 10 (0 new — every change extends an existing file)

## Context

This plan was previously attempted three times via the project's cross-AI (opencode local-model) delegation path; all three failed to produce a Task 2/3 commit (one deadlocked on a headless permission prompt, two exhausted their time budget in pure exploration). Per the project's documented cross-AI-failure fallback rule, this dispatch is a direct on-session executor picking up from Task 1's already-committed clone-derivation core (`internal/identity/clone.go`, committed `082571e`) and completing Task 2 (pre-filled wizard entry + review flag) and Task 3 (D-16 gate confirmation).

## Accomplishments

- **Clone routes into the EXISTING create wizard, pre-filled — no second write pipeline (D-15).** `internal/tuikit/identities.go`'s `newWizard` was decomposed into a shared `newWizardBase` two constructors build on: `newWizard` (the fresh-create defaults) and the new `newWizardPrefilled(b, pre ClonePrefillView)` (the clone entry). `handleCloneKey`'s Enter branch now calls `Backend.ClonePrefill` and, on success, opens the wizard pre-filled instead of dispatching a `CloneIdentity` action — proven at the handler level by `TestCloneEnterEmitsNoCloneIdentityAction`, which asserts the returned `keyResult.actions` never contains one. `tuikit.CloneIdentity` and its `Reduce` case in `store.go` are deliberately left untouched (the dummy fixture's frozen behavior is unchanged); only the real binary's emission path moved.
- **`tuikit.ClonePrefillView` is the one pre-fill DTO all three Backends speak.** `views.go` defines it carrying `SourceName`/`CloneName`/`AliasPrefix`/`Hostname`/`Port`/`GitName`/`GitEmail`/`MatchStrategy`/`GitDir`/`ReuseKeyPath`/`CopiedFields` — plain Go types only, upholding the Backend file-header rule. `backend.go` adds `SuggestCloneName(source string) string` and `ClonePrefill(source, cloneName string, reuseSourceKey bool) (ClonePrefillView, error)` to the interface. All three implementers exist: `stubBackend` (test-only, `backend_stub_test.go`), `FixtureBackend` (the design demo, `dummytui/fixturebackend.go`), and `realBackend` (`cmd/gitid/wiring.go`, wrapping `identity.SuggestCloneName`/`identity.DeriveCloneInput` from Task 1). `TestNoBackendAllowlist` confirms the new methods leaked no backend type into `internal/tuikit`'s import graph, and `go list -deps ./internal/tuikit` shows zero first-party backend imports.
- **The D-14 review flag costs zero extra rows and lands on exactly two fields.** `formFieldLineFlagged` extends `formFieldLine` with an optional trailing suffix, wired only into the Git step's `user.name`/`user.email` rows (`gitForm.view`) via `gitForm.copiedFields`/`sourceName` and a `fieldCopied` membership check — every other field (SSH, gitdir, strategy, key rows) keeps calling the unflagged renderer, so the flag can never leak. `TestCloneReviewFlagOnAuthorRowsOnly` proves the flagged and unflagged renders have the IDENTICAL line count (the 05-UI-SPEC.md Spacing table's explicit 0-row budget), and `TestCloneReviewFlagNeverOnSSHOrStrategyRows` is the negative proof over the raw per-row rendered output (not the whitespace-collapsing `paneFlat` test helper, which would make a per-row adjacency check meaningless).
- **Two DISTINCT checks close review R-29's collision gap.** `cmd/gitid/wiring.go`'s `takenCloneNames()` builds the D-17 availability list from `b.accounts()` UNION every LITERAL (non-wildcard, non-negated) parsed Host pattern's derived name, feeding `identity.SuggestCloneName`'s silent bump. Separately, `ClonePrefill` checks the DERIVED ALIAS against every parsed Host pattern using REAL OpenSSH pattern semantics via the new `sshconfig.MatchingHostStanzas` (backed by `kevinburke/ssh_config`'s own `*Host.Matches` — never a hand-rolled globber): a wildcard match (e.g. `Host *.github.com`) returns a typed `*identity.FieldError` naming the shadowing pattern, refusing rather than silently substituting a different name. `TestSuggestCloneNameAndShadowing_AreDistinctCodePaths` seeds one hermetic home with both a literal collision (bumps) and, added afterward, a wildcard shadow (refuses the SAME bumped name) — proving the two checks are genuinely separate code paths with separate outcomes.
- **D-16 required zero production code — only confirming tests (Task 3).** `specFingerprint` already incorporates `Name` and `Alias` (not only the key path), so two clones of one source already get distinct fingerprints; `newWizardPrefilled` inherits `newWizardBase`'s idle `testPhase` exactly like `newWizard` does, so entering pre-filled never skips the gate. Four new tests pin both guarantees structurally: `TestClonePrefilledWizardTestPhaseMatchesFreshWizard` (tuikit), `TestSpecFingerprint_DistinguishesByIdentityNameAndAlias`, `TestCloneStoreGate_LockedUntilBothStagesAccepted`, and `TestCloneStoreGate_SourceOutcomeDoesNotUnlockClone` (wiring) — the last of which drives `storeUnlockedFor` directly to prove an accepted outcome recorded against the SOURCE's fingerprint never unlocks the CLONE's write.
- **Two pre-existing tests were updated for the behavior change, not deleted.** `batch3_test.go`'s `TestMouseCloneButtonClones` and `TestCloneFocusRingInputToButton` asserted the pre-05-05 immediate-clone flow (clicking/Entering the Clone control created the identity synchronously); both now assert the D-15 outcome (pane becomes `paneCreate`, the wizard's alias prefix holds the suggested name, and — for the mouse test — the identity count stays at 8, proving no write happened yet).

## Task Commits

1. **Task 1: Clone derivation — copy the copyable, re-derive the rest (D-14, D-17)** — `082571e` (feat) — already committed before this dispatch; not re-done.
2. **Task 2 + Task 3: Pre-filled wizard entry, the two-field review flag, and the D-16 gate confirmation (D-14, D-15, D-16, D-17, R-29)** — `22bdc2a` (feat)

Per CLAUDE.md's commit-grouping rule ("one coherent change... is a single commit... let the buildable boundary, not file count, set the commit granularity"), Task 2 and Task 3 were committed together: Task 3 required **zero production code changes** — it is purely confirmation testing of guarantees Task 2's own implementation (and prior plans' `specFingerprint`) already provides, sharing the exact same files (`identities.go`, `identities_test.go`, `wiring.go`, `wiring_test.go`) with no clean implementation boundary between them. Splitting identical-file, no-separate-implementation tests into two artificial commits would be exactly the "small per-step chunks" CLAUDE.md's own rule forbids.

## Files Created/Modified

- `internal/tuikit/views.go` — `ClonePrefillView` (new pre-fill DTO).
- `internal/tuikit/backend.go` — `Backend.SuggestCloneName`/`.ClonePrefill` (new interface methods).
- `internal/tuikit/identities.go` — `newWizardBase` (shared construction), `newWizardPrefilled`, `providerFromHostname`, `applyReuseKey`, `formFieldLineFlagged`, `copiedFieldFlag`/`copiedFieldFlagTemplate`, `copiedFieldGitName`/`copiedFieldGitEmail`; `gitForm` gains `copiedFields`/`sourceName`/`fieldCopied`; `gitForm.view` routes the name/email rows through the flagged renderer; `handleDetailKey`'s `"c"` case seeds from `backend.SuggestCloneName`; `handleCloneKey`'s Enter branch calls `backend.ClonePrefill` and opens the pre-filled wizard; `identitiesModel` gains `cloneErr`; the `paneClone` render block shows `cloneErr` inline.
- `internal/tuikit/identities_test.go` — `TestCloneOpensPrefilledWizard`, `TestCloneEscapeReturnsToDetailWithoutOpeningWizard`, `TestCloneEnterEmitsNoCloneIdentityAction`, `TestCloneReviewFlagOnAuthorRowsOnly`, `TestCloneReviewFlagNeverOnSSHOrStrategyRows`, `clonedWizardAtGitStep` helper, `TestClonePrefilledWizardTestPhaseMatchesFreshWizard`, `TestCloneStageCommandsNameTheCloneAlias`; the old `TestCloneValidatesAndSelectsTheClone` was replaced by `TestCloneOpensPrefilledWizard`.
- `internal/tuikit/backend_stub_test.go` — `stubBackend.SuggestCloneName`/`.ClonePrefill`, `stubNameTaken`, `findStubRow`.
- `internal/tuikit/batch3_test.go` — `TestMouseCloneButtonClones`/`TestCloneFocusRingInputToButton` updated for the pre-filled-wizard outcome.
- `internal/dummytui/fixturebackend.go` — `FixtureBackend.SuggestCloneName`/`.ClonePrefill`, `fixtureNameTaken`, `findFixtureRow`.
- `internal/sshconfig/reader.go` — `MatchingHostStanzas` (real-OpenSSH-semantics pattern-shadowing data source).
- `cmd/gitid/wiring.go` — `takenCloneNames`, `nameFromAlias`, `matchStrategyFromMatches`, `gitDirFromMatches`, `realBackend.SuggestCloneName`/`.ClonePrefill`.
- `cmd/gitid/wiring_test.go` — `seedCloneSourceFixture` helper (renders a marker-carrying Host block via `sshconfig.RenderHostBlock` so `Reconstruct` resolves the FULL FQDN provider, needed for the alias-shape assertions); `TestClonePrefill_CopiesAuthorFieldsAndRederivesEverythingElse`, `TestSuggestCloneName_BumpsPastLiteralHostAlias`, `TestClonePrefill_WildcardPatternShadowingReturnsTypedError`, `TestSuggestCloneNameAndShadowing_AreDistinctCodePaths`, `TestSpecFingerprint_DistinguishesByIdentityNameAndAlias`, `TestCloneStoreGate_LockedUntilBothStagesAccepted`, `TestCloneStoreGate_SourceOutcomeDoesNotUnlockClone`, `TestCloneStageCommandsNameCloneAlias_RealBackend`.

## Decisions Made

See `key-decisions` in the frontmatter. The most consequential: `ClonePrefillView.AliasPrefix` deliberately carries only the identity/prefix half of the alias (matching the wizard's own field label), with the full SSH Host alias reconstructed inside `newWizardPrefilled` via a small provider-inverse-lookup — this kept the DTO to the plan's literally-named field list while still producing a correct, provider-aware alias for GitHub/GitLab/Bitbucket clones.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Two pre-existing tests asserted the OLD immediate-clone behavior and would have failed against the new D-15 flow**
- **Found during:** Task 2, first `go test ./internal/tuikit/...` run after wiring `handleCloneKey`'s new Enter branch
- **Issue:** `batch3_test.go`'s `TestMouseCloneButtonClones` and `TestCloneFocusRingInputToButton` (written before this plan) asserted that clicking/Entering the Clone control created the identity synchronously (`selected == "personal-clone"`, `len(a.state.Identities) == 9`). Task 2's whole point is that clone no longer writes anything until the pre-filled wizard's own commit — these assertions are now describing the SUPERSEDED behavior, not a regression.
- **Fix:** Rewrote both tests to assert the D-15 outcome: the pane becomes `paneCreate`, the wizard's alias prefix holds the suggested name, and (mouse test) the identity count stays at 8 — proving no write happened yet.
- **Files modified:** `internal/tuikit/batch3_test.go`
- **Verification:** `go test ./internal/tuikit/... -run Clone` — all pass; full `go test -race ./...` green.
- **Committed in:** `22bdc2a`

**2. [Rule 1 - Bug] `sshconfig.MatchingHostStanzas` test fixtures needed a real provider marker to exercise the FQDN alias shape**
- **Found during:** Task 2, writing `TestClonePrefill_WildcardPatternShadowingReturnsTypedError`
- **Issue:** The existing `seedDeleteFixture` test helper hand-writes an SSH Host block WITHOUT the `# gitid: provider=` marker comment (a legacy-identity simulation other delete tests intentionally use). `identity.Reconstruct`'s no-marker fallback (`hostnameToProvider`) resolves to the SHORT provider form (`"github"`, no dot) rather than the FQDN (`"github.com"`), so `identity.DefaultAlias` produced `"personal-clone.github"` instead of `"personal-clone.github.com"` — a fixture artifact, not a product bug, but one that broke the wildcard-pattern-match assertion since `*.github.com` never matches `.github` (no dot).
- **Fix:** Added `seedCloneSourceFixture`, a sibling fixture helper that renders the Host block through `sshconfig.RenderHostBlock` with an explicit `provider="github.com"` argument, so the block carries the real marker comment and `Reconstruct` resolves the FQDN provider — used only by the two tests whose assertions depend on the alias's exact shape.
- **Files modified:** `cmd/gitid/wiring_test.go`
- **Verification:** `TestClonePrefill_WildcardPatternShadowingReturnsTypedError` and `TestSuggestCloneNameAndShadowing_AreDistinctCodePaths` pass.
- **Committed in:** `22bdc2a`

---

**Total deviations:** 2 auto-fixed (both Rule 1 — fixing tests to match the intended new behavior / a fixture gap the new assertions exposed). No scope creep — both are exactly what proving the plan's own acceptance criteria required.

## Issues Encountered

None beyond the two deviations above. Every task's `<verify>` command passed after implementation. `make lint` (golangci-lint 2.12.2 + gosec) required one `make fmt` pass (a single `goimports` formatting fix on `identities.go`) before reaching 0 issues — resolved before committing, never bypassed with `--no-verify`. The full module suite (`go test -race ./...`, 1283 tests across 19 packages), `make test` (including the `-tags screenshot` half and the copy-freeze gate), and `make test-e2e` (the real-PTY e2e suite) are all green.

## Known Stubs

None. Every deliverable this plan's Task 2/Task 3 promise is fully implemented and tested through both the stub/dummy fixture Backends and the real `cmd/gitid` composition root.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `tuikit.newWizardPrefilled` is ready for D-02's future CLI-flag pre-fill (a second `ClonePrefillView`-shaped caller) to reuse without a second wizard constructor.
- `sshconfig.MatchingHostStanzas` is ready for any future feature needing real-OpenSSH-pattern-semantics collision checks (e.g. a future rename/repair ceremony) without re-deriving pattern matching a third time.
- Plan 05-06 (the identity planner / clone menu entry) can wire `handleDetailKey`'s `"c"` case into its own menu shape unchanged — the `ClonePrefill`/`SuggestCloneName` seams are stable.
- No blockers identified for plans 05-06/05-07.

---
*Phase: 05-identity-manager*
*Completed: 2026-08-25*
