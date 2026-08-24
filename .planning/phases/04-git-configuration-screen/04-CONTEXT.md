# Phase 4: Git Configuration Screen - Context

**Gathered:** 2026-08-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Wire the real backend behind the approved post-SSH Git screen. It collects the
per-identity fragment fields, previews the selected `includeIf` strategy,
reviews the fragment, parent Git config, and `allowed_signers` together, then
confirms one safe write ceremony.

In scope: `~/.gitconfig.d/<identity>`, gitid-owned `~/.gitconfig` `includeIf`
step. Phase 4 removes the real binary's "Git configuration arrives with the
next build" disabled state.

Out of scope: manager list/detail and CLI surfaces (Phase 5), shared Git
defaults other than the per-provider URL rewrite below (Phase 7), Git health or
repair (Phase 8), key upload (Phase 9), and real configuration or account
mutation during planning and tests.

</domain>

<decisions>
## Implementation Decisions

### Git artifacts and matching
- **D-01:** The fragment contains `user.name`, `user.email`, fixed
  `gpg.format=ssh`, a public-key path in `user.signingkey`, and
  `commit.gpgsign`; its target is `~/.gitconfig.d/<identity>`.
- **D-02:** `gitdir` is the default match strategy; `hasconfig` and `both` are
  available. The default Git directory is editable and derived as
  `~/git/<identity>/`; trailing slashes are retained.
- **D-03:** The `hasconfig` condition is SSH-only:
  `hasconfig:remote.*.url:git@<ssh-host>:*/**`. Do not add the recipe's HTTPS
  variant. `both` writes the two conditions as an OR.
- **D-04:** `allowed_signers` uses the exact `user.email` bytes as its
  principal. An edit replaces that identity's managed signer block rather than
  appending a stale-email line.

### URL rewriting and reuse
- **D-05:** Phase 4 writes the recipe-shaped `insteadOf` rewrite, not Phase 7.
  It is one idempotent managed block per provider:
  `[url "git@<provider>:"] insteadOf = https://<provider>/`.
- **D-06:** The form offers an opt-out "Force SSH over HTTPS" checkbox, checked
  by default. Opting out never removes a rewrite already managed for another
  identity.
- **D-07:** Use one reusable `internal/tuikit` Git flow for create and edit.
  SSH-only aliases resume it in create mode; complete identities use parsed
  values in edit mode. Phase 5 reuses this flow rather than duplicating it.

### Ceremony and safety
- **D-08:** The wizard's final review remains one combined ceremony: SSH host
  block, fragment, `includeIf` plus optional rewrite, `allowed_signers`, and a
  Git-directory creation note when needed. Skip Git remains SSH-only.
- **D-09:** The standalone/create-edit flow retains the approved seven Git
  states. Edit review uses a true `-/+` diff only for changed lines; create
  review shows plain resulting text.
- **D-10:** Confirmed writes are all-or-nothing. Back up each pre-existing
  target before mutation; on failure restore changed files in reverse order and
  remove files or directories created by the transaction. Report the exact
  failed target and restoration result.
- **D-11:** Register every new non-identity Git managed block, including the
  per-provider URL rewrite, in the doctor reserved-block registry in the same
  change. It must never be reconstructed as an identity or deleted as an
  orphan.

### TUI-only UX policy
- **D-12:** `cmd/gitid-dummy` is the sole Phase 4 UI/UX reference. Preserve
  its seven named Git-screen states, field order, labels, controls, defaults,
  and four-beat mutation ceremony. HTML/MUI artifacts are design history only.
  Exercise the compiled real binary at 100x30 through a real PTY; classify
  every dummy/real difference as an improvement or defect. Unclassified
  differences fail the UI gate.

### Agent discretion
- Exact copy for the rewrite toggle, Git-directory row, collision-resume
  affordance, and preview headings, subject to the existing copy-freeze gate.
- Transaction implementation details, preview line limits, and the precise
  reserved-block name, provided they meet D-01 through D-12 and reuse the
  established safe-write chokepoint.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Product and recipes
- `recipes/README.md` - canonical wiring and the ed25519-over-RSA caveat.
- `recipes/gitconfig.recipe` - fragment, `includeIf`, and `insteadOf` shapes.
- `recipes/ssh-config.recipe` - alias model used by the SSH `hasconfig` match.
- `.planning/ROADMAP.md` - Phase 4 goal and success criteria.
- `.planning/REQUIREMENTS.md` - GITUI-01 through GITUI-05 and DLV policy.

### Approved TUI contract
- `.planning/design/APPROVAL.md` - approved Phase 2 design record.
- `.planning/design/git-screen/FIELDS.md` - binding seven Git-screen states.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md` -
  theme, copy-freeze, keyboard precedence, and row budget.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-DESIGN-DECISIONS-CHECKPOINT-2.md` -
  binding field, radio, mouse, and affordance rules.
- `.planning/phases/04-git-configuration-screen/04-UI-SPEC.md` - scoped UI
  divergences and fixed-geometry implementation contract.

### Phase handoff and safety
- `.planning/phases/03-create-flow-backend/03-CONTEXT.md` - functional Skip
  Git, the disabled Git-step reason Phase 4 removes, and visual-gate policy.
- `docs/agent-instructions/configuration-stack.md` - managed-block and write
  safety rules.
- `docs/agent-instructions/product-ui.md` - TUI-only reference and acceptance
  policy.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable assets
- `internal/tuikit/identities.go` already renders the shared Git form, live
  fragment/includeIf previews, and the wizard review entry point.
- `internal/tuikit/ceremony.go` provides the async confirm, backup receipt,
  retryable failure, bounded exact-text preview, and cancel-first ceremony.
- `internal/gitconfig/fragment.go`, `renderer.go`, and `reader.go` provide
  fragment writes, managed `includeIf` rendering, and edit-mode reconstruction.
- `internal/keygen/signers.go` writes an identity-keyed managed signer block
  whose principal derives byte-identically from email.
- `internal/filewriter/` provides sentinel-only, atomic, idempotent block
  replacement while preserving foreign content.

### Integration points
- `cmd/gitid/wiring.go` contains the real backend's Git paths, current
  create-write plan, disabled Git-step seam, and transactional SSH precedent.
- `internal/gitconfig/reader.go` and `internal/doctor/checks/` are the
  reserved-block boundary that must recognize the URL-rewrite block.
- `e2e/create_flow_pty_e2e_test.go` is the real-binary PTY and isolated-HOME
  precedent; extend it rather than relying on fixture-only tests.

</code_context>

<specifics>
## Recipes Alignment

The implementation follows recipe structure, not its historical RSA examples:
per-identity fragment plus conditional include, provider-level HTTPS-to-SSH
rewrite, SSH alias-aware `hasconfig`, and SSH signing through
`gpg.format=ssh` plus `allowed_signers`. The selected ed25519 public-key path
is configuration data; private key material is never rendered or written by
this phase.

## Safety Boundaries

Planning, unit tests, and PTY tests use an isolated temporary `HOME`. No task
in this phase may touch real `~/.ssh/*`, `~/.gitconfig*`, keys, or external
accounts without the required explicit confirmation, timestamped backup, and
post-write verification. No external-account action belongs to Phase 4.

</specifics>

<deferred>
## Deferred Ideas

- Identity-manager launch, list, and detail UI: Phase 5.
- Global Git defaults beyond the per-provider rewrite: Phase 7.
- Git health checks and repairs: Phase 8.
- HTTPS `hasconfig` variants: excluded while the default rewrite is enabled;
  revisit only if product scope changes.

</deferred>

---

*Phase: 04-git-configuration-screen*
*Context gathered: 2026-08-24*
