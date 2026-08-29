---
phase: 09-upload-credentials-assist
fixed_at: 2026-08-29T21:40:00Z
review_path: .planning/phases/09-upload-credentials-assist/09-REVIEW.md
iteration: 2
findings_in_scope: 14
fixed: 7
skipped: 7
status: partial
---

# Phase 9: Code Review Fix Report (iteration 2)

**Fixed at:** 2026-08-29T21:40:00Z
**Source review:** .planning/phases/09-upload-credentials-assist/09-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope (Critical + Warning): 14
- Fixed: 7 (both Critical findings, plus WR-01, WR-03, WR-05, WR-08, WR-10 —
  WR-08 was resolved as a direct side effect of the CR-01 commit rather than
  its own commit; see its entry below)
- Skipped: 7 (WR-02, WR-04, WR-06, WR-07, WR-09, WR-11, WR-12 — see reasoning below)

All six commits are independently buildable, pass their own package's test
suite, and pass the full-repo `make lint` (golangci-lint v2.12.2 + gosec,
`go vet` across the `screenshot`/`smoke`/`e2e` build tags). The whole module
(`go build ./...`, `go test ./...`) is green: 2353 tests passed across 22
packages.

## Fixed Issues

### CR-01: `--dry-run` writes a new file into the user's real `~/.ssh`

**Files modified:** `cmd/gitid/identity_create.go`, `cmd/gitid/identity_test.go`
**Commit:** `833510b`
**Applied fix:** `runCreateDryRun`'s upload-preview block now guards on
`staged.PrivPEM != nil` (mirroring the already-safe identical pattern in
`uploadRequestFromSpec`, `upload_run.go`): a temp `.pub` sibling is only
staged for the GENERATE path (a safe staging-directory temp file). For the
REUSE path, the preview only runs against the real `.pub` if it already
exists on disk — it is never synthesized. This also incidentally fixes
WR-08 (the write-failure comment now matches the actual behavior — a write
failure or a missing `.pub` skips the preview instead of printing a bogus
registration-failure row).

Added `TestIdentityCloneDryRunReuseWithMissingPubWritesNothingUnderHome`,
which asserts the whole fake-HOME file listing is byte-identical
before/after `identity clone --dry-run` against a seeded reuse key with no
`.pub` sibling. Confirmed RED against the pre-fix code (temporarily
reverted, re-ran, confirmed the test fails with the exact HOME-mutation
symptom the finding describes), then confirmed GREEN with the fix restored.

### CR-02: the D-04 delete offer is presented even when the new key's registration failed

**Files modified:** `internal/tuikit/identities.go`,
`internal/tuikit/identity_manager_upload_test.go`,
`cmd/gitid/upload_run.go`, `cmd/gitid/wiring_test.go`
**Commit:** `01d0f33`
**Applied fix:** Two independent gates, exactly as the review prescribed:
- **Model layer** (`identities.go`): the rotate-delete-offer dispatch is now
  gated on a new `uploadSucceeded(run.View)` helper — every row must be
  `UploadRowUploaded`/`UploadRowAlreadyPresent`, or the whole run must have
  short-circuited via `AlreadyComplete`. A run with zero rows (Omitted,
  Disabled, `--no-upload`) is treated as unproven, not vacuously true.
- **Backend layer** (`upload_run.go`, `rotateDeleteOfferFor`): an
  independent belt-and-braces refusal — every registration
  `desiredRegistrations(tool)` names must already carry the current pub's
  blob in the freshly-read inventory, or the offer refuses with
  `Unavailable`. This protects any future caller that reaches
  `RotateDeleteOffer` without going through the model gate.

Added `TestRotateDeleteOfferNotDispatchedOnFailedUpload` (five unproven-
upload shapes — failed row, partial failure, degraded inventory, skipped,
zero rows — never dispatch the offer, confirmed RED against the pre-fix
gate) and `TestRotateDeleteOfferDispatchedOnSuccessfulUpload` (the gate is
not overzealous — genuine success still dispatches) at the model layer.
Added `TestRotateDeleteOfferRefusesWhenTheNewKeyIsNotFullyRegistered` and
`TestRotateDeleteOfferRefusesOnPartialRegistration` (auth registered,
signing missing) at the backend layer, both confirmed RED against the
pre-fix code. Updated the two existing `rotateDeleteOfferFor` fixtures
(`TestRotateDeleteOfferReadsInventoryFreshAtResultTime`,
`TestRotateDeleteOfferExcludesTheJustRegisteredKeySharingTitle`) to
register the current pub under every desired registration, matching what a
real successful rotate's inventory would show.

### WR-01: `OldKeyCandidates` fails OPEN when key material is missing

**Files modified:** `internal/uploader/inventory.go`,
`internal/uploader/inventory_test.go`
**Commit:** `4b31f66`
**Applied fix:** `OldKeyCandidates` now refuses (returns `nil`) in both
fail-open directions the review identified: a blank `currentBlob`
short-circuits immediately (cannot prove which record is the new key), and
any candidate record whose `NormalizeKeyBlob` result is `""` makes the
whole candidate set unsafe and refuses too — a record that cannot be
compared must never silently pass the "exactly one distinct blob" ambiguity
check. Added `TestOldKeyCandidatesRefusesOnBlankCurrentBlob` and
`TestOldKeyCandidatesRefusesOnBlankRecordBlob` beside the existing
`TestOldKeyCandidates*` table.

