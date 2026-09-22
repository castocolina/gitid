---
id: 260922-cpl
slug: fix-masked-ci-failures-screenshot-region
phase: quick-260922-cpl
plan: 01
status: active
type: execute
wave: 1
depends_on: []
autonomous: true
cross_ai: true
requirements: [BUILD-02, PLAT-03, SSHUI-03, GIGN-01, DLV-04]
files_modified:
  - cmd/gitid/permission_premise_test.go
  - cmd/gitid/git_test.go
  - cmd/gitid/identity_test.go
  - cmd/gitid/wiring_test.go
  - cmd/gitid/upload_run_test.go
  - internal/screenshot/createflow.go
  - internal/screenshot/createflow_test.go
  - internal/tuikit/identities.go
  - internal/tuikit/identities_test.go
  - internal/screenshot/createflow_regions.go
  - internal/screenshot/createflow_regions_test.go
  - e2e/create_flow_pty_e2e_test.go
estimate:
  tokens: 110000
  raw_tokens: 110000
  tasks: 3
  confidence: low
must_haves:
  truths:
    - "`make test` passes locally, including its second line (the `-tags screenshot` internal/screenshot suite). That line is the step the GitHub `check` matrix fails on."
    - "`go test ./... -race -count=1` and `make lint` pass locally."
    - "As a normal user, TestGitFallbackShowUnreadableFileIsRefusal, TestIdentityDeleteRefusesWhenPlanFails and TestDeletePlanFailsOnUnreadableScanSource report `--- PASS` (not SKIP). As uid 0 (`unshare -r`, and a true-root fedora container when available) they report `--- SKIP` with a reason that names the bypassed premise."
    - "TestPlanUploadReadFileFailureRedactsHomePath passes both with `gh`/`glab` on PATH and with them absent. It no longer depends on the host having a provider CLI installed."
    - "At 100x30, with an upload-capable backend and a 7-line provider-marked Host block (the real-binary worst case), wizard step 0 shows `IdentitiesOnly yes`, the `# gitid: provider=` marker, and the preview box's bottom border. The frame is exactly 30 rows. Every D-07 reserved hint and all frozen copy are still rendered."
    - "The `gign-receipt` capture reaches the written-receipt state (its registry StateMarker `Global gitignore written` is present) and RegionGIGNCeremony is non-empty."
    - "`make gate-visual-regression` reports the same first error as the plan's base commit (`ggit-options-list` / `keybar`, a pre-existing failure CI does not run). No create-flow frame gains a new undeclared region difference."
    - "No command touches the real ~/.ssh or ~/.gitconfig. Their sha256 and mtime are identical before and after."
  artifacts:
    - "cmd/gitid/permission_premise_test.go: skipUnlessUnreadableFilesEnforced(t). It probes a mode-0000 temp file at runtime and skips only when the process can read it."
    - "cmd/gitid/upload_run_test.go: TestPlanUploadReadFileFailureRedactsHomePath stubs b.uploaderDeps.LookPath so the pub-key read failure path is always reached."
    - "internal/screenshot/createflow.go: the gign-receipt capture confirms with a single Enter on the Confirm-focused apply ceremony (no leading Tab)."
    - "internal/tuikit/identities.go: step 0 fits 25 body rows in the worst case. The duplicate standalone Key header row is removed. The key-source toggle is one physical row. The Port hint sits inline on the Port row."
    - "internal/screenshot/createflow_regions.go: extractReusePickerEntries starts only when the Reuse radio is the selected one."
    - "internal/tuikit/identities_test.go: TestWizardStep0WorstCaseRowBudgetKeepsFullHostPreview (worst-case 100x30 guard)."
    - "internal/screenshot/createflow_test.go: TestCaptureGitIgnoreReceiptReachesWrittenState."
    - "internal/screenshot/createflow_regions_test.go: TestExtractReusePickerEntriesOnlyInReuseMode."
  key_links:
    - "RenderFrame body budget: 30 rows - header(1) - breadcrumb(1) - status(1) - two-row keybar(2) = 25 body rows for the wizard pane. renderWizard step 0 -> sshForm.view + renderUploadCheckboxRow + renderKeyBody + renderHostBlockPreview(maxLines 7). Any row over 25 is clipped from the bottom, and the bottom of the pane is the Host preview."
    - "internal/sshconfig/renderer.go appends `# gitid: provider=<p>` as the LAST line of the Host block. So in the real binary `IdentitiesOnly yes` is the second-to-last content row and is the first directive lost when the pane overflows."
    - "CaptureGitIgnoreScreens gign-receipt -> keyEnter(review) -> ceremonyModel built by newApplyCeremony (focus = Confirm, quick 260919-jnl) -> GlobalGitIgnoreCommitMsg -> receipt text `Global gitignore written` -> extractGIGNCeremony / gignIsCeremonyFrame."
    - "extractReusePickerEntries anchor <-> renderKeyBody toggle text: dot glyph + space + `Reuse an existing key` (stripANSI of the selected Reuse option is `● Reuse an existing key`)."
    - "(*realBackend).planUpload -> uploader.DetectFor(provider, b.uploaderDeps) (uses deps.LookPath) -> b.uploaderDeps.ReadFile(req.PubPath) -> uploadFailureView(uploader.RedactCLIOutput(...)). DetectFor returning not-found short-circuits to a Skipped view with zero rows."
---

# Quick 260922-cpl: fix the CI failures that were hidden behind earlier red steps

**Goal:** GitHub CI on main is fully green. The `check` matrix is red on the second line of `make test`
(the internal/screenshot suite). The `fedora (container)` job is red on `make test` because it runs
as root without `gh`. Fix each at its root cause without weakening any test.

**All generated content is English only**: code, comments, test names, commit messages, SUMMARY.

