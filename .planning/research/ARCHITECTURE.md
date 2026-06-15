# Architecture Research

**Domain:** CLI + TUI SSH/Git identity manager (single binary, Go)
**Researched:** 2026-06-08
**Confidence:** HIGH

---

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     Entrypoints (cmd/)                           │
│                                                                  │
│  ┌──────────────────────┐   ┌──────────────────────────────┐    │
│  │  cmd/gitid/main.go   │   │  (same binary, TUI mode)     │    │
│  │  Cobra root + sub-   │   │  launched when no args or    │    │
│  │  commands; non-      │   │  via `gitid tui`; wires      │    │
│  │  interactive output  │   │  Bubble Tea program          │    │
│  └──────────┬───────────┘   └──────────────┬───────────────┘    │
│             │  direct function calls        │  direct calls      │
└─────────────┼──────────────────────────────┼────────────────────┘
              │                              │
┌─────────────▼──────────────────────────────▼────────────────────┐
│                  Core (internal/)  — UI-free, TDD                │
│                                                                  │
│  ┌────────────┐  ┌────────────┐  ┌──────────┐  ┌─────────────┐  │
│  │  identity  │  │ sshconfig  │  │gitconfig │  │   doctor    │  │
│  │  (model +  │  │ (parse +   │  │(parse +  │  │ (checks +   │  │
│  │   CRUD)    │  │  render)   │  │ render)  │  │  reporter)  │  │
│  └─────┬──────┘  └─────┬──────┘  └────┬─────┘  └──────┬──────┘  │
│        │               │              │               │          │
│  ┌─────▼──────────────────────────────▼───────────────▼───────┐  │
│  │                   filewriter (internal/)                    │  │
│  │  backup → render managed block → idempotent replace →      │  │
│  │  set permissions → verify (shared by sshconfig+gitconfig)  │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────────┐  │
│  │  keygen  │  │  tester  │  │clipboard │  │  platform/deps  │  │
│  │(ed25519  │  │(ssh -i + │  │(pbcopy/  │  │(OS detection,   │  │
│  │  + sign) │  │ssh -T -G)│  │ xclip)   │  │ tool presence)  │  │
│  └──────────┘  └──────────┘  └──────────┘  └─────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
              │
┌─────────────▼────────────────────────────────────────────────────┐
│              Filesystem (source of truth — no sidecar DB)        │
│  ~/.ssh/config   ~/.gitconfig   ~/.gitconfig.d/<name>.gitconfig  │
│  ~/.ssh/<name>   ~/.ssh/<name>.pub   ~/.ssh/allowed_signers       │
└──────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Notes |
|-----------|----------------|-------|
| `identity` | Domain model: `Account` (name, host alias, provider, email, key path, match strategy). Reconstructs identities by parsing managed blocks across `~/.ssh/config` and `~/.gitconfig`. CRUD. | Source of truth lives in files; this package is the translation layer. |
| `sshconfig` | Parse `~/.ssh/config` for managed blocks; render a Host stanza to text; delegates write to `filewriter`. Wraps `kevinburke/ssh_config` for comment-preserving round-trips. | Knows SSH config syntax; does not own write safety. |
| `gitconfig` | Parse `~/.gitconfig` for managed blocks; render `includeIf` stanzas and per-identity fragments; delegates write to `filewriter`. Owns the fragment files in `~/.gitconfig.d/`. | Custom line-by-line parser required; no existing Go library supports `includeIf` write-back. |
| `filewriter` | Shared safe-write concern: timestamped backup, render-to-temp, atomic rename (`os.Rename`), set correct `chmod`, optional confirmation callback. | Used by `sshconfig` and `gitconfig`. Keeps safety logic in one place. |
| `keygen` | Generate ed25519 key pair (auth + signing); write to `~/.ssh/<name>` with mode 600, `.pub` with mode 644. Returns key path pair. | Thin wrapper over `crypto/ed25519` + `golang.org/x/crypto/ssh`. |
| `tester` | Two-phase test: `ssh -i <key> -T <host>` (explicit key), then `ssh -T <alias>` + `ssh -G <alias>` (resolved config). Returns structured result with raw output. | Runs `os/exec`; pure input/output, no side effects. |
| `doctor` | Health checks: key permissions, SSH config coherence, gitconfig coherence, orphaned blocks, signing wiring, agent presence, dependency presence. Returns structured findings. | Composes `platform`, `deps`, `sshconfig`, `gitconfig`; no writes. |
| `clipboard` | Copy public key text to clipboard. macOS: `pbcopy`; Linux: `xclip`/`xsel`/`wl-copy` (detected at runtime via `deps`). | Thin; wraps `os/exec`. |
| `platform` | Detect OS (`darwin`/`linux`). Provide platform-specific hints (UseKeychain guard, clipboard command, permission fix commands). | No third-party dependency. |
| `deps` | Check for required external tools (`ssh`, `ssh-keygen`, `git`). Check optional tools (`ssh-add`, `pbcopy`, `xclip`). Return structured availability report. | Used by `doctor` and `platform`. |

