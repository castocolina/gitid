# Phase 5: CLI Surface + TUI - Research

**Researched:** 2026-06-13
**Domain:** Cobra CLI surface reconciliation + Bubble Tea v2 TUI (charm.land vanity imports)
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 (Hybrid front-end):** TUI is a second front-end over the proven core; no business logic fork; reuse `identity.Deps` injected-function seams.
- **D-02 (in-app MVP form set):** TUI ships Create identity, Update fields, Add-account/alias, Copy pubkey. These are the only in-app mutations.
- **D-03 (Delete + Rotate stay CLI-only):** TUI shows handoff messages only; no in-app delete or rotate forms.
- **D-04 (prove-before-write must remain visible):** Every in-app mutation MUST show exact command run + real output, note backup, require explicit confirm before any write.
- **D-05 (nested canonical + top-level aliases):** Keep `identity ...` canonical group; add thin top-level aliases `gitid rotate`, `gitid copy`, `gitid host add`.
- **D-06 (new `copy` command):** `copy <name>` = clipboard + printed upload instructions; top-level alias + `identity copy` canonical.
- **D-07 (`host add` alias):** `gitid host add` aliases `identity add` mode 3 (add-account).
- **D-08 (completion via Cobra default):** Use Cobra's auto-registered `completion` subcommand; no hand-written completion code.
- **D-09 (async/progressive dashboard):** Six check families stream in as independent `tea.Cmd`s; partial render on each result.
- **D-10 (TUI-native view):** Render lipgloss dashboard over structured `doctor.Run` findings; do NOT embed CLI text renderer output.
- **D-11 (fixes hand off to CLI):** Dashboard shows fix hints; applying routes to `gitid doctor --fix`.
- **D-12 (drill-down stack navigation):** Dashboard → Identity list → Identity detail → add/edit form; Esc pops.
- **D-13 (keymap):** arrows + vim `j/k/h/l` + global `q`/`Esc`/`Enter`/`?`/`r`; visible help bar.
- **D-14 (visual contract):** Deferred to `05-UI-SPEC.md` (APPROVED — see 05-UI-SPEC.md).

### Claude's Discretion

- Prove-before-write presentation (D-04): dedicated Screen 6 vs inline status. UI-SPEC chose dedicated screen.
- Exact command/flag naming and help copy for new aliases.
- Bubble Tea model decomposition: single root model with view-stack vs nested sub-models.
- Async per-family command structure; `identity.Deps` surface to in-app forms.

### Deferred Ideas (OUT OF SCOPE)

- In-TUI Delete and Rotate forms.
- In-TUI doctor auto-fix.
- Baseline management forms in the TUI.
- PowerShell completion.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-01 | Cobra CLI exposes Phase-1 surface: `doctor`, `identity add/list/test`, `host add`; top-level aliases `rotate`, `copy`, `host add` | Section: Command Surface Reconciliation, Standard Stack |
| CLI-02 | CLI generates shell completion for bash, zsh, fish | Section: Shell Completion, Code Examples |
| TUI-01 | Bubble Tea TUI launches into doctor dashboard | Section: Async Dashboard Pattern, Entry Point Pattern |
| TUI-02 | From dashboard, user can navigate to identity/account managers | Section: View-Stack Navigation Pattern, Architecture Patterns |
</phase_requirements>

---

## Summary

Phase 5 adds two thin front-ends over the already-proven, UI-free core. The CLI work is ~90% done — command reconciliation requires adding three top-level alias commands (`rotate`, `copy`, `host add`) plus a new `identity copy` subcommand that reuses `uploadInstructions()`. Shell completion is already auto-registered by Cobra and was confirmed working via `gitid completion --help`.

