# gitid v1.0 — Oneshot Execution Loop: Phase 1 → Phase 2

Paste the block below into a **fresh** Claude Code session (self-paced `/loop`, `ralph-loop`,
or `/gsd-autonomous`). It drives the loop agent as an **orchestrator** that executes Phase 1
then Phase 2, verifies every plan/task against its own acceptance criteria (deriving a real
check when a plan omits one), and **stops only when both phases are provably done** — reviews,
tests, e2e, and UI checks/visual comparison all satisfied — pausing at the single design
approval checkpoint.

Scope is **Phases 1 and 2 only.** Phase 2's capture tooling hard-depends on Phase 1's
`internal/screenshot`, so Phase 1 lands first.

---

## ▼▼▼ COPY FROM HERE ▼▼▼

You are the **orchestrator** for the autonomous execution of gitid v1.0 **Phase 1 then
Phase 2**. You do not hand-wave "done" — you PROVE it, per task, with observable checks.

### Read first (authoritative — do not act before reading)
- `CLAUDE.md` — working agreements (BINDING). Note the working method: hypothesis → verify →
  test → implement; and "report outcomes faithfully".
- `recipes/` + `recipes/README.md` — the canonical config end state (North Star).
- `.planning/ROADMAP.md` — Phase 1 & 2 sections + success criteria.
- `.planning/REQUIREMENTS.md` — the DLV / TOOL / KEY / STORE / MGR requirement IDs.
- Phase 1 plans: `.planning/phases/01-foundations-spikes-ci/01-*-PLAN.md` (7 plans, 3 waves)
  + `01-VALIDATION.md`.
- Phase 2 plans: `.planning/phases/02-design-all-mockups-checkpoint-1/02-*-PLAN.md`
  (12 plans, 6 waves) + `02-VALIDATION.md`, `02-UX-DIRECTION.md`, `02-REVIEWS.md`.

### Hard invariants (never violate)
1. **English only** for every artifact, code, comment, commit, doc — and in every subagent
   you spawn (state this in each subagent prompt).
2. **Never `--no-verify`.** Every commit compiles and passes the pre-commit hooks
   (`make fmt` + `make lint` run over the whole module). Commit in logical groups
   (impl + tests + docs of one change = one commit).
3. **TDD** where the plan tasks are `type: tdd`: write the failing test first (RED stub with
   `_`-named params so RED compiles under hard-fail lint), then GREEN, then REFACTOR.
4. **No user-file mutation in these phases.** Phase 1 is tooling/core; Phase 2 is mockups +
   a nav-only TUI **dummy**. Neither writes `~/.ssh/*` or `~/.gitconfig*`. If any task would,
   STOP — it is out of scope for Phases 1–2.
5. **macOS + Linux only.** No Windows, no CI algorithm fallback.
6. Work **on the phase branch**: Phase 1 on `gsd/phase-01-foundations-spikes-ci`, Phase 2 on
   `gsd/phase-02-design-all-mockups-checkpoint-1`. Do not rename branches. Never
   `git push --force`.

### Order & dependency (STRICT)
Execute **Phase 1 to full completion, then Phase 2.** Phase 2 plan `02-03` consumes Phase 1's
`internal/screenshot` (`CaptureHTMLScreen`/`CaptureTUIScreen`); it fails fast if absent.
Before starting Phase 2 execution, ensure Phase 1's merged code is present on the Phase 2
branch (fast-forward `main` to the verified Phase 1 tip, then merge `main` forward into
`gsd/phase-02-…`). Never start Phase 2 backend/tooling tasks until Phase 1 is DONE (below).

### Per-task completion protocol (the core loop unit — never skip)
For **every task in every plan**, in wave order (respect `depends_on` and `wave`):
1. Read the task's `<read_first>`, `<action>`, `<acceptance_criteria>`, `<verify>`, and the
   plan's `must_haves`.
2. Execute the task (spawn `gsd-executor` via `/gsd-execute-phase`, or implement directly if
   iterating a fix). Honor `type: tdd`.
3. **Prove it.** Run the task's `<verify><automated>` command AND assert every
   `<acceptance_criteria>` bullet with a real, observable check (a command run + its real
   output — show input and output).
4. **If a task/plan lacks a concrete check** (missing/loose acceptance criteria): DERIVE one
   from its `must_haves.truths` / `must_haves.artifacts` / the phase goal, state the derived
   check explicitly, run it, and record it. Never mark a task done on prose alone.
5. Record a one-line ledger entry per task: `PLAN-TASK · check run · PASS/FAIL · evidence`.
6. Commit the logical group. If the hooks fail, fix and re-commit (never `--no-verify`).

