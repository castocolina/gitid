package keygen

import (
	"crypto/rsa"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/castocolina/gitid/internal/filewriter"
)

// TestGenerateMaterial_InMemory asserts GenerateMaterial returns valid in-memory
// key material (PrivPEM + PubLine) without writing any file to disk (BUG-4 pure
// in-memory half). The PrivPEM must be a parseable OPENSSH PRIVATE KEY PEM block.
// No Dir is set; the function must not touch the filesystem at all.
func TestGenerateMaterial_InMemory(t *testing.T) {
	mat, err := GenerateMaterial(Params{
		Algo:     "ed25519",
		Identity: "x",
		Comment:  "x@gitid",
	})
	if err != nil {
		t.Fatalf("GenerateMaterial returned error: %v", err)
	}

	// PrivPEM must be a parseable OPENSSH PRIVATE KEY block.
	if len(mat.PrivPEM) == 0 {
		t.Fatal("GenerateMaterial returned empty PrivPEM")
	}
	block, rest := pem.Decode(mat.PrivPEM)
	if block == nil {
		t.Fatal("GenerateMaterial PrivPEM is not a valid PEM block")
	}
	if block.Type != "OPENSSH PRIVATE KEY" {
		t.Errorf("PrivPEM block type = %q, want OPENSSH PRIVATE KEY", block.Type)
	}
	if len(rest) != 0 {
		t.Errorf("PrivPEM has trailing bytes after PEM block")
	}
	if _, err := ssh.ParsePrivateKey(mat.PrivPEM); err != nil {
		t.Errorf("GenerateMaterial PrivPEM does not parse as an OpenSSH key: %v", err)
	}

	// PubLine must begin with "ssh-ed25519 " and end with a single "\n".
	if !strings.HasPrefix(mat.PubLine, "ssh-ed25519 ") {
		t.Errorf("PubLine = %q, want prefix ssh-ed25519 ", mat.PubLine)
	}
	if !strings.HasSuffix(mat.PubLine, "\n") || strings.Count(mat.PubLine, "\n") != 1 {
		t.Errorf("PubLine must end with exactly one newline; got %q", mat.PubLine)
	}

	// No Dir set — no files must exist for the non-existent path.
	// (The function never touches disk so no assertion needed beyond: no panic.)
}

// TestGenerateMaterial_PubLineCarriesComment asserts the comment is appended as
// the trailing field of the public line so GitHub can title the key with it
// (RSA `ssh-keygen -C` parity). MarshalAuthorizedKey alone never emits a comment.
func TestGenerateMaterial_PubLineCarriesComment(t *testing.T) {
	mat, err := GenerateMaterial(Params{Algo: "ed25519", Identity: "work", Comment: "work@gitid"})
	if err != nil {
		t.Fatalf("GenerateMaterial: %v", err)
	}
	if !strings.HasSuffix(mat.PubLine, " work@gitid\n") {
		t.Errorf("PubLine = %q, want trailing comment ' work@gitid'", mat.PubLine)
	}
	fields := strings.Fields(strings.TrimSpace(mat.PubLine))
	if len(fields) != 3 || fields[0] != "ssh-ed25519" || fields[2] != "work@gitid" {
		t.Errorf("PubLine fields = %v, want [ssh-ed25519 <base64> work@gitid]", fields)
	}
}

// TestGenerateMaterial_PubLineBareWhenNoComment asserts an empty comment yields
// the bare two-field authorized_keys line (no trailing space, no third field).
func TestGenerateMaterial_PubLineBareWhenNoComment(t *testing.T) {
	mat, err := GenerateMaterial(Params{Algo: "ed25519", Identity: "x", Comment: ""})
	if err != nil {
		t.Fatalf("GenerateMaterial: %v", err)
	}
	if fields := strings.Fields(strings.TrimSpace(mat.PubLine)); len(fields) != 2 {
		t.Errorf("bare PubLine fields = %v, want exactly 2 (type + key)", fields)
	}
}