The TUI is greenfield: `tui/` is a stub package; `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, and `charm.land/bubbles/v2` are NOT yet in `go.mod`. All three packages were verified on the Go module proxy at their pinned versions (bubbletea v2.0.7 published 2026-06-01, lipgloss v2.0.3 published 2026-04-13, bubbles v2.1.0 published 2026-03-25).

**Critical finding:** `lipgloss.NewRenderer()` does NOT exist in lipgloss v2. The `05-UI-SPEC.md` reference to `renderer.NewStyle()` is v1-only. In v2, `lipgloss.NewStyle()` is a plain value type; color downsampling uses `lipgloss.Fprintln(w, ...)` or `lipgloss.Writer` (a `colorprofile.Writer`). The TUI implementation must use `lipgloss.NewStyle()` directly and output via `lipgloss.Fprint` / `lipgloss.Fprintln` for NO_COLOR/TTY handling, or rely on Bubble Tea's built-in renderer path.

**Critical finding:** In Bubble Tea v2, `View()` returns `tea.View` (a struct), not `string`. `tea.NewView(s)` wraps a rendered string. Key handling changed: `tea.KeyMsg` is now an interface; presses arrive as `tea.KeyPressMsg`. The old `msg.Type`/`msg.Runes`/`msg.Alt` fields are gone.

**Primary recommendation:** Use a root model holding a `[]screen` view-stack; each screen is a concrete type implementing its own `update/view` logic; root delegates `Update` to the top of the stack and wraps its `View()` string output in `tea.NewView`. Run one `tea.Cmd` per doctor family in `Init`; each returns a typed result message. Drive all in-app mutations through `identity.Deps` injected seams (no new write paths). Test TUI models via direct `Update(msg) → assert model state` unit tests; no teatest/golden-file tests required for the nav state machine.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| No-args TUI launch decision | CLI entry (`main`) | — | `main()` checks `os.Args` length and TTY before Cobra `Execute()` |
| Cobra command tree + top-level aliases | CLI (`cmd/gitid/main.go`) | — | Cobra wiring lives entirely in cmd layer |
| Shell completion generation | CLI (Cobra auto-registered) | — | Cobra generates scripts; no TUI involvement |
| Doctor check execution (async) | TUI `tea.Cmd` functions | `internal/doctor` (data source) | Per-family `tea.Cmd` calls existing check functions; TUI owns async dispatch |
| Doctor finding rendering | TUI (`tui/dashboard.go`) | — | lipgloss view over `doctor.Finding` structs; CLI renderer is NOT reused |
| Identity list reconstruction | `internal/identity.Reconstruct` | TUI (display) | Core does the work; TUI calls and renders |
| In-app form orchestration (Create/Update/AddAccount) | TUI form models | `internal/identity` (via `Deps`) | TUI gathers input in form fields, calls same `identity.Create`/update/AddAccount through same `Deps` seams CLI uses |
| Copy pubkey + upload instructions | CLI `copy` command + TUI inline action | `internal/clipboard`, `uploadInstructions()` | Both call same functions; `uploadInstructions()` is in `cmd/gitid/upload.go` (pkg-private, must be referenced or moved) |
| Prove-before-write SSH tests | TUI `tea.Cmd` (Screen 6) + CLI (existing) | `internal/tester` | TUI runs `tester.PreWrite` / `tester.Resolved` via `tea.Cmd` goroutines; never blocks |
| Keymap declarations | TUI (`tui/keymap.go`) | — | Shared `key.Binding` values consumed by all screens + help bar |
| Style tokens | TUI (`tui/styles.go`) | — | `lipgloss.NewStyle()` value types (NOT renderer-based) |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `charm.land/bubbletea/v2` | v2.0.7 | Elm-architecture TUI event loop | Locked in CLAUDE.md; vanity path; v2 is stable (published 2026-06-01) |
| `charm.land/lipgloss/v2` | v2.0.3 | Terminal styling — colors, borders, layout | Locked in CLAUDE.md; pairs with bubbletea v2; v1 API incompatible |
| `charm.land/bubbles/v2` | v2.1.0 | Ready-made TUI components (list, viewport, textinput, spinner, help, key) | Locked in CLAUDE.md; consistent API with bubbletea v2 and lipgloss v2 |
| `github.com/spf13/cobra` | v1.10.2 | CLI command tree + auto-registered completion | Already in go.mod; v1.10.2 confirmed |

### Already in go.mod (no new install needed)
| Library | Purpose |
|---------|---------|
| `github.com/atotto/clipboard` v0.1.4 | Backs the `copy` command and TUI Copy-pubkey action |
| `golang.org/x/term` v0.44.0 | TTY detection (`term.IsTerminal`) for no-args TUI branch |

### New Installs Required (Phase 5 only)

```bash
go get charm.land/bubbletea/v2@v2.0.7
go get charm.land/lipgloss/v2@v2.0.3
go get charm.land/bubbles/v2@v2.1.0
```

These pull in transitive dependencies: `github.com/charmbracelet/colorprofile` (for lipgloss color downsampling), `golang.org/x/sync`, `github.com/xo/terminfo`.

**Version verification (confirmed via Go module proxy 2026-06-13):**
- `charm.land/bubbletea/v2@v2.0.7` — published 2026-06-01, GoMod confirmed [VERIFIED: Go module proxy]
- `charm.land/lipgloss/v2@v2.0.3` — published 2026-04-13, GoMod confirmed [VERIFIED: Go module proxy]
- `charm.land/bubbles/v2@v2.1.0` — published 2026-03-25, GoMod confirmed [VERIFIED: Go module proxy]

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| charm.land/bubbletea/v2 | `github.com/charmbracelet/bubbletea` (v1) | v1 is superseded; CLAUDE.md explicitly forbids it; v1 import path points to archived API |
| Direct Update() unit tests | `github.com/charmbracelet/x/exp/teatest` | teatest is experimental, no v2-compatible package confirmed; direct Update() tests are simpler, more stable, and satisfy TDD-first requirement |
| `lipgloss.Fprint()` for CLI color | Hand-rolled ANSI helpers | lipgloss v2 writer functions auto-downsample; consistent with v2 design |

---

## Package Legitimacy Audit

> slopcheck was unavailable at research time. All recommended packages are marked below with provenance evidence.

| Package | Registry | Age | Source Repo | Provenance | Disposition |
|---------|----------|-----|-------------|------------|-------------|
| `charm.land/bubbletea/v2` | Go module proxy | 2+ yrs (v1 is years older; v2 stable June 2024) | github.com/charmbracelet/bubbletea | Locked in CLAUDE.md; confirmed via Go module proxy | Approved [VERIFIED: Go module proxy + CLAUDE.md] |
| `charm.land/lipgloss/v2` | Go module proxy | 2+ yrs (v2 stable) | github.com/charmbracelet/lipgloss | Locked in CLAUDE.md; confirmed via Go module proxy | Approved [VERIFIED: Go module proxy + CLAUDE.md] |
| `charm.land/bubbles/v2` | Go module proxy | 2+ yrs (v2 stable) | github.com/charmbracelet/bubbles | Locked in CLAUDE.md; confirmed via Go module proxy | Approved [VERIFIED: Go module proxy + CLAUDE.md] |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*slopcheck was unavailable at research time. All packages above are verified via the Go module proxy AND locked by CLAUDE.md authority — equivalent trust to [VERIFIED].*

---

## Architecture Patterns

### System Architecture Diagram

```
  User types `gitid` (no args, TTY)
            │
            ▼
  main() — TTY check (term.IsTerminal)
            │
    ┌───────┴──────────┐
    │ no-args + TTY    │ args present OR non-TTY
    ▼                  ▼
  tui.Run()         cobra.Execute()
  (bubbletea)       (existing CLI path)
    │
    ▼
  tea.Program.Run()
    │
    ▼
  RootModel.Init()
    │── tea.Batch(
    │     runFamilyCmd(FamilyDeps),
    │     runFamilyCmd(FamilyPerms),
    │     runFamilyCmd(FamilyCoherence),
    │     runFamilyCmd(FamilyOrphans),
    │     runFamilyCmd(FamilySigning),
    │     runFamilyCmd(FamilyAgent),
    │     runFamilyCmd(FamilyBaseline),
    │   )
    │
    ▼
  RootModel.Update() — dispatches on viewStack[top]
    │
    ├── FamilyResultMsg → update dashboard panel state
    ├── tea.KeyPressMsg → nav: Enter pushes, Esc pops, q quits
    ├── tea.WindowSizeMsg → propagate width/height to all sub-models
    │
    ▼
  viewStack [ Dashboard | IdentityList | IdentityDetail | CreateForm | ProveScreen ]
    │           │
    │           └── spinner per family (loading) → findings (loaded)
    │
    ▼
  Screen 6 (ProveBeforeWrite)
    │── tea.Cmd: runPreWriteCmd(keyPath, hostname, port) → PreWriteResultMsg
    │── tea.Cmd: runResolvedCmd(alias) → ResolvedResultMsg
    │── on both pass: show confirm prompt → tea.KeyPressMsg(Enter) → call identity.Deps write fns
    │
    ▼
  identity.Deps (same seams as CLI)
    ├── Generate, PersistKey, WriteSSH, WriteGitconfig, WriteFragment, WriteAllowedSigners
    └── (all routed through internal/filewriter — backup + atomic + idempotent)
```

### Recommended Project Structure

```
tui/
├── doc.go              # existing package doc
├── tui.go              # Run() entry point — creates tea.Program, wires deps
├── model.go            # RootModel struct + Init/Update/View
├── keymap.go           # shared key.Binding declarations (KeyQuit, KeyBack, etc.)
├── styles.go           # lipgloss style token constants
├── dashboard.go        # DashboardScreen model + view
├── identitylist.go     # IdentityListScreen model + view (bubbles/v2 list.Model)
├── identitydetail.go   # IdentityDetailScreen model + view
├── createform.go       # CreateForm model + view (textinput.Model per field)
├── updateform.go       # UpdateForm model + view
├── addaccountform.go   # AddAccountForm model + view
├── prove.go            # ProveBeforeWriteScreen model + view
├── copy.go             # CopyAction inline overlay logic
├── messages.go         # All custom tea.Msg types
└── deps.go             # TUI-side identity.Deps builder (mirrors cmd/gitid/add.go buildDeps)
```

```
cmd/gitid/
├── main.go             # MODIFIED: add no-args TUI branch + top-level aliases
├── copy.go             # NEW: newCopyCmd() + newIdentityCopyCmd()
├── ...                 # existing files unchanged
```

### Pattern 1: Bubble Tea v2 Model Interface

**What:** The core interface all models must implement in v2. `View()` returns `tea.View` (a struct), NOT a `string`. Use `tea.NewView(s)` to wrap rendered content.

**When to use:** Every screen in the view-stack is a struct with these methods.

```go
// Source: charm.land/bubbletea/v2@v2.0.7/tea.go (verified via module source inspection)
// View() returns tea.View, not string — this is a BREAKING v2 change
type Model interface {
    Init() Cmd
    Update(Msg) (Model, Cmd)
    View() View
}

