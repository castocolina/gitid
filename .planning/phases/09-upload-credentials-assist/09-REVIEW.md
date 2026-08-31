---
phase: 09-upload-credentials-assist
reviewed: 2026-08-31T04:12:44Z
depth: standard
iteration: 5
supersedes: iteration 4 (2026-08-31T02:33:51Z, backed up at 09-REVIEW.iter3.md)
head: 324d7d8
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
  - internal/dummytui/fixturebackend_test.go
  - internal/screenshot/createflow.go
  - internal/screenshot/createflow_packet_test.go
  - internal/screenshot/createflow_regions.go
  - internal/screenshot/createflow_regions_test.go
  - internal/screenshot/createflow_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/design.go
  - internal/tuikit/frame.go
  - internal/tuikit/identities.go
  - internal/tuikit/identity_manager_upload_test.go
  - internal/tuikit/upload_copy_test.go
  - internal/tuikit/upload_section_test.go
  - internal/tuikit/views.go
  - internal/uploader/classify.go
  - internal/uploader/inventory.go
  - internal/uploader/inventory_test.go
  - internal/uploader/uploader.go
findings:
  critical: 1
  warning: 7
  info: 24
  total: 32
status: issues_found
---

# Phase 9: Code Review Report (re-review, iteration 5 — third independent look)

**Reviewed:** 2026-08-31T04:12:44Z
**Depth:** standard
**Files Reviewed:** 51
**Status:** issues_found

## Summary

Third independent pass over Phase 9 at `324d7d8`, after the second fix pass
(`cc37af7..303e5a6`) closed iteration 4's `CR-01` plus four of its seven
warnings.

**Every gate in the battery was run by this reviewer, including both tagged
suites**, and all are green apart from one known-environmental failure:

```
go build ./...                                                        # clean, exit 0
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...                 # 2373 passed / 22 pkgs, exit 0
make lint                                                             # golangci-lint 0 issues (untagged + screenshot-tagged), exit 0
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -timeout 40m ./e2e/...     # 175 passed, 0 failed, exit 0
TERM=dumb SSH_AUTH_SOCK= go test -tags screenshot -timeout 30m \
    ./internal/screenshot/... ./cmd/gitid/...                         # 832 passed, 1 failed (TestCaptureTUI:
                                                                      #   "freeze binary not found on PATH" — environmental)
```

### Verdict on the second fix pass's specific claims (all independently checked)

- **`CR-01` (`UploadRunMsg.Name` on fixture backends) — genuinely fixed, and
  the guard is not vacuous.** All four `tuikit.Backend` implementations now
  set `Name` at every dispatch site: `realBackend` (`wiring.go:1513`, `:1521`,
  `:1525`, `:1528`, `:1579`, `:1583`, `:1586`), `dummytui.FixtureBackend`
  (`fixturebackend.go:503`, `:625`), `screenshot.offlineCaptureBackend`
  (`createflow.go:1113`), and `tuikit.stubBackend`
  (`backend_stub_test.go:569`, `:650`). The only two wrappers,
  `offlineCaptureBackend` and `uploadProbeBackend`, embed the `tuikit.Backend`
  *interface* around a concrete backend and do not override
  `RunUploadForIdentity`, so they inherit the fix rather than needing their
  own — the fixer's claim holds. The new e2e assertion
  (`identity_manager_pty_e2e_test.go:2142`) is a real polling `t.Fatalf`
  (`mustSee`, 8s budget), and `"Authentication key registered"` is emitted
  only by `renderUploadSection`'s `UploadRowUploaded` branch, which is
  unreachable unless the `UploadRunMsg` clears the guard — non-vacuous. The
  key-ceremony half of the same regression is separately, structurally
  covered: the dummy leg's `"Remove the old key from GitHub?"` assertion can
  only pass if `handleMsg` consumed the `UploadRunMsg` (`identities.go:2293-
  2296` is the only dispatch site for `RotateDeleteOffer`). The **real** side
  has its own independent coverage in
  `TestIdentityManager_RegisterKeyModalRuns` (`:903-904`).
- **`budget-2` is arithmetically correct, not a number that happened to make
  a test pass.** Derivation: the overflow branch returns
  `body + "\n" + viewport + "\n " + cue + "\n"`; with
  `rendered = strings.Count(body, "\n")` and `budget = B - rendered - 1`, the
  returned string's `Count("\n")+1` is `rendered + lines + 3`, i.e.
  `body_rows + lines + 2` — the `+2` being the cue row and the trailing
  newline's own row. `lines <= budget - 2` is therefore exact, and the
  review's suggested `budget-1` was genuinely off by one. The fixer's *number*
  is right; its comment claiming this could only be found empirically
  ("not derived on paper") is wrong and unhelpfully mystifies the constant
  (`IN-21`), and the fix is still incomplete for `budget <= 2` (`WR-03`).
