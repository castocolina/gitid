# Phase 2: First Identity End-to-End - Research

**Researched:** 2026-06-09
**Domain:** ed25519 SSH key generation, coordinated SSH/Git config artifact writing, two-phase SSH connectivity testing, safe atomic file mutation (Go CLI)
**Confidence:** HIGH (all critical API signatures and shell behaviors empirically verified this session)

## Summary

Phase 2 fills real logic into the Phase-1 stub packages to deliver one create-new
identity end-to-end: generate an ed25519 key, write four coordinated artifacts
(SSH `Host` block, gitconfig `includeIf`, per-identity fragment, `allowed_signers`
line) safely (backup → atomic write → idempotent managed-block rewrite → confirmation
→ correct permissions), and prove correctness via the two-phase test flow that prints
input command and real output. The stack is already pinned and verified — no new
dependencies beyond the four in STACK.md (`golang.org/x/crypto` v0.53.0,
`github.com/kevinburke/ssh_config` v1.6.0, `github.com/spf13/cobra` v1.10.2,
`github.com/atotto/clipboard` v0.1.4). All four pinned versions and the four
`x/crypto/ssh` function signatures were confirmed against the Go proxy and pkg.go.dev
during this research session.

Two findings **correct the CONTEXT.md design** and must reach the planner:
1. **The algorithm-capability probe in D-09 is wrong.** `ssh-keygen -Q key` does NOT
   enumerate supported key types — on OpenSSH 9.7 it returns `KRL checking requires an
   input file`. The correct probe is **`ssh -Q key`** (lists `ssh-ed25519`, `ecdsa-*`,
   `ssh-rsa`, …), confirmed empirically below. The planner must use `ssh -Q key`.
2. **`ssh -T` exits 0 even on "Permission denied (publickey)"** on this toolchain
   (verified: exit=0 with that message). This validates D-01's "output string is the
   primary signal, exit code only corroborating" — but it means the classifier must
   *not* treat exit code 0 as success. Classify strictly by output substring.

