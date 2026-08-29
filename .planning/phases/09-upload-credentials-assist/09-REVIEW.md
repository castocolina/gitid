---
phase: 09-upload-credentials-assist
reviewed: 2026-08-29T18:04:33Z
depth: standard
files_reviewed: 49
files_reviewed_list:
  - cmd/gitid-frame-promote/main.go
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
  warning: 18
  info: 8
  total: 28
status: issues_found
---

# Phase 9: Code Review Report

**Reviewed:** 2026-08-29T18:04:33Z
**Depth:** standard
**Files Reviewed:** 49
**Status:** issues_found

## Summary

Phase 9 adds autonomous `gh`/`glab` SSH-key registration across the create
wizard, the key-ceremony (rotate/repair), a new register-key TUI pane, and
four CLI write verbs, plus D-04's interactive old-key delete offer.

The subprocess boundary itself is solid: every provider invocation goes
through `uploader.Deps.RunCmd`, which builds `exec.CommandContext` from an
explicit arg slice with no shell, bounded by a 20 s timeout; `DeleteKey`
rejects any non-numeric provider ID before argv construction;
`requirePublicKey` validates the `.pub` by *content* (rejects PEM private
material, requires `ssh.ParseAuthorizedKey`) rather than by suffix alone;
and the e2e harness's PATH deny-shim boundary with an AST guard over every
`cmd.Env` assignment is genuinely good hermeticity work. I found **no
command-injection, path-traversal, or shell-interpolation defect**.

The defects are in the *decision* layer, and the two Critical ones are in
D-04 — the one remotely-destructive path this phase adds:

1. The delete offer resolves its target purely by the D-07 title, and the
   D-07 title is identical for the OLD key and the NEW key that the same
   ceremony uploaded seconds earlier. Nothing in the offer, the matcher, or
   the rendered confirmation distinguishes them.
2. The delete argv (`gh ssh-key delete <id>`) is issued against IDs that
   `Inventory` merges from two *different* GitHub ID namespaces
   (`/user/keys` and `/user/ssh_signing_keys`), and the record's
   `Registration` field — which `DeleteRecordedKey` exists to carry — is
   discarded before the call.

Beyond those, the recurring theme is **state that is classified but never
consumed, and copy that is frozen but never rendered as written**:
`FailureNotAuthenticated` is computed and dropped; `UploadRunView.Skipped`
is overloaded so a self-hosted create prints `"Auto-upload skipped
(--no-upload)."` when no such flag was passed (and a unit test blesses it);
the checkbox's frozen unauth/disabled labels are hard-truncated at a
hardcoded width of 60 while their legibility tests run at width 200. Test
quality is uneven: `TestRegisterKeyDryRunExecutesNoUpload` re-implements the
production dry-run branch inside the test body and asserts on its own
writes, and the R3 AST guard the phase leans on only scans `wiring.go`.

## Critical Issues

### CR-01: D-04 delete offer can target the key that was just uploaded — old and new keys share an identical title

**File:** `cmd/gitid/upload_run.go:398-402`, `internal/uploader/uploader.go:265-274`, `internal/tuikit/identities.go:2269-2291`

**Issue:** The rotate ceremony chains, in order: `CommitRotate` (new key
generated + committed) → `RunUploadForIdentity` (the NEW key is registered
with the provider) → `RotateDeleteOffer`. `rotateDeleteOfferFor` then reads a
fresh inventory and picks the deletion target with:

```go
title := uploader.KeyTitle(name, shortHostname())
found, ok := uploader.FindByTitle(existing, title)
```

`KeyTitle` is `fmt.Sprintf("gitid: %s @ %s", identityName, machine)` — it has
**no key-specific component**. The new key was uploaded under exactly that
same title moments earlier by `planUpload` (`upload_run.go:162`). So at the
moment the offer resolves, the provider inventory contains at least two
records with the identical title, and `FindByTitle` returns whichever the
provider happened to list first:

```go
func FindByTitle(existing []ExistingKey, title string) (ExistingKey, bool) {
	for _, key := range existing {
		if key.Title == title {   // first match wins; no tie-break
```

Nothing enforces "oldest first" — that is an unstated dependency on
`gh api user/keys` / `glab ssh-key list` ordering. The problem compounds on a
machine that has rotated more than once (every prior rotation's leftover
carries the same title too).

