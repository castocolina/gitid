# LEGACY-TRIAGE — gitid v1.0 autonomous completion

**Analyst pass:** 2026-07-08 · **Scope:** every Go package on `main` today.
**Author constraint:** this document only. No deletions, no commits, no code edits
were performed. The orchestrator ratifies each verdict and executes any deletion.

## Scope & framing

Phases 1–2 delivered PLANNING, frozen design mockups, demos, and CI/spike work.
**No shipped product code path is contractually frozen.** Therefore every Go
package on `main` — `cmd/*`, `internal/*`, and the POC `tui/` package — is 0.0.1-POC
inheritance and in triage scope. What sits on `main` is exactly what becomes
"legacy" from Phase 3 onward.

Two structural facts drive most verdicts:

1. **The real app shell is rebuilt from `internal/dummytui`, not from `tui/`.**
   Phase 3 D-17 extracts a shared, backend-free presentation package
   (`internal/tuikit`, does not exist yet) *out of `internal/dummytui`* — the
   frozen design — and wires the real backend behind it. The old `tui/` package
   (POC: `model.go`, `wizard.go`, `sidebar.go`, …) is **superseded wholesale**, not
   reused. Confirmed: `cmd/gitid/main.go` is the ONLY importer of `tui/`;
   `cmd/gitid-dummy/main.go` is the only importer of `internal/dummytui`.
2. **The POC command surface + POC TUI are still wired into the real binary.**
   `cmd/gitid/main.go` imports `tui` and registers the full POC Cobra tree
   (`add/list/test/rotate/update/delete/copy/baseline/doctor/debug/host/adopt/addrepo`).
   Phase 3 D-14 archives that command surface and D-15/D-17 rewires the entry to
   the new shell; Phase 5 SHELL-03 rebuilds the CLI. This teardown is **one atomic
   Phase-3 job** — it is why no "legacy" leaf package (e.g. `internal/repoclone`)
   can be cleanly deleted *now* in isolation without breaking the build.

`internal/version` (Phase 10 D-09) does **not exist yet** and is out of triage
scope (it is new code, not legacy).

---

## Cross-reference matrix (package × phase)

`R` = phase reuses as-is · `M` = phase modifies/extends · `X` = phase supersedes
(deletes when its replacement lands) · blank = not referenced by that phase.

| Package | P3 | P4 | P5 | P6 | P7 | P8 | P9 | P10 |
|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| `cmd/gitid` (POC cmds + entry) | X/M | | M | M | M | M | M | M |
| `cmd/gitid-dummy` | R/M | | | | | | | |
| `internal/dummytui` | M (extract tuikit) | R | R | M | M | M | M | |
| `internal/adopter` | R | R | | | | R | | |
| `internal/clipboard` | R | | | | | | R | |
| `internal/deps` | | | | | R | | | R |
| `internal/doctor` | | M (reserved) | M (reserved) | M (reserved) | M (reserved) | M | | |
| `internal/doctor/checks` | | | | M | | M | | |
| `internal/filewriter` | R | R/M | M | R | M | R | | |
| `internal/gitconfig` | | M | M | | M | M | | |
| `internal/identity` | R | | M | | | R | | |
| `internal/keygen` | R | R | R | | | | | |
| `internal/platform` | R | | | R | | | | R |
| `internal/repoclone` | X (drop) | | | | | | | |
| `internal/screenshot` | R (D-24 gate) | R | R | R | R | R | R | |
| `internal/sshconfig` | R | | M | M | | R | | |
| `internal/tester` | R | | R | R | | | R | |
| `internal/upload` | | | | | | | R | |
| `internal/uploader` | | | | | | | M | |
| `tui/` (POC TUI) | X | X | X | X | X | X | X | |
| `internal/version` (NEW, P10) | | | | | | | | new |

Evidence for every cell is in the CONTEXT.md files (each phase's `<code_context>` /
`Substrate` sections name the exact symbols) and cross-checked against the live
import graph. Detail rows below.

---

## Per-package triage table

