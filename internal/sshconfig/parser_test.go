package sshconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseRoundTripStable asserts CONTEXT D-12/D-13: parse -> String() -> parse
// again yields an equivalent host definition (round-trip stable).
func TestParseRoundTripStable(t *testing.T) {
	src := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work")

	cfg, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(render) failed: %v", err)
	}

	rendered := cfg.String()

	cfg2, err := Parse([]byte(rendered))
	if err != nil {
		t.Fatalf("Parse(String()) failed: %v", err)
	}

	// The resolved IdentityFile for the alias must survive both passes.
	got1, err := cfg.Get("work.github.com", "IdentityFile")
	if err != nil {
		t.Fatalf("cfg.Get pass 1: %v", err)
	}
	got2, err := cfg2.Get("work.github.com", "IdentityFile")
	if err != nil {
		t.Fatalf("cfg.Get pass 2: %v", err)
	}
	if got1 != got2 || got1 != "~/.ssh/id_ed25519_work" {
		t.Fatalf("round-trip IdentityFile drift: pass1=%q pass2=%q", got1, got2)
	}
}

// TestWritePreservesForeignContent asserts SAFE-02 / T-02-17: a hand-written
// host block outside the gitid sentinels is preserved byte-identical and only
// the managed block is inserted.
func TestWritePreservesForeignContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	foreign := "Host legacy.example.com\n  Hostname legacy.example.com\n  User git\n"
	if err := os.WriteFile(path, []byte(foreign), 0o600); err != nil {
		t.Fatalf("seeding foreign config: %v", err)
	}

	host := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work")
	if _, err := Write(path, "work.github.com", host, ""); err != nil {
		t.Fatalf("Write: %v", err)
	}

	out, err := os.ReadFile(path) //nolint:gosec // test reads back a TempDir fixture path it just wrote
	if err != nil {
		t.Fatalf("reading written config: %v", err)
	}
	if !strings.Contains(string(out), foreign) {
		t.Fatalf("foreign content not preserved byte-identical; got:\n%s", out)
	}
	if !strings.Contains(string(out), "# BEGIN gitid managed: work.github.com") {
		t.Fatalf("managed block not inserted; got:\n%s", out)
	}
}

// TestWriteIdempotent asserts SAFE-02: writing the same account+blocks twice
// yields byte-identical content (empty diff).
func TestWriteIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	host := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work")
	global := RenderGlobalBlock("darwin")

	if _, err := Write(path, "work.github.com", host, global); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	first, err := os.ReadFile(path) //nolint:gosec // test reads back a TempDir fixture path it just wrote
	if err != nil {
		t.Fatalf("read after first write: %v", err)
	}

	if _, err := Write(path, "work.github.com", host, global); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	second, err := os.ReadFile(path) //nolint:gosec // test reads back a TempDir fixture path it just wrote
	if err != nil {
		t.Fatalf("read after second write: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("non-idempotent write; first:\n%s\nsecond:\n%s", first, second)
	}
}

// TestWriteGlobalBlockOrderedLast asserts Pitfall 5 / T-02-15: the _global
// block lands after the specific host block in the composed file.
func TestWriteGlobalBlockOrderedLast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	host := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work")
	global := RenderGlobalBlock("darwin")

	if _, err := Write(path, "work.github.com", host, global); err != nil {
		t.Fatalf("Write: %v", err)
	}

	out, err := os.ReadFile(path) //nolint:gosec // test reads back a TempDir fixture path it just wrote
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(out)

	hostIdx := strings.Index(content, "Host work.github.com")
	wildcardIdx := strings.Index(content, "Host *")
	if hostIdx == -1 || wildcardIdx == -1 {
		t.Fatalf("missing host markers; got:\n%s", content)
	}
	if wildcardIdx < hostIdx {
		t.Fatalf("'Host *' must be ordered after specific host; got:\n%s", content)
	}
}

// TestWriteBackupOnPreexisting asserts the writer returns a non-empty backupPath
// when the config pre-existed (delegated to filewriter, mode 0600).
func TestWriteBackupOnPreexisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	if err := os.WriteFile(path, []byte("Host old\n  User git\n"), 0o600); err != nil {
		t.Fatalf("seeding config: %v", err)
	}

	host := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work")
	backupPath, err := Write(path, "work.github.com", host, "")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if backupPath == "" {
		t.Fatalf("expected non-empty backupPath for pre-existing config")
	}
	if _, statErr := os.Stat(backupPath); statErr != nil {
		t.Fatalf("backup file missing at %q: %v", backupPath, statErr)
	}
}
