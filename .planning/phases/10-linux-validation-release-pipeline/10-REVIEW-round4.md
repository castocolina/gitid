---
phase: 10-linux-validation-release-pipeline
reviewed: 2026-09-05T03:33:30Z
depth: deep
files_reviewed: 27
files_reviewed_list:
  - .github/workflows/ci.yml
  - .github/workflows/release.yml
  - .gitignore
  - .goreleaser.yaml
  - .golangci.yml
  - Makefile
  - PLATFORM-NOTES.md
  - README.md
  - cmd/gitid/ci_fedora_test.go
  - cmd/gitid/goreleaser_config_test.go
  - cmd/gitid/identity_test.go
  - cmd/gitid/main.go
  - cmd/gitid/main_test.go
  - cmd/gitid/release_plumbing_test.go
  - cmd/gitid/release_yml_test.go
  - cmd/gitid/version_cmd.go
  - cmd/gitid/version_cmd_test.go
  - e2e/harness_test.go
  - e2e/release_e2e_test.go
  - internal/platform/version_test.go
  - internal/tuikit/app.go
  - internal/tuikit/app_test.go
  - internal/version/version.go
  - internal/version/version_test.go
  - scripts/install.sh
  - .planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 10: Code Review Report — Round 4 (fresh, independent reviewer)

**Reviewed:** 2026-09-05
**Depth:** deep
**Files Reviewed:** 27 (everything changed since 3ef374bf47c3c9b3b0db5580a7a5fbbb4aab3c4e)
**Status:** clean — 0 findings of any severity

## Summary

This is round 4 of the code-review gate for Phase 10 (Linux Validation +
Release Pipeline). I reviewed independently from scratch — no finding from
rounds 1-3 was taken on faith; every claim in the prompt and in the prior
review reports was re-derived against the current source with real command
execution.

**Build/test/lint, executed directly:**
- `go build ./...` — clean, no output.
- `go test -race $(go list ./... | grep -v /e2e)` — all packages pass
  (`cmd/gitid` 65s, `internal/dummytui` 3.3s, `internal/uploader` 2.7s, rest
  cached/fast).
- `make lint` (`lint-tagged` + `lint-shell` + `golangci-lint run ./...`) —
  0 issues, including the `screenshot`-tagged sub-lint.
- `make fmt` — produced no diff (`git status --short` unchanged after).
- `go test -tags e2e -race -run 'TestRelease_|TestInstallScript_' ./e2e/...`
  — all release/install e2e cases pass (32.4s), confirming the D-07 tar.gz
  contract, checksum verification, OS/arch matrix, and PATH-hint behavior all
  still hold under the rewritten suite.
- Every named regression test from rounds 1-3 re-run individually and green:
  `TestReleaseWorkflow*`, `TestCIWorkflow*`, `TestFedoraJob*`,
  `TestGoreleaserConfig*`, `TestVersionCmd*`, `TestComposeVersion`,
  `TestVersionString*`, `TestSetupEnvTargetsNeverInstallUnpinnedGosec`.

**Verification item 1 (gosec):** `grep -rn "gosec" Makefile` shows only
comment/echo text explaining the embedded-linter-only decision — no
`go install .../gosec@...` line exists anywhere in `setup-env` or
`setup-env-release`. `.golangci.yml` still lists `gosec` as an enabled
linter with no exclusions. Confirmed both targets are byte-clean.

**Verification item 2 (regression test proves the gosec fix, not just
documents it):** I reverted the Makefile fix myself — reinserted
`go install github.com/securego/gosec/v2/cmd/gosec@latest` into the
`setup-env` recipe body — and re-ran
`TestSetupEnvTargetsNeverInstallUnpinnedGosec`. It failed exactly as
expected, naming `setup-env` and printing the offending recipe body. I then
restored the Makefile and confirmed `git diff` against `HEAD` was empty and
the test passed again. The regression guard is real, not decorative.

**Verification item 3 (README one-liner fix, reproduced against a real
shell):** I built a trivial fixture script that prints
`GITID_VERSION`/`GITID_INSTALL_DIR` as seen from inside a piped `sh`, and ran
both shapes:
- `GITID_VERSION=v1.0.0 GITID_INSTALL_DIR=/tmp/x cat fixture.sh | sh` (the
  OLD, broken form — env-var prefix on the `curl`/`cat` side of the pipe) →
  both variables printed as `<unset>` inside the piped shell.
- `cat fixture.sh | GITID_VERSION=v1.0.0 GITID_INSTALL_DIR=/tmp/x sh` (the
  CURRENT README form, README.md:98-101) → both variables correctly seen
  inside the piped shell.

