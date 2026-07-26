# Deferred items — Phase 3

Out-of-scope discoveries logged during execution. Each is real, none is caused
by the plan that found it, and none is fixed in that plan (execution scope
boundary).

| Found in | Item | Why deferred |
|----------|------|--------------|
| 03-04 Task 1 | `FIELDS.md create-flow/confirm-write` row 4 `nothing_changed_note` ("Nothing has changed yet") is NOT rendered by `internal/tuikit/ceremony.go`. The ceremony shows `(written first — restore it to undo)` instead. | Pre-existing since the Phase-2 dummy; adding the line costs a body row in the shared ceremony (every mutating flow), which is a checkpoint-2 row-budget decision, not a 03-04 delta. Excluded from the `gate-copy-freeze` list until the owning plan adds it. Candidate owner: 03-05 (confirm-write ceremony wave). |
| 03-04 Task 2 | `renderWizard` iterates the PACKAGE-LEVEL `AlgorithmCatalog` var while `wizardModel.algo()` indexes `w.catalog()` (the Backend seam). With the real Backend the two lists can differ (the real catalog is probe-resolved), so the rendered rows and the selected id could disagree. | Touching the algorithm-catalog render is the KEY-01/KEY-03 surface, owned by a later create-flow wave; 03-04's scope is the SSH form, collision, reuse picker and banner. Flagged for 03-05. |
