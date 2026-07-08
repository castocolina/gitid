# ONESHOT PLAYBOOK — gitid v1.0 Autonomous Completion Run (Phases 3–10)

This is the **playbook** consumed by `.planning/ONESHOT-LOOP-PROMPT.md`
(the `/loop` driver — the intended way to run this). The loop re-reads this
file every iteration; it can also be pasted standalone into a fresh session.
It drives the remaining v1.0 milestone end-to-end with no human in the loop,
ending at a validated `v1.0.0-rc.1` release-candidate tag.

---

## Mission

Complete Phases 3 through 10 of the gitid v1.0 TUI-First Redesign
autonomously. Every phase must close with: verified goal achievement,
clean reviews (Claude + Codex), passing e2e with visual evidence evaluated
by Codex, learnings recorded, state committed, and CI green on origin/main.

## Locked run decisions (do not relitigate)

- **Phase 9 upload e2e**: run against the user's REAL GitHub/GitLab accounts
  (gh/glab are authenticated on this machine), with **mandatory cleanup** —
  see Phase 9 rules below.
- **Human gates**: Codex acts as the approval gate for design captures and
  phase UAT. Every phase accumulates an evidence bundle for deferred human
  UAT — the run never pauses to wait for a human.
- **Git policy — branch-per-phase, ff into main**: GSD works each phase on
  its own `gsd/phase-NN-*` branch (as Phases 1–2 did); review fixes land on
  `gsd-reviewfix/*` and fast-forward back. Phase close = fast-forward main
  from the phase branch (`git checkout main && git merge --ff-only
  gsd/phase-NN-*`), **push origin/main**, and confirm CI green before the
  next phase branches off the updated main. Never merge-commit; if ff is
  impossible, something went wrong — stop and reconcile.
- **Release**: the run creates and pushes tag `v1.0.0-rc.1` and validates the
  release pipeline end-to-end. The final `v1.0.0` tag is a human decision.

## Ground rules (binding for the orchestrator and EVERY subagent)

1. Read `CLAUDE.md` and `recipes/` (including `recipes/README.md`) before any
   planning or implementation. Every subagent prompt MUST state: "Read
   `recipes/` first. All output (code, comments, docs, commit messages,
   report) is English-only." — AND must embed the applicable
   `.planning/LEARNINGS.md` entries verbatim (see Step 2's feedback-loop
   contract). Subagents start with a blank context; anything not injected
   into their prompt does not exist for them.
2. Never use `--no-verify`. Commits in logical groups; every commit passes
   pre-commit hooks.
3. E2E tests default to an isolated temp `HOME`. A test MAY touch the real
   `~/.ssh` / `~/.gitconfig` when the scenario genuinely requires it (gh and
   glab are live on this machine), under the product's own ethos: timestamped
   backup BEFORE the mutation, unconditional restore in teardown (runs on
   failure too), and a post-restore diff proving the files are byte-identical
   to the backup. This is not a license to damage the machine — anything
   irreversible outside the test's own scope (deleting keys the test did not
   create, touching non-config files, machine settings) remains forbidden.
   The other sanctioned real-world mutation is the Phase 9 upload e2e
   (rules below).
4. The orchestrator personally runs `make test`, `make test-e2e`, and
   `make lint` at every wave close and phase close. Do NOT trust executor
   PASS claims — executors have reported PASS without `-race` before.
   Reproduce CI conditions when suspicious: `TERM=dumb SSH_AUTH_SOCK= go test -race ./...`.
5. After every planner/roadmap-writing agent run, sanity-check the artifacts
   it touched (e.g. `wc -l .planning/ROADMAP.md`) — a planner once truncated
   ROADMAP.md to a 23-line stub. Recover from git if it happens.
6. Any NEW managed gitconfig block or path a phase introduces MUST be
   registered as reserved in the doctor's reserved-block registry in the same
   phase, or `health --fix` enters a destructive false-positive loop.
7. Destructive-anomaly halt: if the run detects an unexpected mutation of the
   real HOME config files, or a doctor fix loop that does not converge,
   HALT the run immediately and write a blocker report.

## Step 0 — Preflight (abort the run if any check fails)

- **Tools & auth**: `codex --version` works; `gh auth status` authenticated
  AND both key scopes probe clean — `gh api user/keys` (admin:public_key)
  and `gh api user/ssh_signing_keys` (admin:ssh_signing_key) must both
  return without a scope error; remediation if not:
  `gh auth refresh -h github.com -s admin:ssh_signing_key`;
  `glab auth status` authenticated and `glab ssh-key list` returns the
  inventory (REQUIRED for Phase 9 real-account e2e —
  if glab is missing or unauthenticated, STOP and report: the GitLab side
  would silently degrade to mocks, contradicting a locked run decision);
  `node` available (prefix Volta: `export PATH="$HOME/.volta/bin:$PATH"`);
  `git remote -v` shows origin.
