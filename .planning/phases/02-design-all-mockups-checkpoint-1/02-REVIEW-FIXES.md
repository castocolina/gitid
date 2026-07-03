# 02-REVIEW-FIXES.md — consolidated fix pass (4 reviews)

Fixes every finding from `REVIEW.md` (internal Go review), the Codex cross-vendor
review, the `agent-ui-ux-designer` parity critique, and `SECURITY.md` (security
audit) — the final pass before the 02-12 human design-approval checkpoint. Branch:
`gsd/phase-02-design-all-mockups-checkpoint-1`. No user-file mutation anywhere in
this pass; the dummy stays DLV-05 no-backend the whole time.

Each entry: **finding → fix → proof** (a real command + its real output, not
narrative).

---

## A. Go / TUI dummy (`internal/dummytui`)

### A1 (HIGH, internal review HI-01) — identity-manager modal screens clipped on a real 80×24 terminal

**Finding:** `imOverlay` (surface_identitymanager.go) hardcoded a 100×30 compositing
canvas for its 5 self-composited modal screens (action-menu, clone-name-prompt,
delete-choice, confirm-destructive, backup-notice). On the real, documented 80×24
minimum terminal, centering against the wrong (100-wide) canvas shifted the modal 6
columns past the actual right edge — every line clipped, the border never closing.

**Fix:**
1. `model.go` gained a package-level `currentViewport` (defaults to
   `defaultWidth`/`defaultHeight`), updated by `Update()` on every
   `tea.WindowSizeMsg` — mirroring `m.width`/`m.height`.
2. `imOverlay` now centers/bounds against `currentViewport` instead of the fixed
   `defaultWidth`/`defaultHeight` constants.
