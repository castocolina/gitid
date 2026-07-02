# Phase 3: Full Identity CRUD + Multi-Identity — Pattern Map

**Mapped:** 2026-06-10
**Files analyzed:** 13 new/modified files
**Analogs found:** 13 / 13

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/filewriter/block.go` (extend) | utility | transform | same file — `ReplaceBlock` | exact |
| `internal/filewriter/filewriter.go` (extend) | utility | file-I/O | same file — `Write` + `copyFile` | exact |
| `internal/sshconfig/reader.go` (new) | service | transform | `internal/sshconfig/parser.go` + `internal/filewriter/block.go` | role-match |
| `internal/gitconfig/reader.go` (new) | service | file-I/O | `internal/gitconfig/fragment.go` (`gitConfigSet`, `WriteFragment`) | exact |
| `internal/identity/loader.go` (new) | service | transform | `internal/identity/identity.go` (`runPipeline`) + `modes.go` | role-match |
| `internal/identity/update.go` (new) | service | request-response | `internal/identity/modes.go` (`Rotate`, `rotateInput`) | exact |
| `internal/identity/delete.go` (new) | service | request-response | `internal/identity/modes.go` (`Reuse`, `AddAccount`) | exact |
| `internal/identity/identity.go` (extend) | model | — | same file — `Account` struct | exact |
| `cmd/gitid/list.go` (new) | controller | request-response | `cmd/gitid/add.go` (`newAddCmd`, `runIdentityAdd`) | exact |
| `cmd/gitid/update.go` (new) | controller | request-response | `cmd/gitid/add.go` + `cmd/gitid/rotate.go` | exact |
| `cmd/gitid/delete.go` (new) | controller | request-response | `cmd/gitid/rotate.go` + `cmd/gitid/add.go` | exact |
| `internal/filewriter/block_list_test.go` (new) | test | — | `internal/filewriter/block.go` (test-first) | exact |
| `internal/sshconfig/coexistence_test.go` (new) | test | — | `internal/tester/tester.go` (`ParseResolved`) | exact |

---

## Pattern Assignments

---

### `internal/filewriter/block.go` — extend with `ListBlocks` + `RemoveBlock`

**Analog:** `internal/filewriter/block.go` lines 1–70 (same file — `ReplaceBlock`)

**Package declaration + imports** (lines 1–3):
```go
package filewriter

import "strings"
```
`ListBlocks` also needs `"bytes"`. Add it to the import block.

**Sentinel constants to reuse** (lines 11–14):
```go
const (
    BeginPrefix = "# BEGIN gitid managed: "
    EndPrefix   = "# END gitid managed: "
)
```
Both new functions use these exact constants — do not redefine them.

**Core pattern to mirror — `ReplaceBlock` line scan** (lines 38–52):
```go
lines := strings.SplitAfter(string(existing), "\n")

beginIdx, endIdx := -1, -1
for i, line := range lines {
    trimmed := strings.TrimRight(line, "\n")
    switch {
    case beginIdx == -1 && trimmed == beginMarker:
        beginIdx = i
    case beginIdx != -1 && trimmed == endMarker:
        endIdx = i
    }
    if beginIdx != -1 && endIdx != -1 {
        break
    }
}
```
`RemoveBlock` copies this **exactly** (same `beginMarker`/`endMarker` local vars, same `SplitAfter`, same guard pattern). `ListBlocks` uses the same scan but does NOT `break` at first match — it resets and continues to collect all blocks.

**Core pattern to mirror — `ReplaceBlock` splice** (lines 65–69):
```go
var b strings.Builder
b.WriteString(strings.Join(lines[:beginIdx], ""))
b.WriteString(block)
b.WriteString(strings.Join(lines[endIdx+1:], ""))
return []byte(b.String())
```
`RemoveBlock` splices `lines[:beginIdx]` + `lines[afterEnd:]` (skipping begin through end+optional blank). `ListBlocks` does NOT splice — it collects `strings.Join(lines[beginIdx+1:i], "")` as `Body`.

**`ListBlocks` signature:**
```go
// NamedBlock is one sentinel-delimited block extracted from a file.
type NamedBlock struct {
    Name string // the <name> token from "# BEGIN gitid managed: <name>"
    Body string // lines between (exclusive of) the sentinel markers
}

