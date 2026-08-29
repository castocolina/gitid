---
phase: 09-upload-credentials-assist
fixed_at: 2026-08-29T19:44:44Z
review_path: .planning/phases/09-upload-credentials-assist/09-REVIEW.md
iteration: 1
findings_in_scope: 20
fixed: 19
skipped: 1
status: partial
---

# Phase 9: Code Review Fix Report

**Fixed at:** 2026-08-29T19:44:44Z
**Source review:** .planning/phases/09-upload-credentials-assist/09-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 20 (CR-01, CR-02, WR-01 through WR-18 — `fix_scope: critical_warning`, Info findings excluded)
- Fixed: 19
- Skipped: 1 (WR-16)

Every fix below compiles, passes `go vet ./...` under every build tag
(`screenshot`, `smoke`, `e2e`), and passes `golangci-lint` with zero issues.
Each fix was verified with a regression test that was confirmed to fail
against the pre-fix code (reproduced locally, in most cases by temporarily
reverting the specific change and re-running the test) before being restored
to the passing state. `go test ./...` (whole module) and `go test -race
./cmd/gitid/...` both pass after all 19 fixes are applied together.

## Fixed Issues

### CR-01 + CR-02: D-04 delete offer can target the just-uploaded key / delete argv issued against the wrong GitHub API namespace

**Files modified:** `internal/uploader/inventory.go`, `internal/uploader/inventory_test.go`, `internal/tuikit/views.go`, `internal/tuikit/identities.go`, `cmd/gitid/upload_run.go`, `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `1ba017e`
**Applied fix:** Both findings share the same code path (D-04's interactive
old-key delete) and were fixed together, as the priority note directed.

- Added `uploader.OldKeyCandidates(existing, title, currentBlob)`: reads the
  account's CURRENT public key and excludes whatever inventory record
  matches its blob (the key the rotate ceremony just registered under the
  identical title), then refuses (returns `nil`) rather than guess if what
  remains still spans more than one distinct old key blob.
- Threaded `Registration` through `deleteArgs`/`DeleteKey`/`DeleteCommandPreview`
  so GitHub's `/user/keys` and `/user/ssh_signing_keys` — independent,
  freely-colliding ID spaces — are addressed via the correct REST resource
  (`api -X DELETE user/keys/<id>` vs `user/ssh_signing_keys/<id>`), never the
  old fixed `gh ssh-key delete <id> --yes`.
- `CommitRotateDeleteOldKey` now decodes the full candidate set the offer
  resolved (opaque JSON in `RotateDeleteOfferView.KeyID`, since `internal/tuikit`
  never imports `internal/uploader`) and deletes every one of them, reporting
  failure — never the `✓ Old key removed` copy — unless all succeed.
- Added `RotateDeleteOfferView.KeyDetail`, a new human-readable field
  (`ID <ids> — <blob suffix>`) rendered in `renderRotateDeleteOffer` so the
  confirmation identifies the specific record(s) under review, not just the
  title the new key shares.
- New tests: `TestOldKeyCandidatesExcludesTheJustRegisteredKey`,
  `TestOldKeyCandidatesReturnsBothRegistrationsOfTheSameOldKey`,
  `TestOldKeyCandidatesRefusesWhenMoreThanOneDistinctOldKeyRemains`,
  `TestOldKeyCandidatesReturnsNilWhenNothingRemains`,
  `TestRotateDeleteOfferExcludesTheJustRegisteredKeySharingTitle`,
  `TestCommitRotateDeleteOldKeyDeletesExactlyTheConfirmedID` (rewritten for
  the namespace-scoped argv), `TestCommitRotateDeleteOldKeyDeletesEveryRegistration`,
  `TestCommitRotateDeleteOldKeyReportsPartialFailure`.

### WR-01: unpaginated GitHub key inventory reads (30-key REST default)

**Files modified:** `internal/uploader/inventory.go`, `internal/uploader/inventory_test.go`, `cmd/gitid/wiring_test.go`, `cmd/gitid/gate_visual_regression_test.go`, `e2e/harness_test.go`, `e2e/create_flow_pty_e2e_test.go`
**Commit:** `abcc229`
**Applied fix:** Both GitHub `api` calls now pass `--paginate`.
`decodeProviderKeyPages` replaces the single `json.Unmarshal` with a
streaming `json.Decoder` loop, since `gh api --paginate` writes one JSON
array per page back-to-back with no separator. glab's pagination is
explicitly left as a documented follow-up (see rationale in the commit —
guessing the wrong flag risked breaking every glab inventory read).
Required updating every mock/fixture that positionally matched
`args[1] == "user/keys"` (now `--paginate` occupies that position) across
`cmd/gitid`'s RunCmd mocks and the e2e fake-gh shim.

### WR-02: `printUploadOutcome` claimed a `--no-upload` flag the user never passed

**Files modified:** `cmd/gitid/identity_upload.go`, `cmd/gitid/upload_run.go`, `cmd/gitid/upload_run_test.go`, `internal/tuikit/views.go`
**Commit:** `470f4aa`
**Applied fix:** Added `UploadRunView.SkippedByFlag`, set only by
`runUploadStep`'s `noUpload` branch (which now builds a real view and routes
it through `printUploadOutcome` instead of printing directly).
`printUploadOutcome` gates the `--no-upload` note on `SkippedByFlag` alone;
`Skipped` (the Omitted/Disabled derived states) still renders
`ManualFallback` but never the flag note.

### WR-03: `FailureNotAuthenticated` classified and discarded

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`, `internal/tuikit/design.go`, `internal/uploader/classify_test.go`
**Commit:** `60f679b`
**Applied fix:** Added `tuikit.UploadNotAuthenticatedFmt` and mapped
`uploader.FailureNotAuthenticated` to it in `toUploadResultRow`, using the
already-exported (now wired) `uploader.ToolName(tool)`.

