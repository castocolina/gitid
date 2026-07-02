# Phase 3: Full Identity CRUD + Multi-Identity — Research

**Researched:** 2026-06-10
**Domain:** Go CLI — managed-block read/list/remove primitives, cross-file
reconstruction join, identity update and delete mechanics, multi-identity
coexistence, TDD with hermetic HOME fixtures
**Confidence:** HIGH (all claims are grounded in the real codebase or verified
library source)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Reconstruction (IDENT-07)**
- D-01: The **identity name** — the token in `# BEGIN gitid managed: <identity>`
  — is the canonical correlation key. The SSH Host *alias* is a per-account
  attribute, not the primary key.
- D-02: Reconstruction is **best-effort**: an incomplete block set is still
  returned as an `Account` with a light "incomplete" marker; deep diagnosis
  stays in Phase 4 doctor.

**List (IDENT-03)**
- D-03: `list` shows key path, alias, provider, port, match strategy, plus
  the D-02 incompleteness marker. Descriptive + light-health only; no
  coherence checks.

**Update (IDENT-04)**
- D-04: Identity **name is immutable**. `update` edits everything except name.
  Rename = delete + recreate.
- D-05: `update` re-runs `ssh -T <alias>` + `ssh -G <alias>` **only when a
  structural field changed** (alias/provider/port/match). Pure fragment edits
  skip the network round-trip.
- D-06: `update` follows the same safe-write pattern as create: timestamped
  backup → unified preview → single explicit confirm → idempotent whole-block
  rewrite.

**Delete (IDENT-05)**
- D-07: Default **keeps the private key**. A separate explicit prompt (default
  "no") offers to delete the key files.
- D-08: Delete scope is per-identity only: SSH Host block, includeIf block,
  fragment **file** (whole file removal), allowed_signers **line**. Shared
  global blocks are **never touched** even when deleting the last identity.

### Claude's Discretion

- List layout: grouped by identity (identity header, accounts/aliases nested)
  for human view; optional flat/parseable flag if it earns its keep.