3. `imOverlay` also applies `styleIMModal.Width(mw).Render(...)` (previously
   missing entirely) — mirroring the REAL product's own
   `StyleModal.Width(mw).Render(...)` convention (`tui/model.go`, `tui/confirm.go`,
   `tui/addrepo.go`). Without this second fix, several hand-authored content lines
   (e.g. confirm-destructive's "Default-focused..." sentence, 86 columns) were
   wider than the 72-column modal budget on ANY terminal size, so correcting the
   centering alone was insufficient — lipgloss was silently auto-sizing the border
   box to whichever content line happened to be longest instead of wrapping it.
4. Static capture callers (`RenderScreen`, `screenshot-tui-mockups`,
   `design_capture_test.go`, `manifest_test.go`) never send a `WindowSizeMsg`, so
   `currentViewport` stays at its 100×30 default there — the deterministic capture
   geometry (D-04) is unchanged for the static path.
5. Added `TestIdentityManager_ModalScreensFitReal80x24Viewport` — drives the exact
   live-navigation path (`Model.Update(tea.WindowSizeMsg{80,24})`, not a direct
   `RenderScreen` call) and asserts, for all 5 modal screens, that every line
   carrying a box-border glyph has that glyph's ANSI-aware visible column position
   (`lipgloss.Width`, not raw byte length) at or before column 80, and that the
   border corner glyphs are actually present (closes).

**Proof:**
```
$ go test ./internal/dummytui/... -run TestIdentityManager_ModalScreensFitReal80x24Viewport -v
=== RUN   TestIdentityManager_ModalScreensFitReal80x24Viewport
--- PASS: TestIdentityManager_ModalScreensFitReal80x24Viewport (0.00s)
    --- PASS: .../action-menu (0.00s)
    --- PASS: .../clone-name-prompt (0.00s)
    --- PASS: .../delete-choice (0.00s)
    --- PASS: .../confirm-destructive (0.00s)
    --- PASS: .../backup-notice (0.00s)
PASS
```
(Verified the test actually catches the regression: reverting fix step 2 alone —
keeping only the `.Width(mw)` fix — still failed with `endCol` values of 92-95 vs.
the required ≤80, proving BOTH fixes were needed, not just one.)

### A2 (MED-HIGH, UX systemic) — missing global-health context chip

**Finding:** 02-UX-DIRECTION.md §2 requires the header/context bar to show "app
name, the current view name, and a global context chip (e.g. identity count,
global health ✓/!/✗)". The HTML mockup's `Header.tsx` always rendered this chip;
`shell.go`'s `renderShellHeader` rendered only the app name + breadcrumb.

**Fix:** `renderShellHeader` now appends a static fixture chip —
`"8 identities · ! needs action"` — matching the semantic content the HTML mockup's
own HOME screen (`identity-manager/list-populated`/`detail-ssh-first`, which pass
`headerContext={{identityCount: identityManagerRows.length /* 8 */, health:
'warning'}}`) shows; other HTML routes' smaller ad hoc `{1, 'healthy'}`/`{0,
'healthy'}` props are screen-local demo simplifications for those individual states,
not the shell's global rollup — the home screen's value is the correct semantic
target for a single persistent, package-global TUI chip shown on every surface.
80-column width discipline preserved (longest breadcrumb + chip stays under 80
columns).

**Proof:** re-captured all 50 TUI PNGs; chip visible in every header row (see
`internal/dummytui/tui/*.png` — e.g. `identity-manager/tui/confirm-destructive.png`
header reads `gitid  identity-manager/confirm-destructive  8 identities · ! needs
action`).

### A3 (HIGH, UX) — ecdsa-p256 NIST-provenance caveat dropped

**Finding:** `cfAlgorithmCatalog`'s ecdsa-p256 entry read "Compact NIST P-256
curve; smaller than RSA." — the HTML mockup's verbatim text is "Compact NIST P-256
curve; smaller than RSA, though some users distrust NIST curve provenance versus
ed25519."

**Fix:** Restored the full clause verbatim in `surface_createflow.go`.

**Proof:**
```
$ grep -n "distrust NIST" internal/dummytui/surface_createflow.go \
    .planning/design/mockup-src/src/data/recipeFixtures.ts
internal/dummytui/surface_createflow.go:138: security: "Compact NIST P-256 curve; smaller than RSA, though some users distrust NIST curve provenance versus ed25519.",
.planning/design/mockup-src/src/data/recipeFixtures.ts:261:  'Compact NIST P-256 curve; smaller than RSA, though some users distrust NIST curve provenance versus ed25519.',
```
Re-captured `create-flow/tui/algo-catalog.png` — caveat visible.

### A4 (MED, UX) — signingkey safety clause dropped on git-screen

**Finding:** git-form-empty/git-form-filled's `user.signingkey` HTML helper text is
"A PATH to the public key — never the key material itself."; the TUI dropped "never
the key material itself" entirely (empty state) or showed no helper at all (filled
state).

**Fix:** `renderGitFormEmpty`/`renderGitFormFilled` (`surface_gitscreen.go`) now
carry the safety clause verbatim on both screens.

**Proof:** re-captured `git-screen/tui/git-form-empty.png` and
`git-form-filled.png` — both show "never the key material itself".

### A5 (MED, UX) — identity-manager safety clauses dropped/reworded

**Finding:** clone-name-prompt's HTML explainer says "...the key material itself is
not copied; a new key is generated for the clone." — TUI dropped that clause
entirely. confirm-destructive's HTML opens "This action is irreversible." — TUI
said "This cannot be undone." (same meaning, different words — not permitted by
§3's verbatim-copy requirement).

**Fix:** `renderIMCloneNamePrompt` now carries "the key material itself is not
copied" verbatim; `renderIMConfirmDestructive` now opens with "This action is
irreversible." verbatim (`surface_identitymanager.go`).

**Proof:** re-captured `identity-manager/tui/clone-name-prompt.png` and
`confirm-destructive.png` — both match the HTML wording word-for-word.

### A6 (LOW, UX) — confirm-write keybar mislabels the confirm key

**Finding:** create-flow's `confirm-write` TUI keybar showed "y backup-notice" (the
raw target SCREEN NAME) where the HTML `Keybar` shows "y Yes, write" (a confirm
ACTION).

**Fix:** Added an additive, backward-compatible `ScreenDef.KeyLabels
map[string]string` override to the registry (`registry.go`) — falls back to the
raw target screen ID (today's behavior) when a key has no override.
`renderShellKeybar` (`shell.go`) consults it. `confirm-write`'s `ScreenDef` now sets
`KeyLabels: map[string]string{"y": "Yes, write"}` (`surface_createflow.go`).

**Proof:** re-captured `create-flow/tui/confirm-write.png` — keybar reads
`y Yes, write   Esc back   q quit   ? help`.

### A7 (LOW, UX) — wrong glyph on an informational note

**Finding:** identity-manager's detail-ssh-first SSH-only note ("No Git identity
configured for this alias — SSH-only.") used the yellow "!" WARNING glyph; the HTML
treats it as `severity="info"`, and the surface's own locked severity-glyph
contract (established by `surface_health.go`/`surface_fixer.go`: warning=`!`
yellow, error/critical=`✗` red, info=`~` cyan) reserves `!` for warning-tier items.

**Fix:** Added `styleIMInfo` (cyan, `Color("6")`) to `surface_identitymanager.go`;
`renderIMDetailSSHFirst` now uses `~` instead of `!` for this note.

**Proof:** re-captured `identity-manager/tui/detail-ssh-first.png` — note now reads
`~ No Git identity configured for this alias — SSH-only.` in cyan.

### A8 (MED, Codex — PROVEN failing) — registry tests leak global state across `-shuffle`/`-count` runs

**Finding:** `internal/dummytui`'s package-level `registry` map is mutated by
`Register`/`RegisterOrReplace` calls in `registry_test.go`/`model_test.go` with no
cleanup, so re-running the SAME test under `-count=N` (or in a different order under
`-shuffle=on`) collides with state left behind by a prior iteration.

**Proof this was real (BEFORE the fix):**
```
$ go test -race -shuffle=on -count=10 ./internal/dummytui/...
--- FAIL: TestKeyOwners_FinalFiveOwnNumberKeys (0.00s)
    keyowners_test.go:41: ActivationKey owners: 7 distinct number keys claimed, want exactly 5
--- FAIL: TestRegisterOrReplace_SingleOwner (0.00s)
panic: dummytui: Register("test-replace-placeholder"): activation key "test-replace-key"
  already claimed by surface "test-replace-real" [recovered, repanicked]
FAIL
```

**Fix:** Added a `snapshotRegistry(t *testing.T)` helper (`registry_test.go`) that
snapshots the registry and restores it via `t.Cleanup`. Called at the top of every
test in `registry_test.go`/`model_test.go` that registers a test-scoped surface (14
call sites across 10 top-level tests + the 5 `TestLaunchKeyCollisionGuard`
subtests).

**Proof (AFTER the fix):**
```
$ go test -race -shuffle=on -count=10 ./internal/dummytui/...
ok  	github.com/castocolina/gitid/internal/dummytui	6.602s
```

---

## B. Capture harness (`internal/screenshot`) + e2e + Makefile

### B1 (HIGH, Codex + internal MED-01) — captures gated on breadcrumb alone, never the manifest signature

**Finding:** `design_capture_test.go` gated HTML captures on only `ScreenID(e)` and
TUI captures on only the breadcrumb, never the manifest's own per-screen
`Signature` — a wrong body under the right breadcrumb could be saved as a valid
reference PNG. Every `surface_*.go` file's own doc comment claimed this offline
suite DID check the signature; only the e2e PTY walker actually did (MED-01).

**Fix:**
1. `HTMLOptions` gained a `RequiredTexts []string` field (additive to the existing
   `RequiredText`); `CaptureHTML` now requires ALL of them present before writing a
   PNG.
2. `CaptureHTMLScreen` (`design_adapter.go`) now takes `requiredTexts ...string`.
3. `design_capture_test.go`'s HTML subtest now passes BOTH `ScreenID(e)` and
   `e.Signature`.
4. `design_capture_test.go`'s TUI subtest now ALSO asserts `e.Signature` is present
   in `dummytui.RenderScreen`'s output (previously breadcrumb-only).
5. `manifest_test.go`'s `TestManifestCrossValidation` now ALSO asserts the
   signature (previously breadcrumb-only) — closing MED-01: the doc comments are
   now actually true.
6. The HTML mockup itself had NO signature marker anywhere in its DOM (signatures
   were a TUI-only concept) — added `src/data/screenSignatures.ts` (a byte-identical
   mirror of every `manifest.json`'s `signature` field, keyed by ScreenID, the SAME
   "static, diff-able contract, not derived" precedent the Go dummy's own `sig*`
   constants already established) and wired it into `Shell.tsx`, which renders a
   `[SIG-...]` marker — mirroring every TUI screen's own trailing `[SIG-...]`
   bracket — looked up by the `title` prop every route already passes. Zero
   per-route file edits needed.

**Proof:**
```
$ go test -tags screenshot -run 'TestCaptureAllMockupScreens/.*/html' ./internal/screenshot/... -v
--- PASS: TestCaptureAllMockupScreens (147.25s)   [all 50 html subtests PASS]
$ go test -tags screenshot -run 'TestCaptureAllMockupScreens/.*/tui' ./internal/screenshot/... -v
--- PASS: TestCaptureAllMockupScreens (96.02s)    [all 50 tui subtests PASS]
```
Verified live during development: before `screenSignatures.ts` existed, the SAME
HTML capture failed exactly as designed — `context deadline exceeded` locating the
missing signature text — proving the check has real teeth, not a silent no-op.

### B2 (HIGH, Codex) — dummy-nav-e2e timeout budget too tight

**Finding:** `e2e/dummy_nav_e2e_test.go`'s internal `context.WithTimeout` and the
Makefile's `dummy-nav-e2e` target both used 60s, while the full 50-screen walk
already measured ~47s+ with no CI-variance headroom — `test-e2e` (the sibling
target driving the SAME walk alongside other suites) already uses 180s.

**Fix:** Both raised to 180s (`e2e/dummy_nav_e2e_test.go`, `Makefile`).

**Proof:**
```
$ make dummy-nav-e2e
gate-no-backend-files: OK ...
go build -o bin/gitid-dummy ./cmd/gitid-dummy
go test -tags e2e -race -timeout 180s -run TestDummyNav ./e2e/...
ok  	github.com/castocolina/gitid/e2e	46.688s
```

### B3 (MED, Codex) — HTML required-text check was a single point-in-time read

**Finding:** `CaptureHTML` waited for `load` then checked body text exactly once —
a React SPA can commit its route body milliseconds after `load` fires, risking a
flaky false pass/fail.

**Fix:** Folded into the same B1 change — `CaptureHTML` now polls (25ms interval)
for ALL required text markers until present or the capture's own `Timeout`
expires, rather than checking once immediately after `WaitLoad`.

**Proof:** all 50 HTML captures above passed on the first run with the poll loop in
place; no flakes observed across 2 full runs during this fix pass.

### B4 (WARN, security, SECURITY.md Finding 1 / T-02-BEGATE) — no-backend-files gate not automated

**Finding:** T-02-BEGATE's gate existed only as a one-off shell line in
`02-11-PLAN.md`'s `<verify>` block — not a Makefile target, not a CI step, not a
committed script. Any commit added to the branch before the 02-12 approval would
only be caught by a human manually re-running that exact command.

**Fix:** Added a `gate-no-backend-files` Makefile target (computes
`BASE=$(git merge-base main HEAD)`, fails if the diff touches anything outside
`{.planning/, internal/dummytui/, cmd/gitid-dummy/, internal/screenshot/, e2e/,
Makefile}`), wired as a prerequisite of `dummy-nav-e2e` so it runs automatically on
every invocation.

**Proof:**
```
$ make gate-no-backend-files
gate-no-backend-files: OK -- no files outside {.planning/, internal/dummytui/,
cmd/gitid-dummy/, internal/screenshot/, e2e/, Makefile} changed since main
(321884cf71b381512c74ad1a2ae40e5fc2e24ba8)
```

---

## C. MUI mockup copy + fold-clipped captures

### C1 (LOW, UX) — three HTML reference PNGs fold-clipped

**Finding:** global-ssh `options-list` (row 6, UseKeychain), global-git
`options-list` (rows 5-11), and identity-manager `list-populated` (row 8) were cut
off at the capture fold — content existed in the DOM but was invisible in the
captured PNG. Also requested: add the "(macOS only)" UseKeychain qualifier to the
HTML options-list row.

**Root cause:** `Shell.tsx`'s fixed `height: '100vh'` + `main`'s
`overflow: 'auto'` clipped any body taller than the 800px viewport INSIDE its own
scroll container — a full-page screenshot (`page.Screenshot(true, ...)`) only
captures the OUTER document's scroll height, which a fixed-height shell with its
own internal scroll never exceeds.

**Fix:** `Shell.tsx` changed to `minHeight: '100vh'` (not fixed) + removed `main`'s
`overflow: 'auto'` — the shell now grows to its natural content height; the fixed
capture VIEWPORT (`mockupViewportWidth`/`mockupViewportHeight`,
`design_adapter.go`) is unchanged, but a full-page capture of a taller screen now
captures the real height instead of clipping it.

**"(macOS only)" qualifier:** investigated and found it was ALREADY present in
BOTH media the whole time — `recipeFixtures.ts`'s `currentValue: 'yes (macOS
only)'` and `surface_globalssh.go`'s `current: "yes (macOS only)"` — the text was
simply invisible in the pre-fix clipped capture. No code change needed for this
part; corrected the CRITIQUE.md claim that the qualifier was TUI-only (see D1).

