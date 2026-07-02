# Project Research Summary

**Project:** gitid — SSH/Git Identity Manager
**Domain:** CLI + TUI multi-identity SSH/Git lifecycle manager (Go, single static binary, macOS/Linux)
**Researched:** 2026-06-08
**Confidence:** HIGH

## Executive Summary

`gitid` is a single-binary Go CLI + TUI tool that owns the full lifecycle of a developer's SSH and Git identities: key generation, `~/.ssh/config` stanza management, `~/.gitconfig` `includeIf` wiring, per-identity Git fragments, and `~/.ssh/allowed_signers` for SSH commit signing. The recommended approach is a thin Cobra CLI shell and Bubble Tea TUI over a UI-free, TDD-tested core (`internal/` packages) that treat the real config files as the sole source of truth — no sidecar database, no drift. Every mutation follows a strict parse → model → render → safe-write → verify pipeline, and nothing is written until a two-phase SSH test proves the resolved config is correct.

The stack is high-confidence: Go 1.23+ with Cobra v1.10.2 for CLI, the charm.land v2 suite (Bubble Tea v2.0.7 / Lipgloss v2.0.3 / Bubbles v2.1.0) for TUI — note the **charm.land vanity import paths**, not the old `github.com/charmbracelet/*` paths. For SSH config parsing, `github.com/kevinburke/ssh_config` v1.6.0 is the only actively maintained round-trip-safe Go parser. For gitconfig, **no Go library supports `includeIf` write-back** (go-git issue #388 closed stale; gopass explicitly documents the gap) — a custom sentinel-aware line parser and raw text write is required for `includeIf` blocks, while `git config` via `os/exec` handles ordinary key/value reads and writes.

The two key differentiators that justify the tool are: (1) a **two-phase test flow** — `ssh -i <key> -T <host>` (explicit key) then `ssh -T <alias>` + `ssh -G <alias>` (resolved config) — that no competitor implements, catching `IdentitiesOnly`-missing and wrong-identity bugs at config time rather than at `git push` time; and (2) a **deep doctor** that checks coherence across all four artifacts (SSH stanza, gitconfig block, fragment, `allowed_signers`) including orphan detection, signing wiring, and agent state. The primary risks are config file corruption from non-atomic writes or blind appends — both addressed by the shared `filewriter` package (backup → write-to-temp → atomic rename → chmod) and sentinel-delimited managed blocks.

## Key Findings

### Recommended Stack

The core runtime stack is Go 1.23+, Cobra, and the charm.land v2 TUI suite. All three charm.land packages changed import paths from `github.com/charmbracelet/*` to `charm.land/*/v2` — using the old paths imports the deprecated v1 API with incompatible types. The gitconfig write problem is solved by shelling out to `git config` for standard key/value pairs, and writing `includeIf` and `insteadOf` blocks as raw managed-block text. Quality tooling (golangci-lint v2 binary install, gosec, staticcheck, pre-commit hooks wired to `make` targets) must be bootstrapped before any feature code.

**Core technologies:**
- **Go 1.23+:** language; minimum for `go.mod` toolchain directive
- **`charm.land/bubbletea/v2` v2.0.7:** TUI event loop (Elm MVU); vanity import path — not github.com
- **`charm.land/lipgloss/v2` v2.0.3:** TUI styling; v2 pairs with Bubble Tea v2; v1/v2 types are incompatible
- **`charm.land/bubbles/v2` v2.1.0:** TUI components (list, textinput, spinner); same v2 pairing requirement
- **`github.com/spf13/cobra` v1.10.2:** CLI; built-in `completion` subcommand for bash/zsh/fish/PowerShell
- **`github.com/kevinburke/ssh_config` v1.6.0:** only maintained round-trip-safe Go SSH config parser; `Match` directive unsupported (not needed)
- **`golang.org/x/crypto/ssh` v0.53.0:** ed25519 key gen, OpenSSH serialization, `allowed_signers` line — no third-party key library needed
- **`os/exec` (stdlib):** gitconfig reads/writes via `git config`; `includeIf`/`insteadOf` via raw managed-block text
- **`github.com/atotto/clipboard` v0.1.4:** cross-platform clipboard dispatch (pbcopy/wl-copy/xclip)
- **`golangci-lint` v2.12.2:** binary install (not `go install`); config requires `version: "2"` key; includes gosec, staticcheck, errcheck, unused

**Development toolchain (Phase 0 — before feature code):**
- `Makefile`: `setup-env`, `build`, `install`, `uninstall`, `test`, `lint`, `fmt`
- golangci-lint v2.12.2 binary install; `.golangci.yml` with `linters.default: none` + explicit enable list
- pre-commit hooks wired to `make` targets (not upstream hook repos — prevents CI divergence)
- TDD harness: `go test -race -coverprofile=coverage.out ./...`

### Expected Features

The competitive landscape splits into profile switchers (gitp, git-ego: no SSH ownership, no test flow) and SSH-aware managers (bgit, gitch: write SSH config but no two-phase test, no doctor, no idempotent blocks). `gitid` occupies the gap with lifecycle ownership and safety guarantees neither category provides.

**Must have (table stakes) — Phase 1:**
- Identity CRUD with name, email, provider, alias, key path
- ed25519 key generation (one key per identity, auth + signing)
- `~/.ssh/config` idempotent managed-block write: `IdentitiesOnly yes`, port 443 defaults, `IgnoreUnknown UseKeychain` guard
- `~/.gitconfig` `includeIf` block write (`gitdir:` and `hasconfig:` strategies)
- Per-identity fragment (`~/.gitconfig.d/<name>.gitconfig`) with signing wiring
- `~/.ssh/allowed_signers` entry management
- Timestamped backup before any mutation
- SSH authentication test after config write
- Shell completion (Cobra generates this; near-zero cost)

**Should have (differentiators) — Phase 1:**
- **Two-phase test flow:** `ssh -i <key> -T <host>` then `ssh -T <alias>` + `ssh -G <alias>` — unique in category; blocks every write
- **Deep doctor:** deps, permissions, coherence/drift, orphan detection, signing wiring, agent state — most diagnostic depth in category
- Key rotation with artifact re-pointing + test re-run
- Clipboard copy of public key (on generate and on demand)
- Contextual upload instructions (GitHub/GitLab auth + signing key registration)
- TUI doctor dashboard as home screen (Bubble Tea)

**Defer to Phase 2:**
- `insteadOf` HTTPS→SSH rewriting
- `gitid add repo <url>` workflow (requires full Phase 1 identity foundation)
- Adopt existing plain-style fragments
- Global/shared git config toggles

**Anti-features (excluded):** Windows, GPG signing, web UI, scheduled key rotation, secret-vault integration, HTTPS credential management.

### Architecture Approach

Strict layered architecture: `cmd/gitid/` (Cobra, thin — no logic), `tui/` (Bubble Tea, UI state only), `internal/` (all domain logic, UI-free, independently testable). Dependency arrow is one-directional: entrypoints import `internal/`; `internal/` never imports `cmd/` or `tui/`. The shared `filewriter` package is used by both `sshconfig.writer` and `gitconfig.writer`, ensuring identical safe-write behavior. Filesystem is the only source of truth — identities reconstructed at startup from sentinel-delimited managed blocks.

**Major components (dependency-driven build order):**
1. **`platform` / `deps`** — OS detection, tool availability; leaf packages; no domain dependencies
2. **`filewriter`** — backup, atomic write, chmod, restore; shared by all writers
3. **`keygen` / `clipboard`** — ed25519 key pair, clipboard dispatch
4. **`sshconfig` / `gitconfig`** (parser + renderer + writer) — custom line parser required for gitconfig `includeIf`
5. **`identity`** — Account struct, CRUD, reconstruction from parsed managed blocks
6. **`tester` / `doctor`** — two-phase SSH test, health check orchestration; read-only
7. **`cmd/gitid/`** — Cobra wiring, integration tests
8. **`tui/`** — Bubble Tea models, doctor dashboard, identity forms

**Key patterns:** sentinel-delimited managed blocks; parse → model → render → safe-write → verify on every mutation; all TUI calls to `internal/` wrapped in `tea.Cmd`; Cobra handlers ≤30 lines.

### Critical Pitfalls

1. **Blind appends / grep-guard** — use sentinel-delimited managed blocks; always scan for sentinel pair and replace atomically; second run must produce identical file (idempotency check)
2. **Missing `IdentitiesOnly yes`** — with 7+ keys in ssh-agent, GitHub's `MaxAuthTries` exhausts before the correct key is offered; every aliased `Host` block must emit `IdentitiesOnly yes`; verify with `ssh -G <alias> | grep identitiesonly`
3. **Non-atomic writes** — `os.WriteFile` truncates in place; always write-to-temp then `os.Rename`; `os.Chmod` after rename; timestamped backup before any mutation
4. **`IgnoreUnknown UseKeychain` placement** — `UseKeychain yes` is Apple-patched; Linux rejects it without the guard; `IgnoreUnknown UseKeychain` must be within the same `Host *` block before `UseKeychain yes`; `Host *` must be after all specific host blocks (first-match-wins)
5. **`allowed_signers` format and email mismatch** — correct format: `user@example.com namespaces="git" ssh-ed25519 AAAA...`; email is case-sensitive; `namespaces="git"` is mandatory; `gpg.ssh.allowedSignersFile` must be set per-fragment; `user.signingkey` must be a file path, not an inline literal

**Documented caveats:**
- **Azure DevOps requires RSA** — ed25519-only is a v1 limitation; document clearly; Azure DevOps not supported
- **`hasconfig:remote.*.url` requires Git 2.36+** — doctor must check Git version; fallback to `gitdir:` or warn
- **`hasconfig:` included files cannot declare remote URLs** — fragments must never contain `[remote]` sections; validate at render time
- **`gitdir:` trailing slash required** — `includeIf "gitdir:~/git/client/"` (with slash) matches all repos under directory
- **`atotto/clipboard` maintenance** — functional but minimally maintained; fail gracefully if no clipboard tool found

## Implications for Roadmap

### Phase 0: Bootstrap (Makefile + Toolchain + TDD Harness)
**Rationale:** Quality toolchain must exist before feature code; golangci-lint v2 binary install is a one-time step that prevents Go version mismatch; pre-commit hooks wired to `make` targets establish the CI source of truth from day one.
**Delivers:** `Makefile`, `go.mod`, `.golangci.yml` (`version: "2"`, explicit linter list), `.pre-commit-config.yaml` wired to `make` targets, CI skeleton
**Avoids:** golangci-lint v1/v2 config incompatibility; local/CI hook divergence; unchecked file I/O errors; file permission security misses

### Phase 1: Foundation Infrastructure (filewriter + platform + deps)
**Rationale:** `filewriter` is a shared dependency of every config-writing package; building and fully testing it first ensures both `sshconfig` and `gitconfig` writers inherit identical behavior. `platform` and `deps` are leaf packages with no domain dependencies.
**Delivers:** `internal/filewriter` (backup, atomic write, chmod, restore); `internal/platform` (OS detection, UseKeychain guard); `internal/deps` (ssh, git, ssh-keygen, clipboard tool availability)
**Avoids:** Non-atomic writes (Pitfall 7); permission errors from umask reliance (Pitfall 6); Linux crash from macOS-only SSH directives (Pitfall 4)

### Phase 2: Primitive Operations (keygen + sshconfig + gitconfig parsers/renderers)
**Rationale:** Pure, stateless operations — ideal for TDD; testing renderers without file I/O gives fast coverage; custom gitconfig line parser is bounded in scope (sentinel boundary extraction only, not full spec).
**Delivers:** `internal/keygen`; `internal/sshconfig/{parser,renderer}`; `internal/gitconfig/{parser,renderer,fragment}`; `internal/clipboard`
**Avoids:** `gitdir:` trailing slash missing (Pitfall 8); `hasconfig:` remote URL in fragment (Pitfall 9); `IdentitiesOnly yes` omission (Pitfall 2); `User git` omission (Pitfall 12); `Host *` ordering (Pitfall 5); `allowed_signers` format errors (Pitfall 10); inline `user.signingkey` (Pitfall 11)

### Phase 3: Config Writers + Identity Model
**Rationale:** Writers compose parsers + renderers + filewriter from prior phases. Identity model (CRUD + reconstruction) is the aggregation layer; must come after both config packages exist so the loader can merge SSH and gitconfig state.
**Delivers:** `internal/sshconfig/writer`; `internal/gitconfig/writer`; `internal/identity` (Account struct, CRUD, loader/merger)
**Avoids:** Blind appends (Pitfall 1); block duplication on re-run; round-trip instability

### Phase 4: Tester + Doctor
**Rationale:** Both differentiators depend on the full identity model and all config packages. Both are read-only — safe to develop and test independently. Doctor is the quality gate that proves the whole artifact set is coherent.
**Delivers:** `internal/tester` (two-phase SSH test); `internal/doctor` + checks (permissions, coherence, orphans, signing)
**Research flag:** Doctor check scope (orphan detection, signing verification, `ssh -G` output format across OpenSSH versions) warrants a research pass during phase planning

### Phase 5: CLI Entrypoint (Cobra + integration tests)
**Rationale:** With all internal packages tested, Cobra wiring is mechanical. Handlers are thin. Integration tests (end-to-end create → test → list → delete) live here.
**Delivers:** `cmd/gitid/` (add, list, edit, delete, rotate, test, doctor, copy, completion); integration test suite; shell completion
**Standard patterns:** Cobra v1 API is stable and well-documented — skip research-phase

### Phase 6: TUI (Bubble Tea doctor dashboard + identity forms)
**Rationale:** TUI is the last layer; imports `internal/` but adds no domain logic. All calls to internal packages are wrapped in `tea.Cmd`. Doctor dashboard as home screen turns `gitid` into a persistent tool.
**Delivers:** `tui/` (doctor dashboard, identity list, add/edit form, Lipgloss styles); `gitid` with no args enters TUI mode
**Research flag:** Bubble Tea v2 renderer and `tea.Cmd` concurrency patterns for slow operations (key gen, SSH test) warrant a focused research pass

### Phase 7: Phase 2 Features (insteadOf + add repo + fragment adoption)
**Rationale:** Phase 2 features require the full Phase 1 identity lifecycle to be validated by real users. `add repo` depends on identity store, SSH aliases, `insteadOf`, and `includeIf` fragments all proven stable.
**Delivers:** `insteadOf` HTTPS→SSH managed blocks; `gitid add repo <url>`; fragment adoption migration; global git config toggles
**Research flag:** `insteadOf` interaction with SSH alias vs real hostname needs verification during planning

### Phase Ordering Rationale

- `filewriter` before all config writers — shared safe-write concern must exist before either config package delegates to it
- Parsers/renderers before writers — pure function layer is fully testable without filesystem; bottom-up avoids stubs
- `identity` after both config packages — loader merges SSH and gitconfig state; requires both parsers
- `tester` + `doctor` after `identity` — both require `[]Account` and full config package set
- TUI last — imports everything; no domain logic; deferred to keep CI test suite fast throughout development
- Phase 2 deferred — `add repo` depends on Phase 1 being proven stable; keeps MVP lean

### Research Flags

Needs research during planning:
- **Phase 4 (Doctor):** `ssh -G` output format across OpenSSH versions; `git verify-commit` behavior; orphan detection edge cases
- **Phase 6 (TUI):** Bubble Tea v2 renderer; `tea.Cmd` concurrency for slow operations
- **Phase 7 (insteadOf):** `insteadOf` interaction with SSH alias vs real hostname; `hasconfig:` pattern for alias-form remotes

Standard patterns (skip research-phase):
- **Phase 0 (Bootstrap):** golangci-lint v2 config is fully documented
- **Phase 1 (filewriter/platform/deps):** atomic write pattern is well-documented; stdlib only
- **Phase 2 (keygen/parsers):** ed25519 + x/crypto is well-documented; kevinburke/ssh_config API verified
- **Phase 5 (CLI/Cobra):** Cobra v1 API is stable and extensively documented

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions verified against pkg.go.dev and official release pages; charm.land import paths confirmed; gitconfig library gap confirmed against go-git issue tracker and gopass docs |
| Features | HIGH | PRD requirements explicit; competitive landscape verified across 6+ tools; feature dependencies mapped |
| Architecture | HIGH | Build order derived from actual import dependencies; patterns are well-established; component boundaries enforced by Go `internal/` |
| Pitfalls | HIGH | All 12 pitfalls verified against official docs and multiple sources |

**Overall confidence:** HIGH

### Gaps to Address

- **Azure DevOps / RSA scope:** validate whether Azure DevOps support is needed before Phase 1 requirements lock
- **`hasconfig:` Git version floor:** decide fallback UX (warn + continue vs hard stop) during planning; doctor must check Git 2.36+
- **`atotto/clipboard` runtime fallback:** design explicit UX for "no clipboard tool found" during planning
- **`kevinburke/ssh_config` whitespace normalization:** verify against the user's actual `~/.ssh/config` format during testing — minor normalization is acceptable given the managed-block approach
- **`ssh -G` output format stability:** confirm defensive parsing approach (key-based, not position-based) during Phase 4 planning

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/github.com/kevinburke/ssh_config` v1.6.0 — round-trip API, rawValue design, whitespace TODO confirmed
- `pkg.go.dev/golang.org/x/crypto/ssh` v0.53.0 — MarshalAuthorizedKey, MarshalPrivateKey, NewPublicKey confirmed
- `github.com/go-git/go-git/issues/388` — includeIf not supported, closed stale
- `pkg.go.dev/github.com/gopasspw/gopass/pkg/gitconfig` — includeIf explicitly unsupported
- `github.com/charmbracelet/bubbletea/releases` — v2.0.7, charm.land import path confirmed
- `github.com/charmbracelet/lipgloss/releases` — v2.0.3, charm.land/lipgloss/v2 confirmed
- `github.com/charmbracelet/bubbles/releases` — v2.1.0, charm.land/bubbles/v2 confirmed
- `pkg.go.dev/github.com/spf13/cobra` — v1.10.2, completion commands confirmed
- `golangci-lint.run` — v2.12.2, binary install, `version: "2"` config key
- `git-scm.com/docs/git-config` — includeIf gitdir trailing slash, hasconfig restrictions, remote URL prohibition in included files
- `docs.github.com` — port 443 `User git` requirement; SSH key registration (auth vs signing)
- SSH commit signing `allowed_signers` format + `namespaces="git"`

### Secondary (MEDIUM confidence)
- `golang-standards/project-layout` — cmd/internal/ rationale (community standard, not official)
- `charmbracelet/bubbletea` docs — MVU architecture and `tea.Cmd` patterns
- atomic file write pattern in Go (community references)
- Competitive landscape (bgit, gitch, git-ego, gitp, MultiKey CLI) — README/description analysis

### Tertiary (LOW confidence)
- Azure DevOps ed25519 limitation — via WebSearch summary; needs direct validation if Azure DevOps support is scoped
- `atotto/clipboard` maintenance status — inferred from commit activity

---
*Research completed: 2026-06-08*
*Ready for roadmap: yes*
