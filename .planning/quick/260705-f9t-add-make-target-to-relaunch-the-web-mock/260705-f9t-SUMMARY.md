---
phase: quick-260705-f9t
plan: 01
subsystem: infra
tags: [make, vite, mockup-src, dev-server]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    provides: .planning/design/mockup-src (Vite dev server for the web design mockup)
provides:
  - demo-web Makefile target that (re)launches the mockup-src Vite dev server on a
    dedicated port and opens it in the browser
affects: [phase-02-design-all-mockups-checkpoint-1]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Idempotent lsof-ti + kill (with kill -9 survivor fallback) before relaunch, on a
       dedicated non-standard port reserved off Vite's 5173 default"

key-files:
  created: []
  modified:
    - Makefile

key-decisions:
  - "Target name demo-web (matches screenshot-{tui,html} action-first Makefile convention)"
  - "Dedicated port 45173 (Vite default 5173, 4-prefixed; below macOS ephemeral range
     49152+) so lsof-kill-by-port can never collide with another Vite project on 5173"
  - "pnpm exec vite --port ... --strictPort (not pnpm dev) so CLI flags reach Vite
     directly with no script-arg-forwarding ambiguity; vite.config.ts left untouched"
  - "Server launched via nohup + background (&), logs redirected to
     /tmp/gitid-demo-web.log, so it survives the make invocation returning"

patterns-established:
  - "Bounded readiness-poll loop (40 x 0.25s sleep, checked via lsof) before opening
     the browser, instead of a fixed sleep"

requirements-completed: [QUICK-260705-f9t]

# Metrics
duration: ~20min
completed: 2026-07-05
---

# Quick Task 260705-f9t: Add demo-web Makefile target Summary

**Idempotent `make demo-web` target that stops any prior Vite instance on a dedicated
port (45173), relaunches the `.planning/design/mockup-src` dev server in the background,
and opens it in the browser — verified via `make -n demo-web` dry-run and structural greps.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-05T15:09:27Z
- **Tasks:** 1 of 2 (Task 2 is a blocking `checkpoint:human-verify` gate — see below)
- **Files modified:** 1 (Makefile)

## Accomplishments

- Added `DEMO_WEB_PORT` (45173), `DEMO_WEB_DIR` ($(CURDIR)-anchored), and `DEMO_WEB_LOG`
  (/tmp/gitid-demo-web.log) Make variables, documented inline with the rationale for the
  port choice (off 5173, below macOS ephemeral range).
- Added the `demo-web` `.PHONY` target: kills any previous instance on the port
  (`lsof -ti tcp:PORT` + `kill`, then `kill -9` on survivors), installs `node_modules`
  via `pnpm install --frozen-lockfile` only if missing, launches
  `pnpm exec vite --port $(DEMO_WEB_PORT) --strictPort` backgrounded via `nohup`, polls
  up to 10s (40 x 0.25s) for the port to bind, then runs `open http://localhost:45173`.
- Added `demo-web` to `.PHONY` and the file-header target list (matching the
  `screenshot-html`-style one-line doc entries).
- Verified structurally: `make -n demo-web` parses without error (dry-run only — no
  server was actually launched, no browser opened, per the plan's constraint against
  GUI side effects during automated execution).

## Task Commits

1. **Task 1: Add demo-web target + supporting variables to Makefile** - `9ecfbb4` (feat)

**Plan metadata:** Not yet committed — orchestrator handles the docs commit per this
quick task's constraints (SUMMARY.md / STATE.md are not committed by the executor).

## Files Created/Modified

- `Makefile` - Added `DEMO_WEB_PORT`/`DEMO_WEB_DIR`/`DEMO_WEB_LOG` variables and the
  `demo-web` target (kill-by-port, conditional install, backgrounded Vite launch,
  readiness poll, browser open); added `demo-web` to `.PHONY` and the header doc list.

## Decisions Made

- Followed all "Locked implementation decisions" in the plan's `<context>` verbatim
  (target name, port choice, `pnpm exec vite` over `pnpm dev`, background+log,
  readiness poll before opening, explicit `--port`/`--strictPort`).
- No changes to `vite.config.ts` — the `dev` script is bare `vite`, so CLI flags fully
  set the port, as the plan specified.

## Deviations from Plan