| Package | Role evidence | Dependents (first-party) | Referenced by phases | Verdict | Owning phase(s) | Notes |
|---|---|---|---|---|---|---|
| `internal/clipboard` | `Copy(text)` cross-platform pbcopy/wl-copy dispatch; copy-`.pub` action | `cmd/gitid/{add,copy}.go`, `tui/{copy,deps}.go` | P3 (D-03), P9 | **keep** | — | Stable substrate; POC importers fall away with the P3 teardown but the pkg is unchanged. |
| `internal/deps` | Tool probing; `GitVersionAtLeast(maj,min)` (deps.go:47, gates signing at 2,36) | `cmd/gitid/*`, doctor | P7 (D-08 version gates), P10 | **keep** | — | Reused as-is; no new probe (P7 explicitly "do not add a new probe"). |
| `internal/keygen` | ed25519/rsa registry, `derive.go` (.pub derivation), `signers.go` (allowed_signers) | `cmd/gitid/*`, `internal/identity`, `tui/deps.go` | P3 (D-11), P4 (GITUI-04), P5 (rotate/new-key) | **keep** | — | Core lifecycle substrate; additive only. |
| `internal/platform` | OS/capability probe, `ProbeSSHVersion()` | `cmd/gitid/*`, `internal/sshconfig/renderer.go`, `tui/deps.go` | P3 (KEY-03), P6 (D-11/D-13), P10 | **keep** | — | Reused as-is. |
| `internal/tester` | Two-stage `ssh -T`/`ssh -G` test; PASS/ReachableNotUploaded/Failure by output substring; `-F` isolation (tester.go:162) | `cmd/gitid/{add,test,update}.go`, `internal/identity`, `internal/sshconfig/migrate.go`, `tui/*` | P3 (D-01..04), P5 (rotate/clone), P6 (D-01/D-04 probes), P9 (D-17 verify) | **keep** | — | Read-only; heavily reused. Confirmed reuse assumption holds. |
| `internal/identity` | Inventory + 8-state taxonomy (`state.go`); MGR-01/08 reconstruction; `key-used-*` ref-counts | `cmd/gitid/*`, `internal/sshconfig` tests, `tui/*` | P3 (D-18), P5 (D-12 ref-count), P8 | **keep** | (P5 additive) | Reused; P5 adds delete/ref-count paths but does not restructure. `delete.go:52` carries a hard-coded `"_global"` literal (see P6 finding). |
| `internal/adopter` | Existing-Include layout detection (SSH) + gitconfig-fragment adopt | `cmd/gitid/adopt.go`, `tui/{adopt,deps,model}.go` | P3 (D-05 auto-detect), P4 (fragment adopt), P8 (deferred adopt-offer) | **keep** | — | KNOWN NAMING TRAP (P4 CONTEXT): the SSH-layout detector vs the gitconfig-fragment adopt are distinct; don't conflate. Pkg kept; POC `tui/adopt.go` modal is separate (replace, see below). |
| `internal/screenshot` | `//go:build screenshot`-isolated PNG capture (freeze/headless-Chromium) driven by Makefile capture targets + its own capture tests | none (build-tag + Makefile + tests only) | P3 (D-24 visual gate), P4–P9 (each UI wave) | **keep** | — | Zero source importers is EXPECTED (build-tag isolation), NOT dead. It is the golden/visual-regression substrate every UI wave's DLV-04 gate runs on. |
| `cmd/gitid-dummy` | Fixture-driven demo binary: `tea.NewProgram(dummytui.NewApp())` | none | P3 (D-17: dummy injects fixtures) | **keep** | (P3 rewire) | Survives as the backend-free fixture skin over the extracted `tuikit`; import re-gated by the D-17 split (`gate-no-backend-files`). |
| `internal/upload` | `Instructions(provider)` manual-fallback text + provider substring-match convention | `cmd/gitid/{copy,upload}.go`, `tui/{copy,wizard}.go` | P9 (D-08 manual-fallback block; D-11 reuses match convention) | **keep** | — | Reused; the D-08 upload-section renders `Instructions`. Minor if any. |
| `internal/dummytui` | Frozen screen renderings, central Theme, shell frame, ceremony/fixture data — the raw material for the D-17 shared-package extraction | `cmd/gitid-dummy/*` | P3 (D-17 extract), P4/5 (R), P6–P9 (fixture additions) | **rework** | **P3** (extraction); P6–P9 (fixture edits) | P3 D-17 extracts `internal/tuikit` from it (both binaries then import tuikit). P7 D-08 amends fixtures (`GlobalGitFullManagedBlockText` ~data.go:604 diff3→zdiff3; strip text ~:572). P8 depends on `fixplans.go:43 ConfirmWord` + `data.go:790-833` sentinel-less flagship as fixture evidence. P9 adds upload states. |
| `internal/filewriter` | Timestamped backup + idempotent block rewrite + atomic temp→rename→chmod + `PrependBlockIfNotFound` (block.go:200-230); rollback (empty backupPath = did-not-pre-exist) | `internal/{sshconfig,gitconfig,identity}` | P3 (D-05..09), P4 (D-11 rollback), P5 (D-11), P7 (D-05) | **rework** | **P5**, **P7** (additive) | Core kept. P5 D-11 adds backup-of-deleted-file semantics. P7 D-05 adds an insert-after-floor primitive sibling to `PrependBlockIfNotFound`. No restructure. |
| `internal/gitconfig` | Fragment render/parse (`fragment.go`,`reader.go`,`renderer.go`), `baseline.go` (insteadOf, `WriteBaselineInclude`:610, `ScanConflicts`:70, `RenderBaselineBlock`:401, dormant `WriteGlobalGitignore`), includeIf resolve | `cmd/gitid/*`, `internal/identity`, `tui/*` | P4 (M), P5 (M), P7 (M), P8 (M) | **rework** | **P4**, **P7** (primary); P5, P8 (additive) | P4: refit `baseline.go` insteadOf → per-provider managed block (D-02); register it reserved (D-04). P7: rename baseline sentinel → `global-git` + register reserved (D-01); **demote `core.excludesfile` out of Tier-1 (`baseline.go:424`)** (D-11.2 latent contract break); zdiff3 fixture sync (D-08). P5: `RemoveURLRewritesBlock` needs per-provider granularity (D-09); `RemoveAllowedSignersLine` superstring guard reused. P8: dormant `WriteGlobalGitignore`/`DefaultGitignorePatterns` activated for FIX-01. |
| `internal/sshconfig` | Parse/render/managed blocks; `RenderGlobalBlock` (renderer.go:48-73), `writer.go` `_global` sentinel + identities-first ordering (T-02-15), `IsReservedBlockName`+`EnsureIncludeLine` flooring (include.go:37-93), `Adopt`, `migrate.go` | `cmd/gitid/*`, `internal/{identity,tester}`, `tui/deps.go` | P3 (R), P5 (M), P6 (M), P8 (R) | **rework** | **P6** (primary); P5 (additive) | P6 D-06: refactor `RenderGlobalBlock`+`writer.go` into a shared `EnsureGlobals()` (key-union merge, existing-values-win) so create-ceremony re-normalization can't erase GSSH fixes. P6 D-08: rename `_global`→`global-ssh`, register BOTH in `IsReservedBlockName`, **replace the 4 scattered `== "_global"` literals** (reader.go / overlap.go / migrate.go / identity/delete.go) with registry calls. |
| `internal/doctor` | Health engine: 9 families, `Severity`, `Finding`(IdentityName), `FixDescriptor` (doctor.go) | `cmd/gitid/doctor.go` | P4/5/6/7 (reserved-block registration — L4), P8 (re-home + extend) | **rework** | **P8** (primary); P4/5/6/7 (reserved registry, L4) | P8 D-01 adds `Finding.Target`/Section; D-06 tolerances; consumes reserved-PATH registry (does not exist yet — P8 builds it, D-06.2). **L4 obligation:** every phase that introduces a new managed block MUST register it reserved here or `--fix` enters the destructive false-positive loop. Current reserved set = SSH Include line + gitconfig baseline only. |
| `internal/doctor/checks` | Check impls: `orphans.go` (Class-1 orphan fix + reserved-skip guard), `coherence.go`, `redundancy.go` (consolidate-into-one-block advice :101-179) | `internal/doctor` | P6 (redundancy advice text → new name), P8 (new checks + downgrades) | **rework** | **P8** (primary); P6 (advice text) | P8 D-05 adds ~9 new checks; D-06.1 downgrades `CheckOrphans` Class-1 (git-only-delete state) from destructive-fix to info (removes last live false-positive-loop instance). P6 D-08 updates redundancy advice to the `global-ssh` name. |
| `internal/uploader` | `Detect`/`AuthCheck`/`UploadKey`/`CommandPreview` with `Deps` seam; `buildArgs` (shown==run); `GLabKeyTypeForAuth` | `cmd/gitid/copy.go`, `tui/{copy,deps,model}.go` | P9 (M) | **rework** | **P9** | D-11 `Detect`→`DetectFor(provider)` (fix cross-route correctness bug); D-12 glab `--usage-type auth_and_signing` (rename `GLabKeyTypeForAuth`); D-15 add inventory fn; D-16 `UploadKey`→per-registration result struct. |
| `cmd/gitid` | POC Cobra tree (add/list/test/rotate/update/delete/copy/baseline/doctor/host/adopt/addrepo) + `debug` + `tui.Run` entry | none (top-level main) | P3 (X POC cmds + rewire), P5 (rebuild CLI), P6–P10 (add cmds) | **replace** | **P3** (remove POC cmds, rewire entry D-14/D-15) + **P5** (rebuild SHELL-03) | Package survives; POC command FILES deleted by P3 D-14 (`debug caps` kept). New noun-verb CLI built P5; each later phase adds its subcommands + parity-matrix rows. `const version = "0.0.0-dev"` (main.go:15) → `internal/version` in P10 D-09. |
| `internal/repoclone` | Git-repo clone-into-managed-dir engine (the POC "add repo" feature) | `cmd/gitid/addrepo.go`, `tui/{addrepo,deps,model}.go` | **NONE of P3–P10** | **replace (drop)** | **P3** (teardown) | No phase 3–10 reimplements "add repo". MGR-04 "clone" is *identity* clone (copy+re-derive in the wizard, P5 D-14/15), NOT repo clone. This feature is dropped, not ported. See DROP section for the coupling that blocks a preemptive delete. |
| `tui/` (POC TUI) | Entire POC Bubble Tea app; superseded by the `dummytui`→`tuikit` extraction + real backend wiring | `cmd/gitid/main.go` only | P3–P9 (X, per screen) | **replace** | per file (below) | Split by screen file — verdict is uniformly replace, owning phase differs. See the tui/ split table. |

