package keygen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
)

const samplePubLine = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyDataHere work@gitid\n"

// TestAllowedSignersLine asserts the SIGN-01 format: the email is byte-identical
// to the input, namespaces="git" is mandatory, the trailing newline from
// MarshalAuthorizedKey is stripped, and exactly one newline terminates the line.
func TestAllowedSignersLine(t *testing.T) {
	email := "me@example.com"
	got, err := AllowedSignersLine(email, samplePubLine)
	if err != nil {
		t.Fatalf("AllowedSignersLine returned unexpected error: %v", err)
	}

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

// TestAllowedSignersLine_RejectsCommaPrincipalInjection proves CR-18: OpenSSH's
// allowed_signers format treats the principal field as a COMMA-SEPARATED LIST
// (ssh-keygen(1) "PRINCIPALS"), so an email carrying a bare comma smuggles in
// an attacker-chosen second principal — e.g. "victim@corp.test,*" grants a
// wildcard match verified against real `ssh-keygen -Y verify`. gitForm.valid()
// and validateEmail both accept a comma today, so this is the load-bearing
// write-time gate: it must fail closed rather than emit a multi-principal line.
func TestAllowedSignersLine_RejectsCommaPrincipalInjection(t *testing.T) {
	_, err := AllowedSignersLine("victim@corp.test,*", samplePubLine)
	if err == nil {
		t.Fatal("AllowedSignersLine must reject a comma-containing principal (CR-18), got nil error")
	}
}

// TestAllowedSignersLine_RejectsNewlineLineInjection is the WR-08
// regression: the doc comment calls the comma check "the write-time hard
// gate... independent of whether an upstream form/config validator already
// rejected the comma", but a NEWLINE in the principal was not rejected — it
// injects an entire ADDITIONAL allowed_signers line (an attacker-chosen
// principal + key of the attacker's own choosing), not merely a smuggled
// comma-separated principal.
func TestAllowedSignersLine_RejectsNewlineLineInjection(t *testing.T) {
	for _, email := range []string{
		"victim@corp.test\nevil@attacker.test namespaces=\"git\" ssh-ed25519 AAAA evil",
		"victim@corp.test\revil@attacker.test",
	} {
		if _, err := AllowedSignersLine(email, samplePubLine); err == nil {
			t.Errorf("AllowedSignersLine(%q) must reject a newline/CR line-injection principal (WR-08), got nil error", email)
		}
	}
}

// TestAllowedSignersLine_RejectsSpaceFieldInjection is WR-08's second gap: a
// space (or tab) in the principal silently changes the allowed_signers field
// layout ssh-keygen(1) parses — e.g. embedding a bare "namespaces=..."
// token — rather than being carried as part of a single principal field.
func TestAllowedSignersLine_RejectsSpaceFieldInjection(t *testing.T) {
	for _, email := range []string{
		"victim@corp.test namespaces=\"*\"",
		"victim@corp.test\tnamespaces=\"*\"",
	} {
		if _, err := AllowedSignersLine(email, samplePubLine); err == nil {
			t.Errorf("AllowedSignersLine(%q) must reject a space/tab field-injection principal (WR-08), got nil error", email)
		}
	}
}

// TestAllowedSignersLine_RequiresBareAddress is WR-08's self-sufficiency
// proof: the gate must not rely on an upstream validator having already
// confirmed the value looks like an address — an empty principal or one
// missing "@" must be rejected here too, independently.
func TestAllowedSignersLine_RequiresBareAddress(t *testing.T) {
	for _, email := range []string{"", "not-an-address"} {
		if _, err := AllowedSignersLine(email, samplePubLine); err == nil {
			t.Errorf("AllowedSignersLine(%q) must reject a non-address principal (WR-08), got nil error", email)
		}
	}
}

// TestAllowedSignersLine_StripsTrailingComment asserts the pub line's trailing
// comment (now present on generated keys, e.g. "… work@gitid") never leaks into
// the signer line — the principal there is the email, and only keytype+key follow.
func TestAllowedSignersLine_StripsTrailingComment(t *testing.T) {
	pub := "ssh-ed25519 AAAABASE64KEYDATA work@gitid\n"
	got, err := AllowedSignersLine("me@example.com", pub)
	if err != nil {
		t.Fatalf("AllowedSignersLine returned unexpected error: %v", err)
	}

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
	line, lerr := AllowedSignersLine("me@example.com", samplePubLine)
	if lerr != nil {
		t.Fatalf("AllowedSignersLine returned unexpected error: %v", lerr)
	}

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
	line, lerr := AllowedSignersLine("me@example.com", samplePubLine)
	if lerr != nil {
		t.Fatalf("AllowedSignersLine returned unexpected error: %v", lerr)
	}

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

	workLine, werr := AllowedSignersLine("work@example.com", samplePubLine)
	if werr != nil {
		t.Fatalf("AllowedSignersLine returned unexpected error: %v", werr)
	}
	if _, err := WriteAllowedSigners(path, "work", workLine); err != nil {
		t.Fatalf("writing work block: %v", err)
	}

	personalLine, perr := AllowedSignersLine("personal@example.com", samplePubLine)
	if perr != nil {
		t.Fatalf("AllowedSignersLine returned unexpected error: %v", perr)
	}
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

	line, lerr := AllowedSignersLine("me@example.com", samplePubLine)
	if lerr != nil {
		t.Fatalf("AllowedSignersLine returned unexpected error: %v", lerr)
	}
	backup, err := WriteAllowedSigners(path, "work", line)
	if err != nil {
		t.Fatalf("WriteAllowedSigners: %v", err)
	}
	if backup == "" {
		t.Errorf("expected a non-empty backupPath when the file pre-existed")
	}
}

// pubLine builds a distinguishable public-key authorized-key line for
// AppendAllowedSigners tests, using key text unique to the given label so
// two appended lines for the same identity are visibly distinct.
func pubLine(label string) string {
	return "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI" + label + "KeyDataHere " + label + "@gitid\n"
}

// TestAppendAllowedSigners_CreatesBlockWhenAbsent proves appending to an
// identity with no existing block creates the block containing exactly the
// new line (D-07).
func TestAppendAllowedSigners_CreatesBlockWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")

	backup, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("First"))
	if err != nil {
		t.Fatalf("AppendAllowedSigners: %v", err)
	}
	if backup != "" {
		t.Errorf("backupPath should be empty for a new file; got %q", backup)
	}

	content := readFile(t, path)
	if !strings.Contains(content, "# BEGIN gitid managed: work") {
		t.Errorf("missing BEGIN sentinel; content:\n%s", content)
	}
	if strings.Count(content, "namespaces=\"git\"") != 1 {
		t.Errorf("expected exactly 1 signer line, got content:\n%s", content)
	}
}

