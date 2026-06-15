# Feature Research

**Domain:** Multi-identity SSH/Git identity manager CLI + TUI (Go)
**Researched:** 2026-06-08
**Confidence:** HIGH (PRD requirements explicit; competitive landscape verified across bgit, gitch, git-ego, gitp, MultiKey CLI, manual workflows)

---

## Competitive Landscape Summary

Existing tools fall into two categories:

**Profile switchers** (gitp, git-ego, git-profile, gguser, gituser): store a TOML/JSON profile list, set `user.name`/`user.email` globally or locally on demand. They do NOT own `~/.ssh/config`, do NOT generate keys, do NOT test authentication, and do NOT produce signed-commit wiring. Switching is a manual gesture, not automatic.

**SSH-aware managers** (bgit, MultiKey CLI, gitch): go further — generate keys, write SSH config stanzas, apply directory-based rules automatically. They still lack: two-phase verified test flow before writing, key rotation with artifact re-pointing, doctor coherence/drift/orphan checks, SSH commit signing wiring (`allowed_signers`), clipboard-copy on generate, upload instructions, and fragment adoption.

**Manual workflows** (`~/.ssh/config` + `includeIf` gists, Zoltan Toma 2025, bgauduch gist): are what most developers actually use. Pain points are well-documented: wrong-email commits, forgotten per-repo config, `IdentitiesOnly` omitted, no coherence guarantee, blind appending corrupts config.

**`gitid` occupies the gap:** lifecycle ownership (generate → test → write → rotate → verify) with safety guarantees (backup, idempotent blocks, confirmation) that none of the above provide.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | PRD Phase | Notes |
|---------|--------------|------------|-----------|-------|
| Identity CRUD (create/list/edit/delete) | Every identity manager has this; anything less is a script | MEDIUM | 1 | Includes name, email, provider, alias, key path wiring |
| SSH key generation (ed25519) | Users expect the tool to generate the key, not tell them to run `ssh-keygen` manually | LOW | 1 | Auth + signing, one key per identity; supersedes RSA reference config |
| `~/.ssh/config` stanza write | Core deliverable; without it the tool doesn't replace the manual gist workflow | HIGH | 1 | Must be idempotent managed-block rewrite, never blind append; backup required |
| `~/.gitconfig` `includeIf` write | Users expect Git identity auto-selection; without it commits go out under wrong email | HIGH | 1 | Both `gitdir:` (default) and `hasconfig:` strategies; per-identity fragment |
| Per-identity fragment (`~/.gitconfig.d/`) | Required for `includeIf` to point at something; must include signing wiring | MEDIUM | 1 | `user.name`, `user.email`, `gpg.format=ssh`, `user.signingkey`, `commit.gpgsign true` |
| List identities with status | Every manager lists profiles; without it users can't verify what was created | LOW | 1 | Show key path, provider, alias, signing status |
| SSH authentication test (`ssh -T`) | Developers expect "does this work?" before trusting the setup | LOW | 1 | Standard across GitHub/GitLab/Bitbucket docs; expected after any config write |
| Shell completion (Cobra) | Expected from any serious CLI in 2025; bash/zsh/fish | LOW | 1 | Cobra generates this; low cost, high polish |
| Timestamped backup before mutation | Safety baseline; any tool touching `~/.ssh/config` or `~/.gitconfig` must do this | MEDIUM | 1 | Applies to every write operation; no mutation without backup |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required by users before they try `gitid`, but deliver the core value and drive retention.

