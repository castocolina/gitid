---
id: 260922-brh
slug: fix-7-red-ci-tests-stale-diff3-source-gr
phase: quick-260922-brh
plan: 01
status: active
type: execute
wave: 1
depends_on: []
autonomous: true
requirements: [BUILD-02, STORE-01, STORE-02, GGIT-01]
files_modified:
  - internal/sshconfig/adopt.go
  - internal/sshconfig/adopt_test.go
  - internal/sshconfig/validation.go
  - internal/sshconfig/validation_test.go
  - cmd/gitid/wiring.go
  - cmd/gitid/wiring_test.go
  - cmd/gitid/git_test.go
estimate:
  tokens: 75000
  raw_tokens: 75000
  tasks: 3
  confidence: low
must_haves:
  truths:
    - "A backend built for home X (newBackendForHome(X)) resolves the Include'd SSH storage target, the global-SSH target, and alias collisions only under X, even when the process $HOME holds a sentinel-bearing ~/.ssh/config.d/gitid.config."
    - "The 6 HOME-dependent tests (TestCustomSSHDirectivePlanShowsRealDiff, TestRunCustomSSHDirectiveWriteLandsInTheExistingGlobalBlock, TestCommitGlobalSSHEndToEnd, TestApplyThenCreatePreservesGlobalFix, TestGlobalsFixThenCreateSurvivesCreate/include, TestGlobalsCreateThenFixLeavesIdentityUntouched/include) pass with the real populated HOME, an empty temp HOME, and a decoy HOME."
    - "TestGitOptionsApplyBelowGateAdvisoryOriginatesFromCeremony passes. It still fails if the CLI layer (cmd/gitid/git.go) builds the below-gate advisory text itself, or if lifecycle.go hardcodes the fallback value instead of deriving it from internal/globalgit policy."
    - "`go test ./... -race -count=1` and `make lint` are green."
    - "No test run mutates the real ~/.ssh or ~/.gitconfig: sha256 and mtime are identical before and after."
  artifacts:
    - "internal/sshconfig/adopt.go: DetectIncludeForHome, ParseIncludeLineForHome, AdoptDeps.Home, RealAdoptDepsForHome. The process-home DetectInclude and ParseIncludeLine become thin wrappers whose behavior is unchanged."
    - "internal/sshconfig/validation.go: AliasCollisionForHome. The process-home AliasCollision becomes a thin wrapper whose behavior is unchanged."
    - "cmd/gitid/wiring.go: storage(), hasIncludeLine(), resolveGlobalSSHTargetPath(), and (*realBackend).AliasCollision all use the home-aware sshconfig API with the backend's own home."
    - "cmd/gitid/wiring_test.go: decoy-HOME regression tests plus an AST guard that bans process-home sshconfig Include calls from cmd/gitid production files."
    - "cmd/gitid/git_test.go: diff3 test rewritten to use policy-derived values and AST provenance checks."
  key_links:
    - "(*realBackend).storage() -> sshconfig.Adopt(b.sshConfigPath, AdoptSentinelBearing, \"\", RealAdoptDepsForHome(b.home)) -> DetectIncludeForHome -> expandIncludePathForHome(raw, b.home). This is the edge that leaked the process $HOME."
    - "containedRegularPath(path, b.home) in mutationJournal.watchFile/watchDir: the existing defense-in-depth guard that turned the leak into a refusal instead of a write to the real home. Keep it unchanged."
    - "globalgit.PolicyFor(\"merge.conflictstyle\").Fallback -> gitExplicitValues -> runGlobalGitApply advisory. The test must derive expected values from this policy row, never from a literal in lifecycle.go (WR-09)."
---

# Quick 260922-brh: fix the 7 red cmd/gitid tests

**Goal:** `make test` (race) passes on the GitHub `check` matrix again, and a backend built for
home X can never resolve SSH Include paths against a different home.

## Planning-time evidence (live, this session)

All generated content is English only. That covers code, comments, test names, commit messages, and SUMMARY.