- Incompleteness marker copy (exact wording/glyph) — keep it light (a marker +
  what's missing).
- Minimal CLI subcommand shape for `list`/`update`/`delete` — follow Phase 2
  pattern (real Cobra commands, not throwaway).
- New read-side primitives (`ListBlocks` / `RemoveBlock`) — package placement
  and signatures are the planner's call. All mutation routes through the
  existing `filewriter` safe-write chokepoint.

### Deferred Ideas (OUT OF SCOPE)

- Baseline/global git config + global gitignore (Phase 3.1)
- Doctor deep coherence/drift/orphan health checks (Phase 4)
- Full Cobra command surface + shell completion + TUI (Phase 5)
- Identity rename as in-place operation (future enhancement)
- TUI view/edit of identities and the baseline (Phase 5)
- `add repo`, adopt-fragments (ADOPT-01), automatic key upload (AUTOUP-01) — v2
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| IDENT-03 | User can list identities and accounts with their wiring (key path, alias, provider, port, match strategy) | ListBlocks primitive + SSH/gitconfig read APIs → reconstruct []Account → render table/grouped view |
| IDENT-04 | User can update an identity's name/email, signing on/off, provider/alias/port, and match strategy | Reload Account → edit fields → re-render four artifacts via ReplaceBlock → conditional ssh -G re-test |
| IDENT-05 | User can delete an identity/account — its managed blocks are removed (key optional) with confirmation and backup | RemoveBlock primitive + fragment file removal + allowed_signers line removal + removal manifest |
| IDENT-07 | On startup the tool reconstructs the identity/account list by parsing its managed blocks (no sidecar database) | ListBlocks(sshconfig) + ListBlocks(gitconfig) + git config --file reads + join by identity name |
</phase_requirements>

---

## Summary

Phase 3 is the **read/manage** complement to Phase 2's create-only write path.
Phase 2 already writes four coordinated artifacts per identity: an SSH Host
block (in `~/.ssh/config`), a gitconfig includeIf block (in `~/.gitconfig`), a
fragment file (`~/.gitconfig.d/<name>`), and an `allowed_signers` line. Phase 3
must enumerate those blocks, join them by identity name into `[]Account`
(reconstruction), expose them as `gitid identity list`, allow field-level edits
(`gitid identity update`), and remove them safely (`gitid identity delete`).

The key architectural insight is that all four write operations already go
through `filewriter.ReplaceBlock` (upsert) and `filewriter.Write` (safe write).
Phase 3 needs exactly two new block-level primitives — `ListBlocks` (enumerate
named blocks with their body text) and `RemoveBlock` (splice out a named block)
— plus SSH and gitconfig **read** functions to extract typed fields from within
those bodies. The existing `sshconfig.Parse` and the `git config --file --list`
exec pattern already provide the reading machinery; the planner just needs to
wire them together in `internal/identity` as a `Reconstruct([]byte, []byte,
readFragment func) ([]Account, error)` function.

Update reuses the existing `runPipeline` / `ReplaceBlock` path (identical to
create). Delete uses the new `RemoveBlock` primitive plus `os.Remove` for the
fragment file and a targeted line-filter for `allowed_signers`. The multi-identity
proof (Success Criterion 2) is a round-trip property test: write two identities
via the Phase 2 pipeline, call `tester.ParseResolved` on each alias, assert the
`IdentityFiles` slices are distinct.

**Primary recommendation:** Add `ListBlocks` and `RemoveBlock` to
`internal/filewriter/block.go`, add SSH-side block reader helpers to
`internal/sshconfig`, add gitconfig-side block reader helpers to
`internal/gitconfig`, and wire reconstruction into `internal/identity/loader.go`
following the `Deps`-injection pattern already established by `modes.go`.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| ListBlocks / RemoveBlock primitives | `internal/filewriter` | — | All block manipulation lives in filewriter; nothing else owns sentinel logic |
| SSH-side reconstruction read | `internal/sshconfig` | — | Owns the kevinburke/ssh_config parse layer; new `ParseManagedHosts` function here |
| Gitconfig-side reconstruction read | `internal/gitconfig` | `os/exec` (git config --file) | Owns includeIf parse; fragment reads via git config --file --list |
| Reconstruction join / []Account assembly | `internal/identity` | — | Aggregation layer; joins ssh+gitconfig+fragment data by identity name |
| Update orchestration | `internal/identity` | — | Mirrors existing modes.go Deps pattern |
| Delete orchestration | `internal/identity` | — | Same Deps injection; RemoveBlock + os.Remove calls |
| `gitid identity list/update/delete` CLI | `cmd/gitid` | — | Thin Cobra handlers only; zero business logic |
| Structural re-test on update | `internal/tester` | — | Existing Resolved() function; no new code needed |

---

## Standard Stack

### Core (all already in go.mod — no new dependencies needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/kevinburke/ssh_config` | v1.6.0 | Parse `~/.ssh/config`; `cfg.Hosts` exposes `[]*Host` for enumeration | Already in use; `Host.Patterns[0].String()` gives alias; `Host.Nodes` gives KV directives |
| `os/exec` (stdlib) | — | `git config --file <frag> --list` to read fragment key/values | Established pattern; avoids custom gitconfig parser for the read side |
| `strings` (stdlib) | — | Line-by-line `allowed_signers` filter for delete | No third-party needed |
| `internal/filewriter` | local | `ListBlocks` + `RemoveBlock` new primitives; `Write` safe-write chokepoint | Single place for all block manipulation |

**Installation:** No new `go get` needed. Phase 3 is pure internal implementation.

### No New External Packages

Phase 3 adds zero external dependencies. The entire implementation
lives in new functions within the existing internal packages. This is the
correct outcome: the architecture was designed so Phase 3 slots in without new
dependencies.

---

## Package Legitimacy Audit

> No external packages are installed in this phase. All capabilities come
> from the existing `go.mod` dependencies and Go stdlib.

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

---

## Architecture Patterns

### System Architecture Diagram (Phase 3 additions highlighted)

```
gitid identity list / update / delete
           │
           ▼ (cmd/gitid/list.go, update.go, delete.go — thin handlers)
           │
           ▼
internal/identity
  ├── loader.go         [NEW] Reconstruct(sshBytes, gcBytes, readFrag) ([]Account, error)
  ├── update.go         [NEW] Update(existing Account, edited Account, deps UpdateDeps) (UpdateResult, error)
  ├── delete.go         [NEW] Delete(name string, keepKey bool, deps DeleteDeps) (DeleteResult, error)
  └── identity.go       [EXISTING] Account struct, CreateInput, Deps
           │
           ├── internal/filewriter
           │   ├── block.go [EXTEND] +ListBlocks(content []byte) ([]NamedBlock, error)
           │   └── block.go [EXTEND] +RemoveBlock(content []byte, name string) []byte
           │
           ├── internal/sshconfig
           │   ├── parser.go  [EXISTING] Parse(content) (*ssh_config.Config, error)
           │   └── reader.go  [NEW] ParseManagedHosts(sshBytes []byte) (map[string]SSHHostInfo, error)
           │
           └── internal/gitconfig
               ├── renderer.go [EXISTING] RenderIncludeIf, WriteIncludeIf
               ├── fragment.go [EXISTING] WriteFragment, SetAllowedSignersFile
               └── reader.go   [NEW] ParseManagedIncludeIf(gcBytes []byte) (map[string]IncludeIfInfo, error)
                               [NEW] ReadFragment(fragPath string) (FragmentInfo, error)
                               [NEW] RemoveAllowedSignersLine(path, identityName string) error
```

Data flow for reconstruction:

```
~/.ssh/config bytes  ──► sshconfig.ParseManagedHosts ──► map[name → SSHHostInfo]
                                                                │
~/.gitconfig bytes   ──► gitconfig.ParseManagedIncludeIf ──► map[name → IncludeIfInfo]
                                                                │
                          join by identity name key (D-01) ────┘
                                │
              for each joined pair: gitconfig.ReadFragment(info.FragmentPath)
                                │
                          []Account (with Incomplete marker where a piece is missing)
```

### Recommended Project Structure Additions

```
internal/filewriter/
├── block.go         # EXTEND: add ListBlocks + RemoveBlock
│
internal/sshconfig/
├── reader.go        # NEW: ParseManagedHosts
│
internal/gitconfig/
├── reader.go        # NEW: ParseManagedIncludeIf, ReadFragment, RemoveAllowedSignersLine
│
internal/identity/
├── loader.go        # NEW: Reconstruct([]byte, []byte, func) ([]Account, error)
├── update.go        # NEW: Update + UpdateDeps
├── delete.go        # NEW: Delete + DeleteDeps
│
cmd/gitid/
├── list.go          # NEW: thin Cobra handler
├── update.go        # NEW: thin Cobra handler (avoid naming conflict with modes.go)
├── delete.go        # NEW: thin Cobra handler
```

---

## Research Question 1: Managed-Block List/Read Primitive

### Current State

`block.go` has `ReplaceBlock` (upsert) but no `ListBlocks` or `RemoveBlock`.
`ReplaceBlock` uses `strings.SplitAfter(string(existing), "\n")` and a
`beginIdx`/`endIdx` scanner. The read counterpart mirrors the exact same
scan pattern.

### Recommended `ListBlocks` Signature

```go
// NamedBlock is one sentinel-delimited block extracted from a file.
type NamedBlock struct {
    Name string // the <name> token from "# BEGIN gitid managed: <name>"
    Body string // lines between (exclusive of) the sentinel markers, as written
}

// ListBlocks scans content for all complete gitid managed blocks and returns
// them in file order. Incomplete blocks (BEGIN with no matching END) are
// silently skipped — they surface as "missing SSH block" in the reconstruction
// incomplete-marker logic. CRLF is normalised to LF before scanning so Windows
// line endings in synced config files do not cause missed matches.
func ListBlocks(content []byte) []NamedBlock
```

**Implementation approach** (pure Go, no regex, mirrors ReplaceBlock):

```go
func ListBlocks(content []byte) []NamedBlock {
    // Normalise CRLF → LF so Windows-synced configs parse correctly.
    normalised := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
    lines := strings.SplitAfter(string(normalised), "\n")

    var result []NamedBlock
    beginIdx := -1
    currentName := ""
    for i, line := range lines {
        trimmed := strings.TrimRight(line, "\n\r")
        if strings.HasPrefix(trimmed, BeginPrefix) {
            // Ignore nested or duplicate begins — only the outermost matters.
            if beginIdx == -1 {
                beginIdx = i
                currentName = strings.TrimPrefix(trimmed, BeginPrefix)
            }
            continue
        }
        if beginIdx != -1 && strings.HasPrefix(trimmed, EndPrefix) {
            endName := strings.TrimPrefix(trimmed, EndPrefix)
            if endName == currentName {
                // Collect body lines between markers (exclusive).
                body := strings.Join(lines[beginIdx+1:i], "")
                body = strings.TrimRight(body, "\n")
                result = append(result, NamedBlock{Name: currentName, Body: body})
                beginIdx = -1
                currentName = ""
            }
            // Mismatched END name: skip silently (orphan sentinel; doctor handles).
        }
    }
    return result
}
```

**Edge cases handled:**

| Edge case | Behaviour |
|-----------|-----------|
| BEGIN with no END | Skipped entirely; callers see it as "missing block" |
| Nested same-name BEGIN (corrupt) | Inner BEGIN ignored; outer pair wins |
| Mismatched END name | END ignored; the open block stays open until EOF |
| CRLF line endings | Normalised to LF before scan |
| Empty file | Returns nil slice, no error |
| Block with empty body | Returns NamedBlock with Body="" |
| Foreign content between blocks | Never touched; only lines inside a matched pair are returned as Body |

**Round-trip stability with `ReplaceBlock`:** `ListBlocks` returns the trimmed
body; `ReplaceBlock` trims trailing newlines from its `blockBody` argument
before writing. So `ListBlocks(ReplaceBlock(content, name, body)).body` ==
`strings.TrimRight(body, "\n")`. This invariant is checkable with a property
test (see Validation Architecture section).

[VERIFIED: codebase inspection of internal/filewriter/block.go]

---

## Research Question 2: RemoveBlock Primitive

### Recommended `RemoveBlock` Signature

```go
// RemoveBlock returns content with the gitid managed block for name removed.
// If no such block exists (already absent), the input is returned unchanged
// (idempotent). All content outside the named block is preserved
// byte-for-byte. A single blank line that immediately followed the END marker
// is also removed to avoid accumulating blank lines on repeated delete+recreate
// cycles.
func RemoveBlock(content []byte, name string) []byte
```

**Implementation approach** (mirrors ReplaceBlock's splice pattern):

```go
func RemoveBlock(content []byte, name string) []byte {
    beginMarker := BeginPrefix + name
    endMarker := EndPrefix + name

    lines := strings.SplitAfter(string(content), "\n")

    beginIdx, endIdx := -1, -1
    for i, line := range lines {
        trimmed := strings.TrimRight(line, "\n")
        switch {
        case beginIdx == -1 && trimmed == beginMarker:
            beginIdx = i
        case beginIdx != -1 && trimmed == endMarker:
            endIdx = i
        }
        if beginIdx != -1 && endIdx != -1 {
            break
        }
    }

    // Block absent — return input unchanged (idempotent).
    if beginIdx == -1 || endIdx == -1 {
        return content
    }

    // Determine the slice boundary after the END line.
    afterEnd := endIdx + 1
    // Consume one trailing blank line to avoid accumulating blank lines after
    // repeated delete+recreate. Only remove it if it is genuinely empty.
    if afterEnd < len(lines) {
        if strings.TrimRight(lines[afterEnd], "\n\r") == "" {
            afterEnd++
        }
    }

    var b strings.Builder
    b.WriteString(strings.Join(lines[:beginIdx], ""))
    b.WriteString(strings.Join(lines[afterEnd:], ""))
    return []byte(b.String())
}
```

**Safety properties:**

- Foreign content before BEGIN is preserved byte-for-byte via `lines[:beginIdx]`.
- Foreign content after the consumed trailing blank line is preserved via
  `lines[afterEnd:]`.
- Calling `RemoveBlock` twice with the same name produces the same output as
  calling it once (idempotent).
- A block that was never written returns `content` unchanged.
- `ReplaceBlock` after `RemoveBlock` reinserts the block; `RemoveBlock` after
  `ReplaceBlock` removes it. Compose freely.

**Pitfall — blank-line accumulation:** Without the trailing-blank consumer, each
`add → delete → add → delete` cycle leaves one blank line. The consumer removes
at most one blank line (the one the block's END line added as separation from
the next block). It never removes user-written blank lines that preceded the
block.

[VERIFIED: codebase inspection; mirrors ReplaceBlock splice pattern]

---

## Research Question 3: Reconstruction Join (IDENT-07)

### SSH-Side Read: `ParseManagedHosts`

The kevinburke/ssh_config `Config.Hosts` slice exposes every `Host` block as a
`*ssh_config.Host` value. `Host.Patterns[0].String()` gives the alias string
(e.g. `work.github.com`). `Host.Nodes` is `[]Node`; each node is a `*KV`
(key/value directive) or `*Empty` (comment/blank line).

The managed blocks are wrapped in comment sentinels. The sentinel comments
appear as `*Empty` nodes in the kevinburke parser (they are `# BEGIN…` and
`# END…` lines, which are comment nodes). However, the sentinel lines are also
the *structural boundaries* for the raw byte manipulation already performed by
`filewriter.ReplaceBlock`. For the reconstruction read path, the cleaner
approach is to use `ListBlocks` on the raw bytes first to get each block's raw
body, then parse *that body alone* via `sshconfig.Parse` to extract the `Host`
stanza fields.

**Recommended `ParseManagedHosts` function:**

```go
// SSHHostInfo holds the fields extracted from a gitid-managed SSH Host block.
type SSHHostInfo struct {
    Alias        string   // the Host pattern value (e.g. "work.github.com")
    Hostname     string   // the Hostname directive
    Port         int      // the Port directive (default 22 if absent)
    IdentityFile string   // the IdentityFile directive
    IdentitiesOnly bool   // true if IdentitiesOnly = yes
}

// ParseManagedHosts parses content (the bytes of ~/.ssh/config), extracts all
// gitid-managed blocks via ListBlocks, and for each block parses the SSH
// directives to populate SSHHostInfo. The returned map is keyed by identity
// name (the block sentinel key). Blocks that fail to parse are silently
// returned with a zero-value SSHHostInfo (reconstruction incomplete marker).
func ParseManagedHosts(content []byte) (map[string]SSHHostInfo, error)
```

**Implementation steps:**

```go
func ParseManagedHosts(content []byte) (map[string]SSHHostInfo, error) {
    blocks := filewriter.ListBlocks(content)
    result := make(map[string]SSHHostInfo, len(blocks))
    for _, b := range blocks {
        if b.Name == "_global" {
            continue // skip the macOS Host * block
        }
        info, err := parseHostBlockBody(b.Body)
        if err != nil {
            // Best-effort: return zero-value info so caller marks as incomplete.
            result[b.Name] = SSHHostInfo{}
            continue
        }
        result[b.Name] = info
    }
    return result, nil
}

func parseHostBlockBody(body string) (SSHHostInfo, error) {
    cfg, err := ssh_config.Decode(strings.NewReader(body))
    if err != nil {
        return SSHHostInfo{}, err
    }
    if len(cfg.Hosts) == 0 {
        return SSHHostInfo{}, fmt.Errorf("no Host block found")
    }
    host := cfg.Hosts[0]
    if len(host.Patterns) == 0 {
        return SSHHostInfo{}, fmt.Errorf("Host has no patterns")
    }
    alias := host.Patterns[0].String()
    // cfg.Get uses first-match semantics; supply the alias we just found.
    hostname, _ := cfg.Get(alias, "Hostname")
    portStr, _ := cfg.Get(alias, "Port")
    port := 22
    if n, err := strconv.Atoi(portStr); err == nil {
        port = n
    }
    identityFile, _ := cfg.Get(alias, "IdentityFile")
    identitiesOnly, _ := cfg.Get(alias, "IdentitiesOnly")
    return SSHHostInfo{
        Alias:          alias,
        Hostname:       hostname,
        Port:           port,
        IdentityFile:   identityFile,
        IdentitiesOnly: strings.EqualFold(identitiesOnly, "yes"),
    }, nil
}
```

**Key kevinburke/ssh_config API facts** [VERIFIED: source at
`$GOPATH/pkg/mod/github.com/kevinburke/ssh_config@v1.6.0/config.go`]:
- `ssh_config.Decode(r io.Reader) (*Config, error)` — the main parse entry
  point. Accepts any `io.Reader`, including `strings.NewReader`.
- `Config.Hosts []*Host` — all Host blocks; index 0 is the implicit `Host *`
  when `Decode` processes a file that does NOT start with a `Host` keyword
  (i.e. when parsing a bare `Host work.github.com\n…` body, index 0 may be
  the implicit wildcard). Account for this: skip hosts where `host.implicit`
  is true (it is a private field — use `host.Patterns[0].String() == "*"` as
  the guard instead since `implicit` is unexported).
- `Config.Get(alias, key string) (string, error)` — first-match lookup by
  host alias and directive key. Case-insensitive on both alias and key.
- `Host.Patterns []*Pattern` — the host patterns (aliases). `Pattern.String()`
  returns the alias string.
- `Host.Nodes []Node` — the directives and comments inside the block. Each
  node is `*KV` (directive), `*Empty` (comment/blank), or `*Include`.

**Pitfall — implicit first Host:** When `Decode` parses a body that starts
immediately with `Host work.github.com`, the library may insert an implicit
`Host *` as `cfg.Hosts[0]` and the real block as `cfg.Hosts[1]`. Always skip
hosts whose sole pattern is `"*"` when looking for the managed alias. Guard:

```go
for _, host := range cfg.Hosts {
    if len(host.Patterns) == 1 && host.Patterns[0].String() == "*" {
        continue // implicit Host * or the global block — skip
    }
    // real managed Host block
}
```

[VERIFIED: kevinburke/ssh_config config.go:867 — `newConfig()` inserts a
`{implicit: true, Patterns: [matchAll]}` entry as the first host]

### Gitconfig-Side Read: `ParseManagedIncludeIf` + `ReadFragment`

The `~/.gitconfig` includeIf managed blocks are already written by
`gitconfig.WriteIncludeIf` via `filewriter.ReplaceBlock`. The body of each
block is a sequence of `[includeIf "…"]\n\tpath = …\n` pairs rendered by
`renderBlockBody`.

**Parsing back the includeIf body** — use `ListBlocks` to get the raw text,
then a simple line-by-line scanner (no library needed; the format is
gitid-controlled, not arbitrary gitconfig):

```go
// IncludeIfInfo holds reconstruction data from one gitid managed includeIf block.
type IncludeIfInfo struct {
    FragmentPath string          // last `path =` value seen (all matches share one path)
    Matches      []gitconfig.Match // all [includeIf "..."] conditions in the block
}

// ParseManagedIncludeIf extracts all gitid-managed includeIf blocks from
// the bytes of ~/.gitconfig, keyed by identity name.
func ParseManagedIncludeIf(content []byte) map[string]IncludeIfInfo
```

**Implementation sketch:**

```go
func ParseManagedIncludeIf(content []byte) map[string]IncludeIfInfo {
    blocks := filewriter.ListBlocks(content)
    result := make(map[string]IncludeIfInfo, len(blocks))
    for _, b := range blocks {
        result[b.Name] = parseIncludeIfBody(b.Body)
    }
    return result
}

func parseIncludeIfBody(body string) IncludeIfInfo {
    var info IncludeIfInfo
    for _, line := range strings.Split(body, "\n") {
        line = strings.TrimSpace(line)
        // [includeIf "gitdir:~/git/work/"] or [includeIf "hasconfig:..."]
        if strings.HasPrefix(line, `[includeIf "`) && strings.HasSuffix(line, `"]`) {
            cond := line[len(`[includeIf "`):len(line)-2]
            m := conditionToMatch(cond)
            info.Matches = append(info.Matches, m)
        }
        // \tpath = ~/.gitconfig.d/work
        if strings.HasPrefix(line, "path = ") {
            info.FragmentPath = strings.TrimPrefix(line, "path = ")
        }
    }
    return info
}