### WR-04: `RedactCLIOutput` missed GitHub fine-grained PATs

**Files modified:** `internal/uploader/classify.go`, `internal/uploader/classify_test.go`
**Commit:** `1aeb474`
**Applied fix:** Extended `ghTokenPattern` to also match `github_pat_...`.

### WR-05: raw error strings bypassed redaction

**Files modified:** `cmd/gitid/upload_run.go`, `cmd/gitid/wiring.go`, `cmd/gitid/upload_run_test.go`, `cmd/gitid/wiring_test.go`
**Commit:** `bd8c2cc`
**Applied fix:** `planUpload`'s ReadFile failure and `RunUpload`'s
`uploadRequestFromSpec` failure now route through
`uploader.RedactCLIOutput(err.Error(), b.home, 58)` like every other
failure path on the same beat.

### WR-06: hard-truncated upload checkbox row, tests at the wrong width

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/upload_section_test.go`
**Commit:** `c3af8b9`
**Applied fix:** `ansi.Truncate` now passes `"…"` as the tail marker instead
of `""`. Moved `TestUploadCheckboxRendersAllFourStates` and
`TestUploadCheckboxIsLegibleWithoutColor` to width 60 (the only width the
real wizard caller uses). Shortening the frozen copy itself would need a
separate `design.go` R22 amendment — noted as out of scope for this
targeted fix.

### WR-07: rotate delete-offer trapped keyboard input — Esc could not leave

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/identity_manager_upload_test.go`
**Commit:** `bb27f08`
**Applied fix:** Added an `esc` case resolving to the same non-destructive
"leave it" answer the default choice row produces, guarded by the same
`rotateDeleteCommitPending` check `enter` already uses.

### WR-08: rotate/new-key `--dry-run` allegedly skipped the upload preview for tilde-form identities — **does not reproduce**

**Files modified:** `cmd/gitid/identity_test.go`
**Commit:** `b6517ad`
**Applied fix:** Investigated and found the cited bug does not reproduce:
`printKeyCeremonyDryRun` already calls `acct = b.normalizeAccountForWrite(acct)`
*before* the `fileExists` guard the finding cites, and
`normalizeAccountForWrite` already expands `acct.PubPath`'s tilde form
(confirmed empirically with a throwaway probe against the exact
`seedDeleteFixture` shape the finding cites). No production change was
needed at `cmd/gitid/identity_key.go:217`.

