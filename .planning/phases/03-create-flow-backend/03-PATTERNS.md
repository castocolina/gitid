# Phase 3: Create Flow Backend - Pattern Map

**Mapped:** 2026-07-07
**Files analyzed:** 14 (grouped; several are multi-file archival/extraction sets)
**Analogs found:** 12 / 14

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|---------------|
| `internal/<shared-ui-pkg>/{theme,frame,wizard,ceremony}.go` (D-17 extraction target, name TBD) | component (TUI screen package) | request-response (keystroke → view) | `internal/dummytui/app.go` + `theme.go` + `identities.go` | exact (literal move/split of these files) |
| `internal/dummytui/*` (post-split, shrinks to fixtures + reducer) | component | request-response | itself (pre-split) | exact — same file, reduced |
| `internal/dummytui/nobackend_test.go` (restored) | test | import-graph/static-analysis | recovered `git show 7453561^:internal/dummytui/nobackend_test.go` | exact (verbatim recovery + allowlist edit) |
| `cmd/gitid/main.go` (rewire bare invocation) | controller (CLI entry) | request-response | itself, current `tui.Run()` call site; wiring pattern from `tui/deps.go` | role-match (same file, new callee) |
| `cmd/gitid/<new wiring file>.go` (e.g. `wiring.go`/`deps.go`) | service (Deps composition root) | CRUD (constructs Deps that read/write config) | `cmd/gitid/add.go`'s `buildDeps` (line 529) AND `tui/deps.go`'s `buildTUIDeps` (line 40) | exact — same composition-root pattern, new callee package |
| `cmd/gitid/{add,rotate,delete,doctor,adopt,copy,list,test,update,addrepo,upload,match,baseline}.go` (ARCHIVE, D-14) | controller (Cobra commands) | request-response | n/a — deletion/move to `.planning/archive/`, not a copy-pattern target | n/a (archival task) |
| `cmd/gitid/debug.go` (KEPT, D-14) | controller | request-response | itself — unchanged | exact (no-op) |
| `internal/identity/modes.go` `ensurePub` (KEY-06/D-11 fix) | service (pure function, no I/O side-effect beyond injected deps) | CRUD (read-existing-vs-derive branch) | itself — targeted in-place fix | exact (same file) |
| `internal/identity/modes_test.go` (new RED test for encrypted-key-with-existing-.pub) | test | CRUD | existing `modes_test.go` table-test style (not read this pass; follow existing `Reuse`/`AddAccount` test shapes in the same file) | role-match |
| `internal/keygen/<key-scan helper>.go` (new, D-10 picker backend) | service (filesystem scan + parse) | file-I/O | `internal/identity/inventory.go`'s `listKeyFilesReal` (glob `~/.ssh`) + `internal/keygen/derive.go`'s `DerivePublicKey` (parse via `ssh.ParsePrivateKey`) | role-match (compose both patterns into one new function set) |
| `internal/tester/<stage2 command-string helper>` (new, Pitfall 7 gap — e.g. `ResolvedViaCommand`) | utility (pure, no exec) | transform | `internal/tester/tester.go`'s `PreWriteCommand` (lines 61-70) | exact (same file, sibling function, same pattern) |
| `e2e/ui_pty_e2e_test.go` (extended: new per-screen wizard cases) | test (PTY e2e) | event-driven (keystroke injection) | itself — existing `startPTY`/`ptySession`/`sendKey`/`waitFor` harness (lines 68-254) | exact (same file, new test functions) |
| `e2e/create_e2e_test.go` (SUPERSEDED — POC CLI target, retire alongside D-14) | test | request-response | n/a — retirement, not a copy target | n/a |
| `Makefile` new `gate-visual-regression` (or similar, D-24) target | config/build-tooling | batch (diff two text captures) | `Makefile`'s `gate-no-backend-files` target (lines 245-253) — same shell-script-in-Makefile-target shape | role-match |
| `Makefile` `gate-no-backend-files` allowlist update (Pitfall 4) | config | batch | itself, in-place edit (line 247's grep regex) | exact |

## Pattern Assignments

### `internal/<shared-ui-pkg>/*.go` (D-17 extraction) — component, request-response

**Analog:** `internal/dummytui/app.go`, `internal/dummytui/theme.go`, `internal/dummytui/identities.go`

**Imports pattern** (`internal/dummytui/app.go` lines 16-21):
```go
import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)
```
`theme.go` additionally imports:
```go
import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
)
```

**Core screen-contract pattern to PRESERVE verbatim in the shared package** (`app.go` lines 24-66):
```go
type screenView struct {
	body       string
	crumbs     []string
	status     string
	statusTone string
	actions    []FooterAction
	capturesKeys bool
}

type keyResult struct {
	model   screenModel
	actions []Action
	cmd     tea.Cmd
	handled bool
	note    string
}

// screenModel is the contract every tab's child model implements. Handlers
// are pure over (model, state) so unit tests drive them without a
// terminal; reducer actions flow back to the App, the single Reduce caller.
type screenModel interface {
	handleKey(msg tea.KeyMsg, s DemoState) keyResult
	handleMsg(msg tea.Msg, s DemoState) keyResult
	view(s DemoState, width, height int) screenView
	activate(s DemoState) (screenModel, tea.Cmd)
}

type mouseTarget interface {
	handleClick(x, y, width, height int, s DemoState) keyResult
}
```
**What changes in the extraction:** replace `DemoState` with a generic/parameterized state the dummy populates from fixtures and the real binary populates from `identity.Deps`/`tester.Result` — this is the single central design decision of D-17 (Claude's Discretion). Keep method signatures shape-identical so `screenModel` implementations port with minimal churn.

**Theme role table** (`theme.go` lines 26-40) — move byte-identical; it is ANSI-16, glyph-plus-color, no truecolor — do not "improve" it during the move, it is a frozen contract (12-role table, 02-STYLE-SPEC.md).

---

### `internal/dummytui/nobackend_test.go` (restored, Pitfall 5) — test, import-graph

**Analog:** recovered prior source via `git show 7453561^:internal/dummytui/nobackend_test.go`

```go
package dummytui

import (
	"os/exec"
	"strings"
	"testing"
)

func TestNoBackendAllowlist(t *testing.T) {
	const modulePrefix = "github.com/castocolina/gitid/"
	allowed := map[string]bool{
		"github.com/castocolina/gitid/internal/dummytui": true,
		"github.com/castocolina/gitid/cmd/gitid-dummy":   true,
		// ADD the new D-17 shared package's import path here, e.g.:
		// "github.com/castocolina/gitid/internal/<shared-ui-pkg>": true,
	}
	cmd := exec.Command("go", "list", "-deps", "./cmd/gitid-dummy/...", "./internal/dummytui/...")
	cmd.Dir = repoRootForAllowlistTest(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}
	var offenders []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, modulePrefix) || allowed[line] {
			continue
		}
		offenders = append(offenders, line)
	}
	if len(offenders) > 0 {
		t.Fatalf("dummytui/gitid-dummy ALLOWLIST violated:\n%s", strings.Join(offenders, "\n"))
	}
}
```
**Critical:** restore with the allowlist map updated to include the NEW shared package's import path, in the SAME commit as the D-17 extraction — never restore verbatim against the stale two-member map (it will immediately fail once `internal/dummytui` imports the shared package).

---

### `cmd/gitid/main.go` + new wiring file — controller + service (Deps composition root)

**Analog A (composition-root pattern to mirror):** `cmd/gitid/add.go` `buildDeps` (line 529)
```go
func buildDeps(_ io.Writer) identity.Deps {
	return identity.Deps{
		Generate: func(in identity.CreateInput) (identity.StagedKey, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return identity.StagedKey{}, herr
			}
			sshDir := filepath.Join(home, ".ssh")
			if eerr := filewriter.EnsureDir(sshDir, 0o700); eerr != nil { //nolint:gosec // creating gitid-managed ~/.ssh dir (G301)
				return identity.StagedKey{}, fmt.Errorf("identity add: ensuring ~/.ssh exists: %w", eerr)
			}
			finalPriv, finalPub := keygen.KeyPaths(sshDir, in.Algo, in.Name)
			mat, gerr := keygen.GenerateMaterial(keygen.Params{
				Algo: in.Algo, Identity: in.Name, Comment: in.Name + "@gitid", Passphrase: in.Passphrase,
			})
			if gerr != nil {
				return identity.StagedKey{}, gerr
			}
			if _, werr := filewriter.Write(finalPriv, mat.PrivPEM, 0o600); werr != nil { //nolint:gosec // gitid-managed final path (G306)
				return identity.StagedKey{}, fmt.Errorf("identity add: writing private key to ~/.ssh: %w", werr)
			}
			// ... (continues: pub write, StagedKey assembly)
		},
		// ... PreWrite, WriteSSH, WriteGitconfig, etc. — every field non-nil
	}
}
```

**Analog B (multi-Deps composition-root pattern, tui-specific reference — VIEW CODE NOT REUSABLE, only wiring shape):** `tui/deps.go` `buildTUIDeps` (line 40)
```go
func buildTUIDeps() (doctor.Deps, identity.Deps, identity.UpdateDeps, identity.DeleteDeps, adopter.Deps, repoclone.Deps, uploader.Deps, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return doctor.Deps{}, identity.Deps{}, /* ... */, fmt.Errorf("tui: resolving home dir: %w", err)
	}
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")
	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // path is a trusted gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		return /* ... */, fmt.Errorf("tui: reading ssh config: %w", err)
	}
	// ... reads gitconfig similarly, then delegates to buildTUIDoctorDeps/buildIdentityDeps/etc.
	docDeps := buildTUIDoctorDeps(home, sshBytes, gcBytes)
	idDeps := buildIdentityDeps()
	// ...
	return docDeps, idDeps, upDeps, delDeps, adoptDeps, repoCloneDeps, uploaderDeps, nil
}
```

**How to apply:** the new `cmd/gitid/<wiring file>.go` should follow Analog B's SHAPE (a single `build*Deps()` composition root returning every Deps struct the shared UI package needs, non-nil per-field per the project's "injected-seam wiring blindspot" rule) but inject them into the NEW D-17 shared package's models — never re-import or extend `tui/`'s own view code (Common Pitfall 3). `main.go`'s bare-invocation branch swaps its call from `tui.Run()` to the new app's `Run()`/`Start()` entry point.

---

### `internal/identity/modes.go` `ensurePub` fix — service, CRUD (Pitfall 2 / D-11)

**Current code (the exact function to modify)** (lines 45-65):
```go
func ensurePub(privateKeyPath, pubPath, comment string, deps Deps) (string, error) {
	if deps.PubExists != nil && deps.PubExists(pubPath) {
		// .pub present: derive from the private key so the returned line is
		// guaranteed to match the key actually in use (the existing .pub may be
		// stale or for a different key).
		line, err := deps.DerivePub(privateKeyPath, comment)
		if err != nil {
			return "", fmt.Errorf("identity: deriving public key for reuse: %w", err)
		}
		return line, nil
	}

	line, err := deps.DerivePub(privateKeyPath, comment)
	if err != nil {
		return "", fmt.Errorf("identity: deriving missing public key for reuse: %w", err)
	}
	if werr := deps.WritePub(pubPath, line); werr != nil {
		return "", fmt.Errorf("identity: writing derived public key %s: %w", pubPath, werr)
	}
	return line, nil
}
```

**Required fix (D-11 — never re-derive when `.pub` already exists; read it directly instead):** change the `PubExists` branch to read the existing `.pub` file (a new `deps.ReadPub`-shaped seam, or `os.ReadFile` behind an injected dep matching the project's Deps-injection convention used elsewhere in this file) INSTEAD of calling `deps.DerivePub` — `DerivePub` internally calls `ssh.ParsePrivateKey`, which errors on an encrypted key with no passphrase (see `internal/keygen/derive.go` lines 26-39 below). Only call `DerivePub` in the ELSE branch (`.pub` absent), matching the doc comment's existing "only passphraseless keys are supported on this path" contract.

**Error-handling pattern to keep:** every branch wraps with `fmt.Errorf("identity: <verb> ...: %w", err)` — same prefix convention throughout this file (`Reuse`, `AddAccount`, `Rotate` all follow this).

---

### `internal/keygen/<key-scan helper>` (new, D-10 picker) — service, file-I/O

**Analog 1 (glob+enumerate pattern):** `internal/identity/inventory.go` `listKeyFilesReal` (lines 200-208+) — enumerates `~/.ssh` for `id_*` private-key files, excluding `.pub` siblings.

**Analog 2 (parse pattern):** `internal/keygen/derive.go` `DerivePublicKey` (lines 26-39):
```go
func DerivePublicKey(privateKeyPath, comment string) (string, error) {
	privBytes, err := os.ReadFile(privateKeyPath) //nolint:gosec // privateKeyPath is a gitid-managed path the user selected for reuse
	if err != nil {
		return "", fmt.Errorf("keygen: reading private key %s: %w", privateKeyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(privBytes)
	if err != nil {
		return "", fmt.Errorf("keygen: parsing private key %s: %w", privateKeyPath, err)
	}
	return pubLineWithComment(signer.PublicKey(), comment), nil
}
```

**How to apply:** compose these two patterns into a new function set in `internal/keygen` (lower-friction than a new package, per RESEARCH's Open Question 3 recommendation): glob `~/.ssh` like `listKeyFilesReal`, `ssh.ParsePrivateKey` each candidate like `DerivePublicKey`, and use `ssh.FingerprintSHA256(pub)` (already available via the existing `golang.org/x/crypto/ssh` import — no new dependency) for the fingerprint column. Follow the SAME `fmt.Errorf("keygen: ...: %w", err)` wrapping convention. For encrypted keys that fail to parse, do NOT error the whole scan — skip/flag that entry (needed for D-13's "any parseable key, informational note, never a block" rule) rather than aborting the picker list.

---

### `internal/tester/<stage2 command helper>` (new, Pitfall 7) — utility, transform

**Analog:** `internal/tester/tester.go` `PreWriteCommand` (lines 61-70):
```go
func PreWriteCommand(keyPath, hostname string, port int) string {
	args := preWriteArgs(keyPath, hostname, port)
	cmd := exec.Command("ssh", args...) //nolint:gosec // arg-slice form for cmd.String() display; not executed here
	return cmd.String()
}
```

**How to apply:** add a sibling pure function (e.g. `ResolvedViaCommand(configPath, keyPath, alias string) string`) built the SAME way — construct the exact arg slice `ResolvedVia` (lines 149-164) uses for its connectivity call, wrap in `exec.Command("ssh", args...)` WITHOUT executing, return `.String()`. This guarantees the on-screen "exact command" text (TEST-01 contract) is byte-identical to what `ResolvedVia` actually runs — never hand-build a display string independently (Pitfall 7's exact failure mode).

---

### `e2e/ui_pty_e2e_test.go` extension (DLV-06) — test, event-driven

**Analog:** existing harness in the same file — `ptySession` struct (lines 68-76), `startPTY` (line 96), `sendKey` (line 226), `snapshot`/`waitFor` (lines 234-254).

**Fake-ssh injection pattern to reuse** (from `e2e/harness_test.go` `FakeSSHDir`, lines 121-183, combined with the PTY harness):
```go
fakeSSHDir := FakeSSHDir(t, "denied") // → tester.ReachableNotUploaded
cmd := exec.Command(bin)
cmd.Env = append(os.Environ(),
	"HOME="+home,
	"GITID_FAKE_SSH_MODE=denied",
	"PATH="+fakeSSHDir+":"+os.Getenv("PATH"),
)
sess := startPTY(t, cmd)
```
**Modes available:** `pass` → `tester.PASS`, `denied` → `tester.ReachableNotUploaded` (D-02 warning state — NOT a failure, Pitfall 6), `timeout` → `tester.Failure`. New per-screen test cases should cover: algorithm+SSH form (SSHUI-01..03), both test stages with all three modes (TEST-01..03), the demo'd git-form screen (D-18/D-19 — assert the disabled-Continue copy, not functional Git write), and the confirm-write ceremony (TEST-03/D-05..D-09).

---

### `Makefile` gate targets — config, batch

**Analog:** `gate-no-backend-files` (lines 245-253):
```makefile
gate-no-backend-files:
	@BASE=$$(git merge-base main HEAD); \
	OFFENDING=$$(git diff --name-only "$$BASE"..HEAD | grep -v -E '^(\.planning/|internal/dummytui/|cmd/gitid-dummy/|internal/screenshot/|e2e/|Makefile$$|\.gitignore$$)' || true); \
	if [ -n "$$OFFENDING" ]; then \
		echo "gate-no-backend-files: FAILED -- file(s) outside the Phase 2 design-only allowlist changed since main ($$BASE):"; \
		echo "$$OFFENDING"; \
		exit 1; \
	fi; \
	echo "gate-no-backend-files: OK -- ..."
```
**How to apply for the new D-24 gate:** follow the same "single shell-script target with an explicit success/failure echo + non-zero exit on violation" shape. The D-24 gate instead diffs captured `View()` text (via `internal/screenshot`'s `CaptureTUI` mechanism) against approved goldens, honoring the per-screen divergence allowlist (D-02/D-16/D-19) as a second grep/allowlist file, not inline regex (the diff set is per-screen text, not file paths).
**Allowlist update needed in the SAME target:** add the new D-17 shared-package path into the regex on line 247 (Pitfall 4) so future design-only branches touching it still pass.

## Shared Patterns

### Deps-injection composition root (every backend field non-nil)
**Source:** `cmd/gitid/add.go` `buildDeps`, `tui/deps.go` `buildTUIDeps`
**Apply to:** the new `cmd/gitid` wiring file, and any new Deps struct additions (e.g. `internal/identity.Deps.ReadPub` if added for the `ensurePub` fix).
Every Deps field must be wired to a REAL implementation at the composition root — the project's documented "injected-seam wiring blindspot" is only closed when a PTY e2e exercises the real wiring end-to-end (per project memory), not just unit-tested in isolation.

### Error wrapping convention
**Source:** `internal/identity/modes.go`, `internal/keygen/derive.go`, `internal/tester/tester.go`
**Apply to:** all new/modified backend functions.
```go
return "", fmt.Errorf("<package>: <verb-phrase>: %w", err)
```
Package-name-prefixed, verb-phrase body, always `%w` wrap — never a bare `err.Error()` string.

### Never-shell-out-with-strings (gosec G204 discipline)
**Source:** `internal/tester/tester.go` (every `exec.Command` call site), `e2e/harness_test.go` `FakeSSHDir`'s static script literal
**Apply to:** any new exec-adjacent code (key-scan helper does NOT exec anything — pure parse; stage2 command helper explicitly does NOT execute, only builds display text).
Always arg-slice `exec.Command`, never `sh -c` string interpolation; static script literals for e2e fixtures, never built from user input.

### Output-substring classification, never exit code
**Source:** `internal/tester/tester.go` `ClassifyPreWrite` (lines 50-59)
**Apply to:** the shared UI package's render logic for the two test-stage screens — must call `tester.ClassifyPreWrite`/render `tester.Result.Outcome`, never re-derive PASS/Fail from a process exit code.

### `%w`-wrapped home-dir resolution guard
**Source:** `tui/deps.go` lines 41-44, `internal/keygen/derive.go` lines 26-30
**Apply to:** any new wiring code touching `~/.ssh` or `~/.gitconfig` paths — always check `os.UserHomeDir()` error before use, never swallow it.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/identity/modes_test.go` new encrypted-key RED test | test | CRUD | Not read this pass (existing test file's table shape should be followed directly at plan/implementation time — same file, straightforward extension, no cross-package analog needed) |
| D-24 golden-file storage/layout convention | config | batch | Genuinely new (Claude's Discretion per CONTEXT.md) — no existing golden-diff mechanism in this repo to copy from; `internal/screenshot/tui.go`'s `CaptureTUI` is the capture-side analog but the diff/storage convention itself is new |

## Metadata

**Analog search scope:** `internal/dummytui/`, `internal/identity/`, `internal/tester/`, `internal/keygen/`, `cmd/gitid/`, `tui/`, `e2e/`, `Makefile`
**Files scanned:** `internal/dummytui/app.go`, `theme.go`, `identities.go`, `data.go`; `internal/identity/modes.go`, `identity.go`, `inventory.go`; `internal/tester/tester.go`; `internal/keygen/derive.go`; `cmd/gitid/add.go`; `tui/deps.go`; `e2e/harness_test.go`, `e2e/ui_pty_e2e_test.go`; `Makefile`
**Pattern extraction date:** 2026-07-07
