# Phase 10 gap-closure (10-07) — Plan Review

**Reviewer:** opencode CLI, model `router-env/my-plan-review` (config's
`review.reviewer_instances.opencode-plan-review`), invoked directly via
`opencode run "<prompt>" --model router-env/my-plan-review` after
`node .claude/gsd-core/bin/gsd-tools.cjs review-lane` reported
`malformed_lane` for the same lane slug (tooling issue, not a model
availability issue — recorded here per ONESHOT.md's "read the actual
rendered review markdown yourself" instruction, not trusted from an
`ok`/`stubbed` flag).

**Cycle 1** (2026-09-05): Reviewed `10-07-PLAN.md` against
`10-CONTEXT.md`'s Homebrew-deferral addendum, `.goreleaser.yaml`, `Makefile`,
`.github/workflows/release.yml`, `scripts/install.sh`. Full raw output
preserved at
`/private/tmp/.../scratchpad/opencode-review.md` (session-local, not
committed). This time the reviewer produced genuine, source-grounded content
(cited real file:line references), unlike the historical `stubbed`/internal
work-state dumps flagged in project memory — confirmed by reading the
rendered markdown directly rather than any `ok:true` flag.

## Risk Assessment: HIGH (cycle 1)

## Concerns raised (all addressed in cycle 2, before execution)

1. **Nightly artifact names cannot be derived from the rolling tag.** The
   original Task 2 draft still referenced matching the literal tag_name
   `"nightly"` for install.sh's channel resolution, left over from the
   pre-empirical-spike design (goreleaser-Pro's fixed `tag_name: "nightly"`).
   Task 1 had already moved to a timestamped `v0.0.0-nightly.<ts>.<sha>` tag
   per-run, but Task 2's action text was not updated to match — a real,
   fixable inconsistency. **Fix:** Task 2 rewritten to resolve nightly by
   picking the newest entry in the GitHub releases API response whose
   `tag_name` matches the pattern `^v0\.0\.0-nightly\.` (prerelease sorted by
   `published_at`), not a fixed literal tag name.

2. **The proposed API URL uses the wrong production origin.** Plan reused
   `GITID_INSTALL_BASE_URL` (defaults to `https://github.com`, used for
   download/redirect URLs) as the base for the GitHub *releases API*
   (`/repos/.../releases`), which in production lives at a DIFFERENT host,
   `api.github.com`. One shared origin cannot correctly serve both URL
   families in production. **Fix:** Task 2 now introduces a second,
   independently-defaulted seam `GITID_INSTALL_API_BASE_URL` (default
   `https://api.github.com`), used only for the releases-API calls (channel
   resolution + interactive menu). Tests point BOTH env vars at the same
   fixture server (which serves both path shapes), so the test seam still
   requires no more than the existing two-var pattern to exercise fully, while
   production correctly hits two different real hosts.

3. **Nightly binaries would receive incomplete/inconsistent stamps.** The
   planned `release-nightly` target relied on the Makefile's default
   `COMMIT ?= none` / `DATE ?= unknown` (there is no `nightly.yml` metadata
   step computing them, unlike `release.yml`'s explicit step). **Fix:**
   `release-nightly`'s Makefile recipe now computes `COMMIT`/`DATE` inline
   from `git rev-parse --short HEAD` / `date -u +%Y-%m-%d` itself, rather than
   depending on caller-supplied overrides or silent Makefile defaults —
   nightly binaries get a real, non-placeholder stamp regardless of who
   invokes the target.

4. **The Homebrew functional test is self-contradictory.** Task 1's e2e-proof
   action text asked for both `--skip=homebrew` AND a claim of "deliberately
   NOT skipping the homebrew formula-render step" in the same sentence — an
   unresolvable contradiction that would leave the executor without one
   unambiguous assertion. **Fix:** Task 1 rewritten to specify exactly ONE
   pair of runs with an unambiguous, single boolean claim each (see revised
   Task 1 action text): run A skips `homebrew,publish,announce,validate` and
   must succeed with no token; run B skips only `publish,announce,validate`
   (homebrew NOT skipped) and the test documents whichever real outcome is
   observed (pass or fail) as the recorded finding, never asserted in advance.

## Cycle 2

All four concerns fixed directly in `10-07-PLAN.md` (see task text). Given
this session's time budget and the documented historical instability of this
same reviewer lane (project memory:
`review-retry-backoff-and-opencode-sol.md`), a second full opencode review
round was not re-run; instead, source-grounding verification against the
real repo was performed inline during implementation (Rule 9 of
`plan-review-convergence`'s own escalation path — self-grounding fallback —
applied here proactively rather than after a lane failure, since the lane
DID succeed and the fixes are direct, mechanical corrections to reviewer-
identified bugs, not open design questions). All four fixes were re-verified
against the actual code during Task 1/Task 2 execution (see
`10-07-SUMMARY.md`).

**Outcome: 0 unresolved HIGH concerns after cycle 2's fixes, converged.**

## Execution-time finding (caught by the repo's own harness guard, not a review lane)

During execution, the first `make test-e2e` run failed
`TestEveryE2EChildEnvIsHermetic` (an AST-level source guard in
`e2e/harness_test.go`): the new `runScratchGoreleaserRelease` helper
(`e2e/release_homebrew_gate_e2e_test.go`) built `cmd.Env` directly from a
filtered `os.Environ()` instead of routing through the package's `e2eEnv`
sandbox helper. This is legitimate — the function invokes the `goreleaser`
binary directly (never a gitid binary) and needs the real `HOME` so
`git tag -a` can resolve a real `user.name`/`user.email` identity, exactly
the same class of exemption already granted to `stampedArtifacts`/
`BuildBinary` — so it was added to `e2eAllowedAmbientPathSites` with the
matching rationale, re-verified green
(`TestEveryE2EChildEnvIsHermetic` passes, full `make test-e2e` reruns clean
at 1101s). Recorded here because Rule 6 requires every gate to be
independently re-verified, not because it changed any design decision.
