---
phase: 04-git-configuration-screen
reviewed: 2026-08-25T07:10:00Z
depth: standard
iteration: 4
files_reviewed: 32
files_reviewed_list:
  - .gitignore
  - Makefile
  - cmd/gitid/gate_visual_regression_test.go
  - cmd/gitid/smoke_network_test.go
  - cmd/gitid/wiring.go
  - cmd/gitid/wiring_test.go
  - e2e/create_flow_pty_e2e_test.go
  - e2e/git_configuration_pty_e2e_test.go
  - e2e/ui_pty_e2e_test.go
  - internal/doctor/checks/reserved_test.go
  - internal/dummytui/fixturebackend.go
  - internal/gitconfig/fragment.go
  - internal/gitconfig/fragment_test.go
  - internal/gitconfig/reader.go
  - internal/gitconfig/reader_test.go
  - internal/gitconfig/renderer.go
  - internal/gitconfig/renderer_test.go
  - internal/identity/identity.go
  - internal/identity/loader.go
  - internal/identity/loader_test.go
  - internal/keygen/signers.go
  - internal/screenshot/createflow.go
  - internal/screenshot/createflow_packet.go
  - internal/screenshot/createflow_packet_test.go
  - internal/screenshot/createflow_regions.go
  - internal/screenshot/createflow_test.go
  - internal/screenshot/normalize_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/identities.go
  - internal/tuikit/identities_test.go
  - internal/tuikit/store.go
  - internal/tuikit/views.go
findings:
  critical: 5
  warning: 10
  info: 0
  total: 15
status: issues_found
---

# Phase 4: Code Review Report (iteration 4)

**Reviewed:** 2026-08-25T07:10:00Z
**Depth:** standard
**Files Reviewed:** 32
**Status:** issues_found

## Summary

Adversarial re-review of the 7 fixes claimed in `04-REVIEW-FIX.md` (iteration 3,
commits `4bf7dc7` … `00dde23`), plus a fresh sweep of the full current state of the
changed files. Every claim was re-verified with an executable probe against the real
backend, not by reading the diff. Probe files were created, run, and deleted;
`git status` outside `.planning/` is clean.

**Baseline gate status on the current tree (measured this session, not quoted):**

| Gate | Result |
|---|---|
| `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` | PASS (18 packages) |
| `make lint` | PASS, 0 issues (~5 s) |
| `make test` | PASS |
| `make gate-visual-regression` | PASS (8 tests, no SKIP) |

### What genuinely landed (independently verified by probe)

- **WR-28 (screenshot half) — fixed and proven.** I injected a type error into
  `internal/screenshot/createflow_regions.go` → `make lint` fails (`Error 1` at
  `lint-screenshot`). I then reintroduced the exact WR-26 off-by-one → `make test`
  fails with `--- FAIL: TestExtractRegion_GitCeremonyStartsExactlyOnTheHeadingRowNotOneEarly`.
  Both reverted. The `screenshot` tag is now genuinely gated. **But see CR-13: the
  same blindspot is still wide open for the `smoke` tag, which is where the rot the
  fixer had to repair actually lived.**
- **CR-08 — fixed and proven, in both surfaces.** Probe against the *real* backend
  (`newBackendForHome` + seeded HOME, 100×30): `PROBE CR-08 TAB OK: flipped at tab
  stop 3 (☐ -> ☑)` and `PROBE CR-08 CLICK OK: click at (49,5) + space toggled ☐ -> ☑`.
  Wizard probe: `PROBE WIZARD CLICK OK: true -> false`. `paneGitFocusOrder` is real,
  the `% gitPaneFocusRing` modulo is gone from every call site, and the reordered row
  anchors under `anchoredLabelMatch`.
- **CR-09 — fixed and proven end to end.** Seeded two hermetic HOMEs differing only
  by the `# BEGIN gitid managed: provider-rewrite:github.com` block:
  `ForceSSH=true → "☑ Force SSH"`, `ForceSSH=false → "☐ Force SSH"`. The short-form
  provider path (`acct.Provider == "github"`, no marker) resolves correctly via
  `rewriteLookupProvider` — the case the fixer flagged as trickiest genuinely works.
- **WR-24 — applied as described.** `commitGitArtifacts` now calls
  `journal.ensureManagedDir(b.sshDir, sshDirMode)` behind a new `"ssh-dir"` injection
  boundary, immediately before `WriteAllowedSignersReplacing`, and `~/.ssh` is out of
  the watch-only loop. The rollback matrix carries the new boundary.
- **WR-26 — fixed and proven, including the second-order case.** I hand-traced the
  residual shape the fixer did *not* test (unrelated row directly above a *wrapped*
  heading): at `i` the window is `rp[i]+" "+rp[i+1]` = `"UNRELATED … configured —"`,
  which does not contain the marker, so `start` correctly resolves at the wrapped
  row. No off-by-one remains.
- **WR-29 — fixed.** The e2e assertion is now a real `☐ → ☑ → ☐` round trip.

### What did not land, or is newly broken

1. **WR-27 is cosmetically fixed and functionally vacuous** (CR-10). Six production
   dispositions now carry predicates, but I proved by probe that the shipped
   predicate `absent:"gitdir:~/git/"` **accepts an arbitrary live-side regression**.
   All six predicates are satisfied forever by the frozen dummy side.
