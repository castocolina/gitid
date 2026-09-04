---
phase: 10-linux-validation-release-pipeline
plan: 04
subsystem: infra
tags: [goreleaser, github-actions, release, homebrew, provenance-attestation, ldflags]

requires:
  - phase: 10-01
    provides: "internal/version hybrid resolve — the -X path this plan's .goreleaser.yaml ldflags targets"
  - phase: 10-02
    provides: "ci.yml fedora container job (unaffected by this plan's release-job retirement)"
provides:
  - ".goreleaser.yaml — the D-05/D-07/D-08/D-13 goreleaser release build definition (archives, checksum, release, brews pipes), sharing the Makefile's own VERSION/COMMIT/DATE via {{.Env.*}} ldflags"
  - "make release / make release-snapshot targets, plus make setup-env-release (REVIEW C-7's narrower bootstrap)"
  - ".github/workflows/release.yml — tag-triggered (push: tags: v*), job-scoped-permissions goreleaser publish workflow with a re-run test+lint gate before publish"
  - "ci.yml's raw-binary release job (Phase 9.3) fully retired in the same commit set that adds release.yml"
affects: [10-05]

actuals:
  tokens: 6709
  tasks: 2
  commits: 2

tech-stack:
  added:
    - "goreleaser v2.18.0 (dev-tool binary, go install-pinned, never a runtime dep of the shipped gitid binary)"
    - "actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8 # v4.2.2"
  patterns:
    - "goreleaser invoked ONLY through make (release/release-snapshot targets), never raw in CI YAML — matches this repo's existing 'CI invokes the SAME make targets a human runs locally' discipline"
    - "goreleaser's own version detection deliberately bypassed via {{.Env.VERSION}}/{{.Env.COMMIT}}/{{.Env.DATE}} ldflags, referencing the SAME Makefile-computed vars build/build-cross already use — one version computation, not two"
    - "dist: dist (goreleaser's own default) kept explicit and distinct from bin/ (Makefile's build/build-cross output) so goreleaser's --clean never deletes unrelated build artifacts"

key-files:
  created:
    - .goreleaser.yaml
    - .github/workflows/release.yml
    - cmd/gitid/goreleaser_config_test.go
    - cmd/gitid/release_yml_test.go
  modified:
    - Makefile
    - .gitignore
    - .github/workflows/ci.yml
    - cmd/gitid/release_plumbing_test.go

key-decisions:
  - "dist: dist (never bin/) is the load-bearing REVIEW C-4 fix — verified empirically: a pre-existing bin/.pre-existing-marker file survives a full `make release-snapshot --clean` run"
  - "release: prerelease: auto + make_latest: \"{{ not .Prerelease }}\" (REVIEW C-3) — ties GitHub's 'latest' flag to the SAME tag-suffix signal prerelease: auto already reads, so the two settings can never disagree by construction; this repo's entire tag history (9 consecutive -rc.N tags) exercises the prerelease path as the NORMAL case, not an edge case"
  - "brews: (not homebrew_casks:) honored literally per D-13, despite goreleaser's own docs steering toward the newer stanza since v2.10 — brews: remains fully functional through v2.18.0 (10-RESEARCH.md Pitfall 4)"
  - "setup-env-release (REVIEW C-7) installs ONLY golangci-lint/gosec/goreleaser — release.yml's actual gate dependencies — explicitly skipping Chromium provisioning, goimports, and pre-commit hook installation the full setup-env performs, none of which make test/make lint/make release need"
  - "D-08's --verify-tag clause has no goreleaser or gh-release-action config equivalent (10-RESEARCH.md Pitfall 5); resolved via an explicit, tested code comment in release.yml documenting that the tag-triggered, server-side trigger + fetch-depth: 0 checkout structurally satisfies the same guarantee a local --verify-tag flag protects against"
  - "attest-build-provenance's subject-path covers BOTH dist/*.tar.gz AND dist/*_checksums.txt (REVIEW C-8) — attesting only the archives would leave the checksums manifest that VALIDATES them outside the provenance chain"
  - "Rule 1 auto-fix: added `version: 2` to .goreleaser.yaml after `make release-snapshot`'s first live run warned goreleaser v2.18.0 was silently falling back to a deprecated version: 0 parse mode without an explicit schema key"

