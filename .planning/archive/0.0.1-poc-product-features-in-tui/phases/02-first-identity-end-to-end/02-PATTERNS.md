# Phase 2: First Identity End-to-End - Pattern Map

**Mapped:** 2026-06-09
**Files analyzed:** 23 (8 internal packages + cmd/gitid + per-file splits)
**Analogs found:** 23 / 23 (all map to a Phase-1 stub `doc.go`/`*_stub_test.go` or the established Go stdlib `testing` + `cmd/gitid/main.go` conventions)

> **Important context for the planner.** This is a *greenfield Go module* in which Phase 1
> scaffolded one stub package per seam (`doc.go` documenting the contract + a passing
> `*_stub_test.go`). There are **no real implementation analogs yet** — the "closest analog"
> for every Phase-2 file is the project's own established conventions: the package contract
> in its `doc.go`, the stub-test shape, the `cmd/gitid/main.go`/`main_test.go` style, and the
> safe-write / git-config-via-exec / output-substring rules locked in CLAUDE.md + RESEARCH.md.
> Pattern assignments below are therefore **convention-copy targets**, not "copy this CRUD
> handler" targets. Where a concrete code excerpt exists in the codebase, it is quoted with
> file path + line numbers; where the only authority is RESEARCH.md's verified code example,
> that is cited explicitly so the planner does not invent a competing pattern.

## Project-Wide Conventions (apply to every file)

These are extracted from existing code and CLAUDE.md; every new file must follow them.

**Package doc-comment convention** (from every `internal/*/doc.go`):
- Each package has a `doc.go` whose comment starts `// Package <name> ...` and states the
  contract in prose. Phase 2 replaces the trailing `// Implementation lands in a later phase
  (Phase 2+).` line with real code files, but the package-level prose contract stays the
  authority for what each package may do.

**Stub-test convention** (`internal/filewriter/filewriter_stub_test.go:1-11`):
```go
package filewriter

import "testing"

// TestStub confirms the filewriter package compiles and the test harness
// is green. No real logic exists here yet; implementation is in a later phase.
func TestStub(t *testing.T) {
	if false {
		t.Fatal("unreachable — stub always passes")
	}
}
```
Phase 2 **replaces** each `*_stub_test.go` with a real `<name>_test.go` (or split files per
the structure below). Convention to copy: stdlib `testing` only, table-style `t.Run`,
doc comment on every exported test explaining intent, English-only.

**TDD ordering (CLAUDE.md "hypothesis → test → implement"):** the test file is written and
made to fail *before* the implementation file. Per RESEARCH.md §Validation, every Wave-0 file
listed below is a `❌ Wave 0` test gap — write the test first.

**Entrypoint convention** (`cmd/gitid/main.go:1-15`, `cmd/gitid/main_test.go:1-22`):
- `// Command gitid ...` package-doc header on `main`.
- Thin `main()` → `run()` indirection; no business logic in `main`.
- `run()`-style functions are unit-tested with a `recover()` panic-guard test
  (`main_test.go:14-22`) — copy this exact panic-guard shape for the new Cobra `run*` handlers.

**English-only:** all code, comments, identifiers, UI/log/error strings, commit messages.

**gosec-clean `os/exec`:** arg-slice form only, never a shell string, never `cat <<EOF`.
`exec.Command("git", "config", "--file", path, key, val)` — args as separate slice elements
(RESEARCH.md §Pattern 3, CLAUDE.md §gitconfig strategy). This keeps gosec G204 clean. The
`make lint` target (golangci-lint + gosec via `.golangci.yml`) hard-fails otherwise.