| Feature | Value Proposition | Complexity | PRD Phase | Notes |
|---------|-------------------|------------|-----------|-------|
| Two-phase test flow (explicit `ssh -i` → resolved `ssh -T <alias>` + `ssh -G`) | **No competitor does this.** Proves the resolved config — not just the key — works before writing anything. `ssh -G` exposes exactly which `IdentityFile` OpenSSH will pick, catching `IdentitiesOnly`-missing bugs at config time, not at `git push` time. | MEDIUM | 1 | The hypothesis→test→implement discipline applied to SSH; `ssh -G <alias>` outputs resolved config for that host |
| Doctor health checks (deps, permissions, coherence/drift, orphans, signing wiring, agent) | Competitors have at best a narrow permission fix. A full doctor that explains *why* something is wrong and gives per-OS fixes is novel in this space. Orphan detection (keys without SSH stanzas, stanzas without fragments) prevents silent drift. | HIGH | 1 | Runs on TUI launch and as `gitid doctor`; most diagnostic depth of any tool in this category |
| `allowed_signers` management + signing wiring | SSH commit signing (Git 2.34+, `gpg.format=ssh`) is increasingly required. No profile-switcher handles `allowed_signers`. `gitid` wires it automatically. | MEDIUM | 1 | Generates `~/.ssh/allowed_signers`, adds entry per identity, points `gpg.signingkey` in fragment |
| Key rotation with artifact re-pointing | Rotating a key means updating the SSH stanza, the fragment's `user.signingkey`, `allowed_signers`, and re-running the test flow. No tool chains this. | HIGH | 1 | Replaces old key in all four artifacts atomically; re-runs two-phase test post-rotation |
| Clipboard copy of public key (on generate + on demand) | Removes the "open another terminal, `cat ~/.ssh/id_*.pub`, select-all, copy" friction. Every key-gen guide tells users to do this manually — the tool should do it. | LOW | 1 | Cross-platform: `pbcopy` (macOS), `xclip`/`xsel`/`wl-copy` (Linux); detected at runtime |
| Contextual upload instructions (GitHub/GitLab auth + signing) | After generating a key the next question is always "now what?" No tool answers it in-context. Showing the exact GitHub/GitLab URL with the right key pre-selected closes the loop. | LOW | 1 | Two keys to upload per identity: auth key (SSH keys settings) + signing key (Vigilant mode or Allowed signers) |
| TUI doctor dashboard as home screen | Launching to a health overview — not a blank prompt — turns `gitid` into a persistent tool rather than a one-shot keygen script. Bubble Tea; rare in this category. | HIGH | 1 | Bubble Tea TUI; `gitid doctor` is the CLI equivalent |
| Idempotent whole-block managed-block rewrite | Competitors either blindly append (corrupts config on re-run) or keep a sidecar database (creates drift). Sentinel-delimited blocks parsed from the real files are the only correct approach. | HIGH | 1 | Parse → mutate → render → write cycle; no sidecar state; round-trip stable |
| `insteadOf` HTTPS→SSH rewriting | Eliminates the "clone with SSH URL manually" friction. Users can copy the HTTPS URL from GitHub and `git clone` just works. Companion to the `add repo` workflow. | MEDIUM | 2 | Per-provider; editable HTTPS suggestion; written as managed block in `~/.gitconfig` |
| `gitid add repo <url>` workflow | Detect provider from URL, disambiguate personal/client identity, rewrite alias, clone into `~/git/<client>/`, verify with pull. Closes the last manual step in onboarding a new repo. | HIGH | 2 | Depends on: identity store, SSH aliases, `insteadOf`, `includeIf` fragments all in place (Phase 1) |
| Adopt existing plain-style fragments | Users with hand-written `~/.gitconfig_work` files should not need to start over. Detect, parse, offer migration into `~/.gitconfig.d/` with managed blocks. | MEDIUM | 2 | Prevents "two sources of truth" after adoption; existing `includeIf` lines get updated |
| Global/shared git config toggles | `push.autoSetupRemote`, `core.excludesfile`, `core.ignorecase`, etc. are currently hand-edited. `gitid` can surface and manage them without overwriting user customizations. | LOW | 2 | Managed block in `~/.gitconfig` global section |
| `IgnoreUnknown UseKeychain` portability guard | macOS-only directive. Without the guard, Linux rejects the SSH config. Every macOS user who moves to Linux hits this silently. | LOW | 1 | Written once into the `Host *` global block during init; zero ongoing cost |
| Port 443 / firewall-friendly defaults | Enterprise and hotel WiFi blocks port 22. `ssh.github.com:443` is the standard workaround. The tool should default to this for GitHub/GitLab/Bitbucket, matching the reference config. | LOW | 1 | `Hostname ssh.github.com`, `Port 443` per provider; `altssh.` variants for GitLab/Bitbucket |

