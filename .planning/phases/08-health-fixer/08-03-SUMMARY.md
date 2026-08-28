# 08-03 SUMMARY — Orphan tolerance, reserved archive paths, and Files parse gates

## Outcome

All three tasks are complete. Healthy SSH-only managed blocks are informational and report-only; doctor dependency key paths exclude gitid archive material; and parse failures now produce critical Files findings that replace ordinary Health/Fixer section output and suppress same-section JSON findings.

## Task 1 — CheckOrphans Class 1 downgrade

`CheckOrphans` now emits `SeverityInfo` with `Fix: nil` for an SSH managed block with no Git counterpart. The corrected copy deliberately covers both valid on-disk histories: the block may have been created SSH-only, or its Git side may have been removed intentionally. This intentionally diverges from the `08-UI-SPEC.md` draft that asserted a git-only delete, because that history is not knowable from the common SSH-only on-disk shape.

## Task 2 — Reserved archive key paths

`buildDoctorDeps` filters `Deps.KeyPaths` through `sshconfig.IsReservedPath(sshDir, path)` immediately after collection. This mirrors `identity.filterReservedKeyPaths`: archived material under `sshconfig.ArchiveDir(sshDir)` cannot reach CheckOrphans' unused-key cross-reference.

## Task 3 — HLTH-02 Files family

- Added `doctor.FamilyFiles` after `FamilyRedundancy` in `Families()` and wired `CheckFiles` after the existing checks.
- Amended `SeverityCritical` documentation without changing its numeric value: it covers Permissions key/secret exposure and Files total parse failure, both exit code 3.
- Added parse metadata from `doctor.Finding` through the cmd conversion boundary to the backend-free TUI DTO.
- Health and Fixer select the dedicated parse-error frame only for `Family == "Files" && Severity == critical`; a Permissions-critical finding does not trigger the frame.
- `gitid health --json` drops ordinary findings in a section that has a critical Files finding.

## Tests

New/updated regression coverage includes:

- SSH-only orphan findings are info-only, no-fix, and name both possible histories.
- Archive paths are excluded from real doctor dependency construction.
- Missing SSH config and malformed Git fragment checks produce critical Files findings.
- Both Files-critical and Permissions-critical findings return exit code 3.
- Parse-error render selection is Family-specific and suppresses ordinary same-section rows.
- JSON suppression preserves the Files finding while removing ordinary same-section findings.

## Gates

All commands were run in this worktree:

- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./internal/doctor/checks/... -run TestOrphan` — `16 passed in 1 packages`.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./cmd/gitid/... ./internal/doctor/checks/... -run 'TestDeps|TestArchive|TestOrphan'` — `20 passed in 2 packages`.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./internal/doctor/... ./internal/tuikit/... ./cmd/gitid/... -run 'TestCheckFiles|TestParseError|TestFamilyFiles|TestExitCodeCriticalBothTiers|TestHealthJSONParseErrorSuppression'` — `5 passed in 4 packages`.
- `go build ./...` — exit 0.
- `go vet -tags e2e ./...` — exit 0.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — `2081 passed in 21 packages`.
- `GOLANGCI_LINT_CACHE="$PWD/.golangci-cache" make lint` — `0 issues`; the worktree-local cache was removed afterward.
- `make gate-visual-regression` — PASS; `41 RequiredScreenSpecs frames checked`.
- `make test` — PASS, including `gate-copy-freeze`.
- `make test-e2e` — PASS: `ok github.com/castocolina/gitid/e2e 598.288s`.

The visual/test gates rewrote prior-wave UI frame status counts because the new Files finding changes fixture finding totals. Those generated frame changes were inspected and reverted because they were not plan-owned render changes.

## Bugs found

The initial Git parse-error classifier treated `exit status 128` as `exit status 1` through a substring check, silently suppressing real parse failures. The new `TestCheckFilesReportsCriticalGitParseFailure` exposed it; the classifier now accepts only actual status-1 key absence and reports status 128 as a critical Files finding.

**Real bug found and fixed post-executor (orchestrator pass, empirically verified):** `CheckFiles`'s original implementation treated a MISSING `~/.ssh/config` or `~/.gitconfig`/fragment the SAME as a genuinely CORRUPTED one — both produced the critical, "checks paused" alarm. Manually running `gitid health --json` against a completely fresh home (no config files at all — the single most common state any new user hits, before creating a first identity) showed TWO critical, alarming errors on the very first health check. This is a fresh instance of exactly the false-positive-loop class this wave exists to close, and contradicts this project's own established convention: `CheckBaseline` already treats an entirely-missing artifact as an actionable ERROR with a real fix, never CRITICAL with nothing to do but wait. Fixed: `checkSSHConfig`/`checkGitConfig` now tolerate `os.IsNotExist` (return no finding) and report critical ONLY for a genuine read/parse failure on a file that exists. This is a deliberate, documented divergence from the plan's own literal "missing or failing to parse" phrasing (`08-03-PLAN.md` Task 3's `<behavior>` block) — the SAME scoped-divergence discipline this wave's own Task 1 already established for the Class-1 orphans copy. `TestCheckFilesReportsMissingSSHConfig` (which asserted the OLD, now-corrected behavior) was replaced with `TestCheckFilesToleratesMissingSSHConfig` (asserts zero findings) plus two new tests proving the distinction holds: `TestCheckFilesReportsGenuineSSHReadFailure` (a non-`ErrNotExist` read error still reports critical) and `TestCheckFilesToleratesMissingGitConfigFragment` (the Git-side mirror of the tolerance). Verified empirically before/after: a fresh home now shows zero Files findings (only the pre-existing, correctly-scoped Baseline "not installed" error), while a genuinely corrupted `~/.gitconfig.d/` fragment still correctly produces the critical Files finding with proper same-section suppression.
