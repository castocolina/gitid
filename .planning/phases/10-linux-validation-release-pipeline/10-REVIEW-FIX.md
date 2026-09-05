---
phase: 10-linux-validation-release-pipeline
fixed_at: 2026-09-05T02:45:00Z
review_path: .planning/phases/10-linux-validation-release-pipeline/10-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 10: Code Review Fix Report — Round 1

**Fixed at:** 2026-09-05
**Source review:** .planning/phases/10-linux-validation-release-pipeline/10-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (0 Critical, 3 Warning, 1 Info — `fix_scope` included Info per explicit request)
- Fixed: 4
- Skipped: 0

**Isolation:** all edits and commits ran in an isolated git worktree
(`gsd-reviewfix/10-62531`, based off `gsd/phase-10-linux-validation-release-pipeline`),
per the fixer's worktree-isolation protocol. The branch was fast-forwarded into
the user's branch and the worktree/temp-branch/recovery-sentinel were cleaned
up transactionally after this report was written.

## Fixed Issues

### WR-01: The script-injection regression guard has a blind spot for single-line `run:` steps

**Files modified:** `cmd/gitid/release_yml_test.go`
**Commit:** `5214ef0`
**Applied fix:** Extracted the scanning loop out of
`TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts` into a pure,
independently-testable `findInlinedRunExpression(src string) (lineNo int, line
string, found bool)` function, and added the missing check on the `run:` line
itself (previously the loop `continue`d immediately on matching the `run:`
prefix, before ever scanning that line's own text for an inlined `${{ }}`
expression — so it only ever caught the `run: |` block-scalar continuation-line
case, never the single-line `run: ...${{ }}...` case).

TDD proof performed before committing the fix: temporarily reverted
`findInlinedRunExpression` to the pre-fix logic (removing only the new
run:-line check) and re-ran the new
`TestFindInlinedRunExpressionCatchesSingleLineRunSteps` table — the
single-line-inlined-expression case reported `found = false` (a real, observed
false PASS on exactly the vulnerability shape the guard exists to catch). The
fix was then restored and the same test re-run to confirm `found = true`.
`TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts` (which scans the
real `release.yml`) continues to pass, since today's `release.yml` has no
actual instance of this bug — the fix closes the blind spot in the guard
itself.

### WR-02: `install.sh`'s checksum-line lookup embeds an unescaped, user-controlled version string into a `grep -E` pattern