**Proof:**
```
$ python3 -c "import struct; d=open('.planning/design/global-ssh/html/options-list.png','rb').read(24); print(struct.unpack('>II', d[16:24]))"
(1280, 992)   # was clipped to a fixed 1280x800 before the fix
```
Visually confirmed via the re-captured PNGs: `global-ssh/html/options-list.png`
(all 6 rows + UseKeychain "(macOS only)" visible), `global-git/html/options-list.png`
(all 11 rows visible), `identity-manager/html/list-populated.png` (all 8 rows
visible).

### C2 — HTML/TUI header chip parity

**Confirmed:** after A2 landed, both media's shell header show the SAME semantic
content — `8 identities · ! needs action` (TUI, static fixture) vs. HTML's
per-route `headerContext` (the HOME screen route already used `{identityCount: 8,
health: 'warning'}}`, which is what A2's TUI fixture was deliberately chosen to
match). Verified via side-by-side review of `identity-manager/html/list-populated.png`
and `identity-manager/tui/list-populated.png`.

---

## D. Parity records + re-capture + gates

### D1 — parity.json re-resolution

- **REOPENED and re-resolved** (findings A3/A4/A5/A7 landed on these surfaces):
  - `create-flow/parity.json`: `labels-and-helper-copy-verbatim` (A3 — ecdsa-p256
    caveat).
  - `git-screen/parity.json`: `labels-and-helper-copy-verbatim` (A4 — signingkey
    safety clause).
  - `identity-manager/parity.json`: `field-set-and-order` (re-audited, no
    divergence found/introduced) + `labels-and-helper-copy-verbatim` (A5 —
    clone-name-prompt/confirm-destructive copy).
