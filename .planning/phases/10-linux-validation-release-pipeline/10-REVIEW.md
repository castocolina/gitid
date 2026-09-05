---
phase: 10-linux-validation-release-pipeline
reviewed: 2026-09-04T00:00:00Z
depth: deep
files_reviewed: 23
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
findings:
  critical: 0
  warning: 3
  info: 1
  total: 4
status: issues_found
---

# Phase 10: Code Review Report — Round 1

**Reviewed:** 2026-09-04
**Depth:** deep (cross-file, security-focused re-verification)
**Files Reviewed:** 23
**Status:** issues_found (no Critical/Blocker findings; 3 Warnings, 1 Info)

## Summary

Scope: everything changed between `3ef374b` (phase-planning-complete) and `HEAD`
on `gsd/phase-10-linux-validation-release-pipeline` — the `internal/version`
hybrid version-resolution package, the `gitid version` subcommand, the additive
TUI help version line, the new `fedora:latest` CI job, the new
`.github/workflows/release.yml` goreleaser-driven publish workflow plus
`.goreleaser.yaml`, the rewritten `scripts/install.sh` D-14 tar.gz installer,
and their extensive test coverage (unit, config-string, and real-subprocess e2e
tests). Docs-only changes (`PLATFORM-NOTES.md`, `README.md`) were also read for
factual accuracy against the code they describe.

**On the explicit ask — independently re-verifying the ci-cd-script-injection
fix (commit b08256c):** the fix is correct and complete for the file as it
stands today. `release.yml`'s `make release (goreleaser publish)` step routes
`VERSION`/`COMMIT`/`DATE` (which ultimately derive from the pushed tag name)
through the step's `env:` block and references them as ordinary shell
variables (`$VERSION`/`$COMMIT`/`$DATE`), never inlining a `${{ }}` expression
into the `run:` script body. I found no other `${{ }}`-into-`run:` inlining
anywhere in `release.yml` or in `ci.yml`'s new `fedora` job (the only `${{ }}`
occurrences left in `ci.yml` are in `with:`/`name:`/`runs-on:` fields, which
are not shell-interpreted and are not an injection vector). SHA-pinning is
consistent across every new `uses:` line in both workflows (40-hex commit SHA
+ `# vX.Y.Z` comment). `scripts/install.sh`'s D-14 verify-before-extract
ordering is genuinely enforced in the current script: both the archive and
the checksums manifest are downloaded, the SHA-256 is looked up and compared
*before* any `tar -xzf` runs, and `tar -xzf "$asset" gitid` extracts only the
one named member (no glob, no path-traversal surface).

The three Warnings below are not live vulnerabilities in the current diff —
they are gaps in the regression guards / test assertions that exist
specifically to keep the current, correct state from regressing, plus one
functional-precision nit in `install.sh`'s checksum lookup. Given this phase's
whole premise is "prove the fix holds," a test that cannot actually catch the
next occurrence of the same bug class deserves to be raised rather than
waved through.

## Warnings

### WR-01: The script-injection regression guard has a blind spot for single-line `run:` steps

**File:** `cmd/gitid/release_yml_test.go:28-51` (`TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts`)
**Issue:** The loop's own control flow skips checking the `run:` line itself
for an inlined `${{ }}` expression:

```go
if strings.HasPrefix(trimmed, "run:") {
    inRun = true
    runIndent = indent
    continue          // <-- the line that matched "run:" is never scanned
}
if inRun {
    if trimmed != "" && indent <= runIndent {
        inRun = false
    } else if strings.Contains(line, "${{") {
        t.Fatalf(...)
    }
}
```
Today every `run:` step in `release.yml` either uses the safe `run: |` block
form (whose *continuation* lines this loop does correctly scan) or a
single-line form with no `${{ }}` in it (`run: make setup-env-release`, `run:
make test`, `run: make lint`) — so this gap causes no false negative right
now. But this test's entire reason to exist is to prevent the exact class of
bug the orchestrator just fixed (b08256c) from being reintroduced. A future
single-line step such as
```yaml
- run: make release VERSION=${{ steps.relver.outputs.version }}
```
would reintroduce the identical script-injection vulnerability against a job
carrying `contents:write`/`id-token:write`/`attestations:write` — and this
regression guard, as written, would report PASS.
**Fix:** Check the `run:` line itself too, before (or in addition to) setting
`inRun`:
```go
if strings.HasPrefix(trimmed, "run:") {
    if strings.Contains(line, "${{") {
        t.Fatalf("line %d: single-line run: step inlines a ${{ }} expression directly: %s", i+1, trimmed)
    }
    inRun = true
    runIndent = indent
    continue
}
```
Consider also porting this same guard (or a lighter-weight version) to
`ci.yml`'s new `fedora` job and its other `run:` steps — see IN-01.