**1. [Self-correction during editing — no Rule applies, pre-commit] Fixed a misplaced
insertion that split the `gate-no-backend-files` doc-comment from its target**
- **Found during:** Task 1, while adding the `demo-web` recipe via `Edit`
- **Issue:** My first `Edit` call inserted the new `demo-web` doc-comment + target
  between the existing `gate-no-backend-files` doc-comment and its target line,
  and left a duplicated, truncated one-line comment behind. This was caught before
  committing, not shipped.
- **Fix:** Re-edited the file so `demo-web` (doc-comment + target) sits as its own
  complete block after the intact `gate-no-backend-files` doc-comment + target.
- **Files modified:** Makefile
- **Verification:** Re-read the file (lines 216-295) to confirm both target blocks
  are contiguous and un-duplicated, then re-ran the plan's `<verify>` command.
- **Committed in:** 9ecfbb4 (only the corrected, final state was ever staged/committed)

---

**Total deviations:** 1 self-corrected editing mistake (caught pre-commit, not a Rule 1-4
deviation from the plan's intent).
**Impact on plan:** None on the shipped behavior — the plan's locked decisions were
followed exactly; this was purely an editing-order slip fixed before staging.

## Issues Encountered

- The sandboxed interactive shell's `grep` is aliased to a `ugrep`-based wrapper that
  mishandles the literal `$(...)` sequence in the plan's `<verify>` grep pattern
  (`pnpm exec vite --port $(DEMO_WEB_PORT) --strictPort`), reporting a false non-match.
  Confirmed this is a shell-environment artifact, not a real Makefile defect, by
  re-running the identical check with `command grep` (bypassing the wrapper), which
  matched correctly. The full verification chain from the plan passes when run
  against the real `grep` binary:
  `make -n demo-web >/dev/null && grep -q '^demo-web:' Makefile && grep -q 'DEMO_WEB_PORT :=' Makefile && grep -q 'pnpm exec vite --port $(DEMO_WEB_PORT) --strictPort' Makefile && grep -Eq '^\.PHONY:.*\bdemo-web\b' Makefile && echo "structure OK"`
  → `structure OK`.

## User Setup Required

None - no external service configuration required.

## Checkpoint Status: PENDING (Task 2)

Task 2 is a `checkpoint:human-verify` gate (`gate="blocking"`) and was intentionally
**not** performed by the executor, per this quick task's constraints (no GUI side
effects — the dev server was never actually launched, no browser was opened).

**What was built:** The `make demo-web` target: it stops any prior Vite instance on
port 45173, relaunches the `.planning/design/mockup-src` dev server in the background
(logs at `/tmp/gitid-demo-web.log`), waits for the port to bind, and opens
`http://localhost:45173` in the browser.

**How to verify (user action required):**
1. Run `make demo-web`. Expected: it prints the (re)launch steps, then your default
   browser opens `http://localhost:45173` showing the gitid design mockup SPA.
2. Confirm the server survived `make` returning: run `lsof -ti tcp:45173` — it should
   print a PID. Optionally `tail /tmp/gitid-demo-web.log` to see Vite's "ready" banner.
3. Prove relaunch works: run `make demo-web` a SECOND time. Expected: it reports
   stopping the previous PID, starts a new one, and re-opens the browser — with NO
   "port already in use" / EADDRINUSE error.
4. (Optional) Confirm no other Vite project on 5173 was disturbed.
5. Cleanup when done: `kill $(lsof -ti tcp:45173)`.

**Resume signal:** Type "approved" if the browser opened on 45173 and the second run
relaunched cleanly, or describe what went wrong.

## Next Phase Readiness

- The Makefile change is committed (`9ecfbb4`) and ready for the human verification
  step. No other files were touched; `Makefile` is within the Phase 2 design-only
  `gate-no-backend-files` allowlist.
- Blocked on human verification of Task 2 before this quick task can be marked fully
  complete.

---
*Phase: quick-260705-f9t*
*Completed: 2026-07-05 (Task 1 only; Task 2 checkpoint pending)*

## Self-Check: PASSED

- FOUND: Makefile
- FOUND: .planning/quick/260705-f9t-add-make-target-to-relaunch-the-web-mock/260705-f9t-SUMMARY.md
- FOUND: 9ecfbb4 (git log --oneline --all)
