# Phase 4: Doctor - Research

**Researched:** 2026-06-11
**Domain:** Read-only health-check orchestration — ssh-agent probing, file-permission inspection, finding/severity model, CLI rendering, coherence/orphan checks, baseline checks
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01 (read-only core + decoupled fixer):** `internal/doctor` stays pure — returns structured findings and never writes. A finding may carry an optional fix descriptor; the cmd layer applies it, routing every mutation through `internal/filewriter` (backup + atomic + idempotent). Detection and mutation are separate concerns.

**D-02 (auto-fixable classes):**
- Permissions — `chmod` to KEY-02 targets (`~/.ssh` 700, key 600, `.pub` 644, config 600)
- Orphaned managed blocks — remove a gitid-managed block whose counterpart is gone (fragment with no `includeIf`, alias Host block with no `includeIf`)
- Missing-wiring re-add — re-add a missing `allowed_signers` line / missing `IdentitiesOnly yes`, reconstructed from other managed blocks

**D-03 (everything else report-only):** dependency installs, key-file deletion, ssh-agent loading, user-edited value drift are report-only — doctor shows the exact suggested command but never runs it.

**D-04 (CLI trigger + confirm semantics):**
- `gitid doctor` → pure report; when fixable findings exist, offer a top-level "apply fixes?" gate; on yes → per-finding confirm
- `gitid doctor --fix` → skips the top-level gate, goes straight to per-finding confirm for each fixable finding
- `gitid doctor --fix --yes` → applies all fixable findings without prompts; `--yes` IS the explicit SAFE-03 confirmation
- Permissions may batch under one confirm; orphaned-block removal and wiring re-add confirm individually (higher blast radius)

