---
phase: 7
reviewers: [codex-sol, xai-grok]
reviewed_at: 2026-08-27T12:12:48Z
plans_reviewed: [07-01-PLAN.md, 07-02-PLAN.md, 07-03-PLAN.md, 07-04-PLAN.md, 07-05-PLAN.md, 07-06-PLAN.md]
models:
  codex-sol: "openai/gpt-5.6-sol-fast (reasoning=low)"
  xai-grok: "xai/grok-4.6 (reasoning=low)"
model_sources:
  codex-sol: "pinned"
  xai-grok: "pinned"
---

# Cross-AI Plan Review — Phase 7

> **codex-sol lane failed**: the configured instance model `openai/gpt-5.6-sol-fast`
> is rejected by the local Codex CLI's ChatGPT-account auth
> (`"The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a
> ChatGPT account."`, HTTP 400, returned twice). No review content was produced for
> this lane — see `## Codex Review (codex-sol)` below for the captured stderr. This
> is a reviewer-instance configuration issue (`review.reviewer_instances.codex-sol`
> in `.planning/config.json` or equivalent), not a plan defect. Consider repointing
> `codex-sol` at a model this Codex CLI installation's auth actually supports, or
> dropping it from `review.default_reviewers` until it does.

## Consensus Summary

Only one reviewer (xai-grok, via OpenCode / Grok-4.6) produced a source-grounded
review; codex-sol failed to run (see note above). Consensus in the strict sense
(2+ reviewers agreeing) cannot be established from a single successful lane — the
findings below are xai-grok's alone, weighted as one grounded, source-cited voice,
not as agreed-by-multiple-reviewers consensus.

xai-grok's review is well-grounded: every finding cites concrete `file:line`
evidence (`internal/gitconfig/baseline.go`, `internal/tuikit/globalgit.go`,
`docs/cli-parity-matrix.md`, `cmd/gitid/wiring.go`, etc.) and traces plan claims
against the actual current code rather than restating the plan text. Overall
verdict: **phase risk MEDIUM**, concentrated in the wave-1/wave-2 write and
first-run/adoption logic (07-01, 07-02), not in the later UI/CLI/gate plans
(07-04, 07-05, 07-06 assessed LOW–MEDIUM).

### Agreed Strengths
Not applicable — only one reviewer produced findings, so no strength was
independently corroborated by a second reviewer. (See xai-grok's per-plan
Strengths sections for its own assessment.)

### Agreed Concerns
Not applicable for the same reason. The single reviewer's highest-severity
(HIGH) findings, which the orchestrator should treat as the most actionable
items from this cycle, are:

- **07-01 — Wave-1 apply of pre-checked fixture rows (HIGH).** `newGlobalGitModel`
  (`internal/tuikit/globalgit.go:48-54`) pre-selects every `NeedsAction` key except
  D9, but Task 2 rejects unknown keys at plan time. Until 07-03 fills in
  `PolicyFor`, applying the default selection on the real binary will either be
  refused outright or write a single-key block that **strips the rest of a real
  POC baseline**. The plan never states that wave-1 Apply must scope itself to
  keys present in the live policy table.
- **07-01 — Idempotent skip vs. required fresh backup on second identical apply
  (HIGH).** `WriteBaselineInclude` / `WriteBaselineFile`
  (`internal/gitconfig/baseline.go:624-626`, `562-564`) skip the write and return
  an empty backup path when bytes are unchanged, but the acceptance criteria (SC-1)
  want a fresh backup on a second identical apply. These two behaviors conflict
  unless `filewriter.Write` is forced regardless of byte-equality, and the plan
  does not resolve which one wins.
- **07-02 — Missing anchor: `InsertBlockAfter` on a not-yet-created floor include
  (HIGH).** `InsertBlockAfter` errors when the `baseline-include` sentinel does not
  exist yet, but the wave-2 fallback-author write path does not call
  `WriteBaselineInclude` first. A first-run "set the fallback author before any
  global-git apply" sequence will fail, unlike SSH's equivalent apply path
  (`internal/sshconfig/lifecycle.go:908-928`), which creates the include line when
  needed.

### Divergent Views
Not applicable — with only one successful reviewer this cycle there is no second
independent view to diverge from. Re-running with a corrected codex-sol model (or
another reviewer) would let a future cycle populate this section meaningfully.

---

## Codex Review (codex-sol)

Lane invocation succeeded (Codex CLI launched, connected to the workdir, received
the full review prompt), but the ChatGPT-account auth on this Codex CLI installation
rejected the configured model before any review content could be generated:

```
ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a ChatGPT account."}}
```

No findings were produced by this lane. See the note under Consensus Summary above
for the recommended remediation (repoint the instance's model or drop it from
`review.default_reviewers` on hosts using ChatGPT-account Codex auth).

