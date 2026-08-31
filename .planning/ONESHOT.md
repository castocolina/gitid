# ONESHOT PLAYBOOK - gitid v1.0 Claude Code Autonomous Run

## Starting Point

Determine `--from` at invocation time, not from a hardcoded phase number: read
`ROADMAP.md` and `STATE.md` and use the earliest phase that is not
`phase_complete`. `gsd-autonomous`'s own resume gates (phase-discovery
filtering in `autonomous.md`, plan/wave `has_summary` filtering and the
missing-`VERIFICATION.md` fallback in `execute-phase.md`) already reconcile a
partially-finished phase — including one whose plans are all summarized but
never reached verification, or that surfaces `gaps_found` requiring one more
planning pass. Do not hand-run a phase's closeout procedure; let the same
Per-Phase Checklist below apply to whichever phase is earliest-incomplete,
whether that is Phase 3 today or a different phase after a future stall.

## Per-Phase Checklist

For every phase, confirm all nine of these before moving to the next phase —
do not assume any of them ran just because the previous one did:

1. Discuss — CONTEXT.md exists and reflects the phase.
2. UI-phase — UI-SPEC.md exists, only for phases with a TUI surface.
3. Plan — PLAN.md file(s) exist for every wave, and every single PLAN.md has
   `cross_ai: true` in its frontmatter. This is non-negotiable: no plan may
   be created, replanned, or gap-closed without it. Add it immediately if a
   planner omitted it — never proceed to review or execution with a plan
   missing this field.
4. Plan review convergence — REVIEWS.md shows 0 HIGH concerns.
5. Execute — every wave's SUMMARY.md exists, tree is clean, tests green.
   Confirm cross-AI delegation was actually used per Rule 12 below, not a
   direct on-session executor dispatch, unless a recorded per-plan fallback
   applies.
6. Code review — REVIEW.md shows clean or all findings fixed.
7. Verify-work — VERIFICATION.md shows `passed` (or a resolved
   `human_needed`/`gaps_found` outcome, not left open).
8. UI review — UI-REVIEW.md exists for phases with a TUI surface (see
   Non-Negotiable Rules for what "passing" means here — never a browser
   screenshot).
9. `/gsd-audit-uat` — run it yourself. Nothing above triggers it
   automatically; treat it as a required step, not a periodic extra.
10. Execute `codegraph sync` when enter to new worktree and after every task or wave. Use codegraph_explore MCP tool when available to find symbols in file, prirityze new tools like rg, bat, sd, fx, eza over old ones.

Only close a phase and advance once all nine are evidenced.

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
8. gitid is a terminal TUI, not a web UI — `gsd-browser` (Chrome DevTools
   Protocol) does not apply. From Phase 3 onward, the approved Bubble Tea
   mockup (`cmd/gitid-dummy`) is the authoritative visual and interaction
   reference; the Phase-2 HTML prototype is not a parity target. For a
   TUI-surface phase, "UI review" and Definition-of-Done mean the compiled
   real binary is exercised through a real PTY session (raw keystrokes and
   mouse sequences where relevant) against that mockup's workflow, labels,
   controls, and layout guidance. A unit or wiring test never substitutes for
   this.
9. Stop only for a destructive anomaly, an unrecoverable tool/authentication
   failure, a circuit breaker, or a required confirmation for a real
   user-file or external-account mutation. Record the exact blocker and
   preserve all green work.
10. When verification, code review, UI review, UAT audit, or independent
    evidence review finds a functional, workflow, safety, or interaction gap,
    fix it autonomously: create or revise the smallest corrective plan,
    implement it test-first, rerun every affected gate, and obtain a fresh
    independent review. Repeat this loop until the relevant reports are clean
    and the phase checklist is evidenced. A failing test, review finding,
    incomplete artifact, stale evidence, or an agent's failed attempt is work
    to fix, not a blocker. Stop only for rule 9 safety conditions, a required
    real-file/account confirmation, or a demonstrable repeated zero-progress
    tool failure; record the evidence for that stop.
11. Plan-review convergence remains mandatory for every new or revised plan:
    resolve all functional, workflow, safety, and interaction HIGH findings
    before execution. Automatic UI verification compares the real compiled TUI
    with the live `cmd/gitid-dummy` mockup and accepts no visual or interaction
    defects. Every difference must be explicitly classified as a UX improvement
    or a defect; an unclassified difference fails review. Improvements remain
    recorded for the user's one manual review after all phases complete, not at
    individual phase close. Do not require 100% byte, pixel, or historical-
    artifact parity for Phases 3–10.
12. **Every PLAN.md must carry `cross_ai: true` in its frontmatter — this is
    non-negotiable, with no exceptions and no alternative path.** Plan
    EXECUTION is delegated to cross-AI, never run on this session's own token
    budget. `workflow.cross_ai_execution` is `true` in `.planning/config.json`
    specifically so `/gsd-execute-phase` routes each plan to the configured
    external command (`workflow.cross_ai_command`, currently `opencode run
    --model local-llm-env/my-coding ...`) instead of spawning a
    `gsd-executor` subagent on this Claude session. The config flag alone
    does not activate delegation — `execute-phase.md`'s `cross_ai_delegation`
    step also requires the plan's own frontmatter `cross_ai: true`
    (per-Checklist-item-3 above). Passing `--cross-ai` to force delegation is
    a redundant safety net, not a substitute for the frontmatter field — every
    plan gets the field regardless of whether the flag is also passed. A plan
    is not considered planned (Checklist item 3) until this field is present;
    do not advance a plan to review or execution without it. Only fall back
    to a direct `gsd-executor` dispatch on this session when the cross-AI
    command demonstrably fails for that specific plan (record the failure) —
    never as the default path, and never because adding the field was
    skipped. Burning this session's own context/token budget on ordinary
    plan execution when a configured external provider exists is the exact
    mistake this rule exists to prevent; it previously caused an avoidable
    session-limit stall mid-run.
13. **Every executor uses `codegraph_explore` before Grep/Read** (see
    `AGENTS.md` § Code Exploration for the base rule). Two additions specific
    to autonomous dispatch:
    - Before dispatching any exploration-heavy role (new worktree, new
      subagent, new phase/wave), run `codegraph index || codegraph init -i`
      yourself — don't assume the index is fresh.
    - Restate the `codegraph_explore`-first instruction inline in every
      cross-AI dispatch prompt. A local model does not reliably act on
      policy it only sees via a referenced file in its required reading.

## Phase 9 External Account Policy

The user has authorized Phase 9 GitHub E2E upload tests. Before every upload,
verify `gh auth status` and the required scopes. Generate a disposable public
key only, name every created GitHub key `gitid-e2e:<run-id>:<purpose>`, and
record the exact returned resource ID. Cleanup must delete only those recorded
IDs after confirming their title retains the exact run prefix; never select or
delete by a broad inventory query. Run a final inventory sweep that confirms no
keys from the current `gitid-e2e:<run-id>:` prefix remain. Existing keys,
GitLab, and any external-account action outside this protocol still require
explicit confirmation. If GitHub authentication or scope is unavailable,
record the blocked evidence and stop; do not silently substitute a mock for the
required real-account validation.

## Milestone Close

After Phase 10, run cross-phase integration checks and the full test, lint, and
e2e suite. Create and push `v1.0.0-rc.1`, verify the release pipeline and
artifacts, then commit `.planning/RUN-REPORT.md` with per-phase verification
and remaining human handoff items. The final `v1.0.0` release remains a human
decision.
