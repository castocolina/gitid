---
phase: 5
reviewers: [codex, xai-grok]
reviewed_at: 2026-08-25T12:44:40Z
plans_reviewed:
  - .planning/phases/05-identity-manager/05-01-PLAN.md
  - .planning/phases/05-identity-manager/05-02-PLAN.md
  - .planning/phases/05-identity-manager/05-03-PLAN.md
  - .planning/phases/05-identity-manager/05-04-PLAN.md
  - .planning/phases/05-identity-manager/05-05-PLAN.md
  - .planning/phases/05-identity-manager/05-06-PLAN.md
  - .planning/phases/05-identity-manager/05-07-PLAN.md
  - .planning/phases/05-identity-manager/05-08-PLAN.md
  - .planning/phases/05-identity-manager/05-09-PLAN.md
models:
  codex: "gpt-5.6-sol (reasoning=low)"
  xai-grok: "xai/grok-4.6 (reasoning=low)"
model_sources:
  codex: "banner"
  xai-grok: "pinned"
---

# Cross-AI Plan Review — Phase 5 (Identity Manager)

> **Configured-reviewer note:** `review.default_reviewers` names the instances `codex-sol`
> (`cli: codex`, `model: openai/gpt-5.6-sol-fast`) and `xai-grok` (`cli: opencode`, `model:
> xai/grok-4.6`). `xai-grok` ran as configured. `codex-sol` failed at invocation —
> `openai/gpt-5.6-sol-fast` is rejected by this host's Codex CLI when authenticated with a
> ChatGPT account (`invalid_request_error: The 'openai/gpt-5.6-sol-fast' model is not
> supported when using Codex with a ChatGPT account.`), returning empty output both times it
> was tried. The `codex` builtin lane was invoked instead, using Codex's own default model
> (`gpt-5.6-sol`, resolved from its banner), and its review is reported below under the
> `codex` identity. Fix the `codex-sol` instance's pinned model (or the account's plan) before
> the next `/gsd-review` run if the specific `openai/gpt-5.6-sol-fast` model is actually
> required.

## Consensus Summary

Both reviewers read the nine plans against the live repository (recipes, `internal/identity`,
`internal/keygen`, `internal/gitconfig`, `internal/sshconfig`, `cmd/gitid/wiring.go`,
`internal/tuikit/store.go`/`identities.go`) rather than the plan text alone, and both cite
concrete `file:line` evidence for every finding — no reviewer output carries the
`[reviewed-without-repo-access]` or `[reviewed-without-source-citations]` markers, so both
count at full consensus weight.

Both independently converge on the same root defect class: **`internal/identity` orchestration
functions (`runPipeline`, `ArchiveKeyPair`, `Delete`) were designed as single-purpose primitives,
and Phase 5's plans reuse them for new call shapes (rotate, repair, delete-everything) that
those primitives cannot safely support without refactoring first.** The phase's core wiring
target — `realBackend.Persist`'s fallthrough to the shared demo reducer
(`cmd/gitid/wiring.go:352-361`) — is confirmed as real and load-bearing by both reviewers, but
both also find that closing it correctly depends on fixing the primitives underneath it, not
just adding cases to a switch statement.

Overall risk: both reviewers rate the phase **MEDIUM-to-HIGH**, with Codex calling it flatly
**HIGH** ("Execution should be blocked until shared-key repair, pipeline decomposition, archive
semantics, provider normalization, unused-key representation, and error propagation are
revised") and xai-grok converging on the same blocking set for Plan 05-03 specifically ("Do not
proceed past 05-03 until: (1) repair signer option recorded, (2) rotate does not persist the
new key twice, (3) failed rotate leaves one usable key... and no stray archive").

### Agreed Strengths

- The phase correctly targets the real defect: `realBackend.Persist` only special-cases
  `AddIdentity`/`Reset` today, and every other action (including delete, rotate, new-key)
  silently falls through to `tuikit.Reduce`, the same pure in-memory reducer the dummy fixture
  backend uses (`cmd/gitid/wiring.go:352-361`; both reviewers cite this independently).
- Git-only delete is a real, currently-missing behavior change: `Delete` today unconditionally
  removes the SSH block (`internal/identity/delete.go:59/64-69`) and the allowed-signers block
  (`internal/identity/delete.go:90/95-99`) — both reviewers confirm the scope split is required,
  not cosmetic.
- `WriteAllowedSigners` is confirmed replace-only (`internal/keygen/signers.go:42/51-64,57`), so
  the append-aware rewrite both plans require is a genuine bug fix, not gold-plating — CONTEXT
  D-07's "rotate appends, old line kept" requirement is not satisfied by the existing writer.
  the append-aware rewrite both plans require is a genuine bug fix, not gold-plating.
- Reusing the existing create wizard for Clone (05-05) is the right architecture — both
  reviewers agree it avoids a second mutation pipeline and the existing `specFingerprint`
  already carries most of what clone-detection needs (`cmd/gitid/wiring.go:1735-1742`).
- The Phase-2 approved design has exactly one key-generation menu row
  (`.planning/design/identity-manager/FIELDS.md:71`), so routing new-key vs. rotate vs. repair
  behind that one entry point is confirmed as the correct interpretation of the design, not an
  invented screen.
- 05-09's fail-closed gate design (planted-vs-zero scan hits, negative controls for stale/
  missing/mutated entries, compiled-binary PTY over HTML) is architecturally sound to both
  reviewers, with the caveat below about the shared renderer.

### Agreed Concerns

- **HIGH (both) — `runPipeline` already persists the generated key before rotate's own archive
  and re-point steps run.** `internal/identity/identity.go:527-549` shows `runPipeline` calls
  `PersistKey` then `WriteAllowedSigners` internally. Both reviewers independently conclude that
  if Plan 05-03's Rotate calls `runPipeline` unchanged after archiving, the new key is persisted
  a second time with archive/rollback ordering undefined. Both recommend the same fix direction:
  decompose `runPipeline` into explicit phases (validate/test, persist, write artifacts,
  re-test) so rotate/repair can select only the phases they need, rather than parameterizing it
  with more booleans.