1. **CI fails only on the diff3 test. The brief said otherwise.** `gh run view 35712780094 --log-failed`
   shows one `--- FAIL` on each of macos-15, macos-15-intel and ubuntu-latest:
   `TestGitOptionsApplyBelowGateAdvisoryOriginatesFromCeremony` (git_test.go:682). None of the 6
   HOME tests fail on CI, because CI runners have an empty `~/.ssh`.
2. **The 6 HOME tests leak the process `$HOME`. Production bug, root-caused:**
   `internal/sshconfig/adopt.go` `expandIncludePath` expands `~/` (and bare-relative tokens) with
   `os.UserHomeDir()`. `(*realBackend).storage()` (wiring.go ~4692) calls
   `sshconfig.Adopt(b.sshConfigPath, …, sshconfig.RealAdoptDeps())`. Once the sandboxed home's
   `~/.ssh/config` carries gitid's own `Include ~/.ssh/config.d/*.config`, Adopt globs the
   **process** home and picks its sentinel-bearing `gitid.config`. storage() then returns a path
   outside `b.home`, and `containedRegularPath` refuses it. The same leak affects
   `hasIncludeLine()` (wiring.go ~4723), `resolveGlobalSSHTargetPath()` (wiring.go ~5070), and
   `(*realBackend).AliasCollision()` (wiring.go ~1179, via `sshconfig.AliasCollision` ->
   `aliasCollides` -> `DetectInclude`). The home is not cached (it's `os.UserHomeDir()` on every
   call), and none of these tests call `t.Parallel`.
   - Input: `go test ./cmd/gitid/ -run '<6 tests>'` with real HOME. Output: all 6 fail with
     `refusing path outside managed home: /home/bazzite/.ssh/config.d/gitid.config`.
   - Input: the same run with `HOME=<empty temp dir>`. Output: all 6 pass.
   - Input: the same run with `HOME=<decoy dir holding a sentinel-bearing .ssh/config.d/gitid.config>`.
     Output: all 6 fail, naming the **decoy** path. That makes `$HOME` the proven cause. The decoy file
     was byte-unchanged afterwards, so the guard blocked every write.
   - Decision: fix production code. This is the same WR-35 lesson the codebase already encodes in
     `expandTildeForHome` and `identity.InventoryDepsForHome`: a caller that owns an explicit home
     must never re-derive it from the process environment.
3. **The diff3 source-grep is stale.** Commit 5752078 moved fallback derivation into
   `gitExplicitValues` -> `globalgit.WriteValueFor`/`WriteRequestedValueFor`, so the policy row now
   holds the value (`internal/globalgit/policy.go` ~269), not a literal in lifecycle.go. The advisory
   is still built by the ceremony (`runGlobalGitApply`, lifecycle.go ~1260-1267). The test's
   functional assertions pass. Only the `os.ReadFile(lifecycle.go)` + `strings.Contains(…)` block
   fails.

## Out of scope. Surface these in the SUMMARY; do NOT fix them here.

- ~~Fedora job~~ — MOVED IN SCOPE by the orchestrator as Task 4 (verified live: run 35712780094
  fedora log shows `go: -race requires cgo`), because the quick task's goal is a green main.
- **Latent bug in `hasIncludeLine()`:** it compares the directive's expanded glob
  (`<home>/.ssh/config.d/*.config`) with the directory `b.includeDir` (`<home>/.ssh/config.d`), so it
  never matches gitid's own Include line. Proven live: a scratch HOME holding the canonical layout
  prints `include_line false` from `gitid ssh storage show`. Fixing it changes storage's
  `needsIncludeLine` and the `gitid.ssh.storage/v1` output, so it needs its own task. This plan
  only makes that function home-aware and leaves its comparison alone.
- **Residual read-only exposure:** `internal/globalssh` (shadow.go -> `sshconfig.DetectInclude`) and
  the real `ssh -G` subprocess still resolve `~` against the process/passwd home. Everything there
  is a read; nothing is written. Threading a home into `globalssh.Deps` is a separate change.
