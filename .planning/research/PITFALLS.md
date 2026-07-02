# Pitfalls Research

**Domain:** CLI + TUI SSH/Git identity manager (`gitid`) — multi-identity SSH key management, `~/.ssh/config` mutation, `~/.gitconfig` `includeIf` management, ed25519 SSH commit signing
**Researched:** 2026-06-08
**Confidence:** HIGH (all critical pitfalls verified against official docs and multiple sources)

---

## Critical Pitfalls

### Pitfall 1: Blind Appends and the `grep`-Guard Anti-Pattern

**What goes wrong:**
The legacy script (`ssh-keygen.sh`) guards appends with `grep -q "github" $sshcfile` then blindly appends entire blocks via `>>`. This is the exact failure mode `gitid` must never repeat. Once a provider string appears in any comment or partial block, the guard silently passes and the block is never written. When the guard fails (e.g., the user already has a `github` comment from a different context), the tool silently does nothing. When it triggers, a second run doubles the block.

**Why it happens:**
Treating a config file as an append-only log instead of as a structured document with owned sections. There is no concept of "what did this tool write" vs "what did the user write."

**How to avoid:**
Use sentinel-delimited managed blocks. Every block the tool writes must be wrapped:
```
# BEGIN gitid-managed: <identity-name>
...content...
# END gitid-managed: <identity-name>
```
On every write, scan for the sentinel pair, replace the entire range atomically, never append. If no sentinel is found for an identity, insert at the correct position (specific host blocks before `Host *`). Anything outside managed blocks is untouched.

**Warning signs:**
- Duplicate `Host` stanzas appearing after re-runs
- `ssh -G <alias>` showing different values than expected
- `grep` as the only idempotency guard in code review

**Phase to address:** Phase 1 (SSH config write layer — the `sshconfig` package is the foundation; get this right before anything else builds on it)

---

### Pitfall 2: Missing `IdentitiesOnly yes` — Agent Offers Wrong Key

**What goes wrong:**
Without `IdentitiesOnly yes`, the SSH client offers every identity from the agent in order. GitHub's `MaxAuthTries` defaults to 6 server-side. A developer with 7+ keys loaded in `ssh-agent` will receive `Too many authentication failures` before the correct key is ever offered. The connection fails with a misleading error.

**Why it happens:**
The default `IdentitiesOnly no` is convenient for single-key setups. Multi-identity setups appear to work until the agent accumulates enough keys.

**How to avoid:**
Every aliased `Host` block managed by `gitid` must emit `IdentitiesOnly yes`. The `Host *` global block must not include `IdentitiesOnly yes` (that would break hosts without explicit `IdentityFile`). The two-phase test flow — `ssh -i <keyfile>` then `ssh -T <alias>` plus `ssh -G <alias>` — verifies the resolved identity before the block is written.

**Warning signs:**
- `ssh -T git@personal.github.com` fails with "Too many authentication failures" while `ssh -T git@github.com` succeeds
- `ssh -G <alias> | grep identitiesonly` returns `no`
- User has more than 5 keys in `ssh-agent` (`ssh-add -l | wc -l`)

**Phase to address:** Phase 1 (core SSH config generation; enforced in the `sshconfig` package render path)

---

### Pitfall 3: `Host` Alias vs `Hostname` Confusion in `git remote` URLs

**What goes wrong:**
The SSH `Host` directive is an alias, not the real hostname. `Hostname ssh.github.com` is what SSH actually connects to. But `git remote` URLs must use the `Host` alias — e.g., `git@personal.github.com:user/repo.git` — not the real hostname `ssh.github.com`. If a user clones using the real hostname, SSH uses the wrong (or default) identity; the alias-based `IdentityFile` and `IdentitiesOnly yes` are never evaluated.

**Why it happens:**
The distinction between `Host` (alias, matched by SSH) and `Hostname` (real DNS name, connected to) is non-obvious. Users copy URLs from GitHub's interface (`git@github.com:…`) and expect them to work with aliased configs.

**How to avoid:**
- The `hasconfig:remote.*.url` `includeIf` match strategy handles the case where the remote URL uses the real hostname — it selects the right gitconfig fragment regardless of SSH alias. This is why the PRD supports both `gitdir:` and `hasconfig:` strategies.
- When generating SSH config, clearly document the alias in comments and show the correct clone URL format.
- The doctor command should check that remotes in discovered repos use the expected alias form, not the real hostname.

