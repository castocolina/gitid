package keygen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const samplePubLine = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyDataHere work@gitid\n"

// TestAllowedSignersLine asserts the SIGN-01 format: the email is byte-identical
// to the input, namespaces="git" is mandatory, the trailing newline from
// MarshalAuthorizedKey is stripped, and exactly one newline terminates the line.
func TestAllowedSignersLine(t *testing.T) {
	email := "me@example.com"
	got := AllowedSignersLine(email, samplePubLine)

	wantPrefix := email + ` namespaces="git" ssh-ed25519 `
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("AllowedSignersLine prefix = %q, want %q", head(got, len(wantPrefix)), wantPrefix)
	}
	if strings.Count(got, "\n") != 1 || !strings.HasSuffix(got, "\n") {
		t.Errorf("AllowedSignersLine must end with exactly one newline; got %q", got)
	}
	if !strings.HasPrefix(got, email+" ") {
		t.Errorf("email not byte-identical at start of line; got %q", got)
	}
}

// TestAllowedSignersLine_StripsTrailingComment asserts the pub line's trailing
// comment (now present on generated keys, e.g. "… work@gitid") never leaks into
// the signer line — the principal there is the email, and only keytype+key follow.
func TestAllowedSignersLine_StripsTrailingComment(t *testing.T) {
	pub := "ssh-ed25519 AAAABASE64KEYDATA work@gitid\n"
	got := AllowedSignersLine("me@example.com", pub)

	want := "me@example.com namespaces=\"git\" ssh-ed25519 AAAABASE64KEYDATA\n"
	if got != want {
		t.Errorf("AllowedSignersLine = %q, want %q", got, want)
	}
	if strings.Contains(got, "work@gitid") {
		t.Errorf("signer line must not carry the pub comment; got %q", got)
	}
}

// TestWriteAllowedSignersCreates asserts WriteAllowedSigners creates a missing
// file at mode 0644 containing the line wrapped in the per-identity managed
// block (SIGN-01, KEY-02).
func TestWriteAllowedSignersCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")
	line := AllowedSignersLine("me@example.com", samplePubLine)

	backup, err := WriteAllowedSigners(path, "work", line)
	if err != nil {
		t.Fatalf("WriteAllowedSigners returned error: %v", err)
	}
	if backup != "" {
		t.Errorf("backupPath should be empty for a new file; got %q", backup)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat allowed_signers: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("allowed_signers mode = %o, want 644", got)
	}

	content := readFile(t, path)
	if !strings.Contains(content, "# BEGIN gitid managed: work") {
		t.Errorf("missing BEGIN sentinel for work; content:\n%s", content)
	}
	if !strings.Contains(content, "# END gitid managed: work") {
		t.Errorf("missing END sentinel for work; content:\n%s", content)
	}
	if !strings.Contains(content, strings.TrimRight(line, "\n")) {
		t.Errorf("managed block missing the signers line; content:\n%s", content)
	}
}

// TestWriteAllowedSignersIdempotent asserts a second write with the same
// identity+line yields byte-identical content (SAFE-02 proof for the fourth
// artifact).
func TestWriteAllowedSignersIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")
	line := AllowedSignersLine("me@example.com", samplePubLine)

	if _, err := WriteAllowedSigners(path, "work", line); err != nil {
		t.Fatalf("first write: %v", err)
	}
	first := readFile(t, path)

	if _, err := WriteAllowedSigners(path, "work", line); err != nil {
		t.Fatalf("second write: %v", err)
	}
	second := readFile(t, path)

	if first != second {
		t.Errorf("WriteAllowedSigners not idempotent; first:\n%s\nsecond:\n%s", first, second)
	}
}

// TestWriteAllowedSignersMultiIdentity asserts a second identity appends a
// distinct managed block while preserving the first block and any foreign
// hand-written lines byte-for-byte (SAFE-02).
func TestWriteAllowedSignersMultiIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")

	foreign := "alice@example.com namespaces=\"git\" ssh-ed25519 AAAAForeignKey alice\n"
	if err := os.WriteFile(path, []byte(foreign), 0o600); err != nil { //nolint:gosec // test fixture seed; WriteAllowedSigners rewrites at 0644
		t.Fatalf("seeding foreign content: %v", err)
	}

	workLine := AllowedSignersLine("work@example.com", samplePubLine)
	if _, err := WriteAllowedSigners(path, "work", workLine); err != nil {
		t.Fatalf("writing work block: %v", err)
	}

	personalLine := AllowedSignersLine("personal@example.com", samplePubLine)
	if _, err := WriteAllowedSigners(path, "personal", personalLine); err != nil {
		t.Fatalf("writing personal block: %v", err)
	}

	content := readFile(t, path)
	if !strings.Contains(content, foreign) {
		t.Errorf("foreign content not preserved; content:\n%s", content)
	}
	if !strings.Contains(content, "# BEGIN gitid managed: work") {
		t.Errorf("work block lost; content:\n%s", content)
	}
	if !strings.Contains(content, "# BEGIN gitid managed: personal") {
		t.Errorf("personal block missing; content:\n%s", content)
	}
}

// TestWriteAllowedSignersBackup asserts a non-empty backupPath is returned when
// the file pre-existed (delegates to filewriter).
func TestWriteAllowedSignersBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")
	if err := os.WriteFile(path, []byte("preexisting\n"), 0o600); err != nil { //nolint:gosec // test fixture seed
		t.Fatalf("seeding file: %v", err)
	}

	line := AllowedSignersLine("me@example.com", samplePubLine)
	backup, err := WriteAllowedSigners(path, "work", line)
	if err != nil {
		t.Fatalf("WriteAllowedSigners: %v", err)
	}
	if backup == "" {
		t.Errorf("expected a non-empty backupPath when the file pre-existed")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // path is a test-controlled temp file
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
