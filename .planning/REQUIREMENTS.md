# gitid — Requirements

Derived from the PRD (`docs/prds/ssh-git-identity-manager-v1.0-prd.md`), project
research (`.planning/research/`), and resolved init decisions (tool name `gitid`,
clone base `~/git`, Phase-1 cut deferring `insteadOf`/`add repo` to v2).

REQ-ID format: `[CATEGORY]-[NUMBER]`. v1 = the Phase-1 MVP scope.

## v1 Requirements

### Project Tooling & Standards (TOOL)

- [ ] **TOOL-01**: A `Makefile` exposes `setup-env`, `build`, `install`, `uninstall`, `test`, `lint`, `fmt` targets
- [ ] **TOOL-02**: `make setup-env` bootstraps the dev environment (installs golangci-lint, gosec, pre-commit, and the git hooks)
- [ ] **TOOL-03**: pre-commit hooks run format, lint, security, and tests by invoking the same `make` targets CI uses
- [ ] **TOOL-04**: Core logic is built test-first (TDD); config parse→render→parse is round-trip stable (proven by tests)

### Identity & Account CRUD (IDENT)

- [ ] **IDENT-01**: User can create an identity (name, git name, git email) that generates an ed25519 key used for both authentication and signing
- [ ] **IDENT-02**: User can create an identity that reuses an existing key instead of generating a new one
- [ ] **IDENT-03**: User can list identities and accounts with their wiring (key path, alias, provider, port, match strategy)
- [ ] **IDENT-04**: User can update an identity's name/email, signing on/off, provider/alias/port, and match strategy
- [ ] **IDENT-05**: User can delete an identity/account — its managed blocks are removed (key optional) with confirmation and backup
- [ ] **IDENT-06**: An account maps an identity to a provider via a host alias, so several identities can share one provider
- [ ] **IDENT-07**: On startup the tool reconstructs the identity/account list by parsing its managed blocks (no sidecar database)

### Key Management (KEY)

- [ ] **KEY-01**: User can rotate/replace the key for an existing identity; artifacts re-point to the new key and the test flow re-runs
- [x] **KEY-02**: Generated keys and files receive correct permissions (`~/.ssh` 700, private key 600, `.pub` 644, `config` 600)

### SSH Config Artifact (SSH)

- [ ] **SSH-01**: Creating an account writes a managed `Host <alias>` block with `Hostname`, `Port`, `User git`, `IdentityFile`, and `IdentitiesOnly yes`
- [ ] **SSH-02**: A provider's default identity may use the real host (`github.com`); additional identities use aliases (`work.github.com`)
- [ ] **SSH-03**: On macOS a `Host *` block emits `UseKeychain yes` + `AddKeysToAgent yes` guarded by `IgnoreUnknown UseKeychain`; this block is ordered after specific hosts

### Git Config Artifact (GIT)

- [ ] **GIT-01**: Creating an account writes a managed `[includeIf "<match>"]` block in `~/.gitconfig` pointing to the identity's fragment
- [ ] **GIT-02**: The match strategy supports `gitdir:` (default suggestion, with trailing slash) and `hasconfig:remote.*.url`, combinable per account
- [ ] **GIT-03**: A per-identity fragment (`~/.gitconfig.d/<identity>`) sets `user.name`/`user.email`, `gpg.format=ssh`, `user.signingkey`, `commit.gpgsign true`

### Signing (SIGN)

- [ ] **SIGN-01**: A signing identity gets an `~/.ssh/allowed_signers` line in the form `<email> namespaces="git" ssh-ed25519 AAAA…` (email byte-identical to `user.email`)
- [ ] **SIGN-02**: `user.signingkey` references the public-key file path, never an inline key literal (survives rotation)

### Two-Phase Test Flow (TEST)

- [ ] **TEST-01**: Before writing, an explicit test runs `ssh -i <key> -o IdentitiesOnly=yes -T git@<host>`, proving the key authenticates
- [ ] **TEST-02**: After writing, a resolved test runs `ssh -T git@<alias>` plus `ssh -G <alias>` to prove which `IdentityFile` the config actually resolved
- [ ] **TEST-03**: Every test prints both the command run (input) and its real output

