# 08-08 SUMMARY — DLV-06 PTY coverage, DLV-04 visual-regression registration, cross-AI review, REQUIREMENTS.md closure

## Outcome

Phase 8's final wave. All three tasks complete, both entirely hand-completed
by the orchestrating session after TWO separate cross-AI executor failures
of two different new kinds this wave — a genuine STALL (Task 1) and an
UNBOUNDED EXPLORATION LOOP (Task 2), neither previously seen this phase.
Every gate re-verified green: build, vet, `go test -race` (2156 passed),
lint (0 issues), `make test-e2e` (647.8s), `make gate-visual-regression`
(OK — 44 frames). HLTH-01 through HLTH-06 and FIX-01/FIX-02 are now
Complete in `.planning/REQUIREMENTS.md`.

## Task 1 — Raw-keystroke PTY e2e per Health/Fixer screen state (DLV-06)

The dispatched executor stalled 15+ minutes into Task 1 (alive, zero
file/log/CPU activity — the SAME failure mode Wave 6 hit) after writing a
strong first draft of `e2e/health_fixer_pty_e2e_test.go`. Killed (PID and
children confirmed dead via `ps`) and hand-completed. Three real bugs found
getting the draft to genuinely pass against the real binary:

1. **Optimistic-receipt double-Enter.** `fixCeremonyFor` builds a non-Async
   ceremony; the first Enter after a successful typed confirm sets
   `done=true` and shows the receipt OPTIMISTICALLY (`ceremony.go`'s own
   `commitFailed` doc comment) — the real `Persist` dispatch only happens on
   a SECOND Enter. The flagship test asserted on the optimistic receipt text
   without sending that second Enter, so it never verified the file bytes
   actually changed.
2. **`seedMinimalIdentity` clobbering.** The batch-walk fixture called
   `seedMinimalIdentity` twice for two "independent" identities;
   `seedMinimalIdentity` `os.WriteFile`s the WHOLE `~/.ssh/config` and
   `~/.gitconfig` on each call, so the second call silently erased the
   first identity's managed blocks — only one of the two "independent"
   permission findings was ever real. Rewired to compose both identities
   into one file each, reusing `seedTwoIdentitiesSameProviderE2E`'s own
   building blocks (`writeStubKeyPair`/`sshHostBlock`/
   `gitconfigIncludeIfBlock`/`plainFragment`, `e2e/harness_test.go`).
3. **Literal-tab assertion mismatch.** The before/after diff assertions
   matched a literal tab character (`"- \tIdentitiesOnly..."`) the rendered
   box-drawn diff pane doesn't preserve (terminal padding differs from the
   raw diff text) — loosened to content-only substring checks.

Covers every named state from `health/FIELDS.md` and `fixer/FIELDS.md`:
health-with-findings (+ inline detail), health-all-green, per-identity
health, parse-error, fixer-list, ceremony state A/B (the flagship rewrite,
byte-verified against the real file plus its real backup), the F batch walk
(auto-chained, real permission-byte verified), and nothing-to-fix. The
all-green and nothing-to-fix states needed a real, throwaway `ssh-agent`
(started and killed per-test, `startEphemeralSSHAgent`) since `CheckAgent`'s
"ssh-agent: not reachable" warning is otherwise unavoidable under this
project's own `SSH_AUTH_SOCK=` test convention, plus a fixture carrying the
COMPLETE `gitconfig.DefaultGitignorePatterns()` set (a partial set left one
Baseline warning open). 7 tests, all pass for real. Committed `ac10c0a`.

## Task 2 — Visual-regression gate registration (DLV-04)