- **Clean tree**: resolve every untracked/modified path before Phase 3 —
  commit what belongs (in logical groups), delete what doesn't. Known
  strays at authoring time: `AGENTS.md`, `.planning/intel/`,
  `.planning/phases/05.7-*/`, this file. With push-per-phase, a dirty tree
  leaks into the first phase commit.
- **Origin sync + CI baseline**: local main is far ahead of origin/main —
  Phases 1–2 were never pushed. Push origin/main FIRST and watch that CI
  run to green: it is the run's baseline. A red baseline is a preflight
  failure to fix before Phase 3 — never start phase work on top of an
  unexplained red, or every later failure becomes unattributable.

## Step 0.5 — Orientation

- `ls .planning/phases/` to enumerate the real phase directories and names.
- Read `.planning/ROADMAP.md`, `.planning/STATE.md`, and every
  `NN-CONTEXT.md` for phases 3–10 (all exist; they are binding).
- Current state: Phase 2 (design) complete and approved; Phase 3 has 6 plans
  ready (Wave 1 parallel) and is NOT yet executed; Phases 4–10 have
  CONTEXT.md but no plans (Phase 4 already has UI-SPEC.md).

## Step 1 — Legacy triage (before executing Phase 3)

Scope clarity first: Phases 1–2 delivered planning, design mockups, demos,
and CI/spike work — **no product code**. Therefore EVERY Go package in the
tree (`internal/*`, `tui/*`, `cmd/*`) is 0.0.1 POC inheritance, and all of
it is triage scope: what sits on main today is exactly what becomes legacy
from Phase 3 onward. Before Phase 3 execution, produce
`.planning/LEGACY-TRIAGE.md`. Concrete procedure:

1. **Inventory**: `go list ./...` — one triage row per package (plus
   `tui/` split by screen file where verdicts differ within the package).
2. **Evidence per package**: `codegraph_explore` for its role, public
   surface, and dependents. No verdict without evidence.
3. **Cross-reference matrix** (package × phase): read every phase 3–10
   CONTEXT.md and mark which packages each phase already assumes it will
   reuse (known: `internal/doctor`, `internal/uploader`, `internal/tester`,
   parts of `tui/`; known rework obligation: `tui/copy.go` prompt queue in
   Phase 9).
4. **Verdict per row** — exactly one of:
   - **keep** — reused as-is; do not touch it now.
   - **rework** — reused but must change; record the OWNING PHASE and what
     changes. Not touched in the triage step — the row becomes a binding
     input to that phase's planning (inject it into the planner's prompt).
   - **replace** — the redesign supersedes it; record the owning phase.
     The old code is deleted BY that phase when its replacement lands,
     never preemptively.
   - **delete** — dead code no phase references. These ARE removed in the
     triage commit itself, with all gates green after removal.
5. **Reconcile divergences**: any CONTEXT.md reuse assumption that the
   evidence contradicts is a finding — resolve it in the triage doc and
   carry the correction into the owning phase's planning.
6. **Commit** the triage doc (plus the `delete` removals) on the Phase 3
   branch before executing any Phase 3 plan.

## Step 2 — Learnings ledger

Create `.planning/LEARNINGS.md` (dated entries: *symptom → root cause →
rule*). Seed it with the already-paid lessons:

- Executors report PASS without `-race`; orchestrator re-runs gates itself.
- Injected-seam wiring blindspot (recurred Phases 4 & 5 of the POC): a seam
  built for tests must be nil-guarded and wired in the REAL constructor, and
  covered by a raw-keystroke PTY e2e.
- CI portability: reproduce locally with `TERM=dumb SSH_AUTH_SOCK= go test
  -race ./...`; watch for `ssh -V` distro suffixes, TERM-dependent glyphs,
  headless doctor, exec grandchild-pipe hangs.
- Doctor reserved-block registration (ground rule 6).
- Planner artifact truncation (ground rule 5).
- gsd-code-fixer commits to `gsd-reviewfix/*`; fast-forward into main.

**The ledger is a closed feedback loop, not a diary. Writing without
reading is failure.** The contract, enforced by the orchestrator:

- **Read at phase start**: the orchestrator re-reads the FULL ledger in
  Step 3.1 of every phase — including entries appended by the phases that
  ran earlier in this same run.
- **Inject into every subagent**: researchers, planners, executors, and
  reviewers never see the ledger unless it is in their prompt. For each
  subagent spawn, the orchestrator selects the entries applicable to that
  task and embeds them VERBATIM in the prompt (e.g. an executor touching
  TUI seams gets the injected-seam rule; a planner gets the
  artifact-truncation rule). "Read LEARNINGS.md" as an instruction is NOT
  sufficient — paste the entries.