**Code exploration rule (ONESHOT rule 13, restated for the cross-AI executor):** run
`codegraph index || codegraph init -i` once at task start. Use `codegraph_explore` (MCP) or
`codegraph explore "<symbols>"` (shell) BEFORE any grep/read loop. Fall back to `rg` (not `grep`)
plus targeted reads only if codegraph is unavailable. Line numbers below are approximate, so locate
code by symbol.

**Tracer-first: not applicable.** The plan fixes three independent root causes that share no layer.
Each task is a self-contained vertical fix whose `<verify>` runs the real failing command end to end.

## Planning-time evidence (live, 2026-09-22; base code = 295275a, HEAD ad31afa differs only in .planning docs)

CI run 35724594275 (main @295275a): build-cross green; `check` (ubuntu-latest, macos-15,
macos-15-intel) and `fedora (container)` red.

### Group A — `check` matrix, `make test` line 2 (`Makefile` ~436)

Both failures reproduce locally with real HOME, empty HOME, and at d20d4ea. They predate quick
260922-brh.

**A1. TestExtractRegion_HostPreview (createflow_test.go ~357). The render is the bug, not the extractor.**
- `git bisect` (good 2c26f7b, bad e156586) points to **e156586 `feat(09.7-01)`** as the first bad
  commit. That commit's message claims it keeps "the Host-block IdentitiesOnly line" on "the 100x30
  pane", but it did not.
- The pre-regression step-0 frame (fixture backend, which renders the Phase 9 upload row) used exactly
  **25 of 25** body rows, with no headroom. e156586 added 3 always-on rows: the Hostname hint (D-07),
  the Port hint (D-07), and a standalone bold `Key` header. The pane overflowed by 3. That clipped
  `IdentitiesOnly yes`, the reserved preview row, and the box's bottom border. The extractor then
  runs to the end of the screen.
- The design contract says clipping is a defect. 02-STYLE-SPEC.md §7 requires "Still fits with no
  clipping" at 100x30, and says `PreviewBlock` must never grow a pane beyond budget.
  09.7-CONTEXT D-08 requires measuring the combined row cost and trimming faint explanatory lines
  when it comes up short ("record the trade-off, don't silently drop content").
  recipes/ssh-config.recipe makes `IdentitiesOnly yes` mandatory.
- The existing tuikit guard `TestHostPreview100x30ShowsIdentitiesOnlyYes` stays green only because
  `press(t, a, "n")` never delivers the async UploadEligibilityMsg. That leaves one row free, and the
  stub's 6-line block leaves `IdentitiesOnly yes` as the very last visible row, with the bottom border
  still clipped. The real binary with GitHub renders the upload row AND the 7-line provider-marked
  block. **The worst case needs all 3 rows back.**
- The `renderHostBlockPreview` comment's "23 ≤ 25" arithmetic is stale. It omits the upload row and
  the key-source row wrap.

**A2. TestRegionDiffCoverage: frame "gign-receipt" region "gign-ceremony" empty. The capture sequence is stale.**
- The first bad commit is **3dfdb15 (quick 260919-jnl)**. It switched the gitignore apply ceremony to
  `newApplyCeremony`, which starts with **Confirm focused**. `CaptureGitIgnoreScreens` still does
  `keyEnter(keyTab(review))`. The Tab now moves focus to Cancel, and Enter cancels back to browse. The
  receipt frame was verified to be the browse view.
- Prototype: after dropping the Tab, the whole screenshot suite (except A1) passes, including
  TestRegionDiffCoverage over the full merged registry. `BuildRegionDiffs` returns on its first error,
  so this proves no other frame was hidden behind gign-receipt.

**A3. Prototype of the fix (scratch worktree, discarded; exact edits specified in Task 3).**
- Three trims recover exactly 3 rows (frame dump verified, 25/25 with the full 9-row box):
  (1) drop the standalone `Key` header row; (2) compact the key-source row's faint hint so the row
  fits one physical line at detailWidth 62 (60 cols); (3) render the Port hint inline on the Port row.
- Results: tuikit suite green, the screenshot suite green, and the worst-case guard is RED on the base
  render ("missing IdentitiesOnly yes", "missing # gitid: provider=github") and GREEN after.
- Side effect found: once the toggle is one physical row, `extractReusePickerEntries`'s anchor matches
  it in generate mode too. That anchor was written for the single-line toggle, and at base the wrap
  split it, so it never matched. In generate mode it then captures the algorithm catalog as "picker
  entries", which makes `make gate-visual-regression` report a new `ssh-form-filled` /
  `reuse-picker-entries` difference. The fix is to anchor on the selected Reuse radio
  (`● Reuse an existing key`). With it, the gate's first error returns to the base baseline
  (`ggit-options-list` / `keybar`). `reuse-manual-path` still extracts real picker candidates.

### Group B — `fedora (container)` job (runs as root; the image has no `gh`)

**B1. Three unreadable-file tests: root bypasses mode 0000.**
- TestGitFallbackShowUnreadableFileIsRefusal (git_test.go ~622), TestIdentityDeleteRefusesWhenPlanFails
  (identity_test.go ~726), and TestDeletePlanFailsOnUnreadableScanSource (wiring_test.go ~3942).
- Reproduced with the same messages and line numbers as CI, both as `unshare -r` namespace root
  (`unshare -r id -u` prints 0) and as true root in the cached `registry.fedoraproject.org/fedora:latest`
  image. All three PASS as the normal user.
- Repo-wide inventory: these are the only tests whose premise is an unreadable regular file
  (`rg '0o000' --type go -g '*_test.go'`, plus a full `go test ./...` as uid 0).
  internal/filewriter TestWriteRestoreOnError (read-only *directory* premise) already skips under
  root (`os.Geteuid() == 0`, the precedent). e2e/ has no unreadable-file premise.
