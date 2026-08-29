---
phase: 09-upload-credentials-assist
reviewed: 2026-08-29T18:35:00Z
depth: standard
iteration: 3
supersedes: iteration 2 (2026-08-29T21:05:00Z) + 09-REVIEW-FIX-2.md
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
  critical: 0
  warning: 14
  info: 14
  total: 28
status: issues_found
---

# Phase 9: Code Review Report (re-review, iteration 3 — final planned pass)

**Reviewed:** 2026-08-29T18:35:00Z
**Depth:** standard
**Files Reviewed:** 51
**Status:** issues_found

## Summary

Third and final planned pass over the Phase 9 diff (`48cea9a..HEAD`, 51 Go
files, 12,249 added lines). Two questions were asked: are iteration 2's two
Critical findings genuinely fixed and still correct after everything that
landed alongside them, and is there anything the three latest test/fixture
corrections got subtly wrong in a way that masks a production defect.

**Both iteration-2 Criticals are genuinely and durably fixed.** I verified
them by reading the merged code and by tracing the discriminators they turn
on, not by reading the fix report — details and traces in the verification
section below. `go build ./...`, `go vet ./...`, `go test
./internal/uploader/... ./internal/tuikit/... ./cmd/gitid/...` (1223 tests),
`go test -tags screenshot ./internal/screenshot/... ./cmd/gitid/...` (829
tests, only the environmental `TestCaptureTUI`/`freeze`-not-on-PATH failure),
and `go test -tags e2e ./e2e/... -run TestIdentityManager_RotateDeleteOffer`
(3 tests) are all green as submitted. The registry/allowlist drift iteration 2
recorded as an open pre-existing failure is now genuinely closed —
`TestUploadVisualAllowlistMatchesRegistry` passes.

**There are no new Critical findings.** That is a deliberate, evidenced
conclusion rather than a soft one: I went looking for a third destructive path
and traced every route into `CommitRotateDeleteOldKey`, the dry-run staging
discriminator, the delete-argv namespace, and the ID validation. Where I found
a route that *defeats* one of the two CR-02 gates (WR-01 below), the second,
independent gate provably still holds — which is exactly the value the
belt-and-braces layer was added for, and the reason that finding is a Warning
and not a Critical.

**On the three latest corrections (priority 2):** two are sound; one is
materially less faithful than real GitHub in exactly the dimension CR-01 turns
on. `FakeGHTrackAddedKeys` records each added key under a *hardcoded* title
(`fake-added-auth`/`fake-added-signing`) instead of the `--title` value gitid
actually passed. Real GitHub records the D-07 title — which is byte-identical
to the OLD key's title, and that title collision is the entire reason
`OldKeyCandidates`' blob exclusion exists. Because the fixture's added records
never share the title, they are dropped by the title filter before the
exclusion is ever consulted: the e2e suite sees a two-record candidate set
where production sees four, and a regression that removed the blob exclusion
outright would still pass all three rotate-delete e2e tests (WR-02). The
disposition sync (`internal/screenshot/createflow.go`) is correct as a
registry/allowlist reconciliation but enshrines a real product problem as
"by design" (WR-08). The stale-assertion correction is fine.

The rest of this report is fourteen Warnings — six genuinely new, eight
carried forward because the code still has the defect (the fix pass skipped
seven Warnings, and one of the skipped ones, WR-07, was correctly re-scoped
rather than closed) — and fourteen Info items, four of them new.

## Fix Verification (iteration-2 CR-01 / CR-02)

Verified against the merged source, plus the staging code the fix depends on.