### `tui/` split by screen file (all **replace**; owning phase differs)

| File(s) | Role | Owning phase | Notes |
|---|---|---|---|
| `model.go`, `tui.go`, `keymap.go`, `sidebar.go`, `styles.go`, `palette.go`, `overlay.go`, `help.go`, `messages.go`, `deps.go`, `doc.go`, `scaffold_test.go`, `prove_test.go`, `render_bench_test.go`, `tui_stub_test.go` | POC app shell / chrome / navigation / injected-deps seam | **P3** | Replaced by the real app shell rendered from `internal/tuikit` (D-15/D-17). `deps.go` here is the POC `Build*Deps` seam — the real one is rebuilt in `tuikit` (L2: the new seam must be nil-guarded AND wired into the real constructor AND covered by a raw-keystroke PTY e2e). |
| `wizard.go`, `wizard_test.go`, `confirm.go`, `confirm_test.go` | POC create/add wizard + confirm ceremony | **P3** | Superseded by the live create flow (P3). |
| `detail.go`, `detail_test.go` | POC identity detail view | **P5** | Superseded by the SSH-first manager detail. |
| `globalopts.go`, `globalopts_test.go` | POC "Global Options" combined view | **P6** (SSH) + **P7** (Git) | The frozen design splits global options into view `2` (SSH, P6) and view `3` (Git, P7); the POC single view is replaced by both. |
| `health.go`, `health_test.go` | POC health/doctor view | **P8** | Superseded by Health (view `4`) + Fixer (view `5`). |
| `copy.go`, `copy_test.go`, `upload_test.go` | POC copy-`.pub` + per-key Enter/skip prompt queue | **P9** | **Explicit rework/replace target:** P9 D-02/D-10 replace the per-key prompt queue (it contradicts UP-03 autonomy) with the shared upload-section component. This is the orchestrator's flagged "known rework obligation." |
| `adopt.go`, `adopt_test.go` | POC interactive adopt modal | **P3** (teardown) | No frozen v1.0 screen for an interactive adopt modal; `internal/adopter` (the engine) is kept, but this POC modal is dropped in the teardown. P8 keeps adopt as a *deferred* affordance, not this UI. |
| `addrepo.go`, `addrepo_test.go` | POC add-repo modal | **P3** (teardown, drop) | Part of the dropped "add repo" feature (see `internal/repoclone`). |

