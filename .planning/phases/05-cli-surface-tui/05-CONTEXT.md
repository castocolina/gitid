# Phase 5: CLI Surface + TUI - Context

**Gathered:** 2026-06-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 5 delivers the two user-facing front-ends over the already-built, tested,
UI-free core:

- **CLI surface (CLI-01, CLI-02):** finalize the full `gitid` Cobra command tree
  so every Phase 2–4 capability is reachable, and ship working shell completion
  for bash, zsh, and fish (Cobra's auto-registered `completion` subcommand).
- **TUI (TUI-01, TUI-02):** a Bubble Tea app that launches on the doctor
  dashboard (`gitid` with no args) and lets the user navigate to identity/account
  management, including in-app add/edit forms — **without ever forking the core
  orchestration or bypassing the prove-before-write safety model** (keygen →
  two-phase `ssh` test → timestamped backup → explicit confirm → re-test).

The doctor's grouped-by-family, UI-agnostic finding model (Phase 4 D-06) was
built specifically to be the dashboard's data source; the TUI consumes
`doctor.Run` findings directly rather than re-deriving checks.

**Net-new work:** the TUI is greenfield — `tui/` is currently a stub and
`charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2` are
**not yet in `go.mod`** (only `cobra` is). The CLI is ~90% built (the `identity`,
`baseline`, and `doctor` groups exist); Phase 5 reconciles its command names to
the SC-4 surface and adds the missing on-demand `copy` command.

**In scope (Phase 5 requirements):** CLI-01, CLI-02, TUI-01, TUI-02. Plus the
on-demand `copy` command (CLIP-02) and on-demand upload instructions (UP-02),
which are reachable-surface gaps these front-ends close.

**Out of scope:**
- **Visual design contract** (layout, colors, spacing, component styling) — goes
  to `/gsd-ui-phase 5` → `05-UI-SPEC.md` BEFORE planning, consistent with phases
  03.1 and 04. This CONTEXT captures *behavioral* decisions only.
- **In-TUI Delete and Rotate forms** — these mutating flows stay CLI-only; the
  TUI hands off to the proven `gitid identity delete` / `gitid rotate` commands.
- **In-TUI doctor auto-fix** — the dashboard shows findings + suggested fixes but
  applying routes to the audited CLI path (`gitid doctor --fix`).
- **Baseline management forms in the TUI** — `baseline setup/show` remain CLI
  commands for the MVP (not raised as an in-app form requirement; can be revisited
  if a Baseline tab is added).
- **New core/orchestration logic** — the TUI reuses the existing `identity.Deps`
  injected-function seams and `doctor.Run`; no business logic is reimplemented in
  the UI layer.
- **Windows / PowerShell completion** — v1 is macOS + Linux only (PROJECT.md);
  CLI-02 scopes completion to bash/zsh/fish.

</domain>

<decisions>
## Implementation Decisions

### TUI scope & depth
- **D-01 (Hybrid front-end — one core, two shells):** the TUI is a **second
  front-end over the proven core**, never a fork of its logic. In-app forms drive
  the SAME orchestration the CLI drives, reused through the existing
  `identity.Deps` injected-function seams. No business logic is duplicated in the
  UI layer.
- **D-02 (in-app MVP form set):** the TUI ships native forms for **Create
  identity**, **Update fields**, **Add-account / alias**, and **Copy pubkey**.
  These satisfy SC-2's "add/edit forms without leaving the application."
- **D-03 (Delete + Rotate stay CLI-only):** the higher-blast-radius / heaviest
  mutating flows (`delete`, key `rotate`) are **not** in-app forms for the MVP;
  the TUI hands off to the proven CLI commands. Clean, lean MVP boundary that
  keeps dangerous mutations on one audited path.
- **D-04 (prove-before-write must remain visible):** any in-app mutation MUST
  surface the two-phase flow — show the **exact command run and its real output**
  (CLAUDE.md "input + output" rule), note the timestamped backup, and require an
  **explicit confirm before any write**. The *presentation* (dedicated screen vs
  inline status) is Claude/UI-SPEC discretion; the *contract* (real cmd+output +
  explicit pre-write confirm) is non-negotiable.

### Command surface (SC-4 / CLI-01 / CLI-02)
- **D-05 (nested canonical + top-level aliases):** keep `identity ...` as the
  canonical group (`add`/`list`/`test`/`rotate`/`update`/`delete`) and add thin
  **top-level aliases** for the SC-4-named commands users reach for: `gitid
  rotate`, `gitid copy`, `gitid host add`. Best discoverability; SC-4's literal
  names are present.
- **D-06 (new `copy` command — CLIP-02 + UP-02):** add a `copy <name>` command
  (top-level alias + `identity copy`) that copies the identity's **public key to
  the clipboard** AND prints the existing provider **upload instructions**
  (reusing `uploadInstructions()` in `cmd/gitid/upload.go`). One command covers
  on-demand clipboard copy (CLIP-02) and on-demand upload steps (UP-02).
- **D-07 (`host add` alias):** expose `gitid host add` as a clear alias to the
  existing add-account/alias flow (today `identity add` mode 3 — adds another host
  alias to an existing identity). Matches SC-4 wording without forking logic.
- **D-08 (completion via Cobra default):** rely on Cobra's auto-registered
  `completion` subcommand for bash/zsh/fish (CLI-02). Verify each of the three
  shells produces a valid script; no hand-written completion code.

### Dashboard launch behavior (TUI-01)
- **D-09 (async / progressive launch):** `gitid` (no args) launches **instantly**
  to the dashboard and **streams each of the six check families' results in as
  they complete** (Bubble Tea async `tea.Cmd` per family, partial render on each
  `Msg`). NOT a single blocking `doctor.Run()` on launch. → architectural input:
  per-family commands, result ordering, and partial/loading render states.
- **D-10 (TUI-native view over structured findings):** render a lipgloss-styled
  dashboard (family sections, severity colors, `✓` passes / findings) that
  consumes the **same structured `doctor.Run` findings** the CLI uses — the
  UI-agnostic model Phase 4 D-06 built for exactly this. Do **not** embed the CLI
  text renderer's output, so findings remain individually selectable.
- **D-11 (fixes hand off to CLI):** the dashboard shows each finding + its
  suggested fix, but **applying** routes to the proven CLI flow (`gitid doctor
  --fix`). Consistent with D-03 — mutations with blast radius stay on one audited
  path; the Phase 4 cmd-layer fixer is not re-wired into the TUI for the MVP.

### Navigation & key model (TUI-02)
- **D-12 (drill-down stack navigation):** the doctor dashboard is home; **Enter**
  drills Dashboard → Identity list → Identity detail → add/edit form, **Esc** pops
  back up the stack. One focused screen at a time; simplest mental model for the
  MVP.
- **D-13 (keymap — arrows + vim + global):** arrow keys and vim (`j/k/h/l`) for
  movement, plus a consistent global map: `q` quit, `Esc` back, `Enter` select,
  `?` help, `r` refresh. A visible help/footer bar is shown. `bubbles` components
  support both binding styles natively.
- **D-14 (visual contract deferred to `/gsd-ui-phase 5`):** layout, colors,
  spacing, and component styling are produced as `05-UI-SPEC.md` via
  `/gsd-ui-phase 5` **before** `/gsd-plan-phase 5`, consistent with phases 03.1
  and 04. This CONTEXT is behavioral only.

### Claude's Discretion
- **Prove-before-write presentation** (D-04) — dedicated test+confirm screen vs
  inline status; must keep real cmd+output + explicit pre-write confirm.
- **Exact command/flag naming and help copy** for the new aliases and `copy`
  command — consistent with the Phase 2/3/4 minimal-real-Cobra pattern.
- **Bubble Tea model decomposition** — single root model with a view-stack vs
  nested sub-models; how the async per-family commands are structured; how
  `identity.Deps` seams are surfaced to in-app forms. TDD-first; UI layer holds no
  business logic.
- **Whether to add a Baseline tab/section** to the dashboard later (out of MVP
  scope; not blocked).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase intent, scope & locked decisions
- `.planning/ROADMAP.md` §"Phase 5: CLI Surface + TUI" — goal + 4 success
  criteria + `Mode: mvp` + `UI hint: yes`. SC-4 enumerates the reachable command
  surface (`doctor`, `identity add/list/test`, `host add`, `rotate`, `copy`).
- `.planning/REQUIREMENTS.md` §CLI-01, CLI-02, TUI-01, TUI-02 — the four Phase 5
  requirement statements; plus §CLIP-02 (on-demand pubkey copy) and §UP-02
  (on-demand upload instructions) which the new `copy` command closes.
- `.planning/PROJECT.md` §Constraints + §"Key Decisions" — thin CLI/TUI over a
  tested UI-free core; charm.land v2 vanity import paths (NOT
  `github.com/charmbracelet/*`); macOS + Linux only; English-only artifacts;
  safety (backup + idempotent + confirm) applies to every write the TUI triggers.
- `CLAUDE.md` — Technology Stack table (cobra v1.10.2, bubbletea/v2 v2.0.7,
  lipgloss/v2 v2.0.3, bubbles/v2 v2.1.0 with exact `charm.land/...` import paths)
  and the §"Cobra + shell completion" notes (`InitDefaultCompletionCmd`); the
  hypothesis→test→implement working method and "tests show input + output" rule
  the TUI's prove-before-write screens must honor.

### Phase 4 — the dashboard's data source
- `.planning/phases/04-doctor/04-CONTEXT.md` §D-06 (grouped-by-family layout is
  "the seed of the Phase 5 TUI dashboard"; keep the finding type UI-agnostic),
  §D-05 (4-level severity → dashboard colors), §D-07 (exit codes), §D-08 (TTY
  color / NO_COLOR), §"Integration Points" (the finding family grouping + `✓`/
  severity model is the data shape the TUI consumes).
- `.planning/phases/04-doctor/04-UI-SPEC.md` — the visual contract for the doctor
  report the dashboard reuses (family sections, severity styling).

### Prior front-end patterns the TUI/CLI reuse
- `.planning/phases/03-full-identity-crud-multi-identity/03-CONTEXT.md` — the
  identity CRUD command shapes (list/update/delete) the TUI navigates and the CLI
  finalizes.
- `.planning/phases/02-first-identity-end-to-end/02-CONTEXT.md` — the
  four-artifact create flow, two-phase test/`ssh -G` model, and clipboard/upload
  steps the in-app Create form and `copy` command reuse.
- `.planning/phases/03.1-.../03.1-UI-SPEC.md` and `04-doctor/04-UI-SPEC.md` —
  precedent for the `/gsd-ui-phase` UI-SPEC this phase will also produce.

### Architecture & stack
- `.planning/research/ARCHITECTURE.md` — `internal/` seams; the dependency arrow
  rule (`tui/` imports internal packages, is never imported by them — see
  `tui/doc.go`).
- `.planning/research/STACK.md` — Cobra completion, `ssh -G`/`ssh-add` invocation
  patterns the in-app test/test flow surfaces.

### Existing code the front-ends extend
- `cmd/gitid/main.go` — `newRootCmd()` assembles the current tree (`identity`,
  `baseline`, `doctor`); the SC-4 reconciliation (top-level `rotate`/`copy`/`host
  add` aliases) edits here.
- `cmd/gitid/upload.go` — `uploadInstructions(provider)`, reused by the new `copy`
  command (D-06).
- `cmd/gitid/add.go` — the add-account/alias flow (`runAddAccount`, mode 3)
  `host add` aliases (D-07).
- `cmd/gitid/doctor.go` — the `doctor.Run` invocation + grouped renderer the TUI
  dashboard's data path mirrors (D-10).
- `tui/doc.go`, `tui/tui_stub_test.go` — the current stub package the real TUI
  replaces; the one-directional dependency rule is documented there.
- `go.mod` — currently only `cobra`; Phase 5 adds the three `charm.land/*/v2`
  modules at the pinned versions.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`doctor.Run` + the UI-agnostic finding/severity/family model** (Phase 4) —
  the dashboard consumes these findings directly (D-10); no checks are re-derived
  in the UI.
- **`identity.Deps` injected-function pattern** (`modes.go`/`update.go`/
  `delete.go`) — every external effect is a function field, so the in-app forms
  (D-01/D-02) drive the same orchestration the CLI does, fully fake-testable.
- **`uploadInstructions(provider)`** (`cmd/gitid/upload.go`) — reused by the new
  `copy` command to print upload steps (D-06).
- **`runAddAccount` / add-account mode** (`cmd/gitid/add.go`) — aliased by `host
  add` (D-07); reused by the in-app Add-account form.
- **Cobra tree in `newRootCmd()`** (`cmd/gitid/main.go`) — extended with
  top-level aliases; `completion` is auto-registered (D-08).
- **`internal/clipboard`** (Phase 2, atotto) — backs the `copy` command and the
  in-app Copy-pubkey form (CLIP-02).

### Established Patterns
- **Thin shells over a tested UI-free core; one-directional dependency** —
  `tui/` imports internal packages and is never imported by them (`tui/doc.go`).
  No business logic in the UI layer.
- **TDD-first** — Bubble Tea models are testable via `teatest`/message-driven
  unit tests; the prove-before-write screens assert real cmd+output rendering.
- **Safe-write rule applies to every TUI-triggered mutation** — backup → atomic →
  idempotent block rewrite → explicit confirm, via the same `internal/filewriter`
  chokepoint the CLI uses (D-04). The TUI adds no new write path.
- **charm.land v2 vanity imports** — `charm.land/bubbletea/v2` etc., NOT
  `github.com/charmbracelet/*` (PROJECT.md / CLAUDE.md; v2.0.7 / v2.0.3 / v2.1.0).
- `make test` / `make lint` (golangci-lint + gosec, hard-fail) gate every commit;
  pre-push runs `go test -race`. English-only. Commits squashed at plan close.
- `gsd-tools.cjs` needs Volta's bin on PATH in non-interactive shells (GSD scripts
  only, not the Go build): `export PATH="$HOME/.volta/bin:$PATH"`.

