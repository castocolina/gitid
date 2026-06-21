package keygen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDerivePublicKeyRoundTrips generates a real key with GenerateMaterial,
// writes the private key to a temp file via os.WriteFile (test-controlled path),
// then asserts DerivePublicKey reads that private key back and reproduces an
// authorized-key line with the expected prefix and newline contract (IDENT-02,
// RESEARCH Q3). The derived line must carry exactly one trailing newline,
// matching the GenerateMaterial path's PubLine contract.
func TestDerivePublicKeyRoundTrips(t *testing.T) {
	dir := t.TempDir()

	mat, err := GenerateMaterial(Params{Algo: "ed25519", Identity: "reuse", Comment: "reuse@gitid"})
	if err != nil {
		t.Fatalf("GenerateMaterial returned error: %v", err)
	}

	privPath := filepath.Join(dir, "id_ed25519_reuse")
	if err := os.WriteFile(privPath, mat.PrivPEM, 0o600); err != nil { //nolint:gosec // test-controlled temp path
		t.Fatalf("writing private key fixture: %v", err)
	}

	got, err := DerivePublicKey(privPath, "reuse@gitid")
	if err != nil {
		t.Fatalf("DerivePublicKey returned error: %v", err)
	}

	if !strings.HasPrefix(got, "ssh-ed25519 ") {
		t.Errorf("derived line prefix = %q, want ssh-ed25519 ", head(got, 20))
	}
	if !strings.HasSuffix(got, "\n") || strings.Count(got, "\n") != 1 {
		t.Errorf("derived line must end with exactly one newline; got %q", got)
	}
	if got != mat.PubLine {
		t.Errorf("derived line mismatch\n got: %q\nwant: %q", got, mat.PubLine)
	}
}

// TestDerivePublicKeyMissingFile asserts DerivePublicKey returns an error for a
// non-existent key path.
func TestDerivePublicKeyMissingFile(t *testing.T) {
	_, err := DerivePublicKey(filepath.Join(t.TempDir(), "does-not-exist"), "x@gitid")
	if err == nil {
		t.Fatal("DerivePublicKey on a missing file must return an error")
	}
}

// TestDerivePublicKeyRejectsGarbage asserts non-key bytes produce a parse error
// rather than a panic or empty success.
func TestDerivePublicKeyRejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "not-a-key")
	if err := os.WriteFile(bad, []byte("this is not a private key\n"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	if _, err := DerivePublicKey(bad, "x@gitid"); err == nil {
		t.Fatal("DerivePublicKey on garbage input must return an error")
	}
}
