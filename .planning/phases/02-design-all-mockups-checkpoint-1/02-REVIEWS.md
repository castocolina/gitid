---
phase: 2
reviewers: [codex]
reviewed_at: 2026-07-02T21:10:04Z
plans_reviewed: [02-01-PLAN.md,02-02-PLAN.md,02-03-PLAN.md,02-04-PLAN.md,02-05-PLAN.md,02-06-PLAN.md,02-07-PLAN.md,02-08-PLAN.md,02-09-PLAN.md,02-10-PLAN.md,02-11-PLAN.md,02-12-PLAN.md]
codex_model: default (codex-cli 0.142.5)
verdict: MEDIUM-HIGH
---

# Cross-AI Plan Review — Phase 2 (DESIGN — All Mockups, ★ CHECKPOINT #1)

> Independent review by Codex CLI (codex-cli 0.142.5). Claude Code self-skipped per the review workflow's independence rule. Feed back with `/gsd-plan-phase 2 --reviews`.

## Codex Review

## Summary

The Phase 2 plan is directionally strong and mostly satisfies the stated goal: it creates a design-first pipeline with MUI mockups, a backend-free Go TUI dummy, screenshots, semantic parity review, full navigation proof, and a single human approval gate before backend work. The sequencing is sensible: foundation, dummy skeleton, capture tooling, pilot, fan-out, assembly, approval. The main risks are not conceptual but executional: the no-backend gate is too regex/package-list dependent, the manifest/e2e navigation contract is under-specified enough to produce false confidence, the Node toolchain/version claims may be brittle, and the “0 OPEN” critique gate can become ceremonial unless made mechanically auditable.

## Strengths

- The phase has a clear delivery order: HTML mockup → screenshot → TUI dummy → screenshot → critique → approval → later backend.
- The separate `cmd/gitid-dummy` binary is the right isolation boundary for DLV-05.
- The terminal-skinned MUI direction correctly avoids building a generic web dashboard that cannot map to a terminal app.
- Semantic parity instead of pixel parity is the right call for HTML vs TUI.
- The pilot-first create-flow plan is valuable; it exercises the hardest patterns before six-surface fan-out.
- The full assembly plan correctly runs comprehensive dummy-nav e2e before presentation/approval.
- The plan explicitly treats Health as read-only and Fixer as mutating, which prevents a common UX ambiguity.
- The approval checklist is appropriately serious about freezing copy, defaults, option sets, and safety affordances.

## Concerns

- **HIGH: No-backend import gate is not airtight.**  
  The `go list -deps | grep internal/(identity|...)` approach only catches known package names. New backend packages, renamed packages, or helper packages with side effects can slip through. The later expanded denylist includes `platform`, `clipboard`, `deps`, etc., but still depends on humans remembering every risky package.

- **HIGH: Surface registration may conflict with placeholder surfaces.**  
  Plan 02-02 seeds placeholder `SurfaceDef`s for the five primary views, while later plans “replace” them. The registry rejects duplicate activation keys. Unless replacement semantics are explicitly designed, plans 02-06 through 02-10 may fail when registering keys `1`–`5`.

- **HIGH: Manifest-driven PTY e2e may not prove real navigation strongly enough.**  
  The manifest gives each screen a `Keys` sequence and a `Signature`. If keys are absolute-from-entry but tests don’t reset state between entries, results become order-dependent. If signatures are generic strings like `backup` or `IdentitiesOnly yes`, the test can pass on the wrong screen.

- **HIGH: “agent-ui-ux-designer critique” is not enforceable as written.**  
  The plan says “spawn agent” and “0 OPEN,” but the artifact format is Markdown and grep-based. That can devolve into self-certification unless every MUST-match dimension is mapped to concrete rows with required statuses.

- **MEDIUM: Node version/package claims are fragile.**  
  The plan pins versions based on a specific research date. That is good, but `react@19.2.7`, `vite@8.1.3`, `typescript@6.0.3`, and MUI v7.3.11 may not exist or may not remain compatible in the actual execution environment. The plan should fail cleanly, but it should not assume registry research is immutable.

- **MEDIUM: `@fontsource/jetbrains-mono` is added without the same audit discipline.**  
  The plan says “pin exact latest 5.x,” but the package is not in the listed slopcheck audit table. It is a new npm dependency and should be audited/pinned with the same standard.

