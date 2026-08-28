# 08-05 SUMMARY — Coherence-family new checks: shadowed options, directive-above-block, author resolution

## Outcome

The cross-AI executor crashed almost immediately: it hit a permission
auto-reject on an external directory path after a typo'd `cd` command
(`gitit-phase08-wave5` instead of `gitid-phase08-wave5`), before writing any
code — zero commits, still in the investigation phase. This plan was
hand-implemented directly instead of re-dispatching, per this phase's
established Wave 1/2 hand-recovery pattern.

All three tasks are complete: two new report-only Coherence/SSH checks
(shadowed global option, hand-written directive above gitid's managed
block), one new report-only Coherence/Git check (author resolution
mismatch), and the resolved Pitfall 6 Family-conflict question. All three
new checks reuse an existing Phase 6/7 probe unchanged — no shadow-detection
or author-resolution logic was reimplemented.

## Task 1 — Shadowed-option and directive-above-block checks

`checkShadowedGlobalOptions` (internal/doctor/checks/coherence.go) calls
`internal/globalssh.Verify` — the exact same D-04 post-write probe
`cmd/gitid/lifecycle.go`'s `runGlobalSSHApply` already runs after every
Global SSH apply — through a new `doctor.Deps.GlobalSSHShadowCheck` closure.
The closure (`cmd/gitid/wiring.go`) locates gitid's global `Host *` managed
block (in-file or the Include'd `config.d/gitid.config`, auto-detected via
`sshconfig.Adopt(AdoptSentinelBearing, ...)` — the same primitive
`(*realBackend).storage()` calls, simplified to a read-only lookup), reads
which non-per-alias policy keys are actually written into it (a key gitid
never applied is "not configured", never "shadowed"), and calls `Verify`
with those keys only.