### WR-02: `install.sh`'s checksum-line lookup embeds an unescaped, user-controlled version string into a `grep -E` pattern

**File:** `scripts/install.sh:120`
**Issue:**
```sh
expected=$(grep -E "^[0-9a-f]{64}  ${asset}\$" "${tmp}/${checksums_name}" | head -n 1 | cut -d ' ' -f 1)
```
`${asset}` is `gitid_${version_num}_${os_tag}_${arch_tag}.tar.gz`, and
`version_num` is `${tag#v}` where `tag` is taken verbatim from the
user-supplied `GITID_VERSION` environment variable when set (line 77,
`tag="$GITID_VERSION"`, no validation). `os_tag`/`arch_tag` are allowlisted,
but `version_num` is not, and it is spliced unescaped into an **extended
regular expression**. Any regex metacharacter in a version string —
critically `.`, which is ubiquitous in version tags (`v1.2.3`) — is
interpreted as "match any character" rather than a literal dot, weakening the
precision of the one check the whole script exists to get right (D-14
verify-before-extract). This is not exploitable as a shell/command injection
(it never reaches `sh -c` or `eval`), and today's os/arch allowlisting plus
the requirement that the *computed* SHA-256 also match means a wrong match
would still very likely fail the final `[ "$expected" != "$actual" ]`
comparison — but a user-supplied tag containing regex-special characters
(e.g. `v1.0.0(rc1)`, unbalanced parens) could make the pattern fail to compile
under some `grep -E` implementations, or match a broader/narrower line than
intended, causing a spurious "no entry for" refusal or (in a contrived case)
a wrong-but-well-formed 64-hex line to be picked up.
**Fix:** Match the asset name literally instead of as a regex. Since `grep`
has no fixed-string equivalent of `$` anchoring combined with `-F`, split the
match instead of building one combined pattern, e.g.:
```sh
expected=$(awk -v name="$asset" '$2 == name { print $1; exit }' "${tmp}/${checksums_name}")
```
(the checksums file's two-space-separated `<hash>  <filename>` format makes
this an exact-field match with no regex involved), or escape `asset` for
`grep -E` before interpolating it.

### WR-03: `TestReleaseWorkflowJobHasExactlyThreeScopedPermissions` doesn't assert "exactly three"

**File:** `cmd/gitid/release_yml_test.go:60-71`
**Issue:** The test's name promises the `release` job has *exactly* three
scoped permissions, but the body only asserts these three strings are
*present* (`contents: write`, `id-token: write`, `attestations: write`) — it
never asserts the permissions block contains *nothing else*. A future PR that
adds a fourth permission (e.g. `actions: write`, widening this
secret-bearing job's blast radius) would pass this test silently, even
though its name claims to guard against exactly that. This repository's own
Makefile has an explicit precedent for this exact class of bug (see the
`gate-copy-freeze` header comment describing WR-08: "the sentence was never
added to the list — a false verification claim") — a test whose name
overstates what it checks is the same failure mode.
**Fix:** Assert the permissions block has exactly 3 non-comment lines under
the `permissions:` key, e.g. extract the block between `permissions:` and the
next same-or-lower-indent key and count lines matching `^\s+\w[\w-]*:\s`.

## Info

### IN-01: The `${{ }}`-into-`run:` script-injection regression guard exists only for `release.yml`

**File:** `.github/workflows/ci.yml` (`fedora` job, lines 110-159); `cmd/gitid/release_yml_test.go`
**Issue:** `ci.yml`'s `fedora` job runs several `run:` steps too (`dnf
install …`, `git config --global --add safe.directory "$GITHUB_WORKSPACE"`,
`make test`, `make lint`, `make test-e2e`) and none of them inline a `${{ }}`
expression today, but there is no equivalent regression test guarding this
job the way `TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts` guards
`release.yml`. The blast radius is much smaller here (`ci.yml`'s top-level
`permissions: contents: read`, no repository secret referenced anywhere in
the file per its own header comment), so this is Info rather than Warning,
but it's a one-line generalization of an already-written test.
**Fix:** Factor `TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts`'s
body into a shared helper parameterized on the workflow path, and call it for
both `ci.yml` and `release.yml`.

---

_Reviewed: 2026-09-04_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