### Safe Writes (SAFE)

- [x] **SAFE-01**: Every mutation creates a timestamped backup before writing (e.g. `~/.ssh/config.bak.<ts>`)
- [x] **SAFE-02**: Writes use an idempotent whole-block rewrite of sentinel-delimited blocks (never blind append); content outside managed blocks is preserved verbatim
- [x] **SAFE-03**: Writes are atomic (write-to-temp → rename → chmod) and no write path proceeds without explicit confirmation

### Doctor (DOC)

- [ ] **DOC-01**: `gitid doctor` checks dependencies (`ssh`, `ssh-keygen`, `ssh-add`, `git`, clipboard tool) with per-OS install hints (brew / apt / dnf / pacman)
- [ ] **DOC-02**: Doctor checks permissions on `~/.ssh`, keys, `.pub`, and `config`
- [ ] **DOC-03**: Doctor checks coherence/drift — every `IdentityFile` resolves, every `includeIf` points to an existing fragment, `IdentitiesOnly yes` is present, signing identities have an `allowed_signers` line
- [ ] **DOC-04**: Doctor detects orphans — unused keys, non-included fragments, aliases without a matching `includeIf`
- [ ] **DOC-05**: Doctor checks signing wiring (`gpg.format=ssh`, `allowed_signers` path) and `ssh-agent` status (running, keys loaded), and warns if `git < 2.36` when `hasconfig:` is used
- [ ] **DOC-06**: Each finding has severity + explanation + suggested fix (auto-fix offered with confirmation)
- [ ] **DOC-07**: Doctor runs first when the TUI launches, and is available as `gitid doctor` on the CLI

### Clipboard (CLIP)

- [ ] **CLIP-01**: The public key is copied to the clipboard when generated and on demand when reusing an identity
- [ ] **CLIP-02**: Clipboard support is cross-platform (`pbcopy` macOS; `wl-copy`/`xclip` Linux) and fails gracefully when no tool is found

### Upload Instructions (UP)

- [ ] **UP-01**: For GitHub/GitLab, the tool shows concrete steps to add the public key for **authentication**
- [ ] **UP-02**: For GitHub/GitLab, the tool shows concrete steps to add the public key for **signing**

### CLI (CLI)

- [ ] **CLI-01**: A Cobra CLI exposes the Phase-1 surface: `doctor`, `identity add/list/test`, `host add`
- [ ] **CLI-02**: The CLI generates shell completion for bash, zsh, and fish

### TUI (TUI)

- [ ] **TUI-01**: A Bubble Tea TUI launches into the doctor dashboard
- [ ] **TUI-02**: From the dashboard the user can navigate to the identity/account managers

## v2 Requirements (deferred)

- [ ] **GLOBAL-01**: Global/shared git config toggles (`push.autoSetupRemote`, `core.ignorecase`, `pull.rebase`, `fetch.prune`, aliases, color)
- [ ] **URLRW-01**: When an SSH host/alias is added, suggest the HTTPS equivalent and let the user edit it before generating the `insteadOf` rewrite
- [ ] **ADOPT-01**: Detect plain-style fragments (`~/.gitconfig_personal`, etc.) and offer to reference or migrate them into `~/.gitconfig.d/`
- [ ] **REPO-01**: `gitid add repo <url>` detects the provider, asks personal/client (candidates from `~/git/<client>` folders and existing accounts), rewrites the clone URL to the alias, clones into `~/git/<client>`, and verifies with a `git -C <dest> pull` (output shown)
- [ ] **AUTOUP-01** (Phase 3): Automatic key upload via `gh`/`glab` when present

## Out of Scope

- **Windows** — v1 targets macOS + Linux only; SSH/keychain/clipboard behavior diverges
- **GPG commit signing** — replaced by ssh-key signing (`gpg.format=ssh` + `allowed_signers`)
- **Web UI** — terminal-first; CLI + TUI cover the use case
- **Scheduled / automatic key rotation** — rotation is user-initiated only
- **Secret-vault integration** — keys live in `~/.ssh` with correct permissions
- **Azure DevOps** — requires RSA keys; gitid is ed25519-only (documented limitation; architecture leaves key-type as a future per-identity field)

