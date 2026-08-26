# gitid Agent Instructions

`gitid` safely manages coordinated SSH and Git identities. The canonical
configuration outcome is defined by [`recipes/`](./recipes/), not by invented
config shapes.

## Required Start

1. Read `recipes/README.md`, `recipes/ssh-config.recipe`, and
   `recipes/gitconfig.recipe` before planning or implementation.
2. For milestone work, read `.planning/ONESHOT.md`, `.planning/STATE.md`,
   `.planning/ROADMAP.md`, and `.planning/LEARNINGS.md` before each phase.
3. State the hypothesis, run an observable check, then implement test-first.
   Resolve only genuine ambiguity with the user; continue routine work
   autonomously.

## Non-Negotiable Rules

- Generate code, documentation, tests, comments, and commit messages in English.
- Keep core logic UI-free and use TDD. Record the command and result for every
  behavior change.
- Never write real `~/.ssh/*`, `~/.gitconfig*`, keys, or external accounts
  without the product confirmation, timestamped backup, and re-verification.
- Commit coherent implementation, tests, and documentation together. Never use
  `--no-verify`.
- Use a GSD workflow before repository edits: `/gsd-quick`, `/gsd-debug`, or
  `/gsd-execute-phase` as appropriate.

## UI Reference

Phase 2's approved Bubble Tea mockup (`cmd/gitid-dummy`) is the sole UI/UX
reference for Phases 3–10. Compare the real compiled TUI with that live mockup
through real PTY interaction. HTML/MUI artifacts are Phase-2 design history and
are not a later-phase parity target. Every UI difference must be classified as
an improvement or a defect; unclassified differences fail automated review.

## Code Exploration

Use the `codegraph_explore` MCP tool before any Grep/Read loop ("how does X
work," "where is X defined," "what calls X"). Run `codegraph index || codegraph
init -i` once at task start to refresh the index. Fall back to `rg` (not
`grep`) + Read only if codegraph is unavailable or insufficient.

## Commands

- `make test`
- `make lint`
- `make test-e2e`
- `make gate-visual-regression`

## Detailed Guidance

- [Product and UI policy](docs/agent-instructions/product-ui.md)
- [Milestone workflow](docs/agent-instructions/workflow.md)
- [Engineering and commits](docs/agent-instructions/engineering.md)
- [Configuration and stack](docs/agent-instructions/configuration-stack.md)
