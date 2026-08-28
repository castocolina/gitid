# Phase 9: Upload / Credentials Assist - Research

**Researched:** 2026-08-28
**Domain:** Go CLI/TUI wiring of external hosted-git CLIs (`gh`, `glab`) for autonomous SSH key registration, inside a Bubble Tea v2 wizard
**Confidence:** HIGH for the substrate/wiring findings (all `[VERIFIED]` against files read this session); MEDIUM for `gh`/`glab` flag semantics (official docs, `[CITED]`); the product-behavior decisions themselves are OUT OF SCOPE — they are already locked in `09-CONTEXT.md`/`09-UI-SPEC.md`.

<user_constraints>
## User Constraints (from CONTEXT.md and UI-SPEC.md)

### Locked Decisions (09-CONTEXT.md D-01..D-18 — do not re-litigate)

- **D-01 (Checkbox model):** the wizard exposes an auto-upload checkbox whose
  enabled/checked state is DERIVED from `(hostname, CLI presence, auth status)`:
  main-domain match + tool present + authenticated → enabled+pre-checked;
  match + tool present + not logged in → enabled, unchecked; no match/unknown
  host → disabled. Headless CLI mirrors the checkbox with flags.
- **D-02 (Announce-and-do):** autonomous upload prints each command exactly as
  executed via `CommandPreview`, then per-key results. No prompt, no cancel
  timer.
- **D-03 (Failure semantics):** upload failure never affects the primary
  operation's exit code (exit 0 + warning + manual instructions). Upload is
  integrated into the TUI identity wizard as its own step/section;
  rotate/clone/copy reuse the same component.
- **D-04 (Rotate old key — USER OVERRIDE):** after rotate, present an
  interactive delete offer for the old remote key (confirmed ceremony, never
  autonomous), using the D-15 provider inventory to resolve the old key's ID.
- **D-05 (Timing):** in create, upload runs after the key exists and before
  the `ssh -T` test loop.
- **D-06 (Controls):** `--no-upload` opt-out flag on create/rotate/clone/
  add-account; `--dry-run` prints `CommandPreview` output and never executes.
  No env-var opt-out in v1.0.
- **D-07 (Key title):** keys are registered under `gitid: <name> @ <hostname>`;
  title matching is machine-scoped; key-content comparison is the primary
  dedupe truth.
- **D-08 (Hybrid amendment):** upload is a section, not a navigable surface.
  ONE shared upload-section component contract covers all states.
- **D-09 (Visual gate satisfiability):** Phase 9's UI wave must mint fresh
  approved captures for exactly the amended screens — no static Phase-2 PNGs
  survive; the live demos ARE the approved reference.
- **D-10 (POC contradiction):** the archived `tui/copy.go` per-key Enter/skip
  prompt queue contradicts UP-03 autonomy — it is redesigned in the contract,
  not patched.
- **D-11 (DetectFor):** replace first-found `Detect` with `DetectFor(provider)`
  — `github` → gh only, `gitlab` → glab only, never cross-route; unknown →
  manual-only.
- **D-12 (glab value):** use `--usage-type auth_and_signing` (glab's own
  default; accepted values `auth`/`signing`/`auth_and_signing`, flag since
  glab v1.54.0). Rename `GLabKeyTypeForAuth` to reflect combined usage.
- **D-13 (Self-hosted scope):** v1.0 autonomous upload is gated to
  `github.com`/`gitlab.com` hosts only; self-hosted gets manual instructions.
  `gh ssh-key add` has no `--hostname` flag (verified).
- **D-14 (Auth probe + scopes):** probe with `auth status --hostname <host>`
  (never bare `gh auth status`). Do not parse scopes pre-flight; attempt
  upload and classify scope errors into remediation copy (`gh auth refresh -h
  <host> -s admin:public_key` / `-s admin:ssh_signing_key`).
- **D-15 (Provider key inventory):** ONE new `internal/uploader` inventory
  function: `gh api user/keys` + `gh api user/ssh_signing_keys` (JSON via
  `--jq`), `glab ssh-key list -F json`; compares the normalized key blob.
  Powers dedupe, GitHub per-type gap detection, rotate leftover report, D-04's
  delete-offer key-ID lookup. Inventory failure degrades to plain upload,
  never gates. `gh` duplicate → benign (dedupes client-side, exits 0); `glab`
  `"has already been taken"` → cross-account-conflict finding, never silent
  success.
- **D-16 (Per-type ensure):** GitHub's two registrations are attempted
  independently, driven by the inventory's missing-type diff; uploader
  returns a per-registration result struct (uploaded/already-present/failed).
- **D-17 (Verification):** post-upload, re-run `internal/tester`'s `ssh -T`
  expecting `ReachableNotUploaded → PASS`, plus one post-upload inventory
  read; one bounded retry for propagation lag.
- **D-18 (Health):** no persisted upload state — the live tester's
  `ReachableNotUploaded` IS the health signal.

### Claude's Discretion (from 09-CONTEXT.md)

- Exact copy: announced-running lines, per-key result rows, manual-fallback
  block wording, scope-remediation messages, `gh auth login` hint, checkbox
  labels/disabled-state note, rotate delete-offer ceremony copy (now DRAFTED
  in 09-UI-SPEC.md's Copywriting Contract — treat those strings as the
  binding draft unless the planner has a concrete reason to deviate).
- Flag naming/wiring details (`--no-upload` vs per-flow variants) within the
  Phase 5 adaptive-CLI + outcome-parity contract.
- Inventory function shape (single call returning per-type presence vs
  separate calls), JSON parsing details, normalized-blob comparison.
- Hostname source for the D-07 title (`os.Hostname()` trimming/normalization).
- Where the upload step sits in the wizard beat sequence — NOW PINNED by
  09-UI-SPEC.md's "Design Decision: Where the section lives" (a new
  `testUpload` sub-beat inside wizard step 1, between key-staging and the
  `ssh -T` probe; checkbox row on step 0, last row of the SSH form, keystroke
  `u`).
- Error-string classifiers' looseness (match on scope name / fingerprint
  keyword, not full sentences).

### Deferred Ideas (OUT OF SCOPE for this phase)

- `GH_HOST`/`GITLAB_HOST` env plumbing for self-hosted autonomous upload.
- `GITID_NO_UPLOAD` env-var opt-out.
- Inventory-backed signing-registration finding in the health screen.
- Tool-inventory display ("glab installed but this is a GitHub identity").
- Distinct exit code for "primary OK, upload degraded".

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UP-01 | Concrete steps to register the `.pub` for authentication and signing (GitHub = two registrations, GitLab = one). | `internal/upload.Instructions(provider)` already implements this text-only path — `[VERIFIED: internal/upload/upload.go:31-54]`, quoted in full below. This phase's job is wiring it into a rebuilt CLI surface (see "Substrate Reality Check") and the new wizard section, not rewriting the copy. |
| UP-02 | `gh`/`glab` detect + prompt + upload; shown command == run command; absent/unauth falls back to manual, never gates create/copy. | `internal/uploader.CommandPreview`/`buildArgs` already give shown==run structurally — `[VERIFIED: internal/uploader/uploader.go:150-185]`. D-11's `DetectFor` replaces `Detect`; see Architecture Patterns and Common Pitfalls for the exact call-site work. |
| UP-03 | Autonomous upload when `gh`/`glab` authenticated + valid identity exists; no stop; shown command == run command; manual fallback otherwise. | This is the phase's actual delta: the checkbox-derived autonomy (D-01), the `testUpload` wizard sub-beat (UI-SPEC), and the CLI's adaptive-depth resolver mirroring it (see Architecture Patterns §CLI verb pattern). |

</phase_requirements>

## Summary

