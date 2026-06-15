# Phase 4: Doctor - Pattern Map

**Mapped:** 2026-06-11
**Files analyzed:** 11 new/modified files
**Analogs found:** 10 / 11

---

## File Classification

| New / Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---------------------|------|-----------|----------------|---------------|
| `internal/doctor/doctor.go` | model + orchestrator | pure-data transform | `internal/identity/identity.go` | exact |
| `internal/doctor/checks/deps.go` | service | request-response (compose deps pkg) | `internal/deps/deps.go` | role-match |
| `internal/doctor/checks/perms.go` | service | request-response (os.Stat) | `internal/identity/update.go` (expandTilde / readPubLine) | partial |
| `internal/doctor/checks/coherence.go` | service | request-response (existence checks) | `internal/identity/loader.go` (Reconstruct) | role-match |
| `internal/doctor/checks/orphans.go` | service | request-response (ListBlocks) | `internal/filewriter/block.go` (ListBlocks) | role-match |
| `internal/doctor/checks/signing.go` | service | request-response + subprocess | `internal/deps/deps.go` (GitVersionAtLeast / exec.Command) | role-match |
| `internal/doctor/checks/baseline.go` | service | request-response | `internal/gitconfig/baseline.go` (ReadBaselineState) | exact |
| `cmd/gitid/doctor.go` | controller | request-response | `cmd/gitid/baseline.go` | exact |
| `cmd/gitid/doctor_test.go` | test | — | `cmd/gitid/baseline_test.go` | exact |
| `internal/doctor/doctor_test.go` | test | — | `cmd/gitid/baseline_test.go` + `internal/identity/identity.go` pattern | role-match |
| `internal/platform/platform.go` | utility (extend) | pure-data transform | self (extend existing switch) | self |

---

## Pattern Assignments

### `internal/doctor/doctor.go` (model + orchestrator, pure-data transform)

**Analog:** `internal/identity/identity.go`

**Imports pattern** (identity.go lines 12-19):
```go
import (
    "fmt"

    "github.com/castocolina/gitid/internal/gitconfig"
    "github.com/castocolina/gitid/internal/sshconfig"
    // doctor.go will import: os, identity, filewriter, gitconfig, sshconfig, deps, platform
)
```

**Deps struct pattern** (identity.go lines 119-148 — every external effect is an injected function field):
```go
// identity.Deps shape to mirror exactly — doctor.Deps follows the same convention.
type Deps struct {
    // Each injectable field: named function type, one per external effect.
    Generate   func(in CreateInput) (StagedKey, error)
    PersistKey func(s StagedKey) (KeyResult, error)
    Cleanup    func(s StagedKey)
    // ... and so on — every real I/O call is a field.
}
```

For `doctor.Deps`, the fields cover reads only (D-01: core never writes). Fix capabilities
(for auto-fixable findings) are injected as function fields (`FixPerm`, `RemoveBlock`,
`AddWiring`) rather than imported from filewriter, so `internal/doctor` never imports
`internal/filewriter`. This mirrors the identity.Deps injection pattern and keeps the
package write-free while remaining fully fake-testable.

**Core orchestrator pattern** (identity.go lines 190-196 and 200-284):
```go
// Create is the thin orchestrator — delegates all work to helpers, returns
// a result struct. Doctor mirrors this: Run delegates to per-family check funcs.
func Create(in CreateInput, deps Deps) (CreateResult, error) {
    staged, err := deps.Generate(in)
    if err != nil {
        return CreateResult{}, fmt.Errorf("identity: generating key: %w", err)
    }
    defer deps.Cleanup(staged)
    return runPipeline(in, staged, deps)
}
```

Doctor's Run function follows the same shape:
```go
func Run(deps Deps) []Finding {
    var all []Finding
    all = append(all, CheckDeps(deps)...)
    // ... one call per family, returns []Finding
    return all
}
```

**Error message format** (`add.go` line 287 — wrapping errors with package context):
```go
return fmt.Errorf("identity add: resolving home dir: %w", err)
// doctor.go format: "doctor: reading ~/.ssh/config: %w", err
```

**Incomplete / Finding field pattern** (identity.go lines 47-49):
```go
// Incomplete is non-empty when reconstruction found this identity name in
// some but not all four artifacts (D-02). It names the missing pieces.
Incomplete string
```
Doctor's `Finding` struct is the diagnostic generalization: `Family`, `Severity`,
`Title`, `Explanation`, `SuggestedFix`, optional `*FixDescriptor`.