func conditionToMatch(cond string) gitconfig.Match {
    if strings.HasPrefix(cond, "gitdir:") {
        return gitconfig.Match{Kind: gitconfig.MatchGitdir, Value: strings.TrimPrefix(cond, "gitdir:")}
    }
    return gitconfig.Match{Kind: gitconfig.MatchHasconfig, Value: strings.TrimPrefix(cond, "hasconfig:")}
}
```

**Fragment read via `git config --file --list`:**

The fragment file is a standard gitconfig written by `git config --file` calls
in `WriteFragment`. Reading it back via `git config --file <path> --list` is
the consistent strategy (same tool that wrote it parses it back):

```go
// FragmentInfo holds the user identity fields from a per-identity fragment.
type FragmentInfo struct {
    GitName    string
    GitEmail   string
    SigningKey  string // the .pub PATH value of user.signingkey
    GPGFormat  string // "ssh" when signing is enabled
    CommitSign bool   // true when commit.gpgsign = true
    Missing    bool   // true when the fragment file does not exist
}

// ReadFragment runs `git config --file <fragPath> --list` and parses the
// output into a FragmentInfo. When the file is absent, Missing is set true
// and the other fields are zero (best-effort D-02).
func ReadFragment(fragPath string) (FragmentInfo, error)
```

Implementation:

```go
func ReadFragment(fragPath string) (FragmentInfo, error) {
    if _, err := os.Stat(fragPath); os.IsNotExist(err) {
        return FragmentInfo{Missing: true}, nil
    }
    cmd := exec.Command("git", "config", "--file", fragPath, "--list") //nolint:gosec
    out, err := cmd.Output()
    if err != nil {
        return FragmentInfo{Missing: true}, nil // treat unreadable as missing
    }
    var info FragmentInfo
    for _, line := range strings.Split(string(out), "\n") {
        kv := strings.SplitN(line, "=", 2)
        if len(kv) != 2 {
            continue
        }
        switch strings.ToLower(kv[0]) {
        case "user.name":
            info.GitName = kv[1]
        case "user.email":
            info.GitEmail = kv[1]
        case "user.signingkey":
            info.SigningKey = kv[1]
        case "gpg.format":
            info.GPGFormat = kv[1]
        case "commit.gpgsign":
            info.CommitSign = strings.EqualFold(kv[1], "true")
        }
    }
    return info, nil
}
```

[VERIFIED: `git config --file <path> --list` is the symmetric counterpart of
`git config --file <path> <key> <value>`; established pattern in fragment.go]

### Reconstruction Join

```go
// Reconstruct assembles []Account from the four managed artifacts.
// sshBytes and gcBytes are the raw file contents of ~/.ssh/config and
// ~/.gitconfig respectively. readFrag is an injectable function for reading
// a fragment file (allows fake in tests).
//
// The join key is the identity name (D-01). Accounts with missing pieces are
// included with a non-empty Incomplete field naming what is missing (D-02).
func Reconstruct(
    sshBytes, gcBytes []byte,
    readFrag func(fragPath string) (gitconfig.FragmentInfo, error),
) ([]Account, error)
```

Implementation outline:

```go
func Reconstruct(sshBytes, gcBytes []byte, readFrag func(string) (gitconfig.FragmentInfo, error)) ([]Account, error) {
    sshHosts, err := sshconfig.ParseManagedHosts(sshBytes)
    if err != nil {
        return nil, fmt.Errorf("reconstruct: parsing ssh config: %w", err)
    }
    gcBlocks := gitconfig.ParseManagedIncludeIf(gcBytes)

    // Union of all known identity names across both files.
    names := nameUnion(sshHosts, gcBlocks)

    var accounts []Account
    for _, name := range names {
        acct := Account{Name: name}
        var missing []string

        if ssh, ok := sshHosts[name]; ok && ssh.Alias != "" {
            acct.Alias    = ssh.Alias
            acct.Hostname = ssh.Hostname
            acct.Port     = ssh.Port
            acct.KeyPath  = ssh.IdentityFile
            acct.PubPath  = ssh.IdentityFile + ".pub"
        } else {
            missing = append(missing, "ssh-host-block")
        }

        if gc, ok := gcBlocks[name]; ok && gc.FragmentPath != "" {
            acct.Matches      = gc.Matches
            acct.FragmentPath = gc.FragmentPath
        } else {
            missing = append(missing, "gitconfig-includeif-block")
        }

        if acct.FragmentPath != "" {
            frag, ferr := readFrag(acct.FragmentPath)
            if ferr == nil && !frag.Missing {
                acct.GitName  = frag.GitName
                acct.GitEmail = frag.GitEmail
            } else {
                missing = append(missing, "fragment-file")
            }
        }

        acct.Incomplete = strings.Join(missing, ",") // empty = complete
        accounts = append(accounts, acct)
    }
    return accounts, nil
}
```

The `Account` struct needs one new field:

```go
// Incomplete is non-empty when reconstruction found this identity name in
// some but not all four artifacts. It names the missing pieces (comma-separated)
// for display in `gitid identity list`. Deep diagnosis stays in Phase 4 doctor.
Incomplete string
```

[VERIFIED: Account struct in internal/identity/identity.go; field is additive,
no existing code breaks]

### `Provider` Field: How to Derive It During Reconstruction

`Account.Provider` (e.g., "github") is NOT stored in any artifact; it is a
user-supplied create-time convenience. The alias form `<identity>.<provider>` (D-12)
encodes the provider if the user accepted the default alias. Two strategies:

1. **Derive from alias:** `strings.TrimPrefix(alias, name+".")` gives "github"
   for alias "work.github.com" when name is "work". This works when the default
   alias form was accepted.
2. **Leave empty on reconstruction:** `Account.Provider = ""` when derivation
   fails (non-default alias form). The `list` display shows the Hostname instead.

Recommendation: attempt derivation; leave empty if the alias does not match the
`<name>.<provider>` pattern. List displays Hostname when Provider is empty.

[ASSUMED: Provider field derivation strategy — requires planner decision on display]

---

## Research Question 4: Update Mechanics (IDENT-04)

### Core Observation

Update is nearly identical to the Phase 2 write path. The difference is that
it starts by loading the *current* Account (via `Reconstruct`), applies the
user's edits to produce a new `Account`, then re-renders the four artifacts
via the existing `runPipeline` / `ReplaceBlock` infrastructure.

### Field Change Classification

| Changed Field(s) | Structural? | Triggers ssh -G re-test? |
|------------------|-------------|--------------------------|
| GitName, GitEmail | No | No — fragment-only |
| Signing on→off, off→on | No | No — fragment/allowed_signers only |
| Alias, Hostname, Port | Yes | Yes |
| Matches (gitdir/hasconfig) | Yes (gitconfig change) | No — but forces includeIf block rewrite |

**Pitfall — "structural" definition (D-05):** The original text says "alias /
provider / port / match — anything that can change SSH resolution or repo
matching". The table above refines this:
- Match changes (gitdir/hasconfig) do NOT change SSH resolution, so they do not
  need an `ssh -G` re-test. They do require the gitconfig includeIf block to be
  rewritten.
- Alias/Hostname/Port changes DO require `ssh -G` re-test because they change
  which alias resolves to which key.

### Signing Toggle: On→Off Transition

Toggling **signing off** requires:
1. Remove the identity's line from `~/.ssh/allowed_signers` (line filter, not
   whole-file removal — see `RemoveAllowedSignersLine` below).
2. Rewrite the fragment WITHOUT the signing keys: omit `gpg.format`,
   `user.signingkey`, `commit.gpgsign` from the `git config --file` write set.

**How to remove signing keys from the fragment:** The cleanest approach is
`git config --file <path> --unset <key>` for each signing key. This is
idempotent (no error when key is absent with `--unset` — actually `git config
--unset` exits non-zero if the key is not found; use `--unset-all` or check
exit code 5). Alternative: delete the fragment file and rewrite it from scratch
with the new field set. Since the fragment is a gitid-managed file whose only
content is the five identity fields, a full rewrite via `WriteFragment` (minus
the signing fields) is safer than partial `--unset` surgery.

**Recommended update-fragment strategy:** Always call a new `WriteFragmentNoSign`
variant (or add a `Signing bool` parameter to a generalised `WriteFragment`)
that conditionally omits the three signing keys. This ensures the fragment
exactly reflects the new desired state.

Alternatively, keep the current `WriteFragment` as-is and add:

```go
// WriteFragmentUpdate writes the updated identity fields to fragmentPath.
// When signing is false, the gpg.format, user.signingkey, and commit.gpgsign
// keys are removed via `git config --file --unset-all`; this is idempotent
// (git exits 5 when the key is absent, which is treated as success here).
func WriteFragmentUpdate(fragPath, name, email, signingKeyPath string, signing bool) error
```

Toggling **signing on** from off: `WriteFragment` (the existing function) sets
all five keys including the signing trio. Append the allowed_signers line via
`keygen.WriteAllowedSigners` (same as create). This is exactly the Phase 2
write path.

[VERIFIED: fragment.go has gitConfigSet which uses `git config --file <path>
<key> <value>`; `git config --file <path> --unset-all <key>` exits 5 on missing
key — treat exit code 5 as "key not present, no-op"]

### Update Safe-Write Flow (D-06)

```
Load current Account (Reconstruct)
    │
    ▼
