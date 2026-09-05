---
status: testing
phase: 10-linux-validation-release-pipeline
source: [10-VERIFICATION.md]
started: 2026-09-05T04:15:00Z
updated: 2026-09-05T04:15:00Z
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
  (Ptyxis/Konsole) — none of which exist in any automated CI/agent sandbox.
  This is the deliberately-human-only residue of D-01(ii)/D-03's design,
  run once per release, not a one-time phase gate.
result: [pending]

### 2. Create the Homebrew tap repo + PAT, then cut a real release tag
expected: Create the castocolina/homebrew-tap GitHub repo, mint a
  fine-grained PAT scoped to it, add it as the HOMEBREW_TAP_GITHUB_TOKEN
  secret on the gitid repo, then push a real v* tag and observe release.yml
  run to completion — a live GitHub Release appears with 4 tar.gz archives
  + 1 checksums.txt (both attested via actions/attest-build-provenance),
  castocolina/homebrew-tap gets a Formula/gitid.rb commit, and
  `brew install castocolina/homebrew-tap/gitid` installs a working binary
  reporting the correct stamped version. Repo creation and PAT minting both
  require the user's own GitHub account (confirmed live during verification:
  `curl -sI https://github.com/castocolina/homebrew-tap` returned 404 — the
  tap repo does not exist yet). Explicitly named as a user_setup item in
  10-04-SUMMARY.md's own frontmatter.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps

None — both pending items are genuine human_needed actions (real hardware,
real GitHub account credentials), not code defects, stubs, or broken
wiring. All automated verification (build, test -race, lint, full e2e
suite, 4-round independent code review, a real `make release-snapshot`
dry-run reproducing the exact D-07 artifact shape) passed clean. See
10-VERIFICATION.md for the full report.
