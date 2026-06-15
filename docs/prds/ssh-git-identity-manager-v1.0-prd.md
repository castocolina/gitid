# SSH/Git Identity Manager — Product Requirements Document (PRD)

**Provisional tool name:** `gid` (*Git IDentity*) — see Open Decisions.
**Document version:** 1.0
**Date:** 2026-06-08
**Status:** Draft for user review

---

## 1. Background

### Business problem
Developers who juggle multiple Git identities (personal, several clients, work
GitHub/GitLab/Bitbucket) must keep three things consistent by hand:

- `~/.ssh/config` — which key authenticates to which host.
- `~/.gitconfig` — which identity (name/email/signing) applies to which repo.
- per-identity Git fragments and `~/.ssh/allowed_signers`.

Doing this manually is error-prone. The starting point for this project is a
monolithic Bash script (`ssh-keygen.sh`) that only generates an RSA key and
blindly appends blocks to `~/.ssh/config`, plus two reference gists describing
the desired end state (a conditional-include `~/.gitconfig` and a multi-alias
`~/.ssh/config`). None of them manage the lifecycle or keep the files coherent.

### Target users
A single developer on macOS or Linux managing several Git identities across
providers, who lives in the terminal.

### Value proposition
One tool that **owns the identity lifecycle** and keeps SSH + Git configuration
coherent, with a built-in **doctor** that detects and explains problems, and a
**hypothesis → test → implement** flow so nothing is written until it is proven
to work.

---

## 2. Domain model

**Identity** — the root entity.

| Field | Meaning |
|---|---|
| `name` | slug, e.g. `personal`, `acme` |
| `gitName`, `gitEmail` | committer identity |
| `keyPath` | `~/.ssh/id_ed25519_<name>` (ed25519) |
| `signing` | the same key is used to sign commits (auth + signing) |

**Account** — maps an Identity to a provider so the same provider can host
several identities.

| Field | Meaning |
|---|---|
| `provider` | `github.com`, `gitlab.com`, `bitbucket.org`, or custom/enterprise |
| `alias` | host alias that disambiguates, e.g. `personal.github.com` |
| `identity` | which Identity this account uses |
| `port` | `443` (firewall-friendly) or `22` |
| `match` | how Git selects this identity: `gitdir:` and/or `hasconfig:` |
| `rewriteHTTPS` | optional `insteadOf` HTTPS→SSH rewrite |

The default identity of a provider may use the real host (`github.com`);
additional identities use aliases (`work.github.com`, `acme.github.com`).

### Generated artifacts (per Account, coordinated)

| File | Content |
|---|---|
| `~/.ssh/config` | `Host <alias>` + `Hostname`/`Port`/`User git`/`IdentityFile`/`IdentitiesOnly yes` |
| `~/.gitconfig` | `[includeIf "<match>"] path = ~/.gitconfig.d/<identity>` |
| `~/.gitconfig.d/<identity>` | `user.name/email`, `gpg.format=ssh`, `user.signingkey`, `commit.gpgsign true` |
| `~/.ssh/allowed_signers` | `<email> ssh-ed25519 AAAA…` |

### Signing model
One **ed25519** key per identity used for **both authentication and commit
signing** via `gpg.format=ssh` + `allowed_signers`. No GPG.

---

## 3. Source of truth & safe writes

- **The real files are the only source of truth.** No parallel database/sidecar.
  On startup the tool parses its managed blocks to reconstruct the identity and
  account list (zero drift).
- The tool owns only **sentinel-delimited blocks**; anything outside is
  preserved verbatim:

  ```
  # >>> gid: personal >>>
  Host personal.github.com
      Hostname ssh.github.com
      Port 443
      User git
      IdentityFile ~/.ssh/id_ed25519_personal
      IdentitiesOnly yes
  # <<< gid: personal <<<
  ```