---

### `internal/doctor/checks/deps.go` (service, request-response — compose deps pkg)

**Analog:** `internal/deps/deps.go`

**Imports pattern** (deps.go lines 1-7):
```go
package deps

import (
    "fmt"
    "os/exec"
    "strings"
)
```

**Core pattern — tool probing** (deps.go lines 36-40):
```go
// found reports whether a tool resolves on the current PATH.
func found(name string) bool {
    _, err := exec.LookPath(name)
    return err == nil
}
```

**Detect composition** (deps.go lines 78-86):
```go
func Detect() Report {
    return Report{
        SSH:       found("ssh"),
        SSHKeygen: found("ssh-keygen"),
        Git:       found("git"),
        SSHAdd:    found("ssh-add"),
        Clipboard: found("pbcopy") || found("wl-copy") || found("xclip") || found("xsel"),
    }
}
```

**Usage in doctor.CheckDeps:** call `deps.Detect()` and `report.MissingRequired()` via
the injected `deps.DetectTools` function field; call `deps.GitVersionAtLeast(2, 36)` via
the injected `deps.GitVersionAtLeast` field.

**MissingRequired** (deps.go lines 22-34):
```go
func (r Report) MissingRequired() []string {
    var missing []string
    if !r.SSH {
        missing = append(missing, "ssh")
    }
    // ... pattern: test each bool field, append name on false
    return missing
}
```

---

### `internal/doctor/checks/perms.go` (service, request-response — os.Stat)

**Analog:** `internal/identity/update.go` (readPubLine/expandTilde) and
`cmd/gitid/baseline.go` (snapshotFile)

**os.Stat pattern with gosec annotation** (baseline.go line 200):
```go
info, err := os.Stat(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
```

**Mode extraction** (use `info.Mode().Perm()` — the 9 permission bits as `os.FileMode`):
```go
got := info.Mode().Perm()
if got != want {
    // produce finding
}
```

**os.IsNotExist guard pattern** (gitconfig/reader.go line 72):
```go
if _, statErr := os.Stat(fragPath); os.IsNotExist(statErr) {
    return FragmentInfo{Missing: true}, nil
}
```

**FixDescriptor.Fn pattern for chmod fixes:**
The fix capability is injected into `doctor.Deps` as a function field (e.g.
`FixPerm func(path string, mode os.FileMode) error`). The check function closes over
the injected dep when building the `FixDescriptor.Fn` field — `internal/doctor` never
calls `os.Chmod` directly.

```go
// Injected fix capability closes over the injected dep, keeping doctor write-free.
Fix: &FixDescriptor{
    Summary: fmt.Sprintf("chmod %04o %s", want, path),
    Fn: func() error {
        return deps.FixPerm(path, want)   // calls the injected function
    },
},
```

**gosec G304 / G306 annotations required on every `os.Stat` / `os.Chmod` call:**
```go
info, _ := deps.Stat(keyPath) //nolint:gosec // keyPath is a trusted gitid-managed path (G304)
// For chmod in the cmd-layer fixer:
err := os.Chmod(path, mode)   //nolint:gosec // chmod to KEY-02 target modes (G306)
```

---

### `internal/doctor/checks/coherence.go` (service, request-response — existence checks)

**Analog:** `internal/identity/loader.go` (Reconstruct)

**Reconstruct call pattern** (loader.go lines 17-22):
```go
func Reconstruct(
    sshBytes []byte,
    gcBytes []byte,
    readFrag func(fragPath string) (gitconfig.FragmentInfo, error),
) ([]Account, error) {
    sshHosts, err := sshconfig.ParseManagedHosts(sshBytes)
    ...
    gcBlocks := gitconfig.ParseManagedIncludeIf(gcBytes)
```

Coherence check receives the reconstructed `[]identity.Account` via injected
`deps.Identities []identity.Account` (pre-built in cmd layer from `identity.Reconstruct`)
and checks each account's artifact paths with `deps.Stat`.

**Account field access pattern** (loader.go lines 40-75):
```go
if ssh, ok := sshHosts[name]; ok && ssh.Alias != "" {
    acct.KeyPath = ssh.IdentityFile
    acct.PubPath = ssh.IdentityFile + ".pub"
    // ...
} else {
    missing = append(missing, "ssh-host-block")
}
```