- Under `unshare -r` only, the TestCustomSSHDirectivePlanShowsRealDiff / TestRunCustomSSHDirectiveWrite*
  family also fails (`ssh -G` exit 255). The cause is an emulation artifact: host-root-owned
  `/etc/ssh/ssh_config.d/*` show up as uid 65534 inside the namespace, and ssh refuses them ("Bad owner
  or permissions"). They pass in the real fedora container; CI lists only 4 failures. Do not "fix" them.
- Why a skip rather than running the container step as an unprivileged user: the test-side skip is the
  minimal robust option. It has existing precedent, needs no container user, chown, or
  setup-go/toolchain ownership changes, and the non-root `check` matrix (3 OSes) keeps running these
  tests.

**B2. TestPlanUploadReadFileFailureRedactsHomePath (upload_run_test.go ~269, `rows = []`) has nothing to do with root.**
- `planUpload` calls `uploader.DetectFor` before `ReadFile`. With no `gh` on PATH it returns a
  `Skipped` view with zero rows. GitHub-hosted runners and this machine have `gh`; the fedora image does
  not. Reproduced locally with a PATH mirror that omits only `gh`/`glab`. A full `go test ./...` with
  that PATH shows this is the ONLY host-`gh`-dependent test. The existing hermetic pattern is
  `b.uploaderDeps.LookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }`
  (cmd/gitid/identity_upload_test.go ~370).

**B3. True-root container command (network-free; repo and module cache mounted read-only; no SELinux relabel).**
Replace `<SCRATCH>` with a fresh `mktemp -d` directory. Requires podman plus the cached image and the
toolchain dir below. All three were verified present at planning time.

```
podman run --rm --network=none --security-opt label=disable \
  -v "$PWD":/src:ro \
  -v "$HOME/go/pkg/mod":/gomod:ro \
  -v <SCRATCH>:/gocache \
  -v "$HOME/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.linux-amd64":/goroot:ro \
  -e GOROOT=/goroot -e GOMODCACHE=/gomod -e GOCACHE=/gocache -e GOTOOLCHAIN=local \
  -e GOPROXY=off -e GOFLAGS=-buildvcs=false -e CGO_ENABLED=0 -e TERM=dumb \
  -w /src registry.fedoraproject.org/fedora:latest \
  sh -c 'id -u; /goroot/bin/go test -count=1 -v -run "TestGitFallbackShowUnreadableFileIsRefusal\$|TestIdentityDeleteRefusesWhenPlanFails\$|TestDeletePlanFailsOnUnreadableScanSource\$" ./cmd/gitid/'
```

At base this printed `0` followed by the three CI failures verbatim.

## Row-budget decision record (09.7 D-08 "Claude's Discretion": which faint lines to compact)

Worst-case step-0 accounting after the fix. The real binary with GitHub renders the upload row and a
7-line provider-marked block:
stepper 1 + chord hint 1 + SSH form 7 (alias, alias hint, SSH host, SSH host hint, real hostname,
hostname hint, port-with-inline-hint) + upload checkbox 1 + key-source row 1 + algorithm catalog 5 +
preview box 9 (border, 7 lines, border) = **25 of 25**.

| Trim | Rows | Why it is a compaction, not a content loss |
|------|------|--------------------------------------------|
| Remove e156586's standalone ` Key` header row in `renderWizard` step 0 | -1 | The key-source row already starts with a bold `Key` label. `renderKeyBody`'s doc makes it the ONE combined header row (02-STYLE-SPEC §7 row-budget trap), so the standalone header duplicated it. 09.7 G-2 is still met because the Key cluster still opens with a bold `Key` header. |
| Key-source row faint hint `(←/→ change)` -> `(←/→)` | -1 | The row was 67 cols at detailWidth 62 and wrapped mid-label ("Reuse an" / "existing key"). That contradicted the D-10/D2 "side by side, on the SAME line" intent. With the compacted hint the row is 60 cols. Both option labels stay verbatim, and the ←/→ affordance stays on the header line. Only the faint word "change" goes, which is the lever D-08 names. The Git step's match-strategy header keeps `(←/→ change)` verbatim because D2 pins it there (TestGitFormStrategyAlwaysExpandedWithHeaderHint). |
| Port hint `Default 22; 443 for alt-SSH` moves inline onto the Port row | -1 | The Port row already has an inline slot: validation errors, `digits only`, and the D-21 `altSSHHint` render "inline on the EXISTING row (no new row)". The hint becomes that slot's default, so it is always present on the same row. D-07's no-layout-jump goal holds, and now also holds on port errors. When the D-21 warning occupies the slot, the generic hint is not shown. The warning already says "22; alt-SSH 443 is provider-specific". |

Not touched: frozen copy (gate-copy-freeze list, the D5 chord hint), D-07 reserved rows for Alias /
SSH Host / Real hostname, the D-01 upload row, the KEY-01 five-row catalog, and the SSHUI-03 full
Host block.

**Phase 9.8 note:** 9.8 was inserted 2026-09-22 and is not yet planned. It will later rework wizard
key hints, the Shift chord line, footer generation, and receipt pages (9.8 D-00/D-12/D-16/D-22). This
quick fix restores the CURRENT contracts only and must NOT pre-implement any 9.8 decision.

## Out of scope. Surface these in the SUMMARY; do NOT fix them here.

1. **`make gate-visual-regression` is already red at base**: first error
   `frame "ggit-options-list" region "keybar" differs without a screen-specific declared disposition`.
   CI does not run it. This plan only guarantees it does not get worse.
2. **The `reuse-key-vs-generate` capture has been stale since the Phase 9 D-01 upload checkbox joined
   the focus order.** `tabN(m, 4)` now lands on the checkbox, and `keyRight` unchecks upload
   (`☐ Register…`) instead of selecting Reuse. Its StateMarker `Reuse an existing key` is always
   on screen, so no test catches it. `reuse-manual-path` reaches reuse mode only by accident. This
   needs its own quick task, because re-sequencing changes three registered frames.
3. **The fedora job's later steps** (`make lint`, then `make test-e2e` as root) have never run on CI.
   Run what can be run locally (Task 3). The post-push CI run is the final proof. Any CI-only e2e
   failure gets its own task.

<objective>
Fix the masked CI failures at their root causes. Per 09.7 D-08 (row budget), SSHUI-03 (live Host
preview) and 02-STYLE-SPEC §7 ("no clipping at 100x30"):
(1) make the four fedora-job failures hermetic: a runtime unreadable-file probe skip for the three
chmod-0000 tests, and a LookPath stub for the host-`gh`-dependent upload test (BUILD-02, PLAT-03);
(2) drive the gign-receipt capture through the Confirm-focused apply ceremony (GIGN-01, DLV-04);
(3) restore wizard step 0 to 25/25 rows in the real-binary worst case, and keep the reuse-picker
region contract correct once the toggle is one row (SSHUI-03, DLV-04).
The result is that both `make test` lines, the race suite, and lint pass, so CI on main can go green.

Purpose: CI must be green, and the recipe-mandatory `IdentitiesOnly yes` must be visible in the
wizard's live preview at the design-minimum geometry.
Output: three logical commits (one per task), each passing the pre-commit hooks.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@CLAUDE.md
@recipes/README.md
@recipes/ssh-config.recipe
@.planning/STATE.md
@.planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md
@.planning/phases/09.7-new-identity-wizard-consistency-fixes-visual-grouping-for-ke/09.7-CONTEXT.md
@.planning/phases/09.7-new-identity-wizard-consistency-fixes-visual-grouping-for-ke/09.7-UI-SPEC.md

Recipes check: no config shape changes. The fix restores the visibility of the recipe-mandatory
`IdentitiesOnly yes` directive in the live preview, and nothing gitid writes changes.

Relevant anchors (approximate; locate by symbol):
- internal/tuikit/identities.go: `renderWizard` step 0 (~5200-5212, the ` Key` header write between
  `renderUploadCheckboxRow()` and `renderKeyBody()`), `renderKeyBody` (~5040-5070),
  `sshForm.view` (~515-578, the port-line switch and the trailing Port helper block),
  `renderHostBlockPreview` (~1626-1636, stale budget comment), `helperLine`/`formFieldLine` (~462-480),
  `altSSHHint` (~269).
- internal/tuikit/identities_test.go: `identitiesApp`, `pressSeq`, `pressAndRun` (~23-71), and
  `TestHostPreview100x30ShowsIdentitiesOnlyYes` (~2213) for the 100x30 resize pattern.
  `stubBackend.HostBlockPreview` is in internal/tuikit/backend_stub_test.go (~427).
- internal/screenshot/createflow.go: `CaptureGitIgnoreScreens` (~2496-2545, `keyEnter(keyTab(review))`),
  `ScreenSpecRegistry`/`RequiredScreenSpecs`.
- internal/screenshot/createflow_regions.go: `RegionReusePickerEntries` doc (~67), and
  `extractReusePickerEntries` (~857).
- internal/screenshot/createflow_test.go: external package `screenshot_test`, imports
  `internal/dummytui`. internal/screenshot/createflow_regions_test.go: internal package `screenshot`.
  Both carry `//go:build screenshot`.
- cmd/gitid/upload_run_test.go (~259), cmd/gitid/identity_upload_test.go (~370, LookPath stub pattern),
  cmd/gitid/upload_run.go `planUpload` (~150-178).
- internal/filewriter/filewriter_test.go (~127): the root-skip precedent.
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Make the cmd/gitid tests hermetic for the fedora root container (unreadable-file probe skip + gh LookPath stub)</name>
  <files>cmd/gitid/permission_premise_test.go, cmd/gitid/git_test.go, cmd/gitid/identity_test.go, cmd/gitid/wiring_test.go, cmd/gitid/upload_run_test.go</files>
  <precondition>`unshare -r id -u` prints 0 (unprivileged user namespaces are enabled; verified at planning time).</precondition>
  <read_first>cmd/gitid/git_test.go (TestGitFallbackShowUnreadableFileIsRefusal), cmd/gitid/identity_test.go (TestIdentityDeleteRefusesWhenPlanFails), cmd/gitid/wiring_test.go (TestDeletePlanFailsOnUnreadableScanSource), cmd/gitid/upload_run_test.go (TestPlanUploadReadFileFailureRedactsHomePath), cmd/gitid/identity_upload_test.go (~370 LookPath stub), cmd/gitid/upload_run.go (planUpload), internal/filewriter/filewriter_test.go (~127 precedent)</read_first>
  <behavior>
    - RED (before any edit): as uid 0 (`unshare -r`), the three unreadable-file tests FAIL with the CI messages (git_test.go "unreadable file exit = &lt;nil&gt;, want 1"; identity_test.go "identity delete must refuse…"; wiring_test.go "DeletePlan must error…"). With `gh`/`glab` absent from PATH, TestPlanUploadReadFileFailureRedactsHomePath FAILS with "rows = [], want exactly 1 failed row".
    - GREEN: as uid 0 the three tests report `--- SKIP` with a reason naming the bypassed premise and the euid. As the normal user they report `--- PASS` (never SKIP).
    - GREEN: TestPlanUploadReadFileFailureRedactsHomePath passes both with and without `gh`/`glab` on PATH, and still asserts the HOME path is redacted.
  </behavior>
  <action>
Before editing anything, record the base commit (`git rev-parse HEAD`, needed for Task 3's
comparisons). Also record sha256 and mtime of ~/.ssh/config and ~/.gitconfig, noting "absent" for a
missing file. Then reproduce RED with the Task 1 verify commands and keep the outputs for the SUMMARY.

1. Create cmd/gitid/permission_premise_test.go. Use the same package clause as cmd/gitid/git_test.go.
   Add one helper, `skipUnlessUnreadableFilesEnforced(t *testing.T)`, which calls `t.Helper()`, then:
   - writes a probe file in `t.TempDir()` with mode 0o000 (`t.Fatalf` if the write fails);
   - registers a cleanup that restores 0o600;
   - attempts `os.ReadFile` on it.
   If the read SUCCEEDS, call `t.Skipf` with this message: "unreadable-file premise cannot hold here: a
   mode-0000 file is readable by this process (euid=%d; root or CAP_DAC_OVERRIDE bypasses file
   permissions) — this test still runs on the non-root CI check matrix", passing `os.Geteuid()`.
   If the read fails, return, and the test runs normally.
   Doc comment: this probes the actual premise instead of checking only for euid 0, so it also covers
   CAP_DAC_OVERRIDE without uid 0. It can never skip on a runner where mode 0000 is enforced. Name the
   fedora container job as the motivating case, and point to internal/filewriter TestWriteRestoreOnError
   as the directory-permission precedent. Annotate the ReadFile with the repo's
   `//nolint:gosec // <reason> (G304)` style (test-owned temp file).
2. Call `skipUnlessUnreadableFilesEnforced(t)` as the FIRST statement of
   TestGitFallbackShowUnreadableFileIsRefusal, TestIdentityDeleteRefusesWhenPlanFails, and
   TestDeletePlanFailsOnUnreadableScanSource. Change nothing else in those tests.
3. Re-run the inventory: `rg -n '0o000' --type go -g '*_test.go'`, and read every `os.Chmod` in
   *_test.go files. Any other test whose premise is an unreadable REGULAR FILE gets the same call. Leave
   internal/filewriter TestWriteRestoreOnError unchanged: it is a directory-permission premise and
   already skips under root. List the inventory result in the SUMMARY.
4. In TestPlanUploadReadFileFailureRedactsHomePath, right after `b := newBackendForHome(home)`, stub
   provider-CLI detection: set `b.uploaderDeps.LookPath` to a func that returns
   `"/usr/local/bin/" + name, nil`, the same pattern as cmd/gitid/identity_upload_test.go ~370.
   Add a comment explaining why: the test's premise is "provider CLI detected, then the pub-key read
   fails". Without the stub, `uploader.DetectFor` short-circuits to a Skipped view with zero rows
   whenever the host lacks `gh` (the fedora image), so the test silently depended on the host's PATH.
   Do NOT stub ReadFile, because the real missing-file read and its redaction are what this WR-05 test
   proves.
5. Run the GREEN checks (verify block). Then run the full suite as uid 0:
   `unshare -r env TERM=dumb SSH_AUTH_SOCK= go test -count=1 ./... -skip 'TestCustomSSHDirectivePlanShowsRealDiff|TestRunCustomSSHDirectiveWrite'`
   It must be green. The `-skip` excludes only the documented namespace-only ssh "Bad owner" artifact
   (Planning-time evidence B1).
   Best effort: run the true-root container command from Planning-time evidence B3 (it runs `-v`) and
   record whether it prints `0` and three `--- SKIP` lines. If podman, the image, or the toolchain dir
   is unavailable, record the exact error and rely on the `unshare -r` proof.
6. Commit (one logical commit; hooks must pass; never `--no-verify`). Message:
   `test(gitid): make cmd/gitid tests hermetic for the fedora root container`. The body names the 4
   tests, both root causes (root bypasses mode 0000; `gh` absent from the fedora image), the probe-skip
   design, and BUILD-02 / PLAT-03.
  </action>
  <verify>
    <automated>unshare -r env TERM=dumb SSH_AUTH_SOCK= go test -count=1 -v -run 'TestGitFallbackShowUnreadableFileIsRefusal$|TestIdentityDeleteRefusesWhenPlanFails$|TestDeletePlanFailsOnUnreadableScanSource$' ./cmd/gitid/ 2>&1 | tee /dev/stderr | grep -c -- '--- SKIP' | grep -qx 3 && env TERM=dumb SSH_AUTH_SOCK= go test -count=1 -v -run 'TestGitFallbackShowUnreadableFileIsRefusal$|TestIdentityDeleteRefusesWhenPlanFails$|TestDeletePlanFailsOnUnreadableScanSource$|TestPlanUploadReadFileFailureRedactsHomePath$' ./cmd/gitid/ 2>&1 | tee /dev/stderr | grep -c -- '--- PASS' | grep -qx 4 && M=$(mktemp -d) && IFS=: && for d in $PATH; do [ -d "$d" ] || continue; for f in "$d"/*; do b=${f##*/}; case "$b" in gh|glab) continue;; esac; [ -x "$f" ] && [ ! -e "$M/$b" ] && ln -s "$f" "$M/$b"; done; done; unset IFS; env PATH="$M" TERM=dumb SSH_AUTH_SOCK= go test -count=1 -run 'TestPlanUploadReadFileFailureRedactsHomePath$' ./cmd/gitid/</automated>
  </verify>
  <done>As uid 0 the three unreadable-file tests SKIP with the premise reason, and as the normal user all four tests PASS. The upload test passes without `gh`/`glab` on PATH. The full suite is green as `unshare -r` root (minus the documented ssh namespace artifact). The true-root container result is recorded. One commit passed the hooks.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Drive the gign-receipt capture through the Confirm-focused apply ceremony</name>
  <files>internal/screenshot/createflow.go, internal/screenshot/createflow_test.go</files>
  <read_first>internal/screenshot/createflow.go (CaptureGitIgnoreScreens and its comment block above the receipt capture), internal/tuikit/ceremony.go (newApplyCeremony), internal/screenshot/createflow_regions.go (extractGIGNCeremony, gignIsCeremonyFrame), internal/screenshot/createflow_test.go (imports and existing test style)</read_first>
  <behavior>
    - RED: new TestCaptureGitIgnoreReceiptReachesWrittenState fails on the current capture sequence. The receipt frame is the browse view: it lacks the gign-receipt StateMarker, and RegionGIGNCeremony is empty.
    - GREEN: the receipt frame contains the registry StateMarker for ScreenID "gign-receipt", and ExtractRegion(frame, RegionGIGNCeremony) is non-empty. TestRegionDiffCoverage passes over the full registry.
  </behavior>
  <action>