**D-05 (severity model — 4 levels):** `critical` / `error` / `warning` / `info`
- `critical` = key/secret exposure (e.g., private key world-readable)
- `error` = broken (missing required dep, `IdentityFile` that won't resolve, auth/config will fail)
- `warning` = degraded/risky (agent not running, `git < 2.36` with `hasconfig:`, locked-value drift, unreferenced key)
- `info` = advisory (optional tool missing)

**D-06 (layout — grouped by family, show passes):** render in sections — Dependencies, Permissions, Coherence, Orphans, Signing, Agent (+ Baseline). Within each, show `✓` for passing checks and the finding for failures.

**D-07 (tiered exit codes):** highest severity present sets the code — `0` clean / `1` warning+info / `2` error / `3` critical. Anything not-clean is non-zero.

**D-08 (color):** color on a TTY (red/yellow/green per severity), auto-plain when piped/redirected, respect `NO_COLOR` env var.

**D-09 (orphan = artifact with no owning block):** orphans are the inverse of Phase 3's "incomplete" marker.

**D-10 (distinct families):** orphans report under their own `Orphans` family, distinct from `Coherence`.

**D-11 (managed-block orphans are the fixable ones):** a fragment file with no `includeIf`, or an alias `Host` block with no matching `includeIf`, is a managed-block orphan → auto-fixable removal.

**D-12 (unused key scope — cross-ref ALL Host blocks):** unused-key check cross-references a private key against every `Host` block in `~/.ssh/config`, gitid-managed AND hand-written.

**D-13 (unused key ⇒ `warning` only, honest wording):** severity warning, never error/critical; wording admits gitid cannot know it is unused.

**D-14 (`known_hosts` correlation — REJECTED):** `known_hosts` stores server host keys, not client-key→host usage. Do not propose.

**D-15 (coherence = existence/resolution only):** no full content compare.

**D-16 (fold in ALL four Phase 3.1 baseline checks):** excludesfile wiring, baseline `[include]` resolves, ignorecase drift, curated excludes present.

**D-17 (bounded locked-value carve-outs):** `ignorecase=false`, `gpg.format=ssh`, `allowed_signers` email == `user.email` are the only value checks.

**D-18 (ssh-agent = reachable + managed keys loaded):** check reachable (`ssh-add -l`), then warn per managed identity whose key is not currently loaded.

**D-19 (full content-compare rejected).**

**D-20 (git<2.36 + `hasconfig:` warning):** use `deps.GitVersionAtLeast(2, 36)` already implemented.

### Claude's Discretion

- Command/flag naming and help text (exact flag names, help copy, consistent with Phase 2/3 pattern)
- Family ordering and exact line formatting of grouped report (whether Baseline is its own section vs folded)
- Per-OS install-hint text for `git` and the clipboard tool (extend `platform.InstallHint` pattern)
- Finding/severity type shape (struct fields, how the optional fix descriptor is modeled)
- Where the fixer lives in `cmd/gitid` and how it re-uses `filewriter` removal / gitconfig/sshconfig writers

### Deferred Ideas (OUT OF SCOPE)

- Map non-git keys to SSH server hosts (future — no reliable persistent source today)
- `known_hosts`-based correlation (D-14, rejected)
- TUI doctor dashboard (Phase 5 — DOC-07 TUI half)
- Full Cobra surface + shell completion (Phase 5)
- `url-rewrites` block health checks
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DOC-01 | `gitid doctor` checks dependencies (`ssh`, `ssh-keygen`, `ssh-add`, `git`, clipboard tool) with per-OS install hints | `deps.Detect()` + `deps.Report.MissingRequired()` already implemented; extend `platform.InstallHint` switch to git + clipboard tools |
| DOC-02 | Doctor checks permissions on `~/.ssh`, keys, `.pub`, and config | `os.Stat` + `FileInfo.Mode().Perm()` for read; `os.Chmod` for fix; gosec G304/G306 annotations needed |
| DOC-03 | Doctor checks coherence/drift — every `IdentityFile` resolves, every `includeIf` points to existing fragment, `IdentitiesOnly yes` is present, signing identities have an `allowed_signers` line | Compose `identity.Reconstruct` + `sshconfig.ParseManagedHosts` + `gitconfig.ReadFragment` + `os.Stat` for resolution checks |
| DOC-04 | Doctor detects orphans — unused keys, non-included fragments, aliases without matching `includeIf` | `filewriter.ListBlocks` cross-reference against `identity.Reconstruct` result to find unowned artifacts |
| DOC-05 | Doctor checks signing wiring (`gpg.format=ssh`, `allowed_signers` path) and ssh-agent status, and warns if `git < 2.36` when `hasconfig:` is used | `ssh-add -l` probe via `os/exec` + exit-code semantics; `ssh-keygen -lf` for fingerprint matching; `deps.GitVersionAtLeast(2,36)` |
| DOC-06 | Each finding has severity + explanation + suggested fix; auto-fix offered with confirmation | UI-agnostic `Finding` struct with optional `Fix` descriptor; cmd-layer fixer drives `filewriter` |
| DOC-07 (CLI half) | `gitid doctor` available as CLI command | Cobra subcommand in `cmd/gitid/doctor.go` following baseline/list patterns |
</phase_requirements>

---

## Summary

Phase 4 composes six check families (Dependencies, Permissions, Coherence, Orphans, Signing/Agent, Baseline) from already-proven primitives into a new `internal/doctor` package that returns `[]Finding` — pure data, no writes. The cmd layer renders the grouped report, detects which findings are auto-fixable, and drives the fixer through `internal/filewriter`.

The three genuinely new surfaces to research in depth are: (1) **ssh-agent probing** via `os/exec ssh-add -l` with precise exit-code semantics and fingerprint-based identity matching, (2) **file-permission inspection** in Go using `os.Stat`/`FileInfo.Mode().Perm()`, and (3) **TTY detection** for color vs. plain output using the already-present `golang.org/x/sys` transitive dependency.

No new external packages are needed. The only potential new `go.mod` dependency is `golang.org/x/term` for isatty, but `golang.org/x/sys/unix` (already a transitive dep via `golang.org/x/crypto`) provides `unix.IsTerminal(fd)` which is sufficient on darwin/linux; the stdlib `os.Stdout.Fd()` returns the file descriptor.

**Primary recommendation:** Model the `Finding` struct as pure data with an optional `FixDescriptor` carrying the fixer function as a field — this keeps `internal/doctor` write-free while allowing the cmd layer to execute fixes. Mirror the `identity.Deps` injected-function pattern exactly so the Phase 5 TUI can reuse.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Dependency presence checks | `internal/doctor` (reads) | `internal/deps` (probe) | `deps.Detect()` already owns tool probing; doctor composes it |
| Permission reading | `internal/doctor` (reads) | — | `os.Stat()` in doctor check functions; pure read, no write |
| Permission fixing | `cmd/gitid` (fixer layer) | `internal/filewriter` (chmod) | Doctor returns `FixDescriptor`; fixer calls `os.Chmod` via filewriter |
| Coherence reads | `internal/doctor` (reads) | `internal/sshconfig`, `internal/gitconfig`, `internal/identity` (parsers) | Doctor orchestrates the existing readers; no new parse logic |
| Orphan detection | `internal/doctor` (reads) | `internal/filewriter.ListBlocks` | Cross-reference managed block names against disk presence |
| ssh-agent probe | `internal/doctor` (reads) | os/exec (stdlib) | `ssh-add -l` + exit-code classification; injectable for testing |
| Report rendering | `cmd/gitid` (thin cmd layer) | — | Doctor returns `[]Finding`; cmd owns grouping + color + exit-code |
| Fix execution | `cmd/gitid` (fixer layer) | `internal/filewriter`, `internal/gitconfig`, `internal/sshconfig` | All mutations route through filewriter chokepoint |
| TTY detection | `cmd/gitid` (cmd layer) | `golang.org/x/sys/unix.IsTerminal` | Render decision (color vs plain) belongs in output layer |
| Baseline checks | `internal/doctor` (reads) | `internal/gitconfig.ReadBaselineState` | Doctor re-uses the existing `ReadBaselineState` primitive |

---

## Standard Stack

### Core (Phase 4 — no new external packages)

| Package | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os/exec` (stdlib) | — | Run `ssh-add -l`, `ssh-add -L`, `ssh-keygen -lf` | Already used everywhere in the project (tester, platform, gitconfig) |
| `os` (stdlib) | — | `os.Stat`, `FileInfo.Mode().Perm()`, `os.Chmod` for permission checks/fixes | Stdlib; no dependency needed |
| `golang.org/x/sys/unix` | v0.46.0 (already in go.sum as transitive dep) | `unix.IsTerminal(int(os.Stdout.Fd()))` for TTY detection | Already a transitive dep via x/crypto; no new go.mod entry needed |
| `internal/deps` | project | `Detect()`, `MissingRequired()`, `GitVersionAtLeast(2,36)` | Already proven, DOC-01 and D-20 are direct compositions |
| `internal/platform` | project | `InstallHint(os)` extended to git + clipboard | Extend existing switch, no new package |
| `internal/identity` | project | `Reconstruct(...)`, `Account` struct | Foundation for coherence + orphan checks |
| `internal/filewriter` | project | `ListBlocks`, `RemoveBlock`, `ReplaceBlock` | Orphan fix routing |
| `internal/gitconfig` | project | `ReadFragment`, `ParseManagedIncludeIf`, `ReadBaselineState`, `BaselineKeySet` | Coherence + baseline checks |
| `internal/sshconfig` | project | `ParseManagedHosts` | SSH coherence checks |

**No new `go get` needed.** `golang.org/x/sys` is already in `go.sum` (transitive via `golang.org/x/crypto`). If the planner decides to avoid `x/sys` entirely, `os.Getenv("TERM")` + checking for `NO_COLOR` is an acceptable fallback that requires zero new deps.

### Package Legitimacy Audit

No new external packages are introduced in this phase. All code uses project-internal packages and Go stdlib.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| (none) | — | — | — | — | — | No new packages |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

---

## Architecture Patterns

### System Architecture Diagram

```
gitid doctor [--fix] [--yes]
        │
        ▼
cmd/gitid/doctor.go
  ├── builds doctor.Deps from real internal packages
  ├── calls doctor.Run(deps) → []Finding
  ├── renders grouped report (Families, ✓ / severity)
  ├── computes exit code (highest severity)
  ├── [if fixable findings && --fix or user says yes]
  │       ├── per-finding confirm (or --yes bypasses)
  │       └── fixer routes through internal/filewriter
  └── os.Exit(exitCode)

doctor.Run(deps) [internal/doctor]
  ├── checkDeps(deps)         → []Finding   [DOC-01]
  ├── checkPermissions(deps)  → []Finding   [DOC-02]
  ├── checkCoherence(deps)    → []Finding   [DOC-03]
  ├── checkOrphans(deps)      → []Finding   [DOC-04]
  ├── checkSigning(deps)      → []Finding   [DOC-05 signing half]
  ├── checkAgent(deps)        → []Finding   [DOC-05 agent half]
  └── checkBaseline(deps)     → []Finding   [D-16]

Each check function:
  - receives injected deps (fake-testable)
  - reads filesystem / runs subprocesses (read-only)
  - returns []Finding (pure data)

Finding {Family, Severity, Title, Explanation, SuggestedFix, FixDescriptor?}
  ├── FixDescriptor is non-nil only for auto-fixable classes (D-02)
  └── FixDescriptor carries enough info for the cmd fixer to act
      without any reference back to the core
```

### Recommended Project Structure (Phase 4 additions)

```
internal/doctor/
├── doc.go            # (exists) package doc — never writes
├── doctor.go         # Finding type, Severity enum, Deps struct, Run() func
├── doctor_test.go    # Table-driven tests with fake deps
├── checks/
│   ├── deps.go       # checkDeps() — composes internal/deps
│   ├── deps_test.go
│   ├── perms.go      # checkPermissions() — os.Stat + Mode().Perm()
│   ├── perms_test.go
│   ├── coherence.go  # checkCoherence() — existence/resolution checks
│   ├── coherence_test.go
│   ├── orphans.go    # checkOrphans() — ListBlocks vs disk
│   ├── orphans_test.go
│   ├── signing.go    # checkSigning() + checkAgent() — gpg.format, allowed_signers, ssh-add
│   ├── signing_test.go
│   ├── baseline.go   # checkBaseline() — folds D-16 four checks
│   └── baseline_test.go

cmd/gitid/
├── doctor.go         # newDoctorCmd(), runDoctor(), renderReport(), runFixer()
└── doctor_test.go    # thin cmd-layer tests (render output shape)
```

### Pattern 1: Finding + Severity Data Model

**What:** A UI-agnostic struct returned from every check function. The optional `FixDescriptor` carries the fixer function as a value so `internal/doctor` never imports `filewriter` and stays write-free.

```go
// Source: internal/doctor/doctor.go (to be created)

// Severity represents the urgency of a finding.
// [ASSUMED] — type names are planner discretion (D-Claude)
type Severity int

const (
    SeverityInfo     Severity = iota // advisory only
    SeverityWarning                  // degraded / risky
    SeverityError                    // broken, will fail
    SeverityCritical                 // key/secret exposure
)

// String returns the lowercase display name.
func (s Severity) String() string {
    switch s {
    case SeverityCritical:
        return "critical"
    case SeverityError:
        return "error"
    case SeverityWarning:
        return "warning"
    default:
        return "info"
    }
}

// Family groups findings for the report sections (D-06).
type Family string

const (
    FamilyDeps       Family = "Dependencies"
    FamilyPerms      Family = "Permissions"
    FamilyCoherence  Family = "Coherence"
    FamilyOrphans    Family = "Orphans"
    FamilySigning    Family = "Signing"
    FamilyAgent      Family = "Agent"
    FamilyBaseline   Family = "Baseline"
)

// FixDescriptor carries the information needed for the cmd-layer fixer
// to apply a fix WITHOUT the core writing anything. The Fn field is nil
// for report-only findings (D-03). Fn is set only for auto-fixable classes (D-02).
// The cmd layer calls Fn; internal/doctor never calls it.
type FixDescriptor struct {
    Summary string            // one-line human label shown in the confirm prompt
    Fn      func() error      // executes the fix when called by the cmd layer
}

// Finding is one health-check result. It is pure data — no write operations.
// [ASSUMED] — exact field names are planner discretion.
type Finding struct {
    Family      Family
    Severity    Severity
    Title       string         // short identifier (e.g., "key-600")
    Explanation string         // plain-English description shown to user
    SuggestedFix string        // exact suggested command string (always shown)
    Fix         *FixDescriptor // non-nil = auto-fixable (D-02); nil = report-only
}

// Passed represents a check family where ALL checks passed — shown as ✓.
type Passed struct {
    Family  Family
    Message string // e.g., "all permissions correct"
}
```

**Severity → exit code aggregation (D-07):**

```go
// ExitCode returns the tiered exit code for a result set.
// 0 = clean, 1 = warning/info, 2 = error, 3 = critical (highest wins).
func ExitCode(findings []Finding) int {
    code := 0
    for _, f := range findings {
        switch f.Severity {
        case SeverityCritical:
            return 3 // can't get worse
        case SeverityError:
            if code < 2 {
                code = 2
            }
        case SeverityWarning, SeverityInfo:
            if code < 1 {
                code = 1
            }
        }
    }
    return code
}
```

### Pattern 2: Injected-Deps for Fake-Testable Checks

**What:** Mirror the `identity.Deps` pattern exactly. Every external effect is an injected function field.

```go
// Source: internal/doctor/doctor.go (to be created)

// Deps holds every external read the doctor performs, as injectable function fields.
// Real implementations call the actual packages; tests inject fakes.
// [ASSUMED] — exact field set grows with check families; shown as representative subset.
type Deps struct {
    // ReadFile reads a file; injectable for tests.
    ReadFile func(path string) ([]byte, error)
    // Stat returns file info; injectable for tests.
    Stat func(path string) (os.FileInfo, error)
    // RunSSHAdd runs ssh-add -l and returns (stdout, exitCode).
    // The injected version is the seam for agent tests — no live agent needed.
    RunSSHAdd func() (output string, exitCode int)
    // RunSSHKeygenFingerprint runs ssh-keygen -lf <path> and returns the fingerprint line.
    RunSSHKeygenFingerprint func(pubKeyPath string) (string, error)
    // RunGitConfigGet runs git config --file <path> --get <key>.
    RunGitConfigGet func(filePath, key string) (string, error)
    // Identities is the reconstructed account list (from identity.Reconstruct).
    Identities []identity.Account
    // GitVersionAtLeast reports git version constraint; injectable for tests.
    GitVersionAtLeast func(major, minor int) bool
    // CurrentOS returns the OS token for install-hint dispatch.
    CurrentOS func() string
}

// Run executes all check families and returns all findings.
// Passing checks are NOT returned as findings; they are expressed by
// the absence of a finding for that check. The cmd layer derives ✓ lines
// from the absence of findings per family.
func Run(deps Deps) []Finding {
    var all []Finding
    all = append(all, CheckDeps(deps)...)
    all = append(all, CheckPermissions(deps)...)
    all = append(all, CheckCoherence(deps)...)
    all = append(all, CheckOrphans(deps)...)
    all = append(all, CheckSigning(deps)...)
    all = append(all, CheckAgent(deps)...)
    all = append(all, CheckBaseline(deps)...)
    return all
}
```

### Pattern 3: ssh-agent Probing via os/exec

**What:** Exact exit-code semantics for `ssh-add -l` and fingerprint-based identity matching.

**Exit-code semantics for `ssh-add -l`:** [VERIFIED from local probe on macOS OpenSSH]

| Condition | Exit code | stdout | Interpretation |
|-----------|-----------|--------|----------------|
| Agent running, keys loaded | 0 | one line per key: `<bits> SHA256:<hash> <comment> (<type>)` | Agent up, keys present |
| Agent running, NO keys | 1 | `The agent has no identities.` | Agent up, empty |
| Agent unreachable (`SSH_AUTH_SOCK` unset or socket dead) | 2 | `Could not open a connection to your authentication agent.` | Agent not running |

**Important portability note:** [ASSUMED] Some older OpenSSH versions (pre-7.x) may emit exit 2 for "no keys" instead of exit 1. Modern macOS and Linux OpenSSH (9.x) behave as documented above. The safe parsing strategy is: treat exit 0 as "running with keys", treat exit 1 or "agent has no identities" text as "running empty", treat exit 2 or connection error text as "unreachable".

```go
// Source: internal/doctor/checks/signing.go (representative implementation)

// runSSHAdd is the real RunSSHAdd function used in production Deps.
// Returns the stdout output and the raw exit code.
// [ASSUMED] — exact function signature is planner discretion.
func realRunSSHAdd() (string, int) {
    cmd := exec.Command("ssh-add", "-l") //nolint:gosec // fixed args, no user input (G204)
    out, err := cmd.Output()
    if err != nil {
        if ee, ok := err.(*exec.ExitError); ok {
            return string(ee.Stderr), ee.ExitCode()
        }
        return "", 2 // treat exec failure as unreachable
    }
    return string(out), 0
}

// classifyAgentState interprets the (output, exitCode) from ssh-add -l.
// [ASSUMED] — internal helper; names are planner discretion.
type agentState int
const (
    agentUnreachable agentState = iota
    agentRunningEmpty
    agentRunningWithKeys
)

func classifyAgentState(output string, exitCode int) agentState {
    switch exitCode {
    case 0:
        return agentRunningWithKeys
    case 1:
        // Double-check text in case of version quirks.
        if strings.Contains(output, "no identities") || strings.Contains(output, "has no identities") {
            return agentRunningEmpty
        }
        return agentRunningEmpty // treat any exit 1 as empty
    default: // 2 or any error
        return agentUnreachable
    }
}
```

**Fingerprint matching strategy for D-18:**

`ssh-add -l` outputs one line per loaded key in the form:
```
256 SHA256:vRBdzHYKWKt131j4W3gBbBwqid2tALp3weJk9eZz1hE castocolina@gmail.com (ED25519)
```

`ssh-keygen -lf <path-to-pub-key>` outputs the same format for a specific key file:
```
256 SHA256:vRBdzHYKWKt131j4W3gBbBwqid2tALp3weZz1hE castocolina@gmail.com (ED25519)
```

[VERIFIED from local probe] Strategy: for each managed identity, run `ssh-keygen -lf <account.PubPath>`, extract the `SHA256:...` token, then check if that token appears in the `ssh-add -l` output. This is robust because SHA256 fingerprints do not collide in practice, and both commands output the same format.

```go
// extractFingerprint parses the SHA256:... token from a ssh-keygen -lf line.
// Input: "256 SHA256:vRBdzHY... comment (ED25519)"
// Output: "SHA256:vRBdzHY..."
// [ASSUMED] — pure function, testable without subprocess.
func extractFingerprint(keygenLine string) string {
    fields := strings.Fields(keygenLine)
    for _, f := range fields {
        if strings.HasPrefix(f, "SHA256:") {
            return f
        }
    }
    return ""
}

// isKeyLoaded reports whether the key at pubKeyPath is currently in the agent.
// It calls the injected RunSSHKeygenFingerprint and searches the agentOutput.
func isKeyLoaded(agentOutput, pubKeyPath string, runFp func(string) (string, error)) bool {
    fpLine, err := runFp(pubKeyPath)
    if err != nil {
        return false // treat fingerprint failure as not-loaded
    }
    fp := extractFingerprint(fpLine)
    return fp != "" && strings.Contains(agentOutput, fp)
}
```

**`ssh-add -L` (public key bodies) is NOT needed** for this check. Only `-l` (fingerprints) is needed.

### Pattern 4: File Permission Checking in Go

**What:** Read and compare Unix permissions using `os.Stat().Mode().Perm()`.

```go
// Source: internal/doctor/checks/perms.go (representative implementation)

// targetMode maps each path class to its required permission bits.
// [ASSUMED] — matches KEY-02 requirements exactly.
var targetModes = map[string]os.FileMode{
    "dir":    0o700, // ~/.ssh directory
    "key":    0o600, // private key files
    "pub":    0o644, // .pub files
    "config": 0o600, // ~/.ssh/config, ~/.gitconfig
}

// checkFilePerm returns a Finding if the file at path does not have the
// expected mode. Returns nil if perm is correct or file does not exist.
// [ASSUMED] — function signature is planner discretion.
func checkFilePerm(path string, want os.FileMode, severity Severity, deps Deps) *Finding {
    info, err := deps.Stat(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil // absent files are handled by coherence, not perms
        }
        return &Finding{/* ... */}
    }
    got := info.Mode().Perm()
    if got == want {
        return nil // pass
    }
    return &Finding{
        Family:       FamilyPerms,
        Severity:     severity,
        Title:        fmt.Sprintf("perm-%s", filepath.Base(path)),
        Explanation:  fmt.Sprintf("%s has mode %04o; expected %04o", path, got, want),
        SuggestedFix: fmt.Sprintf("chmod %04o %s", want, path),
        Fix: &FixDescriptor{
            Summary: fmt.Sprintf("chmod %04o %s", want, path),
            Fn: func() error {
                return os.Chmod(path, want)
            },
        },
    }
}
```

**Severity classification for permission failures (D-05):**
- Private key world-readable (mode & 0o044 != 0) → `critical` (key exposure)
- Private key group-readable (mode & 0o040 != 0) → `critical`
- `~/.ssh/config` world-readable → `error` (SSH may reject the file)
- `~/.ssh` directory with wrong mode → `error` (enumeration risk)
- `.pub` with wrong mode → `warning` (read errors for tools, not security risk)

**Symlink edge case:** `os.Stat` follows symlinks (resolves to the target). If the key path is a symlink, the check evaluates the target's permissions, which is the correct behavior since SSH opens the target. Use `os.Lstat` only if you need to check the symlink itself — for permission checks, `os.Stat` is appropriate. [ASSUMED]

**Ownership check:** `FileInfo.Sys().(*syscall.Stat_t).Uid` returns the owner UID. Cross-reference with `os.Getuid()` to detect files owned by another user. Doctor should check ownership only when the permission check itself passes (a file owned by root with mode 600 is still accessible to root but not to the current user). [ASSUMED — ownership check is an optional enhancement; the locked decisions do not require it explicitly.]

**gosec annotation required:** Any `os.Stat` or `os.ReadFile` on a user-supplied or derived path must carry `//nolint:gosec // path is a trusted gitid-managed path (G304)` consistent with the existing codebase pattern.

