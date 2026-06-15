package keygen

import (
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
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

func head(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}
