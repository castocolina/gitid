---
phase: 04-git-configuration-screen
reviewed: 2026-08-25T09:40:00Z
depth: standard
iteration: 5
files_reviewed: 36
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
  - internal/identity/inventory.go
  - internal/identity/loader.go
  - internal/identity/loader_test.go
  - internal/identity/update.go
  - internal/keygen/signers.go
  - internal/screenshot/createflow.go
  - internal/screenshot/createflow_packet.go
  - internal/screenshot/createflow_packet_test.go
  - internal/screenshot/createflow_regions.go
  - internal/screenshot/createflow_test.go
  - internal/screenshot/normalize_test.go
  - internal/screenshot/region_disposition_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/ceremony.go
  - internal/tuikit/identities.go
  - internal/tuikit/identities_test.go
  - internal/tuikit/store.go
  - internal/tuikit/views.go
  - .planning/design/git-screen/visual-divergence-allowlist.txt
findings:
  critical: 5
  warning: 16
  info: 0
  total: 21
status: issues_found
---

# Phase 4: Code Review Report (iteration 5)

**Reviewed:** 2026-08-25T09:40:00Z
**Depth:** standard
**Files Reviewed:** 36
**Status:** issues_found

## Summary

Adversarial re-review of the 9 fixes claimed in `04-REVIEW-FIX.md` (iteration 4,
commits `5a6051d` … `37d667e`), an assessment of the 7 items the fixer explicitly
skipped, and a fresh sweep of the full current state. Every verdict below is backed
by an executed probe — mutation-revert (make the fix wrong, re-run the test that
allegedly proves it), fault injection, or real end-to-end execution against a
sandboxed HOME. Probe files were created, run, and deleted; the working tree outside
`.planning/` is clean and `make lint` reports 0 issues.

**Baseline gate status measured this session (not quoted from the fixer):**

| Gate | Result |
|---|---|
| `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` | PASS (1061 tests, 19 packages) |
| `make lint` (incl. `lint-tagged`) | PASS, 0 issues |
| `make gate-visual-regression` | PASS (10 tests, no SKIP) |
| `go test -tags e2e -race -run TestGitConfiguration_CompiledRealVsLiveDummyPTY ./e2e/...` | PASS (57 s) |

### What genuinely landed (each independently proven load-bearing by mutation-revert)

| Fix | Probe | Verdict |
|---|---|---|
| **BL-15** | Stubbed the `under(s.path)` guard → `TestRollbackKeepsHardenedRootSecuredWhenAFileUnderItFailsToRestore` fails with `~/.ssh mode after rollback = -rwxr-xr-x, want -rwx------` | **Landed.** The motivating scenario is genuinely covered: `journal.watchFile(staged.FinalPrivatePath)` (wiring.go:1917) puts the private key in `j.files`, so a failed key restore does reach `failedFilePaths`. |
| **WR-35** | Reverted `accounts()` to `identity.BuildInventoryDeps()` → test fails with `SSHHost = "github.com"` (my real `~/.ssh/config`) | **Landed.** |
| **CR-14** | Replaced `strategyCopy`'s gitdir case with `~/BUGGY/` → `TestStrategyLabelAgreesWithIncludeIfPreviewAndGitDirField` fails | **Landed** (product fix). Call sites at `:940`, `:2759`, `:3023`, `:2953` all thread the real gitdir. **But see CR-15 — neither gate can catch its recurrence.** |
| **CR-12** | Hardcoded the ceremony's `Backups` → `TestGitCeremonyForSourcesTargetsBackupsCreatesFromBackend` fails. Separately drove `GitWritePlan` + the REAL `commitGitTransaction` across 4 scenarios (fresh/existing × ForceSSH on/off): declared backup counts `0/1/3/4` matched actual `0/1/3/4` exactly, with correct `.bak.` naming. | **Landed and fidelity-verified.** Residuals: WR-47, WR-48. |
| **CR-11** | The new `assertAllComparableEqualRegionsAreMutationSensitive` really does mutate region TEXT and really is exhaustive over its scope — 116 comparable-equal regions checked | **Landed for the regions it covers.** But it deliberately `continue`s past every `!Equal` region — see CR-16. |
| **CR-13** | Injected `//go:build probetag` → `make lint-tagged` correctly fails with the guard message | **Landed for simple tags.** Bypassed by compound/parenthesized constraints — see WR-45. |
| **WR-36 / WR-37** | `RegionContinueDisabledReason` + `extractContinueDisabledReason` are gone (only a historical comment remains); `gitPaneFocusRing` is gone; the 16 `RegionName` constants match `AllRegionNames()`'s 16 entries | **Landed.** Structural half unfixed — see WR-58. |
| **CR-10 (the grammar itself)** | Reverted both branches to `\|\|` → `TestRegionPredicateSatisfiedRejectsSymmetricCases` fails on 3 subtests | **Grammar landed.** Its *production-level* regression test does not — see CR-17. |

### What is newly broken, or was never actually closed

1. **Both visual gates are structurally blind to the exact class of bug they were built to catch** (CR-15). I reintroduced CR-14's bug and `make gate-visual-regression` AND the real-PTY e2e suite both passed fully green.
2. **`ValidateRegionDiffs` never evaluates the predicate CR-10 fixed** (CR-16). 44 of 160 comparable regions accept an arbitrary live-side text mutation.
3. **CR-10's headline regression test is vacuous** (CR-17) — it passes with the bug fully restored. The fixer's claimed red-before-fix evidence for it is not reproducible.
4. **WR-44 is confirmed as a real security defect and escalated** (CR-18) — but via a mechanism the prior review did not identify, which its proposed fix would not have closed.
5. **Three ceremonies on a non-demo-bannered tab announce writes and backups that never happen** (CR-19).

---

## Critical Issues

### CR-15: both visual gates pass green with CR-14's BLOCKER reintroduced — a real/dummy comparison cannot detect a regression in code the two binaries share

**File:** `internal/screenshot/createflow.go:1288-1430` (`gitScreenSpecs`),
`e2e/git_configuration_pty_e2e_test.go:884-926` (`compareGitScreenCheckpoint`),
`.planning/design/git-screen/visual-divergence-allowlist.txt:3-19`
**Severity:** BLOCKER

