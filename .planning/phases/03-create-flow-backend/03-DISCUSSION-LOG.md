# Phase 3: Create Flow Backend - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-07
**Phase:** 3-create-flow-backend
**Areas discussed:** Test gate semantics, Storage-target choice, Reuse-existing-key UX, Real-binary entry point, Wizard Git-step in Phase 3, Provider model in SSH form, PTY e2e vs network, DLV-04 diff mechanics

---

## Test gate semantics

| Option | Description | Selected |
|--------|-------------|----------|
| PASS or ReachableNotUploaded | Store unlocks on proven connectivity + `ssh -G` resolution; identity persists with "key not yet uploaded" status | ✓ |
| Only PASS | Hard gate: manual upload mid-flow, re-test until auth banner | |
| User decides in-flow | Explicit store-anyway vs upload-now choice screen | |

| Option | Description | Selected |
|--------|-------------|----------|
| New warning state | Yellow ! + word, "Reachable — key not uploaded yet"; documented scoped divergence | ✓ |
| Render as success + note | Green ✓ with hint — overclaims auth | |
| Render as failure + affordance | Red ✗ + store-anyway — misleading for the common path | |

| Option | Description | Selected |
|--------|-------------|----------|
| Copy .pub + one-line hint | Clipboard keystroke + provider key-settings hint; Phase 9 owns the rest | ✓ |
| Message only | Hint line, no clipboard action | |
| Full manual instructions inline | UP-01 copy inside create flow — duplication risk | |

| Option | Description | Selected |
|--------|-------------|----------|
| Gate stage 2 on stage 1 | Stage 2 only after stage-1 success; failure stops with retry | ✓ (refined) |
| Always run both stages | Both run regardless — noisy duplicate failure | |
| User-driven stages | Independent run actions per stage | |

**User's choice:** Option 1 with refinement: "But trigger 2 instantly if 1 successful and the user set alias" — stage 2 auto-chains with no manual keypress.

---

## Storage-target choice

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-detect, follow layout | Detect the active layout, name the resolved target in confirm-write; no new screen | ✓ |
| In-flow prompt each create | Storage-choice step per create — design divergence | |
| One-time setting | Persisted preference — sidecar state the project avoids | |

| Option | Description | Selected |
|--------|-------------|----------|
| Not in Phase 3 (migrate UX) | Migrate surfaces later (Phase 6/8) | ✓ (refined) |
| CLI-only escape hatch now | `gitid storage migrate` Cobra command | |
| Yes, in the create flow | Offer migration in the happy path | |

**User's choice:** Option 1 with refinement: "Includes as default" — the Include'd layout becomes the fresh-setup DEFAULT.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — Include'd is default | Fresh setups write config.d/gitid.config + Include line; confirm previews BOTH changes; supersedes STORE-01's in-file default | ✓ |
| No — keep in-file default | Stick with STORE-01's documented default | |

| Option | Description | Selected |
|--------|-------------|----------|
| Every create, idempotent | Host * globals (re)normalized on every write ceremony | ✓ |
| First create only | Written once; user-deleted block stays missing until Fixer | |
| Separate opt-in step | Extra decision the approved design doesn't surface | |

| Option | Description | Selected |
|--------|-------------|----------|
| Block at the SSH form | Alias validated against ALL parsed Host patterns as-you-type; inline error; form won't advance | ✓ |
| Warn but allow | Persist knowingly-ambiguous config | |
| Only block managed collisions | Hand-written entries get a hint only | |

---

## Reuse-existing-key UX

| Option | Description | Selected |
|--------|-------------|----------|
| Scan ~/.ssh + picker | List filename + algorithm + fingerprint; manual-path fallback row | ✓ |
| Manual path entry only | Single text field | |
| Picker, no manual fallback | Blocks keys outside ~/.ssh | |

| Option | Description | Selected |
|--------|-------------|----------|
| Parse + derive .pub + fix perms | Must parse; derive missing .pub; perms 600/644 in ceremony (previewed); encrypted OK when .pub exists | ✓ |
| Existence check only | Let the connectivity test catch problems | |
| Strict: reject encrypted keys | Excludes passphrase-protected setups | |

| Option | Description | Selected |
|--------|-------------|----------|
| Warn, allow | "in use by: <identity>" label + same-provider warning; never blocks | ✓ |
| Block same-provider reuse | Blocks on inference gitid can't verify | |
| Allow silently | Sets up confusing auth failures | |

| Option | Description | Selected |
|--------|-------------|----------|
| Any parseable SSH key | Legacy algorithms accepted with informational note | ✓ |
| Catalog algorithms only | Rejects real-world legacy keys | |
| Only ed25519 + rsa-4096 | Mirrors generate path — least useful | |

---

## Real-binary entry point

| Option | Description | Selected |
|--------|-------------|----------|
| Shell + placeholders | Bare `gitid` opens the real shell; create live; unbuilt views placeholder | (refined) |
| `gitid create` only, no shell yet | Wizard alone; shell waits for Phase 5 | |
| Both entries | Shell + deep-link subcommand | |

