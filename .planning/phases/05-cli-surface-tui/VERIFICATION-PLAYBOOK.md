---
title: gitid — Guided Verification Playbook
status: awaiting-user-run
created: 2026-06-13
scope: all delivered phases (1, 2, 3, 3.1, 4, 5)
purpose: >
  Walk every supposedly-implemented capability, one command at a time, with the
  exact invocation, the expected flow/output, a shell check that proves it, and
  known considerations. Collect ALL bugs into the FEEDBACK ZONE below — NO fixes
  are made during this pass. Reconciliation is the next derived phase.
---

# How to use this playbook

1. Read the **Setup & Safety** section once and pick your environment per test.
2. Go test by test. For each: run the **Command**, watch for the **Expected flow**,
   then run the **Verify** shell check and compare to **Expect**.
3. Mark the result inline: change `RESULT: [ ] ok  [ ] bug` and write a one-line
   `NOTES:`. If it's a bug, also drop a bullet in the **FEEDBACK ZONE** at the
   bottom (that's the part I read to generate the next phase).
4. When you finish (or stop early), tell me. I read this same file and turn the
   FEEDBACK ZONE into the immediate next ROADMAP phase.

**Legend:** 🟢 safe (read-only / dry-run / sandbox) · 🟡 mutates files (backup first) ·
🔴 mutates + needs network/uploaded key · ⌨️ interactive prompts · 🧪 has a shell verify.

**Already-known gaps** are pre-marked `KNOWN ⚠️` so you don't re-discover them — just
confirm they still reproduce (or note if fixed).

---

# Setup & Safety

## Build the binary (always do this first)

```bash
cd /Users/ramon/git/personal/ssh-git-config
go build -o bin/gitid ./cmd/gitid && ./bin/gitid --version
```
**Expect:** prints `gitid version 0.0.0-dev` (or similar). If the build fails, stop and tell me.

## Two environments

- **Real `$HOME`** — needed for anything that does an `ssh -T` to GitHub/GitLab
  (create/rotate/test/add-account) because it authenticates against your real
  uploaded keys. 🟡/🔴 These mutate `~/.ssh/config`, `~/.gitconfig`,
  `~/.gitconfig.d/`, `~/.ssh/allowed_signers`. The tool backs up every file it
  touches, but **take your own snapshot first**:

  ```bash
  ts=$(date +%s); mkdir -p ~/gitid-uat-backup-$ts
  cp -a ~/.ssh/config ~/.gitconfig ~/gitid-uat-backup-$ts/ 2>/dev/null
  cp -a ~/.gitconfig.d ~/gitid-uat-backup-$ts/ 2>/dev/null
  cp -a ~/.ssh/allowed_signers ~/gitid-uat-backup-$ts/ 2>/dev/null
  echo "snapshot in ~/gitid-uat-backup-$ts"
  ```

- **Sandbox `HOME`** — for everything that does NOT need real network/keys
  (`list`, `baseline`, `doctor`, `--dry-run`, `completion`, TUI navigation). Zero
  risk to your real config:

  ```bash
  export GITID_SANDBOX=/tmp/gitid-sandbox
  rm -rf "$GITID_SANDBOX"; mkdir -p "$GITID_SANDBOX"
  # run commands as:  HOME="$GITID_SANDBOX" ./bin/gitid <args>
  ```
  When a test says "sandbox OK", prefix the command with `HOME="$GITID_SANDBOX"`.

> Tip: to restore real config after a 🟡/🔴 test, copy the files back from your
> snapshot dir. The tool's own `.bak.<timestamp>` files are the per-write backups.

---

# PHASE 1 — Bootstrap (dev toolchain)

> What it does: proves any engineer can build, test, lint, and install the tool.
> Why it matters: this is the floor the rest of the project stands on.

### T1.1 — Build, test, lint, format 🟢 🧪
**Command:**
```bash
make build && make test && make lint && make fmt
```
**Expected flow:** `make build` produces `bin/gitid`; `make test` runs the Go test
suite and exits 0 with a coverage line; `make lint` (golangci-lint v2 + gosec) exits
0; `make fmt` rewrites/format-checks with goimports and exits 0.
**Verify:**
```bash
ls -l bin/gitid && go test ./... >/dev/null 2>&1 && echo "TESTS OK" || echo "TESTS FAIL"
```
**Expect:** `bin/gitid` exists and `TESTS OK`.
**Considerations:** `make lint` needs golangci-lint v2 on PATH (binary install, not `go install`). If missing, run `make setup-env` first.
> RESULT: [x] ok  [ ] bug
> NOTES:

### T1.2 — Install / uninstall 🟡 🧪
**Command:**
```bash
make install && which gitid && make uninstall && (which gitid || echo "uninstalled")
```
**Expected flow:** installs to your `GOBIN`/`$GOPATH/bin`, prints its path, then removes it.
**Verify:** the final line prints `uninstalled` (or `which gitid` returns nothing).
**Considerations:** `make install` is just `go install ./cmd/gitid` — it does NOT install shell completion (see T5.5 / KNOWN gap).
> RESULT: [ ] ok  [x] bug
> NOTES:

```
ssh-git-config git:(main) ✗ make install && which gitid && make uninstall && (which gitid || echo "uninstalled")
go install ./cmd/gitid
gitid not found
➜  ssh-git-config git:(main) ✗
```

---

# PHASE 2 — First Identity, End-to-End

> What it does: create one identity and write four coordinated artifacts (SSH Host
> block, gitconfig includeIf, per-identity fragment, allowed_signers) with backup +
> prove-before-write, then copy the pubkey and show upload steps.
> Why it matters: this is the core promise of the tool. **G-05 (below) says the
> create-flow shape is wrong** — this is the most important area to scrutinize.

### T2.1 — Create a new identity (dry-run, no writes) 🟢 ⌨️ 🧪
**Command:**
```bash
HOME="$GITID_SANDBOX" ./bin/gitid identity add --dry-run
```
**Expected flow:** mode selector (1=new default, 2=reuse, 3=add-account) → prompts for
Identity name, Git name, Git email, Provider [github], Host alias, Hostname, Port,
Match gitdir, Passphrase. Then it prints a **preview of all four artifacts** and the
resolved `ssh -G` plan — and writes **nothing** (no final confirm).
**Verify (after it exits):**
```bash
ls -la "$GITID_SANDBOX/.ssh" "$GITID_SANDBOX/.gitconfig" 2>&1 | tail -n +1
```
**Expect:** no `id_*` key files, no `.gitconfig` created — dry-run wrote nothing.
**Considerations:** use a throwaway name like `uattest`. The preview is the contract you'll see for real in T2.2.
> RESULT: [ ] ok  [ ] bug
> NOTES:

Fine But I didnt wanted dry run, I need real

### T2.2 — Create a new identity for real 🔴 ⌨️ 🧪 — KNOWN ⚠️ (G-05)
**Command (real HOME — back up first per Setup):**
```bash
./bin/gitid identity add
```
**Expected flow (intended):** generate key → show upload instructions → wait → loop
the connectivity test until it authenticates (PASS) → only then persist the four
artifacts → copy pubkey → done.
**KNOWN ⚠️ G-05 (actual flow today):** it asks **"Write all four artifacts now?"
BEFORE you've uploaded the key**, runs the connectivity test **once**, treats
`Permission denied (publickey)` ("ReachableNotUploaded") as good-enough, and then
either writes blindly on `y` or writes **nothing** on the default `N` — so pressing
Enter looks like "nothing happened / the key vanished" (the key was staged to a temp
path). It never verifies a real authenticated `PASS` before committing.
**Verify what actually got written:**
```bash
ls -la ~/.ssh/id_* 2>/dev/null; echo "---"; \
grep -n "gitid managed" ~/.ssh/config ~/.gitconfig 2>/dev/null; echo "---"; \
ls -la ~/.gitconfig.d/ 2>/dev/null
```
**Expect (per the bug):** likely either all-four-written-without-real-auth, or
nothing-written despite completing the prompts. **Record exactly what you observe** —
this is the central reconciliation item.
**Considerations:** This blocks T2.3/T5.4/T5.6 (they need a real managed identity to copy/test). If you want a managed identity to exist for later tests, answer `y` at the write prompt and note it.
> RESULT: [x] ok  [ ] bug   (expected: bug — confirm shape)
> NOTES:

It is imcompleted if dont promt for upload comnfirmation and then test it and show success or error and ask for retry or discar/quit/return/close/save/continue or whatever option depends of result.

```
ssh-git-config git:(main) ✗ ./bin/gitid identity add
Create mode:
  1) new          — generate a fresh key (default)
  2) reuse        — reuse an existing private key
  3) add-account  — add an alias for an existing identity
Choose mode [1]:
Identity name: user_z3r0_gh
Git user.name: User Z3r0
Git user.email: castocolina.dev@gmail.com
Provider (github/gitlab) [github]:
Host alias [user_z3r0_gh.github]: userz3r0.personal.github
Hostname [ssh.github.com]:
Port [443]:
Match gitdir [~/git/user_z3r0_gh/]: ~/git/personal/
Passphrase (empty for none):
Write all four artifacts now? [y/N]: y
Pre-write connectivity test:
$ /usr/bin/ssh -i /var/folders/5w/2d0vm3b96_qdc9q9x1_m59tw0000gn/T/gitid-key-3436069540/key -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -p 443 -T git@ssh.github.com
git@ssh.github.com: Permission denied (publickey).

=== Preview: four coordinated artifacts ===
--- ~/.ssh/config (Host block) ---
Host userz3r0.personal.github
  Hostname ssh.github.com
  Port 443
  User git
  IdentityFile /Users/ramon/.ssh/id_ed25519_user_z3r0_gh
  IdentitiesOnly yes
--- ~/.gitconfig (includeIf) ---
# BEGIN gitid managed: user_z3r0_gh
[includeIf "gitdir:~/git/personal/"]
        path = /Users/ramon/.gitconfig.d/user_z3r0_gh
# END gitid managed: user_z3r0_gh
--- gitconfig fragment ---
[/Users/ramon/.gitconfig.d/user_z3r0_gh fragment]
  user.name       = User Z3r0
  user.email      = castocolina.dev@gmail.com
  gpg.format      = ssh
  user.signingkey = /Users/ramon/.ssh/id_ed25519_user_z3r0_gh.pub
  commit.gpgsign  = true
--- ~/.ssh/allowed_signers ---
castocolina.dev@gmail.com namespaces="git" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINso+0DVQ2mEucF9e5Y6hd4lXdP23F26yQkaEHxzB3bP

Resolved test:
$ /usr/bin/ssh -o BatchMode=yes -o ConnectTimeout=10 -T git@userz3r0.personal.github
git@ssh.github.com: Permission denied (publickey).
  user=git hostname=ssh.github.com port=443 identitiesonly=yes

Upload your public key to GitHub (TWO separate registrations of the SAME key):
  1. Open https://github.com/settings/ssh/new
  2. Authentication key: paste the .pub, set "Key type" = Authentication key, Add SSH key.
  3. Open https://github.com/settings/ssh/new again.
  4. Signing key: paste the SAME .pub, set "Key type" = Signing key, Add SSH key.
GitHub requires the key registered twice — once for authentication, once for signing.

Public key (also copied to your clipboard):
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINso+0DVQ2mEucF9e5Y6hd4lXdP23F26yQkaEHxzB3bP
➜  ssh-git-config git:(main) ✗ tree ~/.ssh
/Users/ramon/.ssh
├── allowed_signers
├── allowed_signers.bak.20260613-150932
├── castocolina
├── castocolina.pub
├── config
├── config.bak.20260613-150708
├── config.bak.20260613-150932
├── id_castocolina_gmail_com
├── id_castocolina_gmail_com.pub
├── id_ed25519_castocolina
├── id_ed25519_castocolina.pub
├── id_ed25519_user_z3r0_gh
├── id_ed25519_user_z3r0_gh.pub
├── known_hosts
└── known_hosts.old

1 directory, 15 files
➜  ssh-git-config git:(main) ✗ ./bin/gitid
➜  ssh-git-config git:(main) ✗ ./bin/gitid --help
Manage multiple Git identities by coordinating SSH and Git configuration

Usage:
  gitid [command]

Available Commands:
  baseline    Manage the shared global git baseline (core/push/pull defaults, gitignore, url rewrites)
  completion  Generate the autocompletion script for the specified shell
  copy        Copy the public key to the clipboard and print upload instructions
  doctor      Run a health check on the gitid-managed environment
  help        Help about any command
  host        Manage SSH host aliases
  identity    Create and verify Git identities
  rotate      Rotate the SSH key for an identity and re-test all artifacts

Flags:
  -h, --help      help for gitid
  -v, --version   version for gitid

Use "gitid [command] --help" for more information about a command.
➜  ssh-git-config git:(main) ✗ ./bin/gitid identity --help
Create and verify Git identities

Usage:
  gitid identity [command]

Available Commands:
  add         Create a new Git identity (key, SSH config, gitconfig, allowed_signers)
  copy        Copy the public key to the clipboard and print upload instructions
  delete      Delete a gitid-managed identity — removes its four managed artifacts with backup (IDENT-05)
  list        List gitid-managed identities reconstructed from ~/.ssh/config and ~/.gitconfig (IDENT-03)
  rotate      Rotate (replace) the key for an existing identity, re-pointing all four artifacts
  test        Re-run the resolved ssh -T / ssh -G test for an identity alias
  update      Update an existing Git identity's fields (email, signing, alias, port, match strategy — name immutable)

Flags:
  -h, --help   help for identity

Use "gitid identity [command] --help" for more information about a command.
➜  ssh-git-config git:(main) ✗ ./bin/gitid identity list
identity: castocolina
  key:      /Users/ramon/.ssh/id_ed25519_castocolina
  git:      User Z3r0 <castocolina.dev@gmail.com>
  alias:    castocolina.github
  provider: github
  port:     443
  match:    gitdir:~/git/personal/

identity: user_z3r0_gh
  key:      /Users/ramon/.ssh/id_ed25519_user_z3r0_gh
  git:      User Z3r0 <castocolina.dev@gmail.com>
  alias:    userz3r0.personal.github
  provider: ssh.github.com
  port:     443
  match:    gitdir:~/git/personal/
➜  ssh-git-config git:(main) ✗

```

![Details](../../../tmp/images/image.png)

```
The details list is empty, is posible to navigate and enter for details but there is not identification in list page.
```

### T2.3 — Two-phase test + ssh -G resolution on a created identity 🔴 🧪
**Precondition:** T2.2 actually wrote an identity (alias e.g. `uattest.github`).
**Command:**
```bash
./bin/gitid identity test uattest.github
```
**Expected flow:** prints `$ ssh -T uattest.github` and its real output, then
`$ ssh -G uattest.github` parsed into user / hostname / port / identitiesonly /
identityfile lines.
**Verify (independent):**
```bash
ssh -G uattest.github | grep -Ei "identityfile|identitiesonly|hostname|port|user"
```
**Expect:** `identitiesonly yes`, `user git`, the right hostname/port, and the
identityfile pointing at the key you created.
**Considerations:** if T2.2 couldn't create a real identity, mark this **blocked** and move on.
> RESULT: [x] ok  [ ] bug  [ ] blocked
> NOTES:

This is OK but in restrospective the concept alias is confusing during identification creation. We muest say host alias or similar.

Also, why this test is not integrated with the same base command UI? Well we need to design better UI/UX.

BTW is not only about the path for `includeIf`, it is also posible havig the same folder use different id based in URL alias? like user.personal.github.com or companyname.github.com and also combine with folders? May we need to promt for that during adding.
Right now exist two ids pointing same folders and that is a possible carrer for who is the identification correct in this case, the first, the last? Look like a work of doctor detect that and need to share that doctor share funtionality to prevent that classs of carrer.

``` 
 ssh-git-config git:(main) ✗ tree ~/.ssh
/Users/ramon/.ssh
├── allowed_signers
├── allowed_signers.bak.20260613-150932
├── castocolina
├── castocolina.pub
├── config
├── config.bak.20260613-150708
├── config.bak.20260613-150932
├── id_castocolina_gmail_com
├── id_castocolina_gmail_com.pub
├── id_ed25519_castocolina
├── id_ed25519_castocolina.pub
├── id_ed25519_user_z3r0_gh
├── id_ed25519_user_z3r0_gh.pub
├── known_hosts
└── known_hosts.old

1 directory, 15 files
➜  ssh-git-config git:(main) ✗ ./bin/gitid identity test id_ed25519_user_z3r0_gh
➜  ssh-git-config git:(main) ✗ subl ~/.gitconfig
➜  ssh-git-config git:(main) ✗ subl ~/.gitconfig.d
➜  ssh-git-config git:(main) ✗ subl ~/.gitconfig
ç%                                                                                                                           ➜  ssh-git-config git:(main) ✗ ./bin/gitid identity test userz3r0.personal.github
$ /usr/bin/ssh -o BatchMode=yes -o ConnectTimeout=10 -T git@userz3r0.personal.github
git@ssh.github.com: Permission denied (publickey).
$ ssh -G userz3r0.personal.github
  user           git
  hostname       ssh.github.com
  port           443
  identitiesonly yes
  identityfile   /Users/ramon/.ssh/id_ed25519_user_z3r0_gh
➜  ssh-git-config git:(main) ✗ subl ~/.ssh
➜  ssh-git-config git:(main) ✗ ./bin/gitid identity test userz3r0.personal.github
$ /usr/bin/ssh -o BatchMode=yes -o ConnectTimeout=10 -T git@userz3r0.personal.github
Hi castocolina! You've successfully authenticated, but GitHub does not provide shell access.
$ ssh -G userz3r0.personal.github
  user           git
  hostname       ssh.github.com
  port           443
  identitiesonly yes
  identityfile   /Users/ramon/.ssh/id_ed25519_user_z3r0_gh
➜  ssh-git-config git:(main) ✗ ssh -G userz3r0.personal.github | grep -Ei "identityfile|identitiesonly|hostname|port|user"
host userz3r0.personal.github
user git
hostname ssh.github.com
port 443
canonicalizehostname false
gatewayports no
identitiesonly yes
identityfile /Users/ramon/.ssh/id_ed25519_user_z3r0_gh
userknownhostsfile /Users/ramon/.ssh/known_hosts /Users/ramon/.ssh/known_hosts2
syslogfacility USER
➜  ssh-git-config git:(main) ✗
``` 

---

# PHASE 3 — Full CRUD + Multi-Identity

> What it does: list, update, delete, rotate identities; reuse an existing key;
> add a second account/alias; reconstruct everything from disk (no sidecar DB).
> Why it matters: proves identities are durable and editable, and that two
> identities on one provider coexist via distinct aliases.

### T3.1 — List identities 🟢 🧪
**Command (real HOME to see your real state):**
```bash
./bin/gitid identity list
```
**Expected flow:** one section per identity: `key`, `git`, `alias`, `provider`,
`port`, `match`, plus `! incomplete: ...` if any artifact is missing. If none are
gitid-managed, prints `no gitid-managed identities found`.
**Verify:** cross-check the count against managed blocks on disk:
```bash
grep -c "BEGIN gitid managed" ~/.ssh/config 2>/dev/null
```
**Expect:** the number of SSH Host blocks roughly matches the identities listed
(minus reserved/baseline blocks).
**Considerations:** Your hand-written `Host github.com` / `gitlab.com` blocks and
non-managed keys will **NOT** appear — `list` only shows gitid-managed identities.
This is the "real visualization TUI" scope item, not a bug.
> RESULT: [x] ok  [ ] bug
> NOTES:

Three sections, two identitties.
This list correct but not the `dashboard`. The dashboard need to wire this screen.

```
 ssh-git-config git:(main) ✗ ./bin/gitid identity list
identity: castocolina
  key:      /Users/ramon/.ssh/id_ed25519_castocolina
  git:      User Z3r0 <castocolina.dev@gmail.com>
  alias:    castocolina.github
  provider: github
  port:     443
  match:    gitdir:~/git/personal/

identity: user_z3r0_gh
  key:      /Users/ramon/.ssh/id_ed25519_user_z3r0_gh
  git:      User Z3r0 <castocolina.dev@gmail.com>
  alias:    userz3r0.personal.github
  provider: ssh.github.com
  port:     443
  match:    gitdir:~/git/personal/
➜  ssh-git-config git:(main) ✗ grep -c "BEGIN gitid managed" ~/.ssh/config 2>/dev/null
3

```

### T3.2 — Update an identity (dry-run) 🟢 ⌨️
**Command:**
```bash
./bin/gitid identity update <name> --dry-run
```
**Expected flow:** prompts pre-filled with current values for Git name, email, alias,
hostname, port, match gitdir, and a signing toggle (default = current state). Name is
**not** prompted (immutable). Prints a preview and writes nothing.
**Verify:** nothing changed on disk (compare `git config --file ~/.gitconfig.d/<name> --list` before/after — identical).
**Considerations:** needs an existing identity name (from T3.1). If none, use the sandbox after a successful create, or mark blocked.
> RESULT: [ ] ok  [ ] bug  [ ] blocked
> NOTES:

### T3.3 — Delete an identity (dry-run manifest) 🟢 ⌨️
**Command:**
```bash
./bin/gitid identity delete <name> --dry-run
```
**Expected flow:** prints a 4-item removal manifest (SSH block, gitconfig block,
fragment file, allowed_signers line) and writes nothing. (Real run asks two confirms:
remove blocks+fragment [N], then optionally delete key files [N, irreversible].)
**Verify:** identity still present in `./bin/gitid identity list`.
> RESULT: [ ] ok  [ ] bug  [ ] blocked
> NOTES:

### T3.4 — Rotate an identity's key (dry-run) 🟢 ⌨️
**Command:**
```bash
./bin/gitid identity rotate <name> --dry-run
# alias form:
./bin/gitid rotate <name> --dry-run
```
**Expected flow:** prompts for identity details (rotate does NOT reconstruct from
disk — D-06), previews a new key + re-pointed artifacts, writes nothing.
**Considerations:** note whether having to re-type details (vs pre-filled) is acceptable UX or a gap.
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T3.5 — Reuse an existing key 🟡 ⌨️
**Command:**
```bash
HOME="$GITID_SANDBOX" ./bin/gitid identity add --dry-run   # choose mode 2 (reuse)
```
**Expected flow:** mode `2`/`reuse` → asks for an existing private key path → builds
the four artifacts around that key instead of generating one.
**Considerations:** point it at any existing key path; in dry-run nothing is written. Confirm the reuse branch is reachable and previews correctly.
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T3.6 — Add a second account/alias to an identity 🟡 ⌨️
**Command:**
```bash
./bin/gitid identity add --dry-run     # choose mode 3 (add-account)
# also reachable as:
./bin/gitid host add
```
**Expected flow:** mode `3`/`add-account`/`alias` → asks existing identity name, new
provider, new alias, hostname, port, match dir → previews a new Host block + includeIf
sharing the existing key. `gitid host add` is the same flow under the `host` group.
**Considerations:** verify both entry points (`identity add` mode 3 and `host add`) reach the same flow.
> RESULT: [ ] ok  [ ] bug
> NOTES:

---

# PHASE 3.1 — Baseline Global Git Config + Gitignore

> What it does: seed a shared baseline gitconfig (core/push/pull/fetch + color +
> aliases + `ignorecase=false`), a curated global gitignore, and optional HTTPS→SSH
> `insteadOf` rewrites — all in idempotent managed blocks with backup→preview→confirm.
> Why it matters: this is the "sane defaults" layer; it must never clobber your own keys.

### T3.1a — Baseline setup (dry-run) 🟢 ⌨️ 🧪
**Command:**
```bash
HOME="$GITID_SANDBOX" ./bin/gitid baseline setup --dry-run
```
**Expected flow:** prints a unified preview: the `00-baseline` block (Tier-1 locked
keys + Tier-2 opt-out keys), the url-rewrites block (github/gitlab) with the
"affects ALL HTTPS ops (go get, npm, CI)" warning, the gitignore block, and the
prepended `[include]` block named `baseline-include`. Plus a conflicts section if
your config already sets those keys. Writes nothing.
**Verify:** nothing written in sandbox:
```bash
ls -la "$GITID_SANDBOX/.gitconfig" "$GITID_SANDBOX/.gitconfig.d" "$GITID_SANDBOX/.gitignore_global" 2>&1
```
**Expect:** none of these exist after a dry-run.
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T3.1b — Baseline setup for real (sandbox) 🟡 ⌨️ 🧪
**Command:**
```bash
HOME="$GITID_SANDBOX" ./bin/gitid baseline setup
```
**Expected flow:** Tier-2 opt-in prompt [Y/n] → per-rewrite keep prompts [Y/n] →
final "Write baseline now?" [y/N] → writes three surfaces atomically (rollback on
partial failure) and prints backup paths.
**Verify:**
```bash
HOME="$GITID_SANDBOX" git config --file "$GITID_SANDBOX/.gitconfig.d/00-baseline" --list | head
grep -n "baseline-include" "$GITID_SANDBOX/.gitconfig"
test -f "$GITID_SANDBOX/.gitignore_global" && echo "gitignore OK"
```
**Expect:** baseline keys listed (e.g. `core.ignorecase=false`, `push.autosetupremote=true`),
the include block present at the TOP of `.gitconfig`, and `gitignore OK`.
**Considerations:** run it **twice** — the second run must be idempotent (no new
changes / clean re-preview).
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T3.1c — Baseline show 🟢 🧪
**Command:**
```bash
HOME="$GITID_SANDBOX" ./bin/gitid baseline show
```
**Expected flow:** after T3.1b, prints `baseline: installed`, the file/include paths,
the baseline keys, the active url rewrites, and the managed gitignore pattern count.
Before setup it prints `no gitid-managed baseline found`.
**Verify:** the keys/rewrites shown match what T3.1b wrote.
> RESULT: [ ] ok  [ ] bug
> NOTES:

---

# PHASE 4 — Doctor (health checks + auto-fix)

> What it does: 7 health-check families (Dependencies, Permissions, Coherence,
> Orphans, Signing, Agent, Baseline) with severity, tiered exit codes (0/1/2/3),
> and a `--fix` convergence loop that re-evaluates until clean.
> Why it matters: this is the self-healing safety net. It previously had a
> destructive infinite-loop bug (now fixed via the reserved-block carve-out).

### T4.1 — Doctor read-only health check 🟢 🧪
**Command (real HOME to see your real state):**
```bash
./bin/gitid doctor; echo "exit=$?"
```
**Expected flow:** prints each of the 7 families with ✓ / ✗ / ! findings, a summary
line (`N error, N warning, N critical, N info`), and an `exit code: N`. Bare `doctor`
on a TTY may offer a fix gate ("Apply N fix(es)?" [N]) — decline to keep it read-only.
**Verify:** the printed `exit code` matches `echo "exit=$?"`:
- 0 = all pass · 1 = warnings/info only · 2 = error · 3 = critical.
**Considerations:** Run on real HOME so it inspects your actual `~/.ssh` perms and managed blocks.
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T4.2 — Doctor self-heals from a broken state 🟡 🧪
**Setup (sandbox: build a broken baseline, then break it):**
```bash
HOME="$GITID_SANDBOX" ./bin/gitid baseline setup   # answer the prompts, write it
# now corrupt: remove the include block but leave the fragment (incoherent state)
# (do this by editing $GITID_SANDBOX/.gitconfig to delete the baseline-include block)
HOME="$GITID_SANDBOX" ./bin/gitid doctor; echo "pre-fix exit=$?"
HOME="$GITID_SANDBOX" ./bin/gitid doctor --fix --yes; echo "post-fix exit=$?"
HOME="$GITID_SANDBOX" ./bin/gitid doctor; echo "verify exit=$?"
```
**Expected flow:** pre-fix reports the baseline finding (non-zero exit); `--fix --yes`
runs the full baseline setup to restore it (no prompts under `--yes`), re-evaluates,
and returns the post-fix code; the final read-only doctor returns **0**.
**KNOWN-FIXED:** this used to loop forever and exit non-zero even after fixing —
confirm it now converges and exits 0. If it loops or stays non-zero, that's a regression.
**Verify:** `verify exit=0` and the include block is back:
```bash
grep -n "baseline-include" "$GITID_SANDBOX/.gitconfig"
```
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T4.3 — Doctor permission tighten-only fix 🟡 🧪
**Setup (sandbox):**
```bash
mkdir -p "$GITID_SANDBOX/.ssh"; : > "$GITID_SANDBOX/.ssh/id_ed25519_x"
chmod 0644 "$GITID_SANDBOX/.ssh/id_ed25519_x"     # too-open private key
HOME="$GITID_SANDBOX" ./bin/gitid doctor --fix --yes
stat -f "%Sp" "$GITID_SANDBOX/.ssh/id_ed25519_x"
```
**Expected flow:** Permissions family flags `0644 (expected 0600)` [error/critical],
the fix chmods it to 0600; the `stat` shows `-rw-------`.
**Considerations:** doctor's perm fix is **tighten-only** (it will narrow perms, never widen).
> RESULT: [ ] ok  [ ] bug
> NOTES:

---

# PHASE 5 — CLI Surface + TUI

> What it does: the full Cobra command tree, shell completion, and a Bubble Tea TUI
> that launches to the doctor dashboard with identity navigation.
> Why it matters: this is the surface the user actually touches. Several UX gaps
> (G-01..G-04) and dead keybindings were logged here.

> **⚑ DIRECTION (decided 2026-06-13):** the CLI surface is fine and STAYS the
> scriptable engine. The **current TUI is a thin MVP** — a screen-stack of
> full-screen modals that *hands off to the CLI* for delete/rotate and shows blank
> list rows. The **target is a full, integrated TUI app** (see *Target TUI App*
> below the tests), modeled on `../tools-installer` (a Textual app) but in
> Go/Bubble Tea v2. So in T5.3–T5.5: confirm what exists, but **do not log
> cosmetic gaps as patch-it bugs** — they feed the rebuild. The throwaway thin
> TUI's polish (G-01/G-02/G-04, dead keys) gets *replaced*, not patched. The one
> thing worth getting right now is whatever the **core** must support — the
> create-flow (G-05) and provider/match reconstruction.

### T5.1 — Command tree & help 🟢 🧪
**Command:**
```bash
./bin/gitid --help
./bin/gitid identity --help
./bin/gitid baseline --help
./bin/gitid host --help
```
**Expected flow:** root lists `identity`, `baseline`, `doctor`, `host`, `completion`,
plus top-level aliases `rotate` and `copy`. Each group lists its subcommands.
**Verify:**
```bash
./bin/gitid --help | grep -E "identity|baseline|doctor|host|completion|rotate|copy"
```
**Expect:** all of those names appear.
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T5.2 — Shell completion scripts load cleanly 🟢 🧪 — KNOWN ⚠️ (auto-install gap)
**Command:**
```bash
./bin/gitid completion bash | bash -n && echo "bash OK"
./bin/gitid completion zsh  | zsh -n  && echo "zsh OK"
./bin/gitid completion fish | fish -c 'source -' 2>/dev/null && echo "fish OK"   # only if fish installed
```
**Expected flow:** each prints a valid completion script; `-n` (no-exec syntax check) passes.
**Expect:** `bash OK` and `zsh OK` (and `fish OK` if fish is present).
**KNOWN ⚠️:** completion is NOT auto-installed by `make install` — you must source it
manually. Candidate for a `make install-completions` target (next-phase polish).
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T5.3 — TUI launches to the dashboard 🟢 ⌨️
**Command (real HOME, interactive — needs a real terminal):**
```bash
./bin/gitid
```
**Expected flow:** enters alt-screen on the **Health Check Dashboard**; the 7 families
stream in with spinners; footer shows `q quit · Enter identities · r refresh · ? help`.
- `r` refreshes all checks.
- `Enter`/`a` → Identity List.
- `Esc` pops back, `q` quits.
**KNOWN ⚠️ G-02/G-04:** the footer hint bar is faint/linear (`StyleFaint`, space-separated) — easy to miss. Note if it reads OK to you.
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T5.4 — TUI drill-down + dead keys 🟢 ⌨️ — KNOWN ⚠️ (G-01, G-03, dead bindings)
**Command:** (from the dashboard) press `Enter` → Identity List → `Enter` on an item →
Identity Detail. In Detail try: `e` (edit form), `H` (add-host form), `c` (copy pubkey),
`d` / `R` (CLI handoff overlays), `Esc` (pop), `?` (help).
**Expected flow:** drill-down and Esc-pop work; `c` copies and shows a key-preview overlay.
**KNOWN ⚠️ to confirm:**
- **G-01:** empty Identity List renders blank (no "press `a` to create" empty-state) when no managed identities exist.
- **G-03:** `?` Help does **nothing** (defined in keymap, never handled in any Update).
- **Dead bindings:** `←/→` (`h`/`l`), `g` (top), `G` (bottom) are defined in the
  keymap but no screen handles them — pressing them should do nothing. Confirm.
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T5.5 — TUI in-app create (prove-before-write) 🔴 ⌨️ — KNOWN ⚠️ (G-05, destructive)
**Command:** from Identity List press `a` → fill the Create form (Tab/Shift+Tab cycle
fields; bad name shows inline red validation) → submit → **Prove screen** runs phase 1
(`ssh -T`) then phase 2 (`ssh -G`); the write confirm should only enable after both pass.
**Expected flow (intended):** write routes through the proven core only after the test
gate; backup path shown on success.
**KNOWN ⚠️ G-05:** same create-flow defect as T2.2 — the gate proves *reachability*,
not authenticated PASS, for new keys. Run only against a throwaway identity.
> RESULT: [ ] ok  [ ] bug
> NOTES:

### T5.6 — `gitid copy <name>` end-to-end 🟡 🧪
**Precondition:** a real managed identity exists (from T2.2).
**Command:**
```bash
./bin/gitid copy <name>
./bin/gitid identity copy <name>   # same thing under the identity group
```
**Expected flow:** copies the `.pub` to the clipboard via the real clipboard tool and
prints provider upload instructions. On clipboard failure it prints the key to copy manually.
**Verify (macOS):**
```bash
pbpaste | head -c 40; echo
```
**Expect:** the clipboard contains the `ssh-ed25519 ...` public key line.
**Considerations:** blocked if no managed identity exists.
> RESULT: [ ] ok  [ ] bug  [ ] blocked
> NOTES:

---

# Quick run sheet (copy-paste order)

```
# build + snapshot
go build -o bin/gitid ./cmd/gitid
export GITID_SANDBOX=/tmp/gitid-sandbox; rm -rf "$GITID_SANDBOX"; mkdir -p "$GITID_SANDBOX"

# safe (sandbox / read-only / dry-run)
make build && make test && make lint                                  # T1.1
HOME="$GITID_SANDBOX" ./bin/gitid identity add --dry-run              # T2.1 (mode 1)
./bin/gitid identity list                                            # T3.1
HOME="$GITID_SANDBOX" ./bin/gitid baseline setup --dry-run           # T3.1a
HOME="$GITID_SANDBOX" ./bin/gitid baseline setup                     # T3.1b (then re-run = idempotent)
HOME="$GITID_SANDBOX" ./bin/gitid baseline show                      # T3.1c
./bin/gitid doctor; echo $?                                          # T4.1
./bin/gitid completion bash | bash -n && echo ok                     # T5.2
./bin/gitid --help                                                   # T5.1

# self-heal + perms (sandbox)
HOME="$GITID_SANDBOX" ./bin/gitid doctor --fix --yes; echo $?        # T4.2/T4.3

# interactive TUI (real terminal)
./bin/gitid                                                          # T5.3/T5.4/T5.5

# destructive (real HOME, snapshot first!)
./bin/gitid identity add                                             # T2.2 (G-05)
./bin/gitid identity test <alias>                                    # T2.3
./bin/gitid copy <name>                                              # T5.6
```

---

# Target TUI App (the rebuild — Phase B, after reconciliation)

> Decided 2026-06-13. Medium = **terminal**, Bubble Tea v2 + Lipgloss v2 + Bubbles v2
> (runs over SSH; the committed stack). Ergonomics modeled on `../tools-installer`
> (a Textual app: one app shell, a base screen + shallow stack, a view palette with
> `1..N` keys, rich widgets). The current TUI feels "pobre" because of *scope*, not
> the framework. Target shape:

- **One integrated app shell**, persistent layout — not full-screen modal commands:
  - **Header**: app title + active view + global health badge.
  - **Left sidebar**: list of identities; each row shows name · provider · alias/site
    count (this is the master list — it never disappears).
  - **Main pane (master-detail)**: selecting an identity fills it — fields, all its
    **aliases/sites**, **match conditions** (gitdir / URL), signing state, and
    per-identity **health badges**.
  - **Footer**: bold, comma-separated key hints (fix for G-02/G-04).
- **View switcher** (command palette + `1..N`), like the installer's
  catalog/doctor/fix/…:
  - **Identities** (default) · **Health** (doctor, integrated — findings as badges +
    actionable fixes) · **Global Options** (baseline) · later: Cleanup/Uninstall.
- **In-app create / add wizard with LIVE feedback** — this IS the G-05 flow:
  keygen progress → copy **public** key + upload instructions → connectivity test
  running → PASS/fail → **retry / skip-&-save / quit** (persist only after PASS,
  with an explicit skip escape). Same wizard for add-account (new alias/site).
- **Per-site + global options editable in-pane**: global = baseline; per-site =
  match strategy (gitdir / URL / both) + signing + port + hostname.
- **Match strategy picker**: gitdir (folder), URL (`hasconfig:remote.*.url`), or both
  — surfaces the new per-URL requirement; warns on overlapping matches (shared with
  the new doctor check).
- **Copy = public key only** (private key never copied; reveal path at most).
- Mouse + keyboard; modal confirms for destructive actions (delete/rotate done
  in-app, no CLI handoff).

---

# ===================== FEEDBACK ZONE (you fill this) =====================

> Drop everything here as you go. Free-form is fine — bullet per observation.
> When you're done, tell me and I'll turn this into the next ROADMAP phase.
> Pre-seeded with the gaps already logged in 05-UAT.md so they're not lost — strike
> through (`~~...~~`) any that turn out to be fixed.

## Confirmed bugs / gaps (from prior UAT — confirm or strike)
- **G-01** (UX): empty TUI Identity List has no empty-state message.
- **G-02/G-04** (UX): dashboard/footer hint bars too faint/linear; want bold key tokens, comma-separated.
- **G-03** (FUNCTIONAL): `?` Help key does nothing (defined, never wired).
- **G-05** (HIGH): create-flow doesn't generate→upload→wait→loop-test-until-PASS→persist-after-PASS (with `[s]` skip). Proves reachability, not auth.
- **Completion auto-install** (polish): `make install` doesn't install shell completion.
- **Dead keybindings**: `←/→` (h/l), `g`, `G` defined but unwired.
- **Visualization scope**: TUI shows only gitid-managed identities, not hand-written Host blocks / existing keys.

## Decisions locked (2026-06-13)
- **Copy = public key only.** Private key is never copied to the clipboard; at most reveal its path.
- **Sequencing = (A) reconciliation first, (B) full TUI app after.** (A) fixes CLI/core
  bugs; (B) is the integrated TUI rebuild (see *Target TUI App* above).
  > Recommendation: in (A) do NOT polish the throwaway thin TUI's cosmetics (G-01/G-02/G-04, dead keys) — they're replaced in (B). (A) = core/CLI only.
- **TUI medium = terminal, Bubble Tea v2** (runs over SSH); ergonomics modeled on `../tools-installer`.

## Findings extracted by Claude from inline notes (2026-06-13) — confirm/correct
- **F-1 (install PATH, T1.2):** `make install` runs but `gitid not found` — GOBIN not on PATH; no install-location/PATH feedback. → print install path + PATH hint; fold completion install.
- **F-2 (create-flow, T2.2 = G-05 sharpened):** wrote on `y` BEFORE upload and proceeded despite `Permission denied (publickey)`. Wanted: generate → prompt upload → test → show success/error → retry / discard / quit / save / continue.
- **F-3 (provider mis-derived):** `identity list` shows `provider: ssh.github.com` for `user_z3r0_gh` (should be `github`) — provider derived from hostname when host-alias ≠ `name.provider`. Reconstruction bug.
- **F-4 (TUI list rows blank):** with identities present, list rows render with no name/identification (worse than G-01 empty-state). [screenshot tmp/images/image.png]
- **F-5 (terminology):** rename `alias` prompt → "Host alias".
- **F-6 (matching, NEW req):** support per-URL (`hasconfig:remote.*.url`) matching + per-folder (`gitdir`) + combinations; prompt for strategy during add (e.g. `user.personal.github.com` vs `companyname.github.com`).
- **F-7 (doctor, NEW check):** detect overlapping/duplicate match conditions (two ids both match `gitdir:~/git/personal/` = ambiguous "who wins"); share detection so add/update warn too.
- **F-8 (integration):** dashboard must wire the real identity list; `test` (and all actions) integrated into one base UI — not separate commands.
- **F-9 (T3.1 "3 sections, 2 identities") — NOT a bug, clarified:** the 3 SSH managed
  blocks are `castocolina`, `_global`, `user_z3r0_gh`. `_global` is the reserved
  `Host *` block (like `baseline-include` in gitconfig), not an identity — so `list`
  correctly shows 2. The playbook's `grep -c "BEGIN gitid managed"` Verify over-counts
  (it includes `_global`). Action: keep `_global` registered as a reserved block
  (doctor must not treat it as an identity/orphan), same carve-out as `baseline-include`.

## New findings (add below)
- 

## Per-test notes overflow (if a NOTES line wasn't enough)
- 

## Overall impression / priorities
- 

# =========================================================================
