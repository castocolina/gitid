---
phase: 10-linux-validation-release-pipeline
verified: 2026-09-05T04:10:00Z
reverified: 2026-09-05T16:00:00Z
status: human_needed
score: 8/10 must-haves verified (original); gap-closure (10-07) adds D-18/D-19/D-20, all automated-verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Run the Bazzite manual UAT checklist (.planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md) on real Bazzite hardware, once per release, and update PLATFORM-NOTES.md's 5 'Pending manual UAT' rows to Verified/Accepted limitation."
    expected: "Each of the 5 rows (SELinux spot-check, /home->/var/home symlink includeIf resolution, real-terminal TUI rendering, wl-clipboard presence, ssh-add no-agent degradation) resolves to a real PASS/FAIL finding, not a placeholder."
    why_human: "Requires physical/real Bazzite hardware with SELinux enforcement, a real Wayland desktop session, and real terminal emulators (Ptyxis/Konsole) — none of which exist in this (or any) automated CI/agent sandbox. This is explicitly the D-01(ii)/D-03 design: the container CI job proves everything a container CAN exercise; this checklist is the deliberately-human-only residue, by design run once per release, not a one-time phase gate. UNCHANGED by the 2026-09-05 gap-closure (10-07) — out of scope per its own instructions."
---

## 2026-09-05 gap-closure addendum (10-07-PLAN.md, D-18/D-19/D-20)

**The Homebrew-tap human_verification item above has been REMOVED, not just
reworded** — it previously blocked "a real v* tag push" on the tap repo/PAT
existing; that is no longer true (D-18). The release pipeline now succeeds
without the tap repo/PAT via `--skip=homebrew` (verified live —
`e2e/release_homebrew_gate_e2e_test.go`'s
`TestReleaseHomebrewGate_SkippedNeverEntersHomebrewPipe`/
`_NotSkippedStillSucceedsLocally`, both passing, run against this repo's
REAL `.goreleaser.yaml` with a real scratch git tag and the real pinned
goreleaser v2.18.0 binary — not a synthetic/mocked config). Creating the tap
repo remains available to the user as an optional future enhancement (see
10-UAT.md item 2), never again as a release blocker.

Two new items were added and both fully automated-verified (no human
action needed, see 10-UAT.md items 3/4 for the "first live observation"
residue, which self-resolves on ordinary future use rather than requiring a
human to do anything special):

- D-19 nightly release automation (`make release-nightly`,
  `.github/workflows/nightly.yml`) — GoReleaser's native `nightly:`/`--nightly`
  mode was found to be GoReleaser-Pro-only (empirically verified: pinned OSS
  v2.18.0's `goreleaser release --help` lists no `--nightly` flag;
  `goreleaser jsonschema` has zero `nightly` occurrences). Implemented
  instead via goreleaser's ordinary `release` command against a freshly
  created `v0.0.0-nightly.<timestamp>.<sha>` tag.
- D-20 `scripts/install.sh` `GITID_CHANNEL=stable|nightly` resolution plus a
  real usable-`/dev/tty`-gated interactive menu, adapted from
  `castocolina/wezterm-setup`'s proven technique — with one empirically-found
  and fixed correction: the probe must use `true < /dev/tty`, not `:` (the
  wezterm-setup bash original), because POSIX classifies `:` as a special
  builtin and a redirection error on one unconditionally terminates a
  non-interactive POSIX-conformant shell (reproduced directly against
  `dash`) — `true` is an ordinary builtin and degrades gracefully instead.

All new code is covered by real, passing tests (not asserted): 6 new
structural tests (`cmd/gitid/goreleaser_config_test.go`,
`cmd/gitid/nightly_yml_test.go`), 4 new install.sh e2e tests exercising a
real fixture server and a real pty
(`e2e/release_channel_e2e_test.go`), and 2 new e2e tests exercising a REAL
goreleaser release-mode invocation against this repo's actual config
(`e2e/release_homebrew_gate_e2e_test.go`). `make test` (-race), `make lint`,
and `make test-e2e` all re-run clean after this gap-closure (see
`10-07-SUMMARY.md` for exact commands/output).

# Phase 10: Linux Validation + Release Pipeline Verification Report