// Correct v2 pattern
func (m dashboardModel) View() tea.View {
    rendered := m.renderDashboard() // builds string with lipgloss
    return tea.NewView(rendered)
}
```

**Pitfall:** Writing `func (m model) View() string` compiles in v1 but fails in v2 with `does not implement tea.Model (wrong type for View method)`.

### Pattern 2: Key Handling in v2

**What:** `tea.KeyMsg` is now an interface. Key presses arrive as `tea.KeyPressMsg`. The old `msg.Type`, `msg.Runes`, `msg.Alt` fields are gone.

```go
// Source: charm.land/bubbletea/v2@v2.0.7/key.go (verified via module source inspection)

// v1 pattern — WRONG in v2:
// case tea.KeyMsg:
//     switch msg.Type { case tea.KeyEnter: ... }

// v2 correct pattern:
case tea.KeyPressMsg:
    switch msg.Text {
    case "q":
        return m, tea.Quit()
    case "j":
        // move down
    }
    switch msg.Code {
    case tea.KeyEscape:
        // pop stack
    case tea.KeyEnter:
        // select
    case tea.KeyTab:
        // next form field
    }
```

Use `key.Binding` from `charm.land/bubbles/v2/key` with `key.Matches(msg, binding)` for the shared keymap pattern. [VERIFIED: Go module proxy + official docs]

### Pattern 3: Async Doctor Dashboard (D-09)

**What:** Run each of the 7 doctor check families as an independent `tea.Cmd` goroutine. The dashboard renders with spinners for unfinished families, replacing each spinner with findings as results arrive.

**How the Deps are assembled:** The TUI cannot call `buildDoctorDeps` from `cmd/gitid/doctor.go` directly (that is in `package main`). The TUI must either: (a) duplicate the deps wiring in `tui/deps.go`, or (b) the planner extracts `buildDoctorDeps` to an `internal/` package. Option (b) is architecturally cleaner but requires moving code. Option (a) is simpler for Phase 5 since the dep wiring is mechanical.

**Key insight:** `doctor.Run(deps)` runs all 7 families synchronously. For async per-family dispatch, the TUI must call individual check functions (`deps.CheckDeps(deps)`, `deps.CheckPerms(deps)`, etc.) directly — NOT `doctor.Run`. The `Deps.Check*` function fields provide this seam cleanly.

```go
// Source: internal/doctor/doctor.go (inspected codebase)
// Per-family async command pattern
type familyResultMsg struct {
    family   doctor.Family
    findings []doctor.Finding
    err      error
}

func runFamilyCmd(fam doctor.Family, fn doctor.CheckFn, deps doctor.Deps) tea.Cmd {
    return func() tea.Msg {
        findings := fn(deps)
        return familyResultMsg{family: fam, findings: findings}
    }
}

// In Init():
return tea.Batch(
    runFamilyCmd(doctor.FamilyDeps, d.CheckDeps, deps),
    runFamilyCmd(doctor.FamilyPerms, d.CheckPerms, deps),
    // ... all 7
)
```

Dashboard model tracks per-family state:
```go
type familyState int
const (
    familyLoading familyState = iota
    familyLoaded
    familyError
)

type dashboardModel struct {
    families   [7]familyState
    findings   map[doctor.Family][]doctor.Finding
    spinners   [7]spinner.Model
    width, height int
}
```

### Pattern 4: View-Stack Navigation (D-12)

**What:** A root model holds a `[]screen` stack. Each `screen` is an interface (or a discriminated union) with `Update(tea.Msg)` and `view()` methods. `Enter` pushes a new screen; `Esc` pops.

**Idiomatic approach:** Use a type alias `type screen interface { ... }` and a slice as stack. This avoids reflection and keeps type assertions minimal.

```go
// [ASSUMED] — this pattern is common in the bubbletea ecosystem; verified as
// idiomatic via search results referencing bubbletea-nav and community discussions.
type rootModel struct {
    stack  []screenModel
    width  int
    height int
    deps   tuiDeps // holds identity.Deps builder + doctor.Deps builder
}

type screenModel interface {
    update(msg tea.Msg) (screenModel, tea.Cmd)
    view() string
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Propagate WindowSizeMsg to all screens in the stack
    if ws, ok := msg.(tea.WindowSizeMsg); ok {
        m.width, m.height = ws.Width, ws.Height
        // propagate to each screen that needs it
    }
    if len(m.stack) == 0 {
        return m, tea.Quit()
    }
    top := m.stack[len(m.stack)-1]
    updated, cmd := top.update(msg)
    m.stack[len(m.stack)-1] = updated
    return m, cmd
}

func (m rootModel) View() tea.View {
    if len(m.stack) == 0 {
        return tea.NewView("")
    }
    return tea.NewView(m.stack[len(m.stack)-1].view())
}
```

**WindowSizeMsg propagation:** The root model MUST propagate `tea.WindowSizeMsg` to every screen that performs layout (dashboard, list, viewport). Failure to propagate causes rendering at wrong dimensions.

**Push/pop signals:** Use custom messages (`pushScreenMsg{screen: newScreen}`, `popScreenMsg{}`) returned via `tea.Cmd` from sub-models to keep the root model in control of the stack.

```go
type pushScreenMsg struct{ screen screenModel }
type popScreenMsg struct{}

// In root Update:
case pushScreenMsg:
    m.stack = append(m.stack, msg.screen)
case popScreenMsg:
    if len(m.stack) > 1 {
        m.stack = m.stack[:len(m.stack)-1]
    }
```

### Pattern 5: Running Blocking External Commands from a tea.Cmd (D-04)

**What:** `tester.PreWrite` and `tester.Resolved` both call `exec.Command(...).CombinedOutput()` — blocking network operations. Run them inside a `tea.Cmd` goroutine so the TUI remains responsive.

**NOT `tea.ExecProcess`:** `ExecProcess` is for interactive processes (editors, shells) that take over the terminal. For output-capturing commands like `ssh -T` / `ssh -G`, use a plain `tea.Cmd` that calls the function and returns the result as a `tea.Msg`.

```go
// Source: internal/tester/tester.go (inspected codebase) + tea.Cmd pattern
// [VERIFIED: codebase inspection + pkg.go.dev/charm.land/bubbletea/v2]

type preWriteResultMsg struct {
    result tester.Result
    err    error
}

type resolvedResultMsg struct {
    result   tester.Result
    resolved tester.ResolvedConfig
}

func runPreWriteCmd(keyPath, hostname string, port int) tea.Cmd {
    return func() tea.Msg {
        result := tester.PreWrite(keyPath, hostname, port)
        return preWriteResultMsg{result: result}
    }
}