1. RED first. In internal/screenshot/createflow_test.go add TestCaptureGitIgnoreReceiptReachesWrittenState:
   - capture with `screenshot.CaptureGitIgnoreScreens(dummytui.NewFixtureBackend())` (`t.Fatalf` on error);
   - look up the spec whose ScreenID is "gign-receipt" in `screenshot.RequiredScreenSpecs()` (`t.Fatal`
     if absent), and take its StateMarker from the registry instead of duplicating a literal;
   - assert that `screenshot.StripANSIExported(frame)` contains that StateMarker;
   - assert that `screenshot.ExtractRegion(frame, screenshot.RegionGIGNCeremony)` is non-empty.
   Run it and confirm it FAILS.
2. In CaptureGitIgnoreScreens, change the receipt capture from Enter-after-Tab on `review` to a single
   `keyEnter(review)`. Rewrite the comment block above it. Per quick 260919-jnl (commit 3dfdb15),
   gitignore apply now uses `newApplyCeremony`, which starts with Confirm focused, so one Enter confirms.
   A leading Tab moves focus to Cancel, and Enter then cancels back to browse. That is the regression
   this fixes: gign-receipt had rendered the browse view, and RegionGIGNCeremony was empty. Keep the
   existing explanation of why there is no second Enter (Enter on the receipt finishes the ceremony).
   Do not change any other capture sequence. The stale `reuse-key-vs-generate` sequence is out of scope.