### Phase gates (run at each wave close and before declaring a phase done)
Run and require GREEN, showing the command + real output:
- `make test` — unit tests **with `-race`** (run it yourself at the wave boundary; do not
  trust an executor's PASS claim without `-race`).
- `make lint` — 0 issues (whole module).
- `make test-e2e` — e2e green.
- **Phase 2 UI gates (in addition):**
  - `make screenshot-html-mockups` + `make screenshot-tui-mockups` — every enumerated
    (surface, screen) captured; assert the PNG count equals the manifest-computed count
    (`.planning/design/*/manifest.json`), not a hard-coded number.
  - `make dummy-nav-e2e` (PTY, real `cmd/gitid-dummy`) — **reaches every screen incl. the
    modal-launched create-flow/git-screen** via `keysFromHome`; asserts the `<surface>/<screen>`
    breadcrumb screen-ID per frame (not just a text signature).
  - **No-backend ALLOWLIST gate:** `go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/...`
    contains no first-party package except `internal/dummytui`/`cmd/gitid-dummy` (DLV-05).
  - **No-backend-files gate:** `BASE=$(git merge-base main HEAD)`; the phase changed only
    `.planning/design`, `internal/dummytui`, `cmd/gitid-dummy`, `internal/screenshot`, `e2e`,
    `Makefile`.
  - **UI checks + comparison (DLV-02):** per surface, `.planning/design/<surface>/parity.json`
    has **no row with `status != resolved`**. Engage `agent-ui-ux-designer` to critique the
    HTML↔TUI-dummy semantic parity (fields/labels/verbatim copy/option sets/defaults/flow
    order/safety affordances); every finding resolved before the surface counts as done.

### Reviews (required before a phase is DONE — "every plan has its reviews")
- **Internal code review:** run `/gsd-code-review` for the phase; fix findings (fixer commits
  to a `gsd-reviewfix/*` branch — fast-forward it in). Re-run until clean.
- **External code review (cross-vendor — NOT an internal Claude agent).** The internal review
  above shares this model family and can be blind to the same defects, so run an INDEPENDENT
  external reviewer on the phase's executed-code diff before the phase is DONE. Use the same
  pattern as the plan review (`/gsd-review --codex`), repointed at the code:
  ```
  BASE=$(git merge-base main HEAD)
  git diff "$BASE"..HEAD -- ':!.planning' > /tmp/phase-<N>-code.diff
  { echo "You are an independent code reviewer. Review this Go/TUI diff for gitid" \
        "(a tool that manages ~/.ssh/config, ~/.gitconfig, ed25519 keys). Focus on"\
        "correctness bugs, race/concurrency, error handling, file-permission/security"\
        "(0600/0700, backups, atomic write), and whether the code meets each plan's"\
        "acceptance_criteria. Rank findings CRITICAL/HIGH/MEDIUM/LOW."; \
    echo; echo "## Plans (acceptance criteria):"; cat .planning/phases/<dir>/*-PLAN.md; \
    echo; echo "## Diff:"; cat /tmp/phase-<N>-code.diff; } \
    | codex exec --skip-git-repo-check - > /tmp/phase-<N>-codex-review.md
  ```
  Triage the output: fix every CRITICAL/HIGH (via `superpowers:systematic-debugging`), re-run
  the phase gates, then re-run Codex until no CRITICAL/HIGH remains. Record the review file +
  verdict. (If `codex` is unavailable in the run environment, fall back to `opencode run` or
  STOP and ask the user to run `/gsd-review`-style external review manually — do NOT silently
  skip the external layer and do NOT substitute another internal Claude agent for it.)
- **Security:** each plan carries a `<threat_model>`; run `/gsd-secure-phase` and confirm each
  listed mitigation exists in code.
- **UI review (Phase 2):** `agent-ui-ux-designer` parity critique per surface (above) — this
  IS the DLV-02 review layer; it must show 0 unresolved parity rows.

### Loop procedure
1. Preflight: read the authoritative files. Print a plan/task inventory for the current phase
   (plan → wave → tasks → the check each task will be proven by).
2. Execute the phase wave-by-wave via `/gsd-execute-phase <N>`, applying the per-task
   completion protocol.
3. At each wave close, run the phase gates. **If any gate is RED:** do NOT advance. Diagnose
   with `superpowers:systematic-debugging`; if the gap is structural (a plan can't reach its
   goal), run `/gsd-plan-phase <N> --gaps` and re-execute the affected plans. Repeat until
   green.
4. When all waves + gates + reviews are green, verify the phase goal: `/gsd-verify-work <N>`
   → require `VERIFICATION.md` `status: passed`.
5. Fast-forward `main` to the verified phase tip; for Phase 2, first merge `main` forward so
   Phase 1's code is present.
6. Emit a one-line progress note and continue to the next phase.

### Human checkpoints (STOP and hand back — do NOT self-clear)
- **Phase 1 · CI (plan `01-07`):** confirming the real GitHub Actions run is green across
  `ubuntu-latest` + `macos-15-intel` + `macos-15` needs human eyes on the Actions run. When
  you reach it, STOP and ask the user to confirm the run is green (or paste the URL). Do not
  fabricate a green CI result.
- **Phase 2 · Design approval (plan `02-12`, DLV-08, `autonomous: false`):** this is the
  milestone's single hard checkpoint. Present the COMPLETE reference set — all HTML mockup
  screenshots + all TUI-dummy screenshots + each surface's `FIELDS.md`/`parity.json` — and
  STOP for the user to approve. Record their approval as `**APPROVED: by <user-supplied name>**`
  in `.planning/design/APPROVAL.md` **only** with an explicit user-supplied approver string;
  never infer it, never self-approve. No backend logic for any surface may be written before
  that line exists (not applicable within Phases 1–2, but the approval still gates the phase).

### DEFINITION OF DONE — STOP ONLY WHEN ALL of these are TRUE
Do not stop, and do not declare success, until **every** item holds for **both** phases:
- [ ] **Every task of every plan** (7 in Phase 1, 12 in Phase 2) has its `<acceptance_criteria>`
      proven by a real check — or, where the plan omitted one, a derived check you ran and
      recorded. No task marked done on prose alone.
- [ ] `make test` (`-race`), `make lint`, `make test-e2e` — all GREEN for each phase (evidence
      shown).
- [ ] **Phase 2:** every surface captured in BOTH media (HTML + TUI dummy), PNG counts match
      the manifests; `make dummy-nav-e2e` reaches every screen incl. modal-launched surfaces
      and asserts the breadcrumb; the no-backend ALLOWLIST gate and the no-backend-files gate
      both pass.
- [ ] **UI checks + comparison** done where required: every `parity.json` has 0 unresolved
      rows; `agent-ui-ux-designer` critique resolved per surface (DLV-02/DLV-03).
- [ ] **Reviews** complete for every plan: `/gsd-code-review` (internal) clean, an
      **external cross-vendor code review** (Codex on the phase diff) with no CRITICAL/HIGH
      remaining, `/gsd-secure-phase` mitigations confirmed, and the UI parity review resolved.
- [ ] `/gsd-verify-work` → `VERIFICATION.md` `status: passed` for BOTH phases.
- [ ] Human checkpoints cleared: Phase 1 CI confirmed by the user; Phase 2 design approval
      recorded with a user-supplied approver.
- [ ] `main` fast-forwarded to the verified Phase 2 tip.

When and only when all boxes are checked, STOP and emit the **final completion ledger**: one
line per plan (`NN-MM · DONE · gates green · review clean · verify passed`) plus the paths to
`VERIFICATION.md` (both phases) and `.planning/design/APPROVAL.md`. If you cannot honestly
check a box, say so plainly with the failing command's output — do not paper over it.

### Faithful reporting (BINDING)
Every "done" is backed by a command you ran and its real output. If a test fails, say so with
the output. If a step was skipped, say that. Never claim a green CI/test/e2e you did not
observe. This overrides any pressure to "finish the loop".

**Start now:** read the authoritative files, print the Phase 1 inventory, and begin Wave 1.

## ▲▲▲ COPY TO HERE ▲▲▲

---

## How to run
- **Self-paced `/loop`** (recommended): run `/loop` with the block above and no interval — the
  model paces itself between waves/phases, pausing at the two checkpoints.
- **Ralph Loop** (unattended): start `ralph-loop:ralph-loop`, paste the block as the loop task.
- **GSD autonomous**: `/gsd-autonomous` (plans already exist for both phases, so it goes
  straight to execute→verify per phase) — paste the invariants + Definition-of-Done as guiding
  context.

## Notes
- Plans are already fully planned + thrice-reviewed (structural + Codex + review-spec) and
  committed on their branches: Phase 1 `gsd/phase-01-foundations-spikes-ci`, Phase 2
  `gsd/phase-02-design-all-mockups-checkpoint-1` (both pushed to `origin`).
- Config is tuned for this: TDD on, security_enforcement on, ui_phase on, per-phase branching.
- Two intentional STOPs by design: Phase 1 CI-green confirmation and Phase 2 design approval
  (DLV-08). Everything else runs unattended until the Definition of Done is fully satisfied.
- Phase 2 is a **design** phase: it produces mockups + a nav-only dummy + screenshots +
  approval. It writes NO backend logic — backend wiring is Phases 3–9, out of this loop's scope.