**Issue:** The allowlist's own header states the gate's purpose: *"A structural divergence here means either a genuine, classified fixture-vs-live-data difference or a real regression the gate must catch."* It cannot. Both `cmd/gitid` and `cmd/gitid-dummy` render Configure-Git through the **same** `internal/tuikit/identities.go` code (the D-17 extraction, stated verbatim in the allowlist header). Any defect in that shared code changes **both** sides identically, so the comparison stays equal and the gate stays green.

Proven directly. I re-injected exactly the CR-14 bug that was classified BLOCKER and fixed in *this* iteration:

```go
// internal/tuikit/identities.go strategyCopy
return "gitdir (default) — applies inside " + strings.Replace(gitDir, "~/git/", "~/", 1)
```

Result:

```
$ make gate-visual-regression
    gate_visual_regression_test.go:318: gate-visual-regression: OK — 23 RequiredScreenSpecs frames checked
--- PASS: TestGateVisualRegression (1.53s)
--- PASS: TestNegativeControl_AllComparableEqualRegionsAreMutationSensitive (1.65s)
--- PASS: TestNegativeControl_AllGitScreenComparableEqualRegionsAreMutationSensitive (1.15s)
PASS   ok  github.com/castocolina/gitid/cmd/gitid  8.150s

$ go test -tags e2e -race -run TestGitConfiguration_CompiledRealVsLiveDummyPTY ./e2e/...
ok      github.com/castocolina/gitid/e2e        57.048s
```

Ten gate tests, 23 frames, a full 57-second raw-PTY real-vs-dummy comparison — and the user-visible falsehood CR-14 was raised about renders on every frame, undetected. The **only** thing that catches it is the unit test `TestStrategyLabelAgreesWithIncludeIfPreviewAndGitDirField`, which D-12 explicitly says cannot stand in for this proof (*"no unit test, no in-process tea.Msg synthesis stands in for this proof"*).

This is the root cause behind three consecutive iterations of gate work (WR-19 → WR-27 → CR-10; CR-04 → CR-11; WR-28 → CR-13). Each fix hardened the *classification* of divergences between two sides that are computed by the same function. The gate's real detection surface is the backend seam (real vs fixture data), not the renderer — and the renderer is where all of Phase 4's product code lives.

**Fix:** stop describing this gate as regression detection for the screen and add a mechanism that actually is one. Concretely, one of:

```go
// (a) pin the real binary's rendered regions against a COMMITTED golden, so a
//     shared-code change moves the frame away from a fixed reference, not away
//     from a co-moving twin.
func TestGitScreenRegionsMatchApprovedGolden(t *testing.T) {
    for _, region := range allGitScreenRegions() {
        want := readGolden(t, "git-form-filled", region) // committed under .planning/design/git-screen/goldens/
        got := normalizeGitCheckpoint(extractGitScreenRegion(realFrame, region))
        if got != want { t.Errorf(...) }   // fails on ANY shared-code drift
    }
}
```

or (b) assert semantic invariants across widgets on the same real frame for every git-screen region (the `TestStrategyLabelAgreesWith…` pattern, generalised), and demote the real-vs-dummy comparison to what it demonstrably is: a *backend-seam parity* check. Update the allowlist header and `gitScreenSpecs`' comments to say so — the current text asserts a guarantee the mechanism does not provide, which is the exact defect pattern this loop keeps re-manufacturing.

---

### CR-16: `ValidateRegionDiffs` never evaluates the predicate — 44 of 160 comparable regions accept an arbitrary live-side text mutation, and CR-11's exhaustive control deliberately skips all of them

**File:** `internal/screenshot/createflow_packet.go:1386-1485` (`ValidateRegionDiffs`,
`else if !region.Equal` at `:1457-1462`),
`cmd/gitid/gate_visual_regression_test.go:537-577` (`assertAllComparableEqualRegionsAreMutationSensitive`, the `continue` at `:546-548`)
**Severity:** BLOCKER

**Issue:** CR-10 put `regionPredicateSatisfied` on the `BuildRegionDiffs` path only. `ValidateRegionDiffs` — which re-derives and re-checks *every other* invariant (hashes, comparability, equality, classification, decision linkage, region inventory) — never calls it. Its `!region.Equal` branch checks only that the record's metadata equals the disposition's metadata:

```go
} else if !region.Equal {
    disposition, found := RegionDispositionFor(spec, region.Name)
    if !found || region.Divergence != disposition.Divergence || region.Decision != disposition.Decision ||
        region.Justification != disposition.Decision+": "+disposition.Reason || region.Classification != disposition.Classification {
        return fmt.Errorf("…unexplained or unclassified divergence…")
    }
}   // region.LiveText is never examined
```

CR-11's new exhaustive control then skips exactly these regions (`if !region.Comparable || !region.Equal { continue }`), with a comment claiming they are *"covered by TestNegativeControl_UnclassifiedDifferenceRejected instead"* — that test only proves a *cleared metadata field* is rejected, never a text change.

Measured with a probe that iterates every region of every `RequiredScreenSpecs()` screen, appends `"\nGATE-CANARY-TOTALLY-BROKEN"` to `LiveText`, recomputes the hash, and calls `ValidateRegionDiffs`:

```
PROBE totals: comparable-equal=116  comparable-differing(dispositioned)=44  acceptedMutations=44
ACCEPTED arbitrary live mutation: screen=git-form-filled region=git-strategy
ACCEPTED arbitrary live mutation: screen=git-form-filled region=git-preview
ACCEPTED arbitrary live mutation: screen=git-form-filled region=git-form-fields
ACCEPTED arbitrary live mutation: screen=review-readonly region=git-ceremony
ACCEPTED arbitrary live mutation: screen=result-success region=git-ceremony
… 44/44 accepted, 0 rejected
```

Every git-screen region that carries product-specific copy is in that 44. So the stored `REGION-DIFFS.json` evidence packet — the artifact that gets published and re-validated — carries no integrity guarantee at all for 27.5 % of the regions, and CR-11's headline "exhaustive negative control" cannot, by construction, ever reach them.