**Incomplete → Coherence family** (loader.go line 75 + identity.go lines 47-49):
```go
// account.Incomplete != "" means a managed block exists but a piece is absent.
// This maps to the Coherence family in doctor, NOT Orphans.
acct.Incomplete = strings.Join(missing, ",")
```

**ParseManagedIncludeIf** (gitconfig/reader.go lines 29-36):
```go
func ParseManagedIncludeIf(content []byte) map[string]IncludeIfInfo {
    blocks := filewriter.ListBlocks(content)
    result := make(map[string]IncludeIfInfo, len(blocks))
    for _, b := range blocks {
        result[b.Name] = parseIncludeIfBody(b.Body)
    }
    return result
}
```

---

### `internal/doctor/checks/orphans.go` (service, request-response — ListBlocks)

**Analog:** `internal/filewriter/block.go`

**ListBlocks call pattern** (block.go lines 18-50):
```go
func ListBlocks(content []byte) []NamedBlock {
    // Returns []NamedBlock{Name, Body} for every complete sentinel block.
    // Incomplete blocks (BEGIN without END) are silently skipped.
}
```

**Usage for orphan detection:**
```go
// In doctor/checks/orphans.go — via injected deps.ListSSHBlocks / deps.ListGitconfigBlocks
sshBlockNames := mapNames(deps.ListSSHBlocks(sshBytes))    // set of managed SSH block names
gcBlockNames  := mapNames(deps.ListGitconfigBlocks(gcBytes)) // set of managed gitconfig block names

// For each reconstructed account, cross-reference:
// artifact exists on disk && no block in either set claims it → Orphans family.
// account.Incomplete != "" && block exists → Coherence family.
```

**Sentinel constants** (block.go lines 99-108):
```go
const (
    BeginPrefix = "# BEGIN gitid managed: "
    EndPrefix   = "# END gitid managed: "
)
```

**RemoveBlock idempotency** (block.go lines 52-97):
```go
// RemoveBlock returns content with the named block removed; returns content
// unchanged (idempotent) when no such block exists.
func RemoveBlock(content []byte, name string) []byte { ... }
```

The orphan fixer routes through injected `deps.RemoveBlock` function fields, not
`filewriter.RemoveBlock` directly — same injection pattern as perms.

---

### `internal/doctor/checks/signing.go` (service, request-response + subprocess)

**Analog:** `internal/deps/deps.go` (GitVersionAtLeast pattern for subprocess invocation)

**GitVersionAtLeast pattern — exec.Command with fixed args** (deps.go lines 47-73):
```go
func GitVersionAtLeast(major, minor int) bool {
    cmd := exec.Command("git", "--version") //nolint:gosec // arg-slice form, no shell; fixed argument (G204)
    out, err := cmd.Output()
    if err != nil {
        return true // optimistic fallback
    }
    // parse vX.Y.Z from "git version X.Y.Z"
    ...
}
```

**ssh-add -l invocation pattern** (mirrors deps.go G204 annotation):
```go
// injected as deps.RunSSHAdd in doctor.Deps — real implementation:
func realRunSSHAdd() (string, int) {
    cmd := exec.Command("ssh-add", "-l") //nolint:gosec // fixed args, no user input (G204)
    out, err := cmd.Output()
    if err != nil {
        if ee, ok := err.(*exec.ExitError); ok {
            return string(out) + string(ee.Stderr), ee.ExitCode()
        }
        return "", 2
    }
    return string(out), 0
}
```

**Exit-code classification pattern** (pure function, no imports needed):
```go
// classifyAgentState maps (output, exitCode) → agentState.
// Uses both exit code AND text for portability across OpenSSH versions.
switch exitCode {
case 0:
    return agentRunningWithKeys
case 1:
    return agentRunningEmpty // exit 1 + "no identities" text = empty agent
default:
    return agentUnreachable  // exit 2 or exec error
}
```

**ssh-keygen -lf invocation** (same G204 annotation pattern):
```go
cmd := exec.Command("ssh-keygen", "-lf", pubKeyPath) //nolint:gosec // pubKeyPath is a gitid-managed path (G204)
```

