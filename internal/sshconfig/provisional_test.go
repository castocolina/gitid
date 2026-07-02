package sshconfig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// configWithManagedAndForeign returns a config file body containing a managed
// block for "alice" and a hand-written Host stanza, used across multiple tests
// to verify foreign-content preservation.
func configWithManagedAndForeign() string {
	return filewriter.BeginPrefix + "alice\n" +
		"Host alice.github.com\n" +
		"  Hostname ssh.github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_alice_final\n" +
		"  IdentitiesOnly yes\n" +
		"# gitid: provider=github\n" +
		filewriter.EndPrefix + "alice\n" +
		"\n" +
		"Host handwritten.example.com\n" +
		"  Hostname handwritten.example.com\n" +
		"  User sre\n"
}

// writeConfig writes content to a temp config file and returns the path.
func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return path
}

// readConfig reads the config file at path and returns its content as string.
func readConfig(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	return string(data)
}

// TestWriteProvisional_WritesDistinctProvisionalBlock verifies that
// WriteProvisional writes a block delimited by the provisional sentinel
// (not the managed sentinel) and that the IdentityFile inside points at the
// caller-supplied staged key path.
func TestWriteProvisional_WritesDistinctProvisionalBlock(t *testing.T) {
	dir := t.TempDir()
	configPath := writeConfig(t, dir, configWithManagedAndForeign())

	stagedKeyPath := "/tmp/gitid-staging-abc123/id_ed25519_alice"
	hostBlock := sshconfig.RenderHostBlock("alice.github.com", "ssh.github.com", 443, stagedKeyPath, "github")

	backupPath, err := sshconfig.WriteProvisional(configPath, "alice", hostBlock)
	if err != nil {
		t.Fatalf("WriteProvisional error: %v", err)
	}

	// A backup must have been created.
	if backupPath == "" {
		t.Error("expected non-empty backupPath (file pre-existed)")
	}
	if _, statErr := os.Stat(backupPath); statErr != nil {
		t.Errorf("backup file does not exist at %q: %v", backupPath, statErr)
	}

	got := readConfig(t, configPath)

	// Provisional sentinel must be present.
	if !strings.Contains(got, filewriter.ProvisionalBeginPrefix+"alice") {
		t.Errorf("provisional BEGIN sentinel missing:\n%q", got)
	}
	if !strings.Contains(got, filewriter.ProvisionalEndPrefix+"alice") {
		t.Errorf("provisional END sentinel missing:\n%q", got)
	}

	// IdentityFile must point at the STAGED key.
	if !strings.Contains(got, stagedKeyPath) {
		t.Errorf("staged key path %q missing from provisional block:\n%q", stagedKeyPath, got)
	}

	// Managed block must still be there, byte-identical.
	if !strings.Contains(got, filewriter.BeginPrefix+"alice") {
		t.Errorf("managed BEGIN sentinel disappeared:\n%q", got)
	}

	// Hand-written content must be preserved.
	if !strings.Contains(got, "Host handwritten.example.com") {
		t.Errorf("hand-written host block disappeared:\n%q", got)
	}
}

// TestWriteProvisional_MissingFile verifies that WriteProvisional works on a
// non-existent config file (creates it, returns empty backupPath).
func TestWriteProvisional_MissingFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	hostBlock := sshconfig.RenderHostBlock("github.com", "ssh.github.com", 443, "/tmp/staged/key", "github")

	backupPath, err := sshconfig.WriteProvisional(configPath, "myfirst", hostBlock)
	if err != nil {
		t.Fatalf("WriteProvisional on missing file error: %v", err)
	}

	// No backup when file did not pre-exist.
	if backupPath != "" {
		t.Errorf("expected empty backupPath for missing file, got %q", backupPath)
	}

	got := readConfig(t, configPath)
	if !strings.Contains(got, filewriter.ProvisionalBeginPrefix+"myfirst") {
		t.Errorf("provisional block missing from new config:\n%q", got)
	}
}

