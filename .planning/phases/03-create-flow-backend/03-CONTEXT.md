# Phase 3: Create Flow Backend - Context

**Gathered:** 2026-07-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Wire the **real backend** behind the approved create-flow design in the **real
`gitid` binary**: algorithm catalog → SSH form (live `Host` block preview,
mouse + keyboard focus) → two-stage connectivity test against throwaway temp
configs (exact commands + real output shown) → confirm-write ceremony →
persist with timestamped backup. Gated by per-screen PTY e2e on the real built
binary (DLV-06) and the visual-regression gate vs the approved Phase 2
screenshots (DLV-04).

Requirements: SSHUI-01..05, TEST-01..03, KEY-06, DLV-04, DLV-06.

**Design is FROZEN by Phase 2** (approved 2026-07-06 by Pepe). Phase 3 changes
zero design decisions; the only design deltas are the scoped divergences
explicitly decided below (each documented like D9 and cleared through the
visual-regression review).

**Explicitly NOT in this phase:** Git fragment/`includeIf`/`allowed_signers`
writes (Phase 4), identity manager + CLI command surface rebuild (Phase 5,
SHELL-01..03), upload automation and full provider instructions (Phase 9,
UP-01..03), layout-migration UX (Phase 6/8).

</domain>

<decisions>
## Implementation Decisions

### Test gate semantics (TEST-01/02/03)
- **D-01 — Store gate = PASS or ReachableNotUploaded.** The confirm-write step
  unlocks when connectivity + `ssh -G` IdentityFile resolution are proven, even
  though a fresh key cannot authenticate before its `.pub` is uploaded
  (Phase 9). The identity persists with a clear "key not yet uploaded" status
  (taxonomy: key-unused). Only hard `Failure` blocks store.
- **D-02 — ReachableNotUploaded renders as a NEW warning state** — yellow `!`
  + word per the established glyph contract (warning=! yellow), copy like
  "Reachable — key not uploaded yet". This is a **documented scoped design
  divergence** (like D9): the approved demos only have success/fail test
  states. Must clear the visual-regression review with its allowlist entry.
- **D-03 — At ReachableNotUploaded, offer: copy `.pub` to clipboard
  (`internal/clipboard`) + ONE hint line** naming the provider's key-settings
  page. Full instructions/automation stay in Phase 9 — no duplicated UP-01 copy.
- **D-04 — Stage sequencing: gate stage 2 on stage 1, auto-chain on success.**
  Stage 2 (by-alias via temp config + `ssh -G`) fires **instantly** after a
  stage-1 success (PASS or ReachableNotUploaded) when the user set an alias —
  no manual keypress between stages. A hard stage-1 `Failure` stops with a
  retry affordance.

### Storage target (TEST-03, STORE-01)
- **D-05 — Auto-detect the active layout; no in-flow choice.** Create writes to
  the detected layout (existing gitid Include'd file or adopted external
  Include → write there; existing in-file managed blocks → keep in-file). The
  confirm-write screen names the resolved target file. No new screen.
- **D-06 — Include'd layout is the DEFAULT for fresh setups.** ⚠ **Supersedes
  STORE-01's documented in-file default.** On a machine with no existing gitid
  layout, create writes `~/.ssh/config.d/gitid.config` and adds the
  `Include ~/.ssh/config.d/*.config` line near the top of `~/.ssh/config`; the
  confirm-write ceremony previews **BOTH** file changes on first create.
  Update REQUIREMENTS.md STORE-01 wording when touched.
- **D-07 — Layout switching (migrate) is NOT surfaced in Phase 3.** The
  Phase 1 migrate core stays headless until Global SSH Options (Phase 6) /
  Fixer (Phase 8).
- **D-08 — macOS `Host *` globals block: written on EVERY create,
  idempotent** whole-block rewrite, ordered after specific hosts (SSHUI-05).
  First create previews it alongside the identity block; later creates are
  no-ops when unchanged (self-healing).
- **D-09 — Alias collision blocks at the SSH form.** Validate the alias
  against ALL parsed `Host` patterns (managed + hand-written) as the user
  types; a collision shows an inline error naming the conflicting entry and
  the form won't advance. Never write ambiguous first-match-wins config.

