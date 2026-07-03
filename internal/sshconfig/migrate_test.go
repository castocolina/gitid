package sshconfig

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/tester"
)

// errFakeCrash is the sentinel error used by afterStep injection in the
// rollback tests below — it stands in for an arbitrary mid-transaction crash.
var errFakeCrash = errors.New("simulated crash for test")

// parseIdentityFiles extracts the resolved IdentityFile list from raw
// `ssh -G` output via the existing tester.ParseResolved helper.
func parseIdentityFiles(sshGOutput string) []string {
	return tester.ParseResolved(sshGOutput).IdentityFiles
}

// migrateFixture builds a hermetic t.TempDir() HOME with ~/.ssh seeded, and
// returns the configPath/includePath pair Migrate operates on (Pitfall 5:
// real, filesystem-backed fixtures, never bare in-memory Include content).
func migrateFixture(t *testing.T) (home, configPath, includePath string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seeding hermetic .ssh dir: %v", err)
	}
	configPath = filepath.Join(sshDir, "config")
	includePath = filepath.Join(sshDir, "config.d", "gitid.config")
	return home, configPath, includePath
}

// skipIfNoSSH skips the test when the real ssh binary is unavailable.
func skipIfNoSSH(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found; skipping migrate test")
	}
}

// mustReadFile reads path or fails the test.
func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path) //nolint:gosec // path is a hermetic t.TempDir() fixture path (G304)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return content
}

