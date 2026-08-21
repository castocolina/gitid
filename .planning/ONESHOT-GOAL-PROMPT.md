# gitid v1.0 - OpenCode Autonomous Run Driver

Copy-paste driver for a single long-running OpenCode session (expect a
multi-hour, possibly multi-day run). All operational detail — model routing,
per-phase checklist, gates, rules — lives in `.planning/ONESHOT.md`; this file
only starts the run and points there.

## How To Run

From the repository root:

```sh
opencode .
```

Paste the block below as the first message, then leave it running.

---

▼▼▼ COPY FROM HERE ▼▼▼

/gsd-autonomous --from 3 --to 10 --converge

You are the OpenCode orchestrator for the remaining gitid v1.0 milestone.

Read `.planning/ONESHOT.md` in full now, and again at the start of every
turn, along with `.planning/STATE.md`, `.planning/ROADMAP.md`, and
`.planning/LEARNINGS.md` if present. Follow `.planning/ONESHOT.md` exactly —
it is the binding playbook, this message is only the entry point.

Objective: run the milestone to completion, unattended, from the earliest
incomplete phase through Phase 10, stopping only for a real safety circuit
breaker or a required confirmation before mutating the user's real SSH/Git
configuration or external account. Do not ask the user to send a
continuation command at any point between here and milestone close.

Treat verification as an autonomous convergence loop: any failing test,
review finding, divergent visual artifact, incomplete evidence, or gap report
must be planned, fixed, re-tested, and independently re-reviewed until clean.
Do not halt merely because a previous corrective attempt failed; halt only for
a real safety/authentication/confirmation condition or a demonstrated
repeated zero-progress tool failure.

If a `gaps_found` verification result appears at any phase, choose "Run gap
closure" yourself and continue the convergence loop described in
`.planning/ONESHOT.md` Non-Negotiable Rules, rule 10.

▲▲▲ COPY TO HERE ▲▲▲