---

## DELETE candidates (orchestrator to execute) — NONE safe *now*

**No package qualifies for a clean, standalone delete in the triage commit.**

The verdict rule for `delete` is "dead code, zero first-party dependents, safe to
remove NOW." The only genuinely unreferenced *feature* on `main` is the POC
"add repo" surface (`internal/repoclone` + `tui/addrepo.go` + `cmd/gitid/addrepo.go`),
which **no phase 3–10 reimplements**. But it does NOT have zero first-party
dependents today:

```
internal/repoclone  ← cmd/gitid/addrepo.go        (POC Cobra cmd, still registered in main.go)
internal/repoclone  ← tui/addrepo.go              (POC modal)
tui/addrepo.go      ← tui/model.go, tui/deps.go   (POC shell wires the modal)
```

Deleting `internal/repoclone` now would break the build (CLAUDE.md requires every
commit to compile and pass hooks over the whole module). Removing `tui/addrepo.go`
requires surgery on `tui/model.go` + `tui/deps.go` — the exact POC shell Phase 3
replaces wholesale. Doing it preemptively is wasted, risky work.

**Recommendation:** route this as a **DROP folded into the Phase 3 teardown**
(D-14 command archival + D-17 shell replacement), explicitly flagged so the Phase 3
planner *drops* the add-repo feature rather than porting it. Zero-dependents proof
for the orchestrator to re-verify: `grep -rl 'internal/repoclone' --include='*.go'`
returns only `cmd/gitid/addrepo*.go` and `tui/addrepo*.go` — all POC surfaces slated
for removal; no phase CONTEXT.md mentions `repoclone`, `addrepo`, or "add repo".