- **REWORDED (no reopen)** — overclaiming "byte-identical" corrected to
  "semantically equivalent, TUI-compacted for 80×24, load-bearing content intact":
  `global-ssh/parity.json` row 2 (`labels-and-helper-copy-verbatim`);
  `fixer/parity.json`'s labels row (`labels-and-helper-copy-verbatim`).
- Also corrected two now-stale literal-text quotes in
  `identity-manager/parity.json` (`safety-affordances-presence`,
  `delete-choice-safe-default` rows quoted the pre-fix "This cannot be undone"
  wording — updated to "This action is irreversible.").
- Updated all 7 `CRITIQUE.md` files' finding #1 to log the A2 shell-chip fix.
  Corrected `global-ssh/CRITIQUE.md`'s wrong "no clipping" claim (C1) and
  `identity-manager/CRITIQUE.md`'s / `global-git/CRITIQUE.md`'s "minor
  observation, no fix needed" framing of the SAME fold-clipping (now actually
  fixed, not just tolerated).

**Proof:**
```
$ python3 -c "
import json, glob
total = 0; unresolved = []
for f in sorted(glob.glob('.planning/design/*/parity.json')):
    data = json.load(open(f)); total += len(data)
    unresolved += [(f, r['dimension']) for r in data if r['status'] != 'resolved']
print('total rows:', total); print('unresolved:', unresolved)
"
total rows: 63
unresolved: []
```