---

## OpenCode Review (xai-grok)

I'll verify plan claims against the recipes and the live code, then write a structured review.

# Phase 7 plan review (07-01 … 07-06)

Verified against `recipes/gitconfig.recipe:66-67` (floor `[include]`), `internal/gitconfig/baseline.go:537-571` (`WriteBaselineFile` + `ReplaceBlock(..., "baseline", …)`), `reader.go:34-49` (`IsReservedBlockName` today only `baseline-include` + rewrite prefix), `cmd/gitid/wiring.go:679-682` (`DemoBanner` still on Global Git), `internal/tuikit/globalgit.go:58` (noop `activate`), `docs/cli-parity-matrix.md:42` (`deferred Phase 7`).

---

## 07-01 — Tracer / floor include / one ceremony

### Summary
Strong Phase-6 mirror: UI-free `internal/globalgit`, `EnsureGlobalGit` as single block owner, journaled `runGlobalGitApply`, `GlobalGitPlanner`. D-01 floor include matches the recipe. Highest risk is **wave-1 apply vs a still-full fixture selection**, plus **SC-1 skip-write vs "fresh backup on second apply"**.

### Strengths
- Floor `[include]` via existing `WriteBaselineInclude` (`baseline.go:610-622` → `PrependBlockIfNotFound`) is the right last-wins model.
- Legacy name is live: `WriteBaselineFile` uses sentinel `"baseline"` (`baseline.go:551`); reserving both names in the same commit matches L4 / SSH `IsReservedBlockName` (`sshconfig/include.go:94-95`).
- `-z` probes, non-repo cwd, `GIT_CONFIG_NOSYSTEM` left unset, value-before-source classify — all match D-03 / Phase 6.
- D-11.2: reject `core.excludesfile` instead of hoping `RenderBaselineBlock:424` goes unused.
- Sync `activate()` matches `globalgit.go:58` and Global SSH (`backend.go:91-107`).

### Concerns
- **HIGH — Wave-1 apply of pre-checked fixture rows.** `newGlobalGitModel` (`globalgit.go:48-54`) pre-selects every `NeedsAction` key except D9. Task 2 rejects unknown keys at plan. Until 07-03 fills `PolicyFor`, **Apply on the real binary will refuse the default selection** (or write a one-key block and **strip the rest of a real POC baseline**). Plan never says: ceremony only submits keys in the live policy table / disable other checkboxes in wave 1.
- **HIGH — Idempotent skip vs required fresh backup.** `WriteBaselineInclude` / `WriteBaselineFile` skip write and return empty backup when bytes match (`baseline.go:624-626`, `562-564`). Acceptance wants a **new backup on second identical apply**. That fights SC-1 unless `filewriter.Write` is forced even when unchanged. Unspecified.
- **MEDIUM — `WriteBaselineFile` still owns URL rewrites** (`baseline.go:552-560`). Repointing the baseline half to `EnsureGlobalGit` is right; a careless signature change will break rewrite tests and the archived POC under `.planning/archive/…/cmd-gitid/baseline.go`.
- **MEDIUM — `go list -deps` allowlist includes `internal/filewriter` for a never-writes package.** Easy to grow a write later; pin "test files only" or drop `filewriter` from the allowlist.
- **LOW — Isolation contract quoting real `git` output** is good; claimed `-z` layout (`<scope> NUL <origin> NUL <key> LF <value> NUL`) is not pinned as a parser golden in the plan itself.

### Suggestions
- Wave 1: `runGlobalGitApply` / TUI apply only keys present in `PolicyFor`; leave other rows display-only.
- Spell the second-apply backup rule: always `filewriter.Write` after confirm, **or** change the test to "backup only if bytes changed" and keep SC-1.
- Keep `RenderURLRewritesBlock` / rewrite `ReplaceBlock` on `WriteBaselineFile`; only delete `RenderBaselineBlock` / `BaselineConfig`.

### Risk
**HIGH** until wave-1 selection/write scope is pinned; architecture otherwise **MEDIUM**.

---

## 07-02 — D9 two-field fallback

### Summary
Correct user override (two fields, empty = unset) and correct placement (`InsertBlockAfter` the floor include). D-04 vs UI-SPEC "partial" resolved by seeding. Gap: **fallback write if the floor include does not exist yet**.

### Strengths
- `InsertBlockAfter` documented against append-after-`includeIf` (`gitconfig.recipe:66-136`).
- Empty pair → `RemoveBlock`; never write empty-string keys.
- Separate `GitFallbackAuthorPlanner` / `runGitFallbackAuthorApply` matches D-05 and SSH storage-vs-options split (`backend.go:91-93`).
- D-06 matched/unmatched/`matchedDir` empty = not-verifiable is the right three-way outcome.
- Reserved name in the same commit as the owner (L4).
- Dummy pane shape change recorded for 07-06 instead of fake byte-identity.