2. **`TestNegativeControls_AllProtectedRegionsDetectMutation` does not do what its
   name and comment say** (CR-11) — it mutates exactly one region's metadata field
   and `break`s out of both loops. It is the single load-bearing "the gate really
   catches drift" control, and it is a lie.
3. **The Configure-Git confirmation ceremony's pre-write disclosure is materially
   false** (CR-12) — it names backup files that will never exist, understates the
   backup count 2-vs-4, and omits a directory the transaction creates.
4. **WR-28's root cause survives for the `smoke` build tag** (CR-13) — I injected a
   compile break into `cmd/gitid/smoke_network_test.go`; `make lint` and `make test`
   both stayed green.
5. **The match-strategy copy contradicts two other widgets on the same rendered
   frame** (CR-14, was WR-31, skipped) — `● gitdir (default) — applies inside
   ~/work/` sits three rows above `[includeIf "gitdir:~/git/work/"]`.
6. **WR-25 (skipped) is a real security defect in the rollback path**, not a
   deferrable nicety — see BL-15.
7. Latent/structural: a live HOME-leak in the "hermetic" backend seam (WR-35),
   `RegionContinueDisabledReason` is still unreachable dead code despite a comment
   claiming it was re-wired (WR-36), and the constant block that has produced three
   consecutive regressions now carries comments that state the opposite of the code
   (WR-37).

---

## Critical Issues

### CR-10: WR-27's six new predicates cannot reject any real-binary regression — the `absent:`/`contains:` grammar is satisfied by whichever side structurally never matches

**File:** `internal/screenshot/createflow.go:222-234` (`regionPredicateSatisfied`),
`:1272-1283`, `:1336-1338`, `:1368-1370` (the six shipped dispositions),
`e2e/git_configuration_pty_e2e_test.go:862-872` (`gitScreenPredicateSatisfied`, same bug)
**Severity:** BLOCKER

**Issue:** The predicate holds when the needle condition is true on **either** side:

```go
case strings.HasPrefix(predicate, "absent:"):
    needle := ...
    return !strings.Contains(live, needle) || !strings.Contains(approved, needle)
```

Every one of the six shipped predicates is anchored on text that the **frozen dummy
fixture structurally never contains** (or always contains), so the predicate is
permanently satisfied regardless of what the live binary renders:

| Disposition | Predicate | Always-satisfying side |
|---|---|---|
| `fixtureSidebarDisposition` | `absent:"clientB"` | real sidebar never has `clientB` |
| `fixtureHeaderStatusDisposition` | `contains:"ids"` | both sides always have `ids` |
| `gitPreviewDisposition` | `absent:"gitdir:~/git/"` | dummy fixture predates `~/git/` |
| `formFieldsDisposition` | `absent:"User"` | dummy seeds `"<id> identity"` |
| `git-ceremony` sentinel | `absent:"BEGIN gitid managed"` | dummy sample has no sentinels |
| `git-ceremony` backups | `absent:".bak."` | dummy uses `.backup.` naming |

Reproduced with the **shipped** predicate string, driving the real `BuildRegionDiffs`:

```
live     = "shared header\n│ includeIf block\n│ TOTALLY BROKEN GARBAGE OUTPUT\n│ Write it\n…"
approved = "shared header\n│ includeIf block\n│ [includeIf \"gitdir:~/acme/\"]\n│ Write it\n…"
Predicate: `absent:"gitdir:~/git/"`

PROBE RESULT: predicate ACCEPTED an unrelated live-side regression (vacuous gate)
```

`TestBuildRegionDiffsRejectsDivergenceViolatingScopedPredicate` passes only because it
puts the needle on **both** sides — a state the real dummy fixture can never reach. So
the one test that "proves" the mechanism tests a case that cannot occur in production.
The identical `||` grammar is duplicated in the e2e gate, so **both** gates share the
hole. Net effect: WR-19 → WR-27 has now consumed two fix iterations and the set of
divergences accepted inside an allowlisted region is still exactly "anything".

**Fix:** the predicate must express the *shape* of the authorized divergence, not "one
side happens to lack the string".