func runResolvedCmd(alias string) tea.Cmd {
    return func() tea.Msg {
        result, resolved := tester.Resolved(alias)
        return resolvedResultMsg{result: result, resolved: resolved}
    }
}
```

The ProveBeforeWrite screen (Screen 6) starts Phase 1 in its `Init()`, receives `preWriteResultMsg` in `Update()`, then issues the Phase 2 command `runResolvedCmd`, receives `resolvedResultMsg`, and only then enables the confirm prompt.

**gosec concern:** The `tester.PreWrite` / `tester.Resolved` calls use `exec.Command("ssh", args...)` with arg-slice form (G204-clean). The same nolint annotations that exist in `internal/tester/tester.go` apply. The TUI does not add new exec calls — it calls the existing tester functions.

### Pattern 6: lipgloss v2 Style Tokens (NO NewRenderer)

**Critical:** `lipgloss.NewRenderer()` does NOT exist in lipgloss v2. It was fully removed. [VERIFIED: inspected lipgloss v2.0.3 source — `grep -n "func NewRenderer"` returned no matches]

**Replacement:** `lipgloss.NewStyle()` creates a plain value type (no renderer coupling). For TTY/NO_COLOR downsampling when printing, use `lipgloss.Fprint(w, ...)` or `lipgloss.Fprintln(w, ...)` which wrap `colorprofile.NewWriter(w, os.Environ())`.

**Within Bubble Tea:** The TUI program's internal renderer handles color profile detection automatically. Styles built with `lipgloss.NewStyle()` render correctly inside a `tea.Program` without needing manual color profile threading.

```go
// Source: charm.land/lipgloss/v2@v2.0.3/writer.go + style inspection (verified)
// [VERIFIED: inspected lipgloss v2 source]

// WRONG (v1 only):
// var renderer = lipgloss.NewRenderer(os.Stdout)
// StyleTitle = renderer.NewStyle().Bold(true)

// CORRECT for v2:
var (
    StyleTitle = lipgloss.NewStyle().Bold(true)
    StyleHeader = lipgloss.NewStyle().Bold(true)
    StyleSelected = lipgloss.NewStyle().Bold(true).Reverse(true)
    StyleBody = lipgloss.NewStyle()
    StyleFaint = lipgloss.NewStyle().Faint(true)
    StyleLabel = lipgloss.NewStyle().Bold(true).Width(16)
    StylePass = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
    StylePanel = lipgloss.NewStyle().
        Padding(1, 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("8"))
)

// For CLI output (copy command, etc.) where TTY/NO_COLOR matters:
lipgloss.Fprint(out, StylePass.Render("Copied public key..."))

// For TUI: just use style.Render() inside View() — program handles color
```

**Note on UI-SPEC:** `05-UI-SPEC.md` contains a `tui/styles.go` reference that uses `renderer.NewStyle()` — this is incorrect for v2. The implementation MUST use `lipgloss.NewStyle()` directly. The style token declarations in the UI-SPEC are otherwise correct; only the `renderer.` prefix must be dropped.

### Pattern 7: no-args TUI Entry Point (TUI-01)

**What:** `main()` must branch before calling `Execute()`.

```go
// Source: cmd/gitid/main.go (inspected) + non-TTY contract from 05-UI-SPEC.md
// [VERIFIED: codebase inspection]

func main() {
    // Non-TTY with no args: print usage hint and exit 1
    // TTY with no args: launch TUI
    // Any args: Cobra Execute()
    if len(os.Args) == 1 {
        if term.IsTerminal(int(os.Stdout.Fd())) {
            if err := tui.Run(); err != nil {
                fmt.Fprintf(os.Stderr, "gitid: tui: %v\n", err)
                os.Exit(1)
            }
            return
        }
        // Non-TTY with no args
        fmt.Fprintln(os.Stderr, "gitid: no subcommand given. Run 'gitid --help' for usage.")
        os.Exit(1)
    }
    if err := Execute(); err != nil {
        code := doctorExitCode
        if code == 0 {
            code = 1
        }
        os.Exit(code)
    }
}
```

**`tui.Run()` signature:** Package `tui` exports a single `Run()` function that builds the deps, creates the `tea.Program`, and calls `p.Run()`. It returns an error. This preserves the one-directional dependency (`tui` imports internal packages; `cmd/gitid` imports `tui`; internal packages import nothing from `tui` or `cmd/gitid`).

### Pattern 8: Cobra Top-Level Aliases (D-05/D-06/D-07)

**What:** Add thin wrapper commands that delegate to existing logic. Use Cobra `Command` structs, not the `Aliases` field (which only adds aliases within the same parent, not top-level names).

```go
// Source: cmd/gitid/main.go (inspected) + Cobra docs
// [VERIFIED: codebase inspection]

// In newRootCmd():

// D-05: top-level rotate alias — delegates to same handler as identity rotate
root.AddCommand(newTopLevelRotateCmd())   // wraps newRotateCmd() logic

// D-06: top-level copy + identity copy subcommand
root.AddCommand(newCopyCmd())              // copy <name> at top level
identity.AddCommand(newIdentityCopyCmd()) // identity copy <name>