- The real `~/.ssh/config.d/gitid.config` on this machine (mtime 2026-09-07) must NOT be touched,
  moved, or deleted.

<objective>
Fix the 7 failing cmd/gitid tests at their root causes:
(1) make SSH Include detection home-aware and wire every cmd/gitid call site to the backend's
own home, a production fix for the process-$HOME leak (STORE-01/STORE-02);
(2) replace the stale diff3 source-grep with policy-derived, AST-based checks that still prove
the advisory comes from the ceremony (GGIT-01, WR-09);
so `make test -race` is green on the GitHub check matrix again (BUILD-02).

Purpose: a backend constructed for home X must never read or target paths under a different
home, and CI must be green.
Output: the home-aware sshconfig API, the rewired backend call sites, the regression and guard
tests, the rewritten diff3 test, and two logical commits.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@CLAUDE.md
@recipes/README.md
@.planning/STATE.md
@internal/sshconfig/adopt.go
@internal/sshconfig/validation.go

Recipes check: this change writes no new config shape. The Include line gitid writes stays
`Include ~/.ssh/config.d/*.config`, and only the Go-side resolution of `~` changes. There is
no divergence from recipes/.

Relevant anchors (line numbers approximate; locate by symbol):
- cmd/gitid/wiring.go: newBackendForHome (~368, fields home/sshConfigPath/includeDir),
  (*realBackend).AliasCollision (~1172), storage() (~4682), hasIncludeLine() (~4722),
  expandTildeForHome (~4520, the WR-35 precedent to cite), containedRegularPath (~4530),
  resolveGlobalSSHTargetPath (~5069).
- cmd/gitid/wiring_test.go: existing helpers writeFile, readFile, managedBlock, seedSSHDir,
  seedInFileIdentity, countBackupSiblings. The file already imports go/parser.
- cmd/gitid/identity_test.go: testRepoRoot(t).
- cmd/gitid/git_test.go ~659-683: TestGitOptionsApplyBelowGateAdvisoryOriginatesFromCeremony.
- cmd/gitid/lifecycle.go ~1245-1270: runGlobalGitApply advisory construction.
- internal/sshconfig/adopt_test.go: mustMkdir, mustWriteFile, and the TestAdopt fixture style.
- internal/sshconfig/validation_test.go: writeFile helper and the existing AliasCollision cases.

Tooling: run `codegraph index || codegraph init -i` once at the start. Use `codegraph explore`
before Grep/Read loops, with `rg` as the fallback.
</context>

<tasks>

<task type="tracer">
  <name>Task 1 (tracer): home-aware Include expansion, end to end through Adopt and storage()</name>
  <files>internal/sshconfig/adopt.go, internal/sshconfig/adopt_test.go, cmd/gitid/wiring.go, cmd/gitid/wiring_test.go</files>
  <action>
Safety baseline first, read-only. Before any test run, write the sha256 and mtime of the real
~/.ssh/config, ~/.ssh/config.d/gitid.config and ~/.gitconfig (only those that exist) to a temp file
outside the repo, for example one made with mktemp. Task 3 compares against it. Never open these
files for writing.

RED (TDD authoring order; record each command and its real failing output in the SUMMARY):
- In internal/sshconfig/adopt_test.go, add TestDetectIncludeForHomeExpandsAgainstGivenHome.
  Make two t.TempDir() homes, managed and decoy, and call t.Setenv("HOME", decoy). Write
  managed/.ssh/config with three Include directives: a tilde glob into ~/.ssh/config.d, a
  bare-relative config.d token, and one absolute path. Assert that DetectIncludeForHome(configPath,
  managed) expands the tilde and bare-relative tokens under managed/.ssh, never under decoy, and
  returns the absolute token unchanged.
- In the same file, add TestAdoptForHomeNeverSelectsProcessHomeTarget. Seed decoy/.ssh/config.d/gitid.config
  with a gitid sentinel block, following TestAdopt's fixture style. Write managed/.ssh/config with the
  tilde config.d glob Include. Subcase "managed config.d empty": Adopt(configPath,
  AdoptSentinelBearing, "", RealAdoptDepsForHome(managed)) returns an empty TargetPath and the
  documented no-qualifier result, AdoptCreateConfigD. Subcase "managed sentinel-bearing
  gitid.config present": TargetPath is exactly the managed file.