**Fingerprint extraction helper** (pure function — testable without subprocess):
```go
// extractFingerprint parses the SHA256:... token from a ssh-keygen -lf output line.
func extractFingerprint(keygenLine string) string {
    for _, f := range strings.Fields(keygenLine) {
        if strings.HasPrefix(f, "SHA256:") {
            return f
        }
    }
    return ""
}
```

**D-20 git version gate** (deps.go lines 47-73 — inject as `deps.GitVersionAtLeast`):
```go
if !deps.GitVersionAtLeast(2, 36) {
    // produce warning finding for hasconfig: used with old git
}
```

---

### `internal/doctor/checks/baseline.go` (service, request-response)

**Analog:** `internal/gitconfig/baseline.go`

**ReadBaselineState call pattern** (referenced in RESEARCH.md and baseline.go):
```go
state, err := gitconfig.ReadBaselineState(absGitconfig, absBaseline, absGitignore)
// state.Installed       — baseline include + file both exist
// state.Incomplete      — some-but-not-all artifacts present
// state.BaselineKeys["core.ignorecase"] — for D-17 drift check (must equal "false")
// state.BaselineKeys["core.excludesfile"] — for excludesfile wiring check
// state.GitignorePatterns — for curated-excludes check
```

**Baseline check composition:**
All four D-16 checks compose `ReadBaselineState` via injected `deps.ReadBaselineState`
function field — no direct import of `internal/gitconfig` inside the check function
(consistent with injected-deps pattern).

---

### `cmd/gitid/doctor.go` (controller, request-response)

**Analog:** `cmd/gitid/baseline.go`

**Cobra command constructor pattern** (baseline.go lines 20-31):
```go
func newBaselineSetupCmd() *cobra.Command {
    var dryRun bool
    cmd := &cobra.Command{
        Use:   "setup",
        Short: "Seed the global baseline git config ...",
        RunE: func(cmd *cobra.Command, _ []string) error {
            return runBaselineSetup(cmd.InOrStdin(), cmd.OutOrStdout(), dryRun)
        },
    }
    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without writing anything (SAFE-03)")
    return cmd
}
```

For doctor:
```go
func newDoctorCmd() *cobra.Command {
    var fix, yes bool
    cmd := &cobra.Command{
        Use:   "doctor",
        Short: "Run a health check on the gitid-managed environment",
        RunE: func(cmd *cobra.Command, _ []string) error {
            return runDoctor(cmd.OutOrStdout(), fix, yes)
        },
    }
    cmd.Flags().BoolVar(&fix, "fix", false, "apply auto-fixable findings (per-finding confirm)")
    cmd.Flags().BoolVar(&yes, "yes", false, "apply all fixes without prompts (requires --fix; SAFE-03)")
    return cmd
}
```

**fp() helper pattern** (add.go lines 259-262):
```go
// fp writes s to out, ignoring the write error.
func fp(out io.Writer, s string) {
    _, _ = io.WriteString(out, s)
}
```
Reuse the existing `fp()` function from `add.go` (package-level in `main`).

**confirm() pattern — default N** (add.go lines 512-517):
```go
func confirm(r *bufio.Reader, out io.Writer, label string) bool {
    fp(out, fmt.Sprintf("%s [y/N]: ", label))
    line, _ := r.ReadString('\n')
    line = strings.ToLower(strings.TrimSpace(line))
    return line == "y" || line == "yes"
}
```

**promptYN() pattern — default Y** (baseline.go lines 445-455):
```go
func promptYN(r *bufio.Reader, out io.Writer, label string) bool {
    fp(out, fmt.Sprintf("%s [Y/n]: ", label))
    line, err := r.ReadString('\n')
    if err != nil && err != io.EOF {
        return false // fail safe
    }
    line = strings.ToLower(strings.TrimSpace(line))
    return line == "" || line == "y" || line == "yes"
}
```

**Family header pattern** (baseline.go `printBaselinePreview` uses `=== ... ===`):
```go
fp(out, "=== Preview: baseline setup ===\n")
// doctor renders: "=== Dependencies ===\n", "=== Permissions ===\n", etc.
```