### Pattern 5: TTY Detection and Color Output

**What:** Detect whether stdout is a terminal to enable color; respect `NO_COLOR`.

**The `golang.org/x/term` package** (`term.IsTerminal(fd)`) is the canonical stdlib-adjacent approach and is already used as a transitive dependency path through `x/crypto`. However, `go.mod` does not currently list it explicitly. To avoid adding a new `require` entry, use the `golang.org/x/sys/unix.IsTerminal` variant which is already indirectly present.

**Simplest zero-new-dependency approach:** [ASSUMED]

```go
// isTerminal reports whether fd is a terminal, using the syscall layer.
// Falls back to false (plain output) on any error.
// [ASSUMED] — internal helper in cmd/gitid/doctor.go
func isTerminalOutput(f *os.File) bool {
    if os.Getenv("NO_COLOR") != "" {
        return false // NO_COLOR spec: https://no-color.org/
    }
    // golang.org/x/sys is a transitive dep via golang.org/x/crypto.
    // If it becomes unavailable, replace with: return false (always plain).
    // Using the file descriptor directly (unix.IsTerminal requires int fd).
    fi, err := f.Stat()
    if err != nil {
        return false
    }
    return (fi.Mode() & os.ModeCharDevice) != 0
}
```

**Note on `os.ModeCharDevice`:** `fi.Mode() & os.ModeCharDevice != 0` is the portable Go-stdlib approach for "is this a character device (i.e., a terminal)?". It works on both darwin and linux without importing `x/sys`. [ASSUMED — this is a well-known Go pattern but should be confirmed against Go docs.]