The confirmation UI cannot save the user either: `renderRotateDeleteOffer`
(`identities.go:2965-2993`) renders only `RotateDeleteOfferBodyFmt`
interpolated with `IdentityName`/`MachineName` — i.e. the same ambiguous
title. `KeyID` is present on the view and never displayed. The user is asked
to confirm deleting a key identified solely by a string that at least two
keys share. If the wrong one is picked, gitid deletes the key it *just*
registered, and the identity it reports as rotated cannot authenticate.

**Fix:** Never match on title alone for a destructive action. The current
key's blob is already in hand (`plan.pubLine` / the account's `.pub`), and
`NormalizeKeyBlob` already exists — exclude it, and surface the ID:

```go
// in rotateDeleteOfferFor, after reading `existing`
currentPub, rerr := b.uploaderDeps.ReadFile(expandTildeForHome(acct.PubPath, b.home))
if rerr != nil {
	return tuikit.RotateDeleteOfferView{Unavailable: "could not read the current public key"}
}
currentBlob := uploader.NormalizeKeyBlob(string(currentPub))

var candidates []uploader.ExistingKey
for _, rec := range existing {
	if rec.Title != title {
		continue
	}
	if uploader.NormalizeKeyBlob(rec.Key) == currentBlob {
		continue // this is the key we just registered — never offer it
	}
	candidates = append(candidates, rec)
}
if len(candidates) != 1 {
	// 0 -> nothing to remove; >1 -> ambiguous, do not guess
	return tuikit.RotateDeleteOfferView{Unavailable: "no unambiguous old key found on this machine"}
}
```

and add the ID plus a key-blob suffix to the rendered offer so the reviewed
target is identifiable, satisfying D-04's "the user reviewed this exact key".

---

### CR-02: the delete call ignores the inventory record's registration type — GitHub auth and signing key IDs are different namespaces

**File:** `internal/uploader/inventory.go:27-44`, `internal/uploader/inventory.go:99-138`, `cmd/gitid/upload_run.go:399-411`, `cmd/gitid/wiring.go:1570`

**Issue:** `Inventory` for `ToolGH` concatenates two independent API
collections into one slice:

```go
auth, _ := inventoryFor(tool, toolPath, deps, []string{"api", "user/keys"}, RegistrationAuthentication)
signing, _ := inventoryFor(tool, toolPath, deps, []string{"api", "user/ssh_signing_keys"}, RegistrationSigning)
return append(auth, signing...), nil
```

`/user/keys/{id}` and `/user/ssh_signing_keys/{id}` are separate GitHub
resources with **independent, freely-colliding integer ID spaces**. The
`Registration` field records which namespace each `ID` came from — and then
every consumer throws it away:

- `FindByTitle` does not filter on `Registration`.
- `DeleteRecordedKey` (which takes the whole `ExistingKey`, i.e. the only
  API that *could* dispatch per namespace) is dead code — nothing calls it.
- `CommitRotateDeleteOldKey` calls the low-level `uploader.DeleteKey(tool,
  toolPath, keyID, ...)`, and `deleteArgs` emits the fixed
  `gh ssh-key delete <id> --yes`, which addresses the authentication-key
  namespace.

Two consequences, both reachable:

1. **Wrong-key deletion.** If the title match lands on a *signing* record
   (e.g. the identity's authentication registration was removed or never
   succeeded, so the signing entry is the first title match), gitid runs
   `gh ssh-key delete <signing-key-id> --yes`. That ID is interpreted in the
   authentication namespace and can name a completely unrelated
   authentication key on the user's account.
2. **False success claim.** Even in the happy path, a rotated GitHub key has
   *two* registrations (that is the entire premise of `recipes/README.md` and
   `upload.Instructions`). At most one is removed, yet the UI prints
   `RotateDeleteOfferResultRemovedFmt` — `"✓ Old key removed from GitHub."` —
   which is untrue: the old key remains registered for signing and the user
   is told the cleanup is complete.

**Fix:** Make the registration type load-bearing all the way to the argv, and
delete every registration of the old key:

```go
// inventory.go
func deleteArgs(tool Tool, reg Registration, id string) ([]string, error) {
	switch tool {
	case ToolGH:
		switch reg {
		case RegistrationAuthentication:
			return []string{"api", "-X", "DELETE", "user/keys/" + id}, nil
		case RegistrationSigning:
			return []string{"api", "-X", "DELETE", "user/ssh_signing_keys/" + id}, nil
		}
	case ToolGLab:
		return []string{"ssh-key", "delete", id}, nil
	}
	return nil, fmt.Errorf("uploader: unsupported delete for %s/%d", toolName(tool), reg)
}
```

and have `rotateDeleteOfferFor` return the full set of matching records
(post-CR-01 filtering) so `CommitRotateDeleteOldKey` removes both, or — if
one-at-a-time is preferred — scope the result copy to the registration
actually deleted instead of claiming the key is gone.

## Warnings

### WR-01: provider inventory reads are unpaginated — silently truncated at 30 keys

**File:** `internal/uploader/inventory.go:30-40`

**Issue:** `gh api user/keys` and `gh api user/ssh_signing_keys` return the
REST default page size (30) with no `--paginate`; `glab ssh-key list -F json`
paginates similarly. Every D-15/D-16/D-17/D-04 decision reads this list as if
it were complete. On an account with more than 30 keys: the dedupe diff
reports registrations as missing that already exist, the D-17 confirmation
never converges and marks a genuinely-successful upload "unconfirmed", and
the D-04 offer reports "no matching old key found on this machine" for a key
that is right there. The fake `gh` shim always returns a short fixture, so no
test can catch it.

**Fix:**

```go
auth, err := inventoryFor(tool, toolPath, deps,
	[]string{"api", "--paginate", "user/keys"}, RegistrationAuthentication)
```

(`gh api --paginate` concatenates JSON arrays; either pass `--slurp`/`--jq`
or unmarshal the concatenated arrays explicitly.) For `glab`, add
`--per-page`/page iteration.

### WR-02: `printUploadOutcome` prints the `--no-upload` note when `--no-upload` was never passed

**File:** `cmd/gitid/upload_run.go:133-141`, `cmd/gitid/upload_run.go:248-253`

**Issue:** `UploadRunView.Skipped` is set by `planUpload` for two *derived*
terminal states — provider not gated (`{Skipped: true}`) and matching CLI not
on PATH (`{Skipped: true, ManualFallback: ...}`) — while the actual
`--no-upload` opt-out never builds a view at all (`identity_upload.go:165-169`
prints the note directly). `printUploadOutcome` then treats the flag as the
only meaning of the field:

```go
if view.Skipped {
	fmt.Fprintln(w, tuikit.UploadSkippedByFlagNote) // "Auto-upload skipped (--no-upload)."
}
```

So every `gitid identity create` against a self-hosted GHE / self-managed
GitLab host (D-13's explicitly supported degrade path) tells the user they
passed a flag they did not pass. `printUploadOutcome`'s own doc comment
compounds it: "It writes nothing at all for the omitted state (view is the
zero value)" — the omitted state is `{Skipped: true}`, not the zero value.
`TestPrintUploadOutcomeRendersEachSection` asserts the zero value writes
nothing (a state production never produces) and separately asserts the
`{Skipped: true, ManualFallback: ...}` shape *should* print the `--no-upload`
note — so the bug is currently enshrined as expected behavior
(`upload_run_test.go:129-138`).

