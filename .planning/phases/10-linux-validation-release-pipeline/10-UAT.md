---
status: testing
phase: 10-linux-validation-release-pipeline
source: [10-VERIFICATION.md]
started: 2026-09-05T04:15:00Z
updated: 2026-09-05T16:00:00Z
---

## Current Test

number: 1
name: Run the Bazzite manual UAT checklist on real Bazzite hardware
expected: |
  Each of the 5 rows in bazzite-uat-checklist.md (SELinux spot-check,
  /home->/var/home symlink includeIf resolution, real-terminal TUI
  rendering, wl-clipboard presence, ssh-add no-agent degradation)
  resolves to a real PASS/FAIL finding, not a placeholder. Update
  PLATFORM-NOTES.md's 5 "Pending manual UAT" rows to Verified / Accepted
  limitation accordingly.
awaiting: user response

## Tests

### 1. Run the Bazzite manual UAT checklist on real Bazzite hardware
expected: Each of the 5 rows (SELinux spot-check, /home->/var/home symlink
  includeIf resolution, real-terminal TUI rendering, wl-clipboard presence,
  ssh-add no-agent degradation) resolves to a real PASS/FAIL finding, not a
  placeholder. Requires physical/real Bazzite hardware with SELinux
  enforcement, a real Wayland desktop session, and real terminal emulators
  (Ptyxis/Konsole), none of which exist in any automated CI/agent sandbox.
  This is the deliberately-human-only residue of D-01(ii)/D-03's design,
  run once per release, not a one-time phase gate.
result: [pending]

### 2. Create the Homebrew tap repo + PAT, then cut a real release tag
status: RESOLVED — no longer a blocking pending item (2026-09-05 gap-closure,
  10-07-PLAN.md, D-18 in 10-CONTEXT.md's addendum).

Previously this item asked for `castocolina/homebrew-tap` + a PAT to be
created BEFORE a real release tag could be cut, because `.goreleaser.yaml`'s
`brews:` stanza (D-13) would otherwise fail to publish without them. That is
no longer true: the `release`/`release-nightly` Make targets now pass
`--skip=homebrew` to goreleaser whenever `HOMEBREW_TAP_GITHUB_TOKEN` is
absent (verified by a real `goreleaser release` invocation against a scratch
tag in this repo — see `e2e/release_homebrew_gate_e2e_test.go`,
`TestReleaseHomebrewGate_SkippedNeverEntersHomebrewPipe`), so **a real `v*`
tag push now succeeds end-to-end without the tap repo/PAT existing**. The
`brews:` stanza itself is untouched (D-13's original decision is preserved,
not deleted) and will activate automatically, with zero code changes, the
moment `castocolina/homebrew-tap` exists and `HOMEBREW_TAP_GITHUB_TOKEN` is
added as a real repo secret.

expected (revised): Creating the tap repo + PAT is now an OPTIONAL, deferred
  enhancement the user can complete at any future point to activate Homebrew
  distribution — not a precondition for cutting a real release. When the
  user does complete it, pushing a `v*` tag should produce a live GitHub
  Release (4 tar.gz archives + 1 checksums.txt, both attested via
  `actions/attest-build-provenance`) AND a `Formula/gitid.rb` commit on
  `castocolina/homebrew-tap`, after which `brew install
  castocolina/homebrew-tap/gitid` installs a working binary reporting the
  correct stamped version. This remains genuinely untestable without the
  user's own GitHub account (repo creation + PAT minting) — confirmed live
  again during this gap-closure: `curl -sI https://github.com/castocolina/homebrew-tap`
  still returns 404. The DIFFERENCE from the original item is that this is
  now a "do this whenever you want Homebrew" note, not a release blocker.
result: [deferred-by-design, not blocking]

### 3. (NEW, 2026-09-05 gap-closure) Nightly release automation
expected: `.github/workflows/nightly.yml` runs on a daily schedule (and
  `workflow_dispatch`), invoking `make release-nightly`, which creates a
  fresh `v0.0.0-nightly.<timestamp>.<sha>` tag and runs goreleaser's
  ordinary `release` command against it (GoReleaser's native `--nightly`
  mode is GoReleaser-Pro-only — empirically verified absent from the pinned
  OSS v2.18.0 binary; see D-19 in 10-CONTEXT.md's addendum). This has NOT
  yet run for real on GitHub Actions (no push to `main` has happened since
  this workflow was added in this same session) — the structural/unit tests
  (`cmd/gitid/nightly_yml_test.go`) and a real local `goreleaser release`
  dry run (this file's homebrew-gate e2e tests) prove the mechanism works,
  but the FIRST live scheduled/dispatched run on GitHub's infrastructure is
  still outstanding. This is expected to self-resolve on the next push to
  `main` or the next scheduled cron tick — not a human-only blocker, just
  not yet observed live.
result: [pending — will self-resolve on first live push/schedule tick]

### 4. (NEW, 2026-09-05 gap-closure) install.sh channel/menu real-world use
expected: A user actually running `curl | sh` against the real
  `raw.githubusercontent.com` URL, or running the script directly with a
  real interactive terminal, exercises `GITID_CHANNEL=nightly` resolution
  and the numbered interactive menu against REAL GitHub API responses (not
  the e2e fixture server). Covered locally by
  `e2e/release_channel_e2e_test.go` (fixture-server + real-pty tests, all
  passing) but not yet observed against the real github.com/api.github.com
  endpoints.
result: [pending — not human-only, just not yet observed against the real
  GitHub endpoints; will self-resolve on first real-world use of the
  installer's new flags]

## Summary

total: 4
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0
resolved_as_non_blocking: 1

## Gaps

None of the 3 remaining pending items are code defects, stubs, or broken
wiring. Item 1 (Bazzite manual UAT) is the original, deliberately human-only
residue, unchanged by this gap-closure and out of scope per the task
instructions. Items 3 and 4 are "first live observation" items: the
underlying mechanisms are real and tested (structurally, plus via real local
goreleaser/install.sh runs against fixture servers and a real pty), and are
expected to self-resolve the next time `main` receives a push, a cron tick
fires, or a real user runs the installer — they are not human-only actions
requiring special credentials the way item 1 (and, previously, item 2) did.
Item 2 (Homebrew tap) is resolved from a blocking standpoint: the release
pipeline no longer requires the tap repo/PAT to exist for a real release to
succeed (D-18); creating the tap repo remains available to the user as an
optional future enhancement, not a precondition.

All automated verification for this gap-closure (`make test` -race,
`make lint`, `make test-e2e`, plus the new goreleaser-homebrew-gate and
install.sh-channel/TTY e2e tests) passed clean — see 10-VERIFICATION.md for
the updated report.