---

## Recommended Project Structure

```
gitid/
├── main.go                         # Minimal: calls cmd.Execute()
├── go.mod
├── go.sum
├── Makefile                        # setup-env, build, test, lint, fmt, install, uninstall
│
├── cmd/
│   └── gitid/
│       ├── main.go                 # Cobra root wiring — thin, no business logic
│       ├── add.go                  # `gitid add` — calls identity.Create(...)
│       ├── list.go                 # `gitid list` — calls identity.List(...)
│       ├── edit.go                 # `gitid edit` — calls identity.Update(...)
│       ├── delete.go               # `gitid delete` — calls identity.Delete(...)
│       ├── rotate.go               # `gitid rotate` — calls keygen + sshconfig + gitconfig
│       ├── test.go                 # `gitid test` — calls tester.Run(...)
│       ├── doctor.go               # `gitid doctor` — calls doctor.Check(...)
│       ├── copy.go                 # `gitid copy` — calls clipboard.Copy(...)
│       └── tui.go                  # `gitid tui` (or naked `gitid`) — starts Bubble Tea program
│
├── internal/
│   ├── identity/
│   │   ├── identity.go             # Account struct, CRUD interface
│   │   ├── identity_test.go
│   │   ├── loader.go               # Reconstruct []Account from parsed managed blocks
│   │   └── loader_test.go
│   │
│   ├── sshconfig/
│   │   ├── parser.go               # Parse ~/.ssh/config; extract managed blocks
│   │   ├── parser_test.go
│   │   ├── renderer.go             # Render Account → SSH Host stanza text
│   │   ├── renderer_test.go
│   │   └── writer.go               # Compose: parser + renderer + filewriter
│   │
│   ├── gitconfig/
│   │   ├── parser.go               # Parse ~/.gitconfig; extract managed includeIf blocks
│   │   ├── parser_test.go
│   │   ├── renderer.go             # Render Account → includeIf block + fragment text
│   │   ├── renderer_test.go
│   │   ├── fragment.go             # Read/write ~/.gitconfig.d/<name>.gitconfig
│   │   ├── fragment_test.go
│   │   └── writer.go               # Compose: parser + renderer + filewriter
│   │
│   ├── filewriter/
│   │   ├── filewriter.go           # backup, write-to-temp, atomic rename, chmod
│   │   └── filewriter_test.go
│   │
│   ├── keygen/
│   │   ├── keygen.go               # ed25519 key pair generation
│   │   └── keygen_test.go
│   │
│   ├── tester/
│   │   ├── tester.go               # ssh -i test + ssh -T -G resolved test
│   │   └── tester_test.go
│   │
│   ├── doctor/
│   │   ├── doctor.go               # Orchestrate all health checks; return Finding[]
│   │   ├── doctor_test.go
│   │   ├── checks/
│   │   │   ├── permissions.go      # Key/config file permission checks
│   │   │   ├── coherence.go        # SSH ↔ gitconfig coherence
│   │   │   ├── orphans.go          # Managed blocks without matching key files
│   │   │   └── signing.go          # allowed_signers + gpg.format wiring
│   │
│   ├── clipboard/
│   │   ├── clipboard.go            # pbcopy / xclip / wl-copy dispatch
│   │   └── clipboard_test.go
│   │
│   ├── platform/
│   │   ├── platform.go             # OS detection, UseKeychain guard, hints
│   │   └── platform_test.go
│   │
│   └── deps/
│       ├── deps.go                 # External tool availability checks
│       └── deps_test.go
│
└── tui/
    ├── app.go                      # Bubble Tea Program wiring
    ├── model.go                    # Root model; holds sub-models
    ├── doctor/
    │   ├── model.go                # Doctor dashboard model (initial screen)
    │   └── view.go
    ├── identity/
    │   ├── list.go                 # Identity list model
    │   ├── form.go                 # Add/edit form model
    │   └── view.go
    └── styles/
        └── styles.go               # Lip Gloss color/style definitions
```