**Phase Goal:** The whole app is validated end-to-end on a mainstream Linux distro, alongside macOS.
**Verified:** 2026-09-05
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A `fedora:latest` container CI job runs the FULL automated suite (`make test -race`, `make lint`, `make test-e2e`) — not a subset — on push-to-main and release tags. | ✓ VERIFIED | `.github/workflows/ci.yml` `fedora:` job read in full: steps are `dnf install` (prereqs) → `safe.directory` → checkout `fetch-depth: 0` → setup-go → `make setup-env` → `make test` → `make lint` → `make test-e2e`, gated `if: github.event_name == 'push'` (covers both push-to-main and tag pushes). 8 structural tests in `cmd/gitid/ci_fedora_test.go` (`TestFedoraJobExists`, `...RunsOnlyOnPush`, `...UsesFedoraLatestContainer`, `...DnfInstallIsFirstStep`, `...SafeDirectoryBeforeCheckout`, `...CheckoutFetchesFullTagHistory`, `...DeclaresNoPermissions`, `...RunsTheFullAutomatedSuite`) all pass when I ran them directly. I could not execute the container itself in this sandbox (no Docker) — this is a structural/backstop verification of the job definition, not a live container run. |
| 2 | The `ssh -V` distro-suffix regression fixture closes a real, previously-risky gap (not a vacuous pass). | ✓ VERIFIED | Read `internal/platform/version_test.go`: the pre-existing "Debian/Ubuntu distro suffix" case and the new "Fedora, no distro suffix (OpenSSH 10.x)" case both assert exact parsed `OpenSSHVersion`/`SSLFlavor`/`SSLVersion` fields. Ran `go test ./internal/platform/... -run TestParseSSHVersion -v` myself: 7/7 pass, including both suffix and no-suffix forms — a genuinely meaningful, non-trivial assertion. |
| 3 | The Bazzite manual UAT checklist is a genuinely runnable runbook (not a placeholder). | ✓ VERIFIED | Read `bazzite-uat-checklist.md` in full: 5 numbered sections, each with an exact command, a concrete PASS description, and a concrete FAIL description (SELinux `ls -Z`, symlink `git config --show-origin`, real-terminal rendering, `wl-clipboard` paste check, `unset SSH_AUTH_SOCK` degradation check), plus a closing instruction to update PLATFORM-NOTES.md rows. This is a real, executable document, not a stub. |
| 4 | `PLATFORM-NOTES.md` is a real per-distro ledger with actual sourced rows (not empty/stub). | ✓ VERIFIED | Read `PLATFORM-NOTES.md` in full: 7 rows across Fedora/Bazzite, 2 already resolved with sourced findings (`ssh -V` no-suffix confirmation, dnf prerequisite list), 5 explicitly marked "Pending manual UAT" (honest, not fabricated as done). |
| 5 | The Bazzite manual UAT has actually been **executed** at least once, closing the human-only residue of PLAT-03's "validated end-to-end" claim. | ⚠️ Not done — routed to human verification | `PLATFORM-NOTES.md`'s 5 Bazzite rows are all "Pending manual UAT," not Verified/Accepted. No real Bazzite hardware exists in this (or any) automated execution context. See `human_verification` item 1. |
| 6 | `ci.yml`'s OLD raw-binary release job (Phase 9.3) was actually **deleted**, not left running alongside `release.yml` (which would double-publish on the same tag push). | ✓ VERIFIED | Read `.github/workflows/ci.yml` end-to-end: only `build-cross`, `check`, and `fedora` jobs exist; no `release:` job. A large header comment explicitly documents the retirement and the double-publish race it avoids. `git log` confirms `release.yml` was added in the same commit series (`ebfe8e1`) that retired the old job. |
| 7 | `.goreleaser.yaml` produces the D-07 tar.gz artifact shape (checksummed, correctly named) and the D-13 Homebrew tap formula. | ✓ VERIFIED (explicit, reproduced live) | I personally installed goreleaser v2.18.0 (the pinned version) and ran `make release-snapshot` myself (not trusting the SUMMARY). It produced `dist/gitid_<version>_{darwin,linux}_{amd64,arm64}.tar.gz` (4 archives), `dist/gitid_<version>_checksums.txt`, and `dist/homebrew/Formula/gitid.rb`. I extracted the `darwin_amd64` archive and ran the binary: `gitid --version` printed the correct stamped version matching the snapshot build. The Homebrew formula correctly branches on `Hardware::CPU.intel?/.arm?` for macOS/Linux with matching URLs/sha256 per archive. |
| 8 | `gitid --version` / `gitid version --json` are genuinely stamped end-to-end, matching D-11's format. | ✓ VERIFIED (explicit) | I built the binary myself (`make build`) and ran it: `gitid --version` → `gitid version 0.1.0-rc.9-175-g2911788 (none, unknown, darwin/amd64)` (D-11's exact 4-part format). `gitid version --json` → a stable `gitid.version/v1` schema envelope with `version`/`commit`/`build_date`/`platform` fields. `internal/version` unit tests (6/6, covering all 4 build-path cases) and `cmd/gitid` version-cmd tests (3/3) pass. |
| 9 | `scripts/install.sh`'s verify-before-extract security property is real (checksum check strictly BEFORE `tar -xzf`), not just claimed. | ✓ VERIFIED | Read the script source directly: the SHA-256 comparison (`actual != expected` → `fail`) executes before the `tar -xzf "$asset" gitid` line; extraction pulls only the exact known member name (no glob, no path traversal). Ran the full release/install e2e subset myself: `go test -tags e2e -race -run 'TestRelease_\|TestInstallScript_' ./e2e/...` — 33/33 pass, including `TestInstallScript_ChecksumMismatchRefuses` (asserts the script refuses and never extracts on a bad checksum). |
| 10 | The real, live end-to-end release-publish path (a real `v*` tag pushed to GitHub, triggering `release.yml`, publishing a GitHub Release AND pushing the Homebrew tap formula) has actually run for Phase 10's new pipeline. | ⚠️ Not done — routed to human verification | Only the zero-secrets local `make release-snapshot` dry run (item 7) and structural/unit tests have run; no real tag has been pushed against this pipeline. Confirmed live: `curl -sI https://github.com/castocolina/homebrew-tap` → HTTP 404 — the tap repo does not exist yet, so the `brews:` publish leg cannot have run. `10-04-SUMMARY.md` itself explicitly and honestly names this as a `user_setup` item requiring the user's own GitHub account (repo creation + PAT minting), not something an agent can complete. See `human_verification` item 2. |