**Files modified:** `scripts/install.sh`, `e2e/release_e2e_test.go`
**Commit:** `d44ca14`
**Applied fix:** Replaced the `grep -E "^[0-9a-f]{64}  ${asset}\$" ... | head -n 1 | cut -d ' ' -f 1`
pipeline with an exact-field `awk -v name="$asset" '$2 == name { print $1; exit }'`
match against the checksums manifest's `<hash>  <filename>` two-space format —
no regex involved, so regex metacharacters in an unvalidated `GITID_VERSION`
(e.g. the review's own `v1.0.0(rc1)` example) can no longer change matching
semantics.

Hypothesis→test→implementation discipline followed per this repo's CLAUDE.md:
before writing any fix, manually reproduced the bug at a shell prompt with
both BSD grep (macOS, this host) and GNU grep (Linux, via a throwaway
`docker run debian:bookworm-slim`) — both implementations fail to match the
literal substring `(rc1)` against an unescaped `(rc1)` group in the `-E`
pattern. Added
`TestInstallScript_VersionWithRegexMetacharactersMatchesChecksumExactly`
(`e2e/release_e2e_test.go`), which crafts a real checksums-manifest entry
(genuine SHA-256 of a real renamed archive) for a version tag containing
regex metacharacters and asserts install.sh installs it successfully with
the correct verified checksum. Confirmed RED against the pre-fix script
(`gitid: gitid_1.0.0(rc1)_checksums.txt has no entry for
gitid_1.0.0(rc1)_darwin_amd64.tar.gz — refusing to install an unverified
archive`), then GREEN after the fix.

Also updated `TestInstallScript_FallsBackToShasumWhenSha256sumAbsent`'s
curated minimal-tools PATH list: `grep`/`head` are no longer required by the
checksum-lookup path, `awk` now is (this test intentionally builds a PATH
missing tools install.sh doesn't need, to prove the `shasum` fallback works —
it needed updating to stay accurate after the tool-dependency change).

### WR-03: `TestReleaseWorkflowJobHasExactlyThreeScopedPermissions` doesn't assert "exactly three"

**Files modified:** `cmd/gitid/release_yml_test.go`
**Commit:** `5214ef0` (same commit as WR-01 — both are regression-guard-blind-spot
fixes in the same test file; combined into one commit per this repo's CLAUDE.md
"commit in logical groups" guidance)
**Applied fix:** Added `permissionsBlockEntries(t, block string) []string`,
which extracts a job block's `permissions:` mapping's own entries (lines
strictly indented deeper than the `permissions:` key, up to the next
same-or-lower-indent key) and returns them as a list. The real test now
asserts `len(entries) == 3` in addition to the pre-existing presence checks
for the 3 expected permission strings — a 4th, unexpected permission key
(e.g. `actions: write`) is no longer invisible to this guard.

TDD proof: `TestPermissionsBlockEntriesDetectsExtraPermission` constructs a
synthetic job block with a 4th permission added on top of the 3 expected
ones, and (a) confirms `permissionsBlockEntries` correctly reports 4 entries,
and (b) directly reproduces the PRE-FIX test's own presence-only logic
against that same synthetic block to demonstrate it silently reports "OK"
(all 3 expected substrings present) despite the extra permission — the exact
blind spot WR-03 named.

### IN-01: The `${{ }}`-into-`run:` script-injection regression guard exists only for `release.yml`

**Files modified:** `cmd/gitid/ci_fedora_test.go`
**Commit:** `1e7a6d4`
**Applied fix:** Addressed rather than deferred — this was a small,
low-risk addition. Added `TestCIWorkflowNeverInlinesExpressionsIntoRunScripts`,
which reuses the `findInlinedRunExpression` helper (extracted for WR-01) against
`ci.yml` in full (covering the `fedora` job's `run:` steps: `dnf install`,
`git config --global --add safe.directory`, `make test`, `make lint`, `make
test-e2e`). No source changes were needed to `ci.yml` itself — today's file has
no instance of the bug class; this closes the coverage gap the review flagged.

## Skipped Issues

None — all 4 in-scope findings were fixed.

## Verify Battery (run after all fixes, on the final committed state)

- `go build ./...` — passed
- `TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./...` (all non-e2e packages) — passed, all `ok`
- `make lint` (golangci-lint + gosec + shell POSIX parse check) — `0 issues.` twice (default build tags + `screenshot` tag), `lint-shell` OK for both `scripts/*.sh`
- `make fmt` — clean, no diffs produced
- `go test -count=1 ./cmd/gitid/... -run 'TestReleaseWorkflow'` — all 8 subtests pass
- `go test -count=1 ./cmd/gitid/... -run 'TestFindInlinedRunExpressionCatchesSingleLineRunSteps|TestPermissionsBlockEntriesDetectsExtraPermission|TestCIWorkflowNeverInlinesExpressionsIntoRunScripts'` — all pass
- `go test -tags e2e -race -count=1 -run 'TestInstallScript_' ./e2e/...` — all 17 install-script tests pass, including the new `TestInstallScript_VersionWithRegexMetacharactersMatchesChecksumExactly`
- `make test-e2e` (`go test -tags e2e -race -timeout 2400s ./e2e/...`, the project's own full e2e budget) — `ok`, 1064.8s

All verification ran inside the isolated fixer worktree
(`/Users/ramon/git/personal/ssh-git-config/.claude/worktrees/rf-10-62531-1788574044`,
removed after this report was written); the numbers above are reproducible
from `gsd/phase-10-linux-validation-release-pipeline` after the fast-forward.

---

_Fixed: 2026-09-05_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