### Reuse-existing-key UX (KEY-06)
- **D-10 — Picker: scan `~/.ssh` for parseable private keys** and list
  filename + algorithm + fingerprint, plus a manual-path row for keys living
  elsewhere.
- **D-11 — Validation: parse + derive missing `.pub` + normalize perms.**
  Private key must parse; a missing `.pub` is derived (`internal/keygen`
  derive.go); permissions normalized to 600/644 as part of the write ceremony
  (previewed, never silent). Encrypted keys accepted when a matching `.pub`
  exists alongside (no passphrase prompt).
- **D-12 — In-use keys: warn, allow.** The picker labels keys already
  referenced by an identity ("in use by: personal") and shows a same-provider
  warning on selection (providers bind one auth key to one account), but never
  blocks — cross-provider reuse is legitimate.
- **D-13 — Algorithm breadth: any parseable SSH key** (x/crypto/ssh),
  including legacy non-catalog algorithms, with an informational note — never
  a block. Only the *generate* path is catalog-locked.

### Real-binary entry point & code structure
- **D-14 — Archive the POC command surface.** Phase 3 removes the 0.0.1 POC
  Cobra commands from `cmd/gitid` (identity add/rotate/delete, doctor, adopt,
  copy, list, …; git history + `.planning/archive/` preserve them). The
  Phase 1 `debug caps` command stays. Internal packages remain substrate. The
  v1.0 CLI surface is rebuilt deliberately in Phase 5 (SHELL-03).
- **D-15 — Bare `gitid` opens the real app shell** rendering the approved
  chrome; the create flow is fully live.