```go
func regionPredicateSatisfied(predicate, live, approved string) bool {
    switch {
    case predicate == "":
        return true
    case strings.HasPrefix(predicate, "contains:"):
        needle := strings.Trim(strings.TrimPrefix(predicate, "contains:"), `"`)
        // BOTH sides must still carry the marker — the difference is elsewhere.
        return strings.Contains(live, needle) && strings.Contains(approved, needle)
    case strings.HasPrefix(predicate, "absent:"):
        needle := strings.Trim(strings.TrimPrefix(predicate, "absent:"), `"`)
        // The authorized divergence IS the presence/absence asymmetry: exactly
        // one side carries it. Both-present or both-absent is an unreviewed change.
        return strings.Contains(live, needle) != strings.Contains(approved, needle)
    }
    return false
}
```

Apply the same change to `gitScreenPredicateSatisfied`. Then add a test that mutates
the **live** side of a production disposition (not a synthetic one) and asserts
`BuildRegionDiffs` errors — the probe above is the exact fixture to reuse. Until such a
test exists and is red before the fix, WR-27 must not be recorded as closed.

---

### CR-11: `TestNegativeControls_AllProtectedRegionsDetectMutation` mutates one region and stops — the name, the doc comment, and the sibling git-screen control all overstate what is gated

**File:** `cmd/gitid/gate_visual_regression_test.go:456-507`, and the same shape at
`:561-612` (`TestNegativeControl_StaleGitScreenClassification`)
**Severity:** BLOCKER

**Issue:** The doc comment reads *"proves that **every** protected (non-allowlisted)
region on **every** screen is sensitive to mutations — the gate can catch any
meaningful drift (CR-04 exhaustive negative controls)"*. The body does none of that:

```go
for i := range records {
    for j := range records[i].Regions {
        region := &records[i].Regions[j]
        if !region.Comparable || !region.Equal {
            region.Classification = ""   // clear ONE metadata field
            mutated = true
            break                        // first hit only
        }
    }
    if mutated { break }                 // first record only
}
```

Three separate overclaims:

1. **One region, not every region.** Both `break`s fire on the first match, so exactly
   one `NamedRegionDiff` on one screen is ever exercised. Which one depends on
   `RequiredScreenSpecs()`/`AllRegionNames()` iteration order — a registry reorder
   silently changes what is tested.
2. **It targets `!Equal` regions — i.e. the *allowlisted* ones — not "protected
   (non-allowlisted)" regions.** The regions the comment claims to protect are the
   ones the loop skips.
3. **No region *text* is ever mutated.** Clearing `Classification` proves
   `ValidateRegionDiffs` requires a classification field. It proves nothing about
   whether a changed *rendering* is detected. This is the single control the phase
   leans on for "the gate is not vacuously permissive", and combined with CR-10 the
   real answer is that an allowlisted region's text can change arbitrarily and both
   gates stay green.

`TestNegativeControl_StaleGitScreenClassification` is a copy-paste of the same body
scoped to git-screen IDs, so it inherits all three defects.

**Fix:** make the control exhaustive over region *content*, and separate the two
properties:

```go
// (a) classification requirement — keep the existing single-mutation check but
//     rename it to what it is: TestNegativeControl_UnclassifiedDifferenceRejected.

// (b) NEW: every comparable, currently-Equal region must be mutation-sensitive.
for i := range records {
    for j := range records[i].Regions {
        r := records[i].Regions[j]
        if !r.Comparable || !r.Equal {
            continue
        }
        mutatedRecords := deepCopy(records)
        m := &mutatedRecords[i].Regions[j]
        m.LiveText += "\nGATE-CANARY"
        m.LiveHash = sha256Hex([]byte(m.LiveText))
        data, _ := json.Marshal(screenshot.RegionDiffs{ /* … */ Screens: mutatedRecords})
        if err := screenshot.ValidateRegionDiffs(data, "negative-control", specs); err == nil {
            t.Errorf("region %q on screen %q is NOT mutation-sensitive — the gate would miss a real regression here",
                r.Name, mutatedRecords[i].ScreenID)
        }
    }
}
```

---

### CR-12: the Configure-Git confirmation ceremony discloses backups that will never be created, undercounts them 2-vs-4, and omits a directory it creates

**File:** `internal/tuikit/identities.go:2086-2095` (`gitCeremonyFor`),
`internal/tuikit/store.go:448-451` (`NewBackupPath`),
`internal/filewriter/filewriter.go:138` (the real naming),
`cmd/gitid/wiring.go:1122` (undisclosed `ensureManagedDir`), `:1132` (undisclosed `ensureDir(gitDirPath)`)
**Severity:** BLOCKER

**Issue:** CLAUDE.md makes the confirm-write ceremony the auditable contract for a
mutation of the user's real files. It is currently wrong in three independent ways.
Rendered by the **real** backend against a seeded HOME (my probe, verbatim):

```
│ Write Git identity for "work"
│ Touches ~/.gitconfig.d/work · ~/.gitconfig · ~/.ssh/allowed_signers
│ Backup → ~/.gitconfig.backup.2026-08-25T05-43-08Z
│ Backup → ~/.ssh/allowed_signers.backup.2026-08-25T05-43-08Z
│   (written first — restore it to undo)
```

versus what the transaction actually writes (real receipt, e2e frame
`git-configuration-mouse-field-focus.txt`, and `filewriter.Write`):

```
Backed up → …/.gitconfig.d/acme.bak.1787620111536588000
Backed up → …/.gitconfig.bak.1787620111627806000
Backed up → …/.gitconfig.bak.1787620111671276000
Backed up → …/.ssh/allowed_signers.bak.1787620111726253000
```

1. **The declared filenames cannot exist.** `NewBackupPath` mints
   `<file>.backup.<ISO-8601>`; `filewriter.Write` mints
   `<file>.bak.<unix-nanoseconds>`. The ceremony's own instruction — *"(written first
   — restore it to undo)"* — points the user at a path that is never created. A user
   who trusts this line to recover cannot.
2. **The count is wrong.** Two backups are declared; four are taken on the
   edit-an-existing-identity path (the phase's headline flow). The `~/.gitconfig.d/<id>`
   fragment backup is never disclosed at all, even though `Touches` lists the file.
3. **Directory creation is still undisclosed** (this is WR-16, skipped again).
   `commitGitArtifacts` calls `journal.ensureManagedDir(b.fragmentDir, 0o700)` and
   `journal.ensureDir(gitDirPath, 0o700)`, creating `~/.gitconfig.d/` and
   `~/git/<identity>/` (and their parents) if absent — mutations the user confirmed
   nothing about. `grep -rn "create directory" internal/ cmd/` still returns nothing.

These are three symptoms of one root cause: `gitCeremonyFor` builds its disclosure from
hardcoded strings in the UI layer instead of asking the backend what the write will do.
`CreateWritePlan(spec, git)` (`wiring.go:678-710`) is the established seam for exactly
this and already computes targets/backups from the real filesystem.

**Fix:** add the `GitWritePlan` sibling and drive the ceremony from it.

```go
// internal/tuikit/backend.go
GitWritePlan(spec GitSpec) WritePlan   // Targets, Backups, CreatedDirs