3. GREEN: run the verify command. Everything except TestExtractRegion_HostPreview must pass, because
   Task 3 fixes that one.
4. Commit: `fix(screenshot): drive gign-receipt through the Confirm-focused apply ceremony`. The body
   covers the root cause (3dfdb15), the new test, and GIGN-01 / DLV-04. Hooks must pass; never
   `--no-verify`.
  </action>
  <verify>
    <automated>go test -count=1 -tags screenshot -run 'TestCaptureGitIgnoreReceiptReachesWrittenState$|TestRegionDiffCoverage$' ./internal/screenshot/... && go test -count=1 -tags screenshot -skip 'TestCaptureTUI|TestCaptureHTML|TestProvisionPinnedChromium|TestExtractRegion_HostPreview' ./internal/screenshot/...</automated>
  </verify>
  <done>The gign-receipt capture shows the written receipt. The new test and TestRegionDiffCoverage pass. The rest of the screenshot suite (except the Task-3 test) passes. One commit passed the hooks.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Restore the wizard step-0 row budget (IdentitiesOnly yes visible at 100x30), keep the reuse-picker region contract, then run full local verification</name>
  <files>internal/tuikit/identities.go, internal/tuikit/identities_test.go, internal/screenshot/createflow_regions.go, internal/screenshot/createflow_regions_test.go, e2e/create_flow_pty_e2e_test.go</files>
  <read_first>internal/tuikit/identities.go (renderWizard step 0, renderKeyBody, sshForm.view, renderHostBlockPreview, helperLine, formFieldLine, altSSHHint), internal/tuikit/identities_test.go (identitiesApp, pressSeq, pressAndRun, TestHostPreview100x30ShowsIdentitiesOnlyYes, TestWizardSSHFormAlwaysReservesHostnameHint, TestWizardStep0HasKeyHeader), internal/tuikit/backend_stub_test.go (stubBackend.HostBlockPreview), internal/sshconfig/renderer.go (~17-42 provider marker is the LAST line), internal/screenshot/createflow_regions.go (RegionReusePickerEntries, extractReusePickerEntries), e2e/create_flow_pty_e2e_test.go (~1205-1220)</read_first>
  <behavior>
    - RED: new TestWizardStep0WorstCaseRowBudgetKeepsFullHostPreview fails on the current render ("IdentitiesOnly yes" and the provider marker are clipped, the toggle wraps, and the Port hint is on its own row).
    - RED: new TestExtractReusePickerEntriesOnlyInReuseMode fails on the current extractor (a one-line generate-mode toggle makes it capture the algorithm row).
    - GREEN: at 100x30 with an upload-capable backend and a 7-line provider-marked Host block, the frame is exactly 30 rows. It shows "Register with GitHub automatically", "IdentitiesOnly yes", "# gitid: provider=github", and the bottom border "╰" on the line right after the marker. "Generate a new key" and "Reuse an existing key" share one physical line. "Port" and "Default 22; 443 for alt-SSH" share one physical line. "The true SSH endpoint", "Blank prefix → SSH Host = the provider host itself", "Auto-joined: &lt;prefix&gt;.&lt;provider&gt; — editable", "Shift+→ next section · Shift+← exits the wizard" and "Step 1/4" are all present. The 30-row, marker/border and Port-line assertions still hold after three Tabs move focus onto Port.
    - GREEN: in generate mode the reuse-picker region is empty. In reuse mode it contains the candidate rows and excludes the toggle line.
    - GREEN: TestExtractRegion_HostPreview, TestHostPreview100x30ShowsIdentitiesOnlyYes, TestWizardSSHFormAlwaysReservesHostnameHint, TestWizardStep0HasKeyHeader and TestGitFormStrategyAlwaysExpandedWithHeaderHint all pass.
  </behavior>
  <action>
