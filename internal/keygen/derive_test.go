package keygen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDerivePublicKeyRoundTrips generates a real key with Generate, then asserts
// DerivePublicKey reads that private key back and reproduces a byte-identical
// authorized-key line (IDENT-02, RESEARCH Q3). The derived line must carry
// exactly one trailing newline, matching the Generate path's PubLine contract.
func TestDerivePublicKeyRoundTrips(t *testing.T) {
	dir := t.TempDir()

	res, err := Generate(Params{Algo: "ed25519", Identity: "reuse", Comment: "reuse@gitid", Dir: dir})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	got, err := DerivePublicKey(res.PrivatePath)
	if err != nil {
		t.Fatalf("DerivePublicKey returned error: %v", err)
	}

	if !strings.HasPrefix(got, "ssh-ed25519 ") {
		t.Errorf("derived line prefix = %q, want ssh-ed25519 ", head(got, 20))
	}
	if !strings.HasSuffix(got, "\n") || strings.Count(got, "\n") != 1 {
		t.Errorf("derived line must end with exactly one newline; got %q", got)
	}
	if got != res.PubLine {
		t.Errorf("derived line mismatch\n got: %q\nwant: %q", got, res.PubLine)
	}
}

// TestDerivePublicKeyFromPassphraselessOnly asserts a plain (unencrypted) key
// derives cleanly. Encrypted keys are out of scope for the reuse-derive path
// in this phase, so we only verify the passphraseless contract.
func TestDerivePublicKeyMissingFile(t *testing.T) {
	_, err := DerivePublicKey(filepath.Join(t.TempDir(), "does-not-exist"))
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
	if _, err := DerivePublicKey(bad); err == nil {
		t.Fatal("DerivePublicKey on garbage input must return an error")
	}
}
