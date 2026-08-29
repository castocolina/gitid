---
phase: 09-upload-credentials-assist
plan: 03
subsystem: upload-credentials-assist
tags: [uploader, github-cli, gitlab-cli, inventory, ssh, security]
requires:
  - phase: 09-upload-credentials-assist
    plan: 02
    provides: uploader seam, real command dependency wiring, and upload view DTOs
provides:
  - Per-registration upload results and title requests for independent GitHub and combined GitLab registration
  - Content-validated public-key uploads, provider inventory, validated remote deletion, and machine-scoped titles
  - Stable failure classification with bounded token- and home-path-redacted CLI output
affects: [09-04, 09-05, 09-06, 09-08]
actuals:
  tasks: 3
  commits: 3
tech-stack:
  added: []
  patterns: [per-registration request DTOs, fail-soft provider inventory, validated destructive provider IDs]
key-files:
  created:
    - internal/uploader/inventory.go
    - internal/uploader/inventory_test.go
    - internal/uploader/classify.go
    - internal/uploader/classify_test.go
  modified:
    - internal/uploader/uploader.go
    - internal/uploader/uploader_test.go
    - cmd/gitid/wiring.go
key-decisions:
  - "GitLab uses auth_and_signing, its documented combined default, so one key covers both recipe roles."
  - "Inventory errors return a nil slice and error, allowing callers to degrade to upload instead of treating a failed read as an empty account."
  - "Delete IDs are numeric-only before argv construction; GitLab delete has no confirmation flag in the installed CLI help."
patterns-established:
  - "Provider command previews and execution derive from the same argument builder."
  - "Public-key paths are content-validated through injected ReadFile before subprocess execution."
requirements-completed: [UP-01, UP-02, UP-03]
coverage:
  - id: D1
    description: Per-registration uploads, GitLab combined registration, and public-key content validation
    requirement: UP-01
    verification:
      - kind: unit
        ref: internal/uploader/uploader_test.go
        status: pass
    human_judgment: false
  - id: D2
    description: Inventory diff, exact-title lookup, and validated remote deletion
    requirement: UP-02
    verification:
      - kind: unit
        ref: internal/uploader/inventory_test.go
        status: pass
    human_judgment: false
  - id: D3
    description: Scope, conflict, duplicate, and redacted-output classification
    requirement: UP-03
    verification:
      - kind: unit
        ref: internal/uploader/classify_test.go
        status: pass
    human_judgment: false
completed: 2026-08-28
status: complete
---

# Phase 09 Plan 03: Uploader Engine Summary

**The uploader now independently records each registration, validates public-key content before execution, inventories existing provider keys, and classifies provider failures without exposing raw secrets or home paths.**

## Accomplishments

- Added per-registration request/result types, independent batch attempts, GitHub auth/signing support, and GitLab's combined `auth_and_signing` registration.
- Enforced the `.pub` invariant with injected file reads, PEM private-key rejection, and OpenSSH authorized-key parsing before any provider subprocess runs.
- Added provider inventories, normalized key-blob dedupe helpers, exact machine-scoped title matching, numeric remote-key ID validation, and previewable deletion.
- Added scope/conflict/duplicate classifiers plus bounded token- and home-path-redacted raw fallback output.

## Task Commits

1. **Task 1: Per-registration upload engine and public-key validation** — `51c494d`
2. **Task 2: Provider inventory and remote deletion** — `631c89f`
3. **Task 3: Failure classification and output redaction** — `bfcf815`

## Verification

- `go build ./...` — passed.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./internal/uploader/... ./cmd/gitid/...` — passed (643 tests).
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — passed (2206 tests).
- `make lint` — passed after clearing a stale golangci-lint cache that referenced a removed sibling worktree.
- `go list -deps ./internal/uploader` includes the existing `golang.org/x/crypto/ssh` parser and no new dependency was added.
- `git diff go.mod go.sum` — no changes.
- No Go source references `GLabKeyTypeForAuth`.

## Provider Delete Help Observed

- `gh ssh-key delete --help`: `gh ssh-key delete <id> [flags]` and `-y, --yes Skip the confirmation prompt`; the GitHub argv uses `--yes`.
- `glab ssh-key delete --help`: `glab ssh-key delete [<key-id>] [--flags]`; listed flags include help, pagination, and repository selection, with no confirmation flag; the GitLab argv uses no confirmation flag.

## Deviations from Plan

The installed GitLab CLI does not support the planned `-y` delete flag. The implementation uses `glab ssh-key delete <id>` and tests assert the observed argv.

## Issues Encountered

`make lint` initially surfaced five cached gosec diagnostics with paths under a removed sibling worktree. Clearing the golangci-lint cache resolved the stale diagnostics; the subsequent lint run reported zero issues.

## Next Phase Readiness

Plans 09-04 through 09-06 can consume `MissingRegistrations`, `RegistrationRequestsWithTitle`, per-registration results, classifier kinds, and `DeleteRecordedKey` without provider-specific argv logic.

---
*Phase: 09-upload-credentials-assist*
*Completed: 2026-08-28*