### Anti-Features (Deliberately NOT Building)

| Anti-Feature | Why Requested | Why It's Excluded | What to Do Instead |
|--------------|---------------|-------------------|--------------------|
| Windows support | Many developers use Windows | SSH/keychain/clipboard behavior differs materially; `UseKeychain`, clipboard commands, key paths all diverge; would double the conditional surface area in Phase 1 | v1 ships macOS + Linux; Windows deferred; document clearly |
| GPG commit signing | GPG is the historical standard | `gpg.format=ssh` (Git 2.34+) achieves the same result with the SSH key already in hand; no second key type to manage, no GPG daemon to troubleshoot | SSH signing is wired automatically per identity |
| Web UI / dashboard | Visibility appeal | `gitid` is a terminal-first tool; a web server would require a daemon, introduce a listening port, and exceed scope | TUI + CLI cover 100% of the use case; no daemon needed |
| Scheduled / automatic key rotation | "Set it and forget it" appeal | Automating rotation without user review risks locking out active sessions; silent rotation of `allowed_signers` can invalidate existing signed commits | Rotation is user-initiated via `gitid rotate`; doctor warns when keys are old |
| Secret-vault integration (1Password, Bitwarden, etc.) | Passphrase convenience | Each vault has a different CLI/SDK; adds a runtime dependency and a new failure mode; out of scope for v1 | Keys live in `~/.ssh` with correct permissions (600/644/700); macOS Keychain via `UseKeychain yes` handles passphrase unlocking |
| Commit audit / history scan | "Who committed as the wrong user?" | Useful but orthogonal to identity lifecycle; adds git-log parsing with no clear fix path from within `gitid` | Use `git log --format="%ae"` + `gitch`-style audit if needed; `gitid` prevents future wrong-identity commits via correct `includeIf` wiring |
| Shell prompt injection | Show active identity in PS1 | Shell prompt code must be sourced into the user's shell init file, adding a persistent dependency; fragile across shell versions | `gitid doctor` and `gitid list` answer "what identity am I?" on demand |
| HTTPS credential management (PATs, GCM) | Some repos require HTTPS | HTTPS credential management is a separate problem domain (GCM, macOS Keychain, `~/.netrc`); mixing it in blurs `gitid`'s scope | Document: use GitHub CLI (`gh auth login`) or GCM for HTTPS; `gitid` covers SSH |
| Per-repo `.git/config` mutation | Fine-grained control | `includeIf gitdir:` achieves directory-level identity selection without touching each repo; per-repo mutation is manual and doesn't survive re-clone | `includeIf` in `~/.gitconfig` is the correct abstraction level |

---

## Feature Dependencies

```
[Identity CRUD]
    └──requires──> [SSH key generation]
    └──requires──> [Timestamped backup infrastructure]
    └──produces──> [SSH config stanza]
    └──produces──> [~/.gitconfig includeIf block]
    └──produces──> [Per-identity fragment]
    └──produces──> [allowed_signers entry]

[Two-phase test flow]
    └──requires──> [SSH config stanza written]
    └──requires──> [Identity CRUD (alias known)]
    └──blocks──>   [Any file write] (must pass before write)

[Doctor health checks]
    └──requires──> [Identity CRUD (to know what to check)]
    └──requires──> [SSH config stanza]
    └──requires──> [Per-identity fragment]
    └──requires──> [allowed_signers]
    └──enhances──> [TUI home screen]

[Key rotation]
    └──requires──> [Identity CRUD]
    └──requires──> [Two-phase test flow] (re-runs after rotation)
    └──requires──> [allowed_signers management]
    └──requires──> [Timestamped backup infrastructure]

[Clipboard copy]
    └──requires──> [SSH key generation] (key must exist)
    └──enhances──> [Upload instructions]

[Upload instructions]
    └──requires──> [Identity CRUD (provider known)]
    └──enhances──> [Clipboard copy]

[insteadOf URL rewriting]  [Phase 2]
    └──requires──> [Identity CRUD]
    └──requires──> [SSH config stanza] (alias must exist for rewrite target)
    └──required-by──> [add repo workflow]

[add repo workflow]  [Phase 2]
    └──requires──> [Identity CRUD]
    └──requires──> [SSH config stanza]
    └──requires──> [includeIf gitdir: strategy]
    └──requires──> [insteadOf URL rewriting]

[Adopt existing fragments]  [Phase 2]
    └──requires──> [Identity CRUD]
    └──requires──> [Per-identity fragment infrastructure]
    └──conflicts-with──> [Existing hand-written includeIf lines] (migration replaces them)

[Global/shared git config toggles]  [Phase 2]
    └──requires──> [Managed-block infrastructure] (already built in Phase 1)
```