**Primary recommendation:** Build `internal/filewriter` first (it is the SAFE-01/02/03 +
KEY-02 chokepoint every other writer reuses), then `keygen` + `platform`/`deps`, then the
two renderers (`sshconfig`, `gitconfig`), then `tester`, then wire `gitid identity add`
in Cobra. Classify SSH test results by output substring, never exit code. Use `ssh -Q key`
for the algorithm probe.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| ed25519 key generation + OpenSSH serialization | `internal/keygen` | `internal/platform` (algo probe shell-out) | Pure crypto + file write; isolates `x/crypto/ssh` usage |
| Algorithm capability probe (`ssh -Q key`) + fallback chain | `internal/platform` / `internal/deps` | `internal/keygen` (consumer) | OS/toolchain concern; D-14 install hints live here (shared seam with Phase 4 doctor) |
| Safe write (backup, temp→fsync→rename, chmod, restore) | `internal/filewriter` | — | Single chokepoint; SSH + gitconfig writers both delegate here |
| SSH config managed-block render + round-trip parse | `internal/sshconfig` | `internal/filewriter` (write) | Owns ssh_config syntax via kevinburke/ssh_config; delegates write safety |
| gitconfig key/value sets in fragment | `internal/gitconfig` (via `git config` os/exec) | — | git is the authoritative parser of its own format |
| gitconfig `includeIf`/`include` headers + fragment file | `internal/gitconfig` (raw sentinel text) | `internal/filewriter` (write) | `git config` cannot write `includeIf` headers natively |
| `allowed_signers` line generation + file write | `internal/keygen` (line + `WriteAllowedSigners`) | `internal/filewriter` (write safety) | Built from same pubkey + email used in fragment; persisted via the filewriter chokepoint into `~/.ssh/allowed_signers` (SIGN-01) |
| Two-phase SSH test (`ssh -i`, `ssh -T`, `ssh -G`) | `internal/tester` | — | Pure os/exec input/output; read-only, no side effects |
| Clipboard copy of `.pub` | `internal/clipboard` | `internal/deps` (tool detection) | Cross-platform dispatch via atotto/clipboard |
| Interactive prompts + orchestration | `cmd/gitid` (Cobra `identity add`/`identity test`) | all internal packages | Thin handler; D-04 real command, foundation for Phase 5 |
| Upload instructions (auth + signing) | `cmd/gitid` (presentation) | — | Static per-provider copy; UP-01/UP-02 |

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| IDENT-01 | Create identity generating ed25519 key (auth+signing) | keygen via `ed25519.GenerateKey` + `ssh.MarshalPrivateKey` (verified); orchestrated by `identity add` |
| IDENT-02 | Create identity reusing existing key | fast-follow plan; point Account at existing key path, skip keygen, copy `.pub` |
| IDENT-06 | Account maps identity to provider via alias | fast-follow plan; render second `Host <alias>` block sharing one key path |
| KEY-01 | Rotate/replace key (artifacts re-point, re-test) | **In Phase 2 scope** (Open Question 1 RESOLVED — KEY-01 is in the Phase-2 ID list); thin fast-follow slice in 02-07 reuses keygen + the four writers to re-point + re-test |
| KEY-02 | Correct permissions (`~/.ssh` 700, key 600, `.pub` 644, `config` 600) | filewriter `os.Chmod` after rename; keygen sets key/pub modes; Pitfall 6 |
| SSH-01 | Managed `Host <alias>` block with Hostname/Port/User git/IdentityFile/IdentitiesOnly yes | sshconfig renderer; verified `ssh -G` key names |
| SSH-02 | Default identity uses real host; others use aliases | alias form `<identity>.<provider>` (D-12); renderer supports both |
| SSH-03 | macOS `Host *` block: IgnoreUnknown UseKeychain → UseKeychain yes + AddKeysToAgent yes, ordered last | platform-guarded; Pitfall 4 + 5 (placement + ordering) |
| GIT-01 | Managed `[includeIf "<match>"]` block pointing to fragment | raw sentinel text write (git cannot write includeIf) |
| GIT-02 | Match strategy: `gitdir:` (default, trailing slash) + `hasconfig:remote.*.url`, combinable | renderer emits both; Pitfall 8 (trailing slash) |
| GIT-03 | Fragment sets user.name/email, gpg.format=ssh, user.signingkey, commit.gpgsign true | `git config --file <fragment>` sets; Pitfall 9 (no `[remote]` in fragment) |
| SIGN-01 | `allowed_signers` line `<email> namespaces="git" ssh-ed25519 AAAA…`, email byte-identical to user.email | verified format below; build from `MarshalAuthorizedKey` output AND persist into `~/.ssh/allowed_signers` via `keygen.WriteAllowedSigners` (idempotent managed block) |
| SIGN-02 | `user.signingkey` references pubkey file path, never inline | Pitfall 11; renderer uses path form |
| TEST-01 | Pre-write `ssh -i <key> -o IdentitiesOnly=yes -T git@<host>` proving key authenticates | tester phase 1; classify by output (D-01) |
| TEST-02 | Post-write `ssh -T <alias>` + `ssh -G <alias>` proving resolved IdentityFile | tester phase 2; parse `ssh -G` lowercase keys; reusable via `gitid identity test <name>` |
| TEST-03 | Every test prints command (input) and real output | tester returns structured result with raw cmd string + combined output |
| SAFE-01 | Timestamped backup before write (`<file>.bak.<ts>`) | filewriter step 1 |
| SAFE-02 | Idempotent whole-block rewrite; content outside blocks preserved verbatim | sentinel scan-and-replace; Pitfall 1 |
| SAFE-03 | Atomic write (temp→rename→chmod); no write without confirmation | filewriter; confirmation callback in `identity add` |
| CLIP-01 | Public key copied to clipboard on generate | atotto/clipboard `WriteAll` |
| CLIP-02 | Cross-platform clipboard, graceful failure when no tool | atotto/clipboard + `Unsupported` error handling |
| UP-01 | GitHub/GitLab steps to add key for authentication | static copy + URLs (below) |
| UP-02 | GitHub/GitLab steps to add key for signing | static copy + URLs; separate registration required |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/crypto/ssh` | v0.53.0 | ed25519 OpenSSH private/public serialization, allowed_signers | stdlib extension; no third-party needed [VERIFIED: proxy.golang.org @latest = v0.53.0] |
| `crypto/ed25519` (stdlib) | Go 1.26 | Key pair generation (`GenerateKey`) | stdlib; implements `crypto.Signer` |
| `github.com/kevinburke/ssh_config` | v1.6.0 | Parse + render `~/.ssh/config` round-trip | only maintained comment-preserving Go parser [VERIFIED: proxy.golang.org @latest = v1.6.0, 2026-02-16] |
| `github.com/spf13/cobra` | v1.10.2 | `gitid identity add` command (D-04) | de-facto Go CLI standard [VERIFIED: proxy.golang.org @latest = v1.10.2, 2025-12-03] |
| `github.com/atotto/clipboard` | v0.1.4 | Cross-platform clipboard (pbcopy/wl-copy/xclip) | single import, platform dispatch [VERIFIED: proxy.golang.org @latest = v0.1.4] |
| `os/exec` (stdlib) | Go 1.26 | `git config`, `ssh`, `ssh -Q`, `ssh-add` shell-out | git owns its own format; ssh owns resolution |

### Verified function signatures (`golang.org/x/crypto/ssh` v0.53.0)
```go
func MarshalPrivateKey(key crypto.PrivateKey, comment string) (*pem.Block, error)
func MarshalPrivateKeyWithPassphrase(key crypto.PrivateKey, comment string, passphrase []byte) (*pem.Block, error)
func MarshalAuthorizedKey(key PublicKey) []byte        // return value ENDS WITH newline
func NewPublicKey(key interface{}) (PublicKey, error)
func ParsePrivateKey(pemBytes []byte) (Signer, error)
```
[CITED: pkg.go.dev/golang.org/x/crypto/ssh] — all confirmed this session.

**Installation (additive to Phase-1 go.mod):**
```bash
go get golang.org/x/crypto@v0.53.0
go get github.com/kevinburke/ssh_config@v1.6.0
go get github.com/spf13/cobra@v1.10.2
go get github.com/atotto/clipboard@v0.1.4
```

## Package Legitimacy Audit

> slopcheck was not available this session. All four packages are pre-pinned in the
> project's prior verified STACK.md research and were independently re-confirmed against
> the Go module proxy (`proxy.golang.org/<mod>/@latest`) during this session. Go modules
> have no install/postinstall scripts; integrity is enforced via `go.sum` checksums.

| Package | Registry | Age | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-------------|-----------|-------------|
| golang.org/x/crypto | Go proxy | mature (golang.org official) | github.com/golang/crypto | unavailable | Approved [VERIFIED: registry] |
| github.com/kevinburke/ssh_config | Go proxy | v1.6.0 2026-02-16 | github.com/kevinburke/ssh_config (Tailscale/Indeed sponsored) | unavailable | Approved [VERIFIED: registry] |
| github.com/spf13/cobra | Go proxy | v1.10.2 2025-12-03 | github.com/spf13/cobra | unavailable | Approved [VERIFIED: registry] |
| github.com/atotto/clipboard | Go proxy | v0.1.4 2021-02-24 | github.com/atotto/clipboard | unavailable | Approved [VERIFIED: registry] |

**Packages removed due to [SLOP]:** none
**Packages flagged [SUS]:** none (`atotto/clipboard` is old but stable, widely used, and named verbatim in CLAUDE.md — not a hallucination risk)

## Architecture Patterns

### System Architecture Diagram

```
  gitid identity add  (cmd/gitid — Cobra, interactive prompts D-05)
        │
        ▼
  collect inputs: name, git name, git email, provider, host binding(alias|real), match dir
        │
        ▼
  platform.ProbeAlgorithms()  ──► run `ssh -Q key`  ──► parse supported types
        │                                                  │
        │  ed25519 present? ──no──► offer fallback ed25519→rsa-4096→ecdsa (D-09)
        │                          none available? ──► STOP + per-OS install hint (D-14)
        ▼ (ed25519 default path)
  keygen.Generate(name, comment, passphrase?)
        │  ed25519.GenerateKey → ssh.MarshalPrivateKey (+WithPassphrase if set)
        │  write ~/.ssh/id_ed25519_<name> (0600), .pub (0644); ~/.ssh dir 0700
        ▼
  clipboard.WriteAll(pubKeyLine)              (CLIP-01)
        ▼
  tester.PreWrite(keyPath, host)  ──► ssh -i <key> -o IdentitiesOnly=yes -T git@<host>
        │  classify by OUTPUT SUBSTRING (D-01):
        │    "successfully authenticated"      → PASS (reused/uploaded key)
        │    "Permission denied (publickey)"   → REACHABLE-BUT-NOT-UPLOADED (new key, expected)
        │    refused/DNS/timeout (+exit code)  → FAILURE → abort, no write
        ▼ (PASS or REACHABLE)
  RENDER all four artifacts (in-memory) → unified preview/diff
        ▼
  CONFIRM (single explicit y/N; --dry-run skips write, SAFE-03)
        ▼
  filewriter.Write × 4   (backup → temp → fsync → rename → chmod; idempotent block rewrite)
    ├─ ~/.ssh/config           (managed Host <alias> block + macOS Host * block)
    ├─ ~/.gitconfig            (managed [include] + [includeIf] sentinel block)
    ├─ ~/.gitconfig.d/<name>   (fragment: git config --file sets the keys)
    └─ ~/.ssh/allowed_signers  (keygen.WriteAllowedSigners: managed per-identity block line: <email> namespaces="git" ssh-ed25519 …)
        ▼
  ssh-add (D-08; macOS: ssh-add --apple-use-keychain) load key into agent/Keychain
        ▼
  tester.Resolved(alias)  ──► ssh -T git@<alias>  +  ssh -G <alias>   (reusable via `gitid identity test`)
        │  assert ssh -G resolved: identityfile, identitiesonly yes, user git, hostname, port (D-03)
        ▼
  print upload steps (auth + signing URLs) → user confirms after uploading (D-02)