**Deliberate, documented divergence from the plan's literal text:**
`globalssh.Verify`'s own doc comment records it never sets
`ShadowedByFile`/`ShadowedByLine` (no access to the config graph on the
live-machine post-write path) — `cmd/gitid/lifecycle.go` already documents
this exact limitation for its own advisory wording ("the file/line branch
here was dead code"). Rather than reimplement a graph-aware naming path (a
second detector, contradicting the plan's own "reuse, don't invent" framing),
this check mirrors that existing precedent: when the source can't be named,
the SuggestedFix says so honestly ("source unnameable") instead of promising
a file:line the probe cannot provide. Verified empirically against a real
fixture (see below) that the named-source path DOES work correctly when
`ShadowedByFile` is populated.

`checkDirectiveAboveManagedBlock` reuses `deps.AllHostBlocks`
(`sshconfig.ParseAllHostBlocks`) — the same data source
`checkHandWrittenIdentitiesOnly` already depends on — rather than a second
raw scanner. **Scope interpretation** (the plan's own estimate flagged this
task "confidence: low"): every gitid-managed Host block's `Host ...` header
line is INSIDE its sentinel-delimited body
(`RenderHostBlock`/`renderGlobalBody` both emit it as the block's first
line), so "a directive above the block, inside the same stanza" cannot occur
structurally for a gitid-managed block. What this check flags instead is a
hand-written Host stanza positioned entirely before gitid's FIRST managed
block in file order — the "predates gitid's management entirely" condition
D-09 actually names. Only stanzas before the first managed block are
flagged, not every hand-written stanza preceding some later managed block,
to avoid multiplying warnings.

## Task 2 — Author-resolution check

`checkAuthorResolution` calls `internal/globalgit.VerifyAuthorResolution` —
the same D-06 probe `appendFallbackAuthorAdvisories` already runs after every
Global Git fallback-author write — through a new
`doctor.Deps.AuthorResolutionCheck` closure. The closure
(`buildAuthorResolutionCheck`, `cmd/gitid/wiring.go`) reuses
`gitconfig.ParseManagedIncludeIf` and `findGitWorkTree` (the same helper
`fallbackMatchedDir` already uses) to find a real, currently-existing
directory matching the identity's `gitdir:` pattern, and reproduces
`appendFallbackAuthorAdvisories`'s own representative-unmatched-directory
selection (`home`, falling back to `.gitconfig.d` when present) — duplicated
as a small snippet rather than extracted into a shared helper, since
`lifecycle.go` was not in this task's file list and the snippet is three
lines. `ok=false` (`MatchedNotVerifiable`, or no includeIf on record) is a
graceful no-finding state, never a false positive.

## Task 3 — Pitfall 6 resolution

Read `orphans.go`'s Class 2 (gitconfig block with no SSH partner) and
`coherence.go`'s Incomplete branch side by side: fragment-existence
detection lives ENTIRELY in Coherence (`identity.Reconstruct` marks
`Incomplete="...fragment-file..."` when the fragment is missing, and
`coherenceForAccount` reports it) — `orphans.go` has no fragment-existence
logic at all. **D-05's "Coherence / Git" label is the correct, resolved
answer; the frozen dummy fixture's `Family: "Orphans"` tag
(`internal/dummytui/data.go:392`) is a stale dummy-data artifact**, not a
claim about the real engine.

Without a guard, the frozen scenario (identity "legacy": includeIf present,
fragment missing, no SSH counterpart) trips BOTH Coherence's Incomplete
finding AND Orphans' "no SSH Host block" finding for the same identity — a
real, verified duplicate. Added a dedup guard to `orphans.go`'s Class 2 loop:
skip any gitconfig block name that also has a non-empty `Incomplete` marker
on its `identity.Account` (Coherence's Incomplete finding is the
authoritative "what's wrong" signal; Orphans' "no SSH partner" becomes noise
once the identity is already known broken). `TestMissingFragmentNoDuplicate`
(orphans_test.go) proves exactly one finding via the REAL `doctor.Run()`
path (both `CheckCoherence` and `CheckOrphans` wired), not a direct
single-check call.

## Empirical verification (manual, real fixtures)

- **Fresh-home check**: built the real binary, ran `HOME=<fresh empty
  ~/.ssh> gitid health --json` — only the pre-existing Baseline findings
  appeared; none of the three new checks alarmed. Confirms graceful
  degradation on the single most common first-run state.
- **Real shadowed/directive-above fixture**: hand-built a `~/.ssh/config`
  with a hand-written `Host handwritten.example.com` stanza BEFORE gitid's
  managed `global-ssh` block (setting `ForwardAgent no` / `HashKnownHosts
  yes`), and ran `gitid health --json` against it. Both new checks fired
  correctly: `checkDirectiveAboveManagedBlock` named the hand-written stanza;
  `checkShadowedGlobalOptions` reported `HashKnownHosts` shadowed (verified
  independently via `ssh -G`/`ssh -F <file> -G` that the check's logic
  correctly reflects whatever the live machine's `ssh -G` reports).
- **Observed, unrelated platform quirk** (not a Wave 5 defect): on this
  development machine, `ssh -G` without `-F` resolves its config path via
  the passwd-database home directory rather than a bash-prefixed `HOME=` env
  var override for a raw shell invocation. This is a pre-existing
  characteristic of `globalssh.Verify`'s already-shipped Phase 6 call
  pattern (unchanged by this plan), not something Wave 5 introduced. It does
  not affect the Go test suite: `t.Setenv("HOME", ...)` in a real `go test`
  process correctly propagates through `os.Environ()` to the `ssh`
  subprocess, and none of the new automated tests exercise this exact path
  (they use injected fakes for the shadow-detection logic itself, and the
  wiring tests either avoid the `ssh` subprocess entirely — the fresh-home
  no-keys-to-check short-circuit — or test the deterministic, non-probe
  helpers `resolveGlobalSSHTargetPath`/`appliedGlobalSSHKeys` directly).
- No `Fix` exists on any of the three new checks (all `Fix: nil`,
  report-only per the plan) — the apply-fix-then-rescan convergence check
  (Wave 4's standing lesson) does not apply to this wave. Task 3's dedup
  guard was verified via the real `doctor.Run()` path instead, the correct
  equivalent rigor for a report-only wave.

## Verification

- `go build ./...` — exit 0.
- `go vet -tags e2e ./...` — exit 0.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — 2134 passed in
  21 packages (two independent runs).
- `GOLANGCI_LINT_CACHE="$PWD/.golangci-cache" make lint` — 0 issues;
  worktree-local cache cleaned before and after.
- `make gate-visual-regression` — PASS, 58.3s.
- `make test` (includes `gate-copy-freeze`) — PASS (confirmed twice).
- `make test-e2e` — PASS: `ok github.com/castocolina/gitid/e2e 601.708s`.

The e2e/visual gates legitimately rewrote several `06-global-ssh-options`
UI-frame fixtures' header chip counts (e.g. `! 1 ✗ 2` → `! 2 ✗ 2`) — the new
checks now correctly detect real shadow/directive-above conditions those
fixtures' hermetic e2e homes already construct but nothing previously
caught (most notably `global-ssh-shadowed-receipt.txt`'s finding count
rising from 1 to 3, on a fixture whose own frame text already names
`StrictHostKeyChecking` as shadowed by `~/.ssh/config` line 2 — this is the
new check correctly surfacing a condition the fixture was already built to
exercise). These content changes were kept, in full (including their
re-written backup timestamps, per this phase's established "a mixed diff is
kept whole" convention). Pure re-run noise (backup timestamps with no
content change, and three files carrying an ephemeral `t.TempDir()` path
baked into a frozen frame) was reverted via `git checkout --`.