**Warning signs:**
- `git push` authenticates as the wrong GitHub account (wrong email in commit)
- `ssh -G github.com` shows the wrong `IdentityFile` for a repo that should use a non-default identity
- Remote URL contains `ssh.github.com` directly instead of the alias

**Phase to address:** Phase 1 (SSH config generation) and Phase 2 (`add repo` workflow — must emit clone URLs using the alias, not the raw provider hostname)

---

### Pitfall 4: `IgnoreUnknown UseKeychain` Must Be in `Host *` Before `UseKeychain yes`

**What goes wrong:**
`UseKeychain yes` is an Apple-patched OpenSSH directive. Upstream OpenSSH on Linux treats it as an error: `Bad configuration option: usekeychain`. The tool generates files that crash `ssh` on Linux if `UseKeychain` appears without `IgnoreUnknown UseKeychain` ahead of it.

**Why it happens:**
The reference target config (`target-sshconfig.md`) does not include `IgnoreUnknown`. The legacy script (`ssh-keygen.sh`) adds `IgnoreUnknown UseKeychain` but does so inside a `Host *` block rather than before it — placement matters because SSH parses line-by-line.

**Correct form:**
```
Host *
  IgnoreUnknown UseKeychain
  UseKeychain yes
  AddKeysToAgent yes
```
`IgnoreUnknown` must appear within the same `Host *` block, before `UseKeychain yes`. (OpenSSH applies `IgnoreUnknown` to subsequent unknowns in the same parsing context.)

**Warning signs:**
- `ssh -G anyhost` on Linux prints `Bad configuration option: usekeychain` and exits non-zero
- Generated config is tested on macOS only during development
- CI runs only on macOS

**Phase to address:** Phase 1 (SSH config `Host *` global block generation; add a Linux integration test or doctor check)

---

### Pitfall 5: `Host *` Global Block Position — First-Match-Wins Cascade

**What goes wrong:**
SSH config uses first-match-wins per directive. If `Host *` is placed first in the file, any directive it sets (e.g., `ServerAliveInterval`, `IdentityFile`) cannot be overridden by specific `Host alias` blocks below it.

**Why it happens:**
Appending the `Host *` block at the top when it's the "global" block feels logical. In fact it must be at the end (or last in the managed section) so specific aliases can override.

**How to avoid:**
When writing managed blocks, always place the `Host *` global block after all specific `Host` alias blocks. The managed-block ordering in the `sshconfig` render path should enforce: specific hosts first, `Host *` last.

**Warning signs:**
- `ssh -G <alias> | grep identityfile` returns the global key, not the per-identity one
- Adding `IdentitiesOnly yes` to a specific host block has no effect

**Phase to address:** Phase 1 (sshconfig render ordering logic)

---

### Pitfall 6: `~/.ssh/config` File Permissions — `600` Not `644`

**What goes wrong:**
If `~/.ssh/config` has permissions `644` (world-readable), SSH on some systems ignores the file entirely or logs a warning and proceeds with defaults — silently using the wrong identity. If `~/.ssh` directory is `755`, other local users can enumerate key filenames.

The legacy script makes this exact mistake in reverse: it sets `.pub` files to `600` (too restrictive). Public keys should be `644`; SSH operations that read the `.pub` file (e.g., `ssh-copy-id`, `gitid doctor`) fail with permission errors.

**Correct permissions:**
| Path | Mode | Rationale |
|------|------|-----------|
| `~/.ssh/` | `700` | No group/world access to enumerate files |
| `~/.ssh/config` | `600` | Not world-readable; SSH enforces this |
| `~/.ssh/<key>` | `600` | Private key; SSH refuses keys with looser perms |
| `~/.ssh/<key>.pub` | `644` | Public key; must be readable for upload/copying |

**How to avoid:**
- Set permissions explicitly after every write operation, not just on creation
- `os.Chmod` the files, never rely on the user's `umask`
- Doctor command checks all four permission classes and reports deviations

**Warning signs:**
- `ssh -vvv` shows `bad permissions` or `unprotected private key file`
- `pbcopy`/`xclip` on `.pub` fails (if `.pub` was mistakenly `600`)
- Key works on first run but fails after a permissions audit

**Phase to address:** Phase 1 (keygen + sshconfig write path; doctor health check)

---

### Pitfall 7: Non-Atomic Writes — Partial Read Race Condition