```

### Recommended Project Structure (fills Phase-1 stubs)
```
internal/
├── filewriter/   # backup, temp→fsync→rename, chmod, restore, idempotent block replace  [BUILD FIRST]
├── platform/     # OS detect, `ssh -Q key` probe, per-OS install hints (D-14 mini-DOC-01)
├── deps/         # tool presence (ssh, ssh-keygen, git, ssh-add, clipboard tools)
├── keygen/       # ed25519 gen + OpenSSH serialize + allowed_signers line + WriteAllowedSigners
├── sshconfig/    # parse (kevinburke) + render Host stanza + Host* block; delegate write
├── gitconfig/    # `git config --file` sets + raw includeIf sentinel block + fragment file
├── identity/     # Account model; orchestrate create (deps injected as params)
├── tester/       # ssh -i / ssh -T / ssh -G; classify by output substring
├── clipboard/    # atotto/clipboard wrapper
cmd/gitid/        # identity add (+ identity test); thin Cobra handlers, prompts
```

### Pattern 1: Safe Write (filewriter) — the SAFE-01/02/03 + KEY-02 chokepoint
**What:** Every file mutation goes through one function. Backup → render full content
(foreign lines preserved + managed blocks) → write to unique temp in same dir → `Sync()` →
`Chmod` → `os.Rename` (atomic on same filesystem) → return backup path for restore.
**When:** Every write to `~/.ssh/config`, `~/.gitconfig`, fragments, `allowed_signers`.
```go
// Source: pattern from .planning/research/ARCHITECTURE.md §"Safe Write Flow" + PITFALLS P7
func Write(targetPath string, content []byte, mode os.FileMode) (backupPath string, err error) {
    // 1. backup (only if target exists): copy → targetPath + ".bak." + time.Now().Format("20060102-150405")
    //    apply SAME restrictive mode to the backup (PITFALLS security: backup is world-readable risk)
    // 2. tmp, _ := os.CreateTemp(filepath.Dir(targetPath), "gitid-*.tmp")  // unique name, NOT fixed .tmp
    // 3. tmp.Write(content); tmp.Sync(); tmp.Close()
    // 4. os.Chmod(tmp.Name(), mode)                     // 0600 config, 0644 .pub, 0600 key
    // 5. os.Rename(tmp.Name(), targetPath)              // atomic
    // 6. ensure parent dir mode (~/.ssh 0700) via os.MkdirAll + os.Chmod
}
```
Key rules (verified from PITFALLS): never `os.WriteFile` in place; never a fixed `.tmp`
suffix; always explicit `os.Chmod` (never rely on umask); apply 0600 to backups too.

### Pattern 2: Idempotent Sentinel-Delimited Managed Block
**What:** Each owned region is wrapped in markers; on write, scan for the pair, replace the
whole range; content outside is byte-identical after the run (SAFE-02 idempotency proof).
```
# BEGIN gitid managed: <identity-name>
...generated content...
# END gitid managed: <identity-name>
```
**When:** SSH `Host` blocks, gitconfig `includeIf` blocks, AND `~/.ssh/allowed_signers`
lines (keyed per identity name so multiple identities and re-runs never duplicate signing
lines). The macOS `Host *` block is also a managed block but keyed by a fixed sentinel
(e.g. `# BEGIN gitid managed: _global`) so it is rewritten idempotently and always ordered
LAST (Pitfall 5).
**Idempotency proof:** second `gitid identity add` (or re-render of same Account) → `diff` is
empty. This is a required verification step.