If the orchestrator wants a delete in the triage commit anyway, the only build-safe
unit is the whole coupled set removed together — which is effectively the start of
Phase 3's teardown, not an independent triage delete.

---

## Rework / Replace obligations by phase (planner hand-off)

Each phase's planner should be handed its rows below as binding inputs.

### Phase 3 (Create Flow Backend) — primary teardown owner
- **replace** `cmd/gitid` POC command files (D-14): remove add/list/test/rotate/update/delete/copy/baseline/doctor/host/adopt/addrepo; keep `debug caps`; rewire entry to the real shell (D-15).
- **replace** entire `tui/` shell + create/confirm screens (`model.go`,`sidebar.go`,`wizard.go`,`confirm.go`, chrome files) with the `tuikit`-rendered real app.
- **rework** `internal/dummytui`: extract `internal/tuikit` (D-17); restore the dummytui no-backend import-graph test (W2 carry-over) and re-shape the `gate-no-backend-files` allowlist for the new split.
- **replace (drop)** `internal/repoclone` + `tui/addrepo.go` + `cmd/gitid/addrepo.go` + `tui/adopt.go` (add-repo & interactive-adopt POC features; no v1.0 reuse).
- **L2:** the new `tuikit` `Build*Deps()` seam MUST be nil-guarded, wired into the REAL constructor, and covered by a raw-keystroke PTY e2e (the D-22 PATH-shim harness) — the recurring injected-seam wiring blindspot.

### Phase 4 (Git Configuration Screen)
- **rework** `internal/gitconfig` `baseline.go`: refit insteadOf → one per-PROVIDER managed block (D-02).
- **L4 (MANDATORY):** register the insteadOf managed block as doctor-reserved (D-04) or `--fix` enters the destructive false-positive loop.
- **replace** `tui/detail.go`-adjacent git surfaces via the new reusable git-form flow in `tuikit` (create + edit modes, D-06).

### Phase 5 (Identity Manager)
- **replace** `tui/detail.go` (identity detail) with the manager list/detail.
- **rework** `internal/filewriter`: add backup-of-deleted-file (D-11).
- **rework** `internal/gitconfig`: `RemoveURLRewritesBlock` per-provider granularity for ref-counted delete (D-09); reuse `RemoveAllowedSignersLine` superstring guard.
- **L4:** register `~/.ssh/gitid-archive/` as doctor-reserved (D-06) — note this is a reserved-PATH, and the path registry does NOT exist yet (P8 D-06.2 builds it); P5 must at minimum record the obligation.
- **replace** `cmd/gitid` CLI: rebuild noun-verb tree (SHELL-03) + parity matrix.

