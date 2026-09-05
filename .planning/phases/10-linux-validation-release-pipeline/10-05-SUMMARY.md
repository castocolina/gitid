---
phase: 10-linux-validation-release-pipeline
plan: 05
subsystem: infra
tags: [install-script, tar-gz, goreleaser, e2e, checksum-verification, posix-sh]

requires:
  - phase: 10-04
    provides: ".goreleaser.yaml + make release/release-snapshot producing the D-07 tar.gz archives and gitid_<version>_checksums.txt this plan's install.sh and e2e suite consume"
provides:
  - "scripts/install.sh rewritten for the D-07 tar.gz archive contract: versioned asset/checksums filename construction, GitHub /releases/latest redirect resolution (GITID_VERSION unset) or exact-tag pin (GITID_VERSION set), verify-before-extract over the archive, GITID_INSTALL_DIR override"
  - "e2e/release_e2e_test.go fully migrated (17 rewritten + 3 net-new functions) to fixture servers mirroring GitHub's real /releases/download/<tag>/<asset> and /releases/latest -> /releases/tag/<tag> redirect URL shape (REVIEW C-2)"
  - "Makefile's checksums target (and dead SHA256SUM var) retired — goreleaser's checksum: pipe is the sole manifest producer now"
affects: []

actuals:
  tokens: 5798
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Fixture servers root a temp dir at <root>/castocolina/gitid/releases/download/<tag>/ (and, for the redirect test, also /releases/latest + /releases/tag/<tag>) so install.sh's REAL URL-construction code is exercised end-to-end by every test, never a flat/simplified test-only layout"
    - "GITID_INSTALL_BASE_URL is a pure origin substitution (GITHUB_ORIGIN=${GITID_INSTALL_BASE_URL:-https://github.com}) — every line of tag-resolution/asset-construction/download/verify/extract logic below it runs unconditionally, whether the origin is github.com or a test fixture"
    - "Every rewritten TestInstallScript_* case pins GITID_VERSION to a real, dynamically-discovered tag (never a hardcoded literal) so it drives install.sh's REAL versioned filename construction; ONE dedicated test (TestInstallScript_ResolvesLatestViaGitHubRedirect) leaves GITID_VERSION unset to exercise the redirect-resolution/tag-stripping path"

key-files:
  created: []
  modified:
    - scripts/install.sh
    - e2e/release_e2e_test.go
    - e2e/harness_test.go
    - Makefile

key-decisions:
  - "Fixed a genuinely stale, pre-existing test bug while rewriting: the old e2eStampLine literal (\"gitid version 9.9.9-e2e (deadbee, 2001-02-03)\") predated Plan 10-01's D-11 four-part --version format (which appends \"<goos>/<goarch>\") and was failing independently of this migration, exactly as this plan's required_reading flagged. Replaced with a function computing the platform-suffixed expectation."
  - "OS/arch allowlist refusal happens BEFORE tag/version resolution in the rewritten install.sh (not just before the download step) — strengthens the existing 'refuse before any URL is built' discipline to also cover the new /releases/latest redirect call, not just the asset/checksums downloads."
  - "Extraction uses \"tar -xzf <archive> gitid\" to pull only the known relative member out of the archive (never a glob or full extract) — the same tar path-traversal mitigation discipline the old raw-binary script achieved by construction (there was no archive to traverse), now made explicit for the new archive-based flow (T-10-05-03)."
  - "Dropped the now-fully-dead SHA256SUM Makefile variable alongside the checksums target it exclusively fed — leaving it would be dead code with zero remaining consumers."

patterns-established:
  - "gitidMemberSHA256(t, archivePath) extracts the \"gitid\" tar member via archive/tar + compress/gzip (Go stdlib, no shelling to tar) and hashes it — the correct comparison point for \"what install.sh should have installed\" now that the on-disk asset is an archive, not the raw binary."

requirements-completed: [BUILD-03, BUILD-05]