### Pattern 3: gitconfig — `git config` for values, raw text for includeIf
**What:** Fragment key/values are set via `git config --file ~/.gitconfig.d/<name> section.key value`
(idempotent, git owns the format). The `[include]` / `[includeIf]` *headers* in `~/.gitconfig`
cannot be created by `git config` — write them as a raw sentinel-delimited managed block.
```go
// fragment writes (idempotent, comment-safe — git is authoritative):
exec.Command("git", "config", "--file", fragPath, "user.name", name)
exec.Command("git", "config", "--file", fragPath, "user.email", email)
exec.Command("git", "config", "--file", fragPath, "gpg.format", "ssh")
exec.Command("git", "config", "--file", fragPath, "user.signingkey", pubKeyPath) // PATH not inline (SIGN-02)
exec.Command("git", "config", "--file", fragPath, "commit.gpgsign", "true")
// global allowedSignersFile (managed-block or git config --global):
exec.Command("git", "config", "--file", gitconfigPath, "gpg.ssh.allowedSignersFile", allowedSignersPath)
// includeIf header → raw managed block appended/replaced in ~/.gitconfig:
//   # BEGIN gitid managed: <name>
//   [includeIf "gitdir:~/git/<name>/"]
//   	path = ~/.gitconfig.d/<name>
//   # END gitid managed: <name>
```
Never use `os/exec` with shell expansion — stdlib `os/exec` bypasses the shell; pass args
as a slice (gosec G204 clean). [CITED: CLAUDE.md §gitconfig strategy]

### Anti-Patterns to Avoid
- **Blind append / grep-guard** (legacy script): duplicates blocks on re-run. Use sentinel rewrite.
- **`os.WriteFile` in place**: non-atomic; partial-read race can lock you out of SSH.
- **`UseKeychain yes` without `IgnoreUnknown UseKeychain` first**: errors on Linux `ssh -G`.
- **`Host *` placed first**: first-match-wins means specific aliases can't override. Must be LAST.
- **Inline `user.signingkey`**: stale after rotation. Use the `.pub` path.
- **`[remote]` in a fragment under `hasconfig:`**: hard git error (circular). Fragment is identity-only.
- **`chmod 600` on `.pub`**: clipboard/upload tools fail. `.pub` must be 0644.
- **Classifying SSH test by exit code**: `ssh -T` exits 0 on "Permission denied" here. Use output substring.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| ed25519 → OpenSSH PEM | Custom OpenSSH key serializer | `ssh.MarshalPrivateKey` | Binary format with checksum/padding; easy to corrupt |
| authorized_keys line | Manual base64 wire-format encoding | `ssh.MarshalAuthorizedKey(ssh.NewPublicKey(pub))` | Wire format is fiddly; trailing newline handled |
| ssh_config parse/render | Custom tokenizer | `kevinburke/ssh_config` | Comment/whitespace round-trip is hard; only maintained lib |
| gitconfig key/value writes | Custom INI parser | `git config --file` via os/exec | git is the authoritative parser; comment-safe by design |
| Algorithm enumeration | Hardcode "ed25519 always works" | `ssh -Q key` probe | Some toolchains lack ed25519 (D-09); probe prevents hard failure |
| Cross-platform clipboard | per-OS `exec` dispatch | `atotto/clipboard.WriteAll` | pbcopy/wl-copy/xclip detection in one import (CLIP-02) |
| Atomic write | `os.WriteFile` + chmod | temp→fsync→rename→chmod | Non-atomic write corrupts SSH config under crash/race |

**Key insight:** Every "format" in this phase (OpenSSH key, authorized_keys, allowed_signers,
gitconfig, ssh_config) has a non-obvious edge that a hand-rolled implementation gets subtly
wrong. The only thing gitid legitimately hand-rolls is the **sentinel managed-block scan/replace**
(no Go lib supports `includeIf` write-back) — and that is a bounded line-range operation, not a
full-format parser.

## Common Pitfalls

### Pitfall 1: `ssh-keygen -Q key` is NOT the algorithm probe (CONTEXT D-09 correction)
**What goes wrong:** D-09 specifies `ssh-keygen -Q key` to enumerate supported key types.
On OpenSSH 9.7 this returns `KRL checking requires an input file` — it's the KRL-query mode,
not an algorithm list.
**How to avoid:** Use **`ssh -Q key`** (verified output: `ssh-ed25519`, `ecdsa-sha2-nistp256`,
`ssh-rsa`, …). Probe for membership: `ssh-ed25519` present → ed25519 path; else check fallback
chain rsa→ecdsa against the same list; none present → D-14 stop + install hint.
**Warning signs:** probe always "fails" or always returns the KRL error string.

