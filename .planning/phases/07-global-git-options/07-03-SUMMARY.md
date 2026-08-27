---
phase: 07-global-git-options
plan: 03
type: summary
wave: 3
---

# 07-03 Summary — the complete twelve-row policy table, honest states/provenance/bundles, and the fixture parity pin

## What was built

Four commits (Tasks 1, 2, and Task 3 split into an opening data-corrections commit plus the rest), all cross-AI-first with two orchestrator crash-recovery episodes and one substantial orchestrator hand-finish (see Deviations).

### Task 1 — complete policy table, version-gated write value, composer `[user]` section, deletion (`e6e2d0e`)

`internal/globalgit/policy.go` grew from Wave 1's single live row to the full 12-row D-08 pinned table, in the frozen display order, each row carrying a member-key SET (scalar rows one key, line-endings two, alias eight, color four, fallback-author zero) plus a frozen CLI token (R-5). Alias/color values are verbatim from `recipes/gitconfig.recipe`'s `~/.gitconfig_default` example. `internal/globalgit/version.go`'s `WriteValueFor` is the one place deciding `zdiff3` vs `diff3` for `merge.conflictstyle` (the only hard-gated row): informational gates never change the written value (old git silently ignores an unknown key), the hard gate substitutes the fallback when not met, and an unreadable version is treated as below-gate for the hard gate specifically (the conservative direction — `diff3` is accepted by every git that accepts `zdiff3`). `EnsureGlobalGit`'s section order grew a `[user]` section for `useConfigOnly` only — never the fallback author's name/email, which stay in Wave 2's separate block. `gitconfig.ScanConflicts`, `BaselineKeySet`, and the orphaned `Conflict` type were deleted per R-7, verified by a full `go build`/`go vet`/`go test ./...` pass in the same commit, not by `rg` alone. A pager-preservation test proves `core.pager` survives a full 12-row apply.

### Task 2 — four-state classifier, bundle aggregate, and the one D-10 tally (`b1dd165`)

`internal/globalgit/classify.go` extended to the full four-state vocabulary (needs-action, already-set, set-but-differs, not-applicable with a reason enum), keeping the value-before-source branch order. D-02's posture: a set-but-differs row is informational and never selectable — gitid's block sits at the floor, so a write into a key the user set later is provably a no-op. `internal/globalgit/bundle.go` re-derives the D-09 aggregate (set/differs counts, per-key differs list) from the two probes already taken, replacing the deleted `ScanConflicts` with strictly better evidence. Composing a bundle row's selection emits every member key regardless of collisions, PROVEN with a real lifecycle test (`TestRunGlobalGitApply_BundleCollisionUserValueWins`): seeds `alias.co=pull` in the user's own `~/.gitconfig`, applies the alias bundle, reads `alias.co` back with real `git config --show-origin --get`, and asserts git names the user's own file and the user's own value. The single D-10 tally predicate (`gitNeedsAttention`) is read by both the status line and the apply ceremony's count — no second counting loop. `GlobalGitOptionView.Selectable()` is the one predicate for the toggle key, checkbox glyph, and click target.

### Task 3, opening corrections — `merge.conflictstyle` fix + `user.useConfigOnly` row (`1c910d5`)

The `merge.conflictstyle` recommendation was stale at `diff3` in four places — `internal/tuikit/design.go`'s fixture row, `GlobalGitBaselineStripText`, `GlobalGitFullManagedBlockText`, and `.planning/design/mockup-src/src/data/recipeFixtures.ts`'s `globalGitDefaults` — all corrected to `zdiff3` in this one commit, per D-08. The `user.useConfigOnly` row was added to both fixtures at the pinned position (immediately after the fallback-author row), unchecked by default, with its `[user]` line added to both managed-block texts.

### Task 3, the rest — parity pin, new copy, gate extension, no-colour legibility (`bc23a2e`)