patterns-established:
  - "Release build definition sharing: any future build-tooling change to VERSION/COMMIT/DATE must update both the Makefile's LDFLAGS and .goreleaser.yaml's {{.Env.*}} references together, or the two build descriptions will silently diverge again"

requirements-completed: [BUILD-03]

coverage:
  - id: D1
    description: "make release-snapshot (zero secrets, no real tag) reproduces the D-07 artifact set: 4 platform tar.gz archives + one checksums.txt under dist/, never bin/"
    requirement: BUILD-03
    verification:
      - kind: unit
        ref: "cmd/gitid/goreleaser_config_test.go#TestGoreleaserConfigDistIsNeverBin"
        status: pass
      - kind: manual_procedural
        ref: "make release-snapshot; ls dist/*.tar.gz dist/*_checksums.txt; test -f bin/.pre-existing-marker (REVIEW C-4 regression guard)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every archive's checksum verifies and each archive contains a runnable gitid binary plus LICENSE/README"
    requirement: BUILD-03
    verification:
      - kind: manual_procedural
        ref: "cd dist && shasum -a 256 -c gitid_*_checksums.txt --ignore-missing (all OK); tar -tzf gitid_*_darwin_arm64.tar.gz (LICENSE, README.md, gitid); extracted darwin_amd64 binary's --version prints the ldflags-stamped git-describe version"
        status: pass
    human_judgment: false
  - id: D3
    description: "goreleaser's build ldflags reference the SAME Makefile-computed VERSION/COMMIT/DATE build/build-cross use, targeting internal/version's exact -X path (D-05, no drift between two version descriptions)"
    requirement: BUILD-03
    verification:
      - kind: unit
        ref: "cmd/gitid/goreleaser_config_test.go#TestGoreleaserConfigLdflagsTargetVersionPackage"
        status: pass
    human_judgment: false
  - id: D4
    description: "A prerelease tag is never marked GitHub's 'latest' release (REVIEW C-3, restoring the retired ci.yml job's make_latest guarantee)"
    requirement: BUILD-03
    verification:
      - kind: unit
        ref: "cmd/gitid/goreleaser_config_test.go#TestGoreleaserConfigMakeLatestTiedToPrerelease"
        status: pass
    human_judgment: false
  - id: D5
    description: "release.yml is tag-scoped, carries job-scoped-only permissions (contents:write/id-token:write/attestations:write), bootstraps via the narrower make setup-env-release (REVIEW C-7), re-verifies test+lint before make release, attests provenance over both archives and checksums (REVIEW C-8), documents the D-08 --verify-tag structural-equivalence resolution, and every action is SHA-pinned"
    requirement: BUILD-03
    verification:
      - kind: unit
        ref: "cmd/gitid/release_yml_test.go (9 test functions: tag-scoped, exact permissions, step sequence/ordering, SHA-pinning, no post-publish amendment, dual subject-path, verify-tag comment presence)"
        status: pass
    human_judgment: false
  - id: D6
    description: "ci.yml's release: job is fully removed in the same commit set that adds release.yml, avoiding a double-publish race on the same tag push"
    requirement: BUILD-03
    verification:
      - kind: unit
        ref: "cmd/gitid/release_plumbing_test.go (TestWorkflowPinsEveryActionToACommitSHA, TestWorkflowTopLevelPermissionsStayReadOnly, both still pass against the retired-job-free ci.yml)"
        status: pass
      - kind: manual_procedural
        ref: "grep -c \"^  release:$\" .github/workflows/ci.yml -> 0"
        status: pass
    human_judgment: false
  - id: D7
    description: "A real v* tag push (once user_setup's tap repo + PAT exist) triggers release.yml, publishing checksummed, provenance-attested archives plus a Homebrew tap update"
    requirement: BUILD-03
    verification: []
    human_judgment: true
    rationale: "Requires castocolina/homebrew-tap repo creation + a fine-grained HOMEBREW_TAP_GITHUB_TOKEN PAT secret — both explicit user_setup items this plan's own frontmatter names as needing a human (Claude cannot create GitHub repos or mint PATs on the user's account). Everything locally verifiable (goreleaser config validity, release.yml structure/tests, a full --snapshot dry run) has been verified above; the actual publish path can only be proven end-to-end once the tap repo/PAT exist and a real tag is pushed."

