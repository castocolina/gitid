---
phase: 10-linux-validation-release-pipeline
reviewed: 2026-09-04T00:00:00Z
depth: deep
files_reviewed: 26
files_reviewed_list:
  - .github/workflows/ci.yml
  - .github/workflows/release.yml
  - .gitignore
  - .goreleaser.yaml
  - Makefile
  - PLATFORM-NOTES.md
  - README.md
  - cmd/gitid/ci_fedora_test.go
  - cmd/gitid/goreleaser_config_test.go
  - cmd/gitid/identity_test.go
  - cmd/gitid/main.go
  - cmd/gitid/main_test.go
  - cmd/gitid/release_plumbing_test.go
  - cmd/gitid/release_yml_test.go
  - cmd/gitid/version_cmd.go
  - cmd/gitid/version_cmd_test.go
  - e2e/harness_test.go
  - e2e/release_e2e_test.go
  - internal/platform/version_test.go
  - internal/tuikit/app.go
  - internal/tuikit/app_test.go
  - internal/version/version.go
  - internal/version/version_test.go
  - scripts/install.sh
  - .golangci.yml
  - .planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md
findings:
  critical: 0
  warning: 2
  info: 3
  total: 5
status: issues_found
---

# Phase 10: Code Review Report — Round 3 (fresh, independent reviewer)

**Reviewed:** 2026-09-04
**Depth:** deep
**Files Reviewed:** 26 (everything changed since 3ef374bf47c3c9b3b0db5580a7a5fbbb4aab3c4e)
**Status:** issues_found — no Critical/Blocker findings, but the phase is not yet a clean 0-finding pass

## Summary

I reviewed this independently from scratch: read every changed file, rebuilt the
module, ran `go build ./...`, `go vet ./...`, `go test -race` (all non-e2e
packages), `go test -tags e2e ./e2e/...` for every `TestRelease_*` /
`TestInstallScript_*` case, `make lint`, and `make fmt` — all clean, all green,
no diffs produced by `make fmt`. I re-ran every regression-guard test named in
round 1/round 2/round 3's write-ups directly (`TestReleaseWorkflow*`,
`TestCIWorkflow*`, `TestFedora*`, `TestGoreleaserConfig*`,
`TestInstallScript_VersionWithRegexMetacharactersMatchesChecksumExactly`, etc.)
and confirmed they pass and are not vacuous (each has a real negative-control
case reproducing the pre-fix bug).