**Fix:** Split the field, e.g. add `SkippedByFlag bool` (set only by
`runUploadStep`'s `noUpload` branch) and leave `Skipped` meaning "autonomy did
not apply here"; print the flag note only for the former, and update
`TestPrintUploadOutcomeRendersEachSection` to cover both real shapes.

### WR-03: `FailureNotAuthenticated` is classified and then discarded

**File:** `internal/uploader/classify.go:54-56`, `cmd/gitid/wiring.go:1401-1414`

**Issue:** `ClassifyUploadFailure` returns `FailureNotAuthenticated` for
"not logged in"/"not authenticated" output, but `toUploadResultRow`'s switch
only handles `FailureScopeAuth`, `FailureScopeSigning`, and
`FailureCrossAccountConflict`; everything else falls to
`RedactCLIOutput(raw, ...)`. `grep` confirms no other consumer exists and no
test asserts the kind. This is precisely the D-01 scenario-2 path the UI
invites the user into ("or check anyway") — the user gets a raw, truncated
CLI line instead of the "run `gh auth login`" guidance the phase designed.

**Fix:** Add a frozen remediation constant and map the kind:

```go
case uploader.FailureNotAuthenticated:
	row.Reason = fmt.Sprintf(tuikit.UploadNotAuthenticatedFmt, providerToolName(provider), providerHost)
```

plus a `classify_test.go` case for both providers.

### WR-04: `RedactCLIOutput` does not match GitHub fine-grained PATs

**File:** `internal/uploader/classify.go:27-28`

**Issue:** `ghTokenPattern = \bgh[pousr]_[A-Za-z0-9]{20,}\b` covers `ghp_`,
`gho_`, `ghu_`, `ghs_`, `ghr_` but **not** `github_pat_...` (fine-grained
personal access tokens — `gh` after `gh` is `i`, which is not in `[pousr]`).
The function's own doc calls itself "defence in depth ... rather than an
assumption that current upload commands never echo tokens"; the most common
modern token shape slips through into a user-visible reason row.

**Fix:**

```go
ghTokenPattern = regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})\b`)
```

and add a `github_pat_` case to `TestRedactCLIOutputRemovesTokenShapes`.

### WR-05: raw error strings bypass the redaction path

**File:** `cmd/gitid/upload_run.go:143-147`, `cmd/gitid/wiring.go:1457-1460`

**Issue:** Recovered panics and classified CLI output are routed through
`RedactCLIOutput(..., b.home, 58)`, but two adjacent paths hand the raw
`err.Error()` straight to `uploadFailureView`: `planUpload`'s `ReadFile`
failure and `RunUpload`'s `uploadRequestFromSpec` failure. Those errors
embed absolute paths (`/Users/<user>/.ssh/...`, the staging temp dir) — the
exact home-path leakage the redactor exists to prevent, and unbounded in
length inside a fixed 100x30 frame.

**Fix:** `view := uploadFailureView(uploader.RedactCLIOutput(err.Error(), b.home, 58), req.Hostname)` at both sites.

### WR-06: the D-01 checkbox row is hard-truncated at a hardcoded width of 60, and its legibility tests run at 200

**File:** `internal/tuikit/identities.go:4640-4647`, `internal/tuikit/upload_section_test.go:350,377`

**Issue:** `renderUploadCheckboxRow` ends with `ansi.Truncate(line, width, "")`
— no ellipsis — and the wizard's only caller passes a literal `60`. The
frozen labels are far longer:

- unauth: `"Register with %s automatically — not logged in to %s; run \"%s auth login\" first, or check anyway"` ≈ 110 columns
- disabled: `"Auto-registration unavailable — %s has no gh/glab match here. Manual steps are shown after create."` ≈ 103 columns

So in production both states render as a mid-word cut that drops the entire
actionable half of the sentence, with no visual cue that anything was cut.
The two tests that would catch this
(`TestUploadCheckboxRendersAllFourStates`,
`TestUploadCheckboxIsLegibleWithoutColor`) call the renderer with `width=200`,
a width no caller uses; `TestUploadCheckboxRowIsExactlyOnePhysicalLine` uses
`60` but only asserts the row *fits*, never that its meaning survives.

**Fix:** Either shorten the frozen unauth/disabled copy to fit the real pane
width (an approved amendment per `design.go`'s R22 rule), or truncate with an
explicit `"…"` marker and assert at the production width:

```go
got := renderUploadCheckboxRow(view, false, false, 60)
if !strings.Contains(stripANSI(got), "auth login") { t.Error(...) }
```

### WR-07: the rotate delete-offer traps keyboard input — Esc cannot leave the pane

**File:** `internal/tuikit/identities.go:2768-2799`

**Issue:** While `rotateDeleteOffer.Available && !rotateDeleteResolved`, the
handler intercepts *all* keys before the shared `ceremonyModel` and only acts
on arrows/tab and `enter`; every other key (including `esc`) hits the trailing
`return keyResult{model: m, handled: true}` and is swallowed. The rotate has
already been committed at this point, so a user who does not want to answer
the destructive question has no way out of the pane — which contradicts the
phase's own "upload/offer never gates" non-negotiable (a soft gate is still a
gate) and makes the failure mode of a hung/incorrect offer a stuck screen.

**Fix:** Treat `esc` as the non-destructive "leave it" answer:

```go
case "esc":
	m.rotateDeleteResult = fmt.Sprintf(RotateDeleteOfferResultLeftFmt, m.rotateDeleteOffer.ManualCommand)
	m.rotateDeleteResolved = true
	return keyResult{model: m, handled: true}