### Concerns
- **HIGH — Missing anchor.** `InsertBlockAfter` errors if `baseline-include` is absent. Wave-2 write does **not** call `WriteBaselineInclude`. First-run "set fallback before any global-git apply" fails. SSH apply creates the include line when needed (`lifecycle.go:908-928`); this plan does not.
- **MEDIUM — Independent per-field apply vs one ceremony with both fields.** Discussion wanted independent apply; UI-SPEC wants one ceremony of "what is on screen." Seeding makes that consistent, but CLI in 07-05 uses explicit set/clear flags — TUI cannot clear one half without applying the other half's current seed. Call that out so 07-05 does not "fix" the TUI.
- **MEDIUM — `global-git-author` vs identity aliases.** Collision argument is thin; reserved registry is the real guard — keep the orphans fixture.
- **LOW — T-07-12 (guessed-name) deferred to 07-03** is fine if the email-only path cannot ship to users before 07-03 (sequential waves).

### Suggestions
- If anchor missing: `WriteBaselineInclude` first (same journal), then insert; add that lifecycle test.
- Guard comment: TUI apply is "pair snapshot," CLI is "explicit flags"; both call the same `EnsureGitFallbackAuthor`.

### Risk
**MEDIUM** (anchor/first-run). Placement and empty-means-unset are solid.

---

## 07-03 — Twelve honest rows

### Summary
This is where GGIT-01 becomes true. Pinned table, hard vs informational gates, conservative unreadable → `diff3`, D-10 tally vs Phase 6, delete `ScanConflicts`/`BaselineKeySet` in favor of probes — all match the repo (`BaselineKeySet` still emits `core.excludesfile` at `baseline.go:52-60`; production callers of `ScanConflicts` are effectively none).

### Strengths
- Recipe-sourced aliases/colors (`gitconfig.recipe` default block).
- Hard gate only on `merge.conflictstyle`; unreadable treated as below-gate (correct vs `GitVersionAtLeast` optimistic true).
- Two `[user]` sections called out (baseline `useConfigOnly` vs fallback author).
- Fixture/policy parity test; zdiff3 correction in Go + `recipeFixtures.ts` in one commit.
- `Selectable()` as single toggle/glyph/click predicate copies SSH (`views.go` contract).
- Copy-freeze path adds `internal/globalgit`; dynamic exclusions proven by deliberate fail.

### Concerns
- **MEDIUM — Deleting `ScanConflicts`/`BaselineKeySet`.** Tests and archive POC still use them (`baseline_test.go`, archive `cmd-gitid`). Plan `rg` scope is `internal/ cmd/ e2e/` — archive may still compile depending on modules. Confirm `./...` after delete.
- **MEDIUM — `core.pager`.** Deferred in CONTEXT; current `RenderBaselineBlock` may still emit it. Selection-driven compose **drops pager on first real write** unless "keep if already present." Plan 07-01 compose-from-selection already implies drop; 07-03 should say "pager is not a row and is not preserved."
- **LOW — Bundle apply writes colliding aliases** (T-07-19 accept) needs the promised post-apply resolve test in Task 2, not only compose-emits-all-keys.
- **LOW — `user.useConfigOnly` in the included baseline file** vs fallback `[user]` in `~/.gitconfig`: last-wins is per file order; include is first, so a later user `useConfigOnly` still wins. Classifier must use origin path, not "key exists in baseline file."

### Suggestions
- Explicit pager policy on rewrite.
- Expand delete `rg` to archive or exclude archive from the Go build.
- Lifecycle test: user `alias.co` after include still wins after bundle apply (`git config --show-origin`).

### Risk
**MEDIUM**. Values and gates are the phase's write-risk; tests listed are the right ones.

---

## 07-04 — Scroll, error, PTY

### Summary
Implements frozen overflow/error/loading. Click-at-offset is the right fear (`handleClick` today is screen-row only). PTY-on-compiled-binary matches DLV-04/06. Banner drop at end of Task 2 is correctly ordered after 07-03 honesty.

### Strengths
- Measured `frameBodyRows`, no hardcoded budget (02-15 lesson).
- Cue both-edges → down; SSH advisory sentence reused.
- Fake git shim + dedicated proof test (06-04 indented-pattern / wrong-arrow bugs).
- Write cases assert files + backup path, not only frames.
- `DemoBanner` (`wiring.go:681`) actually includes `TabGlobalGit` today.

