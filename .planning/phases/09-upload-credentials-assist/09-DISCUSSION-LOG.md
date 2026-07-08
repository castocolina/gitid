# Phase 9: Upload / Credentials Assist - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-08
**Phase:** 9-upload-credentials-assist
**Areas discussed:** Autonomy boundary & triggers, Missing design surface, Tool selection & provider match, Idempotency & partial success
**Mode:** `--research` — four parallel `gsd-advisor-researcher` agents; substrate facts pre-gathered via codegraph (per user instruction: legacy `internal/upload`/`internal/uploader` + dummytui). All external facts verified with citations (gh source/manual, glab docs + tag bisection, GitLab API docs).

---

## Autonomy boundary & triggers

| Option | Description | Selected |
|--------|-------------|----------|
| Fresh pairings (Recommended) | create/rotate/clone/add-account/reuse-key auto; copy stays --upload-keys | |
| New-key flows only | Tightest boundary; leaves add-account/reuse manual | |
| Everything including copy | Read command mutating remote breaks least-surprise | |
| USER FREEFORM | Wizard checkbox with DERIVED state: main-domain match + CLI present + authenticated → enabled+pre-checked; present-but-unauth → enabled+unchecked (pre-checkable, degrades to manual if still unauth); no match/self-hosted → disabled | ✓ (user model, confirmed "Correct as stated") |

| Option | Description | Selected |
|--------|-------------|----------|
| Announce-and-do (Recommended) | Print each command exactly as executed + per-key results; no prompt, no timer | ✓ |
| Announced + cancel window | A timed window IS a stop; flaky PTY e2e | |
| Fully silent | Violates audit ethos + shown==run | |

| Option | Description | Selected |
|--------|-------------|----------|
| Exit 0 + warn (Recommended) | D-11 honored; duplicates = idempotent success | ✓ + USER ADDITION |
| Non-zero exit on upload failure | Contradicts D-11 observably | |
| Distinct exit code | Nobody checks tertiary codes | |

**Notes:** USER: "CLI is fine but I need it integrated in UI in the identity wizard" → in-wizard upload step (announced commands, per-key results, manual fallback); rotate/clone/copy reuse the component. Confirmed "Yes — in-wizard step".

| Option | Description | Selected |
|--------|-------------|----------|
| Leave + report (Recommended) | Remote deletion irreversible/un-backupable; report exact delete command | |
| Interactive delete offer | Confirmed ceremony post-rotate; needs key-ID lookup | ✓ (USER OVERRIDE) |
| Autonomous remote delete | Destructive remote mutation, no undo | rejected |

| Option | Description | Selected |
|--------|-------------|----------|
| Before ssh -T test (Recommended) | Gate can genuinely PASS; existing gate absorbs lag | ✓ |
| After persist/test | Always lands ReachableNotUploaded, needs re-test | |

| Option | Description | Selected |
|--------|-------------|----------|
| --no-upload + --dry-run (Recommended) | gh --skip-ssh-key precedent; dry-run prints CommandPreview only | ✓ |
| Also GITID_NO_UPLOAD env | Second config surface | |
| No opt-out | Cautious users have no recourse | |

**Late user-raised decision (key title):**

| Option | Description | Selected |
|--------|-------------|----------|
| gitid: \<name\> @ \<hostname\> (Recommended) | Machine-distinguishable, greppable prefix; title matching machine-scoped | ✓ |
| gitid: \<name\> (\<user\>@\<hostname\>) | ssh-keygen-style, noisier | |
| Keep gitid: \<name\> | Two machines indistinguishable on provider | |

**Notes:** USER: "personal ramon@gmail.com at linuxmachine is not same at machostname" — raised at the create-context gate; content match stays dedupe truth, title adds the machine axis.

---

## Missing design surface

| Option | Description | Selected |
|--------|-------------|----------|
| Hybrid amendment (Recommended) | ONE shared upload-section component contract; amend create-flow + identity-manager FIELDS.md; dummytui states + fresh approved captures minted in UI wave; rework tui/copy.go | ✓ |
| New frozen 'upload' surface | Upload is not a navigable view; gate would diff a synthetic frame | |
| CLI-first minimal TUI | Cannot render 2 per-key results/command echo/manual block; contradicts in-wizard decision | |

| Option | Description | Selected |
|--------|-------------|----------|
| Approve sequence (Recommended) | ui-phase spec → FIELDS amendments → dummytui/mockup + parity critique → capture+approve → tui/copy.go rework → backend + PTY e2e + visual diff | ✓ |
| Revise sequence | | |