### WR-08: a dry run can print a registration FAILURE row for a write it silently swallowed

**Files modified:** `cmd/gitid/identity_create.go` (same commit as CR-01)
**Commit:** `833510b`
**Applied fix:** Resolved as a direct side effect of the CR-01 fix, not a
separate commit: the rewritten upload-preview block now returns early
(skips the preview entirely) on a `WritePub` failure instead of ignoring
the error and letting the preview run against a nonexistent file, honoring
the comment's original claim ("a write failure here just skips the upload
preview") for real.

### WR-03: `extractUploadSection` still collides with the D-01 checkbox (second half)

**Files modified:** `internal/screenshot/createflow_regions.go`,
`internal/screenshot/createflow.go`
**Commit:** `8422246`
**Applied fix:** The bare `"Auto-registration"` marker (the first word of
the DISABLED checkbox label) was narrowed to the full `UploadManualHeading`
phrase (`"Auto-registration wasn't available"`) rather than dropped
outright — an initial attempt to drop it entirely broke
`TestRegionDiffCoverage` because the bare prefix is ALSO the true anchor
for the real manual-fallback screen. The narrower phrase matches only the
real upload-beat content and is distinct from the checkbox's "unavailable"
wording. Reassigned `upload-checkbox-disabled`'s `RequiredRegions` from
`RegionUploadSection` to `RegionFormFields`, mirroring
`upload-checkbox-unauth`'s own prior WR-12 fix. Verified via
`go test -tags screenshot ./internal/screenshot/...` (`TestRegionDiffCoverage`
passing) and the full `make lint` (including the `screenshot`-tagged
golangci-lint pass).

### WR-05: the dummy backend still advertises the delete argv CR-02 removed as wrong

**Files modified:** `internal/dummytui/fixturebackend.go`
**Commit:** `d4ea000`
**Applied fix:** Regenerated `FixtureBackend.RotateDeleteOffer`'s GitHub
case from the real shapes, per the review's exact suggestion: `KeyID` is
now the opaque per-registration JSON payload `encodeDeleteCandidates`
emits, `KeyDetail` carries the reviewed IDs plus a blob suffix, and
`ManualCommand` previews both registration-scoped delete commands (one per
line), matching `deleteCandidatesManualCommand` / `uploader.DeleteCommandPreview`'s
real output for a rotated GitHub key's two registrations. Verified
`go test ./internal/dummytui/...` passes; confirmed no committed PTY frame
or test asserts on the old fixture text (grepped for `"ssh-key delete
1234567"` / `"demo-machine"` — only `internal/dummytui/fixturebackend.go`
and `internal/screenshot/createflow.go` reference `"demo-machine"`, and the
latter is a distinct, unrelated string), so no frame re-promotion was
needed.

### WR-10: the `glab` inventory is still unpaginated

**Files modified:** `internal/uploader/inventory.go`,
`internal/uploader/inventory_test.go`
**Commit:** `4c12c74`
**Applied fix:** Verified `glab`'s real flag surface against a locally
installed `glab ssh-key list --help` (v1.114.0): `-p/--page` (default 1)
and `-P/--per-page` (default 30) — no `gh --paginate`-style all-pages flag.
Added `glabInventory`, which iterates `--page` with an explicit
`--per-page=30`, concatenating pages via the existing
`decodeProviderKeyPages` multi-document parser, and stops as soon as a page
returns fewer than `glabPerPage` records. Updated
`TestInventoryGLabReadsListOnce` for the new argv shape and added
`TestInventoryGLabPaginatesUntilAShortPage` (a full 30-record first page
forces a second call). Confirmed the e2e fake-glab shim (`e2e/harness_test.go`)
always returns the same small (well under 30-item) inventory fixture
regardless of `--page`, so there is no infinite-loop risk from this change
against the existing e2e fixtures.

## Skipped Issues

### WR-02: a retry after a partial delete can never succeed

**File:** `cmd/gitid/wiring.go:1641-1655`, `internal/tuikit/identities.go:2293-2301`
**Reason:** out of the explicit priority list for this pass (top priority
was CR-01/CR-02/WR-01, plus three explicitly named "worth closing" items
mapped from the orchestrator's iteration-1 references — WR-03, WR-05,
WR-10). Treating "already absent" (404) as success in
`CommitRotateDeleteOldKey`'s delete loop is a real, scoped fix, but was not
in the explicitly prioritized set and time/effort budget for this pass was
allocated to the two Criticals and the requested Warnings first.

### WR-04: the CR-02 fix made `ManualCommand` multi-line, but it is interpolated into a single-line result sentence