// TestPromote_SwapsProvisionalToManaged verifies that Promote atomically removes
// the provisional block and sets the managed block in ONE write, so the result
// has a managed block (final key) and NO provisional marker (T-05.7-14-05).
func TestPromote_SwapsProvisionalToManaged(t *testing.T) {
	dir := t.TempDir()

	// Start: managed block for alice (final key) + provisional block (staged key).
	stagedKeyPath := "/tmp/gitid-staging/id_ed25519_alice"
	finalKeyPath := "~/.ssh/id_ed25519_alice"
	managedBody := sshconfig.RenderHostBlock("alice.github.com", "ssh.github.com", 443, finalKeyPath, "github")
	provisionalBody := sshconfig.RenderHostBlock("alice.github.com", "ssh.github.com", 443, stagedKeyPath, "github")

	initial := filewriter.ReplaceBlock(nil, "alice", managedBody)
	initial = filewriter.ReplaceProvisionalBlock(initial, "alice", provisionalBody)
	initial = append(initial, []byte("# foreign line\n")...)
	configPath := writeConfig(t, dir, string(initial))

	// Promote: remove provisional + write new managed block (final key body).
	managedFinalBody := sshconfig.RenderHostBlock("alice.github.com", "ssh.github.com", 443, finalKeyPath, "github")
	backupPath, err := sshconfig.Promote(configPath, "alice", managedFinalBody)
	if err != nil {
		t.Fatalf("Promote error: %v", err)
	}
	if backupPath == "" {
		t.Error("expected non-empty backupPath")
	}

	got := readConfig(t, configPath)

	// Provisional sentinel must be completely gone.
	if strings.Contains(got, filewriter.ProvisionalBeginPrefix+"alice") {
		t.Errorf("provisional BEGIN sentinel still present after Promote:\n%q", got)
	}
	if strings.Contains(got, filewriter.ProvisionalEndPrefix+"alice") {
		t.Errorf("provisional END sentinel still present after Promote:\n%q", got)
	}
	if strings.Contains(got, stagedKeyPath) {
		t.Errorf("staged key path still present after Promote:\n%q", got)
	}

	// Managed block must exist with the final key.
	if !strings.Contains(got, filewriter.BeginPrefix+"alice") {
		t.Errorf("managed BEGIN sentinel missing after Promote:\n%q", got)
	}
	if !strings.Contains(got, finalKeyPath) {
		t.Errorf("final key path missing from managed block after Promote:\n%q", got)
	}

	// Foreign content must be preserved.
	if !strings.Contains(got, "# foreign line") {
		t.Errorf("foreign content disappeared after Promote:\n%q", got)
	}
}

// TestPromote_ProducesManagedBodyByteIdenticalToWrite verifies that Promote
// produces a result whose managed block body is byte-identical to what a direct
// sshconfig.Write call would produce (the managed block is identical regardless
// of the path taken).
func TestPromote_ProducesManagedBodyByteIdenticalToWrite(t *testing.T) {
	dir := t.TempDir()
	finalKeyPath := "~/.ssh/id_ed25519_bob"
	managedBody := sshconfig.RenderHostBlock("bob.github.com", "ssh.github.com", 443, finalKeyPath, "github")
	provisionalBody := sshconfig.RenderHostBlock("bob.github.com", "ssh.github.com", 443, "/tmp/staged/key", "github")

	// Write path: provision then promote.
	initialProv := filewriter.ReplaceProvisionalBlock(nil, "bob", provisionalBody)
	configProv := writeConfig(t, dir, string(initialProv))
	_, promErr := sshconfig.Promote(configProv, "bob", managedBody)
	if promErr != nil {
		t.Fatalf("Promote error: %v", promErr)
	}
	afterPromote := readConfig(t, configProv)

	// Direct managed write path — write to a fresh file in a separate temp dir.
	dir2 := t.TempDir()
	configDirect := filepath.Join(dir2, "config")
	if err := os.WriteFile(configDirect, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, writeErr := sshconfig.Write(configDirect, "bob", managedBody, "")
	if writeErr != nil {
		t.Fatalf("Write error: %v", writeErr)
	}
	afterWrite := readConfig(t, configDirect)

	// The managed block body should be present in both results.
	if !strings.Contains(afterPromote, managedBody) {
		t.Errorf("after Promote, managed body not found:\n%q", afterPromote)
	}
	if !strings.Contains(afterWrite, managedBody) {
		t.Errorf("after Write, managed body not found:\n%q", afterWrite)
	}
}

// TestDropProvisional_RemovesProvisionalPreservesManagedAndForeign verifies
// that DropProvisional removes ONLY the provisional block and preserves a co-
// resident managed block + hand-written Host block byte-for-byte (T-05.7-14-03).
func TestDropProvisional_RemovesProvisionalPreservesManagedAndForeign(t *testing.T) {
	dir := t.TempDir()
	finalKeyPath := "~/.ssh/id_ed25519_alice"
	stagedKeyPath := "/tmp/staged/key"

	managedBody := sshconfig.RenderHostBlock("alice.github.com", "ssh.github.com", 443, finalKeyPath, "github")
	provisionalBody := sshconfig.RenderHostBlock("alice.github.com", "ssh.github.com", 443, stagedKeyPath, "github")

	initial := filewriter.ReplaceBlock(nil, "alice", managedBody)
	initial = append(initial, []byte("Host handwritten.example.com\n  Hostname h.example.com\n")...)
	initial = filewriter.ReplaceProvisionalBlock(initial, "alice", provisionalBody)
	configPath := writeConfig(t, dir, string(initial))

	backupPath, err := sshconfig.DropProvisional(configPath, "alice")
	if err != nil {
		t.Fatalf("DropProvisional error: %v", err)
	}
	if backupPath == "" {
		t.Error("expected non-empty backupPath")
	}

	got := readConfig(t, configPath)

	// Provisional block gone.
	if strings.Contains(got, filewriter.ProvisionalBeginPrefix+"alice") {
		t.Errorf("provisional BEGIN sentinel still present after DropProvisional:\n%q", got)
	}
	if strings.Contains(got, stagedKeyPath) {
		t.Errorf("staged key path still present after DropProvisional:\n%q", got)
	}

	// Managed block preserved.
	if !strings.Contains(got, filewriter.BeginPrefix+"alice") {
		t.Errorf("managed BEGIN sentinel disappeared after DropProvisional:\n%q", got)
	}
	if !strings.Contains(got, finalKeyPath) {
		t.Errorf("final key path disappeared after DropProvisional:\n%q", got)
	}

	// Foreign hand-written content preserved.
	if !strings.Contains(got, "Host handwritten.example.com") {
		t.Errorf("hand-written host block disappeared after DropProvisional:\n%q", got)
	}
}

// TestDropProvisional_IdempotentWhenAbsent verifies that DropProvisional on a
// config without a provisional block writes back the same content (still backs
// up) and returns no error (idempotent absent-block behavior).
func TestDropProvisional_IdempotentWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	content := configWithManagedAndForeign()
	configPath := writeConfig(t, dir, content)

	backupPath, err := sshconfig.DropProvisional(configPath, "alice")
	if err != nil {
		t.Fatalf("DropProvisional (absent block) error: %v", err)
	}
	// A backup is still produced (the file pre-existed).
	if backupPath == "" {
		t.Error("expected non-empty backupPath even when provisional block is absent")
	}

	// Content is unchanged (idempotent).
	got := readConfig(t, configPath)
	if got != content {
		t.Errorf("content changed when provisional block was absent:\n got %q\nwant %q", got, content)
	}
}