- In cmd/gitid/wiring_test.go, add the helper seedDecoyProcessHome(t) (decoy home, decoy file
  path, decoy bytes). It creates a t.TempDir() decoy with .ssh and .ssh/config.d (0700) and
  .ssh/config.d/gitid.config (0600) holding two managed blocks built with managedBlock: a global-ssh
  block (Host *, HashKnownHosts yes) and a "decoy" identity block whose Host is decoy.github.com.
  Then it calls t.Setenv("HOME", decoy).
- Add the tracer test TestStorageIgnoresProcessHomeIncludeTarget. Set home := t.TempDir(), call
  seedDecoyProcessHome, then b := newBackendForHome(home). The first b.CommitGlobalSSH for
  HashKnownHosts must succeed. After it, b.storage().targetPath must equal
  filepath.Join(home, ".ssh", "config.d", "gitid.config"). A second CommitGlobalSSH must succeed
  with an empty Err. The decoy file bytes and the decoy config.d entry count must be unchanged, so
  no backup sibling appeared there. Don't call t.Parallel (t.Setenv forbids it).
- Run the RED check. The cmd/gitid test compiles and fails with "refusing path outside managed
  home" naming the decoy path. The sshconfig tests fail to compile because DetectIncludeForHome and
  RealAdoptDepsForHome are undefined. Both count as RED.

GREEN:
- internal/sshconfig/adopt.go:
  - Add an unexported processHome() helper. It returns os.UserHomeDir() and ignores the error,
    exactly as today.
  - Add expandIncludePathForHome(raw, home string) with the same switch as expandIncludePath:
    absolute stays unchanged, "~/" joins onto home, and bare-relative joins onto home/.ssh.
    expandIncludePath then delegates with processHome().
  - Add the exported ParseIncludeLineForHome(line, home string) and
    DetectIncludeForHome(configPath, home string). Turn ParseIncludeLine and DetectInclude into
    thin wrappers that pass processHome(), so every existing caller behaves exactly as before.
  - Doc comments must say the ForHome variants are REQUIRED for any caller that owns an explicit
    managed home (the WR-35 lesson; cite cmd/gitid expandTildeForHome). The process-home wrappers
    exist only for callers that own no home.
  - Add a Home string field to AdoptDeps. Document it as the managed home that "~/" and
    bare-relative Include tokens expand against. Empty means the process home, which keeps the
    old RealAdoptDeps behavior.
  - In Adopt, when deps.Home is non-empty, use DetectIncludeForHome(configPath, deps.Home).
    Otherwise use DetectInclude.
  - Add RealAdoptDepsForHome(home string) AdoptDeps: RealAdoptDeps() with Home set.
- cmd/gitid/wiring.go storage(): pass sshconfig.RealAdoptDepsForHome(b.home) to Adopt. Extend
  storage()'s doc comment with one sentence saying Include tokens resolve against b.home (WR-35).
  Leave hasIncludeLine, resolveGlobalSSHTargetPath and AliasCollision to Task 2. Keep
  containedRegularPath unchanged; it is defense-in-depth.