- **Check at plan review**: the plan-review gate (Step 3.4) explicitly
  verifies the plans do not repeat a ledgered mistake; a plan that violates
  a ledger rule is a blocking finding.
- **Violations feed back**: if a review or e2e failure turns out to be a
  repeat of an existing ledger entry, that is itself a new entry —
  "rule X was ledgered and still recurred in phase N because …" — so the
  injection targeting improves for the next phase.
- **Append at close**: at minimum, what broke or surprised in the
  e2e/visual loop, capture determinism issues, review friction, and any
  new rule for later phases.

## Step 3 — Per-phase loop (order: 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10)

For each phase N:

1. **Load context**: phase `CONTEXT.md` (+ `RESEARCH.md` / `UI-SPEC.md` if
   present), `LEARNINGS.md`, `recipes/`, and the frozen design references
   (live demos + per-surface `FIELDS.md`) for the screens this phase touches.
   Record the phase-base commit SHA (`git rev-parse HEAD`) — it anchors the
   external code-review range in step 6c.
2. **UI contract** — phases with a TUI surface (4–9): if no `UI-SPEC.md`,
   run `/gsd-ui-phase N`. Phases 3 and 10 have no UI wave. Phase 4's
   UI-SPEC already exists — do not redo it.
3. **Plan** — if the phase has no plans: `/gsd-plan-phase N`.
   Phase 3 already has its 6 plans — skip THIS step only.
4. **Plan review with Codex**: run `/gsd-review --codex N` over the
   PLAN.md files AND the UI-SPEC — it produces `REVIEWS.md`. If it yields
   findings, replan with `/gsd-plan-phase N --reviews` (which consumes
   REVIEWS.md), then re-verify with the plan checker. Max 3 review→replan
   iterations (circuit breaker below).
   **Phase 3 does NOT skip this step**: its plans were checker-verified but
   never Codex-reviewed (no REVIEWS.md exists in its directory).
5. **Execute**: `/gsd-execute-phase N`. Honor wave structure; ground rule 4
   at every wave close. GSD auto-triggers the goal verifier
   (`gsd-verifier` → `VERIFICATION.md`) at phase end — if it did not run,
   run it explicitly before proceeding.
6. **Post-execution review battery** (four distinct lenses, all required,
   in this order; every finding joins the same fixer flow):
   a. **Security review**: `/security-review` (built-in) over the phase's
      changes — this codebase mutates `~/.ssh` and runs subprocesses, so
      gosec alone is not enough.
   b. **Fresh-eyes Claude review**: superpowers `/requesting-code-review`
      over the phase's diff — a reviewer with no execution context.
   c. **GSD code review**: `/gsd-code-review` (then its `--fix` flow for
      findings).
   d. **External Codex code review** over the phase's full commit range,
      invoked directly via bash. Using the phase-base SHA recorded in
      step 1:

      ```bash
      codex exec "External code review of gitid phase <N> (<phase goal>).
      Review the following commit range for bugs, security issues, and
      contract violations against the phase CONTEXT.md. Return
      severity-classified findings (CRITICAL/MAJOR/MINOR) with file:line.

      $(git log --oneline <phase-base>..HEAD)

      $(git diff <phase-base>..HEAD -- ':!.planning')"
      ```

      If the diff exceeds a single invocation, split it per commit group
      or per package and merge the findings. Codex findings join the
      Claude review findings in the same fixer flow.
      If `codex` is unavailable, fall back to `opencode run`; if NO
      external (non-Claude) reviewer is available, STOP the run and report
      — never silently skip the cross-vendor layer or substitute another
      internal Claude agent for it.
   Fix findings via the fixer flow (`gsd-reviewfix/*` → ff into main).
7. **E2E + visual gate** (phases with TUI surfaces): PTY e2e per screen with
   raw keystrokes, plus screen captures under a DETERMINISTIC terminal
   (fixed size, fixed TERM) compared against the approved live-demo mockups.
   Bundle the evidence — capture PNGs/text frames paired with the mockup
   reference and the screen's `FIELDS.md` — and **hand the bundle to Codex
   for evaluation**. Codex's verdict is the approval gate (it replaces the
   human DLV approval for this run). Store everything under
   `.planning/phases/<phase-dir>/evidence/` for deferred human UAT.
   *Phase 9 exception*: its e2e additionally verifies real autoupload to
   GitHub/GitLab (rules below). *Non-UI phases* (3 backend, 10 release):
   the evidence Codex evaluates is command-level execution transcripts —
   the command run and its real output — instead of screenshots.