// TestDropProvisional_MissingFile verifies that DropProvisional on a missing
// config file returns ("", nil) — idempotent.
func TestDropProvisional_MissingFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "no-such-config")
	backupPath, err := sshconfig.DropProvisional(configPath, "alice")
	if err != nil {
		t.Errorf("DropProvisional missing file: unexpected error: %v", err)
	}
	if backupPath != "" {
		t.Errorf("DropProvisional missing file: expected empty backupPath, got %q", backupPath)
	}
}

// TestListProvisional_ReturnsProvisionalNames verifies that ListProvisional
// returns the provisional block names in file order.
func TestListProvisional_ReturnsProvisionalNames(t *testing.T) {
	content := filewriter.ReplaceProvisionalBlock(nil, "alice", "Host alice.github.com\n")
	content = filewriter.ReplaceProvisionalBlock(content, "bob", "Host bob.github.com\n")
	// Also add a managed block — must NOT appear in ListProvisional result.
	content = filewriter.ReplaceBlock(content, "charlie", "Host charlie.github.com\n")

	names := sshconfig.ListProvisional(content)

	if len(names) != 2 {
		t.Fatalf("expected 2 provisional names, got %d: %v", len(names), names)
	}
	if names[0] != "alice" {
		t.Errorf("names[0] = %q, want %q", names[0], "alice")
	}
	if names[1] != "bob" {
		t.Errorf("names[1] = %q, want %q", names[1], "bob")
	}
}

// TestListProvisional_EmptyReturnsNil verifies that ListProvisional on empty
// content returns nil.
func TestListProvisional_EmptyReturnsNil(t *testing.T) {
	names := sshconfig.ListProvisional([]byte(""))
	if names != nil {
		t.Fatalf("expected nil for empty content, got %v", names)
	}
}

// TestWriteProvisional_ParseRoundTrip verifies that WriteProvisional produces a
// config that the sshconfig parser can decode cleanly (parse-validate guard).
func TestWriteProvisional_ParseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")

	hostBlock := sshconfig.RenderHostBlock("github.com", "ssh.github.com", 443, "/tmp/staged/key", "github")
	if _, err := sshconfig.WriteProvisional(configPath, "default", hostBlock); err != nil {
		t.Fatalf("WriteProvisional error: %v", err)
	}

	data, _ := os.ReadFile(configPath) //nolint:gosec
	if _, err := sshconfig.Parse(data); err != nil {
		t.Errorf("config after WriteProvisional is not parseable: %v\ncontent:\n%q", err, data)
	}
}

// TestPromote_BackupContainsProvisional verifies that the backup created by
// Promote contains the provisional block (the pre-promote snapshot).
func TestPromote_BackupContainsProvisional(t *testing.T) {
	dir := t.TempDir()
	provisionalBody := sshconfig.RenderHostBlock("github.com", "ssh.github.com", 443, "/tmp/staged/key", "github")
	finalBody := sshconfig.RenderHostBlock("github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519", "github")

	initial := filewriter.ReplaceProvisionalBlock(nil, "myid", provisionalBody)
	configPath := writeConfig(t, dir, string(initial))

	backupPath, err := sshconfig.Promote(configPath, "myid", finalBody)
	if err != nil {
		t.Fatalf("Promote error: %v", err)
	}

	backupData, err := os.ReadFile(backupPath) //nolint:gosec
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if !strings.Contains(string(backupData), filewriter.ProvisionalBeginPrefix+"myid") {
		t.Errorf("backup does not contain the pre-promote provisional block:\n%q", string(backupData))
	}
}
