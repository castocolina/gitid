// Package identity provides the domain model for gitid: the Account type
// (name, host alias, provider, email, key path, match strategy) and CRUD
// operations. It reconstructs identities by parsing managed blocks from
// ~/.ssh/config and ~/.gitconfig via the sshconfig and gitconfig packages.
// The filesystem is the source of truth; this package is the translation layer.
//
// Implementation lands in a later phase (Phase 2+).
package identity
