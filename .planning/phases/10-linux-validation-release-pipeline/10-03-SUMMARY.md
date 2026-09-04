---
phase: 10-linux-validation-release-pipeline
plan: 03
subsystem: docs
tags: [platform-notes, linux-validation, bazzite, fedora, uat, portability]

# Dependency graph
requires: []
provides:
  - PLATFORM-NOTES.md — root-level D-04 portability ledger with 7 sourced rows (2 Fedora, 5 Bazzite)
  - bazzite-uat-checklist.md — D-03 runnable manual UAT runbook for the 5 container-invisible risk items
affects: [10-linux-validation-release-pipeline, README-refresh]

# Actuals (#2632)
actuals:
  tokens: 1874
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Portability ledger + runbook pairing: bazzite-uat-checklist.md drives the manual check, PLATFORM-NOTES.md records the outcome (D-04's routing contract)"

key-files:
  created:
    - PLATFORM-NOTES.md
    - .planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md
  modified: []

key-decisions:
  - "Left README.md untouched — this plan's files_modified/verify scope covers only PLATFORM-NOTES.md and bazzite-uat-checklist.md; the D-04 README link is Plan 10-06's full README-refresh job (D-16), not this plan's"
  - "The five Bazzite rows are logged as 'Pending manual UAT' rather than fabricated Verified/Accepted rows — no real Bazzite hardware is available in this autonomous execution; pending-and-mechanism-provided is itself a PLAT-03-compliant accepted-limitation state per the plan's objective"

patterns-established:
  - "PLATFORM-NOTES.md is the single root-level ledger every future Bazzite UAT pass updates in place (not append-only) — status transitions from Pending manual UAT to Verified or Accepted limitation"

requirements-completed: [PLAT-03]

coverage:
  - id: D1
    description: "PLATFORM-NOTES.md exists at the repo root with the D-04 five-column table and all 7 sourced rows (2 Verified, 5 Pending manual UAT)"
    requirement: "PLAT-03"
    verification:
      - kind: other
        ref: "test -f PLATFORM-NOTES.md && grep -c '^| Fedora\\|^| Bazzite' PLATFORM-NOTES.md -> 7"
        status: pass
    human_judgment: false
  - id: D2
    description: "bazzite-uat-checklist.md exists with all 5 D-03 UAT-only risk items, each with a concrete command, PASS criterion, and routing instruction back into PLATFORM-NOTES.md"
    requirement: "PLAT-03"
    verification:
      - kind: other
        ref: "test -f .planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md && grep -c '^## \\|^### ' ... -> 6 (>= 5 required)"
        status: pass
    human_judgment: false

# Metrics
duration: ~15min
completed: 2026-09-04
status: complete
---

# Phase 10 Plan 03: Linux Portability Ledger + Bazzite UAT Checklist Summary

**D-04's PLATFORM-NOTES.md ledger and D-03's runnable Bazzite UAT checklist, sourced from RESEARCH's real, verified findings rather than blank templates**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-04
- **Completed:** 2026-09-04
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Created `PLATFORM-NOTES.md` at the repo root: the D-04 five-column ledger (Distro | Aspect | Status | Workaround | Issue) with 7 real, sourced rows — 2 Fedora rows marked Verified (container-confirmed `ssh -V` version-string parsing with no distro suffix, and the exact `dnf install` prerequisite list), and 5 Bazzite rows marked Pending manual UAT (wl-clipboard presence, ssh-add graceful degradation, SELinux spot-check, `/home`→`/var/home` symlink includeIf resolution, real-terminal TUI rendering).
- Created `.planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md`: a runbook with one section per D-03 UAT-only risk item, each carrying an exact command, a concrete PASS criterion, an explicit FAIL description, and a closing instruction routing findings back into PLATFORM-NOTES.md with a redaction reminder (no real hostnames/usernames/local paths in committed findings).
- Both artifacts close PLAT-03's "portability gaps are fixed or logged as accepted limitations" clause: 2 rows closed by the container CI job's own verified findings, 5 rows explicitly pending with a concrete, runnable mechanism to close them.

## Task Commits

Each task was committed atomically:

1. **Task 1: PLATFORM-NOTES.md — the per-distro portability ledger (D-04)** - `4630395` (docs)
2. **Task 2: bazzite-uat-checklist.md — the runnable manual UAT runbook (D-03)** - `fda380a` (docs)

**Plan metadata:** committed via the orchestrator's final wave-close commit (docs); this executor is a parallel worktree agent and does not run the final metadata commit itself — see note below.

## Files Created/Modified
- `PLATFORM-NOTES.md` - D-04 root-level portability ledger, 7 sourced rows + "How this ledger is updated" section
- `.planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md` - D-03 Bazzite manual UAT runbook, 5 risk-item sections + routing instruction

## Decisions Made
- Did not touch README.md. This plan's `files_modified` frontmatter and `<verification>` block scope to exactly the two new files; the D-04 "linked from README" requirement is explicitly Plan 10-06's job (D-16's full README refresh via the README-crafting skill), not a task action in this plan.
- Logged all five Bazzite rows as "Pending manual UAT" (not fabricated Verified/Accepted entries) — no real Bazzite hardware exists in this autonomous execution context; this is the plan's own stated legitimate PLAT-03-compliant end state until the human runs the checklist.

## Deviations from Plan

None - plan executed exactly as written. Both tasks' `<action>` content was followed verbatim (row content, column set, section list, command/PASS-criterion pairs), and both `<verify>` automated checks pass exactly as specified.

## Issues Encountered

None. This plan is doc-only (no Go code changes), so `make fmt`/`make lint` ran over the whole module unchanged by these two new Markdown files and passed on both commits.

## User Setup Required

None - no external service configuration required. The actual Bazzite UAT run itself is a recurring, per-release **human** action (running `bazzite-uat-checklist.md` on real hardware) — that is the intended, documented mechanism this plan ships, not a gap in this plan's own completion.

## Next Phase Readiness

- PLATFORM-NOTES.md and bazzite-uat-checklist.md are both in place and cross-linked (checklist → ledger routing, ledger → checklist reference) for any future plan or release cycle to consume.
- Plan 10-06 (README refresh, D-16) still owes the PLATFORM-NOTES.md link from README.md — flagging so it isn't missed, since this plan intentionally left README.md untouched.
- No blockers for sibling wave-1 plans (10-01, 10-02) — this plan touched only its own two new, isolated files, no shared file overlap.

---
*Phase: 10-linux-validation-release-pipeline*
*Completed: 2026-09-04*

## Self-Check: PASSED

- FOUND: PLATFORM-NOTES.md
- FOUND: .planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md
- FOUND: commit 4630395
- FOUND: commit fda380a