**What goes wrong:**
Writing directly to `~/.ssh/config` or `~/.gitconfig` leaves a window where another process (git, ssh, a terminal multiplexer restoring sessions) reads a half-written file. On Linux, `os.WriteFile` is not atomic. A crash mid-write leaves a corrupt config and potentially a locked-out machine (no valid SSH config = no remote access).

**Why it happens:**
`os.WriteFile(path, data, 0600)` truncates then writes in place. Any read between truncate and complete-write sees garbage.

**How to avoid:**
Write-then-rename pattern:
1. Write complete content to a temp file in the same directory (same filesystem ensures atomic rename)
2. `os.Rename(tmpPath, targetPath)` — atomic on Linux/macOS at the filesystem level
3. `os.Chmod(targetPath, 0600)` after rename
4. Use a unique temp filename (e.g., `config.gitid.tmp.<pid>.<nonce>`) not a fixed `.tmp` suffix (fixed names cause their own concurrent-write race)

Additionally: take a timestamped backup before any mutation. The backup is the recovery path if the new content is semantically wrong even if physically intact.

**Warning signs:**
- Writes use `os.WriteFile` or `ioutil.WriteFile` without intermediate temp file
- No backup step before mutation
- No recovery path for partial write

**Phase to address:** Phase 1 (the safe-write utility in the `sshconfig`/`gitconfig` packages — this must be the first infrastructure built, before any config mutation)

---

### Pitfall 8: `includeIf "gitdir:…"` — Trailing Slash Required for Directory Match

**What goes wrong:**
`includeIf "gitdir:~/git/client"` (no trailing slash) matches only the exact path `~/git/client`, not repos inside it. The intended behavior is `~/git/client/` (with trailing slash), which Git expands to `~/git/client/**` — matching all repos under that directory.

**Why it happens:**
The trailing slash rule is documented but easily overlooked. It works as expected in a flat test repo (`~/git/client/.git`) but silently fails for nested structures.

**How to avoid:**
Always append a trailing slash when generating `gitdir:` patterns for directory-based matching. Add an integration test: create `~/git/<client>/repo/.git`, verify `git config user.email` resolves to the correct identity.

**Warning signs:**
- `git config user.email` returns the global default, not the per-client identity, inside `~/git/<client>/repo/`
- `gitdir:~/git/client` without trailing slash in generated config

**Phase to address:** Phase 1 (gitconfig includeIf generation; test with actual `.git` directory, not just config rendering)

---

### Pitfall 9: `hasconfig:remote.*.url` — Included Files Cannot Declare Remote URLs

**What goes wrong:**
Git prohibits remote URL declarations inside files included via `hasconfig:remote.*.url` condition. If the per-identity fragment (e.g., `~/.gitconfig.d/client.gitconfig`) attempts to set `[remote "origin"] url = …`, Git refuses with an error about circular dependencies. This is a hard runtime error, not a silent failure.

**Why it happens:**
The `hasconfig:` condition requires a scan-ahead pass to resolve remote URLs before includes are applied. Allowing included files to set remotes would create a chicken-and-egg situation.

**How to avoid:**
Per-identity fragments managed by `gitid` must contain only: `user.name`, `user.email`, `user.signingkey`, `gpg.format`, `commit.gpgsign`, `gpg.ssh.allowedSignersFile` — never `[remote]` sections. Document this restriction explicitly in code comments and reject fragment content that includes remote URLs at render time.

**Warning signs:**
- Fragment file contains `[remote "…"]` section
- `git config user.email` inside a repo fails with a Git error rather than returning a value
- Template for per-identity fragment includes a remote URL example

**Phase to address:** Phase 1 (fragment template definition; validate at render time)

---

### Pitfall 10: SSH Commit Signing — `allowed_signers` Format and Email Mismatch

**What goes wrong:**
SSH commit signing verification fails silently or shows "unverified" when:
1. The `allowed_signers` entry email does not exactly match `user.email` in the fragment (case-sensitive comparison)
2. The `namespaces="git"` field is omitted — Git requires it to prevent cross-protocol key reuse attacks
3. `gpg.ssh.allowedSignersFile` is not set in the fragment (local verification fails even if GitHub shows verified)
4. The signing key is registered on GitHub under "Authentication keys" only, not "Signing keys" (GitHub requires separate registration for each purpose)

**Correct `allowed_signers` line format:**
```
user@example.com namespaces="git" ssh-ed25519 AAAA...
```