**Module path:** `github.com/castocolina/gitid` (from `go.mod`). Internal imports are
`github.com/castocolina/gitid/internal/<pkg>`.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/filewriter/filewriter.go` | safe file writer | file-I/O (write) | `internal/filewriter/doc.go` contract + RESEARCH Pattern 1 | contract + verified-example |
| `internal/filewriter/filewriter_test.go` *(replaces stub)* | test | file-I/O | `filewriter_stub_test.go` + stdlib `testing` | convention-exact |
| `internal/platform/platform.go` | platform probe | request-response (os/exec read) | `internal/platform/doc.go` + RESEARCH `ssh -Q key` example | contract + verified-example |
| `internal/platform/platform_test.go` *(replaces stub)* | test | transform (parse fixed output) | `platform_stub_test.go` | convention-exact |
| `internal/deps/deps.go` | platform probe | request-response (tool detection) | `internal/deps/doc.go` | contract |
| `internal/deps/deps_test.go` *(replaces stub)* | test | request-response | `deps_stub_test.go` | convention-exact |
| `internal/keygen/keygen.go` | key generation | transform + file-I/O | `internal/keygen/doc.go` + RESEARCH ed25519 example | contract + verified-example |
| `internal/keygen/keygen_test.go` *(replaces stub)* | test | transform | `keygen_stub_test.go` | convention-exact |
| `internal/sshconfig/renderer.go` | SSH config renderer | transform | `internal/sshconfig/doc.go` + ARCHITECTURE Pattern 1/2 | contract |
| `internal/sshconfig/parser.go` | SSH config parser | transform (round-trip) | `internal/sshconfig/doc.go` (kevinburke/ssh_config) | contract |
| `internal/sshconfig/writer.go` | SSH config writer | file-I/O (compose) | doc.go + delegates to filewriter | contract |
| `internal/sshconfig/*_test.go` *(replaces stub)* | test | transform | `sshconfig_stub_test.go` | convention-exact |
| `internal/gitconfig/renderer.go` | gitconfig writer | transform | `internal/gitconfig/doc.go` + RESEARCH Pattern 3 | contract + verified-example |
| `internal/gitconfig/fragment.go` | gitconfig writer | file-I/O (os/exec) | `internal/gitconfig/doc.go` (`git config --file`) | contract + verified-example |
| `internal/gitconfig/*_test.go` *(replaces stub)* | test | transform | `gitconfig_stub_test.go` | convention-exact |
| `internal/tester/tester.go` | test runner | request-response (os/exec read) | `internal/tester/doc.go` + RESEARCH classifier example | contract + verified-example |
| `internal/tester/tester_test.go` *(replaces stub)* | test | transform (fixture strings) | `tester_stub_test.go` | convention-exact |
| `internal/clipboard/clipboard.go` | clipboard | request-response (os/exec) | `internal/clipboard/doc.go` + atotto example | contract + verified-example |
| `internal/clipboard/clipboard_test.go` *(replaces stub)* | test | request-response | `clipboard_stub_test.go` | convention-exact |
| `internal/identity/identity.go` | identity orchestrator | event-driven (orchestration) | `internal/identity/doc.go` + ARCHITECTURE Pattern 3 | contract |
| `internal/identity/identity_test.go` *(replaces stub)* | test | event-driven (fakes) | `identity_stub_test.go` | convention-exact |
| `cmd/gitid/add.go` (or `identity.go`) | Cobra command | request-response (interactive) | `cmd/gitid/main.go` thin-handler style | role-match |
| `cmd/gitid/main.go` *(modified)* | Cobra root | request-response | itself (`main.go:8-15`) | self |

## Pattern Assignments

### `internal/filewriter/filewriter.go` (safe file writer, file-I/O) — BUILD FIRST

**Analog (contract):** `internal/filewriter/doc.go:1-10` — package may do only:
"timestamped backup, render-to-temp, atomic rename via os.Rename, correct file-permission
setting, and optional restore on error." It backs sshconfig writes and gitconfig raw managed
blocks/fragments; plain key/value gitconfig mutations go through `git config`, NOT here.

**Authoritative implementation pattern (RESEARCH.md Pattern 1, lines 196-209):**
```go
func Write(targetPath string, content []byte, mode os.FileMode) (backupPath string, err error) {
    // 1. backup (only if target exists): copy → targetPath + ".bak." + time.Now().Format("20060102-150405")
    //    apply SAME restrictive mode to the backup (backup is world-readable risk)
    // 2. tmp, _ := os.CreateTemp(filepath.Dir(targetPath), "gitid-*.tmp")  // unique name, NOT a fixed .tmp
    // 3. tmp.Write(content); tmp.Sync(); tmp.Close()
    // 4. os.Chmod(tmp.Name(), mode)        // 0600 config/key, 0644 .pub
    // 5. os.Rename(tmp.Name(), targetPath) // atomic on same filesystem
    // 6. ensure parent dir mode (~/.ssh 0700) via os.MkdirAll + os.Chmod
}
```

**Hard rules to copy (RESEARCH Pitfall 6, lines 309-318; CLAUDE.md safe-write):**
- Never `os.WriteFile` in place; never a fixed `.tmp` suffix; always explicit `os.Chmod`
  (never rely on umask); apply 0600 to backups too.
- Permission table: `~/.ssh/` 0700, `~/.ssh/config` 0600, private key 0600, `.pub` 0644,
  `allowed_signers` 0644, `~/.gitconfig`/fragment 0644, all backups 0600.
- Idempotent sentinel block replace (RESEARCH Pattern 2, lines 211-224): scan for
  `# BEGIN gitid managed: <name>` … `# END gitid managed: <name>`, replace the whole range,
  preserve foreign content byte-identical (SAFE-02 idempotency: second write = empty diff).

**Test analog:** replace `filewriter_stub_test.go` with `filewriter_test.go`; cover SAFE-01
(backup created), SAFE-03/KEY-02 (temp→rename→chmod, modes), restore-on-error, idempotent
block rewrite (RESEARCH §Wave 0 Gaps, lines 491-492).

> **Note — ARCHITECTURE.md line 521 suggests `github.com/google/renameio` (MEDIUM).**
> RESEARCH.md and CLAUDE.md (the higher-authority docs) specify the raw
> `os.CreateTemp`→`Sync`→`Chmod`→`os.Rename` recipe and add NO new dependency beyond the four
> pinned libs. **Planner: prefer the stdlib recipe; do not add renameio unless explicitly
> re-decided.**

---

### `internal/platform/platform.go` (platform probe, request-response) — D-09 / D-14

**Analog (contract):** `internal/platform/doc.go:1-7` — OS detection (darwin/linux),
UseKeychain guard, clipboard command selection, permission-fix hints; **no third-party deps.**

**Authoritative probe pattern — CORRECTED (RESEARCH Pitfall 1 + example, lines 277-283, 371-378):**
```go
out, err := exec.Command("ssh", "-Q", "key").Output()   // NOT `ssh-keygen -Q key` (that is KRL mode)
supported := strings.Split(strings.TrimSpace(string(out)), "\n")
// has "ssh-ed25519" → ed25519 path (default).
// else walk fallback chain ed25519 → rsa(4096) → ecdsa against `supported`; none → D-14 stop.
```
**Critical:** the CONTEXT.md D-09 command `ssh-keygen -Q key` is **wrong** (returns
`KRL checking requires an input file`). Use `ssh -Q key`. Probe is membership-test only.

**D-14 install hint:** if none of ed25519/rsa/ecdsa present, STOP with per-OS guidance
(macOS `brew install openssh`; Linux `apt`/`dnf`/`pacman`) — this is the mini-DOC-01 seam
the Phase-4 doctor will generalize. Build it here, not in `cmd/`.

**Test analog:** replace stub; parse a *fixed* `ssh -Q key` output fixture (no live shell-out
in unit test) and assert fallback selection (RESEARCH line 471, 495).

---

### `internal/deps/deps.go` (platform probe, request-response)

**Analog (contract):** `internal/deps/doc.go:1-7` — required: `ssh`, `ssh-keygen`, `git`;
optional: `ssh-add`, `pbcopy`, `xclip`, `xsel`, `wl-copy`; returns a structured availability
report used by doctor and platform. Use `exec.LookPath` for presence; arg-slice exec for any
probe. No business logic beyond availability.

---

### `internal/keygen/keygen.go` (key generation, transform + file-I/O) — IDENT-01, SIGN-01

**Analog (contract):** `internal/keygen/doc.go:1-7` — ed25519 pair, private key 0600, `.pub`
0644, thin wrapper over `crypto/ed25519` + `golang.org/x/crypto/ssh`.

**Authoritative generation pattern (RESEARCH §Code Examples, lines 346-361, verified this session):**
```go
pub, priv, err := ed25519.GenerateKey(rand.Reader)          // crypto/ed25519
block, err := ssh.MarshalPrivateKey(priv, comment)          // value type works; comment "<identity>@gitid"
// passphrase set (D-07 optional): ssh.MarshalPrivateKeyWithPassphrase(priv, comment, []byte(pass))
privPEM := pem.EncodeToMemory(block)                        // "-----BEGIN OPENSSH PRIVATE KEY-----"
sshPub, err := ssh.NewPublicKey(pub)
pubLine := ssh.MarshalAuthorizedKey(sshPub)                 // "ssh-ed25519 AAAA...\n" (TRAILING newline)
// delegate both writes to filewriter.Write(path, bytes, 0600 / 0644)
```
**allowed_signers line (RESEARCH lines 363-369, SIGN-01, Pitfall 8 lines 325-330):**
```go
keyText := strings.TrimRight(string(pubLine), "\n")          // MarshalAuthorizedKey appends \n — strip it
signersLine := fmt.Sprintf("%s namespaces=\"git\" %s\n", userEmail, keyText)
// email MUST be byte-identical to the fragment's user.email; namespaces="git" mandatory.
```
**Rules:** key filename `~/.ssh/id_<algo>_<identity>` (D-06); never hand-roll OpenSSH
serialization (RESEARCH §Don't Hand-Roll, lines 261-262); pass the value from `GenerateKey`
directly (Pitfall 10, value works for marshal at v0.53.0). All file writes go through
`filewriter`, not `os.WriteFile`.

**Test analog:** replace stub; assert PEM header shape, authorized-line prefix, signers-line
format + email byte-match (RESEARCH lines 464-465).

---

### `internal/sshconfig/renderer.go` + `parser.go` + `writer.go` (SSH renderer/parser, transform) — SSH-01/02/03, SAFE-02

**Analog (contract):** `internal/sshconfig/doc.go:1-7` — parse managed blocks, render Account
→ Host stanza, wrap `github.com/kevinburke/ssh_config` for round-trips, **delegate writes to
filewriter.**

**Render rules (RESEARCH Pitfalls 3-5, lines 294-307; ARCHITECTURE Pattern 1, lines 196-214):**
- Host alias block emits: `Hostname`, `Port`, `User git`, `IdentityFile`, `IdentitiesOnly yes`
  (SSH-01). `ssh -G` resolves these as lowercase keys — keep that in mind for the tester.
- macOS `Host *` block ordered **LAST** (first-match-wins, Pitfall 5):
  `IgnoreUnknown UseKeychain` → `UseKeychain yes` → `AddKeysToAgent yes` (Pitfall 4 — the
  `IgnoreUnknown` line MUST precede the Apple-only directive so Linux `ssh -G` does not error).
- platform-guarded via `internal/platform` (ARCHITECTURE Anti-Pattern 5, lines 489-495).
- Round-trip stability: parse → render → parse must be stable (CONTEXT D-12/D-13; validate
  with a second decode pass).

**Split convention (ARCHITECTURE recommended structure, lines 101-106):** `parser.go`,
`renderer.go`, `writer.go` (compose), each with a `_test.go`. Replace `sshconfig_stub_test.go`
with these.

---

### `internal/gitconfig/renderer.go` + `fragment.go` (gitconfig writer, transform + file-I/O) — GIT-01/02/03, SIGN-02

**Analog (contract):** `internal/gitconfig/doc.go:1-10` — reads/writes plain key/value via
`git config` (os/exec, git is authoritative); writes `includeIf`/`url` headers `git config`
**cannot** create as sentinel managed blocks; fragment files in `~/.gitconfig.d/`; raw block +
fragment writes delegated to filewriter.

**Authoritative pattern (RESEARCH Pattern 3, lines 226-245 — arg-slice, gosec-clean):**
```go
// fragment key/values (idempotent, comment-safe — git owns the format):
exec.Command("git", "config", "--file", fragPath, "user.name", name)
exec.Command("git", "config", "--file", fragPath, "user.email", email)
exec.Command("git", "config", "--file", fragPath, "gpg.format", "ssh")
exec.Command("git", "config", "--file", fragPath, "user.signingkey", pubKeyPath) // PATH, not inline (SIGN-02)
exec.Command("git", "config", "--file", fragPath, "commit.gpgsign", "true")
// global: gpg.ssh.allowedSignersFile via git config --file gitconfigPath ...
// includeIf header → raw managed block appended/replaced in ~/.gitconfig (filewriter):
//   # BEGIN gitid managed: <name>
//   [includeIf "gitdir:~/git/<name>/"]    <-- trailing slash REQUIRED (Pitfall 7, GIT-02)
//   	path = ~/.gitconfig.d/<name>
//   # END gitid managed: <name>
```
**Rules:** never shell expansion (arg slice only, gosec G204); `gitdir:` trailing slash
mandatory (Pitfall 7, lines 320-323); `hasconfig:remote.*.url:` also renderable (GIT-02);
**reject `[remote]` in fragments** at render time (Pitfall 9, lines 332-335 — hard git error);
`user.signingkey` is the `.pub` path, never inline (SIGN-02, RESEARCH line 70).

---

### `internal/tester/tester.go` (test runner, request-response) — TEST-01/02/03, D-01/D-03

**Analog (contract):** `internal/tester/doc.go:1-8` — two phases (`ssh -i <key> -T <host>`,
then `ssh -T <alias>` + `ssh -G <alias>`); structured result with pass/fail + raw output;
**read-only, no side effects.**

**Authoritative classifier — by OUTPUT SUBSTRING, never exit code (RESEARCH lines 380-409, Pitfall 2):**
```go
cmd := exec.Command("ssh", "-i", keyPath, "-o", "IdentitiesOnly=yes",
    "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-T", "git@"+host)
out, _ := cmd.CombinedOutput()              // IGNORE exit code: `ssh -T` exits 0 even on denial (verified)
s := string(out)
switch {
case strings.Contains(s, "successfully authenticated"): return PASS                 // reused/uploaded key
case strings.Contains(s, "Permission denied (publickey)"): return REACHABLE_NOT_UPLOADED // new key → proceed (D-02)
default: return FAILURE                                                              // refused/DNS/timeout → abort
}
// TEST-03: result carries cmd.String() (input) + s (output); both printed.
```
**Resolved parse (`ssh -G`, lowercase keys, Pitfall 3, lines 398-410):** match
`^identityfile `, `^identitiesonly ` (assert `yes`), `^user ` (assert `git`), `^hostname `,
`^port `; `identityfile` may repeat — assert the expected path is among them (D-03).

**Test analog:** replace stub; feed fixture strings to the classifier and the `ssh -G` parser
(RESEARCH lines 472-473) — no live network in unit tests.

---

### `internal/clipboard/clipboard.go` (clipboard, request-response) — CLIP-01/02

**Analog (contract):** `internal/clipboard/doc.go:1-7` — copy `.pub` text; atotto dispatch.

**Authoritative pattern (RESEARCH lines 412-418):**
```go
if err := clipboard.WriteAll(string(pubLine)); err != nil {
    // CLIP-02 graceful failure: warn "no clipboard tool found; copy manually:" + print pubLine
}
```
Do not hand-roll per-OS exec (RESEARCH §Don't Hand-Roll, line 266).

> **Note:** `internal/clipboard/doc.go:6` currently says "Phase 5+". Phase 2 needs CLIP-01/02
> (RESEARCH lines 77-78, 474). Planner: pull the clipboard implementation forward into Phase 2
> and update the doc.go phase marker.

---

### `internal/identity/identity.go` (identity orchestrator, event-driven)

**Analog (contract):** `internal/identity/doc.go:1-8` — `Account` type + CRUD; reconstructs
from managed blocks; filesystem is source of truth.

**Orchestration pattern — dependencies injected as params (ARCHITECTURE Pattern 3, lines 251-285):**
```go
result, err := identity.Create(accts, input, keygen.Generate, sshconfig.Write, gitconfig.Write)
```
`identity.Create` receives keygen/write functions as parameters so it is testable with fakes
(RESEARCH line 498) and the future TUI reuses the same path. **No business logic in `cmd/`**
(ARCHITECTURE Anti-Pattern 1, lines 457-463). Phase 2 needs only the create-new orchestration
(D-11); reuse/alias are fast-follow plans.

**Test analog:** replace stub; drive `Create` with injected fakes for keygen + writers.

---

### `cmd/gitid/add.go` + modified `cmd/gitid/main.go` (Cobra command, request-response) — D-04/D-05

**Analog (style):** `cmd/gitid/main.go:1-15` (thin `main()`→`run()`, package-doc header) and
`cmd/gitid/main_test.go:14-22` (panic-guard test pattern — copy verbatim for new handlers):
```go
func TestRunDoesNotPanic(t *testing.T) {
    defer func() {
        if r := recover(); r != nil { t.Fatalf("run() panicked: %v", r) }
    }()
    run()
}
```
**Rules:** real, minimal `gitid identity add` Cobra command (D-04 — foundation for Phase 5,
not throwaway). Interactive prompts with sensible defaults (D-05). Handler ≤30 lines: gather
input → call `internal/` functions → print (ARCHITECTURE Anti-Pattern 1). Orchestration
sequence (RESEARCH lines 134-174): probe → keygen → clipboard → pre-write test → render →
unified preview + single confirm (`--dry-run` skips write, SAFE-03) → 4× filewriter writes →
`ssh-add` (macOS `--apple-use-keychain`, D-08) → resolved test → upload steps (UP-01/02:
**two separate GitHub registrations** auth + signing; GitLab one key both, RESEARCH lines 558-563).
Cobra (`github.com/spf13/cobra` v1.10.2) is the only new CLI dep.

## Shared Patterns

### Safe Write (the chokepoint)
**Source of truth:** `internal/filewriter/filewriter.go` (built first) + RESEARCH Pattern 1.
**Apply to:** every mutation of `~/.ssh/config`, `~/.gitconfig`, `~/.gitconfig.d/<name>`,
`~/.ssh/allowed_signers`, the private key, and the `.pub`. `keygen`, `sshconfig/writer`,
`gitconfig` all delegate here. Never call `os.WriteFile` directly in any other package.

### Idempotent Sentinel Managed Block
**Source of truth:** RESEARCH Pattern 2 + CLAUDE.md. Marker format:
`# BEGIN gitid managed: <identity-name>` … `# END gitid managed: <identity-name>`
(macOS global block keyed `_global`, ordered LAST).
**Apply to:** SSH Host blocks, gitconfig `includeIf` blocks. Idempotency proof: a second
identical write produces an empty `diff` — make this a verification step.

### os/exec arg-slice (gosec-clean)
**Source of truth:** RESEARCH Pattern 3 + CLAUDE.md §gitconfig strategy + Security Domain
(lines 503-527).
**Apply to:** `platform` (`ssh -Q key`), `deps` (`exec.LookPath`), `gitconfig`
(`git config --file`), `tester` (`ssh -i`/`-T`/`-G`), `clipboard`, `cmd` (`ssh-add`).
Args as separate slice elements; never a shell string; never heredoc. Validate
identity/email/alias charset before interpolating into args (V5 input validation).

### TDD test-first + English-only
**Source of truth:** CLAUDE.md working method + every `*_stub_test.go`.
**Apply to:** every package — write the failing `_test.go` before the implementation; stdlib
`testing` only; doc comment per exported test; all artifacts in English. `make test`
(`go test -race`) + `make lint` (golangci-lint + gosec) gate every commit.

## No Analog Found

None. Every Phase-2 file maps to an existing Phase-1 stub package contract (`doc.go`), the
`cmd/gitid` thin-handler convention, or a RESEARCH.md verified code example. There are no real
implementation analogs in the codebase (this is the first vertical slice), so the planner must
treat the cited `doc.go` contracts + RESEARCH verified examples + CLAUDE.md rules as the
copy-from authority rather than expecting an existing handler to clone.

## Metadata

**Analog search scope:** `internal/`, `cmd/`, `tui/` (all 23 `.go` files), `go.mod`,
`Makefile`, `.golangci.yml`; canonical docs CONTEXT.md, RESEARCH.md, ARCHITECTURE.md.
**Files scanned:** 23 Go files + 4 config/doc files.
**Pattern extraction date:** 2026-06-09