**IN-01 re-verification (round 2's install.sh PATH-glob claim):** I
independently reproduced this with both `bash` and `dash` on this host, using
a directory literally containing `*`/`?` on both sides of the `case` match
(both as a value coincidentally present elsewhere on `$PATH` and as the
`INSTALL_DIR` itself). In every combination the quoted `"${INSTALL_DIR}"`
inside the `case` pattern matched **literally**, never as a glob — confirming
round 3's claim. This was correctly a false positive; no code change was
needed, and the comment now in `scripts/install.sh:160-169` accurately
documents why. **I concur: IN-01 stays closed as a false positive.**

**Round 2's WR-01 fix (unpinned `gosec@latest`) is only half-applied.** It
correctly removed the standalone `gosec` install from `setup-env-release`
(release.yml's job — the one round 2 flagged), but the *identical* unpinned
`go install github.com/securego/gosec/v2/cmd/gosec@latest` line is still
present, completely untouched, in the full `setup-env` target — which is what
`ci.yml`'s `check` job (3-runner matrix, every PR + every push to main) and
`fedora` job (push-to-main + tags) both invoke. I grepped the entire
`.github/workflows/`, `Makefile`, and `.golangci.yml` for any standalone
invocation of a bare `gosec` command and found none — the binary is dead
weight in `setup-env` for exactly the same reason round 2 called it dead
weight in `setup-env-release`. See WR-01 (round 3) below.

I also found one new, concrete, independently-reproduced bug: **README.md's
documented `curl | sh` one-liner for `GITID_VERSION`/`GITID_INSTALL_DIR` does
not work as written** — the env-var-prefix-before-a-pipe shell idiom only
scopes those variables to the `curl` side of the pipe, never to the `sh` that
actually runs the script. I reproduced this directly (bash, dash, sh) with the
exact documented command shape. See WR-02 (round 3) below.

Two Info-level stale-documentation findings round out this pass — both
consequences of this phase's own migration to goreleaser/`internal/version`
not being fully propagated to every comment that described the old shape.

**Verdict:** Build/tests/lint are genuinely clean, and the security-relevant
regression guards this phase's prior two rounds added are real and hold up
under my own reproduction. This is **not yet a 0-finding pass**, however: WR-01
(round 3) is a legitimate completeness gap in a previously "fixed" security
finding, and WR-02 (round 3) is a real, user-facing documentation bug for a
security-relevant install feature (version pinning). Neither is Critical —
both are inert until someone acts on the affected instruction/target — but
both should be fixed before calling this phase done.

## Warnings

### WR-01 (round 3): Round-2's unpinned-`gosec@latest` fix was applied to only one of two targets that need it

**File:** `Makefile:198-220` (the `setup-env` target, specifically line 204)

**Issue:** Round 2 flagged `setup-env-release`'s standalone
`go install github.com/securego/gosec/v2/cmd/gosec@latest` as dead weight
that widens the supply-chain surface for zero linting benefit (golangci-lint's
own embedded `gosec` linter, enabled in `.golangci.yml`, is the actual
coverage `make lint` uses). Round 3's fix (commit `1d0d67b`) removed it —
but only from `setup-env-release`. The full `setup-env` target still has the
byte-identical line:

```makefile
@echo "==> Installing gosec (standalone binary)"
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

`setup-env` is not just a local-dev convenience — `.github/workflows/ci.yml`
calls it directly in both the `check` job (`ubuntu-latest`, `macos-15-intel`,
`macos-15`; runs on every PR and every push to main) and the `fedora`
container job (push-to-main + release tags). I verified with
`grep -rn "\bgosec\b"` across `.github/workflows/`, `Makefile`, and
`.golangci.yml` that no target anywhere invokes a bare `gosec` command — the
binary this line installs is never executed by any gate in this repo, in
either target. The Makefile's own doc comment for the binary (line 179-180,
"installed separately for direct invocation if needed") only weakly
rationalizes this for a human's local ad-hoc `gosec ./...` run; it does not
justify paying the identical unpinned `@latest` dependency-resolution cost on
every automated `check`/`fedora` CI run, which round 2's own reasoning
("widening its supply-chain surface via an unpinned dependency resolution for
zero linting benefit") applies to just as much as it did to
`setup-env-release`.

No regression test guards either target's gosec-install behavior (I checked
`release_plumbing_test.go`, `goreleaser_config_test.go`, `ci_fedora_test.go`,
`release_yml_test.go` — none assert on `setup-env`'s or `setup-env-release`'s
own install-step contents), so nothing would catch this drifting back either
way in the future.

**Fix:** Either remove the standalone gosec install from `setup-env` too (for
parity with `setup-env-release`'s now-corrected reasoning), or, if the
"direct invocation for local triage" use case is genuinely wanted, pin it to
an explicit version the same way `GOLANGCI_LINT_VERSION`/`GORELEASER_VERSION`/
`FREEZE_VERSION` are pinned elsewhere in this same file, and add a one-line
regression test (mirroring the `TestReleaseWorkflowStepSequence` style already
used in this phase) asserting `setup-env`'s gosec line is pinned, not
`@latest`.

### WR-02 (round 3): README's documented `GITID_VERSION`/`GITID_INSTALL_DIR` one-liner silently does not work

**File:** `README.md:98-101`

**Issue:** The documented example is:

```sh
GITID_VERSION=v1.0.0 GITID_INSTALL_DIR="$HOME/bin" \
  curl -fsSL https://raw.githubusercontent.com/castocolina/gitid/main/scripts/install.sh | sh
```

This is a single shell pipeline: `<env-prefixed curl> | sh`. POSIX shell
variable-assignment prefixes (`VAR=val cmd`) scope the variable to *only* the
one simple command they immediately precede — here, `curl`. They are **not**
inherited by the other side of a pipe. I reproduced this directly against the
exact documented shape (both with a local HTTP fixture and with a trivial
`cat file | sh` reduction) on bash, dash, and `/bin/sh`:

```sh
$ GITID_VERSION=v1.0.0 GITID_INSTALL_DIR="$HOME/bin" \
  curl -fsSL http://localhost:8768/fake_installsh.sh | sh
GITID_VERSION seen inside script: UNSET
GITID_INSTALL_DIR seen inside script: UNSET
```

Anyone following this exact, copy-pasteable README instruction to pin a
specific vetted release (a security-relevant practice — the whole point of
`GITID_VERSION` per D-14) or to install to a custom directory gets neither:
`install.sh` silently falls back to resolving `/releases/latest` and
installing to `~/.local/bin`, with no error and no indication anything
diverged from what was requested. This is not a bug in `scripts/install.sh`
itself (which correctly reads `GITID_VERSION`/`GITID_INSTALL_DIR` from its own
environment — the e2e suite drives it that way, via `cmd.Env`, and that path
is correctly tested) — it is purely a broken *documented usage example*.

**Fix:** Either export the variables into the current shell first, or fold
the whole thing into one process on the `sh` side, e.g.:

```sh
export GITID_VERSION=v1.0.0 GITID_INSTALL_DIR="$HOME/bin"
curl -fsSL https://raw.githubusercontent.com/castocolina/gitid/main/scripts/install.sh | sh
```

or

```sh
curl -fsSL https://raw.githubusercontent.com/castocolina/gitid/main/scripts/install.sh \
  | GITID_VERSION=v1.0.0 GITID_INSTALL_DIR="$HOME/bin" sh
```

(the second form is the one that actually keeps the assignment-prefix
scoped to the command that needs it, `sh`, rather than `curl`). Whichever
form is chosen, add it to the manual-verification checklist so this doesn't
silently regress in a future README edit.

## Info

### IN-01 (round 3): Stale Makefile header still documents the retired `checksums` target

**File:** `Makefile:22-24`

**Issue:** The top-of-file target-list comment still reads:

```
#   checksums      Cross-build then write bin/checksums.txt with one SHA-256 line per
#                  published asset, hashed from inside bin/ so each line names the bare
#                  asset (BUILD-03, D-03).
```

The `checksums:` target itself was fully removed by commit `e9a49b3`
(feat(10-04)) in favor of `release`/`release-snapshot` (goreleaser-driven);
its own inline doc comment directly above the old recipe was correctly
replaced. This top-of-file summary entry, however, was left behind — it is
not in `.PHONY` either. A reader trusting the header as the target reference
(which is exactly what it's for, per the file's own opening line) would look
for a target that no longer exists.

**Fix:** Delete lines 22-24 (or replace with the `release`/`release-snapshot`
entries that already exist further down at lines ~25-34 — check for
duplication).

### IN-02 (round 3): Stale "installs gosec" wording left in two places after round-3's WR-01 fix

**File:** `Makefile:10-15`, `.github/workflows/release.yml:53`

**Issue:** Round 3's own fix (commit `1d0d67b`) rewrote `setup-env-release`'s
recipe and its immediately-preceding doc block (Makefile lines 223-237) to
correctly state it "Installs ONLY golangci-lint ... and goreleaser — the two
tools ... explicitly SKIPPING ... gosec". Two other places describing the
same target were not updated to match:

- `Makefile:10-15` (the top-of-file target list) still says: "installs ONLY
  golangci-lint, gosec, and goreleaser — the three tools...".
- `.github/workflows/release.yml:53` step name still reads: `make
  setup-env-release (golangci-lint + gosec + goreleaser only)`.

Both now contradict the target's actual, corrected behavior and the more
detailed (correct) doc comment sitting right next to the recipe. This is
purely cosmetic (no functional impact — `setup-env-release` really does skip
the standalone gosec install, as verified by `grep`), but it's exactly the
kind of self-contradictory security-relevant comment that erodes trust in
this codebase's otherwise very thorough self-documentation discipline.

**Fix:** Update both to drop "gosec" from the tool list (matching the
corrected doc block at Makefile:223-237).

### IN-03 (round 3): No regression test asserts `gosec` pinning/absence in either `setup-env` target

**File:** `cmd/gitid/release_plumbing_test.go`, `cmd/gitid/goreleaser_config_test.go`

**Issue:** This phase added dedicated regression tests for essentially every
other WR/C-finding fix it made (`TestReleaseWorkflowStepSequence` for C-7,
`TestGoreleaserConfigDistIsNeverBin` for C-4, etc.), but no test exists
asserting `setup-env`'s or `setup-env-release`'s own tool-install lines (e.g.
"no unpinned `gosec@latest` line", or "no standalone `gosec` install at all").
This is why WR-01 (round 3) above was able to regress silently in `setup-env`
while `setup-env-release` was fixed.

**Fix:** Add a `TestSetupEnvTargetsNeverInstallUnpinnedGosec`-style test
(mirroring the `readRepoFile(t, makefilePath(t))` pattern already used in
this same package) once WR-01 (round 3) is resolved, asserting the chosen
final state (removed entirely, or pinned to an explicit version) for both
targets.

---

_Reviewed: 2026-09-04_
_Reviewer: Claude (gsd-code-reviewer), round 3_
_Depth: deep_
