package keygen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// TestGenerateEd25519 asserts that Generate produces a valid OpenSSH private
// key (0600) and authorized public-key line (0644) at the D-06 path
// id_<algo>_<identity>, satisfying IDENT-01 and KEY-02.
func TestGenerateEd25519(t *testing.T) {
	dir := t.TempDir()

	res, err := Generate(Params{
		Algo:     "ed25519",
		Identity: "work",
		Comment:  "work@gitid",
		Dir:      dir,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	wantPriv := filepath.Join(dir, "id_ed25519_work")
	wantPub := wantPriv + ".pub"
	if res.PrivatePath != wantPriv {
		t.Errorf("PrivatePath = %q, want %q", res.PrivatePath, wantPriv)
	}
	if res.PubPath != wantPub {
		t.Errorf("PubPath = %q, want %q", res.PubPath, wantPub)
	}

	privBytes, err := os.ReadFile(wantPriv) //nolint:gosec // path is a test-controlled temp file
	if err != nil {
		t.Fatalf("reading private key: %v", err)
	}
	if !strings.HasPrefix(string(privBytes), "-----BEGIN OPENSSH PRIVATE KEY-----") {
		t.Errorf("private key does not start with OpenSSH PEM header; got prefix %q", head(string(privBytes), 40))
	}
	if _, err := ssh.ParsePrivateKey(privBytes); err != nil {
		t.Errorf("private key does not parse as an OpenSSH key: %v", err)
	}

	pubBytes, err := os.ReadFile(wantPub) //nolint:gosec // path is a test-controlled temp file
	if err != nil {
		t.Fatalf("reading public key: %v", err)
	}
	if !strings.HasPrefix(string(pubBytes), "ssh-ed25519 ") {
		t.Errorf(".pub does not start with %q; got prefix %q", "ssh-ed25519 ", head(string(pubBytes), 20))
	}

	if !strings.HasPrefix(res.PubLine, "ssh-ed25519 ") {
		t.Errorf("PubLine prefix = %q, want ssh-ed25519 ", head(res.PubLine, 20))
	}
	if !strings.HasSuffix(res.PubLine, "\n") || strings.Count(res.PubLine, "\n") != 1 {
		t.Errorf("PubLine must end with exactly one newline; got %q", res.PubLine)
	}
}

// TestGenerateModes asserts the private key is 0600 and the .pub is 0644 (KEY-02).
func TestGenerateModes(t *testing.T) {
	dir := t.TempDir()

	res, err := Generate(Params{Algo: "ed25519", Identity: "modes", Comment: "modes@gitid", Dir: dir})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	privInfo, err := os.Stat(res.PrivatePath)
	if err != nil {
		t.Fatalf("stat private key: %v", err)
	}
	if got := privInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("private key mode = %o, want 600", got)
	}

	pubInfo, err := os.Stat(res.PubPath)
	if err != nil {
		t.Fatalf("stat public key: %v", err)
	}
	if got := pubInfo.Mode().Perm(); got != 0o644 {
		t.Errorf("public key mode = %o, want 644", got)
	}
}

// TestGenerateWithPassphrase asserts that a non-empty passphrase still yields a
// serializable OpenSSH private key (encrypted form) with the PEM header present.
func TestGenerateWithPassphrase(t *testing.T) {
	dir := t.TempDir()

	res, err := Generate(Params{
		Algo:       "ed25519",
		Identity:   "secured",
		Comment:    "secured@gitid",
		Passphrase: "correct horse battery staple",
		Dir:        dir,
	})
	if err != nil {
		t.Fatalf("Generate with passphrase returned error: %v", err)
	}

	privBytes, err := os.ReadFile(res.PrivatePath)
	if err != nil {
		t.Fatalf("reading private key: %v", err)
	}
	if !strings.HasPrefix(string(privBytes), "-----BEGIN OPENSSH PRIVATE KEY-----") {
		t.Errorf("encrypted private key missing OpenSSH PEM header; got prefix %q", head(string(privBytes), 40))
	}
}

func head(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}
