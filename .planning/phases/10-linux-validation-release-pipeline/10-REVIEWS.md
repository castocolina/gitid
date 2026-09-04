---
phase: 10
reviewers: [opencode-plan-review]
reviewed_at: "2026-09-04T21:54:13Z"
plans_reviewed:
  - .planning/phases/10-linux-validation-release-pipeline/10-01-PLAN.md
  - .planning/phases/10-linux-validation-release-pipeline/10-02-PLAN.md
  - .planning/phases/10-linux-validation-release-pipeline/10-03-PLAN.md
  - .planning/phases/10-linux-validation-release-pipeline/10-04-PLAN.md
  - .planning/phases/10-linux-validation-release-pipeline/10-05-PLAN.md
  - .planning/phases/10-linux-validation-release-pipeline/10-06-PLAN.md
models:
  opencode-plan-review: "router-env/my-plan-review (reasoning=high)"
model_sources:
  opencode-plan-review: "pinned"
---

# Cross-AI Plan Review — Phase 10

> **Infra note (session-scoped):** the four standard CLI lanes for this workflow
> (`opencode-plan-review`'s default configuration, `opencode-sol`, `codex`,
> `antigravity`) were briefed earlier today as down/exhausted on this
> branch/session for infrastructure reasons unrelated to plan quality. On
> investigation, `opencode-plan-review`'s configured model id
> (`review.reviewer_instances.opencode-plan-review.model` =
> `local-llm-env/my-plan-review` in `.planning/config.json`) does not match any
> model `opencode models` actually reports — the correct id is
> `router-env/my-plan-review`. Invoking with the corrected model id directly
> (bypassing the stored misconfiguration) succeeded and produced a real,
> source-grounded review below. **`gemini`, `qwen`, and `cursor` were not
> installed on this host and were not attempted; `claude` was skipped for
> reviewer independence (this review is being orchestrated from a Claude Code
> session).** This is a single-reviewer review, not a multi-reviewer
> convergence pass — no cross-reviewer corroboration was possible this cycle.
> The `.planning/config.json` model-id typo (`local-llm-env` →
> `router-env`) should be corrected outside this review so future `/gsd-review`
> runs do not require this manual workaround.

## OpenCode Review (opencode-plan-review)

I'll verify the plans against the actual repository state before assessing.

## Summary

Six plans, four waves, verified against the live repo at `fbf833e`. The plan set is unusually well-grounded: nearly every file, symbol, line range, and test name it cites actually exists as described. I spot-checked ~25 concrete claims and found **only one factual error** (a stale test-name reference), which is remarkable for a plan of this size.

The architecture is sound: goreleaser replaces a hand-rolled pipeline, the artifact-shape migration is correctly isolated into a single plan (10-05) rather than smeared across the phase, and 10-04 Task 1 explicitly defers `checksums`/`build-cross` retirement to keep every intermediate commit green. That is exactly right and shows the highest-risk item (RESEARCH Pitfall 1) was internalized, not just noted.

What the plans *don't* handle well is the **version-string contract across the three build paths** and the **test-seam divergence in `install.sh`**. Both are silent-failure classes: they compile, they pass CI, and they produce wrong output only in production. Those are the findings worth acting on before execution.

**Verdict: proceed with 10-01/10-02/10-03 as written; fix C-1 and C-2 in 10-01 and 10-05 respectively before executing wave 2+.**

---

## Verification of `fbf833e` (the user's specific ask)

**Confirmed present and structurally correct.** `git show fbf833e` changed exactly 5 lines in `10-04-PLAN.md` plus the `REQUIREMENTS.md` BUILD-03 annotation:

- Task 2 `<action>` now mandates the comment above `release:`'s `on:` trigger with the full rationale text (no local/forged-tag path in CI; `fetch-depth: 0` checkout resolves the same commit).
- `<verify><automated>` gained `&& grep -qi "verify-tag" .github/workflows/release.yml`.
- `<fails_when>` covers the grep exiting non-zero.
- `release_yml_test.go` is required to assert the file contains `"verify-tag"`.
- `<success_criteria>` (10-04-PLAN.md:139) no longer claims D-08 is "fully implemented" — it explicitly states the clause has no goreleaser equivalent and is resolved by a documented structural argument.