### Structure Rationale

- **`cmd/gitid/`** — All Cobra command definitions. Each file is thin: parse flags, call `internal/` functions, format and print output. Zero business logic here.
- **`internal/`** — All domain logic. The Go compiler enforces that nothing outside this module can import these packages. This is the correct choice because `gitid` is a standalone binary, not a library. All packages are independently testable without any UI dependency.
- **`tui/`** — Bubble Tea models and views. Lives outside `internal/` so it is a peer to `cmd/`, not a dependency of core packages. It imports `internal/` but `internal/` never imports `tui/`. This keeps the dependency arrow one-directional and core logic UI-free.
- **`internal/filewriter/`** — Extracted as its own package so both `sshconfig.writer` and `gitconfig.writer` share identical backup and safe-write behavior. Prevents divergence.
- **No `pkg/`** — This is a single binary, not a library. Using `internal/` only is correct per official Go module guidance. `pkg/` would signal intent to export, which is not the goal.

---

## Architectural Patterns

### Pattern 1: Sentinel-Delimited Managed Block

**What:** Both `~/.ssh/config` and `~/.gitconfig` contain regions bounded by comment markers:

```
# BEGIN GITID MANAGED — <account-name>
...generated content...
# END GITID MANAGED — <account-name>
```

Content outside managed blocks is **never touched**. On startup, the `loader` in `internal/identity` scans both files, finds all pairs of sentinel markers, and reconstructs the `[]Account` slice from the parsed content inside each block. No sidecar database; the files are the database.

**When to use:** Any time the tool needs to know what identities exist (list, edit, delete, doctor, TUI startup).

**Trade-offs:**
- Pro: Zero drift — config state and tool state are always identical.
- Pro: Human-editable files remain safe; hand-written sections are preserved.
- Con: Parsing cost on every operation. For a developer tool with a handful of identities, this is negligible.
- Con: Block corruption (e.g., user deletes just the END marker) requires a recovery path in `doctor`.

**Example (sentinel layout in `~/.ssh/config`):**

```
# (hand-written content preserved verbatim above)

# BEGIN GITID MANAGED — personal
Host personal.github.com
  HostName ssh.github.com
  Port 443
  User git
  IdentityFile ~/.ssh/gitid_personal
  IdentitiesOnly yes
# END GITID MANAGED — personal

# BEGIN GITID MANAGED — client-acme
Host acme.github.com
  ...
# END GITID MANAGED — client-acme
```

### Pattern 2: Parse → Model → Render → Safe Write → Verify

**What:** Every mutation follows this strict pipeline. No step may be skipped.

```
1. PARSE   sshconfig.Parse(path)      → RawFile{managed: []Block, foreign: []Line}
           gitconfig.Parse(path)      → RawFile{managed: []Block, foreign: []Line}

2. MODEL   identity.Loader.Load(      → []Account
             sshBlocks, gitBlocks)

3. MUTATE  identity.Create/Update/    → []Account (modified in memory)
           Delete(accounts, input)

4. RENDER  sshconfig.Render(accounts) → string  (all managed blocks)
           gitconfig.Render(accounts) → string  (all managed blocks)

5. COMPOSE filewriter.Write(path, {   → (backup path, error)
             foreign: foreign,
             managed: rendered,
           })

6. VERIFY  tester.Run(account)        → TestResult{pass bool, output string}

7. CONFIRM if !TestResult.pass:
             filewriter.Restore(backupPath, path)
```