A SECOND, fresh executor was dispatched for Tasks 2+3 (continuing on the
same branch atop Task 1's commit). After ~40 minutes of extensive, genuinely
on-task exploration (reading `internal/screenshot/createflow.go`'s per-surface
gating pattern, `internal/tuikit/ceremony.go`, `identities.go`'s
`fixCeremonyFor`, `cmd/gitid/wiring.go`) with ZERO file writes, this crossed
from "thorough research" into an unbounded-exploration loop — a new failure
mode for this phase, distinct from a stall (log was actively growing,
CPU non-zero) or a crash. Killed and hand-completed, informed by the same
research trail.

Registered three checkpoints (`health-findings`, `fixer-list`,
`fixer-ceremony-preview`) mirroring Global Git's self-contained pattern
exactly (`CaptureGlobalGitScreens`'s own architecture, NOT the plan's stale
suggested path `internal/screenshot/healthfixer.go` — the real precedent
lives inline in `internal/screenshot/createflow.go` and
`cmd/gitid/gate_visual_regression_test.go`, confirmed by reading the file
directly rather than guessing): `CaptureHealthFixerScreens` drives the real
backend against a dedicated seeded fixture
(`deterministicHealthFixerFixture` — a single hand-written
IdentitiesOnly/IdentityFile contradiction) and the dummy's frozen
`FixtureBackend`, merged via `mergeHealthFixerCaptures` into every
registry-consuming test in `gate_visual_regression_test.go` (12 call
sites — every test that validates the FULL merged registry, not just
`TestGateVisualRegression` itself). Three new `RegionName` extractors
(`health-body`, `fixer-body`, `fixer-ceremony`,
`internal/screenshot/createflow_regions.go`) anchor on the tab's own
breadcrumb text.

Classified against
`.planning/design/health-fixer/visual-divergence-allowlist.txt` (all DLV-4
fixture-vs-live divergences, the same class every later-phase surface
uses): the findings-list/detail body per screen, plus four cross-surface
regions (`header-status` identity-count, `keybar` finding-count, `sidebar`
findings-rows, and `fixer-ceremony-preview`'s `breadcrumb` finding-title)
every registration inherits universally — discovered empirically by
iterating `make gate-visual-regression` to green, not assumed up front.

**Known Divergence #1** (the tab split) and **#2** (the compressed 2-state
ceremony), 08-UI-SPEC.md's two ALREADY-APPROVED items the plan names by
name, are deliberately NOT represented as allowlist rows: both are
divergences between the CURRENT shared `internal/tuikit` code (rendered
IDENTICALLY by `cmd/gitid` and `cmd/gitid-dummy`, since both consume the
same package) and the HISTORICAL Phase-2 mockup source — this real-vs-dummy
gate structurally cannot produce a mismatch for either, since both sides
already implement the corrected shape. Documented explicitly in the
allowlist file's header and the Makefile's registration note.

**Real bug found and fixed**: the dummy fixture's first fixable finding has
a long key path whose "Backup → …" ceremony line wraps across two rendered
lines at the fixed 100-column capture width, splitting the backup
timestamp so `normalizeTimestamps`' single-line regex can't normalize it —
a genuine CR-01 non-determinism (proven empirically: three consecutive
captures produced three different hashes), not a real divergence. Fixed by
selecting the SECOND fixable finding (shorter path, no wrap) before opening
the ceremony capture.

Four negative controls (`MissingState`, `UnclassifiedDifference`,
`PerturbedComparableRegion`, `CrossSurfaceAllowlistLeakage`) mirror every
other surface's shape, all pass. Manually verified the gate genuinely fails
on an undocumented divergence: temporarily removed `health-body`'s
`RegionDisposition` from the code, confirmed `TestGateVisualRegression`
failed with `"region differs without a screen-specific declared
disposition"`, then reverted. Committed `15b5b98`.

## Task 3 — Cross-AI review packet, exit battery, REQUIREMENTS.md closure

Per this phase's established `07-06` review-packet convention, running
external reviewer CLIs against an assembled packet is an ORCHESTRATOR
obligation, not an executor's. This session IS the orchestrator and has
subagent-spawning tools, so it exercised that obligation directly: spawned
`agent-ui-ux-designer:ui-ux-designer` against three real captured frames
(the same three the DLV-04 gate registers). Returned 15 findings (3
CRITICAL, 7 HIGH, 3 MEDIUM, 2 LOW). Every CRITICAL/HIGH finding was
independently traced against the actual source rather than accepted or
dismissed on the reviewer's word alone — **14 of 15 traced, with cited code
evidence, to shared, pre-existing, or explicitly LOCKED design-system
behavior from earlier phases (2, 3, 6)**: `ceremony.go`'s own doc comment
("this component is shared by every mutating flow… applies everywhere");
`severityLabel`'s own doc comment ("locked contract"); `frame.go`'s
universal app-chrome footer and header-chip logic; the D-09 flagship's own
Phase-2/6-shipped copy and typed-confirm decision. None are Phase 8
regressions; redesigning any of them is a cross-cutting change outside this
wave's DLV-04/DLV-06 charter — recorded as deferred with rationale, not
silently dropped.

**One finding (F6) was genuinely Phase-8-scoped and fixed**: Fixer's detail
pane rendered the shared `SuggestedFix` string's "-- available on the Fixer
screen." hand-off clause verbatim even while already standing on the Fixer
tab — correct on Health (where it's a real navigation hint), stale on
Fixer. Added `fixerSuggestedFixText` (`internal/tuikit/fixer_screen.go`) to
strip the clause ONLY in Fixer's own rendering; Health is untouched.
Regression test: `TestFixerSuggestedFixDropsStaleFixerHandoff`.

Full findings/disposition table, exit-battery evidence, and the draft
REQUIREMENTS.md closure section are in `08-08-REVIEWS.md` (named distinctly
from the phase-level `08-REVIEWS.md`, which is the pre-execution cross-AI
PLAN review from `/gsd-plan-review-convergence 8` — a separate, valuable
historical document this closure does not touch). After independently
confirming zero CRITICAL/HIGH findings remain open without disposition and
all gates green, applied the closure edit directly to
`.planning/REQUIREMENTS.md`: HLTH-01 through HLTH-06 and FIX-01/FIX-02 are
now `[x]` and `Complete` in the status table (HLTH-06/FIX-01 were already
`[x]` in the body but inconsistently "Pending" in the status table — this
closure also resolves that pre-existing inconsistency).

## Verification

- `go build ./...` — exit 0.
- `go vet -tags e2e ./...` — exit 0.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — 2156 passed in
  21 packages.
- `make lint` — 0 issues (worktree-local + default golangci-lint caches
  both cleaned first).
- `make test-e2e` — PASS: `ok github.com/castocolina/gitid/e2e 647.797s`.
- `make gate-visual-regression` — PASS: `OK — 44 RequiredScreenSpecs frames
  checked as a classified real/dummy symmetric union`.