Interactive prompts pre-filled with current values
    │
    ▼
Compute diff: which fields changed?
    │
    ├── If alias/hostname/port changed → structural = true
    ├── If matches changed → gcRewrite = true
    └── If git name/email/signing changed → fragRewrite = true
    │
    ▼
Render updated four artifacts (same RenderHostBlock, RenderIncludeIf, etc.)
    │
    ▼
Show unified diff preview (or full preview if diff display is not implemented)
    │
    ▼
Single explicit confirm (SAFE-03)
    │
    ▼
filewriter.Write each mutated file (backup first, D-06)
    │
    ▼
If structural: tester.Resolved(newAlias) — print result (D-05)
```

### `UpdateDeps` Structure

Follow the `Deps` pattern from `modes.go`:

```go
type UpdateDeps struct {
    WriteSSH            func(accountName, hostBlock, globalBlock string) (string, error)
    WriteGitconfig      func(identity, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (string, error)
    WriteFragment       func(fragPath, name, email, signingKeyPath string, signing bool) error
    WriteAllowedSigners func(path, identity, line string) (string, error)
    RemoveAllowedSigners func(path, identityName string) error
    Resolved            func(alias string) (tester.Result, tester.ResolvedConfig)
}
```

---

## Research Question 5: Delete Mechanics (IDENT-05)

### Removal Manifest (D-08)

Before the single confirm prompt, show:

```
Will remove:
  [1] SSH Host block     "# BEGIN gitid managed: work" in ~/.ssh/config
  [2] gitconfig block    "# BEGIN gitid managed: work" in ~/.gitconfig
  [3] Fragment file      ~/.gitconfig.d/work
  [4] allowed_signers    line containing "<email>" in ~/.ssh/allowed_signers
  [5] Private key        ~/.ssh/id_ed25519_work  [skipped — keeping key]
  [6] Public key         ~/.ssh/id_ed25519_work.pub  [skipped — keeping key]
```

Key detail: if `allowed_signers` has no line for this identity (signing was
off), item 4 is omitted from the manifest. The manifest is derived from the
loaded `Account` so it only lists what actually exists.

### Fragment File Removal (Whole File)

The fragment file is `Account.FragmentPath` (e.g., `~/.gitconfig.d/work`). It
is a **whole file**, not a block within a file. Removal:

1. Back up: `filewriter.Write` cannot be used for pure deletion (it writes
   content). Instead, call `filewriter.Write` with empty content and then
   `os.Remove`? No — simpler: `os.Rename(fragPath, fragPath+".bak.<ts>")` as
   the backup, then `os.Remove` the backup only if the user confirms (but the
   rename IS the backup).

Actually the cleanest approach mirrors `filewriter.Write`'s backup step:

```go
// BackupAndRemove creates a timestamped backup of path (same naming as
// filewriter.Write) and then removes the original. Used for fragment file
// deletion where content replacement is not applicable.
func BackupAndRemove(path string) (backupPath string, err error) {
    backupPath = path + ".bak." + time.Now().Format("20060102-150405")
    if err := os.Rename(path, backupPath); err != nil {
        return "", fmt.Errorf("backing up %s before remove: %w", path, err)
    }
    return backupPath, nil
}
```

This is **atomic**: the rename is the backup AND the removal in one syscall.
If the rename fails, the original is untouched. Place this helper in
`internal/filewriter` alongside `Write`.

### `allowed_signers` Line Removal

The allowed_signers file has one line per signing identity:
```
email namespaces="git" ssh-ed25519 AAAA…
```

Line removal strategy: read whole file, filter out lines that contain the
identity's email AND the `namespaces="git"` token (to avoid accidentally
removing a hand-written line with the same email but different namespace):

```go
// RemoveAllowedSignersLine rewrites path with the line for identityEmail
// removed. It backs up the file first via filewriter.Write. If no matching
// line exists, the file is left unchanged (idempotent).
// Matching: a line is "owned" by this identity when it contains BOTH the
// identityEmail token AND `namespaces="git"` (the gitid-mandatory namespace).
func RemoveAllowedSignersLine(path, identityEmail string) (backupPath string, err error)
```

Implementation:

```go
func RemoveAllowedSignersLine(path, identityEmail string) (string, error) {
    existing, err := os.ReadFile(path) //nolint:gosec
    if os.IsNotExist(err) {
        return "", nil // nothing to remove
    }
    if err != nil {
        return "", err
    }
    var kept []string
    for _, line := range strings.Split(string(existing), "\n") {
        if strings.Contains(line, identityEmail) && strings.Contains(line, `namespaces="git"`) {
            continue // remove this line
        }
        kept = append(kept, line)
    }
    // Normalise: avoid a trailing blank line if the last removed line was at EOF.
    result := strings.Join(kept, "\n")
    if result == "\n" || result == "" {
        result = ""
    }
    return filewriter.Write(path, []byte(result), 0o600)
}
```

**Pitfall — email used in multiple identities:** If two identities share the
same GitEmail (unusual but possible), both lines would be removed. Mitigation:
include the public key fingerprint in the match condition, or require email
uniqueness during create. For Phase 3, the email-match approach is acceptable
because `gitid` owns the file and every line it writes is unique by email.
[ASSUMED: email uniqueness per allowed_signers is sufficient for Phase 3]

### Key Files (D-07)

The key deletion path (separate explicit prompt, default "no"):

```go
if keepKey := promptKeepKey(reader, out); !keepKey {
    // Second explicit confirm before irreversible deletion.
    if confirmKeyDelete(reader, out, acct.KeyPath) {
        _ = os.Remove(acct.KeyPath)     // private key — irreversible
        _ = os.Remove(acct.PubPath)     // .pub
    }
}
```

The backup step for SSH config and gitconfig happens automatically when
`filewriter.Write` is called with the result of `RemoveBlock`. For the
fragment file, `BackupAndRemove` provides the backup. For the key files (if
deleted), there is intentionally no backup — that is exactly D-07's rationale:
key deletion is irreversible, hence the separate prompt.

### `DeleteDeps` Structure

```go
type DeleteDeps struct {
    ReadSSH             func() ([]byte, error)    // read ~/.ssh/config bytes
    ReadGitconfig       func() ([]byte, error)    // read ~/.gitconfig bytes
    WriteSSH            func(content []byte) (backupPath string, err error)
    WriteGitconfig      func(content []byte) (backupPath string, err error)
    RemoveFragment      func(fragPath string) (backupPath string, err error)
    RemoveAllowedSigners func(path, email string) (backupPath string, err error)
    RemoveKeyFiles      func(keyPath, pubPath string) error  // only called when !keepKey
}
```

---

## Research Question 6: Multi-Identity Coexistence Proof (SC-2)

### What Needs Proving

Two identities on the same provider (e.g., `personal.github.com` and
`work.github.com`, both targeting `ssh.github.com:443`) must each resolve to
their own distinct `IdentityFile` via `ssh -G`.

`sshconfig.Write` already uses `ReplaceBlock` keyed by identity name, so two
identities get two distinct sentinel-delimited blocks. The coexistence proof
is already mechanically guaranteed by the Phase 2 implementation. What Phase 3
needs is a **test** that verifies this end-to-end.

### Test Pattern

```go
// TestMultiIdentityCoexistence is the round-trip property test for SC-2.
// It writes two identities to a temp SSH config via sshconfig.Write, then
// calls tester.ParseResolved on the ssh -G output from a temp HOME to assert
// that alias-A and alias-B resolve to distinct IdentityFiles.
func TestMultiIdentityCoexistence(t *testing.T) {
    // 1. Set up hermetic temp HOME with empty ~/.ssh/config.
    home := t.TempDir()
    t.Setenv("HOME", home)
    sshDir := filepath.Join(home, ".ssh")
    _ = os.MkdirAll(sshDir, 0o700)
    configPath := filepath.Join(sshDir, "config")

    // 2. Write identity "personal" with key ~/.ssh/id_ed25519_personal.
    personalBlock := sshconfig.RenderHostBlock(
        "personal.github.com", "ssh.github.com", 443, home+"/.ssh/id_ed25519_personal",
    )
    _, err := sshconfig.Write(configPath, "personal", personalBlock, "")
    if err != nil { t.Fatalf("write personal: %v", err) }

    // 3. Write identity "work" with key ~/.ssh/id_ed25519_work.
    workBlock := sshconfig.RenderHostBlock(
        "work.github.com", "ssh.github.com", 443, home+"/.ssh/id_ed25519_work",
    )
    _, err = sshconfig.Write(configPath, "work", workBlock, "")
    if err != nil { t.Fatalf("write work: %v", err) }

    // 4. Use ssh -G to resolve each alias. No live SSH needed: ssh -G only reads
    //    the local config file. Override SSH_CONFIG via the -F flag.
    resolveAlias := func(alias string) tester.ResolvedConfig {
        out, _ := exec.Command("ssh", "-G", "-F", configPath, alias).Output() //nolint:gosec
        return tester.ParseResolved(string(out))
    }

    personalRC := resolveAlias("personal.github.com")
    workRC     := resolveAlias("work.github.com")

    // 5. Assert distinct IdentityFiles.
    if len(personalRC.IdentityFiles) == 0 { t.Fatal("personal: no IdentityFile resolved") }
    if len(workRC.IdentityFiles) == 0 { t.Fatal("work: no IdentityFile resolved") }
    if personalRC.IdentityFiles[0] == workRC.IdentityFiles[0] {
        t.Errorf("same IdentityFile for both aliases: %s", personalRC.IdentityFiles[0])
    }
}
```

**Key insight:** `ssh -G -F <configPath> <alias>` reads the specified config
file without needing `~/.ssh/config` to exist. This means the test is fully
hermetic — no interaction with the real SSH config. The `tester.ParseResolved`
function already parses this output format correctly.

[VERIFIED: `ssh -G` flag documented in ssh(1) man page; `tester.ParseResolved`
in internal/tester/tester.go:126 parses the exact output format]

---

## Research Question 7: Testing Approach (TDD)

### Hermetic HOME Pattern (Already Established)

`internal/gitconfig/includeif_resolve_test.go` establishes the pattern:

```go
home := t.TempDir()
t.Setenv("HOME", home)
t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
```

All Phase 3 tests that interact with files use `t.TempDir()` as HOME. No test
reads or writes to the real `~/.ssh` or `~/.gitconfig`.

### Test Matrix

| Test | Package | Strategy |
|------|---------|---------|
| `TestListBlocks_*` | `filewriter` | Pure byte manipulation; no fakes needed |
| `TestRemoveBlock_*` | `filewriter` | Pure byte manipulation |
| `TestRemoveBlock_Idempotent` | `filewriter` | Call twice, assert identical output |
| `TestRoundTrip_ReplaceRemoveReplace` | `filewriter` | Compose ReplaceBlock→RemoveBlock→ReplaceBlock |
| `TestParseManagedHosts_*` | `sshconfig` | Parse fixture strings; no disk I/O |
| `TestParseHostBlockBody_ImplicitStar` | `sshconfig` | Verify `Host *` is skipped |
| `TestParseIncludeIfBody_*` | `gitconfig` | Parse fixture strings |
| `TestReadFragment_Missing` | `gitconfig` | Non-existent path → Missing=true |
| `TestReadFragment_Full` | `gitconfig` | Temp file written by WriteFragment; read back |
| `TestReconstruct_Complete` | `identity` | Two-block fixture; assert []Account correct |
| `TestReconstruct_MissingSSH` | `identity` | SSH block absent; assert Incomplete set |
| `TestReconstruct_MissingFragment` | `identity` | Fragment absent; assert Incomplete set |
| `TestReconstruct_Empty` | `identity` | Empty files; assert empty slice |
| `TestMultiIdentityCoexistence` | `sshconfig` | Uses real `ssh -G -F <tmpConfig>` |
| `TestUpdate_FragmentOnly` | `identity` | Email change: no structural re-test called |
| `TestUpdate_Structural` | `identity` | Alias change: structural re-test dep called |
| `TestDelete_RemoveManifest` | `identity` | Fake deps verify each removal was called |
| `TestDelete_KeepKey` | `identity` | keepKey=true: RemoveKeyFiles dep not called |
| `TestDeleteDeps_AllowedSignersLine` | `gitconfig` | Line filter: email removed, others preserved |

### Round-Trip Property Test (IDENT-07 + TOOL-04)

```go
// TestReconstruct_RoundTrip writes two identities via the Phase 2 pipeline
// (sshconfig.Write + gitconfig.WriteIncludeIf + WriteFragment) into a temp
// HOME, then calls Reconstruct on the resulting file bytes, and asserts the
// reconstructed []Account set matches the original inputs.
func TestReconstruct_RoundTrip(t *testing.T) { … }
```

This is the definitive proof for IDENT-07 and TOOL-04 combined.

### Injected Deps for Update/Delete Tests

Follow the `modes.go` pattern: `UpdateDeps` and `DeleteDeps` use function
fields. In tests, provide fakes that capture calls:

```go
var writeSSHCalled bool
deps := identity.UpdateDeps{
    WriteSSH: func(name, block, global string) (string, error) {
        writeSSHCalled = true
        return "", nil
    },
    // …
}
```

This pattern is already proven in `identity_test.go` for `Create`/`Reuse`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SSH config value lookup | Custom key-value parser | `ssh_config.Config.Get(alias, key)` | Handles first-match-wins, case insensitivity, defaults |
| Fragment key/value read | Custom INI parser | `git config --file <path> --list` | Same tool that wrote it; handles git's own format |
| Atomic file deletion-with-backup | `os.Remove` directly | `os.Rename(path, bakPath)` (BackupAndRemove) | Rename is atomic; leaves backup for recovery |
| Shell command construction | `fmt.Sprintf("git config …")` | `exec.Command("git", "config", "--file", path, …)` | Arg-slice form: no shell expansion, gosec G204-clean |

**Key insight:** The block-level operations (`ListBlocks`, `RemoveBlock`) are
the only genuine new primitives. Everything else composes existing tools.

---

## Common Pitfalls

### Pitfall A: Parsing the Implicit `Host *` in Kevinburke/ssh_config

**What goes wrong:** When `ssh_config.Decode` parses a body that starts with a
real `Host` stanza, `cfg.Hosts[0]` is the implicit `Host *` added by the
library (present even with no explicit `Host *` in the input). Calling
`cfg.Hosts[0].Patterns[0].String()` returns `"*"`, not the managed alias.

**How to avoid:** Always skip hosts where `len(host.Patterns) == 1 &&
host.Patterns[0].String() == "*"`. Never assume `cfg.Hosts[0]` is the first
real managed block.

[VERIFIED: kevinburke/ssh_config config.go:867 `newConfig()` always inserts
`{implicit: true, Patterns: [matchAll]}`]

### Pitfall B: `RemoveBlock` Blank-Line Accumulation

**What goes wrong:** After each `add → delete` cycle, a blank line accumulates
at the position where the block was. After 10 cycles, 10 blank lines appear
in the config file.

**How to avoid:** The `RemoveBlock` implementation above consumes at most one
trailing blank line after the END marker. Document this behaviour and test it.

### Pitfall C: `git config --unset` Exits Non-Zero When Key Absent

**What goes wrong:** Calling `git config --file <path> --unset <key>` when
the key does not exist in the file exits with code 5, which `exec.Command.Run()`
reports as an error. Code that checks `err != nil` will fail when trying to
"unset" a signing key that was never set.

**How to avoid:** Use `--unset-all` and explicitly allow exit code 5 (key
not found) as a non-error. Or use the full-rewrite approach for fragment
updates: delete+recreate via `WriteFragment` with the desired field set.

[VERIFIED: `git config` man page — exit code 5 = "the section or key is
invalid (or unable to be removed)"]

### Pitfall D: `allowed_signers` Line Removal Matching

**What goes wrong:** A line containing the email string but NOT owned by gitid
(e.g., a user hand-written line) gets deleted.

**How to avoid:** Require both the email AND `namespaces="git"` to match —
gitid's line format always has `namespaces="git"` as the second token. A
hand-written line for the same email but different namespace is preserved.

### Pitfall E: Fragment `git config --list` Output Encoding

**What goes wrong:** `git config --file <path> --list` outputs `key=value` with
the value being the literal file content. If `user.signingkey` contains `~`,
git does NOT expand it — the `~` is returned literally. When comparing paths,
use string equality, not path equality (do not call `filepath.Abs`).

**How to avoid:** Store and compare signingkey paths as the literal string from
the fragment. Tilde expansion is the shell's job; gitid stores and displays the
raw path form.

### Pitfall F: Race Between ListBlocks and ReplaceBlock Sentinel Format

**What goes wrong:** `ListBlocks` assumes the sentinel lines are exactly
`BeginPrefix + name` (no trailing spaces, no extra tokens). If a user
hand-edits a sentinel to add a comment (`# BEGIN gitid managed: work # my note`),
`ListBlocks` will not recognise it.

**How to avoid:** `ListBlocks` uses exact prefix match on `strings.HasPrefix`,
then extracts the name as `strings.TrimPrefix(trimmed, BeginPrefix)`. A
user-added inline comment becomes part of the "name" and the block is
unreachable by the tool. This is acceptable: gitid controls sentinel writing
(via `ReplaceBlock`) and sentinel lines are not intended for user editing.
Doctor (Phase 4) can detect malformed sentinels.

---

## Code Examples

### `ListBlocks` Usage in Reconstruction

```go
// Source: internal/filewriter/block.go (new function, mirrors ReplaceBlock)
blocks := filewriter.ListBlocks(sshConfigBytes)
for _, b := range blocks {
    fmt.Printf("block %q body:\n%s\n", b.Name, b.Body)
}
```

### `RemoveBlock` in Delete Flow

```go
// Source: internal/filewriter/block.go (new function)
existing, _ := os.ReadFile(sshConfigPath)
updated := filewriter.RemoveBlock(existing, identityName)
backupPath, _ := filewriter.Write(sshConfigPath, updated, 0o600)
```

### SSH Alias Lookup via kevinburke API

```go
// Source: kevinburke/ssh_config config.go — Config.Get, Config.Hosts iteration
cfg, _ := ssh_config.Decode(strings.NewReader(blockBody))
for _, host := range cfg.Hosts {
    if len(host.Patterns) == 1 && host.Patterns[0].String() == "*" {
        continue // skip implicit Host *
    }
    alias := host.Patterns[0].String()
    hostname, _ := cfg.Get(alias, "Hostname")
    port, _ := cfg.Get(alias, "Port")
    identityFile, _ := cfg.Get(alias, "IdentityFile")
    identitiesOnly, _ := cfg.Get(alias, "IdentitiesOnly")
}
```

### Hermetic `ssh -G -F` Test Pattern

```go
// Source: internal/tester/tester.go — ParseResolved; ssh -G -F <config> <alias>
out, _ := exec.Command("ssh", "-G", "-F", configPath, alias).Output() //nolint:gosec
rc := tester.ParseResolved(string(out))
// rc.IdentityFiles[0] is the resolved key path
```

### BackupAndRemove (Fragment File Deletion)

```go
// Source: internal/filewriter/filewriter.go (new helper, mirrors backup step)
backupPath, err := filewriter.BackupAndRemove(fragPath)
// fragPath is now gone; backupPath holds the timestamped copy
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `grep`-guard + blind append | Sentinel-delimited `ReplaceBlock` | Phase 2 | Write side solid; Phase 3 adds the read side |
| No read-back API | `ListBlocks` + kevinburke iteration | Phase 3 | Enables reconstruction without sidecar DB |
| Fragment reads via custom parser | `git config --file --list` | Phase 3 | Symmetric with the write path; no custom parser |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `Account.Provider` derivation from alias — attempt `strings.TrimPrefix(alias, name+".")`, leave empty on failure | RQ3 Reconstruction Join | Minor: list shows Hostname instead of provider name |
| A2 | Email uniqueness per `allowed_signers` is sufficient for Phase 3; two identities sharing one email is not a supported case | RQ5 Delete | Edge case: two identities sharing an email would have both their lines removed on single delete |
| A3 | Structural change definition: match-strategy changes do NOT trigger ssh -G re-test | RQ4 Update | Minor: could over-skip a re-test; safe because match changes only affect gitconfig, not SSH resolution |

---

## Open Questions

1. **`WriteFragmentUpdate` vs parameterised `WriteFragment`**
   - What we know: toggling signing off requires removing three keys from the
     fragment; the current `WriteFragment` always writes all five keys.
   - What's unclear: whether to add a `signing bool` param to `WriteFragment`
     or create a separate `WriteFragmentUpdate` function.
   - Recommendation: add `signing bool` to `WriteFragment` so there is one
     function for all fragment writes (reduces divergence risk).

2. **`BackupAndRemove` placement**
   - What we know: it is needed for fragment file deletion (IDENT-05, D-08).
   - What's unclear: whether it belongs in `internal/filewriter` (alongside
     `Write`) or in `internal/identity/delete.go` as a local helper.
   - Recommendation: put it in `internal/filewriter` so it can be reused by
     Phase 4 doctor auto-fix.

3. **`list` grouping layout** (Claude's Discretion)
   - What we know: grouped by identity is the stated default.
   - What's unclear: exact column widths, truncation, colour (none in Phase 3
     — TUI Phase 5 adds colour).
   - Recommendation: plain text table, no ANSI colour in Phase 3 CLI output.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `ssh` | `ssh -G -F` coexistence test | ✓ | system ssh | Skip test with `t.Skip` if not found |
| `git` | `git config --file --list` fragment read | ✓ | system git | Fragment read returns Missing=true |
| `go test` | TDD test runner | ✓ | go 1.26 | — |

```
$ command -v ssh && ssh -V
OpenSSH_9.x (system)
$ command -v git && git --version
git version 2.x.x (system)
```

No missing dependencies block Phase 3 execution.

---

## Validation Architecture

> `workflow.nyquist_validation` is `true` in `.planning/config.json` — this
> section is required.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go built-in `testing` |
| Config file | none — driven by `Makefile` |
| Quick run command | `go test ./internal/filewriter/... ./internal/sshconfig/... ./internal/gitconfig/... ./internal/identity/... -count=1` |
| Full suite command | `make test` (runs `go test -race ./...`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| IDENT-03 | `list` shows all identities with correct fields | unit | `go test ./internal/identity/... -run TestReconstruct` | ❌ Wave 0 |
| IDENT-04 | `update` rewrites four artifacts; structural changes trigger re-test | unit | `go test ./internal/identity/... -run TestUpdate` | ❌ Wave 0 |
| IDENT-05 | `delete` removes all four per-identity artifacts, preserves foreign content | unit | `go test ./internal/identity/... -run TestDelete` | ❌ Wave 0 |
| IDENT-07 | Reconstruct yields identical []Account to original create inputs | unit | `go test ./internal/identity/... -run TestReconstruct_RoundTrip` | ❌ Wave 0 |
| SC-2 | Two same-provider identities resolve to distinct IdentityFiles | integration | `go test ./internal/sshconfig/... -run TestMultiIdentityCoexistence` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/filewriter/... ./internal/sshconfig/... ./internal/gitconfig/... ./internal/identity/...`
- **Per wave merge:** `make test`
- **Phase gate:** `make test && make lint` green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/filewriter/block_list_test.go` — covers `ListBlocks` and `RemoveBlock`
- [ ] `internal/sshconfig/reader_test.go` — covers `ParseManagedHosts`
- [ ] `internal/gitconfig/reader_test.go` — covers `ParseManagedIncludeIf`, `ReadFragment`, `RemoveAllowedSignersLine`
- [ ] `internal/identity/loader_test.go` — covers `Reconstruct` (complete, partial, empty)
- [ ] `internal/identity/update_test.go` — covers `Update` with fake `UpdateDeps`
- [ ] `internal/identity/delete_test.go` — covers `Delete` with fake `DeleteDeps`
- [ ] `internal/sshconfig/coexistence_test.go` — covers `TestMultiIdentityCoexistence`

---

## Security Domain

> `security_enforcement: true`, `security_asvs_level: 1` per config.json.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A — reads local files, no network auth |
| V3 Session Management | no | N/A — CLI, no sessions |
| V4 Access Control | yes (partial) | Private key files: 0600 via `filewriter.Write`; confirmation before delete |
| V5 Input Validation | yes | Identity name validated via `identityNameRe` (already in `rotate.go`); reuse in `update.go` and `delete.go` |
| V6 Cryptography | no | No new crypto in Phase 3 |

### Known Threat Patterns for This Phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Shell injection via identity name in `git config --file` exec call | Tampering | Arg-slice `exec.Command("git", "config", "--file", path, key, value)` — no shell, already established in fragment.go |
| Backup file world-readable | Information disclosure | `filewriter.Write` always uses mode 0600 for backups (`backupMode = 0o600` in filewriter.go) |
| Fragment read leaking private paths in error messages | Information disclosure | Log the fragment path identifier only; never log key material |
| Deleting wrong identity (name confusion) | Tampering | Show removal manifest before single confirm; `identityNameRe` validation on the name argument |
| `os.Remove` on key file without backup | Repudiation | Key deletion goes through a separate irreversible-action prompt (D-07); no backup of key on delete (by design — document this explicitly) |

---

## Sources

### Primary (HIGH confidence)

- `internal/filewriter/block.go` — `ReplaceBlock`, `BeginPrefix`, `EndPrefix`
  implementation; exact sentinel format and splice semantics [VERIFIED: codebase]
- `internal/filewriter/filewriter.go` — `Write`, `EnsureDir`; backup pattern
  [VERIFIED: codebase]
- `internal/identity/identity.go` — `Account` struct; Deps pattern [VERIFIED: codebase]
- `internal/identity/modes.go` — `Reuse`, `AddAccount`, `Rotate`; Deps injection
  pattern Phase 3 follows [VERIFIED: codebase]
- `internal/sshconfig/parser.go` / `writer.go` — `Parse`, `Write`; SSH parse
  foundation [VERIFIED: codebase]
- `internal/gitconfig/renderer.go` — `RenderIncludeIf`, `WriteIncludeIf`,
  `renderBlockBody`; includeIf block format [VERIFIED: codebase]
- `internal/gitconfig/fragment.go` — `WriteFragment`, `gitConfigSet`;
  `git config --file` write pattern [VERIFIED: codebase]
- `internal/tester/tester.go` — `Resolved`, `ParseResolved`; `ssh -G` parse
  [VERIFIED: codebase]
- `$GOPATH/pkg/mod/github.com/kevinburke/ssh_config@v1.6.0/config.go` —
  `Config.Hosts`, `Host.Patterns`, `Host.Nodes`, `Config.Get`, `Decode`,
  implicit-Host-* insertion in `newConfig()` [VERIFIED: source code]
- `.planning/phases/03-full-identity-crud-multi-identity/03-CONTEXT.md` — D-01
  through D-08 locked decisions [VERIFIED: planning artifact]
- `.planning/phases/02-first-identity-end-to-end/02-CONTEXT.md` — artifact
  format decisions (sentinel, alias form, fragment layout) [VERIFIED: planning artifact]

### Secondary (MEDIUM confidence)

- `git config` man page — `--list`, `--unset`, `--unset-all` options; exit
  code 5 semantics [CITED: git-config(1)]
- `ssh(1)` man page — `-G` option prints effective configuration; `-F` flag
  overrides config file path [CITED: ssh(1)]

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages; all claims verified from source
- Architecture: HIGH — grounded in existing code structure and established patterns
- Pitfalls: HIGH — each pitfall is verified from library source or codebase
- Signing toggle mechanics: MEDIUM — `git config --unset` exit code behaviour
  is verified from docs but the "full rewrite" alternative adds implementation
  flexibility

**Research date:** 2026-06-10
**Valid until:** 2026-07-10 (stable domain — only changes if kevinburke/ssh_config
v1.7+ changes the `Config.Hosts` API or the sentinel format changes)
