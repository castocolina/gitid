---
phase: 05-identity-manager
plan: 07
subsystem: identity-manager
tags: [identity-manager, lifecycle-ceremony, delete, persist-exhaustiveness, honest-detail, severity, confirmation-mode]

# Dependency graph
requires:
  - phase: 05-identity-manager
    provides: "05-06's render layer — the action menu, DeletePlanView/KeyCeremonyPlan/KeyActionFor seams, RotateIdentity action, and IdentityPlanner — the UI surface this plan closes over the real backend"
  - phase: 05-identity-manager
    provides: "05-03's depsForTransaction(journal) + archiveKeyPairSeam(onCreated) + recordCreatedFile — the transaction-scoped Deps binding every lifecycle function consumes (never b.deps), and 05-04's deleteDepsForTransaction"
  - phase: 05-identity-manager
    provides: "05-01's runDelete (the ONE delete writer this plan extends) plus CommitDelete, the initial Persist switch, and 05-05's wizard commit paths"
provides:
  - "cmd/gitid/lifecycle.go's runRotate / runRepair / runDelete — ONE complete-lifecycle chokepoint per verb, recording per-verb stage sequences against the shared lifecycleStages table and owned by BOTH the TUI commit seams and 05-08's CLI"
  - "confirmationMode (confirmationRequired / confirmationAlreadyObtained / confirmationBypassedWithYes) with a fail-closed zero value: a confirmationRequired call with no prompt returns errConfirmationUnavailable before any backup or write"
  - "mutationJournal.recordCreatedDir and mode restoration — rollback removes newly created archive directories and restores 0600 as well as bytes; the journal's two sets are disjoint by construction"
  - "real DeletePlan / KeyCeremonyPlan / KeyActionFor seams and CommitRotate / CommitNewKey thin adapters over the lifecycle functions"
  - "an exhaustive three-way Persist switch (real-owned / demo-only / invalid) that closes the dummy-reducer fallthrough, guarded by tuikit.AllActions() plus a go/parser completeness test"
  - "internal/identity/state.go's Severity + SeverityFor — the domain-owned problem→severity table, consumed again by Phase 8's doctor"
  - "an MGR-03-honest detail renderer: absent hostname/port/key/signing values render the absence marker, the fragment's real signing key is carried (never derived), and User git / IdentitiesOnly yes sit under a 'gitid always writes' policy label; findings come from the real per-identity classification (MGR-07)"
affects: [05-08-cli-lifecycle, 08-doctor, uat-audit]

# Actuals (#2632) — pairs with the plan's `estimate` (124000/62000) to calibrate future estimates.
actuals:
  tokens: 34146
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "One complete-lifecycle chokepoint per verb: runRotate/runRepair/runDelete own the whole D-02 ceremony and the TUI seam and the CLI both call them; a seam contributes no stage of its own (R-11-CLI)"
    - "Stage sequences as data: lifecycleStages declares per-verb rows; every row must contain confirm → backup → write in that relative order; delete legitimately has no test stage"
    - "Three-valued confirmationMode enum instead of a nilable func, with the dangerous zero value failing closed (R2-03)"
    - "Transaction-scoped Deps: lifecycle functions call the domain through depsForTransaction/deleteDepsForTransaction, never b.deps whose archive seam refuses (R3-01)"
    - "Disjoint journal sets: watched paths restored, created paths removed, no path in both — enforced at registration, not resolved at restore (R2-08)"
    - "Explicit action registry: tuikit.AllActions() plus a go/parser completeness gate over isAction() receivers, because Go reflection cannot enumerate interface implementers (R-08)"
    - "Domain-owned severity policy: identity.SeverityFor; cmd/gitid only translates vocabularies at one conversion site (R-19)"

key-files:
  created:
    - cmd/gitid/lifecycle.go
    - cmd/gitid/lifecycle_test.go
  modified:
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go
    - cmd/gitid/identity_delete.go
    - internal/identity/state.go
    - internal/identity/state_test.go
    - internal/identity/identity.go
    - internal/identity/loader.go
    - internal/identity/clone_test.go
    - internal/tuikit/identities.go
    - internal/tuikit/identities_test.go
    - internal/tuikit/store.go
    - internal/tuikit/store_test.go
    - internal/tuikit/app_test.go
    - internal/tuikit/backend_stub_test.go
    - internal/dummytui/fixturebackend.go