### Pitfall 2: `ssh -T` exit code is unreliable (validates D-01)
**What goes wrong:** `ssh -T git@github.com` returns exit code **0** even when it prints
`Permission denied (publickey).` (verified this session). A classifier keyed on exit code
treats the new-key case as success.
**How to avoid:** Classify strictly by output substring per D-01. Capture combined stdout+stderr
(`cmd.CombinedOutput()` or separate buffers) and match `successfully authenticated` /
`Permission denied (publickey)` / connection errors. Exit code is corroborating only.

### Pitfall 3: `ssh -G` emits lowercase keys; `identityfile` can repeat
**What goes wrong:** Parsing `ssh -G <alias>` expecting `IdentityFile` (camelCase) finds nothing.
**How to avoid:** Keys are lowercase: `user`, `hostname`, `port`, `identitiesonly`, `identityfile`
(verified). `identityfile` may appear multiple lines — assert the expected path is present among
them, and `identitiesonly yes`. Match on `^<key> ` prefix, case-sensitive lowercase.

### Pitfall 4: `IgnoreUnknown UseKeychain` placement (Linux portability, SSH-03)
**What goes wrong:** `UseKeychain` is Apple-only; Linux `ssh` errors `Bad configuration option`.
**How to avoid:** Emit, in this order, inside `Host *`: `IgnoreUnknown UseKeychain` then
`UseKeychain yes` then `AddKeysToAgent yes`. `IgnoreUnknown` must precede the unknown directive.

### Pitfall 5: `Host *` block must be ordered LAST (first-match-wins)
**What goes wrong:** SSH applies first match per directive; `Host *` first prevents alias override.
**How to avoid:** Renderer always places specific `Host <alias>` blocks before the `Host *` block.

### Pitfall 6: File permissions — explicit chmod, never umask
| Path | Mode |
|------|------|
| `~/.ssh/` | 0700 |
| `~/.ssh/config` | 0600 |
| private key | 0600 |
| `.pub` | 0644 |
| `allowed_signers` | 0644 (readable; not secret) |
| `~/.gitconfig` / fragment | 0644 (standard) / backups 0600 |
**How to avoid:** `os.Chmod` after every write; apply 0600 to config/key backups.

### Pitfall 7: `gitdir:` trailing slash required
**What goes wrong:** `gitdir:~/git/<id>` (no slash) matches only the exact path, not repos inside.
**How to avoid:** Always emit `gitdir:~/git/<identity>/` with trailing slash (D-13). Test with a
real `~/git/<id>/repo/.git`, assert `git config user.email` resolves correctly.

### Pitfall 8: `allowed_signers` email byte-identical + `namespaces="git"`
**What goes wrong:** Email case/byte mismatch vs `user.email`, or missing namespace → signature
shows "unverified" or untrusted-principal.
**How to avoid:** Build the line from the SAME email string used in the fragment. Format
(verified): `<email> namespaces="git" <MarshalAuthorizedKey output without trailing newline>`.
Note `MarshalAuthorizedKey` already appends `\n` — strip it when composing, re-add one newline.

### Pitfall 9: Fragment must not contain `[remote]` (hasconfig circular error)
**What goes wrong:** A remote URL in a fragment included via `hasconfig:` is a hard git error.
**How to avoid:** Fragment is identity-only: user.name/email, gpg.format, user.signingkey,
commit.gpgsign. Reject `[remote]` at render time.

### Pitfall 10: ed25519 pointer-vs-value (parse side only)
**What goes wrong:** golang/go#51974 — `ssh.ParseRawPrivateKey` returns `*ed25519.PrivateKey`
(pointer) while `ed25519.GenerateKey` returns a value, causing type-assertion confusion.
**How to avoid:** For the *generate→marshal* path (this phase), **both value and pointer work**
with `ssh.MarshalPrivateKey` at v0.53.0 (verified empirically below). Pass the value from
`GenerateKey` directly. Only worry about the pointer form if you later parse keys back.

## Code Examples

### ed25519 generation → OpenSSH private key + public key (verified empirically)
```go
// Source: empirically verified with golang.org/x/crypto v0.53.0, Go 1.26, this session
pub, priv, err := ed25519.GenerateKey(rand.Reader)   // crypto/ed25519
if err != nil { return err }

block, err := ssh.MarshalPrivateKey(priv, comment)   // value type works; comment e.g. "<identity>@gitid"
// OR, if passphrase set (D-07 optional):
// block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, comment, []byte(passphrase))
privPEM := pem.EncodeToMemory(block)                 // "-----BEGIN OPENSSH PRIVATE KEY-----" (verified)
// filewriter.Write(privPath, privPEM, 0600)

sshPub, err := ssh.NewPublicKey(pub)
pubLine := ssh.MarshalAuthorizedKey(sshPub)          // "ssh-ed25519 AAAA...\n" (trailing newline)
// filewriter.Write(pubPath, pubLine, 0644)
```

### allowed_signers line + file write (verified)
```go
// MarshalAuthorizedKey ends with '\n'; trim it, then compose one line + newline.
keyText := strings.TrimRight(string(pubLine), "\n")
signersLine := fmt.Sprintf("%s namespaces=\"git\" %s\n", userEmail, keyText)
// verified output e.g.: me@example.com namespaces="git" ssh-ed25519 AAAAC3Nza...
//
// Persist into ~/.ssh/allowed_signers via the filewriter chokepoint as an idempotent
// per-identity managed block (keyed by identity name) so re-runs and multiple identities
// never duplicate signing lines (SAFE-02):
existing, _ := os.ReadFile(allowedSignersPath)                       // empty if absent
composed := filewriter.ReplaceBlock(existing, identity, strings.TrimRight(signersLine, "\n"))
filewriter.Write(allowedSignersPath, composed, 0644)                 // 0644 — readable, not secret
```

