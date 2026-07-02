# recipes/ — the target shape gitid manages

These are **recipes**, not throwaway samples: annotated reference configurations
that define the canonical end state `gitid` exists to produce and keep coherent.
Every agent and contributor should read them before planning, because they ARE
the objective — the files below show exactly how `~/.ssh/config` and `~/.gitconfig`
must look so that multiple identities (work / personal / …) resolve automatically
per repository.

| Recipe | Represents | Provenance |
|--------|------------|------------|
| [`ssh-config.recipe`](./ssh-config.recipe) | `~/.ssh/config` | gist [`2c98cff…`](https://gist.github.com/castocolina/2c98cff5920aff0f528ca847f56d7627) |
| [`gitconfig.recipe`](./gitconfig.recipe) | `~/.gitconfig` | gist [`60f2f1d…`](https://gist.github.com/castocolina/60f2f1d08c38eb9bd59e61e7e3ee0f5e) |

Legacy starting point: `~/git/personal/scripts/common/ssh-keygen.sh`.

## What the recipes establish (the wiring gitid must reproduce)

1. **SSH alias per identity** — `Host <identity>.<provider>` (e.g. `personal.github.com`)
   with `Hostname ssh.github.com`, `Port 443` (alt-SSH to bypass firewalls),
   `User git`, an explicit `IdentityFile`, and `IdentitiesOnly yes`.
2. **Git identity selection by remote URL** — the *primary* match is
   `[includeIf "hasconfig:remote.*.url:git@<alias>:*/**"] → path = ~/.gitconfig_<identity>`,
   so cloning `git@personal.github.com:you/repo` loads the right identity. A
   `gitdir:~/…/` match is shown only as an *alternative*.
3. **URL rewriting** — `[url "git@<provider>:"] insteadOf = https://<provider>/`
   forces SSH over HTTPS.
4. **Per-identity fragment** — name, email, signing key, `commit.gpgsign`.

## Caveat: structure, not key type

The gists predate the project and use RSA (`id_rsa_*`). `gitid` supersedes this
with **one ed25519 key per identity** (authentication + commit signing via
`gpg.format=ssh` + `~/.ssh/allowed_signers`). Take the *structure* from these
recipes — aliases, port 443, `IdentitiesOnly yes`, `includeIf gitdir:`/`hasconfig:`,
`insteadOf` — never the key algorithm.