`internal/upload` and `internal/uploader` exist and are well-designed — `Instructions()`,
`Detect`/`AuthCheck`/`UploadKey`/`CommandPreview`/`buildArgs` are exactly the shape
`09-CONTEXT.md` describes. But **neither package has a single production caller anywhere
in the current tree.** `cmd/gitid/copy.go` and `tui/copy.go` — the two files
`09-CONTEXT.md`'s "Built substrate (this phase modifies)" section names as the wiring
this phase corrects — were **archived wholesale** in commit `d60a4d7` ("archive the POC
surface and open the real app shell", Phase 3), and `cmd/gitid/main_test.go`'s
`TestNewRootCmdArchivedPOCCommandsAreGone` **asserts `copy` and `upload` are absent** as
a locked regression test. This phase is not "fix two routing bugs in existing wiring" —
it is **wire a CLI verb and a TUI wizard section from scratch**, reusing the untouched
`internal/upload`/`internal/uploader` packages as the backend engine. `09-UI-SPEC.md`'s
placement analysis (the `testUpload` sub-beat, the step-0 checkbox, the focus-index math)
is independently verified against the current `internal/tuikit/identities.go` and is
accurate — **except** its Identity-Manager modal citation ("reuses `placeOverlay`
pixel-for-pixel"): `placeOverlay` does not exist anywhere in the current source tree (it
was part of the Phase-2 `internal/dummytui/model.go`, which was deleted when Phase 3
consolidated the dummy binary onto a `FixtureBackend` implementing the real
`tuikit.Backend` interface). The real modal mechanism is the `identPane` enum
(`paneClone`, `paneDeleteScope`, `paneKeyCeremony`, …) with two hand-written switch
statements (`handleKey`, `view`); the register-key-modal is a new `identPane` value
following that pattern, not an overlay-compositing call.

**Primary recommendation:** Treat this phase as new-CLI-verb + new-wizard-section
construction against the *current* `internal/tuikit`/`cmd/gitid` composition-root
architecture, using `internal/upload`/`internal/uploader` unmodified except for the
D-11/D-12/D-15/D-16 API changes CONTEXT.md already specifies. Ground every planned task
in the exact `identities.go` line ranges cited below (`wizardSteps`, the focus-index
`iota` chain, `wizardFooter`, `identPane`, `actionMenuLabels`/`actionMenuRows`) — this
file's constants are exactly the kind of parallel, easy-to-desync structures this
project's own `LEARNINGS`/`CLAUDE.md` doctrine warns about (nil-guard wiring blindspot,
reserved-block false-positive loop).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `gh`/`glab` detection, auth probe, key upload, inventory read | Backend (composition root: `cmd/gitid/wiring.go`) | — | `internal/uploader.Deps` is the injectable exec/LookPath seam; the real closure is built once in `wiring.go`, exactly like every other Deps struct in this codebase (`identity.Deps`, `globalgit` probe deps). |
| Auto-upload checkbox derived state (D-01) | Backend (pure function, no I/O) | TUI (`internal/tuikit`) | Derivation needs `Detect`/`AuthCheck` results — a backend concern — but the RENDER (checkbox glyph/label) is `internal/tuikit`'s job via a new View DTO, mirroring the `TestResultView`/`ReusableKeyView` DTO-conversion-only-in-wiring.go rule. |
| `testUpload` wizard sub-beat (announce/results) | TUI (`internal/tuikit/identities.go`) | Backend (`tea.Cmd`-returning method) | Same pattern as `TestStage1`/`TestStage2`: Backend returns a `tea.Cmd` that eventually delivers a typed `tea.Msg`; `internal/tuikit` owns the state machine and rendering, never the exec call itself (no-backend-import gate). |
| Manual-fallback instructions text | Backend/shared (`internal/upload.Instructions`) | TUI (render slot only) | Already built, provider-templated, and imported by both surfaces without an import cycle — reuse verbatim per D-08/UI-SPEC's "byte-identical" requirement. |
| CLI verb (`gitid identity copy` or equivalent) | CLI (`cmd/gitid`) | — | Must be built via the existing `identityVerb`/`newVerbCmd`/`depthResolver` pattern (`cmd/gitid/identity.go`), NOT a bespoke `cobra.Command` — the D-02 adaptive-depth resolver and D-06 flags need to share that infrastructure to stay parity-matrix-consistent with every other write verb. |
| Rotate delete-offer (D-04) | TUI (`internal/tuikit/identities.go`, key-ceremony pane) | Backend (`internal/uploader` inventory + a delete call) | Same shared-component-in-a-new-sub-beat pattern as the wizard's `testUpload`, appended to the existing rotate result screen (`keyCeremonyFor`, line 2438). |
| No persisted upload state (D-18) | N/A (explicitly stateless) | — | Confirmed: no doctor/health finding family currently references upload; nothing to wire there this phase. |

## Standard Stack

### Core

