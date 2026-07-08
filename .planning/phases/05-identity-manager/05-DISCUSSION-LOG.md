# Phase 5: Identity Manager - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-07
**Phase:** 5-identity-manager
**Areas discussed:** CLI parity surface (SHELL-03), Key lifecycle rotate vs new-key, Delete semantics shared artifacts, Clone semantics
**Mode:** `--research` — each area researched by a parallel `gsd-advisor-researcher` agent (comparison tables + rationale) before the user decided. All four research recommendations were accepted.

---

## CLI parity surface (SHELL-03)

| Option | Description | Selected |
|--------|-------------|----------|
| Noun-verb + flat aliases (Recommended) | `gitid identity <verb>` + `ssh`/`git`/`health`/`fix` noun groups; Cobra aliases keep `gitid create` | ✓ |
| Pure noun-verb | Same tree, no top-level aliases | |
| Flat verbs | Shortest now; verb collisions when Phases 6-8 land | |
| Hybrid | Identity verbs top-level, rest nested — inconsistent model | |

| Option | Description | Selected |
|--------|-------------|----------|
| Adaptive gh-style (Recommended) | Complete flags → headless (`--yes` skips prompt only, `--dry-run` test+preview, backups unconditional); incomplete+TTY → pre-filled wizard; incomplete+non-TTY → error | ✓ |
| Headless-only | All inputs via flags; hostile first-run UX | |
| Deep-link TUI only | Cosmetic parity; fails in CI | |

| Option | Description | Selected |
|--------|-------------|----------|
| --json + TTY-aware plain (Recommended) | Table on TTY, tab-delimited piped, `--json` marshals the 8-state model; PTY-free e2e surface | ✓ |
| Plain TTY-aware only | No schema commitment; brittle column parsing | |
| gh-style --json fields + --jq | gojq dependency — overkill at v1.0 | |

| Option | Description | Selected |
|--------|-------------|----------|
| Outcome parity + matrix (Recommended) | Every product outcome gets a command via the shared ceremony chokepoint; requirement-keyed parity matrix | ✓ |
| Literal 1:1 | Ceremony steps as commands — invocable out of order | |
| Read parity + deep-links | Writes unusable in CI; interim quality | |

**Notes:** Researcher framing accepted: gitid maps onto gh/glab (two skins over one core), not lazygit/k9s (TUI companions to an external CLI).

---

## Key lifecycle: rotate vs new-key

| Option | Description | Selected |
|--------|-------------|----------|
| Retirement vs repair (Recommended) | Rotate = archive old key + provider guidance (healthy); new-key = fresh key, old material untouched (key-missing/shared-key repair) | ✓ |
| Single replace-key flow + toggle | One path; buries retirement semantics; breaks KEY-05/KEY-07 traceability | |
| Algorithm-change distinction | Weak distinction; filename churn | |

| Option | Description | Selected |
|--------|-------------|----------|
| Archive dir (Recommended) | `~/.ssh/gitid-archive/<file>.<timestamp>` (700/600); canonical path freed; MUST be doctor-reserved | ✓ |
| Rename .old-<ts> in place | Stray keys clutter ~/.ssh | |
| Keep untouched, new filename | Breaks `id_ed25519_<name>` convention permanently | |
| (Delete immediately) | Rejected in research — destroys the only provider-accepted credential pre-upload | |

| Option | Description | Selected |
|--------|-------------|----------|
| Append, keep old line (Recommended) | Preserves `git log --show-signature` for pre-rotation commits | ✓ |
| Replace old line | Historic verification silently breaks | |

| Option | Description | Selected |
|--------|-------------|----------|
| Grace-window hint (Recommended) | "old key remains valid at <provider>; upload the new key, verify, then remove the old one" | ✓ |
| Hint + advisory old-key pre-check | Extra ssh -T; old key often already dead when rotating | |
| Immediate-invalidation guidance | Guaranteed auth outage; compromised-key path only | |

