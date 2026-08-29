---
phase: 09-upload-credentials-assist
reviewed: 2026-08-29T21:05:00Z
depth: standard
iteration: 2
supersedes: iteration 1 (2026-08-29T18:04:33Z) + 09-REVIEW-FIX.md
files_reviewed: 51
files_reviewed_list:
  - cmd/gitid-frame-promote/main.go
  - cmd/gitid-frame-promote/main_test.go
  - cmd/gitid/gate_visual_regression_test.go
  - cmd/gitid/identity.go
  - cmd/gitid/identity_clone.go
  - cmd/gitid/identity_create.go
  - cmd/gitid/identity_key.go
  - cmd/gitid/identity_test.go
  - cmd/gitid/identity_upload.go
  - cmd/gitid/identity_upload_test.go
  - cmd/gitid/main_test.go
  - cmd/gitid/upload_run.go
  - cmd/gitid/upload_run_test.go
  - cmd/gitid/wiring.go
  - cmd/gitid/wiring_test.go
  - e2e/create_flow_pty_e2e_test.go
  - e2e/debug_e2e_test.go
  - e2e/dummy_demo_e2e_test.go
  - e2e/git_configuration_pty_e2e_test.go
  - e2e/global_git_cli_e2e_test.go
  - e2e/global_git_pty_e2e_test.go
  - e2e/global_ssh_cli_e2e_test.go
  - e2e/global_ssh_pty_e2e_test.go
  - e2e/global_ssh_storage_pty_e2e_test.go
  - e2e/harness_test.go
  - e2e/health_fix_cli_e2e_test.go
  - e2e/health_fixer_pty_e2e_test.go
  - e2e/identity_cli_e2e_test.go
  - e2e/identity_manager_pty_e2e_test.go
  - e2e/ui_pty_e2e_test.go
  - internal/dummytui/doc.go
  - internal/dummytui/fixturebackend.go
  - internal/screenshot/createflow.go
  - internal/screenshot/createflow_packet_test.go
  - internal/screenshot/createflow_regions.go
  - internal/screenshot/createflow_regions_test.go
  - internal/screenshot/createflow_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/design.go
  - internal/tuikit/identities.go
  - internal/tuikit/identity_manager_upload_test.go
  - internal/tuikit/upload_copy_test.go
  - internal/tuikit/upload_section_test.go
  - internal/tuikit/views.go
  - internal/uploader/classify.go
  - internal/uploader/classify_test.go
  - internal/uploader/inventory.go
  - internal/uploader/inventory_test.go
  - internal/uploader/uploader.go
  - internal/uploader/uploader_test.go
findings:
  critical: 2
  warning: 12
  info: 10
  total: 24
status: issues_found
---

# Phase 9: Code Review Report (re-review, iteration 2)

**Reviewed:** 2026-08-29T21:05:00Z
**Depth:** standard
**Files Reviewed:** 51
**Status:** issues_found

## Summary

This is a follow-up review after the fix pass documented in `09-REVIEW-FIX.md`
(17 fix commits, `1ba017e`..`146548b`, plus the two e2e corrections in
`9712f70`). Two things were asked for: verify the CR-01/CR-02 fixes are
genuinely correct, and re-scan the whole Phase 9 diff with fresh eyes.

**CR-01 and CR-02 are correctly fixed at the level they were reported.** The
key-blob exclusion, the ambiguity refusal, the registration-scoped delete
argv, and the multi-registration delete-all-or-report-failure semantics all
hold end-to-end, and the supporting tests are real (they assert on recorded
argv, not on their own writes). Details and the exact traces are in the
verification section below. The threading of `Registration` through
`deleteArgs`/`DeleteKey`/`DeleteRecordedKey` and the opaque-JSON
`RotateDeleteOfferView.KeyID` payload introduced no compile-time or
call-chain regression — the whole module builds, vets, and its unit suites
pass (`go build ./...`, `go vet ./...`, `go test ./cmd/... ./internal/...`
all green as submitted).

What the fix pass did **not** close, and what it introduced, is the substance
of this report. Two new Critical findings:

1. **A `--dry-run` writes into the user's real `~/.ssh`.** The Phase 9
   upload-preview block in `runCreateDryRun` derives its staging `.pub` from
   `staged.TempPrivatePath`, which for the *reuse* path IS the user's real
   key path — so `gitid identity create --reuse-key … --dry-run` (and
   `identity clone --dry-run`, where key reuse is the default) creates a new
   file under `~/.ssh` with no confirmation and no backup, three lines above
   a comment promising "nothing is written under `~/.ssh`". The sibling
   function `uploadRequestFromSpec` guards the identical pattern with
   `staged.PrivPEM != nil`; this one does not.