- Do not commit yet. Per CLAUDE.md, Tasks 1 and 2 are one logical change and ship as one commit at
  the end of Task 2.
  </action>
  <verify>
    <automated>go test ./internal/sshconfig/ -count=1 -run 'TestDetectInclude|TestAdopt' && go test ./cmd/gitid/ -count=1 -run 'TestStorageIgnoresProcessHomeIncludeTarget|TestStorage|TestCustomSSHDirectivePlanShowsRealDiff|TestRunCustomSSHDirectiveWriteLandsInTheExistingGlobalBlock|TestCommitGlobalSSHEndToEnd|TestApplyThenCreatePreservesGlobalFix|TestGlobalsFixThenCreateSurvivesCreate|TestGlobalsCreateThenFixLeavesIdentityUntouched'</automated>
  </verify>
  <done>The new sshconfig tests and the tracer test pass. The 6 formerly-red HOME tests pass on this
  machine with the real populated HOME. All existing TestAdopt, TestDetectInclude and TestStorage
  cases still pass unchanged. The RED command and output are recorded for the SUMMARY.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: route the remaining Include-reading call sites through the backend home, add an AST guard, and commit</name>
  <files>internal/sshconfig/validation.go, internal/sshconfig/validation_test.go, cmd/gitid/wiring.go, cmd/gitid/wiring_test.go</files>
  <behavior>
    - AliasCollisionForHome(managedConfig, managed, "decoy.github.com") is false when that host is declared only in the process-HOME decoy's Include'd file. The same call is true for a host declared in managed's own Include'd file.
    - The process-home AliasCollision(configPath, candidate) behaves exactly as before: every existing validation_test case passes unchanged.
    - With a decoy process HOME, resolveGlobalSSHTargetPath(home, b.sshConfigPath) returns home/.ssh/config.d/gitid.config and never the decoy path. b.AliasCollision("decoy.github.com") is false. b.AliasCollision for the managed home's own included identity alias is true.
    - An AST guard fails, listing file:line, if any non-_test.go file in cmd/gitid calls sshconfig.DetectInclude, sshconfig.ParseIncludeLine, sshconfig.AliasCollision or sshconfig.RealAdoptDeps (the process-home variants).
  </behavior>
  <action>
RED: write these tests first, run them, and record the failing output in the SUMMARY.
- internal/sshconfig/validation_test.go: add TestAliasCollisionForHomeIgnoresProcessHomeIncludes.
  It covers both managed and decoy homes, calls t.Setenv("HOME", decoy), and gives each home an
  Include'd config.d file declaring a different Host. It fails to compile until
  AliasCollisionForHome exists.
- cmd/gitid/wiring_test.go: add TestBackendIncludeReadsIgnoreProcessHome. Seed the managed home with
  the Include layout: an ssh-include managed block in ~/.ssh/config, plus a sentinel-bearing
  config.d/gitid.config holding a managed identity block for an alias of your choice. Use
  seedDecoyProcessHome from Task 1 and assert the behavior bullets. It fails today, because
  resolveGlobalSSHTargetPath returns the decoy path and AliasCollision reports the decoy host.
- cmd/gitid/wiring_test.go: add TestCmdGitidIncludeDetectionIsHomeAware.
  - Parse every non-_test.go file under filepath.Join(testRepoRoot(t), "cmd", "gitid") with
    go/parser.
  - Walk ast.CallExpr nodes whose Fun is an ast.SelectorExpr with an *ast.Ident receiver named
    sshconfig and a Sel in the banned set: DetectInclude, ParseIncludeLine, AliasCollision,
    RealAdoptDeps.
  - Report each hit as file:line. The AST ignores comments, so doc-comment mentions stay allowed.
  - It fails today and lists the wiring.go call sites.

GREEN:
- internal/sshconfig/validation.go:
  - Add AliasCollisionForHome(configPath, home, candidate string) (bool, error).
  - Add a home parameter to aliasCollides and thread it through the recursion. Replace its
    DetectInclude call with DetectIncludeForHome(abs, home).
  - AliasCollision(configPath, candidate) becomes a wrapper passing processHome(), so its
    behavior is unchanged.
  - Update the doc comments the same way as Task 1 (the ForHome variant is required when the
    caller owns a home).
- cmd/gitid/wiring.go:
  - hasIncludeLine() calls sshconfig.DetectIncludeForHome(b.sshConfigPath, b.home). Keep its
    comparison exactly as-is; the glob-vs-directory mismatch is an out-of-scope finding.
  - resolveGlobalSSHTargetPath passes sshconfig.RealAdoptDepsForHome(home) to Adopt.
  - (*realBackend).AliasCollision returns sshconfig.AliasCollisionForHome(b.sshConfigPath, b.home, alias).

