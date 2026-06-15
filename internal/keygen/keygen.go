package keygen

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// Params configures a key generation request.
type Params struct {
	// Algo is the key algorithm; only "ed25519" is supported in this phase.
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

// GenerateMaterial generates an ed25519 key pair in memory and returns the
// Material (PrivPEM + PubLine) WITHOUT writing anything to disk. It is the
// pure in-memory key generation function; the caller is responsible for staging
// and persisting the key bytes at the appropriate time (BUG-4 temp-then-promote
// flow). Only "ed25519" is supported; other algorithms return an
// unsupported-algorithm error.
func GenerateMaterial(p Params) (Material, error) {
	if p.Algo != "ed25519" {
		return Material{}, fmt.Errorf("keygen: unsupported algorithm %q (only ed25519)", p.Algo)
	}

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
	pubLine := ssh.MarshalAuthorizedKey(sshPub) // ends with a single '\n'

	return Material{
		PrivPEM: privPEM,
		PubLine: string(pubLine),
	}, nil
}

// KeyPaths returns the D-06 convention private-key and public-key paths for
// the given dir, algo, and identity: <dir>/id_<algo>_<identity> and its .pub
// sibling. No filesystem access is performed.
func KeyPaths(dir, algo, identity string) (privPath, pubPath string) {
	privPath = filepath.Join(dir, fmt.Sprintf("id_%s_%s", algo, identity))
	return privPath, privPath + ".pub"
}
