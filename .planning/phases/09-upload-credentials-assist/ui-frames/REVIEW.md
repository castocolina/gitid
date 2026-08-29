# Phase 9 UX Review Packet — Upload & Credentials Assist

09-07-PLAN.md Task 3 (DLV-04/DLV-06). This packet closes the phase's
real-vs-dummy comparison and per-Focal-point judgement obligations.

## Parity critique — PENDING

The `agent-ui-ux-designer` critique (D-09 step 3, review R15) is an
orchestrator/human-initiated step and cannot be summoned from inside this
autonomous execution wave. **No critique has been run.** This section
records that fact explicitly rather than fabricating one — per R15's own
instruction, a fabricated review packet is worse than a missing one, because
the missing one is visible.

D-09's parity-critique obligation stays **OPEN**. To close it, a
human/interactive session must run the `agent-ui-ux-designer` subagent
against the committed frames below (`.planning/phases/09-upload-credentials-assist/ui-frames/*.txt`)
compared to `09-UI-SPEC.md`, and fold its findings into this section with the
same improvement-or-defect classification the rest of this document uses.

## Per-screen classified differences

Every real-vs-dummy difference this phase's paired PTY comparisons and the
in-process visual gate surfaced is classified in
`.planning/design/create-flow/visual-divergence-allowlist.txt` and
`.planning/design/identity-manager/visual-divergence-allowlist.txt` (Phase 9
/ UP-4 rows). Summarized here per screen:

| Screen | Real frame | Dummy comparator | Classified differences | Class |
|---|---|---|---|---|
| upload-checkbox-ready / upload-results | `create-flow-upload-autonomous-github.txt`, `create-flow-upload-partial-scope.txt`, `create-flow-upload-already-complete.txt` | in-process `CaptureUploadScreens` (dummy backend) / paired PTY dummy session | sidebar (fixture-vs-live identity set), header-status (identity count), upload-section + connectivity-output (real temp-dir key path & PATH-resolved gh vs dummy's frozen `/usr/local/bin/gh ...` shape) | fixture-vs-live artifact (UP-4), not a defect |
| upload-checkbox-unauth | `create-flow-upload-checkbox-unauth.txt` | non-comparable (live-only; captured for both backends via `uploadProbeBackend`, held non-comparable — see `uploadVisualSpecs`' doc comment) | n/a | non-applicable, documented |
| upload-checkbox-disabled | `create-flow-upload-checkbox-disabled.txt` | non-comparable, same reasoning | n/a | non-applicable, documented |
| upload-manual-fallback | `create-flow-upload-partial-scope.txt` (concrete instance of the manual-guidance concept) | non-comparable, same reasoning | n/a | non-applicable, documented |
| register-key-modal | `identity-manager-register-key-modal-runs.txt`, `-manual-fallback.txt`, `-u-key.txt` | in-process capture / paired PTY dummy session | sidebar, header-status, breadcrumb (identity-name embedding — PTY-level comparison never sees this: identity tokens are normalized away before comparing; the in-process gate's own unnormalized comparison is what needs it), upload-section, connectivity-output | fixture-vs-live artifact (UP-4), not a defect |
| rotate-delete-offer | `identity-manager-rotate-delete-offer-default.txt`, `-delete.txt`, `-absent.txt` | NOT capturable in the in-process gate (requires a real key-rotation commit); IS comparable through this task's own compiled real-vs-dummy PTY test | sidebar, header-status, detail (superset — deliberately skipped at PTY comparison level, redundant with upload-section/connectivity-output at finer granularity), upload-section, connectivity-output | fixture-vs-live artifact (UP-4), not a defect |
| rotate-result / repair-result (pre-existing checkpoints, now also carrying the chained upload beat since 09-06-PLAN.md Task 2) | pre-existing frames | paired PTY dummy session | NEW: upload-section, connectivity-output (this task's new region coverage surfaced them for the first time) | fixture-vs-live artifact (UP-4), not a defect — documented as a Task-3 finding, not a regression |

Zero differences are left unclassified: every diverging region on every
compared checkpoint has a matching allowlist row, verified by
`TestUploadVisualAllowlistMatchesRegistry` (in-code sync) and by
`TestRegisterKeyModal_CompiledRealVsLiveDummyPTY` /
`TestUploadSection_CompiledRealVsLiveDummyPTY`'s own "unused entry" checks
(no stale rows). Removing any row was demonstrated to make its owning test
fail (recorded in 09-07-SUMMARY.md).

## Per-Focal-point judgement (09-UI-SPEC.md)

| Focal point | Judgement | Evidencing frame |
|---|---|---|
| Step 0's checkbox row never wraps, disabled/hint reason inline | Satisfied — `TestCreateFlow_UploadCheckboxTabAndClickReachable`/`UnauthState`/`DisabledState` (09-07-PLAN.md Task 1) assert the row renders on one line in all three states | `create-flow-upload-checkbox-unauth.txt`, `create-flow-upload-checkbox-disabled.txt`, `create-flow-upload-checkbox-tab-and-click.txt` |
| `testUpload`'s `announcing` → `per-key-results` transition matches `renderStageOutcome`'s existing treatment, no same-frame flash | Satisfied — `renderUploadSection` reuses the same faint/command styling as the pre-existing stage rows (09-06-PLAN.md Task 1); the paired PTY test's own "Running:" wait (`mustSeeSlow`) proves the announce state is legible for a real, observable window before the result rows replace it | `create-flow-upload-autonomous-github.txt` |
| Manual-fallback block byte-identical to `internal/upload.Instructions(provider)` | Satisfied — `TestCreateFlow_UploadManualFallbackWhenUnauthenticated` (Task 1) asserts the GitHub instruction lines render verbatim; `gate-copy-freeze` independently freezes `UploadManualHeading` at the source level | `create-flow-upload-partial-scope.txt` |
| Rotate's delete-offer defaults to "No" (leave), never delete | Satisfied — `TestIdentityManager_RotateDeleteOfferDefaultsToLeave` (Task 1) and this task's `TestRegisterKeyModal_CompiledRealVsLiveDummyPTY/rotate-delete-offer` both assert the leave option is focused by default and Enter never issues a delete invocation | `identity-manager-rotate-delete-offer-default.txt` |
| Identity Manager's register-key modal matches the existing `paneClone`/`paneDeleteScope` pane convention | Satisfied — `paneRegisterKey` follows the identical `handleKey`/`view` switch-case shape (09-06-PLAN.md Task 1); no overlay-composition helper introduced | `identity-manager-register-key-modal-runs.txt` |

## Improvements carried to final review

No UX improvements were identified this task beyond the fixture-vs-live
artifacts already classified above (all of which are accepted-by-design
divergences between a live probe and a frozen fixture, not user-facing UX
changes). Per ONESHOT.md rule 11, any recorded improvement would be carried
to the user's single manual review after all phases complete rather than
resolved at this phase's close — none apply here.

## Defects found and fixed at this task's close

- **Rotate result screen offer-clipping (Task 1):** the rotate result
  screen's assembled body could silently clip the D-04 delete offer off the
  visible 30-row frame for a realistic rotate. Fixed in `renderKeyCeremony`
  (commit `8a7c678`) — the offer now renders before the upload beat's
  redundant announce lines in the tail, so any unavoidable clipping trims
  the less-critical content first.
- **rotate-result/repair-result missing upload-beat timing waits (this
  task):** the pre-existing `TestIdentityManager_CompiledRealVsLiveDummyPTY`
  subtests for these two checkpoints snapshotted before the chained upload
  beat (09-06-PLAN.md Task 2) had settled on one or both sides, an
  intermittent race this task's new region coverage surfaced. Fixed by
  adding `mustSeeSlow(t, ..., "Running:", ...)` waits on both real and dummy
  sessions before snapshotting.