// cmd/gitid/wiring.go — reuse the same fileExists probing CreateWritePlan uses,
// and the same ".bak." suffix filewriter actually produces.
func (b *realBackend) GitWritePlan(spec tuikit.GitSpec) tuikit.WritePlan { … }

// internal/tuikit/identities.go gitCeremonyFor
plan := m.backend.GitWritePlan(m.gitPaneForm.spec(sel.Name, sel.KeyPath))
return newCeremony(ceremonyConfig{
    Targets: plan.Targets,
    Backups: plan.Backups,           // real, existence-gated, correctly suffixed
    Creates: plan.CreatedDirs,       // NEW disclosure row
    …
})
```

At minimum, and independently of the seam work, `NewBackupPath`'s suffix must be
changed to `.bak.` so the declared and actual naming conventions cannot disagree, and
a test must pin `NewBackupPath`'s format against `filewriter`'s. Note the golden frames
`make gate-visual-regression` pins will change — that is the point.

---

### CR-13: WR-28 closed the `screenshot` blindspot and left the `smoke` blindspot open — the exact place the rot it repaired came from

**File:** `Makefile:lint-screenshot` (`go vet -tags screenshot ./...`), `Makefile:test`,
`cmd/gitid/smoke_network_test.go`
**Severity:** BLOCKER

**Issue:** WR-28's stated root cause was *"a `//go:build` tag no gate ever compiles"*.
The fix closes that for `screenshot` only. `smoke` is still invisible — which is
notable because the fixer's own report records that wiring the gate *"surfaced a real
pre-existing compile break (`cmd/gitid/smoke_network_test.go` called `tester.PreWrite`
with a stale 3-arg signature — never compiled by any gate)"*. That file was repaired,
and then left in exactly the ungated state that let it rot.

Reproduced — injected `func smokeProbeBreak() { var x int = "nope"; _ = x }` into
`cmd/gitid/smoke_network_test.go`:

```
LINT_EXIT=0          # make lint: green
TEST_EXIT=0          # go test -race ./cmd/gitid/...: green
SMOKE_TAG_EXIT=1     # go test -tags smoke ./cmd/gitid/...:
  cmd/gitid/smoke_network_test.go:75:38: cannot use "nope" (untyped string constant) as int value
```

`make smoke-network-test` is documented as *"NEVER a `make test`/`make test-e2e`/CI
prerequisite"* — correct for *running* a network test, but it also means nothing ever
**compiles** it. `go vet -tags smoke ./...` costs ~1 s and requires no network.

**Fix:**

```make
lint-screenshot:
	go vet -tags screenshot ./...
	go vet -tags smoke ./...       # WR-28 root cause: any build tag no gate compiles rots
	go vet -tags e2e ./...         # e2e is compiled by make test-e2e, but not vetted
	$(GOLANGCI_LINT) run --build-tags screenshot ./internal/screenshot/...
```

(Rename the target to `lint-tagged` — its scope is no longer "screenshot".) Add a
grep-based guard that fails when a new `//go:build <tag>` appears in the tree without a
matching `go vet -tags <tag>` line in the Makefile; otherwise this recurs on the next
tag introduced.

---

### CR-14: the match-strategy option copy contradicts two other widgets on the same rendered frame (carried from WR-31, skipped)

**File:** `internal/tuikit/identities.go:652-661` (`strategyCopy`), `:735-740` (`gitDirFor`),
`internal/dummytui/fixturebackend.go:164-166`
**Severity:** BLOCKER

**Issue:** Not a style nit — three widgets on **one** frame state three different
things about the same value. Real backend, seeded HOME, verbatim from my probe:

```
│      ● gitdir (default) — applies inside ~/work/          ← strategyCopy
│ ╭╌ ~/.gitconfig (includeIf block — preview) ╌╌╌╌╌╌╌╌╌╮
│ ┊ [includeIf "gitdir:~/git/work/"]                   ┊   ← what is written
│    gitdir path     [~/git/work/]                          ← the editable field
```

`strategyCopy` hardcodes `"~/" + name + "/"`; every write path uses
`spec.GitDir` = `gitDirFor(identity)` = `"~/git/" + identity + "/"`. On a
confirmation-gated flow whose whole purpose is showing the user what will change, the
*selected radio option's own label* is the one element that lies. It has now survived
four review iterations while CR-03/CR-07 were spent making the *rest* of this surface
truthful.