// ListBlocks scans content for all complete gitid managed blocks and returns
// them in file order. Incomplete blocks (BEGIN with no matching END) are
// silently skipped. CRLF is normalised to LF before scanning.
func ListBlocks(content []byte) []NamedBlock
```

**`RemoveBlock` signature:**
```go
// RemoveBlock returns content with the gitid managed block for name removed.
// If no such block exists the input is returned unchanged (idempotent). A single
// blank line immediately following the END marker is also consumed to avoid
// blank-line accumulation on repeated delete+recreate cycles.
func RemoveBlock(content []byte, name string) []byte
```

---

### `internal/filewriter/filewriter.go` — extend with `BackupAndRemove`

**Analog:** `internal/filewriter/filewriter.go` lines 33–85 (`Write`)

**Backup step to mirror** (lines 35–39):
```go
if _, statErr := os.Stat(targetPath); statErr == nil {
    backupPath = targetPath + ".bak." + time.Now().Format("20060102-150405")
    if copyErr := copyFile(targetPath, backupPath, backupMode); copyErr != nil {
        return "", fmt.Errorf("backing up %s: %w", targetPath, copyErr)
    }
}
```
`BackupAndRemove` uses `os.Rename(path, backupPath)` instead of `copyFile` — rename is atomic (backup AND removal in one syscall). The timestamp format `"20060102-150405"` is identical to what `Write` uses.

**`BackupAndRemove` signature:**
```go
// BackupAndRemove creates a timestamped backup of path (same naming convention
// as Write) and removes the original via atomic rename. Used for whole-file
// deletion where content replacement does not apply (fragment file delete,
// IDENT-05 D-08). If path does not exist, returns ("", nil) — idempotent.
func BackupAndRemove(path string) (backupPath string, err error)
```

**Imports to add** (already present in the file — no change needed): `"os"`, `"time"`.

---

### `internal/sshconfig/reader.go` (new file)

**Analog 1:** `internal/sshconfig/parser.go` lines 1–23 (package declaration, `ssh_config.Decode` usage)

**Analog 2:** `internal/filewriter/block.go` lines 1–14 (`ListBlocks` — called here)

**Package + imports pattern** (mirror `parser.go`):
```go
package sshconfig

import (
    "fmt"
    "strconv"
    "strings"

    ssh_config "github.com/kevinburke/ssh_config"

    "github.com/castocolina/gitid/internal/filewriter"
)
```

**`Parse` pattern to mirror — `ssh_config.Decode` call** (`parser.go` lines 17–23):
```go
cfg, err := ssh_config.Decode(bytes.NewReader(content))
if err != nil {
    return nil, fmt.Errorf("parsing ssh config: %w", err)
}
```
`parseHostBlockBody` replaces `bytes.NewReader(content)` with `strings.NewReader(body)`.

**Implicit Host* guard pattern** (from RESEARCH.md, verified against kevinburke source):
```go
for _, host := range cfg.Hosts {
    if len(host.Patterns) == 1 && host.Patterns[0].String() == "*" {
        continue // implicit Host * inserted by newConfig() — skip
    }
    // real managed Host block
}
```
This guard is **mandatory** in `ParseManagedHosts` — the library always inserts an implicit `Host *` as `cfg.Hosts[0]`.

**Key API calls** (`cfg.Get` — first-match lookup):
```go
hostname, _ := cfg.Get(alias, "Hostname")
portStr, _  := cfg.Get(alias, "Port")
identityFile, _ := cfg.Get(alias, "IdentityFile")
identitiesOnly, _ := cfg.Get(alias, "IdentitiesOnly")
```

**Types to define:**
```go
type SSHHostInfo struct {
    Alias          string
    Hostname       string
    Port           int      // default 22 when absent
    IdentityFile   string
    IdentitiesOnly bool
}
```

**`ParseManagedHosts` signature:**
```go
// ParseManagedHosts parses content (bytes of ~/.ssh/config), extracts all
// gitid-managed blocks via filewriter.ListBlocks, and for each block parses the
// SSH directives into SSHHostInfo. Keyed by identity name (D-01). Blocks that
// fail to parse return a zero-value SSHHostInfo (reconstruction incomplete
// marker, D-02).
func ParseManagedHosts(content []byte) (map[string]SSHHostInfo, error)
```

---

### `internal/gitconfig/reader.go` (new file)

**Analog:** `internal/gitconfig/fragment.go` — `WriteFragment` (lines 30–58) and `gitConfigSet` (lines 74–80)

**Package + imports pattern** (mirror `fragment.go` imports):
```go
package gitconfig

