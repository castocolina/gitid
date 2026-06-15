// Package gitconfig manages ~/.gitconfig for gitid. It reads and writes plain
// key/value settings through `git config` (os/exec) — git is the authoritative
// parser of its own format — and writes the includeIf/url section headers that
// `git config` cannot create natively as sentinel-delimited managed blocks,
// alongside the per-identity fragment files. Backup-before-write and atomic
// writes for those raw managed-block and fragment files are delegated to the
// filewriter package.
//
// Implementation lands in a later phase (Phase 2+).
package gitconfig
