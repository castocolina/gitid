# Phase 5: CLI Surface + TUI - Pattern Map

**Mapped:** 2026-06-13
**Files analyzed:** 16 (14 new, 2 modified)
**Analogs found:** 14 / 16 (2 have no direct codebase analog — greenfield TUI models)

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/gitid/main.go` (modified) | CLI entry | request-response | self | exact |
| `cmd/gitid/copy.go` (new) | CLI command | request-response | `cmd/gitid/rotate.go` | exact |
| `tui/tui.go` | TUI entry | event-driven | `cmd/gitid/doctor.go` runDoctor() | role-match |
| `tui/model.go` | TUI root model | event-driven | none (greenfield) | no analog |
| `tui/keymap.go` | TUI config | — | none (greenfield) | no analog |
| `tui/styles.go` | TUI config | — | `cmd/gitid/doctor.go` ansi()/severityCode() | partial |
| `tui/messages.go` | TUI types | event-driven | `internal/doctor/doctor.go` Finding/Family types | partial |
| `tui/deps.go` | deps wiring | — | `cmd/gitid/add.go` buildDeps() | exact |
| `tui/dashboard.go` | TUI screen model | event-driven | `cmd/gitid/doctor.go` renderReport()/renderFinding() | role-match |
| `tui/identitylist.go` | TUI screen model | CRUD | `cmd/gitid/add.go` runAddAccount() | partial |
| `tui/identitydetail.go` | TUI screen model | request-response | `cmd/gitid/rotate.go` gatherRotateAccount() | partial |
| `tui/createform.go` | TUI form model | CRUD | `cmd/gitid/add.go` gatherCreateInput() | role-match |
| `tui/updateform.go` | TUI form model | CRUD | `cmd/gitid/update.go` | role-match |
| `tui/addaccountform.go` | TUI form model | CRUD | `cmd/gitid/add.go` runAddAccount()/gatherAddAccount() | role-match |
| `tui/prove.go` | TUI screen model | request-response | `cmd/gitid/add.go` printPreWrite()/printResolved() | role-match |
| `tui/copy.go` | TUI inline action | request-response | `cmd/gitid/upload.go` uploadInstructions() | role-match |
| `internal/upload/upload.go` (new) | utility | transform | `cmd/gitid/upload.go` uploadInstructions() | exact |
| `cmd/gitid/copy_test.go` (new) | test | — | `cmd/gitid/add_test.go` | exact |
| `cmd/gitid/completion_test.go` (new) | test | — | `cmd/gitid/doctor_test.go` TestDoctorCmdRegistered | exact |
| `tui/model_test.go` (new) | test | — | `cmd/gitid/add_test.go` fakeDeps pattern | role-match |
| `tui/dashboard_test.go` (new) | test | — | `cmd/gitid/doctor_test.go` | role-match |

---

## Pattern Assignments

### `cmd/gitid/main.go` (modified — CLI entry, no-args TUI branch + top-level aliases)

**Analog:** self (`cmd/gitid/main.go`)

**Current structure** (lines 1-67): `main()` → `Execute()` → `newRootCmd()` assembles the tree. Phase 5 modifies `main()` to branch on no-args + TTY, and extends `newRootCmd()` with three new top-level commands.

**No-args TUI branch to add** (modify main(), lines 12-24):
```go
func main() {
    if len(os.Args) == 1 {
        if term.IsTerminal(int(os.Stdout.Fd())) {
            if err := tui.Run(); err != nil {
                fmt.Fprintf(os.Stderr, "gitid: tui: %v\n", err)
                os.Exit(1)
            }
            return
        }
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

**New imports to add:**
```go
import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "golang.org/x/term"

    "github.com/castocolina/gitid/tui"
)
```

**Top-level aliases to add in newRootCmd()** (after existing adds, lines 54-65):
```go
// D-05: top-level rotate alias — delegates to same handler as identity rotate
rotateTL := &cobra.Command{
    Use:   "rotate <name>",
    Short: "Rotate the SSH key for an identity and re-test all artifacts",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return runIdentityRotate(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], false, buildDeps)
    },
}
root.AddCommand(rotateTL)

// D-06: top-level copy + identity copy subcommand
root.AddCommand(newCopyCmd())
identity.AddCommand(newIdentityCopyCmd())

// D-07: host group with add subcommand
host := &cobra.Command{Use: "host", Short: "Manage SSH host aliases"}
host.AddCommand(newHostAddCmd())
root.AddCommand(host)
```

---

### `cmd/gitid/copy.go` (new — CLI command, request-response)

**Analog:** `cmd/gitid/rotate.go`

**Imports pattern** (copy from rotate.go lines 1-16, adapt):
```go
package main

import (
    "fmt"
    "io"
    "os"

    "github.com/spf13/cobra"

    "github.com/castocolina/gitid/internal/clipboard"
    "github.com/castocolina/gitid/internal/identity"
    "github.com/castocolina/gitid/internal/upload"
)
```

**Cobra command pattern** (from rotate.go lines 34-46):
```go
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

func newIdentityCopyCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "copy <name>",
        Short: "Copy the public key to the clipboard and print upload instructions",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runCopy(cmd.OutOrStdout(), args[0])
        },
    }
}

func newHostAddCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "add",
        Short: "Add a host alias (SSH account) to an existing identity",
        RunE: func(cmd *cobra.Command, _ []string) error {
            return runIdentityAdd(cmd.InOrStdin(), cmd.OutOrStdout(), false, buildDeps)
            // NOTE: runIdentityAdd dispatches on mode; caller selects mode 3
            // interactively. The host add alias should pre-select modeAddAccount.
        },
    }
}
```

**Core handler pattern** (mirrors runIdentityRotate validation pattern from rotate.go lines 52-60):
```go
func runCopy(out io.Writer, name string) error {
    name = sanitizeName(name)   // reuse from rotate.go
    if name == "" {
        return fmt.Errorf("copy: identity name is required")
    }
    if !identityNameRe.MatchString(name) {  // reuse from rotate.go
        return fmt.Errorf("copy: invalid identity name %q", name)
    }
    // 1. Read SSH config + gitconfig to reconstruct account
    // 2. Read pubPath -> pubLine
    // 3. clipboard.Copy(pubLine)
    // 4. fp(out, ...) upload instructions from internal/upload.Instructions(provider)
    home, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("copy: resolving home dir: %w", err)
    }
    // reconstruct identity (same pattern as buildDoctorDeps identity.Reconstruct call)
    sshBytes, _ := os.ReadFile(filepath.Join(home, ".ssh", "config"))   //nolint:gosec // gitid-managed path (G304)
    gcBytes, _ := os.ReadFile(filepath.Join(home, ".gitconfig"))        //nolint:gosec // gitid-managed path (G304)
    accounts, _ := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
    // find by name, read .pub, copy, print
    ...
}
```

**Output pattern** (from add.go fp() pattern, lines 260-262):
```go
// fp() is already defined in add.go (same package main)
fp(out, fmt.Sprintf("Copied public key for %q to clipboard.\n", name))
fp(out, "Key: "+pubLine+"\n")
fp(out, "\n"+upload.Instructions(provider)+"\n")
```

---

### `internal/upload/upload.go` (new — extracted utility, transform)

**Analog:** `cmd/gitid/upload.go` — the entire file is the source. Extract as-is.

**Exact source to move** (`cmd/gitid/upload.go` lines 1-42):
```go
package upload

import (
    "fmt"
    "strings"
)

// Instructions returns the provider-specific steps for uploading a public key.
// Extracted from cmd/gitid/upload.go so both cmd/gitid/copy.go and tui/copy.go
// can import it without an import cycle.
func Instructions(provider string) string {
    switch strings.ToLower(provider) {
    case "github":
        var b strings.Builder
        b.WriteString("Upload your public key to GitHub (TWO separate registrations of the SAME key):\n")
        b.WriteString("  1. Open https://github.com/settings/ssh/new\n")
        b.WriteString("  2. Authentication key: paste the .pub, set \"Key type\" = Authentication key, Add SSH key.\n")
        b.WriteString("  3. Open https://github.com/settings/ssh/new again.\n")
        b.WriteString("  4. Signing key: paste the SAME .pub, set \"Key type\" = Signing key, Add SSH key.\n")
        b.WriteString("GitHub requires the key registered twice — once for authentication, once for signing.\n")
        return b.String()
    case "gitlab":
        var b strings.Builder
        b.WriteString("Upload your public key to GitLab (ONE key covers both roles):\n")
        b.WriteString("  1. Open https://gitlab.com/-/user_settings/ssh_keys\n")
        b.WriteString("  2. Paste the .pub, set \"Usage type\" = Authentication & Signing, Add key.\n")
        return b.String()
    default:
        return fmt.Sprintf(
            "Upload your public key to %s as both an authentication key and a signing key,\n"+
                "following that provider's SSH key settings page.\n", provider)
    }
}
```

Note: the original `uploadInstructions` in `cmd/gitid/upload.go` must be replaced with a call to `upload.Instructions(provider)` after extraction, or the file deleted and `cmd/gitid/add.go` updated to import `internal/upload`.

---

### `tui/tui.go` (new — TUI entry point, event-driven)

**Analog:** `cmd/gitid/doctor.go` `runDoctor()` (builds deps, orchestrates top-level flow)

**Pattern:** Single exported `Run()` function that builds deps, creates the program, calls `p.Run()`. Mirrors runDoctor's deps-building role.

**Imports pattern:**
```go
package tui

import (
    "fmt"

    tea "charm.land/bubbletea/v2"

    "github.com/castocolina/gitid/internal/doctor"
    "github.com/castocolina/gitid/internal/identity"
)
```

**Entry point pattern** (from RESEARCH.md Pattern 7):
```go
// Run launches the Bubble Tea TUI. It builds the doctor and identity deps,
// creates the root model, and runs the tea.Program. It returns an error on
// program failure; the caller (cmd/gitid/main.go) owns os.Exit.
func Run() error {
    doctorDeps, identityDeps, err := buildTUIDeps()
    if err != nil {
        return fmt.Errorf("tui: building deps: %w", err)
    }
    m := newRootModel(doctorDeps, identityDeps)
    p := tea.NewProgram(m, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        return fmt.Errorf("tui: program error: %w", err)
    }
    return nil
}
```

---

### `tui/deps.go` (new — deps wiring, mirrors buildDoctorDeps + buildDeps)

**Analog:** `cmd/gitid/add.go` `buildDeps()` (lines 319-425) AND `cmd/gitid/doctor.go` `buildDoctorDeps()` (lines 164-352)

**Pattern:** Builds both `doctor.Deps` and `identity.Deps` from real internal packages. The TUI cannot import `package main`, so it must replicate the wiring here. Use the same closure pattern as `buildDeps` in add.go.

**Imports pattern** (combines add.go and doctor.go imports, minus `cobra`):
```go
package tui

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"

    "github.com/castocolina/gitid/internal/clipboard"
    "github.com/castocolina/gitid/internal/deps"
    "github.com/castocolina/gitid/internal/doctor"
    "github.com/castocolina/gitid/internal/doctor/checks"
    "github.com/castocolina/gitid/internal/filewriter"
    "github.com/castocolina/gitid/internal/gitconfig"
    "github.com/castocolina/gitid/internal/identity"
    "github.com/castocolina/gitid/internal/keygen"
    "github.com/castocolina/gitid/internal/platform"
    "github.com/castocolina/gitid/internal/sshconfig"
    "github.com/castocolina/gitid/internal/tester"
)
```

**Core deps builder pattern** (copy structure from add.go buildDeps lines 319-425):
```go
// buildIdentityDeps wires identity.Deps from real internal packages.
// Copy from cmd/gitid/add.go buildDeps() verbatim, removing the io.Writer param
// (TUI does not use it for console output).
func buildIdentityDeps() identity.Deps {
    return identity.Deps{
        Generate:   ..., // same closure as add.go buildDeps Generate
        PersistKey: ..., // same closure as add.go buildDeps PersistKey
        Cleanup:    ..., // same closure as add.go buildDeps Cleanup
        CopyPub:    clipboard.Copy,
        PreWrite:   func(keyPath, hostname string, port int) tester.Result {
                        return tester.PreWrite(keyPath, hostname, port)
                    },
        WriteSSH:          ..., // same as add.go
        WriteGitconfig:    ..., // same as add.go
        WriteFragment:     ..., // same as add.go
        WriteAllowedSigners: keygen.WriteAllowedSigners,
        Resolved:          tester.Resolved,
        PubExists:         ..., // same as add.go
        DerivePub:         keygen.DerivePublicKey,
        WritePub:          ..., // same as add.go
    }
}
```

**Doctor deps builder pattern** (copy structure from doctor.go buildDoctorDeps lines 164-352):
```go
// buildTUIDoctorDeps constructs doctor.Deps for the TUI dashboard.
// Mirrors cmd/gitid/doctor.go buildDoctorDeps() — the TUI cannot import
// package main, so the wiring is replicated here.
// nolint:gosec // all paths are gitid-managed trusted paths (G304)
func buildTUIDoctorDeps(home string, sshBytes, gcBytes []byte) doctor.Deps {
    // Copy verbatim from doctor.go buildDoctorDeps, changing only the
    // function field assignments for ReadFile/Stat/FixPerm (same pattern).
    ...
    return doctor.Deps{
        ReadFile: func(path string) ([]byte, error) {
            return os.ReadFile(path) //nolint:gosec // trusted gitid-managed path (G304)
        },
        ...
        CheckPerms:     checks.CheckPermissions,
        CheckDeps:      checks.CheckDeps,
        CheckCoherence: checks.CheckCoherence,
        CheckOrphans:   checks.CheckOrphans,
        CheckSigning:   checks.CheckSigning,
        CheckAgent:     checks.CheckAgent,
        CheckBaseline:  checks.CheckBaseline,
    }
}
```

**gosec annotation pattern** (from doctor.go lines 83-93):
```go
// All os.ReadFile calls with gitid-managed paths must carry the same nolint:
// annotation used in doctor.go and add.go.
return os.ReadFile(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
```

---

### `tui/model.go` (new — root TUI model, event-driven)

**Analog:** No direct codebase analog. Use RESEARCH.md Pattern 4 (view-stack navigation).

**v2-correct interface pattern** (RESEARCH.md Pattern 1 — not in codebase yet):
```go
package tui

import tea "charm.land/bubbletea/v2"

type rootModel struct {
    stack  []screenModel
    width  int
    height int
    deps   tuiDeps
}

type screenModel interface {
    update(msg tea.Msg) (screenModel, tea.Cmd)
    view() string
}

// Init starts the async dashboard load. Returns Batch of 7 tea.Cmd (one per family).
func (m rootModel) Init() tea.Cmd {
    // see dashboard.go for the per-family cmd construction
    return nil // stub — dashboard.go implements the real Init
}

// Update delegates to the top of the stack; handles WindowSizeMsg and push/pop.
func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        // propagate to all screens that need it
    case pushScreenMsg:
        m.stack = append(m.stack, msg.next)
        return m, nil
    case popScreenMsg:
        if len(m.stack) > 1 {
            m.stack = m.stack[:len(m.stack)-1]
        }
        return m, nil
    }
    if len(m.stack) == 0 {
        return m, tea.Quit()
    }
    top := m.stack[len(m.stack)-1]
    updated, cmd := top.update(msg)
    m.stack[len(m.stack)-1] = updated
    return m, cmd
}

// View delegates rendering to the top screen.
// CRITICAL: returns tea.View (not string) — v2 breaking change.
func (m rootModel) View() tea.View {
    if len(m.stack) == 0 {
        return tea.NewView("")
    }
    return tea.NewView(m.stack[len(m.stack)-1].view())
}
```

**Push/pop message types** (RESEARCH.md Code Examples, also used by all screen models):
```go
// messages.go — these types belong here or in messages.go (shared)
type pushScreenMsg struct{ next screenModel }
type popScreenMsg struct{}

func pushCmd(s screenModel) tea.Cmd {
    return func() tea.Msg { return pushScreenMsg{next: s} }
}

func popCmd() tea.Cmd {
    return func() tea.Msg { return popScreenMsg{} }
}
```

---

### `tui/keymap.go` (new — shared key.Binding declarations)

**Analog:** No direct codebase analog. Use RESEARCH.md Pattern 2 and Code Examples.

**Pattern** (RESEARCH.md Code Examples — key.Binding with bubbles/v2):
```go
package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
    Up      key.Binding
    Down    key.Binding
    Left    key.Binding
    Right   key.Binding
    Select  key.Binding
    Back    key.Binding
    Quit    key.Binding
    Help    key.Binding
    Refresh key.Binding
    Add     key.Binding
    Edit    key.Binding
    Copy    key.Binding
    Delete  key.Binding
    Rotate  key.Binding
    AddHost key.Binding
    Next    key.Binding   // Tab
    Prev    key.Binding   // Shift+Tab
    Submit  key.Binding
    Confirm key.Binding
    Top     key.Binding
    Bottom  key.Binding
}

var keys = keyMap{
    Up:      key.NewBinding(key.WithKeys("up", "k"),          key.WithHelp("↑/k", "up")),
    Down:    key.NewBinding(key.WithKeys("down", "j"),        key.WithHelp("↓/j", "down")),
    Select:  key.NewBinding(key.WithKeys("enter"),            key.WithHelp("enter", "select")),
    Back:    key.NewBinding(key.WithKeys("esc"),              key.WithHelp("esc", "back")),
    Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"),      key.WithHelp("q", "quit")),
    Help:    key.NewBinding(key.WithKeys("?"),                key.WithHelp("?", "help")),
    Refresh: key.NewBinding(key.WithKeys("r"),                key.WithHelp("r", "refresh")),
    Add:     key.NewBinding(key.WithKeys("a"),                key.WithHelp("a", "add")),
    Edit:    key.NewBinding(key.WithKeys("e"),                key.WithHelp("e", "edit")),
    Copy:    key.NewBinding(key.WithKeys("c"),                key.WithHelp("c", "copy pubkey")),
    Delete:  key.NewBinding(key.WithKeys("d", "delete"),      key.WithHelp("d", "delete (CLI)")),
    Rotate:  key.NewBinding(key.WithKeys("r"),                key.WithHelp("r", "rotate (CLI)")),
    AddHost: key.NewBinding(key.WithKeys("h"),                key.WithHelp("h", "add host")),
    Next:    key.NewBinding(key.WithKeys("tab"),              key.WithHelp("tab", "next field")),
    Prev:    key.NewBinding(key.WithKeys("shift+tab"),        key.WithHelp("shift+tab", "prev field")),
    Submit:  key.NewBinding(key.WithKeys("enter"),            key.WithHelp("enter", "submit")),
    Confirm: key.NewBinding(key.WithKeys("enter"),            key.WithHelp("enter", "confirm write")),
    Top:     key.NewBinding(key.WithKeys("g"),                key.WithHelp("g", "top")),
    Bottom:  key.NewBinding(key.WithKeys("G"),                key.WithHelp("G", "bottom")),
}

// In Update() — key handling v2 pattern (RESEARCH.md Pattern 2):
// case tea.KeyPressMsg:
//     switch {
//     case key.Matches(msg, keys.Quit):
//         return m, tea.Quit()
//     case key.Matches(msg, keys.Back):
//         return m, popCmd()
//     }
```

---

### `tui/styles.go` (new — lipgloss v2 style tokens)

**Analog:** `cmd/gitid/doctor.go` `ansi()`/`severityCode()` (lines 454-471) — the color model is the same; the implementation switches from ANSI strings to lipgloss v2 style values.

**CRITICAL:** The `05-UI-SPEC.md` style block uses `renderer.NewStyle()` — this is v1-only and does NOT compile in lipgloss v2. Use `lipgloss.NewStyle()` directly (RESEARCH.md Pattern 6, Pitfall 1).

**Imports pattern:**
```go
package tui

import (
    lipgloss "charm.land/lipgloss/v2"

    "github.com/castocolina/gitid/internal/doctor"
)
```

**Style token declarations** (v2-correct — RESEARCH.md Pattern 6):
```go
// tui/styles.go — all lipgloss.NewStyle() calls; NO renderer.NewStyle()
var (
    StyleTitle    = lipgloss.NewStyle().Bold(true)
    StyleHeader   = lipgloss.NewStyle().Bold(true)
    StyleSelected = lipgloss.NewStyle().Bold(true).Reverse(true)
    StyleBody     = lipgloss.NewStyle()
    StyleFaint    = lipgloss.NewStyle().Faint(true)
    StyleLabel    = lipgloss.NewStyle().Bold(true).Width(16)

    StylePass    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
    StyleFinding = lipgloss.NewStyle() // color applied per-severity via SeverityStyle()

    StyleInputActive = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("4"))

    StyleInputInactive = lipgloss.NewStyle().
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("8"))

    StyleHelpKey  = lipgloss.NewStyle().Faint(true).Bold(true)
    StyleHelpDesc = lipgloss.NewStyle().Faint(true)

    StylePanel = lipgloss.NewStyle().
        Padding(1, 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("8"))

    StylePanelFocused = lipgloss.NewStyle().
        Padding(1, 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("4"))
)

// SeverityStyle maps doctor.Severity to a lipgloss foreground style.
// Mirrors severityCode() in cmd/gitid/doctor.go lines 462-471.
func SeverityStyle(s doctor.Severity) lipgloss.Style {
    switch s {
    case doctor.SeverityCritical, doctor.SeverityError:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
    case doctor.SeverityWarning:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
    default: // info
        return lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
    }
}
```

---

### `tui/messages.go` (new — custom tea.Msg types)

**Analog:** `internal/doctor/doctor.go` Finding/Family type definitions (lines 19-80) — the data shapes being wrapped in messages.

**Pattern** (RESEARCH.md Pattern 3 — async family result message):
```go
package tui

import (
    "github.com/castocolina/gitid/internal/doctor"
    "github.com/castocolina/gitid/internal/identity"
)

// familyResultMsg is the async result from one doctor check family (D-09).
// runID prevents stale results from a previous refresh overwriting fresh ones
// (RESEARCH.md Pitfall 4).
type familyResultMsg struct {
    runID    int
    family   doctor.Family
    findings []doctor.Finding
    err      error
}

// identityListResultMsg carries the reconstructed identity list.
type identityListResultMsg struct {
    accounts []identity.Account
    err      error
}

// preWriteResultMsg carries the SSH pre-write test result (Screen 6 Phase 1).
type preWriteResultMsg struct {
    result tester.Result
    err    error
}

// resolvedResultMsg carries the SSH resolved config test result (Screen 6 Phase 2).
type resolvedResultMsg struct {
    result   tester.Result
    resolved tester.ResolvedConfig
}

// writeResultMsg carries the outcome of an identity write operation.
type writeResultMsg struct {
    backupPath string
    err        error
}

// clipboardResultMsg carries the outcome of a clipboard copy operation.
type clipboardResultMsg struct {
    err error
}
```

---

### `tui/dashboard.go` (new — dashboard screen model, event-driven)

**Analog:** `cmd/gitid/doctor.go` `renderReport()`/`renderFinding()` (lines 357-451) — same finding data, lipgloss styling instead of ANSI strings.

**Async family cmd pattern** (RESEARCH.md Pattern 3 + Code Examples):
```go
package tui

import (
    tea "charm.land/bubbletea/v2"
    "charm.land/bubbles/v2/spinner"

    "github.com/castocolina/gitid/internal/doctor"
)

type familyState int
const (
    familyLoading familyState = iota
    familyLoaded
    familyError
)

type dashboardModel struct {
    families  [7]familyState
    findings  map[doctor.Family][]doctor.Finding
    spinners  [7]spinner.Model
    width, height int
    runID     int
    doctorDeps doctor.Deps
}

// init starts 7 independent tea.Cmd goroutines (D-09). Mirrors the Batch pattern
// from doctor.Run but dispatches each family separately.
func (m dashboardModel) init() (dashboardModel, tea.Cmd) {
    fams := doctor.Families()
    cmds := make([]tea.Cmd, len(fams))
    for i, fam := range fams {
        cmds[i] = makeFamilyCmd(m.runID, fam, m.doctorDeps)
    }
    return m, tea.Batch(cmds...)
}

func makeFamilyCmd(runID int, fam doctor.Family, d doctor.Deps) tea.Cmd {
    var fn doctor.CheckFn
    switch fam {
    case doctor.FamilyDeps:      fn = d.CheckDeps
    case doctor.FamilyPerms:     fn = d.CheckPerms
    case doctor.FamilyCoherence: fn = d.CheckCoherence
    case doctor.FamilyOrphans:   fn = d.CheckOrphans
    case doctor.FamilySigning:   fn = d.CheckSigning
    case doctor.FamilyAgent:     fn = d.CheckAgent
    case doctor.FamilyBaseline:  fn = d.CheckBaseline
    }
    return func() tea.Msg {
        findings := fn(d)
        return familyResultMsg{runID: runID, family: fam, findings: findings}
    }
}
```

**Finding render pattern** (lipgloss equivalent of doctor.go renderFinding lines 413-451):
```go
// renderFinding builds a styled string for one finding.
// Mirrors renderFinding() in cmd/gitid/doctor.go but uses lipgloss styles
// instead of raw ANSI codes.
func renderFinding(f doctor.Finding) string {
    glyph := "  ✗ "
    if f.Severity == doctor.SeverityInfo {
        glyph = "  ! "
    }
    severityStyle := SeverityStyle(f.Severity)
    titleLine := severityStyle.Render(glyph + f.Title)

    switch f.Severity {
    case doctor.SeverityCritical:
        titleLine += " [critical]"
    case doctor.SeverityWarning:
        titleLine += " [warning]"
    case doctor.SeverityInfo:
        titleLine += " [info]"
    }

    var s string
    s += titleLine + "\n"
    if f.Explanation != "" {
        s += StyleBody.PaddingLeft(4).Render(f.Explanation) + "\n"
    }
    if f.SuggestedFix != "" {
        s += StyleFaint.PaddingLeft(4).Render("fix: "+f.SuggestedFix) + "\n"
    }
    if f.Fix != nil {
        s += "    [fix]\n"
    }
    return s
}
```

**Key handling v2 pattern** (RESEARCH.md Pattern 2 — not `tea.KeyMsg`, use `tea.KeyPressMsg`):
```go
// In dashboardModel.update():
case tea.KeyPressMsg:
    switch {
    case key.Matches(msg, keys.Quit):
        return m, tea.Quit()
    case key.Matches(msg, keys.Refresh):
        m.runID++
        // reset all families to loading
        for i := range m.families {
            m.families[i] = familyLoading
        }
        m.findings = make(map[doctor.Family][]doctor.Finding)
        _, cmd := m.init()
        return m, cmd
    case key.Matches(msg, keys.Select):
        return m, pushCmd(newIdentityListScreen(m.identityDeps))
    }
```

---

### `tui/identitylist.go` (new — identity list screen, CRUD)

**Analog:** `cmd/gitid/add.go` `runAddAccount()` (lines 176-213) — same identity list reconstruction; bubbles/v2 list.Model for display.

**Bubbles list pattern** (RESEARCH.md Code Examples):
```go
package tui

import (
    "charm.land/bubbles/v2/list"
    tea "charm.land/bubbletea/v2"

    "github.com/castocolina/gitid/internal/identity"
)

type identityItem struct {
    account identity.Account
}

func (i identityItem) FilterValue() string { return i.account.Name }
func (i identityItem) Title() string       { return i.account.Name }
func (i identityItem) Description() string { return i.account.Provider }

type identityListModel struct {
    list        list.Model
    width, height int
    identityDeps tuiDeps
}
```

---

### `tui/createform.go` and `tui/updateform.go` and `tui/addaccountform.go` (new — form models)

**Analog:** `cmd/gitid/add.go` `gatherCreateInput()` (lines 268-313) and `gatherAddAccount()` (lines 216-255) — same fields, same order, same validation.

**Field list from analog** (gatherCreateInput lines 268-313): name, gitName, gitEmail, provider, alias, hostname, port, matchDir, passphrase.

**Field list from analog** (gatherAddAccount lines 216-255): name(existing), gitName, gitEmail, keyPath, newProvider, newAlias, hostname, port, matchDir.

**textinput pattern** (RESEARCH.md Code Examples):
```go
package tui

import (
    "charm.land/bubbles/v2/textinput"
    tea "charm.land/bubbletea/v2"
)

type createFormModel struct {
    inputs   [9]textinput.Model  // name, gitName, gitEmail, provider, alias, hostname, port, match, passphrase
    focusIdx int
    err      string  // validation error message for current field
}

func newCreateForm() createFormModel {
    m := createFormModel{}
    for i := range m.inputs {
        ti := textinput.New()
        ti.Placeholder = formPlaceholders[i]
        m.inputs[i] = ti
    }
    _ = m.inputs[0].Focus()
    return m
}

// Validation — mirrors gatherCreateInput name validation from add.go line 270-275:
// if !identityNameRe.MatchString(name) { ... }
// IMPORTANT: identityNameRe is in package main. The form must call
// identity.ValidateName() (to be extracted to internal/identity per RESEARCH.md
// Pitfall 8 and Open Question 2).
```

**Tab/Shift+Tab focus advance** (RESEARCH.md Code Examples):
```go
// In createFormModel.update():
case tea.KeyPressMsg:
    switch msg.Code {
    case tea.KeyTab:
        m.inputs[m.focusIdx].Blur()
        m.focusIdx = (m.focusIdx + 1) % len(m.inputs)
        cmd := m.inputs[m.focusIdx].Focus()
        return m, cmd
    case tea.KeyEscape:
        return m, popCmd()
    }
```

---

### `tui/prove.go` (new — prove-before-write screen, request-response)

**Analog:** `cmd/gitid/add.go` `printPreWrite()`/`printResolved()` (lines 459-481) — same two-phase test output, same data types. TUI version runs tests as `tea.Cmd` goroutines (RESEARCH.md Pattern 5).

**Async SSH test pattern** (RESEARCH.md Pattern 5):
```go
package tui

import (
    tea "charm.land/bubbletea/v2"

    "github.com/castocolina/gitid/internal/tester"
)

// runPreWriteCmd wraps tester.PreWrite as a tea.Cmd so it does not block Update().
// RESEARCH.md Pitfall 7: NEVER call tester.PreWrite() directly in Update().
func runPreWriteCmd(keyPath, hostname string, port int) tea.Cmd {
    return func() tea.Msg {
        result := tester.PreWrite(keyPath, hostname, port)
        return preWriteResultMsg{result: result}
    }
}

// runResolvedCmd wraps tester.Resolved as a tea.Cmd.
func runResolvedCmd(alias string) tea.Cmd {
    return func() tea.Msg {
        result, resolved := tester.Resolved(alias)
        return resolvedResultMsg{result: result, resolved: resolved}
    }
}
```

**Two-phase state machine** (mirrors printPreWrite + printResolved sequencing from add.go lines 459-481):
```go
type provePhase int
const (
    provePhase1Running provePhase = iota
    provePhase1Done
    provePhase2Running
    provePhase2Done
    provePhase1Failed
    provePhase2Failed
)

type proveModel struct {
    phase       provePhase
    phase1Result tester.Result
    phase2Result tester.Result
    phase2Resolved tester.ResolvedConfig
    action      string  // human-readable action description
    spinner     spinner.Model
    viewport    viewport.Model  // for scrollable command output
    confirmActive bool
}
```

---

### `tui/copy.go` (new — inline copy action)

**Analog:** `cmd/gitid/upload.go` `uploadInstructions()` (entire file) + `cmd/gitid/add.go` `printPubForManualCopy()` (line 483)

**Pattern:** Does not push a new screen. Returns a `clipboardResultMsg` from a `tea.Cmd`, then renders an inline overlay within the identity detail view.

```go
package tui

import (
    tea "charm.land/bubbletea/v2"

    "github.com/castocolina/gitid/internal/clipboard"
    "github.com/castocolina/gitid/internal/upload"
)

func runClipboardCopyCmd(pubLine string) tea.Cmd {
    return func() tea.Msg {
        err := clipboard.Copy(pubLine)
        return clipboardResultMsg{err: err}
    }
}

// renderCopyOverlay builds the inline copy confirmation block.
// Mirrors printPubForManualCopy (add.go line 483) + uploadInstructions() layout.
func renderCopyOverlay(pubLine, provider string, copyErr error) string {
    var s string
    if copyErr != nil {
        s += StyleFinding.Foreground(lipgloss.Color("6")).Render("! clipboard copy failed [info]") + "\n"
        s += StyleFaint.Render(copyErr.Error()) + "\n"
        s += StyleFaint.Render("Key is printed above — copy manually.") + "\n"
    } else {
        s += StylePass.Render("Public key copied to clipboard.") + "\n"
    }
    s += StyleFaint.Render("Key: "+truncatePubLine(pubLine)) + "\n\n"
    s += upload.Instructions(provider)     // from internal/upload (extracted)
    s += "\n" + StyleFaint.Render("Press any key to dismiss") + "\n"
    return s
}
```

---

### Test files: `cmd/gitid/copy_test.go` and `cmd/gitid/completion_test.go`

**Analog:** `cmd/gitid/add_test.go` (package main, table tests, `fakeDeps`, `bytes.Buffer` for output capture) and `cmd/gitid/doctor_test.go` (command registration test pattern).

**Command registration pattern** (from doctor_test.go lines 18-30):
```go
// TestCopyCmdRegistered — copy from TestDoctorCmdRegistered pattern
func TestCopyCmdRegistered(t *testing.T) {
    root := newRootCmd()
    found := false
    for _, cmd := range root.Commands() {
        if cmd.Use == "copy <name>" {
            found = true
            break
        }
    }
    if !found {
        t.Error("newRootCmd() does not have a top-level 'copy' command registered")
    }
}
```

**Completion test pattern** (RESEARCH.md Pattern 9):
```go
// TestCompletionBash — pattern from RESEARCH.md Pattern 9
func TestCompletionBash(t *testing.T) {
    root := newRootCmd()
    var buf bytes.Buffer
    root.SetOut(&buf)
    root.SetArgs([]string{"completion", "bash"})
    if err := root.Execute(); err != nil {
        t.Fatalf("completion bash: %v", err)
    }
    if !strings.Contains(buf.String(), "gitid") {
        t.Errorf("completion bash output does not contain 'gitid'; got:\n%s", buf.String())
    }
}
```

**runCopy test pattern** (from add_test.go `TestRunIdentityAddDryRunDoesNotPanic` structure, lines 93-130):
```go
// TestRunCopyNotFound — uses t.TempDir() for HOME isolation, bytes.Buffer for output
func TestRunCopyNotFound(t *testing.T) {
    t.Setenv("HOME", t.TempDir())
    var out bytes.Buffer
    err := runCopy(&out, "nonexistent")
    if err == nil {
        t.Fatal("runCopy with nonexistent identity must return error")
    }
}
```

---

### Test files: `tui/model_test.go`, `tui/dashboard_test.go`, `tui/navigation_test.go`, `tui/form_test.go`, `tui/prove_test.go`

**Analog:** `cmd/gitid/add_test.go` — the `fakeDeps` pattern (lines 17-51) is the primary idiom: inject fake function fields, call the handler with controlled input, assert on output/state.

**TUI fake deps pattern** (adapted from add_test.go fakeDeps):
```go
// tui/model_test.go — package tui
package tui

import (
    "testing"
    tea "charm.land/bubbletea/v2"
    "github.com/castocolina/gitid/internal/doctor"
    "github.com/castocolina/gitid/internal/tester"
)

// fakeDocDeps returns a doctor.Deps that returns no findings for all families.
// Mirrors fakeDeps in cmd/gitid/add_test.go — same pattern, different type.
func fakeDocDeps() doctor.Deps {
    noFindings := func(_ doctor.Deps) []doctor.Finding { return nil }
    return doctor.Deps{
        CheckDeps:      noFindings,
        CheckPerms:     noFindings,
        CheckCoherence: noFindings,
        CheckOrphans:   noFindings,
        CheckSigning:   noFindings,
        CheckAgent:     noFindings,
        CheckBaseline:  noFindings,
    }
}
```

**Direct Update() unit test pattern** (RESEARCH.md Validation Architecture — no teatest required):
```go
// TestDashboardFamilyResult — drive Update directly; assert model state
func TestDashboardFamilyResult(t *testing.T) {
    m := newDashboardModel(fakeDocDeps())

    // Simulate a familyResultMsg arriving
    msg := familyResultMsg{runID: m.runID, family: doctor.FamilyDeps, findings: nil}
    updated, _ := m.update(msg)

    dm := updated.(dashboardModel)
    if dm.families[0] != familyLoaded {
        t.Errorf("expected familyLoaded after familyResultMsg; got %v", dm.families[0])
    }
}
```

**RED stub under strict lint** (from MEMORY.md TDD-RED-stub convention):
```go
// Every new tui/*.go file starts with a RED stub that satisfies the interface
// without logic, so `make lint` passes before tests are green:
func (m dashboardModel) update(_ tea.Msg) (screenModel, tea.Cmd) { return m, nil }
func (m dashboardModel) view() string                             { return "" }
```

---

## Shared Patterns

### Auth / Guard (none applicable)
Not applicable — gitid has no user authentication layer.

### Error Handling
**Source:** `cmd/gitid/add.go` lines 50-78 (error propagation via `%w` wrapping)
**Apply to:** All new files in `cmd/gitid/` and `tui/`
```go
// Pattern: prefix error with package:function context, wrap with %w
return fmt.Errorf("copy: resolving home dir: %w", err)
return fmt.Errorf("tui: building deps: %w", err)
```

### Identity Name Validation
**Source:** `cmd/gitid/rotate.go` lines 20-28 (`identityNameRe`, `sanitizeName`)
**Apply to:** `cmd/gitid/copy.go` (can reuse directly — same package); `tui/` form models (MUST use extracted `identity.ValidateName()` — see RESEARCH.md Pitfall 8)
```go
// rotate.go lines 20-28 — reuse directly in copy.go (same package main)
var identityNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
func sanitizeName(name string) string { return strings.TrimSpace(name) }

// tui/ forms — cannot use identityNameRe (package main). Planner must add
// task: move ValidateName to internal/identity.
```

### gosec Annotations
**Source:** `cmd/gitid/doctor.go` lines 83-93, `cmd/gitid/add.go` line 342
**Apply to:** `tui/deps.go` all `os.ReadFile` calls with gitid-managed paths
```go
return os.ReadFile(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
```

### TTY / NO_COLOR Detection
**Source:** `cmd/gitid/doctor.go` `isTerminalOutput()` lines 588-597
**Apply to:** `cmd/gitid/main.go` (no-args TUI branch uses `golang.org/x/term.IsTerminal` — already in go.mod); `cmd/gitid/copy.go` for colored output
```go
// isTerminalOutput from doctor.go lines 588-597 — reuse for copy command output
// For the TUI branch in main.go — use term.IsTerminal (same as isTerminalInput):
if term.IsTerminal(int(os.Stdout.Fd())) { ... }
```

### Cobra Command Shape
**Source:** `cmd/gitid/rotate.go` `newRotateCmd()` lines 34-46
**Apply to:** `cmd/gitid/copy.go` `newCopyCmd()`, `newIdentityCopyCmd()`, `newHostAddCmd()`
```go
// Pattern: thin RunE that calls a separate run*() function; Args validation;
// no business logic in RunE body.
cmd := &cobra.Command{
    Use:   "...",
    Short: "...",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return runXxx(cmd.OutOrStdout(), args[0])
    },
}
```

### HOME isolation in tests
**Source:** `cmd/gitid/add_test.go` line 102; `cmd/gitid/doctor_test.go` line 36
**Apply to:** All new test files
```go
t.Setenv("HOME", t.TempDir())
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `tui/model.go` | TUI root model | event-driven | View-stack navigation pattern is greenfield; no existing Bubble Tea code in codebase |
| `tui/keymap.go` | TUI config | — | `key.Binding` declarations from bubbles/v2 are greenfield; library not yet imported |

Both files must use RESEARCH.md Patterns 2 and 4 (code examples marked [VERIFIED: official docs] and [ASSUMED]).

---

## Critical Implementation Notes for Planner

1. **`identityNameRe` / `sanitizeName` are in `package main`** — `tui/` forms cannot use them. Planner must add a Wave 0 task: extract `identity.ValidateName(name string) error` to `internal/identity` (RESEARCH.md Open Question 2, Pitfall 8).

2. **`uploadInstructions()` is in `package main`** — `tui/copy.go` cannot call it. Planner must add a Wave 0 task: extract to `internal/upload/upload.go` (RESEARCH.md Open Question, Pitfall 6). The pattern assignment for `internal/upload/upload.go` above documents the exact source to move.

3. **`buildDoctorDeps()` is in `package main`** — `tui/deps.go` must replicate the wiring independently (RESEARCH.md Pitfall 6, Assumption A3). The pattern for `tui/deps.go` above documents the structure.

4. **`View()` returns `tea.View`, not `string`** — every screen model's `view()` helper returns `string`; the root `rootModel.View()` wraps with `tea.NewView()`. Sub-model helpers return strings freely (RESEARCH.md Pitfall 2).

5. **Key press type is `tea.KeyPressMsg`** — NOT `tea.KeyMsg`. All `switch msg.(type)` blocks for keyboard must use `case tea.KeyPressMsg:` (RESEARCH.md Pitfall 3).

6. **`doctor.Run(deps)` is synchronous** — do NOT use it for the async dashboard. Call the individual `d.CheckDeps(d)`, `d.CheckPerms(d)`, etc. function fields in 7 separate `tea.Cmd` goroutines (RESEARCH.md Pitfall 5).

---

## Metadata

**Analog search scope:** `cmd/gitid/`, `internal/doctor/`, `tui/`
**Files read:** `cmd/gitid/main.go`, `cmd/gitid/add.go`, `cmd/gitid/rotate.go`, `cmd/gitid/doctor.go`, `cmd/gitid/upload.go`, `cmd/gitid/add_test.go`, `cmd/gitid/doctor_test.go`, `tui/doc.go`, `tui/tui_stub_test.go`, `internal/doctor/doctor.go` (partial)
**Pattern extraction date:** 2026-06-13