import (
    "fmt"
    "os"
    "os/exec"
    "strings"

    "github.com/castocolina/gitid/internal/filewriter"
)
```

**`git config --file` exec pattern** (`fragment.go` lines 74–80):
```go
func gitConfigSet(path, key, value string) error {
    cmd := exec.Command("git", "config", "--file", path, key, value) //nolint:gosec // arg-slice form, no shell; values validated above (G204)
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("git config --file %s %s: %w: %s", path, key, err, strings.TrimSpace(string(out)))
    }
    return nil
}
```
`ReadFragment` uses the symmetric `--list` form:
```go
cmd := exec.Command("git", "config", "--file", fragPath, "--list") //nolint:gosec
out, err := cmd.Output()
```
Same arg-slice pattern, same `//nolint:gosec` annotation, no shell expansion.

**Key/value parse pattern** (output of `git config --list` is `key=value\n` lines):
```go
for _, line := range strings.Split(string(out), "\n") {
    kv := strings.SplitN(line, "=", 2)
    if len(kv) != 2 {
        continue
    }
    switch strings.ToLower(kv[0]) {
    case "user.name":  info.GitName = kv[1]
    case "user.email": info.GitEmail = kv[1]
    // …
    }
}
```

**`filewriter.Write` for `RemoveAllowedSignersLine`** (mirror the `Write` call pattern from `add.go` line 342):
```go
return filewriter.Write(path, []byte(result), 0o600)
```
`allowed_signers` uses mode `0o600` (same as `Write`'s `backupMode` — private material).

**Types to define:**
```go
type FragmentInfo struct {
    GitName    string
    GitEmail   string
    SigningKey  string
    GPGFormat  string
    CommitSign bool
    Missing    bool
}

type IncludeIfInfo struct {
    FragmentPath string
    Matches      []Match
}
```

**Signatures:**
```go
func ParseManagedIncludeIf(content []byte) map[string]IncludeIfInfo

func ReadFragment(fragPath string) (FragmentInfo, error)

// RemoveAllowedSignersLine rewrites path with the line for identityEmail
// removed (matched by BOTH identityEmail AND namespaces="git"). Backs up via
// filewriter.Write. Idempotent when no matching line exists.
func RemoveAllowedSignersLine(path, identityEmail string) (backupPath string, err error)
```

---

### `internal/identity/identity.go` — extend `Account` struct

**Analog:** same file, `Account` struct lines 25–44.

**Field to add** (additive — no existing field changes):
```go
// Incomplete is non-empty when reconstruction found this identity name in some
// but not all four artifacts. It names the missing pieces (comma-separated) for
// display in `gitid identity list`. Deep diagnosis stays in Phase 4 doctor.
Incomplete string
```
Place after the last existing field (`AllowedSignersPath`). No other change to this file.

---

### `internal/identity/loader.go` (new file)

**Analog:** `internal/identity/identity.go` lines 183–278 (`Create` + `runPipeline` — the orchestration structure); `internal/identity/modes.go` lines 19–38 (`Reuse` — the "load existing + funnel" pattern).

**Package + imports pattern** (mirror `identity.go`):
```go
package identity

import (
    "fmt"
    "strings"

    "github.com/castocolina/gitid/internal/gitconfig"
    "github.com/castocolina/gitid/internal/sshconfig"
)
```

**Deps-injection pattern** (mirror `Deps` struct in `identity.go` lines 113–142): inject `readFrag` as a function parameter (not a struct field) because `Reconstruct` is a pure read function with no write side effects. This keeps the signature simple while remaining fake-testable.

**Name-union helper pattern** (follows the `runPipeline` compose-then-iterate pattern):
```go
func nameUnion(sshHosts map[string]sshconfig.SSHHostInfo, gcBlocks map[string]gitconfig.IncludeIfInfo) []string
```
Returns sorted slice of all identity names seen in either map.

**`Reconstruct` signature:**
```go
// Reconstruct assembles []Account from the four managed artifacts.
// sshBytes and gcBytes are the raw bytes of ~/.ssh/config and ~/.gitconfig.
// readFrag is injectable for testing (fake reads). The join key is the identity
// name (D-01). Accounts with missing pieces are included with Incomplete set
// (D-02); deep diagnosis stays in Phase 4 doctor.
func Reconstruct(
    sshBytes, gcBytes []byte,
    readFrag func(fragPath string) (gitconfig.FragmentInfo, error),
) ([]Account, error)
```

---

### `internal/identity/update.go` (new file)

**Analog:** `internal/identity/modes.go` lines 119–140 (`Rotate`) and `identity.go` lines 209–278 (`runPipeline`)

**Deps-injection pattern to mirror** (from `identity.go` lines 113–142, `Deps` struct):
```go
type UpdateDeps struct {
    WriteSSH             func(accountName, hostBlock, globalBlock string) (string, error)
    WriteGitconfig       func(identity, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (string, error)
    WriteFragment        func(fragPath, name, email, signingKeyPath string, signing bool) error
    WriteAllowedSigners  func(path, identity, line string) (string, error)
    RemoveAllowedSigners func(path, identityEmail string) (string, error)
    Resolved             func(alias string) (tester.Result, tester.ResolvedConfig)
}
```
Each field is a function — same convention as `Deps` in `identity.go`. No method receivers.

**`rotateInput` pattern to mirror** (from `modes.go` lines 146–163 — builds `CreateInput` from `Account`):
```go
func rotateInput(a Account) CreateInput {
    return CreateInput{
        Name:               a.Name,
        GitName:            a.GitName,
        // …all fields from Account…
        Confirmed: true,
    }
}
```
`update.go` has an analogous `updateInput(existing Account, edited Account) CreateInput` helper that applies the edited fields over the existing account's paths.

**Structural-change detection pattern** (D-05 — compare specific fields):
```go
structural := edited.Alias != existing.Alias ||
    edited.Hostname != existing.Hostname ||
    edited.Port != existing.Port
```
If `structural`, call `deps.Resolved(edited.Alias)` after writes.

**`Update` signature:**
```go
// Update applies the edited fields to the existing identity, re-renders the
// four artifacts via the safe-write path, and runs the resolved re-test when a
// structural field changed (D-05, D-06).
func Update(existing Account, edited Account, deps UpdateDeps) (UpdateResult, error)
```

**`UpdateResult` type** (mirror `CreateResult` from `identity.go` lines 147–161):
```go
type UpdateResult struct {
    Resolved     tester.ResolvedConfig
    ResolvedTest tester.Result
    // PreviewOnly is true when no writes were performed (Confirmed was false).
    PreviewOnly bool
}
```

---

### `internal/identity/delete.go` (new file)

**Analog:** `internal/identity/modes.go` lines 67–117 (`AddAccount` — orchestrates without key generation, reads existing `Account`, calls `runPipeline`)

**Deps-injection pattern to mirror** (`Deps` convention from `identity.go`):
```go
type DeleteDeps struct {
    ReadSSH              func() ([]byte, error)
    ReadGitconfig        func() ([]byte, error)
    WriteSSH             func(content []byte) (backupPath string, err error)
    WriteGitconfig       func(content []byte) (backupPath string, err error)
    RemoveFragment       func(fragPath string) (backupPath string, err error)
    RemoveAllowedSigners func(path, email string) (backupPath string, err error)
    RemoveKeyFiles       func(keyPath, pubPath string) error
}
```

**Error wrapping pattern to mirror** (from `identity.go` lines 261–271 — every error wrapped with `fmt.Errorf("identity: …: %w", err)`):
```go
if _, werr := deps.WriteSSH(in.Name, hostBlock, in.GlobalBlock); werr != nil {
    return res, fmt.Errorf("identity: writing ssh config: %w", werr)
}
```
`delete.go` follows the same `fmt.Errorf("identity: <action>: %w", err)` convention.

**`Delete` signature:**
```go
// Delete removes the four per-identity artifacts (SSH Host block, includeIf
// block, fragment file, allowed_signers line) with backup. When keepKey is
// false, a separate deps.RemoveKeyFiles call removes the private and public key
// files (irreversible — D-07). Shows removal manifest before the single confirm.
func Delete(acct Account, keepKey bool, deps DeleteDeps) (DeleteResult, error)
```

**`DeleteResult` type:**
```go
type DeleteResult struct {
    SSHBackup        string
    GitconfigBackup  string
    FragmentBackup   string
    AllowedSignersBackup string
}
```

---

### `cmd/gitid/list.go` (new file)

**Analog:** `cmd/gitid/add.go` lines 30–41 (`newAddCmd` — Cobra command factory pattern)

**Cobra command factory pattern** (lines 30–41):
```go
func newAddCmd() *cobra.Command {
    var dryRun bool
    cmd := &cobra.Command{
        Use:   "add",
        Short: "Create a new Git identity …",
        RunE: func(cmd *cobra.Command, _ []string) error {
            return runIdentityAdd(cmd.InOrStdin(), cmd.OutOrStdout(), dryRun, buildDeps)
        },
    }
    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "…")
    return cmd
}
```
`list.go` uses `newListCmd()` returning `*cobra.Command`; `RunE` delegates to `runIdentityList(cmd.InOrStdin(), cmd.OutOrStdout())`. No `--dry-run` for list.

**`fp` helper to reuse** (`add.go` line 260 — already defined in the package):
```go
func fp(out io.Writer, s string) { _, _ = io.WriteString(out, s) }
```
`fp` is defined once in the `main` package (`add.go`). `list.go`, `update.go`, `delete.go` all share it — do NOT redefine it.

**`buildDeps`-style wiring pattern** (`add.go` lines 319–423): `list.go` does NOT need a Deps struct (read-only). It calls `os.ReadFile` for `~/.ssh/config` and `~/.gitconfig`, then calls `identity.Reconstruct` with the real `gitconfig.ReadFragment` function.

**Package + imports pattern** (mirror `add.go` lines 1–24):
```go
package main

import (
    "fmt"
    "io"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"

    "github.com/castocolina/gitid/internal/gitconfig"
    "github.com/castocolina/gitid/internal/identity"
)
```

---

### `cmd/gitid/update.go` (new file)

**Analog:** `cmd/gitid/rotate.go` (name validation + prompt + confirm pattern) and `cmd/gitid/add.go` (`buildDeps` wiring)

**`identityNameRe` + `sanitizeName` reuse** (`rotate.go` lines 18–28 — defined once in the package):
```go
func sanitizeName(name string) string { return strings.TrimSpace(name) }
var identityNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
```
These are defined in `rotate.go`. `update.go` uses them without re-declaring.

**Confirm pattern** (`add.go` lines 510–515):
```go
func confirm(r *bufio.Reader, out io.Writer, label string) bool {
    fp(out, fmt.Sprintf("%s [y/N]: ", label))
    line, _ := r.ReadString('\n')
    line = strings.ToLower(strings.TrimSpace(line))
    return line == "y" || line == "yes"
}
```
`update.go` uses `confirm(reader, out, "Apply these changes now?")` — same function, not redefined.

**`buildUpdateDeps` pattern** (mirror `buildDeps` in `add.go` lines 319–423 — wire `UpdateDeps` from real internal packages):
```go
func buildUpdateDeps(_ io.Writer) identity.UpdateDeps {
    return identity.UpdateDeps{
        WriteSSH:       func(name, block, global string) (string, error) { … },
        WriteGitconfig: func(id, fragPath, allowedPath string, matches []gitconfig.Match) (string, error) { … },
        // …
        Resolved: tester.Resolved,
    }
}
```

**`RunE` delegation pattern** (same as `add.go` line 36):
```go
RunE: func(cmd *cobra.Command, args []string) error {
    return runIdentityUpdate(cmd.InOrStdin(), cmd.OutOrStdout(), args, buildUpdateDeps)
},
```

---

### `cmd/gitid/delete.go` (new file)

**Analog:** `cmd/gitid/rotate.go` + `cmd/gitid/add.go`

**Two-step confirm pattern** (D-07 — keep key by default, second prompt for key deletion):
```go
// First confirm: remove managed blocks.
if !confirm(reader, out, "Remove managed blocks and fragment file now?") {
    fp(out, "Delete cancelled; no files were written.\n")
    return nil
}
// … perform block removal …

// Second confirm (default no): irreversible key deletion.
if !confirm(reader, out, "Also delete private key files? (irreversible, default no)") {
    keepKey = true
}
```
The `confirm` function defaults to "N" (`line == "y" || line == "yes"`), so pressing Enter keeps the key — the D-07 safe default.

**Removal manifest print pattern** (mirror `printPreview` in `add.go` lines 462–472):
```go
fp(out, "Will remove:\n")
fp(out, fmt.Sprintf("  [1] SSH Host block     \"# BEGIN gitid managed: %s\" in %s\n", acct.Name, acct.SSHConfigPath))
fp(out, fmt.Sprintf("  [2] gitconfig block    \"# BEGIN gitid managed: %s\" in %s\n", acct.Name, acct.GitconfigPath))
fp(out, fmt.Sprintf("  [3] Fragment file      %s\n", acct.FragmentPath))
fp(out, fmt.Sprintf("  [4] allowed_signers    line for <%s> in %s\n", acct.GitEmail, acct.AllowedSignersPath))
```

**`buildDeleteDeps` pattern** (same wiring style as `buildDeps`):
```go
func buildDeleteDeps(_ io.Writer) identity.DeleteDeps {
    return identity.DeleteDeps{
        ReadSSH:       func() ([]byte, error) { return os.ReadFile(/* ~/.ssh/config */) },
        ReadGitconfig: func() ([]byte, error) { return os.ReadFile(/* ~/.gitconfig */) },
        WriteSSH:      func(content []byte) (string, error) { return filewriter.Write(/* sshConfigPath */, content, 0o600) },
        // …
        RemoveFragment:       filewriter.BackupAndRemove,
        RemoveAllowedSigners: gitconfig.RemoveAllowedSignersLine,
    }
}
```

---

### Test files

#### `internal/filewriter/block_list_test.go` (new)

**Analog:** existing test files in `internal/filewriter/` (pattern: `package filewriter`, table-driven subtests, pure byte manipulation — no fakes needed)

**Key test conventions** (match existing test style in the repo):
```go
package filewriter

import "testing"

func TestListBlocks_Empty(t *testing.T) { … }
func TestListBlocks_OneBlock(t *testing.T) { … }
func TestRemoveBlock_Idempotent(t *testing.T) { … }
func TestRoundTrip_ReplaceRemoveReplace(t *testing.T) { … }
```
No `t.TempDir()` needed — pure string transforms.

#### `internal/sshconfig/coexistence_test.go` (new)

**Analog:** `internal/tester/tester.go` lines 113–143 (`Resolved`, `ParseResolved`) — the `ssh -G -F` hermetic test pattern:
```go
out, _ := exec.Command("ssh", "-G", "-F", configPath, alias).Output() //nolint:gosec
rc := tester.ParseResolved(string(out))
```
The `-F` flag overrides the config path, making the test fully hermetic (no real `~/.ssh/config` read). Guard with `t.Skip` if `ssh` is not found.

**Hermetic HOME pattern** (from `internal/gitconfig/includeif_resolve_test.go`):
```go
home := t.TempDir()
t.Setenv("HOME", home)
t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
```

---

## Shared Patterns

### 1. Deps-Injection Pattern (all `internal/identity/*.go` files)

**Source:** `internal/identity/identity.go` lines 113–142 (`Deps` struct)

Every external effect (reads, writes, exec calls) is a function field on a `*Deps` struct. Internal logic never calls `os.ReadFile`, `exec.Command`, or `filewriter.Write` directly — it always calls through the injected dep. This makes every operation fake-testable and TUI-reusable.

```go
// Pattern: function fields, not interface methods.
type UpdateDeps struct {
    WriteSSH func(name, block, global string) (string, error)
    // … one field per external effect …
}
```

### 2. Error Wrapping Convention (all `internal/` files)

**Source:** `internal/identity/identity.go` lines 261–271

All errors are wrapped with `fmt.Errorf("<package>/<function>: <action>: %w", err)`:
```go
return res, fmt.Errorf("identity: writing ssh config: %w", werr)
```

### 3. `git config --file` Exec Pattern (all `internal/gitconfig/*.go` files)

**Source:** `internal/gitconfig/fragment.go` lines 74–80

Always use arg-slice `exec.Command("git", "config", "--file", path, key, value)` — never `fmt.Sprintf` into a shell string. Always annotate with `//nolint:gosec // arg-slice form, no shell; values validated above (G204)`.

### 4. `filewriter.Write` as the Only Write Chokepoint

**Source:** `internal/filewriter/filewriter.go` lines 33–85

Every file mutation routes through `filewriter.Write(targetPath, content, mode)` or `filewriter.BackupAndRemove(path)`. No `os.WriteFile` or `os.Remove` in business logic files.

### 5. `fp(out, …)` for Command Output

**Source:** `cmd/gitid/add.go` line 260

```go
func fp(out io.Writer, s string) { _, _ = io.WriteString(out, s) }
```
All command output uses `fp`. Never `fmt.Fprintf(os.Stdout, …)`. `fp` is defined once in `add.go` — do not redefine in new command files.

### 6. `identityNameRe` + `sanitizeName` Validation

**Source:** `cmd/gitid/rotate.go` lines 18–28

```go
func sanitizeName(name string) string { return strings.TrimSpace(name) }
var identityNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
```
Apply to every user-supplied identity name in `update.go` and `delete.go`. Do not redefine — they are already in the `main` package.

### 7. Hermetic Test HOME Pattern

**Source:** `internal/gitconfig/includeif_resolve_test.go`

```go
home := t.TempDir()
t.Setenv("HOME", home)
t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
```
All Phase 3 tests that interact with files use this setup. No test reads or writes the real `~/.ssh` or `~/.gitconfig`.

### 8. Cobra `RunE` Delegation Pattern

**Source:** `cmd/gitid/add.go` lines 30–41

```go
RunE: func(cmd *cobra.Command, _ []string) error {
    return runIdentity<X>(cmd.InOrStdin(), cmd.OutOrStdout(), …)
},
```
Zero business logic in `RunE`. All orchestration in a `runIdentity<X>(in io.Reader, out io.Writer, …)` function.

---

## No Analog Found

All files have close analogs in the codebase. No file requires falling back to RESEARCH.md patterns only.

---

## Metadata

**Analog search scope:** `internal/filewriter/`, `internal/sshconfig/`, `internal/gitconfig/`, `internal/identity/`, `cmd/gitid/`, `internal/tester/`
**Files read:** 9 source files fully read
**Pattern extraction date:** 2026-06-10
