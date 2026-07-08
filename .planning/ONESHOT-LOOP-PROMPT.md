# gitid v1.0 — Phases 3–10 Autonomous Loop Prompt (v2)

Supersedes the pre-execution v1 (see git history) — Phases 1–2 are complete,
all ten phases have CONTEXT.md, and the run rules now live in the playbook.

**This file is the `/loop` driver. `.planning/ONESHOT.md` is the playbook.**
Paste the block below into a fresh session with `/loop` (self-paced, no
interval), or as the task of a `ralph-loop` run.

---

## ▼▼▼ COPY FROM HERE ▼▼▼

You are executing the **gitid v1.0 TUI-First Redesign, Phases 3–10**,
autonomously as a loop.

**The playbook is `.planning/ONESHOT.md` and it is BINDING.** At the start
of EVERY iteration, read in full:

1. `.planning/ONESHOT.md` — ground rules, preflight, legacy triage,
   per-phase command sequence, review battery, e2e evidence gates,
   Phase 9 real-account rules, circuit breakers, run close.
2. `.planning/STATE.md` — where the previous iteration left off.
3. `.planning/LEARNINGS.md` — if present; inject applicable entries into
   every subagent you spawn, verbatim.

**Each iteration:**

1. Determine your exact position: which phase, which step of the playbook's
   per-phase sequence (Step 3.1–3.8), or preflight / legacy triage / run
   close if phase work has not started or has finished.
2. Advance the NEXT unit of work — one playbook step, or one wave within an
   execution step. Never skip a step in the sequence; never re-do a step
   STATE.md records as done.
3. Record progress exactly as the playbook's close rules dictate
   (`state.record-session`, logical-group commits, learnings).
4. Emit a one-line progress note and schedule the next iteration
   immediately — the work is continuous; there is nothing external to
   wait for except CI watches, which you run inline.

**Stop the loop only when:**

- The playbook's Step 4 run close is complete (`.planning/RUN-REPORT.md`
  committed, `v1.0.0-rc.1` pipeline validated) → final report, stop.
- A circuit breaker trips (3 review→fix iterations, 2 replans, destructive
  anomaly) → blocker report per the playbook, stop.
- Preflight fails (e.g. `glab` missing/unauthenticated, red CI baseline
  that resists fixing) → report what is needed from the user, stop.

**Never:** `--no-verify`; non-English artifacts; silently skipping the
external cross-vendor review layer (fallback `opencode run`; if no
non-Claude reviewer exists, stop and report); mutating real HOME config
without the playbook's backup/restore ceremony; self-approving what the
playbook routes through Codex gates.

**Start now:** read the three files above and act from your current
position.

## ▲▲▲ COPY TO HERE ▲▲▲

---

## How to run

- **Self-paced `/loop`** (recommended): `/loop` with the block above, no
  interval — the model paces itself; iterations are back-to-back since the
  work is continuous.
- **Ralph Loop**: start `ralph-loop:ralph-loop`, paste the block as the
  loop task.

## Why loop + playbook are split

The loop prompt is re-sent verbatim every iteration — it must be small and
position-agnostic. The playbook carries the detail (command sequences,
review battery, Phase 9 cleanup rules) and is re-READ each iteration, so
corrections to the playbook take effect on the next iteration without
touching the running loop.
