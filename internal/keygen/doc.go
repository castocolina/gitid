// Package keygen generates ed25519 SSH key pairs for use with gitid identities
// and builds the git SSH-signing allowed_signers artifact.
//
// Generate writes the private key to <dir>/id_<algo>_<identity> with mode 0600
// and the public key to the same path with a ".pub" suffix at mode 0644 (D-06,
// KEY-02). It is a thin wrapper over crypto/ed25519 and golang.org/x/crypto/ssh
// for OpenSSH serialization; it never hand-rolls the key format.
//
// AllowedSignersLine composes the git SSH-signing line
// `<email> namespaces="git" ssh-ed25519 AAAA…` (SIGN-01), and WriteAllowedSigners
// persists it into ~/.ssh/allowed_signers (mode 0644) inside an idempotent
// per-identity managed block so re-runs and multiple identities never duplicate
// signing lines (SAFE-02).
//
// All file writes are delegated to internal/filewriter (backup, atomic
// temp→rename, explicit chmod); this package never calls os.WriteFile directly.
package keygen
