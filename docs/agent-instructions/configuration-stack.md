# Configuration And Stack

## Configuration Safety

- Manage only sentinel-delimited blocks owned by gitid.
- Back up real files with a timestamp before confirmed mutation.
- Preserve hand-written content and validate SSH changes by parsing and resolving
  the resulting configuration.
- Use `git config` through `os/exec` for normal Git configuration operations;
  write conditional include blocks as managed text because Git cannot create
  those section headers directly.

## Technology Constraints

- Go is the implementation language.
- Use Bubble Tea, Lip Gloss, and Bubbles v2 imports only.
- Use `golang.org/x/crypto/ssh` for ed25519/OpenSSH serialization.
- Use `github.com/kevinburke/ssh_config` only within gitid-owned SSH blocks.
- Keep browser/HTML capture tooling isolated to Phase-2 design artifacts; it is
  not part of later-phase TUI verification.

## Avoid

- Generic SSH config setters across a whole user file.
- Gitconfig libraries that cannot safely write `includeIf` blocks.
- Bubble Tea and Lip Gloss v1 imports.
- Hard-coded platform-specific clipboard commands.