**When to use:** Every create, update, delete, and rotate operation.

**Trade-offs:**
- Pro: Atomic from the user's perspective — backup exists before any file is touched.
- Pro: The render→parse round-trip is deterministic and testable without touching the filesystem.
- Con: Requires reading the full file on every write, even for a single-stanza update. Acceptable at this scale.

### Pattern 3: Cobra Command Delegates to Core; TUI Command Calls Same Core

**What:** Both CLI and TUI call the same `internal/` functions directly. There is no "service layer" or intermediate abstraction between the entrypoints and the core.

```go
// cmd/gitid/add.go  (Cobra handler)
func runAdd(cmd *cobra.Command, args []string) error {
    accts, err := identity.LoadFromFiles(sshConfigPath, gitconfigPath)
    // ... build input from flags ...
    result, err := identity.Create(accts, input, keygen.Generate, sshconfig.Write, gitconfig.Write)
    fmt.Println(result.Summary())
    return err
}

// tui/identity/form.go  (Bubble Tea Update)
func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case submitMsg:
        return m, func() tea.Msg {
            result, err := identity.Create(m.accounts, m.input, keygen.Generate, sshconfig.Write, gitconfig.Write)
            return createResultMsg{result: result, err: err}
        }
    }
    // ...
}
```

The key point: `identity.Create` receives its dependencies (keygen function, write functions) as parameters. This makes it trivially testable with fakes and prevents the TUI from having to replicate logic.

**When to use:** Every user-triggered action (create, test, rotate, doctor, copy).

**Trade-offs:**
- Pro: No duplicated logic; one path to verify.
- Pro: TUI and CLI automatically stay in sync.
- Con: Cobra command handlers must be written carefully to not accumulate logic. Enforce with code review.

### Pattern 4: Bubble Tea Command (tea.Cmd) for All Core Calls

**What:** All calls to `internal/` packages from TUI models are wrapped in `tea.Cmd` (functions that run off the event loop and return a `tea.Msg`). The `Update` function never blocks.

```go
func loadIdentities(sshPath, gitPath string) tea.Cmd {
    return func() tea.Msg {
        accts, err := identity.LoadFromFiles(sshPath, gitPath)
        return identitiesLoadedMsg{accounts: accts, err: err}
    }
}
```

**When to use:** Any call that touches the filesystem, runs external commands (`ssh -T`, `ssh-keygen`), or may take non-trivial time.

**Trade-offs:**
- Pro: UI remains responsive during slow operations (key generation, SSH test).
- Pro: Error handling is centralized in `Update` via message types.
- Con: Adds indirection; developers unfamiliar with Elm/MVU need a short ramp-up.

---

## Data Flow

### Identity Creation Flow

```
User input (CLI flags or TUI form)
        │
        ▼
identity.LoadFromFiles(~/.ssh/config, ~/.gitconfig)
        │  parse managed blocks from both files
        ▼
[]Account  (current state)
        │
        ▼
identity.Create(accounts, input)
        │  validate (no alias collision, valid email, valid provider)
        ▼
keygen.Generate(name, comment)
        │  writes ~/.ssh/<name>, ~/.ssh/<name>.pub  (mode 600/644)
        ▼
sshconfig.Render(updatedAccounts)  +  gitconfig.Render(updatedAccounts)
        │  produces string representations of all managed blocks
        ▼
filewriter.Write(~/.ssh/config, rendered)
filewriter.Write(~/.gitconfig, rendered)
filewriter.Write(~/.gitconfig.d/<name>.gitconfig, fragment)
        │  each: backup(timestamp) → write-to-temp → atomic rename → chmod
        ▼
tester.Run(newAccount)
        │  ssh -i <keypath> -T <host>  (phase 1)
        │  ssh -T <alias>  +  ssh -G <alias>  (phase 2)
        ▼
TestResult{pass, output}
        │  if !pass → filewriter.Restore(backups)
        ▼
Return result to entrypoint (CLI prints; TUI displays)
```