- Every mutation: **timestamped backup** first (`~/.ssh/config.bak.<ts>`),
  **idempotent** whole-block rewrite (never blind append), correct permissions
  (`~/.ssh` 700, private key 600, `.pub` 644, `config` 600), and **explicit
  confirmation**.
- **Adopt existing fragments**: detect plain-style files (`~/.gitconfig_personal`,
  etc.) and offer to reference or migrate them into `~/.gitconfig.d/`.

---

## 4. Functional requirements

### 4.1 Identity / Account / Credential CRUD
- **Create** an identity: generate an ed25519 key (auth + signing) or reuse an
  existing key; produce the four coordinated artifacts.
- **Read/List** identities, accounts, and keys with their wiring.
- **Update** name/email, signing on/off, provider/alias/port, match strategy.
- **Delete** an identity/account: remove its managed blocks and (optionally) its
  key, with confirmation and backup.
- **Rotate / replace a key**: regenerate a key for an existing identity (e.g.
  the key was exposed, or a shared key must be replaced by a server-specific
  one), re-point the artifacts, and re-run the test flow.

### 4.2 Two-phase test flow (input & output shown)
Every test prints the **command** and its **real output**.

1. **Explicit test (hypothesis):** `ssh -i <key> -o IdentitiesOnly=yes -T git@<host>`
   — proves the key authenticates, showing which file is used.
2. **Write (confirm + backup):** on success and confirmation, write the blocks.
3. **Resolved test (verification):** `ssh -T git@<alias>` plus `ssh -G <alias>`
   to show **which IdentityFile the config actually resolved**, confirming the
   wiring matches expectation.

### 4.3 Clipboard
Copy the public key to the clipboard when it is generated **and** on demand when
reusing an identity. Cross-platform: `pbcopy` (macOS), `wl-copy`/`xclip` (Linux).

### 4.4 Upload instructions
For GitHub/GitLab, show the concrete steps to add the public key (where to go,
which value to paste) for both **authentication** and **signing** keys.

### 4.5 Doctor (health checks)
Runs **first** when the TUI launches; also `gid doctor` on the CLI. Each finding
has severity + explanation + suggested fix (auto-fix with confirmation):

- **Dependencies** present (`ssh`, `ssh-keygen`, `ssh-add`, `git`, clipboard
  tool) with **install hints per OS** (macOS `brew …`; Linux `apt`/`dnf`/`pacman`).
- **Permissions** on `~/.ssh`, keys, `.pub`, `config`.
- **Coherence/drift**: every `IdentityFile` resolves to an existing key; every
  `includeIf` points to an existing fragment; signing identities have an
  `allowed_signers` line; `IdentitiesOnly yes` is present.
- **Orphans**: unused keys, non-included fragments, aliases without a matching
  `includeIf`.
- **Signing wiring**: `gpg.format=ssh` and `allowed_signers` path configured.
- **Agent**: `ssh-agent` running and keys loaded.

### 4.6 Git configuration management
- **Global/shared config** (toggles): `push.autoSetupRemote`, `core.ignorecase`,
  `pull.rebase`, `fetch.prune`, aliases, color.
- **URL rewriting**: when an SSH host/alias is added, **suggest the HTTPS
  equivalent** and let the user **edit** it before generating the `insteadOf`.
