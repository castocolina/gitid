# Deferred items — Phase 3

Out-of-scope discoveries logged during execution. Each is real, none is caused
by the plan that found it, and none is fixed in that plan (execution scope
boundary).

| Found in | Item | Why deferred |
|----------|------|--------------|
| 03-04 Task 1 | `FIELDS.md create-flow/confirm-write` row 4 `nothing_changed_note` ("Nothing has changed yet") is NOT rendered by `internal/tuikit/ceremony.go`. The ceremony shows `(written first — restore it to undo)` instead. | Pre-existing since the Phase-2 dummy; adding the line costs a body row in the shared ceremony (every mutating flow), which is a checkpoint-2 row-budget decision, not a 03-04 delta. Excluded from the `gate-copy-freeze` list until the owning plan adds it. Candidate owner: 03-05 (confirm-write ceremony wave). |
| 03-04 Task 2 | `renderWizard` iterates the PACKAGE-LEVEL `AlgorithmCatalog` var while `wizardModel.algo()` indexes `w.catalog()` (the Backend seam). With the real Backend the two lists can differ (the real catalog is probe-resolved), so the rendered rows and the selected id could disagree. | Touching the algorithm-catalog render is the KEY-01/KEY-03 surface, owned by a later create-flow wave; 03-04's scope is the SSH form, collision, reuse picker and banner. Flagged for 03-05 — NOT in 03-05's task list (D-02/D-03/D-19/D-18 only), re-deferred to 03-06 or later. |
| 03-03 (carried via STATE.md) | `realBackend.Persist` has no error channel; a committed write that fails is recorded on `persistErr` and exposed via `PersistError()`, but NO caller in `internal/tuikit` ever reads it after the ceremony's `ceremonyFinished` ⇒ `AddIdentity` dispatch — a failed real write shows the SAME "created" toast/note as a successful one. | 03-05's task list (D-02/D-03/D-19/D-18) does not name this surface; wiring the ceremony-finish path to re-check `PersistError()` and reroute to an honest failure render is a small but real behavior change to `identitiesModel.handleWizardKey`'s step-3 branch, outside this plan's explicit scope. Candidate owner: 03-06 or a dedicated fix pass — flagging again since 03-05 was the last named candidate owner and did not resolve it. |

## Resolution (audited 2026-08-26, Phase 5 closeout / `/gsd-audit-uat`)

All 3 items above are **stale** — verified against the current codebase, not
against these notes:

- **03-04 Task 1** (`nothing_changed_note`): `ceremony.go:414` now renders
  `styleFaint.Render("Nothing has changed yet")` unconditionally in the
  state-A preview. Resolved by a later wave.
- **03-04 Task 2** (`AlgorithmCatalog` divergence): every reference to the
  algorithm catalog in `internal/tuikit/identities.go` (render, mouse
  hit-test, selection) now goes through `w.catalog()` (`w.backend.
  AlgorithmCatalog()`) consistently; the package-level `AlgorithmCatalog` var
  in `design.go` is only the stub/dummy Backend's default return value.
  Resolved by a later wave.
- **03-03** (`PersistError()` never read): the function no longer exists in
  the codebase. Superseded by the async ceremony's `commitErr`/`commitFailed`
  flow (`ceremony.go`), which renders a concrete, retryable error state on a
  failed write instead of a generic success toast.

No action needed; kept here as a resolved record rather than deleted.
