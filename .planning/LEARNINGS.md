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
