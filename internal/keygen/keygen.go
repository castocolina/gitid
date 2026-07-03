package keygen

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Params configures a key generation request.
type Params struct {
	// Algo is the key algorithm name, one of the registry's registered keys
	// (see registry.go). "ed25519" and "rsa-4096" generate real key material;
	// "ecdsa-p256", "ed25519-sk", and "ecdsa-sk" are registered as
	// not-yet-implemented and always error. Any other value is an unsupported
	// algorithm error.
	Algo string
	// Identity is the gitid identity name; it forms the key filename
	// id_<algo>_<identity> (D-06).
	Identity string
	// Comment is embedded in the OpenSSH private key (e.g. "<identity>@gitid").
	Comment string
	// Passphrase, when non-empty, encrypts the serialized private key.
	Passphrase string
	// Dir is the directory the key pair is written to when using Generate
	// (normally ~/.ssh). It is a trusted, gitid-managed path supplied
	// in-process. Unused by GenerateMaterial.
	Dir string
}

// Material holds the in-memory result of GenerateMaterial: the private key as
// an OpenSSH PEM block and the authorized-key line for the public key.
// PrivPEM is private key material and must never be logged or printed.
type Material struct {
	// PrivPEM is the OpenSSH private key serialized to a PEM block (type
	// "OPENSSH PRIVATE KEY"). It is an in-memory only value — never written to
	// disk by GenerateMaterial itself.
	PrivPEM []byte
	// PubLine is the authorized-key line ("ssh-ed25519 AAAA…\n") for the public
	// key, always ending with a single '\n'.
	PubLine string
}

// GenerateMaterial generates a key pair in memory and returns the Material
// (PrivPEM + PubLine) WITHOUT writing anything to disk. It is the pure
// in-memory key generation function; the caller is responsible for staging
// and persisting the key bytes at the appropriate time (BUG-4
// temp-then-promote flow).
//
// Dispatch is a name-keyed lookup into the package-level registry
// (registry.go): p.Algo not present in the registry returns a clear
// unsupported-algorithm error (no panic); an algorithm present but backed by
// a notYetImplemented stub returns a named "not yet implemented" error and a
// zero-value Material — registry presence never implies generation support
// (T-01-21).
func GenerateMaterial(p Params) (Material, error) {
	gen, ok := registry[p.Algo]
	if !ok {
		return Material{}, fmt.Errorf("keygen: unsupported algorithm %q", p.Algo)
	}
	return gen(p)
}

// generateEd25519 generates an ed25519 key pair in memory. Extracted
// verbatim from the pre-registry GenerateMaterial body — behavior is
// unchanged (default algorithm, KEY-02).
func generateEd25519(p Params) (Material, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Material{}, fmt.Errorf("keygen: generating ed25519 key: %w", err)
	}

	// Pass the value from GenerateKey directly: value works for marshal at
	// x/crypto v0.53.0 (RESEARCH Pitfall 10).
	var block *pem.Block
	if p.Passphrase != "" {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, p.Comment, []byte(p.Passphrase))
	} else {
		block, err = ssh.MarshalPrivateKey(priv, p.Comment)
	}
	if err != nil {
		return Material{}, fmt.Errorf("keygen: serializing private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(block)

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return Material{}, fmt.Errorf("keygen: building public key: %w", err)
	}

	return Material{
		PrivPEM: privPEM,
		PubLine: pubLineWithComment(sshPub, p.Comment),
	}, nil
}

// generateRSA4096 generates a 4096-bit RSA key pair in memory (KEY-02).
//
// CRITICAL: rsa.GenerateKey returns *rsa.PrivateKey (a pointer) — it is
// passed AS THE POINTER to ssh.MarshalPrivateKey(WithPassphrase) and
// &priv.PublicKey to ssh.NewPublicKey, never dereferenced. Unlike
// ed25519.GenerateKey (which returns a value type that happens to satisfy
// the signer interface directly), *rsa.PrivateKey has pointer-receiver
// methods; passing the dereferenced value fails to satisfy
// crypto.PrivateKey (RESEARCH Pitfall 7).
func generateRSA4096(p Params) (Material, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return Material{}, fmt.Errorf("keygen: generating rsa-4096 key: %w", err)
	}

	var block *pem.Block
	if p.Passphrase != "" {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, p.Comment, []byte(p.Passphrase))
	} else {
		block, err = ssh.MarshalPrivateKey(priv, p.Comment)
	}
	if err != nil {
		return Material{}, fmt.Errorf("keygen: serializing private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(block)

	sshPub, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return Material{}, fmt.Errorf("keygen: building public key: %w", err)
	}

	return Material{
		PrivPEM: privPEM,
		PubLine: pubLineWithComment(sshPub, p.Comment),
	}, nil
}

// pubLineWithComment renders the authorized-key line for pub and appends the
// comment as the trailing field ("ssh-ed25519 AAAA… <comment>\n").
// ssh.MarshalAuthorizedKey emits only "<type> <base64>\n" — it never carries a
// comment — so we splice one in to match the OpenSSH authorized_keys format
// "<type> <base64> [comment]". GitHub surfaces this comment as the key's default
// title, restoring the identification convenience RSA keys had via `ssh-keygen -C`.
// An empty comment yields the bare two-field line unchanged.
func pubLineWithComment(pub ssh.PublicKey, comment string) string {
	keyText := strings.TrimRight(string(ssh.MarshalAuthorizedKey(pub)), "\n")
	if comment != "" {
		keyText += " " + comment
	}
	return keyText + "\n"
}

// KeyPaths returns the D-06 convention private-key and public-key paths for
// the given dir, algo, and identity: <dir>/id_<algo>_<identity> and its .pub
// sibling. No filesystem access is performed.
func KeyPaths(dir, algo, identity string) (privPath, pubPath string) {
	privPath = filepath.Join(dir, fmt.Sprintf("id_%s_%s", algo, identity))
	return privPath, privPath + ".pub"
}