duration: ~55min
completed: 2026-09-04
status: complete
---

# Phase 10 Plan 04: goreleaser release pipeline + release.yml Summary

**Replaced Phase 9.3's raw-binary GitHub release job with a goreleaser-driven tar.gz-archive pipeline (`.goreleaser.yaml` + `make release`/`make release-snapshot`) and a dedicated tag-triggered `release.yml`, retiring `ci.yml`'s old `release:` job in the same commit set.**

## Performance

- **Duration:** ~55 min
- **Tasks:** 2
- **Files modified:** 8 (4 created, 4 modified)
- **Commits:** 2

## Accomplishments

- `.goreleaser.yaml`: builds/archives/checksum/release/brews pipes implementing D-05, D-07, D-08, D-13 — all using goreleaser's own default naming (zero `name_template` overrides needed), with `dist: dist` deliberately kept distinct from the Makefile's `bin/` output (REVIEW C-4).
- `Makefile`: `GORELEASER_VERSION` (pinned v2.18.0) + `GORELEASER` binary path var; `setup-env` now also installs goreleaser; new `setup-env-release` target (REVIEW C-7) installs only the three tools `release.yml` actually needs; `release`/`release-snapshot` targets export the same `VERSION`/`COMMIT`/`DATE` vars `build`/`build-cross` already compute.
- `.github/workflows/release.yml` (new): tag-triggered, job-scoped permissions, re-runs `make test`+`make lint` before `make release`, attests provenance over both the archives and the checksums manifest (REVIEW C-8), and documents D-08's `--verify-tag` clause as structurally satisfied rather than silently dropped.
- `.github/workflows/ci.yml`: the entire Phase-9.3 `release:` job (raw binaries + `softprops/action-gh-release`) removed in the same commit that adds `release.yml`, eliminating the double-publish race both would otherwise create on an identical `push: tags: v*` event.
- Live-verified: `make release-snapshot` (zero secrets, no real tag) reproduced exactly the D-07 artifact shape — 4 tar.gz archives + 1 checksums.txt under `dist/` — every checksum verified, the darwin_arm64 archive contained a runnable `gitid` + `LICENSE` + `README.md`, and a pre-existing `bin/` marker file survived the `--clean` run.

## Task Commits

1. **Task 1: .goreleaser.yaml + make release/make release-snapshot** - `e9a49b3` (feat)
2. **Task 2: release.yml — retire ci.yml's release job** - `ebfe8e1` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified

- `.goreleaser.yaml` - Release build definition (builds/archives/checksum/release/brews)
- `Makefile` - GORELEASER_VERSION/GORELEASER var, setup-env-release, release/release-snapshot targets
- `.gitignore` - Added `/dist/`
- `cmd/gitid/goreleaser_config_test.go` - Asserts dist:/prerelease/make_latest/ldflags contract
- `.github/workflows/release.yml` - New tag-triggered goreleaser publish workflow
- `.github/workflows/ci.yml` - Retired the raw-binary `release:` job
- `cmd/gitid/release_plumbing_test.go` - Deleted 9 tests scoped to the retired job
- `cmd/gitid/release_yml_test.go` - New tests for release.yml's structure/permissions/sequencing

## Decisions Made

See `key-decisions` in the frontmatter above for the full list with rationale (dist:/bin: separation, make_latest template, brews: vs homebrew_casks:, setup-env-release scope, --verify-tag structural equivalence, dual subject-path attestation, and the `version: 2` schema-key auto-fix).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added `version: 2` to .goreleaser.yaml**
- **Found during:** Task 1, first live `make release-snapshot` run
- **Issue:** goreleaser v2.18.0 printed `only version: 2 configuration files are supported, yours is version: 0, please update your configuration` — the config was silently parsed under a deprecated legacy schema mode with no explicit key present.
- **Fix:** Added a `version: 2` top-level key with an explanatory comment.
- **Files modified:** `.goreleaser.yaml`
- **Verification:** Re-ran `make release-snapshot`; the warning no longer appears; all 4 archives + checksums.txt still produced identically.
- **Committed in:** `e9a49b3` (Task 1 commit)