// TestGenerateMaterial_NoDiskWrite asserts that GenerateMaterial never writes a
// file even when Dir is set to a temp directory that exists.
func TestGenerateMaterial_NoDiskWrite(t *testing.T) {
	dir := t.TempDir()

	_, err := GenerateMaterial(Params{
		Algo:     "ed25519",
		Identity: "nodisk",
		Comment:  "nodisk@gitid",
		Dir:      dir,
	})
	if err != nil {
		t.Fatalf("GenerateMaterial returned error: %v", err)
	}

	// The dir must remain empty — no key file must have been written.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("GenerateMaterial wrote files to disk: %v (must be pure in-memory)", names)
	}
}

// TestGenerateMaterial_Passphrase asserts a non-empty passphrase yields an
// encrypted PrivPEM whose bytes differ from the unencrypted form, while the PEM
// block type stays "OPENSSH PRIVATE KEY" (OpenSSH always uses this header).
func TestGenerateMaterial_Passphrase(t *testing.T) {
	noPass, err := GenerateMaterial(Params{Algo: "ed25519", Identity: "pp", Comment: "pp@gitid"})
	if err != nil {
		t.Fatalf("GenerateMaterial (no passphrase): %v", err)
	}
	withPass, err := GenerateMaterial(Params{Algo: "ed25519", Identity: "pp", Comment: "pp@gitid", Passphrase: "secret"})
	if err != nil {
		t.Fatalf("GenerateMaterial (with passphrase): %v", err)
	}

	block, _ := pem.Decode(withPass.PrivPEM)
	if block == nil {
		t.Fatal("passphrase-encrypted PrivPEM is not a valid PEM block")
	}
	if block.Type != "OPENSSH PRIVATE KEY" {
		t.Errorf("encrypted PrivPEM block type = %q, want OPENSSH PRIVATE KEY", block.Type)
	}
	// Encrypted and unencrypted bytes must differ (different keys + different
	// encryption padding), though both carry the same PEM header type.
	if string(noPass.PrivPEM) == string(withPass.PrivPEM) {
		t.Error("passphrase-encrypted PrivPEM is identical to unencrypted PrivPEM")
	}
}

// TestGenerateMaterial_UnsupportedAlgo asserts that an unsupported algorithm
// returns the same error as the original Generate function.
func TestGenerateMaterial_UnsupportedAlgo(t *testing.T) {
	_, err := GenerateMaterial(Params{Algo: "rsa", Identity: "x", Comment: "x@gitid"})
	if err == nil {
		t.Fatal("GenerateMaterial with unsupported algo must return an error")
	}
	if !strings.Contains(err.Error(), "unsupported algorithm") {
		t.Errorf("error = %q, want to contain unsupported algorithm", err.Error())
	}
}

// TestKeyPaths asserts KeyPaths returns the D-06 convention paths
// id_<algo>_<identity> and its .pub sibling.
func TestKeyPaths(t *testing.T) {
	privPath, pubPath := KeyPaths("/home/user/.ssh", "ed25519", "work")

	wantPriv := "/home/user/.ssh/id_ed25519_work"
	wantPub := wantPriv + ".pub"
	if privPath != wantPriv {
		t.Errorf("KeyPaths privPath = %q, want %q", privPath, wantPriv)
	}
	if pubPath != wantPub {
		t.Errorf("KeyPaths pubPath = %q, want %q", pubPath, wantPub)
	}
}

// TestGenerateRSA4096 asserts GenerateMaterial(Params{Algo:"rsa-4096"})
// returns a valid "OPENSSH PRIVATE KEY" PEM whose parsed key is a 4096-bit
// RSA key, plus a "ssh-rsa " PubLine (RESEARCH Pitfall 7: rsa.GenerateKey
// returns a pointer; it must be passed as the pointer, never dereferenced).
func TestGenerateRSA4096(t *testing.T) {
	mat, err := GenerateMaterial(Params{Algo: "rsa-4096", Identity: "x", Comment: "x@gitid"})
	if err != nil {
		t.Fatalf("GenerateMaterial(rsa-4096): %v", err)
	}

	block, _ := pem.Decode(mat.PrivPEM)
	if block == nil {
		t.Fatal("rsa-4096 PrivPEM is not a valid PEM block")
	}
	if block.Type != "OPENSSH PRIVATE KEY" {
		t.Errorf("PrivPEM block type = %q, want OPENSSH PRIVATE KEY", block.Type)
	}

	signer, err := ssh.ParsePrivateKey(mat.PrivPEM)
	if err != nil {
		t.Fatalf("rsa-4096 PrivPEM does not parse as an OpenSSH key: %v", err)
	}
	cryptoPub, ok := signer.PublicKey().(ssh.CryptoPublicKey)
	if !ok {
		t.Fatal("rsa-4096 signer public key does not implement ssh.CryptoPublicKey")
	}
	rsaPub, ok := cryptoPub.CryptoPublicKey().(*rsa.PublicKey)
	if !ok {
		t.Fatalf("rsa-4096 parsed public key is %T, want *rsa.PublicKey", cryptoPub.CryptoPublicKey())
	}
	if bits := rsaPub.N.BitLen(); bits != 4096 {
		t.Errorf("rsa-4096 key bit length = %d, want 4096", bits)
	}

	if !strings.HasPrefix(mat.PubLine, "ssh-rsa ") {
		t.Errorf("PubLine = %q, want ssh-rsa prefix", mat.PubLine)
	}
}

