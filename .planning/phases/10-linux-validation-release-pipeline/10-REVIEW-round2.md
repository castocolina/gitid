---
phase: 10-linux-validation-release-pipeline
reviewed: 2026-09-05T03:03:02Z
depth: deep
files_reviewed: 25
files_reviewed_list:
  - .github/workflows/ci.yml
  - .github/workflows/release.yml
  - .gitignore
  - .goreleaser.yaml
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
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 10: Code Review Report — Round 2 (independent, fresh reviewer)

**Reviewed:** 2026-09-05
**Depth:** deep (cross-file, security-focused, with real test execution)
**Files Reviewed:** 25
**Status:** issues_found (0 Critical/Blocker; 1 Warning, 1 Info) — **no blocking issues; this phase's code is genuinely clean for release.**

## Summary

Scope: `3ef374b` (phase-planning-complete) through `HEAD` on
`gsd/phase-10-linux-validation-release-pipeline`, independently re-verified
with no assumption that round-1's findings (10-REVIEW.md) or the fixer's
claims (10-REVIEW-FIX.md) were correctly applied.

**Verification method, not just reading:** beyond reading every changed file,
I actually executed the regression guards and the real e2e battery myself:

- `go build ./...` — clean.
- `TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./...` (every non-e2e
  package) — all `ok`.
- `make lint` — `0 issues.` (golangci-lint incl. embedded gosec, twice: default
  + `screenshot` tag) and `lint-shell` OK for both `scripts/*.sh`.
- `make fmt` — no diff produced.
- The 25 `cmd/gitid` tests named in 10-REVIEW-FIX.md
  (`TestReleaseWorkflow*`, `TestFindInlinedRunExpression*`,
  `TestPermissionsBlockEntries*`, `TestCIWorkflowNeverInlines*`,
  `TestFedoraJob*`) — all pass, individually confirmed with `-v`.
- The WR-02 install.sh fix specifically: I copied the repo into an isolated
  scratch directory, reverted **only** the `expected=...` line back to the
  pre-fix `grep -E "^[0-9a-f]{64}  ${asset}\$" ... | head -n 1 | cut ...`
  form, and re-ran
  `TestInstallScript_VersionWithRegexMetacharactersMatchesChecksumExactly`
  against that reverted copy: it failed exactly as the fixer's report claims
  (`gitid: gitid_1.0.0(rc1)_checksums.txt has no entry for
  gitid_1.0.0(rc1)_darwin_amd64.tar.gz`). Re-running the same test against the
  real, un-reverted `scripts/install.sh` passes. This is a genuine RED→GREEN
  reproduction, not a re-read of the fixer's own narrative.
- `go test -tags e2e -race -count=1 -timeout 300s` for
  `TestRelease_SnapshotBuildProducesArchivesForEveryTarget`,
  `TestRelease_UnstampedBuildKeepsDevDefaults`,
  `TestRelease_ChecksumsManifestMatchesArchives`,
  `TestInstallScript_InstallsVerifiedHostBinary`,
  `TestInstallScript_ChecksumMismatchRefuses`, `TestInstallScript_OSArchMatrix`
  (8 sub-cases), `TestInstallScript_UnsupportedOSRefuses` — all pass against a
  real `make release-snapshot` build (goreleaser, real 4-target archives, real
  SHA-256 checksums manifest).

**On the four explicit re-verification asks:**

1. **release.yml script-injection fix (b08256c) + WR-01/WR-03/IN-01 regression
   guards.** All correct and, importantly, *provably* correct — not just
   plausible. `findInlinedRunExpression` now scans the `run:` line itself (the
   exact blind spot WR-01 named) and its own table test
   (`TestFindInlinedRunExpressionCatchesSingleLineRunSteps`) exercises the
   single-line-inlined-expression shape directly; I traced the logic by hand
   and confirmed the fixed loop cannot re-develop the same blind spot for any
   YAML shape currently used in either workflow. `permissionsBlockEntries`
   (WR-03) correctly extracts only the `permissions:` mapping's own entries
   bounded by indentation, and the real
   `TestReleaseWorkflowJobHasExactlyThreeScopedPermissions` now asserts
   `len(entries) == 3`, not just presence. IN-01's
   `TestCIWorkflowNeverInlinesExpressionsIntoRunScripts` reuses the same
   helper against `ci.yml`. I could not construct a case where these guards
   would report a false PASS on a real script-injection shape in either
   workflow's current YAML formatting conventions.