2. **The D-04 delete offer is still ungated on whether the NEW key was
   actually registered.** The offer is dispatched on *any* `UploadRunMsg`,
   success or failure, and `OldKeyCandidates` returns a clean, unambiguous
   old-key candidate set precisely when the new key failed to register.
   Accepting the offer then removes the last key the account has. CR-01's own
   fix makes this *more* reachable, and the overflow backstop that CR-01's
   new `KeyDetail` line pushed the pane past (see the allowlist change in
   `9712f70`) now deterministically clips the upload beat's ✓/✗ rows off the
   very screen where the destructive question is asked.

Beyond those, the recurring themes are **half-applied fixes** (WR-12's marker
collision removed for the unauth label but not for the disabled one, which is
the same defect; WR-01's `--paginate` applied to `gh` but not `glab`; WR-06's
ellipsis added but the copy still unreadable at the production width) and
**fix-induced drift the fixture layer never followed** (the dummy backend
still advertises the exact `gh ssh-key delete <id> --yes` argv CR-02 removed
as wrong, and the now-multi-line `ManualCommand` is interpolated into a
single-line result sentence). All ten Info findings from iteration 1 are
still present verbatim — they were out of the fixer's `critical_warning`
scope — and are re-listed here so nothing is lost.

## Fix Verification (CR-01 / CR-02)

Verified by reading the merged code, not the fix report.

**CR-01 — title-collision exclusion.** `rotateDeleteOfferFor`
(`cmd/gitid/upload_run.go:437-450`) now reads the account's current `.pub`
through the tilde-expanding path, normalizes it, and passes it as
`currentBlob` to `uploader.OldKeyCandidates`
(`internal/uploader/inventory.go:222-240`). The exclusion (`blob ==
currentBlob → continue`) is correct, and the ambiguity refusal is stronger
than the review asked for: instead of `len(candidates) != 1` it refuses
whenever the surviving candidates span more than one *distinct blob*, which
correctly admits the legitimate two-record (auth + signing) GitHub case while
still refusing a prior rotation's leftover collision. `KeyDetail`
(`upload_run.go:526-538`, rendered at `internal/tuikit/identities.go:2988-2994`)
puts the reviewed IDs and a blob suffix on screen. **Correct**, with one
residual fail-open: see WR-01.

**CR-02 — registration-namespace-correct argv.** `deleteArgs`
(`inventory.go:177-193`) now dispatches per `Registration` to
`api -X DELETE user/keys/<id>` vs `user/ssh_signing_keys/<id>`; the fixed
`ssh-key delete <id> --yes` is gone. The namespace travels from
`OldKeyCandidates` → `encodeDeleteCandidates` → `RotateDeleteOfferView.KeyID`
(opaque JSON, never rendered — confirmed by grep: no render site reads
`KeyID`) → `rotateDeleteConfirmedID` (retained before dispatch, R12) →
`decodeDeleteCandidates` → `uploader.DeleteRecordedKey`
(`cmd/gitid/wiring.go:1637-1655`). `DeleteKey`'s `isAllDigits` guard still
runs *before* the ID is concatenated into the REST path, so the new
string-concatenation URL construction is not injectable. Every candidate is
deleted and success is claimed only when all succeed. The three
`TestCommitRotateDeleteOldKey*` tests assert on recorded argv, including a
negative assertion that an authentication delete never touches the signing
namespace. **Correct**, with two residuals: the retry path (WR-02) and the
now-multi-line manual command (WR-04).

**No regression from threading `Registration` through the chain.** The only
production callers of the changed signatures are
`CommitRotateDeleteOldKey` and `deleteCandidatesManualCommand`; `DeleteKey`'s
other former caller is gone. `FindByTitle` survives but is now dead in
production (IN-09). The R3 AST guard was widened to the whole package and has
a non-vacuous scan-count assertion (`upload_run_test.go:38-71`); it correctly
does not flag `DeleteRecordedKey`/`AuthCheck`/`DetectFor` in `wiring.go`,
which are outside the guarded set by design.

## Narrative Findings (AI reviewer)

### Critical Issues

#### CR-01: `--dry-run` writes a new file into the user's real `~/.ssh`

**File:** `cmd/gitid/identity_create.go:474-486` (with
`internal/identity/modes.go:64-79`, `cmd/gitid/wiring.go:473-476`,
`cmd/gitid/identity_clone.go:102`)