key-decisions:
  - "Rollback-versus-archive resolution: archive copies are created THROUGH a journal (depsForTransaction/deleteDepsForTransaction), so the observer records them as created paths and restore() removes them; the domain result's archive paths are a second, independent recording source because recordCreatedFile is idempotent (R-10, R3-01)."
  - "delete runs plan → confirm → backup → write → verify and never a test stage — a deleted identity cannot be SSH-tested; verify re-reads the inventory and fails if the identity is still reconstructable (R2-02)."
  - "confirmationRequired (ZERO value) needs a prompt and fails closed with errConfirmationUnavailable; confirmationAlreadyObtained may be asserted ONLY by a layer that showed a confirm screen; confirmationBypassedWithYes is the CLI's --yes."
  - "Problem-to-severity mapping lives in the domain as identity.SeverityFor (info/warning/error); cmd/gitid translates to tuikit severities at the one conversion site — Phase 8's doctor is the second consumer (R-19)."
  - "R-35 taken as option (b) — the cheaper resolution: User git / IdentitiesOnly yes stay canonical constants rendered under the explicit 'gitid always writes' label, separate from parsed-value lines (the plan's preferred option (a) would need sshconfig.ParseManagedHosts to already surface them)."
  - "Findings derive from one BuildInventory call shared with the list rows (healthByName()), never re-derived (MGR-07)."
  - "The all-or-nothing real commits and the exhaustive Persist switch replace the dummy reducer as the real backend's mutation path."

patterns-established:
  - "Per-verb lifecycle table as single source of truth: adding a verb means adding a row to lifecycleStages first and a property test keeps backup in every row."
  - "Deps ordinary value type re-bound per transaction: identity.Deps being a value type lets depsForTransaction rebind ONE archive seam without a second wiring."
  - "Loud unmapped-policy failures: SeverityFor returns '' for an unmapped Problem so the go/parser completeness test fails loudly; the exhaustive Persist switch records errUnhandledAction for any unclassified action."

requirements-completed: [MGR-01, MGR-03, MGR-06, MGR-07, MGR-08, KEY-05, KEY-07, SHELL-01, SHELL-02]

# Metrics
duration: 2h00m
completed: 2026-08-26
status: complete
---

# Plan 05-07 Summary

**Wired every Phase 5 manager ceremony to the real filesystem through one complete-lifecycle chokepoint per verb, closed the dummy-reducer fallthrough with an exhaustive Persist switch, and made the detail view honest — no fabricated hostname, port, or signing key, with real per-identity findings carrying domain-owned severities.**

## Performance

- **Duration:** 2h 00m
- **Started:** 2026-08-25T22:56:02-04:00
- **Completed:** 2026-08-26T00:52:59-04:00
- **Tasks:** 3
- **Files modified:** 18 (2 created, 16 modified)

## Accomplishments

- Real rotate/repair/delete transactions, each through ONE lifecycle function that runs its verb's declared stage sequence, backs up before writing, and rolls back atomically — restoring bytes AND modes AND removing every archive entry the transaction created, including archive copies that only came into existence mid-transaction.
- Confirmation surgery: the nilable func is gone, replaced by a three-valued `confirmationMode` whose zero value fails closed with `errConfirmationUnavailable` before any backup or write — a scripted destructive run with no TTY and no `--yes` can never be misread as authorized.
- Delete finished end to end: the real delete plan, everything-scope removal with reference-counted provider-rewrite cleanup, shared-key sibling preservation, and the single `runDelete` writer serving both the TUI seam and every scope.
- The dummy reducer is unreachable from the real backend: every `Action` is classified real-owned, demo-only, or invalid in an exhaustive switch, and `tuikit.AllActions()` + a `go/parser` completeness test (with a negative control) prove no future action can slip through silently.
- Detail view honesty (MGR-03): `ssh.github.com`/`443`/key-path+` .pub` fabrications removed — absent values render the `— missing` marker, the fragment's real signing key is carried through `Account.SigningKeyPath`/`DemoIdentity.SigningKeyPath`, and the policy constants are labelled.
- Real per-identity health (MGR-07): findings are populated from the same `BuildInventory` classification the list rows use, each problem attributed to its identity name with a severity from the new domain-owned `identity.SeverityFor`.
- SHELL-02 pinned: number keys 1–4 reach the four primary views and the palette offers all five entries, with the cover banner predicate asserted per view.

## Task Commits

Each task was committed atomically:

1. **Task 1: The one-lifecycle-per-verb chokepoint, then real rotate and new-key transactions with all-or-nothing rollback (KEY-05, KEY-07)** - `0ec7a43` (feat)
2. **Task 2: Real delete plan, everything-scope delete, and an exhaustive Persist switch (MGR-06)** - `304ac09` (feat)
3. **Task 3: An honest detail view, real per-identity health, and the five-view surface (MGR-01, MGR-03, MGR-07, MGR-08, SHELL-02)** - `699686f` (feat)

## Files Created/Modified