**Score:** 8/10 truths verified (2 routed to human verification — neither is a code/wiring failure; both are extrinsic human-only actions the phase's own design and documentation already anticipated).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/version/version.go` + `_test.go` | Hybrid ldflags/buildinfo version resolver | ✓ VERIFIED | Read in full; 6/6 unit tests pass covering all 4 D-09 build-path cases. |
| `cmd/gitid/version_cmd.go` | `gitid version [--json]` subcommand | ✓ VERIFIED | 3/3 tests pass; output matches `--version` exactly per test `TestVersionCmdMatchesVersionFlagOutput`. |
| `.github/workflows/ci.yml` (fedora job) | Full automated suite in a real Fedora container, push/tag-gated | ✓ VERIFIED | Read in full; 8 structural tests pass. |
| `internal/platform/version_test.go` | ssh -V distro-suffix regression | ✓ VERIFIED | Ran directly; 7/7 pass, meaningful assertions. |
| `PLATFORM-NOTES.md` | Per-distro ledger | ✓ VERIFIED | Real content, 7 rows, 2 resolved + 5 honestly pending. |
| `bazzite-uat-checklist.md` | Runnable manual UAT runbook | ✓ VERIFIED | Real, concrete, executable document. |
| `.goreleaser.yaml` | D-07/D-08/D-12/D-13 release build definition | ✓ VERIFIED | Reproduced a real build myself; exact artifact shape confirmed. |
| `.github/workflows/release.yml` | Tag-triggered goreleaser publish, retiring the old job | ✓ VERIFIED | Read in full; job-scoped permissions, `env:`-routed values (no script injection), attests both archives + checksums. |
| `scripts/install.sh` (rewritten) | tar.gz-aware, verify-before-extract installer | ✓ VERIFIED | Read in full + 33 e2e tests pass. |
| `e2e/release_e2e_test.go` (rewritten) | tar.gz-shape release/install e2e coverage | ✓ VERIFIED | 21 named tests, all pass under `-race`. |
| `README.md` (refreshed) | D-16 install-path documentation | ✓ VERIFIED | Read in full; covers install.sh, brew tap, manual path, `go install` caveat, D-15 verify commands, PLATFORM-NOTES.md link. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| Makefile `LDFLAGS -X` paths | `internal/version.{version,commit,buildDate}` | name-for-name `-X` symbol match | ✓ WIRED | Confirmed by building a binary and observing the stamped values appear correctly. |
| `.goreleaser.yaml` builds.ldflags `{{.Env.VERSION\|COMMIT\|DATE}}` | Makefile's `release`/`release-snapshot` targets exporting the same vars | shared env-var contract (D-05) | ✓ WIRED | Confirmed live: my `make release-snapshot` run stamped the snapshot binaries with the same VERSION the Makefile computed (`0.1.0-rc.9-175-g2911788`), with no drift. |
| `release.yml`'s `make test` + `make lint` (blocking) | `make release` (goreleaser publish) | sequential steps, no `continue-on-error` | ✓ WIRED | Read the workflow: `make test` and `make lint` are unconditional prior steps to `make release`; a red gate would fail the job before publish. |
| `release.yml` VERSION/COMMIT/DATE | shell `env:` block, never inlined `${{ }}` into `run:` script text | script-injection defense | ✓ WIRED | Confirmed by reading the workflow YAML directly — values are exported via `env:` and referenced as `$VERSION`/`$COMMIT`/`$DATE`. |
| `scripts/install.sh` asset-name construction | goreleaser's default `{{.ProjectName}}_{{.Version}}_{{.Os}}_{{.Arch}}.tar.gz` template | proven against real `make release-snapshot` output | ✓ WIRED | I extracted my own snapshot archive and confirmed the naming pattern install.sh constructs matches exactly. |
| `.goreleaser.yaml` `brews:` stanza | `castocolina/homebrew-tap` repo via `HOMEBREW_TAP_GITHUB_TOKEN` | goreleaser `brews:` publish | ⚠️ WIRED IN CONFIG, NOT YET LIVE | The config is correct and a snapshot run generates a well-formed formula locally; the actual publish target repo does not exist yet (confirmed 404), so this link has never fired for real. Human action required — see human_verification item 2. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `gitid --version` stamped correctly | `make build && ./bin/gitid --version` | `gitid version 0.1.0-rc.9-175-g2911788 (none, unknown, darwin/amd64)` | ✓ PASS |
| `gitid version --json` stable schema | `./bin/gitid version --json` | `{"schema":"gitid.version/v1","version":"...","commit":"none","build_date":"unknown","platform":"darwin/amd64"}` | ✓ PASS |
| `make release-snapshot` produces D-07 artifacts | `make release-snapshot` (after installing pinned goreleaser v2.18.0) | 4 tar.gz archives + 1 checksums.txt + Homebrew formula, all correctly named | ✓ PASS |
| Extracted release archive runs and reports correct version | `tar -xzf gitid_*_darwin_amd64.tar.gz -C /tmp/x && /tmp/x/gitid --version` | Matches snapshot version exactly | ✓ PASS |
| Release/install e2e subset | `go test -tags e2e -race -run 'TestRelease_\|TestInstallScript_' ./e2e/...` | 33 tests passed | ✓ PASS |
| ssh -V distro-suffix parser | `go test ./internal/platform/... -run TestParseSSHVersion -v` | 7 tests passed | ✓ PASS |
| Full non-e2e suite under -race | `go test -race $(go list ./... \| grep -v /e2e)` | All packages pass | ✓ PASS |
| Lint | `make lint` | 0 issues (golangci-lint, gosec-embedded, shell POSIX check, tagged vet) | ✓ PASS |
| Build | `go build ./...` | Clean | ✓ PASS |
| Homebrew tap repo existence | `curl -sI https://github.com/castocolina/homebrew-tap` | HTTP 404 | ✗ CONFIRMS PENDING (expected — human_needed) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| PLAT-03 | 10-02, 10-03 | Linux validation end-to-end + portability gaps fixed/logged | ✓ SATISFIED (automated half) / ? NEEDS HUMAN (manual Bazzite execution) | Fedora container job wired correctly and structurally proven; Bazzite UAT mechanism real but not yet executed on hardware. |
| BUILD-03 | 10-01, 10-04, 10-05 | goreleaser tar.gz pipeline, versioned binary, install script, Homebrew tap | ✓ SATISFIED (pipeline/artifact-shape) / ? NEEDS HUMAN (live tag-triggered publish + tap repo) | `.goreleaser.yaml`/`release.yml`/`internal/version`/`scripts/install.sh` all verified directly; the actual GitHub-side publish requires human-created infrastructure that does not exist yet. |
| BUILD-05 | 10-05 | curl\|bash installer, checksum-verified | ✓ SATISFIED | `scripts/install.sh` rewritten for tar.gz shape, verify-before-extract confirmed, 33 e2e tests pass. REQUIREMENTS.md's BUILD-05 row correctly annotated as re-opened/re-verified against Plan 10-05 (commit `2299a6b`). |

No orphaned requirements found — REQUIREMENTS.md's PLAT-03/BUILD-03/BUILD-05 rows all map to a plan in this phase.

**Note on REQUIREMENTS.md/ROADMAP.md checkbox state:** As of this verification, `.planning/ROADMAP.md`'s Phase 10 entry and its 6 plan checkboxes are still unchecked (`[ ]`), and `.planning/REQUIREMENTS.md`'s PLAT-03/BUILD-05 checkboxes are still `[ ]` ("Pending"). This is **consistent with the project's own established workflow** — these get flipped to `[x]` by a "mark phase complete" commit that follows phase verification (as seen for every prior phase, e.g. `17e2825 chore(09.4): mark phase complete`) — and is not itself a phase-goal gap. It does mean the docs are honest: they do not currently overclaim completion beyond what has actually been verified.

### Anti-Patterns Found

None. Scanned every file touched by this phase's plans (`internal/version/*`, `internal/platform/version*.go`, `cmd/gitid/{main,version_cmd,ci_fedora_test,goreleaser_config_test,release_yml_test}.go`, `scripts/install.sh`, `.goreleaser.yaml`, `.github/workflows/{ci,release}.yml`, `e2e/release_e2e_test.go`, `PLATFORM-NOTES.md`, `README.md`, `Makefile`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers — zero matches.

### Human Verification Required

### 1. Bazzite Manual UAT Execution

**Test:** Run all 5 items in `bazzite-uat-checklist.md` on real Bazzite hardware.
**Expected:** Each item resolves to a real PASS/FAIL/accepted-limitation finding; `PLATFORM-NOTES.md`'s 5 "Pending manual UAT" rows get updated with real evidence.
**Why human:** Requires physical Bazzite hardware with real SELinux enforcement, a real Wayland desktop session, and real terminal emulators — none of which any automated agent/CI sandbox can provide. This is the explicit, by-design D-01(ii)/D-03 human residue, intended to run once per release (not a one-time phase gate this execution could have closed itself).

### 2. Homebrew Tap + Live Tagged Release

**Test:** Create the `castocolina/homebrew-tap` GitHub repo, mint a scoped PAT, add it as the `HOMEBREW_TAP_GITHUB_TOKEN` secret, then push a real `v*` tag and observe `release.yml` run to completion.
**Expected:** A live GitHub Release with 5 attested assets, plus a `Formula/gitid.rb` commit pushed to the tap repo; `brew install castocolina/homebrew-tap/gitid` installs a working, correctly-stamped binary.
**Why human:** Repo creation and PAT minting require the user's own GitHub account — confirmed live that the tap repo does not exist yet (404). `10-04-SUMMARY.md` already names this as an explicit `user_setup` item its own frontmatter could not close.

### Gaps Summary

No code, wiring, or test gaps were found. Every artifact this phase's plans committed to exists, is substantive, is correctly wired, and — everywhere I could exercise it locally (build, unit tests, the full non-e2e race suite, the targeted release/install e2e subset, and a real `make release-snapshot` run I reproduced myself rather than trusting the SUMMARY) — behaves exactly as documented. Four independent rounds of cross-AI/independent code review converged on 0 findings, and I independently re-derived the two most consequential ones myself (the artifact shape via a live snapshot build, and the checksum-before-extract security property via source reading + a passing negative test).

The two items left open are not implementation gaps: they are **extrinsic, human-only actions** the phase's own design and its own SUMMARY documents already named as out of an agent's reach — running the Bazzite checklist on real hardware, and creating the Homebrew tap repo/PAT so a real tag push can complete the live publish path. Both are honestly reflected as "Pending" in the project's own ledgers (`PLATFORM-NOTES.md`, `10-04-SUMMARY.md`, `REQUIREMENTS.md`), not silently claimed as done.

**Recommendation:** This phase's code and infrastructure are ready to ship. The two human_needed items above should be resolved by the developer (or explicitly and consciously deferred, per this project's own established pattern of gating the actual v1.0.0 tag cut on a "blocking-human decision," as Phase 9.3 did for its first prerelease tag) before treating PLAT-03/BUILD-03 as fully, terminally closed in REQUIREMENTS.md. Neither blocks proceeding with the milestone-completion workflow, provided the developer consciously acknowledges them.

---

_Verified: 2026-09-05_
_Verifier: Claude (gsd-verifier)_
