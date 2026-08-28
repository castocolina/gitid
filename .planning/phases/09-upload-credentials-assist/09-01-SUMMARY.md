---
phase: 09-upload-credentials-assist
plan: 01
subsystem: ui
tags: [bubbletea, design-contract, frozen-copy, gate]

requires:
  - phase: 08-health-fixer
    provides: gate-copy-freeze convention, design.go frozen-copy pattern
provides:
  - Byte-exact Go constants in internal/tuikit/design.go for every Upload/RotateDeleteOffer copy string in 09-UI-SPEC.md's Copywriting Contract
  - Makefile gate-copy-freeze extended with the Phase 9 frozen-string list and a fifth D-07 exclusion check
  - D-08 FIELDS.md amendments (create-flow, identity-manager) preceding any Phase 9 render code
  - The `u` key allocation claimed once in 02-UX-DIRECTION.md §2 and mirrored in internal/dummytui/doc.go
affects: [09-02, 09-04, 09-06]

actuals:
  tokens: ~9500
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "One string, four places that must agree: 09-UI-SPEC.md Copywriting Contract row -> design.go constant -> gate-copy-freeze grep entry -> upload_copy_test.go byte-exact assertion"

key-files:
  created: []
  modified:
    - internal/tuikit/design.go
    - internal/tuikit/upload_copy_test.go
    - Makefile
    - .planning/design/create-flow/FIELDS.md
    - .planning/design/identity-manager/FIELDS.md
    - .planning/design/APPROVAL.md
    - .planning/design/02-UX-DIRECTION.md
    - internal/dummytui/doc.go

key-decisions:
  - "gate-copy-freeze's grep roots already cover internal/tuikit (where design.go declares the new constants), so no root change was needed."
  - "UploadKeyTitleFmt (per-machine hostname format, D-07) is deliberately excluded from the frozen list, matching the four existing per-machine/dynamic-text exclusion precedents (D-13 OpenSSH prefix, 07-03 git-version prefix, D-09 bundle aggregate, applied/selected counts) -- a fifth runtime-assembled exclusion check proves it stays out."

patterns-established:
  - "Negative-control demonstration for a source-presence grep gate must break BOTH the source constant and its test-file literal copy simultaneously -- the test file's own literal string (asserted by TestFrozenUploadCopy) lives under the same scanned root and independently satisfies the grep, so a design.go-only edit does not reproduce a MISSING failure."

requirements-completed: [UP-01, UP-03]

coverage:
  - id: D1
    description: "Every Copywriting Contract string 09-UI-SPEC.md drafted exists as a byte-exact Go constant in internal/tuikit/design.go, protected by make gate-copy-freeze."
    requirement: UP-01
    verification:
      - kind: unit
        ref: "internal/tuikit/upload_copy_test.go#TestFrozenUploadCopy"
        status: pass
      - kind: other
        ref: "make gate-copy-freeze"
        status: pass
    human_judgment: false
  - id: D2
    description: "gate-copy-freeze fails by name when a Phase 9 frozen string is deleted, and the comment block states it is a secondary source-presence guard (R21)."
    verification:
      - kind: other
        ref: "manual negative-control run (design.go + upload_copy_test.go both edited, MISSING reproduced, both restored) -- see commit f2406cf"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-08 FIELDS.md amendments (auto-upload checkbox, upload-announcing/-results states, register-key-modal state, action_register_key row) land before any Phase 9 render code."
    requirement: UP-01
    verification:
      - kind: manual_procedural
        ref: ".planning/design/create-flow/FIELDS.md and .planning/design/identity-manager/FIELDS.md diffs in aee0e1f"
        status: pass
    human_judgment: false
  - id: D4
    description: "The `u` key is claimed exactly once in 02-UX-DIRECTION.md's key-allocation table and mirrored in internal/dummytui/doc.go."
    requirement: UP-03
    verification:
      - kind: manual_procedural
        ref: ".planning/design/02-UX-DIRECTION.md and internal/dummytui/doc.go diffs in aee0e1f"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-08-28
status: complete
---

# Phase 9 Plan 01: Design-Contract Amendments and Frozen Copy Summary

**Every Upload/RotateDeleteOffer string from 09-UI-SPEC.md's Copywriting Contract now exists as a byte-exact, gate-protected Go constant, with the D-08 FIELDS.md amendments and the `u` key claim landed ahead of any Phase 9 render or backend code.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- 25 new frozen copy strings added to `internal/tuikit/design.go` (checkbox labels, announce/result rows, scope-remediation messages, the rotate delete-offer ceremony, the register-key modal) with byte-exact assertions in `internal/tuikit/upload_copy_test.go`.
- `Makefile`'s `gate-copy-freeze` target extended to register all 25 strings plus a fifth D-07 exclusion check (the per-machine key-title format must never be frozen).
- D-08 FIELDS.md amendments landed in both `create-flow` and `identity-manager` design docs, plus a Phase 9 amendment addendum in `APPROVAL.md`.
- The `u` intra-step key claimed once in `02-UX-DIRECTION.md` §2's key-allocation table and mirrored in `internal/dummytui/doc.go`.