**Fix:** enforce the predicate on both paths and extend the control to cover dispositioned regions:

```go
// createflow_packet.go, inside ValidateRegionDiffs' `else if !region.Equal` branch:
if !regionPredicateSatisfied(disposition.Predicate, region.LiveText, region.ApprovedText) {
    return fmt.Errorf("screenshot: ValidateRegionDiffs: frame %q region %q no longer satisfies disposition predicate %q",
        spec.ScreenID, region.Name, disposition.Predicate)
}
```

```go
// gate_visual_regression_test.go — replace the `continue` with a second control:
// a dispositioned region must reject a mutation that BREAKS its predicate.
if !region.Equal {
    m.LiveText = strings.ReplaceAll(m.LiveText, needleOf(disposition.Predicate), "MUTATED")
    …
    if err := screenshot.ValidateRegionDiffs(data, …); err == nil {
        t.Errorf("dispositioned region %q/%q accepts a mutation that violates its own predicate", …)
    }
}
```

This must be red before the change lands — the probe above is the exact fixture to reuse.

---

### CR-17: CR-10's production-disposition regression test passes with the bug fully restored — the fixer's red-before-fix evidence for it is not reproducible

**File:** `internal/screenshot/region_disposition_test.go:91-135`
(`TestBuildRegionDiffsRejectsUnrelatedLiveRegressionUnderProductionPredicate`)
**Severity:** BLOCKER

**Issue:** The prior review's fix instruction was explicit: *"add a test that mutates the **live** side of a production disposition (not a synthetic one) and asserts `BuildRegionDiffs` errors… Until such a test exists and is red before the fix, WR-27 must not be recorded as closed."* The fixer added this test and reported: *"Red-before-fix: reverted the grammar to the old `||`/`!…||!…` form and re-ran the new regression test — it failed with `CR-10 regression: BuildRegionDiffs accepted an unrelated live-side regression…`"*

That is not reproducible. I reverted **both** predicate branches to the exact pre-CR-10 grammar and the test stays green:

```go
case strings.HasPrefix(predicate, "contains:"):
    return strings.Contains(live, needle) || strings.Contains(approved, needle)
case strings.HasPrefix(predicate, "absent:"):
    return !strings.Contains(live, needle) || !strings.Contains(approved, needle)
```
```
=== RUN   TestBuildRegionDiffsRejectsUnrelatedLiveRegressionUnderProductionPredicate
--- PASS (0.00s)
```

The reason, from a probe against the same fixture:

```
PROBE err = screenshot: BuildRegionDiffs: frame "cr-10-regression-real-disposition"
            region "breadcrumb" differs without a screen-specific declared disposition
PROBE predicate = "absent:\"clientB\""
PROBE predicateSatisfied = false
```

`BuildRegionDiffs` iterates `AllRegionNames()` and returns on the **first** failing region. The synthetic fixture's `breadcrumb` region differs and carries no disposition, so the function errors there — before or regardless of the sidebar predicate. The test's assertion (`if err == nil { t.Fatal }`) is therefore satisfied by an unrelated missing-disposition error and is completely insensitive to the predicate grammar it claims to guard.

Across the whole `-tags screenshot` suite, reverting both branches turns exactly **one** test red — the pure-unit `TestRegionPredicateSatisfiedRejectsSymmetricCases`. Every `BuildRegionDiffs`-level test is predicate-insensitive.

**Fix:** make the fixture minimal so the predicate is the only thing that can fail, and assert on the error *text*, not merely on non-nil:

```go
spec.RegionDispositions = []RegionDisposition{prodDisposition}
// give EVERY other region that would differ a blanket disposition, or make the
// fixture render only the sidebar region, so nothing else can error first.
_, err := BuildRegionDiffs(…)
if err == nil || !strings.Contains(err.Error(), "disposition predicate") {
    t.Fatalf("CR-10 regression: want a predicate rejection, got %v", err)
}
```

Then re-run the mutation-revert above and confirm it is genuinely red.

---

### CR-18 (security): a comma in the Git e-mail writes a wildcard principal into `~/.ssh/allowed_signers`, making the signing key verify as ANY identity (WR-44 confirmed and escalated)

**File:** `internal/keygen/signers.go:22-28` (`AllowedSignersLine`),
`internal/gitconfig/fragment.go:144-152` (`validateEmail`),
`internal/tuikit/identities.go:809-811` (`gitForm.valid`),
`cmd/gitid/wiring.go:1321`
**Severity:** BLOCKER (security)

**Issue:** WR-44 is real, but the exploitable mechanism is **not** the embedded newline the prior review proposed a fix for. `~/.ssh/allowed_signers`'s first field is a **comma-separated list of principal patterns**, and `*` is a valid pattern. `AllowedSignersLine` interpolates the e-mail verbatim into that field, and no layer rejects a comma or an asterisk:

- `gitForm.valid()` requires only `strings.Contains(email, "@")`.
- `gitconfig.validateEmail` rejects `\n \r " " \t` and requires `@` — commas and `*` pass.
- `keygen.AllowedSignersLine` is documented as deliberately unvalidating (*"The email is used byte-identically to the supplied value (Pitfall 8)"*).

End-to-end through the **real** backend (`commitGitTransaction`, seeded HOME, `Email: "victim@corp.test,*"` — a value the form accepts and the ceremony displays as an ordinary e-mail):

```
PROBE allowed_signers on disk:
# BEGIN gitid managed: acme
victim@corp.test,* namespaces="git" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5…
# END gitid managed: acme
```

And the effect of that line, verified against real OpenSSH:

```
$ ssh-keygen -Y verify -f allowed -I victim@corp.test        -n git -s msg.sig < msg
Good "git" signature for victim@corp.test with ED25519 key SHA256:4WtIi8…
$ ssh-keygen -Y verify -f allowed -I ceo@othercompany.example -n git -s msg.sig < msg
Good "git" signature for ceo@othercompany.example with ED25519 key SHA256:4WtIi8…
```