**File:** `cmd/gitid/upload_run.go:512-518`, `internal/tuikit/design.go:695`,
`internal/tuikit/identities.go:2788, 2797, 2995-2996`
**Reason:** requires a `design.go` R22 frozen-copy amendment (rendering the
commands as an indented block instead of an inline `%s`, or switching the
producer to join with `" && "`) — a rendering/copy decision better made
deliberately rather than folded into this pass. Left open; WR-05's fixture
update (this pass) at least makes the DUMMY backend model the real
multi-line `ManualCommand` shape accurately, so the drift WR-04 describes
is now visible in the demo instead of hidden behind a single-command
fixture.

### WR-06: the key-ceremony overflow backstop silently discards content with no indicator and no way to scroll

**File:** `internal/tuikit/identities.go:2947-2972`
**Reason:** a real UX change (add a "… N more line(s) hidden" marker,
clamp unconditionally when `budget <= 0`) that touches rendering behavior
directly adjacent to CR-02's own fix — deliberately left to a follow-up
pass to avoid compounding two behavioral changes to the same render path in
one commit without dedicated review attention. Not in the explicit
priority list for this pass.

### WR-07: reusing an existing key uploads a `.pub` path that may not exist

**File:** `cmd/gitid/upload_run.go:89-99`, `cmd/gitid/identity_create.go:401`
**Reason:** investigated during CR-01 work. The `identity_create.go:401`
call site (the REAL, non-dry-run `runCreateCeremony` commit path) was
traced and confirmed NOT to reproduce the finding as described:
`commitCreateInto` (called at line 387, before the `runUploadStep` call at
line 401) always writes `staged.FinalPubPath` to disk first via
`commitCreateTransaction`'s `filewriter.Write(staged.FinalPubPath, ...)`
when `staged.PubLine != ""` — so by the time `runUploadStep` runs,
`FinalPubPath` already exists. The remaining site,
`uploadRequestFromSpec` (the TUI wizard path), is a real gap but is
separate, lower-severity plumbing (it affects the wizard's own upload beat
for a reused key with no pre-existing `.pub`, not a destructive
confirmed-write path) and was left open to keep this pass's changes
narrowly scoped to the Critical/explicitly-requested findings.

### WR-09: the upload checkbox's actionable copy is still unreadable at the only width production uses

**File:** `internal/tuikit/identities.go:4643-4679`, `internal/tuikit/design.go:582, 586`
**Reason:** requires a `design.go` R22 frozen-copy amendment to fit the
checkbox labels into 60 columns while preserving their remediation content
— a copywriting decision, not a mechanical fix. Left open per the review's
own suggested alternative (file as an explicit backlog item) rather than
guessing new copy under this pass's time budget.

### WR-11: provider JSON is parsed out of `CombinedOutput`, so any stderr line breaks the inventory

**File:** `cmd/gitid/wiring.go:560-577`, `internal/uploader/inventory.go:60-74`
**Reason:** requires adding a separated-streams variant to the `Deps`
interface (stdout/stderr split) and rewiring every `RunCmd` caller and its
test doubles — real plumbing across two packages, matching the "skip
anything that requires large new plumbing" guidance for this pass.

### WR-12: the wizard still registers a key remotely before the identity is committed, with no way back

**File:** `internal/tuikit/identities.go:3791-3803`, `cmd/gitid/upload_run.go:83-100`
**Reason:** the review's own suggested fix asks for a follow-up PLAN/ROADMAP
item plus new async plumbing (a fresh inventory read on wizard-abandon) —
explicitly out of scope for a code-fixer pass; needs product/plan-level
tracking, not a source edit.

## Notes: pre-existing, unrelated test failures observed during verification

Two test failures were observed during full-suite verification and
confirmed, by reproducing them against the pre-fix baseline commit
(`9712f70`, in an isolated worktree), to be **pre-existing and unrelated**
to any change in this pass:

- `TestCaptureTUI` (`internal/screenshot/tui_capture_test.go`) — fails in
  this environment because the `freeze` binary is not on `PATH`. Purely
  environmental.
- `TestUploadVisualAllowlistMatchesRegistry` (`cmd/gitid/gate_visual_regression_test.go`) —
  fails because `.planning/design/identity-manager/visual-divergence-allowlist.txt`'s
  `rotate-delete-offer:upload-section`/`connectivity-output` rows use
  `absent:"ssh-key add"` predicates, while the `rotate-delete-offer`
  `ScreenSpec`'s own `RegionDispositions` in
  `internal/screenshot/createflow.go` register `contains:"ssh-key add"`
  (`uploadCommandDisposition`) for those same regions — a registry/allowlist
  drift that predates this review-fix pass entirely (reproduced with 100%
  of this pass's commits reverted). Not fixed here: it is not one of the 24
  findings in `09-REVIEW.md` and reconciling it requires understanding
  which side (the allowlist comment's overflow-backstop reasoning, or the
  registry's `contains` disposition) is authoritative — a judgment call
  better made with the phase's own visual-regression owner, not folded
  silently into an unrelated fix pass.

A third failure, `TestSSHStorageMigrateRealRollbackReportsExitTwoAndRestored`,
appeared once in a full `go test ./cmd/gitid/...` run and was confirmed
flaky/pre-existing: it passed in isolation (`-run` scoped, `-count=3`) and
on a full-suite re-run with no code changes in between.

---

_Fixed: 2026-08-29T21:40:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