- **Identity selection strategy**: support both `gitdir:` (default suggestion,
  matching the user's `~/git/<client>/…` layout) and `hasconfig:remote.*.url`
  (by remote URL); may be combined per account.

### 4.7 `add repo` workflow
`gid add repo <url>`:

1. Detect the provider from the URL.
2. If it is a default provider (GitHub/GitLab), ask whether it is **personal**
   or belongs to a **specific client**, offering candidates discovered from
   existing `~/git/<client>` folders **and** from existing accounts in
   ssh/git config. A new client can be created on the fly.
3. Resolve the correct **alias** and rewrite the clone URL to it
   (`git@personal.github.com:user/repo.git`) so the right key is used.
4. **Clone** into the corresponding folder (base dir default `~/git`, per-client
   subfolder).
5. On completion, run a **`git -C <dest> pull`** to verify and print the clone +
   pull **output**.

### 4.8 Interfaces
- **Core** package: UI-free, the single source of logic, built test-first.
- **CLI** (Cobra): `gid doctor | identity add/list/test | host add | add repo …`
  with **shell completion** (bash/zsh/fish) generated by Cobra.
- **TUI** (Bubble Tea): launches into the **doctor dashboard**, then navigation
  to the identity/account/config managers.

---

## 5. Technical approach

- **Language/stack:** Go + Bubble Tea (TUI) + Cobra (CLI). Single static binary,
  no runtime dependency, native macOS/Linux.
- **Architecture:** thin CLI/TUI shells over a tested core
  (`identity`, `sshconfig`, `gitconfig`, `doctor`, `keygen`, `tester`,
  `clipboard`, `deps`, `platform`).
- **Platform specifics:** macOS `Host *` uses `UseKeychain yes` +
  `AddKeysToAgent yes` guarded by `IgnoreUnknown UseKeychain`; clipboard and
  dependency hints branch per OS.

---

## 6. Constraints, risks, non-goals

**Constraints:** macOS + Linux; never mutate user files without backup + confirm;
all generated content in English (see `CLAUDE.md`).

**Risks & mitigations:**
- *Corrupting existing config* → backups, managed blocks, dry-run diff, tests.
- *Wrong key resolved by SSH agent* → `IdentitiesOnly yes`; resolved test via
  `ssh -G`.
- *Tool name collisions* (`gid`) → confirm name before scaffolding.

**Non-goals (out of scope for v1):** Windows; GPG signing; web UI; scheduled/auto
key rotation; secret-vault integration.

---

## 7. Phasing

- **Phase 1 (MVP):** core + doctor + identity CRUD (create ed25519 auth+signing,
  generate the four artifacts with confirm + backup, two-phase test flow,
  clipboard, upload steps), key rotation, minimal TUI + `gid doctor`.
- **Phase 2:** global config toggles, `insteadOf` with HTTPS suggestion, fragment
  adoption, full CLI completions, `add repo` workflow.
- **Phase 3:** automatic key upload via `gh`/`glab` when present, cross-OS
  polish, TUI theming/UX.

---

## 8. Acceptance criteria

### Functional
- [ ] Create an identity end-to-end; the four artifacts are written with backup
      and confirmation; both test phases pass and show input + output.
- [ ] Two identities on the same provider coexist via distinct aliases and each
      resolves to its own key (`ssh -G` proof).
- [ ] Rotate a key: artifacts re-point to the new key and the resolved test
      passes.
- [ ] Delete an identity: managed blocks removed, files outside blocks intact.
- [ ] Public key copied to clipboard on generate and on demand (per-OS).
- [ ] `gid doctor` reports dependencies, permissions, drift, orphans, signing,
      and agent status, each with a suggested fix; runs first in the TUI.
- [ ] `gid add repo <url>` detects provider, asks personal/client, clones into
      the right folder via the alias, and verifies with a pull (output shown).

### Quality
- [ ] Core has unit tests written test-first; config parse/render is round-trip
      safe (parse → render → parse is stable).
- [ ] No write path lacks a backup + confirmation.
- [ ] All generated content is in English.

### User acceptance
- [ ] Existing hand-written config outside managed blocks is preserved.
- [ ] Upload steps are clear enough to add a key without external docs.

---

## 9. Open decisions

1. **Tool/binary name** — provisional `gid`. Alternatives: `gitid`, `sgc`.
2. **Default clone base dir** — assumed `~/git`; confirm.
3. **Phase 1 cut** — is `insteadOf` / `add repo` correctly deferred to Phase 2?

---

**Clarification rounds:** 4 · **Working method:** hypothesis → test → implement
(see `CLAUDE.md`).