Second half: `FixtureBackend.IncludeIfPreview` ignores `spec.GitDir` entirely
(`strings.ReplaceAll(GitScreenMatchStrategyPreview[spec.Strategy], "personal", spec.Identity)`),
so the dummy and real previews can never agree about the gitdir — which is precisely
the divergence CR-10's now-vacuous `absent:"gitdir:~/git/"` predicate is supposed to be
narrowly authorizing.

**Fix:**

```go
func strategyCopy(strategy, name, gitDir string) string {
    switch strategy {
    case "gitdir":
        return "gitdir (default) — applies inside " + gitDir
    …
}
// call sites: strategyCopy(s, name, g.gitDirFor(name))
// hitStrategyRow must pass the same gitDir so the click target still matches the label.
```

and make `FixtureBackend.IncludeIfPreview` / `stubBackend.IncludeIfPreview` substitute
`spec.GitDir` rather than a frozen literal.

---

### BL-15: rollback re-loosens a hardened managed root even when a file under it failed to restore (carried from WR-25, skipped)

**File:** `cmd/gitid/wiring.go:964-1037` (`restore()`)
**Severity:** BLOCKER (security)

**Issue:** `restore()` runs the file loop first, collecting `failures`, then runs the
`chmodDirs` loop **unconditionally**:

```go
for i := len(j.files) - 1; i >= 0; i-- { … failures = append(failures, outcome) }

for i := len(j.chmodDirs) - 1; i >= 0; i-- {   // no check against `failures`
    s := j.chmodDirs[i]
    err = os.Chmod(s.path, s.mode)             // reverts ~/.ssh to its LOOSE prior mode
}
```

If a file under a managed root could not be removed or restored — e.g. the freshly
written private key at `~/.ssh/id_ed25519_<name>` — the transaction still reverts
`~/.ssh` from `0700` back to whatever loose mode it had (`0755`, `0777`), leaving a
**newly created private key inside a group/world-readable directory**, and reports the
rollback as partially successful. This is precisely the posture CR-05 was raised to
establish; WR-24 has now extended the same `ensureManagedDir` (and therefore the same
exposure) to the standalone Configure-Git path, widening the window rather than
narrowing it.

**Fix:**

```go
under := func(path string) bool {
    for _, f := range failures {
        if strings.HasPrefix(f, j.b.displayPath(path)+string(os.PathSeparator)) {
            return true
        }
    }
    return false
}
for i := len(j.chmodDirs) - 1; i >= 0; i-- {
    s := j.chmodDirs[i]
    if under(s.path) {
        outcomes = append(outcomes, j.b.displayPath(s.path)+
            ": mode kept at "+s.mode.String()+" — a file under it could not be restored")
        continue
    }
    …
}
```

Track the failure set by path rather than by formatted string if the prefix match is
too loose. Add a fault-injection case: fail `restore:<sshDir>/id_ed25519_x` and assert
`~/.ssh` is still `0700` afterwards.

---

## Warnings

### WR-35: `newBackendForHome` does not actually root the identity inventory at `home` — `accounts()` reads the developer's real `$HOME`

**File:** `cmd/gitid/wiring.go:142-157` (the doc comment), `:1440-1455` (`accounts()`),
`internal/identity/inventory.go:132-142`, `:153-157`
**Severity:** WARNING

**Issue:** `newBackendForHome`'s comment states it exists *"so tests can drive the whole
composition root over a hermetic fake home without ever touching the developer's real
~/.ssh"*. That is false for the primary read path:

```go
func (b *realBackend) accounts() []identity.Account {
    deps := identity.BuildInventoryDeps()   // ignores b.home / b.sshConfigPath entirely
    sshBytes, err := deps.ReadSSHConfig()   // readSSHConfigIncludeAware → os.UserHomeDir()
    …
}
```

I hit this live: my first CR-09 probe called `newBackendForHome(t.TempDir())` with a
seeded `work` identity and got back

```
PROBE identity "castocolina" ForceSSH=false Provider="github.com" SSHHost="github.com" GitFragmentPath=""
```

— the reviewer's **real** GitHub identity, read from the real `~/.ssh/config`. The
probe only became meaningful after adding `t.Setenv("HOME", home)`.

Every consumer today happens to also set `HOME`, so nothing is red — but the seam
offers a `home` parameter that silently does not govern the inventory, and there is no
guard. This is the same "injected-seam wiring blindspot" class that has already
recurred twice in this project (Phase 4 doctor, Phase 5 TUI). Concretely it means a
`newBackendForHome(tmp)` written without `t.Setenv` renders the developer's real
identity names/emails into gate captures and evidence packets.

**Fix:** thread the paths the backend already owns through the seam.

```go
func newBackendForHome(home string) *realBackend {
    b := &realBackend{ home: home, … }
    b.inventory = identity.InventoryDepsForHome(home)   // NEW: home-parameterised
    b.deps = buildIdentityDeps(b)
    return b
}
func (b *realBackend) accounts() []identity.Account { deps := b.inventory; … }
```

Then add a test that seeds a temp HOME **without** `t.Setenv` and asserts the seeded
identity — it must fail before the fix.

### WR-36: `RegionContinueDisabledReason` is still unreachable dead code, and a comment claims otherwise