Investigating it surfaced a real, adjacent defect instead: because
`acct.PubPath` resolves to a REAL, existing seeded key,
`TestIdentityRotateDryRunNamesCurrentKeyAndCaveat` (which drives
`runIdentityKeyVerb(... DryRun: true)` without `NoUpload: true`, and which
builds its own `*realBackend` internally with no injectable `uploaderDeps`
seam) genuinely reaches `planUpload` → `uploader.Inventory`, which shells
out to whatever real `gh`/`glab` is on PATH — confirmed concretely on the
machine this fix was written on, which has an authenticated `gh` session.
Every sibling `DryRun` CLI test in this file already sets `NoUpload: true`
for exactly this reason; this was the one test that predated the upload
preview addition and was never updated. Fixed by adding `NoUpload: true` —
the test's actual assertions (R2-13 current-key labelling and caveat) are
unrelated to the upload preview.

### WR-09: R3 provider-call guard scanned only `wiring.go`

**Files modified:** `cmd/gitid/upload_run_test.go`, `cmd/gitid/wiring_test.go`
**Commit:** `7e2fa8c`
**Applied fix:** Both `TestRunUploadForIsTheOnlyOrchestration` and
`TestRunUploadDoesNotCallProviderCommandsOutsideATeaCmd` now
`filepath.Glob("*.go")` and scan every file in the package except
`upload_run.go` itself and `_test.go` files, with a positive-scan-count
assertion so the walk cannot vacuously pass.

### WR-10: `TestRegisterKeyDryRunExecutesNoUpload` re-implemented the production branch

**Files modified:** `cmd/gitid/identity_upload_test.go`
**Commit:** `085a4f4`
**Applied fix:** Rewrote the test to drive `runIdentityRegisterKey` with
`DryRun: true` directly, asserting on `cmd.OutOrStdout()`. Added
`fakeGHOnPath`, a local unit-test-scoped fake `gh` binary (mirroring
`wiring_storage_test.go`'s existing `backendWithFakeSSH` PATH-prepend
technique) so the real entry point — which builds its own `*realBackend`
with the real `uploaderDeps` and has no injectable seam — can be driven
end-to-end with zero risk of reaching an actual `gh`/`glab` CLI. Verified
the rewritten test fails when the production dry-run branch is disabled
(temporarily, locally) and that the failure is a genuine "recorded a REAL
ssh-key add" catch, not a vacuous pass.

### WR-11: `confirmUpload` coupled `wanted[i]` to `rows[i]` positionally

**Files modified:** `cmd/gitid/upload_run.go`, `cmd/gitid/wiring_test.go`
**Commit:** `dccb8d1`
**Applied fix:** Added `registrationOf(row)`, the reverse of
`toUploadResultRow`'s `uploader.Registration` → `tuikit.UploadRegistration`
mapping. `confirmUpload` now keys `toConfirm` on each row's own
`Registration` instead of a positional zip against `wanted`; the now-unused
`wanted` parameter was dropped from the signature.

### WR-12: `extractUploadSection`'s marker list collided with the D-01 checkbox's own label

**Files modified:** `internal/screenshot/createflow.go`, `internal/screenshot/createflow_regions.go`, `internal/screenshot/createflow_regions_test.go`
**Commit:** `e23c082`
**Applied fix:** Dropped `"not logged in to"` from `uploadMarkers` (a
substring of `UploadCheckboxLabelUnauthFmt`, contradicting the function's
own stated exclusion of `"Register with "` for the same collision reason).
This required updating the `"upload-checkbox-unauth"` screen spec, whose
`RequiredRegions` was satisfied only via that same buggy match — reassigned
to `RegionFormFields` (the semantically correct region, since the checkbox
row is literally the last row of the SSH form per its own doc comment).
Verified the whole `internal/screenshot` suite and the relevant
`cmd/gitid` gate-visual-regression suite pass after the change (only the
pre-existing, environment-only `TestCaptureTUI` failure remains — missing
`freeze` binary, unrelated to this fix).

### WR-13: `gitid-frame-promote` overwrote tracked baselines before validating the capture set

**Files modified:** `cmd/gitid-frame-promote/main.go`, `cmd/gitid-frame-promote/main_test.go` (new)
**Commit:** `f9772c7`
**Applied fix:** Extracted the promotion loop into a directly-testable
`promoteFrames(srcDir, dstDir, frames, commit, progress)` that reads and
validates every source frame into memory first, and only writes to `dstDir`
once none are missing.

### WR-14: dead exported API in `internal/uploader`, including the D-11-violating `Detect`

