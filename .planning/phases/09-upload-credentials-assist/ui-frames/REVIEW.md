# Phase 9 UX Review Packet — Upload & Credentials Assist

09-07-PLAN.md Task 3 (DLV-04/DLV-06). This packet closes the phase's
real-vs-dummy comparison and per-Focal-point judgement obligations.

## Parity critique

The `agent-ui-ux-designer` critique (D-09 step 3, review R15) HAS now run
against the committed frames in this directory compared to `09-UI-SPEC.md`,
closing D-09's parity-critique obligation. It found 5 real defects (D1-D5)
and 5 UX improvements (I1-I5). All 5 defects are FIXED below, by commit SHA
(quick task `260831-3a9`); all 5 improvements were deliberately NOT
implemented in this pass — a later reader must not mistake this section for
a claim that everything the critique found was fixed.

### Defects (5, all FIXED)

- **D1 — the register-key manual-fallback modal was a dead end.**
  Instructions to paste the `.pub` key with no way to copy it, unlike the
  create-flow's equivalent state (which has offered "c copy public key"
  since Phase 3). Fixed in `ad64789`: a `registerKeyCopyable()` predicate
  gates BOTH the footer hint and the "c" key binding, so the pane can never
  show a hint without a binding or the reverse.
- **D2 — the modal's footer contradicted itself, and the action-menu row
  hid its own mutation.** The status line paired "without writing
  anything" with "registration runs on open" in every state, including
  states where nothing had run or ever would; the action-menu row
  ("Register key (u)") did not disclose the immediate `gh`/`glab` provider
  mutation it triggers before it is pressed. Fixed in `06b74c1`: the status
  line now splits per state (registration ran vs. nothing was registered),
  and the row was renamed to "Register key with provider now (u)" — a
  deliberate, documented 09-UI-SPEC.md Copywriting Contract amendment (the
  project's D-09 precedent for a scoped frozen-copy divergence). The
  "opening the modal IS the opt-in" behavior itself is unchanged
  (deferred-items.md item 1's ratified contract).
- **D3 — `create-flow-upload-checkbox-disabled.txt` shows the UNAUTH
  label, and this REVIEW.md cited it as evidence of the disabled state.**
  Docs-only correction, fixed in `6335687`: the Per-Focal-point judgement
  table now names `TestUploadCheckboxRowIsExactlyOnePhysicalLine`
  (`internal/tuikit/upload_section_test.go:415`) as the real DISABLED-state
  evidence, and no longer offers the mislabeled frame as such.
- **D4 — the rotate delete-offer's live confirmation choice had no visible
  affordance, and a stale `Done (Enter)` misattributed the one visible
  Enter.** While the offer was live and unresolved, the frame showed
  `Done (Enter)` above the still-unanswered question, with no visible
  navigate/confirm hint for the choice row itself. Fixed in `d7ad2ee`: a
  `rotateDeleteOfferLive()` predicate now gates both a new
  `↑↓/Tab choose · Enter confirm` footer action and an opt-in `hideDone`
  suppression on a LOCAL copy of the shared ceremony model — every other
  ceremony's "Done (Enter)" rendering is unchanged
  (`ceremony_test.go` unmodified).
- **D5 — committed reference frames predated `c3af8b9`'s ellipsis fix and
  no longer matched HEAD.** Fixed in `2bac4a1`: all 13 tracked frames
  re-promoted from a cleared capture directory by `cmd/gitid-frame-promote`
  alone, absorbing both the ellipsis fix and the D1/D2/D4 render changes;
  `TestUploadFrameProvenanceMatches` passes in both directions.

### Improvements (5, deliberately NOT implemented — deferred)

Per ONESHOT.md rule 11, these are carried to the user's post-milestone
manual UX review rather than resolved here. Recorded verbatim as the
critique found them:

- **I1** — upload/connectivity results share one undifferentiated glyph
  list, no group label
- **I2** — the upload beat renders above the key-generation event that
  caused it, temporally inverted
- **I3** — wrapped continuation lines outdent one column, breaking
  numbered-list structure in the manual-fallback modal
- **I4** — a resolved delete-offer leaves the already-answered question
  visible on screen alongside its result
- **I5** — inconsistent truncation marker: some cuts use "…", one doesn't

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
| Step 0's checkbox row never wraps, disabled/hint reason inline | Satisfied — the PTY frames evidence only TWO of the row's three states (ready and unauth); the DISABLED state's one-line claim is covered by a unit test, not a PTY frame. `create-flow-upload-checkbox-disabled.txt` (produced by `TestCreateFlow_UploadCheckboxDisabledState`) does NOT evidence the DISABLED state — its own doc comment explains the e2e deny shim yields the UNAUTH shape instead, because `exec.LookPath("gh")` still succeeds against the shim binary on PATH (`DetectFor` only reports `AuthToolNotFound` when `exec.LookPath` itself fails, which never happens here); the genuinely-tool-absent DISABLED shape is not producible by this harness. `create-flow-upload-checkbox-disabled.txt` and `create-flow-upload-checkbox-unauth.txt` are in fact byte-identical (README.md records the same SHA-256, `3eb17e5e…`, for both). The DISABLED state's one-line claim is instead proven by `TestUploadCheckboxRowIsExactlyOnePhysicalLine` (`internal/tuikit/upload_section_test.go:415`), which drives `UploadEligibilityDisabled` directly through `renderUploadCheckboxRow` and asserts no `\n` and width ≤ 60 at production width — confirmed by reading the test before citing it here | `create-flow-upload-checkbox-unauth.txt`, `create-flow-upload-checkbox-tab-and-click.txt` (ready/unauth states only); DISABLED covered by `TestUploadCheckboxRowIsExactlyOnePhysicalLine`, not a PTY frame |
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