## Task Commits

1. **Task 1: D-08 FIELDS.md amendments + APPROVAL addendum + u key claim** - `aee0e1f` (docs)
2. **Task 2: Frozen Upload/RotateDeleteOffer copy block in design.go** - `4b3cd81` (feat)
3. **Task 3: Register every new frozen string in Makefile gate-copy-freeze** - `f2406cf` (feat)

## Files Created/Modified
- `internal/tuikit/design.go` - 25 new frozen copy constants (Upload*, RotateDeleteOffer*, IdentityManagerActionRegisterKey, RegisterKeyModalHeadingFmt)
- `internal/tuikit/upload_copy_test.go` - `TestFrozenUploadCopy`, byte-exact assertions for every new constant
- `Makefile` - `gate-copy-freeze` target extended with the Phase 9 string list + D-07 exclusion check
- `.planning/design/create-flow/FIELDS.md` - amended `ssh-form-filled`/`-blank-prefix` + two new states
- `.planning/design/identity-manager/FIELDS.md` - amended `action-menu` + new `register-key-modal` state
- `.planning/design/APPROVAL.md` - Phase 9 amendment addendum
- `.planning/design/02-UX-DIRECTION.md` - `u` key allocation row
- `internal/dummytui/doc.go` - `u` key mirror

## Decisions Made
None beyond what the plan specified — followed the plan as written. See `key-decisions` in frontmatter for the two decisions the plan itself called for (grep-root reuse, D-07 exclusion precedent).

## Deviations from Plan

### Auto-fixed Issues

**1. [Hand-recovery] Executor process stopped before writing this SUMMARY**
- **Found during:** Task 3 verification (the negative-control demonstration required by the plan's own acceptance criteria, line 412)
- **Issue:** The dispatched cross-AI executor (opencode) was blocked by a sandbox permission rejection when attempting to write a backup file to `/tmp` during the negative-control demonstration, and exited without completing verification or writing `09-01-SUMMARY.md`. Tasks 1 and 2 were already committed cleanly (`aee0e1f`, `4b3cd81`); Task 3's Makefile changes were complete but uncommitted.
- **Fix:** The orchestrating session took over: committed Task 3's Makefile changes (`f2406cf`), then completed the negative-control demonstration inside the worktree (no `/tmp` access needed). Discovered the demonstration as first attempted (editing only `design.go`) does not reproduce a `MISSING` failure, because `gate-copy-freeze`'s grep roots scan the whole `internal/tuikit` directory, and `upload_copy_test.go`'s own literal copy of each string (used by `TestFrozenUploadCopy`'s byte-exact assertions) independently satisfies the grep. Re-ran the demonstration editing BOTH files' occurrence of `UploadDryRunNote`'s value simultaneously — this correctly reproduced `MISSING --dry-run: the command(s) above were shown, not run.` with a non-zero exit — then restored both files exactly and re-confirmed `make gate-copy-freeze` exits 0.
- **Files modified:** None beyond the already-planned `Makefile` change; the negative-control edits to `design.go`/`upload_copy_test.go` were temporary and fully reverted.
- **Verification:** `make gate-copy-freeze` exits 0 clean; `go build ./...`, `go vet -tags e2e ./...`, `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` (all packages pass), and `make lint` (0 issues) all re-run independently after recovery.
- **Committed in:** `f2406cf` (Task 3 commit; the negative-control run itself left no diff to commit, as documented in that commit's message)

---

**Total deviations:** 1 auto-fixed (executor hand-recovery after a sandbox permission block)
**Impact on plan:** No scope change. The hand-recovery also surfaced and documented a genuine nuance in the plan's own acceptance criterion (line 412) — the criterion's "delete one constant" framing implicitly assumed the string appears nowhere else under the scanned roots, which is false for any string `TestFrozenUploadCopy` also asserts. The gate still functions exactly as the plan's own comment block (R21) already documents: a SECONDARY source-presence guard, not authoritative — `TestFrozenUploadCopy` remains the byte-exact contract. No plan or code change was needed to correct this; only the demonstration methodology needed adjusting.

## Issues Encountered
None beyond the executor hand-recovery documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Every design-contract prerequisite Wave 2 (the tracer) needs is in place: frozen copy constants exist and are gate-protected, FIELDS.md carries the D-08 amendments, and the `u` key is claimed and available for Wave 2's step-0 checkbox toggle.
- No blockers or concerns for Wave 2.

---
*Phase: 09-upload-credentials-assist*
*Completed: 2026-08-28*