**How to avoid:**
When generating the per-identity fragment, `gitid` must:
- Write the `allowed_signers` entry programmatically from the same `user.email` value used in the fragment (no manual copy-paste)
- Always include `namespaces="git"`
- Set `gpg.ssh.allowedSignersFile = ~/.ssh/allowed_signers` in the fragment
- The doctor command must check: file exists, email matches fragment, `namespaces="git"` present, key fingerprint matches the identity's key

**Warning signs:**
- `git log --show-signature` shows "No signature" or "Good signature from an untrusted principal"
- `gpg.ssh.allowedSignersFile` is set globally but not per-identity
- Email in `allowed_signers` has different case from `user.email`

**Phase to address:** Phase 1 (signing wiring in fragment generation and doctor checks)

---

### Pitfall 11: `user.signingkey` — Path vs. Inline Value

**What goes wrong:**
`user.signingkey` accepts either a file path (`~/.ssh/id_ed25519_client.pub`) or an inline `key::ssh-ed25519 AAAA...` value. Using an inline key means the fragment contains the literal public key — it becomes stale if the key is rotated and the fragment is not regenerated. Using a path means the fragment stays correct after key rotation as long as the path is stable.

**How to avoid:**
Always use the file path form for `user.signingkey`. The `gitid` key lifecycle assigns stable filenames (`id_ed25519_<identity>`); the path reference stays valid across rotations that replace the file content.

**Warning signs:**
- Fragment contains a literal `key::ssh-ed25519` value
- Key rotation updates the key file but signing fails because the inline value is stale

**Phase to address:** Phase 1 (fragment render; key rotation in Phase 1 rotation feature)

---

### Pitfall 12: `port 443` / `ssh.github.com` — Missing `User git` Directive

**What goes wrong:**
Port 443 on `ssh.github.com` requires `User git`. Without it, SSH uses the system username, and the server rejects with "Permission denied". This is a silent config gap — the `Host` alias resolves correctly via `ssh -G` but authentication fails.

**Why it happens:**
The `User git` directive is easy to overlook when copying the port-443 pattern. The error message does not mention the missing directive — it looks like a key problem.

**How to avoid:**
The SSH config template for GitHub/GitLab provider aliases must always emit `User git`. Add it to the mandatory field list validated by the doctor command.

**Warning signs:**
- `ssh -T personal.github.com` fails with "Permission denied (publickey)" despite correct key
- `ssh -G personal.github.com | grep user` returns the system username, not `git`

**Phase to address:** Phase 1 (provider template in sshconfig package)

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Parse SSH config into `map[string]Block` (losing order/comments) | Simpler data structure | Round-trip destroys user comments and ordering; any write corrupts unmanaged content | Never — use a comment-preserving AST or sentinel approach |
| Store identity state in a sidecar JSON/DB | Simpler queries | State drift: DB diverges from actual files; requires sync on every launch | Never — per PRD, the real files are the only source of truth |
| Rely on `umask` for file permissions | Fewer `os.Chmod` calls | Permissions are environment-dependent; wrong `umask` silently breaks SSH | Never — always call `os.Chmod` explicitly after writes |
| Use fixed `.tmp` suffix for temp files | Simpler implementation | Two concurrent `gitid` runs corrupt each other's temp file | Only acceptable in single-process context with a file lock |
| Global `gitconfig` `user.signingkey` instead of per-identity | One less config value | Wrong identity signs commits when multiple identities are active | Never for multi-identity setup |
| `grep`-guard before append | Simple idempotency check | Guard triggers on partial matches; duplicate blocks after edge cases | Never — sentinel blocks are the correct solution |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| GitHub SSH over port 443 | `Host github.com / Hostname ssh.github.com / Port 443` without `IdentityFile` + `IdentitiesOnly yes` (so default identity is used for all accounts) | Each GitHub account gets its own aliased `Host` (e.g., `personal.github.com`) with explicit `IdentityFile` + `IdentitiesOnly yes`; the bare `github.com` host serves the default identity only |
| GitHub commit signing key registration | Registering key only as "Authentication key" on GitHub | Register the same ed25519 key separately under both "Authentication keys" and "Signing keys" in GitHub SSH settings |
| GitLab port 443 | Using `altssh.gitlab.com` for SaaS GitLab | Correct: `Hostname altssh.gitlab.com Port 443`; self-hosted GitLab uses a different subdomain or no alternative port at all |
| Bitbucket port 443 | Assuming same `ssh.` prefix as GitHub | Bitbucket uses `altssh.bitbucket.org`, not `ssh.bitbucket.org` |
| `insteadOf` with SSH alias | Writing `git@personal.github.com:` as the HTTPS-to-SSH rewrite target when the remote URL is `github.com` | `insteadOf` rewrites affect `git clone https://github.com/...` → `git@github.com:...` (real hostname); the SSH alias is irrelevant to `insteadOf` |
| `hasconfig:remote.*.url` with SSH aliases | Pattern `git@personal.github.com:*/**` won't match remotes set with the real hostname | Need patterns for both alias form and real-hostname form; or use `gitdir:` as the primary strategy |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| `chmod 600` on `.pub` files (legacy script does this) | `ssh-copy-id`, `gitid doctor`, and clipboard tools fail silently; user cannot share public key without fixing permissions manually | Always emit `chmod 644` for `.pub`; doctor validates `.pub` is `644` |
| Passphrase-less key generation | Key compromise = immediate account access; no time to rotate before damage | Generate keys without `-N ""` (empty passphrase); prompt for passphrase or document the security tradeoff clearly |
| Backup file left world-readable | Timestamped backup of `~/.ssh/config` contains all identity aliases and key paths | Apply same `600` permissions to backup files |
| Inline private key path in error messages | Logging `opening /home/user/.ssh/id_ed25519_client failed` in a publicly-visible log or TUI debug output | Log key identifier/alias only, never full filesystem paths in user-facing output |
| No namespace in `allowed_signers` | Cross-protocol key reuse: a key authorized for email signing could be used to forge a git signature | Always include `namespaces="git"` in every `allowed_signers` entry |