Git commit-signature verification is silently reduced to "any signature by this key verifies as anybody". Aggravating factors:

- The value **persists and re-applies**: `gitconfig.ReadFragment` (reader.go:112-129) truncates a value at the first newline but passes a comma straight through, so the poisoned e-mail is read back into the form and re-written on every subsequent Configure-Git write.
- `gitconfig.RemoveAllowedSignersLine` (reader.go:199) matches `fields[0] == identityEmail` **exactly**, so the wildcard line cannot be removed by e-mail — the very CR-01 prefix-safety reasoning in that function's own doc comment is defeated by the field it never validates.
- The prior review's proposed fix (`ContainsAny(" \t\r\n")` + require `@`) does **not** close this.

**Fix:** validate at the boundary that owns the file, and make the function fallible:

```go
// internal/keygen/signers.go
var allowedSignersPrincipal = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)

// AllowedSignersLine … The principal MUST be a single, literal address: OpenSSH
// reads field 1 as a comma-separated list of PATTERNS, so a comma or a wildcard
// character there silently widens the key's trust to other identities.
func AllowedSignersLine(email, pubLine string) (string, error) {
	if !allowedSignersPrincipal.MatchString(email) {
		return "", fmt.Errorf("keygen: allowed_signers principal is not a single literal address: %q", email)
	}
	…
}
```

Propagate the error through `WriteAllowedSignersReplacing` and the four `internal/identity` call sites, and mirror the same check in `gitForm.valid()` so the ceremony is never reachable with such a value. Add a table test covering `"a@b.c,*"`, `"a@b.c,other@d.e"`, `"*"`, `"a@b.c "`, and `"a@b\nevil@c namespaces=\"git\" ssh-ed25519 AAAA"`.

---

### CR-19: the Delete, Edit-SSH and Clone ceremonies announce backups and writes that never happen, on a tab that carries no demo banner

**File:** `internal/tuikit/identities.go:1980` (edit ceremony), `:2251`, `:2267`, `:2310`
(delete), `cmd/gitid/wiring.go:340-361` (`DemoBanner`, `Persist`)
**Severity:** BLOCKER

**Issue:** `realBackend.Persist` performs a real write for exactly one action:

```go
func (b *realBackend) Persist(state tuikit.DemoState, action tuikit.Action) tuikit.DemoState {
	switch a := action.(type) {
	case tuikit.Reset:      return b.InitialState()
	case tuikit.AddIdentity: return b.persistCreate(state, a)
	default:                 return tuikit.Reduce(state, action)   // in-memory only
	}
}
```

There is no `CommitDelete`/`CommitEdit` on `*realBackend` at all. Meanwhile `DemoBanner` returns **false** for `TabIdentities`, so the user gets no "this is a demo" signal, and the delete flow presents a full confirmation ceremony that declares a backup:

```go
// identities.go:2251, :2267
Backups: []string{NewBackupPath("~/.ssh/config"), NewBackupPath("~/.gitconfig")},
// rendered by ceremony.go:318-322 as
//   Backup → ~/.ssh/config.backup.2026-08-25T09-40-00Z
//     (written first — restore it to undo)
```

Nothing is written and nothing is backed up. The identity vanishes from the list, the receipt claims a recovery point, and the identity reappears on the next launch. CLAUDE.md makes this ceremony the auditable contract for mutating the user's real files; here it is an unconditional falsehood on the one destructive operation in the screen.

This may be deliberate deferred scope (Phase 5.7), but it is not *marked* as such anywhere the user can see, and `DemoBanner`'s own exclusion of `TabIdentities` actively suppresses the only signal that would have made it honest.

**Fix:** either wire the writes, or make the unwired flows visibly inert until they are:

```go
// cmd/gitid/wiring.go
func (b *realBackend) DemoBanner(tab tuikit.TabID) bool { return true } // until Persist covers every action
// or, better: gate the ceremony itself
func (b *realBackend) DeleteDisabledReason() (string, bool) {
    return "Delete is not wired to disk yet — nothing will be written.", true
}
```

and add a test asserting that every `Action` a ceremony can dispatch from `TabIdentities` is either handled by a real `Persist` branch or reports a disabled reason.

---

## Warnings

### WR-45: CR-13's build-tag guard is bypassed by compound and parenthesized build constraints, and `KNOWN_BUILD_TAGS` can drift from the `go vet` lines it claims to mirror

**File:** `Makefile:218-232` (`KNOWN_BUILD_TAGS`, `lint-tagged`), `:203-207` (the comment)
**Severity:** WARNING

**Issue:** The guard's own doc comment claims completeness: *"The guard loop below fails the instant a NEW `//go:build <tag>` line appears anywhere in the tree without a matching `go vet -tags <tag>` line added here — so this exact blindspot … cannot recur a third time on a future tag."* Two probes disprove it. Each file below contains `var x int = "nope"`:

```go
//go:build screenshot && probehidden     →  make lint-tagged: "0 issues."  ✗
//go:build (probea || probeb)            →  make lint-tagged: "0 issues."  ✗
//go:build probetag                      →  make lint-tagged: guard fires  ✓
```

`grep -hoE '^//go:build [A-Za-z0-9_]+' | awk '{print $2}'` extracts only the first bare identifier, so `screenshot && probehidden` reports `screenshot` (a known tag) and the parenthesized form matches nothing at all — while neither file is compiled by any `go vet -tags` line. A negated constraint (`//go:build !x`) is safe, because default vet compiles it.

Second, weaker gap: the guard compares found tags against the `KNOWN_BUILD_TAGS` *variable*, but the `go vet -tags` lines are hardcoded separately. Adding a name to the variable silences the guard without wiring the vet, which is precisely the "declared, not enforced" split that produced CR-13.

**Fix:** derive the tag list from the recipe, and parse the whole constraint:

```make
lint-tagged:
	@found=$$(find . -name '*.go' -not -path './.planning/*' -print0 \
	    | xargs -0 grep -hE '^//go:build ' 2>/dev/null \
	    | sed -e 's|^//go:build ||' -e 's/[()!]/ /g' -e 's/&&/ /g' -e 's/||/ /g' \
	    | tr ' ' '\n' | grep -E '^[A-Za-z0-9_]+$$' | sort -u); \
	vetted=$$(grep -oE '^\tgo vet -tags [A-Za-z0-9_]+' $(MAKEFILE_LIST) | awk '{print $$4}' | sort -u); \
	missing=$$(comm -23 <(echo "$$found") <(echo "$$vetted")); \
	[ -z "$$missing" ] || { echo "lint-tagged: ungated build tag(s): $$missing"; exit 1; }
```

Then add a compound-tag fixture to whatever gates the Makefile, so this specific bypass stays closed.

### WR-46: the `git-strategy` region — where CR-14's bug actually rendered — carries no predicate at all, and its disposition still quotes the buggy pre-CR-14 label

**File:** `internal/screenshot/createflow.go:1332-1333` (`gitStrategyDisposition`),
`:1328-1331` (`emptyFormFieldsDisposition`, `breadcrumbDisposition`)
**Severity:** WARNING

**Issue:** The CR-14 fix comment at `:1315-1321` claims the new `contains:"gitdir:~/git/"` predicate *"actively guards against CR-14's exact regression recurring."* It guards `git-preview` only. The `git-strategy` region — the one that renders `strategyCopy`'s output — uses blanket `uxRegionDifference` with an empty `Predicate`, which `regionPredicateSatisfied` treats as *always true*. Same for `breadcrumb` and `git-form-empty`'s `git-form-fields`.

Worse, the disposition's own reason string still documents the buggy text:

```go
gitStrategyDisposition := uxRegionDifference(RegionGitStrategy, "identity-name", "CTX-D-12",
    "the gitdir strategy option's label (\"gitdir (default) — applies inside ~/<identity>/\") …")
                                                                          ^^^^^^^^^^^^^^ pre-CR-14 shape
```

The reviewed, recorded justification for accepting divergence in that region describes exactly the string CR-14 declared a BLOCKER. The e2e allowlist has no `git-strategy` entry at all (the region normalizes to equal via `gitScreenGitDirPattern`), so neither gate carries a scoped guard for it.

**Fix:** re-predicate `gitStrategyDisposition` with `contains:"gitdir:~/git/"` (the same guard `gitPreviewDisposition` uses) and correct the reason text to `"~/git/<identity>/"`. Audit the remaining blanket dispositions in `gitScreenSpecs()` the same way.

### WR-47: `GitWritePlan.CreatedDirs` omits the intermediate parents `MkdirAll` creates — probe shows `~/git` created on a fresh home and never disclosed

**File:** `cmd/gitid/wiring.go:772-784` (`GitWritePlan`), `:946-978` (`ensureDir`)
**Severity:** WARNING

**Issue:** `ensureDir` walks up from the target and creates **every** missing ancestor (`for current := clean; ; current = filepath.Dir(current)` … `os.Mkdir(missing[i], mode)`), recording each in `createdDirs`. `GitWritePlan` only tests the three leaf paths. Probe against a fresh home:

```
PLAN creates: [~/.gitconfig.d ~/git/acme]
post-commit dir exists: …/001/.gitconfig.d
post-commit dir exists: …/001/git/acme
post-commit dir exists: …/001/git          ← created, never disclosed
```

CR-12 was raised precisely because directory creation was an undisclosed side effect of a confirmed write. The leaf case is now disclosed; the parent case is not.

**Fix:** mirror `ensureDir`'s ancestor walk in the plan:

```go
addCreated := func(target string) {
    var missing []string
    for cur := filepath.Clean(target); cur != filepath.Clean(b.home); cur = filepath.Dir(cur) {
        if fileExists(cur) { break }
        missing = append(missing, b.displayPath(cur))
    }
    for i := len(missing) - 1; i >= 0; i-- { plan.CreatedDirs = append(plan.CreatedDirs, missing[i]) }
}
```

and extend `TestGitWritePlanReportsFreshHomeNoBackupsButCreatesEveryDir` to expect `~/git` alongside `~/git/acme`.

### WR-48: the corrected ceremony renders two byte-identical backup lines and a literal `<timestamp>` placeholder, then tells the user to "restore it to undo"

**File:** `cmd/gitid/wiring.go:60-64` (`backupSuffixPreview`), `:762-767`,
`internal/tuikit/ceremony.go:318-322`
**Severity:** WARNING

**Issue:** On the phase's headline flow (edit an existing identity with Force SSH on) the confirmation now reads:

```
Backup → ~/.gitconfig.d/acme.bak.<timestamp>
Backup → ~/.gitconfig.bak.<timestamp>
Backup → ~/.gitconfig.bak.<timestamp>          ← byte-identical to the line above
Backup → ~/.ssh/allowed_signers.bak.<timestamp>
  (written first — restore it to undo)
```

Two things degrade an auditable-contract screen. (1) The duplicate is real and correct — `~/.gitconfig` genuinely is backed up twice — but the disclosure gives the user no way to tell that from a rendering bug, and no way to know which of the two to restore. (2) `<timestamp>` is a literal placeholder token in user-facing copy; the instruction "restore it to undo" points at a filename that will never exist verbatim. CR-12's stated defect was *"points the user at a path that is never created"*; the naming convention is now right, the actionability is not.

**Fix:** collapse repeats and label the ordering:

```go
// wiring.go — annotate rather than repeat
plan.Backups = append(plan.Backups, b.displayPath(b.gitconfigPath)+backupSuffixPreview+" (again, after the includeIf write)")
// ceremony.go — say what the token means
b.WriteString(styleFaint.Render("  (each written before its file is modified; <timestamp> is filled in at write time — the exact paths are listed in the receipt)") + "\n")
```

### WR-49: unchecking Force SSH is a silent no-op — probe-confirmed round trip (was WR-38)

**File:** `internal/tuikit/store.go:244`, `internal/tuikit/identities.go:1788`,
`cmd/gitid/wiring.go:1280-1289`
**Severity:** WARNING

**Issue:** Confirmed by executing the real transaction twice against one sandboxed home:

