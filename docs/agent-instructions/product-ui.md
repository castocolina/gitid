# Product And UI Policy

## Product Outcome

`gitid` safely manages `~/.ssh/config`, `~/.gitconfig`, per-identity Git
fragments, `~/.ssh/allowed_signers`, and one ed25519 key per identity. Match the
recipes' structure: identity aliases, port-443 alternative SSH, `IdentitiesOnly
yes`, conditional Git includes, and URL rewriting. Use ed25519 rather than the
recipes' historical RSA examples.

## Phase Boundary

- Phase 2 created and received approval for both HTML/MUI and Bubble Tea design
  artifacts.
- From Phase 3 through Phase 10, `cmd/gitid-dummy` is the only UI/UX reference.
- The real compiled TUI is exercised at fixed terminal geometry through real PTY
  keyboard and mouse interaction.
- HTML/MUI captures remain Phase-2 history. Do not capture, compare, or require
  them for later-phase parity.

## UI Acceptance

- Preserve the mockup's workflow, labels, controls, field order, and usable
  layout.
- Classify every real-TUI difference as either an improvement or a defect.
- Fix every defect. Record improvements for the single user review after all
  milestone phases complete.
- Do not require byte-identical PNGs, pixel parity, or historical artifact
  parity when semantic interaction and usable layout are proven.
