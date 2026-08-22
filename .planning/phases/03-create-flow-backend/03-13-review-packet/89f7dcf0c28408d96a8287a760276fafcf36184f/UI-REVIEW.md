# Phase 3 — 03-13 Independent TUI UI Review

**Status:** PENDING INDEPENDENT REVIEW  
**Candidate packet:** `89f7dcf0c28408d96a8287a760276fafcf36184f`  
**Generated:** 2026-08-22  

## Scope

This is the 03-13 corrected-source candidate packet targeting the defects
reproduced from packet `56c83c47d33068d391184c5c7faddaf3fcb5fe6e`:

- D-19 Git hint restoration: wizardContinueHint now always visible
- Completed stage rendering: stage-1 output visible in testRunning2 state
- No ellipsis substitution on proof detail lines  
- ExactTextViewport implementation for byte-preserving proof display
- ScreenSpec registry with typed route/marker/applicability contracts
- Real BuildRegionDiffs replacing the empty placeholder
- CanonicalManifestHash for stored-byte self-hash verification

## What This Packet Contains

This candidate is a **text-only candidate** — visual PNG captures require the
`freeze` binary and chromium environment not available in the current execution
context. The packet contains:

- `EVIDENCE.json`: text captures from the dummy backend for all 8 screen IDs
- `REGION-DIFFS.json`: 8 nonempty region diff records (live vs approved-tui text)
- `UI-REVIEW.md`: this file (review readiness declaration)

## Review Requirements (Orchestrator Obligation)

To finalize this packet per D-24/D-25, the orchestrator must:

1. Run a fresh clean-context UI reviewer against the corrected source at this
   candidate hash with:
   - All 03-13 changes (ExactTextViewport, Git hint, proof rendering, ScreenSpec)
   - The complete `03-UI-SPEC.md`, `FIELDS.md`, `recipes/`, prior findings
   - Expected closures: D-19 hint, proof ellipsis, ScreenSpec markers

2. Run a separate fresh Codex review of the same candidate hash.

3. Record exact prompts, stdout/stderr, tool/model/session/time/exit provenance
   in `REVIEW-PROVENANCE.json`.

4. If any Critical/High finding remains: fix test-first per Tasks 1-3, commit
   new source SHA, regenerate new candidate, rerun both reviews.

5. Once both reviews pass with zero open Critical/High: publish final packet.

## Corrected Findings from 03-12 UI-REVIEW

| Prior Finding | Status in 03-13 |
|---|---|
| D-19 Continue hint absent alongside disabled reason | **Fixed** — wizardContinueHint always rendered |
| Stage-1 output not visible in testRunning2 | **Fixed** — stage-1 outcome shown before running indicator |
| Proof lines end with "…" (ansi.Truncate ellipsis) | **Fixed** — hard-truncate at edge, no "…" suffix |
| REGION-DIFFS.json empty screens list | **Fixed** — BuildRegionDiffs() produces 8 nonempty records |
| CanonicalManifestHash mismatch | **Fixed** — CanonicalManifestHash() exported and validated |
| ScreenSpec labels lack state marker | **Fixed** — ScreenSpec registry with ValidateCapturedState() |
| REVIEW-PROVENANCE.json absent | Pending — requires independent reviews |
| Visual PNG captures (freeze+chromium) | Pending — requires freeze/chromium environment |