**File:** `internal/screenshot/createflow_regions.go:124-131`, `:174-175`, `:666-686`
(`AllRegionNames`), `internal/screenshot/createflow.go:347-357`
**Severity:** WARNING

**Issue:** WR-08 added `RegionContinueDisabledReason` to `ExtractRegion`'s switch and
left a comment saying `extractContinueDisabledReason` was *"orphaned dead code — now
re-wired in createflow_regions.go"*. It is not reachable:

- `AllRegionNames()` (`:667-686`) **omits** `RegionContinueDisabledReason`.
- `BuildRegionDiffs` only calls `ExtractRegion` from two loops:
  `spec.RequiredRegions` (`createflow_packet.go:1280`) and `AllRegionNames()` (`:1290`).
- No spec lists it in `RequiredRegions` — `createflow.go:347-357` explains why and does
  not add it.
- `grep -rn "RegionContinueDisabledReason" internal/ cmd/` outside its own definition
  returns only that comment.

So the extractor has zero call sites, exactly the `min`/`currentGitCommit`/
`parseAllowlist` pattern WR-03/WR-13/WR-18 were each raised about — except this time
the dead code is documented as live, which is worse than silent dead code.

Structurally worse: because `BuildRegionDiffs` and `ValidateRegionDiffs` both iterate
`AllRegionNames()`, **any** `RegionName` constant that is not in that slice is silently
never compared. There is no compile-time or test-time link between the `const` block
and `AllRegionNames()`.

**Fix:** either delete `RegionContinueDisabledReason` + `extractContinueDisabledReason`
and correct the comment, or add the missing `git-form-invalid-email` spec that requires
it. Independently, add the exhaustiveness guard:

```go
func TestAllRegionNamesCoversEveryExtractableRegion(t *testing.T) {
    listed := map[screenshot.RegionName]bool{}
    for _, n := range screenshot.AllRegionNames() { listed[n] = true }
    for _, n := range screenshot.EveryRegionConstant() { // new, generated or hand-kept
        if !listed[n] {
            t.Errorf("region %q is extractable but absent from AllRegionNames() — never compared by any gate", n)
        }
    }
}
```

### WR-37: the constant block that has produced three consecutive regressions now carries comments that state the opposite of the code

**File:** `internal/tuikit/identities.go:504-535`
**Severity:** WARNING

**Issue:** CR-08 changed the pane's ring but not the doc comments that justify the
constants' values. All three statements below are now false:

```go
// gitPaneFocusRing is the pane's own Tab/Shift+Tab ring —
// deliberately the CONTIGUOUS range [0, gitPaneFocusButton], so the pane's
// raw `(m.gitFocus+1) % gitPaneFocusRing` arithmetic stays correct.        ← no such arithmetic exists
// ForceSSH and gitDir are reached in the pane via direct mouse click /
// ctrl+g only, never Tab-cycled, so they must stay OUTSIDE this range.     ← ForceSSH IS Tab-cycled now

// gitFieldForceSSH and gitFieldGitDir are reached via direct mouse click in
// the configure-Git pane (never Tab-cycled there — see gitPaneFocusRing above)  ← false
```

`gitPaneFocusRing` itself now has exactly one remaining use — as the arithmetic base of
`gitFieldForceSSH = gitPaneFocusRing + iota` — so a constant *named* "ring size" is no
longer any ring's size (the pane ring is `len(paneGitFocusOrder) == 5`). This is the
exact file and the exact constant block behind CR-04 → CR-06 → CR-08; leaving
contradictory guidance in it is how the next agent reintroduces the same bug.

**Fix:** rewrite the three comment blocks to describe `paneGitFocusOrder` /
`wizardGitFocusOrder` as the sole rings, and rename `gitPaneFocusRing` to something
that means what it now does (e.g. `gitFocusSlotBase`) — or drop it and write
`gitFieldForceSSH = gitPaneFocusButton + 1 + iota` directly.

### WR-38: unchecking Force SSH desynchronises the in-memory row from disk — the checkbox flips back on the next launch

**File:** `internal/tuikit/identities.go:1768-1772` (`ConfigureGit{… ForceSSH: spec.ForceSSH}`),
`internal/tuikit/store.go:244`, `cmd/gitid/wiring.go:1160-1169`
**Severity:** WARNING

**Issue:** `commitGitArtifacts` deliberately never *removes* the shared
`provider-rewrite:<host>` block when `spec.ForceSSH` is false (D-06: another identity
may depend on it). But the reducer unconditionally writes `row.ForceSSH = a.ForceSSH`,
so after an unchecked write the list says `ForceSSH: false` while the block is still on
disk. `identity.Reconstruct` reads the disk, so the very next `gitid` launch renders
`☑ Force SSH` again — the user's choice appears to have been silently reverted.

The ceremony's WR-05 note ("…is left in place because other identities may use it") now
correctly explains the *disk* state, which makes the *list* state the odd one out.