// TestMigrateToIncludeMovesBlockAndPreservesResolution proves STORE-03: a
// managed identity block starts IN-LINE in ~/.ssh/config (no Include layout
// yet); after MigrateToInclude it lives in config.d/gitid.config, the Include
// line is floored in ~/.ssh/config, and real `ssh -G` resolves the SAME
// IdentityFile before and after (behavior-preserving).
func TestMigrateToIncludeMovesBlockAndPreservesResolution(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	if _, err := Write(configPath, "personal", hostBlock, ""); err != nil {
		t.Fatalf("seeding in-line identity: %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})

	preOut, preErr := exec.Command("ssh", "-G", "-F", configPath, "personal.github.com").Output() //nolint:gosec // arg-slice form, hermetic fixture path (G204)
	if preErr != nil {
		t.Fatalf("pre-migration ssh -G: %v", preErr)
	}

	result, err := Migrate(MigrateToInclude, deps)
	if err != nil {
		t.Fatalf("Migrate(MigrateToInclude): %v", err)
	}
	if result.Recovery == "" {
		t.Error("expected non-empty Recovery description")
	}

	includeContent := mustReadFile(t, includePath)
	if len(filewriter.ListBlocks(includeContent)) == 0 {
		t.Error("expected the migrated block to be present in the Include'd file")
	}

	configContent := mustReadFile(t, configPath)
	for _, b := range filewriter.ListBlocks(configContent) {
		if b.Name == "personal" {
			t.Error("identity block must be REMOVED from ~/.ssh/config after MigrateToInclude")
		}
	}
	if !bytes.Contains(configContent, []byte("Include ~/.ssh/config.d/*.config")) {
		t.Error("expected the Include line to be floored in ~/.ssh/config after MigrateToInclude")
	}

	postOut, postErr := exec.Command("ssh", "-G", "-F", configPath, "personal.github.com").Output() //nolint:gosec // arg-slice form, hermetic fixture path (G204)
	if postErr != nil {
		t.Fatalf("post-migration ssh -G: %v", postErr)
	}
	if string(preOut) != string(postOut) {
		t.Errorf("ssh -G resolution changed after migration:\nbefore:\n%s\nafter:\n%s", preOut, postOut)
	}

	if _, perr := Parse(configContent); perr != nil {
		t.Errorf("post-migration ~/.ssh/config does not parse: %v", perr)
	}
	if _, perr := Parse(includeContent); perr != nil {
		t.Errorf("post-migration Include'd file does not parse: %v", perr)
	}
}

// TestMigrateToInFileMovesBlockBack proves the REVERSE direction: a managed
// identity block starts in the Include'd file; after MigrateToInFile it is
// back in-line in ~/.ssh/config, trimmed from the Include'd file, and
// resolution is preserved.
func TestMigrateToInFileMovesBlockBack(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	if err := EnsureIncludeDir(filepath.Dir(includePath)); err != nil {
		t.Fatalf("EnsureIncludeDir: %v", err)
	}
	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("EnsureIncludeLine: %v", err)
	}
	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	includeBody := filewriter.ReplaceBlock(nil, "personal", hostBlock)
	if _, err := filewriter.Write(includePath, includeBody, 0o600); err != nil {
		t.Fatalf("seeding Include'd identity: %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})

	preOut, preErr := exec.Command("ssh", "-G", "-F", configPath, "personal.github.com").Output() //nolint:gosec // arg-slice form, hermetic fixture path (G204)
	if preErr != nil {
		t.Fatalf("pre-migration ssh -G: %v", preErr)
	}

	if _, err := Migrate(MigrateToInFile, deps); err != nil {
		t.Fatalf("Migrate(MigrateToInFile): %v", err)
	}

	configContent := mustReadFile(t, configPath)
	var foundInline bool
	for _, b := range filewriter.ListBlocks(configContent) {
		if b.Name == "personal" {
			foundInline = true
		}
	}
	if !foundInline {
		t.Error("expected the migrated block to be present IN-LINE in ~/.ssh/config after MigrateToInFile")
	}

	includeContent := mustReadFile(t, includePath)
	for _, b := range filewriter.ListBlocks(includeContent) {
		if b.Name == "personal" {
			t.Error("identity block must be REMOVED from the Include'd file after MigrateToInFile")
		}
	}

	postOut, postErr := exec.Command("ssh", "-G", "-F", configPath, "personal.github.com").Output() //nolint:gosec // arg-slice form, hermetic fixture path (G204)
	if postErr != nil {
		t.Fatalf("post-migration ssh -G: %v", postErr)
	}
	if string(preOut) != string(postOut) {
		t.Errorf("ssh -G resolution changed after migration:\nbefore:\n%s\nafter:\n%s", preOut, postOut)
	}
}

// TestMigrateIdempotentReRunConverges proves re-running Migrate after a
// successful run converges (no duplication, no error) — the same guarantee
// that protects a crash-induced duplicate.
func TestMigrateIdempotentReRunConverges(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	if _, err := Write(configPath, "personal", hostBlock, ""); err != nil {
		t.Fatalf("seeding in-line identity: %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})

	if _, err := Migrate(MigrateToInclude, deps); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	firstConfig := mustReadFile(t, configPath)
	firstInclude := mustReadFile(t, includePath)

	if _, err := Migrate(MigrateToInclude, deps); err != nil {
		t.Fatalf("second (idempotent) Migrate: %v", err)
	}
	secondConfig := mustReadFile(t, configPath)
	secondInclude := mustReadFile(t, includePath)

	if !bytes.Equal(firstConfig, secondConfig) {
		t.Errorf("re-running Migrate changed ~/.ssh/config:\nfirst:\n%s\nsecond:\n%s", firstConfig, secondConfig)
	}
	if !bytes.Equal(firstInclude, secondInclude) {
		t.Errorf("re-running Migrate changed the Include'd file:\nfirst:\n%s\nsecond:\n%s", firstInclude, secondInclude)
	}
}

// TestMigratePreservesForeignContent proves hand-written content outside any
// gitid managed block survives migration byte-for-byte (parse->render->parse
// stability / no managed-block drift).
func TestMigratePreservesForeignContent(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	foreign := "Host handwritten\n  Hostname example.com\n"
	if err := os.WriteFile(configPath, []byte(foreign), 0o600); err != nil {
		t.Fatalf("seeding foreign content: %v", err)
	}
	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	if _, err := Write(configPath, "personal", hostBlock, ""); err != nil {
		t.Fatalf("seeding in-line identity: %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	if _, err := Migrate(MigrateToInclude, deps); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	configContent := mustReadFile(t, configPath)
	if !bytes.Contains(configContent, []byte("Host handwritten")) {
		t.Errorf("foreign hand-written content was lost during migration; got:\n%s", configContent)
	}
}

// TestMigrateInjectedFailureAfterDestinationWrittenRollsBack proves an
// injected failure right after the destination write leaves NO block loss,
// `ssh -G` still resolving, and the on-disk state restored byte-for-byte to
// the pre-migration snapshot (T-01-22).
func TestMigrateInjectedFailureAfterDestinationWrittenRollsBack(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	if _, err := Write(configPath, "personal", hostBlock, ""); err != nil {
		t.Fatalf("seeding in-line identity: %v", err)
	}

	preConfig := mustReadFile(t, configPath)

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	injected := errFakeCrash
	deps.afterStep = func(step MigrateStep) error {
		if step == StepDestinationWritten {
			return injected
		}
		return nil
	}

	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("expected Migrate to fail when afterStep injects an error after the destination write")
	}

	// No block loss: the identity must still resolve via SOME file.
	out, sshErr := exec.Command("ssh", "-G", "-F", configPath, "personal.github.com").Output() //nolint:gosec // arg-slice form, hermetic fixture path (G204)
	if sshErr != nil {
		t.Fatalf("ssh -G after injected failure: %v", sshErr)
	}
	resolved := parseIdentityFiles(string(out))
	if len(resolved) == 0 || resolved[0] != identityKey {
		t.Errorf("resolution broken after injected failure: got %v, want %s", resolved, identityKey)
	}

	// Recoverable: auto-rollback restored ~/.ssh/config byte-for-byte.
	postConfig := mustReadFile(t, configPath)
	if !bytes.Equal(preConfig, postConfig) {
		t.Errorf("~/.ssh/config was not restored to its pre-migration content after injected failure:\nwant:\n%s\ngot:\n%s",
			preConfig, postConfig)
	}
}

// TestMigrateInjectedFailureAfterSourceTrimmedRollsBack proves an injected
// failure right after the source trim (the migration's final write) still
// leaves NO block loss, `ssh -G` still resolving, and byte-for-byte
// restoration of BOTH files to their pre-migration snapshot.
func TestMigrateInjectedFailureAfterSourceTrimmedRollsBack(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	if _, err := Write(configPath, "personal", hostBlock, ""); err != nil {
		t.Fatalf("seeding in-line identity: %v", err)
	}

	preConfig := mustReadFile(t, configPath)

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	injected := errFakeCrash
	deps.afterStep = func(step MigrateStep) error {
		if step == StepSourceTrimmed {
			return injected
		}
		return nil
	}

	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("expected Migrate to fail when afterStep injects an error after the source trim")
	}

	out, sshErr := exec.Command("ssh", "-G", "-F", configPath, "personal.github.com").Output() //nolint:gosec // arg-slice form, hermetic fixture path (G204)
	if sshErr != nil {
		t.Fatalf("ssh -G after injected failure: %v", sshErr)
	}
	resolved := parseIdentityFiles(string(out))
	if len(resolved) == 0 || resolved[0] != identityKey {
		t.Errorf("resolution broken after injected failure: got %v, want %s", resolved, identityKey)
	}

	postConfig := mustReadFile(t, configPath)
	if !bytes.Equal(preConfig, postConfig) {
		t.Errorf("~/.ssh/config was not restored to its pre-migration content after injected failure:\nwant:\n%s\ngot:\n%s",
			preConfig, postConfig)
	}
	// The Include'd file must not have been left with an orphaned copy either.
	if _, statErr := os.Stat(includePath); statErr == nil {
		includeContent := mustReadFile(t, includePath)
		for _, b := range filewriter.ListBlocks(includeContent) {
			if b.Name == "personal" {
				t.Error("Include'd file retains the migrated block after rollback; expected full restoration")
			}
		}
	}
}

// TestMigrateRollbackDoesNotClobberPristineBackup proves Codex HIGH #1: a
// migration that fails validation AFTER the source trim must NOT let its
// own rollback restore-write create a NEW backup of the failed live file —
// that new backup is exactly what could clobber a PRISTINE backup (a file
// the Recovery message tells the user to restore from), under the pre-fix
// second-resolution backup naming (STORE-03 crash-safety gap).
//
// Uses RealMigrateDeps (real `ssh -G`, never faked — the CONTEXT.md-locked
// constraint for this package) and the afterStep MigrateDeps seam to inject
// the failure exactly at StepSourceTrimmed — the point immediately after
// the real post-source-trim validateResolution call has already succeeded
// and the source-trim write has already landed on disk, i.e. "fails
// [right after validating] the source trim". The test snapshots the
// backup-file count for ~/.ssh/config at that exact point (capturing the
// legitimate backup the source-trim WRITE itself creates, via
// filewriter.Write's normal backup-before-overwrite step) BEFORE the
// injected failure triggers rollback, then asserts rollback added NO
// additional backup file and that every backup file present is still
// byte-for-byte the pristine pre-migration content.
func TestMigrateRollbackDoesNotClobberPristineBackup(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	if _, err := Write(configPath, "personal", hostBlock, ""); err != nil {
		t.Fatalf("seeding in-line identity: %v", err)
	}

	// Ground truth: the pristine pre-migration bytes, captured independently
	// of Migrate's own bookkeeping.
	preConfig := mustReadFile(t, configPath)

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	var backupCountAtSourceTrimmed int
	deps.afterStep = func(step MigrateStep) error {
		if step == StepSourceTrimmed {
			matches, globErr := filepath.Glob(configPath + ".bak.*")
			if globErr != nil {
				t.Fatalf("globbing for backup files at StepSourceTrimmed: %v", globErr)
			}
			backupCountAtSourceTrimmed = len(matches)
			return errFakeCrash
		}
		return nil
	}

	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("expected Migrate to fail when afterStep injects an error after the source trim")
	}

	matchesAfter, globErr := filepath.Glob(configPath + ".bak.*")
	if globErr != nil {
		t.Fatalf("globbing for backup files after rollback: %v", globErr)
	}
	if len(matchesAfter) != backupCountAtSourceTrimmed {
		t.Fatalf("rollback changed the number of %s backup files (expected no new backup from restoring): before rollback=%d, after rollback=%d, files=%v",
			configPath, backupCountAtSourceTrimmed, len(matchesAfter), matchesAfter)
	}

	// Every backup file present must still be byte-for-byte pristine — none
	// may have been clobbered with the (bad, source-trimmed) live content by
	// rollback's own restore-write.
	for _, backupPath := range matchesAfter {
		backupBytes := mustReadFile(t, backupPath)
		if !bytes.Equal(backupBytes, preConfig) {
			t.Errorf("pristine backup %s was NOT byte-for-byte intact after rollback:\nwant:\n%s\ngot:\n%s",
				backupPath, preConfig, backupBytes)
		}
	}
}

// TestMigrateReturnsTimeoutErrorWhenSSHHangs proves Codex HIGH #2: a hung
// `ssh -G` resolution during Migrate's preflight/validation (e.g. a
// pathological config with a hanging `Match exec`) never blocks Migrate
// indefinitely. RealMigrateDeps' resolver is bounded by
// migrateResolveTimeout (exec.CommandContext), mirroring internal/platform's
// probeTimeout pattern (T-01-03). A fake `ssh` binary that sleeps 30s is
// placed first on PATH; migrateResolveTimeout is shrunk so the test itself
// stays fast.
func TestMigrateReturnsTimeoutErrorWhenSSHHangs(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "ssh")
	// #nosec G306 -- test fixture in a t.TempDir(), not a managed gitid file
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatalf("writing fake hung ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	oldTimeout := migrateResolveTimeout
	migrateResolveTimeout = 100 * time.Millisecond
	t.Cleanup(func() { migrateResolveTimeout = oldTimeout })

	_, configPath, includePath := migrateFixture(t)
	if err := os.WriteFile(configPath, []byte("Host example\n  Hostname example.com\n"), 0o600); err != nil {
		t.Fatalf("seeding config: %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"example"})

	start := time.Now()
	_, err := Migrate(MigrateToInclude, deps)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected Migrate to return an error when ssh -G hangs, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("Migrate() with a hung ssh took %v, want it bounded by migrateResolveTimeout (did not honor exec.CommandContext timeout)", elapsed)
	}
}