- **MEDIUM: Phase 1 dependency handling is still too optimistic.**  
  Plan 02-03 depends on unexecuted Phase 1 screenshot internals and names functions like `captureHTML` / `captureTUI`. A compile failure is technically fail-fast, but the plan should specify the exact function signatures expected from Phase 1 or include an adapter layer.

- **MEDIUM: 50-screen count is brittle.**  
  Plans hard-code `50+50` in assembly. If a surface legitimately changes screen count during pilot/design, assembly fails until hand-edited. Better to compute expected counts from manifests and separately assert the seven required surfaces exist.

- **MEDIUM: Glob-based route auto-discovery has hidden ordering/type risks.**  
  `import.meta.glob(..., { eager: true })` does not by itself enforce route module shape, route uniqueness, or deterministic route conflicts. Bad or duplicate route exports can silently create confusing capture behavior.

- **MEDIUM: Same-wave fan-out still has shared-file risks.**  
  The plans avoid editing `App.tsx` and capture drivers, which helps. But all six fan-out plans edit `internal/dummytui` package registration behavior indirectly and all add route files under the same TS project. Parallel execution can still conflict through generated lock/build artifacts, formatting, test snapshots, and registry placeholder replacement.

- **MEDIUM: `make lint` in every plan may be too broad for parallel fan-out.**  
  Broad lint across the whole repo while adjacent fan-out branches are incomplete can create false failures if work is merged incrementally or run in isolation.

- **LOW: Some acceptance checks are weak grep checks.**  
  Examples: `grep -q "none"` for shadows, `grep -q "recommended"` for option explanations, `grep -q "OPEN"` for critique status. These can pass while the intended behavior is absent.

- **LOW: Approval line says “today’s date + user’s name” but user identity source is undefined.**  
  The executor may not know the approver’s preferred name. This is minor but can create ambiguity in the one authoritative gate.

## Suggestions

- Replace the backend denylist with an allowlist for `cmd/gitid-dummy` dependencies. For example, allow only:
  - stdlib packages
  - `charm.land/bubbletea/v2`
  - `charm.land/lipgloss/v2`
  - `charm.land/bubbles/v2`
  - `github.com/charmbracelet/x/ansi`
  - `internal/dummytui`
  
  Then fail on any other `github.com/castocolina/gitid/internal/...` package except `internal/dummytui`.

- Make registry replacement explicit. Add `RegisterOrReplace` for placeholder surfaces, or do not register placeholders with final activation keys. Test that final registered surfaces for keys `1`–`5` are exactly identity-manager/global-ssh/global-git/health/fixer.

- Strengthen `manifest.json` schema:
  - require `surface`
  - require unique `screen`
  - require unique `htmlRoute`
  - require unique, screen-specific `signature`
  - require `keysFromHome` or `keysFromSurfaceEntry` explicitly
  - validate every manifest screen exists in both MUI routes and `dummytui.RenderScreen`

- In PTY e2e, reset to a known home state before each manifest entry, or make every key sequence absolute from startup. Assert the active screen ID if possible, not only a text signature.

- Turn parity into structured data, not only Markdown. Keep `FIELDS.md` for humans, but add `parity.json` or frontmatter tables with machine-checkable rows:
  - `dimension`
  - `html_present`
  - `tui_present`
  - `status`
  - `resolution`
  
  Then verify no row has `status != resolved/pass`.

- Audit and pin `@fontsource/jetbrains-mono` explicitly in the supply-chain table.

- Define the exact expected Phase 1 screenshot API:
  - `captureHTML(t?, url, outPath)` or equivalent
  - `captureTUI(view string, outPath string)`
  
  If Phase 1 differs, add a small adapter in `internal/screenshot` rather than letting Phase 2 plans guess.

- Compute final expected screenshot counts from manifests:
  ```sh
  expected=$(python3 - <<'PY'
  import glob,json
  print(sum(len(json.load(open(p))) for p in glob.glob('.planning/design/*/manifest.json')))
  PY
  )
  ```
  Then assert `expected == 50` only if the 50-state inventory is intentionally frozen.

- Add route validation in the mockup build:
  - no duplicate paths
  - every manifest `htmlRoute` has a route module
  - every route renders a stable screen title/signature

- Add a “no backend files changed” gate before approval. For example, assert Phase 2 changed only `.planning/design`, `internal/dummytui`, `cmd/gitid-dummy`, `internal/screenshot` tooling, `e2e`, and Makefile.

