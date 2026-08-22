# Milestone Workflow

## Required Sequence

For each phase, read `.planning/ONESHOT.md`, `.planning/STATE.md`,
`.planning/ROADMAP.md`, and `.planning/LEARNINGS.md`, then run:

1. Discuss only unresolved product decisions.
2. Create or revise a plan.
3. Run plan-review convergence before execution.
4. Implement test-first and run the phase gates.
5. Resolve functional, safety, workflow, and interaction findings until clean.
6. Complete code review, verification, UI review, and UAT audit before advancing.

## Autonomous Run

Use the configured review lane without reviewer-selector flags:

```text
/gsd-autonomous --from 3 --to 10 --converge
```

`/gsd-plan-review-convergence <phase>` uses the configured default reviewer.
Do not add `--codex`, `--claude`, `--all`, or another selector unless the user
explicitly changes the reviewer configuration.

## Safety Stops

Continue ordinary fix loops autonomously. Pause only for a destructive anomaly,
missing required authentication, or the confirmation required before mutating a
real user configuration or external account.

Phase 9 may create only disposable public GitHub keys following the exact-ID
cleanup policy in `.planning/ONESHOT.md`.
