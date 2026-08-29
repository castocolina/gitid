---
phase: 09-upload-credentials-assist
plan: 06
subsystem: upload-credentials-assist
tags: [tui, identity-manager, key-ceremony, delete, remote-destructive]
requires:
  - phase: 09-upload-credentials-assist
    plan: 05
    provides: the one upload orchestration (upload_run.go's runUploadFor) both the TUI and CLI call
provides:
  - The register-key modal (D-08) — a real identPane, reachable from the action menu's derived fifth row and the `u` shortcut, that resolves upload eligibility asynchronously and runs the upload beat with no further keystroke
  - actionMenuRows replaced by len(actionMenuLabels()) everywhere (09-RESEARCH.md Pitfall 3 closed structurally)
  - renderUploadSection (renamed from renderUploadRun) as the ONE upload-section renderer, now with three call sites — the create wizard, the register-key pane, and the rotate/repair key-ceremony's own upload beat
  - The same-key-clone/new-key-clone split in wizardModel.checkUploadEligibility
  - D-04's interactive old-key delete offer on the rotate result screen: warning-tier, defaults to leave, resolves the target by a fresh machine-scoped exact title match, retains the confirmed target across a failed delete for one retry, and never renders after a repair
affects: [09-07, 09-08]
actuals:
  tasks: 3
  commits: 3
tech-stack:
  added: []
  patterns:
    - "Async-message-chaining instead of tea.Batch for a message handler that needs to dispatch two dependent provider probes: the delete-offer probe is chained off the upload beat's own UploadRunMsg handler rather than batched alongside it, because this test suite's harness (and the plan's own acceptance tests) drive async cmds by calling them synchronously and feeding the result back through Update — a batched cmd would deliver an unroutable BatchMsg instead of the expected typed message."
    - "A concurrent-overlay ceremony beat: the upload/delete-offer beats never gate or replace the ceremony's own result screen — they render ALONGSIDE it (uploadRunHasContent/rotateDeleteOffer.Available gates on top of the always-rendered m.keyCeremony.view() body), so 'upload/offer never gates' is a structural property of the render function, not a behavioral rule enforced by tests alone."
key-files:
  created: []
  modified:
    - internal/tuikit/identities.go
    - internal/tuikit/identity_manager_upload_test.go
    - internal/tuikit/backend.go
    - internal/tuikit/backend_stub_test.go
    - internal/tuikit/views.go
    - internal/tuikit/upload_section_test.go
    - internal/dummytui/fixturebackend.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go
    - cmd/gitid/upload_run.go
    - e2e/identity_manager_pty_e2e_test.go
key-decisions:
  - "renderUploadRun was renamed to renderUploadSection (Task 1, its own small commit) to satisfy the plan's literal grep-count acceptance criterion — the plan names this exact identifier as the shared component's contract, and Task 2 needed to add its call site under that name, not a differently-named function that happens to do the same thing."
  - "Task 2's 'insert into the phase chain, positioned after the new key material exists' is satisfied literally: rotate/repair generate and commit the new key ATOMICALLY (no pre-commit staging like the create wizard has), so the upload beat can only genuinely run post-commit. A new keyCeremonyPhase value ('upload') tracks this in-flight window as real state, but the ceremony's already-visible result screen is never blocked by it — resolved by reading the ground truth in lifecycle.go/CommitRotate/CommitNewKey rather than assuming the wizard's staging model applies here too."
  - "The D-04 delete-offer probe is dispatched by CHAINING off the upload beat's UploadRunMsg handler (not tea.Batch'd alongside the upload dispatch) — this repo's own test harness has no live Bubble Tea event loop to unwrap a BatchMsg, and batching would have made the offer probe's message unroutable in every test that drives it by calling the returned cmd() directly. Sequential dispatch (offer runs right after upload resolves) satisfies every acceptance criterion just as well as true concurrency would."
  - "rotateDeleteOfferFor's body (the ONE uploader.Inventory call this beat makes) was placed in upload_run.go, not wiring.go, to satisfy the pre-existing R3 AST guard tests (TestRunUploadForIsTheOnlyOrchestration, TestRunUploadDoesNotCallProviderCommandsOutsideATeaCmd) that forbid wiring.go from calling uploader.Inventory/UploadKeys directly — discovered by running the full suite after the first RotateDeleteOffer draft and seeing both guards fail; fixed by moving the whole decision body, not by weakening the guard."
  - "RotateDeleteOfferView gained IdentityName/MachineName fields (beyond the plan's literal field list) so RotateDeleteOfferBodyFmt's two %s verbs (identity name, machine name) can interpolate correctly — the plan's own KeyTitle field is the ALREADY-ASSEMBLED 'gitid: name @ host' string, which cannot be re-split back into the format's two slots without fragile parsing."
patterns-established:
  - "A ceremony sub-beat that must intercept plain keys (arrows/tab/enter) before they reach the shared ceremonyModel's own handleKey — because the ceremony is already `done` (post-commit) and its own handleKey treats every key but Enter as inert, Enter as closing — must explicitly guard and return before the ceremonyModel delegation, restoring the fallthrough once the sub-beat resolves."
requirements-completed: [UP-01, UP-03]
coverage:
  - id: D1
    description: The register-key modal exists as a real identPane matching sibling-pane geometry, reachable from the derived fifth action-menu row and the u key; opening it resolves eligibility asynchronously and runs the upload beat with no further keystroke.
    requirement: UP-01
    verification:
      - kind: unit
        ref: internal/tuikit/identity_manager_upload_test.go
        status: pass
      - kind: e2e
        ref: e2e/identity_manager_pty_e2e_test.go (TestIdentityManager_ActionMenu, manifest backstop)
        status: pass
    human_judgment: false
  - id: D2
    description: Rotate and repair run the same upload section through the same renderer (renderUploadSection) and the same backend seam (RunUploadForIdentity), at the point the new key material genuinely exists; every upload outcome lets the ceremony continue unchanged.
    requirement: UP-01
    verification:
      - kind: unit
        ref: internal/tuikit/identity_manager_upload_test.go (TestRotateCeremonyRunsTheUploadBeatAfterCommitSucceeds, TestRepairCeremonyRunsTheUploadBeat, TestKeyCeremonyUploadFailureStillAdvances, TestKeyCeremonyUploadUsesTheSharedRenderer)
        status: pass
    human_judgment: false
  - id: D3
    description: actionMenuRows is derived from len(actionMenuLabels()) everywhere; no hand-maintained row-count constant remains.
    requirement: UP-01
    verification:
      - kind: unit
        ref: "grep -rn 'actionMenuRows' internal/tuikit/ (zero occurrences)"
        status: pass
    human_judgment: false
  - id: D4
    description: A same-key clone omits the upload section entirely (nothing new to upload); a new-key clone runs the same probe the create wizard always has.
    requirement: UP-01
    verification:
      - kind: unit
        ref: internal/tuikit/identity_manager_upload_test.go (TestSameKeyCloneOmitsTheUploadSection, TestNewKeyCloneRunsTheUploadSection)
        status: pass
    human_judgment: false
  - id: D5
    description: "D-04's delete offer: defaults to leave, requires an explicit move to the delete option plus Enter (never a single default keystroke), resolves the old key's provider ID by a FRESH machine-scoped exact title match at result-screen time, deletes exactly the confirmed ID, retains that confirmed target across a failed delete for one retry, discards it on leaving the result screen, never renders after a repair, and degrades to the existing frozen grace-window hint whenever unavailable."
    requirement: UP-03
    verification:
      - kind: unit
        ref: internal/tuikit/identity_manager_upload_test.go (TestRotateDeleteOffer*, TestRotateDeleteFailureRetriesTheSameConfirmedTarget, TestRotateDeleteConfirmedTargetIsDiscardedOnLeavingTheScreen)
        status: pass
      - kind: unit
        ref: cmd/gitid/wiring_test.go (TestRotateDeleteOfferMatchesOnlyThisMachinesTitle, TestRotateDeleteOfferReadsInventoryFreshAtResultTime, TestCommitRotateDeleteOldKeyDeletesExactlyTheConfirmedID)
        status: pass
    human_judgment: false
  - id: D6
    description: No Backend method in this plan performs provider I/O synchronously (R3); the register-key plan, the upload beat, and the delete offer are all asynchronous commands.
    requirement: UP-01
    verification:
      - kind: unit
        ref: "cmd/gitid/upload_run_test.go / wiring_test.go's existing R3 AST guards (extended to cover the new rotateDeleteOfferFor relocation)"
        status: pass
    human_judgment: false
deviations:
  - "renderUploadRun was renamed to renderUploadSection mid-Task-1 as a small follow-up commit (not folded into Task 1's own commit) once the plan's literal acceptance grep was re-read carefully after the first pass had already used the pre-existing (Wave 4) name."
  - "The delete-offer probe (RotateDeleteOffer) is dispatched sequentially, chained off the upload beat's completion, rather than batched (tea.Batch) alongside the upload dispatch as a first draft attempted — batching produced an unroutable tea.BatchMsg in this test suite's harness, which has no live Bubble Tea event loop to unwrap it. This is a deviation from a natural first reading of 'independent async beats' as necessarily concurrent; sequential dispatch satisfies every plan acceptance criterion (both are still asynchronous, neither still gates the ceremony) without the harness incompatibility."
  - "RotateDeleteOfferView gained two fields (IdentityName, MachineName) beyond the plan's literal field list, needed to correctly interpolate RotateDeleteOfferBodyFmt's two %s verbs without re-parsing the already-assembled KeyTitle string."
---

## What this plan built

Three deliverables, matching 09-06-PLAN.md exactly in intent:

1. **The register-key modal (D-08).** A new `paneRegisterKey` `identPane`, reachable from the
   action menu's now-derived fifth row (`IdentityManagerActionRegisterKey`) and the `u` key on the
   detail pane. Opening it dispatches `Backend.RegisterKeyPlan(name)` (async, R3); a ready answer
   immediately dispatches `Backend.RunUploadForIdentity(name)` with no further keystroke (D-02: opening
   the pane IS the opt-in). Non-ready states fall back to the existing manual-instructions text; a
   plan error fails closed. The pane mirrors `paneClone`'s exact geometry, footer, and status shape.

2. **The key-ceremony's own upload beat — the shared component's third call site.** After
   `CommitRotate`/`CommitNewKey` succeeds — the first point at which the new key material genuinely
   exists, since rotate/repair generate and commit atomically with no pre-commit staging — the
   ceremony dispatches `RunUploadForIdentity` (the SAME seam Task 1 added, no fourth seam) and renders
   the result through `renderUploadSection` (renamed from `renderUploadRun` so the shared-component
   name matches the plan's own contract). A new `"upload"` `keyCeremonyPhase` value tracks the
   in-flight beat as real state, but the ceremony's already-visible result screen (`commitSucceeded`,
   already rendered by the time upload dispatches) is never blocked by it. A same-key clone's
   `wizardModel.checkUploadEligibility` explicitly short-circuits (nothing new to upload); a new-key
   clone runs the wizard's existing probe unchanged.

3. **D-04's interactive old-key delete offer — the phase's one remotely-destructive action.** After a
   rotate whose upload beat has resolved, the ceremony chains a dispatch of `Backend.RotateDeleteOffer`
   (a fresh, uncached provider inventory read matched by `uploader.KeyTitle` + `uploader.FindByTitle`
   exact-equality against THIS machine's title — D-07). An available offer renders a warning-tier
   (never destructive-red/typed-confirm) two-option choice row defaulting to "leave," intercepted
   ahead of the shared `ceremonyModel`'s own key handling so the offer owns plain keys until resolved.
   Confirming delete dispatches `Backend.CommitRotateDeleteOldKey(name, id)` with the EXACT reviewed
   ID — never re-resolved. A failed delete retains the confirmed `{ID, title}` pair so a retry reuses
   it with zero intervening inventory reads (review R12); leaving the result screen discards it.
   Repair never gets an offer. Every unavailable/pending/repair path falls back to the existing frozen
   `keyCeremonyGraceHintFmt` sentence, unchanged.

## Verification

- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — 2314 tests pass.
- `go test -tags e2e -race -timeout 900s ./e2e/...` — passes (including the FIELDS.md manifest-backstop
  fix for `action_register_key`, discovered mid-Task-1).
- `make lint` — 0 issues.
- `make gate-copy-freeze` — passes; `keyCeremonyGraceHintFmt`, `keyCeremonyArchiveFmt`,
  `keyCeremonyNotTestedDetail` byte-unchanged.
- `grep -rn 'actionMenuRows' internal/tuikit/'` — zero occurrences.
- `grep -c 'renderUploadSection' internal/tuikit/identities.go` — 6 (one definition, five call sites
  across the wizard, the register-key pane, and the key-ceremony beat).

## Environment note carried forward

The development machine's system Go was upgraded to 1.27.0 mid-session; golangci-lint 2.12.2 (built
with go1.26.2) cannot parse 1.27's stdlib for its own type-checking pass. `go build`/`go test` are
unaffected (GOTOOLCHAIN=auto fetches go1.26.4 to satisfy go.mod's `go 1.26` directive transparently),
but `golangci-lint` must be invoked with `GOTOOLCHAIN=go1.26.4` explicitly set in the environment for
every lint run in this wave and any that follow, until the toolchain drift is resolved.
