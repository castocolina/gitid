---
phase: 09-upload-credentials-assist
fixed_at: 2026-08-31T02:10:00Z
review_path: .planning/phases/09-upload-credentials-assist/09-REVIEW.md
iteration: 1
findings_in_scope: 14
fixed: 11
skipped: 3
status: partial
---

# Phase 9: Code Review Fix Report

**Fixed at:** 2026-08-31T02:10:00Z
**Source review:** .planning/phases/09-upload-credentials-assist/09-REVIEW.md (iteration 3)
**Iteration:** 1

**Summary:**
- Findings in scope: 14 (0 Critical, 14 Warnings — `fix_scope: critical_warning`, the 14 Info findings excluded)
- Fixed: 11 (WR-01, WR-02, WR-04, WR-05, WR-06, WR-07, WR-08, WR-09, WR-12, WR-13, WR-14)
- Skipped: 3 (WR-03, WR-10, WR-11 — all three require a design/copy decision, not a mechanical code fix; see below)

Every code fix below was written test-first: a regression test was added
(or an existing one extended), confirmed to FAIL against the pre-fix code
(reproduced via `git stash` on just the production file, or — for WR-07 —
via a compile-time RED to avoid an actual infinite-loop hang), then
confirmed to PASS once the fix was restored. After all ten new fixes landed:

```
go build ./...                                              # clean
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...        # all packages ok
make lint                                                    # golangci-lint: 0 issues (both the untagged and screenshot-tagged runs)
```

Targeted `-tags e2e` runs were also used per-fix to verify no regression in
the real-binary PTY suites the change touched (see each entry below).

## Fixed Issues

### WR-01: `UploadRunMsg` carries no identity — a stale reply from a different identity's in-flight upload could be consumed as the current one's

**Files modified:** `internal/tuikit/views.go`, `internal/tuikit/identities.go`, `internal/tuikit/backend_stub_test.go`, `internal/tuikit/identity_manager_upload_test.go`, `cmd/gitid/wiring.go`
**Commit:** `cdc6932`
**Applied fix:** Added `UploadRunMsg.Name`, set it at every `RunUpload`/`RunUploadForIdentity` dispatch site (real backend and stub), and guarded both consumers (`paneKeyCeremony`, `paneRegisterKey`) on `run.Name == m.selected` / `run.Name == m.registerKeyName`, mirroring the existing `RegisterKeyPlanMsg` stale-guard idiom. Two new regression tests deliver a stale `UploadRunMsg` for identity A while the model is mid-ceremony/mid-registration on B and assert the reply is discarded (no `RotateDeleteOffer` dispatch, pending flag untouched). Four existing tests that constructed `UploadRunMsg{View: ...}` directly were updated to carry the matching `Name`.

### WR-02: the fake-gh fixture recorded added keys under a hardcoded title instead of the real `--title` value

**Files modified:** none — already fixed
**Commit:** none (verified, no new commit)
**Applied fix:** This finding was already resolved in the working tree by commit `9c77d9a` ("fix(09): record the real gh --title in FakeGHTrackAddedKeys, not a synthetic one"), which lands after the review's own timestamp (18:35:00Z) but before HEAD (18:41:54Z) — evidently a fix applied in this same session window, before this fixer pass started. I verified it is genuinely effective, not vacuous: I temporarily disabled `OldKeyCandidates`' `blob == currentBlob` exclusion and re-ran the three rotate-delete e2e tests; two of three went RED (`TestIdentityManager_RotateDeleteOfferDefaultsToLeave`, `TestIdentityManager_RotateDeleteOfferDeletesOnExplicitChoice`), proving the fixture now genuinely exercises CR-01's exclusion logic. Restored the temporary change and confirmed all three tests pass again with the real code.

### WR-04: e2e frame captures for phases 6/7 wrote straight into TRACKED baseline directories

**Files modified:** `e2e/global_ssh_pty_e2e_test.go`, `e2e/global_git_pty_e2e_test.go`, `e2e/global_ssh_storage_pty_e2e_test.go`
**Commit:** `532a9cc`
**Applied fix:** Routed `captureGlobalSSHFrame`, `captureGlobalGitFrame`, and `captureStorageFrame` through the existing `saveFrame` helper (gitignored `tmp/ui-frames/`, the same fix WR-12/iteration-2 already applied to the 05.7 helper) instead of writing into `.planning/phases/06-*/07-*/ui-frames/`. Verified by running one representative test per file and confirming `git status` on the tracked directories stays clean while `tmp/ui-frames/` receives the captured frames.

### WR-05: the dummy backend's upload-preview commands were unquoted, contradicting WR-18's real-backend fix

