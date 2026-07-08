# Phase 6: Global SSH Options - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-07
**Phase:** 6-global-ssh-options
**Areas discussed:** Effective-value detection & shadowing, Managed-block home vs create's Host *, Platform set + recommended values, Decline persistence & apply selection
**Mode:** `--research` — each area researched by a parallel `gsd-advisor-researcher` agent (comparison tables + rationale; one agent read the live substrate and verified platform behavior locally). All research recommendations were accepted.

---

## Effective-value detection & shadowing

| Option | Description | Selected |
|--------|-------------|----------|
| Three-probe diff (Recommended) | File parse + `ssh -G <probe>` + `ssh -G -F /dev/null <probe>`; classifies user-set / system-set / default | ✓ |
| File-parse only | Line numbers but blind to /etc/ssh + defaults; needs hardcoded defaults table | |
| ssh -G only | Effective value, no provenance, misses UseKeychain | |

| Option | Description | Selected |
|--------|-------------|----------|
| Simulate + re-test (Recommended) | Pre-write temp-file `ssh -G -F <temp>` simulation in fix-preview + post-write re-test; static scan only to NAME the line | ✓ |
| Pre-write static scan only | Reimplements Host matching; Match/Include false negatives | |
| Post-write re-test only | Warn-after-the-fact | |

| Option | Description | Selected |
|--------|-------------|----------|
| Dummy .invalid host (Recommended) | gitid-probe.invalid — matches only Host * + system + defaults; offline | ✓ |
| Per-alias probes | N×6 matrix; overlaps internal/tester | |
| github.com | Hits hand-written blocks — not global | |

**Notes:** Settled by frozen design: three-tier provenance labels. Verified finding: Apple `ssh -G` omits `usekeychain` even when set → UseKeychain is file-parse-only with hedged wording, by design.

---

## Managed-block home vs create's Host *

| Option | Description | Selected |
|--------|-------------|----------|
| Single block, key-union merge (Recommended) | One globals module; create + GSSH both call EnsureGlobals(); existing values beat platform defaults | ✓ |
| Two blocks | _global + global-ssh — gitid-vs-gitid shadowing on overlapping keys; doctor flags gitid's own output | |
| Desired-state store | Introduces the sidecar-state pattern the project avoids | |

| Option | Description | Selected |
|--------|-------------|----------|
| Layout-follows-identities (Recommended) | Last block of gitid.config (Include'd) / last gitid block (in-file); floored Include gives precedence | ✓ |
| Always main ~/.ssh/config | Splits ownership; cross-file gitid shadowing | |
| Block-prepend under Include | Precedence-grabbing; inverts identities-first | |

| Option | Description | Selected |
|--------|-------------|----------|
| Rename to global-ssh + registry (Recommended) | Frozen sentinel honored; both names reserved; 4+ string literals → registry calls | ✓ |
| Keep _global, amend copy | Violates frozen contractual text | |

**Notes:** Code findings grounding this: create whole-block-replaces `_global` every create (naive Phase 6 fixes would be erased); `_global` NOT in IsReservedBlockName (string-literal special cases in reader/overlap/migrate/delete); doctor's redundancy check already tells users to consolidate into ONE gitid block. Settled: identities-first / Host*-last ordering (T-02-15), recipe's top-of-file Host * divergence documented (inert there, harmful here).

---

## Platform set + recommended values

| Option | Description | Selected |
|--------|-------------|----------|
| Show-disabled + always guard (Recommended) | UseKeychain row visible on Linux ("macOS-only" note, keeps 6-row contract); IgnoreUnknown UseKeychain always written lexically first | ✓ |
| Hide row on Linux | Breaks frozen option_row × 6; forks baselines per OS | |
| Guard only | Linux users see an unusable recommendation | |

| Option | Description | Selected |
|--------|-------------|----------|
| Approve as proposed (Recommended) | accept-new(M) / no(H) / yes(L) / yes-PER-ALIAS(H) / yes(L) / yes-guarded(L) | ✓ |
| Approve with StrictHostKeyChecking=ask | Weaker MITM posture | |
| Revise the table | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Distinct word state (Recommended) | Yellow ! + "set, differs from recommendation" — danger visible, choice respected | ✓ |
| Same yellow ! as unset | Nags deliberate choices | |
| Suppress severity | Hides the exact High-risk case; fails GSSH-01 | |

| Option | Description | Selected |
|--------|-------------|----------|
| Static facts + dynamic line (Recommended) | Frozen copy has stable version facts; non-contractual "your OpenSSH: X.Y" via ProbeSSHVersion() gates accept-new | ✓ |
| Timeless copy | Weaker "why dangerous" answer | |

**Notes:** IdentitiesOnly recommendation scoped per-alias, never Host * (recipe scoping; global would break non-gitid hosts) — its fix verifies per-alias conformance. SHKC/ForwardAgent/HashKnownHosts are beyond-recipe advisory hardening, documented. Recipe's literal first directive IS `IgnoreUnknown UseKeychain`.

---

## Decline persistence & apply selection

| Option | Description | Selected |
|--------|-------------|----------|
| Stateless + derived state (Recommended) | No decline record (doctor-tool precedent + MGR-08); any explicit value → "already set"/"set, differs — your choice" | ✓ |
| Marker comment in block | Write ceremony for a non-change; re-opens doctor false-positive wound | |
| Pure stateless only | Loses the file-native silencing path | |

| Option | Description | Selected |
|--------|-------------|----------|
| Empty, opt-in (Recommended) | Selection starts empty; space toggles; f guarded when empty | ✓ |
| Preselected, opt-out | Fewer keystrokes; consent-by-default tension | |

**Notes:** Settled by frozen design: multi-select → ONE combined ceremony ("Applying 3 of 4" banner, one diff, linear f→w→y→z); toward-recommendations only (no free-form editor — hand-edit + "set, differs" covers deliberate values). Ecosystem: brew/flutter doctor are fully stateless; in-file suppression markers (eslint/noqa style) rejected for this context.

---

## Claude's Discretion

- Whether "set, differs" counts in the "N of M need action" tally (pin during UI wave, before copy freeze).
- Temp-file location/perms for the shadowing simulation (ssh's config-perm checks).
- Exact copy: not-applicable row, "set, differs — your choice", shadowed-fix warning, provenance labels, six contractual explanations (static version facts only).
- `gitid ssh options …` CLI shape per Phase 5 D-01..D-04 + parity-matrix entry.
- Legacy `_global` adoption/rename mechanics + migration test.

## Deferred Ideas

- known_hosts hashing execution (`ssh-keygen -H` run for the user) → Phase 8 fixer candidate.
- "User directive above the managed block" health check → Phase 8 (HLTH-04), fed by the "applied but shadowed" finding class.
- Per-alias drill-down of global options (N×6 matrix) → only if a real need emerges.
- Free-form SSH option value editor → out of scope; needs a design re-freeze.
- Preselected opt-out selection default → revisit at UAT (identical code path).
