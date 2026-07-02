# Reference: target `~/.ssh/config` structure

Captured from a user-provided gist during initialization. Shows the **structural**
end-state the tool should produce: global keychain/agent block, per-account host
aliases, port 443, explicit `IdentityFile` with `IdentitiesOnly yes`.

> Note: this reference uses RSA (`id_rsa_*`). The PRD supersedes the key type with
> a single **ed25519** key per identity (auth + signing). Treat aliases, ports,
> and `IdentitiesOnly` as the target — not the key type.

```ssh-config
# Global configuration - store passphrases in macOS Keychain
Host *
  UseKeychain yes
  AddKeysToAgent yes

# Standard GitHub - use port 443 to bypass firewalls
Host github.com
  Hostname ssh.github.com
  Port 443
  User git

# Company GitHub with custom alias and specific SSH key
Host github.companyname.com
  Hostname ssh.github.companyname.com
  Port 443
  User git
  IdentityFile ~/.ssh/id_rsa_company
  IdentitiesOnly yes

# Personal GitHub with custom alias and specific SSH key
Host personal.github.com
  Hostname ssh.github.com
  Port 443
  User git
  IdentityFile ~/.ssh/id_rsa_personal
  IdentitiesOnly yes

# Company GitLab with specific SSH key
Host gitlab.companyname.com
  Hostname altssh.gitlab.companyname.com
  Port 443
  User git
  IdentityFile ~/.ssh/id_rsa_company
  IdentitiesOnly yes

# Bitbucket using alternative port
Host bitbucket.org
  Hostname altssh.bitbucket.org
  Port 443
  User git
  IdentityFile ~/.ssh/id_rsa_personal
  IdentitiesOnly yes
```

## Notable directives
- **Default vs aliased provider:** `github.com` (default identity, no IdentityFile)
  coexists with `personal.github.com` (aliased, explicit key) — same provider,
  multiple identities.
- **Firewall-friendly:** every host on `Port 443` via the `ssh.`/`altssh.` hostnames.
- **`IdentitiesOnly yes`** on every aliased host — prevents the agent offering the
  wrong key (a key risk the PRD's `ssh -G` resolved test guards against).
- **macOS global block:** `UseKeychain`/`AddKeysToAgent` under `Host *` (PRD wraps
  these in `IgnoreUnknown UseKeychain` for Linux portability).