**Sufficient? Yes for the blocker as filed, with one caveat.** The plan-checker's objection was "D-08's `--verify-tag` clause is silently unaddressed." That is now resolved: the resolution is explicit, recorded, and enforced. But note what the enforcement actually proves — `grep -qi "verify-tag"` asserts *a string exists in a YAML comment*. It cannot detect that someone later adds a `workflow_dispatch` trigger, which would immediately invalidate the structural argument while leaving the comment (and the test) green. See M-1 for the cheap fix.

The security reasoning itself is correct. `gh release create --verify-tag` guards a local publish flow against a stale/mis-resolved local tag; a `push: tags:` trigger with `fetch-depth: 0` checkout has no such path.

---

## Strengths

1. **Factual grounding is near-perfect.** Verified as accurate: the 9 tests slated for deletion all exist (`cmd/gitid/release_plumbing_test.go:91-251`); `TestWorkflowPinsEveryActionToACommitSHA` and `TestWorkflowTopLevelPermissionsStayReadOnly` (`:55`, `:74`) correctly identified as ci.yml-scoped survivors; helpers `workflowPath`/`readRepoFile`/`jobBlock` exist at `:11`, `:21`, `:30`; `e2e/release_e2e_test.go` has exactly 17 `func Test` (matching the plan's "17 rewritten") of which 13 are `TestInstallScript_*` (matching "13 total"); `internal/tuikit/app.go` `NewApp` at `:159` and `renderHelp` at `:689` are within the cited ranges; `sshVersionPattern` at `internal/platform/version.go:40` does handle unsuffixed input; `parityToolingExcluded` at `cmd/gitid/identity_test.go:1827` with its doc comment at `:1824`; `healthDocument`/`healthSchema` at `cmd/gitid/health.go:91-93` and `writeJSON` at `cmd/gitid/identity_read.go:360` are real mirror targets; the repo genuinely carries `poc-0.0.1` and `backup/*` tags justifying `--match "v*"`.

2. **The riskiest migration is correctly sequenced.** 10-04 Task 1 explicitly says "Do NOT touch the existing `checksums`/`build-cross` targets or their e2e coverage in this task ... keeping this task's own change set additive-only, with zero risk of leaving `make test-e2e` red between this plan and the next." This is the single most important decision in the phase and it was made correctly.

3. **The double-publish race is caught and fixed atomically.** ci.yml's `release:` job (`:126`, `if: startsWith(github.ref, 'refs/tags/v')`) and a new `release.yml` on `push: tags: v*` would race on the same Release object. 10-04 Task 2 requires deletion **in the same commit**. Correct.

4. **Zero-blast-radius TUI change.** `App.WithVersion` as a value-receiver method with a conditional render (`when a.version != ""`) leaves all existing `NewApp*` call sites byte-identical. Verified: `renderHelp` at `app.go:689` has no existing version line, and the `<fails_when>` explicitly names byte-level golden diffs.

5. **The `container:` + `dnf`-before-`checkout` ordering is right.** `fedora:latest` ships no Node.js; every first-party action needs it. Putting `dnf install` as the first `run:` step before any `uses:` is the correct and non-obvious fix.

6. **Local dry-run vehicle exists.** `make release-snapshot` (snapshot mode skips tag validation, `release:`, and `brews:`) means the whole pipeline is verifiable with zero secrets — which matters because the `user_setup` block genuinely blocks a real tag push.

---

## Concerns

### C-1 — CRITICAL: `VERSION` default embeds a leading `v`, release path strips it

`10-01-PLAN.md:90` sets `VERSION ?= $(shell git describe --tags --match "v*" --always --dirty)`. Executed live in this repo right now:

```
v0.1.0-rc.9-142-gfbf833e
```

Note the leading **`v`**. But `10-04-PLAN.md:99` computes the release-path version as `version=${GITHUB_REF_NAME#v}` — **stripped**. And `.goreleaser.yaml`'s ldflags consume `{{.Env.VERSION}}`, so:

- `make build` locally → `gitid version v0.1.0-rc.9-142-gfbf833e (...)`
- real release → `gitid version 1.0.0 (...)`

D-11 locks the output as `gitid version 1.0.0 (abc1234, 2026-07-08, darwin/arm64)` — no `v`. The local path violates D-11.

Worse, this is **designed to pass its own test**. `10-01-PLAN.md:90` replaces the e2e literal assertion with the regex `^gitid version \S+ \(none, unknown, (darwin|linux)/(amd64|arm64)\)$`. `\S+` matches `v0.1.0-rc.9-142-gfbf833e` happily. The inconsistency ships silently and green.

**Fix:** in the Makefile, strip the prefix at the source:
```make
VERSION ?= $(patsubst v%,%,$(shell git describe --tags --match "v*" --always --dirty))
```
and tighten the e2e regex to reject a leading `v` (e.g. `\(?!v\)` isn't available in RE2 — use `[^v]\S*` or assert `!strings.HasPrefix(gotVersion, "v")` separately).

*Independently confirmed by the orchestrating session: `git describe --tags --match "v*" --always --dirty` in this repo right now returns `v0.1.0-rc.9-142-gfbf833e`, matching the claim exactly.*

### C-2 — CRITICAL: the `install.sh` test seam bypasses the two behaviors most likely to break

`10-05-PLAN.md:92` specifies that when `GITID_INSTALL_BASE_URL` is set, install.sh must "skip BOTH the redirect-resolution and the versioned-path construction, fetching `${BASE_URL}/${asset}` and `${BASE_URL}/checksums.txt` directly, exactly as the current script does."

Since every e2e test uses the fixture server (i.e. always sets `GITID_INSTALL_BASE_URL`), this means **zero test coverage** for:

1. The GitHub `/releases/latest` redirect resolution + `/tag/` string-stripping — brand-new, string-munging, network-shaped logic.
2. The **real** checksums filename. Tests fetch flat `checksums.txt`; production fetches `gitid_<version>_checksums.txt` (goreleaser's default, per `10-04-PLAN.md:82`). These are *different filenames*, and only the untested one is real.

The plan's own rationale — "keeps the e2e fixture-server tests simple" — trades away coverage of exactly the two things that are new and unverified. A typo in the versioned checksums name is undetectable until a user runs the one-liner against a real release.

**Fix:** make the fixture server serve the *real* layout (`<base>/gitid_<version>_checksums.txt` + versioned archive names) so the production filename construction is exercised. Gate only the `/releases/latest` redirect behind the seam, and add one test that sets `GITID_VERSION` explicitly to drive the versioned path end-to-end without needing redirect simulation.

*Independently confirmed by the orchestrating session against 10-05-PLAN.md's Task 2 action text: it literally specifies `"${BASE_URL}/${asset}"` and `"${BASE_URL}/checksums.txt"` (flat, unversioned) for the test-seam path.*

### C-3 — HIGH: prerelease classification is dropped, and this repo lives on prerelease tags

`10-04-PLAN.md:99` deletes `TestWorkflowPrereleaseIsDerivedFromTheTag` and specifies `release: prerelease: auto` in `.goreleaser.yaml`. The retired ci.yml job (`.github/workflows/ci.yml:152-164`) had explicit `prerelease` **and `make_latest`** outputs with a documented rationale that is worth re-reading:

> "a prerelease briefly presented as the repository's latest stable download is exactly the failure a checksum-verifying installer cannot protect anyone from"

The repo's tag list is `v0.1.0-rc.1` … `v0.1.0-rc.9` — **nine consecutive prerelease tags and no stable release**. This is not a hypothetical path; it is the *only* path this repo has ever used.

`prerelease: auto` covers the prerelease flag, but `make_latest` has no goreleaser equivalent called out anywhere in the plans, and no replacement assertion exists in the new `release_yml_test.go` spec. A behavior with an explicitly-documented security rationale is being deleted with nothing taking its place.

**Fix:** add an assertion in `release_yml_test.go` (or a `.goreleaser.yaml`-shape test) that prerelease classification is configured, and explicitly decide + record what happens to `make_latest`. If goreleaser can't express it, that deserves the same treatment D-08's `--verify-tag` just got.

*Independently confirmed by the orchestrating session: `.github/workflows/ci.yml` lines 158-163 currently set both `prerelease=` and `make_latest=` outputs with the quoted rationale comment above them; `.goreleaser.yaml` does not yet exist (Phase 10 unexecuted), so there is no current artifact to check `prerelease: auto` against — this is a documented gap in what the plan specifies, not yet observable code.*

### C-4 — HIGH: `dist: bin` + `--clean` will delete build artifacts mid-test

`10-04-PLAN.md:82` sets goreleaser's `dist: bin`, and both `make release` and `make release-snapshot` run with `--clean`. goreleaser's `--clean` **removes the dist directory before building**.

`bin/` is not a goreleaser-private directory in this repo — `Makefile:68-69` writes `bin/gitid` (`BINARY := $(BIN_DIR)/gitid`) and `build-cross` writes `bin/gitid-<os>-<arch>`. So `make release-snapshot` silently deletes the developer's `make build` output and any `build-cross` artifacts.

This becomes a correctness issue in 10-05, whose rewritten e2e helper calls `make release-snapshot`. If any test in that file (or a parallel one) also builds via `make build` — as `TestRelease_UnstampedBuildKeepsDevDefaults` does today — ordering determines whether the binary still exists. That's a flaky-test generator.

**Fix:** either use goreleaser's default `dist: dist/` (add to `.gitignore`), or have the e2e helper build into a dedicated temp dir. The plan gives no rationale for the `dist: bin` choice beyond "the same `$(BIN_DIR)` the Makefile already uses" — which is precisely the collision.

*Independently confirmed by the orchestrating session: `Makefile:68` sets `BIN_DIR := bin` and `:69` `BINARY := $(BIN_DIR)/gitid` — the collision with goreleaser's `dist: bin` + `--clean` is real.*

### C-5 — MEDIUM: stale test name in the 10-01 read-list

`10-01-PLAN.md:79` lists `TestBuildCrossStampsEveryTarget` under `cmd/gitid/release_plumbing_test.go` — correct. But `e2e/release_e2e_test.go` *also* defines `TestRelease_BuildCrossStampsEveryTarget`. The plan text at `:90` says "TestBuildCrossStampsEveryTarget needs no change (it only checks for the literal string `-ldflags "$(LDFLAGS)"`)" — true of the `cmd/gitid` one, false of the e2e one, which actually runs binaries and asserts stamps.

Since 10-01 changes the LDFLAGS `-X` path, the executor could reasonably read that sentence as covering both and skip inspecting the e2e variant. Low blast radius (10-05 rewrites it anyway) but it's an ambiguity in a plan that is otherwise scrupulously precise.

### C-6 — MEDIUM: `git describe` inside the fedora container will likely fail

`10-01` makes `VERSION` a `$(shell git describe ...)` call evaluated on **every** `make` invocation. `10-02`'s fedora job runs as root in a container against a checkout owned by a different uid — the classic `detected dubious ownership in repository` failure. `git describe` then errors, `$(shell ...)` yields empty, and `LDFLAGS` becomes `-X ...version=` (empty).

That won't fail the build — `internal/version.Resolve()` is specified to fall back to `debug.ReadBuildInfo()` on empty. So it degrades silently rather than loudly. Also note the fedora job's `actions/checkout` uses default `fetch-depth: 1`, so **no tags are fetched** — `git describe --tags` has nothing to match and falls to `--always` (bare SHA) even if ownership is fine.

**Fix:** add `git config --global --add safe.directory "$GITHUB_WORKSPACE"` to the fedora job's dnf step, and decide whether the container job should get `fetch-depth: 0` for consistency.

### C-7 — MEDIUM: `make setup-env` in the release path is heavy and unnecessary

`10-04-PLAN.md:99` puts `make setup-env` in release.yml before `make test`/`make lint`. `setup-env` installs goimports, golangci-lint, gosec, pre-commit, **freeze**, and provisions a pinned Chromium revision. Downloading Chromium on the release path adds minutes and a network-flake failure mode to the one workflow where a failure means a tag is pushed but nothing publishes.

It *is* needed (goreleaser is installed there per `10-04-PLAN.md:82`), so this isn't wrong — but it's worth a scoped install target or at least a noted acceptance.

### C-8 — LOW: checksums file is not provenance-attested

`subject-path: "bin/*.tar.gz"` attests the four archives but not `gitid_<version>_checksums.txt`. Since the installer trusts the checksums file to validate the archive, attesting the archives but not the manifest is a slightly odd trust boundary. Common practice (and arguably stronger) is to attest the checksums file. Minor — the archives being individually attested is the load-bearing part.

### C-9 — LOW: all six plans self-report `confidence: low`

Every plan carries `confidence: low` in its estimate block. Given how well-verified the content is, this looks like an un-tuned default rather than a real signal. If it *is* a real signal, the token estimates (55k for 10-04) deserve a second look; if not, it's noise that will train the reader to ignore the field.

---

## Suggestions

1. **Make the `--verify-tag` argument tamper-evident (M-1).** The grep proves the comment exists, not that the premise holds. Add one assertion to `release_yml_test.go`: the `on:` block contains *only* `push:` with `tags:` — no `workflow_dispatch`, no `pull_request`. That test fails the moment someone invalidates the structural argument, which is the actual guarantee D-08 wants.

2. **Add a single "version consistency" test.** The phase now has three version producers (Makefile `git describe`, goreleaser `{{.Env.VERSION}}`, `ReadBuildInfo` fallback) and three consumers (`--version`, `version`, `version --json`). One test asserting `gitid version` output ≡ the `gitid --version` suffix ≡ the JSON doc's fields would catch C-1 and any future drift. 10-01's `<done>` already asserts the first equivalence informally — make it a test.

3. **Reconsider plan 10-06's dependency.** It depends on `10-05` and `10-03`, but its context block includes `@.goreleaser.yaml` (from 10-04) and it documents the Homebrew tap formula path. That's fine transitively (10-05 → 10-04), but the README also needs `gitid version --json` from **10-01**, which isn't in the dependency chain except transitively. Worth an explicit `10-01` for clarity.

4. **Record the `brews:` deprecation as a dated decision.** `10-04-PLAN.md:82` correctly notes `brews:` is soft-deprecated since v2.10 in favor of `homebrew_casks:` and that D-13 locks it deliberately. Good. Add the pin rationale to `PLATFORM-NOTES.md` or a decision log so the next person doesn't "fix" it — the plan comment lives in `.goreleaser.yaml`, which is the right place, but the *why we chose the deprecated one* belongs somewhere durable.

5. **Consider splitting 10-05.** It rewrites 17 e2e tests *and* rewrites `install.sh` *and* removes the `checksums` target. It is the phase's highest-risk plan by a wide margin (RESEARCH Pitfall 1). Task 1 (RED) / Task 2 (GREEN) is the right shape, but the Makefile `checksums` removal could be its own trailing commit for a cleaner revert boundary.

---

## Risk Assessment

| Risk | Likelihood | Impact | Notes |
|---|---|---|---|
| C-1 version `v`-prefix inconsistency | **High** | Medium | Ships green; violates D-11 on the local path. Trivial fix. |
| C-2 untested real checksums filename / redirect | Medium | **High** | Breaks the public one-liner install; discovered by users, not CI. |
| C-4 `dist: bin` + `--clean` artifact deletion | Medium | Medium | Flaky e2e, confusing local DX. |
| C-3 `make_latest` regression | Medium | **High** | Repo has 9 rc tags and 0 stable — this path is the norm here. |
| C-6 fedora `git describe` degradation | **High** | Low | Silent, benign fallback; worth fixing for signal quality. |
| Artifact-shape migration (RESEARCH Pitfall 1) | Medium | **High** | **Well-mitigated** by 10-04's additive-only constraint. |
| `user_setup` blocks real release | **Certain** | Low | Correctly declared; `make release-snapshot` de-risks everything else. |
| goreleaser not installed locally (verified absent) | — | Low | `setup-env` handles it; noting that no plan step can be locally verified until then. |

**Overall: MEDIUM risk, well-controlled.** The sequencing and dependency structure are the strongest part of this plan set, and the one genuinely dangerous migration is explicitly fenced. The concerns cluster in a single theme — *the version/artifact-naming contract is asserted in prose but not pinned by tests* — which is fixable with roughly three small test additions (M-1, suggestion 2, and C-2's fixture change) before wave 2 executes.

Waves 1 (10-01 with the C-1 fix, 10-02 with the C-6 fix, 10-03 as-is) can proceed immediately.

---

## Consensus Summary

Only one reviewer lane produced a result this cycle (`opencode-plan-review`) — the
gemini/qwen/cursor CLIs are not installed on this host, and `claude` was skipped for
reviewer independence per this workflow's self-CLI-skip rule. There is therefore no
cross-reviewer corroboration possible; every finding below is single-source. The
consensus gate in the companion convergence workflow does not engage with a single
reviewer, so every HIGH/CRITICAL finding below counts at full weight in that
workflow's `current_high`.

### Agreed Strengths
N/A — single reviewer this cycle.

### Agreed Concerns
N/A — single reviewer this cycle. See "Concerns" above for the full, individually
source-grounded list (4 HIGH/CRITICAL: C-1, C-2, C-3, C-4; 3 MEDIUM: C-5, C-6, C-7;
2 LOW: C-8, C-9). Two of the HIGH/CRITICAL findings (C-1, C-4) were independently
re-verified against the live repo by the orchestrating session (see inline notes
under each), not just restated from the reviewer's own claim.

### Divergent Views
N/A — single reviewer this cycle.