**Issue:** The Phase 9 upload-preview block added to `runCreateDryRun` stages
a `.pub` next to the staged private key:

```go
if staged.PubLine != "" {
    tempPub := staged.TempPrivatePath + ".pub"
    if !b.deps.PubExists(tempPub) {
        _ = b.deps.WritePub(tempPub, staged.PubLine)
    }
    runUploadStep(cmd.OutOrStdout(), b, uploadRequest{... PubPath: tempPub}, noUpload, true)
}
```

For the **generate** path `TempPrivatePath` is inside the mode-0700 staging
directory, which is what the comment describes. For the **reuse** path it is
not: `identity.StageReuse` sets

```go
TempPrivatePath:  existingKeyPath,   // the user's REAL key, e.g. ~/.ssh/id_ed25519_old
FinalPrivatePath: existingKeyPath,
```

so `tempPub` resolves to `~/.ssh/<key>.pub`, and `ensurePubReadOnly` returns
a non-empty `PubLine` even when that file is **absent** (it derives it from
the private key — that is the whole reason the fallback exists). The guard is
`!PubExists(tempPub)`, i.e. the write fires *exactly* in the case where the
file is missing. `WritePub` is the real `filewriter.Write`, which creates a
`gitid-*.tmp` file in `~/.ssh` and renames it into place.

Reachable via:

- `gitid identity create --reuse-key ~/.ssh/id_x --dry-run` where
  `~/.ssh/id_x.pub` does not exist (a hand-made key imported into gitid).
- `gitid identity clone <src> --name y --dry-run` — key reuse is the
  **default** (`!flags.NewKey`), so no opt-in flag is needed.

The cleanup guard three lines below deliberately (and correctly) skips
`Cleanup` for exactly this shape — `RemoveAll(filepath.Dir(TempPrivatePath))`
would delete `~/.ssh` — so the stray file is never removed. The literal
comment above the cleanup reads "nothing is written under `~/.ssh` or
`~/.gitconfig`", and CLAUDE.md's Engineering section makes an unconfirmed,
unbacked-up `~/.ssh` write a hard prohibition. The sibling site
(`cmd/gitid/upload_run.go:90-98`) guards the identical pattern with
`if staged.PrivPEM != nil` — the guard that distinguishes generate from
reuse — and is safe; this one omits it.

**Fix:** Apply the same guard, and fall back to the real (already existing)
public key when reusing:

```go
if staged.PubLine != "" {
    pubPath := staged.FinalPubPath
    if staged.PrivPEM != nil { // generated: a temp sibling in the staging dir only
        pubPath = staged.TempPrivatePath + ".pub"
        if !b.deps.PubExists(pubPath) {
            if werr := b.deps.WritePub(pubPath, staged.PubLine); werr != nil {
                return nil // skip the preview, as the comment already claims
            }
        }
    }
    if b.deps.PubExists(pubPath) {
        runUploadStep(cmd.OutOrStdout(), b, uploadRequest{Identity: in.Name, Hostname: in.Hostname, PubPath: pubPath}, noUpload, true)
    }
}
```

and add a regression test that runs `clone --dry-run` against a seeded reuse
key with no `.pub` and asserts the `~/.ssh` directory listing is byte-identical
before and after.

---

#### CR-02: the D-04 delete offer is presented even when the new key's registration failed — accepting it removes the account's last working key

**File:** `internal/tuikit/identities.go:2269-2285`,
`cmd/gitid/upload_run.go:421-450`, `internal/tuikit/identities.go:2947-2972`

**Issue:** The rotate result screen chains `CommitRotate` →
`RunUploadForIdentity` → `RotateDeleteOffer`. The chaining branch inspects
nothing about the upload's outcome:

```go
if run, ok := msg.(UploadRunMsg); ok && m.pane == paneKeyCeremony && m.keyCeremonyUploadPending {
    m.keyCeremonyUploadPending = false
    m.keyCeremonyUploadRun = run.View          // outcome stored, never consulted
    m.keyCeremonyPhase = "review"
    if m.keyCeremonyMode == KeyCeremonyModeRotate {
        m.rotateDeleteOfferPending = true
        return keyResult{model: m, cmd: m.backend.RotateDeleteOffer(m.selected)}
    }
```