**Notes:** Post-rotate test gate reuses Phase 3 D-01/D-02/D-03 verbatim; expected landing state is ReachableNotUploaded.

---

## Delete semantics: shared artifacts

| Option | Description | Selected |
|--------|-------------|----------|
| Ref-counted auto-removal (Recommended) | Last-identity-for-provider delete removes the insteadOf block, named in confirm file list | ✓ |
| Leave in place | Orphaned rewrite breaks all https clones for the provider | |
| Extra ask in ceremony | Third question on the frozen delete-choice screen | |

| Option | Description | Selected |
|--------|-------------|----------|
| Keep line in git-only (Recommended) | allowed_signers removed only in delete-everything; safer mode stays safer | ✓ |
| Remove in both modes | Old commits flip to "unverified" in the safer mode | |

| Option | Description | Selected |
|--------|-------------|----------|
| Backup-copy then delete (Recommended) | Key pair copied to timestamped backup dir (0700/0600) before removal; precise confirm copy | ✓ |
| True delete, no copy | Only artifact with no undo | |
| Backup + purge toggle | Third field on frozen confirm-destructive screen | |

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-downgrade (Recommended) | Shared key kept; SSH+Git deleted; note names the sibling identity | ✓ |
| Loud warn + explicit confirm | Extra decision at the worst moment | |

**Notes:** Two research findings presented as settled-unless-objected and accepted: (1) fragment + includeIf both removed in git-only mode with fragment backed up before unlink (orphaning = the doctor-loop bug class); (2) unmanaged-reference static scan, warn-never-block, honest about unscannable repo remotes.

---

## Clone semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Copy + re-derive (Recommended) | SSH copied w/ new alias; name/email flagged "copied from <source> — review"; gitdir/hasconfig/signingkey/allowed_signers re-derived from NEW name+key | ✓ |
| Full verbatim copy | Wrong-by-definition signingkey when key changes | |
| SSH fields only | Clone barely beats create | |

| Option | Description | Selected |
|--------|-------------|----------|
| Pre-filled create wizard (Recommended) | clone-name-prompt → full Phase 3/4 wizard with injected initial model; ONE write path | ✓ |
| Short prompt → ceremony | Second write path; no edit stop | |
| Hybrid review-with-edit | Undesigned navigation seam | |

| Option | Description | Selected |
|--------|-------------|----------|
| Full two-stage re-run (Recommended) | New Host block's ssh -G resolution is the unproven artifact; zero new gate code | ✓ |
| Stage 2 only | Forks gate semantics between create and clone | |

| Option | Description | Selected |
|--------|-------------|----------|
| -clone + auto-bump (Recommended) | `<source>-clone`, bump to `-clone-2` when taken, live D-09 validation; never opens in error | ✓ |
| No auto-bump | Prompt can open already-in-error | |

**Notes:** Instant-duplicate patterns (write-then-rename) were discarded in research as violating MGR-04 "customizes before writing" + prove-before-write.

---

## Claude's Discretion

- Cobra flag names/shorthands; parity-matrix document location/format.
- `list`/`show` JSON schema shape (stable once shipped).
- gitid-archive filename details + reserved-registration mechanics.
- Exact copy for: grace-window hint, shared-key downgrade note, unmanaged-reference warning, "copied from <source> — review" flag, delete-everything confirm phrasing (freeze via 02-STYLE-SPEC §6).
- D-13 scan substring false-positive guard (superstring-principal pattern analog).

## Deferred Ideas

- Archived-key / stale-signers pruning (`gitid key purge` or fixer action) → Phase 8.
- Compromised-key path (immediate invalidation + true no-copy delete) → Phase 8 / post-v1.0.
- gh-style `--json fields` + `--jq` → only if an automation ecosystem emerges.
- Auto-upload of the rotated `.pub` → Phase 9 (UP-01..03).
- Real Global SSH / Global Git / Health / Fixer view content → Phases 6–8.