```
after ForceSSH=true  write : ForceSSH=true
after ForceSSH=false write : ForceSSH=true     ← re-read from disk
~/.gitconfig now:
  # BEGIN gitid managed: provider-rewrite:github.com
  [url "git@github.com:"]
      insteadOf = https://github.com/
  # END gitid managed: provider-rewrite:github.com
```

`commitGitArtifacts` has no removal branch (`if spec.ForceSSH && spec.Provider != ""` writes; `false` does nothing — D-06, intentional), but the reducer writes `row.ForceSSH = a.ForceSSH` unconditionally. So the session shows `☐` while disk says `☑`, and the next launch flips the checkbox back. The ceremony's WR-05 note explains the *disk* state correctly, which makes the *list* state the sole liar.

**Fix:** as previously recommended — either re-read after the Git commit (the `persistCreate` path already does `return b.InitialState()`), or drop `ForceSSH` from the `ConfigureGit` payload so the row keeps its disk-derived value. Add a write-then-reload test asserting the checkbox survives the round trip. Separately, document and test the provider-scoped semantics (`Reconstruct` sets `ForceSSH: true` for *every* identity on that host).

### WR-50: Enter on the Force-SSH checkbox — and on the gitdir field — opens the write ceremony instead of editing (was WR-39)

**File:** `internal/tuikit/identities.go:2160-2167` (pane), `:2499-2518` (wizard),
`:842-845` (`handleEdit`'s unreachable `"enter"` clause)
**Severity:** WARNING

**Issue:** Unchanged and confirmed. `handleGitKey`'s `case "enter"` fires regardless of `m.gitFocus` and regardless of `m.gitPaneForm.gitDirFocused`, so Enter while focused on the checkbox — or mid-edit in the gitdir path field — jumps straight to the write ceremony. `handleEdit`'s `case gitFieldForceSSH: if key == "enter" { g.forceSSH = !g.forceSSH }` is therefore dead code that reads as if the behaviour were implemented.

**Fix:** route Enter to the focused control first in **both** surfaces, then keep (and test) `handleEdit`'s now-reachable `"enter"` clause:

```go
case "enter":
    if m.gitFocus == gitFieldForceSSH || m.gitPaneForm.gitDirFocused {
        m.gitPaneForm = m.gitPaneForm.handleEdit(msg, m.gitFocus)
        return keyResult{model: m, handled: true}
    }
    if m.gitPaneForm.valid() { … }
```

### WR-51: `ConfigureGit.Name` reads `m.selected` — the divergence does not require async timing (was WR-40, assessment corrected)

**File:** `internal/tuikit/identities.go:1786-1790`, `:1725-1735` (`selectedIdentity`)
**Severity:** WARNING

**Issue:** The prior review and the fixer both classified this as latent behind `gitCommitPending`. That framing is wrong. `selectedIdentity` **silently falls back to row 0** whenever `m.selected` is not present in `s.Identities`:

```go
for _, row := range s.Identities { if row.Name == m.selected { return row, true } }
if len(s.Identities) > 0 { return s.Identities[0], true }   // fallback, `ok` discarded at :2128
```

`spec.Identity` is captured from `sel.Name` (the fallback row); `ConfigureGit{Name: m.selected}` targets the stale name. Whenever `m.selected` goes stale — the identity list is re-read from disk on every `InitialState()`, and nothing reconciles `m.selected` against it — the write lands on identity A while the reducer updates nothing (no row matches), and the note reads `Git identity "<stale>" configured.` No async window is involved; the two values diverge at the same instant. There is also no guard on `ok` at `:2128`, so with zero identities the ceremony renders `Write Git identity for ""` and dispatches a spec the backend rejects.

**Fix:** `Name: spec.Identity`, note text `spec.Identity`, and honour `ok`:

```go
sel, ok := m.selectedIdentity(s)
if !ok { return keyResult{model: m, handled: true} }
```

### WR-52: `handleWizardClick` routes the algorithm row through a bare literal `5` (was WR-41)

**File:** `internal/tuikit/identities.go:2971-2976`
**Severity:** WARNING

**Issue:** Unchanged. `w.focus = 5` / `w.form.setFocus(5)` is correct only as long as `sshFieldPrefix…sshFieldPort` and `wizardFocusKeySource` keep their current values — the exact shape that produced CR-04 and CR-06.

**Fix:** `w.focus = wizardFocusKeyBody` at both sites, plus the membership-guard test asserting every slot a click handler can assign is a member of `wizardStep0FocusRing(w.keySource)`.

### WR-53: `displayMessage`'s substring replace fails open on symlinked HOMEs and mangles prefix-sharing siblings (was WR-42)

**File:** `cmd/gitid/wiring.go:2140-2145`
**Severity:** WARNING

**Issue:** Unchanged. `strings.ReplaceAll(msg, b.home, "~")` is unanchored, and `b.home` comes from `os.UserHomeDir()` without `filepath.EvalSymlinks`. On a symlinked HOME (including this project's own `/tmp → /private/tmp` sandbox shape) nothing is scrubbed and raw absolute paths reach the receipt; `/Users/ramonaldo/x` becomes `~aldo/x`. Note `displayPath` (`:2118-2126`) is correct — it uses `filepath.Rel` — so the two sibling helpers disagree.

**Fix:** anchor on the separator and cover both spellings, longest first, as previously specified; add a symlinked-HOME fixture and a prefix-sharing-sibling fixture.

### WR-54: stale cross-reference to `splitAllowlistLine`, a symbol deleted three iterations ago (was WR-43)

**File:** `e2e/git_configuration_pty_e2e_test.go:730`
**Severity:** WARNING

**Issue:** Confirmed still live. `grep -rn "splitAllowlistLine" cmd/ internal/ e2e/` returns exactly one hit — this comment. It claims a shared implementation that does not exist, which is materially misleading given that CR-10/CR-16 show the two gates' predicate logic really is duplicated and really does need one source of truth.

**Fix:** describe the inline parser, or point at a shared predicate helper once one exists.

### WR-55: `identity.Update` writes `allowed_signers` **before** the validating `WriteFragment` — and has zero production callers

**File:** `internal/identity/update.go:89-92` vs `:104-106`
**Severity:** WARNING

**Issue:** The only thing that keeps CR-18's newline vector closed on the Phase-4 path is ordering: `commitGitArtifacts` runs `gitconfig.WriteFragment` (which calls `validateEmail`) at step `git-fragment`, before `keygen.WriteAllowedSignersReplacing` at step `allowed-signers`. `identity.Update` reverses exactly that:

```go
signersLine := keygen.AllowedSignersLine(edited.GitEmail, pubLine)   // :89  unvalidated
deps.WriteAllowedSigners(edited.AllowedSignersPath, existing.Name, signersLine)  // :90  WRITES
…
deps.WriteFragment(edited.FragmentPath, edited.GitName, edited.GitEmail, …)      // :104 validates, aborts
```

A malformed e-mail is persisted into `~/.ssh/allowed_signers` and *then* the update fails, leaving the corrupted file behind. This is not currently reachable — `grep -rn "identity.Update("` outside `_test.go` returns nothing, so the function is dead code — but it is live proof that the "safety by incidental ordering" the package relies on is already violated in-tree.

**Fix:** fix CR-18 at the boundary (which makes the ordering irrelevant), and either delete `identity.Update` or wire it. Independently, move `WriteFragment` ahead of `WriteAllowedSigners` in `Update` so the two write paths agree on ordering.

### WR-56: `NewBackupPath` still mints a naming convention `filewriter` never produces — CR-12's stated minimum fix was not applied

**File:** `internal/tuikit/store.go:448-451`, consumers at
`internal/tuikit/identities.go:1938`, `:1980`, `:2251`, `:2267`, `:2310`
**Severity:** WARNING

**Issue:** The prior review's CR-12 said: *"At minimum, and independently of the seam work, `NewBackupPath`'s suffix must be changed to `.bak.` so the declared and actual naming conventions cannot disagree, and a test must pin `NewBackupPath`'s format against `filewriter`'s."* Neither was done. `NewBackupPath` still returns `<file>.backup.<ISO>` while `filewriter.Write` (`filewriter.go:138`) mints `<file>.bak.<unix-nanos>`, and five UI call sites still use it. The Configure-Git ceremony was migrated off it; every other ceremony was not (see CR-19 for why that currently matters more than a naming nit).

**Fix:** change the suffix to `.bak.` and add the pinning test:

```go
func TestNewBackupPathMatchesFilewriterNamingConvention(t *testing.T) {
    real, _ := filewriter.Write(seeded, []byte("x"), 0o600)
    if !strings.Contains(filepath.Base(NewBackupPath("~/f")), ".bak.") ||
       !strings.Contains(filepath.Base(real), ".bak.") {
        t.Fatalf("declared %q and actual %q disagree", NewBackupPath("~/f"), real)
    }
}
```

### WR-57: `normalizeForRegion` is a no-op whose doc comment describes work it does not do

**File:** `internal/screenshot/createflow_packet.go:1487-1494`
**Severity:** WARNING

**Issue:**

```go
// normalizeForRegion strips disposable absolute temp-path prefixes and
// timestamp strings from a capture for stable region comparison — retaining
// full commands, outputs, config values, ANSI semantic codes, and markers.
func normalizeForRegion(text string) string {
	return text
}
```

It is called six times on the hot path of `BuildRegionDiffs` and does nothing. Whether the normalization genuinely moved upstream (the trailing comment suggests it did) or was lost, the comment as written is the same "asserts a guarantee the body does not provide" defect that produced WR-28, CR-13, CR-11 and CR-17 — and it sits on the function every region hash is derived through.

**Fix:** delete the function and inline the identity, or restore the normalization and test it. If it truly is redundant with `CaptureCreateFlowScreens`' `normalizeTimestamps`, say that in one line and delete the body's misleading claim.

### WR-58: `AllRegionNames()` still has no exhaustiveness guard — a new `RegionName` constant is silently never compared

**File:** `internal/screenshot/createflow_regions.go:637-656`
**Severity:** WARNING

**Issue:** WR-36's concrete half landed (`RegionContinueDisabledReason` and its extractor are gone; 16 constants match 16 slice entries today). The structural half did not. `BuildRegionDiffs` and `ValidateRegionDiffs` both iterate `AllRegionNames()`, so any constant omitted from that hand-maintained slice is invisible to every gate, with no compile-time or test-time link between the two.

**Fix:** add the guard the prior review specified — a test that enumerates every declared `RegionName` (hand-kept `EveryRegionConstant()` or a `go:generate` extraction) and asserts membership in `AllRegionNames()`.

### WR-59: `gitScreenPredicateSatisfied` — the e2e half of CR-10's fix — has no direct test coverage

**File:** `e2e/git_configuration_pty_e2e_test.go:872-882`
**Severity:** WARNING

**Issue:** `grep -rn "gitScreenPredicateSatisfied" e2e/` returns its definition and one call site. There is no unit test. Its sibling `regionPredicateSatisfied` is the *only* function in this pair whose grammar is actually verified (`TestRegionPredicateSatisfiedRejectsSymmetricCases`), and the two are kept in sync by comment alone. Given CR-17, comment-enforced synchronisation between two copies of security-relevant gate logic is not adequate.

**Fix:** extract the predicate into one shared, tested helper both gates import (the e2e package can import an exported `screenshot.PredicateSatisfied`), or mirror `TestRegionPredicateSatisfiedRejectsSymmetricCases` verbatim into the e2e package.

### WR-60: `gpg.ssh.allowedSignersFile` is written as an absolute HOME path, unlike every other gitid-written path in `~/.gitconfig`, and is not disclosed in the ceremony

**File:** `cmd/gitid/wiring.go:1305`, `internal/gitconfig/fragment.go:87-92`
**Severity:** WARNING

**Issue:** Observed on disk after a real commit:

```
# BEGIN gitid managed: acme
[includeIf "gitdir:~/git/acme/"]
	path = ~/.gitconfig.d/acme            ← tilde form
# END gitid managed: acme
[gpg "ssh"]
	allowedSignersFile = /var/folders/…/001/.ssh/allowed_signers   ← absolute
```

Two consequences: the file is not portable (a moved or differently-named HOME silently breaks signature verification, with no gitid diagnostic), and the raw absolute path is written outside any managed block — so it is also outside `filewriter.ReplaceBlock`'s idempotency guarantee and outside the doctor's reserved-block accounting. It is likewise absent from the ceremony's `Targets`/`Preview` disclosure, even though it mutates `~/.gitconfig` a third time in the same transaction (after `WriteIncludeIf` and `WriteProviderRewrite`).

**Fix:** write the tilde form and disclose the key:

```go
if writeErr := gitconfig.SetAllowedSignersFile(b.gitconfigPath, "~/.ssh/allowed_signers"); writeErr != nil { … }
```

(git expands `~` for this value.) Add it to `GitWritePlan`'s preview text, and a test asserting `~/.gitconfig` contains no absolute HOME path after a commit.

---

## Verdict on the fixer's 7 skips (explicitly requested)

| Skip | Verdict this pass |
|---|---|
| **WR-38** Force-SSH desync | **Confirmed, evidence strengthened.** Reproduced end-to-end against the real backend (two `commitGitTransaction` runs, disk re-read). Carried as **WR-49**, WARNING — it is a silent no-op on a user's explicit choice, not merely a stale row. |
| **WR-39** Enter on checkbox | **Confirmed, scope widened.** Also fires while the *gitdir path field* is focused (`m.gitPaneForm.gitDirFocused` is not consulted at `:2160`). Carried as **WR-50**, WARNING. |
| **WR-40** `Name: m.selected` | **Assessment corrected — no longer "latent behind gitCommitPending".** `selectedIdentity`'s discarded-`ok` fallback makes the divergence synchronous. Carried as **WR-51**, WARNING (not escalated: it needs a stale `m.selected`, which requires an external list change). |
| **WR-41** bare `5` | **Confirmed unchanged.** Carried as **WR-52**, WARNING. Correctly assessed as latent. |
| **WR-42** `displayMessage` | **Confirmed unchanged.** Carried as **WR-53**, WARNING. Note the sibling `displayPath` already does it correctly — the fix is a two-line alignment, not a research task. |
| **WR-43** stale comment | **Confirmed unchanged.** Carried as **WR-54**, WARNING. |
| **WR-44** unvalidated principal | **Confirmed and ESCALATED to BLOCKER (CR-18).** The fixer was right to flag it as the priority. It is worse than described: the exploitable field is not the newline (blocked today by widget sanitisation, `ReadFragment`'s truncation, and step ordering) but the **comma-separated principal list**, which every existing layer accepts and which the prior review's proposed fix would not have caught. Proven end-to-end through `commitGitTransaction` and against real `ssh-keygen -Y verify`. |

**Interaction between the skips:** WR-49 and WR-51 both write into the same `ConfigureGit` payload and should be fixed together (one is "stop trusting the optimistic reducer", the other is "stop reading live model state") — fixing only one leaves the reducer half-authoritative. WR-55 is the same defect family as CR-18 and must be resolved by the same boundary validation, not separately.

---

## Convergence assessment (explicitly requested)

**This loop has not converged, and the two halves of the work are converging at very different rates.**

**Product code is converging.** Every product-side fix this pass survived an independent mutation-revert probe: BL-15, WR-35, CR-14, CR-12 (including a 4-scenario fidelity check against the real transaction), WR-36, WR-37. The CR-04 → CR-06 → CR-08 regression chain has stayed closed for two consecutive iterations. Of the five BLOCKERs raised this pass, only two (CR-18, CR-19) are in product code, and CR-19 is plausibly deferred scope rather than a defect introduced by this phase.

**The evidence apparatus is not converging, and the trend is flat.** Four consecutive iterations have each produced a new BLOCKER in the same three files — `internal/screenshot/createflow.go`, `cmd/gitid/gate_visual_regression_test.go`, `Makefile` — and each fix has been proven insufficient by the following pass:

- WR-19 → WR-27 → CR-10 → **CR-17** (the predicate's own regression test is vacuous) + **CR-16** (the predicate is not enforced where the evidence is validated)
- CR-04 → CR-11 → **CR-16** (the exhaustive control skips 27.5 % of regions by construction)
- WR-28 → CR-13 → **WR-45** (the anti-recurrence guard is bypassable)

**This pass did find a genuinely new class**, which is why I do not think the loop has simply run out of signal: **CR-15** is not a variation on a known finding type. Every prior gate finding was "the classification logic is too permissive." CR-15 is "the comparison being classified cannot observe the defect at all" — a real/dummy diff over a shared renderer. It subsumes and explains the previous three: three iterations of BLOCKER work went into narrowing which divergences are accepted, when the regression under discussion produces **no divergence**. Two further new classes appeared: build-time vs validate-time enforcement asymmetry (CR-16), and a confirmation ceremony for a write that is not wired (CR-19).

**Recommendation to the orchestrator:**

1. **Stop running unattended fixer iterations on the gate.** CR-15 needs a design decision a fixer cannot make: whether the real-vs-dummy comparison can serve as Phase 4 acceptance evidence at all, given D-12's explicit prohibition on substituting unit tests. Four automated attempts have each produced a locally-correct fix to a mechanism that does not do what the phase needs.
2. **Fix CR-18 first, on its own.** It is a self-contained ~15-line change plus a table test, it is the only finding with a security impact, and it is independent of everything else.
3. **Then CR-17 + CR-16 together** (one gate, one commit) and the product warnings WR-49/WR-51, WR-47/WR-48.
4. **Route CR-19 back to the human** — "delete announces a backup and writes nothing on a tab with no demo banner" is either a scope decision or a P0, and the reviewer cannot tell which from the code.
5. **Do not accept another `04-REVIEW-FIX.md` that reports red-before-fix evidence without the reverted-source diff.** CR-17 shows a claimed red-before-fix that does not reproduce; that specific verification claim is now the loop's weakest link.

---

_Reviewed: 2026-08-25T09:40:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
_Iteration: 5 (adversarial re-review of 04-REVIEW-FIX.md iteration 4 + full-state sweep)_
