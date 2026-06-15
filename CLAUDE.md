# CLAUDE.md

Working agreements for this repository. Read before doing anything.

## Working method: hypothesis → test → implementation

Always work in this loop, in order:

1. **Hypothesis** — state what you believe and what is still unclear.
2. **Verify before planning** — *before* generating any plan or writing code,
   explicitly surface what is ambiguous and resolve it with the user. Do not
   produce a plan on top of unverified assumptions.
3. **Test** — prove the hypothesis with a real, observable check. Tests must
   show **input (the command run) and output (the real result)**.
4. **Implement** — only after the hypothesis is confirmed, and only with user
   confirmation for anything that mutates the user's files.

This applies to the product's own behavior too: config changes are applied as
*test → confirm + backup → re-test with the resolved configuration*.

## Language

**All generated content is in English** — code, comments, identifiers, UI text,
log/error messages, commit messages, and documentation. (Conversation with the
user may be in Spanish; artifacts are not.)

## Engineering

- Core logic lives in a UI-free package and is built test-first (TDD).
- Never write to a user's `~/.ssh/config` or `~/.gitconfig` without a
  timestamped backup, idempotent managed blocks, and explicit confirmation.

<!-- GSD:stack-start source:research/STACK.md -->

## Technology Stack

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.26.x (latest: 1.26.4) | Language runtime | Active current branch; 1.25.x is previous stable. Both supported. Pin minimum at 1.23+ for `go.mod` toolchain directive support. |
| github.com/spf13/cobra | v1.10.2 | CLI framework, shell completion | De-facto standard for Go CLIs. Built-in `completion` subcommand generates bash/zsh/fish/PowerShell scripts without extra code. import path stable at `github.com/spf13/cobra`. |
| charm.land/bubbletea/v2 | v2.0.7+ | TUI event loop (Elm architecture) | v2 is the recommended stable release (June 2024). Major renderer rewrite; atomic terminal updates via Mode 2026. Import path changed from `github.com/charmbracelet/bubbletea` — use `charm.land/bubbletea/v2`. |
| charm.land/lipgloss/v2 | v2.0.3 | TUI styling (colors, borders, layout) | v2.0.3 stable (April 2025). Pairs with Bubble Tea v2; manual color downsampling model is cleaner for terminal portability. Import: `charm.land/lipgloss/v2`. |
| charm.land/bubbles/v2 | v2.1.0 | Ready-made TUI components (list, viewport, textinput, spinner) | v2.1.0 stable (March 2025). Consistent API with Bubble Tea v2 and Lipgloss v2. Import: `charm.land/bubbles/v2`. |
| golang.org/x/crypto | v0.53.0 | Ed25519 key generation, OpenSSH serialization, `allowed_signers` | Standard library extension. `ssh.MarshalAuthorizedKey`, `ssh.MarshalPrivateKey`, `ssh.NewPublicKey` handle all key serialization needs. No third-party dependency required. |
| github.com/kevinburke/ssh_config | v1.6.0 | Parse + render `~/.ssh/config` round-trip | Best available Go parser for ssh_config. Explicitly designed for comment-preserving round-trip. See detailed notes below. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os/exec` (stdlib) | — | Shell out to `git config` for gitconfig reads/writes | Always, for gitconfig mutations — see gitconfig strategy below |
| `github.com/atotto/clipboard` | v0.1.4 | Cross-platform clipboard (`pbcopy`/`wl-copy`/`xclip`) | Clipboard copy of public key; platform dispatch in one import |
| `github.com/google/uuid` or `time`-based stamps | stdlib or v1.6.x | Timestamped backup filenames | Only stdlib `time.Now().Format` is strictly needed; no UUID required |

### Development Tools

| Tool | Version | Purpose | Configuration |
|------|---------|---------|---------------|
| golangci-lint | v2.12.2 | Lint aggregator (replaces running each linter separately) | `.golangci.yml` with `version: "2"` — see config template below |
| staticcheck | embedded in golangci-lint v2 | Advanced static analysis | Included as `staticcheck` linter entry in golangci-lint config |
| gosec | embedded in golangci-lint v2 | Security-focused checks (file perms, exec injection, etc.) | Included as `gosec` linter entry; critical for a tool that mutates `~/.ssh` |
| `unused` | embedded in golangci-lint v2 | Dead-code detection | Included as `unused` linter entry |
| goimports | standalone + golangci-lint | Format + manage import blocks | Run as `goimports -w ./...` in `make fmt`; also available as golangci-lint formatter |
| pre-commit | latest (Python tool) | Git hook runner that calls `make` targets | Hook repo: `TekWizely/pre-commit-golang` provides `golangci-lint-repo-mod`, `go-fmt-repo`, `go-test-repo` hooks |
| Make | system | Single task runner for all build/test/lint/install ops | `setup-env`, `build`, `install`, `uninstall`, `test`, `lint`, `fmt` targets minimum |

## Area-by-Area Rationale

### 1. `~/.ssh/config` parsing: kevinburke/ssh_config

- `Config.String()` and `Config.MarshalText()` are the render path
- The parser stores `rawValue` (preserving quoted values), `leadingSpace`, and per-node comments explicitly to support round-trip
- Known limitation: mixed tabs/spaces handling has a `TODO` in the source — minor whitespace normalization possible on re-write
- The `Match` directive is unsupported (not needed for gitid's use case — gitid only writes `Host`, `IdentityFile`, `IdentitiesOnly`, `Hostname`, `Port`, `User`, and global `Host *` directives)
- Issue #74 is open for "add/edit/delete hosts while preserving original content" — the current API requires traversing `cfg.Hosts[].Nodes` directly; there is no high-level `SetKey()` method
- **Conclusion: suitable for gitid.** The round-trip caveats (minor whitespace) are acceptable because gitid uses sentinel-delimited managed blocks: it only rewrites the blocks it owns and leaves everything else untouched. Parse → modify managed block → `cfg.String()` → write is stable enough.
- Never call a generic `SetKey(host, key, value)` across the whole file
- Only rewrite the `Host` nodes inside managed block boundaries
- Validate post-write with a second `Decode` pass (parse → render → parse stability check)

### 2. `~/.gitconfig` parsing and writing

| Library | includeIf support | insteadOf write | Comment-safe | Verdict |
|---------|-------------------|-----------------|--------------|---------|
| `go-git/v5/config` | No (issue #388, closed as stale) | Partial struct support only | `Raw` field preserves some structure, but not guaranteed for comments | Rejected — no includeIf write support |
| `gopasspw/gopass/pkg/gitconfig` | No (explicitly documented) | Unknown | No (whitespace normalization admitted) | Rejected — missing core feature |
| `muja/goconfig` | Unknown — unmaintained | Unknown | Unknown | Rejected — unmaintained |
| `git config` via `os/exec` | Full (git handles it natively) | Full | N/A — git owns the file | **Selected** |

- `git config --file ~/.gitconfig --add section.key value` is idempotent-safe and comment-preserving by design — git is the authoritative parser of its own format
- The tool already has `git` as a required runtime dependency (it tests SSH with `ssh -T`; the doctor checks `git` presence)
- `git config --get-regexp` and `--list` are reliable for reading managed blocks' keys
- The `os/exec` approach does not introduce external-process security risks when called without shell expansion (stdlib `os/exec` intentionally bypasses shell)
- `[includeIf "gitdir:..."]` and `[includeIf "hasconfig:remote.*.url:..."]` blocks — `git config` cannot write `includeIf` section headers natively; it can read them but not create them
- `[url "..."] insteadOf = ...` blocks (Phase 2)
- These are written as sentinel-delimited managed block text, appended/replaced in the file, not line-by-line via `git config` set

# BEGIN gitid managed: <identity-name>

# END gitid managed: <identity-name>

### 3. Ed25519 key generation + OpenSSH formatting + `allowed_signers`

### 4. Cobra + shell completion

- Import path: `github.com/spf13/cobra` (unchanged from v1)
- Completion: `rootCmd.InitDefaultCompletionCmd()` or let Cobra auto-register the `completion` subcommand, which handles `bash`, `zsh`, `fish`, and `powershell`
- No extra library needed for completion generation
- `pflag` is a transitive dependency (bundled with Cobra) — no separate import needed for flags

### 5. Quality toolchain

## Installation

# Module init (greenfield)

# Core TUI stack (charm.land vanity domain)

# CLI

# SSH config parser

# Crypto (already a transitive dep via x/crypto; explicit pin)

# Clipboard

# Dev tools (not in go.mod)

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| TUI framework | charm.land/bubbletea/v2 | tview, termdash | Not Elm-architecture; harder to test; bubbletea is the Go community standard for this style of TUI |
| gitconfig library | `git config` via os/exec | go-git/v5 config, gopass/gitconfig | Neither supports `includeIf` write; go-git issue #388 closed as stale; gopass explicitly documents the gap |
| SSH config parser | kevinburke/ssh_config | Custom parser, go-git's internal parser | Only maintained option; custom parser scope too large for this project |
| Crypto / key gen | golang.org/x/crypto/ssh | charmbracelet/keygen | x/crypto is authoritative and covers all needed operations; charmbracelet/keygen is a thin wrapper that adds a dependency without benefit |
| Lint aggregator | golangci-lint v2 | Running each linter separately | Parallel execution, unified config, unified output; v2 is current |
| Pre-commit hooks | Local hooks → make targets | TekWizely/pre-commit-golang upstream hooks | Upstream hooks bypass make, diverge from CI; local hooks + make = single source of truth |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `gopasspw/gopass/pkg/gitconfig` | No `includeIf` support (explicitly documented); no round-trip comment safety | `git config` via `os/exec` for reads/writes; raw text for `includeIf` blocks |
| `go-git/v5/config` for gitconfig write | No `includeIf` support (issue #388, closed stale); `Raw` field doesn't guarantee comment preservation on re-write | Same as above |
| `go-git/v6` (alpha) | Alpha pre-release as of June 2026; not stable | go-git/v5.19.1 if any go-git functionality is needed; but gitid does not use go-git for its core workflow |
| `charmbracelet/bubbletea` (v1, github.com import) | v1 is superseded; import path `github.com/charmbracelet/bubbletea` now points to archived/old API | `charm.land/bubbletea/v2` |
| `charmbracelet/lipgloss` without v2 prefix | v1 and v2 have incompatible APIs; v1 import paths exist but v2 is the stable target | `charm.land/lipgloss/v2` |
| `golangci-lint` installed via `go install` | Go version mismatch can cause silently wrong behavior; binary install is the project's documented recommendation | Binary install via `install.sh` script |
| `pbcopy` hardcoded for clipboard | macOS-only; breaks on Linux | `github.com/atotto/clipboard` dispatches correctly |

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| charm.land/bubbletea/v2 v2.0.7 | charm.land/lipgloss/v2, charm.land/bubbles/v2 | All three are v2 — mismatching v1 and v2 of any pairing will cause type errors |
| github.com/spf13/cobra v1.10.2 | github.com/spf13/pflag (bundled) | pflag is a transitive dep, do not add separately |
| golang.org/x/crypto v0.53.0 | Go 1.23+ | Uses crypto/ed25519 from stdlib; min Go version constraint aligns |
| golangci-lint v2.12.2 | Go 1.23+ | v2 config format requires `version: "2"` key; v1 configs will error |

## Sources

- `pkg.go.dev/github.com/kevinburke/ssh_config` — version v1.6.0, MarshalText, String method confirmed (HIGH)
- `github.com/kevinburke/ssh_config/blob/master/config.go` — rawValue round-trip design, whitespace TODO confirmed (HIGH)
- `github.com/kevinburke/ssh_config/issues` — issue #74 (add/edit/delete hosts) open; no lossy-String issue (HIGH)
- `pkg.go.dev/golang.org/x/crypto/ssh` — MarshalAuthorizedKey, MarshalPrivateKey, NewPublicKey confirmed at v0.53.0 (HIGH)
- `github.com/go-git/go-git/issues/388` — includeIf not supported, closed stale (HIGH)
- `pkg.go.dev/github.com/gopasspw/gopass/pkg/gitconfig` — includeIf explicitly unsupported (HIGH)
- `github.com/charmbracelet/bubbletea/releases` — v2.0.7 latest stable, charm.land import path (HIGH)
- `github.com/charmbracelet/lipgloss/releases` — v2.0.3 stable, charm.land/lipgloss/v2 (HIGH)
- `github.com/charmbracelet/bubbles/releases` — v2.1.0 stable, charm.land/bubbles/v2 (HIGH)
- `pkg.go.dev/github.com/spf13/cobra` — v1.10.2, completion commands confirmed (HIGH)
- `golangci-lint.run/docs/welcome/install/local/` — v2.12.2, binary install recommended (HIGH)
- `ldez.github.io/blog/2025/03/23/golangci-lint-v2/` — v2 config format, linters.default, migrate command (HIGH)
- `go.dev/dl/` — Go 1.26.4 current stable (HIGH)

<!-- GSD:stack-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->
