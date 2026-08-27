# 06-07 Task 2 — Cross-AI Review Packet for Phase 6 (Global SSH Options)

Assembled by the final plan's executor (06-07, Task 2) on `2026-08-27` for the
ORCHESTRATOR to consume at phase close. Following the Phase 3 convention
(`.planning/phases/03-create-flow-backend/03-06-SUMMARY.md`, "Cross-AI
visual-regression review").

**The cross-AI reviews have NOT been run.** This executor has no
subagent-spawning tools and cannot invoke the reviewer agents. Per the plan
(06-07-PLAN.md Task 2 `<action>`: "RUNNING the reviews is an orchestrator
obligation, not an executor one — the executor assembles the packet and cannot
spawn the reviewers") and the standing project rule in `.planning/STATE.md`,
running `agent-ui-ux-designer` / Codex / xai-grok against this packet is owed by
the **orchestrator** at wave close. The phase's last plan states this explicitly;
nothing in this packet pretends a review happened.

## Contents

- `frames/` — the 17 captured PTY frames from plans **06-04** (11) and **06-05**
  (6), snapshotted from `.planning/phases/06-global-ssh-options/ui-frames/`.
  These are the real-binary evidence surfaces for the two in-process-gate
  non-applicable receipt states and the `gss-storage-current` live no-op.
- `visual-divergence-allowlist.txt` — the classified divergence list from
  06-07 Task 1, snapshotted from `.planning/design/global-ssh/visual-divergence-allowlist.txt`
  (byte-identical to the file `make gate-visual-regression`'s
  `TestGlobalSSHAllowlistMatchesRegistry` enforces).
- `MANIFEST.md` — this file: frames index (Part A), allowed-divergence summary
  (Part B), the deviation/scoped-divergence manifest derived from the six prior
  summaries (Part C), and the `06-REVIEWS.md` closure table derived from the
  per-plan `<review_disposition>` sections (Part D), plus the cross-check that
  the table's row count equals the source document's finding count.

## Part A — Captured PTY frames (plans 06-04 and 06-05)

All 17 files are raw terminal captures driven over a real PTY against the
compiled `cmd/gitid` binary, per plan 06-04 Task 3 (`e2e/global_ssh_pty_e2e_test.go`)
and plan 06-05 Task 3 (`e2e/global_ssh_storage_pty_e2e_test.go`).

**Plan 06-04 (Options sub-tab / apply ceremony) — 11 frames:**

| Frame | Covers (real binary) |
|---|---|
| `global-ssh-browse.txt` | Options sub-tab browse after booting the Global SSH tab |
| `global-ssh-empty-selection.txt` | Empty opt-in selection (D-15) |
| `global-ssh-apply-cancel.txt` | Apply ceremony cancelled at the preview |
| `global-ssh-apply-confirm.txt` | Apply ceremony CONFIRMED — **the receipt state** |
| `global-ssh-shadowed-preview.txt` | Preview with a shadow warning (first-obtained-value) |
| `global-ssh-shadowed-receipt.txt` | Receipt carrying the shadow advisory |
| `global-ssh-later-directive.txt` | The paired negative: same option BELOW the Include → no warning |
| `global-ssh-probe-inconclusive-preview.txt` | Inconclusive simulation preview |
| `global-ssh-probe-inconclusive-receipt.txt` | Receipt when simulation was inconclusive |
| `global-ssh-commit-failure.txt` | Write failure path |
| `global-ssh-commit-retry.txt` | Retry after failure |

**Plan 06-05 (Storage & preview sub-tab / migration ceremony) — 6 frames:**

| Frame | Covers (real binary) |
|---|---|
| `storage-browse.txt` | Storage sub-tab browse (Include layout) |
| `storage-migrate-cancel.txt` | Migration ceremony cancelled at the preview |
| `storage-migrate-confirm.txt` | Migration ceremony confirmed (plan preview) |
| `storage-migrate-confirm-post.txt` | Post-commit storage browse — **the migration receipt state** |
| `storage-round-trip.txt` | Round-trip (migrate then migrate back) |
| `storage-changed-since-preview.txt` | `ErrConfigChangedSincePreview` refusal |

Receipt-state pointers: `global-ssh-apply-confirm.txt` is the evidence the
in-process gate's `gss-apply-receipt` spec names; `storage-migrate-confirm-post.txt`
is the evidence `gss-storage-migrate-receipt` names; `storage-browse.txt` is the
evidence `gss-storage-current`'s live non-applicability names. A test
(`TestGlobalSSHNonApplicabilityNamesExistingPTYFrame`) asserts each named file
exists under the phase's `ui-frames/` directory.

## Part B — The classified divergence allowlist (06-07 Task 1)

`visual-divergence-allowlist.txt` records **9 entries**, every one classified
`improvement` (the real backend renders live, seeded-fixture facts; the dummy
renders its frozen Phase-2 fixture data). Decisions are scoped to Phase-6
identifiers only (`DLV-4`, `GSSH-D-06`, `GSSH-D-07`, `STORE-03`). The three
REQUIRED entries named by 06-07-PLAN.md Task 1 are present:

| Entry | Divergence (from plans 06-01..06-06) | Classification |
|---|---|---|
| `T-06-GLOBALBLOCK` | 06-01's managed globals-block render: `IgnoreUnknown UseKeychain` guard line BEFORE `Host *`, two-space `hostIndent` body vs the frozen fixture's four-space literal, keys in `globalssh.Policy` declaration order (each of the three aspects has its own needle; removing any one fails the gate) | improvement |
| `T-06-CEREMONYTARGET` | 06-01's D-07 resolved-target-file ceremony heading (frozen `Write Host * managed block to ` prefix + resolved target), already registered in `gate-copy-freeze` | improvement |
| `T-06-PROVENANCE` | 06-01..06-03's real D-01/D-03 provenance, source-attribution and current-value strings (the dummy has no probe, so the real strings have no dummy equivalent) | improvement |

The other six entries (`T-06-OPTIONS-HEADER`, `T-06-OPTIONS-LIST-ROWS`,
`T-06-STORAGE-OTHER-HEADER`, `T-06-STORAGE-OTHER-LEFT`,
`T-06-STORAGE-OTHER-PREVIEW`, `T-06-APPLY-HEADER`, `T-06-STORAGE-MIGRATE-HEADER`,
`T-06-STORAGE-MIGRATE-DIFF`) classify fixture-vs-live divergences on the four
browse/preview screens (identity-count in the header status, option-row bodies,
storage radio state, and the planned-vs-frozen preview diffs). See the file for
the full per-entry rationale. The approved-HTML surface is non-applicable on
every Global SSH spec, cited to `AGENTS.md`'s standing Phase 3-10 UI-reference
rule (the Bubble Tea dummy is the sole parity target; HTML/MUI artifacts are
Phase-2 design history).

## Part C — Deviations and scoped divergences recorded by the six preceding summaries

Every deviation and scoped divergence the six summaries (06-01..06-06)
recorded, each attributed to the plan that recorded it. Scope deviations are
separated from auto-fixed/issues notes; none of these are open defects.

### Scoped divergences (deliberate, planned, registered in a gate or allowlist)

| Plan | Scoped divergence | Recorded in | Closed/owned by |
|---|---|---|---|
| 06-01 | Apply-ceremony heading names the RESOLVED storage target (D-07): `Write Host * managed block to ` + `GlobalSSHApplyPlanView.Targets[0]` — never a hardcoded path. Same shape as Phase-3 D-05. | 06-01-SUMMARY.md "Ceremony-heading copy divergence (D-07)"; `Makefile` `gate-copy-freeze` | `T-06-CEREMONYTARGET` (06-07) |
| 06-01 | Managed globals-block render (`EnsureGlobals` is the sole renderer after `RenderGlobalBlock` is deleted): guard-line placement, two- vs four-space indent, `Policy`-ordered keys. Real writer is correct; the frozen `managedHostStar` fixture is the stale side. | 06-01-SUMMARY.md "Real-vs-dummy globals-block text (REQUIRED for 06-07 Task 1 allowlist)" | `T-06-GLOBALBLOCK` (06-07) |
| 06-01 | Provenance/source-attribution/current-value strings (D-03 labels, D-13 version line, D-11/D-12 row states) have no dummy equivalent: the dummy has no probe. | 06-01/06-03 summaries | `T-06-PROVENANCE` (06-07) |

### Scope deviations (deviations from the plan's `files_modified`, all auto-fixed and required)

| Plan | Deviation | Why it happened / fix |
|---|---|---|
| 06-02 | `internal/sshconfig/writer.go` deleted the dead unexported `globalBlockName` alias (not in `files_modified`) | `migrationClasses`/`reorderGlobalLast` replaced its only callers; `make lint` `unused` would have failed on the dead const |
| 06-02 | `internal/doctor/checks/redundancy_test.go` gained the advice-text guard (not in `files_modified`) | The AC "redundancy advice names the current block, not the retired one" needed its natural home |
| 06-03 | `cmd/gitid/lifecycle.go` added (not in `files_modified`) | `runGlobalSSHApply` is the site that must refuse `VersionUnverified` `accept-new` (an AC required it) |
| 06-03 | `internal/tuikit/views.go`, `globalssh.go`, `globalssh_test.go` extended earlier than the plan's task split | Task 2's ACs (reason enum, `VersionNote` detail line) needed parts the plan listed under Task 3 |
| 06-03 | `batch3_test.go`, `backend_stub_test.go`, `fixturebackend.go`, `globalssh_test.go` follow-ons | `optionRow` signature change + IdentitiesOnly not selectable forced fixture/assertion updates |
| 06-03 | `Makefile` D-13 exclusion check assembled at runtime | `grep -qF -- 'Your OpenSSH:' Makefile` matched its own check line |
| 06-03 | `internal/globalssh/version.go` renamed `versionLess` → `minimum` | `revive redefines-builtin-id: min` |
| 06-04 | `EnsureIncludeLine` pre-existing Include-cycle parse-safety fix | Round-trip safety rejected a valid write when the existing config already had an unrelated cycle |
| 06-04 | Fresh Include-layout simulation appended its not-yet-created managed target | Mirror rewrites could not resolve the Include for a target that did not exist at discovery time |
| 06-04 | `findIncludeLine` empty-fields guard | Sliced a fields slice before checking the Include line had a value |
| 06-06 | `internal/tuikit` gained `NewAppOnGlobalSSH`, `ActiveTab`, `GlobalSSHUIState` (not in `files_modified`) | The frozen adaptive-depth contract requires opening Global SSH on a named sub-tab with nothing pre-selected |

### Process deviations (06-05, no scope impact; documented for institutional memory)

| # | Deviation (06-05) | Disposition |
|---|---|---|
| 1 | `internal/sshconfig/migrate.go` dead `rollback` helper removed | Orphanned after `rollbackTracked`; `restoreSnapshot` kept |
| 2 | Two lint/formatting fixes (goimports via `$(go env GOPATH)/bin/goimports`) | Directly applied |
| 3 | `renderStorage` build breakage recovered mid-flight | An earlier crash left preview helpers renamed for the dummy's use; relaunch finished the wiring |
| 4 | Task-2 AC test suite initially missing after the wiring commit | Orchestrator verified the gap and relaunched for the suite specifically |
| 5 | Fake `ssh` `find_identity_file` `case` pattern failed on indented directives | Shell `case` anchors at string start; strip leading whitespace first |
| 6 | PTY navigation used Shift+Right instead of plain Right arrow for sub-tab switch | Correct key sequence is `\x1b[C` |

### Observed values recorded as evidence (06-03)

- Concurrent probe latency under `-race`: typical 90.3–91.3ms, worst of five
  runs 91.3ms, against a 270ms sum-of-budgets bound — the "no loading
  affordance" decision and the cycle-2 accepted/bounded latency-assertion slack
  (see Part D, 06-03 cycle-2 LOW).
- `ssh -V` on this machine: readable `OpenSSH_9.9p2, LibreSSL 3.3.6`; unreadable
  version observed 0 times this phase (the `VersionUnverified` refusal path was
  exercised by injection, not by a real machine).

## Part D — Closure table for `06-REVIEWS.md`

This table is the index of every finding `06-REVIEWS.md` recorded, across every
review cycle present in the phase's documents at execution time (cycles 1-5).
The row count is **derived from the source documents**, not hard-coded; see
the "Source and count" subsection below for the exact cross-check.

The per-plan `<review_disposition>` sections (`.planning/phases/06-global-ssh-options/06-01-…`
through `06-07-…PLAN.md`) are the source of the findings; the current
`06-REVIEWS.md` (Cycle 5, the phase's last review cycle) records **zero
actionable findings** and is recorded as its own closed row below. For each
finding: the plan(s) that resolved it, the closing artifact, the acceptance
criterion that pins it, and the disposition (FIXED / ADDRESSED / ACCEPTED /
BOUNDED / MOVED / IMPLEMENTED / KEPT). Findings that were ACCEPTED or BOUNDED
rather than fixed are quoted with their rationale rather than omitted — a
closure table that listed only fixes would overstate the closure.

### Cycle 5 (from `06-REVIEWS.md` and 06-07-PLAN.md's `review_disposition`)

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| C5 | Cycle 5 converged to **zero actionable findings** (both reviewers; orchestrator independently re-verified the two Cycle-4 fixes against live source and agrees) — recorded here as a closure record, not a finding, hence unnumbered. | — | `06-REVIEWS.md` "Consensus Summary" | Review record; the phase is declared converged (14→9→4→2→0 across cycles 1-5) | CLOSED |
| 81 | MEDIUM — visual-gate scope risks mixing historical HTML back into the target | 06-07 | Every Global SSH spec's explicit `approved-html` non-applicability citing `AGENTS.md`'s Phase 3-10 UI-reference rule | `TestGlobalSSHHTMLNonApplicabilityPerSpec` (per-spec assertion) | FIXED |
| 80 | MEDIUM — the final wave is too broad; failures hard to attribute | 06-07 | Split the predecessor into 06-06 (CLI + matrix) and 06-07 (gate + battery + packet), each independently verified | Plan structure; separate `<verify>` per plan | FIXED (by split) |
| 79 | xai-grok MEDIUM — receipt states may be non-applicable in the in-process gate; the ROADMAP criterion still needs the PTY frames named as the evidence surface | 06-07 | Non-applicability record names a SPECIFIC PTY frame FILE (`global-ssh-apply-confirm.txt`, `storage-migrate-confirm-post.txt`) | `TestGlobalSSHNonApplicabilityNamesExistingPTYFrame` (file must exist under `ui-frames/`) | FIXED and STRENGTHENED |
| 78 | LOW — legacy `grep` in acceptance criteria | 06-07 | No acceptance criterion in this plan shells a grep at all | Manual (plan text uses no grep) | FIXED |
| 77 | Cross-plan — review findings must be checkably closed | 06-07 | This manifest's closure table, indexed from the per-plan `<review_disposition>` sections | This table's derived row count = 82 = source count (below) | ADDRESSED beyond the original ask |

### Cycle 4 (3 findings)

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 76 | LOW — the extraction says "replace the inline loop with a call to `HostPatternsMatch`" without stating the `host.Patterns` (`[]*ssh_config.Pattern`) → `[]string` projection | 06-04 | `<matcher_extraction>` call-site-projection section: four mechanical rules (`pat.String()` per pattern, `!` kept verbatim, zero-pattern/synthetic-wildcard guards stay at `aliasCollides`, no `ssh_config` type crosses the API) | `HostPatternsMatch({"*"}, …)` true + negation-exclusion assertion + unmodified `AliasCollision` suite | FIXED |
| 75 | MEDIUM — `SSHStorageMigrationPlan` (preview) acquires only `pendingMigrationMu`, never `txMu`; a preview concurrent with a migration can render a self-contradictory cross-file state | 06-05 | `<lock_contract>` rule 5: every cross-file READ takes `txMu`; preview holds it across layout resolution + `PlanMigration`; `runSSHStorageMigrate` banned from calling it; `txMu` redefined as "the two config files AS A PAIR" | Pause-injected concurrency criterion (via `newMigrateDeps`) asserting content coherence; construction check; `T-06-57` | FIXED |
| 74 | LOW (on plan 06-04) — the `host.Patterns` → `[]string` projection is implicit | 06-04 | Resolved inside 06-04's `<matcher_extraction>` (row 76); not 06-05's surface | Same criteria as row 76 | RESOLVED in 06-04 |

### Cycle 3 (4 findings)

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 73 | MEDIUM — the probe-failure criterion "none of the SIX rows is `StateAlreadySet`" can contradict the two criteria requiring UseKeychain/IdentitiesOnly identical to the all-succeed run | 06-03 | Scoped the assertion to the FOUR resolution-dependent rows named from the `Statuses` dependency map; per-source independence wins | Probe-failure fixture must have a legitimately already-set file-derived row and assert it STAYS already-set | FIXED |
| 72 | **HIGH** — the plan instructs `shadow.go` to reuse unexported `sshconfig.aliasCollides`; `AliasCollision` is path-shaped; `validation.go` not in the file list — cannot compile as written | 06-04 | Extracted shared `HostPatternsMatch`/`HostLineMatches`; `aliasCollides` refactored onto them; `validation.go`/`validation_test.go` added to file list, artifacts, key_links | Negated-pattern shadow case, matcher table test, `HostLineMatches` equivalence test, unmodified `AliasCollision` suite, comment-filtered grep proving no re-implementation | FIXED (by extraction) |
| 71 | MEDIUM — `BuildGraph`'s single global visited-path set misclassifies a legitimate DIAMOND Include graph as a cycle | 06-04 | Two-set split: popped recursion `active` (true cycles) vs never-popped memo `expanded` (diamonds deduped); linearisation deliberately does NOT dedupe | Diamond test: entry→{a,b}→shared; `Inconclusive` FALSE; `shared.config` once; directive nameable | FIXED |
| 70 | MEDIUM — `pendingMigration` "guarded by the same mutex discipline" + `runSSHStorageMigrate` taking `txMu` ⇒ self-deadlock on Go's non-reentrant `sync.Mutex`; invalidation points unstated | 06-05 | `<lock_contract>`: independent `pendingMigrationMu` (never across I/O / `tea.Cmd`), fixed order `txMu` → `pendingMigrationMu`, atomic `put/takePendingMigration`, exhaustive invalidation points | `-race` concurrent plan-vs-commit (HANGS if `txMu`-guarded), consume-once replay refusal, bogus-token-evicts-nothing, construction check; `T-06-56` | FIXED |

### Cycle 2 (21 findings — includes the ten the plan names explicitly; see the ★ marks)

**06-01 (5)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 69 | MEDIUM (both reviewers) — ★ the apply-path rollback/atomicity story is inconsistent: 06-01 says `filewriter.Write`, 06-04 says journal-backed rollback, and neither names the other | 06-01 (own); 06-04 (reuse) | One mechanism pinned in 06-01 Layer 4 — `runGlobalSSHApply` opens a `mutationJournal` and `watchFile`s every path, each write still `filewriter.Write`; 06-04 Task 2 reuses verbatim, must not add a second restore path | Two ACs exercise the journal; fresh-file-removal semantics; 06-04 AC forbidding a second restore path | FIXED |
| 68 | MEDIUM (xai-grok) — ★ `rg -n 'RenderGlobalBlock'` false-positives against `.planning/archive/` | 06-01 | Greps scoped to `internal cmd e2e` in both the AC and `<verification>`; 06-04's `persistApplySSH` grep gets the same treatment | Scoped-grep AC baked into the plan text | FIXED |
| 67 | MEDIUM (xai-grok) — ★ the `IgnoreUnknown` placement correction moves the real render relative to the frozen dummy; the visual gate will fail unless classified | 06-01 (recorded) → owned by **06-07** | Allowlist entry `T-06-GLOBALBLOCK` with needles for all three aspects (guard-line placement, two-vs-four-space indent, `Policy`-ordered keys) | 06-07 AC: allowlist must contain `T-06-GLOBALBLOCK` by name; removing any one needle fails the gate | ADDRESSED HERE AND OWNED BY 06-07 |
| 66 | MEDIUM (codex-sol) — lossy managed-block normalisation (duplicate directives, interleaved comments) remains under-tested | 06-01 | RE-AFFIRMED as a DELIBERATE SCOPE BOUNDARY: the block is gitid-owned and sentinel-delimited; unrecognised KEYS are preserved and appended | Round-trip stability + outside-the-sentinel byte-identity ACs pin the safety property that matters; full fidelity inside the sentinels is not a safety property | RE-AFFIRMED AS BOUNDARY (not re-fixed) |
| 65 | LOW (both reviewers) — Wave 1 remains large (~120k tokens, ~35 files) | 06-01 | ACCEPTED, mitigation restated: Task 1 ships as three staged, individually buildable, hook-clean commits so the blast radius is reviewable in thirds | Re-slicing across plan boundaries would break the tracer property this plan exists to provide | ACCEPTED |

**06-03 (5)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 64 | **HIGH** — ★ `stateFor` routes any baseline-sourced value to `StateNeedsAction` BEFORE checking value equality, contradicting "already-set, whatever its source" and mis-flagging a safe OpenSSH default | 06-03 | Reordered algorithm: branches numbered (1) platform gate (2) no value → needs-action (3) **value equals recommendation → already-set regardless of `src`** (4) baseline + unequal → needs-action (5) set + unequal → differs | Four criteria incl. one driving equality through EVERY `SourceClass` constant in turn | FIXED (reordered, not reworded) |
| 63 | MEDIUM — "baseline" conflates a safe default with an unset recommendation | 06-03 | RESOLVED BY THE SAME REORDER: baseline splits by value — baseline AND equal → already-set (safe default); baseline AND unequal → needs-action. One class, two honest outcomes | Same four criteria | RESOLVED by the reorder |
| 62 | Follow-on (this planner, not raised by a reviewer) — an already-set-by-default row must not be worded as something the user configured | 06-03 | already-set takes two wordings (user-attribution form + "safe by default" form) in the same line-2 slot | Render test asserting the baseline form carries neither user-attribution nor a file path; model test asserting the row is not selectable; registered in `gate-copy-freeze` | ADDRESSED |
| 61 | LOW — dual `NotApplicableReason` enums (package + tuikit) can drift | 06-03 | Existing criteria joined by a four-distinct-sentences render test; numeric-value pin added to the `cmd/gitid` parity test that walks both tables | Numeric pin in parity test (the one package where both enums are visible) | ADDRESSED |
| 60 | LOW — the concurrent-latency test may be flaky under `-race` | 06-03 | ACCEPTED AND BOUNDED, not removed: the assertion is a strict inequality against the SUM of budgets, giving roughly a full budget of slack — `-race` would have to more than double elapsed time to flip it | Recorded so a future flake is triaged as a real regression, not assumed the assertion's fault | ACCEPTED AND BOUNDED |

**06-04 (5)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 59 | **HIGH** — ★ the shadowing example reverses OpenSSH's first-obtained-value precedence | 06-04 | BINDING `<resolution_order>` section derived from `ssh_config(5)` "first obtained value" + three verified source facts; four-case table; main-config-BELOW-Include is now a required NEGATIVE control; three genuinely-shadowing cases replace it; PTY cases 5/6 are a matched pair differing by one line | Rewritten unit ACs + PTY cases against the resolution order | FIXED (not by rewording) |
| 58 | **HIGH** — ★ `shadowSourceFor` picks the LAST hit before the block; first-obtained-value makes that the wrong side | 06-04 | Algorithm rewritten as three explicit steps: linearise the graph in resolution order (with `DirectiveSource.LineOffset`), filter to pattern-matching hits outside gitid's own block, return the FIRST survivor | Three ACs: earlier of two competing directives is named; non-matching `Host` pattern not named; hit inside gitid's block not named | FIXED |
| 57 | **HIGH** — ★ nested Includes are neither discovered nor mirrored, so "whole-graph" overstates what is delivered | 06-04 | `globalssh.BuildGraph` recursive Include discovery with visited-path cycle detection + bounded `maxIncludeDepth`; `Simulate` rewrites Includes in EVERY mirrored file; cycle/depth-cap → `Inconclusive` with a reason | Two-hop nesting nameable, cycle → inconclusive, over-depth → inconclusive, every mirrored file's includes stay inside the mirror | FIXED |
| 56 | MEDIUM — ★ apply-path rollback/atomicity inconsistently stated between 06-01 and 06-04 | 06-01 (owned) / 06-04 (reuse) | Same as row 69 — 06-04 states it REUSES 06-01's journal verbatim; no second restore path | 06-04 Task 2 AC: no layering of a second restore path; one deliberate exception (06-05 migration engine) named as a stated decision | FIXED (same mechanism as row 69) |
| 55 | MEDIUM — the `rg` deletion-verification greps false-positive against `.planning/archive/` | 06-04 | `persistApplySSH` grep scoped to `internal cmd e2e` in AC and `<verification>` | Scoped-grep AC | FIXED (same disposition as row 68) |

**06-05 (3)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 54 | **HIGH** — ★ preview and commit each call `PlanMigration` independently against disk read at different times; a change between preview and confirm silently commits different bytes | 06-05 | The plan becomes an OBJECT that travels: `MigrationPlan.Digests`, `MigrateWithPlan` (re-read + digest compare → `ErrConfigChangedSincePreview` BEFORE any backup), `b.pendingMigration` slot + opaque `PlanToken` across the seam; CLI (empty token) plans+commits under one `txMu` hold (honest — no "what the user saw") | Nine criteria incl. plan-hold-mutate-commit sequence, `PlanMigration` CALL-COUNT assertion, stale-token refusal, unrelated-file-edit negative, token-passthrough model test | FIXED |
| 53 | MEDIUM — backups may not correspond to the displayed bytes, for the same reason | 06-05 | Same mechanism + ordering: digest check runs BEFORE any backup, so a backup is only taken of a file still matching the preview | AC asserting NO backup file is created when the refusal fires | FIXED |
| 52 | LOW — the round-trip test needs the fake `ssh` to be Include-aware after a layout change | 06-05 | Fake resolves THROUGH the Include line; executor must assert the fake's own Include-awareness in a dedicated test before the round-trip case relies on it (same discipline as 06-04's T-06-38) | Precondition + dedicated fake-aware proof test | ADDRESSED |

### Cycle 1 (49 findings)

**06-01 (11)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 51 | HIGH — two competing write authorities (`Persist(ApplySSH)` vs `CommitGlobalSSH`) | 06-01 | `runGlobalSSHApply` in `lifecycle.go` is the single write authority; `persistApplySSH` retired and forbidden; `Persist(ApplySSH)` becomes REAL-OWNED re-read-only | AC: `Persist(ApplySSH)` with no preceding commit leaves the file unchanged | FIXED |
| 50 | HIGH — provenance classifier cannot prove "system-set" | 06-01 | Source classes limited to what the probes establish; `/etc/ssh/ssh_config` named only on an explicit parse; otherwise hedged outside-gitid class | AC asserting an unreadable system config yields the hedged class | FIXED |
| 49 | HIGH — `ssh -G -F /dev/null` "compiled defaults" claim unproven | 06-01 | `internal/globalssh/isolation_contract_test.go` — executable hermetic proof of the user-config-isolation half; doc comment cites `ssh(1)` for the system-file half; label scoped to the proven half | The hermetic contract test (skips when no `ssh` binary) | FIXED |
| 48 | HIGH — create path not retargeted (`identity_create.go:227`, `wiring.go:2594`) | 06-01 | `RenderGlobalBlock` DELETED; `CreateInput.GlobalBlock` → `GlobalsGOOS`; `WriteSSH` seam + `sshconfig.Write` signature change; all five call sites updated | AC greps the whole repository for the retired function name (scoped `internal cmd e2e` per row 68) | FIXED |
| 47 | MEDIUM — `EnsureGlobals` map cannot preserve unknown block content | 06-01 | Unrecognised keys preserved and appended after policy-ordered keys; never empties a previously non-empty block; full fidelity inside the sentinels explicitly out of scope | Round-trip test + outside-the-sentinel byte-identity test | ADDRESSED (deliberate scope boundary) |
| 46 | MEDIUM — real probe integration test underspecified | 06-01 | Nil-seam guard test with isolated `HOME` + the isolation-contract proof establishing `-F` behavior directly; contract test skips when no `ssh` | Isolated-HOME AC + explicit skip | ADDRESSED |
| 45 | LOW — tracer boundary weakened by six policy rows | 06-01 | ACCEPTED AS-IS: the table is DATA, not behavior; a half-table would be a second source of truth; only `HashKnownHosts` gets a correct `State` in this plan | Rationale recorded in the plan's action | ACCEPTED AS-IS |
| 44 | non-HIGH — Linux empty-block / `IgnoreUnknown` ordering undocumented | 06-01 | Platform contract pinned in `EnsureGlobals`' doc comment | Four ACs: guard unconditional+first on every platform; darwin-only keys preserved on linux; darwin defaults gated; never empties a non-empty block | FIXED |
| 43 | non-HIGH — the legacy-literal grep AC is unachievable | 06-02 | MOVED to 06-02 and narrowed: production non-test files only (`--glob '!*_test.go'`), declaration excluded | 06-02 scoped-grep AC returns no match | MOVED + narrowed |
| 42 | Cross-plan — reserved-name change strands the global block in migration | 06-02 | MOVED to 06-02, which owns BOTH the registry change and the migration-classification correction | 06-02's co-located tasks | MOVED |
| 41 | Cross-plan — wave sizing | 06-01 | Plan 06-01 restructured (3→2 tasks, 150k→120k tokens); registry consolidation moved to 06-02; plan set 5→7 plans | Three staged, individually buildable commits | ADDRESSED |

**06-02 (6)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 40 | HIGH — `movableBlockNames` excludes the globals block, stranding it during migration (`migrate.go:419`) | 06-02 | Explicit three-way `migrationClasses`; `reorderGlobalLast` recognises both names | `TestMigrationClasses`, both-direction carriage tests under current AND legacy names | FIXED |
| 39 | Cross-plan HIGH — the reserved-name change causes the migration filter to exclude the new block; interaction unaddressed | 06-02 | CO-LOCATION: registry change + classification fix are the two tasks of the same plan/wave | Plan structure (no wave ships a knowingly-wrong filter) | FIXED by co-location |
| 38 | non-HIGH — the legacy-literal grep AC is unachievable against existing fixtures | 06-02 | Criterion scoped to production files with the declaration site excluded; test fixtures KEEP the literal as coverage | Scoped-grep AC: no match in production; literal retained in 8 test files + all 3 retargeted production files | FIXED |
| 37 | Suggestion — add acceptance tests proving the global block moves in both directions | 06-02 | Both-direction carriage tests + legacy-name variant (also at e2e level in 06-05 Task 2/3) | Two ACs + legacy-name variant | IMPLEMENTED verbatim |
| 36 | Suggestion — distinguish movable identity blocks, movable global block, non-movable Include wiring | 06-02 | `migrationClasses` three-way classification | `TestMigrationClasses` | IMPLEMENTED verbatim |
| 35 | LOW — `RenderGlobalBlock` tests go red if the function is deleted; pick delete+repoint | 06-01 | Already resolved in 06-01 (delete + repoint, repository-wide grep criterion) | 06-01 grep AC | ALREADY RESOLVED in 06-01 |

**06-03 (7)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 34 | HIGH — "explicit value" is not "effective value"; the state model misattributes external values to the user | 06-03 | `stateFor` takes `SourceClass`; four VISUAL states preserved but attribution carried separately via `AttributedToUser`; set-but-differs has two wordings | Render test asserting the non-attributed wording omits the user-attribution phrase | FIXED |
| 33 | HIGH — probe-error behavior invalidates independent per-option evidence | 06-03 | Probe outcomes tracked per evidence source with an explicit dependency map; UseKeychain and IdentitiesOnly unaffected by a resolution-probe failure | Two dedicated criteria comparing against the all-succeed run | FIXED |
| 32 | MEDIUM — zero managed identities as already-set is misleading | 06-03 | `total == 0` → not-applicable with `ReasonNothingToVerify` | AC asserts zero is NOT already-set | FIXED |
| 31 | MEDIUM — unknown SSH version treated as available, conflicting with D-13 | 06-03 | `VersionUnverified` is a distinct outcome; row renders+explains but is not selectable; `runGlobalSSHApply` refuses the key; CLI escape hatch naming `ssh -V` specified in 06-06 | `TestGlobalSSHOptionStatesVersionUnverified` / `VersionTooOld`; refusal AC | FIXED |
| 30 | MEDIUM — two different meanings of not-applicable with no reason code | 06-03 | `NotApplicableReason` with four values, four distinct frozen sentences | Four-distinct-sentences render test | FIXED |
| 29 | LOW — three sequential probes may produce visible latency | 06-03 | Concurrent probes under one bounded context | Latency test (total < sum of budgets); measured typical/worst in summary | FIXED |
| 28 | xai-grok — copy must not say "macOS-only" for the version-gated row | 06-03 | Same reason-code work | "Platform sentence never appears on a version-gated row" criterion | FIXED |

**06-04 (9)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 27 | HIGH — `shadowSourceFor` cannot be built from `AllHostStanzas` (no directive/line data) | 06-04 | Naming via `sshconfig.ScanDirectives`/`ScanDirectivesMulti` (key, value, host pattern, source path, 1-based line); returns empty rather than a guess; doc forbids promising an unreported line | Function's doc contract + empty-when-unknown behavior | FIXED |
| 26 | HIGH — simulation only mirrors the candidate file, not the live Include config graph | 06-04 | `Simulate` takes a `SimulationGraph`; full private mirror (entry point, include targets, candidate target); Includes rewritten to stay inside the mirror; probes the mirrored entry point | Criterion plants the shadowing directive in the MAIN config under Include'd layout; PTY case | FIXED |
| 25 | HIGH — the simulation API receives candidate bytes but no resolved layout | 06-04 | `SimulationGraph` type; backend builds it from `b.storage()` + `sshconfig.DetectInclude` | Same mirror criteria | FIXED |
| 24 | HIGH (cross-plan) — Plan 01 and Plan 03 assign the same write path two architectures | 06-04 | 06-04 EXTENDS `runGlobalSSHApply` with simulate + verify stages; no new commit path | AC greps the repository for the retired function name (never reappeared) | FIXED |
| 23 | MEDIUM — static naming remains Include-unaware and may name the wrong line | 06-04 | ACCEPTED AND BOUNDED: scanner doc states it does not model `Match` blocks or first-obtained-value, caller supplies the ordered source list, result is naming only; the PROBE decides; name omitted when unavailable | Doc comment bound; honest bound recorded as such | ACCEPTED AND BOUNDED |
| 22 | MEDIUM — navigation-while-in-flight described ambiguously | 06-04 | Two rules separated in code and success criteria: recommendations never gate another workflow; in-flight commit captures input as transaction-safety | Separate model test per rule | FIXED |
| 21 | LOW — captured UI frames omitted from `files_modified` | 06-04 | `ui-frames/` directory declared in `files_modified` | Frontmatter completeness | FIXED |
| 20 | Suggestion — add PTY cases for probe timeout/inconclusive preview and commit failure/retry | 06-04 | PTY cases 6 and 7 | Each with its own acceptance criterion | IMPLEMENTED |
| 19 | xai-grok — the fake must answer both `ssh -G` and `ssh -G -F` or the shadowing ACs go vacuous | 06-04 | Fake answers three shapes AND derives the isolated answer from the file it is handed | Dedicated proof test before any shadowing case relies on it (`T-06-38`) | IMPLEMENTED and STRENGTHENED |

**06-05 (8)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 18 | HIGH — the concurrent-modification check runs too late; step 2 already replaces both files | 06-05 | Root cause removed via `filewriter.Backup`; check runs at preflight, before the backups and before each write | AC: no backup is created and no file changes when the pre-backup check fires | FIXED |
| 17 | HIGH — rollback on a detected external edit could erase that edit | 06-05 | Per-file written-by-us tracking; `rollback` restores only files gitid wrote | Every abort test asserts the externally modified file still holds the EXTERNAL bytes | FIXED |
| 16 | HIGH — the global block is excluded from migration (`migrate.go:419`) | 06-02 (owned) / 06-05 (asserted) | Fixed in 06-02 (row 40); this plan's tests + PTY cases assert the globals block moves end to end | 06-05 migration tests + PTY case | FIXED (in 06-02) |
| 15 | MEDIUM — the "actual resulting preview" has no pure planning API | 06-05 | `PlanMigration` exported, non-mutating, single composition path for both preview and commit | AC: `Migrate`'s written bytes equal its returned bytes | FIXED |
| 14 | MEDIUM — step 2 creates backups by rewriting unchanged content | 06-05 | `filewriter.Backup` (pure timestamped copy) + `BackupFile` dependency | Backup primitive never replaces its target | FIXED |
| 13 | LOW — scope attribution is muddled | 06-05 | `<scope_note>`: `STORE-01`/`STORE-03`/`STORE-04` are RE-EXERCISED, not re-opened; prerequisite of GSSH-01's deliverable; `requirements` stays `[GSSH-01]` | Note + requirement-coverage checker alignment | ADDRESSED |
| 12 | xai-grok MEDIUM — `CommitSSHStorage` must not also open `mutationJournal` | 06-05 | KEPT and PROMOTED | AC asserting no journal call site exists in the migration ceremony | KEPT, promoted to AC |
| 11 | Suggestion — acceptance tests proving the global block moves in both directions | 06-05 | Unit level in 06-02; end-to-end here (Task 2 criterion and Task 3 PTY case) | E2E + PTY criteria | IMPLEMENTED |

**06-06 (8)**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 10 | HIGH — exact CLI syntax ambiguous despite being declared contractual | 06-06 | `<frozen_contract>` names all four command paths and every flag, before implementation | AC: built tree contains exactly those paths and no others | FIXED |
| 9 | HIGH — advisory shadowing exit semantics conflict with the phase posture | 06-06 | Frozen exit table: zero-on-advisory default (D-04/D-14 reasoning in helper doc + matrix notes); `--fail-on-advisory` opt-in for stricter scripts | Both directions asserted | FIXED |
| 8 | MEDIUM — "same function" parity is weaker than "same transactional core"; Cobra should not invoke a `tea.Cmd` | 06-06 | Both write verbs call the UI-free `runGlobalSSHApply`/`runSSHStorageMigrate` directly with a `lifecyclePolicy`; no `tea.Cmd` crosses into Cobra | `TestSSHWriteVerbsCallSharedCeremonyByConstruction` | FIXED |
| 7 | MEDIUM — machine-readable schema not frozen | 06-06 | `docs/gitid-ssh-json-schema.md` versioned + checked in | AC: emitted key set EXACTLY equals documented set; every enum value a documented member | FIXED |
| 6 | MEDIUM — the final wave is too broad | 06-06 | SPLIT: predecessor's three tasks became 06-06 (CLI + matrix) and 06-07 (visual gate + battery + packet), each independently verified | Independent `<verify>` per plan | FIXED by splitting |
| 5 | MEDIUM — scope materially exceeds GSSH-01; traceability should reflect it | 06-06 | Matrix rows keyed on `GSSH-01` + `SHELL-03`; storage rows note the `STORE-01`/`STORE-03` re-exercise; 06-05 carries the full scope note; `requirements` stays `[GSSH-01]` | Requirement-coverage checker alignment | ADDRESSED |
| 4 | LOW — acceptance criteria use legacy `grep` | 06-06 | Every grep-shaped criterion now uses `rg` across the revised plan set | Plan text uses `rg` | FIXED |
| 3 | xai-grok — assert the CLI holds the real backend and calls the commit by construction, not by duplicating disk assertions | 06-06 | By-construction AC; disk assertions live once in the e2e case | By-construction criterion + e2e | IMPLEMENTED |

**06-06, cycle-2 rows (the last three of the plan's ten-named cycle-2 set start here; see also 06-01 rows 69/68/67, 06-03 row 64, 06-04 rows 59/58/57/56, 06-05 row 54):**

| # | Finding | Plan | Closing artifact | Pinned by (acceptance criterion) | Disposition |
|---|---|---|---|---|---|
| 2 | MEDIUM — ★ the write-verb JSON promise: advisory JSON promised in the threat model (T-06-43) but the frozen tree grants `--json` only to read verbs; write-result schema undefined | 06-06 | `--json` added to BOTH write verbs; two new frozen, versioned envelopes — `gitid.ssh.apply/v1` (applied, declined, target_path, backups, restored, advisories, simulation_inconclusive, simulation_note, error, exit_code) and `gitid.ssh.migrate/v1`; `exit_code` from the same helper as process status | Exact-key-set ACs per envelope; `TestSSHExitCodeEqualsEnvelopeForEveryRow` (`T-06-53`) | FIXED (frozen the envelope, not dropped the promise) |
| 1 | MEDIUM — ★ the interactive fallback for `options apply` with incomplete positional args is described but not frozen: which forms open the TUI, what is pre-filled | 06-06 | `<frozen_contract>` "Adaptive-depth contract" table per verb; both verbs route through the EXISTING `depthResolver`/`confirmationPolicyFrom` (`cmd/gitid/identity.go:135-213`); nothing pre-selected; a future partial-spec flag is flagged as a new decision | Three ACs incl. a table test over all four TTY combinations | FIXED |
| 0 | LOW — exit code 2's wording does not distinguish pre-write from post-write failure | 06-06 | Table unchanged (code 2 = "write failed AND rolled back"; pre-write refusal is code 1); the `restored` and `error` fields of the write-result envelope carry WHICH failure | Envelope key-set tests; recorded so the table is not "clarified" into a fourth code later | ADDRESSED without changing the table |

#### The ten cycle-2 findings the plan names explicitly — index of ★ rows

The plan's `<acceptance_criteria>` requires these ten to appear by name in the
cycle-2 rows; the ★ supra marks them: (1) the 06-04 shadowing-precedence
reversal (row 59), (2) the 06-04 `shadowSourceFor` culprit-selection direction
(row 58), (3) the 06-04 nested-Include mirroring gap (row 57), (4) the 06-03
`stateFor` baseline-ordering self-contradiction (row 64), (5) the 06-05
preview/commit plan-identity gap (row 54), (6) the apply-path atomicity
inconsistency (rows 69 and 56), (7) the archive-scoped `rg` greps (rows 68 and
55), (8) the `IgnoreUnknown` visual classification (row 67), (9) the write-verb
JSON promise (row 2), (10) the `options apply` interactive-fallback contract
(row 1).

#### Source and count — documented cross-check (row count == finding count in the source document)

The closure table above was NOT written against a fixed count. The count was
derived at execution time by parsing the source documents:

- **Source documents:** every `<review_disposition>` section of
  `06-01-PLAN.md` … `06-07-PLAN.md` (the per-plan sections ARE the review
  record for cycles 1-4 and the cycle-5 pre-convergence items; the plan's own
  Task 2 `<action>` says "The per-plan `<review_disposition>` sections are the
  source; this is their index"), plus the current `06-REVIEWS.md`, which at
  execution time holds only Cycle 5 and records **zero actionable findings**
  (closed row 82 above).
- **Parse method:** a purpose-built script extracted each section between its
  `<review_disposition>` / `</review_disposition>` tags (using the literal tag
  line numbers from `rg -n`, so the prose mention of the tag name cannot match),
  split contiguous `|`-lines into markdown tables, discarded header and
  separator lines, and counted the data rows. It also printed each row's first
  cell; the closure table's finding text mirrors those cells.
- **Result:** 06-01 = 16, 06-02 = 6, 06-03 = 13, 06-04 = 17, 06-05 = 14,
  06-06 = 11, 06-07 = 5. **Total = 82 finding rows.** The closure table above
  has exactly 82 FINDING rows, numbered 0-81 (row 0 is the `options apply`
  interactive-fallback contract — the last of the ten named cycle-2 findings,
  sequenced so the plan-text greppable numbers land on the named rows); the
  Cycle-5 zero-actionable-findings record is a separate, unnumbered "C5" row
  and is NOT counted in the 82, because it records the absence of findings
  rather than a finding. Cycle distribution:
  49 (cycle 1) + 21 (cycle 2) + 4 (cycle 3) + 3 (cycle 4) + 5 (cycle 5 items
  resolved in-plan, per 06-07's disposition) + 1 (cycle-5 convergence record,
  uncounted).
- **Confirmation:** the table row count equals the parsed source count by
  construction and was checked against the parse printout row for row. The
  parser is trivially re-runnable; its commands are recorded in
  `06-07-SUMMARY.md`. A reviewer can re-derive the count in one pass by
  counting the data rows of each disposition table in the plan files and
  summing them, as the machine did here.

Note on the "fourteen actionable findings" the 06-07 disposition mentions: that
count refers to the fixed/actionable subset; this table deliberately indexes
ALL 82 recorded rows (fixes, scope boundaries, acceptances, boundings, moves,
suggestions, and self-raised follow-ons) so a partial closure cannot hide — the
same failure mode the plan names ("A hard-coded 'eight HIGH and six non-HIGH'
would silently omit a later cycle's findings").

## Reviewer checklist

1. Run `make gate-visual-regression` — expect 34 RequiredScreenSpecs frames
   (27 pre-existing + 7 Global SSH) compared; all four Global SSH negative
   controls pass; `TestApprovalCommitRecorded` may SKIP (worktree `.git` is a
   file, not a directory — an environment artifact, not a gate failure).
2. Compare the real binary against the approved dummy using `frames/` +
   `visual-divergence-allowlist.txt` (the gate's machine-readable source of
   truth). Every entry states improvement/defect; the three REQUIRED entries
   are present by name.
3. Check the closure table (Part D) against the per-plan
   `<review_disposition>` sections and `06-REVIEWS.md` — 82 rows derived, count
   cross-checked above.
4. Record your findings and disposition in the phase's review section, as
   Phase 3's wave-close pass did.