- **HIGH (Codex) / MEDIUM (xai-grok) — Archive semantics conflict between Plan 05-02 and Plan
  05-04.** Codex identifies this as a hard contradiction: `ArchiveKeyPair` is specified as
  move-and-remove-source in 05-02, but 05-04's delete-everything flow needs copy-then-remove-
  live-last so a mid-failure leaves both the archive copy and the live key intact. xai-grok
  frames the same fact ("Archive-then-remove last: on failure, live key + archive copy both
  exist") as an accepted design point rather than a blocking contradiction, but agrees the two
  plans need one consistent primitive, not two divergent expectations of the same function.
  Resolve by splitting into a copy primitive and a move primitive (or an explicit copy-then-
  journaled-remove orchestration), as Codex recommends.
- **HIGH (Codex) / HIGH (xai-grok, differently scoped) — The doctor-reservation regression test
  and the exhaustive-Persist acceptance criteria are not provably correct against current code.**
  Codex: the doctor regression is vacuous because current key enumeration uses the non-recursive
  glob `~/.ssh/id_*` (`internal/identity/inventory.go:264`), so a key under
  `~/.ssh/gitid-archive/` is never returned even without the proposed `IsReservedPath` guard —
  the required red-test-without-the-guard step cannot be observed. Separately, Codex flags that
  "reflect over every type implementing Action" cannot be implemented in Go (interface
  implementers cannot be enumerated via reflection); xai-grok's independent HIGH on the same
  plan (05-07) is that the exhaustive `Persist` switch specifically omits `ConfigureGit`
  (`internal/tuikit/store.go:109-119`, a real action with its own `CommitGit`), which is exactly
  the kind of silent omission an explicit registry (Codex's recommendation) would catch and a
  hand-written switch will not.
- **HIGH (Codex) / MEDIUM (xai-grok) — Provider foreign-reference counting compares the wrong
  values.** Recipe hosts resolve through `Hostname ssh.github.com` on port 443
  (`recipes/README.md:15`), while `insteadOf` rewrite blocks are keyed on the bare provider
  (`github.com`, `internal/gitconfig/renderer.go:154/174`). Both reviewers independently find
  that Plan 05-04's `ProviderRefCount`, if it compares a foreign Host's `Hostname` directly
  against the provider key, will under-count references and can leave a destructive rewrite
  removal (or an under-protective ref-count) in place for hosts that use the recipe's canonical
  alt-SSH endpoint. Both recommend reusing the existing `rewriteLookupProvider`-style
  normalization (`internal/identity/loader.go:124`) rather than a new comparison.
- **MEDIUM (both) — Shared-key repair is under-specified for what happens to the sibling.**
  Codex states this as a HIGH standalone contradiction (repairing at "the canonical path"
  necessarily overwrites a shared key's bytes while the plan also requires the sibling's bytes
  to remain unchanged); xai-grok frames the same fact as the reason the blocking human
  checkpoint on repair-signer semantics is the right one-way door, and recommends allocating the
  repaired identity a new, non-shared key path rather than reusing the shared one. Both agree
  the current plan text does not yet resolve which of these two irreconcilable requirements
  wins.

### Divergent Views

- **05-03 blocking checkpoint (DLV-08 conflict).** Codex raises a MEDIUM concern that a new
  human checkpoint for the repair-signer decision may violate the project's single-checkpoint
  autonomous-loop contract (DLV-08, `.planning/REQUIREMENTS.md:77`), since design approval is
  already recorded in Phase 2. xai-grok does not flag this as a process conflict at all —
  it treats the blocking checkpoint as simply "the right one-way door" for a destructive,
  hard-to-reverse decision. This is worth resolving explicitly: either fold the repair-signer
  decision into Phase 2's already-closed checkpoint (by deciding it now, in this review cycle,
  rather than deferring to execution), or explicitly document why this one decision earns a
  second checkpoint.
- **05-06 Backend interface error channels.** Codex raises a standalone HIGH: `DeletePlan` and
  `KeyCeremonyPlan` returning bare view structs (no error return) means a read/parse failure
  during destructive-action planning is indistinguishable from a legitimately empty plan, risking
  confirming an incomplete target list as "nothing to delete." xai-grok does not surface this
  concern in its 05-06 section at all — it focuses instead on backend-surface growth and
  dummy/real drift. This looks like a real gap in xai-grok's coverage rather than a genuine
  disagreement: the underlying code fact (`DeletePlanView`/`KeyCeremonyView` as bare structs) is
  not disputed by either reviewer where it is discussed, only raised as a top-level concern by
  one of them.
- **05-08 CLI/TUI parity chokepoint.** Codex rates this **HIGH** ("Direct CLI calls to
  transaction functions do not by themselves prove the same complete chokepoint... the plan must
  identify one orchestration function containing the entire lifecycle") and separately flags
  incomplete dry-run semantics for delete as a second HIGH. xai-grok reviews the same plan and
  rates it **MEDIUM**, focused on scope/timeout risk (matrix vs. reserved `health`/`fix` nouns,
  the 600s e2e budget) rather than the chokepoint-proof concern. Codex's chokepoint point and
  xai-grok's "shared lifecycle services... parameterized by confirmation and dry-run policy"
  suggestion (05-08 section) are actually the same underlying recommendation restated at
  different severities — treat this as one HIGH concern to resolve in planning, not two
  independent MEDIUMs.
- **05-09 shared-renderer blind spot severity.** Both reviewers note that the real-vs-dummy PTY
  comparison cannot catch a render/layout defect present in both binaries because they share
  `internal/tuikit`'s render stack. Codex frames this as MEDIUM risk to "appearance parity"
  guarantees; xai-grok frames it more sharply as meaning the gate "won't catch `identities.go`
  fabrication if dummy also fabricates until 05-07 lands **and** dummy fixtures stay dummy" —
  effectively a sequencing dependency (05-07 must land before 05-09 closes) rather than a
  standalone design flaw. Both agree the mitigation is the same: keep independent manifest/state
  assertions that do not rely solely on the real-vs-dummy diff.

---

## Codex Review

reflected model: `gpt-5.6-sol (reasoning=low)`, resolved from banner (instance `codex-sol`'s
pinned model `openai/gpt-5.6-sol-fast` was rejected by this account — see the configured-reviewer
note above).

# Cross-AI Plan Review — Phase 5 Identity Manager

## Overall Summary

The nine-plan sequence is thoughtfully decomposed and unusually strong on test intent, rollback, CLI/TUI parity, and config safety. It also correctly preserves much of the recipe contract: generated SSH blocks contain explicit aliases, alt-SSH endpoints, port 443, `IdentityFile`, and `IdentitiesOnly yes` ([internal/sshconfig/renderer.go:15](internal/sshconfig/renderer.go#L15)); `includeIf` supports both `gitdir:` and `hasconfig:` ([internal/gitconfig/renderer.go:15](internal/gitconfig/renderer.go#L15)); provider rewriting uses the recipe-shaped `insteadOf` form ([internal/gitconfig/renderer.go:167](internal/gitconfig/renderer.go#L167)).

However, the plans are not execution-ready. Four load-bearing contradictions need replanning:

1. Repairing an identity that shares its key cannot both write a new key to the same canonical path and leave the sibling's key bytes unchanged.
2. Rotation as described would persist the new key twice unless `runPipeline` is split or refactored.
3. The proposed archive helper moves keys, while delete-everything later treats it as a non-destructive copy primitive.
4. The doctor-reservation regression test is vacuous because current key discovery never descends into the archive directory.

Overall risk: **HIGH** until these are resolved.

---

## Plan 05-01 — Tracer, Read Surface, Command Tree

### Summary

A good architectural tracer that directly targets the real-backend fallthrough defect. The filesystem restart assertion is the right proof that a TUI mutation was persisted rather than merely reduced in memory.

### Strengths

- Correctly identifies the current silent fallthrough: only `Reset` and `AddIdentity` are handled explicitly; all other actions call the dummy reducer ([cmd/gitid/wiring.go:344](cmd/gitid/wiring.go#L344), [cmd/gitid/wiring.go:352](cmd/gitid/wiring.go#L352)).
- Git-only deletion correctly needs to change current behavior because `Delete` presently removes the SSH block and allowed-signers block unconditionally ([internal/identity/delete.go:59](internal/identity/delete.go#L59), [internal/identity/delete.go:90](internal/identity/delete.go#L90)).
- The richer JSON health schema is grounded in the actual two-axis model, which exposes `IdentityState`, `KeyState`, and `Problems` separately ([internal/identity/state.go:100](internal/identity/state.go#L100)).
- Re-reading through `BuildInventory` respects MGR-08 because it reconstructs from SSH/Git artifacts each invocation ([internal/identity/inventory.go:46](internal/identity/inventory.go#L46)).

### Concerns

- **HIGH — The read surface cannot represent every taxonomy row as designed.** `Inventory.Identities` contains only reconstructed accounts, while unreferenced on-disk keys are returned separately as `UnusedKeys` ([internal/identity/inventory.go:14](internal/identity/inventory.go#L14), [internal/identity/inventory.go:90](internal/identity/inventory.go#L90)). Joining `Inventory.Identities` to `b.accounts()` by identity name cannot create standalone `key-unused` rows. This conflicts with the phase goal's "each identity/key" taxonomy and later eight-state list tests.
- **MEDIUM — The proposed no-op delete contract is underspecified.** `filewriter.RemoveBlock` may return unchanged bytes, but the plan still calls writers and removal seams. Tests should distinguish "no error" from "no backup/no filesystem mutation."
- **MEDIUM — Flat wrapper delegation can mishandle Cobra state.** Calling another command's `RunE` directly does not automatically reproduce its flag parsing, `Args`, `PreRunE`, context, or usage behavior. The wrapper mechanism needs a shared handler/configuration function rather than delegation to the child command object.
- **LOW — Completion support includes PowerShell in Cobra's current root documentation, while the requirement only names bash/zsh/fish** ([cmd/gitid/main.go:67](cmd/gitid/main.go#L67)). This is harmless but should be documented rather than silently omitted.

### Suggestions

- Define whether unused keys are first-class `identityRecord` rows or a separate JSON collection. If first-class, introduce an explicit record type for inventory-only key entries.
- Make Git-only delete idempotency assert zero writes and zero backups on the second run.
- Build noun and flat commands from one shared verb specification/handler rather than having one Cobra command invoke another's `RunE`.
- Add a recipe-shape regression to the tracer: surviving SSH block must still contain `IdentitiesOnly yes`, the original `Hostname`, and port.

### Risk Assessment

**MEDIUM-HIGH.** The tracer is sound, but the read model cannot currently satisfy the full state taxonomy it claims to expose.

---

## Plan 05-02 — Archive, Allowed Signers, Provider Rewrite Removal

### Summary

The package placement is good, and the plan correctly separates low-level primitives from lifecycle orchestration. The doctor-reservation verification, however, does not actually exercise the proposed exclusion.

### Strengths

- Correctly avoids using `BackupAndRemove` for the dedicated archive convention.
- Append-aware signer writing addresses a real data-integrity issue: the existing writer replaces the entire identity block ([internal/keygen/signers.go:42](internal/keygen/signers.go#L42), [internal/keygen/signers.go:57](internal/keygen/signers.go#L57)).
- Routing appended lines through `AllowedSignersLine` preserves the comma-injection guard ([internal/keygen/signers.go:23](internal/keygen/signers.go#L23)).
- `RemoveProviderRewrite` is correctly distinguished from the unrelated global baseline remover. Provider blocks are named `provider-rewrite:<host>` ([internal/gitconfig/renderer.go:154](internal/gitconfig/renderer.go#L154)), while `RemoveURLRewritesBlock` operates on another artifact.

### Concerns

- **HIGH — The doctor regression test is vacuous.** Current enumeration uses the non-recursive glob `~/.ssh/id_*` ([internal/identity/inventory.go:264](internal/identity/inventory.go#L264)). A key under `~/.ssh/gitid-archive/` is never returned, even without the proposed `IsReservedPath` guard. Therefore the requirement to temporarily remove the guard and observe a red test cannot be satisfied.
- **HIGH — `ArchiveKeyPair` is defined as move-and-remove, but Plan 05-04 later uses it as copy-before-delete.** Plan 05-02 explicitly removes source files after copying. Plan 05-04 expects a failure after the copy to leave the live key intact. One primitive cannot provide both contracts without a mode or separate functions.
- **MEDIUM — Reserved-path matching covers only direct children.** The proposed registry handles the archive directory and files directly inside it. If later pruning groups archives by identity or generation, nested paths become visible to doctor. This should be an explicit invariant.
- **MEDIUM — `os.Chmod` on an existing archive directory changes user-owned metadata.** That may be appropriate for a gitid-owned directory, but the behavior statement only discusses an already-correct 0700 directory. Tests should cover an existing unsafe directory and rollback/error behavior.

### Suggestions

- Split the primitive into `CopyKeyPairToArchive` and `MoveKeyPairToArchive`, or make the orchestration explicitly perform copy followed by a separately journaled remove.
- Test doctor reservation through `IsReservedPath` directly and through a recursive/future-proof key enumerator. Do not claim the current non-recursive production path proves the guard.
- Add symlink rejection for archive directory, sources, and destinations.
- Pin archive filename parsing and collision behavior for both ed25519 and RSA filenames.

### Risk Assessment

**HIGH.** The archive semantic mismatch affects both rotation and deletion, and the mandatory doctor proof is presently non-causal.

---

## Plan 05-03 — Rotate, Repair, State Router

### Summary

The distinction between retirement and repair is conceptually strong, but the proposed implementation contradicts the existing write pipeline and cannot safely repair shared-key identities.

### Strengths

- Correctly recognizes that existing `Rotate` is only a skeleton: it simply generates and calls `runPipeline` ([internal/identity/modes.go:170](internal/identity/modes.go#L170)).
- The append requirement is necessary because `runPipeline` currently calls the replacing signer seam ([internal/identity/identity.go:541](internal/identity/identity.go#L541), [internal/identity/identity.go:549](internal/identity/identity.go#L549)).
- Carrying provider and archive facts in `RotateResult` keeps UI-free domain ownership.
- A pure state router fits the existing `IdentityHealth` model ([internal/identity/state.go:115](internal/identity/state.go#L115)).

### Concerns

- **HIGH — Shared-key repair is internally impossible as specified.** `Account.KeyPath` is the path referenced by the identity ([internal/identity/identity.go:26](internal/identity/identity.go#L26)). `runPipeline` persists generated bytes at `StagedKey.FinalPrivatePath` ([internal/identity/identity.go:527](internal/identity/identity.go#L527)). If two identities share that path, repairing one at the "canonical path" overwrites the sibling's key bytes. The plan simultaneously requires the sibling's key file bytes to remain unchanged.
- **HIGH — Rotation would persist the replacement twice.** The plan says archive, then persist the new pair, then run the four-writer re-point. But `runPipeline` itself persists the staged key before those writers ([internal/identity/identity.go:527](internal/identity/identity.go#L527)). A selector for only the signer writer does not fix this.
- **HIGH — Repair cannot "never read or touch existing key material" while also writing to the same existing shared path.** Avoiding archive/removal is insufficient; overwriting is itself destructive.
- **MEDIUM — The blocking checkpoint violates the project's single-checkpoint delivery contract.** DLV-08 says the autonomous loop has one human stop ([.planning/REQUIREMENTS.md:77](.planning/REQUIREMENTS.md#L77)), and design approval is already recorded in Phase 2 ([.planning/ROADMAP.md:148](.planning/ROADMAP.md#L148)). This ambiguity should be resolved before plan execution.
- **MEDIUM — Routing shared-key state is not directly represented by `IdentityHealth`.** The current key axis reports use kind, not owner count ([internal/identity/state.go:185](internal/identity/state.go#L185)). A router taking only `IdentityHealth` cannot distinguish one owner from several.

### Suggestions

- Refactor `runPipeline` into explicit phases: validate/test, persist key, write artifacts, resolved test. Let rotate and repair select the required phases without double persistence.
- For shared-key repair, allocate a new identity-specific path and update only the repaired identity's SSH and signing references. Do not reuse the shared path.
- Extend routing input with owner-count/shared-key facts; do not infer sharing from `StateKeyUsedBoth`.
- Resolve signer semantics before execution and remove the extra human checkpoint.

### Risk Assessment

**HIGH.** As written, repair can corrupt a sibling identity and rotation can double-write key material.

---

## Plan 05-04 — Delete Everything, Ref-counting, Scan, DeletePlan

### Summary

The pure preview is an excellent direction, but provider counting and key archival require redesign before implementation.

### Strengths

- A pure `DeletePlan` shared by TUI confirm and CLI dry-run is the correct mechanism to prevent preview/write drift.
- Returning all shared-key owners fixes the current lossy map, which stores only one label per key path ([cmd/gitid/wiring.go:1587](cmd/gitid/wiring.go#L1587), [cmd/gitid/wiring.go:1604](cmd/gitid/wiring.go#L1604)).
- Whole-token matching is appropriate for unmanaged-reference scanning.
- Git-only preserving SSH, key, signer history, and provider rewrite is consistent with the safer delete choice.

### Concerns

- **HIGH — Archive semantics conflict with Plan 05-02.** `ArchiveKeyPair` removes its sources, but this plan requires "copy first; remove live key last" and expects certain failures to leave both archive copies and live keys. Separate copy and removal operations are required.
- **HIGH — The foreign-provider reference predicate compares incompatible concepts.** Canonical identities use aliases such as `personal.github.com` but resolve through `Hostname ssh.github.com` on port 443 ([recipes/README.md:15](recipes/README.md#L15), [internal/identity/identity.go:231](internal/identity/identity.go#L231)). Counting a foreign stanza only when its resolved `Hostname` equals provider `github.com` misses the canonical `ssh.github.com` endpoint.
- **MEDIUM — `ProviderRefCount` based on `Account.Provider` needs normalization.** Reconstruction may store short provider values such as `github`, while rewrite lookup needs `github.com`; current code already contains a dedicated normalization helper because these differ ([internal/identity/loader.go:68](internal/identity/loader.go#L68), [internal/identity/loader.go:124](internal/identity/loader.go#L124)).
- **MEDIUM — Scan-source stripping by managed block name may miss references inside other gitid-managed identities.** D-13 says unmanaged regions, but references in sibling managed blocks can still be operationally significant. The plan should distinguish "own block," "other managed block," and "foreign text."
- **MEDIUM — Target equality is ambiguous.** A provider rewrite, allowed-signers block, and SSH block are regions inside shared files, not distinct files. Comparing "actually modified file set" against logical target strings can pass despite removing the wrong block.

### Suggestions

- Normalize provider identity through the existing `rewriteLookupProvider` behavior or extract an exported domain equivalent.
- Count foreign references using provider endpoint aliases, provider markers, and canonical endpoint mapping—not raw hostname equality.
- Separate logical targets from physical files in `DeletePlan`.
- Define archive copy and live removal as distinct injected effects, both journaled.

### Risk Assessment

**HIGH.** Incorrect provider counting can either break remaining identities or leave destructive rewrite state behind.

---

## Plan 05-05 — Clone

### Summary

Routing clone into the existing create wizard is the correct architectural choice and strongly reduces duplication. The derivation contract needs tighter grounding in the current match representation.

### Strengths

- Reusing the existing wizard avoids a second mutation pipeline.
- Same-key clone fingerprinting is already largely supported: the current fingerprint includes alias, hostname, port, algorithm, name, provider, and reused key path ([cmd/gitid/wiring.go:1735](cmd/gitid/wiring.go#L1735)).
- The two copied author fields versus re-derived identity-scoped fields is clear and testable.
- Using the checked SSH rendering boundary is important because production writers must not call unchecked `RenderHostBlock` ([internal/sshconfig/renderer.go:81](internal/sshconfig/renderer.go#L81)).

### Concerns

- **MEDIUM — "Substitute the new name in every match value" is unsafe.** `Match` is only a kind and an opaque value ([internal/gitconfig/renderer.go:25](internal/gitconfig/renderer.go#L25)). A source value may not contain the identity name, may contain it coincidentally elsewhere, or may be user-customized. The plan needs kind-specific constructors, not textual substitution.
- **MEDIUM — The source account does not retain the parsed fragment signing key.** `Account` contains key paths and author values but no literal fragment `SigningKey` ([internal/identity/identity.go:26](internal/identity/identity.go#L26)). Clone can re-derive the new signing path, but tests must not claim it compared against the source's literal signing configuration unless that value is loaded.
- **MEDIUM — Name availability and Host-pattern collision are different checks.** Case-insensitive identity-name bumping does not prove a derived alias avoids wildcard Host patterns such as `*.github.com`.
- **LOW — `DefaultMatch` currently defaults to `gitdir:`** ([internal/identity/identity.go:276](internal/identity/identity.go#L276)), while the recipe describes `hasconfig:` as primary and `gitdir:` as an alternative ([recipes/README.md:18](recipes/README.md#L18)). This is an intentional current-project divergence, but the plan should surface it explicitly when cloning/re-deriving matches.

### Suggestions

- Rebuild matches from explicit structured clone inputs:
  - `gitdir` from the selected clone directory.
  - `hasconfig` from the derived clone alias.
- Validate derived alias against actual OpenSSH pattern semantics, not merely a taken-name list.
- Add round-trip tests demonstrating recipe-canonical `hasconfig:remote.*.url:git@<clone-alias>:*/**`.
- Document the current gitdir-default divergence from the recipe.

### Risk Assessment

**MEDIUM.** The architecture is sound, but naïve match substitution could generate wrong identity-selection rules.

---

## Plan 05-06 — TUI Surface

### Summary

The rendering plan is careful about frozen geometry, copy, and optimistic updates. The backend DTO methods need error channels before the UI can safely describe destructive actions.

### Strengths

- A distinct `RotateIdentity` action avoids hiding materially different behavior behind a boolean.
- The single visible key action is consistent with the approved design, which contains one `action_new_key` row ([.planning/design/identity-manager/FIELDS.md:71](.planning/design/identity-manager/FIELDS.md#L71)).
- Reducing actions only after typed commit messages directly addresses false success.
- Rendering delete confirmations from a domain plan rather than `DemoIdentity` is a significant improvement.

### Concerns

- **HIGH — `Backend.DeletePlan(name, scope) DeletePlanView` has no error channel.** Domain planning reads several files and can fail. The proposed synchronous signature cannot distinguish an empty valid plan from a read/parse failure, risking confirmation of an incomplete target list.
- **MEDIUM — `KeyCeremonyPlan` has the same problem.** Missing accounts, invalid paths, or classification failures become zero-value view data.
- **MEDIUM — Adding five mandatory methods to the central Backend interface creates broad coupling.** Every fixture and test stub must implement unrelated behavior, increasing breakage. Narrow capability interfaces or effect messages would be safer.
- **MEDIUM — The shared-key downgrade note may exceed one line with several long identity names.** The plan mandates comma-joining on one line while also requiring fixed geometry; clipping owner names would hide safety-critical information.
- **LOW — The planned "same beat count" assertion may be too structural.** A ceremony can have the same count but incorrect ordering or controls.

### Suggestions

- Return `(DeletePlanView, error)` and `(KeyCeremonyView, error)`, or make both asynchronous typed messages with explicit errors.
- Allow the shared-owner warning to wrap or use a clipped summary plus a full preview list.
- Test semantic ceremony ordering, not only beat count.
- Consider separate read/planning and mutation backend capability interfaces.

### Risk Assessment

**MEDIUM-HIGH.** Destructive confirmation must fail closed when plan construction fails.

---

## Plan 05-07 — Real Wiring and Honest Detail

### Summary

This plan targets the correct final integration points and correctly removes fabricated detail values. Transaction rollback and action exhaustiveness need more precise mechanisms.

### Strengths

- The current detail view demonstrably fabricates `ssh.github.com`, port 443, and a `.pub` signing key ([internal/tuikit/identities.go:3111](internal/tuikit/identities.go#L3111), [internal/tuikit/identities.go:3133](internal/tuikit/identities.go#L3133)). Removing those defaults directly satisfies MGR-03.
- The mutation journal already snapshots both content and mode ([cmd/gitid/wiring.go:879](cmd/gitid/wiring.go#L879)) and restores them through `WriteNoBackup` ([cmd/gitid/wiring.go:1053](cmd/gitid/wiring.go#L1053)).
- Replacing real-backend reducer fallback with a loud error is correct.
- Populating per-identity findings from `IdentityHealth.Problems` uses the actual classification source.

### Concerns

- **HIGH — "Reflect over every type implementing Action" is not implementable as stated.** Go reflection cannot enumerate all concrete types implementing an interface. `Action` is an unexported-method marker interface with concrete implementations declared throughout `store.go` ([internal/tuikit/store.go:99](internal/tuikit/store.go#L99)). The test requires an explicit registry or generated list.
- **HIGH — Archive cleanup cannot rely only on watched files.** Archive filenames are timestamp-generated during the transaction and are not known when initial watches are registered. The journal needs a first-class "created file" record, or the domain result must return archive paths before any later failure.
- **MEDIUM — An exhaustive real `Persist` switch can break currently active demo views.** The action union also includes `ApplySSH`, `ApplyGitBaseline`, `FixFinding`, and `MarkScanned` ([internal/tuikit/store.go:142](internal/tuikit/store.go#L142)). These support the not-yet-real views. If they become errors, the demo behavior promised for Phases 6-8 changes.
- **MEDIUM — Problem severity mapping is new policy.** The current `Problem` model contains no severity ([internal/identity/state.go:76](internal/identity/state.go#L76)). Mapping it in `cmd/gitid` duplicates future doctor policy unless explicitly scoped.
- **LOW — The detail still prints `User git` and `IdentitiesOnly yes` as constants.** Those are canonical generated values, but MGR-03's "only parsed values" language means either they should be carried from parsing or labeled as required policy rather than observed configuration.

### Suggestions

- Add an explicit `AllActionKindsForTest()` registry or generate the exhaustive switch/test from one action manifest.
- Extend `mutationJournal` with `recordCreatedFile(path)` and `recordCreatedDir(path)`.
- Preserve reducer-backed behavior for explicitly demo-only actions while failing on unclassified mutations.
- Put problem-to-severity mapping in the domain or a shared presentation mapper that Phase 8 can reuse.
- Carry parsed `IdentitiesOnly` through the account/view model if detail claims to show observed state.

### Risk Assessment

**HIGH.** Transaction cleanup and action-enumeration acceptance criteria are not yet implementable as written.

---

## Plan 05-08 — CLI Parity

### Summary

The adaptive-depth model and parity matrix are valuable, but the plan substantially underestimates how much lifecycle logic is needed to make headless and prefilled execution genuinely equivalent.

### Strengths

- Centralizing adaptive-depth resolution avoids per-command behavior drift.
- The matrix is requirement-keyed and mechanically checked in both directions.
- `--yes` suppressing confirmation only, not backups, is the correct safety boundary.
- Post-write re-test driving the exit code is a strong correctness property.

### Concerns

- **HIGH — Direct CLI calls to transaction functions do not by themselves prove the same complete chokepoint.** Several transactions are described as write wrappers, while D-02 requires test → preview → confirmation → backup → write → re-test. The plan must identify one orchestration function containing the entire lifecycle, not merely reuse the final write transaction.
- **HIGH — Dry-run semantics are incomplete for delete.** Delete has no connectivity test stage, while the plan says dry-run runs "the test and preview stage" for all verbs. The per-verb contract should be explicit.
- **MEDIUM — TTY detection must consider stdin and stdout separately.** A terminal stdout with piped stdin, or vice versa, can otherwise unexpectedly launch a TUI or prompt.
- **MEDIUM — Byte-equivalence tests normalize the most important mutable outputs.** Generated key bytes and timestamp paths legitimately differ, but after normalizing them the test must still verify referential coherence: each SSH `IdentityFile`, fragment `signingkey`, signer line, and archive path must point to the corresponding generated artifact.
- **MEDIUM — Matrix completeness limited to the identity noun will not enforce future Phase 6-9 updates.** The document says future phases must add rows, but the proposed test only walks `identity`.

### Suggestions

- Introduce shared lifecycle services such as `RunRotate`, `RunRepair`, and `RunDelete`, parameterized by confirmation and dry-run policy. Both CLI and TUI should invoke these.
- Specify dry-run per verb.
- Detect interactive mode from both input and output descriptors.
- In parity e2e, compare normalized topology and references, not just normalized text.
- Design the matrix checker to support every product noun from the outset.

### Risk Assessment

**HIGH.** Outcome parity could appear green while CLI and TUI still execute different ceremony logic.

---

## Plan 05-09 — PTY and Visual Gates

### Summary

The phase-close gate is comprehensive and correctly uses compiled binaries rather than HTML artifacts. Its live-real-versus-dummy comparison has an acknowledged but significant blind spot because both binaries share the renderer.

### Strengths

- Raw keystrokes and post-ceremony filesystem assertions are appropriate DLV-06 evidence.
- The plan explicitly excludes browser/HTML parity, matching the project's post-Phase-2 UI policy.
- Stale-entry rejection, defect failure, missing-state failure, and mutation negative controls make the gate meaningfully fail-closed.
- Synthetic sandbox keys avoid writing real user SSH material.

### Concerns

- **MEDIUM — The paired comparison cannot detect shared-renderer defects.** Both binaries use the same `internal/tuikit` render stack; the current architecture explicitly separates fixture versus real persistence while sharing views ([internal/tuikit/store.go:1](internal/tuikit/store.go#L1)). The gate can catch data/wiring differences but not a layout, label, or interaction defect present in both.
- **MEDIUM — Eight "taxonomy labels" are not necessarily eight identity rows.** As noted for Plan 05-01, `key-unused` is currently global inventory data rather than an account-backed identity ([internal/identity/inventory.go:14](internal/identity/inventory.go#L14)). The seeded-all-eight PTY fixture is impossible until the UI model resolves that distinction.
- **MEDIUM — Exact two-hit assertions can be brittle.** The alias may legitimately occur in several scanned artifacts. The fixture needs exact source control and a clearly defined scan-source list.
- **LOW — Recording literal full command output in summaries risks noise and incidental paths.** A stable terminal success line plus test count/duration is usually better evidence than unrestricted output.
- **LOW — Manual human walk conflicts with `autonomous: true`.** The verification includes a human check even though the plan metadata declares autonomous execution.

### Suggestions

- Retain independent manifest assertions from `FIELDS.md` so shared-renderer defects can still fail without relying on dummy differences.
- Resolve standalone key rows before requiring all eight states in one identity list.
- Make planted scan fixtures deterministic and assert file/line identities, not only count.
- Mark the plan non-autonomous if the manual check remains, or convert it into automated PTY assertions.

### Risk Assessment

**MEDIUM.** The gate is strong for wiring and state, but weaker than its "appearance parity" language suggests.

---

# Cross-Plan Suggestions

Before execution, revise the plan set around these decisions:

1. **Split lifecycle orchestration into phases.** Extract key generation/testing, key persistence, artifact writes, and re-test from `runPipeline`. Rotation and repair cannot safely compose the current all-in-one function.
2. **Define shared-key repair correctly.** A repaired identity must receive a new distinct key path; overwriting the shared path violates D-05.
3. **Separate archive copy from archive move.** Rotation needs move semantics; delete-everything needs copy-then-later-remove semantics.
4. **Normalize providers once.** Reuse or extract the current `rewriteLookupProvider` mechanism ([internal/identity/loader.go:124](internal/identity/loader.go#L124)) for reference counting.
5. **Resolve the key-unused row model.** Decide whether unused keys appear as manager rows, a separate section, or JSON-only inventory records.
6. **Make destructive plans error-aware.** `DeletePlan` and `KeyCeremonyPlan` must fail closed.
7. **Replace impossible reflection checks with explicit registries.**
8. **Preserve and explicitly test canonical recipe shape.** The current SSH renderer is aligned with aliases, alt-SSH, port, and `IdentitiesOnly` ([internal/sshconfig/renderer.go:15](internal/sshconfig/renderer.go#L15)). The current default match is `gitdir:` ([internal/identity/identity.go:276](internal/identity/identity.go#L276)), while the recipe describes `hasconfig:` as primary ([recipes/README.md:18](recipes/README.md#L18)); keep this as an explicit accepted project divergence rather than letting clone or manager behavior obscure it.

# Final Risk Assessment

**Overall risk: HIGH.**

The plans are comprehensive and security-conscious, but several core mechanisms cannot satisfy their own acceptance criteria against the current source. Execution should be blocked until shared-key repair, pipeline decomposition, archive semantics, provider normalization, unused-key representation, and error propagation are revised.

---

## OpenCode Review (xai-grok)

I'll verify plan claims against the repo, recipes, and the referenced source files.

# Phase 5 plan review (plans 05-01 … 05-09)

Verified against `recipes/README.md`, `internal/identity/delete.go`, `cmd/gitid/wiring.go`, `internal/tuikit/store.go` / `identities.go`, `internal/keygen/signers.go`, `internal/gitconfig/renderer.go`, `internal/sshconfig/include.go`, `cmd/gitid/main.go`, `internal/identity/modes.go` / `identity.go`.

**Recipe binding:** Canonical shape is alias-per-identity, `IdentitiesOnly yes`, port 443, `includeIf` `hasconfig:`/`gitdir:`, per-provider `insteadOf` (`git@<provider>:`, not alias), ed25519 not gist RSA. Plans keep that structure; clone re-derives match **kind** (hasconfig stays hasconfig). No RSA fallback in these plans.

---

## 05-01 — Tracer: Git-only delete + `--json` + D-01 tree

**Summary.** Right first cut: Pitfall 1 is real (`Persist` default is `tuikit.Reduce` at `cmd/gitid/wiring.go:358-360`). Git-only delete is the right verb. Current `Delete` always `RemoveBlock`s SSH (`delete.go:64-69`) and always `RemoveAllowedSigners` (`delete.go:95-99`) — the scope change is required, not cosmetic.

**Strengths**
- Restart-after-delete PTY assertion actually closes the reducer illusion.
- D-10 keep-signers matches `delete.go` today doing the opposite.
- JSON two-axis schema is a concrete freeze of Open Q2.
- `newRootCmd` really is a blank slate (`main.go:74-86`).

**Concerns**
- **HIGH** — `Delete` still writes SSH even when Git-only unless Task 1's skip is exact; any leftover `WriteSSH` on unchanged bytes still timestamps a "backup" and confuses receipts.
- **MEDIUM** — `--all` stub in 05-01 vs full semantics in 05-04/05-08: TUI can still open delete-choice "everything" while CLI errors — parity hole until 05-08.
- **LOW** — `delete.go:50-53` still cites old D-07 (keepKey); comment rewrite must not confuse CONTEXT D-07 (append signers).

**Suggestions**
- Git-only: skip `ReadSSH`/`WriteSSH` entirely, not "remove empty then write".
- PTY: assert `allowed_signers` **bytes** equal, not only "line still present".

**Risk:** MEDIUM — architecture is sound; the live `Delete` SSH write is the landmine.

---

## 05-02 — Archive / append signers / `RemoveProviderRewrite`

**Summary.** Correctly names three missing primitives. `WriteAllowedSigners` is replace-only (`signers.go:57`). `HasProviderRewrite` exists; no remover (`renderer.go:180-223`). `BackupAndRemove` is sibling `.bak`, not `gitid-archive`. `ReservedPaths` is only `config.d` (`include.go:78-81`).

**Strengths**
- Archive dir as parameter so `keygen` does not import `sshconfig`.
- Doctor regression for orphans is the right bug-class test.
- `RemoveProviderRewrite` vs `RemoveURLRewritesBlock` (`baseline.go:145`) is explicit.

**Concerns**
- **MEDIUM** — `IsReservedPath` today requires `*.config` in `config.d` (`include.go:99`). Archive files are `id_* .stamp` — Task 1's "dir or file whose Dir is archive" must not accidentally reserve random `~/.ssh` paths.
- **LOW** — Public archive mode 0644 vs private 0600 is right; stamp collision across rotate+delete sharing one dir needs exclusive-create (plan has it).

**Suggestions**
- Pin `IsReservedPath` tests: `~/.ssh/config` false, archive file true, `config.d/foo.txt` still false.

**Risk:** LOW — small, isolated, well-scoped.

---

## 05-03 — Rotate vs repair + blocking signer decision

**Summary.** Existing `Rotate` is generate + `runPipeline` (`modes.go:181-189`) with no archive. `runPipeline` always `PersistKey` then `WriteAllowedSigners` (`identity.go:530-549`). Blocking checkpoint on repair signers is the right one-way door.

**Strengths**
- Distinct `RepairKey` vs `Rotate`; `KeyActionFor` behind one FIELDS.md row (`FIELDS.md:71`).
- Failure must restore canonical keys (T-05-11).
- `ProviderHost` from `Account.Provider`, not alias suffix.

**Concerns**
- **HIGH** — `runPipeline` already persists the key. If Rotate archives then calls `runPipeline` unchanged, PersistKey runs **again** and archive/rollback ordering is undefined. Plan must **skip** `PersistKey` inside `runPipeline` for rotate, or not use `runPipeline` for the persist step.
- **MEDIUM** — Rotate twice → three signer lines: `RemoveAllowedSigners` is block-keyed (whole block). Delete-everything after two rotates still OK; Git-only keep-block also OK. Document that.
- **LOW** — `KeyActionFor` vs collapsed `ClassifyState`: table must cover orthogonal axes so `incomplete`+`key-missing` is repair not rotate.

**Suggestions**
- Inject signer writer as an explicit `runPipeline` parameter; never a bool on `Deps`.
- Wiring test: after failed persist, `ssh -G` still resolves the **old** IdentityFile bytes.

**Risk:** HIGH until persist/archive vs `runPipeline` is sequenced in code, not only comments.

---

## 05-04 — Delete-everything semantics + `DeletePlan`

**Summary.** Completes MGR-06. Reusing `gitid-archive` for D-11 is a justified planner resolution. `keyOwners()` collapsing to one label (`wiring.go:1594-1605`) is a real D-12 bug if reused.

**Strengths**
- Managed + foreign Host ref-count; Git-only never drops rewrite (alias still needs `insteadOf`).
- Pure `PlanDelete` / shared target helper.
- Token-boundary scan vs `strings.Contains`.
- Recipe `insteadOf` is provider-keyed (`renderer.go:174`); ref-count on `Hostname`/`Provider` matches.

**Concerns**
- **MEDIUM** — Foreign Host count on `Hostname` matching provider: recipe Hosts use `Hostname ssh.github.com` while provider rewrite key is `github.com`. A naive equality misses hand-written aliases. Need the same host→provider mapping `WriteProviderRewrite` uses.
- **MEDIUM** — Archive-then-remove last: on failure, live key + archive copy both exist (plan wants that). Doctor-reserved archive means `--fix` won't delete extras; still two copies until Phase 8.
- **LOW** — Disclaimer constant vs 05-06 copy-freeze: freeze once.

**Suggestions**
- Test: Host `foo.github.com` with `Hostname ssh.github.com` still counts as GitHub rewrite ref.
- Do not scan private key files (plan already excludes).

**Risk:** MEDIUM — rewrite ref-count host matching is the silent https-clone breaker.

---

## 05-05 — Clone as pre-filled create

**Summary.** D-15 (no second write path) is the correct anti-pattern. `specFingerprint` already includes `Name` and `Alias` (`wiring.go:1741-1742`), so D-16 fingerprint work is mostly prove-don't-invent.

**Strengths**
- Copy only `user.name`/`user.email`; re-derive gitdir/hasconfig/signingkey.
- Taken list unions hand-written Hosts (T-05-23).
- Leaves dummy `CloneIdentity` Reduce intact (`store.go:121-126`).

**Concerns**
- **MEDIUM** — Recipe primary match is `hasconfig:remote.*.url:git@<alias>:*/**` (`recipes/README.md:22-24`). If source is gitdir-only, preserving kind is right; defaulting clone to gitdir-only would diverge from recipe for new-key clones of incomplete sources.
- **LOW** — Review-flag copy must be allowlisted in 05-09 or DLV-04 fails.

**Suggestions**
- Assert derived alias still `DefaultAlias(name, provider)` (`personal.github.com` shape).
- Prefill must not skip test phase (plan Task 3 — keep it).

**Risk:** LOW-MEDIUM.

---

## 05-06 — tuikit menu, delete chrome, key ceremony

**Summary.** Pitfall 3 confirmed: no `RotateIdentity` (`store.go:136-140` only `NewKey`). Action key `a` is in `FIELDS.md:22`. Fabricated delete targets in UI is the Phase-4 class of bug; feeding `DeletePlanView` is right.

**Strengths**
- No boolean on `NewKey`.
- Copy-freeze registration.
- Grace hint only on rotate + ReachableNotUploaded.
- Maximal-fixture frame-height tests.

**Concerns**
- **MEDIUM** — Backend grows many methods (`CommitRotate`, `DeletePlan`, …) before 05-07 implements them; fixture stubs must not look "done" in dummy vs real pairing (05-09 allowlist).
- **LOW** — Direct `c`/`d` shortcuts plus menu: two entry points must share handlers (plan says so; tests should prove both).

**Risk:** MEDIUM (shared package surface, dummy/real drift).

---

## 05-07 — Real wiring, exhaustive Persist, honest detail

**Summary.** This is the phase's load-bearing plan. Detail **does** fabricate hostname/port (`identities.go:3113-3120`) and signingkey from `KeyPath+".pub"` (`identities.go:3137`) — MGR-03 work is real. `DemoBanner` already false for Identities (`wiring.go:340-342`).

**Strengths**
- Journal restore of modes + removing failed-rotate archive copies.
- Reflection over Action union vs silent `Reduce`.
- `SigningKeyPath` from fragment, not derived.

**Concerns**
- **HIGH** — Exhaustive Persist: `ConfigureGit` is a real Action (`store.go:109-119`) with its own `CommitGit`. Missing it in the switch turns a working Git edit into a persist error. Enumerate from `store.go` today, not from memory.
- **MEDIUM** — Rollback vs archive: if journal restore writes old key bytes **and** archive copy remains, doctor-reserved dir hides leftovers; tests must assert archive empty after failed rotate.
- **LOW** — Eight taxonomy fixture in unit tests vs PTY in 05-09; keep labels verbatim (`FIELDS.md:36`).

**Suggestions**
- Generate the Persist case list from the same `isAction` types the dummy implements.

**Risk:** MEDIUM-HIGH (rollback/archive + Persist exhaustiveness).

---

## 05-08 — CLI parity + matrix + headless e2e

**Summary.** SHELL-03 is greenfield (`main.go:69-73`). Adaptive depth + one chokepoint matches D-02. Matrix makes ROADMAP criterion 4 checkable.

**Strengths**
- `--yes` does not skip backups; dry-run byte-identity tests.
- Exit code from re-test; ReachableNotUploaded = success.
- Headless delete prints plan (T-05-38).

**Concerns**
- **MEDIUM** — D-04 lists `health`/`fix` as outcomes; 05-01 reserved nouns error "later phase". Matrix must not claim those commands exist yet, or Task 2 "every identity verb named" vs "every product outcome" fights 05-01.
- **MEDIUM** — Paired CLI/TUI e2e for five verbs + PTY 05-09 may blow 600s; plan allows raising timeout — do it **before** flakes, with a recorded budget.
- **LOW** — Flag names are discretion; freeze in matrix immediately.

**Suggestions**
- Matrix rows for health/fix: `deferred Phase 8` with command `gitid health` reserved, not required to resolve to a writer.

**Risk:** MEDIUM (scope/time, matrix vs reserved nouns).

---

## 05-09 — DLV-04/06 gates

**Summary.** Matches delivery policy: dummy is the only parity reference; no HTML. Per-state PTY + filesystem after ceremonies is the right close.

**Strengths**
- Planted vs zero D-13 hits as backstop.
- Allowlist format copied from git-screen.
- Negative controls (stale, missing, mutation).
- Human-check walk at the end.

**Concerns**
- **MEDIUM** — Shared-renderer limitation (Phase 4) means dummy and real can share a render bug; gate won't catch `identities.go` fabrication if dummy also fabricates until 05-07 lands **and** dummy fixtures stay dummy. Order 05-07 before 05-09 is correct; allowlist must not hide remaining fabrication.
- **LOW** — Captures vs OpenSSH headers: good T-05-43 check.

**Risk:** MEDIUM (suite size/flake), LOW on design.

---

## Cross-plan / phase-level

| Goal | Coverage |
|------|----------|
| MGR-01/08 list from disk | 05-01 JSON + 05-07 taxonomy fixture + 05-09 PTY |
| MGR-03 SSH-first, no fake Git | 05-07 removes `ssh.github.com`/`443` defaults (`identities.go:3114-3119`) |
| MGR-04 clone | 05-05 wizard; 05-08 CLI |
| MGR-05/KEY-07 vs KEY-05 | 05-03 router + 05-06 one menu row |
| MGR-06 delete choice | 05-01 git-only; 05-04 everything; 05-08 `--all` |
| SHELL-02 five views | 05-07; other tabs stay D-16 banner |
| DLV-04/06 | 05-09 |

**Phase risk: MEDIUM**, bordering **HIGH** only on 05-03 `runPipeline`/`PersistKey` double-write and 05-04 provider Hostname vs `github.com` rewrite key.

**Do not proceed past 05-03 until:** (1) repair signer option recorded, (2) rotate does not persist the new key twice, (3) failed rotate leaves one usable key at the canonical path and no stray archive for that generation.
