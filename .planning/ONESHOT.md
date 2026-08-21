# ONESHOT PLAYBOOK - gitid v1.0 OpenCode Autonomous Run

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
3. Plan — PLAN.md file(s) exist for every wave.
4. Plan review convergence — REVIEWS.md shows 0 HIGH concerns.
5. Execute — every wave's SUMMARY.md exists, tree is clean, tests green.
6. Code review — REVIEW.md shows clean or all findings fixed.
7. Verify-work — VERIFICATION.md shows `passed` (or a resolved
   `human_needed`/`gaps_found` outcome, not left open).
8. UI review — UI-REVIEW.md exists for phases with a TUI surface (see
   Non-Negotiable Rules for what "passing" means here — never a browser
   screenshot).
9. `/gsd-audit-uat` — run it yourself. Nothing above triggers it
   automatically; treat it as a required step, not a periodic extra.

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
   Protocol) does not apply. For a TUI-surface phase, "UI review" and
   Definition-of-Done mean the screens are proven against the approved
   mockup through a real PTY session on the compiled binary (raw keystrokes,
   mouse sequences where relevant) — the existing DLV-04/DLV-06 gates. A
   unit or wiring test never substitutes for this.
9. Stop only for a destructive anomaly, an unrecoverable tool/authentication
   failure, a circuit breaker, or a required confirmation for a real
   user-file or external-account mutation. Record the exact blocker and
   preserve all green work.
10. When a phase's post-execution verification returns `gaps_found`, choose
    "Run gap closure" without waiting for a live response — this is
    pre-authorized so the run stays unattended. Gap closure is capped at one
    retry by `gsd-autonomous` itself; if gaps persist after that retry,
    treat it as a real blocker under rule 9.

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
