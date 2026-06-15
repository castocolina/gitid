// Package keygen generates ed25519 SSH key pairs for use with gitid identities.
// It writes the private key to ~/.ssh/<name> with mode 0600 and the public key
// to ~/.ssh/<name>.pub with mode 0644. It is a thin wrapper over crypto/ed25519
// and golang.org/x/crypto/ssh for OpenSSH serialization.
//
// Implementation lands in a later phase (Phase 2+).
package keygen