2. **WR-02's install.sh checksum-lookup fix.** Correct for both GNU and BSD
   `awk`: the fix replaced an unescaped `grep -E` regex match with a literal,
   POSIX-standard `awk -v name="$asset" '$2 == name { print $1; exit }'`
   field-equality comparison — no regex construction at all, so regex
   metacharacters in an unvalidated `GITID_VERSION` can no longer change
   matching semantics. `-v` variable assignment and `==` field comparison are
   POSIX awk, portable across gawk/mawk/busybox-awk/one-true-awk (macOS's
   default) with no behavioral divergence for this construct. I independently
   reproduced both the RED (pre-fix) and GREEN (post-fix) states myself (see
   above) rather than trusting the fixer's write-up.
3. **Other security issues in the new workflows/Go code/install.sh.** Found
   one: see WR-01 below (an unpinned, unused `gosec@latest` binary install
   newly duplicated into the secrets-bearing release job's own bootstrap
   target). No injectable `${{ }}`-into-`run:` usage anywhere else, no
   hardcoded secrets, no path-traversal in `install.sh`'s `tar -xzf "$asset"
   gitid` (exact member name, no glob), no SSRF-style concern in
   `GITID_INSTALL_BASE_URL` (a self-selected env var the same local user
   controls, not an externally-supplied value), and `internal/version`'s
   linker-`-X` seam is correctly gated (never a compiled-in non-empty
   default, so the `debug.ReadBuildInfo()` fallback is never masked).
4. **General code quality / test-vacuity check.** No vacuous tests found in
   the areas I ran: every `TestInstallScript_*`/`TestRelease_*` case asserts
   against artifacts and hashes discovered from a real `make release-snapshot`
   run (never hardcoded literals), and the round-1-fix regression tests
   (`TestFindInlinedRunExpressionCatchesSingleLineRunSteps`,
   `TestPermissionsBlockEntriesDetectsExtraPermission`) each include the
   pre-fix logic's own reproduction to prove the blind spot they close was
   real, not assumed.
5. **Build/test claims verified by actually running them**, not by reading
   code and trusting the fix report — see the Verification method list above.

The one Warning below is a genuine, newly-discovered (not carried over from
round 1) supply-chain/waste issue in the phase's own new
`setup-env-release` Makefile target. It is not exploitable today without a
compromise of an upstream package's release infrastructure, and it mirrors a
pre-existing pattern already present in `setup-env` — but Phase 10 is the one
that duplicated that pattern into the specific bootstrap step now feeding the
`contents:write`/`id-token:write`/`attestations:write` release job, which is
exactly the kind of blast-radius question this phase's own security posture
(D-06, SHA-pinned actions, `golangci-lint`/`goreleaser` version-pinned) cares
about elsewhere. It does not block this phase.

## Warnings

### WR-01: `setup-env-release` installs an unused, unpinned (`@latest`) `gosec` binary into the secrets-bearing release job's bootstrap

**File:** `Makefile:203-204` (pre-existing `setup-env` copy) and
`Makefile:237-238` (**new**, Phase 10 plan 10-04, `setup-env-release`); also
referenced by `.github/workflows/release.yml:53` (`make setup-env-release`)
and comments in `ci.yml`/`release.yml`/`.pre-commit-config.yaml` that all say
"`make lint` (golangci-lint + gosec)".
**Issue:** `make lint` (`Makefile:343-344`) runs exactly one command:
`$(GOLANGCI_LINT) run ./...`. It never shells out to a standalone `gosec`
binary. Security coverage from gosec IS real — `.golangci.yml` lists `gosec`
as one of golangci-lint's own embedded linters (`.golangci.yml:15,23`,
unchanged by this phase) — but the *standalone* `gosec` binary that
`setup-env`/`setup-env-release` install via
`go install github.com/securego/gosec/v2/cmd/gosec@latest` is dead weight: no
Makefile target, pre-commit hook, or CI step ever invokes it directly. This
was already true of `setup-env` before this phase (not a Phase 10 regression
there), but this phase's own new `setup-env-release` target — written
specifically to be the *narrower*, only-what's-needed bootstrap for the
highest-privilege job in the pipeline (its own header comment: "installs ONLY
... the three tools `make test`/`make lint`/`make release` actually need") —
duplicated this exact unused install into that job. Concretely, this means the
`release` job (permissions: `contents: write`, `id-token: write`,
`attestations: write`) now runs `go install .../gosec@v-whatever-latest-
resolves-to-today` — an unpinned, mutable dependency resolution, fetching and
compiling arbitrary upstream Go source — for a tool that provides it zero
actual linting benefit, widening the job's supply-chain surface for nothing.
Every other tool in this exact target (`golangci-lint`, `goreleaser`) is
deliberately version-pinned with an explicit "do NOT change without a fresh
verification" comment; `gosec@latest` breaks that same discipline in the same
target, for no benefit.
**Fix:** Either (a) remove the standalone `gosec` install from
`setup-env-release` (and, ideally, from `setup-env` too, in a follow-up) since
golangci-lint's embedded `gosec` linter already provides the real coverage, or
(b) if a standalone gosec binary genuinely needs to keep existing for some
out-of-repo/manual workflow, pin it to an explicit version the same way
`GOLANGCI_LINT_VERSION`/`GORELEASER_VERSION` are pinned, e.g.:
```make
GOSEC_VERSION := v2.22.4  # pin — verify via git ls-remote --tags before bumping
...
go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
```
Also correct the "`make lint` (golangci-lint + gosec)" comments in
`ci.yml`/`release.yml`/`.pre-commit-config.yaml` to say "golangci-lint
(including its embedded gosec linter)" so a future maintainer doesn't assume
removing the standalone install would silently drop gosec coverage.

## Info

### IN-01: `install.sh`'s PATH-membership check treats `GITID_INSTALL_DIR` as a shell glob pattern, not a literal string

**File:** `scripts/install.sh:160-161`
**Issue:**
```sh
case ":$PATH:" in
	*":${INSTALL_DIR}:"*)
```
`INSTALL_DIR` (from `GITID_INSTALL_DIR`, unvalidated when the user sets it) is
interpolated directly into a `case` pattern, where `*`, `?`, and `[...]` are
glob metacharacters, not literal characters. A custom install dir containing
one of those characters (e.g. `GITID_INSTALL_DIR="$HOME/bin[test]"`) would
make the PATH-membership check match more loosely than the literal directory
string warrants, producing a misleading "PATH: OK" or "PATH: NOT on your
PATH" message. This cannot execute code or escape the `case` construct (case
patterns are not evaluated as commands), and it is entirely self-inflicted by
the same local user who set the variable — not a security boundary — so it is
Info, not Warning; it is the same flavor of "unescaped user value spliced
into a pattern-matching construct" as WR-02, just in a cosmetic message
rather than the checksum-verification path.
**Fix:** Compare literally instead of via a glob pattern, e.g. iterate
`$PATH` split on `:` with a `case` arm that compares each component with `=`,
or use a small awk/loop that does exact string equality rather than case-glob
matching.

---

_Reviewed: 2026-09-05_
_Reviewer: Claude (gsd-code-reviewer), round 2 — independent from round 1_
_Depth: deep_