### Identity Reconstruction Flow (startup / list)

```
identity.LoadFromFiles(sshConfigPath, gitconfigPath)
        │
        ├── sshconfig.Parse(sshConfigPath)
        │     tokenize lines
        │     find BEGIN/END GITID MANAGED sentinel pairs
        │     for each pair: extract account-name from sentinel comment
        │                    parse Host stanza inside block
        │     return: []ManagedSSHBlock{name, hostAlias, hostname, port, identityFile}
        │
        ├── gitconfig.Parse(gitconfigPath)
        │     find BEGIN/END GITID MANAGED sentinel pairs
        │     parse includeIf key + fragment path inside block
        │     return: []ManagedGitBlock{name, includeIfValue, fragmentPath, matchStrategy}
        │
        └── identity.Merge(sshBlocks, gitBlocks)
              join by account-name (sentinel tag)
              for each joined pair: read fragment file → extract user.name, user.email,
                                    gpg.format, user.signingkey
              return: []Account
```

### Safe Write Flow (shared via filewriter)

```
filewriter.Write(targetPath, content string, opts WriteOpts)
        │
        ├── 1. backup:  cp targetPath → targetPath.<timestamp>.bak
        │               record backup path for possible restore
        │
        ├── 2. compose: foreignLines + managedBlocks → full file content
        │               (foreign lines are those that were outside any managed block)
        │
        ├── 3. write:   os.CreateTemp(dir(targetPath), "gitid-*.tmp")
        │               write full content to temp file
        │               f.Sync()
        │
        ├── 4. chmod:   os.Chmod(tmpPath, targetMode)  (600 for config, 644 for .pub)
        │
        ├── 5. rename:  os.Rename(tmpPath, targetPath)  — atomic on Linux/macOS
        │
        └── 6. return:  (backupPath, nil) on success
                        restore from backup on error (if backup exists)
```

---

## Component Boundaries

| Boundary | Direction | Rule |
|----------|-----------|------|
| `cmd/` → `internal/` | cmd imports internal | Cobra commands call internal functions directly. No logic in cmd. |
| `tui/` → `internal/` | tui imports internal | Bubble Tea commands call internal functions. No logic in tui models beyond UI state. |
| `internal/identity` → `internal/sshconfig` + `internal/gitconfig` | identity imports both | `LoadFromFiles` coordinates parsers; `Create/Update/Delete` calls writers. |
| `internal/sshconfig` → `internal/filewriter` | sshconfig imports filewriter | The write step is delegated; sshconfig owns render only. |
| `internal/gitconfig` → `internal/filewriter` | gitconfig imports filewriter | Same pattern as sshconfig. |
| `internal/doctor` → all other internal | doctor imports all except filewriter | Doctor is read-only; it inspects state but never writes. |
| `internal/*` → `internal/platform` + `internal/deps` | leaf packages import platform/deps | Low-level OS concerns are resolved in platform/deps and passed up. |
| `internal/` → `tui/` | **NEVER** | Core packages must never import TUI. Dependency arrow is one-directional. |
| `internal/` → `cmd/` | **NEVER** | Core packages must never import Cobra. |

---

## Build Order (dependency-driven)

Build and test in this order. Each phase can proceed only when its dependencies pass tests.