// TestAppendAllowedSigners_AppendsSecondLine proves a second append yields a
// block with TWO lines, oldest first, both preserved verbatim (D-07: never
// replace, always append).
func TestAppendAllowedSigners_AppendsSecondLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")

	if _, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("First")); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if _, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("Second")); err != nil {
		t.Fatalf("second append: %v", err)
	}

	content := readFile(t, path)
	firstIdx := strings.Index(content, "FirstKeyDataHere")
	secondIdx := strings.Index(content, "SecondKeyDataHere")
	if firstIdx == -1 || secondIdx == -1 {
		t.Fatalf("expected both signer lines present; content:\n%s", content)
	}
	if firstIdx > secondIdx {
		t.Errorf("expected the first-appended line before the second (oldest first); content:\n%s", content)
	}
	if strings.Count(content, "namespaces=\"git\"") != 2 {
		t.Errorf("expected exactly 2 signer lines, got content:\n%s", content)
	}
}

// TestAppendAllowedSigners_IdempotentSameLine proves appending an
// already-present line is a byte-identical no-op with an empty backup path.
func TestAppendAllowedSigners_IdempotentSameLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")

	if _, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("First")); err != nil {
		t.Fatalf("first append: %v", err)
	}
	before := readFile(t, path)

	backup, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("First"))
	if err != nil {
		t.Fatalf("re-append of the same line: %v", err)
	}
	if backup != "" {
		t.Errorf("expected an empty backup path for an idempotent re-append; got %q", backup)
	}
	after := readFile(t, path)
	if before != after {
		t.Errorf("re-appending an already-present line changed the file;\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestAppendAllowedSigners_PreservesForeignContent proves foreign content
// outside the identity's block and another identity's own block survive
// byte-for-byte across two appends.
func TestAppendAllowedSigners_PreservesForeignContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")

	foreign := "alice@example.com namespaces=\"git\" ssh-ed25519 AAAAForeignKey alice\n"
	if err := os.WriteFile(path, []byte(foreign), 0o600); err != nil { //nolint:gosec // test fixture seed; AppendAllowedSigners rewrites at 0644
		t.Fatalf("seeding foreign content: %v", err)
	}
	if _, err := WriteAllowedSigners(path, "personal", mustLine(t, "personal@example.com", pubLine("Personal"))); err != nil {
		t.Fatalf("seeding personal identity block: %v", err)
	}

	if _, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("First")); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if _, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("Second")); err != nil {
		t.Fatalf("second append: %v", err)
	}

	content := readFile(t, path)
	if !strings.Contains(content, foreign) {
		t.Errorf("foreign content not preserved; content:\n%s", content)
	}
	if !strings.Contains(content, "# BEGIN gitid managed: personal") {
		t.Errorf("other identity's block not preserved; content:\n%s", content)
	}
	if !strings.Contains(content, "PersonalKeyDataHere") {
		t.Errorf("other identity's signer line not preserved; content:\n%s", content)
	}
}

