package keygen

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// DerivePublicKey reads the OpenSSH private key at privateKeyPath and returns its
// authorized-key public line ("ssh-ed25519 AAAA… <comment>\n"), reproducing the
// exact form keygen.GenerateMaterial emits — including the trailing comment — so a
// reused identity gets an identical .pub (IDENT-02, RESEARCH Q3). The OpenSSH
// private key's own comment is not recoverable via x/crypto's public API, so the
// caller supplies the deterministic gitid comment ("<identity>@gitid"); an empty
// comment yields the bare two-field line. The caller writes the returned line 0644
// via filewriter when the existing `<key>.pub` is absent.
//
// The private key body is parsed in-memory only and never returned or logged
// (T-02-28): only the derived public line leaves this function. privateKeyPath is
// a gitid-managed path the user pointed at for reuse.
//
// Only passphraseless keys are supported on this path; an encrypted key surfaces
// the underlying ssh.ParsePrivateKey error so the caller can prompt instead of
// failing silently.
func DerivePublicKey(privateKeyPath, comment string) (string, error) {
	privBytes, err := os.ReadFile(privateKeyPath) //nolint:gosec // privateKeyPath is a gitid-managed path the user selected for reuse
	if err != nil {
		return "", fmt.Errorf("keygen: reading private key %s: %w", privateKeyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(privBytes)
	if err != nil {
		return "", fmt.Errorf("keygen: parsing private key %s: %w", privateKeyPath, err)
	}

	// Reproduce the Generate path's PubLine contract: "<type> <base64> <comment>\n"
	// with exactly one trailing newline.
	return pubLineWithComment(signer.PublicKey(), comment), nil
}