Tests first (RED). Run both new tests and confirm they fail before touching production code.

A. internal/tuikit/identities_test.go: add TestWizardStep0WorstCaseRowBudgetKeepsFullHostPreview.
   - Declare a test backend `providerMarkerBackend struct{ stubBackend }` whose HostBlockPreview returns
     `stubBackend{}.HostBlockPreview(spec)` plus a newline and `# gitid: provider=github`. This mirrors
     internal/sshconfig/renderer.go, which emits the marker as the LAST line, i.e. the real-binary
     7-line block.
   - Build `NewApp(providerMarkerBackend{})`, send `tea.WindowSizeMsg{Width: 100, Height: 30}` through
     Update (the resize pattern of TestHostPreview100x30ShowsIdentitiesOnlyYes), then call
     `pressAndRun(t, a, "n")` so the async UploadEligibilityMsg is delivered and the upload row renders.
     Do NOT drain commands recursively; textinput blink ticks would loop.
   - Split `ansi.Strip(a.View().Content)` on "\n" and assert every item in the behavior block. The first
     assertion is the "Register with GitHub automatically" precondition, which proves this is the worst
     case.
   - Then `pressSeq(t, a, "tab", "tab", "tab")` to focus Port, and re-assert the 30-row count, the
     marker/border adjacency, and the Port-plus-hint single line.
   - The doc comment explains the 25-row worst-case accounting (Row-budget decision record) and why this
     guard exists: the existing stub-backend guard never delivers the upload row, and its 6-line block
     hid the clipping.
