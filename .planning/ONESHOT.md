# ONESHOT PLAYBOOK - gitid v1.0 OpenCode Completion Run

## Runtime And Model Routing

This playbook runs through OpenCode with Open GSD. `.planning/config.json` is
the single source of truth for runtime and model routing. Do not hard-code
Claude Code commands, Codex CLI configuration, model names, or provider
credentials in this file.

- `gsd-planner` and `gsd-code-reviewer` use OpenAI Sol.
- `gsd-plan-checker` and `gsd-verifier` use Claude Opus through OmniRoute.
- `gsd-executor` uses OpenCode Go Qwen.
- `/gsd-plan-review-convergence` uses `review.default_reviewers`.

Run `/gsd-autonomous --from 4 --converge` after the Phase 3 closeout below.
It owns the normal discuss, plan, review, execute, and verification sequence
for every remaining incomplete phase.

## Non-Negotiable Rules

1. Read `AGENTS.md`, `recipes/README.md`, and both recipes before planning or
   implementation. Generated artifacts, code, comments, and commit messages
   are English-only.
2. Never use `--no-verify`. Keep implementation, tests, and documentation for
   one logical change in the same commit.
3. Continue autonomously through ordinary implementation, review, testing,
   planning, and commit work. Do not stop simply because a previous executor
   lacked subagent access; the orchestrator must perform the omitted gate.
4. Do not discard a dirty worktree to begin or complete a phase. Classify each
   path with Git evidence: commit active project work, ignore only reproducible
   local output, and delete only stale output with no active repository role.
5. Never mutate real `~/.ssh/*`, `~/.gitconfig*`, or external account state
   without the product's required confirmation, timestamped backup, and
   post-action verification. Tests default to an isolated temporary `HOME`.
6. At every wave and phase close, independently run `make test`, `make lint`,
   and `make test-e2e`. Do not accept an executor's unverified claim.
7. A newly introduced managed Git or SSH path must be registered in the
   doctor's reserved-path/block registry in the same phase.
8. Stop only for a destructive anomaly, an unrecoverable tool/authentication
   failure, a circuit breaker, or a required confirmation for a real user-file
   or external-account mutation. Record the exact blocker and preserve all
   green work.

## Current Resume Point: Phase 3 Closeout

Phase 3 implementation is complete: all six plans have summaries, and
`6fca783` records the completed UI/UX review fixes. State prose predating that
commit is stale. Do not rerun Phase 3 planning, legacy triage, or completed
waves.

Complete these items in order:

1. Run the independent Phase 3 code and visual-diff review using the configured
   OpenCode Sol reviewer. Review the full Phase 3 commit range and the
   `03-06-review-packet` evidence. Record all findings and dispositions in
   `03-06-SUMMARY.md`; critical or high findings must be fixed and re-reviewed.
2. Run the configured Opus verifier against Phase 3 and create
   `03-VERIFICATION.md`. It must check the phase goal, requirements, review
   results, and current commit range rather than trusting stale state prose.
3. Update `STATE.md`, `ROADMAP.md`, and requirement status only after the
   review and verification pass. Commit the closeout artifacts with their
   evidence.
4. Fast-forward or otherwise integrate the verified phase according to the
   repository's active branch policy, push the intended branch, and confirm CI
   before opening Phase 4 work.

## Remaining Phase Loop: 4 Through 10

Use `/gsd-autonomous --from 4 --converge`. The convergence flag makes Open GSD
run external plan review before execution; the internal Opus plan checker still
guards generated plans. For each phase:

1. Read the phase context, current design contracts, recipes, applicable
   learnings, and completed phase summaries.
2. Generate a UI contract when the phase has a TUI surface and none exists.
   Do not regenerate an approved contract without a concrete gap.
3. Plan, converge-review, and execute in GSD's declared wave order. Cross-AI
   execution occurs only for plans explicitly marked `cross_ai: true` or when
   forced by a command flag; `workflow.cross_ai_execution` alone is not a
   blanket executor override.
4. Run security review, GSD code review, the configured external OpenCode
   review lane, UI review/evidence for TUI surfaces, and the configured Opus
   goal verifier. Resolve critical/high findings; record a fixed or accepted
   disposition for lower-severity findings.
5. Commit all implementation, tests, review evidence, summaries, verification,
   state, and roadmap changes required to close the phase. Do not leave phase
   closeout artifacts uncommitted.

## Phase 9 External Account Policy

Phase 9 may interact with real GitHub/GitLab accounts only after explicit user
confirmation. Before any upload, verify the applicable CLI authentication and
required scopes. Test uploads use a recognizable test title, upload public keys
only, and unconditionally remove only keys created by the test. If the required
provider authentication is unavailable, record the blocked evidence and stop;
do not silently substitute a mock for the required real-account validation.

## Milestone Close

After Phase 10, run cross-phase integration checks and the full test, lint, and
e2e suite. Create and push `v1.0.0-rc.1`, verify the release pipeline and
artifacts, then commit `.planning/RUN-REPORT.md` with per-phase verification
and remaining human handoff items. The final `v1.0.0` release remains a human
decision.