```
Phase 1: Foundation (no external dependencies)
  platform/  → no deps
  deps/      → imports platform
  filewriter/ → imports os, no domain deps

Phase 2: Primitive Operations
  keygen/    → imports crypto/ed25519, golang.org/x/crypto/ssh, platform
  clipboard/ → imports platform, deps

Phase 3: Config Parse + Render (stateless, pure functions first)
  sshconfig/parser    → imports kevinburke/ssh_config (or custom tokenizer)
  sshconfig/renderer  → no external deps
  gitconfig/parser    → custom line parser (no adequate external lib for includeIf write-back)
  gitconfig/renderer  → no external deps
  gitconfig/fragment  → imports filewriter

Phase 4: Config Writers (compose previous phases)
  sshconfig/writer   → imports sshconfig/{parser,renderer} + filewriter
  gitconfig/writer   → imports gitconfig/{parser,renderer,fragment} + filewriter

Phase 5: Identity Model (the aggregation layer)
  identity/          → imports sshconfig, gitconfig (read), platform

Phase 6: Test + Doctor
  tester/            → imports platform, deps, identity (for host info)
  doctor/            → imports all internal packages (read-only)

Phase 7: CLI Entrypoint
  cmd/gitid/         → imports identity + all internal packages
  Integration tests here: full create → test → list → delete round-trips

Phase 8: TUI
  tui/               → imports identity + all internal packages
  Manual verification of doctor dashboard, form flows, keyboard navigation
```

---

## Anti-Patterns

### Anti-Pattern 1: Business Logic in Cobra Handlers

**What people do:** Add validation, file manipulation, or decision logic directly inside `cmd/gitid/add.go`.

**Why it's wrong:** The TUI has no way to reuse it. It will be duplicated or the TUI will have to call the CLI, coupling layers badly. Testing requires invoking Cobra.

**Do this instead:** All logic lives in `internal/identity`. Cobra handlers are `≤30 lines`: parse flags, call one `internal/` function, format output.

### Anti-Pattern 2: Blind Append to Config Files

**What people do:** Open `~/.ssh/config` with `os.OpenFile(O_APPEND)` and write a new Host stanza without checking what already exists.

**Why it's wrong:** This is how the original Bash script works and it creates duplicates on every re-run. There is no idempotency.

**Do this instead:** Always parse the full file, reconstruct from managed blocks (which enforces one block per account-name), render all managed blocks together, and write the full file atomically via `filewriter`.

### Anti-Pattern 3: Using an External Library for gitconfig Write-Back

**What people do:** Reach for `gopasspw/gopass/pkg/gitconfig` or similar for writing `~/.gitconfig` changes.

**Why it's wrong:** None of the available Go gitconfig libraries support `includeIf` write-back (the gopass package explicitly states includes are unsupported). Using such a library for reads and then doing custom writes creates two code paths and potential inconsistency.

**Do this instead:** Write a custom line-by-line parser in `internal/gitconfig/parser.go` that handles the sentinel block pattern. It does not need to understand the full gitconfig spec — only enough to extract and replace managed blocks. Foreign lines are preserved verbatim.

### Anti-Pattern 4: TUI Model Holding Core State

**What people do:** Store `[]Account` inside a Bubble Tea model struct and mutate it directly in `Update`.

**Why it's wrong:** If the TUI and the CLI run concurrently (unlikely but possible), or if the user edits files manually between TUI refresh cycles, the in-memory state diverges from the files.

**Do this instead:** TUI models hold UI state only (cursor position, form field values, loading booleans). Identity data is loaded from files at the start of each user action via a `tea.Cmd`. The Elm architecture's unidirectional flow enforces this naturally.

### Anti-Pattern 5: Cross-Platform `~/.ssh/config` Directives Without Guards

**What people do:** Always emit `UseKeychain yes` and `AddKeysToAgent yes` inside `Host *`.

**Why it's wrong:** These are macOS-only directives. Linux `ssh` will error on `UseKeychain`.

**Do this instead:** `sshconfig/renderer.go` wraps the `Host *` global block with `IgnoreUnknown UseKeychain` when `platform.IsDarwin()` is false, or emits the macOS block only on macOS. The `platform` package owns this decision.

---

## Integration Points

### External Tools (via `os/exec`)