Related, and worth deciding explicitly: because the block is provider-scoped,
`Reconstruct` sets `ForceSSH: true` for **every** identity on that provider, including
one whose own write had it unchecked. That may be the intended semantics ("the rewrite
is in effect for this host"), but nothing states it and nothing tests it.

**Fix:** either have `Persist` re-read the real state after a Git commit (the
`AddIdentity` path already does this — `wiring.go:352-360`), or keep the optimistic
reducer but drop `ForceSSH` from the `ConfigureGit` payload so the row keeps the
disk-derived value. Add a test asserting the checkbox state survives a
write-with-Force-SSH-unchecked → reload cycle.

### WR-39: Enter on the Force-SSH checkbox opens the write ceremony instead of toggling — in the pane as well as the wizard (carried from WR-32)

**File:** `internal/tuikit/identities.go:2132-2139` (pane), `:2499-2518` (wizard),
`:824-827` (`handleEdit`'s unreachable `"enter"` clause)
**Severity:** WARNING

**Issue:** Confirmed still live and confirmed to affect **both** surfaces (the previous
review named only the wizard; the fixer noticed the pane and left it). In
`handleGitKey`:

```go
case "enter":
    if m.gitPaneForm.valid() {
        m.gitCeremony = m.gitCeremonyFor(sel)
        m.pane = paneGitCeremony
    }
```

fires regardless of whether `m.gitFocus == gitFieldForceSSH`. Meanwhile
`handleEdit`'s `case gitFieldForceSSH:` accepts `key == "enter"`, but both callers
intercept `enter` first, so that clause is unreachable dead code that reads as if the
behaviour were implemented. A checkbox that jumps to a write ceremony on Enter is a
hazardous default; it is mitigated only by the ceremony's own second confirmation.

**Fix:** add `case gitFieldForceSSH: m.gitPaneForm = m.gitPaneForm.handleEdit(msg, m.gitFocus)`
ahead of the primary-action fall-through in **both** `handleGitKey` and the wizard's
step-2 `enter` switch, then keep `handleEdit`'s `"enter"` clause (now genuinely
reachable) and test it.

### WR-40: `ConfigureGit.Name` and the receipt note still read `m.selected` live after WR-20 (carried from WR-30)

**File:** `internal/tuikit/identities.go:1768-1769`
**Severity:** WARNING

**Issue:** WR-20 moved five of six payload fields onto `m.gitCommitSpec`, but
`Name: m.selected` — the field that decides **which** row the reducer mutates — and the
note text still read the live model. `m.gitCommitSpec.Identity` is captured at
`:2114-2118` and is the correct source. The `gitCommitPending` early return
(`:2103-2106`) keeps this latent today, so it is not currently observable; it is
nonetheless the highest-blast-radius field on the exact code path WR-14 and WR-20 were
both raised to eliminate.

**Fix:** `Name: spec.Identity` and
`note: 'Git identity "' + spec.Identity + '" configured.'`.

### WR-41: `handleWizardClick` routes the algorithm-row click through a bare `5`

**File:** `internal/tuikit/identities.go:2940-2946`
**Severity:** WARNING

**Issue:**

```go
if idx, ok := hitAlgorithmRow(w.catalog(), body, x, y); ok {
    w.focus = 5
    w.algoIdx = idx
    w.form = w.form.setFocus(5)
```

`5` is `wizardFocusKeyBody` (`= wizardFocusKeySource + 1 = sshFieldPort + 2`). It is
correct **today**, and it is the literal shape of the bug that produced CR-04
(`gitFieldForceSSH == gitPaneFocusButton == 3`) and CR-06 (Tab modulo over
non-contiguous constants). Any change to `sshFieldPrefix…sshFieldPort` or to
`wizardFocusKeySource` silently repoints this click.

**Fix:** `w.focus = wizardFocusKeyBody` in both places, and add the same
membership-guard test `TestWizardClickTableEntriesAreAllWizardRingMembers` already
provides for `gitFormFieldSlots` — asserting every slot a click handler can assign is a
member of `wizardStep0FocusRing(w.keySource)`.

### WR-42: `displayMessage`'s substring replace is fragile at both ends (carried from WR-34)

**File:** `cmd/gitid/wiring.go:2010-2020`
**Severity:** WARNING

**Issue:** Unchanged. `strings.ReplaceAll(msg, b.home, "~")` fails open in two
directions: (a) `b.home` comes from `os.UserHomeDir()` (`:134`) and is never
symlink-resolved, while `os`/`git`/`exec` errors can carry the resolved path — on a
symlinked HOME (`/home/u → /mnt/data/u`, or the macOS `/tmp → /private/tmp` shape this
project's own sandboxes hit) nothing is scrubbed; (b) a sibling directory sharing the
prefix (`/Users/ramon` vs `/Users/ramonaldo`) is rewritten to `~aldo`.

**Fix:** anchor on a path separator and cover both spellings (longest first):

```go
for _, root := range b.homeSpellings() { // b.home + its filepath.EvalSymlinks form
    msg = strings.ReplaceAll(msg, root+string(os.PathSeparator), "~"+string(os.PathSeparator))
    msg = strings.ReplaceAll(msg, root, "~")
}
```

Add a symlinked-HOME fixture and a prefix-sharing-sibling fixture.

### WR-43: stale cross-reference to the symbol WR-18 deleted (carried from WR-33)

**File:** `e2e/git_configuration_pty_e2e_test.go:730`
**Severity:** WARNING

**Issue:** Still live, now at line 730. The comment points readers at
`cmd/gitid/gate_visual_regression_test.go`'s `splitAllowlistLine`, deleted in
`f7b7745`; `grep -rn "splitAllowlistLine" cmd/ internal/ e2e/` returns only this
comment. It implies a shared implementation that no longer exists — which matters
because CR-10 shows the two gates' predicate logic really *is* duplicated and really
does need one source of truth.

**Fix:** describe the inline parser, or replace the reference with a pointer to the
shared predicate helper introduced by CR-10's fix.

### WR-44: `keygen.AllowedSignersLine` accepts an unvalidated principal

**File:** `internal/keygen/signers.go:22-28`
**Severity:** WARNING

**Issue:** `AllowedSignersLine(email, pubLine)` interpolates `email` verbatim into a
line of `~/.ssh/allowed_signers` — the file git consults to decide whose signatures are
trusted. The package applies no validation of its own; safety depends entirely on
`commitGitArtifacts` happening to call `gitconfig.WriteFragment` (which does run
`validateEmail`, rejecting `\n`, `\r`, spaces and tabs) at an *earlier* step of the same
transaction. That ordering is incidental, undocumented, and unenforced: any future
caller of the exported `WriteAllowedSigners`/`WriteAllowedSignersReplacing` that does
not write a fragment first can inject an extra principal via an embedded newline.

**Fix:** validate at the boundary that owns the file.

```go
func AllowedSignersLine(email, pubLine string) (string, error) {
    if strings.ContainsAny(email, " \t\r\n") || !strings.Contains(email, "@") {
        return "", fmt.Errorf("keygen: allowed_signers principal is malformed: %q", email)
    }
    …
}
```

Add a table test covering `"a@b\nevil@c namespaces=\"git\" ssh-ed25519 AAAA"` and
`"a@b c@d"`.

---

## Verdict on the fixer's 7 skips (explicitly requested)

| Skip | Verdict |
|---|---|
| **WR-16** (undisclosed dir creation) | **Escalated — was masquerading.** Not a standalone deferrable item: it is one of three symptoms of a single defect, the ceremony building its disclosure from hardcoded UI strings. Folded into **CR-12** at BLOCKER. The "it changes golden frames" objection is real and is not a reason to leave a confirmed write undisclosed; the goldens exist to be updated deliberately. |
| **WR-25** (rollback re-loosens hardened root) | **Escalated — was masquerading.** Skipping a security defect in an error path because it "deserves its own verification pass" is the wrong trade, and WR-24 widened its blast radius in the same commit series. Raised to BLOCKER as **BL-15**. |
| **WR-30** (`Name: m.selected`) | **Skip reasonable.** Genuinely latent behind `gitCommitPending`. Carried as **WR-40**. |
| **WR-31** (match-strategy copy) | **Escalated — was masquerading.** "Wider blast radius" is accurate but the finding is a user-facing falsehood on a confirmation-gated write screen, contradicted by two other widgets on the same frame. Raised to BLOCKER as **CR-14**. |
| **WR-32** (Enter on checkbox) | **Skip reasonable, scope understated.** The fixer correctly flagged that the pane shares the bug. Carried as **WR-39** with both surfaces named. |
| **WR-33** (stale comment) | **Skip reasonable.** Cosmetic. Carried as **WR-43**. |
| **WR-34** (`displayMessage`) | **Skip reasonable.** Needs its own fixtures. Carried as **WR-42**. |

Two of the fixer's self-reported near-regressions during this pass (the added-row
attempt that broke `TestWizardGitStepButtonsAreFocusable`, and the naive window check
that reintroduced the off-by-one) were caught by the fixer itself and are not present
in the shipped code — I verified both by trace and by probe. That part of the account
is accurate.

---

## Notes for the escalation

The loop has now produced a regression in iterations 1 → 2 → 3 (CR-04 → CR-06 → CR-08)
and this pass produced none in that area: **CR-08, CR-09, WR-24, WR-26, WR-29 and the
`screenshot` half of WR-28 all genuinely landed and are probe-verified.** That is real
progress and the focus/enum area is now covered by behaviour-level tests.

What this iteration surfaces instead is that the *gates themselves* are the remaining
soft spot, and three of the five BLOCKERs are gate defects (CR-10, CR-11, CR-13). The
pattern across all three is identical to WR-28's: **a mechanism whose name and comment
assert a guarantee its body does not provide.** `regionPredicateSatisfied` says
"narrowly scoped" and accepts anything; `TestNegativeControls_AllProtectedRegionsDetectMutation`
says "every protected region on every screen" and tests one metadata field on one
region; `lint-screenshot` says the build-tag blindspot is closed and closes one of three
tags. Until those are fixed, a green `make test` / `make gate-visual-regression` remains
weak evidence for anything rendered inside an allowlisted region — which is most of the
Phase 4 screen.

**Recommended order:** CR-13 first (one Makefile line, unblocks detection everywhere),
then CR-11 and CR-10 together (they are one gate), then CR-12, CR-14 and BL-15. Do not
run another unattended automated iteration on CR-12/CR-14 — both change frozen golden
frames and need a human decision on the new copy.

---

_Reviewed: 2026-08-25T07:10:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
_Iteration: 4 (adversarial re-review of 04-REVIEW-FIX.md + full-state sweep)_