coverage:
  - id: D1
    description: "scripts/install.sh downloads and verifies the D-07 tar.gz archive (never the old raw-binary shape) BEFORE extracting, then installs only the known gitid binary out of the extracted contents"
    requirement: BUILD-03
    verification:
      - kind: e2e
        ref: "e2e/release_e2e_test.go#TestInstallScript_InstallsVerifiedHostBinary"
        status: pass
      - kind: e2e
        ref: "e2e/release_e2e_test.go#TestInstallScript_ChecksumMismatchRefuses"
        status: pass
      - kind: e2e
        ref: "e2e/release_e2e_test.go#TestInstallScript_LeavesNothingBehindOnRefusal"
        status: pass
    human_judgment: false
  - id: D2
    description: "A GITID_VERSION pin and a custom GITID_INSTALL_DIR override both work — neither existed in the Phase 9.3 script"
    requirement: BUILD-03
    verification:
      - kind: e2e
        ref: "e2e/release_e2e_test.go#TestInstallScript_GITIDVersionPinsToASpecificVersion"
        status: pass
      - kind: e2e
        ref: "e2e/release_e2e_test.go#TestInstallScript_CustomInstallDirOverride"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every install.sh behavior is proven against real, locally-built goreleaser snapshot archives — not hand-rolled fixtures that could silently diverge from the real artifact shape"
    requirement: BUILD-03
    verification:
      - kind: e2e
        ref: "e2e/release_e2e_test.go#stampedArtifacts (invokes real `make release-snapshot`, all 20 TestRelease_*/TestInstallScript_* functions consume its output)"
        status: pass
    human_judgment: false
  - id: D4
    description: "REVIEW C-2: the fixture server mimics GitHub's real /releases/latest redirect and /releases/download/<tag>/<asset> URL shape — every test drives the real versioned filename construction, and one dedicated test drives the real redirect-resolution/tag-stripping code"
    requirement: BUILD-05
    verification:
      - kind: e2e
        ref: "e2e/release_e2e_test.go#TestInstallScript_ResolvesLatestViaGitHubRedirect"
        status: pass
      - kind: unit
        ref: "make lint-shell (POSIX sh -n parse check on scripts/install.sh)"
        status: pass
    human_judgment: false
  - id: D5
    description: "A real, real-network end-to-end run of the published one-liner against an actual HTTP server on this machine, outside the go test process tree"
    verification: []
    human_judgment: true
    rationale: "Attempted a manual `make release-snapshot && GITID_VERSION=... GITID_INSTALL_BASE_URL=http://127.0.0.1:PORT ./scripts/install.sh` run against a python3 http.server fixture per this plan's own <verification> section. The harness's Bash tool sandboxes network loopback across separate tool invocations (a backgrounded server process does not survive to a subsequent curl call, and curl consistently returned exit 28/connection-refused even with dangerouslyDisableSandbox) — this is an environment constraint of the execution harness, not a defect in install.sh. The EQUIVALENT scenario (a real /bin/sh subprocess running scripts/install.sh, a real curl invocation, a real httptest.Server serving real goreleaser-built archives at the exact GitHub URL shape) is exhaustively proven by the automated e2e suite (32/32 passing under -race), because both ends of that connection live inside the SAME `go test` process tree, which the sandbox does not isolate. A human with unrestricted shell access can trivially re-run the exact manual command this plan's <verification> section specifies to close this out."

duration: ~45min
completed: 2026-09-04
status: complete
---

# Phase 10 Plan 05: Migrate install.sh + its e2e suite to the D-07 tar.gz archive contract Summary

**Rewrote `scripts/install.sh` and all 20 of `e2e/release_e2e_test.go`'s test functions (17 migrated + 3 net-new) from Phase 9.3's raw-binary artifact shape to Plan 10-04's goreleaser-produced tar.gz archives, closing REVIEW C-2's "untested real checksums filename / redirect-resolution" finding via fixture servers that mirror GitHub's actual URL shape.**

## Performance