`TestGlobalGitFixturePolicyParity` (in `cmd/gitid/wiring_test.go`, mirroring the SSH side's own parity test) walks `internal/globalgit.Policy` and `internal/tuikit.GlobalGitOptions` together, asserting row identity, order, and recommended value agree — 12/12 rows pass. New copy: the case-sensitivity caveat (appended to `core.ignorecase`'s detail explanation), `GlobalGitConflictStyleGateNote` (the STATIC half of the hard-gate explanation, deliberately separate from the pre-existing dynamic `VersionNote` line so it stays freezable — a new `GateNotMet bool` field carries the distinction), the D-07 cross-warning as two static constants (rendered on the `user.useConfigOnly` row's own detail pane when selected with the fallback pair half-set), the guessed-name warning (rendered on the fallback-author row's own detail pane, independent of `useConfigOnly`'s selection), and `GlobalGitResultTail` (the apply ceremony's result message now substitutes real selected/pending counts, mirroring `globalssh.go`'s `chosen`/`pending` shape, with only the static tail frozen). `gate-copy-freeze`'s search path now includes `internal/globalgit`, every new static string is registered, and three new dynamic exclusions (git-version line, bundle aggregate, applied/selected counts) are proven not-frozen. A no-colour test confirms all four states remain distinguishable by glyph and word with ANSI stripped. `TestRunGlobalGitApply_LeavesFallbackAuthorBlockUntouched` proves the baseline apply ceremony never touches the separate fallback-author block, converting `GlobalGitResultTail`'s claim into a checked invariant.

## Deviations

**Two cross-AI runtime failures on Task 3's "new copy" portion, both recovered by the orchestrator — the second recovery done as a substantial direct hand-implementation rather than a third relaunch:**

1. **First attempt** stalled — the agent repeated the exact same log lines (checking `GlobalGit.tsx` for a hardcoded row count) for 20+ minutes with zero `out.log` growth. The orchestrator killed it, confirmed the salvageable uncommitted work (the corrections + `useConfigOnly` row addition) built and tested cleanly, and committed it as `1c910d5`.

2. **Second attempt** (a scoped continuation) crashed on a banned `/tmp` scratch-write while investigating whether `git config --show-origin` reports line numbers (relevant to the "five provenance labels... mirroring Phase 6's own registered set... with its line" phrasing in the plan). It had made zero implementation progress — pure exploration. The orchestrator answered the underlying question directly (`git config --show-origin --get` returns only `file:<path>` — no line number, unlike SSH's parsed-config line tracking) and, given two consecutive failures on this exact remaining scope, implemented the rest of Task 3 directly rather than risking a third relaunch: the parity test, all new copy, the `GateNotMet` field and its wiring-site computation, the gate extension with its three proven exclusions, the no-colour test, and the baseline-untouched lifecycle test. This is documented as a deviation from the plan's cross-AI-first default, made deliberately after the evidence (two failures, zero progress on the specific remaining piece) crossed the threshold this session has used throughout the phase.

**One self-caught bug during the gate-copy-freeze extension, fixed on the spot and left in the commit message as evidence the technique works**: the first draft of the third dynamic exclusion (`counts_dyn="baseline options applied to"`) used a single literal string assignment — the exact D-13 self-matching trap the plan's `<authority>` block warns about, where the check's OWN literal value in the Makefile makes the `grep -qF` against the Makefile find a match (itself) and report a false FAIL. Caught immediately by running the gate, fixed with the same two-part runtime-concatenation technique already used for the other three exclusions (`counts_dyn="baseline options applied"; counts_dyn="$counts_dyn to"`).

## The three copy-freeze exclusion proofs (fail-then-revert, as the plan requires)

Each dynamic exclusion was deliberately broken by temporarily adding its literal prefix to the frozen list, running `make gate-copy-freeze`, observing the real failure, then reverting:

**1. Dynamic git-version line** — added `'Your git:'` to the frozen list:
```
    ok   D-13 exclusion (dynamic version prefix not frozen)
    FAIL  dynamic git-version prefix must stay out of the frozen list (07-03 D-13 precedent)
make: *** [gate-copy-freeze] Error 1
```

**2. Dynamic bundle aggregate count** — added `'of set'` to the frozen list:
```
    ok   07-03 exclusion (dynamic git-version prefix not frozen)
    FAIL  dynamic bundle aggregate count must stay out of the frozen list (D-09)
make: *** [gate-copy-freeze] Error 1
```

**3. Dynamic applied/selected counts** — added `'baseline options applied to'` to the frozen list:
```
    ok   D-09 exclusion (dynamic bundle aggregate count not frozen)
    FAIL  dynamic applied/selected counts must stay out of the frozen list (result message)
make: *** [gate-copy-freeze] Error 1
```

After each proof, the Makefile was reverted (`git diff --stat Makefile` confirmed no residual change), and the gate was re-run clean:
```
    ok   D-13 exclusion (dynamic version prefix not frozen)
    ok   07-03 exclusion (dynamic git-version prefix not frozen)
    ok   D-09 exclusion (dynamic bundle aggregate count not frozen)
    ok   result-message exclusion (dynamic applied/selected counts not frozen)
```

## The bundle-collision D-06/D-09 proof (real `git` binary, quoted output — from Task 2's commit)

`TestRunGlobalGitApply_BundleCollisionUserValueWins` seeds `alias.co = pull` in a sandbox `~/.gitconfig`, applies the alias bundle row (which composes all 8 canonical aliases into the baseline block, including `alias.co = checkout`), then reads the effective value back:

```
$ git config --show-origin --get alias.co
file:/<sandbox-home>/.gitconfig	pull
```

The origin names the user's own `~/.gitconfig` (not the gitid baseline file), and the value is the user's own `pull` — proving floor placement plus git's last-wins rule really does let the user's own collision win, exactly as the per-key "yours differs — yours wins" detail-pane note claims.

## Exit Battery Results (independently run by the orchestrator, real output)

```
$ go build ./...
(clean)

$ go vet ./...
(clean)

$ TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...
ok  	github.com/castocolina/gitid/cmd/gitid	46.9s
ok  	github.com/castocolina/gitid/cmd/gitid-dummy	1.6s
ok  	github.com/castocolina/gitid/internal/adopter	2.9s
ok  	github.com/castocolina/gitid/internal/clipboard	4.7s
ok  	github.com/castocolina/gitid/internal/deps	3.3s
ok  	github.com/castocolina/gitid/internal/doctor	5.7s
ok  	github.com/castocolina/gitid/internal/doctor/checks	6.2s
ok  	github.com/castocolina/gitid/internal/dummytui	2.7s
ok  	github.com/castocolina/gitid/internal/filewriter	2.3s
ok  	github.com/castocolina/gitid/internal/gitconfig	8.1s
ok  	github.com/castocolina/gitid/internal/globalgit	5.8s
ok  	github.com/castocolina/gitid/internal/globalssh	6.9s
ok  	github.com/castocolina/gitid/internal/identity	6.9s
ok  	github.com/castocolina/gitid/internal/keygen	24.4s
ok  	github.com/castocolina/gitid/internal/platform	5.8s
?   	github.com/castocolina/gitid/internal/screenshot	[no test files]
ok  	github.com/castocolina/gitid/internal/sshconfig	11.3s
ok  	github.com/castocolina/gitid/internal/tester	8.5s
ok  	github.com/castocolina/gitid/internal/tuikit	23.5s
ok  	github.com/castocolina/gitid/internal/upload	3.8s
ok  	github.com/castocolina/gitid/internal/uploader	4.9s

$ golangci-lint cache clean && make lint
0 issues (both plain and -tags screenshot)

$ make gate-copy-freeze
(all frozen strings present, all three new dynamic exclusions confirmed not-frozen)

$ rg -n 'ScanConflicts|BaselineKeySet|Conflict\b' internal/ cmd/ e2e/
internal/globalgit/classify_test.go:193: (comment referencing "the retired ScanConflicts" — not a symbol reference)
internal/globalgit/classify.go:96: (comment referencing "the retired ScanConflicts" — not a symbol reference)
(no actual symbol usage remains)

$ go test ./internal/dummytui/ -run TestNoBackendAllowlist -v
--- PASS: TestNoBackendAllowlist (0.33s)

$ pnpm --dir .planning/design/mockup-src typecheck
(clean)

$ pnpm --dir .planning/design/mockup-src build
✓ built in 496ms (verify-routes: OK)
```

## Confirmation: parity, deletion, and gate integrity all hold

- `TestGlobalGitFixturePolicyParity`: 12/12 rows agree between `internal/globalgit.Policy` and `internal/tuikit.GlobalGitOptions` on identity, order, and recommended value.
- The `ScanConflicts`/`BaselineKeySet`/`Conflict` deletion left zero symbol references anywhere the Go toolchain compiles (the archived POC is untouched and invisible to `go build ./...`, confirmed at planning time and unaffected here).
- `make gate-copy-freeze` search path now covers `internal/globalgit`; every string this plan introduces is registered; every machine-specific line (git version, bundle counts, applied/selected counts) is genuinely excluded, each proven by a real fail-then-revert cycle recorded above.