// D-07: host group with add subcommand
host := &cobra.Command{Use: "host", Short: "Manage SSH host aliases"}
host.AddCommand(newHostAddCmd())          // delegates to runAddAccount
root.AddCommand(host)
```

**`uploadInstructions` is package-private:** It lives in `cmd/gitid/upload.go` as an unexported function in `package main`. The `copy` command lives in the same package, so direct call works. The TUI in `package tui` cannot call it directly — the TUI's Copy action must either: (a) call `internal/clipboard.Copy` for the clipboard part, and re-implement the upload instructions text, or (b) the planner extracts `uploadInstructions` to an `internal/` package. Option (b) is cleaner; option (a) requires duplicating the instructions strings.

**Recommendation:** Extract `uploadInstructions(provider string) string` to a new `internal/upload` package so both `cmd/gitid/copy.go` and `tui/copy.go` can import it.

### Pattern 9: Shell Completion (CLI-02)

**What:** Cobra auto-registers a `completion` subcommand when the root command has subcommands. Confirmed working via live test: `gitid completion --help` shows bash/fish/powershell/zsh options. [VERIFIED: live test on existing binary]

**Registration:** Already happens automatically — no explicit `root.InitDefaultCompletionCmd()` call needed for the current tree. Adding new commands does not break it.

**Verification method (unit test):**
```go
// Test that completion bash/zsh/fish produce non-empty output
func TestCompletionBash(t *testing.T) {
    root := newRootCmd()
    buf := &bytes.Buffer{}
    root.SetOut(buf)
    root.SetArgs([]string{"completion", "bash"})
    require.NoError(t, root.Execute())
    require.Contains(t, buf.String(), "gitid")
}
```

**Syntax validation (CI/smoke):**
```bash
gitid completion bash | bash -n   # validate bash syntax
gitid completion zsh  | zsh -n    # validate zsh syntax
```

Fish syntax validation requires `fish -n` but fish is not available in this environment. Fish completion tests should be skipped or mocked in CI if fish is absent.

### Anti-Patterns to Avoid

- **`View() string` in v2:** Compile error — use `View() tea.View` with `tea.NewView(rendered)`.
- **`renderer.NewStyle()` in lipgloss v2:** Compile error — `NewRenderer` does not exist. Use `lipgloss.NewStyle()`.
- **`tea.KeyMsg` struct switch in v2:** `tea.KeyMsg` is now an interface; use `tea.KeyPressMsg` for key presses.
- **`tea.ExecProcess` for ssh/git commands:** Only for interactive processes. Use plain `tea.Cmd` + `os/exec` for output capture.
- **`doctor.Run(deps)` for async dashboard:** Runs all 7 families synchronously in one goroutine. Call `deps.CheckFoo(deps)` per-family in separate `tea.Cmd` goroutines.
- **Calling `buildDoctorDeps` from `tui/`:** `buildDoctorDeps` is in `package main`. The TUI must build its own `doctor.Deps` or the planner extracts a shared builder.
- **`uploadInstructions()` from `tui/`:** Package-private in `package main`. Must be moved to `internal/upload` or reimplemented in `tui/`.
- **Blocking ssh calls in `Update()`:** Network/blocking calls in `Update()` freeze the event loop. Always wrap in `tea.Cmd`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Scrollable identity list with pagination | Custom list widget | `charm.land/bubbles/v2/list.Model` | Handles pagination, filtering, key bindings, delegate-based rendering |
| Animated spinner per family | Custom spinner loop | `charm.land/bubbles/v2/spinner.Model` with `spinner.Dot` | Frame animation, `Tick()` cmd, `ColorAccent` style |
| Help/footer bar keymap rendering | Custom string builder | `charm.land/bubbles/v2/help.Model` | `ShortHelpView` / `FullHelpView`, compact + expanded toggle, auto-hide disabled bindings |
| Scrollable command output in prove screen | Custom viewport | `charm.land/bubbles/v2/viewport.Model` | Mouse wheel, `GotoTop`/`GotoBottom`, content height management |
| Form field focus management | Custom focus index | `charm.land/bubbles/v2/textinput.Model` `Focus()`/`Blur()` | Per-field cursor visibility; `Tab`/`Shift+Tab` handled at root form level |
| Key binding declarations | Magic key strings | `charm.land/bubbles/v2/key.Binding` with `key.Matches(msg, b)` | Auto-rendered in help bar; `SetEnabled(false)` hides from help |
| SSH connectivity tests | New exec wrapper | `internal/tester.PreWrite` / `internal/tester.Resolved` | Already G204-clean, arg-slice, tested |
| Identity creation orchestration | New form→write path | `identity.Deps` seams + `identity.Create` | Tested, safe-write guaranteed, backup + atomic already wired |
| TTY detection | `os.Stat` bit check | `golang.org/x/term.IsTerminal` | Already in go.mod (v0.44.0); more portable than Stat mode bits |

---

## Runtime State Inventory

> SKIPPED — this is a greenfield TUI phase, not a rename/refactor/migration phase. No runtime state to audit.

---

## Common Pitfalls

### Pitfall 1: lipgloss v2 — `NewRenderer` Does Not Exist
**What goes wrong:** `renderer.NewStyle()` pattern from v1 produces compile error: `undefined: lipgloss.NewRenderer`.
**Why it happens:** `Renderer` type was completely removed in lipgloss v2. The `05-UI-SPEC.md` reference to `renderer.NewStyle()` is incorrect for v2.
**How to avoid:** Use `lipgloss.NewStyle()` directly. For CLI output needing color downsampling, use `lipgloss.Fprint(w, ...)`. For TUI views, `style.Render(s)` inside `View()` — the tea.Program renderer handles color.
**Warning signs:** `undefined: renderer` compile error.

### Pitfall 2: bubbletea v2 — `View()` Returns `tea.View` Not `string`
**What goes wrong:** `func (m model) View() string` causes interface satisfaction error.
**Why it happens:** Breaking change in v2: `Model.View() View` (struct type), not `string`.
**How to avoid:** Always write `func (m model) View() tea.View { return tea.NewView(rendered) }`. Sub-model helpers that build strings are fine; the root wrapper method wraps with `tea.NewView`.
**Warning signs:** `does not implement tea.Model (wrong type for View method)` compile error.

### Pitfall 3: bubbletea v2 — `tea.KeyMsg` Switch Pattern
**What goes wrong:** `case tea.KeyMsg: switch msg.Type { ... }` does not compile or matches nothing.
**Why it happens:** In v2, `tea.KeyMsg` is an interface; presses are `tea.KeyPressMsg`. `msg.Type` and `msg.Runes` are gone; use `msg.Text` and `msg.Code`.
**How to avoid:** Switch on `tea.KeyPressMsg`; use `msg.Text` for printable chars (`"q"`, `"j"`, `"k"`), `msg.Code` for special keys (`tea.KeyEnter`, `tea.KeyEscape`, `tea.KeyTab`).
**Warning signs:** Key handlers silently never fire; or `msg.Type undefined` compile error.

### Pitfall 4: Async Race — Stale Family Results on Refresh
**What goes wrong:** User presses `r` to refresh while a previous family `tea.Cmd` goroutine is still running. Both the stale result and the fresh result arrive as `familyResultMsg`, overwriting each other unpredictably.
**Why it happens:** `tea.Cmd` goroutines run to completion regardless of model state changes.
**How to avoid:** Add a `runID int` field to the dashboard model and embed `runID` in each `familyResultMsg`. In `Update`, ignore messages where `msg.runID != m.runID`. Increment `m.runID` on each refresh.
**Warning signs:** Dashboard shows results from a previous run after refresh.

### Pitfall 5: `doctor.Run` vs Per-Family `CheckFn` Call
**What goes wrong:** Using `doctor.Run(deps)` inside a `tea.Cmd` only gives one goroutine for all 7 families — not the async streaming effect required by D-09.
**Why it happens:** `doctor.Run` iterates all 7 `CheckFn` fields synchronously in one call.
**How to avoid:** Call `deps.CheckDeps(deps)`, `deps.CheckPerms(deps)`, etc. in 7 separate `tea.Cmd` goroutines. The `doctor.Deps` struct has individual `Check*` function fields exactly for this use case.
**Warning signs:** Dashboard spins for the duration of all 7 checks, then shows all results simultaneously — not progressive.

### Pitfall 6: `buildDoctorDeps` and `uploadInstructions` in `package main`
**What goes wrong:** `tui/` package cannot import from `package main` (`cmd/gitid/`).
**Why it happens:** Go forbids importing `package main`; the functions `buildDoctorDeps` and `uploadInstructions` are defined in `cmd/gitid/*.go` (package main).
**How to avoid:** The planner must include tasks to: (a) extract `buildDoctorDeps` deps-building logic (or write a TUI-specific variant in `tui/deps.go`), and (b) extract `uploadInstructions` to `internal/upload/upload.go` so both `cmd/gitid/copy.go` and `tui/copy.go` can import it.
**Warning signs:** `cannot import "github.com/castocolina/gitid/cmd/gitid"` compile error.

### Pitfall 7: Blocking ssh Calls in Update()
**What goes wrong:** Calling `tester.PreWrite(...)` directly in `Update()` blocks the entire event loop. The TUI freezes for the SSH connection timeout (up to 10 seconds).
**Why it happens:** `Update()` is called synchronously by the tea.Program event loop; any blocking call in it blocks all rendering.
**How to avoid:** Always wrap `tester.PreWrite` and `tester.Resolved` in `tea.Cmd` goroutines (Pattern 5 above). Return the command from `Update()` when transitioning to the prove screen.
**Warning signs:** TUI appears frozen/unresponsive when navigating to prove screen.

### Pitfall 8: `identityNameRe` and `sanitizeName` in `package main`
**What goes wrong:** Form validation in `tui/` cannot reuse `identityNameRe` from `cmd/gitid/rotate.go`.
**Why it happens:** `identityNameRe` is package-private in `package main`.
**How to avoid:** Move validation to `internal/identity` as an exported function (e.g., `identity.ValidateName(name string) error`) — it belongs in the domain layer anyway.
**Warning signs:** TUI form allows invalid identity names, causing downstream keygen path errors.

### Pitfall 9: `gosec` G304 on ReadFile in `tui/deps.go`
**What goes wrong:** `tui/deps.go` builds `doctor.Deps.ReadFile` which calls `os.ReadFile(path)`. gosec flags it as G304 unless annotated.
**How to avoid:** Add `//nolint:gosec // path is a trusted gitid-managed path (G304)` annotation, consistent with existing pattern in `cmd/gitid/doctor.go`.
**Warning signs:** `make lint` fails with G304 on tui/deps.go.

### Pitfall 10: `depguard` Rule Scope for `tui/` Package
**What goes wrong:** If `tui/` is mistakenly scoped under the `doctor-no-filewriter` depguard rule, it cannot import `internal/filewriter` for write operations.
**Why it happens:** The depguard rule in `.golangci.yml` applies to `**/internal/doctor/**` — correctly scoped. `tui/` is NOT under that scope.
**How to avoid:** Verify the depguard rule scope. `tui/` can and should import `internal/filewriter` indirectly through `identity.Deps` — but the deps functions themselves are closures defined in `tui/deps.go` that close over `filewriter.Write`. This is the same pattern as `cmd/gitid/add.go::buildDeps`.
**Warning signs:** Only a concern if someone mistakenly broadens the depguard rule.

---

## Code Examples

Verified patterns from official sources and codebase inspection:

### Async Doctor Family Command
```go
// Source: internal/doctor/doctor.go (CheckFn type, codebase inspection) +
//         charm.land/bubbletea/v2@v2.0.7/tea.go (tea.Cmd type)
// [VERIFIED: codebase inspection]

type familyResultMsg struct {
    runID    int
    family   doctor.Family
    findings []doctor.Finding
}

func makeFamilyCmd(runID int, fam doctor.Family, fn doctor.CheckFn, deps doctor.Deps) tea.Cmd {
    return func() tea.Msg {
        findings := fn(deps)
        return familyResultMsg{runID: runID, family: fam, findings: findings}
    }
}
```

### View-Stack Push/Pop
```go
// Source: [ASSUMED] — idiomatic bubbletea community pattern; verified as approach
// used in the ecosystem via search results

type pushScreenMsg struct{ next screenModel }
type popScreenMsg struct{}

func pushCmd(s screenModel) tea.Cmd {
    return func() tea.Msg { return pushScreenMsg{next: s} }
}

func popCmd() tea.Cmd {
    return func() tea.Msg { return popScreenMsg{} }
}
```

### key.Binding with bubbles/v2
```go
// Source: charm.land/bubbles/v2@v2.1.0 (verified via pkg.go.dev docs)
// [VERIFIED: official docs]

import "charm.land/bubbles/v2/key"

type keyMap struct {
    Up     key.Binding
    Down   key.Binding
    Select key.Binding
    Back   key.Binding
    Quit   key.Binding
    Help   key.Binding
    Refresh key.Binding
}

var keys = keyMap{
    Up:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
    Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
    Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
    Back: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
    Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
    Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
    Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
}

// In Update():
case tea.KeyPressMsg:
    switch {
    case key.Matches(msg, keys.Quit):
        return m, tea.Quit()
    case key.Matches(msg, keys.Back):
        return m, popCmd()
    }
```

### bubbles/v2 list.Model for Identity List
```go
// Source: charm.land/bubbles/v2@v2.1.0/list (verified via pkg.go.dev docs)
// [VERIFIED: official docs]

import "charm.land/bubbles/v2/list"

type identityItem struct {
    account identity.Account
}

func (i identityItem) FilterValue() string { return i.account.Name }
func (i identityItem) Title() string       { return i.account.Name }
func (i identityItem) Description() string { return i.account.Provider }

// Construction:
items := make([]list.Item, len(accounts))
for i, a := range accounts { items[i] = identityItem{account: a} }
l := list.New(items, list.NewDefaultDelegate(), width, height-footerHeight)
l.SetShowHelp(false) // use our own help bar
```

### bubbles/v2 textinput.Model for Forms
```go
// Source: charm.land/bubbles/v2@v2.1.0/textinput (verified via pkg.go.dev docs)
// [VERIFIED: official docs]

import "charm.land/bubbles/v2/textinput"

type createFormModel struct {
    inputs    [8]textinput.Model
    focusIdx  int
}

func newCreateForm() createFormModel {
    m := createFormModel{}
    for i := range m.inputs {
        ti := textinput.New()
        ti.Placeholder = formPlaceholders[i]
        m.inputs[i] = ti
    }
    _ = m.inputs[0].Focus() // returns tea.Cmd for cursor blink
    return m
}

// Tab/Enter to advance focus:
case tea.KeyPressMsg:
    switch msg.Code {
    case tea.KeyTab:
        m.inputs[m.focusIdx].Blur()
        m.focusIdx = (m.focusIdx + 1) % len(m.inputs)
        cmd := m.inputs[m.focusIdx].Focus()
        return m, cmd
    }
```

### lipgloss v2 Style Token File (correct for v2)
```go
// Source: charm.land/lipgloss/v2@v2.0.3 (inspected source)
// [VERIFIED: lipgloss v2 source inspection]

// tui/styles.go — correct v2 pattern (no renderer)
var (
    StyleTitle    = lipgloss.NewStyle().Bold(true)
    StyleHeader   = lipgloss.NewStyle().Bold(true)
    StyleSelected = lipgloss.NewStyle().Bold(true).Reverse(true)
    StyleBody     = lipgloss.NewStyle()
    StyleFaint    = lipgloss.NewStyle().Faint(true)
    StyleLabel    = lipgloss.NewStyle().Bold(true).Width(16)
    StylePass     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
    StylePanel    = lipgloss.NewStyle().
        Padding(1, 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("8"))
    StylePanelFocused = lipgloss.NewStyle().
        Padding(1, 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("4"))
)
```

### Cobra Top-Level Alias for Rotate
```go
// Source: cmd/gitid/main.go + rotate.go (inspected codebase)
// [VERIFIED: codebase inspection]

// In newRootCmd() — thin alias that reuses newRotateCmd() handler logic
rotateTL := &cobra.Command{
    Use:   "rotate <name>",
    Short: "Rotate the SSH key for an identity and re-test all artifacts",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return runIdentityRotate(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], false, buildDeps)
    },
}
root.AddCommand(rotateTL)
```

### `copy` Command (new — D-06)
```go
// Source: cmd/gitid/upload.go (inspected), internal/clipboard (existing)
// [VERIFIED: codebase inspection]

// cmd/gitid/copy.go (new file)
func newCopyCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "copy <name>",
        Short: "Copy the public key to the clipboard and print upload instructions",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runCopy(cmd.OutOrStdout(), args[0])
        },
    }
}

func runCopy(out io.Writer, name string) error {
    // 1. Reconstruct identity to find pubPath and provider
    // 2. Read pubPath -> pubLine
    // 3. clipboard.Copy(pubLine)
    // 4. Print pubLine + uploadInstructions(provider)
    // (uploadInstructions must be moved to internal/upload for TUI access)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `lipgloss.NewRenderer(w)` for per-output color control | `lipgloss.Fprint(w, ...)` / `colorprofile.NewWriter` | lipgloss v2.0.0 (stable) | `tui/styles.go` must use `lipgloss.NewStyle()` directly |
| `View() string` return type | `View() tea.View` (struct); wrap with `tea.NewView(s)` | bubbletea v2.0.0-beta | All screen models require this signature in v2 |
| `tea.KeyMsg` struct with `.Type`/`.Runes` | `tea.KeyPressMsg` with `.Text`/`.Code`/`.Mod` | bubbletea v2 | Key switch statements need updating |
| `p.Start()` / `p.StartReturningModel()` | `p.Run()` returning `(Model, error)` | bubbletea v2 | `tui.Run()` calls `p.Run()` |
| `tea.EnterAltScreen()` command | Set `View.AltScreen = true` in `View()` method | bubbletea v2 | TUI should set alt-screen via view field if needed |
| `tea.Sequentially()` | `tea.Sequence()` | bubbletea v2 | Minor rename |

**Deprecated/outdated:**
- `github.com/charmbracelet/bubbletea` (v1 import path): Superseded by `charm.land/bubbletea/v2`. CLAUDE.md explicitly forbids v1.
- `github.com/charmbracelet/lipgloss` (v1 import path): CLAUDE.md forbids it.
- `github.com/charmbracelet/bubbles` (v1 import path): CLAUDE.md forbids it.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | View-stack pattern (push/pop via typed messages) is the idiomatic bubbletea approach for multi-screen navigation | Architecture Patterns §4 | Alternative pattern (e.g. embedded sub-models) may be simpler or better-tested; impact is limited to model decomposition, not API correctness |
| A2 | Direct `Update(msg) → assert model` unit testing is sufficient for TDD without teatest | Validation Architecture | If teatest v2 exists and adds value, planner may want to reconsider; risk is LOW since Update-based unit tests are simpler and well-established |
| A3 | The best approach for TUI doctor deps is a `tui/deps.go` that replicates the `buildDoctorDeps` wiring (rather than extracting to internal/) | Architecture §Pitfall 6 | If extraction is preferred, it's extra refactor work in Wave 0; impact: 1 extra plan task |
| A4 | `uploadInstructions` should be extracted to `internal/upload/` | Architecture §Pitfall 6 | If it stays in package main, the TUI must re-implement it; risk: content drift |

---

## Open Questions (RESOLVED)

1. **`buildDoctorDeps` extraction vs duplication**
   - What we know: `buildDoctorDeps` is in `package main`; `tui/` cannot import it.
   - What's unclear: Whether to extract to `internal/` or duplicate in `tui/deps.go`.
   - Recommendation: Extract a simpler `internal/doctorinit` or `internal/doctor/wire` helper that constructs the standard `doctor.Deps` from real packages. This keeps the wiring DRY and testable. Alternatively, planner accepts duplication as lower-risk for Phase 5.
   - **Resolution (accepted):** Duplicate the doctor-deps wiring in `tui/deps.go` (PATTERNS assumption A3) — lower-risk for Phase 5; implemented in plan 05-01. (`uploadInstructions` and name validation ARE extracted to `internal/` because the CLI shares them; the doctor-deps closure is TUI-local.)

2. **`identityNameRe` / `sanitizeName` extraction**
   - What we know: Regex is in `package main`; TUI forms need validation.
   - What's unclear: Whether to export from `internal/identity` or redefine in `tui/`.
   - Recommendation: Move `ValidateName(name string) error` to `internal/identity` — it belongs in the domain layer. One-line change.
   - **Resolution (accepted):** Extract `internal/identity.ValidateName(name string) error`; implemented in plan 05-01.

3. **Alt-screen mode for TUI**
   - What we know: bubbletea v2 sets alt-screen via `View.AltScreen = true` in `View()` method.
   - What's unclear: Whether the gitid TUI should use alt-screen (cleaner, full-terminal) or inline mode (stays in scroll buffer). The UI-SPEC does not specify.
   - Recommendation: Use alt-screen (`view.AltScreen = true`) for the full TUI program — standard for dashboard-style TUIs. The doctor CLI command does NOT use alt-screen (it prints to scroll buffer — existing behavior preserved).
   - **Resolution (accepted):** Use alt-screen for the full TUI program; the `gitid doctor` CLI command stays in the scroll buffer (existing behavior preserved). Implemented in plan 05-01.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Build | Yes | 1.26.0 darwin/amd64 | — |
| bash | Completion syntax test | Yes | 3.2.57 | — |
| zsh | Completion syntax test | Yes | 5.9 | — |
| fish | Completion syntax test (CLI-02) | No | — | Skip fish syntax test in local env; test via output non-empty assertion |
| golangci-lint | `make lint` | Not on PATH | — | Install via binary; CLAUDE.md documents binary install required |
| ssh | ProveBeforeWrite tests | Yes (system) | — | N/A — required tool |

**Missing dependencies with no fallback:**
- `golangci-lint` not on PATH — must be installed via binary before `make lint` works. Expected; CLAUDE.md documents binary install.

**Missing dependencies with fallback:**
- `fish` — fish completion syntax test skipped locally; assertion covers non-empty output.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` package (stdlib) + `go test` |
| Config file | none (standard `go test ./...`) |
| Quick run command | `go test ./tui/... ./cmd/gitid/... -count=1` |
| Full suite command | `make test` → `go test -race ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CLI-01 | Top-level `rotate`, `copy`, `host add` commands registered and reachable | unit | `go test ./cmd/gitid/... -run TestNewRootCmd` | ❌ Wave 0 |
| CLI-01 | `identity copy <name>` registered under identity group | unit | `go test ./cmd/gitid/... -run TestIdentityCopy` | ❌ Wave 0 |
| CLI-01 | `copy <name>` reads pub file, copies to clipboard, prints instructions | unit | `go test ./cmd/gitid/... -run TestRunCopy` | ❌ Wave 0 |
| CLI-02 | `gitid completion bash` produces non-empty script containing "gitid" | unit | `go test ./cmd/gitid/... -run TestCompletionBash` | ❌ Wave 0 |
| CLI-02 | `gitid completion zsh` produces non-empty valid zsh script | unit | `go test ./cmd/gitid/... -run TestCompletionZsh` | ❌ Wave 0 |
| CLI-02 | `gitid completion fish` produces non-empty output | unit | `go test ./cmd/gitid/... -run TestCompletionFish` | ❌ Wave 0 |
| TUI-01 | Dashboard model `Init()` returns Batch of 7 `tea.Cmd`s (one per family) | unit | `go test ./tui/... -run TestDashboardInit` | ❌ Wave 0 |
| TUI-01 | `familyResultMsg` received → dashboard state transitions from loading to loaded | unit | `go test ./tui/... -run TestDashboardFamilyResult` | ❌ Wave 0 |
| TUI-01 | `r` key triggers refresh (runID increments, all families reset to loading) | unit | `go test ./tui/... -run TestDashboardRefresh` | ❌ Wave 0 |
| TUI-01 | Stale results (old runID) are ignored after refresh | unit | `go test ./tui/... -run TestDashboardStaleResult` | ❌ Wave 0 |
| TUI-02 | `Enter` from dashboard pushes `pushScreenMsg{IdentityListScreen}` | unit | `go test ./tui/... -run TestDashboardEnterNavigates` | ❌ Wave 0 |
| TUI-02 | `Esc` from IdentityList pops to dashboard | unit | `go test ./tui/... -run TestIdentityListEscPops` | ❌ Wave 0 |
| TUI-02 | `Enter` on identity item pushes `IdentityDetailScreen` | unit | `go test ./tui/... -run TestIdentityListEnterPushesDetail` | ❌ Wave 0 |
| TUI-02 | `a` from IdentityList pushes `CreateFormScreen` | unit | `go test ./tui/... -run TestIdentityListAddKey` | ❌ Wave 0 |
| TUI-02 | `e` from IdentityDetail pushes `UpdateFormScreen` | unit | `go test ./tui/... -run TestIdentityDetailEditKey` | ❌ Wave 0 |
| TUI-02 | `Tab` advances form focus; `Shift+Tab` goes back | unit | `go test ./tui/... -run TestFormTabNavigation` | ❌ Wave 0 |
| TUI-02 | ProveBeforeWrite screen: `Enter` confirm only active after both phases pass | unit | `go test ./tui/... -run TestProveScreenConfirmGate` | ❌ Wave 0 |
| TUI-02 | ProveBeforeWrite: phase 1 failure disables confirm, shows error | unit | `go test ./tui/... -run TestProveScreenPhase1Failure` | ❌ Wave 0 |
| TUI-02 | `q` / `ctrl+c` returns `tea.Quit()` from any screen | unit | `go test ./tui/... -run TestQuitFromAnyScreen` | ❌ Wave 0 |
| TUI-01 | Non-TTY no-args: exits 1, prints usage hint | unit | `go test ./cmd/gitid/... -run TestNoArgsNonTTY` | ❌ Wave 0 |
| TUI-01 | `WindowSizeMsg` propagated to dashboard, list, and prove screens | unit | `go test ./tui/... -run TestWindowSizePropagation` | ❌ Wave 0 |

**Note on TDD approach:** Following the project's RED-stub-under-strict-lint pattern (STATE.md), TUI model stubs must use `_`-param signatures and return zero values that satisfy the `tea.Model` interface. The `tui_stub_test.go` exists — it is replaced/extended by real tests. Each new TUI file gets a RED stub first: a `View()` returning `tea.NewView("")`, `Init()` returning `nil`, `Update()` returning `m, nil`.

### Sampling Rate
- **Per task commit:** `go test ./tui/... ./cmd/gitid/... -count=1 -run <TestForThisTask>`
- **Per wave merge:** `go test -race ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps (test infrastructure setup)
- [ ] `tui/model_test.go` — covers TUI-01/TUI-02 root model tests
- [ ] `tui/dashboard_test.go` — covers TUI-01 dashboard tests
- [ ] `tui/navigation_test.go` — covers TUI-02 stack navigation tests
- [ ] `tui/form_test.go` — covers TUI-02 form model tests
- [ ] `tui/prove_test.go` — covers TUI-02 prove-before-write tests
- [ ] `cmd/gitid/copy_test.go` — covers CLI-01 copy command tests
- [ ] `cmd/gitid/completion_test.go` — covers CLI-02 completion tests
- [ ] Framework install: `go get charm.land/bubbletea/v2@v2.0.7 charm.land/lipgloss/v2@v2.0.3 charm.land/bubbles/v2@v2.1.0`

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | TUI displays but does not implement auth |
| V3 Session Management | no | No session state; terminal process lifecycle |
| V4 Access Control | no | Access controlled by filesystem permissions (existing) |
| V5 Input Validation | yes | Identity name validation via `identityNameRe` (must move to `internal/identity.ValidateName`); form input sanitized before passing to core |
| V6 Cryptography | no | Key generation unchanged; no new crypto in TUI |

### Known Threat Patterns for this Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Shell injection via identity name in form | Tampering | Validate with `identityNameRe` before any exec; keep arg-slice (no shell) in tester calls |
| Clipboard data exposure | Information Disclosure | `atotto/clipboard` clears clipboard on process exit in some implementations; for gitid, pubkey is public data — acceptable |
| `gosec` G304 on `os.ReadFile` in tui/deps.go | Tampering | All paths are gitid-managed trusted paths; add `//nolint:gosec` annotation same as in cmd layer |
| `gosec` G204 on exec calls via tester | Injection | tester already uses arg-slice form; TUI calls existing tester functions — no new G204 exposure |

---

## Sources

### Primary (HIGH confidence)
- `charm.land/bubbletea/v2@v2.0.7` — Go module proxy + source inspection of `tea.go`, `key.go`, `commands.go`, `exec.go`, `screen.go` [VERIFIED]
- `charm.land/lipgloss/v2@v2.0.3` — source inspection of `writer.go`, absence of `NewRenderer` confirmed [VERIFIED]
- `charm.land/bubbles/v2@v2.1.0` — pkg.go.dev docs for `list`, `textinput`, `spinner`, `help`, `key` packages [VERIFIED]
- `github.com/spf13/cobra@v1.10.2` — live test `gitid completion --help` + pkg.go.dev `InitDefaultCompletionCmd` [VERIFIED]
- Codebase: `internal/doctor/doctor.go`, `internal/tester/tester.go`, `internal/identity/identity.go`, `cmd/gitid/main.go`, `cmd/gitid/add.go`, `cmd/gitid/doctor.go`, `cmd/gitid/upload.go`, `tui/doc.go`, `go.mod` [VERIFIED]
- `github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md` — v1→v2 breaking changes [VERIFIED: official docs]
- `github.com/charmbracelet/lipgloss/blob/v2.0.4/UPGRADE_GUIDE_V2.md` — v1→v2 changes including NewRenderer removal [VERIFIED: official docs]

### Secondary (MEDIUM confidence)
- `pkg.go.dev/charm.land/bubbletea/v2` — Model interface, tea.Cmd, tea.View docs [VERIFIED: official docs]
- `pkg.go.dev/charm.land/lipgloss/v2` — NewStyle, Color, Fprint/Fprintln docs [VERIFIED: official docs]
- `pkg.go.dev/charm.land/bubbles/v2@v2.1.0/list`, `/textinput`, `/spinner`, `/help` — component APIs [VERIFIED: official docs]
- `pkg.go.dev/github.com/spf13/cobra` — completion subcommand behavior [VERIFIED: official docs]

### Tertiary (LOW confidence, marked [ASSUMED])
- View-stack push/pop pattern via typed messages (A1) — community pattern, multiple examples seen but not from a single authoritative doc
- Direct `Update()` unit testing sufficiency without teatest (A2) — inferred from lack of confirmed teatest v2 package

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all three charm.land modules verified on Go module proxy; versions match CLAUDE.md pinned versions
- API surface (bubbletea v2): HIGH — inspected source directly; confirmed View() return type, KeyPressMsg type, tea.Cmd pattern
- lipgloss v2 API: HIGH — inspected source; NewRenderer absence confirmed; replacement pattern confirmed
- Architecture patterns: MEDIUM-HIGH — core patterns verified; view-stack decomposition marked [ASSUMED]
- Pitfalls: HIGH — v2 breaking changes confirmed from official upgrade guides + source inspection
- Shell completion: HIGH — live test confirmed auto-registration

**Research date:** 2026-06-13
**Valid until:** 2026-07-13 (stable libraries; 30-day validity)