- **`WR-03` (`extractUploadedTitle`) — correct.** The flag-position scan is
  immune to earlier quoting; the `'\''` unescape is right (`i += 3` plus the
  loop's own `i++` advances exactly the 4 bytes of the escape); the
  no-flag-present case returns `""`. One residual nit at `IN-18`.
- **`WR-02` (fixture preview quoting) — correct**, and now covered by a real
  unit test (`internal/dummytui/fixturebackend_test.go`).
- **`WR-05`/`WR-06`/`WR-07` are correctly still open.** Each was
  re-verified in source this pass and each genuinely requires an amendment to
  frozen artifacts (`design.go`'s R22 copy, `FIELDS.md`), not a mechanical
  edit. They are restated below with the evidence, and one of them
  (`WR-05`) carries a newly-found, cheaply-fixable sub-defect.

### Why this is not `clean`

One **BLOCKER** that all three prior review rounds missed: **D-17's entire
post-upload confirmation result is written to a struct field that no renderer
in the codebase reads.** `confirmUpload` stamps `uploadUnconfirmedReasonFmt`
onto `rows[idx].Reason` for rows whose outcome is `UploadRowUploaded` /
`UploadRowAlreadyPresent`, but both presentation paths — the TUI's
`renderUploadSection` and the CLI's `printUploadOutcome` — read `row.Reason`
**only** in their `UploadRowFailed` branch. The user is shown a plain
`"✓ Authentication key registered"` for a registration gitid's own
verification explicitly could not see, and the deliberate two-second
`confirmSleep` retry buys nothing observable. This is dead code plus a false
success claim on the phase's flagship feature.

Seven warnings follow, of which three are the deliberately-deferred
frozen-copy/design items (`WR-05`/`WR-06`/`WR-07`) and four are new.

## Narrative Findings (AI reviewer)

### Critical Issues

#### CR-01 (new): D-17's post-upload confirmation result is never rendered — the UI claims "✓ registered" for a registration gitid could not confirm

**File:** `cmd/gitid/upload_run.go:399` (producer), `:298-307` (CLI renderer),
`internal/tuikit/identities.go:4765-4774` (TUI renderer),
`cmd/gitid/upload_run.go:327-334` (the unreachable constant),
`cmd/gitid/wiring.go:262-275` (the 2s retry sleep it pays for)

`confirmUpload` is D-17's whole reason for existing: after a successful
registration it re-reads the provider inventory, waits `2 * time.Second`
(`uploadConfirmRetryInterval`), re-reads once more, and — when a registration
the provider *accepted* still is not visible — records that fact:

```go
// cmd/gitid/upload_run.go:397-401
for registration, idx := range toConfirm {
    if !present[registration] {
        rows[idx].Reason = fmt.Sprintf(uploadUnconfirmedReasonFmt, providerHost)
    }
}
```

Those rows keep `Outcome == UploadRowUploaded` (or `UploadRowAlreadyPresent`)
— deliberately, per the constant's own doc comment ("no fourth
`UploadRowOutcome` value exists for this case"). But **neither renderer reads
`Reason` for those outcomes**:

```go
// internal/tuikit/identities.go:4765-4774 (TUI)
case UploadRowUploaded:
    b.WriteString(" " + styleHealthy.Render(fmt.Sprintf(UploadResultOKFmt, row.Label)) + "\n")   // Label only
case UploadRowAlreadyPresent:
    b.WriteString(" " + styleHealthy.Render(fmt.Sprintf(UploadResultSkippedFmt, row.Label)) + "\n") // Label only
case UploadRowFailed:
    b.WriteString(" " + styleError.Render(fmt.Sprintf(UploadResultFailedFmt, row.Label, row.Reason)) + "\n")
```

`cmd/gitid/upload_run.go:298-307` (`printUploadOutcome`) has the identical
three-way switch with the identical omission, and
`UploadResultOKFmt`/`UploadResultSkippedFmt` (`design.go:596`, `:599`) each
carry exactly one `%s` verb, so there is nowhere for the text to go even if a
caller passed it.

Verification that this is not rendered somewhere else: `rg '\.Reason'` over
the non-test tree returns exactly two `UploadResultRow.Reason` sites — the
producer at `upload_run.go:399` and the `UploadRowFailed`-only consumer at
`:305`. `rg 'uploadUnconfirmedReasonFmt|accepted but not yet visible'` returns
only the constant, its doc comment, and the one assignment: **no test asserts
it either**, which is why five gates and three review rounds all stayed green
over it. The string is also absent from `design.go`, where every other
user-facing Upload* string is frozen and covered by `upload_copy_test.go` —
another signal it was never wired to a surface.

Consequences, in order of severity:

1. **False success claim.** After a rotate or a create-wizard upload, the user
   reads `"✓ Signing key registered"` for a registration gitid re-checked
   twice and could not find. That is precisely the class of unproven claim the
   project's own `CR-03` note (`identities.go:2633`) calls out as a defect.
2. **A silent 2-second stall.** Every unconfirmed run pays a real
   `time.Sleep(2s)` inside the `tea.Cmd` goroutine (`wiring.go:268`) whose
   only output is discarded. The e2e harness even documents having to raise
   `mustSee`'s budget to 20s for it (`identity_manager_pty_e2e_test.go:997`).
3. **Dead code.** `uploadUnconfirmedReasonFmt`, the second `read()`, and
   `confirmSleep` are unobservable; a future refactor deleting them would
   change nothing a user or a test can see, which is the definition of an
   untestable requirement.

**Fix (minimal, no new outcome enum, no frozen-copy break):** render the
reason as an extra faint continuation row in both renderers when it is set on
a non-failed row. In `internal/tuikit/identities.go`:

```go
case UploadRowUploaded:
    b.WriteString(" " + styleHealthy.Render(fmt.Sprintf(UploadResultOKFmt, row.Label)) + "\n")
    if row.Reason != "" {
        b.WriteString("   " + styleWarning.Render(row.Reason) + "\n")
    }
case UploadRowAlreadyPresent:
    b.WriteString(" " + styleHealthy.Render(fmt.Sprintf(UploadResultSkippedFmt, row.Label)) + "\n")
    if row.Reason != "" {
        b.WriteString("   " + styleWarning.Render(row.Reason) + "\n")
    }
```

and the mirrored two lines in `printUploadOutcome`. Move
`uploadUnconfirmedReasonFmt` into `internal/tuikit/design.go` beside the other
frozen `Upload*` copy and add it to `upload_copy_test.go`'s table, so the
"shown == run" contract covers it. Add a regression test at the
`confirmUpload` level asserting the rendered output contains the phrase when
the second inventory read still does not see the key — the assertion that
would have caught this. Note that the extra rows are inside
`renderUploadSection`, so they are already accounted for by that function's
own overflow backstop budget.

### Warnings

#### WR-01 (new): `RotateDeleteCommitMsg` is the one Phase-9 async reply with no stale-guard — and its payload steers the next remote delete

**File:** `internal/tuikit/views.go:832-835`, `internal/tuikit/identities.go:2306`, `:2319-2321`, `:2838`

Every other async reply this phase added carries the identity it was resolved
for and is guarded on it — `RegisterKeyPlanMsg.Name` (`identities.go:2360`),
`RotateDeleteOfferMsg.Name` (`:2300`), and, since the iteration-3 `WR-01` fix,
`UploadRunMsg.Name` (`:2274`, `:2376`). `RotateDeleteCommitMsg` has **no
`Name` field at all**:

```go
type RotateDeleteCommitMsg struct {
    Err            string
    RemainingKeyID string
}
```

and its handler guards only on pane + pending:

```go
// identities.go:2306
if commit, ok := msg.(RotateDeleteCommitMsg); ok && m.pane == paneKeyCeremony && m.rotateDeleteCommitPending {
```

This is the *most* consequential message in the phase to leave unguarded,
because a failure reply does not merely display text — it **rewrites the ID
set the next destructive call will send**:

```go
// identities.go:2319-2321
if commit.RemainingKeyID != "" {
    m.rotateDeleteConfirmedID = commit.RemainingKeyID
}
```

and `m.rotateDeleteConfirmedID` is exactly what the retry hands to the
provider (`:2838`, `CommitRotateDeleteOldKey(sel.Name, m.rotateDeleteConfirmedID)`).
A reply belonging to identity A consumed inside identity B's ceremony would
therefore point B's retry at A's provider key IDs — on a shared `gh`-
authenticated account, deleting the wrong remote keys.

**Reachability, stated honestly:** today this is defensive, not
demonstrable. While `rotateDeleteCommitPending` is true the offer branch
swallows every key including `esc` (`:2809-2811`), so the user cannot leave
the pane, and `openKeyCeremony` resets the flag (`:2659`); `RunCmd` is
bounded by `providerCommandTimeout` (`wiring.go:561`), so the pending window
cannot be stretched indefinitely. The finding is a Warning rather than a
blocker for that reason. But the invariant that makes it safe is an
*accidental* one (a key-swallowing branch three hundred lines away), not a
stated one — exactly the shape of the iteration-3 `WR-01` race that was
accepted as real for the strictly less dangerous `UploadRunMsg`.

**Fix:** symmetric with the fix already applied to `UploadRunMsg`.

```go
// views.go
type RotateDeleteCommitMsg struct {
    Name           string // the identity this delete was dispatched for
    Err            string
    RemainingKeyID string
}
// identities.go:2306
if commit, ok := msg.(RotateDeleteCommitMsg); ok && m.pane == paneKeyCeremony &&
    m.rotateDeleteCommitPending && commit.Name == m.selected {
```

Set `Name` at both producers: `cmd/gitid/wiring.go:1627`, `:1631`, `:1635`,
`:1639`, `:1670`, `:1672` and `internal/dummytui/fixturebackend.go:548-552`.
**Set it at every producer in the same commit** — this is the identical
shape as the regression the previous fix pass shipped (guard added to the
consumer, `Name` never set on the fixture backends), so the fixture and
`stubBackend` producers must not be left behind. Add an anti-drift assertion
to the dummy leg of `TestRegisterKeyModal_CompiledRealVsLiveDummyPTY`'s
`rotate-delete-offer` subtest that the removal *result* row renders, not only
the offer heading.

#### WR-02 (new): the dummy backend fabricates a successful GitHub registration for identities that have no provider at all

**File:** `internal/dummytui/fixturebackend.go:605-635`, `internal/dummytui/data.go:245-246`

`FixtureBackend.RunUploadForIdentity` branches on the fixture SSH host and
falls through to the GitHub two-row success for **anything that is not
gitlab**, including the empty string:

```go
host := identityManagerSSHHost(name)   // "" for a row with no SSHHost
...
if strings.Contains(host, "gitlab") { ... } else {
    view = ... {Authentication: Uploaded}, {Signing: Uploaded} ...   // fabricated
}
```

Two seeded fixture rows have no `SSHHost` at all — `opensource` (`git-only`)
and `archived` (`key-unused`) (`data.go:245-246`) — and both are reachable
through the key ceremony, which dispatches `RunUploadForIdentity`
unconditionally after any successful commit (`identities.go:2259`). The dummy
therefore renders
`Running: /usr/local/bin/gh ssh-key add ~/.ssh/id_ed25519_opensource.pub …`
followed by two green `✓ … key registered` rows for an identity with no
provider.

The real backend does the opposite: `planUpload`'s D-13 gate returns
`UploadRunView{Skipped: true}` for `provider == ""` (`upload_run.go:159-162`),
and `uploadRunHasContent` is false for that view (`identities.go:4630-4632`),
so the real ceremony renders **nothing**. Its sibling `RegisterKeyPlan`
already models this correctly (`fixturebackend.go:589`, `Omitted` in the
default case) — only `RunUploadForIdentity` does not.

This matters more than an ordinary fixture nit because `cmd/gitid-dummy` is
this project's design/screenshot source of truth and one half of the paired
real-vs-dummy PTY comparison. The comparison drives `personal`
(`personal.github.com`), so it structurally cannot see this divergence.

**Fix:** mirror `RegisterKeyPlan`'s three-way branch and return a
provider-less view for a non-gated host.

```go
switch {
case strings.Contains(host, "gitlab"):
    view = ... combined row ...
case strings.Contains(host, "github"):
    view = ... auth + signing rows ...
default:
    view = tuikit.UploadRunView{Skipped: true}   // matches planUpload's D-13 gate
}
```

#### WR-03 (carried forward from the WR-01 fix, still open): the key-ceremony overflow backstop still overruns the frame whenever `budget <= 2` — the case its own comment says is routine

**File:** `internal/tuikit/identities.go:2980-3016`

The `budget-2` clamp is exact (see the Summary's derivation) **only while
`budget >= 3`**. Below that the `maxInt(1, …)` floor takes over and the
arithmetic stops holding:

```go
lines := maxInt(1, budget-2)   // budget <= 2  =>  lines == 1
```

giving `body_rows + lines + 2 = body_rows + 3` returned rows. With
`budget = B - rendered - 1` and `body_rows = rendered + 1`, `budget <= 2`
means `body_rows >= B - 2`, so the pane returns up to `B + 1` rows — one to
three rows past `frameBodyRows(minFrameHeight)`.

That is not a hypothetical corner: the backstop's own comment
(`:2971-2979`) says a real rotate's receipt "already lists 6+ written paths
and 3 backups before the upload beat's own long provider command lines even
start, routinely exceeding frameBodyRows(30)" — i.e. `rendered` at or above
`B = 25`, which is `budget <= 0`. And nothing downstream rescues it: unlike
Health (`health_screen.go:217`), Fixer (`fixer_screen.go:384`), Global SSH
(`globalssh.go:950`) and Global Git (`globalgit.go:1044`), the Identities
screen never calls `fitPane` — it hands the pane straight to
`joinMasterDetail` (`identities.go:5395`), whose `lipgloss.JoinHorizontal`
pads the *shorter* column up and never clips the taller one. So an overrun
here pushes the frame's status/footer rows off the terminal.

The base case is worse still: `if tail.Len() == 0 { return body }`
(`:2968-2970`) returns the receipt completely unbounded, so a long receipt
overflows the frame even with no upload beat at all.

**Fix:** clamp the whole pane, not just the tail, and give the Identities
screen the same `fitPane` safety net every other master-detail screen already
has:

```go
// identities.go:5395 — mirrors health_screen.go:217
body = joinMasterDetail(sidebar, sbWidth,
    fitPane(lipgloss.NewStyle().Width(detailWidth).Render(pane), frameBodyRows(height)),
    frameBodyRows(height))
```

`fitPane` already appends its own visible `"… (+n more lines)"` cue, so the
result degrades honestly instead of silently. Add a regression test that
forces `budget <= 0` (a receipt long enough on its own) and asserts the
rendered `sv.body` is `<= frameBodyRows(minFrameHeight)`; the existing
`TestKeyCeremonyOverflowBackstopStaysWithinFrameBudget` only exercises the
positive-budget branch.

#### WR-04 (new): the overflow backstop stacks two disagreeing truncation cues, advertises PgDn/PgUp keys nothing handles, and silently slices 4 columns off every clipped line

**File:** `internal/tuikit/identities.go:3012-3015`, `internal/tuikit/frame.go:806-864`

```go
lines := maxInt(1, budget-2)
v := ExactTextViewport{Text: wrapped, VisibleLines: lines, Width: maxInt(20, deleteChoiceNoteWidth-4)}
return body + "\n" + v.Clamp().View() + "\n " +
    styleFaint.Render(fmt.Sprintf("… %d more line(s) hidden", tailLines-lines)) + "\n"
```

Three distinct defects follow from reusing `ExactTextViewport` this way:

1. **Two cues, one of them a lie.** `ExactTextViewport.View()` renders its
   *own* navigation cue whenever content is hidden below
   (`frame.go:808-828`): `"↓ lines 7–19 of 19  PgDn↓ PgUp↑"`. That row is then
   immediately followed by the outer `"… N more line(s) hidden"` row. The
   viewport built here is a *local variable inside a render function* — it is
   never stored on the model, and `paneKeyCeremony`'s key handler has no
   `pgdown`/`pgup` case (the only `ScrollDown` callers in the file are the
   wizard's `w.proof`, `:3774` and `:3790`). So the pane tells the user to
   press PgDn and PgDn does nothing.
2. **Off-by-one hidden count.** Because the viewport reserves its last row for
   its own cue (`frame.go:833-835`, `contentRows = VisibleLines - 1`), only
   `lines-1` content rows are actually shown, while the outer cue reports
   `tailLines - lines` hidden. It always under-reports by exactly one.
3. **Silent horizontal truncation.** `wrapped` was word-wrapped to
   `deleteChoiceNoteWidth` (62) at `:2990`, and the viewport then
   horizontally slices each line to `deleteChoiceNoteWidth-4` (58)
   (`frame.go:846-851`, `sliceColumns`). Any line that used the full wrap
   width loses its last 4 columns. The viewport's own horizontal cue is
   suppressed here (`showHorizontalCue` requires
   `len(visible) < VisibleLines`, which is false in this branch), so the cut
   is invisible. A backstop whose entire purpose is "never truncate silently"
   truncates silently in the other axis.

**Fix:** stop borrowing the scrollable-viewport component for a
non-scrollable, non-focusable clip. Use `fitPane` (`frame.go:164-175`), which
is the file's existing helper for exactly this — clip to `n` rows, append one
visible cue, no phantom navigation affordance:

```go
if tailLines > budget {
    return body + "\n" + fitPane(wrapped, maxInt(2, budget))
}
```

That also removes the double cue and the off-by-one in one step. If the
viewport must be kept, at minimum pass `Width: deleteChoiceNoteWidth`
(matching the wrap width, so nothing is sliced) and compute the cue as
`tailLines - (lines - 1)`. The same three defects apply to the sibling
backstop in `renderUploadSection` (`:4784-4788`), which additionally uses a
*different* formula (`maxInt(1, budget)` against a `budget` that already
subtracted 2) for the identical viewport-plus-cue shape — see `IN-24`.

#### WR-05 (carried, DEFERRED design decision — plus a new, cheaply-fixable sub-defect): the D-08 register-key pane mutates the provider account on open, and abandoning it mid-flight says nothing

**File:** `internal/tuikit/identities.go:2669-2677`, `:2360-2372`, `:2683-2689`, `.planning/design/identity-manager/FIELDS.md:87`

Re-verified in source: `openRegisterKey` dispatches `RegisterKeyPlan`, and the
plan handler dispatches the upload the instant the probe answers `Ready`
(`:2368-2370`) — no D-01 checkbox, no confirm step. Two separate assessments:

**The design decision itself is correctly still open.** `FIELDS.md:87`
records it as an intentional contract ("opening the modal IS the explicit
opt-in"), it is restated in `TestIdentityManager_RegisterKeyModalRuns`'s doc
comment, and changing it means amending `FIELDS.md`, `design.go`'s frozen
copy, the visual-divergence allowlist and at least three real-binary e2e
tests. It also does not violate `CLAUDE.md`'s literal confirmation rule, which
is scoped to the user's local `~/.ssh/config` / `~/.gitconfig`. Route through
`/gsd-discuss-phase` or a tracked design amendment, as the previous review
said — not a mechanical fix, and **not blocking Phase 9's own goal**.

**But one concrete sub-defect inside it is not a design decision and was
missed by all three prior rounds.** `handleRegisterKeyKey`'s `esc` case
returns no note:

```go
// identities.go:2683-2689
case "esc":
    m.pane = paneDetail
    return keyResult{model: m, handled: true}
```

If the user presses Esc while `registerKeyPending` is true, the pane closes,
the in-flight `gh ssh-key add` runs to completion anyway, and its
`UploadRunMsg` is discarded by the `m.pane == paneRegisterKey` guard
(`:2376`). The user is told **nothing** — yet the wizard's byte-for-byte
identical situation was given a mitigation in the previous fix pass:
`wizardAbandonUploadNote` (`:4715-4738`), wired at `:3640` and `:3681`. The
asymmetry is arbitrary and the fix is three lines:

```go
case "esc":
    m.pane = paneDetail
    return keyResult{model: m, handled: true, note: registerKeyAbandonNote(m.registerKeyPending, m.registerKeyName)}
```

where `registerKeyAbandonNote` returns `""` unless a beat was in flight and
otherwise says a registration may have completed for that identity. (The
existing `wizardAbandonUploadNote` cannot be reused verbatim — it keys off
`run.Rows`, and here the run has not arrived yet — but the copy can be.)

#### WR-06 (carried, DEFERRED — correctly still open): the multi-line `ManualCommand` is interpolated into a single-line sentence

**File:** `cmd/gitid/upload_run.go:552-558`, `internal/tuikit/design.go:695`, `internal/tuikit/identities.go:2812`, `:2821`

Re-verified: `deleteCandidatesManualCommand` joins one preview per candidate
with `"\n"` (`upload_run.go:555-558`), and a rotated GitHub key always has two
(authentication + signing, separate REST namespaces). Both "leave it" result
paths interpolate that whole multi-line string into
`RotateDeleteOfferResultLeftFmt`'s single `%s`:

```go
// identities.go:2812 (esc) and :2821 (Enter on "Leave it")
m.rotateDeleteResult = fmt.Sprintf(RotateDeleteOfferResultLeftFmt, m.rotateDeleteOffer.ManualCommand)
```

The embedded newline splits the frozen sentence across rows mid-clause.

**Correctly deferred:** the fix is either an R22 frozen-copy amendment in
`design.go` (render the commands as an indented block under the sentence) or
a producer change to join with `" && "`. Both are deliberate copy/rendering
decisions. File as a tracked design/backlog item; **not blocking Phase 9's own
goal.**

#### WR-07 (carried, DEFERRED — correctly still open): the upload checkbox's actionable copy is truncated away at the only width production uses

**File:** `internal/tuikit/identities.go:4820-4828`, `:4835`, `internal/tuikit/design.go:582`, `:586`

Re-verified: `renderUploadCheckboxRow` is always called at `width = 60`
(`:4835`) and ends in `ansi.Truncate(line, width, "…")` (`:4828`). The frozen
unauth/disabled labels are longer than 60 columns, so the actionable half —
the `run "<tool> auth login"` remediation — is the part that gets cut. The
previous pass's mitigation added the `"…"` marker so the cut is at least
visible, which is the right interim step but not the fix.

**Correctly deferred:** shortening the label is an R22 frozen-copy amendment
(a copywriting decision), and widening the row breaks the wizard's one-line
row budget. File as a ROADMAP/backlog item; **not blocking Phase 9's own
goal.**

### Info

#### IN-01 (carried): `mustSeeSlow`'s rationale is stale after `FakeGHTrackAddedKeys`
**File:** `e2e/identity_manager_pty_e2e_test.go:997-1003` — still says the fixture "never reflects what was 'added'", which the stateful shim changed for the three opt-in tests. **Fix:** update the comment; the 20s budget stays justified by the chained offer probe.

#### IN-02 (carried): the fake-gh shim's `api -X DELETE` always succeeds and never mutates its own state
**File:** `e2e/harness_test.go:914`, `:929` — no test can assert "after the delete, the key is gone", and a delete addressing the wrong resource still reports success. **Fix:** branch on `-X DELETE`, drop the ID from the state file, exit 404-shaped for an unknown ID. This would also give `IN-16`'s retry path real coverage.

#### IN-03 (carried, now broader): the fake-gh shim reads argv positionally as `$3` and `$5`
**File:** `e2e/harness_test.go:894`, `:897` — both assume `buildArgs` keeps emitting `ssh-key add <path> --title <title> --type <type>` (`internal/uploader/uploader.go:315-324`). Fails closed but inscrutably. **Fix:** scan `"$@"` for `--title`; add a comment on `buildArgs` naming the shim as a positional consumer.

#### IN-04 (carried): frame-promote stamps HEAD as the "source commit" of frames captured earlier
**File:** `cmd/gitid-frame-promote/main.go:82-95`, `:140-141`. **Fix:** record the commit at capture time, or rename the column to "promoted at commit".

#### IN-05 (carried): `deleteArgs` silently ignores `reg` for GitLab
**File:** `internal/uploader/inventory.go:271-274` — the gh branch errors on an unsupported registration; the glab branch returns `ssh-key delete <id>` for any value, dropping the namespace guard `DeleteKey`'s doc comment calls load-bearing. Harmless today (`desiredRegistrations` only yields `Combined` for glab). **Fix:** error for anything other than `RegistrationCombined`.

#### IN-06 (carried): two doc comments detached from the functions they describe
**File:** `internal/tuikit/identities.go:4588-4598` — `renderUploadCheckboxRow`'s and `uploadRunHasContent`'s doc comments are stacked above `renderRegisterKey`; godoc attributes both to the wrong symbol. **Fix:** move each above its own declaration.

#### IN-07 (carried): `uploadFailureView`'s `provider` parameter is a hostname, sometimes empty
**File:** `cmd/gitid/wiring.go:1478-1484`, `:1579` — the not-found branch passes `""`, rendering `"Upload your public key to  as both…"`. **Fix:** rename to `hostname` and skip the manual block when empty.

#### IN-08 (carried): `fmt.Fprintln(w, fmt.Sprintf(...))` instead of `fmt.Fprintf`
**File:** `cmd/gitid/upload_run.go:292`, `:296`, `:301`, `:303`, `:305`; `cmd/gitid/identity_upload.go:201`. **Fix:** `fmt.Fprintf(w, fmt+"\n", …)`.

#### IN-09 (carried): frame-promote's header documents `PROVENANCE.md`; the tool writes `README.md`, and the registry count disagrees with its comment
**File:** `cmd/gitid-frame-promote/main.go:13`, `:34`, `:46`, `:92-95`. **Fix:** align all three.

#### IN-10 (carried): UTF-8-unsafe byte slice of a frozen format string in a test assertion
**File:** `cmd/gitid/identity_upload_test.go:393` — `tuikit.UploadResultFailedFmt[:5]` slices `"✗ %s key …"` at byte 5, yielding `"✗ %"`, which never appears in rendered output; the assertion only passes via its `||` fallback. **Fix:** assert on the rendered substring.

#### IN-11 (carried, mitigated): e2e children inherit the ambient environment, including real provider tokens
**File:** `e2e/harness_test.go:509-537` — `e2eEnv` builds `append(os.Environ(), …)`, so `GH_TOKEN`/`GITHUB_TOKEN`/`GH_CONFIG_DIR`/`GLAB_*` reach every child. Kept at Info: `ProviderDenyDir` is prepended to PATH and `ambientPathSentinelHit` hard-fails any caller that smuggles the ambient PATH ahead of it, so a real `gh` cannot resolve. **Fix (defense-in-depth):** blank the provider-credential variables inside `e2eEnv`, given this phase added a real `api -X DELETE` path.

#### IN-12 (carried): `u` is consumed on wizard step 0 even when the checkbox row is not rendered
**File:** `internal/tuikit/identities.go:3668` — the branch checks `w.focus > sshFieldPort` and the D-10 manual-path exclusion, but not `w.uploadRowVisible()`, so on a non-gated host `u` is swallowed with no effect. **Fix:** add `&& w.uploadRowVisible()`.

#### IN-13 (carried): `TestRegisterKeyExitContract` documents six cases, defines five
**File:** `cmd/gitid/identity_upload_test.go:201-205` — the comment says "six canned views … a nil error for the first five"; the table has five rows and the dry-run case named in the R10 exit contract is absent. **Fix:** add the row or correct the comment.

#### IN-14 (carried): `RotateDeleteOffer`'s doc comment still describes the pre-CR-01 matching
**File:** `cmd/gitid/wiring.go:1595-1598` — "by EXACT title equality … (`uploader.KeyTitle` + `uploader.FindByTitle`)" documents the behavior `OldKeyCandidates` replaced. **Fix:** point it at `OldKeyCandidates`.

#### IN-15 (carried): `stripLeadingNonJSONLines` only handles a *leading* banner, and a JSON-shaped stderr line is taken as the payload
**File:** `internal/uploader/inventory.go:150-159` — `RunCmd` returns `CombinedOutput`, whose interleaving is unordered: a stderr line flushed after or inside the JSON still corrupts the decode, and `{"level":"warn",…}` is accepted as the start of the payload. **Fix:** a separated-streams variant on `Deps` for parsing calls is the durable answer; file it rather than extending the line scanner.

#### IN-16 (carried): the retry narrowing does not treat "already absent" as success
**File:** `cmd/gitid/wiring.go:1649-1670` — a candidate that 404s because it was removed out-of-band is recorded in both `failed` and `remaining`, so its retry fails permanently and `"✓ Old key removed"` stays unreachable. **Fix:** classify a not-found delete response as success in the loop so it never enters `remaining`.

#### IN-17 (new): the new `WR-01` regression test can pass vacuously
**File:** `internal/tuikit/identity_manager_upload_test.go:761-793` — `TestKeyCeremonyOverflowBackstopStaysWithinFrameBudget` asserts only `lines <= budget`; it never asserts that the overflow branch was actually entered. A future change that lets its fixture tail fit turns it into a no-op that still passes. **Fix:** also assert the rendered output contains `"more line(s) hidden"`.

#### IN-18 (new): `extractUploadedTitle`'s `-t ` search is an unanchored substring scan
**File:** `internal/tuikit/identities.go:4676-4681` — `strings.Index(command, "-t ")` is not anchored to an argument boundary, so any earlier `-t ` (inside a quoted path, or a future flag) would be taken as the title flag. Not reachable with today's `buildArgs` shapes (`--type`/`--usage-type` do not contain `-t `), but it is a silent, guess-shaped failure if it ever is. **Fix:** require the match to be preceded by a space (or split on the quote-aware token boundary) before accepting it.

#### IN-19 (new): the key ceremony commits on `sel.Name` but dispatches, guards, notes and records actions on `m.selected`
**File:** `internal/tuikit/identities.go:2850`, `:2852`, `:2838` (`sel.Name`) vs `:2259`, `:2261`, `:2265`, `:2274`, `:2296` (`m.selected`) — `selectedIdentity` falls back to the first row when `m.selected` matches nothing (`:2167-2177`), so the two can diverge; the commit would then target one identity while the receipt, the emitted `RotateIdentity`/`NewKey` action and the upload beat all name another. No path today makes `m.selected` name a nonexistent row, so this is latent. **Fix:** use one source consistently — either set `m.selected = sel.Name` in `openKeyCeremony`, or read `sel.Name` throughout the ceremony handler.

#### IN-20 (new): dry-run derives the public path by string concatenation instead of using the staged one
**File:** `cmd/gitid/identity_create.go:450` — `PublicKeyPath: staged.FinalPrivatePath + ".pub"` where `staged.FinalPubPath` is right there and is what every other site uses. Equal today; a divergence in the staging naming would silently desync the preview from the write. **Fix:** use `staged.FinalPubPath`.

#### IN-21 (new): the `budget-2` comment mystifies a constant that is exactly derivable
**File:** `internal/tuikit/identities.go:3002-3012` — the comment says the value was "verified empirically against the actual render output, not derived on paper". It *is* derivable (see this report's Summary): the two reserved rows are the cue line and the trailing `"\n"`'s own row. Presenting it as a magic empirical number invites a future maintainer to re-tune it by trial and error. **Fix:** replace the comment with the one-line derivation, and name the two rows.

#### IN-22 (new): `RedactCLIOutput` misses newer GitLab token prefixes
**File:** `internal/uploader/classify.go:32-33` — `glabTokenPattern` covers `glpat-` only; GitLab also issues `gldt-` (deploy), `glrt-` (runner) and `glsoat-` (OAuth). Low impact because the function keeps only the first non-empty line, but the pattern is billed as defence in depth. **Fix:** widen to `gl[a-z]{2,6}-[A-Za-z0-9_-]{20,}`.

#### IN-23 (new): the fixture backend has no failure shape for `RunUploadForIdentity`
**File:** `internal/dummytui/fixturebackend.go:605-635` — `RunUpload` models a `partial.` failure host, but `RunUploadForIdentity` always succeeds, so `CR-02`'s `uploadSucceeded == false` gate (the one that suppresses the D-04 delete offer after a failed registration) has no demo or screenshot coverage at all. **Fix:** add a failing fixture host to `RunUploadForIdentity` mirroring `RunUpload`'s `partial.` branch.

#### IN-24 (new): the two sibling overflow backstops use different arithmetic for the identical viewport-plus-cue shape
**File:** `internal/tuikit/identities.go:3012` (`maxInt(1, budget-2)`, with `budget = B - rendered - 1`) vs `:4779` and `:4785` (`maxInt(1, budget)`, with `budget = B - rendered - 2`). Both are within budget today, by two different routes, with comments that cross-reference each other as "identical". **Fix:** factor the clip into one helper (see `WR-04`'s `fitPane` suggestion) so there is one formula, not two.

---

## Verification commands run

```
go build ./...                                                        # clean, exit 0
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...                 # 2373 passed / 22 pkgs, exit 0
make lint                                                             # 0 issues (untagged + screenshot-tagged), exit 0
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -count=1 -timeout 40m ./e2e/...
                                                                      # 175 passed, 0 failed, exit 0
TERM=dumb SSH_AUTH_SOCK= go test -tags screenshot -count=1 -timeout 30m \
    ./internal/screenshot/... ./cmd/gitid/...                         # 832 passed, 1 failed, exit 0
                                                                      # TestCaptureTUI: "freeze binary not found on PATH"
                                                                      # — environmental, unchanged from prior iterations
```

Note on the tagged suites: both need a generous `-timeout`. At the default
10m per-package budget the e2e package aborts mid-suite with
`panic: test timed out`, on a different test each run — a package-wide
timeout artifact, not a test failure. `-timeout 40m` / `-timeout 30m`
complete cleanly.

---

_Reviewed: 2026-08-31T04:12:44Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
_Iteration: 5 (supersedes iteration 4; prior content preserved at 09-REVIEW.iter3.md)_