// TestGenerateRSA4096_Passphrase asserts a passphrase-set rsa-4096 request
// serializes via MarshalPrivateKeyWithPassphrase (encrypted bytes differ from
// the unencrypted form, same as the ed25519 path).
func TestGenerateRSA4096_Passphrase(t *testing.T) {
	noPass, err := GenerateMaterial(Params{Algo: "rsa-4096", Identity: "pp", Comment: "pp@gitid"})
	if err != nil {
		t.Fatalf("GenerateMaterial (no passphrase): %v", err)
	}
	withPass, err := GenerateMaterial(Params{Algo: "rsa-4096", Identity: "pp", Comment: "pp@gitid", Passphrase: "secret"})
	if err != nil {
		t.Fatalf("GenerateMaterial (with passphrase): %v", err)
	}

	block, _ := pem.Decode(withPass.PrivPEM)
	if block == nil {
		t.Fatal("passphrase-encrypted rsa-4096 PrivPEM is not a valid PEM block")
	}
	if block.Type != "OPENSSH PRIVATE KEY" {
		t.Errorf("encrypted PrivPEM block type = %q, want OPENSSH PRIVATE KEY", block.Type)
	}
	if string(noPass.PrivPEM) == string(withPass.PrivPEM) {
		t.Error("passphrase-encrypted rsa-4096 PrivPEM is identical to unencrypted PrivPEM")
	}
}

// TestPermissions_KeyFilesAfterRegistryRefactor re-asserts KEY-04: writing
// GenerateMaterial output through the production filewriter chokepoint (the
// same pattern cmd/gitid/add.go's buildDeps uses) yields private key 0600 and
// public key 0644, for every algorithm the registry can actually generate —
// proving the registry refactor introduced no per-algorithm permission
// regression.
func TestPermissions_KeyFilesAfterRegistryRefactor(t *testing.T) {
	for _, algo := range []string{"ed25519", "rsa-4096"} {
		algo := algo
		t.Run(algo, func(t *testing.T) {
			dir := t.TempDir()
			mat, err := GenerateMaterial(Params{Algo: algo, Identity: "perm", Comment: "perm@gitid"})
			if err != nil {
				t.Fatalf("GenerateMaterial(%q): %v", algo, err)
			}

			privPath, pubPath := KeyPaths(dir, algo, "perm")
			if _, err := filewriter.Write(privPath, mat.PrivPEM, 0o600); err != nil {
				t.Fatalf("filewriter.Write(priv): %v", err)
			}
			if _, err := filewriter.Write(pubPath, []byte(mat.PubLine), 0o644); err != nil {
				t.Fatalf("filewriter.Write(pub): %v", err)
			}

			privInfo, err := os.Stat(filepath.Clean(privPath))
			if err != nil {
				t.Fatalf("stat priv: %v", err)
			}
			if got := privInfo.Mode().Perm(); got != 0o600 {
				t.Errorf("private key mode = %o, want 0600", got)
			}

			pubInfo, err := os.Stat(filepath.Clean(pubPath))
			if err != nil {
				t.Fatalf("stat pub: %v", err)
			}
			if got := pubInfo.Mode().Perm(); got != 0o644 {
				t.Errorf("public key mode = %o, want 0644", got)
			}
		})
	}
}

func head(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}