- **D-16 — Not-yet-wired views show their approved DEMO content with a
  persistent warning note** at the top ("Preview — demo data, not wired to
  your system yet"). Each later phase removes the note as it wires the view.
  The note is a documented scoped divergence for the visual gate.
- **D-17 — Extract a shared backend-free presentation package** (theme, shell
  frame, screen views) that BOTH `cmd/gitid` and `cmd/gitid-dummy` import: the
  dummy injects fixture data, the real app injects backend state — ONE source
  of truth for the frozen design. Constraint: the dummy side stays provably
  backend-free; **restore the dummytui no-backend import-graph test (Phase 2
  VERIFICATION W2 carry-over)** and update the `gate-no-backend-files`
  allowlist for the new package split.

### Wizard Git-step in Phase 3
- **D-18 — "Skip Git" is FUNCTIONAL; the Git form is demo'd.** Skipping
  proceeds to the review ceremony writing SSH artifacts only; the identity is
  stored SSH-only and marked incomplete (the frozen hint copy already says
  exactly this). The Git form renders with the D-16 demo+warning treatment.
- **D-19 — Git-form `[ Continue ]` stays disabled with a NEW scoped-divergence
  reason** (e.g. "— Git configuration arrives with the next build"),
  documented next to D9's precedent and removed in Phase 4. The frozen
  validation reason ("— needs user.name + a valid email") is NOT reused to
  lie about capability.

### Provider model in the SSH form (SSHUI-01)
- **D-20 — Provider inferred from the SSH Host suffix; no new field.**
  Auto-join starts from a `github.com` default; editing the SSH Host suffix
  consults a known-provider table (github.com / gitlab.com / bitbucket.org →
  their alt-SSH endpoints: `ssh.github.com`, `altssh.gitlab.com`,
  `altssh.bitbucket.org`, all port 443 per recipes) which auto-fills Real
  hostname + Port. Everything stays editable; the approved 4-field form stays
  byte-exact.
- **D-21 — Unknown/custom provider → Port defaults to 22** with Real
  hostname = the host itself, plus a hint explaining the alt-SSH story. Known
  providers keep the recipe pairing (alt-SSH endpoint + 443). Recorded as a
  provider-aware refinement of SSHUI-01's blanket "default 443".

### PTY e2e vs network (DLV-06)
- **D-22 — Deterministic seam = PATH-shim fake `ssh`.** The e2e harness
  prepends a fake `ssh` executable to PATH emitting recorded real outputs
  (auth banner / "Permission denied (publickey)" / `ssh -G` resolution) keyed
  by args. The binary under test is 100% real — real exec plumbing, real
  output parsing, real UI; only the external tool is swapped. No in-Go mock
  tester in e2e (that would defeat DLV-06's purpose).
- **D-23 — Skippable real-network smoke:** a `make` target running the real
  two-stage flow against github.com asserting ReachableNotUploaded/PASS,
  auto-skipped when the network/provider is unreachable. Local + UAT use;
  NOT a required CI gate.

### Visual-regression gate mechanics (DLV-04)
- **D-24 — Two-layer gate + cross-AI review.**
  1. An automated `make` gate diffs the live TUI's deterministic View() text
     captures against the approved dummy goldens — byte-exact except where a
     **per-screen documented divergence allowlist** (D-02, D-16, D-19) says
     otherwise.
  2. Reviewer critique of the PNG pairs: `agent-ui-ux-designer` **and Codex**
     (cross-AI, per the Phase 2 pattern) review BOTH the text diffs and the
     screenshots.
- **D-25 — Failure policy: hard gate + severity triage.** Any unallowlisted
  golden-text diff fails the make gate outright. Reviewer findings triage by
  severity: CRITICAL/HIGH block the wave; MEDIUM/LOW are recorded and fixed or
  explicitly accepted — same convention as Phase 2's review rounds.

### Claude's Discretion
- Exact package name/layout for the extracted shared UI package (D-17), and
  how theme/screen registries split between fixture-driven and backend-driven
  construction.
- Capture geometry, golden-file layout, and allowlist file format for the
  D-24 gate (keep Phase 1's 100×30 TUI geometry so diffs are apples-to-apples).
- Known-provider table location/shape in the core (D-20).
- Exact copy for the new warning-state line, demo-note banner, and Continue
  disabled reason — draft during the UI wave, freeze via the same copy-freeze
  grep mechanism as 02-STYLE-SPEC.md §6.
- Timeouts/retry budget for the two test stages (Phase 1's probe-timeout
  pattern is precedent).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### North Star — canonical config end state
- `recipes/README.md` — what the recipes establish (alias per identity, Port 443 alt-SSH, `IdentitiesOnly yes`); structure, not key type.
- `recipes/ssh-config.recipe` — canonical `~/.ssh/config` shape the create flow's `Host` blocks must produce.
- `recipes/gitconfig.recipe` — canonical `~/.gitconfig` shape (context for the Git step handoff to Phase 4).

### Requirements / roadmap (authoritative)
- `.planning/ROADMAP.md` §"Phase 3" — goal + 5 success criteria.
- `.planning/REQUIREMENTS.md` §A (DLV-04/06), §C (KEY-06), §D (SSHUI-01..05), §E (TEST-01..03), §F (STORE-01 — superseded default, see D-06).

### Approved design (BINDING — the Phase 2 contract)
- `.planning/design/APPROVAL.md` — the DLV-08 approval record (2026-07-06, Pepe).
- `.planning/design/create-flow/FIELDS.md` — per-screen field contract for all 12 create-flow states + the D1–D9 parity dimensions.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md` — semantic style contract (theme roles, arrow-key precedence, copy freeze §6, parity dimensions §3).
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-DESIGN-DECISIONS-CHECKPOINT-2.md` — binding D1–D9 checkpoint-2 contract (D9 is the scoped-divergence documentation precedent).
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-REDESIGN-SPEC.md` — surface-level redesign spec.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-UX-DIRECTION.md` — §4.1 create-flow 12 named states; key-allocation table; §5 mutation ceremony beats.
- `.planning/design/<surface>/{html,tui}/*.png` + `.planning/design/REFERENCE-INDEX.md` — the approved screenshot reference set the D-24 gate diffs against.

### Prior phase context / carry-overs
- `.planning/phases/01-foundations-spikes-ci/01-CONTEXT.md` — Phase 1 decisions (D-09 Include layout, D-11 taxonomy, capture geometry D-04).
- `.planning/STATE.md` §Blockers — W1 (`insteadOf` demo gap, Phase 4/7 concern) and W2 (**restore the dummytui no-backend import-graph test in Phase 3** — folded into D-17).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/tester/` — two-stage SSH test with exact-command capture and the
  PASS / **ReachableNotUploaded** / Failure classification (by output
  substring, never exit code); read-only. Directly powers D-01..D-04.
- `internal/keygen/` — algorithm registry (ed25519 + rsa-4096 real,
  probe-gated catalog), `derive.go` (.pub derivation for D-11),
  `signers.go`.
- `internal/sshconfig/` + `internal/filewriter/` — dual-layout parse/render,
  sentinel managed blocks, timestamped backup + atomic write, block-prepend
  (Include-near-top), round-trip proven. Powers D-05..D-09.
- `internal/adopter/` — existing-Include detection for D-05 auto-detect.
- `internal/platform/` — capability probe driving catalog availability
  (KEY-03 hints on the algorithm screen).
- `internal/identity/` — inventory + 8-state taxonomy (SSH-only identities
  land as incomplete per D-18).
- `internal/clipboard/` — the D-03 copy-`.pub` action.
- `internal/dummytui/` + `cmd/gitid-dummy/` — the frozen screen renderings,
  central Theme, shell frame, key-allocation — the raw material for the D-17
  extraction.
- `e2e/ui_pty_e2e_test.go` — raw-keystroke PTY harness (real xterm CSI
  injection, bounded close) to extend for DLV-06 per-screen e2e.
- `internal/screenshot/` + Makefile capture targets — the deterministic
  capture pipeline the D-24 gate builds on.

### Established Patterns
- Safe-write invariant: timestamped backup + idempotent whole-block rewrite +
  atomic temp→rename→chmod + explicit confirm; non-managed content preserved
  verbatim.
- Injectable seams with EXPORTED real constructors (`Build*Deps()`), exercised
  end-to-end — the recurring injected-seam wiring blindspot is CLOSED only
  when a PTY e2e drives the real wiring (hence D-22's PATH-shim, not Go mocks).
- Copy freeze via repo-wide grep gates (02-STYLE-SPEC.md §6) — new frozen
  strings (D-02/D-16/D-19 copy) join that mechanism.
- `gate-no-backend-files` make target + import-graph allowlist — must be
  re-shaped by the D-17 package split, not weakened.

### Integration Points
- `cmd/gitid/main.go` — POC commands removed (D-14), new TUI entry wired
  (D-15); `debug caps` kept.
- New shared UI package ← `internal/dummytui` extraction (D-17); both
  binaries' import graphs re-gated.
- Create-flow backend seams: keygen registry ↔ platform probe ↔ tester ↔
  sshconfig/filewriter ↔ identity inventory (state after store).
- Makefile: new gate target(s) for the golden-text visual diff (D-24) + the
  skippable real-network smoke (D-23); CI wiring on the existing 3-runner
  matrix.

</code_context>

<specifics>
## Specific Ideas

- User's entry-point direction, verbatim intent: "Archive the old app, copy
  the demo shell to main, adding note labels warn at beginning of the other
  screens, and start working on the corresponding one" — realized as
  D-14/D-15/D-16/D-17.
- Stage 2 should feel instant after stage 1 passes ("trigger 2 instantly if 1
  successful and the user set alias") — no ceremony between stages (D-04).
- "Includes as default" — the user explicitly wants the Include'd file layout
  as the fresh-setup default (D-06), superseding STORE-01's in-file default.
- Codex participates in the visual-regression review of BOTH text diffs and
  screenshots (D-24), extending the Phase 2 cross-AI review pattern.

</specifics>

<deferred>
## Deferred Ideas

- **Layout-migration UX** (in-file ↔ Include'd) — core exists since Phase 1;
  surface it in Global SSH Options (Phase 6) or Fixer (Phase 8).
- **Full upload instructions + automation** (UP-01..03) — Phase 9; Phase 3
  ships only the D-03 clipboard-copy + one-line hint.
- **CLI create command / non-interactive flags** — the v1.0 CLI surface is
  rebuilt in Phase 5 (SHELL-03); Phase 3 removes the POC commands without
  replacing them beyond the TUI + `debug caps`.
- **Git form wiring** (fragment + `includeIf` + `allowed_signers`) — Phase 4;
  Phase 3 leaves the form demo'd with Continue disabled (D-18/D-19).
- **W1 carry-over** (`insteadOf` URL rewriting not rendered in any demo) —
  Phase 4/7 design concern, noted in STATE.md; not a Phase 3 item.

</deferred>

---

*Phase: 3-Create Flow Backend*
*Context gathered: 2026-07-07*
