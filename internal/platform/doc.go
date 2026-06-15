// Package platform detects the operating system (darwin or linux) and provides
// platform-specific hints for gitid: the UseKeychain guard for SSH config on
// macOS, OpenSSH key-algorithm selection, and per-OS install guidance.
// It has no third-party dependencies.
//
// Algorithm selection follows the deliberate fallback chain ed25519 -> rsa-4096
// -> ecdsa (D-09): ProbeKeyTypes runs `ssh -Q key` (NOT `ssh-keygen -Q key`,
// which is KRL-query mode) and SelectAlgorithm performs membership tests over
// the parsed tokens. When none of the chain is available, callers receive
// per-OS install guidance via InstallHint rather than an opaque failure (D-14).
package platform