B. internal/screenshot/createflow_regions_test.go: add TestExtractReusePickerEntriesOnlyInReuseMode.
   Use synthetic frames, joined with "\n", that mimic the wizard pane.
   - Generate-mode frame, three lines in this order:
     - a toggle line holding `Key (←/→)`, then `● Generate a new key`, then `○ Reuse an existing key`;
     - an algorithm row with `● ed25519 — ★ recommended`;
     - a preview-box top line starting with `╭╌ Live Host-block preview`.
   - Reuse-mode frame: the same three lines, except the toggle line uses `○ Generate a new key` and
     `● Reuse an existing key`, and the second line is a candidate row with `● id_ed25519_personal  ed25519`.
   - Assert `ExtractRegion(generateFrame, RegionReusePickerEntries)` is empty.
   - Assert the reuse-mode region contains `id_ed25519_personal` and does not contain `Generate a new key`.

Production (GREEN), per 09.7 D-08 and the Row-budget decision record:
C. renderWizard step 0: delete the standalone bold ` Key` header row that e156586 inserted between
   `w.renderUploadCheckboxRow()` and `w.renderKeyBody()`. Add a short comment there: the key-source row
   renders its own bold `Key` label, and that label is the Key cluster header (09.7 G-2, 02-STYLE-SPEC
   §7 ONE combined header row).
D. renderKeyBody: change the faint hint on the key-source row from `(←/→ change)` to `(←/→)`. Keep
   `Generate a new key` and `Reuse an existing key` verbatim. Extend its doc comment: the row must fit
   ONE physical line at detailWidth 62. It is 60 cols with the compacted hint and 67 with the long one,
   and the long one wrapped mid-label. This is the D-08 faint-line compaction. The Git step's
   match-strategy header keeps `(←/→ change)` because D2 pins it there.
E. sshForm.view: add a `default:` case to the existing port-line switch, after the unknownProvider case.
   It appends two spaces plus `styleFaint.Render("Default 22; 443 for alt-SSH")` to portLine. Delete the
   trailing block that wrote the same text as a separate helperLine row. Replace the stale comment above
   it ("focused-only helper … costs no extra row while blurred") with one that says:
   - the Port hint is the default of the Port row's existing inline slot (D-21 precedent);
   - it is always present (D-07, no layout jump, including on port errors) and costs zero rows;
   - the D-21 warning supersedes it, because that warning carries the same guidance.
   Update the sshForm.view doc comment, which still says descriptive helpers render only for the focused
   field (untrue since 09.7 D-07).
F. renderHostBlockPreview: replace the stale "23 ≤ 25" arithmetic in its comment with the measured worst
   case from the Row-budget decision record (25 of 25). Name TestWizardStep0WorstCaseRowBudgetKeepsFullHostPreview
   as the guard.
G. internal/screenshot/createflow_regions.go extractReusePickerEntries: the region starts only on a line
   whose ANSI-stripped text contains `● Reuse an existing key`, i.e. the Reuse radio is selected. Update
   the RegionReusePickerEntries doc: the rows between the toggle and the preview box when Reuse is the
   selected source. In generate mode those rows are the algorithm catalog, which RegionKeySection covers,
   so this region is empty there.
H. e2e/create_flow_pty_e2e_test.go (~1216-1218): comment only. The key-source toggle now renders on one
   physical row (quick 260922-cpl). Asserting the `existing key` tail keeps the check independent of the
   header-hint wording. Do not change the assertion itself.

Full local verification (record input command plus real output in the SUMMARY):
1. The verify command below: the new and affected tuikit tests, the full `make test` (both lines), and
   `make lint`.
