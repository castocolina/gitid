package keygen

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"path/filepath"

	"golang.org/x/crypto/ssh"

	"github.com/castocolina/gitid/internal/filewriter"
)

const (
	privKeyMode = 0o600
	pubKeyMode  = 0o644
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
	// Dir is the directory the key pair is written to (normally ~/.ssh). It is a
	// trusted, gitid-managed path supplied in-process.
	Dir string
}

// Result reports the paths and public-key line produced by Generate.
type Result struct {
	// PrivatePath is the absolute path of the written private key (mode 0600).
	PrivatePath string
	// PubPath is the absolute path of the written public key (mode 0644).
	PubPath string
	// PubLine is the authorized-key line ("ssh-ed25519 AAAA…\n").
	PubLine string
}

// Generate creates an ed25519 key pair, serializes it to OpenSSH format, and
// writes the private key (0600) and public key (0644) through filewriter
// (IDENT-01, KEY-02). The key filename follows the D-06 convention
// id_<algo>_<identity> with the .pub alongside.
//
// Only the ed25519 algorithm is supported in this phase; other values are
// rejected so callers fail fast rather than silently generating an ed25519 key.
func Generate(p Params) (Result, error) {
	if p.Algo != "ed25519" {
		return Result{}, fmt.Errorf("keygen: unsupported algorithm %q (only ed25519)", p.Algo)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Result{}, fmt.Errorf("keygen: generating ed25519 key: %w", err)
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
		return Result{}, fmt.Errorf("keygen: serializing private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(block)

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return Result{}, fmt.Errorf("keygen: building public key: %w", err)
	}
	pubLine := ssh.MarshalAuthorizedKey(sshPub) // ends with a single '\n'

	privPath := filepath.Join(p.Dir, fmt.Sprintf("id_%s_%s", p.Algo, p.Identity))
	pubPath := privPath + ".pub"

	// All writes go through the filewriter chokepoint (never a direct write).
	if _, err := filewriter.Write(privPath, privPEM, privKeyMode); err != nil {
		return Result{}, fmt.Errorf("keygen: writing private key: %w", err)
	}
	if _, err := filewriter.Write(pubPath, pubLine, pubKeyMode); err != nil {
		return Result{}, fmt.Errorf("keygen: writing public key: %w", err)
	}

	return Result{
		PrivatePath: privPath,
		PubPath:     pubPath,
		PubLine:     string(pubLine),
	}, nil
}