### Phase 6 (Global SSH Options)
- **rework** `internal/sshconfig`: refactor `RenderGlobalBlock`+`writer.go` into shared `EnsureGlobals()` key-union merge (D-06) so create re-normalization can't erase GSSH fixes.
- **rework** `internal/sshconfig`: rename `_global`→`global-ssh`, register BOTH in `IsReservedBlockName`, replace the 4 scattered `== "_global"` literals with registry calls (D-08) — confirmed live in `reader.go`/`overlap.go`(:29)/`migrate.go`/`identity/delete.go`.
- **rework** `internal/doctor/checks/redundancy.go`: update advice text to the `global-ssh` name.
- **replace** `tui/globalopts.go` (SSH half) with view `2`.

### Phase 7 (Global Git Options)
- **rework** `internal/gitconfig`: rename baseline sentinel → `global-git` (register reserved, D-01); **demote `core.excludesfile` out of Tier-1 at `baseline.go:424`** (D-11.2 latent contract break — real write must match the frozen block byte-for-byte).
- **rework** `internal/dummytui/data.go`: amend `GlobalGitFullManagedBlockText` (~:604) + strip text (~:572) diff3→zdiff3 in ONE commit, before copy freeze (D-08).
- **rework** `internal/filewriter/block.go`: add insert-after-floor primitive (D-05).
- **L4:** register the D-05 fallback block reserved.
- **replace** `tui/globalopts.go` (Git half) with view `3`.

### Phase 8 (Health + Fixer)
- **rework** `internal/doctor`: add `Finding.Target`/Section (D-01); build the reserved-PATH registry (D-06.2) so `~/.ssh/gitid-archive/` isn't swept; D-14 convergence alarm (upgrade silent `convergeFixes` stop at `cmd/gitid/doctor.go:185`).
- **rework** `internal/doctor/checks`: ~9 new checks (D-05); downgrade `CheckOrphans` Class-1 for the git-only-delete state (D-06.1).
- **rework** `internal/gitconfig`: activate dormant `WriteGlobalGitignore`/`DefaultGitignorePatterns` for FIX-01.
- **replace** `tui/health.go` with Health (view `4`) + Fixer (view `5`).
- **L4:** register the gitignore managed block reserved.

### Phase 9 (Upload / Credentials Assist)
- **rework** `internal/uploader`: `Detect`→`DetectFor` (D-11, fixes cross-route correctness bug); glab `--usage-type auth_and_signing` (D-12); inventory fn (D-15); `UploadKey`→per-type result struct (D-16).
- **replace** `tui/copy.go` per-key prompt queue with the shared upload-section component (D-02/D-10) — the orchestrator's flagged known rework obligation.
- **rework** `internal/dummytui`: add upload-section states.
- **L2:** the new inventory fn + `DetectFor` join the nil-guard wiring test (`tui/wiring_test.go` successor in `tuikit`).

### Phase 10 (Linux Validation + Release)
- **rework** `cmd/gitid/main.go`: `const version` (main.go:15) → new `internal/version` package (D-09) — NEW code, not legacy.
- **keep/reuse** `internal/deps`, `internal/platform` (validation targets), `internal/screenshot` (gate).

---

## Divergences & findings (CONTEXT-vs-evidence + recipes/)

1. **`tui/copy.go` rework assumption is coherent but sequencing-sensitive.**
   The orchestrator's brief and P9 CONTEXT both name `tui/copy.go` as a Phase 9
   rework target. Evidence check: `tui/copy.go` is part of the POC `tui/` package
   that **Phase 3 replaces wholesale** (the real UI is rebuilt in `tuikit`, not by
   editing `tui/`). So by the time Phase 9 runs, the *file* `tui/copy.go` will not
   exist — what Phase 9 actually reworks is the **upload/copy behavior** now
   embodied in `tui/copy.go`, re-homed into the `tuikit` upload-section component.
   **Correction for the P9 planner:** treat "`tui/copy.go`" as the *behavioral
   contract to replace*, not a file to patch; the replacement lands in `tuikit`.
   Not a blocker — the P9 CONTEXT D-10 already says "redesigned in the contract
   before backend work, not patched in code review," which is consistent once the
   file→component re-homing is made explicit.

