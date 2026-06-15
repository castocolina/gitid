package gitconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
)

// TestParseIncludeIfBody_GitdirMatch verifies that a block with one gitdir
// condition is parsed into an IncludeIfInfo with one MatchGitdir match.
func TestParseIncludeIfBody_GitdirMatch(t *testing.T) {
	fragPath := "~/.gitconfig.d/work"
	body := "[includeIf \"gitdir:~/git/work/\"]\n\tpath = " + fragPath

	result := parseIncludeIfBody(body)
	if result.FragmentPath != fragPath {
		t.Errorf("FragmentPath: got %q want %q", result.FragmentPath, fragPath)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}
	if result.Matches[0].Kind != MatchGitdir {
		t.Errorf("match kind: got %v want MatchGitdir", result.Matches[0].Kind)
	}
	if result.Matches[0].Value != "~/git/work/" {
		t.Errorf("match value: got %q want %q", result.Matches[0].Value, "~/git/work/")
	}
}

// TestParseIncludeIfBody_HasconfigMatch verifies that a hasconfig condition
// produces a MatchHasconfig match.
func TestParseIncludeIfBody_HasconfigMatch(t *testing.T) {
	fragPath := "~/.gitconfig.d/work"
	body := "[includeIf \"hasconfig:remote.*.url:git@github.com:work/**\"]\n\tpath = " + fragPath

	result := parseIncludeIfBody(body)
	if result.FragmentPath != fragPath {
		t.Errorf("FragmentPath: got %q want %q", result.FragmentPath, fragPath)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}
	if result.Matches[0].Kind != MatchHasconfig {
		t.Errorf("match kind: got %v want MatchHasconfig", result.Matches[0].Kind)
	}
}

// TestParseManagedIncludeIf_TwoBlocks verifies that two managed includeIf
// blocks are parsed into a map with two IncludeIfInfo entries.
func TestParseManagedIncludeIf_TwoBlocks(t *testing.T) {
	personalBody := "[includeIf \"gitdir:~/git/personal/\"]\n\tpath = ~/.gitconfig.d/personal"
	workBody := "[includeIf \"gitdir:~/git/work/\"]\n\tpath = ~/.gitconfig.d/work"

	content := []byte(
		filewriter.BeginPrefix + "personal\n" + personalBody + "\n" + filewriter.EndPrefix + "personal\n" +
			filewriter.BeginPrefix + "work\n" + workBody + "\n" + filewriter.EndPrefix + "work\n",
	)

	got := ParseManagedIncludeIf(content)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(got), got)
	}

	personal, ok := got["personal"]
	if !ok {
		t.Fatal("missing 'personal' entry")
	}
	if personal.FragmentPath != "~/.gitconfig.d/personal" {
		t.Errorf("personal FragmentPath: got %q want ~/.gitconfig.d/personal", personal.FragmentPath)
	}

	if _, ok := got["work"]; !ok {
		t.Fatal("missing 'work' entry")
	}
}

// TestParseManagedIncludeIf_Empty verifies that empty content returns an empty
// map.
func TestParseManagedIncludeIf_Empty(t *testing.T) {
	got := ParseManagedIncludeIf([]byte(""))
	if len(got) != 0 {
		t.Fatalf("expected empty map for empty content, got %d entries", len(got))
	}
}

// TestReadFragment_Missing verifies that ReadFragment returns FragmentInfo with
// Missing=true and no error when the file does not exist.
func TestReadFragment_Missing(t *testing.T) {
	dir := t.TempDir()
	nonExistent := filepath.Join(dir, "nonexistent")

	info, err := ReadFragment(nonExistent)
	if err != nil {
		t.Fatalf("ReadFragment on missing file returned error: %v", err)
	}
	if !info.Missing {
		t.Error("Missing should be true for a non-existent fragment file")
	}
}