### Dependency Notes

- **Two-phase test flow blocks every write**: The test must pass before `~/.ssh/config` or `~/.gitconfig` is mutated. This is the central safety invariant.
- **Doctor requires all four artifacts**: It checks coherence *across* SSH stanza + gitconfig block + fragment + `allowed_signers`. It cannot run meaningfully until Phase 1 identity CRUD is complete.
- **`add repo` requires all of Phase 1**: URL detection, alias rewrite, and `gitdir:` path matching all depend on the full artifact set being present.
- **Fragment adoption conflicts with existing `includeIf` lines**: The migration must atomically update the `includeIf` path in `~/.gitconfig` to point at the new `~/.gitconfig.d/` location or the old reference becomes an orphan.

---

## MVP Definition

### Phase 1 — Core identity lifecycle (launch with)

Everything needed to replace the existing Bash script and deliver the core value proposition.

- [ ] Identity CRUD — foundation of everything else
- [ ] ed25519 key generation (auth + signing) — replaces `ssh-keygen` manual step
- [ ] `~/.ssh/config` managed-block write with `IdentitiesOnly yes`, port 443 defaults, `IgnoreUnknown UseKeychain` guard
- [ ] `~/.gitconfig` `includeIf` block write (both `gitdir:` and `hasconfig:` strategies)
- [ ] Per-identity fragment with signing wiring (`gpg.format=ssh`, `user.signingkey`, `commit.gpgsign true`)
- [ ] `~/.ssh/allowed_signers` management
- [ ] Two-phase test flow (`ssh -i` explicit → `ssh -T <alias>` + `ssh -G` resolved) — blocks every write
- [ ] Timestamped backup before any mutation
- [ ] Key rotation with artifact re-pointing + test re-run
- [ ] Clipboard copy of public key (on generate + on demand)
- [ ] Contextual upload instructions (GitHub/GitLab auth + signing)
- [ ] Doctor health checks (deps, permissions, coherence/drift, orphans, signing wiring, agent)
- [ ] Minimal TUI (doctor dashboard home) + Cobra CLI with shell completion

### Phase 2 — Workflow integration (after Phase 1 is validated)

- [ ] `insteadOf` HTTPS→SSH rewriting — trigger: users reporting clone friction
- [ ] `gitid add repo <url>` workflow — trigger: Phase 1 identity lifecycle proven stable
- [ ] Adopt existing plain-style fragments — trigger: existing-user adoption requests
- [ ] Global/shared git config toggles — trigger: user feedback on settings they still edit manually

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Identity CRUD | HIGH | MEDIUM | P1 |
| ed25519 key generation | HIGH | LOW | P1 |
| `~/.ssh/config` managed-block write | HIGH | HIGH | P1 |
| `~/.gitconfig` includeIf write | HIGH | HIGH | P1 |
| Per-identity fragment + signing wiring | HIGH | MEDIUM | P1 |
| `allowed_signers` management | HIGH | MEDIUM | P1 |
| Two-phase test flow (ssh -G resolved) | HIGH | MEDIUM | P1 — **key differentiator** |
| Timestamped backup | HIGH | MEDIUM | P1 |
| Key rotation | HIGH | HIGH | P1 |
| Clipboard copy | MEDIUM | LOW | P1 |
| Upload instructions | MEDIUM | LOW | P1 |
| Doctor health checks | HIGH | HIGH | P1 — **key differentiator** |
| TUI doctor dashboard | MEDIUM | HIGH | P1 |
| Cobra CLI + shell completion | MEDIUM | LOW | P1 |
| `insteadOf` URL rewriting | MEDIUM | MEDIUM | P2 |
| `add repo` workflow | MEDIUM | HIGH | P2 |
| Adopt existing fragments | LOW | MEDIUM | P2 |
| Global git config toggles | LOW | LOW | P2 |