2. `go test ./... -race -count=1`.
3. `make gate-visual-regression` is expected to fail. Its first reported error must be byte-identical to
   the base commit's (`frame "ggit-options-list" region "keybar" differs without a screen-specific
   declared disposition`). Confirm by running the same target in a scratch `git worktree` at the recorded
   base commit, then remove that worktree. Any other first error, especially a create-flow frame, is a
   regression: fix it before committing.
4. Run `make build`, then `go test -tags e2e -race -timeout 2400s -run 'TestCreateFlow' ./e2e/...` (the
   PTY suite that drives step 0). Then run the full `make test-e2e` once in the background and report
   it. For any e2e failure, re-run that single test at the base commit in a scratch worktree and classify
   it as pre-existing (fails at base too) or a regression (fix it).
5. Real-file safety: recompute sha256 and mtime of ~/.ssh/config and ~/.gitconfig and compare them with
   the values recorded before Task 1. They must be identical.
6. Commit: `fix(tui): restore the wizard step-0 row budget so the Host preview shows IdentitiesOnly yes`.
   The body covers:
   - the e156586 root cause and the bisect;
   - the three trims (the D-08 trade-off record);
   - the reuse-picker anchor fix and why the one-row toggle needs it;
   - the new tests, and SSHUI-03 / DLV-04.
   Hooks must pass; never `--no-verify`.
Write the SUMMARY with every input/output pair, the Row-budget decision record, the Task 1 inventory
result, and the three Out-of-scope items. State that the post-push CI run (check matrix plus the
fedora job's `make lint` and `make test-e2e` as root) is the final proof.
  </action>
  <verify>
    <automated>TERM=dumb SSH_AUTH_SOCK= go test -count=1 -run 'TestWizardStep0WorstCaseRowBudgetKeepsFullHostPreview$|TestHostPreview100x30ShowsIdentitiesOnlyYes$|TestWizardSSHFormAlwaysReservesHostnameHint$|TestWizardStep0HasKeyHeader$|TestGitFormStrategyAlwaysExpandedWithHeaderHint$' ./internal/tuikit/ && go test -count=1 -tags screenshot -run 'TestExtractReusePickerEntriesOnlyInReuseMode$|TestExtractRegion_HostPreview$|TestRegionDiffCoverage$|TestCaptureGitIgnoreReceiptReachesWrittenState$' ./internal/screenshot/... && make test && make lint</automated>
  </verify>
  <done>The worst-case 100x30 step-0 frame shows the full 9-row Host preview including `IdentitiesOnly yes`. Both `make test` lines, `go test ./... -race -count=1`, and `make lint` pass. The gate-visual-regression first error is unchanged from base. The e2e results are recorded and classified. The real ~/.ssh/config and ~/.gitconfig are unchanged. One commit passed the hooks. The SUMMARY is written.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| test process -> user's real HOME | Tests and verification commands must stay inside `t.TempDir()` homes and scratch dirs. The real ~/.ssh and ~/.gitconfig are out of bounds. |
| TUI render -> user's decision to write | The live Host-block preview is what the user approves before gitid writes ~/.ssh config (SSHUI-03). A clipped preview hides the safety-critical `IdentitiesOnly yes` directive. |
| host container runtime -> repository | The best-effort true-root podman proof mounts the repo and module cache. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-cpl-01 | Repudiation / Information disclosure | skipUnlessUnreadableFilesEnforced hides the "unreadable allowed_signers must abort delete" regression tests | medium | mitigate | The skip triggers only when a runtime probe proves a mode-0000 file is readable. Task 1 verify requires `--- PASS` (not SKIP) for all three as the normal user, and the non-root check matrix keeps running them. |
| T-cpl-02 | Tampering | Real ~/.ssh/config and ~/.gitconfig during test runs | high | mitigate | Every touched test uses a `t.TempDir()` HOME. sha256 and mtime are recorded before Task 1 and compared after Task 3 (must be identical). No command writes outside the repo, scratch dirs, or Go caches. |
| T-cpl-03 | Information disclosure | TestPlanUploadReadFileFailureRedactsHomePath (WR-05 HOME-path redaction) | medium | mitigate | Only LookPath is stubbed. The real ReadFile failure and RedactCLIOutput path still run, and the test still asserts the HOME path is absent from the Reason. |
| T-cpl-04 | Spoofing (misleading UI) | Wizard step-0 live Host preview clipped at 100x30 | high | mitigate | The row-budget trims restore the full 9-row box. TestWizardStep0WorstCaseRowBudgetKeepsFullHostPreview pins the real-binary worst case (upload row plus provider marker) so the directive cannot be silently clipped again. |
| T-cpl-05 | Tampering | Podman true-root proof mounting the repo | low | mitigate | `--network=none`, the repo and module cache mounted read-only, and `--security-opt label=disable` instead of `:Z`, so host SELinux labels are never rewritten. A scratch GOCACHE is the only writable mount. |
| T-cpl-06 | Denial of service (evidence integrity) | extractReusePickerEntries capturing the algorithm catalog as picker entries | low | mitigate | Anchor on the selected Reuse radio. TestExtractReusePickerEntriesOnlyInReuseMode pins both modes. The gate-visual-regression first error must equal base. |
</threat_model>

<verification>
- Task 1: uid-0 SKIP x3 and normal-user PASS x4; upload test green without `gh`/`glab`; full suite green under `unshare -r` (minus the documented ssh namespace artifact); true-root podman result recorded.
- Task 2: TestCaptureGitIgnoreReceiptReachesWrittenState and TestRegionDiffCoverage green.
- Task 3: the worst-case budget guard and reuse-picker mode test green; TestExtractRegion_HostPreview green; `make test` (both lines), `go test ./... -race -count=1` and `make lint` green; gate-visual-regression first error unchanged from base; `TestCreateFlow` e2e subset and full `make test-e2e` results recorded and classified.
- Real ~/.ssh/config and ~/.gitconfig: sha256 and mtime unchanged.
- The post-push GitHub CI run on main is the final proof. It is out of this plan's reach and must be stated as such.
</verification>

<success_criteria>
- `make test` (both lines) and `make lint` pass locally, which are exactly the steps the `check` matrix failed on.
- The four fedora-job failures pass (upload test) or skip with a precise reason (three unreadable-file tests) under uid 0, and all four still pass as a normal user.
- The recipe-mandatory `IdentitiesOnly yes` is visible in the wizard's live preview at 100x30 in the real-binary worst case. No test was weakened into a no-op, and no frozen copy or locked 09.7 decision was reverted.
- Three logical commits, each passing the pre-commit hooks, all content English only.
</success_criteria>

<output>
Create `.planning/quick/260922-cpl-fix-masked-ci-failures-screenshot-region/260922-cpl-SUMMARY.md` when done.
</output>
