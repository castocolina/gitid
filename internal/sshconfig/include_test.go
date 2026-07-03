package sshconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/tester"
)

// TestEnsureIncludeLineFloorsAndParses proves the Include block is prepended at
// the TOP of ~/.ssh/config (floor model — D-10), ahead of pre-existing
// hand-written content, and that the composed config parses cleanly.
func TestEnsureIncludeLineFloorsAndParses(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")

	// Seed pre-existing hand-written content; the Include block must land
	// BEFORE this, not after (floor semantics).
	if err := os.WriteFile(configPath, []byte("Host existing\n  Hostname example.com\n"), 0o600); err != nil {
		t.Fatalf("seeding existing config: %v", err)
	}

	backupPath, err := EnsureIncludeLine(configPath)
	if err != nil {
		t.Fatalf("EnsureIncludeLine: %v", err)
	}
	if backupPath == "" {
		t.Error("expected non-empty backup path for a pre-existing config")
	}

	composed, err := os.ReadFile(configPath) //nolint:gosec // configPath is a hermetic t.TempDir() fixture path (G304)
	if err != nil {
		t.Fatalf("reading composed config: %v", err)
	}

	if !strings.HasPrefix(string(composed), filewriter.BeginPrefix+sshIncludeBlockName) {
		t.Errorf("Include block is not floored at the top of the file; got:\n%s", composed)
	}
	if !strings.Contains(string(composed), "Include ~/.ssh/config.d/*.config") {
		t.Errorf("composed config missing the canonical Include line; got:\n%s", composed)
	}
	// The pre-existing content must still be present, after the floored block.
	if !strings.Contains(string(composed), "Host existing") {
		t.Errorf("pre-existing content was lost; got:\n%s", composed)
	}
	if strings.Index(string(composed), "Host existing") < strings.Index(string(composed), "Include ~/.ssh/config.d/*.config") {
		t.Errorf("pre-existing content appears BEFORE the Include line; floor semantics violated:\n%s", composed)
	}

	if _, perr := Parse(composed); perr != nil {
		t.Errorf("composed config does not parse: %v", perr)
	}
}

// TestEnsureIncludeLineIdempotent proves re-running EnsureIncludeLine does not
// duplicate the Include line (SC-1 idempotency).
func TestEnsureIncludeLineIdempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")

	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("first EnsureIncludeLine: %v", err)
	}
	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("second EnsureIncludeLine: %v", err)
	}

	composed, err := os.ReadFile(configPath) //nolint:gosec // configPath is a hermetic t.TempDir() fixture path (G304)
	if err != nil {
		t.Fatalf("reading composed config: %v", err)
	}

	count := strings.Count(string(composed), "Include ~/.ssh/config.d/*.config")
	if count != 1 {
		t.Errorf("expected exactly 1 Include line after two runs, got %d; composed:\n%s", count, composed)
	}
}

// TestEnsureIncludeLineMissingFileTolerated proves a missing config file is
// tolerated (os.IsNotExist), not an error — the common first-run case.
func TestEnsureIncludeLineMissingFileTolerated(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config") // does not exist yet

	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("EnsureIncludeLine on missing file: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config to be created: %v", err)
	}
}

// TestEnsureIncludeDirCreatesAt0700 proves the config.d directory is created at
// mode 0700 (STORE-01, private material — never relies on the umask).
func TestEnsureIncludeDirCreatesAt0700(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".ssh", "config.d")

	if err := EnsureIncludeDir(configDir); err != nil {
		t.Fatalf("EnsureIncludeDir: %v", err)
	}

	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("stat config.d: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("config.d dir mode = %o, want 0700", perm)
	}
}

// TestEnsureIncludeDirChmodsExistingBackTo0700 proves an already-existing
// loosely-permissioned directory is chmod'd back to 0700.
func TestEnsureIncludeDirChmodsExistingBackTo0700(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".ssh", "config.d")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatalf("seeding loose-permission dir: %v", err)
	}

	if err := EnsureIncludeDir(configDir); err != nil {
		t.Fatalf("EnsureIncludeDir: %v", err)
	}

	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("stat config.d: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("config.d dir mode = %o, want 0700 after chmod-back", perm)
	}
}

// TestIsReservedBlockName proves the reserved Include block name is
// recognised, mirroring gitconfig.IsReservedBlockName (Pitfall 4).
func TestIsReservedBlockName(t *testing.T) {
	if !IsReservedBlockName("ssh-include") {
		t.Error(`IsReservedBlockName("ssh-include") = false, want true`)
	}
	if IsReservedBlockName("personal") {
		t.Error(`IsReservedBlockName("personal") = true, want false`)
	}
}

// TestIncludeResolution proves real `ssh -G` resolves an alias THROUGH the
// Include'd config.d/*.config file (Pattern 4, first-match-wins) using a real,
// filesystem-backed fixture under a hermetic t.TempDir() home (Pitfall 5 — an
// in-memory-only Include fixture would either leak the real runner's home or
// silently no-op). It also proves the config.d dir is 0700 and the Include'd
// file is 0600 (STORE-01).
func TestIncludeResolution(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found; skipping include-resolution test")
	}

	// Hermetic HOME: the real `ssh` binary tilde-expands Include paths against
	// $HOME (verified empirically), so setting HOME pins resolution to this
	// t.TempDir() fixture tree — no real ~/.ssh is touched (T-03-08).
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seeding hermetic .ssh dir: %v", err)
	}
	configPath := filepath.Join(sshDir, "config")
	configDir := filepath.Join(sshDir, "config.d")

	if err := EnsureIncludeDir(configDir); err != nil {
		t.Fatalf("EnsureIncludeDir: %v", err)
	}
	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("EnsureIncludeLine: %v", err)
	}

	// Write the Include'd gitid.config fixture as a REAL filesystem-backed file
	// through the filewriter chokepoint at 0600 (STORE-04/Pitfall 5).
	includeFilePath := filepath.Join(configDir, "gitid.config")
	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	if _, err := filewriter.Write(includeFilePath, []byte(hostBlock), 0o600); err != nil {
		t.Fatalf("writing Include'd gitid.config fixture: %v", err)
	}

	// Prove the permission bits, not just file presence.
	dirInfo, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("stat config.d: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("config.d dir mode = %o, want 0700", perm)
	}
	fileInfo, err := os.Stat(includeFilePath)
	if err != nil {
		t.Fatalf("stat gitid.config: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("gitid.config file mode = %o, want 0600", perm)
	}

	// Real ssh -G -F <configPath> proves first-match-wins resolution THROUGH
	// the Include'd file — never faked (Pattern 4, CONTEXT.md-locked constraint).
	out, err := exec.Command("ssh", "-G", "-F", configPath, "personal.github.com").Output() //nolint:gosec // arg-slice form, no shell; configPath is a hermetic t.TempDir() fixture (G204)
	if err != nil {
		t.Fatalf("ssh -G -F %s personal.github.com: %v", configPath, err)
	}
	resolved := tester.ParseResolved(string(out))
	if len(resolved.IdentityFiles) == 0 {
		t.Fatal("resolved no IdentityFiles; Include resolution failed")
	}
	if resolved.IdentityFiles[0] != identityKey {
		t.Errorf("resolved IdentityFile = %q, want %q (Include'd file was not consulted first-match-wins)",
			resolved.IdentityFiles[0], identityKey)
	}
}