8. **Close**: append `LEARNINGS.md`; record session state
   (`state.record-session`); commit in logical groups on the phase branch;
   fast-forward main from it; **push origin/main**; watch CI
   (`gh run watch`) until green. CI red = phase not closed — fix forward
   before proceeding.

### Phase-specific obligations (from the phases' own CONTEXT.md — binding)

- **Phase 7**: amend REQUIREMENTS §J; demote `excludesfile` from baseline
  Tier-1; fixture `diff3` → `zdiff3`.
- **Phase 8**: scoped CLAUDE.md amendment for surgical fixer edits;
  FIELDS.md typed-confirm amendment; CheckOrphans git-only-delete downgrade;
  reserved-PATH registry + archive regression test.
- **Phase 9**: rework `tui/copy.go` prompt queue per the shared
  upload-section contract; amend create-flow + identity-manager FIELDS.md;
  mint FRESH approved captures (the old reference PNGs were removed
  repo-wide — live demos are the reference); key title format
  `gitid: <name> @ <hostname>`.
- **Phase 10**: explicit `CGO_ENABLED=0 -trimpath` on all release builds;
  `fetch-depth: 0` in workflows; ensure `bin/` is gitignored (dirty-flag
  guard); create root `PLATFORM-NOTES.md`; as the phase's last deliverable,
  refresh README.md via the `crafting-effective-readmes` skill with two
  install paths spelled out: (a) the curl|bash `install.sh` one-liner as
  the primary path, with clear usage (including `GITID_VERSION` pinning and
  the inspect-first variant), and (b) manual install with explicit per-OS
  checksum validation — Linux `sha256sum --ignore-missing -c checksums.txt`,
  macOS `shasum -a 256 --ignore-missing -c checksums.txt`.
- **Carried warnings**: Phase 2 verification carried W1 (insteadOf) and W2
  (no-backend-test) as open. The phase that lands each area (insteadOf →
  global-git phase; backend tests → Phase 3) must explicitly close its
  warning in its VERIFICATION.md — do not let them ride to milestone close.

### Phase 9 real-account e2e rules (mandatory)

- Only `.pub` paths ever reach the uploader — never a private key.
- Every key uploaded by a test carries a distinguishable test marker in its
  title (e.g. `gitid-e2e:` prefix) so it can never be confused with a real
  key.
- **Cleanup is unconditional**: delete uploaded test keys via provider
  inventory IDs in a defer/teardown that runs even when the test fails.
  After the suite, run an inventory sweep asserting zero `gitid-e2e:` keys
  remain; a leftover key is a test failure.
- NEVER delete a key the test run did not create.
- If gh/glab auth is missing at runtime, degrade those e2e to the mock path
  and record it in the evidence bundle — do not block the run.

## Step 4 — Run close + milestone handoff (after Phase 10)

The run does NOT close the milestone — that is the user's, done when they
validate on a non-macOS machine. The run's closing duties:

1. Cross-phase integration check (`gsd-integration-checker`): the full
   user journeys across all phases.
2. Full gate sweep: `make test`, `make test-e2e`, `make lint`, fedora
   container job green in CI.
3. Tag `v1.0.0-rc.1`, push the tag, watch `release.yml` end-to-end (this
   validates Phase 10's own deliverable). Verify: 4 artifacts +
   `checksums.txt`, checksum verification passes, attestation present,
   `install.sh` works from the published release, `gitid version` reports
   the tag truthfully. If the Homebrew tap step fails for a missing PAT,
   record it as EXPECTED-BLOCKED (human item) — do not fail the run.
4. Final report (English, committed as `.planning/RUN-REPORT.md`):
   per-phase outcomes, evidence index, LEARNINGS highlights, and the
   **milestone handoff checklist for the user**: `/gsd-audit-milestone` →
   `/gsd-complete-milestone`, Bazzite/non-macOS manual UAT, human review of
   all Codex-approved captures, Homebrew tap PAT, final `v1.0.0` tag.
   Then STOP.

## Circuit breakers

- Max **3 iterations** per review→fix loop; max **2 replans** per phase.
- On breach: write the blocker into `STATE.md`, commit everything that is
  green, produce a blocker report, and STOP the run (phases are sequential —
  do not skip ahead over a blocked phase).
- Ground rule 7 (destructive anomaly) overrides everything: halt.

## Context management

- GSD's subagents (researchers, planners, executors, reviewers) do the
  heavy lifting in their own contexts — the orchestrator's context stays
  thin: state, decisions, gate results, learnings.
- Each phase is nevertheless a resumable unit: state recorded + committed +
  pushed at every close. If compaction happens mid-phase, re-read
  `STATE.md`, the phase directory, and `LEARNINGS.md`, and resume from the
  last recorded position rather than restarting the phase.
