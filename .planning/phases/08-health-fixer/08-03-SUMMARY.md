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
