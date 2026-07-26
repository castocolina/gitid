# LEARNINGS — gitid v1.0 Autonomous Run (Phases 3–10)

Closed feedback loop, not a diary. Each entry: **symptom → root cause → rule**.
The orchestrator re-reads this FULL file at every phase start (Step 3.1) and
injects the applicable entries VERBATIM into every subagent prompt. Entries are
dated; convert relative dates to absolute.

---

## Seed entries (already-paid lessons carried into this run)

### L1 — Executors report PASS without `-race` (2026-07-08, from POC Phases 1–5)
- **Symptom:** an executor/subagent reported the gate suite PASS, but the race
  detector had not actually run; a data race slipped past.
- **Root cause:** executors run a lighter command than CI and trust their own
  exit code.
- **Rule:** the ORCHESTRATOR personally re-runs `make test`, `make test-e2e`,
  `make lint` at every wave close and phase close. Never trust an executor's
  PASS claim. When suspicious, reproduce CI exactly:
  `TERM=dumb SSH_AUTH_SOCK= go test -race ./...`.

### L2 — Injected-seam wiring blindspot (2026-07-08, RECURRED POC Phases 4 & 5)
- **Symptom:** a dependency seam built for tests worked in unit tests but was
  never wired into the real constructor, so the production path silently used a
  nil/default and the feature was dead in the real binary.
- **Root cause:** the seam was injected only in test setup; the real
  constructor path was never covered by an end-to-end test.
- **Rule:** any test seam MUST be nil-guarded AND wired in the REAL constructor
  (e.g. exported `Build*Deps()`), AND covered by a raw-keystroke PTY e2e that
  exercises the real wiring — not just a unit test with a hand-injected mock.

### L3 — CI portability divergence (2026-07-08, from POC Phase 1, 8 defects)
- **Symptom:** tests green locally, red in CI (headless, `TERM=dumb`, no
  `SSH_AUTH_SOCK`, distro-suffixed `ssh -V`, TERM-dependent glyphs, exec
  grandchild-pipe hangs, uv PATH).
- **Root cause:** local dev environment masks CI's headless/minimal terminal.
- **Rule:** reproduce CI locally before pushing:
  `TERM=dumb SSH_AUTH_SOCK= go test -race ./...`. Watch for `ssh -V` distro
  suffixes, TERM glyphs, headless doctor, exec grandchild-pipe hangs. CI triggers
  only on PR / push-to-main, so watch `gh run watch` after every push.

### L4 — Doctor reserved-block false-positive loop (2026-07-08, ground rule 6)
- **Symptom:** a new managed gitconfig/SSH block introduced by a phase was seen
  by `health --fix` as an orphan/identity, triggering a destructive fix loop
  that did not converge.
- **Root cause:** the new managed block/path was not registered in the doctor's
  reserved-block registry in the same phase that introduced it.
- **Rule:** any NEW managed block or reserved path a phase introduces MUST be
  registered as reserved in the doctor's registry in that SAME phase, with an
  archive/regression test. (Phases 7, 8, 9 all introduce new managed blocks.)

### L5 — Planner artifact truncation (2026-07-08, ground rule 5)
- **Symptom:** `gsd-planner --gaps` once overwrote the full ROADMAP.md with a
  23-line stub.
- **Root cause:** a planner/roadmap-writing agent rewrote a file it should have
  appended to.
- **Rule:** after EVERY planner/roadmap-writing agent run, sanity-check the
  artifacts it touched (`wc -l .planning/ROADMAP.md`, spot-read). Recover from
  git immediately if truncated.

### L6 — gsd-code-fixer branch flow (2026-07-08)
- **Symptom:** fixer commits landed on a branch, not main.
- **Root cause:** gsd-code-fixer creates and commits to `gsd-reviewfix/*` by
  design (repo branching mode is none).
- **Rule:** after a fixer run, fast-forward main from `gsd-reviewfix/*`
  (`git checkout main && git merge --ff-only gsd-reviewfix/...`). Never expect
  the fixer to have touched main directly.

---

## Run-context entries (this run's environment facts)

### L7 — Preflight: origin already synced, baseline green (2026-07-08)
- **Fact:** at run start, HEAD `2478493` == origin/main; latest CI on main is
  green (run 28946982136). The playbook's authoring-time claim that Phases 1–2
  were "never pushed" is STALE — the tree was pushed since.
- **Rule:** trust the live checks over playbook prose. Verify origin/main == HEAD
  and CI green with `gh run list --branch main` before each phase branches off.

### L8 — Real provider key inventory is SACRED (2026-07-08, Phase 9 guardrail)
- **Fact:** the live accounts carry REAL keys that predate this run and must
  NEVER be deleted:
  - GitHub (castocolina): probe `gh api user/keys` + `gh api user/ssh_signing_keys`.
  - GitLab (casto.dev): `/home/ramon/.ssh/castocolina` (rsa, auth_and_signing),
    `user-zero@pop-os` (ed25519, auth_and_signing),
    `castocolina@gmail.com` (ed25519, auth).
