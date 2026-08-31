---
phase: 09-upload-credentials-assist
reviewed: 2026-08-31T02:33:51Z
depth: standard
iteration: 4
supersedes: iteration 3 (2026-08-29T18:35:00Z, backed up at 09-REVIEW.iter2.md)
head: df0729a
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
  critical: 1
  warning: 7
  info: 16
  total: 24
status: issues_found
---

# Phase 9: Code Review Report (re-review, iteration 4 — post-fix-pass verification)

**Reviewed:** 2026-08-31T02:33:51Z
**Depth:** standard
**Files Reviewed:** 51
**Status:** issues_found

## Summary

Fourth pass, scoped to verifying the iteration-3 fix pass (`6b5bf7b..df0729a`,
10 fix commits, 20 Go files, +683/-68) and re-assessing everything it left
open. Each of the 11 "fixed" warnings was checked against the merged source,
not against the fix report; each of the 14 Info items was re-read; the full
gate battery was run from scratch.

**Nine of the eleven claimed fixes are genuinely and correctly applied**
(WR-02, WR-04, WR-06, WR-07, WR-09, WR-12, WR-14, and the two halves of
WR-08's clamp condition). One — WR-05 — is only half-applied, on the wrong
half. One — WR-01 — is **correct in the production backend and broken in
every fixture backend**, and it took the phase's own gate battery red.

**CR-01 is a genuine, bisected regression introduced by the fix pass.**
`cdc6932` added the `UploadRunMsg.Name` correlation guard to both consumers
but only set `Name` on the *real* backend's dispatch sites.
`dummytui.FixtureBackend.RunUpload`/`RunUploadForIdentity` and
`screenshot.offlineCaptureBackend.RunUpload` still emit `Name: ""`, so every
fixture reply is now silently discarded by the new guard. Consequences:
`gitid-dummy`'s D-08 register-key pane hangs on "Registering…" forever, its
D-04 rotate-delete offer is never dispatched at all, and eleven visual-gate
tests plus three e2e tests fail. I bisected this across all ten fix commits
(table in CR-01) — it is red from `cdc6932` onward and green at `6b5bf7b`.

**The fix report's gate claims are not reliable** (WR-04 below). It states
`TestRegionDiffCoverage` was "pre-existing before this change" and
"confirmed via A/B stash comparison"; the bisect proves it was introduced by
the fixer's own first commit. The report's three green gates (`go build`,
`go test -race ./...`, `make lint`) are all real — but none of the three
covers `-tags screenshot` or `-tags e2e`, which is exactly where the damage
landed. Both tagged gates must be part of the fixer's own close-out from now
on.

**Three genuinely new defects were introduced by the fixes themselves**
beyond CR-01: WR-01 (WR-08's new truncation cue overruns the very row budget
the backstop exists to enforce, and contradicts the viewport's own built-in
cue), WR-02 (WR-05 quoted `RunUpload` but not `RunUploadForIdentity` — the
`register-key-modal` surface the finding explicitly named), WR-03
(WR-13's `extractUploadedTitle` reports the *pub path* as the title whenever
`$HOME` contains a space).

**WR-03/WR-10/WR-11 from iteration 3 are correctly still open.** I verified
each is a frozen-copy or sequencing-contract decision, not a bug: WR-03's
"opening the modal IS the explicit opt-in" is a tracked contract in
`.planning/design/identity-manager/FIELDS.md:87`, and WR-10/WR-11 both
require a `design.go` R22 amendment. They are carried below as WR-05/WR-06/
WR-07 with an explicit **do-not-mechanically-fix** marker. They do **not**
block Phase 9's own goal; CR-01 does.

**None of the 14 Info items warrants promotion.** IN-11 (ambient provider
tokens reaching e2e children) was the strongest candidate, but `e2eEnv`
prepends a `ProviderDenyDir` shim and `ambientPathSentinelHit` fails the test
if a caller smuggles the ambient PATH back in front of it, so a real `gh`
cannot resolve. It stays Info as defense-in-depth. IN-14 is now partially
stale (`titleMatchesThisMachine` was unexported with an accurate comment;
`FindByTitle` gained a redirecting doc comment and a real e2e caller) — only
`wiring.go:1596`'s stale prose remains. Two new Info items were added.

## Gate battery (run by this reviewer, from a clean tree at df0729a)

```
go build ./...                                                         # clean
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...                  # 2364 passed, 22 pkgs, exit 0
make lint                                                              # golangci-lint 0 issues (both untagged and screenshot-tagged)
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e ./e2e/                      # 172 passed, 3 FAILED
TERM=dumb SSH_AUTH_SOCK= go test -tags screenshot ./internal/screenshot/... ./cmd/gitid/...
                                                                       # 112 passed, 2 FAILED (screenshot)
                                                                       # + 9 FAILED (cmd/gitid visual gate)
                                                                       # 1 of the 2 screenshot failures is environmental (TestCaptureTUI: freeze not on PATH)
```

All 13 non-environmental failures share one root cause: CR-01.

## Fix Verification (iteration-3 warnings)

| Finding | Claim | Verdict |
|---|---|---|
| WR-01 | `UploadRunMsg.Name` + guards | **Regression** — real backend correct, all three fixture backends left at `Name: ""`. See CR-01. |
| WR-02 | fixture records real `--title` | **Correct** — `harness_test.go:894,897` use `"$5"`; argv is `["ssh-key","add",pub,"--title",title,…]`, so `$5` is the title. |
| WR-04 | frame captures routed via `saveFrame` | **Correct** — all three helpers call `saveFrame`; working tree stays clean across a full e2e run. |
| WR-05 | dummy previews shell-quoted | **Half-applied** — `RunUpload` quoted, `RunUploadForIdentity` (the `register-key-modal` surface) not. See WR-02. |
| WR-06 | dry-run help discloses probes | **Correct** — `dryRunProbeClause` appended via two shared constants, bound on all four verbs (`identity_create.go:93`, `identity_clone.go:49`, `identity_key.go:43,62`). |
| WR-07 | `glabInventory` page cap | **Correct** — `glabMaxPages = 40`, loop bounded, named error on exhaustion (`inventory.go:83-107`). |
| WR-08 | clamp unconditionally | **Correct condition, new overflow** — the `budget > 0` guard is gone at both sites, but the added cue line is unaccounted for in `renderKeyCeremony`'s budget. See WR-01. |
| WR-09 | `RemainingKeyID` narrows retry | **Correct** — every early-return path carries `RemainingKeyID`, encode-failure falls back to the full set explicitly (`wiring.go:1624-1670`). |
| WR-12 | strip leading non-JSON | **Correct for the accepted minimum** — `stripLeadingNonJSONLines` (`inventory.go:150-159`); residual noted in IN-15. |
| WR-13 | abandon note carries D-07 title | **Applied, brittle extraction** — both step-0 exits carry the note; the title parser is wrong for quoted paths. See WR-03. |
| WR-14 | stage the derived `.pub` | **Correct** — reuse path writes `staged.PubLine` into `stagingDir()` under `filepath.Base(FinalPrivatePath)+".pub"`, never `TempPrivatePath`/`~/.ssh` (`upload_run.go:100-123`). |

## Narrative Findings (AI reviewer)

### Critical Issues

#### CR-01 (BLOCKER, new — regression from `cdc6932`): the `UploadRunMsg.Name` stale-guard was added to the consumers but not to any fixture backend, permanently breaking `gitid-dummy`'s D-08 and D-04 surfaces and taking 13 gate tests red

**File:** `internal/dummytui/fixturebackend.go:503`, `:617`;
`internal/screenshot/createflow.go:1113`; consumers at
`internal/tuikit/identities.go:2274`, `:2376`

**Issue:** `cdc6932` correctly tightened both consumers:

```go
// identities.go:2274
if run, ok := msg.(UploadRunMsg); ok && m.pane == paneKeyCeremony && m.keyCeremonyUploadPending && run.Name == m.selected {
// identities.go:2376
if run, ok := msg.(UploadRunMsg); ok && m.pane == paneRegisterKey && m.registerKeyPending && run.Name == m.registerKeyName {
```

and set `Name` at all six real-backend dispatch sites (`wiring.go:1513-1586`).
It did **not** touch the fixture backends, which still emit:

```go
// dummytui/fixturebackend.go:617 — RunUploadForIdentity
return tuikit.UploadRunMsg{View: view}          // Name == ""
// dummytui/fixturebackend.go:503 — RunUpload
return tuikit.UploadRunMsg{View: view}          // Name == ""
// screenshot/createflow.go:1113 — offlineCaptureBackend.RunUpload
return func() tea.Msg { return tuikit.UploadRunMsg{View: view} }
```

`openRegisterKey` sets `m.registerKeyName = sel.Name` (`identities.go:2671`)
and the ceremony path dispatches `RunUploadForIdentity(m.selected)`
(`:2259`); both are non-empty fixture identity names ("personal"/"work").
`"" == "personal"` is false, so **every fixture `UploadRunMsg` is now
discarded**. `offlineCaptureBackend` embeds rather than overrides
`RunUploadForIdentity`, so it inherits the broken fixture path too.

Reachable consequences, all deterministic:

1. `gitid-dummy`'s D-08 register-key pane never leaves
   `renderRegisterKey`'s `"Registering…"` branch (`identities.go:4612-4617`)
   — `m.registerKeyRun` is never assigned, so `uploadRunHasContent` stays
   false forever. This is a shipped demo binary and the project's
   design/screenshot source of truth.
2. The D-04 rotate-delete offer is dispatched from *inside* the discarded
   block (`identities.go:2293-2297`), so the dummy's rotate ceremony never
   reaches the offer at all, and `keyCeremonyPhase` never advances to
   `"review"`.
3. The in-process visual gate loses the `register-key-modal` frame's
   `upload-section` region in **both** evidence sets.

Bisected across all ten fix commits with
`go test -tags screenshot ./internal/screenshot/ -run TestRegionDiffCoverage`:

```
6b5bf7b :: ok       (pre-fix-pass HEAD)
cdc6932 :: FAIL     <- WR-01 fix
532a9cc :: FAIL     ... every subsequent fix commit FAIL through dbcaf78
```

and confirmed at the PTY level: `TestRegisterKeyModal_CompiledRealVsLiveDummyPTY`
passes at `cdc6932^` (3/3) and fails at HEAD (0/3). Observed failures:

```
e2e:        TestRegisterKeyModal_CompiledRealVsLiveDummyPTY/register-key-modal
              -> dummy: "Running:" never appeared
            TestRegisterKeyModal_CompiledRealVsLiveDummyPTY/rotate-delete-offer
              -> dummy: "Remove the old key from GitHub?" never appeared
            TestRegisterKeyModal_CompiledRealVsLiveDummyPTY  (5 stale-allowlist errors)
screenshot: TestRegionDiffCoverage
              -> frame "register-key-modal" required region "upload-section" is empty in live evidence
cmd/gitid:  9 x TestNegativeControl_* (identity-manager, global SSH, global Git, health/fixer, upload)
              -> frame "register-key-modal" required region "upload-section" is empty in approved-tui evidence
```

**Fix:** set `Name` at every fixture dispatch site, exactly as the real
backend does:

```go
// internal/dummytui/fixturebackend.go — RunUploadForIdentity
return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
    return tuikit.UploadRunMsg{Name: name, View: view}
})

// internal/dummytui/fixturebackend.go — RunUpload
return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
    return tuikit.UploadRunMsg{Name: spec.Identity, View: view}
})

// internal/screenshot/createflow.go — offlineCaptureBackend.RunUpload
return func() tea.Msg { return tuikit.UploadRunMsg{Name: spec.Identity, View: view} }
```

Then add the anti-drift guard this class of defect needs, because the
existing dummy assertions only check the *announce* line and never the
result rows: extend `TestIdentityManager_RegisterKeyModalRuns`' dummy sibling
(and `TestRegisterKeyModal_CompiledRealVsLiveDummyPTY`'s dummy leg) with
`mustSee(t, dummy, "Authentication key registered", …)` so a dropped fixture
reply fails loudly instead of silently degrading to "Registering…". Re-run
both tagged gates before closing.

### Warnings

#### WR-01 (new, introduced by `354769e`): WR-08's truncation cue pushes `renderKeyCeremony` one row past the frame budget the backstop exists to enforce, and contradicts the viewport's own built-in cue

**File:** `internal/tuikit/identities.go:3001-3007` (`renderKeyCeremony`),
`:4732-4736` (`renderUploadSection`), vs
`internal/tuikit/frame.go:806-860` (`ExactTextViewport.View`)

**Issue:** Two problems, both from the added cue line.

1. **Budget overrun.** `renderKeyCeremony` computes
   `budget := frameBodyRows(minFrameHeight) - rendered - 1` (`:2981`) — a
   single reserved row for the `"\n"` separator — and previously returned
   `body + "\n" + v.Clamp().View() + "\n"`, i.e. exactly `budget` tail rows,
   exactly filling the 25-row body. The fix keeps `VisibleLines: budget` and
   then appends an *additional* row:

   ```go
   return body + "\n" + v.Clamp().View() + "\n " +
       styleFaint.Render(fmt.Sprintf("… %d more line(s) hidden", tailLines-lines)) + "\n"
   ```

   That is `budget + 1` rows — the frame overflow the backstop was written to
   prevent, now in the *common* positive-budget path that was previously
   correct. `renderUploadSection` is unaffected only by accident: its own
   budget uses `- rendered - 2` (`:4727`), so the second reserved row absorbs
   the cue. The two sites are now silently asymmetric.

2. **Duplicate, disagreeing cue.** `ExactTextViewport.View()` already emits
   its own truncation cue and *reserves the final row for it*
   (`frame.go:840-844`: `contentRows = v.VisibleLines - 1` whenever
   `cueLine != ""`). So the viewport renders `budget-1` content rows plus its
   own `"↓ lines N–M of T  PgDn↓ PgUp↑"` line. The new external cue is
   computed as `tailLines - budget`, but the number actually hidden is
   `tailLines - (budget-1)`. The two cues sit adjacent and disagree by
   exactly one line.

The fixer's A/B check found "byte-identical output" precisely because the
tracked rotate scenarios never enter this branch (`tailLines <= budget`), so
neither problem has coverage.

**Fix:** reserve the cue's row, or drop the redundant cue entirely and rely
on the viewport's:

```go
if tailLines > budget {
    lines := maxInt(1, budget-1)             // reserve the cue's own row
    v := ExactTextViewport{Text: wrapped, VisibleLines: lines, Width: maxInt(20, deleteChoiceNoteWidth-4)}
    return body + "\n" + v.Clamp().View() + "\n " +
        styleFaint.Render(fmt.Sprintf("… %d more line(s) hidden", tailLines-lines)) + "\n"
}
```

and add a test that renders the branch with a positive budget and asserts
`strings.Count(out, "\n") + 1 <= frameBodyRows(minFrameHeight)` — the
invariant the backstop actually owes, which nothing currently pins.

#### WR-02 (new, `be319ab` half-applied): the dummy backend's *register-key* preview commands are still unquoted — the exact surface iteration-3's WR-05 named

**File:** `internal/dummytui/fixturebackend.go:601-603`

**Issue:** WR-05's fix quoted `FixtureBackend.RunUpload` (`:480-482`) and
`offlineCaptureBackend.RunUpload`, but left the sibling untouched:

```go
func (b FixtureBackend) RunUploadForIdentity(name string) tea.Cmd {
	title := fmt.Sprintf(tuikit.UploadKeyTitleFmt, name, "demo-machine")
	...
	authCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title %s --type authentication", keyPath, title)
	signCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title %s --type signing", keyPath, title)
	glabCmd := fmt.Sprintf("/usr/local/bin/glab ssh-key add %s.pub -t %s --usage-type auth_and_signing", keyPath, title)
```

`title` is `gitid: personal @ demo-machine` — three spaces. `RunUploadForIdentity`
is the method that feeds the D-08 register-key pane and the key-ceremony
upload beat, i.e. the `register-key-modal` frame the iteration-3 finding
called out by name ("every demo frame and every promoted
`register-key-modal`/upload screen shows a command that, pasted into a shell,
does the wrong thing"). The half that was fixed serves the create wizard;
the half that was named is still broken. The real backend routes both through
`uploader.previewLine`/`shellQuote` (`uploader.go:291-313`), so the fixture is
now internally inconsistent as well as wrong.

Note this is currently masked: CR-01 means the dummy never renders these
commands at all. Fixing CR-01 without fixing this will surface the unquoted
previews in the promoted frames.

**Fix:** apply the same hardcoded-quoted shape `RunUpload` now uses:

```go
authCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title '%s' --type authentication", keyPath, title)
signCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title '%s' --type signing", keyPath, title)
glabCmd := fmt.Sprintf("/usr/local/bin/glab ssh-key add %s.pub -t '%s' --usage-type auth_and_signing", keyPath, title)
```

and re-promote the affected frames after CR-01 is closed.

#### WR-03 (new, introduced by `0dc56e3`): WR-13's abandon note reports the **pub path** as the registered title whenever any earlier argv element needs quoting

**File:** `internal/tuikit/identities.go:4655-4661` (`extractUploadedTitle`),
`:4675-4686` (`wizardAbandonUploadNote`), vs
`internal/uploader/uploader.go:291-320`

**Issue:** `extractUploadedTitle` takes the text between the *first* pair of
single quotes:

```go
func extractUploadedTitle(command string) string {
	parts := strings.SplitN(command, "'", 3)
	if len(parts) < 3 { return "" }
	return parts[1]
}
```

Its doc comment asserts the title is "the ONLY quoted argument … (the key
path and flags normally do not [contain spaces])". `shellQuote` quotes **any**
argument containing `space \t " ' \ $` or a backtick
(`uploader.go:292`), and `buildArgs` places `pubPath` **before**
`--title title` (`uploader.go:315-320`). So:

- `$HOME` containing a space (`/Users/John Smith/.ssh/id_ed25519_work.pub`,
  a routine macOS/Windows account name) → `pubPath` is quoted first and the
  note tells the user to remove a provider key "under the title
  `/Users/John Smith/.ssh/id_ed25519_work.pub`" — a title that does not exist.
- an identity name containing `'` → `shellQuote` escapes it as `'\''`, and
  `SplitN(…, "'", 3)` returns the truncated prefix (`gitid: o` for
  `o'brien`).
- a `toolPath` containing a space (a Homebrew prefix under a spaced HOME,
  `/Users/John Smith/.local/bin/gh`) → the quoted tool path is returned.

This is the *one* piece of information the mitigation exists to deliver: the
user has an orphaned remote key and this note is the only pointer to it.
Emitting a confidently-wrong title is worse than the generic fallback the
function already has.

**Fix:** parse by flag position, not by quote position, and fall back to the
generic sentence rather than to a wrong string:

```go
// extractUploadedTitle returns the operand following --title (gh) or -t (glab).
func extractUploadedTitle(command string) string {
	fields := strings.Fields(command)
	for i, f := range fields {
		if (f == "--title" || f == "-t") && i+1 < len(fields) {
			return strings.Trim(strings.Join(fields[i+1:], " "), "'")
		}
	}
	return ""
}
```

Better still, carry the title on `UploadResultRow` (the backend already
computes it via `uploader.KeyTitle`) instead of re-parsing a rendered
display string at all — the row is the correct carrier and removes the
quoting coupling entirely. Add a regression whose `row.Command` has a quoted
pub path preceding the title.

#### WR-04 (new): the fix report asserts a gate result that the bisect disproves, and its close-out never ran the two tagged gates where the damage landed

**File:** `.planning/phases/09-upload-credentials-assist/09-REVIEW-FIX.md:29-33`,
`:62`

**Issue:** The report's WR-05 entry states:

> two pre-existing unrelated failures — `TestCaptureTUI`, environmental, and
> `TestRegionDiffCoverage`, pre-existing before this change — confirmed via
> A/B stash comparison

`TestRegionDiffCoverage` was **not** pre-existing. It is green at `6b5bf7b`
(the pre-fix-pass HEAD) and red from `cdc6932` onward — the fixer's own first
commit, five commits before the one the claim is attached to. The A/B stash
comparison was performed against `be319ab`'s file only, which cannot detect a
break introduced by an earlier commit in the same pass; the conclusion drawn
from it ("pre-existing", therefore ignorable) is unsupported.

The report's three green gates are all genuine and I reproduced them, but
none of them covers `-tags screenshot` or `-tags e2e`:

```
go build ./...                                        # true
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./... # true (untagged only)
make lint                                             # true
```

That gap is exactly how 13 red tests shipped as `status: partial`. The
project's own memory note *review-gate-race-vs-executor-nonrace* records this
same failure mode for executors; it now applies to the fixer.

**Fix:** correct the WR-05 entry, and make the fixer's close-out battery
mandatory and complete:

```
go build ./...
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...
TERM=dumb SSH_AUTH_SOCK= go test -tags screenshot ./internal/screenshot/... ./cmd/gitid/...
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e ./e2e/...
make lint
```

Treat "pre-existing" as a claim requiring a bisect against the pass's *base*
commit, never against a single stashed file.

---

The three findings below are **deferred by design decision, not defects to
fix mechanically.** They were correctly skipped by the fix pass and they do
not block Phase 9's own goal (upload/credentials-assist working correctly).
A fixer should route them to `/gsd-discuss-phase` or a tracked design
amendment, and must not attempt a code change. They are recorded as Warnings
only so they stay visible until they are filed.

#### WR-05 (carried from iteration 3 WR-03 — DEFERRED, do not mechanically fix): the D-08 register-key pane performs a real remote mutation on open, with no confirmation and no cancel

**File:** `internal/tuikit/identities.go:2528-2534`, `:2650-2662`,
`:2360-2370`, `:2665-2675`

**Issue:** Unchanged and confirmed still present. Pressing `u` opens
`paneRegisterKey`, whose resolved plan dispatches `RunUploadForIdentity`
unconditionally — a real `gh/glab ssh-key add` — with no preview-and-confirm
beat, and `handleRegisterKeyKey` accepts only `esc`, which does not cancel
the in-flight command.

**Why it stays open:** the sequencing is a *tracked contract*, verified at
`.planning/design/identity-manager/FIELDS.md:87`:

> the D-01 checkbox is OMITTED here — opening the modal IS the explicit
> opt-in, so it announces-and-runs immediately when the provider matches and
> the tool is authenticated

restated in `TestIdentityManager_RegisterKeyModalRuns`' own doc comment
(`e2e/identity_manager_pty_e2e_test.go:878-879`). It genuinely tensions with
CLAUDE.md's "only with user confirmation" rule, and the mutation is additive
and reversible, but changing it means amending FIELDS.md, the frozen copy in
`design.go`, the visual allowlist, and at least three real-binary e2e tests.

**Fix:** file as a design amendment (ROADMAP/backlog item), not a code change.

#### WR-06 (carried from iteration 3 WR-10 — DEFERRED, do not mechanically fix): the multi-line `ManualCommand` is interpolated into a single-line sentence

**File:** `cmd/gitid/upload_run.go:552-558`, `internal/tuikit/design.go:695`,
`internal/tuikit/identities.go:2812`, `:2821`

**Issue:** Unchanged. `deleteCandidatesManualCommand` joins one preview per
candidate with `"\n"`; `RotateDeleteOfferResultLeftFmt = "Left in place —
remove it yourself: %s"` renders the second command at column 0 with no
leading space and no context. A rotated GitHub key always has two
registrations, so the two-line case is the *normal* one.

**Fix:** requires a `design.go` R22 frozen-copy amendment (indented block, or
`" && "` join). File it; do not patch the format string in isolation.

#### WR-07 (carried from iteration 3 WR-11 — DEFERRED, do not mechanically fix): the upload checkbox's actionable copy is unreadable at the only width production uses

**File:** `internal/tuikit/identities.go:4747`, `:4779-4783`,
`internal/tuikit/design.go:582`, `:586`

**Issue:** Unchanged and confirmed: `renderUploadCheckboxRow` is called at a
hardcoded width of 60 (`:4783`), while `UploadCheckboxLabelUnauthFmt` renders
~106 columns, so the entire remediation clause (`run "gh auth login" first,
or check anyway`) is truncated away, and `UploadCheckboxLabelDisabledFmt`
loses `Manual steps are shown after create.`

**Fix:** requires amending frozen copy to fit 60 columns (a copywriting
decision) or widening the row, both R22 amendments. Still tracked only inside
review/fix reports — file it as a ROADMAP/backlog item so it stops recurring.

### Info

#### IN-01 (carried): `mustSeeSlow`'s rationale is stale after `FakeGHTrackAddedKeys`

**File:** `e2e/identity_manager_pty_e2e_test.go:997-1003`

Still says the fixture "never reflects what was 'added'" — exactly what the
stateful shim changed for the three tests that opt in. **Fix:** update the
comment; the 20s budget remains justified by the chained offer probe.

#### IN-02 (carried): the fake-gh shim's `api -X DELETE` always succeeds and never mutates its own state

**File:** `e2e/harness_test.go:914,929`

`delete-ok` answers every `api` invocation — including
`-X DELETE user/keys/555` — with the inventory fixture and exit 0. No test can
assert "after the delete, the key is gone", and a delete addressing the wrong
resource still reports success. **Fix:** branch on `-X DELETE`, remove the ID
from the state file, exit 404-shaped for an unknown ID — which would also give
WR-09's retry path real coverage.

#### IN-03 (carried, now broader): the fake-gh shim reads argv positionally as `$3` and `$5`

**File:** `e2e/harness_test.go:894`, `:897`

`cat "$3"` and `"$5"` both assume `buildArgs` keeps emitting
`ssh-key add <path> --title <title> --type <type>`. The coupling is now
load-bearing for CR-01's blob-exclusion coverage and invisible from
`uploader.buildArgs` (`uploader.go:315-324`). It fails closed, but
inscrutably. **Fix:** scan `"$@"` for `--title`/the first non-flag operand,
and add a comment on `buildArgs` naming the shim as a positional consumer.

#### IN-04 (carried): frame-promote stamps HEAD as the "source commit" of frames captured earlier

**File:** `cmd/gitid-frame-promote/main.go:82-95`, `:140-141`

`gitHead(root)` is read at promote time; the frames come from a
`tmp/ui-frames/` capture that may predate HEAD arbitrarily. The SHA-256
column delivers the stated guarantee; the commit column can misattribute.
**Fix:** record the commit at capture time, or rename the column to
"promoted at commit".

#### IN-05 (carried): `deleteArgs` silently ignores `reg` for GitLab

**File:** `internal/uploader/inventory.go:273-274`

The gh branch errors on an unsupported registration; the glab branch returns
`ssh-key delete <id>` for *any* value including `Authentication`/`Signing`.
Harmless today (`desiredRegistrations` only produces `Combined` for glab) but
it removes the namespace guard `DeleteKey`'s doc comment calls load-bearing.
**Fix:** return an error for anything other than `RegistrationCombined`.

#### IN-06 (carried): doc comments detached from the functions they describe

**File:** `internal/tuikit/identities.go:4578-4592`

`renderUploadCheckboxRow`'s and `uploadRunHasContent`'s doc comments still sit
stacked above `renderRegisterKey`; godoc attributes both to the wrong symbol.
**Fix:** move each above its own declaration.

#### IN-07 (carried): `uploadFailureView`'s `provider` parameter is a hostname, sometimes empty

**File:** `cmd/gitid/wiring.go:1478-1484`, `:1579`

The not-found branch still passes `""`, rendering `"Upload your public key to
 as both an authentication key and a signing key,"`. **Fix:** rename the
parameter to `hostname` and skip the manual block when empty.

#### IN-08 (carried): `fmt.Fprintln(w, fmt.Sprintf(...))` instead of `fmt.Fprintf`

**File:** `cmd/gitid/upload_run.go`, `cmd/gitid/identity_upload.go` — 7 sites
remain. **Fix:** `fmt.Fprintf(w, fmt+"\n", …)`.

#### IN-09 (carried): frame-promote's header documents `PROVENANCE.md`; the tool writes `README.md`, and the registry count disagrees with its comment

**File:** `cmd/gitid-frame-promote/main.go:13`, `:34`, `:46`, `:92-95`

The package comment promises `ui-frames/PROVENANCE.md` while `run()` writes
`README.md` (the error string still says "writing PROVENANCE table"), and the
comment says "twelve new PTY tests plus the two pre-existing" (14) for a
13-row table. **Fix:** align all three.

#### IN-10 (carried): UTF-8-unsafe byte slice of a frozen format string in a test assertion

**File:** `cmd/gitid/identity_upload_test.go:393`

`tuikit.UploadResultFailedFmt[:5]` slices `"✗ %s key …"` at byte 5, yielding
`"✗ %"`, which never appears in rendered output; the assertion passes only via
its `||` fallback. **Fix:** assert on the rendered substring.

#### IN-11 (carried, mitigated): e2e children inherit the ambient environment, including real provider tokens

**File:** `e2e/harness_test.go:509-537`

`e2eEnv` still builds `append(os.Environ(), …)`, so `GH_TOKEN`,
`GITHUB_TOKEN`, `GH_CONFIG_DIR`, `GLAB_*` reach every child. Re-examined for
promotion this iteration and deliberately kept at Info: `ProviderDenyDir` is
prepended to PATH and `ambientPathSentinelHit` hard-fails any caller that
smuggles the ambient PATH ahead of it, so a real `gh` cannot resolve — the
tokens are unreachable in practice. **Fix (defense-in-depth):** blank the
provider-credential variables inside `e2eEnv` so hermeticity does not rest on
the deny shim alone, given this phase added a real `api -X DELETE` path.

#### IN-12 (carried): `u` is consumed on wizard step 0 even when the checkbox row is not rendered

**File:** `internal/tuikit/identities.go:3654`

The branch checks `w.focus > sshFieldPort` (plus the D-10 manual-path
exclusion) but not `w.uploadRowVisible()`, so on a non-gated host `u` is
swallowed with no effect. **Fix:** add `&& w.uploadRowVisible()`.

#### IN-13 (carried): `TestRegisterKeyExitContract` documents six cases, defines five

**File:** `cmd/gitid/identity_upload_test.go:202-205`

The comment still says "six canned views … a nil error for the first five";
the table has five rows and the dry-run case named in the R10 exit contract is
absent. **Fix:** add the row or correct the comment.

#### IN-14 (carried, partially stale): `RotateDeleteOffer`'s doc comment still describes the pre-CR-01 matching

**File:** `cmd/gitid/wiring.go:1596`

Two thirds of this item are now resolved: `titleMatchesThisMachine` is
unexported with an accurate "no production caller" note
(`uploader.go:271-277`), and `FindByTitle` gained a doc comment redirecting
destructive callers to `OldKeyCandidates` plus a genuine e2e caller
(`e2e/upload_real_account_e2e_test.go:510,515`). What remains is the stale
prose at `wiring.go:1596` — "by EXACT title equality … (`uploader.KeyTitle` +
`uploader.FindByTitle`)" — which documents the behavior CR-01 replaced.
**Fix:** point it at `OldKeyCandidates`.

#### IN-15 (new): `stripLeadingNonJSONLines` only handles a *leading* banner, and a stderr line starting with `{` is taken as the payload

**File:** `internal/uploader/inventory.go:150-159`

WR-12's fix implements the review's explicitly-accepted minimum, so this is
not a re-open — just the residual. `RunCmd` returns `CombinedOutput`, whose
interleaving is not ordered: a stderr line flushed *after* or *inside* the
JSON still corrupts the decode, and a JSON-shaped stderr log line
(`{"level":"warn",…}`) is silently accepted as the start of the payload,
producing a parse error whose discarded-prefix diagnostic is then empty.
**Fix:** the larger option the review named — a separated-streams variant on
`Deps` for parsing calls — remains the durable answer; file it rather than
extending the line-scanner.

#### IN-16 (new): WR-09's retry narrowing does not treat "already absent" as success

**File:** `cmd/gitid/wiring.go:1641-1670`

The review offered two remedies and the fixer implemented one
(`RemainingKeyID`), which correctly fixes the partial-delete retry it was
raised for. The other half is still open: a candidate that returns 404
because it was removed out-of-band is recorded in both `failed` and
`remaining`, so its retry fails permanently and `"✓ Old key removed"` stays
unreachable. **Fix:** classify a not-found delete response as success in the
loop so it never enters `remaining`.

---

## Verification commands run

```
go build ./...                                                          # clean
TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...                   # 2364 passed / 22 pkgs
make lint                                                               # 0 issues (untagged + screenshot)
TERM=dumb SSH_AUTH_SOCK= go test -tags e2e ./e2e/                       # 172 passed, 3 FAILED (CR-01)
TERM=dumb SSH_AUTH_SOCK= go test -tags screenshot ./internal/screenshot/... ./cmd/gitid/...
                                                                        # 11 FAILED (CR-01) + 1 environmental
git worktree bisect over 6b5bf7b..dbcaf78 x TestRegionDiffCoverage       # green at 6b5bf7b, red from cdc6932
git worktree at cdc6932^ x TestRegisterKeyModal_CompiledRealVsLiveDummyPTY  # 3 passed (red at HEAD)
```

---

_Reviewed: 2026-08-31T02:33:51Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard — iteration 4 (independent re-review after 09-REVIEW-FIX.md's 10-commit fix pass)_