**Priority key:** P1 = Phase 1 (MVP), P2 = Phase 2 (post-validation)

---

## Competitor Feature Analysis

| Feature | gitp / git-profile / git-ego (profile switchers) | gitch / bgit / MultiKey CLI (SSH-aware) | **gitid** |
|---------|--------------------------------------------------|----------------------------------------|-----------|
| Identity CRUD | Yes (TOML/JSON profiles) | Yes | Yes |
| SSH key generation | No — user runs ssh-keygen | Yes (bgit, gitch) | Yes (ed25519, auth + signing) |
| `~/.ssh/config` write | No | Yes (append or own block) | Yes (idempotent managed block, never append) |
| `~/.gitconfig` includeIf write | No | Partial (MultiKey) | Yes (gitdir: + hasconfig: both) |
| Per-identity fragment | No | No | Yes (with signing wiring) |
| `allowed_signers` management | No | No | Yes |
| SSH commit signing wiring | No | No (GPG only in gitch) | Yes (gpg.format=ssh) |
| Two-phase test before write | No | No | Yes — **unique** |
| `ssh -G` resolved config verification | No | No | Yes — **unique** |
| Key rotation + re-point artifacts | No | No | Yes |
| Doctor coherence/drift/orphan checks | Narrow (gitch pre-commit hook only) | Permission fix only (bgit) | Full — **differentiating** |
| Clipboard copy on generate | No | No | Yes |
| Upload instructions in-context | No | No | Yes |
| Timestamped backup before mutation | No | No | Yes |
| Idempotent managed blocks (no append) | N/A (no file writes) | No (append or full-own) | Yes — **unique** |
| `insteadOf` URL rewriting | No | No | Yes (Phase 2) |
| `add repo` workflow | No | No | Yes (Phase 2) |
| Fragment adoption migration | No | No | Yes (Phase 2) |
| TUI interface | No | No | Yes (Bubble Tea) |
| Windows support | Yes (git-ego, gitp) | Partial | No (v1 macOS + Linux) |
| HTTPS credential management | Yes (gitp, git-ego) | No | No (out of scope) |
| GPG signing | Yes (gitp optional) | Yes (gitch) | No (SSH signing only) |

---

## Sources

- bgit CLI: https://github.com/byterings/bgit (features inferred from bgitcli.com description, GitHub README)
- gitch: https://github.com/orzazade/gitch (README scraped via WebFetch)
- git-ego: https://github.com/bgreenwell/git-ego (README scraped via WebFetch)
- gitp: https://lib.rs/crates/gitp
- MultiKey CLI: https://multikeycli.com/ (403; features from WebSearch summary)
- Managing multiple git identities (2025): https://zoltantoma.com/posts/2025/2025-10-12-managing-multiple-git-identities/
- Git includeIf / hasconfig: https://www.nite07.com/en/posts/git-includeif/ and https://git-scm.com/docs/git-config
- insteadOf URL rewriting: https://www.meziantou.net/using-git-insteadof-to-automatically-replace-https-urls-with-ssh.htm
- SSH key pain points: https://iambacon.co.uk/blog/the-pitfalls-of-using-a-global-author-identity-in-git
- GitHub managing multiple accounts: https://docs.github.com/en/account-and-profile/how-tos/account-management/managing-multiple-accounts
- Testing SSH connection: https://docs.github.com/en/authentication/connecting-to-github-with-ssh/testing-your-ssh-connection
- SSH commit signing: https://blog.dbrgn.ch/2021/11/16/git-ssh-signatures/
- Azure DevOps RSA-only note (2026): via WebSearch result summary

---

*Feature research for: multi-identity SSH/Git identity manager CLI + TUI (gitid)*
*Researched: 2026-06-08*