### Algorithm probe (CORRECTED — `ssh -Q key`)
```go
// Source: empirically verified — `ssh -Q key` on OpenSSH 9.7 lists supported key types.
out, err := exec.Command("ssh", "-Q", "key").Output()
supported := strings.Split(strings.TrimSpace(string(out)), "\n")
// has "ssh-ed25519" → ed25519 path (default).
// else walk fallback chain ed25519→rsa(4096)→ecdsa against `supported`; none → D-14 stop.
```

### Two-phase test classification (D-01) with combined output
```go
// Pre-write: ssh -i <key> -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=10 -T git@<host>
cmd := exec.Command("ssh", "-i", keyPath, "-o", "IdentitiesOnly=yes",
    "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-T", "git@"+host)
out, _ := cmd.CombinedOutput()                 // ignore exit code (unreliable; verified exit=0 on denial)
s := string(out)
switch {
case strings.Contains(s, "successfully authenticated"):
    return PASS                                // reused/uploaded key
case strings.Contains(s, "Permission denied (publickey)"):
    return REACHABLE_NOT_UPLOADED              // new key, expected → proceed to write (D-02)
default:
    return FAILURE                             // refused/DNS/timeout → abort, no write
}
// TEST-03: store cmd.String() (input) + s (output) in the result, print both.
```

### Resolved test parse (`ssh -G`, lowercase keys verified)
```go
out, _ := exec.Command("ssh", "-G", alias).Output()
for _, line := range strings.Split(string(out), "\n") {
    switch {
    case strings.HasPrefix(line, "identityfile "):  // may repeat
    case strings.HasPrefix(line, "identitiesonly "): // assert "identitiesonly yes"
    case strings.HasPrefix(line, "user "):           // assert "user git"
    case strings.HasPrefix(line, "hostname "):
    case strings.HasPrefix(line, "port "):
    }
}
```

### Clipboard (atotto)
```go
// Source: github.com/atotto/clipboard v0.1.4
if err := clipboard.WriteAll(string(pubLine)); err != nil {
    // CLIP-02 graceful failure: warn "no clipboard tool found; copy manually:" + print pubLine
}
```

## Runtime State Inventory

> Phase 2 is greenfield create-new logic (no rename/migration). One subtle live-state item:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no datastore; filesystem is source of truth | None |
| Live service config | **ssh-agent / macOS Keychain** holds loaded keys at runtime (D-08 loads the new key via `ssh-add`). If a stale key with same alias was previously loaded, the agent may offer it. | `IdentitiesOnly yes` on the alias block neutralizes this (Pitfall: too-many-keys). Run `ssh-add` after generate. |
| OS-registered state | None for Phase 2 (no Task Scheduler / launchd) | None |
| Secrets/env vars | Passphrase (D-07) optionally stored in macOS Keychain via `UseKeychain yes` + `ssh-add --apple-use-keychain` | None — Keychain handles it; no env var |
| Build artifacts | None — Phase 1 packages are stubs to be filled, no stale artifacts | None |

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `ssh` | tester (phases 1+2), `ssh -Q key` probe | ✓ | OpenSSH_9.7p1 (LibreSSL 3.3.6) | none — hard requirement |
| `ssh-keygen` | (NOT used for probe — see Pitfall 1); only if shelling out for keygen | ✓ | 9.7p1 | x/crypto generates keys in-process (no ssh-keygen needed) |
| `git` | gitconfig writes (`git config --file`) | ✓ (assumed; repo is a git repo) | — | none — hard requirement |
| `ssh-add` | D-08 agent/Keychain load | likely ✓ | — | warn + continue (load is convenience, not correctness) |
| clipboard tool (`pbcopy`) | CLIP-01 | ✓ (macOS) | — | atotto returns error → print key, instruct manual copy (CLIP-02) |
| Go | build | ✓ | go1.26.0 darwin/amd64 | none |

**Missing dependencies with no fallback:** none on this dev machine. (Linux CI must also have `ssh`+`git`.)
**Note:** Probe behavior was tested on macOS OpenSSH 9.7p1; `ssh -Q key` is portable to Linux OpenSSH.

## Validation Architecture