- `cmd/gitid/lifecycle.go` - created - `runRotate`/`runRepair`/`runDelete`, `lifecycleStages`, `confirmationMode`, `errConfirmationUnavailable`.
- `cmd/gitid/lifecycle_test.go` - created - stage recorder, fail-closed authorization, per-step rollback, archive-second-source-removal regression.
- `cmd/gitid/wiring.go` - CommitRotate/CommitNewKey thin adapters, real DeletePlan/KeyCeremonyPlan/KeyActionFor, exhaustive three-way Persist switch, findings from `BuildInventory`, `inventoryDeps` with tilde expansion.
- `cmd/gitid/wiring_test.go` - everything-scope/sibling/rollback delete tests, `AllActions` exhaustiveness, `ConfigureGit` real-owned, demo-only reduce-equality.
- `cmd/gitid/identity_delete.go` - `CommitDelete` everything-scope delegation to `runDelete`.
- `internal/identity/state.go` - `Severity` + `SeverityFor` (domain-owned severity policy).
- `internal/identity/state_test.go` - `go/parser` completeness test over Problem constants; `SeverityFor` table gate.
- `internal/identity/identity.go` - `SigningKeyPath` field on `Account`.
- `internal/identity/loader.go` - `Reconstruct` populates `SigningKeyPath` from the parsed fragment.
- `internal/identity/clone_test.go` - comment updated: clone still derives, does not copy `SigningKeyPath`.
- `internal/tuikit/identities.go` - honest `renderDetail` (`observedOrMissing`/`observedPort`), policy label, real signing line, findings section.
- `internal/tuikit/identities_test.go` - Task 3 render tests: honest hostname/port/signing/SSH-only/policy-label/findings.
- `internal/tuikit/store.go` - `DemoIdentity.SigningKeyPath` field.
- `internal/tuikit/store_test.go` - `AllActions()` completeness + negative control.
- `internal/tuikit/app_test.go` - number-key/palette/banner SHELL-02 pins.
- `internal/tuikit/backend_stub_test.go`, `internal/dummytui/fixturebackend.go` - demo fixtures render explicit hostname/port/signing values.
- `Makefile` - copy-freeze gate registers the new `gitid always writes` label.

## Decisions Made

