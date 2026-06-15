// Package filewriter provides the shared safe-write concern for gitid:
// timestamped backup, render-to-temp, atomic rename via os.Rename,
// correct file-permission setting, and optional restore on error.
// It backs the sshconfig writes and gitconfig's raw managed-block writes
// (includeIf/url sentinel blocks and per-identity fragment files); plain
// gitconfig key/value mutations instead go through `git config`, which owns
// ~/.gitconfig directly.
//
// Implementation lands in a later phase (Phase 2+).
package filewriter