- Make approval recording require the user-supplied approver string. Do not infer the name if absent.

## Risk Assessment

**Overall risk: MEDIUM-HIGH.**

The architecture is sound and the plan is unusually thorough, but the phase is large, process-heavy, and depends on several custom gates behaving exactly as intended. The biggest risks are false positives: a dummy that passes import checks while still depending on newly introduced risky internals, navigation tests that prove signatures rather than screens, and parity critiques that appear resolved because Markdown says so. Tightening the import allowlist, manifest schema, route validation, and parity artifact structure would reduce this to MEDIUM.

---

## Consensus Summary

Single independent reviewer (Codex). Verdict: **MEDIUM-HIGH** — architecture and sequencing are sound; the residual risk is **false-positive gates** (checks that pass while the intended property is absent). Priority ranking for `--reviews` replanning:

### Agreed Concerns (priority order)

1. **[HIGH] No-backend gate is a denylist, not an allowlist.** `go list -deps | grep internal/(identity|…)` only catches *named* packages; a new/renamed backend package slips through. → Flip to an **allowlist**: `cmd/gitid-dummy` may import only stdlib + `charm.land/{bubbletea,lipgloss,bubbles}/v2` + `github.com/charmbracelet/x/ansi` + `internal/dummytui`; fail on any other `github.com/castocolina/gitid/internal/*`. (Strictly stronger than the current DLV-05 proof.)
2. **[HIGH] Placeholder-surface registration may duplicate activation keys.** 02-02 seeds placeholder `SurfaceDef`s for keys `1`–`5`; fan-out plans 02-06…02-10 "replace" them, but the registry *rejects duplicate activation keys* → fan-out registration can panic/fail. → Add explicit replacement semantics (`RegisterOrReplace`) OR don't give placeholders final keys; test that final `1`–`5` owners are exactly identity-manager/global-ssh/global-git/health/fixer.
3. **[HIGH] Manifest-driven PTY e2e may prove signatures, not screens.** Order-dependent state + generic signatures (`backup`, `IdentitiesOnly yes`) can pass on the wrong screen. → Reset to home before each manifest entry (or absolute key sequences from startup); assert the active **screen ID**, not just a text signature. Add manifest schema: unique `screen`/`htmlRoute`/screen-specific `signature`, explicit `keysFromHome`, cross-validate every screen exists in both MUI routes and `dummytui.RenderScreen`.
4. **[HIGH] ui-ux-designer "0 OPEN" critique is self-certifiable.** Markdown + `grep -q OPEN` can devolve into rubber-stamping. → Make parity **structured/machine-checkable** (`parity.json` rows: dimension / html_present / tui_present / status / resolution); verify no row `status != resolved`.

### Also raised (MEDIUM/LOW)
- **[MED]** Node version pins are date-stamped assumptions (`react@19.2.7`, `vite@8.1.3`, `typescript@6.0.3`, MUI 7.3.11) — fail cleanly, don't assume registry immutability.
- **[MED]** `@fontsource/jetbrains-mono` added without the slopcheck audit discipline the other 10 npm deps got.
- **[MED]** Phase-1 dependency: pin the **exact expected `captureHTML`/`captureTUI` signatures** or add an adapter in `internal/screenshot` rather than guessing.
- **[MED]** `50+50` hard-coded in assembly is brittle → compute expected counts from manifests; assert the 7 required surfaces separately.
- **[MED]** `import.meta.glob(..., {eager:true})` doesn't enforce route-module shape/uniqueness → add mockup-build route validation (no dup paths; every `htmlRoute` has a module).
- **[MED]** Residual same-wave fan-out conflicts via lockfile/build/snapshot artifacts + registry placeholder replacement; broad `make lint` per plan may false-fail during incremental parallel work.
- **[LOW]** Weak grep acceptance checks (`grep -q none` for shadows, `grep -q recommended`); approver-name source for the DLV-08 line is undefined → require a user-supplied approver string, don't infer.

### Divergent Views
None — single reviewer.

### Suggested new gate (Codex)
Add a **"no backend files changed"** assertion before approval: Phase 2 may only touch `.planning/design`, `internal/dummytui`, `cmd/gitid-dummy`, `internal/screenshot` tooling, `e2e`, and the Makefile — a positive-space complement to the import-graph gate.
