---
phase: 05-identity-manager
plan: 06
status: complete
completed: 2026-08-25
tags: [identity-manager, action-menu, delete-safety, key-ceremony, rotate, repair]
commits:
  - 79b5c9d
  - e4415cd
  - e02fc90
---

# Plan 05-06 Summary

Implemented the identity-manager render layer for the approved action menu, delete additions, and rotate/repair key ceremonies.

## Delivered

- Added the distinct `RotateIdentity` action and the approved four-row action menu. The key row routes through `KeyActionFor` to rotate or repair without a boolean-overloaded action.
- Added the named `IdentityPlanner` seams with noop fixture support and fail-closed handling for action, delete-plan, and key-ceremony planning failures.
- Rendered delete plans from `DeletePlanView`, including shared-key ownership notes, provider-removal targets, backup paths, unmanaged-reference hits, and the domain-owned disclaimer.
- Moved the delete warning block before the bounded exact-change preview so the full unmanaged-scan disclaimer remains visible within the fixed pane geometry.
- Implemented the shared key ceremony with the existing two-stage gate, async commit handling, result receipts, rotate-only archive notice, and rotate-only grace-window hint.
- Registered new frozen copy for the shared-key note, overflow suffix, scan hit/disclaimer, delete safety text, grace-window hint, and archive-path label.

## Resolutions

- Shared-key ownership is plural-safe: all sibling names are comma-joined when they fit; overflow shows whole names plus a remaining count, with the complete list retained in the preview block.
- Long unmanaged scan excerpts use the existing preview clipping convention and cue; no new truncation rule was introduced.
- The unmanaged-scan disclaimer is rendered from `DeletePlanView.Disclaimer`, which carries `identity.UnmanagedScanDisclaimer`; `tuikit` does not redeclare the literal.
- Delete warnings render before the constrained preview block so the fixed disclaimer cannot be clipped by the pane’s line budget.

## Verification

Passed:

- `go test ./internal/tuikit/... -run TestDeleteConfirmTwoHitsRenderDisclaimer -v`
- `go build ./...`
- `make lint`
- `TERM=dumb SSH_AUTH_SOCK= go test -race ./...`
- `go test ./internal/tuikit/... ./internal/dummytui/... -run 'KeyCeremony|Rotate|Repair|Grace'`
- `make gate-copy-freeze`
