# gitid — Milestones

## 0.0.1 — POC "Product Features in TUI" (archived, never released)

**Status:** Archived 2026-07-02. Never released.
**Archive:** `.planning/archive/0.0.1-poc-product-features-in-tui/`
(requirements, roadmap, and all 10 phase directories 01–06 / 05.5 / 05.6 / 05.7).

The prior build cycle. It delivered the working core mechanics — safe writes
(backup + atomic + idempotent managed blocks), `ssh -G` prove-before-write, the
doctor engine, temp-config SSH testing, identity CRUD, and an integrated Bubble Tea
TUI — but the create/manage UX accreted debt and, mid-cycle, we discovered a better
way to build (design-first, screenshot-verified) and a clearer set of goals.

Because nothing shipped, this cycle is reframed as a **0.0.1 POC**. Its code is kept
as **reusable substrate** (not a behavior contract) for the v1.0 redesign.

## v1.0 — TUI-First Redesign (active)

**Status:** Started 2026-07-02. Defining roadmap.
**Requirements:** `.planning/REQUIREMENTS.md` (sections A–P).
**PRD:** `docs/prds/gitid-tui-redesign-v1.0-prd.md`.

The real first release: a design-driven, screenshot-verified terminal app. See
PROJECT.md → Current Milestone for the feature set and the loop at
`.planning/ONESHOT-LOOP-PROMPT.md`.