Actually the more reliable stdlib approach uses `os.Stdin`/`os.Stdout` with `os.ModeCharDevice`. The canonical check is:

```go
// checkIsTerminal returns true when f is a real TTY (not a pipe/file).
// Does NOT require x/term or x/sys — pure stdlib.
func checkIsTerminal(f *os.File) bool {
    if os.Getenv("NO_COLOR") != "" {
        return false
    }
    stat, err := f.Stat()
    if err != nil {
        return false
    }
    return (stat.Mode() & os.ModeCharDevice) != 0
}
```

This is the recommended approach: zero new imports, works identically to `term.IsTerminal` for the stdout-piping use case. If the planner needs full VT100 terminal capability detection (which doctor does not), then `x/term` would be needed.

**Color codes (minimal, no lipgloss needed for CLI):** [ASSUMED — consistent with Phase 5 TUI using lipgloss, CLI uses ANSI directly]

```go
const (
    colorReset   = "\033[0m"
    colorRed     = "\033[31m"   // critical + error
    colorYellow  = "\033[33m"   // warning
    colorGreen   = "\033[32m"   // passing checks (✓)
    colorCyan    = "\033[36m"   // info
)
```

### Pattern 6: Coherence and Orphan Check Mechanics

**Coherence checks (DOC-03 + D-15):** Existence/resolution only — no value comparison except for the three locked-value carve-outs (D-17).

