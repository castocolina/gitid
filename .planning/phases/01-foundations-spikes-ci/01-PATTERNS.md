# Phase 1: Foundations, Spikes & CI - Pattern Map

**Mapped:** 2026-07-02
**Files analyzed:** ~16 new/modified files across 5 workstreams
**Analogs found:** 14 / 16 (2 have no direct repo analog — see "No Analog Found")

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/keygen/registry.go` (NEW) | service (algorithm registry) | transform (in-memory key generation) | `internal/keygen/keygen.go` `GenerateMaterial` | exact — same package, same function, being refactored in place |
| `internal/keygen/keygen.go` (MODIFIED — add `generateRSA4096`) | service | transform | `internal/keygen/keygen.go` `GenerateMaterial`/`pubLineWithComment` (self) | exact |
| `internal/platform/capabilities.go` (NEW) | service (injectable probe) | request-response (exec.Command → parse) | `internal/platform/platform.go` `ProbeKeyTypes`/`parseKeyTypes` | exact |
| `internal/platform/version.go` (NEW, or extend `platform.go`) | service | request-response | `internal/platform/platform.go` `ProbeKeyTypes`/`parseKeyTypes` | exact |
| `internal/sshconfig/include.go` (NEW) | service (safe-write orchestration) | CRUD (read-modify-write of managed block) | `internal/gitconfig/baseline.go` `PrependBlockIfNotFound`-based writer pattern (see below); primitive itself is `internal/filewriter/block.go` `PrependBlockIfNotFound` | exact — same primitive, same floor-placement intent, different target file |
| `internal/sshconfig/adopt.go` (NEW) | service (detection) | request-response (scan → classify) | `internal/adopter/adopter.go` `ListCandidates`/`MatchIdentityName` (PATTERN only, not code — see Anti-Pattern) | role-match (pattern, not code) |
| `internal/sshconfig/migrate.go` (NEW) | service (reversible transform) | CRUD (backup + atomic move) | `internal/adopter/adopter.go` `Adopt` (migrate/reference-in-place branch) — pattern only | role-match |
| `internal/identity/state.go` (NEW) | model/classifier (pure function) | transform (batch classification) | `internal/identity/loader.go` `Reconstruct` (assembles `[]Account` from managed blocks, computes `Incomplete`) | exact — same input shape (`Reconstruct` output), same "no sidecar DB, pure function" constraint |
| `internal/doctor/checks/orphans.go` (MODIFIED — add SSH-side reserved-block guard) | service (diagnostic check) | batch (Findings list) | `internal/doctor/checks/orphans.go` (self) `CheckOrphans` Class 2 gitconfig-side guard (`gitconfig.IsReservedBlockName`) | exact — mirror existing guard onto SSH side |
| `internal/sshconfig/` reserved-name guard (NEW, e.g. `IsReservedBlockName`) | utility | transform | `internal/gitconfig/reader.go` `IsReservedBlockName` (gitconfig package) | exact — direct sibling function to mirror |
| `internal/screenshot/tui.go` (NEW) | utility (dev/build tool) | file-I/O (text → PNG via subprocess) | no close in-repo analog; nearest shape is `internal/platform/platform.go` `ProbeKeyTypes` (exec.Command wrapper pattern) | partial |
| `internal/screenshot/html.go` (NEW) | utility (dev/build tool) | file-I/O (headless browser → PNG) | no close in-repo analog; nearest shape is `internal/tester/tester.go` `ResolvedVia` (exec/driver wrapper returning a result) | partial |
| `cmd/gitid/debug.go` (NEW — `gitid debug caps` / `keygen catalog`) | controller (Cobra command) | request-response | `cmd/gitid/doctor.go` `newDoctorCmd` | exact — thin-glue Cobra command calling into internal packages and printing |
| `Makefile` targets `screenshot-tui`/`screenshot-html` (NEW) | config | batch | `Makefile` existing `test`/`lint`/`build` targets | exact — same file, same target-composition convention |
| `.github/workflows/ci.yml` (NEW) | config (CI) | batch/orchestration | `Makefile` (`setup-env`, `fmt`, `lint`, `test`, `build`, `test-e2e` targets it must invoke) | role-match (no prior CI YAML in repo — first one) |
| `internal/sshconfig/include_test.go`, `adopt_test.go`, `migrate_test.go` (NEW tests) | test | CRUD/round-trip | `internal/sshconfig/marker_roundtrip_test.go`, `internal/sshconfig/coexistence_test.go` | exact |
| `internal/identity/state_test.go` (NEW test) | test | transform (table-driven) | `internal/identity/loader_test.go` / `internal/identity/modes_test.go` | exact |

## Pattern Assignments

### `internal/keygen/registry.go` + `internal/keygen/keygen.go` (service, transform)

**Analog:** `internal/keygen/keygen.go` (this repo, current state)

**Imports pattern** (keygen.go lines 1-12):
```go
package keygen

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)
```
For RSA, add `"crypto/rsa"`.

**Core pattern to replace** (keygen.go lines 44-53, the hard-coded dispatch to refactor into a registry):
```go
func GenerateMaterial(p Params) (Material, error) {
	if p.Algo != "ed25519" {
		return Material{}, fmt.Errorf("keygen: unsupported algorithm %q (only ed25519)", p.Algo)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	...
```
Target shape (per RESEARCH.md "Pattern 1"): `var registry = map[string]generatorFunc{"ed25519": generateEd25519, "rsa-4096": generateRSA4096, ...stub entries...}`, with `GenerateMaterial` doing `registry[p.Algo]` lookup and delegating. Extract the existing ed25519 body unchanged into `generateEd25519`.

**Serialization pattern to copy for RSA** (keygen.go lines 62-81 — mirror exactly, but note the pointer-vs-value pitfall):
```go
var block *pem.Block
if p.Passphrase != "" {
	block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, p.Comment, []byte(p.Passphrase))
} else {
	block, err = ssh.MarshalPrivateKey(priv, p.Comment)
}
...
privPEM := pem.EncodeToMemory(block)
sshPub, err := ssh.NewPublicKey(pub)
...
return Material{PrivPEM: privPEM, PubLine: pubLineWithComment(sshPub, p.Comment)}, nil
```
**CRITICAL DIFFERENCE for RSA:** `rsa.GenerateKey` returns `*rsa.PrivateKey` (a pointer) — pass it AS THE POINTER to `MarshalPrivateKey`/`ssh.NewPublicKey(&priv.PublicKey)`, unlike ed25519 which passes the value `priv` directly (documented in this file's own comment at line 60-61: "Pass the value from GenerateKey directly: value works for marshal at x/crypto v0.53.0"). This asymmetry is Pitfall 7 in RESEARCH.md.

**Error handling pattern** (consistent throughout keygen.go): every error is wrapped `fmt.Errorf("keygen: <action>: %w", err)` — follow this prefix convention for all new registry/RSA errors.

**Naming convention pattern** (keygen.go lines 99-105):
```go
func KeyPaths(dir, algo, identity string) (privPath, pubPath string) {
	privPath = filepath.Join(dir, fmt.Sprintf("id_%s_%s", algo, identity))
	return privPath, privPath + ".pub"
}
```
Already algo-parameterized — no change needed, but the registry must produce material for algos this function already supports path-wise.

---

### `internal/platform/capabilities.go` + `version.go` (service, request-response probe)

**Analog:** `internal/platform/platform.go` `ProbeKeyTypes`/`parseKeyTypes` (lines 47-76)

**Full pattern to mirror** (exec wrapper + pure parse split):
```go
func ProbeKeyTypes() ([]string, error) {
	out, err := exec.Command("ssh", "-Q", "key").Output() // #nosec G204 -- fixed args, no user input
	if err != nil {
		return nil, fmt.Errorf("probing ssh key types via `ssh -Q key`: %w", err)
	}
	return parseKeyTypes(string(out)), nil
}

func parseKeyTypes(out string) []string {
	lines := strings.Split(out, "\n")
	tokens := make([]string, 0, len(lines))
	for _, line := range lines {
		tok := strings.TrimSpace(line)
		if tok == "" {
			continue
		}
		tokens = append(tokens, tok)
	}
	return tokens
}
```
Apply this EXACT split (thin `exec.Command` I/O function + pure, independently-unit-testable parse function) for:
- `ProbeSSHVersion()` / `parseSSHVersion(out string) (opensshVersion, sslFlavor, sslVersion string)` — `ssh -V` writes to stderr, so use `CombinedOutput()` (per RESEARCH.md Pattern 2 example).
- `ProbeLibfido2()` / agent-running probe / macOS keychain probe — same shape, injectable via a `Deps`-style seam per CLAUDE.md's "injected-seam wiring blindspot" note.

**gosec annotation pattern** (line 56): `// #nosec G204 -- fixed args, no user input` — copy verbatim comment style for every new `exec.Command` call with fixed arg slices.

**FIDO2 token mapping note (Pitfall 2):** do NOT match `"ed25519-sk"` — the real `ssh -Q key` token is `sk-ssh-ed25519@openssh.com`. Build an explicit token→name map, do not string-contains match.

**Fallback-chain pattern already in file** (lines 10-30, 87-104) — `SelectAlgorithm` already demonstrates the "ordered candidates + membership test + install-hint-on-failure" pattern; the new catalog (KEY-01) should follow the same `algoCandidate`-style struct-slice shape, extended to 5 entries with per-OS availability metadata, not generate-vs-not-yet-implemented booleans hard-coded elsewhere.

**Install-hint reuse** (lines 106-216): `InstallHint(tool, os string) string` already exists with `"openssh"`/`"git"`/`"clipboard"` tool families — extend `normalizeTool` with a `"libfido2"` case rather than writing a parallel hint function.

---

### `internal/sshconfig/include.go` (service, CRUD floor-placement write)

**Analog:** `internal/filewriter/block.go` `PrependBlockIfNotFound` (the exact primitive, already shipping) + `internal/gitconfig/baseline.go` (the existing CALLER of that primitive for the analogous gitconfig `[include]` floor block)

**Primitive signature to reuse directly** (`internal/filewriter/block.go` lines 200-250):
```go
// PrependBlockIfNotFound returns existing with the gitid managed block for name
// set to blockBody, placing the block at the TOP of the file when no block for
// name currently exists (floor model — D-10) ... When a block for name already
// exists, the call delegates to ReplaceBlock so the block is updated in place
// and its floor position is preserved.
func PrependBlockIfNotFound(existing []byte, name, blockBody string) []byte
```

**Caller pattern to mirror** (from RESEARCH.md Pattern 3, itself derived from `internal/gitconfig/baseline.go`'s real usage of this primitive):
```go
const sshIncludeBlockName = "ssh-include" // reserved, non-identity block name

func EnsureIncludeLine(configPath string) (backupPath string, err error) {
	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", configPath, err)
	}
	composed := filewriter.PrependBlockIfNotFound(
		existing, sshIncludeBlockName, "Include ~/.ssh/config.d/*.config")
	if _, perr := Parse(composed); perr != nil {
		return "", fmt.Errorf("composed ssh config with Include line is not parseable: %w", perr)
	}
	return filewriter.Write(configPath, composed, configMode) // configMode = 0o600, existing const
}
```

**Reserved-block-name guard to mirror** — sibling function in the gitconfig package (`internal/gitconfig` — check `reader.go`'s `IsReservedBlockName`) must be duplicated on the sshconfig side:
```go
func IsReservedBlockName(name string) bool {
	return name == sshIncludeBlockName
}
```
**Why mandatory (Pitfall 4 / project memory "Doctor reserved-block false-positive loop"):** without this guard, `internal/doctor/checks/orphans.go` Class 1/2 cross-referencing will treat the `ssh-include` block as an orphaned identity block and offer to delete it, which then gets silently recreated next run — an infinite false-positive loop that already happened once for the gitconfig side and was fixed with exactly this guard pattern.

**Error handling pattern:** `fmt.Errorf("sshconfig: <action>: %w", err)` — follow the same per-package error-prefix convention as `internal/keygen` (`"keygen: ..."`) and other internal packages.

---

### `internal/sshconfig/adopt.go` + `migrate.go` (service, detection + reversible transform)

**Analog (PATTERN ONLY — do not import/reuse code):** `internal/adopter/adopter.go`

**Detect→adopt pattern to mirror the SHAPE of, not the code:**
```go
// Deps holds all external effects... every function field must be non-nil
type Deps struct {
	ReadFile func(path string) ([]byte, error)
	WriteFile func(path string, content []byte, mode os.FileMode) (backupPath string, err error)
	...
}

func Adopt(sourcePath, identityName, gitconfigPath string, method AdoptMethod, matches []gitconfig.Match, deps Deps) (AdoptResult, error) {
	...
	switch method {
	case AdoptMigrate:
		...
	case AdoptReferenceInPlace:
		...
	}
}
```
Mirror: injectable `Deps` struct with every external effect as a function field (filesystem reads, `filewriter`-chokepoint writes), an `AdoptMethod` enum (migrate vs reference-in-place), and an `AdoptResult` carrying `BackupPaths`/`MigratedPath`. For SSH's STORE-02/03, the equivalent shape is: detect existing `Include` directive in `~/.ssh/config` (scan for `Include` lines + gitid sentinels via `sshconfig.Parse`), then either adopt the discovered path or migrate blocks between in-file/Include'd layouts, each direction backed up via `filewriter.Write`.

**Symlink-guard pattern to reuse** (`adopter.go` lines 180-188):
```go
lstat, err := os.Lstat(sourcePath) //nolint:gosec // sourcePath is a gitid-derived candidate path returned by filepath.Glob (G304)
if err != nil && !os.IsNotExist(err) {
	return "", fmt.Errorf("adopter: stat candidate %s: %w", sourcePath, err)
}
if err == nil && lstat.Mode()&os.ModeSymlink != 0 {
	return "", fmt.Errorf("adopter: candidate %s is a symlink — adoption of symlinds is not supported (T-05.7-02-02)", sourcePath)
}
```
Apply the identical symlink-rejection guard to any user-confirmed external SSH Include path (path-traversal mitigation, per RESEARCH.md's Security Domain table).

**Why NOT to import `internal/adopter` directly (Anti-Pattern, confirmed by reading the file):** its `Deps.WriteIncludeIf` wires specifically to `gitconfig.WriteIncludeIf` and its `ListCandidates` globs `~/.gitconfig_*` — entirely gitconfig/`includeIf`-specific. STORE-02/03 is SSH `Include`-specific, a different directive/file/library surface (`kevinburke/ssh_config`, not `git config` shell-out). Build new, parallel code in `internal/sshconfig`.

---

### `internal/identity/state.go` (model/classifier, pure transform)

**Analog:** `internal/identity/loader.go` `Reconstruct` (lines 23-91)

**Pattern to build on top of (input shape + "no sidecar DB, pure function over managed blocks" constraint):**
```go
func Reconstruct(
	sshBytes []byte,
	gcBytes []byte,
	readFrag func(fragPath string) (gitconfig.FragmentInfo, error),
) ([]Account, error) {
	sshHosts, err := sshconfig.ParseManagedHosts(sshBytes)
	...
	gcBlocks := gitconfig.ParseManagedIncludeIf(gcBytes)
	names := nameUnion(sshHosts, gcBlocks)
	...
	for _, name := range names {
		acct := Account{Name: name}
		var missing []string
		if ssh, ok := sshHosts[name]; ok && ssh.Alias != "" {
			...
		} else {
			missing = append(missing, "ssh-host-block")
		}
		...
		acct.Incomplete = strings.Join(missing, ",")
		accounts = append(accounts, acct)
	}
	return accounts, nil
}
```
**How MGR-02's 8-state taxonomy extends this:** `internal/identity/state.go` should add a `ClassifyState(acct Account, keyExists, keyUsedInSSH, keyUsedInGit bool) State` pure function consuming `Reconstruct`'s `[]Account` output (already the "no sidecar DB" input) plus key-existence checks, mapping to the 8 locked states (complete / incomplete / git-only / key-unused / key-used-ssh-only / key-used-both / key-missing / fragment-path-missing). `acct.Incomplete` (the existing missing-pieces string) is a direct precursor signal for the `incomplete`/`git-only`/`fragment-path-missing` states.

**Shared-helper note (RESEARCH.md Open Question 2):** `internal/doctor/checks/orphans.go` Class 3 (unused-key detection, shown below) computes something close to `key-unused`. Per RESEARCH's own recommendation, extract a small shared `crossReferenceUnusedKeys` pure function living in `internal/identity` (not duplicated in `internal/doctor`), since `internal/doctor` already depends on identity-shaped data and the reverse import would risk the depguard rule (`internal/doctor` must never import `internal/filewriter`).

**Error handling / naming convention:** `fmt.Errorf("identity: <action>: %w", err)` — same per-package prefix convention.

**Table-driven test pattern to mirror:** `internal/identity/modes_test.go` / `internal/identity/loader_test.go` (Go stdlib `testing`, table-driven, no third-party framework, per RESEARCH.md's Test Framework section).

---

### `internal/doctor/checks/orphans.go` (MODIFIED — SSH-side reserved-block guard)

**Analog:** `internal/doctor/checks/orphans.go` (self) — the EXISTING gitconfig-side guard (lines 79-90) that must be mirrored onto the SSH side (Class 1, lines 49-77 currently lacks the equivalent check):
```go
for _, name := range deps.GitconfigManagedBlockNames {
	// Reserved non-identity wiring (e.g. baseline-include) has no SSH Host
	// block by design — it is NOT an orphan. Skip it, or its removal fix
	// would delete the legitimate baseline include and fight the Baseline
	// check's restore in an endless loop.
	if gitconfig.IsReservedBlockName(name) {
		continue
	}
	if !sshNames[name] {
		...
	}
}
```
**Required change:** add the mirror-image guard in the Class 1 loop (SSH block names, lines 51-77) using the new `sshconfig.IsReservedBlockName(name)` (see `include.go` pattern above) so the `ssh-include` block is skipped there too — this is the exact bug class documented in project memory ("Doctor reserved-block false-positive loop") and must ship in the SAME phase/task that introduces the reserved block, per Pitfall 4.

---

### `cmd/gitid/debug.go` (controller, request-response — D-08 surface)

**Analog:** `cmd/gitid/doctor.go` `newDoctorCmd` (lines 1-40+)

**Imports pattern** (doctor.go lines 1-24):
```go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/castocolina/gitid/internal/deps"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
)
```
For `debug.go`, the equivalent import set is `internal/keygen` (catalog), `internal/platform` (capability probe results), `internal/identity` (state taxonomy), plus `github.com/spf13/cobra`.

**Command construction pattern** (doctor.go lines ~30-40):
```go
func newDoctorCmd() *cobra.Command {
	var fix, yes bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run a health check on the gitid-managed environment",
		...
```
Mirror: `newDebugCmd()` (or `newKeygenCatalogCmd()`) returning a thin `*cobra.Command` whose `RunE` calls into `platform`/`keygen`/`identity` packages and prints — "no logic lives in the command itself" (RESEARCH.md's Architectural Responsibility Map). **Security note:** per RESEARCH.md's Security Domain table, this command must NEVER print `keygen.Material`/`PrivPEM` — only catalog metadata and public-key-derived info.

---

### `.github/workflows/ci.yml` (config, CI orchestration)

**Analog:** `Makefile` targets it must invoke as-is (no analog YAML exists — this is genuinely new)

**Makefile targets confirmed present** (must be called verbatim by CI, per CLAUDE.md's stated invariant "CI must call the SAME make targets a human runs locally"):
```
setup-env:
install-hooks:
fmt:
lint:
test:
build:
install:
uninstall:
```
CI job steps should be `make setup-env` → `make build` (cross-compile matrix) → `make test` (already includes `-race` per RESEARCH.md D-13/BUILD-02) → `make lint` → `make test-e2e` (add if not already a target — RESEARCH.md references it as expected to exist; verify before planning task-splits).

**CORRECTED runner matrix (per RESEARCH.md "State of the Art" + CONTEXT.md's own D-12 correction note):**
```yaml
strategy:
  matrix:
    include:
      - os: ubuntu-latest       # linux/amd64
      - os: macos-15-intel      # darwin/amd64 (Intel)
      - os: macos-15            # darwin/arm64 (Apple Silicon) — or macos-latest
```
Do NOT use `macos-13` (fully unsupported since Dec 2025) or `macos-14` (deprecation begins 2026-07-06). This is a HIGH-confidence, empirically-sourced correction — apply it verbatim.

---

### `internal/screenshot/tui.go` + `html.go` (utility, dev/build tooling)

**No strong in-repo analog** — see "No Analog Found" below for the exec-wrapper shape to imitate structurally (from `internal/platform/platform.go` and `internal/tester/tester.go`), plus RESEARCH.md's own verified `freeze` invocation:
```bash
go build -o /tmp/freezebin github.com/charmbracelet/freeze
/tmp/freezebin --execute "cat /tmp/sample.txt" -o /tmp/sample.png
```
Follow the arg-slice `exec.Command` convention (no shell) exactly as `internal/platform/platform.go` line 56 does, with the same `#nosec G204` annotation style, when invoking `freeze`/the headless-Chromium driver from Go code (if the make targets don't just shell out directly).

## Shared Patterns

### Safe-write chokepoint (STORE-04, applies to all `internal/sshconfig` new writers)
**Source:** `internal/filewriter/block.go` (`PrependBlockIfNotFound`, `ReplaceBlock`) + `internal/filewriter/filewriter.go` (`Write` — backup + atomic temp→rename→chmod)
**Apply to:** `internal/sshconfig/include.go`, `migrate.go`, and any other new SSH-config writer. NEVER call `os.WriteFile` directly — always route through `filewriter.Write`/`PrependBlockIfNotFound`/`ReplaceBlock`.

### Per-package error-wrapping convention
**Source:** every internal package read this session (`keygen.go`, `platform.go`, `adopter.go`, `loader.go`)
**Apply to:** all new files
```go
return nil, fmt.Errorf("<packagename>: <action>: %w", err)
```

### Injectable exec.Command probe seam (PLAT-01)
**Source:** `internal/platform/platform.go` `ProbeKeyTypes`/`parseKeyTypes` split
**Apply to:** `internal/platform/capabilities.go`, `version.go`, and any screenshot-tooling exec wrapper
```go
out, err := exec.Command("ssh", "-Q", "key").Output() // #nosec G204 -- fixed args, no user input
```
Always arg-slice form, never shell string interpolation; always split into a thin I/O function + a pure, independently-testable parse function.

### Reserved managed-block-name guard (recurring bug class — Pitfall 4)
**Source:** `internal/gitconfig` `IsReservedBlockName` + its consumer in `internal/doctor/checks/orphans.go` lines 82-88
**Apply to:** `internal/sshconfig` (new `IsReservedBlockName` for `ssh-include`) AND `internal/doctor/checks/orphans.go`'s Class 1 loop (must add the SSH-side skip in the SAME change).

### Cobra thin-glue command surface
**Source:** `cmd/gitid/doctor.go` `newDoctorCmd`
**Apply to:** `cmd/gitid/debug.go` — command layer only gathers input/prints output; all logic lives in `internal/*` packages.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/screenshot/tui.go` | utility | file-I/O (subprocess → PNG) | No prior screenshot/rendering code in this repo; nearest structural analog is the `exec.Command` wrapper shape in `internal/platform/platform.go`, but the domain (ANSI→PNG via `freeze`) is genuinely new. Use RESEARCH.md's "Code Examples" verified `freeze --execute` invocation and Pitfall 6 (`--font.file` determinism) as the reference instead. |
| `internal/screenshot/html.go` | utility | file-I/O (headless browser → PNG) | Same — no prior browser-automation code in this repo. Use RESEARCH.md's `go-rod/rod` Standard Stack entry and Anti-Patterns note (keep out of the shipped binary's dependency graph via build tag) as the reference. |
| `.github/workflows/ci.yml` | config | batch/orchestration | No prior CI YAML exists in this repo (`.github/workflows/` directory absent, confirmed by RESEARCH.md). Analog is the `Makefile` targets it must invoke, not a prior workflow file. |

## Metadata

**Analog search scope:** `internal/keygen`, `internal/platform`, `internal/sshconfig`, `internal/filewriter`, `internal/identity`, `internal/adopter`, `internal/gitconfig`, `internal/doctor/checks`, `internal/tester`, `cmd/gitid`, `Makefile`
**Files read directly:** `internal/keygen/keygen.go`, `internal/platform/platform.go`, `internal/filewriter/block.go`, `internal/adopter/adopter.go`, `internal/identity/loader.go`, `internal/identity/modes.go`, `internal/identity/identity.go`, `internal/gitconfig/baseline.go` (partial), `internal/doctor/checks/orphans.go` (partial), `cmd/gitid/doctor.go` (partial), `Makefile` (target names)
**Pattern extraction date:** 2026-07-02
</content>
