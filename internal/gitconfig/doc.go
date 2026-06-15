// Package gitconfig manages ~/.gitconfig for gitid. It reads and writes plain
// key/value settings through `git config` (os/exec) — git is the authoritative
// parser of its own format — and writes the includeIf/url section headers that
// `git config` cannot create natively as sentinel-delimited managed blocks,
// alongside the per-identity fragment files. Backup-before-write and atomic
// writes for those raw managed-block and fragment files are delegated to the
// filewriter package.
//
// WriteFragment sets the per-identity fragment keys (user.name, user.email,
// gpg.format=ssh, user.signingkey as a .pub PATH, commit.gpgsign) via
// `git config --file`, rejecting any [remote] section (Pitfall 9). RenderIncludeIf
// / WriteIncludeIf build and idempotently install the includeIf managed block
// (gitdir with mandatory trailing slash, and/or hasconfig). SetAllowedSignersFile
// wires the global gpg.ssh.allowedSignersFile for SSH-signed-commit verification.
package gitconfig