### D2 — re-capture

All 50 TUI PNGs re-captured (`make screenshot-tui-mockups` equivalent — the shell
chip changes every TUI frame). All 50 HTML PNGs re-captured (full
`screenshot-html-mockups` run — the Shell.tsx fold fix and signature marker affect
every HTML frame too).

**Count invariant (per-surface manifest count == html PNG count == tui PNG
count):**

| Surface | manifest | html | tui |
|---|---|---|---|
| create-flow | 12 | 12 | 12 |
| fixer | 6 | 6 | 6 |
| git-screen | 7 | 7 | 7 |
| global-git | 6 | 6 | 6 |
| global-ssh | 6 | 6 | 6 |
| health | 5 | 5 | 5 |
| identity-manager | 8 | 8 | 8 |
| **Total** | **50** | **50** | **50** |

### D3 — final gates

```
$ make test
go test -race -coverprofile=coverage.out ./...
ok  	... (19 packages, all ok)

$ make lint
/Users/ramon/go/bin/golangci-lint run ./...
0 issues.

$ go test -race -shuffle=on -count=10 ./internal/dummytui/...
ok  	github.com/castocolina/gitid/internal/dummytui	6.602s

$ make dummy-nav-e2e
gate-no-backend-files: OK -- no files outside the Phase 2 allowlist changed since main
go build -o bin/gitid-dummy ./cmd/gitid-dummy
go test -tags e2e -race -timeout 180s -run TestDummyNav ./e2e/...
ok  	github.com/castocolina/gitid/e2e	46.688s

$ go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/... | grep github.com/castocolina/gitid
github.com/castocolina/gitid/internal/dummytui
github.com/castocolina/gitid/cmd/gitid-dummy
```
DLV-05 allowlist clean — exactly the two allowed first-party packages, nothing
else.

---

## Not fixed (explicitly out of this pass's scope, still tracked)

- **SECURITY.md Finding 2** (informational): `make lint` still has no
  `--build-tags` for `screenshot`/`e2e`, so gosec/staticcheck never run over
  `html.go`, `design_capture_test.go`, or any `e2e/*.go` file. Pre-existing
  (documented in `deferred-items.md` since 02-03), not part of the 4 reviews'
  fix-list items A-D above, and not re-flagged as a live finding by this pass
  (manual inspection of every new `exec.Command` call site in the touched files
  found arg-slice form with inline `#nosec`/`nolint` justification, consistent
  with the existing pattern). Recommended for a future phase's CI hardening, as
  `SECURITY.md` itself already recommends.
- **identity-manager's own keybar label-wording** (the "y backup-notice" vs. a
  semantic action phrase class A6 fixed for create-flow's `confirm-write` only) —
  `shell.go` gained the `KeyLabels` override mechanism this pass; applying it to
  identity-manager's own screens was left for a future pass since it was not
  explicitly requested in the fix list (only create-flow's confirm-write was
  named, A6).