```

### WR-08: rotate/new-key `--dry-run` skips the upload preview for tilde-form identities

**File:** `cmd/gitid/identity_key.go:217`

**Issue:**

```go
if fileExists(acct.PubPath) {
	runUploadStep(w, b, uploadRequestForAccount(acct, b.home), noUpload, true)
}
```

`uploadRequestForAccount` exists *because* `identity.Account.PubPath` is the
display form and "may carry a literal `~/` prefix" (its own doc comment,
`upload_run.go:61-67`) — recipe-shaped `IdentityFile ~/.ssh/id_...` entries
produce exactly that. The guard checks the **unexpanded** path, so
`fileExists("~/.ssh/id_ed25519_work.pub")` is false and the dry-run upload
preview is silently omitted for precisely the config shape `recipes/` defines
as canonical. This is the same tilde-expansion class already fixed once in
this repo (`da03009 fix(identity): expand tilde before reading identity Git
fragments`).

**Fix:**

```go
req := uploadRequestForAccount(acct, b.home)
if fileExists(req.PubPath) {
	runUploadStep(w, b, req, noUpload, true)
}
```

### WR-09: the R3 boundary guard only scans `wiring.go`

**File:** `cmd/gitid/upload_run_test.go:24-48`, `cmd/gitid/wiring_test.go:5518-5545`

**Issue:** Both AST guards hard-code `assertNoGuardedCalls("wiring.go")` /
`os.ReadFile("wiring.go")`. The invariant they are meant to protect is
"`uploader.Inventory`/`UploadKeys`/`MissingRegistrations` are called only from
`upload_run.go`", but any *other* file in `package main` (e.g.
`identity_upload.go`, or a new file added next phase) can call them with both
guards staying green. `internal/tuikit` is protected structurally by the
no-backend-import rule; `cmd/gitid` is not.

**Fix:** Walk the package instead of one filename:

```go
entries, _ := filepath.Glob("*.go")
for _, f := range entries {
	if f == "upload_run.go" || strings.HasSuffix(f, "_test.go") {
		continue
	}
	assertNoGuardedCalls(f)
}
```

### WR-10: `TestRegisterKeyDryRunExecutesNoUpload` does not exercise the dry-run path

**File:** `cmd/gitid/identity_upload_test.go:102-147`

**Issue:** The test never calls `runIdentityRegisterKey` with
`identityRegisterKeyFlags{DryRun: true}`. It calls `b.planUpload(req)` and
then **re-implements the production branch inside the test body**:

```go
for _, c := range plan.commands {
	out.WriteString("Running: " + c + "\n")
}
out.WriteString(tuikit.UploadDryRunNote + "\n")
...
if !strings.Contains(out.String(), tuikit.UploadDryRunNote) {   // asserts its own write
```

It asserts on strings it wrote itself, and the hand-rolled `"Running: "` is
not even `UploadRunningLineFmt`. Deleting the entire `if flags.DryRun` block
from `identity_upload.go:110-121` leaves this test green — the exact
regression it is named for goes undetected.

**Fix:** Drive the real entry point and assert on `cmd.OutOrStdout()`:

```go
err := runIdentityRegisterKey(cmd, []string{"acme"}, identityRegisterKeyFlags{DryRun: true}, false, false)
// then assert the note is present and `recorded` holds no "ssh-key add"
```

### WR-11: `confirmUpload` couples `wanted[i]` to `rows[i]` positionally and fails silently

**File:** `cmd/gitid/upload_run.go:307-316`

**Issue:**

```go
for i, registration := range wanted {
	if i >= len(rows) { continue }
	if rows[i].Outcome == ... { toConfirm[registration] = i }
}
```

The mapping is valid only because `executeUpload` happens to append exactly
one row per `plan.wanted` entry in order — an invariant that is nowhere
asserted and is easy to break (any future early-`continue` in the row loop, a
per-type filter, or a `wanted` list with an entry `UploadKeys` does not answer
for). When it breaks, the `i >= len(rows)` guard silently *skips* and the
`toConfirm[registration] = i` entries silently mis-attribute: the D-17
confirmation would then check the wrong registration's presence and stamp the
"accepted but not yet visible" reason on the wrong row.

**Fix:** Key the lookup on the registration instead of the index:

```go
for i := range rows {
	reg := registrationOf(rows[i]) // rows already carry Registration
	if rows[i].Outcome == tuikit.UploadRowUploaded || rows[i].Outcome == tuikit.UploadRowAlreadyPresent {
		toConfirm[reg] = i
	}
}
```

(`UploadResultRow.Registration` is already populated by `toUploadResultRow`.)

### WR-12: `extractUploadSection`'s marker list contradicts its own documented exclusion

**File:** `internal/screenshot/createflow_regions.go` (`extractUploadSection`, marker list)

**Issue:** The function's comment states it deliberately excludes
`"Register with "` because that is the D-01 checkbox's own step-0 label and
anchoring on it "would make RegionUploadSection collide with every
pre-existing step-0 spec". The marker list then includes `"not logged in to"`
— which is a substring of that very label
(`UploadCheckboxLabelUnauthFmt = "Register with %s automatically — not logged
in to %s; ..."`), and survives the width-60 truncation. Any step-0 screen in
the unauth state (the dummy fixture's GitLab host, every deny-shim PTY
session) therefore anchors `RegionUploadSection` on the ordinary form body —
the collision the comment claims to have avoided, producing region content
for screens that have no upload beat.

**Fix:** Drop `"not logged in to"` from `uploadMarkers` (the unauth checkbox
is covered by the step-0 form region), or anchor it on a marker unique to the
beat, e.g. `UploadManualHeading`.

### WR-13: `gitid-frame-promote` overwrites tracked baselines before validating the capture set

**File:** `cmd/gitid-frame-promote/main.go:92-113`

**Issue:** The loop writes each found frame into the tracked
`.planning/.../ui-frames/` directory as it goes, collecting misses in
`missing`, and only *after* the loop returns an error. So a partial capture
run (e.g. one PTY test skipped) rewrites some approved baselines, leaves the
rest at their previous commit's content, and never regenerates
`README.md` — producing a baseline directory whose frames and provenance
table disagree, which is exactly the "a frame from a DIFFERENT run mistaken
for the reviewed one" hazard the tool's header says it prevents.

**Fix:** Read and validate every source first, then write:

```go
contents := make(map[string][]byte, len(entries))
for _, e := range entries {
	data, err := os.ReadFile(filepath.Join(srcDir, e.frame+".txt"))
	if err != nil { missing = append(missing, e.frame); continue }
	contents[e.frame] = data
}
if len(missing) > 0 { return fmt.Errorf(...) }  // nothing written yet
```

### WR-14: dead exported API in `internal/uploader`, including the D-11-violating `Detect`

**File:** `internal/uploader/uploader.go:154-168`, `internal/uploader/uploader.go:276-279,319,324`, `internal/uploader/inventory.go:114-118`

**Issue:** `Detect` — the first-found router that 09-CONTEXT.md D-11 identifies
as "a correctness bug" that "sends GitLab identities to gh and lets an
unauthenticated gh shadow an authenticated glab" — is still exported, still
covered by six passing tests (`uploader_test.go:70-196`), and called by no
production code. A green test suite around a function the design forbids is an
active invitation to reuse it. Also dead outside tests:
`DeleteRecordedKey`, `TitleMatchesThisMachine`, `TrimOutput`, `ToolName`,
and `UploadKey` (used only inside the package). `DeleteRecordedKey`'s death is
load-bearing — see CR-02.

**Fix:** Delete `Detect` and its tests (`DetectFor` is the replacement); either
wire `DeleteRecordedKey` (CR-02) or remove it; unexport or drop the rest.

### WR-15: `DetectFor` always reports `AuthNotLoggedIn` for a tool it found

**File:** `internal/uploader/uploader.go:171-181`

**Issue:** The function returns an `AuthStatus` but never probes auth:

```go
p, err := deps.LookPath(name)
if err != nil { return 0, "", AuthToolNotFound }
return toolForName(name), p, AuthNotLoggedIn   // always
```

Every caller must know to ignore the third value and call `AuthCheck`
separately (`wiring.go:1345`, `wiring.go:1500`, `upload_run.go:391`), while
`planUpload` (`upload_run.go:137-141`) checks only `== AuthToolNotFound`. A
caller that reasonably trusts the returned status would conclude the user is
never authenticated. This is a signature that lies.

**Fix:** Return `(tool, toolPath string, found bool)`, or rename the third
result to make the "unresolved" semantics explicit
(`AuthUnknown`/`AuthNotProbed`) and add it to the `AuthStatus` enum.

### WR-16: the wizard registers a key remotely before the identity is committed, with no cleanup path

**File:** `internal/tuikit/identities.go:3766-3781`, `cmd/gitid/upload_run.go:81-98`

**Issue:** D-05 places the upload beat inside wizard step 1, before the
`ssh -T` gate and long before step 3's write ceremony. `uploadRequestFromSpec`
stages a temp `.pub` from the not-yet-persisted key and uploads it. If the
user then abandons the wizard (Esc at any later step, or the write ceremony
fails and rolls back), `deps.Cleanup` removes the staging directory — but the
public key is already registered on the user's GitHub/GitLab account under
`gitid: <name> @ <host>`, with the matching private key destroyed. gitid
offers no way to remove it (the D-04 offer is rotate-only), and the D-01
checkbox is *pre-checked by default* in the Ready state, so this is the
default path, not an opt-in edge case. It also sits uneasily against
CLAUDE.md's "never mutate without explicit confirmation + a way back".

**Fix:** At minimum, tell the user: when the wizard is abandoned after
`uploadRun` reported an upload, surface the exact
`uploader.DeleteCommandPreview` line for the key that was registered. Better:
move the beat after the commit ceremony for the generate path (the
`ReachableNotUploaded → PASS` gate that D-05 cites already tolerates an
unregistered key on the first probe).

### WR-17: the eligibility memo mutex is held across a subprocess call of up to 20 s

**File:** `cmd/gitid/wiring.go` (`UploadEligibility`, memo lock through `AuthCheck`)

**Issue:** `b.uploadEligibilityMemoMu.Lock(); defer ...Unlock()` wraps the
`DetectFor` + `AuthCheck` calls, each of which shells out through
`RunCmd`'s 20 s `providerCommandTimeout`. One global mutex covers *all*
providers, so a slow/hung `gh auth status` blocks an unrelated `glab` probe
for the full timeout. The 09-04 fix that introduced this (making the
check-then-set atomic) is correct in intent, but the scope is wider than
needed.

**Fix:** Use a per-provider lock (`map[string]*sync.Mutex` guarded by the
outer mutex) or a `golang.org/x/sync/singleflight`-style keyed barrier, so the
critical section is per provider rather than global.

### WR-18: `CommandPreview` produces a command line that cannot be copied and re-run

**File:** `internal/uploader/uploader.go:282-288`

**Issue:** `strings.Join(append([]string{toolPath}, args...), " ")` performs no
shell quoting, and the D-07 title *always* contains spaces. The announced line
is therefore:

```
/usr/local/bin/gh ssh-key add /path/key.pub --title gitid: acme @ mbp --type authentication
```

Pasting that into a shell runs a different command (`--title gitid:` with three
stray operands). D-02's "announce exactly as executed" holds structurally for
the argv, but the manual re-run affordance the phase repeatedly points users
at ("then retry from the Identity Manager", the manual-fallback block) is
broken for the one string users are most likely to copy.

**Fix:** Quote arguments that need it in the preview only (the executed argv
stays a slice):

```go
func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"'\\$`") { return s }
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

## Info

### IN-01: doc comments detached from the functions they describe

**File:** `internal/tuikit/identities.go:4518-4531`, `internal/tuikit/upload_section_test.go:334-337`

**Issue:** The `renderUploadCheckboxRow` and `uploadRunHasContent` doc
comments both sit immediately above `renderRegisterKey` (with no blank line
separating them from it), so godoc attributes them to the wrong symbol —
`renderUploadCheckboxRow` is actually defined 100 lines later at 4618.
Likewise, `TestStaleUploadEligibilityMsgIsDiscarded`'s doc comment sits above
`TestUploadCheckboxRendersAllFourStates`, while the test it describes is at
line 529.

**Fix:** Move each comment to sit directly above its own declaration.

### IN-02: `uploadFailureView`'s `provider` parameter is always given a hostname, sometimes empty

**File:** `cmd/gitid/wiring.go:1419-1426`, `cmd/gitid/wiring.go:1520`

**Issue:** The parameter is named `provider` but every call site passes a
hostname (`spec.Hostname`, `req.Hostname`, `acct.Hostname`); it works only
because `upload.Instructions` substring-matches. `RunUploadForIdentity`'s
not-found branch passes `""`, which renders the default branch as
`"Upload your public key to  as both an authentication key and a signing
key,"` — a sentence with a hole in it.

**Fix:** Rename the parameter to `hostname`, and skip the manual-fallback
block entirely when the hostname is unknown.

### IN-03: `fmt.Fprintln(w, fmt.Sprintf(...))` instead of `fmt.Fprintf`

**File:** `cmd/gitid/upload_run.go:250-271`, `cmd/gitid/identity_upload.go:117-119,178-180`

**Issue:** Eight call sites wrap `Sprintf` in `Fprintln`, allocating an
intermediate string and needing a `//nolint:errcheck` on each.

**Fix:** `fmt.Fprintf(w, tuikit.UploadResultOKFmt+"\n", row.Label)`.

### IN-04: frame-promote's header documents `PROVENANCE.md`, the tool writes `README.md`

**File:** `cmd/gitid-frame-promote/main.go:13`, `cmd/gitid-frame-promote/main.go:112-115`

**Issue:** The package comment says provenance lands in
`ui-frames/PROVENANCE.md`; `run()` writes `filepath.Join(dstDir,
"README.md")` and the error message still says "writing PROVENANCE table".

**Fix:** Align the comment with the actual filename (or vice versa).

### IN-05: UTF-8-unsafe byte slice of a frozen format string in a test assertion

**File:** `cmd/gitid/identity_upload_test.go:320`

**Issue:** `tuikit.UploadResultFailedFmt[:5]` slices `"✗ %s key ..."` at byte
5, and `✗` is 3 bytes — the result is `"✗ %"`, which never appears in rendered
output. The assertion only passes because of the `||` fallback on
`"registration failed"`. The first half is permanently dead.

**Fix:** Drop the slice and assert on the rendered substring directly.

### IN-06: e2e children inherit the ambient environment, including real provider tokens

**File:** `e2e/harness_test.go:509-537`

**Issue:** `e2eEnv` builds `append(os.Environ(), HOME=..., PATH=..., ...)`, so
`GH_TOKEN`, `GITHUB_TOKEN`, `GH_CONFIG_DIR`, `GLAB_*` and friends reach every
child. The hermetic guarantee rests entirely on PATH ordering; a single
missed `pathPrefixes` argument turns "cannot reach a real CLI" into "reaches
it fully authenticated". Cheap defense in depth given how carefully the rest
of the boundary is built.

**Fix:** Filter provider-credential variables out of the inherited
environment (or set them to empty) inside `e2eEnv`.

### IN-07: `u` is consumed on step 0 even when the checkbox row is not rendered

**File:** `internal/tuikit/identities.go:3598-3607`

**Issue:** The hotkey branch checks only the focus slot, not
`w.uploadRowVisible()`, so on a non-gated host `u` is swallowed (returns
`handled: true`) with no visible effect. `toggleUploadCheckbox` correctly
no-ops for the Omitted state, so nothing breaks — the key is just silently
inert.

**Fix:** `if key == "u" && w.uploadRowVisible() && w.focus > sshFieldPort && ...`.

### IN-08: `TestRegisterKeyExitContract` documents six cases, defines five

**File:** `cmd/gitid/identity_upload_test.go:172-194`

**Issue:** The doc comment says "six canned views ... a nil error for the
first five"; the table has five entries and the dry-run case named in the
R10 exit contract ("full success, partial success, already-complete,
ineligible provider, dry run") is not among them.

**Fix:** Add the dry-run row (or correct the comment).

---

_Reviewed: 2026-08-29T18:04:33Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