`rotateDeleteOfferFor` likewise never looks at the upload result. Now trace
the failure case: the new key's registration fails (missing
`admin:public_key` scope, a rate limit, a cross-account conflict — all
classified paths this phase built). The provider inventory therefore contains
**only the old key** under the D-07 title. `OldKeyCandidates` excludes nothing
(the new blob is not there), finds one distinct blob, and returns a clean,
"unambiguous" candidate set. The offer renders as `Available`, the user
presses Delete, and `CommitRotateDeleteOldKey` removes the only key the
account still has — the private half of which was archived by the rotate
moments earlier. The identity can no longer authenticate or sign, and gitid
reports `✓ Old key removed from GitHub.`

CR-01's fix makes this *more* likely to be reached, not less: before it, a
two-record title collision was resolved by first-match; now the only shape
that yields a confident answer is precisely "exactly one distinct old blob",
which is the shape a failed upload produces.

It is compounded by the render path. `9712f70` reclassified the
`rotate-delete-offer` visual-divergence rows from `contains:"ssh-key add"` to
`absent:"ssh-key add"` because CR-01's new `KeyDetail` line pushes the tail
past `frameBodyRows(30)`, so `renderKeyCeremony`'s backstop
(`identities.go:2956-2972`) clips the tail. The offer is written to the tail
**first**, and `ExactTextViewport.Clamp()` keeps the first `budget` lines from
`LineOffset = 0` — so what gets cut is the upload beat's rows, including the
`✗ Signing key registration failed: …` line. The pane is not focused or
scrollable while the offer owns key input (`identities.go:2768-2816`), so the
clipped text is unreachable. The user is asked a destructive question with the
evidence that should stop them cut off-screen, by design and now enshrined in
the allowlist.

**Fix:** Gate the offer on a successful registration of the new key, in the
model (cheapest, and keeps the backend decision layer untouched):

```go
if m.keyCeremonyMode == KeyCeremonyModeRotate && uploadSucceeded(run.View) {
    m.rotateDeleteOfferPending = true
    m.rotateDeleteOffer = RotateDeleteOfferView{}
    return keyResult{model: m, cmd: m.backend.RotateDeleteOffer(m.selected)}
}
// otherwise: no offer — the old key is the only working credential left
```

where `uploadSucceeded` requires every row to be `UploadRowUploaded` or
`UploadRowAlreadyPresent` (`AlreadyComplete` also qualifies). Belt-and-braces:
have `rotateDeleteOfferFor` refuse unless the *current* key's blob is present
in the freshly-read inventory for every desired registration — it already has
`existing`, `currentBlob`, and `uploader.HasRegistration` in hand:

```go
for _, reg := range desiredRegistrations(tool) {
    if !uploader.HasRegistration(existing, string(currentPub), reg) {
        return tuikit.RotateDeleteOfferView{Unavailable: "the new key is not fully registered yet — not offering to remove the old one"}
    }
}
```

Add a PTY/unit regression for "rotate + failed upload ⇒ no delete offer".

### Warnings

#### WR-01: `OldKeyCandidates` fails OPEN when key material is missing — CR-01's protection silently evaporates

**File:** `internal/uploader/inventory.go:222-240`, `cmd/gitid/upload_run.go:441-447`

**Issue:** The exclusion is conditional on non-empty blobs:

```go
blob := NormalizeKeyBlob(rec.Key)
if currentBlob != "" && blob == currentBlob {
    continue // never offer the key we just registered
}
```

`NormalizeKeyBlob` returns `""` for anything with fewer than two whitespace
fields. Two fail-open directions, both of which restore the exact CR-01 defect:

1. `currentBlob == ""` (the account's `.pub` read succeeded but is empty or
   truncated): the exclusion is skipped entirely. If the old key is no longer
   in the inventory, the sole surviving title match is the just-registered new
   key — one distinct blob — so it is returned as "the unambiguous old key".
2. Every `rec.Key` empty (a provider/`glab` JSON field rename, an API version
   that omits `key`): all blobs are `""`, the map has exactly one entry, the
   ambiguity refusal passes, and the just-registered key is back in the
   candidate set.

A destructive decision must never treat "I could not read the key material" as
"they do not match".

**Fix:**

```go
func OldKeyCandidates(existing []ExistingKey, title, currentBlob string) []ExistingKey {
    if currentBlob == "" {
        return nil // cannot prove which record is the NEW key — refuse
    }
    ...
        blob := NormalizeKeyBlob(rec.Key)
        if blob == "" {
            return nil // a record we cannot compare makes the whole set unsafe
        }
    ...
}
```

and add the two cases to `inventory_test.go` beside the existing
`TestOldKeyCandidates*` table.

#### WR-02: a retry after a partial delete can never succeed