**User's choice (freeform):** "Archive the old app, copy the demo shell to main adding note labels warn at beginning of the others screens and start working to the corresponding one."

| Option | Description | Selected |
|--------|-------------|----------|
| Demo content + warning note | Unbuilt views render approved demo screens with a persistent "Preview — demo data" note | ✓ |
| Bare placeholder | Shell chrome + "coming in a later build" line | |

| Option | Description | Selected |
|--------|-------------|----------|
| Remove POC commands, keep debug | Delete POC Cobra surface; keep `debug caps`; CLI rebuilt in Phase 5 | ✓ |
| Keep POC CLI until Phase 5 | Ships known-obsolete UX under v1.0 | |
| Move POC to a hidden namespace | `gitid legacy …` escape hatch | |

| Option | Description | Selected |
|--------|-------------|----------|
| Extract shared UI package | Backend-free presentation package imported by both binaries; fixtures vs backend state injected | ✓ |
| Literal copy into the real app | Two copies of a frozen design | |
| Real app imports dummytui | Baked-in fixtures; muddies the no-backend gate | |

---

## Wizard Git-step in Phase 3

| Option | Description | Selected |
|--------|-------------|----------|
| Skip functional, form demo'd | Skip writes SSH-only identity (incomplete); Git form demo+warning; Continue unlocks in Phase 4 | ✓ |
| Wire the Git form now | Pulls Phase 4 scope forward | |
| Auto-skip the step | Hides an approved wizard step | |

| Option | Description | Selected |
|--------|-------------|----------|
| Disabled + new reason | Scoped-divergence disabled reason ("— Git configuration arrives with the next build"), removed in Phase 4 | ✓ |
| Keep frozen reason only | Silently lies about why Continue won't proceed | |
| Enabled into a demo review | Users type data the build visibly ignores | |

---

## Provider model in SSH form

| Option | Description | Selected |
|--------|-------------|----------|
| Infer from SSH Host suffix | Known-provider table auto-fills Real hostname + Port; 4-field form stays byte-exact | ✓ |
| Add a provider selector | New field = visible scoped divergence | |
| No inference, plain defaults | Manual edits for non-GitHub providers | |

| Option | Description | Selected |
|--------|-------------|----------|
| 22 for unknown + hint | Unknown suffix → port 22, hostname = host itself, alt-SSH hint; known providers keep recipe 443 pairing | ✓ |
| Always 443 | Wrong default for custom servers | |
| Empty for unknown | Friction the approved form doesn't show | |

---

## PTY e2e vs network

| Option | Description | Selected |
|--------|-------------|----------|
| PATH-shim fake `ssh` | Fake ssh executable on PATH emits recorded real outputs; binary 100% real | ✓ |
| Local in-process SSH server | Real connections; materially heavier harness | |
| Real provider in CI | Flaky; can't produce PASS without registered keys | |

| Option | Description | Selected |
|--------|-------------|----------|
| Skippable local smoke | make target vs github.com asserting ReachableNotUploaded/PASS; auto-skip offline; not a CI gate | ✓ |
| No real-network tests | Nothing exercises the true handshake before UAT | |
| Required in CI | Adopts provider availability as a CI failure mode | |

---

## DLV-04 diff mechanics

| Option | Description | Selected |
|--------|-------------|----------|
| Golden-text diff + agent critique | Automated make gate on View() text goldens (divergence allowlist) + agent-ui-ux-designer PNG critique | ✓ (refined) |
| Pixel-diff threshold | Arbitrary thresholds; misses what text-diff catches | |
| Agent review only | Non-deterministic; burns review cycles | |

**User's choice:** Option 1 with refinement: "Also use codex to review both, text and screen" — Codex joins the review of both text diffs and screenshots (cross-AI, Phase 2 pattern).

| Option | Description | Selected |
|--------|-------------|----------|
| Hard gate + severity triage | Unallowlisted text diff fails outright; CRITICAL/HIGH findings block; MEDIUM/LOW recorded | ✓ |
| Every finding blocks | Nitpicks can stall the wave | |
| Gate advisory, reviews decide | Reintroduces non-determinism | |

---

## Claude's Discretion

- Shared UI package name/layout and fixture-vs-backend injection shape (D-17).
- Capture geometry, golden-file layout, allowlist format for the D-24 gate.
- Known-provider table location/shape (D-20).
- Exact copy for the warning state, demo-note banner, and Continue disabled reason (frozen via the 02-STYLE-SPEC §6 grep mechanism once drafted).
- Test-stage timeout/retry budgets.

## Deferred Ideas

- Layout-migration UX → Phase 6 (Global SSH Options) / Phase 8 (Fixer).
- Full upload instructions + automation (UP-01..03) → Phase 9.
- CLI create / non-interactive flags → Phase 5 (SHELL-03).
- Git form wiring (fragment + includeIf + allowed_signers) → Phase 4.
- W1 `insteadOf` demo gap → Phase 4/7 design concern.