**Files modified:** `internal/dummytui/fixturebackend.go`, `internal/screenshot/createflow.go`
**Commit:** `be319ab`
**Applied fix:** Shell-quoted the D-07 title in both `FixtureBackend.RunUpload`'s preview commands and `offlineCaptureBackend.RunUpload`'s mirror (the latter's own doc comment requires byte-identical shape to the former — leaving it unquoted would have desynced the two). Verified via `go test -tags screenshot ./internal/screenshot/...` (all passing tests still pass; two pre-existing unrelated failures — `TestCaptureTUI`, environmental, and `TestRegionDiffCoverage`, pre-existing before this change — confirmed via A/B stash comparison) and the full `internal/dummytui`/`internal/tuikit`/`cmd/gitid` suites.

### WR-06: create/clone/rotate/new-key `--dry-run` help text did not disclose the read-only provider probes

**Files modified:** `cmd/gitid/identity_upload.go`, `cmd/gitid/identity_create.go`, `cmd/gitid/identity_clone.go`, `cmd/gitid/identity_key.go`, `cmd/gitid/identity_upload_test.go`
**Commit:** `522b56b`
**Applied fix:** Extracted the two duplicated dry-run help strings (create/clone share one, rotate/new-key share the other) into shared constants and appended the same probe-disclosure clause register-key's own `--dry-run` already used. Added `TestDryRunFlagTextIsPinnedAcrossWriteVerbs`, the `--dry-run` sibling of the existing `--no-upload` anti-drift test.

### WR-07: `glabInventory` had no page cap — a provider ignoring `--page` would loop forever

**Files modified:** `internal/uploader/inventory.go`, `internal/uploader/inventory_test.go`
**Commit:** `4720cde`
**Applied fix:** Bounded the loop at `glabMaxPages` (40 pages / 1200 keys) and return a named error on exhaustion instead of spinning. RED was proven via a compile-time failure (`undefined: glabMaxPages`) rather than actually letting the pre-fix infinite loop run, to avoid a real hang; the new regression test's fake `RunCmd` always returns a full page and asserts termination after exactly `glabMaxPages` calls.

### WR-08: the overflow backstop skipped clamping entirely when `budget <= 0`

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/upload_section_test.go`
**Commit:** `354769e`
**Applied fix:** Removed the `budget > 0` guard at both overflow-backstop sites (`renderKeyCeremony`'s D-04/upload tail, `renderUploadSection`'s manual-fallback block); both now clamp through `maxInt(1, budget)` unconditionally and append a "N more line(s) hidden" cue. `TestUploadSectionClampsFallbackEvenWhenBudgetIsNegative` isolates the fallback's own contribution (vs. a rows-only render) to prove it stays bounded even when budget is deeply negative. I also re-ran the real `TestIdentityManager_RotateDeleteOffer*` PTY suite with and without the fix (A/B via `git stash`) and confirmed byte-identical output in the scenarios those tests exercise — `budget` stays positive there, so the negative-budget branch is not reached by the currently tracked frames/allowlist dispositions. **Note for a human:** if a future real rotate scenario (more identities, longer paths/hostnames) genuinely drives `budget <= 0`, the `rotate-delete-offer:upload-section:absent:"ssh-key add"` disposition in `.planning/design/identity-manager/visual-divergence-allowlist.txt` may need re-evaluation at that point — the review explicitly flagged this as a follow-up and I found no evidence it is needed today.

### WR-09: a retry after a partial delete could never succeed

**Files modified:** `internal/tuikit/views.go`, `internal/tuikit/identities.go`, `internal/tuikit/backend_stub_test.go`, `cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`
**Commit:** `4c95fd3`
**Applied fix:** Added `RotateDeleteCommitMsg.RemainingKeyID` — the same opaque candidate encoding, narrowed to only the candidates that did NOT delete successfully. The model applies it to `rotateDeleteConfirmedID` on a failed commit, preserving R12's retention rule (still the same reviewed targets, just narrowed). `TestCommitRotateDeleteOldKeyRemainingKeyIDDropsSucceededCandidates` proves a partial failure's `RemainingKeyID` decodes to only the failed candidate and a subsequent retry never re-sends the one that already succeeded.

### WR-12: provider JSON was parsed straight out of `CombinedOutput`

**Files modified:** `internal/uploader/inventory.go`, `internal/uploader/inventory_test.go`
**Commit:** `8f98f0a`
**Applied fix:** Applied the review's "at minimum" option (not the larger separated-streams `Deps` variant): added `stripLeadingNonJSONLines`, which skips whole lines before the first line that looks like the start of a JSON value, so a stderr banner on a successful call no longer corrupts a healthy inventory read. When the cleaned tail still fails to parse, the discarded prefix is carried into the error text for diagnostic context. Two new tests cover the success case (banner + valid JSON parses) and the negative control (a genuinely malformed payload still fails, with the discarded text in the error).

### WR-13: the wizard registers a key before the identity is committed, with no removal path — cheap mitigation applied; full fix deferred

**Files modified:** `internal/tuikit/identities.go`, `internal/tuikit/upload_section_test.go`
**Commit:** `0dc56e3`
**Applied fix:** Per the task instructions, only the review's own "cheap mitigation" was applied, not the full fix. Added `wizardAbandonUploadNote`: on leaving the wizard from step 0 (both the Shift+← and Esc exit paths) after the upload beat registered at least one NEW key (`UploadRowUploaded`), a note carrying the exact D-07 title is surfaced so the user can find and remove it manually. The title is extracted from the already-rendered announce command line rather than recomputed client-side, since the wizard model has no access to the machine hostname the real title embeds. A positive regression (note appears) and a negative control (no note when nothing was genuinely registered) were added.
**Deferred (not attempted):** the review's own Fix section explicitly calls for a full PLAN/ROADMAP-level follow-up — either warn before the upload beat runs, or give D-04-style removal reach to a key registered by an abandoned wizard. This is a product-behavior decision beyond a code-review fix pass and was intentionally left out of scope, exactly as instructed.

### WR-14: the wizard's reuse path could upload a `.pub` that does not exist

**Files modified:** `cmd/gitid/upload_run.go`, `cmd/gitid/wiring_test.go`
**Commit:** `dbcaf78`
**Applied fix:** When the reuse path's `FinalPubPath` does not actually exist on disk, the already-derived `staged.PubLine` is now written to a throwaway sibling in the session's staging directory (the same one the generate path and both connectivity test stages already use) instead of failing to read a file that was never written — never to `TempPrivatePath`, which for reuse equals `FinalPrivatePath` (the user's real key), per CR-01's lesson. `TestRunUploadReusePathWithMissingPubSucceeds` reuses a real generated key with its `.pub` removed, drives it through the full `RunUpload` beat, and reproduced the exact pre-fix failure verbatim in RED (`open ~/.ssh/id_ed25519_acme.pub: no such file or directory`) before confirming GREEN.

## Skipped Issues

### WR-03: the D-08 register-key pane mutates on open with no confirmation step

**File:** `internal/tuikit/identities.go:2528-2534` (the `u` hotkey), `:2650-2662` (`openRegisterKey`), `:2346-2364` (plan → immediate upload), `:2665-2675` (`handleRegisterKeyKey`)
**Reason:** The review's fix is correct in spirit (CLAUDE.md's "mutations happen only after ... user confirmation" is a binding project rule this screen genuinely violates), but implementing it collides with an explicit, DOCUMENTED design decision I found while reading the actual source and the project's design contracts before editing — not something the review itself surfaced:
- `.planning/design/identity-manager/FIELDS.md`'s own field-4 note for `register-key-modal` states: *"the D-01 checkbox is OMITTED here — opening the modal IS the explicit opt-in, so it announces-and-runs immediately when the provider matches and the tool is authenticated"* — a deliberate, tracked design contract, not an oversight.
- `TestIdentityManager_RegisterKeyModalRuns`'s own doc comment encodes the same intent: *"D-02: opening the pane IS the opt-in, so registration runs with no further keystroke."*
- The project's established convention (confirmed by `RegisterKeyModalHeadingFmt`, `RotateDeleteOfferHeadingFmt`, etc.) routes ALL frozen UI copy through `design.go` with a Copywriting Contract row in `09-UI-SPEC.md` — a confirm-step's copy would need the same, which is a copywriting/design decision, not a mechanical code change.
- The blast radius is large and cross-cutting: at least 3 real-binary e2e tests, a real-vs-dummy compiled comparison test, tracked UI-frame baseline snapshots, and the `FIELDS.md` manifest parser would all need coordinated updates to stay consistent with a changed sequencing contract.

This is the same category as WR-10/WR-11 (a design/copy decision, not a scoped bug fix) even though it was not one of the three the task pre-identified as deferred — I discovered the conflict only after reading the actual design docs, per the fixer's own "verify before applying" charter. Recommend routing through `/gsd-discuss-phase` or an explicit design amendment before implementing.

### WR-10: the multi-line `ManualCommand` is interpolated into a single-line sentence

**File:** `cmd/gitid/upload_run.go:520-527`, `internal/tuikit/design.go:695`, `internal/tuikit/identities.go:2796, 2805, 3003`
**Reason:** Pre-identified as deferred by the task instructions. Fixing this requires a `design.go` R22 frozen-copy amendment (either an indented-block rendering decision or switching to a `" && "`-joined single line) — a deliberate copy/rendering decision, not a mechanical bug fix.

### WR-11: the upload checkbox's actionable copy is unreadable at the only width production uses

**File:** `internal/tuikit/identities.go:4673-4703`, `internal/tuikit/design.go:582, 586`
**Reason:** Pre-identified as deferred by the task instructions. Fixing this requires amending the frozen copy to fit 60 columns (a copywriting decision) or filing the R22 amendment as an explicit tracked item — not a mechanical bug fix.

---

_Fixed: 2026-08-31T02:10:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