**buildDeps / wiring pattern** (add.go lines 319-330):
```go
func buildDeps(_ io.Writer) identity.Deps {
    return identity.Deps{
        Generate: func(in identity.CreateInput) (identity.StagedKey, error) {
            // wire to real keygen package
        },
        // ... one field per injected dep, wired to real internal packages
    }
}
```
`cmd/gitid/doctor.go` must contain a `buildDoctorDeps()` function that wires
`doctor.Deps` from real internal packages: `os.Stat`, `os.ReadFile`, `identity.Reconstruct`,
`filewriter.ListBlocks`, `filewriter.RemoveBlock`, `gitconfig.ReadBaselineState`,
`gitconfig.ReadFragment`, `sshconfig.ParseManagedHosts`, `deps.Detect`,
`deps.GitVersionAtLeast`, `platform.CurrentOS`, `platform.InstallHint`.

**--yes without --fix guard** (add.go RunE error pattern):
```go
if yes && !fix {
    return fmt.Errorf("doctor: --yes requires --fix")
}
```

**TTY detection** (pure stdlib, zero new imports — RESEARCH.md Pattern 5):
```go
func isTerminalOutput(f *os.File) bool {
    if os.Getenv("NO_COLOR") != "" {
        return false
    }
    stat, err := f.Stat()
    if err != nil {
        return false
    }
    return (stat.Mode() & os.ModeCharDevice) != 0
}
```

**ANSI color helper** (UI-SPEC lines 141-159):
```go
func ansi(code, text string, colorEnabled bool) string {
    if !colorEnabled {
        return text
    }
    return "\033[" + code + "m" + text + "\033[0m"
}
```

**Home resolution + file read pattern** (list.go lines 36-56):
```go
home, err := os.UserHomeDir()
if err != nil {
    return fmt.Errorf("identity list: resolving home dir: %w", err)
}
sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // sshConfigPath is a gitid-managed path
if err != nil && !os.IsNotExist(err) {
    return fmt.Errorf("identity list: reading %s: %w", sshConfigPath, err)
}
```

**2-space indent list rendering** (list.go lines 78-109):
```go
fp(out, fmt.Sprintf("  key:      %s\n", acct.KeyPath))
// doctor uses same indent for passing checks and finding body lines:
// "  ✓ ssh present\n"
// "  ✗ title\n"
// "    explanation\n"
// "    fix: command\n"
```

**Incomplete marker pattern** (list.go line 107 — the `!` prefix convention):
```go
fp(out, fmt.Sprintf("  ! incomplete: missing %s\n", acct.Incomplete))
// doctor info findings use same glyph: "  ! pbcopy not found [info]\n"
```

---

### `cmd/gitid/doctor_test.go` (test — cmd layer)

**Analog:** `cmd/gitid/baseline_test.go`

**Test structure pattern** (baseline_test.go lines 17-56):
```go
func TestBaselineSetup_DryRun(t *testing.T) {
    tmpHome := t.TempDir()
    t.Setenv("HOME", tmpHome)

    var in strings.Reader
    var out bytes.Buffer

    err := runBaselineSetup(&in, &out, true /* dryRun */)
    if err != nil {
        t.Fatalf("runBaselineSetup returned error: %v", err)
    }

    if !strings.Contains(out.String(), "=== Preview: baseline setup ===") {
        t.Errorf("expected preview header; got:\n%s", out.String())
    }
}
```

Doctor tests follow the same shape: `t.TempDir()` + `t.Setenv("HOME", ...)`,
`strings.Reader` for stdin, `bytes.Buffer` for stdout, call the private `runDoctor`
function directly, assert output strings.

---

### `internal/doctor/doctor_test.go` (test — core package)

**Analog:** `internal/identity/identity.go` Deps pattern (unit tests with fake Deps)

Tests inject fake `doctor.Deps` with in-memory function stubs — no real filesystem,
no real subprocess. The pattern is table-driven with named test cases:

```go
func TestCheckDeps_MissingRequired(t *testing.T) {
    fakeDeps := doctor.Deps{
        DetectTools: func() deps.Report {
            return deps.Report{SSH: false, SSHKeygen: true, Git: true}
        },
        InstallHint: func(tool, os string) string { return "brew install " + tool },
        CurrentOS:   func() string { return "darwin" },
    }
    findings := CheckDeps(fakeDeps)
    // assert len, severity, family, title
}
```

`FixDescriptor.Fn` is tested with a recording stub:
```go
var called bool
fd := &doctor.FixDescriptor{
    Summary: "chmod 0600 /tmp/key",
    Fn:      func() error { called = true; return nil },
}
_ = fd.Fn()
if !called { t.Error("Fn not called") }
```

