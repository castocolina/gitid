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
// recognised, mirroring gitconfig.IsReservedBlockName (Pitfall 4), and that
// BOTH globals sentinel keys — the current GlobalBlockName and the
// pre-D-08 LegacyGlobalBlockName — are reserved too: the create ceremony
// writes the wildcard block on EVERY create, so an unregistered name would
// let the doctor Orphans fix path delete it in a destructive false-positive
// loop (L4), and a machine still carrying the legacy name must be treated
// identically until its next write adopts it (D-08).
func TestIsReservedBlockName(t *testing.T) {
	if !IsReservedBlockName(sshIncludeBlockName) {
		t.Error(`IsReservedBlockName(sshIncludeBlockName) = false, want true`)
	}
	for _, name := range []string{GlobalBlockName, LegacyGlobalBlockName} {
		if !IsReservedBlockName(name) {
			t.Errorf("IsReservedBlockName(%q) = false, want true (D-08 globals block, L4)", name)
		}
	}
	if IsReservedBlockName("personal") {
		t.Error(`IsReservedBlockName("personal") = true, want false`)
	}
}

// TestIsGlobalBlockName proves the NARROW wildcard-block predicate recognises
// both globals sentinel names and — critically — returns FALSE for the
// Include-line block name: IsGlobalBlockName is not the broad reserved
// predicate, and conflating the two is what would make the Include wiring
// movable in a storage migration (06-REVIEWS.md HIGH).
func TestIsGlobalBlockName(t *testing.T) {
	for _, name := range []string{GlobalBlockName, LegacyGlobalBlockName} {
		if !IsGlobalBlockName(name) {
			t.Errorf("IsGlobalBlockName(%q) = false, want true", name)
		}
	}
	if IsGlobalBlockName(sshIncludeBlockName) {
		t.Error("IsGlobalBlockName(sshIncludeBlockName) = true, want false — the Include line is reserved but is NOT a movable globals stanza")
	}
	if IsGlobalBlockName("personal") {
		t.Error(`IsGlobalBlockName("personal") = true, want false`)
	}
}

// TestReservedPaths proves the gitid-owned Include'd storage locations are
// registered: the config.d directory and its *.config glob, PLUS (D-06) the
// key-archive directory and a recursive glob beneath it, appended AFTER the
// config.d entries (L4; the SSH-side seed of the reserved-PATH registry
// Phase 8 D-06.2 generalizes).
func TestReservedPaths(t *testing.T) {
	sshDir := filepath.Join(t.TempDir(), ".ssh")
	got := ReservedPaths(sshDir)

	want := []string{
		filepath.Join(sshDir, "config.d"),
		filepath.Join(sshDir, "config.d", "*.config"),
		filepath.Join(sshDir, "gitid-archive"),
		filepath.Join(sshDir, "gitid-archive", "**"),
	}
	if len(got) != len(want) {
		t.Fatalf("ReservedPaths(%q) = %v, want %v", sshDir, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ReservedPaths()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestIsReservedPath proves only the gitid-owned Include'd storage is
// reserved: the config.d directory itself and the `*.config` files inside
// it, PLUS (D-06) the key-archive directory itself and any path nested
// underneath it at any depth (review R-19). The main config, key material
// and non-`.config` files inside config.d are NOT gitid storage and must
// stay outside the registry.
func TestIsReservedPath(t *testing.T) {
	sshDir := filepath.Join(t.TempDir(), ".ssh")

	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(sshDir, "config.d"), true},
		{filepath.Join(sshDir, "config.d", "gitid.config"), true},
		// Uncleaned form must still match (filepath.Clean comparison).
		{filepath.Join(sshDir, "config.d") + "/./gitid.config", true},
		{filepath.Join(sshDir, "config"), false},
		{filepath.Join(sshDir, "id_ed25519"), false},
		{filepath.Join(sshDir, "config.d", "notes.txt"), false},
		{filepath.Join(sshDir, "config.d", "foo.txt"), false},
		{filepath.Join(sshDir, "config.d", "foo.config"), true},
		// D-06 archive additions (review R-19): reserved at any depth.
		{filepath.Join(sshDir, "gitid-archive"), true},
		{filepath.Join(sshDir, "gitid-archive", "id_ed25519_work.170000"), true},
		{filepath.Join(sshDir, "gitid-archive", "generation-1", "id_ed25519_work.170000"), true},
	}
	for _, tc := range cases {
		if got := IsReservedPath(sshDir, tc.path); got != tc.want {
			t.Errorf("IsReservedPath(%q, %q) = %v, want %v", sshDir, tc.path, got, tc.want)
		}
	}
}

// TestManagedBlockNamesIsIncludeAware proves managed-block discovery unions the
// blocks of ~/.ssh/config with the blocks of every Include'd gitid-owned file.
//
// This is the L4 hazard D-06 introduces: on a fresh Include'd machine every
// identity Host block lives in config.d/gitid.config, so a doctor composition
// root reading only ~/.ssh/config sees ZERO identity blocks and CheckOrphans
// offers a DESTRUCTIVE removal fix for every legitimate gitconfig block.
func TestManagedBlockNamesIsIncludeAware(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath, _ := seedIncludeLayout(t, home)

	names, err := ManagedBlockNames(configPath)
	if err != nil {
		t.Fatalf("ManagedBlockNames: %v", err)
	}

	// The main config alone only carries the reserved wiring blocks; the
	// identity block is only reachable THROUGH the Include.
	mainOnly := filewriter.ListBlocks(mustRead(t, configPath))
	for _, b := range mainOnly {
		if b.Name == "personal" {
			t.Fatal("fixture invalid: the identity block must live in config.d, not in ~/.ssh/config")
		}
	}

	for _, want := range []string{"ssh-include", "_global", "personal"} {
		if !containsName(names, want) {
			t.Errorf("ManagedBlockNames = %v, want it to contain %q", names, want)
		}
	}
}

// TestManagedBlockNamesToleratesMissingIncludedFile proves a glob that resolves
// to nothing (the Include'd file was never created) is skipped, not an error.
func TestManagedBlockNamesToleratesMissingIncludedFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seeding .ssh: %v", err)
	}
	configPath := filepath.Join(sshDir, "config")
	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("EnsureIncludeLine: %v", err)
	}
	// No config.d directory at all — the Include glob matches nothing.

	names, err := ManagedBlockNames(configPath)
	if err != nil {
		t.Fatalf("ManagedBlockNames with a missing Include'd file: %v", err)
	}
	if !containsName(names, "ssh-include") {
		t.Errorf("ManagedBlockNames = %v, want it to contain %q", names, "ssh-include")
	}
}

