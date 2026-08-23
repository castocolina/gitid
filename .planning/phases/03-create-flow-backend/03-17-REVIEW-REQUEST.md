# Plan Review Request: Phase 3 Corrective Plan 03-17

Review only `.planning/phases/03-create-flow-backend/03-17-PLAN.md`. Do not
review earlier Phase 3 plans except where the corrective plan explicitly cites
them for immutable history or existing behavior.

Use the repository source and cited artifacts to verify the plan, not plan text
alone. Cite concrete `path:line` evidence for every concern.

This is a TUI-only corrective plan for three known PTY UI defects:

1. A stage-2-in-progress real-TUI frame is labelled as though all test stages
   completed, even though completed proof must remain reachable through the
   existing proof viewport.
2. The completed `ReachableNotUploaded` state duplicates the D-02 warning and
   D-03 copy/provider action, while both stages' real output must remain
   available.
3. The compact Git `includeIf` preview spends its single content row on a
   managed-block sentinel instead of the meaningful condition presented by the
   live `cmd/gitid-dummy` mockup.

Binding policy:

- From Phase 3 onward, compare only compiled `cmd/gitid` and live compiled
  `cmd/gitid-dummy` through 100x30 real PTYs. HTML, MUI, browser, Chromium,
  screenshots of historical web artifacts, and substitute UI references are
  out of scope.
- Every real/dummy difference must be classified as an improvement or a defect;
  unclassified differences fail review. The corrective plan must leave no UI
  defects for the three states above.
- The plan must use a disposable HOME and fake SSH only, preserve all unrelated
  dirty paths and prior immutable review/packet artifacts, and never mutate
  real user configuration, keys, accounts, or the network.
- Review provenance and subsequent evidence finalization must use only the
  configured instance `opencode-my-plan-review` via `opencode` with model
  `local-llm-env/my-plan-review`. No second or substitute reviewer is allowed.

Assess whether the plan is executable test-first: every named test must be
proved to exist before selective execution; it must obtain expected RED before
the smallest production change, then green; changes must be committed with
normal hooks. Check that the candidate/review/finalization sequence is bound to
the corrected source and that the plan's packet path and candidate manifest
contracts agree with the existing source.

Return markdown with:

1. Summary
2. Strengths
3. Concerns, each with severity `HIGH`, `MEDIUM`, or `LOW`, source evidence,
   mechanism, and exact plan correction needed
4. Risk assessment
5. An explicit disposition for each of the three UI defects and the binding
   TUI-only/no-defects policy

At the end, include exactly:

`CYCLE_SUMMARY: current_high=<N> current_actionable=<M>`

`current_high` counts unresolved HIGH findings. `current_actionable` counts
MEDIUM or LOW findings that must be incorporated into 03-17-PLAN.md or
explicitly deferred/rejected there. Then include, in this order and with no
later headings:

## Current HIGH Concerns

List unresolved HIGH findings, one bullet each, or exactly `None.`.

## Current Actionable Non-HIGH Concerns

List unresolved actionable MEDIUM/LOW findings and their required plan change,
one bullet each, or exactly `None.`.