**Notes:** Research discovered the 100 static reference PNGs were removed repo-wide (REFERENCE-INDEX.md) — live demos are the approved reference, so Phase 9 must mint criterion-4 captures under ANY option. Also: tui/copy.go's POC per-key Enter/skip queue contradicts UP-03 — redesigned in the contract, not in code review.

---

## Tool selection & provider match

| Option | Description | Selected |
|--------|-------------|----------|
| DetectFor(provider) (Recommended) | github→gh, gitlab→glab, unknown→manual; never cross-route | ✓ |
| Probe both, pick match | Same outcome + extra probe for a status display | |
| Keep first-found | GitLab identities route to gh; unauth gh shadows authenticated glab | rejected |

| Option | Description | Selected |
|--------|-------------|----------|
| auth_and_signing (Recommended) | VERIFIED glab default; values auth/signing/auth_and_signing; flag since v1.54.0 (2025-03-19). Pinned "auth" silently loses signing — A2 closed | ✓ |
| Omit the flag | Intent disappears from shown command; default drift | |
| Runtime-probe --help | Brittle parsing for shrinking old-glab population | |

| Option | Description | Selected |
|--------|-------------|----------|
| Main domains only (Recommended) | github.com/gitlab.com; matches checkbox model; GH_HOST plumbing deferred (Deps.RunCmd API + preview parity). VERIFIED: gh ssh-key add has no --hostname | ✓ |
| Env plumbing now | ~5 files, API change, preview must render env prefix | |

| Option | Description | Selected |
|--------|-------------|----------|
| Host-scoped + classify (Recommended) | auth status --hostname; attempt-and-classify scope errors → remediation copy (gh auth refresh -s admin:public_key / admin:ssh_signing_key); gh partial success handled | ✓ |
| Keep bare auth status | Checks ALL hosts — false both directions | |
| Pre-flight scope parsing | "Token scopes:" line is not a stable API | |

---

## Idempotency & partial success

| Option | Description | Selected |
|--------|-------------|----------|
| Inventory-first (Recommended) | gh api user/keys + user/ssh_signing_keys; glab ssh-key list -F json; normalized-blob compare; skip present types; list failure → plain upload (never gates); supplies rotate delete-offer key IDs | ✓ |
| No dedupe, classify errors | glab "has already been taken" is AMBIGUOUS (fingerprints globally unique across accounts) — cross-account conflict could read as success | |

| Option | Description | Selected |
|--------|-------------|----------|
| Independent per-type ensure (Recommended) | Inventory-driven missing-type diff; per-registration result struct; partial failure reported per-type; next run converges | ✓ |
| Sequential abort-on-first | Signing gap invisible; one opaque error for two registrations | |

| Option | Description | Selected |
|--------|-------------|----------|
| ssh -T + inventory read (Recommended) | ReachableNotUploaded → PASS (end-to-end + catches GitLab cross-account); inventory read verifies signing; one bounded retry | ✓ |
| ssh -T only | Signing stays unverified in an autonomous flow | |
| Report-only | Silent signing failures persist | |

| Option | Description | Selected |
|--------|-------------|----------|
| No persisted state (Recommended) | ReachableNotUploaded IS the health signal; derived-state files = doctor false-positive footgun | ✓ |
| Persist per-identity upload state | Drifts on server-side deletion; new doctor surface | |
| Live inventory in health | Network dependency in health runs | |

**Notes:** Verified: gh dedupes client-side since v2.27.0 (same release window as --type signing → any gh that runs gitid's command dedupes, exit 0); glab does NOT dedupe (400 fingerprint taken); titles not unique on either platform (rotate collision allowed).

---

## Claude's Discretion

- Copy: announced-running lines, per-key results, manual-fallback wording, scope remediation, auth-login hint, checkbox labels, delete-offer ceremony.
- Flag naming/wiring within Phase 5 adaptive-CLI + parity contract.
- Inventory function shape + JSON parsing + blob normalization.
- Hostname normalization for the key title.
- Upload step's exact wizard-beat position (pinned during UI wave).
- Error-classifier looseness (scope name / fingerprint keyword substrings).

## Deferred Ideas

- GH_HOST/GITLAB_HOST env plumbing (self-hosted autonomous upload) → post-v1.0.
- GITID_NO_UPLOAD env opt-out → only if a fleet use case appears.
- Inventory-backed signing finding in health screen → post-v1.0.
- Tool-inventory status display → only if the TUI wants it.
- Distinct "upload degraded" exit code → only if a CI consumer materializes.
