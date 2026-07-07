# Phase 3: Create Flow Backend - Research

**Researched:** 2026-07-07
**Domain:** Go Bubble Tea v2 TUI backend wiring (SSH/Git identity create flow) — package extraction, real-binary entry point, PTY e2e, visual-regression gate
**Confidence:** HIGH (this phase is overwhelmingly substrate-reuse: almost every backend capability already exists as tested Go code; the work is extraction, wiring, and gate mechanics, not new algorithm design)

## Summary

Phase 3 is smaller than its requirement count suggests, because Phases 1 and
the 0.0.1-era POC already built nearly all of the backend logic the create
flow needs: `internal/identity.Create/Reuse/AddAccount/PersistSSH/
PersistGitconfig/PersistAll`, `internal/tester` (two-stage PASS/
ReachableNotUploaded/Failure classification by output substring),
`internal/sshconfig.Adopt`/`DetectInclude` (Include-layout auto-detection),
`internal/keygen` (catalog + registry + `DerivePublicKey`), and
`internal/identity.DefaultHostname` (the known-provider table D-20/D-21
already asks for) are all real, tested, UI-free packages. **The actual new
work in Phase 3 is: (1) extracting a shared, backend-free presentation
package out of `internal/dummytui` so both `cmd/gitid` and
`cmd/gitid-dummy` render the identical frozen design (D-17); (2) building a
NEW real-binary entry point that wires that shared package to the
already-existing backend Deps (the pattern the now-superseded `tui/`
package and `cmd/gitid`'s POC Cobra commands already prove is possible);
(3) closing three small, load-bearing GAPS the substrate does NOT yet
cover — the `ReachableNotUploaded` render state, encrypted-key-with-existing-.pub
handling in `Reuse`, and PTY-level e2e for the new wizard; and (4) the D-24
visual-regression gate mechanics.**

The single most important correction to CONTEXT.md's own code-context
section: **D-05's SSH-layout auto-detect logic lives in
`internal/sshconfig.Adopt`/`DetectInclude` (STORE-01/02), NOT in
`internal/adopter`** — `internal/adopter` is the *gitconfig-fragment*
adopter (`~/.gitconfig_<name>` migration), a completely different package
with a similar name. Do not point Phase 3 tasks at `internal/adopter` for
SSH layout detection.

The second major finding: **`cmd/gitid/main.go`'s bare-invocation entry
point currently wires the OLD, pre-redesign `tui/` package** (13,631 lines,
its own `wizard.go`/`deps.go`/`model.go`), not `internal/dummytui`. This
`tui/` package predates the Phase 2 redesign entirely and renders a
DIFFERENT (non-approved) visual design. D-15 ("bare gitid opens the real
app shell rendering the approved chrome") is only satisfiable by REPLACING
`tui.Run()`'s wiring with the new D-17 shared package — the existing
`tui/` package cannot simply be pointed at new data; its own view code
renders the wrong design. `tui/deps.go`'s `buildTUIDeps()` function,
however, is a useful reference for how backend Deps (doctor, identity,
adopter, repoclone, uploader) are wired together at the cmd layer — the
wiring PATTERN transfers even though the view layer does not.

The third major finding: **D-22's PATH-shim fake-`ssh` harness already
exists** — `e2e/harness_test.go`'s `FakeSSHDir(t, mode)` (modes `pass` /
`denied` / `timeout`) is already used by `e2e/create_e2e_test.go` to drive
the OLD POC `identity add` Cobra command end-to-end via plain
`exec.Command`. D-22 does not require inventing a new fake-ssh mechanism —
it requires reusing `FakeSSHDir` from WITHIN the raw-keystroke PTY harness
(`e2e/ui_pty_e2e_test.go`) to drive the NEW wizard screens. `create_e2e_test.go`
itself targets the POC CLI archived by D-14 and will need to be superseded
by PTY-driven equivalents, not kept as-is.

A concrete, non-obvious pitfall for D-02: the dummy's own frozen
`test-fail` fixture text — `internal/dummytui/data.go`'s
`CreateFlowTestFailOutput = "git@ssh.github.com: Permission denied
(publickey)."` — is **exactly** the substring `tester.ClassifyPreWrite`
maps to `ReachableNotUploaded` (a non-blocking, D-02 WARNING state), not
`Failure`. The approved dummy's "failure" demo screen and the real
backend's "reachable, not yet uploaded" screen are, byte-for-byte, the
SAME test output. Getting this backwards (rendering the new warning state
with the OLD demo's failure styling, or vice versa) is the single easiest
way to violate the frozen glyph/color contract in this phase.

**Primary recommendation:** Do the D-17 extraction FIRST (it is the
critical path every other task depends on), wire the new real-binary entry
point second, then layer in the connectivity-test warning state, the
provider table (already built — just needs SSHUI-01 field wiring), the
storage auto-detect (already built), the key-reuse picker (mostly built —
needs the encrypted-key gap closed), and finally the two gates (DLV-04
golden-text diff, DLV-06 PTY e2e via the existing fake-ssh harness).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Algorithm catalog + SSH form + live Host preview | TUI presentation (shared pkg) | Backend (`internal/keygen`, `internal/identity`) | Rendering/focus/validation is presentation; the catalog data and hostname defaults come from `internal/keygen.Catalog()`/`internal/identity.DefaultHostname` |
| Two-stage connectivity test | Backend (`internal/tester`) | TUI presentation (progress/spinner, exact-output render) | Test execution + classification is pure backend logic (no UI import); the TUI only renders the `tester.Result` and drives `tea.Cmd` async sequencing |
| Temp-config staging for pre-write tests | Backend (`internal/identity.Deps.StageTestConfig`/`ResolvedVia`, `internal/tester.ResolvedVia`) | — | Already built (SSHUI-04); writes only to a throwaway path, never `~/.ssh/config` |
| Storage-layout auto-detect (in-file vs Include'd) | Backend (`internal/sshconfig.Adopt`/`DetectInclude`) | TUI presentation (confirm-write preview naming the resolved target) | Pure filesystem/text detection; the TUI only displays the resolved `TargetPath` |
| Confirm-write ceremony + backup | Backend (`internal/filewriter.Write`, `internal/identity.PersistSSH/PersistGitconfig`) | TUI presentation (ceremony beats/copy) | Atomic backup+write+chmod is backend; ceremony sequencing/copy is presentation |
| Key-reuse picker (scan `~/.ssh`, fingerprint, in-use warning) | Backend (new: key-scan helper over `internal/keygen`/`x/crypto/ssh`) | TUI presentation (list render, manual-path row) | Parsing/fingerprinting keys is backend logic; NOT yet built — see Don't Hand-Roll |
| Provider inference from SSH Host suffix | Backend (`internal/identity.DefaultHostname`, already built) | TUI presentation (auto-fill on keystroke) | Pure string-table lookup; TUI calls it reactively as the user edits the Host field |
| macOS `Host *` globals block | Backend (`internal/sshconfig.RenderGlobalBlock`, `internal/platform.CurrentOS`) | TUI presentation (preview on first create) | Already built and wired in `cmd/gitid/add.go`'s `buildDeps` |
| Shared presentation package (theme/frame/screen views) | TUI presentation (new extracted package) | — | The D-17 extraction target; must stay backend-free (no `internal/identity` etc. imports) |
| Real-binary Deps wiring (cmd/gitid entry point) | CLI/App wiring layer (new `cmd/gitid` main + a small `app`/`wiring` file) | Backend (every internal package) | Thin composition root — gathers real constructors, injects into the shared presentation package's screen models |
| PTY e2e / fake-ssh harness | Test infrastructure (`e2e/`) | — | `e2e/harness_test.go`'s `FakeSSHDir` already exists; extend PTY tests to use it |
| Visual-regression gate (golden text diff) | Build tooling (`Makefile`, `internal/screenshot`) | TUI presentation (View() dump source) | New `make` target diffing captured `View()` text against approved goldens |

## Standard Stack

### Core (all already in go.mod — no new installs)

| Library | Version (go.mod, verified) | Purpose | Why Standard |
|---------|---------|---------|--------------|
| charm.land/bubbletea/v2 | v2.0.7 [VERIFIED: go.mod] | TUI event loop | Already the project's pinned TUI framework (CLAUDE.md, `internal/dummytui`) |
| charm.land/lipgloss/v2 | v2.0.3 [VERIFIED: go.mod] | Styling | Theme/role system already built in `internal/dummytui/theme.go` |
| charm.land/bubbles/v2 | v2.1.0 [VERIFIED: go.mod] | `textinput` etc. | Already used by `sshForm`/`gitForm` field inputs |
| golang.org/x/crypto | v0.53.0 [VERIFIED: go.mod] | Ed25519/RSA key parse (`ssh.ParsePrivateKey`), fingerprinting | Already used by `internal/keygen.DerivePublicKey`; the key-reuse picker's parse+fingerprint step reuses this, not a new dependency |
| github.com/kevinburke/ssh_config | v1.6.0 [VERIFIED: go.mod] | SSH config parse/render | Already wrapped by `internal/sshconfig` |
| github.com/spf13/cobra | v1.10.2 [VERIFIED: go.mod] | CLI framework | `debug caps` (kept, D-14) still uses it; the create-flow itself is TUI-only in Phase 3 |
| github.com/atotto/clipboard | v0.1.4 [VERIFIED: go.mod] | Clipboard copy | Already wired via `internal/clipboard.Copy`, reused for D-03 |
| github.com/creack/pty | v1.1.24 [VERIFIED: go.mod] | PTY e2e harness | Already used by `e2e/ui_pty_e2e_test.go` |
| github.com/charmbracelet/x/vt | (pinned, go.mod) [VERIFIED: go.mod] | VT100 emulation for PTY e2e frame decode | Already used by `e2e/ui_pty_e2e_test.go` |
| github.com/charmbracelet/freeze (dev tool, not a go.mod runtime dep) | v0.2.2 [VERIFIED: Makefile `FREEZE_VERSION`] | ANSI→PNG capture for the D-24 screenshot pipeline | Already wired via `make screenshot-tui`; build-tag isolated (`internal/screenshot/tui.go`) |

**No new external packages are required for Phase 3.** Every capability
(key parsing/fingerprinting, clipboard, PTY driving, SSH config
parsing) is already available through an existing go.mod dependency or an
existing internal package. Confirm this holds at plan time by re-running
`go list -m all` if any task appears to need something not in the table
above — that is a signal the task is scoped wrong, not that a new
dependency is needed.

### Supporting (existing internal packages Phase 3 wires, does not rebuild)

| Package | Purpose | Status |
|---------|---------|--------|
| `internal/identity` | `Create`/`Reuse`/`AddAccount`, `PersistSSH`/`PersistGitconfig`/`PersistAll`, `CreateInput`/`Deps`, `DefaultHostname`/`DefaultPort`/`DefaultAlias`/`DefaultMatch`, `ClassifyState`/`Classify` (MGR-02 taxonomy) | Fully built; Phase 3 wires it into the new TUI, does not modify its contract (a small addition may be needed — see Common Pitfalls #3) |
| `internal/tester` | `PreWrite`/`Resolved`/`ResolvedVia`, `ClassifyPreWrite` (PASS/ReachableNotUploaded/Failure), `ParseResolved` | Fully built; Phase 3 only renders `Result`/`ResolvedConfig` |
| `internal/sshconfig` | `Adopt`/`DetectInclude` (STORE-01/02 auto-detect), `EnsureIncludeDir`/`EnsureIncludeLine` (D-06 first-create dual-file), `RenderHostBlock`/`RenderGlobalBlock`, `Write` | Fully built |
| `internal/filewriter` | `Write`/`WriteNoBackup`/`BackupAndRemove`, `ListBlocks`/`ReplaceBlock`/`PrependBlockIfNotFound` (sentinel blocks) | Fully built; the atomic-write/backup invariant Phase 3 must never bypass |
| `internal/keygen` | `Catalog()`/`ResolveAvailability` (KEY-01 top-5), `registry.go` (ed25519/rsa-4096 real, 3 stubs), `DerivePublicKey` (KEY-06/D-11) | Fully built |
| `internal/platform` | `ProbeKeyTypes`, `SelectAlgorithm`, `CurrentOS` | Fully built |
| `internal/clipboard` | `Copy` (D-03) | Fully built |
| `internal/dummytui` | Approved design source (theme, frame, wizard, ceremony, data fixtures) — the D-17 EXTRACTION SOURCE | Fully built as a demo; Phase 3 splits it, does not rewrite its visuals |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Extending `internal/dummytui` in place with `if realBackend` branches | A new shared package imported by both binaries (D-17, chosen) | In-place branching would violate "dummy stays provably backend-free" and re-couple the two binaries; explicitly rejected by D-17 |
| Reusing the OLD `tui/` package's wizard views, only swapping its Deps | Building the new entry point against the D-17 shared package | The old `tui/` package's visual design predates the Phase 2 redesign and does not match the approved goldens — reusing its VIEWS would fail the D-24 gate outright. Its Deps-WIRING PATTERN (`buildTUIDeps`) is still a useful reference. |
| A new Go mock SSH client for PTY e2e | The existing `FakeSSHDir` PATH-shim (D-22, chosen) | D-22 explicitly requires a real external-process swap, not an in-Go mock — a Go mock would defeat DLV-06's purpose; `FakeSSHDir` already satisfies this and already exists |
| Inventing a new provider→alt-SSH table for SSHUI-01 | `internal/identity.DefaultHostname` (already built) | Already implements exactly github→ssh.github.com, gitlab→altssh.gitlab.com, bitbucket→altssh.bitbucket.org, port 443, with the unknown-provider fallback D-21 describes |

**Installation:** none required (`go build ./...` with existing go.mod).

**Version verification:** all versions above were read directly from this
repo's `go.mod` and `Makefile` (not training-data recall) — see the
`[VERIFIED: go.mod]` tags. No `npm view`/`pip index` equivalent applies
(Go modules); `go list -m all` is the equivalent verification command if
a task appears to need a package not already present.

## Package Legitimacy Audit

**Not applicable — Phase 3 introduces zero new external dependencies.**
Every capability required (key parsing, PTY driving, VT100 emulation,
clipboard, SSH config parsing, screenshot capture) is already present in
`go.mod` or wired as a build-tag-isolated dev tool in the `Makefile`. The
slopcheck/registry-verification protocol does not apply because there is
nothing to install. If, during planning or execution, a task is found to
need a package not listed in the Standard Stack table above, that is a
signal to re-scope the task against existing substrate before reaching
for a new dependency — re-run this audit at that point.

## Architecture Patterns

### System Architecture Diagram

```
                     ┌─────────────────────────────────────────┐
                     │   cmd/gitid  (real binary, D-15)         │
                     │   main.go: bare invocation → app.Run()   │
                     └───────────────┬───────────────────────────┘
                                     │ injects REAL Deps
                                     ▼
   ┌───────────────────────────────────────────────────────────────────┐
   │        NEW shared presentation package (D-17 extraction target)    │
   │  (theme.go, frame.go, screen views split out of internal/dummytui) │
   │                                                                     │
   │  screenModel interface: handleKey / handleMsg / view / activate    │
   │  wizardModel: step0 catalog+SSH form → step1 test → step2 git      │
   │               (demo'd, D-18/D-19) → step3 review ceremony          │
   └───────┬───────────────────────────────────────┬─────────────────────┘
           │ dummy injects fixtures                 │ real injects backend state
           ▼                                        ▼
 ┌───────────────────────┐              ┌─────────────────────────────────┐
 │ cmd/gitid-dummy        │              │  cmd/gitid (real, THIS PHASE)    │
 │ internal/dummytui       │              │  new wiring file(s)              │
 │ (unchanged behavior,    │              │  (pattern like tui/deps.go's     │
 │  fixtures only)         │              │   buildTUIDeps, but pointed at   │
 └───────────────────────┘              │   the NEW shared package)         │
                                          └───────────┬───────────────────────┘
                                                       │ calls real Deps
                        ┌──────────────────────────────┼───────────────────────────┐
                        ▼                              ▼                           ▼
              internal/keygen                internal/tester              internal/sshconfig
              (Catalog, DerivePublicKey)      (PreWrite, Resolved,         (Adopt, DetectInclude,
                                               ResolvedVia, Classify-       EnsureIncludeLine,
                                               PreWrite)                    RenderHostBlock)
                        │                              │                           │
                        └──────────────┬───────────────┴─────────────┬─────────────┘
                                       ▼                              ▼
                             internal/identity                internal/filewriter
                             (Create/Reuse/AddAccount,         (Write: backup+atomic+
                              PersistSSH/PersistGitconfig,      chmod, sentinel blocks)
                              DefaultHostname/Port/Alias/Match)

  Test/verification path (parallel, not in the render graph):
  e2e/ui_pty_e2e_test.go ──drives real keystrokes──▶ cmd/gitid binary
        │                                                    │
        └── PATH-shim: e2e/harness_test.go's FakeSSHDir ─────┘
            (swaps the `ssh` binary the real process execs;
             modes pass/denied/timeout ↔ PASS/ReachableNotUploaded/Failure)

  Visual-regression path (parallel):
  captured View() text (real binary) ──diff──▶ approved dummy goldens
                                          │
                              per-screen allowlist (D-02/D-16/D-19)
                                          │
                         agent-ui-ux-designer + Codex review (D-24.2)
```

### Recommended Project Structure

```
internal/
├── <new-shared-ui-package>/    # D-17 extraction target — Claude's discretion on name
│   ├── theme.go                # moved from internal/dummytui (byte-identical)
│   ├── frame.go                # moved from internal/dummytui (shell chrome)
│   ├── wizard.go / identities.go-equivalent   # create-flow screen views, parameterized
│   │                             over an injected data/backend seam instead of DemoState
│   ├── ceremony.go             # confirm-write ceremony machinery (moved)
│   └── doc.go                  # documents the backend-free contract + allowlist test
├── dummytui/                    # SHRINKS: keeps only fixture data (data.go) + the
│   │                             dummy-specific reducer (store.go) + import of the
│   │                             new shared package for rendering
│   └── nobackend_test.go       # RESTORED (see Common Pitfalls #1), updated allowlist
├── identity/                    # unchanged — substrate
├── tester/                      # unchanged — substrate
├── sshconfig/                   # unchanged — substrate
├── keygen/                      # unchanged — substrate
└── ...

cmd/
├── gitid/
│   ├── main.go                 # CHANGED: bare invocation wires the NEW app, not tui.Run()
│   ├── debug.go                # KEPT (D-14)
│   ├── (add.go, rotate.go, delete.go, doctor.go, adopt.go, copy.go,
│   │   list.go, test.go, update.go, addrepo.go, upload.go, match.go,
│   │   baseline.go)             # ARCHIVED (D-14) — moved to .planning/archive/
│   └── <new wiring file>       # NEW: real Deps construction (identity.Deps,
│                                 tester wiring, sshconfig.Adopt call, etc.),
│                                 modeled on tui/deps.go's buildTUIDeps pattern
└── gitid-dummy/                 # unchanged entry point, now imports the shared package
                                  # transitively through internal/dummytui

tui/                              # ARCHIVE CANDIDATE (pre-redesign POC TUI) — its
                                  # views cannot pass the D-24 gate; buildTUIDeps is a
                                  # useful reference only, not a reuse target

e2e/
├── harness_test.go              # UNCHANGED — FakeSSHDir already exists (D-22)
├── ui_pty_e2e_test.go            # EXTENDED: new per-screen PTY tests for the wizard,
│                                  reusing FakeSSHDir for the connectivity-test screens
└── create_e2e_test.go            # SUPERSEDED by PTY equivalents once the POC
                                   `identity add` command is archived (D-14)
```

### Pattern 1: screenModel interface reuse across two binaries (D-17)

**What:** `internal/dummytui`'s `screenModel` interface
(`handleKey`/`handleMsg`/`view`/`activate`) and `mouseTarget` interface
(`handleClick`) are already binary-agnostic in shape — they take a state
value and return a `keyResult`. The extraction should keep this contract
identical in the shared package, changing only WHAT populates the state
(dummy: `DemoState` seeded from fixtures; real: a state struct backed by
`identity.Deps` calls).

**When to use:** For every screen the create-flow wizard touches
(algorithm+SSH form, test stages, git form (demo'd), review ceremony).

**Example (existing contract to preserve, from `internal/dummytui/app.go`):**
```go
// Source: internal/dummytui/app.go (existing, read this session)
type screenModel interface {
    handleKey(msg tea.KeyMsg, s DemoState) keyResult
    handleMsg(msg tea.Msg, s DemoState) keyResult
    view(s DemoState, width, height int) screenView
    activate(s DemoState) (screenModel, tea.Cmd)
}
```
The extraction's central design decision (Claude's Discretion, D-17) is
what replaces `DemoState` in the shared package's generic form — likely a
narrower interface or a parameterized state struct the dummy populates
with fixtures and the real binary populates with `identity.Reconstruct`/
`BuildInventory` results plus live `tester.Result`s.

### Pattern 2: Deps-construction composition root (real binary wiring)

**What:** `cmd/gitid/add.go`'s `buildDeps()` and `tui/deps.go`'s
`buildTUIDeps()` are both existing, working examples of the "thin
composition root" pattern CLAUDE.md requires: gather real constructors
from internal packages, assemble them into a `Deps` struct, inject into
the (UI-free) orchestration function. Phase 3's new real-binary wiring
file should follow this SAME pattern, just pointed at the new shared
presentation package instead of `tui/`'s old models.

**When to use:** The single new wiring file cmd/gitid needs for D-15.

**Example (existing pattern to mirror, from `cmd/gitid/add.go`):**
```go
// Source: cmd/gitid/add.go buildDeps (existing, read this session)
func buildDeps(_ io.Writer) identity.Deps {
    return identity.Deps{
        Generate:  /* real key generation via internal/keygen */,
        PreWrite:  func(keyPath, hostname string, port int) tester.Result {
            return tester.PreWrite(keyPath, hostname, port)
        },
        WriteSSH:  /* real internal/sshconfig.Write call */,
        // ... etc, every field non-nil (the project's documented
        // injected-seam wiring blindspot rule)
    }
}
```

### Pattern 3: Temp-config staged testing (already built, SSHUI-04)

**What:** `internal/identity.Deps.StageTestConfig`/`ResolvedVia` and
`internal/tester.ResolvedVia` already implement exactly what SSHUI-04 and
the D-04 stage-1→stage-2 auto-chain need: render the Host block to a
throwaway temp config, then run `ssh -F <tmp> -i <key> -T git@<alias>` and
`ssh -F <tmp> -G <alias>` against it — never touching `~/.ssh/config`
until the confirmed write.

**When to use:** Both test stages in the wizard's test screen(s).

### Anti-Patterns to Avoid

- **Re-deriving the provider→alt-SSH table inside the TUI layer:** it
  already exists as `internal/identity.DefaultHostname` — a second,
  TUI-local copy would drift (exactly the class of duplication the
  project's own `configDirGlob`/`sshIncludeLineBody` comments warn about
  elsewhere, though there mirroring was an ACCEPTED tradeoff for wave
  independence — this is not that case; there is no DAG-independence
  reason to duplicate `DefaultHostname`).
- **Treating "Permission denied (publickey)" as Failure:** it is
  `ReachableNotUploaded` (a PASS-adjacent, store-gate-unlocking state,
  D-01/D-02) — never a hard-stop `Failure`. Only genuinely-refused/
  timed-out/DNS-failed connections are `Failure`.
- **Reusing the OLD `tui/` package's wizard views as a starting point:**
  they render a pre-redesign, non-approved layout; adapting them would
  fail the D-24 visual-regression gate. Extract from `internal/dummytui`
  instead.
- **Calling `internal/adopter` for SSH layout detection:** that package
  is the gitconfig-fragment adopter (`~/.gitconfig_<name>`), unrelated to
  D-05's SSH Include-layout auto-detect (`internal/sshconfig.Adopt`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Two-stage SSH connectivity classification | A new pass/fail parser | `internal/tester.ClassifyPreWrite`/`ParseResolved` | Already handles the exit-code-unreliability pitfall and the three-way PASS/ReachableNotUploaded/Failure split by output substring |
| SSH config Include-layout detection | A new glob+parse routine | `internal/sshconfig.DetectInclude`/`Adopt` | Already handles quoted paths, multi-token Include lines, symlink rejection, and the sentinel-bearing-vs-caller-chosen selection rules |
| Atomic config writes with backup | A new temp-file+rename routine | `internal/filewriter.Write`/`WriteNoBackup` | Already handles collision-proof backup naming, fsync-before-rename, explicit chmod (never relies on umask) |
| Fake-ssh e2e test double | A new Go SSH mock or a new PATH-shim script | `e2e/harness_test.go.FakeSSHDir` | Already implements the exact D-22 contract (external-process swap, three modes) and is already exercised by an existing e2e test |
| Key algorithm catalog + top-5 ordering | A new catalog data structure | `internal/keygen.Catalog()`/`ResolveAvailability` | Already resolves Implemented vs Available orthogonally (KEY-01/KEY-02/PLAT-01) |
| Public-key derivation from a private key | Hand-rolled `x/crypto/ssh` parsing | `internal/keygen.DerivePublicKey` | Already reproduces the exact `<type> <base64> <comment>` line format `GenerateMaterial` emits |

**Key insight:** because the internal packages were built test-first ahead
of any UI (DLV-07), the safest posture for Phase 3 planning is "assume the
capability already exists as a UI-free function; only write new backend
code for the 2-3 genuinely uncovered gaps below (Common Pitfalls #2/#3),
and spend the actual planning effort on the extraction/wiring/gate work."

## Common Pitfalls

### Pitfall 1: The `internal/adopter` vs `internal/sshconfig.Adopt` naming trap

**What goes wrong:** A plan task references "the adopter package" for
D-05 SSH-layout auto-detection and gets pointed at `internal/adopter`,
which has an `Adopt` function too — but for gitconfig fragments, not SSH
config layout.
**Why it happens:** Both packages export an `Adopt`/`AdoptMethod`/
`AdoptResult` shape (deliberate parallel design, per `adopter.go`'s own
doc comment referencing "the sibling gitconfig-fragment adopter"), and
CONTEXT.md's own code-context section makes this same conflation.
**How to avoid:** For SSH Include-layout detection (STORE-01/02/D-05), use
`internal/sshconfig.DetectInclude`/`Adopt`/`AdoptDeps`/`RealAdoptDeps`.
Reserve `internal/adopter` for gitconfig fragment adoption (out of scope
for Phase 3 — that is Phase 4's GITUI-01..05 territory, though the
package already exists as substrate).
**Warning signs:** A task or PR diff touching `internal/adopter` for
anything SSH-config-shaped.

### Pitfall 2: `identity.Reuse`'s `ensurePub` does not yet satisfy D-11's encrypted-key acceptance

**What goes wrong:** D-11 requires "Encrypted keys accepted when a
matching `.pub` exists alongside (no passphrase prompt)." The current
`internal/identity/modes.go` `ensurePub` helper ALWAYS calls
`deps.DerivePub` (which parses the private key via
`ssh.ParsePrivateKey`) — even when the `.pub` already exists on disk. An
encrypted private key will fail to parse in `DerivePublicKey`
(`ssh.ParsePrivateKey` errors on an encrypted key with no passphrase),
so `Reuse` will currently error out on exactly the case D-11 says must be
accepted.
**Why it happens:** `ensurePub` was built before D-11's exact rule was
locked; it optimizes for "always re-derive from the source of truth" (the
private key), which is safe for unencrypted keys but breaks the
encrypted-with-existing-.pub case.
**How to avoid:** Phase 3 needs a small, targeted change: when
`deps.PubExists(pubPath)` is true, read the EXISTING `.pub` file directly
(no `DerivePub` call, no private-key parse) rather than re-deriving it.
Only call `DerivePub` when the `.pub` is ABSENT (the case that genuinely
needs a fresh derivation, and that already assumes an unencrypted key per
the `DerivePublicKey` doc comment: "Only passphraseless keys are supported
on this path"). This is a real, scoped code change to `internal/identity`
(`ensurePub` in `modes.go`), not just TUI wiring — flag it explicitly as a
task, not an assumption that the picker's UI layer alone can satisfy D-11.
**Warning signs:** An e2e/unit test that reuses an encrypted key with an
existing `.pub` sibling fails with a "parsing private key" error instead
of succeeding.

### Pitfall 3: `tui/` (old POC TUI) vs `internal/dummytui` (approved design) confusion

**What goes wrong:** `cmd/gitid/main.go` currently calls `tui.Run()` for
bare invocation, which builds `tui/`'s OWN root model — a
13,631-line package with its own `wizard.go`, `deps.go`, `model.go`
predating the Phase 2 redesign. This is NOT the approved design; its
visual output will not match the D-24 goldens.
**Why it happens:** The repository's history includes an earlier TUI
attempt (the `tui/` package) built before the redesign (`.planning/
archive/0.0.1-poc-product-features-in-tui/` era or an early v1.0
attempt); `main.go` was never updated to point at the new design.
**How to avoid:** D-15's real-binary entry point MUST be wired against
the NEW D-17 shared package (extracted from `internal/dummytui`), not
`tui/`. Treat `tui/deps.go`'s Deps-construction functions
(`buildTUIDeps`, `buildIdentityDeps`, `buildTUIDoctorDeps`, etc.) as a
useful WIRING-PATTERN reference only — do not import or extend the `tui/`
package's view code. Decide explicitly (a planner/discuss-phase question,
not a silent assumption) whether `tui/` is archived alongside the POC
Cobra commands in this phase or left dormant/removed in a later cleanup —
either way it must stop being `main.go`'s entry point.
**Warning signs:** `go list -deps ./cmd/gitid/...` still showing
`github.com/castocolina/gitid/tui` after Phase 3 claims D-15 is done; a
D-24 visual diff showing the OLD design's layout instead of the approved
one.

### Pitfall 4: `gate-no-backend-files`'s allowlist will reject the D-17 extraction's new package path

**What goes wrong:** The existing `gate-no-backend-files` Makefile target
allowlists exactly `.planning/`, `internal/dummytui/`,
`cmd/gitid-dummy/`, `internal/screenshot/`, `e2e/`, `Makefile`,
`.gitignore`. A new shared-UI package directory (whatever name D-17
picks) is NOT in that list, and this gate's own doc comment says it is
meant to run "directly on design-only branches" — Phase 3 is explicitly
NOT a design-only branch, so this specific gate likely does not run as a
CI-blocking check on Phase 3's branch. Still, D-17 explicitly calls out
updating "the allowlist for the new package split" — meaning the intent
is for the new shared package to remain reachable by this gate on FUTURE
design-only branches (e.g., if Phase 4-9's own dummy screens need
design-only iteration before their own backend phases).
**Why it happens:** The gate was written against Phase 2's two-binary
world (`internal/dummytui` + `cmd/gitid-dummy` only); D-17 introduces a
third backend-free location.
**How to avoid:** Add the new shared package's path to
`gate-no-backend-files`'s regex allowlist in the SAME commit that creates
it, and add/restore the import-graph allowlist test (Pitfall 5) covering
the shared package + `internal/dummytui` + `cmd/gitid-dummy` (NOT
`cmd/gitid`, which legitimately imports backend packages).
**Warning signs:** A future design-only branch failing
`gate-no-backend-files` for touching the new shared package.

### Pitfall 5: The deleted `nobackend_test.go` must be restored against the NEW three-member allowlist, not the old two-member one

**What goes wrong:** STATE.md's W2 blocker flags that
`internal/dummytui/nobackend_test.go` (a `go list -deps` allowlist test)
was deleted in commit `7453561` and never restored. Its ORIGINAL
allowlist was exactly `{internal/dummytui, cmd/gitid-dummy}`. After the
D-17 extraction, restoring it VERBATIM would immediately fail, because
`internal/dummytui` will legitimately import the new shared package.
**Why it happens:** The historical test predates the extraction this
phase performs.
**How to avoid:** Restore the test (full source recovered via `git show
7453561^:internal/dummytui/nobackend_test.go` this session — see Code
Examples) with its allowlist UPDATED to `{internal/dummytui,
cmd/gitid-dummy, <new-shared-package-import-path>}`. Do this in the SAME
task/commit as the D-17 extraction, not as a follow-up — an unrestored or
stale-allowlist test is worse than no test (false confidence).
**Warning signs:** `go test ./internal/dummytui/...` passing with no
`TestNoBackendAllowlist`-shaped test present at all.

### Pitfall 6: `CreateFlowTestFailOutput`'s exact text is `ReachableNotUploaded`, not `Failure`

**What goes wrong:** Planning or implementing the D-02 warning state by
looking at the dummy's existing "test-fail" fixture
(`internal/dummytui/data.go`'s `CreateFlowTestFailOutput = "git@ssh.
github.com: Permission denied (publickey)."`) and assuming it demonstrates
a hard failure. It does not — that exact string is
`tester.ClassifyPreWrite`'s ReachableNotUploaded trigger substring.
**Why it happens:** The approved Phase 2 demo only had binary PASS/
Failure states (per 03-UI-SPEC.md's own framing of D-02 as a scoped
divergence); its "failure" fixture was chosen for a plausible SSH error
message without knowledge of the real backend's classification rule.
**How to avoid:** When wiring the real test-outcome screen, `ssh`
returning "Permission denied (publickey)" must render the NEW D-02
warning state (`! Reachable — key not uploaded yet`), not the demo's
existing red `✗` failure treatment. Reserve the red failure rendering for
genuinely different substrings (connection refused, DNS failure, timeout
— see `e2e/harness_test.go`'s `FakeSSHDir` "timeout" mode for a concrete
`Failure`-classified fixture).
**Warning signs:** A PTY e2e or visual-regression capture showing red/✗
for a "Permission denied (publickey)" output.

### Pitfall 7: `stage2Cmd`'s dummy fixture omits `-i <key>` "by design" — verify this against the REAL `ResolvedVia` call shape

**What goes wrong:** `internal/dummytui/identities.go`'s
`wizardModel.stage2Cmd()` renders `ssh -G -F <tmp> <host> | grep
identityfile` with a comment "no -i BY DESIGN" (stage 2 tests alias
RESOLUTION, not authentication). The real `internal/tester.ResolvedVia`,
however, DOES take a `keyPath` argument and includes `-i <keyPath>` in
BOTH its connectivity args and its `-G` invocation does not use `-i` at
all (only the connectivity `ssh -F ... -i ... -T` call does — the `-G`
call is `ssh -F <configPath> -G <alias>`, no `-i`). Confirm the exact
command text shown to the user (TEST-01's "exact command shown" contract)
is generated FROM the real `tester` package's actual invocation, not
copy-pasted from the dummy's cosmetic string-building.
**Why it happens:** The dummy's `stage1Cmd`/`stage2Cmd` are STRINGS
built for display purposes only (no real exec); the real `tester`
functions build their OWN argument slices independently. A naive port
risks the demo's hand-written string diverging from what `tester`
actually runs — which would violate TEST-01's "shown command == run
command" contract in the FIRST discrepancy either package's flag order
changes.
**How to avoid:** Render the exact-command text for both test stages by
calling `tester.PreWrite`'s (or an equivalent read-only command-string
helper) actual command construction — `internal/tester.go` already
exposes `PreWriteCommand` for exactly this "display without executing"
need for stage 1. If no equivalent exists for the `ResolvedVia`/stage-2
shape, that is a small new pure-function need (mirroring
`PreWriteCommand`'s pattern), not a re-hand-rolled string in the TUI
layer.
**Warning signs:** The on-screen "exact command" text not matching what a
`strace`/PTY capture shows was actually executed.

## Code Examples

### Restoring the no-backend allowlist test (verified prior source, D-17/Pitfall 5)

```go
// Source: git show 7453561^:internal/dummytui/nobackend_test.go
// (recovered this session; UPDATE the `allowed` map for the new shared
// package's import path before restoring)
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

### Reusing the existing fake-ssh PATH-shim for PTY e2e (D-22)

```go
// Source: e2e/harness_test.go FakeSSHDir (existing, read this session) —
// already used by e2e/create_e2e_test.go via plain exec.Command. Extend
// the SAME helper into ui_pty_e2e_test.go's PTY-based tests by prepending
// its returned dir to the PTY-launched cmd.Env, e.g.:
fakeSSHDir := FakeSSHDir(t, "denied") // → tester.ReachableNotUploaded
cmd := exec.Command(bin)
cmd.Env = append(os.Environ(),
	"HOME="+home,
	"GITID_FAKE_SSH_MODE=denied",
	"PATH="+fakeSSHDir+":"+os.Getenv("PATH"),
)
sess := startPTY(t, cmd) // existing PTY harness from ui_pty_e2e_test.go
```

### The provider table Phase 3 must reuse, not reinvent (D-20/D-21)

```go
// Source: internal/identity/identity.go DefaultHostname (existing, read
// this session) — already implements D-20/D-21 exactly, including the
// recipe-cited endpoints and the unknown-provider fallback.
func DefaultHostname(provider string) string {
	token := strings.ToLower(provider)
	if idx := strings.IndexByte(token, '.'); idx >= 0 {
		token = token[:idx]
	}
	switch token {
	case "github":
		return "ssh.github.com"
	case "gitlab":
		return "altssh.gitlab.com"
	case "bitbucket":
		return "altssh.bitbucket.org"
	default:
		if strings.ContainsRune(strings.ToLower(provider), '.') {
			return strings.ToLower(provider)
		}
		return strings.ToLower(provider) + ".com"
	}
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Interactive Cobra prompts (`cmd/gitid identity add`, bufio.Reader) | Real Bubble Tea v2 TUI wizard (this phase) | Phase 3 (D-14/D-15) | The entire `add.go`/`rotate.go`/`delete.go`/`doctor.go`/`adopt.go`/`copy.go`/`list.go`/`update.go`/`test.go` Cobra command surface becomes historical; `identity.Create`/`PersistAll` etc. remain the reused orchestration layer underneath |
| `tui/` package (pre-redesign Bubble Tea model) | `internal/dummytui`-derived shared presentation package | Phase 2 redesign (already decided), realized in Phase 3 | `tui/` stops being `main.go`'s entry point; its Deps-wiring pattern is the only thing worth carrying forward |
| Static 50-screen PNG reference set | Interactive web demo + interactive Go TUI dummy demo | 2026-07-04 (`7453561`), already complete | The D-24 gate diffs against the LIVE dummy's `View()` output, not static PNGs |

**Deprecated/outdated:**
- The static screenshot-manifest capture layer (`internal/screenshot`'s
  old `manifest.go`/`design_adapter.go`) was already removed in `7453561`
  — do not resurrect it; the current `internal/screenshot/tui.go`
  (`CaptureTUI`) is the live mechanism.
- `e2e/dummy_nav_e2e_test.go` and its `BuildDummyBinary` harness were
  already removed in the same commit — do not reference them as a
  pattern; `e2e/ui_pty_e2e_test.go`'s `startPTY`/`BuildBinary` (the real
  binary) is the current pattern.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | GitLab's alt-SSH port-443 endpoint is `altssh.gitlab.com` (matches `internal/identity.DefaultHostname`, already-shipped code, but the ORIGINAL provenance of that hostname value is training-data/gist-derived, not independently re-verified against current GitLab docs this session) | Standard Stack / Code Examples (DefaultHostname) | If GitLab has changed its alt-SSH hostname, the SSHUI-01 auto-fill would be wrong for GitLab users; low risk since this is EXISTING shipped code the user has presumably already validated via the recipes' gist provenance, not a new claim Phase 3 introduces |
| A2 | The `tui/` package should be archived/retired as part of Phase 3 rather than left in place and simply un-wired from `main.go` | Common Pitfalls #3 | If left in place unused, it is dead code that could confuse future contributors or accidentally get re-wired; if archived without the user's explicit sign-off, it could be seen as scope creep beyond D-14's literal "Cobra commands" wording — recommend surfacing this as an explicit discuss-phase/planning question rather than a silent decision |
| A3 | No task in Phase 3 needs a new external Go dependency (Package Legitimacy Audit's "not applicable" verdict) | Package Legitimacy Audit | If a key-reuse picker's fingerprint display wants a friendlier format than raw `x/crypto/ssh`, a future task might reach for a formatting helper package unnecessarily — re-verify at planning time that `ssh.FingerprintSHA256` (stdlib-adjacent, part of x/crypto/ssh, already a dependency) suffices |

**If this table is empty:** N/A — see above.

## Open Questions

1. **What is the D-17 shared package's exact name and directory?**
   - What we know: it must live under `internal/` (backend-free, but not
     itself a "public" API), be imported by both `cmd/gitid` and
     `cmd/gitid-dummy`, and be excluded from any future `internal/doctor`
     depguard-style restriction (it will need to import bubbletea/
     lipgloss/bubbles, none of which are backend packages, so this is not
     expected to conflict with the existing `doctor-no-filewriter`
     depguard rule).
   - What's unclear: whether to name it by role (`internal/tuikit`,
     `internal/uiframe`) or by design-system convention (`internal/
     presentation`, mirroring "the presentation layer" language in
     03-UI-SPEC.md).
   - Recommendation: this is explicitly Claude's Discretion per
     CONTEXT.md — the planner should pick one name and use it
     consistently across every task; do not leave it to per-task
     improvisation.

2. **Should `tui/` be archived in Phase 3 or left dormant?**
   - What we know: it must stop being `cmd/gitid/main.go`'s entry point
     (D-15 requires the approved chrome); its Cobra-adjacent Deps-wiring
     functions are a useful reference.
   - What's unclear: D-14's context text lists only Cobra COMMANDS to
     archive (`identity add/rotate/delete, doctor, adopt, copy, list`),
     never mentioning the `tui/` package by name — an oversight in
     CONTEXT.md, or a deliberate signal that `tui/` handling is out of
     Phase 3's stated scope and should just be silently un-imported?
   - Recommendation: surface this explicitly to the user/discuss-phase
     before planning locks it in, since it changes the size of the
     "archive" task materially (13.6k more lines to move).

3. **Where does the key-reuse picker's key-scan/fingerprint helper live?**
   - What we know: no existing package scans `~/.ssh` for parseable
     private keys and reports filename+algorithm+fingerprint (D-10). The
     closest existing logic is `internal/identity/inventory.go`'s
     `listKeyFilesReal` (globs `id_*`, excludes `.pub`) and
     `internal/keygen.DerivePublicKey` (parses a single key).
   - What's unclear: whether this belongs as a new function in
     `internal/keygen` (algorithm-adjacent) or a new small package
     (`internal/keyscan`), and whether fingerprint computation needs a
     new helper or whether `x/crypto/ssh`'s `ssh.FingerprintSHA256(pub)`
     suffices directly.
   - Recommendation: a new small, focused function set in
     `internal/keygen` (extending the existing package rather than adding
     a new one) is the lower-friction choice, since `DerivePublicKey`
     already lives there and the picker needs the same parse step.

## Environment Availability

Not applicable — Phase 3 has no new external tool/service/runtime
dependency beyond what Phase 1's `make setup-env` already provisions
(golangci-lint, gosec, pre-commit, `freeze`, the pinned Chromium
revision). `ssh`/`ssh-add` binary presence is already a documented
runtime assumption of the shipped `gitid` tool (CLAUDE.md's "git" runtime
dependency note applies equally to `ssh`), not a new Phase 3 concern.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go's built-in `testing` package (`go test`), race-enabled (`make test`); a separate `e2e` build-tag suite (`make test-e2e`) |
| Config file | none — `go test` flags live in `Makefile`'s `test`/`test-e2e` targets |
| Quick run command | `go test -race ./internal/... ./cmd/...` (scoped to changed packages during a task) |
| Full suite command | `make test && make lint && make test-e2e` |

### Phase Requirement → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SSHUI-01 | Field model + provider auto-fill from Host suffix | unit | `go test -race ./internal/identity/... -run TestDefaultHostname` | ✅ (existing, `identity_test.go`) |
| SSHUI-02 | Clickable fields (mouse focus) | unit + PTY e2e | `go test -race ./internal/dummytui/... -run TestMouse` (pattern) + new PTY e2e | ✅ pattern exists (`mouse_test.go`); ❌ new-package + real-binary PTY test — Wave 0 gap |
| SSHUI-03 | Live Host block preview | unit | new golden test in the shared package (mirrors `hostBlockText`/`renderHostBlockPreview` tests) | ❌ Wave 0 gap (new package) |
| SSHUI-04 | Temp-file testing (never mutates live config until confirm) | unit + e2e | `go test -race ./internal/identity/... -run TestPersistSSH` (existing-adjacent) + e2e asserting `~/.ssh/config` untouched pre-confirm | ✅ substrate tested; ❌ new e2e assertion for the real wizard — Wave 0 gap |
| SSHUI-05 | macOS `Host *` globals block, idempotent | unit | `go test -race ./internal/sshconfig/... -run TestRenderGlobalBlock` (pattern) | ✅ (existing pattern in `sshconfig` tests) |
| TEST-01 | Two-stage, exact command shown | unit + PTY e2e | `go test -race ./internal/tester/...` (existing) + new PTY e2e capturing on-screen command text | ✅ unit; ❌ PTY e2e for the new wizard — Wave 0 gap |
| TEST-02 | `ssh -G` proof | unit | `go test -race ./internal/tester/... -run TestParseResolved` | ✅ existing |
| TEST-03 | Store or adopt on pass | unit + e2e | `go test -race ./internal/sshconfig/... -run TestAdopt` (existing) + new e2e for the real wizard's confirm-write | ✅ unit; ❌ new e2e — Wave 0 gap |
| KEY-06 | Reuse existing key (incl. encrypted-with-.pub) | unit | new test in `internal/identity/modes_test.go` for the `ensurePub` fix (Pitfall 2) | ❌ Wave 0 gap — RED test needed for the encrypted-key case before the fix |
| DLV-04 | Visual-regression gate | new `make` target + golden diff | `make gate-visual-regression` (new target name, TBD) | ❌ Wave 0 gap — new Makefile target + golden files |
| DLV-06 | Per-screen PTY e2e on real binary | e2e | extend `e2e/ui_pty_e2e_test.go` with new wizard-screen cases, `make test-e2e` | ✅ harness exists; ❌ new per-screen test cases — Wave 0 gap |

### Sampling Rate

- **Per task commit:** `go test -race ./internal/... ./cmd/...` scoped to
  touched packages (fast).
- **Per wave merge:** `make test && make lint && make test-e2e` (the
  orchestrator-run full battery per the project's established
  review-gate convention — see project memory "Review gate -race vs
  executor non-race": executors report PASS without `-race`; the
  orchestrator must independently run the race-enabled full suite at
  wave close).
- **Phase gate:** Full suite green (`make test`/`lint`/`test-e2e`) PLUS
  the new `gate-no-backend-files` allowlist update PLUS the new D-24
  golden-diff gate, before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] A RED unit test in `internal/identity/modes_test.go` proving
      `Reuse` currently fails (or will fail once wired) for an encrypted
      key with an existing `.pub` sibling — covers KEY-06/D-11 (Pitfall 2)
- [ ] The restored `internal/dummytui/nobackend_test.go` (updated
      allowlist) — covers D-17/W2 (Pitfall 5)
- [ ] A new `gate-no-backend-files` allowlist entry test/assertion for
      the shared package path — covers D-17 (Pitfall 4)
- [ ] New PTY e2e test files/cases in `e2e/ui_pty_e2e_test.go` for: the
      algorithm+SSH form screen, the test-stage screens (using
      `FakeSSHDir` modes `pass`/`denied`/`timeout` to hit
      PASS/ReachableNotUploaded/Failure), the git-form-demo'd screen
      (D-18/D-19), and the confirm-write ceremony — covers TEST-01..03,
      DLV-06, KEY-06
- [ ] A new Makefile target for the D-24 golden-text diff gate (name
      TBD — e.g. `gate-visual-regression`), plus the golden-file
      capture/storage convention (Claude's Discretion per CONTEXT.md)
- [ ] A new pure command-string helper for stage-2 display parity
      (Pitfall 7), if none already exists alongside `PreWriteCommand`

## Security Domain

`security_enforcement` is enabled (`security_asvs_level: 1`,
`security_block_on: "high"` per `.planning/config.json`).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | gitid is a local single-user CLI/TUI; no auth surface of its own |
| V3 Session Management | no | not applicable — no session concept |
| V4 Access Control | no | single local user, no multi-tenant boundary |
| V5 Input Validation | yes | identity/alias name validation (`identityNameRe`, `identity.ValidateName`), provider validation (`identity.ValidateProvider`), alias-collision validation against ALL parsed Host patterns (D-09) — all ALREADY IMPLEMENTED patterns to reuse, not reinvent |
| V6 Cryptography | yes | key generation/parsing via `golang.org/x/crypto/ssh` only — never hand-roll ed25519/RSA math; already the project's established pattern (CLAUDE.md "What NOT to Use" table) |
| V7 Error Handling / Logging | yes | private-key material (`StagedKey.PrivPEM`) must NEVER be logged or printed — already an explicit doc-comment contract in `internal/identity/identity.go`; Phase 3's new TUI rendering code must uphold this when displaying test output/previews |
| V9 Communications | yes | `ssh`/`ssh -G` invocations already use arg-slice `exec.Command` (never shell string interpolation) — gosec G204-clean pattern already established; new PTY e2e / provider-table code must preserve this |
| V12 File and Resources | yes | every write already goes through `internal/filewriter`'s backup+atomic+chmod chokepoint; V12-relevant permission bits (0700 `~/.ssh`, 0600 keys/config, 0644 `.pub`) are already enforced constants, not decisions Phase 3 makes fresh |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Command injection via SSH host/alias/key-path values | Tampering | Arg-slice `exec.Command` (no shell), already the established pattern in `internal/tester`/`cmd/gitid/add.go`'s `loadKeyIntoAgent` — new PTY/wizard code must not introduce a `sh -c` string-interpolation path anywhere |
| Path traversal via a user-supplied "existing key path" (KEY-06 manual-path row) | Tampering / Information Disclosure | `internal/adopter.MatchIdentityName`'s existing `os.Lstat` symlink-rejection pattern (T-05.7-02-02) is the established mitigation for "candidate file the user points at" — the new key-reuse picker's manual-path row should apply the SAME symlink-rejection discipline before parsing |
| Private key material leaking into logs/preview text | Information Disclosure | `StagedKey.PrivPEM` is already documented "NEVER log or print"; the new TUI ceremony/preview rendering must only ever surface `PubLine`/paths, never `PrivPEM` |
| Backup-file collision clobbering a still-live recovery snapshot | Tampering / Denial of Service (data loss) | Already mitigated by `internal/filewriter`'s exclusive-create + nanosecond-retry `backupExistingTarget` — Phase 3 must route ALL new writes (including the D-06 dual-file first-create) through `filewriter.Write`, never a raw `os.WriteFile` |
| Encrypted private key silently accepted then mis-parsed, corrupting the derived `.pub`/allowed_signers line | Tampering (data integrity) | Fix `ensurePub` (Pitfall 2) to read the EXISTING `.pub` rather than blindly re-deriving; never attempt a passphrase prompt (D-11 explicitly forbids it) |

## Sources

### Primary (HIGH confidence — direct repo inspection this session)
- `internal/tester/tester.go`, `doc.go` — PASS/ReachableNotUploaded/Failure classification, `PreWriteCommand`, `ResolvedVia`
- `internal/sshconfig/adopt.go`, `include.go`, `doc.go` — `DetectInclude`/`Adopt` (STORE-01/02 auto-detect), `EnsureIncludeLine`
- `internal/adopter/adopter.go` — confirmed this is the GITCONFIG-fragment adopter, distinct from `internal/sshconfig.Adopt`
- `internal/filewriter/filewriter.go`, `block.go` — `Write`/`WriteNoBackup`/`BackupAndRemove`, sentinel-block helpers
- `internal/keygen/derive.go`, `registry.go`, `catalog.go` — `DerivePublicKey`, algorithm registry/catalog
- `internal/identity/identity.go`, `state.go`, `inventory.go`, `modes.go` — `Create`/`Reuse`/`AddAccount`/`PersistSSH`/`PersistGitconfig`/`PersistAll`, `DefaultHostname`/`Port`/`Alias`/`Match`, MGR-02 taxonomy, the `ensurePub` gap (Pitfall 2)
- `cmd/gitid/main.go`, `add.go` — current POC command surface, `buildDeps` wiring pattern, confirmed bare-invocation calls `tui.Run()`
- `tui/tui.go` — confirmed the old pre-redesign package is what `main.go` currently wires
- `cmd/gitid-dummy/main.go`, `internal/dummytui/app.go`, `doc.go`, `identities.go`, `theme.go` — the D-17 extraction source, `screenModel`/`mouseTarget` contracts, wizard step machinery, `CreateFlowTestFailOutput` fixture text
- `internal/dummytui/data.go` — `CreateFlowTestStage1Command`/`CreateFlowTestStage2Command`/`CreateFlowTestFailOutput` fixture constants
- `e2e/harness_test.go` — `FakeSSHDir` (D-22's PATH-shim, already built), modes pass/denied/timeout
- `e2e/create_e2e_test.go` — existing (POC-CLI-targeting) use of `FakeSSHDir`, to be superseded by PTY equivalents
- `e2e/ui_pty_e2e_test.go` — the raw-keystroke PTY harness (`startPTY`, `ptySession`) to extend for DLV-06
- `internal/screenshot/tui.go` — `CaptureTUI` (freeze-based golden→PNG pipeline, D-24's screenshot half)
- `Makefile` — `gate-no-backend-files` target and its exact allowlist regex, `test-e2e`/`screenshot-tui` targets, `FREEZE_VERSION`
- `.golangci.yml` — the `doctor-no-filewriter` depguard rule (confirms `internal/doctor` stays write-free; not directly Phase 3 scope but relevant to any doctor-adjacent D-16 banner work)
- `git show 7453561` and `7453561^:internal/dummytui/nobackend_test.go` — recovered the exact deleted no-backend allowlist test source (Pitfall 5/Code Examples)
- `go.mod` — verified every dependency version cited in Standard Stack
- `.planning/config.json` — confirmed `nyquist_validation: true`, `security_enforcement: true`, `security_asvs_level: 1`
- `recipes/README.md`, `recipes/ssh-config.recipe` — canonical Host-block shape (alias, alt-SSH hostname, Port 443, `IdentitiesOnly yes`) cross-checked against `DefaultHostname`'s literal values

### Secondary (MEDIUM confidence)
- None — every claim in this document was verified directly against this repository's source this session; no external web search was needed because the phase is substrate-reuse over an already-fully-inspected codebase.

### Tertiary (LOW confidence)
- A1 in the Assumptions Log (GitLab's alt-SSH hostname) — the VALUE is already-shipped code, but its ultimate provenance (the maintainer's gist, per `recipes/README.md`) was not re-verified against GitLab's current documentation this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every dependency verified directly against `go.mod`/`Makefile`, zero new packages
- Architecture: HIGH — the extraction target (`internal/dummytui`) and every backend package it must wire to were read directly this session
- Pitfalls: HIGH — five of seven pitfalls are backed by direct source inspection (not inference); two (Pitfall 6/7) are derived from cross-referencing existing fixture text against existing classifier code

**Research date:** 2026-07-07
**Valid until:** 30 days (stable, internal-codebase-driven research; re-validate if `internal/dummytui`, `internal/identity`, or `internal/tester` change materially before Phase 3 planning executes)
