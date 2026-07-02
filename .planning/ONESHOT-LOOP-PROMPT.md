# gitid v1.0 Redesign — Oneshot Loop Prompt

Paste the block below into a **fresh** Claude Code session (ideally started with the
`ralph-loop` plugin, or run via `/gsd-autonomous` — see "How to run" at the bottom).
It drives the whole redesign as a loop, stopping only at the single design checkpoint.

---

## ▼▼▼ COPY FROM HERE ▼▼▼

You are executing the **gitid v1.0 TUI-First Redesign** autonomously as a loop.

**Read first (authoritative):**
- `.planning/REQUIREMENTS.md` (REQ ledger, sections A–O)
- `docs/prds/gitid-tui-redesign-v1.0-prd.md` (narrative + phased roadmap)
- `recipes/` + `recipes/README.md` (canonical config end state — the North Star)
- `CLAUDE.md` (working agreements — binding)

**Hard invariants (never violate):**
1. English-only for all artifacts, code, comments, commits, docs.
2. Never `--no-verify`. Every commit compiles and passes hooks (`make fmt`+`lint`+`test`).
3. UI-free core, TDD (write the failing test first). Every file write to a user's
   `~/.ssh/*` or `~/.gitconfig*` goes through the `filewriter` chokepoint (timestamped
   backup + idempotent sentinel block + atomic write + explicit confirm).
4. Local-use tool, macOS + Linux only. No CI/CD algorithm fallback. No Windows.
5. Gates each wave: `make test` (race), `make lint`, `make test-e2e` — all green.

**Delivery method (enforced every UI surface — DLV-01..06):**
- Engage the `/mui` skill AND the `agent-ui-ux-designer` agent on EVERY UI task —
  during planning, execution, AND review.
- Per-surface build order is FIXED: HTML mockup (mui) → screenshot every flow → Go
  TUI **dummy** mockup (full nav, no backend) → screenshot every screen → **STOP for
  user design approval (checkpoint #1)** → backend logic → e2e (PTY, real binary) →
  visual-regression review (reviewer agent diffs live TUI screens vs the APPROVED
  HTML+mockup screenshots). Store screenshots under `.planning/design/<surface>/`.
- Never write backend logic for a surface before its dummy mockup is approved.

**The ONE human checkpoint:**
1. End of Phase 1: user approves the complete design (all HTML mockups + TUI dummy
   mockup screenshots). Do NOT proceed to any backend before approval. STOP and ask;
   do not self-approve.

Credential upload (Phase 8) is **autonomous** when `gh`/`glab` is authenticated AND a
valid identity exists (shown command == run command); only if that is unavailable does
it fall back to a manual step. It is NOT a mandatory checkpoint.

**Loop procedure:**
1. If no v1.0 roadmap exists in `.planning/ROADMAP.md`, create it from the PRD's
   Execution Phases (Phase 0 → Phase 9) via `/gsd-roadmapper` (or `/gsd-new-milestone`
   naming it `v1.0 TUI-First Redesign`). Confirm the 4 Open Assumptions in
   REQUIREMENTS.md with the user IF they are online; otherwise proceed on the
   documented defaults and flag them.
2. Execute phases in order. For each phase: `/gsd-plan-phase` then
   `/gsd-execute-phase`, honoring the per-surface UI gate above. Use the configured
   agents (research, plan_check, verifier, deep code_review, ui_review) — they are ON.
3. After each phase, run the gates; if red, fix before advancing (systematic-debugging).
4. At a checkpoint phase, STOP and hand back to the user.
5. Between phases, emit a one-line progress note and continue the loop until Phase 9
   completes or a checkpoint is reached.

**Start now:** read the four authoritative files, then begin at step 1.

## ▲▲▲ COPY TO HERE ▲▲▲

---

## How to run

- **Ralph Loop** (recommended for unattended): start `ralph-loop:ralph-loop`, paste
  the block as the loop task. It will iterate, pausing at the checkpoints.
- **GSD autonomous**: `/gsd-autonomous` after the roadmap exists — it chains
  discuss→plan→execute per phase with the configured agents. Paste the invariants +
  delivery-method sections as the run's guiding context.
- **Self-paced `/loop`**: `/loop` with the block (no interval) lets the model pace
  itself between phases.

## Notes
- Config is already tuned for this: model_profile adaptive, TDD on, code_review deep,
  verifier/ui_review/ui_phase/nyquist on, branching per-phase, worktrees on.
- Resolved: 1 checkpoint (design), auto-upload when gh+auth, provider catalog, STORE
  default, CI/CD builds — all in `.planning/REQUIREMENTS.md` "Resolved Decisions".
- 3 items intentionally open until their phase: GSSH dangerous-options list, KEY-01
  catalog ordering/copy, screenshot tooling choice (see "Still Open" in REQUIREMENTS).
