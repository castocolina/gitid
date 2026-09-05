---
phase: 10-linux-validation-release-pipeline
plan: 06
subsystem: docs
tags: [readme, install-docs, goreleaser, checksums, homebrew, d-16]

# Dependency graph
requires:
  - phase: 10-03
    provides: "PLATFORM-NOTES.md — the per-distro portability ledger this README links to"
  - phase: 10-05
    provides: "scripts/install.sh rewritten for the D-07 tar.gz archive contract (GITID_VERSION pin, GITID_INSTALL_DIR override) — the real behavior this README documents"
provides:
  - "README.md Install section refreshed to document the real Plan 10-04/10-05 goreleaser tar.gz-archive install story (manual/inspect-first, curl|sh one-liner, Homebrew tap, go install with its version-fallback caveat) and D-15's verbatim per-OS checksum-verify commands"
affects: []

actuals:
  tokens: 1136
  tasks: 1
  commits: 1

tech-stack:
  added: []
  patterns:
    - "README Install section ordering: manual/inspect-first path always precedes the curl|sh one-liner (D-14's security-conscious ordering), carried forward from install.sh's own header-comment convention"

key-files:
  created: []
  modified:
    - README.md

key-decisions:
  - "Invoked the crafting-effective-readmes skill per D-16's explicit instruction rather than hand-writing the section — the skill returned process guidance (task-type classification: 'Updating existing content'; project-type classification: OSS; section checklist; style guide) rather than performing the edit itself, so this executor followed that returned process (read current README, identify stale Phase-9.3-era raw-binary install shape, propose+apply specific edits) to produce the refreshed content, per the skill's own documented workflow."
  - "Added a new Platforms table (not explicitly required by the plan's verify grep, but the plan's must_haves truth requires documenting linux-arm64 as best-effort) rather than folding platform info as prose into the Install section — table format matches the skill's own style-guide preference for tables over prose walls."
  - "Kept the existing 'What gitid manages (objective)', 'Quick Start', and 'Status' sections untouched per the plan's explicit scope boundary — only the Install section (and the new Platforms section immediately above it) were rewritten."

patterns-established:
  - "Verify-before-extract checksum commands are shown twice (once in the manual-install numbered steps, once in the automated-script prose) so a reader following either path sees the exact command without cross-referencing"

requirements-completed: [BUILD-03, PLAT-03]

coverage:
  - id: D1
    description: "README.md documents every real install path (install.sh, Homebrew tap, manual ~/.local/bin, go install with its version-fallback caveat), leads with the manual/inspect-first path per D-14's security framing, and shows the exact D-15 per-OS checksum-verify commands"
    requirement: "BUILD-03"
    verification:
      - kind: other
        ref: "grep -c 'install.sh\\|homebrew-tap\\|GITID_VERSION\\|GITID_INSTALL_DIR\\|sha256sum --ignore-missing\\|shasum -a 256 --ignore-missing\\|PLATFORM-NOTES.md' README.md -> 10 (all 7 individual tokens independently confirmed non-zero: install.sh=3, homebrew-tap=1, GITID_VERSION=2, GITID_INSTALL_DIR=2, sha256sum --ignore-missing=1, shasum -a 256 --ignore-missing=1, PLATFORM-NOTES.md=1)"
        status: pass
      - kind: other
        ref: "grep -n 'Manual install|curl | sh one-liner' README.md -> Manual install at line 47, curl|sh one-liner at line 76 (manual precedes automated, D-14 ordering confirmed)"
        status: pass
    human_judgment: false
  - id: D2
    description: "README.md links to PLATFORM-NOTES.md so a Linux user can see gitid's current portability status before installing"
    requirement: "PLAT-03"
    verification:
      - kind: other
        ref: "grep -c 'PLATFORM-NOTES.md' README.md -> 1 (Platforms section, linked before the Install section)"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-09-05
status: complete
---

# Phase 10 Plan 06: README.md refresh via the README-crafting skill (D-16) Summary

**Rewrote README.md's Install section through the crafting-effective-readmes skill, replacing the stale Phase-9.3 raw-binary install story with the real goreleaser tar.gz-archive pipeline (manual/inspect-first path, curl|sh one-liner, Homebrew tap, go install caveat) and D-15's verbatim checksum-verify commands, closing Phase 10.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-05T01:34:07Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- Invoked the `crafting-effective-readmes` skill (D-16's explicit requirement — not hand-written outside the skill) to drive the README refresh. The skill returned its process guidance (task type = "Updating existing content"; project type = OSS; a section checklist and style guide), which this executor then followed to read the current README, identify the stale Phase-9.3 raw-binary install shape, and apply the refresh.
- Rewrote the Install section with four real, verified install paths in D-14's security-conscious order: (1) manual download+inspect+verify+extract walkthrough first, (2) the `curl -fsSL .../install.sh | sh` one-liner (documenting the real `GITID_VERSION` pin and `GITID_INSTALL_DIR` override env vars scripts/install.sh actually supports per Plan 10-05), (3) the Homebrew tap (`brew install castocolina/homebrew-tap/gitid`, matching `.goreleaser.yaml`'s real `brews:` stanza from Plan 10-04), and (4) `go install .../gitid@latest` with its version-fallback caveat (falls back to `runtime/debug.ReadBuildInfo()` instead of ldflags stamping).
- Documented D-15's exact per-OS checksum-verify commands verbatim: Linux `sha256sum --ignore-missing -c gitid_<v>_checksums.txt`, macOS `shasum -a 256 --ignore-missing -c gitid_<v>_checksums.txt`.
- Added a new Platforms section (table: darwin/linux × amd64/arm64, linux/arm64 flagged best-effort/non-CI-gated) linking `PLATFORM-NOTES.md` immediately above Install, so a Linux user sees gitid's current per-distro validation status before installing.
- Left "What gitid manages (objective)", "Quick Start", and "Status" untouched per the plan's scope boundary.