// TestReadFragment_Full verifies that ReadFragment on a file written by
// WriteFragment returns the correct GitName, GitEmail, SigningKey, GPGFormat,
// and CommitSign fields.
func TestReadFragment_Full(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	fragPath := filepath.Join(dir, ".gitconfig.d", "work")
	if err := os.MkdirAll(filepath.Dir(fragPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pubKeyPath := filepath.Join(dir, ".ssh", "id_ed25519_work.pub")
	if err := os.MkdirAll(filepath.Dir(pubKeyPath), 0o700); err != nil {
		t.Fatalf("mkdir ssh: %v", err)
	}

	// Write a minimal fragment using WriteFragment.
	if err := WriteFragment(fragPath, "Test User", "test@example.com", pubKeyPath, true); err != nil {
		t.Fatalf("WriteFragment: %v", err)
	}

	info, err := ReadFragment(fragPath)
	if err != nil {
		t.Fatalf("ReadFragment returned error: %v", err)
	}
	if info.Missing {
		t.Error("Missing should be false for an existing fragment")
	}
	if info.GitName != "Test User" {
		t.Errorf("GitName: got %q want %q", info.GitName, "Test User")
	}
	if info.GitEmail != "test@example.com" {
		t.Errorf("GitEmail: got %q want %q", info.GitEmail, "test@example.com")
	}
	if info.SigningKey != pubKeyPath {
		t.Errorf("SigningKey: got %q want %q", info.SigningKey, pubKeyPath)
	}
	if info.GPGFormat != "ssh" {
		t.Errorf("GPGFormat: got %q want %q", info.GPGFormat, "ssh")
	}
	if !info.CommitSign {
		t.Error("CommitSign should be true")
	}
}

// TestRemoveAllowedSignersLine_RemovesMatchingLine verifies that the line
// containing BOTH the email AND namespaces="git" is removed.
func TestRemoveAllowedSignersLine_RemovesMatchingLine(t *testing.T) {
	dir := t.TempDir()
	allowedPath := filepath.Join(dir, "allowed_signers")

	content := "test@example.com namespaces=\"git\" ssh-ed25519 AAAA1234\n" +
		"other@example.com namespaces=\"git\" ssh-ed25519 AAAA5678\n"
	if err := os.WriteFile(allowedPath, []byte(content), 0o600); err != nil {
		t.Fatalf("seeding allowed_signers: %v", err)
	}

	backupPath, err := RemoveAllowedSignersLine(allowedPath, "test@example.com")
	if err != nil {
		t.Fatalf("RemoveAllowedSignersLine returned error: %v", err)
	}
	if backupPath == "" {
		t.Error("expected non-empty backupPath for existing file")
	}

	got, err := os.ReadFile(allowedPath) //nolint:gosec // test reads back the file under test
	if err != nil {
		t.Fatalf("reading result: %v", err)
	}
	if strings.Contains(string(got), "test@example.com") {
		t.Errorf("test@example.com line not removed:\n%s", got)
	}
	if !strings.Contains(string(got), "other@example.com") {
		t.Errorf("other@example.com line was incorrectly removed:\n%s", got)
	}
}

// TestRemoveAllowedSignersLine_PreservesNonGitNamespace verifies that a line
// with the same email but a different namespace is NOT removed (Pitfall D /
// T-03-01).
func TestRemoveAllowedSignersLine_PreservesNonGitNamespace(t *testing.T) {
	dir := t.TempDir()
	allowedPath := filepath.Join(dir, "allowed_signers")

	// Line with different namespace — must NOT be removed.
	content := "test@example.com namespaces=\"email\" ssh-ed25519 AAAA1234\n"
	if err := os.WriteFile(allowedPath, []byte(content), 0o600); err != nil {
		t.Fatalf("seeding allowed_signers: %v", err)
	}

	_, err := RemoveAllowedSignersLine(allowedPath, "test@example.com")
	if err != nil {
		t.Fatalf("RemoveAllowedSignersLine returned error: %v", err)
	}

	got, err := os.ReadFile(allowedPath) //nolint:gosec
	if err != nil {
		t.Fatalf("reading result: %v", err)
	}
	if !strings.Contains(string(got), "test@example.com") {
		t.Error("non-git namespace line was incorrectly removed (Pitfall D)")
	}
}

// TestRemoveAllowedSignersLine_Idempotent verifies that calling
// RemoveAllowedSignersLine when no matching line exists returns "", nil (no
// change, no error).
func TestRemoveAllowedSignersLine_Idempotent(t *testing.T) {
	dir := t.TempDir()
	allowedPath := filepath.Join(dir, "allowed_signers")

	content := "other@example.com namespaces=\"git\" ssh-ed25519 AAAA5678\n"
	if err := os.WriteFile(allowedPath, []byte(content), 0o600); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	_, err := RemoveAllowedSignersLine(allowedPath, "nonexistent@example.com")
	if err != nil {
		t.Fatalf("RemoveAllowedSignersLine returned unexpected error: %v", err)
	}
}

// TestRemoveAllowedSignersLine_MissingFile verifies that a missing file
// returns ("", nil).
func TestRemoveAllowedSignersLine_MissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "allowed_signers")

	backupPath, err := RemoveAllowedSignersLine(missing, "test@example.com")
	if err != nil {
		t.Fatalf("RemoveAllowedSignersLine on missing file returned error: %v", err)
	}
	if backupPath != "" {
		t.Errorf("expected empty backupPath for missing file, got %q", backupPath)
	}
}