Run gofmt/goimports through make fmt, then `go test ./internal/sshconfig/ ./cmd/gitid/ -count=1`.

COMMIT: one logical commit for Tasks 1 and 2. Stage only the six files of Tasks 1 and 2.
- Message: "fix(sshconfig): expand Include ~ against the backend's managed home, not $HOME".
- The body must state the root cause: expandIncludePath used os.UserHomeDir, so storage(),
  hasIncludeLine, resolveGlobalSSHTargetPath and AliasCollision followed gitid's own Include line
  into the process home.
- The body must list the 6 fixed tests, the new ForHome API, the AST guard, and requirement IDs
  STORE-01, STORE-02, BUILD-02.
- End the message with the session's attribution trailer lines.
- The pre-commit hooks (make fmt + make lint) must pass. Never use --no-verify.
  </action>
  <verify>
    <automated>go test ./internal/sshconfig/ -count=1 && go test ./cmd/gitid/ -count=1 -run 'TestBackendIncludeReadsIgnoreProcessHome|TestCmdGitidIncludeDetectionIsHomeAware|TestStorageIgnoresProcessHomeIncludeTarget|TestStorage|TestAliasCollision|TestGlobals|TestCommitGlobalSSH' && git log -1 --format=%s</automated>
  </verify>
  <done>The new tests pass. The whole internal/sshconfig package passes. The AST guard reports zero
  process-home sshconfig Include calls in cmd/gitid production files. The commit exists and passed
  the hooks.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: replace the stale diff3 source-grep with policy-derived AST provenance checks, run the full gate, and commit</name>
  <files>cmd/gitid/git_test.go</files>
  <behavior>
    - The below-gate apply advisory names policy.Key, the quoted fallback (strconv.Quote(policy.Fallback)) and the quoted recommended value (strconv.Quote(policy.Recommended)). All three values come from globalgit.PolicyFor("merge.conflictstyle"), never from literals.
    - The advisory contains its fixed wording "below the git version gate", as produced by the backend ceremony call itself.
    - No string literal in cmd/gitid/git.go (the `gitid git` CLI layer) contains that wording: the CLI layer only renders res.Advisories.
    - No string literal in cmd/gitid/lifecycle.go equals policy.Fallback: the fallback stays derived from internal/globalgit policy (WR-09).
  </behavior>
  <action>
RED is already live: this test fails today at git_test.go:682. Record that failing output in the
SUMMARY as the RED evidence.

Edit TestGitOptionsApplyBelowGateAdvisoryOriginatesFromCeremony and keep its name. Keep the setup:
t.Setenv HOME, newBackendForHome, the GateBelow gitGate stub, the b.runGlobalGitApply call, and
the non-empty Advisories check. Then:
- Resolve policy with globalgit.PolicyFor("merge.conflictstyle"). Require ok,
  policy.Gate == globalgit.GateHard and a non-empty policy.Fallback.
- Replace the bare-value Contains check with checks for policy.Key,
  strconv.Quote(policy.Fallback) and strconv.Quote(policy.Recommended) in the joined advisories.
  The bare fallback value is a substring of the recommended value, so only the quoted forms (the
  advisory renders both with %q) discriminate. Add a one-line comment saying so.
- Assert the joined advisories contain "below the git version gate". This shows the ceremony
  produced it on the backend result before any CLI rendering.
- Delete the os.ReadFile(lifecycle.go) + strings.Contains source-grep block.
- Add a small helper in git_test.go, goStringLiterals(t, path) []string. It parses the file with
  go/parser, collects every ast.BasicLit of kind token.STRING, and strconv.Unquotes each one.
- Using that helper, over filepath.Join(testRepoRoot(t), "cmd", "gitid", …):
  - assert no literal in git.go contains "below the git version gate" (failure message: the
    advisory must originate in the ceremony, not the CLI layer);
  - assert no literal in lifecycle.go equals policy.Fallback (failure message: WR-09, the
    fallback must be derived from globalgit policy, not hardcoded).