**Check 1 — IdentityFile resolves:**
```go
// For each account in identity.Reconstruct result:
if account.KeyPath != "" {
    _, err := deps.Stat(account.KeyPath)
    if os.IsNotExist(err) {
        // error: IdentityFile does not exist
    }
}
```

**Check 2 — includeIf fragment exists:**
```go
// For each account:
if account.FragmentPath != "" {
    _, err := deps.Stat(account.FragmentPath)
    if os.IsNotExist(err) {
        // error: includeIf points to missing fragment
    }
}
```

**Check 3 — IdentitiesOnly yes is present:**
```
sshconfig.ParseManagedHosts returns SSHHostInfo.IdentitiesOnly (bool).
If false for a managed host → error finding.
```

**Check 4 — signing identity has allowed_signers line:**
```go
// Read ~/.ssh/allowed_signers via deps.ReadFile.
// For each account with FragmentPath set, read fragment via gitconfig.ReadFragment.
// If frag.GPGFormat == "ssh" (signing enabled), verify that allowed_signers
// contains a line whose first field == frag.GitEmail AND contains namespaces="git".
// If absent → error finding.
```

**Locked-value checks (D-17):**
```go
// ignorecase drift: git config --file ~/.gitconfig.d/00-baseline --get core.ignorecase
// Must equal "false"; if "true" → warning finding.

// gpg.format=ssh: for each identity whose fragment is present,
// frag.GPGFormat must equal "ssh" → if not, error finding.

// allowed_signers email match: email in signing line must byte-match frag.GitEmail.
```

**Orphan detection (DOC-04 + D-09):**

```go
// Orphan = artifact exists on disk but no managed block claims it.
// Algorithm:
// 1. Enumerate all managed block names: filewriter.ListBlocks(sshConfigBytes) + ListBlocks(gitconfigBytes)
// 2. Cross-reference against identity.Reconstruct result (which tracks Incomplete).
// 3. An artifact is orphaned when:
//    a. A gitid-named key file exists at ~/.ssh/gitid_<name> but no SSH managed block with that name exists.
//    b. A fragment file exists at ~/.gitconfig.d/<name> but no gitconfig managed includeIf block with that name exists.
//    c. A managed SSH block exists with a name, but no gitconfig includeIf block with that name → fragment orphan.
//    d. A gitignore_global file exists with no managed block (handled under Baseline orphan check).
//
// Incomplete (from Reconstruct) is distinct from orphan:
//   incomplete = managed block exists, artifact missing → Coherence family
//   orphan = artifact exists, no owning block → Orphans family
```

**Baseline checks (D-16):**

```go
// Compose gitconfig.ReadBaselineState to check:
// 1. excludesfile wiring: state.BaselineKeys["core.excludesfile"] must be set and file must exist
// 2. baseline [include] resolves: state.Installed must be true; if Incomplete → Baseline finding
// 3. ignorecase drift: state.BaselineKeys["core.ignorecase"] must equal "false"
// 4. curated excludes present: verify gitconfig.DefaultGitignorePatterns() subset is in state.GitignorePatterns
```

### Anti-Patterns to Avoid

- **Writing from the doctor core:** `internal/doctor` MUST be write-free (D-01). No `os.WriteFile`, `os.Chmod`, `filewriter.Write` inside the package. The `FixDescriptor.Fn` captures the fix as a closure but is never called from within the package.
- **known_hosts correlation (D-14):** Do not read `~/.ssh/known_hosts` for any purpose. It stores server host keys, not client-key usage.
- **Full content-compare of managed blocks (D-19):** Do not re-render blocks and diff. Existence + the three locked-value checks is the correct scope.
- **Using ssh-add exit code alone:** Always check both exit code AND output text. Old OpenSSH versions may use different exit codes.
- **os.Lstat for permission checks:** Use `os.Stat` (follows symlinks) for permission checks, not `os.Lstat`. The permission on the symlink target is what SSH sees.
- **Hardcoding color codes when piped:** Always gate color output on the isTerminal check + NO_COLOR. Broken color codes in CI logs are a significant usability problem.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tool presence check | Custom `exec.LookPath` wrappers | `deps.Detect()` already in `internal/deps` | Proven, tested, handles optional vs required distinction |
| Per-OS install hints | Duplicate switch statements | Extend `platform.InstallHint()` switch for git + clipboard | Single authoritative location; CLAUDE.md mandates this pattern |
| Managed block enumeration | Custom sentinel scanner | `filewriter.ListBlocks()` | CRLF-tolerant, handles incomplete blocks, test-proven |
| Managed block removal (fixer) | Custom splice logic | `filewriter.RemoveBlock()` | Anti-accumulation guard already implemented |
| Fragment reads | Custom gitconfig line parser | `gitconfig.ReadFragment()` | Handles missing files, uses `git config --list` for correctness |
| Identity reconstruction | Re-implementing join logic | `identity.Reconstruct()` | Proven join-by-name logic with Incomplete marker |
| Allowed-signers line validation | Custom regex | Check first-field email match + `strings.Contains(line, namespaces="git")` | The existing `RemoveAllowedSignersLine` already shows the correct parse pattern |
| Git version gating | Custom semver parser | `deps.GitVersionAtLeast(2, 36)` | Already implemented in `internal/deps` |

**Key insight:** Doctor is almost entirely composition of proven primitives. The only genuinely new code is the `Finding` type definition, the `Deps` struct wiring, the ssh-agent probe helper, the TTY detection helper, and the per-family check functions that connect the primitives.

---

## Common Pitfalls

### Pitfall 1: ssh-add Exit Code Portability

**What goes wrong:** Assuming exit code 1 always means "agent empty" — some OpenSSH versions use exit 2. Code that returns `agentUnreachable` for exit 1 will falsely warn that every key is not loaded.

**Why it happens:** OpenSSH has changed the exit code semantics for `ssh-add -l` between versions. The "no identities" text is more portable than the exit code alone.

**How to avoid:** Classify agent state by BOTH exit code AND presence of the "no identities" text in output. The classifyAgentState function above shows the correct approach.

**Warning signs:** `warning: identity <x> key not loaded in agent` appears even when the key IS loaded.

### Pitfall 2: `os.Stat` vs `os.Lstat` for Symlinks

**What goes wrong:** Using `os.Lstat` on a key path that is a symlink returns the symlink's own permissions (usually 0777), not the target's permissions. The check always passes, missing actual 644 private keys behind a symlink.

**Why it happens:** `os.Lstat` is chosen to "avoid following symlinks" but permission checking should evaluate what SSH will see (the target).

**How to avoid:** Use `os.Stat` (not `os.Lstat`) for permission checks. SSH itself follows symlinks when reading key files.

**Warning signs:** `~/.ssh/id_ed25519` is a symlink to a 644-mode file; doctor reports no permission issue but SSH fails to authenticate.