2. **"Add repo" feature has no v1.0 home (repoclone drop).** `internal/repoclone`
   and the addrepo command/modal implement clone-a-repo-into-a-managed-dir. No
   phase 3–10 references it; MGR-04 "clone" is identity clone. This is a **feature
   drop**, and Phase 3's teardown must explicitly drop (not port) it. Flag for the
   Phase 3 planner so it isn't accidentally preserved as "substrate."

3. **L4 reserved-registry debt is spread across P4–P8.** The current reserved set
   is only the SSH Include line (`sshconfig.IsReservedBlockName`) + the gitconfig
   baseline. Every new managed block introduced downstream MUST be registered in
   the same phase: insteadOf (P4 D-04), gitid-archive PATH (P5 D-06 / P8 D-06.2
   builds the path registry), global-ssh + legacy `_global` (P6 D-08), global-git +
   fallback pair (P7 D-01/D-05), gitignore (P8 D-05). The 4 live `== "_global"`
   string literals (P6 D-08 target) are confirmed in `reader.go`, `overlap.go:29`,
   `migrate.go`, `identity/delete.go`. Missing any registration re-opens the
   destructive `--fix` false-positive loop.

4. **Latent contract break already on `main` (P7 to pay).** `internal/gitconfig/
   baseline.go:424` emits `core.excludesfile` unconditionally (Tier-1) while the
   frozen fixture block omits it. P7 D-11.2 must demote it or the real global-git
   write won't match the frozen block byte-for-byte (fails the DLV-04 golden gate).
   Surfaced here so it isn't discovered mid-wave.

### recipes/ divergences (canonical config shape)

5. **`Host *` block placement (P6 D-09).** The recipe shows the global `Host *`
   block near the TOP of `~/.ssh/config`; gitid writes it LAST inside its managed
   region (identities-first, T-02-15 invariant). This is a **deliberate, documented
   divergence**: recipe top-placement is inert (no key collides with a host block),
   but gitid's identities-first ordering is the only ordering that keeps per-alias
   `IdentityFile`/`IdentitiesOnly` values winning under OpenSSH first-match-wins.
   Correct as-is; already documented in P6 CONTEXT. No action beyond keeping the
   documented note.

6. **`hasconfig:` https variant dropped (P4 D-09).** The recipe includes both
   `hasconfig:remote.*.url:git@<host>:*/**` AND an `https://` variant per identity.
   gitid writes the SSH pattern only (the https variant is dead weight given the
   default-ON insteadOf URL rewriting, and matches the approved demo). Deliberate
   simplification of the recipe — documented in P4 CONTEXT. Correct as-is.

7. **`IdentitiesOnly` scope (P6 D-10).** Recipe and gitid agree: `IdentitiesOnly
   yes` is written PER-ALIAS, never on `Host *` (global scope would break non-gitid
   hosts). gitid conforms; the P6 row is verify/informational only. No divergence —
   noted for completeness because it is the project's core failure mode.

8. **Key algorithm (README caveat, universal).** Recipes use RSA (`id_rsa_*`);
   gitid uses one ed25519 key per identity (auth + signing via `gpg.format=ssh` +
   `allowed_signers`). Take structure, not key type — already the standing rule.
   No package diverges from this; keygen is ed25519-first with rsa-4096 in the
   probe-gated catalog.

---

## Summary tally

- **Packages inventoried:** 20 (`go list ./...`) + `tui/` split into 8 screen-file
  groups. `internal/version` (P10) noted but out of scope (does not exist).
- **keep:** 10 — `clipboard`, `deps`, `keygen`, `platform`, `tester`, `identity`,
  `adopter`, `screenshot`, `cmd/gitid-dummy`, `upload`.
- **rework:** 7 — `dummytui`, `filewriter`, `gitconfig`, `sshconfig`, `doctor`,
  `doctor/checks`, `uploader`.
- **replace:** 3 packages — `cmd/gitid`, `internal/repoclone` (drop), `tui/`
  (whole POC package; per-file owning phases P3/P5/P6/P7/P8/P9).
- **delete (safe now):** 0 — the one dead feature (repoclone/addrepo) is coupled to
  the still-wired POC surface; folded into the Phase 3 teardown as a drop.