// TestAppendAllowedSigners_RejectsCommaEmail proves a comma-containing email
// is rejected before any write (CR-18), leaving the file untouched/absent.
func TestAppendAllowedSigners_RejectsCommaEmail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")

	if _, err := AppendAllowedSigners(path, "work", "victim@corp.test,*", pubLine("First")); err == nil {
		t.Fatal("AppendAllowedSigners must reject a comma-containing principal (CR-18), got nil error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected the file to remain absent after a rejected append; stat err = %v", err)
	}
}

// TestAppendAllowedSigners_Mode0644 proves the written file's mode is 0644.
func TestAppendAllowedSigners_Mode0644(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")

	if _, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("First")); err != nil {
		t.Fatalf("AppendAllowedSigners: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat allowed_signers: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("allowed_signers mode = %o, want 0644", got)
	}
}

// TestAppendAllowedSigners_ThenBlockRemovalRemovesAllLines proves the
// existing block-keyed removal helper (gitconfig.RemoveAllowedSignersBlock)
// still removes the WHOLE identity block after two appends — all three
// lines (the two appended plus none foreign inside the block) disappear
// together, because removal is keyed by identity NAME, not by line (review
// R-22): rotate-twice accumulation leaves N+1 lines in one block; delete-
// everything removes the whole block regardless of line count, and a
// Git-only delete keeps the whole block regardless of line count (D-10).
func TestAppendAllowedSigners_ThenBlockRemovalRemovesAllLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allowed_signers")

	if _, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("First")); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if _, err := AppendAllowedSigners(path, "work", "work@example.com", pubLine("Second")); err != nil {
		t.Fatalf("second append: %v", err)
	}
	before := readFile(t, path)
	if strings.Count(before, "namespaces=\"git\"") != 2 {
		t.Fatalf("fixture setup: expected 2 lines before removal, got content:\n%s", before)
	}

	if _, err := gitconfig.RemoveAllowedSignersBlock(path, "work"); err != nil {
		t.Fatalf("RemoveAllowedSignersBlock: %v", err)
	}

	after := readFile(t, path)
	if strings.Contains(after, "# BEGIN gitid managed: work") {
		t.Errorf("work block still present after block-keyed removal; content:\n%s", after)
	}
	if strings.Contains(after, "FirstKeyDataHere") || strings.Contains(after, "SecondKeyDataHere") {
		t.Errorf("both accumulated lines must disappear together with the block; content:\n%s", after)
	}
}

// mustLine is a small test helper wrapping AllowedSignersLine for fixture
// setup where the line is known-good and an error would indicate a broken
// test, not a case under test.
func mustLine(t *testing.T, email, pub string) string {
	t.Helper()
	line, err := AllowedSignersLine(email, pub)
	if err != nil {
		t.Fatalf("AllowedSignersLine fixture setup: %v", err)
	}
	return line
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // path is a test-controlled temp file
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