**CR-01 — `--dry-run` no longer writes into the real `~/.ssh`. Correct.**
`runCreateDryRun` (`cmd/gitid/identity_create.go:483-503`) now stages a temp
`.pub` sibling *only* when `staged.PrivPEM != nil`, and for the reuse path
previews against `staged.FinalPubPath` **only if it already exists**
(`pubPath != "" && b.deps.PubExists(pubPath)`). I confirmed `PrivPEM` is a
sound generate/reuse discriminator at the source: `identity.StageReuse`
(`internal/identity/modes.go:64-79`) returns `PrivPEM: nil` with
`TempPrivatePath == FinalPrivatePath == <the user's real key>`, and the
generate path's `TempPrivatePath` lives under `os.MkdirTemp("",
"gitid-stage-")` (`cmd/gitid/wiring.go:5143`) — i.e. `/tmp`, never `~/.ssh`.
A `WritePub` failure now sets `pubPath = ""` and skips the preview, honoring
the comment WR-08 flagged as false. The regression test
(`identity_test.go:1216-1246`) is non-vacuous: the `NoUpload: true` it passes
neuters only `runUploadStep`, which is called *after* the `WritePub` decision,
so the code path CR-01 flagged is still fully exercised and the HOME-listing
assertion still binds.

**CR-02 — the D-04 offer is gated twice, independently. Correct.**
- Model layer: `identities.go:2288` gates the dispatch on
  `uploadSucceeded(run.View)` (`identities.go:4606-4619`), which requires
  `AlreadyComplete` or a non-empty row set in which *every* row is
  `UploadRowUploaded`/`UploadRowAlreadyPresent`. A zero-row run (Omitted,
  Disabled, Skipped, `--no-upload`) is treated as unproven, not vacuously
  true — the correct polarity.
- Backend layer: `rotateDeleteOfferFor` (`cmd/gitid/upload_run.go:454-458`)
  independently refuses unless a **freshly read** inventory already carries
  the current pub's blob for every `desiredRegistrations(tool)` entry. I
  traced the compositions this depends on: `desiredRegistrations` returns
  `[Auth, Signing]` for gh and `[Combined]` for glab (`wiring.go:1484-1489`),
  matching the `Registration` values `Inventory` stamps on each record
  (`inventory.go:39-47, 86`), so the check cannot pass vacuously for either
  tool. `HasRegistration` fails closed on a blank blob
  (`inventory.go:148-152`).
- The two layers are genuinely independent — the backend gate fires even when
  the model gate is bypassed, which is what saves WR-01 below from being a
  Critical.
- Both layers have RED-verified tests
  (`identity_manager_upload_test.go:355-408`, five unproven shapes plus three
  proven ones; `wiring_test.go:6179, 6204`).

**WR-01 (iteration 2) — `OldKeyCandidates` now fails CLOSED.** Both
directions are closed at `inventory.go:269-292`: a blank `currentBlob`
short-circuits to `nil`, and any candidate record with an uncomparable blob
returns `nil` for the whole set. Correct.

**No regression from the WR-01/WR-03/WR-05/WR-10 fixes landing alongside.**
`glabInventory` (`inventory.go:82-95`) is new but does not touch the D-04
path's semantics; `extractUploadSection`'s narrowed marker
(`createflow_regions.go:1311-1315`) no longer collides with either checkbox
label and `TestRegionDiffCoverage` passes; the dummy fixture now models the
real per-registration delete shapes. The disposition sync
(`createflow.go:2747-2765`) reconciles the registry with the allowlist rather
than weakening the gate's predicate schema.

## Narrative Findings (AI reviewer)

### Critical Issues

None. See the Summary for why this is an evidenced conclusion, and WR-01 for
the one path that defeats a CR-02 gate but is stopped by its sibling.

### Warnings

#### WR-01 (new): `UploadRunMsg` carries no identity — the CR-02 model gate can be satisfied by a *different* beat's upload result

**File:** `internal/tuikit/views.go` (`UploadRunMsg` definition),
`internal/tuikit/identities.go:2269-2294`, `:2360-2364`

**Issue:** Every other async Phase 9 message carries a correlation field
*specifically* so a stale reply can be discarded — `RegisterKeyPlanMsg` has
`Name` (guarded at `identities.go:2346`), `UploadEligibilityMsg` has
`Hostname` (guarded at `:2468`, with a comment naming the stale-guard idiom).
`UploadRunMsg` has neither. Its two consumers guard only on pane + a boolean
pending flag:

```go
if run, ok := msg.(UploadRunMsg); ok && m.pane == paneKeyCeremony && m.keyCeremonyUploadPending {
...
if run, ok := msg.(UploadRunMsg); ok && m.pane == paneRegisterKey && m.registerKeyPending {
```

The upload beat is multi-second (auth probe + two paginated inventory reads +
two `ssh-key add` calls + a D-17 confirmation with a real sleep), so a reply
outliving its originating pane state is not hypothetical. Two reachable
consequences:

1. **Cross-identity D-04 dispatch.** Rotate identity A; close the ceremony
   while A's upload is still in flight (`openKeyCeremony` resets the flag, but
   the *in-flight command* is not cancellable); rotate identity B and confirm.
   B's commit sets `keyCeremonyUploadPending = true`; A's late `UploadRunMsg`
   is then consumed as B's, jumps B's ceremony to `"review"`, renders A's rows
   as B's, and — if A's upload succeeded — dispatches `RotateDeleteOffer(B)`
   off A's evidence. This is the *same defect class* CR-02 was raised for (the
   destructive offer dispatched on a registration that was never proven for
   *this* identity), reached by a different route. It does not become a
   Critical only because `rotateDeleteOfferFor`'s belt-and-braces check
   re-reads the inventory for B and refuses independently.
2. **Cross-pane result contamination.** Press `u` on identity A (which
   registers immediately — see WR-03), Esc, press `u` on B: A's result is
   displayed as B's, so the user is told B's key registered (or failed) based
   on A's outcome, and A's *real* registration outcome is silently discarded.

**Fix:** give `UploadRunMsg` the correlation field its siblings already have
and guard on it, exactly as `RegisterKeyPlanMsg` does:

```go
type UploadRunMsg struct {
    Name string // the identity this run is about ("" for the create wizard)
    View UploadRunView
}
...
if run, ok := msg.(UploadRunMsg); ok && m.pane == paneKeyCeremony &&
    m.keyCeremonyUploadPending && run.Name == m.selected { ... }
if run, ok := msg.(UploadRunMsg); ok && m.pane == paneRegisterKey &&
    m.registerKeyPending && run.Name == m.registerKeyName { ... }
```

`RunUploadForIdentity` already has `name` in scope (`wiring.go:1570-1587`);
`RunUpload` can pass `spec.Identity`. Add a unit regression that delivers an
`UploadRunMsg` for identity A while the model is mid-ceremony on B and asserts
no `RotateDeleteOffer` command is returned.

#### WR-02 (new): the new `FakeGHTrackAddedKeys` fixture is *less* faithful than real GitHub in exactly the dimension CR-01 turns on

**File:** `e2e/harness_test.go:885-892`, with
`e2e/identity_manager_pty_e2e_test.go:976-985, 1026, 1082, 2148`

**Issue:** The stateful shim records each added key under a hardcoded title:

```sh
printf '{"id":9001,"title":"fake-added-auth","key":"%s"}\n' "$(cat "$3")" >> "$GITID_FAKE_GH_KEYS_AUTH_FILE"
```

Real `gh ssh-key add … --title "$TITLE"` records the title gitid passed, and
for a rotate that title is `uploader.KeyTitle(name, machine)` — **byte-
identical to the old key's title** (`seedRotateDeleteOfferFixture` seeds the
old record with exactly that value). That title collision is the entire
premise of CR-01 and of `OldKeyCandidates`' `blob == currentBlob` exclusion.

Because the fixture's added records carry a *different* title, they are
dropped by the title filter (`inventory.go:276-278`) before the blob exclusion
is ever consulted. Concretely:

| | records matching the D-07 title | what disambiguates them |
|---|---|---|
| Real GitHub | 4 (old auth+signing, new auth+signing) | the blob exclusion (CR-01) |
| This fixture | 2 (old auth+signing only) | nothing — the exclusion is a no-op |

So the three rotate-delete e2e tests exercise a candidate set that cannot
occur in production, and **deleting the `blob == currentBlob` exclusion
entirely would leave all three tests green** — while in production it would
produce a 2-distinct-blob candidate set and a refusal (or, before the
ambiguity check, a delete of the key just registered). The fixture is more
permissive than the real provider precisely where the destructive decision is
made. It also silently weakens the belt-and-braces check it was added to
satisfy: `HasRegistration` matches on blob only, so the check passes for the
wrong reason relative to what a real inventory would return.

**Fix:** record the real `--title`, since the shim already has the argv:

```sh
title=$(echo "$*" | sed -n 's/.*--title \(.*\) --type.*/\1/p')
printf '{"id":9001,"title":"%s","key":"%s"}\n' "$title" "$(cat "$3")" >> "$GITID_FAKE_GH_KEYS_AUTH_FILE"
```

(or capture it by scanning `"$@"` for `--title` and taking the next operand,
which is more robust than a `sed` over `"$*"`). Then re-run the three tests:
they must *still* pass, and that pass is the first end-to-end evidence that
CR-01's exclusion works. As a durability check, temporarily delete the
exclusion and confirm at least one of them goes RED.

#### WR-03 (new): the D-08 register-key pane performs a real remote mutation on open, from a single unmodified keypress, with no confirmation and no cancel

**File:** `internal/tuikit/identities.go:2528-2534` (the `u` hotkey),
`:2650-2662` (`openRegisterKey`), `:2346-2364` (plan → immediate upload),
`:2665-2675` (`handleRegisterKeyKey`)

**Issue:** Pressing `u` in the identity detail pane opens `paneRegisterKey`,
which dispatches `RegisterKeyPlan`; the moment that resolves `Ready`, the
model dispatches `RunUploadForIdentity` unconditionally — a real
`gh/glab ssh-key add` against the user's provider account. There is no
confirmation step, no preview of the command about to run, and
`handleRegisterKeyKey` accepts only `esc` (every other key is swallowed), so
there is no abort. Escaping after dispatch does not cancel the in-flight
registration; it only guarantees the user never sees whether it succeeded
(the reply is dropped by the pane guard — see WR-01).

Compare the sibling destructive-ish surfaces in the same pane switch: `d`
opens a delete-choice ceremony with a typed confirm, `f` opens a fix ceremony
with a diff preview. `u` is the only one that mutates first and shows the
result second. CLAUDE.md's Working method is explicit — mutations happen "only
after the hypothesis is confirmed, and only with user confirmation" — and D-02
/ R7's own announce-before-run contract is honored everywhere *except* here.
The mutation is additive and reversible, which is why this is a Warning and
not a Critical, but it is still an unconfirmed remote write triggered by one
letter.

**Fix:** render the resolved plan's commands and require an explicit
confirmation keystroke before dispatching `RunUploadForIdentity`, reusing the
same announce-then-run beat `RunUpload` already provides via
`UploadStartedMsg`:

```go
if plan.View.State == UploadEligibilityReady {
    m.registerKeyAwaitingConfirm = true   // render "Enter registers, Esc cancels"
    return keyResult{model: m}
}
```

At minimum, keep the in-flight result reachable after Esc so a registration
that *did* happen is never invisible.

#### WR-04 (new): running the e2e suite rewrites TRACKED approved baseline frames with sandbox-specific paths

**File:** `e2e/global_ssh_pty_e2e_test.go:65-76`,
`e2e/global_git_pty_e2e_test.go:56-67`,
`e2e/global_ssh_storage_pty_e2e_test.go:160-172`

**Issue:** These three helpers write PTY snapshots straight into the tracked
baseline directories:

```go
path := filepath.Join(repoRoot(t), ".planning", "phases", "06-global-ssh-options", "ui-frames", name+".txt")
os.WriteFile(path, []byte(frame), 0o644)
```

This is the exact defect `saveFrame`'s own WR-12 comment
(`e2e/ui_pty_e2e_test.go:264-276`) documents and fixes for the 05.7 helper —
"every run embeds absolute sandbox paths (`t.TempDir()`) … so every run on
every machine produced a different file and dirtied the working tree". It is
reproducible right now in this tree: sixteen frames under
`.planning/phases/06-*/ui-frames/` and `.planning/phases/07-*/ui-frames/` are
modified, differing only in the sandbox path
(`IdentityFile /tmp/h825086898/... → /tmp/h3889901778/...`), with mtimes
matching this session's e2e run.

Consequences: `make test-e2e` cannot be run without dirtying the repo; a real
frame regression in phases 6/7 is indistinguishable from this noise in
`git status`; and any provenance/hash gate over those directories (the model
`cmd/gitid-frame-promote` establishes for phase 9) is unstable there. These
files are pre-existing but are in this phase's diff and this phase set the
precedent for the fix.

**Fix:** route all three through `saveFrame` (or its `tmp/ui-frames`
directory) and promote deliberately, as phase 9 does:

```go
func captureGlobalSSHFrame(t *testing.T, name string, s *ptySession) string {
    frame := s.snapshot()
    saveFrame(t, name, s) // tmp/ui-frames, gitignored
    return frame
}
```

Then restore the sixteen dirty baselines from git.

#### WR-05 (new): the dummy backend advertises unquoted upload commands the real product no longer emits

**File:** `internal/dummytui/fixturebackend.go:472-475`

**Issue:** WR-18's fix made the real `CommandPreview`/`DeleteCommandPreview`
shell-quote every argument, because "the D-07 title always contains spaces
… so an unquoted preview pasted into a shell ran a DIFFERENT command"
(`uploader.go:298-305`). The dummy backend — this project's design and
screenshot source of truth — still builds its previews by hand, unquoted:

```go
authCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title %s --type authentication", spec.KeyPath, title)
```

`title` is `gitid: <name> @ demo-machine`, so every demo frame and every
promoted `register-key-modal`/upload screen shows a command that, pasted into
a shell, does the wrong thing — the precise bug WR-18 closed in production.
WR-05 (iteration 2) fixed exactly this class of drift for `RotateDeleteOffer`
but left `RunUpload` behind, so the fixture is now half-migrated.

**Fix:** build the fixture previews through the same quoting rule (or, better,
hardcode the already-quoted strings the real `previewLine` emits):

```go
authCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title '%s' --type authentication", spec.KeyPath, title)
```

and re-promote the affected frames.

#### WR-06 (new): `create`/`clone`/`rotate`/`new-key` `--dry-run` now makes real provider API calls that their flag help does not mention

**File:** `cmd/gitid/identity_create.go:93`, `cmd/gitid/identity_clone.go:49`,
`cmd/gitid/identity_key.go` (rotate/new-key bindings), vs
`cmd/gitid/identity_upload.go:47-52`

**Issue:** The Phase 9 upload preview added to all four write verbs' dry runs
calls `planUpload`, which runs `gh auth status`, `gh api --paginate
user/keys` and `gh api --paginate user/ssh_signing_keys` (or the glab
pagination loop) — network calls that enumerate the user's account keys. The
four flag help strings still say only:

```
"run both connectivity stages, print the artifact previews, and exit 0 without writing"
```

`register-key`'s own `--dry-run`, written under the same R11 rule ("precise
about scope"), documents the probes explicitly. One of five surfaces got the
rule applied; the other four now understate what a dry run does.

**Fix:** extend the four help strings with `register-key`'s clause, e.g.
`"… and exit 0 without writing; the provider auth-status check and key-inventory read may still run as read-only probes"`, and extend
`TestNoUploadFlagIsBoundOnAllFourWriteVerbs`'s sibling assertion to pin the
dry-run text across the four verbs so they cannot drift again.

#### WR-07 (new): `glabInventory` has no page cap — a provider that ignores `--page` loops forever

**File:** `internal/uploader/inventory.go:82-95`

**Issue:**

```go
for page := 1; ; page++ {
    keys, err := inventoryFor(...)
    all = append(all, keys...)
    if len(keys) < glabPerPage { return all, nil }
}
```

The only termination condition is a short page. Any endpoint that returns a
full page regardless of `--page` — an older `glab` that silently ignores the
unknown flag, a corporate proxy, a `glab` alias, a future flag rename —
produces an unbounded loop that spawns a subprocess per iteration and grows
`all` without limit, inside a `tea.Cmd` goroutine with no cancellation. The
TUI hangs with no visible cause. gh's `--paginate` has no equivalent exposure
because pagination is the CLI's own responsibility there.

**Fix:** bound the loop and degrade honestly rather than spinning:

```go
const glabMaxPages = 40 // 1200 keys — far beyond any real account
for page := 1; page <= glabMaxPages; page++ { ... }
return all, fmt.Errorf("uploader: glab ssh-key list did not terminate after %d pages", glabMaxPages)
```

An error here is already non-gating (callers treat it as D-15 degradation), so
the failure mode becomes "inventory unavailable" instead of a hang. Add a test
whose fake `RunCmd` returns a full page unconditionally and assert the call
returns.

#### WR-08 (carried, aggravated): the overflow backstop still discards content silently, still un-clamps when `budget <= 0`, and the divergence is now enshrined as by-design

**File:** `internal/tuikit/identities.go:2965-2981`, `:4658-4670`,
`internal/screenshot/createflow.go:2747-2765`,
`.planning/design/identity-manager/visual-divergence-allowlist.txt:173-174`

**Issue:** Iteration 2's WR-06 was skipped, so both halves remain:

1. `if budget > 0 && tailLines > budget` — when `budget <= 0` (the receipt
   already filled the frame, which the code's own comment says a real rotate
   routinely does) the **entire unclamped tail** is appended, overflowing the
   fixed 100x30 frame instead of being bounded. The identical pattern is
   repeated at `:4660` for the manual-fallback block.
2. No truncation indicator, and the pane is not scrollable while the offer
   owns key input (`:2776-2824`), so clipped content is unreachable.

What is new in this iteration is that the divergence is now *registered as
correct* on both sides — the code-side disposition
(`rotateDeleteOfferUploadDisposition`, `absent:"ssh-key add"`) and the
allowlist row both assert the real pane shows no upload text at all on the
rotate-delete-offer screen. The visual gate will now fail if that ever gets
fixed. Post-CR-02 the clipped rows are ✓ rows rather than ✗ rows, so this is
no longer a safety issue — but the destructive question is still asked with
its own premise ("your new key registered") invisible, and the delete
keybinding stays live even when the choice row itself is clipped off-screen.

**Fix:** clamp unconditionally and mark the cut, at both sites:

```go
if tailLines > budget {
    lines := maxInt(1, budget)
    v := ExactTextViewport{Text: wrapped, VisibleLines: lines, Width: maxInt(20, deleteChoiceNoteWidth-4)}
    return body + "\n" + v.Clamp().View() + "\n " +
        styleFaint.Render(fmt.Sprintf("… %d more line(s) hidden", tailLines-lines)) + "\n"
}
```

and re-evaluate the two `absent:` dispositions afterwards rather than leaving
them as permanent cover.

#### WR-09 (carried from iteration 2, WR-02): a retry after a partial delete can never succeed

**File:** `cmd/gitid/wiring.go:1641-1656`,
`internal/tuikit/identities.go:2301-2315`

**Issue:** Unchanged. `CommitRotateDeleteOldKey` loops over every candidate
and reports failure if any one fails; the model keeps the full encoded set and
tells the user "press Enter on Delete to retry", which re-sends the
already-deleted IDs. `gh api -X DELETE user/keys/<gone-id>` returns 404, so
the retry fails permanently and the "✓ Old key removed" state is unreachable
even once the goal state is in fact true.

**Fix:** treat "already absent" as success in the loop, or drop each
successfully-deleted candidate from `rotateDeleteConfirmedID` before reporting
the partial failure.

#### WR-10 (carried from iteration 2, WR-04): the multi-line `ManualCommand` is still interpolated into a single-line sentence

**File:** `cmd/gitid/upload_run.go:520-527`, `internal/tuikit/design.go:695`,
`internal/tuikit/identities.go:2796, 2805, 3003`

**Issue:** Unchanged. `deleteCandidatesManualCommand` joins one preview per
candidate with `"\n"`; `RotateDeleteOfferResultLeftFmt = "Left in place —
remove it yourself: %s"` renders the second command at column 0 with no
leading space and no context. The iteration-2 fix pass updated the dummy
fixture to emit the real two-line shape, so the demo now *shows* the broken
rendering instead of hiding it — the drift is visible but not fixed.

**Fix:** render the commands as an indented block (a `design.go` R22
amendment) or join with `" && "` if one pasteable line is the intent.

#### WR-11 (carried from iteration 2, WR-09): the upload checkbox's actionable copy is still unreadable at the only width production uses

**File:** `internal/tuikit/identities.go:4673-4703`,
`internal/tuikit/design.go:582, 586`

**Issue:** Unchanged. `renderUploadCheckboxRow` is always called at width 60
(`:4707`); `UploadCheckboxLabelUnauthFmt` renders ~106 columns, so the entire
remediation (`run "gh auth login" first, or check anyway`) is truncated away,
and the disabled label loses `Manual steps are shown after create.` The
ellipsis makes the loss visible without making the row useful. The fix
report's deferral is still recorded only in a fix report, not as a
ROADMAP/backlog item.

**Fix:** amend the frozen copy to fit 60 columns, or file the R22 amendment as
an explicit tracked item.

#### WR-12 (carried from iteration 2, WR-11): provider JSON is parsed out of `CombinedOutput`

**File:** `cmd/gitid/wiring.go:560-577`, `internal/uploader/inventory.go:97-111`

**Issue:** Unchanged. `RunCmd` returns `cmd.CombinedOutput()` and
`inventoryFor` feeds it straight to `json.Decoder`, so any stderr diagnostic
on a *successful* call (gh deprecation notices, glab's `Using host …` banner,
a proxy message) corrupts the JSON stream and turns a healthy inventory into a
D-15 degradation. No fixture can reproduce it because every shim writes only
to stdout. This is now more consequential than at iteration 2, because
`rotateDeleteOfferFor`'s belt-and-braces check treats an inventory error as
"refuse the offer" — a stderr banner silently disables D-04 entirely.

**Fix:** add a separated-streams variant to `Deps` for parsing calls; at
minimum skip leading non-`[`/`{` lines and carry the discarded text into the
degradation reason.

#### WR-13 (carried from iteration 2, WR-12/WR-16): the wizard registers a key remotely before the identity is committed, with no way back

**File:** `internal/tuikit/identities.go:3799-3811`,
`cmd/gitid/upload_run.go:83-100`

**Issue:** Unchanged. The D-05 upload beat runs at wizard step 1 with the
checkbox pre-checked whenever eligibility resolves `Ready`; abandoning the
wizard afterwards leaves a key registered on the user's account under
`gitid: <name> @ <host>` whose private half `deps.Cleanup` destroyed, and
gitid offers no removal path (D-04 is rotate-only). Still tracked only inside
a fix report.

**Fix:** file the follow-up as a plan/ROADMAP item, and add the cheap
mitigation now — on wizard abandon after at least one `UploadRowUploaded`,
emit a note carrying the D-07 title so the user can find and remove the key.

#### WR-14 (carried from iteration 2, WR-07, correctly re-scoped): the wizard's reuse path can upload a `.pub` that does not exist

**File:** `cmd/gitid/upload_run.go:89-100`

**Issue:** The fix pass correctly disproved half of this finding — the CLI
`runCreateCeremony` site is safe because `commitCreateInto` writes
`FinalPubPath` before `runUploadStep` runs. The remaining half is real and
unchanged: `uploadRequestFromSpec` falls through to `pubPath :=
staged.FinalPubPath` for `PrivPEM == nil`, and `StageReuse` sets
`FinalPubPath = existingKeyPath + ".pub"` **without guaranteeing the file
exists** (`internal/identity/modes.go:64-79`; `ensurePubReadOnly` explicitly
supports deriving the line in memory). A wizard user reusing a hand-imported
key with no `.pub` gets a red `✗ Authentication key registration failed:
reading public key …` for a perfectly valid configuration.

**Fix:** when `FinalPubPath` is absent, write the derived line to a **staging
dir** temp sibling — never to `TempPrivatePath` when it equals
`FinalPrivatePath` (CR-01's lesson) — and upload that.

### Info

#### IN-01 (new): `mustSeeSlow`'s rationale is stale after `FakeGHTrackAddedKeys`

**File:** `e2e/identity_manager_pty_e2e_test.go:997-1003`

The comment justifies the 20s budget by "a real 2s sleep the fake gh's static
inventory fixture always fails to converge against, since it never reflects
what was added" — which is exactly what the new stateful fixture changed. In
the three tests that call `FakeGHTrackAddedKeys`, D-17 now converges on the
first read and no retry sleep occurs. **Fix:** update the comment (the longer
budget is still justified by the chained offer probe).

#### IN-02 (new): the fake-gh shim's `api -X DELETE` always succeeds and never mutates its own state

**File:** `e2e/harness_test.go:918-945`

In `delete-ok` mode the `api` verb answers every invocation — including
`-X DELETE user/keys/555` — with the inventory fixture and exit 0. Nothing is
removed from the tracked state, so no test can assert "after the delete, the
key is gone", and a delete that addressed the *wrong* resource would still
report success. The argv assertions in
`TestIdentityManager_RotateDeleteOfferDeletesOnExplicitChoice` are the only
thing standing in for real semantics. **Fix:** branch the `api` verb on
`-X DELETE`, remove the matching ID from the state file, and exit non-zero
(404-shaped) for an unknown ID — which would also give WR-09 a home.

#### IN-03 (new): the fake-gh shim reads the pubkey path positionally as `$3`

**File:** `e2e/harness_test.go:886, 889`

`cat "$3"` assumes `buildArgs` keeps emitting `ssh-key add <path> --title …`.
The coupling is undocumented and invisible from `uploader.buildArgs`
(`uploader.go:315-324`). It fails closed today (a wrong `$3` yields an empty
blob → `HasRegistration` false → the offer refuses → tests fail loudly), which
is the right direction, but the failure message would be inscrutable.
**Fix:** scan `"$@"` for the first operand that is not a flag, and add a
comment on `buildArgs` naming the shim as a positional consumer.

#### IN-04 (new): frame-promote stamps HEAD as the "source commit" of frames captured earlier

**File:** `cmd/gitid-frame-promote/main.go:82-95, 140-141`

`gitHead(root)` is read at promote time and recorded as each row's source
commit, but the frames come from a `tmp/ui-frames/` capture that may predate
HEAD by any number of commits. The table's stated purpose is that "a later
frame from a DIFFERENT run can never be mistaken for the reviewed one" — the
SHA-256 column delivers that; the commit column can actively misattribute.
**Fix:** record the commit at capture time (write it into `tmp/ui-frames/` as
a sidecar during the PTY run), or rename the column to "promoted at commit".

#### IN-05 (new): `deleteArgs` silently ignores `reg` for GitLab

**File:** `internal/uploader/inventory.go:225-226`

The gh branch validates the registration and errors on an unsupported value;
the glab branch returns `ssh-key delete <id>` for *any* `Registration`,
including `Authentication`/`Signing`, which GitLab does not model. Harmless
today (`desiredRegistrations` only ever produces `Combined` for glab) but it
removes the namespace guard `DeleteKey`'s doc comment says is load-bearing.
**Fix:** return an error for anything other than `RegistrationCombined`.

#### IN-06 (carried): doc comments detached from the functions they describe

**File:** `internal/tuikit/identities.go:4551-4566`

The `renderUploadCheckboxRow` and `uploadRunHasContent` doc comments still sit
stacked immediately above `renderRegisterKey`; godoc attributes both to the
wrong symbol. **Fix:** move each above its own declaration.

#### IN-07 (carried): `uploadFailureView`'s `provider` parameter is a hostname, sometimes empty

**File:** `cmd/gitid/wiring.go:1478-1485`, `:1579`

`RunUploadForIdentity`'s not-found branch still passes `""`, rendering
`"Upload your public key to  as both an authentication key and a signing
key,"`. **Fix:** rename the parameter to `hostname` and skip the manual block
when empty.

#### IN-08 (carried): `fmt.Fprintln(w, fmt.Sprintf(...))` instead of `fmt.Fprintf`

**File:** `cmd/gitid/upload_run.go:258-287`,
`cmd/gitid/identity_upload.go:117-119, 181-183`

Ten call sites. **Fix:** `fmt.Fprintf(w, fmt+"\n", …)`.

#### IN-09 (carried): frame-promote's header documents `PROVENANCE.md`; the tool writes `README.md`, and the registry count disagrees with its comment

**File:** `cmd/gitid-frame-promote/main.go:13`, `:45-63`, `:92-95`

The package comment promises `ui-frames/PROVENANCE.md` while `run()` writes
`README.md` (the error string still says "writing PROVENANCE table"), and the
comment says "twelve new PTY tests plus the two pre-existing" (14) for a
13-row table. **Fix:** align all three.

#### IN-10 (carried): UTF-8-unsafe byte slice of a frozen format string in a test assertion

**File:** `cmd/gitid/identity_upload_test.go:349`

`tuikit.UploadResultFailedFmt[:5]` slices `"✗ %s key …"` at byte 5, yielding
`"✗ %"`, which never appears in rendered output; the assertion passes only via
its `||` fallback. **Fix:** assert on the rendered substring.

#### IN-11 (carried): e2e children inherit the ambient environment, including real provider tokens

**File:** `e2e/harness_test.go:531-536`

`e2eEnv` still builds `append(os.Environ(), …)`, so `GH_TOKEN`,
`GITHUB_TOKEN`, `GH_CONFIG_DIR`, `GLAB_*` reach every child; hermeticity rests
entirely on PATH ordering. **Fix:** blank the provider-credential variables
inside `e2eEnv`.

#### IN-12 (carried): `u` is consumed on wizard step 0 even when the checkbox row is not rendered

**File:** `internal/tuikit/identities.go:3631`

The hotkey branch checks only `w.focus > sshFieldPort`, not
`w.uploadRowVisible()`, so on a non-gated host `u` is swallowed with no
effect. **Fix:** add `&& w.uploadRowVisible()`.

#### IN-13 (carried): `TestRegisterKeyExitContract` documents six cases, defines five

**File:** `cmd/gitid/identity_upload_test.go:201-223`

The comment still says "six canned views … a nil error for the first five";
the table has five rows and the dry-run case named in the R10 exit contract is
absent. **Fix:** add the row or correct the comment.

#### IN-14 (carried): `RotateDeleteOffer`'s doc comment describes the pre-CR-01 matching, and two helpers are dead in production

**File:** `cmd/gitid/wiring.go:1590-1602`,
`internal/uploader/inventory.go:232-243`, `internal/uploader/uploader.go:271-277`

The comment still says matching is "by EXACT title equality …
(`uploader.KeyTitle` + `uploader.FindByTitle`)" — the behavior CR-01 replaced,
i.e. a stale comment that documents a fixed defect as the design.
`FindByTitle` and `titleMatchesThisMachine` now have no production caller.
**Fix:** point the comment at `OldKeyCandidates`, and delete or justify the
two helpers (WR-14 removed `Detect`/`TrimOutput` for the same reason).

---

## Verification commands run

```
go build ./...                                                        # clean
go vet ./...                                                          # clean
go test ./internal/uploader/... ./internal/tuikit/... ./cmd/gitid/...  # 1223 passed
go test -tags screenshot ./internal/screenshot/... ./cmd/gitid/...     # 829 passed, 1 failed (TestCaptureTUI: freeze not on PATH — environmental)
go test -tags e2e ./e2e/... -run TestIdentityManager_RotateDeleteOffer # 3 passed
```

---

_Reviewed: 2026-08-29T18:35:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard — iteration 3 (final planned pass, after 09-REVIEW-FIX-2.md and this session's three follow-up corrections)_
