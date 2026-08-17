# gitid v1.0 — Phases 3–10 Autonomous Goal Prompt (v3)

Supersedes the `/loop`-based v2 (see git history: `.planning/ONESHOT-LOOP-PROMPT.md`
up to commit before this file's rename). `/loop` is time/interval-driven —
each turn fires on a schedule and the SAME agent grades its own "am I done"
prose. `/goal` (Claude Code ≥2.1.139) is completion-driven and **oneshot**:
"setting the goal immediately starts a turn — there's no separate prompt
step required." A SEPARATE evaluator model then reads the condition against
the transcript after every subsequent turn and only starts another turn on
"no." That fits this run better than `/loop`: the playbook's own Step 4
run-close criteria already ARE a verifiable completion condition, and an
independent grader policing it is stronger than self-policing.

**This file is the `/goal` driver. `.planning/ONESHOT.md` is the playbook.**
Because `/goal` is oneshot, there is exactly ONE thing to copy: the command
below. It references this very file for the detailed working instructions
(Claude reads it as its first action of the turn) rather than inlining
everything into the condition — that keeps `ONESHOT.md` corrections
effective immediately, without ever having to re-issue `/goal`.

▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼ COPY FROM HERE ▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼▼
```
/goal Read and follow .planning/ONESHOT-GOAL-PROMPT.md (Working Instructions section below) and .planning/ONESHOT.md (the binding playbook) in full before acting, then work until: all of gitid v1.0 Phases 3 through 10 are shipped — ONESHOT.md Step 4's run close is complete for every phase (RUN-REPORT.md committed, v1.0.0-rc.1 tag pushed, gh run watch shows the release pipeline green on origin/main), demonstrated in this transcript by literal git log / git tag / gh run view output rather than a claim — OR a circuit breaker/preflight failure has been reported and the run is paused awaiting the user. Stop after 8 hours of wall time if neither is reached and report the current position instead.
```
▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲ COPY TO HERE ▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲

Copy the single `/goal ...` line between those two markers (the ``` fence
marks are just markdown formatting — don't paste those, only the line
itself). Nothing above `▼▼▼` or below `▲▲▲` is part of it.

---

The evaluator reads ONLY this conversation's transcript, not files
independently — every turn must print the actual command output proving
its claims (git log excerpt, test summary, gate results), not just assert
them, or the evaluator has nothing to grade against.

## Working Instructions

(Read automatically by the `/goal` command above — nothing here needs to be
copy-pasted separately.)

You are executing the **gitid v1.0 TUI-First Redesign, Phases 3–10**.
**The playbook is `.planning/ONESHOT.md` and it is BINDING.** At the start
of EVERY turn, read in full:

1. `.planning/ONESHOT.md` — ground rules, preflight, legacy triage,
   per-phase command sequence (Step 3.1–3.8), review battery, e2e evidence
   gates, Phase 9 real-account rules, circuit breakers, run close.
2. `.planning/STATE.md` — where the previous turn left off. Trust the git
   log over prose if they disagree; STATE.md has gone stale before.
3. `.planning/LEARNINGS.md` — if present; inject applicable entries into
   every subagent you spawn, verbatim.

**Each turn:**

1. Determine your exact position: which phase, which step of the
   playbook's per-phase sequence, or preflight / legacy triage / run close
   if phase work has not started or has finished. Phases 4–10 currently
   have only `CONTEXT.md` — Step 3 (`/gsd-plan-phase N --tdd --chain
   --research`, using `codegraph_explore` for codebase grounding) is a real,
   necessary unit of work for each of them, not a skip.
2. Advance the NEXT unit of work — one playbook step, or one wave within an
   execution step. Never skip a step in the sequence; never re-do a step
   STATE.md AND git both confirm is done.
3. Record progress exactly as the playbook's close rules dictate
   (`state.record-session`, logical-group commits, learnings).
4. End the turn with a short, verifiable status line AND the actual
   command output backing it — this is what makes the goal condition
   checkable at all.

**Never:** `--no-verify`; non-English artifacts; silently skipping the
external cross-vendor review layer (fallback `opencode run`; if no
non-Claude reviewer exists, stop and report); mutating real HOME config
without the playbook's backup/restore ceremony; self-approving what the
playbook routes through Codex gates; auto-approving a tool permission
prompt the user's settings don't already cover — `/goal` does not change
the permission model, it only changes when the next turn starts.

**Report a blocker (the goal condition's second branch) and pause, rather
than retrying blindly, when:**

- A circuit breaker trips (3 review→fix iterations, 2 replans, destructive
  anomaly).
- Preflight fails (e.g. `glab` missing/unauthenticated, red CI baseline
  that resists fixing).
- Phase 9's real-account rules require a human decision the playbook marks
  as a human gate.

**Start now:** read `.planning/ONESHOT.md`, `.planning/STATE.md`, and
`.planning/LEARNINGS.md` (if present), then act from the current position.

## How to run

- **`/goal`** (recommended): copy-paste the single line above, once. It
  starts working immediately — no follow-up message needed. Re-invoke with
  a bare `/goal` in a later session to resume — it re-reads its own
  condition and checks current progress before deciding whether to start a
  new turn.
- **Ralph Loop** (fallback if native `/goal` is unavailable): start
  `ralph-loop:ralph-loop`, paste this file's Working Instructions section
  plus the completion condition as the loop task; this reverts to
  time-driven self-pacing, so re-derive position from `STATE.md`/git at the
  top of every iteration as instructed above.

## Why playbook and driver are split

`ONESHOT.md` carries the detail (command sequences, review battery, Phase 9
cleanup rules) and is re-READ each turn rather than baked into the `/goal`
condition — so corrections to the playbook take effect on the very next
turn without ever having to re-issue `/goal`.