- **Per-verb stage sequences as data, not one universal sequence.** `lifecycleStages` (single source of truth for D-02's no-behavioral-fork claim):

  ```go
  var lifecycleStages = map[string][]string{
      "rotate": {"test", "plan", "confirm", "backup", "write", "retest"},
      "repair": {"test", "plan", "confirm", "backup", "write", "retest"},
      "delete": {"plan", "confirm", "backup", "write", "verify"},
  }
  ```

  Delete has no test stage and invokes the connectivity tester ZERO times, because a deleted identity cannot be SSH-tested; its closing `verify` re-reads the reconstructed inventory. A property test keeps `confirm` → `backup` → `write` in every row so a future verb cannot omit the backup stage. Plan 05-08's matrix and dry-run contract must be written against this table.

- **Three-valued confirmation authorization.** `confirmationRequired` is the zero value and fails closed with `errConfirmationUnavailable` when no prompt is installed — before the backup stage, before any write. `confirmationAlreadyObtained` may be set ONLY by the TUI's commit seams (a human saw the confirm screen); `confirmationBypassedWithYes` is the CLI's distinct value for `--yes`. No policy field can suppress the backup stage once authorized.

- **Rollback-versus-archive resolution (R-10, R3-01).** The journal's two sets are disjoint: every pre-existing watched path is restored (bytes AND mode), every recorded created path is removed, and a path may not be registered in both. The lifecycle functions call the domain through `depsForTransaction(j)`/`deleteDepsForTransaction(j)` — never `b.deps`, whose archive binding refuses — and record the result-named archive paths as a second source. A rotation that fails inside the archive step's own second-source removal leaves no archive entry and restores both canonical keys at 0600; the regression passes and is the observed result of that rollback regression.

- **Problem-to-severity mapping owned by the domain (R-19).** `identity.SeverityFor` (`info` / `warning` / `error`) is the complete table over every `Problem` constant, backed by a `go/parser` completeness test that fails on an unmapped constant. `cmd/gitid` translates to `tuikit` severities at the one conversion site; Phase 8's doctor is the documented second consumer.

- **R-35 taken as option (b)** — the cheaper resolution the plan allowed: `User git` and `IdentitiesOnly yes` stay canonical constants, but render under an explicit `gitid always writes` label on their own line, visually separated from parsed values. A render test asserts the policy constants never share a line with a parsed hostname/port.

- **Signing key carried, never derived.** `Account.SigningKeyPath` (populated in `loader.Reconstruct`) flows through `DemoIdentity.SigningKeyPath` to `renderDetail`; the derived `keypath + ".pub"` guess is gone, so an identity whose fragment names a different signer renders the fragment's value.

- **One classification, one list.** `healthByName()` does a single `BuildInventory`; both the per-row state word and the findings slice consume it (with a tilde-expanding `inventoryDeps` so recipe-shaped `IdentityFile` values classify against real files). Findings are attributed by identity name for the existing `FindingsFor` helper.

- **Exhaustive Persist classification instead of fallthrough.** Real-owned: AddIdentity, ConfigureGit, DeleteIdentity, NewKey, RotateIdentity, EditSSH, Reset. Demo-only (blocks behind the D-16 banner keep their approved in-memory behavior via `tuikit.Reduce`): MarkScanned, FixFinding (Phase 8), ApplySSH, SetSSHStorage (Phase 6), ApplyGitBaseline, ApplyGitGlobalEmail (Phase 7). Invalid for the real backend: CloneIdentity (D-15 routes it through the wizard). Anything else records `errUnhandledAction` naming its dynamic type.

## Deviations from Plan

### Auto-fixed Issues

**1. Test self-contradiction in `TestDetailAbsentHostnameUsesAbsenceMarker` (test-only fix; no behavior change)**
- **Found during:** Task 3 (honest detail render tests) verification
- **Issue:** The assertion `strings.Contains(got, "github.com")` could never pass for the seeded identity `SSHHost: "sparse.github.com"` — by the repo's naming convention the SSH alias IS `<identity-name>.<provider-domain>`, so the legitimately-rendered `Host alias: sparse.github.com` line always contains `github.com`. The blanket whole-string check conflated "the Hostname line must not fabricate a provider hostname" with "the word github.com must not appear anywhere".
- **Fix:** Narrowed the assertion to the `Hostname:` line only, mirroring the adjacent `TestDetailAbsentPortUsesAbsenceMarker` (iterate `strings.Split(got, "\n")`, check only lines containing `Hostname:`), asserting the line shows the absence marker and no provider hostname literal. The implementation already renders `Hostname: — missing`; no production code changed. Verified against the plan's Task 3 acceptance criterion ("the rendered detail does not contain any provider hostname literal") — honest-rendering applies to the Hostname/Port/IdentityFile positions, not the alias line.
- **Files modified:** `internal/tuikit/identities_test.go`
- **Verification:** `go test ./internal/identity/... ./internal/tuikit/...` (594 pass), the Task 3 `-run 'Detail|Findings|Taxonomy|Banner|Tabs'` race suite, `make lint`, `TERM=dumb SSH_AUTH_SOCK= go test -race ./...` (1379 pass), `make test`, `make test-e2e`, `make gate-copy-freeze`.
- **Committed in:** `699686f` (Task 3 commit)

**2. goimports formatting on two Task 3 files**
- **Found during:** `make lint`
- **Issue:** `internal/identity/identity.go` (struct alignment with the new `SigningKeyPath` field) and `internal/identity/state_test.go` (import block ordering) failed the golangci-lint goimports check; golangci-lint's `--fix` cannot run against the installed Go 1.27 toolchain (stdlib typecheck failure).
- **Fix:** Ran the `goimports` binary (`/Users/ramon/go/bin/goimports -w`) directly on both files.
- **Files modified:** `internal/identity/identity.go`, `internal/identity/state_test.go`
- **Verification:** `make lint` → 0 issues.
- **Committed in:** `699686f` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 test-defect fix, 1 formatting-only)
**Impact on plan:** All auto-fixes necessary for correctness/quality; no scope creep, no production behavior change.

## Issues Encountered

- `TestDetailAbsentHostnameUsesAbsenceMarker` failed because its assertion contradicted the alias naming convention (see deviation 1) — resolved in the test, not the renderer.
- golangci-lint's `--fix` fails typecheck against Go 1.27 stdlib (`math/rand/v2` type-parameter error) in this toolchain; worked around with the standalone `goimports` binary.
- `make test-e2e` needs ~4.5 minutes (its own `-timeout 360s`) and was first cut short by a 5-minute shell timeout; re-run completed in 273.75s.

## Next Phase Readiness

- **05-08 (CLI lifecycle):** plan 05-08's handler can call `runRotate`/`runRepair`/`runDelete` directly and map its flags onto `confirmationMode` — the parity claim is structural, not incidental. Its stage matrix and delete dry-run contract must agree with the final `lifecycleStages` table recorded above.
- **Phase 8 (Doctor):** `identity.SeverityFor` is the shared attention mapper; no second severity table exists anywhere in `cmd/gitid`.
- **Shared reducer is sealed:** any new `Action` added in a later phase must be registered in `tuikit.AllActions()` and classified in `Persist`, or the exhaustiveness gate fails loudly.
- All five views reachable, four still behind the D-16 banner; the Identities view is fully wired and banner-free.

---
*Phase: 05-identity-manager*
*Completed: 2026-08-26*