### Pitfall 3: gosec G304 Annotations on Stat/ReadFile Paths

**What goes wrong:** `golangci-lint` (gosec G304) flags any `os.ReadFile(path)` where `path` comes from a variable as a potential file-path injection vector. The CI hard-fails.

**Why it happens:** gosec does not reason about provenance — it cannot tell whether `path` is a gitid-managed path or free user input.

**How to avoid:** Follow the existing project pattern: every `os.ReadFile`, `os.Stat`, `os.Chmod` on a path derived from gitid's own config must carry the annotation:
```go
info, _ := os.Stat(keyPath) //nolint:gosec // keyPath is a trusted gitid-managed path (G304)
```
**Warning signs:** `make lint` fails on new doctor code before any tests run.

### Pitfall 4: Doctor Importing filewriter (Breaks Write-Free Contract)

**What goes wrong:** A check function needs to report a managed-block list AND returns a `FixDescriptor.Fn` that calls `filewriter.RemoveBlock`. If the doctor package imports filewriter, the architectural write-free contract is visibly (if not functionally) violated. The `internal/doctor` package doc says "never writes."

**Why it happens:** The fix closure captures a filewriter function reference to execute the fix. If captured as an actual import, the dependency is present even if the function is never called from the core.

**How to avoid:** Two viable approaches:
1. **Closure injection:** The cmd-layer `buildDoctorDeps` function creates the `FixDescriptor.Fn` closures using `filewriter` package functions, and passes them into doctor as part of the finding payload. `internal/doctor` never imports `filewriter`.
2. **Fix function injection in Deps:** Add `FixPerm func(path string, mode os.FileMode) error`, `RemoveBlock func(path, name string) error` etc. to the `Deps` struct, so the fixer capabilities are injected. The `FixDescriptor.Fn` closes over the injected dep.

Approach 2 is more consistent with the existing `identity.Deps` injected-function pattern. Approach 1 is simpler but moves fix logic into cmd.

**Warning signs:** `import "github.com/castocolina/gitid/internal/filewriter"` appears in `internal/doctor/*.go`.

### Pitfall 5: "Incomplete" vs "Orphan" Confusion

**What goes wrong:** Reporting an identity's missing fragment under Orphans when it should be under Coherence (and vice versa). The `identity.Reconstruct` `Incomplete` field already classifies the "managed block exists, piece missing" case.

**Why it happens:** Both look like "something is missing." The distinction is:
- **Coherence (DOC-03):** A managed block exists with a name → references a thing → the thing is absent. The block is the source of truth; the artifact is broken.
- **Orphan (DOC-04):** An artifact exists on disk → no managed block claims it. The artifact has no owner.

**How to avoid:** Check `account.Incomplete != ""` under Coherence, not Orphans. Check for key files / fragment files with NO corresponding block under Orphans.

**Warning signs:** Two findings with the same artifact path appear under both families.

### Pitfall 6: allowed_signers Email Must Be Byte-Identical

**What goes wrong:** Treating `user@Example.com` and `user@example.com` as matching when checking the `allowed_signers` file for the presence of a signing line. Git's SSH signing verification is case-sensitive for the principal email.

**Why it happens:** String comparison with `strings.EqualFold` instead of direct `==`.

**How to avoid:** Use `==` (exact byte match) for the email comparison in allowed_signers checks, consistent with `RemoveAllowedSignersLine` in `internal/gitconfig/reader.go` which also uses exact match (line 156: `fields[0] == identityEmail`).

**Warning signs:** `git log --show-signature` shows "No public key" even though an allowed_signers line exists.

### Pitfall 7: ssh-keygen -lf on Missing/Inaccessible Key

**What goes wrong:** `ssh-keygen -lf ~/.ssh/gitid_work` returns exit 1 with error output when the file does not exist or is unreadable. If the caller does not handle the error and treats a blank fingerprint as "not loaded," it may produce a false "key not in agent" finding when the real problem is the key file is missing (which is ALREADY a coherence error).

**Why it happens:** Agent check runs after coherence check conceptually, but in a flat check list all checks run.

**How to avoid:** In the agent check, only attempt fingerprint matching for identities that already passed the coherence check (i.e., `account.KeyPath != ""` AND `Stat(account.PubPath)` succeeds). If the pub file is missing, skip the agent fingerprint check — the coherence check already reported the issue.

---

## Code Examples

### Verified Patterns from Existing Code

**`os.Stat` for file existence and mode (from existing codebase):**
```go
// Source: internal/gitconfig/reader.go:72
if _, statErr := os.Stat(fragPath); os.IsNotExist(statErr) {
    return FragmentInfo{Missing: true}, nil
}
```

```go
// Source: cmd/gitid/baseline.go:200 (snapshotFile)
info, err := os.Stat(path) //nolint:gosec // path is a trusted gitid-managed path
// info.Mode().Perm() — extracts the 9 permission bits as os.FileMode
```

**`deps.GitVersionAtLeast` for version gating (from existing codebase):**
```go
// Source: cmd/gitid/baseline.go:68
if !deps.GitVersionAtLeast(2, 35) {
    cfg.MergeConflictStyle = ""
}
// For D-20: use GitVersionAtLeast(2, 36) for hasconfig: warning
```

**`identity.Reconstruct` for coherence check foundation:**
```go
// Source: internal/identity/loader.go:17
accounts, err := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
// accounts[i].Incomplete != "" → coherence finding
// accounts[i].KeyPath → check os.Stat(account.KeyPath) for resolution
```

**`filewriter.ListBlocks` for orphan detection:**
```go
// Source: internal/filewriter/block.go:18
blocks := filewriter.ListBlocks(sshConfigBytes)
// blocks[i].Name → identity name
// Cross-reference against identity.Reconstruct account names to find orphans
```

**`gitconfig.ReadBaselineState` for baseline checks:**
```go
// Source: internal/gitconfig/baseline.go:171
state, err := gitconfig.ReadBaselineState(absGitconfig, absBaseline, absGitignore)
// state.Installed — baseline include + file both exist
// state.Incomplete — some-but-not-all artifacts
// state.BaselineKeys["core.ignorecase"] — for D-17 drift check
// state.GitignorePatterns — for curated-excludes check
```

**`deps.Detect()` and `platform.InstallHint()` for DOC-01:**
```go
// Source: internal/deps/deps.go:79 + internal/platform/platform.go:109
report := deps.Detect()
if !report.Git {
    hint := platform.InstallHint(platform.CurrentOS()) // extend this for git
}
// Currently InstallHint only covers OpenSSH — extend the switch in platform.go
```

**Cobra command pattern (from cmd/gitid/baseline.go):**
```go
// Mirror newBaselineSetupCmd() structure:
func newDoctorCmd() *cobra.Command {
    var fix, yes bool
    cmd := &cobra.Command{
        Use:   "doctor",
        Short: "Run a health check on the gitid-managed environment",
        RunE: func(cmd *cobra.Command, _ []string) error {
            return runDoctor(cmd.OutOrStdout(), fix, yes)
        },
    }
    cmd.Flags().BoolVar(&fix, "fix", false, "apply auto-fixable findings (per-finding confirm)")
    cmd.Flags().BoolVar(&yes, "yes", false, "apply all fixes without prompts (requires --fix; SAFE-03)")
    return cmd
}
```

