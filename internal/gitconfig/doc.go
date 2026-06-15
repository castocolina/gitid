// Package gitconfig parses ~/.gitconfig for gitid-managed blocks and renders
// includeIf stanzas and per-identity fragment files. It uses a custom line-by-line
// parser (no existing Go library supports includeIf write-back) and delegates
// file writes to the filewriter package.
//
// Implementation lands in a later phase (Phase 2+).
package gitconfig
