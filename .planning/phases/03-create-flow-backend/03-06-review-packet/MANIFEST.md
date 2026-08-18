# 03-06 Task 3 — Cross-AI Visual-Regression Review Packet

Assembled by the Task-1/2 executor for the ORCHESTRATOR to consume at wave
close (DLV-04.2/D-25). The executor has no subagent-spawning tools and did
NOT run the review itself — see 03-06-SUMMARY.md "Cross-AI visual-regression
review" for the placeholder section the orchestrator fills in.

## Contents

- `text-diffs.txt` — the FULL, unfiltered output of
  `make gate-visual-regression` (`TestGateVisualRegression`,
  `cmd/gitid/gate_visual_regression_test.go`), captured with a temporary
  local instrumentation change that also logs the DUMMY/REAL text for every
  ALLOWLISTED screen (not just the unallowlisted-failure path the committed
  test normally prints). That temporary change was reverted before
  committing Task 2 — `git diff` against commit `4a9c939` shows
  `cmd/gitid/gate_visual_regression_test.go` unchanged; this file is a
  point-in-time capture, not a build artifact. Contains raw ANSI SGR escape
  codes (the actual styled terminal output, e.g. the yellow `!` warning and
  the accent-blue header nav) — the SAME text a human would see running
  either binary in a real terminal.
- This MANIFEST.md — the expected-divergence list below.

## PNG pairs — NOT included, and why

Task 2's `internal/screenshot/createflow.go` capture mechanism is
TEXT-ONLY (in-process `tuikit.App.Update()`/`.View()` driving, no PTY, no
`freeze`/PNG rendering) — see 03-06-SUMMARY.md "Task 2" for the Claude's-
Discretion rationale (geometry parity with `screenshot-tui` matters for
text wrapping; vendored font/theme only affects PNG pixels, not text
content, so the byte-exact TEXT diff does not need them). There is
therefore no PNG pair for the reviewer to look at from this plan's own
output.

If the orchestrator's review needs PNGs: `internal/screenshot.CaptureTUI`
(the existing `screenshot-tui` mechanism) can render any of the 8 golden
strings `CaptureCreateFlowScreens` returns to a deterministic PNG via
`freeze`, at the vendored `$(SCREENSHOT_FONT)`/`$(SCREENSHOT_THEME)` — it
was not wired into a `make` target this session because the plan's byte-
exact TEXT gate does not need it, and the REFERENCE-INDEX.md era's static
per-screen PNG set for create-flow was already removed as stale before
this phase (`.planning/design/REFERENCE-INDEX.md` "Removed as stale") —
there is no pre-existing PNG golden to pair against; any PNG the
orchestrator wants would be freshly rendered from these SAME text
captures, not diffed against an old static file.

## Expected divergences (manifest)

Cross-reference against `.planning/design/create-flow/visual-divergence-allowlist.txt`
(the machine-readable source of truth `TestGateVisualRegression` actually
parses) — this is the human-readable summary for the reviewers.

1. **D-02 — `ReachableNotUploaded` warning state** (screens
   `test-stage1-direct`, `test-stage2-by-alias`): a third connectivity-test
   outcome (yellow `!`, never red) the approved Phase-2 demo never had.
   Confirm it reads as intended: glyph + word (never color alone), never
   confused with the red hard-`Failure` treatment.
2. **D-16 — "Preview — demo data" banner** (no screen in THIS gate's
   captured set — create-flow is the ONE tab Phase 3 wires, so
   `Backend.DemoBanner(TabIdentities)` is `false` for both binaries;
   recorded in the allowlist for grep-completeness only). Nothing for the
   reviewers to check on THESE 8 screens; a later phase's own gate
   (git-screen/identity-manager/etc.) is where D-16 actually fires.
3. **D-19 — Git-form `[ Continue ]` disabled reason** (screen
   `git-form-demo`): the real binary's own scoped-divergence reason
   ("— Git configuration arrives with the next build") replaces the
   dummy's validity-gated one ("— needs user.name + a valid email").
   Confirm it reads as an honest capability statement, not a bug.
4. **T-03-HOSTBLOCK (carried, NOT one of the original three) —
   HostBlockPreview format** (screens `ssh-form-filled`,
   `reuse-key-vs-generate`, `reuse-manual-path`, `mouse-focused-field`,
   `confirm-write`): the real binary renders the live Host-block preview
   through the ACTUAL `sshconfig.RenderHostBlock` (2-space indent + a
   trailing `# gitid: provider=<p>` marker comment) — matching exactly
   what a confirmed write persists (SSHUI-03) — while the dummy's
   `FixtureBackend.HostBlockPreview` is an unrelated 4-space markerless
   literal that predates this contract. First flagged by the 03-03
   executor, re-flagged by 03-04 — this plan's allowlist entry is where
   the carried obligation is finally discharged (not a code change; the
   dummy fixture is frozen Phase-2 surface, out of this task's scope).
   Confirm this reads as a legitimate "the real thing is more correct
   than the demo" divergence, not an accidental visual regression.

## No undocumented divergence — self-check (executor's own pass)

Every one of the 8 captured screens differed from its dummy counterpart in
this run (see `text-diffs.txt`); every difference maps to exactly one of
the four categories above (D-02 x2, D-19 x1, T-03-HOSTBLOCK x5) — the
`TestGateVisualRegression` fail-path was hand-verified by removing the
`git-form-demo` allowlist entry and observing the gate turn red (then
reverting), proving the allowlist mechanism is not vacuous. The orchestrator
should still independently confirm no OTHER, unnoticed divergence is hiding
inside one of the four broad categories above (e.g. the reuse-picker screens
carry BOTH the HostBlockPreview divergence AND are a brand-new render
surface with no frozen dummy baseline at all — see
`03-04-SUMMARY.md` "Next Phase Readiness").