---

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| Bash `ssh-add -l` without exit-code parsing | Go `os/exec` + exit-code classification | Deterministic; injectable for tests |
| `known_hosts` for key-to-host mapping | Rejected — D-14 | `~/.ssh/config` `IdentityFile` per Host is the only reliable signal |
| isatty via C binding | `file.Stat().Mode() & os.ModeCharDevice` (pure stdlib) | Zero new imports |
| Separate color library for CLI | ANSI codes directly in cmd layer | Lipgloss for TUI (Phase 5); bare ANSI for CLI is lighter |

**Deprecated/outdated:**
- `ssh-add -l | grep` pattern: Use exit-code + output-text classification, not grep.
- Hardcoded 600 octal as string: Use `os.FileMode(0o600)` constants for clarity and go vet compatibility.

---

## Runtime State Inventory

This phase adds no new stored data, live service config, OS-registered state, secrets/env vars, or build artifacts. It is read-only at the detection layer and writes only through the existing `filewriter` chokepoint (identical to all prior phases).

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | None — doctor reads existing managed blocks, no new state | None |
| Live service config | None | None |
| OS-registered state | None | None |
| Secrets/env vars | `SSH_AUTH_SOCK` — read (not set) by doctor to interpret ssh-add behavior | Code reads only; no env mutation |
| Build artifacts | None | None |

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `ssh-add` | DOC-05 agent check (D-18) | Yes (macOS OpenSSH) | OpenSSH 9.x | If absent: report as info finding "ssh-add not found, agent check skipped" |
| `ssh-keygen` | Fingerprint matching for D-18 | Yes | OpenSSH 9.x | If absent: warn "cannot verify key is loaded" |
| `git` | D-17 locked-value reads via `git config --file` | Yes (already required) | 2.39+ on this machine | If absent: already surfaced by deps check |
| `golang.org/x/sys/unix.IsTerminal` | TTY detection | Transitive dep via x/crypto | v0.46.0 | Use `file.Stat().Mode() & os.ModeCharDevice` fallback — no import needed |

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `go test` (stdlib), `go test -race` on pre-push |
| Config file | `go.mod` only; no `testconfig` file |
| Quick run command | `go test ./internal/doctor/... ./cmd/gitid/... -run TestDoctor` |
| Full suite command | `make test` (go test -race -coverprofile=coverage.out ./...) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DOC-01 | Missing required tool reported with install hint | unit | `go test ./internal/doctor/... -run TestCheckDeps` | No — Wave 0 |
| DOC-01 | Optional tool missing reported as info (not error) | unit | `go test ./internal/doctor/... -run TestCheckDepsOptional` | No — Wave 0 |
| DOC-02 | Private key world-readable reported as critical | unit | `go test ./internal/doctor/... -run TestCheckPermsCritical` | No — Wave 0 |
| DOC-02 | Correct permissions produce no finding | unit | `go test ./internal/doctor/... -run TestCheckPermsPass` | No — Wave 0 |
| DOC-02 | FixDescriptor.Fn applies chmod (fixer test) | unit | `go test ./internal/doctor/... -run TestPermFixer` | No — Wave 0 |
| DOC-03 | IdentityFile path does not exist → error finding | unit | `go test ./internal/doctor/... -run TestCoherenceIdentityFileGone` | No — Wave 0 |
| DOC-03 | includeIf fragment path missing → error finding | unit | `go test ./internal/doctor/... -run TestCoherenceFragmentGone` | No — Wave 0 |
| DOC-03 | IdentitiesOnly absent → error finding | unit | `go test ./internal/doctor/... -run TestCoherenceIdentitiesOnly` | No — Wave 0 |
| DOC-03 | allowed_signers line absent for signing identity → error | unit | `go test ./internal/doctor/... -run TestCoherenceSignersLine` | No — Wave 0 |
| DOC-04 | Key file with no managed block → orphan warning | unit | `go test ./internal/doctor/... -run TestOrphanKey` | No — Wave 0 |
| DOC-04 | Fragment with no includeIf → orphan, auto-fixable | unit | `go test ./internal/doctor/... -run TestOrphanFragment` | No — Wave 0 |
| DOC-05 | gpg.format != ssh → error finding | unit | `go test ./internal/doctor/... -run TestSigningGPGFormat` | No — Wave 0 |
| DOC-05 | Agent unreachable (exit 2) → warning finding | unit | `go test ./internal/doctor/... -run TestAgentUnreachable` | No — Wave 0 |
| DOC-05 | Key not loaded in agent → warning per identity | unit | `go test ./internal/doctor/... -run TestAgentKeyNotLoaded` | No — Wave 0 |
| DOC-05 | git < 2.36 with hasconfig → warning | unit | `go test ./internal/doctor/... -run TestGitVersionGate` | No — Wave 0 |
| DOC-06 | Finding has severity + explanation + suggested fix | unit | `go test ./internal/doctor/... -run TestFindingFields` | No — Wave 0 |
| DOC-06 | ExitCode 3 when any critical finding present | unit | `go test ./internal/doctor/... -run TestExitCodeCritical` | No — Wave 0 |
| DOC-06 | ExitCode 2 when only error findings | unit | `go test ./internal/doctor/... -run TestExitCodeError` | No — Wave 0 |
| DOC-06 | ExitCode 0 when no findings | unit | `go test ./internal/doctor/... -run TestExitCodeClean` | No — Wave 0 |
| DOC-07 | `gitid doctor` command registered in root | unit | `go test ./cmd/gitid/... -run TestDoctorCmdRegistered` | No — Wave 0 |
| DOC-07 | Report groups findings by family with ✓ for passes | unit | `go test ./cmd/gitid/... -run TestDoctorRenderGrouped` | No — Wave 0 |
| D-16 | ignorecase drift detected | unit | `go test ./internal/doctor/... -run TestBaselineIgnoreCaseDrift` | No — Wave 0 |
| D-16 | excludesfile missing | unit | `go test ./internal/doctor/... -run TestBaselineExcludesfile` | No — Wave 0 |
| D-16 | curated excludes missing from gitignore | unit | `go test ./internal/doctor/... -run TestBaselineCuratedExcludes` | No — Wave 0 |

**TDD mode is ON.** Every test listed above should be written FIRST (RED commit), then implemented (GREEN commit). All checks in `internal/doctor` are pure functions receiving injected deps — they pass fakes, no filesystem access needed in unit tests. The `FixDescriptor.Fn` can be tested with an in-memory fake that records calls.

### Sampling Rate