// TestManagedBlockNamesUnreadableIncludedFileErrors proves a genuinely
// unreadable Include'd match is reported, not silently swallowed: the glob
// matched, so gitid must not pretend the file's blocks do not exist.
func TestManagedBlockNamesUnreadableIncludedFileErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshDir := filepath.Join(home, ".ssh")
	configDir := filepath.Join(sshDir, "config.d")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("seeding config.d: %v", err)
	}
	configPath := filepath.Join(sshDir, "config")
	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("EnsureIncludeLine: %v", err)
	}
	// A DIRECTORY matching the *.config glob is unreadable as a file on every
	// platform and for every user (root included) — a deterministic stand-in
	// for a permission-denied match that needs no privilege assumptions.
	if err := os.MkdirAll(filepath.Join(configDir, "broken.config"), 0o700); err != nil {
		t.Fatalf("seeding unreadable match: %v", err)
	}

	if _, err := ManagedBlockNames(configPath); err == nil {
		t.Error("ManagedBlockNames with an unreadable Include'd match = nil error, want an error")
	} else if !strings.Contains(err.Error(), "reading included") {
		t.Errorf("error = %v, want it to name the unreadable included file", err)
	}
}

// seedIncludeLayout writes the D-06 Include'd fresh-machine layout under home:
// ~/.ssh/config carrying ONLY the reserved wiring (the Include line block and
// the macOS `_global` block), plus ~/.ssh/config.d/gitid.config carrying the
// identity Host block. It returns the main config path and the Include'd file
// path.
func seedIncludeLayout(t *testing.T, home string) (configPath, includedPath string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	configDir := filepath.Join(sshDir, "config.d")
	if err := EnsureIncludeDir(configDir); err != nil {
		t.Fatalf("EnsureIncludeDir: %v", err)
	}
	configPath = filepath.Join(sshDir, "config")
	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("EnsureIncludeLine: %v", err)
	}
	// The macOS globals block, written on EVERY create (D-08).
	globals := filewriter.ReplaceBlock(mustRead(t, configPath), "_global",
		"Host *\n  IgnoreUnknown UseKeychain\n  UseKeychain yes\n  AddKeysToAgent yes\n")
	if _, err := filewriter.Write(configPath, globals, 0o600); err != nil {
		t.Fatalf("writing globals block: %v", err)
	}

	includedPath = filepath.Join(configDir, "gitid.config")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443,
		filepath.Join(sshDir, "id_ed25519_personal"), "github.com")
	included := filewriter.ReplaceBlock(nil, "personal", hostBlock)
	if _, err := filewriter.Write(includedPath, included, 0o600); err != nil {
		t.Fatalf("writing Include'd gitid.config: %v", err)
	}
	return configPath, includedPath
}

// mustRead reads path or fails the test.
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return b
}

// containsName reports whether names contains want.
func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
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
