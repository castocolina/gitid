# Phase 7: Global Git Options - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-07
**Phase:** 7-global-git-options
**Areas discussed:** Last-wins conflicts & block home, Alias/color bundle granularity, autocrlf/eol + platform values, GITIGNORE-01 fold-in tension, Global user.name and user.email (user-added area)
**Mode:** `--research` — five parallel `gsd-advisor-researcher` agents; codebase facts pre-gathered via codegraph and injected into prompts (per user instruction); one agent ran empirical git 2.47 probes. All recommendations accepted except the D9 pair shape (user override) and validation (user chose per-field).

---

## Last-wins conflicts & block home

| Option | Description | Selected |
|--------|-------------|----------|
| Include'd baseline file (Recommended) | POC floor-include architecture; user wins by construction (VERIFIED); recipe's own idiom; sentinel adopts global-git name | ✓ |
| In-gitconfig sentinel near top | Byte-exact frozen copy; duplicates the baseline home | |
| Appended block | VERIFIED silent override of user values | rejected |

| Option | Description | Selected |
|--------|-------------|----------|
| Informational, no fix (Recommended) | "set, differs — your choice", not selectable, excluded from N-of-M — floor-block write is provably a no-op | ✓ |
| Selectable fix + simulate-and-warn | Uniform pipeline; decorative-button risk | |

| Option | Description | Selected |
|--------|-------------|----------|
| Own early sentinel block (Recommended) | Right after floor include; includeIf fragments always win (VERIFIED); post-write matched/unmatched verification | ✓ |
| Inside the baseline file | Contractual-copy tension with D9's separation | |
| Plain git config --global | VERIFIED: lands after includeIf → hijacks every identity | rejected |

**Notes:** Settled by stack doc: detection via native git probes (`--show-origin --show-scope` from non-repo cwd, diffed vs `--file` without `--includes`).

---

## Alias/color bundle granularity

| Option | Description | Selected |
|--------|-------------|----------|
| Approve combo (Recommended) | Unit-apply canonical block + include-all with per-key "yours differs — yours wins" notes + aggregate current cell ("3 of 8 set, 1 differs") | ✓ |
| Per-key sub-selection | New nested widget; breaks byte-stability; defer to post-v1 | |

---

## autocrlf/eol + platform values

| Option | Description | Selected |
|--------|-------------|----------|
| zdiff3, version-gated (Recommended) | GitVersionAtLeast(2,35); WRITE diff3 fallback below (old git errors on unknown VALUES at merge time); amend frozen fixture diff3→zdiff3 in one commit | ✓ |
| diff3 everywhere | No fixture churn; strictly worse style for ~all 2026 macOS users | |

| Option | Description | Selected |
|--------|-------------|----------|
| Approve as proposed (Recommended) | main / ignorecase=false + APFS caveat / autocrlf=input + eol=lf / D9 unset / autoSetupRemote=true / pull.rebase=true / fetch.prune=true / 8 aliases / color auto×4 / zebra | ✓ |
| Revise rows | | |

**Notes:** Gate asymmetry pinned: unknown KEYS are silently ignored (informational gates); unknown VALUES error (zdiff3 = the only write-changing gate). ignorecase copy must state per-repo probing overrides global.

---

## GITIGNORE-01 fold-in tension

| Option | Description | Selected |
|--------|-------------|----------|
| Defer to Phase 8 fixer (Recommended) | FIX-01 finding writes BOTH key + pattern file; amend §J now; demote excludesfile out of baseline Tier-1 (latent contract break baseline.go:424) | ✓ |
| Fold in as 12th row | D9-style dedicated ceremony; largest divergence yet vs binding design | |
| Key-only in baseline | Silently broken half-state | rejected |

---

## Global user.name and user.email (user-added area)

| Option | Description | Selected |
|--------|-------------|----------|
| Amend D9 to the pair (Recommended) | Two fields, ONE checkbox, atomic ceremony | |
| Keep email-only + info row | Frozen contract intact; guessed-name junk persists | |
| Email-only + warning copy | Copy amendment without fixing behavior | |
| USER FREEFORM | "git user.email and user.name are separate config, why checkbox… we need two fields, if empty we set empty config if set we set the variables" → two independent fields, NO checkbox, empty = key left unset (never empty-string), set = written | ✓ (user override) |

| Option | Description | Selected |
|--------|-------------|----------|
| 12th advisory row (Recommended) | user.useConfigOnly=true opt-in, default unchecked; fail-loud instead of guessed junk authors; cross-warning on partial pair | ✓ |
| Fold into the D9 ceremony | Hides it from users with no fallback | |
| Not in Phase 7 | Silent junk-author path stays open | |

| Option | Description | Selected |
|--------|-------------|----------|
| Atomic pair (Recommended) | Checkbox gates both; no half-state | |
| Independent per-field apply | Each field applies alone; guessed-name warning on name-empty state | ✓ (user choice, consistent with no-checkbox override) |

**Notes:** Research verified: email-only fallback → git guesses the name from GECOS/username and commits succeed silently as `guessed-name <email>`; useConfigOnly (git 2.8+) disables guessing entirely; useConfigOnly + partial pair = hard-fatal unmatched-repo commits (hence the cross-warning).

---

## Claude's Discretion

- The N-of-M tally rule for "set, differs" (scalar + bundle, one rule, pinned in UI wave).
- Copy: APFS caveat, guessed-name warning, useConfigOnly cross-warning, "yours differs" notes, provenance labels, 11 explanations + amended D9 strings.
- Insert-after-floor filewriter primitive shape; POC sentinel adoption/migration.
- `gitid git options …` CLI shape + parity-matrix entries.
- cwd discipline + parsing for the git probes.

## Deferred Ideas

- Global gitignore surface → Phase 8 FIX-01 (substrate dormant and ready; §J amendment lands in Phase 7).
- Per-key alias/color sub-selection → post-v1.0 on real feedback.
- core.pager (`less -FRX`) → no UI row in v1.0; revisit in Phase 8.
- Selectable fix + simulate-and-warn for user-set keys → only if UAT shows confusion.
- Windows line-ending guidance → out of scope until a Windows target exists.