> nyquist_validation is enabled (not disabled in config). The planner derives VALIDATION.md from this.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (+ `-race`) |
| Config file | none — `go test` convention; Phase-1 `_stub_test.go` files exist per package |
| Quick run command | `go test ./internal/<pkg>/...` |
| Full suite command | `make test` (`go test -race -coverprofile=coverage.out ./...`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| KEY-02 / SAFE-03 | backup + temp→rename→chmod; modes 0600/0644; restore on error | unit | `go test ./internal/filewriter/...` | ❌ Wave 0 |
| SAFE-01 | timestamped backup created before write | unit | `go test ./internal/filewriter/... -run Backup` | ❌ Wave 0 |
| SAFE-02 | idempotent block rewrite; second write = identical bytes; foreign content preserved | unit | `go test ./internal/sshconfig/... -run Idempotent` | ❌ Wave 0 |
| IDENT-01/KEY-01(gen) | ed25519 gen → valid OpenSSH PEM + authorized line | unit | `go test ./internal/keygen/...` | ❌ Wave 0 |
| SIGN-01 | allowed_signers line `<email> namespaces="git" ssh-ed25519 …`, email byte-match, AND line persisted to file in idempotent per-identity block | unit | `go test ./internal/keygen/... -run 'Signers|AllowedSigners'` | ❌ Wave 0 |
| SIGN-02 | user.signingkey is a path, never inline | unit | `go test ./internal/gitconfig/... -run SigningKey` | ❌ Wave 0 |
| SSH-01/02 | rendered Host block has Hostname/Port/User git/IdentityFile/IdentitiesOnly yes | unit | `go test ./internal/sshconfig/... -run Render` | ❌ Wave 0 |
| SSH-03 | macOS Host* block: IgnoreUnknown→UseKeychain→AddKeysToAgent, ordered last | unit | `go test ./internal/sshconfig/... -run Global` | ❌ Wave 0 |
| GIT-01/02 | includeIf block (gitdir trailing slash + hasconfig) renders, points to fragment | unit | `go test ./internal/gitconfig/... -run Include` | ❌ Wave 0 |
| GIT-03 | fragment sets user.name/email, gpg.format=ssh, signingkey, commit.gpgsign | unit | `go test ./internal/gitconfig/... -run Fragment` | ❌ Wave 0 |
| D-09 | `ssh -Q key` probe parsing + fallback chain selection | unit (parse fixed output) | `go test ./internal/platform/... -run Probe` | ❌ Wave 0 |
| TEST-01/02 | output-substring classifier maps the 3 D-01 outcomes; ssh -G key parse | unit (fixture strings) | `go test ./internal/tester/...` | ❌ Wave 0 |
| TEST-03 | result carries input command string + raw output | unit | `go test ./internal/tester/... -run Echo` | ❌ Wave 0 |
| CLIP-02 | graceful no-tool failure path | unit | `go test ./internal/clipboard/...` | ❌ Wave 0 |
| SSH-03/Pitfall 4 | generated config does not error `ssh -G` on **Linux** | integration | `ssh -G testalias` exit 0 in Linux container | ❌ manual/CI |
| GIT-02/Pitfall 7 | `git config user.email` resolves inside `~/git/<id>/repo/` | integration | scripted: real `.git` dir + `git config` | ❌ manual/CI |
| TEST-02 (e2e) | full create → resolved `ssh -G` shows expected identityfile | integration | manual against a real provider (network) | manual-only |
| SIGN (e2e) | `git log --show-signature` shows Good signature on a test commit | integration | manual (requires uploaded signing key + written allowed_signers) | manual-only |

**Manual-only justification:** end-to-end auth and signing verification require a real provider
account and an uploaded key (D-02 gates the resolved test on the user uploading first). The unit
tier fully covers rendering/parsing/classification (including the allowed_signers file write);
integration covers Linux portability and gitdir resolution offline.

### Sampling Rate
- **Per task commit:** `go test ./internal/<pkg>/...` for the touched package
- **Per wave merge:** `make test` (full `-race` suite)
- **Phase gate:** `make test` + `make lint` (golangci-lint + gosec) green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/filewriter/filewriter_test.go` — backup, atomic rename, chmod, restore (SAFE-01/03, KEY-02)
- [ ] `internal/keygen/keygen_test.go` — ed25519 gen, PEM shape, authorized line (IDENT-01)
- [ ] `internal/keygen/signers_test.go` — allowed_signers line byte-match + idempotent file write (SIGN-01, SAFE-02)
- [ ] `internal/sshconfig/{renderer,parser}_test.go` — block render, idempotency, Host* ordering, round-trip (SSH-01/02/03, SAFE-02)
- [ ] `internal/gitconfig/{renderer,fragment}_test.go` — includeIf, fragment, no-`[remote]` guard (GIT-01/02/03, SIGN-02)
- [ ] `internal/platform/platform_test.go` — `ssh -Q key` parse + fallback selection (D-09); per-OS hint (D-14)
- [ ] `internal/tester/tester_test.go` — output-substring classifier on fixtures, ssh -G parse (TEST-01/02/03)
- [ ] `internal/clipboard/clipboard_test.go` — graceful failure (CLIP-02)
- [ ] `internal/identity/identity_test.go` — Create orchestration with injected fakes (Architecture Pattern 3)
- [ ] Linux integration check for `ssh -G` non-error (Pitfall 4) — CI or Docker
- [ ] `gitdir:` resolution integration check with real `.git` (Pitfall 7)
- [ ] Framework: none to install — Go stdlib `testing` is in place; stub tests already green

## Security Domain

> security_enforcement assumed enabled (not set false in config). This is a tool that mutates
> `~/.ssh` and generates private keys — security is central.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | UI-free tested core; filewriter chokepoint for all mutations |
| V6 Cryptography | yes | `crypto/ed25519` + `x/crypto/ssh` — never hand-roll key serialization |
| V8 Data Protection / file perms | yes | Explicit `os.Chmod`: key 0600, `.pub` 0644, config 0600, `~/.ssh` 0700; backups 0600 |
| V12 Files & Resources | yes | Atomic temp→rename; unique temp names; no in-place truncate; gosec G304/G306 |
| V5 Input Validation | yes | Validate identity/email/alias inputs before render; reject `[remote]` in fragments |
| OS Command Injection | yes | `os/exec` arg-slice form only (no shell); gosec G204 |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Private key written world-readable | Information Disclosure | `os.Chmod(0600)` after write; doctor (Phase 4) audits |
| Backup file leaks identity/key paths | Information Disclosure | Apply 0600 to backups (PITFALLS security table) |
| Command injection via identity name in `ssh`/`git` args | Tampering/Elevation | arg-slice `exec.Command` (no shell); validate name charset |
| Partial-write corrupts SSH config (lockout) | Denial of Service | atomic temp→rename + timestamped backup recovery path |
| Cross-protocol signing-key reuse | Spoofing | `namespaces="git"` mandatory in allowed_signers (SIGN-01) |
| Wrong key offered to provider (too-many-keys) | Spoofing | `IdentitiesOnly yes` + `IdentityFile` on every alias block |
| Key paths logged in user-facing output | Information Disclosure | log alias/identifier, not full filesystem paths |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| RSA keys (legacy script, reference gists) | ed25519 (one key, auth+signing) | PRD supersession | Smaller, faster, modern default |
| GPG commit signing | SSH-key signing (`gpg.format=ssh` + allowed_signers) | git 2.34+ | One key for auth+signing; no GPG dependency |
| `ssh-keygen -Q key` (assumed in D-09) | `ssh -Q key` | n/a — D-09 was simply wrong | Probe must use `ssh`, not `ssh-keygen` |
| Blind append to config | Sentinel managed-block idempotent rewrite | this project | No duplicates; foreign content preserved |

**Deprecated/outdated:** `github.com/charmbracelet/*` v1 import paths (use `charm.land/*/v2`) —
not relevant to Phase 2 (TUI is Phase 5).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | KEY-01 (rotation) IS in the Phase-2 slice as a thin fast-follow (Open Question 1 RESOLVED — it is in the Phase-2 ID list) | phase_requirements | Resolved: planned in 02-07 reusing the four writers |
| A2 | `git` is installed on the target (not directly probed this session, but repo is a git repo) | Environment | gitconfig writes fail; but git is table-stakes for this tool |
| A3 | `ssh -Q key` output format is stable across OpenSSH versions on Linux (verified on macOS 9.7p1) | Pitfall 1 | Probe parse could miss types on an exotic build; low risk — format is long-stable |
| A4 | GitHub/GitLab upload URLs below are current | Open Questions (RESOLVED) | Stale URL in printed instructions; low impact, easily corrected |
| A5 | `allowed_signers` file at 0644 is acceptable (it is not secret) | Pitfall 6 | If a stricter posture is wanted, 0600 also works; cosmetic |

## Open Questions (RESOLVED)

1. **KEY-01 (key rotation) scope in Phase 2 — RESOLVED: keep rotation in Phase 2.**
   - KEY-01 is an explicitly mapped Phase-2 requirement (REQUIREMENTS.md traceability → Phase 2;
     it appears in the phase requirement ID list and in CONTEXT.md's in-scope list). The thin
     rotation slice in 02-07 is therefore correct and required for requirement coverage.
   - This extends D-10/D-11's three *create* modes (new / reuse / alias) with the separately-mapped
     KEY-01 rotation requirement — noted explicitly so there is no ambiguity: rotation is a distinct
     requirement, not merely "covered by mechanism." 02-07 implements `Rotate` reusing keygen + the
     four writers (re-point all four artifacts, including allowed_signers) and re-runs the two-phase
     test. SAFE-01/02/03 (backup + idempotent rewrite + confirmation) apply.

2. **GitHub/GitLab upload step exact copy (UP-01/UP-02) — RESOLVED.**
   - GitHub requires **TWO separate registrations**: the same ed25519 `.pub` must be added once as
     an **Authentication key** and once as a **Signing key**, both at `https://github.com/settings/ssh/new`
     using the "Key type" selector (verified). GitLab allows **one key marked for both** via the
     "Usage type" (Authentication / Signing / both) at `https://gitlab.com/-/user_settings/ssh_keys`.
   - The planner encodes this fact in the 02-06 upload-steps task (`cmd/gitid/upload.go`): GitHub two
     registrations (same `.pub` twice), GitLab one key with usage-type both.

3. **Reuse-existing-key `.pub` derivation (IDENT-02) — RESOLVED.**
   - On reuse, if `<key>.pub` is missing, derive it from the private key via `ssh.ParsePrivateKey`
     → public key → `ssh.MarshalAuthorizedKey`, and write it 0644 through filewriter. Implemented as
     `keygen.DerivePublicKey` in 02-07.

## Sources

### Primary (HIGH confidence)
- pkg.go.dev/golang.org/x/crypto/ssh — MarshalPrivateKey/WithPassphrase, MarshalAuthorizedKey, NewPublicKey, ParsePrivateKey signatures + v0.53.0 (fetched this session)
- proxy.golang.org @latest — x/crypto v0.53.0, ssh_config v1.6.0, cobra v1.10.2, atotto/clipboard v0.1.4 (fetched this session)
- Empirical: `ssh -Q key`, `ssh -G`, `ssh -T` exit-code behavior, ed25519 Marshal value/pointer (run this session on OpenSSH 9.7p1 / Go 1.26)
- `.planning/research/{STACK,ARCHITECTURE,PITFALLS}.md` — prior verified project research (HIGH)
- CLAUDE.md — git-config-via-exec strategy, sentinel format, safe-write rules, ed25519-only

### Secondary (MEDIUM confidence)
- docs.github.com/.../adding-a-new-ssh-key — auth vs signing separate registration (verified via WebSearch)
- golang/go#51974 — ed25519 pointer/value parse asymmetry (parse-side only; marshal verified fine)

### Tertiary (LOW confidence)
- GitLab SSH key usage-type UI exact wording (URL known; copy left to planner)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all 4 versions + x/crypto signatures re-verified this session
- Architecture: HIGH — pattern set carried from verified ARCHITECTURE.md, fits Phase-1 seams
- Pitfalls: HIGH — critical ones (probe command, exit-code, ssh -G keys, marshal type) verified empirically
- Validation: HIGH — Go stdlib testing in place; manual-only items justified by network/upload dependency

**Research date:** 2026-06-09
**Valid until:** 2026-07-09 (stable Go ecosystem; re-confirm GitHub/GitLab upload UI if copy drifts)