**File:** `cmd/gitid/wiring.go:1641-1655`, `internal/tuikit/identities.go:2293-2301`

**Issue:** `CommitRotateDeleteOldKey` loops over every candidate and reports
failure if any one fails. The model keeps `rotateDeleteConfirmedID` (the full
encoded set) and leaves `rotateDeleteResolved = false` so the user can "press
Enter on Delete to retry" — and the retry re-sends **all** candidates,
including the ones already deleted. `gh api -X DELETE user/keys/<gone-id>`
returns 404 (non-zero), so the retry fails again, permanently. The user is
told to retry an action that structurally cannot succeed, and the "✓ Old key
removed" state is unreachable even once every registration is in fact gone.

**Fix:** Treat "already absent" as success in the loop:

```go
if out, err := uploader.DeleteRecordedKey(tool, toolPath, rec, b.uploaderDeps); err != nil {
    if isNotFound(out, err) { // 404 / "Not Found" — the goal state is already true
        continue
    }
    failed = append(failed, uploader.RedactCLIOutput(err.Error(), b.home, 58))
}
```

or drop each successfully-deleted candidate from `rotateDeleteConfirmedID`
before reporting the partial failure.

#### WR-03: `extractUploadSection` still collides with the D-01 checkbox — the disabled label was left in the marker list

**File:** `internal/screenshot/createflow_regions.go:1288-1302`,
`internal/tuikit/design.go:586`

**Issue:** WR-12's fix removed `"not logged in to"` because it is a substring
of the unauth checkbox label, contradicting the function's own stated
exclusion of `"Register with "`. The list still contains `"Auto-registration"`,
which is the first word of the *disabled* checkbox label:

```go
UploadCheckboxLabelDisabledFmt = "Auto-registration unavailable — %s has no gh/glab match here. …"
```

It survives the width-60 truncation, so every step-0 screen in the Disabled
state (the deny-shim PTY sessions, `upload-checkbox-disabled`, any future
non-`gh` spec) anchors `RegionUploadSection` on the ordinary form body — the
identical collision the fix pass just removed for the sibling state. The fix
was applied to one of two instances of the same defect.

**Fix:** Drop `"Auto-registration"` too; the disabled checkbox row belongs to
`RegionFormFields` for the same reason the unauth row was reassigned there.
Re-check the `upload-checkbox-disabled` spec's `RequiredRegions` afterwards.

#### WR-04: the CR-02 fix made `ManualCommand` multi-line, but it is interpolated into a single-line result sentence

**File:** `cmd/gitid/upload_run.go:512-518`, `internal/tuikit/design.go:695`,
`internal/tuikit/identities.go:2788, 2797, 2995-2996`

**Issue:** `deleteCandidatesManualCommand` now joins one preview line **per
candidate** with `"\n"` (correct — a rotated GitHub key has two
registrations). That value is then substituted into a sentence:

```go
RotateDeleteOfferResultLeftFmt = "Left in place — remove it yourself: %s"
...
m.rotateDeleteResult = fmt.Sprintf(RotateDeleteOfferResultLeftFmt, m.rotateDeleteOffer.ManualCommand)
...
b.WriteString(" " + m.rotateDeleteResult + "\n")
```

For the normal two-registration case the rendered result is a sentence whose
second command starts at column 0 with no leading space and no context, inside
a fixed 100x30 frame that CR-02 already pushed into the overflow backstop.
The frozen copy assumes one command; the producer now emits N. The dummy
fixture emits exactly one, so no baseline frame shows the real shape.

**Fix:** Render the commands as their own indented block instead of an inline
`%s` — e.g. keep the sentence as `"Left in place — remove it yourself:"` and
write each preview on its own `"   "`-prefixed line (a `design.go` R22 copy
amendment), or have `deleteCandidatesManualCommand` join with `" && "` if a
single copy-pasteable line is the intent.

#### WR-05: the dummy backend still advertises the delete argv CR-02 removed as wrong

**File:** `internal/dummytui/fixturebackend.go:503-521`

**Issue:** `FixtureBackend.RotateDeleteOffer` was not updated by the fix pass:

```go
KeyID:         "1234567",
ManualCommand: "/usr/local/bin/gh ssh-key delete 1234567 --yes",
```