- Do not change lifecycle.go or git.go production code.

Full gate. Record each command and its real output in the SUMMARY.
- Run `go test ./... -race -count=1`, then `make lint`.
- Run the 7 target tests plus the new tests under three HOME conditions:
  - (a) the real HOME;
  - (b) an empty mktemp HOME;
  - (c) a mktemp decoy HOME seeded with .ssh/config.d/gitid.config holding a gitid global-ssh
    sentinel block.
- For (b) and (c), capture GOCACHE, GOMODCACHE and GOPATH from `go env` into shell variables
  BEFORE overriding HOME. Pass them explicitly, because expanding them inline on the same command
  line resolves them under the new HOME.
- Compare the real-home safety snapshot from Task 1: sha256 and mtime must be identical.

COMMIT: stage only cmd/gitid/git_test.go.
- Message: "test(gitid): replace stale diff3 source-grep with policy-derived AST provenance checks".
- The body must explain that commit 5752078 moved fallback derivation into
  globalgit.WriteValueFor/WriteRequestedValueFor, name WR-09, and list requirement IDs GGIT-01
  and BUILD-02.
- End with the attribution trailer lines.
- The hooks must pass. Never use --no-verify.
  </action>
  <verify>
    <automated>GC=$(go env GOCACHE); GM=$(go env GOMODCACHE); GP=$(go env GOPATH); R='TestGitOptionsApplyBelowGateAdvisoryOriginatesFromCeremony|TestCustomSSHDirectivePlanShowsRealDiff|TestRunCustomSSHDirectiveWriteLandsInTheExistingGlobalBlock|TestCommitGlobalSSHEndToEnd|TestApplyThenCreatePreservesGlobalFix|TestGlobalsFixThenCreateSurvivesCreate|TestGlobalsCreateThenFixLeavesIdentityUntouched|TestStorageIgnoresProcessHomeIncludeTarget|TestBackendIncludeReadsIgnoreProcessHome|TestCmdGitidIncludeDetectionIsHomeAware'; go test ./... -race -count=1 && make lint && go test ./cmd/gitid/ -count=1 -run "$R" && E=$(mktemp -d) && HOME=$E GOCACHE=$GC GOMODCACHE=$GM GOPATH=$GP go test ./cmd/gitid/ -count=1 -run "$R" && D=$(mktemp -d) && mkdir -p "$D/.ssh/config.d" && chmod 700 "$D/.ssh" "$D/.ssh/config.d" && printf '# BEGIN gitid managed: global-ssh\nHost *\n  HashKnownHosts yes\n# END gitid managed: global-ssh\n' > "$D/.ssh/config.d/gitid.config" && chmod 600 "$D/.ssh/config.d/gitid.config" && HOME=$D GOCACHE=$GC GOMODCACHE=$GM GOPATH=$GP go test ./cmd/gitid/ -count=1 -run "$R"</automated>
  </verify>
  <done>`go test ./... -race -count=1` and `make lint` are green. The 7 formerly-red tests and the new
  tests pass under the real, empty and decoy HOMEs. The real ~/.ssh and ~/.gitconfig snapshot is
  unchanged. The second commit exists and passed the hooks. The SUMMARY records the RED and GREEN
  evidence and the four out-of-scope findings.</done>
</task>

<task type="auto">
  <name>Task 4: give the fedora CI container a C compiler so `make test` (-race) can run</name>
  <files>.github/workflows/ci.yml, cmd/gitid/ci_fedora_test.go (only if it asserts the dnf line)</files>
  <action>
Evidence: run 35712780094, job "fedora (container)", step "make test (-race)":
`go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`. The dnf step (ci.yml ~117-118) installs
`git openssh-clients make nodejs tar gzip` but no C compiler, so Go auto-disables cgo.
- Add `gcc` to that `dnf install` line and update the adjacent comment to say gcc is required for
  `-race` (cgo). Do not add CGO_ENABLED overrides; with gcc present Go enables cgo by default.
