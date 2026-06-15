# Stack Research

**Domain:** CLI + TUI SSH/Git identity manager (Go, single static binary, macOS/Linux)
**Researched:** 2026-06-08
**Confidence:** HIGH (core stack verified against official sources; medium on one gitconfig strategy)

---

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

---

## Area-by-Area Rationale

### 1. `~/.ssh/config` parsing: kevinburke/ssh_config

**Use: `github.com/kevinburke/ssh_config` v1.6.0**

This is the only actively maintained Go library that explicitly targets ssh_config comment-preserving round-trip fidelity. Published February 2026. Sponsored by Tailscale and Indeed.

**Round-trip assessment (MEDIUM confidence with caveats):**
- `Config.String()` and `Config.MarshalText()` are the render path
- The parser stores `rawValue` (preserving quoted values), `leadingSpace`, and per-node comments explicitly to support round-trip
- Known limitation: mixed tabs/spaces handling has a `TODO` in the source — minor whitespace normalization possible on re-write
- The `Match` directive is unsupported (not needed for gitid's use case — gitid only writes `Host`, `IdentityFile`, `IdentitiesOnly`, `Hostname`, `Port`, `User`, and global `Host *` directives)
- Issue #74 is open for "add/edit/delete hosts while preserving original content" — the current API requires traversing `cfg.Hosts[].Nodes` directly; there is no high-level `SetKey()` method
- **Conclusion: suitable for gitid.** The round-trip caveats (minor whitespace) are acceptable because gitid uses sentinel-delimited managed blocks: it only rewrites the blocks it owns and leaves everything else untouched. Parse → modify managed block → `cfg.String()` → write is stable enough.

**What gitid must do to be safe:**
- Never call a generic `SetKey(host, key, value)` across the whole file
- Only rewrite the `Host` nodes inside managed block boundaries
- Validate post-write with a second `Decode` pass (parse → render → parse stability check)

**Alternative rejected:** No other actively maintained Go ssh_config parser exists. Rolling a custom parser would require supporting all directives and is unjustifiable.

### 2. `~/.gitconfig` parsing and writing

**Use: shell out to `git config` via `os/exec`**

**Why not a pure-Go library:**

| Library | includeIf support | insteadOf write | Comment-safe | Verdict |
|---------|-------------------|-----------------|--------------|---------|
| `go-git/v5/config` | No (issue #388, closed as stale) | Partial struct support only | `Raw` field preserves some structure, but not guaranteed for comments | Rejected — no includeIf write support |
| `gopasspw/gopass/pkg/gitconfig` | No (explicitly documented) | Unknown | No (whitespace normalization admitted) | Rejected — missing core feature |
| `muja/goconfig` | Unknown — unmaintained | Unknown | Unknown | Rejected — unmaintained |
| `git config` via `os/exec` | Full (git handles it natively) | Full | N/A — git owns the file | **Selected** |

**Rationale for shelling out:**
- `git config --file ~/.gitconfig --add section.key value` is idempotent-safe and comment-preserving by design — git is the authoritative parser of its own format
- The tool already has `git` as a required runtime dependency (it tests SSH with `ssh -T`; the doctor checks `git` presence)
- `git config --get-regexp` and `--list` are reliable for reading managed blocks' keys
- The `os/exec` approach does not introduce external-process security risks when called without shell expansion (stdlib `os/exec` intentionally bypasses shell)

**What gitid must own as raw text writes (bypassing `git config`):**
- `[includeIf "gitdir:..."]` and `[includeIf "hasconfig:remote.*.url:..."]` blocks — `git config` cannot write `includeIf` section headers natively; it can read them but not create them
- `[url "..."] insteadOf = ...` blocks (Phase 2)
- These are written as sentinel-delimited managed block text, appended/replaced in the file, not line-by-line via `git config` set

**Managed-block text approach for includeIf:**
The tool writes a clearly delimited block:
```
# BEGIN gitid managed: <identity-name>
[includeIf "gitdir:~/git/client/"]
	path = ~/.gitconfig.d/client.gitconfig
# END gitid managed: <identity-name>
```
Parse the whole file as text, find the sentinel lines, replace the block. This is the only safe approach given no library supports `includeIf` writes.

**Confidence:** MEDIUM — shelling out to `git` is pragmatic but requires `git` to be installed (acceptable: gitid is a git-identity tool and git is table-stakes). The managed-block text-write for `includeIf` is a known pattern used by other tools (e.g., Homebrew, direnv).

### 3. Ed25519 key generation + OpenSSH formatting + `allowed_signers`

**Use: `golang.org/x/crypto/ssh` v0.53.0 — no third-party library needed**

Full workflow is stdlib + x/crypto:

```go
// Key generation
pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)  // crypto/ed25519

// Private key → OpenSSH PEM
privPEM, _ := ssh.MarshalPrivateKey(privKey, comment)   // x/crypto/ssh
pem.Encode(privFile, privPEM)

// Public key → authorized_keys format
sshPub, _ := ssh.NewPublicKey(pubKey)
authorizedLine := ssh.MarshalAuthorizedKey(sshPub)       // "ssh-ed25519 AAAA... comment\n"

// allowed_signers line format: "<email> namespaces=\"git\" <ssh-ed25519 AAAA...>"
// Build this manually from MarshalAuthorizedKey output — no library needed
```

The `allowed_signers` line is not a separate format requiring a library: it is `email namespaces="git" ` prepended to the `authorized_keys` line (without trailing newline) plus a newline. Build it as a string.

**Confidence:** HIGH — all functions verified against official `pkg.go.dev` documentation for `golang.org/x/crypto` v0.53.0.

### 4. Cobra + shell completion

**Use: `github.com/spf13/cobra` v1.10.2**

- Import path: `github.com/spf13/cobra` (unchanged from v1)
- Completion: `rootCmd.InitDefaultCompletionCmd()` or let Cobra auto-register the `completion` subcommand, which handles `bash`, `zsh`, `fish`, and `powershell`
- No extra library needed for completion generation
- `pflag` is a transitive dependency (bundled with Cobra) — no separate import needed for flags

**Confidence:** HIGH — verified on pkg.go.dev.

### 5. Quality toolchain

**golangci-lint v2.12.2**

Install via binary (not `go install` — avoids Go version mismatch):
```bash
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.12.2
```

Minimum `.golangci.yml` for this project:
```yaml
version: "2"
linters:
  default: none
  enable:
    - govet        # correctness: misuse of sync, printf format, etc.
    - errcheck     # unchecked errors — critical for file I/O in ~/ mutations
    - staticcheck  # advanced static analysis (SA, S, ST rules)
    - gosec        # security: file permissions (G304/G306), exec injection (G204)
    - unused       # dead code detection
    - revive       # style linter (drop-in for golint)
    - misspell     # typos in comments/strings
    - goimports    # import formatting
  settings:
    gosec:
      excludes:
        - G115     # suppress uint conversion false positives if any
run:
  timeout: 5m
```

**Key v2 migration note:** `enable-all`/`disable-all` replaced by `linters.default: none|standard|all|fast`. Using `none` + explicit `enable` list gives maximum control and CI stability (a new linter release won't silently break the build).

**pre-commit integration**

`.pre-commit-config.yaml` (invokes `make` targets for single source of truth):
```yaml
repos:
  - repo: local
    hooks:
      - id: go-fmt
        name: gofmt + goimports
        language: system
        entry: make fmt
        pass_filenames: false
      - id: go-lint
        name: golangci-lint
        language: system
        entry: make lint
        pass_filenames: false
      - id: go-test
        name: go test
        language: system
        entry: make test
        pass_filenames: false
```

Using `repo: local` with `make` targets ensures hooks and CI share exactly the same invocation — no divergence between local hook behavior and CI behavior.

**Makefile minimum targets:**
```makefile
.PHONY: setup-env build install uninstall test lint fmt

setup-env:  ## Install tools + pre-commit hooks
    go install golang.org/x/tools/cmd/goimports@latest
    curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.12.2
    pre-commit install

fmt:
    goimports -w ./...
    gofmt -w ./...

lint:
    golangci-lint run ./...

test:
    go test -race -coverprofile=coverage.out ./...

build:
    go build -o bin/gitid ./cmd/gitid

install:
    go install ./cmd/gitid

uninstall:
    rm -f $(go env GOPATH)/bin/gitid
```

**Confidence:** HIGH for golangci-lint configuration; MEDIUM for pre-commit approach (local hooks with make targets is well-established but slightly more setup than using upstream hook repos).

---

## Installation

```bash
# Module init (greenfield)
go mod init github.com/castocolina/gitid   # adjust to actual module path

# Core TUI stack (charm.land vanity domain)
go get charm.land/bubbletea/v2
go get charm.land/lipgloss/v2
go get charm.land/bubbles/v2

# CLI
go get github.com/spf13/cobra@v1.10.2

# SSH config parser
go get github.com/kevinburke/ssh_config@v1.6.0

# Crypto (already a transitive dep via x/crypto; explicit pin)
go get golang.org/x/crypto@v0.53.0

# Clipboard
go get github.com/atotto/clipboard@v0.1.4

# Dev tools (not in go.mod)
go install golang.org/x/tools/cmd/goimports@latest
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.12.2
pip install pre-commit && pre-commit install
```

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| TUI framework | charm.land/bubbletea/v2 | tview, termdash | Not Elm-architecture; harder to test; bubbletea is the Go community standard for this style of TUI |
| gitconfig library | `git config` via os/exec | go-git/v5 config, gopass/gitconfig | Neither supports `includeIf` write; go-git issue #388 closed as stale; gopass explicitly documents the gap |
| SSH config parser | kevinburke/ssh_config | Custom parser, go-git's internal parser | Only maintained option; custom parser scope too large for this project |
| Crypto / key gen | golang.org/x/crypto/ssh | charmbracelet/keygen | x/crypto is authoritative and covers all needed operations; charmbracelet/keygen is a thin wrapper that adds a dependency without benefit |
| Lint aggregator | golangci-lint v2 | Running each linter separately | Parallel execution, unified config, unified output; v2 is current |
| Pre-commit hooks | Local hooks → make targets | TekWizely/pre-commit-golang upstream hooks | Upstream hooks bypass make, diverge from CI; local hooks + make = single source of truth |

---

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

---

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| charm.land/bubbletea/v2 v2.0.7 | charm.land/lipgloss/v2, charm.land/bubbles/v2 | All three are v2 — mismatching v1 and v2 of any pairing will cause type errors |
| github.com/spf13/cobra v1.10.2 | github.com/spf13/pflag (bundled) | pflag is a transitive dep, do not add separately |
| golang.org/x/crypto v0.53.0 | Go 1.23+ | Uses crypto/ed25519 from stdlib; min Go version constraint aligns |
| golangci-lint v2.12.2 | Go 1.23+ | v2 config format requires `version: "2"` key; v1 configs will error |

---

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

---
*Stack research for: gitid — SSH/Git identity manager CLI+TUI*
*Researched: 2026-06-08*