No new third-party Go dependency is required. `go.mod` already has everything this phase
needs:

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os/exec` (stdlib) | Go 1.26 stdlib | Shell out to `gh`/`glab` with an explicit arg slice (no shell) | Same pattern already used by `internal/deps.Detect`, `internal/tester`, and the archived `buildUploaderDeps` — `[VERIFIED: .planning/archive/.../cmd-gitid/copy.go:186-203]` |
| `encoding/json` (stdlib) | Go 1.26 stdlib | Parse `gh api ... --jq` / `glab ssh-key list -F json` output for D-15's inventory | No third-party JSON library exists anywhere in `go.mod` — stdlib is the established convention project-wide |
| `charm.land/bubbletea/v2` | v2.0.7 (pinned, `go.mod:7`) | The wizard's `tea.Cmd`/`tea.Msg` state machine the new `testUpload` sub-beat must join | `[VERIFIED: go.mod:7]` |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/spf13/cobra` | v1.10.2 (`go.mod:16`) | The new CLI verb | Build it via `identityVerb`/`newVerbCmd`, not a raw `&cobra.Command{}` literal — see Architecture Patterns |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Shelling out to `gh`/`glab` | GitHub/GitLab REST API clients (`go-github`, `go-gitlab`) with a stored PAT | Rejected by the locked design itself (D-01..D-18 assume the user's existing `gh`/`glab` CLI auth) — REJECTED, not this phase's call to revisit |
| stdlib `encoding/json` | A schema/codegen JSON library | Massive overkill for parsing `id`/`key`/`title` fields off two small array responses; no precedent anywhere in this codebase |

**Installation:** none — no `go get` is needed for this phase.

**Version verification:**

```
$ go version
go1.27.0 darwin/amd64      # local toolchain; go.mod pins "go 1.26"
$ gh --version
gh version 2.98.0 (2026-08-20)     # well above D-15's "gh >= 2.27.0 dedupes client-side"
$ glab --version
glab 1.114.0 (4d7c6cda7)           # well above D-12's "flag since glab v1.54.0"
```
`[VERIFIED: local shell, this session]`. Both CLIs are present on the research
machine, which lets the planner schedule a real (non-network, `--dry-run`-only or
fake-Deps) integration test rather than assuming a hard environment gap — see
Environment Availability below.

## Package Legitimacy Audit

Not applicable — this phase adds zero new Go module dependencies. `gh` and `glab` are
external CLI binaries the user installs themselves (already an established pattern:
`internal/deps.Report` treats `git`/`ssh`/`ssh-keygen` the same way — required-or-optional
tool detection, never a vendored/bundled binary).

**Packages removed due to [SLOP] verdict:** none — no packages evaluated (none proposed).
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │   cmd/gitid (composition root, wiring.go)    │
                    │                                               │
  os.Hostname() ───▶│  buildUploaderDeps() -> uploader.Deps{        │
                    │    LookPath: exec.LookPath,                  │
                    │    RunCmd:  exec.Command(...).CombinedOutput │
                    │  }                                            │
                    └──────────────┬────────────────────────────────┘
                                   │ injected once, real closures
                                   ▼
        ┌──────────────────────────────────────────────────────────┐
        │              internal/uploader (UNCHANGED SHAPE,          │
        │              D-11/D-12/D-15/D-16 API deltas only)          │
        │                                                             │
        │  DetectFor(provider, deps) -> Tool, path, AuthStatus        │
        │  AuthCheck(path, deps, host) -> AuthStatus                  │
        │  Inventory(tool, path, deps) -> []ExistingKey  (NEW, D-15)  │
        │  UploadKey(tool, path, pub, title, type, deps)              │
        │       -> PerTypeResult{Uploaded|AlreadyPresent|Failed} (D-16)│
        │  CommandPreview(...) == the exact args UploadKey executes   │
        └──────────────────────────┬───────────────────────────────┘
                                   │ tea.Cmd (async), typed tea.Msg reply
                                   ▼
   ┌───────────────────────────────────────────────────────────────────┐
   │        internal/tuikit (Backend-seam only, NO uploader import)     │
   │                                                                     │
   │  wizardModel.testPhase: testIdle -> testUpload (NEW) -> testRunning1│
   │       -> testStage1 -> testRunning2 -> testStage2 -> step 2         │
   │                                                                     │
   │  step 0's sshForm gains a checkbox row (new focus slot, D-01)       │
   │  wizardFooter() gains a `testUpload` case + step-0 "u" hint         │
   │                                                                     │
   │  identPane gains `paneRegisterKey` (Identity Manager modal, D-08)   │
   │  keyCeremonyFor() gains the D-04 delete-offer sub-beat               │
   └──────────────────────────┬────────────────────────────────────────┘
                              │ view DTOs (UploadCheckboxView, UploadResultView …)
                              ▼
                   rendered TUI (cmd/gitid) / FixtureBackend (cmd/gitid-dummy)
                              │
                              ▼
                   e2e/create_flow_pty_e2e_test.go (+ new upload cases)
                   e2e/identity_manager_pty_e2e_test.go (+ register-key-modal)
                   internal/screenshot/createflow.go-class capture (DLV-04/06)
```

Entry points: the wizard's `enter` keystroke on `testIdle` (create) and the
key-ceremony pane's own pre-test beat (rotate/repair) are the two places the
new `testUpload` sub-beat is entered. The CLI's new verb is a THIRD,
independent entry point that never touches `internal/tuikit` at all (no TUI
render, per UI-SPEC's own table).

### Substrate Reality Check (read before planning any task)

`[VERIFIED]` — confirmed by direct `Read`/`grep` this session, not from CONTEXT.md's
own claims:

1. **`internal/upload/upload.go`** and **`internal/uploader/uploader.go`** exist,
   compile, and are covered ONLY by their own package tests
   (`internal/upload/upload_test.go`, `internal/uploader/uploader_test.go`).
   `grep -rn "internal/uploader\|internal/upload\b" --include="*.go" .` (excluding
   `.planning/` and `archive/`) returns **zero production callers**.
2. **`cmd/gitid/copy.go` and `tui/copy.go` do not exist in the live tree.** They exist
   only under `.planning/archive/0.0.1-poc-product-features-in-tui/{cmd-gitid,tui}/copy.go`.
   `git log --oneline -- cmd/gitid/copy.go tui/copy.go` shows the last real-tree commit
   touching them is `d60a4d7 feat(03-03): archive the POC surface and open the real app
   shell...` — Phase 3 deliberately removed them.
3. **`cmd/gitid/main_test.go`'s `TestNewRootCmdArchivedPOCCommandsAreGone`** asserts
   `{"copy"}` and `{"upload"}` (among others) are NOT registered as commands — this is a
   locked regression test the planner's new verb must not violate (i.e., don't literally
   resurrect `gitid copy`; either register it under the `identity` noun group per Phase 5's
   grammar — `gitid identity copy` / a flat alias — and delete this line from the archived
   list, or pick a distinct name; either way this test file needs a deliberate, reviewed
   edit, not an accidental collision).
4. **`internal/tuikit/views.go:8`** documents the no-backend-import contract by name:
   *"internal/tuikit imports ZERO first-party backend packages — no identity, tester,
   sshconfig, keygen, filewriter, doctor, adopter, platform, clipboard or uploader."*
   `[VERIFIED: internal/tuikit/views.go:1-20]` — `uploader` is explicitly named in this
   list already, meaning the DTO-conversion-in-wiring.go rule was anticipated for this
   phase. Any new upload-related Backend method must return a `tuikit`-local DTO
   (`UploadCheckboxView`, `UploadResultView`, …), never an `uploader.PerTypeResult` or
   `uploader.Tool` directly.
5. **`placeOverlay` does not exist anywhere in the current source tree.**
   `grep -rln "placeOverlay" --include="*.go" .` (excluding `.planning/` and `bin/`
   binaries) returns nothing, in neither `internal/tuikit` NOR `internal/dummytui`.
   `internal/dummytui/` currently contains only `data.go`, `data_test.go`, `doc.go`,
   `fixturebackend.go`, `nobackend_test.go` — no `model.go`, no screen registry. The
   Phase-2-era compositing code `placeOverlay` belonged to was deleted when Phase 3
   replaced the dummy's own Bubble Tea model with a `FixtureBackend` implementing the
   SAME `tuikit.Backend` interface `cmd/gitid` uses (this is also why STATE.md's Phase-4
   finding CR-15 says "the visual-regression gate cannot detect a shared-renderer defect,
   since `cmd/gitid` and `cmd/gitid-dummy` render Configure-Git through the SAME
   `internal/tuikit` code"). 09-UI-SPEC.md's Identity-Manager modal row ("reuses
   `placeOverlay` pixel-for-pixel") is describing a mechanism that no longer exists —
   see Common Pitfalls #5 for the concrete substitute.

### Pattern 1: The `wizardModel.testPhase` state-machine insertion point

**What:** step 1's `enter`-on-`testIdle` handler currently does staging + jumps straight
to `testRunning1` in one keystroke.

```go
// Source: internal/tuikit/identities.go:3233-3238 (verbatim, current on disk)
case "enter":
    switch w.testPhase {
    case testIdle:
        w.testPhase = testRunning1
        m.wizard = w
        return keyResult{model: m, handled: true, cmd: w.backend.TestStage1(w.spec())}
```

**When to use:** this is the D-05 insertion point. The new sub-beat must intercept
BEFORE `TestStage1` fires, because `TestStage1` itself does the key staging
(`b.stagedKeyFor`) that the upload step needs the `.pub` path from:

```go
// Source: cmd/gitid/wiring.go:1073-1085 (verbatim, current on disk)
func (b *realBackend) TestStage1(spec tuikit.CreateSpec) tea.Cmd {
    return func() tea.Msg {
        in := b.createInputFromSpec(spec)
        staged, err := b.stagedKeyFor(in, spec.ReuseKeyPath)
        if err != nil {
            return b.stageFailure(1, b.Stage1Command(spec), err, in)
        }
        res := b.deps.PreWrite(staged.TempPrivatePath, in.Hostname, in.Port)
        view := toTestResultView(res, res.Command)
        b.recordOutcomeFor(1, view.Outcome, in)
        return tuikit.WizardStageMsg{Stage: 1, Result: view}
    }
}
```

**Recommended shape:** add a NEW Backend method, e.g. `StageForUpload(spec)
tea.Cmd`, that calls `b.stagedKeyFor` ONCE and returns a new `tuikit.UploadStagedMsg`
carrying the `.pub` path — then `TestStage1` must NOT re-call `stagedKeyFor` a second
time for the same spec (it is idempotent per `stagedKeyFor`'s own doc comment at
`wiring.go:4273-4281`, "keyed on BOTH the ..." — re-staging is safe but wasteful, not
a correctness bug; still, sharing ONE staged result between the upload sub-beat and
`TestStage1` avoids a second key-material read). `w.testPhase` gains one new constant
`testUpload = "upload"`, inserted between `testIdle` and `testRunning1` in both the
`const (...)` block (`identities.go:1046-1054`) and this switch.

### Pattern 2: The focus-index `iota` chain (step 0's checkbox insertion)

**What:** step 0's Tab-ring focus slots are NOT independently numbered — they are a
chained `iota`-style sequence, each one computed from the previous:

```go
// Source: internal/tuikit/identities.go:116-156 (verbatim, current on disk)
const (
    sshFieldPrefix = iota
    sshFieldHost
    sshFieldHostname
    sshFieldPort
)
// ...
const (
    wizardFocusKeySource  = sshFieldPort + 1
    wizardFocusKeyBody    = wizardFocusKeySource + 1
    wizardFocusManualPath = wizardFocusKeyBody + 1
)

func wizardStep0FocusRing(keySource int) int {
    if keySource == keySourceReuse {
        return wizardFocusManualPath + 1
    }
    return wizardFocusKeyBody + 1
}
```

**When to use:** UI-SPEC pins the checkbox as "the LAST row of the SSH form, after
Port" — i.e. it must land at `sshFieldPort + 1`, which is CURRENTLY
`wizardFocusKeySource`'s value. Inserting it means EITHER (a) renumber the whole chain
(`wizardFocusUploadCheckbox = sshFieldPort + 1`, then rebase every subsequent constant
off it), which is mechanical but touches every literal comparison against these
constants project-wide, OR (b) give the checkbox its own out-of-band boolean toggle
(the `u` key, NOT part of the Tab ring) and leave the `iota` chain untouched. UI-SPEC's
own wording ("New intra-step key: `u` toggles the checkbox... No new key is needed
inside `testUpload`") reads as consistent with (b) — a global step-0 hotkey, like
`space` toggles `simulateFail` today (`identities.go:3210-3215`) WITHOUT occupying a
Tab-ring focus slot. **Recommend (b)** unless the UI review explicitly requires
click-to-focus / Tab-reachability on the checkbox row (Checkpoint-2's D8 "click-to-focus
on every form row" precedent argues FOR a real focus slot — this is exactly the kind of
call 09-UI-SPEC.md left implicit and the planner should resolve explicitly, flagging it
as a plan-time decision with a one-line rationale either way).

### Pattern 3: Backend interface — `tea.Cmd`-returning async methods vs. pure sync ones

**What:** the `Backend`/`IdentityPlanner` interfaces mix two method shapes:

```go
// Source: internal/tuikit/backend.go:419-462 (signatures, verbatim)
TestStage1(spec CreateSpec) tea.Cmd
TestStage2(spec CreateSpec) tea.Cmd
CreateWritePlan(spec CreateSpec, git *GitSpec) WritePlanView   // sync, pure
CopyPublicKey(pubKeyPath string) (note string, err error)      // sync, side-effecting
```

**When to use:** the D-01 checkbox derivation (`Provider`/`Hostname` known, needs
`DetectFor`+`AuthCheck`) must be **synchronous** — it renders on step 0 the instant the
provider is known, with no spinner precedent anywhere in this codebase for that kind of
check (confirmed: `TestStage1`/`TestStage2` show no spinner either — UI-SPEC's own
"loading" row already flags this as a 🧪 backstop item). Model it as a sync method
(`UploadEligibility(hostname string) UploadEligibilityView`, or similar), mirroring
`CreateWritePlan`'s sync-pure shape — NOT a `tea.Cmd`. The actual upload run
(`announcing`→`per-key-results`) IS a subprocess round trip and MUST be a `tea.Cmd`
delivering a typed `tea.Msg` (mirroring `TestStage1`'s `WizardStageMsg` pattern), because
Bubble Tea's Update loop must stay non-blocking.

### Pattern 4: `identPane` — the real "modal" mechanism (replaces UI-SPEC's `placeOverlay` citation)

```go
// Source: internal/tuikit/identities.go:41-56 (verbatim, current on disk)
type identPane int

const (
    paneDetail identPane = iota
    paneCreate
    paneEditSSH
    paneEditCeremony
    paneGit
    paneGitCeremony
    paneClone
    paneDeleteScope
    paneDelete
    paneFix
    paneActions
    paneKeyCeremony
)
```

Two switch statements dispatch on it: `handleKey` (`identities.go:2124-2145`) and
`view` (`identities.go:4326+`, cases at `4399`, `4419`, `4460`). **When to use:** the D-08
"Identity Manager copy-modal" (UI-SPEC's `register-key-modal`) is a NEW `identPane` value
(e.g. `paneRegisterKey`), added to this const block, with one new `case` in each switch —
the exact same shape as `paneClone`/`paneDeleteScope`, NOT a call into a nonexistent
overlay-compositing primitive.

### Pattern 5: The CLI verb — `identityVerb`/`newVerbCmd`/`depthResolver`

**What:** every write verb (`create`, `clone`, `rotate`, `new-key`, `delete`) is a single
`identityVerb{use, aliases, short, args, bindFlags, run}` spec, built into TWO
`*cobra.Command` objects (noun-form child + flat root alias) by `newVerbCmd`, added once
to `identityVerbSpecs()` (`cmd/gitid/identity.go:61-71`). Confirmed the CURRENT verb list
via `main_test.go`'s `TestNewRootCmdSurfaceIsPhase5CLI`: `create`, `clone`, `new-key`,
`rotate`, `delete`, `list`, `show` — **no `copy` verb exists today.**

**When to use:** the new upload/register CLI surface (UP-02's "shown command == run
command" + D-06's `--no-upload`/`--dry-run`) MUST be built as a new `identityVerb` entry,
reusing `depthResolver` (`identity.go:131-155`) for the D-02 headless/pre-filled-TUI/
missing-flags trichotomy, and `confirmationPolicyFrom` (`identity.go:197-205`) if the
verb performs any write beyond the upload itself. A bespoke `&cobra.Command{}` (the
archived `copy.go`'s own pattern — `newCopyCmd()`, a hand-rolled command with its own
`RunE`) would silently diverge from every other verb's flag/confirmation/parity
semantics; this is exactly the class of drift `identity.go`'s own doc comment (line
17-27) warns "keeps their flags... byte-identical" is why the two-cobra-command-per-spec
pattern exists.

### Recommended Project Structure (files this phase touches — no new directories)

```
internal/uploader/
├── uploader.go          # DetectFor (replaces Detect), Inventory (NEW, D-15),
│                         # UploadKey -> per-type result struct (D-16), GLabKeyTypeForAuth rename (D-12)
└── uploader_test.go

internal/upload/
└── upload.go             # UNCHANGED — Instructions() reused verbatim

cmd/gitid/
├── wiring.go              # buildUploaderDeps() (real exec closures), new realBackend
│                          # methods (UploadEligibility, StageForUpload/RunUpload)
├── wiring_test.go          # NEW: TestUploaderDepsEveryFieldIsWired (nil-guard, see Pitfall 1)
├── identity.go             # identityVerbSpecs() gains the new verb
├── identity_upload.go      # NEW file, following identity_create.go/identity_key.go naming
└── main_test.go            # TestNewRootCmdArchivedPOCCommandsAreGone: remove/replace the
                             # {"copy"} entry deliberately, or pick a name that doesn't collide

internal/tuikit/
├── identities.go           # wizardModel.testPhase gains testUpload; sshForm/focus changes;
│                           # wizardFooter() gains a testUpload case + step-0 "u" hint;
│                           # identPane gains paneRegisterKey; keyCeremonyFor() gains D-04
├── backend.go               # Backend interface gains the new methods (sync + tea.Cmd)
├── views.go                 # NEW DTOs: UploadEligibilityView, UploadAnnounceView,
│                            # UploadResultView, RotateDeleteOfferView
└── design.go                 # NEW frozen-copy constants (Copywriting Contract strings)

internal/dummytui/
└── fixturebackend.go        # implements the new Backend methods with fixture data
                             # (NO placeOverlay — see Pattern 4)

e2e/
├── create_flow_pty_e2e_test.go     # NEW cases: checkbox states, testUpload sub-beat
└── identity_manager_pty_e2e_test.go # NEW case: register-key-modal, rotate delete-offer
```

### Anti-Patterns to Avoid

- **Resurrecting `cmd/gitid/copy.go` verbatim:** it predates the D-02 adaptive-depth
  resolver, the D-11/D-16 per-type result struct, and the Phase-5 `identityVerb`
  infrastructure entirely. Copying it forward would reintroduce the exact "first-found
  routing" bug D-11 exists to fix (its `runCopy` calls `uploader.Detect`, not
  `DetectFor`) and bypass the parity-matrix discipline every other verb follows.
- **Calling `placeOverlay`:** does not exist (Pattern 4). Do not plan a task that
  references it by name.
- **Re-deriving the checkbox state inside `testUpload`:** UI-SPEC is explicit — "nothing
  about D-01's derivation logic re-runs" once step 1 is reached; the checkbox's resolved
  boolean travels forward as plain wizard state, not re-probed.
- **A second, independent `stagedKeyFor` call inside the new upload Cmd** when
  `TestStage1` already exists and stages the same key — wasteful and risks the two
  staged results silently diverging if `stagedKeyFor`'s fingerprint inputs ever change
  independently. Share one staged result.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| `gh`/`glab` argument construction | A second copy of the arg-slice logic inside the TUI or CLI layer | `internal/uploader.buildArgs`/`CommandPreview` (already the single source both `UploadKey` and the preview render from) | Shown==run is STRUCTURAL only if there is exactly one function building the args — a second copy is exactly the class of drift D-02 exists to prevent |
| Manual-fallback instruction text | A second GitHub/GitLab instructions string in `internal/tuikit` | `internal/upload.Instructions(provider)`, imported unchanged | Already provider-templated, already avoids an import cycle between `cmd/gitid` and `tui` (well, now `internal/tuikit`); UI-SPEC requires byte-identical reuse |
| SSH connectivity re-verification after upload (D-17) | A bespoke post-upload probe | `internal/tester.PreWrite`/`ResolvedVia` (the SAME functions the wizard's own test stages call) | `ReachableNotUploaded → PASS` is already `tester`'s own three-way classification (`PASS`/`ReachableNotUploaded`/`Failure`, `internal/tester/tester.go:14-23`) — no new classifier needed |
| The D-02 headless/interactive CLI trichotomy | A new flag-presence check per verb | `depthResolver`/`confirmationPolicyFrom` (`cmd/gitid/identity.go:131-205`) | Already generalized and exhaustively tested for every other write verb; a bespoke check would be the ONE verb with different edge-case behavior |
| Nil-guard for the new `uploader.Deps` wiring | Trusting the composition root by inspection | The `reflect.ValueOf`-over-every-func-field pattern (`cmd/gitid/wiring_test.go:98-114`, `TestIdentityDepsEveryFieldIsWired`) | This project has hit this exact defect class twice already (project MEMORY.md: "Doctor injected-seam wiring blindspot", RECURRING across Phase 4 and Phase 5) — a nil `RunCmd` field silently no-ops instead of panicking |

**Key insight:** almost everything this phase needs at the exec/classification layer
already exists and is already correct — the risk is entirely in the WIRING (new call
sites, new Deps struct, new nil-guard test, new CLI verb scaffolding) and in the TUI
STATE-MACHINE insertion (the `iota` focus chain, the `testPhase` chain, the `identPane`
chain) — three places this codebase deliberately encodes as parallel, hand-maintained
sequences rather than data-driven structures, which is exactly where a careless insert
silently shifts or collides with an existing value.

## Common Pitfalls

### Pitfall 1: The nil-Deps-field wiring blindspot (RECURRING project-wide)

**What goes wrong:** `uploader.Deps{LookPath, RunCmd}` gets built in `wiring.go`, but a
field is left nil (typo, copy-paste from a different Deps struct, forgotten during a
later refactor). No compile error, no panic — `Detect`/`UploadKey` just silently treat
`nil` as "no tool found" or crash only when actually invoked from a live TUI/CLI
session, never from a fake-Deps unit test.
**Why it happens:** Go's zero value for a func field is `nil`, and every unit test
supplies its own fake `Deps` — so a real-constructor gap is invisible to `go test`
unless something specifically asserts on the REAL constructor's output.
**How to avoid:** add `TestUploaderDepsEveryFieldIsWired` mirroring
`TestIdentityDepsEveryFieldIsWired` exactly (`cmd/gitid/wiring_test.go:98-114`) —
`reflect.ValueOf(buildUploaderDeps())`, iterate every `Func`-kind field, fail loudly by
name if nil.
**Warning signs:** a PTY e2e test that "detects" `gh` as absent even when it's actually
on PATH in CI — that's this bug, not a real environment gap.

### Pitfall 2: The chained `iota` focus-index constants desyncing

**What goes wrong:** `wizardFocusKeySource = sshFieldPort + 1` and its two dependents are
literal-computed from `sshFieldPort`. Any new focus slot inserted BEFORE
`wizardFocusKeySource` without rebasing the chain leaves `wizardFocusKeySource` pointing
at the WRONG UI element (the new row silently gets focus meant for "Generate/Reuse", or
vice versa) — a bug that will not fail to compile and may not even fail an existing test
if that test never asserts on the literal numeric value.
**Why it happens:** the constants are deliberately chained (not independently assigned)
so the whole block shifts together when `sshFieldPort` changes — but a NEW value
squeezed into the middle breaks that invariant unless every downstream constant is
touched in the same commit.
**How to avoid:** either (a) rebase the ENTIRE chain in one commit and grep for every
literal comparison against the shifted constants (`wizardFocusKeySource`,
`wizardFocusKeyBody`, `wizardFocusManualPath`, and `wizardStep0FocusRing`'s two return
values), or (b) — the recommended path per Pattern 2 — keep the checkbox OUT of the Tab
ring entirely (a `u`-keyed toggle, not a focus slot), avoiding the chain altogether.
**Warning signs:** `wizardStep0FocusRing` returning the wrong ring size (Tab wraps one
slot early or late); a PTY e2e test's Tab-key assertions failing intermittently only in
the reuse-key branch (which uses the `+1` longer ring).

### Pitfall 3: `actionMenuLabels()` and `actionMenuRows` are two separate hand-maintained values

**What goes wrong:** `actionMenuLabels()` (`identities.go:2406-2413`) returns a
`[]string` of 4 entries; `actionMenuRows = 4` (`identities.go:58`) is an INDEPENDENT
constant used elsewhere for layout math. Adding a 5th row (D-08's `action_register_key`)
to the labels slice without also updating `actionMenuRows` is a classic parallel-constant
desync — the menu will render 5 labels inside a layout budgeted for 4.
**Why it happens:** the row COUNT and the row CONTENT are tracked in two places instead
of one (`len(actionMenuLabels())` would have been self-synchronizing; it is not what the
code does today).
**How to avoid:** grep every use of `actionMenuRows` before touching
`actionMenuLabels()`, and update both in the same commit; consider replacing
`actionMenuRows` with `len(actionMenuLabels())` at the call site as a drive-by
correctness fix if the plan's scope tolerates it (flag as a scoped, reviewed change, not
an accidental side effect).
**Warning signs:** the register-key-modal renders visually correct in isolation but the
action-menu PANE (not the modal) overflows its row budget once the 5th label exists.

### Pitfall 4: `gh auth refresh` does not automatically retry the failed upload (real, reported upstream behavior)

**What goes wrong:** `gh auth refresh -h <host> -s admin:public_key` opens a browser
flow to ADD the scope to the existing token, but does not itself re-run the SSH key
upload — a real GitHub CLI issue (cli/cli#7738) reports exactly this confusion ("Does
not add ssh key to Git"). If `UploadScopeRemediationAuth`'s copy (UI-SPEC) implies the
key gets added automatically once scopes are refreshed, that is incorrect.
**Why it happens:** scope refresh and key upload are two independent `gh` operations;
nothing chains them.
**How to avoid:** the copy already says "...then retry from the Identity Manager" (UI-
SPEC's `UploadScopeRemediationAuth`/`UploadScopeRemediationSigning` strings) — keep that
explicit "retry" instruction; do not let an implementation shortcut silently auto-retry
inside the SAME `gh auth refresh` call (it cannot).
**Warning signs:** a user reports the key never got added even after following the
scope-remediation hint — check whether the re-run actually happened, not whether the
scope grant happened.
`[CITED: github.com/cli/cli issue #7738]`

### Pitfall 5: `gh ssh-key add --type` defaults to `"authentication"` — an unset `--type` silently uploads the WRONG type for the signing registration

**What goes wrong:** `gh ssh-key add [<key-file>] --title <t> --type <type>` — `--type`
accepts `{authentication|signing}` and defaults to `"authentication"`
`[CITED: cli.github.com/manual/gh_ssh-key_add]`. `buildArgs` (Pattern/Code Examples
below) always passes `--type` explicitly today, so this is not a live bug — but any NEW
call site (the D-15 inventory function, a future convenience wrapper) that constructs a
`gh ssh-key add` invocation WITHOUT the explicit flag will silently register an
authentication key when a signing key was intended.
**How to avoid:** never add a second `gh ssh-key add` call site outside
`buildArgs`/`UploadKey` (see Don't Hand-Roll's first row).

### Pitfall 6: 09-CONTEXT.md's/09-UI-SPEC.md's own file citations describe archived or nonexistent code — verify before grounding a plan task in them

**What goes wrong:** `09-CONTEXT.md`'s "Built substrate" section cites `cmd/gitid/copy.go`,
`tui/copy.go` as files "this phase modifies" — neither exists in the live tree (Substrate
Reality Check #2). `09-UI-SPEC.md` cites `placeOverlay` for the Identity Manager modal
(Substrate Reality Check #5) — does not exist. Both documents are otherwise accurate and
were clearly grounded in SOME real reading (the `identities.go` line numbers for
`wizardSteps`, `keyCeremonyGraceHintFmt`, `stageWarningLine`, `actionMenuLabels`, etc.
are all independently `[VERIFIED]` correct this session) — but these two specific
citations describe an earlier project state (the archived POC, and the Phase-2 dummytui
model) that no longer matches the current tree.
**Why it happens:** `09-CONTEXT.md` was gathered 2026-07-08, well before Phase 3's
archival commit (`d60a4d7`) and before `internal/dummytui`'s Phase-3+ consolidation onto
`FixtureBackend` — the discuss-phase session's own substrate description was accurate
AT THE TIME but the codebase moved since.
**How to avoid:** the planner must re-verify every file:line citation in `09-CONTEXT.md`
and `09-UI-SPEC.md` against the current tree before writing a task's `read_first` list —
do not trust either document's substrate claims without a fresh grep/read, even though
both are otherwise binding for PRODUCT decisions.

## Code Examples

### The exact `gh`/`glab` argument construction (already correct, do not duplicate)

```go
// Source: internal/uploader/uploader.go:164-185 (verbatim, current on disk)
func buildArgs(tool Tool, pubPath, title, keyType string) ([]string, error) {
	switch tool {
	case ToolGH:
		// gh ssh-key add <key-file> --title "gitid: <name>" --type authentication|signing
		return []string{"ssh-key", "add", pubPath, "--title", title, "--type", keyType}, nil
	case ToolGLab:
		// glab ssh-key add <key-file> -t "gitid: <name>" --usage-type auth
		return []string{"ssh-key", "add", pubPath, "-t", title, "--usage-type", keyType}, nil
	default:
		return nil, fmt.Errorf("uploader: unknown tool %d", tool)
	}
}
```

`gh ssh-key add` confirmed flags (`[CITED: cli.github.com/manual/gh_ssh-key_add]`):
`-t, --title <string>` and `--type <string>` (values `authentication`/`signing`, default
`"authentication"`).

`glab ssh-key add` confirmed flags (`[CITED: docs.gitlab.com/cli/ssh-key/add/]`):
`-t, --title <string>` (required) and `-u, --usage-type <string>` (values
`auth`/`signing`/`auth_and_signing`, default `"auth_and_signing"`). This CONFIRMS D-12's
decision is current: glab's OWN default is already `auth_and_signing`.

### The real exec closure pattern (arg-slice, no shell — reuse for `buildUploaderDeps`)

```go
// Source: .planning/archive/0.0.1-poc-product-features-in-tui/cmd-gitid/copy.go:186-203
// (archived — cited as the KNOWN-GOOD pattern to re-derive in the new composition-root
// function, not to copy the surrounding runCopy/newCopyCmd code around it)
func buildUploaderDeps() uploader.Deps {
	return uploader.Deps{
		LookPath: exec.LookPath,
		RunCmd: func(name string, args ...string) (string, int, error) {
			cmd := exec.Command(name, args...) //nolint:gosec // arg-slice; no shell; name is a trusted resolved binary path (G204)
			out, err := cmd.CombinedOutput()
			output := string(out)
			if err == nil {
				return output, 0, nil
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return output, exitErr.ExitCode(), nil
			}
			return "", 2, err
		},
	}
}
```

### The nil-guard wiring test to replicate for the new `uploader.Deps` seam

```go
// Source: cmd/gitid/wiring_test.go:98-115 (verbatim pattern, adapt struct/constructor name)
func TestUploaderDepsEveryFieldIsWired(t *testing.T) {
	deps := buildUploaderDeps()

	v := reflect.ValueOf(deps)
	typ := v.Type()
	if typ.NumField() == 0 {
		t.Fatal("uploader.Deps has no fields; the guard would be vacuous")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.Func {
			continue
		}
		if v.Field(i).IsNil() {
			t.Errorf("uploader.Deps.%s is nil in the REAL constructor — a nil seam silently changes behavior (L2)", field.Name)
		}
	}
}
```

### The `tester.Outcome` three-way classification this phase's D-17 verification reuses unmodified

```go
// Source: internal/tester/tester.go:10-23 (verbatim, current on disk)
type Outcome int

const (
	// PASS — the key is already authorized: "successfully authenticated".
	PASS Outcome = iota
	// ReachableNotUploaded — host reachable but key not yet uploaded:
	// "Permission denied (publickey)". Expected for a brand-new key; the create
	// flow proceeds (D-02).
	ReachableNotUploaded
	// Failure — connection refused, DNS failure, timeout, etc. Abort, no write.
	Failure
)
```

The tuikit-local mirror (the DTO the TUI actually renders against — `TestOutcomePass` /
`TestOutcomeReachableNotUploaded` / `TestOutcomeFailure`) is
`[VERIFIED: internal/tuikit/views.go:42-57]`.

### `wizardFooter`'s per-state hint pattern (extend for `testUpload`)

```go
// Source: internal/tuikit/identities.go:4508-4529 (verbatim, current on disk)
case 1:
    switch w.testPhase {
    case testIdle:
        return []FooterAction{{Key: "Enter", Label: "run stage 1"}, {Key: "space", Label: "toggle failure demo"}}
    case testFailed:
        return []FooterAction{{Key: "Enter", Label: "retry"}}
    case testStage1:
        actions := []FooterAction{{Key: "Enter", Label: "run stage 2"}}
        if w.copyable() {
            actions = append(actions, FooterAction{Key: "c", Label: "copy public key"})
        }
        return actions
    // ... a NEW `case testUpload:` branch belongs here, mirroring this shape —
    // likely a bare `return nil` (D-02: no prompt, auto-advances) or a single
    // informational hint, never an actionable key the user must press.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `uploader.Detect` first-found routing (gh preferred, unconditionally) | `uploader.DetectFor(provider)` — provider-scoped, never cross-routes | This phase (D-11) | Fixes the documented bug: a GitLab identity with an authenticated `gh` and unauthenticated `glab` currently silently "succeeds" against the wrong provider |
| `GLabKeyTypeForAuth = "auth"` (conservative fallback, chosen because glab wasn't available to verify at write time) | `"auth_and_signing"` — confirmed as glab's OWN default via official docs this session | This phase (D-12) | The old value actively DOWNGRADES from glab's default and silently drops signing registration |
| `tui/copy.go`'s per-key Enter/skip prompt queue | Announce-and-do (D-02), no prompt | This phase (D-10 explicitly calls the old pattern a "contradiction" with UP-03) | The archived pattern cannot simply be un-archived; it must be rebuilt to the new contract |
| Static Phase-2 reference PNGs (`REFERENCE-INDEX.md`) | Live-demo-is-the-reference; each UI phase (6, 7, 8, and now 9) mints its own fresh approved captures | Phase 2 closeout | D-09: Phase 9's visual gate baseline does not exist yet and must be minted as part of this phase's own UI wave, same as Phases 6-8 did |

**Deprecated/outdated:**
- `cmd/gitid/copy.go`/`tui/copy.go` (archived POC forms): superseded by the `identityVerb`
  CLI pattern and the shared upload-section TUI component this phase builds from
  scratch — do not resurrect verbatim (Anti-Patterns, Pitfall 6).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|----------------|
| A1 | The checkbox row should be a global `u`-keyed toggle OUTSIDE the Tab focus ring (Pattern 2, option b) rather than a real focus slot requiring the `iota` chain to be rebased. | Architecture Patterns §Pattern 2 | If the UI/plan-checker requires Tab/click-to-focus parity with every other form row (Checkpoint-2's D8), the chain-rebase path (option a) is correct instead, and every downstream constant/test asserting on `wizardFocusKeySource`'s numeric value needs updating in the same commit. |
| A2 | The new upload CLI verb should be named to avoid colliding with the archived, test-asserted-absent `copy` command (e.g. `gitid identity copy` reusing the noun-verb form, or a distinct verb name) rather than literally re-registering `copy` at the root. | Architecture Patterns §Pattern 5, Substrate Reality Check #3 | If the planner instead resurrects a root-level `gitid copy`, `TestNewRootCmdArchivedPOCCommandsAreGone` must be edited (removing that entry) as a DELIBERATE, reviewed change — not simply overridden — or CI will red on the very first commit. |
| A3 | `internal/tuikit`'s `Backend`/`IdentityPlanner` split should gain the new upload methods on `Backend` (not `IdentityPlanner`), since upload is closer to "the create/rotate write pipeline" than to the five existing planner-owned preview/write seams. | Architectural Responsibility Map, Architecture Patterns §Pattern 3 | If planning instead extends `IdentityPlanner`, the ownership-rule doc comment at `backend.go:16-20` ("Backend owns CommitDelete; IdentityPlanner owns KeyActionFor/DeletePlan/KeyCeremonyPlan/CommitRotate/CommitNewKey... does NOT absorb CommitDelete") would need an explicit, documented amendment — a bigger interface-contract change than this phase's scope implies. |

**If this table is empty:** N/A — see rows above; all are implementation-shape
recommendations flowing from verified facts, not unverified factual claims about the
provider CLIs or the locked product decisions (those are separately `[CITED]`/`[VERIFIED]`
throughout).

## Open Questions

1. **Does the D-15 inventory function need its own `Deps.RunCmd` reuse, or a second
   exec seam?**
   - What we know: `gh api user/keys`/`gh api user/ssh_signing_keys --jq` and
     `glab ssh-key list -F json` are just more subprocess invocations — `Deps.RunCmd`'s
     signature (`name string, args ...string`) already accommodates them.
   - What's unclear: whether `--jq` filtering should happen in the `gh` invocation
     itself (simpler Go-side parsing, but couples the arg slice to a specific JSON
     shape) or via a full `--json`-then-`encoding/json`-in-Go approach (more robust to
     API shape changes, more Go code).
   - Recommendation: prefer full JSON + `encoding/json` in Go (matches the codebase's
     stdlib-only convention) unless a concrete `gh api` response-shape edge case argues
     otherwise; this is explicitly Claude's Discretion per `09-CONTEXT.md`.

2. **Where exactly does `RotateDeleteOffer`'s key-ID lookup call the D-15 inventory —
   inside `KeyCeremonyPlan` (a sync preview) or a new post-confirm `tea.Cmd`?**
   - What we know: the delete offer appears on the ROTATE RESULT screen (after the new
     key already passed), per UI-SPEC's table — i.e., after `CommitRotate` already ran.
   - What's unclear: whether the old key's provider-side ID should be resolved
     eagerly (during `KeyCeremonyPlan`, before the user even confirms the rotate) or
     lazily (only once the result screen is reached and the offer is about to render) —
     the former risks a stale/wrong ID if inventory changed between plan and confirm;
     the latter is one more subprocess round-trip on an already-multi-stage screen.
   - Recommendation: lazy resolution (fetch fresh at result-screen time), consistent
     with D-18's "everything transient, computed fresh" and D-15's own dedupe-uses-fresh-
     inventory precedent.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `gh` (GitHub CLI) | UP-02/UP-03 autonomous GitHub upload | ✓ (this research machine) | 2.98.0 | Manual instructions (`internal/upload.Instructions`) — already built |
| `glab` (GitLab CLI) | UP-02/UP-03 autonomous GitLab upload | ✓ (this research machine) | 1.114.0 | Manual instructions — already built |
| `gh`/`glab` authenticated session | Full autonomous path (D-01 scenario 1) | Not probed this session (no live `gh auth status`/`glab auth status` network call was made — out of scope for static research; both packages already fake `Deps.RunCmd` in tests, and the planner should do the same, not depend on this machine's live auth state) | — | Unauthenticated → checkbox unchecked (D-01 scenario 2) or disabled (scenario 3), never blocks |

**Missing dependencies with no fallback:** none — both `gh` and `glab` are ABSENT-safe by
design (D-01 scenario 3, D-13's self-hosted gate); no code path in this phase requires
either CLI to exist.

**Missing dependencies with fallback:** both `gh` and `glab` themselves — see table.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + `-race`, project-wide |
| Config file | none — `go test ./...` driven via `Makefile` targets |
| Quick run command | `TERM=dumb SSH_AUTH_SOCK= go test -race ./internal/uploader/... ./internal/upload/... ./internal/tuikit/... ./cmd/gitid/...` |
| Full suite command | `make test` (unit, race) + `make test-e2e` (PTY) + `make lint` (golangci-lint incl. gosec) + `make gate-visual-regression` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|-------------|
| UP-01 | Manual instructions render for both providers | unit | `go test ./internal/upload/... -run TestInstructions` | ✅ (`internal/upload/upload_test.go`) |
| UP-02 | `DetectFor` never cross-routes; shown==run via shared `buildArgs` | unit | `go test ./internal/uploader/... -run TestDetectFor` (NEW — Wave 0 gap) | ❌ Wave 0 |
| UP-02 | Real `uploader.Deps` wiring has no nil field | unit (composition-root guard) | `go test ./cmd/gitid/... -run TestUploaderDepsEveryFieldIsWired` (NEW) | ❌ Wave 0 |
| UP-03 | Checkbox derived state (3 scenarios) renders correctly on step 0 | unit (`internal/tuikit`) | `go test ./internal/tuikit/... -run TestUploadCheckbox` (NEW) | ❌ Wave 0 |
| UP-03 | `testUpload` sub-beat auto-advances into `testRunning1` with no keystroke | PTY e2e | `go test -race ./e2e/... -run TestCreateFlow.*Upload` (NEW case in `create_flow_pty_e2e_test.go`) | ❌ Wave 0 |
| UP-01/UP-03 | Rotate delete-offer (D-04) defaults focus to "No"/Leave | PTY e2e | `go test -race ./e2e/... -run TestIdentityManager.*DeleteOffer` (NEW case in `identity_manager_pty_e2e_test.go`) | ❌ Wave 0 |
| DLV-04/DLV-06 | Real-vs-dummy semantic comparison for every new/amended screen | visual-regression gate | `make gate-visual-regression` (existing target, new screens registered per D-09) | ✅ (target exists; new fixtures are Wave-0-adjacent, not framework gaps) |

### Sampling Rate

- **Per task commit:** the quick-run command above, scoped to touched packages.
- **Per wave merge:** `make test && make test-e2e && make lint` (orchestrator obligation,
  per this project's own `LEARNINGS`/ground-rule-4 convention — never trust an executor's
  own PASS claim).
- **Phase gate:** full battery green (`go test -race ./...`, `make lint`, `make test-e2e`,
  `make gate-visual-regression`) before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/uploader/uploader_test.go` — add `TestDetectFor` (provider-scoped,
      never-cross-route cases) and tests for the new `Inventory`/per-type `UploadKey`
      result struct — covers UP-02/UP-03.
- [ ] `cmd/gitid/wiring_test.go` — add `TestUploaderDepsEveryFieldIsWired` (nil-guard,
      Pitfall 1) — covers UP-02.
- [ ] `internal/tuikit/identities_test.go` — add checkbox-derived-state unit coverage
      (3 scenarios × sync method, no PTY needed) — covers UP-03/D-01.
- [ ] `e2e/create_flow_pty_e2e_test.go` — add the `testUpload` sub-beat raw-keystroke
      cases (checkbox toggle via `u`, announce→results, manual-fallback) — covers
      UP-03/DLV-06.
- [ ] `e2e/identity_manager_pty_e2e_test.go` — add the register-key-modal and rotate
      delete-offer cases — covers UP-01/UP-03/D-04/DLV-06.
- [ ] `cmd/gitid/main_test.go` — DELIBERATE edit to `TestNewRootCmdArchivedPOCCommandsAreGone`
      (remove/replace the `{"copy"}` entry) in the SAME commit that registers the new
      verb — not a Wave-0 test file per se, but a locked-test edit every task touching
      the new CLI verb must account for.

## Security Domain

### Applicable ASVS Categories (Level 1, per `.planning/config.json`)

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | gitid never authenticates a user itself; it delegates entirely to `gh`/`glab`'s own OAuth-token storage. No credential is read, stored, or transmitted by this phase's code. |
| V3 Session Management | No | No session state introduced. |
| V4 Access Control | No | Single-user local CLI/TUI tool — no multi-principal access control surface. |
| V5 Input Validation | Yes | `pubPath` must NEVER be a private-key path — enforced by convention today ("SECURITY: only pubPath (.pub) is ever passed to uploader — never the private key", archived `copy.go:115`) and must be re-established as an explicit, testable invariant in the new call sites (e.g., a helper that refuses any path not ending `.pub`, or asserting the caller always derives it from `acct.PubPath`/`staged.TempPublicPath` never a raw private-key field). |
| V6 Cryptography | No | This phase performs zero cryptographic operations — key generation is Phase 1/3 substrate, untouched here. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Argument injection into `gh`/`glab` via a crafted identity name / title / hostname | Tampering | Already mitigated project-wide: every `exec.Command` call uses an explicit arg slice, never `sh -c` (gosec G204 comment on every call site, e.g. `buildUploaderDeps`'s `//nolint:gosec // arg-slice; no shell`) — preserve this discipline in every NEW call site this phase adds (the D-15 inventory calls, any future convenience wrapper). |
| Uploading the private key instead of the public key | Information Disclosure | V5 row above — keep the `.pub`-only convention explicit and, ideally, testable (a unit test asserting `UploadKey`/`Inventory` reject or never receive a path lacking `.pub`). |
| Leaking `gh`/`glab` raw CLI output containing sensitive local paths or a partial token into the UI/logs | Information Disclosure | D-14's "raw trimmed CLI output as last resort" fallback (`UploadResultFailed`) should be reviewed for whether `gh`/`glab` ever echo a token or absolute filesystem path in error output before it is rendered verbatim in the TUI/CLI — `gh`/`glab` auth tokens are not normally printed by `ssh-key add`/`auth status`, but this is worth a targeted check during implementation, not assumed safe by omission. |
| Cross-account key collision silently treated as success (GitLab) | Repudiation / Tampering (wrong identity bound) | Already correctly designed: D-15 explicitly classifies GitLab's `"has already been taken"` as a cross-account-conflict FINDING, never silent success — preserve this exact classification, do not simplify it to "any duplicate = skip". |

## Sources

### Primary (HIGH confidence — `[VERIFIED]`, read this session)

- `internal/upload/upload.go` — `Instructions(provider)`, full text quoted above
- `internal/uploader/uploader.go` — `Deps`, `Detect`, `AuthCheck`, `UploadKey`,
  `CommandPreview`, `buildArgs`, `GLabKeyTypeForAuth`
- `internal/tester/tester.go` — `Outcome`, `ClassifyPreWrite`
- `internal/tuikit/views.go` — no-backend-import contract, `TestOutcome`
- `internal/tuikit/identities.go` — `wizardSteps`, `testPhase` consts, the step-0 focus
  `iota` chain, `sshForm.handleEdit`/`setFocus`, `wizardFooter`, `identPane`,
  `actionMenuLabels`/`actionMenuRows`, `keyCeremonyGraceHintFmt`,
  `keyCeremonyFor`/`archivePathForKeyCeremony`, `stageWarningLine`,
  `keyUnusedResultMessage`
- `internal/tuikit/backend.go` — `Backend`/`IdentityPlanner` interface split and
  ownership doc comment
- `cmd/gitid/wiring.go` — `TestStage1`, `Stage1Command`/`Stage2Command`,
  `stagedKeyFor` doc comment
- `cmd/gitid/wiring_test.go` — `TestIdentityDepsEveryFieldIsWired` (nil-guard pattern)
- `cmd/gitid/identity.go` — `identityVerb`, `newVerbCmd`, `identityVerbSpecs`,
  `depthResolver`, `confirmationPolicyFrom`
- `cmd/gitid/main_test.go` — `TestNewRootCmdArchivedPOCCommandsAreGone`,
  `TestNewRootCmdSurfaceIsPhase5CLI`
- `internal/dummytui/` directory listing (only `data.go`, `data_test.go`, `doc.go`,
  `fixturebackend.go`, `nobackend_test.go` — no `model.go`)
- `git log --oneline -- cmd/gitid/copy.go tui/copy.go` — confirms `d60a4d7` as the
  archival commit
- `.planning/archive/0.0.1-poc-product-features-in-tui/cmd-gitid/copy.go` — the archived
  `runCopy`/`newCopyCmd`/`buildUploaderDeps` reference pattern
- `recipes/README.md` — the "one ed25519 key, auth + signing" model UP-01 automates
- `go.mod` — dependency list, Go 1.26 pin
- Local shell: `go version`, `gh --version`, `glab --version` (this session)

### Secondary (MEDIUM confidence — `[CITED]`, official docs)

- `cli.github.com/manual/gh_ssh-key_add` — `--title`/`--type` flags, default
  `"authentication"`
- `docs.gitlab.com/cli/ssh-key/add/` — `--title`/`--usage-type` flags, default
  `"auth_and_signing"`
- `cli.github.com/manual/gh_auth_status` (via search) — `-h/--hostname` flag semantics
- `github.com/cli/cli` issue #7738 — `gh auth refresh -s admin:public_key` does not
  itself re-run the SSH key upload
- GitHub REST API docs — `/user/ssh_signing_keys`, `read:ssh_signing_key` scope

### Tertiary (LOW confidence — flagged, not load-bearing for any plan decision)

- None — every claim above either traces to a file read this session or an official
  docs page. All product-behavior claims in `09-CONTEXT.md`/`09-UI-SPEC.md` are treated
  as locked and out of scope for re-verification, EXCEPT the two substrate citations
  corrected in "Substrate Reality Check" and Pitfall 6, which are corrections grounded
  in this session's own file reads, not speculation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies, stdlib-only, directly verified against `go.mod`
- Architecture: HIGH — every cited pattern is a verbatim quote from a file read this
  session, with line numbers
- Pitfalls: HIGH for the codebase-structural ones (focus-index chain, `placeOverlay`,
  archived `copy` command — all directly verified); MEDIUM for the `gh auth refresh`
  behavioral pitfall (community-reported issue, not independently reproduced this
  session)

**Research date:** 2026-08-28
**Valid until:** 30 days (stable domain — the codebase substrate this research grounds
itself in only changes when this phase's own plan executes; the `gh`/`glab` CLI facts are
version-pinned citations, re-verify if either CLI's major version changes before
execution)