### Integration Points
- **`gitid` no-args entry** — `main()` must branch: no args → launch TUI;
  otherwise → existing Cobra `Execute()`. Preserve the tiered `doctorExitCode`
  path (IN-03) for CLI doctor runs.
- **Dashboard → core** — async per-family `tea.Cmd`s call into `doctor.Run`'s
  family checks (D-09); the TUI renders findings as they arrive.
- **In-app forms → core** — Create/Update/Add-account/Copy drive
  `identity.Create`/update/add-account/clipboard via `identity.Deps`; Delete/Rotate
  hand off to the CLI commands (D-03).
- **New top-level aliases + `copy`** wire into `newRootCmd()` (`cmd/gitid/main.go`).

</code_context>

<specifics>
## Specific Ideas

- Command surface after reconciliation: canonical `identity add/list/test/rotate/
  update/delete` + `baseline setup/show` + `doctor`, PLUS top-level aliases `gitid
  rotate`, `gitid copy <name>`, `gitid host add`, and the auto-registered `gitid
  completion {bash|zsh|fish}`.
- `copy <name>` = pubkey → clipboard + printed upload instructions in one command.
- TUI launches async to the doctor dashboard; six families stream in; TUI-native
  lipgloss view over structured findings; `r` refreshes.
- Drill-down stack: Dashboard ⇄ Identity list ⇄ Identity detail ⇄ add/edit form;
  Esc pops. Keymap: arrows + `j/k/h/l` + `q`/`Esc`/`Enter`/`?`/`r`; visible help bar.
- In-app forms: Create, Update, Add-account, Copy. Delete + Rotate hand off to CLI.
- Doctor fixes shown in the dashboard but applied via `gitid doctor --fix`.

</specifics>

<deferred>
## Deferred Ideas

- **In-TUI Delete and Rotate forms** — kept CLI-only for the MVP (D-03); could be
  added to the TUI in a later pass once the in-app prove-before-write pattern is
  proven for the lighter mutations.
- **In-TUI doctor auto-fix** (per-finding confirm + apply, reusing the Phase 4
  cmd-layer fixer through `filewriter`) — deferred (D-11); MVP shows findings and
  hands off to `gitid doctor --fix`.
- **Baseline management in the TUI** (a Baseline tab/section over `baseline
  setup/show`) — not in the MVP form set; revisit if a tabbed navigation is added.
- **PowerShell completion** — out of scope for v1 (macOS + Linux only); Cobra
  would generate it for free if Windows is ever targeted.

### Reviewed Todos (not folded)
None — `todo.match-phase 5` returned no matches.

</deferred>

---

*Phase: 5-CLI Surface + TUI*
*Context gathered: 2026-06-12*