**2. [Rule 1 - Bug] Fixed a `staticcheck` finding (De Morgan's law) in a new test**
- **Found during:** Task 2, `make lint` re-run before committing
- **Issue:** `cmd/gitid/release_yml_test.go`'s step-ordering assertion used a negated compound `&&` condition golangci-lint's `staticcheck` (QF1001) flagged as simplifiable.
- **Fix:** Rewrote as an equivalent `||`-chained early-exit condition.
- **Files modified:** `cmd/gitid/release_yml_test.go`
- **Verification:** `make lint` reports 0 issues; `go test -run TestReleaseWorkflowStepSequence` still passes.
- **Committed in:** `ebfe8e1` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1, both caught by the plan's own live-verification/pre-commit-hook discipline before committing).
**Impact on plan:** Both fixes are exactly what the plan's own `<verify>` blocks and pre-commit hooks exist to catch. No scope creep.

## Issues Encountered

- A stray `make lint` run (before the first commit) surfaced 11 `gosec`/`revive` findings pointing at files under a *different*, sibling worktree path (`agent-a81069eeb88049831/internal/screenshot/...`) that does not exist in this worktree. Diagnosed as a stale `golangci-lint` result cache shared across worktrees on this machine (not a defect introduced by this plan). Running `golangci-lint cache clean` resolved it immediately; a subsequent `make lint` ran cleanly against this worktree's actual files. Not logged as a deviation since it required no code change — purely a local tool-cache artifact, noted here for the record per the phase's diligence discipline.

## User Setup Required

**External services require manual configuration** for the release publish path (D-13's Homebrew tap) to actually run end-to-end on a real tag push. Per this plan's own frontmatter `user_setup` block:

1. Create the `castocolina/homebrew-tap` GitHub repository (can be empty/initialized with a README) — https://github.com/new
2. Add a fine-grained PAT scoped to **only** `castocolina/homebrew-tap` (contents: write) as the `HOMEBREW_TAP_GITHUB_TOKEN` repository secret on `castocolina/gitid` — Settings → Secrets and variables → Actions → New repository secret

Neither step can be performed by an automated agent (repo creation and PAT minting both require the user's own GitHub account). Everything locally verifiable without these — `.goreleaser.yaml`'s validity, `release.yml`'s structure/tests, and a full zero-secrets `make release-snapshot` dry run — has been verified and is green (see coverage D1-D6 above). D7 (the real tag-push → tap-push path) is recorded as `human_judgment: true` pending this setup.

## Next Phase Readiness

- The goreleaser release pipeline is fully wired, tested, and locally dry-run-verified. `ci.yml` no longer double-publishes.
- Plan 10-05 (per 10-RESEARCH.md's own flagged scope) still owes: migrating `scripts/install.sh` and its 17 `e2e/release_e2e_test.go` test functions from the OLD raw-binary artifact shape (`gitid-<os>-<arch>` + `bin/checksums.txt`) to the NEW tar.gz-archive shape (`gitid_<version>_<os>_<arch>.tar.gz` + `gitid_<version>_checksums.txt`) this plan introduces. This plan deliberately did NOT touch `checksums`/`build-cross`/`install.sh`/`release_e2e_test.go` (additive-only change set, per the plan's own action text), so `make test-e2e`'s existing raw-binary release coverage is untouched and still green — but it now tests a pipeline (`make checksums` + the old asset naming) that is no longer what `release.yml` actually publishes. This is the single largest remaining item for Plan 10-05, not a regression introduced here.
- No blockers for Plan 10-05.

---
*Phase: 10-linux-validation-release-pipeline*
*Completed: 2026-09-04*

## Self-Check: PASSED