## Task Commits

Each task was committed atomically:

1. **Task 1: README.md refresh via the README-crafting skill (D-16)** - `75f6131` (docs)

**Plan metadata:** not committed by this executor — per this plan's `<parallel_execution>` instructions, the orchestrator owns STATE.md/ROADMAP.md updates after this wave's worktree merges back; this SUMMARY.md is committed separately by this executor per the `<objective>`'s instruction to commit SUMMARY.md itself.

_Note: this is a non-TDD `type="execute"` plan with a single task — one commit._

## Files Created/Modified
- `README.md` - Install section fully rewritten (manual-first ordering, curl|sh one-liner with env vars, Homebrew tap, go install caveat, D-15 verbatim checksum commands); new Platforms section added linking PLATFORM-NOTES.md; all other sections preserved verbatim.

## Decisions Made
See `key-decisions` in the frontmatter above: (1) the skill's own process-guidance response was followed as the mechanism for producing the content, satisfying D-16's "invoke the skill, don't hand-write outside it" instruction; (2) a new Platforms table was added beyond the plan's literal grep tokens because the plan's own `must_haves.truths` requires the linux-arm64 best-effort status to be documented; (3) sections outside Install were left untouched per scope.

## Deviations from Plan

**1. [Rule 1 - Bug] Cleared a stale cross-worktree golangci-lint cache before the commit**
- **Found during:** Task 1, first commit attempt
- **Issue:** The pre-commit hook's `make lint` (tagged-build variant, `lint-tagged`) surfaced 11 gosec/revive findings pointing at file paths under a DIFFERENT sibling worktree (`agent-a8ec8d58e518b0336/internal/screenshot/...`) that does not exist in this worktree — the same class of cross-worktree golangci-lint result-cache artifact recorded in `10-05-SUMMARY.md`'s "Deviations from Plan".
- **Fix:** Ran `/Users/ramon/go/bin/golangci-lint cache clean`, then re-ran the commit.
- **Files modified:** none (tool-cache artifact only, no source change — this plan touches only README.md, a Markdown file the tagged-build Go lint pass shouldn't even reference).
- **Verification:** The re-run pre-commit hook reported `Go lint (golangci-lint + gosec) ... Passed`; the commit succeeded on the second attempt.
- **Committed in:** n/a (pre-commit tooling fix, not a source change; the commit itself is `75f6131`)

---

**Total deviations:** 1 (a pre-commit tooling-cache fix, no code/doc change beyond what the plan specified).
**Impact on plan:** None on scope — README.md content is exactly what the plan's `<action>` and `<verify>` specify; the deviation only unblocked the commit's pre-commit hook.

## Issues Encountered

The `crafting-effective-readmes` skill is a process-guidance skill (task-type/project-type classification + checklists), not an autonomous content-generation subagent — invoking it returned instructions for HOW to update a README rather than directly producing the new README.md content. This executor followed those returned instructions (task = "Updating existing content": read current README, identify stale sections, propose+apply specific edits; project type = OSS per the section-checklist) to author the refreshed Install section, which still satisfies D-16's "invoke the skill, not hand-write outside it" instruction — the skill's process was used, its guidance followed, and no independent hand-rolled README-writing approach was substituted.

## User Setup Required

None - no external service configuration required. This plan is docs-only.

## Next Phase Readiness

- Phase 10 (linux-validation-release-pipeline) is now fully closed: PLATFORM-NOTES.md (10-03), the Fedora CI job (10-02), the version-stamping package (10-01), the goreleaser release pipeline (10-04), the migrated install.sh + e2e suite (10-05), and this README refresh (10-06) are all in place.
- README.md's install documentation matches the ACTUAL install.sh/`.goreleaser.yaml` behavior verified by reading those files directly in this plan — no aspirational or Phase-9.3-carryover content remains in the Install section.
- No blockers. The orchestrator's wave-close gates (go build, go test -race, make lint, make test-e2e) are unaffected by this docs-only change.

---
*Phase: 10-linux-validation-release-pipeline*
*Completed: 2026-09-05*

## Self-Check: PASSED

- FOUND: README.md (path exists, content verified via grep tokens above)
- FOUND: commit 75f6131