## User Stories

- As a developer with personal + multiple client identities, I can create a new identity and have my SSH and Git config wired coherently in one step, so I stop hand-editing four files.
- As a developer, I can prove a new identity authenticates and resolves to the right key (`ssh -G`) *before* anything is written, so I never push with the wrong identity.
- As a developer, I can run `gitid doctor` and get a clear, fixable report of what's wrong across deps, permissions, drift, orphans, and signing.
- As a developer whose key was exposed, I can rotate it and have every artifact re-pointed and re-tested automatically.

## Acceptance Criteria

### Functional

- [ ] Create an identity end-to-end; the four artifacts are written with backup + confirmation; both test phases pass and show input + output
- [ ] Two identities on the same provider coexist via distinct aliases and each resolves to its own key (`ssh -G` proof)
- [ ] Rotate a key: artifacts re-point to the new key and the resolved test passes
- [ ] Delete an identity: managed blocks removed, files outside blocks intact
- [ ] Public key copied to clipboard on generate and on demand (per-OS)
- [ ] `gitid doctor` reports deps, permissions, drift, orphans, signing, and agent status, each with a suggested fix; runs first in the TUI

### Quality

- [ ] Core has unit tests written test-first; config parse/render is round-trip safe
- [ ] No write path lacks a backup + confirmation
- [ ] All generated content is in English
- [ ] `make lint` (golangci-lint + gosec) and `make test` pass; pre-commit hooks enforce them

### User acceptance

- [ ] Existing hand-written config outside managed blocks is preserved
- [ ] Upload steps are clear enough to add a key without external docs

## Definition of Done

- All v1 requirements above are implemented and traced to a phase
- Acceptance criteria pass with observable test evidence (input + output shown for SSH tests)
- `make test` and `make lint` are green; pre-commit hooks installed via `make setup-env`
- No mutation path lacks backup + idempotent managed-block write + confirmation
- Commit history for the milestone is clean and compacted (squashed at each plan close + user review)

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| TOOL-01 | Phase 1 | Pending |
| TOOL-02 | Phase 1 | Pending |
| TOOL-03 | Phase 1 | Pending |
| TOOL-04 | Phase 1 | Pending |
| IDENT-01 | Phase 2 | Pending |
| IDENT-02 | Phase 2 | Pending |
| IDENT-06 | Phase 2 | Pending |
| KEY-01 | Phase 2 | Pending |
| KEY-02 | Phase 2 | Complete |
| SSH-01 | Phase 2 | Pending |
| SSH-02 | Phase 2 | Pending |
| SSH-03 | Phase 2 | Pending |
| GIT-01 | Phase 2 | Pending |
| GIT-02 | Phase 2 | Pending |
| GIT-03 | Phase 2 | Pending |
| SIGN-01 | Phase 2 | Pending |
| SIGN-02 | Phase 2 | Pending |
| TEST-01 | Phase 2 | Pending |
| TEST-02 | Phase 2 | Pending |
| TEST-03 | Phase 2 | Pending |
| SAFE-01 | Phase 2 | Complete |
| SAFE-02 | Phase 2 | Complete |
| SAFE-03 | Phase 2 | Complete |
| CLIP-01 | Phase 2 | Pending |
| CLIP-02 | Phase 2 | Pending |
| UP-01 | Phase 2 | Pending |
| UP-02 | Phase 2 | Pending |
| IDENT-03 | Phase 3 | Pending |
| IDENT-04 | Phase 3 | Pending |
| IDENT-05 | Phase 3 | Pending |
| IDENT-07 | Phase 3 | Pending |
| DOC-01 | Phase 4 | Pending |
| DOC-02 | Phase 4 | Pending |
| DOC-03 | Phase 4 | Pending |
| DOC-04 | Phase 4 | Pending |
| DOC-05 | Phase 4 | Pending |
| DOC-06 | Phase 4 | Pending |
| DOC-07 | Phase 4 | Pending |
| CLI-01 | Phase 5 | Pending |
| CLI-02 | Phase 5 | Pending |
| TUI-01 | Phase 5 | Pending |
| TUI-02 | Phase 5 | Pending |