---

## "Looks Done But Isn't" Checklist

- [ ] **SSH config generated:** Check `ssh -G <alias>` returns correct `identityfile`, `identitiesonly yes`, `hostname`, `port`, `user git` — not just that the file was written
- [ ] **Git identity active:** Check `git config user.email` inside a repo under the matched `gitdir:` path — not just that `includeIf` was written
- [ ] **Commit signing wired:** Verify `git log --show-signature` on a test commit shows "Good signature" — not just that `gpg.format=ssh` is in the fragment
- [ ] **`allowed_signers` coherent:** Verify email in file matches `user.email` exactly (case-sensitive), `namespaces="git"` present, key fingerprint matches identity key
- [ ] **Permissions correct:** Verify after every write that `~/.ssh/config` is `600`, `~/.ssh` is `700`, key is `600`, `.pub` is `644` — not just at creation time
- [ ] **Backup exists:** Verify timestamped backup was written before the mutation, not just that the write succeeded
- [ ] **Linux portability:** Verify generated config does not cause `ssh -G anyhost` to error on a Linux system (test `IgnoreUnknown UseKeychain` placement)
- [ ] **Idempotency:** Running `gitid` twice produces identical output and no duplicate blocks (second run is a no-op diff)
- [ ] **Unmanaged content preserved:** Any comments or non-managed blocks in `~/.ssh/config` before the run are byte-identical after the run

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Corrupt `~/.ssh/config` after partial write | LOW (if backup exists) | Restore from timestamped backup: `cp ~/.ssh/config.gitid-backup-<timestamp> ~/.ssh/config` |
| Corrupt `~/.ssh/config` with no backup | HIGH | Manually reconstruct from `~/.gitconfig.d/` fragments and re-run `gitid doctor` |
| Duplicate managed blocks after blind-append | MEDIUM | Re-run `gitid` after implementing sentinel rewrite; sentinel scan removes duplicates on first idempotent write |
| Wrong permissions on private key (`644`) | LOW | `chmod 600 ~/.ssh/<keyname>` |
| Wrong identity signing commits | MEDIUM | Fix `user.signingkey` path in fragment, amend/rebase recent commits, re-push |
| `allowed_signers` email mismatch | LOW | Regenerate entry: `gitid doctor --fix` updates `~/.ssh/allowed_signers` from fragment values |
| `hasconfig:` match fails because remote uses alias, not real hostname | MEDIUM | Add second `includeIf` with alias-form pattern, or switch strategy to `gitdir:` |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Blind appends / grep-guard (P1) | Phase 1: sshconfig write layer | Second `gitid` run produces identical file; diff is empty |
| Missing `IdentitiesOnly yes` (P2) | Phase 1: sshconfig render | `ssh -G <alias> \| grep identitiesonly` returns `yes` |
| Host alias vs Hostname confusion (P3) | Phase 1: SSH config; Phase 2: add-repo workflow | Clone URL in instructions uses alias form; `ssh -G` resolves correctly |
| `IgnoreUnknown UseKeychain` placement (P4) | Phase 1: `Host *` block template | `ssh -G anyhost` exits 0 on a Linux test (CI/Docker) |
| `Host *` position first-match-wins (P5) | Phase 1: sshconfig render ordering | `ssh -G <alias> \| grep identityfile` returns per-identity key, not global |
| Wrong file permissions (P6) | Phase 1: keygen + sshconfig write | `stat` checks in doctor report all four permission classes as correct |
| Non-atomic writes (P7) | Phase 1: safe-write utility | No partial-file state observable even under `kill -9` mid-write |
| `gitdir:` trailing slash (P8) | Phase 1: gitconfig includeIf render | `git config user.email` correct inside `~/git/<client>/repo/` |
| `hasconfig:` remote URL restriction (P9) | Phase 1: fragment template | Fragment schema validation rejects `[remote]` sections at render time |
| `allowed_signers` format / email mismatch (P10) | Phase 1: fragment generation + doctor | `git log --show-signature` on test commit shows "Good signature" |
| `user.signingkey` inline vs path (P11) | Phase 1: fragment render | Fragment contains path, not literal key; key rotation test re-checks signing |
| Port 443 missing `User git` (P12) | Phase 1: provider template | `ssh -G <alias> \| grep "^user "` returns `git` |

