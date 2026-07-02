// Package sshconfig parses ~/.ssh/config for gitid-managed blocks and renders
// Account values into SSH Host stanzas. It wraps github.com/kevinburke/ssh_config
// for comment-preserving round-trips and delegates actual file writes to the
// filewriter package.
//
// Implementation lands in a later phase (Phase 2+).
package sshconfig
