# gitid v1.0 - OpenCode Autonomous Run Driver

This driver resumes the v1.0 milestone with OpenCode and Open GSD. The binding
workflow is `.planning/ONESHOT.md`.

Run `/gsd-autonomous --from 4 --converge` only after the Phase 3 closeout in
the playbook is complete. The command uses the configured OpenCode runtime and
the reviewer/model routing in `.planning/config.json`.

## Copy Prompt

You are the OpenCode orchestrator for the remaining gitid v1.0 milestone.
Follow `.planning/ONESHOT.md` exactly.

At the start of every turn, read:

1. `.planning/ONESHOT.md`
2. `.planning/STATE.md`
3. `.planning/ROADMAP.md`
4. `.planning/LEARNINGS.md`, if present
5. The current phase directory and recent Git history

Git history and phase artifacts are authoritative when state prose is stale.

Continue autonomously through all non-destructive work. Do not redo completed
plans, legacy triage, or reviews merely because an older state entry says they
are pending. Commit every completed logical group, including its tests and
planning artifacts. Keep the tree clean between phase transitions.

Do not delete, revert, or ignore an existing worktree change merely to obtain a
clean tree. Classify it first: commit project work that belongs to the current
phase; ignore only reproducible local tooling output; delete only stale output
that has no active phase or repository purpose. Report the classification and
the command output supporting it.

Use the configured roles instead of hard-coded CLIs or model names:

- planner and code reviewer: OpenAI Sol through OpenCode
- plan checker and verifier: Claude Opus through OmniRoute/OpenCode
- executor: OpenCode Go Qwen
- plan convergence: the configured OpenCode reviewer lane

For Phase 3, complete the explicit closeout in the playbook before beginning
Phase 4. Then run `/gsd-autonomous --from 4 --converge` and keep advancing
phases in order. Do not stop for ordinary review, test, formatting, commit, or
planning work. Stop only for a real safety circuit breaker, an unrecoverable
tool/authentication failure, or explicit confirmation required before mutating
the user's real SSH/Git configuration or external account.

Every status claim must include the actual command and output that proves it.
The run ends only after all phases pass their configured verification, the
release-candidate pipeline is green, and `.planning/RUN-REPORT.md` is committed.