- **Rule:** Phase 9 cleanup deletes ONLY keys whose title carries the
  `gitid-e2e:` marker, matched by provider inventory ID. Never delete a key the
  test run did not create. Post-suite sweep asserts zero `gitid-e2e:` keys
  remain; any real key touched = destructive-anomaly HALT (ground rule 7).

### L9 — A subagent killed mid-Write corrupts the file it was rewriting (2026-07-08, Phase 3 replan)
- **Symptom:** the `gsd-planner` replanning Phase 3 died on an API/session-limit
  error (`403 socket closed`) while rewriting `03-01-PLAN.md` with Write. The
  file survived with GOOD content but a corrupted tail: stray tool-protocol
  closing tags and a diff-summary line (`25 -16`) were appended after the real
  final line.
- **Root cause:** the agent had no Edit tool available, so it rewrote whole
  files with Write; when the API connection dropped mid-emission, the partially
  streamed tool-call scaffolding landed in the file body. Line count alone did
  NOT reveal it — the file GREW (214 -> 223), so a pure truncation check passes.
- **Rule:** after ANY agent that writes files, verify BOTH ends: `wc -l`
  (truncation, L5) AND `tail -5` / last-line structure (corruption). For plan
  and doc files, assert the final line is the expected closing tag. Prefer
  giving file-editing agents the Edit tool and instructing surgical edits over
  whole-file rewrites. On a mid-write agent death, always inspect
  `git diff` of the touched file before trusting or discarding it — the content
  may be salvageable with only the tail removed (it was, here).

### L11 — Parallel executors cannot commit independently when hooks lint the whole module (2026-07-25, Phase 3 Wave 1)
- **Symptom:** Wave 1 ran 03-01 and 03-02 in parallel on disjoint FILES. 03-01
  finished its code but could not commit: the pre-commit hooks run
  `make fmt` + `make lint` over the WHOLE module (`pass_filenames: false`), and
  the parallel 03-02 executor had the module mid-rename (non-compiling). With
  `--no-verify` forbidden, 03-01 was structurally blocked from committing. It
  correctly chose to wait — and then the watchdog killed it, so its work
  survived only as uncommitted working-tree changes.
- **Root cause:** "disjoint files" is NOT sufficient for parallel execution when
  the commit gate is module-wide. The true unit of isolation is the BUILD, not
  the file set.
- **Rule:** when pre-commit hooks validate the whole module, either (a) run
  plans that touch the same Go module SEQUENTIALLY, or (b) give each parallel
  executor its own git worktree, or (c) have parallel executors write code but
  let the ORCHESTRATOR do a single reconciled commit once the module builds
  again. Never plan a parallel wave whose members can leave the module
  non-compiling for each other. Re-examine every later phase's wave plan for
  this same trap before executing it.

### L12 — A plan can be internally complete and still leave an unbuildable seam for its own tests (2026-07-25, Phase 3 03-02)
- **Symptom:** plan 03-02 correctly specified `tuikit.NewApp(b Backend)`,
  `dummytui.FixtureBackend`, and the rewired dummy entry point — but did not
  address that `internal/tuikit`'s ~48 EXISTING tests are internal
  (`package tuikit`) and call `NewApp()` with no argument. They cannot import
  `dummytui` for a fixture Backend because `dummytui` imports `tuikit`
  (import cycle), and they cannot move to an external test package because they
  reach unexported symbols.
- **Root cause:** the plan reasoned about the PRODUCTION import graph and the
  no-backend gate, but not about the TEST import graph the same refactor
  implies. Both external review and the plan checker missed it too — they were
  checking the production boundary.
- **Rule:** when a plan extracts a package behind a new injected seam, it must
  state explicitly how the extracted package's OWN tests obtain that seam, and
  whether that creates an import cycle with the fixture provider. Add this to
  the plan-review checklist for every later extraction phase.

### L10 — Literal closing-tag sequences in a subagent prompt truncate the prompt (2026-07-08, orchestrator)
- **Symptom:** an Agent spawn silently received a truncated prompt: the launch
  succeeded but the instructions were cut off partway, because the prompt text
  itself contained a literal tool-protocol closing tag sequence while warning
  the agent not to emit one.
- **Root cause:** the prompt parameter is delimited by those same tag
  sequences, so embedding one literally ends the parameter early. The failure
  is silent — the agent starts work on a partial brief.
- **Rule:** never write literal tool-protocol tag sequences (closing tags named
  `content`, `invoke`, `parameter`, `function_calls`) inside a subagent prompt.
  Describe them by name instead ("a closing tag named content"). If an agent is
  launched with a suspect prompt, TaskStop it immediately and verify the tree is
  unchanged before relaunching.
