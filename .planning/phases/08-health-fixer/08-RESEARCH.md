# Phase 8: Health + Fixer - Research

**Researched:** 2026-08-27
**Domain:** Re-homing an existing Go health-check engine (`internal/doctor`) into two new Bubble Tea v2 TUI screens + a new Cobra CLI surface, with one deliberately scoped destructive write path.
**Confidence:** HIGH (all core claims verified by reading live source this session; a small number of architectural decisions are explicitly flagged LOW/open — see Assumptions Log)

## Summary

Phase 8 is **not greenfield**. `internal/doctor` (9 check families, `Finding`/`Deps`/`FixDescriptor` types) is a complete, well-tested, UI-free engine that already exists and has never been wired into the live `cmd/gitid` binary or `internal/tuikit`. The **entire CLI command layer that used to drive it** (`newDoctorCmd`, `runDoctor`, `convergeFixes`, `applyFixes`, `buildDoctorDeps`) was archived wholesale with the 0.0.1 POC in Phase 3 (D-14) and lives only under `.planning/archive/0.0.1-poc-product-features-in-tui/cmd-gitid/doctor.go` — **08-CONTEXT.md's canonical_refs citing `cmd/gitid/doctor.go:185`/`:607` point at a file that does not exist in the live tree.** This is the single most important correction this research makes: Phase 8 must **build a new CLI/wiring layer from scratch** (informed by, not literally extending, the archived file), not "extend" an existing one.

A second major architectural fact, also unrecorded in 08-CONTEXT.md/08-UI-SPEC.md: the real backend (`cmd/gitid/wiring.go:667-695`, `realBackend.InitialState()`) **already synthesizes `tuikit.DemoFinding` values today** — but from `identity.BuildInventory`'s simpler 5-value `Problem` taxonomy (`internal/identity/state.go`), not from `internal/doctor.Run()`. This is a *second, independent* per-identity findings pipeline that already feeds `state.Findings`, sets no `Section`, and is what `orderedFindings`/`groupFindings` in `internal/tuikit/doctor.go` render today (behind the still-live `DemoBanner(TabDoctor) == true`). Phase 8's real backend wiring must decide how `internal/doctor.Run()`'s richer, family-based findings (which is what D-05's whole 8-check queue and D-01's `Target` field assume) converge with — or replace — this already-shipping `identity.Problem`-sourced feed. This is the central "what genuinely still needs new code" fact the UI-SPEC's own Known-Divergence sections do not cover, because they analyze only the render layer, not the data layer.

Everything else 08-CONTEXT.md and 08-UI-SPEC.md already establish is verified accurate and is not re-litigated here: the `internal/tuikit` render substrate (`doctor.go`, `ceremony.go`, `identities.go:fixCeremonyFor`) is real, tested, and directly reusable; the D-09/D-11 flagship fixture (`FixerFixPreviewLines`, `ConfirmWord: "clientb.github.com"`) is byte-verified in this session; and the Known Divergence #1 (`TabID` 4→5) and #2 (ceremony already compressed) findings in `08-UI-SPEC.md` are both confirmed correct against the current `frame.go`/`app.go`/`ceremony.go` source.