- **Per task commit:** `go test ./internal/doctor/... ./cmd/gitid/... -run "TestCheck|TestDoctor|TestFinding|TestExitCode"` (fast, < 5s)
- **Per wave merge:** `make test` (full race-enabled suite, covers all packages)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/doctor/doctor_test.go` — covers DOC-06 Finding type + ExitCode + Run
- [ ] `internal/doctor/checks/deps_test.go` — covers DOC-01
- [ ] `internal/doctor/checks/perms_test.go` — covers DOC-02
- [ ] `internal/doctor/checks/coherence_test.go` — covers DOC-03
- [ ] `internal/doctor/checks/orphans_test.go` — covers DOC-04
- [ ] `internal/doctor/checks/signing_test.go` — covers DOC-05 (signing + agent)
- [ ] `internal/doctor/checks/baseline_test.go` — covers D-16
- [ ] `cmd/gitid/doctor_test.go` — covers DOC-07 CLI (render output, exit code propagation)
- [ ] Framework already installed — no new tooling needed

---

## Security Domain

Security enforcement is enabled (`security_enforcement: true`, ASVS level 1). All of the Phase 4 ASVS concerns are READ operations on files the user already owns — threat surface is low, but gosec compliance is hard-fail.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Doctor does not authenticate users |
| V3 Session Management | No | No session state |
| V4 Access Control | No | Doctor reads files the current user already owns |
| V5 Input Validation | Yes (minimal) | File paths are gitid-controlled (never free-form user input); gosec G304 annotations on all `os.ReadFile`/`os.Stat` calls |
| V6 Cryptography | No | No new crypto operations |

### Known Threat Patterns for This Phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal in fix-descriptor fixer function | Tampering | Fix paths are constructed from `account.KeyPath`/`account.FragmentPath` (gitid-managed, validated at create time); never accept free-form paths |
| gosec G204: subprocess arg injection | Tampering | Use arg-slice `exec.Command("ssh-add", "-l")` form; never shell interpolation. Annotate with `//nolint:gosec // fixed args, no user input (G204)` |
| gosec G304: file path reading | Info Disclosure | Annotate trusted paths with `//nolint:gosec // path is a trusted gitid-managed path (G304)` |
| Private key content logging | Info Disclosure | Doctor reads `os.Stat` for permissions (no content read of private keys). Never log key file contents — log only path and permission mode |
| gosec G306: chmod to broad mode | Tampering | The fixer sets TO the correct mode (600/644/700); gosec flags chmod as G306 when setting a "permissive" mode. Annotate `.pub` chmod 644 with `//nolint:gosec // chmod 0644 is correct for .pub files (G306)` |

---

## Open Questions

1. **Does `FixDescriptor.Fn` import filewriter directly, or are fix capabilities injected in `Deps`?**
   - What we know: D-01 says core never writes; the fixer lives in cmd layer; identity.Deps pattern uses injected functions.
   - What's unclear: Whether the planner prefers (a) FixDescriptor.Fn closed over real filewriter in cmd-layer buildDeps, or (b) fix capabilities in doctor.Deps struct.
   - Recommendation: Use approach (b) — add `FixPerm`, `RemoveBlock`, `AddWiring` function fields to `doctor.Deps` for the three auto-fix classes. This keeps the architecture consistent with `identity.Deps` and makes all fix paths fake-testable from the doctor package itself. The cmd layer populates these fields from real `filewriter`/`gitconfig`/`sshconfig` functions.

2. **Is Baseline its own report family or folded into Coherence/Orphans?**
   - What we know: D-06 lists "Dependencies, Permissions, Coherence, Orphans, Signing, Agent (+ Baseline, see D-16)"; CONTEXT says "or fold into Coherence/Orphans as the planner sees fit."
   - What's unclear: Whether a separate Baseline section provides enough user value.
   - Recommendation: Separate `Baseline` family. The four checks map naturally to two families (ignorecase drift + excludesfile wiring → Baseline; orphaned baseline include → Orphans), but grouping them together under Baseline makes the Phase 5 TUI dashboard design cleaner and maps directly to `gitid baseline show`.

3. **How should the `--yes` / `--fix` interaction be enforced?**
   - What we know: `--yes` without `--fix` should be rejected (or silently treated as `--fix --yes`). D-04 says `--yes` requires `--fix`.
   - Recommendation: In the cobra RunE, check `if yes && !fix { return fmt.Errorf("--yes requires --fix") }`. This matches shell convention and prevents silent no-op.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | ssh-add exit 1 = "agent running, no keys" on modern OpenSSH | Pattern 3 / ssh-agent | Minor: wrong classification; mitigated by also checking output text |
| A2 | `file.Stat().Mode() & os.ModeCharDevice != 0` is sufficient for TTY detection on darwin + linux | Pattern 5 | Low: worst case is color output in a pipe; mitigated by NO_COLOR fallback |
| A3 | Using approach (b) for FixDescriptor (fix capabilities in Deps struct) is the correct architecture | Open Question 1 | Planning: if planner chooses approach (a), the finding type shape changes slightly |
| A4 | gosec G306 will fire on `os.Chmod(path, 0o644)` for .pub fix | Security Domain | Low: if lint passes without annotation, no harm done |
| A5 | `account.PubPath` is set correctly for all identities from `Reconstruct` | Code Examples | Medium: if PubPath is empty for some identities, the agent fingerprint check must be guarded |
| A6 | Separate Baseline family (not folded into Coherence) is cleaner | Open Question 2 | Planning only: cosmetic impact |

---

## Sources

### Primary (HIGH confidence)
- `internal/deps/deps.go` — `Detect()`, `GitVersionAtLeast`, `MissingRequired()` signatures confirmed from live codebase
- `internal/platform/platform.go` — `InstallHint(os)` switch pattern confirmed; currently OpenSSH-only
- `internal/identity/loader.go` — `Reconstruct()` return type, `Incomplete` field, join-by-name logic
- `internal/filewriter/block.go` — `ListBlocks`, `RemoveBlock`, `ReplaceBlock` signatures
- `internal/gitconfig/reader.go` — `ReadFragment`, `ParseManagedIncludeIf`, `RemoveAllowedSignersLine` email match pattern
- `internal/gitconfig/baseline.go` — `ReadBaselineState`, `BaselineKeySet`, `DefaultGitignorePatterns` confirmed
- `internal/sshconfig/reader.go` — `ParseManagedHosts`, `SSHHostInfo.IdentitiesOnly` confirmed
- `internal/identity/identity.go` — `Account` struct fields (KeyPath, PubPath, FragmentPath, AllowedSignersPath, etc.)
- Local probe of `ssh-add -l` with/without agent: exit 0 (keys), exit 1 (no keys), exit 2 (unreachable) confirmed
- Local probe of `ssh-keygen -lf ~/.ssh/*.pub`: output format confirmed
- `go.mod` — no `golang.org/x/term` in direct deps; `golang.org/x/sys v0.46.0` is indirect dep

### Secondary (MEDIUM confidence)
- PITFALLS.md §Pitfall 6 — KEY-02 target permissions (700/600/644/600) confirmed as project-wide standard
- ARCHITECTURE.md §Component Responsibilities — doctor imports confirmed; `internal/doctor` → all except filewriter (per diagram note: "no writes")
- CONTEXT.md §D-02 through D-20 — all decisions locked; researched HOW not WHETHER

### Tertiary (LOW confidence / assumed)
- A2: `os.ModeCharDevice` for TTY detection — standard Go community pattern but not verified against official docs in this session
- A3: FixDescriptor injection architecture preference
- A4: gosec G306 annotation need for chmod 644

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified in live go.mod; no new external packages
- Architecture: HIGH — derived directly from existing Deps-injection pattern in identity.go; ssh-agent exit codes confirmed by live probe
- Pitfalls: HIGH — most pitfalls derived from existing codebase patterns and live probes; two LOW-confidence items flagged in Assumptions Log
- TDD test map: HIGH — test names and commands follow existing project patterns exactly

**Research date:** 2026-06-11
**Valid until:** 2026-07-11 (stable stdlib + project conventions; no fast-moving external deps)
