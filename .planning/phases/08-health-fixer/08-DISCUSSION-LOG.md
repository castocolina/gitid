# Phase 8: Health + Fixer - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-07
**Phase:** 8-health-fixer
**Areas discussed:** Family→section mapping + CLI, New checks scope (5-7 queue), Fix-in-place on hand-written config, Batch fixes + re-evaluation loop
**Mode:** `--research` — four parallel `gsd-advisor-researcher` agents; doctor-substrate facts pre-gathered via codegraph and injected (per user instruction). All research recommendations accepted.

---

## Family→section mapping + CLI

| Option | Description | Selected |
|--------|-------------|----------|
| Hybrid Target field (Recommended) | Finding.Target + family-default fallback; only cross-file families set it explicitly; computed once for health/fixer/--json/MGR-07 | ✓ |
| Full per-finding classification | Most accurate; widest diff through tested package | |
| Static family→section table | Provably wrong for cross-file families | |

| Option | Description | Selected |
|--------|-------------|----------|
| health + fix, hidden doctor alias (Recommended) | gitid health [--json] + gitid fix [--yes --dry-run]; doctor = permanent hidden alias + --fix forwarding shim (docker ps precedent) | ✓ |
| Visible deprecated doctor | Deprecation noise + breaking removal later | |
| Keep doctor --fix canonical | Conflicts with locked Phase 5 noun groups + read-only affordance | |

| Option | Description | Selected |
|--------|-------------|----------|
| --identity flag (Recommended) | Filter over existing Finding.IdentityName + completion | ✓ |
| Positional name | Occupies future subcommand slot | |

**Notes:** In-section grouping settled by frozen design: flat severity-sorted rows; family only as detail chip.

---

## New checks scope (5-7 queue)

| Option | Description | Selected |
|--------|-------------|----------|
| Full queue minus known_hosts (Recommended) | Parse gates (new Files family), gitignore fix, shadowed, set-differs, author-resolution, both pinned contradictions, directive-above-block + mandatory tolerances | ✓ |
| Full queue + known_hosts | Also ships ssh-keygen -H; new file surface in the same phase | |
| Fixture minimum | Silently breaks Phase 6/7 written hand-offs | |

| Option | Description | Selected |
|--------|-------------|----------|
| Backlog with named home (Recommended) | known_hosts hashing → post-v1.0 or Phase 8 stretch; gated on HashKnownHosts=yes already set | ✓ |
| Ship now as gated info fix | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Approve disposition + obligations (Recommended) | Per-check severities per fixtures (IdentitiesOnly = ERROR; parse fail = CRITICAL pause-gate; set-differs = info hard cap); CheckOrphans git-only-delete downgrade; reserved-PATH registry + archive regression test; escalation-cap tests | ✓ |
| Revise | | |

**Notes:** Research found CheckOrphans Class-1 currently offers a destructive RemoveBlock fix against Phase 5's deliberate git-only-delete state — the false-positive-loop class, now a binding downgrade obligation. Reserved-path registry does not exist (block names only).

---

## Fix-in-place on hand-written config

| Option | Description | Selected |
|--------|-------------|----------|
| Surgical edit, scoped amendment (Recommended) | Fixer-only, ONE directive, full ceremony; CLAUDE.md scoped amendment. FIXTURE-VERIFIED: flagship diff is sentinel-less (hand-written) + result copy pins "only the rewritten directive changed" | ✓ |
| Adopt-then-fix | Sentinels would change more than the directive — contradicts frozen copy | |
| Managed-blocks-only | Makes the DLV-approved flagship unimplementable | |

| Option | Description | Selected |
|--------|-------------|----------|
| Typed hostname confirm (Recommended) | dummytui ConfirmWord depiction (test-asserted) wins; amend the FIELDS.md sentence | ✓ |
| Strong focus-confirm, no typing | Honor FIELDS.md sentence; change fixture + tests | |

| Option | Description | Selected |
|--------|-------------|----------|
| Full loop, mandatory (Recommended) | parse→render→re-parse + post-apply ssh -G / git config re-verify + auto-restore on mismatch; headless degrade | ✓ |
| Render + re-parse only | Weaker than the Phases 6-7 loops | |

**Notes:** Research surfaced the frozen-artifact divergence (FIELDS.md "short of typed" vs dummytui typed ConfirmWord) — ruled for typed. Certbot's rollback history cited as the cautionary precedent.

---

## Batch fixes + re-evaluation loop

| Option | Description | Selected |
|--------|-------------|----------|
| Approve bundle (Recommended) | Auto-advancing queue of per-finding ceremonies (frozen fixerBatchFixNote); full re-check after EVERY fix; explicit "fix did not converge" error finding + permanent exclusion (upgrades convergeFixes' silent stop; ESLint precedent); re-evaluated list = remaining-findings report; single-fix rollback scope | ✓ |
| Adjust parts | | |

**Notes:** Fixture check settled batch shape first: "Apply all N fixes — each one still previews" + no-7th-state rule foreclose a combined ceremony. convergeFixes already exists (cmd/gitid/doctor.go:185, maxPasses=10) but stops silently.

---

## Claude's Discretion

- Global-findings presentation in the --identity scoped view.
- Files family display name; finding-signature disambiguation.
- Copy: convergence alarm, shadowed-fix file:line format, parse-error wiring, queue-halt message.
- Shared detector implementations with Phases 6-7 probes.
- doctor --fix shim flag mapping.

## Deferred Ideas

- known_hosts hashing fix (gated) → post-v1.0 or Phase 8 stretch.
- Adopt-this-block offer after a surgical fix → later phase, outside the 6 fixer states.
- Signers-line prune fixer (opt-in; excludes git-only-deleted principals) → later.
- result-applied "N remaining · also resolved: M" field → UI-wave designer's call.
- Phase 9 upload-artifact health checks → Phase 9.
