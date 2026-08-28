---
phase: 8
reviewers: [codex-sol, xai-grok]
reviewed_at: 2026-08-28T02:47:34Z
plans_reviewed: [08-01-PLAN.md, 08-02-PLAN.md, 08-03-PLAN.md, 08-04-PLAN.md, 08-05-PLAN.md, 08-06-PLAN.md, 08-07-PLAN.md, 08-08-PLAN.md]
models:
  codex-sol: "openai/gpt-5.6-sol-fast (reasoning=low)"
  xai-grok: "xai/grok-4.6 (reasoning=low)"
model_sources:
  codex-sol: "pinned"
  xai-grok: "pinned"
---

> **codex-sol failed with a configuration error, not a timeout or empty response.**
> The Codex CLI on this host is authenticated with a ChatGPT account, which rejects
> the instance's pinned model: `The 'openai/gpt-5.6-sol-fast' model is not supported
> when using Codex with a ChatGPT account.` (HTTP 400, returned immediately, twice).
> This is a `.planning/config.json` → `review.reviewer_instances.codex-sol.model`
> problem, not a repo or plan problem — it needs a model this Codex CLI login can
> actually serve, or a different account/API-key auth mode, before it can review
> anything. Only **xai-grok** (OpenCode CLI, `xai/grok-4.6`) produced a real,
> source-grounded review this cycle.

# Cross-AI Plan Review — Phase 8

## Codex Review (codex-sol)

**FAILED — configuration error, not a review.**

```
ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a ChatGPT account."}}
```

No findings were produced. Fix the pinned model (or the Codex CLI's auth mode) in
`review.reviewer_instances.codex-sol` and re-run before treating this lane's
absence as "no concerns."

---

## OpenCode Review (xai-grok)

Verified against live source: `cmd/gitid/wiring.go:667-713`, `cmd/gitid/main.go:105-106`, `internal/doctor/doctor.go:24-108,251-272`, `internal/tuikit/frame.go:58-71` / `app.go:643-649`, `internal/doctor/checks/orphans.go:50-85`, `coherence.go:87-155`, `internal/sshconfig/include.go:127-171`, `internal/identity/state.go:100-119`.

### 1. Summary

The wave set is the right architecture: **tracer first**, `doctor.Run()` as the sole findings source, D-09 surgical rewrite isolated in Wave 2, false-positive loop closed before new checks, then render/CLI/DLV gates. Research corrections (archived `cmd/gitid/doctor.go`, two pipelines, reserved-path registry already in `sshconfig`) are baked in. Main risks: **`SeverityCritical` semantics**, **MGR-07 badge vs `row.State` vs `Findings`**, **tuikit importing doctor vs Backend seams**, **Class-1 copy lying about git-only delete**, **report-only shadow copy pointing at Fixer**, and **08-01 JSON array vs 08-07 versioned envelope**. Overall: **MEDIUM** — executable if those are pinned before Wave 1.

### 2. Strengths

- **Sole-source decision is correct.** `InitialState()` still synthesizes `DemoFinding` from `h.Problems` (`wiring.go:680-689`); `Run()` is unwired (`grep` of cmd/tuikit is empty). Replacing that loop, keeping `row.State = collapseState(h)` (`wiring.go:679`), matches `state.go:100-104`.
- **Tab split matches code.** Four tabs, `TabDoctor`, comment still says Fixer is not a tab (`frame.go:58-71`); `newScreens` is `[4]screenModel` with `newDoctorModel()` (`app.go:643-649`); `DemoBanner` is true only for `TabDoctor` (`wiring.go:706-713`).
- **CLI stubs are real.** `newReservedNounCmd("health"/"fix")` at `main.go:105-106`.
- **D-09 detection gap is real.** Check 3 only reads `deps.ManagedHosts` (`coherence.go:104-155`). Hand-written Hosts need `AllHostBlocks`.
- **Class-1 false positive is live.** Warning + `RemoveBlock` (`orphans.go:68-85`) for any SSH block without gitconfig — including `--git-only` and SSH-only-by-design.
- **D-06.2 registry already exists.** `ReservedPaths` / `IsReservedPath` (`include.go:127-171`); Wave 3 Task 2 correctly says apply at `KeyPaths` collection, not invent a registry.
- **includeIf missing fragment already exists** as Coherence Check 2 (`coherence.go:87-100`). Wave 5 Task 3 is the right place to prove one row.
- **Wave 8 human gate** for REQUIREMENTS.md is appropriate (`autonomous: false`).
- **Threat models** (T-08-04/05/13/14) match STORE-04 / CR-18 / hermetic HOME.

### 3. Concerns

#### HIGH

- **`SeverityCritical` meaning vs HLTH-02.** `doctor.go:31-32`: *"key/secret exposure"*. Plans 03/07 use critical for parse failure. `ExitCode` 3 then means parse error, not exposure. Pin a comment + test before Wave 3, or parse stays `SeverityError` and the parse-error **frame** is the UI, not the enum.
- **MGR-07 is not "unchanged."** Manager badges today mix `collapseState(h)` (inventory) and `state.Findings` (Problems). Doctor families (perms, agent, redundancy) will change per-identity badges vs Phase 5. Wave 1 Task 3 must assert **badge + `row.State`**, not only Findings count.
- **`tuikit` must not import `internal/doctor`.** Wave 1/2 screens stay render-only; conversion stays in `cmd/gitid`. Plan says this in 08-01 done-criteria — **Task 1 file list must not put mapping helpers in tuikit.**
- **Dynamic `PlanFor` vs frozen dummy.** 08-02 Task 2: dummy `PlanFor` frozen; real diffs need a **Backend** method (like `GitWritePlan`). Putting real-file reads in `internal/tuikit/fixplans.go` breaks dummy + no-backend gate.
- **Class-1 copy vs A1.** Downgrade is *any* well-formed SSH-without-git (SSH-only create, not just `--git-only`). Frozen copy "deliberately removed (git-only delete)" is false for incomplete identities. **HIGH UX/honesty.**

#### MEDIUM

- **Wave 1 `Run()` with nil CheckFns.** `Run` skips nil (`doctor.go:267-269`). Task 1 can "wire" Deps and still run only Coherence. Task 1 done-criteria ("all 9 families") fights Task 2. Split truths: Task 1 = one Coherence finding E2E; Task 2 = 9 non-nil CheckFns.
- **JSON contract churn.** 08-01: array; 08-07: versioned envelope (ssh/git). Scripts/tests from Wave 1 break. Envelope in Wave 1, or call Wave 1 "unstable."
- **`Fix.Fn` vs `Interactive`.** `FixDescriptor.Interactive` is preferred in archived CLI. 08-02/07 `--yes` must document Fn vs Interactive (gitignore / baseline).
- **Shadow SuggestedFix vs Fix:nil.** 08-05: *"available on the Fixer screen"* but `Fix: nil`. Users hit Fixer and see nothing. Copy must be advisory-only.
- **D-13 `maxPasses` in TUI Persist.** TUI applies one fix per ceremony; `maxPasses` belongs on CLI `fix --yes` / batch, not every `FixFinding`.
- **Parse-pause only at render.** `--json` still dumps downstream findings from unparseable files unless Wave 3 also filters CLI/JSON.
- **TabID 4→5 vs dummy goldens.** Dummy `FixtureBackend` is 4-tab chrome until Wave 8 allowlist. Wave 1 must update dummy tab count or visual gate fails early.
- **08-02 CLAUDE.md scoped amendment** is D-09 required but not in `files_modified`.
- **Wave 4 gitignore fix in `checks`.** Closures calling `WriteGlobalGitignore` pull write deps into checks unless injected like `RemoveBlock`/`AddWiring`. Prefer injected `FixExcludesfile` on `Deps`.

#### LOW

- `family|title` IDs collide (two Host *). Plan says disambiguate with IdentityName; also need host pattern / path.
- `doctor --fix` flag mapping still "discretion" in 08-07 — pin `--fix` ⇒ `fix --yes` or require `--yes` still.
- `newScreens` has no covering tests (codegraph). Wave 1 TabID tests must be exhaustive-switch.
- known_hosts backlog is correctly excluded.

### 4. Suggestions

1. Wave 1: add `Finding.Target`; `defaultTargetForFamily`; conversion **only** in `cmd/gitid`; Backend `FindingsFromDoctor`; dummy keeps fixture `HealthFindings`.
2. Add Backend `FixPlan(finding) WritePlanView` in Wave 2 — do not teach tuikit to read `~/.ssh/config`.
3. Amend `SeverityCritical` docs **or** keep parse as Error + dedicated frame.
4. Class-1: copy = "SSH-only / no Git partner — informational; not removed automatically." Fixture: `--git-only` **and** SSH-only create.
5. Shadow/directive-above: SuggestedFix without "Fixer screen."
6. Wave 1 JSON = same envelope as `cmd/gitid/ssh.go`; Wave 7 only adds alias/exit/parity.
7. `buildDoctorDeps`: nil-guard for 9 CheckFns in Wave 1 test (fail if nil), even if Task 2 fills them.
8. Filter `--json` when Files-critical for that section (same as TUI pause).
9. List `.planning/design/fixer/FIELDS.md` **and** `AGENTS.md`/`CLAUDE.md` D-09 note on 08-02 `files_modified`.
10. Wave 5 Task 3: assert Family **Coherence** (Check 2), zero Orphans Class-2 for the same missing fragment (Class-2 is git-without-SSH, not missing file).

### 5. Risk Assessment

**Overall: MEDIUM**

Justification: sequencing, substrate, and D-09/D-10/D-14 are sound and source-backed. Residual risk is **wiring/shape** (severity enum, badges, Backend vs tuikit, JSON, Class-1 honesty), not missing phase goals. HLTH-01–06 / FIX-01/02 / DLV-04/06 are covered if Waves 1–2 land the chokepoints and Wave 8 stays blocking-human.

Per-plan: **08-01 MEDIUM** (load-bearing, several landmines) · **08-02 HIGH** (hand-written write; mitigations specified) · **08-03 MEDIUM** (copy + SeverityCritical) · **08-04 LOW–MEDIUM** · **08-05 MEDIUM** (copy + Pitfall 6) · **08-06 LOW–MEDIUM** · **08-07 LOW** · **08-08 LOW** (process-heavy, well-precedented).

---

## Consensus Summary

Only one reviewer (**xai-grok**) produced a grounded review this cycle — **codex-sol
failed with a configuration error** (see banner above) before reading any plan
content, so there is no second independent voice to agree or disagree with. Treat
everything below as xai-grok's single-reviewer findings, not a cross-AI consensus,
until codex-sol (or another reviewer) is re-run successfully.

### Agreed Strengths

Not applicable this cycle — only one reviewer completed. xai-grok's source-verified
strengths (sole-source doctor decision, tab-split fidelity, real CLI stubs, D-09/D-06.2
gaps confirmed live in code) stand on their own; re-run codex-sol to corroborate.

### Agreed Concerns

Not applicable this cycle for the same reason. xai-grok's five HIGH concerns —
`SeverityCritical` semantics vs HLTH-02, MGR-07 badge/state drift, tuikit importing
`internal/doctor`, dynamic `PlanFor` vs the frozen dummy, and the Class-1 copy
misrepresenting git-only deletes — are single-reviewer findings, each backed by
`file:line` citations, and should be treated as open items pending a second opinion.

### Divergent Views

None recorded — no second reviewer ran to diverge from xai-grok.

**Action needed before this review can be called complete:** fix
`review.reviewer_instances.codex-sol.model` (or the Codex CLI auth mode) and
re-run `/gsd-review 8` to get the second independent pass this phase's plans
were meant to receive.