---

### `internal/platform/platform.go` (utility — extend existing switch)

**Analog:** self (extend existing `InstallHint` switch at lines 109-123)

**Current InstallHint pattern** (platform.go lines 109-123):
```go
func InstallHint(os string) string {
    const projectLink = "See https://www.openssh.com/ ..."
    switch os {
    case "darwin":
        return "Install or upgrade OpenSSH with Homebrew: `brew install openssh`.\n" + projectLink
    case "linux":
        return "Install or upgrade OpenSSH with your package manager:\n" +
            "  Debian/Ubuntu: `sudo apt install openssh-client`\n" +
            "  Fedora/RHEL:   `sudo dnf install openssh-clients`\n" +
            "  Arch:          `sudo pacman -S openssh`\n" +
            projectLink
    default:
        return "Install or upgrade OpenSSH for your platform.\n" + projectLink
    }
}
```

**Extension pattern** — add a `tool` parameter or add new per-tool hint functions
following the same switch-on-os structure. The UI-SPEC install-hint format (showing
only the current platform's hint when known, all four when unknown) is implemented
in `platform.go` since `CurrentOS()` is also there.

---

## Shared Patterns

### Injected-Deps Pattern
**Source:** `internal/identity/identity.go` lines 119-148
**Apply to:** `internal/doctor/doctor.go` (doctor.Deps struct definition)

Every external effect is a function field. Real implementations call the actual
packages; tests inject fakes. The cmd layer's `buildDoctorDeps()` wires real packages
into the function fields. This is the foundational pattern for all of Phase 4.

### fp() Print Helper
**Source:** `cmd/gitid/add.go` lines 259-262
**Apply to:** `cmd/gitid/doctor.go` — reuse the package-level `fp()` function as-is.

### confirm() and promptYN() Prompts
**Source:** `cmd/gitid/add.go` lines 512-517 (`confirm`, default N); `cmd/gitid/baseline.go` lines 445-455 (`promptYN`, default Y)
**Apply to:** `cmd/gitid/doctor.go` — reuse both unchanged.

### Error Message Format `pkg: context: err`
**Source:** `cmd/gitid/add.go` line 287: `"identity add: resolving home dir: %w"`
**Apply to:** all doctor error returns — use `"doctor: <context>: %w"` format.

### gosec Annotations
**Source:** `cmd/gitid/list.go` line 44; `cmd/gitid/update.go` line 130; `internal/deps/deps.go` line 48
**Apply to:** every `os.ReadFile`, `os.Stat`, `exec.Command` in new doctor code.

```go
os.ReadFile(path)   //nolint:gosec // path is a trusted gitid-managed path (G304)
exec.Command("ssh-add", "-l") //nolint:gosec // fixed args, no user input (G204)
os.Chmod(path, mode) //nolint:gosec // chmod to KEY-02 target mode (G306)
```

### Home Resolution + Safe File Read
**Source:** `cmd/gitid/list.go` lines 36-56
**Apply to:** `cmd/gitid/doctor.go` `runDoctor()` — same pattern: resolve home,
build absolute paths, `os.ReadFile` with `!os.IsNotExist` guard.

### `=== Name ===` Family Header
**Source:** `cmd/gitid/baseline.go` `printBaselinePreview` (uses `=== Preview: baseline setup ===`)
**Apply to:** `cmd/gitid/doctor.go` `renderReport()` — one header per family in fixed order.

### 2-Space Indent Line Convention
**Source:** `cmd/gitid/list.go` lines 79-109 (`"  key: %s\n"`); `cmd/gitid/baseline.go` (`"  ! message\n"`)
**Apply to:** all doctor output lines (passing checks, finding title, explanation at 4 spaces, fix at 4 spaces).

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/doctor/checks/` (subdirectory layout) | package | — | The nested `checks/` subdirectory is new; identity uses flat package layout. The RESEARCH.md recommended structure is the reference. |

---

## Metadata

**Analog search scope:** `internal/identity/`, `internal/deps/`, `internal/platform/`,
`internal/filewriter/`, `internal/gitconfig/`, `internal/sshconfig/`, `cmd/gitid/`

**Files scanned:** 18 source files + 6 test files

**Pattern extraction date:** 2026-06-11