**Primary recommendation:** Treat Phase 8 as two coupled but separable efforts — (1) a **new backend-wiring layer** (`internal/doctor.Run()` wired into `realBackend`, a new `internal/doctor.Finding.Target`/`Section` field, a new CLI command tree built fresh) that converges with the already-shipping `identity.Problem` findings feed, and (2) a **render-layer split** (`TabDoctor` → `TabHealth`+`TabFixer`, porting `doctor.go`'s logic largely unchanged per Known Divergence #1/#2). Do the wiring-layer tracer plan FIRST (same TDD-tracer pattern every prior phase used) so the planner does not discover the two-findings-pipeline conflict mid-wave.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Health check execution (parse gates, coherence, orphans, signing, agent, redundancy, overlap, baseline) | Domain Core (`internal/doctor` + `internal/doctor/checks`) | — | UI-free, already built, DLV-07 TDD core; must stay backend-free per the tuikit no-backend-import gate |
| New-checks-queue detectors (shadow simulation, author resolution) | Domain Core (`internal/globalssh`, `internal/globalgit`) | `internal/doctor/checks` (new check wrappers) | Phase 6/7 already built and tested the probes (`globalssh.Verify`, `globalgit.VerifyAuthorResolution`); Phase 8 wraps them as new `CheckFn`s, does not reimplement |
| Per-finding fix descriptors + Interactive fix callables | Domain Core / CLI wiring boundary (`internal/doctor.FixDescriptor.Fn`) | CLI Command Layer | doctor core never calls `os.Chmod`/`filewriter` directly (D-01 in REQUIREMENTS' historical doc); the cmd layer injects the callables |
| `gitid health` / `gitid fix` command surface, exit codes, `--identity` filter | CLI Command Layer (new `cmd/gitid/health.go`/`fix.go`, replacing the `newReservedNounCmd` stubs) | — | `health`/`fix` are currently RESERVED noun stubs (`cmd/gitid/main.go:105-106`) with zero real implementation — build fresh |
| Health screen render (list+detail, read-only) | TUI Render (`internal/tuikit`, new Health screen model) | — | Forks from `doctor.go`'s existing list+detail render, strips `f`/`F` keys (08-UI-SPEC.md, verified against `doctor.go:154-246`) |
| Fixer screen render + ceremony (typed confirm, batch queue) | TUI Render (`internal/tuikit`, new Fixer screen model) | Domain Core (backup/restore, re-verification) | `fixCeremonyFor`/`PlanFor`/`doctorBatch` already implement the interaction shape (verified `identities.go:2487-2499`, `fixplans.go:30-67`, `doctor.go:29-32`) |
| Convergence re-scan after each applied fix (D-13) | CLI/Backend wiring boundary (`realBackend.Persist` for `FixFinding`, new `runDoctor`-equivalent) | — | Currently `FixFinding`/`MarkScanned` are DEMO-ONLY in `realBackend.Persist` (`wiring.go:788-795`) — real wiring is 100% new code |
| Backup + atomic write + idempotent re-write of managed blocks | Domain Core (`internal/filewriter`) | — | Existing chokepoint, reused unchanged (STORE-04 invariant) |
| Surgical single hand-written-directive rewrite (D-09) | Domain Core (new `sshconfig`-level directive rewrite, e.g. `IdentitiesOnly no→yes`) + TUI ceremony (typed confirm) | CLI (`gitid fix`) | Genuinely NEW write primitive — no existing function rewrites one directive's VALUE in place outside a managed block; this is the phase's one true "new hand-roll," scoped by design (see Common Pitfalls) |
| Storage of findings signature / re-offer exclusion (D-14 convergence alarm) | Domain Core or CLI wiring (in-memory this session; no sidecar DB) | — | No existing persistence — `findingsSignature` pattern from the archived POC (`doctor.go:167-177`) is the reusable ALGORITHM, not reusable CODE (file is archived) |

## Standard Stack

No new external dependencies are required for this phase. Phase 8 is 100% internal-package wiring + one new Cobra command group, on top of the existing Go 1.26 / Bubble Tea v2 / Cobra v1.10.2 stack already pinned project-wide (`go.mod`, verified `go 1.26` at `go.mod:3`, `[VERIFIED: go.mod]`).

### Core (existing, reused unchanged)
| Package | Role in Phase 8 | Why Standard |
|---------|------------------|--------------|
| `internal/doctor` | 9-family check engine, `Finding`/`Deps`/`FixDescriptor` types | Already built, UI-free, TDD (`internal/doctor/doctor_test.go`, 265 lines); `[VERIFIED: internal/doctor/doctor.go:1-330]` |
| `internal/doctor/checks` | 9 check functions (deps, perms, coherence, orphans, signing, agent, baseline, overlap, redundancy) | Already built and independently tested (`orphans_test.go` 471 lines, `coherence_test.go` 372 lines, etc.) — `[VERIFIED: file listing]` |
| `internal/identity` (`state.go`, `inventory.go`) | `Problem`/`Severity`/`IdentityHealth`/`Classify`/`BuildInventory` — the ALREADY-WIRED per-identity findings feed | `realBackend.InitialState()` already consumes this (`wiring.go:667-695`); `[VERIFIED: internal/identity/state.go:100-131]` |
| `internal/tuikit` (`doctor.go`, `ceremony.go`, `identities.go`, `frame.go`) | Render substrate: master-detail helpers, `ceremonyModel`, `fixCeremonyFor`, `groupFindings`/`orderedFindings` | Directly reusable per 08-UI-SPEC.md Known Divergence #1/#2, verified against live source this session |
| `internal/globalssh` (`shadow.go`) | `Verify(deps, keys) ShadowResult` — the shadow-simulation probe D-05's "shadowed option" check reuses | `[VERIFIED: internal/globalssh/shadow.go:293]` (function signature read directly) |
| `internal/globalgit` (`authorresolve.go`) | `VerifyAuthorResolution(deps, matchedDir, unmatchedDir) (AuthorResolution, error)` — the author-resolution probe D-05's check reuses | `[VERIFIED: internal/globalgit/authorresolve.go:54]` |
| `internal/gitconfig` (`baseline.go`) | `WriteGlobalGitignore`, `DefaultGitignorePatterns` — D-05's gitignore fix substrate | `[VERIFIED: internal/gitconfig/baseline.go:264, :473]` |
| `internal/sshconfig` (`include.go`) | `IsReservedBlockName`, `ArchiveDir`, `ArchiveDirName` | `[VERIFIED: internal/sshconfig/include.go:54-96]` |
| `github.com/spf13/cobra` v1.10.2 | New `gitid health`/`gitid fix` command tree | Already the project's CLI framework (CLAUDE.md stack table) |
| `charm.land/bubbletea/v2` v2.0.7 / `charm.land/lipgloss/v2` v2.0.3 | TUI render (unchanged) | Existing project stack |

### Supporting
| Package | Purpose | When to Use |
|---------|---------|-------------|
| `internal/filewriter` | Atomic write, timestamped backup, idempotent block replace/remove | Every fixer write (D-10's mandatory verification loop wraps this) |
| `internal/keygen` (`archive.go`) | `ArchiveDirName`/archive path helpers, already 0700-enforced | D-06.2's reserved-path registry references this same archive concept |
| `os/exec` (stdlib) | `ssh -G` / `git config --show-origin` re-verification calls | D-10's mandatory post-apply proof, mirrors Phase 6/7's own probe pattern |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Reusing `internal/doctor`'s existing `Deps`-injection design for the new checks | A parallel, simpler ad-hoc check runner | Rejected — `Deps`/`CheckFn` is proven, tested, and D-05's 8 new checks are all expressible as ordinary `CheckFn`s; a parallel runner would fragment the family/severity vocabulary the TUI already renders |
| Building the archive-path exclusion fresh for the doctor's `KeyPaths` field | Copying `identity.BuildInventory`'s `filterReservedKeyPaths`/`IsReservedKeyPath` pattern | The identity package's pattern (`inventory.go:113-129`) is the right model to mirror — apply the SAME "exclude at the causal collection point" discipline to however `doctor.Deps.KeyPaths` gets populated for the new cmd-layer wiring |

### Installation

No `go get`/`npm install` needed — every package above is already an internal package or an existing pinned dependency.

**Version verification:** `go.mod:3` pins `go 1.26`; `charm.land/bubbletea/v2 v2.0.7`, `charm.land/lipgloss/v2 v2.0.3`, `github.com/kevinburke/ssh_config v1.6.0` all confirmed present at their CLAUDE.md-documented versions by reading `go.mod` directly this session — `[VERIFIED: go.mod:1-15]`.

## Package Legitimacy Audit

**Not applicable — no new external packages are introduced by this phase.** Every dependency Phase 8 touches (`internal/doctor`, `internal/identity`, `internal/tuikit`, `internal/globalssh`, `internal/globalgit`, `internal/gitconfig`, `internal/sshconfig`, `github.com/spf13/cobra`) is either a first-party internal package or an existing entry in `go.mod` verified present this session. The Package Legitimacy Gate protocol is skipped per its own trigger condition (phase installs no external packages).

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│ USER (TUI keypress "4"/"5", or CLI `gitid health`/`gitid fix`)       │
└───────────────┬─────────────────────────────┬─────────────────────┘
                │                             │
       TUI path │                             │ CLI path
                ▼                             ▼
   ┌─────────────────────────┐   ┌──────────────────────────────┐
   │ internal/tuikit          │   │ cmd/gitid (NEW: health.go,   │
   │  TabHealth / TabFixer    │   │   fix.go — currently only    │
   │  (split from doctorModel,│   │   newReservedNounCmd stubs)  │
   │  Known Divergence #1)    │   └──────────────┬───────────────┘
   └───────────┬───────────────┘                │
                │  DemoState.Findings            │  doctor.Run(Deps)
                │  (via App.state, Backend.Persist)                 │
                ▼                                ▼
   ┌─────────────────────────────────────────────────────────────┐
   │ realBackend (cmd/gitid/wiring.go)                            │
   │  InitialState(): TODAY synthesizes DemoFinding FROM           │
   │    identity.BuildInventory's Problem taxonomy ONLY            │
   │    (wiring.go:667-695) — no Section, no Family, no Fix.       │
   │  Persist(FixFinding / MarkScanned): TODAY demo-only,           │
   │    routes to tuikit.Reduce (wiring.go:788-795) — NO REAL      │
   │    WRITE HAPPENS. Phase 8 must replace both paths.            │
   └───────────┬─────────────────────────────────┬─────────────────┘
                │ NEW: must also call             │ NEW: must call
                ▼                                 ▼
   ┌─────────────────────────┐      ┌───────────────────────────────┐
   │ internal/identity        │      │ internal/doctor.Run(Deps)      │
   │  BuildInventory/Classify │      │  9 CheckFns (existing) +       │
   │  (existing, per-identity │      │  D-05's new checks (new,       │
   │  Problem taxonomy)       │      │  wrapping globalssh.Verify /   │
   └─────────────────────────┘      │  globalgit.VerifyAuthorRes.)   │
                                     └───────────┬─────────────────────┘
                                                  │ Finding{Family,
                                                  │  Severity, Target/
                                                  │  Section (NEW),
                                                  │  Fix *FixDescriptor}
                                                  ▼
                                     ┌───────────────────────────────┐
                                     │ Fix apply path (Fixer only):    │
                                     │  D-09 typed-confirm ceremony →  │
                                     │  filewriter backup+atomic write │
                                     │  → D-10 parse→render→re-parse   │
                                     │  + ssh -G / git config re-verify│
                                     │  → D-13 re-run doctor.Run() ALL │
                                     │  → D-14 convergence check        │
                                     └───────────────────────────────┘
```

A reader tracing the flagship walkthrough (fix `IdentitiesOnly no→yes` on `clientb.github.com`) follows: TUI key `5` → Fixer screen (fixableFindings) → `f` → `fixCeremonyFor` builds the ceremony from `PlanFor` → typed confirm (`ConfirmWord: "clientb.github.com"`) → `FixFinding` action dispatched → **(NEW)** `realBackend.Persist` performs the real directive rewrite + backup + D-10 verification loop → **(NEW)** re-runs `doctor.Run()` (D-13) → the re-rendered Fixer list either drops the finding (success) or shows the D-14 convergence alarm.

### Recommended Project Structure

No new top-level packages are needed. New files fit the existing layout:

```
internal/doctor/
├── doctor.go              # ADD: Target/Section field on Finding (D-01)
└── checks/
    ├── files.go           # NEW: HLTH-02 parse-gate checks (Files family)
    ├── coherence.go       # EXTEND: shadowed-option, author-resolution,
    │                      #   IdentitiesOnly-contradiction, missing-fragment
    ├── orphans.go         # EXTEND: D-06.1 Class-1 downgrade
    └── baseline.go        # EXTEND: gitignore-pair check (D-05)

cmd/gitid/
├── health.go              # NEW: `gitid health [--json] [--identity]`
├── fix.go                 # NEW: `gitid fix [--yes] [--dry-run]`
└── wiring.go              # EXTEND: realBackend doctor.Run() wiring,
                            #   real FixFinding/MarkScanned Persist cases

internal/tuikit/
├── health_screen.go       # NEW: read-only fork of doctor.go's list+detail
└── fixer_screen.go        # NEW: writable fork, keeps f/F + ceremony
                            # (doctor.go itself is retired/renamed once
                            # both new screens exist — see Pitfall 1)
```

### Pattern 1: Deps-injection check function (existing, reuse exactly)

**What:** Every doctor check is a pure `func(Deps) []Finding` — no direct I/O, all reads/probes/fixes come through injected fields.
**When to use:** Every one of D-05's 8 new checks.
**Example:**
```go
// Source: internal/doctor/checks/orphans.go:41 (verbatim, read this session)
func CheckOrphans(deps doctor.Deps) []doctor.Finding {
    var findings []doctor.Finding
    // ... iterates deps.SSHManagedBlockNames, deps.GitconfigManagedBlockNames,
    // deps.KeyPaths — never touches os.ReadFile or exec.Command directly.
    return findings
}
```

### Pattern 2: Fix descriptor with cmd-layer-injected callable (existing, reuse exactly)

**What:** `FixDescriptor.Fn`/`Interactive` are closures built by the cmd layer, not the doctor core, so `internal/doctor` never imports `internal/filewriter` or calls `os.Chmod`.
**When to use:** Every new fixable finding (except D-05's shadowed-option row, which is explicitly report-only per its own table).
**Example:**
```go
// Source: internal/doctor/checks/orphans.go:68-76 (verbatim, read this session)
var fix *doctor.FixDescriptor
if removeBlock != nil && sshConfigPath != "" {
    fix = &doctor.FixDescriptor{
        Summary: fmt.Sprintf("remove orphaned SSH Host block %q", n),
        Fn: func() error {
            return removeBlock(sshConfigPath, n)
        },
    }
}
```

### Pattern 3: Ceremony-driven destructive fix (existing, reuse exactly — D-09/D-11 flagship)

**What:** The Fixer's ONE hand-written-directive rewrite goes through `ceremonyModel` with `Destructive.ConfirmWord` gating the apply key.
**When to use:** The `IdentitiesOnly no→yes` flagship, and any future fix D-05 marks as rewriting an existing hand-written directive.
**Example:**
```go
// Source: internal/tuikit/fixplans.go:38-47 (verbatim, read this session)
case "ssh-identitiesonly-contradiction":
    return FixPlan{
        File: "~/.ssh/config",
        Diff: strings.Join(FixerFixPreviewLines, "\n"),
        Destructive: &FixDestructive{
            ConfirmWord: "clientb.github.com",
            Warning:     `This rewrites a directive already present in your SSH config. Type the Host name "clientb.github.com" to confirm — this cannot be undone without restoring the backup.`,
        },
        Result: "IdentitiesOnly set to yes on Host clientb.github.com in ~/.ssh/config.",
    }
```
```go
// Source: internal/tuikit/identities.go:2487-2499 (verbatim, read this session)
func fixCeremonyFor(finding DemoFinding) ceremonyModel {
    plan := PlanFor(finding)
    return newCeremony(ceremonyConfig{
        Heading:       "Fix: " + finding.Title,
        Targets:       []string{plan.File},
        Backups:       []string{NewBackupPath(plan.File)},
        Preview:       plan.Diff,
        PreviewDiff:   true,
        Destructive:   plan.Destructive,
        ResultMessage: plan.Result,
        ConfirmLabel:  "Apply fix",
    })
}
```

### Pattern 4: Re-verification probe reuse (existing, reuse exactly for D-05's new checks)

**What:** Phase 6/7 already built and tested the exact probes D-05's "shadowed option" and "author-resolution" checks need — wrap them, do not reimplement.
**Example:**
```go
// Source: internal/globalssh/shadow.go:293 (signature read this session)
func Verify(deps Deps, keys []string) ShadowResult

// Source: internal/globalgit/authorresolve.go:54 (signature read this session)
func VerifyAuthorResolution(deps Deps, matchedDir, unmatchedDir string) (AuthorResolution, error)
```

### Anti-Patterns to Avoid

- **Reimplementing `convergeFixes`/`applyFixes` by copy-pasting the archived POC file:** the archived `cmd-gitid/doctor.go` is a CLI-report-and-prompt design (bufio.Reader-driven `confirm()` prompts) built for a text report, not for the D-12 auto-advancing TUI queue (`doctorBatch`) that already exists. Port the *convergence algorithm* (`findingsSignature`, the max-passes loop) — do not port the text-UI shape.
- **Porting `doctor.go`'s `f`/`F` key handling into the new Health screen by copy-paste:** 08-UI-SPEC.md's own "primary focus" warning — Health must have ZERO write affordance, and copy-paste is the most likely way that regresses silently.
- **Treating `identity.BuildInventory`'s Problem-sourced findings as replaced/removed:** `realBackend.InitialState()` already ships this pipeline and MGR-07 (Identity Manager per-identity health) depends on it independently of Phase 8. Converge, don't delete.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Severity-sorted, section-grouped finding list rendering | A new grouping/sort function | `groupFindings`/`orderedFindings`/`severityRank` (`internal/tuikit/doctor.go:66-128`) | Already correct, already tested, exact shape both new screens need |
| Typed-confirm destructive gate | A new confirm-word input widget | `ceremonyModel`'s existing `Destructive`/typed-confirm mechanism (`ceremony.go:135-152`) | Same mechanism `identities.go`'s "delete everything" ceremony already uses (`identities.go:2817`) |
| Config parse→render→re-parse stability check | A new round-trip verifier | The existing pattern every prior phase's write ceremony already implements (per CLAUDE.md's "second-Decode pass") | D-10 is explicitly this pattern applied to the fixer's write |
| Shadow/redundancy simulation for the "shadowed option" check | A new static-scan namer | `internal/globalssh.Verify`/`BuildGraph`/`Simulate` (`shadow.go`) | Phase 6 built and tested this; D-05 explicitly calls for reuse |
| Author-resolution probe for the Coherence/Git check | A new `git config --show-origin` wrapper | `internal/globalgit.VerifyAuthorResolution` (`authorresolve.go:54`) | Phase 7 built and tested this |
| Archive-path exclusion for unused-key findings | A fresh glob/predicate | Mirror `internal/identity/inventory.go`'s `filterReservedKeyPaths`/`IsReservedKeyPath` causal-exclusion pattern (`inventory.go:113-129`), applied to however `doctor.Deps.KeyPaths` gets populated | Same bug class already fixed once in this codebase (review R-04); D-06.2 is this exact fix applied to the doctor engine's own `KeyPaths` field |

**Key insight:** almost nothing in Phase 8's *render* layer is new — the risk is entirely in wiring the two divergent findings pipelines (`identity.Problem` vs `doctor.Finding`) and building the write path that does not yet exist anywhere in the live binary.

## Runtime State Inventory

Not applicable — Phase 8 is not a rename/refactor/migration phase. No identifiers, keys, or paths are being renamed. (Included per protocol as an explicit negative: "None — this phase adds new checks and two new screens; it renames no existing artifact, key, or path.")

## Common Pitfalls

### Pitfall 1: 08-CONTEXT.md's canonical_refs point at archived, not live, files
**What goes wrong:** A planner or executor reads `cmd/gitid/doctor.go:185` (convergeFixes) or `internal/dummytui/fixplans.go:43` (ConfirmWord) literally and either fails to find the file or edits the WRONG file (the live `internal/tuikit/fixplans.go:43`, which happens to have the same line number for a different reason).
**Why it happens:** 08-CONTEXT.md was gathered 2026-07-07, before Phase 3's D-14/D-17 archived the POC CLI and extracted `internal/tuikit` out of `internal/dummytui`. The file paths are stale.
**How to avoid:** Use this table when following 08-CONTEXT.md's canonical_refs:

| 08-CONTEXT.md says | Actually lives at | Status |
|---|---|---|
| `cmd/gitid/doctor.go:185` (convergeFixes) | `.planning/archive/0.0.1-poc-product-features-in-tui/cmd-gitid/doctor.go:185` | Archived — algorithm reusable, file is not live |
| `cmd/gitid/doctor.go:607` (applyFixes) | same archive path, `:621` | Archived |
| `internal/dummytui/fixplans.go:43` (ConfirmWord) | `internal/tuikit/fixplans.go:43` | Moved (Phase 3 D-17 extraction) — coincidentally same line number |
| `internal/dummytui/data.go:790-833` (flagship diff) | `internal/tuikit/design.go:508-524` (`FixerFixPreviewLines`) — `internal/dummytui/data.go` DOES still exist and DOES still hold `HealthFindings`/`FixerFindings`/`HealthReadOnlyNote`/etc. at different line numbers than cited | Partially moved — verify current line numbers before citing |

**Warning signs:** `go build` failure on a path from 08-CONTEXT.md; a grep for a cited symbol returning zero hits in the live tree.

### Pitfall 2: `internal/doctor` is completely unwired — there is no real "extend" to do for the CLI
**What goes wrong:** Assuming `gitid doctor`/`gitid health`/`gitid fix` already work in some form and only need new checks added.
**Why it happens:** The doctor ENGINE (`internal/doctor`) is fully built and tested; it is easy to mistake "the engine exists" for "the engine is wired in."
**How to avoid:** `grep -rn "internal/doctor" cmd/gitid/ internal/tuikit/` returns zero hits (verified this session) — confirm this is still true before assuming any wiring exists. `cmd/gitid/main.go:105-106` shows `health`/`fix` as `newReservedNounCmd` placeholder stubs only.
**Warning signs:** `gitid health` at HEAD prints "Show identity/config health (arrives in Phase 8)" and exits — that is the ENTIRE current implementation.

### Pitfall 3: Two independent per-identity findings pipelines will silently diverge if not converged
**What goes wrong:** `realBackend.InitialState()` (`wiring.go:667-695`) already builds `state.Findings` from `identity.BuildInventory`'s `Problem` taxonomy (5 values, no `Section`, no `Family`, no `Fix`). If Phase 8 wires `internal/doctor.Run()` as a SEPARATE, ADDITIONAL findings source without reconciling IDs/dedup, the Health/Fixer screens will show duplicate or contradictory rows for the same underlying issue (e.g., a missing fragment reported once as `ProblemFragmentMissing` and once as a `doctor.Finding` from `CheckCoherence`).
**Why it happens:** The two systems were built in different phases for different purposes (MGR-07's simple per-row health badge vs. the full doctor engine) and were never designed to be merged.
**How to avoid:** Decide explicitly (planner-level decision, not resolved by this research) whether: (a) `doctor.Run()` becomes the SOLE findings source and `identity.BuildInventory`'s Problem→Finding conversion in `InitialState()` is replaced by wiring `Identities`/`ManagedHosts`/etc. into `doctor.Deps` so `CheckCoherence` etc. produce the equivalent findings with richer Section/Family/Fix data, or (b) both remain, with a documented, tested de-duplication rule at the `IdentityName`+concept-signature level.
**Warning signs:** A PTY e2e test observing the SAME real misconfiguration (e.g., a missing gitconfig fragment) produces two rows in the Health list, not one.

### Pitfall 4: CheckOrphans Class-1's false-positive loop is real and reachable TODAY via `--git-only` delete
**What goes wrong:** `gitid identity delete --git-only <name>` (`cmd/gitid/identity_delete.go:42`) removes only the gitconfig side, leaving the SSH Host block intact — producing exactly the state `CheckOrphans` Class 1 (`orphans.go:52-87`) flags as an orphan with a DESTRUCTIVE `RemoveBlock` fix. Applying that fix would delete a Host block the user deliberately kept.
**Why it happens:** `CheckOrphans` has no way today to distinguish "this SSH block never had a gitconfig counterpart because it's SSH-only by design" from "this SSH block used to have a counterpart, deliberately removed via `--git-only`" from "this is a genuine accidental orphan" — all three produce the identical on-disk shape (SSH Host block present, no matching gitconfig includeIf block).
**How to avoid:** D-06.1 requires downgrading Class 1 to info/no-fix for "the git-only-delete state," but **no persisted marker distinguishes deliberate git-only-delete from an SSH-only-by-design identity** — both are structurally identical on disk. This is a genuine open design question the planner must resolve (see Open Questions), not something this research can pin definitively without inventing project-state that does not exist. The safest documented interpretation matching 08-CONTEXT.md's severity discipline ("critical = parse gate only") is to downgrade Class 1 broadly for any structurally well-formed, currently-referenced SSH Host block (key exists, block parses) rather than attempt to detect "was deliberately deleted" — since a destructive auto-offered removal of a healthy, in-use SSH-only identity is the bug regardless of history.
**Warning signs:** A regression test asserting `CheckOrphans` does NOT offer to remove a `--git-only`-deleted identity's surviving SSH block.

### Pitfall 5: `doctor.Deps.KeyPaths` has no archive-path exclusion — Class-3 unused-key findings can flag archived keys
**What goes wrong:** `CheckOrphans` Class 3 (`orphans.go:130-156`) iterates `deps.KeyPaths` directly with no reserved-path filter. `internal/identity/inventory.go`'s `BuildInventory` already solved this exact problem for its OWN unused-key cross-reference (`filterReservedKeyPaths`, `inventory.go:113-129`) — but that fix lives in a different package and is not automatically inherited by however the NEW cmd-layer wiring populates `doctor.Deps.KeyPaths`.
**Why it happens:** `doctor.Deps.KeyPaths` was, in the archived POC, populated from `identity.Reconstruct`'s per-account `KeyPath` values (never a raw glob), so the archive dir was structurally unreachable there — but the NEW wiring is being built fresh and could easily reintroduce a raw glob that DOES reach `~/.ssh/gitid-archive/`.
**How to avoid:** Whatever function populates `doctor.Deps.KeyPaths` for the new `cmd/gitid` wiring must apply the exact same causal-exclusion discipline `inventory.go` already established — filter at the point the path LIST is collected, not by hoping the collection method happens to miss the archive dir.
**Warning signs:** A regression test asserting a populated `~/.ssh/gitid-archive/` never produces an unused-key finding — mirroring `identity`'s own "populated-archive regression test" 08-CONTEXT.md D-06.2 explicitly calls for.

### Pitfall 6: The pinned `git-includeif-missing-fragment` fixture's Family ("Orphans") may conflict with D-05's table ("Coherence / Git")
**What goes wrong:** The frozen dummy fixture (`internal/dummytui/data.go:392`, `[VERIFIED: internal/dummytui/data.go:392]`) tags the missing-includeIf-fragment finding as `Family: "Orphans"`. 08-CONTEXT.md's D-05 new-checks table assigns "includeIf → missing fragment" to `Coherence / Git`. If these are meant to be the SAME check, the family assignment conflicts; if they are meant to be two DIFFERENT checks (one pre-existing in Orphans, one new in Coherence) detecting overlapping conditions, that needs to be explicit or the Health list will show a duplicate.
**How to avoid:** Planner must explicitly decide/confirm whether D-05's "includeIf → missing fragment" row IS the existing `git-includeif-missing-fragment` fixture (in which case its Family stays `Orphans`, and D-05's table description is imprecise) or a genuinely new, additional Coherence-family check.
**Warning signs:** Two rows in the Health Git section both describing a missing/dangling `includeIf` target for the same identity.

## Code Examples

### Existing `Finding`/`Deps`/`FixDescriptor` shape (verbatim, D-01's `Target` field lands here)
```go
// Source: internal/doctor/doctor.go:94-108 (verbatim, read this session)
type Finding struct {
    Family       Family
    Severity     Severity
    Title        string
    Explanation  string
    SuggestedFix string
    Fix          *FixDescriptor
    // IdentityName is the name of the managed identity this finding belongs to.
    // Empty string means the finding is global (not scoped to a single identity).
    IdentityName string
}
```
D-01 requires adding a `Target`/`Section` field here — note the TUI-side `HealthFinding` (`internal/tuikit/design.go:494-502`) ALREADY has a `Section string` field; the new `doctor.Finding.Target` (or reused `Section`) field is what the eventual cmd-layer conversion function maps onto that existing TUI field.

### Existing `HealthFinding` shape the TUI already renders (verbatim)
```go
// Source: internal/tuikit/design.go:492-502 (verbatim, read this session)
type HealthFinding struct {
    ID           string
    Section      string
    Family       string
    Title        string
    Explanation  string
    SuggestedFix string
    Severity     HealthSeverity
}
```

### Existing demo-only Persist cases that Phase 8 must replace (verbatim)
```go
// Source: cmd/gitid/wiring.go:788-795 (verbatim, read this session)
case tuikit.MarkScanned:
    // Demo-only: the Phase 8 doctor scan does not write in-disk config;
    // keep the approved in-memory reducer behavior behind the D-16 banner.
    return tuikit.Reduce(state, action)
case tuikit.FixFinding:
    // Demo-only: Phase 8's fixer is not wired yet; keep the banner
    // behavior pinned rather than turning it into an error (R-09-DEMO).
    return tuikit.Reduce(state, action)
```

### Existing `DemoBanner` gate that must drop `TabDoctor`/gain the two new tabs (verbatim)
```go
// Source: cmd/gitid/wiring.go:706-713 (verbatim, read this session)
func (b *realBackend) DemoBanner(tab tuikit.TabID) bool {
    switch tab {
    case tuikit.TabDoctor:
        return true
    default:
        return false
    }
}
```

### Existing convergence-loop algorithm to PORT (not literally reuse — file is archived)
```go
// Source: .planning/archive/0.0.1-poc-product-features-in-tui/cmd-gitid/doctor.go:179-209
// (verbatim, read this session — archived, reference the ALGORITHM only)
func convergeFixes(
    initial []doctor.Finding,
    apply func(fixable []doctor.Finding) int,
    runChecks func() []doctor.Finding,
    maxPasses int,
) []doctor.Finding {
    findings := initial
    prevSig := findingsSignature(findings)
    for pass := 0; pass < maxPasses; pass++ {
        fixable := collectFixable(findings)
        if len(fixable) == 0 {
            break
        }
        if apply(fixable) == 0 {
            break // nothing applied (declined / all failed) — no progress possible
        }
        findings = runChecks()
        sig := findingsSignature(findings)
        if sig == prevSig {
            break // pass changed nothing observable — stop instead of spinning
        }
        prevSig = sig
    }
    return findings
}
```
D-14's convergence alarm is the missing piece this archived loop never had: it silently `break`s on no-progress instead of emitting an error finding. The new implementation must detect "a fix reported success but the SAME finding signature reappeared next pass" and emit the alarm finding instead of silently stopping.

## State of the Art

| Old Approach (archived POC) | Current Approach (Phase 8 must build) | When Changed | Impact |
|---|---|---|---|
| `gitid doctor [--fix] [--yes]` — single command, text report, bufio-prompt-driven fix flow | `gitid health [--json] [--identity]` (read) + `gitid fix [--yes] [--dry-run]` (write), `doctor` as a permanent hidden alias (D-03) | Phase 3 (D-14 archival) → Phase 8 (D-03) | Read/write CLI split enforces the read-only affordance at the CLI layer, matching the TUI's Health/Fixer split |
| `doctorModel` single combined TUI tab (list all findings, `f`/`F` fixes inline) | `TabHealth` (read-only) + `TabFixer` (fixable-only + ceremony) — Known Divergence #1 | This phase | `TabID` count 4→5, `newScreens` returns `[5]screenModel` |
| Silent `convergeFixes` stop on no-progress | D-14 explicit "fix did not converge" error finding, permanently excluded from re-offer | This phase | Prevents the false-positive-loop class from recurring silently |

**Deprecated/outdated:** the archived `cmd-gitid/doctor.go`'s bufio-prompt CLI fix flow (`confirm()`, batching-by-family) is superseded by the D-12 "auto-advancing queue of per-finding ceremonies" model already implemented in `doctorBatch`/`fixCeremonyFor` for the TUI; the new CLI's `--yes`/`--dry-run` flags should drive the SAME underlying convergence+ceremony logic, not reintroduce a separate prompt loop.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | D-06.1's Class-1 downgrade should apply broadly to any structurally well-formed, currently-referenced SSH block (rather than requiring a specific "was deliberately git-only-deleted" marker, which does not exist on disk) | Common Pitfalls #4 | If the planner instead requires detecting deliberate-delete specifically, a new persisted marker or heuristic must be designed — larger scope than this research assumes |
| A2 | D-05's "includeIf → missing fragment" row IS the same check as the existing `git-includeif-missing-fragment` fixture (Family "Orphans"), and D-05's table's "Coherence / Git" label is an imprecise gloss, not a literal new-Family instruction | Common Pitfalls #6 | If wrong, two distinct checks/findings must both exist, requiring dedup logic neither 08-CONTEXT.md nor 08-UI-SPEC.md specify |
| A3 | The right resolution for the two findings pipelines (`identity.Problem` vs `doctor.Finding`) is for `internal/doctor.Run()` to become the sole findings source feeding both Health/Fixer AND the existing MGR-07 per-identity slice, with `identity.BuildInventory`'s Problem→Finding conversion in `wiring.go:667-695` either removed or reduced to a data source `CheckCoherence`/`CheckOrphans` etc. consume | Summary, Pitfall 3 | This is presented as the most coherent reading of 08-CONTEXT.md's D-01 ("consumed by ... the HLTH-05/MGR-07 per-identity slice") but is explicitly a planner-level architecture decision, not something this research resolves definitively |

## Open Questions

1. **How does D-06.1's Class-1 downgrade actually detect "deliberate git-only delete" vs. "genuinely orphaned"?**
   - What we know: both states produce an IDENTICAL on-disk shape (SSH Host block present, no gitconfig includeIf counterpart); `--git-only` delete (`identity_delete.go:42`) does not write any marker recording that the deletion was deliberate.
   - What's unclear: whether D-06.1 intends a NEW persisted marker, a broader "never destructively remove a well-formed/referenced SSH block via Orphans" policy (this research's Assumption A1), or something else.
   - Recommendation: resolve during planning/discuss, before any Class-1 code changes — this is the phase's own governing precedent (the false-positive loop) and deserves an explicit decision, not an inferred one.

2. **Do the two per-identity findings pipelines (`identity.Problem` via `InitialState()`, and the new `internal/doctor.Run()`) converge into one, or coexist with deduplication?**
   - What we know: `InitialState()` already ships one pipeline live; D-01/D-05/08-CONTEXT.md assume the doctor engine's richer Finding shape for the new checks and for HLTH-05/MGR-07.
   - What's unclear: the exact convergence mechanism — 08-CONTEXT.md and 08-UI-SPEC.md do not address this because they were written analyzing the render layer, not `wiring.go`'s data layer.
   - Recommendation: a dedicated tracer plan/task (mirroring every prior phase's "wave 1 tracer" pattern) should resolve this FIRST, before D-05's 8 new checks are added — get one check's data flowing end-to-end (real disk → doctor.Run → TUI render → CLI --json) before fanning out.

3. **Per-identity health entry point (D-04)** — already flagged as unresolved by 08-UI-SPEC.md itself (three candidates: CLI-flag-only, Identity-Manager deep-link, or an in-surface filter key). Not re-litigated here; 08-UI-SPEC.md's recommendation (deep-link from the Identity Manager row, consistent with the existing `HealthPerIdentityMgrHandoff` copy) is sound and this research adds no new information.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `git` binary | `RunGitConfigGet`, author-resolution probes, parse gates | ✓ (assumed present — project requires it as a runtime dependency project-wide) | — | D-10 explicitly degrades to render+re-parse only when git/ssh are absent (headless fallback, "known CI portability class") |
| `ssh`/`ssh-keygen` binary | `RunSSHAdd`, `RunSSHKeygenFingerprint`, shadow-simulation `ssh -G` probes | ✓ (assumed present) | — | Same D-10 headless degradation |
| Go 1.26 toolchain | build | ✓ `[VERIFIED: go.mod:3]` | 1.26 | — |

No new external environment dependency is introduced by this phase; it reuses probes (`ssh -G`, `git config --show-origin`) already exercised by Phases 6/7's own e2e suites.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `go test -race` (project-wide convention) |
| Config file | none — `Makefile` targets (`make test`, `make test-e2e`, `make lint`) |
| Quick run command | `TERM=dumb SSH_AUTH_SOCK= go test -count=1 ./internal/doctor/... ./internal/tuikit/... ./cmd/gitid/...` |
| Full suite command | `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./... && make lint && make test-e2e` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HLTH-01 | Health screen has SSH + Git sections | unit + PTY e2e | `go test ./internal/tuikit/... -run TestHealth` | ❌ Wave 0 (new `health_screen_test.go`) |
| HLTH-02 | Files-exist + parse checks; parse fail = critical, downstream paused | unit | `go test ./internal/doctor/checks/... -run TestCheckFiles` | ❌ Wave 0 (new `files_test.go`) |
| HLTH-03 | Redundancy/override detection | unit | `go test ./internal/doctor/checks/... -run TestCheckRedundancy` | ✅ (`redundancy_test.go`, 226 lines, existing) |
| HLTH-04 | Contradiction detection (IdentitiesOnly no + IdentityFile; missing includeIf fragment) | unit | `go test ./internal/doctor/checks/... -run TestCheckCoherence` | ✅ existing file, ❌ new cases needed (`coherence_test.go`, 372 lines — extend) |
| HLTH-05 | Per-identity + global health | unit + CLI e2e | `go test ./cmd/gitid/... -run TestHealthIdentityFlag` | ❌ Wave 0 |
| HLTH-06 | Deps/perms/coherence/orphans/signing/agent families reused | unit | existing per-family tests (`deps_test.go`, `perms_test.go`, `signing_test.go`, `overlap_test.go`) | ✅ all exist |
| FIX-01 | Confirmed, backed-up fixes | unit + PTY e2e | `go test ./internal/tuikit/... -run TestFixCeremony` | ❌ Wave 0 (fork of `doctor_test.go`, 237 lines) |
| FIX-02 | Two-section fixer UX | PTY e2e | new e2e test in `e2e/` mirroring `git_configuration_pty_e2e_test.go`'s pattern | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** targeted package test (`go test ./internal/doctor/...` etc.)
- **Per wave merge:** `go test -race ./...` + `make lint`
- **Phase gate:** full suite green (`make test`, `make test-e2e`, `make lint`) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/doctor/checks/files_test.go` — covers HLTH-02
- [ ] `internal/tuikit/health_screen_test.go` — covers HLTH-01, HLTH-03/04 render
- [ ] `internal/tuikit/fixer_screen_test.go` — covers FIX-01/02 render + ceremony
- [ ] `cmd/gitid/health_test.go` / `cmd/gitid/fix_test.go` — CLI surface, replacing the reserved-noun stub tests
- [ ] `e2e/health_fixer_pty_e2e_test.go` — real-binary PTY coverage per DLV-06
- Framework install: none — `go test`/`golangci-lint` already fully bootstrapped project-wide

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Phase 8 has no auth surface |
| V3 Session Management | no | n/a |
| V4 Access Control | no | Local single-user tool; no access-control boundary |
| V5 Input Validation | yes | The typed-confirm `ConfirmWord` comparison MUST be exact-string match (already the pattern in `ceremony.go:152`); the directive-rewrite target (`clientb.github.com`-style hostname) must be validated the SAME way `sshconfig`'s existing host-block validation already validates hostnames elsewhere in the project — do not hand-roll a new hostname validator |
| V6 Cryptography | no | No new crypto surface — keys are read-only inputs to signing checks, never generated/modified by this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Comma/shell-metacharacter injection into a rewritten SSH directive value | Tampering | The project already fixed this class for `allowed_signers` principals (CR-18, `04-git-configuration-screen`) — the D-09 directive-rewrite primitive must go through the SAME validated-value discipline (`internal/sshconfig`'s existing directive validation), never a raw string splice |
| A fixer write silently corrupting hand-written config outside the ONE targeted directive | Tampering | D-10's mandatory parse→modify→render→re-parse stability check IS the mitigation — this is the CLAUDE.md "second-Decode pass" pattern, non-negotiable per D-09's scoped amendment |
| A convergence loop that never terminates (e.g. two checks disagreeing about the same artifact) | Denial of Service (local) | `maxPasses` hard backstop (existing pattern in the archived `convergeFixes`, `maxPasses=10`) — the new implementation must keep an equivalent hard cap |
| `os/exec` calls (`ssh -G`, `git config --show-origin`) executed with unsanitized identity-derived arguments | Tampering (command injection) | Every existing probe in this codebase uses arg-slice `exec.Command` (never shell string interpolation) — `[VERIFIED: cmd-gitid/doctor.go:217,236]` shows the pattern (`//nolint:gosec // arg-slice form, no shell`); the new checks reusing `globalssh.Verify`/`globalgit.VerifyAuthorResolution` inherit this discipline already |

## Sources

### Primary (HIGH confidence — read directly this session)
- `internal/doctor/doctor.go` (full file, 330 lines) — Severity/Family/Finding/Deps/FixDescriptor/Run/ExitCode/Families
- `internal/doctor/checks/orphans.go` (full file, 169 lines) — CheckOrphans Class 1/2/3, IsReservedBlockName usage
- `internal/identity/state.go` (lines 100-215) — Problem/Severity/SeverityFor/Classify
- `internal/identity/inventory.go` (lines 30-150) — BuildInventory, filterReservedKeyPaths, IsReservedKeyPath
- `internal/tuikit/doctor.go` (full file via codegraph, ~250+ lines) — doctorModel, groupFindings, orderedFindings, fixableFindings, handleKey
- `internal/tuikit/fixplans.go` (full file, 67 lines) — PlanFor, FixPlan, FixDestructive
- `internal/tuikit/design.go` (lines 455-533) — HealthFinding, HealthSeverityGlyph, FixerFixPreviewLines, FixerTargetHost
- `internal/dummytui/data.go` (grep + targeted reads) — HealthFindings/FixerFindings fixture list, copy constants
- `cmd/gitid/main.go` (full file, 110 lines) — newRootCmd, health/fix reserved-noun stubs
- `cmd/gitid/wiring.go` (lines 660-880) — InitialState, DemoBanner, Persist (all action cases)
- `internal/gitconfig/baseline.go` (lines 255-320, 460-520) — DefaultGitignorePatterns, WriteGlobalGitignore
- `internal/sshconfig/include.go` (lines 49-96) — IsReservedBlockName, ArchiveDir, ArchiveDirName
- `internal/globalssh/shadow.go` (function list via grep) — Verify signature
- `internal/globalgit/authorresolve.go` (function list via grep) — VerifyAuthorResolution signature
- `.planning/archive/0.0.1-poc-product-features-in-tui/cmd-gitid/doctor.go` (full file, 768 lines) — convergeFixes, applyFixes, buildDoctorDeps, findingsSignature
- `go.mod` (lines 1-15) — Go 1.26, dependency versions
- `.planning/config.json` — workflow toggles (nyquist_validation: true, security_enforcement: true, security_asvs_level: 1)

### Secondary (MEDIUM confidence)
- `.planning/phases/08-health-fixer/08-CONTEXT.md` — binding user decisions D-01..D-16, cross-checked against live source (several path citations found stale, decisions themselves treated as authoritative)
- `.planning/phases/08-health-fixer/08-UI-SPEC.md` — binding UI contract, cross-checked against live `frame.go`/`app.go`/`ceremony.go` (all claims verified accurate)
- `.planning/REQUIREMENTS.md` §K/§L — HLTH-01..06, FIX-01/02
- `.planning/ROADMAP.md` §"Phase 8"

### Tertiary (LOW confidence)
- None — no WebSearch was needed for this phase; it is entirely an internal-codebase archaeology task.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new external packages; every internal package cited was read directly this session
- Architecture: HIGH for what exists (doctor engine, tuikit render substrate) — MEDIUM for the convergence design (Pitfall 3/Open Question 2), which is a genuine unresolved architecture decision, not a research gap
- Pitfalls: HIGH — all six pitfalls are grounded in source read this session, not inferred

**Research date:** 2026-08-27
**Valid until:** 30 days (stable internal-codebase research; re-verify path citations if any Phase 8 wave renames/moves files before this document is consumed)