---

## Sources

- Legacy script anti-patterns: `.planning/references/legacy-ssh-keygen.md` (blind append, `chmod 600` on `.pub`, RSA, symlink `id_rsa`, macOS-only clipboard)
- Target SSH config structure: `.planning/references/target-sshconfig.md`
- Target gitconfig structure: `.planning/references/target-gitconfig.md`
- SSH `IdentitiesOnly yes` necessity: [Can't SSH? You Might Have Too Many Keys](https://www.tutorialworks.com/ssh-fail-too-many-keys/)
- SSH first-match-wins ordering: [OpenSSH Configuration Ordering](https://utcc.utoronto.ca/~cks/space/blog/sysadmin/OpenSSHConfigurationOrdering)
- SSH config parsing quirks (Match Final, HostName timing): [Quirks of Parsing SSH Configs](https://sthbrx.github.io/blog/2023/08/04/quirks-of-parsing-ssh-configs/)
- `IgnoreUnknown UseKeychain` / Linux portability: [SSH: Bad configuration option: usekeychain](https://www.unixtutorial.org/ssh-bad-configuration-option-usekeychain/)
- Atomic write for SSH config: [netbird atomic write PR #5867](https://github.com/netbirdio/netbird/pull/5867)
- `includeIf gitdir` trailing slash + `hasconfig` restrictions: [git-config official docs](https://git-scm.com/docs/git-config)
- `hasconfig:remote.*.url` circular dependency error: [Decoding the Git Error: Remote URLs Cannot Be Configured in Conditionally Included Files](https://codeinput.com/en/guides/git/errors/error-02)
- SSH commit signing `allowed_signers` format: [Git: The complete guide to sign your commits with an SSH key](https://dev.to/ccoveille/git-the-complete-guide-to-sign-your-commits-with-an-ssh-key-35bg)
- `user.signingkey` path vs inline: [Correctly Telling git about your SSH key for signing commits](https://dev.to/li/correctly-telling-git-about-your-ssh-key-for-signing-commits-4c2c)
- ed25519 modern recommendation: [SSH Keys in 2024: Why Ed25519 Replaced RSA as the Default](https://dev.to/theisraelolaleye/ssh-keys-in-2024-why-ed25519-replaced-rsa-as-the-default-47aa)
- Cross-platform clipboard detection: [clipper Go library](https://github.com/zyedidia/clipper)
- GitHub using SSH over HTTPS port: [Using SSH over the HTTPS port - GitHub Docs](https://docs.github.com/en/authentication/troubleshooting-ssh/using-ssh-over-the-https-port)

---
*Pitfalls research for: CLI + TUI SSH/Git identity manager (`gitid`)*
*Researched: 2026-06-08*
