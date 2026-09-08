# gitid v1.0 - Claude Code Autonomous Run Driver

Copy-paste driver for a single long-running Claude Code session (expect a
multi-hour, possibly multi-day run). All operational detail — model routing,
per-phase checklist, gates, rules — lives in `.planning/ONESHOT.md`; this file
only starts the run and points there.

## How To Run

From the repository root:

```sh
claude --dangerously-skip-permissions
```

`--dangerously-skip-permissions` is required so the session doesn't stall on
a tool-permission prompt partway through an unattended run — the real safety
gates for this run are `.planning/ONESHOT.md`'s own Non-Negotiable Rules
(rules 5 and 9), not Claude Code's generic per-call confirmation.

Paste the block below as the first message, then leave it running.

---

▼▼▼ COPY FROM HERE ▼▼▼

/gsd-autonomous --to 10 --converge

You are the Claude Code orchestrator for the remaining gitid v1.0 milestone.

Read `.planning/ONESHOT.md` in full now, and again at the start of every
turn, along with `.planning/STATE.md`, `.planning/ROADMAP.md`, and
`.planning/LEARNINGS.md` if present. Follow `.planning/ONESHOT.md` exactly —
it is the binding playbook, this message is only the entry point.

Objective: run the milestone to completion, unattended, from the earliest
incomplete phase through Phase 10, stopping only for a real safety circuit
breaker or a required confirmation before mutating the user's real SSH/Git
configuration or external account. Do not ask the user to send a
continuation command at any point between here and milestone close.

Apply ONESHOT.md rule 10 (the autonomous convergence loop) to every
verification, review, and audit result — do not halt on a failed corrective
attempt.

If a `gaps_found` verification result appears at any phase, choose "Run gap
closure" yourself and continue the convergence loop described in
`.planning/ONESHOT.md` Non-Negotiable Rules, rule 10.

▲▲▲ COPY TO HERE ▲▲▲