- **Duration:** ~45 min
- **Tasks:** 2 (RED then GREEN, per this plan's `type: tdd`)
- **Files modified:** 4

## Accomplishments

- `e2e/release_e2e_test.go` fully migrated: every archive/checksums filename is discovered dynamically from a real `make release-snapshot` run (never hardcoded), and both fixture-server constructors (`startReleaseFixtureServer`, `startGitHubRedirectFixtureServer`) root their download layout at the exact GitHub Releases path shape (`/castocolina/gitid/releases/download/<tag>/<asset>`, `/releases/latest` -> `/releases/tag/<tag>`).
- `scripts/install.sh` rewritten: asset/checksums naming now matches goreleaser's `gitid_<version>_<os>_<arch>.tar.gz` / `gitid_<version>_checksums.txt` defaults; GitHub `/releases/latest` redirect resolution when `GITID_VERSION` is unset; exact-tag pin (skipping the redirect entirely) when it is set; checksum verification over the downloaded ARCHIVE strictly before `tar -xzf`; extraction pulls only the known `gitid` member; new `GITID_INSTALL_DIR` override.
- Fixed a real, pre-existing bug surfaced during the rewrite: `TestRelease_BuildCrossStampsEveryTarget`'s `e2eStampLine` literal predated Plan 10-01's D-11 four-part `--version` format (missing the `<goos>/<goarch>` suffix) — it would have failed independently of this migration. Replaced with a function that computes the platform-suffixed expectation.
- `Makefile`'s `checksums` target (and its now-fully-dead `SHA256SUM` variable) retired — goreleaser's `checksum:` pipe is the only manifest producer left in the tree.
- All 3 net-new REVIEW C-2 cases pass: `TestInstallScript_GITIDVersionPinsToASpecificVersion` (and asserts `/releases/latest` was never hit), `TestInstallScript_CustomInstallDirOverride`, `TestInstallScript_ResolvesLatestViaGitHubRedirect` (asserts all 4 expected requests — latest redirect, tag page, asset, checksums — fired, and no others).

## Task Commits

1. **Task 1 (RED): rewrite e2e/release_e2e_test.go for the D-07 tar.gz contract** - `265f697` (test)
2. **Task 2 (GREEN): rewrite scripts/install.sh for the tar.gz archive, GITID_VERSION pin, and custom install dir** - `74491b1` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified

- `e2e/release_e2e_test.go` - Full rewrite: fixture-server helpers, archive/checksums naming derived from a real build, all 17 migrated tests + 3 net-new tests
- `e2e/harness_test.go` - One-line doc-string update (`e2eAllowedAmbientPathSites["stampedArtifacts"]` now says `make release-snapshot`, matching the function's new body)
- `scripts/install.sh` - Full rewrite for tar.gz download/verify/extract, GitHub redirect resolution, GITID_VERSION pin, GITID_INSTALL_DIR override
- `Makefile` - Removed the `checksums` target + its `.PHONY` entry + the now-dead `SHA256SUM` variable; updated `build-cross`'s comment (no longer references `make checksums`)

## Decisions Made

See `key-decisions` in the frontmatter above for the full list with rationale (the stale e2eStampLine fix, OS/arch-check-before-redirect ordering, tar member-extraction discipline, dead-variable cleanup).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed the stale, pre-existing `e2eStampLine` literal**
- **Found during:** Task 1, first `go test` run after the rewrite compiled
- **Issue:** `e2eStampLine = "gitid version 9.9.9-e2e (deadbee, 2001-02-03)"` was missing the `<goos>/<goarch>` suffix Plan 10-01's D-11 format added — `TestRelease_SnapshotBuildProducesArchivesForEveryTarget` failed on a real `--version` mismatch (`... (deadbee, 2001-02-03, darwin/amd64)` vs the literal without the suffix), exactly the known-broken test this plan's `required_reading` flagged.
- **Fix:** Replaced the flat constant with `e2eStampLine(goos, goarch string) string`, computing the expectation with `runtime.GOOS`/`runtime.GOARCH` at each call site.
- **Files modified:** `e2e/release_e2e_test.go`
- **Verification:** Re-ran the full suite; both call sites now pass.
- **Committed in:** `265f697` (Task 1 commit)

**2. [Rule 3 - Blocking] Cleared a stale golangci-lint result cache before Task 1's commit**
- **Found during:** Task 1, first commit attempt
- **Issue:** The pre-commit hook's `make lint` surfaced 11 gosec/revive findings pointing at file paths under a DIFFERENT sibling worktree (`agent-a1ebeef2e9a3b0a53/internal/screenshot/...`) that does not exist in this worktree — the identical class of cross-worktree cache artifact recorded in `10-04-SUMMARY.md`'s "Issues Encountered".
- **Fix:** `golangci-lint cache clean`, then re-ran the commit.
- **Files modified:** none (tool-cache artifact, no code change)
- **Verification:** The re-run `make lint` reported 0 issues against this worktree's actual files.
- **Committed in:** n/a (pre-commit tooling fix, not a source change)

---

**Total deviations:** 2 (1 Rule 1 bug fix in test code, 1 Rule 3 tooling-cache fix with no code change). Both directly served this plan's own stated goal (a correct RED state and a clean commit) — no scope creep.

## Issues Encountered

- The plan's own `<verification>` section calls for one manual, real-network confirmation: `make release-snapshot && GITID_VERSION=v<discovered-version> GITID_INSTALL_BASE_URL=<local fixture server URL> ./scripts/install.sh`. Attempting this literally (spinning up a `python3 -m http.server` fixture in a separate shell process and `curl`-ing it) failed consistently with a loopback-connection error (curl exit 28) — the execution harness's Bash tool appears to sandbox network loopback across separate tool invocations; a backgrounded server process does not survive to be reached by a subsequent `curl` call, even with `dangerouslyDisableSandbox: true`. This is an environment constraint, not a defect in `install.sh`: the EQUIVALENT scenario — a real `/bin/sh` subprocess running the real `scripts/install.sh`, issuing real `curl` requests, against a real `httptest.Server` serving real `make release-snapshot` output at the exact GitHub URL shape — is exhaustively exercised by the automated e2e suite (32/32 passing under `-race`), because both ends of that connection live inside the same `go test` process tree, which the harness's sandboxing does not isolate. Recorded as coverage item D5 (`human_judgment: true`) above so a human with unrestricted shell access can close it out by re-running the exact command this plan specifies.

## User Setup Required

None - no external service configuration required for this plan (the Homebrew tap PAT setup remains 10-04's already-recorded, still-outstanding user_setup item, unaffected by this plan).

## Next Phase Readiness

- `scripts/install.sh` and its entire e2e suite now agree on the D-07 tar.gz contract end-to-end; no half-migrated state remains in the tree (`grep -rn "make checksums\|SHA256SUM"` across the tree returns no hits).
- The Phase 10 goal's install-script and artifact-shape migration work is complete. Remaining Phase 10 scope (per `ROADMAP.md`) is unaffected by this plan and can proceed independently.
- The one open item is D5 above (a literal real-network manual re-run of the published one-liner) — recorded as `human_judgment: true` rather than silently marked done, since the harness could not complete it.

---
*Phase: 10-linux-validation-release-pipeline*
*Completed: 2026-09-04*

## Self-Check: PASSED