| Tool | Called By | Purpose | Availability Check |
|------|-----------|---------|-------------------|
| `ssh-keygen` | `keygen` | Generate ed25519 key pair | `deps.Check("ssh-keygen")` |
| `ssh` | `tester` | Phase 1 + Phase 2 tests | `deps.Check("ssh")` |
| `git` | `doctor/checks` | Verify gitconfig resolution | `deps.Check("git")` |
| `ssh-add` | `doctor/checks` | Check agent state | `deps.CheckOptional("ssh-add")` |
| `pbcopy` | `clipboard` | macOS clipboard | `deps.CheckOptional("pbcopy")` |
| `xclip` / `xsel` / `wl-copy` | `clipboard` | Linux clipboard | `deps.CheckOptional(...)` |

### Go Libraries

| Library | Package | Why | Confidence |
|---------|---------|-----|-----------|
| `github.com/spf13/cobra` | `cmd/gitid/` | Standard Go CLI framework; subcommands, completion, help | HIGH |
| `github.com/charmbracelet/bubbletea` | `tui/` | Standard Go TUI framework; Elm MVU, async commands | HIGH |
| `github.com/charmbracelet/lipgloss` | `tui/styles/` | Terminal styling for Bubble Tea views | HIGH |
| `github.com/kevinburke/ssh_config` | `internal/sshconfig/parser` | Comment-preserving SSH config parser; round-trip safe | HIGH |
| `golang.org/x/crypto/ssh` | `internal/keygen/` | ed25519 key serialization to OpenSSH format | HIGH |
| `github.com/google/renameio` | `internal/filewriter/` | Production-grade atomic rename (preferred over raw `os.Rename`) | MEDIUM |

**Note on gitconfig library:** No existing Go library supports `includeIf` write-back. `internal/gitconfig/parser.go` must be a custom implementation. The parser only needs to handle the sentinel block boundary pattern — it does not need to be a full gitconfig spec parser. This is a deliberate, bounded scope.

---

## Scaling Considerations

This is a developer tool — "scale" means complexity of configuration, not concurrent users.

| Concern | With 1-5 identities | With 10-20 identities | Notes |
|---------|--------------------|-----------------------|-------|
| Parse cost | Negligible | Negligible | Files are <200 lines total |
| Managed block size | Small | Moderate | Render all blocks on every write — still fast |
| Doctor check time | <1s | <1s | Dominated by `ssh -G` subprocess |
| TUI list rendering | Trivial | Trivial | Lip Gloss renders in microseconds |

There is no scaling concern for this domain. Optimize for correctness and test coverage over performance.

---

## Sources

- [Go official module layout guidance](https://go.dev/doc/modules/layout) — HIGH confidence
- [golang-standards/project-layout (cmd/internal/pkg rationale)](https://github.com/golang-standards/project-layout) — MEDIUM confidence (community standard, not official)
- [Laurent SV: No nonsense Go package layout](https://laurentsv.com/blog/2024/10/19/no-nonsense-go-package-layout.html) — MEDIUM confidence
- [Ethan Lewis: Charming Cobras with Bubbletea](https://elewis.dev/charming-cobras-with-bubbletea-part-1) — MEDIUM confidence
- [BotMonster: Build CLI tool in Go with Cobra and Bubble Tea](https://botmonster.com/posts/build-cli-tool-go-cobra-bubble-tea/) — MEDIUM confidence
- [charmbracelet/bubbletea DeepWiki — architecture and MVU pattern](https://deepwiki.com/charmbracelet/bubbletea) — HIGH confidence
- [Zack Proser: Bubbletea state machine pattern](https://zackproser.com/blog/bubbletea-state-machine) — MEDIUM confidence
- [kevinburke/ssh_config pkg.go.dev](https://pkg.go.dev/github.com/kevinburke/ssh_config) — HIGH confidence
- [gopasspw/gopass gitconfig — no includeIf support](https://pkg.go.dev/github.com/gopasspw/gopass/pkg/gitconfig) — HIGH confidence (verified limitation)
- [Michael Stapelberg: Atomically writing files in Go](https://michael.stapelberg.ch/posts/2017-01-28-golang_atomically_writing/) — HIGH confidence
- [natefinch/atomic — atomic file write package](https://github.com/natefinch/atomic) — MEDIUM confidence

---
*Architecture research for: gitid — SSH/Git identity manager (Go CLI + TUI)*
*Researched: 2026-06-08*