### Concerns
- **MEDIUM — Failure-and-retry PTY** needs a real inject path in the compiled binary (env/`failCommitAt` is test-only on `realBackend`). Unspecified how production binary fails mid-write in e2e.
- **MEDIUM — Shim vs real git for writes.** Version override must not break `git config` writes; plan says delegate — keep the harness proof from matching `git version` only.
- **LOW — Ceremony viewport widen** could change SSH ceremony layout if they share `ExactTextViewport` budget. Assert Global SSH preview unchanged or scope the budget to Global Git.

### Suggestions
- Document the commit-failure e2e hook or drop that case if it cannot be triggered without test-only APIs.
- Capture dummy vs real scroll-cue frames here for 07-06 (dummy six-row list may never cue).

### Risk
**MEDIUM** (harness/failure injection). Scroll/click design is **LOW** if offset tests land.

---

## 07-05 — CLI + §J

### Summary
Right frozen tree (two nouns, two ceremonies). Explicit set/clear flags are the correct CLI reading of D-04. Parity matrix row at `docs/cli-parity-matrix.md:42` is real. D-11.1 §J correction belongs here, status flip delayed to 07-06.

### Strengths
- Source-level "verbs call ceremonies only" (SSH `TestSSHWriteVerbsCallSharedCeremonyByConstruction`).
- Apply uses `Selectable()` so differs keys refuse (D-02).
- Empty `--name` refused (not silent clear).
- Adaptive depth + `NewAppOnGlobalGit` (06-06 surprise).
- Version substitution advisory from ceremony plan stage, not CLI.
- One `gitWriteExitCode`; envelope on refusal.

### Concerns
- **MEDIUM — `options apply` key identity.** Policy rows vs git keys vs display keys (`core.autocrlf / core.eol`, alias bundle). Plan says `<key>...` but not whether users pass row ids or member keys. Bundles need one token.
- **MEDIUM — Incomplete invocation** opening TUI with empty selection vs TUI default pre-checks (`globalgit.go:48-54`). "Nothing pre-selected" fights the screen's own defaults unless `NewAppOnGlobalGit` clears `chosen`.
- **LOW — `fallback set` with only `--clear-email`** must read current name and rewrite — needs a readable `~/.gitconfig` before plan; say so (headless vs missing file).

### Suggestions
- Freeze apply argv as **display row keys** from the 07-03 table; reject member keys with "use `alias` not `alias.lg`".
- Test `NewAppOnGlobalGit` actually empty `chosen`.

### Risk
**MEDIUM** (key naming / TUI preselect). Contract shape is **LOW**.

---

## 07-06 — Visual gate + close GGIT-01

### Summary
Correct closer: classify real vs dummy, four negative controls, packet from per-plan summaries, battery recorded not asserted, GGIT-01 only after green. Known divergences listed from prior summaries. Orchestrator-not-executor for cross-AI review is honest.

### Strengths
- Probe-error as live non-applicability (dummy `FixtureBackend` never runs git).
- Re-verify 07-03 fixture amendments before allowlisting zdiff3/12th row.
- Multi-byte truncation hash (arrows + cue).
- Makefile filter: repo root + scan from target (06-07 bugs).
- Requirement index not flipped in 07-05.

### Concerns
- **MEDIUM — Dummy after 07-02/07-03 may already match** two-field pane and zdiff3; if 07-01 heading (resolved baseline path) is the only real divergence, allowlist must not keep dead rows.
- **LOW — In-process capture vs PTY frames** are different questions; manifest must not treat screenshot registry hashes as PTY evidence for DLV-04.
- **LOW — Closure-table parser** over SUMMARY markdown will drift; pin a `## Deviations` / `## Review` heading convention in 07-01…05 output (those files do not exist yet).

### Suggestions
- Require 07-01…05 SUMMARY headings the parser will grep.
- Battery skip policy: worktree-only skips recorded as skip, not pass (06-07).

### Risk
**LOW** for the gate; **MEDIUM** only if allowlist is written from the plan's guessed list instead of re-measured diffs.

---

## Phase-level

| Goal | Covered? |
|------|----------|
| GGIT-01 explained global git options | 07-03 (+ 07-04 render) |
| Backup + managed block + confirm + verbatim outside | 07-01 / 07-02 ceremonies |
| DLV-04/06 PTY + dummy compare | 07-04 PTY, 07-06 gate |
| Recipe floor include | 07-01 D-01 |
| GITIGNORE not falsely closed | 07-05 D-11.1, close in 07-06 |

**Do not execute 07-01 until:** (1) wave-1 cannot strip a full POC baseline or refuse the default checkbox set; (2) second-apply backup vs SC-1 is one rule; (3) fallback-without-include is owned (07-02 or 07-01 pre-create include).

**Overall phase risk: MEDIUM**, driven by write/adoption in 07-01–02, not by later UI/CLI/gate plans.