This independently confirms round 3's WR-02 finding was real and round 3's
fix (commit `c10708f`) is the correct, working shape.

**Verification item 4 (test suite health):** No flaky or newly-broken tests
observed. The targeted e2e release/install subset ran clean under `-race`.
The full non-e2e suite ran clean under `-race`. `go vet` (via `lint-tagged`,
covering every isolated build tag: `screenshot`, `smoke`, `e2e`,
`realaccount`, `realaccountgitlab`) is clean.

**Round 1/2/3 fix survival, spot-checked directly:**
- Round 3's IN-01 (stale `checksums` target entry in the Makefile header) —
  confirmed gone; `grep -n checksums Makefile` now only matches prose in the
  `release`/`release-snapshot` doc comments, describing the artifact these
  goreleaser-driven targets produce, not a stale target reference.
- Round 3's IN-02 (contradictory "installs gosec" wording in two places) —
  confirmed both call sites now correctly describe `setup-env-release` as
  golangci-lint's embedded gosec only, in both the Makefile top-of-file
  target list and `release.yml`'s step name (`golangci-lint incl. embedded
  gosec + goreleaser only`).
- Round 3's IN-03 (missing regression test) — confirmed present as
  `TestSetupEnvTargetsNeverInstallUnpinnedGosec` and independently proven to
  catch the regression (see item 2 above).
- Round 2's WR-01/IN-01 (gosec pinning, install.sh PATH-glob false positive)
  — still correctly resolved; no standalone gosec anywhere, and the
  `scripts/install.sh:160-169` comment documenting the false-positive glob
  concern is still accurate (POSIX `case` quoting turns off glob
  interpretation for the quoted `${INSTALL_DIR}` substring).
- Round 1's script-injection fix in `release.yml` (values routed through
  `env:`, never inlined via `${{ }}` into a `run:` script body) — still
  intact; `TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts` and its
  companion `TestFindInlinedRunExpressionCatchesSingleLineRunSteps`
  negative-control both pass, and I read `release.yml` end-to-end to confirm
  `VERSION`/`COMMIT`/`DATE` are still referenced only via `env:` + shell
  `$VAR`, never `${{ steps.relver.outputs.* }}` inlined into script text.

**Additional adversarial checks performed, nothing found:**
- No hardcoded secrets/tokens in any file changed since the phase-planning
  point (only legitimate `secrets.GITHUB_TOKEN` / `secrets.HOMEBREW_TAP_GITHUB_TOKEN`
  references, correctly scoped to the `release` job's `env:` block).
- No `TODO`/`FIXME`/`XXX`/`HACK`/`console.log`/`debugger;` artifacts in any
  changed file.
- `install.sh`'s extraction step (`tar -xzf "$asset" gitid`) still pulls only
  the exact, known member name — no glob, no arbitrary-archive-entry path
  traversal.
- `install.sh`'s checksum lookup remains the WR-02 (round-1) exact-field
  `awk` match (`$2 == name`), not a regex, so `GITID_VERSION`/derived
  filenames containing regex metacharacters cannot cause a false match.
- `.goreleaser.yaml`'s `dist:` stays pointed at `dist`, distinct from the
  Makefile's own `bin/` build-cross output (no `--clean` collision risk).
- `release.yml`'s provenance attestation (`subject-path:`) still covers both
  `dist/*.tar.gz` and `dist/*_checksums.txt`.
- `internal/version.Resolve()` / `resolveFromBuildInfo()` fallback logic
  (go-install-stamped, plain-build-clean, plain-build-dirty, `ok==false`
  cases) is fully covered and each case is asserted with an exact expected
  `Info{}` value — no gaps.
- `internal/tuikit.WithVersion` is additive-only by both a positive
  (`TestWithVersionAddsHelpOverlayLine`, asserting every base line survives
  verbatim) and a receiver-mutation (`TestWithVersionDoesNotMutateReceiver`)
  test — no risk to the 20+ existing `App` callers that never invoke it.

## Verdict

**This phase's code is genuinely clean for release.** Zero Critical/Blocker
findings, zero Warnings, zero Info items — a full 0-finding pass. All
functional claims from the round-3 fix commit (`c10708f`) were independently
reproduced with real command execution (not re-read from the commit message
or prior review text), including a RED/GREEN cycle proving the new
regression test actually catches the regression it was written to catch, and
a live shell reproduction proving the corrected README one-liner works and
the old documented form did not. `go build`, `go test -race` (all non-e2e
packages), the targeted release/install e2e subset under `-race`, `make
lint`, and `make fmt` are all clean with no diffs. The code-review gate can
close for this phase.

---

_Reviewed: 2026-09-05_
_Reviewer: Claude (gsd-code-reviewer), round 4_
_Depth: deep_