`gh ssh-key delete <id> --yes` is precisely the fixed, namespace-blind argv
CR-02 established as incorrect — and `ManualCommand` is copy shown to the user
as the thing to run by hand. The fixture also has no `KeyDetail`, so the
dummy demo (this project's design/screenshot source of truth) renders the
pre-CR-01 confirmation, which is now permanently divergent from the real pane.
`KeyID` is likewise no longer in the real backend's encoding, so the fixture
no longer models the contract it exists to demonstrate.

**Fix:** Regenerate the fixture from the real shapes:

```go
KeyID:         `[{"id":"1234567","registration":0},{"id":"7654321","registration":1}]`,
KeyDetail:     "ID 1234567, 7654321 — …AAAIDEMOKEY",
ManualCommand: "/usr/local/bin/gh api -X DELETE user/keys/1234567\n/usr/local/bin/gh api -X DELETE user/ssh_signing_keys/7654321",
```

(after WR-04 decides the multi-line rendering), and re-promote the affected
`rotate-delete-offer-*` frames.

#### WR-06: the key-ceremony overflow backstop silently discards content with no indicator and no way to scroll

**File:** `internal/tuikit/identities.go:2947-2972`

**Issue:** Two problems in the same block, both now load-bearing after CR-01
made the real pane overflow deterministically:

1. The clipped remainder is unmarked and unreachable — no "… N more lines"
   affordance, and the offer intercepts every key while it is unresolved, so
   the viewport can never be scrolled. Silent truncation of an upload
   *outcome* is the aggravating factor in CR-02 above.
2. When `budget <= 0` (a receipt with many written paths and backups, which
   the plan's own comment says is routine) the condition
   `budget > 0 && tailLines > budget` is false and the **entire** unclamped
   tail is appended, overflowing the fixed frame instead of being bounded —
   the opposite of the backstop's purpose.

**Fix:** Emit a truncation marker line and clamp unconditionally:

```go
if tailLines > budget {
    v := ExactTextViewport{Text: wrapped, VisibleLines: maxInt(1, budget), Width: maxInt(20, deleteChoiceNoteWidth-4)}
    return body + "\n" + v.Clamp().View() + "\n " + styleFaint.Render(fmt.Sprintf("… %d more line(s) hidden", tailLines-maxInt(1, budget))) + "\n"
}
```

and prefer clipping the *announce* lines explicitly (build the tail as two
parts) rather than relying on write order.

#### WR-07: reusing an existing key uploads a `.pub` path that may not exist

**File:** `cmd/gitid/upload_run.go:89-99`, `cmd/gitid/identity_create.go:401`

**Issue:** For the reuse path `uploadRequestFromSpec` falls through to
`pubPath := staged.FinalPubPath` (`PrivPEM == nil`), and `runCreateCeremony`
passes `staged.FinalPubPath` directly. `StageReuse` sets
`FinalPubPath = existingKeyPath + ".pub"` **without guaranteeing the file
exists** — `ensurePubReadOnly` explicitly supports the derive-from-private-key
case. When the user reuses a key that has no `.pub` on disk, `planUpload`'s
`ReadFile` fails and the user is shown a red
`✗ Authentication key registration failed: reading public key ~/...` for a
perfectly valid configuration, plus the manual-fallback block. The wizard
pre-checked the box and then produced a failure it caused itself.

**Fix:** In `uploadRequestFromSpec`, write the derived line to a staging temp
sibling whenever `FinalPubPath` is absent (not only when `PrivPEM != nil`),
using the staging dir — never `TempPrivatePath` when it equals
`FinalPrivatePath` (see CR-01). Same for the CLI create/clone call site.

#### WR-08: a dry run can print a registration FAILURE row for a write it silently swallowed

**File:** `cmd/gitid/identity_create.go:474-479`

**Issue:**

```go
_ = b.deps.WritePub(tempPub, staged.PubLine) //nolint:errcheck // … a write failure here just skips the upload preview
runUploadStep(cmd.OutOrStdout(), b, ...)
```

The comment is wrong: nothing skips. When the write fails (read-only staging
dir, ENOSPC), `runUploadStep` still runs, `planUpload`'s `ReadFile` fails, and
the dry run prints a `✗ … registration failed:` row and a manual-fallback
block — announcing a failed *registration* for a mode that performs no
registration at all. The error that actually happened (a local file write) is
never reported.

**Fix:** Honor the comment — `if werr := b.deps.WritePub(...); werr != nil { return nil }` (or
report the write error), and only call `runUploadStep` when the file exists.

#### WR-09: the upload checkbox's actionable copy is still unreadable at the only width production uses

**File:** `internal/tuikit/identities.go:4643-4679`, `internal/tuikit/design.go:582, 586`

**Issue:** WR-06's fix changed `ansi.Truncate(line, width, "")` to
`ansi.Truncate(line, width, "…")` and moved the tests to width 60. That makes
the truncation *visible*; it does not make the row *useful*. At the wizard's
literal `60`, the unauth label still renders as roughly
`☐ Register with GitHub automatically — not logged in to git…`, dropping the
entire remediation (`run "gh auth login" first, or check anyway`), and the
disabled label loses `Manual steps are shown after create.` The rendered
result now asserts only that a marker is present
(`upload_section_test.go:391-408`), i.e. the tests bless the truncation. The
fix report defers the real fix ("would need a design.go R22 amendment") with
no tracking item filed anywhere in `.planning/`.

**Fix:** Amend the frozen copy to fit 60 columns (R22 amendment) — e.g.
`"Register with %s — not logged in; run \"%s auth login\""` and
`"Auto-registration unavailable — no %s CLI here"` — or file the amendment as
an explicit ROADMAP/backlog item rather than a comment.

#### WR-10: the `glab` inventory is still unpaginated while every decision treats it as complete

**File:** `internal/uploader/inventory.go:47-54`

**Issue:** WR-01 was applied to `gh` only. The comment is honest about it, but
the consequence is unchanged for GitLab users: `glab ssh-key list -F json`
returns one default page, and `MissingRegistrations` (D-16), the D-17
confirmation, and D-04's `OldKeyCandidates` all read that list as exhaustive.
For GitLab this is worse than for GitHub, because `RegistrationCombined` means
a truncated list makes gitid re-upload a key the account already has and
then classify GitLab's `already taken` response as
`FailureCrossAccountConflict` — a user-visible claim that the key belongs to
*another account*, which is false.

**Fix:** Verify the flag once against a real `glab` (`glab ssh-key list --help`)
and thread it, or — since `decodeProviderKeyPages` already handles multiple
documents — iterate `--page N` until a short/empty page. Failing that, detect
a full page and set `degraded = true` so the D-15 "inventory unavailable"
notice fires instead of a wrong answer.

#### WR-11: provider JSON is parsed out of `CombinedOutput`, so any stderr line breaks the inventory

**File:** `cmd/gitid/wiring.go:560-577`, `internal/uploader/inventory.go:60-74`

**Issue:** `RunCmd` returns `cmd.CombinedOutput()`, and `inventoryFor` feeds
that string straight into `json.Decoder`. Any diagnostic the CLI writes to
stderr on a *successful* call (gh's deprecation/redirect warnings, glab's
`Using host …` banner, a proxy notice) is spliced into the JSON stream, the
decode fails, and `Inventory` returns an error — which the callers correctly
treat as non-gating degradation, so the user instead gets "could not read the
inventory", duplicate upload attempts, and (in D-04) a refused offer. The
failure is silent and version-dependent, and no fixture can reproduce it
because the fake shims only write to stdout.

**Fix:** Give `Deps` a separated-streams variant for parsing calls (return
stdout and stderr distinctly, parse stdout, keep stderr for error text only).
At minimum, skip leading non-`[`/`{` lines before decoding and include the
discarded text in the degradation reason.

#### WR-12: the wizard still registers a key remotely before the identity is committed, with no way back (WR-16, still open)

**File:** `internal/tuikit/identities.go:3791-3803`, `cmd/gitid/upload_run.go:83-100`

**Issue:** Restated because the code still has the defect: the D-05 upload
beat runs at wizard step 1, the checkbox is pre-checked by default whenever
eligibility resolves `Ready` (`identities.go:2367`), and abandoning the wizard
afterwards leaves a public key registered on the user's account under
`gitid: <name> @ <host>` whose private half `deps.Cleanup` has destroyed.
gitid offers no removal path (D-04 is rotate-only). The fixer's deferral
rationale is reasonable — the "at minimum" option needs a fresh inventory read
and new async plumbing — but the deferral is recorded only in
`09-REVIEW-FIX.md`, not as a plan or ROADMAP item, so nothing will surface it
again.

**Fix:** File the follow-up plan the fix report recommends, and in the
meantime add the cheap mitigation: when the wizard is abandoned after
`uploadRun` reported at least one `UploadRowUploaded`, emit a note carrying
the D-07 title so the user can find and remove the key by hand.

### Info

#### IN-01: doc comments detached from the functions they describe (unfixed)

**File:** `internal/tuikit/identities.go:4543-4556`, `internal/tuikit/upload_section_test.go:334-337`

The `renderUploadCheckboxRow` and `uploadRunHasContent` doc comments still sit
immediately above `renderRegisterKey` (line 4557), so godoc attributes both to
the wrong symbol; the real declarations are at 4643 and 4585.
**Fix:** move each comment above its own declaration.

#### IN-02: `uploadFailureView`'s `provider` parameter is always a hostname, sometimes empty (unfixed)

**File:** `cmd/gitid/wiring.go:1478-1485`, `cmd/gitid/wiring.go:1579`

`RunUploadForIdentity`'s not-found branch still passes `""`, rendering
`"Upload your public key to  as both an authentication key and a signing
key,"`. **Fix:** rename the parameter to `hostname` and skip the manual block
when it is empty.

#### IN-03: `fmt.Fprintln(w, fmt.Sprintf(...))` instead of `fmt.Fprintf` (unfixed)

**File:** `cmd/gitid/upload_run.go:258-287`, `cmd/gitid/identity_upload.go:117-119, 181-183`

Ten call sites now (WR-02's fix added two). **Fix:** `fmt.Fprintf(w, fmt+"\n", …)`.

#### IN-04: frame-promote's header documents `PROVENANCE.md`; the tool writes `README.md` (unfixed), and the registry count disagrees with its own comment

**File:** `cmd/gitid-frame-promote/main.go:13`, `:45-63`, `:92-95`

The package comment promises `ui-frames/PROVENANCE.md` while `run()` writes
`README.md` (and the error string still says "writing PROVENANCE table"). The
registry comment says "twelve new PTY tests plus the two pre-existing … tracer
tests" (14) for a 13-row table. **Fix:** align both.

#### IN-05: UTF-8-unsafe byte slice of a frozen format string in a test assertion (unfixed)

**File:** `cmd/gitid/identity_upload_test.go:349`

`tuikit.UploadResultFailedFmt[:5]` slices `"✗ %s key …"` at byte 5, yielding
`"✗ %"`, which never appears in rendered output; the assertion passes only via
the `||` fallback. **Fix:** assert on the rendered substring.

#### IN-06: e2e children inherit the ambient environment, including real provider tokens (unfixed)

**File:** `e2e/harness_test.go:531-536`

`e2eEnv` still builds `append(os.Environ(), …)`, so `GH_TOKEN`,
`GITHUB_TOKEN`, `GH_CONFIG_DIR`, `GLAB_*` reach every child; hermeticity rests
entirely on PATH ordering. **Fix:** blank provider-credential variables inside
`e2eEnv`.

#### IN-07: `u` is consumed on step 0 even when the checkbox row is not rendered (unfixed)

**File:** `internal/tuikit/identities.go:3623`

The hotkey branch still checks only the focus slot, not `w.uploadRowVisible()`,
so on a non-gated host `u` is swallowed with no effect. **Fix:** add
`&& w.uploadRowVisible()`.

#### IN-08: `TestRegisterKeyExitContract` documents six cases, defines five (unfixed)

**File:** `cmd/gitid/identity_upload_test.go:201-223`

The comment still says "six canned views … a nil error for the first five";
the table has five rows and the dry-run case named in the R10 exit contract is
still absent. **Fix:** add the row or correct the comment.

#### IN-09: CR-01's fix left `FindByTitle` dead in production, and the doc comments still describe the pre-fix matching

**File:** `internal/uploader/inventory.go:195-206`, `cmd/gitid/wiring.go:1595-1598`,
`e2e/harness_test.go:854`, `internal/uploader/uploader.go:271-277`

`FindByTitle` now has no production caller (only `inventory_test.go` and stale
comments); `titleMatchesThisMachine` is likewise kept alive only by its own
test. `RotateDeleteOffer`'s doc comment still says matching is "by EXACT title
equality … (uploader.KeyTitle + uploader.FindByTitle)", which is exactly the
behavior CR-01 replaced — the most misleading kind of stale comment, since it
describes a defect as the design. **Fix:** update the comment to name
`OldKeyCandidates`, and delete or keep-with-justification the two dead helpers
(WR-14 removed `Detect`/`TrimOutput` for the same reason).

#### IN-10: the dry-run print block is duplicated between the two CLI entry points

**File:** `cmd/gitid/identity_upload.go:110-121` and `:174-185`

`runIdentityRegisterKey` and `runUploadStep` contain byte-identical
plan-print-and-note blocks. WR-10's fix hardened the test for one of them
only; the other can still drift. **Fix:** extract
`printDryRunPlan(w, plan)` and call it from both.

---

_Reviewed: 2026-08-29T21:05:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard — iteration 2 (re-review after 09-REVIEW-FIX.md)_