- If cmd/gitid/ci_fedora_test.go (or any other workflow-content test) pins the dnf package list,
  update it test-first (RED: assert gcc is present, see it fail; GREEN: edit ci.yml).
- If `podman` or `docker` is available locally, optionally prove it with
  `podman run --rm -v "$PWD":/src:Z -w /src fedora:latest sh -c 'dnf install -y --setopt=install_weak_deps=False gcc golang make git && CGO_ENABLED=1 go env CGO_ENABLED'`
  (report output; skip if no container runtime — CI after push is the real proof, note that in the SUMMARY).
- Commit: "ci(fedora): install gcc so the race-enabled test step has cgo" (hooks enabled, attribution trailers).
  </action>
  <verify>
    <automated>grep -n 'dnf install' .github/workflows/ci.yml | grep -q gcc && go test ./cmd/gitid/ -count=1 -run 'Fedora|CI'</automated>
  </verify>
  <done>ci.yml fedora dnf line includes gcc; workflow-content tests pass; commit exists with hooks enabled.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| process environment ($HOME) -> realBackend | The process home can differ from the backend's managed home (tests, sandboxes). The backend must treat b.home as the only authority. |
| managed home -> any other filesystem location | gitid writes and reads SSH config only inside b.home. Anything else belongs to another principal or context. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-260922-01 | Tampering | storage() -> sshconfig.Adopt Include resolution | high | mitigate | Adopt resolves tokens through RealAdoptDepsForHome(b.home). containedRegularPath stays as the second guard. Covered by TestStorageIgnoresProcessHomeIncludeTarget (decoy bytes unchanged). |
| T-260922-02 | Information Disclosure | hasIncludeLine / resolveGlobalSSHTargetPath / AliasCollision reading another home's Include'd files | medium | mitigate | These call sites use DetectIncludeForHome / AliasCollisionForHome with b.home. TestBackendIncludeReadsIgnoreProcessHome covers them, and the AST guard prevents regressions to the process-home variants. |
| T-260922-03 | Information Disclosure | internal/globalssh shadow simulation and `ssh -G` resolving ~ against the process/passwd home | low | accept | Pre-existing, read-only, out of scope. Recorded as a follow-up in the SUMMARY. |
| T-260922-04 | Tampering | test runs against the developer's real ~/.ssh and ~/.gitconfig | medium | mitigate | Take a read-only sha256+mtime snapshot before Task 1 and compare it after Task 3. New tests use t.TempDir() homes and t.Setenv decoys only. |

No package installs are planned, so the supply-chain gate does not apply.
</threat_model>

<verification>
- Input `go test ./... -race -count=1` -> all packages ok.
- Input `make lint` -> 0 issues.
- Input: the 7 formerly-red tests plus the new tests under the real HOME, an empty mktemp HOME and a
  decoy mktemp HOME. Output: all pass (Task 3 verify command).
- Input: before/after sha256 + mtime of the real ~/.ssh/config, ~/.ssh/config.d/gitid.config and
  ~/.gitconfig. Output: identical.
- Input `git log -2 --format='%s'`. Output: the fix(sshconfig) commit and the test(gitid) commit,
  both created with hooks enabled.
</verification>

<success_criteria>
- The GitHub `check` matrix (macos-15, macos-15-intel, ubuntu-latest) `make test (-race)` step would
  pass: the only CI failure (the diff3 test) is fixed, and the local-only HOME leak is fixed at its
  production root cause.
- No cmd/gitid production code resolves SSH Include paths against the process $HOME, and the AST
  guard enforces this.
- The diff3 test still proves ceremony provenance and WR-09 without any hardcoded literal in
  lifecycle.go.
- The SUMMARY surfaces the four out-of-scope findings: the fedora cgo/gcc job failure, the
  hasIncludeLine glob-vs-directory bug, the globalssh/ssh -G read-only home exposure, and the
  untouched real ~/.ssh/config.d/gitid.config.
</success_criteria>

<output>
Create `.planning/quick/260922-brh-fix-7-red-ci-tests-stale-diff3-source-gr/260922-brh-SUMMARY.md` when done.
</output>