**Files modified:** `internal/uploader/uploader.go`, `internal/uploader/uploader_test.go`, `cmd/gitid/wiring.go`
**Commit:** `008484f`
**Applied fix:** Deleted `Detect` (superseded by `DetectFor`) and its six
tests. Wired `DeleteRecordedKey` into `CommitRotateDeleteOldKey` in place of
the lower-level `DeleteKey` call. Unexported `UploadKey` → `uploadKey` and
`TitleMatchesThisMachine` → `titleMatchesThisMachine` (no production
caller for either exported form). Dropped the exported `TrimOutput`
wrapper entirely (nothing called it, production or test).

### WR-15: `DetectFor` always reported `AuthNotLoggedIn` for a found tool

**Files modified:** `internal/uploader/uploader.go`, `internal/uploader/uploader_test.go`, `cmd/gitid/upload_run.go`, `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `adff61e`
**Applied fix:** Changed `DetectFor`'s third return value from
`status AuthStatus` (always `AuthNotLoggedIn` for a found tool, since it
never actually probed) to a plain `found bool`. Updated all five call sites
— all of which already special-cased only the not-found branch and ran
their own separate `AuthCheck` when needed, confirming the old `AuthStatus`
return carried no real information beyond found/not-found.

### WR-16: wizard registers a key remotely before commit, with no cleanup path — **skipped**

**Reason:** See Skipped Issues below.

### WR-17: eligibility memo mutex held across a subprocess call of up to 20s

**Files modified:** `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `1eff5c8`
**Applied fix:** Added a per-provider `*sync.Mutex` map
(`uploadEligibilityLocks`), created lazily under the existing outer mutex.
The outer mutex now guards only brief map accesses (get-or-create the
per-provider lock; read/write the memo cache) — never a subprocess call.
The per-provider lock is held for the whole probe, preserving the atomic
check-then-set property per provider while letting different providers run
concurrently. Verified under `go test -race`.

### WR-18: `CommandPreview` produced a line that could not be copied and re-run

**Files modified:** `internal/uploader/uploader.go`, `internal/uploader/inventory.go`, `internal/uploader/uploader_test.go`
**Commit:** `146548b`
**Applied fix:** Added `shellQuote` + `previewLine`, shared by
`CommandPreview` and `DeleteCommandPreview`. An argument with no
shell-special characters stays bare (readable common case); the D-07 title
(always contains spaces) is now single-quoted. The executed argv
(`deps.RunCmd(toolPath, args...)`) was never affected — only the rendered
preview string.

## Skipped Issues

### WR-16: the wizard registers a key remotely before the identity is committed, with no cleanup path

**File:** `internal/tuikit/identities.go:3766-3781`, `cmd/gitid/upload_run.go:81-98`
**Reason:** A correct fix — even the review's "At minimum" option — requires
resolving the newly-registered key's real provider ID via a fresh inventory
read after the fact (`uploader.DeleteCommandPreview` needs a `Registration`
+ `id`, neither of which `UploadResultRow`/`RegistrationResult` carries
today), which is new async plumbing (a message type + a dispatched `tea.Cmd`)
comparable in scope to the D-04 `RotateDeleteOffer` feature this same phase
already built — not a small, targeted fix. The review's "Better" option
(move the beat after the write ceremony) is a larger sequencing change
touching wizard step ordering, with a wide test/screenshot-baseline blast
radius this fixer cannot visually verify in a text-only environment. Given
the finding is Warning-severity, not Critical, and forcing a rushed
implementation risks destabilizing wizard navigation (extensive existing
test coverage, no visual verification available here), this was left
unimplemented rather than force a partial or unverified change.
**Original issue:** D-05 places the upload beat inside wizard step 1, before
the `ssh -T` gate and long before step 3's write ceremony. If the user
abandons the wizard after an upload succeeded, the public key is already
registered on the user's GitHub/GitLab account with the matching private
key destroyed, and gitid offers no way to remove it (the D-04 offer is
rotate-only).
**Recommendation:** A dedicated follow-up plan, mirroring D-04's
`RotateDeleteOffer` pattern but triggered from wizard-abandonment instead
of rotate-completion — at minimum surfacing `uploader.DeleteCommandPreview`
for the key that was registered when the wizard is abandoned after a
reported upload.

---

_Fixed: 2026-08-29T19:44:44Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
